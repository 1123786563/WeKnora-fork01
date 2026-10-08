package adapters

import (
	"context"
	stderrors "errors"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/ports"
	"gorm.io/gorm"
)

// GormExportJobStore satisfies ports.ExportJobStore over GORM. It mirrors the
// legacy queryHistoryExportJobRepository SQL (same table, same tenant-scoped
// Get, same always-rewrite-both-columns transitions) with one deliberate
// hardening the port mandates: UpdateStatus is tenant-scoped too, so a
// foreign workspace's transition can never land.
//
// domain.ExportJob is the Wave 1 alias of types.QueryHistoryExportJob, so the
// statements operate on the very same rows the legacy repository serves.
type GormExportJobStore struct {
	db *gorm.DB
}

// NewGormExportJobStore builds the export-job store adapter.
func NewGormExportJobStore(db *gorm.DB) *GormExportJobStore {
	return &GormExportJobStore{db: db}
}

// compile-time port conformance.
var _ ports.ExportJobStore = (*GormExportJobStore)(nil)

// Create inserts a new export job, defaulting the lifecycle status to pending.
func (s *GormExportJobStore) Create(ctx context.Context, job *domain.ExportJob) error {
	if job.Status == "" {
		job.Status = domain.ExportPending
	}
	return s.db.WithContext(ctx).Create(job).Error
}

// Get loads one export job scoped to the tenant. A missing id and a
// cross-tenant probe answer the SAME legacy sentinel — the existing 404
// AppError (repository.ErrQueryHistoryExportJobNotFound) — so a job id cannot
// be probed across workspaces and callers map both to 404.
func (s *GormExportJobStore) Get(
	ctx context.Context, tenantID uint64, jobID uint64,
) (*domain.ExportJob, error) {
	var job domain.ExportJob
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, jobID).
		First(&job).Error
	if err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrQueryHistoryExportJobNotFound
		}
		return nil, err
	}
	return &job, nil
}

// UpdateStatus transitions the job lifecycle, scoped to the tenant.
// file_path and error_message are both rewritten on every call so a retry
// that flips a failed job back to running clears the stale error message, and
// a done job never keeps a path from an earlier attempt. Like the legacy
// repository, a zero-row update (foreign tenant, or the job row vanished
// between the worker's read and write) is not an error: nothing was leaked
// and nothing was clobbered.
func (s *GormExportJobStore) UpdateStatus(
	ctx context.Context, tenantID uint64, jobID uint64,
	status domain.ExportStatus, filePath string, errMsg string,
) error {
	return s.db.WithContext(ctx).
		Model(&domain.ExportJob{}).
		Where("tenant_id = ? AND id = ?", tenantID, jobID).
		Updates(map[string]interface{}{
			"status":        status,
			"file_path":     filePath,
			"error_message": errMsg,
		}).Error
}
