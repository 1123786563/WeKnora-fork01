package session

// Handler-level tests for the Admin+ query-history snapshot endpoint: the
// response envelope and the error mapping (404 session miss, 403 policy
// denial, 500 unexpected). The Admin role gate and the full-access API-key
// policy live on the route registration (see router routes_query_history.go),
// mirroring how the audit listing's Viewer+ gate is route-owned.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type recordingSnapshotService struct {
	interfaces.SessionService
	snapshot *types.QueryHistorySnapshot
	err      error
	calls    int
}

func (s *recordingSnapshotService) GetQueryHistorySnapshot(
	_ context.Context, _ uint64, _ string,
) (*types.QueryHistorySnapshot, error) {
	s.calls++
	return s.snapshot, s.err
}

func newSnapshotHandlerEnv(t *testing.T, svc interfaces.SessionService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := &Handler{sessionService: svc}
	engine := gin.New()
	engine.Use(middleware.ErrorHandler())
	engine.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	engine.GET("/api/v1/admin/sessions/:session_id/snapshot", h.GetQueryHistorySnapshot)
	return engine
}

func serveSnapshot(t *testing.T, engine *gin.Engine) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/sessions/s1/snapshot", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func TestGetQueryHistorySnapshotHandlerEnvelope(t *testing.T) {
	svc := &recordingSnapshotService{snapshot: &types.QueryHistorySnapshot{
		Session:   types.Session{ID: "s1", TenantID: 1, UserID: "anonymous"},
		Messages:  []*types.Message{{ID: "m1", SessionID: "s1", Role: "user"}},
		Feedback:  []types.MessageFeedback{{UserID: "anonymous", MessageID: "m1", Rating: types.FeedbackRatingLike}},
		Truncated: true,
	}}
	w := serveSnapshot(t, newSnapshotHandlerEnv(t, svc))

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, svc.calls)

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
}

func TestGetQueryHistorySnapshotHandlerErrorMapping(t *testing.T) {
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
			svc := &recordingSnapshotService{err: tc.err}
			w := serveSnapshot(t, newSnapshotHandlerEnv(t, svc))

			require.Equal(t, tc.wantCode, w.Code)
			var body struct {
				Success bool `json:"success"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			require.False(t, body.Success)
		})
	}
}
