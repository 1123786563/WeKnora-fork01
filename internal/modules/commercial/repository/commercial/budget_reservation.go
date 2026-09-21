package commercial

import (
	"context"
	"errors"
	"fmt"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"gorm.io/gorm"
	"time"
)

// Reservation dimension: holds for keyed calls and their release/dispatch
// transitions. The fence ties a hold to the lease that created it.
// Reserve holds req.Upper credits for one keyed call of one run, atomically
// across workers and processes. In a single transaction it CAS-updates the
// account and task counters (account availability, task headroom, deadline
// validity, watermark-stable version), inserts the reservation, and
// allocates lot capacity earliest-expiry-first. Any guarded step affecting
// a row count other than 1 rolls the whole transaction back for a re-read;
// a guard that still fails on fresh data is a rejection, not a retry.
// Replaying an identical (tenant, key) is idempotent.
func (s *BudgetStore) Reserve(ctx context.Context, req domain.BudgetRequest) (domain.Reservation, error) {
	if req.TenantID == 0 || req.RunID == "" || req.Key == "" || req.Upper <= 0 {
		return domain.Reservation{}, ErrInvalidBudgetRequest
	}
	if req.Deadline.IsZero() || !req.Deadline.After(time.Now()) {
		return domain.Reservation{}, ErrInvalidBudgetRequest
	}
	var outcome *domain.Reservation
	for attempt := 0; attempt < budgetCASAttempts && outcome == nil; attempt++ {
		res, retry, err := s.tryReserve(ctx, req)
		if err != nil {
			return domain.Reservation{}, err
		}
		if !retry {
			outcome = &res
		}
	}
	if outcome == nil {
		return domain.Reservation{}, ErrBudgetContention
	}
	return *outcome, nil
}

func (s *BudgetStore) tryReserve(ctx context.Context, req domain.BudgetRequest) (domain.Reservation, bool, error) {
	now := time.Now().UTC()
	var out domain.Reservation
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Idempotent replay: an existing identical hold succeeds without
		// touching counters; the same key with different content conflicts.
		var existing ReservationRow
		err := tx.Where("tenant_id = ? AND key = ?", req.TenantID, req.Key).First(&existing).Error
		if err == nil {
			if existing.RunID == req.RunID && domain.Credits(existing.UpperMicro) == req.Upper && existing.State == domain.ReservationStateHeld {
				out = existing.toDomain()
				return nil
			}
			return ErrReservationKeyConflict
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// Account hold: guarded on the read version, projected availability,
		// and the verification window. The version guard is also the
		// watermark acceptance: any reconciliation that moves the watermark
		// bumps the version, forcing this reservation to re-read first.
		var acct BudgetAccountRow
		if err := tx.Where("tenant_id = ?", req.TenantID).First(&acct).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrBudgetAccountMissing
			}
			return err
		}
		res := tx.Exec(`UPDATE commercial_budget_accounts
			SET held_micro = held_micro + ?, version = version + 1
			WHERE tenant_id = ? AND version = ?
			  AND verified_micro - unreflected_micro - held_micro - refund_locked_micro >= ?
			  AND verified_until > ?`,
			int64(req.Upper), req.TenantID, acct.Version, int64(req.Upper), now)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return budgetAccountDenial(tx, req.TenantID, int64(req.Upper), now)
		}

		// Task budget: a child run resolves to its parent's owner row, so
		// parent and child draws share ONE budget and never duplicate it.
		var task TaskBudgetRow
		if err := tx.Where("tenant_id = ? AND run_id = ?", req.TenantID, req.RunID).First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTaskBudgetMissing
			}
			return err
		}
		ownerRun := task.RunID
		if task.RootRunID != "" {
			ownerRun = task.RootRunID
			if err := tx.Where("tenant_id = ? AND run_id = ?", req.TenantID, ownerRun).First(&task).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrTaskBudgetMissing
				}
				return err
			}
		}
		res = tx.Exec(`UPDATE commercial_task_budgets
			SET held_micro = held_micro + ?, version = version + 1
			WHERE tenant_id = ? AND run_id = ? AND version = ?
			  AND limit_micro - spent_micro - held_micro >= ?
			  AND deadline > ?`,
			int64(req.Upper), req.TenantID, ownerRun, task.Version, int64(req.Upper), now)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return budgetTaskDenial(tx, req.TenantID, ownerRun, int64(req.Upper), now)
		}

		// Reservation row with its fence. A unique-index loss here means a
		// concurrent identical key committed first: roll back and resolve
		// the replay on the next pass.
		fence, err := newReservationFence()
		if err != nil {
			return fmt.Errorf("fence: %w", err)
		}
		row := ReservationRow{
			TenantID:   req.TenantID,
			Key:        req.Key,
			RunID:      req.RunID,
			UpperMicro: int64(req.Upper),
			State:      domain.ReservationStateHeld,
			Deadline:   req.Deadline.UTC(),
			Fence:      fence,
			Version:    1,
			CreatedAt:  now,
		}
		if err := tx.Create(&row).Error; err != nil {
			// A unique-index loss (or any transient insert failure) must
			// roll back the account/task increments already applied in
			// this transaction: return the sentinel so GORM rolls the
			// whole attempt back; returning nil would COMMIT partial
			// state.
			return errBudgetCASRetry
		}

		// Lot allocation, earliest-expiry-first, same transaction.
		var lots []BudgetLotRow
		if err := tx.Raw(`SELECT * FROM commercial_budget_lots
			WHERE tenant_id = ? AND remaining_micro - held_micro > 0
			  AND (expires_at IS NULL OR expires_at > ?)
			ORDER BY expires_at ASC`, req.TenantID, now).Scan(&lots).Error; err != nil {
			return err
		}
		need := int64(req.Upper)
		for _, lot := range lots {
			if need == 0 {
				break
			}
			take := minI64(lot.RemainingMicro-lot.HeldMicro, need)
			if take <= 0 {
				continue
			}
			res := tx.Exec(`UPDATE commercial_budget_lots
				SET held_micro = held_micro + ?
				WHERE tenant_id = ? AND lot_id = ? AND remaining_micro - held_micro >= ?`,
				take, req.TenantID, lot.LotID, take)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				// A concurrent hold took this lot's capacity between the
				// read and the guarded UPDATE: return the sentinel so GORM
				// rolls back every write of this attempt (earlier lot
				// holds included); returning nil would COMMIT partial
				// state and later replay it as success.
				return errBudgetCASRetry
			}
			if err := tx.Create(&BudgetLotAllocationRow{TenantID: req.TenantID, LotID: lot.LotID, ReservationKey: req.Key, Micro: take}).Error; err != nil {
				return err
			}
			need -= take
		}
		if need > 0 {
			return ErrBudgetLotsInsufficient
		}
		out = row.toDomain()
		return nil
	})
	if err != nil {
		// The sentinel means the transaction rolled back whole after a
		// guarded miss or transient race: consume it here so the retry
		// loop re-reads fresh data — it never crosses the API surface.
		if errors.Is(err, errBudgetCASRetry) {
			return domain.Reservation{}, true, nil
		}
		return domain.Reservation{}, false, err
	}
	return out, false, nil
}

// LockRefunds locks amount credits against pending refunds in the same
// CAS-disciplined transaction a reservation uses: the guard requires the
// projected availability to still cover the lock, so refund locks and
// reservations compete for one projection and never overdraw it together.

func (s *BudgetStore) ReleaseReservation(ctx context.Context, tenantID uint64, reservationKey string) error {
	if tenantID == 0 || reservationKey == "" {
		return ErrInvalidBudgetRequest
	}
	return s.casRetry(func() (bool, error) {
		return s.tryReleaseReservation(ctx, tenantID, reservationKey)
	})
}

func (s *BudgetStore) tryReleaseReservation(ctx context.Context, tenantID uint64, reservationKey string) (bool, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var res ReservationRow
		if err := tx.Where("tenant_id = ? AND key = ?", tenantID, reservationKey).First(&res).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrReservationNotFound
			}
			return err
		}
		if res.State != domain.ReservationStateHeld {
			return ErrReservationNotHeld
		}
		r := tx.Exec(`UPDATE commercial_reservations
			SET state = ?, version = version + 1
			WHERE tenant_id = ? AND key = ? AND state = ? AND version = ?`,
			domain.ReservationStateReleased, tenantID, reservationKey,
			domain.ReservationStateHeld, res.Version)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return errBudgetCASRetry
		}
		var acct BudgetAccountRow
		if err := tx.Where("tenant_id = ?", tenantID).First(&acct).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrBudgetAccountMissing
			}
			return err
		}
		r = tx.Exec(`UPDATE commercial_budget_accounts
			SET held_micro = held_micro - ?, version = version + 1
			WHERE tenant_id = ? AND version = ? AND held_micro >= ?`,
			res.UpperMicro, tenantID, acct.Version, res.UpperMicro)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return errBudgetCASRetry
		}
		var task TaskBudgetRow
		if err := tx.Where("tenant_id = ? AND run_id = ?", tenantID, res.RunID).First(&task).Error; err != nil {
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
		r = tx.Exec(`UPDATE commercial_task_budgets
			SET held_micro = held_micro - ?, version = version + 1
			WHERE tenant_id = ? AND run_id = ? AND version = ? AND held_micro >= ?`,
			res.UpperMicro, tenantID, ownerRun, task.Version, res.UpperMicro)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return errBudgetCASRetry
		}
		var allocs []BudgetLotAllocationRow
		if err := tx.Where("tenant_id = ? AND reservation_key = ?", tenantID, reservationKey).Find(&allocs).Error; err != nil {
			return err
		}
		for _, a := range allocs {
			r := tx.Exec(`UPDATE commercial_budget_lots
				SET held_micro = held_micro - ?
				WHERE tenant_id = ? AND lot_id = ? AND held_micro >= ?`,
				a.Micro, tenantID, a.LotID, a.Micro)
			if r.Error != nil {
				return r.Error
			}
			if r.RowsAffected != 1 {
				return errBudgetCASRetry
			}
			if err := tx.Where("tenant_id = ? AND lot_id = ? AND reservation_key = ?",
				tenantID, a.LotID, reservationKey).Delete(&BudgetLotAllocationRow{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errBudgetCASRetry) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

// MarkReservationDispatched flips an unstarted reservation to dispatched
// with a version bump in one guarded UPDATE. This is the CAS complement of
// ReleaseReservation: once dispatch wins, any concurrent release attempt
// re-reads state dispatched and rejects with ErrReservationNotHeld, so a
// dispatched reservation is never released (zeroed) by cancel or expiry.
func (s *BudgetStore) MarkReservationDispatched(ctx context.Context, tenantID uint64, reservationKey string) error {
	if tenantID == 0 || reservationKey == "" {
		return ErrInvalidBudgetRequest
	}
	r := s.db.WithContext(ctx).Exec(`UPDATE commercial_reservations
		SET state = ?, version = version + 1
		WHERE tenant_id = ? AND key = ? AND state = ?`,
		domain.ReservationStateDispatched, tenantID, reservationKey, domain.ReservationStateHeld)
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return ErrReservationNotHeld
	}
	return nil
}

// ExtendTaskLimit raises one run task budget limit by extra credits in ONE
// transaction that first verifies, under the account version CAS, that the
// CURRENT funded availability (verified minus unreflected, held, and
// refund-locked) covers the extension and the verification window is still
// open: a space balance alone is never authorization. Expired quota does
// NOT extend validity: the deadline column is never touched, and an
// already-expired task budget refuses with ErrTaskBudgetExpired. Idempotency
// is the extension key: replaying the same (tenant, run, key) never
// increases the limit twice.
