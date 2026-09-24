package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/modules/plugins"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// stubPluginInstallService 记录安装侧调用的 stub（T06/T07 handler 面）。
type stubPluginInstallService struct {
	interfaces.PluginService
	confirmResp *types.PluginInstallationResult
	confirmErr  error
	gotTenant   uint64
	gotActor    string
	gotPreview  string

	stateResp  *types.PluginInstallationResult
	stateErr   error
	gotStateID string
	gotState   string

	listResp []*types.PluginInstallationSummary
	listErr  error

	getResp  *types.PluginInstallationResult
	getErr   error
	gotGetID string
}

func (s *stubPluginInstallService) ConfirmInstallation(
	_ context.Context, tenantID uint64, actorID, previewID string,
) (*types.PluginInstallationResult, error) {
	s.gotTenant, s.gotActor, s.gotPreview = tenantID, actorID, previewID
	return s.confirmResp, s.confirmErr
}

func (s *stubPluginInstallService) SetInstallationState(
	_ context.Context, tenantID uint64, installationID, state string,
) (*types.PluginInstallationResult, error) {
	s.gotTenant, s.gotStateID, s.gotState = tenantID, installationID, state
	return s.stateResp, s.stateErr
}

func (s *stubPluginInstallService) ListInstallations(
	_ context.Context, tenantID uint64,
) ([]*types.PluginInstallationSummary, error) {
	s.gotTenant = tenantID
	return s.listResp, s.listErr
}

func (s *stubPluginInstallService) GetInstallation(
	_ context.Context, tenantID uint64, installationID string,
) (*types.PluginInstallationResult, error) {
	s.gotTenant, s.gotGetID = tenantID, installationID
	return s.getResp, s.getErr
}

func newPluginInstallRouter(svc interfaces.PluginService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(7))
		c.Set(types.UserIDContextKey.String(), "admin-1")
		c.Next()
	})
	h := &PluginHandler{pluginService: svc}
	// 静态段先注册（routes_plugins.go 同序，防 gin 通配冲突）。
	r.POST("/plugins/installations", h.ConfirmInstallation)
	r.POST("/plugins/installations/preview", h.PreviewManifest)
	r.POST("/plugins/installations/:id/disable", h.DisableInstallation)
	r.POST("/plugins/installations/:id/enable", h.EnableInstallation)
	r.GET("/plugins/installations", h.ListInstallations)
	r.GET("/plugins/installations/:id", h.GetInstallation)
	return r
}

func doPluginInstallRequest(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	r.ServeHTTP(w, req)
	return w
}

func fixedInstallationResult() *types.PluginInstallationResult {
	enabled := false
	return &types.PluginInstallationResult{
		InstallationID: "inst-1",
		PluginID:       "com.example.jira-todo",
		Name:           "Jira 本周待办",
		Description:    "desc",
		Version:        "1.2.0",
		State:          types.PluginInstallationActive,
		DriftState:     types.PluginDriftNone,
		TransportType:  "http-streamable",
		EndpointURL:    "https://plugins.example.com/mcp",
		ServiceID:      "svc-1",
		Tools: []types.PluginInstallationToolView{
			{Name: "search_my_week_issues", ReadOnly: true, Scopes: []string{"read:jira"}},
			{Name: "create_issue", ReadOnly: false, Enabled: &enabled},
		},
	}
}

func TestConfirmInstallationHandler(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		stub := &stubPluginInstallService{confirmResp: fixedInstallationResult()}
		w := doPluginInstallRequest(newPluginInstallRouter(stub), http.MethodPost, "/plugins/installations", `{"preview_id":"preview-1"}`)
		require.Equal(t, http.StatusOK, w.Code)
		require.True(t, strings.Contains(w.Body.String(), `"installation_id":"inst-1"`))
		require.Equal(t, uint64(7), stub.gotTenant)
		require.Equal(t, "admin-1", stub.gotActor)
		require.Equal(t, "preview-1", stub.gotPreview)
	})

	t.Run("missing preview_id is 400", func(t *testing.T) {
		stub := &stubPluginInstallService{confirmResp: fixedInstallationResult()}
		w := doPluginInstallRequest(newPluginInstallRouter(stub), http.MethodPost, "/plugins/installations", `{}`)
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	cases := []struct {
		name string
		err  error
		code int
	}{
		{"preview not found is 404", service.ErrPluginPreviewNotFound, http.StatusNotFound},
		{"already consumed is 400", service.ErrPreviewAlreadyConsumed, http.StatusBadRequest},
		{"expired is 400", service.ErrPreviewExpired, http.StatusBadRequest},
		{"content changed is 400", service.ErrPreviewContentChanged, http.StatusBadRequest},
		{"already installed is 409", service.ErrPluginAlreadyInstalled, http.StatusConflict},
		{"fetch failed is 503", plugins.ErrManifestFetchFailed, http.StatusServiceUnavailable},
		{"persist failed is 500", service.ErrInstallationPersistFailed, http.StatusInternalServerError},
		{"materialize failed is 500", service.ErrInstallationMaterializeFailed, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubPluginInstallService{confirmErr: tc.err}
			w := doPluginInstallRequest(newPluginInstallRouter(stub), http.MethodPost, "/plugins/installations", `{"preview_id":"preview-1"}`)
			require.Equal(t, tc.code, w.Code)
		})
	}
}

func TestSetInstallationStateHandler(t *testing.T) {
	t.Run("disable and enable are 200 and pass state", func(t *testing.T) {
		stub := &stubPluginInstallService{stateResp: fixedInstallationResult()}
		r := newPluginInstallRouter(stub)
		w := doPluginInstallRequest(r, http.MethodPost, "/plugins/installations/inst-1/disable", "")
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "inst-1", stub.gotStateID)
		require.Equal(t, types.PluginInstallationDisabled, stub.gotState)

		w = doPluginInstallRequest(r, http.MethodPost, "/plugins/installations/inst-1/enable", "")
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, types.PluginInstallationActive, stub.gotState)
	})

	t.Run("not found is 404 and invalid state is 400", func(t *testing.T) {
		stub := &stubPluginInstallService{stateErr: service.ErrInstallationNotFound}
		w := doPluginInstallRequest(newPluginInstallRouter(stub), http.MethodPost, "/plugins/installations/inst-1/disable", "")
		require.Equal(t, http.StatusNotFound, w.Code)

		stub2 := &stubPluginInstallService{stateErr: service.ErrInstallationStateInvalid}
		w = doPluginInstallRequest(newPluginInstallRouter(stub2), http.MethodPost, "/plugins/installations/inst-1/disable", "")
		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestPluginDiscoveryHandlers(t *testing.T) {
	t.Run("list is 200 with summaries", func(t *testing.T) {
		stub := &stubPluginInstallService{listResp: []*types.PluginInstallationSummary{{
			InstallationID: "inst-1", PluginID: "com.example.jira-todo", Name: "Jira 本周待办",
			Version: "1.2.0", State: types.PluginInstallationActive, DriftState: types.PluginDriftNone,
			RequiresPersonalAuth: true, ToolCount: 2,
		}}}
		w := doPluginInstallRequest(newPluginInstallRouter(stub), http.MethodGet, "/plugins/installations", "")
		require.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		require.Contains(t, body, `"plugin_id":"com.example.jira-todo"`)
		require.Contains(t, body, `"tool_count":2`)
		require.Equal(t, uint64(7), stub.gotTenant)
	})

	t.Run("get is 200 and foreign id is 404", func(t *testing.T) {
		stub := &stubPluginInstallService{getResp: fixedInstallationResult()}
		r := newPluginInstallRouter(stub)
		w := doPluginInstallRequest(r, http.MethodGet, "/plugins/installations/inst-1", "")
		require.Equal(t, http.StatusOK, w.Code)
		require.Contains(t, w.Body.String(), `"installation_id":"inst-1"`)
		require.Equal(t, "inst-1", stub.gotGetID)

		stub2 := &stubPluginInstallService{getErr: service.ErrInstallationNotFound}
		w = doPluginInstallRequest(newPluginInstallRouter(stub2), http.MethodGet, "/plugins/installations/inst-1", "")
		require.Equal(t, http.StatusNotFound, w.Code)
	})
}
