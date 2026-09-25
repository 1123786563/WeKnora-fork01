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
	g.POST("/applications", h.CreateApplication)
	g.GET("/applications/receipt", h.ApplicationReceipt)
	g.GET("/applications/:applicationId", h.GetApplication)
	g.POST("/applications/link/reconcile", h.ReconcileApplicationLink)
	g.POST("/searches", h.SearchOnce)
	g.GET("/searches/receipt", h.SearchReceipt)
	g.GET("/searches/:searchId", h.GetSearch)
	g.POST("/materials", h.EditMaterial)
	g.POST("/materials/confirm", h.ConfirmMaterialBody)
	g.GET("/materials/receipt", h.MaterialReceiptHandler)
	g.GET("/materials/:materialId", h.GetMaterial)
	g.GET("/materials/:materialId/versions", h.ListMaterialVersions)
	g.GET("/materials/:materialId/versions/:versionId", h.GetMaterialVersion)
	g.GET("/materials/:materialId/versions/:versionId/compare", h.CompareMaterialVersions)
	g.POST("/materials/:materialId/exports", h.PublishMaterialHandler)
	g.GET("/materials/:materialId/exports", h.ListMaterialExports)
	g.POST("/materials/:materialId/exports/:exportId/signed-url", h.MaterialExportSignedURL)
	g.GET("/materials/:materialId/exports/:exportId/download", h.DownloadMaterialExport)
	g.DELETE("/materials/:materialId/exports/:exportId", h.RevokeMaterialExport)
	g.POST("/applications/:applicationId/progress", h.AppendProgress)
	g.POST("/applications/:applicationId/progress/correct", h.CorrectProgress)
	g.GET("/applications/:applicationId/progress", h.ApplicationProgress)
	g.GET("/progress/receipt", h.ProgressReceiptHandler)
}
