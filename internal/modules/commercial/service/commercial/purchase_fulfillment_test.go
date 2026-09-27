package commercial

import (
	"context"
	"encoding/json"
	"fmt"
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

// ---- code-review round 1, R82-1: the D7 total budget actually bites ----

// TestPurchaseFulfillBudgetExceededTurnsAttention: once the event's
// cumulative drain passes exhaust the total budget (attempt_count ×
// (drainInterval+firstPass) ≥ budget), a still-unactivated purchase must
// surface attention — no grant, no endless first-pass observation — and
// the paid order stays recoverable (never fabricated).
func TestPurchaseFulfillBudgetExceededTurnsAttention(t *testing.T) {
	purchaser, _, fake, db, store := setupPurchaseFulfillment(t)
	const tenant = uint64(68)
	seedPaidPurchase(t, store, db, tenant, "ord-68", "weknora-pro", "pub-pro-b1", 9900)
	// The gated purchase exists but NEVER activates.
	if _, err := fake.SubmitCommand(context.Background(), domain.Command{
		Kind:  domain.CommandKindCreatePurchaseSubscription,
		Key:   domain.CreatePurchaseSubscriptionCommandKey(domain.ExternalPurchaseSubscriptionID(tenant), "pub-pro-b1"),
		Actor: "test", Reason: "purchase",
		Payload: domain.CreatePurchaseSubscriptionPayload{
			TenantID: tenant, ExternalCustomerID: domain.ExternalCustomerID(tenant),
			ExternalPurchaseSubscriptionID: domain.ExternalPurchaseSubscriptionID(tenant),
			PlanCode: "pub-pro-b1", AmountFen: 9900, Currency: domain.CurrencyCNY,
		},
	}); err != nil {
		t.Fatal(err)
	}
	events := fulfillEvents(t, db)
	if len(events) != 1 {
		t.Fatalf("one fulfill event expected, got %d", len(events))
	}
	// Past the cap: 40 passes × (30s+15s) = 30min ≥ the 10min budget.
	over := events[0]
	over.AttemptCount = 40
	start := time.Now()
	if err := purchaser.Fulfill(context.Background(), over); err != nil {
		t.Fatalf("over-budget pass must land attention cleanly, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("the budget path must not burn a first-pass window, took %s", elapsed)
	}
	if n := len(fake.Wallets()); n != 0 {
		t.Fatalf("an over-budget unactivated purchase must never grant, wallets=%d", n)
	}
	if state := orderState(t, db, "ord-68"); state != domain.OrderStatePaid {
		t.Fatalf("the order stays paid (recoverable), got %q", state)
	}
	recs := fulfillmentRecords(t, db, "ord-68")
	if len(recs) != 1 || recs[0].State != domain.FulfillmentStateAttention {
		t.Fatalf("the total budget must surface attention, got %+v", recs)
	}
}

// TestPurchaseFulfillBudgetRecoveryWhenActive: past the budget BUT the
// authority HAS activated (the operator fixed the webhook leg late) — the
// cheap snapshot probe keeps the recovery path open and fulfillment
// proceeds normally.
func TestPurchaseFulfillBudgetRecoveryWhenActive(t *testing.T) {
	purchaser, _, fake, db, store := setupPurchaseFulfillment(t)
	const tenant = uint64(69)
	seedPaidPurchase(t, store, db, tenant, "ord-69", "weknora-pro", "pub-pro-b2", 9900)
	primeFakePurchaseWithFees(t, fake, tenant, "pub-pro-b2", 9900)
	// The operator fixed the webhook leg late: the authority IS active now
	// (the fake's deterministic settle would also do it, but the budget
	// path must succeed WITHOUT another settle drive).
	fake.ActivatePurchase(domain.ExternalPurchaseSubscriptionID(tenant))
	events := fulfillEvents(t, db)
	over := events[0]
	over.AttemptCount = 40 // past the budget, but the authority IS active
	if err := purchaser.Fulfill(context.Background(), over); err != nil {
		t.Fatalf("recovery past the budget must fulfill, got %v", err)
	}
	if state := orderState(t, db, "ord-69"); state != domain.OrderStateFulfilled {
		t.Fatalf("an activated purchase fulfills past the budget, got %q", state)
	}
	if n := len(fake.Wallets()); n != 1 {
		t.Fatalf("the grant lands exactly once on recovery, wallets=%d", n)
	}
	recs := fulfillmentRecords(t, db, "ord-69")
	if len(recs) != 1 || recs[0].State != domain.FulfillmentStateApplied {
		t.Fatalf("the terminal applied record wins, got %+v", recs)
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

// TestPurchaseFulfillBudgetTransientSnapshotErrorKeepsPending（A-31 / F114）：
// 预算耗尽分支的快照探测失败必须分流——瞬时（unreachable/unconfigured）错误
// 保持 pending（return nil），与步骤②/⑤ 的处理一致；一笔恰好落在首个超预算
// pass 上的 429/网络抖动不得铸成终态 attention 记录（运营据其退款后权威再激
// 活会打开退款+发放双得窗口）。
func TestPurchaseFulfillBudgetTransientSnapshotErrorKeepsPending(t *testing.T) {
	purchaser, _, fake, db, store := setupPurchaseFulfillment(t)
	const tenant = uint64(81)
	seedPaidPurchase(t, store, db, tenant, "ord-81", "weknora-pro", "pub-pro-b31", 9900)
	// 权威面存在（awaiting）但注入瞬时读失败。
	if _, err := fake.SubmitCommand(context.Background(), domain.Command{
		Kind:  domain.CommandKindCreatePurchaseSubscription,
		Key:   domain.CreatePurchaseSubscriptionCommandKey(domain.ExternalPurchaseSubscriptionID(tenant), "pub-pro-b31"),
		Actor: "test", Reason: "purchase",
		Payload: domain.CreatePurchaseSubscriptionPayload{
			TenantID: tenant, ExternalCustomerID: domain.ExternalCustomerID(tenant),
			ExternalPurchaseSubscriptionID: domain.ExternalPurchaseSubscriptionID(tenant),
			PlanCode: "pub-pro-b31", AmountFen: 9900, Currency: domain.CurrencyCNY,
		},
	}); err != nil {
		t.Fatal(err)
	}
	fake.FailPurchaseSnapshotsWith(fmt.Errorf("%w: lab blip", domain.ErrPlatformUnreachable))
	events := fulfillEvents(t, db)
	over := events[0]
	over.AttemptCount = 40 // past the D7 budget
	if err := purchaser.Fulfill(context.Background(), over); err != nil {
		t.Fatalf("a transient snapshot blip on the over-budget probe must keep the event pending (nil), got %v", err)
	}
	if recs := fulfillmentRecords(t, db, "ord-81"); len(recs) != 0 {
		t.Fatalf("a transient probe failure must NOT mint a terminal attention record, got %+v", recs)
	}
	if state := orderState(t, db, "ord-81"); state != domain.OrderStatePaid {
		t.Fatalf("the order stays paid (recoverable), got %q", state)
	}
}

// TestPurchaseFulfillDefinitiveSnapshotErrorLandsAttentionWithoutAborting（A-32
// / F115）：预算耗尽后、settle 后观察窗与 D6' 复核的确定性（invalid_response）
// 快照失败落 attention 且返回 nil——绝不作为 error 中断共享排水 pass（Recover
// 对批内第一个 error 直接终止整轮，后续所有事件会饿死）。
func TestPurchaseFulfillDefinitiveSnapshotErrorLandsAttentionWithoutAborting(t *testing.T) {
	purchaser, _, fake, db, store := setupPurchaseFulfillment(t)
	const tenant = uint64(82)
	seedPaidPurchase(t, store, db, tenant, "ord-82", "weknora-pro", "pub-pro-b32", 9900)
	// 权威面存在（awaiting）——settle（fake 内部不走 ReadSnapshot）会成功，
	// 随后的 observeActivation 读快照拿到确定性失败。
	if _, err := fake.SubmitCommand(context.Background(), domain.Command{
		Kind:  domain.CommandKindCreatePurchaseSubscription,
		Key:   domain.CreatePurchaseSubscriptionCommandKey(domain.ExternalPurchaseSubscriptionID(tenant), "pub-pro-b32"),
		Actor: "test", Reason: "purchase",
		Payload: domain.CreatePurchaseSubscriptionPayload{
			TenantID: tenant, ExternalCustomerID: domain.ExternalCustomerID(tenant),
			ExternalPurchaseSubscriptionID: domain.ExternalPurchaseSubscriptionID(tenant),
			PlanCode: "pub-pro-b32", AmountFen: 9900, Currency: domain.CurrencyCNY,
		},
	}); err != nil {
		t.Fatal(err)
	}
	fake.FailPurchaseSnapshotsWith(fmt.Errorf("%w: multiple finalized purchase invoices", domain.ErrPlatformInvalidResponse))
	events := fulfillEvents(t, db)
	ev := events[0]
	if err := purchaser.Fulfill(context.Background(), ev); err != nil {
		t.Fatalf("a DEFINITIVE snapshot failure must land attention and return nil (never abort the shared drain), got %v", err)
	}
	recs := fulfillmentRecords(t, db, "ord-82")
	if len(recs) != 1 || recs[0].State != domain.FulfillmentStateAttention {
		t.Fatalf("a definitive snapshot failure must land the attention record, got %+v", recs)
	}
	if n := len(fake.Wallets()); n != 0 {
		t.Fatalf("a definitive failure never grants, wallets=%d", n)
	}
}

// TestPurchaseFulfillObservationTransientErrorKeepsPending（A-32）：观察窗
// （settle 之后的激活观察）的瞬时快照失败保持 pending——与确定性失败的
// attention 分流对照。
func TestPurchaseFulfillObservationTransientErrorKeepsPending(t *testing.T) {
	purchaser, _, fake, db, store := setupPurchaseFulfillment(t)
	const tenant = uint64(83)
	seedPaidPurchase(t, store, db, tenant, "ord-83", "weknora-pro", "pub-pro-b33", 9900)
	if _, err := fake.SubmitCommand(context.Background(), domain.Command{
		Kind:  domain.CommandKindCreatePurchaseSubscription,
		Key:   domain.CreatePurchaseSubscriptionCommandKey(domain.ExternalPurchaseSubscriptionID(tenant), "pub-pro-b33"),
		Actor: "test", Reason: "purchase",
		Payload: domain.CreatePurchaseSubscriptionPayload{
			TenantID: tenant, ExternalCustomerID: domain.ExternalCustomerID(tenant),
			ExternalPurchaseSubscriptionID: domain.ExternalPurchaseSubscriptionID(tenant),
			PlanCode: "pub-pro-b33", AmountFen: 9900, Currency: domain.CurrencyCNY,
		},
	}); err != nil {
		t.Fatal(err)
	}
	fake.FailPurchaseSnapshotsWith(fmt.Errorf("%w: authority rebooting", domain.ErrPlatformUnreachable))
	events := fulfillEvents(t, db)
	if err := purchaser.Fulfill(context.Background(), events[0]); err != nil {
		t.Fatalf("a transient observation failure must keep the event pending (nil return), got %v", err)
	}
	if recs := fulfillmentRecords(t, db, "ord-83"); len(recs) != 0 {
		t.Fatalf("a transient observation failure must not land any terminal record, got %+v", recs)
	}
}

// ---- #82 Task 14 (OCR r2): drain-shape guards ----

// seedPaidOrderRaw plants one paid order through the REAL CreateOrder /
// RegisterAttempt / ConfirmPayment chain with a fully controlled quote
// snapshot and an OPTIONAL publication row (the publication-missing test
// omits it).
func seedPaidOrderRaw(t *testing.T, store *repocommercial.OrderStore, db *gorm.DB,
	tenant uint64, orderID string, snapshotJSON string, amountFen int64, withPublication bool) {
	t.Helper()
	ctx := context.Background()
	if err := db.Create(&repocommercial.QuoteRow{
		ID: "q-" + orderID, TenantID: tenant, SubscriptionVersion: 1, SnapshotJSON: snapshotJSON,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if withPublication {
		if err := db.Create(&repocommercial.PublicationRow{
			CommandKey: "publish_raw:" + orderID, PlanKey: "weknora-pro", Version: 1, PlanCode: "pub-raw-1",
			ReceiptJSON: "{}", PublishedAt: time.Now().UTC(),
		}).Error; err != nil {
			t.Fatal(err)
		}
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

// TestPurchaseFulfillPublicationMissingTurnsAttentionNotAbort（r2:119）：
// 确定性数据失败（快照指向的 publication 行不存在——迁移缺口）不再上抛中止
// 共享 drain 批次：该购买事件落 attention（运营面），同批的普通充值事件照常
// 完成。修复前每轮 lease 到期即重复中止整趟 pass（延迟其后所有事件、永不自
// 愈、无 attention 记录）。
func TestPurchaseFulfillPublicationMissingTurnsAttentionNotAbort(t *testing.T) {
	_, svc, _, db, store := setupPurchaseFulfillment(t)
	const tenant = uint64(84)
	// 购买单：快照有 plan 面，但 publications 表无行（确定性数据缺口）。
	seedPaidOrderRaw(t, store, db, tenant, "ord-84", mustJSON(t, map[string]any{
		"plan_key": "weknora-pro", "plan_version": int64(1), "price_fen": int64(9900),
		"credits_micro": int64(9_900_000), "currency": domain.CurrencyCNY,
		"line_items": []map[string]any{{"kind": "subscription_fee", "name": "weknora-pro", "amount_fen": int64(9900)}},
	}), 9900, false)
	// 同批充值单：无 subscription_fee 行的 paid 订单（top-up 形态；不种
	// publication——top-up 路径不读它，且同 plan 的行会喂给购买单）。
	seedPaidOrderRaw(t, store, db, tenant, "ord-84t", mustJSON(t, map[string]any{
		"plan_key": "weknora-pro", "plan_version": int64(1), "price_fen": int64(9900),
		"credits_micro": int64(9_900_000), "currency": domain.CurrencyCNY,
		"line_items": []map[string]any{{"kind": "top_up", "name": "top-up", "amount_fen": int64(9900)}},
	}), 9900, false)

	if err := svc.Recover(context.Background()); err != nil {
		t.Fatalf("a deterministic publication gap must NOT abort the shared drain pass, got %v", err)
	}
	// 购买单：attention 记录（运营面），绝不 fulfilled。
	if state := orderState(t, db, "ord-84"); state == domain.OrderStateFulfilled {
		t.Fatal("a publication-missing purchase must never reach fulfilled")
	}
	recs := fulfillmentRecords(t, db, "ord-84")
	if len(recs) != 1 || recs[0].State != domain.FulfillmentStateAttention {
		t.Fatalf("a deterministic publication gap must surface attention, got %+v", recs)
	}
	// 同批充值单照常完成（不被购买事件的中止饥饿）。
	if state := orderState(t, db, "ord-84t"); state != domain.OrderStateFulfilled {
		t.Fatalf("the later top-up event in the same batch must still complete, got %q", state)
	}
}

// TestPurchaseFulfillQuoteSnapshotCorruptTurnsAttention（r2:119）：quote 的
// SnapshotJSON 非法（快照损坏——确定性形状）同姿态：attention + nil，不上抛、
// 不 panic、不中止共享 drain。
func TestPurchaseFulfillQuoteSnapshotCorruptTurnsAttention(t *testing.T) {
	purchaser, _, _, db, store := setupPurchaseFulfillment(t)
	const tenant = uint64(85)
	seedPaidOrderRaw(t, store, db, tenant, "ord-85", `{"plan_key": "weknora-pro", bad json`, 9900, true)
	events := fulfillEvents(t, db)
	if len(events) == 0 {
		t.Fatal("the seeded paid order must carry a fulfill event")
	}
	if err := purchaser.Fulfill(context.Background(), events[0]); err != nil {
		t.Fatalf("a corrupt quote snapshot must turn attention with a nil return (never abort the drain), got %v", err)
	}
	recs := fulfillmentRecords(t, db, "ord-85")
	if len(recs) != 1 || recs[0].State != domain.FulfillmentStateAttention {
		t.Fatalf("a corrupt quote snapshot must surface attention, got %+v", recs)
	}
}
