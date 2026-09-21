package commercial

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestReserveWorksOnMigratedSchema proves the sqlite migration track builds a
// commercial_reservations table the GORM rows can actually write to: run the
// 000036 budget DDL followed by the 000079 owner-column ALTER, then take one
// reservation through Reserve on that schema (no AutoMigrate anywhere).
func TestReserveWorksOnMigratedSchema(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "..")
	exec := func(db *gorm.DB, rel string) {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		for _, stmt := range strings.Split(string(raw), ";") {
			if trimmed := strings.TrimSpace(stmt); trimmed != "" {
				if err := db.Exec(trimmed).Error; err != nil {
					t.Fatalf("exec %s: %v", rel, err)
				}
			}
		}
	}
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "budget.db")),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	exec(db, "migrations"+string(filepath.Separator)+"sqlite"+string(filepath.Separator)+"000036_commercial_budgets.up.sql")
	exec(db, "migrations"+string(filepath.Separator)+"sqlite"+string(filepath.Separator)+"000082_commercial_reservations_owner.up.sql")

	store := NewBudgetStore(db)
	// Seed one account so Reserve passes the account guard.
	db.Exec(`INSERT INTO commercial_budget_accounts
		(tenant_id, verified_micro, unreflected_micro, held_micro, refund_locked_micro, verified_until, version)
		VALUES (42, 1000000000, 0, 0, 0, ?, 0)`, time.Now().Add(time.Hour).UTC())
	// Seed the run's task budget so Reserve passes the task-budget guard
	// (Reserve reads account AND task rows before inserting the reservation).
	db.Exec(`INSERT INTO commercial_task_budgets
		(tenant_id, run_id, limit_micro, spent_micro, held_micro, deadline, version)
		VALUES (42, 'run_migration', 1000000, 0, 0, ?, 0)`, time.Now().Add(time.Hour).UTC())
	// Seed one non-expiring lot so the earliest-expiry-first allocation pass
	// has capacity to take the hold from.
	db.Exec(`INSERT INTO commercial_budget_lots
		(tenant_id, lot_id, remaining_micro, held_micro, expires_at, issued_at)
		VALUES (42, 'lot_migration', 1000000, 0, NULL, ?)`, time.Now().UTC())
	if _, err := store.Reserve(context.Background(), domain.BudgetRequest{
		TenantID: 42, RunID: "run_migration", Key: "key_migration", Upper: 1000,
		Deadline: time.Now().Add(time.Hour).UTC(),
	}); err != nil {
		t.Fatalf("reserve on migrated schema failed (owner column missing?): %v", err)
	}
}
