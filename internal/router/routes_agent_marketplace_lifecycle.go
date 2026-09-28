package router

import (
	"net/http"

	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterAgentMarketplaceLifecycleRoutes mounts Tenant marketplace
// retirement, ending, unlisting and release deprecation governance actions.
// Every mutation is Admin+ with the full-access API-key floor; a nil handler
// fails closed and mounts no routes.
func RegisterAgentMarketplaceLifecycleRoutes(r *gin.RouterGroup, lifecycleHandler *handler.AgentMarketplaceLifecycleHandler, g *rbacGuards) {
	if lifecycleHandler == nil {
		return
	}
	admin := apiKeyFullAccess()
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/tenant/variants/:id/retire", admin, g.Admin(), lifecycleHandler.RetireVariant)
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/tenant/adoptions/:id/end", admin, g.Admin(), lifecycleHandler.EndAdoption)
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/tenant/listings/:id/unlist", admin, g.Admin(), lifecycleHandler.UnlistListing)
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/tenant/releases/:id/deprecate", admin, g.Admin(), lifecycleHandler.DeprecateRelease)
}
