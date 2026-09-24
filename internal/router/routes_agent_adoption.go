package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/handler"
)

// RegisterAgentAdoptionRoutes mounts the Tenant Adoption governance
// workflow beside the tenant release routes. Writing an Adoption, deriving
// Variants, mapping capabilities, testing and publishing are admin-grade
// governance acts (spec §13 adopt_agent / configure_variant /
// test_variant / publish_agent_version), so every mutation is Admin+ with
// the full-access API-key floor; only the available-agent read model is
// Viewer+ (spec §2 mobile read-only surface).
func RegisterAgentAdoptionRoutes(r *gin.RouterGroup, adoptionHandler *handler.AgentAdoptionHandler, g *rbacGuards) {
	if adoptionHandler == nil {
		return
	}
	admin := apiKeyFullAccess()
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/tenant/adoptions", admin, g.Admin(), adoptionHandler.Adopt)
	g.apiKeyRoute(r, http.MethodGet, "/marketplace/tenant/adoptions", admin, g.Admin(), adoptionHandler.ListAdoptions)
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/tenant/adoptions/:id/variants", admin, g.Admin(), adoptionHandler.CreateVariant)
	g.apiKeyRoute(r, http.MethodPut, "/marketplace/tenant/variants/:id/capability-mapping", admin, g.Admin(), adoptionHandler.UpdateCapabilityMapping)
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/tenant/variants/:id/test", admin, g.Admin(), adoptionHandler.TestVariant)
}
