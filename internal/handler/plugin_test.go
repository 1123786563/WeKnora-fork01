package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/modules/plugins"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// stubPluginService 替身：classifier 模拟服务层按输入 URL 做出的分类
// （SSRF 拒绝 / OAuth 保护端点），其余路径返回固定 types 层结果（整分支
// OCR 一轮 F2：服务契约已回归 types 层，DTO 组装归 handler）。
type stubPluginService struct {
	interfaces.PluginService
	resp       *types.PluginPreviewResult
	classifier func(manifestURL string) error
	gotTenant  uint64
	gotActor   string
	gotURL     string
}

func (s *stubPluginService) PreviewFromManifest(
	_ context.Context, tenantID uint64, actorID, manifestURL string,
) (*types.PluginPreviewResult, error) {
	s.gotTenant, s.gotActor, s.gotURL = tenantID, actorID, manifestURL
	if s.classifier != nil {
		if err := s.classifier(manifestURL); err != nil {
			return nil, err
		}
	}
	return s.resp, nil
}

func newPluginPreviewRouter(svc interfaces.PluginService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(7))
		c.Set(types.UserIDContextKey.String(), "admin-1")
		c.Next()
	})
	h := &PluginHandler{pluginService: svc}
	r.POST("/plugins/installations/preview", h.PreviewManifest)
	return r
}

func fixedPreviewResponse() *types.PluginPreviewResult {
	return &types.PluginPreviewResult{
		PreviewID:           "preview-1",
		PluginID:            "com.example.jira-todo",
		Version:             "1.2.0",
		Name:                "Jira 本周待办",
		Description:         "desc",
		TransportType:       "http-streamable",
		EndpointURL:         "https://plugins.example.com/jira-todo/v1.2.0/mcp",
		Tools:               []types.PluginPreviewToolReview{{Name: "search_my_week_issues", ReadOnly: true, Scopes: []string{"read:jira"}}},
		IdentityFingerprint: "f" + strings.Repeat("0", 63),
		ExpiresAt:           time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC),
	}
}

// postPluginPreview 发起一次预览请求（handler 包已有其他 postPreview 助手，
// 此处用插件专属名避免重声明）。
func postPluginPreview(t *testing.T, svc interfaces.PluginService, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/plugins/installations/preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	newPluginPreviewRouter(svc).ServeHTTP(w, req)
	return w
}

func TestPreviewManifestHandlerReturnsPreview(t *testing.T) {
	stub := &stubPluginService{resp: fixedPreviewResponse()}
	w := postPluginPreview(t, stub, `{"manifest_url":"https://plugins.example.com/manifest.json"}`)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"preview_id":"preview-1"`)
	require.Contains(t, w.Body.String(), `"success":true`)
	// 主体/操作者/URL 从认证上下文与请求体正确传递。
	require.Equal(t, uint64(7), stub.gotTenant)
	require.Equal(t, "admin-1", stub.gotActor)
	require.Equal(t, "https://plugins.example.com/manifest.json", stub.gotURL)
}

func TestPreviewManifestHandlerRejectsMissingManifestURL(t *testing.T) {
	for _, body := range []string{`{}`, `{"manifest_url":""}`, ``} {
		w := postPluginPreview(t, &stubPluginService{resp: fixedPreviewResponse()}, body)
		require.Equalf(t, http.StatusBadRequest, w.Code, "body %q must be rejected with 400", body)
	}
}

// TestPreviewManifestHandlerRejectsOversizedManifestURL（T02-R1-1）：
// plugin_previews.manifest_url 列宽 varchar(512)，超长清单 URL 必须在
// 绑定层被 400 拒绝，而不是走完抓取核验后以 500 落库溢出。
func TestPreviewManifestHandlerRejectsOversizedManifestURL(t *testing.T) {
	stub := &stubPluginService{resp: fixedPreviewResponse()}
	oversized := `{"manifest_url":"https://example.com/` + strings.Repeat("a", 600) + `"}`
	w := postPluginPreview(t, stub, oversized)
	require.Equal(t, http.StatusBadRequest, w.Code)
	// 绑定层拒绝：service 根本未被调用（零抓取、零写入）。
	require.Empty(t, stub.gotURL)
}

func TestPreviewManifestHandlerMapsOAuthProtectedEndpoint(t *testing.T) {
	stub := &stubPluginService{classifier: func(string) error {
		return fmt.Errorf("%w: endpoint demands OAuth", plugins.ErrOAuthProtectedEndpoint)
	}}
	w := postPluginPreview(t, stub, `{"manifest_url":"https://plugins.example.com/manifest.json"}`)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "插件服务要求授权才能核验工具目录")
}

func TestPreviewManifestHandlerPassesThroughSSRFRejection(t *testing.T) {
	stub := &stubPluginService{classifier: func(manifestURL string) error {
		if manifestURL == "file:///x" {
			return fmt.Errorf("%w: invalid scheme: file (only http/https allowed)", service.ErrManifestURLRejected)
		}
		return nil
	}}
	w := postPluginPreview(t, stub, `{"manifest_url":"file:///x"}`)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "manifest URL rejected")

	// 对照：合法 URL 不被该分类器拒绝。
	w = postPluginPreview(t, stub, `{"manifest_url":"https://plugins.example.com/manifest.json"}`)
	require.Equal(t, http.StatusOK, w.Code)
}
