package router

import (
	"net/http"
	"testing"

	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/gin-gonic/gin"
)

// Archive is a separate namespace: its three reads and every attempted
// mutation must be mounted together, so an old client cannot get a generic
// 404/405 instead of the frozen archive_read_only protocol response.
func TestNativeArchiveRoutesMountReadAndMutationRejection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterNativeArchiveRoutes(r.Group("/api/v1"), &session.NativeArchiveHandler{}, &rbacGuards{})

	routes := r.Routes()
	for _, want := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/agent-archive/sessions"},
		{http.MethodGet, "/api/v1/agent-archive/records/:id"},
		{http.MethodGet, "/api/v1/agent-archive/artifacts/:id"},
		{http.MethodPost, "/api/v1/agent-archive/*path"},
		{http.MethodPut, "/api/v1/agent-archive/*path"},
		{http.MethodPatch, "/api/v1/agent-archive/*path"},
		{http.MethodDelete, "/api/v1/agent-archive/*path"},
	} {
		if !hasRoute(routes, want.method, want.path) {
			t.Fatalf("%s %s is not registered", want.method, want.path)
		}
	}
}
