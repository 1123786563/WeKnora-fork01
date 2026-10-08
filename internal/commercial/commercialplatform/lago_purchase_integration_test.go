//go:build lago_integration

// Tagged real-Lago payment-gated purchase evidence (T09, #81). The build
// tag keeps this out of every normal suite run; the test is env-gated on
// LAGO_INTEGRATION_BASE_URL + LAGO_INTEGRATION_API_KEY plus a binding
// source (LAGO_INTEGRATION_STRIPE_API_KEY or
// LAGO_INTEGRATION_PROVIDER_CUSTOMER_PREFIX) and skips otherwise, so a
// missing stack records blocked-env instead of failing or faking a pass.
//
// Phases (ONE test, run in order; each maps an acceptance criterion):
//  1. (AC1) Publish pro v1 (9900 fen CNY, no charges, advanced_models) via
//     the #79 draft+publish flow, then cut a Quote and assert the frozen
//     face: CNY, a single subscription_fee line of 9900, frozen features,
//     a bounded expiry.
//  2. (AC3) The gated create through the REAL seam: the receipt echoes the
//     deterministic purchase identity; the purchase snapshot answers
//     awaiting_payment with the frozen price face.
//  3. (AC3) Entitlements stay CLOSED while gated: the customer entitlement
//     read answers 404 (raw probe — the F6 evidence shape).
//  4. (AC4) Replay: the same command key succeeds WITHOUT a second create;
//     the explicit-status[] index shows EXACTLY ONE subscription (t02
//     lesson: the index defaults to active).
//  5. (AC4) The replay left no second gating invoice candidate: the index
//     count for the identity is still one and the frozen face is unchanged
//     (the API never sees open invoices — F3-F5 — so the DB count check
//     lives in the Task 12 run evidence, not here).
//  6. (AC4) A different plan_code against the SAME identity is a
//     definitive conflict; the authority still holds exactly one
//     subscription. Tenant B runs the same chain and A's counts are
//     unchanged (isolation).
package commercialplatform

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/commercial/service/commercial"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLagoPurchaseIntegration(t *testing.T) {
	baseURL := os.Getenv("LAGO_INTEGRATION_BASE_URL")
	apiKey := os.Getenv("LAGO_INTEGRATION_API_KEY")
	stripeKey := os.Getenv("LAGO_INTEGRATION_STRIPE_API_KEY")
	prefix := os.Getenv("LAGO_INTEGRATION_PROVIDER_CUSTOMER_PREFIX")
	if baseURL == "" || apiKey == "" || (stripeKey == "" && prefix == "") {
		t.Skip("blocked-env: LAGO_INTEGRATION_BASE_URL / LAGO_INTEGRATION_API_KEY / binding source not set (operator stack not provided)")
	}
	cfg := Config{Provider: ProviderLago, BaseURL: baseURL, APIKey: apiKey, Release: lockedRelease(t)}
	// The integration stack is loopback; the #82 outbound host policy
	// (EnvOutboundAllowLoopback) requires the explicit dev bypass — same
	// as the sibling integration tests.
	cfg.OutboundAllowLoopback = strings.Contains(baseURL, "127.0.0.1") || strings.Contains(baseURL, "localhost")
	if stripeKey != "" {
		cfg.StripeAPIKey = stripeKey
		// F11: a provider test payment-method token so the binding ensure
		// can attach a default payment method (dev/test only — the token is
		// an operator env, never committed).
		cfg.StripePmToken = os.Getenv("LAGO_INTEGRATION_STRIPE_PM_TOKEN")
	} else {
		cfg.ProviderCustomerPrefix = prefix
	}
	p := NewLagoAdapter(cfg)
	ctx := context.Background()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	tenantA := uint64(810000) + uint64(rng.Intn(100000))
	tenantB := tenantA + 1_000_000

	// Services over in-memory sqlite (quote/publish only — no channel
	// adapter is wired here; the channel order chain is unit-covered).
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&repocommercial.OrderRow{}, &repocommercial.PaymentAttemptRow{},
		&repocommercial.OutboxEvent{}, &repocommercial.PlanRow{}, &repocommercial.QuoteRow{},
		&repocommercial.Subscription{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if _, err := commercialsvc.NewOrderService(db, nil); err != nil {
		t.Fatal(err)
	}
	plans, err := commercialsvc.NewPlanVersionService(db, p)
	if err != nil {
		t.Fatal(err)
	}
	orders, err := commercialsvc.NewOrderService(db, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Phase 1 (AC1): publish pro v1 through the real #79 flow, then quote.
	view, err := plans.CreateDraft(ctx, "integration:t09", commercialsvc.DraftInput{
		PlanKey: "pro", Name: "Pro", AmountFen: 9900, IncludedCreditsMicro: 9_900_000,
		Features: map[string]bool{"advanced_models": true}, Currency: commercial.CurrencyCNY,
	})
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if _, err := plans.Publish(ctx, "integration:t09", "t09 evidence", "pro", view.Version); err != nil {
		t.Fatalf("publish: %v", err)
	}
	q, err := orders.CreateQuote(ctx, tenantA, "pro")
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if q.Currency != commercial.CurrencyCNY || len(q.LineItems) != 1 ||
		q.LineItems[0].Kind != "subscription_fee" || q.LineItems[0].AmountFen != 9900 ||
		!q.Features["advanced_models"] || q.ExpiresAt == "" {
		t.Fatalf("frozen quote face mismatch: %+v", q)
	}

	purchaseCommand := func(tenant uint64, planCode string) commercial.Command {
		return commercial.Command{
			Kind:  commercial.CommandKindCreatePurchaseSubscription,
			Key:   commercial.CreatePurchaseSubscriptionCommandKey(commercial.ExternalPurchaseSubscriptionID(tenant), planCode),
			Actor: "integration:t09", Reason: "purchase evidence",
			Payload: commercial.CreatePurchaseSubscriptionPayload{
				TenantID: tenant, ExternalCustomerID: commercial.ExternalCustomerID(tenant),
				ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(tenant),
				PlanCode:                       planCode, AmountFen: 9900, Currency: commercial.CurrencyCNY,
			},
		}
	}

	// Phase 2 (AC3): the gated create through the REAL seam. The plan code
	// is the #79 deterministic derivation the publication recorded.
	planCode := commercial.DeterministicPlanCode("pro", view.Version)
	receipt, err := p.SubmitCommand(ctx, purchaseCommand(tenantA, planCode))
	if err != nil {
		t.Fatalf("gated create: %v", err)
	}
	if receipt.ExternalID != commercial.ExternalPurchaseSubscriptionID(tenantA) {
		t.Fatalf("receipt identity = %q", receipt.ExternalID)
	}
	snap, err := p.ReadSnapshot(ctx, commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: tenantA})
	if err != nil {
		t.Fatalf("purchase snapshot: %v", err)
	}
	purchase := snap.Purchase
	if purchase == nil || purchase.State != commercial.PurchaseStateAwaitingPayment ||
		purchase.AmountFen != 9900 || purchase.Currency != commercial.CurrencyCNY || len(purchase.InvoiceFees) != 0 {
		t.Fatalf("awaiting-payment snapshot mismatch: %+v", purchase)
	}

	// Phase 3 (AC3): entitlements stay CLOSED while gated (F6). Read
	// through the v1.53 SUBSCRIPTION entitlements route — the pinned
	// release exposes no customer-nested entitlements route (F-2), so the
	// old customers-path assertion was vacuously 404 on the real stack.
	status, body := rawIntegrationRequest(t, http.MethodGet, baseURL, apiKey,
		fmt.Sprintf("/api/v1/subscriptions/%s/entitlements", commercial.ExternalPurchaseSubscriptionID(tenantA)), "")
	if status == http.StatusOK && strings.Contains(body, "advanced_models") {
		t.Fatalf("gated tenant must hold no purchase entitlements (status %d), got %s", status, body)
	}

	// Phase 4 (AC4): replay resolves by identity, exactly one subscription.
	if _, err := p.SubmitCommand(ctx, purchaseCommand(tenantA, planCode)); err != nil {
		t.Fatalf("replay: %v", err)
	}
	if n := purchaseIdentityCount(t, baseURL, apiKey, tenantA); n != 1 {
		t.Fatalf("the authority must hold EXACTLY ONE purchase subscription after the replay, got %d", n)
	}

	// Phase 5 (AC4): the replay changed nothing observable.
	snap2, err := p.ReadSnapshot(ctx, commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: tenantA})
	if err != nil {
		t.Fatal(err)
	}
	if snap2.Purchase == nil || snap2.Purchase.State != commercial.PurchaseStateAwaitingPayment ||
		snap2.Purchase.AmountFen != 9900 {
		t.Fatalf("replay mutated the purchase face: %+v", snap2.Purchase)
	}

	// Phase 6 (AC4): a different plan on the same identity is a conflict.
	_, err = p.SubmitCommand(ctx, purchaseCommand(tenantA, "weknora-pro-max-v999999"))
	if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("concurrent plan change must conflict, got %v", err)
	}
	if n := purchaseIdentityCount(t, baseURL, apiKey, tenantA); n != 1 {
		t.Fatalf("conflict must not create, identity count = %d", n)
	}

	// Tenant isolation: B's chain runs fully; A's counts are unchanged.
	if _, err := p.SubmitCommand(ctx, purchaseCommand(tenantB, planCode)); err != nil {
		t.Fatalf("tenant B create: %v", err)
	}
	if n := purchaseIdentityCount(t, baseURL, apiKey, tenantA); n != 1 {
		t.Fatalf("tenant B must not touch A's objects, A count = %d", n)
	}
	if n := purchaseIdentityCount(t, baseURL, apiKey, tenantB); n != 1 {
		t.Fatalf("tenant B holds exactly one purchase, got %d", n)
	}
}

// purchaseIdentityCount probes the subscription index WITH the explicit
// status filter for the tenant's purchase identity (t02 lesson: without
// status[] the index answers active-only and incomplete is invisible).
func purchaseIdentityCount(t *testing.T, baseURL, apiKey string, tenant uint64) int {
	t.Helper()
	status, body := rawIntegrationRequest(t, http.MethodGet, baseURL, apiKey,
		fmt.Sprintf("/api/v1/subscriptions?external_id=%s&status%%5B%%5D=incomplete&status%%5B%%5D=active&status%%5B%%5D=canceled&status%%5B%%5D=terminated",
			commercial.ExternalPurchaseSubscriptionID(tenant)), "")
	if status != http.StatusOK {
		t.Fatalf("subscription index: status %d", status)
	}
	return strings.Count(body, `"external_id":"`+commercial.ExternalPurchaseSubscriptionID(tenant)+`"`)
}
