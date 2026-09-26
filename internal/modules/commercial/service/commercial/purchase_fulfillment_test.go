package commercial

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Tencent/WeKnora/internal/modules/commercial/payment"

	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	"github.com/Tencent/WeKnora/internal/modules/commercial/commercialplatform"
)

// ---- #82 Task 7: the purchase fulfiller orchestration (D2'/D4/D6'/D7) ----

// setupPurchaseFulfillment wires one sqlite store + the deterministic fake
// platform + the purchase fulfiller; purchase orders are seeded through the
// REAL ConfirmPayment path (a paid order + one fulfill outbox event, the
// state a crashed worker leaves behind).
func setupPurchaseFulfillment(t *testing.T) (*PurchaseFulfiller, *FulfillmentService, *commercialplatform.FakeAdapter, *gorm.DB, *repocommercial.OrderStore) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repocommercial.OrderRow{}, &repocommercial.PaymentAttemptRow{},
		&repocommercial.OutboxEvent{}, &repocommercial.PlanRow{}, &repocommercial.QuoteRow{},
		&repocommercial.PublicationRow{}, &repocommercial.Subscription{}, &repocommercial.BillingAccount{}); err != nil {
		t.Fatal(err)
	}
	fake := commercialplatform.NewFakeAdapter()
	store := repocommercial.NewOrderStore(db)
	purchaser, err := NewPurchaseFulfiller(db, fake, nil)
	if err != nil {
		t.Fatal(err)
	}
	gw := &stubGateway{findable: true}
	svc, err := NewFulfillmentService(db, gw, purchaser)
	if err != nil {
		t.Fatal(err)
	}
	return purchaser, svc, fake, db, store
}

// seedPaidPurchase seeds one paid purchase order: quote snapshot + publication
// + real CreateOrder/RegisterAttempt/ConfirmPayment — leaving the paid row,
// the succeeded attempt (provider txn recorded) and one fulfill outbox event.
func seedPaidPurchase(t *testing.T, store *repocommercial.OrderStore, db *gorm.DB, tenant uint64, orderID, planKey, planCode string, amountFen int64) {
	t.Helper()
	ctx := context.Background()
	if err := db.Create(&repocommercial.QuoteRow{
		ID: "q-" + orderID, TenantID: tenant, SubscriptionVersion: 1,
		SnapshotJSON: mustJSON(t, map[string]any{
			"plan_key": planKey, "plan_version": int64(1), "price_fen": amountFen,
			"credits_micro": int64(9_900_000), "currency": domain.CurrencyCNY,
			"line_items": []map[string]any{{"kind": "subscription_fee", "name": planKey, "amount_fen": amountFen}},
		}),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&repocommercial.PublicationRow{
		CommandKey: "publish_test:" + planKey + ":1", PlanKey: planKey, Version: 1, PlanCode: planCode,
		ReceiptJSON: "{}", PublishedAt: time.Now().UTC(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.CreateOrder(ctx, repocommercial.OrderRow{
		ID: orderID, TenantID: tenant, QuoteID: "q-" + orderID, Kind: domain.OrderKindPurchase,
		AmountFen: amountFen, Currency: domain.CurrencyCNY,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.RegisterAttempt(ctx, repocommercial.PaymentAttemptRow{
		ID: "att-" + orderID, TenantID: tenant, OrderID: orderID, Provider: "alipay", Merchant: "weknora",
		MerchantOrderID: "mo-" + orderID, AmountFen: amountFen, Currency: domain.CurrencyCNY,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ConfirmPayment(ctx, domain.PaymentFact{
		TenantID: tenant, OrderID: orderID, AttemptID: "mo-" + orderID, Provider: "alipay", Merchant: "weknora",
		Transaction: "txn-" + orderID, Amount: domain.CNYFen(amountFen), Currency: domain.CurrencyCNY, State: "succeeded",
	}); err != nil {
		t.Fatal(err)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// primeFakePurchaseWithFees activates the fake purchase subscription and
// injects the finalized invoice lines (the D6' review input).
func primeFakePurchaseWithFees(t *testing.T, fake *commercialplatform.FakeAdapter, tenant uint64, planCode string, amountFen int64) {
	t.Helper()
	ext := domain.ExternalPurchaseSubscriptionID(tenant)
	if _, err := fake.SubmitCommand(context.Background(), domain.Command{
		Kind: domain.CommandKindCreatePurchaseSubscription,
		Key:  domain.CreatePurchaseSubscriptionCommandKey(ext, planCode),
		Payload: domain.CreatePurchaseSubscriptionPayload{
			TenantID: tenant, ExternalCustomerID: domain.ExternalCustomerID(tenant),
			ExternalPurchaseSubscriptionID: ext, PlanCode: planCode,
			AmountFen: amountFen, Currency: domain.CurrencyCNY,
		},
	}); err != nil {
		t.Fatal(err)
	}
	fake.SetPurchaseInvoiceFees(ext, []domain.InvoiceLineSnapshot{{
		Kind: "subscription_fee", Name: planCode + " Plan", AmountFen: amountFen,
	}})
}

func TestPurchaseFulfillSettlesGrantsAndFulfills(t *testing.T) {
	_, svc, fake, db, store := setupPurchaseFulfillment(t)
	const tenant = uint64(61)
	seedPaidPurchase(t, store, db, tenant, "ord-61", "weknora-pro", "pub-pro-1", 9900)
	primeFakePurchaseWithFees(t, fake, tenant, "pub-pro-1", 9900)

	if err := svc.Recover(context.Background()); err != nil {
		t.Fatalf("drain: %v", err)
	}
	if state := orderState(t, db, "ord-61"); state != domain.OrderStateFulfilled {
		t.Fatalf("order must be fulfilled after the drain, got %q", state)
	}
	snap, err := fake.ReadSnapshot(context.Background(), domain.SnapshotQuery{Kind: domain.SnapshotKindPurchase, TenantID: tenant})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Purchase.State != domain.PurchaseStateActive {
		t.Fatalf("purchase must be active after settle, got %q", snap.Purchase.State)
	}
	// The purchase_activation record carries the grant receipt.
	recs := fulfillmentRecords(t, db, "ord-61")
	if len(recs) != 1 || recs[0].Kind != "purchase_activation" ||
		recs[0].State != domain.FulfillmentStateApplied || recs[0].ExternalID == "" {
		t.Fatalf("purchase_activation record mismatch: %+v", recs)
	}
	// The purchase wallet grant landed under its own identity.
	wallets := fake.Wallets()
	if len(wallets) != 1 {
		t.Fatalf("exactly one purchase wallet grant expected, got %d", len(wallets))
	}
	prefix := domain.ExternalPurchaseSubscriptionID(tenant) + "-"
	if !strings.HasPrefix(wallets[0].Name, prefix) ||
		!regexp.MustCompile(`^\d{4}-\d{2}$`).MatchString(strings.TrimPrefix(wallets[0].Name, prefix)) {
		t.Fatalf("grant must use the purchase wallet identity (<purchase>-<YYYY-MM>), got %q", wallets[0].Name)
	}
}

func TestPurchaseFulfillOverPaymentNoSecondBenefit(t *testing.T) {
	_, svc, fake, db, store := setupPurchaseFulfillment(t)
	const tenant = uint64(62)
	seedPaidPurchase(t, store, db, tenant, "ord-62", "weknora-pro", "pub-pro-2", 9900)
	primeFakePurchaseWithFees(t, fake, tenant, "pub-pro-2", 9900)
	if err := svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	// A SECOND, different channel transaction succeeds on the same order
	// (the user double-pays): the channel layer records ONLY an over-payment
	// audit fact — the fulfillment right is never duplicated.
	if err := store.ConfirmPayment(context.Background(), domain.PaymentFact{
		TenantID: tenant, OrderID: "ord-62", AttemptID: "mo-ord-62", Provider: "alipay", Merchant: "weknora",
		Transaction: "txn-DIFFERENT", Amount: domain.CNYFen(9900), Currency: domain.CurrencyCNY, State: "succeeded",
	}); err != nil {
		t.Fatalf("over-payment is an audit fact, not an error: %v", err)
	}
	if events := fulfillEvents(t, db); len(events) != 1 {
		t.Fatalf("no second fulfill event may appear, got %d", len(events))
	}
	if err := svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := len(fake.Wallets()); n != 1 {
		t.Fatalf("over-payment must not double-grant, wallets=%d", n)
	}
	if state := orderState(t, db, "ord-62"); state != domain.OrderStateFulfilled {
		t.Fatalf("state drift: %q", state)
	}
}

func TestPurchaseFulfillCrashReplayConverges(t *testing.T) {
	purchaser, _, fake, db, store := setupPurchaseFulfillment(t)
	const tenant = uint64(63)
	seedPaidPurchase(t, store, db, tenant, "ord-63", "weknora-pro", "pub-pro-3", 9900)
	primeFakePurchaseWithFees(t, fake, tenant, "pub-pro-3", 9900)
	events := fulfillEvents(t, db)
	if len(events) != 1 {
		t.Fatalf("seed must leave one fulfill event, got %d", len(events))
	}
	// Crash model: run Fulfill TWICE on the same event (the second run is the
	// recovery replay after the first process died post-settle).
	for i := 0; i < 2; i++ {
		if err := purchaser.Fulfill(context.Background(), events[0]); err != nil {
			t.Fatalf("fulfill pass %d: %v", i+1, err)
		}
	}
	if state := orderState(t, db, "ord-63"); state != domain.OrderStateFulfilled {
		t.Fatalf("replay must converge to fulfilled, got %q", state)
	}
	if n := len(fake.Wallets()); n != 1 {
		t.Fatalf("replay must grant exactly once, wallets=%d", n)
	}
	recs := fulfillmentRecords(t, db, "ord-63")
	if len(recs) != 1 || recs[0].State != domain.FulfillmentStateApplied {
		t.Fatalf("purchase_activation must be applied exactly once: %+v", recs)
	}
}

func TestPurchaseFulfillCanceledStaysClosed(t *testing.T) {
	_, svc, fake, db, store := setupPurchaseFulfillment(t)
	const tenant = uint64(64)
	seedPaidPurchase(t, store, db, tenant, "ord-64", "weknora-pro", "pub-pro-4", 9900)
	// The authority canceled the purchase (timeout boundary): the local
	// order is paid, but the benefit must NEVER open.
	ext := domain.ExternalPurchaseSubscriptionID(tenant)
	if _, err := fake.SubmitCommand(context.Background(), domain.Command{
		Kind: domain.CommandKindCreatePurchaseSubscription,
		Key:  domain.CreatePurchaseSubscriptionCommandKey(ext, "pub-pro-4"),
		Payload: domain.CreatePurchaseSubscriptionPayload{
			TenantID: tenant, ExternalCustomerID: domain.ExternalCustomerID(tenant),
			ExternalPurchaseSubscriptionID: ext, PlanCode: "pub-pro-4",
			AmountFen: 9900, Currency: domain.CurrencyCNY,
		},
	}); err != nil {
		t.Fatal(err)
	}
	fake.CancelPurchase(ext)
	if err := svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := len(fake.Wallets()); n != 0 {
		t.Fatalf("a canceled purchase must never grant, wallets=%d", n)
	}
	if state := orderState(t, db, "ord-64"); state == domain.OrderStateFulfilled {
		t.Fatal("a canceled purchase must never reach fulfilled")
	}
	recs := fulfillmentRecords(t, db, "ord-64")
	if len(recs) != 1 || recs[0].State != domain.FulfillmentStateAttention {
		t.Fatalf("canceled purchase must surface attention, got %+v", recs)
	}
}

func TestPurchaseFulfillShortWindowDoesNotBlockDrain(t *testing.T) {
	purchaser, _, fake, db, store := setupPurchaseFulfillment(t)
	const tenant = uint64(65)
	seedPaidPurchase(t, store, db, tenant, "ord-65", "weknora-pro", "pub-pro-5", 9900)
	// The purchase exists but NEVER activates (no settle rail outcome): the
	// first-pass observation window must return without blocking for the
	// full budget.
	ext := domain.ExternalPurchaseSubscriptionID(tenant)
	if _, err := fake.SubmitCommand(context.Background(), domain.Command{
		Kind: domain.CommandKindCreatePurchaseSubscription,
		Key:  domain.CreatePurchaseSubscriptionCommandKey(ext, "pub-pro-5"),
		Payload: domain.CreatePurchaseSubscriptionPayload{
			TenantID: tenant, ExternalCustomerID: domain.ExternalCustomerID(tenant),
			ExternalPurchaseSubscriptionID: ext, PlanCode: "pub-pro-5",
			AmountFen: 9900, Currency: domain.CurrencyCNY,
		},
	}); err != nil {
		t.Fatal(err)
	}
	events := fulfillEvents(t, db)
	start := time.Now()
	if err := purchaser.Fulfill(context.Background(), events[0]); err != nil {
		t.Fatalf("first pass must return nil (event stays pending), got %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 30*time.Second {
		t.Fatalf("the first-pass observation window must stay short, took %s", elapsed)
	}
	// The order is NOT fulfilled and the grant NEVER landed — but a SECOND
	// (top-up-shaped) event on the same drain would proceed; modeled here by
	// the caller looping without deadlock.
	if n := len(fake.Wallets()); n != 0 {
		t.Fatalf("no grant before activation, wallets=%d", n)
	}
	if state := orderState(t, db, "ord-65"); state != domain.OrderStatePaid {
		t.Fatalf("order stays paid while activating, got %q", state)
	}
}

func TestPurchaseGrantPeriodTakesActivationMonth(t *testing.T) {
	purchaser, _, fake, db, store := setupPurchaseFulfillment(t)
	const tenant = uint64(66)
	seedPaidPurchase(t, store, db, tenant, "ord-66", "weknora-pro", "pub-pro-6", 9900)
	primeFakePurchaseWithFees(t, fake, tenant, "pub-pro-6", 9900)
	events := fulfillEvents(t, db)
	// Order placed at Aug 31 23:35 UTC; activation observed Sep 1 00:05 —
	// the first-period grant must ride SEPTEMBER (the activation month),
	// never August (whose period end is already past and would refuse).
	purchaser.now = func() time.Time { return time.Date(2026, 9, 1, 0, 5, 0, 0, time.UTC) }
	if err := purchaser.Fulfill(context.Background(), events[0]); err != nil {
		t.Fatal(err)
	}
	wallets := fake.Wallets()
	if len(wallets) != 1 {
		t.Fatalf("one grant expected, got %d", len(wallets))
	}
	want := domain.PurchaseWalletName(tenant, "2026-09")
	if wallets[0].Name != want {
		t.Fatalf("grant wallet must take the ACTIVATION month identity %q, got %q", want, wallets[0].Name)
	}
	end, _ := domain.PeriodEnd("2026-09")
	if !wallets[0].ExpiresAt.Equal(end) {
		t.Fatalf("grant expiry must be the activation period end %s, got %s", end, wallets[0].ExpiresAt)
	}
}

func TestPaidAwaitingActivationSynthesized(t *testing.T) {
	// D3: the coordinator composes paid_awaiting_activation — a local paid
	// order + an authority snapshot still awaiting payment.
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repocommercial.OrderRow{}, &repocommercial.PaymentAttemptRow{},
		&repocommercial.OutboxEvent{}, &repocommercial.PlanRow{}, &repocommercial.QuoteRow{},
		&repocommercial.PublicationRow{}, &repocommercial.Subscription{}, &repocommercial.BillingAccount{}); err != nil {
		t.Fatal(err)
	}
	fake := commercialplatform.NewFakeAdapter()
	accounts, err := NewBillingAccountService(db, fake)
	if err != nil {
		t.Fatal(err)
	}
	plans, err := NewPlanVersionService(db, fake)
	if err != nil {
		t.Fatal(err)
	}
	ordersSvc, err := NewOrderService(db, map[string]payment.Provider{})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := NewPurchaseService(db, accounts, plans, ordersSvc, fake)
	if err != nil {
		t.Fatal(err)
	}
	store := repocommercial.NewOrderStore(db)
	const tenant = uint64(67)
	seedPaidPurchase(t, store, db, tenant, "ord-67", "weknora-pro", "pub-pro-7", 9900)
	// The authority still holds the gated subscription incomplete.
	if _, err := fake.SubmitCommand(context.Background(), domain.Command{
		Kind: domain.CommandKindCreatePurchaseSubscription,
		Key:  domain.CreatePurchaseSubscriptionCommandKey(domain.ExternalPurchaseSubscriptionID(tenant), "pub-pro-7"),
		Payload: domain.CreatePurchaseSubscriptionPayload{
			TenantID: tenant, ExternalCustomerID: domain.ExternalCustomerID(tenant),
			ExternalPurchaseSubscriptionID: domain.ExternalPurchaseSubscriptionID(tenant),
			PlanCode: "pub-pro-7", AmountFen: 9900, Currency: domain.CurrencyCNY,
		},
	}); err != nil {
		t.Fatal(err)
	}
	view, err := svc.PurchaseStatus(context.Background(), tenant)
	if err != nil {
		t.Fatal(err)
	}
	if view.State != domain.PurchaseStatePaidAwaitingActivation {
		t.Fatalf("paid local order + awaiting authority = paid_awaiting_activation, got %q", view.State)
	}
	// After activation the authority truth wins: active.
	fake.ActivatePurchase(domain.ExternalPurchaseSubscriptionID(tenant))
	view, err = svc.PurchaseStatus(context.Background(), tenant)
	if err != nil {
		t.Fatal(err)
	}
	if view.State != domain.PurchaseStateActive {
		t.Fatalf("active authority must surface active, got %q", view.State)
	}
}
