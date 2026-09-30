package container

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWireCraftInputFeatureRegistersOnlyOnItsAssembly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	first := session.NewCraftFeatureRoutes()
	second := session.NewCraftFeatureRoutes()
	svc := &service.CraftSessionService{}

	require.NoError(t, wireCraftInputFeature(svc, first))
	require.Error(t, wireCraftInputFeature(svc, first), "duplicate registration must fail closed")

	mount := func(routes *session.CraftFeatureRoutes) *gin.Engine {
		r := gin.New()
		require.NoError(t, routes.Mount(r.Group("/sessions")))
		return r
	}
	firstRouter := mount(first)
	secondRouter := mount(second)
	assertRoutePresent(t, firstRouter, "POST", "/sessions/:session_id/craft/input-rounds", true)
	assertRoutePresent(t, firstRouter, "POST", "/sessions/:session_id/craft/inputs/decision", true)
	assertRoutePresent(t, secondRouter, "POST", "/sessions/:session_id/craft/input-rounds", false)
	assertRoutePresent(t, secondRouter, "POST", "/sessions/:session_id/craft/inputs/decision", false)
}

func TestWireCraftInputFeatureFailsClosedForMissingDependencies(t *testing.T) {
	routes := session.NewCraftFeatureRoutes()
	require.Error(t, wireCraftInputFeature(nil, routes))
	require.Error(t, wireCraftInputFeature(&service.CraftSessionService{}, nil))
}

func assertRoutePresent(t *testing.T, r *gin.Engine, method, path string, want bool) {
	t.Helper()
	for _, route := range r.Routes() {
		if route.Method == method && route.Path == path {
			require.True(t, want, "unexpected route %s %s", method, path)
			return
		}
	}
	require.False(t, want, "missing route %s %s", method, path)
}
