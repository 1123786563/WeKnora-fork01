package router

import (
	"net/http"
	"testing"

	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/gin-gonic/gin"
)

func TestRegisterCareerArtifactRoutesExposesOnlyIssueOnAuthenticatedGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	issueGroup := r.Group("/api/v1")
	h := session.NewCareerArtifactHandler(nil, nil, nil, nil)
	RegisterCareerArtifactRoutes(issueGroup, h)
	RegisterCareerArtifactDownloadRoute(r, h)
	routes := map[string]bool{}
	for _, route := range r.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	if !routes[http.MethodPost+" /api/v1/career/resources/:resource_id/versions/:version_id/signed-url"] {
		t.Fatal("authenticated issuance route missing")
	}
	if !routes[http.MethodGet+" /api/v1/career/artifacts/download"] {
		t.Fatal("public signed download route missing")
	}
}
