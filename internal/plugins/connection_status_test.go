// T11（plan 05 Task 11 Step 1）：GetMyConnectionStatus 的 fake 层契约测试。
//
// 三态判定（authorized / expired / unauthorized）+ 无账号插件恒 authorized +
// AuthorizeURLPath/RevokePath 映射到物化 service_id 的既有 OAuth 端点 +
// 跨租户 not found。GAP-4：身份映射复用既有 per-principal MCP OAuth
// （oauthRepo.GetTokenForPrincipal），不新建 token 表。
package plugins_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// fakeConnectionOAuthRepo 是面向 connections/me 的最小内存 OAuth 仓储：
// 只实现 GetTokenForPrincipal（本视图的唯一读取面）；嵌入 nil 接口满足
// 其余方法集（本测试绝不调用它们）。
type fakeConnectionOAuthRepo struct {
	interfaces.MCPOAuthRepository
	tokens map[string]*types.MCPOAuthToken
}

func oauthTokenKey(tenantID uint64, principal types.Principal, serviceID string) string {
	return fmt.Sprintf("%d|%s|%s", tenantID, principal.Normalize().StorageID(), serviceID)
}

func (r *fakeConnectionOAuthRepo) GetTokenForPrincipal(
	_ context.Context, tenantID uint64, principal types.Principal, serviceID string,
) (*types.MCPOAuthToken, error) {
	return r.tokens[oauthTokenKey(tenantID, principal, serviceID)], nil
}

// newConnectionStatusService 组装仅含安装存储 + OAuth 仓储的服务实例：
// GetMyConnectionStatus 不触碰物化服务行（ServiceID 直接取自安装行），
// 其余依赖传 nil。安装存储复用 installPreviewRepo（fakePluginInstallRepo
// 的完整接口形态）。
func newConnectionStatusService(
	installations []*types.PluginInstallation,
) (interfaces.PluginService, *fakeConnectionOAuthRepo) {
	repo := &installPreviewRepo{}
	repo.installations = installations
	oauthRepo := &fakeConnectionOAuthRepo{tokens: map[string]*types.MCPOAuthToken{}}
	return service.NewPluginService(repo, nil, nil, nil, nil, nil, oauthRepo), oauthRepo
}

func connectionStatusInstallation(id, serviceID string, authTool bool) *types.PluginInstallation {
	snapshot := []types.PluginToolSnapshot{
		{Name: "health", Description: "no account", ReadOnly: true},
	}
	if authTool {
		snapshot = append(snapshot, types.PluginToolSnapshot{
			Name: "search", Description: "personal", ReadOnly: true,
			RequiresPersonalAuth: true, Scopes: []string{"read:jira"},
		})
	}
	return &types.PluginInstallation{
		ID: id, TenantID: 1, PluginID: "com.example.p", Name: "P",
		ServiceID: serviceID, State: types.PluginInstallationActive,
		ToolsSnapshot: snapshot,
	}
}

func TestGetMyConnectionStatusStates(t *testing.T) {
	ctx := context.Background()
	tenantID := uint64(1)
	principal := types.Principal{Type: types.PrincipalWebUser, ID: "user-a"}

	svc, oauthRepo := newConnectionStatusService([]*types.PluginInstallation{
		connectionStatusInstallation("inst-auth", "svc-auth", true),
		connectionStatusInstallation("inst-noauth", "svc-noauth", false),
	})

	// a) 无 token → unauthorized，路径指向物化 service_id 的既有端点。
	status, err := svc.GetMyConnectionStatus(ctx, tenantID, "inst-auth", principal)
	require.NoError(t, err)
	require.False(t, status.Authorized)
	require.Equal(t, types.PluginConnectionUnauthorized, status.State)
	require.True(t, status.RequiresPersonalAuth)
	require.Equal(t, "/api/v1/mcp-services/svc-auth/oauth/authorize-url", status.AuthorizeURLPath)
	require.Equal(t, "/api/v1/mcp-services/svc-auth/oauth/token", status.RevokePath)
	require.Equal(t, []string{"search"}, status.RequiresAuthTools)

	// b) 有效 token（ExpiresAt 未来）→ authorized。
	oauthRepo.tokens[oauthTokenKey(tenantID, principal, "svc-auth")] = &types.MCPOAuthToken{
		AccessToken: "tok-a", ExpiresAt: time.Now().Add(time.Hour),
	}
	status, err = svc.GetMyConnectionStatus(ctx, tenantID, "inst-auth", principal)
	require.NoError(t, err)
	require.True(t, status.Authorized)
	require.Equal(t, types.PluginConnectionAuthorized, status.State)

	// c) token 过期且无 RefreshToken → expired（引导重授权）。
	oauthRepo.tokens[oauthTokenKey(tenantID, principal, "svc-auth")] = &types.MCPOAuthToken{
		AccessToken: "tok-a", ExpiresAt: time.Now().Add(-time.Minute),
	}
	status, err = svc.GetMyConnectionStatus(ctx, tenantID, "inst-auth", principal)
	require.NoError(t, err)
	require.False(t, status.Authorized)
	require.Equal(t, types.PluginConnectionExpired, status.State)

	// c2) token 过期但有 RefreshToken → 仍 authorized：运行时会用既有同意
	// 自动续期（mcp oauthRuntime.ensureFresh），成员无需重新授权。
	oauthRepo.tokens[oauthTokenKey(tenantID, principal, "svc-auth")] = &types.MCPOAuthToken{
		AccessToken: "tok-a", RefreshToken: "rt-a", ExpiresAt: time.Now().Add(-time.Minute),
	}
	status, err = svc.GetMyConnectionStatus(ctx, tenantID, "inst-auth", principal)
	require.NoError(t, err)
	require.True(t, status.Authorized)
	require.Equal(t, types.PluginConnectionAuthorized, status.State)

	// d) 清单无 personal_oauth → RequiresPersonalAuth=false，State 恒
	// authorized（无账号插件），路径与工具清单为空。
	status, err = svc.GetMyConnectionStatus(ctx, tenantID, "inst-noauth", principal)
	require.NoError(t, err)
	require.True(t, status.Authorized)
	require.Equal(t, types.PluginConnectionAuthorized, status.State)
	require.False(t, status.RequiresPersonalAuth)
	require.Empty(t, status.AuthorizeURLPath)
	require.Empty(t, status.RevokePath)
	require.Empty(t, status.RequiresAuthTools)
}

func TestGetMyConnectionStatusPerPrincipal(t *testing.T) {
	// A 的授权不改变 B 的视图：principal 是状态查询的入参（隔离由
	// (tenant, principal, service) 三元组存储键保证）。
	ctx := context.Background()
	tenantID := uint64(1)
	a := types.Principal{Type: types.PrincipalWebUser, ID: "user-a"}
	b := types.Principal{Type: types.PrincipalWebUser, ID: "user-b"}

	svc, oauthRepo := newConnectionStatusService([]*types.PluginInstallation{
		connectionStatusInstallation("inst-auth", "svc-auth", true),
	})
	oauthRepo.tokens[oauthTokenKey(tenantID, a, "svc-auth")] = &types.MCPOAuthToken{
		AccessToken: "tok-a", ExpiresAt: time.Now().Add(time.Hour),
	}

	statusA, err := svc.GetMyConnectionStatus(ctx, tenantID, "inst-auth", a)
	require.NoError(t, err)
	require.Equal(t, types.PluginConnectionAuthorized, statusA.State)

	statusB, err := svc.GetMyConnectionStatus(ctx, tenantID, "inst-auth", b)
	require.NoError(t, err)
	require.Equal(t, types.PluginConnectionUnauthorized, statusB.State)
}

func TestGetMyConnectionStatusTenantScoped(t *testing.T) {
	ctx := context.Background()
	svc, _ := newConnectionStatusService([]*types.PluginInstallation{
		connectionStatusInstallation("inst-t1", "svc-t1", true),
	})
	// 租户 2 成员查租户 1 安装 → not found（不泄漏存在性）。
	_, err := svc.GetMyConnectionStatus(ctx, 2, "inst-t1",
		types.Principal{Type: types.PrincipalWebUser, ID: "user-x"})
	require.ErrorIs(t, err, service.ErrInstallationNotFound)
}
