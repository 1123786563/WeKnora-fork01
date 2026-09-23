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
	g.POST("/act", h.Act)
}
