package router

// The session-share routes (SP13 Task 5) must coexist with the full session
// route table on one engine: gin keeps a radix tree per verb and panics at
// registration on wildcard-name conflicts, so mounting both together is the
// executable proof the router still boots. The test also pins the exact
// paths and the new /shared top-level segment.

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler"
	sessionhandler "github.com/Tencent/WeKnora/internal/handler/session"
)

func TestRegisterSessionShareRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := &rbacGuards{cfg: &config.Config{}}
	v1 := r.Group("/api/v1")

	// The neighbouring surfaces sharing the verb trees.
	RegisterSessionRoutes(v1, &sessionhandler.Handler{}, &handler.MessageSuggestionHandler{}, g)
	RegisterFeedbackRoutes(v1, &handler.FeedbackHandler{}, g)
	RegisterQueryHistoryAdminRoutes(v1, &sessionhandler.Handler{}, g)
	RegisterSessionShareRoutes(v1, &sessionhandler.Handler{}, g)

	seen := map[string]bool{}
	for _, route := range r.Routes() {
		seen[route.Method+" "+route.Path] = true
	}
	require.True(t, seen[http.MethodPost+" /api/v1/sessions/:session_id/share"],
		"the POST tree binds :session_id like its sibling POST routes")
	require.True(t, seen[http.MethodDelete+" /api/v1/sessions/:id/share"],
		"the DELETE tree reuses :id like its sibling DELETE routes")
	require.True(t, seen[http.MethodGet+" /api/v1/shared/sessions/:token"],
		"the read-only share view is a new /shared top-level segment")
	// The pre-existing routes stay intact.
	require.True(t, seen[http.MethodGet+" /api/v1/sessions/:id"])
	require.True(t, seen[http.MethodPost+" /api/v1/sessions/:session_id/pin"])
	require.True(t, seen[http.MethodDelete+" /api/v1/sessions/:id/pin"])
	require.True(t, seen[http.MethodGet+" /api/v1/admin/sessions/:session_id/snapshot"])
}
