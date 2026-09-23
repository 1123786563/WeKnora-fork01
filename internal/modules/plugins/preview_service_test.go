package plugins_test

import (
	"context"
	"encoding/json"
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
)

// previewDeclaredNoArgSchema 与清单声明 digest 的输入 schema：受控 lister
// 返回的 live 工具携带完全相同的 bytes，声明与实际一致（happy path 前提）。
const previewDeclaredNoArgSchema = `{"type":"object","properties":{},"additionalProperties":false}`

// fakePluginPreviewRepo 记录 CreatePreview 收到的行，方法集与
// interfaces.PluginRepository 一致（本测试只消费 CreatePreview）。
type fakePluginPreviewRepo struct {
	interfaces.PluginRepository
	created []*types.PluginPreview
}

func (r *fakePluginPreviewRepo) CreatePreview(_ context.Context, p *types.PluginPreview) error {
	r.created = append(r.created, p)
	return nil
}

func previewFixtureManifest() *types.PluginManifest {
	return &types.PluginManifest{
		Protocol:  "weknora.plugin/1",
		PluginID:  "com.example.jira-todo",
		Version:   "1.2.0",
		Name:      "Jira 本周待办",
		Transport: types.PluginTransport{Type: "http-streamable", Endpoint: "https://plugins.example.com/jira-todo/v1.2.0/mcp"},
		Auth:      &types.PluginAuth{PersonalOAuth: true},
		Tools: []types.PluginToolDecl{{
			Name: "search_my_week_issues", ReadOnly: true, RequiresPersonalAuth: true,
			Scopes:            []string{"read:jira"},
			InputSchemaDigest: plugins.ToolSchemaDigest([]byte(previewDeclaredNoArgSchema)),
		}},
	}
}

// previewControlledHost 只承载清单 JSON；远端核验由 fake lister 提供，
// 服务层测试关注 TTL/指纹持久化/SSRF 拒绝，不需要真实 MCP 握手。
// manifestJSON 按指针传入：调用方把端点改指受控主机后重新 marshal，
// host 无需重建。
func previewControlledHost(t *testing.T, manifestJSON *[]byte) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(*manifestJSON)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

func previewFakeLister() plugins.EndpointLister {
	return func(_ context.Context, _, _ string) ([]*types.MCPTool, error) {
		return []*types.MCPTool{{
			Name:        "search_my_week_issues",
			Description: "desc",
			InputSchema: []byte(previewDeclaredNoArgSchema),
		}}, nil
	}
}

func TestPreviewPersistsFingerprintAndExpiry(t *testing.T) {
	// httptest 监听 127.0.0.1，需快照恢复式放行（-count>=2 安全）。
	t.Cleanup(utils.SnapshotSSRFWhitelistForTest())
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")

	m := previewFixtureManifest()
	manifestJSON, err := json.Marshal(m)
	require.NoError(t, err)
	base := previewControlledHost(t, &manifestJSON)
	// 清单端点改指受控主机（白名单内的回环地址，否则 FetchAndVerify 的
	// 端点 SSRF 前置校验会拒绝假域名），重新 marshal 后由同一 host 提供。
	m.Transport.Endpoint = base + "/mcp"
	manifestJSON, err = json.Marshal(m)
	require.NoError(t, err)

	repo := &fakePluginPreviewRepo{}
	svc := service.NewPluginService(repo, previewFakeLister())

	start := time.Now()
	resp, err := svc.PreviewFromManifest(context.Background(), 7, "admin-1", base+"/manifest.json")
	require.NoError(t, err)

	// 断言 1：preview_id 非空；expires_at = now + TTL（默认 15m，容差 5s）。
	require.NotEmpty(t, resp.PreviewID)
	require.True(t, resp.ExpiresAt.After(start.Add(15*time.Minute-5*time.Second)),
		"expires_at must be start+TTL, got %v (start %v)", resp.ExpiresAt, start)
	require.True(t, resp.ExpiresAt.Before(start.Add(15*time.Minute+5*time.Second)),
		"expires_at must be start+TTL, got %v (start %v)", resp.ExpiresAt, start)

	// 断言 2：repo 收到指纹/摘要非空；快照可反序列化回 []PluginToolSnapshot。
	require.Len(t, repo.created, 1)
	row := repo.created[0]
	require.Equal(t, resp.PreviewID, row.ID)
	require.Equal(t, uint64(7), row.TenantID)
	require.Equal(t, "admin-1", row.CreatedBy)
	require.Equal(t, base+"/manifest.json", row.ManifestURL)
	require.Regexp(t, `^[0-9a-f]{64}$`, row.IdentityFingerprint)
	require.Regexp(t, `^[0-9a-f]{64}$`, row.ToolsDigest)
	require.Equal(t, resp.ExpiresAt, row.ExpiresAt)
	raw, err := json.Marshal(row.ToolsSnapshot)
	require.NoError(t, err)
	var back []types.PluginToolSnapshot
	require.NoError(t, json.Unmarshal(raw, &back))
	require.Len(t, back, 1)
	require.Equal(t, "search_my_week_issues", back[0].Name)
	require.True(t, back[0].ReadOnly)
	require.True(t, back[0].RequiresPersonalAuth)
	require.Equal(t, []string{"read:jira"}, back[0].Scopes)

	// 响应 DTO 的审阅字段来自核验结果。
	require.Equal(t, "com.example.jira-todo", resp.PluginID)
	require.Equal(t, "1.2.0", resp.Version)
	require.Equal(t, "http-streamable", resp.TransportType)
	require.Equal(t, row.EndpointURL, resp.EndpointURL)
	require.Equal(t, row.IdentityFingerprint, resp.IdentityFingerprint)
	require.Len(t, resp.Tools, 1)
	require.Equal(t, "search_my_week_issues", resp.Tools[0].Name)
	require.Equal(t, "desc", resp.Tools[0].Description)
	require.True(t, resp.Tools[0].ReadOnly)
	require.True(t, resp.Tools[0].RequiresPersonalAuth)
}

// TestPreviewRejectsPrivateManifestURLWithoutWrite：清单 URL 为私网地址 →
// 返回错误且 repo 未收到任何写入（SSRF 拒绝必须发生在持久化之前）。
func TestPreviewRejectsPrivateManifestURLWithoutWrite(t *testing.T) {
	t.Cleanup(utils.SnapshotSSRFWhitelistForTest())
	utils.SetSSRFWhitelistFromRaw("127.0.0.1") // 只放行回环，10.x 仍被拒绝

	repo := &fakePluginPreviewRepo{}
	svc := service.NewPluginService(repo, previewFakeLister())

	resp, err := svc.PreviewFromManifest(context.Background(), 7, "admin-1", "http://10.1.2.3/manifest.json")
	require.Nil(t, resp)
	require.ErrorIs(t, err, service.ErrManifestURLRejected)
	require.Empty(t, repo.created, "SSRF rejection must not persist anything")
}

func TestPreviewTTLBoundary(t *testing.T) {
	now := time.Now()
	expired := &types.PluginPreview{ExpiresAt: now.Add(-time.Second)}
	require.True(t, expired.Expired(now))
	live := &types.PluginPreview{ExpiresAt: now.Add(time.Minute)}
	require.False(t, live.Expired(now))
	// 已消费即过期：一次性消费语义。
	consumed := &types.PluginPreview{ExpiresAt: now.Add(time.Minute), ConsumedAt: &now}
	require.True(t, consumed.Expired(now))
}
