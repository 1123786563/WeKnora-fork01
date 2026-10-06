package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"gorm.io/gorm"
)

// ErrWorkbenchTaskNotFound marks an archive/restore request whose task the
// caller does not own in this tenant. The read model's ownership predicate
// (agent_runs.tenant_id + owner_id) is the single authority.
var ErrWorkbenchTaskNotFound = errors.New("workbench task not found for owner")

type WorkbenchTaskStateStore struct{ db *gorm.DB }

func NewWorkbenchTaskStateStore(db *gorm.DB) *WorkbenchTaskStateStore {
	return &WorkbenchTaskStateStore{db: db}
}

// SetTaskArchived archives or restores the caller's task (taskId = sessionId,
// ADR-0004). Ownership follows the read model exactly: the caller must own at
// least one run of this session inside this tenant, OR own the session itself
// (T14 Legacy Task — a run-less session archives by session ownership; soft-
// deleted sessions are no longer tasks). Identity always comes from the
// authenticated context at the handler layer, never from the request body.
func (s *WorkbenchTaskStateStore) SetTaskArchived(ctx context.Context, tenantID uint64, ownerID, taskID string, archived bool, now time.Time) error {
	if s == nil || s.db == nil || tenantID == 0 || strings.TrimSpace(ownerID) == "" || strings.TrimSpace(taskID) == "" {
		return agentruntime.ErrNotFound
	}
	taskID = strings.TrimSpace(taskID)
	var owned int64
	if err := s.db.WithContext(ctx).Table("agent_runs").
		Where("tenant_id = ? AND session_id = ? AND owner_id = ?", tenantID, taskID, ownerID).
		Limit(1).Count(&owned).Error; err != nil {
		return err
	}
	if owned == 0 {
		// T14：Legacy Task 兜底——0 run 的旧会话按 session 归属判定（全参数绑定）。
		var sessionOwned int64
		if err := s.db.WithContext(ctx).Table("sessions").
			Where("tenant_id = ? AND id = ? AND user_id = ? AND deleted_at IS NULL", tenantID, taskID, ownerID).
			Limit(1).Count(&sessionOwned).Error; err != nil {
			return err
		}
		if sessionOwned == 0 {
			return ErrWorkbenchTaskNotFound
		}
	}
	var value any
	if archived {
		value = now.UTC()
	}
	return s.db.WithContext(ctx).Table("sessions").
		Where("tenant_id = ? AND id = ?", tenantID, taskID).
		Update("archived_at", value).Error
}
