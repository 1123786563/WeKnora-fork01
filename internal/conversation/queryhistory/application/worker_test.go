package application

// Tests for the async query-history export worker body (Wave 1, Task 6),
// ported from the legacy query_history_export_test.go: full-chain success,
// anonymized user ids, storage failure, error truncation, idempotent done
// rerun, missing-job drop, disabled policy (including the Review Focus case
// of a policy flipped to disabled after enqueue), permanent payloads, and
// retry recovery. Everything runs against the fake ports defined in
// export_test.go — no real database or task framework.

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/stretchr/testify/require"
)

// exportWorkerTask marshals a payload the way the queue adapter will hand it
// to Process.
func exportWorkerTask(t *testing.T, payload domain.ExportPayload) []byte {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	return raw
}

// requireExportUpdates pins that every recorded status update is scoped to
// the tenant and job and matches the wanted status sequence.
func requireExportUpdates(t *testing.T, jobs *fakeExportJobStore, tenantID, jobID uint64, statuses []domain.ExportStatus) {
	t.Helper()
	require.Len(t, jobs.updates, len(statuses))
	for i, want := range statuses {
		require.Equal(t, tenantID, jobs.updates[i].tenantID, "update %d carries the tenant id", i)
		require.Equal(t, jobID, jobs.updates[i].jobID, "update %d carries the job id", i)
		require.Equal(t, want, jobs.updates[i].status, "update %d status", i)
	}
}

// TestProcessExportFullChainDone ports the legacy end-to-end test: a pending
// job transitions running -> done, the audit reader is consulted once with
// the tenant scope and the payload-rebuilt filter, the CSV is stored via the
// file port under the per-job temp name, and the stored bytes are
// byte-identical to the legacy CSV contract (header order, UTC RFC3339
// timestamps, tallies, no BOM, LF endings).
func TestProcessExportFullChainDone(t *testing.T) {
	svc, policy, reader, jobs, files, _ := newExportHarness(domain.Normal, exportTestRows())
	ctx := context.Background()

	created := &domain.ExportJob{TenantID: 1, RequestedBy: "admin-1"}
	require.NoError(t, jobs.Create(ctx, created))

	payload := domain.ExportPayload{
		JobID: created.ID, TenantID: 1, RequestedBy: "admin-1",
		UserID: "u1", StartTimeMs: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		FeedbackRating: "like",
	}
	require.NoError(t, svc.Process(ctx, exportWorkerTask(t, payload)))

	// Policy recheck happened before the running claim, reader once.
	require.Equal(t, 1, policy.modeCalls)
	require.Equal(t, uint64(1), policy.lastTenantID)
	require.Equal(t, 1, reader.rowsCalls)
	require.Equal(t, uint64(1), reader.lastTenant)
	require.Equal(t, domain.ExportFilter{
		UserID:         "u1",
		StartTime:      time.UnixMilli(payload.StartTimeMs),
		FeedbackRating: "like",
	}, reader.lastFilter, "the payload rebuilds the audit filter verbatim; zero end time stays open")

	requireExportUpdates(t, jobs, 1, created.ID, []domain.ExportStatus{domain.ExportRunning, domain.ExportDone})
	update := jobs.updates[1]
	require.Equal(t, files.savedPath, update.filePath)
	require.Empty(t, update.errMsg)

	job, err := jobs.Get(ctx, 1, created.ID)
	require.NoError(t, err)
	require.Equal(t, domain.ExportDone, job.Status)
	require.Empty(t, job.ErrorMessage)

	wantName := "query_history_export_" + strconv.FormatUint(created.ID, 10) + ".csv"
	require.Equal(t, 1, files.saveCalls)
	require.Equal(t, wantName, files.savedName)
	require.Equal(t, uint64(1), files.savedTenant)
	require.True(t, files.savedTemp, "the archive rides the temp expiry policy")

	// Byte-for-byte pin of the legacy CSV contract (no BOM, LF endings).
	expected := "session_id,title,user_id,source,engine_type,created_at,updated_at,message_count,like_count,dislike_count\n" +
		"exp-a,Alpha,u1,web,builtin,2026-09-01T10:00:00Z,2026-09-01T11:00:00Z,2,1,0\n" +
		"exp-b,Beta,u2,api,builtin,2026-09-02T08:30:00Z,2026-09-02T09:00:00Z,1,0,1\n"
	require.Equal(t, expected, string(files.savedData))

	// And parsed: same records, real owner ids in normal mode.
	records, err := csv.NewReader(strings.NewReader(string(files.savedData))).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 3)
	require.Equal(t, "exp-a", records[1][0])
	require.Equal(t, "u1", records[1][2], "normal mode keeps the owner id")
	require.Equal(t, "2", records[1][7])
	require.Equal(t, "1", records[1][8])
	require.Equal(t, "0", records[1][9])
}

// TestProcessExportAnonymizedMasksUserID ports the legacy anonymized test:
// under the anonymized policy the CSV masks the owner column as "anonymous"
// while every other field stays identical.
func TestProcessExportAnonymizedMasksUserID(t *testing.T) {
	svc, _, _, jobs, files, _ := newExportHarness(domain.Anonymized, exportTestRows())
	ctx := context.Background()

	created := &domain.ExportJob{TenantID: 1}
	require.NoError(t, jobs.Create(ctx, created))
	require.NoError(t, svc.Process(ctx, exportWorkerTask(t, domain.ExportPayload{JobID: created.ID, TenantID: 1})))

	expected := "session_id,title,user_id,source,engine_type,created_at,updated_at,message_count,like_count,dislike_count\n" +
		"exp-a,Alpha,anonymous,web,builtin,2026-09-01T10:00:00Z,2026-09-01T11:00:00Z,2,1,0\n" +
		"exp-b,Beta,anonymous,api,builtin,2026-09-02T08:30:00Z,2026-09-02T09:00:00Z,1,0,1\n"
	require.Equal(t, expected, string(files.savedData))
}

// TestProcessExportFailureMarksFailedAndReturnsError ports the legacy storage
// failure test: the error wraps with the legacy context and surfaces for
// retry, the job is marked failed with the reason and no file path.
func TestProcessExportFailureMarksFailedAndReturnsError(t *testing.T) {
	svc, _, _, jobs, files, _ := newExportHarness(domain.Normal, exportTestRows())
	files.saveErr = io.ErrClosedPipe
	ctx := context.Background()

	created := &domain.ExportJob{TenantID: 1}
	require.NoError(t, jobs.Create(ctx, created))

	err := svc.Process(ctx, exportWorkerTask(t, domain.ExportPayload{JobID: created.ID, TenantID: 1}))
	require.Error(t, err)
	require.ErrorIs(t, err, io.ErrClosedPipe)
	require.Contains(t, err.Error(), "store export file")

	job, getErr := jobs.Get(ctx, 1, created.ID)
	require.NoError(t, getErr)
	require.Equal(t, domain.ExportFailed, job.Status)
	require.Empty(t, job.FilePath)
	require.Contains(t, job.ErrorMessage, "store export file")
}

// TestProcessExportTruncatesStoredError pins the 1,024-byte cap on the stored
// error message: the job row keeps the first 1,024 bytes of the failure text
// while the caller still receives the full original error for retry.
func TestProcessExportTruncatesStoredError(t *testing.T) {
	svc, _, reader, jobs, _, _ := newExportHarness(domain.Normal, nil)
	reader.err = errors.New(strings.Repeat("y", 5000))
	ctx := context.Background()

	created := &domain.ExportJob{TenantID: 1}
	require.NoError(t, jobs.Create(ctx, created))

	err := svc.Process(ctx, exportWorkerTask(t, domain.ExportPayload{JobID: created.ID, TenantID: 1}))
	require.Error(t, err)
	require.Greater(t, len(err.Error()), 1024, "the returned error is the full original")

	job, getErr := jobs.Get(ctx, 1, created.ID)
	require.NoError(t, getErr)
	require.Equal(t, domain.ExportFailed, job.Status)
	require.Len(t, job.ErrorMessage, 1024, "the stored message is truncated to 1024 bytes")
	require.Equal(t, err.Error()[:1024], job.ErrorMessage)
	require.Equal(t, "aggregate export rows: "+strings.Repeat("y", 1001), job.ErrorMessage)
}

// TestProcessExportIdempotentDoneRerun ports the legacy idempotency test: a
// retry of an already-done job returns nil without rebuilding the file,
// without touching the recorded path, and without re-consulting the policy.
func TestProcessExportIdempotentDoneRerun(t *testing.T) {
	svc, policy, _, jobs, files, _ := newExportHarness(domain.Normal, exportTestRows())
	ctx := context.Background()

	created := &domain.ExportJob{TenantID: 1}
	require.NoError(t, jobs.Create(ctx, created))
	task := exportWorkerTask(t, domain.ExportPayload{JobID: created.ID, TenantID: 1})

	require.NoError(t, svc.Process(ctx, task))
	done, err := jobs.Get(ctx, 1, created.ID)
	require.NoError(t, err)
	require.Equal(t, domain.ExportDone, done.Status)
	require.NotEmpty(t, done.FilePath)
	require.Equal(t, 1, policy.modeCalls)
	requireExportUpdates(t, jobs, 1, created.ID, []domain.ExportStatus{domain.ExportRunning, domain.ExportDone})

	// A retry after the committed done must not rebuild the file.
	require.NoError(t, svc.Process(ctx, task))
	require.Equal(t, 1, files.saveCalls)
	require.Equal(t, 1, policy.modeCalls, "the done short-circuit precedes the policy recheck")
	requireExportUpdates(t, jobs, 1, created.ID, []domain.ExportStatus{domain.ExportRunning, domain.ExportDone})

	again, err := jobs.Get(ctx, 1, created.ID)
	require.NoError(t, err)
	require.Equal(t, done.FilePath, again.FilePath)
}

// TestProcessExportMissingJobDropsTask ports the legacy missing-job test: a
// stale task whose job row is gone returns nil (retrying cannot fix it) and
// writes no file and no status update.
func TestProcessExportMissingJobDropsTask(t *testing.T) {
	svc, _, _, jobs, files, _ := newExportHarness(domain.Normal, nil)

	require.NoError(t, svc.Process(context.Background(), exportWorkerTask(t, domain.ExportPayload{JobID: 424242, TenantID: 1})))
	require.Equal(t, 1, jobs.getCalls, "the job read happens (tenant-scoped) before the drop")
	require.Equal(t, uint64(1), jobs.lastGetTenant)
	require.Equal(t, uint64(424242), jobs.lastGetJob)
	require.Empty(t, jobs.updates)
	require.Equal(t, 0, files.saveCalls)
}

// TestProcessExportDisabledAfterEnqueueFailsWithoutFile is the Review Focus
// case: the export was admitted under a normal policy, the workspace then
// flipped to disabled, and the worker's processing-time recheck refuses —
// the job is marked failed with the policy reason, NO CSV file is written,
// and (per the mandated worker order) the job is never even claimed as
// running.
func TestProcessExportDisabledAfterEnqueueFailsWithoutFile(t *testing.T) {
	svc, policy, reader, jobs, files, _ := newExportHarness(domain.Normal, exportTestRows())
	ctx := context.Background()

	created := &domain.ExportJob{TenantID: 1}
	require.NoError(t, jobs.Create(ctx, created))

	// The policy changes to disabled between enqueue and processing.
	policy.mode = domain.Disabled

	err := svc.Process(ctx, exportWorkerTask(t, domain.ExportPayload{JobID: created.ID, TenantID: 1}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "disabled")

	job, getErr := jobs.Get(ctx, 1, created.ID)
	require.NoError(t, getErr)
	require.Equal(t, domain.ExportFailed, job.Status)
	require.Empty(t, job.FilePath)
	require.Contains(t, job.ErrorMessage, "query history is disabled for this tenant")
	require.Equal(t, 0, files.saveCalls, "no file may be written when the policy refuses")
	require.Equal(t, 0, reader.rowsCalls, "the audit reader is never consulted when the policy refuses")
	requireExportUpdates(t, jobs, 1, created.ID, []domain.ExportStatus{domain.ExportFailed})
}

// TestProcessExportMalformedPayloadPermanent ports the legacy skip-retry
// test: a payload that cannot parse answers a domain.ErrPermanentPayload
// wrapper (the adapter maps it to the queue's skip-retry) and never touches a
// job.
func TestProcessExportMalformedPayloadPermanent(t *testing.T) {
	svc, _, _, jobs, files, _ := newExportHarness(domain.Normal, nil)

	err := svc.Process(context.Background(), []byte("{not json"))
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrPermanentPayload)
	require.Contains(t, err.Error(), "unmarshal payload")
	require.Equal(t, 0, jobs.getCalls, "a malformed payload never reaches the job store")
	require.Empty(t, jobs.updates)
	require.Equal(t, 0, files.saveCalls)
}

// TestProcessExportTenantlessPayloadPermanent is the Review Focus case: a
// structurally valid payload missing its tenant or job scope is permanent —
// it can never succeed on retry — and never touches a job.
func TestProcessExportTenantlessPayloadPermanent(t *testing.T) {
	svc, _, _, jobs, files, _ := newExportHarness(domain.Normal, nil)

	cases := []struct {
		name    string
		payload domain.ExportPayload
	}{
		{name: "missing tenant", payload: domain.ExportPayload{JobID: 9}},
		{name: "missing job", payload: domain.ExportPayload{TenantID: 1}},
		{name: "missing both", payload: domain.ExportPayload{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.Process(context.Background(), exportWorkerTask(t, tc.payload))
			require.Error(t, err)
			require.ErrorIs(t, err, domain.ErrPermanentPayload)
			require.Contains(t, err.Error(), "missing job_id/tenant_id")
		})
	}

	require.Equal(t, 0, jobs.getCalls, "an unscoped payload never reaches the job store")
	require.Equal(t, 0, jobs.createCalls)
	require.Empty(t, jobs.updates)
	require.Equal(t, 0, files.saveCalls)
}

// TestProcessExportRetryRecovers pins the retry contract: a failed attempt
// returns the original error so the queue retries, and a later successful
// attempt flips the same job to running -> done, clearing the stale error
// message (every status update rewrites both path and message).
func TestProcessExportRetryRecovers(t *testing.T) {
	svc, _, reader, jobs, files, _ := newExportHarness(domain.Normal, exportTestRows())
	ctx := context.Background()

	created := &domain.ExportJob{TenantID: 1}
	require.NoError(t, jobs.Create(ctx, created))
	task := exportWorkerTask(t, domain.ExportPayload{JobID: created.ID, TenantID: 1})

	// First attempt fails while aggregating rows.
	connErr := errors.New("connection reset")
	reader.err = connErr
	err := svc.Process(ctx, task)
	require.Error(t, err)
	require.ErrorIs(t, err, connErr)
	require.Contains(t, err.Error(), "aggregate export rows")

	failed, getErr := jobs.Get(ctx, 1, created.ID)
	require.NoError(t, getErr)
	require.Equal(t, domain.ExportFailed, failed.Status)
	require.Contains(t, failed.ErrorMessage, "aggregate export rows")

	// The retry succeeds and clears the stale failure.
	reader.err = nil
	require.NoError(t, svc.Process(ctx, task))

	requireExportUpdates(t, jobs, 1, created.ID,
		[]domain.ExportStatus{domain.ExportRunning, domain.ExportFailed, domain.ExportRunning, domain.ExportDone})
	done, getErr := jobs.Get(ctx, 1, created.ID)
	require.NoError(t, getErr)
	require.Equal(t, domain.ExportDone, done.Status)
	require.Empty(t, done.ErrorMessage, "the done transition clears the stale error message")
	require.Equal(t, files.savedPath, done.FilePath)
	require.Equal(t, 1, files.saveCalls)
}

// TestProcessExportJobReadErrorPropagates pins that a job-store failure other
// than not-found surfaces unchanged for retry without inventing a transition:
// the job's existence is unknown, so nothing is marked.
func TestProcessExportJobReadErrorPropagates(t *testing.T) {
	svc, _, _, jobs, files, _ := newExportHarness(domain.Normal, exportTestRows())
	storeErr := errors.New("db unavailable")
	jobs.getErr = storeErr

	err := svc.Process(context.Background(), exportWorkerTask(t, domain.ExportPayload{JobID: 1, TenantID: 1}))
	require.ErrorIs(t, err, storeErr)
	require.Empty(t, jobs.updates)
	require.Equal(t, 0, files.saveCalls)
}

// TestProcessExportPolicyErrorMarksFailed pins that a failing policy lookup
// (not just an explicit disabled mode) fails the job and surfaces the
// original error, with the audit reader and file store untouched.
func TestProcessExportPolicyErrorMarksFailed(t *testing.T) {
	svc, policy, reader, jobs, files, _ := newExportHarness(domain.Normal, exportTestRows())
	policy.err = errors.New("tenant lookup down")
	ctx := context.Background()

	created := &domain.ExportJob{TenantID: 1}
	require.NoError(t, jobs.Create(ctx, created))

	err := svc.Process(ctx, exportWorkerTask(t, domain.ExportPayload{JobID: created.ID, TenantID: 1}))
	require.ErrorIs(t, err, policy.err)

	job, getErr := jobs.Get(ctx, 1, created.ID)
	require.NoError(t, getErr)
	require.Equal(t, domain.ExportFailed, job.Status)
	require.Contains(t, job.ErrorMessage, "tenant lookup down")
	require.Equal(t, 0, reader.rowsCalls)
	require.Equal(t, 0, files.saveCalls)
	requireExportUpdates(t, jobs, 1, created.ID, []domain.ExportStatus{domain.ExportFailed})
}

// TestProcessExportNotFoundIsAppErrorBased documents the not-found contract
// the Wave 1 adapter must honor: the job store's not-found answer is an
// AppError with the 404 code, which is what the worker's drop path detects.
func TestProcessExportNotFoundIsAppErrorBased(t *testing.T) {
	_, _, _, jobs, _, _ := newExportHarness(domain.Normal, nil)
	ctx := context.Background()
	require.NoError(t, jobs.Create(ctx, &domain.ExportJob{TenantID: 1}))

	_, err := jobs.Get(ctx, 2, jobs.nextID)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, apperrors.ErrNotFound, appErr.Code)
}
