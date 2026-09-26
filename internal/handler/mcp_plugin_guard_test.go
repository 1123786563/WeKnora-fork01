package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 跨任务转交 T06-OCR1-F11 / T07-OCR1-F9 回归测试。
//
// T06-OCR1-F11: ErrPluginManagedService 是服务层的确定性策略拒绝（通用
// MCP 写面对插件物化行一律拒绝），通用 handler 不得把它渲染成 500 污染
// 5xx 告警——应映射 409 Conflict（与 plugin 域 handler 把哨兵映射 4xx 的
// 约定一致）。三个触发面：UpdateMCPService / DeleteMCPService /
// UpdateMCPCredentials(Put)。
//
// T07-OCR1-F9: CreateMCPService 直接 ShouldBindJSON 到 types.MCPService，
// json tag plugin_installation_id 让客户端能伪造插件物化行（生而 Enabled、
// 进 Agent 目录、三面守卫拒绝修改/删除、卸载级联够不到）。handler 必须在
// 调用服务层前剥离该字段——物化路径只走插件安装服务（install_service）
// 的内部调用，不经通用 HTTP 入口。

type pluginGuardService struct {
	interfaces.MCPServiceService
	created   *types.MCPService
	createErr error
	updateErr error
	deleteErr error
	credErr   error
	toolsErr  error
	clearErr  error
	// B+A 裁决 #4：资源/连通测试两面同族守卫。
	resourcesErr error
	testErr      error
}

func (s *pluginGuardService) CreateMCPService(_ context.Context, svc *types.MCPService) error {
	s.created = svc
	return s.createErr
}

func (s *pluginGuardService) UpdateMCPService(
	_ context.Context, _ *types.MCPService, _ map[string]bool,
) error {
	return s.updateErr
}

func (s *pluginGuardService) DeleteMCPService(_ context.Context, _ uint64, _ string) error {
	return s.deleteErr
}

func (s *pluginGuardService) UpdateMCPCredentials(
	_ context.Context, _ uint64, _ string, _, _ *string,
) (*types.MCPService, error) {
	return nil, s.credErr
}

// OCR R1 F07：通用读取面 GET /mcp-services/{id}/tools（Viewer+）对插件物化
// 行不得返回 live 目录（未接受能力经此面泄露）。
func (s *pluginGuardService) GetMCPServiceTools(_ context.Context, _ uint64, _ string) ([]*types.MCPTool, error) {
	return nil, s.toolsErr
}

// OCR R1 F06：ClearMCPCredential 与 PUT 凭据同族，插件行必须同口径拒绝。
func (s *pluginGuardService) ClearMCPCredential(_ context.Context, _ uint64, _, _ string) error {
	return s.clearErr
}

// B+A 裁决 #4：实时资源列表（Viewer+）与连通测试（Admin+）同族守卫。
func (s *pluginGuardService) GetMCPServiceResources(_ context.Context, _ uint64, _ string) ([]*types.MCPResource, error) {
	return nil, s.resourcesErr
}

func (s *pluginGuardService) TestMCPService(_ context.Context, _ uint64, _ string) (*types.MCPTestResult, error) {
	return nil, s.testErr
}

func pluginGuardRouter(svc *pluginGuardService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(7))
		c.Next()
	})
	h := NewMCPServiceHandler(svc, nil, nil, nil)
	creds := NewMCPCredentialsHandler(svc)
	r.POST("/mcp-services", h.CreateMCPService)
	r.PUT("/mcp-services/:id", h.UpdateMCPService)
	r.DELETE("/mcp-services/:id", h.DeleteMCPService)
	r.PUT("/mcp-services/:id/credentials", creds.Put)
	r.DELETE("/mcp-services/:id/credentials/:field", creds.DeleteField)
	r.GET("/mcp-services/:id/tools", h.GetMCPServiceTools)
	r.GET("/mcp-services/:id/resources", h.GetMCPServiceResources)
	r.POST("/mcp-services/:id/test", h.TestMCPService)
	return r
}

func pluginGuardRequest(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
	}
	r.ServeHTTP(w, req)
	return w
}

func TestUpdateMCPServicePluginManagedIsConflictNot500(t *testing.T) {
	svc := &pluginGuardService{updateErr: service.ErrPluginManagedService}
	w := pluginGuardRequest(pluginGuardRouter(svc), http.MethodPut, "/mcp-services/svc-1", `{"name":"renamed"}`)
	require.Equal(t, http.StatusConflict, w.Code,
		"deterministic policy rejection must surface as 409, not a 500 that pollutes 5xx alerts: %s", w.Body.String())
}

func TestDeleteMCPServicePluginManagedIsConflictNot500(t *testing.T) {
	svc := &pluginGuardService{deleteErr: service.ErrPluginManagedService}
	w := pluginGuardRequest(pluginGuardRouter(svc), http.MethodDelete, "/mcp-services/svc-1", "")
	require.Equal(t, http.StatusConflict, w.Code,
		"deterministic policy rejection must surface as 409, not a 500 that pollutes 5xx alerts: %s", w.Body.String())
}

func TestPutMCPCredentialsPluginManagedIsConflictNot500(t *testing.T) {
	svc := &pluginGuardService{credErr: service.ErrPluginManagedService}
	w := pluginGuardRequest(pluginGuardRouter(svc), http.MethodPut, "/mcp-services/svc-1/credentials", `{"api_key":"k"}`)
	require.Equal(t, http.StatusConflict, w.Code,
		"deterministic policy rejection must surface as 409, not a 500 that pollutes 5xx alerts: %s", w.Body.String())
}

func TestCreateMCPServiceStripsClientPluginInstallationID(t *testing.T) {
	svc := &pluginGuardService{}
	body := `{"name":"forged","transport_type":"sse","plugin_installation_id":"11111111-2222-4333-8444-555555555555"}`
	w := pluginGuardRequest(pluginGuardRouter(svc), http.MethodPost, "/mcp-services", body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NotNil(t, svc.created, "service layer must be invoked")
	require.Nil(t, svc.created.PluginInstallationID,
		"a client-supplied plugin_installation_id must be stripped before the service call — a forged row would be born Enabled, enter the agent catalog, be rejected by all three write guards, and be unreachable by the uninstall cascade")
}

// OCR R1 F07：Viewer 可达的通用工具列表面对插件物化行必须确定性拒绝——
// live 目录携带未接受能力/漂移后 schema，绕过快照封堵泄露。
func TestGetMCPServiceToolsPluginManagedIsConflictNot500(t *testing.T) {
	svc := &pluginGuardService{toolsErr: service.ErrPluginManagedService}
	w := pluginGuardRequest(pluginGuardRouter(svc), http.MethodGet, "/mcp-services/svc-1/tools", "")
	require.Equal(t, http.StatusConflict, w.Code,
		"plugin-materialized rows must not serve the live directory through the generic tools endpoint: %s", w.Body.String())
}

// OCR R1 F06：DELETE /credentials/{field} 与 PUT 凭据同族写面，插件行 409。
func TestDeleteMCPCredentialFieldPluginManagedIsConflictNot500(t *testing.T) {
	svc := &pluginGuardService{clearErr: service.ErrPluginManagedService}
	w := pluginGuardRequest(pluginGuardRouter(svc), http.MethodDelete, "/mcp-services/svc-1/credentials/api_key", "")
	require.Equal(t, http.StatusConflict, w.Code,
		"credential write faces must reject plugin-managed rows uniformly (PUT already does): %s", w.Body.String())
}

// B+A 裁决 #4：GET /mcp-services/{id}/resources（Viewer+）与
// POST /mcp-services/{id}/test（Admin+）对插件物化行必须确定性 409——
// 实时远端资源/连通测试结果绕过已接受快照边界（未接受能力/漂移后目录
// 泄露），且 test 面把服务层错误包成 200 的失败结果会掩盖确定性策略拒绝。
func TestGetMCPServiceResourcesPluginManagedIsConflictNot500(t *testing.T) {
	svc := &pluginGuardService{resourcesErr: service.ErrPluginManagedService}
	w := pluginGuardRequest(pluginGuardRouter(svc), http.MethodGet, "/mcp-services/svc-1/resources", "")
	require.Equal(t, http.StatusConflict, w.Code,
		"plugin-materialized rows must not serve live remote resources through the generic resources endpoint: %s", w.Body.String())
}

func TestMCPServiceTestPluginManagedIsConflictNotTestFailure200(t *testing.T) {
	svc := &pluginGuardService{testErr: service.ErrPluginManagedService}
	w := pluginGuardRequest(pluginGuardRouter(svc), http.MethodPost, "/mcp-services/svc-1/test", "")
	require.Equal(t, http.StatusConflict, w.Code,
		"a deterministic policy rejection must surface as 409, not be wrapped into a 200 test-failure payload: %s", w.Body.String())
}

// OCR R1 F09 + F08：drift 响应四列表绝不为 null（one wire shape），且
// pluginManagedConflict 不得插在 swaggo 注释块与其函数之间（swag 按紧邻
// 配对会把注解误绑到辅助函数）。
func TestDriftReportResponseListsNeverSerializeAsNull(t *testing.T) {
	report := &types.PluginDriftReport{
		InstallationID:    "inst-1",
		DriftState:        types.PluginDriftDetected,
		SnapshotToolNames: nil,                        // 持久化层 omitempty 反序列化回 nil 的形态
		Detail:            &types.PluginDriftDetail{}, // 全空列表输入
	}
	out := driftReportResponseDTO(report)
	require.NotNil(t, out)
	require.NotNil(t, out.Detail)
	encoded, err := json.Marshal(out)
	require.NoError(t, err)
	body := string(encoded)
	for _, key := range []string{`"added":[]`, `"removed":[]`, `"schema_changed":[]`, `"description_changed":[]`} {
		require.Contains(t, body, key, "drift detail lists must serialize as [] — null breaks string[] consumers (.map/.length)")
	}
	require.Contains(t, body, `"snapshot_tool_names":[]`)
}

func TestSwagAnnotationsPrecedeCreateMCPServiceDeclaration(t *testing.T) {
	source, err := os.ReadFile("mcp_service.go")
	require.NoError(t, err)
	text := string(source)
	annotAt := strings.Index(text, "/mcp-services [post]")
	require.NotEqual(t, -1, annotAt, "the POST annotation must exist")
	createAt := strings.Index(text, "func (h *MCPServiceHandler) CreateMCPService")
	require.NotEqual(t, -1, createAt)
	// 注释块与声明之间不得再出现函数声明（swag 按「注释组紧邻其后函数」
	// 配对——中间插入 pluginManagedConflict 会让注解误绑、Create 从
	// OpenAPI 规格消失）。
	between := text[annotAt:createAt]
	require.NotContains(t, between, "func ", "no function declaration may sit between the swag block and CreateMCPService")
}
