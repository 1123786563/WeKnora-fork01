package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/handler"
)

// RegisterAgentVersionRoutes wires the Agent domain's immutable version
// surface (T28 Wave 1): freeze/read frozen snapshots under
// /agents/:id/versions.
//
// The routes are NOT a new /agents group: a new wildcard name (e.g.
// :agentID) panics against the existing /agents/:id tree, so they register
// with the same :id name via apiKeyRoute — the same out-of-group pattern
// routes_persona.go and routes_subagent.go use for agent-scoped siblings.
//
// Guards follow the binding matrix:
//
//   - POST (freeze) mirrors the agent-scoped persona mutations: OwnedAgentOrAdmin
//     for JWT sessions — the creator decides their agent is frozen as-is,
//     otherwise Admin+ — with the exact agentsWrite API-key policy
//     (manage_agents/full-access).
//   - GET (list/read one) carries the agentsRead matrix: Viewer+ sessions,
//     read_agents/chat/manage_agents/full-access keys (agent config inside a
//     snapshot can carry sensitive bindings, so plain scoped keys stay out).
func RegisterAgentVersionRoutes(r *gin.RouterGroup, versionHandler *handler.AgentVersionHandler, g *rbacGuards) {
	if versionHandler == nil {
		// Not wired (embedded/test trees): nothing registers, exactly like
		// the tenant expert-market precedent. Production always provides it.
		return
	}
	// Freeze — the persona-route pattern on the shared /agents/:id tree.
	g.apiKeyRoute(r, http.MethodPost, "/agents/:id/versions",
		apiKeyManageAgents(apiKeyFullAccess()), g.OwnedAgentOrAdmin(),
		versionHandler.FreezeAgentVersion)
	// Reads — the agentsRead policy matrix.
	agentsVersionRead := apiKeyReadAgents(apiKeyManageAgents(apiKeyChat(apiKeyFullAccess())))
	g.apiKeyRoute(r, http.MethodGet, "/agents/:id/versions",
		agentsVersionRead, g.Viewer(), versionHandler.ListAgentVersions)
	g.apiKeyRoute(r, http.MethodGet, "/agents/:id/versions/:versionId",
		agentsVersionRead, g.Viewer(), versionHandler.GetAgentVersion)
}
