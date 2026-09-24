package plugins_test

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// TestListInstallationsTenantScoped（T07）：租户 1 安装 1 个插件、租户 2
// 安装 1 个插件——各自只看到自己的（B4 发现面 + 跨空间隔离）。
func TestListInstallationsTenantScoped(t *testing.T) {
	s1 := newInstallTestStack(t, 1)
	resp1, err := s1.confirm(t, 1, s1.previewID)
	require.NoError(t, err)

	s2 := newInstallTestStack(t, 2)
	_, err = s2.confirm(t, 2, s2.previewID)
	require.NoError(t, err)

	// 租户 1：恰好 1 个（自己的）；字段含 state/drift_state/工具数。
	list1, err := s1.svc.ListInstallations(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, list1, 1)
	require.Equal(t, resp1.InstallationID, list1[0].InstallationID)
	require.Equal(t, "com.example.jira-todo", list1[0].PluginID)
	require.Equal(t, types.PluginInstallationActive, list1[0].State)
	require.Equal(t, types.PluginDriftNone, list1[0].DriftState)
	require.Equal(t, 2, list1[0].ToolCount)
	require.True(t, list1[0].RequiresPersonalAuth)

	// 租户 2：恰好 1 个（自己的），互不可见。
	list2, err := s2.svc.ListInstallations(context.Background(), 2)
	require.NoError(t, err)
	require.Len(t, list2, 1)
	require.NotEqual(t, list1[0].InstallationID, list2[0].InstallationID)

	// 空租户：空列表（非 nil，wire 形态稳定）。
	list3, err := s1.svc.ListInstallations(context.Background(), 3)
	require.NoError(t, err)
	require.Empty(t, list3)
}

// TestGetInstallationRejectsForeignTenant（T07）：租户 2 取租户 1 的
// installation id → "not found"，不泄露存在性。
func TestGetInstallationRejectsForeignTenant(t *testing.T) {
	s1 := newInstallTestStack(t, 1)
	resp1, err := s1.confirm(t, 1, s1.previewID)
	require.NoError(t, err)

	_, err = s1.svc.GetInstallation(context.Background(), 2, resp1.InstallationID)
	require.ErrorIs(t, err, service.ErrInstallationNotFound)

	// 本租户详情：工具视图含快照元数据，写工具 Enabled=false。
	detail, err := s1.svc.GetInstallation(context.Background(), 1, resp1.InstallationID)
	require.NoError(t, err)
	require.Len(t, detail.Tools, 2)
	for _, tool := range detail.Tools {
		if tool.Name == "create_issue" {
			require.NotNil(t, tool.Enabled)
			require.False(t, *tool.Enabled)
		}
	}
}

// TestUninstallInstallationHealsEmptyServiceIDAnchor（T07-OCR1-F5）：
// ConfirmInstallation 在 CreateMCPService 成功后、UpdateInstallationServiceID
// 持久化绑定之前中断（进程崩溃，或绑定更新失败且补偿级联同样失败保留安装行）
// 时，安装行 service_id 为空而物化服务行已在。卸载必须按
// mcp_services.plugin_installation_id 反查孤儿并入级联——否则删掉锚行后留下
// Enabled=true 的孤儿服务：运行时守卫查不到安装行按非插件服务处理（未接受
// 工具目录继续暴露），且孤儿占用 (tenant_id, name='plugin:<plugin_id>')
// 唯一索引阻断重装。
func TestUninstallInstallationHealsEmptyServiceIDAnchor(t *testing.T) {
	s := newInstallTestStack(t, 7)
	resp, err := s.confirm(t, 7, s.previewID)
	require.NoError(t, err)
	require.Len(t, s.mcpRepo.services, 1)
	orphanServiceID := s.mcpRepo.services[0].ID

	// 模拟中断窗口：绑定从未落库——安装行 service_id 为空，物化服务行
	// （含 plugin_installation_id 回引）已存在。
	for _, inst := range s.pluginRepo.installations {
		if inst.ID == resp.InstallationID {
			inst.ServiceID = ""
		}
	}

	require.NoError(t, s.svc.UninstallInstallation(context.Background(), 7, resp.InstallationID))

	// 锚行与孤儿服务一并清理，不滞留 Enabled=true 的死行。
	require.Empty(t, s.pluginRepo.installations)
	require.Empty(t, s.mcpRepo.services)
	require.Empty(t, s.approvalRepo.rows)
	require.Equal(t, []string{orphanServiceID}, s.pluginRepo.hardDeletedSvc)
	require.Equal(t, []string{orphanServiceID}, *s.closedClients,
		"healed cascade must still close the manager's cached client for the orphan service")

	// 唯一槽已释放：同插件可重新安装（孤儿不再阻断）。
	newPreview, err := s.svc.PreviewFromManifest(context.Background(), 7, "admin-1", s.pluginRepo.previews[0].ManifestURL)
	require.NoError(t, err)
	reinstalled, err := s.confirm(t, 7, newPreview.PreviewID)
	require.NoError(t, err)
	require.NotEqual(t, resp.InstallationID, reinstalled.InstallationID)
}

// TestConfirmInstallationVanishedPreviewReadsExpired（T07-OCR1 low）：
// MarkPreviewConsumed 零行命中后重读发现预览行已消失（DeleteExpiredPreviews
// 惰性清理只删已过期行）→ 判决必须是「已过期」，不得误报「已消费」——
// 与 reclassifyPreviewMiss 自身 R12 F17 契约 "row vanished reads as
// expired" 对齐。
func TestConfirmInstallationVanishedPreviewReadsExpired(t *testing.T) {
	s := newInstallTestStack(t, 7)
	// Step 3（重复安装判定）时模拟惰性清理把（已过期的）预览行删走：
	// Step 7 MarkPreviewConsumed 零行命中 → reclassify 重读为行消失。
	s.pluginRepo.onTenantPlugin = func() {
		for _, p := range s.pluginRepo.previews {
			p.ExpiresAt = time.Now().Add(-time.Minute)
		}
		s.pluginRepo.previews = nil
	}
	_, err := s.confirm(t, 7, s.previewID)
	require.ErrorIs(t, err, service.ErrPreviewExpired)
}

// TestGetMyConnectionStatusHealsEmptyServiceIDAnchor（T07-OCR2-F3）：
// confirm 中断窗口（安装行 service_id 空、物化服务与成员令牌已在）下，
// 连接视图不得退化为「unauthorized 且授权/撤销路径皆空」的死端——按
// mcp_services.plugin_installation_id 反查孤儿后照常给出三态与端点路径
// （与 UninstallInstallation 的 T07-OCR1-F5 自愈同款）。
func TestGetMyConnectionStatusHealsEmptyServiceIDAnchor(t *testing.T) {
	ctx := context.Background()
	tenantID := uint64(1)
	principal := types.Principal{Type: types.PrincipalWebUser, ID: "user-a"}

	inst := connectionStatusInstallation("inst-win", "", true) // 中断窗口：service_id 空
	repo := &installPreviewRepo{}
	repo.installations = []*types.PluginInstallation{inst}
	mcpRepo := &fakeInstallMCPServiceRepo{services: []*types.MCPService{{
		ID: "svc-orphan", TenantID: tenantID, Name: "plugin:com.example.p",
		PluginInstallationID: &inst.ID,
	}}}
	oauthRepo := &fakeConnectionOAuthRepo{tokens: map[string]*types.MCPOAuthToken{}}
	oauthRepo.tokens[oauthTokenKey(tenantID, principal, "svc-orphan")] = &types.MCPOAuthToken{
		AccessToken: "tok-a", ExpiresAt: time.Now().Add(time.Hour),
	}
	svc := service.NewPluginService(repo, nil, mcpRepo, nil, nil, nil, oauthRepo)

	status, err := svc.GetMyConnectionStatus(ctx, tenantID, "inst-win", principal)
	require.NoError(t, err)
	require.True(t, status.Authorized, "token found via the orphan back-reference, not the empty service_id")
	require.Equal(t, types.PluginConnectionAuthorized, status.State)
	require.Equal(t, "svc-orphan", status.ServiceID)
	require.Equal(t, "/api/v1/mcp-services/svc-orphan/oauth/authorize-url", status.AuthorizeURLPath)
	require.Equal(t, "/api/v1/mcp-services/svc-orphan/oauth/token", status.RevokePath)

	// 反查不到孤儿（物化从未发生，如崩溃早于 CreateMCPService）但装配了
	// stub 存储时：不报错、按降级视图返回（unauthorized、无路径）。
	repoNone := &installPreviewRepo{}
	repoNone.installations = []*types.PluginInstallation{
		connectionStatusInstallation("inst-none", "", true),
	}
	degradedSvc := service.NewPluginService(repoNone, nil, &fakeInstallMCPServiceRepo{}, nil, nil, nil, oauthRepo)
	degraded, err := degradedSvc.GetMyConnectionStatus(ctx, tenantID, "inst-none", principal)
	require.NoError(t, err)
	require.False(t, degraded.Authorized)
	require.Empty(t, degraded.AuthorizeURLPath)
	require.Empty(t, degraded.RevokePath)

	// nil 装配（生产经 dig 必注入）必须响亮失败，不得静默按「无孤儿」
	// 跳过——那会让连接视图退回死端、卸载删锚行后物化服务成永久孤儿
	// （与 oauthRepo == nil 的 fail-loudly 惯例一致，T07-OCR3-F2）。
	nilWired, _ := newConnectionStatusService([]*types.PluginInstallation{
		connectionStatusInstallation("inst-nilwired", "", true),
	})
	_, err = nilWired.GetMyConnectionStatus(ctx, tenantID, "inst-nilwired", principal)
	require.ErrorIs(t, err, service.ErrConnectionQueryFailed)
}

// TestManualMCPServiceUnaffectedByInstallations（T07/B9 守护）：预置一行
// 手工 mcp_services（plugin_installation_id NULL、无 approval 行）——安装
// 插件后手工服务列表语义不变，且手工工具在无 approval 行时仍视为启用
// （types/mcp.go MCPToolApproval.Enabled 的缺省语义保持）。
func TestManualMCPServiceUnaffectedByInstallations(t *testing.T) {
	s := newInstallTestStack(t, 7)

	// 预置手工服务（URL 指向白名单内的受控主机，绕开真实 DNS）。
	manualHost := previewControlledHost(t, new([]byte))
	manualURL := manualHost + "/manual-mcp"
	manual := &types.MCPService{
		ID:            "manual-svc-1",
		TenantID:      7,
		Name:          "manual-weather",
		Description:   "manual service",
		Enabled:       true,
		TransportType: types.MCPTransportHTTPStreamable,
		URL:           &manualURL,
	}
	require.NoError(t, s.mcpSvcService.CreateMCPService(context.Background(), manual))

	// 安装插件（物化第二行）。
	_, err := s.confirm(t, 7, s.previewID)
	require.NoError(t, err)

	// 手工服务行为不变：仍在列表中、字段原样、无 approval 行时工具启用。
	services, err := s.mcpSvcService.ListMCPServices(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, services, 2)
	var manualRow *types.MCPService
	for _, svc := range services {
		if svc.ID == "manual-svc-1" {
			manualRow = svc
		}
	}
	require.NotNil(t, manualRow, "manual service must survive a plugin install")
	require.Nil(t, manualRow.PluginInstallationID)
	require.Equal(t, "manual-weather", manualRow.Name)
	require.True(t, manualRow.Enabled)
	require.Equal(t, manualURL, *manualRow.URL)

	enabled, err := s.approvalSvc.IsEnabled(context.Background(), 7, "manual-svc-1", "any_manual_tool")
	require.NoError(t, err)
	require.True(t, enabled, "manual tools with no approval row stay enabled (default semantics)")
}
