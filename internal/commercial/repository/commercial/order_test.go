package commercial

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	domain "github.com/Tencent/WeKnora/internal/commercial"
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
	if err := db.AutoMigrate(&OrderRow{}, &PaymentAttemptRow{}, &OutboxEvent{}, &PaymentAnomalyRow{}); err != nil {
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

func TestPaymentSecondSuccessfulTransactionOnSameAttemptPreservesOriginal(t *testing.T) {
	s, db := testOrderStore(t)
	ctx := context.Background()
	mustCreateOrder(t, s, "o1", "q1")
	first := mustRegisterAttempt(t, s, "a1", "o1", "wechat", "wxm", "m1")
	first.Transaction = "txn_first"
	second := first
	second.Transaction = "txn_second"
	for _, fact := range []domain.PaymentFact{first, second, first, second} {
		if err := s.ConfirmPayment(ctx, fact); err != nil {
			t.Fatal(err)
		}
	}
	attempt := getAttempt(t, db, "a1")
	if attempt.ProviderTransactionID == nil || *attempt.ProviderTransactionID != first.Transaction {
		t.Fatalf("winner changed: %+v", attempt)
	}
	order, err := s.GetOrder(ctx, "o1")
	if err != nil {
		t.Fatal(err)
	}
	if order.State != domain.OrderStatePaid || order.Version != 2 {
		t.Fatalf("order: %+v", order)
	}
	if n := countOutbox(t, db, OutboxKindFulfill); n != 1 {
		t.Fatalf("fulfill=%d", n)
	}
	if n := countOutbox(t, db, OutboxKindOverPaid); n != 1 {
		t.Fatalf("overpaid=%d", n)
	}
	var audits []OutboxEvent
	if err := db.Where("kind = ?", OutboxKindOverPaid).Find(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if len(audits) != 1 || !strings.Contains(audits[0].PayloadJSON, second.Transaction) || strings.Contains(audits[0].PayloadJSON, first.Transaction) {
		t.Fatalf("wrong audit: %+v", audits)
	}
	var anomalies []PaymentAnomalyRow
	if err := db.Where("order_id = ?", "o1").Find(&anomalies).Error; err != nil {
		t.Fatal(err)
	}
	if len(anomalies) != 1 {
		t.Fatalf("later same-attempt collection must have one durable anomaly binding, got %+v", anomalies)
	}
	anomaly := anomalies[0]
	if anomaly.TenantID != 7 || anomaly.AttemptID != "m1" || anomaly.Provider != "wechat" || anomaly.Merchant != "wxm" ||
		anomaly.Transaction != second.Transaction || anomaly.Kind != PaymentAnomalyKindOverPaid ||
		anomaly.ExpectedAmountFen != 0 || anomaly.ActualAmountFen != int64(second.Amount) ||
		anomaly.ExpectedCurrency != second.Currency || anomaly.ActualCurrency != second.Currency || anomaly.State != PaymentAnomalyStateAwaiting {
		t.Fatalf("later same-attempt fact not bound exactly: %+v", anomaly)
	}
}

func TestPaymentAnomalyUniqueConflictRequiresExactImmutableFact(t *testing.T) {
	s, db := testOrderStore(t)
	ctx := context.Background()
	row := PaymentAnomalyRow{
		TenantID: 7, OrderID: "o1", AttemptID: "m1", Provider: "wechat", Merchant: "wxm", Transaction: "txn_second",
		Kind: PaymentAnomalyKindOverPaid, ExpectedAmountFen: 0, ActualAmountFen: 100,
		ExpectedCurrency: "CNY", ActualCurrency: "CNY", State: PaymentAnomalyStateAwaiting,
	}
	if err := s.RecordPaymentAnomaly(ctx, row); err != nil {
		t.Fatal(err)
	}
	contradiction := row
	contradiction.OrderID = "o2"
	if err := s.RecordPaymentAnomaly(ctx, contradiction); err == nil {
		t.Fatal("same provider/merchant/transaction with a contradictory fact must not be treated as idempotent success")
	}
	var anomalies []PaymentAnomalyRow
	if err := db.Where("provider = ? AND merchant = ? AND `transaction` = ?", row.Provider, row.Merchant, row.Transaction).Find(&anomalies).Error; err != nil {
		t.Fatal(err)
	}
	if len(anomalies) != 1 || anomalies[0].OrderID != row.OrderID {
		t.Fatalf("conflicting duplicate changed the durable fact: %+v", anomalies)
	}
}

func TestPaymentSecondSameAttemptConfirmationRollsBackAnomalyWhenOutboxWriteFails(t *testing.T) {
	s, db := testOrderStore(t)
	ctx := context.Background()
	mustCreateOrder(t, s, "o1", "q1")
	winner := mustRegisterAttempt(t, s, "a1", "o1", "wechat", "wxm", "m1")
	winner.Transaction = "txn_first"
	if err := s.ConfirmPayment(ctx, winner); err != nil {
		t.Fatal(err)
	}
	callback := "test:reject_overpayment_outbox"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "commercial_outbox_events" {
			if ev, ok := tx.Statement.Dest.(*OutboxEvent); ok && ev.Kind == OutboxKindOverPaid {
				tx.AddError(errors.New("injected outbox insert failure"))
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer db.Callback().Create().Remove(callback)
	later := winner
	later.Transaction = "txn_second"
	if err := s.ConfirmPayment(ctx, later); err == nil {
		t.Fatal("expected injected over-payment event write failure")
	}
	if attempt := getAttempt(t, db, "a1"); attempt.ProviderTransactionID == nil || *attempt.ProviderTransactionID != winner.Transaction {
		t.Fatalf("failed later confirmation changed winner attempt: %+v", attempt)
	}
	if n := countOutbox(t, db, OutboxKindOverPaid); n != 0 {
		t.Fatalf("failed outbox write persisted %d over-payment events", n)
	}
	var anomalies []PaymentAnomalyRow
	if err := db.Where("`transaction` = ?", later.Transaction).Find(&anomalies).Error; err != nil {
		t.Fatal(err)
	}
	if len(anomalies) != 0 {
		t.Fatalf("failed transaction left an anomaly without its event: %+v", anomalies)
	}
}

func TestPaymentTransactionUniqueConflictDoesNotMutateAttemptOrOrder(t *testing.T) {
	s, db := testOrderStore(t)
	ctx := context.Background()
	mustCreateOrder(t, s, "o1", "q1")
	mustCreateOrder(t, s, "o2", "q2")
	first := mustRegisterAttempt(t, s, "a1", "o1", "wechat", "wxm", "m1")
	second := mustRegisterAttempt(t, s, "a2", "o2", "wechat", "wxm", "m2")
	first.Transaction = "txn_shared"
	second.Transaction = first.Transaction
	if err := s.ConfirmPayment(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfirmPayment(ctx, second); err == nil {
		t.Fatal("expected unique transaction conflict")
	}
	attempt := getAttempt(t, db, "a2")
	if attempt.State != PaymentAttemptStatePending || attempt.ProviderTransactionID != nil {
		t.Fatalf("attempt mutated: %+v", attempt)
	}
	order, err := s.GetOrder(ctx, "o2")
	if err != nil {
		t.Fatal(err)
	}
	if order.State != domain.OrderStatePending || order.Version != 1 {
		t.Fatalf("order mutated: %+v", order)
	}
	var n int64
	if err := db.Model(&OutboxEvent{}).Where("event_key = ? OR event_key LIKE ?", OutboxKindFulfill+":o2", OutboxKindOverPaid+":o2:%").Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("order B events=%d", n)
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

// TestCurrentPurchaseOrderSkipsLinklessPendingDegradedRow（OCR84-R1-03 high）：
// CurrentPurchaseOrder 的 pending 偏好谓词必须与注释声明的
// CurrentPendingPurchaseOrder 的可支付谓词也要求 checkout_url 非空。
// 持久化降级行（渠道 Create 成功但 SetCheckoutURL 失败且不标 channel_failed，
// R2-27 形态）不携带支付入口：若它赢得偏好，purchase.go 的 orderViewFromRow
// 投影出无支付入口的永久 pending 订单，并遮蔽同价位已支付订单、错过
// paid_awaiting_activation 合成窗口，直到 SweepStaleLinklessPending 清扫。
func TestCurrentPurchaseOrderSkipsLinklessPendingDegradedRow(t *testing.T) {
	s, db := testOrderStore(t)
	ctx := context.Background()
	// 新的 link-less pending 降级行 + 旧的已支付行，同价位同租户。
	insertCreatedAtOrder(t, db, "ord_0000degraded", "pending", "2026-07-01T00:00:00Z", false)
	insertCreatedAtOrder(t, db, "ord_0001paid", "paid", "2026-06-01T00:00:00Z", true)
	row, err := s.CurrentPurchaseOrder(ctx, 7, 100, "CNY")
	if err != nil || row.ID != "ord_0001paid" {
		t.Fatalf("a link-less degraded pending row must NOT win the preference over the paid order, got %+v err=%v", row, err)
	}
	if row.State != domain.OrderStatePaid {
		t.Fatalf("the projected current purchase must be the PAID row, got state=%s", row.State)
	}
	// 对照：带链接的 pending 仍按既有语义赢得偏好（最新 pending 优先）。
	insertCreatedAtOrder(t, db, "ord_0002payable", "pending", "2026-08-01T00:00:00Z", true)
	row, err = s.CurrentPurchaseOrder(ctx, 7, 100, "CNY")
	if err != nil || row.ID != "ord_0002payable" {
		t.Fatalf("a payable pending row must still win the preference, got %+v err=%v", row, err)
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
	var beforeConflict []PaymentAnomalyRow
	if err := db.Order("id ASC").Find(&beforeConflict).Error; err != nil {
		t.Fatal(err)
	}
	if len(beforeConflict) != 1 {
		t.Fatalf("mismatch anomaly rows=%d, want 1", len(beforeConflict))
	}
	if _, err := s.GetOrder(ctx, "missing"); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("missing order: %v", err)
	}
	if err := s.ConfirmPayment(ctx, func() domain.PaymentFact {
		f := registered
		f.OrderID = "missing"
		f.AttemptID = "m1"
		return f
	}()); !errors.Is(err, ErrOrderNotFound) && !errors.Is(err, domain.ErrPaymentMismatch) && !errors.Is(err, ErrPaymentAnomalyConflict) {
		t.Fatalf("fact on missing order: %v", err)
	}
	var afterConflict []PaymentAnomalyRow
	if err := db.Order("id ASC").Find(&afterConflict).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(afterConflict, beforeConflict) {
		t.Fatalf("conflicting replay mutated anomaly facts: before=%+v after=%+v", beforeConflict, afterConflict)
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

// ---- #82 Task 13 (OCR r2): purchase race hardening ----

// TestCurrentPayableFallbackSkipsDeadPending（r2:348）：pending 偏好循环之后的
// fallback 不得无条件返回最新行——同价面最新行是 channel_failed 死 pending（paid
// 订单之后渠道 Create 又失败的形状）时，fallback 必须跳过死行、返回最新的
// paid/fulfilled 行，否则 PurchaseStatus 投影永久 pending 死单、丢掉
// paid_awaiting_activation 合成窗口。
func TestCurrentPayableFallbackSkipsDeadPending(t *testing.T) {
	s, db := testOrderStore(t)
	ctx := context.Background()
	// O1 paid（较早）——真实支付入口已完结的订单。
	insertCreatedAtOrder(t, db, "ord_paid_older", "paid", "2026-03-01T00:00:00Z", true)
	// O2 pending + channel_failed（较新、无链接）——死的支付入口。
	if err := db.Exec(`INSERT INTO commercial_orders (id, tenant_id, quote_id, kind, amount_fen, currency, state, version, created_at, channel_failed)
		VALUES ('ord_dead_newer', 7, 'qt_dead_newer', 'purchase', 100, 'CNY', 'pending', 1, '2026-07-01T00:00:00Z', 1)`).Error; err != nil {
		t.Fatal(err)
	}
	row, err := s.CurrentPurchaseOrder(ctx, 7, 100, "CNY")
	if err != nil || row.ID != "ord_paid_older" {
		t.Fatalf("the fallback must skip the dead pending and answer the newest paid row, got %+v err=%v", row, err)
	}
}

// TestCurrentPaidAwaitingActivationPurchaseOrder（D11 / r2:253）：paid-awaiting
// 窗口探测——state=paid 即「已付款未履约」（paid→fulfilled 由 OrderRow.State 承
// 载，fulfilled 行自然不命中）。三形态：命中最新 paid；fulfilled/异 kind/异价面
// NotFound。
func TestCurrentPaidAwaitingActivationPurchaseOrder(t *testing.T) {
	s, db := testOrderStore(t)
	ctx := context.Background()
	insertCreatedAtOrder(t, db, "ord_paid_old", "paid", "2026-03-01T00:00:00Z", true)
	insertCreatedAtOrder(t, db, "ord_paid_new", "paid", "2026-07-01T00:00:00Z", true)
	row, err := s.CurrentPaidAwaitingActivationPurchaseOrder(ctx, 7, 100, "CNY")
	if err != nil || row.ID != "ord_paid_new" {
		t.Fatalf("the NEWEST paid unfulfilled purchase order must answer, got %+v err=%v", row, err)
	}
	// fulfilled 之后退出窗口。
	if err := db.Exec(`UPDATE commercial_orders SET state = 'fulfilled' WHERE id = ?`,
		"ord_paid_new").Error; err != nil {
		t.Fatal(err)
	}
	row, err = s.CurrentPaidAwaitingActivationPurchaseOrder(ctx, 7, 100, "CNY")
	if err != nil || row.ID != "ord_paid_old" {
		t.Fatalf("a fulfilled order leaves the paid-awaiting window (the next paid row answers), got %+v err=%v", row, err)
	}
	if err := db.Exec(`UPDATE commercial_orders SET state = 'fulfilled' WHERE id = ?`,
		"ord_paid_old").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.CurrentPaidAwaitingActivationPurchaseOrder(ctx, 7, 100, "CNY"); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("no paid unfulfilled order must answer ErrOrderNotFound, got %v", err)
	}
	// 异 kind / 异价面不命中。
	insertCreatedAtOrder(t, db, "ord_upgrade_paid", "paid", "2026-08-01T00:00:00Z", true)
	if err := db.Exec(`UPDATE commercial_orders SET kind = 'upgrade' WHERE id = ?`,
		"ord_upgrade_paid").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.CurrentPaidAwaitingActivationPurchaseOrder(ctx, 7, 100, "CNY"); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("a paid upgrade order must not answer the purchase window read, got %v", err)
	}
	if _, err := s.CurrentPaidAwaitingActivationPurchaseOrder(ctx, 7, 9900, "CNY"); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("a different price face must not answer, got %v", err)
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
		{"postgres index name (dotted, defensive)", errors.New(`duplicate key value violates unique constraint "idx_commercial_orders.quote_id"`), true},
		// (r2:212) The REAL PG driver face: gorm's default NamingStrategy
		// renders idx_<table>_<column> — the underscore form the previous
		// row's dotted literal never matches on a live PostgreSQL.
		{"postgres index name (underscore, real driver face)", errors.New(`duplicate key value violates unique constraint "idx_commercial_orders_quote_id"`), true},
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

// ---- #84 Task 1: abnormal payment facts are retained (spec L127) ----

// TestConfirmPaymentMismatchRetainsExternalFactWithoutFulfillment（G1）：
// 错金额的已验签事实不再整体丢弃——事务照旧回滚（不履约、无 outbox 事件、
// attempt 不动），但异常资金事实以独立事务幂等落库为 awaiting_disposition
// 的 PaymentAnomalyRow（kind 由闭合分类定：actual < expected → partial）。
func TestConfirmPaymentMismatchRetainsExternalFactWithoutFulfillment(t *testing.T) {
	s, db := testOrderStore(t)
	ctx := context.Background()
	mustCreateOrder(t, s, "o1", "q1")
	att := mustRegisterAttempt(t, s, "a1", "o1", "wechat", "wxm", "m1")
	mismatched := att
	mismatched.Amount = 50 // order face 100, fact 50 → partial payment
	if err := s.ConfirmPayment(ctx, mismatched); !errors.Is(err, domain.ErrPaymentMismatch) {
		t.Fatalf("want ErrPaymentMismatch, got %v", err)
	}
	ok, err := s.HasUnresolvedPaymentAnomaly(ctx, "o1")
	if err != nil || !ok {
		t.Fatalf("mismatch fact must be retained as unresolved anomaly, got %v %v", ok, err)
	}
	var anomaly PaymentAnomalyRow
	if err := db.Where("order_id = ?", "o1").First(&anomaly).Error; err != nil {
		t.Fatal(err)
	}
	if anomaly.Kind != PaymentAnomalyKindPartial ||
		anomaly.ExpectedAmountFen != 100 || anomaly.ActualAmountFen != 50 ||
		anomaly.Transaction != att.Transaction || anomaly.State != PaymentAnomalyStateAwaiting {
		t.Fatalf("anomaly snapshot mismatch: %+v", anomaly)
	}
	row, err := s.GetOrder(ctx, "o1")
	if err != nil || row.State != domain.OrderStatePending {
		t.Fatalf("mismatch must never fulfill: state=%s err=%v", row.State, err)
	}
	if n := countOutbox(t, db, OutboxKindFulfill); n != 0 {
		t.Fatalf("fulfill events %d, want 0", n)
	}
	attempt := getAttempt(t, db, "a1")
	if attempt.State != PaymentAttemptStatePending || attempt.ProviderTransactionID != nil {
		t.Fatalf("the attempt must stay pending: %+v", attempt)
	}
	// 渠道重投同一通知：仍是 mismatch + anomaly 行数不变（终态幂等的事实面）。
	if err := s.ConfirmPayment(ctx, mismatched); !errors.Is(err, domain.ErrPaymentMismatch) {
		t.Fatalf("replay want ErrPaymentMismatch, got %v", err)
	}
	var n int64
	db.Model(&PaymentAnomalyRow{}).Count(&n)
	if n != 1 {
		t.Fatalf("replay must not mint a second anomaly row, got %d", n)
	}
}

// TestConfirmPaymentMismatchAnomalyInsertFailurePropagates（Review Focus 2）：
// anomaly 落库失败必须上抛原始错误（覆盖 ErrPaymentMismatch）——调用方因此
// 回退非 2xx，渠道重试兜底，绝不能「200 却无事实」。
func TestConfirmPaymentMismatchAnomalyInsertFailurePropagates(t *testing.T) {
	s, db := testOrderStore(t)
	ctx := context.Background()
	mustCreateOrder(t, s, "o1", "q1")
	att := mustRegisterAttempt(t, s, "a1", "o1", "wechat", "wxm", "m1")
	mismatched := att
	mismatched.Amount = 50
	if err := db.Migrator().DropTable(&PaymentAnomalyRow{}); err != nil {
		t.Fatal(err)
	}
	err := s.ConfirmPayment(ctx, mismatched)
	if err == nil || errors.Is(err, domain.ErrPaymentMismatch) {
		t.Fatalf("want the raw insert error to propagate, got %v", err)
	}
}

// TestConfirmPaymentNotSucceededStaysRollbackNoAnomaly（Review Focus 1 /
// 审查 R1）：金额/币种/身份全对、仅 trade_state 非 SUCCESS 的已验签通知
// （两渠道 Verify 均不过滤 trade_state）是独立分类 ErrPaymentNotSucceeded——
// 零落库、整体回滚、绝不混入 anomaly（ValidatePayment 的 State 判定不再触发，
// 否则「金额全对但状态 CLOSED」会被误判为假 amount_mismatch）。
func TestConfirmPaymentNotSucceededStaysRollbackNoAnomaly(t *testing.T) {
	s, db := testOrderStore(t)
	ctx := context.Background()
	mustCreateOrder(t, s, "o1", "q1")
	att := mustRegisterAttempt(t, s, "a1", "o1", "wechat", "wxm", "m1")
	closed := att
	closed.Transaction = "txn_closed"
	closed.State = "closed"
	if err := s.ConfirmPayment(ctx, closed); !errors.Is(err, domain.ErrPaymentNotSucceeded) {
		t.Fatalf("want ErrPaymentNotSucceeded, got %v", err)
	}
	if ok, _ := s.HasUnresolvedPaymentAnomaly(ctx, "o1"); ok {
		t.Fatal("non-succeeded must not land an anomaly")
	}
	var n int64
	db.Model(&PaymentAnomalyRow{}).Count(&n)
	if n != 0 {
		t.Fatalf("non-succeeded must persist nothing, got %d rows", n)
	}
	row, err := s.GetOrder(ctx, "o1")
	if err != nil || row.State != domain.OrderStatePending {
		t.Fatalf("state=%s err=%v, want pending", row.State, err)
	}
	if got := countOutbox(t, db, ""); got != 0 {
		t.Fatalf("outbox events=%d, want 0", got)
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

func TestListAwaitingPaymentAnomalyOrderIDsScopesTenantAndState(t *testing.T) {
	s, _ := testOrderStore(t)
	ctx := context.Background()
	for _, row := range []PaymentAnomalyRow{
		{ID: "a1", TenantID: 7, OrderID: "o1", Provider: "p", Merchant: "m", Transaction: "t1", State: PaymentAnomalyStateAwaiting},
		{ID: "a2", TenantID: 7, OrderID: "o2", Provider: "p", Merchant: "m", Transaction: "t2", State: PaymentAnomalyStateResolved},
		{ID: "a3", TenantID: 8, OrderID: "o3", Provider: "p", Merchant: "m", Transaction: "t3", State: PaymentAnomalyStateAwaiting},
		{ID: "a4", TenantID: 7, OrderID: "o1", Provider: "p", Merchant: "m", Transaction: "t4", State: PaymentAnomalyStateAwaiting},
	} {
		if err := s.db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	ids, err := s.ListAwaitingPaymentAnomalyOrderIDs(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 {
		t.Fatalf("awaiting tenant set = %#v, want only o1", ids)
	}
	if _, ok := ids["o1"]; !ok {
		t.Fatalf("missing awaiting order: %#v", ids)
	}
}
