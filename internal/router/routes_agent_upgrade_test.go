package router

// Agent-upgrade route tests (T31 #61). This file hosts the governance-floor
// assertions (Task 5) and the full-lifecycle HTTP e2e (Task 6).

import (
	"net/http"
	"testing"

	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAgentUpgradeRoutesRequireAdminAndFullAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// 升级建议审阅是治理写面（spec §9 管理员比较/接受/驳回）：与 Adoption
	// 治理路由同款 Admin+ full-access 地板（spec §13 adopt_agent /
	// configure_variant）。
	g := &rbacGuards{}
	v1 := gin.New().Group("/api/v1")
	RegisterAgentUpgradeRoutes(v1, handler.NewAgentUpgradeHandler(nil), g)

	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals"},
		{http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals/:id"},
		{http.MethodPost, "/api/v1/marketplace/tenant/upgrade-proposals/:id/accept"},
		{http.MethodPost, "/api/v1/marketplace/tenant/upgrade-proposals/:id/dismiss"},
	}
	for _, route := range routes {
		policy := mustLookupAPIKeyPolicy(t, g, route.method, route.path)
		require.Truef(t, policy.RequireFullAccess, "%s %s 必须要求 full-access", route.method, route.path)
		require.Falsef(t, policyHasCapability(policy, types.APIKeyCapabilityIngest),
			"%s %s 不得被无关能力放行", route.method, route.path)
	}

	// nil handler fail closed：不挂载任何路由。
	bare := gin.New()
	RegisterAgentUpgradeRoutes(bare.Group("/api/v1"), nil, &rbacGuards{})
	require.Empty(t, bare.Routes())
}
