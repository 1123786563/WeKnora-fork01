package router

import (
	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/handler"
)

// RegisterAnalyticsRoutes 注册控制台分析路由（SP11）。
//
// Analytics is a tenant-admin surface: the aggregate rows are tenant-wide and
// carry no per-user ownership, so the group gates at Admin+ (JWT roles) and,
// for API keys, requires full tenant access — a scoped key (chat / ingest /
// retrieve ...) must not be able to read usage dashboards.
func RegisterAnalyticsRoutes(r *gin.RouterGroup, handler *handler.AnalyticsHandler, g *rbacGuards) {
	analytics := g.apiKeyGroup(r.Group("/analytics", g.Admin()), apiKeyFullAccess())
	{
		analytics.GET("/queries", handler.QueryTrend)
		analytics.GET("/users", handler.ActiveUsers)
		analytics.GET("/channels", handler.ChannelSessions)
		analytics.GET("/agents/:agent_id", handler.AgentMessages)
	}
}
