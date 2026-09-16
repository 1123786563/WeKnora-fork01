package router

import (
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/gin-gonic/gin"
)

// RegisterWorkbenchRoutes exposes the versioned, ownership-scoped mobile
// execution read API. The handler performs the final owner predicate; this
// route guard only establishes the existing Viewer/API-key boundary.
func RegisterWorkbenchRoutes(r *gin.RouterGroup, h *session.WorkbenchReadHandler, g *rbacGuards) {
	if h == nil || g == nil {
		return
	}
	executions := r.Group("/workbench/executions", g.Viewer())
	workbench := g.apiKeyGroup(executions, apiKeyChat(apiKeyFullAccess()))
	workbench.GET("/:run_id", h.GetWorkbenchExecution)
	workbench.GET("/:run_id/snapshot", h.GetWorkbenchSnapshot)
	workbench.GET("/:run_id/events", h.StreamWorkbenchEvents)
}
