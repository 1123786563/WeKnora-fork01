package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/tracing/langfuse"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/hibiken/asynq"
)

// queryHistoryExportColumns is the CSV header of a query-history export.
// Column order is part of the contract (admin tooling parses positionally);
// new columns may only be appended.
var queryHistoryExportColumns = []string{
	"session_id", "title", "user_id", "source", "engine_type",
	"created_at", "updated_at", "message_count", "like_count", "dislike_count",
}

// QueryHistoryExportService backs the Admin+ async query-history CSV export
// (SP13): StartExport admits a job and enqueues the asynq task;
// ProcessExport is the worker body that aggregates session rows, renders the
// CSV, stores it via the FileService, and transitions the job.
//
// The privacy gate intentionally lives at the HTTP entrance (the handler
// calls CheckAccess on every export endpoint) rather than being re-run by
// StartExport; the worker, however, re-enforces the policy at processing
// time via CheckQueryHistoryAccess so an export enqueued before a policy
// change is masked (or refused) under the stricter current policy.
type QueryHistoryExportService struct {
	jobRepo     interfaces.QueryHistoryExportJobRepository
	tenantRepo  interfaces.TenantRepository
	fileService interfaces.FileService
	enqueuer    interfaces.TaskEnqueuer
}

// NewQueryHistoryExportService builds the async query-history export service.
func NewQueryHistoryExportService(
	jobRepo interfaces.QueryHistoryExportJobRepository,
	tenantRepo interfaces.TenantRepository,
	fileService interfaces.FileService,
	enqueuer interfaces.TaskEnqueuer,
) *QueryHistoryExportService {
	return &QueryHistoryExportService{
		jobRepo:     jobRepo,
		tenantRepo:  tenantRepo,
		fileService: fileService,
		enqueuer:    enqueuer,
	}
}

// CheckAccess enforces the tenant's query-history privacy policy for the
// export HTTP endpoints (single choke point: disabled → ForbiddenError,
// anonymized → mode returned so callers know masking applies).
func (s *QueryHistoryExportService) CheckAccess(
	ctx context.Context, tenantID uint64,
) (string, error) {
	return CheckQueryHistoryAccess(ctx, s.tenantRepo, tenantID)
}

// GetExportJob loads one export job scoped to the caller's tenant. A miss or
// cross-tenant id answers an AppError NotFound the handler maps to 404.
func (s *QueryHistoryExportService) GetExportJob(
	ctx context.Context, tenantID uint64, jobID uint64,
) (*types.QueryHistoryExportJob, error) {
	job, err := s.jobRepo.GetByID(ctx, tenantID, jobID)
	if err != nil {
		if stderrors.Is(err, repository.ErrQueryHistoryExportJobNotFound) {
			return nil, err
		}
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"tenant_id": tenantID,
			"job_id":    jobID,
		})
		return nil, err
	}
	return job, nil
}

// StartExport admits a pending export job and enqueues the worker task. The
// export always spans the Admin+ source=all audit view; only the audit
// filters (user / time window / feedback rating) ride along in the payload.
// When enqueueing fails the job is marked failed with the reason so the
// status endpoint can explain it, and the error surfaces to the caller.
func (s *QueryHistoryExportService) StartExport(
	ctx context.Context, tenantID uint64, requestedBy string, filter types.SessionListQuery,
) (uint64, error) {
	if tenantID == 0 {
		return 0, stderrors.New("workspace id is required")
	}
	// The export is the audit listing flattened to CSV: same tenant scope,
	// same filters, no paging (the repo caps the row count).
	filter.TenantID = tenantID
	filter.Source = types.SessionListSourceAll
	filter.Keyword = ""
	filter.AgentID = ""
	filter.Page = 0
	filter.PageSize = 0

	job := &types.QueryHistoryExportJob{
		TenantID:    tenantID,
		RequestedBy: requestedBy,
		Status:      types.QueryHistoryExportPending,
	}
	if err := s.jobRepo.Create(ctx, job); err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{"tenant_id": tenantID})
		return 0, err
	}

	payload := &types.QueryHistoryExportPayload{
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
	langfuse.InjectTracing(ctx, payload)

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	task := asynq.NewTask(types.TypeQueryHistoryExport, payloadJSON,
		asynq.Queue(types.QueueMaintenance), asynq.MaxRetry(3), asynq.Timeout(10*time.Minute))
	if _, err := s.enqueuer.Enqueue(task); err != nil {
		logger.Errorf(ctx, "query history export %d: enqueue failed: %v", job.ID, err)
		markErr := s.jobRepo.UpdateStatus(ctx, job.ID,
			types.QueryHistoryExportFailed, "", "failed to enqueue export task: "+err.Error())
		if markErr != nil {
			logger.Errorf(ctx, "query history export %d: mark failed after enqueue error: %v", job.ID, markErr)
		}
		return 0, err
	}
	logger.Infof(ctx, "query history export %d enqueued (tenant=%d requested_by=%s)",
		job.ID, tenantID, requestedBy)
	return job.ID, nil
}

// ProcessExport is the asynq worker body. Failures mark the job failed and
// return the error so asynq retries (MaxRetry(3)); a retry of a job that
// already reached done returns immediately — the export is idempotent once
// its file exists.
func (s *QueryHistoryExportService) ProcessExport(ctx context.Context, t *asynq.Task) error {
	var payload types.QueryHistoryExportPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		// A malformed payload can never succeed on retry; skip the budget.
		return fmt.Errorf("query history export: unmarshal payload (%v): %w", err, asynq.SkipRetry)
	}
	if payload.TenantID == 0 || payload.JobID == 0 {
		return fmt.Errorf("query history export: payload missing job_id/tenant_id: %w", asynq.SkipRetry)
	}

	job, err := s.jobRepo.GetByID(ctx, payload.TenantID, payload.JobID)
	if err != nil {
		if stderrors.Is(err, repository.ErrQueryHistoryExportJobNotFound) {
			// The job row is gone (tenant wiped between enqueue and run):
			// nothing to update, retrying cannot bring it back.
			logger.Warnf(ctx, "query history export %d: job row not found, dropping task", payload.JobID)
			return nil
		}
		return err
	}
	if job.Status == types.QueryHistoryExportDone {
		// Idempotent rerun: a retry after a committed export must not rebuild
		// the file or reset the path.
		logger.Infof(ctx, "query history export %d: already done, skipping rerun", job.ID)
		return nil
	}

	if err := s.runExport(ctx, &payload, job.ID); err != nil {
		msg := err.Error()
		if len(msg) > 1024 {
			msg = msg[:1024]
		}
		if markErr := s.jobRepo.UpdateStatus(ctx, job.ID, types.QueryHistoryExportFailed, "", msg); markErr != nil {
			logger.Errorf(ctx, "query history export %d: mark failed: %v", job.ID, markErr)
		}
		return err
	}
	return nil
}

// runExport executes one export attempt: claim the job as running, enforce
// the privacy policy at processing time, aggregate the rows, render and
// store the CSV, then flip the job to done with the file path.
func (s *QueryHistoryExportService) runExport(
	ctx context.Context, payload *types.QueryHistoryExportPayload, jobID uint64,
) error {
	if err := s.jobRepo.UpdateStatus(ctx, jobID, types.QueryHistoryExportRunning, "", ""); err != nil {
		return err
	}

	// The policy is re-read at processing time (stricter than trusting the
	// request-time snapshot): disabled now → the export refuses to run;
	// anonymized now → the CSV masks the owner column.
	mode, err := CheckQueryHistoryAccess(ctx, s.tenantRepo, payload.TenantID)
	if err != nil {
		return err
	}

	query := &types.SessionListQuery{
		TenantID:       payload.TenantID,
		Source:         types.SessionListSourceAll,
		UserID:         payload.UserID,
		FeedbackRating: payload.FeedbackRating,
	}
	if payload.StartTimeMs > 0 {
		query.StartTime = time.UnixMilli(payload.StartTimeMs)
	}
	if payload.EndTimeMs > 0 {
		query.EndTime = time.UnixMilli(payload.EndTimeMs)
	}

	rows, err := s.jobRepo.ExportSessionRows(ctx, query)
	if err != nil {
		return fmt.Errorf("aggregate export rows: %w", err)
	}
	data, err := buildQueryHistoryExportCSV(rows, mode == types.QueryHistoryModeAnonymized)
	if err != nil {
		return fmt.Errorf("render export csv: %w", err)
	}
	fileName := fmt.Sprintf("query_history_export_%d.csv", jobID)
	// temp=true: the archive rides the same expiry policy as other derived
	// artifacts rather than occupying permanent storage quota.
	filePath, err := s.fileService.SaveBytes(ctx, data, payload.TenantID, fileName, true)
	if err != nil {
		return fmt.Errorf("store export file: %w", err)
	}
	if err := s.jobRepo.UpdateStatus(ctx, jobID, types.QueryHistoryExportDone, filePath, ""); err != nil {
		return fmt.Errorf("mark export done: %w", err)
	}
	logger.Infof(ctx, "query history export %d done: %d rows -> %s", jobID, len(rows), filePath)
	return nil
}

// buildQueryHistoryExportCSV renders the export rows with encoding/csv so
// delimiter/quote escaping in titles is correct by construction. The stored
// file carries no BOM; the download endpoint prepends one for Excel (same
// contract as the usage export).
func buildQueryHistoryExportCSV(rows []types.QueryHistoryExportRow, anonymize bool) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(queryHistoryExportColumns); err != nil {
		return nil, err
	}
	for _, row := range rows {
		userID := row.UserID
		if anonymize {
			userID = "anonymous"
		}
		record := []string{
			row.SessionID,
			row.Title,
			userID,
			row.Source,
			row.EngineType,
			row.CreatedAt.UTC().Format(time.RFC3339),
			row.UpdatedAt.UTC().Format(time.RFC3339),
			strconv.FormatInt(row.MessageCount, 10),
			strconv.FormatInt(row.LikeCount, 10),
			strconv.FormatInt(row.DislikeCount, 10),
		}
		if err := w.Write(record); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
