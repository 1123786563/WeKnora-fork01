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
//
// Install slice (T06/T07): confirm/state are Admin-only governance; the
// discovery endpoints are Viewer+ — every member of the tenant can see
// WHAT is installed (B4 discovery), while cross-tenant isolation is
// enforced in the service layer (foreign IDs read as not found). Same
// default-deny for API keys as preview: these are interactive workspace
// governance flows.
func RegisterPluginRoutes(r *gin.RouterGroup, pluginHandler *handler.PluginHandler, g *rbacGuards) {
	pluginRoutes := r.Group("/plugins")
	{
		// Static segment first, then :id — gin wildcard discipline
		// (routes_agent.go "/placeholders before /:id" precedent).
		pluginRoutes.POST("/installations/preview", g.Admin(), pluginHandler.PreviewManifest)
		pluginRoutes.POST("/installations", g.Admin(), pluginHandler.ConfirmInstallation)
		pluginRoutes.POST("/installations/:id/disable", g.Admin(), pluginHandler.DisableInstallation)
		pluginRoutes.POST("/installations/:id/enable", g.Admin(), pluginHandler.EnableInstallation)
		// Ops-only self-heal channel (rulings.md R4): removes a failed
		// confirm's leftover rows. NOT a user-facing feature — the
		// user-visible governance endpoint stays "disable" and the web UI
		// must never surface a delete/uninstall entry. Guarded at
		// SystemAdmin, NOT workspace Admin (T06-OCR1-F1): the workspace
		// admin's sanctioned lever is disable (reversible, keeps audit
		// trails); a hard cascade that destroys member approvals and
		// releases the unique slot is an operator action.
		pluginRoutes.DELETE("/installations/:id", g.SystemAdmin(), pluginHandler.UninstallInstallation)
		pluginRoutes.GET("/installations", g.Viewer(), pluginHandler.ListInstallations)
		pluginRoutes.GET("/installations/:id", g.Viewer(), pluginHandler.GetInstallation)
	}
}
