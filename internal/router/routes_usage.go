package router

import (
	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/handler"
)

// RegisterUsageRoutes 注册 SP12 用量聚合路由。
//
// The /usage/me surface is a per-user read exactly like the caller's own
// sessions: the handler resolves identity from the request context, so
// Viewer+ gating here only keeps non-members out once RBAC is on. A scoped
// API key needs the chat capability (or full tenant access) — same tier as
// reading one's own messages.
//
// The two /admin/usage surfaces expose tenant-wide rows with no per-user
// ownership, so they gate at Admin+ (JWT roles) and full tenant access for
// API keys — a scoped chat key must not enumerate or export other users'
// consumption.
func RegisterUsageRoutes(r *gin.RouterGroup, handler *handler.UsageHandler, g *rbacGuards) {
	me := g.apiKeyGroup(r.Group("/usage", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
	{
		me.GET("/me", handler.MyUsage)
	}

	admin := g.apiKeyGroup(r.Group("/admin/usage", g.Admin()), apiKeyFullAccess())
	{
		admin.GET("/by-user", handler.AllUsers)
		admin.GET("/export", handler.Export)
	}
}
