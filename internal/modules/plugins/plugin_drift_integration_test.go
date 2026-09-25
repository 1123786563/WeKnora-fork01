//go:build integration

// T17（plan 09 Task 17 Step 5）：真 PostgreSQL 应用边界集成测试——漂移
// 从成员对话观察阻断、漂移只影响单安装、手工服务不受治理。
//
// 环境契约（与 plugin_pg_integration_test.go 同一约定）：
//
//	PLUGIN_TEST_DATABASE_URL  一次性 PostgreSQL DSN。
//	                          缺失 → t.Fatal("blocked-env: ...")，绝不 Skip 通过。
//
// 复用同包基建：openPluginDB（隔离 schema + 冻结模型）、plugintest 受控远端
// （SetTools 热切目录/schema）、newManagerLister（nonce-exclusive 真实
// manager lister）与 plugin_agent_integration_test.go 的 agentIT* 会话组装
// 辅助（RegisterMCPTools + 快照守卫 + discover/describe/call）。
//
// 成员可观察文案口径（T09 已定型，tools 层不在本任务所有权）：漂移时
// loadPluginDirectory 返回 ErrPluginDrift（"plugin capability drift detected;
// administrator review required: tool ..."），catalog 把它折叠为目录级
// 泛化状态文案（"MCP server %q is error; retry discovery ..."）呈现给成员；
// ErrPluginDrift 原文是服务端审计日志（mcp_tool.go 检测点 Errorf）与
// CheckDrift/GetDrift 管理面。本文件从成员对话断言「调用被阻」（fail-closed
// + 状态文案），从管理面断言「提示复审」（drift_state/detail/复审闭环）。
package plugins_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/approval"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/tools"
	internalmcp "github.com/Tencent/WeKnora/internal/modules/airesource/mcp"
	"github.com/Tencent/WeKnora/internal/modules/plugins/plugintest"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// driftPGGate 读真实 PG 策略行的审批 gate（与 agentITGate 的恒真替身相对）：
// IsEnabled 委托 MCPToolApprovalService（repo.IsEnabled 的缺行=启用语义），
// 审批等待自动批准——漂移场景断言的是启停策略面，不是审批流。resolve 后
// 「新增 t2 保守关闭 → 成员目录不可见」必须经真实策略行过滤才可观察。
type driftPGGate struct {
	approvalSvc interfaces.MCPToolApprovalService
}

func (g *driftPGGate) IsEnabled(ctx context.Context, tenantID uint64, serviceID, toolName string) (bool, error) {
	return g.approvalSvc.IsEnabled(ctx, tenantID, serviceID, toolName)
}

func (*driftPGGate) NeedsApproval(context.Context, uint64, string, string) bool { return true }

func (*driftPGGate) RequestAndWait(
	_ context.Context, _ approval.PendingRequest,
) (approval.Decision, error) {
	return approval.Decision{Approved: true}, nil
}

// driftPG 工具 schema：v1 无参（与 agentITNoArgSchema 同形）；v2 是它的
// schema 变体（新增可选属性——仍无 required，空参调用依旧合法，resolve 后
// 成员按新 schema 调用成功的断言不需要参数）。
const (
	driftPGT1SchemaV1 = `{"type":"object","properties":{},"additionalProperties":false}`
	driftPGT1SchemaV2 = `{"type":"object","properties":{"note":{"type":"string"}},"additionalProperties":false}`
)

// driftPGStack：plugintest 单远端（t1 只读无账号工具）+ 真实 PG 仓储/服务/
// manager，落一条已确认 v1 安装。与 upgradePGStack 同构，命名独立以承载
// 漂移场景的成员会话辅助。
type driftPGStack struct {
	tenantID       uint64
	svc            interfaces.PluginService
	pluginRepo     interfaces.PluginRepository
	mcpRepo        interfaces.MCPServiceRepository
	manager        *internalmcp.MCPManager
	remote         *plugintest.Server
	installationID string
	service        *types.MCPService
	// gate 读真实 PG 策略行（driftPGGate）——resolve 后新增工具的保守
	// Enabled=false 必须经真实行过滤才可从成员目录观察。
	gate *driftPGGate
	// guard 是生产形态的运行时守卫 provider（含 T17 best-effort 漂移
	// 置位）——成员会话经它组装，手工服务断言经它直接查询。
	guard tools.PluginSnapshotProvider
}

// newDriftPGStack assembles the stack and lands ONE confirmed v1 installation
// of the (per-stack) plugin with the no-account read tool t1 at schema V1.
func newDriftPGStack(t *testing.T, db *gorm.DB, tenantID uint64, pluginID string) *driftPGStack {
	t.Helper()

	remote := plugintest.New()
	remote.PluginID = pluginID
	remote.Version = "1.0.0"
	remote.SetTools([]plugintest.Tool{{
		Name: "t1", Description: "v1 tool", ReadOnly: true,
		InputSchema: driftPGT1SchemaV1,
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

	return &driftPGStack{
		tenantID:       tenantID,
		svc:            svc,
		pluginRepo:     pluginRepo,
		mcpRepo:        mcpRepo,
		manager:        manager,
		remote:         remote,
		installationID: confirmed.InstallationID,
		service:        serviceRow,
		gate:           &driftPGGate{approvalSvc: approvalSvc},
		// 生产形态守卫：含 best-effort 漂移置位（agent_service 的装配
		// 同款——manager 在场时选 WithDriftMarking 变体）。
		guard: service.PluginSnapshotLookupWithDriftMarking(pluginRepo, newManagerLister(manager)),
	}
}

// driftPGRegister assembles a session registry over the approval.MCPApproval
// INTERFACE（agentITRegister 收具体 *agentITGate——共享基建的形状；本文件
// 的 driftPGGate 同样实现该接口，走自己的同构注册辅助）。
func driftPGRegister(
	ctx context.Context,
	t *testing.T,
	manager *internalmcp.MCPManager,
	gate approval.MCPApproval,
	guard tools.PluginSnapshotProvider,
	services ...*types.MCPService,
) *tools.ToolRegistry {
	t.Helper()
	registry := tools.NewToolRegistry()
	_, err := tools.RegisterMCPTools(ctx, registry, services, manager, gate, 0, nil, nil, guard)
	require.NoError(t, err)
	return registry
}

// memberSession assembles ONE member conversation's tool surface the way the
// Agent engine does per session (fresh registry = a NEW conversation).
func (st *driftPGStack) memberSession(t *testing.T, memberID string) *tools.ToolRegistry {
	t.Helper()
	services, err := st.mcpRepo.ListEnabled(context.Background(), st.tenantID)
	require.NoError(t, err)
	return driftPGRegister(
		agentITContext(st.tenantID, memberID), t, st.manager, st.gate,
		st.guard, services...)
}

// listToolNamesResult runs list_tools and returns the RAW result — blocked
// assertions need Success=false and the error text, not an assert-pass helper.
func (st *driftPGStack) listToolNamesResult(t *testing.T, memberID string) *types.ToolResult {
	t.Helper()
	return agentITDiscoverResult(agentITContext(st.tenantID, memberID), t, st.memberSession(t, memberID),
		map[string]any{"mode": "list_tools", "server_id": st.service.ID})
}

// listToolNames asserts a WORKING directory and returns its tool names.
func (st *driftPGStack) listToolNames(t *testing.T, memberID string) []string {
	t.Helper()
	page := agentITDiscover(agentITContext(st.tenantID, memberID), t, st.memberSession(t, memberID),
		map[string]any{"mode": "list_tools", "server_id": st.service.ID})
	return agentITToolNames(page)
}

// TestDriftBlocksFromMemberConversation（计划 Step 5 用例 1）：安装 v1（工具
// t1）→ 成员调用成功；受控服务 SetTools：t1 schema 变 + 新增 t2 → 成员新
// 会话 list_tools 被阻（catalog 折叠为目录级 error 状态文案；ErrPluginDrift
// 原文在服务端审计日志）；CheckDrift → detected 且 detail 正确
// （Added=[t2], SchemaChanged=[t1]）；ResolveDrift 后新快照生效：成员目录
// 恢复可用、t1 按新 schema 调用成功、漂移清零；新增 t2 因远端目录无法自证
// 只读而保守落策略行 Enabled=false（复审 Ruling——治理面启用是 T18 范畴），
// 成员目录不可见 t2。
func TestDriftBlocksFromMemberConversation(t *testing.T) {
	db := openPluginDB(t)
	st := newDriftPGStack(t, db, 1, "com.example.drift-member")
	ctx := agentITContext(1, "member-1")
	bg := context.Background()

	// 漂移前：成员目录 [t1]，调用成功。
	require.Equal(t, []string{"t1"}, st.listToolNames(t, "member-1"))
	sessionBefore := st.memberSession(t, "member-1")
	def := agentITDescribe(ctx, t, sessionBefore, st.service.ID, "t1")
	call := agentITCall(ctx, t, sessionBefore, def.ToolRef, json.RawMessage(`{}`))
	require.True(t, call.Success, call.Error)
	require.Contains(t, call.Output, "ok:t1:")

	// 漂移：t1 schema 变 + 新增 t2（目录公开的替身，成员无授权面）。
	st.remote.SetTools([]plugintest.Tool{
		{Name: "t1", Description: "v1 tool", ReadOnly: true, InputSchema: driftPGT1SchemaV2},
		{Name: "t2", Description: "new tool", ReadOnly: true, InputSchema: agentITNoArgSchema},
	})

	// 成员新会话：list_tools 被阻（T09 fail-closed；catalog 折叠文案）。
	blocked := st.listToolNamesResult(t, "member-1")
	require.False(t, blocked.Success, "a drifted directory must block member discovery")
	require.Contains(t, blocked.Error, "is error", "the catalog folds the drift verdict into a server-state error")
	require.Contains(t, blocked.Error, "retry discovery")

	// t2 不可见：describe 也被同一漂移判定拒绝（快照内 t1 的 schema 变使
	// 整个目录加载 fail-closed，新增 t2 不进入成员可见面）。
	describeBlocked := agentITDiscoverResult(ctx, t, st.memberSession(t, "member-1"), map[string]any{
		"mode": "describe", "server_id": st.service.ID, "tool_name": "t2",
	})
	require.False(t, describeBlocked.Success)

	// 管理面：CheckDrift → detected 且 detail 正确（不经清单——清单 host
	// 未变化，直连已接受端点的实况）。
	checked, err := st.svc.CheckDrift(bg, 1, st.installationID)
	require.NoError(t, err)
	require.Equal(t, types.PluginDriftDetected, checked.DriftState)
	require.NotNil(t, checked.Detail)
	require.Equal(t, []string{"t2"}, checked.Detail.Added)
	require.Equal(t, []string{"t1"}, checked.Detail.SchemaChanged)
	require.Empty(t, checked.Detail.Removed)

	// drift_detail 不含远端 schema 原文。
	raw, err := json.Marshal(checked.Detail)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "inputSchema")
	require.NotContains(t, string(raw), driftPGT1SchemaV2)

	// 复审闭环：ResolveDrift 接受当前远端目录为新快照（版本不变）。
	resolved, err := st.svc.ResolveDrift(bg, 1, "admin-1", st.installationID)
	require.NoError(t, err)
	require.Equal(t, "1.0.0", resolved.Version, "resolve keeps the accepted version — it is not an upgrade")
	require.Equal(t, types.PluginDriftNone, resolved.DriftState)

	// 新快照生效：成员新会话目录恢复 [t1]（t2 保守关闭不可见），t1 按
	// 新 schema 调用成功。
	require.Equal(t, []string{"t1"}, st.listToolNames(t, "member-1"))
	sessionAfter := st.memberSession(t, "member-1")
	defAfter := agentITDescribe(ctx, t, sessionAfter, st.service.ID, "t1")
	callAfter := agentITCall(ctx, t, sessionAfter, defAfter.ToolRef, json.RawMessage(`{}`))
	require.True(t, callAfter.Success, callAfter.Error)
	require.Contains(t, callAfter.Output, "ok:t1:")

	// t2 的保守策略行：安装视图 Enabled=false（新增工具不能经 live
	// ListTools 自证只读）。
	view, err := st.svc.GetInstallation(bg, 1, st.installationID)
	require.NoError(t, err)
	enabledByTool := map[string]*bool{}
	for _, tool := range view.Tools {
		enabledByTool[tool.Name] = tool.Enabled
	}
	require.NotNil(t, enabledByTool["t1"])
	require.True(t, *enabledByTool["t1"])
	require.NotNil(t, enabledByTool["t2"])
	require.False(t, *enabledByTool["t2"], "a NEW tool lands disabled — it cannot self-certify read-only (T17 Ruling)")

	// 漂移清零后管理面读回 none。
	after, err := st.svc.GetDrift(bg, 1, st.installationID)
	require.NoError(t, err)
	require.Equal(t, types.PluginDriftNone, after.DriftState)
	require.Nil(t, after.Detail)
}

// TestDriftScopedToSingleInstallation（计划 Step 5 用例 2）：两个独立受控
// 服务实例 A/B；空间 1 装 A、空间 2 装 B（同 plugin_id 不同 URL——
// plugin_id 唯一性约束在 (tenant, plugin) 上，跨空间可同名）：仅 A 漂移 →
// 空间 1 成员被阻、空间 2 成员正常，两安装行漂移状态各自独立。
func TestDriftScopedToSingleInstallation(t *testing.T) {
	db := openPluginDB(t)
	const pluginID = "com.example.drift-scope"
	st1 := newDriftPGStack(t, db, 1, pluginID)
	st2 := newDriftPGStack(t, db, 2, pluginID)
	bg := context.Background()

	// 两空间漂移前均正常。
	require.Equal(t, []string{"t1"}, st1.listToolNames(t, "member-1"))
	require.Equal(t, []string{"t1"}, st2.listToolNames(t, "member-2"))

	// 仅空间 1 的远端漂移（t1 schema 变）。
	st1.remote.SetTools([]plugintest.Tool{{
		Name: "t1", Description: "v1 tool", ReadOnly: true, InputSchema: driftPGT1SchemaV2,
	}})

	// 空间 1 成员被阻；空间 2 成员不受影响（目录正常、调用成功）。
	blocked := st1.listToolNamesResult(t, "member-1")
	require.False(t, blocked.Success, "the drifted installation blocks ITS tenant's directory")
	require.Equal(t, []string{"t1"}, st2.listToolNames(t, "member-2"),
		"another space's installation of the same plugin must stay unaffected")
	ctx2 := agentITContext(2, "member-2")
	session2 := st2.memberSession(t, "member-2")
	def2 := agentITDescribe(ctx2, t, session2, st2.service.ID, "t1")
	call2 := agentITCall(ctx2, t, session2, def2.ToolRef, json.RawMessage(`{}`))
	require.True(t, call2.Success, call2.Error)
	require.Contains(t, call2.Output, "ok:t1:")

	// 两安装行漂移状态各自独立：空间 1 detected（管理员核验），空间 2
	// none（管理员核验后确认基线未变）。
	checked1, err := st1.svc.CheckDrift(bg, 1, st1.installationID)
	require.NoError(t, err)
	require.Equal(t, types.PluginDriftDetected, checked1.DriftState)
	require.Equal(t, []string{"t1"}, checked1.Detail.SchemaChanged)
	checked2, err := st2.svc.CheckDrift(bg, 2, st2.installationID)
	require.NoError(t, err)
	require.Equal(t, types.PluginDriftNone, checked2.DriftState, "space 2's own baseline is intact")
}

// TestDriftManualServiceNotGoverned（计划 Step 5 用例 3，原名
// TestManualServiceNotGovernedByDrift——以 TestDrift 前缀命名使 Brief 的
// 测试命令 `-run 'TestDrift'` 覆盖全部三用例）：手工服务（无插件关联）
// 工具 schema 变化后：Agent 目录正常反映远端现状（schema 变化直接生效
// ——无快照基线可比对），调用成功；守卫 provider 对手工服务返回 nil
//（不治理）；无 drift 状态可查（GetByServiceID 无安装行、drift API 对
// 不存在的安装 ID 一律 not found）。
func TestDriftManualServiceNotGoverned(t *testing.T) {
	db := openPluginDB(t)
	const tenantID = uint64(3)

	manual := plugintest.New()
	manual.PluginID = "com.example.manual-standin"
	manual.Version = "1.0.0"
	manual.SetTools([]plugintest.Tool{{
		Name: "m1", Description: "manual tool", ReadOnly: true,
		InputSchema: driftPGT1SchemaV1,
	}})
	manual.Start(t)

	pluginRepo := repository.NewPluginRepository(db)
	mcpRepo := repository.NewMCPServiceRepository(db)
	manager := internalmcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)

	// 手工服务行：无 PluginInstallationID、不经插件域任何 API 创建。
	endpoint := manual.BaseURL() + "/mcp"
	manualRow := &types.MCPService{
		TenantID:      tenantID,
		Enabled:       true,
		Name:          "manual-drift-coexists",
		URL:           &endpoint,
		TransportType: types.MCPTransportHTTPStreamable,
	}
	require.NoError(t, mcpRepo.Create(context.Background(), manualRow))

	// 生产形态守卫（含 best-effort 置位）：手工服务解析为 nil——无漂移
	// 治理概念。
	guard := service.PluginSnapshotLookupWithDriftMarking(pluginRepo, newManagerLister(manager))
	snap, err := guard(context.Background(), tenantID, manualRow.ID)
	require.NoError(t, err)
	require.Nil(t, snap, "a manual service has no installation snapshot — the guard must not govern it")

	// 成员会话辅助（与 driftPGStack 同款组装，直接经 ListEnabled）。
	memberSession := func(t *testing.T, memberID string) *tools.ToolRegistry {
		t.Helper()
		services, err := mcpRepo.ListEnabled(context.Background(), tenantID)
		require.NoError(t, err)
		return agentITRegister(agentITContext(tenantID, memberID), t, manager, &agentITGate{}, guard, services...)
	}
	ctx := agentITContext(tenantID, "member-1")

	// 漂移前：目录 [m1]、调用成功。
	page := agentITDiscover(ctx, t, memberSession(t, "member-1"), map[string]any{
		"mode": "list_tools", "server_id": manualRow.ID,
	})
	require.Equal(t, []string{"m1"}, agentITToolNames(page))

	// 手工服务远端 schema 随意变化：目录正常反映远端现状、调用成功——
	// 无快照基线可比对，无漂移阻断（现状语义；mcp_exposure 的 describe/call
	// ref 校验除外，非本需求新增治理）。
	manual.SetTools([]plugintest.Tool{{
		Name: "m1", Description: "manual tool", ReadOnly: true,
		InputSchema: driftPGT1SchemaV2,
	}})
	pageAfter := agentITDiscover(ctx, t, memberSession(t, "member-1"), map[string]any{
		"mode": "list_tools", "server_id": manualRow.ID, "refresh": true,
	})
	require.Equal(t, []string{"m1"}, agentITToolNames(pageAfter),
		"a manual service's directory reflects the remote as-is — no drift governance")
	session := memberSession(t, "member-1")
	def := agentITDescribe(ctx, t, session, manualRow.ID, "m1")
	call := agentITCall(ctx, t, session, def.ToolRef, json.RawMessage(`{}`))
	require.True(t, call.Success, call.Error)
	require.Contains(t, call.Output, "ok:m1:")

	// 无 drift 状态：手工服务无安装行（GetByServiceID 空）；drift API 对
	// 它没有可查对象（安装 ID 不存在 → not found）。
	inst, err := pluginRepo.GetByServiceID(context.Background(), tenantID, manualRow.ID)
	require.NoError(t, err)
	require.Nil(t, inst)
	svc := service.NewPluginService(pluginRepo, nil, nil, nil, nil, nil, nil)
	_, err = svc.GetDrift(context.Background(), tenantID, manualRow.ID)
	require.ErrorIs(t, err, service.ErrInstallationNotFound)
}
