package plugins_test

// T14（plan 07 Task 14）：候选版本五维差异计算与升级预览——纯函数
// DiffSnapshots（added/removed/schema/scope+读写+授权面/endpoint 五维，
// IsDowngrade semver 标注）+ PluginService.PreviewUpgrade（重抓
// installation.ManifestURL 长期清单来源核验、与已接受快照对比、只读不写
// 任何安装状态）。
//
// 本文件为外部测试包 plugins_test 并 import plugintest（受控远端）：
// plugintest 是 import plugins 的非测试包，内部测试包会构成导入环
// （总索引「测试包名约定」）。fake 仓储与 mutableLister 等基建复用
// install_service_test.go（同测试包）。

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/service"
	internalmcp "github.com/Tencent/WeKnora/internal/modules/airesource/mcp"
	"github.com/Tencent/WeKnora/internal/modules/plugins"
	"github.com/Tencent/WeKnora/internal/modules/plugins/plugintest"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

const (
	upgradeV1SearchSchema = `{"type":"object","properties":{},"additionalProperties":false}`
	upgradeV2WriteSchema  = `{"type":"object","properties":{"summary":{"type":"string"}},"required":["summary"],"additionalProperties":false}`
)

// TestDiffSnapshotsCoversAllDimensions（计划 Task 14 Step 1 原文）：五维差异
// 独立断言——added/removed/schema/scope/读写分类/授权面/endpoint 每一维都有
// 自己的断言；scope 与读写分类合并为一个「权限面」ChangedTools 维度但
// 标志分开。
func TestDiffSnapshotsCoversAllDimensions(t *testing.T) {
	current := []types.PluginToolSnapshot{
		{Name: "keep", InputSchemaDigest: "d1", ReadOnly: true, Scopes: []string{"read"}},
		{Name: "schema_change", InputSchemaDigest: "d2"},
		{Name: "scope_change", InputSchemaDigest: "d3", Scopes: []string{"read"}},
		{Name: "rw_change", InputSchemaDigest: "d4", ReadOnly: true},
		{Name: "auth_change", InputSchemaDigest: "d5", RequiresPersonalAuth: false},
		{Name: "removed"},
	}
	candidate := []types.PluginToolSnapshot{
		{Name: "keep", InputSchemaDigest: "d1", ReadOnly: true, Scopes: []string{"read"}},
		{Name: "schema_change", InputSchemaDigest: "dX"},
		{Name: "scope_change", InputSchemaDigest: "d3", Scopes: []string{"read", "write"}},
		{Name: "rw_change", InputSchemaDigest: "d4", ReadOnly: false},
		{Name: "auth_change", InputSchemaDigest: "d5", RequiresPersonalAuth: true},
		{Name: "added"},
	}
	d := plugins.DiffSnapshots(current, candidate, "https://old/e", "https://new/e")
	require.Len(t, d.AddedTools, 1)   // added
	require.Len(t, d.RemovedTools, 1) // removed
	require.Len(t, d.ChangedTools, 4)
	byName := map[string]types.PluginToolChange{}
	for _, c := range d.ChangedTools {
		byName[c.Name] = c
	}
	require.True(t, byName["schema_change"].SchemaChanged)
	require.True(t, byName["scope_change"].ScopeChanged)
	require.True(t, byName["rw_change"].ReadWriteClassChanged)
	require.True(t, byName["auth_change"].PersonalAuthChanged)
	require.True(t, d.EndpointChanged)
}

// TestDiffSnapshotsUnchangedDimensionsStayFalse：每一维的「不变」也独立
// 断言——目录完全一致且端点一致时，所有差异标志为假且版本字段不被纯函数
// 臆造（版本与 IsDowngrade 由 PreviewUpgrade 依据安装行与候选清单填充）。
func TestDiffSnapshotsUnchangedDimensionsStayFalse(t *testing.T) {
	snapshot := []types.PluginToolSnapshot{
		{Name: "keep", InputSchemaDigest: "d1", ReadOnly: true, Scopes: []string{"read"}},
	}
	d := plugins.DiffSnapshots(snapshot, snapshot, "https://same/e", "https://same/e")
	require.Empty(t, d.AddedTools)
	require.Empty(t, d.RemovedTools)
	require.Empty(t, d.ChangedTools)
	require.False(t, d.EndpointChanged)
	require.False(t, d.IsDowngrade)
	require.Empty(t, d.CurrentVersion)
	require.Empty(t, d.CandidateVersion)
}

// ---------------------------------------------------------------------------
// PreviewUpgrade 应用边界（plugintest 受控远端 + 真 MCPManager lister）
// ---------------------------------------------------------------------------

// upgradeStack：plugintest v1/v2 双远端（版本端点隔离——v2 清单指向 v2 端点，
// endpoint_changed=true 的前提）+ 可热换清单 host + 内存仓储 + 真实
// PluginService，落一条已确认安装。
type upgradeStack struct {
	svc          interfaces.PluginService
	pluginRepo   *installPreviewRepo
	mcpRepo      *fakeInstallMCPServiceRepo
	approvalRepo *fakeInstallApprovalRepo
	manager      *internalmcp.MCPManager
	remoteV1     *plugintest.Server
	remoteV2     *plugintest.Server
	manifestHost *httptest.Server
	manifestNext *[]byte
	inst         *types.PluginInstallation
	// baseline：confirm 落定后的全量状态快照（安装行/物化服务/预览行，
	// 深拷贝）——「预览零状态变化」断言的比较基准。
	baseline struct {
		inst     *types.PluginInstallation
		services []*types.MCPService
		previews []types.PluginPreview
	}
}

// newManagerLister 把 airesource MCPManager 粘合到 EndpointLister 契约
// （container.NewPluginMCPEndpointLister 的同款 nonce-exclusive 模式：
// internal/modules 不得互相 import 容器装配，PG 集成同款本地 seam）。
func newManagerLister(manager *internalmcp.MCPManager) plugins.EndpointLister {
	return func(ctx context.Context, transportType, endpointURL string) ([]*types.MCPTool, error) {
		var nonce [4]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return nil, err
		}
		sum := sha256.Sum256([]byte(endpointURL))
		serviceID := "plugin-verify-" + hex.EncodeToString(sum[:8]) + "-" + hex.EncodeToString(nonce[:])
		verify := &types.MCPService{
			ID: serviceID, Enabled: true,
			TransportType: types.MCPTransportType(transportType), URL: &endpointURL,
		}
		client, err := manager.GetOrCreateClient(ctx, verify)
		if err != nil {
			_ = manager.CloseClient(verify.ID)
			return nil, err
		}
		defer func() {
			_ = client.Disconnect()
			_ = manager.CloseClient(verify.ID)
		}()
		return client.ListTools(ctx)
	}
}

// marshalManifest 序列化替身当前清单（digest 与 live 目录同步计算）。
func marshalManifest(remote *plugintest.Server) ([]byte, error) {
	return json.Marshal(remote.Manifest())
}

func newUpgradeStack(t *testing.T, tenantID uint64) *upgradeStack {
	t.Helper()

	remoteV1 := plugintest.New()
	remoteV1.PluginID = "com.example.upgrade-diff"
	remoteV1.Version = "1.0.0"
	remoteV1.SetTools([]plugintest.Tool{{
		Name: "search", Description: "search v1", ReadOnly: true,
		InputSchema: upgradeV1SearchSchema,
	}})
	remoteV1.Start(t)

	remoteV2 := plugintest.New()
	remoteV2.PluginID = "com.example.upgrade-diff"
	remoteV2.Version = "2.0.0"
	remoteV2.SetTools([]plugintest.Tool{
		{Name: "search", Description: "search v1", ReadOnly: true,
			InputSchema: upgradeV1SearchSchema},
		{Name: "create_issue", Description: "create v2", ReadOnly: false,
			InputSchema: upgradeV2WriteSchema},
	})
	remoteV2.Start(t)

	// 清单 host（可热换文档）：安装期指向 v1 远端，升级预览期切到 v2
	// 版本端点——版本化清单的真实形态（同一清单地址当前声明的候选版本）。
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

	pluginRepo := &installPreviewRepo{}
	mcpRepo := &fakeInstallMCPServiceRepo{}
	approvalRepo := &fakeInstallApprovalRepo{}
	pluginRepo.mcpRepo = mcpRepo
	pluginRepo.approvalRepo = approvalRepo
	mcpSvcService := service.NewMCPServiceService(mcpRepo, nil, nil)
	approvalSvc := service.NewMCPToolApprovalService(approvalRepo, mcpRepo)
	svc := service.NewPluginService(pluginRepo, mcpSvcService, mcpRepo, approvalSvc, newManagerLister(manager), nil, nil)

	ctx := context.Background()
	preview, err := svc.PreviewFromManifest(ctx, tenantID, "admin-1", manifestHost.URL+"/manifest.json")
	require.NoError(t, err)
	confirmed, err := svc.ConfirmInstallation(ctx, tenantID, "admin-1", preview.PreviewID)
	require.NoError(t, err)
	require.Len(t, pluginRepo.installations, 1)
	inst := pluginRepo.installations[0]
	require.Equal(t, confirmed.InstallationID, inst.ID)
	require.Equal(t, manifestHost.URL+"/manifest.json", inst.ManifestURL,
		"installation row must carry the long-lived manifest source (T06 column)")

	return &upgradeStack{
		svc:          svc,
		pluginRepo:   pluginRepo,
		mcpRepo:      mcpRepo,
		approvalRepo: approvalRepo,
		manager:      manager,
		remoteV1:     remoteV1,
		remoteV2:     remoteV2,
		manifestHost: manifestHost,
		manifestNext: &manifestJSON,
		inst:         inst,
	}
}

// takeBaseline 深拷贝当前全部持久化状态作为零变化断言的比较基准
// （confirm 已把 preview 消费——ConsumedAt 非 nil 是合法终态，「不变」
// 断言比较的是预览不得再次改写任何行，而不是「未消费」）。
func (s *upgradeStack) takeBaseline() {
	s.baseline.inst = cloneInstallation(s.pluginRepo.installations[0])
	s.baseline.services = cloneServiceRows(s.mcpRepo.services)
	previews := make([]types.PluginPreview, 0, len(s.pluginRepo.previews))
	for _, p := range s.pluginRepo.previews {
		cp := *p
		cp.ToolsSnapshot = append(types.PluginPreviewTools(nil), p.ToolsSnapshot...)
		if p.ConsumedAt != nil {
			consumed := *p.ConsumedAt
			cp.ConsumedAt = &consumed
		}
		previews = append(previews, cp)
	}
	s.baseline.previews = previews
}

// switchManifestToV2 把清单 host 的文档切到 v2 清单（版本 2.0.0、端点指向
// v2 远端、目录含新增写工具）。
func (s *upgradeStack) switchManifestToV2(t *testing.T) {
	t.Helper()
	v2Manifest, err := marshalManifest(s.remoteV2)
	require.NoError(t, err)
	*s.manifestNext = v2Manifest
}

// cloneInstallation 深拷贝安装行（slice 字段不别名），供「逐字段不变」
// 断言捕获任何直接改写仓储内指针内容的路径。
func cloneInstallation(inst *types.PluginInstallation) *types.PluginInstallation {
	cp := *inst
	cp.ToolsSnapshot = append(types.PluginPreviewTools(nil), inst.ToolsSnapshot...)
	cp.DriftDetail = append(inst.DriftDetail, nil...)
	return &cp
}

// cloneServiceRows 克隆当前物化服务面快照（供不变断言比较）。
func cloneServiceRows(services []*types.MCPService) []*types.MCPService {
	out := make([]*types.MCPService, len(services))
	for i, svc := range services {
		out[i] = cloneMCPService(svc)
	}
	return out
}

func cloneMCPService(svc *types.MCPService) *types.MCPService {
	cp := *svc
	if svc.URL != nil {
		u := *svc.URL
		cp.URL = &u
	}
	if svc.PluginInstallationID != nil {
		p := *svc.PluginInstallationID
		cp.PluginInstallationID = &p
	}
	return &cp
}

// assertInstallationUntouched：安装行逐字段不变 + 物化服务行/预览行
// 零变化（对照 takeBaseline 快照）+ Agent 运行时守卫来源
// （PluginSnapshotLookup，RegisterMCPTools 快照过滤的直接数据源，T09）
// 仍返回 v1 已接受目录。
func (s *upgradeStack) assertInstallationUntouched(t *testing.T, tenantID uint64) {
	t.Helper()
	require.Equal(t, s.baseline.inst, cloneInstallation(s.pluginRepo.installations[0]),
		"preview must leave EVERY installation field untouched (accepted_version/tools_digest/state included)")
	require.Equal(t, s.baseline.services, cloneServiceRows(s.mcpRepo.services),
		"materialized service rows must stay untouched")
	nowPreviews := make([]types.PluginPreview, 0, len(s.pluginRepo.previews))
	for _, p := range s.pluginRepo.previews {
		cp := *p
		cp.ToolsSnapshot = append(types.PluginPreviewTools(nil), p.ToolsSnapshot...)
		if p.ConsumedAt != nil {
			consumed := *p.ConsumedAt
			cp.ConsumedAt = &consumed
		}
		nowPreviews = append(nowPreviews, cp)
	}
	require.Equal(t, s.baseline.previews, nowPreviews,
		"preview rows must stay untouched (the confirm-time consumption included)")

	// Agent 侧仍走旧版目录：运行时快照守卫（service.PluginSnapshotLookup
	// → RegisterMCPTools 的 PluginSnapshotProvider，T09）读取的是
	// GetByServiceID 命中的安装行 ToolsSnapshot——预览后必须仍是 v1 目录。
	lookup := service.PluginSnapshotLookup(s.pluginRepo)
	snap, err := lookup(context.Background(), tenantID, s.inst.ServiceID)
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Equal(t, []types.PluginToolSnapshot(s.baseline.inst.ToolsSnapshot), snap.Tools,
		"the agent runtime snapshot guard must still serve the accepted v1 directory")
}

// TestPreviewUpgradeDoesNotTouchInstallation：plugintest v1 已安装 → 清单
// 切 v2（目录 + 新写工具 + 新版本端点）→ PreviewUpgrade 返回差异
// （added=1、endpoint_changed=true、candidate 版本 2.0.0、非降级），安装行
// 逐字段不变、Agent 仍走 v1 目录；重复预览（幂等读）结果一致且不累积状态。
func TestPreviewUpgradeDoesNotTouchInstallation(t *testing.T) {
	const tenantID = uint64(7)
	s := newUpgradeStack(t, tenantID)
	s.takeBaseline()

	s.switchManifestToV2(t)
	resp, err := s.svc.PreviewUpgrade(context.Background(), tenantID, s.inst.ID)
	require.NoError(t, err)

	// 五维差异的预览面：added=1（create_issue，写工具）、removed=0、
	// changed=0、endpoint 变化、版本对与降级标注。
	require.Len(t, resp.Diff.AddedTools, 1)
	require.Equal(t, "create_issue", resp.Diff.AddedTools[0].Name)
	require.False(t, resp.Diff.AddedTools[0].ReadOnly, "the added tool is a write tool")
	require.Empty(t, resp.Diff.RemovedTools)
	require.Empty(t, resp.Diff.ChangedTools)
	require.True(t, resp.Diff.EndpointChanged, "v2 manifest points at the v2 version-pinned endpoint")
	require.Equal(t, "1.0.0", resp.Diff.CurrentVersion)
	require.Equal(t, "2.0.0", resp.Diff.CandidateVersion)
	require.False(t, resp.Diff.IsDowngrade)
	require.Equal(t, "com.example.upgrade-diff", resp.Diff.PluginID)
	require.NotEmpty(t, resp.CandidateFingerprint)
	require.NotEmpty(t, resp.CandidateToolsDigest)

	s.assertInstallationUntouched(t, tenantID)

	// 幂等：重复预览同结果（纯读不写、无状态累积）。
	resp2, err := s.svc.PreviewUpgrade(context.Background(), tenantID, s.inst.ID)
	require.NoError(t, err)
	require.Equal(t, resp, resp2, "repeated previews must return the identical result")
	s.assertInstallationUntouched(t, tenantID)
}

// TestPreviewUpgradeMarksDowngrade：候选版本低于已接受版本（降级也是管理员
// 决定）——IsDowngrade semver 三段数值比较标注为真，且同样零状态变化。
func TestPreviewUpgradeMarksDowngrade(t *testing.T) {
	const tenantID = uint64(7)
	s := newUpgradeStack(t, tenantID)
	s.takeBaseline()

	// v1 端点目录回退到旧版本 0.9.0（同一远端改版本号——目录不变，仅版本
	// 语义回退；plugintest.Version 可写）。
	s.remoteV1.Version = "0.9.0"
	v09Manifest, err := marshalManifest(s.remoteV1)
	require.NoError(t, err)
	*s.manifestNext = v09Manifest

	resp, err := s.svc.PreviewUpgrade(context.Background(), tenantID, s.inst.ID)
	require.NoError(t, err)
	require.True(t, resp.Diff.IsDowngrade, "0.9.0 vs accepted 1.0.0 is a downgrade")
	require.Equal(t, "0.9.0", resp.Diff.CandidateVersion)
	require.Empty(t, resp.Diff.AddedTools)
	require.Empty(t, resp.Diff.RemovedTools)
	require.Empty(t, resp.Diff.ChangedTools)

	s.assertInstallationUntouched(t, tenantID)
}

// TestPreviewUpgradeCandidateUnreachable：候选不可达（清单 host 下线）报错
// 且零状态变化；清单声明与候选端点目录不符（digest 声明漂移）复用 T01
// 核验拒绝——两路径都不得触碰安装行。
func TestPreviewUpgradeCandidateUnreachable(t *testing.T) {
	const tenantID = uint64(7)
	t.Run("manifest unreachable", func(t *testing.T) {
		s := newUpgradeStack(t, tenantID)
		s.takeBaseline()
		s.manifestHost.Close() // 候选清单来源不可达

		_, err := s.svc.PreviewUpgrade(context.Background(), tenantID, s.inst.ID)
		require.Error(t, err)
		require.ErrorIs(t, err, plugins.ErrManifestFetchFailed)
		require.Contains(t, err.Error(), "unreachable")
		s.assertInstallationUntouched(t, tenantID)
	})

	t.Run("declaration mismatch is rejected by verification", func(t *testing.T) {
		s := newUpgradeStack(t, tenantID)
		s.takeBaseline()
		// 声明 digest 漂移：plugintest 的清单 digest 与 live 永远同步
		// （Manifest() 从 live schema 现算），要制造「清单声明与候选端点
		// 目录不符」必须在序列化后的清单文档上篡改声明 digest——模拟
		// 真实世界的声明漂移，live 目录不变 → T01 核验必须拒绝。
		s.switchManifestToV2(t)
		var doc types.PluginManifest
		require.NoError(t, json.Unmarshal(*s.manifestNext, &doc))
		require.Len(t, doc.Tools, 2)
		doc.Tools[0].InputSchemaDigest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		tampered, err := json.Marshal(doc)
		require.NoError(t, err)
		*s.manifestNext = tampered

		_, err = s.svc.PreviewUpgrade(context.Background(), tenantID, s.inst.ID)
		require.Error(t, err)
		require.ErrorIs(t, err, service.ErrPluginVerifyFailed)
		require.Contains(t, err.Error(), "verification failed")
		s.assertInstallationUntouched(t, tenantID)
	})

	t.Run("foreign tenant installation is not found", func(t *testing.T) {
		s := newUpgradeStack(t, tenantID)
		_, err := s.svc.PreviewUpgrade(context.Background(), tenantID+1, s.inst.ID)
		require.ErrorIs(t, err, service.ErrInstallationNotFound)
	})
}
