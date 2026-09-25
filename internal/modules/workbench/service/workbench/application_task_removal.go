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
	var rows []applicationTaskRow
	if err := c.db.WithContext(ctx).
		Where(
			"tenant_id = ? AND owner_id = ? AND origin = ?",
			tenantID, ownerID, careerApplicationTaskOrigin,
		).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	removed := make([]interfaces.CareerApplicationTaskProjection, 0, len(rows))
	taskIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		removed = append(removed, interfaces.CareerApplicationTaskProjection{
			TaskID: row.TaskID, RunID: row.RunID, ApplicationID: row.ApplicationID,
		})
		taskIDs = append(taskIDs, row.TaskID)
	}
	err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("workbench_application_tasks").
			Where(
				"tenant_id = ? AND owner_id = ? AND origin = ?",
				tenantID, ownerID, careerApplicationTaskOrigin,
			).
			Delete(nil).Error; err != nil {
			return err
		}
		if err := tx.Table("agent_runs").Where("session_id IN ?", taskIDs).Delete(nil).Error; err != nil {
			return err
		}
		return tx.Table("sessions").Where("id IN ?", taskIDs).Delete(nil).Error
	})
	if err != nil {
		return nil, err
	}
	return removed, nil
}
