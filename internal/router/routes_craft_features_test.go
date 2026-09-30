package router

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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

func TestCraftAccessProductionSessionAssemblyAuthAndAudit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:craft-access-router?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, ddl := range []string{
		`CREATE TABLE sessions (id text primary key, tenant_id integer not null, user_id text not null, deleted_at datetime)`,
		`CREATE TABLE craft_sessions (session_id text primary key, tenant_id integer not null, kind text not null)`,
		`CREATE TABLE tenant_members (id integer primary key autoincrement, tenant_id integer not null, user_id text not null, status text not null, deleted_at datetime)`,
		`CREATE TABLE craft_task_grants (tenant_id integer not null, session_id text not null, user_id text not null, membership_id integer not null, role text not null, granted_by text not null, created_at datetime, updated_at datetime, primary key (tenant_id, session_id, user_id))`,
		`CREATE TABLE audit_logs (id integer primary key autoincrement, tenant_id integer, actor_user_id text, action text, scope_type text, scope_id text, target_type text, target_id text, target_user_id text, outcome text, details text, created_at datetime)`,
		`INSERT INTO sessions (id, tenant_id, user_id) VALUES ('task-router', 1, 'owner')`,
		`INSERT INTO craft_sessions (session_id, tenant_id, kind) VALUES ('task-router', 1, 'web')`,
		`INSERT INTO tenant_members (tenant_id, user_id, status) VALUES (1, 'owner', 'active'), (1, 'viewer', 'active'), (1, 'admin', 'active')`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatal(err)
		}
	}

	features := session.NewCraftFeatureRoutes()
	access := service.NewCraftAccessService(db)
	if err := session.RegisterCraftAccessFeature(features, access); err != nil {
		t.Fatal(err)
	}
	enabled := true
	g := &rbacGuards{cfg: &config.Config{Tenant: &config.TenantConfig{EnableRBAC: &enabled}}}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		user := c.GetHeader("X-Test-User")
		role := types.TenantRoleViewer
		if user == "owner" {
			role = types.TenantRoleOwner
		} else if user == "admin" {
			role = types.TenantRoleAdmin
		}
		if user != "" {
			ctx = types.WithCaller(ctx, types.Caller{TenantID: 1, UserID: user, Role: role})
			ctx = context.WithValue(ctx, types.UserIDContextKey, user)
			ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(1))
			c.Set(types.TenantIDContextKey.String(), uint64(1))
		}
		if key := c.GetHeader("X-Test-Key"); key != "" {
			scope := types.TenantAPIKeyScope{KeyID: 1}
			if key == "chat" {
				scope.Capabilities = types.StringArray{string(types.APIKeyCapabilityChat)}
			}
			ctx = types.WithTenantAPIKeyScope(ctx, scope)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.Use(g.ensureAPIKeyAuthorizer().Middleware())
	v1 := r.Group("/api/v1")
	registerSessionRoutes(v1, RouterParams{SessionHandler: &session.Handler{}, CraftFeatureRoutes: features}, g)

	request := func(method, path, user, key, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if user != "" {
			req.Header.Set("X-Test-User", user)
		}
		if key != "" {
			req.Header.Set("X-Test-Key", key)
		}
		response := httptest.NewRecorder()
		r.ServeHTTP(response, req)
		return response
	}
	path := "/api/v1/sessions/task-router/craft/access"
	if got := request(http.MethodGet, path, "", "", "").Code; got != http.StatusUnauthorized {
		t.Fatalf("unauthenticated route status = %d, want 401", got)
	}
	if got := request(http.MethodGet, path, "", "no-chat", "").Code; got != http.StatusForbidden {
		t.Fatalf("scoped key without chat status = %d, want 403", got)
	}
	if got := request(http.MethodPost, path, "admin", "", `{"user_id":"viewer","role":"viewer"}`).Code; got != http.StatusForbidden {
		t.Fatalf("tenant admin grant status = %d, want 403", got)
	}
	if got := request(http.MethodPost, path, "owner", "", `{"user_id":"viewer","role":"viewer"}`).Code; got != http.StatusOK {
		t.Fatalf("owner grant status = %d, want 200", got)
	}
	if got := request(http.MethodGet, path, "viewer", "", "").Code; got != http.StatusOK {
		t.Fatalf("granted Viewer list status = %d, want 200", got)
	}
	revoke := "/api/v1/sessions/task-router/craft/access/revoke"
	if got := request(http.MethodPost, revoke, "owner", "", `{"user_id":"viewer"}`).Code; got != http.StatusOK {
		t.Fatalf("owner revoke status = %d, want 200", got)
	}
	if got := request(http.MethodGet, path, "viewer", "", "").Code; got != http.StatusForbidden {
		t.Fatalf("revoked Viewer list status = %d, want 403", got)
	}
	var grants, audits int64
	if err := db.Table("craft_task_grants").Count(&grants).Error; err != nil || grants != 0 {
		t.Fatalf("remaining grants = %d, err=%v", grants, err)
	}
	if err := db.Table("audit_logs").Where("action IN ?", []string{"craft.member_added", "craft.member_revoked"}).Count(&audits).Error; err != nil || audits != 2 {
		t.Fatalf("successful access audit rows = %d, err=%v", audits, err)
	}
}
