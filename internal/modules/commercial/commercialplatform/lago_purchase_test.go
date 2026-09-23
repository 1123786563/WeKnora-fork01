package commercialplatform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	commercial "github.com/Tencent/WeKnora/internal/modules/commercial"
)

// ---- purchase stub (the subscriptionsStub pattern, extended with the
// provider-binding customer surface) ----

// purchaseSubRec is one recorded purchase subscription inside the stub; the
// index answer carries its frozen price face (plan_amount_cents /
// plan_amount_currency, the t02 index evidence fields).
type purchaseSubRec struct {
	ExternalID       string
	ExternalCustomer string
	PlanCode         string
	AmountFen        int64
	Currency         string
	Status           string
}

func purchaseSubscriptionsJSON(subs []purchaseSubRec) string {
	out := make([]string, 0, len(subs))
	for _, sub := range subs {
		out = append(out, fmt.Sprintf(
			`{"lago_id":"sub_%s","external_id":%q,"external_customer_id":%q,"plan_code":%q,"status":%q,"plan_amount_cents":%d,"plan_amount_currency":%q}`,
			sub.ExternalID, sub.ExternalID, sub.ExternalCustomer, sub.PlanCode, sub.Status, sub.AmountFen, sub.Currency))
	}
	return `{"subscriptions":[` + strings.Join(out, ",") + `]}`
}

// purchaseStub is a stateful mini authority with provider-binding semantics:
// the customers collection POST (billing_configuration upsert) and the
// single GET; the subscriptions POST (records body + activation_rules, the
// side effect ALWAYS happens) and the identity index (external_id +
// status[] filter, the RawQuery recorded). createNext scripts response
// statuses (the 422 race branch).
type purchaseStub struct {
	mu         sync.Mutex
	requests   []stubReq
	rawQueries []string                  // subscriptions index RawQuery (the status[] assertion face)
	customer   map[string]map[string]any // external_id -> customer body
	subs       []purchaseSubRec
	subBodies  []map[string]any // every subscription POST body
	createNext []int            // scripted statuses; empty means 200
	mux        *http.ServeMux   // assembled by newPurchaseStub
}

func newPurchaseStub() *purchaseStub {
	s := &purchaseStub{customer: map[string]map[string]any{}}
	// Lock discipline (the respond precedent): each handler branch mutates
	// state under a temporary lock, then calls respond unlocked (respond
	// takes mu itself — calling it under the lock would deadlock).
	customers := func(w http.ResponseWriter, r *http.Request) {
		blob, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		ext := strings.TrimPrefix(r.URL.Path, "/api/v1/customers/")
		switch {
		case r.Method == http.MethodGet && ext != "": // GET /api/v1/customers/{id}
			s.mu.Lock()
			c, ok := s.customer[ext]
			var b []byte
			if ok {
				b, _ = json.Marshal(map[string]any{"customer": c})
			}
			s.mu.Unlock()
			if !ok {
				respond(w, r, &s.requests, &s.mu, http.StatusNotFound, "{}", nil)
				return
			}
			respond(w, r, &s.requests, &s.mu, http.StatusOK, string(b), nil)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers":
			// The case condition MUST be an exact-path comparison: TrimPrefix's
			// prefix carries "/", so the tail-less /api/v1/customers comes back
			// unchanged (non-empty), making `ext == ""` unreachable and falling
			// into the default 404 (verified by compiling and running this
			// stub); the key also comes from the request body
			// customer.external_id, never s.customer[ext].
			var parsed struct {
				Customer map[string]any `json:"customer"`
			}
			_ = json.Unmarshal(blob, &parsed)
			body := parsed.Customer
			if body == nil {
				respond(w, r, &s.requests, &s.mu, http.StatusUnprocessableEntity, "{}", blob)
				return
			}
			id, _ := body["external_id"].(string)
			s.mu.Lock()
			if existing, ok := s.customer[id]; ok { // upsert: merge billing_configuration
				if bc, ok := body["billing_configuration"]; ok {
					existing["billing_configuration"] = bc
				}
				body = existing
			}
			s.customer[id] = body
			s.mu.Unlock()
			b, _ := json.Marshal(map[string]any{"customer": body})
			respond(w, r, &s.requests, &s.mu, http.StatusOK, string(b), blob)
		default:
			http.NotFound(w, r)
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/customers", customers)  // collection endpoint (exact, no trailing slash)
	mux.HandleFunc("/api/v1/customers/", customers) // single lookup subtree
	mux.HandleFunc("/api/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		blob, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		switch r.Method {
		case http.MethodPost:
			var body map[string]any
			_ = json.Unmarshal(blob, &body)
			status := func() int {
				s.mu.Lock()
				defer s.mu.Unlock()
				s.subBodies = append(s.subBodies, body)
				st := http.StatusOK
				if len(s.createNext) > 0 {
					st = s.createNext[0]
					s.createNext = s.createNext[1:]
				}
				// The side effect always happens: a scripted 422 simulates the
				// "created but the response failed" race (t02 duplicates).
				if sub, ok := body["subscription"].(map[string]any); ok {
					if id, _ := sub["external_id"].(string); id != "" {
						code, _ := sub["plan_code"].(string)
						cust, _ := sub["external_customer_id"].(string)
						s.subs = append(s.subs, purchaseSubRec{
							ExternalID: id, ExternalCustomer: cust, PlanCode: code, Status: "incomplete",
						})
					}
				}
				return st
			}()
			s.mu.Lock()
			snapshot := append([]purchaseSubRec(nil), s.subs...)
			s.mu.Unlock()
			respond(w, r, &s.requests, &s.mu, status, purchaseSubscriptionsJSON(snapshot), blob)
		case http.MethodGet: // identity index: models the v1.53.0 default status=active filter
			s.mu.Lock()
			s.rawQueries = append(s.rawQueries, r.URL.RawQuery)
			q := r.URL.Query()
			want := q.Get("external_id")
			statuses := q["status[]"]
			out := []purchaseSubRec{}
			for _, sub := range s.subs {
				if sub.ExternalID != want {
					continue
				}
				if len(statuses) == 0 && sub.Status != "active" {
					continue // default filter: incomplete is invisible without status[] (F6 trap)
				}
				matched := len(statuses) == 0
				for _, st := range statuses {
					if st == sub.Status {
						matched = true
					}
				}
				if matched {
					out = append(out, sub)
				}
			}
			s.mu.Unlock()
			respond(w, r, &s.requests, &s.mu, http.StatusOK, purchaseSubscriptionsJSON(out), nil)
		default:
			http.NotFound(w, r)
		}
	})
	s.mux = mux
	return s
}

// ServeHTTP exposes the stub as an http.Handler (server() mounts s.mux).
func (s *purchaseStub) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func (s *purchaseStub) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	return srv
}

func (s *purchaseStub) countSubscriptionPosts() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, req := range s.requests {
		if req.Method == http.MethodPost && strings.HasSuffix(req.Path, "/api/v1/subscriptions") {
			n++
		}
	}
	return n
}

// countCustomerPosts counts collection-endpoint POSTs (the assertion face of
// the "skip when already bound" semantics: the replay's ensureProviderBinding
// must GET-hit the existing binding and never POST again).
func (s *purchaseStub) countCustomerPosts() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, req := range s.requests {
		if req.Method == http.MethodPost && req.Path == "/api/v1/customers" {
			n++
		}
	}
	return n
}

// purchaseAdapterWithPrefix runs the binding through the placeholder prefix
// (no Stripe env dependency). The API key rides the package's
// secret-for-test-only constant (never a real credential shape).
func purchaseAdapterWithPrefix(t *testing.T, srv *httptest.Server) *LagoAdapter {
	t.Helper()
	return NewLagoAdapter(Config{
		Provider: ProviderLago, BaseURL: srv.URL, APIKey: testAPIKey, Release: "v1.53.0",
		ProviderCustomerPrefix: "cus-dev",
	})
}

func purchaseCmd(tenant uint64, planCode string, amount int64) commercial.Command {
	return commercial.Command{
		Kind:  commercial.CommandKindCreatePurchaseSubscription,
		Key:   commercial.CreatePurchaseSubscriptionCommandKey(commercial.ExternalPurchaseSubscriptionID(tenant), planCode),
		Actor: "test", Reason: "purchase",
		Payload: commercial.CreatePurchaseSubscriptionPayload{
			TenantID: tenant, ExternalCustomerID: commercial.ExternalCustomerID(tenant),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(tenant),
			PlanCode:                       planCode, AmountFen: amount, Currency: commercial.CurrencyCNY,
		},
	}
}

func TestLagoCreatePurchaseBindsProviderThenCreatesGated(t *testing.T) {
	stub := newPurchaseStub()
	srv := stub.server(t)
	a := purchaseAdapterWithPrefix(t, srv)
	if _, err := a.SubmitCommand(context.Background(), purchaseCmd(21, "weknora-pro-v1", 9900)); err != nil {
		t.Fatalf("create: %v", err)
	}
	// Assertion 1: the customer was upserted with the billing_configuration.
	stub.mu.Lock()
	cust := stub.customer[commercial.ExternalCustomerID(21)]
	stub.mu.Unlock()
	if cust == nil {
		t.Fatal("customer must be present")
	}
	bc, _ := cust["billing_configuration"].(map[string]any)
	if bc == nil || bc["provider_customer_id"] != "cus-dev-"+commercial.ExternalCustomerID(21) {
		t.Fatalf("provider binding missing/wrong: %+v", bc)
	}
	// Assertion 2: the subscription POST body carries the payment rule with
	// timeout 0 (F1/F8).
	stub.mu.Lock()
	var lastBody map[string]any
	if len(stub.subBodies) > 0 {
		lastBody = stub.subBodies[len(stub.subBodies)-1]
	}
	stub.mu.Unlock()
	sub, _ := lastBody["subscription"].(map[string]any)
	rules, _ := sub["activation_rules"].([]any)
	if len(rules) != 1 {
		t.Fatalf("activation_rules must carry exactly one payment rule, body=%v", sub)
	}
	rule, _ := rules[0].(map[string]any)
	if rule["type"] != "payment" || fmt.Sprint(rule["timeout_hours"]) != "0" {
		t.Fatalf("rule = %+v, want payment/timeout_hours 0", rule)
	}
}

func TestLagoCreatePurchaseReplayNeverReposts(t *testing.T) {
	stub := newPurchaseStub()
	srv := stub.server(t)
	a := purchaseAdapterWithPrefix(t, srv)
	cmd := purchaseCmd(22, "weknora-pro-v1", 9900)
	if _, err := a.SubmitCommand(context.Background(), cmd); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SubmitCommand(context.Background(), cmd); err != nil {
		t.Fatal(err)
	}
	if n := stub.countSubscriptionPosts(); n != 1 {
		t.Fatalf("replay must NEVER re-POST the subscription (F7 deferred-terminate risk), posts=%d", n)
	}
	// "Skip when already bound": the replay's ensureProviderBinding must
	// GET-hit the existing binding and never POST /api/v1/customers again.
	if n := stub.countCustomerPosts(); n != 1 {
		t.Fatalf("replay must skip the provider-binding POST (GET hit expected), customer posts=%d", n)
	}
}

func TestLagoCreatePurchase422ResolvedByIdentityReread(t *testing.T) {
	stub := newPurchaseStub()
	stub.mu.Lock()
	// The first POST answers 422 but the side effect has already happened
	// (the stub always records) — the race-created-success shape.
	stub.createNext = []int{http.StatusUnprocessableEntity}
	stub.mu.Unlock()
	srv := stub.server(t)
	a := purchaseAdapterWithPrefix(t, srv)
	if _, err := a.SubmitCommand(context.Background(), purchaseCmd(23, "weknora-pro-v1", 9900)); err != nil {
		t.Fatalf("422 must resolve by identity re-read: %v", err)
	}
	if n := stub.countSubscriptionPosts(); n != 1 {
		t.Fatalf("422 path must not issue a second create, posts=%d", n)
	}
}

func TestLagoCreatePurchaseDifferentPlanIsConflict(t *testing.T) {
	stub := newPurchaseStub()
	srv := stub.server(t)
	a := purchaseAdapterWithPrefix(t, srv)
	if _, err := a.SubmitCommand(context.Background(), purchaseCmd(24, "weknora-pro-v1", 9900)); err != nil {
		t.Fatal(err)
	}
	_, err := a.SubmitCommand(context.Background(), purchaseCmd(24, "weknora-max-v1", 19900))
	if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("concurrent plan change must be a definitive conflict, got %v", err)
	}
	if n := stub.countSubscriptionPosts(); n != 1 {
		t.Fatalf("conflict must not create a second subscription, posts=%d", n)
	}
}

func TestLagoCreatePurchaseFailsClosedWithoutBindingSource(t *testing.T) {
	stub := newPurchaseStub()
	srv := stub.server(t)
	a := NewLagoAdapter(Config{Provider: ProviderLago, BaseURL: srv.URL, APIKey: testAPIKey, Release: "v1.53.0"})
	_, err := a.SubmitCommand(context.Background(), purchaseCmd(25, "weknora-pro-v1", 9900))
	if !errors.Is(err, commercial.ErrPlatformUnconfigured) {
		t.Fatalf("no stripe key and no prefix must fail closed unconfigured, got %v", err)
	}
}

func TestValidateOutboundHost(t *testing.T) {
	for _, ok := range []string{"https://api.stripe.com", "http://api.stripe.com/v1"} {
		if err := validateOutboundHost(ok); err != nil {
			t.Errorf("%s must pass: %v", ok, err)
		}
	}
	for _, bad := range []string{
		"https://localhost", "https://127.0.0.1", "https://127.1.2.3", "https://[::1]",
		"https://10.0.0.5", "https://172.16.0.1", "https://192.168.1.4", "https://169.254.1.1",
		"ftp://api.stripe.com", "https://0.0.0.0",
	} {
		if err := validateOutboundHost(bad); err == nil {
			t.Errorf("%s must be rejected", bad)
		}
	}
}
