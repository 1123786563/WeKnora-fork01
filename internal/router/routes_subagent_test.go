package router

import (
	"net/http"
	"testing"

	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// TestSubagentRoutesRegisterBesideAgentAndExpertRoutes mirrors the M1/M2
// coexistence tests: the subagent routes must register on the SAME v1 tree as
// the agent, persona, and expert routes without gin's wildcard-name panic.
// The agent-scoped sub-routes reuse the existing /agents/:id wildcard name
// via apiKeyRoute (the persona pattern) — a parallel :agentID group would
// panic — and every route must declare the API-key policy the v1-level
// authorizer asserts at startup (assertAPIKeyPoliciesMatchRoutes).
func TestSubagentRoutesRegisterBesideAgentAndExpertRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	g := &rbacGuards{}
	r := gin.New()
	v1 := r.Group("/api/v1")

	RegisterCustomAgentRoutes(v1, &handler.CustomAgentHandler{}, g)
	RegisterOrganizationRoutes(v1, &handler.OrganizationHandler{}, g)
	RegisterPersonaRoutes(v1, &handler.PersonaHandler{}, g)
	RegisterExpertRoutes(v1, &handler.ExpertHandler{}, g)
	RegisterSubagentRoutes(v1, &handler.SubagentHandler{}, g)

	// The full production /agents/:id surface, the persona/expert sub-routes,
	// and the new subagent routes coexist on one tree.
	registered := map[string]bool{}
	for _, ri := range r.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}
	for _, want := range []string{
		http.MethodGet + " /api/v1/agents/:id",
		http.MethodPost + " /api/v1/agents/:id/copy",
		http.MethodPost + " /api/v1/agents/:id/shares",
		http.MethodPut + " /api/v1/agents/:id/persona",
		http.MethodGet + " /api/v1/experts",
		http.MethodPost + " /api/v1/experts/:id/instantiate",
		http.MethodGet + " /api/v1/subagent-catalog",
		http.MethodGet + " /api/v1/subagent-catalog/:slug",
		http.MethodGet + " /api/v1/agents/:id/subagents",
		http.MethodPost + " /api/v1/agents/:id/subagents",
		http.MethodDelete + " /api/v1/agents/:id/subagents/:slug",
	} {
		if !registered[want] {
			t.Fatalf("route %s not registered", want)
		}
	}

	// Policy pins: catalog reads are full-access only (like /mbti and
	// /experts); the agent-scoped reads carry the agentsRead matrix and the
	// mutations the agentsWrite matrix (manage_agents), matching persona.
	list := mustLookupAPIKeyPolicy(t, g, http.MethodGet, "/api/v1/subagent-catalog")
	if !list.RequireFullAccess || len(list.Capabilities) != 0 {
		t.Fatalf("GET /subagent-catalog policy = %#v, want full-access only", list)
	}
	detail := mustLookupAPIKeyPolicy(t, g, http.MethodGet, "/api/v1/subagent-catalog/:slug")
	if !detail.RequireFullAccess || len(detail.Capabilities) != 0 {
		t.Fatalf("GET /subagent-catalog/:slug policy = %#v, want full-access only", detail)
	}
	agentList := mustLookupAPIKeyPolicy(t, g, http.MethodGet, "/api/v1/agents/:id/subagents")
	if !policyHasCapability(agentList, types.APIKeyCapabilityReadAgents) {
		t.Fatalf("GET /agents/:id/subagents policy = %#v, want read_agents capability", agentList.Capabilities)
	}
	install := mustLookupAPIKeyPolicy(t, g, http.MethodPost, "/api/v1/agents/:id/subagents")
	if !policyHasCapability(install, types.APIKeyCapabilityManageAgents) {
		t.Fatalf("POST /agents/:id/subagents policy = %#v, want manage_agents capability", install.Capabilities)
	}
	remove := mustLookupAPIKeyPolicy(t, g, http.MethodDelete, "/api/v1/agents/:id/subagents/:slug")
	if !policyHasCapability(remove, types.APIKeyCapabilityManageAgents) {
		t.Fatalf("DELETE /agents/:id/subagents/:slug policy = %#v, want manage_agents capability", remove.Capabilities)
	}
}
