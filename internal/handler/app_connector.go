package handler

import (
	"net/http"

	appconnectorhandler "github.com/Tencent/WeKnora/internal/appconnector/handler"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Pass B (27-appconnector) transitional shim: the implementation moved to
// internal/appconnector/handler. Consumers kept compiling with zero
// assembly changes: container.go:933-936/:980-999 (dig Provide + Invoke),
// router.go:131-134/:413-417, routes_app_connectors.go:22-25 and
// open_connector_test.go:386-389 (type aliases + constructor forwarding);
// commercial_task_budget.go (12-commercial) and craft_model_gateway.go
// (41-craft) (helper copies). Shim deletion obligation is registered in
// docs/architecture/passb/briefs/b2-appconnector.md (deleted by IB2 after
// the assembly switch).

type (
	AppInstallationHandler = appconnectorhandler.AppInstallationHandler
	AppConnectionHandler   = appconnectorhandler.AppConnectionHandler
	AppSyncHandler         = appconnectorhandler.AppSyncHandler
	AppActionHandler       = appconnectorhandler.AppActionHandler
	AppOAuthProviderConfig = appconnectorhandler.AppOAuthProviderConfig
)

func NewAppInstallationHandler(db *gorm.DB) *AppInstallationHandler {
	return appconnectorhandler.NewAppInstallationHandler(db)
}
func NewAppConnectionHandler(db *gorm.DB) *AppConnectionHandler {
	return appconnectorhandler.NewAppConnectionHandler(db)
}
func NewAppSyncHandler(db *gorm.DB) *AppSyncHandler {
	return appconnectorhandler.NewAppSyncHandler(db)
}
func NewAppActionHandler(db *gorm.DB) *AppActionHandler {
	return appconnectorhandler.NewAppActionHandler(db)
}

func DefaultAppOAuthProviderConfigs() map[string]AppOAuthProviderConfig {
	return appconnectorhandler.DefaultAppOAuthProviderConfigs()
}

// ---- Host helper copies (conventions §7.1 multi-owner duplicate helper
// family; IB2 consolidation ruling pending) ----
// Verbatim-equivalent to the module originals
// (…/handler/app_connector.go:28-55); consumers: commercial_task_budget.go:27+
// (appFail), craft_model_gateway.go:270-388 (appOK/appFail/appTenantScope).
// ErrMissingTenantScope keeps using the host original (commercial.go:34); no
// new host copy is added.
func appOK(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"success": true, "data": data})
}
func appFail(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"success": false, "error": gin.H{"code": code, "message": message}})
}
func appTenantScope(c *gin.Context) (uint64, string, string, bool) {
	tenantID, ok := types.TenantIDFromContext(c.Request.Context())
	if !ok || tenantID == 0 {
		appFail(c, http.StatusForbidden, "MISSING_TENANT_SCOPE", ErrMissingTenantScope.Error())
		return 0, "", "", false
	}
	role := string(types.TenantRoleFromContext(c.Request.Context()))
	userID, _ := types.UserIDFromContext(c.Request.Context())
	return tenantID, role, userID, true
}

// appRequireWriteCapability gates every non-read method with the given
// role capability; memberCode/memberMsg shape the fail-closed response.
// UI hiding is not authorization - this server gate is authoritative.
func appRequireWriteCapability(allowed func(role string) bool, memberCode, memberMsg string) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		tenantID, ok := types.TenantIDFromContext(c.Request.Context())
		if !ok || tenantID == 0 {
			appFail(c, http.StatusForbidden, "MISSING_TENANT_SCOPE", ErrMissingTenantScope.Error())
			c.Abort()
			return
		}
		role := string(types.TenantRoleFromContext(c.Request.Context()))
		if !allowed(role) {
			appFail(c, http.StatusForbidden, memberCode, memberMsg)
			c.Abort()
			return
		}
		c.Next()
	}
}
