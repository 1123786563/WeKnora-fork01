package commercial

import (
	"context"
	"errors"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"gorm.io/gorm"
	"time"
)

// Refund-lock dimension: credits locked against a requested refund. The
// lock keeps the refundable amount out of the spendable projection without
// spending it.
func (s *BudgetStore) LockRefunds(ctx context.Context, tenantID uint64, amount domain.Credits) error {
	if tenantID == 0 || amount <= 0 {
		return ErrInvalidBudgetRequest
	}
	return s.casRetry(func() (bool, error) {
		return s.tryLockRefunds(ctx, tenantID, amount)
	})
}

func (s *BudgetStore) tryLockRefunds(ctx context.Context, tenantID uint64, amount domain.Credits) (bool, error) {
	now := time.Now().UTC()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var acct BudgetAccountRow
		if err := tx.Where("tenant_id = ?", tenantID).First(&acct).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrBudgetAccountMissing
			}
			return err
		}
		res := tx.Exec(`UPDATE commercial_budget_accounts
			SET refund_locked_micro = refund_locked_micro + ?, version = version + 1
			WHERE tenant_id = ? AND version = ?
			  AND verified_micro - unreflected_micro - held_micro - refund_locked_micro >= ?
			  AND verified_until > ?`,
			int64(amount), tenantID, acct.Version, int64(amount), now)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return budgetAccountDenial(tx, tenantID, int64(amount), now)
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

// ApplyExternalBalance folds an independently read external balance into
// the local projection. It may advance verified_micro and the watermark —
// under version CAS and a monotonic-watermark guard — but it NEVER
// overwrites unreflected_micro, held_micro, or refund_locked_micro: those
// are local in-flight facts an external balance reader cannot see, and
// blindly refreshing them would erase unacknowledged spend.
