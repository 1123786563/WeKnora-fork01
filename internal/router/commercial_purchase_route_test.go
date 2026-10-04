package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/handler"
	commercial "github.com/Tencent/WeKnora/internal/modules/commercial"
	commercialplatform "github.com/Tencent/WeKnora/internal/modules/commercial/commercialplatform"
	"github.com/Tencent/WeKnora/internal/modules/commercial/payment"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial"
	"github.com/gin-gonic/gin"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// routePurchaseProvider 计数渠道 Create（AC2「不拉起支付请求」断言面）。
type routePurchaseProvider struct{ creates int }

func (p *routePurchaseProvider) Create(context.Context, payment.OrderRequest) (payment.AttemptResult, error) {
	p.creates++
	return payment.AttemptResult{State: payment.StatePending, CheckoutURL: "https://pay.example/qr"}, nil
}
func (p *routePurchaseProvider) Query(context.Context, string) (payment.AttemptResult, error) {
	return payment.AttemptResult{State: payment.StatePending}, nil
}
func (p *routePurchaseProvider) Close(context.Context, string) error { return nil }
func (p *routePurchaseProvider) Verify(context.Context, http.Header, []byte) (commercial.PaymentFact, error) {
	return commercial.PaymentFact{}, nil
}
func (p *routePurchaseProvider) Refund(context.Context, payment.RefundRequest) (payment.RefundResult, error) {
	return payment.RefundResult{State: payment.StatePending}, nil
}
func (p *routePurchaseProvider) QueryRefund(context.Context, string) (payment.RefundResult, error) {
	return payment.RefundResult{State: payment.StatePending}, nil
}

// MerchantID models the channel merchant identity contract (NOT the provider
// name — see the Provider interface doc).
func (p *routePurchaseProvider) MerchantID() string { return "1900000109" }

// newPurchaseEngine 组装真实 PurchaseService 链 + 渠道计数 stub。返回
// engine/db/fake/provider 供断言，plans/orders 供用例做
// draft→publish→quote seed。
func newPurchaseEngine(t *testing.T) (*gin.Engine, *gorm.DB, *commercialplatform.FakeAdapter, *routePurchaseProvider, *commercialsvc.PlanVersionService, *commercialsvc.OrderService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&repocommercial.OrderRow{}, &repocommercial.PaymentAttemptRow{},
		&repocommercial.OutboxEvent{}, &repocommercial.PlanRow{}, &repocommercial.QuoteRow{},
		&repocommercial.Subscription{}, &repocommercial.BillingAccount{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS tenants (
		id INTEGER PRIMARY KEY, name TEXT NOT NULL DEFAULT '', storage_used INTEGER NOT NULL DEFAULT 0
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS tenant_members (
		id INTEGER PRIMARY KEY AUTOINCREMENT, user_id TEXT, tenant_id INTEGER NOT NULL,
		status TEXT NOT NULL DEFAULT 'active', deleted_at DATETIME
	)`).Error; err != nil {
		t.Fatal(err)
	}
	fake := commercialplatform.NewFakeAdapter()
	accounts, err := commercialsvc.NewBillingAccountService(db, fake)
	if err != nil {
		t.Fatal(err)
	}
	plans, err := commercialsvc.NewPlanVersionService(db, fake)
	if err != nil {
		t.Fatal(err)
	}
	provider := &routePurchaseProvider{}
	orders, err := commercialsvc.NewOrderService(db, map[string]payment.Provider{"wechat": provider})
	if err != nil {
		t.Fatal(err)
	}
	purchases, err := commercialsvc.NewPurchaseService(db, accounts, plans, orders, fake)
	if err != nil {
		t.Fatal(err)
	}
	h := handler.NewCommercialHandler(db)
	h.SetOrderService(orders)
	h.SetPurchaseService(purchases)
	engine := gin.New()
	v1 := engine.Group("/api/v1")
	RegisterCommercialRoutes(v1, h, nil)
	return engine, db, fake, provider, plans, orders
}

// routeSeedPlanAndQuote 走真实 draft→publish→quote 链。
func routeSeedPlanAndQuote(t *testing.T, engine *gin.Engine, plans *commercialsvc.PlanVersionService, orders *commercialsvc.OrderService, tenant uint64, planKey string) commercialsvc.QuoteView {
	t.Helper()
	_ = engine
	view, err := plans.CreateDraft(context.Background(), "route:seed", commercialsvc.DraftInput{
		PlanKey: planKey, Name: planKey, AmountFen: 9900, IncludedCreditsMicro: 9_900_000,
		Features: map[string]bool{"advanced_models": true}, Currency: commercial.CurrencyCNY,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plans.Publish(context.Background(), "route:seed", "seed", planKey, view.Version); err != nil {
		t.Fatal(err)
	}
	q, err := orders.CreateQuote(context.Background(), tenant, planKey)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

// purchaseReq 以 owner 身份发一次经鉴权的 commercial 请求。
func purchaseReq(t *testing.T, engine *gin.Engine, tenant uint64, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	served := gin.New()
	served.Use(authAs(tenant, "user-1", "owner"))
	served.Handle(method, "/api/v1/*rest", func(c *gin.Context) { engine.ServeHTTP(c.Writer, c.Request) })
	served.ServeHTTP(w, req)
	return w
}

// purchasePost 以 owner 身份 POST /commercial/purchases。
func purchasePost(t *testing.T, engine *gin.Engine, tenant uint64, body string) *httptest.ResponseRecorder {
	t.Helper()
	return purchaseReq(t, engine, tenant, http.MethodPost, "/api/v1/commercial/purchases", body)
}

func TestPurchaseRouteHappyPathAwaitingPayment(t *testing.T) {
	engine, _, _, provider, plans, orders := newPurchaseEngine(t)
	q := routeSeedPlanAndQuote(t, engine, plans, orders, 41, "pro")
	w := purchasePost(t, engine, 41, `{"quote_id":"`+q.ID+`","provider":"wechat"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	for _, want := range []string{
		`"state":"awaiting_payment"`, `"checkout_url"`, `"amount_fen":"9900"`, `"plan_key":"pro"`,
	} {
		if !strings.Contains(w.Body.String(), want) {
			t.Fatalf("body must contain %s, got %s", want, w.Body.String())
		}
	}
	if provider.creates != 1 {
		t.Fatalf("channel creates = %d, want 1", provider.creates)
	}
}

func TestPurchaseRouteMismatchReturns409WithoutOrder(t *testing.T) {
	engine, _, fake, provider, plans, orders := newPurchaseEngine(t)
	q := routeSeedPlanAndQuote(t, engine, plans, orders, 42, "pro")
	// 预置偏差：权威面订阅金额 8800 ≠ Quote 9900（Task 7 同法）。
	if _, err := fake.SubmitCommand(context.Background(), commercial.Command{
		Kind: commercial.CommandKindCreatePurchaseSubscription,
		Key: commercial.CreatePurchaseSubscriptionCommandKey(
			commercial.ExternalPurchaseSubscriptionID(42), commercial.DeterministicPlanCode("pro", 1)),
		Payload: commercial.CreatePurchaseSubscriptionPayload{
			TenantID: 42, ExternalCustomerID: commercial.ExternalCustomerID(42),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(42),
			PlanCode:                       commercial.DeterministicPlanCode("pro", 1),
			AmountFen:                      8800, Currency: commercial.CurrencyCNY,
		},
	}); err != nil {
		t.Fatal(err)
	}
	w := purchasePost(t, engine, 42, `{"quote_id":"`+q.ID+`","provider":"wechat"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "invoice_quote_mismatch") {
		t.Fatalf("body must carry the closed mismatch token, got %s", w.Body.String())
	}
	if provider.creates != 0 {
		t.Fatalf("NO channel request may fire on mismatch, creates = %d", provider.creates)
	}
	w2 := purchaseReq(t, engine, 42, http.MethodGet, "/api/v1/commercial/orders", "")
	if w2.Code != http.StatusOK || strings.Contains(w2.Body.String(), `"id":"ord_`) {
		t.Fatalf("orders must be empty, code=%d body=%s", w2.Code, w2.Body.String())
	}
}

func TestPurchaseRouteExpiredQuoteReturns409(t *testing.T) {
	engine, db, _, _, plans, orders := newPurchaseEngine(t)
	q := routeSeedPlanAndQuote(t, engine, plans, orders, 43, "pro")
	// 参数绑定过期（S2：无拼接 SQL）。
	if err := db.Exec(`UPDATE commercial_quotes SET expires_at = ? WHERE id = ?`,
		time.Now().Add(-time.Minute), q.ID).Error; err != nil {
		t.Fatal(err)
	}
	w := purchasePost(t, engine, 43, `{"quote_id":"`+q.ID+`","provider":"wechat"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "quote expired") {
		t.Fatalf("body must carry the expired token, got %s", w.Body.String())
	}
}

func TestPurchaseRouteStatusAnswersState(t *testing.T) {
	engine, _, _, _, plans, orders := newPurchaseEngine(t)
	q := routeSeedPlanAndQuote(t, engine, plans, orders, 44, "pro")
	if w := purchasePost(t, engine, 44, `{"quote_id":"`+q.ID+`","provider":"wechat"}`); w.Code != http.StatusCreated {
		t.Fatalf("purchase status = %d body=%s", w.Code, w.Body.String())
	}
	w := purchaseReq(t, engine, 44, http.MethodGet, "/api/v1/commercial/purchase", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"awaiting_payment"`) {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	// 另一未购买租户：诚实 absent。
	other := purchaseReq(t, engine, 45, http.MethodGet, "/api/v1/commercial/purchase", "")
	if other.Code != http.StatusOK || !strings.Contains(other.Body.String(), `"absent"`) {
		t.Fatalf("untouched tenant must answer absent, status = %d body = %s", other.Code, other.Body.String())
	}
}
