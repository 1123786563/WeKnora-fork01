package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/handler"
)

// RegisterPersonaRoutes registers the MBTI persona routes.
//
// The /mbti catalog and test endpoints are static, read-only views over the
// embedded persona package; a Viewer floor matches the other catalog reads
// (skills, agent presets), and only full-access API keys may reach them —
// scoped keys keep the default-deny.
//
// The agent-scoped persona mutations are NOT a new /agents group: a new
// wildcard name (e.g. :agentID) panics against the existing /agents/:id
// tree, so they register with the same :id name via apiKeyRoute — the same
// out-of-group pattern routes_agent.go uses for /agents/:id/suggested-questions
// — with the exact guard and API-key policy of the agentsWrite group
// (creator OR Admin+, manage_agents/full-access keys).
func RegisterPersonaRoutes(r *gin.RouterGroup, personaHandler *handler.PersonaHandler, g *rbacGuards) {
	mbti := g.apiKeyGroup(r.Group("/mbti"), apiKeyFullAccess())
	{
		// Static siblings first, then the :code routes.
		mbti.GET("/types", g.Viewer(), personaHandler.ListTypes)
		mbti.GET("/test/questions", g.Viewer(), personaHandler.TestQuestions)
		mbti.POST("/test/submit", g.Viewer(), personaHandler.TestSubmit)
		mbti.GET("/types/:code", g.Viewer(), personaHandler.GetType)
		mbti.GET("/preview/:code", g.Viewer(), personaHandler.Preview)
	}
	// Apply/remove persona — creator OR Admin+, same matrix as agent updates.
	g.apiKeyRoute(r, http.MethodPut, "/agents/:id/persona",
		apiKeyManageAgents(apiKeyFullAccess()), g.OwnedAgentOrAdmin(), personaHandler.ApplyPersona)
	g.apiKeyRoute(r, http.MethodDelete, "/agents/:id/persona",
		apiKeyManageAgents(apiKeyFullAccess()), g.OwnedAgentOrAdmin(), personaHandler.RemovePersona)
}
