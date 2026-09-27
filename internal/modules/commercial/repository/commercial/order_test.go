package commercial

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"time"
)

// testOrderStore mirrors the account_test.go SQLite setup. A single pooled
// connection serializes concurrent goroutines at the driver level: SQLite is
// single-writer anyway, and this keeps the confirmation transaction — a
// read-then-write flow — free of shared-cache table-lock deadlocks while the
// guarded UPDATE below still decides the single winner.
func testOrderStore(t *testing.T) (*OrderStore, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&OrderRow{}, &PaymentAttemptRow{}, &OutboxEvent{}); err != nil {
		t.Fatal(err)
	}
	for _, idx := range []string{
		"CREATE UNIQUE INDEX IF NOT EXISTS uq_attempt_merchant_order ON commercial_payment_attempts(provider, merchant, merchant_order_id)",
		"CREATE UNIQUE INDEX IF NOT EXISTS uq_attempt_provider_txn ON commercial_payment_attempts(provider, merchant, provider_transaction_id)",
	} {
		if err := db.Exec(idx).Error; err != nil {
			t.Fatal(err)
		}
	}
	return NewOrderStore(db), db
}

func mustCreateOrder(t *testing.T, s *OrderStore, id, quote string) {
	t.Helper()
	if err := s.CreateOrder(context.Background(), OrderRow{ID: id, TenantID: 7, QuoteID: quote, AmountFen: 100, Currency: "CNY"}); err != nil {
		t.Fatal(err)
	}
}

// mustRegisterAttempt registers a pending attempt and returns the succeeded
// payment fact the provider would later report for it.
func mustRegisterAttempt(t *testing.T, s *OrderStore, id, order, provider, merchant, merchantOrder string) domain.PaymentFact {
	t.Helper()
	if err := s.RegisterAttempt(context.Background(), PaymentAttemptRow{
		ID: id, TenantID: 7, OrderID: order, Provider: provider, Merchant: merchant,
		MerchantOrderID: merchantOrder, AmountFen: 100, Currency: "CNY",
	}); err != nil {
		t.Fatal(err)
	}
	return domain.PaymentFact{
		TenantID: 7, OrderID: order, AttemptID: merchantOrder, Provider: provider,
		Merchant: merchant, Transaction: "txn_" + merchantOrder, Amount: 100,
		Currency: "CNY", State: "succeeded",
	}
}

func countOutbox(t *testing.T, db *gorm.DB, kind string) int64 {
	t.Helper()
	q := db.Model(&OutboxEvent{})
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func getAttempt(t *testing.T, db *gorm.DB, id string) PaymentAttemptRow {
	t.Helper()
	var row PaymentAttemptRow
	if err := db.Where("id = ?", id).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row
}

func TestOrderConfirmPaymentPersistsFactMarksPaidAndEnqueuesFulfill(t *testing.T) {
	s, db := testOrderStore(t)
	ctx := context.Background()
	mustCreateOrder(t, s, "o1", "q1")
	fact := mustRegisterAttempt(t, s, "a1", "o1", "wechat", "wxm", "m1")

	if err := s.ConfirmPayment(ctx, fact); err != nil {
		t.Fatal(err)
	}
	order, err := s.GetOrder(ctx, "o1")
	if err != nil {
		t.Fatal(err)
	}
	if order.State != domain.OrderStatePaid {
		t.Fatalf("order state=%s, want paid", order.State)
	}
	if order.State == domain.OrderStateFulfilled {
		t.Fatal("confirmed order must stay paid, not fulfilled, until delivery")
	}
	if order.Version != 2 {
		t.Fatalf("version=%d, want 2", order.Version)
	}
	attempt := getAttempt(t, db, "a1")
	if attempt.State != PaymentAttemptStateSucceeded {
		t.Fatalf("attempt state=%s, want succeeded", attempt.State)
	}
	if attempt.ProviderTransactionID == nil || *attempt.ProviderTransactionID != fact.Transaction {
		t.Fatalf("provider transaction not persisted: %+v", attempt.ProviderTransactionID)
	}
	if got := countOutbox(t, db, OutboxKindFulfill); got != 1 {
		t.Fatalf("fulfill events=%d, want 1", got)
	}
	if got := countOutbox(t, db, OutboxKindOverPaid); got != 0 {
		t.Fatalf("over-payment events=%d, want 0", got)
	}
	var event OutboxEvent
	if err := db.Where("kind = ?", OutboxKindFulfill).First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.EventKey != OutboxKindFulfill+":o1" || event.State != OutboxStatePending {
		t.Fatalf("unexpected event %+v", event)
	}
	if !strings.Contains(event.PayloadJSON, fact.Transaction) || !strings.Contains(event.PayloadJSON, "q1") {
		t.Fatalf("payload missing transaction/quote: %s", event.PayloadJSON)
	}
}

func TestPaymentConfirmFailsBetweenConfirmAndOutboxRollsBackAll(t *testing.T) {
	s, db := testOrderStore(t)
	ctx := context.Background()
	mustCreateOrder(t, s, "o1", "q1")
	fact := mustRegisterAttempt(t, s, "a1", "o1", "wechat", "wxm", "m1")

	// Inject a failure exactly at the outbox insert: the order has already
	// been marked paid and the fact saved inside the same transaction.
	db.Callback().Create().Before("gorm:create").Register("test/fail_outbox", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "commercial_outbox_events" {
			_ = tx.AddError(errors.New("injected outbox failure"))
		}
	})
	defer db.Callback().Create().Remove("test/fail_outbox")

	err := s.ConfirmPayment(ctx, fact)
	if err == nil || !strings.Contains(err.Error(), "injected outbox failure") {
		t.Fatalf("expected injected outbox failure, got %v", err)
	}
	order, err := s.GetOrder(ctx, "o1")
	if err != nil {
		t.Fatal(err)
	}
	if order.State != domain.OrderStatePending || order.Version != 1 {
		t.Fatalf("rollback failed: state=%s version=%d, want pending/1", order.State, order.Version)
	}
	attempt := getAttempt(t, db, "a1")
	if attempt.State != PaymentAttemptStatePending || attempt.ProviderTransactionID != nil {
		t.Fatalf("payment fact survived rollback: %+v", attempt)
	}
	if got := countOutbox(t, db, ""); got != 0 {
		t.Fatalf("outbox events=%d after rollback, want 0", got)
	}
}

func TestPaymentConfirmReplayAfterResponseLossKeepsSingleEvent(t *testing.T) {
	s, db := testOrderStore(t)
	ctx := context.Background()
	mustCreateOrder(t, s, "o1", "q1")
	fact := mustRegisterAttempt(t, s, "a1", "o1", "wechat", "wxm", "m1")

	if err := s.ConfirmPayment(ctx, fact); err != nil {
		t.Fatal(err)
	}
	// The provider response was lost; the same notification is retried.
	if err := s.ConfirmPayment(ctx, fact); err != nil {
		t.Fatal(err)
	}
	order, err := s.GetOrder(ctx, "o1")
	if err != nil {
		t.Fatal(err)
	}
	if order.State != domain.OrderStatePaid || order.Version != 2 {
		t.Fatalf("replay changed order: state=%s version=%d", order.State, order.Version)
	}
	if got := countOutbox(t, db, OutboxKindFulfill); got != 1 {
		t.Fatalf("fulfill events=%d after replay, want 1", got)
	}
	if got := countOutbox(t, db, OutboxKindOverPaid); got != 0 {
		t.Fatalf("replay produced %d audit events, want 0", got)
	}
}

func TestPaymentConcurrentChannelsYieldSingleFulfillmentPlusAudit(t *testing.T) {
	s, db := testOrderStore(t)
	mustCreateOrder(t, s, "o1", "q1")
	facts := []domain.PaymentFact{
		mustRegisterAttempt(t, s, "a1", "o1", "wechat", "wxm", "m1"),
		mustRegisterAttempt(t, s, "a2", "o1", "alipay", "alim", "m2"),
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(facts))
	for _, f := range facts {
		wg.Add(1)
		go func(f domain.PaymentFact) {
			defer wg.Done()
			errs <- s.ConfirmPayment(context.Background(), f)
		}(f)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := countOutbox(t, db, OutboxKindFulfill); got != 1 {
		t.Fatalf("fulfill events=%d, want exactly 1", got)
	}
	if got := countOutbox(t, db, OutboxKindOverPaid); got != 1 {
		t.Fatalf("over-payment audits=%d, want exactly 1", got)
	}
	var audit OutboxEvent
	if err := db.Where("kind = ?", OutboxKindOverPaid).First(&audit).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(audit.PayloadJSON, "over_paid") {
		t.Fatalf("audit payload missing reason: %s", audit.PayloadJSON)
	}
	order, err := s.GetOrder(context.Background(), "o1")
	if err != nil {
		t.Fatal(err)
	}
	if order.State != domain.OrderStatePaid || order.Version != 2 {
		t.Fatalf("order state=%s version=%d, want paid/2", order.State, order.Version)
	}
	// Both attempts recorded their facts; the txn unique constraint held.
	for _, id := range []string{"a1", "a2"} {
		attempt := getAttempt(t, db, id)
		if attempt.State != PaymentAttemptStateSucceeded || attempt.ProviderTransactionID == nil {
			t.Fatalf("attempt %s fact missing: %+v", id, attempt)
		}
	}
}

func TestOrderQuoteConsumableByExactlyOneOrder(t *testing.T) {
	s, _ := testOrderStore(t)
	ctx := context.Background()
	mustCreateOrder(t, s, "o1", "q1")
	err := s.CreateOrder(ctx, OrderRow{ID: "o2", TenantID: 7, QuoteID: "q1", AmountFen: 100, Currency: "CNY"})
	if !errors.Is(err, ErrQuoteAlreadyUsed) {
		t.Fatalf("second order on same quote: %v, want ErrQuoteAlreadyUsed", err)
	}
	if err := s.CreateOrder(ctx, OrderRow{ID: "o3", TenantID: 7, QuoteID: "q2", AmountFen: 100, Currency: "CNY"}); err != nil {
		t.Fatal(err)
	}
}

// insertCreatedAtOrder 以参数绑定落一张指定 created_at 的购买单（R1-20 测试
// 的可控时间锚；order ids 故意用字典序与时间序相反的构造）。payable=true 时
// 带非空 checkout_url 且非 channel_failed（R2-28 的可付形态）。
func insertCreatedAtOrder(t *testing.T, db *gorm.DB, id, state, createdAt string, payable bool) {
	t.Helper()
	url := ""
	if payable {
		url = "https://pay.example/" + id
	}
	if err := db.Exec(`INSERT INTO commercial_orders (id, tenant_id, quote_id, kind, amount_fen, currency, state, version, created_at, checkout_url)
		VALUES (?, 7, ?, 'purchase', 100, 'CNY', ?, 1, ?, ?)`, id, "qt_"+id, state, createdAt, url).Error; err != nil {
		t.Fatal(err)
	}
}

// TestCurrentPurchaseOrderPrefersNewestByCreatedAt（R1-20）：order id 是
// "ord_"+随机 hex，id 序不是时间序——同价面历史购买单全部命中价格面匹配时，
// 唯一确定的"当前购买"锚点是 created_at（新 pending 优先、双 pending 取最新、
// 全终态取最新终态，而非 id ASC 的随机 tie-break / 最旧一单）。
func TestCurrentPurchaseOrderPrefersNewestByCreatedAt(t *testing.T) {
	s, db := testOrderStore(t)
	ctx := context.Background()
	// 字典序最 OLD 的行时间最新——id ASC 会选错。
	insertCreatedAtOrder(t, db, "ord_0000newest", "pending", "2026-06-01T00:00:00Z", true)
	insertCreatedAtOrder(t, db, "ord_0001mid", "paid", "2026-03-01T00:00:00Z", true)
	insertCreatedAtOrder(t, db, "ord_0002oldest", "paid", "2026-01-01T00:00:00Z", true)
	row, err := s.CurrentPurchaseOrder(ctx, 7, 100, "CNY")
	if err != nil || row.ID != "ord_0000newest" {
		t.Fatalf("newest pending must win, got %+v err=%v", row, err)
	}
	// 双 pending：created_at 最新者胜（id 序随机）。
	insertCreatedAtOrder(t, db, "ord_0003newerpending", "pending", "2026-07-01T00:00:00Z", true)
	row, err = s.CurrentPurchaseOrder(ctx, 7, 100, "CNY")
	if err != nil || row.ID != "ord_0003newerpending" {
		t.Fatalf("newest of two pendings must win, got %+v err=%v", row, err)
	}
	// 全终态：取最新终态（id ASC 的 rows[0] 会投影最旧一单的陈旧 quote_id）。
	if err := db.Exec(`UPDATE commercial_orders SET state = 'paid' WHERE state = ?`,
		domain.OrderStatePending).Error; err != nil {
		t.Fatal(err)
	}
	row, err = s.CurrentPurchaseOrder(ctx, 7, 100, "CNY")
	if err != nil || row.ID != "ord_0003newerpending" {
		t.Fatalf("all-terminal must project the NEWEST row, got %+v err=%v", row, err)
	}
	// 无匹配行：ErrOrderNotFound。
	if _, err := s.CurrentPurchaseOrder(ctx, 8, 100, "CNY"); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("no matching order must be ErrOrderNotFound, got %v", err)
	}
}

// TestCurrentPendingPurchaseOrderReturnsNewestPayablePending（R1-22/R2-28）：
// 重放面只认「可付」pending——渠道创建成功（checkout_url 非空）且未被标记
// 渠道失败；channel_failed 死单与无链接 pending 不作为支付入口重放，无 match
// 报 ErrOrderNotFound。
func TestCurrentPendingPurchaseOrderReturnsNewestPayablePending(t *testing.T) {
	s, db := testOrderStore(t)
	ctx := context.Background()
	insertCreatedAtOrder(t, db, "ord_0000oldpending", "pending", "2026-01-01T00:00:00Z", true)
	insertCreatedAtOrder(t, db, "ord_0001paid", "paid", "2026-06-01T00:00:00Z", true)
	insertCreatedAtOrder(t, db, "ord_0002newpending", "pending", "2026-07-01T00:00:00Z", true)
	row, err := s.CurrentPendingPurchaseOrder(ctx, 7, 100, "CNY")
	if err != nil || row.ID != "ord_0002newpending" {
		t.Fatalf("newest payable pending must win over paid and older pending, got %+v err=%v", row, err)
	}
	// channel-failed 死单：仍是 pending 但不可付——不入选。
	if err := db.Exec(`UPDATE commercial_orders SET channel_failed = 1 WHERE id = ?`,
		"ord_0002newpending").Error; err != nil {
		t.Fatal(err)
	}
	row, err = s.CurrentPendingPurchaseOrder(ctx, 7, 100, "CNY")
	if err != nil || row.ID != "ord_0000oldpending" {
		t.Fatalf("channel-failed pending must be skipped, got %+v err=%v", row, err)
	}
	// 无链接 pending（持久化降级形态）：同样不是支付入口。
	if err := db.Exec(`UPDATE commercial_orders SET checkout_url = ''`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.CurrentPendingPurchaseOrder(ctx, 7, 100, "CNY"); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("no PAYABLE pending order must be ErrOrderNotFound, got %v", err)
	}
	// CurrentPayablePendingOrder（R2-26 冲突回读）：不限价面，可付 pending
	// 最新优先——含无链接但渠道成功的降级单（索引仍在保护它）。
	insertCreatedAtOrder(t, db, "ord_0004degraded", "pending", "2026-08-01T00:00:00Z", false)
	if err := db.Exec(`UPDATE commercial_orders SET checkout_url = 'https://pay.example/kept' WHERE id = ?`,
		"ord_0004degraded").Error; err != nil {
		t.Fatal(err)
	}
	pr, err := s.CurrentPayablePendingOrder(ctx, 7)
	if err != nil || pr.ID != "ord_0004degraded" {
		t.Fatalf("conflict replay read must return the newest payable pending, got %+v err=%v", pr, err)
	}
}

// TestPartialPendingIndexRejectsSecondPayableOrder（R2-26/R3-26/R3-27）：
// 数据库层不变量——同租户至多一张 channel_failed=false 的 pending 购买单
// （boolean 字面量而非 0/1——PG 无 boolean=integer 隐式转换）。两张不同
// quote 的并发形态（前置 SELECT 均未命中彼时对方的未提交单）由部分唯一
// 索引在 INSERT 时兜住：第二张撞索引报 ErrPurchasePendingExists（与
// ErrQuoteAlreadyUsed 同构的回放触发）。channel_failed 死单让槽；无链接
// 残留占槽（由 SweepStaleLinklessPending 解锁——见下个测试）。
func TestPartialPendingIndexRejectsSecondPayableOrder(t *testing.T) {
	s, db := testOrderStore(t)
	ctx := context.Background()
	// (testOrderStore 的 AutoMigrate 不含索引——服务装配点建。DDL 与生产
	// NewOrderService 逐字一致。)
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_purchase_pending_per_tenant
		ON commercial_orders (tenant_id)
		WHERE kind = 'purchase' AND state = 'pending' AND channel_failed = false`).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.CreateOrder(ctx, OrderRow{ID: "o1", TenantID: 7, QuoteID: "q1",
		AmountFen: 100, Currency: "CNY", CheckoutURL: "https://pay.example/o1"}); err != nil {
		t.Fatal(err)
	}
	err := s.CreateOrder(ctx, OrderRow{ID: "o2", TenantID: 7, QuoteID: "q2",
		AmountFen: 100, Currency: "CNY", CheckoutURL: "https://pay.example/o2"})
	if !errors.Is(err, ErrPurchasePendingExists) {
		t.Fatalf("second concurrent payable pending must hit the partial index at INSERT time, got %v", err)
	}
	// quote_id 唯一索引的冲突形态不误判为 pending 冲突（同 quote 第二张）。
	err = s.CreateOrder(ctx, OrderRow{ID: "o3", TenantID: 8, QuoteID: "q1",
		AmountFen: 100, Currency: "CNY"})
	if errors.Is(err, ErrPurchasePendingExists) {
		t.Fatalf("quote_id conflict must NOT map to the pending sentinel, got %v", err)
	}
	if err == nil {
		t.Fatal("quote reuse must still fail")
	}
	// channel_failed 死单让出 pending 槽：新单可插入。
	if err := db.Exec(`UPDATE commercial_orders SET channel_failed = true WHERE id = 'o1'`).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.CreateOrder(ctx, OrderRow{ID: "o4", TenantID: 7, QuoteID: "q4",
		AmountFen: 100, Currency: "CNY", CheckoutURL: "https://pay.example/o4"}); err != nil {
		t.Fatalf("a channel-failed dead order must not block a fresh payable order: %v", err)
	}
	// 终态单同样不占槽。
	if err := db.Exec(`UPDATE commercial_orders SET state = 'paid' WHERE id = 'o4'`).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.CreateOrder(ctx, OrderRow{ID: "o5", TenantID: 7, QuoteID: "q5",
		AmountFen: 100, Currency: "CNY"}); err != nil {
		t.Fatalf("a paid order must not block a fresh payable order: %v", err)
	}
}

// TestSweepStaleLinklessPendingReleasesTheSlot（R3-26）：channel_failed=false
// 且无链接的 pending（降级残留——渠道调用成功但链接未持久化）占着索引槽：
// 清扫把它标记 channel_failed 后槽位释放，新单可插入。
func TestSweepStaleLinklessPendingReleasesTheSlot(t *testing.T) {
	s, db := testOrderStore(t)
	ctx := context.Background()
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_purchase_pending_per_tenant
		ON commercial_orders (tenant_id)
		WHERE kind = 'purchase' AND state = 'pending' AND channel_failed = false`).Error; err != nil {
		t.Fatal(err)
	}
	// 降级残留：pending、channel_failed=false、无链接。
	if err := s.CreateOrder(ctx, OrderRow{ID: "z1", TenantID: 7, QuoteID: "zq1",
		AmountFen: 100, Currency: "CNY"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateOrder(ctx, OrderRow{ID: "z2", TenantID: 7, QuoteID: "zq2",
		AmountFen: 100, Currency: "CNY"}); !errors.Is(err, ErrPurchasePendingExists) {
		t.Fatalf("the link-less residue occupies the slot, got %v", err)
	}
	// (OCR r4) The sweep is time-scoped: a FRESH link-less row (its checkout
	// may still be mid-landing) is NOT swept — prove that first, then age
	// the residue past the sweep window and sweep again.
	if swept, err := s.SweepStaleLinklessPending(ctx, 7); err != nil || swept != 0 {
		t.Fatalf("a fresh link-less row must NOT be swept, swept=%d err=%v", swept, err)
	}
	if err := db.Exec(`UPDATE commercial_orders SET created_at = ? WHERE id = 'z1'`,
		time.Now().UTC().Add(-SweepStaleAge-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	swept, err := s.SweepStaleLinklessPending(ctx, 7)
	if err != nil || swept != 1 {
		t.Fatalf("sweep must release exactly the one stale row, swept=%d err=%v", swept, err)
	}
	if err := s.CreateOrder(ctx, OrderRow{ID: "z3", TenantID: 7, QuoteID: "zq3",
		AmountFen: 100, Currency: "CNY", CheckoutURL: "https://pay.example/z3"}); err != nil {
		t.Fatalf("after the sweep a fresh payable order must insert, got %v", err)
	}
}

func TestOrderGetDistinguishesPaidFromFulfilled(t *testing.T) {
	s, _ := testOrderStore(t)
	ctx := context.Background()
	mustCreateOrder(t, s, "o1", "q1")
	fact := mustRegisterAttempt(t, s, "a1", "o1", "wechat", "wxm", "m1")
	if err := s.ConfirmPayment(ctx, fact); err != nil {
		t.Fatal(err)
	}
	order, err := s.GetOrder(ctx, "o1")
	if err != nil || order.State != domain.OrderStatePaid {
		t.Fatalf("after confirm: %v state=%s", err, order.State)
	}
	if err := s.MarkFulfilled(ctx, "o1"); err != nil {
		t.Fatal(err)
	}
	order, err = s.GetOrder(ctx, "o1")
	if err != nil || order.State != domain.OrderStateFulfilled {
		t.Fatalf("after fulfill: %v state=%s", err, order.State)
	}
	if err := s.MarkFulfilled(ctx, "o1"); err != nil {
		t.Fatalf("mark fulfilled not idempotent: %v", err)
	}
	mustCreateOrder(t, s, "o2", "q2")
	if err := s.MarkFulfilled(ctx, "o2"); !errors.Is(err, ErrInvalidOrderState) {
		t.Fatalf("pending order marked fulfilled: %v", err)
	}
}

func TestPaymentConfirmRejectsUnregisteredOrMismatchedMerchant(t *testing.T) {
	s, db := testOrderStore(t)
	ctx := context.Background()
	mustCreateOrder(t, s, "o1", "q1")
	unknown := domain.PaymentFact{
		TenantID: 7, OrderID: "o1", AttemptID: "mX", Provider: "wechat",
		Merchant: "ghost", Transaction: "txnX", Amount: 100, Currency: "CNY", State: "succeeded",
	}
	if err := s.ConfirmPayment(ctx, unknown); !errors.Is(err, ErrPaymentAttemptNotFound) {
		t.Fatalf("unregistered merchant accepted: %v", err)
	}
	registered := mustRegisterAttempt(t, s, "a1", "o1", "wechat", "wxm", "m1")
	tampered := registered
	tampered.Amount = 50
	if err := s.ConfirmPayment(ctx, tampered); !errors.Is(err, domain.ErrPaymentMismatch) {
		t.Fatalf("tampered amount accepted: %v", err)
	}
	order, err := s.GetOrder(ctx, "o1")
	if err != nil || order.State != domain.OrderStatePending {
		t.Fatalf("rejected confirm mutated order: %v %s", err, order.State)
	}
	if got := countOutbox(t, db, ""); got != 0 {
		t.Fatalf("outbox events=%d, want 0", got)
	}
	if _, err := s.GetOrder(ctx, "missing"); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("missing order: %v", err)
	}
	if err := s.ConfirmPayment(ctx, func() domain.PaymentFact {
		f := registered
		f.OrderID = "missing"
		f.AttemptID = "m1"
		return f
	}()); !errors.Is(err, ErrOrderNotFound) && !errors.Is(err, domain.ErrPaymentMismatch) {
		t.Fatalf("fact on missing order: %v", err)
	}
}

// TestCurrentPurchaseOrderSkipsChannelFailedPending（A-29 / F109）：R2-28
// 之后 pending 不再等价于活的支付入口——channel_failed 死单不得赢得 pending
// 偏好并遮蔽同价面的已支付订单（否则 PurchaseStatus 会把永久 pending 的死单
// 投影为当前购买，恰好错过 awaiting+paid 的 paid_awaiting_activation 合成窗口）。
func TestCurrentPurchaseOrderSkipsChannelFailedPending(t *testing.T) {
	s, db := testOrderStore(t)
	ctx := context.Background()
	// finding 场景：渠道失败留下较旧的 channel_failed pending 死单（无链接）
	// ……
	if err := db.Exec(`INSERT INTO commercial_orders (id, tenant_id, quote_id, kind, amount_fen, currency, state, version, created_at, channel_failed)
		VALUES ('ord_dead_old', 7, 'qt_dead', 'purchase', 100, 'CNY', 'pending', 1, '2026-03-01T00:00:00Z', 1)`).Error; err != nil {
		t.Fatal(err)
	}
	// ……换新 quote 的订单经回调支付成功（更晚 created_at）。
	insertCreatedAtOrder(t, db, "ord_paid_new", "paid", "2026-07-01T00:00:00Z", false)
	row, err := s.CurrentPurchaseOrder(ctx, 7, 100, "CNY")
	if err != nil || row.ID != "ord_paid_new" {
		t.Fatalf("a channel-failed dead pending must never shadow the paid order of the same face (paid_awaiting_activation window), got %+v err=%v", row, err)
	}
	// 可付 pending 依旧优先于更旧的 channel_failed 死单。
	insertCreatedAtOrder(t, db, "ord_live_pending", "pending", "2026-02-01T00:00:00Z", true)
	row, err = s.CurrentPurchaseOrder(ctx, 7, 100, "CNY")
	if err != nil || row.ID != "ord_live_pending" {
		t.Fatalf("a payable pending must still win the pending preference, got %+v err=%v", row, err)
	}
}

// TestCreateOrderNormalizesCreatedAtToUTC（A-30 / F110）：SQLite 驱动以文本
// 存储 time.Time 且排序/清扫门按词法比较——本地时区渲染（+08:00）会排到同刻
// UTC 行之后、把清扫年龄门推过日边界（负偏移时区甚至立即清掉刚建的单）。
// createOrderTx 必须把任何来源的 CreatedAt 归一成 UTC 渲染（时刻不变）。
func TestCreateOrderNormalizesCreatedAtToUTC(t *testing.T) {
	s, db := testOrderStore(t)
	// +08:00 本地墙钟（与 UTC 相差 8 小时的同一时刻语义下取一个明确值）。
	local := time.Date(2026, 6, 1, 12, 0, 0, 0, time.FixedZone("CST", 8*3600))
	if err := s.CreateOrder(context.Background(), OrderRow{
		ID: "ord_tz", TenantID: 7, QuoteID: "qt_tz", AmountFen: 100, Currency: "CNY", CreatedAt: local,
	}); err != nil {
		t.Fatal(err)
	}
	var row OrderRow
	if err := db.Where("id = ?", "ord_tz").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if got := row.CreatedAt; got.Location() != time.UTC {
		t.Fatalf("created_at must be stored in its UTC rendering, got %v (loc=%v)", got, got.Location())
	}
	if got, want := row.CreatedAt.Unix(), local.Unix(); got != want {
		t.Fatalf("normalization must not move the instant: stored=%d original=%d", got, want)
	}
	// 零值 CreatedAt 也必须以 UTC 落库（否则 gorm 自动填充会用服务器本地时区）。
	if err := s.CreateOrder(context.Background(), OrderRow{
		ID: "ord_zero", TenantID: 7, QuoteID: "qt_zero", AmountFen: 100, Currency: "CNY",
	}); err != nil {
		t.Fatal(err)
	}
	var zero OrderRow
	if err := db.Where("id = ?", "ord_zero").First(&zero).Error; err != nil {
		t.Fatal(err)
	}
	if zero.CreatedAt.Location() != time.UTC || zero.CreatedAt.IsZero() {
		t.Fatalf("a zero CreatedAt must be auto-filled in UTC, got %v", zero.CreatedAt)
	}
}

// TestIsQuoteUniqueConflictClassifier（A-20 / F89）：quote_id 唯一索引冲突的
// 双驱动消息形状（SQLite 列面 / PostgreSQL 索引名）必须翻成
// ErrQuoteAlreadyUsed——并发双结账的 insert 竞态败者靠该映射走幂等重放分支，
// 而不是收到未映射的裸驱动错误。pending 唯一性形状（tenant_id 面）不得误匹配。
func TestIsQuoteUniqueConflictClassifier(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"sqlite column face", errors.New("UNIQUE constraint failed: commercial_orders.quote_id"), true},
		{"postgres index name", errors.New(`duplicate key value violates unique constraint "idx_commercial_orders.quote_id"`), true},
		{"pending-uniqueness index (pg)", errors.New(`duplicate key value violates unique constraint "uq_purchase_pending_per_tenant"`), false},
		{"pending-uniqueness (sqlite tenant face)", errors.New("UNIQUE constraint failed: commercial_orders.tenant_id"), false},
		{"unrelated", errors.New("no such table: commercial_orders"), false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		if got := isQuoteUniqueConflict(tc.err); got != tc.want {
			t.Fatalf("%s: isQuoteUniqueConflict = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestCloseAttemptAndRetireChannelAtomicPair（OCR C-07）：close 决策对
// （attempt→closed + 订单→channel_failed）单事务原子落地——干净路径两写同
// 落；订单在竞争窗口内离开 pending（并发支付）时整体拒绝（ErrOrderNotFound）
// 且 attempt 的 closed 写入随事务回滚（绝不留下「attempt 已 closed 但订单仍
// payable」的半状态——该状态无任何 API 恢复路径）。
func TestCloseAttemptAndRetireChannelAtomicPair(t *testing.T) {
	s, db := testOrderStore(t)
	ctx := context.Background()
	// 干净路径：pending 订单 + pending attempt → 两写原子落地。
	mustCreateOrder(t, s, "ord_atom_ok", "qt_atom_ok")
	if err := s.RegisterAttempt(ctx, PaymentAttemptRow{
		ID: "att_atom_ok", TenantID: 7, OrderID: "ord_atom_ok", Provider: "wechat",
		Merchant: "1900000109", MerchantOrderID: "mo_atom_ok", AmountFen: 100, Currency: "CNY",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.CloseAttemptAndRetireChannel(ctx, "ord_atom_ok"); err != nil {
		t.Fatalf("the atomic close pair must land: %v", err)
	}
	row, err := s.GetOrder(ctx, "ord_atom_ok")
	if err != nil || !row.ChannelFailed {
		t.Fatalf("the order must be retired (channel_failed), got %+v err=%v", row, err)
	}
	att, err := s.FirstPendingAttempt(ctx, "ord_atom_ok")
	if !errors.Is(err, ErrPaymentAttemptNotFound) {
		t.Fatalf("the attempt must be closed (no pending left), got %+v err=%v", att, err)
	}

	// 竞争路径：订单已 paid（channel_failed 谓词 0 行）→ 整体拒绝 + 回滚。
	mustCreateOrder(t, s, "ord_atom_paid", "qt_atom_paid")
	if err := s.RegisterAttempt(ctx, PaymentAttemptRow{
		ID: "att_atom_paid", TenantID: 7, OrderID: "ord_atom_paid", Provider: "wechat",
		Merchant: "1900000109", MerchantOrderID: "mo_atom_paid", AmountFen: 100, Currency: "CNY",
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE commercial_orders SET state = 'paid' WHERE id = 'ord_atom_paid'`).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.CloseAttemptAndRetireChannel(ctx, "ord_atom_paid"); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("a concurrently-paid order must refuse the half landing, got %v", err)
	}
	// The attempt write rolled back with the transaction: still pending.
	att2, err := s.FirstPendingAttempt(ctx, "ord_atom_paid")
	if err != nil || att2.ID != "att_atom_paid" {
		t.Fatalf("the attempt closed-write must roll back (still pending), got %+v err=%v", att2, err)
	}
	row2, err := s.GetOrder(ctx, "ord_atom_paid")
	if err != nil || row2.ChannelFailed {
		t.Fatalf("the paid order must stay un-retired, got %+v err=%v", row2, err)
	}
}
