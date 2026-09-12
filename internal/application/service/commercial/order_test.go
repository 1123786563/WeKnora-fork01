package commercial

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/payment"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// stubCheckoutProvider fakes the channel boundary for the order pipeline
// tests: Create returns a checkout URL (or a configured error), Query
// returns the configured attempt state.
type stubCheckoutProvider struct {
	mu          sync.Mutex
	queryState  payment.AttemptState
	createErr   error
	createCalls []string
	queryCalls  []string
}

func (p *stubCheckoutProvider) Create(_ context.Context, req payment.OrderRequest) (payment.AttemptResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.createCalls = append(p.createCalls, req.MerchantOrderID)
	if p.createErr != nil {
		return payment.AttemptResult{}, p.createErr
	}
	return payment.AttemptResult{State: payment.StatePending, ProviderID: req.MerchantOrderID, CheckoutURL: "https://pay.example/qr"}, nil
}
func (p *stubCheckoutProvider) Query(_ context.Context, id string) (payment.AttemptResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.queryCalls = append(p.queryCalls, id)
	return payment.AttemptResult{State: p.queryState, ProviderID: "txn_" + id}, nil
}
func (p *stubCheckoutProvider) Close(context.Context, string) error { return nil }
func (p *stubCheckoutProvider) Verify(context.Context, http.Header, []byte) (domain.PaymentFact, error) {
	return domain.PaymentFact{}, nil
}
func (p *stubCheckoutProvider) Refund(context.Context, payment.RefundRequest) (payment.RefundResult, error) {
	return payment.RefundResult{State: payment.StatePending}, nil
}
func (p *stubCheckoutProvider) QueryRefund(_ context.Context, id string) (payment.RefundResult, error) {
	return payment.RefundResult{State: payment.StatePending, ProviderID: id}, nil
}

func newOrderTestEnv(t *testing.T) (*OrderService, *stubCheckoutProvider, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repocommercial.OrderRow{}, &repocommercial.PaymentAttemptRow{},
		&repocommercial.OutboxEvent{}, &repocommercial.PlanRow{}, &repocommercial.QuoteRow{}, &repocommercial.Subscription{}); err != nil {
		t.Fatal(err)
	}
	provider := &stubCheckoutProvider{queryState: payment.StateSucceeded}
	svc, err := NewOrderService(db, map[string]payment.Provider{"wechat": provider})
	if err != nil {
		t.Fatal(err)
	}
	return svc, provider, db
}

func seedPublishedPlan(t *testing.T, db *gorm.DB) {
	t.Helper()
	def, _ := json.Marshal(domain.PlanVersion{Key: "pro", Version: 3, Price: 99_00, Monthly: 9_900_000})
	if err := db.Create(&repocommercial.PlanRow{
		PlanKey: "pro", Version: 3, DefinitionJSON: string(def),
		ExternalID: "ext-pro-3", State: domain.PlanStatePublished,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func TestOrderPipelineQuoteOrderRecover(t *testing.T) {
	svc, provider, _ := newOrderTestEnv(t)
	seedPublishedPlan(t, svc.db)
	ctx := context.Background()

	// Quote: exact price from the published definition.
	q, err := svc.CreateQuote(ctx, 101, "pro")
	if err != nil {
		t.Fatal(err)
	}
	if q.AmountFen != 99_00 || q.CreditsMicro != 9_900_000 {
		t.Fatalf("quote drifted: %+v", q)
	}

	// Order: pending, checkout URL produced, quote consumed.
	order, err := svc.CreateOrder(ctx, 101, q.ID, "wechat")
	if err != nil {
		t.Fatal(err)
	}
	if order.State != domain.OrderStatePending || order.CheckoutURL == "" {
		t.Fatalf("unexpected order: %+v", order)
	}
	if _, err := svc.CreateOrder(ctx, 101, q.ID, "wechat"); !errors.Is(err, repocommercial.ErrQuoteAlreadyUsed) {
		t.Fatalf("quote reuse: %v", err)
	}

	// Recovery: channel says succeeded — the order becomes paid through the
	// standard confirmation, exactly once.
	got, err := svc.RecoverOrderStatus(ctx, 101, order.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != domain.OrderStatePaid {
		t.Fatalf("recovery state = %s, want paid", got.State)
	}
	got, err = svc.RecoverOrderStatus(ctx, 101, order.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != domain.OrderStatePaid {
		t.Fatalf("second recovery state = %s, want paid", got.State)
	}
	if len(provider.queryCalls) != 1 {
		t.Fatalf("paid order re-queried the channel %d times", len(provider.queryCalls))
	}

	// Space isolation: another space can neither see nor recover the order.
	if _, err := svc.RecoverOrderStatus(ctx, 202, order.ID); !errors.Is(err, ErrOrderTenantMismatch) {
		t.Fatalf("cross-space recovery: %v", err)
	}
	list, err := svc.ListOrders(ctx, 202)
	if err != nil || len(list) != 0 {
		t.Fatalf("cross-space list leak: %v %+v", err, list)
	}
}

func TestOrderQuoteExpiryAndUnconfiguredProvider(t *testing.T) {
	svc, _, db := newOrderTestEnv(t)
	seedPublishedPlan(t, svc.db)
	ctx := context.Background()

	// Unknown plan → not found.
	if _, err := svc.CreateQuote(ctx, 101, "nope"); !errors.Is(err, repocommercial.ErrPlanNotFound) {
		t.Fatalf("unknown plan: %v", err)
	}
	q, err := svc.CreateQuote(ctx, 101, "pro")
	if err != nil {
		t.Fatal(err)
	}
	// Unconfigured channel fails EXPLICITLY and burns nothing recoverable:
	// the order row is removed, the quote stays unused.
	if _, err := svc.CreateOrder(ctx, 101, q.ID, "alipay"); !errors.Is(err, ErrPaymentProviderUnconfigured) {
		t.Fatalf("unconfigured provider: %v", err)
	}
	var used int64
	if err := db.Model(&repocommercial.QuoteRow{}).Where("id = ?", q.ID).
		Where("used_order_id IS NULL").Count(&used).Error; err != nil || used != 1 {
		t.Fatalf("quote consumed by failed checkout: used=%d err=%v", used, err)
	}
	// Expired quote cannot settle: backdate it, then order with the
	// configured provider — consumption must refuse.
	if err := db.Model(&repocommercial.QuoteRow{}).Where("id = ?", q.ID).
		Update("expires_at", time.Now().Add(-time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateOrder(ctx, 101, q.ID, "wechat"); err == nil {
		t.Fatal("expired quote accepted")
	}
}
