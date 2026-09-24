package router

import (
	"github.com/Tencent/WeKnora/internal/modules/career"
	"github.com/gin-gonic/gin"
)

func RegisterCareerRoutes(r *gin.RouterGroup, h *career.Handler) {
	if h == nil {
		return
	}
	g := r.Group("/career")
	g.GET("/open", h.Open)
	g.GET("/list", h.List)
	g.GET("/changes", h.Changes)
	g.GET("/receipt", h.Receipt)
	g.GET("/sources", h.Sources)
	g.POST("/sources/upload", h.Upload)
	g.POST("/act", h.Act)
	g.POST("/opportunities/import", h.ImportJD)
	g.POST("/opportunities/import-url", h.ImportURL)
	g.GET("/opportunities/receipt", h.OpportunityReceipt)
	g.GET("/opportunities/:opportunityId", h.OpportunityEvidence)
	g.GET("/opportunities/:opportunityId/observations", h.OpportunityObservations)
	g.POST("/evaluations", h.EvaluateOpportunity)
	g.GET("/evaluations/receipt", h.EvaluationReceipt)
	g.GET("/evaluations/:evaluationId", h.Evaluation)
}
