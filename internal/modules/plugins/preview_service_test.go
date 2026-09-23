package plugins_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

// fakePluginPreviewRepo 记录 CreatePreview 收到的行与 DeleteExpiredPreviews
// 的调用（整分支 OCR 一轮 F1：惰性清理触发断言），方法集与
// interfaces.PluginRepository 一致。
type fakePluginPreviewRepo struct {
	interfaces.PluginRepository
	created       []*types.PluginPreview
	deleteCalls   int
	deleteCutoffs []time.Time
}

func (r *fakePluginPreviewRepo) CreatePreview(_ context.Context, p *types.PluginPreview) error {
	r.created = append(r.created, p)
	return nil
}

func (r *fakePluginPreviewRepo) DeleteExpiredPreviews(_ context.Context, before time.Time) error {
	r.deleteCalls++
	r.deleteCutoffs = append(r.deleteCutoffs, before)
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

// TestPreviewRejectsOversizedManifestURL（T02-R1-1）：manifest_url 列宽
// varchar(512)。超长 URL 必须在任何网络 I/O 之前被拒绝——否则会先完成
// SSRF 校验与整轮远端抓取核验，再在 CreatePreview 因列溢出失败被误报为
// 500 服务端故障。host 用白名单内回环地址：长度校验缺失（RED）时错误
// 会是 connection refused 而非长度拒绝，测试不依赖外网 DNS。
func TestPreviewRejectsOversizedManifestURL(t *testing.T) {
	t.Cleanup(utils.SnapshotSSRFWhitelistForTest())
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")

	repo := &fakePluginPreviewRepo{}
	svc := service.NewPluginService(repo, previewFakeLister())
	oversized := "http://127.0.0.1/m/" + strings.Repeat("a", 600)
	require.Greater(t, len(oversized), 512)

	resp, err := svc.PreviewFromManifest(context.Background(), 7, "admin-1", oversized)
	require.Nil(t, resp)
	require.ErrorContains(t, err, "exceeds")
	require.Empty(t, repo.created, "oversized manifest URL must not persist anything")
}

// TestPreviewSweepsExpiredRowsOnSuccess（整分支 OCR 一轮 F1）：预览是 TTL
// 绑定的临时审阅工件——成功写入路径必须惰性触发过期行清理（best-effort，
// 对齐 service/resource.go:244 先例）；拒绝路径（SSRF 拒绝、持久化失败）
// 不得触发：清理副作用只属于成功写入，不耦合进拒绝语义。
func TestPreviewSweepsExpiredRowsOnSuccess(t *testing.T) {
	t.Cleanup(utils.SnapshotSSRFWhitelistForTest())
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")

	// 成功路径：受控清单 + 受控端点 + 匹配 lister。
	m := previewFixtureManifest()
	manifestJSON, err := json.Marshal(m)
	require.NoError(t, err)
	base := previewControlledHost(t, &manifestJSON)
	m.Transport.Endpoint = base + "/mcp"
	manifestJSON, err = json.Marshal(m)
	require.NoError(t, err)

	repo := &fakePluginPreviewRepo{}
	svc := service.NewPluginService(repo, previewFakeLister())
	start := time.Now()
	resp, err := svc.PreviewFromManifest(context.Background(), 7, "admin-1", base+"/manifest.json")
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 1, repo.deleteCalls, "success path must trigger the lazy sweep exactly once")
	require.Len(t, repo.deleteCutoffs, 1)
	require.False(t, repo.deleteCutoffs[0].Before(start), "cutoff must be a current timestamp")
	// 新写入行 ExpiresAt = now+TTL > cutoff：惰性清理不会误删刚创建的行。
	require.Len(t, repo.created, 1)
	require.True(t, repo.created[0].ExpiresAt.After(repo.deleteCutoffs[0]),
		"the row just written must outlive the sweep cutoff")

	// 拒绝路径 1：SSRF 拒绝发生在任何持久化/清理之前。
	rejected := &fakePluginPreviewRepo{}
	svc2 := service.NewPluginService(rejected, previewFakeLister())
	_, err = svc2.PreviewFromManifest(context.Background(), 7, "admin-1", "http://10.1.2.3/manifest.json")
	require.ErrorIs(t, err, service.ErrManifestURLRejected)
	require.Equal(t, 0, rejected.deleteCalls, "an SSRF rejection must not trigger cleanup")

	// 拒绝路径 2：持久化失败不触发清理。
	m2 := previewFixtureManifest()
	manifestJSON2, err := json.Marshal(m2)
	require.NoError(t, err)
	base2 := previewControlledHost(t, &manifestJSON2)
	m2.Transport.Endpoint = base2 + "/mcp"
	manifestJSON2, err = json.Marshal(m2)
	require.NoError(t, err)
	failed := &failingPluginPreviewRepo{err: errors.New("boom")}
	svc3 := service.NewPluginService(failed, previewFakeLister())
	_, err = svc3.PreviewFromManifest(context.Background(), 7, "admin-1", base2+"/manifest.json")
	require.ErrorIs(t, err, service.ErrPreviewPersistFailed)
	require.Equal(t, 0, failed.deleteCalls, "a persist failure must not trigger cleanup")
}

// failingPluginPreviewRepo 让 CreatePreview 恒失败（err 含内部细节标记）。
type failingPluginPreviewRepo struct {
	fakePluginPreviewRepo
	err error
}

func (r *failingPluginPreviewRepo) CreatePreview(_ context.Context, _ *types.PluginPreview) error {
	return r.err
}

// TestPreviewPersistFailureDoesNotLeakRepoError（整分支终评 r3-001）：
// 持久化失败返回的错误文本只含哨兵语义——该文本经 handler
// NewInternalServerError(err.Error()) 原样进入 500 响应体，底层 DB 错误
// 细节（表名/约束名/驱动内部信息）不得拼接进去；细节只留服务端日志
// （plugin_service.go CreatePreview 失败分支的 Errorf）。
func TestPreviewPersistFailureDoesNotLeakRepoError(t *testing.T) {
	t.Cleanup(utils.SnapshotSSRFWhitelistForTest())
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")

	m := previewFixtureManifest()
	manifestJSON, err := json.Marshal(m)
	require.NoError(t, err)
	base := previewControlledHost(t, &manifestJSON)
	// 端点指向受控 host：FetchAndVerify 对端点做 SSRF 复检（外网域名会因
	// DNS 无法判定被拒），复检通过后才会走到 CreatePreview——那才是本测试
	// 要触发的分支。
	m.Transport.Endpoint = base + "/mcp"
	manifestJSON, err = json.Marshal(m)
	require.NoError(t, err)

	repo := &failingPluginPreviewRepo{err: errors.New(
		`pq: duplicate key value violates unique constraint "plugin_previews_pkey" SECRET-DB-DETAIL-123`)}
	svc := service.NewPluginService(repo, previewFakeLister())

	resp, err := svc.PreviewFromManifest(context.Background(), 7, "admin-1", base+"/manifest.json")
	require.Nil(t, resp)
	require.ErrorIs(t, err, service.ErrPreviewPersistFailed)
	require.NotContains(t, err.Error(), "SECRET-DB-DETAIL-123",
		"underlying DB error detail must not leak into the client-facing error text")
	require.NotContains(t, err.Error(), "plugin_previews_pkey",
		"schema/constraint names are internal detail, not client-facing text")
}

// TestPreviewRejectsOversizedEndpoint（T02-R1-1）：远端清单可声明任意长度
// 的 transport.endpoint，而 endpoint_url 列宽 varchar(512)。超长端点必须在
// 持久化之前被拒绝为输入问题（4xx 语义），而非落库溢出走 500 路径。
func TestPreviewRejectsOversizedEndpoint(t *testing.T) {
	t.Cleanup(utils.SnapshotSSRFWhitelistForTest())
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")

	m := previewFixtureManifest()
	manifestJSON, err := json.Marshal(m)
	require.NoError(t, err)
	base := previewControlledHost(t, &manifestJSON)
	// host 在白名单内（跳过 DNS），超长的是 path 部分。
	m.Transport.Endpoint = base + "/mcp?" + strings.Repeat("x", 600)
	manifestJSON, err = json.Marshal(m)
	require.NoError(t, err)

	repo := &fakePluginPreviewRepo{}
	svc := service.NewPluginService(repo, previewFakeLister())

	resp, err := svc.PreviewFromManifest(context.Background(), 7, "admin-1", base+"/manifest.json")
	require.Nil(t, resp)
	require.ErrorContains(t, err, "exceeds")
	require.Empty(t, repo.created, "oversized endpoint must not persist anything")
}
