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

// queryHistoryExportRowLimit caps one export at 10000 session rows; the
// constant and the shared SQL live in session_audit.go next to the other
// audit predicates.

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
// chronological order (oldest first). The SQL is the ONE shared predicate
// implementation in session_audit.go (exportSessionRows), which the Query
// History module's SessionAuditRepository.ExportRows entry lands on too, so
// the legacy listing and the export can never drift.
func (r *queryHistoryExportJobRepository) ExportSessionRows(
	ctx context.Context, q *types.SessionListQuery,
) ([]types.QueryHistoryExportRow, error) {
	return exportSessionRows(ctx, r.db, q)
}
