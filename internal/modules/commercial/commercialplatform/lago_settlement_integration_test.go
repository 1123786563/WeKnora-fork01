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

func TestPaymentIntentCandidateSelection(t *testing.T) {
	preSettleRows := []any{
		map[string]any{"id": "pi_older", "created": float64(100), "status": "requires_payment_method", "metadata": map[string]any{"lago_invoice_id": "inv_current"}},
		map[string]any{"id": "pi_newest", "created": float64(200), "status": "requires_action", "metadata": map[string]any{"lago_invoice_id": "inv_current"}},
		map[string]any{"id": "pi_unlinked", "created": float64(300), "status": "requires_action", "metadata": map[string]any{}},
		map[string]any{"id": "pi_already_done", "created": float64(400), "status": "succeeded", "metadata": map[string]any{"lago_invoice_id": "inv_old"}},
	}
	candidates, err := preSettlePaymentIntentCandidates(preSettleRows)
	if err != nil {
		t.Fatal(err)
	}
	preObservedIDs := paymentIntentIDSet(preSettleRows)
	for _, id := range []string{"pi_older", "pi_newest", "pi_unlinked", "pi_already_done"} {
		if _, ok := preObservedIDs[id]; !ok {
			t.Errorf("pre-settle identity set is missing %q", id)
		}
	}
	if got := len(preObservedIDs); got != 4 {
		t.Fatalf("pre-settle identity set has %d entries, want exactly 4: %v", got, preObservedIDs)
	}
	expectedID, err := expectedPaymentIntentID(candidates)
	if err != nil {
		t.Fatal(err)
	}
	if expectedID != "pi_newest" {
		t.Fatalf("expected newest candidate, got %q", expectedID)
	}
	if _, ok := candidates["pi_unlinked"]; ok {
		t.Fatal("intent without Lago invoice metadata was captured")
	}

	rows := []any{
		map[string]any{"id": "pi_newest", "status": "succeeded", "metadata": map[string]any{"lago_invoice_id": "inv_current"}},
		map[string]any{"id": "pi_older", "status": "succeeded", "metadata": map[string]any{"lago_invoice_id": "inv_current"}},
	}
	selected, err := succeededPaymentIntent(rows, expectedID, preObservedIDs)
	if err != nil {
		t.Fatalf("expected newest candidate to be selected: %v", err)
	}
	if selected["id"] != "pi_newest" {
		t.Fatalf("selected %v, want pi_newest", selected["id"])
	}

	_, err = succeededPaymentIntent(rows[1:], expectedID, preObservedIDs)
	if err == nil || !strings.Contains(err.Error(), "pi_newest") || !strings.Contains(err.Error(), "pi_older") {
		t.Fatalf("older-only success must fail with expected/observed IDs, got %v", err)
	}

	_, err = succeededPaymentIntent(append(rows, map[string]any{
		"id": "pi_interleaved", "status": "succeeded", "metadata": map[string]any{"lago_invoice_id": "inv_current"},
	}), expectedID, preObservedIDs)
	if err == nil || !strings.Contains(err.Error(), "pi_interleaved") {
		t.Fatalf("new invoice-linked success must fail and name its identity, got %v", err)
	}

	// A pre-existing succeeded history row is allowed, and a newly listed
	// succeeded row without invoice linkage does not represent an invoice race.
	withHistory := append(rows, map[string]any{
		"id": "pi_already_done", "status": "succeeded", "metadata": map[string]any{"lago_invoice_id": "inv_old"},
	})
	if _, err := succeededPaymentIntent(withHistory, expectedID, preObservedIDs); err != nil {
		t.Fatalf("pre-existing historical success must be allowed: %v", err)
	}
	withoutInvoice := append(rows, map[string]any{"id": "pi_unlinked_new", "status": "succeeded", "metadata": map[string]any{}})
	if _, err := succeededPaymentIntent(withoutInvoice, expectedID, preObservedIDs); err != nil {
		t.Fatalf("new success without invoice metadata must not trigger identity guard: %v", err)
	}

	tied, err := preSettlePaymentIntentCandidates([]any{
		map[string]any{"id": "pi_tie_a", "created": float64(500), "status": "requires_action", "metadata": map[string]any{"lago_invoice_id": "inv_current"}},
		map[string]any{"id": "pi_tie_b", "created": float64(500), "status": "requires_payment_method", "metadata": map[string]any{"lago_invoice_id": "inv_current"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := expectedPaymentIntentID(tied); err == nil || !strings.Contains(err.Error(), "share created timestamp") {
		t.Fatalf("tied newest timestamps must fail closed before settle, got %v", err)
	}
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
	Created   int64
	InvoiceID string
}

// preSettlePaymentIntentCandidates captures the identity and ordering fields
// needed to mirror latestIntent before issuing the settle command.
func preSettlePaymentIntentCandidates(rows []any) (map[string]paymentIntentCandidate, error) {
	candidates := make(map[string]paymentIntentCandidate)
	for _, row := range rows {
		intent, _ := row.(map[string]any)
		if intent == nil {
			continue
		}
		switch intent["status"] {
		case "requires_payment_method", "requires_action", "requires_confirmation":
			meta, _ := intent["metadata"].(map[string]any)
			invoiceID, _ := meta["lago_invoice_id"].(string)
			id, _ := intent["id"].(string)
			if invoiceID == "" {
				continue
			}
			if id == "" {
				return nil, fmt.Errorf("invoice-linked unsettled PaymentIntent has empty ID")
			}
			created, ok := intent["created"].(float64)
			if !ok || created <= 0 || created != float64(int64(created)) {
				return nil, fmt.Errorf("invoice-linked unsettled PaymentIntent %s has invalid created timestamp %v", id, intent["created"])
			}
			candidates[id] = paymentIntentCandidate{Created: int64(created), InvoiceID: invoiceID}
		}
	}
	return candidates, nil
}

func expectedPaymentIntentID(candidates map[string]paymentIntentCandidate) (string, error) {
	if len(candidates) == 0 {
		return "", fmt.Errorf("no invoice-linked unsettled PaymentIntent candidates")
	}
	var latestID string
	var latestCreated int64
	tied := false
	for id, candidate := range candidates {
		if candidate.Created > latestCreated {
			latestID, latestCreated, tied = id, candidate.Created, false
		} else if candidate.Created == latestCreated {
			tied = true
		}
	}
	if tied {
		return "", fmt.Errorf("multiple latest PaymentIntent candidates share created timestamp %d", latestCreated)
	}
	return latestID, nil
}

func succeededPaymentIntent(rows []any, expectedID string, preObservedIDs map[string]struct{}) (map[string]any, error) {
	var observed []string
	var unexpected []string
	var selected map[string]any
	for _, row := range rows {
		intent, _ := row.(map[string]any)
		if intent == nil || intent["status"] != "succeeded" {
			continue
		}
		id, _ := intent["id"].(string)
		observed = append(observed, id)
		meta, _ := intent["metadata"].(map[string]any)
		invoiceID, _ := meta["lago_invoice_id"].(string)
		if id != "" && invoiceID != "" {
			if _, wasObserved := preObservedIDs[id]; !wasObserved {
				unexpected = append(unexpected, id)
			}
		}
		if id == expectedID && id != "" && invoiceID != "" {
			selected = intent
		}
	}
	if len(unexpected) > 0 {
		sort.Strings(unexpected)
		return nil, fmt.Errorf("post-settle list contains newly appeared succeeded invoice-linked PaymentIntent IDs %v", unexpected)
	}
	if selected != nil {
		return selected, nil
	}
	return nil, fmt.Errorf("no succeeded PaymentIntent matched expected pre-settle ID %s; observed succeeded IDs %v", expectedID, observed)
}

func paymentIntentIDSet(rows []any) map[string]struct{} {
	ids := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		intent, _ := row.(map[string]any)
		if id, _ := intent["id"].(string); id != "" {
			ids[id] = struct{}{}
		}
	}
	return ids
}

func paymentIntentIDs(body map[string]any) []string {
	rows, _ := body["data"].([]any)
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		intent, _ := row.(map[string]any)
		if id, _ := intent["id"].(string); id != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
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
		var candidates map[string]paymentIntentCandidate
		var preObservedIDs map[string]struct{}
		var expectedIntentID string
		waitDeadline := time.Now().Add(60 * time.Second)
		for {
			status, body = stripeForm(t, stripeKey, http.MethodGet,
				"/v1/payment_intents?customer="+url.QueryEscape(providerCustomerID)+"&limit=10", nil)
			candidates = map[string]paymentIntentCandidate{}
			if status == 200 {
				rows, _ := body["data"].([]any)
				preObservedIDs = paymentIntentIDSet(rows)
				var err error
				candidates, err = preSettlePaymentIntentCandidates(rows)
				if err != nil {
					t.Fatalf("invalid pre-settle PaymentIntent candidate: %v", err)
				}
			}
			if len(candidates) > 0 {
				var err error
				expectedIntentID, err = expectedPaymentIntentID(candidates)
				if err != nil {
					t.Fatalf("cannot resolve pre-settle PaymentIntent identity: %v", err)
				}
				break
			}
			if time.Now().After(waitDeadline) {
				t.Fatalf("timed out waiting for an unsettled invoice-linked PaymentIntent; last HTTP %d, observed IDs %v", status, paymentIntentIDs(body))
			}
			time.Sleep(3 * time.Second)
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
		intent, err := succeededPaymentIntent(rows, expectedIntentID, preObservedIDs)
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
