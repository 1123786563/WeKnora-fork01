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

// TestFetchRejectsUserinfoCredentialsInManifestURL (OCR T01-R3-F2): the
// manifest URL must be refused BEFORE any request when it embeds userinfo
// credentials, mirroring ValidateManifest's transport-endpoint rule
// (T01-R1-F3). ValidateURLForSSRF never inspects parsed.User (Hostname()
// strips it), so without this gate the stdlib http client would send the
// credentials as a Basic Auth header to the host — asserted here directly
// by the controlled server — and the credential-bearing URL would be
// persisted into plugin_previews.manifest_url.
func TestFetchRejectsUserinfoCredentialsInManifestURL(t *testing.T) {
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	t.Cleanup(utils.ResetSSRFWhitelistForTest)
	var sawAuthHeader atomic.Bool
	m := validManifest()
	manifestJSON, _ := json.Marshal(m)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			sawAuthHeader.Store(true)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(manifestJSON)
	}))
	t.Cleanup(srv.Close)
	lister := func(context.Context, string, string) ([]*types.MCPTool, error) {
		t.Fatal("lister must not be called for a userinfo credential URL")
		return nil, nil
	}
	_, err := FetchAndVerify(context.Background(), "http://user:pw@"+srv.Listener.Addr().String()+"/manifest.json", lister)
	require.ErrorContains(t, err, "userinfo")
	require.False(t, sawAuthHeader.Load(),
		"credentials embedded in the manifest URL must never be sent to any host")
}

// TestBuildVerifiedSnapshotDeduplicatesUndeclaredEchoes (OCR T01-R3-F3): a
// live name appearing N times while undeclared used to emit (N-1) duplicate
// messages plus N undeclared messages for the SAME fact — one name, one
// contradiction. Each must be reported exactly once.
func TestBuildVerifiedSnapshotDeduplicatesUndeclaredEchoes(t *testing.T) {
	m := validManifest()
	live := []*types.MCPTool{
		{Name: "rogue", InputSchema: []byte(declaredNoArgSchema)},
		{Name: "rogue", InputSchema: []byte(declaredNoArgSchema)},
		{Name: "rogue", InputSchema: []byte(declaredNoArgSchema)},
	}
	_, _, err := BuildVerifiedSnapshot(m, live)
	require.Error(t, err)
	require.Equal(t, 1, strings.Count(err.Error(), `undeclared tool "rogue"`),
		"one undeclared name is one discrepancy, reported once")
	require.Equal(t, 1, strings.Count(err.Error(), `duplicate tool name "rogue"`),
		"one duplicated name is one discrepancy, reported once")
}

// TestBuildVerifiedSnapshotRechecksDeclaredNameAndScopes (OCR T01-R3-F4):
// BuildVerifiedSnapshot's contract says it must not silently rely on the
// caller having run ValidateManifest. It already re-checks duplicates,
// digest format and emptiness — but not the declaration's name hygiene and
// scopes: unchecked scopes would flow verbatim into the persisted snapshot
// and the admin/member authorization surfaces, and an unhygienic name gets
// a misleading "missing from live endpoint" (it can never match a vetted
// live name) instead of the real problem.
func TestBuildVerifiedSnapshotRechecksDeclaredNameAndScopes(t *testing.T) {
	// (1) Malformed scopes on a fully matching declaration: without the
	// re-check the snapshot is minted carrying the hostile scope.
	m := validManifest()
	m.Tools[0].Scopes = []string{strings.Repeat("s", maxScopeLen+1)} // ValidateManifest would reject this
	live := []*types.MCPTool{{Name: "search_my_week_issues", InputSchema: []byte(declaredNoArgSchema)}}
	snapshot, _, err := BuildVerifiedSnapshot(m, live)
	require.ErrorContains(t, err, "scope")
	require.Nil(t, snapshot, "a rejected verification must not produce a snapshot")

	// (2) Unhygienic declared name (Cf bidi override): must be rejected for
	// the name itself, not reported as missing from the endpoint.
	m2 := validManifest()
	m2.Tools[0].Name = "bad\u202Ename"
	_, _, err = BuildVerifiedSnapshot(m2, nil)
	require.ErrorContains(t, err, "must not contain control")
	require.NotContains(t, err.Error(), "missing from live endpoint")
}

// TestFetchBoundsRedirectErrorEcho (OCR T01-R4-F4): when the redirect
// policy or hop limit rejects a fetch, net/http wraps the error as
// *url.Error embedding the LAST redirect target verbatim — and the
// Location header is attacker-controlled and NOT bounded by
// maxManifestBytes (only by the transport's header limits). A hostile
// manifest host redirecting to a ~100KiB URL used to balloon "manifest
// fetch failed" into the 400 response via %w pass-through.
func TestFetchBoundsRedirectErrorEcho(t *testing.T) {
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	t.Cleanup(utils.ResetSSRFWhitelistForTest)
	longTail := strings.Repeat("y", 100*1024)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+"/hop"+longTail, http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	lister := func(context.Context, string, string) ([]*types.MCPTool, error) {
		t.Fatal("lister must not be called when the fetch itself fails")
		return nil, nil
	}
	_, err := FetchAndVerify(context.Background(), srv.URL+"/manifest.json", lister)
	require.Error(t, err)
	require.Less(t, len(err.Error()), 2048, "redirect-target echo must be bounded")
}

// TestFetchBoundsListerErrorEcho (OCR T01-R4-F6): the non-OAuth lister
// branch used to pass the adapter error through with %w — the production
// adapter surfaces the remote MCP server's JSON-RPC error.message
// verbatim, unbounded (tens of MB within the 30s timeout). Bound the echo;
// the OAuth sentinel branch keeps its pass-through (both identities
// preserved there by contract).
func TestFetchBoundsListerErrorEcho(t *testing.T) {
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	t.Cleanup(utils.ResetSSRFWhitelistForTest)
	m := validManifest()
	manifestJSON, _ := json.Marshal(m)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(manifestJSON)
	}))
	t.Cleanup(srv.Close)
	// The manifest's declared endpoint only needs to PASS the SSRF gate —
	// the lister below is a stub and never dials it.
	m.Transport.Endpoint = srv.URL + "/mcp"
	manifestJSON, _ = json.Marshal(m)
	lister := func(context.Context, string, string) ([]*types.MCPTool, error) {
		return nil, errors.New("remote says: " + strings.Repeat("z", 1<<20))
	}
	_, err := FetchAndVerify(context.Background(), srv.URL+"/manifest.json", lister)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrOAuthProtectedEndpoint)
	require.Less(t, len(err.Error()), 2048, "remote JSON-RPC error message echo must be bounded")
}

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
	// 名字须为合法长度（≤128）：本轮起 BuildVerifiedSnapshot 复检声明名
	// hygiene（OCR T01-R3-F4），3000 字符超长名在到达 duplicate 路径前即被
	// 名字复检拒绝（消息只含索引 where，天然有界）。100 字符合法名保持
	// duplicate 回显截断覆盖：echoQuoted 截到 maxEchoRunes=64 runes。
	longName := strings.Repeat("x", 100)
	m := validManifest()
	m.Tools = []types.PluginToolDecl{
		{Name: longName, InputSchemaDigest: strings.Repeat("a", 64)},
		{Name: longName, InputSchemaDigest: strings.Repeat("a", 64)},
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

	// 场景 1：missing——未匹配 live 的 decl.Name 原样回显。名字与 digest 均
	// 须合法（本轮起两者在 missing 前被复检，OCR T01-R1-F4/T01-R3-F4），
	// 100 字符合法长名保持 missing 回显截断覆盖（echoQuoted 截到 64 runes）。
	m := validManifest()
	m.Tools = []types.PluginToolDecl{{Name: strings.Repeat("x", 100), InputSchemaDigest: strings.Repeat("a", 64)}}
	_, _, err := BuildVerifiedSnapshot(m, nil)
	require.ErrorContains(t, err, "missing from live endpoint")
	require.LessOrEqual(t, len(err.Error()), 400, "missing-tool echo must be truncated")
	require.NotContains(t, err.Error(), strings.Repeat("x", 100))

	// 场景 2：任意长度假 digest——原威胁（进 schema digest mismatch 回显）
	// 已被本轮 digest 格式复检消灭：格式非法的声明在 mismatch 比较前即被
	// 拒绝，且拒绝消息不回显无界的 digest 值（mismatch 回显自此天然有界：
	// decl.Name 匹配 live 名 ≤128 前置、decl digest 恒 64-hex、liveDigest
	// 本地计算）。此处锁定新拒绝行为的回显有界性。
	m2 := validManifest()
	m2.Tools = []types.PluginToolDecl{{Name: "search_my_week_issues", InputSchemaDigest: long}}
	live := []*types.MCPTool{{Name: "search_my_week_issues", InputSchema: schema}}
	_, _, err = BuildVerifiedSnapshot(m2, live)
	require.ErrorContains(t, err, "input_schema_digest")
	require.ErrorContains(t, err, "64 lowercase hex")
	require.LessOrEqual(t, len(err.Error()), 400, "malformed-digest rejection must not echo the unbounded value")
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

// TestBuildVerifiedSnapshotRechecksDigestFormatAndEmptyTools (OCR T01-R1-F4):
// BuildVerifiedSnapshot is exported and must not silently rely on the caller
// having run ValidateManifest first. Two unchecked inputs currently break the
// "snapshot digests are always 64-hex" invariant when the function is called
// directly: (1) a declared InputSchemaDigest of "" matches a live tool whose
// schema is unparseable (ToolSchemaDigest returns "" for both) and produces an
// authoritative snapshot carrying an empty digest; (2) a manifest with no
// tools yields an empty snapshot plus the well-formed digest of "[]" instead
// of a rejection.
func TestBuildVerifiedSnapshotRechecksDigestFormatAndEmptyTools(t *testing.T) {
	// (1) Unvalidated manifest with an empty declared digest against a live
	// tool whose schema does not parse: "" == "" must NOT count as a match.
	m := validManifest()
	m.Tools[0].InputSchemaDigest = "" // ValidateManifest would reject this
	live := []*types.MCPTool{{Name: "search_my_week_issues", InputSchema: []byte(`{not-json`)}}
	snapshot, _, err := BuildVerifiedSnapshot(m, live)
	require.ErrorContains(t, err, "input_schema_digest")
	require.ErrorContains(t, err, "64 lowercase hex")
	require.Nil(t, snapshot, "a rejected verification must not produce a snapshot")

	// (2) A manifest declaring no tools (ValidateManifest would reject it)
	// must not produce an empty snapshot with a well-formed digest.
	m2 := validManifest()
	m2.Tools = nil
	snapshot2, digest, err := BuildVerifiedSnapshot(m2, nil)
	require.ErrorContains(t, err, "at least one tool")
	require.Nil(t, snapshot2)
	require.Empty(t, digest)
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
