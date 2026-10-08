package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
)

const maxCraftSessionPageQueryLimit = 101 // 100 visible rows plus one sentinel for next-cursor detection.

// CraftTaskListQuery is the bounded collection-query port consumed by the
// Craft session service. Pagination happens in SQL after current membership
// and explicit Task grants have filtered the row set.
type CraftTaskListQuery interface {
	ListAccessibleCraftSessions(
		ctx context.Context,
		tenantID uint64,
		userID string,
		afterTime *time.Time,
		afterSessionID string,
		limit int,
	) ([]CraftSessionSummary, error)
}

var _ CraftTaskListQuery = (*CraftAccessService)(nil)

// ListAccessibleCraftSessions returns at most limit rows for one active
// tenant membership incarnation. A rejoined user does not inherit grants
// stored against a prior membership ID.
func (s *CraftAccessService) ListAccessibleCraftSessions(
	ctx context.Context,
	tenantID uint64,
	userID string,
	afterTime *time.Time,
	afterSessionID string,
	limit int,
) ([]CraftSessionSummary, error) {
	if s == nil || s.db == nil || tenantID == 0 || userID == "" || limit <= 0 || limit > maxCraftSessionPageQueryLimit {
		return nil, craft.ErrForbidden
	}
	if afterTime == nil && afterSessionID != "" || afterTime != nil && afterSessionID == "" {
		return nil, fmt.Errorf("%w: invalid Craft list cursor", craft.ErrInvalidInput)
	}
	membershipID, err := activeMemberID(ctx, s.db, tenantID, userID)
	if err != nil {
		return nil, err
	}
	type listRow struct {
		SessionID   string    `gorm:"column:session_id"`
		WorkspaceID string    `gorm:"column:workspace_id"`
		Kind        string    `gorm:"column:kind"`
		Title       string    `gorm:"column:title"`
		UpdatedAt   time.Time `gorm:"column:updated_at"`
	}
	query := s.db.WithContext(ctx).Table("sessions AS s").
		Select("s.id AS session_id, c.kind, s.title, s.updated_at, w.id AS workspace_id").
		Joins("JOIN craft_sessions AS c ON c.session_id = s.id AND c.tenant_id = s.tenant_id").
		Joins("JOIN tenant_members AS m ON m.tenant_id = s.tenant_id AND m.user_id = ? AND m.id = ? AND m.status = ? AND m.deleted_at IS NULL", userID, membershipID, "active").
		Joins("LEFT JOIN craft_task_grants AS g ON g.tenant_id = s.tenant_id AND g.session_id = s.id AND g.user_id = ? AND g.membership_id = m.id AND g.role IN (?, ?)",
			userID, string(craft.TaskRoleViewer), string(craft.TaskRoleCollaborator)).
		Joins("LEFT JOIN craft_workspaces AS w ON w.tenant_id = c.tenant_id AND w.session_id = c.session_id").
		Where("s.tenant_id = ? AND s.deleted_at IS NULL AND (s.user_id = ? OR g.user_id IS NOT NULL)", tenantID, userID).
		Order("s.updated_at DESC, s.id DESC").
		Limit(limit)
	if afterTime != nil {
		query = query.Where("(s.updated_at < ? OR (s.updated_at = ? AND s.id < ?))",
			*afterTime, *afterTime, afterSessionID)
	}
	var rows []listRow
	if err := query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list accessible Craft sessions: %w", err)
	}
	out := make([]CraftSessionSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, CraftSessionSummary{
			SessionID: row.SessionID, WorkspaceID: row.WorkspaceID, Kind: row.Kind,
			Title: row.Title, EngineType: string(types.AgentEngineTRPC), UpdatedAt: row.UpdatedAt,
		})
	}
	return out, nil
}
