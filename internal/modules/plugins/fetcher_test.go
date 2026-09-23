package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

// TestFetchAndVerifyPreservesOAuthErrorChain (OCR T01-R1-2): the error
// returned for an OAuth-protected endpoint must keep BOTH identities — the
// ErrOAuthProtectedEndpoint sentinel AND the underlying *mcp.OAuthRequiredError
// (so IsOAuthProtected keeps working on wrapped errors).
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
		return nil, &mcp.OAuthRequiredError{
			MetadataURL: "https://auth.example.invalid/.well-known/oauth-protected-resource",
			Err:         errors.New("401 unauthorized"),
		}
	}
	_, err := FetchAndVerify(context.Background(), base+"/manifest.json", lister)
	require.ErrorIs(t, err, ErrOAuthProtectedEndpoint)
	require.True(t, IsOAuthProtected(err), "wrapped error must keep the OAuthRequiredError identity")
}

// TestConcurrentVerificationOfSameEndpointIsolated (OCR T01-R1-1): two
// verifications of the SAME endpoint must each own their connection. Sharing
// one manager-cached client lets the first finisher's Disconnect tear down
// the other's in-flight ListTools — a false-negative preview rejection.
//
// Reproduction: the endpoint holds the FIRST tools/list until the second one
// arrives (both verifications provably mid-flight), then answers the second
// one slowly. With a shared client the first finisher disconnects while the
// second request is still on the wire.
func TestConcurrentVerificationOfSameEndpointIsolated(t *testing.T) {
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	t.Cleanup(utils.ResetSSRFWhitelistForTest)
	inner := streamableMCPServer(t, "search_my_week_issues", []byte(declaredNoArgSchema))
	var toolsListArrivals atomic.Int32
	secondArrived := make(chan struct{})
	var releaseOnce sync.Once
	mcpHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			body, readErr := io.ReadAll(r.Body)
			if readErr == nil {
				if bytes.Contains(body, []byte(`"tools/list"`)) {
					if toolsListArrivals.Add(1) == 1 {
						// Hold the first tools/list until the second arrives so
						// both verifications are in flight on the endpoint.
						select {
						case <-secondArrived:
						case <-time.After(5 * time.Second):
						}
					} else {
						releaseOnce.Do(func() { close(secondArrived) })
						// The later request answers slowly: under the
						// shared-client bug the first finisher's Disconnect
						// cancels it mid-flight.
						time.Sleep(300 * time.Millisecond)
					}
					r.Body = io.NopCloser(bytes.NewReader(body))
					inner(w, r)
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
			}
		}
		inner(w, r)
	}
	m := validManifest()
	manifestJSON, _ := json.Marshal(m)
	base := newControlledPluginHost(t,
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(manifestJSON)
		},
		mcpHandler,
	)
	m.Transport.Endpoint = base + "/mcp"
	manifestJSON, _ = json.Marshal(m)
	manager := mcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	lister := NewMCPEndpointLister(manager)

	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = FetchAndVerify(context.Background(), base+"/manifest.json", lister)
		}()
	}
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
}
