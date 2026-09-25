//go:build integration

// T16（plan 08 Task 16 Step 5）：真 PostgreSQL 应用边界集成测试——升级
// 效果从成员对话验证新旧版调用 + 两空间各自版本互不影响。
//
// 环境契约（与 plugin_pg_integration_test.go 同一约定）：
//
//	PLUGIN_TEST_DATABASE_URL  一次性 PostgreSQL DSN。
//	                          缺失 → t.Fatal("blocked-env: ...")，绝不 Skip 通过。
//
// 复用同包基建：openPluginDB（隔离 schema + 冻结模型）、plugintest 受控远端
// （SetTools 热切目录）、newManagerLister（nonce-exclusive 真实 manager
// lister）与 plugin_agent_integration_test.go 的 agentIT* 会话组装辅助
// （RegisterMCPTools + 快照守卫 + discover/describe/call）。
package plugins_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/tools"
	internalmcp "github.com/Tencent/WeKnora/internal/modules/airesource/mcp"
	"github.com/Tencent/WeKnora/internal/modules/plugins/plugintest"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// upgradePGStack：无 OAuth 噪声的升级栈（成员场景避免个人授权面）——
// plugintest 远端（v1_tool 只读无账号工具）+ 真实 PG 仓储/服务/manager，
// 落一条已确认 v1 安装。
type upgradePGStack struct {
	tenantID       uint64
	svc            interfaces.PluginService
	pluginRepo     interfaces.PluginRepository
	mcpRepo        interfaces.MCPServiceRepository
	manager        *internalmcp.MCPManager
	remote         *plugintest.Server
	installationID string
	service        *types.MCPService
}

// newUpgradePGStack assembles the stack and lands ONE confirmed v1
// installation of the (per-stack) plugin with the no-account read tool
// v1_tool.
func newUpgradePGStack(t *testing.T, db *gorm.DB, tenantID uint64, pluginID string) *upgradePGStack {
	t.Helper()

	remote := plugintest.New()
	remote.PluginID = pluginID
	remote.Version = "1.0.0"
	remote.SetTools([]plugintest.Tool{{
		Name: "v1_tool", Description: "v1 only tool", ReadOnly: true,
		InputSchema: agentITNoArgSchema,
	}})
	remote.Start(t)

	pluginRepo := repository.NewPluginRepository(db)
	mcpRepo := repository.NewMCPServiceRepository(db)
	approvalRepo := repository.NewMCPToolApprovalRepository(db)
	manager := internalmcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	mcpSvcService := service.NewMCPServiceService(mcpRepo, manager, nil)
	approvalSvc := service.NewMCPToolApprovalService(approvalRepo, mcpRepo)
	closer := service.MCPClientCloser(func(serviceID string) { _ = manager.CloseClient(serviceID) })
	svc := service.NewPluginService(pluginRepo, mcpSvcService, mcpRepo, approvalSvc, newManagerLister(manager), closer, nil)

	ctx := context.Background()
	preview, err := svc.PreviewFromManifest(ctx, tenantID, "admin-1", remote.ManifestURL())
	require.NoError(t, err)
	confirmed, err := svc.ConfirmInstallation(ctx, tenantID, "admin-1", preview.PreviewID)
	require.NoError(t, err)
	require.NotEmpty(t, confirmed.ServiceID)
	serviceRow, err := mcpRepo.GetByID(ctx, tenantID, confirmed.ServiceID)
	require.NoError(t, err)
	require.NotNil(t, serviceRow)

	return &upgradePGStack{
		tenantID:       tenantID,
		svc:            svc,
		pluginRepo:     pluginRepo,
		mcpRepo:        mcpRepo,
		manager:        manager,
		remote:         remote,
		installationID: confirmed.InstallationID,
		service:        serviceRow,
	}
}

// memberSession assembles ONE member conversation's tool surface the way the
// Agent engine does per session: the tenant's ENABLED services + the REAL
// runtime snapshot guard (service.PluginSnapshotLookup over the PG
// repository) via RegisterMCPTools. A fresh registry per call models a NEW
// conversation after an upgrade.
func (st *upgradePGStack) memberSession(t *testing.T, memberID string) *tools.ToolRegistry {
	t.Helper()
	services, err := st.mcpRepo.ListEnabled(context.Background(), st.tenantID)
	require.NoError(t, err)
	return agentITRegister(
		agentITContext(st.tenantID, memberID), t, st.manager, &agentITGate{},
		service.PluginSnapshotLookup(st.pluginRepo), services...)
}

// listToolNames runs list_tools for the stack's materialized service.
func (st *upgradePGStack) listToolNames(t *testing.T, memberID string) []string {
	t.Helper()
	page := agentITDiscover(agentITContext(st.tenantID, memberID), t, st.memberSession(t, memberID),
		map[string]any{"mode": "list_tools", "server_id": st.service.ID})
	return agentITToolNames(page)
}

// TestUpgradeVisibleFromMemberConversation（计划 Step 5 用例 1）：成员在
// 升级前从会话调旧版 v1_tool 成功；受控服务 SetTools 切 v2（v1_tool 移除、
// v2_tool 新增）后管理员 PreviewUpgrade→AcceptUpgrade；同一成员的 NEW 会话
// （重新 RegisterMCPTools 组装）新目录只见 v2_tool、v2_tool 调用成功、
// v1_tool describe 不可用。
func TestUpgradeVisibleFromMemberConversation(t *testing.T) {
	db := openPluginDB(t)
	st := newUpgradePGStack(t, db, 1, "com.example.upgrade-visible")
	ctx := agentITContext(1, "member-1")

	// 升级前：成员会话可见且可调旧版工具。
	before := st.listToolNames(t, "member-1")
	require.Equal(t, []string{"v1_tool"}, before)
	sessionBefore := st.memberSession(t, "member-1")
	def := agentITDescribe(ctx, t, sessionBefore, st.service.ID, "v1_tool")
	call := agentITCall(ctx, t, sessionBefore, def.ToolRef, json.RawMessage(`{}`))
	require.True(t, call.Success, call.Error)
	require.Contains(t, call.Output, "ok:v1_tool:", "the member hits the OLD version before the upgrade")

	// 受控服务热切 v2 目录，版本号同步前进（同一清单地址现声明 2.0.0）。
	st.remote.SetTools([]plugintest.Tool{{
		Name: "v2_tool", Description: "v2 only tool", ReadOnly: true,
		InputSchema: agentITNoArgSchema,
	}})
	st.remote.Version = "2.0.0"

	bg := context.Background()
	preview, err := st.svc.PreviewUpgrade(bg, 1, st.installationID)
	require.NoError(t, err)
	require.Equal(t, "2.0.0", preview.Diff.CandidateVersion)
	require.Equal(t, "v2_tool", preview.Diff.AddedTools[0].Name)
	require.Equal(t, "v1_tool", preview.Diff.RemovedTools[0].Name)

	accepted, err := st.svc.AcceptUpgrade(bg, 1, "admin-1", st.installationID, preview.CandidateFingerprint)
	require.NoError(t, err)
	require.Equal(t, "2.0.0", accepted.Version)

	// 升级后：同一成员的 NEW 会话只见新版工具。
	require.Equal(t, []string{"v2_tool"}, st.listToolNames(t, "member-1"),
		"the member's NEW conversation must expose only the new version's tools")

	// v2_tool describe → call 成功（真外呼到已切换目录）。
	sessionAfter := st.memberSession(t, "member-1")
	def2 := agentITDescribe(ctx, t, sessionAfter, st.service.ID, "v2_tool")
	call2 := agentITCall(ctx, t, sessionAfter, def2.ToolRef, json.RawMessage(`{}`))
	require.True(t, call2.Success, call2.Error)
	require.Contains(t, call2.Output, "ok:v2_tool:", "the member calls the NEW version after the upgrade")

	// v1_tool describe 不可用（目录已切，快照过滤后旧工具不再存在）。
	gone := agentITDiscoverResult(ctx, t, sessionAfter, map[string]any{
		"mode": "describe", "server_id": st.service.ID, "tool_name": "v1_tool",
	})
	require.False(t, gone.Success, "the removed tool must not be describable")
	require.Contains(t, gone.Error, "tool is unavailable")
}

// TestTwoSpacesDifferentVersions（计划 Step 5 用例 2）：空间 1、2 各自安装
// 同一插件 v1；空间 1 升到 v2、空间 2 停留 v1：空间 2 成员仍调 v1 工具成功、
// 空间 1 成员只见 v2 工具——两空间各自版本互不影响。
func TestTwoSpacesDifferentVersions(t *testing.T) {
	db := openPluginDB(t)
	const pluginID = "com.example.two-spaces"
	st1 := newUpgradePGStack(t, db, 1, pluginID)
	st2 := newUpgradePGStack(t, db, 2, pluginID)

	// 空间 1 的受控服务切 v2 并接受升级（空间 2 的远端不动）。
	st1.remote.SetTools([]plugintest.Tool{{
		Name: "v2_tool", Description: "v2 only tool", ReadOnly: true,
		InputSchema: agentITNoArgSchema,
	}})
	st1.remote.Version = "2.0.0"
	bg := context.Background()
	preview, err := st1.svc.PreviewUpgrade(bg, 1, st1.installationID)
	require.NoError(t, err)
	_, err = st1.svc.AcceptUpgrade(bg, 1, "admin-1", st1.installationID, preview.CandidateFingerprint)
	require.NoError(t, err)

	// 安装行版本：空间 1 = 2.0.0，空间 2 = 1.0.0。
	inst1, err := st1.svc.GetInstallation(bg, 1, st1.installationID)
	require.NoError(t, err)
	require.Equal(t, "2.0.0", inst1.Version)
	inst2, err := st2.svc.GetInstallation(bg, 2, st2.installationID)
	require.NoError(t, err)
	require.Equal(t, "1.0.0", inst2.Version, "space 2 must stay on its own accepted version")

	// 空间 1 成员：只见 v2_tool 且可调用。
	ctx1 := agentITContext(1, "member-1")
	require.Equal(t, []string{"v2_tool"}, st1.listToolNames(t, "member-1"))
	session1 := st1.memberSession(t, "member-1")
	def1 := agentITDescribe(ctx1, t, session1, st1.service.ID, "v2_tool")
	call1 := agentITCall(ctx1, t, session1, def1.ToolRef, json.RawMessage(`{}`))
	require.True(t, call1.Success, call1.Error)

	// 空间 2 成员：仍只见 v1_tool 且可调用（空间 1 的升级不外溢）。
	ctx2 := agentITContext(2, "member-2")
	require.Equal(t, []string{"v1_tool"}, st2.listToolNames(t, "member-2"))
	session2 := st2.memberSession(t, "member-2")
	def2 := agentITDescribe(ctx2, t, session2, st2.service.ID, "v1_tool")
	call2 := agentITCall(ctx2, t, session2, def2.ToolRef, json.RawMessage(`{}`))
	require.True(t, call2.Success, call2.Error)
	require.Contains(t, call2.Output, "ok:v1_tool:", "space 2 members keep calling their accepted v1")

	// 空间 2 的目录里 v2_tool 不可用（未接受的新版能力不可见）。
	gone := agentITDiscoverResult(ctx2, t, session2, map[string]any{
		"mode": "describe", "server_id": st2.service.ID, "tool_name": "v2_tool",
	})
	require.False(t, gone.Success)
	require.Contains(t, gone.Error, "tool is unavailable")
}
