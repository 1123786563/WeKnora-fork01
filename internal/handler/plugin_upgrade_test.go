package handler

// T14（plan 07 Task 14）：POST /plugins/installations/:id/upgrade-preview
// handler 面——语义 stub 覆盖 200（差异五维 JSON）/404/400（核验拒绝）/
// 503（候选不可达）与 tenant/id 透传。Admin 挂载在 routes_plugins.go
// （g.Admin()，与 confirm/disable 同卫）。

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/modules/plugins"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// stubPluginUpgradeService 记录 PreviewUpgrade 调用的语义 stub。
type stubPluginUpgradeService struct {
	interfaces.PluginService
	previewResp *types.PluginUpgradePreviewResult
	previewErr  error
	gotTenant   uint64
	gotID       string
}

func (s *stubPluginUpgradeService) PreviewUpgrade(
	_ context.Context, tenantID uint64, installationID string,
) (*types.PluginUpgradePreviewResult, error) {
	s.gotTenant, s.gotID = tenantID, installationID
	return s.previewResp, s.previewErr
}

func newPluginUpgradeRouter(svc interfaces.PluginService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(7))
		c.Set(types.UserIDContextKey.String(), "admin-1")
		c.Next()
	})
	h := &PluginHandler{pluginService: svc}
	r.POST("/plugins/installations/:id/upgrade-preview", h.PreviewUpgrade)
	return r
}

func fixedUpgradePreviewResult() *types.PluginUpgradePreviewResult {
	return &types.PluginUpgradePreviewResult{
		Diff: types.PluginVersionDiff{
			PluginID:          "com.example.jira-todo",
			CurrentVersion:    "1.2.0",
			CandidateVersion:  "2.0.0",
			IsDowngrade:       false,
			EndpointChanged:   true,
			CurrentEndpoint:   "https://old.example.com/mcp",
			CandidateEndpoint: "https://new.example.com/mcp",
			AddedTools: []types.PluginToolSnapshot{
				{Name: "create_issue", Description: "write", ReadOnly: false},
			},
			RemovedTools: []types.PluginToolSnapshot{},
			ChangedTools: []types.PluginToolChange{{
				Name:                  "search",
				SchemaChanged:         false,
				ScopeChanged:          true,
				ReadWriteClassChanged: false,
				PersonalAuthChanged:   false,
				Current:               types.PluginToolSnapshot{Name: "search", Scopes: []string{"read:jira"}},
				Candidate:             types.PluginToolSnapshot{Name: "search", Scopes: []string{"read:jira", "write:jira"}},
			}},
		},
		CandidateFingerprint: "fingerprint-candidate",
		CandidateToolsDigest: "digest-candidate",
	}
}

func TestPreviewUpgradeHandler(t *testing.T) {
	t.Run("happy path maps diff onto wire shape", func(t *testing.T) {
		stub := &stubPluginUpgradeService{previewResp: fixedUpgradePreviewResult()}
		w := doPluginInstallRequest(
			newPluginUpgradeRouter(stub), http.MethodPost, "/plugins/installations/inst-1/upgrade-preview", "")
		require.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		require.Contains(t, body, `"candidate_version":"2.0.0"`)
		require.Contains(t, body, `"current_version":"1.2.0"`)
		require.Contains(t, body, `"is_downgrade":false`)
		require.Contains(t, body, `"endpoint_changed":true`)
		require.Contains(t, body, `"added_tools"`)
		require.Contains(t, body, `"create_issue"`)
		require.Contains(t, body, `"scope_changed":true`)
		require.Contains(t, body, `"candidate_fingerprint":"fingerprint-candidate"`)
		require.Contains(t, body, `"candidate_tools_digest":"digest-candidate"`)
		require.Equal(t, uint64(7), stub.gotTenant)
		require.Equal(t, "inst-1", stub.gotID)
	})

	cases := []struct {
		name string
		err  error
		code int
	}{
		{"installation not found is 404", service.ErrInstallationNotFound, http.StatusNotFound},
		{"verification failed is 400", service.ErrPluginVerifyFailed, http.StatusBadRequest},
		{"manifest fetch failed is 503", plugins.ErrManifestFetchFailed, http.StatusServiceUnavailable},
		{"wrapped fetch failed stays 503", fmt.Errorf("candidate plugin unreachable: %w", plugins.ErrManifestFetchFailed), http.StatusServiceUnavailable},
		{"persist fault is 500", service.ErrInstallationPersistFailed, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubPluginUpgradeService{previewErr: tc.err}
			w := doPluginInstallRequest(
				newPluginUpgradeRouter(stub), http.MethodPost, "/plugins/installations/inst-1/upgrade-preview", "")
			require.Equal(t, tc.code, w.Code)
		})
	}
}
