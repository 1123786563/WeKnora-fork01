package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// The R06 verifiable stop HTTP surface: the honest phase ("stopping" is a
// real answer, never folded into a boolean) and the read-only delegation
// status poll. Only the API seam is faked — the service contract is the
// production CraftControlService one.

type stopFakeAPI struct {
	stopErr          error
	stopGot          service.CraftStopRequest
	stopReply        service.CraftStopStatus
	stopCalls        int
	delegationReply  service.CraftStopStatus
	delegationGotRun string
}

func (f *stopFakeAPI) ListInteractions(context.Context, craft.Scope, string) ([]service.CraftInteractionRecord, error) {
	return nil, nil
}

func (f *stopFakeAPI) Decide(context.Context, service.CraftDecisionRequest) (service.CraftDecisionOutcome, error) {
	return service.CraftDecisionOutcome{}, nil
}

func (f *stopFakeAPI) Stop(_ context.Context, req service.CraftStopRequest) (service.CraftStopStatus, error) {
	f.stopCalls++
	f.stopGot = req
	return f.stopReply, f.stopErr
}

func (f *stopFakeAPI) DelegationStatus(_ context.Context, _ craft.Scope, key agentruntime.RunKey, _ string) (service.CraftStopStatus, error) {
	f.delegationGotRun = key.RunID
	return f.delegationReply, nil
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
	r := stopTestRouter(&stopFakeAPI{delegationReply: service.CraftStopStatus{Phase: "stopping"}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/s1/craft/runs/run_1/delegations/dlg_1/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"phase":"stopping"`) {
		t.Fatalf("expected 200 with the polled phase, got %d %s", w.Code, w.Body.String())
	}
}

// T17 (#136): a repeated stop over HTTP answers the same durable phase —
// the entrance replays the persisted stop intent instead of rejecting or
// re-classifying it — and the read-only status poll a page refresh runs
// reports the persisted phase for the same run.
func TestStopCraftRunRepeatedStopIsIdempotentAcrossRefresh(t *testing.T) {
	api := &stopFakeAPI{stopReply: service.CraftStopStatus{Phase: "stopping", Note: "abort accepted, still running"}}
	r := stopTestRouter(api)
	post := func() int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/s1/craft/runs/run_1/stop", strings.NewReader(`{"task_id":"dlg_1"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"phase":"stopping"`) {
			t.Fatalf("expected 200 with the honest stopping phase, got %d %s", w.Code, w.Body.String())
		}
		return w.Code
	}
	if first, second := post(), post(); first != second || api.stopCalls != 2 {
		t.Fatalf("repeated stops must replay the same durable answer: codes %d/%d, calls %d", first, second, api.stopCalls)
	}
	// The refresh poll: a NEW request after the page reload reads the same
	// persisted run state without re-issuing a stop.
	api.delegationReply = service.CraftStopStatus{Phase: "stopping"}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/s1/craft/runs/run_1/delegations/dlg_1/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"phase":"stopping"`) {
		t.Fatalf("expected the refresh poll to report the persisted stopping phase, got %d %s", w.Code, w.Body.String())
	}
	if api.delegationGotRun != "run_1" || api.stopCalls != 2 {
		t.Fatalf("the poll must read run_1 read-only without another stop: run %q, stops %d", api.delegationGotRun, api.stopCalls)
	}
}
