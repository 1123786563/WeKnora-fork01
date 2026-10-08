package workbench

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

var _ interfaces.CareerApplicationTaskProjectionRemover = (*ApplicationTaskCoordinator)(nil)

// RemoveCareerApplicationTaskProjections deletes every durable application
// task this owner created through Career: the mapping rows, their agent runs,
// and the backing sessions. The call is idempotent — a retry after a partial
// failure simply removes whatever is still there. Career reaches this only
// through the interfaces port; it never writes these tables itself.
func (c *ApplicationTaskCoordinator) RemoveCareerApplicationTaskProjections(
	ctx context.Context,
	tenantID uint64,
	ownerID string,
) ([]interfaces.CareerApplicationTaskProjection, error) {
	if tenantID == 0 || ownerID == "" {
		return nil, nil
	}
	var removed []interfaces.CareerApplicationTaskProjection
	err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// The mapping table is the single source of truth, read INSIDE the
		// deletion transaction. The pre-fix shape read it outside and then
		// broad-deleted mappings while runs/sessions were deleted by the
		// stale snapshot: a task committed in between lost its mapping but
		// kept its orphaned run/session, and the retry (finding no mapping)
		// could never clean it up.
		var rows []applicationTaskRow
		if err := tx.
			Where(
				"tenant_id = ? AND owner_id = ? AND origin = ?",
				tenantID, ownerID, careerApplicationTaskOrigin,
			).
			Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		removed = make([]interfaces.CareerApplicationTaskProjection, 0, len(rows))
		for _, row := range rows {
			removed = append(removed, interfaces.CareerApplicationTaskProjection{
				TaskID: row.TaskID, RunID: row.RunID, ApplicationID: row.ApplicationID,
			})
		}
		// Every delete below is driven by a subquery over the mapping table,
		// so all three statements agree on one set inside this transaction.
		// Sessions cascade agent_runs and mappings on PostgreSQL; the
		// explicit statements keep the same outcome where FKs are not
		// enforced (SQLite without the FK pragma).
		if err := tx.Exec(
			`DELETE FROM sessions WHERE tenant_id = ? AND id IN (
				SELECT task_id FROM workbench_application_tasks
				WHERE tenant_id = ? AND owner_id = ? AND origin = ?)`,
			tenantID, tenantID, ownerID, careerApplicationTaskOrigin,
		).Error; err != nil {
			return err
		}
		if err := tx.Exec(
			`DELETE FROM agent_runs WHERE tenant_id = ? AND session_id IN (
				SELECT task_id FROM workbench_application_tasks
				WHERE tenant_id = ? AND owner_id = ? AND origin = ?)`,
			tenantID, tenantID, ownerID, careerApplicationTaskOrigin,
		).Error; err != nil {
			return err
		}
		return tx.Table("workbench_application_tasks").
			Where(
				"tenant_id = ? AND owner_id = ? AND origin = ?",
				tenantID, ownerID, careerApplicationTaskOrigin,
			).
			Delete(nil).Error
	})
	if err != nil {
		return nil, err
	}
	return removed, nil
}
