package router

import (
	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/handler/session"
)

// RegisterQueryHistoryAdminRoutes registers the Admin+ query-history audit
// surface (SP13). Like /admin/usage these endpoints expose tenant-wide rows
// with no per-user ownership: JWT callers need Admin+ and API keys need full
// tenant access — a scoped chat key must not read other principals'
// transcripts. The privacy policy itself (disabled / anonymized) is enforced
// inside the service, not here.
//
// Task 4's async export endpoints will join this group.
func RegisterQueryHistoryAdminRoutes(r *gin.RouterGroup, handler *session.Handler, g *rbacGuards) {
	admin := g.apiKeyGroup(r.Group("/admin/sessions", g.Admin()), apiKeyFullAccess())
	{
		admin.GET("/:session_id/snapshot", handler.GetQueryHistorySnapshot)
	}
}
