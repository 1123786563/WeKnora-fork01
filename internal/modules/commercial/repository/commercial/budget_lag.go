package commercial

import (
	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	"gorm.io/gorm"
	"time"
)

// denyPricingLaggedTask refuses new charge actions for a run whose oldest
// unconfirmed settlement is older than the pause threshold (#90 / Lago 18).
// The pause is the unconfirmed state itself, so it is reversible: once the
// settlement confirms, reserves flow again. The comparison stays in SQL so
// it is dialect-proof. The settlement-store table (owned by the service
// package's SettlementRecord) is probed once per store: where it is absent
// the guard stands down — no settlement traffic, no lag to pause on.
func (s *BudgetStore) denyPricingLaggedTask(tx *gorm.DB, tenantID uint64, runID string, now time.Time) error {
	s.lagTableOnce.Do(func() {
		s.lagTableOK = tx.Migrator().HasTable("commercial_settlement_records")
	})
	if !s.lagTableOK {
		return nil
	}
	var lagged int64
	if err := tx.Raw(`SELECT COUNT(*) FROM commercial_settlement_records
		WHERE tenant_id = ? AND run_id = ? AND state <> ? AND updated_at <= ?`,
		tenantID, runID, domain.SettlementStateConfirmed, now.Add(-domain.SettlementLagPauseThreshold)).
		Scan(&lagged).Error; err != nil {
		return err
	}
	if lagged > 0 {
		return ErrTaskPricingLagged
	}
	return nil
}
