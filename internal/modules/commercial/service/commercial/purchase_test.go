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
	// (R1-35) The replay re-serves the persisted checkout_url verbatim: the
	// quote is already consumed, so the link in the replay answer is the
	// ONLY way back to the payment page after a lost first answer.
	if second.Order.CheckoutURL == "" || second.Order.CheckoutURL != first.Order.CheckoutURL {
		t.Fatalf("replay must carry the persisted checkout_url verbatim, first=%q second=%q",
			first.Order.CheckoutURL, second.Order.CheckoutURL)
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

// TestPurchaseSecondQuoteReplaysExistingPendingOrder（R1-22）：同一 awaiting
// 购买的幂等键此前只有 quote——客户端换一张新 quote（过期重报价/双开标签页
// 各自 POST /quotes）再提交时，第 8 步会为同一 gating invoice 开出第二张并存
// 的可支付渠道订单（两张各自可付、各自回调 ConfirmPayment → 双重收款）。修复
// 后新 quote 的提交必须原样重放该购买既有的 pending 订单。
func TestPurchaseSecondQuoteReplaysExistingPendingOrder(t *testing.T) {
	svc, _, cp, _ := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	q1 := purchaseQuote(t, svc.orders, 44, "pro")
	first, err := svc.Purchase(context.Background(), 44, q1.ID, "wechat", "a", "WeKnora Space 44")
	if err != nil {
		t.Fatal(err)
	}
	if first.Order == nil || first.Order.ID == "" {
		t.Fatalf("first purchase must open an order, got %+v", first)
	}
	// 新 quote（同 plan 同价面——match gate 之下这是唯一能走到第 8 步的形态）。
	q2 := purchaseQuote(t, svc.orders, 44, "pro")
	second, err := svc.Purchase(context.Background(), 44, q2.ID, "wechat", "a", "WeKnora Space 44")
	if err != nil {
		t.Fatalf("second quote must replay, got %v", err)
	}
	if second.Order == nil || second.Order.ID != first.Order.ID {
		t.Fatalf("second quote must replay the SAME pending order, first=%v second=%+v",
			first.Order.ID, second.Order)
	}
	if second.Order.CheckoutURL == "" || second.Order.CheckoutURL != first.Order.CheckoutURL {
		t.Fatalf("replayed order must carry the same checkout entry, first=%q second=%q",
			first.Order.CheckoutURL, second.Order.CheckoutURL)
	}
	if n := len(cp.createCalls); n != 1 {
		t.Fatalf("ONE payable channel order per purchase: channel creates=%d", n)
	}
}

// TestConcurrentFreshQuoteConflictReplaysWinner（R2-26/R3-26）：PAYABLE 谓词
// 三处一致（索引、前置检查、冲突回读）后的端到端不变量语义：同租户已有一
// 张可付 pending（链接已持久化）时，新报价的提交以重放该单回答、渠道零调用
// （确定性路径走前置检查；真并发的前置放行窗口由 repository 层
// TestPartialPendingIndexRejectsSecondPayableOrder 锁定——插入撞索引报哨兵）。
func TestConcurrentFreshQuoteConflictReplaysWinner(t *testing.T) {
	svc, fake, cp, db := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	// 权威面：租户 46 的 awaiting 购买（9900 冻结面）。
	if _, err := fake.SubmitCommand(context.Background(), commercial.Command{
		Kind: commercial.CommandKindCreatePurchaseSubscription,
		Key: commercial.CreatePurchaseSubscriptionCommandKey(
			commercial.ExternalPurchaseSubscriptionID(46), commercial.DeterministicPlanCode("pro", 1)),
		Actor: "test", Reason: "purchase",
		Payload: commercial.CreatePurchaseSubscriptionPayload{
			TenantID: 46, ExternalCustomerID: commercial.ExternalCustomerID(46),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(46),
			PlanCode:                       commercial.DeterministicPlanCode("pro", 1),
			AmountFen:                      9900, Currency: commercial.CurrencyCNY,
		},
	}); err != nil {
		t.Fatal(err)
	}
	// 「并发胜者」：同价面、链接已持久化的可付 pending（占住索引槽；价面与
	// 新 quote 相同——match gate 放行，冲突由索引裁决）。(A-21) 胜者订单的
	// quote 必须真实存在且冻结计划与本次购买一致——重放分支现在校验计划归
	// 属（quoteBoughtPlan），裸 quote_id 会被判为异计划冲突而拒绝重放。
	winnerQuote := purchaseQuote(t, svc.orders, 46, "pro")
	if err := db.Exec(`INSERT INTO commercial_orders (id, tenant_id, quote_id, kind, amount_fen, currency, state, version, created_at, checkout_url)
		VALUES ('ord_winner', 46, ?, 'purchase', 9900, 'CNY', 'pending', 1, ?, 'https://pay.example/winner')`,
		winnerQuote.ID, time.Now().UTC().Format(time.RFC3339Nano)).Error; err != nil {
		t.Fatal(err)
	}
	q := purchaseQuote(t, svc.orders, 46, "pro")
	view, err := svc.Purchase(context.Background(), 46, q.ID, "wechat", "a", "WeKnora Space 46")
	if err != nil {
		t.Fatalf("the payable pending must be replayed, got %v", err)
	}
	if view.Order == nil || view.Order.ID != "ord_winner" {
		t.Fatalf("the loser must replay the winner's payable order, got %+v", view.Order)
	}
	if view.Order.CheckoutURL != "https://pay.example/winner" {
		t.Fatalf("replay must carry the winner's payment entry, got %q", view.Order.CheckoutURL)
	}
	if n := len(cp.createCalls); n != 0 {
		t.Fatalf("a replay must NEVER open a second channel request, creates=%d", n)
	}
}

// TestLinkLessPendingOrderDoesNotBlockFreshQuote（R3-26）：channel_failed=0 但
// 无链接的 pending（SetCheckoutURL 降级残留 / 旧管线存量行 / 空链接应答）
// 不是可付单——不占索引槽、不被重放，新报价照常开新渠道单。
func TestLinkLessPendingOrderDoesNotBlockFreshQuote(t *testing.T) {
	svc, fake, cp, db := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	if _, err := fake.SubmitCommand(context.Background(), commercial.Command{
		Kind: commercial.CommandKindCreatePurchaseSubscription,
		Key: commercial.CreatePurchaseSubscriptionCommandKey(
			commercial.ExternalPurchaseSubscriptionID(49), commercial.DeterministicPlanCode("pro", 1)),
		Actor: "test", Reason: "purchase",
		Payload: commercial.CreatePurchaseSubscriptionPayload{
			TenantID: 49, ExternalCustomerID: commercial.ExternalCustomerID(49),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(49),
			PlanCode:                       commercial.DeterministicPlanCode("pro", 1),
			AmountFen:                      9900, Currency: commercial.CurrencyCNY,
		},
	}); err != nil {
		t.Fatal(err)
	}
	// link-less 残留（channel_failed=0、URL 空——迁移前存量行/降级残留形态）。
	// (OCR r4) The sweep is time-scoped: residue older than SweepStaleAge
	// releases the slot; a FRESH link-less row (its checkout still
	// mid-landing) must NOT be swept — covered by the ordering test below.
	if err := db.Exec(`INSERT INTO commercial_orders (id, tenant_id, quote_id, kind, amount_fen, currency, state, version, created_at, checkout_url, channel_failed)
		VALUES ('ord_zombie', 49, 'qt_zombie', 'purchase', 9900, 'CNY', 'pending', 1, ?, '', 0)`,
		time.Now().UTC().Add(-repocommercial.SweepStaleAge-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	q := purchaseQuote(t, svc.orders, 49, "pro")
	view, err := svc.Purchase(context.Background(), 49, q.ID, "wechat", "a", "WeKnora Space 49")
	if err != nil {
		t.Fatalf("a link-less pending must NOT block the fresh quote: %v", err)
	}
	if view.Order == nil || view.Order.ID == "ord_zombie" || view.Order.CheckoutURL == "" {
		t.Fatalf("the fresh quote must open a NEW payable order, got %+v", view.Order)
	}
	if n := len(cp.createCalls); n != 1 {
		t.Fatalf("channel creates = %d, want 1", n)
	}
}

// TestChannelFailedOrderDoesNotBlockFreshQuote（R2-28）：渠道 Create 失败留下
// 的 pending 死单（无 checkout_url、channel_failed 标记）不是支付入口也不挡
// 新报价——一次瞬时网关故障后，用户用新报价重试必须能拿到新的可付渠道单，
// 而不是被永久卡死在无链接的死单上。
func TestChannelFailedOrderDoesNotBlockFreshQuote(t *testing.T) {
	svc, _, cp, _ := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	q1 := purchaseQuote(t, svc.orders, 47, "pro")
	cp.createErr = errors.New("channel timeout")
	first, err := svc.Purchase(context.Background(), 47, q1.ID, "wechat", "a", "WeKnora Space 47")
	if err != nil {
		t.Fatal(err)
	}
	if first.Order == nil || first.Order.CheckoutError == "" || first.Order.CheckoutURL != "" {
		t.Fatalf("first attempt must be the channel-failure posture, got %+v", first.Order)
	}
	// 瞬时故障恢复：新报价重试。
	cp.createErr = nil
	q2 := purchaseQuote(t, svc.orders, 47, "pro")
	second, err := svc.Purchase(context.Background(), 47, q2.ID, "wechat", "a", "WeKnora Space 47")
	if err != nil {
		t.Fatalf("a fresh quote must not be blocked by the dead order: %v", err)
	}
	if second.Order == nil || second.Order.ID == first.Order.ID || second.Order.CheckoutURL == "" {
		t.Fatalf("fresh quote must open a NEW payable order, first=%v second=%+v", first.Order.ID, second.Order)
	}
	if n := len(cp.createCalls); n != 2 {
		t.Fatalf("channel creates = %d, want 2 (failed first, fresh second)", n)
	}
}

// TestCheckoutLinkDegradedIsSeparateFromChannelFailure（R2-27）：checkout_url
// 持久化失败时渠道调用实际已成功（客户端拿到有效链接）——降级必须是独立
// 标记（CheckoutLinkDegraded），绝不能复用渠道失败的 CheckoutError（那会让
// handler 把一次成功下单误判为渠道失败降 202，并把 SQL 细节透到边界）。
func TestCheckoutLinkDegradedIsSeparateFromChannelFailure(t *testing.T) {
	svc, _, _, db := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	// 让 SetCheckoutURL 的 UPDATE 确定性失败：BEFORE UPDATE OF checkout_url
	// 触发器 ABORT（OpenOrder 的 INSERT 不受影响——渠道调用前订单已落库）。
	if err := db.Exec(`CREATE TRIGGER block_url_persist BEFORE UPDATE OF checkout_url ON commercial_orders
		BEGIN SELECT RAISE(ABORT, 'test block'); END`).Error; err != nil {
		t.Fatal(err)
	}
	q := purchaseQuote(t, svc.orders, 48, "pro")
	view, err := svc.Purchase(context.Background(), 48, q.ID, "wechat", "a", "WeKnora Space 48")
	if err != nil {
		t.Fatalf("the channel call succeeded — the answer must stay a clean success, got %v", err)
	}
	if view.Order == nil || view.Order.CheckoutURL == "" {
		t.Fatalf("the client must still hold the working link, got %+v", view.Order)
	}
	if !view.Order.CheckoutLinkDegraded {
		t.Fatalf("the degradation must be its OWN marker, got %+v", view.Order)
	}
	if view.Order.CheckoutError != "" {
		t.Fatalf("persistence degradation must NOT reuse the channel-failure CheckoutError (202 trigger), got %q",
			view.Order.CheckoutError)
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

// ---- #82 Task 13 (OCR r2): purchase race hardening ----

// seedAwaitingPurchase plants the authority-side awaiting purchase (the
// frozen 9900 CNY face) for a tenant through the real seam command.
func seedAwaitingPurchase(t *testing.T, fake *commercialplatform.FakeAdapter, tenant uint64, planKey string, amountFen int64) {
	t.Helper()
	if _, err := fake.SubmitCommand(context.Background(), commercial.Command{
		Kind: commercial.CommandKindCreatePurchaseSubscription,
		Key: commercial.CreatePurchaseSubscriptionCommandKey(
			commercial.ExternalPurchaseSubscriptionID(tenant), commercial.DeterministicPlanCode(planKey, 1)),
		Actor: "test", Reason: "purchase",
		Payload: commercial.CreatePurchaseSubscriptionPayload{
			TenantID: tenant, ExternalCustomerID: commercial.ExternalCustomerID(tenant),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(tenant),
			PlanCode:                       commercial.DeterministicPlanCode(planKey, 1),
			AmountFen:                      amountFen, Currency: commercial.CurrencyCNY,
		},
	}); err != nil {
		t.Fatal(err)
	}
}

// TestPurchasePaidAwaitingWindowDoesNotOpenSecondOrder（D11 / r2:253 high）：
// 支付成功但权威侧尚未激活的窗口（本地 O1 已 paid、订阅仍 awaiting——正常为
// drain 30s + settle + webhook 的数十秒）内，用户换新 quote 再次提交购买不得
// 开出第二张渠道订单（O3 的回调独立 ConfirmPayment、settle 幂等键绑定自己的
// 渠道流水号——同一订阅双重收款）。必须返回既有 paid 订单并以
// paid_awaiting_activation 合成态回答（与 PurchaseStatus 投影一致）。
func TestPurchasePaidAwaitingWindowDoesNotOpenSecondOrder(t *testing.T) {
	svc, fake, cp, db := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	seedAwaitingPurchase(t, fake, 60, "pro", 9900)
	// O1：同 plan 真实 quote 的 paid 未履约订单（回调已确认、履约未收敛）。
	paidQuote := purchaseQuote(t, svc.orders, 60, "pro")
	if err := db.Exec(`INSERT INTO commercial_orders (id, tenant_id, quote_id, kind, amount_fen, currency, state, version, created_at, checkout_url)
		VALUES ('ord_paid_win', 60, ?, 'purchase', 9900, 'CNY', 'paid', 1, '2026-07-01T00:00:00Z', 'https://pay.example/o1')`,
		paidQuote.ID).Error; err != nil {
		t.Fatal(err)
	}
	// 窗口内的新 quote 提交。
	q2 := purchaseQuote(t, svc.orders, 60, "pro")
	view, err := svc.Purchase(context.Background(), 60, q2.ID, "wechat", "a", "WeKnora Space 60")
	if err != nil {
		t.Fatalf("the paid-awaiting window must replay the paid order, got %v", err)
	}
	if view.Order == nil || view.Order.ID != "ord_paid_win" {
		t.Fatalf("the paid order must be replayed (no second channel order), got %+v", view.Order)
	}
	if view.State != commercial.PurchaseStatePaidAwaitingActivation {
		t.Fatalf("the replay must synthesize paid_awaiting_activation, got %q", view.State)
	}
	if n := len(cp.createCalls); n != 0 {
		t.Fatalf("a paid order in the activation window must NEVER open a second channel order, creates=%d", n)
	}
}

// TestPurchasePaidAwaitingWindowDifferentPlanConflicts（D11）：paid 探测命中价
// 面但归属另一 plan（republish 窗口同价异 plan 的历史单）时，与 pending 探测同
// 防线——ErrPurchasePlanConflict，绝不把异 plan 的支付入口重放给本次购买。
func TestPurchasePaidAwaitingWindowDifferentPlanConflicts(t *testing.T) {
	svc, fake, cp, db := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	purchaseSeedPlan(t, svc.plans, "pro-max", 29900) // 另一 plan（订单行价面手插成同价的 republish 形态）
	seedAwaitingPurchase(t, fake, 61, "pro", 9900)
	// 历史 paid 单：quote 冻结的是 max，订单行价面手插 9900（republish 窗口形
	// 态——价面匹配但 quote 归属另一 plan）。
	foreignQuote := purchaseQuote(t, svc.orders, 61, "pro-max")
	if err := db.Exec(`INSERT INTO commercial_orders (id, tenant_id, quote_id, kind, amount_fen, currency, state, version, created_at, checkout_url)
		VALUES ('ord_paid_foreign', 61, ?, 'purchase', 9900, 'CNY', 'paid', 1, '2026-07-01T00:00:00Z', 'https://pay.example/f')`,
		foreignQuote.ID).Error; err != nil {
		t.Fatal(err)
	}
	q := purchaseQuote(t, svc.orders, 61, "pro")
	_, err := svc.Purchase(context.Background(), 61, q.ID, "wechat", "a", "WeKnora Space 61")
	if !errors.Is(err, ErrPurchasePlanConflict) {
		t.Fatalf("a paid order of a DIFFERENT plan must conflict, got %v", err)
	}
	if n := len(cp.createCalls); n != 0 {
		t.Fatalf("no channel order may open on the conflict, creates=%d", n)
	}
}

// TestPurchaseConcurrentPendingExistsAnswers409WithReplay（D12 / r2:306）：
// 并发 checkout 败者在 winner 渠道 Create 窗口内（link-less pending 占住索引、
// 不到 sweep 年龄、冲突回读也不可付）时，服务层把 ErrPurchasePendingExists
// 哨兵交还调用方（handler 409），绝不再开第二渠道单。
func TestPurchaseConcurrentPendingExistsAnswers409WithReplay(t *testing.T) {
	svc, fake, cp, db := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	seedAwaitingPurchase(t, fake, 62, "pro", 9900)
	// winner 形态：OpenOrder 已提交（pending、未 channel_failed、URL 仍空——
	// 正在外呼渠道 Create），占住 pending 索引且不到 SweepStaleAge。
	if err := db.Exec(`INSERT INTO commercial_orders (id, tenant_id, quote_id, kind, amount_fen, currency, state, version, created_at, checkout_url, channel_failed)
		VALUES ('ord_winner_creating', 62, 'qt_winner', 'purchase', 9900, 'CNY', 'pending', 1, ?, '', 0)`,
		time.Now().UTC().Format(time.RFC3339Nano)).Error; err != nil {
		t.Fatal(err)
	}
	q := purchaseQuote(t, svc.orders, 62, "pro")
	_, err := svc.Purchase(context.Background(), 62, q.ID, "wechat", "a", "WeKnora Space 62")
	if !errors.Is(err, repocommercial.ErrPurchasePendingExists) {
		t.Fatalf("an unreplayable pending-exists race must surface the conflict sentinel (handler 409), got %v", err)
	}
	if n := len(cp.createCalls); n != 0 {
		t.Fatalf("the loser must NEVER open a second channel order, creates=%d", n)
	}
}

// ---- #82 Task 11: final-audit OCR open findings 11/9/10 (service layer) ----

// Finding 11: the disambiguation probe must cover the ACTIVE conflict too —
// an active held purchase on a DIFFERENT plan is a deterministic conflict
// (409), not an unavailable 503.
func TestPurchaseDisambiguationCoversActiveConflict(t *testing.T) {
	svc, fake, _, _ := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	// An ACTIVE purchase held on a different plan than the one being bought.
	other := commercial.DeterministicPlanCode("pro", 1) + "-x"
	if _, err := fake.SubmitCommand(context.Background(), commercial.Command{
		Kind: commercial.CommandKindCreatePurchaseSubscription,
		Key:  commercial.CreatePurchaseSubscriptionCommandKey(commercial.ExternalPurchaseSubscriptionID(81), other),
		Payload: commercial.CreatePurchaseSubscriptionPayload{
			TenantID: 81, ExternalCustomerID: commercial.ExternalCustomerID(81),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(81),
			PlanCode:                       other, AmountFen: 9900, Currency: commercial.CurrencyCNY,
		},
	}); err != nil {
		t.Fatal(err)
	}
	fake.ActivatePurchase(commercial.ExternalPurchaseSubscriptionID(81))
	// A held plan code that never matches makes the create submit fail with
	// the plan-conflict sentinel the probe re-reads.
	q := purchaseQuote(t, svc.orders, 81, "pro")
	_, err := svc.Purchase(context.Background(), 81, q.ID, "wechat", "a", "Space 81")
	if !errors.Is(err, ErrPurchasePlanConflict) {
		t.Fatalf("active foreign-plan conflict must answer ErrPurchasePlanConflict, got %v", err)
	}
}

// Finding 9: a historical same-price order on a DIFFERENT plan is never
// projected as the current purchase.
func TestPurchaseStatusSkipsForeignPlanOrder(t *testing.T) {
	svc, fake, _, db := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	purchaseSeedPlan(t, svc.plans, "pro-max", 29900)
	if _, err := fake.SubmitCommand(context.Background(), commercial.Command{
		Kind: commercial.CommandKindCreatePurchaseSubscription,
		Key:  commercial.CreatePurchaseSubscriptionCommandKey(commercial.ExternalPurchaseSubscriptionID(82), commercial.DeterministicPlanCode("pro", 1)),
		Payload: commercial.CreatePurchaseSubscriptionPayload{
			TenantID: 82, ExternalCustomerID: commercial.ExternalCustomerID(82),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(82),
			PlanCode:                       commercial.DeterministicPlanCode("pro", 1),
			AmountFen:                      9900, Currency: commercial.CurrencyCNY,
		},
	}); err != nil {
		t.Fatal(err)
	}
	// A historical PAID order at the same price face but on plan "pro-max".
	if err := db.Create(&repocommercial.QuoteRow{
		ID: "q-foreign", TenantID: 82, SubscriptionVersion: 1,
		SnapshotJSON: `{"plan_key":"pro-max","plan_version":1,"price_fen":9900,"credits_micro":9900000,"currency":"CNY","line_items":[{"kind":"subscription_fee","name":"pro-max","amount_fen":9900}]}`,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO commercial_orders (id, tenant_id, quote_id, kind, amount_fen, currency, state, version, created_at, channel_failed)
		VALUES ('ord_foreign', 82, 'q-foreign', 'purchase', 9900, 'CNY', 'paid', 1, ?, 0)`,
		time.Now().UTC().Add(-time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	view, err := svc.PurchaseStatus(context.Background(), 82)
	if err != nil {
		t.Fatal(err)
	}
	if view.State != commercial.PurchaseStateAwaitingPayment {
		t.Fatalf("the foreign-plan paid order must not compose paid_awaiting_activation, got %q", view.State)
	}
	if view.Order != nil && view.Order.ID == "ord_foreign" {
		t.Fatalf("the foreign-plan order must not be projected as the current purchase")
	}
}

// Finding 10 (minimal face): a mismatched purchase attempt leaves an
// attention audit trail (the closed-token audit sink fires).
func TestPurchaseMismatchWritesAttentionAudit(t *testing.T) {
	audits := make([][2]string, 0, 2)
	oldAudit := purchaseAudit
	purchaseAudit = func(tenantID uint64, quoteID, token string) {
		audits = append(audits, [2]string{quoteID, token})
	}
	t.Cleanup(func() { purchaseAudit = oldAudit })
	svc, fake, _, _ := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	// The authority holds a DIFFERENT price face than the quote: the match
	// gate (AC2) refuses with ErrInvoiceQuoteMismatch — the audit must fire.
	if _, err := fake.SubmitCommand(context.Background(), commercial.Command{
		Kind: commercial.CommandKindCreatePurchaseSubscription,
		Key:  commercial.CreatePurchaseSubscriptionCommandKey(commercial.ExternalPurchaseSubscriptionID(83), commercial.DeterministicPlanCode("pro", 1)),
		Payload: commercial.CreatePurchaseSubscriptionPayload{
			TenantID: 83, ExternalCustomerID: commercial.ExternalCustomerID(83),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(83),
			PlanCode:                       commercial.DeterministicPlanCode("pro", 1),
			AmountFen:                      12345, Currency: commercial.CurrencyCNY,
		},
	}); err != nil {
		t.Fatal(err)
	}
	q := purchaseQuote(t, svc.orders, 83, "pro")
	_, err := svc.Purchase(context.Background(), 83, q.ID, "wechat", "a", "Space 83")
	if !errors.Is(err, ErrInvoiceQuoteMismatch) && !errors.Is(err, ErrPurchaseNotAwaiting) {
		t.Fatalf("the mismatch face must answer a closed sentinel, got %v", err)
	}
	if len(audits) < 1 {
		t.Fatalf("an attention audit must fire for the mismatched purchase (found %d)", len(audits))
	}
	if audits[0][1] != "invoice_quote_mismatch" && audits[0][1] != "purchase_not_awaiting" {
		t.Fatalf("the audit must carry the closed token, got %q", audits[0][1])
	}
}

// TestPurchaseReplayRejectsForeignPlanPendingOrder（A-21 / F90）：价面匹配
// 不证明归属——重放分支必须校验既有 pending 订单自己的 quote 买的计划与本次
// 购买一致。旧 POST /commercial/orders 面开出的同价异计划 pending 单（如
// republish 窗口的 pro-max@pro 价面）不得被当作本次 pro 购买的支付入口重放
// ——其 CheckoutURL 结算的会是旧计划的 quote。
func TestPurchaseReplayRejectsForeignPlanPendingOrder(t *testing.T) {
	svc, fake, cp, db := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	// 权威面：租户 52 awaiting 的 pro 购买（9900 冻结面）。
	if _, err := fake.SubmitCommand(context.Background(), commercial.Command{
		Kind: commercial.CommandKindCreatePurchaseSubscription,
		Key: commercial.CreatePurchaseSubscriptionCommandKey(
			commercial.ExternalPurchaseSubscriptionID(52), commercial.DeterministicPlanCode("pro", 1)),
		Actor: "test", Reason: "purchase",
		Payload: commercial.CreatePurchaseSubscriptionPayload{
			TenantID: 52, ExternalCustomerID: commercial.ExternalCustomerID(52),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(52),
			PlanCode:                       commercial.DeterministicPlanCode("pro", 1),
			AmountFen:                      9900, Currency: commercial.CurrencyCNY,
		},
	}); err != nil {
		t.Fatal(err)
	}
	// 一张「旧订单面」遗留的同价异计划 pending 单（OCR final audit 9 的
	// republish 窗口形状：pro-max 曾按 pro 的价面 9900 售出）。其 quote 冻
	// 结的是 pro-max——手工造行，绕过发布阶梯（该订单面本就先于 Purchase
	// 流程存在）。
	if err := db.Create(&repocommercial.QuoteRow{
		ID: "qt_foreign", TenantID: 52, SubscriptionVersion: 1,
		SnapshotJSON: mustJSON(t, map[string]any{
			"plan_key": "pro-max", "plan_version": int64(1), "price_fen": int64(9900),
			"credits_micro": int64(9_900_000), "currency": commercial.CurrencyCNY,
			"line_items": []map[string]any{{"kind": "subscription_fee", "name": "pro-max", "amount_fen": 9900}},
		}),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO commercial_orders (id, tenant_id, quote_id, kind, amount_fen, currency, state, version, created_at, checkout_url)
		VALUES ('ord_foreign', 52, 'qt_foreign', 'purchase', 9900, 'CNY', 'pending', 1, ?, 'https://pay.example/foreign')`,
		time.Now().UTC().Format(time.RFC3339Nano)).Error; err != nil {
		t.Fatal(err)
	}
	q := purchaseQuote(t, svc.orders, 52, "pro")
	_, err := svc.Purchase(context.Background(), 52, q.ID, "wechat", "a", "WeKnora Space 52")
	if !errors.Is(err, ErrPurchasePlanConflict) {
		t.Fatalf("a same-price pending order whose quote bought ANOTHER plan must surface the plan conflict, never replay its checkout entry, got %v", err)
	}
	if n := len(cp.createCalls); n != 0 {
		t.Fatalf("the foreign order's payment entry must never be replayed as this purchase's answer, channel creates=%d", n)
	}
}

// TestPurchaseStatusSkipsProjectionWhenPublicationUnreadable（A-28 / F104）：
// 计划一致性校验的两侧都不可证明（publication 读失败 + quoteBoughtPlan 默认
// ""）时绝不投影订单——旧行为的 "" == "" 空串匹配会把异计划历史订单投影成
// 当前购买（甚至合成 paid_awaiting_activation）。publication 持续缺失时投影
// 永久跳过（可从日志排查），而不是静默降级为假匹配。
func TestPurchaseStatusSkipsProjectionWhenPublicationUnreadable(t *testing.T) {
	svc, fake, _, db := newPurchaseTestEnv(t)
	purchaseSeedPlan(t, svc.plans, "pro", 9900)
	const tenant = uint64(53)
	// 权威 awaiting + 一张本地已支付订单（价面一致、quote 冻结 pro）。
	if _, err := fake.SubmitCommand(context.Background(), commercial.Command{
		Kind: commercial.CommandKindCreatePurchaseSubscription,
		Key: commercial.CreatePurchaseSubscriptionCommandKey(
			commercial.ExternalPurchaseSubscriptionID(tenant), commercial.DeterministicPlanCode("pro", 1)),
		Actor: "test", Reason: "purchase",
		Payload: commercial.CreatePurchaseSubscriptionPayload{
			TenantID: tenant, ExternalCustomerID: commercial.ExternalCustomerID(tenant),
			ExternalPurchaseSubscriptionID: commercial.ExternalPurchaseSubscriptionID(tenant),
			PlanCode:                       commercial.DeterministicPlanCode("pro", 1),
			AmountFen:                      9900, Currency: commercial.CurrencyCNY,
		},
	}); err != nil {
		t.Fatal(err)
	}
	q := purchaseQuote(t, svc.orders, tenant, "pro")
	if _, err := svc.Purchase(context.Background(), tenant, q.ID, "wechat", "a", "WeKnora Space 53"); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE commercial_orders SET state = 'paid' WHERE tenant_id = ?`, tenant).Error; err != nil {
		t.Fatal(err)
	}
	// 基线：publication 可读时 awaiting+paid 合成 paid_awaiting_activation。
	base, err := svc.PurchaseStatus(context.Background(), tenant)
	if err != nil || base.Order == nil || base.State != commercial.PurchaseStatePaidAwaitingActivation {
		t.Fatalf("baseline projection must synthesize paid_awaiting_activation, got %+v err=%v", base, err)
	}
	// publication 行消失（迁移缺口/DB 故障的持久形状）：订单投影整体跳过。
	if err := db.Exec(`DELETE FROM commercial_plan_publications WHERE plan_code = ?`,
		commercial.DeterministicPlanCode("pro", 1)).Error; err != nil {
		t.Fatal(err)
	}
	degraded, err := svc.PurchaseStatus(context.Background(), tenant)
	if err != nil {
		t.Fatalf("a failed publication read must degrade the projection, not fail the endpoint: %v", err)
	}
	if degraded.Order != nil {
		t.Fatalf("with the plan face unprovable the order must NOT project (the vacuous \"\"==\"\" match), got %+v", degraded.Order)
	}
	if degraded.PlanKey != "" || degraded.PlanVersion != 0 {
		t.Fatalf("the plan face must stay empty when the publication read fails, got %+v", degraded)
	}
	if degraded.State != commercial.PurchaseStateAwaitingPayment {
		t.Fatalf("the authority state still answers honestly, got %q", degraded.State)
	}
}
