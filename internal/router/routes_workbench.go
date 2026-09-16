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

// RegisterExecutionRegistrationRoutes exposes only the authenticated personal
// node control plane. The handler still rechecks tenant and owner predicates;
// the route guard is not an ownership substitute.
func RegisterExecutionRegistrationRoutes(r *gin.RouterGroup, h *handler.ExecutionRegistrationHandler, g *rbacGuards, targetHandlers ...*handler.ExecutionTargetHandler) {
	if h == nil || g == nil {
		return
	}
	registrations := g.apiKeyGroup(r.Group("/execution-registrations", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
	registrations.POST("/challenges", h.CreateChallenge)
	registrations.POST("", h.Complete)
	registrations.DELETE("/:id", h.Revoke)
	// W23 contract routes. The legacy aliases above remain during the published
	// compatibility window; all new clients use the execution-target facade.
	targetRegistrations := g.apiKeyGroup(r.Group("/execution-targets/registrations", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
	targetRegistrations.POST("/challenges", h.CreateChallenge)
	targetRegistrations.POST("", h.Complete)
	targets := g.apiKeyGroup(r.Group("/execution-targets", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
	if len(targetHandlers) > 0 && targetHandlers[0] != nil {
		targets.POST("/:id/revoke", targetHandlers[0].Revoke)
	} else {
		// Keep the legacy registration handler as a compatibility fallback for
		// callers that have not yet supplied the target facade. The production
		// router always passes ExecutionTargetHandler so this route revokes all
		// target projections in one transaction.
		targets.POST("/:id/revoke", h.Revoke)
	}
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
