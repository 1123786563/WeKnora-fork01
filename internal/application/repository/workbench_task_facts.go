package repository

import (
	"context"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"gorm.io/gorm"
)

// WorkbenchTaskFacts carries the task-level projection facts (T05 three-layer
// state) resolved for one run: title and archive lifecycle come from the
// session row; attention follows the exact rule used by list rows (a
// waiting_user run or a pending interaction on that run).
type WorkbenchTaskFacts struct {
	TaskID     string
	Title      string
	Attention  string // "none" | "required"
	ArchivedAt string // "" = not archived
}

type workbenchTaskFactsRow struct {
	SessionID        string
	Title            *string
	ArchivedAt       *time.Time
	RunStatus        string
	AttentionPending bool
}

func (workbenchTaskFactsRow) TableName() string { return "agent_runs" }

// ReadTaskFactsForRun resolves task facts anchored on one owned run. The run
// row is the anchor (the HTTP caller already proved ownership through
// GetOwnedRun); the session row is a LEFT JOIN so a drifted session cannot
// 404 an otherwise readable run — it only degrades title/archive facts to
// unknown. Tenant, owner and run id always arrive as bound parameters.
func (s *WorkbenchListStore) ReadTaskFactsForRun(ctx context.Context, tenantID uint64, ownerID, runID string) (WorkbenchTaskFacts, error) {
	if s == nil || s.db == nil || tenantID == 0 || ownerID == "" || runID == "" {
		return WorkbenchTaskFacts{}, agentruntime.ErrNotFound
	}
	var row workbenchTaskFactsRow
	err := s.db.WithContext(ctx).Table("agent_runs").
		Select("agent_runs.session_id AS session_id, agent_runs.status AS run_status, sessions.title AS title, sessions.archived_at AS archived_at, "+attentionPendingExpr+" AS attention_pending").
		Joins("LEFT JOIN sessions ON sessions.tenant_id = agent_runs.tenant_id AND sessions.id = agent_runs.session_id").
		Where("agent_runs.tenant_id = ? AND agent_runs.owner_id = ? AND agent_runs.run_id = ?", tenantID, ownerID, runID).
		Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return WorkbenchTaskFacts{}, agentruntime.ErrNotFound
		}
		return WorkbenchTaskFacts{}, err
	}
	facts := WorkbenchTaskFacts{
		TaskID:    row.SessionID,
		Attention: attentionOf(row.RunStatus, row.AttentionPending),
	}
	if row.Title != nil {
		facts.Title = strings.TrimSpace(*row.Title)
	}
	if row.ArchivedAt != nil {
		facts.ArchivedAt = row.ArchivedAt.UTC().Format(time.RFC3339Nano)
	}
	return facts, nil
}
