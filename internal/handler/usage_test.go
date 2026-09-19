package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// stubUsageRepository records what each handler forwarded to the repo.
type stubUsageRepository struct {
	interfaces.UsageRepository

	queryErr error

	byUserTenant uint64
	byUserUser   string
	byUserFrom   time.Time
	byUserTo     time.Time
	byUserOut    []interfaces.UsageAggregate

	allTenant uint64
	allFrom   time.Time
	allTo     time.Time
	allLimit  int
	allOffset int
	allOut    []interfaces.UsageAggregate

	exportTenant uint64
	exportFrom   time.Time
	exportTo     time.Time
	exportOut    []interfaces.UsageAggregate
}

func (s *stubUsageRepository) AggregateByUser(
	_ context.Context, tenantID uint64, userID string, from, to time.Time,
) ([]interfaces.UsageAggregate, error) {
	s.byUserTenant, s.byUserUser, s.byUserFrom, s.byUserTo = tenantID, userID, from, to
	return s.byUserOut, s.queryErr
}

func (s *stubUsageRepository) AggregateAllUsers(
	_ context.Context, tenantID uint64, from, to time.Time, limit, offset int,
) ([]interfaces.UsageAggregate, error) {
	s.allTenant, s.allFrom, s.allTo, s.allLimit, s.allOffset = tenantID, from, to, limit, offset
	return s.allOut, s.queryErr
}

func (s *stubUsageRepository) ExportRows(
	_ context.Context, tenantID uint64, from, to time.Time,
) ([]interfaces.UsageAggregate, error) {
	s.exportTenant, s.exportFrom, s.exportTo = tenantID, from, to
	return s.exportOut, s.queryErr
}

// newUsageTestRouter mounts the three handlers with the shared error
// middleware, mirroring the production envelope rendering.
func newUsageTestRouter(t *testing.T, repo interfaces.UsageRepository) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	h := &UsageHandler{UsageRepo: repo}
	r.GET("/usage/me", h.MyUsage)
	r.GET("/admin/usage/by-user", h.AllUsers)
	r.GET("/admin/usage/export", h.Export)
	return r
}

func withUsageIdentity(req *http.Request, tenant uint64, user string) *http.Request {
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, tenant)
	if user != "" {
		ctx = context.WithValue(ctx, types.UserIDContextKey, user)
	}
	return req.WithContext(ctx)
}

// /usage/me：tenant/user 一律取自 ctx（绝不信 query），repo 只查本人窗口。
func TestUsageHandler_MyUsageScopedToCaller(t *testing.T) {
	repo := &stubUsageRepository{
		byUserOut: []interfaces.UsageAggregate{{
			WindowStart: "2026-09-18", Model: "glm-4.7", InputTokens: 100, OutputTokens: 40,
			CacheReadTokens: 5, CacheWriteTokens: 2, CostMicrocredits: 990,
		}},
	}
	router := newUsageTestRouter(t, repo)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, withUsageIdentity(httptest.NewRequest(http.MethodGet,
		"/usage/me?start_time=2026-09-01&end_time=2026-09-10", nil), uint64(7), "u-1"))

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var body struct {
		Success bool                         `json:"success"`
		Data    []interfaces.UsageAggregate `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body.Success)
	require.Len(t, body.Data, 1)
	assert.Equal(t, "2026-09-18", body.Data[0].WindowStart)
	assert.Equal(t, "glm-4.7", body.Data[0].Model)
	assert.EqualValues(t, 990, body.Data[0].CostMicrocredits)

	assert.EqualValues(t, 7, repo.byUserTenant)
	assert.Equal(t, "u-1", repo.byUserUser)
	assert.Equal(t, time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local).UTC(), repo.byUserFrom)
	// Date-only end_time covers the whole end day (parseAnalyticsRange semantics).
	assert.Equal(t, time.Date(2026, 9, 10, 0, 0, 0, 0, time.Local).UTC().Add(24*time.Hour), repo.byUserTo)
}

// /usage/me 无 user context → 403，repo 不被调用（“me”无从解析）。
func TestUsageHandler_MyUsageRequiresUserContext(t *testing.T) {
	repo := &stubUsageRepository{}
	router := newUsageTestRouter(t, repo)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, withUsageIdentity(
		httptest.NewRequest(http.MethodGet, "/usage/me", nil), uint64(7), ""))

	require.Equal(t, http.StatusForbidden, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "user context required")
	assert.Zero(t, repo.byUserTenant, "repo must not be called")
}

// /usage/me 无 tenant context → 403。
func TestUsageHandler_MyUsageRequiresTenantContext(t *testing.T) {
	repo := &stubUsageRepository{}
	router := newUsageTestRouter(t, repo)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, withUsageIdentity(
		httptest.NewRequest(http.MethodGet, "/usage/me", nil), uint64(0), "u-1"))

	require.Equal(t, http.StatusForbidden, w.Code, "body=%s", w.Body.String())
	assert.Zero(t, repo.byUserTenant, "repo must not be called")
}

// /usage/me 空结果必须渲染 "data":[]（GORM 零行返回 nil 切片，null 违反契约）。
func TestUsageHandler_MyUsageEmptyRowsRenderEmptyArray(t *testing.T) {
	repo := &stubUsageRepository{}
	router := newUsageTestRouter(t, repo)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, withUsageIdentity(
		httptest.NewRequest(http.MethodGet, "/usage/me", nil), uint64(7), "u-1"))

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), `"data":[]`)
	assert.NotContains(t, w.Body.String(), `"data":null`)
}

// by-user 分页防御：page/page_size → clamped limit(1..200 默认 50)/offset(>=0) 透传。
func TestUsageHandler_AllUsersPaginationClamp(t *testing.T) {
	for _, tc := range []struct {
		name      string
		url       string
		wantLimit int
		wantOff   int
	}{
		{"defaults", "/admin/usage/by-user", 50, 0},
		{"explicit", "/admin/usage/by-user?page=2&page_size=30", 30, 60},
		{"page_zero", "/admin/usage/by-user?page=0&page_size=25", 25, 0},
		{"negative_page", "/admin/usage/by-user?page=-5&page_size=25", 25, 0},
		{"huge_page_size", "/admin/usage/by-user?page=1&page_size=100000", 200, 200},
		{"page_size_clamp_max", "/admin/usage/by-user?page=3&page_size=200", 200, 600},
		{"zero_page_size", "/admin/usage/by-user?page=1&page_size=0", 50, 50},
		{"negative_page_size", "/admin/usage/by-user?page=1&page_size=-10", 50, 50},
		{"non_numeric", "/admin/usage/by-user?page=abc&page_size=def", 50, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubUsageRepository{}
			router := newUsageTestRouter(t, repo)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, withUsageIdentity(httptest.NewRequest(http.MethodGet, tc.url, nil), uint64(7), "ignored"))

			require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
			assert.Equal(t, tc.wantLimit, repo.allLimit, "limit")
			assert.Equal(t, tc.wantOff, repo.allOffset, "offset")
			assert.EqualValues(t, 7, repo.allTenant)
		})
	}
}

// by-user 返回行含 user_id（全员视图的用户维度）。
func TestUsageHandler_AllUsersRowsCarryUserID(t *testing.T) {
	repo := &stubUsageRepository{
		allOut: []interfaces.UsageAggregate{{UserID: "u-2", WindowStart: "2026-09-18", Model: "glm-4.7"}},
	}
	router := newUsageTestRouter(t, repo)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, withUsageIdentity(
		httptest.NewRequest(http.MethodGet, "/admin/usage/by-user", nil), uint64(7), "ignored"))

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), `"user_id":"u-2"`)
}

// export：Content-Type / Disposition / BOM / 表头 / 数据行（含空 model 直出）。
func TestUsageHandler_ExportCSVShape(t *testing.T) {
	repo := &stubUsageRepository{
		exportOut: []interfaces.UsageAggregate{
			{UserID: "u,1", InputTokens: 100, OutputTokens: 40, CacheReadTokens: 5, CacheWriteTokens: 2, CostMicrocredits: 990},
			{UserID: "u-2", Model: "", InputTokens: 7, CostMicrocredits: 3},
		},
	}
	router := newUsageTestRouter(t, repo)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, withUsageIdentity(httptest.NewRequest(http.MethodGet,
		"/admin/usage/export?start_time=2026-09-01&end_time=2026-09-10", nil), uint64(7), "ignored"))

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	assert.Equal(t, "text/csv; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "attachment; filename=usage_export.csv", w.Header().Get("Content-Disposition"))

	body := w.Body.Bytes()
	require.GreaterOrEqual(t, len(body), 3, "body must carry a BOM")
	require.Equal(t, []byte{0xEF, 0xBB, 0xBF}, body[:3], "UTF-8 BOM for Excel")

	text := string(body[3:])
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	require.Len(t, lines, 3)
	assert.Equal(t,
		"user_id,model,flow,window_start,input_tokens,output_tokens,cache_read_tokens,cache_write_tokens,cost_microcredits",
		lines[0])
	// csv.Writer quotes fields containing the delimiter; empty model /
	// window_start on export rows are emitted as empty fields verbatim.
	assert.Equal(t, `"u,1",,,,100,40,5,2,990`, lines[1])
	assert.Equal(t, `u-2,,,,7,0,0,0,3`, lines[2])
}

// repo 查询失败 → 500 envelope（三个端点一致）。
func TestUsageHandler_RepoErrorMapsTo500(t *testing.T) {
	for _, tc := range []struct{ name, path string }{
		{"me", "/usage/me"},
		{"by_user", "/admin/usage/by-user"},
		{"export", "/admin/usage/export"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubUsageRepository{queryErr: assert.AnError}
			router := newUsageTestRouter(t, repo)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, withUsageIdentity(
				httptest.NewRequest(http.MethodGet, tc.path, nil), uint64(7), "u-1"))

			require.Equal(t, http.StatusInternalServerError, w.Code, "body=%s", w.Body.String())
			var body struct {
				Success bool `json:"success"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.False(t, body.Success)
		})
	}
}

// start_time/end_time 解析失败 → 400（复用 parseAnalyticsRange 语义）。
func TestUsageHandler_InvalidTimeParams(t *testing.T) {
	for _, tc := range []struct {
		name string
		path string
		url  string
		want string
	}{
		{"me_bad_start", "/usage/me", "/usage/me?start_time=abc", "start_time"},
		{"by_user_bad_end", "/admin/usage/by-user", "/admin/usage/by-user?end_time=zzz", "end_time"},
		{"export_bad_start", "/admin/usage/export", "/admin/usage/export?start_time=not-a-date", "start_time"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubUsageRepository{}
			router := newUsageTestRouter(t, repo)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, withUsageIdentity(httptest.NewRequest(http.MethodGet, tc.url, nil), uint64(7), "u-1"))

			require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
			assert.Contains(t, w.Body.String(), tc.want)
			assert.Zero(t, repo.byUserTenant+repo.allTenant+repo.exportTenant, "repo must not be called")
		})
	}
}
