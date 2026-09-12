package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterAppConnectorRoutes registers the app-connector endpoints under
// /api/v1/apps. Like the commercial routes, the brief-mandated signature
// carries no rbacGuards: these routes are intentionally NOT declared in the
// API-key route authorizer, so the /api/v1 gate default-denies every
// X-API-Key principal (403) - a machine key never auto-inherits app install
// or connection authority. Tenant scope always comes from the authenticated
// context, and each lifecycle handler declares its OWN write gate so
// installation, connection and action policies evolve independently:
//   - installations: RequireInstallCapabilityForWrites (owner/admin;
//     members get the request-an-installation path, never a fake success)
//   - connections:   RequireConnectionCapabilityForWrites
//   - actions:       RequireActionCapabilityForWrites
func RegisterAppConnectorRoutes(
	r *gin.RouterGroup,
	installationHandler *handler.AppInstallationHandler,
	connectionHandler *handler.AppConnectionHandler,
	syncHandler *handler.AppSyncHandler,
	actionHandler *handler.AppActionHandler,
) {
	if installationHandler != nil {
		installations := r.Group("/apps/installations", installationHandler.RequireInstallCapabilityForWrites())
		{
			installations.GET("", installationHandler.ListInstallations)
			installations.POST("", installationHandler.CreateInstallation)
			installations.POST("/:id/upgrade", installationHandler.UpgradeInstallation)
			installations.POST("/:id/disable", installationHandler.DisableInstallation)
		}
	}
	if connectionHandler != nil {
		connections := r.Group("/apps/connections", connectionHandler.RequireConnectionCapabilityForWrites())
		{
			connections.GET("", connectionHandler.ListConnections)
			// Connection creation starts the A02 provider OAuth flow; with no
			// provider configured the handler fails closed (501) instead of
			// minting a connection without credentials.
			connections.POST("", connectionHandler.CreateConnection)
			connections.POST("/:id/revoke", connectionHandler.RevokeConnection)
		}
		// A02: the provider redirect leg hangs DIRECTLY on the parent group
		// (public; no bearer) — the opaque single-use state parameter is the
		// only credential, mirroring the commercial provider callbacks.
		r.GET("/apps/connections/oauth/callback", connectionHandler.ConnectionOAuthCallback)
	}
	if syncHandler != nil {
		// A07: sync status of a data source - binding projection plus the
		// live pause reason, with the data source looked up BY tenant. A
		// read endpoint: no write gate applies.
		r.GET("/apps/datasources/:id/sync-status", syncHandler.GetSyncStatus)
	}
	if actionHandler != nil {
		// W05: A03 action approval pipeline. GET reads the persisted
		// tenant-scoped snapshot directly; the writes (prepare/approve/
		// execute) fail closed (501) until SetActionService wires the A03
		// service — no approval or dispatch is ever fabricated here.
		actions := r.Group("/apps/actions", actionHandler.RequireActionCapabilityForWrites())
		{
			actions.POST("/prepare", actionHandler.PrepareAction)
			actions.GET("/:id", actionHandler.GetAction)
			actions.POST("/:id/approve", actionHandler.ApproveAction)
			actions.POST("/:id/execute", actionHandler.ExecuteAction)
		}
	}
}
