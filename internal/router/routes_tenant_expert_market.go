package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/handler"
)

// RegisterTenantExpertMarketRoutes wires the tenant-internal expert market
// surface (M4 Task 5): publishing an agent as an expert on the agents tree
// and the member-facing listing/unpublish/install under
// /market/tenant/experts.
//
// Guards follow the binding matrix, each anchored on a precedent:
//
//   - POST /agents/:id/publish-expert mirrors the agent-scoped persona
//     mutations (routes_persona.go): OwnedAgentOrAdmin for JWT sessions —
//     the creator decides their agent goes to the market, otherwise Admin+ —
//     with the exact agentsWrite API-key policy (manage_agents/full-access).
//     It is NOT a new /agents group: a second wildcard name would panic
//     against the existing /agents/:id tree, so it registers out-of-group
//     via apiKeyRoute with the same :id name.
//   - GET /market/tenant/experts is Viewer+ (catalog-read precedent:
//     GET /experts, GET /market/tenant/skills) with full-access-only keys.
//   - DELETE /market/tenant/experts/:id is Admin+ — removing market
//     inventory is a workspace-admin decision (the T4 unpublish mirror).
//   - POST /market/tenant/experts/:id/install mirrors the expert
//     instantiation it composes with (POST /experts/:id/instantiate):
//     Contributor+ for JWT sessions, manage_agents/full-access keys.
//
// The /market subtree is shared with the tenant skill market (different
// second segments); both verified by the coexistence test.
func RegisterTenantExpertMarketRoutes(r *gin.RouterGroup, marketHandler *handler.TenantExpertMarketHandler, g *rbacGuards) {
	if marketHandler == nil {
		// Not wired (embedded/test trees): nothing registers, exactly like
		// the tenant skill-market precedent. Production always provides it.
		return
	}
	// Publish — the persona-route pattern on the shared /agents/:id tree.
	g.apiKeyRoute(r, http.MethodPost, "/agents/:id/publish-expert",
		apiKeyManageAgents(apiKeyFullAccess()), g.OwnedAgentOrAdmin(), marketHandler.PublishAgentExpert)
	// Tenant market reads — Viewer+ so every member can browse what their
	// workspace published; full-access keys only (catalog-read precedent).
	tenantMarket := g.apiKeyGroup(r.Group("/market/tenant/experts"), apiKeyFullAccess())
	// tenantMarketInstantiate mirrors the expert instantiate it composes
	// with: manage_agents/full-access keys may create agents.
	tenantMarketInstantiate := tenantMarket.With(apiKeyManageAgents(apiKeyFullAccess()))
	{
		tenantMarket.GET("", g.Viewer(), marketHandler.ListPublishedExperts)
		// Unpublish — the T4 unpublish mirror: an inventory decision.
		tenantMarket.DELETE("/:id", g.Admin(), marketHandler.UnpublishExpert)
		// Published-expert install — the instantiate guard mirrored.
		tenantMarketInstantiate.POST("/:id/install", g.Contributor(), marketHandler.InstallPublishedExpert)
	}
}
