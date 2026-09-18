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
		// source-events is the Paseo bridge's authenticated write callback
		// (remote nodes reporting events back), not a read: mounting it in
		// the gated group would let WEKNORA_WORKBENCH_READ_ENABLED=false
		// sever remote event ingestion — the read gate never gates writes.
		writes := g.apiKeyGroup(r.Group("/workbench/executions", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
		writes.POST("/:run_id/source-events", h.IngestWorkbenchSourceEvent)
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

// RegisterWorkbenchArtifactRoutes exposes the execution-artifact surfaces:
// an owned-run metadata list and short-lived signed download links, plus the
// credential-free download endpoint those links point at. The download route
// carries no login session — the HMAC grant is the authorization fact.
func RegisterWorkbenchArtifactRoutes(r *gin.RouterGroup, artifacts *session.WorkbenchArtifactHandler, sessionHandler *session.Handler, g *rbacGuards) {
	if g == nil {
		return
	}
	if artifacts != nil {
		executions := r.Group("/workbench/executions", g.Viewer())
		workbench := g.apiKeyGroup(executions, apiKeyChat(apiKeyFullAccess()))
		workbench.GET("/:run_id/artifacts", artifacts.ListWorkbenchArtifacts)
		workbench.POST("/:run_id/artifacts/:index/signed-url", artifacts.CreateWorkbenchArtifactSignedURL)
	}
	// The credential-free download endpoint is registered on the root router
	// before the global Auth middleware (see router.go); nothing to do here.
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

// RegisterMobileVoiceRoutes exposes the W30 mobile voice surface: the
// authorized short-lived session (POST), its stop/settle path (DELETE) and
// the server-proxied transcription consumed by W29's dictation port. The
// handler performs the product-context ownership predicate; this guard only
// establishes the existing Viewer/API-key boundary, matching the workbench
// lanes. A nil handler mounts nothing (the container fails closed instead
// of assembling a half-wired voice plane).
func RegisterMobileVoiceRoutes(r *gin.RouterGroup, h *handler.MobileVoiceHandler, g *rbacGuards) {
	if h == nil || g == nil {
		return
	}
	voice := g.apiKeyGroup(r.Group("/mobile/voice", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
	voice.POST("/sessions", h.CreateVoiceSession)
	voice.DELETE("/sessions/:id", h.StopVoiceSession)
	voice.POST("/transcriptions", h.TranscribeAudio)
}

// RegisterMobileDeviceRoutes keeps device registration under the same
// authenticated API boundary as the workbench. The handler derives owner and
// tenant from the auth context; neither route parameter nor body can rewrite
// that scope.
func RegisterMobileDeviceRoutes(r *gin.RouterGroup, h *handler.MobileDeviceHandler, g *rbacGuards) {
	if h == nil || g == nil {
		return
	}
	devices := g.apiKeyGroup(r.Group("/mobile/devices", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
	devices.GET("", h.List)
	devices.POST("/:id/registration-intent", h.IssueIntent)
	devices.PUT("/:id", h.Register)
	devices.DELETE("/:id", h.Revoke)
	devices.POST("/:id/presence", h.Presence)
	devices.GET("/:id/presence", h.GetPresence)
	devices.PUT("/:id/presence", h.PutPresence)
	devices.DELETE("/:id/presence", h.DeletePresence)
}
