package router

import (
	"net/http"
	"testing"

	"github.com/Tencent/WeKnora/internal/handler"
	sessionhandler "github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/gin-gonic/gin"
)

// W27 wiring tests: the isolated artifact preview's ticket-issuance entry
// must be reachable through the real RegisterSessionRoutes registration
// (inheriting the sessions auth chain), and it must stay fail-closed when
// the container assembly did not register a preview handler — mirroring the
// W26 versioned-download mounting tests.
//
// The isolated-origin /ap/<token> redemption route is mounted by NewRouter
// itself (router.go, before the global Auth middleware — the craft preview
// one-liner position); its fail-closed nil behaviour is asserted in the
// handler package (TestArtifactPreviewFile_DisabledOrUnwiredIs404) and the
// handler's RegisterArtifactPreviewRoutes is what that one-liner calls.

const artifactPreviewIssueRoute = "/api/v1/sessions/:session_id/artifact-versions/:version_id/preview-ticket"

func routesWithArtifactPreviewAssembly(t *testing.T, wired bool) []gin.RouteInfo {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	v1 := engine.Group("/api/v1")
	if wired {
		// Production runs both container Invokes (W26 downloads, W27
		// preview), so the wired case registers both handlers.
		sessionhandler.RegisterArtifactVersionDownloadHandler(&sessionhandler.ArtifactVersionDownloadHandler{})
		t.Cleanup(func() { sessionhandler.RegisterArtifactVersionDownloadHandler(nil) })
		handler.RegisterArtifactPreviewHandler(&handler.ArtifactPreviewHandler{})
		t.Cleanup(func() { handler.RegisterArtifactPreviewHandler(nil) })
	}
	RegisterSessionRoutes(v1, &sessionhandler.Handler{}, &handler.MessageSuggestionHandler{}, &rbacGuards{})
	return engine.Routes()
}

func TestArtifactPreviewIssueRouteMounted(t *testing.T) {
	routes := routesWithArtifactPreviewAssembly(t, true)
	if !hasRoute(routes, http.MethodPost, artifactPreviewIssueRoute) {
		t.Fatalf("POST %s must be registered by RegisterSessionRoutes when the assembly is wired", artifactPreviewIssueRoute)
	}
	// The W26 versioned download and the legacy index entry stay mounted.
	if !hasRoute(routes, http.MethodGet, "/api/v1/sessions/:id/artifact-versions/:version_id/download") {
		t.Fatal("W26 versioned artifact download route must stay registered")
	}
	if !hasRoute(routes, http.MethodGet, "/api/v1/sessions/:id/messages/:message_id/artifacts/:index/download") {
		t.Fatal("legacy message-index artifact download route must stay registered")
	}
}

func TestArtifactPreviewIssueRouteFailClosedWithoutAssembly(t *testing.T) {
	routes := routesWithArtifactPreviewAssembly(t, false)
	if hasRoute(routes, http.MethodPost, artifactPreviewIssueRoute) {
		t.Fatalf("POST %s must not exist when the container assembly is not wired (fail-closed mounting)", artifactPreviewIssueRoute)
	}
}

// The isolated-origin redemption route exists exactly when a handler is
// registered: the same one-liner NewRouter runs (craft preview position).
func TestArtifactPreviewIsolatedRouteMountsOnlyWithHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handler.RegisterArtifactPreviewRoutes(engine, nil)
	for _, route := range engine.Routes() {
		if route.Path == "/ap/:token" {
			t.Fatal("no /ap route may exist without a registered handler")
		}
	}

	wired := gin.New()
	handler.RegisterArtifactPreviewRoutes(wired, &handler.ArtifactPreviewHandler{})
	foundGET, foundHEAD := false, false
	for _, route := range wired.Routes() {
		if route.Path != "/ap/:token" {
			t.Fatalf("preview origin must expose only /ap/:token, found %s %s", route.Method, route.Path)
		}
		if route.Method == http.MethodGet {
			foundGET = true
		}
		if route.Method == http.MethodHead {
			foundHEAD = true
		}
	}
	if !foundGET || !foundHEAD {
		t.Fatalf("preview origin must mount GET+HEAD /ap/:token (got GET=%v HEAD=%v)", foundGET, foundHEAD)
	}
}
