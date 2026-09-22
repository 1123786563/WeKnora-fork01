package repository

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"gorm.io/gorm"
)

// ErrWorkbenchCursor marks a cursor that is not decodable, targets a different
// filter/owner/tenant, or otherwise cannot continue the pagination. Handlers
// must reject the request instead of silently restarting the list.
var ErrWorkbenchCursor = errors.New("invalid workbench list cursor")

const (
	workbenchListDefaultLimit = 30
	workbenchListMaxLimit     = 100
)

// WorkbenchExecutionFilter is the caller-owned facet set. Tenant and owner are
// never part of the filter: they are separate, required arguments so every
// generated query binds them.
type WorkbenchExecutionFilter struct {
	Status  string
	AgentID string
	Cursor  string
	Limit   int
}

// WorkbenchExecutionSummary is one list row. The agent/target/workspace/space
// fields are projected from the immutable run snapshot so mobile navigation can
// rebuild the full product session destination without a second lookup.
type WorkbenchExecutionSummary struct {
	RunID        string `json:"run_id"`
	SessionID    string `json:"session_id"`
	AgentID      string `json:"agent_id,omitempty"`
	TargetID     string `json:"target_id,omitempty"`
	WorkspaceRef string `json:"workspace_ref,omitempty"`
	SpaceID      string `json:"space_id,omitempty"`
	Status       string `json:"status"`
	WaitReason   string `json:"wait_reason,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type WorkbenchExecutionPage struct {
	Items      []WorkbenchExecutionSummary `json:"items"`
	NextCursor string                      `json:"next_cursor,omitempty"`
}

// workbenchListCursor is the opaque pagination token payload. It repeats the
// scope and filter it was created under: a token replayed against a different
// tenant, owner, agent or status is rejected rather than trusted.
type workbenchListCursor struct {
	Version   int    `json:"v"`
	TenantID  uint64 `json:"tenant_id"`
	OwnerID   string `json:"owner_id"`
	AgentID   string `json:"agent_id,omitempty"`
	Status    string `json:"status,omitempty"`
	CreatedAt string `json:"created_at"`
	RunID     string `json:"run_id"`
}

type workbenchListRow struct {
	TenantID   uint64
	RunID      string
	SessionID  string
	Status     string
	WaitReason string
	Snapshot   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (workbenchListRow) TableName() string { return "agent_runs" }

// WorkbenchListStore reads the owner-scoped execution list. It is a separate
// store from AgentRunStore so the list query can grow (indexes, projections)
// without coupling admission mutations to read pagination.
type WorkbenchListStore struct{ db *gorm.DB }

func NewWorkbenchListStore(db *gorm.DB) *WorkbenchListStore {
	return &WorkbenchListStore{db: db}
}

func validWorkbenchListStatus(value string) bool {
	switch value {
	case "queued", "running", "waiting_user", "reconciling", "succeeded", "failed", "canceled":
		return true
	default:
		return false
	}
}

// snapshotAgentExpr returns the dialect-specific projection of the agent id
// stored inside the run snapshot JSON. The list never joins the IM session
// table for this: the immutable snapshot is the product-owned attribution.
func snapshotAgentExpr(db *gorm.DB) string {
	switch db.Dialector.Name() {
	case "postgres":
		return "snapshot->>'agent_id'"
	case "mysql":
		return "JSON_UNQUOTE(JSON_EXTRACT(snapshot, '$.agent_id'))"
	default:
		return "json_extract(snapshot, '$.agent_id')"
	}
}

// listOrderExpr and the keyset predicate normalize created_at per dialect.
// SQLite stores datetimes as driver-formatted text (whose zone offset can
// differ between writers), so ordering and comparison go through julianday;
// PostgreSQL TIMESTAMPTZ and MySQL DATETIME compare natively.
func listOrderExpr(db *gorm.DB) string {
	if db.Dialector.Name() == "sqlite" {
		return "julianday(created_at) DESC, run_id DESC"
	}
	return "created_at DESC, run_id DESC"
}

func listKeysetPredicate(db *gorm.DB, anchor time.Time, runID string) string {
	if db.Dialector.Name() == "sqlite" {
		return "(julianday(created_at) < julianday(?) OR (julianday(created_at) = julianday(?) AND run_id < ?))"
	}
	return "(created_at < ? OR (created_at = ? AND run_id < ?))"
}

func encodeWorkbenchListCursor(cursor workbenchListCursor) string {
	raw, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeWorkbenchListCursor(raw string) (workbenchListCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return workbenchListCursor{}, ErrWorkbenchCursor
	}
	var cursor workbenchListCursor
	if err := json.Unmarshal(decoded, &cursor); err != nil {
		return workbenchListCursor{}, ErrWorkbenchCursor
	}
	if cursor.Version != 1 || cursor.TenantID == 0 || strings.TrimSpace(cursor.OwnerID) == "" ||
		strings.TrimSpace(cursor.RunID) == "" || strings.TrimSpace(cursor.CreatedAt) == "" {
		return workbenchListCursor{}, ErrWorkbenchCursor
	}
	if _, err := time.Parse(time.RFC3339Nano, cursor.CreatedAt); err != nil {
		return workbenchListCursor{}, ErrWorkbenchCursor
	}
	return cursor, nil
}

// ListOwnedExecutions returns the authenticated owner's executions, newest
// first, with a stable (created_at, run_id) ordering and an opaque cursor that
// is bound to the exact tenant/owner/filter it was issued under. Concurrent
// inserts newer than the cursor cannot shift the keyset window, so pages never
// repeat or drop rows.
func (s *WorkbenchListStore) ListOwnedExecutions(ctx context.Context, tenantID uint64, ownerID string, filter WorkbenchExecutionFilter) (WorkbenchExecutionPage, error) {
	if s == nil || s.db == nil || tenantID == 0 || strings.TrimSpace(ownerID) == "" {
		return WorkbenchExecutionPage{}, agentruntime.ErrNotFound
	}
	filter.Status = strings.TrimSpace(filter.Status)
	filter.AgentID = strings.TrimSpace(filter.AgentID)
	if filter.Status != "" && !validWorkbenchListStatus(filter.Status) {
		return WorkbenchExecutionPage{}, fmt.Errorf("%w: unknown status %q", ErrWorkbenchCursor, filter.Status)
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = workbenchListDefaultLimit
	}
	if limit > workbenchListMaxLimit {
		limit = workbenchListMaxLimit
	}

	query := s.db.WithContext(ctx).Table("agent_runs").
		Select("tenant_id", "run_id", "session_id", "status", "wait_reason", "snapshot", "created_at", "updated_at").
		Where("tenant_id = ? AND owner_id = ?", tenantID, ownerID)
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.AgentID != "" {
		query = query.Where(snapshotAgentExpr(s.db)+" = ?", filter.AgentID)
	}
	if strings.TrimSpace(filter.Cursor) != "" {
		cursor, err := decodeWorkbenchListCursor(filter.Cursor)
		if err != nil {
			return WorkbenchExecutionPage{}, err
		}
		if cursor.TenantID != tenantID || cursor.OwnerID != ownerID ||
			cursor.AgentID != filter.AgentID || cursor.Status != filter.Status {
			return WorkbenchExecutionPage{}, fmt.Errorf("%w: cursor does not match the active filter", ErrWorkbenchCursor)
		}
		anchor, err := time.Parse(time.RFC3339Nano, cursor.CreatedAt)
		if err != nil {
			return WorkbenchExecutionPage{}, ErrWorkbenchCursor
		}
		query = query.Where(listKeysetPredicate(s.db, anchor, cursor.RunID), anchor, anchor, cursor.RunID)
	}

	var rows []workbenchListRow
	if err := query.Order(listOrderExpr(s.db)).Limit(limit + 1).Find(&rows).Error; err != nil {
		return WorkbenchExecutionPage{}, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	page := WorkbenchExecutionPage{Items: make([]WorkbenchExecutionSummary, 0, len(rows))}
	for _, row := range rows {
		page.Items = append(page.Items, workbenchSummaryFromRow(row))
	}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		page.NextCursor = encodeWorkbenchListCursor(workbenchListCursor{
			Version: 1, TenantID: tenantID, OwnerID: ownerID,
			AgentID: filter.AgentID, Status: filter.Status,
			CreatedAt: last.CreatedAt.UTC().Format(time.RFC3339Nano), RunID: last.RunID,
		})
	}
	return page, nil
}

// workbenchSummaryFromRow projects the durable run row plus its immutable
// snapshot. A snapshot that fails to parse contributes no navigation metadata;
// the row itself (status, session, timestamps) stays authoritative.
func workbenchSummaryFromRow(row workbenchListRow) WorkbenchExecutionSummary {
	summary := WorkbenchExecutionSummary{
		RunID: row.RunID, SessionID: row.SessionID,
		Status: row.Status, WaitReason: row.WaitReason,
		CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt: row.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if strings.TrimSpace(row.Snapshot) == "" {
		return summary
	}
	var snapshot struct {
		AgentID      string `json:"agent_id"`
		TargetID     string `json:"target_id"`
		WorkspaceRef string `json:"workspace_ref"`
		SpaceID      string `json:"space_id"`
	}
	if json.Unmarshal([]byte(row.Snapshot), &snapshot) != nil {
		return summary
	}
	summary.AgentID = strings.TrimSpace(snapshot.AgentID)
	summary.TargetID = strings.TrimSpace(snapshot.TargetID)
	summary.WorkspaceRef = strings.TrimSpace(snapshot.WorkspaceRef)
	summary.SpaceID = strings.TrimSpace(snapshot.SpaceID)
	return summary
}
