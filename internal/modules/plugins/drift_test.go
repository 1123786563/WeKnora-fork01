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
	"errors"
	"fmt"
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
	driftWrites    []driftWrite
	updateDriftErr error
	// conditionalWrites 记录 UpdateDriftIfToolsDigest 调用（T17-OCR1-F2：
	// best-effort 置位走基线前校的条件写）；applied 标记条件命中与否。
	conditionalWrites []driftConditionalWrite
	// staleRead 非 nil 时 GetByServiceID 返回它——模拟 provider 读行后、
	// 落库前行快照被并发重定基推进的竞态窗口（T17-OCR1-F2 回归）。
	staleRead *types.PluginInstallation
}

// driftConditionalWrite 记录一次基线前校的条件漂移写。
type driftConditionalWrite struct {
	expectDigest string
	state        string
	applied      bool
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

// UpdateDriftIfToolsDigest 是条件漂移写的同构内存实现：行的 tools_digest
// 仍等于 expectDigest 才落（零行 = 基线已前进，applied=false，非错误）。
func (r *driftRepo) UpdateDriftIfToolsDigest(
	_ context.Context, tenantID uint64, id, expectToolsDigest, state string, detail json.RawMessage,
) (bool, error) {
	for _, inst := range r.installations {
		if inst.TenantID == tenantID && inst.ID == id {
			if inst.ToolsDigest != expectToolsDigest {
				r.conditionalWrites = append(r.conditionalWrites, driftConditionalWrite{
					expectDigest: expectToolsDigest, state: state, applied: false,
				})
				return false, nil
			}
			inst.DriftState = state
			if detail == nil {
				inst.DriftDetail = nil
			} else {
				inst.DriftDetail = append(json.RawMessage(nil), detail...)
			}
			r.conditionalWrites = append(r.conditionalWrites, driftConditionalWrite{
				expectDigest: expectToolsDigest, state: state, applied: true,
			})
			return true, nil
		}
	}
	return false, nil
}

// GetByServiceID 遮蔽嵌入实现：staleRead 在场时返回过期视图（竞态模拟）。
func (r *driftRepo) GetByServiceID(ctx context.Context, tenantID uint64, serviceID string) (*types.PluginInstallation, error) {
	if r.staleRead != nil {
		return r.staleRead, nil
	}
	return r.upgradeAcceptRepo.GetByServiceID(ctx, tenantID, serviceID)
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
	// setLister 运行时替换服务持有的 EndpointLister（T17-OCR1-F6 回归：
	// 注入重名/超限/坏名目录的 stub，观察 resolve 的卫生门拒绝）。
	setLister func(plugins.EndpointLister)
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
	// 可替换 lister：默认真实 manager lister，测试可热换 stub。
	var listerFn plugins.EndpointLister = newManagerLister(manager)
	stack.setLister = func(fn plugins.EndpointLister) { listerFn = fn }
	stack.svc = service.NewPluginService(pluginRepo, mcpSvcService, mcpRepo, approvalSvc,
		func(ctx context.Context, transportType, endpointURL string) ([]*types.MCPTool, error) {
			return listerFn(ctx, transportType, endpointURL)
		}, nil, nil)

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
		detail, err := plugins.DiffLiveAgainstSnapshot(live, snap)
		require.NoError(t, err)
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
		detail, err := plugins.DiffLiveAgainstSnapshot(live, snap)
		require.NoError(t, err)
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
		detail, err := plugins.DiffLiveAgainstSnapshot(live, snap)
		require.NoError(t, err)
		require.NotNil(t, detail)
		require.Empty(t, detail.Added)
		require.Empty(t, detail.Removed)
		require.Empty(t, detail.SchemaChanged)
		require.Empty(t, detail.DescriptionChanged)
		require.False(t, detail.HasDrift())
	})
}

// TestDiffLiveAgainstSnapshotGatesUnvettedLiveDirectory（OCR R1 F15）：
// Diff 是唯一直接消费未受限 live 目录的路径——超限（>1024）、重名、名字
// 不过卫生规则的目录必须拒绝产出判定（返回 error），不得把恶意端点的海
// 量名单/重复名 last-wins 持久化进 drift_detail 并回显管理员。
func TestDiffLiveAgainstSnapshotGatesUnvettedLiveDirectory(t *testing.T) {
	snap := []types.PluginToolSnapshot{
		{Name: "a", Description: "tool a", InputSchemaDigest: plugins.ToolSchemaDigest([]byte(driftSchemaA))},
	}

	t.Run("oversized directory is refused", func(t *testing.T) {
		live := make([]*types.MCPTool, 1025)
		for i := range live {
			live[i] = &types.MCPTool{Name: fmt.Sprintf("t%04d", i), InputSchema: json.RawMessage(`{}`)}
		}
		detail, err := plugins.DiffLiveAgainstSnapshot(live, snap)
		require.Error(t, err, "an unvetted oversized directory must not yield a drift verdict")
		require.Nil(t, detail)
	})

	t.Run("duplicate live names are refused", func(t *testing.T) {
		live := []*types.MCPTool{
			{Name: "dup", InputSchema: json.RawMessage(`{}`)},
			{Name: "dup", InputSchema: json.RawMessage(`{"x":1}`)},
		}
		detail, err := plugins.DiffLiveAgainstSnapshot(live, snap)
		require.Error(t, err)
		require.Nil(t, detail)
	})

	t.Run("unhygienic live name is refused", func(t *testing.T) {
		live := []*types.MCPTool{{Name: "bad\nname", InputSchema: json.RawMessage(`{}`)}}
		detail, err := plugins.DiffLiveAgainstSnapshot(live, snap)
		require.Error(t, err)
		require.Nil(t, detail)
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

// TestResolveDriftIdempotentShortCircuitCompletesMissingPolicyRows（B+A
// 裁决 #1 同族）：resolve 的幂等短路（漂移 none + rebased digest 相等 → 零
// 写入）与 AcceptUpgrade 同款崩溃窗口——5a 快照落库后 5b 策略行写完前被
// 硬杀，重试命中短路零写入，缺行写工具按运行时门默认启用。短路前必须校验
// rebased 快照每工具都有显式策略行，缺失则落入增量补齐。
func TestResolveDriftIdempotentShortCircuitCompletesMissingPolicyRows(t *testing.T) {
	const tenantID = uint64(11)
	s := newDriftStack(t, tenantID)
	ctx := context.Background()

	// 漂移（新增 c + w）后 resolve：快照 [a,c,w]、c/w 行落地。
	s.remote.SetTools([]plugintest.Tool{
		{Name: "a", Description: "tool a", ReadOnly: true, InputSchema: driftSchemaA},
		{Name: "c", Description: "tool c", ReadOnly: true, InputSchema: upgradeV2WriteSchema},
		{Name: "w", Description: "write w", ReadOnly: false, InputSchema: upgradeV2WriteSchema},
	})
	_, err := s.svc.ResolveDrift(ctx, tenantID, "admin-1", s.inst.ID)
	require.NoError(t, err)

	// 崩溃窗口等价形态：5a 已落、5b 中途硬杀——删除写工具 w 的行。
	require.NoError(t, s.pluginRepo.DeleteInstallationToolPolicies(ctx, tenantID, s.inst.ServiceID, []string{"w"}))
	require.NotContains(t, s.policyRows(t, tenantID), "w", "fixture: the crash window leaves the write tool without a policy row")

	// 重试 resolve：不得短路零写入放行——缺行落入增量补齐。
	retried, err := s.svc.ResolveDrift(ctx, tenantID, "admin-1", s.inst.ID)
	require.NoError(t, err)
	require.Equal(t, types.PluginDriftNone, retried.DriftState)

	policies := s.policyRows(t, tenantID)
	require.Contains(t, policies, "w",
		"the retried resolve must complete the missing policy row instead of idempotently no-op'ing")
	require.False(t, policies["w"].Enabled, "the completed write-tool row must land disabled, not ride the runtime gate's default-enabled verdict")
	require.True(t, policies["a"].Enabled, "the completion is incremental — the existing verdict stays")
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

// TestResolveDriftCompensationKeepsDriftState（T17-OCR1-F1）：resolve 中途
// 失败补偿回写旧快照时，必须一并恢复调用前的漂移判定与已持久化的漂移
// 明细（审计材料）——UpdateInstallationAccepted 无条件清 drift 列，而补偿
// 语义是回到调用前：此时端点仍偏离旧快照，行上必须仍是 detected + 原
// detail，否则成员目录被 ErrPluginDrift 阻断而 GetDrift 显示 none。
func TestResolveDriftCompensationKeepsDriftState(t *testing.T) {
	const tenantID = uint64(11)
	s := newDriftStack(t, tenantID)
	ctx := context.Background()

	// 漂移（a schema 变 + 新增 c——新增工具驱动 5b 策略行写入，是补偿
	// 路径的触发面）→ CheckDrift 置位 detected 并落明细。
	s.remote.SetTools([]plugintest.Tool{
		{Name: "a", Description: "tool a", ReadOnly: true, InputSchema: driftSchemaA2},
		{Name: "b", Description: "tool b", ReadOnly: true, InputSchema: upgradeV1SearchSchema},
		{Name: "c", Description: "tool c", ReadOnly: true, InputSchema: upgradeV2WriteSchema},
	})
	_, err := s.svc.CheckDrift(ctx, tenantID, s.inst.ID)
	require.NoError(t, err)
	driftedState := cloneInstallation(s.innerRepo.installations[0])
	require.Equal(t, types.PluginDriftDetected, driftedState.DriftState)
	require.NotNil(t, driftedState.DriftDetail)
	driftedDetail := append(json.RawMessage(nil), driftedState.DriftDetail...)

	// 5b 策略行写入失败 → 补偿。
	s.approvalRepo.upsertErr = errors.New("injected policy fault on resolve")
	_, err = s.svc.ResolveDrift(ctx, tenantID, "admin-1", s.inst.ID)
	require.Error(t, err)

	// 补偿后：快照回旧（既有语义）+ 漂移判定/明细恢复调用前（F1 修复面）。
	after := cloneInstallation(s.innerRepo.installations[0])
	require.Equal(t, driftedState.ToolsDigest, after.ToolsDigest, "the snapshot must roll back")
	require.ElementsMatch(t, snapshotToolNames(driftedState.ToolsSnapshot), snapshotToolNames(after.ToolsSnapshot))
	require.Equal(t, types.PluginDriftDetected, after.DriftState,
		"compensation must restore the drift VERDICT — the endpoint still deviates from the rolled-back snapshot (T17-OCR1-F1)")
	require.Equal(t, string(driftedDetail), string(after.DriftDetail),
		"compensation must restore the persisted drift detail document (audit material)")
}

// TestDriftMarkerConditionalWriteSkipsRebasedRow（T17-OCR1-F2）：best-effort
// 置位与重定基的竞态窗口——provider 读到旧基线行（digest D1）后、落库前，
// 并发 ResolveDrift 已把行重定基（快照含 c、digest D2、state=none）：此时
// 基于 D1 基线算出的 detected+明细不得落库到已重定基的行上（管理端会看到
// 虚假 detected、明细描述已不存在的基线，且置位后 marker 不再自愈）。
// 修复形态：置位走「以 tools_digest 为前置的条件更新」，基线已前进则零写。
func TestDriftMarkerConditionalWriteSkipsRebasedRow(t *testing.T) {
	const tenantID = uint64(11)
	s := newDriftStack(t, tenantID)
	ctx := context.Background()

	// 调用前基线（快照 [a,b]、digest D1、state=none）——竞态模拟里
	// provider 读到的过期视图；必须在 resolve 之前保存（栈内 inst 与行
	// 存储共享指针，resolve 会把它重定基）。
	staleBaseline := cloneInstallation(s.inst)
	staleBaseline.DriftState = types.PluginDriftNone
	staleBaseline.DriftDetail = nil

	// 行的真实状态：已被（模拟的）并发 resolve 重定基到 [a, c]——digest
	// D2、state=none。
	s.remote.SetTools([]plugintest.Tool{
		{Name: "a", Description: "tool a", ReadOnly: true, InputSchema: driftSchemaA},
		{Name: "c", Description: "tool c", ReadOnly: true, InputSchema: upgradeV2WriteSchema},
	})
	live := []*types.MCPTool{
		{Name: "a", Description: "tool a", InputSchema: json.RawMessage(driftSchemaA)},
		{Name: "c", Description: "tool c", InputSchema: json.RawMessage(upgradeV2WriteSchema)},
	}
	rebased, err := s.svc.ResolveDrift(ctx, tenantID, "admin-1", s.inst.ID)
	require.NoError(t, err)
	rebasedState := cloneInstallation(s.innerRepo.installations[0])
	require.Equal(t, types.PluginDriftNone, rebasedState.DriftState)
	require.Equal(t, "1.0.0", rebased.Version)

	// 竞态模拟：provider/标记器读到的是重定基前的过期行——staleRead
	// 遮蔽 GetByServiceID。
	s.pluginRepo.staleRead = staleBaseline

	// 目录实况 [a, c]：按 D1 过期基线算出漂移（Added=[c], Removed=[b]）。
	// 标记器必须以 D1 为前置条件落库——行的 digest 已是 D2 → 零写放弃。
	guard := service.PluginSnapshotLookupWithDriftMarking(s.pluginRepo, func(
		_ context.Context, _, _ string,
	) ([]*types.MCPTool, error) {
		return live, nil
	})
	snap, err := guard(ctx, tenantID, s.inst.ServiceID)
	require.NoError(t, err)
	require.NotNil(t, snap)

	current := s.innerRepo.installations[0]
	require.Equal(t, types.PluginDriftNone, current.DriftState,
		"a stale-baseline detected verdict must NOT land on the rebased row (T17-OCR1-F2)")
	require.Nil(t, current.DriftDetail)
	require.Empty(t, s.pluginRepo.driftWrites,
		"the unconditional UpdateDrift seam must not be used by the best-effort marker")
	require.NotEmpty(t, s.pluginRepo.conditionalWrites,
		"the marker must have attempted the baseline-guarded write")
	require.False(t, s.pluginRepo.conditionalWrites[len(s.pluginRepo.conditionalWrites)-1].applied,
		"the conditional write must observe the moved baseline and apply zero rows")
	require.Equal(t, rebasedState.ToolsDigest, current.ToolsDigest)
}

// TestValidateLiveDirectoryForRebase（T17-OCR1-F6 纯函数面）：重定基入口
// 对不可信 live 目录的卫生门与安装侧 BuildVerifiedSnapshot 同一套——上限
// maxLiveTools、重名拒绝、名字过 validateName；空/合法目录放行。
func TestValidateLiveDirectoryForRebase(t *testing.T) {
	t.Run("accepts a clean directory", func(t *testing.T) {
		live := []*types.MCPTool{
			{Name: "a", Description: "tool a", InputSchema: json.RawMessage(driftSchemaA)},
			{Name: "b", Description: "tool b", InputSchema: json.RawMessage(upgradeV1SearchSchema)},
		}
		require.NoError(t, plugins.ValidateLiveDirectoryForRebase(live))
		require.NoError(t, plugins.ValidateLiveDirectoryForRebase(nil))
	})

	t.Run("rejects duplicate names", func(t *testing.T) {
		live := []*types.MCPTool{
			{Name: "a", Description: "one", InputSchema: json.RawMessage(driftSchemaA)},
			{Name: "a", Description: "two", InputSchema: json.RawMessage(driftSchemaA)},
		}
		err := plugins.ValidateLiveDirectoryForRebase(live)
		require.Error(t, err)
		require.Contains(t, err.Error(), "duplicate tool name")
	})

	t.Run("rejects unhygienic names", func(t *testing.T) {
		live := []*types.MCPTool{
			{Name: "bad\x02name", Description: "ctrl", InputSchema: json.RawMessage(driftSchemaA)},
		}
		err := plugins.ValidateLiveDirectoryForRebase(live)
		require.Error(t, err)
	})

	t.Run("rejects an oversized directory", func(t *testing.T) {
		live := make([]*types.MCPTool, 1025)
		for i := range live {
			live[i] = &types.MCPTool{Name: fmt.Sprintf("t%d", i), InputSchema: json.RawMessage(driftSchemaA)}
		}
		err := plugins.ValidateLiveDirectoryForRebase(live)
		require.Error(t, err)
		require.Contains(t, err.Error(), "exceeding the maximum")
	})
}

// TestResolveDriftRejectsUnhygienicLiveDirectory（T17-OCR1-F6 服务面）：
// 恶意/劣化端点的重名目录不得被铸成已接受快照——resolve 在重定基前过
// 卫生门，确定性拒绝（4xx 语义）且零写入。
func TestResolveDriftRejectsUnhygienicLiveDirectory(t *testing.T) {
	const tenantID = uint64(11)
	s := newDriftStack(t, tenantID)
	ctx := context.Background()

	before := cloneInstallation(s.innerRepo.installations[0])
	acceptedWritesBefore := len(s.pluginRepo.acceptedWrites)

	// 注入重名目录（真实 MCP ListTools 下游已按名去重，这里是卫生门的
	// 直接注入面——与 BuildVerifiedSnapshot 对清单路径的防备同构）。
	s.setLister(func(_ context.Context, _, _ string) ([]*types.MCPTool, error) {
		return []*types.MCPTool{
			{Name: "a", Description: "one", InputSchema: json.RawMessage(driftSchemaA)},
			{Name: "a", Description: "two", InputSchema: json.RawMessage(driftSchemaA2)},
		}, nil
	})

	_, err := s.svc.ResolveDrift(ctx, tenantID, "admin-1", s.inst.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, service.ErrPluginVerifyFailed)
	require.Contains(t, err.Error(), "duplicate tool name")

	// 零写入：安装行逐字段不变、无升级写。
	require.Equal(t, before, cloneInstallation(s.innerRepo.installations[0]))
	require.Len(t, s.pluginRepo.acceptedWrites, acceptedWritesBefore)
}

// Compile-time guard: drift_test.go 的栈组装依赖 upgradeV1SearchSchema /
// upgradeV2WriteSchema / snapshotToolNames / cloneInstallation /
// marshalManifest / newManagerLister（upgrade_diff_test.go /
// upgrade_accept_test.go / install_service_test.go 同包提供）。
var _ = upgradeV1SearchSchema
var _ = time.Now
