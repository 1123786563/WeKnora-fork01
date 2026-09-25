//go:build integration

// T18（plan 10 Task 18 Step 5）：真 PostgreSQL 应用边界集成测试——写工具
// 默认关闭的零外部写入验收 + 成员授权不启用 + 手工缺省语义回归。
//
// 环境契约（与 plugin_pg_integration_test.go 同一约定）：
//
//	PLUGIN_TEST_DATABASE_URL  一次性 PostgreSQL DSN。
//	                          缺失 → t.Fatal("blocked-env: ...")，绝不 Skip 通过。
//
// 复用同包基建：openPluginDB（隔离 schema + 冻结模型）、plugintest 受控远端
// （SetTools 热切目录、WriteCalls 写计数、EnableOAuth 完整授权端点）、
// newManagerLister 与 plugin_agent_integration_test.go 的 agentIT* 会话组装、
// plugin_drift_integration_test.go 的 driftPGGate（读真实 PG 策略行的
// approval gate——写工具关闭必须经真实行过滤才可从成员会话观察）。
package plugins_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
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

// completeAuthorizationForMember drives the member-consent leg of the real
// OAuth flow inline (the T11 pluginPGStack.authorizeMember shape): fetch the
// authorize page (registers state), submit the member's credentials, and hand
// the one-time code + state to the caller for CompleteAuthorization.
func completeAuthorizationForMember(
	t *testing.T, authURL, username, password string,
) (code, state string) {
	t.Helper()
	u, err := url.Parse(authURL)
	require.NoError(t, err)
	state = u.Query().Get("state")
	require.NotEmpty(t, state)
	require.NotEmpty(t, u.Query().Get("code_challenge"), "PKCE code_challenge required")

	noRedirect := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	page, err := noRedirect.Get(u.String())
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, page.StatusCode, "authorize page must render")

	form := url.Values{"state": {state}, "username": {username}, "password": {password}}
	resp, err := noRedirect.PostForm(u.String(), form)
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, resp.StatusCode, "member consent must 302 back with a code")
	location, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	code = location.Query().Get("code")
	require.NotEmpty(t, code)
	require.Equal(t, state, location.Query().Get("state"))
	return code, state
}

// writeToolsPGStack：plugintest 单远端 + 真实 PG 仓储/服务/manager，落一条
// 已确认 v1 安装；gate 读真实 PG 策略行（driftPGGate），guard 为生产形态
// 运行守卫。无 OAuth 面（升级写工具场景用无账号写工具——brief 二选一裁决）。
type writeToolsPGStack struct {
	tenantID       uint64
	svc            interfaces.PluginService
	pluginRepo     interfaces.PluginRepository
	mcpRepo        interfaces.MCPServiceRepository
	approvalSvc    interfaces.MCPToolApprovalService
	manager        *internalmcp.MCPManager
	remote         *plugintest.Server
	installationID string
	service        *types.MCPService
	gate           *driftPGGate
	guard          tools.PluginSnapshotProvider
}

// newWriteToolsPGStack assembles the stack and lands ONE confirmed v1
// installation with the no-account read tool r.
func newWriteToolsPGStack(t *testing.T, db *gorm.DB, tenantID uint64, pluginID string) *writeToolsPGStack {
	t.Helper()

	remote := plugintest.New()
	remote.PluginID = pluginID
	remote.Version = "1.0.0"
	remote.SetTools([]plugintest.Tool{{
		Name: "r", Description: "v1 read tool", ReadOnly: true,
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

	return &writeToolsPGStack{
		tenantID:       tenantID,
		svc:            svc,
		pluginRepo:     pluginRepo,
		mcpRepo:        mcpRepo,
		approvalSvc:    approvalSvc,
		manager:        manager,
		remote:         remote,
		installationID: confirmed.InstallationID,
		service:        serviceRow,
		gate:           &driftPGGate{approvalSvc: approvalSvc},
		guard:          service.PluginSnapshotLookupWithDriftMarking(pluginRepo, newManagerLister(manager)),
	}
}

// memberSession assembles ONE member conversation's tool surface the way the
// Agent engine does per session (fresh registry = a NEW conversation), over
// the REAL policy gate — the disabled write row must block through it.
func (st *writeToolsPGStack) memberSession(t *testing.T, memberID string) *tools.ToolRegistry {
	t.Helper()
	services, err := st.mcpRepo.ListEnabled(context.Background(), st.tenantID)
	require.NoError(t, err)
	registry := tools.NewToolRegistry()
	_, err = tools.RegisterMCPTools(
		agentITContext(st.tenantID, memberID), registry, services, st.manager, st.gate, 0, nil, nil, st.guard)
	require.NoError(t, err)
	return registry
}

// upgradeToV2WithWriteTool switches the remote to v2: the read tool r plus the
// no-account WRITE tool w, and accepts the upgrade (admin preview → accept).
func (st *writeToolsPGStack) upgradeToV2WithWriteTool(t *testing.T) {
	t.Helper()
	st.remote.SetTools([]plugintest.Tool{
		{Name: "r", Description: "v1 read tool", ReadOnly: true, InputSchema: agentITNoArgSchema},
		{Name: "w", Description: "v2 write tool", ReadOnly: false, InputSchema: agentITNoArgSchema},
	})
	st.remote.Version = "2.0.0"
	bg := context.Background()
	preview, err := st.svc.PreviewUpgrade(bg, st.tenantID, st.installationID)
	require.NoError(t, err)
	require.Equal(t, "2.0.0", preview.Diff.CandidateVersion)
	_, err = st.svc.AcceptUpgrade(bg, st.tenantID, "admin-1", st.installationID, preview.CandidateFingerprint)
	require.NoError(t, err)
}

// toolPolicy 返回治理列表中一个工具的行（必须存在）。
func (st *writeToolsPGStack) toolPolicy(t *testing.T, name string) interfaces.PluginInstallationToolPolicy {
	t.Helper()
	rows, err := st.svc.ListInstallationTools(context.Background(), st.tenantID, st.installationID)
	require.NoError(t, err)
	for _, row := range rows {
		if row.Name == name {
			return row
		}
	}
	t.Fatalf("tool %q missing from the governance list", name)
	return interfaces.PluginInstallationToolPolicy{}
}

// TestUpgradedWriteToolStaysDisabledAndZeroExternalWrites（计划 Step 5 用例
// 1）：v1（只读 r）安装 → 升级 v2（新增无账号写工具 w，受控服务写计数）：
//  1. 治理面：w Enabled=false + 默认关闭原因，r Enabled=true——「目录可见」
//     的治理面形态（成员看得见插件带了什么、为何不可用）；
//  2. 成员 Agent 目录对 w 整体不可达（既有 catalog 语义：list_tools 按
//     gate.IsEnabled 批量过滤、describe 的 checkEnabled 拒绝、call 的 ref
//     解析拒绝——与手工服务禁用工具同款，Global Constraints 不变）；
//     受控服务 WriteCalls()==0——关闭期间外部写入计数恒为零；
//  3. 只读 r 升级前后全程可调；
//  4. 管理员启用 w 后目录可见、调用成功且 WriteCalls()==1（恰一次外部写
//     入）；再关闭后，已 describe 的旧 ref 调用仍被拒（调用时策略重查），
//     写计数保持 1。
func TestUpgradedWriteToolStaysDisabledAndZeroExternalWrites(t *testing.T) {
	db := openPluginDB(t)
	st := newWriteToolsPGStack(t, db, 1, "com.example.write-tools")
	ctx := agentITContext(1, "member-1")
	bg := context.Background()

	// 升级前：只读 r 可调（原策略，全程可调的第一段）。
	sessionV1 := st.memberSession(t, "member-1")
	defR := agentITDescribe(ctx, t, sessionV1, st.service.ID, "r")
	callR := agentITCall(ctx, t, sessionV1, defR.ToolRef, json.RawMessage(`{}`))
	require.True(t, callR.Success, callR.Error)

	// 升级 v2：新增写工具 w。
	st.upgradeToV2WithWriteTool(t)

	// 治理面（GET .../tools 的服务契约）：w 关闭带默认原因；r 原策略。
	wPolicy := st.toolPolicy(t, "w")
	require.False(t, wPolicy.Enabled)
	require.Equal(t, interfaces.PluginWriteToolDisabledReason, wPolicy.DisabledReason)
	rPolicy := st.toolPolicy(t, "r")
	require.True(t, rPolicy.Enabled)
	require.Empty(t, rPolicy.DisabledReason)

	// 成员新会话（同一会话贯穿后续启停——agent engine 可超越设置变更存活）：
	// 目录只见 r（w 被真实策略行过滤出成员目录）。
	sessionV2 := st.memberSession(t, "member-1")
	page := agentITDiscover(ctx, t, sessionV2, map[string]any{"mode": "list_tools", "server_id": st.service.ID})
	require.Equal(t, []string{"r"}, agentITToolNames(page),
		"a disabled write tool is policy-hidden from the member agent directory (shipped catalog semantics)")

	// describe w → 拒绝（checkEnabled）：调用前置被拒，写计数 0。
	describeBlocked := agentITDiscoverResult(ctx, t, sessionV2, map[string]any{
		"mode": "describe", "server_id": st.service.ID, "tool_name": "w",
	})
	require.False(t, describeBlocked.Success, "describing the disabled write tool must be rejected")
	require.Contains(t, describeBlocked.Error, "enabled")
	require.Zero(t, st.remote.WriteCalls(),
		"zero external writes while the write tool stays disabled")

	// 只读 r 在升级后仍可调（原策略，全程可调的第二段）。
	defR2 := agentITDescribe(ctx, t, sessionV2, st.service.ID, "r")
	callR2 := agentITCall(ctx, t, sessionV2, defR2.ToolRef, json.RawMessage(`{}`))
	require.True(t, callR2.Success, callR2.Error)

	// 管理员启用 w（T18 治理面）→ 同一会话的目录立即反映（策略总是现查）：
	// describe 成功拿 ref，call 成功，恰一次外部写入。
	enable := true
	rows, err := st.svc.SetInstallationToolPolicy(bg, st.tenantID, st.installationID, "w", &enable, nil)
	require.NoError(t, err)
	for _, row := range rows {
		if row.Name == "w" {
			require.True(t, row.Enabled)
			require.Empty(t, row.DisabledReason)
		}
	}
	pageEnabled := agentITDiscover(ctx, t, sessionV2, map[string]any{"mode": "list_tools", "server_id": st.service.ID})
	require.ElementsMatch(t, []string{"r", "w"}, agentITToolNames(pageEnabled),
		"after the admin enables the tool it enters the member directory")
	defW := agentITDescribe(ctx, t, sessionV2, st.service.ID, "w")
	allowed := agentITCall(ctx, t, sessionV2, defW.ToolRef, json.RawMessage(`{}`))
	require.True(t, allowed.Success, allowed.Error)
	require.Equal(t, int64(1), st.remote.WriteCalls(),
		"exactly ONE external write after the admin enables the tool")

	// 再关闭：已 describe 的旧 ref 调用仍被拒（调用时策略重查——已注册的
	// agent 会话不能绕过设置变更），写计数保持 1。
	disable := false
	_, err = st.svc.SetInstallationToolPolicy(bg, st.tenantID, st.installationID, "w", &disable, nil)
	require.NoError(t, err)
	staleRef := agentITCall(ctx, t, sessionV2, defW.ToolRef, json.RawMessage(`{}`))
	require.False(t, staleRef.Success, "a stale tool_ref of a re-disabled write tool must be rejected")
	require.Equal(t, int64(1), st.remote.WriteCalls(), "the re-disable must not add external writes")
}

// TestMemberAuthorizationDoesNotEnableWriteTool（计划 Step 5 用例 2）：插件
// 声明 personal_oauth；成员完成个人授权（真实 OAuth 流 + connections/me
// authorized）后，写工具（requires_personal_auth=true, read_only=false）的
// call 仍被真实策略行拒绝、写计数 0——授权 ≠ 启用。
func TestMemberAuthorizationDoesNotEnableWriteTool(t *testing.T) {
	db := openPluginDB(t)
	const tenantID = uint64(2)

	// 远端：OAuth 端点集 + 一个需个人授权的写工具。
	remote := plugintest.New()
	remote.PluginID = "com.example.write-auth"
	remote.Version = "1.0.0"
	remote.EnableOAuth(map[string]string{"user-a": "pass-a"})
	remote.SetTools([]plugintest.Tool{{
		Name: "push_todo", Description: "write, personal auth", ReadOnly: false,
		RequiresPersonalAuth: true, Scopes: []string{"write:demo"},
		InputSchema: agentITNoArgSchema,
		Call: func(member string) (string, error) {
			return "pushed-by:" + member, nil
		},
	}})
	remote.Start(t)

	pluginRepo := repository.NewPluginRepository(db)
	mcpRepo := repository.NewMCPServiceRepository(db)
	approvalRepo := repository.NewMCPToolApprovalRepository(db)
	oauthRepo := repository.NewMCPOAuthRepository(db)
	manager := internalmcp.NewMCPManager(oauthRepo)
	t.Cleanup(manager.Shutdown)
	mcpSvcService := service.NewMCPServiceService(mcpRepo, manager, oauthRepo)
	approvalSvc := service.NewMCPToolApprovalService(approvalRepo, mcpRepo)
	closer := service.MCPClientCloser(func(serviceID string) { _ = manager.CloseClient(serviceID) })
	svc := service.NewPluginService(pluginRepo, mcpSvcService, mcpRepo, approvalSvc, newManagerLister(manager), closer, oauthRepo)
	oauthManager := internalmcp.NewOAuthManager(oauthRepo, mcpRepo, nil)

	bg := context.Background()
	preview, err := svc.PreviewFromManifest(bg, tenantID, "admin-1", remote.ManifestURL())
	require.NoError(t, err)
	confirmed, err := svc.ConfirmInstallation(bg, tenantID, "admin-1", preview.PreviewID)
	require.NoError(t, err)
	serviceRow, err := mcpRepo.GetByID(bg, tenantID, confirmed.ServiceID)
	require.NoError(t, err)
	require.NotNil(t, serviceRow)

	// 安装即写策略行：push_todo Enabled=false（B5）。
	rows, err := svc.ListInstallationTools(bg, tenantID, confirmed.InstallationID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.False(t, rows[0].Enabled, "the personal-auth WRITE tool installs disabled")
	require.Equal(t, interfaces.PluginWriteToolDisabledReason, rows[0].DisabledReason)

	// 成员完成个人授权（真实 OAuth 流——与 T11 同款驱动）。
	member := memberPrincipal("user-a")
	authURL, _, err := oauthManager.StartAuthorization(bg, serviceRow, tenantID, member, "http://127.0.0.1:1/cb", "/")
	require.NoError(t, err)
	code, state := completeAuthorizationForMember(t, authURL, "user-a", "pass-a")
	_, _, err = oauthManager.CompleteAuthorization(bg, state, code)
	require.NoError(t, err)

	// connections/me → authorized：成员确实完成了个人授权。
	status, err := svc.GetMyConnectionStatus(bg, tenantID, confirmed.InstallationID, member)
	require.NoError(t, err)
	require.Equal(t, types.PluginConnectionAuthorized, status.State)
	require.True(t, status.Authorized)

	// 授权不启用：治理面仍关闭、调用仍拒绝、写计数 0。
	rows, err = svc.ListInstallationTools(bg, tenantID, confirmed.InstallationID)
	require.NoError(t, err)
	require.False(t, rows[0].Enabled, "authorization must not flip the tool policy")
	require.Equal(t, interfaces.PluginWriteToolDisabledReason, rows[0].DisabledReason)

	gate := &driftPGGate{approvalSvc: approvalSvc}
	guard := service.PluginSnapshotLookupWithDriftMarking(pluginRepo, newManagerLister(manager))
	// OAuth 插件服务的目录加载要求调用 ctx 携带 ToolExecContext（T13
	// askJiraTool 先例——成员在对话中执行工具的常态）。
	memberCtx := agentITCallCtx(agentITContext(tenantID, "user-a"))
	registry := tools.NewToolRegistry()
	_, err = tools.RegisterMCPTools(agentITContext(tenantID, "user-a"), registry,
		[]*types.MCPService{serviceRow}, manager, gate, 0, nil, nil, guard)
	require.NoError(t, err)

	// 已授权成员的 Agent 目录：写工具被真实策略行过滤，目录为空；
	// describe 前置被拒（checkEnabled）——调用面整体不可达，写计数 0。
	page := agentITDiscover(memberCtx, t, registry, map[string]any{
		"mode": "list_tools", "server_id": serviceRow.ID,
	})
	require.Empty(t, agentITToolNames(page),
		"the disabled write tool is hidden from an AUTHORIZED member's directory")
	describeBlocked := agentITDiscoverResult(memberCtx, t, registry, map[string]any{
		"mode": "describe", "server_id": serviceRow.ID, "tool_name": "push_todo",
	})
	require.False(t, describeBlocked.Success, "describing the disabled write tool must stay rejected after authorization")
	require.Contains(t, describeBlocked.Error, "enabled")
	require.Zero(t, remote.WriteCalls(), "zero external writes — authorization is not enablement")
}

// TestManualDefaultSemanticsUnchanged（计划 Step 5 用例 3）：手工服务（无
// 插件关联）加 1 只读 + 1 写工具（无任何 approval 行）→ Agent 目录可见且
// 双双可调（缺行默认启用语义——types.MCPToolApproval.Enabled 的兼容语义保留
// 给手工服务，不因插件策略设施引入而改变；含写工具：手工域无「写默认关」
// 概念）。真实 gate（driftPGGate 委托 IsEnabled 的缺行=启用）从成员会话
// 观察。
func TestManualDefaultSemanticsUnchanged(t *testing.T) {
	db := openPluginDB(t)
	const tenantID = uint64(3)

	manual := plugintest.New()
	manual.PluginID = "com.example.manual-write-standin"
	manual.Version = "1.0.0"
	manual.SetTools([]plugintest.Tool{
		{Name: "m_read", Description: "manual read", ReadOnly: true, InputSchema: agentITNoArgSchema},
		{Name: "m_write", Description: "manual write", ReadOnly: false, InputSchema: agentITNoArgSchema},
	})
	manual.Start(t)

	pluginRepo := repository.NewPluginRepository(db)
	mcpRepo := repository.NewMCPServiceRepository(db)
	approvalRepo := repository.NewMCPToolApprovalRepository(db)
	manager := internalmcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	approvalSvc := service.NewMCPToolApprovalService(approvalRepo, mcpRepo)

	// 手工服务行：无 PluginInstallationID、不经插件域任何 API 创建。
	endpoint := manual.BaseURL() + "/mcp"
	manualRow := &types.MCPService{
		TenantID:      tenantID,
		Enabled:       true,
		Name:          "manual-write-default",
		URL:           &endpoint,
		TransportType: types.MCPTransportHTTPStreamable,
	}
	require.NoError(t, mcpRepo.Create(context.Background(), manualRow))

	// 前提自证：该服务无任何策略行；IsEnabled 的缺行判定 = 启用（未改动）。
	enabled, err := approvalSvc.IsEnabled(context.Background(), tenantID, manualRow.ID, "m_write")
	require.NoError(t, err)
	require.True(t, enabled, "the manual missing-row default stays ENABLED (types/mcp.go:148-152 untouched)")

	// 成员会话（真实 gate + 生产守卫——手工服务解析 nil 快照不治理）：
	// 目录可见只读+写工具，双双可调。
	gate := &driftPGGate{approvalSvc: approvalSvc}
	guard := service.PluginSnapshotLookupWithDriftMarking(pluginRepo, newManagerLister(manager))
	ctx := agentITContext(tenantID, "member-1")
	registry := tools.NewToolRegistry()
	_, err = tools.RegisterMCPTools(ctx, registry,
		[]*types.MCPService{manualRow}, manager, gate, 0, nil, nil, guard)
	require.NoError(t, err)

	page := agentITDiscover(ctx, t, registry, map[string]any{"mode": "list_tools", "server_id": manualRow.ID})
	require.ElementsMatch(t, []string{"m_read", "m_write"}, agentITToolNames(page),
		"a manual service's directory shows its tools as-is — no plugin governance")

	defRead := agentITDescribe(ctx, t, registry, manualRow.ID, "m_read")
	callRead := agentITCall(ctx, t, registry, defRead.ToolRef, json.RawMessage(`{}`))
	require.True(t, callRead.Success, callRead.Error)

	defWrite := agentITDescribe(ctx, t, registry, manualRow.ID, "m_write")
	callWrite := agentITCall(ctx, t, registry, defWrite.ToolRef, json.RawMessage(`{}`))
	require.True(t, callWrite.Success, callWrite.Error,
		"a manual WRITE tool with no approval row stays callable — the plugin-domain default-off never leaks into manual services")
	require.Equal(t, int64(1), manual.WriteCalls(), "exactly the one manual write dispatch")
	require.Equal(t, int64(1), manual.Calls(), "exactly the one manual read dispatch")
}
