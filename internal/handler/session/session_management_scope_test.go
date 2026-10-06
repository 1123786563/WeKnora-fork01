package session

// #3926: the ownership pre-check of the session management mutations (PUT
// /sessions/:id, DELETE /sessions/:id, batch delete) falls back to the
// tenant-wide scope for Admin+ callers, so an admin can manage embed-channel
// sessions whose user_id is the embed principal storage id and never matches
// the strict owner scope. Non-admin callers keep the strict check: a miss is
// a 404 and the mutation never runs.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// managementScopeSessions simulates the service behind a channel session:
// GetOwnedSession always misses (no user id ever matches an embed/API/IM
// principal storage id) while the tenant-scoped GetSessionByID finds it.
type managementScopeSessions struct {
	interfaces.SessionService

	deleted []string
	updated []string
	batched [][]string
	loaded  *types.Session
}

func (s *managementScopeSessions) GetOwnedSession(context.Context, string) (*types.Session, error) {
	return nil, apperrors.ErrSessionNotFound
}

func (s *managementScopeSessions) GetSessionByID(_ context.Context, tenantID uint64, id string) (*types.Session, error) {
	if s.loaded != nil {
		return s.loaded, nil
	}
	return &types.Session{ID: id, TenantID: tenantID, EngineType: "trpc"}, nil
}

func (s *managementScopeSessions) DeleteSession(_ context.Context, id string) error {
	s.deleted = append(s.deleted, id)
	return nil
}

func (s *managementScopeSessions) UpdateSession(_ context.Context, sess *types.Session) error {
	s.updated = append(s.updated, sess.ID)
	return nil
}

func (s *managementScopeSessions) BatchDeleteSessions(_ context.Context, ids []string) error {
	s.batched = append(s.batched, ids)
	return nil
}

func (s *managementScopeSessions) GetSession(_ context.Context, id string) (*types.Session, error) {
	return &types.Session{ID: id, TenantID: 1, EngineType: "trpc"}, nil
}

// managementScopeRunStore extends the shared runStoreFake with the lifecycle
// methods fenceSessionRuns needs: DeleteSessionRuns must succeed for the
// delete/update/batch-delete paths to proceed.
type managementScopeRunStore struct {
	runStoreFake
}

func (managementScopeRunStore) CancelRun(context.Context, agentruntime.RunKey, string) error {
	return nil
}

func (managementScopeRunStore) DeleteSessionRuns(context.Context, uint64, string) error {
	return nil
}

func newManagementScopeEnv(t *testing.T, svc *managementScopeSessions, role types.TenantRole) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := &Handler{
		sessionService: svc,
	}
	h.SetAgentRunService(service.NewAgentRunService(&managementScopeRunStore{}))
	engine := gin.New()
	engine.Use(middleware.ErrorHandler())
	engine.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	engine.DELETE("/api/v1/sessions/:id", h.DeleteSession)
	engine.PUT("/api/v1/sessions/:id", h.UpdateSession)
	engine.DELETE("/api/v1/sessions/batch", h.BatchDeleteSessions)
	return engine
}

func TestDeleteSessionManagementScopeFallback(t *testing.T) {
	for _, tc := range []struct {
		name        string
		role        types.TenantRole
		wantStatus  int
		wantDeleted int
	}{
		{"admin deletes embed session", types.TenantRoleAdmin, http.StatusOK, 1},
		{"owner deletes embed session", types.TenantRoleOwner, http.StatusOK, 1},
		{"viewer is still denied", types.TenantRoleViewer, http.StatusNotFound, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &managementScopeSessions{}
			engine := newManagementScopeEnv(t, svc, tc.role)

			req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/embed-s1", nil)
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			require.Equal(t, tc.wantStatus, w.Code)
			require.Len(t, svc.deleted, tc.wantDeleted)
		})
	}
}

func TestBatchDeleteSessionsManagementScopeFallback(t *testing.T) {
	for _, tc := range []struct {
		name        string
		role        types.TenantRole
		wantStatus  int
		wantBatches int
	}{
		{"admin batch deletes embed sessions", types.TenantRoleAdmin, http.StatusOK, 1},
		{"viewer batch delete is denied", types.TenantRoleViewer, http.StatusNotFound, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &managementScopeSessions{}
			engine := newManagementScopeEnv(t, svc, tc.role)

			body := `{"ids": ["embed-s1", "embed-s2"]}`
			req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/batch", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			require.Equal(t, tc.wantStatus, w.Code)
			require.Len(t, svc.batched, tc.wantBatches)
			if tc.wantBatches > 0 {
				require.Equal(t, []string{"embed-s1", "embed-s2"}, svc.batched[0])
			}
		})
	}
}

func TestUpdateSessionManagementScopeFallback(t *testing.T) {
	for _, tc := range []struct {
		name       string
		role       types.TenantRole
		wantStatus int
		wantUpdate int
	}{
		{"admin renames embed session", types.TenantRoleAdmin, http.StatusOK, 1},
		{"viewer rename is denied", types.TenantRoleViewer, http.StatusNotFound, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &managementScopeSessions{}
			engine := newManagementScopeEnv(t, svc, tc.role)

			body := `{"title": "admin renamed"}`
			req := httptest.NewRequest(http.MethodPut, "/api/v1/sessions/embed-s1", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)

			require.Equal(t, tc.wantStatus, w.Code)
			require.Len(t, svc.updated, tc.wantUpdate)
		})
	}
}
