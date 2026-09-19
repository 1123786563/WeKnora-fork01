package router

// The admin query-history snapshot route must coexist with the session and
// usage routes on one engine: gin keeps a radix tree per verb and panics at
// registration on wildcard-name conflicts, so mounting all three together is
// the executable proof the router still boots. The test also pins the exact
// path so the Admin+ audit surface cannot silently move.

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler"
	sessionhandler "github.com/Tencent/WeKnora/internal/handler/session"
)

func TestRegisterQueryHistoryAdminRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := &rbacGuards{cfg: &config.Config{}}
	v1 := r.Group("/api/v1")

	// Neighbouring surfaces that share the verb trees.
	RegisterSessionRoutes(v1, &sessionhandler.Handler{}, &handler.MessageSuggestionHandler{}, g)
	RegisterUsageRoutes(v1, &handler.UsageHandler{}, g)
	RegisterQueryHistoryAdminRoutes(v1, &sessionhandler.Handler{}, g)

	seen := map[string]bool{}
	for _, route := range r.Routes() {
		seen[route.Method+" "+route.Path] = true
	}
	require.True(t, seen[http.MethodGet+" /api/v1/admin/sessions/:session_id/snapshot"])
	// The pre-existing routes stay intact.
	require.True(t, seen[http.MethodGet+" /api/v1/sessions/:id"])
	require.True(t, seen[http.MethodGet+" /api/v1/admin/usage/by-user"])
}
