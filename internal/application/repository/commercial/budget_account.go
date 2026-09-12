package commercial

import (
	"context"
	"errors"
	"time"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"gorm.io/gorm"
)


// Account projection dimension: folding verified external balances and
// watermarks into the tenant account row through a version-CAS update.
func (s *BudgetStore) ApplyExternalBalance(ctx context.Context, tenantID uint64, verified domain.Credits, watermark string) error {
	if tenantID == 0 || verified < 0 || watermark == "" {
		return ErrInvalidBudgetRequest
	}
	return s.casRetry(func() (bool, error) {
		return s.tryApplyExternalBalance(ctx, tenantID, verified, watermark)
	})
}

func (s *BudgetStore) tryApplyExternalBalance(ctx context.Context, tenantID uint64, verified domain.Credits, watermark string) (bool, error) {
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
			if domain.Credits(acct.VerifiedMicro) == verified {
				return nil // identical replay
			}
			return ErrStaleWatermark
		}
		res := tx.Exec(`UPDATE commercial_budget_accounts
			SET verified_micro = ?, watermark = ?, version = version + 1
			WHERE tenant_id = ? AND version = ?`,
			int64(verified), watermark, tenantID, acct.Version)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
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

// budgetAccountDenial re-reads the account after a failed guard and turns
// the failure into either a hard rejection (expired verification, genuine
// insufficiency) or a retry sentinel (the guard would pass on fresh data —
// we merely lost the version race).
func budgetAccountDenial(tx *gorm.DB, tenantID uint64, upper int64, now time.Time) error {
	var fresh BudgetAccountRow
	if err := tx.Where("tenant_id = ?", tenantID).First(&fresh).Error; err != nil {
		return err
	}
	if !fresh.VerifiedUntil.After(now) {
		return ErrBudgetVerificationExpired
	}
	if fresh.VerifiedMicro-fresh.UnreflectedMicro-fresh.HeldMicro-fresh.RefundLockedMicro >= upper {
		return errBudgetCASRetry
	}
	return ErrInsufficientBudget
}

// ErrReservationNotFound rejects settlement of a reservation the store has
// no record of; ErrBudgetProtectionShortfall rejects releasing more
// unreflected protection than the account carries (a double release).
var (
	ErrReservationNotFound       = errors.New("reservation_not_found")
	ErrBudgetProtectionShortfall = errors.New("budget_protection_shortfall")
)

// SettleReservationHold is the U03 Finalize accounting, run as ONE local
// transaction with the caller's persist closure (usage fact, settlement
// record, settlement outbox event): the reservation's consumed part moves
// held → unreflected, the actually-unused part is released, the task budget
// converts its hold the same way, and lot capacity is returned. On the
// first settlement of an attempt the hold (res.Upper) is released and delta
// (the revision's charge) is protected; a later correction revision of the
// same attempt passes only its delta — external corrections and negative
// reversals adjust the protection without re-releasing the hold. The
// unreflected part stays protected until ApplyConfirmedSettlement runs under
// an explicit remote confirmation. Version-CAS guarded misses roll the WHOLE
// transaction back (persist writes included) and retry on fresh reads — the
// same errBudgetCASRetry discipline as Reserve/LockRefunds.
