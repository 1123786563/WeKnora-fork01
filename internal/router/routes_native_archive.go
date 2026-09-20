package router

import (
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/gin-gonic/gin"
)

// RegisterNativeArchiveRoutes mounts the immutable historical namespace. It
// shares the established Viewer/API-key read boundary; the handler and its
// service recheck the current tenant/resource scope before every lookup.
func RegisterNativeArchiveRoutes(r *gin.RouterGroup, h *session.NativeArchiveHandler, g *rbacGuards) {
	if r == nil || h == nil || g == nil {
		return
	}
	archive := g.apiKeyGroup(r.Group("/agent-archive", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
	archive.GET("/sessions", h.List)
	archive.GET("/records/:id", h.GetRecord)
	archive.GET("/artifacts/:id", h.GetArtifact)
}
