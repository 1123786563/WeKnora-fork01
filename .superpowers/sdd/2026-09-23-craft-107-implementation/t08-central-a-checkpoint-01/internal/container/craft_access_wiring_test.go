package container

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCraftAccessFeatureUsesPersistentPolicyOnConstrainedRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := wiringTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO craft_sessions (session_id, tenant_id, kind) VALUES ('s-wiring', 1, 'web')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id, user_id, role, status, joined_at, created_at, updated_at) VALUES (1, 'u-wiring', 'contributor', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP), (1, 'u-admin', 'admin', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP), (1, 'u-viewer', 'viewer', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)

	policy := service.NewCraftAccessService(db)
	var checker craft.TaskAccessChecker = craftTaskAccessChecker(policy)
	require.Same(t, policy, checker, "handler and TaskAccessChecker must share the one policy instance")

	features := session.NewCraftFeatureRoutes()
	require.NoError(t, wireCraftAccessFeature(policy, features.Register))
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		user := c.GetHeader("X-Test-User")
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), types.UserIDContextKey, user))
	})
	group := r.Group("/api/v1/sessions")
	require.NoError(t, features.Mount(group))

	request := func(method, path, user, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("X-Test-User", user)
		response := httptest.NewRecorder()
		r.ServeHTTP(response, req)
		return response
	}
	// Tenant administrator status alone does not grant access to a private Task.
	admin := request(http.MethodPost, "/api/v1/sessions/s-wiring/craft/access", "u-admin", `{"user_id":"u-viewer","role":"viewer"}`)
	require.Equal(t, http.StatusForbidden, admin.Code)

	owner := request(http.MethodPost, "/api/v1/sessions/s-wiring/craft/access", "u-wiring", `{"user_id":"u-viewer","role":"viewer"}`)
	require.Equal(t, http.StatusOK, owner.Code)
	listed := request(http.MethodGet, "/api/v1/sessions/s-wiring/craft/access", "u-viewer", "")
	require.Equal(t, http.StatusOK, listed.Code)
	denied := request(http.MethodGet, "/api/v1/sessions/s-wiring/craft/access", "u-admin", "")
	require.Equal(t, http.StatusForbidden, denied.Code)
	revoked := request(http.MethodPost, "/api/v1/sessions/s-wiring/craft/access/revoke", "u-wiring", `{"user_id":"u-viewer"}`)
	require.Equal(t, http.StatusOK, revoked.Code)
	denied = request(http.MethodGet, "/api/v1/sessions/s-wiring/craft/access", "u-viewer", "")
	require.Equal(t, http.StatusForbidden, denied.Code)
	unauthenticated := request(http.MethodGet, "/api/v1/sessions/s-wiring/craft/access", "", "")
	require.Equal(t, http.StatusUnauthorized, unauthenticated.Code)

	paths := map[string]bool{}
	for _, route := range r.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	require.True(t, paths["GET /api/v1/sessions/:id/craft/access"])
	require.True(t, paths["POST /api/v1/sessions/:session_id/craft/access"])
	require.True(t, paths["POST /api/v1/sessions/:session_id/craft/access/revoke"])
	require.Len(t, paths, 3, "the access feature registers only its constrained route set")

	var grants, audits int64
	require.NoError(t, db.Table("craft_task_grants").Count(&grants).Error)
	require.Zero(t, grants)
	require.NoError(t, db.Table("audit_logs").Where("action IN ?", []string{"craft.member_added", "craft.member_revoked"}).Count(&audits).Error)
	require.EqualValues(t, 2, audits)
}
