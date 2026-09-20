package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/handler"
)

// RegisterSkillMarketRoutes wires the SkillHub market surface (M4 Task 3):
// skill search/rankings/install under /skills/market and the skillset
// (expert) index/install under /experts/market.
//
// Guards follow the plan's binding matrix, each anchored on a precedent:
//
//   - search/rankings and the skillset reads are Viewer+ (catalog-read
//     precedent: GET /skills/catalog, GET /experts) with full-access-only
//     API keys — scoped keys keep the default-deny on a surface they have
//     no capability for;
//   - POST /skills/market/install mirrors the catalog install it composes
//     with (POST /skills/catalog/:id/install): Admin+, full-access keys —
//     the route bakes remote code into sandbox images;
//   - POST /experts/market/:slug/install mirrors the expert instantiate
//     precedent (POST /experts/:id/instantiate): Contributor+ for JWT
//     sessions, manage_agents/full-access keys for machine principals.
//
// The /experts/market/:slug wildcard lives beside the existing
// /experts/:id tree (different subtree depth, no shared segment), and
// /skills/market adds no wildcard at all — both verified by the coexistence
// test.
func RegisterSkillMarketRoutes(r *gin.RouterGroup, marketHandler *handler.SkillMarketHandler, g *rbacGuards) {
	if marketHandler == nil {
		// Not wired (embedded/test trees): nothing registers, exactly like
		// the embed-channel precedent. Production always provides it.
		return
	}
	// Skill market reads — Viewer+, full-access keys only.
	skillMarketRead := g.apiKeyGroup(r.Group("/skills/market"), apiKeyFullAccess())
	{
		skillMarketRead.GET("/search", g.Viewer(), marketHandler.Search)
		skillMarketRead.GET("/rankings/:kind", g.Viewer(), marketHandler.Rankings)
	}
	// Remote skill install — the catalog-install guard mirrored: scoped API
	// keys cannot bake remote code into sandbox images.
	g.apiKeyRoute(r, http.MethodPost, "/skills/market/install", apiKeyFullAccess(), g.Admin(), marketHandler.InstallSkill)

	// Expert market (skillset) reads — Viewer+, full-access keys only.
	expertMarketRead := g.apiKeyGroup(r.Group("/experts/market"), apiKeyFullAccess())
	{
		expertMarketRead.GET("", g.Viewer(), marketHandler.ListSkillsets)
		expertMarketRead.GET("/:slug", g.Viewer(), marketHandler.GetSkillset)
	}
	// Skillset install — the expert-instantiate matrix: Contributor+ for
	// JWT sessions, manage_agents/full-access keys.
	expertMarketWrite := expertMarketRead.With(apiKeyManageAgents(apiKeyFullAccess()))
	{
		expertMarketWrite.POST("/:slug/install", g.Contributor(), marketHandler.InstallSkillset)
	}
}
