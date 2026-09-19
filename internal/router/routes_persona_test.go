package router

import (
	"net/http"
	"testing"

	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// TestPersonaRoutesRegisterBesideAgentRoutes resolves the brief's
// execution-time check: the persona agent routes must register on the SAME
// v1 tree as every existing /agents/:id route without gin's wildcard-name
// panic. Registering the production Register* calls side by side (agents,
// organization — which owns /agents/:id/shares — and persona) both proves no
// panic at registration time and pins the API-key policy declarations the
// v1-level authorizer needs, otherwise scoped keys would 403 and
// assertAPIKeyPoliciesMatchRoutes would fail at startup for a dead policy.
func TestPersonaRoutesRegisterBesideAgentRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	g := &rbacGuards{}
	r := gin.New()
	v1 := r.Group("/api/v1")

	RegisterCustomAgentRoutes(v1, &handler.CustomAgentHandler{}, g)
	RegisterOrganizationRoutes(v1, &handler.OrganizationHandler{}, g)
	RegisterPersonaRoutes(v1, &handler.PersonaHandler{}, g)

	// The full production /agents/:id surface and the persona sub-routes
	// coexist on one tree.
	registered := map[string]bool{}
	for _, ri := range r.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}
	for _, want := range []string{
		http.MethodGet + " /api/v1/agents/:id",
		http.MethodPut + " /api/v1/agents/:id",
		http.MethodPost + " /api/v1/agents/:id/copy",
		http.MethodPost + " /api/v1/agents/:id/shares",
		http.MethodPut + " /api/v1/agents/:id/persona",
		http.MethodDelete + " /api/v1/agents/:id/persona",
		http.MethodGet + " /api/v1/mbti/types",
		http.MethodGet + " /api/v1/mbti/types/:code",
		http.MethodGet + " /api/v1/mbti/preview/:code",
		http.MethodGet + " /api/v1/mbti/test/questions",
		http.MethodPost + " /api/v1/mbti/test/submit",
	} {
		if !registered[want] {
			t.Fatalf("route %s not registered", want)
		}
	}

	// The agent persona mutations declare the same manage_agents policy as
	// the agentsWrite group; the catalog routes are full-access only.
	personaPut := mustLookupAPIKeyPolicy(t, g, http.MethodPut, "/api/v1/agents/:id/persona")
	if !policyHasCapability(personaPut, types.APIKeyCapabilityManageAgents) {
		t.Fatalf("PUT persona policy = %#v, want manage_agents capability", personaPut.Capabilities)
	}
	mbtiTypes := mustLookupAPIKeyPolicy(t, g, http.MethodGet, "/api/v1/mbti/types")
	if !mbtiTypes.RequireFullAccess {
		t.Fatal("GET /mbti/types policy should require full access (no capability grants)")
	}
}
