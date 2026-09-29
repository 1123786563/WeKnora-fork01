package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterAppFeishuPublishRoutes registers the T19 (#49) Feishu publish
// closed loop under /api/v1/apps/feishu-publish. Like the notion publish
// routes, these are intentionally NOT declared in the API-key route
// authorizer: the /api/v1 gate default-denies every X-API-Key principal.
func RegisterAppFeishuPublishRoutes(r *gin.RouterGroup, h *handler.AppFeishuPublishHandler) {
	if h == nil {
		return
	}
	g := r.Group("/apps/feishu-publish", h.RequireActionCapabilityForWrites())
	{
		g.POST("/plans", h.FormFeishuPublishPlan)
		g.POST("/actions/:id/publish", h.PublishFeishuAction)
		g.POST("/actions/:id/reconcile", h.ReconcileFeishuAction)
		g.GET("/actions/:id", h.GetFeishuPublication)
	}
}
