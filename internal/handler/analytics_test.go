package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// stubAnalyticsRepository records what each handler forwarded to the repo.
type stubAnalyticsRepository struct {
	interfaces.AnalyticsRepository

	queryErr error

	trendTenant uint64
	trendFrom   time.Time
	trendTo     time.Time
	trendOut    []interfaces.QueryTrendPoint

	usersTenant uint64
	usersFrom   time.Time
	usersTo     time.Time
	usersOut    []interfaces.ActiveUsersPoint

	channelsTenant uint64
	channelsFrom   time.Time
	channelsTo     time.Time

	agentsTenant uint64
	agentsID     string
	agentsFrom   time.Time
	agentsTo     time.Time
}

func (s *stubAnalyticsRepository) QueryTrend(
	_ context.Context, tenantID uint64, from, to time.Time,
) ([]interfaces.QueryTrendPoint, error) {
	s.trendTenant, s.trendFrom, s.trendTo = tenantID, from, to
	return s.trendOut, s.queryErr
}

func (s *stubAnalyticsRepository) ActiveUsers(
	_ context.Context, tenantID uint64, from, to time.Time,
) ([]interfaces.ActiveUsersPoint, error) {
	s.usersTenant, s.usersFrom, s.usersTo = tenantID, from, to
	return s.usersOut, s.queryErr
}

func (s *stubAnalyticsRepository) ChannelSessions(
	_ context.Context, tenantID uint64, from, to time.Time,
) ([]interfaces.ChannelSessionsPoint, error) {
	s.channelsTenant, s.channelsFrom, s.channelsTo = tenantID, from, to
	return nil, s.queryErr
}

func (s *stubAnalyticsRepository) AgentMessages(
	_ context.Context, tenantID uint64, agentID string, from, to time.Time,
) ([]interfaces.AgentUsagePoint, error) {
	s.agentsTenant, s.agentsID, s.agentsFrom, s.agentsTo = tenantID, agentID, from, to
	return nil, s.queryErr
}

// newAnalyticsTestRouter mounts the four handlers with the shared error
// middleware, mirroring the production envelope rendering. The extra
// param-less /analytics/agents route exists only to exercise the empty
// agent_id branch (gin never matches an empty wildcard on the real route).
func newAnalyticsTestRouter(t *testing.T, repo interfaces.AnalyticsRepository) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	h := &AnalyticsHandler{AnalyticsRepo: repo}
	r.GET("/analytics/queries", h.QueryTrend)
	r.GET("/analytics/users", h.ActiveUsers)
	r.GET("/analytics/channels", h.ChannelSessions)
	r.GET("/analytics/agents/:agent_id", h.AgentMessages)
	r.GET("/analytics/agents", h.AgentMessages)
	return r
}

func withAnalyticsTenant(req *http.Request, tenant uint64) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), types.TenantIDContextKey, tenant))
}

// 未传时间参数 → 200 envelope，repo 收到 tenant 与 now-30d..now（UTC）窗口。
func TestAnalyticsHandler_DefaultRange30Days(t *testing.T) {
	repo := &stubAnalyticsRepository{
		trendOut: []interfaces.QueryTrendPoint{{Date: "2026-09-01", Queries: 2, Likes: 1}},
	}
	router := newAnalyticsTestRouter(t, repo)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, withAnalyticsTenant(
		httptest.NewRequest(http.MethodGet, "/analytics/queries", nil), uint64(7)))

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var body struct {
		Success bool                        `json:"success"`
		Data    []interfaces.QueryTrendPoint `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body.Success)
	require.Len(t, body.Data, 1)
	assert.EqualValues(t, 2, body.Data[0].Queries)

	assert.EqualValues(t, 7, repo.trendTenant)
	window := repo.trendTo.Sub(repo.trendFrom)
	assert.GreaterOrEqual(t, window, 29*24*time.Hour)
	assert.LessOrEqual(t, window, 31*24*time.Hour)
	assert.Equal(t, time.UTC, repo.trendFrom.Location())
	assert.Equal(t, time.UTC, repo.trendTo.Location())
}

// 显式 start_time（本地日期 layout）+ end_time（RFC3339）→ 原样（转 UTC）透传。
func TestAnalyticsHandler_ExplicitRangePassthrough(t *testing.T) {
	repo := &stubAnalyticsRepository{}
	router := newAnalyticsTestRouter(t, repo)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, withAnalyticsTenant(httptest.NewRequest(http.MethodGet,
		"/analytics/users?start_time=2026-09-01&end_time=2026-09-10T12:00:00Z", nil), uint64(7)))

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	assert.Equal(t, time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local).UTC(), repo.usersFrom)
	assert.Equal(t, time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC), repo.usersTo)
	assert.EqualValues(t, 7, repo.usersTenant)
}

// start_time / end_time 解析失败 → 400，错误信息带参数名，repo 不被调用。
func TestAnalyticsHandler_InvalidTimeParams(t *testing.T) {
	for _, tc := range []struct {
		name string
		url  string
		want string
	}{
		{"invalid_start", "/analytics/queries?start_time=abc", "start_time"},
		{"invalid_end", "/analytics/queries?end_time=not-a-date", "end_time"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubAnalyticsRepository{}
			router := newAnalyticsTestRouter(t, repo)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, withAnalyticsTenant(
				httptest.NewRequest(http.MethodGet, tc.url, nil), uint64(7)))

			require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
			assert.Contains(t, w.Body.String(), tc.want)
			assert.Zero(t, repo.trendTenant, "repo must not be called")
		})
	}
}

// 无 tenant context → 403，repo 不被调用。
func TestAnalyticsHandler_TenantRequired(t *testing.T) {
	repo := &stubAnalyticsRepository{}
	router := newAnalyticsTestRouter(t, repo)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/analytics/queries", nil))

	require.Equal(t, http.StatusForbidden, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "tenant context required")
	assert.Zero(t, repo.trendTenant, "repo must not be called")
}

// AgentMessages 透传 agent_id 与时间窗口。
func TestAnalyticsHandler_AgentMessagesPassthrough(t *testing.T) {
	repo := &stubAnalyticsRepository{}
	router := newAnalyticsTestRouter(t, repo)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, withAnalyticsTenant(httptest.NewRequest(http.MethodGet,
		"/analytics/agents/agent-42?start_time=2026-09-01&end_time=2026-09-02", nil), uint64(7)))

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var body struct {
		Success bool `json:"success"`
		Data    []interfaces.AgentUsagePoint `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body.Success)
	assert.Empty(t, body.Data)

	assert.Equal(t, "agent-42", repo.agentsID)
	assert.EqualValues(t, 7, repo.agentsTenant)
	assert.Equal(t, time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local).UTC(), repo.agentsFrom)
	assert.Equal(t, time.Date(2026, 9, 2, 0, 0, 0, 0, time.Local).UTC(), repo.agentsTo)
}

// :agent_id 为空串 → 400。
func TestAnalyticsHandler_AgentMessagesEmptyAgentID(t *testing.T) {
	repo := &stubAnalyticsRepository{}
	router := newAnalyticsTestRouter(t, repo)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, withAnalyticsTenant(
		httptest.NewRequest(http.MethodGet, "/analytics/agents", nil), uint64(7)))

	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "agent_id")
	assert.Zero(t, repo.agentsTenant, "repo must not be called")
}

// repo 查询失败 → 500 envelope（不泄露底层错误细节给客户端）。
func TestAnalyticsHandler_RepoErrorMapsTo500(t *testing.T) {
	repo := &stubAnalyticsRepository{queryErr: assert.AnError}
	router := newAnalyticsTestRouter(t, repo)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, withAnalyticsTenant(
		httptest.NewRequest(http.MethodGet, "/analytics/channels", nil), uint64(7)))

	require.Equal(t, http.StatusInternalServerError, w.Code, "body=%s", w.Body.String())
	var body struct {
		Success bool `json:"success"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.False(t, body.Success)
}
