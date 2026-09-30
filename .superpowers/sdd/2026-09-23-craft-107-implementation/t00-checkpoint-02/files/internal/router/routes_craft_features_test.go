package router

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// Use the same Viewer+ group, chat API-key wrapper and authorizer that
// RegisterSessionRoutes uses in production, rather than a synthetic header.
func TestCraftFeatureProductionRouterPolicyAndEscape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	g := &rbacGuards{cfg: &config.Config{Tenant: &config.TenantConfig{EnableRBAC: &enabled}}}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		if c.GetHeader("X-Test-Revoked") == "yes" {
			ctx = context.WithValue(ctx, types.CallerContextKey, types.Caller{Role: types.TenantRole("revoked")})
		}
		if c.GetHeader("X-Test-Key") != "" {
			scope := types.TenantAPIKeyScope{KeyID: 1}
			if c.GetHeader("X-Test-Key") == "chat" {
				scope.Capabilities = types.StringArray{string(types.APIKeyCapabilityChat)}
			}
			ctx = types.WithTenantAPIKeyScope(ctx, scope)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.Use(g.ensureAPIKeyAuthorizer().Middleware())
	v1 := r.Group("/api/v1")
	sessions := g.apiKeyGroup(v1.Group("/sessions", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
	features := session.NewCraftFeatureRoutes()
	if err := features.Register("sources", func(group session.CraftRouteGroup) {
		group.GET("/:id/craft/sources", func(c *gin.Context) { c.Status(http.StatusOK) })
	}); err != nil {
		t.Fatal(err)
	}
	if err := features.Mount(sessions); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/sessions/task-1/craft/sources"
	for _, tc := range []struct {
		name, header, value string
		want                int
	}{
		{"viewer", "", "", http.StatusOK},
		{"revoked", "X-Test-Revoked", "yes", http.StatusForbidden},
		{"scoped key without chat", "X-Test-Key", "no-chat", http.StatusForbidden},
		{"scoped key with chat", "X-Test-Key", "chat", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			if tc.header != "" {
				req.Header.Set(tc.header, tc.value)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
		})
	}
	policy := mustLookupAPIKeyPolicy(t, g, http.MethodGet, "/api/v1/sessions/:id/craft/sources")
	if !policy.RequireFullAccess || !policyHasCapability(policy, types.APIKeyCapabilityChat) {
		t.Fatalf("Craft feature policy = %#v", policy)
	}

	escape := session.NewCraftFeatureRoutes()
	if err := escape.Register("escape", func(group session.CraftRouteGroup) {
		group.GET("/:id/craft/../../share", func(c *gin.Context) { c.Status(http.StatusOK) })
	}); err != nil {
		t.Fatal(err)
	}
	var rejected any
	func() { defer func() { rejected = recover() }(); _ = escape.Mount(sessions) }()
	if rejected == nil || !strings.Contains(fmt.Sprint(rejected), "invalid Craft feature path") {
		t.Fatalf("escape was not rejected before router registration: %v", rejected)
	}
	if _, ok := g.apiKeyAuthorizer.Lookup(http.MethodGet, "/api/v1/sessions/share"); ok {
		t.Fatal("escape registered an API-key policy outside Craft")
	}
	for _, route := range r.Routes() {
		if route.Path == "/api/v1/sessions/share" {
			t.Fatal("escape registered a sibling route")
		}
	}
}
