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

	commercial "github.com/Tencent/WeKnora/internal/commercial"
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
	ID        string
	Customer  string
	Status    string
	Created   int64
	LagoInvID string // metadata.lago_invoice_id ("" = absent)
	Amount    int64  // the charge face; 0 renders as 9900 (the frozen quote every existing settle test uses — A-16 mismatch tests set an explicit value)
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
	attachStatus        int
	defaultStatus       int
	updateStatus        int
	confirmStatus       int
	confirmIntentStatus string // the status field in the confirm body ("" = succeeded)
	// page1Count > 0 splits the intent list into pages of that size with
	// has_more + starting_after cursor semantics (A-17 pagination tests).
	page1Count int
	// hijackListBody answers the intent list with a TRUNCATED body: 2xx
	// headers arrive, the body dies mid-stream (A-19).
	hijackListBody bool
	// oversizeListBody answers the intent list with a body beyond the
	// provider body cap (A-19).
	oversizeListBody bool
	mux              *http.ServeMux
}

func newSettleStripeStub() *settleStripeStub {
	s := &settleStripeStub{}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/payment_intents", func(w http.ResponseWriter, r *http.Request) {
		// GET list (the P-A locator) — POST on the collection is not used.
		s.mu.Lock()
		s.requests = append(s.requests, settleReq{Method: r.Method, Path: r.URL.Path})
		cust := r.URL.Query().Get("customer")
		pageSize, hijack, oversize := s.page1Count, s.hijackListBody, s.oversizeListBody
		rows := make([]stripeIntentRec, 0, len(s.intents))
		for _, it := range s.intents {
			if it.Customer == cust {
				rows = append(rows, it)
			}
		}
		s.mu.Unlock()
		if hijack {
			// (A-19) 2xx headers, then the connection dies mid-body.
			if hj, ok := w.(http.Hijacker); ok {
				if conn, _, err := hj.Hijack(); err == nil {
					_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 400\r\n\r\n{\"data\":["))
					_ = conn.Close()
				}
			}
			return
		}
		if oversize {
			w.WriteHeader(http.StatusOK)
			chunk := strings.Repeat("x", 4096)
			for i := 0; i < 17; i++ { // 17*4096 = 69632 > the 64 KiB cap
				_, _ = w.Write([]byte(chunk))
			}
			return
		}
		// (A-17) starting_after cursor pagination with has_more, mirroring
		// the provider's real list contract.
		start := 0
		if cursor := r.URL.Query().Get("starting_after"); cursor != "" {
			for i, it := range rows {
				if it.ID == cursor {
					start = i + 1
					break
				}
			}
		}
		end := len(rows)
		if pageSize > 0 && start+pageSize < end {
			end = start + pageSize
		}
		out := []string{}
		for _, it := range rows[start:end] {
			amount := it.Amount
			if amount == 0 {
				amount = 9900 // the frozen quote face every existing settle test settles
			}
			if amount < 0 {
				amount = 0 // explicit zero-amount intent (negative = sentinel in the rec)
			}
			out = append(out, fmt.Sprintf(
				`{"id":%q,"status":%q,"created":%d,"amount":%d,"metadata":{"lago_invoice_id":%q}}`,
				it.ID, it.Status, it.Created, amount, it.LagoInvID))
		}
		hasMore := end < len(rows)
		body := `{"data":[` + strings.Join(out, ",") + `],"has_more":` + fmt.Sprintf("%v", hasMore) + `}`
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	})
	mux.HandleFunc("/v1/payment_methods/", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests = append(s.requests, settleReq{Method: r.Method, Path: r.URL.Path, Idem: r.Header.Get("Idempotency-Key")})
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
		s.requests = append(s.requests, settleReq{Method: r.Method, Path: r.URL.Path, Idem: r.Header.Get("Idempotency-Key")})
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
	lago    *purchaseStub
	stripe  *settleStripeStub
	adapter *LagoAdapter
}

func newSettleHarness(t *testing.T, subStatus string, intents []stripeIntentRec) *settleHarness {
	return newSettleHarnessForTenant(t, 41, subStatus, intents)
}

func newSettleHarnessForTenant(t *testing.T, tenant uint64, subStatus string, intents []stripeIntentRec) *settleHarness {
	t.Helper()
	lago := newPurchaseStub()
	ext := commercial.ExternalCustomerID(tenant)
	lago.mu.Lock()
	lago.planAmount = map[string]int64{"weknora-pro-v1": 9900, "weknora-contract-v1": 9900}
	lago.subs = []purchaseSubRec{{
		ExternalID: commercial.ExternalPurchaseSubscriptionID(tenant), ExternalCustomer: ext,
		PlanCode: "weknora-pro-v1", AmountFen: 9900, Currency: "CNY", Status: subStatus,
	}}
	lago.customer[ext] = map[string]any{
		"external_id": ext, "name": "settle tenant",
		"billing_configuration": map[string]any{
			"provider_customer_id":  "cus_stripe_1",
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
			PlanCode:                       "weknora-pro-v1", ChannelTransaction: txn, AmountFen: 9900, Currency: commercial.CurrencyCNY,
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
	want := []string{ // D10: no POST /v1/customers/{id} — the default pm is never rewritten
		"GET /v1/payment_intents",
		"POST /v1/payment_methods/pm_settle/attach",
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
	// The idempotency identities DERIVE from the command key with
	// per-endpoint suffixes (Stripe binds a key to one endpoint forever —
	// one key across update+confirm answers 400, t9 evidence).
	for _, path := range []string{"POST /v1/payment_intents/pi_1", "POST /v1/payment_intents/pi_1/confirm"} {
		suffix := ":update"
		if strings.HasSuffix(path, "/confirm") {
			suffix = ":confirm"
		}
		if idems[path] != cmd.Key+suffix {
			t.Fatalf("%s must carry Idempotency-Key == cmd key%s, got %q", path, suffix, idems[path])
		}
	}
}

// (D9) The latest-created ranking only ever runs inside a candidate set
// that spans ONE invoice: two unsettled intents for the SAME gating
// invoice (a provider-side retry shape) rank by created, and the drive
// touches only the newest. Candidates spanning DIFFERENT invoices fail
// closed — TestLagoSettleMultipleInvoiceCandidatesFailClosed.
func TestLagoSettlePicksLatestUnsettledIntent(t *testing.T) {
	intents := []stripeIntentRec{
		{ID: "pi_old", Customer: "cus_stripe_1", Status: "requires_payment_method", Created: 900, LagoInvID: "inv_gating"},
		{ID: "pi_new", Customer: "cus_stripe_1", Status: "requires_action", Created: 1000, LagoInvID: "inv_gating"},
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
	if len(paths) != 4 { // D10: locator + attach + update + confirm
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
	if n := h.stripe.requestCount(); n != 0 { // the config gate fires BEFORE the locator read (r2:166)
		t.Fatalf("an unconfigured settle must make ZERO provider calls, got %v", h.stripe.paths())
	}
}

func TestLagoSettleNoStuckIntentInvalidResponse(t *testing.T) {
	cases := [][]stripeIntentRec{
		{}, // empty
		{{ID: "pi_metaless", Customer: "cus_stripe_1", Status: "requires_payment_method", Created: 1000}}, // no invoice metadata
	}
	for i, intents := range cases {
		h := newSettleHarness(t, "incomplete", intents)
		_, err := h.adapter.SubmitCommand(context.Background(), settleCmd(41, "txn-1"))
		if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
			t.Fatalf("case %d: no stuck intent must fail closed invalid_response, got %v", i, err)
		}
		// Reads only: the single gating list (unsettled candidates AND the
		// settled-invoice window in one read — F-1). No charge-bearing
		// call may fire.
		for _, path := range h.stripe.paths() {
			if strings.Contains(path, "attach") || strings.Contains(path, "confirm") ||
				strings.Contains(path, "POST /v1/payment_intents/pi") {
				t.Fatalf("case %d: no charge may fire, got %v", i, h.stripe.paths())
			}
		}
	}
}

// TestLagoSettleEmptyCandidatesFailClosedEvenWithSettledHistory
// (OCR84-R1-01 high, rewrites the former settled-window replay contract):
// with NO unsettled candidate, the settled-invoice map alone proves nothing
// about THIS order's gate — it collects invoice ids from ANY of the
// customer's succeeded intents (a previous purchase's residue rides in it
// beside the t10 settle-vs-finalize window's own, and the payload carries
// no invoice identity to tell them apart). The empty-candidate branch must
// therefore answer the definitive sentinel (fail closed, zero writes), NOT
// mint an idempotent receipt on a bare len(settledInvoices) > 0 — a fake
// receipt there masked the "gate never created / canceled" data anomaly as
// a silent no-op the fulfiller replayed until D7 exhausted. The honest
// settle-vs-finalize replay recovery is the pending event's next pass (the
// authority activates via the webhook chain and the grant overwrites the
// transient attention marker), never an unprovable receipt.
func TestLagoSettleEmptyCandidatesFailClosedEvenWithSettledHistory(t *testing.T) {
	intents := []stripeIntentRec{{
		ID: "pi_done", Customer: "cus_stripe_1", Status: "succeeded", Created: 1000, LagoInvID: "inv_x",
	}}
	h := newSettleHarness(t, "incomplete", intents)
	_, err := h.adapter.SubmitCommand(context.Background(), settleCmd(41, "txn-1"))
	if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("empty candidates with settled history must fail closed invalid_response, got %v", err)
	}
	paths := h.stripe.paths()
	if len(paths) != 1 || paths[0] != "GET /v1/payment_intents" {
		t.Fatalf("the empty-candidate branch must be ONE locator read and zero writes, got %v", paths)
	}
}

// TestLagoSettleDrivesRequiresConfirmationReplayShape (OCR84-R1-02 high):
// a settle that died between the pm update and the confirm leaves the
// gating intent in requires_confirmation (pm attached, charge missing). The
// replay must treat it as an unsettled candidate — the idempotent pm update
// is a no-op and the confirm is the missing step — instead of skipping it
// into the empty-candidate fail-closed branch where the deterministic
// update/confirm keys would never run again.
func TestLagoSettleDrivesRequiresConfirmationReplayShape(t *testing.T) {
	intents := []stripeIntentRec{{
		ID: "pi_half", Customer: "cus_stripe_1", Status: "requires_confirmation", Created: 1000, LagoInvID: "inv_gating",
	}}
	h := newSettleHarness(t, "incomplete", intents)
	if _, err := h.adapter.SubmitCommand(context.Background(), settleCmd(41, "txn-rc")); err != nil {
		t.Fatalf("a requires_confirmation gating intent must be driven to confirm, got %v", err)
	}
	paths := h.stripe.paths()
	want := []string{
		"GET /v1/payment_intents",
		"POST /v1/payment_methods/pm_settle/attach",
		"POST /v1/payment_intents/pi_half",
		"POST /v1/payment_intents/pi_half/confirm",
	}
	if len(paths) != len(want) {
		t.Fatalf("call sequence mismatch:\n got %v\nwant %v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("call %d mismatch: got %q want %q (all: %v)", i, paths[i], want[i], paths)
		}
	}
}

// (F-1 flow evidence) The residual sibling: a failed 3DS confirm leaves an
// unsettled PaymentIntent for the SAME invoice next to the one this
// command already drove to succeeded. The replay must answer the
// idempotent receipt — never re-key "<cmd.Key>:update" onto the sibling
// (Stripe keys are endpoint-bound → 400 idempotency_error → mislabeled
// attention) and never confirm the sibling (a real second charge).
func TestLagoSettleResidualSiblingIntentReplaysIdempotently(t *testing.T) {
	intents := []stripeIntentRec{
		{ID: "pi_sibling", Customer: "cus_stripe_1", Status: "requires_payment_method", Created: 900, LagoInvID: "inv_gating"},
		{ID: "pi_done", Customer: "cus_stripe_1", Status: "succeeded", Created: 1000, LagoInvID: "inv_gating"},
	}
	h := newSettleHarness(t, "incomplete", intents)
	receipt, err := h.adapter.SubmitCommand(context.Background(), settleCmd(41, "txn-1"))
	if err != nil {
		t.Fatalf("a residual sibling of a settled invoice must replay idempotently, got %v", err)
	}
	if receipt.ExternalID != commercial.ExternalPurchaseSubscriptionID(41) {
		t.Fatalf("receipt identity = %q", receipt.ExternalID)
	}
	paths := h.stripe.paths()
	if len(paths) != 1 || paths[0] != "GET /v1/payment_intents" {
		t.Fatalf("the residual-sibling replay must be READS only (one locator list), got %v", paths)
	}
}

// The invoice-bound idempotency must not over-trigger: an unsettled
// candidate gating a DIFFERENT invoice than every succeeded one is a live
// gate — the settle drives it.
func TestLagoSettleDrivesWhenSucceededInvoiceDiffers(t *testing.T) {
	intents := []stripeIntentRec{
		{ID: "pi_gating", Customer: "cus_stripe_1", Status: "requires_payment_method", Created: 900, LagoInvID: "inv_current"},
		{ID: "pi_past", Customer: "cus_stripe_1", Status: "succeeded", Created: 1000, LagoInvID: "inv_past"},
	}
	h := newSettleHarness(t, "incomplete", intents)
	if _, err := h.adapter.SubmitCommand(context.Background(), settleCmd(41, "txn-1")); err != nil {
		t.Fatal(err)
	}
	paths := h.stripe.paths()
	drove := false
	for _, p := range paths {
		if strings.Contains(p, "pi_gating") {
			drove = true
		}
	}
	if !drove || len(paths) != 4 {
		t.Fatalf("the current gating intent must be driven (4 calls, D10), got %v", paths)
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

// TestLagoSettleAmountGuardFailsClosed (A-16 / F84): the resolved gating
// intent's amount must stay WITHIN the frozen quote face (1..payload, the
// proration-aware guard — the live 82flow stack gates a 9900-fen plan on a
// 1650 mid-month prorated intent). An intent LARGER than the frozen quote
// (a wrong-object shape) or a zero amount fails closed BEFORE any
// charge-bearing call.
func TestLagoSettleAmountGuardFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		amount int64
	}{
		{"larger than frozen quote", 9999},
		{"zero amount", -1}, // the stub renders a negative sentinel as amount 0
	} {
		t.Run(tc.name, func(t *testing.T) {
			intents := []stripeIntentRec{
				{ID: "pi_foreign", Customer: "cus_stripe_1", Status: "requires_payment_method",
					Created: 2000, LagoInvID: "inv_other", Amount: tc.amount},
			}
			h := newSettleHarness(t, "incomplete", intents)
			_, err := h.adapter.SubmitCommand(context.Background(), settleCmd(41, "txn-amt"))
			if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
				t.Fatalf("an intent outside the frozen quote face must fail closed invalid_response, got %v", err)
			}
			if !strings.Contains(err.Error(), "amount") {
				t.Fatalf("the failure must name the amount guard, got %v", err)
			}
			// Fail-closed happens BEFORE the attach/update/confirm rails:
			// only the list call (plus the authority reads) happened —
			// never a charge.
			for _, p := range h.stripe.paths() {
				if strings.Contains(p, "attach") || strings.Contains(p, "/confirm") ||
					strings.HasPrefix(p, "POST /v1/payment_intents/") {
					t.Fatalf("an out-of-face intent must never reach the charge rails, saw %q (all: %v)", p, h.stripe.paths())
				}
			}
		})
	}
}

// TestLagoSettleProratedIntentStillSettles (A-16): the prorated gating
// intent (amount BELOW the frozen quote — the live-stack 1650-of-9900
// shape) settles normally; the guard never blocks a legitimate proration.
func TestLagoSettleProratedIntentStillSettles(t *testing.T) {
	intents := []stripeIntentRec{
		{ID: "pi_prorated", Customer: "cus_stripe_1", Status: "requires_payment_method",
			Created: 2000, LagoInvID: "inv_gating", Amount: 1650},
	}
	h := newSettleHarness(t, "incomplete", intents)
	if _, err := h.adapter.SubmitCommand(context.Background(), settleCmd(41, "txn-pror")); err != nil {
		t.Fatalf("a prorated gating intent (1650 of a 9900 frozen face) must settle, got %v", err)
	}
	paths := h.stripe.paths()
	drove := false
	for _, p := range paths {
		if p == "POST /v1/payment_intents/pi_prorated/confirm" {
			drove = true
		}
	}
	if !drove {
		t.Fatalf("the prorated gating intent must be driven to confirm, calls: %v", paths)
	}
}

// TestLagoSettleIntentListPaginatesToCompletion (A-17 / F85): the locator
// list is correctness-critical (unsettled candidates AND the settled
// idempotency window) — it must consume has_more/starting_after pages to
// completion instead of silently truncating at a page boundary.
func TestLagoSettleIntentListPaginatesToCompletion(t *testing.T) {
	// The gating intent is the NEWEST (created 2000) and lands on page 2
	// when the page size is 1; the settled older intent rides page 1.
	intents := []stripeIntentRec{
		{ID: "pi_new", Customer: "cus_stripe_1", Status: "requires_payment_method",
			Created: 2000, LagoInvID: "inv_gating"},
		{ID: "pi_old_settled", Customer: "cus_stripe_1", Status: "succeeded",
			Created: 1000, LagoInvID: "inv_gating"},
	}
	h := newSettleHarness(t, "incomplete", intents)
	h.stripe.page1Count = 1
	if _, err := h.adapter.SubmitCommand(context.Background(), settleCmd(41, "txn-page")); err != nil {
		t.Fatalf("a gating intent on a later page must still be found and driven: %v", err)
	}
	listCalls := 0
	h.stripe.mu.Lock()
	for _, req := range h.stripe.requests {
		if req.Method == http.MethodGet && req.Path == "/v1/payment_intents" {
			listCalls++
		}
	}
	h.stripe.mu.Unlock()
	if listCalls != 2 {
		t.Fatalf("the list read must page to completion (2 pages at page size 1), got %d list calls", listCalls)
	}
}

// TestLagoSettleAttachRailDerivesIdempotencyKeys (A-18 / F86): the attach
// rail call carries a DETERMINISTIC idempotency key derived from the
// command key, so a lost response can never double-attach. (The set-default
// key left the chain with D10's default-rewrite removal.)
func TestLagoSettleAttachRailDerivesIdempotencyKeys(t *testing.T) {
	intents := []stripeIntentRec{{
		ID: "pi_1", Customer: "cus_stripe_1", Status: "requires_payment_method",
		Created: 1000, LagoInvID: "inv_gating",
	}}
	h := newSettleHarness(t, "incomplete", intents)
	cmd := settleCmd(41, "txn-idem")
	if _, err := h.adapter.SubmitCommand(context.Background(), cmd); err != nil {
		t.Fatalf("settle: %v", err)
	}
	h.stripe.mu.Lock()
	defer h.stripe.mu.Unlock()
	idems := map[string]string{}
	for _, req := range h.stripe.requests {
		if req.Idem != "" {
			idems[req.Method+" "+req.Path] = req.Idem
		}
	}
	for path, suffix := range map[string]string{
		"POST /v1/payment_methods/pm_settle/attach": ":attach",
	} {
		if idems[path] != cmd.Key+suffix {
			t.Fatalf("%s must carry Idempotency-Key == cmd key%s, got %q", path, suffix, idems[path])
		}
	}
}

// TestLagoSettleTruncatedBodyIsUnreachable (A-19 / F87): a 2xx whose body
// dies mid-stream is a TRANSIENT transport failure — surfacing it as
// truncated JSON would misclassify downstream as the definitive
// invalid-response sentinel and park the order instead of retrying.
func TestLagoSettleTruncatedBodyIsUnreachable(t *testing.T) {
	h := newSettleHarness(t, "incomplete", nil)
	h.stripe.hijackListBody = true
	_, err := h.adapter.SubmitCommand(context.Background(), settleCmd(41, "txn-trunc"))
	if !errors.Is(err, commercial.ErrPlatformUnreachable) {
		t.Fatalf("a mid-stream body cut must classify unreachable (retryable), got %v", err)
	}
}

// TestLagoSettleOversizedBodyIsInvalidResponse (A-19): a body beyond the
// provider cap is a definitive oversized-response error — an explicit
// verdict, never a quiet truncation that only later fails the JSON parse.
func TestLagoSettleOversizedBodyIsInvalidResponse(t *testing.T) {
	h := newSettleHarness(t, "incomplete", nil)
	h.stripe.oversizeListBody = true
	_, err := h.adapter.SubmitCommand(context.Background(), settleCmd(41, "txn-big"))
	if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("an oversized provider body must classify invalid_response, got %v", err)
	}
}

// ---- #82 Task 12 (OCR r2): settle-chain correctness closure ----

// TestLagoSettleMultipleInvoiceCandidatesFailClosed (D9 / r2:139 high): the
// unsettled candidate set must span EXACTLY ONE distinct gating invoice
// identity before any charge-bearing call. Candidates from TWO different
// invoices (a renewal's or a foreign purchase's stuck intent next to the
// current gate) mean the locator cannot prove which invoice the settle owes
// — the amount guard alone (1..AmountFen) would happily pass a smaller
// foreign intent — so the whole drive fails closed: one locator read, ZERO
// writes, the attention face hands the ambiguity to operations. The
// SAME-invoice residual sibling (R-20) is unaffected: siblings share one
// invoice id, distinct == 1.
func TestLagoSettleMultipleInvoiceCandidatesFailClosed(t *testing.T) {
	intents := []stripeIntentRec{
		{ID: "pi_a", Customer: "cus_stripe_1", Status: "requires_payment_method", Created: 900, LagoInvID: "inv_old"},
		{ID: "pi_b", Customer: "cus_stripe_1", Status: "requires_payment_method", Created: 1000, LagoInvID: "inv_cur"},
	}
	h := newSettleHarness(t, "incomplete", intents)
	_, err := h.adapter.SubmitCommand(context.Background(), settleCmd(41, "txn-1"))
	if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("unsettled candidates spanning multiple invoices must fail closed invalid_response, got %v", err)
	}
	if !strings.Contains(err.Error(), "distinct") {
		t.Fatalf("the failure must name the distinct-invoice gate, got %v", err)
	}
	paths := h.stripe.paths()
	if len(paths) != 1 || paths[0] != "GET /v1/payment_intents" {
		t.Fatalf("the multi-invoice ambiguity must answer ONE locator read and zero writes, got %v", paths)
	}
}

// TestLagoSettleDoesNotTouchCustomerDefault (D10 / r2:357 high): the settle
// drive must NEVER rewrite the customer's default payment method. The
// confirm call carries the attached settle pm explicitly, so the permanent
// default rewrite buys nothing for THIS charge while silently rerouting
// every RENEWAL invoice's auto-collection onto the settlement instrument —
// a real monthly charge on the wrong card. The call sequence is exactly
// locator + attach + update + confirm.
func TestLagoSettleDoesNotTouchCustomerDefault(t *testing.T) {
	intents := []stripeIntentRec{{
		ID: "pi_1", Customer: "cus_stripe_1", Status: "requires_payment_method",
		Created: 1000, LagoInvID: "inv_gating",
	}}
	h := newSettleHarness(t, "incomplete", intents)
	if _, err := h.adapter.SubmitCommand(context.Background(), settleCmd(41, "txn-1")); err != nil {
		t.Fatalf("settle: %v", err)
	}
	want := []string{
		"GET /v1/payment_intents",
		"POST /v1/payment_methods/pm_settle/attach",
		"POST /v1/payment_intents/pi_1",
		"POST /v1/payment_intents/pi_1/confirm",
	}
	paths := h.stripe.paths()
	if len(paths) != len(want) {
		t.Fatalf("call sequence mismatch:\n got %v\nwant %v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("call %d mismatch: got %q want %q (all: %v)", i, paths[i], want[i], paths)
		}
	}
	for _, p := range paths {
		if strings.HasPrefix(p, "POST /v1/customers/") {
			t.Fatalf("settle must never rewrite the customer default payment method (D10), saw %q (all: %v)", p, paths)
		}
	}
}

// TestLagoSettleBindingRead429KeepsRetriable (D13 / r2:445 medium): the
// second binding read (boundProviderCustomerID — the cus_… extraction)
// must classify a 429 as UNREACHABLE, the same split customerProviderBound
// already applies: a throttled authority is a transient condition the
// fulfiller keeps pending and retries — never the definitive
// invalid_response that would mint a paid order into terminal attention.
// The stub scripts [200, 429]: the FIRST binding read passes, the SECOND
// answers the throttle.
func TestLagoSettleBindingRead429KeepsRetriable(t *testing.T) {
	intents := []stripeIntentRec{{
		ID: "pi_1", Customer: "cus_stripe_1", Status: "requires_payment_method",
		Created: 1000, LagoInvID: "inv_gating",
	}}
	h := newSettleHarness(t, "incomplete", intents)
	h.lago.mu.Lock()
	h.lago.customerGetStatuses = []int{http.StatusOK, http.StatusTooManyRequests}
	h.lago.mu.Unlock()
	_, err := h.adapter.SubmitCommand(context.Background(), settleCmd(41, "txn-429"))
	if !errors.Is(err, commercial.ErrPlatformUnreachable) {
		t.Fatalf("a 429 on the binding read must classify unreachable (keeps the event pending/retriable), got %v", err)
	}
	if errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("a transient 429 must NOT be the definitive sentinel, got %v", err)
	}
}

// TestLagoSettleFailsClosedWithoutStripeApiKey (r2:166 low): a missing
// provider API key is a CONFIGURATION gap — ErrPlatformUnconfigured — not
// an authority rejection. Without the guard the empty key rides the
// Authorization header, the provider answers 401, and the definitive
// classification parks the order in attention for what is an operator
// fixable env gap. Zero outbound calls may fire.
func TestLagoSettleFailsClosedWithoutStripeApiKey(t *testing.T) {
	intents := []stripeIntentRec{{
		ID: "pi_1", Customer: "cus_stripe_1", Status: "requires_payment_method",
		Created: 1000, LagoInvID: "inv_gating",
	}}
	h := newSettleHarness(t, "incomplete", intents)
	h.adapter.cfg.StripeAPIKey = ""
	_, err := h.adapter.SubmitCommand(context.Background(), settleCmd(41, "txn-nokey"))
	if !errors.Is(err, commercial.ErrPlatformUnconfigured) {
		t.Fatalf("a missing provider API key must fail closed unconfigured, got %v", err)
	}
	if n := h.stripe.requestCount(); n != 0 {
		t.Fatalf("an unconfigured settle must make ZERO provider calls, got %d: %v", n, h.stripe.paths())
	}
}
