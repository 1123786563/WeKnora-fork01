package commercialplatform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

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
	planAmount map[string]int64 // plan_code -> amount_cents (the authority's plan truth the index echoes)
	// pmAlwaysEmpty models an authority whose payment-method import NEVER
	// lands (the placeholder-prefix reality on a real Lago): the PM list
	// answers 200 with an empty array forever, so a waitForPaymentMethodSync
	// call would poll until its budget is exhausted.
	pmAlwaysEmpty bool
	mux           *http.ServeMux // assembled by newPurchaseStub
}

func newPurchaseStub() *purchaseStub {
	s := &purchaseStub{customer: map[string]map[string]any{}}
	// Lock discipline (the respond precedent): each handler branch mutates
	// state under a temporary lock, then calls respond unlocked (respond
	// takes mu itself — calling it under the lock would deadlock).
	customers := func(w http.ResponseWriter, r *http.Request) {
		blob, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		ext := strings.TrimPrefix(r.URL.Path, "/api/v1/customers/")
		// F11: the authority's payment-method list — the binding ensure
		// polls it until the default payment method is imported. The stub
		// models an ALREADY-synced method for every bound customer (the
		// async import lag is covered by the real-stack evidence).
		if pm, cut := strings.CutSuffix(ext, "/payment_methods"); cut && r.Method == http.MethodGet {
			s.mu.Lock()
			_, held := s.customer[pm]
			alwaysEmpty := s.pmAlwaysEmpty
			s.mu.Unlock()
			if !held {
				respond(w, r, &s.requests, &s.mu, http.StatusNotFound, `{"payment_methods":[]}`, nil)
				return
			}
			if alwaysEmpty {
				respond(w, r, &s.requests, &s.mu, http.StatusOK, `{"payment_methods":[]}`, nil)
				return
			}
			respond(w, r, &s.requests, &s.mu, http.StatusOK, `{"payment_methods":[{"id":"pm_stub_1","type":"card"}]}`, nil)
			return
		}
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
			if existing, ok := s.customer[id]; ok { // upsert (UpsertFromApiService): a field is overwritten ONLY when its key is present in the body
				if bc, ok := body["billing_configuration"]; ok {
					existing["billing_configuration"] = bc
				}
				if name, ok := body["name"]; ok {
					existing["name"] = name
				}
				body = existing
			}
			s.customer[id] = body
			s.mu.Unlock()
			b, _ := json.Marshal(map[string]any{"customer": body})
			respond(w, r, &s.requests, &s.mu, http.StatusOK, string(b), blob)
		case r.Method == http.MethodPut && ext != "": // The pinned v1.53.0 shared API exposes customers as create/index/show/destroy ONLY — there is no update route, so a single-customer PUT must 404 (issue #82 flow defect 3: the real stack answered exactly this before the fix moved the binding write to the upsert POST).
			respond(w, r, &s.requests, &s.mu, http.StatusNotFound, "{}", blob)
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
							ExternalID: id, ExternalCustomer: cust, PlanCode: code,
							// The price face comes from the authority's PLAN
							// record (never the create request): the index
							// echoes plan_amount_cents/CNY.
							AmountFen: s.planAmount[code], Currency: "CNY",
							Status: "incomplete",
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

// countCustomerPuts counts single-customer PUTs — the update-only binding
// write for an EXISTING customer (R1-V05: the name-preserving path).
func (s *purchaseStub) countCustomerPuts() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, req := range s.requests {
		if req.Method == http.MethodPut && strings.HasPrefix(req.Path, "/api/v1/customers/") &&
			!strings.HasSuffix(req.Path, "/payment_methods") {
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

// TestLagoBindingUpdateOnExistingCustomerKeepsDisplayName (R1-V05, restated
// on the pinned v1.53.0 contract after issue #82 flow defect 3): the
// customer already exists (onboarding created it with the REAL display
// name) but carries no provider binding. The shared API has NO customer
// update route (create/index/show/destroy only) — the binding write MUST be
// the collection POST upsert with NO name key (a present key would overwrite
// the display name; the old PUT form 404'd on the real stack and killed
// every first purchase with 503 invalid_response).
func TestLagoBindingUpdateOnExistingCustomerKeepsDisplayName(t *testing.T) {
	stub := newPurchaseStub()
	ext := commercial.ExternalCustomerID(27)
	stub.mu.Lock()
	stub.customer[ext] = map[string]any{"external_id": ext, "name": "真实空间名"}
	stub.mu.Unlock()
	srv := stub.server(t)
	a := purchaseAdapterWithPrefix(t, srv)
	if _, err := a.SubmitCommand(context.Background(), purchaseCmd(27, "weknora-pro-v1", 9900)); err != nil {
		t.Fatalf("create: %v", err)
	}
	stub.mu.Lock()
	cust := stub.customer[ext]
	var bindingPostBody string
	for _, req := range stub.requests {
		if req.Method == http.MethodPost && req.Path == "/api/v1/customers" {
			bindingPostBody = req.Body
		}
	}
	stub.mu.Unlock()
	if cust["name"] != "真实空间名" {
		t.Fatalf("existing customer's display name must be preserved, got %v", cust["name"])
	}
	bc, _ := cust["billing_configuration"].(map[string]any)
	if bc == nil || bc["provider_customer_id"] != "cus-dev-"+ext {
		t.Fatalf("binding must still land: %+v", bc)
	}
	// The upsert body must carry external_id + billing_configuration and NO
	// name key — that is what keeps the display name on the real stack.
	var sent struct {
		Customer map[string]any `json:"customer"`
	}
	if err := json.Unmarshal([]byte(bindingPostBody), &sent); err != nil {
		t.Fatalf("binding POST body unreadable: %v (%s)", err, bindingPostBody)
	}
	if _, hasName := sent.Customer["name"]; hasName {
		t.Fatalf("binding upsert must NOT carry a name key (it would overwrite the display name): %s", bindingPostBody)
	}
	if sent.Customer["external_id"] != ext {
		t.Fatalf("binding upsert must locate the customer by external_id, body: %s", bindingPostBody)
	}
	if n := stub.countCustomerPosts(); n != 1 {
		t.Fatalf("existing customer must be bound through exactly one collection POST (upsert), posts=%d", n)
	}
	if n := stub.countCustomerPuts(); n != 0 {
		t.Fatalf("single-customer PUT must never be sent (no update route on pinned v1.53.0, it 404s), puts=%d", n)
	}
}

// TestLagoPlaceholderBindingSkipsPaymentMethodSyncWait (R1-V06): with a
// placeholder-prefix provider customer id no provider-side customer
// exists, so the payment-method import can never land. The binding step
// must NOT poll (the old code burned the whole pmSyncWait on every
// purchase and then misclassified the stall as transient unreachable);
// the import-less stack fails fast at the gated create instead.
func TestLagoPlaceholderBindingSkipsPaymentMethodSyncWait(t *testing.T) {
	stub := newPurchaseStub()
	stub.mu.Lock()
	stub.pmAlwaysEmpty = true // the import never lands
	stub.mu.Unlock()
	srv := stub.server(t)
	a := purchaseAdapterWithPrefix(t, srv)
	oldWait, oldTick := pmSyncWait, pmSyncTick
	pmSyncWait, pmSyncTick = 700*time.Millisecond, 50*time.Millisecond
	t.Cleanup(func() { pmSyncWait, pmSyncTick = oldWait, oldTick })
	start := time.Now()
	if _, err := a.SubmitCommand(context.Background(), purchaseCmd(28, "weknora-pro-v1", 9900)); err != nil {
		t.Fatalf("placeholder binding must skip the PM sync wait entirely, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("binding must not poll the PM list for a placeholder id, took %s", elapsed)
	}
}

// TestLagoAPIBindingWithoutPmTokenSkipsPaymentMethodSyncWait (R1-24): the
// production posture leaves StripePmToken EMPTY — no default payment method
// is attached (the real card arrives through the provider checkout, #82), so
// the provider-side import can never land. Polling would burn the whole
// pmSyncWait on every purchase and then answer the wrong transient
// unreachable; the contract (config.go F11 note, providerCreateCustomer)
// promises the flow REACHES the gated create and fails closed there
// (no_default_payment_method). The binding must skip the poll entirely.
// The provider-customer derivation and the sync poll are injected through
// the R1-24 test seams (the outbound host policy refuses loopback, so the
// provider side cannot be stubbed over local HTTP).
func TestLagoAPIBindingWithoutPmTokenSkipsPaymentMethodSyncWait(t *testing.T) {
	stub := newPurchaseStub()
	stub.mu.Lock()
	stub.pmAlwaysEmpty = true // the import never lands
	stub.mu.Unlock()
	srv := stub.server(t)
	a := purchaseAdapterWithPrefix(t, srv)
	a.deriveProviderCustomer = func(_ context.Context, externalCustomerID string) (string, providerCustomerSource, error) {
		return "cus-real-" + externalCustomerID, providerCustomerAPI, nil
	}
	syncCalls := 0
	a.syncPaymentMethods = func(context.Context, string) error {
		syncCalls++
		return nil
	}
	oldWait, oldTick := pmSyncWait, pmSyncTick
	pmSyncWait, pmSyncTick = 700*time.Millisecond, 50*time.Millisecond
	t.Cleanup(func() { pmSyncWait, pmSyncTick = oldWait, oldTick })
	start := time.Now()
	if _, err := a.SubmitCommand(context.Background(), purchaseCmd(29, "weknora-pro-v1", 9900)); err != nil {
		t.Fatalf("binding without a PM token must skip the sync wait, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("binding must not poll the PM list without a PM token, took %s", elapsed)
	}
	if syncCalls != 0 {
		t.Fatalf("payment-method sync must not run without a PM token, calls=%d", syncCalls)
	}
	// The gated create was REACHED (the contract's fail-closed point), not
	// short-circuited before it.
	if n := stub.countSubscriptionPosts(); n != 1 {
		t.Fatalf("the flow must reach the gated create, subscription posts=%d", n)
	}
}

// TestLagoAPIBindingWithPmTokenStillPollsPaymentMethodSync (R1-24): the
// dev/test posture attaches a payment-method token (StripePmToken set), so
// the import CAN land and the bounded sync poll must still run — the R1-24
// skip is scoped to the empty-token production posture only.
func TestLagoAPIBindingWithPmTokenStillPollsPaymentMethodSync(t *testing.T) {
	stub := newPurchaseStub()
	srv := stub.server(t)
	a := purchaseAdapterWithPrefix(t, srv)
	a.cfg.StripePmToken = "pm_test_canary" // dev/test posture: a PM is attached
	a.deriveProviderCustomer = func(_ context.Context, externalCustomerID string) (string, providerCustomerSource, error) {
		return "cus-real-" + externalCustomerID, providerCustomerAPI, nil
	}
	syncCalls := 0
	a.syncPaymentMethods = func(context.Context, string) error {
		syncCalls++
		return nil
	}
	if _, err := a.SubmitCommand(context.Background(), purchaseCmd(30, "weknora-pro-v1", 9900)); err != nil {
		t.Fatalf("create: %v", err)
	}
	if syncCalls != 1 {
		t.Fatalf("the PM sync must run exactly once when a token is attached, calls=%d", syncCalls)
	}
}

// TestLagoPaymentMethodSyncCancellationIsNotUnreachable (R1-24): a cancelled
// caller context (client disconnect, command budget deadline) is the
// caller's own end — it must surface context.Canceled on its own error
// chain, never the platform-unreachable sentinel (which would invite a
// doomed retry into a poll the caller just cancelled). The stub cancels the
// context after its first response is flushed and the poll tick (500ms) is
// far longer than the delivery latency, so the cancellation is observed by
// sleepCtx INSIDE a sleep, never by an in-flight request.
func TestLagoPaymentMethodSyncCancellationIsNotUnreachable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	served := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"payment_methods":[]}`))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		served <- struct{}{}
	}))
	t.Cleanup(srv.Close)
	go func() {
		<-served
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	a := NewLagoAdapter(Config{Provider: ProviderLago, BaseURL: srv.URL, APIKey: testAPIKey, Release: "v1.53.0"})
	oldWait, oldTick := pmSyncWait, pmSyncTick
	pmSyncWait, pmSyncTick = 10*time.Second, 500*time.Millisecond
	t.Cleanup(func() { pmSyncWait, pmSyncTick = oldWait, oldTick })
	err := a.waitForPaymentMethodSync(ctx, commercial.ExternalCustomerID(31))
	if err == nil {
		t.Fatal("cancelled sync must fail")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("caller cancellation must surface context.Canceled, got %v", err)
	}
	if errors.Is(err, commercial.ErrPlatformUnreachable) {
		t.Fatalf("caller cancellation must NOT be classified unreachable, got %v", err)
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
		// (R1-V08) inet_aton shorthand loopback/private shapes that
		// net.ParseIP does not recognize as literals but resolvers may
		// still answer at dial time — refused on shape (no letters).
		"https://127.1", "https://2130706433", "https://0x7f000001",
		"https://0177.0.0.1", "https://3232235521",
	} {
		if err := validateOutboundHost(bad); err == nil {
			t.Errorf("%s must be rejected", bad)
		}
	}
}

// TestProviderOutboundClientRejectsInternalRedirect (R1-V08): a hostile or
// compromised endpoint cannot 302 the egress onto internal infrastructure —
// every redirect hop is re-validated against the same host policy and a
// link-local/loopback Location aborts the request instead of being
// followed, while a public hop passes.
func TestProviderOutboundClientRejectsInternalRedirect(t *testing.T) {
	client := providerOutboundClient()
	for _, hop := range []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://127.0.0.1:48889/api/v1/customers",
		"https://10.1.2.3/internal",
	} {
		u, err := url.Parse(hop)
		if err != nil {
			t.Fatal(err)
		}
		if err := client.CheckRedirect(&http.Request{URL: u}, nil); err == nil {
			t.Errorf("redirect hop %s must be refused", hop)
		}
	}
	public, _ := url.Parse("https://api.stripe.com/v1/customers")
	if err := client.CheckRedirect(&http.Request{URL: public}, nil); err != nil {
		t.Fatalf("a public redirect hop must be allowed: %v", err)
	}
}

func TestLagoPurchaseSnapshotMapsClosedStates(t *testing.T) {
	stub := newPurchaseStub()
	// The authority's plan truth: the index echoes the plan's frozen amount.
	stub.mu.Lock()
	stub.planAmount = map[string]int64{"weknora-pro-v1": 9900}
	stub.mu.Unlock()
	srv := stub.server(t)
	a := purchaseAdapterWithPrefix(t, srv)
	// Untouched: absent.
	snap, err := a.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 26})
	if err != nil || snap.Purchase.State != commercial.PurchaseStateAbsent {
		t.Fatalf("absent expected, got %+v err=%v", snap.Purchase, err)
	}
	if _, err := a.SubmitCommand(context.Background(), purchaseCmd(26, "weknora-pro-v1", 9900)); err != nil {
		t.Fatal(err)
	}
	snap, err = a.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 26})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Purchase.State != commercial.PurchaseStateAwaitingPayment {
		t.Fatalf("incomplete must map to awaiting_payment, got %+v", snap.Purchase)
	}
	if snap.Purchase.AmountFen != 9900 || snap.Purchase.Currency != commercial.CurrencyCNY ||
		snap.Purchase.PlanCode != "weknora-pro-v1" {
		t.Fatalf("frozen price face mismatch: %+v", snap.Purchase)
	}
	if len(snap.Purchase.InvoiceFees) != 0 {
		t.Fatalf("open-stage invoice fees must be EMPTY (API-invisible, never fabricated), got %+v", snap.Purchase.InvoiceFees)
	}
	// The subscription index request must carry explicit status[] (the F6
	// default-active trap — the stub answers active-only WITHOUT status[],
	// so both the result and the RawQuery prove the explicit set).
	stub.mu.Lock()
	raw := ""
	if len(stub.rawQueries) > 0 {
		raw = stub.rawQueries[len(stub.rawQueries)-1]
	}
	stub.mu.Unlock()
	if !strings.Contains(raw, "status%5B%5D=incomplete") && !strings.Contains(raw, "status[]=incomplete") {
		t.Fatalf("subscription index must pass explicit status[] (F6), raw query = %q", raw)
	}
	// Advance to active (the real environment moves via the provider
	// payment, F9).
	stub.mu.Lock()
	for i := range stub.subs {
		if stub.subs[i].ExternalID == commercial.ExternalPurchaseSubscriptionID(26) {
			stub.subs[i].Status = "active"
		}
	}
	stub.mu.Unlock()
	snap, err = a.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 26})
	if err != nil || snap.Purchase.State != commercial.PurchaseStateActive {
		t.Fatalf("active expected, got %+v err=%v", snap.Purchase, err)
	}
}
