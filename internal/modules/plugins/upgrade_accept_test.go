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
	"testing"

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
// pluginRepository.UpdateInstallationAccepted 的同构内存实现：版本/端点/
// 快照/digest 落行 + 漂移重置），并记录全部调用供补偿序断言。
type upgradeAcceptRepo struct {
	*installPreviewRepo
	acceptedWrites []acceptedUpgradeWrite
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

// upgradeAcceptMCPRepo 包装 fakeInstallMCPServiceRepo：Update 全字段持久化
// （含 URL——T06 fake 只拷 enabled/name/description，升级切片的端点切换
// 断言需要 URL 落行），failUpdateAt 注入第 N 次 Update 失败（0=不注入）。
type upgradeAcceptMCPRepo struct {
	*fakeInstallMCPServiceRepo
	updates      int
	failUpdateAt int
}

func (r *upgradeAcceptMCPRepo) Update(_ context.Context, svc *types.MCPService) error {
	r.updates++
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
// 安装（v1 目录 1 只读工具 search）。
type upgradeAcceptStack struct {
	svc          interfaces.PluginService
	pluginRepo   *upgradeAcceptRepo
	innerRepo    *installPreviewRepo
	mcpRepo      *upgradeAcceptMCPRepo
	approvalRepo *fakeInstallApprovalRepo
	approvalSvc  interfaces.MCPToolApprovalService
	manager      *internalmcp.MCPManager
	remoteV1     *plugintest.Server
	remoteV2     *plugintest.Server
	manifestHost *httptest.Server
	manifestNext *[]byte
	inst         *types.PluginInstallation
	closerCalls  int
}

func newUpgradeAcceptStack(t *testing.T, tenantID uint64) *upgradeAcceptStack {
	t.Helper()

	remoteV1 := plugintest.New()
	remoteV1.PluginID = upgradeAcceptPluginID
	remoteV1.Version = "1.0.0"
	remoteV1.SetTools([]plugintest.Tool{{
		Name: "search", Description: "search v1", ReadOnly: true,
		InputSchema: upgradeV1SearchSchema,
	}})
	remoteV1.Start(t)

	// v2：保留 search（同 schema）+ 新增 1 只读 lookup + 1 写 create_issue；
	// 独立远端 → 端点路径切换（endpoint_changed 的前提）。
	remoteV2 := plugintest.New()
	remoteV2.PluginID = upgradeAcceptPluginID
	remoteV2.Version = "2.0.0"
	remoteV2.SetTools([]plugintest.Tool{
		{Name: "search", Description: "search v1", ReadOnly: true,
			InputSchema: upgradeV1SearchSchema},
		{Name: "lookup", Description: "lookup v2", ReadOnly: true,
			InputSchema: upgradeV1SearchSchema},
		{Name: "create_issue", Description: "create v2", ReadOnly: false,
			InputSchema: upgradeV2WriteSchema},
	})
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
	approvalRepo := &fakeInstallApprovalRepo{}
	innerRepo.mcpRepo = mcpRepo.fakeInstallMCPServiceRepo
	innerRepo.approvalRepo = approvalRepo
	pluginRepo := &upgradeAcceptRepo{installPreviewRepo: innerRepo}

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
