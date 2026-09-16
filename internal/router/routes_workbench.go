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

// RegisterWorkbenchStartRoutes adds the write and request-reconciliation
// endpoints. They share the same authenticated API-key policy as reads.
func RegisterWorkbenchStartRoutes(r *gin.RouterGroup, h *session.WorkbenchStartHandler, g *rbacGuards) {
	if h == nil || g == nil {
		return
	}
	executions := r.Group("/workbench/executions", g.Viewer())
	workbench := g.apiKeyGroup(executions, apiKeyChat(apiKeyFullAccess()))
	workbench.POST("", h.Start)
	workbench.GET("/requests/:request_id", h.Lookup)
}

// RegisterWorkbenchCommandRoutes exposes typed interaction decisions and the
// closed cancel/steer command union. The handler is optional while deployments
// are migrating their durable approval adapter; no unsafe fallback is used.
func RegisterWorkbenchCommandRoutes(r *gin.RouterGroup, h *session.WorkbenchCommandHandler, g *rbacGuards) {
	if h == nil || g == nil {
		return
	}
	executions := r.Group("/workbench/executions", g.Viewer())
	workbench := g.apiKeyGroup(executions, apiKeyChat(apiKeyFullAccess()))
	workbench.GET("/:run_id/interactions", h.ListInteractions)
	workbench.POST("/interactions/:id/decisions", h.DecideInteraction)
	workbench.POST("/:run_id/commands", h.Command)
}
