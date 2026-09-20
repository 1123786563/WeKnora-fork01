package router

import (
	"net/http"
	"testing"

	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// TestTenantSkillMarketRoutesRegisterBesideCatalogRoutes pins the M4 Task 4
// coexistence contract: the tenant-internal market registers on the SAME v1
// tree as the existing skills catalog routes without a gin wildcard panic —
// /skills/catalog/:id/publish sits beside /skills/catalog/:id,
// /skills/catalog/:id/install and /skills/catalog/:id/files (same :id
// wildcard at the same depth), and /market/tenant/skills/:catalogId/install
// adds the first /market subtree — and declares the API-key policies the v1
// gate asserts at startup: publish/unpublish mirror the catalog-write guard
// (Admin+, full-access keys), the listing mirrors the catalog read (Viewer+,
// full-access only) and the install mirrors POST /skills/catalog/:id/install
// (Admin+, full-access — the route bakes code into sandbox images).
func TestTenantSkillMarketRoutesRegisterBesideCatalogRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	g := &rbacGuards{}
	r := gin.New()
	v1 := r.Group("/api/v1")

	RegisterSkillRoutes(v1, &handler.SkillHandler{}, g)
	RegisterSkillMarketRoutes(v1, &handler.SkillMarketHandler{}, g)
	RegisterTenantSkillMarketRoutes(v1, &handler.TenantSkillMarketHandler{}, g)

	registered := map[string]bool{}
	for _, ri := range r.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}

	// The tenant-market routes themselves.
	for _, want := range []string{
		http.MethodPost + " /api/v1/skills/catalog/:id/publish",
		http.MethodDelete + " /api/v1/skills/catalog/:id/publish",
		http.MethodGet + " /api/v1/market/tenant/skills",
		http.MethodPost + " /api/v1/market/tenant/skills/:catalogId/install",
	} {
		if !registered[want] {
			t.Fatalf("route %s not registered", want)
		}
	}

	// The pre-existing neighbours survive on the same tree (the :id wildcard
	// segment is shared, not conflicting).
	for _, want := range []string{
		http.MethodGet + " /api/v1/skills/catalog",
		http.MethodPost + " /api/v1/skills/catalog",
		http.MethodPost + " /api/v1/skills/catalog/:id/install",
		http.MethodGet + " /api/v1/skills/catalog/:id/files",
		http.MethodGet + " /api/v1/skills/catalog/:id/files/content",
		http.MethodDelete + " /api/v1/skills/catalog/:id",
		http.MethodGet + " /api/v1/skills/market/search",
		http.MethodPost + " /api/v1/skills/market/install",
	} {
		if !registered[want] {
			t.Fatalf("pre-existing route %s vanished from the shared tree", want)
		}
	}

	// Policy pins: publish/unpublish carry the catalog-write policy
	// (full-access only), the listing the catalog-read policy, and the
	// install the catalog-install policy it composes with.
	publish := mustLookupAPIKeyPolicy(t, g, http.MethodPost, "/api/v1/skills/catalog/:id/publish")
	if !publish.RequireFullAccess || len(publish.Capabilities) != 0 {
		t.Fatalf("POST /skills/catalog/:id/publish policy = %#v, want full-access only (catalog-write mirror)", publish)
	}
	unpublish := mustLookupAPIKeyPolicy(t, g, http.MethodDelete, "/api/v1/skills/catalog/:id/publish")
	if !unpublish.RequireFullAccess || len(unpublish.Capabilities) != 0 {
		t.Fatalf("DELETE /skills/catalog/:id/publish policy = %#v, want full-access only (catalog-write mirror)", unpublish)
	}
	list := mustLookupAPIKeyPolicy(t, g, http.MethodGet, "/api/v1/market/tenant/skills")
	if !list.RequireFullAccess || len(list.Capabilities) != 0 {
		t.Fatalf("GET /market/tenant/skills policy = %#v, want full-access only (catalog-read mirror)", list)
	}
	install := mustLookupAPIKeyPolicy(t, g, http.MethodPost, "/api/v1/market/tenant/skills/:catalogId/install")
	if !install.RequireFullAccess || len(install.Capabilities) != 0 {
		t.Fatalf("POST /market/tenant/skills/:catalogId/install policy = %#v, want full-access only (catalog-install mirror)", install)
	}
}
