package session

// T13 (#43) Task 5: the legal-hold gate covers EVERY internal deletion
// entrance — single delete, batch (ids and delete_all) and message clear.
// The gate is nil-safe (ungated deployments keep today's flow), a refusal
// is 409, and an infrastructure error fails closed with 500.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type recordingGuard struct {
	calls []struct {
		tenant    uint64
		actor     string
		sessionID string
	}
	err error
}

func (g *recordingGuard) AllowsTaskDeletion(_ context.Context, tenant uint64, actor, sessionID string) error {
	g.calls = append(g.calls, struct {
		tenant    uint64
		actor     string
		sessionID string
	}{tenant, actor, sessionID})
	return g.err
}

func newGuardEnv(t *testing.T, guard TaskDeletionGuard) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := openCraftHTTPDB(t)
	require.NoError(t, db.Exec(
		"INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s-g', 1, 'guard', 'u1', 'trpc')").Error)
	h := &Handler{
		sessionService:    deletionSessions{},
		agentRunService:   service.NewAgentRunService(repository.NewAgentRunStore(db)),
		taskDeletionGuard: guard,
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
	engine.DELETE("/api/v1/sessions/batch", h.BatchDeleteSessions)
	engine.DELETE("/api/v1/sessions/:id/messages", h.ClearSessionMessages)
	return engine
}

func TestSessionDeleteBlockedByLegalHold(t *testing.T) {
	guard := &recordingGuard{err: apperrors.NewConflictError("task deletion is blocked by the tenant legal hold")}
	engine := newGuardEnv(t, guard)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/s-g", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusConflict, w.Code, "legal hold refuses the single delete: %s", w.Body.String())
	require.Len(t, guard.calls, 1, "the gate ran exactly once")
	require.Equal(t, "s-g", guard.calls[0].sessionID)
}

func TestBatchAndClearBlockedByLegalHold(t *testing.T) {
	guard := &recordingGuard{err: apperrors.NewConflictError("task deletion is blocked by the tenant legal hold")}
	engine := newGuardEnv(t, guard)

	// Review Focus 5: batch (ids branch) cannot bypass the hold.
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/batch", httptestBody(t, `{"ids":["s-g"]}`))
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusConflict, w.Code, "batch delete is gated: %s", w.Body.String())

	// delete_all=true branch is gated too (tenant-level check, one call).
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/batch", httptestBody(t, `{"delete_all":true}`))
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusConflict, w.Code, "delete_all is gated: %s", w.Body.String())

	// Clearing messages destroys evidence — gated as well.
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/s-g/messages", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusConflict, w.Code, "message clear is gated: %s", w.Body.String())
}

func TestDeletionGuardNilKeepsFlowAndInfraErrorFailsClosed(t *testing.T) {
	// nil guard → today's ungated flow (200, zero gate calls).
	engine := newGuardEnv(t, nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/s-g", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// An infrastructure error fails CLOSED (500), never falls through to
	// deleting under a broken policy check.
	engine = newGuardEnv(t, &recordingGuard{err: context.DeadlineExceeded})
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/s-g", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusInternalServerError, w.Code, "a broken gate must not un-gate deletion: %s", w.Body.String())
}

func httptestBody(t *testing.T, body string) *strings.Reader {
	t.Helper()
	return strings.NewReader(body)
}
