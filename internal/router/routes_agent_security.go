package router

import (
	"net/http"

	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterAgentSecurityRoutes registers admin-only security revocation and
// audit routes. A missing handler fails closed by exposing no routes.
func RegisterAgentSecurityRoutes(r *gin.RouterGroup, securityHandler *handler.AgentSecurityHandler, g *rbacGuards) {
	if securityHandler == nil {
		return
	}
	admin := apiKeyFullAccess()
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/tenant/security-revocations/releases", admin, g.Admin(), securityHandler.RevokeRelease)
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/tenant/security-revocations/dependencies", admin, g.Admin(), securityHandler.RevokeDependency)
	g.apiKeyRoute(r, http.MethodGet, "/marketplace/tenant/security-revocations", admin, g.Admin(), securityHandler.ListRevocations)
	g.apiKeyRoute(r, http.MethodGet, "/marketplace/tenant/security-revocations/:id", admin, g.Admin(), securityHandler.GetRevocation)
}
