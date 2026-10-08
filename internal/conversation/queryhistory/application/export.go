package application

// The async query-history CSV export submission and download surfaces (Wave
// 1, Task 6), moved from the legacy
// internal/application/service/query_history_export.go: Start admits a job
// and enqueues the worker task through the TaskQueue port (the adapter owns
// the task type, queue, retry, and timeout options), Job serves the
// tenant-scoped status read, and Open streams a completed archive. The worker
// body itself lives in worker.go. Like the rest of the package this file
// depends only on the module's domain and ports plus the shared AppError
// vocabulary — never on a task framework, storage driver, or legacy
// repository.

import (
	"context"
	"errors"
	"fmt"
	"io"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/ports"
)

// ExportService backs the Admin+ async query-history CSV export: Start
// admits a job and enqueues the worker task; Job serves the status read;
// Open streams the finished archive; Process (worker.go) is the worker body.
//
// The privacy gate intentionally lives at the transport entrance (the module
// caller runs AuditService.CheckAccess on every export surface), exactly as
// the legacy handler did — Start does not re-run the policy. The worker,
// however, re-enforces the policy at processing time via the same
// AuditService so an export enqueued before a policy change is masked (or
// refused) under the stricter current policy.
type ExportService struct {
	// gate reuses the Task 5 audit policy service for the worker's
	// processing-time policy recheck (CheckAccess).
	gate *AuditService
	// rows serves the aggregated per-session rows a CSV export is built
	// from; it is the same AuditReader the gate was built on.
	rows  ports.AuditReader
	jobs  ports.ExportJobStore
	files ports.FileStore
	queue ports.TaskQueue
}

// NewExportService wires the export use cases onto their ports. The policy
// and audit readers are the same adapters the AuditService consumes; the
// export flows build their own gate instance over them.
func NewExportService(
	policy ports.PolicyReader, audit ports.AuditReader,
	jobs ports.ExportJobStore, files ports.FileStore, queue ports.TaskQueue,
) *ExportService {
	return &ExportService{
		gate:  NewAuditService(policy, audit),
		rows:  audit,
		jobs:  jobs,
		files: files,
		queue: queue,
	}
}

// Start admits a pending export job and enqueues the worker task. The export
// always spans the Admin+ source=all audit view, so only the audit window the
// admin saw (user / time range / feedback rating) rides along in the payload;
// the domain filter type enforces that scope structurally. When enqueueing
// fails the job is marked failed with the reason so the status read can
// explain it, and the error surfaces to the caller.
func (s *ExportService) Start(
	ctx context.Context, tenantID uint64, requestedBy string, filter domain.ExportFilter,
) (uint64, error) {
	if tenantID == 0 {
		return 0, errors.New("workspace id is required")
	}

	job := &domain.ExportJob{
		TenantID:    tenantID,
		RequestedBy: requestedBy,
		Status:      domain.ExportPending,
	}
	if err := s.jobs.Create(ctx, job); err != nil {
		return 0, err
	}

	payload := domain.ExportPayload{
		JobID:          job.ID,
		TenantID:       tenantID,
		RequestedBy:    requestedBy,
		UserID:         filter.UserID,
		FeedbackRating: filter.FeedbackRating,
	}
	if !filter.StartTime.IsZero() {
		payload.StartTimeMs = filter.StartTime.UnixMilli()
	}
	if !filter.EndTime.IsZero() {
		payload.EndTimeMs = filter.EndTime.UnixMilli()
	}
	// The queue adapter injects the tracing fields and applies the task
	// options (type, queue, retry, timeout) — the application seam carries
	// only the domain payload.
	if err := s.queue.EnqueueQueryHistoryExport(ctx, payload); err != nil {
		markErr := s.jobs.UpdateStatus(ctx, tenantID, job.ID,
			domain.ExportFailed, "", "failed to enqueue export task: "+err.Error())
		if markErr != nil {
			// Nothing further can be done here without a logger; the job
			// stays as-is and the enqueue error still surfaces below.
			_ = markErr
		}
		return 0, err
	}
	return job.ID, nil
}

// Job loads one export job scoped to the caller's tenant. A miss or
// cross-tenant id answers the store's not-found AppError (404); every other
// store failure propagates unchanged.
func (s *ExportService) Job(
	ctx context.Context, tenantID uint64, jobID uint64,
) (*domain.ExportJob, error) {
	return s.jobs.Get(ctx, tenantID, jobID)
}

// Open streams the archive of a completed export and answers its download
// filename (query_history_export_<job id>.csv). A job that has not reached
// done (or has no stored path) answers the bad-request AppError built from
// its status and error message; an archive that can no longer be opened
// answers the legacy internal-error AppError. The stored bytes carry no BOM
// — the transport prepends one for Excel when streaming the response.
func (s *ExportService) Open(
	ctx context.Context, tenantID uint64, jobID uint64,
) (io.ReadCloser, string, error) {
	job, err := s.jobs.Get(ctx, tenantID, jobID)
	if err != nil {
		return nil, "", err
	}
	if job.Status != domain.ExportDone || job.FilePath == "" {
		message := "export is not ready: status=" + job.Status
		if job.ErrorMessage != "" {
			message += ", error=" + job.ErrorMessage
		}
		return nil, "", apperrors.NewBadRequestError(message)
	}

	reader, err := s.files.Open(ctx, job.FilePath)
	if err != nil {
		return nil, "", apperrors.NewInternalServerError("export file is no longer available")
	}
	return reader, fmt.Sprintf("query_history_export_%d.csv", job.ID), nil
}
