package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/handler"
)

// RegisterTenantSkillMarketRoutes wires the tenant-internal skill market
// surface (M4 Task 4): publish/unpublish on the skills catalog tree and the
// member-facing listing/install under /market/tenant/skills.
//
// Guards follow the binding matrix, each anchored on a precedent:
//
//   - POST/DELETE /skills/catalog/:id/publish mirror the catalog-write
//     guard (POST /skills/catalog/:id install et al.): Admin+, full-access
//     keys — publishing decides what code every member can bake into
//     sandbox images;
//   - GET /market/tenant/skills is Viewer+ (catalog-read precedent:
//     GET /skills/catalog) with full-access-only API keys;
//   - POST /market/tenant/skills/:catalogId/install mirrors the catalog
//     install it composes with (POST /skills/catalog/:id/install): Admin+,
//     full-access keys — the route bakes code into sandbox images.
//
// The /skills/catalog/:id/publish segment shares the :id wildcard with the
// existing /skills/catalog/:id tree (same depth, same name — no gin
// conflict), and /market is a fresh subtree; both verified by the
// coexistence test.
func RegisterTenantSkillMarketRoutes(r *gin.RouterGroup, marketHandler *handler.TenantSkillMarketHandler, g *rbacGuards) {
	if marketHandler == nil {
		// Not wired (embedded/test trees): nothing registers, exactly like
		// the skill-market precedent. Production always provides it.
		return
	}
	// Publish/unpublish — the catalog-write guard mirrored: scoped API keys
	// cannot decide what the whole workspace can install.
	catalogPublish := g.apiKeyGroup(r.Group("/skills/catalog"), apiKeyFullAccess())
	{
		catalogPublish.POST("/:id/publish", g.Admin(), marketHandler.PublishSkill)
		catalogPublish.DELETE("/:id/publish", g.Admin(), marketHandler.UnpublishSkill)
	}
	// Tenant market reads — Viewer+ so every member can browse what their
	// workspace published; full-access keys only (catalog-read precedent).
	tenantMarketRead := g.apiKeyGroup(r.Group("/market/tenant/skills"), apiKeyFullAccess())
	{
		tenantMarketRead.GET("", g.Viewer(), marketHandler.ListPublishedSkills)
	}
	// Published-skill install — the catalog-install guard mirrored.
	g.apiKeyRoute(r, http.MethodPost, "/market/tenant/skills/:catalogId/install",
		apiKeyFullAccess(), g.Admin(), marketHandler.InstallPublishedSkill)
}
