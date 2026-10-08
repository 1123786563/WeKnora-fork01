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

	commercial "github.com/Tencent/WeKnora/internal/commercial"
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
	// rejectCreateNoDefaultPM models the gated create answering 422
	// no_default_payment_method (R1-12): the payment-method import has not
	// landed yet, so the create is refused and NO subscription record is
	// created.
	rejectCreateNoDefaultPM bool
	invoices                []purchaseInvoiceRec // the finalized-stage visible set
	// invoiceIndexStatus / invoiceDetailStatus script a NON-2xx answer on the
	// corresponding face (0 = the normal 200) — the A-25 transient tests.
	invoiceIndexStatus  int
	invoiceDetailStatus int
	// subscriptionIndexStatus scripts a NON-2xx answer on the subscriptions
	// identity index (0 = the normal 200) — the D13 transient-classification
	// tests (429 must be unreachable, never the definitive sentinel).
	subscriptionIndexStatus int
	// customerGetStatuses scripts per-call statuses on the single-customer
	// GET (consumed front-first; empty = the normal path). Settle runs TWO
	// binding reads back to back (customerProviderBound then
	// boundProviderCustomerID) — the queue distinguishes them: [200, 429]
	// makes the SECOND read answer the transient throttle.
	customerGetStatuses []int
	mux                 *http.ServeMux // assembled by newPurchaseStub
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
			scripted := 0
			if len(s.customerGetStatuses) > 0 {
				scripted = s.customerGetStatuses[0]
				s.customerGetStatuses = s.customerGetStatuses[1:]
			}
			c, ok := s.customer[ext]
			var b []byte
			if ok {
				b, _ = json.Marshal(map[string]any{"customer": c})
			}
			s.mu.Unlock()
			if scripted != 0 && scripted != http.StatusOK {
				// A scripted NON-200 answer (the transient-classification
				// tests). A scripted 200 falls through to the REAL body —
				// the queue distinguishes consecutive reads without
				// corrupting the first one's payload.
				respond(w, r, &s.requests, &s.mu, scripted, "{}", nil)
				return
			}
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
	mux.HandleFunc("/api/v1/invoices", func(w http.ResponseWriter, r *http.Request) {
		// The finalized-stage read (Task 4): external_customer_id +
		// status[]=finalized; the RawQuery is recorded for the assertion
		// face, the rows answer the stub's set (visible finalized only).
		s.mu.Lock()
		if s.invoiceIndexStatus != 0 {
			code := s.invoiceIndexStatus
			s.mu.Unlock()
			respond(w, r, &s.requests, &s.mu, code, "{}", nil)
			return
		}
		s.rawQueries = append(s.rawQueries, r.URL.RawQuery)
		q := r.URL.Query()
		want := q.Get("external_customer_id")
		statuses := q["status[]"]
		out := []purchaseInvoiceRec{}
		for _, inv := range s.invoices {
			if inv.ExternalCust != want {
				continue
			}
			matched := false
			for _, st := range statuses {
				if st == "finalized" {
					matched = true
				}
			}
			if len(statuses) > 0 && !matched {
				continue
			}
			out = append(out, inv)
		}
		s.mu.Unlock()
		respond(w, r, &s.requests, &s.mu, http.StatusOK, purchaseInvoicesJSON(out), nil)
	})
	mux.HandleFunc("/api/v1/invoices/", func(w http.ResponseWriter, r *http.Request) {
		// The single-invoice read (fees ride ONLY here — the pinned index
		// answer carries none, t9 evidence).
		s.mu.Lock()
		if s.invoiceDetailStatus != 0 {
			code := s.invoiceDetailStatus
			s.mu.Unlock()
			respond(w, r, &s.requests, &s.mu, code, "{}", nil)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/invoices/")
		var found *purchaseInvoiceRec
		for i := range s.invoices {
			if s.invoices[i].LagoID == id {
				found = &s.invoices[i]
				break
			}
		}
		s.mu.Unlock()
		if found == nil {
			respond(w, r, &s.requests, &s.mu, http.StatusNotFound, "{}", nil)
			return
		}
		respond(w, r, &s.requests, &s.mu, http.StatusOK,
			purchaseInvoiceDetailJSON(*found, commercial.ExternalPurchaseSubscriptionID(31)), nil)
	})
	mux.HandleFunc("/api/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		blob, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		switch r.Method {
		case http.MethodPost:
			var body map[string]any
			_ = json.Unmarshal(blob, &body)
			s.mu.Lock()
			noDefaultPM := s.rejectCreateNoDefaultPM
			s.mu.Unlock()
			if noDefaultPM {
				// (R1-12) the import-not-landed refusal: no record is created.
				respond(w, r, &s.requests, &s.mu, http.StatusUnprocessableEntity,
					`{"status":422,"error":"Unprocessable Entity","code":"no_default_payment_method"}`, blob)
				return
			}
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
			if s.subscriptionIndexStatus != 0 {
				code := s.subscriptionIndexStatus
				s.mu.Unlock()
				respond(w, r, &s.requests, &s.mu, code, "{}", nil)
				return
			}
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

// ---- finalized-invoice stub leg (#82 Task 4: readPurchaseInvoiceFees) ----

// purchaseInvoiceRec is one finalized invoice the stub answers on the
// invoices index (the status[]=finalized visible window).
type purchaseInvoiceRec struct {
	LagoID          string
	ExternalCust    string
	PaymentStatus   string
	SubscriptionFee *struct {
		Name        string
		AmountCents int64
	} // nil = no subscription line
	// FeeSubscriptionID is the fee line's external_subscription_id — the
	// F-5 fees→subscription location predicate. Empty defaults to the
	// PURCHASE subscription (the historical tests' intent); a Base-plan
	// invoice sets the "-sub" identity.
	FeeSubscriptionID string
	// CreatedAt is the invoice detail's created_at (ISO-8601). The INDEX
	// answer never carries it (the pinned runtime's shape) — it rides the
	// single-invoice read only, the explicit-comparison selection face
	// (OCR r2 lago_purchase.go:624).
	CreatedAt string
}

func purchaseInvoicesJSON(rows []purchaseInvoiceRec) string {
	// The pinned runtime's INDEX answer carries NO fees (t9 integration
	// evidence): fees only ride the single-invoice read below.
	out := make([]string, 0, len(rows))
	for _, inv := range rows {
		out = append(out, fmt.Sprintf(
			`{"lago_id":%q,"external_customer_id":%q,"status":"finalized","payment_status":%q,"invoice_type":"subscription","fees":[]}`,
			inv.LagoID, inv.ExternalCust, inv.PaymentStatus))
	}
	return `{"invoices":[` + strings.Join(out, ",") + `]}` // no meta.next_page: single page
}

// purchaseInvoiceDetailJSON answers the SINGLE-invoice read with the fee
// rows (the index never carries them). Each fee carries the v1.53
// external_subscription_id (the F-5 location predicate's real shape).
func purchaseInvoiceDetailJSON(inv purchaseInvoiceRec, purchaseSubscriptionID string) string {
	fees := "[]"
	if inv.SubscriptionFee != nil {
		subID := inv.FeeSubscriptionID
		if subID == "" {
			subID = purchaseSubscriptionID
		}
		fees = fmt.Sprintf(
			`[{"amount_cents":%d,"amount_currency":"USD","external_subscription_id":%q,"item":{"type":"subscription","name":%q,"code":"sub-fee"}}]`,
			inv.SubscriptionFee.AmountCents, subID, inv.SubscriptionFee.Name)
	}
	return fmt.Sprintf(
		`{"invoice":{"lago_id":%q,"external_customer_id":%q,"status":"finalized","payment_status":%q,"created_at":%q,"invoice_type":"subscription","fees":%s}}`,
		inv.LagoID, inv.ExternalCust, inv.PaymentStatus, inv.CreatedAt, fees)
}

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
	// OutboundAllowLoopback: the stub rides an httptest loopback origin —
	// the explicit dev-only S1 bypass (production never sets it).
	return NewLagoAdapter(Config{
		Provider: ProviderLago, BaseURL: srv.URL, APIKey: testAPIKey, Release: "v1.53.0",
		ProviderCustomerPrefix: "cus-dev", OutboundAllowLoopback: true,
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
	a := NewLagoAdapter(Config{Provider: ProviderLago, BaseURL: srv.URL, APIKey: testAPIKey, Release: "v1.53.0", OutboundAllowLoopback: true})
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
	a := NewLagoAdapter(Config{Provider: ProviderLago, BaseURL: srv.URL, APIKey: testAPIKey, Release: "v1.53.0", OutboundAllowLoopback: true})
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

// TestPurchaseRequestTimeoutBudgetCoversProviderChainAndSyncWait (R1-12):
// the command budget must cover the chain it actually runs — up to three
// provider round trips plus the bounded payment-method import wait plus the
// subscription-grade tail — and must shrink with a shortened pmSyncWait
// (call-time computation, not package-init constant).
func TestPurchaseRequestTimeoutBudgetCoversProviderChainAndSyncWait(t *testing.T) {
	oldWait := pmSyncWait
	pmSyncWait = 20 * time.Second
	t.Cleanup(func() { pmSyncWait = oldWait })
	budget := purchaseRequestTimeout()
	want := subscriptionRequestTimeout + 3*outboundProviderTimeout + 20*time.Second
	if budget != want {
		t.Fatalf("budget = %s, want the full chain sum %s", budget, want)
	}
	if budget < pmSyncWait+3*outboundProviderTimeout {
		t.Fatalf("budget %s must cover provider round trips AND the sync wait", budget)
	}
	// Self-consistency: shortening pmSyncWait shrinks the budget with it.
	pmSyncWait = 700 * time.Millisecond
	if got := purchaseRequestTimeout(); got != want-(20*time.Second-700*time.Millisecond) {
		t.Fatalf("budget must follow a shortened pmSyncWait, got %s", got)
	}
}

// TestLagoGatedCreateNoDefaultPaymentMethodIsRetryable (R1-12): a 422
// no_default_payment_method from the gated create is TRANSIENT import lag
// (the bounded sync wait above is the primary absorber). The replay's bound
// short-circuit does not re-wait, so escalating this to the terminal
// invalid-response verdict would permanently lose the sync window — it must
// classify as the retryable unreachable instead.
func TestLagoGatedCreateNoDefaultPaymentMethodIsRetryable(t *testing.T) {
	stub := newPurchaseStub()
	stub.mu.Lock()
	stub.rejectCreateNoDefaultPM = true
	stub.mu.Unlock()
	srv := stub.server(t)
	// (R1-12) With a provider key AND a payment-method token configured,
	// the 422 is TRANSIENT import lag — retryable unreachable, never a
	// terminal verdict.
	a := purchaseAdapterWithPrefix(t, srv)
	a.cfg.StripeAPIKey = "sk-test-shape-for-stub-only"
	a.cfg.StripePmToken = "pm_test_canary"
	// The provider customer exists (no outbound create in this test — the
	// seam-level function injection is the established pattern).
	a.deriveProviderCustomer = func(_ context.Context, externalCustomerID string) (string, providerCustomerSource, error) {
		return "cus-stub-" + externalCustomerID, providerCustomerAPI, nil
	}
	// (A-34) The 422 recovery leg re-drives attach+sync through the
	// injected seams — stubbed here (the customer is bound; the stub has
	// no provider endpoint to attach against).
	a.reAttachDefaultPM = func(context.Context, string, string) error { return nil }
	a.syncPaymentMethods = func(context.Context, string) error { return nil }
	_, err := a.SubmitCommand(context.Background(), purchaseCmd(32, "weknora-pro-v1", 9900))
	if !errors.Is(err, commercial.ErrPlatformUnreachable) {
		t.Fatalf("import-not-landed 422 must be retryable unreachable, got %v", err)
	}
	if errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("import-not-landed 422 must NOT be a terminal verdict, got %v", err)
	}
}

// TestLagoNoDefaultPmTerminalWhenUnconfigured (OCR r4): with NO provider
// key/payment-method token, the same 422 is STRUCTURAL — no import can
// ever land, so the adapter fails closed unconfigured instead of looping
// a retryable verdict on a wiring gap.
func TestLagoNoDefaultPmTerminalWhenUnconfigured(t *testing.T) {
	stub := newPurchaseStub()
	stub.mu.Lock()
	stub.rejectCreateNoDefaultPM = true
	stub.mu.Unlock()
	a := purchaseAdapterWithPrefix(t, stub.server(t))
	_, err := a.SubmitCommand(context.Background(), purchaseCmd(33, "weknora-pro-v1", 9900))
	if !errors.Is(err, commercial.ErrPlatformUnconfigured) {
		t.Fatalf("structural no_default_payment_method must fail closed unconfigured, got %v", err)
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

// ---- #82 Task 11: final-audit OCR open findings 7/6 (seam semantics) ----

// Finding 7: a found (replayed) purchase subscription in a TERMINAL state
// must fail closed — never a success receipt that mints a dead checkout.
func TestLagoPurchaseCreateReplayCanceledFailsClosed(t *testing.T) {
	stub := newPurchaseStub()
	stub.mu.Lock()
	stub.planAmount = map[string]int64{"weknora-pro-v1": 9900}
	ext := commercial.ExternalCustomerID(34)
	stub.subs = []purchaseSubRec{{
		ExternalID: commercial.ExternalPurchaseSubscriptionID(34), ExternalCustomer: ext,
		PlanCode: "weknora-pro-v1", AmountFen: 9900, Currency: "CNY", Status: "canceled",
	}}
	stub.mu.Unlock()
	a := purchaseAdapterWithPrefix(t, stub.server(t))
	_, err := a.SubmitCommand(context.Background(), purchaseCmd(34, "weknora-pro-v1", 9900))
	if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("a canceled purchase replay must fail closed, got %v", err)
	}
}

// Finding 7 (regression lock): the R1-V03 verbatim-replay semantics stay —
// an ACTIVE held purchase on the SAME plan still answers a success receipt.
func TestLagoPurchaseCreateReplayActiveStillReplays(t *testing.T) {
	stub := newPurchaseStub()
	stub.mu.Lock()
	stub.planAmount = map[string]int64{"weknora-pro-v1": 9900}
	ext := commercial.ExternalCustomerID(35)
	stub.subs = []purchaseSubRec{{
		ExternalID: commercial.ExternalPurchaseSubscriptionID(35), ExternalCustomer: ext,
		PlanCode: "weknora-pro-v1", AmountFen: 9900, Currency: "CNY", Status: "active",
	}}
	stub.mu.Unlock()
	a := purchaseAdapterWithPrefix(t, stub.server(t))
	if _, err := a.SubmitCommand(context.Background(), purchaseCmd(35, "weknora-pro-v1", 9900)); err != nil {
		t.Fatalf("an active same-plan replay must keep the R1-V03 receipt semantics: %v", err)
	}
}

// Finding 6: the PM-sync poll budget derives from the CALLER's remaining
// context deadline (min(pmSyncWait, ctx budget - guard)), so a request near
// its budget never nominally polls past the caller's cancellation.
func TestPaymentMethodSyncDeadlineDerivedFromCtx(t *testing.T) {
	stub := newPurchaseStub()
	srv := stub.server(t)
	a := purchaseAdapterWithPrefix(t, srv)
	a.cfg.StripePmToken = "pm_test_canary"
	a.deriveProviderCustomer = func(_ context.Context, externalCustomerID string) (string, providerCustomerSource, error) {
		return "cus-real-" + externalCustomerID, providerCustomerAPI, nil
	}
	stub.mu.Lock()
	stub.pmAlwaysEmpty = true // the poll can never succeed; only the budget ends it
	stub.mu.Unlock()
	oldWait, oldTick := pmSyncWait, pmSyncTick
	pmSyncWait, pmSyncTick = 30*time.Second, 50*time.Millisecond
	t.Cleanup(func() { pmSyncWait, pmSyncTick = oldWait, oldTick })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	start := time.Now()
	_, err := a.SubmitCommand(ctx, purchaseCmd(36, "weknora-pro-v1", 9900))
	elapsed := time.Since(start)
	if elapsed > 5*time.Second {
		t.Fatalf("the sync poll must yield to the caller budget in ~3s, took %s", elapsed)
	}
	if err == nil {
		t.Fatal("an always-empty import must eventually fail")
	}
}

// ---- #82 Task 4: the finalized-stage invoice fees read ----

// feeLine builds one subscription fee row for the stub's invoice records.
func feeLine(name string, cents int64) *struct {
	Name        string
	AmountCents int64
} {
	return &struct {
		Name        string
		AmountCents int64
	}{Name: name, AmountCents: cents}
}

func activePurchaseStub(t *testing.T, invoices []purchaseInvoiceRec) (*purchaseStub, *LagoAdapter) {
	t.Helper()
	stub := newPurchaseStub()
	stub.mu.Lock()
	stub.planAmount = map[string]int64{"weknora-pro-v1": 9900}
	ext := commercial.ExternalCustomerID(31)
	stub.subs = []purchaseSubRec{{
		ExternalID: commercial.ExternalPurchaseSubscriptionID(31), ExternalCustomer: ext,
		PlanCode: "weknora-pro-v1", AmountFen: 9900, Currency: "CNY", Status: "active",
	}}
	stub.invoices = invoices
	stub.mu.Unlock()
	return stub, purchaseAdapterWithPrefix(t, stub.server(t))
}

func TestLagoReadPurchaseInvoiceFeesMapsSubscriptionLine(t *testing.T) {
	invoices := []purchaseInvoiceRec{{
		LagoID: "inv_f1", ExternalCust: commercial.ExternalCustomerID(31),
		PaymentStatus: "succeeded", SubscriptionFee: feeLine("Pro plan", 1980), CreatedAt: "2026-08-15T00:00:00Z",
	}}
	stub, a := activePurchaseStub(t, invoices)
	snap, err := a.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 31})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Purchase.State != commercial.PurchaseStateActive {
		t.Fatalf("state = %q", snap.Purchase.State)
	}
	if len(snap.Purchase.InvoiceFees) != 1 {
		t.Fatalf("exactly one subscription fee line expected, got %+v", snap.Purchase.InvoiceFees)
	}
	fee := snap.Purchase.InvoiceFees[0]
	if fee.Kind != "subscription_fee" || fee.Name != "Pro plan" || fee.AmountFen != 1980 {
		t.Fatalf("fee mapping mismatch (closed port word + integer fen): %+v", fee)
	}
	if snap.Purchase.InvoicePaymentStatus != "succeeded" {
		t.Fatalf("invoice payment_status must ride the snapshot, got %q", snap.Purchase.InvoicePaymentStatus)
	}
	// The read must scope by identity AND the visible finalized status.
	stub.mu.Lock()
	raw := ""
	if len(stub.rawQueries) > 0 {
		raw = stub.rawQueries[len(stub.rawQueries)-1]
	}
	stub.mu.Unlock()
	if !strings.Contains(raw, "status%5B%5D=finalized") && !strings.Contains(raw, "status[]=finalized") {
		t.Fatalf("invoice index must pass explicit status[]=finalized, raw = %q", raw)
	}
	if !strings.Contains(raw, commercial.ExternalCustomerID(31)) {
		t.Fatalf("invoice index must scope by external_customer_id, raw = %q", raw)
	}
}

func TestLagoReadPurchaseInvoiceFeesZeroFinalizedIsEmpty(t *testing.T) {
	_, a := activePurchaseStub(t, nil)
	snap, err := a.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 31})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Purchase.InvoiceFees) != 0 || snap.Purchase.InvoicePaymentStatus != "" {
		t.Fatalf("zero finalized invoices = empty fees + empty payment status (awaiting semantics), got %+v", snap.Purchase)
	}
}

// (A-24 / F95) Two-or-more finalized purchase invoices is NOT a data
// anomaly: the authority's recurring billing issues a renewal invoice (the
// SAME external_subscription_id) at every monthly boundary of the purchase
// subscription (t02-duplicates lab evidence recorded one minutes after the
// 200), and a repurchase reuses the identity too. The read answers the
// NEWEST succeeded match and STOPS there (A-26): the snapshot keeps
// working a month after the first payment instead of permanently failing
// invalid_response, and the hot path does not walk the whole finalized
// history on every refresh.
func TestLagoReadPurchaseInvoiceFeesTwoFinalizedAnswersNewestSucceeded(t *testing.T) {
	// Index order is newest-first: the renewal (this month) precedes the
	// original gating invoice.
	invoices := []purchaseInvoiceRec{
		{LagoID: "inv_renewal", ExternalCust: commercial.ExternalCustomerID(31), PaymentStatus: "succeeded", SubscriptionFee: feeLine("Pro plan", 9900), CreatedAt: "2026-09-01T00:00:00Z"},
		{LagoID: "inv_gating", ExternalCust: commercial.ExternalCustomerID(31), PaymentStatus: "succeeded", SubscriptionFee: feeLine("Pro plan", 1980), CreatedAt: "2026-08-01T00:00:00Z"},
	}
	stub, a := activePurchaseStub(t, invoices)
	snap, err := a.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 31})
	if err != nil {
		t.Fatalf("a renewal invoice beside the settled gating invoice must NOT fail the snapshot (recurring billing is normal), got %v", err)
	}
	if len(snap.Purchase.InvoiceFees) != 1 || snap.Purchase.InvoiceFees[0].AmountFen != 9900 {
		t.Fatalf("the NEWEST succeeded purchase invoice must answer, got %+v", snap.Purchase.InvoiceFees)
	}
	if snap.Purchase.InvoicePaymentStatus != "succeeded" {
		t.Fatalf("payment status must ride the answered invoice, got %q", snap.Purchase.InvoicePaymentStatus)
	}
	// (r2:624) Every index row is detail-read: the newest selection ranks
	// the invoice detail's own created_at, so the walk cannot stop at the
	// first succeeded match (A-26's early exit left with the explicit
	// ordering — the N+1 shape stays disclosed, R-23).
	stub.mu.Lock()
	detailReads := 0
	for _, req := range stub.requests {
		if req.Method == http.MethodGet && strings.HasPrefix(req.Path, "/api/v1/invoices/") {
			detailReads++
		}
	}
	stub.mu.Unlock()
	if detailReads != 2 {
		t.Fatalf("both finalized invoices must be ranked by their own created_at, got %d detail reads", detailReads)
	}
}

// (A-24 defensive shape) A still-OPEN renewal invoice (finalized but
// payment_status pending — the fresh billing cycle before the subscription
// charge lands) must not shadow the settled gating fact: the read answers
// the newest SUCCEEDED match, falling back to the newest match only when
// no succeeded one exists.
func TestLagoReadPurchaseInvoiceFeesOpenRenewalDoesNotShadowGating(t *testing.T) {
	invoices := []purchaseInvoiceRec{
		{LagoID: "inv_renewal", ExternalCust: commercial.ExternalCustomerID(31), PaymentStatus: "pending", SubscriptionFee: feeLine("Pro plan", 9900), CreatedAt: "2026-09-01T00:00:00Z"},
		{LagoID: "inv_gating", ExternalCust: commercial.ExternalCustomerID(31), PaymentStatus: "succeeded", SubscriptionFee: feeLine("Pro plan", 1980), CreatedAt: "2026-08-01T00:00:00Z"},
	}
	_, a := activePurchaseStub(t, invoices)
	snap, err := a.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 31})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Purchase.InvoiceFees) != 1 || snap.Purchase.InvoiceFees[0].AmountFen != 1980 {
		t.Fatalf("the newest SUCCEEDED match (the settled gating invoice) must answer despite a newer pending renewal, got %+v", snap.Purchase.InvoiceFees)
	}
	if snap.Purchase.InvoicePaymentStatus != "succeeded" {
		t.Fatalf("payment status must ride the settled gating invoice, got %q", snap.Purchase.InvoicePaymentStatus)
	}
}

// (F-5 flow evidence) The Base plan's own finalized invoices (the lazy
// onboarding's 0-amount monthly line, "-sub" identity) sit on the SAME
// customer index — the purchase projection must locate the gating invoice
// by its fee lines' subscription identity, never by the index row count.
func TestLagoReadPurchaseInvoiceFeesIgnoresBaseSubscriptionInvoices(t *testing.T) {
	invoices := []purchaseInvoiceRec{
		{
			LagoID: "inv_base", ExternalCust: commercial.ExternalCustomerID(31),
			PaymentStatus: "succeeded", SubscriptionFee: feeLine("Base Plan", 0),
			FeeSubscriptionID: commercial.ExternalSubscriptionID(31), CreatedAt: "2026-09-01T00:00:00Z",
		},
		{
			LagoID: "inv_gating", ExternalCust: commercial.ExternalCustomerID(31),
			PaymentStatus: "succeeded", SubscriptionFee: feeLine("Pro plan", 1650), CreatedAt: "2026-08-01T00:00:00Z",
		},
	}
	stub, a := activePurchaseStub(t, invoices)
	snap, err := a.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 31})
	if err != nil {
		t.Fatalf("a Base finalized invoice beside the gating invoice must not collapse the purchase face, got %v", err)
	}
	if snap.Purchase.State != commercial.PurchaseStateActive {
		t.Fatalf("state = %q", snap.Purchase.State)
	}
	if len(snap.Purchase.InvoiceFees) != 1 ||
		snap.Purchase.InvoiceFees[0].Name != "Pro plan" || snap.Purchase.InvoiceFees[0].AmountFen != 1650 {
		t.Fatalf("exactly the PURCHASE subscription's fee line must answer, got %+v", snap.Purchase.InvoiceFees)
	}
	if snap.Purchase.InvoicePaymentStatus != "succeeded" {
		t.Fatalf("payment status must ride the gating invoice, got %q", snap.Purchase.InvoicePaymentStatus)
	}
	// Both invoices were detail-read (the fees→subscription predicate runs
	// on the single-invoice read — the index carries no fees).
	stub.mu.Lock()
	detailReads := 0
	for _, req := range stub.requests {
		if req.Method == http.MethodGet && strings.HasPrefix(req.Path, "/api/v1/invoices/") {
			detailReads++
		}
	}
	stub.mu.Unlock()
	if detailReads != 2 {
		t.Fatalf("every finalized invoice must be inspected once (fees→subscription), got %d detail reads", detailReads)
	}
}

func TestLagoReadPurchaseInvoiceFeesNoSubscriptionLineAnswersEmpty(t *testing.T) {
	invoices := []purchaseInvoiceRec{{
		LagoID: "inv_c", ExternalCust: commercial.ExternalCustomerID(31),
		PaymentStatus: "succeeded", SubscriptionFee: nil, CreatedAt: "2026-08-15T00:00:00Z",
	}}
	_, a := activePurchaseStub(t, invoices)
	snap, err := a.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 31})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Purchase.InvoiceFees) != 0 {
		t.Fatalf("no subscription fee line = empty fees (the D6' review refuses downstream), got %+v", snap.Purchase.InvoiceFees)
	}
}

// TestLagoInvoiceFacesClassifyTransient5xxUnreachable (A-25 / F96): both
// invoice faces (the finalized index and the single-invoice detail read)
// must split transient from definitive the way every other read path in
// this file already does — 429/5xx is the RETRYABLE unreachable, never the
// definitive invalid_response that would park a settle replay in attention.
func TestLagoInvoiceFacesClassifyTransient5xxUnreachable(t *testing.T) {
	invoices := []purchaseInvoiceRec{
		{LagoID: "inv_a", ExternalCust: commercial.ExternalCustomerID(31), PaymentStatus: "succeeded", SubscriptionFee: feeLine("Pro plan", 1980), CreatedAt: "2026-08-15T00:00:00Z"},
	}
	for _, tc := range []struct {
		name   string
		inject func(*purchaseStub)
	}{
		{"index 502", func(s *purchaseStub) { s.invoiceIndexStatus = 502 }},
		{"index 429", func(s *purchaseStub) { s.invoiceIndexStatus = 429 }},
		{"detail 503", func(s *purchaseStub) { s.invoiceDetailStatus = 503 }},
		{"detail 429", func(s *purchaseStub) { s.invoiceDetailStatus = 429 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub, a := activePurchaseStub(t, invoices)
			stub.mu.Lock()
			tc.inject(stub)
			stub.mu.Unlock()
			_, err := a.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 31})
			if !errors.Is(err, commercial.ErrPlatformUnreachable) {
				t.Fatalf("a transient %s must classify unreachable (retryable), got %v", tc.name, err)
			}
			if errors.Is(err, commercial.ErrPlatformInvalidResponse) {
				t.Fatalf("a transient %s must NOT be the definitive sentinel, got %v", tc.name, err)
			}
		})
	}
}

// TestLagoInvoiceFacesKeepDefinitive4xxInvalidResponse (A-25): a
// non-transient non-200 (e.g. 404 on the index) keeps the definitive
// invalid_response classification — only 429/5xx moved.
func TestLagoInvoiceFacesKeepDefinitive4xxInvalidResponse(t *testing.T) {
	stub, a := activePurchaseStub(t, nil)
	stub.mu.Lock()
	stub.invoiceIndexStatus = http.StatusNotFound
	stub.mu.Unlock()
	_, err := a.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 31})
	if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("a definitive 404 index answer must stay invalid_response, got %v", err)
	}
}

// ---- #82 Task 12 (OCR r2): explicit created_at ordering + transient reads ----

// TestLagoReadPurchaseInvoiceFeesExplicitCreatedAtOrdering (r2:624
// medium, correctness half): the NEWEST selection must compare the invoice
// detail's own created_at, never the index row order. Here the index
// answers OLDEST-FIRST (the opposite of the assumed newest-first
// convention): trusting the row order would lock in the stale 1980 gating
// invoice; the explicit comparison must answer the newer 9900 renewal.
func TestLagoReadPurchaseInvoiceFeesExplicitCreatedAtOrdering(t *testing.T) {
	invoices := []purchaseInvoiceRec{
		{LagoID: "inv_old", ExternalCust: commercial.ExternalCustomerID(31),
			PaymentStatus: "succeeded", SubscriptionFee: feeLine("Pro plan", 1980), CreatedAt: "2026-08-01T00:00:00Z"},
		{LagoID: "inv_new", ExternalCust: commercial.ExternalCustomerID(31),
			PaymentStatus: "succeeded", SubscriptionFee: feeLine("Pro plan", 9900), CreatedAt: "2026-09-01T00:00:00Z"},
	}
	_, a := activePurchaseStub(t, invoices)
	snap, err := a.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 31})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Purchase.InvoiceFees) != 1 || snap.Purchase.InvoiceFees[0].AmountFen != 9900 {
		t.Fatalf("the created_at-NEWEST invoice must answer regardless of index row order, got %+v", snap.Purchase.InvoiceFees)
	}
}

// TestLagoReadPurchaseInvoiceFeesUnparsableCreatedAtFailsClosed (D13
// companion): a matching invoice whose created_at cannot be parsed is a
// malformed authority shape — the selection cannot rank it, so the read
// fails closed invalid_response instead of silently guessing (the
// unparsable row may be the newest).
func TestLagoReadPurchaseInvoiceFeesUnparsableCreatedAtFailsClosed(t *testing.T) {
	invoices := []purchaseInvoiceRec{
		{LagoID: "inv_bad", ExternalCust: commercial.ExternalCustomerID(31),
			PaymentStatus: "succeeded", SubscriptionFee: feeLine("Pro plan", 1980), CreatedAt: "not-a-timestamp"},
	}
	_, a := activePurchaseStub(t, invoices)
	_, err := a.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 31})
	if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("an unparsable invoice created_at must fail closed invalid_response, got %v", err)
	}
}

// TestLagoSubscriptionRead429Unreachable (D13 / r2:688 medium): a 429 on
// the subscription identity index is a TRANSIENT authority condition —
// ErrPlatformUnreachable, never the definitive invalid_response. The
// fulfiller's error taxonomy keys off this split: unreachable keeps a paid
// order pending for retry; invalid_response mints it into terminal
// attention. A rate-limited snapshot read must never do the latter.
func TestLagoSubscriptionRead429Unreachable(t *testing.T) {
	stub, a := activePurchaseStub(t, nil)
	stub.mu.Lock()
	stub.subscriptionIndexStatus = http.StatusTooManyRequests
	stub.mu.Unlock()
	_, err := a.ReadSnapshot(context.Background(), commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: 31})
	if !errors.Is(err, commercial.ErrPlatformUnreachable) {
		t.Fatalf("a 429 subscription index must classify unreachable (retryable), got %v", err)
	}
	if errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("a transient 429 must NOT be the definitive sentinel, got %v", err)
	}
}
