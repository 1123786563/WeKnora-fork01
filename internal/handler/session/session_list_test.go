package session

// Audit-listing parameter tests for GET /sessions: the user_id / start_time /
// end_time / feedback filters parse into the SessionListQuery (400 on bad
// values), and a policy denial from the service keeps its own HTTP status.

import (
	"context"
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

type recordingListSessions struct {
	interfaces.SessionService
	captured *types.SessionListQuery
	err      error
}

func (s *recordingListSessions) ListSessions(
	_ context.Context, query *types.SessionListQuery,
) (*types.PageResult, error) {
	s.captured = query
	if s.err != nil {
		return nil, s.err
	}
	return types.NewPageResult(0, &types.Pagination{Page: 1, PageSize: 20}, []*types.SessionListItem{}), nil
}

func newSessionListEnv(t *testing.T, svc interfaces.SessionService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := &Handler{sessionService: svc}
	engine := gin.New()
	engine.Use(middleware.ErrorHandler())
	engine.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	engine.GET("/api/v1/sessions", h.GetSessionsByTenant)
	return engine
}

func serveSessionList(t *testing.T, engine *gin.Engine, query string) *httptest.ResponseRecorder {
	t.Helper()
	if query != "" && !strings.HasPrefix(query, "?") {
		query = "?" + query
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions"+query, nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func TestGetSessionsByTenantParsesAuditFilters(t *testing.T) {
	svc := &recordingListSessions{}
	w := serveSessionList(t, newSessionListEnv(t, svc),
		"?source=all&user_id=alice&feedback=like"+
			"&start_time=2026-09-01T00:00:00Z&end_time=2026-09-19")
	require.Equal(t, http.StatusOK, w.Code)

	require.NotNil(t, svc.captured)
	q := svc.captured
	require.Equal(t, "all", q.Source)
	require.Equal(t, "alice", q.UserID)
	require.Equal(t, types.FeedbackRatingLike, q.FeedbackRating)
	require.Equal(t, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), q.StartTime)
	require.Equal(t, time.Date(2026, 9, 19, 0, 0, 0, 0, time.Local), q.EndTime)
}

func TestGetSessionsByTenantOmittedFiltersStayOpen(t *testing.T) {
	svc := &recordingListSessions{}
	w := serveSessionList(t, newSessionListEnv(t, svc), "/")
	require.Equal(t, http.StatusOK, w.Code)

	require.NotNil(t, svc.captured)
	q := svc.captured
	require.Empty(t, q.UserID)
	require.True(t, q.StartTime.IsZero())
	require.True(t, q.EndTime.IsZero())
	require.Empty(t, q.FeedbackRating)
}

func TestGetSessionsByTenantRejectsInvalidFilters(t *testing.T) {
	cases := []struct {
		name  string
		query string
	}{
		{"bad start_time", "?start_time=not-a-date"},
		{"bad end_time", "?end_time=2026-13-99"},
		{"unknown feedback", "?feedback=love"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &recordingListSessions{}
			w := serveSessionList(t, newSessionListEnv(t, svc), tc.query)
			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Nil(t, svc.captured, "a rejected request must not reach the service")
			require.Contains(t, w.Body.String(), "invalid")
		})
	}
}

func TestGetSessionsByTenantFeedbackAcceptsLikeDislikeAndEmpty(t *testing.T) {
	for _, tc := range []struct {
		query string
		want  string
	}{
		{"?feedback=like", types.FeedbackRatingLike},
		{"?feedback=dislike", types.FeedbackRatingDislike},
		{"?feedback=", ""},
		{"?feedback=like+and+junk", ""}, // anything else 400s; body checked below
	} {
		t.Run(tc.query, func(t *testing.T) {
			svc := &recordingListSessions{}
			w := serveSessionList(t, newSessionListEnv(t, svc), tc.query)
			if strings.Contains(tc.query, "junk") {
				require.Equal(t, http.StatusBadRequest, w.Code)
				return
			}
			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, tc.want, svc.captured.FeedbackRating)
		})
	}
}

// A policy denial (query history disabled / Admin+ gate) must surface with its
// own status instead of being flattened into a 500.
func TestGetSessionsByTenantPropagatesPolicyDenial(t *testing.T) {
	svc := &recordingListSessions{
		err: errors.NewForbiddenError("query history is disabled for this tenant"),
	}
	w := serveSessionList(t, newSessionListEnv(t, svc), "/?source=all")
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "query history is disabled")

	unexpected := &recordingListSessions{err: context.DeadlineExceeded}
	w = serveSessionList(t, newSessionListEnv(t, unexpected), "/")
	require.Equal(t, http.StatusInternalServerError, w.Code)
}
