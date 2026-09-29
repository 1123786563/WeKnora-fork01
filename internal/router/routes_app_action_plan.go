package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterAppActionPlanRoutes registers the T21 (#51) multi-action
// Action Plan surface under /api/v1/apps/action-plans. Like the
// notion-publish routes, these are intentionally NOT declared in the
// API-key route authorizer: the /api/v1 gate default-denies every
// X-API-Key principal. The group carries the action write gate; the
// approval authority predicate lives in the handler (initiator or
// tenant owner/admin).
func RegisterAppActionPlanRoutes(r *gin.RouterGroup, h *handler.AppActionPlanHandler) {
	if h == nil {
		return
	}
	g := r.Group("/apps/action-plans", h.RequireActionCapabilityForWrites())
	{
		g.POST("", h.FormActionPlan)
		g.POST("/:id/approve", h.ApproveActionPlan)
		g.POST("/:id/execute", h.ExecuteActionPlan)
		g.GET("/:id", h.GetActionPlan)
	}
}
