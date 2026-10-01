package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/handler"
)

// RegisterPublicMarketplaceRoutes mounts the Public Marketplace workflow
// (T30 #60) beside the tenant release/adoption routes.
//
// Tenant-facing surface (spec §2「平台允许的 Tenant」= 本部署全部已认证
// 租户，首版无按租户屏蔽清单):
//   - GET  /marketplace/public/catalog            Viewer+（发现）
//   - GET  /marketplace/public/listings/:id       Viewer+（采用前检视）
//   - POST /marketplace/public/listings/:id/adopt Admin+（spec §13 adopt_agent）
//   - POST /marketplace/public/release-submissions Admin+（Verified Publisher 门槛在服务层）
//   - GET  /marketplace/public/release-submissions Admin+（仅本租户提交）
//
// 平台面（spec §13 review_public_release / Verified Publisher 治理）全部
// SystemAdmin，且不对 API key 声明策略——沿 routes_auth_tenant.go 中
// promote/revoke 的先例：平台 API key 能力面属后续工作，默认拒绝。
func RegisterPublicMarketplaceRoutes(r *gin.RouterGroup, publicHandler *handler.PublicMarketplaceHandler, g *rbacGuards) {
	if publicHandler == nil {
		return
	}
	admin := apiKeyFullAccess()
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/public/release-submissions", admin, g.Admin(), publicHandler.SubmitPublicRelease)
	g.apiKeyRoute(r, http.MethodGet, "/marketplace/public/release-submissions", admin, g.Admin(), publicHandler.ListPublicSubmissions)
	g.apiKeyRoute(r, http.MethodGet, "/marketplace/public/release-submissions/review-queue", admin, g.SystemAdmin(), publicHandler.ListPublicReviewQueue)
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/public/release-submissions/:id/review", admin, g.SystemAdmin(), publicHandler.ReviewPublicSubmission)
	g.apiKeyRoute(r, http.MethodGet, "/marketplace/public/catalog", admin, g.Viewer(), publicHandler.ListPublicCatalog)
	g.apiKeyRoute(r, http.MethodGet, "/marketplace/public/listings/:id", admin, g.Viewer(), publicHandler.GetPublicListing)
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/public/listings/:id/adopt", admin, g.Admin(), publicHandler.AdoptPublicListing)
	// T35 #65 Task 4：平台结构化 Evaluation 授权面——仅 SystemAdmin，且刻意
	// 不经 apiKeyRoute 声明策略：API-key 门对未声明路由默认拒绝，平台 API
	// key 能力面属后续工作（沿 verified-publishers 治理路由先例）。
	r.POST("/marketplace/public/evaluations", g.SystemAdmin(), publicHandler.RecordPublicEvaluation)
	r.GET("/marketplace/public/verified-publishers", g.SystemAdmin(), publicHandler.ListVerifiedPublishers)
	r.POST("/marketplace/public/verified-publishers", g.SystemAdmin(), publicHandler.VerifyPublisher)
	r.DELETE("/marketplace/public/verified-publishers/:tenant_id", g.SystemAdmin(), publicHandler.RevokePublisher)
}
