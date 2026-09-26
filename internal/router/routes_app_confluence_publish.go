package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterAppConfluencePublishRoutes registers the T20 Confluence publish
// closed loop under /api/v1/apps/confluence-publish. Like the
// app-connector routes, these are intentionally NOT declared in the
// API-key route authorizer: the /api/v1 gate default-denies every
// X-API-Key principal. The group carries the action write gate; approval
// authority stays on the existing POST /apps/actions/:id/approve predicate.
func RegisterAppConfluencePublishRoutes(r *gin.RouterGroup, h *handler.AppConfluencePublishHandler) {
	if h == nil {
		return
	}
	g := r.Group("/apps/confluence-publish", h.RequireActionCapabilityForWrites())
	{
		g.POST("/plans", h.FormConfluencePublishPlan)
		g.POST("/actions/:id/publish", h.PublishConfluenceAction)
		g.POST("/actions/:id/reconcile", h.ReconcileConfluenceAction)
		g.GET("/actions/:id", h.GetConfluencePublication)
	}
}
