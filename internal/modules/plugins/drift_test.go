package plugins_test

// T17（plan 09 Task 17 Step 1）：漂移检测持久化与管理员复审闭环——fake 层
// 语义测试。
//
// 覆盖计划 Step 1 的四个用例：
//  1. TestDiffLiveAgainstSnapshot：live 目录 vs 已接受快照的三形态差异
//     （新增/移除/schema 变）+ description 变（与 T09 运行时守卫同口径的
//     第四证据维度，见文件内 Ruling 注释）。
//  2. TestCheckDriftPersistsDetectedState：受控服务 SetTools 漂移后
//     CheckDrift → drift_state=detected、detail 落库；目录复原后 →
//     drift_state 重回 none、detail 清空；核验不经清单（清单 host 下线后
//     仍按已接受端点实况工作——总索引「安装后远端真相的统一口径」）。
//  3. TestResolveDriftRebasesSnapshot：漂移（新增 c/w）后 ResolveDrift：
//     新快照含 c/w、新增工具策略行 Enabled=false（保守写分类 Ruling）、
//     drift_state=none、版本/端点不变、守卫目录见 c/w、幂等零写入。
//  4. TestResolveDriftRejectsWhenEndpointBroken：远端不可达时 resolve
//     拒绝且快照/漂移状态不变。
//
// 外部测试包 plugins_test 并 import plugintest（总索引「测试包名约定」）。
// fake 基建复用 install_service_test.go / upgrade_accept_test.go（同包）：
// driftRepo 在 upgradeAcceptRepo（升级写 seam）之上补 UpdateDrift（漂移
// 持久化 seam），栈组装仿 newUpgradeAcceptStack。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/tools"
	internalmcp "github.com/Tencent/WeKnora/internal/modules/airesource/mcp"
	"github.com/Tencent/WeKnora/internal/modules/plugins"
	"github.com/Tencent/WeKnora/internal/modules/plugins/plugintest"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const driftTestPluginID = "com.example.drift"

// driftSchemaB 与 driftSchemaA 是同名工具的两个不同 schema（schema 变形态）。
const (
	driftSchemaA  = `{"type":"object","properties":{"q":{"type":"string"}},"required":["q"],"additionalProperties":false}`
	driftSchemaA2 = `{"type":"object","properties":{"q":{"type":"string"},"limit":{"type":"integer"}},"required":["q"],"additionalProperties":false}`
)

// driftWrite 记录一次漂移状态持久化（state + detail 原文）。
type driftWrite struct {
	state  string
	detail json.RawMessage
}

// driftRepo 在 upgradeAcceptRepo 之上补齐 UpdateDrift（对真实 gorm
// pluginRepository.UpdateDrift 的同构内存实现：state/detail 落行），并记录
// 全部调用供断言。
type driftRepo struct {
	*upgradeAcceptRepo
	driftWrites     []driftWrite
	updateDriftErr error
}

func (r *driftRepo) UpdateDrift(
	_ context.Context, tenantID uint64, id, state string, detail json.RawMessage,
) error {
	if r.updateDriftErr != nil {
		return r.updateDriftErr
	}
	for _, inst := range r.installations {
		if inst.TenantID == tenantID && inst.ID == id {
			inst.DriftState = state
			if detail == nil {
				inst.DriftDetail = nil
			} else {
				inst.DriftDetail = append(json.RawMessage(nil), detail...)
			}
			r.driftWrites = append(r.driftWrites, driftWrite{state: state, detail: detail})
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

// driftStack：plugintest 单远端 + 可下线清单 host + 包装仓储 + 真实
// PluginService，落一条已确认安装（快照 [a, b]，均只读）。
type driftStack struct {
	svc          interfaces.PluginService
	pluginRepo   *driftRepo
	innerRepo    *installPreviewRepo
	mcpRepo      *upgradeAcceptMCPRepo
	approvalRepo *fakeInstallApprovalRepo
	approvalSvc  interfaces.MCPToolApprovalService
	manager      *internalmcp.MCPManager
	remote       *plugintest.Server
	manifestHost *httptest.Server
	manifestNext *[]byte
	inst         *types.PluginInstallation
}

// driftInstalledTools 是安装时（快照基线）的目录：a、b 两个只读工具。
func driftInstalledTools() []plugintest.Tool {
	return []plugintest.Tool{
		{Name: "a", Description: "tool a", ReadOnly: true, InputSchema: driftSchemaA},
		{Name: "b", Description: "tool b", ReadOnly: true, InputSchema: upgradeV1SearchSchema},
	}
}

func newDriftStack(t *testing.T, tenantID uint64) *driftStack {
	t.Helper()

	remote := plugintest.New()
	remote.PluginID = driftTestPluginID
	remote.Version = "1.0.0"
	remote.SetTools(driftInstalledTools())
	remote.Start(t)

	manifestJSON, err := marshalManifest(remote)
	require.NoError(t, err)
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(manifestJSON)
	})
	manifestHost := httptest.NewServer(mux)
	t.Cleanup(manifestHost.Close)

	manager := internalmcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)

	innerRepo := &installPreviewRepo{}
	mcpRepo := &upgradeAcceptMCPRepo{fakeInstallMCPServiceRepo: &fakeInstallMCPServiceRepo{}}
	approvalRepo := &fakeInstallApprovalRepo{}
	innerRepo.mcpRepo = mcpRepo.fakeInstallMCPServiceRepo
	innerRepo.approvalRepo = approvalRepo
	upgradeRepo := &upgradeAcceptRepo{installPreviewRepo: innerRepo, approvalRows: approvalRepo}
	pluginRepo := &driftRepo{upgradeAcceptRepo: upgradeRepo}

	mcpSvcService := service.NewMCPServiceService(mcpRepo, nil, nil)
	approvalSvc := service.NewMCPToolApprovalService(approvalRepo, mcpRepo)

	stack := &driftStack{
		pluginRepo:   pluginRepo,
		innerRepo:    innerRepo,
		mcpRepo:      mcpRepo,
		approvalRepo: approvalRepo,
		approvalSvc:  approvalSvc,
		manager:      manager,
		remote:       remote,
		manifestHost: manifestHost,
		manifestNext: &manifestJSON,
	}
	stack.svc = service.NewPluginService(pluginRepo, mcpSvcService, mcpRepo, approvalSvc,
		newManagerLister(manager), nil, nil)

	ctx := context.Background()
	preview, err := stack.svc.PreviewFromManifest(ctx, tenantID, "admin-1", manifestHost.URL+"/manifest.json")
	require.NoError(t, err)
	_, err = stack.svc.ConfirmInstallation(ctx, tenantID, "admin-1", preview.PreviewID)
	require.NoError(t, err)
	require.Len(t, innerRepo.installations, 1)
	stack.inst = innerRepo.installations[0]
	require.NotEmpty(t, stack.inst.ServiceID)
	return stack
}

// policyRows 返回安装物化服务的全部策略行（按工具名）。
func (s *driftStack) policyRows(t *testing.T, tenantID uint64) map[string]*types.MCPToolApproval {
	t.Helper()
	rows, err := s.approvalRepo.ListByService(context.Background(), tenantID, s.inst.ServiceID)
	require.NoError(t, err)
	out := map[string]*types.MCPToolApproval{}
	for _, row := range rows {
		out[row.ToolName] = row
	}
	return out
}

// guardSnapshot 返回运行时快照守卫当前供给的目录（T09 provider）。
func (s *driftStack) guardSnapshot(t *testing.T, tenantID uint64) []types.PluginToolSnapshot {
	t.Helper()
	lookup := service.PluginSnapshotLookup(s.pluginRepo)
	snap, err := lookup(context.Background(), tenantID, s.inst.ServiceID)
	require.NoError(t, err)
	require.NotNil(t, snap)
	return snap.Tools
}

// TestDiffLiveAgainstSnapshot（计划 Step 1 用例 1）：快照 [a, b]；live 目录
// [a(schema 变), c(新增)]（b 被移除）→ Detail.Added=[c], Removed=[b],
// SchemaChanged=[a]。补充分支：description 变（a 同 schema 新描述）→
// DescriptionChanged=[a]（与 T09 FilterToolsBySnapshot 的 fail-closed 口径
// 对齐——运行时把 description 变化视为漂移证据，CheckDrift 的判定必须同
// 口径，否则 description-only 漂移下成员被阻而管理面显示 none，复审闭环
// 断裂）；无漂移 → 三列表全空。
func TestDiffLiveAgainstSnapshot(t *testing.T) {
	schemaADigest := plugins.ToolSchemaDigest([]byte(driftSchemaA))
	snap := []types.PluginToolSnapshot{
		{Name: "a", Description: "tool a", InputSchemaDigest: schemaADigest, ReadOnly: true},
		{Name: "b", Description: "tool b", InputSchemaDigest: plugins.ToolSchemaDigest([]byte(upgradeV1SearchSchema)), ReadOnly: true},
	}

	t.Run("added removed and schema changed", func(t *testing.T) {
		live := []*types.MCPTool{
			{Name: "a", Description: "tool a", InputSchema: json.RawMessage(driftSchemaA2)},
			{Name: "c", Description: "tool c", InputSchema: json.RawMessage(upgradeV2WriteSchema)},
		}
		detail := plugins.DiffLiveAgainstSnapshot(live, snap)
		require.NotNil(t, detail)
		require.Equal(t, []string{"c"}, detail.Added)
		require.Equal(t, []string{"b"}, detail.Removed)
		require.Equal(t, []string{"a"}, detail.SchemaChanged)
		require.Empty(t, detail.DescriptionChanged)
		require.False(t, detail.CheckedAt.IsZero(), "CheckedAt must be stamped")
		require.True(t, detail.HasDrift())
	})

	t.Run("description only change is drift evidence", func(t *testing.T) {
		live := []*types.MCPTool{
			{Name: "a", Description: "tool a REWRITTEN", InputSchema: json.RawMessage(driftSchemaA)},
			{Name: "b", Description: "tool b", InputSchema: json.RawMessage(upgradeV1SearchSchema)},
		}
		detail := plugins.DiffLiveAgainstSnapshot(live, snap)
		require.NotNil(t, detail)
		require.Empty(t, detail.Added)
		require.Empty(t, detail.Removed)
		require.Empty(t, detail.SchemaChanged)
		require.Equal(t, []string{"a"}, detail.DescriptionChanged)
		require.True(t, detail.HasDrift())
	})

	t.Run("no drift on identical directory", func(t *testing.T) {
		live := []*types.MCPTool{
			{Name: "a", Description: "tool a", InputSchema: json.RawMessage(driftSchemaA)},
			{Name: "b", Description: "tool b", InputSchema: json.RawMessage(upgradeV1SearchSchema)},
		}
		detail := plugins.DiffLiveAgainstSnapshot(live, snap)
		require.NotNil(t, detail)
		require.Empty(t, detail.Added)
		require.Empty(t, detail.Removed)
		require.Empty(t, detail.SchemaChanged)
		require.Empty(t, detail.DescriptionChanged)
		require.False(t, detail.HasDrift())
	})
}

// TestCheckDriftPersistsDetectedState（计划 Step 1 用例 2）：受控服务
// SetTools 漂移后 CheckDrift → drift_state=detected、detail 落库；目录复原
// 后 CheckDrift → drift_state 重回 none、detail 清空。核验远端真相来自
// EndpointLister 直连已接受端点：清单 host 下线后 CheckDrift 仍按端点实况
// 报漂移（不经清单重抓）。GetDrift（Viewer 读）读回已持久化状态；未检测
// 过时 drift_state 为安装行当前值。
func TestCheckDriftPersistsDetectedState(t *testing.T) {
	const tenantID = uint64(11)
	s := newDriftStack(t, tenantID)
	ctx := context.Background()

	// 未检测过：GetDrift 返回安装行当前值（none）+ 快照工具名。
	before, err := s.svc.GetDrift(ctx, tenantID, s.inst.ID)
	require.NoError(t, err)
	require.Equal(t, types.PluginDriftNone, before.DriftState)
	require.Nil(t, before.Detail)
	require.ElementsMatch(t, []string{"a", "b"}, before.SnapshotToolNames)

	// 漂移：a schema 变 + 新增 c + 移除 b。
	s.remote.SetTools([]plugintest.Tool{
		{Name: "a", Description: "tool a", ReadOnly: true, InputSchema: driftSchemaA2},
		{Name: "c", Description: "tool c", ReadOnly: true, InputSchema: upgradeV2WriteSchema},
	})
	// 清单 host 下线：CheckDrift 不得经清单重抓（统一口径）。
	s.manifestHost.Close()

	checked, err := s.svc.CheckDrift(ctx, tenantID, s.inst.ID)
	require.NoError(t, err)
	require.Equal(t, types.PluginDriftDetected, checked.DriftState)
	require.NotNil(t, checked.Detail)
	require.Equal(t, []string{"c"}, checked.Detail.Added)
	require.Equal(t, []string{"b"}, checked.Detail.Removed)
	require.Equal(t, []string{"a"}, checked.Detail.SchemaChanged)
	require.False(t, checked.Detail.CheckedAt.IsZero())
	require.ElementsMatch(t, []string{"a", "b"}, checked.SnapshotToolNames)

	// detail 落库：安装行 state=detected、detail JSON 携带三列表与 checked_at，
	// 且不含远端 schema 原文（全局约束）。
	inst := s.innerRepo.installations[0]
	require.Equal(t, types.PluginDriftDetected, inst.DriftState)
	require.NotNil(t, inst.DriftDetail)
	var persisted types.PluginDriftDetail
	require.NoError(t, json.Unmarshal(inst.DriftDetail, &persisted))
	require.Equal(t, []string{"c"}, persisted.Added)
	require.Equal(t, []string{"b"}, persisted.Removed)
	require.Equal(t, []string{"a"}, persisted.SchemaChanged)
	require.NotContains(t, string(inst.DriftDetail), "inputSchema", "drift_detail must not carry remote schema text")
	require.NotContains(t, string(inst.DriftDetail), driftSchemaA2)

	// GetDrift 读回已持久化状态。
	reread, err := s.svc.GetDrift(ctx, tenantID, s.inst.ID)
	require.NoError(t, err)
	require.Equal(t, types.PluginDriftDetected, reread.DriftState)
	require.NotNil(t, reread.Detail)
	require.Equal(t, []string{"c"}, reread.Detail.Added)

	// 目录复原 → CheckDrift → none、detail 清空（漂移不是单向棘轮）。
	s.remote.SetTools(driftInstalledTools())
	healed, err := s.svc.CheckDrift(ctx, tenantID, s.inst.ID)
	require.NoError(t, err)
	require.Equal(t, types.PluginDriftNone, healed.DriftState)
	require.Nil(t, healed.Detail)
	inst = s.innerRepo.installations[0]
	require.Equal(t, types.PluginDriftNone, inst.DriftState)
	require.Nil(t, inst.DriftDetail)
}

// TestResolveDriftRebasesSnapshot（计划 Step 1 用例 3，Ruling 保守写分类）：
// 漂移（新增 c + w）后 ResolveDrift：新快照含 c/w；新增工具策略行
// Enabled=false——live MCP 目录不携带治理分类（readOnly/personal-auth 是
// 清单声明域字段），不经清单重抓的 resolve 无法核实新增工具的只读自证，
// 一律保守按写关闭；既有 a 行保持 Enabled=true。drift_state=none、版本与
// 端点不变、物化服务行不动；运行时守卫目录见 a/c/w；无漂移时再次
// ResolveDrift 幂等零写入。
func TestResolveDriftRebasesSnapshot(t *testing.T) {
	const tenantID = uint64(11)
	s := newDriftStack(t, tenantID)
	ctx := context.Background()

	// 漂移：a 不变 + 新增 c（只读替身声明）+ 新增 w（写替身声明）。
	s.remote.SetTools([]plugintest.Tool{
		{Name: "a", Description: "tool a", ReadOnly: true, InputSchema: driftSchemaA},
		{Name: "c", Description: "tool c", ReadOnly: true, InputSchema: upgradeV2WriteSchema},
		{Name: "w", Description: "write w", ReadOnly: false, InputSchema: upgradeV2WriteSchema},
	})

	resolved, err := s.svc.ResolveDrift(ctx, tenantID, "admin-1", s.inst.ID)
	require.NoError(t, err)

	// 新快照生效：目录 = [a, c, w]；漂移清零。
	require.Equal(t, types.PluginDriftNone, resolved.DriftState)
	require.ElementsMatch(t, []string{"a", "c", "w"}, installationToolViewNames(resolved.Tools))
	inst := s.innerRepo.installations[0]
	require.Equal(t, types.PluginDriftNone, inst.DriftState)
	require.Nil(t, inst.DriftDetail)
	require.Equal(t, types.PluginDriftNone, resolved.DriftState)
	require.ElementsMatch(t, []string{"a", "c", "w"}, snapshotToolNames(inst.ToolsSnapshot))

	// 版本号不变、端点不变（resolve 是「接受当前目录为已核验快照」，不是升级）。
	require.Equal(t, "1.0.0", inst.AcceptedVersion)
	require.Equal(t, s.remote.BaseURL()+"/mcp", inst.EndpointURL)

	// 策略行：既有 a/b 保持原值（b 已被移出快照，其行残留且无害——快照
	// 过滤先于策略查询，与 AcceptUpgrade 对移除工具的同语义）；新增 c/w
	// 一律保守 Enabled=false。
	policies := s.policyRows(t, tenantID)
	require.Len(t, policies, 4)
	require.True(t, policies["a"].Enabled, "the existing row keeps the admin's verdict")
	require.True(t, policies["b"].Enabled, "the REMOVED tool's residual row is inert (snapshot filtering precedes policy)")
	require.False(t, policies["c"].Enabled,
		"a NEW tool cannot self-certify read-only over live ListTools — it lands disabled (conservative Ruling)")
	require.False(t, policies["w"].Enabled, "a NEW write tool stays disabled after review")

	// 运行时守卫目录见 a/c/w（resolve 后新快照对成员会话生效）。
	require.ElementsMatch(t, []string{"a", "c", "w"}, snapshotToolNames(s.guardSnapshot(t, tenantID)))

	// 守卫过滤零漂移：live 目录与新快照一致 → T09 真实
	// FilterToolsBySnapshot 全保留（成员对话恢复可用的 fake 层等价断言）。
	guardSnap := &tools.PluginRuntimeSnapshot{
		InstallationID: s.inst.ID,
		Tools:          s.guardSnapshot(t, tenantID),
	}
	live := []*types.MCPTool{
		{Name: "a", Description: "tool a", InputSchema: json.RawMessage(driftSchemaA)},
		{Name: "c", Description: "tool c", InputSchema: json.RawMessage(upgradeV2WriteSchema)},
		{Name: "w", Description: "write w", InputSchema: json.RawMessage(upgradeV2WriteSchema)},
	}
	filtered, err := tools.FilterToolsBySnapshot(guardSnap, live)
	require.NoError(t, err)
	require.Len(t, filtered, 3)

	// 幂等：无漂移时再次 ResolveDrift → 零写入（升级写不重复落）。
	acceptedWritesBefore := len(s.pluginRepo.acceptedWrites)
	driftWritesBefore := len(s.pluginRepo.driftWrites)
	again, err := s.svc.ResolveDrift(ctx, tenantID, "admin-1", s.inst.ID)
	require.NoError(t, err)
	require.Equal(t, types.PluginDriftNone, again.DriftState)
	require.Len(t, s.pluginRepo.acceptedWrites, acceptedWritesBefore, "an already-clean resolve must not rewrite the installation row")
	require.Len(t, s.pluginRepo.driftWrites, driftWritesBefore)
}

// TestResolveDriftRejectsWhenEndpointBroken（计划 Step 1 用例 4）：远端
// 不可达时 resolve 拒绝（不接受一个无法核验的目录为新快照）且快照/漂移
// 状态不变。
func TestResolveDriftRejectsWhenEndpointBroken(t *testing.T) {
	const tenantID = uint64(11)
	s := newDriftStack(t, tenantID)
	ctx := context.Background()

	// 制造漂移并置位（走 CheckDrift，漂移态真实落库）。
	s.remote.SetTools([]plugintest.Tool{
		{Name: "a", Description: "tool a", ReadOnly: true, InputSchema: driftSchemaA2},
	})
	_, err := s.svc.CheckDrift(ctx, tenantID, s.inst.ID)
	require.NoError(t, err)
	drifted := cloneInstallation(s.innerRepo.installations[0])
	require.Equal(t, types.PluginDriftDetected, drifted.DriftState)

	// 远端不可达：安装行端点指向一个已关闭的本地端口（SSRF 白名单已由
	// plugintest.Start 放行 127.0.0.1）。
	dead := httptest.NewServer(http.NotFoundHandler())
	deadURL := dead.URL + "/mcp"
	dead.Close()
	s.innerRepo.installations[0].EndpointURL = deadURL

	_, err = s.svc.ResolveDrift(ctx, tenantID, "admin-1", s.inst.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, service.ErrDriftEndpointUnreachable)

	// 状态不变：快照/版本/漂移态逐字段回到拒绝前（除被测试改写的端点列）。
	after := cloneInstallation(s.innerRepo.installations[0])
	after.EndpointURL = deadURL
	drifted.EndpointURL = deadURL
	require.Equal(t, drifted, after, "a rejected resolve must leave the installation untouched")
	require.Empty(t, s.pluginRepo.acceptedWrites, "a rejected resolve must write nothing")
}

// installationToolViewNames 提取安装视图行的工具名。
func installationToolViewNames(tools []types.PluginInstallationToolView) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}

// Compile-time guard: drift_test.go 的栈组装依赖 upgradeV1SearchSchema /
// upgradeV2WriteSchema / snapshotToolNames / cloneInstallation /
// marshalManifest / newManagerLister（upgrade_diff_test.go /
// upgrade_accept_test.go / install_service_test.go 同包提供）。
var _ = upgradeV1SearchSchema
var _ = time.Now
