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
