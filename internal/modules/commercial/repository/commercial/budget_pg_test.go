//go:build commercial_integration

package commercial

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	domain "github.com/Tencent/WeKnora/internal/commercial"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// budgetPGFixture provisions the same isolated-schema, migration-backed
// PostgreSQL database the main template test uses, for the additional
// concurrency scenarios. It never runs without SAAS_TEST_PG_DSN.
func budgetPGFixture(t *testing.T) *gorm.DB {
	t.Helper()
	raw := os.Getenv("SAAS_TEST_PG_DSN")
	if raw == "" {
		t.Fatal("SAAS_TEST_PG_DSN is required for isolated integration evidence")
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		t.Fatal("test DSN must be a PostgreSQL URL")
	}
	admin, err := gorm.Open(postgres.Open(raw), &gorm.Config{})
	if err != nil {
		t.Fatal("test database unavailable")
	}
	adminSQL, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer adminSQL.Close()
	schema := fmt.Sprintf("saas_budget_%d", time.Now().UnixNano())
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Exec("DROP SCHEMA " + schema + " CASCADE") })
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
	t.Cleanup(func() { pool.Close() })
	pool.SetMaxOpenConns(20)
	migration, err := os.ReadFile("../../../../migrations/versioned/000116_commercial_budgets.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(string(migration)).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func TestBudgetPGConcurrentReservation(t *testing.T) {
	raw := os.Getenv("SAAS_TEST_PG_DSN")
	if raw == "" {
		t.Fatal("SAAS_TEST_PG_DSN is required for isolated integration evidence")
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		t.Fatal("test DSN must be a PostgreSQL URL")
	}
	admin, err := gorm.Open(postgres.Open(raw), &gorm.Config{})
	if err != nil {
		t.Fatal("test database unavailable")
	}
	adminSQL, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer adminSQL.Close()
	schema := fmt.Sprintf("saas_budget_%d", time.Now().UnixNano())
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	defer admin.Exec("DROP SCHEMA " + schema + " CASCADE")
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
	defer pool.Close()
	pool.SetMaxOpenConns(20)
	migration, err := os.ReadFile("../../../../migrations/versioned/000116_commercial_budgets.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(string(migration)).Error; err != nil {
		t.Fatal(err)
	}
	end := time.Now().UTC().Add(time.Hour)
	statements := []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO commercial_budget_accounts (tenant_id,verified_micro,unreflected_micro,held_micro,refund_locked_micro,watermark,version,verified_until) VALUES (7,100,0,0,0,'w1',1,?)", []any{end}},
		{"INSERT INTO commercial_task_budgets (tenant_id,run_id,limit_micro,spent_micro,held_micro,deadline,version) VALUES (7,'r1',100,0,0,?,1)", []any{end}},
		{"INSERT INTO commercial_budget_lots (tenant_id,lot_id,remaining_micro,held_micro,expires_at,issued_at) VALUES (7,'lot1',100,0,?,?)", []any{end, time.Now().UTC()}},
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt.sql, stmt.args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	store := NewBudgetStore(db)
	var accepted atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := store.Reserve(context.Background(), domain.BudgetRequest{TenantID: 7, RunID: "r1", Key: fmt.Sprintf("call-%d", i), Upper: 60, Deadline: time.Now().Add(time.Minute)})
			if err == nil {
				accepted.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if got := accepted.Load(); got != 1 {
		t.Fatalf("accepted=%d; want exactly one reservation", got)
	}
	var held int64
	if err := db.Raw("SELECT held_micro FROM commercial_budget_accounts WHERE tenant_id=7").Scan(&held).Error; err != nil {
		t.Fatal(err)
	}
	if held != 60 {
		t.Fatalf("held=%d; want 60", held)
	}
}

// TestBudgetPGTwoTasksCompeteForAccount: two independent task budgets draw
// from one account projection. Per-task limits both fit alone, but the
// account cannot cover two holds — exactly one task wins, and the winning
// hold appears on the account AND exactly that task, never both.
func TestBudgetPGTwoTasksCompeteForAccount(t *testing.T) {
	db := budgetPGFixture(t)
	end := time.Now().UTC().Add(time.Hour)
	statements := []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO commercial_budget_accounts (tenant_id,verified_micro,unreflected_micro,held_micro,refund_locked_micro,watermark,version,verified_until) VALUES (7,100,0,0,0,'w1',1,?)", []any{end}},
		{"INSERT INTO commercial_task_budgets (tenant_id,run_id,limit_micro,spent_micro,held_micro,deadline,version) VALUES (7,'r1',60,0,0,?,1)", []any{end}},
		{"INSERT INTO commercial_task_budgets (tenant_id,run_id,limit_micro,spent_micro,held_micro,deadline,version) VALUES (7,'r2',60,0,0,?,1)", []any{end}},
		{"INSERT INTO commercial_budget_lots (tenant_id,lot_id,remaining_micro,held_micro,expires_at,issued_at) VALUES (7,'lot1',100,0,?,?)", []any{end, time.Now().UTC()}},
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt.sql, stmt.args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	store := NewBudgetStore(db)
	var accepted atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			run := "r1"
			if i%2 == 1 {
				run = "r2"
			}
			_, err := store.Reserve(context.Background(), domain.BudgetRequest{TenantID: 7, RunID: run, Key: fmt.Sprintf("task-%d", i), Upper: 60, Deadline: time.Now().Add(time.Minute)})
			if err == nil {
				accepted.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if got := accepted.Load(); got != 1 {
		t.Fatalf("accepted=%d; want exactly one reservation across both tasks", got)
	}
	var accountHeld int64
	if err := db.Raw("SELECT held_micro FROM commercial_budget_accounts WHERE tenant_id=7").Scan(&accountHeld).Error; err != nil {
		t.Fatal(err)
	}
	var taskHeldSum int64
	if err := db.Raw("SELECT COALESCE(SUM(held_micro),0) FROM commercial_task_budgets WHERE tenant_id=7").Scan(&taskHeldSum).Error; err != nil {
		t.Fatal(err)
	}
	if accountHeld != 60 || taskHeldSum != 60 {
		t.Fatalf("account held=%d task held sum=%d; want 60/60", accountHeld, taskHeldSum)
	}
	var winners int64
	if err := db.Raw("SELECT COUNT(*) FROM commercial_task_budgets WHERE tenant_id=7 AND held_micro=60").Scan(&winners).Error; err != nil {
		t.Fatal(err)
	}
	if winners != 1 {
		t.Fatalf("tasks at full hold=%d; want exactly one winning task", winners)
	}
}

// TestBudgetPGRefundLockCompetesWithReservation: refund locks and
// reservations share one CAS-guarded projection. Twenty concurrent draws of
// 60 against 100 admit exactly one winner overall; the final DB protections
// must reflect that single winner on refund_locked_micro and held_micro.
func TestBudgetPGRefundLockCompetesWithReservation(t *testing.T) {
	db := budgetPGFixture(t)
	end := time.Now().UTC().Add(time.Hour)
	statements := []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO commercial_budget_accounts (tenant_id,verified_micro,unreflected_micro,held_micro,refund_locked_micro,watermark,version,verified_until) VALUES (7,100,0,0,0,'w1',1,?)", []any{end}},
		{"INSERT INTO commercial_task_budgets (tenant_id,run_id,limit_micro,spent_micro,held_micro,deadline,version) VALUES (7,'r1',100,0,0,?,1)", []any{end}},
		{"INSERT INTO commercial_budget_lots (tenant_id,lot_id,remaining_micro,held_micro,expires_at,issued_at) VALUES (7,'lot1',100,0,?,?)", []any{end, time.Now().UTC()}},
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt.sql, stmt.args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	store := NewBudgetStore(db)
	var wins atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				if err := store.LockRefunds(context.Background(), 7, 60); err == nil {
					wins.Add(1)
				}
				return
			}
			if _, err := store.Reserve(context.Background(), domain.BudgetRequest{TenantID: 7, RunID: "r1", Key: fmt.Sprintf("refund-call-%d", i), Upper: 60, Deadline: time.Now().Add(time.Minute)}); err == nil {
				wins.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if got := wins.Load(); got != 1 {
		t.Fatalf("winners=%d; want exactly one lock-or-reserve", got)
	}
	var held, refundLocked int64
	if err := db.Raw("SELECT held_micro FROM commercial_budget_accounts WHERE tenant_id=7").Scan(&held).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Raw("SELECT refund_locked_micro FROM commercial_budget_accounts WHERE tenant_id=7").Scan(&refundLocked).Error; err != nil {
		t.Fatal(err)
	}
	if held+refundLocked != 60 || (held != 0 && held != 60) {
		t.Fatalf("held=%d refund_locked=%d; want a single 60 draw", held, refundLocked)
	}
}
