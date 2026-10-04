package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterCommercialRoutes registers the commercial read endpoints under
// /api/v1/commercial. The brief-mandated signature carries no rbacGuards,
// so these routes are intentionally NOT declared in the API-key route
// authorizer: the /api/v1 gate default-denies every X-API-Key principal
// (403) — full access never auto-implies purchase authority. The handler
// adds a second layer that admits only keys carrying an explicit
// commercial capability. Write operations land on this same group in
// later tasks; until then a guarded POST stub keeps the
// commercial.CanManageBilling gate observable (unauthorized Admin 403,
// authorised caller 501). Tenant scope is ALWAYS derived from the
// authenticated context — no tenant path parameter exists by design.
func RegisterCommercialRoutes(r *gin.RouterGroup, commercialHandler *handler.CommercialHandler, callbacksHandlers ...*handler.PaymentCallbacksHandler) {
	if commercialHandler == nil {
		return
	}
	commercialGroup := r.Group("/commercial",
		commercialHandler.RequireExplicitCommercialCapability(),
		commercialHandler.RequireManageBillingForWrites(),
	)
	{
		commercialGroup.GET("/summary", commercialHandler.Summary)
		commercialGroup.GET("/plans", commercialHandler.Plans)
		commercialGroup.GET("/usage", commercialHandler.Usage)
		commercialGroup.GET("/orders", commercialHandler.Orders)
		// T05: the one protected, read-only Billing API operation through the
		// frozen Commercial Platform seam — the billing authority's
		// readiness/version snapshot, answered in closed product vocabulary.
		// The capability gate above applies; as a GET it bypasses the
		// billing-role write gate by design (it is a read). The handler fails
		// closed when no platform is wired (unavailable/unconfigured).
		commercialGroup.GET("/platform/readiness", commercialHandler.PlatformReadiness)
		// T06 (#78): the caller space's billing account status. This GET is
		// the documented LAZY ENSURE trigger (first billing access): an
		// idempotent, authority-failure-safe establish of the space's
		// Billing Account, answered in the closed linked|pending envelope —
		// no provider identifier ever crosses.
		commercialGroup.GET("/account", commercialHandler.AccountStatus)
		// P02: quote and order pipeline. The billing gate and capability
		// checks above apply; tenant comes exclusively from the authenticated
		// context. Checkout names the channel provider explicitly.
		commercialGroup.POST("/quotes", commercialHandler.CreateQuote)
		// #85 G-A: the one-shot credit top-up offer. Same checkout chain
		// as /quotes; the amount is the caller's choice within the closed
		// whole-CNY book rate.
		commercialGroup.POST("/topup-quotes", commercialHandler.CreateTopUpQuote)
		commercialGroup.POST("/orders", commercialHandler.CreateOrder)
		commercialGroup.GET("/orders/:id", commercialHandler.GetOrder)
		// W5 (#81): the payment-gated purchase. The POST submits a frozen
		// quote into the gated subscription creation — the quote-vs-authority
		// match gate runs BEFORE any channel payment request exists (AC2).
		// The GET answers the space's purchase state in closed product
		// vocabulary (awaiting_payment shows 待付款; entitlements stay closed
		// while gated — D4). The billing gate and capability checks above
		// apply; the tenant comes exclusively from the authenticated context.
		commercialGroup.POST("/purchases", commercialHandler.Purchase)
		commercialGroup.GET("/purchase", commercialHandler.PurchaseStatus)
		// Commerce.ChangePlan: an upgrade settles as a prorated order through
		// the same checkout pipeline; anything else is scheduled for the end
		// of the paid period. Conflicts answer with a re-quote signal.
		commercialGroup.POST("/plans/change", commercialHandler.ChangePlan)
		// C05: space-scoped refund REQUEST. The group's billing gate and
		// capability checks apply; the tenant comes from the authenticated
		// context. The request registers intent only — money moves solely
		// through the admin review path below.
		commercialGroup.POST("/refunds", commercialHandler.CreateRefund)
	}

	// W05 → T12 (#42): raising one task run's budget admits the TASK OWNER
	// as well as billing authority (CONTEXT.md 任务预算：只有任务所有者或获
	// 授权的账单管理员可以增加上限). The owner arm lives inside the handler
	// (it needs the run row), so this route leaves the group-level billing
	// write gate and keeps only the explicit commercial capability gate —
	// the same direct-on-parent shape the platform refund review uses below.
	// Calls the U04 BudgetService.Extend semantics (exactly-once per
	// idempotency key, explicit pause reasons); a budget extension never
	// authorizes an external write action — that approval lives on
	// /apps/actions.
	budgetGroup := r.Group("/commercial", commercialHandler.RequireExplicitCommercialCapability())
	budgetGroup.POST("/tasks/:id/budget/extend", commercialHandler.ExtendTaskBudget)
	// T09 (#39): the task budget readout (estimated/used/reserved/remaining
	// from the ROOT budget row — delegated runs charge it exactly once) plus
	// delegated/paused run lists and the caller's own can_extend verdict.
	// Read gate lives in the handler: billing authority, the task owner
	// (sessions.user_id) or a #42 task-grant holder.
	budgetGroup.GET("/tasks/:id/budget", commercialHandler.GetTaskBudget)

	// C05: platform refund REVIEW — a separate permission path from the
	// tenant billing gate above (review moves money out of the space, so
	// tenant billing authority alone must never admit a reviewer). It
	// hangs DIRECTLY on the parent group with its own admin-authority
	// guard; the id names the refund under review.
	r.POST("/admin/refunds/:id/review",
		commercialHandler.RequirePlatformRefundReviewer(),
		commercialHandler.AdminReviewRefund)

	// #84: platform payment-anomaly disposition surface — the operator queue
	// for retained abnormal payment facts (mismatched / partial / wrong
	// currency / multiple-success, spec L127). Same authority model as the
	// refund review above: cross-space money handling hangs DIRECTLY on the
	// parent group behind the platform-operator gate; space administration
	// is 403. Responses are closed WeKnora vocabulary only (spec L170).
	anomalyAdmin := r.Group("/admin/payment-anomalies", commercialHandler.RequirePlatformRefundReviewer())
	{
		anomalyAdmin.GET("", commercialHandler.AdminListPaymentAnomalies)
		anomalyAdmin.POST("/:id/resolve", commercialHandler.AdminResolvePaymentAnomaly)
	}

	fulfillmentAdmin := r.Group("/admin/fulfillment-attentions", commercialHandler.RequirePlatformRefundReviewer())
	fulfillmentAdmin.GET("", commercialHandler.AdminListFulfillmentAttentions)

	// T07 (#79): platform plan-version admin surface. Catalog operations
	// are cross-space, so the endpoints hang DIRECTLY on the parent group
	// behind their OWN platform-operator gate — the exact refund-review
	// precedent: a platform API key (scope.IsPlatform) or the explicit
	// plan_publish grant at platform scope (tenant_id=0). A space owner,
	// Admin or billing grantee is 403; drafts are never reachable through
	// tenant authority. Responses are closed WeKnora vocabulary only: the
	// external plan code lives in commercial_plan_publications and never
	// crosses this API (ADR-0014).
	planAdmin := r.Group("/admin/plans", commercialHandler.RequirePlatformPlanPublisher())
	{
		planAdmin.POST("/drafts", commercialHandler.CreatePlanDraft)
		planAdmin.PATCH("/drafts/:key/:version", commercialHandler.UpdatePlanDraft)
		planAdmin.POST("/drafts/:key/:version/validate", commercialHandler.ValidatePlanDraft)
		planAdmin.POST("/drafts/:key/:version/publish", commercialHandler.PublishPlanVersion)
		planAdmin.GET("/versions", commercialHandler.ListPlanVersions)
		planAdmin.GET("/versions/:key/:version", commercialHandler.GetPlanVersion)
	}

	// Provider payment callbacks: publicly reachable and authenticated by
	// provider signature verification instead of session/API-key. They hang
	// DIRECTLY on the parent group — the capability/billing guards above
	// would reject an unauthenticated provider call. There is no tenant
	// parameter by design: the handler rebuilds the trusted tenant from the
	// local order registry after Verify succeeds. Production passes the
	// container-built handler (db + ProvidersFromEnv channels); a nil
	// argument falls back to a fail-closed handler (503 FAIL, nothing
	// persisted) so tests and partial deployments stay honest.
	var callbacks *handler.PaymentCallbacksHandler
	for _, ch := range callbacksHandlers {
		if ch != nil {
			callbacks = ch
			break
		}
	}
	if callbacks == nil {
		callbacks = handler.NewPaymentCallbacksHandler(nil, nil)
	}
	callbacksGroup := r.Group("/commercial/callbacks")
	{
		callbacksGroup.POST("/:provider", callbacks.HandleProviderCallback)
	}
}
