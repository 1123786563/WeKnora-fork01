//go:build commercial_integration

package commercial

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	domain "github.com/Tencent/WeKnora/internal/commercial"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func testOrderPGStore(t *testing.T) (*OrderStore, *gorm.DB) {
	t.Helper()
	raw := os.Getenv("SAAS_TEST_PG_DSN")
	if raw == "" {
		t.Fatal("SAAS_TEST_PG_DSN is required for PostgreSQL race evidence")
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		t.Fatal("SAAS_TEST_PG_DSN must be a PostgreSQL URL")
	}
	admin, err := gorm.Open(postgres.Open(raw), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	adminPool, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := adminPool.Close(); err != nil {
			t.Errorf("close admin pool: %v", err)
		}
	})
	schema := fmt.Sprintf("saas_order_%d", time.Now().UnixNano())
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Errorf("drop schema: %v", err)
		}
	})
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Close(); err != nil {
			t.Errorf("close schema pool: %v", err)
		}
	})
	pool.SetMaxOpenConns(10)
	if err := db.AutoMigrate(&OrderRow{}, &PaymentAttemptRow{}, &OutboxEvent{}); err != nil {
		t.Fatal(err)
	}
	for _, idx := range []string{
		"CREATE UNIQUE INDEX uq_attempt_merchant_order ON commercial_payment_attempts(provider, merchant, merchant_order_id)",
		"CREATE UNIQUE INDEX uq_attempt_provider_txn ON commercial_payment_attempts(provider, merchant, provider_transaction_id)",
	} {
		if err := db.Exec(idx).Error; err != nil {
			t.Fatal(err)
		}
	}
	return NewOrderStore(db), db
}

func TestPaymentConcurrentDistinctTransactionsOnSameAttemptKeepsOneWinner(t *testing.T) {
	s, db := testOrderPGStore(t)
	mustCreateOrder(t, s, "o1", "q1")
	first := mustRegisterAttempt(t, s, "a1", "o1", "wechat", "wxm", "m1")
	second := first
	first.Transaction = "txn_first"
	second.Transaction = "txn_second"

	// Both transactions pass their pending read before either claim UPDATE.
	// PostgreSQL then serializes the two conditional writes and one gets 0 rows.
	release := make(chan struct{})
	var mu sync.Mutex
	arrivals := 0
	var claimRows []int64
	callbackTable := func(tx *gorm.DB) bool {
		return tx.Statement != nil && tx.Statement.Table == "commercial_payment_attempts" &&
			strings.Contains(tx.Statement.SQL.String(), "provider_transaction_id")
	}
	if err := db.Callback().Update().Before("gorm:update").Register("test/attempt_claim_barrier", func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != "commercial_payment_attempts" {
			return
		}
		mu.Lock()
		arrivals++
		if arrivals == 2 {
			close(release)
		}
		mu.Unlock()
		select {
		case <-release:
		case <-time.After(5 * time.Second):
			mu.Lock()
			gotArrivals := arrivals
			mu.Unlock()
			_ = tx.AddError(fmt.Errorf("attempt claim barrier timed out after %d arrivals", gotArrivals))
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Callback().Update().After("gorm:update").Register("test/attempt_claim_rows", func(tx *gorm.DB) {
		if callbackTable(tx) && tx.Error == nil {
			mu.Lock()
			claimRows = append(claimRows, tx.RowsAffected)
			mu.Unlock()
		}
	}); err != nil {
		t.Fatal(err)
	}

	errs := make(chan error, 2)
	go func() { errs <- s.ConfirmPayment(context.Background(), first) }()
	go func() { errs <- s.ConfirmPayment(context.Background(), second) }()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	for i := 0; i < 2; i++ {
		select {
		case err := <-errs:
			if err != nil {
				t.Fatalf("confirm %d: %v", i, err)
			}
		case <-deadline.C:
			t.Fatalf("timed out waiting for confirmation %d of 2", i+1)
		}
	}
	mu.Lock()
	gotRows := append([]int64(nil), claimRows...)
	gotArrivals := arrivals
	mu.Unlock()
	if gotArrivals != 2 || len(gotRows) != 2 || !((gotRows[0] == 1 && gotRows[1] == 0) || (gotRows[0] == 0 && gotRows[1] == 1)) {
		t.Fatalf("claims: arrivals=%d rows=%v; want two arrivals and 1/0", gotArrivals, gotRows)
	}
	attempt := getAttempt(t, db, "a1")
	if attempt.ProviderTransactionID == nil {
		t.Fatal("no winner")
	}
	winner := *attempt.ProviderTransactionID
	loser := first.Transaction
	if winner == first.Transaction {
		loser = second.Transaction
	} else if winner != second.Transaction {
		t.Fatalf("unexpected winner %q", winner)
	}
	order, err := s.GetOrder(context.Background(), "o1")
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
		t.Fatalf("overpayment=%d", n)
	}
	if n := countOutbox(t, db, ""); n != 2 {
		t.Fatalf("all events=%d", n)
	}
	var audits []OutboxEvent
	if err := db.Where("kind = ?", OutboxKindOverPaid).Find(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if len(audits) != 1 || !strings.Contains(audits[0].PayloadJSON, loser) || strings.Contains(audits[0].PayloadJSON, winner) {
		t.Fatalf("wrong audit: %+v", audits)
	}
	var attempts int64
	if err := db.Model(&PaymentAttemptRow{}).Count(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if attempts != 1 {
		t.Fatalf("attempt rows=%d", attempts)
	}
}

func TestPaymentConcurrentDistinctAttemptsStoreOnlyOrderCASWinner(t *testing.T) {
	s, db := testOrderPGStore(t)
	ctx := context.Background()
	mustCreateOrder(t, s, "o-cas-winner", "q-cas-winner")
	facts := []domain.PaymentFact{}
	for _, id := range []string{"z-winner", "a-later"} {
		merchantOrder := "mo-" + id
		if err := s.RegisterAttempt(ctx, PaymentAttemptRow{ID: id, TenantID: 7, OrderID: "o-cas-winner", Provider: "wechat", Merchant: "wxm", MerchantOrderID: merchantOrder, AmountFen: 1000, Currency: domain.CurrencyCNY}); err != nil {
			t.Fatal(err)
		}
		facts = append(facts, domain.PaymentFact{TenantID: 7, OrderID: "o-cas-winner", AttemptID: merchantOrder, Provider: "wechat", Merchant: "wxm", Transaction: "txn-" + id, Amount: 1000, Currency: domain.CurrencyCNY, State: "succeeded"})
	}
	release := make(chan struct{})
	var mu sync.Mutex
	arrivals := 0
	if err := db.Callback().Update().Before("gorm:update").Register("test/order_cas_barrier", func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != "commercial_orders" {
			return
		}
		mu.Lock()
		arrivals++
		if arrivals == 2 {
			close(release)
		}
		mu.Unlock()
		select {
		case <-release:
		case <-time.After(5 * time.Second):
			tx.AddError(fmt.Errorf("order CAS barrier timed out"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	errs := make(chan error, 2)
	for _, fact := range facts {
		f := fact
		go func() { errs <- s.ConfirmPayment(ctx, f) }()
	}
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for i := 0; i < 2; i++ {
		select {
		case err := <-errs:
			if err != nil {
				t.Fatalf("confirm: %v", err)
			}
		case <-timer.C:
			t.Fatal("confirmation timed out")
		}
	}
	if err := db.Callback().Update().Remove("test/order_cas_barrier"); err != nil {
		t.Fatal(err)
	}
	var order OrderRow
	if err := db.Where("id = ?", "o-cas-winner").First(&order).Error; err != nil {
		t.Fatal(err)
	}
	var fulfill, audit []OutboxEvent
	if err := db.Where("kind = ?", OutboxKindFulfill).Find(&fulfill).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("kind = ?", OutboxKindOverPaid).Find(&audit).Error; err != nil {
		t.Fatal(err)
	}
	if order.State != domain.OrderStatePaid || len(fulfill) != 1 || len(audit) != 1 {
		t.Fatalf("order=%+v fulfill=%d audit=%d", order, len(fulfill), len(audit))
	}
	var winner paymentEventPayload
	if err := json.Unmarshal([]byte(fulfill[0].PayloadJSON), &winner); err != nil {
		t.Fatal(err)
	}
	var attempt PaymentAttemptRow
	if err := db.Where("id = ?", winner.AttemptID).First(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	if attempt.State != PaymentAttemptStateSucceeded || attempt.ProviderTransactionID == nil || *attempt.ProviderTransactionID != winner.Transaction {
		t.Fatalf("payload winner=%+v attempt=%+v", winner, attempt)
	}
	if !strings.Contains(audit[0].PayloadJSON, "txn-") || strings.Contains(audit[0].PayloadJSON, winner.Transaction) {
		t.Fatalf("audit=%+v winner=%+v", audit[0], winner)
	}
}
