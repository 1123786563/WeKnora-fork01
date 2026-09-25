package plugins_test

// T16（plan 08 Task 16）：管理员接受插件升级——fake 层语义测试。
//
// 覆盖计划 Step 1 的三个用例：
//  1. TestAcceptUpgradeSwitchesSnapshotAndEndpoint：重核通过后安装行切换
//     （版本/端点/快照/digest/漂移重置）、物化服务仅 URL 切换（Name/ID 不变）、
//     策略行增量（新只读 Enabled=true、新写 Enabled=false、既有行保持原值）。
//  2. TestAcceptUpgradeIdempotentAndFingerprintGuard：同 fingerprint 二次接受
//     幂等（零写入、策略行数不变）；远端再变（v3）后旧 fingerprint 拒绝
//     （"candidate changed since preview"）且安装行零变化。
//  3. TestAcceptUpgradeFailureKeepsOldVersion：远端不可达零写入；物化/策略
//     写失败补偿回写保旧版（安装行、物化 URL、运行时守卫目录均回到 v1）。
//
// 外部测试包 plugins_test 并 import plugintest（总索引「测试包名约定」）。
// fake 基建复用 install_service_test.go / upgrade_diff_test.go（同测试包）；
// 本文件以嵌入包装注入升级写方法与 URL 持久化 Update（T06 时代的 fake 先于
// 升级切片存在，包装而非改写）。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	internalmcp "github.com/Tencent/WeKnora/internal/modules/airesource/mcp"
	"github.com/Tencent/WeKnora/internal/modules/plugins/plugintest"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const upgradeAcceptPluginID = "com.example.upgrade-accept"

// acceptedUpgradeWrite 记录一次安装行升级写入（正向切换或补偿回写）。
type acceptedUpgradeWrite struct {
	version  string
	endpoint string
	digest   string
}

// upgradeAcceptRepo 在 installPreviewRepo 之上补齐升级写方法（对真实 gorm
// pluginRepository.UpdateInstallationAccepted / DeleteInstallationToolPolicies
// 的同构内存实现：版本/端点/快照/digest 落行 + 漂移重置；按名删除本轮回写的
// 策略行），并记录全部调用供补偿序断言。
type upgradeAcceptRepo struct {
	*installPreviewRepo
	acceptedWrites   []acceptedUpgradeWrite
	approvalRows     *fakeInstallApprovalRepo
	deletedPolicies  []string
	deletePolicyFail bool
}

func (r *upgradeAcceptRepo) UpdateInstallationAccepted(
	_ context.Context, tenantID uint64, id, acceptedVersion, endpointURL string,
	snapshot types.PluginPreviewTools, toolsDigest string,
) error {
	for _, inst := range r.installations {
		if inst.TenantID == tenantID && inst.ID == id {
			inst.AcceptedVersion = acceptedVersion
			inst.EndpointURL = endpointURL
			inst.ToolsSnapshot = append(types.PluginPreviewTools(nil), snapshot...)
			inst.ToolsDigest = toolsDigest
			inst.DriftState = types.PluginDriftNone
			inst.DriftDetail = nil
			r.acceptedWrites = append(r.acceptedWrites, acceptedUpgradeWrite{
				version: acceptedVersion, endpoint: endpointURL, digest: toolsDigest,
			})
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

// DeleteInstallationToolPolicies 删除本次接受新写入的策略行（补偿清理）——
// 内存实现直接从审批 fake 的行集移除。
func (r *upgradeAcceptRepo) DeleteInstallationToolPolicies(
	_ context.Context, tenantID uint64, serviceID string, toolNames []string,
) error {
	if r.deletePolicyFail {
		return fmt.Errorf("injected policy-cleanup fault")
	}
	for _, name := range toolNames {
		delete(r.approvalRows.rows, approvalKey(serviceID, name))
		r.deletedPolicies = append(r.deletedPolicies, name)
	}
	return nil
}

// upgradeAcceptApprovalRepo 包装 fakeInstallApprovalRepo，按调用序号注入
// UpsertPolicy 故障（OCR T16-OCR1-F2：制造「前一工具已落行、后一工具失败」
// 的中途中断窗口——快照迭代顺序不应影响测试确定性，故按第 N 次调用注入而
// 非按工具名），断言补偿删除已落行的新策略行。
type upgradeAcceptApprovalRepo struct {
	*fakeInstallApprovalRepo
	failOnNthUpsert int // 0=不注入；N=第 N 次 UpsertPolicy 调用失败
	upsertCalls     int
}

func (r *upgradeAcceptApprovalRepo) UpsertPolicy(
	ctx context.Context, tenantID uint64, serviceID, toolName string, patch types.MCPToolPolicyPatch,
) error {
	r.upsertCalls++
	if r.failOnNthUpsert > 0 && r.upsertCalls == r.failOnNthUpsert {
		return fmt.Errorf("injected policy fault on call #%d (%s)", r.failOnNthUpsert, toolName)
	}
	return r.fakeInstallApprovalRepo.UpsertPolicy(ctx, tenantID, serviceID, toolName, patch)
}

// upgradeAcceptMCPRepo 包装 fakeInstallMCPServiceRepo：Update 全字段持久化
// （含 URL——T06 fake 只拷 enabled/name/description，升级切片的端点切换
// 断言需要 URL 落行），failUpdateAt 注入第 N 次 Update 失败（0=不注入），
// onUpdate 在每次 Update 入口触发一次（并发串行化测试的 7b 中途挂钩）。
type upgradeAcceptMCPRepo struct {
	*fakeInstallMCPServiceRepo
	updates      int
	failUpdateAt int
	onUpdate     func()
}

func (r *upgradeAcceptMCPRepo) Update(_ context.Context, svc *types.MCPService) error {
	r.updates++
	if r.onUpdate != nil {
		r.onUpdate()
	}
	if r.failUpdateAt > 0 && r.updates == r.failUpdateAt {
		return fmt.Errorf("injected materialization fault")
	}
	for _, existing := range r.services {
		if existing.TenantID == svc.TenantID && existing.ID == svc.ID {
			*existing = *cloneMCPService(svc)
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

// upgradeAcceptStack：plugintest v1/v2 双远端（端点隔离，endpoint 切换的
// 前提）+ 可热换清单 host + 包装仓储 + 真实 PluginService，落一条已确认
// 安装。
type upgradeAcceptStack struct {
	svc          interfaces.PluginService
	pluginRepo   *upgradeAcceptRepo
	innerRepo    *installPreviewRepo
	mcpRepo      *upgradeAcceptMCPRepo
	approvalRepo *upgradeAcceptApprovalRepo
	approvalSvc  interfaces.MCPToolApprovalService
	manager      *internalmcp.MCPManager
	remoteV1     *plugintest.Server
	remoteV2     *plugintest.Server
	manifestHost *httptest.Server
	manifestNext *[]byte
	inst         *types.PluginInstallation
	closerCalls  int
}

// v1/v2 默认目录：v1 = 1 只读 search；v2 = search（同 schema）+ 新增只读
// lookup + 新增写 create_issue（独立远端 → 端点切换）。
func upgradeAcceptV1Tools() []plugintest.Tool {
	return []plugintest.Tool{{
		Name: "search", Description: "search v1", ReadOnly: true,
		InputSchema: upgradeV1SearchSchema,
	}}
}

func upgradeAcceptV2Tools() []plugintest.Tool {
	return []plugintest.Tool{
		{Name: "search", Description: "search v1", ReadOnly: true,
			InputSchema: upgradeV1SearchSchema},
		{Name: "lookup", Description: "lookup v2", ReadOnly: true,
			InputSchema: upgradeV1SearchSchema},
		{Name: "create_issue", Description: "create v2", ReadOnly: false,
			InputSchema: upgradeV2WriteSchema},
	}
}

func newUpgradeAcceptStack(t *testing.T, tenantID uint64) *upgradeAcceptStack {
	return newUpgradeAcceptStackWithTools(t, tenantID, upgradeAcceptV1Tools(), upgradeAcceptV2Tools())
}

func newUpgradeAcceptStackWithTools(t *testing.T, tenantID uint64, v1Tools, v2Tools []plugintest.Tool) *upgradeAcceptStack {
	t.Helper()

	remoteV1 := plugintest.New()
	remoteV1.PluginID = upgradeAcceptPluginID
	remoteV1.Version = "1.0.0"
	remoteV1.SetTools(v1Tools)
	remoteV1.Start(t)

	remoteV2 := plugintest.New()
	remoteV2.PluginID = upgradeAcceptPluginID
	remoteV2.Version = "2.0.0"
	remoteV2.SetTools(v2Tools)
	remoteV2.Start(t)

	v1Manifest, err := marshalManifest(remoteV1)
	require.NoError(t, err)
	manifestJSON := v1Manifest
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
	approvalRepo := &upgradeAcceptApprovalRepo{fakeInstallApprovalRepo: &fakeInstallApprovalRepo{}}
	innerRepo.mcpRepo = mcpRepo.fakeInstallMCPServiceRepo
	innerRepo.approvalRepo = approvalRepo.fakeInstallApprovalRepo
	pluginRepo := &upgradeAcceptRepo{installPreviewRepo: innerRepo, approvalRows: approvalRepo.fakeInstallApprovalRepo}

	mcpSvcService := service.NewMCPServiceService(mcpRepo, nil, nil)
	approvalSvc := service.NewMCPToolApprovalService(approvalRepo, mcpRepo)

	stack := &upgradeAcceptStack{
		pluginRepo:   pluginRepo,
		innerRepo:    innerRepo,
		mcpRepo:      mcpRepo,
		approvalRepo: approvalRepo,
		approvalSvc:  approvalSvc,
		manager:      manager,
		remoteV1:     remoteV1,
		remoteV2:     remoteV2,
		manifestHost: manifestHost,
		manifestNext: &manifestJSON,
	}
	stack.svc = service.NewPluginService(pluginRepo, mcpSvcService, mcpRepo, approvalSvc,
		newManagerLister(manager),
		service.MCPClientCloser(func(string) { stack.closerCalls++ }),
		nil)

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

// switchManifestToV2 把清单文档切到 v2（版本 2.0.0、端点指向 v2 远端、目录
// 新增 lookup/create_issue）。
func (s *upgradeAcceptStack) switchManifestToV2(t *testing.T) {
	t.Helper()
	v2Manifest, err := marshalManifest(s.remoteV2)
	require.NoError(t, err)
	*s.manifestNext = v2Manifest
}

// previewV2 切清单到 v2 并返回升级预览（候选指纹来自 T14 只读预览）。
func (s *upgradeAcceptStack) previewV2(t *testing.T, tenantID uint64) *types.PluginUpgradePreviewResult {
	t.Helper()
	s.switchManifestToV2(t)
	resp, err := s.svc.PreviewUpgrade(context.Background(), tenantID, s.inst.ID)
	require.NoError(t, err)
	return resp
}

// materialized 返回安装行绑定的物化服务（必须恰好一行）。
func (s *upgradeAcceptStack) materialized(t *testing.T) *types.MCPService {
	t.Helper()
	require.Len(t, s.mcpRepo.services, 1)
	return s.mcpRepo.services[0]
}

// policyRows 返回物化服务的全部策略行。
func (s *upgradeAcceptStack) policyRows(t *testing.T, tenantID uint64) map[string]*types.MCPToolApproval {
	t.Helper()
	rows, err := s.approvalRepo.ListByService(context.Background(), tenantID, s.inst.ServiceID)
	require.NoError(t, err)
	out := map[string]*types.MCPToolApproval{}
	for _, row := range rows {
		out[row.ToolName] = row
	}
	return out
}

// guardSnapshot 返回运行时快照守卫当前供给的目录（RegisterMCPTools 的
// PluginSnapshotProvider 数据源，T09）。
func (s *upgradeAcceptStack) guardSnapshot(t *testing.T, tenantID uint64) []types.PluginToolSnapshot {
	t.Helper()
	lookup := service.PluginSnapshotLookup(s.pluginRepo)
	snap, err := lookup(context.Background(), tenantID, s.inst.ServiceID)
	require.NoError(t, err)
	require.NotNil(t, snap)
	return snap.Tools
}

// snapshotToolNames 提取快照内的工具名。
func snapshotToolNames(tools []types.PluginToolSnapshot) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}

// TestAcceptUpgradeSwitchesSnapshotAndEndpoint（计划 Step 1 用例 1）：
// v1（1 只读工具）已安装、search 策略行预置停用、漂移态预置 detected →
// 清单切 v2 → AcceptUpgrade(预览指纹) 成功：
//   - 安装行：accepted_version=2.0.0、endpoint_url=v2 端点、tools_digest
//     重算、快照 = search+lookup+create_issue、drift_state 重置 none；
//   - 物化服务：URL 切到 v2 端点，Name/ID 不变（会话内 server_id 稳定）；
//   - 策略行增量：新只读 lookup Enabled=true、新写 create_issue
//     Enabled=false；既有 search 行保持预置的 Enabled=false 不被覆盖；
//   - URL 切换触发缓存客户端回收（closer 调用）；
//   - 运行时守卫目录切到 v2（成员新会话只见新版工具的 fake 层等价）。
func TestAcceptUpgradeSwitchesSnapshotAndEndpoint(t *testing.T) {
	const tenantID = uint64(9)
	s := newUpgradeAcceptStack(t, tenantID)
	ctx := context.Background()

	// 既有 search 行预置停用（管理员的既有启停决定必须保留）。
	disabled := false
	require.NoError(t, s.approvalSvc.SetPolicy(ctx, tenantID, s.inst.ServiceID, "search", nil, &disabled))
	// 漂移态预置 detected（漂移重置是接受写入的一部分）。
	s.inst.DriftState = types.PluginDriftDetected
	s.inst.DriftDetail = json.RawMessage(`{"reason":"preset"}`)

	resp := s.previewV2(t, tenantID)
	svcBefore := s.materialized(t)
	nameBefore, idBefore := svcBefore.Name, svcBefore.ID

	accepted, err := s.svc.AcceptUpgrade(ctx, tenantID, "admin-1", s.inst.ID, resp.CandidateFingerprint)
	require.NoError(t, err)

	// 安装行切换。
	require.Equal(t, "2.0.0", accepted.Version)
	require.Equal(t, s.remoteV2.BaseURL()+"/mcp", accepted.EndpointURL)
	require.Equal(t, types.PluginDriftNone, accepted.DriftState)
	inst := s.innerRepo.installations[0]
	require.Equal(t, "2.0.0", inst.AcceptedVersion)
	require.Equal(t, s.remoteV2.BaseURL()+"/mcp", inst.EndpointURL)
	require.Equal(t, resp.CandidateToolsDigest, inst.ToolsDigest)
	require.Equal(t, types.PluginDriftNone, inst.DriftState)
	require.Nil(t, inst.DriftDetail)
	require.ElementsMatch(t, []string{"search", "lookup", "create_issue"}, snapshotToolNames(inst.ToolsSnapshot))

	// 物化服务仅 URL 切换，Name/ID 不变。
	svcAfter := s.materialized(t)
	require.Equal(t, idBefore, svcAfter.ID)
	require.Equal(t, nameBefore, svcAfter.Name)
	require.NotNil(t, svcAfter.URL)
	require.Equal(t, s.remoteV2.BaseURL()+"/mcp", *svcAfter.URL)

	// 策略行增量：既有行保持原值，新行按读写分类落值。
	policies := s.policyRows(t, tenantID)
	require.Len(t, policies, 3)
	require.False(t, policies["search"].Enabled, "the pre-disabled existing row must keep its admin verdict")
	require.True(t, policies["lookup"].Enabled, "a NEW read-only tool lands enabled")
	require.False(t, policies["create_issue"].Enabled, "a NEW write tool lands disabled")

	// URL 切换 → 缓存客户端回收。
	require.GreaterOrEqual(t, s.closerCalls, 1, "the endpoint switch must recycle the manager's cached client")

	// 运行时守卫供给 v2 目录。
	require.ElementsMatch(t, []string{"search", "lookup", "create_issue"},
		snapshotToolNames(s.guardSnapshot(t, tenantID)))
}

// TestAcceptUpgradeIdempotentAndFingerprintGuard（计划 Step 1 用例 2）：
//   a) 同 fingerprint 二次接受 → 成功返回、零写入（安装行深比较不变、
//      UpdateInstallationAccepted 不再被调、物化服务不再 Update、策略行数不变）；
//   b) 远端已再变（清单切 v3）后持旧 v2 指纹接受 → "candidate changed since
//      preview" 拒绝且安装行/物化服务全部字段不变。
func TestAcceptUpgradeIdempotentAndFingerprintGuard(t *testing.T) {
	const tenantID = uint64(9)
	ctx := context.Background()

	t.Run("same fingerprint accept is idempotent", func(t *testing.T) {
		s := newUpgradeAcceptStack(t, tenantID)
		resp := s.previewV2(t, tenantID)

		_, err := s.svc.AcceptUpgrade(ctx, tenantID, "admin-1", s.inst.ID, resp.CandidateFingerprint)
		require.NoError(t, err)
		require.Len(t, s.pluginRepo.acceptedWrites, 1)
		afterFirst := cloneInstallation(s.innerRepo.installations[0])
		rowsAfterFirst := s.policyRows(t, tenantID)
		updatesAfterFirst := s.mcpRepo.updates

		// 二次接受同一指纹：成功且零写入、策略行数不变。
		second, err := s.svc.AcceptUpgrade(ctx, tenantID, "admin-1", s.inst.ID, resp.CandidateFingerprint)
		require.NoError(t, err)
		require.Equal(t, "2.0.0", second.Version)
		require.Len(t, s.pluginRepo.acceptedWrites, 1, "the idempotent accept must not rewrite the installation row")
		require.Equal(t, afterFirst, cloneInstallation(s.innerRepo.installations[0]))
		require.Len(t, s.policyRows(t, tenantID), len(rowsAfterFirst), "policy row count must not change")
		require.Equal(t, updatesAfterFirst, s.mcpRepo.updates, "the materialized service must not be rewritten")
	})

	t.Run("stale fingerprint is rejected with zero change", func(t *testing.T) {
		s := newUpgradeAcceptStack(t, tenantID)
		resp := s.previewV2(t, tenantID)

		// 先落定 v2 接受，再让远端前进到 v3。
		_, err := s.svc.AcceptUpgrade(ctx, tenantID, "admin-1", s.inst.ID, resp.CandidateFingerprint)
		require.NoError(t, err)
		v2State := cloneInstallation(s.innerRepo.installations[0])
		v2Svc := cloneMCPService(s.materialized(t))
		closerBefore := s.closerCalls

		s.remoteV2.Version = "3.0.0"
		v3Manifest, err := marshalManifest(s.remoteV2)
		require.NoError(t, err)
		*s.manifestNext = v3Manifest

		_, err = s.svc.AcceptUpgrade(ctx, tenantID, "admin-1", s.inst.ID, resp.CandidateFingerprint)
		require.Error(t, err)
		require.ErrorIs(t, err, service.ErrUpgradeCandidateChanged)
		require.Contains(t, err.Error(), "candidate changed since preview")

		// 拒绝路径零变化：安装行逐字段不变、物化服务不变、不再回收客户端。
		require.Equal(t, v2State, cloneInstallation(s.innerRepo.installations[0]))
		require.Equal(t, v2Svc, cloneMCPService(s.materialized(t)))
		require.Equal(t, closerBefore, s.closerCalls)
	})
}

// TestAcceptUpgradeFailureKeepsOldVersion（计划 Step 1 用例 3）：
//   a) 远端不可达（清单 host 下线）→ 抓取失败、零写入、旧目录仍由守卫供给；
//   b) 物化 URL 切换失败（第 1 次 Update 注入故障）→ 补偿回写保旧版：
//      安装行回 v1、物化 URL 回 v1、守卫目录回 v1；
//   c) 策略行写入失败（upsertErr 注入）→ 补偿回写保旧版（服务 URL 与
//      安装行双双回 v1）。
func TestAcceptUpgradeFailureKeepsOldVersion(t *testing.T) {
	const tenantID = uint64(9)
	ctx := context.Background()

	t.Run("candidate unreachable keeps v1 untouched", func(t *testing.T) {
		s := newUpgradeAcceptStack(t, tenantID)
		resp := s.previewV2(t, tenantID)
		v1State := cloneInstallation(s.innerRepo.installations[0])
		v1Svc := cloneMCPService(s.materialized(t))

		s.manifestHost.Close()

		_, err := s.svc.AcceptUpgrade(ctx, tenantID, "admin-1", s.inst.ID, resp.CandidateFingerprint)
		require.Error(t, err)
		require.Empty(t, s.pluginRepo.acceptedWrites, "a fetch failure must write nothing")
		require.Equal(t, v1State, cloneInstallation(s.innerRepo.installations[0]))
		require.Equal(t, v1Svc, cloneMCPService(s.materialized(t)))
		require.ElementsMatch(t, []string{"search"}, snapshotToolNames(s.guardSnapshot(t, tenantID)),
			"the accepted v1 directory must stay callable for member conversations")
	})

	t.Run("materialization fault compensates back to v1", func(t *testing.T) {
		s := newUpgradeAcceptStack(t, tenantID)
		resp := s.previewV2(t, tenantID)
		v1State := cloneInstallation(s.innerRepo.installations[0])
		v1Endpoint := s.materialized(t).URL

		s.mcpRepo.failUpdateAt = 1 // 第 1 次物化 Update（正向端点切换）失败
		_, err := s.svc.AcceptUpgrade(ctx, tenantID, "admin-1", s.inst.ID, resp.CandidateFingerprint)
		require.Error(t, err)

		// 补偿回写：安装行回 v1（正向 v2 写入 + 补偿 v1 回写 = 2 次调用）。
		require.Len(t, s.pluginRepo.acceptedWrites, 2,
			"the forward write must be compensated by a write-back of the memory-held old values")
		require.Equal(t, v1State, cloneInstallation(s.innerRepo.installations[0]))
		// 物化服务 URL 保持 v1。
		require.Equal(t, v1Endpoint, s.materialized(t).URL)
		// 旧版目录仍由运行时守卫供给（成员对话可继续调用旧版工具）。
		require.ElementsMatch(t, []string{"search"}, snapshotToolNames(s.guardSnapshot(t, tenantID)))
	})

	t.Run("policy write fault compensates back to v1", func(t *testing.T) {
		s := newUpgradeAcceptStack(t, tenantID)
		resp := s.previewV2(t, tenantID)
		v1State := cloneInstallation(s.innerRepo.installations[0])
		v1Endpoint := s.materialized(t).URL

		s.approvalRepo.upsertErr = errors.New("injected policy fault")
		_, err := s.svc.AcceptUpgrade(ctx, tenantID, "admin-1", s.inst.ID, resp.CandidateFingerprint)
		require.Error(t, err)

		require.Len(t, s.pluginRepo.acceptedWrites, 2, "installation row must be compensated back to v1")
		require.Equal(t, v1State, cloneInstallation(s.innerRepo.installations[0]))
		require.Equal(t, v1Endpoint, s.materialized(t).URL, "the materialized URL must be restored to v1")
		require.ElementsMatch(t, []string{"search"}, snapshotToolNames(s.guardSnapshot(t, tenantID)))
	})
}

// TestAcceptUpgradeDanglingServiceRowStillAccepts（OCR T16-OCR1-F1）：
// 悬空 service_id 锚行（卸载中断：物化服务行已删而安装行残留）下，7b 按
// 「无行可同步」继续，7c 不得因 ListByService 内部 GetByID 落空以
// "mcp service not found" 确定性阻断接受——策略行以服务为键，行已丢失则
// 无可读也无可写。接受应成功切换安装行（安装行保持权威）且零补偿。
func TestAcceptUpgradeDanglingServiceRowStillAccepts(t *testing.T) {
	const tenantID = uint64(9)
	s := newUpgradeAcceptStack(t, tenantID)
	ctx := context.Background()
	resp := s.previewV2(t, tenantID)

	// 模拟卸载中断的悬空锚：物化服务行消失，安装行 service_id 仍指向它。
	s.mcpRepo.services = nil

	accepted, err := s.svc.AcceptUpgrade(ctx, tenantID, "admin-1", s.inst.ID, resp.CandidateFingerprint)
	require.NoError(t, err, "a dangling service_id anchor must not deterministically abort every accept")
	require.Equal(t, "2.0.0", accepted.Version)
	require.Len(t, s.pluginRepo.acceptedWrites, 1, "no compensation write-back on the success path")
	require.Equal(t, "2.0.0", s.innerRepo.installations[0].AcceptedVersion)
	// 策略面保持 confirm 时的原样（search 一行），无新增也无失败残留。
	rows := s.policyRows(t, tenantID)
	require.Len(t, rows, 1)
	require.Contains(t, rows, "search")
	// 运行时守卫目录已切 v2（安装行权威）。
	require.ElementsMatch(t, []string{"search", "lookup", "create_issue"},
		snapshotToolNames(s.guardSnapshot(t, tenantID)))
}

// TestAcceptUpgradePolicyFaultCompensatesNewRows（OCR T16-OCR1-F2）：
// 7c 增量循环中途失败（第 1 个新工具已落行、第 2 个失败）时，本轮回写的
// 新策略行必须在补偿中删除——否则残留行会把同名工具的安装时规则
// （新工具 Enabled=ReadOnly）短路成治理放宽路径（后续接受跳过已有行）。
func TestAcceptUpgradePolicyFaultCompensatesNewRows(t *testing.T) {
	const tenantID = uint64(9)
	s := newUpgradeAcceptStack(t, tenantID)
	ctx := context.Background()
	resp := s.previewV2(t, tenantID)
	v1State := cloneInstallation(s.innerRepo.installations[0])
	v1Endpoint := s.materialized(t).URL

	// v2 恰有两个新工具（lookup、create_issue）：武装时清零计数（confirm
	// 已消耗 1 次调用），第 2 次 UpsertPolicy 失败 → 第 1 次必然已成功落行
	// （顺序无关的确定性中断窗口）。
	s.approvalRepo.upsertCalls = 0
	s.approvalRepo.failOnNthUpsert = 2
	_, err := s.svc.AcceptUpgrade(ctx, tenantID, "admin-1", s.inst.ID, resp.CandidateFingerprint)
	require.Error(t, err)

	// 本轮新写入的策略行必须被补偿删除；search 既有行保留。
	rows := s.policyRows(t, tenantID)
	require.Len(t, rows, 1, "only the pre-existing search row may remain — this accept's new rows must be compensated away")
	require.Contains(t, rows, "search")
	require.NotEmpty(t, s.pluginRepo.deletedPolicies, "the compensation must have deleted the rows this accept wrote")
	require.Len(t, s.pluginRepo.deletedPolicies, 1)

	// 保旧版三件套同既有补偿语义。
	require.Equal(t, v1State, cloneInstallation(s.innerRepo.installations[0]))
	require.Equal(t, v1Endpoint, s.materialized(t).URL)
	require.ElementsMatch(t, []string{"search"}, snapshotToolNames(s.guardSnapshot(t, tenantID)))
}

// TestAcceptUpgradeSyncsOAuthConfigToCandidate（OCR T16-OCR1-F3）：
// 7b 必须按候选快照重导出物化服务的 AuthConfig（镜像 ConfirmInstallation
// 的 oauthConfigFromVerifiedBaseline），否则 v1 无个人授权工具升级到 v2
// 新增个人授权工具时成员拿到永久死链（AuthorizeURL 以 IsOAuth() 拒绝）、
// scope 集变化时授权请求按旧并集发起；候选不再需要个人授权时必须显式
// 清空（共享 Update 对 nil AuthConfig 跳过写列）。
func TestAcceptUpgradeSyncsOAuthConfigToCandidate(t *testing.T) {
	const tenantID = uint64(9)
	ctx := context.Background()

	t.Run("new personal-auth tool arms OAuth on the service row", func(t *testing.T) {
		// v1 无个人授权工具（物化 AuthConfig=nil）；v2 新增个人授权只读工具。
		v2 := upgradeAcceptV2Tools()
		v2 = append(v2, plugintest.Tool{
			Name: "private_lookup", Description: "auth lookup", ReadOnly: true,
			RequiresPersonalAuth: true, Scopes: []string{"read:demo"},
			InputSchema: upgradeV1SearchSchema,
		})
		s := newUpgradeAcceptStackWithTools(t, tenantID, upgradeAcceptV1Tools(), v2)
		resp := s.previewV2(t, tenantID)

		require.Nil(t, s.materialized(t).AuthConfig, "v1 install materializes no OAuth config")
		_, err := s.svc.AcceptUpgrade(ctx, tenantID, "admin-1", s.inst.ID, resp.CandidateFingerprint)
		require.NoError(t, err)

		svcRow := s.materialized(t)
		require.NotNil(t, svcRow.AuthConfig, "the accepted personal-auth snapshot must arm OAuth on the service row")
		require.True(t, svcRow.AuthConfig.IsOAuth(), "authorize-url must not dead-end at IsOAuth()")
		require.Equal(t, []string{"read:demo"}, svcRow.AuthConfig.Scopes)
	})

	t.Run("candidate without personal auth clears OAuth explicitly", func(t *testing.T) {
		// v1 带个人授权工具（物化 OAuth）；v2 全部无个人授权 → 必须显式清空
		// （repo Update 对 nil AuthConfig 不写列，置 nil 不会生效）。
		v1 := append(upgradeAcceptV1Tools(), plugintest.Tool{
			Name: "private_lookup", Description: "auth lookup", ReadOnly: true,
			RequiresPersonalAuth: true, Scopes: []string{"read:demo"},
			InputSchema: upgradeV1SearchSchema,
		})
		v2 := []plugintest.Tool{
			{Name: "search", Description: "search v1", ReadOnly: true, InputSchema: upgradeV1SearchSchema},
			{Name: "lookup", Description: "lookup v2", ReadOnly: true, InputSchema: upgradeV1SearchSchema},
		}
		s := newUpgradeAcceptStackWithTools(t, tenantID, v1, v2)
		resp := s.previewV2(t, tenantID)
		require.True(t, s.materialized(t).AuthConfig.IsOAuth(), "v1 install materializes OAuth")

		_, err := s.svc.AcceptUpgrade(ctx, tenantID, "admin-1", s.inst.ID, resp.CandidateFingerprint)
		require.NoError(t, err)

		svcRow := s.materialized(t)
		require.False(t, svcRow.AuthConfig == nil || svcRow.AuthConfig.IsOAuth(),
			"the candidate's no-auth baseline must CLEAR OAuth on the service row")
	})

	t.Run("compensation restores the old OAuth baseline", func(t *testing.T) {
		// v1 OAuth scope 并集 [read:demo]；v2 扩到 [read:demo write:thing]，
		// 第 2 次策略写失败 → 补偿必须把 AuthConfig 连同 URL/安装行一起回旧值。
		v1 := append(upgradeAcceptV1Tools(), plugintest.Tool{
			Name: "private_lookup", Description: "auth lookup", ReadOnly: true,
			RequiresPersonalAuth: true, Scopes: []string{"read:demo"},
			InputSchema: upgradeV1SearchSchema,
		})
		v2 := []plugintest.Tool{
			{Name: "search", Description: "search v1", ReadOnly: true, InputSchema: upgradeV1SearchSchema},
			{Name: "private_lookup", Description: "auth lookup", ReadOnly: true,
				RequiresPersonalAuth: true, Scopes: []string{"read:demo", "write:thing"},
				InputSchema: upgradeV1SearchSchema},
			{Name: "lookup", Description: "lookup v2", ReadOnly: true, InputSchema: upgradeV1SearchSchema},
			{Name: "create_issue", Description: "create v2", ReadOnly: false, InputSchema: upgradeV2WriteSchema},
		}
		s := newUpgradeAcceptStackWithTools(t, tenantID, v1, v2)
		resp := s.previewV2(t, tenantID)
		oldAuth := s.materialized(t).AuthConfig

		s.approvalRepo.upsertCalls = 0
		s.approvalRepo.failOnNthUpsert = 2
		_, err := s.svc.AcceptUpgrade(ctx, tenantID, "admin-1", s.inst.ID, resp.CandidateFingerprint)
		require.Error(t, err)

		svcRow := s.materialized(t)
		require.True(t, svcRow.AuthConfig.IsOAuth(), "compensation must restore the old OAuth baseline")
		require.Equal(t, oldAuth.Scopes, svcRow.AuthConfig.Scopes, "the widened scope union must roll back")
	})
}

// TestAcceptUpgradeConcurrentSameInstallationSerializes（OCR T16-OCR2-F1）：
// 同一安装的并发接受必须串行化。无串行化时，B 的 ListByService 快照可拍摄于
// A 的 SetPolicy 落行之前 → A 已成功落库的策略行进入 B 的补偿删除集，B 失败
// 触发补偿会把 A 的行整行删除；缺行默认 enabled=true（执行时
// gate.IsEnabled 语义，mcp_tool.go:127）→ 已接受快照中的写工具以暴露状态
// 落地（fail-open）。测试在 A 的 7b 中途（onUpdate 钩子）并发发起 B 并观察：
// A 在途期间 B 必须零进展；A 返回后 B 才能完成。
func TestAcceptUpgradeConcurrentSameInstallationSerializes(t *testing.T) {
	const tenantID = uint64(9)
	s := newUpgradeAcceptStack(t, tenantID)
	resp := s.previewV2(t, tenantID)

	// A、B 均为同指纹健康接受（无故障注入——串行化断言与成败无关）：
	// 在 A 的 7b 中途并发发起 B，A 在途期间 B 必须零进展。
	bDone := make(chan error, 1)
	bProgressedDuringA := make(chan bool, 1)
	var hookArm sync.Once
	s.mcpRepo.onUpdate = func() {
		hookArm.Do(func() {
			s.mcpRepo.onUpdate = nil // 只在 A 的 7b 触发一次
			go func() {
				_, err := s.svc.AcceptUpgrade(context.Background(), tenantID, "admin-1", s.inst.ID, resp.CandidateFingerprint)
				bDone <- err
			}()
			select {
			case <-bDone:
				bProgressedDuringA <- true // B 在 A 未返回前完成 → 未串行化
			case <-time.After(1500 * time.Millisecond):
				bProgressedDuringA <- false // B 全程被阻塞 → 串行化生效
			}
		})
	}

	aErr := make(chan error, 1)
	go func() {
		_, err := s.svc.AcceptUpgrade(context.Background(), tenantID, "admin-1", s.inst.ID, resp.CandidateFingerprint)
		aErr <- err
	}()
	require.NoError(t, <-aErr, "the in-flight accept A completes normally")

	require.False(t, <-bProgressedDuringA,
		"a concurrent accept of the SAME installation must make no progress while another accept is mid-flight (T16-OCR2-F1)")
	require.NoError(t, <-bDone, "B completes once A has returned and released the serialization")
	require.Equal(t, "2.0.0", s.innerRepo.installations[0].AcceptedVersion, "B's accept lands the upgrade")
}
