package commercial

import (
	"context"
	"errors"
	"time"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)


// Task-budget dimension: owner rows, delegated child runs, and U04 limit
// extensions. A child run charges its parent's budget exactly once.
// EnsureTaskBudget registers the owner budget row of a run. Replaying the
// same run is an idempotent no-op; an existing row is never mutated here.
func (s *BudgetStore) EnsureTaskBudget(ctx context.Context, tenantID uint64, runID string, limit domain.Credits, deadline time.Time) error {
	if tenantID == 0 || runID == "" || limit < 0 || deadline.IsZero() {
		return ErrInvalidBudgetRequest
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).
		Create(&TaskBudgetRow{TenantID: tenantID, RunID: runID, LimitMicro: int64(limit), Deadline: deadline.UTC(), Version: 1}).Error
}

// AttachChildRun records that childRun charges the budget owned by
// parentRun. The child row carries a zero limit and only points at the
// parent's row: a delegated child counts against its parent's budget
// exactly once and never receives a copied budget of its own.
func (s *BudgetStore) AttachChildRun(ctx context.Context, tenantID uint64, childRun, parentRun string) error {
	if tenantID == 0 || childRun == "" || parentRun == "" || childRun == parentRun {
		return ErrInvalidBudgetRequest
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var parent TaskBudgetRow
		err := tx.Where("tenant_id = ? AND run_id = ?", tenantID, parentRun).First(&parent).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTaskBudgetMissing
			}
			return err
		}
		root := parent.RootRunID
		if root == "" {
			root = parent.RunID
		}
		var existing TaskBudgetRow
		err = tx.Where("tenant_id = ? AND run_id = ?", tenantID, childRun).First(&existing).Error
		if err == nil {
			if existing.RootRunID == root {
				return nil
			}
			return ErrTaskBudgetRootConflict
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Create(&TaskBudgetRow{TenantID: tenantID, RunID: childRun, RootRunID: root, Deadline: parent.Deadline, Version: 1}).Error
	})
}

// Reserve holds req.Upper credits for one keyed call of one run, atomically
// across workers and processes. In a single transaction it CAS-updates the
// account and task counters (account availability, task headroom, deadline
// validity, watermark-stable version), inserts the reservation, and
// allocates lot capacity earliest-expiry-first. Any guarded step affecting
// a row count other than 1 rolls the whole transaction back for a re-read;
// a guard that still fails on fresh data is a rejection, not a retry.
// Replaying an identical (tenant, key) is idempotent.

func budgetTaskDenial(tx *gorm.DB, tenantID uint64, ownerRun string, upper int64, now time.Time) error {
	var fresh TaskBudgetRow
	if err := tx.Where("tenant_id = ? AND run_id = ?", tenantID, ownerRun).First(&fresh).Error; err != nil {
		return err
	}
	if !fresh.Deadline.After(now) {
		return ErrTaskBudgetExpired
	}
	if fresh.LimitMicro-fresh.SpentMicro-fresh.HeldMicro >= upper {
		return errBudgetCASRetry
	}
	return ErrTaskBudgetExhausted
}

// ---- U04: cancel, expiry, extension and lease-recovery primitives ----
// All additions below are additive: Reserve, LockRefunds, and the
// settlement methods above are untouched; the new primitives reuse the
// same errBudgetCASRetry roll-back-whole-transaction discipline.

var (
	// ErrReservationNotHeld rejects releasing or taking over a reservation
	// that is no longer unstarted: dispatched/settling/settled reservations
	// may carry external spend and are NEVER zeroed by cancel or expiry.
	ErrReservationNotHeld = errors.New("reservation_not_held")
	// ErrLeaseFenceStale rejects a commit presented by a worker whose lease
	// was taken over: the stored owner/fence pair moved on, so the stale
	// worker result must be discarded.
	ErrLeaseFenceStale = errors.New("lease_fence_stale")
)

// TaskBudgetExtensionRow records one applied task-limit extension, unique
// per (tenant, run, key): replaying the same idempotency key never
// increases the limit a second time.
type TaskBudgetExtensionRow struct {
	TenantID   uint64    `gorm:"column:tenant_id;primaryKey;autoIncrement:false"`
	RunID      string    `gorm:"column:run_id;primaryKey"`
	Key        string    `gorm:"column:key;primaryKey"`
	ExtraMicro int64     `gorm:"column:extra_micro;not null"`
	AppliedAt  time.Time `gorm:"column:applied_at;not null"`
}

func (TaskBudgetExtensionRow) TableName() string { return "commercial_task_budget_extensions" }

// ReleaseReservation returns an unstarted reservation hold to the account,
// task budget, and lots in ONE transaction. The reservation flip is a CAS
// on BOTH state=held AND the read version: a concurrent dispatch that
// changed either makes the guarded UPDATE miss, rolls the whole attempt
// back, and the re-read then rejects with ErrReservationNotHeld. A
// reservation can therefore never be released after a concurrent dispatch
// won, and a second release of the same key rejects the same way, so there
// is no double free.

func (s *BudgetStore) ExtendTaskLimit(ctx context.Context, tenantID uint64, runID, key string, extra domain.Credits) error {
	if tenantID == 0 || runID == "" || key == "" || extra <= 0 {
		return ErrInvalidBudgetRequest
	}
	return s.casRetry(func() (bool, error) {
		return s.tryExtendTaskLimit(ctx, tenantID, runID, key, extra)
	})
}

func (s *BudgetStore) tryExtendTaskLimit(ctx context.Context, tenantID uint64, runID, key string, extra domain.Credits) (bool, error) {
	now := time.Now().UTC()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing TaskBudgetExtensionRow
		err := tx.Where("tenant_id = ? AND run_id = ? AND key = ?", tenantID, runID, key).First(&existing).Error
		if err == nil {
			return nil // idempotent replay: this key already applied once
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var acct BudgetAccountRow
		if err := tx.Where("tenant_id = ?", tenantID).First(&acct).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrBudgetAccountMissing
			}
			return err
		}
		r := tx.Exec(`UPDATE commercial_budget_accounts
			SET version = version + 1
			WHERE tenant_id = ? AND version = ?
			  AND verified_micro - unreflected_micro - held_micro - refund_locked_micro >= ?
			  AND verified_until > ?`,
			tenantID, acct.Version, int64(extra), now)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return budgetAccountDenial(tx, tenantID, int64(extra), now)
		}
		var task TaskBudgetRow
		if err := tx.Where("tenant_id = ? AND run_id = ?", tenantID, runID).First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTaskBudgetMissing
			}
			return err
		}
		ownerRun := task.RunID
		if task.RootRunID != "" {
			ownerRun = task.RootRunID
			if err := tx.Where("tenant_id = ? AND run_id = ?", tenantID, ownerRun).First(&task).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrTaskBudgetMissing
				}
				return err
			}
		}
		if !task.Deadline.After(now) {
			return ErrTaskBudgetExpired
		}
		r = tx.Exec(`UPDATE commercial_task_budgets
			SET limit_micro = limit_micro + ?, version = version + 1
			WHERE tenant_id = ? AND run_id = ? AND version = ?`,
			int64(extra), tenantID, ownerRun, task.Version)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return errBudgetCASRetry
		}
		return tx.Create(&TaskBudgetExtensionRow{
			TenantID: tenantID, RunID: runID, Key: key, ExtraMicro: int64(extra), AppliedAt: now,
		}).Error
	})
	if err != nil {
		if errors.Is(err, errBudgetCASRetry) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

// TakeoverLease moves a reservation lease to newOwner and increments the
// fence in ONE guarded transaction (state=held AND version CAS). The
// returned fence is the new single-use commit token: the previous worker
// stored owner/fence pair no longer matches, so its late commit is
// rejected by VerifyLeaseCommit. Recovery never lets two workers commit
// against one hold.
