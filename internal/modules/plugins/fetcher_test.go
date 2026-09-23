package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/airesource/mcp"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/utils"
	"github.com/stretchr/testify/require"
)

func newControlledPluginHost(t *testing.T, manifestHandler http.HandlerFunc, mcpHandler http.HandlerFunc) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", manifestHandler)
	mux.HandleFunc("/mcp", mcpHandler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

// 注：基于真实 MCPManager 的 EndpointLister 适配器测试（happy path / 并发隔离 /
// 错误路径连接回收 / OAuth 哨兵映射）位于 internal/container/plugin_lister_test.go：
// 适配器自本模块迁往组合根（internal/modules 之间禁止互相 import），测试随迁。

func TestFetchRejectsNonHTTPAndPrivateManifestURLs(t *testing.T) {
	lister := func(context.Context, string, string) ([]*types.MCPTool, error) {
		t.Fatal("lister must not be called for rejected URLs")
		return nil, nil
	}
	for _, rawURL := range []string{
		"file:///etc/passwd", "gopher://x", "http://127.0.0.1:8080/manifest.json",
		"http://localhost/manifest.json", "http://169.254.169.254/latest/meta-data",
		"http://10.1.2.3/manifest.json", "http://[::1]/manifest.json",
	} {
		_, err := FetchAndVerify(context.Background(), rawURL, lister)
		require.Errorf(t, err, "url %s must be rejected", rawURL)
	}
}

// TestBuildVerifiedSnapshotDuplicateNameEchoIsBounded（整分支 OCR 一轮 F3）：
// 本函数是导出的且契约注释明文「不得依赖调用方先跑 ValidateManifest」——
// 重复声明名的拒绝错误必须走 echoQuoted 截断（≤maxEchoRunes），不得用 %q
// 原样回显任意长度的未审核名字（本测试直接以未校验清单调用，绕过
// ValidateManifest 的 ≤128 runes 前置，正是该不变量要防的调用形态）。
func TestBuildVerifiedSnapshotDuplicateNameEchoIsBounded(t *testing.T) {
	longName := strings.Repeat("x", 3000)
	m := validManifest()
	m.Tools = []types.PluginToolDecl{
		{Name: longName, InputSchemaDigest: "d"},
		{Name: longName, InputSchemaDigest: "d"},
	}
	_, _, err := BuildVerifiedSnapshot(m, nil)
	require.ErrorContains(t, err, "duplicate tool name")
	require.LessOrEqual(t, len(err.Error()), 160,
		"the echoed unvetted name must be truncated (echoQuoted), not quoted in full")
	require.NotContains(t, err.Error(), longName)
}

// TestBuildVerifiedSnapshotProblemEchoIsBounded（整分支 OCR 二轮 F1）：
// 本函数导出且契约明文「不得依赖调用方先跑 ValidateManifest」——三处
// manifest 侧字段回显（missing 的 decl.Name、digest mismatch 的
// decl.Name/decl.InputSchemaDigest、description 错误前缀的 decl.Name）必须
// 全部走 echoQuoted 截断，单条问题消息不得因 %q 转义膨胀到字段原长。
func TestBuildVerifiedSnapshotProblemEchoIsBounded(t *testing.T) {
	long := strings.Repeat("x", 3000)
	schema := []byte(`{"type":"object","properties":{}}`)

	// 场景 1：missing——未匹配 live 的 decl.Name 原样回显（无任何长度前置）。
	m := validManifest()
	m.Tools = []types.PluginToolDecl{{Name: long, InputSchemaDigest: "d"}}
	_, _, err := BuildVerifiedSnapshot(m, nil)
	require.ErrorContains(t, err, "missing from live endpoint")
	require.LessOrEqual(t, len(err.Error()), 400, "missing-tool echo must be truncated")
	require.NotContains(t, err.Error(), long)

	// 场景 2：digest mismatch——decl.InputSchemaDigest 是未审核字段，
	// 恶意清单可声明任意长度假 digest。
	m2 := validManifest()
	m2.Tools = []types.PluginToolDecl{{Name: "search_my_week_issues", InputSchemaDigest: long}}
	live := []*types.MCPTool{{Name: "search_my_week_issues", InputSchema: schema}}
	_, _, err = BuildVerifiedSnapshot(m2, live)
	require.ErrorContains(t, err, "schema digest mismatch")
	require.LessOrEqual(t, len(err.Error()), 400, "declared-digest echo must be truncated")
	require.NotContains(t, err.Error(), long)

	// 场景 3：live description 校验失败的 where 前缀携带 decl.Name——
	// 名称须匹配 live（受 128 runes 前置约束），此处锚定路径行为与总体有界。
	m3 := validManifest()
	m3.Tools = []types.PluginToolDecl{{Name: "search_my_week_issues", InputSchemaDigest: ToolSchemaDigest(schema)}}
	live3 := []*types.MCPTool{{
		Name:        "search_my_week_issues",
		InputSchema: schema,
		Description: "bad\u202Edescription", // Cf（RTL 覆写）→ validateDescription 拒绝
	}}
	_, _, err = BuildVerifiedSnapshot(m3, live3)
	require.ErrorContains(t, err, "live description of tool")
	require.LessOrEqual(t, len(err.Error()), 400, "description-error prefix echo must be truncated")
}

func TestBuildVerifiedSnapshotRejectsMismatch(t *testing.T) {
	m := validManifest()
	// 远端实际 schema 与清单声明不同 → digest 不符
	live := []*types.MCPTool{{Name: "search_my_week_issues", InputSchema: []byte(`{"type":"object","properties":{"x":{}}}`)}}
	_, _, err := BuildVerifiedSnapshot(m, live)
	require.ErrorContains(t, err, "search_my_week_issues")
	// 远端缺声明工具 → 拒绝
	_, _, err = BuildVerifiedSnapshot(m, nil)
	require.ErrorContains(t, err, "missing")
	// 远端多出未声明工具 → 拒绝（差异工具名出现在错误中）
	live = []*types.MCPTool{
		{Name: "search_my_week_issues", InputSchema: []byte(declaredNoArgSchema)},
		{Name: "undeclared_extra_tool", InputSchema: []byte(`{}`)},
	}
	_, _, err = BuildVerifiedSnapshot(m, live)
	require.ErrorContains(t, err, "undeclared_extra_tool")
}

// TestFetchAndVerifyPreservesOAuthErrorChain (OCR T01-R1-2): the error
// returned for an OAuth-protected endpoint must keep BOTH identities — the
// ErrOAuthProtectedEndpoint sentinel AND the underlying *mcp.OAuthRequiredError
// (so IsOAuthProtected keeps working on wrapped errors). The fake lister here
// emulates the production adapter contract (internal/container's
// NewPluginMCPEndpointLister wraps the MCP error with the sentinel).
func TestFetchAndVerifyPreservesOAuthErrorChain(t *testing.T) {
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	t.Cleanup(utils.ResetSSRFWhitelistForTest)
	m := validManifest()
	manifestJSON, _ := json.Marshal(m)
	base := newControlledPluginHost(t,
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(manifestJSON)
		},
		func(http.ResponseWriter, *http.Request) {}, // endpoint never reached: fake lister
	)
	m.Transport.Endpoint = base + "/mcp"
	manifestJSON, _ = json.Marshal(m)
	lister := func(context.Context, string, string) ([]*types.MCPTool, error) {
		return nil, fmt.Errorf("%w: %w", ErrOAuthProtectedEndpoint, &mcp.OAuthRequiredError{
			MetadataURL: "https://auth.example.invalid/.well-known/oauth-protected-resource",
			Err:         errors.New("401 unauthorized"),
		})
	}
	_, err := FetchAndVerify(context.Background(), base+"/manifest.json", lister)
	require.ErrorIs(t, err, ErrOAuthProtectedEndpoint)
	require.True(t, IsOAuthProtected(err), "wrapped error must keep the OAuthRequiredError identity")
	var oauthErr *mcp.OAuthRequiredError
	require.ErrorAs(t, err, &oauthErr, "underlying *mcp.OAuthRequiredError must stay unwrappable")
}

// TestBuildVerifiedSnapshotScopesAreDefensivelyCopied (OCR T01-R2-2): the
// snapshot is the tenant-facing authority; its Scopes must not share backing
// arrays with the untrusted manifest document handed out in the same
// FetchResult.
func TestBuildVerifiedSnapshotScopesAreDefensivelyCopied(t *testing.T) {
	m := validManifest()
	m.Tools[0].Scopes = []string{"read:jira"}
	live := []*types.MCPTool{{Name: "search_my_week_issues", InputSchema: []byte(declaredNoArgSchema)}}
	snapshot, _, err := BuildVerifiedSnapshot(m, live)
	require.NoError(t, err)
	m.Tools[0].Scopes[0] = "mutated:jira"
	require.Equal(t, []string{"read:jira"}, snapshot[0].Scopes)
}

// TestBuildVerifiedSnapshotRejectsDuplicateLiveTool (OCR T01-R2-6): a live
// directory containing the same tool name twice is self-contradictory; the
// last entry must not silently mask the other's differing schema.
func TestBuildVerifiedSnapshotRejectsDuplicateLiveTool(t *testing.T) {
	m := validManifest()
	live := []*types.MCPTool{
		{Name: "search_my_week_issues", InputSchema: []byte(`{"type":"object","properties":{"rogue":{}}}`)},
		{Name: "search_my_week_issues", InputSchema: []byte(declaredNoArgSchema)},
	}
	_, _, err := BuildVerifiedSnapshot(m, live)
	require.ErrorContains(t, err, "search_my_week_issues")
}

// TestBuildVerifiedSnapshotRejectsOversizedLiveDescription (OCR T01-R2-4):
// live tool descriptions flow into the admin preview and persistence; an
// oversized one is rejected with the tool named, not silently snapshotted.
func TestBuildVerifiedSnapshotRejectsOversizedLiveDescription(t *testing.T) {
	m := validManifest()
	live := []*types.MCPTool{{
		Name:        "search_my_week_issues",
		Description: strings.Repeat("d", 1025),
		InputSchema: []byte(declaredNoArgSchema),
	}}
	_, _, err := BuildVerifiedSnapshot(m, live)
	require.ErrorContains(t, err, "search_my_week_issues")
}

// TestBuildVerifiedSnapshotRejectsBidiLiveDescription (OCR T01-R3-2): a live
// description carrying a bidi override / zero-width format character (Cf) is
// rejected — human review is the core safety gate and must not be shown
// visually-reordered text.
func TestBuildVerifiedSnapshotRejectsBidiLiveDescription(t *testing.T) {
	m := validManifest()
	live := []*types.MCPTool{{
		Name:        "search_my_week_issues",
		Description: "desc with \u202Eembedded RLO and a \u200Bzero width",
		InputSchema: []byte(declaredNoArgSchema),
	}}
	_, _, err := BuildVerifiedSnapshot(m, live)
	require.ErrorContains(t, err, "search_my_week_issues")
}

// TestBuildVerifiedSnapshotRejectsHostileLiveToolName (OCR T01-ocr-r1-001):
// live tool names are untrusted remote data — they must pass the same
// validateName hygiene as manifest names BEFORE any name is quoted into the
// rejection message, so a hostile endpoint cannot balloon or bidi-poison the
// admin-facing error with an oversized / invisible-character name.
func TestBuildVerifiedSnapshotRejectsHostileLiveToolName(t *testing.T) {
	m := validManifest()
	oversized := []*types.MCPTool{{Name: strings.Repeat("a", 5000), InputSchema: []byte(declaredNoArgSchema)}}
	_, _, err := BuildVerifiedSnapshot(m, oversized)
	require.ErrorContains(t, err, "live tools[0].name")
	require.NotContains(t, err.Error(), strings.Repeat("a", 200))

	bidi := []*types.MCPTool{{Name: "evil\u202Ename", InputSchema: []byte(declaredNoArgSchema)}}
	_, _, err = BuildVerifiedSnapshot(m, bidi)
	require.ErrorContains(t, err, "live tools[0].name")
	require.NotContains(t, err.Error(), "\u202E")
}

// TestBuildVerifiedSnapshotBoundsDiscrepancyList (OCR T01-ocr-r1-001): the
// rejection message lists at most maxVerificationProblems discrepancies and
// collapses the rest into a counter — a hostile endpoint returning a huge
// undeclared directory must not grow the single joined error unboundedly.
func TestBuildVerifiedSnapshotBoundsDiscrepancyList(t *testing.T) {
	m := validManifest()
	live := make([]*types.MCPTool, 0, 100)
	for i := 0; i < 100; i++ {
		live = append(live, &types.MCPTool{Name: fmt.Sprintf("extra%d", i), InputSchema: []byte(declaredNoArgSchema)})
	}
	_, _, err := BuildVerifiedSnapshot(m, live)
	// 1 missing declared tool + 100 undeclared = 101 problems, capped at 32 + counter.
	require.ErrorContains(t, err, "and 69 more discrepancies")
	require.NotContains(t, err.Error(), "extra40")
}

// TestBuildVerifiedSnapshotRejectsOversizedLiveDirectory (OCR T01-ocr-r4-003):
// the live tool directory is untrusted remote data with no transport-level
// size bound — BuildVerifiedSnapshot must cap how many entries it is willing
// to process before maps/digests/snapshot construction make the preview path
// O(n) on hostile input.
func TestBuildVerifiedSnapshotRejectsOversizedLiveDirectory(t *testing.T) {
	m := validManifest()
	live := make([]*types.MCPTool, 0, 1025)
	for i := 0; i < 1025; i++ {
		live = append(live, &types.MCPTool{Name: fmt.Sprintf("extra%d", i), InputSchema: []byte(declaredNoArgSchema)})
	}
	_, _, err := BuildVerifiedSnapshot(m, live)
	require.ErrorContains(t, err, "1025")
	require.ErrorContains(t, err, "maximum")

	// At exactly the cap the directory itself is not rejected for SIZE: the
	// mismatch (declared tool missing) is still the reported problem.
	atCap := live[:1024]
	_, _, err = BuildVerifiedSnapshot(m, atCap)
	require.ErrorContains(t, err, "disagree")
	require.NotContains(t, err.Error(), "maximum")
}

// TestBuildVerifiedSnapshotRejectsDuplicateDeclaredTool (OCR T01-ocr-r4-005):
// BuildVerifiedSnapshot is exported and must not silently rely on the caller
// having run ValidateManifest first — a manifest declaring the same tool name
// twice would otherwise produce a snapshot with duplicate entries and no
// error (the live-side duplicate check is the same defensive posture).
func TestBuildVerifiedSnapshotRejectsDuplicateDeclaredTool(t *testing.T) {
	m := validManifest()
	m.Tools = append(m.Tools, m.Tools[0]) // same name twice; ValidateManifest would reject
	live := []*types.MCPTool{{Name: "search_my_week_issues", InputSchema: []byte(declaredNoArgSchema)}}
	_, _, err := BuildVerifiedSnapshot(m, live)
	require.ErrorContains(t, err, "duplicate")
	require.ErrorContains(t, err, "search_my_week_issues")
}

// TestValidateManifestTruncatesUntrustedEchoes (OCR T01-ocr-r4-011): fields
// with no length check BEFORE their format/regex rejection (protocol,
// plugin_id, version, transport type, scope token) are echoed back in the
// error message — a hostile ~1MiB field must not balloon the single error
// (which reaches the HTTP response via the handler) into megabytes.
func TestValidateManifestTruncatesUntrustedEchoes(t *testing.T) {
	huge := strings.Repeat("x", 1<<20)
	cases := []func(m *types.PluginManifest){
		func(m *types.PluginManifest) { m.Protocol = huge },
		func(m *types.PluginManifest) { m.PluginID = huge },
		func(m *types.PluginManifest) { m.Version = huge },
		func(m *types.PluginManifest) { m.Transport.Type = huge },
		func(m *types.PluginManifest) { m.Tools[0].Scopes = []string{huge} },
	}
	for i, mutate := range cases {
		m := validManifest()
		mutate(m)
		err := ValidateManifest(m)
		require.Errorf(t, err, "case %d must be rejected", i)
		require.Less(t, len(err.Error()), 1024, "case %d error must be bounded", i)
		require.Contains(t, err.Error(), "more chars", "case %d must show a truncation counter", i)
	}
}

// TestFetchAndVerifyFailsFastOnNilLister (OCR T01-R3-3): a nil lister is a
// programming error; FetchAndVerify must reject it BEFORE any network I/O.
func TestFetchAndVerifyFailsFastOnNilLister(t *testing.T) {
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	t.Cleanup(utils.ResetSSRFWhitelistForTest)
	var requests atomic.Int32
	base := newControlledPluginHost(t,
		func(w http.ResponseWriter, _ *http.Request) {
			requests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		},
		func(w http.ResponseWriter, _ *http.Request) { requests.Add(1) },
	)
	_, err := FetchAndVerify(context.Background(), base+"/manifest.json", nil)
	require.ErrorContains(t, err, "lister")
	require.Zero(t, requests.Load(), "nil lister must fail before any network request is sent")
}
