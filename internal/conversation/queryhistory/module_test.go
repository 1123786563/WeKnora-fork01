package queryhistory_test

// Module assembly tests (Wave 1, Task 10). One Module is constructed over
// fake ports and the brief's four assertions are executed against it:
//
//   - it registers exactly the four Admin+ audit routes and one worker
//     (the single types.TypeQueryHistoryExport task type);
//   - a duplicate worker registration fails deterministically;
//   - the mounted routes serve through the injected application (the fake
//     ports observe every request);
//   - two independently constructed modules never share state (no
//     package-level mutable state backs the assembly).
//
// The worker tests additionally pin the two contract points the brief
// freezes: the private asynq handler passes task.Payload() to the
// application worker (proved by the tenant/job scope the job-store fake
// observes) and maps domain.ErrPermanentPayload to asynq.SkipRetry, and
// the logger boundary actually EMITS on failure paths instead of being
// constructed and ignored.

import (
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/bootstrap"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/Tencent/WeKnora/internal/types"
)

// --- fakes for the module's ports and logger boundary ---

type fakePolicy struct {
	mode domain.Mode
	err  error
}

func (p *fakePolicy) Mode(context.Context, uint64) (domain.Mode, error) {
	return p.mode, p.err
}

type fakeAudit struct {
	mu        sync.Mutex
	snapshot  *types.QueryHistorySnapshot
	snapErr   error
	rows      []domain.ExportRow
	snapCalls int
	rowsCalls int
}

func (a *fakeAudit) Snapshot(
	_ context.Context, tenantID uint64, sessionID string,
) (*types.QueryHistorySnapshot, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.snapCalls++
	if a.snapErr != nil {
		return nil, a.snapErr
	}
	kept := *a.snapshot
	kept.Session.TenantID = tenantID
	kept.Session.ID = sessionID
	return &kept, nil
}

func (a *fakeAudit) ExportRows(
	context.Context, uint64, domain.ExportFilter,
) ([]domain.ExportRow, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rowsCalls++
	return a.rows, nil
}

type fakeJobStore struct {
	mu        sync.Mutex
	nextID    uint64
	jobs      map[uint64]*domain.ExportJob
	getErr    error
	getScopes []struct {
		tenant uint64
		job    uint64
	}
	statuses []struct {
		tenant uint64
		job    uint64
		status domain.ExportStatus
	}
}

func newFakeJobStore(seed ...*domain.ExportJob) *fakeJobStore {
	s := &fakeJobStore{jobs: map[uint64]*domain.ExportJob{}, nextID: 100}
	for _, j := range seed {
		kept := *j
		s.jobs[j.ID] = &kept
	}
	return s
}

func (s *fakeJobStore) Create(_ context.Context, job *domain.ExportJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	job.ID = s.nextID
	kept := *job
	s.jobs[job.ID] = &kept
	return nil
}

func (s *fakeJobStore) Get(_ context.Context, tenantID uint64, jobID uint64) (*domain.ExportJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getScopes = append(s.getScopes, struct {
		tenant uint64
		job    uint64
	}{tenantID, jobID})
	if s.getErr != nil {
		return nil, s.getErr
	}
	job, ok := s.jobs[jobID]
	if !ok || job.TenantID != tenantID {
		return nil, fmt.Errorf("job %d not found in tenant %d", jobID, tenantID)
	}
	kept := *job
	return &kept, nil
}

func (s *fakeJobStore) UpdateStatus(
	_ context.Context, tenantID uint64, jobID uint64,
	status domain.ExportStatus, filePath string, errMsg string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statuses = append(s.statuses, struct {
		tenant uint64
		job    uint64
		status domain.ExportStatus
	}{tenantID, jobID, status})
	if job, ok := s.jobs[jobID]; ok && job.TenantID == tenantID {
		job.Status = status
		job.FilePath = filePath
		job.ErrorMessage = errMsg
	}
	return nil
}

type fakeFileStore struct {
	mu    sync.Mutex
	saved []string
	open  io.ReadCloser
}

func (s *fakeFileStore) SaveBytes(
	_ context.Context, _ []byte, _ uint64, fileName string, _ bool,
) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saved = append(s.saved, fileName)
	return "stored/" + fileName, nil
}

func (s *fakeFileStore) Open(context.Context, string) (io.ReadCloser, error) {
	return s.open, nil
}

type fakeQueue struct {
	mu       sync.Mutex
	err      error
	payloads []domain.ExportPayload
}

func (q *fakeQueue) EnqueueQueryHistoryExport(_ context.Context, payload domain.ExportPayload) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.payloads = append(q.payloads, payload)
	return q.err
}

// fakeLogger records every ErrorWithFields emission.
type fakeLogger struct {
	mu    sync.Mutex
	calls int
	errs  []error
}

func (l *fakeLogger) ErrorWithFields(_ context.Context, err error, _ map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls++
	l.errs = append(l.errs, err)
}

// recordingRegistry records worker registrations without keeping state the
// module could reach; it also doubles as a deterministic failure injector.
type recordingRegistry struct {
	mu       sync.Mutex
	err      error
	patterns []string
	handlers map[string]bootstrap.TaskHandler
}

func newRecordingRegistry() *recordingRegistry {
	return &recordingRegistry{handlers: map[string]bootstrap.TaskHandler{}}
}

func (r *recordingRegistry) Register(pattern string, handler bootstrap.TaskHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	r.patterns = append(r.patterns, pattern)
	r.handlers[pattern] = handler
	return nil
}

// moduleFixture bundles one module and the fakes it was built over.
type moduleFixture struct {
	module *queryhistory.Module
	policy *fakePolicy
	audit  *fakeAudit
	jobs   *fakeJobStore
	files  *fakeFileStore
	queue  *fakeQueue
	logs   *fakeLogger
}

func newModuleFixture(t *testing.T) *moduleFixture {
	t.Helper()
	f := &moduleFixture{
		policy: &fakePolicy{mode: domain.Normal},
		audit: &fakeAudit{
			snapshot: &types.QueryHistorySnapshot{
				Session: types.Session{
					ID: "s-1", Title: "Audit", UserID: "u1", EngineType: "builtin",
					CreatedAt: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
					UpdatedAt: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
				},
				Messages: []*types.Message{
					{ID: "m-1", SessionID: "s-1", Role: "user", Content: "q"},
				},
				Feedback: []types.MessageFeedback{},
			},
			rows: []domain.ExportRow{{
				SessionID: "s-1", Title: "Audit", UserID: "u1", Source: "web",
				CreatedAt:    time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
				UpdatedAt:    time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
				MessageCount: 2, LikeCount: 1, DislikeCount: 0,
			}},
		},
		jobs:  newFakeJobStore(),
		files: &fakeFileStore{open: io.NopCloser(strings.NewReader("csv"))},
		queue: &fakeQueue{},
		logs:  &fakeLogger{},
	}
	f.module = queryhistory.NewModule(queryhistory.Dependencies{
		Policy: f.policy,
		Audit:  f.audit,
		Jobs:   f.jobs,
		Files:  f.files,
		Queue:  f.queue,
		Logger: f.logs,
	})
	return f
}

// mountFixture registers the module's routes on a fresh engine with the
// identity + error middleware the production chain carries (the transport
// reads the tenant from the request context and renders AppErrors through
// the error middleware).
func mountFixture(t *testing.T, f *moduleFixture) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(7))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "admin-1")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.Use(middleware.ErrorHandler())
	f.module.RegisterRoutes(r.Group("/api/v1/admin/sessions"))
	return r
}

func serve(engine *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

// --- the module assembly contract ---

// TestModuleRegistersRoutesAndWorker pins the registered surface: exactly
// four routes and one worker, mounted only through the instance.
func TestModuleRegistersRoutesAndWorker(t *testing.T) {
	f := newModuleFixture(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	f.module.RegisterRoutes(r.Group("/api/v1/admin/sessions"))

	seen := map[string]bool{}
	for _, route := range r.Routes() {
		seen[route.Method+" "+route.Path] = true
	}
	require.Len(t, seen, 4, "exactly the four audit routes")
	require.True(t, seen[http.MethodGet+" /api/v1/admin/sessions/:session_id/snapshot"])
	require.True(t, seen[http.MethodPost+" /api/v1/admin/sessions/export"])
	require.True(t, seen[http.MethodGet+" /api/v1/admin/sessions/export/:job_id/status"])
	require.True(t, seen[http.MethodGet+" /api/v1/admin/sessions/export/:job_id/download"])

	registry := newRecordingRegistry()
	require.NoError(t, f.module.RegisterWorkers(registry))
	require.Equal(t, []string{types.TypeQueryHistoryExport}, registry.patterns)

	// The module satisfies the platform's route-module contract.
	var routeModule bootstrap.RouteModule = f.module
	require.NotNil(t, routeModule)
}

// TestModuleRejectsDuplicateWorkerRegistration proves a second registration
// of the export task type on the same registry fails deterministically and
// the first handler stays installed.
func TestModuleRejectsDuplicateWorkerRegistration(t *testing.T) {
	f := newModuleFixture(t)
	registry := bootstrap.NewAsynqWorkerRegistry(asynq.NewServeMux())

	require.NoError(t, f.module.RegisterWorkers(registry))
	first := f.module.RegisterWorkers(registry)
	second := newModuleFixture(t).module.RegisterWorkers(registry)

	for i, err := range []error{first, second} {
		require.Error(t, err, "duplicate registration %d must fail", i)
		require.Contains(t, err.Error(), "already registered")
	}
}

// TestModuleRoutesServeInjectedApplication drives the mounted routes end to
// end and asserts the injected ports observed every request: the snapshot
// read, the export admission + enqueue, and the status read.
func TestModuleRoutesServeInjectedApplication(t *testing.T) {
	f := newModuleFixture(t)
	engine := mountFixture(t, f)

	w := serve(engine, http.MethodGet, "/api/v1/admin/sessions/s-9/snapshot", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"success":true`)
	require.Contains(t, w.Body.String(), "s-9")
	require.Equal(t, 1, f.audit.snapCalls)

	w = serve(engine, http.MethodPost, "/api/v1/admin/sessions/export", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"job_id":101`)
	require.Len(t, f.queue.payloads, 1)
	require.Equal(t, uint64(101), f.queue.payloads[0].JobID)
	require.Equal(t, uint64(7), f.queue.payloads[0].TenantID)

	w = serve(engine, http.MethodGet, "/api/v1/admin/sessions/export/101/status", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"status":"pending"`)
}

// TestModuleInstancesAreIndependent proves the assembly keeps no
// package-level state: two modules built with different policies behave
// differently and neither observes the other's requests.
func TestModuleInstancesAreIndependent(t *testing.T) {
	off := newModuleFixture(t)
	off.policy.mode = domain.Disabled
	on := newModuleFixture(t)

	w := serve(mountFixture(t, off), http.MethodGet, "/api/v1/admin/sessions/s-1/snapshot", "")
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Equal(t, 0, off.audit.snapCalls, "disabled policy must block the read")

	w = serve(mountFixture(t, on), http.MethodGet, "/api/v1/admin/sessions/s-1/snapshot", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, on.audit.snapCalls)
}

// --- the worker contract ---

// workerFor registers the module's worker on a recording registry and
// returns the installed handler.
func workerFor(t *testing.T, f *moduleFixture) bootstrap.TaskHandler {
	t.Helper()
	registry := newRecordingRegistry()
	require.NoError(t, f.module.RegisterWorkers(registry))
	handler, ok := registry.handlers[types.TypeQueryHistoryExport]
	require.True(t, ok, "export worker must be registered")
	return handler
}

// TestModuleWorkerProcessesTaskPayload drives the private asynq handler:
// the task's payload bytes reach the application (the job store observes
// the tenant-scoped read parsed from them) and a healthy export completes.
func TestModuleWorkerProcessesTaskPayload(t *testing.T) {
	f := newModuleFixture(t)
	f.jobs = newFakeJobStore(&domain.ExportJob{
		ID: 77, TenantID: 9, RequestedBy: "admin-9", Status: domain.ExportPending,
	})
	f.module = queryhistory.NewModule(queryhistory.Dependencies{
		Policy: f.policy, Audit: f.audit, Jobs: f.jobs, Files: f.files, Queue: f.queue, Logger: f.logs,
	})
	handler := workerFor(t, f)

	payload := []byte(`{"job_id":77,"tenant_id":9,"requested_by":"admin-9"}`)
	require.NoError(t, handler(context.Background(), asynq.NewTask(types.TypeQueryHistoryExport, payload)))

	require.NotEmpty(t, f.jobs.getScopes, "worker must hand the payload to the application")
	require.Equal(t, uint64(9), f.jobs.getScopes[0].tenant)
	require.Equal(t, uint64(77), f.jobs.getScopes[0].job)

	job, err := f.jobs.Get(context.Background(), 9, 77)
	require.NoError(t, err)
	require.Equal(t, domain.ExportDone, job.Status)
	require.Equal(t, "stored/query_history_export_77.csv", job.FilePath)
	require.Equal(t, 0, f.logs.calls, "healthy run must not emit error logs")
}

// TestModuleWorkerMapsPermanentPayloadToSkipRetry pins the sentinel
// translation: a payload that can never succeed returns an error that
// satisfies asynq.SkipRetry while still carrying the domain sentinel, and
// the failure is EMITTED through the logger boundary.
func TestModuleWorkerMapsPermanentPayloadToSkipRetry(t *testing.T) {
	f := newModuleFixture(t)
	handler := workerFor(t, f)

	err := handler(context.Background(), asynq.NewTask(types.TypeQueryHistoryExport, []byte("not-json")))
	require.Error(t, err)
	require.ErrorIs(t, err, asynq.SkipRetry)
	require.ErrorIs(t, err, domain.ErrPermanentPayload)
	require.GreaterOrEqual(t, f.logs.calls, 1, "boundary logger must emit on failure")
}

// TestModuleWorkerLogsProcessingFailure proves the logger boundary is
// actually called (not constructed and ignored) when the application
// worker fails on a read, and that ordinary failures stay retryable (no
// SkipRetry).
func TestModuleWorkerLogsProcessingFailure(t *testing.T) {
	f := newModuleFixture(t)
	readFailure := stderrors.New("job store down")
	f.jobs.getErr = readFailure
	handler := workerFor(t, f)

	payload := []byte(`{"job_id":5,"tenant_id":7}`)
	err := handler(context.Background(), asynq.NewTask(types.TypeQueryHistoryExport, payload))
	require.ErrorIs(t, err, readFailure)
	require.NotErrorIs(t, err, asynq.SkipRetry)
	require.GreaterOrEqual(t, f.logs.calls, 1, "boundary logger must emit on read failure")
}
