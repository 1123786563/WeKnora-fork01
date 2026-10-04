package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial"
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
	// platform is the Lago-era Commercial Platform seam (T05): the deep
	// provider-neutral port for commercial commands, snapshots and
	// reconciliation. nil until the container wires it — the readiness read
	// then fails closed honestly (unavailable/unconfigured), never fabricated.
	platform commercial.CommercialPlatform
	// billingAccounts is the T06 (#78) billing account service: the lazy
	// ensure-on-first-billing-access flow behind GET /commercial/account.
	// nil until the container wires it — the account read then answers
	// honestly pending/unconfigured; the endpoint never 500s for a wiring
	// gap.
	billingAccounts *commercialsvc.BillingAccountService
	// planVersions is the T07 (#79) plan-version lifecycle service; nil
	// until the container wires it — the admin plan endpoints then fail
	// closed with 503, never a fabricated success.
	planVersions *commercialsvc.PlanVersionService
	// benefits is the T08 (#80) lazy Base-Plan chain behind the benefits
	// section of GET /commercial/account; nil until the container wires it
	// — the account read then answers the #78 fail-closed envelope WITHOUT
	// a benefits object (documented, never 500 for a wiring gap).
	benefits *commercialsvc.BenefitsService
	// purchases is the W5 (#81) payment-gated purchase coordination; nil
	// until the container wires it — the purchase endpoints then fail
	// closed with 501 (the order-pipeline posture), never a fabricated
	// state.
	purchases *commercialsvc.PurchaseService
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
// billing. The answer carries the repo-wide {success:true,data:...}
// envelope — a bare object broke the api-client unwrap on every consumer.
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
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
			"tenant_id":          tenantID,
			"subscription":       row,
			"base_tier":          false,
			"can_manage_billing": commercial.CanManageBilling(role, true, h.hasBillingGrant(c, tenantID)),
		}})
	case res.Error == nil:
		// No purchased subscription: the space is on the base tier (B05).
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
			"tenant_id":          tenantID,
			"subscription":       nil,
			"base_tier":          true,
			"base_tier_key":      commercial.BaseTier.Key,
			"can_manage_billing": commercial.CanManageBilling(role, true, h.hasBillingGrant(c, tenantID)),
		}})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error.Error()})
	}
}

// quoteWire projects the service quote onto the wire contract the web
// parsers expect (packages/contracts parseQuoteView): fen amounts are DIGIT
// STRINGS, and the granted credits appear under the contract name
// credit_delta alongside the backend's numeric credits_micro. The backend
// fields stay in the same object (information superset; unknown fields pass
// through the parsers by convention).
func quoteWire(q commercialsvc.QuoteView) gin.H {
	// #81 AC1 additive freeze: the currency, the frozen entitlements and the
	// line items ride along when present (legacy quotes answer without
	// them, JSON-omitted).
	wire := gin.H{
		"id":            q.ID,
		"plan_key":      q.PlanKey,
		"plan_version":  q.PlanVersion,
		"amount_fen":    strconv.FormatInt(q.AmountFen, 10),
		"credit_delta":  strconv.FormatInt(q.CreditsMicro, 10),
		"credits_micro": q.CreditsMicro,
		"expires_at":    q.ExpiresAt,
	}
	if q.Currency != "" {
		wire["currency"] = q.Currency
	}
	if len(q.Features) > 0 {
		wire["features"] = q.Features
	}
	if len(q.LineItems) > 0 {
		lines := make([]gin.H, 0, len(q.LineItems))
		for _, li := range q.LineItems {
			lines = append(lines, gin.H{
				"kind":       li.Kind,
				"name":       li.Name,
				"amount_fen": strconv.FormatInt(li.AmountFen, 10),
			})
		}
		wire["line_items"] = lines
	}
	return wire
}

// orderWire projects the service order onto the wire contract the web
// parsers expect (packages/contracts parseOrderView): a digit-string
// amount_fen plus the payment/fulfillment axes the checkout page polls.
// The backend keeps ONE lifecycle state (pending → paid → fulfilled); the
// projection onto the two axes is mechanical — pending waits for payment,
// paid has settled with fulfillment still processing, fulfilled has the
// benefits live. (#84 / R4 dispatch table) An unresolved payment anomaly
// (PaymentAttention) maps onto fulfillment=attention ONLY for pending
// reads: paid→processing and fulfilled→fulfilled are fulfillment FACTS a
// multiple-success anomaly never rewrites (the anomaly surface is the admin
// list plus the payment_attention add-on). Contract invariant:
// fulfillment==="attention" only ever pairs with payment==="pending".
func orderWire(o commercialsvc.OrderView) gin.H {
	payment, fulfillment := "pending", "pending"
	switch o.State {
	case commercial.OrderStatePaid:
		payment, fulfillment = "paid", "processing"
	case commercial.OrderStateFulfilled:
		payment, fulfillment = "paid", "fulfilled"
	}
	if o.State == commercial.OrderStatePending && o.PaymentAttention {
		fulfillment = "attention"
	}
	w := gin.H{
		"id":             o.ID,
		"quote_id":       o.QuoteID,
		"state":          o.State,
		"amount_fen":     strconv.FormatInt(o.AmountFen, 10),
		"currency":       o.Currency,
		"payment":        payment,
		"fulfillment":    fulfillment,
		"provider":       o.Provider,
		"checkout_url":   o.CheckoutURL,
		"checkout_error": o.CheckoutError,
		"version":        o.Version,
	}
	// (#84) the attention add-on rides along on every state (a fulfilled
	// order with a pending over-payment disposition still carries it) —
	// BillingPage appends its notice from this flag.
	if o.PaymentAttention {
		w["payment_attention"] = true
	}
	// (R2-27) the closed degradation marker rides along when set — the
	// raw persistence error stays in the server log, never on the wire.
	if o.CheckoutLinkDegraded {
		w["checkout_link_degraded"] = true
	}
	return w
}

// Plans lists published catalog rows only; drafts and archived
// definitions are never offered. The answer carries the repo-wide
// {success:true,data:[...]} envelope; an empty catalog serialises data as
// [] (never null — SP11 lesson).
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
	c.JSON(http.StatusOK, gin.H{"success": true, "data": plans})
}

// Usage returns the caller space resource counters. limit is null for an
// unlimited dimension and a real number (possibly zero) otherwise. The
// answer carries the repo-wide {success:true,data:[...]} envelope; an empty
// counter table serialises data as [] (never null — SP11 lesson).
func (h *CommercialHandler) Usage(c *gin.Context) {
	tenantID, _, ok := commercialTenantScope(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": ErrMissingTenantScope.Error()})
		return
	}
	type usageRow struct {
		Resource string `json:"resource"`
		Used     int64  `json:"used"`
		// gorm maps fields to snake_case column names by default, so the
		// hard_limit column needs the explicit tag — without it every row
		// silently reported limit:null (all dimensions "unlimited").
		Limit *int64 `json:"limit" gorm:"column:hard_limit"`
	}
	usage := make([]usageRow, 0)
	if err := h.db.Raw(`SELECT resource, used, hard_limit FROM commercial_resource_counters
		WHERE tenant_id = ? ORDER BY resource`, tenantID).Scan(&usage).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": usage})
}

// SetOrderService wires the order pipeline (injection point for the
// container). Until it is called, POST /commercial/orders fails closed
// with 501 — the edge never fabricates a checkout.
func (h *CommercialHandler) SetOrderService(s *commercialsvc.OrderService) { h.orders = s }

// SetCommercialPlatform wires the Commercial Platform seam (injection point
// for the container, T05). Until it is called, the readiness read answers
// honestly unavailable/unconfigured — it never fabricates platform state.
func (h *CommercialHandler) SetCommercialPlatform(p commercial.CommercialPlatform) { h.platform = p }

// SetBillingAccountService wires the billing account service (injection
// point for the container, T06/#78). Until it is called, the account read
// answers honestly pending/unconfigured — the endpoint never 500s for a
// wiring gap.
func (h *CommercialHandler) SetBillingAccountService(s *commercialsvc.BillingAccountService) {
	h.billingAccounts = s
}

// SetBenefitsService wires the T08 benefits chain (injection point for the
// container, #80). Until it is called, GET /commercial/account answers the
// #78 envelope without a benefits object — never a fabricated plan.
func (h *CommercialHandler) SetBenefitsService(s *commercialsvc.BenefitsService) {
	h.benefits = s
}

// SetPurchaseService wires the W5 purchase chain (injection point for the
// container, #81). Until it is called, the purchase endpoints fail closed
// with 501 — the edge never fabricates a purchase.
func (h *CommercialHandler) SetPurchaseService(s *commercialsvc.PurchaseService) {
	h.purchases = s
}

// purchaseWire projects the service purchase onto the wire contract the web
// parsers expect (packages/contracts parsePurchaseView): the CLOSED state
// token, a digit-string amount_fen and the order object when one exists.
// No Lago vocabulary, no external identity, no raw status ever crosses.
func purchaseWire(p commercialsvc.PurchaseView) gin.H {
	wire := gin.H{"state": p.State}
	if p.Order != nil {
		wire["order"] = orderWire(*p.Order)
	}
	if p.PlanKey != "" {
		wire["plan_key"] = p.PlanKey
	}
	if p.PlanVersion != 0 {
		wire["plan_version"] = p.PlanVersion
	}
	if p.AmountFen != 0 {
		wire["amount_fen"] = strconv.FormatInt(p.AmountFen, 10)
	}
	if p.Currency != "" {
		wire["currency"] = p.Currency
	}
	if p.Reason != "" {
		wire["reason"] = p.Reason
	}
	return wire
}

// Purchase serves POST /commercial/purchases (#81): submit a frozen quote
// into the payment-gated purchase. The match gate runs BEFORE any channel
// payment request: a mismatch answers 409 invoice_quote_mismatch with zero
// channel calls.
func (h *CommercialHandler) Purchase(c *gin.Context) {
	tenantID, _, ok := commercialTenantScope(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": ErrMissingTenantScope.Error()})
		return
	}
	if h.purchases == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "purchase pipeline not configured"})
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
	// The actor is the real authenticated caller (R1-V17): the synthetic
	// "billing-admin" made purchase and ensure audit trails unattributable.
	// The ensure display name comes from the tenant record like the other
	// production call sites.
	actor := commercialUserID(c)
	view, err := h.purchases.Purchase(c.Request.Context(), tenantID, req.QuoteID, req.Provider,
		actor, h.tenantDisplayName(tenantID))
	switch {
	case err == nil && view.Order != nil && view.Order.CheckoutError != "":
		// (R1-V14) The order is durably pending but the channel call failed:
		// mirror POST /orders and answer 202 — the client recovers through
		// GET /commercial/orders/:id instead of reading a clean creation.
		// (R2-27) CheckoutError now means CHANNEL failure only: a checkout
		// whose link persistence degraded (CheckoutLinkDegraded) answers the
		// clean 201 below — the channel call succeeded and the client holds
		// a working link.
		c.JSON(http.StatusAccepted, gin.H{"success": true, "data": purchaseWire(view)})
	case err == nil:
		c.JSON(http.StatusCreated, gin.H{"success": true, "data": purchaseWire(view)})
	case errors.Is(err, commercialsvc.ErrInvoiceQuoteMismatch):
		c.JSON(http.StatusConflict, gin.H{"error": "invoice_quote_mismatch"})
	case errors.Is(err, repocommercial.ErrQuoteExpired):
		c.JSON(http.StatusConflict, gin.H{"error": "quote expired"})
	case errors.Is(err, repocommercial.ErrQuoteVersionConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "subscription changed since the quote was cut; please re-quote"})
	case errors.Is(err, commercial.ErrWorkspaceClosed):
		// (#102 / Lago 30) The workspace is closed: no new commercial work,
		// ever — a definitive conflict, never a retryable one.
		c.JSON(http.StatusConflict, gin.H{"error": commercial.ErrWorkspaceClosed.Error()})
	case errors.Is(err, commercialsvc.ErrPurchasePlanConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "purchase_plan_conflict"})
	case errors.Is(err, commercialsvc.ErrPurchaseNotAwaiting):
		// (R1-V03) The held purchase is active/canceled — no new channel
		// order may be opened for it.
		c.JSON(http.StatusConflict, gin.H{"error": commercialsvc.ErrPurchaseNotAwaiting.Error()})
	case errors.Is(err, commercialsvc.ErrQuoteLegacySnapshot):
		c.JSON(http.StatusConflict, gin.H{"error": "quote predates the purchase freeze; please re-quote"})
	case errors.Is(err, commercialsvc.ErrPurchasePlanInvalid):
		// (R1-V18) Corrupt plan definition: a server-side data problem,
		// never a re-quote signal.
		c.JSON(http.StatusInternalServerError, gin.H{"error": "purchase plan definition is invalid"})
	case errors.Is(err, commercialsvc.ErrPurchasePlanCharges):
		c.JSON(http.StatusConflict, gin.H{"error": "this plan version is not purchasable yet; please re-quote later"})
	case errors.Is(err, commercialsvc.ErrPurchaseUnavailable):
		// (R1-V04) Platform failure: 503 with the CLOSED reason token —
		// never a fabricated success and never raw error text.
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":  "purchase temporarily unavailable",
			"reason": commercialsvc.PurchaseUnavailableReason(err),
		})
	case errors.Is(err, commercialsvc.ErrPaymentProviderUnconfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	case errors.Is(err, commercialsvc.ErrPaymentObservationUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "payment status temporarily unavailable", "reason": "payment_status_unavailable"})
	case errors.Is(err, commercialsvc.ErrQuoteTenantMismatch):
		c.JSON(http.StatusNotFound, gin.H{"error": "quote not found for this tenant"})
	case errors.Is(err, repocommercial.ErrQuoteNotFound):
		// (OCR r4 / review R82-2) A nonexistent/expired quote id is a
		// CLIENT fact: the tenant-guarded quote read miss answers 404 —
		// never the residual 500 (which invites retrying a deterministic
		// failure).
		c.JSON(http.StatusNotFound, gin.H{"error": "quote not found"})
	case errors.Is(err, repocommercial.ErrPlanNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "no published plan version for this quote"})
	case errors.Is(err, repocommercial.ErrQuoteAlreadyUsed):
		// (R1-V21) Same race outcome as POST /orders: 409, not a generic 400.
		c.JSON(http.StatusConflict, gin.H{"error": "quote already used"})
	case errors.Is(err, repocommercial.ErrPurchasePendingExists):
		// (D12 / r2:306) A pending-exists race the service could not replay
		// (the winner is still inside its channel-Create window, link-less
		// and too fresh for the sweep): a RETRYABLE conflict, never the
		// default branch's 500 — whose "server-side, never invites retry"
		// semantics is exactly backwards for a race the caller wins by
		// re-reading a moment later.
		c.JSON(http.StatusConflict, gin.H{"error": "purchase pending exists"})
	default:
		// (R1-09) The residual error face is server-side (gorm/storage
		// failures from EnsureBillingAccount/GetPublication/GetVersion/
		// CreateOrder, snapshot JSON corruption): 500 with a CLOSED
		// message — never a 400 (which invites the caller to retry the
		// same request as if it were a client fault) and never raw error
		// text across the public boundary (same posture as the 503
		// branch's "never raw error text"). The original error stays in
		// the server log.
		log.Printf("commercial: purchase failed for tenant %d: %v", tenantID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "purchase failed"})
	}
}

// PurchaseStatus serves GET /commercial/purchase (#81): the caller space's
// purchase projection in closed product vocabulary — absent when no
// purchase exists, never a fabricated state.
func (h *CommercialHandler) PurchaseStatus(c *gin.Context) {
	tenantID, _, ok := commercialTenantScope(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": ErrMissingTenantScope.Error()})
		return
	}
	if h.purchases == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "purchase pipeline not configured"})
		return
	}
	view, err := h.purchases.PurchaseStatus(c.Request.Context(), tenantID)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{"success": true, "data": purchaseWire(view)})
	case errors.Is(err, commercialsvc.ErrQuoteTenantMismatch):
		c.JSON(http.StatusNotFound, gin.H{"error": "purchase not found for this tenant"})
	default:
		// (OCR r4) A residual failure here is server-side (storage), never
		// a client fault: 500 with a closed message, never raw error text.
		log.Printf("commercial: purchase status failed for tenant %d: %v", tenantID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "purchase status failed"})
	}
}

// tenantDisplayName reads the space's display name for the ADVISORY
// metadata of the ensure command. A missing name (or a missing tenants
// table) degrades to a stable placeholder — never an error.
func (h *CommercialHandler) tenantDisplayName(tenantID uint64) string {
	if h == nil || h.db == nil {
		return fmt.Sprintf("WeKnora Space %d", tenantID)
	}
	var name string
	if err := h.db.Raw(`SELECT name FROM tenants WHERE id = ?`, tenantID).Scan(&name).Error; err != nil || name == "" {
		return fmt.Sprintf("WeKnora Space %d", tenantID)
	}
	return name
}

// AccountStatus serves GET /commercial/account: the caller space's billing
// account status (T06, #78) plus the T08 (#80) additive benefits section
// (plan/features/limits/credits — absent while pending). This GET is the
// documented LAZY ENSURE trigger (first billing access): an idempotent,
// authority-failure-safe run of the whole benefits chain, then the CLOSED
// product envelope — state ∈ {linked, pending}, reason ∈ {"",
// unconfigured, unreachable, invalid_response, unsupported} (empty iff
// linked), ensured_at RFC3339 (omitted when never ensured). No Lago URL,
// path, external id, provider reference or err.Error() text ever crosses;
// the tenant comes exclusively from the authenticated context.
func (h *CommercialHandler) AccountStatus(c *gin.Context) {
	tenantID, _, ok := commercialTenantScope(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": ErrMissingTenantScope.Error()})
		return
	}
	if h.benefits == nil && h.billingAccounts == nil {
		// Fail closed, honestly: no service wired means no account answer.
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
			"state":  "pending",
			"reason": "unconfigured",
		}})
		return
	}
	var status commercialsvc.BenefitsStatus
	var err error
	if h.benefits != nil {
		// T08: the whole lazy chain (seed → account → subscription → grant →
		// projection) runs behind this GET; the #78 envelope rides inside.
		status, err = h.benefits.EnsureBenefits(
			c.Request.Context(), tenantID, h.tenantDisplayName(tenantID), commercialUserID(c))
	} else {
		account, accErr := h.billingAccounts.EnsureBillingAccount(
			c.Request.Context(), tenantID, h.tenantDisplayName(tenantID), commercialUserID(c))
		if accErr != nil {
			err = accErr
		} else {
			status = commercialsvc.BenefitsStatus{Account: account}
			if account.State != commercialsvc.BillingAccountLinked {
				status.Reason = account.Reason
			}
		}
	}
	if err != nil {
		// A database failure is the only error class: generic 500 text, no
		// provider vocabulary ever rides along.
		c.JSON(http.StatusInternalServerError, gin.H{"error": "account status unavailable"})
		return
	}
	data := gin.H{
		"state":  status.Account.State,
		"reason": status.Reason,
	}
	if status.Account.EnsuredAt != nil {
		data["ensured_at"] = status.Account.EnsuredAt.UTC().Format(time.RFC3339)
	}
	if benefits := benefitsWire(status); benefits != nil {
		data["benefits"] = benefits
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// benefitsWire projects the closed benefits answer onto the wire: the plan
// (key/version/state), the feature map, the limits and the credits
// BREAKDOWN (#86 Task 4) — balance, held, refund-locked, available
// (= balance − held − refund_locked, negative honestly when over-committed),
// the projection instant and the per-batch face (source monthly|topup,
// period, grant instant, expiry). Amounts are digit strings — the
// wire-amount convention. nil while the chain is pending: never a
// fabricated plan.
func benefitsWire(status commercialsvc.BenefitsStatus) gin.H {
	if status.Plan == nil {
		return nil
	}
	balance, held, refundLocked := int64(0), int64(0), int64(0)
	projectedAt := ""
	var batches []gin.H
	if status.Credits != nil {
		balance = status.Credits.BalanceMicro
		held = status.Credits.HeldMicro
		refundLocked = status.Credits.RefundLockedMicro
		if !status.Credits.ProjectedAt.IsZero() {
			projectedAt = status.Credits.ProjectedAt.UTC().Format(time.RFC3339)
		}
		batches = make([]gin.H, 0, len(status.Credits.Batches))
		for _, b := range status.Credits.Batches {
			batch := gin.H{
				"source":        b.Source,
				"period":        b.Period,
				"balance_micro": strconv.FormatInt(b.BalanceMicro, 10),
				"expires_at":    b.ExpiresAt.UTC().Format(time.RFC3339),
			}
			if !b.GrantedAt.IsZero() {
				batch["granted_at"] = b.GrantedAt.UTC().Format(time.RFC3339)
			}
			batches = append(batches, batch)
		}
	}
	credits := gin.H{
		"balance_micro":       strconv.FormatInt(balance, 10),
		"held_micro":          strconv.FormatInt(held, 10),
		"refund_locked_micro": strconv.FormatInt(refundLocked, 10),
		"available_micro":     strconv.FormatInt(balance-held-refundLocked, 10),
		"batches":             batches,
	}
	if projectedAt != "" {
		credits["projected_at"] = projectedAt
	}
	return gin.H{
		"plan": gin.H{
			"key":     status.Plan.Key,
			"version": status.Plan.Version,
			"state":   status.Plan.State,
		},
		"features": status.Plan.Features,
		"limits":   status.Plan.Limits,
		"credits":  credits,
	}
}

// PlatformReadiness serves GET /commercial/platform/readiness: the one
// protected, read-only Billing API operation shipped through the frozen
// Commercial Platform seam (T05). The answer is the CLOSED product envelope
// only — state from the closed readiness enum, a closed reason token
// (unconfigured|unreachable|invalid_response|unsupported, empty when
// ready), the deployment-pinned release (omitted when unknown) and the
// check time. No provider URL, identifier, path, raw error text or response
// body ever crosses into the answer: adapter failures map onto the closed
// tokens and err.Error() is never echoed.
func (h *CommercialHandler) PlatformReadiness(c *gin.Context) {
	if h.platform == nil {
		// Fail closed, honestly: no seam wired means no authority answer.
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
			"state":  string(commercial.ReadinessUnavailable),
			"reason": "unconfigured",
		}})
		return
	}
	snap, err := h.platform.ReadSnapshot(c.Request.Context(), commercial.SnapshotQuery{
		Kind: commercial.SnapshotKindReadiness,
	})
	if err != nil {
		reason := "unsupported"
		switch {
		case errors.Is(err, commercial.ErrPlatformUnconfigured):
			reason = "unconfigured"
		case errors.Is(err, commercial.ErrPlatformUnreachable):
			reason = "unreachable"
		case errors.Is(err, commercial.ErrPlatformInvalidResponse):
			reason = "invalid_response"
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
			"state":  string(commercial.ReadinessUnavailable),
			"reason": reason,
		}})
		return
	}
	if snap.Readiness == nil {
		// A snapshot without its readiness section is not an answer; the
		// endpoint stays honest instead of projecting one.
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
			"state":  string(commercial.ReadinessUnavailable),
			"reason": "unsupported",
		}})
		return
	}
	data := gin.H{
		"state":      string(snap.Readiness.State),
		"checked_at": snap.Readiness.CheckedAt.UTC().Format(time.RFC3339),
		"reason":     snap.Readiness.Reason,
	}
	if snap.Readiness.Release != "" {
		data["release"] = snap.Readiness.Release
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// Orders lists the caller space orders inside the shared envelope; every
// element carries the same wire projection as GetOrder.
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
	// make(...) guards the SP11 nil-array lesson: an empty space answers
	// data:[], never data:null.
	data := make([]gin.H, 0, len(list))
	for _, o := range list {
		data = append(data, orderWire(o))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
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
		c.JSON(http.StatusCreated, gin.H{"success": true, "data": quoteWire(q)})
	case errors.Is(err, repocommercial.ErrPlanNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "no published plan with this key"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

// CreateTopUpQuote serves POST /commercial/topup-quotes (#85 G-A): cut a
// one-shot credit top-up offer at the closed book rate
// (CreditsMicro = AmountFen × 100). Whole-CNY amounts only — anything
// else is a client fact and answers 400.
func (h *CommercialHandler) CreateTopUpQuote(c *gin.Context) {
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
		AmountFen int64 `json:"amount_fen"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.AmountFen <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount_fen is required"})
		return
	}
	q, err := h.orders.CreateTopUpQuote(c.Request.Context(), tenantID, req.AmountFen)
	switch {
	case err == nil:
		c.JSON(http.StatusCreated, gin.H{"success": true, "data": quoteWire(q)})
	case errors.Is(err, repocommercial.ErrInvalidQuoteRow):
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount_fen must be a positive whole-CNY fen amount"})
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
		c.JSON(http.StatusAccepted, gin.H{"success": true, "data": orderWire(order)})
	case err == nil:
		c.JSON(http.StatusCreated, gin.H{"success": true, "data": orderWire(order)})
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
	case errors.Is(err, repocommercial.ErrPurchasePendingExists):
		// (R3-25) The partial pending-purchase invariant rejected this
		// insert (the legacy POST /orders path has no PurchaseService-style
		// pre-check, so the sentinel escapes through OpenOrder): answer 409
		// and attach the EXISTING payable pending order so the client holds
		// the payment entry instead of a bare error token.
		// (A-21/F105, r2:818) The attached order must carry a plan-ownership
		// PROOF: its frozen quote must have bought the SAME plan as this
		// request's quote, else the replayed checkout_url settles ANOTHER
		// plan's quote while orderWire carries no plan field for the client
		// to tell them apart. Foreign plan (or an unreadable quote on either
		// side) answers the bare 409.
		if existing, rerr := h.orders.CurrentPayablePendingOrderView(c.Request.Context(), tenantID); rerr == nil {
			_, reqSnap, qerr := h.orders.QuoteSnapshotForTenant(c.Request.Context(), tenantID, req.QuoteID)
			_, exSnap, eerr := h.orders.QuoteSnapshotForTenant(c.Request.Context(), tenantID, existing.QuoteID)
			if qerr == nil && eerr == nil && reqSnap.PlanKey == exSnap.PlanKey {
				c.JSON(http.StatusConflict, gin.H{"error": "purchase pending exists",
					"order": orderWire(existing)})
				return
			}
		} else {
			// (r2:823) The bare-409 degrade keeps one diagnosable line: the
			// index proved a pending exists but the replay read failed
			// (swept mid-flight, storage fault).
			log.Printf("commercial: pending-purchase conflict read failed for tenant %d: %v", tenantID, rerr)
		}
		c.JSON(http.StatusConflict, gin.H{"error": "purchase pending exists"})
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
		c.JSON(http.StatusOK, gin.H{"success": true, "data": orderWire(order)})
	case errors.Is(err, repocommercial.ErrOrderNotFound) || errors.Is(err, commercialsvc.ErrOrderTenantMismatch):
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
	case errors.Is(err, commercialsvc.ErrPaymentProviderUnconfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	case errors.Is(err, commercialsvc.ErrPaymentObservationUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "payment status temporarily unavailable", "reason": "payment_status_unavailable"})
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
			c.JSON(http.StatusAccepted, gin.H{"success": true, "data": view})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"success": true, "data": view})
	case errors.Is(err, commercialsvc.ErrNoSubscriptionToChange):
		c.JSON(http.StatusConflict, gin.H{"error": "no subscription to change; purchase a plan through POST /commercial/orders first"})
	case errors.Is(err, repocommercial.ErrSubscriptionVersionConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "subscription changed since the quote was cut; cut a new quote and retry"})
	case errors.Is(err, repocommercial.ErrScheduledChangeExists):
		c.JSON(http.StatusConflict, gin.H{"error": "a scheduled plan change is already pending for this space"})
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
	// json.Number accepts BOTH the JSON number (server callers) and the
	// digit string the web api-client sends (RefundInput.amount_fen is a
	// string by contract): binding to int64 would 400 on the string form.
	var req struct {
		OrderID      string      `json:"order_id"`
		OrderLineID  string      `json:"order_line_id"`
		AmountFen    json.Number `json:"amount_fen"`
		CreditsMicro json.Number `json:"credits_micro"`
		Reason       string      `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order_id and a positive amount_fen are required"})
		return
	}
	amount, amountErr := req.AmountFen.Int64()
	if req.OrderID == "" || amountErr != nil || amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order_id and a positive amount_fen are required"})
		return
	}
	creditsMicro := int64(0)
	if req.CreditsMicro.String() != "" {
		if creditsMicro, amountErr = req.CreditsMicro.Int64(); amountErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "credits_micro must be an integer"})
			return
		}
	}
	// The web RefundInput carries no order_line_id, and the store requires
	// the fulfillment lot identity. The current pricing policy (TopUpOrderLines)
	// gives a purchase order exactly one line, "credits" (the same lot the
	// refund tests pin) — an omitted line id refunds that line.
	orderLineID := req.OrderLineID
	if orderLineID == "" {
		orderLineID = "credits"
	}
	state, err := h.refunds.CreateRequest(c.Request.Context(), tenantID, req.OrderID, orderLineID,
		commercial.CNYFen(amount), commercial.Credits(creditsMicro))
	switch {
	case err == nil:
		// amount_fen answers as a digit string on the same convention as
		// the quote/order wire (RefundView parses strings).
		c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{
			"id": state.ID, "order_id": state.OrderID, "state": state.State,
			"amount_fen":    strconv.FormatInt(int64(state.Amount), 10),
			"credits_micro": int64(state.CreditAmount),
		}})
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
		c.JSON(http.StatusOK, gin.H{"success": true,
			"data": gin.H{"id": id, "state": commercial.RefundStatePending}})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported review action"})
	}
}

// ---- #84: platform payment-anomaly disposition surface ----

// AdminListPaymentAnomalies serves GET /admin/payment-anomalies: every
// retained abnormal payment fact, newest first, in closed WeKnora vocabulary
// (spec L170) — the operator's disposition queue for mismatched, partial,
// wrong-currency and multiple-success payments (spec L127).
func (h *CommercialHandler) AdminListPaymentAnomalies(c *gin.Context) {
	store := repocommercial.NewOrderStore(h.db)
	rows, err := store.ListPaymentAnomalies(c.Request.Context())
	if err != nil {
		log.Printf("commercial: payment anomaly list failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "internal_error"})
		return
	}
	data := make([]gin.H, 0, len(rows)) // SP11: never null
	for _, r := range rows {
		item := gin.H{
			"id":                  r.ID,
			"order_id":            r.OrderID,
			"tenant_id":           r.TenantID,
			"provider":            r.Provider,
			"kind":                r.Kind,
			"expected_amount_fen": strconv.FormatInt(r.ExpectedAmountFen, 10),
			"actual_amount_fen":   strconv.FormatInt(r.ActualAmountFen, 10),
			"currency":            r.ExpectedCurrency,
			"state":               r.State,
			"version":             r.Version,
			"created_at":          r.CreatedAt.UTC().Format(time.RFC3339),
		}
		if r.ActualCurrency != r.ExpectedCurrency {
			// A currency mismatch carries BOTH closed codes — never a raw
			// provider currency token beyond the codes themselves.
			item["actual_currency"] = r.ActualCurrency
		}
		if r.ResolvedAt != nil {
			item["resolved_at"] = r.ResolvedAt.UTC().Format(time.RFC3339)
		}
		data = append(data, item)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// AdminResolvePaymentAnomaly serves POST /admin/payment-anomalies/:id/resolve:
// the operator's disposition decision. expected_version is the optimistic
// guard (409 anomaly changed since read); a missing row answers 404. The
// retained fund fact itself is never rewritten.
func (h *CommercialHandler) AdminResolvePaymentAnomaly(c *gin.Context) {
	var req struct {
		ExpectedVersion *int64 `json:"expected_version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ExpectedVersion == nil || *req.ExpectedVersion <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "expected_version is required"})
		return
	}
	store := repocommercial.NewOrderStore(h.db)
	row, err := store.ResolvePaymentAnomaly(c.Request.Context(), c.Param("id"), *req.ExpectedVersion)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
			"id":      row.ID,
			"state":   row.State,
			"version": row.Version,
		}})
	case errors.Is(err, repocommercial.ErrPaymentAnomalyNotFound):
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "payment_anomaly_not_found"})
	case errors.Is(err, repocommercial.ErrPaymentAnomalyVersionConflict):
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": "anomaly changed since read"})
	default:
		log.Printf("commercial: payment anomaly resolve failed for %s: %v", c.Param("id"), err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "internal_error"})
	}
}

// AdminListFulfillmentAttentions serves the sanitized platform-only queue.
func (h *CommercialHandler) AdminListFulfillmentAttentions(c *gin.Context) {
	var rows []commercialsvc.FulfillmentExceptionRow
	if h == nil || h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "fulfillment attention unavailable"})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		log.Printf("commercial: fulfillment attention list failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "internal_error"})
		return
	}
	data := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		data = append(data, gin.H{"id": row.ID, "order_id": row.OrderID, "tenant_id": row.TenantID,
			"kind": row.Kind, "reason": row.Reason, "state": row.State,
			"created_at": row.CreatedAt.UTC().Format(time.RFC3339), "updated_at": row.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// ---- T07 (#79): platform plan-version admin surface ----

// PlatformPlanPublisherCapability is the explicit grant (a row in
// commercial_grants with capability='plan_publish' at platform scope
// tenant_id = 0) that admits a human operator to the plan publish surface.
// Catalog operations are cross-space by nature — publishing a plan touches
// every tenant's catalog — so tenant authority (owner, Admin, billing
// grantee) must NEVER admit it: the exact RequirePlatformRefundReviewer
// precedent.
const PlatformPlanPublisherCapability = "plan_publish"

// hasPlatformPlanPublisher reports platform plan-publish authority: either
// a platform API key (scope.IsPlatform) or a human user carrying the
// explicit plan_publish grant at platform scope. Any lookup failure fails
// closed (not a publisher).
func (h *CommercialHandler) hasPlatformPlanPublisher(c *gin.Context) bool {
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
		userID, PlatformPlanPublisherCapability).Scan(&n).Error; err != nil {
		return false
	}
	return n > 0
}

// RequirePlatformPlanPublisher gates the plan admin surface. Space
// administration NEVER admits a publisher: the caller must be an
// authenticated platform operator — a platform API key or a user holding
// the platform-scope plan_publish grant.
func (h *CommercialHandler) RequirePlatformPlanPublisher() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !h.hasPlatformPlanPublisher(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Forbidden: plan publish requires platform operator authority",
			})
			return
		}
		c.Next()
	}
}

// planDraftRequest is the admin draft wire contract. Money arrives as the
// digit-string-or-number json.Number (the quoteWire precedent).
type planDraftRequest struct {
	PlanKey              string              `json:"plan_key"`
	Name                 string              `json:"name"`
	AmountFen            json.Number         `json:"amount_fen"`
	Currency             string              `json:"currency"`
	IncludedCreditsMicro json.Number         `json:"included_credits_micro"`
	Features             map[string]bool     `json:"features"`
	Limits               map[string]int64    `json:"limits"`
	Charges              []planChargeRequest `json:"charges"`
}

type planChargeRequest struct {
	Dimension    string      `json:"dimension"`
	Model        string      `json:"model"`
	AmountFen    json.Number `json:"amount_fen"`
	PackageUnits json.Number `json:"package_units"`
	FreeUnits    json.Number `json:"free_units"`
}

// toDraftInput converts the wire contract onto the service input.
func (r planDraftRequest) toDraftInput() (commercialsvc.DraftInput, error) {
	amount, err := r.AmountFen.Int64()
	if err != nil {
		return commercialsvc.DraftInput{}, err
	}
	credits, err := r.IncludedCreditsMicro.Int64()
	if err != nil {
		return commercialsvc.DraftInput{}, err
	}
	charges := make([]commercial.PlanCharge, 0, len(r.Charges))
	for _, c := range r.Charges {
		chargeAmount, err := c.AmountFen.Int64()
		if err != nil {
			return commercialsvc.DraftInput{}, err
		}
		var packageUnits, freeUnits int64
		if c.PackageUnits.String() != "" {
			if packageUnits, err = c.PackageUnits.Int64(); err != nil {
				return commercialsvc.DraftInput{}, err
			}
		}
		if c.FreeUnits.String() != "" {
			if freeUnits, err = c.FreeUnits.Int64(); err != nil {
				return commercialsvc.DraftInput{}, err
			}
		}
		charges = append(charges, commercial.PlanCharge{
			Dimension: c.Dimension, Model: c.Model, AmountFen: chargeAmount,
			PackageUnits: packageUnits, FreeUnits: freeUnits,
		})
	}
	return commercialsvc.DraftInput{
		PlanKey: r.PlanKey, Name: r.Name, AmountFen: amount,
		IncludedCreditsMicro: credits, Features: r.Features, Limits: r.Limits,
		Currency: r.Currency, Charges: charges,
	}, nil
}

// planVersionWire projects a version view onto the CLOSED admin response
// set: WeKnora product vocabulary only. The external plan code and every
// provider identifier are absent BY DESIGN — the mapping lives in
// commercial_plan_publications and is addressed by (plan_key, version).
func planVersionWire(view repocommercial.VersionView) (gin.H, error) {
	var definition commercial.PlanVersion
	if err := json.Unmarshal([]byte(view.DefinitionJSON), &definition); err != nil {
		return nil, fmt.Errorf("%w: %v", repocommercial.ErrInvalidPlanRow, err)
	}
	features := definition.Features
	if features == nil {
		features = map[string]bool{}
	}
	limits := definition.Limits
	if limits == nil {
		limits = map[string]int64{}
	}
	data := gin.H{
		"plan_key":               view.PlanKey,
		"version":                view.Version,
		"state":                  view.State,
		"name":                   definition.Name,
		"amount_fen":             strconv.FormatInt(int64(definition.Price), 10),
		"currency":               definition.Currency,
		"included_credits_micro": int64(definition.Monthly),
		"features":               features,
		"limits":                 limits,
	}
	charges := make([]gin.H, 0, len(definition.Charges))
	for _, c := range definition.Charges {
		charges = append(charges, gin.H{
			"dimension":     c.Dimension,
			"model":         c.Model,
			"amount_fen":    strconv.FormatInt(c.AmountFen, 10),
			"package_units": c.PackageUnits,
			"free_units":    c.FreeUnits,
		})
	}
	data["charges"] = charges
	if view.ReceiptJSON != "" {
		var receipt commercial.CommandReceipt
		if err := json.Unmarshal([]byte(view.ReceiptJSON), &receipt); err != nil {
			return nil, fmt.Errorf("%w: receipt", repocommercial.ErrInvalidPlanRow)
		}
		data["receipt"] = gin.H{
			"received":     true,
			"published_at": view.PublishedAt.UTC().Format(time.RFC3339),
			"command_key":  receipt.Key, // WeKnora's own key — never the external code
		}
	}
	return data, nil
}

// planAdminError maps the service errors onto closed HTTP answers; err.Error()
// is never echoed.
func planAdminError(c *gin.Context, err error, report commercialsvc.ValidationReport) {
	switch {
	case errors.Is(err, commercialsvc.ErrPublishValidationFailed):
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"success": false, "error": "publish_validation_failed", "validation": report,
		})
	case errors.Is(err, repocommercial.ErrPlanNotFound):
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "plan_version_not_found"})
	case errors.Is(err, repocommercial.ErrPublishedPlanImmutable):
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": "published_plan_immutable"})
	case errors.Is(err, commercial.ErrPlatformInvalidResponse):
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": "publish_conflict"})
	case errors.Is(err, commercial.ErrPlatformUnconfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "platform_unconfigured"})
	case errors.Is(err, commercial.ErrPlatformUnreachable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "platform_unreachable"})
	case errors.Is(err, commercialsvc.ErrInvalidDraftInput):
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid_draft_input"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "internal_error"})
	}
}

// SetPlanVersionService wires the plan version lifecycle service (injection
// point for the container). Until it is called the admin plan endpoints
// fail closed with 503 — never a fabricated success.
func (h *CommercialHandler) SetPlanVersionService(s *commercialsvc.PlanVersionService) {
	h.planVersions = s
}

func (h *CommercialHandler) requirePlanVersionService(c *gin.Context) (*commercialsvc.PlanVersionService, bool) {
	if h.planVersions == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "plan_service_unconfigured"})
		return nil, false
	}
	return h.planVersions, true
}

// CreatePlanDraft serves POST /admin/plans/drafts.
func (h *CommercialHandler) CreatePlanDraft(c *gin.Context) {
	svc, ok := h.requirePlanVersionService(c)
	if !ok {
		return
	}
	var req planDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid_draft_input"})
		return
	}
	in, err := req.toDraftInput()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid_draft_input"})
		return
	}
	view, err := svc.CreateDraft(c.Request.Context(), commercialUserID(c), in)
	if err != nil {
		planAdminError(c, err, commercialsvc.ValidationReport{})
		return
	}
	data, err := planVersionWire(view)
	if err != nil {
		planAdminError(c, err, commercialsvc.ValidationReport{})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": data})
}

// UpdatePlanDraft serves PATCH /admin/plans/drafts/:key/:version.
func (h *CommercialHandler) UpdatePlanDraft(c *gin.Context) {
	svc, ok := h.requirePlanVersionService(c)
	if !ok {
		return
	}
	version, err := strconv.ParseInt(c.Param("version"), 10, 64)
	if err != nil || version <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid_version"})
		return
	}
	var req planDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid_draft_input"})
		return
	}
	in, err := req.toDraftInput()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid_draft_input"})
		return
	}
	view, err := svc.UpdateDraft(c.Request.Context(), c.Param("key"), version, in)
	if err != nil {
		planAdminError(c, err, commercialsvc.ValidationReport{})
		return
	}
	data, err := planVersionWire(view)
	if err != nil {
		planAdminError(c, err, commercialsvc.ValidationReport{})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// ValidatePlanDraft serves POST /admin/plans/drafts/:key/:version/validate:
// the itemized six-axis report, no state change.
func (h *CommercialHandler) ValidatePlanDraft(c *gin.Context) {
	svc, ok := h.requirePlanVersionService(c)
	if !ok {
		return
	}
	version, err := strconv.ParseInt(c.Param("version"), 10, 64)
	if err != nil || version <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid_version"})
		return
	}
	report, err := svc.Validate(c.Request.Context(), c.Param("key"), version)
	if err != nil {
		planAdminError(c, err, commercialsvc.ValidationReport{})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"plan_key":   c.Param("key"),
		"version":    version,
		"validation": report,
	}})
}

// PublishPlanVersion serves POST /admin/plans/drafts/:key/:version/publish.
// The actor comes from the authenticated principal; the reason from the body.
func (h *CommercialHandler) PublishPlanVersion(c *gin.Context) {
	svc, ok := h.requirePlanVersionService(c)
	if !ok {
		return
	}
	version, err := strconv.ParseInt(c.Param("version"), 10, 64)
	if err != nil || version <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid_version"})
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req) // an empty reason is legal
	actor := commercialUserID(c)
	if actor == "" {
		actor = "platform_operator" // platform API keys carry no human id
	}
	result, err := svc.Publish(c.Request.Context(), actor, req.Reason, c.Param("key"), version)
	if err != nil {
		planAdminError(c, err, result.Report)
		return
	}
	data, err := planVersionWire(result.Version)
	if err != nil {
		planAdminError(c, err, commercialsvc.ValidationReport{})
		return
	}
	if result.Receipt != nil {
		data["receipt"] = gin.H{
			"received":     result.Receipt.Received,
			"published_at": result.Receipt.PublishedAt.UTC().Format(time.RFC3339),
			"command_key":  result.Receipt.CommandKey,
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// ListPlanVersions serves GET /admin/plans/versions.
func (h *CommercialHandler) ListPlanVersions(c *gin.Context) {
	svc, ok := h.requirePlanVersionService(c)
	if !ok {
		return
	}
	views, err := svc.ListVersions(c.Request.Context())
	if err != nil {
		planAdminError(c, err, commercialsvc.ValidationReport{})
		return
	}
	data := make([]gin.H, 0, len(views)) // SP11: never null
	for _, view := range views {
		wire, err := planVersionWire(view)
		if err != nil {
			planAdminError(c, err, commercialsvc.ValidationReport{})
			return
		}
		data = append(data, wire)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// GetPlanVersion serves GET /admin/plans/versions/:key/:version.
func (h *CommercialHandler) GetPlanVersion(c *gin.Context) {
	svc, ok := h.requirePlanVersionService(c)
	if !ok {
		return
	}
	version, err := strconv.ParseInt(c.Param("version"), 10, 64)
	if err != nil || version <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid_version"})
		return
	}
	view, err := svc.GetVersion(c.Request.Context(), c.Param("key"), version)
	if err != nil {
		planAdminError(c, err, commercialsvc.ValidationReport{})
		return
	}
	data, err := planVersionWire(view)
	if err != nil {
		planAdminError(c, err, commercialsvc.ValidationReport{})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}
