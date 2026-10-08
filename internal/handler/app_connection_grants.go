package handler

// Space-connection grant management (T23, #53). CONTEXT.md 代码平台连接:
// 空间连接按仓库、成员角色和操作策略授权. The management predicate is
// appconnector.CanManageConnections (owner/admin) — the same vocabulary as
// connection management itself; the target must be a Kind='space'
// connection; the grantee must be an active same-tenant member. Identity
// always comes from the authenticated context (appTenantScope), never from
// the URL or body.

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/repository"
	appconnector "github.com/Tencent/WeKnora/internal/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/appconnector/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AppConnectionGrantHandler manages per-actor grants on space connections.
type AppConnectionGrantHandler struct {
	db      *gorm.DB
	grants  *appconnectorrepo.SpaceConnectionGrantStore
	members interfaces.TenantMemberRepository
}

// NewAppConnectionGrantHandler builds the handler over the shared db.
func NewAppConnectionGrantHandler(db *gorm.DB) *AppConnectionGrantHandler {
	return &AppConnectionGrantHandler{
		db:      db,
		grants:  appconnectorrepo.NewSpaceConnectionGrantStore(db),
		members: repository.NewTenantMemberRepository(db),
	}
}

// grantScope resolves identity + the management predicate shared by all
// three methods. A non-manager gets 403 before any row is read.
func (h *AppConnectionGrantHandler) grantScope(c *gin.Context) (uint64, string, bool) {
	tenantID, role, userID, ok := appTenantScope(c)
	if !ok {
		return 0, "", false
	}
	if !appconnector.CanManageConnections(role) {
		appFail(c, http.StatusForbidden, "SPACE_GRANT_FORBIDDEN",
			"managing space-connection grants requires the tenant owner or an admin")
		return 0, "", false
	}
	return tenantID, userID, true
}

// loadSpaceConnection loads the tenant-scoped connection and refuses
// anything that is not a space connection. A cross-tenant id is one uniform
// 404.
func (h *AppConnectionGrantHandler) loadSpaceConnection(c *gin.Context, tenantID uint64, id string) bool {
	var row appconnectorrepo.ConnectionRow
	err := h.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		appFail(c, http.StatusNotFound, "CONNECTION_NOT_FOUND", "connection not found")
		return false
	}
	if err != nil {
		appFail(c, http.StatusInternalServerError, "CONNECTION_LOOKUP_FAILED", "connection lookup failed")
		return false
	}
	if row.Kind != appconnector.ConnectionKindSpace {
		// AC1: a personal connection is its owner's alone — grants do not
		// apply and would imply a sharing shape that must not exist.
		appFail(c, http.StatusBadRequest, "SPACE_GRANT_NOT_APPLICABLE",
			"only space connections take member grants")
		return false
	}
	return true
}

// requireActiveMember refuses grantees without an active same-tenant
// membership row.
func (h *AppConnectionGrantHandler) requireActiveMember(c *gin.Context, tenantID uint64, userID string) bool {
	member, err := h.members.Get(c.Request.Context(), userID, tenantID)
	if err != nil {
		appFail(c, http.StatusInternalServerError, "GRANTEE_LOOKUP_FAILED", "grantee membership lookup failed")
		return false
	}
	if member == nil || member.Status != types.TenantMemberStatusActive {
		appFail(c, http.StatusBadRequest, "GRANTEE_NOT_ACTIVE_MEMBER",
			"grantee must be an active member of this tenant")
		return false
	}
	return true
}

// Grant POST /apps/connections/:id/grants — body {"grantee_id": "..."}.
func (h *AppConnectionGrantHandler) Grant(c *gin.Context) {
	tenantID, userID, ok := h.grantScope(c)
	if !ok {
		return
	}
	if !h.loadSpaceConnection(c, tenantID, c.Param("id")) {
		return
	}
	var input struct {
		GranteeID string `json:"grantee_id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.GranteeID) == "" {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "grantee_id is required")
		return
	}
	if !h.requireActiveMember(c, tenantID, input.GranteeID) {
		return
	}
	if err := h.grants.GrantSpaceConnection(c.Request.Context(), tenantID, c.Param("id"), input.GranteeID, userID); err != nil {
		appFail(c, http.StatusInternalServerError, "SPACE_GRANT_FAILED", "failed to record the grant")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{
		"connection_id": c.Param("id"), "grantee_id": input.GranteeID, "granted_by": userID,
	}})
}

// Revoke DELETE /apps/connections/:id/grants/:grantee_id — idempotent.
func (h *AppConnectionGrantHandler) Revoke(c *gin.Context) {
	tenantID, _, ok := h.grantScope(c)
	if !ok {
		return
	}
	if !h.loadSpaceConnection(c, tenantID, c.Param("id")) {
		return
	}
	if err := h.grants.RevokeSpaceConnection(c.Request.Context(), tenantID, c.Param("id"), c.Param("grantee_id")); err != nil {
		appFail(c, http.StatusInternalServerError, "SPACE_GRANT_REVOKE_FAILED", "failed to revoke the grant")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"connection_id": c.Param("id"), "grantee_id": c.Param("grantee_id"), "revoked": true,
	}})
}

// List GET /apps/connections/:id/grants — manager-only member list.
func (h *AppConnectionGrantHandler) List(c *gin.Context) {
	tenantID, _, ok := h.grantScope(c)
	if !ok {
		return
	}
	if !h.loadSpaceConnection(c, tenantID, c.Param("id")) {
		return
	}
	grants, err := h.grants.ListSpaceConnectionGrants(c.Request.Context(), tenantID, c.Param("id"))
	if err != nil {
		appFail(c, http.StatusInternalServerError, "SPACE_GRANT_LIST_FAILED", "failed to list grants")
		return
	}
	items := make([]gin.H, 0, len(grants))
	for _, g := range grants {
		items = append(items, gin.H{"grantee_id": g.ActorID, "granted_by": g.GrantedBy, "created_at": g.CreatedAt})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"connection_id": c.Param("id"), "grants": items}})
}
