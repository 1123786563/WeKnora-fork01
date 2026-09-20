package session

// O03 wiring test: the craft tombstone at the session-deletion entrance.
// Deleting a session tombstones its craft resources first (the deleting mark
// blocks new dispatches and restores mid-teardown); a tombstone failure
// never blocks the user-facing delete — the sweep's discovery pass
// re-derives the tombstone.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type recordingTombstoner struct {
	calls []struct {
		tenant    uint64
		sessionID string
		reason    string
	}
	err error
}

func (r *recordingTombstoner) TombstoneSession(_ context.Context, tenant uint64, sessionID, reason string) (*service.CraftTombstoneResult, error) {
	r.calls = append(r.calls, struct {
		tenant    uint64
		sessionID string
		reason    string
	}{tenant, sessionID, reason})
	return nil, r.err
}

type deletionSessions struct {
	interfaces.SessionService
}

func (deletionSessions) GetOwnedSession(_ context.Context, _ string) (*types.Session, error) {
	return &types.Session{ID: "s-del", TenantID: 1, UserID: "u1"}, nil
}

func (deletionSessions) DeleteSession(_ context.Context, _ string) error { return nil }

func newDeletionEnv(t *testing.T, tombstoner *recordingTombstoner) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := openCraftHTTPDB(t)
	require.NoError(t, db.Exec(
		"INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s-del', 1, 'delete', 'u1', 'trpc')").Error)
	h := &Handler{
		sessionService:  deletionSessions{},
		agentRunService: service.NewAgentRunService(repository.NewAgentRunStore(db)),
		craftTombstoner: tombstoner,
	}
	engine := gin.New()
	engine.Use(middleware.ErrorHandler())
	engine.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	engine.DELETE("/api/v1/sessions/:id", h.DeleteSession)
	return engine
}

// Deleting a session tombstones its craft resources first; a tombstone
// failure never blocks the user-facing delete (the sweep re-derives it).
func TestDeleteSessionTombstonesCraftResources(t *testing.T) {
	tombstoner := &recordingTombstoner{}
	engine := newDeletionEnv(t, tombstoner)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/s-del", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, tombstoner.calls, 1)
	require.Equal(t, uint64(1), tombstoner.calls[0].tenant)
	require.Equal(t, "s-del", tombstoner.calls[0].sessionID)
	require.Equal(t, "session deletion", tombstoner.calls[0].reason)

	failing := &recordingTombstoner{err: context.DeadlineExceeded}
	engine = newDeletionEnv(t, failing)
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/s-del", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "a failed tombstone is backstopped by the sweep, not the delete")
}
