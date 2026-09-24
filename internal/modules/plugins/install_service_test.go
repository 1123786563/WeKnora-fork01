package plugins_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
}

func (r *fakeInstallMCPServiceRepo) Create(_ context.Context, svc *types.MCPService) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.services = append(r.services, svc)
	return nil
}
func (r *fakeInstallMCPServiceRepo) GetByID(_ context.Context, tenantID uint64, id string) (*types.MCPService, error) {
	for _, svc := range r.services {
		if svc.TenantID == tenantID && svc.ID == id {
			return svc, nil
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
	for _, existing := range r.services {
		if existing.TenantID == svc.TenantID && existing.ID == svc.ID {
			existing.Enabled = svc.Enabled
			existing.Name = svc.Name
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
// 语义的简化版：nil patch 字段不覆盖）。
type fakeInstallApprovalRepo struct {
	rows map[string]*types.MCPToolApproval
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

func newInstallTestStack(t *testing.T, tenantID uint64) *installTestStack {
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
