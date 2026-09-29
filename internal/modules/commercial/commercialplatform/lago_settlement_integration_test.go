//go:build lago_integration

// Tagged real-Lago settle-rail evidence (#82 Task 9). The build tag keeps
// this out of every normal suite run; the test is env-gated on the
// LAGO_INTEGRATION_* family (operator-owned, never committed) and skips
// otherwise, so a missing stack records blocked-env instead of failing or
// faking a pass.
//
// Chain under test (the α dual-track, t11 P-A..P-E):
//
//	ensure customer+binding (3DS pm env) → gated create → the stuck gating
//	PaymentIntent is visible provider-side (status recorded as observed) →
//	settle_purchase_payment (attach settle pm + update + confirm) → the
//	harness delivers the REAL payment_intent.succeeded event through the
//	built-in webhook route (real-secret HMAC, transport-leg stand-in per
//	D8) → the authority finalizes: subscription active + invoice finalized/
//	payment_status succeeded + EXACTLY ONE succeeded payment.
//
// Negative controls: settle replay is a zero-side-effect no-op; duplicate
// webhook delivery is byte-identical no-op; the activation only happens
// through this chain (never a local write).
package commercialplatform

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func integrationEnv(names ...string) map[string]string {
	out := map[string]string{}
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			out[name] = value
		}
	}
	return out
}

// stripeForm issues one form-encoded Stripe API call with the credential
// ONLY in the Authorization header.
func stripeForm(t *testing.T, apiKey, method, path string, form url.Values) (int, map[string]any) {
	t.Helper()
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		var reader io.Reader
		if form != nil {
			reader = strings.NewReader(form.Encode())
		}
		req, err := http.NewRequest(method, "https://api.stripe.com"+path, reader)
		if err != nil {
			t.Fatalf("stripe request: %v", err)
		}
		req.Header.Set("Authorization", "Basic "+base64Header(apiKey))
		if form != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		if err != nil {
			lastErr = err // transport flake: bounded retry (HTTPError is a real answer)
			time.Sleep(time.Duration(attempt+1) * 1500 * time.Millisecond)
			continue
		}
		defer resp.Body.Close()
		blob, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		var body map[string]any
		_ = json.Unmarshal(blob, &body)
		return resp.StatusCode, body
	}
	t.Fatalf("stripe %s %s: %v", method, path, lastErr)
	return 0, nil
}

func base64Header(apiKey string) string {
	const b64chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	raw := []byte(apiKey + ":")
	var out strings.Builder
	for i := 0; i < len(raw); i += 3 {
		var b [3]byte
		n := copy(b[:], raw[i:])
		out.WriteByte(b64chars[b[0]>>2])
		out.WriteByte(b64chars[(b[0]&0x03)<<4|b[1]>>4])
		if n > 1 {
			out.WriteByte(b64chars[(b[1]&0x0f)<<2|b[2]>>6])
		} else {
			out.WriteByte('=')
		}
		if n > 2 {
			out.WriteByte(b64chars[b[2]&0x3f])
		} else {
			out.WriteByte('=')
		}
	}
	return out.String()
}

type paymentIntentCandidate struct {
	id        string
	status    string
	invoiceID string
}

// selectExpectedPaymentIntent picks only the exact succeeded, invoice-linked
// intent that was observed before settlement.
func selectExpectedPaymentIntent(expectedID string, rows []paymentIntentCandidate) (paymentIntentCandidate, error) {
	for _, row := range rows {
		if row.id == expectedID && row.status == "succeeded" && row.invoiceID != "" {
			return row, nil
		}
	}
	return paymentIntentCandidate{}, fmt.Errorf("expected succeeded invoice-linked PaymentIntent %q not found", expectedID)
}

func paymentIntentCandidates(rows []any) ([]paymentIntentCandidate, map[string]struct{}) {
	var eligible []paymentIntentCandidate
	observed := make(map[string]struct{})
	for _, raw := range rows {
		row, _ := raw.(map[string]any)
		if row == nil {
			continue
		}
		id, _ := row["id"].(string)
		if id != "" {
			observed[id] = struct{}{}
		}
		meta, _ := row["metadata"].(map[string]any)
		invoiceID, _ := meta["lago_invoice_id"].(string)
		status, _ := row["status"].(string)
		if id == "" || invoiceID == "" {
			continue
		}
		switch status {
		case "requires_payment_method", "requires_action", "requires_confirmation":
			eligible = append(eligible, paymentIntentCandidate{id: id, status: status, invoiceID: invoiceID})
		}
	}
	return eligible, observed
}

func requireUniquePaymentIntentCandidate(candidates []paymentIntentCandidate) (paymentIntentCandidate, error) {
	if len(candidates) != 1 {
		return paymentIntentCandidate{}, fmt.Errorf("expected exactly one pre-settle invoice-linked unsettled PaymentIntent, found %d: %+v", len(candidates), candidates)
	}
	return candidates[0], nil
}

func succeededPaymentIntent(expectedID string, preSettleIDs map[string]struct{}, rows []any) (map[string]any, error) {
	candidates := make([]paymentIntentCandidate, 0, len(rows))
	intents := make(map[string]map[string]any)
	var observed []string
	for _, raw := range rows {
		intent, _ := raw.(map[string]any)
		if intent == nil || intent["status"] != "succeeded" {
			continue
		}
		id, _ := intent["id"].(string)
		meta, _ := intent["metadata"].(map[string]any)
		invoiceID, _ := meta["lago_invoice_id"].(string)
		if invoiceID == "" {
			continue
		}
		observed = append(observed, id)
		if _, existed := preSettleIDs[id]; !existed {
			return nil, fmt.Errorf("new succeeded invoice-linked PaymentIntent %q appeared after settle; pre-settle IDs=%v", id, sortedPaymentIntentIDs(preSettleIDs))
		}
		candidates = append(candidates, paymentIntentCandidate{id: id, status: "succeeded", invoiceID: invoiceID})
		intents[id] = intent
	}
	selected, err := selectExpectedPaymentIntent(expectedID, candidates)
	if err != nil {
		return nil, fmt.Errorf("%w; observed succeeded invoice-linked IDs=%v", err, observed)
	}
	return intents[selected.id], nil
}

func sortedPaymentIntentIDs(ids map[string]struct{}) []string {
	out := make([]string, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func TestPaymentIntentCandidatesCaptureAllIDsAndFilterEligibility(t *testing.T) {
	rows := []any{
		map[string]any{"id": "pi_method", "status": "requires_payment_method", "metadata": map[string]any{"lago_invoice_id": "in_1"}},
		map[string]any{"id": "pi_action", "status": "requires_action", "metadata": map[string]any{"lago_invoice_id": "in_2"}},
		map[string]any{"id": "pi_confirmation", "status": "requires_confirmation", "metadata": map[string]any{"lago_invoice_id": "in_3"}},
		map[string]any{"id": "pi_old", "status": "succeeded", "metadata": map[string]any{"lago_invoice_id": "in_old"}},
		map[string]any{"id": "pi_unlinked", "status": "requires_action"},
		map[string]any{"id": "pi_other_status", "status": "processing", "metadata": map[string]any{"lago_invoice_id": "in_4"}},
		map[string]any{"status": "requires_action", "metadata": map[string]any{"lago_invoice_id": "in_missing_id"}},
	}
	eligible, observed := paymentIntentCandidates(rows)
	if len(eligible) != 3 {
		t.Fatalf("eligible candidates = %+v, want exactly the three unsettled invoice-linked candidates", eligible)
	}
	wantEligible := map[string]string{"pi_method": "requires_payment_method", "pi_action": "requires_action", "pi_confirmation": "requires_confirmation"}
	for _, candidate := range eligible {
		if wantEligible[candidate.id] != candidate.status || candidate.invoiceID == "" {
			t.Errorf("unexpected eligible candidate: %+v", candidate)
		}
	}
	for _, id := range []string{"pi_method", "pi_action", "pi_confirmation", "pi_old", "pi_unlinked", "pi_other_status"} {
		if _, ok := observed[id]; !ok {
			t.Errorf("pre-settle ID %q not captured; observed=%v", id, sortedPaymentIntentIDs(observed))
		}
	}
	if _, ok := observed[""]; ok {
		t.Fatal("empty PaymentIntent ID was captured")
	}
}

func TestPaymentIntentCandidatesExposeAmbiguity(t *testing.T) {
	rows := []any{
		map[string]any{"id": "pi_a", "status": "requires_action", "metadata": map[string]any{"lago_invoice_id": "in_a"}},
		map[string]any{"id": "pi_b", "status": "requires_confirmation", "metadata": map[string]any{"lago_invoice_id": "in_b"}},
	}
	eligible, _ := paymentIntentCandidates(rows)
	if len(eligible) != 2 {
		t.Fatalf("ambiguous candidates = %+v, want both candidates exposed", eligible)
	}
	if _, err := requireUniquePaymentIntentCandidate(eligible); err == nil || !strings.Contains(err.Error(), "found 2") {
		t.Fatalf("ambiguous candidate error = %v, want count diagnostic", err)
	}
	if _, err := requireUniquePaymentIntentCandidate(nil); err == nil || !strings.Contains(err.Error(), "found 0") {
		t.Fatalf("missing candidate error = %v, want count diagnostic", err)
	}
}

func TestSucceededPaymentIntentEnforcesPreSettleIdentity(t *testing.T) {
	tests := []struct {
		name      string
		expected  string
		preSettle []string
		rows      []any
		wantID    string
		wantErr   string
	}{
		{name: "expected plus older captured success", expected: "pi_expected", preSettle: []string{"pi_expected", "pi_old"}, rows: []any{
			map[string]any{"id": "pi_old", "status": "succeeded", "metadata": map[string]any{"lago_invoice_id": "in_old"}},
			map[string]any{"id": "pi_expected", "status": "succeeded", "metadata": map[string]any{"lago_invoice_id": "in_expected"}},
		}, wantID: "pi_expected"},
		{name: "new linked success rejected", expected: "pi_expected", preSettle: []string{"pi_expected"}, rows: []any{
			map[string]any{"id": "pi_expected", "status": "succeeded", "metadata": map[string]any{"lago_invoice_id": "in_expected"}},
			map[string]any{"id": "pi_new", "status": "succeeded", "metadata": map[string]any{"lago_invoice_id": "in_new"}},
		}, wantErr: "pi_new"},
		{name: "new unlinked success ignored", expected: "pi_expected", preSettle: []string{"pi_expected"}, rows: []any{
			map[string]any{"id": "pi_new", "status": "succeeded"},
			map[string]any{"id": "pi_expected", "status": "succeeded", "metadata": map[string]any{"lago_invoice_id": "in_expected"}},
		}, wantID: "pi_expected"},
		{name: "absent expected reports expected and observed", expected: "pi_expected", preSettle: []string{"pi_expected", "pi_old"}, rows: []any{
			map[string]any{"id": "pi_old", "status": "succeeded", "metadata": map[string]any{"lago_invoice_id": "in_old"}},
		}, wantErr: "expected succeeded invoice-linked PaymentIntent \"pi_expected\" not found; observed succeeded invoice-linked IDs=[pi_old]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preSettle := make(map[string]struct{}, len(tt.preSettle))
			for _, id := range tt.preSettle {
				preSettle[id] = struct{}{}
			}
			got, err := succeededPaymentIntent(tt.expected, preSettle, tt.rows)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want error containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("selection error = %v", err)
			}
			if got == nil || got["id"] != tt.wantID {
				t.Fatalf("selected intent ID = %v, want %q", got["id"], tt.wantID)
			}
		})
	}
}

// deliverWebhookEvent POSTs the REAL PI event through the built-in webhook
// route, signed with the provider's webhook secret (the transport-leg
// stand-in per D8: production's leg is Stripe's own delivery).
func deliverWebhookEvent(t *testing.T, baseURL, orgID, providerCode, secret string, event map[string]any) int {
	t.Helper()
	payload, _ := json.Marshal(event)
	ts := fmt.Sprintf("%d", time.Now().Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(append([]byte(ts+"."), payload...))
	signature := fmt.Sprintf("t=%s,v1=%x", ts, mac.Sum(nil))
	endpoint := fmt.Sprintf("%s/webhooks/stripe/%s?code=%s", baseURL, orgID, url.QueryEscape(providerCode))
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("webhook request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Stripe-Signature", signature)
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("webhook deliver: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(io.LimitReader(resp.Body, 1<<18))
	return resp.StatusCode
}

// TestLagoIntegrationSettleActivatesGatedSubscription: the full dual-track
// chain on the real pinned stack. Skips (never fails) without the env.
func TestLagoIntegrationSettleActivatesGatedSubscription(t *testing.T) {
	env := integrationEnv(
		"LAGO_INTEGRATION_BASE_URL", "LAGO_INTEGRATION_API_KEY",
		"LAGO_INTEGRATION_STRIPE_KEY", "LAGO_INTEGRATION_STRIPE_SETTLE_PM",
		"LAGO_INTEGRATION_WEBHOOK_SECRET", "LAGO_INTEGRATION_ORG_ID",
		"LAGO_INTEGRATION_GATE_PM",
	)
	for _, name := range []string{
		"LAGO_INTEGRATION_BASE_URL", "LAGO_INTEGRATION_API_KEY",
		"LAGO_INTEGRATION_STRIPE_KEY", "LAGO_INTEGRATION_STRIPE_SETTLE_PM",
		"LAGO_INTEGRATION_WEBHOOK_SECRET", "LAGO_INTEGRATION_ORG_ID",
	} {
		if env[name] == "" {
			t.Skip("lago integration env not configured")
		}
	}
	baseURL := env["LAGO_INTEGRATION_BASE_URL"]
	apiKey := env["LAGO_INTEGRATION_API_KEY"]
	stripeKey := env["LAGO_INTEGRATION_STRIPE_KEY"]
	settlePm := env["LAGO_INTEGRATION_STRIPE_SETTLE_PM"]
	webhookSecret := env["LAGO_INTEGRATION_WEBHOOK_SECRET"]
	orgID := env["LAGO_INTEGRATION_ORG_ID"]
	gatePm := env["LAGO_INTEGRATION_GATE_PM"]
	if gatePm == "" {
		gatePm = "pm_card_threeDSecure2Required" // the stuck-window test card (F7)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	tenant := uint64(time.Now().UnixNano() % 1_000_000)
	extCustomer := commercial.ExternalCustomerID(tenant)
	extPurchase := commercial.ExternalPurchaseSubscriptionID(tenant)
	planKey := "pro" // a ladder tier (the publish validation requires it)
	// The channel transaction ids must be unique per RUN: they key the
	// provider Idempotency-Key, and Stripe binds a key to its first
	// request's parameters forever (a replayed key on a different intent
	// answers 400 idempotency_error).
	txn := fmt.Sprintf("t9-txn-%d", time.Now().UnixNano())

	a := NewLagoAdapter(Config{
		Provider: ProviderLago, BaseURL: baseURL, APIKey: apiKey, Release: lockedRelease(t),
		StripeAPIKey: stripeKey, StripeSettlePmToken: settlePm,
		OutboundAllowLoopback: strings.Contains(baseURL, "127.0.0.1") || strings.Contains(baseURL, "localhost"),
	})

	// The plan must exist on the authority before the gated create — the
	// real #79 draft+publish flow (the t09 integration precedent).
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&repocommercial.PlanRow{}, &repocommercial.QuoteRow{}); err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	plans, err := commercialsvc.NewPlanVersionService(db, a)
	if err != nil {
		t.Fatal(err)
	}
	view, err := plans.CreateDraft(ctx, "integration:t9", commercialsvc.DraftInput{
		PlanKey: planKey, Name: "T9 Pro", AmountFen: 9900, IncludedCreditsMicro: 9_900_000,
		Features: map[string]bool{"advanced_models": true}, Currency: commercial.CurrencyCNY,
	})
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if _, err := plans.Publish(ctx, "integration:t9", "t9 evidence", planKey, view.Version); err != nil {
		t.Fatalf("publish: %v", err)
	}
	planCode := commercial.DeterministicPlanCode(planKey, view.Version)

	t.Run("gated create leaves the purchase incomplete with a stuck intent", func(t *testing.T) {
		// The gate card rides the provider customer: bind through the
		// purchase create (the adapter attaches StripePmToken as default).
		a.cfg.StripePmToken = gatePm
		if _, err := a.SubmitCommand(ctx, commercial.Command{
			Kind:  commercial.CommandKindCreatePurchaseSubscription,
			Key:   commercial.CreatePurchaseSubscriptionCommandKey(extPurchase, planCode),
			Actor: "t9", Reason: "integration",
			Payload: commercial.CreatePurchaseSubscriptionPayload{
				TenantID: tenant, ExternalCustomerID: extCustomer,
				ExternalPurchaseSubscriptionID: extPurchase, PlanCode: planCode,
				AmountFen: 9900, Currency: commercial.CurrencyCNY,
			},
		}); err != nil {
			t.Fatalf("gated create: %v", err)
		}
		snap, err := a.ReadSnapshot(ctx, commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: tenant})
		if err != nil {
			t.Fatal(err)
		}
		if snap.Purchase.State != commercial.PurchaseStateAwaitingPayment {
			t.Fatalf("fresh gated purchase must be awaiting payment, got %q", snap.Purchase.State)
		}
	})

	t.Run("settle drives the rails and the webhook finalizes", func(t *testing.T) {
		// The authority creates the gating PaymentIntent ASYNC
		// (Invoices::Payments::CreateJob): bounded-wait until the provider
		// rails answer an unsettled intent (the settle command itself
		// stays fail-closed — this wait is the test's own pacing).
		providerCustomerID := providerCustomerOf(t, a, extCustomer)
		var status int
		var body map[string]any
		var expectedID string
		var preSettleIDs map[string]struct{}
		waitDeadline := time.Now().Add(60 * time.Second)
		for {
			status, body = stripeForm(t, stripeKey, http.MethodGet,
				"/v1/payment_intents?customer="+url.QueryEscape(providerCustomerID)+"&limit=10", nil)
			found := false
			if status == 200 {
				rows, _ := body["data"].([]any)
				eligible, observed := paymentIntentCandidates(rows)
				candidate, candidateErr := requireUniquePaymentIntentCandidate(eligible)
				if candidateErr == nil {
					found = true
					expectedID = candidate.id
					preSettleIDs = observed
				} else if time.Now().After(waitDeadline) {
					t.Fatalf("pre-settle PaymentIntent identity unresolved: %v", candidateErr)
				}
			}
			if found || time.Now().After(waitDeadline) {
				break
			}
			time.Sleep(3 * time.Second)
		}
		if expectedID == "" {
			t.Fatalf("no unique pre-settle invoice-linked unsettled PaymentIntent found before settle (HTTP %d)", status)
		}
		if _, err := a.SubmitCommand(ctx, commercial.Command{
			Kind:  commercial.CommandKindSettlePurchasePayment,
			Key:   commercial.SettlePurchasePaymentCommandKey(extPurchase, txn),
			Actor: "t9", Reason: "integration settle",
			Payload: commercial.SettlePurchasePaymentPayload{
				TenantID: tenant, ExternalCustomerID: extCustomer,
				ExternalPurchaseSubscriptionID: extPurchase, PlanCode: planCode,
				ChannelTransaction: txn, AmountFen: 9900, Currency: commercial.CurrencyCNY,
			},
		}); err != nil {
			t.Fatalf("settle: %v", err)
		}

		// The transport-leg stand-in (D8): read the REAL PI back from the
		// provider and deliver the built-in chain the event it expects.
		// (providerCustomerID/status/body/rows were declared by the async
		// wait above — re-listing answers the succeeded intent.)
		status, body = stripeForm(t, stripeKey, http.MethodGet,
			"/v1/payment_intents?customer="+url.QueryEscape(providerCustomerID)+"&limit=10", nil)
		if status != 200 {
			t.Fatalf("intent list: HTTP %d", status)
		}
		rows, _ := body["data"].([]any)
		intent, err := succeededPaymentIntent(expectedID, preSettleIDs, rows)
		if err != nil {
			t.Fatal(err)
		}
		event := map[string]any{
			"id":          fmt.Sprintf("evt_t9_%d", time.Now().UnixNano()),
			"object":      "event",
			"api_version": "2024-06-20",
			"created":     time.Now().Unix(),
			"livemode":    false,
			"type":        "payment_intent.succeeded",
			"data":        map[string]any{"object": intent},
		}
		providerCode := providerCodeOf(t, a)
		if code := deliverWebhookEvent(t, baseURL, orgID, providerCode, webhookSecret, event); code != 200 {
			t.Fatalf("webhook delivery answered HTTP %d", code)
		}

		finalizeDeadline := time.Now().Add(120 * time.Second)
		for {
			snap, err := a.ReadSnapshot(ctx, commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: tenant})
			if err != nil {
				t.Fatal(err)
			}
			if snap.Purchase.State == commercial.PurchaseStateActive {
				if len(snap.Purchase.InvoiceFees) != 1 ||
					snap.Purchase.InvoiceFees[0].Kind != "subscription_fee" ||
					snap.Purchase.InvoiceFees[0].AmountFen <= 0 ||
					snap.Purchase.InvoiceFees[0].AmountFen > 9900 ||
					snap.Purchase.InvoicePaymentStatus != "succeeded" {
					t.Fatalf("D6' re-check failed: %+v", snap.Purchase)
				}
				return
			}
			if time.Now().After(finalizeDeadline) {
				t.Fatalf("authority did not finalize within 120s, state=%q", snap.Purchase.State)
			}
			time.Sleep(3 * time.Second)
		}
	})

	t.Run("settle replay is a zero-side-effect no-op", func(t *testing.T) {
		before := purchaseSnapshotJSON(t, a, tenant)
		if _, err := a.SubmitCommand(ctx, commercial.Command{
			Kind:  commercial.CommandKindSettlePurchasePayment,
			Key:   commercial.SettlePurchasePaymentCommandKey(extPurchase, txn),
			Actor: "t9", Reason: "integration settle replay",
			Payload: commercial.SettlePurchasePaymentPayload{
				TenantID: tenant, ExternalCustomerID: extCustomer,
				ExternalPurchaseSubscriptionID: extPurchase, PlanCode: planCode,
				ChannelTransaction: txn, AmountFen: 9900, Currency: commercial.CurrencyCNY,
			},
		}); err != nil {
			t.Fatalf("replay settle: %v", err)
		}
		after := purchaseSnapshotJSON(t, a, tenant)
		if before != after {
			t.Fatalf("replayed settle changed the authority state:\nbefore %s\nafter  %s", before, after)
		}
	})

	// (#84 Task 6 / AC3 authority side) A SECOND settle under a DIFFERENT
	// channel transaction — the multiple-success second payment's shape, the
	// command a buggy coordinator would drive — must short-circuit on the
	// already-active purchase (step (i)): nil error + receipt, and EXACTLY
	// ONE succeeded Lago Payment before and after (never a second Stripe
	// charge, never a second Payment row). Merged as a subtest of the
	// activation case on purpose: reaching active REQUIRES the full
	// gated-create → settle → webhook chain (local activation is forbidden,
	// spec L123-125), so an independent function would re-run the whole
	// multi-minute chain for the same assertion.
	t.Run("TestSettleSecondChannelTransactionShortCircuitsWhenActive", func(t *testing.T) {
		before := countLagoSucceededPayments(t, a, extCustomer)
		if before != 1 {
			t.Fatalf("setup: exactly one succeeded Lago payment expected after the single activation, got %d", before)
		}
		txnOther := txn + "-OTHER"
		if _, err := a.SubmitCommand(ctx, commercial.Command{
			Kind:  commercial.CommandKindSettlePurchasePayment,
			Key:   commercial.SettlePurchasePaymentCommandKey(extPurchase, txnOther),
			Actor: "t9", Reason: "integration second-channel settle must short-circuit",
			Payload: commercial.SettlePurchasePaymentPayload{
				TenantID: tenant, ExternalCustomerID: extCustomer,
				ExternalPurchaseSubscriptionID: extPurchase, PlanCode: planCode,
				ChannelTransaction: txnOther, AmountFen: 9900, Currency: commercial.CurrencyCNY,
			},
		}); err != nil {
			t.Fatalf("the second-channel settle must short-circuit with nil error (idempotent receipt), got %v", err)
		}
		after := countLagoSucceededPayments(t, a, extCustomer)
		if after != before {
			t.Fatalf("the short-circuited second settle must not mint another Lago payment: before=%d after=%d", before, after)
		}
	})
}

// countLagoSucceededPayments reads the authority's Payment ledger for one
// external customer (#84 Task 6): the authoritative count of succeeded
// payments behind the "never a second Lago Payment" assertion.
func countLagoSucceededPayments(t *testing.T, a *LagoAdapter, extCustomer string) int {
	t.Helper()
	status, body, err := a.do(context.Background(), http.MethodGet,
		"/api/v1/payments?external_customer_id="+url.QueryEscape(extCustomer)+"&per_page=100", nil)
	if err != nil || status != 200 {
		t.Fatalf("lago payments read: HTTP %d err=%v", status, err)
	}
	var parsed struct {
		Payments []struct {
			Status string `json:"status"`
		} `json:"payments"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("lago payments parse: %v", err)
	}
	n := 0
	for _, p := range parsed.Payments {
		if p.Status == "succeeded" {
			n++
		}
	}
	return n
}

func providerCustomerOf(t *testing.T, a *LagoAdapter, extCustomer string) string {
	t.Helper()
	status, body, err := a.do(context.Background(), http.MethodGet,
		"/api/v1/customers/"+url.PathEscape(extCustomer), nil)
	if err != nil || status != 200 {
		t.Fatalf("customer read: HTTP %d err=%v", status, err)
	}
	var parsed struct {
		Customer struct {
			BillingConfiguration struct {
				ProviderCustomerID string `json:"provider_customer_id"`
			} `json:"billing_configuration"`
		} `json:"customer"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil ||
		parsed.Customer.BillingConfiguration.ProviderCustomerID == "" {
		t.Fatalf("customer binding malformed")
	}
	return parsed.Customer.BillingConfiguration.ProviderCustomerID
}

func providerCodeOf(t *testing.T, a *LagoAdapter) string {
	t.Helper()
	status, body, err := a.do(context.Background(), http.MethodGet, "/api/v1/organizations", nil)
	if err != nil || status != 200 {
		t.Fatalf("organizations read: HTTP %d err=%v", status, err)
	}
	_ = body
	// The lab seeds exactly one provider; its code is the settle path's own
	// binding source — the adapter's providerCustomerPrefix env carries it
	// on lab stacks. Fall back to the well-known lab code.
	if a.cfg.ProviderCustomerPrefix != "" {
		return a.cfg.ProviderCustomerPrefix
	}
	return "weknora-stripe"
}

func purchaseSnapshotJSON(t *testing.T, a *LagoAdapter, tenant uint64) string {
	t.Helper()
	snap, err := a.ReadSnapshot(context.Background(), commercial.SnapshotQuery{
		Kind: commercial.SnapshotKindPurchase, TenantID: tenant,
	})
	if err != nil {
		t.Fatal(err)
	}
	// CheckedAt is the read's own clock (always different); the no-op
	// comparison is over every AUTHORITY-sourced field.
	stable := struct {
		State                string                           `json:"state"`
		PlanCode             string                           `json:"plan_code"`
		AmountFen            int64                            `json:"amount_fen"`
		Currency             string                           `json:"currency"`
		InvoiceFees          []commercial.InvoiceLineSnapshot `json:"invoice_fees"`
		InvoicePaymentStatus string                           `json:"invoice_payment_status"`
	}{
		State:                snap.Purchase.State,
		PlanCode:             snap.Purchase.PlanCode,
		AmountFen:            snap.Purchase.AmountFen,
		Currency:             snap.Purchase.Currency,
		InvoiceFees:          snap.Purchase.InvoiceFees,
		InvoicePaymentStatus: snap.Purchase.InvoicePaymentStatus,
	}
	blob, _ := json.Marshal(stable)
	return string(blob)
}
