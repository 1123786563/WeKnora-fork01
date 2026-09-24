package handler

// POST /commercial/purchases 的 HTTP 契约回归（issue-81 R1 批次）：
// R1-V14 渠道失败但订单已落库 → 202（与 POST /orders 同一失败模式同语义）；
// R1-V21 报价消费竞态败者 → 409（与 /orders 对齐，不再答 400）；
// R1-V04 平台故障 → 503 + 闭合 reason（不再 201 + fabricated-absent）；
// R1-V17 actor/display name 来自真实调用者与租户名（不再 "billing-admin"/"space"）。

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/commercial"
	"github.com/Tencent/WeKnora/internal/modules/commercial/commercialplatform"
	"github.com/Tencent/WeKnora/internal/modules/commercial/payment"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial"
	"github.com/Tencent/WeKnora/internal/types"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// handlerStubProvider is the minimal channel double for the purchase handler
// contract tests; createErr lets a test drive the "order durably pending but
// the channel call failed" branch.
type handlerStubProvider struct {
	mu          sync.Mutex
	createErr   error
	createCalls int
}

func (p *handlerStubProvider) Create(context.Context, payment.OrderRequest) (payment.AttemptResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.createCalls++
	if p.createErr != nil {
		return payment.AttemptResult{}, p.createErr
	}
	return payment.AttemptResult{State: payment.StatePending, CheckoutURL: "https://pay.example/qr"}, nil
}

func (p *handlerStubProvider) Query(context.Context, string) (payment.AttemptResult, error) {
	return payment.AttemptResult{State: payment.StatePending}, nil
}
func (p *handlerStubProvider) Close(context.Context, string) error { return nil }
func (p *handlerStubProvider) Verify(context.Context, http.Header, []byte) (commercial.PaymentFact, error) {
	return commercial.PaymentFact{}, nil
}
func (p *handlerStubProvider) Refund(context.Context, payment.RefundRequest) (payment.RefundResult, error) {
	return payment.RefundResult{State: payment.StatePending}, nil
}
func (p *handlerStubProvider) QueryRefund(context.Context, string) (payment.RefundResult, error) {
	return payment.RefundResult{State: payment.StatePending}, nil
}

type purchaseHandlerEnv struct {
	handler  *CommercialHandler
	router   *gin.Engine
	fake     *commercialplatform.FakeAdapter
	orders   *commercialsvc.OrderService
	plans    *commercialsvc.PlanVersionService
	db       *gorm.DB
	provider *handlerStubProvider
}

func newPurchaseHandlerEnv(t *testing.T) *purchaseHandlerEnv {
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
		&repocommercial.Subscription{}, &repocommercial.BillingAccount{}); err != nil {
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
	provider := &handlerStubProvider{}
	orders, err := commercialsvc.NewOrderService(db, map[string]payment.Provider{"wechat": provider})
	if err != nil {
		t.Fatal(err)
	}
	purchases, err := commercialsvc.NewPurchaseService(db, accounts, plans, orders, fake)
	if err != nil {
		t.Fatal(err)
	}
	h := NewCommercialHandler(db)
	h.SetPurchaseService(purchases)

	router := gin.New()
	router.POST("/api/v1/commercial/purchases", func(c *gin.Context) {
		// The authenticated scope the Auth middleware would install: the
		// handler must derive tenant AND user exclusively from here.
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(50))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "user-50")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}, h.Purchase)

	return &purchaseHandlerEnv{handler: h, router: router, fake: fake, orders: orders, plans: plans, db: db, provider: provider}
}

func seedHandlerPlan(t *testing.T, plans *commercialsvc.PlanVersionService) {
	t.Helper()
	view, err := plans.CreateDraft(context.Background(), "test:seed", commercialsvc.DraftInput{
		PlanKey: "pro", Name: "pro Plan", AmountFen: 9900, IncludedCreditsMicro: 9_900_000,
		Features: map[string]bool{"advanced_models": true}, Currency: commercial.CurrencyCNY,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plans.Publish(context.Background(), "test:seed", "seed", "pro", view.Version); err != nil {
		t.Fatal(err)
	}
}

func postPurchase(t *testing.T, env *purchaseHandlerEnv, quoteID string) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"quote_id":"` + quoteID + `","provider":"wechat"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/commercial/purchases", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	return rec
}

// (R1-V14) 渠道调用失败但订单已原子落库：Purchase 必须与 POST /orders 一致答
// 202（客户端经 GET /orders/:id 恢复），不再把渠道失败的结账当干净创建答 201。
func TestPurchaseHandlerAnswersAcceptedWhenCheckoutFailed(t *testing.T) {
	env := newPurchaseHandlerEnv(t)
	seedHandlerPlan(t, env.plans)
	q, err := env.orders.CreateQuote(context.Background(), 50, "pro")
	if err != nil {
		t.Fatal(err)
	}
	// 让渠道 Create 失败（订单仍原子落库）。
	env.provider.createErr = errors.New("channel down")
	rec := postPurchase(t, env, q.ID)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("checkout failure with a durable order must answer 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"checkout_error"`)) {
		t.Fatalf("202 answer must carry checkout_error: %s", rec.Body.String())
	}
}

// (R1-V21) 报价消费竞态的败者（回读也失败）必须答 409 quote already used，
// 与 POST /orders 对齐，不再落入 default 的 400。
func TestPurchaseHandlerAnswersConflictOnQuoteAlreadyUsed(t *testing.T) {
	env := newPurchaseHandlerEnv(t)
	seedHandlerPlan(t, env.plans)
	q, err := env.orders.CreateQuote(context.Background(), 50, "pro")
	if err != nil {
		t.Fatal(err)
	}
	// 用另一租户的订单占用同一 quote_id：本租户的 GetOrderByQuote 落空
	// （租户限定读取），CreateOrder 的 quote 唯一约束随即判 already-used，
	// 胜者订单又因租户不同回读不到——正是竞态败者 + 回读失败的形态。
	if err := env.db.Exec(`INSERT INTO commercial_orders (id, tenant_id, quote_id, kind, amount_fen, currency, state, version)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1)`, "ord_winner", uint64(51), q.ID, "purchase", 9900, "CNY", "pending").Error; err != nil {
		t.Fatal(err)
	}
	rec := postPurchase(t, env, q.ID)
	if rec.Code != http.StatusConflict {
		t.Fatalf("lost quote race must answer 409, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("quote already used")) {
		t.Fatalf("409 body must say quote already used: %s", rec.Body.String())
	}
}

// (R1-V04) 平台故障期 POST 必须答 503 + 闭合 reason（unreachable），绝不答
// 201 + fabricated-absent 视图。
func TestPurchaseHandlerAnswersUnavailableOnPlatformFailure(t *testing.T) {
	env := newPurchaseHandlerEnv(t)
	seedHandlerPlan(t, env.plans)
	q, err := env.orders.CreateQuote(context.Background(), 50, "pro")
	if err != nil {
		t.Fatal(err)
	}
	env.fake.FailSubmitsWith(commercial.ErrPlatformUnreachable)
	rec := postPurchase(t, env, q.ID)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("platform failure must answer 503, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"reason":"unreachable"`)) {
		t.Fatalf("503 must carry the closed reason token: %s", rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte(`"state"`)) {
		t.Fatalf("503 must not carry a fabricated purchase state: %s", rec.Body.String())
	}
}

// (R1-V17) ensure 显示名来自 handler 的 tenantDisplayName（无 tenants 表时
// 回退 "WeKnora Space 50"），不再硬编码 "space"；actor 参数同样来自
// commercialUserID(c)（认证中间件注入的 user-50）——fake 的客户记录面
// 观察到的即 ensure 载荷的真实 DisplayName。
func TestPurchaseHandlerCarriesRealActorAndDisplayName(t *testing.T) {
	env := newPurchaseHandlerEnv(t)
	seedHandlerPlan(t, env.plans)
	q, err := env.orders.CreateQuote(context.Background(), 50, "pro")
	if err != nil {
		t.Fatal(err)
	}
	rec := postPurchase(t, env, q.ID)
	if rec.Code != http.StatusCreated {
		t.Fatalf("healthy purchase must answer 201, got %d: %s", rec.Code, rec.Body.String())
	}
	customers := env.fake.Customers()
	if len(customers) != 1 {
		t.Fatalf("exactly one ensured customer expected, got %d", len(customers))
	}
	if customers[0].Name != "WeKnora Space 50" {
		t.Fatalf("ensure display name must come from tenantDisplayName, got %q", customers[0].Name)
	}
}
