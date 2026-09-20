package router

import (
	"net/http"
	"testing"

	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// TestSkillMarketRoutesRegisterBesideSkillAndExpertRoutes pins the M4 Task 3
// coexistence contract: the market subtree registers on the SAME v1 tree as
// the existing skills catalog, agents and experts routes without a gin
// wildcard panic — /skills/market sits beside /skills/catalog, and
// /experts/market/:slug beside the existing /experts/:id wildcard — and
// declares the API-key policies the v1 gate asserts at startup
// (assertAPIKeyPoliciesMatchRoutes): reads full-access only (catalog-read
// precedent), the skill install mirroring the catalog install (full-access),
// and the skillset install carrying manage_agents like the expert
// instantiate route.
func TestSkillMarketRoutesRegisterBesideSkillAndExpertRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	g := &rbacGuards{}
	r := gin.New()
	v1 := r.Group("/api/v1")

	RegisterCustomAgentRoutes(v1, &handler.CustomAgentHandler{}, g)
	RegisterPersonaRoutes(v1, &handler.PersonaHandler{}, g)
	RegisterExpertRoutes(v1, &handler.ExpertHandler{}, g)
	RegisterSkillRoutes(v1, &handler.SkillHandler{}, g)
	RegisterSkillMarketRoutes(v1, &handler.SkillMarketHandler{}, g)

	registered := map[string]bool{}
	for _, ri := range r.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}

	// The market routes themselves.
	for _, want := range []string{
		http.MethodGet + " /api/v1/skills/market/search",
		http.MethodGet + " /api/v1/skills/market/rankings/:kind",
		http.MethodPost + " /api/v1/skills/market/install",
		http.MethodGet + " /api/v1/experts/market",
		http.MethodGet + " /api/v1/experts/market/:slug",
		http.MethodPost + " /api/v1/experts/market/:slug/install",
	} {
		if !registered[want] {
			t.Fatalf("route %s not registered", want)
		}
	}

	// The pre-existing neighbours survive on the same tree.
	for _, want := range []string{
		http.MethodGet + " /api/v1/skills/catalog",
		http.MethodPost + " /api/v1/skills/catalog/:id/install",
		http.MethodGet + " /api/v1/experts/:id",
		http.MethodPost + " /api/v1/experts/:id/instantiate",
		http.MethodGet + " /api/v1/skills",
	} {
		if !registered[want] {
			t.Fatalf("pre-existing route %s vanished from the shared tree", want)
		}
	}

	// Policy pins: reads are full-access only (Viewer+ JWTs; scoped keys
	// stay default-denied), the skill install mirrors the catalog install,
	// and the skillset install carries manage_agents (expert-instantiate
	// precedent).
	search := mustLookupAPIKeyPolicy(t, g, http.MethodGet, "/api/v1/skills/market/search")
	if !search.RequireFullAccess || len(search.Capabilities) != 0 {
		t.Fatalf("GET /skills/market/search policy = %#v, want full-access only", search)
	}
	rankings := mustLookupAPIKeyPolicy(t, g, http.MethodGet, "/api/v1/skills/market/rankings/:kind")
	if !rankings.RequireFullAccess || len(rankings.Capabilities) != 0 {
		t.Fatalf("GET /skills/market/rankings/:kind policy = %#v, want full-access only", rankings)
	}
	installSkill := mustLookupAPIKeyPolicy(t, g, http.MethodPost, "/api/v1/skills/market/install")
	if !installSkill.RequireFullAccess || len(installSkill.Capabilities) != 0 {
		t.Fatalf("POST /skills/market/install policy = %#v, want full-access only (catalog-install mirror)", installSkill)
	}
	listSets := mustLookupAPIKeyPolicy(t, g, http.MethodGet, "/api/v1/experts/market")
	if !listSets.RequireFullAccess || len(listSets.Capabilities) != 0 {
		t.Fatalf("GET /experts/market policy = %#v, want full-access only", listSets)
	}
	getSet := mustLookupAPIKeyPolicy(t, g, http.MethodGet, "/api/v1/experts/market/:slug")
	if !getSet.RequireFullAccess || len(getSet.Capabilities) != 0 {
		t.Fatalf("GET /experts/market/:slug policy = %#v, want full-access only", getSet)
	}
	installSet := mustLookupAPIKeyPolicy(t, g, http.MethodPost, "/api/v1/experts/market/:slug/install")
	if !policyHasCapability(installSet, types.APIKeyCapabilityManageAgents) {
		t.Fatalf("POST /experts/market/:slug/install policy = %#v, want manage_agents capability", installSet.Capabilities)
	}
}
