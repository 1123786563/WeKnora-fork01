// #90 / Lago 18 — pricing-lag task pause at the budget store: a run whose
// oldest unconfirmed settlement is older than the 5-minute threshold has
// its NEW charge actions denied (the run only — other runs continue, the
// original holds stay untouched), and the pause is reversible: once the
// settlement confirms, reserves flow again. A durably suspended task
// (abnormal cost) denies until an operator intervenes.
package commercial

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/Tencent/WeKnora/internal/modules/commercial"

	"gorm.io/gorm"
)

// settlementRecordLagMirror mirrors the settlement-store row this package's
// lag guard reads via raw SQL (the row's owning type lives in the service
// package; only the table shape is needed here).
type settlementRecordLagMirror struct {
	Key       string    `gorm:"primaryKey;column:key"`
	TenantID  uint64    `gorm:"column:tenant_id;index"`
	RunID     string    `gorm:"column:run_id;not null;default:''"`
	State     string    `gorm:"column:state;not null;default:dispatched"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

func (settlementRecordLagMirror) TableName() string { return "commercial_settlement_records" }

func seedLagBudget(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(&settlementRecordLagMirror{}); err != nil {
		t.Fatal(err)
	}
	seedBudget(t, db, 1_000_000, 2_000_000)
	if err := db.Create(&TaskBudgetRow{
		TenantID: 7, RunID: "r2", LimitMicro: 2_000_000,
		Deadline: time.Now().UTC().Add(time.Hour), Version: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func lagReserve(ctx context.Context, s *BudgetStore, run, key string) error {
	_, err := s.Reserve(ctx, domain.BudgetRequest{
		TenantID: 7, RunID: run, Key: key, Upper: domain.Credits(100_000),
		Deadline: time.Now().UTC().Add(time.Hour),
	})
	return err
}

// TestReserveDeniesPricingLaggedRun pins AC1: an unconfirmed settlement
// older than the pause threshold denies the run's new reserves, while
// another run of the same tenant keeps reserving (other safe tasks
// continue) and the lagged run's existing hold is untouched.
func TestReserveDeniesPricingLaggedRun(t *testing.T) {
	s, db := testBudgetStore(t)
	seedLagBudget(t, db)
	ctx := context.Background()

	old := time.Now().UTC().Add(-6 * time.Minute)
	if err := db.Create(&settlementRecordLagMirror{
		Key: "settle:7:lagged", TenantID: 7, RunID: "r1",
		State: domain.SettlementStateDispatched, UpdatedAt: old,
	}).Error; err != nil {
		t.Fatal(err)
	}

	err := lagReserve(ctx, s, "r1", "res_lag_1")
	if !errors.Is(err, ErrTaskPricingLagged) {
		t.Fatalf("lagged run reserve: want ErrTaskPricingLagged, got %v", err)
	}
	if err := lagReserve(ctx, s, "r2", "res_lag_2"); err != nil {
		t.Fatalf("other run reserve: %v", err)
	}

	var task TaskBudgetRow
	if err := db.Where("tenant_id = ? AND run_id = ?", 7, "r1").First(&task).Error; err != nil {
		t.Fatal(err)
	}
	if task.HeldMicro != 0 {
		t.Fatalf("lagged run must keep its face untouched, held=%d", task.HeldMicro)
	}
}

// TestReserveLagIsReversible pins the prototype semantics: the pause is the
// unconfirmed state itself — a fresh settlement (below threshold) passes, and
// a confirmed settlement (any age) never blocks.
func TestReserveLagIsReversible(t *testing.T) {
	s, db := testBudgetStore(t)
	seedLagBudget(t, db)
	ctx := context.Background()

	if err := db.Create(&settlementRecordLagMirror{
		Key: "settle:7:fresh", TenantID: 7, RunID: "r1",
		State: domain.SettlementStateDispatched, UpdatedAt: time.Now().UTC().Add(-time.Minute),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := lagReserve(ctx, s, "r1", "res_rev_1"); err != nil {
		t.Fatalf("fresh settlement must not block: %v", err)
	}

	if err := db.Create(&settlementRecordLagMirror{
		Key: "settle:7:done", TenantID: 7, RunID: "r1",
		State: domain.SettlementStateConfirmed, UpdatedAt: time.Now().UTC().Add(-time.Hour),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := lagReserve(ctx, s, "r1", "res_rev_2"); err != nil {
		t.Fatalf("confirmed settlement must not block: %v", err)
	}
}

// TestSuspendTaskDeniesNewReserves pins AC4's durable face: a suspended task
// denies every new reserve (other runs unaffected), suspension is
// idempotent, and an already-held reservation replays identically instead
// of being double-held.
func TestSuspendTaskDeniesNewReserves(t *testing.T) {
	s, db := testBudgetStore(t)
	seedLagBudget(t, db)
	ctx := context.Background()

	if err := lagReserve(ctx, s, "r1", "res_susp"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := s.SuspendTask(ctx, 7, "r1", "abnormal_cost", now); err != nil {
		t.Fatal(err)
	}
	if err := s.SuspendTask(ctx, 7, "r1", "abnormal_cost", now); err != nil {
		t.Fatalf("suspend replay must be idempotent: %v", err)
	}

	if err := lagReserve(ctx, s, "r1", "res_susp_2"); !errors.Is(err, ErrTaskSuspended) {
		t.Fatalf("suspended run reserve: want ErrTaskSuspended, got %v", err)
	}
	if err := lagReserve(ctx, s, "r2", "res_susp_3"); err != nil {
		t.Fatalf("other run reserve: %v", err)
	}

	res, err := s.Reserve(ctx, domain.BudgetRequest{
		TenantID: 7, RunID: "r1", Key: "res_susp", Upper: domain.Credits(100_000),
		Deadline: time.Now().UTC().Add(time.Hour),
	})
	if err != nil || res.ID != "res_susp" {
		t.Fatalf("identical held reservation must replay, got %v %+v", err, res)
	}

	var task TaskBudgetRow
	if err := db.Where("tenant_id = ? AND run_id = ?", 7, "r1").First(&task).Error; err != nil {
		t.Fatal(err)
	}
	if task.SuspendedReason != "abnormal_cost" || task.SuspendedAt == nil {
		t.Fatalf("suspension must be durable, got %+v", task)
	}
}
