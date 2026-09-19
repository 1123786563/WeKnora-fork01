package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/handler"
)

// RegisterSubagentRoutes registers the builtin sub-agent role catalog routes.
//
// The catalog reads are static views over the shipped
// config/subagents/library — a Viewer floor matches the other catalog reads
// (mbti types, experts, agent presets), and only full-access API keys may
// reach them, keeping scoped keys default-denied on a surface they have no
// capability for (the /experts policy).
//
// The agent-scoped routes are NOT a new /agents group: a new wildcard name
// (e.g. :agentID) panics against the existing /agents/:id tree, so they
// register with the same :id name via apiKeyRoute — the same out-of-group
// pattern routes_persona.go uses for /agents/:id/persona — with the exact
// guards and API-key policies of routes_agent.go's agentsRead/agentsWrite
// matrices (creator OR Admin+; read_agents or manage_agents/full-access
// keys).
func RegisterSubagentRoutes(r *gin.RouterGroup, subagentHandler *handler.SubagentHandler, g *rbacGuards) {
	catalog := g.apiKeyGroup(r.Group("/subagent-catalog"), apiKeyFullAccess())
	{
		// List the role library with installed flags — Viewer+
		catalog.GET("", g.Viewer(), subagentHandler.ListCatalog)
		// One role's both-locale detail — Viewer+
		catalog.GET("/:slug", g.Viewer(), subagentHandler.GetCatalogEntry)
	}
	// Agent-scoped subagent config — the agentsRead/agentsWrite matrices.
	g.apiKeyRoute(r, http.MethodGet, "/agents/:id/subagents",
		apiKeyReadAgents(apiKeyManageAgents(apiKeyFullAccess())), g.OwnedAgentOrAdmin(),
		subagentHandler.ListAgentSubagents)
	g.apiKeyRoute(r, http.MethodPost, "/agents/:id/subagents",
		apiKeyManageAgents(apiKeyFullAccess()), g.OwnedAgentOrAdmin(),
		subagentHandler.InstallForAgent)
	g.apiKeyRoute(r, http.MethodDelete, "/agents/:id/subagents/:slug",
		apiKeyManageAgents(apiKeyFullAccess()), g.OwnedAgentOrAdmin(),
		subagentHandler.RemoveFromAgent)
}
