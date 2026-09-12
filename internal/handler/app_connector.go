package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// The /api/v1/apps surface is served by FOUR single-lifecycle handlers
// (W04/A02/A03/A07): AppInstallationHandler, AppConnectionHandler,
// AppSyncHandler and AppActionHandler. Each owns its routes, its write
// gate and its stores, so installation, connection, sync and action
// policies evolve independently instead of accumulating in one type.
//
// Shared invariants for every handler here: tenant scope is ALWAYS derived
// from the authenticated context (never a path parameter); every id is
// looked up BY tenant so a cross-tenant id is indistinguishable from a
// missing one (404). Connection responses are explicit structs —
// credential material (access/refresh tokens, credential_ref) has no field
// on any view and is therefore structurally unable to leak into a
// response.

func appOK(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"success": true, "data": data})
}

func appFail(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"success": false, "error": gin.H{"code": code, "message": message}})
}

func newAppID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return prefix + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return prefix + hex.EncodeToString(buf)
}

// appTenantScope reads tenant, role and user exclusively from the
// authenticated context. ok=false means the request is rejected already.
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
