package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterAppNotionPublishRoutes registers the T18 Notion publish closed
// loop under /api/v1/apps/notion-publish. Like the app-connector routes,
// these are intentionally NOT declared in the API-key route authorizer:
// the /api/v1 gate default-denies every X-API-Key principal. The group
// carries the action write gate; approval authority stays on the existing
// POST /apps/actions/:id/approve predicate.
func RegisterAppNotionPublishRoutes(r *gin.RouterGroup, h *handler.AppNotionPublishHandler) {
	if h == nil {
		return
	}
	g := r.Group("/apps/notion-publish", h.RequireActionCapabilityForWrites())
	{
		g.POST("/plans", h.FormNotionPublishPlan)
		g.POST("/actions/:id/publish", h.PublishNotionAction)
		g.POST("/actions/:id/reconcile", h.ReconcileNotionAction)
		g.GET("/actions/:id", h.GetNotionPublication)
	}
}
