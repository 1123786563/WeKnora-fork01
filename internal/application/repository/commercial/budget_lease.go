package commercial

import (
	"context"
	"errors"
	"fmt"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"gorm.io/gorm"
)


// Lease dimension: worker crash recovery — takeover of a dead worker's
// reservation under a fresh fence, and commit verification so a stale
// worker can never settle a hold it no longer owns.
func (s *BudgetStore) TakeoverLease(ctx context.Context, tenantID uint64, reservationKey, newOwner string) (string, error) {
	if tenantID == 0 || reservationKey == "" || newOwner == "" {
		return "", ErrInvalidBudgetRequest
	}
	var outcome string
	for attempt := 0; attempt < budgetCASAttempts && outcome == ""; attempt++ {
		fence, retry, err := s.tryTakeoverLease(ctx, tenantID, reservationKey, newOwner)
		if err != nil {
			return "", err
		}
		if !retry {
			outcome = fence
		}
	}
	if outcome == "" {
		return "", ErrBudgetContention
	}
	return outcome, nil
}

func (s *BudgetStore) tryTakeoverLease(ctx context.Context, tenantID uint64, reservationKey, newOwner string) (string, bool, error) {
	var fence string
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
		f, err := newReservationFence()
		if err != nil {
			return fmt.Errorf("fence: %w", err)
		}
		r := tx.Exec(`UPDATE commercial_reservations
			SET owner = ?, fence = ?, version = version + 1
			WHERE tenant_id = ? AND key = ? AND state = ? AND version = ?`,
			newOwner, f, tenantID, reservationKey, domain.ReservationStateHeld, res.Version)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return errBudgetCASRetry
		}
		fence = f
		return nil
	})
	if err != nil {
		if errors.Is(err, errBudgetCASRetry) {
			return "", true, nil
		}
		return "", false, err
	}
	return fence, false, nil
}

// VerifyLeaseCommit checks that the owner/fence pair still matches the
// stored lease before a worker result may commit. A worker that lost a
// takeover fails with ErrLeaseFenceStale; a reservation no longer held
// fails with ErrReservationNotHeld.
func (s *BudgetStore) VerifyLeaseCommit(ctx context.Context, tenantID uint64, reservationKey, owner, fence string) error {
	if tenantID == 0 || reservationKey == "" || owner == "" || fence == "" {
		return ErrInvalidBudgetRequest
	}
	var res ReservationRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND key = ?", tenantID, reservationKey).First(&res).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrReservationNotFound
	}
	if err != nil {
		return err
	}
	if res.Owner != owner || res.Fence != fence {
		return ErrLeaseFenceStale
	}
	if res.State != domain.ReservationStateHeld {
		return ErrReservationNotHeld
	}
	return nil
}
