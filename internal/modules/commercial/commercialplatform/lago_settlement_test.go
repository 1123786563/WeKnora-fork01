package commercialplatform

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	commercial "github.com/Tencent/WeKnora/internal/modules/commercial"
)

// ---- #82 Task 5: settle_purchase_payment on the Lago adapter (D2') ----
//
// The settle rail drives the PROVIDER side only (Stripe rails); activation
// is finalized by the Lago built-in webhook chain (t11 P-A..P-E). The stub
// below is a scripted Stripe state machine; the Lago side reuses the
// purchaseStub (identity index + customer binding read).

// stripeIntentRec is one PaymentIntent row the Stripe stub answers on the
// customer list.
type stripeIntentRec struct {
	ID         string
	Customer   string
	Status     string
	Created    int64
	LagoInvID  string // metadata.lago_invoice_id ("" = absent)
}

// settleReq records one settle-rail call (path + the Idempotency-Key when
// present — the charge-identity assertion face).
type settleReq struct {
	Method string
	Path   string
	Idem   string
}

type settleStripeStub struct {
	mu       sync.Mutex
	requests []settleReq
	intents  []stripeIntentRec
	// scripted responses (0 = default success shapes)
	attachStatus   int
	defaultStatus  int
	updateStatus   int
	confirmStatus  int
	confirmIntentStatus string // the status field in the confirm body ("" = succeeded)
	mux            *http.ServeMux
}

func newSettleStripeStub() *settleStripeStub {
	s := &settleStripeStub{}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/payment_intents", func(w http.ResponseWriter, r *http.Request) {
		// GET list (the P-A locator) — POST on the collection is not used.
		s.mu.Lock()
		s.requests = append(s.requests, settleReq{Method: r.Method, Path: r.URL.Path})
		cust := r.URL.Query().Get("customer")
		out := []string{}
		for _, it := range s.intents {
			if it.Customer == cust {
				out = append(out, fmt.Sprintf(
					`{"id":%q,"status":%q,"created":%d,"metadata":{"lago_invoice_id":%q}}`,
					it.ID, it.Status, it.Created, it.LagoInvID))
			}
		}
		s.mu.Unlock()
		body := `{"data":[` + strings.Join(out, ",") + `]}`
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	})
	mux.HandleFunc("/v1/payment_methods/", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests = append(s.requests, settleReq{Method: r.Method, Path: r.URL.Path})
		status := s.attachStatus
		s.mu.Unlock()
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"id":"pm_clone_settle"}`))
	})
	mux.HandleFunc("/v1/customers/", func(w http.ResponseWriter, r *http.Request) {
		// POST /v1/customers/{id} — the default-payment-method update.
		s.mu.Lock()
		s.requests = append(s.requests, settleReq{Method: r.Method, Path: r.URL.Path})
		status := s.defaultStatus
		s.mu.Unlock()
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"id":"cus_stripe_1"}`))
	})
	mux.HandleFunc("/v1/payment_intents/", func(w http.ResponseWriter, r *http.Request) {
		// POST /v1/payment_intents/{id} (update payment_method) and
		// POST /v1/payment_intents/{id}/confirm.
		s.mu.Lock()
		idempotencyKey := r.Header.Get("Idempotency-Key")
		s.requests = append(s.requests, settleReq{Method: r.Method, Path: r.URL.Path, Idem: idempotencyKey})
		confirmStatus := s.confirmStatus
		confirmIntent := s.confirmIntentStatus
		updateStatus := s.updateStatus
		s.mu.Unlock()
		if strings.HasSuffix(r.URL.Path, "/confirm") {
			status := confirmStatus
			if status == 0 {
				status = http.StatusOK
			}
			intentStatus := confirmIntent
			if intentStatus == "" {
				intentStatus = "succeeded"
			}
			w.WriteHeader(status)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"id":%q,"status":%q}`, strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/payment_intents/"), "/confirm"), intentStatus)))
			return
		}
		status := updateStatus
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"id":"pi_1"}`))
	})
	s.mux = mux
	return s
}

func (s *settleStripeStub) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func (s *settleStripeStub) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	return srv
}

func (s *settleStripeStub) requestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}

func (s *settleStripeStub) paths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.requests))
	for _, req := range s.requests {
		out = append(out, req.Method+" "+req.Path)
	}
	return out
}

// settleHarness wires a Lago purchaseStub (subscription truth + customer
// binding) with a Stripe stub (the settle rails) into one adapter.
type settleHarness struct {
	lago   *purchaseStub
	stripe *settleStripeStub
	adapter *LagoAdapter
}

func newSettleHarness(t *testing.T, subStatus string, intents []stripeIntentRec) *settleHarness {
	t.Helper()
	lago := newPurchaseStub()
	ext := commercial.ExternalCustomerID(41)
	lago.mu.Lock()
	lago.planAmount = map[string]int64{"weknora-pro-v1": 9900}
	lago.subs = []purchaseSubRec{{
		ExternalID: commercial.ExternalPurchaseSubscriptionID(41), ExternalCustomer: ext,
		PlanCode: "weknora-pro-v1", AmountFen: 9900, Currency: "CNY", Status: subStatus,
	}}
	lago.customer[ext] = map[string]any{
		"external_id": ext, "name": "settle tenant",
		"billing_configuration": map[string]any{
			"provider_customer_id": "cus_stripe_1",
			"payment_provider_code": "weknora-stripe",
		},
	}
	lago.mu.Unlock()
	stripe := newSettleStripeStub()
	stripe.intents = intents
	lagoSrv := lago.server(t)
	stripeSrv := stripe.server(t)
	adapter := NewLagoAdapter(Config{
		Provider: ProviderLago, BaseURL: lagoSrv.URL, APIKey: testAPIKey, Release: "v1.53.0",
		StripeAPIKey: "sk-test-shape-for-stub-only", StripeAPIBase: stripeSrv.URL,
		StripeSettlePmToken: "pm_settle", OutboundAllowLoopback: true,
	})
	return &settleHarness{lago: lago, stripe: stripe, adapter: adapter}
}

func settleCmd(tenant uint64, txn string) commercial.Command {
	return commercial.Command{
		Kind:  commercial.CommandKindSettlePurchasePayment,
		Key:   commercial.SettlePurchasePaymentCommandKey(commercial.ExternalPurchaseSubscriptionID(tenant), txn),
		Actor: "test", Reason: "settle",
		Payload: commercial.SettlePurchasePaymentPayload{
			TenantID: tenant, ExternalCustomerID: commercial.ExternalCustomerID(tenant),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(tenant),
			PlanCode: "weknora-pro-v1", ChannelTransaction: txn, AmountFen: 9900, Currency: commercial.CurrencyCNY,
		},
	}
}

func TestLagoSettleAlreadyActiveNoOutbound(t *testing.T) { // Review-Focus 2
	h := newSettleHarness(t, "active", nil)
	receipt, err := h.adapter.SubmitCommand(context.Background(), settleCmd(41, "txn-1"))
	if err != nil {
		t.Fatalf("active settle must be an idempotent no-op, got %v", err)
	}
	if receipt.ExternalID != commercial.ExternalPurchaseSubscriptionID(41) {
		t.Fatalf("receipt identity = %q", receipt.ExternalID)
	}
	if n := h.stripe.requestCount(); n != 0 {
		t.Fatalf("already-active settle must make ZERO provider calls, got %d: %v", n, h.stripe.paths())
	}
}

func TestLagoSettleDrivesProviderRails(t *testing.T) {
	intents := []stripeIntentRec{{
		ID: "pi_1", Customer: "cus_stripe_1", Status: "requires_payment_method",
		Created: 1000, LagoInvID: "inv_gating",
	}}
	h := newSettleHarness(t, "incomplete", intents)
	cmd := settleCmd(41, "txn-1")
	receipt, err := h.adapter.SubmitCommand(context.Background(), cmd)
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if receipt.Key != cmd.Key {
		t.Fatalf("receipt key = %q", receipt.Key)
	}
	paths := h.stripe.paths()
	want := []string{
		"GET /v1/payment_intents",
		"POST /v1/payment_methods/pm_settle/attach",
		"POST /v1/customers/cus_stripe_1",
		"POST /v1/payment_intents/pi_1",
		"POST /v1/payment_intents/pi_1/confirm",
	}
	if len(paths) != len(want) {
		t.Fatalf("call sequence mismatch:\n got %v\nwant %v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("call %d mismatch: got %q want %q (all: %v)", i, paths[i], want[i], paths)
		}
	}
	// The charge-bearing calls carry the command key as the provider
	// idempotency identity (D2' task-level contract).
	h.stripe.mu.Lock()
	defer h.stripe.mu.Unlock()
	idems := map[string]string{}
	for _, req := range h.stripe.requests {
		if req.Idem != "" {
			idems[req.Method+" "+req.Path] = req.Idem
		}
	}
	for _, path := range []string{"POST /v1/payment_intents/pi_1", "POST /v1/payment_intents/pi_1/confirm"} {
		if idems[path] != cmd.Key {
			t.Fatalf("%s must carry Idempotency-Key == cmd key, got %q", path, idems[path])
		}
	}
}

func TestLagoSettlePicksLatestUnsettledIntent(t *testing.T) {
	intents := []stripeIntentRec{
		{ID: "pi_old", Customer: "cus_stripe_1", Status: "requires_payment_method", Created: 900, LagoInvID: "inv_old"},
		{ID: "pi_new", Customer: "cus_stripe_1", Status: "requires_action", Created: 1000, LagoInvID: "inv_new"},
	}
	h := newSettleHarness(t, "incomplete", intents)
	if _, err := h.adapter.SubmitCommand(context.Background(), settleCmd(41, "txn-1")); err != nil {
		t.Fatal(err)
	}
	paths := h.stripe.paths()
	for _, p := range paths {
		if strings.Contains(p, "pi_old") {
			t.Fatalf("settle must drive the LATEST unsettled intent only, drove %v", paths)
		}
	}
	if len(paths) != 5 {
		t.Fatalf("unexpected call count: %v", paths)
	}
}

func TestLagoSettleAmbiguousIntentsFailClosed(t *testing.T) {
	intents := []stripeIntentRec{
		{ID: "pi_a", Customer: "cus_stripe_1", Status: "requires_payment_method", Created: 1000, LagoInvID: "inv_a"},
		{ID: "pi_b", Customer: "cus_stripe_1", Status: "requires_payment_method", Created: 1000, LagoInvID: "inv_b"},
	}
	h := newSettleHarness(t, "incomplete", intents)
	_, err := h.adapter.SubmitCommand(context.Background(), settleCmd(41, "txn-1"))
	if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("ambiguous candidates must fail closed, got %v", err)
	}
	for _, p := range h.stripe.paths() {
		if strings.Contains(p, "attach") || strings.Contains(p, "confirm") {
			t.Fatalf("fail-closed settle must never charge, drove %v", h.stripe.paths())
		}
	}
}

func TestLagoSettleFailsClosedWithoutSettlePm(t *testing.T) {
	intents := []stripeIntentRec{{
		ID: "pi_1", Customer: "cus_stripe_1", Status: "requires_payment_method", Created: 1000, LagoInvID: "inv_gating",
	}}
	h := newSettleHarness(t, "incomplete", intents)
	h.adapter.cfg.StripeSettlePmToken = ""
	_, err := h.adapter.SubmitCommand(context.Background(), settleCmd(41, "txn-1"))
	if !errors.Is(err, commercial.ErrPlatformUnconfigured) {
		t.Fatalf("missing settle pm must fail closed unconfigured, got %v", err)
	}
	if n := h.stripe.requestCount(); n != 1 { // the locator GET is allowed; nothing past it
		t.Fatalf("only the locator read may fire without a settle pm, got %v", h.stripe.paths())
	}
}

func TestLagoSettleNoStuckIntentInvalidResponse(t *testing.T) {
	cases := [][]stripeIntentRec{
		{}, // empty
		{{ID: "pi_done", Customer: "cus_stripe_1", Status: "succeeded", Created: 1000, LagoInvID: "inv_x"}},    // terminal only
		{{ID: "pi_metaless", Customer: "cus_stripe_1", Status: "requires_payment_method", Created: 1000}},      // no invoice metadata
	}
	for i, intents := range cases {
		h := newSettleHarness(t, "incomplete", intents)
		_, err := h.adapter.SubmitCommand(context.Background(), settleCmd(41, "txn-1"))
		if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
			t.Fatalf("case %d: no stuck intent must fail closed invalid_response, got %v", i, err)
		}
		if n := h.stripe.requestCount(); n > 1 {
			t.Fatalf("case %d: no charge may fire, got %v", i, h.stripe.paths())
		}
	}
}

func TestLagoSettleClassifiesStripe429Unreachable(t *testing.T) {
	intents := []stripeIntentRec{{
		ID: "pi_1", Customer: "cus_stripe_1", Status: "requires_payment_method", Created: 1000, LagoInvID: "inv_gating",
	}}
	h := newSettleHarness(t, "incomplete", intents)
	h.stripe.mu.Lock()
	h.stripe.confirmStatus = http.StatusTooManyRequests
	h.stripe.mu.Unlock()
	_, err := h.adapter.SubmitCommand(context.Background(), settleCmd(41, "txn-1"))
	if !errors.Is(err, commercial.ErrPlatformUnreachable) {
		t.Fatalf("provider 429 must classify unreachable (retryable), got %v", err)
	}
}
