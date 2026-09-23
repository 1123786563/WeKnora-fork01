package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/plugins"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/utils"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	mcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

// newTestService 启动完整示例服务（OAuth + MCP + 自托管清单），Jira 指向 fake。
// 返回服务 base URL 与 fake Jira 的调用计数指针。
func newTestService(t *testing.T, jira http.HandlerFunc) (baseURL string, jiraCalls *int32) {
	t.Helper()
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	t.Cleanup(utils.ResetSSRFWhitelistForTest)
	var calls int32
	fakeJira := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		jira(w, r)
	}))
	t.Cleanup(fakeJira.Close)
	// 先占用端口拿真实 base URL（OAuth metadata / WWW-Authenticate / manifest
	// endpoint 都引用它），再把 handler 挂到同一监听上。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	base := "http://" + ln.Addr().String()
	handler, err := NewHandler(Options{BaseURL: base, JiraBaseURL: fakeJira.URL})
	require.NoError(t, err)
	svc := &http.Server{Handler: handler}
	go func() { _ = svc.Serve(ln) }()
	t.Cleanup(func() { _ = svc.Shutdown(context.Background()) })
	return base, &calls
}

// newMCPClient 建立到 /mcp 的已 initialize 客户端（构造器与仓库先例一致：
// internal/modules/airesource/mcp/client.go:221-226 使用
// client.NewStreamableHttpClient(url, transport.WithHTTPBasicClient(...), ...)）。
func newMCPClient(t *testing.T, base string) *client.Client {
	t.Helper()
	c, err := client.NewStreamableHttpClient(base+"/mcp",
		transport.WithHTTPBasicClient(&http.Client{}),
	)
	require.NoError(t, err)
	require.NoError(t, c.Start(context.Background()))
	t.Cleanup(func() { _ = c.Close() })
	_, err = c.Initialize(context.Background(), mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			Capabilities:    mcp.ClientCapabilities{},
			ClientInfo:      mcp.Implementation{Name: "test-client", Version: "1.0.0"},
		},
	})
	require.NoError(t, err)
	return c
}

func TestListToolsIsPublicAndSchemaIsFixed(t *testing.T) {
	base, _ := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Errorf("ListTools must not reach Jira")
	})
	// 1) SDK 客户端（WeKnora 同款构造器）：未认证 initialize/ListTools 可用，
	//    工具唯一、名字正确、schema 语义为无参数且拒绝额外属性。
	c := newMCPClient(t, base)
	tools, err := c.ListTools(context.Background(), mcp.ListToolsRequest{})
	require.NoError(t, err)
	require.Len(t, tools.Tools, 1)
	require.Equal(t, "search_my_week_issues", tools.Tools[0].Name)
	schema := tools.Tools[0].InputSchema
	require.Equal(t, "object", schema.Type)
	require.Empty(t, schema.Properties)
	require.Equal(t, false, schema.AdditionalProperties)

	// 2) 线上负载层：不经客户端结构再序列化（SDK 结构化 marshal 会追加
	//    "required":[] 工件），直接断言 JSON-RPC result 里的 inputSchema
	//    恒等于契约文档。
	_, sessionID := postJSONRPC(t, base, "initialize", map[string]any{
		"protocolVersion": mcp.LATEST_PROTOCOL_VERSION,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "raw-client", "version": "1.0.0"},
	}, 1)
	payload, _ := postJSONRPCSession(t, base, sessionID, "tools/list", map[string]any{}, 2)
	var listResult struct {
		Tools []struct {
			Name        string          `json:"name"`
			InputSchema json.RawMessage `json:"inputSchema"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(payload, &listResult))
	require.Len(t, listResult.Tools, 1)
	require.Equal(t, "search_my_week_issues", listResult.Tools[0].Name)
	require.JSONEq(t, canonicalToolInputSchema, string(listResult.Tools[0].InputSchema))
}

// postJSONRPC 发送一次无 session 的 JSON-RPC POST（initialize 用），返回
// 响应负载与服务端签发的 Mcp-Session-Id。
func postJSONRPC(t *testing.T, base, method string, params any, id int) (payload []byte, sessionID string) {
	t.Helper()
	return postJSONRPCSession(t, base, "", method, params, id)
}

// postJSONRPCSession 发送一次 JSON-RPC POST 并解析响应（application/json
// 或 text/event-stream 的 data 行），返回 JSON-RPC result 的原始字节。
func postJSONRPCSession(t *testing.T, base, sessionID, method string, params any, id int) (payload []byte, gotSessionID string) {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/mcp", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", raw)
	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "text/event-stream") {
		for _, line := range strings.Split(string(raw), "\n") {
			if data, ok := strings.CutPrefix(line, "data: "); ok {
				return []byte(strings.TrimSpace(data)), resp.Header.Get("Mcp-Session-Id")
			}
		}
		t.Fatalf("no data line in SSE response: %s", raw)
	}
	var rpc struct {
		Result json.RawMessage `json:"result"`
	}
	require.NoError(t, json.Unmarshal(raw, &rpc))
	require.NotNil(t, rpc.Result)
	return rpc.Result, resp.Header.Get("Mcp-Session-Id")
}

func TestCallToolWithoutTokenReturns401(t *testing.T) {
	base, jiraCalls := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Errorf("unauthenticated CallTool must not reach Jira")
	})

	// 1) 原始 HTTP 契约：POST JSON-RPC tools/call 无 Bearer → 401 +
	//    WWW-Authenticate: Bearer resource_metadata="<base>/.well-known/oauth-protected-resource"。
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": "search_my_week_issues", "arguments": map[string]any{}},
	})
	require.NoError(t, err)
	resp, err := http.Post(base+"/mcp", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	require.Equal(t,
		`Bearer resource_metadata="`+base+`/.well-known/oauth-protected-resource"`,
		resp.Header.Get("WWW-Authenticate"))

	// 2) MCP 客户端语义（WeKnora 客户端 OAuth 流的触发条件，client.go:119-136）：
	//    无 token CallTool → *transport.AuthorizationRequiredError 且携带 resource_metadata URL。
	c := newMCPClient(t, base)
	_, err = c.CallTool(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "search_my_week_issues"},
	})
	require.Error(t, err)
	var authErr *transport.AuthorizationRequiredError
	require.True(t, errors.As(err, &authErr), "expected AuthorizationRequiredError, got %v", err)
	require.Equal(t, base+"/.well-known/oauth-protected-resource", authErr.ResourceMetadataURL)
	require.EqualValues(t, 0, atomic.LoadInt32(jiraCalls))
}

func TestSelfHostedManifestMatchesContract(t *testing.T) {
	base, _ := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Errorf("manifest fetch must not reach Jira")
	})
	resp, err := http.Get(base + "/manifest.json")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var manifest types.PluginManifest
	require.NoError(t, json.Unmarshal(raw, &manifest))
	require.NoError(t, plugins.ValidateManifest(&manifest))
	require.Equal(t, plugins.PluginProtocolV1, manifest.Protocol)
	require.Equal(t, "jira-todo-mcp", manifest.PluginID)
	require.Equal(t, base+"/mcp", manifest.Transport.Endpoint)
	// digest 由代码计算（ToolSchemaDigest），绝不来自字面量。
	require.Equal(t,
		plugins.ToolSchemaDigest([]byte(canonicalToolInputSchema)),
		manifest.Tools[0].InputSchemaDigest)

	// 仓库内 manifest.json 参考副本同样必须通过协议校验，且 digest 与代码
	// 计算一致（endpoint 为部署占位符，部署后以动态 /manifest.json 为准）。
	reference, err := os.ReadFile("manifest.json")
	require.NoError(t, err)
	var referenceManifest types.PluginManifest
	require.NoError(t, json.Unmarshal(reference, &referenceManifest))
	require.NoError(t, plugins.ValidateManifest(&referenceManifest))
	require.Equal(t, manifest.Tools[0].InputSchemaDigest, referenceManifest.Tools[0].InputSchemaDigest)
}
