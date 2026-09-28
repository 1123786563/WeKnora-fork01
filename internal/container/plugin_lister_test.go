package container

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
	"github.com/Tencent/WeKnora/internal/modules/plugins"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/utils"
	sdkmcp "github.com/mark3labs/mcp-go/mcp"
	sdkserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/require"
)

// 下述 fixture 与 internal/modules/plugins/manifest_test.go 中的同名 fixture
// 内容一致（跨包不可复用测试私有符号）；改动协议 fixture 时两处需同步。

// pluginDeclaredNoArgSchema is the input schema the fixture manifest declares
// for its single tool; the controlled MCP endpoint serves the exact same bytes
// so the declared digest matches the live one.
const pluginDeclaredNoArgSchema = `{"type":"object","properties":{},"additionalProperties":false}`

func pluginFixtureManifest() *types.PluginManifest {
	return &types.PluginManifest{
		Protocol:  "weknora.plugin/1",
		PluginID:  "com.example.jira-todo",
		Version:   "1.2.0",
		Name:      "Jira 本周待办",
		Transport: types.PluginTransport{Type: "http-streamable", Endpoint: "https://plugins.example.com/jira-todo/v1.2.0/mcp"},
		Auth:      &types.PluginAuth{PersonalOAuth: true},
		Tools: []types.PluginToolDecl{{
			Name: "search_my_week_issues", ReadOnly: true, RequiresPersonalAuth: true,
			InputSchemaDigest: plugins.ToolSchemaDigest([]byte(pluginDeclaredNoArgSchema)),
		}},
	}
}

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

// narrowSSRFWhitelistToLoopback narrows the package TestMain whitelist
// ("127.0.0.1,::1,localhost", see ssrf_test.go) to plain 127.0.0.1 for this
// test and RESTORES the prior state afterwards. The previous pattern paired
// the Set with utils.ResetSSRFWhitelistForTest, which CLEARS the whitelist
// instead of restoring it — with -count>=2 the alphabetically-later engine
// tests then ran with an empty whitelist and failed with
// "hostname 127.0.0.1 is restricted".
func narrowSSRFWhitelistToLoopback(t *testing.T) {
	t.Helper()
	t.Cleanup(utils.SnapshotSSRFWhitelistForTest())
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
}

func TestPluginFetchAndVerifyHappyPath(t *testing.T) {
	narrowSSRFWhitelistToLoopback(t)
	m := pluginFixtureManifest()
	manifestJSON, _ := json.Marshal(m)
	base := newControlledPluginHost(t,
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(manifestJSON)
		},
		streamableMCPServer(t, "search_my_week_issues", []byte(pluginDeclaredNoArgSchema)),
	)
	m.Transport.Endpoint = base + "/mcp"
	manifestJSON, _ = json.Marshal(m)
	manager := mcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	res, err := plugins.FetchAndVerify(context.Background(), base+"/manifest.json", NewPluginMCPEndpointLister(manager))
	require.NoError(t, err)
	require.Equal(t, "com.example.jira-todo", res.Manifest.PluginID)
	require.Len(t, res.Snapshot, 1)
	require.Equal(t, plugins.ToolSchemaDigest(nil2raw(res.LiveTools[0].InputSchema)), res.Snapshot[0].InputSchemaDigest)
	require.NotEmpty(t, res.ToolsDigest)
	require.NotEmpty(t, res.IdentityFingerprint)
}

// TestPluginListerMapsOAuthRequiredToSentinel：MCP 层的 *OAuthRequiredError 必须
// 被映射为 plugins.ErrOAuthProtectedEndpoint 哨兵，且双 %w 保留底层身份——
// 调用方既能 errors.Is 哨兵（plugins.IsOAuthProtected），也能 errors.As 取回
// *mcp.OAuthRequiredError。非 OAuth 错误原样透传。
func TestPluginListerMapsOAuthRequiredToSentinel(t *testing.T) {
	oauthErr := &mcp.OAuthRequiredError{
		MetadataURL: "https://auth.example.invalid/.well-known/oauth-protected-resource",
		Err:         errors.New("401 unauthorized"),
	}
	wrapped := wrapPluginOAuth(oauthErr)
	require.ErrorIs(t, wrapped, plugins.ErrOAuthProtectedEndpoint)
	require.True(t, plugins.IsOAuthProtected(wrapped))
	var back *mcp.OAuthRequiredError
	require.ErrorAs(t, wrapped, &back)
	require.Equal(t, oauthErr, back)

	plain := errors.New("connection refused")
	require.Same(t, plain, wrapPluginOAuth(plain), "non-OAuth errors must pass through unchanged")
}

// TestPluginConcurrentVerificationOfSameEndpointIsolated (OCR T01-R1-1): two
// verifications of the SAME endpoint must each own their connection. Sharing
// one manager-cached client lets the first finisher's Disconnect tear down
// the other's in-flight ListTools — a false-negative preview rejection.
//
// Reproduction: the endpoint holds the FIRST tools/list until the second one
// arrives (both verifications provably mid-flight), then answers the second
// one slowly. With a shared client the first finisher disconnects while the
// second request is still on the wire.
func TestPluginConcurrentVerificationOfSameEndpointIsolated(t *testing.T) {
	narrowSSRFWhitelistToLoopback(t)
	inner := streamableMCPServer(t, "search_my_week_issues", []byte(pluginDeclaredNoArgSchema))
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
	m := pluginFixtureManifest()
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
	lister := NewPluginMCPEndpointLister(manager)

	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = plugins.FetchAndVerify(context.Background(), base+"/manifest.json", lister)
		}()
	}
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
}

// TestWithListToolsDeadlineDerivesBound (OCR T01-R2-F8): the lister's
// ListTools call must carry a deadline even when the caller's ctx has none —
// the preview request ctx is only cancelled by the admin client disconnecting
// (cmd/server sets no http.Server timeouts, no route timeout middleware), so
// without a derived bound a hanging third-party endpoint pins the handler
// goroutine indefinitely. Every other production ListTools caller already
// derives 30s (mcp_tool.go listToolsTimeout; manager initializeClient 30s);
// the manifest fetch on the same preview path has 15s.
func TestWithListToolsDeadlineDerivesBound(t *testing.T) {
	ctx, cancel := withListToolsDeadline(context.Background())
	defer cancel()
	dl, ok := ctx.Deadline()
	require.True(t, ok, "a deadline-free caller ctx must still yield a bounded ListTools ctx")
	remaining := time.Until(dl)
	require.Greater(t, remaining, 25*time.Second)
	require.LessOrEqual(t, remaining, 30*time.Second)

	// A nearer caller-supplied deadline must win (context.WithTimeout keeps
	// the closer of the two): admin-side cancellation still propagates.
	parent, parentCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer parentCancel()
	child, childCancel := withListToolsDeadline(parent)
	defer childCancel()
	childDL, _ := child.Deadline()
	parentDL, _ := parent.Deadline()
	require.False(t, childDL.After(parentDL), "the nearer parent deadline must be preserved")
}

// TestPluginListerBoundsHangingListTools (OCR T01-R2-F8 regression anchor):
// an endpoint that completes the initialize handshake but then holds
// tools/list open must not outlive the deadline the caller derived — the
// lister must surface the ctx error instead of returning after the endpoint
// eventually answers. (This exercises deadline propagation through the whole
// GetOrCreateClient → ListTools chain; the 30s default bound itself is
// covered by TestWithListToolsDeadlineDerivesBound.)
func TestPluginListerBoundsHangingListTools(t *testing.T) {
	narrowSSRFWhitelistToLoopback(t)
	inner := streamableMCPServer(t, "search_my_week_issues", []byte(pluginDeclaredNoArgSchema))
	mcpHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			body, readErr := io.ReadAll(r.Body)
			if readErr == nil {
				if bytes.Contains(body, []byte(`"tools/list"`)) {
					// Hold tools/list until the request itself is abandoned.
					select {
					case <-r.Context().Done():
						return
					case <-time.After(10 * time.Second):
					}
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
			}
		}
		inner(w, r)
	}
	m := pluginFixtureManifest()
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

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := plugins.FetchAndVerify(ctx, base+"/manifest.json", NewPluginMCPEndpointLister(manager))
	require.Error(t, err)
	require.Less(t, time.Since(start), 5*time.Second,
		"a hanging tools/list must surface the ctx deadline error, not wait for the endpoint")
}

// TestPluginEndpointListerRetiresPendingConnectionOnError (OCR T01-R2-1): when the
// caller's ctx expires mid-handshake, GetOrCreateClient returns an error
// while its background goroutine (manager lifeCtx) keeps connecting and would
// mount the connected client under a unique nonce key nothing will ever
// reference again. The lister must retire the key on the error path, which
// cancels the in-flight handshake — observed here as the controlled endpoint
// losing the initialize request mid-flight.
func TestPluginEndpointListerRetiresPendingConnectionOnError(t *testing.T) {
	narrowSSRFWhitelistToLoopback(t)
	inner := streamableMCPServer(t, "search_my_week_issues", []byte(pluginDeclaredNoArgSchema))
	var initializeAborted atomic.Bool
	mcpHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			body, readErr := io.ReadAll(r.Body)
			if readErr == nil {
				if bytes.Contains(body, []byte(`"method":"initialize"`)) {
					select {
					case <-r.Context().Done():
						initializeAborted.Store(true)
						return
					case <-time.After(500 * time.Millisecond):
						// proceed: serve the initialize normally
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
	m := pluginFixtureManifest()
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

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_, err := plugins.FetchAndVerify(ctx, base+"/manifest.json", NewPluginMCPEndpointLister(manager))
	require.Error(t, err) // caller ctx expired while the endpoint stalled the handshake

	// Let the stalled handshake observe whether the client abandoned it.
	time.Sleep(800 * time.Millisecond)
	require.True(t, initializeAborted.Load(),
		"pending connection must be retired (CloseClient) when GetOrCreateClient fails; a completed client under an unreferenced nonce key leaks (idle cleanup only removes !IsConnected entries)")
}
