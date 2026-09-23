package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterPluginRoutes registers the plugin routes.
//
// Plugins are tenant-level integrations introduced by manifest install
// (issue #106). The preview endpoint makes WeKnora fetch an untrusted URL
// and mount the review surface for what was verified — an admin-only
// action on both counts. It is deliberately NOT declared for API keys
// (default-deny for X-API-Key principals): plugin preview is an
// interactive admin flow with human review as its safety gate.
func RegisterPluginRoutes(r *gin.RouterGroup, pluginHandler *handler.PluginHandler, g *rbacGuards) {
	pluginRoutes := r.Group("/plugins")
	{
		// Preview a manifest before install — Admin+.
		pluginRoutes.POST("/installations/preview", g.Admin(), pluginHandler.PreviewManifest)
	}
}
