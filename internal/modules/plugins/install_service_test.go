package plugins_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/modules/plugins"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/Tencent/WeKnora/internal/utils"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// installReadSchema / installWriteSchema 是清单声明 digest 的输入 schema：
// 受控 lister 返回的 live 工具携带完全相同的 bytes（happy path 前提）。
const installReadSchema = `{"type":"object","properties":{},"additionalProperties":false}`
const installWriteSchema = `{"type":"object","properties":{"summary":{"type":"string"}},"required":["summary"],"additionalProperties":false}`

// mutableLister 允许测试中途切换 live 工具 schema，模拟「预览后远端漂移」。
type mutableLister struct {
	readSchema  string
	writeSchema string
}

func (l *mutableLister) asEndpointLister() plugins.EndpointLister {
	return func(_ context.Context, _, _ string) ([]*types.MCPTool, error) {
		return []*types.MCPTool{
			{Name: "search_my_week_issues", Description: "desc", InputSchema: []byte(l.readSchema)},
			{Name: "create_issue", Description: "write", InputSchema: []byte(l.writeSchema)},
		}, nil
	}
}

// fakeInstallMCPServiceRepo 内存实现 MCPServiceRepository（物化服务面）。
type fakeInstallMCPServiceRepo struct {
	services  []*types.MCPService
	createErr error
	// onCreate 在 Create 失败前触发（R12 F14：取消调用方 ctx 后返回错误，
	// 检验补偿是否仍能在已取消 ctx 下完成）。
	onCreate  func()
	updateErr error
}

func (r *fakeInstallMCPServiceRepo) Create(_ context.Context, svc *types.MCPService) error {
	if r.onCreate != nil {
		r.onCreate()
	}
	if r.createErr != nil {
		return r.createErr
	}
	r.services = append(r.services, svc)
	return nil
}
func (r *fakeInstallMCPServiceRepo) GetByID(_ context.Context, tenantID uint64, id string) (*types.MCPService, error) {
	for _, svc := range r.services {
		if svc.TenantID == tenantID && svc.ID == id {
			// 拷贝语义（与真实 gorm 扫描一致）：调用方对返回值的修改不落库，
			// 除非 Update 成功——否则「Update 失败但 Enabled 已翻」的假象会
			// 掩盖方向序写入的真实行为（R12 F16 测试）。
			cp := *svc
			return &cp, nil
		}
	}
	return nil, nil
}
func (r *fakeInstallMCPServiceRepo) List(_ context.Context, tenantID uint64) ([]*types.MCPService, error) {
	var out []*types.MCPService
	for _, svc := range r.services {
		if svc.TenantID == tenantID {
			out = append(out, svc)
		}
	}
	return out, nil
}
func (r *fakeInstallMCPServiceRepo) ListEnabled(_ context.Context, tenantID uint64) ([]*types.MCPService, error) {
	var out []*types.MCPService
	for _, svc := range r.services {
		if svc.TenantID == tenantID && svc.Enabled {
			out = append(out, svc)
		}
	}
	return out, nil
}
func (r *fakeInstallMCPServiceRepo) ListByIDs(_ context.Context, tenantID uint64, ids []string) ([]*types.MCPService, error) {
	idset := map[string]bool{}
	for _, id := range ids {
		idset[id] = true
	}
	var out []*types.MCPService
	for _, svc := range r.services {
		if svc.TenantID == tenantID && idset[svc.ID] {
			out = append(out, svc)
		}
	}
	return out, nil
}
func (r *fakeInstallMCPServiceRepo) Update(_ context.Context, svc *types.MCPService) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	for _, existing := range r.services {
		if existing.TenantID == svc.TenantID && existing.ID == svc.ID {
			existing.Enabled = svc.Enabled
			existing.Name = svc.Name
			existing.Description = svc.Description
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}
func (r *fakeInstallMCPServiceRepo) Delete(_ context.Context, tenantID uint64, id string) error {
	kept := r.services[:0]
	for _, svc := range r.services {
		if svc.TenantID != tenantID || svc.ID != id {
			kept = append(kept, svc)
		}
	}
	r.services = kept
	return nil
}

// fakeInstallApprovalRepo 内存实现 MCPToolApprovalRepository（000042 upsert
// 语义的简化版：nil patch 字段不覆盖）。upsertErr 注入策略写失败
// （R12 F21：检验补偿的硬级联清理）。
type fakeInstallApprovalRepo struct {
	rows      map[string]*types.MCPToolApproval
	upsertErr error
}

func approvalKey(serviceID, toolName string) string { return serviceID + "|" + toolName }

func (r *fakeInstallApprovalRepo) ListByService(_ context.Context, tenantID uint64, serviceID string) ([]*types.MCPToolApproval, error) {
	var out []*types.MCPToolApproval
	for _, row := range r.rows {
		if row.TenantID == tenantID && row.ServiceID == serviceID {
			out = append(out, row)
		}
	}
	return out, nil
}
func (r *fakeInstallApprovalRepo) IsRequired(_ context.Context, tenantID uint64, serviceID, toolName string) (bool, error) {
	if row, ok := r.rows[approvalKey(serviceID, toolName)]; ok {
		return row.RequireApproval, nil
	}
	return false, nil
}
func (r *fakeInstallApprovalRepo) IsEnabled(_ context.Context, tenantID uint64, serviceID, toolName string) (bool, error) {
	if row, ok := r.rows[approvalKey(serviceID, toolName)]; ok {
		return row.Enabled, nil
	}
	// 缺省语义：无行视为启用（types/mcp.go MCPToolApproval.Enabled 注释）。
	return true, nil
}
func (r *fakeInstallApprovalRepo) UpsertPolicy(_ context.Context, tenantID uint64, serviceID, toolName string, patch types.MCPToolPolicyPatch) error {
	if r.upsertErr != nil {
		return r.upsertErr
	}
	if r.rows == nil {
		r.rows = map[string]*types.MCPToolApproval{}
	}
	key := approvalKey(serviceID, toolName)
	row, ok := r.rows[key]
	if !ok {
		row = &types.MCPToolApproval{
			ID:              "approval-" + serviceID + "-" + toolName,
			TenantID:        tenantID,
			ServiceID:       serviceID,
			ToolName:        toolName,
			RequireApproval: false,
			Enabled:         true,
		}
		r.rows[key] = row
	}
	if patch.RequireApproval != nil {
		row.RequireApproval = *patch.RequireApproval
	}
	if patch.Enabled != nil {
		row.Enabled = *patch.Enabled
	}
	return nil
}

// fakePluginInstallRepo 是覆盖预览+安装两半方法集的内存仓储。
type fakePluginInstallRepo struct {
	previewErr     error
	installations  []*types.PluginInstallation
	createInstErr  error
	deleteInstCall int
}

func (r *fakePluginInstallRepo) CreatePreview(_ context.Context, p *types.PluginPreview) error {
	return r.previewErr
}
func (r *fakePluginInstallRepo) GetPreview(_ context.Context, tenantID uint64, id string) (*types.PluginPreview, error) {
	return nil, nil
}
func (r *fakePluginInstallRepo) MarkPreviewConsumed(_ context.Context, tenantID uint64, id string) error {
	return gorm.ErrRecordNotFound
}
func (r *fakePluginInstallRepo) DeleteExpiredPreviews(_ context.Context, before time.Time) error {
	return nil
}

func (r *fakePluginInstallRepo) CreateInstallation(_ context.Context, inst *types.PluginInstallation) error {
	if r.createInstErr != nil {
		return r.createInstErr
	}
	r.installations = append(r.installations, inst)
	return nil
}
func (r *fakePluginInstallRepo) GetInstallation(_ context.Context, tenantID uint64, id string) (*types.PluginInstallation, error) {
	for _, inst := range r.installations {
		if inst.TenantID == tenantID && inst.ID == id {
			return inst, nil
		}
	}
	return nil, nil
}
func (r *fakePluginInstallRepo) GetInstallationByTenantPlugin(_ context.Context, tenantID uint64, pluginID string) (*types.PluginInstallation, error) {
	for _, inst := range r.installations {
		if inst.TenantID == tenantID && inst.PluginID == pluginID {
			return inst, nil
		}
	}
	return nil, nil
}
func (r *fakePluginInstallRepo) GetByServiceID(_ context.Context, tenantID uint64, serviceID string) (*types.PluginInstallation, error) {
	for _, inst := range r.installations {
		if inst.TenantID == tenantID && inst.ServiceID == serviceID {
			return inst, nil
		}
	}
	return nil, nil
}
func (r *fakePluginInstallRepo) ListInstallationsByTenant(_ context.Context, tenantID uint64) ([]*types.PluginInstallation, error) {
	var out []*types.PluginInstallation
	for _, inst := range r.installations {
		if inst.TenantID == tenantID {
			out = append(out, inst)
		}
	}
	return out, nil
}
func (r *fakePluginInstallRepo) UpdateInstallationState(_ context.Context, tenantID uint64, id, state string) error {
	for _, inst := range r.installations {
		if inst.TenantID == tenantID && inst.ID == id {
			inst.State = state
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}
func (r *fakePluginInstallRepo) UpdateInstallationServiceID(_ context.Context, tenantID uint64, id, serviceID string) error {
	for _, inst := range r.installations {
		if inst.TenantID == tenantID && inst.ID == id {
			inst.ServiceID = serviceID
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}
func (r *fakePluginInstallRepo) DeleteInstallation(_ context.Context, tenantID uint64, id string) error {
	r.deleteInstCall++
	kept := r.installations[:0]
	for _, inst := range r.installations {
		if inst.TenantID != tenantID || inst.ID != id {
			kept = append(kept, inst)
		}
	}
	r.installations = kept
	return nil
}

// installTestStack 组装确认安装链路：受控清单 host + 可变 lister + 真实
// MCPServiceService/MCPToolApprovalService（内存仓储）+ 预置一个已核验预览。
type installTestStack struct {
	svc           interfaces.PluginService
	mcpSvcService interfaces.MCPServiceService
	approvalSvc   interfaces.MCPToolApprovalService
	pluginRepo    *installPreviewRepo
	mcpRepo       *fakeInstallMCPServiceRepo
	approvalRepo  *fakeInstallApprovalRepo
	lister        *mutableLister
	manifest      *types.PluginManifest
	manifestBytes *[]byte
	previewID     string
}

// replaceManifest 热替换受控 host 上的清单文档（自洽漂移场景）。
func (s *installTestStack) replaceManifest(doc []byte) {
	*s.manifestBytes = doc
}

// installPreviewRepo 在 fakePluginInstallRepo 之上补齐可检索的预览存储。
type installPreviewRepo struct {
	fakePluginInstallRepo
	previews []*types.PluginPreview
	// 与同栈其余 fake 的交叉引用：HardDeleteServiceCascade 需要级联清理
	// mcp_services 与 mcp_tool_approvals（R12 F21/F06）。
	mcpRepo      *fakeInstallMCPServiceRepo
	approvalRepo *fakeInstallApprovalRepo
	// ctxAware 模拟真实 DB 驱动对已取消 ctx 的语义（R12 F14）：补偿若
	// 复用已取消的请求 ctx，删除会以 ctx 错误失败、安装行残留。
	ctxAware       bool
	updateStateErr error
	onTenantPlugin func()
	hardDeletedSvc []string
}

func (r *installPreviewRepo) CreatePreview(_ context.Context, p *types.PluginPreview) error {
	if r.previewErr != nil {
		return r.previewErr
	}
	r.previews = append(r.previews, p)
	return nil
}
func (r *installPreviewRepo) GetPreview(_ context.Context, tenantID uint64, id string) (*types.PluginPreview, error) {
	for _, p := range r.previews {
		if p.TenantID == tenantID && p.ID == id {
			return p, nil
		}
	}
	return nil, nil
}
func (r *installPreviewRepo) MarkPreviewConsumed(_ context.Context, tenantID uint64, id string) error {
	for _, p := range r.previews {
		if p.TenantID == tenantID && p.ID == id {
			if p.ConsumedAt != nil || !time.Now().Before(p.ExpiresAt) {
				return gorm.ErrRecordNotFound
			}
			now := time.Now()
			p.ConsumedAt = &now
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}
func (r *installPreviewRepo) DeleteExpiredPreviews(_ context.Context, before time.Time) error {
	return nil
}

func (r *installPreviewRepo) GetInstallationByTenantPlugin(_ context.Context, tenantID uint64, pluginID string) (*types.PluginInstallation, error) {
	if r.onTenantPlugin != nil {
		r.onTenantPlugin()
	}
	for _, inst := range r.installations {
		if inst.TenantID == tenantID && inst.PluginID == pluginID {
			return inst, nil
		}
	}
	return nil, nil
}

func (r *installPreviewRepo) UpdateInstallationState(ctx context.Context, tenantID uint64, id, state string) error {
	if r.ctxAware {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if r.updateStateErr != nil {
		return r.updateStateErr
	}
	for _, inst := range r.installations {
		if inst.TenantID == tenantID && inst.ID == id {
			inst.State = state
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (r *installPreviewRepo) DeleteInstallation(ctx context.Context, tenantID uint64, id string) error {
	if r.ctxAware {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	r.deleteInstCall++
	kept := r.installations[:0]
	for _, inst := range r.installations {
		if inst.TenantID != tenantID || inst.ID != id {
			kept = append(kept, inst)
		}
	}
	r.installations = kept
	return nil
}

// HardDeleteServiceCascade 模拟仓储的插件派生行硬级联删除：物化服务 +
// 逐工具策略行一并清除（R12 F21：软删服务不触发 FK 级联，策略行会成孤儿）。
func (r *installPreviewRepo) HardDeleteServiceCascade(ctx context.Context, tenantID uint64, serviceID string) error {
	if r.ctxAware {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	r.hardDeletedSvc = append(r.hardDeletedSvc, serviceID)
	if r.mcpRepo != nil {
		kept := r.mcpRepo.services[:0]
		for _, svc := range r.mcpRepo.services {
			if svc.TenantID != tenantID || svc.ID != serviceID {
				kept = append(kept, svc)
			}
		}
		r.mcpRepo.services = kept
	}
	if r.approvalRepo != nil && r.approvalRepo.rows != nil {
		for key, row := range r.approvalRepo.rows {
			if row.TenantID == tenantID && row.ServiceID == serviceID {
				delete(r.approvalRepo.rows, key)
			}
		}
	}
	return nil
}

func newInstallTestStack(t *testing.T, tenantID uint64) *installTestStack {
	return newInstallTestStackCustom(t, tenantID, nil)
}

// newInstallTestStackCustom 在 marshal 前允许测试定制基线清单（auth 基线
// 变体场景）；customize 为 nil 时与 newInstallTestStack 完全一致。
func newInstallTestStackCustom(
	t *testing.T, tenantID uint64,
	customize func(*types.PluginManifest),
) *installTestStack {
	t.Helper()
	t.Cleanup(utils.SnapshotSSRFWhitelistForTest())
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")

	lister := &mutableLister{readSchema: installReadSchema, writeSchema: installWriteSchema}
	m := &types.PluginManifest{
		Protocol:  "weknora.plugin/1",
		PluginID:  "com.example.jira-todo",
		Version:   "1.2.0",
		Name:      "Jira 本周待办",
		Transport: types.PluginTransport{Type: "http-streamable"},
		Auth:      &types.PluginAuth{PersonalOAuth: true, Scopes: []string{"read:jira"}},
		Tools: []types.PluginToolDecl{
			{
				Name: "search_my_week_issues", ReadOnly: true, RequiresPersonalAuth: true,
				Scopes:            []string{"read:jira"},
				InputSchemaDigest: plugins.ToolSchemaDigest([]byte(installReadSchema)),
			},
			{
				Name: "create_issue", ReadOnly: false,
				InputSchemaDigest: plugins.ToolSchemaDigest([]byte(installWriteSchema)),
			},
		},
	}
	if customize != nil {
		customize(m)
	}
	manifestJSON, err := json.Marshal(m)
	require.NoError(t, err)

	// 按指针捕获：场景 e 需要在预览之后整体替换清单（自洽漂移——
	// 声明 digest 与 live schema 同步变化，FetchAndVerify 自身不拒，
	// 由确认侧的 tools_digest 复核拦截）。
	manifestJSONPtr := &manifestJSON
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(*manifestJSONPtr)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// 清单端点改指受控主机（白名单内的回环地址）后重新 marshal。
	m.Transport.Endpoint = srv.URL + "/mcp"
	manifestJSON, err = json.Marshal(m)
	require.NoError(t, err)

	pluginRepo := &installPreviewRepo{}
	mcpRepo := &fakeInstallMCPServiceRepo{}
	approvalRepo := &fakeInstallApprovalRepo{}
	pluginRepo.mcpRepo = mcpRepo
	pluginRepo.approvalRepo = approvalRepo
	mcpSvcService := service.NewMCPServiceService(mcpRepo, nil, nil)
	approvalSvc := service.NewMCPToolApprovalService(approvalRepo, mcpRepo)
	svc := service.NewPluginService(pluginRepo, mcpSvcService, mcpRepo, approvalSvc, lister.asEndpointLister())

	resp, err := svc.PreviewFromManifest(context.Background(), tenantID, "admin-1", srv.URL+"/manifest.json")
	require.NoError(t, err)

	return &installTestStack{
		svc:           svc,
		mcpSvcService: mcpSvcService,
		approvalSvc:   approvalSvc,
		pluginRepo:    pluginRepo,
		mcpRepo:       mcpRepo,
		approvalRepo:  approvalRepo,
		lister:        lister,
		manifest:      m,
		manifestBytes: manifestJSONPtr,
		previewID:     resp.PreviewID,
	}
}

func (s *installTestStack) confirm(t *testing.T, tenantID uint64, previewID string) (*types.PluginInstallationResult, error) {
	t.Helper()
	return s.svc.ConfirmInstallation(context.Background(), tenantID, "admin-1", previewID)
}

func TestConfirmInstallationRejectsStalePreview(t *testing.T) {
	// 场景 a/b：preview 不存在 / 属于其他租户 → "preview not found"。
	s := newInstallTestStack(t, 7)
	_, err := s.confirm(t, 7, "no-such-preview")
	require.ErrorIs(t, err, service.ErrPluginPreviewNotFound)
	_, err = s.confirm(t, 8, s.previewID) // 跨租户读不到 → 同一 not found 语义
	require.ErrorIs(t, err, service.ErrPluginPreviewNotFound)

	// 场景 c：已消费 → "preview already consumed"。
	s2 := newInstallTestStack(t, 7)
	for _, p := range s2.pluginRepo.previews {
		now := time.Now()
		p.ConsumedAt = &now
	}
	_, err = s2.confirm(t, 7, s2.previewID)
	require.ErrorIs(t, err, service.ErrPreviewAlreadyConsumed)

	// 场景 d：已过期 → "preview expired"。
	s3 := newInstallTestStack(t, 7)
	for _, p := range s3.pluginRepo.previews {
		p.ExpiresAt = time.Now().Add(-time.Minute)
	}
	_, err = s3.confirm(t, 7, s3.previewID)
	require.ErrorIs(t, err, service.ErrPreviewExpired)

	// 场景 e：重抓远端自洽漂移（清单声明 digest 与 live schema 同步
	// 变化）→ tools_digest != preview.ToolsDigest → "preview content changed"。
	s4 := newInstallTestStack(t, 7)
	driftedSchema := `{"type":"object","properties":{"q":{"type":"string"}},"additionalProperties":false}`
	s4.lister.readSchema = driftedSchema
	drifted := *s4.manifest // 浅拷贝足够：仅改 Tools 与 Endpoint
	drifted.Tools = []types.PluginToolDecl{
		{
			Name: "search_my_week_issues", ReadOnly: true, RequiresPersonalAuth: true,
			Scopes:            []string{"read:jira"},
			InputSchemaDigest: plugins.ToolSchemaDigest([]byte(driftedSchema)),
		},
		{
			Name: "create_issue", ReadOnly: false,
			InputSchemaDigest: plugins.ToolSchemaDigest([]byte(installWriteSchema)),
		},
	}
	driftedJSON, err := json.Marshal(drifted)
	require.NoError(t, err)
	s4.replaceManifest(driftedJSON)
	_, err = s4.confirm(t, 7, s4.previewID)
	require.ErrorIs(t, err, service.ErrPreviewContentChanged)

	// 五场景零写入：无安装行、无物化服务、无审批行；除场景 c 自身预置
	// 的已消费状态外，预览未被消费。
	for _, st := range []*installTestStack{s, s3, s4} {
		require.Empty(t, st.pluginRepo.installations)
		require.Empty(t, st.mcpRepo.services)
		require.Empty(t, st.approvalRepo.rows)
		for _, p := range st.pluginRepo.previews {
			require.Nil(t, p.ConsumedAt, "rejected confirm must not consume the preview")
		}
	}
	require.Empty(t, s2.pluginRepo.installations)
	require.Empty(t, s2.mcpRepo.services)
	require.Empty(t, s2.approvalRepo.rows)
}

func TestConfirmInstallationCreatesAndMaterializes(t *testing.T) {
	s := newInstallTestStack(t, 7)
	resp, err := s.confirm(t, 7, s.previewID)
	require.NoError(t, err)

	// 断言 1：安装行 state=active、版本/端点与预览一致、manifest_url 复制、
	// service_id 回填、漂移态 none。
	require.Len(t, s.pluginRepo.installations, 1)
	inst := s.pluginRepo.installations[0]
	require.Equal(t, types.PluginInstallationActive, inst.State)
	require.Equal(t, types.PluginDriftNone, inst.DriftState)
	require.Equal(t, "1.2.0", inst.AcceptedVersion)
	require.Equal(t, s.manifest.Transport.Endpoint, inst.EndpointURL)
	require.Equal(t, "http-streamable", inst.TransportType)
	require.NotEmpty(t, inst.ManifestURL)
	require.NotEmpty(t, inst.ServiceID)
	require.Equal(t, uint64(7), inst.TenantID)

	// 断言 2：mcp_services 物化行 Name="plugin:"+plugin_id、
	// PluginInstallationID 指向安装行、OAuth 声明落入 AuthConfig、Enabled。
	require.Len(t, s.mcpRepo.services, 1)
	mat := s.mcpRepo.services[0]
	require.Equal(t, "plugin:com.example.jira-todo", mat.Name)
	require.NotNil(t, mat.PluginInstallationID)
	require.Equal(t, inst.ID, *mat.PluginInstallationID)
	require.True(t, mat.Enabled)
	require.NotNil(t, mat.AuthConfig)
	require.Equal(t, types.MCPAuthOAuth, mat.AuthConfig.AuthType)
	require.Equal(t, []string{"read:jira"}, mat.AuthConfig.Scopes)
	require.Equal(t, uint64(7), mat.TenantID)

	// 断言 3：MCPToolApproval 显式行 = 快照工具数；只读 Enabled=true、
	// 写 Enabled=false（新写工具默认关闭，B5 的安装时落地）。
	require.Len(t, s.approvalRepo.rows, 2)
	byTool := map[string]*types.MCPToolApproval{}
	for _, row := range s.approvalRepo.rows {
		byTool[row.ToolName] = row
	}
	require.True(t, byTool["search_my_week_issues"].Enabled)
	require.False(t, byTool["create_issue"].Enabled)

	// 断言 4：preview.ConsumedAt 已置位。
	for _, p := range s.pluginRepo.previews {
		if p.ID == s.previewID {
			require.NotNil(t, p.ConsumedAt)
		}
	}

	// 服务层结果：安装 ID/状态/工具视图与落库一致。
	require.Equal(t, inst.ID, resp.InstallationID)
	require.Equal(t, types.PluginInstallationActive, resp.State)
	require.Len(t, resp.Tools, 2)
	for _, tool := range resp.Tools {
		if tool.Name == "create_issue" {
			require.NotNil(t, tool.Enabled)
			require.False(t, *tool.Enabled)
		}
	}
}

func TestConfirmInstallationIdempotent(t *testing.T) {
	s := newInstallTestStack(t, 7)
	_, err := s.confirm(t, 7, s.previewID)
	require.NoError(t, err)

	// 同一 preview 二次确认 → already consumed，不产生第二行。
	_, err = s.confirm(t, 7, s.previewID)
	require.ErrorIs(t, err, service.ErrPreviewAlreadyConsumed)
	require.Len(t, s.pluginRepo.installations, 1)

	// 同 (tenant, plugin) 新 preview 再确认 → already installed。
	resp, err := s.svc.PreviewFromManifest(context.Background(), 7, "admin-1", s.pluginRepo.previews[0].ManifestURL)
	require.NoError(t, err)
	_, err = s.confirm(t, 7, resp.PreviewID)
	require.ErrorIs(t, err, service.ErrPluginAlreadyInstalled)
	require.Len(t, s.pluginRepo.installations, 1)
	require.Len(t, s.mcpRepo.services, 1)
}

func TestConfirmInstallationCompensatesOnMaterializeFailure(t *testing.T) {
	s := newInstallTestStack(t, 7)
	// 物化失败：mcp_services Create 报错。
	s.mcpRepo.createErr = errors.New("boom: mcp_services down")
	_, err := s.confirm(t, 7, s.previewID)
	require.Error(t, err)

	// 补偿：安装行已删（(tenant, plugin) 唯一位未被占用）。
	require.Empty(t, s.pluginRepo.installations)

	// 修复物化故障后可重新确认成功。
	s.mcpRepo.createErr = nil
	resp, err := s.confirm(t, 7, s.previewID)
	require.NoError(t, err)
	require.NotEmpty(t, resp.InstallationID)
	require.Len(t, s.pluginRepo.installations, 1)
}

func TestSetInstallationStateSyncsService(t *testing.T) {
	s := newInstallTestStack(t, 7)
	resp, err := s.confirm(t, 7, s.previewID)
	require.NoError(t, err)

	// disable → 安装行 disabled 且物化服务 Enabled=false。
	_, err = s.svc.SetInstallationState(context.Background(), 7, resp.InstallationID, types.PluginInstallationDisabled)
	require.NoError(t, err)
	require.Equal(t, types.PluginInstallationDisabled, s.pluginRepo.installations[0].State)
	require.False(t, s.mcpRepo.services[0].Enabled)

	// enable → 反向。
	_, err = s.svc.SetInstallationState(context.Background(), 7, resp.InstallationID, types.PluginInstallationActive)
	require.NoError(t, err)
	require.Equal(t, types.PluginInstallationActive, s.pluginRepo.installations[0].State)
	require.True(t, s.mcpRepo.services[0].Enabled)

	// 未知 state → 参数错误。
	_, err = s.svc.SetInstallationState(context.Background(), 7, resp.InstallationID, "bogus")
	require.ErrorIs(t, err, service.ErrInstallationStateInvalid)

	// 不存在/跨租户 → not found。
	_, err = s.svc.SetInstallationState(context.Background(), 8, resp.InstallationID, types.PluginInstallationDisabled)
	require.ErrorIs(t, err, service.ErrInstallationNotFound)
}

// TestConfirmInstallationCompensatesWithCancelledContext（OCR 一轮 R12-B
// F14）：确认请求的 ctx 在物化失败时已取消——补偿必须用独立于请求的
// ctx 完成（WithoutCancel），否则补偿删除被同一取消打回，安装行残留并
// 永久占用 (tenant, plugin) 唯一槽。fake 的 ctxAware 模拟 DB 驱动对已
// 取消 ctx 的拒绝语义。
func TestConfirmInstallationCompensatesWithCancelledContext(t *testing.T) {
	s := newInstallTestStack(t, 7)
	s.pluginRepo.ctxAware = true

	ctx, cancel := context.WithCancel(context.Background())
	s.mcpRepo.createErr = errors.New("boom: mcp_services down")
	s.mcpRepo.onCreate = cancel // 物化失败同时取消调用方 ctx

	_, err := s.svc.ConfirmInstallation(ctx, 7, "admin-1", s.previewID)
	require.Error(t, err)
	// 补偿在已取消的请求 ctx 下仍完成：安装行被删（唯一槽释放）。
	require.Empty(t, s.pluginRepo.installations,
		"compensation must run on a cancellation-independent context")

	// 修复物化故障后可重新确认成功。
	s.mcpRepo.createErr = nil
	s.mcpRepo.onCreate = nil
	resp, err := s.confirm(t, 7, s.previewID)
	require.NoError(t, err)
	require.NotEmpty(t, resp.InstallationID)
}

// TestConfirmInstallationDuplicateKeyMapsToConflict（OCR 一轮 R12-B F15）：
// 并发确认同一 plugin 的输家在唯一索引处收到约束冲突——必须改写为
// ErrPluginAlreadyInstalled（handler 409 语义），而非笼统 500。
func TestConfirmInstallationDuplicateKeyMapsToConflict(t *testing.T) {
	s := newInstallTestStack(t, 7)
	s.pluginRepo.createInstErr = repository.ErrInstallationDuplicateKey
	_, err := s.confirm(t, 7, s.previewID)
	require.ErrorIs(t, err, service.ErrPluginAlreadyInstalled)
	require.Empty(t, s.mcpRepo.services, "losing racer must not materialize a service")
}

// TestConfirmInstallationPolicyWriteFailureHardCascades（OCR 一轮 R12-B
// F21）：SetPolicy 循环失败触发补偿——物化服务与已写策略行必须硬级联
// 删除（软删服务不触发 FK 级联，策略行会按死 serviceID 键控成孤儿）。
func TestConfirmInstallationPolicyWriteFailureHardCascades(t *testing.T) {
	s := newInstallTestStack(t, 7)
	s.approvalRepo.upsertErr = errors.New("approval store down")

	_, err := s.confirm(t, 7, s.previewID)
	require.Error(t, err)
	require.Empty(t, s.pluginRepo.installations)
	require.Empty(t, s.mcpRepo.services, "materialized service must be hard-deleted, not soft-deleted")
	require.Empty(t, s.approvalRepo.rows, "derived policy rows must be cascade-cleaned")

	// 修复后可重装。
	s.approvalRepo.upsertErr = nil
	resp, err := s.confirm(t, 7, s.previewID)
	require.NoError(t, err)
	require.NotEmpty(t, resp.InstallationID)
}

// TestConfirmInstallationExpiredAcrossTTLDistinguished（OCR 一轮 R12-B
// F17）：确认跨越 TTL 边界（Step 2 判定未过期、Step 7 执行时已过期）
// ——MarkPreviewConsumed 的 ErrRecordNotFound 必须重读 preview 区分
// 「已消费」与「已过期」，不得一律误报 ErrPreviewAlreadyConsumed。
func TestConfirmInstallationExpiredAcrossTTLDistinguished(t *testing.T) {
	s := newInstallTestStack(t, 7)
	// Step 3（重复安装判定）时把预览改为过期：Step 2 已通过、Step 7 时
	// MarkPreviewConsumed 命中过期分支。
	s.pluginRepo.onTenantPlugin = func() {
		for _, p := range s.pluginRepo.previews {
			p.ExpiresAt = time.Now().Add(-time.Minute)
		}
	}
	_, err := s.confirm(t, 7, s.previewID)
	require.ErrorIs(t, err, service.ErrPreviewExpired)
}

// TestSetInstallationStateFailureStaysFailClosed（OCR 一轮 R12-B F16）：
// 两段写任一失败不得产生「installation=disabled 而 mcp_services.Enabled=
// true」的 fail-open 不一致——方向序写入使失败面收敛 fail-closed。
func TestSetInstallationStateFailureStaysFailClosed(t *testing.T) {
	s := newInstallTestStack(t, 7)
	resp, err := s.confirm(t, 7, s.previewID)
	require.NoError(t, err)

	// 场景 1：服务同步失败（disable：服务先行，失败则安装行不翻转——
	// 两者保持一致的 active/enabled 态，可重试）。
	s.mcpRepo.updateErr = errors.New("svc store down")
	_, err = s.svc.SetInstallationState(context.Background(), 7, resp.InstallationID, types.PluginInstallationDisabled)
	require.Error(t, err)
	require.Equal(t, types.PluginInstallationActive, s.pluginRepo.installations[0].State,
		"disable must not flip the installation when the service sync failed")
	require.True(t, s.mcpRepo.services[0].Enabled)

	// 场景 2：安装行更新失败（disable：服务已禁用——即使安装行仍 active，
	// 运行时只认 service.Enabled，失败面是 fail-closed 的）。
	s.mcpRepo.updateErr = nil
	s.pluginRepo.updateStateErr = errors.New("inst store down")
	_, err = s.svc.SetInstallationState(context.Background(), 7, resp.InstallationID, types.PluginInstallationDisabled)
	require.Error(t, err)
	require.False(t, s.mcpRepo.services[0].Enabled,
		"disable must take effect on the runtime-facing service even if the installation row update failed")
}

// TestUninstallInstallationReleasesUniqueSlot（OCR 一轮 R12-B F06b）：
// 卸载入口硬级联删除物化服务与策略行并释放 (tenant, plugin) 唯一槽——
// 补偿残留自此可自愈，无需人工修库。跨租户/不存在 → not found。
func TestUninstallInstallationReleasesUniqueSlot(t *testing.T) {
	s := newInstallTestStack(t, 7)
	resp, err := s.confirm(t, 7, s.previewID)
	require.NoError(t, err)
	require.Len(t, s.mcpRepo.services, 1)
	require.Len(t, s.approvalRepo.rows, 2)
	serviceID := s.mcpRepo.services[0].ID

	// 跨租户 → not found（不泄露存在性）。
	require.ErrorIs(t, s.svc.UninstallInstallation(context.Background(), 8, resp.InstallationID), service.ErrInstallationNotFound)

	require.NoError(t, s.svc.UninstallInstallation(context.Background(), 7, resp.InstallationID))
	require.Empty(t, s.pluginRepo.installations)
	require.Empty(t, s.mcpRepo.services)
	require.Empty(t, s.approvalRepo.rows)
	require.Equal(t, []string{serviceID}, s.pluginRepo.hardDeletedSvc)

	// 唯一槽已释放：新预览可重新安装。
	newPreview, err := s.svc.PreviewFromManifest(context.Background(), 7, "admin-1", s.pluginRepo.previews[0].ManifestURL)
	require.NoError(t, err)
	reinstalled, err := s.confirm(t, 7, newPreview.PreviewID)
	require.NoError(t, err)
	require.NotEqual(t, resp.InstallationID, reinstalled.InstallationID)
}

// TestConfirmInstallationAuthConfigFromVerifiedBaseline（跨任务转交
// T01-OCR1-F3）：manifest 级 auth（personal_oauth / auth.scopes）不进
// ToolsDigest 也不进 IdentityFingerprint——远端可在预览 TTL 窗口内翻转它
// 而工具目录不变、确认守卫照常通过。物化 AuthConfig 因此绝不能采信 fresh
// 重抓值，只能从 preview（管理员审阅过的基线）的工具级声明推导：
// scopes 扩大攻击在确认面零生效。
func TestConfirmInstallationAuthConfigFromVerifiedBaseline(t *testing.T) {
	// 场景 1：远端在预览后把 auth.scopes 从 [read:jira] 扩为
	// [read:jira, write:jira, admin:jira]（工具声明与目录完全不变 →
	// digest/fingerprint 复核通过）——物化 scopes 仍必须是基线工具级
	// 声明的并集 [read:jira]。
	s := newInstallTestStack(t, 7)
	escalated := *s.manifest // 浅拷贝：Tools 不动
	escalated.Auth = &types.PluginAuth{
		PersonalOAuth: true,
		Scopes:        []string{"read:jira", "write:jira", "admin:jira"},
	}
	escalatedJSON, err := json.Marshal(escalated)
	require.NoError(t, err)
	s.replaceManifest(escalatedJSON)

	resp, err := s.confirm(t, 7, s.previewID)
	require.NoError(t, err, "guard passes: the tool directory is unchanged")
	require.NotEmpty(t, resp.InstallationID)
	require.Len(t, s.mcpRepo.services, 1)
	mat := s.mcpRepo.services[0]
	require.NotNil(t, mat.AuthConfig, "tool-level baseline declares personal OAuth")
	require.Equal(t, types.MCPAuthOAuth, mat.AuthConfig.AuthType)
	require.Equal(t, []string{"read:jira"}, mat.AuthConfig.Scopes,
		"materialized scopes must come from the verified baseline, never the fresh manifest auth block")

	// 场景 2：基线无任何工具级个人授权需求（requires_personal_auth 全
	// false、auth.personal_oauth=false）——远端在预览后翻转
	// personal_oauth=true 并夹带 admin scopes。物化 AuthConfig 必须为
	// nil：fresh 的 manifest 级翻转不构成已审阅的授权需求（spec 49 行：
	// 清单中的权限声明不能单独构成执行授权）。
	s2 := newInstallTestStackCustom(t, 7, func(m *types.PluginManifest) {
		m.Auth = &types.PluginAuth{PersonalOAuth: false}
		for i := range m.Tools {
			m.Tools[i].RequiresPersonalAuth = false
			m.Tools[i].Scopes = nil
		}
	})
	flipped := *s2.manifest
	flipped.Auth = &types.PluginAuth{PersonalOAuth: true, Scopes: []string{"admin:everything"}}
	flippedJSON, err := json.Marshal(flipped)
	require.NoError(t, err)
	s2.replaceManifest(flippedJSON)

	resp2, err := s2.confirm(t, 7, s2.previewID)
	require.NoError(t, err, "guard passes: the tool directory is unchanged")
	require.NotEmpty(t, resp2.InstallationID)
	require.Len(t, s2.mcpRepo.services, 1)
	require.Nil(t, s2.mcpRepo.services[0].AuthConfig,
		"no verified tool-level auth requirement → no materialized OAuth config, fresh flip ignored")
}

// TestGenericMCPAPIRejectsPluginManagedServices（跨任务转交 T04-OCR1-F6）：
// 插件物化的 mcp_services 行必须由插件安装 API 独占治理——通用 MCP 管理
// 面（PUT /mcp-services/:id、DELETE、PUT /:id/credentials）对其改写可绕过
// 已核验端点基线（改 URL/auth/enabled）、DELETE 会留下孤儿安装行叠加软删
// 重名使重装卡死。三处写面一律哨兵拒绝；手工服务（plugin_installation_id
// NULL）行为不变。
func TestGenericMCPAPIRejectsPluginManagedServices(t *testing.T) {
	s := newInstallTestStack(t, 7)
	_, err := s.confirm(t, 7, s.previewID)
	require.NoError(t, err)
	require.Len(t, s.mcpRepo.services, 1)
	managed := s.mcpRepo.services[0]
	require.NotNil(t, managed.PluginInstallationID)

	// PUT /mcp-services/:id → 拒绝，行不变。
	edited := *managed
	edited.Description = "tampered via generic API"
	err = s.mcpSvcService.UpdateMCPService(context.Background(), &edited, map[string]bool{"description": true})
	require.ErrorIs(t, err, service.ErrPluginManagedService)
	require.Equal(t, "Jira 本周待办", s.mcpRepo.services[0].Description,
		"plugin-managed service row must stay untouched")

	// DELETE /mcp-services/:id → 拒绝，行仍在。
	err = s.mcpSvcService.DeleteMCPService(context.Background(), 7, managed.ID)
	require.ErrorIs(t, err, service.ErrPluginManagedService)
	require.Len(t, s.mcpRepo.services, 1, "plugin-managed service must not be deletable via the generic API")

	// PUT /mcp-services/:id/credentials → 拒绝，AuthConfig 未被注入凭证。
	apiKey := "injected-via-generic-api"
	_, err = s.mcpSvcService.UpdateMCPCredentials(context.Background(), 7, managed.ID, &apiKey, nil)
	require.ErrorIs(t, err, service.ErrPluginManagedService)
	require.Empty(t, s.mcpRepo.services[0].AuthConfig.APIKey)

	// 手工服务（plugin_installation_id NULL）同 API 行为不变：description
	// 更新照常成功（挑无 config 变化的字段，避免触碰 nil manager 的
	// CloseClient 路径）。
	manual := &types.MCPService{
		ID: "svc-manual", TenantID: 7, Name: "manual-svc", Enabled: true,
		TransportType: types.MCPTransportSSE, Description: "before",
	}
	require.NoError(t, s.mcpRepo.Create(context.Background(), manual))
	editedManual := *manual
	editedManual.Description = "after"
	require.NoError(t, s.mcpSvcService.UpdateMCPService(
		context.Background(), &editedManual, map[string]bool{"description": true}))
	stored, err := s.mcpRepo.GetByID(context.Background(), 7, "svc-manual")
	require.NoError(t, err)
	require.Equal(t, "after", stored.Description)
}
