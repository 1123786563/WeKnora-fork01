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
		commercialGroup.POST("/orders", commercialHandler.NotImplemented)
	}

	// Provider payment callbacks: publicly reachable and authenticated by
	// provider signature verification instead of session/API-key. They hang
	// DIRECTLY on the parent group — the capability/billing guards above
	// would reject an unauthenticated provider call. There is no tenant
	// parameter by design: the handler rebuilds the trusted tenant from the
	// local order registry after Verify succeeds. With no provider wired
	// yet the route still exists and fails closed (503 FAIL, nothing
	// persisted); later tasks pass a configured callbacks handler.
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
