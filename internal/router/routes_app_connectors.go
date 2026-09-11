package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterAppConnectorRoutes registers the W04 app-connector endpoints under
// /api/v1/apps. Like the commercial routes, the brief-mandated signature
// carries no rbacGuards: these routes are intentionally NOT declared in the
// API-key route authorizer, so the /api/v1 gate default-denies every
// X-API-Key principal (403) - a machine key never auto-inherits app install
// or connection authority. The handler layer adds its own gates: tenant scope
// always comes from the authenticated context, and every write requires
// appconnector.CanInstallInstallation (owner/admin; members get the
// request-an-installation path, never a fake success).
func RegisterAppConnectorRoutes(r *gin.RouterGroup, appConnectorHandler *handler.AppConnectorHandler) {
	if appConnectorHandler == nil {
		return
	}
	appsGroup := r.Group("/apps", appConnectorHandler.RequireInstallCapabilityForWrites())
	{
		appsGroup.GET("/installations", appConnectorHandler.ListInstallations)
		appsGroup.POST("/installations", appConnectorHandler.CreateInstallation)
		appsGroup.POST("/installations/:id/upgrade", appConnectorHandler.UpgradeInstallation)
		appsGroup.POST("/installations/:id/disable", appConnectorHandler.DisableInstallation)
		appsGroup.GET("/connections", appConnectorHandler.ListConnections)
		// Connection creation starts the A02 provider OAuth flow; with no
		// provider configured the handler fails closed (501) instead of
		// minting a connection without credentials.
		appsGroup.POST("/connections", appConnectorHandler.CreateConnection)
		appsGroup.POST("/connections/:id/revoke", appConnectorHandler.RevokeConnection)
		// A07: sync status of a data source - binding projection plus the
		// live pause reason, with the data source looked up BY tenant.
		appsGroup.GET("/datasources/:id/sync-status", appConnectorHandler.GetSyncStatus)
		// W05: A03 action approval pipeline. GET reads the persisted
		// tenant-scoped snapshot directly; the writes (prepare/approve/
		// execute) fail closed (501) until SetActionService wires the A03
		// service — no approval or dispatch is ever fabricated here.
		appsGroup.POST("/actions/prepare", appConnectorHandler.PrepareAction)
		appsGroup.GET("/actions/:id", appConnectorHandler.GetAction)
		appsGroup.POST("/actions/:id/approve", appConnectorHandler.ApproveAction)
		appsGroup.POST("/actions/:id/execute", appConnectorHandler.ExecuteAction)
	}
}
