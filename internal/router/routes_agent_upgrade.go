package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/handler"
)

// RegisterAgentUpgradeRoutes mounts the reviewable upgrade-proposal surface
// beside the adoption routes (T31 #61, spec §9). Proposal review,
// acceptance and dismissal are admin-grade governance acts, so every route
// is Admin+ with the full-access API-key floor — mirroring
// RegisterAgentAdoptionRoutes. A nil handler mounts nothing (fail closed).
func RegisterAgentUpgradeRoutes(r *gin.RouterGroup, upgradeHandler *handler.AgentUpgradeHandler, g *rbacGuards) {
	if upgradeHandler == nil {
		return
	}
	admin := apiKeyFullAccess()
	g.apiKeyRoute(r, http.MethodGet, "/marketplace/tenant/upgrade-proposals", admin, g.Admin(), upgradeHandler.ListUpgradeProposals)
	g.apiKeyRoute(r, http.MethodGet, "/marketplace/tenant/upgrade-proposals/:id", admin, g.Admin(), upgradeHandler.GetUpgradeProposal)
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/tenant/upgrade-proposals/:id/accept", admin, g.Admin(), upgradeHandler.AcceptUpgradeProposal)
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/tenant/upgrade-proposals/:id/dismiss", admin, g.Admin(), upgradeHandler.DismissUpgradeProposal)
}
