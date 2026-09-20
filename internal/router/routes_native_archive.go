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
	// The archive namespace is immutable.  Catch every mutation verb rather
	// than leaving legacy resume/continue/write paths to answer generic 404s.
	archive.POST("/*path", h.RejectMutation)
	archive.PUT("/*path", h.RejectMutation)
	archive.PATCH("/*path", h.RejectMutation)
	archive.DELETE("/*path", h.RejectMutation)
}
