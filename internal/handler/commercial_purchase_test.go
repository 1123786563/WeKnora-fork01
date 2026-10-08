package handler

// POST /commercial/purchases 的 HTTP 契约回归（issue-81 R1 批次）：
// R1-V14 渠道失败但订单已落库 → 202（与 POST /orders 同一失败模式同语义）；
// R1-V21 报价消费竞态败者 → 409（与 /orders 对齐，不再答 400）；
// R1-V04 平台故障 → 503 + 闭合 reason（不再 201 + fabricated-absent）；
// R1-V17 actor/display name 来自真实调用者与租户名（不再 "billing-admin"/"space"）。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/commercial/commercialplatform"
	"github.com/Tencent/WeKnora/internal/commercial/payment"
	repocommercial "github.com/Tencent/WeKnora/internal/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/commercial/service/commercial"
	"github.com/Tencent/WeKnora/internal/types"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"strings"
	"time"
)

// handlerStubProvider is the minimal channel double for the purchase handler
// contract tests; createErr lets a test drive the "order durably pending but
// the channel call failed" branch.
type handlerStubProvider struct {
	mu          sync.Mutex
	createErr   error
	queryErr    error
	closeErr    error
	createCalls int
	queryCalls  int
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
	p.mu.Lock()
	p.queryCalls++
	defer p.mu.Unlock()
	if p.queryErr != nil {
		return payment.AttemptResult{}, p.queryErr
	}
	return payment.AttemptResult{State: payment.StatePending}, nil
}
func (p *handlerStubProvider) Close(context.Context, string) error { return p.closeErr }
func (p *handlerStubProvider) Verify(context.Context, http.Header, []byte) (commercial.PaymentFact, error) {
	return commercial.PaymentFact{}, nil
}
func (p *handlerStubProvider) Refund(context.Context, payment.RefundRequest) (payment.RefundResult, error) {
	return payment.RefundResult{State: payment.StatePending}, nil
}
func (p *handlerStubProvider) QueryRefund(context.Context, string) (payment.RefundResult, error) {
	return payment.RefundResult{State: payment.StatePending}, nil
}

// MerchantID models the channel merchant identity contract (NOT the provider
// name — see the Provider interface doc).
func (p *handlerStubProvider) MerchantID() string { return "1900000109" }

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
	orders, err := commercialsvc.NewOrderService(db, map[string]payment.Provider{"wechat": provider, "alipay": provider})
	if err != nil {
		t.Fatal(err)
	}
	purchases, err := commercialsvc.NewPurchaseService(db, accounts, plans, orders, fake)
	if err != nil {
		t.Fatal(err)
	}
	h := NewCommercialHandler(db)
	h.SetPurchaseService(purchases)
	h.SetOrderService(orders)

	router := gin.New()
	router.POST("/api/v1/commercial/purchases", func(c *gin.Context) {
		// The authenticated scope the Auth middleware would install: the
		// handler must derive tenant AND user exclusively from here.
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(50))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "user-50")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}, h.Purchase)
	router.GET("/api/v1/commercial/orders/:id", func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(50))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}, h.GetOrder)

	return &purchaseHandlerEnv{handler: h, router: router, fake: fake, orders: orders, plans: plans, db: db, provider: provider}
}

func assertPaymentObservationUnavailable(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusServiceUnavailable || strings.TrimSpace(rec.Body.String()) != `{"error":"payment status temporarily unavailable","reason":"payment_status_unavailable"}` {
		t.Fatalf("expected closed payment observation 503, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "provider-private-detail") {
		t.Fatalf("provider detail leaked in response: %s", rec.Body.String())
	}
}

func TestGetOrderHandlerClosesUnavailablePaymentObservation(t *testing.T) {
	env := newPurchaseHandlerEnv(t)
	seedHandlerPlan(t, env.plans)
	q, err := env.orders.CreateQuote(context.Background(), 50, "pro")
	if err != nil {
		t.Fatal(err)
	}
	order, err := env.orders.CreateOrder(context.Background(), 50, q.ID, "wechat")
	if err != nil {
		t.Fatal(err)
	}
	env.provider.queryErr = errors.New("provider-private-detail")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/commercial/orders/"+order.ID, nil)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	assertPaymentObservationUnavailable(t, rec)
}

func TestPurchaseChannelSwitchClosesUnavailablePaymentObservation(t *testing.T) {
	env := newPurchaseHandlerEnv(t)
	seedHandlerPlan(t, env.plans)
	q, err := env.orders.CreateQuote(context.Background(), 50, "pro")
	if err != nil {
		t.Fatal(err)
	}
	if rec := postPurchase(t, env, q.ID); rec.Code != http.StatusCreated {
		t.Fatalf("fixture purchase failed: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := repocommercial.NewOrderStore(env.db).CurrentPendingPurchaseOrder(context.Background(), 50, 9900, "CNY"); err != nil {
		t.Fatalf("fixture must have current payable pending order: %v", err)
	}
	q, err = env.orders.CreateQuote(context.Background(), 50, "pro")
	if err != nil {
		t.Fatal(err)
	}
	env.provider.queryErr = errors.New("provider-private-detail")
	env.provider.closeErr = errors.New("close outcome uncertain")
	body := `{"quote_id":"` + q.ID + `","provider":"alipay"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/commercial/purchases", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	if env.provider.queryCalls != 1 {
		t.Fatalf("channel switch must query after close error, query calls=%d", env.provider.queryCalls)
	}
	assertPaymentObservationUnavailable(t, rec)
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
// (review R82-2 / OCR r4) A nonexistent quote id is a CLIENT fact: the
// tenant-guarded quote read miss answers 404 with the closed token — never
// the residual 500 (which invites retrying a deterministic failure).
func TestPurchaseHandlerAnswersNotFoundOnMissingQuote(t *testing.T) {
	env := newPurchaseHandlerEnv(t)
	seedHandlerPlan(t, env.plans)
	rec := postPurchase(t, env, "qt_does_not_exist")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("a missing quote must answer 404, got HTTP %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "quote not found") {
		t.Fatalf("the closed token must ride the answer, got %s", rec.Body.String())
	}
}

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

// (R1-09) 未映射的基础设施错误（存储层故障）必须答 500 + 固定闭合文案，
// 不得 400（诱导调用方按 4xx 语义重试同一请求）也不得把 SQL/驱动错误文本
// 透到公网边界——与本端点 503 分支自述的闭合词汇约束一致。
func TestPurchaseHandlerAnswersServerErrorClosedOnInfrastructureFailure(t *testing.T) {
	env := newPurchaseHandlerEnv(t)
	seedHandlerPlan(t, env.plans)
	q, err := env.orders.CreateQuote(context.Background(), 50, "pro")
	if err != nil {
		t.Fatal(err)
	}
	// 基础设施故障：报价表被破坏后 QuoteSnapshotForTenant 的存储层读失败，
	// 落到 Purchase switch 的 default 分支。
	if err := env.db.Exec(`DROP TABLE commercial_quotes`).Error; err != nil {
		t.Fatal(err)
	}
	rec := postPurchase(t, env, q.ID)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("infrastructure failure must answer 500, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"purchase failed"`)) {
		t.Fatalf("500 must carry the closed message: %s", rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("no such table")) {
		t.Fatalf("raw driver error text must not cross the boundary: %s", rec.Body.String())
	}
}

// (R1-V17) ensure 显示名来自 handler 的 tenantDisplayName（无 tenants 表时
// 回退 "WeKnora Space 50"），不再硬编码 "space"；actor 参数同样来自
// commercialUserID(c)（认证中间件注入的 user-50）——fake 的客户记录面
// 观察到的即 ensure 载荷的真实 DisplayName。
// (R3-25) POST /commercial/orders（遗留路径，无 PurchaseService 前置检查）：
// OpenOrder 插入撞部分唯一索引返回 ErrPurchasePendingExists 时，必须答 409
// 并附带既有可付 pending 订单（客户端保住支付入口），而不是落 default 的
// 400 + 裸令牌 "purchase_pending_exists"。
func TestCreateOrderHandlerMapsPendingExistsWithReplay(t *testing.T) {
	env := newPurchaseHandlerEnv(t)
	seedHandlerPlan(t, env.plans)
	ctx := context.Background()
	// 既有可付 pending（不同 quote，渠道成功 + 链接已持久化）。
	q1, err := env.orders.CreateQuote(ctx, 50, "pro")
	if err != nil {
		t.Fatal(err)
	}
	first, err := env.orders.CreateOrder(ctx, 50, q1.ID, "wechat")
	if err != nil {
		t.Fatal(err)
	}
	if first.CheckoutURL == "" {
		t.Fatalf("the existing order must be payable, got %+v", first)
	}
	if err := repocommercial.NewOrderStore(env.db).RecordPaymentAnomaly(ctx, repocommercial.PaymentAnomalyRow{TenantID: 50, OrderID: first.ID, AttemptID: "attempt", Provider: "wechat", Merchant: "merchant", Transaction: "txn-attention", Kind: repocommercial.PaymentAnomalyKindAmount, ExpectedCurrency: "CNY", ActualCurrency: "CNY"}); err != nil {
		t.Fatal(err)
	}
	// 新 quote 走遗留 POST /orders。
	q2, err := env.orders.CreateQuote(ctx, 50, "pro")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	env.handler.SetOrderService(env.orders)
	router.POST("/api/v1/commercial/orders", func(c *gin.Context) {
		cctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(50))
		cctx = context.WithValue(cctx, types.UserIDContextKey, "user-50")
		c.Request = c.Request.WithContext(cctx)
		c.Next()
	}, env.handler.CreateOrder)
	body := `{"quote_id":"` + q2.ID + `","provider":"wechat"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/commercial/orders", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("pending-exists must answer 409, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("purchase pending exists")) {
		t.Fatalf("409 must carry the closed message: %s", rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(first.ID)) {
		t.Fatalf("409 must attach the existing payable order for replay: %s", rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(first.CheckoutURL)) {
		t.Fatalf("the replayed order must carry the payment entry: %s", rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"payment_attention":true`)) {
		t.Fatalf("duplicate checkout replay must carry unresolved payment attention: %s", rec.Body.String())
	}
	var anomaly repocommercial.PaymentAnomalyRow
	if err := env.db.Where("order_id = ?", first.ID).First(&anomaly).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repocommercial.NewOrderStore(env.db).ResolvePaymentAnomaly(ctx, anomaly.ID, anomaly.Version); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/commercial/orders", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("resolved replay must retain conflict status, got %d: %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Order map[string]any `json:"order"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode resolved replay: %v: %s", err, rec.Body.String())
	}
	if response.Order["id"] != first.ID || response.Order["checkout_url"] != first.CheckoutURL {
		t.Fatalf("resolved replay lost payable order: %#v", response.Order)
	}
	if rawAttention, present := response.Order["payment_attention"]; present {
		attention, ok := rawAttention.(bool)
		if !ok || attention {
			t.Fatalf("resolved replay must omit payment_attention or encode boolean false: %#v", response.Order)
		}
	}
}

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

// ---- #82 Task 13 (OCR r2): purchase race hardening ----

// TestPurchaseHandlerAnswersConflictOnPendingExists（D12 / r2:306 medium）：
// 并发 checkout 败者在 winner 渠道 Create 窗口内（link-less pending 占索引、冲
// 突回读不可付）时，服务层把 ErrPurchasePendingExists 哨兵交还——handler 必须
// 答 409 + 闭合令牌（可重试语义），绝不落 default 的 500（"server-side, never
// invites retry"——与实际所需正好相反）。
func TestPurchaseHandlerAnswersConflictOnPendingExists(t *testing.T) {
	env := newPurchaseHandlerEnv(t)
	seedHandlerPlan(t, env.plans)
	// 权威面 awaiting 购买（fake seam 真实命令）。
	if _, err := env.fake.SubmitCommand(context.Background(), commercial.Command{
		Kind: commercial.CommandKindCreatePurchaseSubscription,
		Key: commercial.CreatePurchaseSubscriptionCommandKey(
			commercial.ExternalPurchaseSubscriptionID(50), commercial.DeterministicPlanCode("pro", 1)),
		Actor: "test", Reason: "purchase",
		Payload: commercial.CreatePurchaseSubscriptionPayload{
			TenantID: 50, ExternalCustomerID: commercial.ExternalCustomerID(50),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(50),
			PlanCode:                       commercial.DeterministicPlanCode("pro", 1),
			AmountFen:                      9900, Currency: commercial.CurrencyCNY,
		},
	}); err != nil {
		t.Fatal(err)
	}
	// winner 正在渠道 Create 窗口：pending、未 channel_failed、URL 空、新鲜
	// （占索引、不被 sweep、冲突回读不可付）。
	if err := env.db.Exec(`INSERT INTO commercial_orders (id, tenant_id, quote_id, kind, amount_fen, currency, state, version, created_at, checkout_url, channel_failed)
		VALUES ('ord_winner_win', 50, 'qt_winner', 'purchase', 9900, 'CNY', 'pending', 1, ?, '', 0)`,
		time.Now().UTC().Format(time.RFC3339Nano)).Error; err != nil {
		t.Fatal(err)
	}
	q, err := env.orders.CreateQuote(context.Background(), 50, "pro")
	if err != nil {
		t.Fatal(err)
	}
	rec := postPurchase(t, env, q.ID)
	if rec.Code != http.StatusConflict {
		t.Fatalf("an unreplayable pending-exists race must answer 409, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("purchase pending exists")) {
		t.Fatalf("409 must carry the closed message, got %s", rec.Body.String())
	}
}

// TestCreateOrderHandlerPendingExistsForeignPlanBare409（r2:818 medium，
// A-21/F105 一致性缺口）：legacy POST /orders 挂回的 pending 订单必须有 plan
// 归属证明——既有可付 pending 的 quote 买的另一 plan 时，挂回的 checkout_url
// 会结算异 plan 的 quote（orderWire 无 plan 字段，客户端无法分辨），必须答裸
// 409 不附 order。
func TestCreateOrderHandlerPendingExistsForeignPlanBare409(t *testing.T) {
	env := newPurchaseHandlerEnv(t)
	seedHandlerPlan(t, env.plans)
	// 第二个 plan（合法阶梯价 29900——订单行价面手插 9900 构造 republish 窗口
	// 的同价异 plan 形态）。
	maxDraft, err := env.plans.CreateDraft(context.Background(), "test:seed", commercialsvc.DraftInput{
		PlanKey: "pro-max", Name: "pro-max Plan", AmountFen: 29900, IncludedCreditsMicro: 29_900_000,
		Features: map[string]bool{"advanced_models": true}, Currency: commercial.CurrencyCNY,
	})
	if err != nil {
		t.Fatal(err)
	}
	// plan A 的既有可付 pending。
	qPro, err := env.orders.CreateQuote(context.Background(), 50, "pro")
	if err != nil {
		t.Fatal(err)
	}
	first, err := env.orders.CreateOrder(context.Background(), 50, qPro.ID, "wechat")
	if err != nil {
		t.Fatal(err)
	}
	// plan B 发布 + 新 quote 走遗留 POST /orders。
	if _, err := env.plans.Publish(context.Background(), "test:seed", "seed", "pro-max", maxDraft.Version); err != nil {
		t.Fatal(err)
	}
	qMax, err := env.orders.CreateQuote(context.Background(), 50, "pro-max")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	env.handler.SetOrderService(env.orders)
	router.POST("/api/v1/commercial/orders", func(c *gin.Context) {
		cctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(50))
		cctx = context.WithValue(cctx, types.UserIDContextKey, "user-50")
		c.Request = c.Request.WithContext(cctx)
		c.Next()
	}, env.handler.CreateOrder)
	body := `{"quote_id":"` + qMax.ID + `","provider":"wechat"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/commercial/orders", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("a foreign-plan pending must answer 409, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("purchase pending exists")) {
		t.Fatalf("409 must carry the closed message, got %s", rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte(first.ID)) || bytes.Contains(rec.Body.Bytes(), []byte(first.CheckoutURL)) {
		t.Fatalf("a foreign-plan order must NOT be attached as the replay entry, got %s", rec.Body.String())
	}
}
