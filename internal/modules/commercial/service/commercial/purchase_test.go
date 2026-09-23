package commercial

import (
	"context"
	"errors"
	"testing"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/modules/commercial"
	"github.com/Tencent/WeKnora/internal/modules/commercial/commercialplatform"
	"github.com/Tencent/WeKnora/internal/modules/commercial/payment"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newPurchaseTestEnv(t *testing.T) (*PurchaseService, *commercialplatform.FakeAdapter, *stubCheckoutProvider, *gorm.DB) {
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
		&repocommercial.Subscription{}, &repocommercial.BillingAccount{}); err != nil {
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
	provider := &stubCheckoutProvider{queryState: payment.StateSucceeded}
	orders, err := NewOrderService(db, map[string]payment.Provider{"wechat": provider})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := NewPurchaseService(db, accounts, plans, orders, fake)
	if err != nil {
		t.Fatal(err)
	}
	return svc, fake, provider, db
}

// purchaseSeedPlan 走 #79 真实 draft→publish 流程落一个可购版本（含
// publication 行）。
func purchaseSeedPlan(t *testing.T, plans *PlanVersionService, key string, amountFen int64) {
	t.Helper()
	view, err := plans.CreateDraft(context.Background(), "test:seed", DraftInput{
		PlanKey: key, Name: key + " Plan", AmountFen: amountFen, IncludedCreditsMicro: 9_900_000,
		Features: map[string]bool{"advanced_models": true}, Currency: commercial.CurrencyCNY,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plans.Publish(context.Background(), "test:seed", "seed", key, view.Version); err != nil {
		t.Fatal(err)
	}
}

func purchaseQuote(t *testing.T, orders *OrderService, tenant uint64, planKey string) QuoteView {
	t.Helper()
	q, err := orders.CreateQuote(context.Background(), tenant, planKey)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func TestPurchaseHappyPathCreatesAwaitingPayment(t *testing.T) {
	svc, _, cp, _ := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	q := purchaseQuote(t, svc.orders, 31, "pro")
	view, err := svc.Purchase(context.Background(), 31, q.ID, "wechat", "billing-admin")
	if err != nil {
		t.Fatal(err)
	}
	if view.State != commercial.PurchaseStateAwaitingPayment || view.Order == nil || view.Order.CheckoutURL == "" {
		t.Fatalf("view = %+v", view)
	}
	if len(cp.createCalls) != 1 {
		t.Fatalf("channel creates = %d, want 1", len(cp.createCalls))
	}
}

func TestPurchaseAbortsOnInvoiceMismatch(t *testing.T) { // AC2
	svc, fake, cp, _ := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	q := purchaseQuote(t, svc.orders, 32, "pro")
	// 造偏差：先把订阅按 8800 建到 fake（模拟权威面金额与 Quote 不一致——
	// 如本地目录与权威目录漂移）。
	if _, err := fake.SubmitCommand(context.Background(), commercial.Command{
		Kind: commercial.CommandKindCreatePurchaseSubscription,
		Key: commercial.CreatePurchaseSubscriptionCommandKey(
			commercial.ExternalPurchaseSubscriptionID(32), commercial.DeterministicPlanCode("pro", 1)),
		Payload: commercial.CreatePurchaseSubscriptionPayload{
			TenantID: 32, ExternalCustomerID: commercial.ExternalCustomerID(32),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(32),
			PlanCode:                       commercial.DeterministicPlanCode("pro", 1),
			AmountFen:                      8800, Currency: commercial.CurrencyCNY,
		},
	}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Purchase(context.Background(), 32, q.ID, "wechat", "billing-admin")
	if !errors.Is(err, ErrInvoiceQuoteMismatch) {
		t.Fatalf("mismatch must abort, got %v", err)
	}
	if len(cp.createCalls) != 0 {
		t.Fatalf("NO channel payment request may be created on mismatch, got %d", len(cp.createCalls))
	}
}

func TestPurchaseExpiredQuoteRejected(t *testing.T) { // AC4 过期
	svc, _, _, db := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	q := purchaseQuote(t, svc.orders, 33, "pro")
	// 参数绑定过期（S2：无拼接 SQL）。
	if err := db.Exec(`UPDATE commercial_quotes SET expires_at = ? WHERE id = ?`,
		time.Now().Add(-time.Minute), q.ID).Error; err != nil {
		t.Fatal(err)
	}
	_, err := svc.Purchase(context.Background(), 33, q.ID, "wechat", "x")
	if !errors.Is(err, repocommercial.ErrQuoteExpired) {
		t.Fatalf("expired quote must be rejected, got %v", err)
	}
}

func TestPurchaseRetryReturnsExistingOrderWithoutDuplicates(t *testing.T) { // AC4 重试
	svc, fake, cp, _ := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	q := purchaseQuote(t, svc.orders, 34, "pro")
	first, err := svc.Purchase(context.Background(), 34, q.ID, "wechat", "a")
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Purchase(context.Background(), 34, q.ID, "wechat", "a")
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if first.Order.ID != second.Order.ID {
		t.Fatalf("retry must return the SAME order, got %q then %q", first.Order.ID, second.Order.ID)
	}
	if len(cp.createCalls) != 1 {
		t.Fatalf("retry must not open a second channel request, creates=%d", len(cp.createCalls))
	}
	if n := len(fake.PurchaseSubscriptions()); n != 1 {
		t.Fatalf("retry must not create a second subscription, got %d", n)
	}
}

func TestPurchaseConcurrentPlanChangeConflicts(t *testing.T) { // AC4 并发套餐变更
	svc, fake, cp, _ := newPurchaseTestEnv(t)
	// 两个梯位价格（ladder：base@0 / pro@9900 / pro-max@29900）。
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	purchaseSeedPlan(t, svc.plans, "pro-max", 29900)
	qPro := purchaseQuote(t, svc.orders, 35, "pro")
	qMax := purchaseQuote(t, svc.orders, 35, "pro-max")
	if _, err := svc.Purchase(context.Background(), 35, qPro.ID, "wechat", "a"); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Purchase(context.Background(), 35, qMax.ID, "wechat", "a")
	if !errors.Is(err, ErrPurchasePlanConflict) && !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("second concurrent plan change must conflict, got %v", err)
	}
	if n := len(fake.PurchaseSubscriptions()); n != 1 {
		t.Fatalf("exactly one subscription, got %d", n)
	}
	if len(cp.createCalls) != 1 {
		t.Fatalf("exactly one channel request, got %d", len(cp.createCalls))
	}
}
