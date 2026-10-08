package session

// HTTP-layer old-vs-new differential gate (Wave 1, Task 9, brief Step 4).
// The LEGACY handler (this package, constructed through its private fields)
// and the NEW Query History transport (httptransport over fake ports) receive
// the SAME httptest request per scenario; the normalized observations must be
// identical (testkit.Compare). The four transport-only headers generated per
// request (X-Request-Id via middleware.RequestID, Date, X-Trace-Id,
// X-Response-Time) are the only header noise, dropped by the allowlisted
// normalization — everything else, including the BOM, the filename header,
// and every error envelope byte, is significant.
//
// Scenario list (brief Step 4, verbatim):
//
//	snapshot normal / anonymized / disabled / missing and foreign tenant
//	export empty body / filtered by user/time/feedback
//	invalid time, feedback, and job ID
//	status pending/running/done/failed
//	download not ready, missing file, and completed CSV

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/testkit"
	httptransport "github.com/Tencent/WeKnora/internal/conversation/queryhistory/transport/http"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// Fixed fixtures: one tenant, one admin caller, one frozen clock. Every
// timestamp that reaches a response body comes from diffClock, so both stacks
// render identical bytes.
const (
	diffTenantID = uint64(1)
	diffCallerID = "admin-9"
)

var diffClock = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

// diffSnapshotFixture builds one audit snapshot whose owner field carries
// userID ("u1" for normal, "anonymous" for anonymized).
func diffSnapshotFixture(userID string) *types.QueryHistorySnapshot {
	return &types.QueryHistorySnapshot{
		Session: types.Session{
			ID: "diff-s1", TenantID: diffTenantID, Title: "Audit",
			UserID: userID, EngineType: "builtin",
			CreatedAt: diffClock, UpdatedAt: diffClock,
		},
		Messages: []*types.Message{
			{ID: "dm-1", SessionID: "diff-s1", RequestID: "r1", Role: "user", Content: "q", CreatedAt: diffClock},
			{ID: "dm-2", SessionID: "diff-s1", RequestID: "r2", Role: "assistant", Content: "a", CreatedAt: diffClock},
		},
		Feedback: []types.MessageFeedback{{
			ID: 1, TenantID: diffTenantID, UserID: userID, MessageID: "dm-1",
			SessionID: "diff-s1", Rating: types.FeedbackRatingLike, CreatedAt: diffClock,
		}},
	}
}

// diffJobFixture builds one export job at a fixed clock instant.
func diffJobFixture(status, filePath, errMsg string) *types.QueryHistoryExportJob {
	return &types.QueryHistoryExportJob{
		ID: 7, TenantID: diffTenantID, RequestedBy: diffCallerID,
		Status: status, FilePath: filePath, ErrorMessage: errMsg,
		CreatedAt: diffClock, UpdatedAt: diffClock,
	}
}

// --- legacy-side doubles (private-field wiring, same package) ---

// diffSnapshotService stubs the legacy snapshot entrance.
type diffSnapshotService struct {
	interfaces.SessionService
	snapshot *types.QueryHistorySnapshot
	err      error
}

func (s *diffSnapshotService) GetQueryHistorySnapshot(
	context.Context, uint64, string,
) (*types.QueryHistorySnapshot, error) {
	return s.snapshot, s.err
}

// diffLegacyExporter stubs the legacy export service port.
type diffLegacyExporter struct {
	mode          string
	accessErr     error
	startErr      error
	job           *types.QueryHistoryExportJob
	getErr        error
	startedBy     string
	startedFilter *types.SessionListQuery
}

func (e *diffLegacyExporter) CheckAccess(context.Context, uint64) (string, error) {
	return e.mode, e.accessErr
}

func (e *diffLegacyExporter) StartExport(
	_ context.Context, _ uint64, requestedBy string, filter types.SessionListQuery,
) (uint64, error) {
	e.startedBy = requestedBy
	kept := filter
	e.startedFilter = &kept
	if e.startErr != nil {
		return 0, e.startErr
	}
	return 42, nil
}

func (e *diffLegacyExporter) GetExportJob(
	context.Context, uint64, uint64,
) (*types.QueryHistoryExportJob, error) {
	if e.getErr != nil {
		return nil, e.getErr
	}
	return e.job, nil
}

// diffLegacyFiles stubs the legacy download file read.
type diffLegacyFiles struct {
	interfaces.FileService
	body     string
	err      error
	getCalls int
}

func (f *diffLegacyFiles) GetFile(context.Context, string) (io.ReadCloser, error) {
	f.getCalls++
	if f.err != nil {
		return nil, f.err
	}
	return io.NopCloser(strings.NewReader(f.body)), nil
}

// --- new-side fakes (the transport's own consumer interfaces) ---

// diffNewAudit fakes AuditUseCases.
type diffNewAudit struct {
	mode        domain.Mode
	snapshot    *types.QueryHistorySnapshot
	snapshotErr error
}

func (a *diffNewAudit) CheckAccess(context.Context, uint64) (domain.Mode, error) {
	return a.mode, nil
}

func (a *diffNewAudit) Snapshot(
	context.Context, uint64, string,
) (*types.QueryHistorySnapshot, error) {
	return a.snapshot, a.snapshotErr
}

// diffNewExports fakes ExportUseCases.
type diffNewExports struct {
	startErr      error
	job           *domain.ExportJob
	openReader    io.ReadCloser
	openName      string
	openErr       error
	startedBy     string
	startedFilter *domain.ExportFilter
	openCalls     int
}

func (e *diffNewExports) Start(
	_ context.Context, _ uint64, requestedBy string, filter domain.ExportFilter,
) (uint64, error) {
	e.startedBy = requestedBy
	kept := filter
	e.startedFilter = &kept
	if e.startErr != nil {
		return 0, e.startErr
	}
	return 42, nil
}

func (e *diffNewExports) Job(context.Context, uint64, uint64) (*domain.ExportJob, error) {
	return e.job, nil
}

func (e *diffNewExports) Open(context.Context, uint64, uint64) (io.ReadCloser, string, error) {
	e.openCalls++
	if e.openErr != nil {
		return nil, "", e.openErr
	}
	return e.openReader, e.openName, nil
}

// --- engines and observation helpers ---

// diffIdentity stamps the fixed tenant and caller onto the request context.
func diffIdentity() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, diffTenantID)
		ctx = context.WithValue(ctx, types.UserIDContextKey, diffCallerID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// legacyDiffEngine mounts the legacy handlers on a bare engine. The middleware
// chain mirrors production noise: RequestID generates a per-request
// X-Request-Id (different on every request — the normalization allowlist is
// exercised for real in every scenario), the identity middleware stamps the
// fixed tenant/caller, and the platform ErrorHandler renders AppErrors.
func legacyDiffEngine(
	snapshot interfaces.SessionService, exporter queryHistoryExporter, files interfaces.FileService,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &Handler{sessionService: snapshot, queryHistoryExport: exporter, fileService: files}
	engine := gin.New()
	engine.Use(middleware.RequestID())
	engine.Use(diffIdentity())
	engine.Use(middleware.ErrorHandler())
	engine.GET("/api/v1/admin/sessions/:session_id/snapshot", h.GetQueryHistorySnapshot)
	engine.POST("/api/v1/admin/sessions/export", h.StartQueryHistoryExport)
	engine.GET("/api/v1/admin/sessions/export/:job_id/status", h.GetQueryHistoryExportStatus)
	engine.GET("/api/v1/admin/sessions/export/:job_id/download", h.DownloadQueryHistoryExport)
	return engine
}

// newDiffEngine mounts the module transport on the same paths with the same
// middleware chain.
func newDiffEngine(audit httptransport.AuditUseCases, exports httptransport.ExportUseCases) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := httptransport.NewHandler(audit, exports)
	engine := gin.New()
	engine.Use(middleware.RequestID())
	engine.Use(diffIdentity())
	engine.Use(middleware.ErrorHandler())
	httptransport.RegisterRoutes(engine.Group("/api/v1/admin/sessions"), h)
	return engine
}

// diffServe sends one fresh request to one engine and captures the
// observation.
func diffServe(engine *gin.Engine, method, path, body string) testkit.Observation {
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
	return testkit.ObserveHTTP(w.Code, w.Header(), w.Body.Bytes())
}

// diffCheck fails the scenario on any divergence.
func diffCheck(t *testing.T, scenario string, legacy, modern testkit.Observation) {
	t.Helper()
	if err := testkit.Compare(legacy, modern); err != nil {
		t.Fatalf("differential %s: legacy/new divergence: %v", scenario, err)
	}
}

// diffCheckFilters proves both handlers handed the SAME audit window to their
// use cases (the HTTP observation alone cannot see this).
func diffCheckFilters(t *testing.T, legacy *diffLegacyExporter, modern *diffNewExports) {
	t.Helper()
	require.NotNil(t, legacy.startedFilter, "legacy handler must have called StartExport")
	require.NotNil(t, modern.startedFilter, "new handler must have called Start")
	require.Equal(t, legacy.startedBy, modern.startedBy, "requested_by identity")
	require.Equal(t, legacy.startedFilter.UserID, modern.startedFilter.UserID, "user filter")
	require.Equal(t, legacy.startedFilter.StartTime, modern.startedFilter.StartTime, "start_time filter")
	require.Equal(t, legacy.startedFilter.EndTime, modern.startedFilter.EndTime, "end_time filter")
	require.Equal(t, legacy.startedFilter.FeedbackRating, modern.startedFilter.FeedbackRating, "feedback filter")
}

// TestQueryHistoryDifferentialSnapshot covers the snapshot endpoint: normal,
// anonymized, disabled, and the missing / foreign-tenant reads (the tenant
// scope rides the service lookup, so both answer the session-not-found
// sentinel).
func TestQueryHistoryDifferentialSnapshot(t *testing.T) {
	run := func(t *testing.T, name string, snapshot *types.QueryHistorySnapshot, svcErr error) {
		t.Helper()
		legacy := diffServe(
			legacyDiffEngine(&diffSnapshotService{snapshot: snapshot, err: svcErr}, nil, nil),
			http.MethodGet, "/api/v1/admin/sessions/diff-s1/snapshot", "")
		modern := diffServe(
			newDiffEngine(
				&diffNewAudit{mode: domain.Normal, snapshot: snapshot, snapshotErr: svcErr},
				&diffNewExports{}),
			http.MethodGet, "/api/v1/admin/sessions/diff-s1/snapshot", "")
		diffCheck(t, name, legacy, modern)
	}

	t.Run("normal keeps owner identities", func(t *testing.T) {
		run(t, "snapshot normal", diffSnapshotFixture("u1"), nil)
	})
	t.Run("anonymized masks owner identities", func(t *testing.T) {
		run(t, "snapshot anonymized", diffSnapshotFixture("anonymous"), nil)
	})
	t.Run("disabled policy is a 403", func(t *testing.T) {
		run(t, "snapshot disabled", nil,
			errors.NewForbiddenError("query history is disabled for this tenant"))
	})
	t.Run("missing session is a 404", func(t *testing.T) {
		run(t, "snapshot missing", nil, errors.ErrSessionNotFound)
	})
	t.Run("foreign tenant session is the same 404", func(t *testing.T) {
		// A foreign tenant's session is indistinguishable from a missing one
		// at the service seam; the transport must map both identically.
		run(t, "snapshot foreign tenant", nil, errors.ErrSessionNotFound)
	})
}

// TestQueryHistoryDifferentialExport covers the export submission: the empty
// body, the user/time/feedback filter window, and the invalid time / feedback
// / job-id rejections.
func TestQueryHistoryDifferentialExport(t *testing.T) {
	run := func(t *testing.T, name, body string, expectStart bool) (
		*diffLegacyExporter, *diffNewExports,
	) {
		t.Helper()
		legacyExporter := &diffLegacyExporter{mode: types.QueryHistoryModeNormal}
		legacy := diffServe(legacyDiffEngine(nil, legacyExporter, nil),
			http.MethodPost, "/api/v1/admin/sessions/export", body)
		modernFake := &diffNewExports{}
		modern := diffServe(newDiffEngine(&diffNewAudit{mode: domain.Normal}, modernFake),
			http.MethodPost, "/api/v1/admin/sessions/export", body)
		diffCheck(t, name, legacy, modern)
		if expectStart {
			diffCheckFilters(t, legacyExporter, modernFake)
		} else {
			require.Nil(t, legacyExporter.startedFilter, "legacy must reject before StartExport")
			require.Nil(t, modernFake.startedFilter, "new must reject before Start")
		}
		return legacyExporter, modernFake
	}

	t.Run("empty body exports the whole tenant", func(t *testing.T) {
		run(t, "export empty body", "", true)
	})
	t.Run("filtered by user/time/feedback", func(t *testing.T) {
		run(t, "export filtered",
			`{"user_id":"u1","start_time":"2026-09-01T00:00:00Z","end_time":"2026-09-10","feedback":"like"}`,
			true)
	})
	t.Run("invalid start_time is a 400", func(t *testing.T) {
		run(t, "invalid time", `{"start_time":"not-a-date"}`, false)
	})
	t.Run("invalid end_time is a 400", func(t *testing.T) {
		run(t, "invalid time end", `{"end_time":"yesterday"}`, false)
	})
	t.Run("invalid feedback is a 400", func(t *testing.T) {
		run(t, "invalid feedback", `{"feedback":"meh"}`, false)
	})
	t.Run("invalid job id on status is a 400", func(t *testing.T) {
		legacy := diffServe(legacyDiffEngine(nil, &diffLegacyExporter{mode: types.QueryHistoryModeNormal}, nil),
			http.MethodGet, "/api/v1/admin/sessions/export/not-a-number/status", "")
		modern := diffServe(newDiffEngine(&diffNewAudit{mode: domain.Normal}, &diffNewExports{}),
			http.MethodGet, "/api/v1/admin/sessions/export/not-a-number/status", "")
		diffCheck(t, "invalid job id status", legacy, modern)
	})
	t.Run("invalid job id on download is a 400", func(t *testing.T) {
		legacy := diffServe(legacyDiffEngine(nil, &diffLegacyExporter{mode: types.QueryHistoryModeNormal}, nil),
			http.MethodGet, "/api/v1/admin/sessions/export/0/download", "")
		modern := diffServe(newDiffEngine(&diffNewAudit{mode: domain.Normal}, &diffNewExports{}),
			http.MethodGet, "/api/v1/admin/sessions/export/0/download", "")
		diffCheck(t, "invalid job id download", legacy, modern)
	})
}

// TestQueryHistoryDifferentialStatus covers the job status endpoint across the
// full lifecycle. The envelope carries the fixed-clock timestamps, so the
// bodies compare byte for byte after key-order normalization.
func TestQueryHistoryDifferentialStatus(t *testing.T) {
	cases := []struct {
		name string
		job  *types.QueryHistoryExportJob
	}{
		{"pending", diffJobFixture(types.QueryHistoryExportPending, "", "")},
		{"running", diffJobFixture(types.QueryHistoryExportRunning, "", "")},
		{"done", diffJobFixture(types.QueryHistoryExportDone, "local://temp/query_history_export_7.csv", "")},
		{"failed", diffJobFixture(types.QueryHistoryExportFailed, "", "store export file: disk full")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			legacy := diffServe(
				legacyDiffEngine(nil, &diffLegacyExporter{mode: types.QueryHistoryModeNormal, job: tc.job}, nil),
				http.MethodGet, "/api/v1/admin/sessions/export/7/status", "")
			modern := diffServe(
				newDiffEngine(&diffNewAudit{mode: domain.Normal}, &diffNewExports{job: tc.job}),
				http.MethodGet, "/api/v1/admin/sessions/export/7/status", "")
			diffCheck(t, "status "+tc.name, legacy, modern)
		})
	}
}

// TestQueryHistoryDifferentialDownload covers the download endpoint: the
// not-ready guard, the vanished file, and the completed CSV (status, headers,
// BOM, filename, and all CSV bytes compared).
func TestQueryHistoryDifferentialDownload(t *testing.T) {
	t.Run("not ready is a 400 and never touches storage", func(t *testing.T) {
		job := diffJobFixture(types.QueryHistoryExportPending, "", "")
		legacyFiles := &diffLegacyFiles{}
		legacy := diffServe(
			legacyDiffEngine(nil, &diffLegacyExporter{mode: types.QueryHistoryModeNormal, job: job}, legacyFiles),
			http.MethodGet, "/api/v1/admin/sessions/export/7/download", "")
		modernFake := &diffNewExports{job: job}
		modern := diffServe(newDiffEngine(&diffNewAudit{mode: domain.Normal}, modernFake),
			http.MethodGet, "/api/v1/admin/sessions/export/7/download", "")
		diffCheck(t, "download not ready", legacy, modern)
		require.Equal(t, 0, legacyFiles.getCalls)
		require.Equal(t, 0, modernFake.openCalls)
	})

	t.Run("failed job carries its error message", func(t *testing.T) {
		job := diffJobFixture(types.QueryHistoryExportFailed, "", "store export file: disk full")
		legacy := diffServe(
			legacyDiffEngine(nil, &diffLegacyExporter{mode: types.QueryHistoryModeNormal, job: job}, &diffLegacyFiles{}),
			http.MethodGet, "/api/v1/admin/sessions/export/7/download", "")
		modern := diffServe(newDiffEngine(&diffNewAudit{mode: domain.Normal}, &diffNewExports{job: job}),
			http.MethodGet, "/api/v1/admin/sessions/export/7/download", "")
		diffCheck(t, "download failed job", legacy, modern)
	})

	t.Run("missing file is a 500", func(t *testing.T) {
		job := diffJobFixture(types.QueryHistoryExportDone, "local://temp/query_history_export_7.csv", "")
		legacy := diffServe(
			legacyDiffEngine(nil,
				&diffLegacyExporter{mode: types.QueryHistoryModeNormal, job: job},
				&diffLegacyFiles{err: errors.NewInternalServerError("object gone")}),
			http.MethodGet, "/api/v1/admin/sessions/export/7/download", "")
		modern := diffServe(newDiffEngine(&diffNewAudit{mode: domain.Normal},
			&diffNewExports{job: job, openErr: errors.NewInternalServerError("object gone")}),
			http.MethodGet, "/api/v1/admin/sessions/export/7/download", "")
		diffCheck(t, "download missing file", legacy, modern)
	})

	t.Run("completed CSV streams BOM plus stored bytes with the filename", func(t *testing.T) {
		job := diffJobFixture(types.QueryHistoryExportDone, "local://temp/query_history_export_7.csv", "")
		csv := "session_id,title,user_id,source,engine_type,created_at,updated_at,message_count,like_count,dislike_count\n" +
			"diff-s1,Audit,u1,web,builtin,2026-09-01T12:00:00Z,2026-09-01T12:00:00Z,2,1,0\n"

		legacyFiles := &diffLegacyFiles{body: csv}
		legacy := diffServe(
			legacyDiffEngine(nil, &diffLegacyExporter{mode: types.QueryHistoryModeNormal, job: job}, legacyFiles),
			http.MethodGet, "/api/v1/admin/sessions/export/7/download", "")
		modern := diffServe(newDiffEngine(&diffNewAudit{mode: domain.Normal},
			&diffNewExports{
				job:        job,
				openReader: io.NopCloser(strings.NewReader(csv)),
				openName:   "query_history_export_7.csv",
			}),
			http.MethodGet, "/api/v1/admin/sessions/export/7/download", "")
		diffCheck(t, "download completed csv", legacy, modern)

		// The contract details the observation comparison already pinned;
		// these assertions document them for the reader.
		require.Equal(t, 1, legacyFiles.getCalls)
		require.Equal(t, http.StatusOK, legacy.Status)
		require.Equal(t, "text/csv; charset=utf-8", legacy.Headers["Content-Type"][0])
		require.Equal(t, "attachment; filename=query_history_export_7.csv",
			legacy.Headers["Content-Disposition"][0])
		require.True(t, strings.HasPrefix(string(legacy.Body), "\xEF\xBB\xBF"), "BOM precedes the CSV")
		require.True(t, strings.HasSuffix(string(legacy.Body), csv))
	})
}
