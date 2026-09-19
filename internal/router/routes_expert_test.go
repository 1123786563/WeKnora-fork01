package router

import (
	"net/http"
	"testing"

	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// TestExpertRoutesRegisterBesideAgentAndPersonaRoutes mirrors the M1 persona
// coexistence test: the expert routes must register on the SAME v1 tree as
// the agent and persona routes without gin's wildcard-name panic, and must
// declare the API-key policies the v1-level authorizer asserts at startup
// (assertAPIKeyPoliciesMatchRoutes) — catalog reads full-access only (like
// /mbti), instantiate carrying manage_agents (like agentsWrite).
func TestExpertRoutesRegisterBesideAgentAndPersonaRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	g := &rbacGuards{}
	r := gin.New()
	v1 := r.Group("/api/v1")

	RegisterCustomAgentRoutes(v1, &handler.CustomAgentHandler{}, g)
	RegisterOrganizationRoutes(v1, &handler.OrganizationHandler{}, g)
	RegisterPersonaRoutes(v1, &handler.PersonaHandler{}, g)
	RegisterExpertRoutes(v1, &handler.ExpertHandler{}, g)

	// The full production /agents/:id surface, the persona sub-routes, and
	// the new /experts group coexist on one tree.
	registered := map[string]bool{}
	for _, ri := range r.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}
	for _, want := range []string{
		http.MethodGet + " /api/v1/agents/:id",
		http.MethodPost + " /api/v1/agents/:id/copy",
		http.MethodPut + " /api/v1/agents/:id/persona",
		http.MethodGet + " /api/v1/mbti/types",
		http.MethodGet + " /api/v1/experts",
		http.MethodGet + " /api/v1/experts/:id",
		http.MethodPost + " /api/v1/experts/:id/instantiate",
	} {
		if !registered[want] {
			t.Fatalf("route %s not registered", want)
		}
	}

	// Policy pins: the /experts wildcard subtree is new, so it owns its :id
	// name without colliding with /agents/:id.
	list := mustLookupAPIKeyPolicy(t, g, http.MethodGet, "/api/v1/experts")
	if !list.RequireFullAccess || len(list.Capabilities) != 0 {
		t.Fatalf("GET /experts policy = %#v, want full-access only", list)
	}
	detail := mustLookupAPIKeyPolicy(t, g, http.MethodGet, "/api/v1/experts/:id")
	if !detail.RequireFullAccess || len(detail.Capabilities) != 0 {
		t.Fatalf("GET /experts/:id policy = %#v, want full-access only", detail)
	}
	instantiate := mustLookupAPIKeyPolicy(t, g, http.MethodPost, "/api/v1/experts/:id/instantiate")
	if !policyHasCapability(instantiate, types.APIKeyCapabilityManageAgents) {
		t.Fatalf("POST /experts/:id/instantiate policy = %#v, want manage_agents capability", instantiate.Capabilities)
	}
}
