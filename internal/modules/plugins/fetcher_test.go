package plugins

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/airesource/mcp"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/utils"
	sdkmcp "github.com/mark3labs/mcp-go/mcp"
	sdkserver "github.com/mark3labs/mcp-go/server"
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

// streamableMCPServer serves ONE tool whose input schema is the raw schema
// bytes, verbatim. The manifest fixture declares its digest over exactly
// these bytes, so the happy path exercises a MATCHING declaration. (The SDK's
// NewTool generates `{"properties":{},"required":[],"type":"object"}` for a
// no-arg tool — a different document — which is why the raw variant is used.)
func streamableMCPServer(t *testing.T, toolName string, rawSchema []byte) http.HandlerFunc {
	t.Helper()
	server := sdkserver.NewMCPServer("plugin-under-test", "1", sdkserver.WithToolCapabilities(false))
	server.AddTool(
		sdkmcp.NewToolWithRawSchema(toolName, "desc", rawSchema),
		func(_ context.Context, _ sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			return sdkmcp.NewToolResultText("ok"), nil
		},
	)
	transport := sdkserver.NewStreamableHTTPServer(server, sdkserver.WithStateLess(true))
	return transport.ServeHTTP
}

// nil2raw guards against a nil schema from the SDK: digest input stays `{}`.
func nil2raw(b []byte) []byte {
	if b == nil {
		return []byte(`{}`)
	}
	return b
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

func TestFetchAndVerifyHappyPath(t *testing.T) {
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	t.Cleanup(utils.ResetSSRFWhitelistForTest)
	m := validManifest()
	manifestJSON, _ := json.Marshal(m)
	base := newControlledPluginHost(t,
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(manifestJSON)
		},
		streamableMCPServer(t, "search_my_week_issues", []byte(declaredNoArgSchema)),
	)
	m.Transport.Endpoint = base + "/mcp"
	manifestJSON, _ = json.Marshal(m)
	manager := mcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	res, err := FetchAndVerify(context.Background(), base+"/manifest.json", NewMCPEndpointLister(manager))
	require.NoError(t, err)
	require.Equal(t, "com.example.jira-todo", res.Manifest.PluginID)
	require.Len(t, res.Snapshot, 1)
	require.Equal(t, ToolSchemaDigest(nil2raw(res.LiveTools[0].InputSchema)), res.Snapshot[0].InputSchemaDigest)
	require.NotEmpty(t, res.ToolsDigest)
	require.NotEmpty(t, res.IdentityFingerprint)
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
