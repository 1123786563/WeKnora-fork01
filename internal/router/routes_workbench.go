package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/gin-gonic/gin"
)

// RegisterWorkbenchRoutes exposes the versioned, ownership-scoped mobile
// execution read API. The handler performs the final owner predicate; this
// route guard only establishes the existing Viewer/API-key boundary.
func RegisterWorkbenchRoutes(r *gin.RouterGroup, h *session.WorkbenchReadHandler, g *rbacGuards, targetHandlers ...*handler.ExecutionTargetHandler) {
	if g == nil {
		return
	}
	if h != nil {
		executions := r.Group("/workbench/executions", g.Viewer())
		workbench := g.apiKeyGroup(executions, apiKeyChat(apiKeyFullAccess()))
		workbench.GET("/:run_id", h.GetWorkbenchExecution)
		workbench.GET("/:run_id/snapshot", h.GetWorkbenchSnapshot)
		workbench.GET("/:run_id/events", h.StreamWorkbenchEvents)
		workbench.POST("/:run_id/source-events", h.IngestWorkbenchSourceEvent)
	}
	var targetHandler *handler.ExecutionTargetHandler
	if len(targetHandlers) > 0 {
		targetHandler = targetHandlers[0]
	}
	if targetHandler != nil {
		targets := g.apiKeyGroup(r.Group("/execution-targets", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
		targets.GET("", targetHandler.List)
		targets.GET("/:id", targetHandler.Get)
		targets.POST("", targetHandler.Create)
		targets.DELETE("/:id", targetHandler.Revoke)
		workspaces := g.apiKeyGroup(r.Group("/execution-workspaces", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
		workspaces.GET("/:id", targetHandler.GetWorkspace)
	}
}

// RegisterWorkbenchOverviewRoutes exposes the aggregate workbench read model
// (MX-013, B-class GET /workbench/overview). Same Viewer/API-key boundary as
// the per-run reads; the handler applies the tenant+owner predicate.
func RegisterWorkbenchOverviewRoutes(r *gin.RouterGroup, h *session.WorkbenchOverviewHandler, g *rbacGuards) {
	if h == nil || g == nil {
		return
	}
	overview := g.apiKeyGroup(r.Group("/workbench", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
	overview.GET("/overview", h.Overview)
}

// RegisterWorkbenchInboxRoutes exposes the notification inbox read model and
// device registration (MX-021, B-class). Notifications are hints only: no
// approval bodies, no authorization semantics on the client.
func RegisterWorkbenchInboxRoutes(r *gin.RouterGroup, h *session.WorkbenchInboxHandler, g *rbacGuards) {
	if h == nil || g == nil {
		return
	}
	inbox := g.apiKeyGroup(r.Group("/workbench", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
	inbox.GET("/inbox", h.Inbox)
	inbox.POST("/inbox/read", h.MarkRead)
	inbox.POST("/inbox/devices", h.RegisterDevice)
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
