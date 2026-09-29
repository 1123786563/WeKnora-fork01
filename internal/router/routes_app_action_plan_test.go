package router

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// TestActionPlanRoutesNilHandlerRegistersSilently: the router guard —
// nil handler registers nothing and panics nowhere (mirror of #48).
func TestActionPlanRoutesNilHandlerRegistersSilently(t *testing.T) {
	gin.SetMode(gin.TestMode)
	RegisterAppActionPlanRoutes(gin.New().Group("/api/v1"), nil)
}

// TestActionPlanRoutesRegisterWithHandler: the four endpoints exist on
// the engine after registration (route-existence smoke).
func TestActionPlanRoutesRegisterWithHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := handler.NewAppActionPlanHandler(nil) // constructor only stores the field
	r := gin.New()
	RegisterAppActionPlanRoutes(r.Group("/api/v1"), h)
	found := map[string]bool{}
	for _, ri := range r.Routes() {
		found[ri.Method+" "+ri.Path] = true
	}
	for _, want := range []string{
		"POST /api/v1/apps/action-plans",
		"POST /api/v1/apps/action-plans/:id/approve",
		"POST /api/v1/apps/action-plans/:id/execute",
		"GET /api/v1/apps/action-plans/:id",
	} {
		if !found[want] {
			t.Fatalf("route missing: %s (registered: %v)", want, found)
		}
	}
}
