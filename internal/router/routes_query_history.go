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
// by the handler via the export service's CheckAccess (snapshot: inside the
// session service), not here.
func RegisterQueryHistoryAdminRoutes(r *gin.RouterGroup, handler *session.Handler, g *rbacGuards) {
	admin := g.apiKeyGroup(r.Group("/admin/sessions", g.Admin()), apiKeyFullAccess())
	{
		admin.GET("/:session_id/snapshot", handler.GetQueryHistorySnapshot)
		// Async CSV export (SP13 Task 4): three-stage submit / poll /
		// download. The literal "export" segment coexists with the
		// ":session_id" wildcard because gin keeps one radix tree per verb
		// and no same-position wildcard-name conflict exists.
		admin.POST("/export", handler.StartQueryHistoryExport)
		admin.GET("/export/:job_id/status", handler.GetQueryHistoryExportStatus)
		admin.GET("/export/:job_id/download", handler.DownloadQueryHistoryExport)
	}
}
