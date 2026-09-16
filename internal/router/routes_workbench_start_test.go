package router

import (
	"net/http"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterWorkbenchStartRoutesDeclaresStartAndLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := &rbacGuards{cfg: &config.Config{}}
	RegisterWorkbenchStartRoutes(r.Group("/api/v1"), session.NewWorkbenchStartHandler(nil), g)
	routes := r.Routes()
	seen := map[string]bool{}
	for _, route := range routes {
		seen[route.Method+" "+route.Path] = true
	}
	require.True(t, seen[http.MethodPost+" /api/v1/workbench/executions"])
	require.True(t, seen[http.MethodGet+" /api/v1/workbench/executions/requests/:request_id"])
}
