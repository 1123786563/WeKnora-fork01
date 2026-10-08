package repository

// session_audit.go is the temporary Wave 5 seam the Query History module's
// LegacyAudit adapter wraps (Wave 1, Task 7). It hosts the session-audit
// reads shared by the legacy surfaces and the module: the private
// applySessionAuditFilters predicate builder (used by the legacy paged
// listing QueryPaged and the export rows below — ONE predicate
// implementation, no copy), the admin snapshot assembly (tenant-scoped
// session row, most recent messages capped at 200, every feedback row), and
// the aggregated per-session export rows. The module's ports never see a raw
// *gorm.DB across this boundary; the manifest records the
// adapters -> internal/application/repository exception for removal in Wave 5.

import (
	"context"
	stderrors "errors"
	"strings"

	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// SessionAuditRepository serves the admin audit surfaces for the Query
// History module: the per-session snapshot and the aggregated per-session
// rows a CSV export is built from. Temporary interface — removed together
// with the legacy seam in Wave 5.
type SessionAuditRepository interface {
	// Snapshot loads one session's audit snapshot (session row, most recent
	// messages capped at 200, every feedback row), tenant-scoped: a foreign
	// session answers the same not-found error as a missing one.
	Snapshot(ctx context.Context, tenantID uint64, sessionID string) (*types.QueryHistorySnapshot, error)
	// ExportRows returns the aggregated per-session rows for one export,
	// oldest first, honoring the audit filter (user / half-open time window /
	// feedback rating), capped at the export row limit.
	ExportRows(ctx context.Context, tenantID uint64, filter domain.ExportFilter) ([]domain.ExportRow, error)
}

// sessionAuditSnapshotMessageLimit caps the messages carried by one admin
// audit snapshot; it mirrors the legacy service's
// queryHistorySnapshotMessageLimit (the service keeps its own copy until the
// legacy seam is removed in Wave 5).
const sessionAuditSnapshotMessageLimit = 200

// queryHistoryExportRowLimit caps one export at 10000 session rows. The CSV
// is a per-session summary surface (message/feedback tallies), not a dump;
// the cap keeps a runaway tenant export from pinning worker memory or
// object-storage quota while still covering real audit windows.
const queryHistoryExportRowLimit = 10000

// sessionAuditRepository implements SessionAuditRepository over the legacy
// Session/Message/Feedback repositories so the audit reads reuse the exact
// legacy row-fetch behavior.
type sessionAuditRepository struct {
	sessions interfaces.SessionRepository
	messages interfaces.MessageRepository
	feedback interfaces.FeedbackRepository
	db       *gorm.DB
}

// NewSessionAuditRepository creates the session-audit read repository.
func NewSessionAuditRepository(db *gorm.DB) SessionAuditRepository {
	return &sessionAuditRepository{
		sessions: NewSessionRepository(db),
		messages: NewMessageRepository(db),
		feedback: NewFeedbackRepository(db),
		db:       db,
	}
}

// Snapshot assembles the Admin+ audit snapshot of one session: the
// tenant-scoped session row, its most recent messages, and every feedback row
// recorded on the session. The tenant scope rides the session lookup itself
// (tenant_id = ? AND id = ?), so a foreign session is indistinguishable from
// a missing one. Owner masking for anonymized tenants is the application
// layer's call; the repository serves the raw rows.
func (r *sessionAuditRepository) Snapshot(
	ctx context.Context, tenantID uint64, sessionID string,
) (*types.QueryHistorySnapshot, error) {
	if tenantID == 0 {
		return nil, stderrors.New("workspace id is required")
	}
	if sessionID == "" {
		return nil, stderrors.New("session id is required")
	}

	session, err := r.sessions.GetByID(ctx, tenantID, sessionID)
	if err != nil {
		return nil, err
	}

	// The repository returns the newest rows first and re-sorts them
	// oldest-first, so an over-cap slice keeps its tail (the newest messages)
	// when trimmed. One row past the cap is fetched so a session exactly at
	// the cap is not reported truncated.
	messages, err := r.messages.GetRecentMessagesBySession(
		ctx, sessionID, sessionAuditSnapshotMessageLimit+1)
	if err != nil {
		return nil, err
	}
	truncated := len(messages) > sessionAuditSnapshotMessageLimit
	if truncated {
		messages = messages[len(messages)-sessionAuditSnapshotMessageLimit:]
	}

	feedback := []types.MessageFeedback{}
	rows, err := r.feedback.ListBySession(ctx, tenantID, sessionID)
	if err != nil {
		return nil, err
	}
	// Keep the non-nil guarantee so the snapshot serializes feedback as
	// [] rather than null when a session has no ratings.
	if rows != nil {
		feedback = rows
	}

	return &types.QueryHistorySnapshot{
		Session:   *session,
		Messages:  messages,
		Feedback:  feedback,
		Truncated: truncated,
	}, nil
}

// ExportRows adapts the domain export filter onto the shared audit
// predicates: the export always spans the Admin+ source=all audit view, so
// only the user / time-window / feedback filters ride along.
func (r *sessionAuditRepository) ExportRows(
	ctx context.Context, tenantID uint64, filter domain.ExportFilter,
) ([]domain.ExportRow, error) {
	if tenantID == 0 {
		return nil, stderrors.New("workspace id is required")
	}
	q := &types.SessionListQuery{
		TenantID:       tenantID,
		Source:         types.SessionListSourceAll,
		UserID:         filter.UserID,
		FeedbackRating: filter.FeedbackRating,
	}
	if !filter.StartTime.IsZero() {
		q.StartTime = filter.StartTime
	}
	if !filter.EndTime.IsZero() {
		q.EndTime = filter.EndTime
	}
	return exportSessionRows(ctx, r.db, q)
}

// applySessionAuditFilters carries the SessionListQuery predicates shared by
// the paged audit listing (QueryPaged), the legacy async CSV export entry
// (QueryHistoryExportJobRepository.ExportSessionRows), and the module's
// ExportRows: tenant + soft-delete scope, per-user ownership, the
// skill-maintenance exclusion, title keyword, the created_at window, and the
// feedback rating drill-down. Callers add their own source/agent restrictions
// and joins on top.
func applySessionAuditFilters(db *gorm.DB, q *types.SessionListQuery, isPostgres bool) *gorm.DB {
	titleLikeExpr := "LOWER(s.title) LIKE LOWER(?)"
	if isPostgres {
		titleLikeExpr = "s.title ILIKE ?"
	}
	db = db.Where("s.tenant_id = ? AND s.deleted_at IS NULL", q.TenantID)
	if q.UserID != "" {
		db = db.Where("(s.user_id = ? OR s.user_id IS NULL OR s.user_id = '')", q.UserID)
	}
	// Skill image maintenance runs in a real session so its transcript can
	// be read back, but it is not a conversation. Excluding it here rather
	// than in applySource is deliberate: a source branch only covers its
	// own bucket, and this row must be absent from all of them, including
	// the unfiltered listing.
	db = db.Where(
		"(s.description IS NULL OR s.description NOT LIKE ?)",
		types.SkillMaintenanceSessionMarker+"%",
	)
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		db = db.Where(titleLikeExpr, "%"+escapeLikeKeyword(kw)+"%")
	}
	// Audit-listing window over created_at, half-open [StartTime, EndTime).
	// Zero values leave the corresponding side of the range open.
	if !q.StartTime.IsZero() {
		db = db.Where("s.created_at >= ?", q.StartTime)
	}
	if !q.EndTime.IsZero() {
		db = db.Where("s.created_at < ?", q.EndTime)
	}
	// Feedback drill-down: a session qualifies when any of its messages
	// carries a rating row. The tenant predicate inside EXISTS keeps a
	// same-id feedback row from another tenant from ever matching.
	switch strings.ToLower(strings.TrimSpace(q.FeedbackRating)) {
	case types.FeedbackRatingLike, types.FeedbackRatingDislike:
		db = db.Where(
			"EXISTS (SELECT 1 FROM message_feedback f WHERE f.session_id = s.id AND f.tenant_id = s.tenant_id AND f.rating = ?)",
			strings.ToLower(strings.TrimSpace(q.FeedbackRating)),
		)
	}
	return db
}

// exportSessionRows returns the aggregated per-session rows of one export in
// chronological order (oldest first — an audit artifact reads naturally
// top-to-bottom). The row set reuses the audit-listing predicates via
// applySessionAuditFilters, derives the origin classification the listing
// exposes (IM platform / embed / api / web), and counts messages and
// like/dislike feedback with correlated subqueries — one SQL statement
// instead of a paging loop, so a filter change in one place cannot drift
// between the listing and the export. Both the legacy
// QueryHistoryExportJobRepository.ExportSessionRows entry and the module's
// SessionAuditRepository.ExportRows land here: ONE predicate implementation.
func exportSessionRows(
	ctx context.Context, db *gorm.DB, q *types.SessionListQuery,
) ([]types.QueryHistoryExportRow, error) {
	if q == nil {
		return nil, stderrors.New("session list query is required")
	}
	isPostgres := db.Dialector.Name() == "postgres"

	rows := make([]types.QueryHistoryExportRow, 0)
	err := applySessionAuditFilters(
		db.WithContext(ctx).Table("sessions AS s"), q, isPostgres,
	).
		// Same one-row-per-session join contract as QueryPaged; soft-deleted
		// mappings still classify the session as IM-origin.
		Joins("LEFT JOIN im_channel_sessions ics ON ics.session_id = s.id").
		Select(`s.id AS session_id, s.title, s.user_id, s.engine_type,
			s.created_at, s.updated_at,
			CASE
				WHEN ics.id IS NOT NULL THEN ics.platform
				WHEN s.description LIKE ? THEN 'embed'
				WHEN s.user_id LIKE ? OR s.user_id LIKE ? THEN 'api'
				ELSE 'web'
			END AS source,
			(SELECT COUNT(*) FROM messages m WHERE m.session_id = s.id) AS message_count,
			(SELECT COUNT(*) FROM message_feedback f
				WHERE f.session_id = s.id AND f.tenant_id = s.tenant_id AND f.rating = ?) AS like_count,
			(SELECT COUNT(*) FROM message_feedback f
				WHERE f.session_id = s.id AND f.tenant_id = s.tenant_id AND f.rating = ?) AS dislike_count`,
			types.EmbedSessionMarkerPrefix+"%",
			types.SessionOwnerAPITenantKeyPrefix+"%",
			types.SessionOwnerAPIExternalUserPrefix+"%",
			types.FeedbackRatingLike,
			types.FeedbackRatingDislike,
		).
		Order("s.created_at ASC, s.id ASC").
		Limit(queryHistoryExportRowLimit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
