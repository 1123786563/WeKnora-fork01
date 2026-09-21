package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// The R06 verifiable stop HTTP surface: the honest phase ("stopping" is a
// real answer, never folded into a boolean) and the read-only delegation
// status poll. Only the API seam is faked — the service contract is the
// production CraftControlService one.

type stopFakeAPI struct {
	stopErr   error
	stopGot   service.CraftStopRequest
	stopReply service.CraftStopStatus
}

func (f *stopFakeAPI) ListInteractions(context.Context, craft.Scope, string) ([]service.CraftInteractionRecord, error) {
	return nil, nil
}

func (f *stopFakeAPI) Decide(context.Context, service.CraftDecisionRequest) (service.CraftDecisionOutcome, error) {
	return service.CraftDecisionOutcome{}, nil
}

func (f *stopFakeAPI) Stop(_ context.Context, req service.CraftStopRequest) (service.CraftStopStatus, error) {
	f.stopGot = req
	return f.stopReply, f.stopErr
}

func (f *stopFakeAPI) DelegationStatus(context.Context, craft.Scope, agentruntime.RunKey, string) (service.CraftStopStatus, error) {
	return service.CraftStopStatus{Phase: "stopping"}, nil
}

// stopTestRouter mounts the two stop routes behind the production-shaped
// identity chain: craftScope reads the tenant from the gin context (set by
// the auth middleware via c.Set) and the user from the request context
// (types.UserIDContextKey, the SessionOwnerIDFromContext fallback source),
// and middleware.ErrorHandler turns c.Error(apperrors...) into real codes.
func stopTestRouter(api CraftInteractionAPI) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(42))
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(42))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.POST("/api/v1/sessions/:session_id/craft/runs/:run_id/stop", NewCraftInteractionHandler(api).StopCraftRun)
	r.GET("/api/v1/sessions/:id/craft/runs/:run_id/delegations/:task_id/status", NewCraftInteractionHandler(api).GetCraftDelegationStatus)
	return r
}

func TestStopCraftRunKeepsHonestPhase(t *testing.T) {
	api := &stopFakeAPI{stopReply: service.CraftStopStatus{Phase: "stopping", Note: "abort accepted, still running"}}
	r := stopTestRouter(api)
	body := `{"task_id":"dlg_1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/s1/craft/runs/run_1/stop", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"phase":"stopping"`) {
		t.Fatalf("expected 200 with honest stopping phase, got %d %s", w.Code, w.Body.String())
	}
	if api.stopGot.RunKey.RunID != "run_1" || api.stopGot.TaskID != "dlg_1" || api.stopGot.Scope.TenantID != 42 {
		t.Fatalf("unexpected stop request: %+v", api.stopGot)
	}
	if api.stopGot.Scope.SessionID != "s1" || api.stopGot.Scope.UserID != "u1" {
		t.Fatalf("scope must come from the authenticated context, not the body: %+v", api.stopGot.Scope)
	}
}

func TestStopCraftRunRejectsMissingTaskID(t *testing.T) {
	r := stopTestRouter(&stopFakeAPI{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/s1/craft/runs/run_1/stop", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing task_id, got %d %s", w.Code, w.Body.String())
	}
}

func TestGetCraftDelegationStatusIsReadOnlyPoll(t *testing.T) {
	r := stopTestRouter(&stopFakeAPI{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/s1/craft/runs/run_1/delegations/dlg_1/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"phase":"stopping"`) {
		t.Fatalf("expected 200 with the polled phase, got %d %s", w.Code, w.Body.String())
	}
}
