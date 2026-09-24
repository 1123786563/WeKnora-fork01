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
	view, err := svc.Purchase(context.Background(), 31, q.ID, "wechat", "billing-admin", "WeKnora Space 31")
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
	_, err := svc.Purchase(context.Background(), 32, q.ID, "wechat", "billing-admin", "WeKnora Space 32")
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
	_, err := svc.Purchase(context.Background(), 33, q.ID, "wechat", "x", "WeKnora Space 33")
	if !errors.Is(err, repocommercial.ErrQuoteExpired) {
		t.Fatalf("expired quote must be rejected, got %v", err)
	}
}

func TestPurchaseRetryReturnsExistingOrderWithoutDuplicates(t *testing.T) { // AC4 重试
	svc, fake, cp, _ := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	q := purchaseQuote(t, svc.orders, 34, "pro")
	first, err := svc.Purchase(context.Background(), 34, q.ID, "wechat", "a", "WeKnora Space 34")
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Purchase(context.Background(), 34, q.ID, "wechat", "a", "WeKnora Space 34")
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
	if _, err := svc.Purchase(context.Background(), 35, qPro.ID, "wechat", "a", "WeKnora Space 35"); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Purchase(context.Background(), 35, qMax.ID, "wechat", "a", "WeKnora Space 35")
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

// TestPurchaseFailsFastWhenChannelUnconfigured（审查 F1）：渠道 provider 未
// 配置时必须在任何 seam 外部副作用（gated 订阅 + gating invoice）之前拒绝——
// 否则每次提交都在权威面留下一个不可支付的对象（timeout_hours=0 永不自动取消，
// 取消机制属 #84）。
func TestPurchaseFailsFastWhenChannelUnconfigured(t *testing.T) {
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
	// 渠道为空的 OrderService（blocked-env 形态）。
	orders, err := NewOrderService(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := NewPurchaseService(db, accounts, plans, orders, fake)
	if err != nil {
		t.Fatal(err)
	}
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	q := purchaseQuote(t, svc.orders, 36, "pro")
	_, err = svc.Purchase(context.Background(), 36, q.ID, "wechat", "x", "WeKnora Space 36")
	if !errors.Is(err, ErrPaymentProviderUnconfigured) {
		t.Fatalf("missing channel must fail fast with ErrPaymentProviderUnconfigured, got %v", err)
	}
	// 关键断言：seam 外部零副作用——权威面不得持有任何 gating 订阅。
	if n := len(fake.PurchaseSubscriptions()); n != 0 {
		t.Fatalf("NO authority-side object may exist when the channel is unconfigured, got %d", n)
	}
}

// insertOrderRow 以参数绑定直接落一张订单（干扰行构造，R1-V02 测试用）。
func insertOrderRow(t *testing.T, db *gorm.DB, id string, tenant uint64, quoteID, kind, state string, amount int64) {
	t.Helper()
	if err := db.Exec(`INSERT INTO commercial_orders (id, tenant_id, quote_id, kind, amount_fen, currency, state, version)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1)`, id, tenant, quoteID, kind, amount, "CNY", state).Error; err != nil {
		t.Fatal(err)
	}
}

// (R1-V02) PurchaseStatus 只能挂接与当前购买对应的订单（purchase 类、金额/
// 币种与权威冻结面一致、未完结优先）——租户一旦有历史购买或 upgrade 订单，
// 按 `id DESC` 随机取 rows[0] 会把任意旧订单当作当前购买状态。
func TestPurchaseStatusAttachesOnlyTheCurrentPurchaseOrder(t *testing.T) {
	svc, _, _, db := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	q := purchaseQuote(t, svc.orders, 40, "pro")
	purchased, err := svc.Purchase(context.Background(), 40, q.ID, "wechat", "a", "WeKnora Space 40")
	if err != nil {
		t.Fatal(err)
	}
	// 两张干扰订单：一张金额不同的旧购买（id 字典序更小）和一张 upgrade。
	insertOrderRow(t, db, "ord_0000oldpaid", 40, "qt_oldpaid", "purchase", "paid", 4400)
	insertOrderRow(t, db, "ord_0001upgrade", 40, "qt_upgrade", "upgrade", "paid", 5500)
	// 同金额但已支付（id 字典序更小）——优先未完结（pending）。
	insertOrderRow(t, db, "ord_0002samepaid", 40, "qt_samepaid", "purchase", "paid", 9900)

	out, err := svc.PurchaseStatus(context.Background(), 40)
	if err != nil {
		t.Fatal(err)
	}
	if out.Order == nil || out.Order.ID != purchased.Order.ID {
		t.Fatalf("status must attach the CURRENT purchase order (%+v), got %+v", purchased.Order, out.Order)
	}
	if out.Order.State != "pending" {
		t.Fatalf("an unfinished checkout must win over a paid same-face order, got %+v", out.Order)
	}

	// 无购买的租户即使持有订单也不得挂接（absent 附订单即误导）。
	insertOrderRow(t, db, "ord_9000absent", 41, "qt_absent", "purchase", "pending", 9900)
	absent, err := svc.PurchaseStatus(context.Background(), 41)
	if err != nil {
		t.Fatal(err)
	}
	if absent.State != commercial.PurchaseStateAbsent || absent.Order != nil {
		t.Fatalf("absent purchase must attach no order, got %+v", absent)
	}
}

// (R1-V03) 权威面已 active/canceled 时新报价不得再开渠道订单（active 二次扣款
// 无门控发票、canceled 的门控发票永远无法结算）；同一 quote 的既有订单重放
// 保持放行（付款后 POST 重演返回已付订单）。
func TestPurchaseRefusesNewChannelOrderWhenNotAwaiting(t *testing.T) {
	svc, fake, cp, db := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	q1 := purchaseQuote(t, svc.orders, 42, "pro")
	first, err := svc.Purchase(context.Background(), 42, q1.ID, "wechat", "a", "WeKnora Space 42")
	if err != nil {
		t.Fatal(err)
	}
	// 激活为 active（canceled 走同一 p.State != awaiting_payment 分支）。
	fake.ActivatePurchase(commercial.ExternalPurchaseSubscriptionID(42))

	// 新报价 → 拒开新渠道订单。
	q2 := purchaseQuote(t, svc.orders, 42, "pro")
	_, err = svc.Purchase(context.Background(), 42, q2.ID, "wechat", "a", "WeKnora Space 42")
	if !errors.Is(err, ErrPurchaseNotAwaiting) {
		t.Fatalf("non-awaiting purchase must refuse a NEW order, got %v", err)
	}
	var n int64
	if err := db.Raw(`SELECT COUNT(*) FROM commercial_orders WHERE tenant_id = ?`, uint64(42)).Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("exactly one order may exist, got %d", n)
	}
	if len(cp.createCalls) != 1 {
		t.Fatalf("exactly one channel request may exist, got %d", len(cp.createCalls))
	}

	// 同一 quote 重放（付款后返回页的 POST 重演）仍返回原订单。
	replay, err := svc.Purchase(context.Background(), 42, q1.ID, "wechat", "a", "WeKnora Space 42")
	if err != nil {
		t.Fatalf("same-quote replay must stay allowed: %v", err)
	}
	if replay.Order == nil || replay.Order.ID != first.Order.ID {
		t.Fatalf("replay must answer the SAME order, got %+v vs %+v", replay.Order, first.Order)
	}
}

// (R1-V04) 平台故障期 POST 不再应答 201+absent 视图，而是显式哨兵
// ErrPurchaseUnavailable（handler 映射 503+闭合 reason）；GET PurchaseStatus
// 保持状态视图。platform==nil 同口径。
func TestPurchasePlatformFailureAnswersUnavailable(t *testing.T) {
	svc, fake, _, _ := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	q := purchaseQuote(t, svc.orders, 43, "pro")
	fake.FailSubmitsWith(commercial.ErrPlatformUnreachable)
	_, err := svc.Purchase(context.Background(), 43, q.ID, "wechat", "a", "WeKnora Space 43")
	if !errors.Is(err, ErrPurchaseUnavailable) {
		t.Fatalf("platform failure must answer ErrPurchaseUnavailable, got %v", err)
	}
	if got := PurchaseUnavailableReason(err); got != "unreachable" {
		t.Fatalf("closed reason must be unreachable, got %q", got)
	}

	// GET 状态视图保持不变（不因故障报错）。
	status, err := svc.PurchaseStatus(context.Background(), 43)
	if err != nil {
		t.Fatalf("GET must keep the state-view posture: %v", err)
	}
	if status.State != commercial.PurchaseStateAbsent ||
		(status.Reason != "unreachable" && status.Reason != "") {
		t.Fatalf("GET keeps the closed status view, got %+v", status)
	}
}

func TestPurchaseNilPlatformAnswersUnavailable(t *testing.T) {
	svc, _, _, _ := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	q := purchaseQuote(t, svc.orders, 44, "pro")
	nilPlatform, err := NewPurchaseService(svc.db, svc.accounts, svc.plans, svc.orders, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = nilPlatform.Purchase(context.Background(), 44, q.ID, "wechat", "a", "WeKnora Space 44")
	if !errors.Is(err, ErrPurchaseUnavailable) {
		t.Fatalf("nil platform POST must answer ErrPurchaseUnavailable, got %v", err)
	}
	if got := PurchaseUnavailableReason(err); got != "unconfigured" {
		t.Fatalf("closed reason must be unconfigured, got %q", got)
	}
}

// (R1-V18) 损坏的已发布版本定义（反序列化失败）是数据问题：必须以
// ErrPurchasePlanInvalid 应答，绝不能借 ErrQuoteLegacySnapshot 诱导用户
// 反复重新报价（同一损坏每次重报都会再次失败）。已发布定义受
// published_plan_immutable 触发器保护，因此用"损坏的 catalog 行 + 报价
// 快照指向该版本"构造这一数据损坏形态（绕过正常生命周期——正是现实里
// 数据损坏的出现方式）。
func TestPurchaseCorruptPlanDefinitionIsNotALegacyQuote(t *testing.T) {
	svc, _, _, db := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	q := purchaseQuote(t, svc.orders, 45, "pro")
	// 损坏版本行（version 2，直接落库为已发布态）+ 对应 publication 行，
	// 再把报价快照的 plan_version 指到 version 2（参数绑定）。
	if err := db.Exec(`INSERT INTO commercial_plan_catalog (plan_key, version, state, definition_json, external_id)
		VALUES (?, ?, ?, ?, ?)`, "pro", 2, "published", "{broken",
		commercial.DeterministicPlanCode("pro", 2)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO commercial_plan_publications
		(command_key, plan_key, version, plan_code, receipt_json, published_by, published_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"publish_plan_version:pro:2", "pro", 2, commercial.DeterministicPlanCode("pro", 2),
		"{}", "test", time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE commercial_quotes SET snapshot_json = json_set(snapshot_json, '$.plan_version', 2)
		WHERE id = ?`, q.ID).Error; err != nil {
		t.Fatal(err)
	}
	_, err := svc.Purchase(context.Background(), 45, q.ID, "wechat", "a", "WeKnora Space 45")
	if !errors.Is(err, ErrPurchasePlanInvalid) {
		t.Fatalf("corrupt definition must answer ErrPurchasePlanInvalid, got %v", err)
	}
	if errors.Is(err, ErrQuoteLegacySnapshot) {
		t.Fatal("a corrupt definition must NOT be reported as a legacy snapshot (re-quote trap)")
	}
}
