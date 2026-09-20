package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/types"
)

// TestTenantExpertMarketRoutesRegisterBesideAgentRoutes pins the M4 Task 5
// coexistence contract: the tenant expert market registers on the SAME v1
// tree as the existing agents, persona, expert and tenant-skill-market
// routes without a gin wildcard panic — POST /agents/:id/publish-expert
// sits beside /agents/:id, /agents/:id/copy and /agents/:id/persona (same
// :id wildcard at the same depth), and /market/tenant/experts/:id/install
// adds a fresh sibling subtree beside /market/tenant/skills — and declares
// the API-key policies the v1 gate asserts at startup: publish mirrors the
// agentsWrite matrix (manage_agents/full-access), the listing is Viewer+
// full-access (catalog-read precedent), unpublish is Admin+ full-access and
// install mirrors POST /experts/:id/instantiate (Contributor+,
// manage_agents/full-access).
func TestTenantExpertMarketRoutesRegisterBesideAgentRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	g := &rbacGuards{}
	r := gin.New()
	v1 := r.Group("/api/v1")

	RegisterCustomAgentRoutes(v1, &handler.CustomAgentHandler{}, g)
	RegisterPersonaRoutes(v1, &handler.PersonaHandler{}, g)
	RegisterExpertRoutes(v1, &handler.ExpertHandler{}, g)
	RegisterTenantSkillMarketRoutes(v1, &handler.TenantSkillMarketHandler{}, g)
	RegisterTenantExpertMarketRoutes(v1, &handler.TenantExpertMarketHandler{}, g)

	registered := map[string]bool{}
	for _, ri := range r.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}

	// The tenant-expert-market routes themselves.
	for _, want := range []string{
		http.MethodPost + " /api/v1/agents/:id/publish-expert",
		http.MethodGet + " /api/v1/market/tenant/experts",
		http.MethodDelete + " /api/v1/market/tenant/experts/:id",
		http.MethodPost + " /api/v1/market/tenant/experts/:id/install",
	} {
		if !registered[want] {
			t.Fatalf("route %s not registered", want)
		}
	}

	// The pre-existing neighbours survive on the same tree (the :id wildcard
	// segment is shared, not conflicting).
	for _, want := range []string{
		http.MethodGet + " /api/v1/agents/:id",
		http.MethodPut + " /api/v1/agents/:id",
		http.MethodPost + " /api/v1/agents/:id/copy",
		http.MethodPut + " /api/v1/agents/:id/persona",
		http.MethodPost + " /api/v1/experts/:id/instantiate",
		http.MethodGet + " /api/v1/market/tenant/skills",
		http.MethodPost + " /api/v1/market/tenant/skills/:catalogId/install",
	} {
		if !registered[want] {
			t.Fatalf("pre-existing route %s vanished from the shared tree", want)
		}
	}

	// Policy pins.
	publish := mustLookupAPIKeyPolicy(t, g, http.MethodPost, "/api/v1/agents/:id/publish-expert")
	if !publish.RequireFullAccess || !policyHasCapability(publish, types.APIKeyCapabilityManageAgents) {
		t.Fatalf("POST /agents/:id/publish-expert policy = %#v, want manage_agents/full-access (agentsWrite mirror)", publish)
	}
	list := mustLookupAPIKeyPolicy(t, g, http.MethodGet, "/api/v1/market/tenant/experts")
	if !list.RequireFullAccess || len(list.Capabilities) != 0 {
		t.Fatalf("GET /market/tenant/experts policy = %#v, want full-access only (catalog-read mirror)", list)
	}
	unpublish := mustLookupAPIKeyPolicy(t, g, http.MethodDelete, "/api/v1/market/tenant/experts/:id")
	if !unpublish.RequireFullAccess || len(unpublish.Capabilities) != 0 {
		t.Fatalf("DELETE /market/tenant/experts/:id policy = %#v, want full-access only", unpublish)
	}
	install := mustLookupAPIKeyPolicy(t, g, http.MethodPost, "/api/v1/market/tenant/experts/:id/install")
	if !install.RequireFullAccess || !policyHasCapability(install, types.APIKeyCapabilityManageAgents) {
		t.Fatalf("POST /market/tenant/experts/:id/install policy = %#v, want manage_agents/full-access (instantiate mirror)", install)
	}
}
