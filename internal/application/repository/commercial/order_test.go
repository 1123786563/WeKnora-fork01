package commercial

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	domain "github.com/Tencent/WeKnora/internal/commercial"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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
