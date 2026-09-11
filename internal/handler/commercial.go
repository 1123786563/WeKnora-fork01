package handler

import (
	"errors"
	"net/http"

	"github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CommercialAPIKeyCapability is the explicit additive capability an API
// key must carry to reach commercial endpoints. Full access deliberately
// does NOT imply it: purchase authority is never auto-derived from a
// key's blanket tenant access — only an explicit commercial grant on the
// key admits a machine principal.
const CommercialAPIKeyCapability types.APIKeyCapability = types.APIKeyCapabilityCommercial

var (
	// ErrQuotaExceeded is returned when the conditional counter update
	// (CAS) refuses a delta because it would break the hard limit or the
	// zero floor.
	ErrQuotaExceeded = errors.New("quota_exceeded")
	// ErrMissingTenantScope is returned when no authenticated tenant
	// scope is present; commercial routes never accept a tenant from the
	// URL, so there is nothing to fall back to.
	ErrMissingTenantScope = errors.New("missing_tenant_scope")
)

// ResourceWriteFunc performs the actual resource write inside the SAME
// transaction as the quota CAS. Returning an error rolls the whole
// transaction back, so a failed write never consumes quota and a
// quota-rejected write never lands.
type ResourceWriteFunc func(tx *gorm.DB) error

// CommercialHandler serves the /api/v1/commercial endpoints. Queries and
// commands derive tenant, user and role exclusively from the
// authenticated context attached by the Auth middleware — never from a
// path parameter.
type CommercialHandler struct {
	db *gorm.DB
}

// NewCommercialHandler builds the handler and makes sure the resource
// counter table exists (portable SQL, valid on both SQLite and
// PostgreSQL; PostgreSQL dialect evidence remains blocked-env).
func NewCommercialHandler(db *gorm.DB) *CommercialHandler {
	h := &CommercialHandler{db: db}
	if db != nil {
		_ = db.Exec(`CREATE TABLE IF NOT EXISTS commercial_resource_counters (
			tenant_id BIGINT NOT NULL,
			resource TEXT NOT NULL,
			used BIGINT NOT NULL DEFAULT 0,
			hard_limit BIGINT NULL,
			PRIMARY KEY (tenant_id, resource)
		)`).Error
	}
	return h
}

// commercialTenantScope reads the server-side scope. ok=false means no
// authenticated tenant: commercial routes reject the request rather than
// guessing a tenant.
func commercialTenantScope(c *gin.Context) (uint64, string, bool) {
	tenantID, ok := types.TenantIDFromContext(c.Request.Context())
	if !ok || tenantID == 0 {
		return 0, "", false
	}
	role := string(types.TenantRoleFromContext(c.Request.Context()))
	return tenantID, role, true
}

func commercialUserID(c *gin.Context) string {
	userID, ok := types.UserIDFromContext(c.Request.Context())
	if !ok {
		return ""
	}
	return userID
}

// hasBillingGrant reports an explicit billing grant in
// commercial_grants. Any lookup failure fails closed (no grant).
func (h *CommercialHandler) hasBillingGrant(c *gin.Context, tenantID uint64) bool {
	if h == nil || h.db == nil {
		return false
	}
	userID := commercialUserID(c)
	if userID == "" {
		return false
	}
	var n int64
	if err := h.db.Raw(`SELECT COUNT(*) FROM commercial_grants
		WHERE tenant_id = ? AND user_id = ? AND capability IN ('billing', 'manage_billing')`,
		tenantID, userID).Scan(&n).Error; err != nil {
		return false
	}
	return n > 0
}

// RequireExplicitCommercialCapability rejects every API-key principal
// that does not carry the explicit commercial capability — including
// full-access keys. JWT sessions pass through. In production the
// /api/v1 API-key gate default-denies these routes as well (they are
// deliberately left undeclared in the route authorizer); this guard is
// the second, capability-based layer that survives a future policy
// declaration.
func (h *CommercialHandler) RequireExplicitCommercialCapability() gin.HandlerFunc {
	return func(c *gin.Context) {
		if scope, ok := types.TenantAPIKeyScopeFromContext(c.Request.Context()); ok {
			if !scope.HasCapability(CommercialAPIKeyCapability) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "Forbidden: commercial endpoints require an explicit commercial capability",
				})
				return
			}
		}
		c.Next()
	}
}

// RequireManageBillingForWrites gates every non-read method on the
// commercial group with commercial.CanManageBilling: administrative role
// membership alone never grants purchase authority; owners and members
// with an explicit billing grant are the only callers admitted. Active
// membership is established by the Auth middleware before this runs.
func (h *CommercialHandler) RequireManageBillingForWrites() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		tenantID, role, ok := commercialTenantScope(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": ErrMissingTenantScope.Error()})
			return
		}
		if !commercial.CanManageBilling(role, true, h.hasBillingGrant(c, tenantID)) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Forbidden: billing management requires owner role or an explicit billing grant",
			})
			return
		}
		c.Next()
	}
}

type commercialSubscriptionRow struct {
	ID              string `json:"id"`
	PlanKey         string `json:"plan_key"`
	PlanVersion     int64  `json:"plan_version"`
	PaidUntil       string `json:"paid_until"`
	Version         int64  `json:"version"`
	DowngradeReason string `json:"downgrade_reason"`
}

// Summary returns the caller space commercial projection: the purchased
// subscription or the base tier, plus whether this caller may manage
// billing.
func (h *CommercialHandler) Summary(c *gin.Context) {
	tenantID, role, ok := commercialTenantScope(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": ErrMissingTenantScope.Error()})
		return
	}
	var row commercialSubscriptionRow
	err := h.db.Raw(`SELECT id, plan_key, plan_version, paid_until, version, downgrade_reason
		FROM commercial_subscriptions WHERE tenant_id = ? ORDER BY version DESC LIMIT 1`, tenantID).Scan(&row).Error
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{
			"tenant_id":          tenantID,
			"subscription":       row,
			"base_tier":          false,
			"can_manage_billing": commercial.CanManageBilling(role, true, h.hasBillingGrant(c, tenantID)),
		})
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusOK, gin.H{
			"tenant_id":          tenantID,
			"subscription":       nil,
			"base_tier":          true,
			"base_tier_key":      commercial.BaseTier.Key,
			"can_manage_billing": commercial.CanManageBilling(role, true, h.hasBillingGrant(c, tenantID)),
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

// Plans lists published catalog rows only; drafts and archived
// definitions are never offered.
func (h *CommercialHandler) Plans(c *gin.Context) {
	_, _, ok := commercialTenantScope(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": ErrMissingTenantScope.Error()})
		return
	}
	type planRow struct {
		PlanKey        string `json:"plan_key"`
		Version        int64  `json:"version"`
		DefinitionJSON string `json:"definition"`
	}
	plans := make([]planRow, 0)
	if err := h.db.Raw(`SELECT plan_key, version, definition_json FROM commercial_plan_catalog
		WHERE state = ? ORDER BY plan_key, version`, commercial.PlanStatePublished).Scan(&plans).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, plans)
}

// Usage returns the caller space resource counters. limit is null for an
// unlimited dimension and a real number (possibly zero) otherwise.
func (h *CommercialHandler) Usage(c *gin.Context) {
	tenantID, _, ok := commercialTenantScope(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": ErrMissingTenantScope.Error()})
		return
	}
	type usageRow struct {
		Resource string `json:"resource"`
		Used     int64  `json:"used"`
		Limit    *int64 `json:"limit"`
	}
	usage := make([]usageRow, 0)
	if err := h.db.Raw(`SELECT resource, used, hard_limit FROM commercial_resource_counters
		WHERE tenant_id = ? ORDER BY resource`, tenantID).Scan(&usage).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, usage)
}

// Orders lists the caller space orders. The order write path is owned by
// a later task; until it lands the honest answer is an empty array.
func (h *CommercialHandler) Orders(c *gin.Context) {
	_, _, ok := commercialTenantScope(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": ErrMissingTenantScope.Error()})
		return
	}
	c.JSON(http.StatusOK, []gin.H{})
}

// NotImplemented is the guarded placeholder write endpoint. The
// CanManageBilling gate in front of it is live (unauthorized callers get
// 403); authorised callers get an explicit 501 until the order task
// replaces this handler on the same group.
func (h *CommercialHandler) NotImplemented(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not_implemented"})
}

// ReserveResource atomically applies delta to the tenant resource
// counter. The hard limit and the zero floor are enforced by a
// conditional UPDATE (compare-and-set) on the counter row inside the
// SAME transaction as the caller's write (write runs only after the CAS
// succeeds), never by a pure commercial.CanIncrease check followed by a
// blind write — two concurrent adds that each fit individually cannot
// both land. The SQL is portable across SQLite and PostgreSQL (ON
// CONFLICT upsert + conditional UPDATE); PostgreSQL dialect evidence
// remains blocked-env.
func (h *CommercialHandler) ReserveResource(tenantID uint64, resource string, delta int64, write ResourceWriteFunc) error {
	if h == nil || h.db == nil {
		return errors.New("commercial handler has no database")
	}
	return h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`INSERT INTO commercial_resource_counters (tenant_id, resource, used, hard_limit)
			VALUES (?, ?, 0, NULL) ON CONFLICT (tenant_id, resource) DO NOTHING`, tenantID, resource).Error; err != nil {
			return err
		}
		res := tx.Exec(`UPDATE commercial_resource_counters SET used = used + ?
			WHERE tenant_id = ? AND resource = ? AND used + ? >= 0
			AND (hard_limit IS NULL OR used + ? <= hard_limit)`,
			delta, tenantID, resource, delta, delta)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrQuotaExceeded
		}
		if write != nil {
			return write(tx)
		}
		return nil
	})
}
