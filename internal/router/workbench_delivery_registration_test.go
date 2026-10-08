package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/codedelivery"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

type workbenchDeliveryRouteRuns struct{}

func (workbenchDeliveryRouteRuns) GetOwnedRun(_ context.Context, tenantID uint64, ownerID, runID string) (agentruntime.Run, error) {
	return agentruntime.Run{Key: agentruntime.RunKey{TenantID: tenantID, RunID: runID}, Owner: ownerID}, nil
}

type workbenchDeliveryRouteService struct{}

func (workbenchDeliveryRouteService) MaterializeBaseline(context.Context, codedelivery.BaselineInput) (codedelivery.BaselineReceipt, error) {
	return codedelivery.BaselineReceipt{Files: 1, Root: "/workspace/test"}, nil
}
func (workbenchDeliveryRouteService) PrepareDelivery(context.Context, codedelivery.PrepareInput) (codedelivery.DeliveryView, error) {
	return codedelivery.DeliveryView{ID: "dlv-1", State: "prepared"}, nil
}
func (workbenchDeliveryRouteService) DispatchDelivery(context.Context, codedelivery.DispatchInput) (codedelivery.DeliveryView, error) {
	return codedelivery.DeliveryView{ID: "dlv-1", State: "delivered"}, nil
}
func (workbenchDeliveryRouteService) ResolveDeliveryUnknown(context.Context, codedelivery.DispatchInput) (codedelivery.DeliveryView, error) {
	return codedelivery.DeliveryView{ID: "dlv-1", State: "delivered"}, nil
}
func (workbenchDeliveryRouteService) GetDeliveryForRun(context.Context, uint64, string) (codedelivery.DeliveryView, error) {
	return codedelivery.DeliveryView{ID: "dlv-1", State: "delivered"}, nil
}

// TestWorkbenchDeliveryRoutesUseViewerAndReadGateWiring pins the production
// registrar's Viewer floor and split read/write gate. The identity middleware
// below is test-only; global JWT/API-key authentication is outside this
// registrar-level contract test.
func TestWorkbenchDeliveryRoutesUseViewerAndReadGateWiring(t *testing.T) {
	gin.SetMode(gin.TestMode)
	readEnabled := false
	rbacEnabled := true
	cfg := &config.Config{
		Tenant:    &config.TenantConfig{EnableRBAC: &rbacEnabled},
		Workbench: &config.WorkbenchConfig{ReadEnabled: &readEnabled},
	}
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
		role := types.TenantRole(c.GetHeader("X-Test-Role"))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "route-test-user")
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
		// CallerContextKey preserves this deliberately invalid test role
		// instead of TenantRoleFromContext's safe Viewer fallback. Production
		// auth only emits valid roles; this synthetic value exercises the
		// route guard's reject branch below its Viewer floor.
		ctx = context.WithValue(ctx, types.CallerContextKey, types.Caller{TenantID: 1, UserID: "route-test-user", Role: role})
		c.Request = c.Request.WithContext(ctx)
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		c.Set(types.UserIDContextKey.String(), "route-test-user")
		c.Set(types.TenantRoleContextKey.String(), role)
		c.Next()
	})

	h := session.NewWorkbenchDeliveryHandler(workbenchDeliveryRouteRuns{}, nil, workbenchDeliveryRouteService{})
	RegisterWorkbenchDeliveryRoutes(engine.Group("/api/v1"), h, &rbacGuards{cfg: cfg})
	serve := func(role types.TenantRole, method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("X-Test-Role", string(role))
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		return rec
	}

	// The production route's Viewer floor admits a viewer, then the read gate
	// answers 503 before the handler can be reached.
	read := serve(types.TenantRoleViewer, http.MethodGet, "/api/v1/workbench/executions/r1/delivery", "")
	if read.Code != http.StatusServiceUnavailable || !strings.Contains(read.Body.String(), "workbench reads are disabled") {
		t.Fatalf("viewer read: status=%d body=%q, want production read-gate 503", read.Code, read.Body.String())
	}

	// Opening the read lane lets the same Viewer reach the delivery handler.
	// The fake service returns a distinct success, while an unrecognized
	// below-Viewer role is rejected by the production route guard (403).
	readEnabled = true
	read = serve(types.TenantRoleViewer, http.MethodGet, "/api/v1/workbench/executions/r1/delivery", "")
	if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), `"state":"delivered"`) {
		t.Fatalf("viewer read with gate open: status=%d body=%q, want handler 200", read.Code, read.Body.String())
	}
	read = serve(types.TenantRole("below-viewer"), http.MethodGet, "/api/v1/workbench/executions/r1/delivery", "")
	if read.Code != http.StatusForbidden {
		t.Fatalf("below-viewer read: status=%d body=%q, want production Viewer guard 403", read.Code, read.Body.String())
	}

	// Baseline, dispatch and resolve are all production-registered writes. The
	// read gate is closed again: Viewers reach their respective real handlers
	// and get successful fake-service responses, while below-Viewer callers
	// are stopped by the route guard before any handler/provider work.
	readEnabled = false
	for _, tc := range []struct {
		name string
		path string
		body string
		want int
	}{
		{name: "baseline", path: "/api/v1/workbench/executions/r1/baseline", body: `{"connection_id":"conn-1","repo":"octocat/hello","baseline_sha":"b000000000000000000000000000000000000000"}`, want: http.StatusCreated},
		{name: "dispatch", path: "/api/v1/workbench/executions/r1/delivery/dlv-1/dispatch", want: http.StatusOK},
		{name: "resolve", path: "/api/v1/workbench/executions/r1/delivery/dlv-1/resolve", want: http.StatusOK},
	} {
		viewer := serve(types.TenantRoleViewer, http.MethodPost, tc.path, tc.body)
		if viewer.Code != tc.want {
			t.Errorf("viewer %s: status=%d body=%q, want handler %d (route mounted, read gate bypassed)", tc.name, viewer.Code, viewer.Body.String(), tc.want)
		}
		below := serve(types.TenantRole("below-viewer"), http.MethodPost, tc.path, tc.body)
		if below.Code != http.StatusForbidden {
			t.Errorf("below-viewer %s: status=%d body=%q, want production Viewer guard 403", tc.name, below.Code, below.Body.String())
		}
	}
}
