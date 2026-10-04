package commercial

import (
	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	"gorm.io/gorm"
)

// denyClosedWorkspace refuses new charge actions for a workspace whose
// closure tombstone exists (#102 / Lago 30): charging stops at closure
// time, while already-dispatched work still settles. The table-probe
// precedent is denyPricingLaggedTask — where the closure table is absent
// (a deployment without the closure flow wired) the guard stands down.
func (s *BudgetStore) denyClosedWorkspace(tx *gorm.DB, tenantID uint64) error {
	s.closureTableOnce.Do(func() {
		s.closureTableOK = tx.Migrator().HasTable("commercial_workspace_closures")
	})
	if !s.closureTableOK {
		return nil
	}
	var closed int64
	if err := tx.Raw(`SELECT COUNT(*) FROM commercial_workspace_closures WHERE tenant_id = ?`,
		tenantID).Scan(&closed).Error; err != nil {
		return err
	}
	if closed > 0 {
		return domain.ErrWorkspaceClosed
	}
	return nil
}
