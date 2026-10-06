package repository

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"gorm.io/gorm"
)

// T14（Issue #44）：Legacy Task 读模型。从未有过 agent_runs 行的旧 Session 以
// 同一身份（taskId = sessionId，ADR-0004）投影为 Legacy Task。本列表与
// ListOwnedExecutions 把全部会话二分区：有 Run 的进执行列表、无 Run 的进
// legacy 列表，同一 taskId 两处互斥。只投影历史能证明的事实：标题、归档
// 时间、更新时间；attention 恒为 "none"（waiting_user 与 pending interaction
// 都是 Run 作用域事实，无 Run 即无可证明的关注来源）；绝不伪造
// Run/Grant/预算/审批/Agent Version 字段。

// WorkbenchLegacyFilter is the caller-owned facet set. Tenant and owner are
// never part of the filter: they are separate required arguments so every
// generated query binds them, mirroring WorkbenchExecutionFilter.
type WorkbenchLegacyFilter struct {
	Query        string // 任务标题子串搜索（忽略大小写；全空白归一为无搜索）
	ArchivedOnly bool   // false（默认）=仅未归档；true=仅已归档
	Cursor       string
	Limit        int
}

// WorkbenchLegacyTaskSummary is one legacy task row: facts only. Kind is the
// explicit legacy marker clients gate Run-scoped capabilities on; Attention is
// always "none" because no provable attention source exists without a Run.
type WorkbenchLegacyTaskSummary struct {
	TaskID     string `json:"task_id"`
	Title      string `json:"title,omitempty"`
	Attention  string `json:"attention"`
	ArchivedAt string `json:"archived_at,omitempty"`
	UpdatedAt  string `json:"updated_at"`
	Kind       string `json:"kind"`
}

type WorkbenchLegacyPage struct {
	Items      []WorkbenchLegacyTaskSummary `json:"items"`
	NextCursor string                       `json:"next_cursor,omitempty"`
}

// workbenchLegacyCursor is the opaque pagination token. The "legacy" kind
// discriminator makes a run-list cursor replayed here (and vice versa) fail
// validation instead of silently continuing the wrong row space.
type workbenchLegacyCursor struct {
	Version      int    `json:"v"`
	Kind         string `json:"kind"`
	TenantID     uint64 `json:"tenant_id"`
	OwnerID      string `json:"owner_id"`
	Query        string `json:"q,omitempty"`
	ArchivedOnly bool   `json:"archived_only,omitempty"`
	UpdatedAt    string `json:"updated_at"`
	TaskID       string `json:"task_id"`
}

type workbenchLegacyRow struct {
	TaskID     string
	Title      string
	ArchivedAt *time.Time
	UpdatedAt  time.Time
}

func (workbenchLegacyRow) TableName() string { return "sessions" }

type WorkbenchLegacyListStore struct{ db *gorm.DB }

func NewWorkbenchLegacyListStore(db *gorm.DB) *WorkbenchLegacyListStore {
	return &WorkbenchLegacyListStore{db: db}
}

// A session is legacy exactly when no agent_runs row ever referenced it. The
// constant NOT EXISTS anti-join carries no external input; every caller-owned
// value stays behind bound parameters.
const legacyNoRunsExpr = `NOT EXISTS (SELECT 1 FROM agent_runs ar
	WHERE ar.tenant_id = sessions.tenant_id AND ar.session_id = sessions.id)`

// legacyOrderExpr and legacyKeysetPredicate normalize updated_at per dialect
// (same rationale as the run list: SQLite stores driver-formatted text).
func legacyOrderExpr(db *gorm.DB) string {
	if db.Dialector.Name() == "sqlite" {
		return "julianday(sessions.updated_at) DESC, sessions.id DESC"
	}
	return "sessions.updated_at DESC, sessions.id DESC"
}

func legacyKeysetPredicate(db *gorm.DB, anchor time.Time, taskID string) string {
	if db.Dialector.Name() == "sqlite" {
		return "(julianday(sessions.updated_at) < julianday(?) OR (julianday(sessions.updated_at) = julianday(?) AND sessions.id < ?))"
	}
	return "(sessions.updated_at < ? OR (sessions.updated_at = ? AND sessions.id < ?))"
}

func encodeWorkbenchLegacyCursor(cursor workbenchLegacyCursor) string {
	raw, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeWorkbenchLegacyCursor(raw string) (workbenchLegacyCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return workbenchLegacyCursor{}, ErrWorkbenchCursor
	}
	var cursor workbenchLegacyCursor
	if err := json.Unmarshal(decoded, &cursor); err != nil {
		return workbenchLegacyCursor{}, ErrWorkbenchCursor
	}
	if cursor.Version != 1 || cursor.Kind != "legacy" || cursor.TenantID == 0 ||
		strings.TrimSpace(cursor.OwnerID) == "" || strings.TrimSpace(cursor.TaskID) == "" ||
		strings.TrimSpace(cursor.UpdatedAt) == "" {
		return workbenchLegacyCursor{}, ErrWorkbenchCursor
	}
	if _, err := time.Parse(time.RFC3339Nano, cursor.UpdatedAt); err != nil {
		return workbenchLegacyCursor{}, ErrWorkbenchCursor
	}
	return cursor, nil
}

func legacySummaryFromRow(row workbenchLegacyRow) WorkbenchLegacyTaskSummary {
	summary := WorkbenchLegacyTaskSummary{
		TaskID:    row.TaskID,
		Title:     strings.TrimSpace(row.Title),
		Attention: "none",
		UpdatedAt: row.UpdatedAt.UTC().Format(time.RFC3339Nano),
		Kind:      "legacy",
	}
	if row.ArchivedAt != nil {
		summary.ArchivedAt = row.ArchivedAt.UTC().Format(time.RFC3339Nano)
	}
	return summary
}

// ListOwnedLegacyTasks returns the owner's legacy tasks (sessions without any
// agent_runs row), newest first, with a stable (updated_at, id) keyset cursor
// bound to the exact tenant/owner/filter it was issued under.
func (s *WorkbenchLegacyListStore) ListOwnedLegacyTasks(ctx context.Context, tenantID uint64, ownerID string, filter WorkbenchLegacyFilter) (WorkbenchLegacyPage, error) {
	if s == nil || s.db == nil || tenantID == 0 || strings.TrimSpace(ownerID) == "" {
		return WorkbenchLegacyPage{}, agentruntime.ErrNotFound
	}
	filter.Query = normalizeWorkbenchSearch(filter.Query)
	limit := filter.Limit
	if limit <= 0 {
		limit = workbenchListDefaultLimit
	}
	if limit > workbenchListMaxLimit {
		limit = workbenchListMaxLimit
	}

	query := s.db.WithContext(ctx).Table("sessions").
		Select("sessions.id AS task_id, sessions.title, sessions.archived_at, sessions.updated_at").
		Where("sessions.tenant_id = ? AND sessions.user_id = ? AND sessions.deleted_at IS NULL", tenantID, ownerID).
		Where(legacyNoRunsExpr)
	if filter.Query != "" {
		query = query.Where(searchPredicate(s.db), "%"+likeEscaped(filter.Query)+"%")
	}
	if filter.ArchivedOnly {
		query = query.Where("sessions.archived_at IS NOT NULL")
	} else {
		query = query.Where("sessions.archived_at IS NULL")
	}
	if strings.TrimSpace(filter.Cursor) != "" {
		cursor, err := decodeWorkbenchLegacyCursor(filter.Cursor)
		if err != nil {
			return WorkbenchLegacyPage{}, err
		}
		if cursor.TenantID != tenantID || cursor.OwnerID != ownerID ||
			cursor.Query != filter.Query || cursor.ArchivedOnly != filter.ArchivedOnly {
			return WorkbenchLegacyPage{}, fmt.Errorf("%w: cursor does not match the active filter", ErrWorkbenchCursor)
		}
		anchor, err := time.Parse(time.RFC3339Nano, cursor.UpdatedAt)
		if err != nil {
			return WorkbenchLegacyPage{}, ErrWorkbenchCursor
		}
		query = query.Where(legacyKeysetPredicate(s.db, anchor, cursor.TaskID), anchor, anchor, cursor.TaskID)
	}

	var rows []workbenchLegacyRow
	if err := query.Order(legacyOrderExpr(s.db)).Limit(limit + 1).Find(&rows).Error; err != nil {
		return WorkbenchLegacyPage{}, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	page := WorkbenchLegacyPage{Items: make([]WorkbenchLegacyTaskSummary, 0, len(rows))}
	for _, row := range rows {
		page.Items = append(page.Items, legacySummaryFromRow(row))
	}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		page.NextCursor = encodeWorkbenchLegacyCursor(workbenchLegacyCursor{
			Version: 1, Kind: "legacy", TenantID: tenantID, OwnerID: ownerID,
			Query: filter.Query, ArchivedOnly: filter.ArchivedOnly,
			UpdatedAt: last.UpdatedAt.UTC().Format(time.RFC3339Nano), TaskID: last.TaskID,
		})
	}
	return page, nil
}
