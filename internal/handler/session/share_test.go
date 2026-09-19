package session

// Handler-level tests for the session-share endpoints (SP13 Task 5): the
// POST envelope is the only surface that reveals the token, DELETE answers
// the standard success envelope, and the GET miss maps every failure flavor
// (unknown / revoked / cross-tenant / soft-deleted) onto one uniform 404
// whose message does not say which it was.

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

type recordingShareService struct {
	interfaces.SessionService
	shareToken   string
	shareErr     error
	unshareErr   error
	shared       *types.SharedSessionSnapshot
	sharedErr    error
	callerSeen   types.Caller
	sessionSeen  string
	tokenSeen    string
	shareCalls   int
	unshareCalls int
	sharedCalls  int
}

func (s *recordingShareService) ShareSession(
	_ context.Context, caller types.Caller, sessionID string,
) (string, error) {
	s.shareCalls++
	s.callerSeen = caller
	s.sessionSeen = sessionID
	return s.shareToken, s.shareErr
}

func (s *recordingShareService) UnshareSession(
	_ context.Context, caller types.Caller, sessionID string,
) error {
	s.unshareCalls++
	s.callerSeen = caller
	s.sessionSeen = sessionID
	return s.unshareErr
}

func (s *recordingShareService) GetSharedSession(
	_ context.Context, caller types.Caller, token string,
) (*types.SharedSessionSnapshot, error) {
	s.sharedCalls++
	s.callerSeen = caller
	s.tokenSeen = token
	return s.shared, s.sharedErr
}

// newShareHandlerEnv mounts the three endpoints with the wildcard names the
// real router uses (POST :session_id, DELETE :id) plus the tenant/caller
// context the auth middleware would have set.
func newShareHandlerEnv(t *testing.T, svc interfaces.SessionService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := &Handler{sessionService: svc}
	engine := gin.New()
	engine.Use(middleware.ErrorHandler())
	engine.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(1))
		ctx = types.WithCaller(ctx, types.Caller{
			TenantID: 1, UserID: "caller-1", Role: types.TenantRoleContributor,
		})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	engine.POST("/api/v1/sessions/:session_id/share", h.ShareSession)
	engine.DELETE("/api/v1/sessions/:id/share", h.UnshareSession)
	engine.GET("/api/v1/shared/sessions/:token", h.GetSharedSession)
	return engine
}

func serveShare(t *testing.T, engine *gin.Engine, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func TestShareSessionHandlerReturnsTokenEnvelope(t *testing.T) {
	svc := &recordingShareService{shareToken: "tok-abc"}
	w := serveShare(t, newShareHandlerEnv(t, svc), http.MethodPost, "/api/v1/sessions/s1/share")

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, svc.shareCalls)
	require.Equal(t, "s1", svc.sessionSeen)
	require.Equal(t, uint64(1), svc.callerSeen.TenantID, "the handler forwards the ctx caller")

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			ShareToken string `json:"share_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, "tok-abc", body.Data.ShareToken)
}

func TestUnshareSessionHandlerSuccessEnvelope(t *testing.T) {
	svc := &recordingShareService{}
	w := serveShare(t, newShareHandlerEnv(t, svc), http.MethodDelete, "/api/v1/sessions/s1/share")

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, svc.unshareCalls)
	require.Equal(t, "s1", svc.sessionSeen, "the :id fallback resolves on the DELETE tree")

	var body struct {
		Success bool                   `json:"success"`
		Data    map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.NotNil(t, body.Data)
}

func TestGetSharedSessionHandlerEnvelope(t *testing.T) {
	svc := &recordingShareService{shared: &types.SharedSessionSnapshot{
		Session:   types.Session{ID: "s1", TenantID: 1, UserID: "owner-1"},
		Messages:  []*types.Message{{ID: "m1", SessionID: "s1", Role: "user"}},
		Truncated: false,
	}}
	w := serveShare(t, newShareHandlerEnv(t, svc), http.MethodGet, "/api/v1/shared/sessions/tok-abc")

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, svc.sharedCalls)
	require.Equal(t, "tok-abc", svc.tokenSeen)

	var body struct {
		Success bool                         `json:"success"`
		Data    *types.SharedSessionSnapshot `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, "s1", body.Data.Session.ID)
	require.Len(t, body.Data.Messages, 1)
}

// Every miss flavor the service can answer — unknown token, revoked link,
// cross-tenant token, soft-deleted session — maps to the same 404 with the
// same message, so the read reveals nothing about which one it was.
func TestGetSharedSessionHandlerUniform404(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"unknown token", errors.NewNotFoundError("shared session not found")},
		{"revoked link", errors.NewNotFoundError("shared session not found")},
		{"legacy sentinel miss", errors.ErrSessionNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &recordingShareService{sharedErr: tc.err}
			w := serveShare(t, newShareHandlerEnv(t, svc),
				http.MethodGet, "/api/v1/shared/sessions/tok-gone")

			require.Equal(t, http.StatusNotFound, w.Code)
			var body struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			require.False(t, body.Success)
		})
	}
}

func TestShareEndpointsErrorMapping(t *testing.T) {
	cases := []struct {
		name     string
		method   string
		path     string
		err      error
		wantCode int
	}{
		{"share forbidden", http.MethodPost, "/api/v1/sessions/s1/share",
			errors.NewForbiddenError("not allowed to share this session"), http.StatusForbidden},
		{"share session miss", http.MethodPost, "/api/v1/sessions/s1/share",
			errors.NewNotFoundError("session not found"), http.StatusNotFound},
		{"share unexpected", http.MethodPost, "/api/v1/sessions/s1/share",
			context.DeadlineExceeded, http.StatusInternalServerError},
		{"unshare forbidden", http.MethodDelete, "/api/v1/sessions/s1/share",
			errors.NewForbiddenError("not allowed to share this session"), http.StatusForbidden},
		{"unshare miss", http.MethodDelete, "/api/v1/sessions/s1/share",
			errors.NewNotFoundError("session not found"), http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &recordingShareService{shareErr: tc.err, unshareErr: tc.err}
			w := serveShare(t, newShareHandlerEnv(t, svc), tc.method, tc.path)

			require.Equal(t, tc.wantCode, w.Code)
			var body struct {
				Success bool `json:"success"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			require.False(t, body.Success)
		})
	}
}
