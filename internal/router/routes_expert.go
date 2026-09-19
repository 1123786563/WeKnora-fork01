package router

import (
	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/handler"
)

// RegisterExpertRoutes registers the expert-template library routes.
//
// The catalog reads are static views over the shipped config/experts
// templates — a Viewer floor matches the other catalog reads (MBTI types,
// agent presets), and only full-access API keys may reach them, keeping
// scoped keys default-denied on a surface they have no capability for.
//
// Instantiate creates a tenant agent, so it mirrors the agentsWrite matrix
// of routes_agent.go's POST /agents: Contributor+ for JWT sessions, and
// manage_agents/full-access keys for machine principals. The /experts
// subtree is new, so its :id wildcard owns the name without touching the
// existing /agents/:id tree.
func RegisterExpertRoutes(r *gin.RouterGroup, expertHandler *handler.ExpertHandler, g *rbacGuards) {
	experts := g.apiKeyGroup(r.Group("/experts"), apiKeyFullAccess())
	expertsWrite := experts.With(apiKeyManageAgents(apiKeyFullAccess()))
	{
		// List expert summaries — Viewer+
		experts.GET("", g.Viewer(), expertHandler.ListExperts)
		// Expert detail + persona preview — Viewer+
		experts.GET("/:id", g.Viewer(), expertHandler.GetExpert)
		// Instantiate expert as a custom agent — Contributor+ (mirrors POST /agents)
		expertsWrite.POST("/:id/instantiate", g.Contributor(), expertHandler.Instantiate)
	}
}
