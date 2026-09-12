package commercial

import (
	"context"
	"errors"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"gorm.io/gorm"
)

// Settlement dimension: turning reservation holds into confirmed spend
// (or releasing them), and folding confirmed settlements into the account
// with watermark advancement.
func (s *BudgetStore) SettleReservationHold(ctx context.Context, tenantID uint64, reservationKey string, delta domain.Credits, persist func(tx *gorm.DB) error) error {
	if tenantID == 0 || reservationKey == "" {
		return ErrInvalidBudgetRequest
	}
	return s.casRetry(func() (bool, error) {
		return s.trySettleReservationHold(ctx, tenantID, reservationKey, delta, persist)
	})
}

func (s *BudgetStore) trySettleReservationHold(ctx context.Context, tenantID uint64, reservationKey string, delta domain.Credits, persist func(tx *gorm.DB) error) (bool, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var res ReservationRow
		if err := tx.Where("tenant_id = ? AND key = ?", tenantID, reservationKey).First(&res).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrReservationNotFound
			}
			return err
		}
		first := res.State == domain.ReservationStateHeld
		switch res.State {
		case domain.ReservationStateHeld, domain.ReservationStateSettled:
		default:
			return ErrReservationKeyConflict
		}
		if res.State == domain.ReservationStateSettled && first {
			return ErrReservationKeyConflict // unreachable; keeps the invariant explicit
		}
		// Caller's durable writes (fact, settlement record, outbox event)
		// commit atomically with the accounting below — or roll back with it.
		if persist != nil {
			if err := persist(tx); err != nil {
				return err
			}
		}

		var acct BudgetAccountRow
		if err := tx.Where("tenant_id = ?", tenantID).First(&acct).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrBudgetAccountMissing
			}
			return err
		}
		if first {
			// Consumed part held → unreflected; unused part released. Net
			// availability only grows (upper ≥ delta), so the guard is the
			// version CAS plus hold coverage; a miss is a retry, never a
			// partial commit.
			r := tx.Exec(`UPDATE commercial_budget_accounts
				SET held_micro = held_micro - ?, unreflected_micro = unreflected_micro + ?, version = version + 1
				WHERE tenant_id = ? AND version = ? AND held_micro >= ? AND unreflected_micro + ? >= 0`,
				res.UpperMicro, int64(delta), tenantID, acct.Version, res.UpperMicro, int64(delta))
			if r.Error != nil {
				return r.Error
			}
			if r.RowsAffected != 1 {
				return errBudgetCASRetry
			}
		} else if delta != 0 {
			r := tx.Exec(`UPDATE commercial_budget_accounts
				SET unreflected_micro = unreflected_micro + ?, version = version + 1
				WHERE tenant_id = ? AND version = ? AND unreflected_micro + ? >= 0`,
				int64(delta), tenantID, acct.Version, int64(delta))
			if r.Error != nil {
				return r.Error
			}
			if r.RowsAffected != 1 {
				return errBudgetCASRetry
			}
		}

		// Task budget resolves to the owner row exactly as Reserve does, so
		// the converted spend counts against its parent budget once.
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
		if first {
			r := tx.Exec(`UPDATE commercial_task_budgets
				SET held_micro = held_micro - ?, spent_micro = spent_micro + ?, version = version + 1
				WHERE tenant_id = ? AND run_id = ? AND version = ? AND held_micro >= ? AND spent_micro + ? >= 0`,
				res.UpperMicro, int64(delta), tenantID, ownerRun, task.Version, res.UpperMicro, int64(delta))
			if r.Error != nil {
				return r.Error
			}
			if r.RowsAffected != 1 {
				return errBudgetCASRetry
			}
		} else if delta != 0 {
			r := tx.Exec(`UPDATE commercial_task_budgets
				SET spent_micro = spent_micro + ?, version = version + 1
				WHERE tenant_id = ? AND run_id = ? AND version = ? AND spent_micro + ? >= 0`,
				int64(delta), tenantID, ownerRun, task.Version, int64(delta))
			if r.Error != nil {
				return r.Error
			}
			if r.RowsAffected != 1 {
				return errBudgetCASRetry
			}
		}

		if first {
			// Lot capacity: every allocation's hold is returned; the
			// consumed part reduces lot remaining earliest-expiry-first, and
			// the allocation rows are deleted. Corrections do not touch
			// lots — the hold was already returned by the first settlement.
			var allocs []BudgetLotAllocationRow
			if err := tx.Raw(`SELECT a.* FROM commercial_budget_lot_allocations a
				JOIN commercial_budget_lots l ON l.tenant_id = a.tenant_id AND l.lot_id = a.lot_id
				WHERE a.tenant_id = ? AND a.reservation_key = ?
				ORDER BY l.expires_at ASC`, tenantID, reservationKey).Scan(&allocs).Error; err != nil {
				return err
			}
			consume := int64(delta)
			for _, a := range allocs {
				take := minI64(a.Micro, consume)
				r := tx.Exec(`UPDATE commercial_budget_lots
					SET held_micro = held_micro - ?, remaining_micro = remaining_micro - ?
					WHERE tenant_id = ? AND lot_id = ? AND held_micro >= ? AND remaining_micro >= ?`,
					a.Micro, take, tenantID, a.LotID, a.Micro, take)
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
				consume -= take
			}
			r := tx.Exec(`UPDATE commercial_reservations
				SET state = ?, version = version + 1
				WHERE tenant_id = ? AND key = ? AND state = ?`,
				domain.ReservationStateSettled, tenantID, reservationKey, domain.ReservationStateHeld)
			if r.Error != nil {
				return r.Error
			}
			if r.RowsAffected != 1 {
				return errBudgetCASRetry
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

// ApplyConfirmedSettlement releases, in ONE transaction with the caller's
// persist closure (the settlement record flipping to confirmed), the
// protection covered by an explicitly confirmed settlement: the watermark
// advances monotonically, the protected (unreflected) amount is removed, and
// verified drops by the same amount — the confirmation's correlation
// evidence proves the provider already consumed it, so releasing protection
// without dropping verified would transiently re-admit spent credits. An
// older watermark never rewinds anything (ErrStaleWatermark); an equal
// watermark is an idempotent replay.
func (s *BudgetStore) ApplyConfirmedSettlement(ctx context.Context, tenantID uint64, amount domain.Credits, watermark string, persist func(tx *gorm.DB) error) error {
	if tenantID == 0 || amount < 0 || watermark == "" {
		return ErrInvalidBudgetRequest
	}
	return s.casRetry(func() (bool, error) {
		return s.tryApplyConfirmedSettlement(ctx, tenantID, amount, watermark, persist)
	})
}

func (s *BudgetStore) tryApplyConfirmedSettlement(ctx context.Context, tenantID uint64, amount domain.Credits, watermark string, persist func(tx *gorm.DB) error) (bool, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var acct BudgetAccountRow
		if err := tx.Where("tenant_id = ?", tenantID).First(&acct).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrBudgetAccountMissing
			}
			return err
		}
		if watermark < acct.Watermark {
			return ErrStaleWatermark
		}
		if watermark == acct.Watermark {
			return nil // identical replay: the protection is already released
		}
		if int64(amount) > acct.UnreflectedMicro {
			return ErrBudgetProtectionShortfall
		}
		if persist != nil {
			if err := persist(tx); err != nil {
				return err
			}
		}
		r := tx.Exec(`UPDATE commercial_budget_accounts
			SET unreflected_micro = unreflected_micro - ?, verified_micro = verified_micro - ?, watermark = ?, version = version + 1
			WHERE tenant_id = ? AND version = ? AND unreflected_micro >= ?`,
			int64(amount), int64(amount), watermark, tenantID, acct.Version, int64(amount))
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return errBudgetCASRetry
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

// budgetTaskDenial is the task-budget counterpart of budgetAccountDenial.
