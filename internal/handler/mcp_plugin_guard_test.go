package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
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
