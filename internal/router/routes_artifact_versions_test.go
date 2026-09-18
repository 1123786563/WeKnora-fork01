package router

import (
	"net/http"
	"testing"

	"github.com/Tencent/WeKnora/internal/handler"
	sessionhandler "github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/gin-gonic/gin"
)

// W26 wiring tests: the versioned artifact download entry must be reachable
// through the real RegisterSessionRoutes registration (not only through a
// test-built gin router), and it must stay fail-closed when the container
// assembly did not register a version source — mirroring the craft route
// mounting pattern in routes_chat.go.

const artifactVersionDownloadRoute = "/api/v1/sessions/:id/artifact-versions/:version_id/download"

func routesWithArtifactVersionAssembly(t *testing.T, wired bool) []gin.RouteInfo {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	v1 := engine.Group("/api/v1")
	if wired {
		sessionhandler.RegisterArtifactVersionDownloadHandler(&sessionhandler.ArtifactVersionDownloadHandler{})
		t.Cleanup(func() { sessionhandler.RegisterArtifactVersionDownloadHandler(nil) })
	}
	RegisterSessionRoutes(v1, &sessionhandler.Handler{}, &handler.MessageSuggestionHandler{}, &rbacGuards{})
	return engine.Routes()
}

func hasRoute(routes []gin.RouteInfo, method, path string) bool {
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}

func TestArtifactVersionDownloadRouteMounted(t *testing.T) {
	routes := routesWithArtifactVersionAssembly(t, true)
	if !hasRoute(routes, http.MethodGet, artifactVersionDownloadRoute) {
		t.Fatalf("GET %s must be registered by RegisterSessionRoutes when the assembly is wired", artifactVersionDownloadRoute)
	}
	// The legacy index-based entry stays mounted alongside it.
	if !hasRoute(routes, http.MethodGet, "/api/v1/sessions/:id/messages/:message_id/artifacts/:index/download") {
		t.Fatal("legacy message-index artifact download route must stay registered")
	}
}

func TestArtifactVersionDownloadRouteFailClosedWithoutAssembly(t *testing.T) {
	routes := routesWithArtifactVersionAssembly(t, false)
	if hasRoute(routes, http.MethodGet, artifactVersionDownloadRoute) {
		t.Fatalf("GET %s must not exist when the container assembly is not wired (fail-closed mounting)", artifactVersionDownloadRoute)
	}
}
