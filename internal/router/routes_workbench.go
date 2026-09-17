package router

import (
	"net/http"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/gin-gonic/gin"
)

// workbenchReadGate is the W34 read_enabled lane: while workbench reads are
// switched off (workbench.read_enabled / WEKNORA_WORKBENCH_READ_ENABLED),
// the workbench read endpoints answer 503 before any handler runs. Writes,
// admission, and cleanup are deliberately NOT gated here — one switch never
// cuts query and cleanup at the same time. A nil or unset config keeps the
// lane open (safe-on default).
func workbenchReadGate(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cfg.AreWorkbenchReadsEnabled() {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "workbench reads are disabled",
			})
			return
		}
		c.Next()
	}
}

// RegisterWorkbenchRoutes exposes the versioned, ownership-scoped mobile
// execution read API. The handler performs the final owner predicate; this
// route guard only establishes the existing Viewer/API-key boundary.
func RegisterWorkbenchRoutes(r *gin.RouterGroup, h *session.WorkbenchReadHandler, list *session.WorkbenchListHandler, g *rbacGuards, targetHandlers ...*handler.ExecutionTargetHandler) {
	if g == nil {
		return
	}
	if list != nil {
		executions := r.Group("/workbench/executions", g.Viewer(), workbenchReadGate(g.cfg))
		workbench := g.apiKeyGroup(executions, apiKeyChat(apiKeyFullAccess()))
		workbench.GET("", list.ListWorkbenchExecutions)
	}
	if h != nil {
		executions := r.Group("/workbench/executions", g.Viewer(), workbenchReadGate(g.cfg))
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

// RegisterWorkbenchStartRoutes adds the write and request-reconciliation
// endpoints. They share the same authenticated API-key policy as reads.
// Lookup is a status query, so the W34 read gate applies to it too; Start
// (the write path) is gated by the admission coordinator's capability gate
// instead (drain / platform_admission).
func RegisterWorkbenchStartRoutes(r *gin.RouterGroup, h *session.WorkbenchStartHandler, g *rbacGuards) {
	if h == nil || g == nil {
		return
	}
	executions := r.Group("/workbench/executions", g.Viewer())
	workbench := g.apiKeyGroup(executions, apiKeyChat(apiKeyFullAccess()))
	workbench.POST("", h.Start)
	gated := g.apiKeyGroup(r.Group("/workbench/executions/requests", g.Viewer(), workbenchReadGate(g.cfg)), apiKeyChat(apiKeyFullAccess()))
	gated.GET("/:request_id", h.Lookup)
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
