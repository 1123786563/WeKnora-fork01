package httptransport

// Handler-level contract tests for the Query History module's HTTP transport
// (Wave 1, Task 8), ported from internal/handler/session's
// query_history_admin_test.go / query_history_export_test.go: request
// parsing, the privacy-gate mapping, the job status envelope, and the
// download contract (done-only + BOM-once + streaming + headers). The RBAC
// group itself is Task 10's bridge — these tests mount the pre-guarded group
// directly on a bare engine and reuse the exported legacy error-mapping
// middleware (middleware.ErrorHandler) so AppError → status codes map
// exactly as in production.

import (
	"context"
	"encoding/json"
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
	"github.com/Tencent/WeKnora/internal/types"
)

// fakeAudit implements AuditUseCases: the export gate's CheckAccess plus the
// snapshot read, with call counters.
type fakeAudit struct {
	mode          domain.Mode
	accessErr     error
	accessCalls   int
	snapshot      *types.QueryHistorySnapshot
	snapshotErr   error
	snapshotCalls int
	lastSessionID string
}

func (f *fakeAudit) CheckAccess(_ context.Context, _ uint64) (domain.Mode, error) {
	f.accessCalls++
	return f.mode, f.accessErr
}

func (f *fakeAudit) Snapshot(_ context.Context, _ uint64, sessionID string) (*types.QueryHistorySnapshot, error) {
	f.snapshotCalls++
	f.lastSessionID = sessionID
	return f.snapshot, f.snapshotErr
}

// fakeExports implements ExportUseCases: the async CSV export submit / poll /
// download use cases, recording what the handler handed over.
type fakeExports struct {
	startedTenant uint64
	startedBy     string
	startedFilter *domain.ExportFilter
	startErr      error
	startCalls    int
	job           *domain.ExportJob
	jobErr        error
	jobCalls      int
	openReader    io.ReadCloser
	openName      string
	openErr       error
	openCalls     int
}

func (f *fakeExports) Start(
	_ context.Context, tenantID uint64, requestedBy string, filter domain.ExportFilter,
) (uint64, error) {
	f.startCalls++
	f.startedTenant = tenantID
	f.startedBy = requestedBy
	queries := filter
	f.startedFilter = &queries
	if f.startErr != nil {
		return 0, f.startErr
	}
	return 42, nil
}

func (f *fakeExports) Job(_ context.Context, _ uint64, _ uint64) (*domain.ExportJob, error) {
	f.jobCalls++
	if f.jobErr != nil {
		return nil, f.jobErr
	}
	return f.job, nil
}

func (f *fakeExports) Open(_ context.Context, _ uint64, _ uint64) (io.ReadCloser, string, error) {
	f.openCalls++
	if f.openErr != nil {
		return nil, "", f.openErr
	}
	return f.openReader, f.openName, nil
}

// newTestEngine mounts the transport on a bare engine under the legacy admin
// path, with the legacy error mapping and an identity middleware that can be
// told to omit the tenant (the missing-tenant case).
func newTestEngine(audit AuditUseCases, exports ExportUseCases, withTenant bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewHandler(audit, exports)
	engine := gin.New()
	engine.Use(middleware.ErrorHandler())
	engine.Use(func(c *gin.Context) {
		if withTenant {
			ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
			ctx = context.WithValue(ctx, types.UserIDContextKey, "admin-9")
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	})
	RegisterRoutes(engine.Group("/api/v1/admin/sessions"), h)
	return engine
}

func serve(engine *gin.Engine, method, target string, body string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func TestGetQueryHistorySnapshotHandler(t *testing.T) {
	t.Run("valid snapshot keeps the legacy envelope", func(t *testing.T) {
		audit := &fakeAudit{snapshot: &types.QueryHistorySnapshot{
			Session:   types.Session{ID: "s1", TenantID: 1, UserID: "anonymous"},
			Messages:  []*types.Message{{ID: "m1", SessionID: "s1", Role: "user"}},
			Feedback:  []types.MessageFeedback{{UserID: "anonymous", MessageID: "m1", Rating: types.FeedbackRatingLike}},
			Truncated: true,
		}}
		w := serve(newTestEngine(audit, &fakeExports{}, true),
			http.MethodGet, "/api/v1/admin/sessions/s1/snapshot", "")

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, 1, audit.snapshotCalls)
		require.Equal(t, "s1", audit.lastSessionID)

		var body struct {
			Success bool                        `json:"success"`
			Data    *types.QueryHistorySnapshot `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.True(t, body.Success)
		require.NotNil(t, body.Data)
		require.Equal(t, "s1", body.Data.Session.ID)
		require.Equal(t, "anonymous", body.Data.Session.UserID)
		require.Len(t, body.Data.Messages, 1)
		require.Len(t, body.Data.Feedback, 1)
		require.True(t, body.Data.Truncated)
	})

	t.Run("session id that sanitizes to empty is a 400", func(t *testing.T) {
		// "%01" is a bare control character: SanitizeForLog drops it, so the
		// handler sees an empty session id — the branch a raw router can only
		// reach with non-printable path input.
		audit := &fakeAudit{}
		w := serve(newTestEngine(audit, &fakeExports{}, true),
			http.MethodGet, "/api/v1/admin/sessions/%01/snapshot", "")

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), errors.ErrInvalidSessionID.Error())
		require.Equal(t, 0, audit.snapshotCalls)
	})

	t.Run("missing tenant is a 400", func(t *testing.T) {
		audit := &fakeAudit{}
		w := serve(newTestEngine(audit, &fakeExports{}, false),
			http.MethodGet, "/api/v1/admin/sessions/s1/snapshot", "")

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), errors.ErrInvalidTenantID.Error())
		require.Equal(t, 0, audit.snapshotCalls)
	})

	t.Run("error mapping keeps legacy statuses", func(t *testing.T) {
		cases := []struct {
			name     string
			err      error
			wantCode int
		}{
			{
				name:     "session miss maps to 404",
				err:      errors.ErrSessionNotFound,
				wantCode: http.StatusNotFound,
			},
			{
				name:     "policy denial keeps its 403",
				err:      errors.NewForbiddenError("query history is disabled for this tenant"),
				wantCode: http.StatusForbidden,
			},
			{
				name:     "unexpected error maps to 500",
				err:      context.DeadlineExceeded,
				wantCode: http.StatusInternalServerError,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				audit := &fakeAudit{snapshotErr: tc.err}
				w := serve(newTestEngine(audit, &fakeExports{}, true),
					http.MethodGet, "/api/v1/admin/sessions/s1/snapshot", "")

				require.Equal(t, tc.wantCode, w.Code)
				var body struct {
					Success bool `json:"success"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				require.False(t, body.Success)
			})
		}
	})
}

func TestStartQueryHistoryExportHandler(t *testing.T) {
	t.Run("empty body exports the whole tenant", func(t *testing.T) {
		audit := &fakeAudit{mode: domain.Normal}
		exports := &fakeExports{}
		w := serve(newTestEngine(audit, exports, true),
			http.MethodPost, "/api/v1/admin/sessions/export", "")

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, 1, audit.accessCalls, "the policy gate runs before admission")
		var body struct {
			Success bool `json:"success"`
			Data    struct {
				JobID uint64 `json:"job_id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.True(t, body.Success)
		require.Equal(t, uint64(42), body.Data.JobID)
		require.Equal(t, uint64(1), exports.startedTenant)
		require.Equal(t, "admin-9", exports.startedBy, "requested_by comes from the caller identity")
		require.NotNil(t, exports.startedFilter)
		require.Empty(t, exports.startedFilter.UserID)
		require.True(t, exports.startedFilter.StartTime.IsZero())
	})

	t.Run("filters ride through the body", func(t *testing.T) {
		audit := &fakeAudit{mode: domain.Normal}
		exports := &fakeExports{}
		w := serve(newTestEngine(audit, exports, true),
			http.MethodPost, "/api/v1/admin/sessions/export",
			`{"user_id":"u1","start_time":"2026-09-01T00:00:00Z","end_time":"2026-09-10","feedback":"like"}`)

		require.Equal(t, http.StatusOK, w.Code)
		require.NotNil(t, exports.startedFilter)
		require.Equal(t, "u1", exports.startedFilter.UserID)
		require.Equal(t, types.FeedbackRatingLike, exports.startedFilter.FeedbackRating)
		require.Equal(t, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
			exports.startedFilter.StartTime)
		require.False(t, exports.startedFilter.EndTime.IsZero())
	})

	t.Run("validation and policy mapping", func(t *testing.T) {
		cases := []struct {
			name     string
			body     string
			tenant   bool
			audit    *fakeAudit
			exports  *fakeExports
			wantCode int
			wantBody string
			// wantStartCalls pins whether admission was reached: validation /
			// policy rejections never call Start, a start failure by
			// definition did.
			wantStartCalls int
		}{
			{
				name:     "invalid feedback",
				body:     `{"feedback":"meh"}`,
				tenant:   true,
				audit:    &fakeAudit{},
				exports:  &fakeExports{},
				wantCode: http.StatusBadRequest,
				wantBody: "invalid feedback: must be like or dislike",
			},
			{
				name:     "invalid start_time",
				body:     `{"start_time":"not-a-date"}`,
				tenant:   true,
				audit:    &fakeAudit{},
				exports:  &fakeExports{},
				wantCode: http.StatusBadRequest,
				wantBody: "invalid start_time",
			},
			{
				name:     "invalid end_time",
				body:     `{"end_time":"not-a-date"}`,
				tenant:   true,
				audit:    &fakeAudit{},
				exports:  &fakeExports{},
				wantCode: http.StatusBadRequest,
				wantBody: "invalid end_time",
			},
			{
				name:     "malformed json is a 400, not EOF confusion",
				body:     `{bad`,
				tenant:   true,
				audit:    &fakeAudit{},
				exports:  &fakeExports{},
				wantCode: http.StatusBadRequest,
			},
			{
				name:     "missing tenant",
				body:     `{}`,
				tenant:   false,
				audit:    &fakeAudit{},
				exports:  &fakeExports{},
				wantCode: http.StatusBadRequest,
				wantBody: errors.ErrInvalidTenantID.Error(),
			},
			{
				name:     "disabled policy is 403",
				body:     `{}`,
				tenant:   true,
				audit:    &fakeAudit{accessErr: errors.NewForbiddenError("query history is disabled for this tenant")},
				exports:  &fakeExports{},
				wantCode: http.StatusForbidden,
			},
			{
				name:     "policy lookup failure is 500",
				body:     `{}`,
				tenant:   true,
				audit:    &fakeAudit{accessErr: context.DeadlineExceeded},
				exports:  &fakeExports{},
				wantCode: http.StatusInternalServerError,
			},
			{
				name:     "start failure is 500",
				body:     `{}`,
				tenant:   true,
				audit:    &fakeAudit{mode: domain.Normal},
				exports:  &fakeExports{startErr: context.DeadlineExceeded},
				wantCode: http.StatusInternalServerError,

				wantStartCalls: 1,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				w := serve(newTestEngine(tc.audit, tc.exports, tc.tenant),
					http.MethodPost, "/api/v1/admin/sessions/export", tc.body)

				require.Equal(t, tc.wantCode, w.Code)
				if tc.wantBody != "" {
					require.Contains(t, w.Body.String(), tc.wantBody)
				}
				require.Equal(t, tc.wantStartCalls, tc.exports.startCalls,
					"admission must be reached exactly when earlier checks pass")
			})
		}
	})
}

func TestGetQueryHistoryExportStatusHandler(t *testing.T) {
	serveStatus := func(audit *fakeAudit, exports *fakeExports, tenant bool, jobID string) *httptest.ResponseRecorder {
		return serve(newTestEngine(audit, exports, tenant),
			http.MethodGet, "/api/v1/admin/sessions/export/"+jobID+"/status", "")
	}

	t.Run("job reports the full status envelope", func(t *testing.T) {
		audit := &fakeAudit{mode: domain.Normal}
		exports := &fakeExports{job: &domain.ExportJob{
			ID: 7, TenantID: 1, Status: domain.ExportFailed, ErrorMessage: "boom",
			FilePath: "local://temp/query_history_export_7.csv", RequestedBy: "admin-9",
			CreatedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 9, 1, 0, 1, 0, 0, time.UTC),
		}}
		w := serveStatus(audit, exports, true, "7")

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, 1, exports.jobCalls)
		var body struct {
			Success bool `json:"success"`
			Data    struct {
				JobID        uint64 `json:"job_id"`
				Status       string `json:"status"`
				ErrorMessage string `json:"error_message"`
				FilePath     string `json:"file_path"`
				RequestedBy  string `json:"requested_by"`
				CreatedAt    string `json:"created_at"`
				UpdatedAt    string `json:"updated_at"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.True(t, body.Success)
		require.Equal(t, uint64(7), body.Data.JobID)
		require.Equal(t, domain.ExportFailed, body.Data.Status)
		require.Equal(t, "boom", body.Data.ErrorMessage)
		require.Equal(t, "local://temp/query_history_export_7.csv", body.Data.FilePath)
		require.Equal(t, "admin-9", body.Data.RequestedBy)
		require.NotEmpty(t, body.Data.CreatedAt)
		require.NotEmpty(t, body.Data.UpdatedAt)
	})

	t.Run("unknown or cross-tenant job is 404", func(t *testing.T) {
		w := serveStatus(&fakeAudit{mode: domain.Normal},
			&fakeExports{jobErr: errors.NewNotFoundError("query history export job not found")}, true, "9")
		require.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("invalid job id is 400", func(t *testing.T) {
		w := serveStatus(&fakeAudit{mode: domain.Normal}, &fakeExports{}, true, "not-a-number")
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "invalid job_id")
	})

	t.Run("zero job id is 400", func(t *testing.T) {
		w := serveStatus(&fakeAudit{mode: domain.Normal}, &fakeExports{}, true, "0")
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing tenant is 400 before any read", func(t *testing.T) {
		exports := &fakeExports{}
		w := serveStatus(&fakeAudit{}, exports, false, "7")
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Equal(t, 0, exports.jobCalls)
	})

	t.Run("disabled policy is 403", func(t *testing.T) {
		exports := &fakeExports{}
		w := serveStatus(
			&fakeAudit{accessErr: errors.NewForbiddenError("query history is disabled for this tenant")},
			exports, true, "7")
		require.Equal(t, http.StatusForbidden, w.Code)
		require.Equal(t, 0, exports.jobCalls)
	})

	t.Run("unexpected job read error is 500", func(t *testing.T) {
		w := serveStatus(&fakeAudit{mode: domain.Normal},
			&fakeExports{jobErr: context.DeadlineExceeded}, true, "7")
		require.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestDownloadQueryHistoryExportHandler(t *testing.T) {
	doneExports := func(openErr error) *fakeExports {
		return &fakeExports{
			job: &domain.ExportJob{
				ID: 7, TenantID: 1, Status: domain.ExportDone,
				FilePath: "local://temp/query_history_export_7.csv",
			},
			openReader: io.NopCloser(strings.NewReader("session_id,title\ns1,t\n")),
			openName:   "query_history_export_7.csv",
			openErr:    openErr,
		}
	}
	serveDownload := func(audit *fakeAudit, exports *fakeExports, tenant bool, jobID string) *httptest.ResponseRecorder {
		return serve(newTestEngine(audit, exports, tenant),
			http.MethodGet, "/api/v1/admin/sessions/export/"+jobID+"/download", "")
	}

	t.Run("done job streams the file with exactly one BOM and the legacy headers", func(t *testing.T) {
		audit := &fakeAudit{mode: domain.Normal}
		exports := doneExports(nil)
		w := serveDownload(audit, exports, true, "7")

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "text/csv; charset=utf-8", w.Header().Get("Content-Type"))
		require.Equal(t, "attachment; filename=query_history_export_7.csv",
			w.Header().Get("Content-Disposition"))
		require.Equal(t, "\xEF\xBB\xBFsession_id,title\ns1,t\n", w.Body.String(),
			"the BOM must precede the stored CSV bytes")
		require.Equal(t, 1, exports.openCalls)
		// BOM once, at the head, never inside the stored bytes.
		bom := "\xEF\xBB\xBF"
		require.True(t, strings.HasPrefix(w.Body.String(), bom))
		require.NotContains(t, w.Body.String()[3:], bom)
	})

	t.Run("pending job is a semantic 400", func(t *testing.T) {
		exports := &fakeExports{job: &domain.ExportJob{ID: 7, Status: domain.ExportPending}}
		w := serveDownload(&fakeAudit{mode: domain.Normal}, exports, true, "7")
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "pending")
		require.Equal(t, 0, exports.openCalls, "no storage read before done")
	})

	t.Run("done job without a file path is still not ready", func(t *testing.T) {
		exports := &fakeExports{job: &domain.ExportJob{ID: 7, Status: domain.ExportDone}}
		w := serveDownload(&fakeAudit{mode: domain.Normal}, exports, true, "7")
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Equal(t, 0, exports.openCalls)
	})

	t.Run("failed job carries its error message", func(t *testing.T) {
		exports := &fakeExports{job: &domain.ExportJob{
			ID: 7, Status: domain.ExportFailed, ErrorMessage: "disk full",
		}}
		w := serveDownload(&fakeAudit{mode: domain.Normal}, exports, true, "7")
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "disk full")
		require.Equal(t, 0, exports.openCalls)
	})

	t.Run("unknown job is 404", func(t *testing.T) {
		w := serveDownload(&fakeAudit{mode: domain.Normal},
			&fakeExports{jobErr: errors.NewNotFoundError("query history export job not found")}, true, "9")
		require.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("invalid job id is 400", func(t *testing.T) {
		w := serveDownload(&fakeAudit{mode: domain.Normal}, &fakeExports{}, true, "not-a-number")
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "invalid job_id")
	})

	t.Run("missing tenant is 400", func(t *testing.T) {
		w := serveDownload(&fakeAudit{}, &fakeExports{}, false, "7")
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), errors.ErrInvalidTenantID.Error())
	})

	t.Run("disabled policy is 403 before any job read", func(t *testing.T) {
		exports := &fakeExports{}
		w := serveDownload(
			&fakeAudit{accessErr: errors.NewForbiddenError("query history is disabled for this tenant")},
			exports, true, "7")
		require.Equal(t, http.StatusForbidden, w.Code)
		require.Equal(t, 0, exports.jobCalls)
		require.Equal(t, 0, exports.openCalls)
	})

	t.Run("storage failure is a 500 with the legacy message", func(t *testing.T) {
		exports := doneExports(context.DeadlineExceeded)
		w := serveDownload(&fakeAudit{mode: domain.Normal}, exports, true, "7")
		require.Equal(t, http.StatusInternalServerError, w.Code)
		require.Contains(t, w.Body.String(), "export file is no longer available")
	})
}
