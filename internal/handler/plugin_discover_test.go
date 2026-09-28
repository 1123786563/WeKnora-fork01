package handler

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
)

// TestListInstallationsHandler（T07）：成员发现列表 handler 契约——摘要
// 字段经 envelope 透出、租户 ID 从上下文透传、空列表 wire 形态为 []（非
// null）、仓储故障 500；响应不含快照 schema 原文（DTO 无该字段，钉住
// wire 契约防回归）。
func TestListInstallationsHandler(t *testing.T) {
	t.Run("list returns summaries for the context tenant", func(t *testing.T) {
		stub := &stubPluginInstallService{listResp: []*types.PluginInstallationSummary{{
			InstallationID: "inst-1", PluginID: "com.example.jira-todo", Name: "Jira 本周待办",
			Version: "1.2.0", State: types.PluginInstallationActive, DriftState: types.PluginDriftNone,
			RequiresPersonalAuth: true, ToolCount: 2,
		}}}
		w := doPluginInstallRequest(newPluginInstallRouter(stub), http.MethodGet, "/plugins/installations", "")
		require.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		require.Contains(t, body, `"success":true`)
		require.Contains(t, body, `"installation_id":"inst-1"`)
		require.Contains(t, body, `"state":"active"`)
		require.Contains(t, body, `"drift_state":"none"`)
		require.Contains(t, body, `"tool_count":2`)
		require.Contains(t, body, `"requires_personal_auth":true`)
		require.Equal(t, uint64(7), stub.gotTenant)
	})

	t.Run("empty list is an empty array not null", func(t *testing.T) {
		stub := &stubPluginInstallService{}
		w := doPluginInstallRequest(newPluginInstallRouter(stub), http.MethodGet, "/plugins/installations", "")
		require.Equal(t, http.StatusOK, w.Code)
		require.Contains(t, w.Body.String(), `"data":[]`)
	})

	t.Run("repo fault on list is 500", func(t *testing.T) {
		stub := &stubPluginInstallService{listErr: service.ErrInstallationPersistFailed}
		w := doPluginInstallRequest(newPluginInstallRouter(stub), http.MethodGet, "/plugins/installations", "")
		require.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("list wire carries no snapshot schema", func(t *testing.T) {
		stub := &stubPluginInstallService{listResp: []*types.PluginInstallationSummary{{
			InstallationID: "inst-1", PluginID: "com.example.jira-todo",
			State: types.PluginInstallationActive, DriftState: types.PluginDriftNone,
		}}}
		w := doPluginInstallRequest(newPluginInstallRouter(stub), http.MethodGet, "/plugins/installations", "")
		require.Equal(t, http.StatusOK, w.Code)
		require.NotContains(t, w.Body.String(), "schema", "summary wire must stay schema-free")
	})
}

// TestGetInstallationHandler（T07）：详情 handler 契约——200 详情含工具
// 目录元数据且不含快照 schema 原文；跨空间/未知 ID 统一 404 固定文案，
// 响应体不区分二者（不泄露存在性）。
func TestGetInstallationHandler(t *testing.T) {
	t.Run("detail is 200 with tool metadata and no schema echo", func(t *testing.T) {
		stub := &stubPluginInstallService{getResp: fixedInstallationResult()}
		w := doPluginInstallRequest(newPluginInstallRouter(stub), http.MethodGet, "/plugins/installations/inst-1", "")
		require.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		require.Contains(t, body, `"installation_id":"inst-1"`)
		require.Contains(t, body, `"endpoint_url":"https://plugins.example.com/mcp"`)
		require.Contains(t, body, `"name":"search_my_week_issues"`)
		require.Contains(t, body, `"read_only":true`)
		require.Equal(t, "inst-1", stub.gotGetID)
		require.NotContains(t, body, "schema", "detail wire must stay schema-free (untrusted remote data)")
	})

	t.Run("foreign or unknown id is the same plain 404", func(t *testing.T) {
		foreign := &stubPluginInstallService{getErr: service.ErrInstallationNotFound}
		wForeign := doPluginInstallRequest(newPluginInstallRouter(foreign), http.MethodGet, "/plugins/installations/inst-of-other-tenant", "")
		require.Equal(t, http.StatusNotFound, wForeign.Code)

		unknown := &stubPluginInstallService{getErr: service.ErrInstallationNotFound}
		wUnknown := doPluginInstallRequest(newPluginInstallRouter(unknown), http.MethodGet, "/plugins/installations/no-such-inst", "")
		require.Equal(t, http.StatusNotFound, wUnknown.Code)

		// 两类 404 响应体完全一致——存在性不泄露是服务层语义，handler 层
		// 钉住「同哨兵同响应」的 wire 表现。
		require.Equal(t, wForeign.Body.String(), wUnknown.Body.String())
	})
}
