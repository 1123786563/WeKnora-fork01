package handler

// POST /api/v1/commercial/callbacks/:provider 的回调入账链回归（issue #82
// 真实流程验证缺陷 2）：下单侧 attempt.merchant 必须写渠道商户身份
// （SellerID/MchID，即 Provider.MerchantID()），回调侧 resolveByMerchantOrderID
// 与 ConfirmPayment 都按 (provider, merchant, merchant_order_id) 三元组解析——
// 任何一侧写 provider 名字面量都会让真实渠道回调永久 404（验证实录：注册
// 'alipay' vs 通知 SellerID → no registered payment attempt）。
//
// 本文件用真实 OrderService（走完整 openOrder 落库路径）+ 真实
// PaymentCallbacksHandler + 真实 ConfirmPayment 事务，按支付宝分支验收：
// 匿名签名通知形状 → 验签事实 → attempt 解析 → 订单 paid → outbox 恰一。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/commercial"
	"github.com/Tencent/WeKnora/internal/modules/commercial/payment"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// callbackAlipayStub 模拟支付宝渠道边界：MerchantID 返回支付宝形状的
// SellerID（绝不能等于 provider 名），Verify 返回同源签名事实。
type callbackAlipayStub struct{ fact commercial.PaymentFact }

func (p *callbackAlipayStub) Create(context.Context, payment.OrderRequest) (payment.AttemptResult, error) {
	return payment.AttemptResult{State: payment.StatePending, ProviderID: "qr"}, nil
}
func (p *callbackAlipayStub) Query(context.Context, string) (payment.AttemptResult, error) {
	return payment.AttemptResult{State: payment.StatePending}, nil
}
func (p *callbackAlipayStub) Close(context.Context, string) error { return nil }
func (p *callbackAlipayStub) Verify(context.Context, http.Header, []byte) (commercial.PaymentFact, error) {
	return p.fact, nil
}
func (p *callbackAlipayStub) Refund(context.Context, payment.RefundRequest) (payment.RefundResult, error) {
	return payment.RefundResult{State: payment.StatePending}, nil
}
func (p *callbackAlipayStub) QueryRefund(context.Context, string) (payment.RefundResult, error) {
	return payment.RefundResult{State: payment.StatePending}, nil
}
func (p *callbackAlipayStub) MerchantID() string { return "2088000000000000" }

func newCallbackTestEnv(t *testing.T) (*gorm.DB, *callbackAlipayStub, *gin.Engine, *commercialsvc.OrderService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
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
	stub := &callbackAlipayStub{}
	providers := map[string]payment.Provider{payment.ProviderAlipay: stub}
	orders, err := commercialsvc.NewOrderService(db, providers)
	if err != nil {
		t.Fatal(err)
	}
	callbacks := NewPaymentCallbacksHandler(db, providers)
	engine := gin.New()
	engine.POST("/api/v1/commercial/callbacks/:provider", callbacks.HandleProviderCallback)
	return db, stub, engine, orders
}

func seedCallbackPublishedPlan(t *testing.T, db *gorm.DB) {
	t.Helper()
	def, _ := json.Marshal(commercial.PlanVersion{Key: "pro", Version: 1, Price: 99_00, Monthly: 9_900_000})
	if err := db.Create(&repocommercial.PlanRow{
		PlanKey: "pro", Version: 1, DefinitionJSON: string(def),
		ExternalID: "ext-pro-1", State: commercial.PlanStatePublished,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func notifyFact(t *testing.T, db *gorm.DB, stub *callbackAlipayStub, orderID string) commercial.PaymentFact {
	t.Helper()
	var att repocommercial.PaymentAttemptRow
	if err := db.Where("order_id = ?", orderID).First(&att).Error; err != nil {
		t.Fatalf("read attempt: %v", err)
	}
	if att.Merchant != stub.MerchantID() {
		t.Fatalf("attempt.merchant must be the channel merchant id %q (Provider.MerchantID), got %q — a provider-name literal here makes every genuine callback unresolvable (issue #82 defect 2)",
			stub.MerchantID(), att.Merchant)
	}
	return commercial.PaymentFact{
		Provider: payment.ProviderAlipay, Merchant: stub.MerchantID(),
		AttemptID: att.MerchantOrderID, Transaction: "2026092500000000",
		Amount: commercial.CNYFen(att.AmountFen), Currency: att.Currency,
		State: "succeeded",
	}
}

func outboxCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&repocommercial.OutboxEvent{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

// TestAlipayCallbackConfirmsOrderRegisteredWithChannelMerchant: 下单（真实
// openOrder）→ 匿名签名通知 → 200 "success" + 订单 paid + outbox 恰一；重放
// 幂等。整链复刻流程验证第 9/12 断言（修复前在 attempt 解析步 404）。
func TestAlipayCallbackConfirmsOrderRegisteredWithChannelMerchant(t *testing.T) {
	db, stub, engine, orders := newCallbackTestEnv(t)
	seedCallbackPublishedPlan(t, db)
	ctx := context.Background()
	q, err := orders.CreateQuote(ctx, 601, "pro")
	if err != nil {
		t.Fatal(err)
	}
	order, err := orders.CreateOrder(ctx, 601, q.ID, payment.ProviderAlipay)
	if err != nil {
		t.Fatal(err)
	}
	stub.fact = notifyFact(t, db, stub, order.ID)

	post := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost,
			"/api/v1/commercial/callbacks/alipay", strings.NewReader("notify=1"))
		engine.ServeHTTP(w, req)
		return w
	}

	w := post()
	if w.Code != http.StatusOK || w.Body.String() != "success" {
		t.Fatalf("verified alipay notify must confirm the order (200 \"success\"), got %d %q", w.Code, w.Body.String())
	}
	var row repocommercial.OrderRow
	if err := db.Where("id = ?", order.ID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != commercial.OrderStatePaid {
		t.Fatalf("order state = %s, want paid", row.State)
	}
	if n := outboxCount(t, db); n != 1 {
		t.Fatalf("exactly one fulfill outbox event expected, got %d", n)
	}

	// 重放同一签名通知：幂等 200，不产生第二个事件。
	if w := post(); w.Code != http.StatusOK || w.Body.String() != "success" {
		t.Fatalf("duplicate notify must replay the original success, got %d %q", w.Code, w.Body.String())
	}
	if n := outboxCount(t, db); n != 1 {
		t.Fatalf("duplicate notify must not add events, got %d", n)
	}
}

// TestCallbackMerchantMismatchNeverResolves: 三元组中 merchant 是纵深防御
// 维度——一个不同商户身份的（哪怕验签通过的）事实永远解析不到他人 attempt。
func TestCallbackMerchantMismatchNeverResolves(t *testing.T) {
	db, stub, engine, orders := newCallbackTestEnv(t)
	seedCallbackPublishedPlan(t, db)
	ctx := context.Background()
	q, err := orders.CreateQuote(ctx, 602, "pro")
	if err != nil {
		t.Fatal(err)
	}
	order, err := orders.CreateOrder(ctx, 602, q.ID, payment.ProviderAlipay)
	if err != nil {
		t.Fatal(err)
	}
	fact := notifyFact(t, db, stub, order.ID)
	fact.Merchant = "2088999999999999" // a different (foreign) merchant account
	stub.fact = fact
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/commercial/callbacks/alipay", strings.NewReader("notify=1"))
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("foreign-merchant fact must not resolve any attempt, got %d %q", w.Code, w.Body.String())
	}
	var row repocommercial.OrderRow
	if err := db.Where("id = ?", order.ID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != commercial.OrderStatePending {
		t.Fatalf("foreign-merchant fact must leave the order pending, got %s", row.State)
	}
	if n := outboxCount(t, db); n != 0 {
		t.Fatalf("no event expected, got %d", n)
	}
}
