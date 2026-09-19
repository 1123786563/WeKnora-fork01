package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler"
)

// TestRegisterFeedbackRoutesCoexistWithMessageRoutes: the feedback endpoints
// mount next to the existing message routes without tripping gin's per-verb
// wildcard-name rule — the DELETE tree reuses :id because
// DELETE /messages/:session_id/:id already owns that position. Registering
// both on one engine would panic at startup on any violation, so this test
// is the executable proof the router still boots.
func TestRegisterFeedbackRoutesCoexistWithMessageRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := &rbacGuards{cfg: &config.Config{}}
	v1 := r.Group("/api/v1")
	// A nil *handler.MessageHandler is fine here: only the path declarations
	// matter; gin never invokes the handlers without a request.
	RegisterMessageRoutes(v1, nil, g)
	RegisterFeedbackRoutes(v1, &handler.FeedbackHandler{}, g)

	seen := map[string]bool{}
	for _, route := range r.Routes() {
		seen[route.Method+" "+route.Path] = true
	}
	require.True(t, seen[http.MethodPost+" /api/v1/messages/:session_id/:message_id/feedback"])
	require.True(t, seen[http.MethodDelete+" /api/v1/messages/:session_id/:id/feedback"])
	require.True(t, seen[http.MethodGet+" /api/v1/messages/:session_id/feedback/mine"])
	// The pre-existing message routes stay intact.
	require.True(t, seen[http.MethodDelete+" /api/v1/messages/:session_id/:id"])
	require.True(t, seen[http.MethodGet+" /api/v1/messages/:session_id/load"])
}
