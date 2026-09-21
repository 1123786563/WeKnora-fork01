package httptransport

// Route compatibility tests for the Query History module's HTTP transport
// (Wave 1, Task 8): RegisterRoutes must mount the four Admin+ audit routes
// with byte-exact relative paths on a pre-guarded group, and a second
// registration of the same surface must fail loudly (gin panics on duplicate
// method+path registration) rather than silently shadowing handlers. The RBAC
// group itself (Admin+ JWT gate, full-access API-key policy) is Task 10's
// bridge — the caller owns authorization, exactly like the legacy
// routes_query_history.go owned it.

import (
	"net/http"
	"sort"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// routeSpec renders one registered route the way gin reports it.
func routeSpecs(t *testing.T, r *gin.Engine) []string {
	t.Helper()
	specs := make([]string, 0, len(r.Routes()))
	for _, route := range r.Routes() {
		specs = append(specs, route.Method+" "+route.Path)
	}
	sort.Strings(specs)
	return specs
}

func TestRegisterRoutesMountsExactLegacyPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(&fakeAudit{}, &fakeExports{})

	RegisterRoutes(r.Group("/api/v1/admin/sessions"), h)

	// The four legacy Admin+ audit routes, byte-exact: the snapshot, the
	// export submit, and the poll / download pair. The literal "export"
	// segment coexists with the ":session_id" wildcard because gin keeps one
	// radix tree per verb — registering both successfully is the proof, the
	// same executable argument the legacy router test makes.
	want := []string{
		http.MethodGet + " /api/v1/admin/sessions/:session_id/snapshot",
		http.MethodPost + " /api/v1/admin/sessions/export",
		http.MethodGet + " /api/v1/admin/sessions/export/:job_id/status",
		http.MethodGet + " /api/v1/admin/sessions/export/:job_id/download",
	}
	sort.Strings(want)
	require.Equal(t, want, routeSpecs(t, r))
}

func TestRegisterRoutesPanicsOnDuplicateRegistration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(&fakeAudit{}, &fakeExports{})

	RegisterRoutes(r.Group("/api/v1/admin/sessions"), h)

	// A second registration of the same surface must blow up at composition
	// time (gin panics on a duplicate method+path in one radix tree) instead
	// of quietly shadowing or stacking handlers: a route that silently moved
	// or doubled is an outage no test would otherwise catch.
	require.Panics(t, func() {
		RegisterRoutes(r.Group("/api/v1/admin/sessions"), h)
	})
}
