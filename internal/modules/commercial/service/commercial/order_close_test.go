package commercial

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"

	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	"github.com/Tencent/WeKnora/internal/modules/commercial/commercialplatform"
	"github.com/Tencent/WeKnora/internal/modules/commercial/payment"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ---- issue #83 Task 3: race-safe channel-close orchestration ----
//
// CloseChannelOrder 是 #83 唯一的新渠道侧生产行为：一张 pending 订单的支付
// 入口退役（spec L123 close），关单撞在途支付时以查单决胜（spec L165 不定
// 结局必须查单），资金事实保留、履约恰好一次（spec L127）。

// closeRaceStub 模拟可编排的渠道边界：Close 按 closeErr 返回（nil=204 成功；
// 非 nil=渠道错误，含 ORDER_PAID 形状），Query 按 queryRes/queryErr 返回。
type closeRaceStub struct {
	merchant string
	mu       sync.Mutex
	closeErr error
	queryRes payment.AttemptResult
	queryErr error
	// counters
	closeCalls  int
	queryCalls  int
	createCalls int
}

func (p *closeRaceStub) Create(_ context.Context, req payment.OrderRequest) (payment.AttemptResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.createCalls++
	return payment.AttemptResult{State: payment.StatePending, ProviderID: req.MerchantOrderID,
		CheckoutURL: "https://pay.example/" + p.merchant}, nil
}
func (p *closeRaceStub) Query(_ context.Context, id string) (payment.AttemptResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.queryCalls++
	if p.queryErr != nil {
		return payment.AttemptResult{}, p.queryErr
	}
	return p.queryRes, nil
}
func (p *closeRaceStub) Close(_ context.Context, _ string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closeCalls++
	return p.closeErr
}
func (p *closeRaceStub) Verify(context.Context, http.Header, []byte) (domain.PaymentFact, error) {
	return domain.PaymentFact{}, errors.New("close-race stub never verifies callbacks")
}
func (p *closeRaceStub) Refund(context.Context, payment.RefundRequest) (payment.RefundResult, error) {
	return payment.RefundResult{State: payment.StatePending}, nil
}
func (p *closeRaceStub) QueryRefund(context.Context, string) (payment.RefundResult, error) {
	return payment.RefundResult{State: payment.StatePending}, nil
}
func (p *closeRaceStub) MerchantID() string { return p.merchant }

// orderPaidError 复刻微信关单撞在途支付的渠道错误形状（Task 1 pin：
// "wechat api status 400 code=ORDER_PAID"）。
var orderPaidError = errors.New("wechat close mo_x: wechat api status 400 code=ORDER_PAID: order paid")

// newCloseRaceEnv wires one sqlite store + a real OrderService with an
// orchestratable wechat stub and an alipay stub (distinct merchant identities).
func newCloseRaceEnv(t *testing.T) (*OrderService, *closeRaceStub, *closeRaceStub, *gorm.DB) {
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
		&repocommercial.Subscription{}); err != nil {
		t.Fatal(err)
	}
	wechat := &closeRaceStub{merchant: "1900000830"}
	alipay := &closeRaceStub{merchant: "2088000000000083"}
	svc, err := NewOrderService(db, map[string]payment.Provider{
		payment.ProviderWechat: wechat, payment.ProviderAlipay: alipay,
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc, wechat, alipay, db
}

// openRaceOrder seeds the published plan (idempotently — one test may open
// several orders) and opens one payable pending order on the named provider
// through the REAL OrderService path.
func openRaceOrder(t *testing.T, svc *OrderService, db *gorm.DB, tenant uint64, providerName string) (OrderView, repocommercial.PaymentAttemptRow) {
	t.Helper()
	var seeded int64
	if err := db.Model(&repocommercial.PlanRow{}).Where("plan_key = ?", "pro").Count(&seeded).Error; err != nil {
		t.Fatal(err)
	}
	if seeded == 0 {
		seedPublishedPlan(t, db)
	}
	q, err := svc.CreateQuote(context.Background(), tenant, "pro")
	if err != nil {
		t.Fatal(err)
	}
	order, err := svc.CreateOrder(context.Background(), tenant, q.ID, providerName)
	if err != nil {
		t.Fatal(err)
	}
	if order.CheckoutURL == "" {
		t.Fatal("the race fixture needs a payable (link-carrying) order")
	}
	var att repocommercial.PaymentAttemptRow
	if err := db.Where("order_id = ?", order.ID).First(&att).Error; err != nil {
		t.Fatal(err)
	}
	return order, att
}

func readAttempt(t *testing.T, db *gorm.DB, attID string) repocommercial.PaymentAttemptRow {
	t.Helper()
	var att repocommercial.PaymentAttemptRow
	if err := db.Where("id = ?", attID).First(&att).Error; err != nil {
		t.Fatal(err)
	}
	return att
}

func readOrder(t *testing.T, db *gorm.DB, orderID string) repocommercial.OrderRow {
	t.Helper()
	var row repocommercial.OrderRow
	if err := db.Where("id = ?", orderID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row
}

func countOutbox(t *testing.T, db *gorm.DB, kind string) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&repocommercial.OutboxEvent{}).Where("kind = ?", kind).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCloseChannelOrderClosesPendingAndFreesSlot(t *testing.T) {
	svc, wechat, _, db := newCloseRaceEnv(t)
	order, att := openRaceOrder(t, svc, db, 901, payment.ProviderWechat)

	view, err := svc.CloseChannelOrder(context.Background(), 901, order.ID)
	if err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if view.State != domain.OrderStatePending {
		t.Fatalf("a cleanly closed order is still pending locally, got %q", view.State)
	}
	if after := readAttempt(t, db, att.ID); after.State != repocommercial.PaymentAttemptStateClosed {
		t.Fatalf("attempt must be closed, got %q", after.State)
	}
	if row := readOrder(t, db, order.ID); !row.ChannelFailed {
		t.Fatal("the closed order must be marked channel-failed (payment slot released)")
	}
	// The payable-pending slot is free again.
	store := repocommercial.NewOrderStore(db)
	if _, err := store.CurrentPendingPurchaseOrder(context.Background(), 901, 99_00, domain.CurrencyCNY); !errors.Is(err, repocommercial.ErrOrderNotFound) {
		t.Fatalf("the closed order must not be a payable entry, got err=%v", err)
	}
	if wechat.closeCalls != 1 {
		t.Fatalf("exactly one channel close expected, got %d", wechat.closeCalls)
	}
}

func TestCloseChannelOrderPaidRaceConfirmsFundFact(t *testing.T) {
	svc, wechat, _, db := newCloseRaceEnv(t)
	order, att := openRaceOrder(t, svc, db, 902, payment.ProviderWechat)
	// The user pays while we are closing: Close answers ORDER_PAID, the
	// decisive Query answers SUCCESS.
	wechat.closeErr = orderPaidError
	wechat.queryRes = payment.AttemptResult{State: payment.StateSucceeded, ProviderID: "wx_txn_race"}

	view, err := svc.CloseChannelOrder(context.Background(), 902, order.ID)
	if err != nil {
		t.Fatalf("the paid race must resolve through the query, got %v", err)
	}
	if view.State != domain.OrderStatePaid {
		t.Fatalf("an in-flight payment must land as a paid order, got %q", view.State)
	}
	after := readAttempt(t, db, att.ID)
	if after.State != repocommercial.PaymentAttemptStateSucceeded || after.ProviderTransactionID == nil || *after.ProviderTransactionID != "wx_txn_race" {
		t.Fatalf("the fund fact must be recorded, got state=%s txn=%v", after.State, after.ProviderTransactionID)
	}
	if row := readOrder(t, db, order.ID); row.State != domain.OrderStatePaid {
		t.Fatalf("order must be paid, got %s", row.State)
	}
	if n := countOutbox(t, db, repocommercial.OutboxKindFulfill); n != 1 {
		t.Fatalf("exactly one fulfill event expected, got %d", n)
	}
	if wechat.closeCalls != 1 || wechat.queryCalls != 1 {
		t.Fatalf("the race must be close-then-query-decide (close=%d query=%d)", wechat.closeCalls, wechat.queryCalls)
	}
}

func TestLateSuccessAfterCloseAuditsWithoutSecondFulfillment(t *testing.T) {
	svc, wechat, _, db := newCloseRaceEnv(t)
	order, att := openRaceOrder(t, svc, db, 903, payment.ProviderWechat)
	store := repocommercial.NewOrderStore(db)
	ctx := context.Background()

	// Forward order: the payment confirms FIRST, then a close hits an
	// already-paid order, then a SECOND success arrives under a different
	// channel transaction.
	if err := store.ConfirmPayment(ctx, domain.PaymentFact{
		Provider: payment.ProviderWechat, Merchant: att.Merchant,
		AttemptID: att.MerchantOrderID, OrderID: order.ID, TenantID: 903,
		Amount: domain.CNYFen(att.AmountFen), Currency: att.Currency,
		Transaction: "wx_txn_first", State: payment.StateSucceeded.String(),
	}); err != nil {
		t.Fatal(err)
	}
	view, err := svc.CloseChannelOrder(ctx, 903, order.ID)
	if err != nil {
		t.Fatalf("closing a paid order is a no-op view, got %v", err)
	}
	if view.State != domain.OrderStatePaid || wechat.closeCalls != 0 {
		t.Fatalf("a paid order has no channel entry to close (view=%+v closeCalls=%d)", view, wechat.closeCalls)
	}
	if err := store.ConfirmPayment(ctx, domain.PaymentFact{
		Provider: payment.ProviderWechat, Merchant: att.Merchant,
		AttemptID: att.MerchantOrderID, OrderID: order.ID, TenantID: 903,
		Amount: domain.CNYFen(att.AmountFen), Currency: att.Currency,
		Transaction: "wx_txn_second", State: payment.StateSucceeded.String(),
	}); err != nil {
		t.Fatalf("a late second success is an audit fact, not an error: %v", err)
	}
	if n := countOutbox(t, db, repocommercial.OutboxKindOverPaid); n != 1 {
		t.Fatalf("exactly one over-payment audit expected, got %d", n)
	}
	if n := countOutbox(t, db, repocommercial.OutboxKindFulfill); n != 1 {
		t.Fatalf("no second fulfillment right may appear, got %d", n)
	}

	// Reverse order: a CLEAN close first, then the FIRST success arrives
	// late — ConfirmPayment's idempotency still lands the payment exactly
	// once.
	order2, att2 := openRaceOrder(t, svc, db, 904, payment.ProviderWechat)
	if _, err := svc.CloseChannelOrder(ctx, 904, order2.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.ConfirmPayment(ctx, domain.PaymentFact{
		Provider: payment.ProviderWechat, Merchant: att2.Merchant,
		AttemptID: att2.MerchantOrderID, OrderID: order2.ID, TenantID: 904,
		Amount: domain.CNYFen(att2.AmountFen), Currency: att2.Currency,
		Transaction: "wx_txn_late", State: payment.StateSucceeded.String(),
	}); err != nil {
		t.Fatalf("a late success after a clean close must still land the payment: %v", err)
	}
	if row := readOrder(t, db, order2.ID); row.State != domain.OrderStatePaid {
		t.Fatalf("the fund fact survives the close, got %s", row.State)
	}
	if n := countOutbox(t, db, repocommercial.OutboxKindFulfill); n != 2 {
		t.Fatalf("two orders paid ⇒ exactly two fulfill events, got %d", n)
	}
}

func TestCloseChannelOrderIndeterminateStaysPending(t *testing.T) {
	// Sub-case A: both close and query fail — the outcome is indeterminate,
	// nothing may be marked.
	svc, wechat, _, db := newCloseRaceEnv(t)
	order, att := openRaceOrder(t, svc, db, 905, payment.ProviderWechat)
	wechat.closeErr = errors.New("transport reset by peer")
	wechat.queryErr = errors.New("query timeout")
	if _, err := svc.CloseChannelOrder(context.Background(), 905, order.ID); err == nil {
		t.Fatal("an indeterminate close must surface its error")
	}
	if after := readAttempt(t, db, att.ID); after.State != repocommercial.PaymentAttemptStatePending {
		t.Fatalf("attempt must stay pending, got %q", after.State)
	}
	if row := readOrder(t, db, order.ID); row.ChannelFailed {
		t.Fatal("an indeterminate close must not mark the order channel-failed")
	}
	// Sub-case B: close fails, query answers NOTPAY — the original close
	// error is returned and nothing is marked (the channel may still see
	// the order open or the payment in flight).
	wechat.queryErr = nil
	wechat.queryRes = payment.AttemptResult{State: payment.StatePending, ProviderID: att.MerchantOrderID}
	if _, err := svc.CloseChannelOrder(context.Background(), 905, order.ID); !errors.Is(err, wechat.closeErr) {
		t.Fatalf("a NOTPAY query must return the original close error, got %v", err)
	}
	if after := readAttempt(t, db, att.ID); after.State != repocommercial.PaymentAttemptStatePending {
		t.Fatalf("attempt must stay pending, got %q", after.State)
	}
	if row := readOrder(t, db, order.ID); row.ChannelFailed {
		t.Fatal("a NOTPAY outcome must not mark the order channel-failed")
	}
	// The order stays payable (the slot is still held).
	store := repocommercial.NewOrderStore(db)
	if _, err := store.CurrentPendingPurchaseOrder(context.Background(), 905, 99_00, domain.CurrencyCNY); err != nil {
		t.Fatalf("the order must remain a payable entry, got %v", err)
	}
}

func TestCloseChannelOrderQueryClosedIdempotentMark(t *testing.T) {
	svc, wechat, _, db := newCloseRaceEnv(t)
	order, att := openRaceOrder(t, svc, db, 906, payment.ProviderWechat)
	// The channel already closed the order (a replayed close answers the
	// ORDER_CLOSED error); the decisive query answers CLOSED — the same
	// landing as a clean close.
	wechat.closeErr = errors.New("wechat close mo_x: wechat api status 400 code=ORDER_CLOSED: order closed")
	wechat.queryRes = payment.AttemptResult{State: payment.StateClosed, ProviderID: att.MerchantOrderID}

	view, err := svc.CloseChannelOrder(context.Background(), 906, order.ID)
	if err != nil {
		t.Fatalf("a query-closed outcome must land as a clean close, got %v", err)
	}
	if view.State != domain.OrderStatePending {
		t.Fatalf("a closed channel order is still a pending local row, got %q", view.State)
	}
	if after := readAttempt(t, db, att.ID); after.State != repocommercial.PaymentAttemptStateClosed {
		t.Fatalf("attempt must be closed, got %q", after.State)
	}
	if row := readOrder(t, db, order.ID); !row.ChannelFailed {
		t.Fatal("the order must be marked channel-failed")
	}
}

func TestCloseChannelOrderNonPendingNoop(t *testing.T) {
	svc, wechat, _, db := newCloseRaceEnv(t)
	order, att := openRaceOrder(t, svc, db, 907, payment.ProviderWechat)
	store := repocommercial.NewOrderStore(db)
	if err := store.ConfirmPayment(context.Background(), domain.PaymentFact{
		Provider: payment.ProviderWechat, Merchant: att.Merchant,
		AttemptID: att.MerchantOrderID, OrderID: order.ID, TenantID: 907,
		Amount: domain.CNYFen(att.AmountFen), Currency: att.Currency,
		Transaction: "wx_txn_907", State: payment.StateSucceeded.String(),
	}); err != nil {
		t.Fatal(err)
	}
	view, err := svc.CloseChannelOrder(context.Background(), 907, order.ID)
	if err != nil {
		t.Fatalf("closing a non-pending order answers its current view, got %v", err)
	}
	if view.State != domain.OrderStatePaid {
		t.Fatalf("view must carry the paid state, got %q", view.State)
	}
	if wechat.closeCalls != 0 {
		t.Fatalf("no channel call may happen for a non-pending order, got %d", wechat.closeCalls)
	}
}

func TestCloseChannelOrderTenantMismatch(t *testing.T) {
	svc, wechat, _, db := newCloseRaceEnv(t)
	order, _ := openRaceOrder(t, svc, db, 908, payment.ProviderWechat)
	if _, err := svc.CloseChannelOrder(context.Background(), 999, order.ID); !errors.Is(err, ErrOrderTenantMismatch) {
		t.Fatalf("a foreign tenant must be rejected, got %v", err)
	}
	if wechat.closeCalls != 0 {
		t.Fatalf("no channel call may happen on a tenant mismatch, got %d", wechat.closeCalls)
	}
}

// ---- the checkout channel-switch wiring (purchase layer) ----

// newSwitchEnv wires the full purchase stack with BOTH channel stubs so a
// channel switch (wechat↔alipay) exercises the close-then-reopen path.
func newSwitchEnv(t *testing.T) (*PurchaseService, *closeRaceStub, *closeRaceStub, *gorm.DB) {
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
	wechat := &closeRaceStub{merchant: "1900000830"}
	alipay := &closeRaceStub{merchant: "2088000000000083"}
	orders, err := NewOrderService(db, map[string]payment.Provider{
		payment.ProviderWechat: wechat, payment.ProviderAlipay: alipay,
	})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := NewPurchaseService(db, accounts, plans, orders, fake)
	if err != nil {
		t.Fatal(err)
	}
	return svc, wechat, alipay, db
}

func TestPurchaseSwitchesChannelByClosingOldOrder(t *testing.T) {
	for _, tc := range []struct{ old, new string }{
		{payment.ProviderWechat, payment.ProviderAlipay},
		{payment.ProviderAlipay, payment.ProviderWechat},
	} {
		t.Run(tc.old+"->"+tc.new, func(t *testing.T) {
			svc, wechat, alipay, db := newSwitchEnv(t)
			purchaseSeedPlan(t, svc.plans, "pro", 9900)
			qA := purchaseQuote(t, svc.orders, 91, "pro")
			first, err := svc.Purchase(context.Background(), 91, qA.ID, tc.old, "billing-admin", "Space 91")
			if err != nil {
				t.Fatal(err)
			}
			if first.Order == nil || first.Order.CheckoutURL == "" {
				t.Fatalf("the old-channel order must be payable, got %+v", first.Order)
			}

			qB := purchaseQuote(t, svc.orders, 91, "pro")
			second, err := svc.Purchase(context.Background(), 91, qB.ID, tc.new, "billing-admin", "Space 91")
			if err != nil {
				t.Fatalf("the channel switch must succeed: %v", err)
			}
			if second.Order == nil || second.Order.CheckoutURL == "" {
				t.Fatalf("the new-channel order must be payable, got %+v", second.Order)
			}
			if second.Order.ID == first.Order.ID {
				t.Fatal("the switch must open a NEW channel order, not replay the old one")
			}
			// The old order is retired: channel-failed + attempt closed.
			oldRow := readOrder(t, db, first.Order.ID)
			if !oldRow.ChannelFailed {
				t.Fatal("the old order must be marked channel-failed by the switch")
			}
			var oldAtt repocommercial.PaymentAttemptRow
			if err := db.Where("order_id = ?", first.Order.ID).First(&oldAtt).Error; err != nil {
				t.Fatal(err)
			}
			if oldAtt.State != repocommercial.PaymentAttemptStateClosed {
				t.Fatalf("the old attempt must be closed, got %q", oldAtt.State)
			}
			// The new order runs the requested provider and is the ONLY
			// payable entry.
			var newAtt repocommercial.PaymentAttemptRow
			if err := db.Where("order_id = ?", second.Order.ID).First(&newAtt).Error; err != nil {
				t.Fatal(err)
			}
			if newAtt.Provider != tc.new {
				t.Fatalf("the new order must run provider %q, got %q", tc.new, newAtt.Provider)
			}
			store := repocommercial.NewOrderStore(db)
			pending, err := store.CurrentPendingPurchaseOrder(context.Background(), 91, 9900, domain.CurrencyCNY)
			if err != nil || pending.ID != second.Order.ID {
				t.Fatalf("exactly one payable order (the new one) must remain, got %+v err=%v", pending, err)
			}
			// The old channel saw exactly one close; the new channel saw
			// exactly one create.
			oldStub, newStub := wechat, alipay
			if tc.old == payment.ProviderAlipay {
				oldStub, newStub = alipay, wechat
			}
			if oldStub.closeCalls != 1 || newStub.createCalls != 1 {
				t.Fatalf("switch must close old (=%d) and create new (=%d)", oldStub.closeCalls, newStub.createCalls)
			}
		})
	}
}

func TestPurchaseSwitchPaidRaceAnswersPaidOrder(t *testing.T) {
	svc, wechat, alipay, db := newSwitchEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	qA := purchaseQuote(t, svc.orders, 92, "pro")
	first, err := svc.Purchase(context.Background(), 92, qA.ID, payment.ProviderWechat, "billing-admin", "Space 92")
	if err != nil {
		t.Fatal(err)
	}
	// The wechat payment is in flight: close answers ORDER_PAID, query
	// answers SUCCESS.
	wechat.closeErr = orderPaidError
	wechat.queryRes = payment.AttemptResult{State: payment.StateSucceeded, ProviderID: "wx_txn_swpaid"}

	qB := purchaseQuote(t, svc.orders, 92, "pro")
	second, err := svc.Purchase(context.Background(), 92, qB.ID, payment.ProviderAlipay, "billing-admin", "Space 92")
	if err != nil {
		t.Fatalf("the paid race must answer, not fail: %v", err)
	}
	if second.Order == nil || second.Order.ID != first.Order.ID {
		t.Fatalf("the answer must be the OLD paid order, got %+v", second.Order)
	}
	if second.Order.State != domain.OrderStatePaid {
		t.Fatalf("the in-flight wechat payment must surface as paid, got %q", second.Order.State)
	}
	var att repocommercial.PaymentAttemptRow
	if err := db.Where("order_id = ?", first.Order.ID).First(&att).Error; err != nil {
		t.Fatal(err)
	}
	if att.Provider != payment.ProviderWechat {
		t.Fatalf("the answered order is the wechat one, got %q", att.Provider)
	}
	if alipay.createCalls != 0 {
		t.Fatalf("a paid race must never open a second channel order, got %d alipay creates", alipay.createCalls)
	}
	if n := countOutbox(t, db, repocommercial.OutboxKindFulfill); n != 1 {
		t.Fatalf("exactly one fulfill event expected, got %d", n)
	}
	if wechat.closeCalls != 1 || wechat.queryCalls != 1 {
		t.Fatalf("the race must be close-then-query-decide (close=%d query=%d)", wechat.closeCalls, wechat.queryCalls)
	}
}
