package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

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
		ctx = context.WithValue(ctx, types.UserIDContextKey, "viewer-1")
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleViewer)
		c.Request = c.Request.WithContext(ctx)
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		c.Set(types.UserIDContextKey.String(), "viewer-1")
		c.Set(types.TenantRoleContextKey.String(), types.TenantRoleViewer)
		c.Next()
	})

	h := session.NewWorkbenchDeliveryHandler(nil, nil, nil)
	RegisterWorkbenchDeliveryRoutes(engine.Group("/api/v1"), h, &rbacGuards{cfg: cfg})

	// The production route's Viewer floor admits a viewer, then the read gate
	// answers 503 before the handler can be reached.
	read := httptest.NewRecorder()
	engine.ServeHTTP(read, httptest.NewRequest(http.MethodGet,
		"/api/v1/workbench/executions/r1/delivery", nil))
	if read.Code != http.StatusServiceUnavailable || !strings.Contains(read.Body.String(), "workbench reads are disabled") {
		t.Fatalf("viewer read: status=%d body=%q, want production read-gate 503", read.Code, read.Body.String())
	}

	// Writes share the Viewer floor but intentionally bypass the read gate.
	// The nil run reader makes the real handler return its own 401, proving the
	// request passed the route guard and did not receive the read-gate 503.
	write := httptest.NewRecorder()
	engine.ServeHTTP(write, httptest.NewRequest(http.MethodPost,
		"/api/v1/workbench/executions/r1/baseline", strings.NewReader(`{}`)))
	if write.Code != http.StatusUnauthorized {
		t.Fatalf("viewer write: status=%d body=%q, want handler 401 after route guard (read gate bypassed)", write.Code, write.Body.String())
	}
}
