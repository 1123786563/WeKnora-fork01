package handler

import (
	"errors"
	"net/http"

	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/application/service/commercial"
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
	db      *gorm.DB
	refunds *commercialsvc.RefundService
	// orders is the P02 order pipeline; nil until the container wires it,
	// in which case order writes fail closed with 501.
	orders *commercialsvc.OrderService
}

// NewCommercialHandler builds the handler and makes sure the resource
// counter table exists (portable SQL, valid on both SQLite and
// PostgreSQL; PostgreSQL dialect evidence remains blocked-env).
func NewCommercialHandler(db *gorm.DB) *CommercialHandler {
	h := &CommercialHandler{db: db}
	if db != nil {
		// Refund wiring: the gateway/provider/eligibility arrive unconfigured
		// until the container connects them (blocked-env), so refund request
		// creation and review recording work while real channel payouts and
		// revocations surface explicit unconfigured errors.
		h.refunds, _ = commercialsvc.NewRefundService(db, nil, nil, nil)
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
	// Raw().Scan() into a struct returns a nil error and a zero-value row
	// when no subscription exists — gorm.ErrRecordNotFound never fires — so
	// the row count, not the error, decides the B05 base-tier fallback.
	res := h.db.Raw(`SELECT id, plan_key, plan_version, paid_until, version, downgrade_reason
		FROM commercial_subscriptions WHERE tenant_id = ? ORDER BY version DESC LIMIT 1`, tenantID).Scan(&row)
	switch {
	case res.Error == nil && res.RowsAffected > 0:
		c.JSON(http.StatusOK, gin.H{
			"tenant_id":          tenantID,
			"subscription":       row,
			"base_tier":          false,
			"can_manage_billing": commercial.CanManageBilling(role, true, h.hasBillingGrant(c, tenantID)),
		})
	case res.Error == nil:
		// No purchased subscription: the space is on the base tier (B05).
		c.JSON(http.StatusOK, gin.H{
			"tenant_id":          tenantID,
			"subscription":       nil,
			"base_tier":          true,
			"base_tier_key":      commercial.BaseTier.Key,
			"can_manage_billing": commercial.CanManageBilling(role, true, h.hasBillingGrant(c, tenantID)),
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error.Error()})
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

// SetOrderService wires the order pipeline (injection point for the
// container). Until it is called, POST /commercial/orders fails closed
// with 501 — the edge never fabricates a checkout.
func (h *CommercialHandler) SetOrderService(s *commercialsvc.OrderService) { h.orders = s }

// Orders lists the caller space orders.
func (h *CommercialHandler) Orders(c *gin.Context) {
	tenantID, _, ok := commercialTenantScope(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": ErrMissingTenantScope.Error()})
		return
	}
	if h.orders == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "order pipeline not configured"})
		return
	}
	list, err := h.orders.ListOrders(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// CreateQuote serves POST /commercial/quotes: cut an exact-price offer for
// the latest published version of one plan in the caller space.
func (h *CommercialHandler) CreateQuote(c *gin.Context) {
	tenantID, _, ok := commercialTenantScope(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": ErrMissingTenantScope.Error()})
		return
	}
	if h.orders == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "order pipeline not configured"})
		return
	}
	var req struct {
		PlanKey string `json:"plan_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "plan_key is required"})
		return
	}
	q, err := h.orders.CreateQuote(c.Request.Context(), tenantID, req.PlanKey)
	switch {
	case err == nil:
		c.JSON(http.StatusCreated, q)
	case errors.Is(err, repocommercial.ErrPlanNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "no published plan with this key"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

// CreateOrder serves POST /commercial/orders: consume a quote of the
// caller space and open one pending order with a channel checkout.
func (h *CommercialHandler) CreateOrder(c *gin.Context) {
	tenantID, _, ok := commercialTenantScope(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": ErrMissingTenantScope.Error()})
		return
	}
	if h.orders == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "order pipeline not configured"})
		return
	}
	var req struct {
		QuoteID  string `json:"quote_id"`
		Provider string `json:"provider"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.QuoteID == "" || req.Provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "quote_id and provider are required"})
		return
	}
	order, err := h.orders.CreateOrder(c.Request.Context(), tenantID, req.QuoteID, req.Provider)
	switch {
	case err == nil && order.CheckoutError != "":
		// The order is durably pending but the channel call failed: the
		// answer still carries the operation ID and state (product contract:
		// writes return an operation ID), and the client recovers through
		// GET /commercial/orders/:id instead of retrying the consumed quote.
		c.JSON(http.StatusAccepted, order)
	case err == nil:
		c.JSON(http.StatusCreated, order)
	case errors.Is(err, commercialsvc.ErrPaymentProviderUnconfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	case errors.Is(err, commercialsvc.ErrQuoteTenantMismatch):
		c.JSON(http.StatusNotFound, gin.H{"error": "quote not found for this tenant"})
	case errors.Is(err, repocommercial.ErrQuoteAlreadyUsed):
		c.JSON(http.StatusConflict, gin.H{"error": "quote already used"})
	case errors.Is(err, repocommercial.ErrQuoteExpired):
		c.JSON(http.StatusConflict, gin.H{"error": "quote expired"})
	case errors.Is(err, repocommercial.ErrQuoteVersionConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "subscription changed since the quote was cut"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

// GetOrder serves GET /commercial/orders/:id and performs payment status
// recovery for pending orders: the channel is re-queried by the ORIGINAL
// merchant order id and a succeeded fact confirms through the standard
// transaction. Cross-space IDs answer 404, never content.
func (h *CommercialHandler) GetOrder(c *gin.Context) {
	tenantID, _, ok := commercialTenantScope(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": ErrMissingTenantScope.Error()})
		return
	}
	if h.orders == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "order pipeline not configured"})
		return
	}
	order, err := h.orders.RecoverOrderStatus(c.Request.Context(), tenantID, c.Param("id"))
	switch {
	case err == nil:
		c.JSON(http.StatusOK, order)
	case errors.Is(err, repocommercial.ErrOrderNotFound) || errors.Is(err, commercialsvc.ErrOrderTenantMismatch):
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
	case errors.Is(err, commercialsvc.ErrPaymentProviderUnconfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

// ChangePlan serves POST /commercial/plans/change (Commerce.ChangePlan):
// quote_id names the TARGET plan quote; expected_subscription_version
// guards concurrent changes. A price increase settles as a prorated
// upgrade order (same recoverable checkout contract as CreateOrder);
// anything else is scheduled to take effect at the end of the paid
// period. A version conflict answers 409 with a re-quote signal.
func (h *CommercialHandler) ChangePlan(c *gin.Context) {
	tenantID, _, ok := commercialTenantScope(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": ErrMissingTenantScope.Error()})
		return
	}
	if h.orders == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "order pipeline not configured"})
		return
	}
	var req struct {
		QuoteID                     string `json:"quote_id"`
		ExpectedSubscriptionVersion *int64 `json:"expected_subscription_version"`
		Provider                    string `json:"provider"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.QuoteID == "" || req.ExpectedSubscriptionVersion == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "quote_id and expected_subscription_version are required"})
		return
	}
	view, err := h.orders.ChangePlan(c.Request.Context(), tenantID, req.QuoteID, *req.ExpectedSubscriptionVersion, req.Provider)
	switch {
	case err == nil:
		if view.Order != nil && view.Order.CheckoutError != "" {
			c.JSON(http.StatusAccepted, view)
			return
		}
		c.JSON(http.StatusCreated, view)
	case errors.Is(err, commercialsvc.ErrNoSubscriptionToChange):
		c.JSON(http.StatusConflict, gin.H{"error": "no subscription to change; purchase a plan through POST /commercial/orders first"})
	case errors.Is(err, repocommercial.ErrSubscriptionVersionConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "subscription changed since the quote was cut; cut a new quote and retry"})
	case errors.Is(err, commercialsvc.ErrQuoteTenantMismatch):
		c.JSON(http.StatusNotFound, gin.H{"error": "quote not found for this tenant"})
	case errors.Is(err, commercialsvc.ErrPaymentProviderUnconfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	case errors.Is(err, repocommercial.ErrQuoteAlreadyUsed):
		c.JSON(http.StatusConflict, gin.H{"error": "quote already used"})
	case errors.Is(err, repocommercial.ErrQuoteExpired):
		c.JSON(http.StatusConflict, gin.H{"error": "quote expired"})
	case errors.Is(err, repocommercial.ErrQuoteVersionConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "subscription changed since the quote was cut; cut a new quote and retry"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
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

// CreateRefund serves POST /commercial/refunds: a SPACE-SCOPED refund
// request. The tenant comes exclusively from the authenticated context
// (never a path parameter) and must own the order; the request only
// registers intent — locks, channel calls and revocation happen solely
// through the review/approval path, and until P03's occupancy-refundable
// check exists approvals keep refunds in requested/reviewing.
func (h *CommercialHandler) CreateRefund(c *gin.Context) {
	tenantID, _, ok := commercialTenantScope(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": ErrMissingTenantScope.Error()})
		return
	}
	if h.refunds == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "refund service unavailable"})
		return
	}
	var req struct {
		OrderID      string `json:"order_id"`
		OrderLineID  string `json:"order_line_id"`
		AmountFen    int64  `json:"amount_fen"`
		CreditsMicro int64  `json:"credits_micro"`
		Reason       string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.OrderID == "" || req.AmountFen <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order_id and a positive amount_fen are required"})
		return
	}
	state, err := h.refunds.CreateRequest(c.Request.Context(), tenantID, req.OrderID, req.OrderLineID,
		commercial.CNYFen(req.AmountFen), commercial.Credits(req.CreditsMicro))
	switch {
	case err == nil:
		c.JSON(http.StatusCreated, gin.H{
			"id": state.ID, "order_id": state.OrderID, "state": state.State,
			"amount_fen": int64(state.Amount), "credits_micro": int64(state.CreditAmount),
		})
	case errors.Is(err, commercialsvc.ErrRefundOrderMismatch):
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found for this tenant"})
	case errors.Is(err, commercialsvc.ErrInvalidRefundOrderState):
		c.JSON(http.StatusConflict, gin.H{"error": "order is not in a refundable state"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

// PlatformRefundReviewerCapability is the explicit grant (a row in
// commercial_grants with this capability, granted at platform scope
// tenant_id = 0) that admits a human reviewer to the refund review path.
// Review moves money out of a space, so it is a SEPARATE permission path
// from tenant billing AND from space administration: a space Admin who
// merely belongs to some tenant must never be able to drive a refund
// — including one belonging to another tenant — by knowing its ID.
const PlatformRefundReviewerCapability = "refund_review"

// hasPlatformRefundReviewer reports platform refund-review authority:
// either a platform API key (scope.IsPlatform) or a human user carrying
// the explicit refund_review grant at platform scope (tenant_id = 0).
// Any lookup failure fails closed (not a reviewer).
func (h *CommercialHandler) hasPlatformRefundReviewer(c *gin.Context) bool {
	if h == nil || h.db == nil {
		return false
	}
	if scope, ok := types.TenantAPIKeyScopeFromContext(c.Request.Context()); ok {
		return scope.IsPlatform()
	}
	userID := commercialUserID(c)
	if userID == "" {
		return false
	}
	var n int64
	if err := h.db.Raw(`SELECT COUNT(*) FROM commercial_grants
		WHERE tenant_id = 0 AND user_id = ? AND capability = ?`,
		userID, PlatformRefundReviewerCapability).Scan(&n).Error; err != nil {
		return false
	}
	return n > 0
}

// RequirePlatformRefundReviewer gates the platform refund review path.
// Space administration (the Admin role inside any tenant) NEVER admits a
// reviewer: the caller must be an authenticated platform operator — a
// platform API key or a user holding the explicit platform-scope
// refund_review grant. This closes the cross-space escalation where a
// space Admin who learned a global refund ID could approve another
// space's refund.
func (h *CommercialHandler) RequirePlatformRefundReviewer() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !h.hasPlatformRefundReviewer(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Forbidden: refund review requires platform operator authority",
			})
			return
		}
		c.Next()
	}
}

// AdminReviewRefund serves POST /admin/refunds/:id/review: the platform
// review decision on one refund. Approval records the manual basis
// (period not started vs already effective); without a configured
// eligibility policy (P03 pending) the refund honestly stays in review —
// the response says so instead of pretending a payout.
func (h *CommercialHandler) AdminReviewRefund(c *gin.Context) {
	if h.refunds == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "refund service unavailable"})
		return
	}
	var req struct {
		Action string `json:"action"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Action == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "action is required"})
		return
	}
	id := c.Param("id")
	reviewer := commercialUserID(c)
	if reviewer == "" {
		// A platform API key carries no human user ID; the audit trail
		// still needs a stable reviewer identity.
		reviewer = "platform_operator"
	}
	switch req.Action {
	case "approve":
		err := h.refunds.Approve(c.Request.Context(), id, reviewer)
		if errors.Is(err, commercial.ErrRefundNotReady) {
			c.JSON(http.StatusConflict, gin.H{
				"error": "refund kept in review: eligibility policy (P03 occupancy check) not configured",
				"id":    id, "state": commercial.RefundStateReviewing,
			})
			return
		}
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": id, "state": commercial.RefundStatePending})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported review action"})
	}
}
