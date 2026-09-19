package repository

import (
	"context"
	stderrors "errors"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// ErrQueryHistoryExportJobNotFound answers a GetByID whose id does not exist
// in the requesting tenant. Absence and cross-tenant probes deliberately
// produce the same error so a job id cannot be probed across workspaces.
var ErrQueryHistoryExportJobNotFound = apperrors.NewNotFoundError("query history export job not found")

// queryHistoryExportRowLimit caps one export at 10000 session rows. The CSV
// is a per-session summary surface (message/feedback tallies), not a dump;
// the cap keeps a runaway tenant export from pinning worker memory or
// object-storage quota while still covering real audit windows.
const queryHistoryExportRowLimit = 10000

// queryHistoryExportJobRepository implements
// interfaces.QueryHistoryExportJobRepository.
type queryHistoryExportJobRepository struct {
	db *gorm.DB
}

// NewQueryHistoryExportJobRepository creates the async query-history export
// job repository.
func NewQueryHistoryExportJobRepository(db *gorm.DB) interfaces.QueryHistoryExportJobRepository {
	return &queryHistoryExportJobRepository{db: db}
}

// Create inserts a new export job, defaulting the lifecycle status to pending.
func (r *queryHistoryExportJobRepository) Create(
	ctx context.Context, job *types.QueryHistoryExportJob,
) error {
	if job.Status == "" {
		job.Status = types.QueryHistoryExportPending
	}
	return r.db.WithContext(ctx).Create(job).Error
}

// GetByID loads one export job scoped to the tenant.
func (r *queryHistoryExportJobRepository) GetByID(
	ctx context.Context, tenantID uint64, id uint64,
) (*types.QueryHistoryExportJob, error) {
	var job types.QueryHistoryExportJob
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&job).Error
	if err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrQueryHistoryExportJobNotFound
		}
		return nil, err
	}
	return &job, nil
}

// UpdateStatus transitions the job lifecycle. file_path and error_message are
// both rewritten on every call so a retry that flips a failed job back to
// running clears the stale error message, and a done job never keeps a path
// from an earlier attempt.
func (r *queryHistoryExportJobRepository) UpdateStatus(
	ctx context.Context, id uint64, status, filePath, errMsg string,
) error {
	return r.db.WithContext(ctx).
		Model(&types.QueryHistoryExportJob{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        status,
			"file_path":     filePath,
			"error_message": errMsg,
		}).Error
}

// ExportSessionRows returns the aggregated per-session rows of one export in
// chronological order (oldest first — an audit artifact reads naturally
// top-to-bottom). The row set reuses the audit-listing predicates via
// applySessionAuditFilters, derives the origin classification the listing
// exposes (IM platform / embed / api / web), and counts messages and
// like/dislike feedback with correlated subqueries — one SQL statement
// instead of a paging loop, so a filter change in one place cannot drift
// between the listing and the export.
func (r *queryHistoryExportJobRepository) ExportSessionRows(
	ctx context.Context, q *types.SessionListQuery,
) ([]types.QueryHistoryExportRow, error) {
	if q == nil {
		return nil, stderrors.New("session list query is required")
	}
	isPostgres := r.db.Dialector.Name() == "postgres"

	rows := make([]types.QueryHistoryExportRow, 0)
	err := applySessionAuditFilters(
		r.db.WithContext(ctx).Table("sessions AS s"), q, isPostgres,
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
