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
		case r.Method == http.MethodPut && ext != "": // PUT /api/v1/customers/{id} — update-only semantics: merge billing_configuration, NEVER touch any other field (an absent customer is still a 404).
			var parsed struct {
				Customer map[string]any `json:"customer"`
			}
			_ = json.Unmarshal(blob, &parsed)
			body := parsed.Customer
			if body == nil {
				respond(w, r, &s.requests, &s.mu, http.StatusUnprocessableEntity, "{}", blob)
				return
			}
			s.mu.Lock()
			existing, ok := s.customer[ext]
			if ok {
				if bc, has := body["billing_configuration"]; has {
					existing["billing_configuration"] = bc
				}
			}
			s.mu.Unlock()
			if !ok {
				respond(w, r, &s.requests, &s.mu, http.StatusNotFound, "{}", blob)
				return
			}
			b, _ := json.Marshal(map[string]any{"customer": existing})
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

// TestLagoBindingUpdateOnExistingCustomerKeepsDisplayName (R1-V05): the
// customer already exists (onboarding created it with the REAL display
// name) but carries no provider binding — the binding write must be the
// update-only PUT (billing_configuration only) and must never overwrite
// the display name with the bare external id.
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
	stub.mu.Unlock()
	if cust["name"] != "真实空间名" {
		t.Fatalf("existing customer's display name must be preserved, got %v", cust["name"])
	}
	bc, _ := cust["billing_configuration"].(map[string]any)
	if bc == nil || bc["provider_customer_id"] != "cus-dev-"+ext {
		t.Fatalf("binding must still land: %+v", bc)
	}
	if n := stub.countCustomerPosts(); n != 0 {
		t.Fatalf("existing customer must be bound via PUT, not a collection POST, posts=%d", n)
	}
	if n := stub.countCustomerPuts(); n != 1 {
		t.Fatalf("exactly one update-only PUT expected, puts=%d", n)
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

// Task 4 (#82, T09 mandatory condition 3): the finalized-stage invoice line
// read. F6 contract: v1.53 fee objects carry the line TYPE inside item.type
// (no top-level fee_type), and only a FINALIZED (visible-status) invoice
// answers lines; the open stage keeps answering empty — an invisible line is
// never fabricated (F3-F5).
func TestLagoReadPurchaseInvoiceFees(t *testing.T) {
	var invoiceHandler func(w http.ResponseWriter, r *http.Request)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if invoiceHandler != nil {
			invoiceHandler(w, r)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	a := NewLagoAdapter(Config{Provider: ProviderLago, BaseURL: srv.URL, APIKey: testAPIKey})

	// Finalized invoice: the fees map item.type/item.name/amount_cents.
	invoiceHandler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"invoices":[{"lago_id":"inv_1","invoice_type":"subscription",
			"status":"finalized","payment_status":"succeeded","number":"WK-001",
			"total_amount_cents":9900,
			"fees":[{"item":{"type":"subscription","code":"plan-p","name":"Pro"},
				"amount_cents":9900,"amount_currency":"CNY","units":"1"}]}]}`))
	}
	lines, err := a.readPurchaseInvoiceFees(context.Background(), 11)
	if err != nil || len(lines) != 1 {
		t.Fatalf("finalized fees must read, got %v lines=%v err=%v", lines, len(lines), err)
	}
	if lines[0].Kind != "subscription" || lines[0].AmountFen != 9900 || lines[0].Name != "Pro" {
		t.Fatalf("fee mapping wrong: %+v", lines[0])
	}

	// Open stage: no finalized invoice → empty (invisible lines never fabricated).
	invoiceHandler = func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"invoices":[]}`))
	}
	lines, err = a.readPurchaseInvoiceFees(context.Background(), 11)
	if err != nil || lines != nil {
		t.Fatalf("open stage must answer empty, got %v err=%v", lines, err)
	}

	// A missing customer/invoice surface answers 404: v1.53.0 has no
	// invoices for the tenant yet — the 404 folds into the SAME empty
	// semantics (the open stage), never an invalid-response failure.
	invoiceHandler = func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}
	lines, err = a.readPurchaseInvoiceFees(context.Background(), 11)
	if err != nil || lines != nil {
		t.Fatalf("404 must fold into empty semantics, got %v err=%v", lines, err)
	}

	// Other definitive 4xx: invalid response (fail closed).
	invoiceHandler = func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "{}", http.StatusBadRequest)
	}
	if _, err := a.readPurchaseInvoiceFees(context.Background(), 11); !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("4xx must be invalid response, got %v", err)
	}

	// 5xx: unreachable (transient).
	invoiceHandler = func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "{}", http.StatusInternalServerError)
	}
	if _, err := a.readPurchaseInvoiceFees(context.Background(), 11); !errors.Is(err, commercial.ErrPlatformUnreachable) {
		t.Fatalf("5xx must be unreachable, got %v", err)
	}

	// A fee without item.type maps to Kind "unknown" and keeps reading.
	invoiceHandler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"invoices":[{"lago_id":"inv_2","invoice_type":"subscription",
			"status":"finalized","fees":[{"amount_cents":100,"amount_currency":"CNY"}]}]}`))
	}
	lines, err = a.readPurchaseInvoiceFees(context.Background(), 11)
	if err != nil || len(lines) != 1 || lines[0].Kind != "unknown" || lines[0].AmountFen != 100 {
		t.Fatalf("typeless fee must map unknown, got %+v err=%v", lines, err)
	}

	// The read is addressed by the DERIVED customer identity only.
	invoiceHandler = func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "external_customer_id=weknora-tenant-11") {
			t.Errorf("invoices read must be addressed by the derived identity, got %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"invoices":[]}`))
	}
	if _, err := a.readPurchaseInvoiceFees(context.Background(), 11); err != nil {
		t.Fatalf("identity-addressed read failed: %v", err)
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
