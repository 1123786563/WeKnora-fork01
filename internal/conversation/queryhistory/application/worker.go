package application

// The async query-history export worker body (Wave 1, Task 6), moved from
// the legacy ProcessExport / runExport / buildQueryHistoryExportCSV triple.
// Process receives the raw task payload bytes, so the application layer owns
// the payload parse; a payload that can never succeed (malformed JSON,
// missing job/tenant scope) answers domain.ErrPermanentPayload without
// touching a job, and the module's queue adapter (Wave 1, Task 10) translates
// that sentinel into the task framework's skip-retry — this file never
// imports the framework. CSV rendering is application logic per the plan: it
// travels with the worker that owns the column contract.

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
)

// exportColumns is the CSV header of a query-history export. Column order is
// part of the contract (admin tooling parses positionally); new columns may
// only be appended.
var exportColumns = []string{
	"session_id", "title", "user_id", "source", "engine_type",
	"created_at", "updated_at", "message_count", "like_count", "dislike_count",
}

// exportErrorLimit caps the error message persisted on a failed export job so
// one runaway failure cannot balloon the row (same cap as the legacy worker).
const exportErrorLimit = 1024

// exportFileName is the per-job archive name (storage and download share it).
func exportFileName(jobID uint64) string {
	return fmt.Sprintf("query_history_export_%d.csv", jobID)
}

// isExportJobNotFound reports whether a job-store read failed because the job
// does not exist in the requesting tenant. The port contract says absence
// and cross-tenant probes answer the same not-found AppError, which is what
// this detects.
func isExportJobNotFound(err error) bool {
	appErr, ok := apperrors.IsAppError(err)
	return ok && appErr.Code == apperrors.ErrNotFound
}

// Process is the export worker body. Failures mark the job failed (error
// message truncated to exportErrorLimit bytes) and return the original error
// so the queue retries; a retry of a job that already reached done returns
// immediately — the export is idempotent once its file exists.
//
// The order is exactly: validate payload -> tenant-scoped job read -> done
// short-circuit -> policy recheck -> running transition -> audit rows ->
// anonymize -> CSV encode -> file write -> done transition. Every status
// update carries both the tenant id and the job id.
func (s *ExportService) Process(ctx context.Context, payload []byte) error {
	var p domain.ExportPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		// A malformed payload can never succeed on retry; skip the budget.
		return fmt.Errorf("query history export: unmarshal payload (%v): %w", err, domain.ErrPermanentPayload)
	}
	if p.TenantID == 0 || p.JobID == 0 {
		return fmt.Errorf("query history export: payload missing job_id/tenant_id: %w", domain.ErrPermanentPayload)
	}

	job, err := s.jobs.Get(ctx, p.TenantID, p.JobID)
	if err != nil {
		if isExportJobNotFound(err) {
			// The job row is gone (tenant wiped between enqueue and run):
			// nothing to update, retrying cannot bring it back.
			return nil
		}
		return err
	}
	if job.Status == domain.ExportDone {
		// Idempotent rerun: a retry after a committed export must not rebuild
		// the file or reset the path.
		return nil
	}

	if err := s.runExport(ctx, &p); err != nil {
		msg := err.Error()
		if len(msg) > exportErrorLimit {
			msg = msg[:exportErrorLimit]
		}
		if markErr := s.jobs.UpdateStatus(ctx, p.TenantID, p.JobID, domain.ExportFailed, "", msg); markErr != nil {
			// Nothing further can be done here without a logger; the
			// original error still surfaces below for retry.
			_ = markErr
		}
		return err
	}
	return nil
}

// runExport executes one export attempt: enforce the privacy policy at
// processing time (an export enqueued before a policy change is masked — or
// refused — under the stricter current policy, and a refused job is never
// even claimed as running), claim the job as running, aggregate the rows,
// render and store the CSV, then flip the job to done with the file path.
func (s *ExportService) runExport(ctx context.Context, p *domain.ExportPayload) error {
	mode, err := s.gate.CheckAccess(ctx, p.TenantID)
	if err != nil {
		return err
	}

	if err := s.jobs.UpdateStatus(ctx, p.TenantID, p.JobID, domain.ExportRunning, "", ""); err != nil {
		return err
	}

	filter := domain.ExportFilter{
		UserID:         p.UserID,
		FeedbackRating: p.FeedbackRating,
	}
	if p.StartTimeMs > 0 {
		filter.StartTime = time.UnixMilli(p.StartTimeMs)
	}
	if p.EndTimeMs > 0 {
		filter.EndTime = time.UnixMilli(p.EndTimeMs)
	}

	rows, err := s.rows.ExportRows(ctx, p.TenantID, filter)
	if err != nil {
		return fmt.Errorf("aggregate export rows: %w", err)
	}
	data, err := buildExportCSV(rows, mode == domain.Anonymized)
	if err != nil {
		return fmt.Errorf("render export csv: %w", err)
	}
	// temp=true: the archive rides the same expiry policy as other derived
	// artifacts rather than occupying permanent storage quota.
	filePath, err := s.files.SaveBytes(ctx, data, p.TenantID, exportFileName(p.JobID), true)
	if err != nil {
		return fmt.Errorf("store export file: %w", err)
	}
	if err := s.jobs.UpdateStatus(ctx, p.TenantID, p.JobID, domain.ExportDone, filePath, ""); err != nil {
		return fmt.Errorf("mark export done: %w", err)
	}
	return nil
}

// buildExportCSV renders the export rows with encoding/csv so delimiter/
// quote escaping in titles is correct by construction. The stored file
// carries no BOM; the download surface prepends one for Excel (same contract
// as the usage export).
func buildExportCSV(rows []domain.ExportRow, anonymize bool) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(exportColumns); err != nil {
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
