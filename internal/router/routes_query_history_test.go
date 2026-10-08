package router

// The admin query-history snapshot route must coexist with the session and
// usage routes on one engine: gin keeps a radix tree per verb and panics at
// registration on wildcard-name conflicts, so mounting all three together is
// the executable proof the router still boots. The test also pins the exact
// path so the Admin+ audit surface cannot silently move. Since Wave 1 Task 10
// the routes mount through the conversation module's Query History feature:
// a zero-Dependencies module is enough here because the test only exercises
// registration coexistence (the module's own tests drive the handlers).

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler"
	sessionhandler "github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory"
)

func TestRegisterQueryHistoryAdminRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := &rbacGuards{cfg: &config.Config{}}
	v1 := r.Group("/api/v1")
	module := queryhistory.NewModule(queryhistory.Dependencies{})

	// Neighbouring surfaces that share the verb trees.
	RegisterSessionRoutes(v1, &sessionhandler.Handler{}, &handler.MessageSuggestionHandler{}, g)
	RegisterUsageRoutes(v1, &handler.UsageHandler{}, g)
	RegisterQueryHistoryAdminRoutes(v1, module, g)

	seen := map[string]bool{}
	for _, route := range r.Routes() {
		seen[route.Method+" "+route.Path] = true
	}
	require.True(t, seen[http.MethodGet+" /api/v1/admin/sessions/:session_id/snapshot"])
	// The async export surface (SP13 Task 4) mounts alongside the snapshot:
	// the literal "export" segment must not conflict with the :session_id
	// wildcard — registering both on one engine is the proof.
	require.True(t, seen[http.MethodPost+" /api/v1/admin/sessions/export"])
	require.True(t, seen[http.MethodGet+" /api/v1/admin/sessions/export/:job_id/status"])
	require.True(t, seen[http.MethodGet+" /api/v1/admin/sessions/export/:job_id/download"])
	// The pre-existing routes stay intact.
	require.True(t, seen[http.MethodGet+" /api/v1/sessions/:id"])
	require.True(t, seen[http.MethodGet+" /api/v1/admin/usage/by-user"])

	// The bridge must also declare the API-key policy for every mounted
	// module route (the /api/v1 gate denies undeclared routes by default),
	// exactly as the legacy admin.GET/admin.POST registrations did.
	declared := g.apiKeyAuthorizer.RegisteredRoutes()
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/admin/sessions/:session_id/snapshot"},
		{http.MethodPost, "/api/v1/admin/sessions/export"},
		{http.MethodGet, "/api/v1/admin/sessions/export/:job_id/status"},
		{http.MethodGet, "/api/v1/admin/sessions/export/:job_id/download"},
	} {
		require.Contains(t, declared[route.method], route.path,
			"API-key policy must be declared for %s %s", route.method, route.path)
	}
	// The startup self-check (every declared policy matches a real route)
	// must hold with the module-mounted routes.
	g.assertAPIKeyPoliciesMatchRoutes(r)
}
