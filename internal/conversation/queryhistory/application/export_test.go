package application

// Tests for the async query-history export submission and download surfaces
// (Wave 1, Task 6), ported from the legacy
// query_history_export_test.go: job creation, tenant-required validation,
// enqueue-failure marking, tenant-scoped status, not-ready download, and the
// completed filename. Everything runs against fake ports — no real database,
// task queue, or storage backend. The shared fakes at the top of this file
// are also the substrate for worker_test.go.

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/ports"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// fakeJobUpdate records one UpdateStatus call with the scope it carried.
type fakeJobUpdate struct {
	tenantID uint64
	jobID    uint64
	status   domain.ExportStatus
	filePath string
	errMsg   string
}

// fakeExportJobStore is the test double for ports.ExportJobStore: an in-memory
// job table with tenant-scoped reads (a missing or foreign job id answers the
// same NotFound AppError the Wave 1 adapter will), plus call recording.
type fakeExportJobStore struct {
	jobs      map[uint64]*domain.ExportJob
	nextID    uint64
	createErr error
	getErr    error
	updateErr error

	createCalls   int
	getCalls      int
	lastGetTenant uint64
	lastGetJob    uint64
	updates       []fakeJobUpdate
}

func newFakeExportJobStore() *fakeExportJobStore {
	return &fakeExportJobStore{jobs: map[uint64]*domain.ExportJob{}}
}

func (f *fakeExportJobStore) Create(_ context.Context, job *domain.ExportJob) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.createCalls++
	f.nextID++
	job.ID = f.nextID
	if job.Status == "" {
		job.Status = domain.ExportPending
	}
	stored := *job
	f.jobs[job.ID] = &stored
	return nil
}

func (f *fakeExportJobStore) Get(_ context.Context, tenantID uint64, jobID uint64) (*domain.ExportJob, error) {
	f.getCalls++
	f.lastGetTenant, f.lastGetJob = tenantID, jobID
	if f.getErr != nil {
		return nil, f.getErr
	}
	job, ok := f.jobs[jobID]
	if !ok || job.TenantID != tenantID {
		return nil, apperrors.NewNotFoundError("query history export job not found")
	}
	stored := *job
	return &stored, nil
}

func (f *fakeExportJobStore) UpdateStatus(
	_ context.Context, tenantID uint64, jobID uint64,
	status domain.ExportStatus, filePath string, errMsg string,
) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updates = append(f.updates, fakeJobUpdate{
		tenantID: tenantID, jobID: jobID, status: status, filePath: filePath, errMsg: errMsg,
	})
	if job, ok := f.jobs[jobID]; ok {
		job.Status, job.FilePath, job.ErrorMessage = status, filePath, errMsg
	}
	return nil
}

var _ ports.ExportJobStore = (*fakeExportJobStore)(nil)

// fakeTaskQueue is the test double for ports.TaskQueue: it captures the
// payloads Start would dispatch. Task options (queue, retry, timeout) are
// owned by the Wave 1 queue adapter, so the payload is all this seam sees.
type fakeTaskQueue struct {
	payloads []domain.ExportPayload
	err      error
}

func (f *fakeTaskQueue) EnqueueQueryHistoryExport(_ context.Context, payload domain.ExportPayload) error {
	if f.err != nil {
		return f.err
	}
	f.payloads = append(f.payloads, payload)
	return nil
}

var _ ports.TaskQueue = (*fakeTaskQueue)(nil)

// fakeFileStore is the test double for ports.FileStore: SaveBytes captures the
// stored bytes and answers a local://temp/ path (same shape as the legacy
// stub); Open replays the last saved bytes.
type fakeFileStore struct {
	saveErr error
	openErr error

	saveCalls   int
	savedData   []byte
	savedTenant uint64
	savedName   string
	savedTemp   bool
	savedPath   string

	openCalls  int
	openedPath string
}

func (f *fakeFileStore) SaveBytes(
	_ context.Context, data []byte, tenantID uint64, fileName string, temp bool,
) (string, error) {
	if f.saveErr != nil {
		return "", f.saveErr
	}
	f.saveCalls++
	f.savedData = append([]byte(nil), data...)
	f.savedTenant, f.savedName, f.savedTemp = tenantID, fileName, temp
	f.savedPath = "local://temp/" + fileName
	return f.savedPath, nil
}

func (f *fakeFileStore) Open(_ context.Context, path string) (io.ReadCloser, error) {
	if f.openErr != nil {
		return nil, f.openErr
	}
	f.openCalls++
	f.openedPath = path
	return io.NopCloser(bytes.NewReader(f.savedData)), nil
}

var _ ports.FileStore = (*fakeFileStore)(nil)

// fakeExportReader is the AuditReader test double for the export flows: the
// snapshot use case is unused here, ExportRows serves fixed rows and records
// the tenant scope and filter it was consulted with.
type fakeExportReader struct {
	rows []domain.ExportRow
	err  error

	rowsCalls  int
	lastTenant uint64
	lastFilter domain.ExportFilter
}

func (f *fakeExportReader) Snapshot(
	context.Context, uint64, string,
) (*types.QueryHistorySnapshot, error) {
	return nil, nil
}

func (f *fakeExportReader) ExportRows(
	_ context.Context, tenantID uint64, filter domain.ExportFilter,
) ([]domain.ExportRow, error) {
	f.rowsCalls++
	f.lastTenant, f.lastFilter = tenantID, filter
	if f.err != nil {
		return nil, f.err
	}
	return f.rows, nil
}

var _ ports.AuditReader = (*fakeExportReader)(nil)

// newExportHarness wires the export service onto fakes over the given policy
// mode. It mirrors what the Wave 1 wiring will do: one policy reader, one
// audit reader, one job store, one file store, one queue.
func newExportHarness(mode domain.Mode, rows []domain.ExportRow) (
	*ExportService, *fakePolicyReader, *fakeExportReader, *fakeExportJobStore, *fakeFileStore, *fakeTaskQueue,
) {
	policy := &fakePolicyReader{mode: mode}
	reader := &fakeExportReader{rows: rows}
	jobs := newFakeExportJobStore()
	files := &fakeFileStore{}
	queue := &fakeTaskQueue{}
	svc := NewExportService(policy, reader, jobs, files, queue)
	return svc, policy, reader, jobs, files, queue
}

// exportTestRows serves the same shape as the legacy seedExportFixtures: two
// sessions with message/feedback tallies. The first row's CreatedAt uses a
// +08:00 zone to pin the CSV's UTC normalization.
func exportTestRows() []domain.ExportRow {
	plus8 := time.FixedZone("plus8", 8*3600)
	return []domain.ExportRow{
		{
			SessionID: "exp-a", Title: "Alpha", UserID: "u1", Source: "web", EngineType: "builtin",
			CreatedAt: time.Date(2026, 9, 1, 18, 0, 0, 0, plus8), UpdatedAt: time.Date(2026, 9, 1, 11, 0, 0, 0, time.UTC),
			MessageCount: 2, LikeCount: 1, DislikeCount: 0,
		},
		{
			SessionID: "exp-b", Title: "Beta", UserID: "u2", Source: "api", EngineType: "builtin",
			CreatedAt: time.Date(2026, 9, 2, 8, 30, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 9, 2, 9, 0, 0, 0, time.UTC),
			MessageCount: 1, LikeCount: 0, DislikeCount: 1,
		},
	}
}

// exportTestFilter mirrors the legacy StartExport test filter: user + start
// time + feedback rating, zero end time.
func exportTestFilter(t *testing.T) domain.ExportFilter {
	t.Helper()
	return domain.ExportFilter{
		UserID:         "u1",
		StartTime:      time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		FeedbackRating: "like",
	}
}

// TestExportStartCreatesPendingJobAndEnqueues ports the legacy submission
// test: a pending job row is created with the requester, and the queue
// receives the audit filter verbatim — same user, same start time in unix
// milliseconds, same feedback rating, and a zero end time staying open.
func TestExportStartCreatesPendingJobAndEnqueues(t *testing.T) {
	svc, _, _, jobs, _, queue := newExportHarness(domain.Normal, nil)
	filter := exportTestFilter(t)

	jobID, err := svc.Start(context.Background(), 1, "admin-1", filter)
	require.NoError(t, err)
	require.NotZero(t, jobID)

	job, err := jobs.Get(context.Background(), 1, jobID)
	require.NoError(t, err)
	require.Equal(t, domain.ExportPending, job.Status)
	require.Equal(t, "admin-1", job.RequestedBy)
	require.Equal(t, uint64(1), job.TenantID)

	require.Len(t, queue.payloads, 1, "exactly one task is enqueued")
	payload := queue.payloads[0]
	require.Equal(t, domain.ExportPayload{
		JobID:          jobID,
		TenantID:       1,
		RequestedBy:    "admin-1",
		UserID:         "u1",
		StartTimeMs:    filter.StartTime.UnixMilli(),
		FeedbackRating: "like",
	}, payload, "the payload carries the audit filter verbatim; zero end time stays open")
}

// TestExportStartEnqueueFailureMarksJobFailed ports the legacy enqueue-failure
// test: the caller gets the error and no job id, exactly one job row exists,
// and it is marked failed with the enqueue reason. The failed transition is
// tenant+job scoped like every status update.
func TestExportStartEnqueueFailureMarksJobFailed(t *testing.T) {
	svc, _, _, jobs, _, queue := newExportHarness(domain.Normal, nil)
	queue.err = errors.New("redis unavailable")

	jobID, err := svc.Start(context.Background(), 1, "admin-1", domain.ExportFilter{})
	require.Error(t, err)
	require.Zero(t, jobID, "the caller must not get a job id that will never run")
	require.ErrorIs(t, err, queue.err, "the original enqueue error surfaces to the caller")

	require.Equal(t, 1, jobs.createCalls, "exactly one job row is created even when enqueueing fails")
	require.Len(t, jobs.updates, 1)
	update := jobs.updates[0]
	require.Equal(t, uint64(1), update.tenantID)
	require.Equal(t, jobs.nextID, update.jobID)
	require.Equal(t, domain.ExportFailed, update.status)
	require.Empty(t, update.filePath)
	require.Contains(t, update.errMsg, "failed to enqueue export task")
	require.Contains(t, update.errMsg, "redis unavailable")
}

// TestExportStartRequiresTenant ports the legacy zero-tenant validation: the
// exact legacy error text, and neither a job row nor an enqueue happens.
func TestExportStartRequiresTenant(t *testing.T) {
	svc, _, _, jobs, _, queue := newExportHarness(domain.Normal, nil)

	_, err := svc.Start(context.Background(), 0, "admin-1", domain.ExportFilter{})
	require.EqualError(t, err, "workspace id is required")
	require.Equal(t, 0, jobs.createCalls)
	require.Empty(t, queue.payloads)
}

// TestExportJobTenantScoped ports the legacy status test: Job serves the
// stored row for the owning tenant, and a missing or foreign job id answers
// the not-found AppError so ids cannot be probed across workspaces.
func TestExportJobTenantScoped(t *testing.T) {
	svc, _, _, jobs, _, _ := newExportHarness(domain.Normal, nil)
	ctx := context.Background()

	created := &domain.ExportJob{TenantID: 7, RequestedBy: "admin-7", Status: domain.ExportPending}
	require.NoError(t, jobs.Create(ctx, created))

	job, err := svc.Job(ctx, 7, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, job.ID)
	require.Equal(t, domain.ExportPending, job.Status)
	require.Equal(t, "admin-7", job.RequestedBy)

	// Same id, wrong tenant: indistinguishable from missing.
	_, err = svc.Job(ctx, 8, created.ID)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, apperrors.ErrNotFound, appErr.Code)
	require.Equal(t, 404, appErr.HTTPCode)

	_, err = svc.Job(ctx, 7, 424242)
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, apperrors.ErrNotFound, appErr.Code)
}

// TestExportOpenNotReady ports the legacy not-ready download guard: any job
// that is not done (or is done without a file path) answers the exact legacy
// bad-request message built from the status and error message, and no file is
// opened.
func TestExportOpenNotReady(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name    string
		job     *domain.ExportJob
		wantMsg string
	}{
		{
			name:    "pending",
			job:     &domain.ExportJob{TenantID: 1, Status: domain.ExportPending},
			wantMsg: "export is not ready: status=pending",
		},
		{
			name:    "running",
			job:     &domain.ExportJob{TenantID: 1, Status: domain.ExportRunning},
			wantMsg: "export is not ready: status=running",
		},
		{
			name:    "failed with error message",
			job:     &domain.ExportJob{TenantID: 1, Status: domain.ExportFailed, ErrorMessage: "store export file: boom"},
			wantMsg: "export is not ready: status=failed, error=store export file: boom",
		},
		{
			name:    "done without file path",
			job:     &domain.ExportJob{TenantID: 1, Status: domain.ExportDone},
			wantMsg: "export is not ready: status=done",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _, jobs, files, _ := newExportHarness(domain.Normal, nil)
			require.NoError(t, jobs.Create(ctx, tc.job))

			reader, filename, err := svc.Open(ctx, 1, tc.job.ID)
			require.Nil(t, reader)
			require.Empty(t, filename)
			var appErr *apperrors.AppError
			require.ErrorAs(t, err, &appErr)
			require.Equal(t, apperrors.ErrBadRequest, appErr.Code)
			require.Equal(t, 400, appErr.HTTPCode)
			require.Equal(t, tc.wantMsg, appErr.Message)
			require.Equal(t, 0, files.openCalls)
		})
	}
}

// TestExportOpenCompleted ports the legacy download behavior: a done job with
// a file path streams the stored bytes and answers the completed filename.
// The stored bytes carry no BOM (the transport prepends one for Excel) and the
// file is opened at the job's recorded path.
func TestExportOpenCompleted(t *testing.T) {
	svc, _, _, jobs, files, _ := newExportHarness(domain.Normal, exportTestRows())
	ctx := context.Background()

	require.NoError(t, jobs.Create(ctx, &domain.ExportJob{TenantID: 1, Status: domain.ExportPending}))
	jobID := jobs.nextID
	storedPath := "local://temp/query_history_export_" + strconv.FormatUint(jobID, 10) + ".csv"
	require.NoError(t, jobs.UpdateStatus(ctx, 1, jobID, domain.ExportDone, storedPath, ""))
	files.savedData = []byte("session_id,title\nexp-a,Alpha\n")

	rc, filename, err := svc.Open(ctx, 1, jobID)
	require.NoError(t, err)
	require.NotNil(t, rc)
	defer func() { _ = rc.Close() }()

	require.Equal(t, "query_history_export_"+strconv.FormatUint(jobID, 10)+".csv", filename)
	require.Equal(t, 1, files.openCalls)
	require.Equal(t, storedPath, files.openedPath, "the job's recorded path is what gets opened")

	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, "session_id,title\nexp-a,Alpha\n", string(data))
	require.False(t, strings.HasPrefix(string(data), "\xef\xbb\xbf"), "no BOM in stored bytes")
}

// TestExportOpenFileUnavailable pins the legacy open-failure mapping: when the
// stored file can no longer be opened, the caller gets the exact legacy
// internal-error AppError.
func TestExportOpenFileUnavailable(t *testing.T) {
	svc, _, _, jobs, files, _ := newExportHarness(domain.Normal, nil)
	ctx := context.Background()
	files.openErr = errors.New("object gone")

	require.NoError(t, jobs.Create(ctx, &domain.ExportJob{TenantID: 1, Status: domain.ExportDone, FilePath: "local://temp/x.csv"}))

	reader, _, err := svc.Open(ctx, 1, jobs.nextID)
	require.Nil(t, reader)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, apperrors.ErrInternalServer, appErr.Code)
	require.Equal(t, 500, appErr.HTTPCode)
	require.Equal(t, "export file is no longer available", appErr.Message)
}
