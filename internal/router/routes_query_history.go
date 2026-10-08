package router

import (
	"net/http"
	"path"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/conversation/queryhistory"
)

// queryHistoryAdminRoutes is the module's route table, mirrored here only to
// declare the API-key policies. The startup self-check
// (assertAPIKeyPoliciesMatchRoutes) fails the boot if this table ever drifts
// from what the module actually mounts, and the router tests pin both sides.
var queryHistoryAdminRoutes = []struct{ method, rel string }{
	{http.MethodGet, "/:session_id/snapshot"},
	{http.MethodPost, "/export"},
	{http.MethodGet, "/export/:job_id/status"},
	{http.MethodGet, "/export/:job_id/download"},
}

// RegisterQueryHistoryAdminRoutes registers the Admin+ query-history audit
// surface (SP13), served since Wave 1 Task 10 by the conversation module's
// Query History feature instead of the legacy session handler. Like
// /admin/usage these endpoints expose tenant-wide rows with no per-user
// ownership: JWT callers need Admin+ and API keys need full tenant access —
// a scoped chat key must not read other principals' transcripts. The privacy
// policy itself (disabled / anonymized) is enforced inside the module's
// handlers via the audit service's CheckAccess, not here.
func RegisterQueryHistoryAdminRoutes(r *gin.RouterGroup, module *queryhistory.Module, g *rbacGuards) {
	// The exact legacy guarded group: the Admin role middleware rides the
	// group itself, and apiKeyFullAccess() is the API-key policy declared
	// for every route below. The module mounts its four byte-identical
	// routes onto this group; the literal "export" segment coexists with
	// the ":session_id" wildcard because gin keeps one radix tree per verb
	// and no same-position wildcard-name conflict exists.
	admin := r.Group("/admin/sessions", g.Admin())
	module.RegisterRoutes(admin)
	// Every route the module mounted declares the group's API-key policy,
	// exactly as the legacy admin.GET/admin.POST calls did: the /api/v1
	// gate denies undeclared routes by default, so these declarations are
	// load-bearing for API-key principals.
	authorizer := g.ensureAPIKeyAuthorizer()
	for _, route := range queryHistoryAdminRoutes {
		authorizer.Register(route.method, path.Join(admin.BasePath(), route.rel), apiKeyFullAccess())
	}
}
