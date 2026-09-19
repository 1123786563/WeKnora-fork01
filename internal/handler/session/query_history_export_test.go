package session

// Handler-level tests for the Admin+ async query-history export endpoints
// (SP13 Task 4): request validation, the privacy gate mapping, the job
// status envelope, and the download contract (done-only + BOM + streaming).
// The Admin role gate and full-access API-key policy live on the route
// registration (routes_query_history.go), pinned by the router test.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// stubQueryHistoryExporter records what the handler handed to the service.
type stubQueryHistoryExporter struct {
	mode          string
	accessErr     error
	startedFilter *types.SessionListQuery
	startedBy     string
	startErr      error
	job           *types.QueryHistoryExportJob
	getErr        error
}

func (s *stubQueryHistoryExporter) CheckAccess(
	_ context.Context, _ uint64,
) (string, error) {
	return s.mode, s.accessErr
}

func (s *stubQueryHistoryExporter) StartExport(
	_ context.Context, _ uint64, requestedBy string, filter types.SessionListQuery,
) (uint64, error) {
	s.startedBy = requestedBy
	queries := filter
	s.startedFilter = &queries
	if s.startErr != nil {
		return 0, s.startErr
	}
	return 42, nil
}

func (s *stubQueryHistoryExporter) GetExportJob(
	_ context.Context, _ uint64, _ uint64,
) (*types.QueryHistoryExportJob, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.job, nil
}

// stubDownloadFileService serves one stored CSV body.
type stubDownloadFileService struct {
	interfaces.FileService
	body     string
	getCalls int
	err      error
}

func (f *stubDownloadFileService) GetFile(_ context.Context, _ string) (io.ReadCloser, error) {
	f.getCalls++
	if f.err != nil {
		return nil, f.err
	}
	return io.NopCloser(strings.NewReader(f.body)), nil
}

func newExportHandlerEnv(
	t *testing.T, exporter queryHistoryExporter, files interfaces.FileService,
) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := &Handler{queryHistoryExport: exporter, fileService: files}
	engine := gin.New()
	engine.Use(middleware.ErrorHandler())
	engine.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "admin-9")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	engine.POST("/api/v1/admin/sessions/export", h.StartQueryHistoryExport)
	engine.GET("/api/v1/admin/sessions/export/:job_id/status", h.GetQueryHistoryExportStatus)
	engine.GET("/api/v1/admin/sessions/export/:job_id/download", h.DownloadQueryHistoryExport)
	return engine
}

func TestStartQueryHistoryExportHandler(t *testing.T) {
	t.Run("empty body exports the whole tenant", func(t *testing.T) {
		exporter := &stubQueryHistoryExporter{mode: types.QueryHistoryModeNormal}
		w := httptest.NewRecorder()
		engine := newExportHandlerEnv(t, exporter, nil)
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/admin/sessions/export", nil))

		require.Equal(t, http.StatusOK, w.Code)
		var body struct {
			Success bool `json:"success"`
			Data    struct {
				JobID uint64 `json:"job_id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.True(t, body.Success)
		require.Equal(t, uint64(42), body.Data.JobID)
		require.Equal(t, "admin-9", exporter.startedBy, "requested_by comes from the caller identity")
		require.NotNil(t, exporter.startedFilter)
		require.Empty(t, exporter.startedFilter.UserID)
		require.True(t, exporter.startedFilter.StartTime.IsZero())
	})

	t.Run("filters ride through the body", func(t *testing.T) {
		exporter := &stubQueryHistoryExporter{}
		w := httptest.NewRecorder()
		engine := newExportHandlerEnv(t, exporter, nil)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/sessions/export", strings.NewReader(
			`{"user_id":"u1","start_time":"2026-09-01T00:00:00Z","end_time":"2026-09-10","feedback":"like"}`,
		))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.NotNil(t, exporter.startedFilter)
		require.Equal(t, "u1", exporter.startedFilter.UserID)
		require.Equal(t, types.FeedbackRatingLike, exporter.startedFilter.FeedbackRating)
		require.Equal(t, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
			exporter.startedFilter.StartTime)
		require.False(t, exporter.startedFilter.EndTime.IsZero())
	})

	t.Run("validation and policy mapping", func(t *testing.T) {
		cases := []struct {
			name     string
			body     string
			exporter *stubQueryHistoryExporter
			wantCode int
		}{
			{
				name:     "invalid feedback",
				body:     `{"feedback":"meh"}`,
				exporter: &stubQueryHistoryExporter{},
				wantCode: http.StatusBadRequest,
			},
			{
				name:     "invalid start_time",
				body:     `{"start_time":"not-a-date"}`,
				exporter: &stubQueryHistoryExporter{},
				wantCode: http.StatusBadRequest,
			},
			{
				name:     "malformed json is a 400, not EOF confusion",
				body:     `{bad`,
				exporter: &stubQueryHistoryExporter{},
				wantCode: http.StatusBadRequest,
			},
			{
				name:     "disabled policy is 403",
				body:     `{}`,
				exporter: &stubQueryHistoryExporter{accessErr: errors.NewForbiddenError("query history is disabled for this tenant")},
				wantCode: http.StatusForbidden,
			},
			{
				name:     "start failure is 500",
				body:     `{}`,
				exporter: &stubQueryHistoryExporter{startErr: context.DeadlineExceeded},
				wantCode: http.StatusInternalServerError,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				w := httptest.NewRecorder()
				engine := newExportHandlerEnv(t, tc.exporter, nil)
				req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/sessions/export",
					strings.NewReader(tc.body))
				req.Header.Set("Content-Type", "application/json")
				engine.ServeHTTP(w, req)
				require.Equal(t, tc.wantCode, w.Code)
			})
		}
	})
}

func TestGetQueryHistoryExportStatusHandler(t *testing.T) {
	newEngine := func(exporter *stubQueryHistoryExporter) *gin.Engine {
		return newExportHandlerEnv(t, exporter, nil)
	}
	serve := func(engine *gin.Engine, jobID string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet,
			"/api/v1/admin/sessions/export/"+jobID+"/status", nil))
		return w
	}

	t.Run("running job reports status and error message", func(t *testing.T) {
		exporter := &stubQueryHistoryExporter{job: &types.QueryHistoryExportJob{
			ID: 7, TenantID: 1, Status: types.QueryHistoryExportFailed, ErrorMessage: "boom",
		}}
		w := serve(newEngine(exporter), "7")
		require.Equal(t, http.StatusOK, w.Code)
		var body struct {
			Success bool `json:"success"`
			Data    struct {
				Status       string `json:"status"`
				ErrorMessage string `json:"error_message"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.True(t, body.Success)
		require.Equal(t, types.QueryHistoryExportFailed, body.Data.Status)
		require.Equal(t, "boom", body.Data.ErrorMessage)
	})

	t.Run("unknown or cross-tenant job is 404", func(t *testing.T) {
		exporter := &stubQueryHistoryExporter{getErr: errors.NewNotFoundError("query history export job not found")}
		w := serve(newEngine(exporter), "9")
		require.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("invalid job id is 400", func(t *testing.T) {
		w := serve(newEngine(&stubQueryHistoryExporter{}), "not-a-number")
		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestDownloadQueryHistoryExportHandler(t *testing.T) {
	serve := func(exporter *stubQueryHistoryExporter, files *stubDownloadFileService, jobID string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		newExportHandlerEnv(t, exporter, files).ServeHTTP(w, httptest.NewRequest(http.MethodGet,
			"/api/v1/admin/sessions/export/"+jobID+"/download", nil))
		return w
	}

	t.Run("done job streams the file with a BOM", func(t *testing.T) {
		exporter := &stubQueryHistoryExporter{job: &types.QueryHistoryExportJob{
			ID: 7, TenantID: 1, Status: types.QueryHistoryExportDone,
			FilePath: "local://temp/query_history_export_7.csv",
		}}
		files := &stubDownloadFileService{body: "session_id,title\ns1,t\n"}
		w := serve(exporter, files, "7")

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "text/csv; charset=utf-8", w.Header().Get("Content-Type"))
		require.Equal(t, "attachment; filename=query_history_export_7.csv",
			w.Header().Get("Content-Disposition"))
		require.Equal(t, "\xEF\xBB\xBFsession_id,title\ns1,t\n", w.Body.String(),
			"the BOM must precede the stored CSV bytes")
		require.Equal(t, 1, files.getCalls)
	})

	t.Run("pending job is a semantic 400", func(t *testing.T) {
		exporter := &stubQueryHistoryExporter{job: &types.QueryHistoryExportJob{
			ID: 7, Status: types.QueryHistoryExportPending,
		}}
		files := &stubDownloadFileService{}
		w := serve(exporter, files, "7")
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "pending")
		require.Equal(t, 0, files.getCalls, "no storage read before done")
	})

	t.Run("failed job carries its error message", func(t *testing.T) {
		exporter := &stubQueryHistoryExporter{job: &types.QueryHistoryExportJob{
			ID: 7, Status: types.QueryHistoryExportFailed, ErrorMessage: "disk full",
		}}
		w := serve(exporter, &stubDownloadFileService{}, "7")
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "disk full")
	})

	t.Run("unknown job is 404", func(t *testing.T) {
		exporter := &stubQueryHistoryExporter{getErr: errors.NewNotFoundError("query history export job not found")}
		w := serve(exporter, &stubDownloadFileService{}, "9")
		require.Equal(t, http.StatusNotFound, w.Code)
	})
}
