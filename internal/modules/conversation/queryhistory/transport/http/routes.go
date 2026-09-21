package httptransport

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the module's four Admin+ query-history audit routes
// on a PRE-GUARDED group (Wave 1, Task 8). Authorization is the caller's:
// like the legacy routes_query_history.go, JWT callers need Admin+ and API
// keys need full tenant access — a scoped chat key must not read other
// principals' transcripts. Task 10's bridge builds that guarded group
// (r.Group("/admin/sessions", g.Admin()) wrapped with apiKeyFullAccess())
// and hands it here; the privacy policy itself (disabled / anonymized) is
// enforced inside the handlers via AuditUseCases.CheckAccess.
//
// The relative paths are byte-exact with the legacy registration:
//
//	GET  /:session_id/snapshot
//	POST /export
//	GET  /export/:job_id/status
//	GET  /export/:job_id/download
//
// The literal "export" segment coexists with the ":session_id" wildcard
// because gin keeps one radix tree per verb and no same-position
// wildcard-name conflict exists; a second registration of any of these
// method+path pairs panics at composition time instead of shadowing.
func RegisterRoutes(group *gin.RouterGroup, h *Handler) {
	group.GET("/:session_id/snapshot", h.GetQueryHistorySnapshot)
	group.POST("/export", h.StartQueryHistoryExport)
	group.GET("/export/:job_id/status", h.GetQueryHistoryExportStatus)
	group.GET("/export/:job_id/download", h.DownloadQueryHistoryExport)
}
