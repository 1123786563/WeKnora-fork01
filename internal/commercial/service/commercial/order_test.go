package commercial

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	domain "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/commercial/payment"
	repocommercial "github.com/Tencent/WeKnora/internal/commercial/repository/commercial"

	secutils "github.com/Tencent/WeKnora/internal/utils"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// stubCheckoutProvider fakes the channel boundary for the order pipeline
// tests: Create returns a checkout URL (or a configured error), Query
// returns the configured attempt state.
type stubCheckoutProvider struct {
	mu                   sync.Mutex
	queryState           payment.AttemptState
	queryAmountFen       int64  // #84/G2: the collected amount the channel reports (0 = not reported)
	queryCurrency        string // #84/G2 补: the collected currency the channel reports ("" = not reported)
	queryCurrencyUnknown bool
	createErr            error
	createCalls          []string
	queryCalls           []string
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
	currency := p.queryCurrency
	if currency == "" && p.queryAmountFen > 0 && !p.queryCurrencyUnknown {
		currency = "CNY"
	}
	return payment.AttemptResult{State: p.queryState, ProviderID: "txn_" + id, AmountFen: p.queryAmountFen, AmountCurrency: currency}, nil
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

// MerchantID models the channel merchant identity contract: a WeChat-shaped
// mchid that is deliberately NOT the provider name, so any attempt
// registration that writes the provider name instead fails the merchant
// assertions (issue #82 flow defect 2).
func (p *stubCheckoutProvider) MerchantID() string { return "1900000109" }

func newOrderTestEnv(t *testing.T) (*OrderService, *stubCheckoutProvider, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	closeSQLiteDBOnCleanup(t, db)
	if s, err := db.DB(); err != nil {
		t.Fatal(err)
	} else {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repocommercial.OrderRow{}, &repocommercial.PaymentAttemptRow{},
		&repocommercial.OutboxEvent{}, &repocommercial.PlanRow{}, &repocommercial.QuoteRow{},
		&repocommercial.Subscription{}, &repocommercial.PaymentAnomalyRow{}); err != nil {
		t.Fatal(err)
	}
	provider := &stubCheckoutProvider{queryState: payment.StateSucceeded, queryAmountFen: 9900, queryCurrency: "CNY"}
	svc, err := NewOrderService(db, map[string]payment.Provider{"wechat": provider})
	if err != nil {
		t.Fatal(err)
	}
	return svc, provider, db
}

func closeSQLiteDBOnCleanup(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
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
	svc, provider, db := newOrderTestEnv(t)
	seedPublishedPlan(t, db)
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
	seedPublishedPlan(t, db)
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

// TestOrderChannelFailureStillReturnsRecoverableOrder locks the write
// contract: a channel failure AFTER the atomic open leaves a durable
// pending order, and CreateOrder answers with the operation ID + state
// (CheckoutError set, nil error) so the client recovers through
// TestCheckoutURLPersistsAndReplaysVerbatim (R1-35): the channel link used
// to live ONLY in the first CreateOrder answer — a client that timed out or
// refreshed after the channel call lost the payment entry to an
// already-consumed quote. The link is now persisted on the order row and
// re-served verbatim by the recovery projection while the order is pending.
func TestCheckoutURLPersistsAndReplaysVerbatim(t *testing.T) {
	svc, provider, db := newOrderTestEnv(t)
	seedPublishedPlan(t, db)
	ctx := context.Background()
	q, err := svc.CreateQuote(ctx, 104, "pro")
	if err != nil {
		t.Fatal(err)
	}
	order, err := svc.CreateOrder(ctx, 104, q.ID, "wechat")
	if err != nil {
		t.Fatal(err)
	}
	if order.CheckoutURL == "" {
		t.Fatalf("first answer must carry the link: %+v", order)
	}
	// Persisted on the order row (the bounded update after the atomic unit).
	var stored repocommercial.OrderRow
	if err := db.Where("id = ?", order.ID).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.CheckoutURL != order.CheckoutURL {
		t.Fatalf("checkout_url must be persisted, row=%q answer=%q", stored.CheckoutURL, order.CheckoutURL)
	}
	// The recovery projection re-serves it verbatim while the channel is
	// still pending (queryState switched BEFORE the recover call).
	provider.mu.Lock()
	provider.queryState = payment.StatePending
	provider.mu.Unlock()
	got, err := svc.RecoverOrderStatus(ctx, 104, order.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != domain.OrderStatePending || got.CheckoutURL != order.CheckoutURL {
		t.Fatalf("pending recovery must re-serve the checkout link verbatim, got %+v (want %q)", got, order.CheckoutURL)
	}
}

// RecoverOrderStatus instead of retrying the consumed quote.
func TestOrderChannelFailureStillReturnsRecoverableOrder(t *testing.T) {
	svc, provider, db := newOrderTestEnv(t)
	seedPublishedPlan(t, db)
	provider.createErr = errors.New("channel timeout")
	ctx := context.Background()
	q, err := svc.CreateQuote(ctx, 101, "pro")
	if err != nil {
		t.Fatal(err)
	}
	order, err := svc.CreateOrder(ctx, 101, q.ID, "wechat")
	if err != nil {
		t.Fatalf("channel failure must surface on the view, not as an error: %v", err)
	}
	if order.ID == "" || order.State != domain.OrderStatePending || order.CheckoutError == "" {
		t.Fatalf("unrecoverable order answer: %+v", order)
	}
	// The quote is consumed: a retry would conflict, so the ID-carrying
	// answer is the ONLY recovery path — and it works.
	if _, err := svc.CreateOrder(ctx, 101, q.ID, "wechat"); !errors.Is(err, repocommercial.ErrQuoteAlreadyUsed) {
		t.Fatalf("quote not consumed by the failed checkout: %v", err)
	}
	provider.mu.Lock()
	provider.createErr = nil
	provider.queryState = payment.StateSucceeded
	provider.queryAmountFen, provider.queryCurrency = 9900, "CNY"
	provider.mu.Unlock()
	got, err := svc.RecoverOrderStatus(ctx, 101, order.ID)
	if err != nil || got.State != domain.OrderStatePaid {
		t.Fatalf("recovery after channel failure: %+v %v", got, err)
	}
}

func seedSecondPlan(t *testing.T, db *gorm.DB) {
	t.Helper()
	def, _ := json.Marshal(domain.PlanVersion{Key: "lite", Version: 2, Price: 19_00, Monthly: 1_900_000})
	if err := db.Create(&repocommercial.PlanRow{PlanKey: "lite", Version: 2, DefinitionJSON: string(def),
		ExternalID: "ext-lite-2", State: domain.PlanStatePublished}).Error; err != nil {
		t.Fatal(err)
	}
}

// cancellingCreateStub (R3-28): the channel Create fails BECAUSE the caller
// cancelled — the cancellation happens DURING the channel call (the client
// disconnected mid-checkout), exactly the shape whose row-persisting writes
// must survive the request's end.
type cancellingCreateStub struct {
	*stubCheckoutProvider
	onCreate func()
}

func (p *cancellingCreateStub) Create(_ context.Context, _ payment.OrderRequest) (payment.AttemptResult, error) {
	p.onCreate()
	return payment.AttemptResult{}, context.Canceled
}

// TestOpenOrderPersistsChannelFailurePastCallerCancellation（R3-28）：渠道
// Create 因调用方取消而失败时，channel_failed 标记的写入曾复用同一已取消
// ctx（标记丢失 → 僵尸 pending：channel_failed=false 且无链接，永久占用租户
// 的可付槽并阻塞一切新结账）。持久化写现在脱离请求取消——标记必须落地。
func TestOpenOrderPersistsChannelFailurePastCallerCancellation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	closeSQLiteDBOnCleanup(t, db)
	if s, err := db.DB(); err != nil {
		t.Fatal(err)
	} else {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repocommercial.OrderRow{}, &repocommercial.PaymentAttemptRow{},
		&repocommercial.OutboxEvent{}, &repocommercial.PlanRow{}, &repocommercial.QuoteRow{},
		&repocommercial.Subscription{}, &repocommercial.PaymentAnomalyRow{}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	base := &stubCheckoutProvider{queryState: payment.StatePending}
	wrapped := &cancellingCreateStub{stubCheckoutProvider: base, onCreate: cancel}
	svc, err := NewOrderService(db, map[string]payment.Provider{"wechat": wrapped})
	if err != nil {
		t.Fatal(err)
	}
	seedPublishedPlan(t, db)
	q, err := svc.CreateQuote(context.Background(), 110, "pro")
	if err != nil {
		t.Fatal(err)
	}
	view, err := svc.CreateOrder(ctx, 110, q.ID, "wechat")
	if err != nil {
		t.Fatalf("the channel-failure posture must still answer the write: %v", err)
	}
	if view.CheckoutError == "" {
		t.Fatalf("expected the channel-failure posture, got %+v", view)
	}
	var stored repocommercial.OrderRow
	if err := db.Where("id = ?", view.ID).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if !stored.ChannelFailed {
		t.Fatalf("the channel-failed mark must survive the caller's cancellation, row=%+v", stored)
	}
}

func seedChangePlanSubscription(t *testing.T, db *gorm.DB, tenantID uint64, plan domain.PlanVersion, anchor, paidUntil time.Time) {
	t.Helper()
	snap, _ := json.Marshal(plan)
	if err := db.Create(&repocommercial.Subscription{ID: "sub-" + strconv.FormatUint(tenantID, 10),
		TenantID: tenantID, PlanKey: plan.Key, PlanVersion: plan.Version, PlanSnapshotJSON: string(snap),
		Anchor: anchor, PaidUntil: paidUntil, FutureIntervalJSON: "{}", Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
}

// TestChangePlanUpgradeProratesRemainingPeriod: an upgrade settles as an
// order for the price difference prorated over the REMAINING paid span
// (B17): half the period left on a 50.00 difference charges 25.00, and the
// answer is the standard recoverable pending-order contract.
func TestChangePlanUpgradeProratesRemainingPeriod(t *testing.T) {
	svc, _, db := newOrderTestEnv(t)
	seedPublishedPlan(t, db)
	now := time.Now()
	current := domain.PlanVersion{Key: "std", Version: 1, Price: 49_00, Monthly: 4_900_000}
	seedChangePlanSubscription(t, db, 101, current, now.Add(-15*24*time.Hour), now.Add(15*24*time.Hour))
	ctx := context.Background()
	q, err := svc.CreateQuote(ctx, 101, "pro")
	if err != nil {
		t.Fatal(err)
	}
	view, err := svc.ChangePlan(ctx, 101, q.ID, 1, "wechat")
	if err != nil {
		t.Fatal(err)
	}
	if view.Change != "upgrade" || view.Order == nil {
		t.Fatalf("unexpected change view: %+v", view)
	}
	if view.Order.AmountFen != 25_00 {
		t.Fatalf("prorated upgrade amount = %d, want 2500", view.Order.AmountFen)
	}
	if view.Order.State != domain.OrderStatePending || view.Order.CheckoutURL == "" {
		t.Fatalf("upgrade order not checkoutable: %+v", view.Order)
	}
}

// TestChangePlanSchedulesDowngradeAtPeriodEnd: a cheaper target never cuts
// the paid period short — the switch is recorded against paid_until under
// the version guard, the quote is consumed by the change marker, and the
// subscription version bumps so concurrent quotes must re-cut.
func TestChangePlanSchedulesDowngradeAtPeriodEnd(t *testing.T) {
	svc, _, db := newOrderTestEnv(t)
	seedPublishedPlan(t, db)
	seedSecondPlan(t, db)
	now := time.Now()
	current := domain.PlanVersion{Key: "pro", Version: 3, Price: 99_00, Monthly: 9_900_000}
	paidUntil := now.Add(20 * 24 * time.Hour)
	seedChangePlanSubscription(t, db, 101, current, now.Add(-10*24*time.Hour), paidUntil)
	ctx := context.Background()
	q, err := svc.CreateQuote(ctx, 101, "lite")
	if err != nil {
		t.Fatal(err)
	}
	view, err := svc.ChangePlan(ctx, 101, q.ID, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if view.Change != "scheduled_switch" || view.ScheduledPlanKey != "lite" {
		t.Fatalf("unexpected change view: %+v", view)
	}
	if view.SubscriptionVersion != 2 {
		t.Fatalf("subscription version not bumped: %+v", view)
	}
	var sub repocommercial.Subscription
	if err := db.Where("tenant_id = ?", 101).First(&sub).Error; err != nil {
		t.Fatal(err)
	}
	if sub.Version != 2 || !strings.Contains(sub.ScheduledChangeJSON, "lite") {
		t.Fatalf("scheduled downgrade not recorded: %+v", sub)
	}
	// The purchased-future-interval column is NEVER touched by a scheduled
	// switch (design 6.2: 已有提前续费区间不被悄悄改写).
	if sub.FutureIntervalJSON != "{}" {
		t.Fatalf("scheduled switch overwrote the future interval: %q", sub.FutureIntervalJSON)
	}
	var used repocommercial.QuoteRow
	if err := db.Where("id = ?", q.ID).First(&used).Error; err != nil || used.UsedOrderID == nil ||
		!strings.HasPrefix(*used.UsedOrderID, "chg_") {
		t.Fatalf("quote not consumed by the change marker: %+v %v", used, err)
	}
}

// TestChangePlanVersionConflictAndMissingSubscription: a stale
// expected_subscription_version answers ErrSubscriptionVersionConflict
// (re-quote, never overwrite) and a base-tier space has nothing to change.
func TestChangePlanVersionConflictAndMissingSubscription(t *testing.T) {
	svc, _, db := newOrderTestEnv(t)
	seedPublishedPlan(t, db)
	now := time.Now()
	current := domain.PlanVersion{Key: "std", Version: 1, Price: 49_00, Monthly: 4_900_000}
	seedChangePlanSubscription(t, db, 101, current, now.Add(-15*24*time.Hour), now.Add(15*24*time.Hour))
	ctx := context.Background()
	q, err := svc.CreateQuote(ctx, 101, "pro")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ChangePlan(ctx, 101, q.ID, 99, "wechat"); !errors.Is(err, repocommercial.ErrSubscriptionVersionConflict) {
		t.Fatalf("stale expected version: %v", err)
	}
	q2, err := svc.CreateQuote(ctx, 202, "pro")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ChangePlan(ctx, 202, q2.ID, 0, "wechat"); !errors.Is(err, ErrNoSubscriptionToChange) {
		t.Fatalf("base-tier change: %v", err)
	}
}

// TestOpenOrderClaimLosesWholeUnitOnStaleVersion proves the ATOMIC claim:
// an upgrade command whose claimed subscription version is stale is
// rejected inside the transaction — no order row, no consumed quote, no
// attempt survive the lost race, so a concurrent upgrade can never bill
// twice against the same version.
func TestOpenOrderClaimLosesWholeUnitOnStaleVersion(t *testing.T) {
	svc, _, db := newOrderTestEnv(t)
	seedPublishedPlan(t, db)
	now := time.Now()
	current := domain.PlanVersion{Key: "std", Version: 1, Price: 49_00, Monthly: 4_900_000}
	seedChangePlanSubscription(t, db, 101, current, now.Add(-15*24*time.Hour), now.Add(15*24*time.Hour))
	ctx := context.Background()
	q, err := svc.CreateQuote(ctx, 101, "pro")
	if err != nil {
		t.Fatal(err)
	}
	store := repocommercial.NewOrderStore(db)
	// First upgrade wins the claim (version 1 -> 2).
	if err := store.OpenOrder(ctx, repocommercial.OpenOrderCommand{
		OrderID: "ord_win", AttemptID: "att_win", TenantID: 101, QuoteID: q.ID,
		Kind: domain.OrderKindUpgrade, AmountFen: 25_00, Currency: "CNY",
		Provider: "wechat", Merchant: "wechat", MerchantOrderID: "mo_win",
		SubscriptionVersion: 1, ClaimSubscriptionID: "sub-101", ClaimVersion: 1,
		Now: now,
	}); err != nil {
		t.Fatalf("winning upgrade claim rejected: %v", err)
	}
	var qrow repocommercial.QuoteRow
	if err := db.Where("id = ?", q.ID).First(&qrow).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&repocommercial.QuoteRow{ID: "qt_second", TenantID: 101,
		SubscriptionVersion: 2, SnapshotJSON: qrow.SnapshotJSON, ExpiresAt: now.Add(time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	// Second upgrade claims the STALE version 1: the whole unit must lose.
	err = store.OpenOrder(ctx, repocommercial.OpenOrderCommand{
		OrderID: "ord_lose", AttemptID: "att_lose", TenantID: 101, QuoteID: "qt_second",
		Kind: domain.OrderKindUpgrade, AmountFen: 25_00, Currency: "CNY",
		Provider: "wechat", Merchant: "wechat", MerchantOrderID: "mo_lose",
		SubscriptionVersion: 2, ClaimSubscriptionID: "sub-101", ClaimVersion: 1,
		Now: now,
	})
	if !errors.Is(err, repocommercial.ErrSubscriptionVersionConflict) {
		t.Fatalf("stale claim accepted: %v", err)
	}
	var orders int64
	if err := db.Model(&repocommercial.OrderRow{}).Where("id = ?", "ord_lose").Count(&orders).Error; err != nil || orders != 0 {
		t.Fatalf("losing claim left an order row: count=%d err=%v", orders, err)
	}
	var used int64
	if err := db.Model(&repocommercial.QuoteRow{}).Where("id = ? AND used_order_id IS NOT NULL", "qt_second").Count(&used).Error; err != nil || used != 0 {
		t.Fatalf("losing claim consumed its quote: count=%d err=%v", used, err)
	}
}

// seedPublishedProWithFeatures 按 seedPublishedPlan 的同型模式 seed 一个带
// features 的版本（#81 AC1：quote 冻结名/币种/权益）。
func seedPublishedProWithFeatures(t *testing.T, db *gorm.DB) {
	t.Helper()
	def, _ := json.Marshal(domain.PlanVersion{Key: "pro", Version: 4, Price: 99_00, Monthly: 9_900_000,
		Features: map[string]bool{"advanced_models": true}, Currency: domain.CurrencyCNY, Name: "Pro"})
	if err := db.Create(&repocommercial.PlanRow{PlanKey: "pro", Version: 4,
		DefinitionJSON: string(def), ExternalID: "ext-pro-4", State: domain.PlanStatePublished}).Error; err != nil {
		t.Fatal(err)
	}
}

func TestCreateQuoteFreezesLineItemsFeaturesAndCurrency(t *testing.T) {
	svc, _, db := newOrderTestEnv(t)
	seedPublishedProWithFeatures(t, db)
	q, err := svc.CreateQuote(context.Background(), 7, "pro")
	if err != nil {
		t.Fatal(err)
	}
	if q.Currency != "CNY" {
		t.Fatalf("currency = %q, want CNY", q.Currency)
	}
	if len(q.LineItems) != 1 || q.LineItems[0].Kind != "subscription_fee" || q.LineItems[0].AmountFen != 9900 {
		t.Fatalf("line items = %+v, want single subscription_fee 9900", q.LineItems)
	}
	if !q.Features["advanced_models"] {
		t.Fatalf("features = %+v, must freeze plan entitlements", q.Features)
	}
	if q.ExpiresAt == "" {
		t.Fatal("expiry must be present")
	}
}

// TestGetOrderSurfacesUnresolvedAnomaly（#84 Task 5 / AC4 后端半）：读路径
// （RecoverOrderStatus）必须把未处置异常投影为 PaymentAttention；运营处置
// （resolve）后 attention 消失。
func TestGetOrderSurfacesUnresolvedAnomaly(t *testing.T) {
	svc, provider, db := newOrderTestEnv(t)
	provider.queryState = payment.StatePending // the channel says still pending
	seedPublishedPlan(t, db)
	ctx := context.Background()
	q, err := svc.CreateQuote(ctx, 44, "pro")
	if err != nil {
		t.Fatal(err)
	}
	view, err := svc.CreateOrder(ctx, 44, q.ID, "wechat")
	if err != nil {
		t.Fatal(err)
	}
	// No anomaly yet: a plain pending read carries no attention.
	plain, err := svc.RecoverOrderStatus(ctx, 44, view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if plain.PaymentAttention {
		t.Fatal("a clean pending order must not surface attention")
	}
	store := repocommercial.NewOrderStore(db)
	if err := store.RecordPaymentAnomaly(ctx, repocommercial.PaymentAnomalyRow{
		TenantID: 44, OrderID: view.ID, AttemptID: "mo_x", Provider: "wechat", Merchant: "1900000109",
		Transaction: "txn_x", Kind: repocommercial.PaymentAnomalyKindAmount,
		ExpectedAmountFen: 9900, ActualAmountFen: 19900,
		ExpectedCurrency: "CNY", ActualCurrency: "CNY",
	}); err != nil {
		t.Fatal(err)
	}
	flagged, err := svc.RecoverOrderStatus(ctx, 44, view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !flagged.PaymentAttention {
		t.Fatal("an unresolved anomaly must surface payment attention")
	}
	// Operator disposition clears the flag.
	if _, err := store.ResolvePaymentAnomaly(ctx, "anom_missing", 1); err == nil {
		t.Fatal("setup sanity: resolve must key the real id")
	}
	var anomaly repocommercial.PaymentAnomalyRow
	if err := db.Where("order_id = ?", view.ID).First(&anomaly).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.ResolvePaymentAnomaly(ctx, anomaly.ID, 1); err != nil {
		t.Fatal(err)
	}
	cleared, err := svc.RecoverOrderStatus(ctx, 44, view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cleared.PaymentAttention {
		t.Fatal("a resolved anomaly must clear the attention flag")
	}
}

// TestRecoverOrderStatusWrongCurrencyRetainedAsAnomaly（真栈缺陷回归锁，
// #84 第一幕错币种变体实抓）：渠道 Query 报 succeeded 且实收币种 ≠ attempt
// 面额币种（金额恰好相等）时，恢复路径绝不能用 attempt 的 CNY 构造 fact
// 洗白确认——必须与金额不符同型分流：不确认、落 currency_mismatch、
// PaymentAttention 置位（spec L127 wrong-currency 不激活）。
func TestRecoverOrderStatusWrongCurrencyRetainedAsAnomaly(t *testing.T) {
	svc, provider, db := newOrderTestEnv(t)
	provider.queryState = payment.StateSucceeded
	provider.queryAmountFen = 9900 // same amount as the face...
	provider.queryCurrency = "USD" // ...but a WRONG currency
	seedPublishedPlan(t, db)
	ctx := context.Background()
	q, err := svc.CreateQuote(ctx, 45, "pro")
	if err != nil {
		t.Fatal(err)
	}
	view, err := svc.CreateOrder(ctx, 45, q.ID, "wechat")
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := svc.RecoverOrderStatus(ctx, 45, view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.State == domain.OrderStatePaid {
		t.Fatal("a wrong-currency recovery must NEVER confirm (the attempt's currency must not launder the collection)")
	}
	if !recovered.PaymentAttention {
		t.Fatal("a wrong-currency recovery must surface attention")
	}
	store := repocommercial.NewOrderStore(db)
	ok, err := store.HasUnresolvedPaymentAnomaly(ctx, view.ID)
	if err != nil || !ok {
		t.Fatalf("the wrong-currency collection must be retained, got %v %v", ok, err)
	}
	var anomaly repocommercial.PaymentAnomalyRow
	if err := db.Where("order_id = ?", view.ID).First(&anomaly).Error; err != nil {
		t.Fatal(err)
	}
	if anomaly.Kind != repocommercial.PaymentAnomalyKindCurrency ||
		anomaly.ExpectedCurrency != "CNY" || anomaly.ActualCurrency != "USD" {
		t.Fatalf("currency anomaly snapshot mismatch: %+v", anomaly)
	}
	nFulfill := countOutbox(t, db, repocommercial.OutboxKindFulfill)
	if nFulfill != 0 {
		t.Fatalf("a wrong-currency collection must not mint a fulfill right, got %d", nFulfill)
	}
}

// ---- #84 Task 4: collected-amount comparison on the recovery paths (G2) ----

// TestRecoverOrderStatusAmountMismatchRecordsAnomalyWithoutConfirm（G2）：
// 恢复路径的渠道 Query 报 succeeded 但实收额（5000）≠ attempt 面额（9900）——
// 绝不按 attempt 金额盲目确认：不 ConfirmPayment、订单仍 pending、事实落
// amount_mismatch anomaly、view 标 PaymentAttention（Task 5 消费投影）。
func TestRecoverOrderStatusAmountMismatchRecordsAnomalyWithoutConfirm(t *testing.T) {
	svc, provider, db := newOrderTestEnv(t)
	provider.queryState = payment.StateSucceeded
	provider.queryAmountFen = 5000 // collected 5000, order face 9900
	seedPublishedPlan(t, db)
	ctx := context.Background()
	q, err := svc.CreateQuote(ctx, 41, "pro")
	if err != nil {
		t.Fatal(err)
	}
	view, err := svc.CreateOrder(ctx, 41, q.ID, "wechat")
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := svc.RecoverOrderStatus(ctx, 41, view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.State == domain.OrderStatePaid {
		t.Fatal("a wrong-amount recovery must never confirm the payment")
	}
	if !recovered.PaymentAttention {
		t.Fatal("a recovered mismatch must surface payment attention")
	}
	store := repocommercial.NewOrderStore(db)
	ok, err := store.HasUnresolvedPaymentAnomaly(ctx, view.ID)
	if err != nil || !ok {
		t.Fatalf("the query-path mismatch must retain its fact, got %v %v", ok, err)
	}
	var anomaly repocommercial.PaymentAnomalyRow
	if err := db.Where("order_id = ?", view.ID).First(&anomaly).Error; err != nil {
		t.Fatal(err)
	}
	// Closed classification: collected 5000 < face 9900 → partial_payment.
	if anomaly.Kind != repocommercial.PaymentAnomalyKindPartial ||
		anomaly.ExpectedAmountFen != 9900 || anomaly.ActualAmountFen != 5000 {
		t.Fatalf("recovery anomaly snapshot mismatch: %+v", anomaly)
	}
	// The order stays a payable pending entry — the customer may still pay it
	// correctly, and a correct later callback confirms through the normal leg.
	if len(provider.queryCalls) == 0 {
		t.Fatal("the recovery must have queried the channel")
	}
}

// TestRecoverOrderStatusCollectedAmountMatchConfirmsNormally：实收额与面额
// 一致时恢复路径照常确认（比对是分流器，不是新障碍）。
func TestRecoverOrderStatusCollectedAmountMatchConfirmsNormally(t *testing.T) {
	svc, provider, db := newOrderTestEnv(t)
	provider.queryState = payment.StateSucceeded
	provider.queryAmountFen = 9900
	seedPublishedPlan(t, db)
	ctx := context.Background()
	q, err := svc.CreateQuote(ctx, 42, "pro")
	if err != nil {
		t.Fatal(err)
	}
	view, err := svc.CreateOrder(ctx, 42, q.ID, "wechat")
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := svc.RecoverOrderStatus(ctx, 42, view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.State != domain.OrderStatePaid {
		t.Fatalf("a matching collected amount must confirm normally, got %s", recovered.State)
	}
	if recovered.PaymentAttention {
		t.Fatal("a clean recovery must not surface attention")
	}
}

func TestRecoverOrderStatusUnknownCollectionFaceIsRetryable(t *testing.T) {
	for _, tc := range []struct {
		name            string
		amount          int64
		unknownCurrency bool
	}{
		{name: "amount unknown"},
		{name: "currency unknown", amount: 9900, unknownCurrency: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, provider, db := newOrderTestEnv(t)
			provider.queryState = payment.StateSucceeded
			provider.queryAmountFen = tc.amount
			provider.queryCurrency = ""
			provider.queryCurrencyUnknown = tc.unknownCurrency
			seedPublishedPlan(t, db)
			ctx := context.Background()
			q, err := svc.CreateQuote(ctx, 46, "pro")
			if err != nil {
				t.Fatal(err)
			}
			view, err := svc.CreateOrder(ctx, 46, q.ID, "wechat")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := svc.RecoverOrderStatus(ctx, 46, view.ID); !errors.Is(err, ErrPaymentObservationUnavailable) {
				t.Fatalf("expected typed retryable observation error, got %v", err)
			}
			if row := readOrder(t, db, view.ID); row.State != domain.OrderStatePending {
				t.Fatalf("unknown collection face settled order: %s", row.State)
			}
			if n := countOutbox(t, db, repocommercial.OutboxKindFulfill); n != 0 {
				t.Fatalf("unknown collection face emitted %d fulfill events", n)
			}
		})
	}
}

func TestRecoverOrderStatusKnownCurrencyMismatchWithUnknownAmountRecordsZero(t *testing.T) {
	svc, provider, db := newOrderTestEnv(t)
	provider.queryState = payment.StateSucceeded
	provider.queryAmountFen = 0
	provider.queryCurrency = "USD"
	seedPublishedPlan(t, db)
	ctx := context.Background()
	q, err := svc.CreateQuote(ctx, 47, "pro")
	if err != nil {
		t.Fatal(err)
	}
	view, err := svc.CreateOrder(ctx, 47, q.ID, "wechat")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecoverOrderStatus(ctx, 47, view.ID); err != nil {
		t.Fatal(err)
	}
	var anomaly repocommercial.PaymentAnomalyRow
	if err := db.Where("order_id = ?", view.ID).First(&anomaly).Error; err != nil {
		t.Fatal(err)
	}
	if anomaly.Kind != repocommercial.PaymentAnomalyKindCurrency || anomaly.ActualAmountFen != 0 || anomaly.ActualCurrency != "USD" {
		t.Fatalf("expected known currency anomaly with unknown amount preserved as zero: %+v", anomaly)
	}
}

func newRealWechatObservationEnv(t *testing.T, response string) (*OrderService, *gorm.DB) {
	t.Helper()
	t.Setenv("SSRF_WHITELIST", "127.0.0.1")
	secutils.ResetSSRFWhitelistForTest()
	t.Cleanup(secutils.ResetSSRFWhitelistForTest)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/close") {
			http.Error(w, `{"code":"ORDER_PAID","message":"close raced"}`, http.StatusBadRequest)
			return
		}
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"code_url":"https://pay.example/qr"}`))
			return
		}
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(server.Close)
	merchantKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	platformKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	merchantPath := filepath.Join(t.TempDir(), "merchant.pem")
	if err := os.WriteFile(merchantPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(merchantKey)}), 0o600); err != nil {
		t.Fatal(err)
	}
	platformDER, err := x509.MarshalPKIXPublicKey(&platformKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	platformPath := filepath.Join(t.TempDir(), "platform.pem")
	if err := os.WriteFile(platformPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: platformDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := payment.NewWechatProvider(payment.WechatConfig{AppID: "wx-test", MchID: "1900000109", MchSerial: "merchant-serial", MchKeyPath: merchantPath, APIBaseURL: server.URL,
		PlatformCerts: []payment.WechatPlatformCertRef{{Serial: "platform-serial", PublicKeyPath: platformPath}}})
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	closeSQLiteDBOnCleanup(t, db)
	if s, err := db.DB(); err != nil {
		t.Fatal(err)
	} else {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repocommercial.OrderRow{}, &repocommercial.PaymentAttemptRow{}, &repocommercial.OutboxEvent{}, &repocommercial.PlanRow{}, &repocommercial.QuoteRow{}, &repocommercial.Subscription{}, &repocommercial.PaymentAnomalyRow{}); err != nil {
		t.Fatal(err)
	}
	svc, err := NewOrderService(db, map[string]payment.Provider{"wechat": provider})
	if err != nil {
		t.Fatal(err)
	}
	return svc, db
}

func TestRealWechatWrongCurrencyUnknownAmountRemainsAnomalyOnRecoveryPaths(t *testing.T) {
	for _, path := range []string{"recover", "close"} {
		for _, tc := range []struct{ name, amount string }{
			{name: "missing-total", amount: ""},
			{name: "negative-total", amount: ",\"total\":-1"},
		} {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				response := `{"trade_state":"SUCCESS","transaction_id":"wx-txn-unknown-amount","out_trade_no":"ignored","amount":{"currency":"USD"` + tc.amount + `}}`
				svc, db := newRealWechatObservationEnv(t, response)
				seedPublishedPlan(t, db)
				q, err := svc.CreateQuote(context.Background(), 71, "pro")
				if err != nil {
					t.Fatal(err)
				}
				order, err := svc.CreateOrder(context.Background(), 71, q.ID, "wechat")
				if err != nil {
					t.Fatal(err)
				}
				var got OrderView
				if path == "recover" {
					got, err = svc.RecoverOrderStatus(context.Background(), 71, order.ID)
				} else {
					got, err = svc.CloseChannelOrder(context.Background(), 71, order.ID)
				}
				if err != nil {
					t.Fatal(err)
				}
				if got.State != domain.OrderStatePending || !got.PaymentAttention {
					t.Fatalf("known wrong currency must remain pending with attention: %+v", got)
				}
				var anomaly repocommercial.PaymentAnomalyRow
				if err := db.Where("order_id = ?", order.ID).First(&anomaly).Error; err != nil {
					t.Fatal(err)
				}
				if anomaly.Kind != repocommercial.PaymentAnomalyKindCurrency || anomaly.ActualAmountFen != 0 || anomaly.ActualCurrency != "USD" {
					t.Fatalf("wrong-currency unknown-amount fact lost or fabricated: %+v", anomaly)
				}
				if n := countOutbox(t, db, repocommercial.OutboxKindFulfill); n != 0 {
					t.Fatalf("wrong-currency unknown amount emitted %d fulfill events", n)
				}
			})
		}
	}
}

func TestRealWechatMatchingCurrencyUnknownAmountIsRetryableOnRecoveryPaths(t *testing.T) {
	for _, path := range []string{"recover", "close"} {
		for _, tc := range []struct{ name, amount string }{
			{name: "missing-total", amount: ""},
			{name: "negative-total", amount: ",\"total\":-1"},
		} {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				response := `{"trade_state":"SUCCESS","transaction_id":"wx-txn-unknown-amount","amount":{"currency":"CNY"` + tc.amount + `}}`
				svc, db := newRealWechatObservationEnv(t, response)
				seedPublishedPlan(t, db)
				q, err := svc.CreateQuote(context.Background(), 72, "pro")
				if err != nil {
					t.Fatal(err)
				}
				order, err := svc.CreateOrder(context.Background(), 72, q.ID, "wechat")
				if err != nil {
					t.Fatal(err)
				}
				if path == "recover" {
					_, err = svc.RecoverOrderStatus(context.Background(), 72, order.ID)
				} else {
					_, err = svc.CloseChannelOrder(context.Background(), 72, order.ID)
				}
				if !errors.Is(err, ErrPaymentObservationUnavailable) {
					t.Fatalf("matching currency with unknown amount must be retryable, got %v", err)
				}
				if row := readOrder(t, db, order.ID); row.State != domain.OrderStatePending {
					t.Fatalf("unknown amount settled order: %s", row.State)
				}
				var n int64
				if err := db.Model(&repocommercial.PaymentAnomalyRow{}).Where("order_id = ?", order.ID).Count(&n).Error; err != nil || n != 0 {
					t.Fatalf("unknown amount must not create an anomaly: n=%d err=%v", n, err)
				}
				if n := countOutbox(t, db, repocommercial.OutboxKindFulfill); n != 0 {
					t.Fatalf("unknown amount emitted %d fulfill events", n)
				}
			})
		}
	}
}

func TestRecoverWhitespaceCurrencyIsUnknownNotMismatch(t *testing.T) {
	svc, provider, db := newOrderTestEnv(t)
	provider.queryState, provider.queryAmountFen, provider.queryCurrency = payment.StateSucceeded, 9900, "   "
	seedPublishedPlan(t, db)
	q, err := svc.CreateQuote(context.Background(), 73, "pro")
	if err != nil {
		t.Fatal(err)
	}
	order, err := svc.CreateOrder(context.Background(), 73, q.ID, "wechat")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecoverOrderStatus(context.Background(), 73, order.ID); !errors.Is(err, ErrPaymentObservationUnavailable) {
		t.Fatalf("whitespace currency must be retryable, got %v", err)
	}
	assertNoPaymentAnomalyOrFulfillment(t, db, order.ID)
}

func assertNoPaymentAnomalyOrFulfillment(t *testing.T, db *gorm.DB, orderID string) {
	t.Helper()
	if row := readOrder(t, db, orderID); row.State != domain.OrderStatePending {
		t.Fatalf("unknown currency settled order: %s", row.State)
	}
	var n int64
	if err := db.Model(&repocommercial.PaymentAnomalyRow{}).Where("order_id = ?", orderID).Count(&n).Error; err != nil || n != 0 {
		t.Fatalf("unknown currency must not create anomaly: n=%d err=%v", n, err)
	}
	if n := countOutbox(t, db, repocommercial.OutboxKindFulfill); n != 0 {
		t.Fatalf("unknown currency emitted %d fulfill events", n)
	}
}

// TestRecoverOrderStatusLateSuccessAfterFulfilledIsIdempotentOverPayment（G5）：
// 订单已 fulfilled 后第二渠道 late succeeded——恢复读幂等（仍 fulfilled）、
// 不产生第二个 fulfill 事件、不二次履约，第二笔只落 over_payment 事件（由
// Task 3 的 drain 消费为 anomaly）。
func TestRecoverOrderStatusLateSuccessAfterFulfilledIsIdempotentOverPayment(t *testing.T) {
	svc, _, db := newOrderTestEnv(t)
	seedPublishedPlan(t, db)
	ctx := context.Background()
	q, err := svc.CreateQuote(ctx, 43, "pro")
	if err != nil {
		t.Fatal(err)
	}
	view, err := svc.CreateOrder(ctx, 43, q.ID, "wechat")
	if err != nil {
		t.Fatal(err)
	}
	store := repocommercial.NewOrderStore(db)
	var first repocommercial.PaymentAttemptRow
	if err := db.Where("order_id = ?", view.ID).First(&first).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.ConfirmPayment(ctx, domain.PaymentFact{
		TenantID: 43, OrderID: view.ID, AttemptID: first.MerchantOrderID, Provider: "wechat",
		Merchant: first.Merchant, Transaction: "txn_first", Amount: 9900, Currency: "CNY", State: "succeeded",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkFulfilled(ctx, view.ID); err != nil {
		t.Fatal(err)
	}
	// The SECOND channel's late success (a second registered attempt).
	if err := store.RegisterAttempt(ctx, repocommercial.PaymentAttemptRow{
		ID: "att2-late", TenantID: 43, OrderID: view.ID, Provider: "alipay", Merchant: "2088000000000000",
		MerchantOrderID: "mo2-late", AmountFen: 9900, Currency: "CNY",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ConfirmPayment(ctx, domain.PaymentFact{
		TenantID: 43, OrderID: view.ID, AttemptID: "mo2-late", Provider: "alipay",
		Merchant: "2088000000000000", Transaction: "txn2_late", Amount: 9900, Currency: "CNY", State: "succeeded",
	}); err != nil {
		t.Fatal(err)
	}
	// The recovery read on the fulfilled order is idempotent and honest.
	recovered, err := svc.RecoverOrderStatus(ctx, 43, view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.State != domain.OrderStateFulfilled {
		t.Fatalf("a fulfilled order must stay fulfilled through recovery, got %s", recovered.State)
	}
	nFulfill := countOutbox(t, db, repocommercial.OutboxKindFulfill)
	nOver := countOutbox(t, db, repocommercial.OutboxKindOverPaid)
	if nFulfill != 1 {
		t.Fatalf("the late success must never mint a second fulfill right, got %d", nFulfill)
	}
	if nOver != 1 {
		t.Fatalf("the late success must be audited as over_payment, got %d", nOver)
	}
}

func TestListOrdersProjectsAnomalyAttentionFromTenantBatch(t *testing.T) {
	svc, _, db := newOrderTestEnv(t)
	ctx := context.Background()
	store := repocommercial.NewOrderStore(db)
	ids := make([]string, 0, 40)
	for i := 0; i < 40; i++ {
		id := fmt.Sprintf("order_batch_%02d", i)
		if err := db.Create(&repocommercial.OrderRow{ID: id, TenantID: 101, QuoteID: fmt.Sprintf("quote_batch_%02d", i), Kind: "purchase", AmountFen: 9900, Currency: "CNY", State: domain.OrderStatePaid, Version: 1}).Error; err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := store.RecordPaymentAnomaly(ctx, repocommercial.PaymentAnomalyRow{TenantID: 101, OrderID: ids[29], AttemptID: "m", Provider: "wechat", Merchant: "m", Transaction: "t", Kind: repocommercial.PaymentAnomalyKindAmount, ExpectedCurrency: "CNY", ActualCurrency: "CNY"}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.ListOrders(ctx, 101)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 40 {
		t.Fatalf("got %d rows", len(got))
	}
	for _, v := range got {
		if v.PaymentAttention != (v.ID == ids[29]) {
			t.Fatalf("attention projection for %s = %v", v.ID, v.PaymentAttention)
		}
	}
}

func TestListOrdersUsesOneAnomalyQueryForSmallAndLargeResults(t *testing.T) {
	svc, _, db := newOrderTestEnv(t)
	ctx := context.Background()
	for i := 0; i < 40; i++ {
		if err := db.Create(&repocommercial.OrderRow{ID: fmt.Sprintf("count_order_%02d", i), TenantID: 111, QuoteID: fmt.Sprintf("count_quote_%02d", i), Kind: "purchase", AmountFen: 100, Currency: "CNY", State: domain.OrderStatePaid, Version: 1}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := repocommercial.NewOrderStore(db).RecordPaymentAnomaly(ctx, repocommercial.PaymentAnomalyRow{TenantID: 111, OrderID: "count_order_29", AttemptID: "a", Provider: "wechat", Merchant: "count-merchant", Transaction: "count-txn", Kind: repocommercial.PaymentAnomalyKindAmount, ExpectedCurrency: "CNY", ActualCurrency: "CNY"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&repocommercial.OrderRow{ID: "count_single_order", TenantID: 112, QuoteID: "count_single_quote", Kind: "purchase", AmountFen: 100, Currency: "CNY", State: domain.OrderStatePaid, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := repocommercial.NewOrderStore(db).RecordPaymentAnomaly(ctx, repocommercial.PaymentAnomalyRow{TenantID: 112, OrderID: "count_single_order", AttemptID: "a", Provider: "wechat", Merchant: "count-merchant", Transaction: "count-single-txn", Kind: repocommercial.PaymentAnomalyKindAmount, ExpectedCurrency: "CNY", ActualCurrency: "CNY"}); err != nil {
		t.Fatal(err)
	}
	var anomalyQueries atomic.Int64
	callbackName := "test/count_anomaly_queries/" + strings.ReplaceAll(t.Name(), "/", "_")
	if err := db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "commercial_payment_anomalies" {
			anomalyQueries.Add(1)
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer db.Callback().Query().Remove(callbackName)
	for _, tc := range []struct {
		name                  string
		tenant                uint64
		wantRows, wantQueries int
		attentionOrderID      string
	}{
		{name: "one order", tenant: 112, wantRows: 1, wantQueries: 1, attentionOrderID: "count_single_order"},
		{name: "forty orders", tenant: 111, wantRows: 40, wantQueries: 1, attentionOrderID: "count_order_29"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			anomalyQueries.Store(0)
			got, err := svc.ListOrders(ctx, tc.tenant)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tc.wantRows {
				t.Fatalf("rows=%d want %d", len(got), tc.wantRows)
			}
			if n := int(anomalyQueries.Load()); n != tc.wantQueries {
				t.Fatalf("anomaly-table SELECT count=%d want %d", n, tc.wantQueries)
			}
			for _, v := range got {
				if v.PaymentAttention != (v.ID == tc.attentionOrderID) {
					t.Fatalf("attention for %s=%v", v.ID, v.PaymentAttention)
				}
			}
		})
	}
}

func TestListOrdersFailsOpenWhenAnomalyQueryFails(t *testing.T) {
	svc, _, db := newOrderTestEnv(t)
	ctx := context.Background()
	row := repocommercial.OrderRow{ID: "fail_open_order", TenantID: 121, QuoteID: "fail_open_quote", Kind: "purchase", AmountFen: 1234, Currency: "CNY", State: domain.OrderStatePaid, Version: 7}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	store := repocommercial.NewOrderStore(db)
	if err := store.RecordPaymentAnomaly(ctx, repocommercial.PaymentAnomalyRow{TenantID: 121, OrderID: row.ID, AttemptID: "a", Provider: "wechat", Merchant: "m", Transaction: "fail-open-txn", Kind: repocommercial.PaymentAnomalyKindAmount, ExpectedCurrency: "CNY", ActualCurrency: "CNY"}); err != nil {
		t.Fatal(err)
	}
	callbackName := "test/fail_anomaly_query/" + strings.ReplaceAll(t.Name(), "/", "_")
	if err := db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "commercial_payment_anomalies" {
			tx.AddError(fmt.Errorf("injected anomaly query failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer db.Callback().Query().Remove(callbackName)
	got, err := svc.ListOrders(ctx, 121)
	if err != nil {
		t.Fatalf("anomaly lookup failure must fail open, err=%v", err)
	}
	if len(got) != 1 || got[0].ID != row.ID || got[0].State != row.State || got[0].AmountFen != row.AmountFen || got[0].Version != row.Version {
		t.Fatalf("primary order projection lost: %+v", got)
	}
	if got[0].PaymentAttention {
		t.Fatalf("failed anomaly lookup must not assert attention: %+v", got[0])
	}
}

func TestCurrentPayablePendingOrderViewProjectsAndClearsAttention(t *testing.T) {
	svc, _, db := newOrderTestEnv(t)
	seedPublishedPlan(t, db)
	ctx := context.Background()
	q, err := svc.CreateQuote(ctx, 101, "pro")
	if err != nil {
		t.Fatal(err)
	}
	order, err := svc.CreateOrder(ctx, 101, q.ID, "wechat")
	if err != nil {
		t.Fatal(err)
	}
	store := repocommercial.NewOrderStore(db)
	if err = store.RecordPaymentAnomaly(ctx, repocommercial.PaymentAnomalyRow{TenantID: 101, OrderID: order.ID, AttemptID: "m", Provider: "wechat", Merchant: "m", Transaction: "t", Kind: repocommercial.PaymentAnomalyKindAmount, ExpectedCurrency: "CNY", ActualCurrency: "CNY"}); err != nil {
		t.Fatal(err)
	}
	v, err := svc.CurrentPayablePendingOrderView(ctx, 101)
	if err != nil || !v.PaymentAttention {
		t.Fatalf("unresolved view=%+v err=%v", v, err)
	}
	if v.CheckoutURL != order.CheckoutURL {
		t.Fatalf("unresolved view checkout URL=%q want %q", v.CheckoutURL, order.CheckoutURL)
	}
	var a repocommercial.PaymentAnomalyRow
	if err = db.Where("order_id = ?", order.ID).First(&a).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = store.ResolvePaymentAnomaly(ctx, a.ID, 1); err != nil {
		t.Fatal(err)
	}
	v, err = svc.CurrentPayablePendingOrderView(ctx, 101)
	if err != nil || v.PaymentAttention {
		t.Fatalf("resolved view=%+v err=%v", v, err)
	}
	if v.CheckoutURL != order.CheckoutURL {
		t.Fatalf("resolved view checkout URL=%q want %q", v.CheckoutURL, order.CheckoutURL)
	}
}
