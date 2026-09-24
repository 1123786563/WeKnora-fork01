package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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

// --- T05：示例服务自测（fake Jira 全场景） ---

// testRandomFakeCredential 生成测试内随机生成的 fake Jira 凭据（邮箱 + API
// token）。凭据仅对本次测试的 httptest fake 有意义，与任何真实账号无关；
// 每次运行随机生成，源码不留下可用凭据字面量（全局约束 + T05 Brief）。
func testRandomFakeCredential(t *testing.T) (email, apiToken string) {
	t.Helper()
	buf := make([]byte, 16)
	_, err := rand.Read(buf)
	require.NoError(t, err)
	suffix := hex.EncodeToString(buf)
	return "member-" + suffix + "@example.test", "fake-token-" + suffix
}

// toolResultText 提取工具结果的首个文本块。
func toolResultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	require.NotEmpty(t, result.Content, "successful tool result must carry content")
	text, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "content[0] type = %T", result.Content[0])
	return text.Text
}

// jiraIssuePayload 构造 fake Jira 搜索响应里的单条事项。
func jiraIssuePayload(key, summary, status, due string) map[string]any {
	return map[string]any{
		"key": key,
		"fields": map[string]any{
			"summary": summary,
			"status":  map[string]any{"name": status},
			"duedate": due,
		},
	}
}

// TestOAuthCodeFlowIssuesMemberScopedToken 驱动完整授权码流：动态注册 →
// GET /authorize 表单 → POST 成员凭据 → 302 带一次性 code（state 原样回传）
// → POST /token（PKCE S256）换 access_token；令牌绑定成员会话（只查到该
// 成员的事项）。同时断言两条失败路径：错误凭据 → 凭据错误页且无 code；
// code 二次使用 → /token 拒绝。
func TestOAuthCodeFlowIssuesMemberScopedToken(t *testing.T) {
	email, apiToken := testRandomFakeCredential(t)
	issues := []map[string]any{jiraIssuePayload("A-1", "本周任务", "进行中", "2026-09-25")}
	base, _ := newTestService(t, testJiraAuthOK(t, email, apiToken, issues))
	redirectURI := "https://client.example/callback"

	// 1) POST /register → client_id。
	clientID := testRegisterClient(t, base, redirectURI)

	// 2) GET /authorize（PKCE S256）→ 表单；POST 错误凭据（fake Jira 401）
	//    → 凭据错误页且无 code；同 state 重试正确凭据 → 302 带 code。
	verifier, challenge := testPKCE(t)
	state := "t05-flow-state"
	require.Equal(t, http.StatusOK, testAuthorizeGET(t, base, clientID, redirectURI, state, challenge))
	status, location := testAuthorizePOST(t, base, state, email, "wrong-"+apiToken)
	require.Equal(t, http.StatusUnauthorized, status, "bad credentials must render the credential error page")
	require.Empty(t, location, "no authorization code may be issued for bad credentials")
	status, location = testAuthorizePOST(t, base, state, email, apiToken)
	require.Equal(t, http.StatusFound, status, "location: %s", location)
	parsed, err := url.Parse(location)
	require.NoError(t, err)
	code := parsed.Query().Get("code")
	require.NotEmpty(t, code, "successful authorization must issue a code")
	require.Equal(t, state, parsed.Query().Get("state"), "state must round-trip verbatim")

	// 3) POST /token（code + code_verifier）→ access_token。
	tokenStatus, payload := testExchangeCode(t, base, code, verifier, clientID, redirectURI)
	require.Equal(t, http.StatusOK, tokenStatus, "payload: %v", payload)
	accessToken, _ := payload["access_token"].(string)
	require.NotEmpty(t, accessToken)

	// 4) code 一次性：二次使用 → /token 拒绝。
	tokenStatus, payload = testExchangeCode(t, base, code, verifier, clientID, redirectURI)
	require.Equal(t, http.StatusBadRequest, tokenStatus, "a consumed code must not exchange twice")
	require.Equal(t, "invalid_grant", payload["error"])

	// 5) 令牌绑定成员会话：用该 token 调工具，只查到该成员的事项。
	result, err := callToolWithToken(t, base, accessToken)
	require.NoError(t, err)
	require.Contains(t, toolResultText(t, result), "[A-1] 本周任务",
		"the issued token must carry the authorizing member's session")
}

// TestTwoMembersIsolated 两成员数据隔离：fake Jira 按 Authorization 分账本
// 返回 A={A-1}、B={B-1}；成员各自走完整 OAuth 授权后，A 的 token 只查到
// A-1（含 fake Jira 的 /browse/A-1 链接），B 同理——绝无交叉泄漏。
func TestTwoMembersIsolated(t *testing.T) {
	emailA, tokenA := testRandomFakeCredential(t)
	emailB, tokenB := testRandomFakeCredential(t)
	basicA := "Basic " + base64.StdEncoding.EncodeToString([]byte(emailA+":"+tokenA))
	basicB := "Basic " + base64.StdEncoding.EncodeToString([]byte(emailB+":"+tokenB))
	// fakeJiraBase 在 fake 首次收到请求时捕获（浏览链接 = fakeJiraBase/browse/KEY）。
	fakeJiraBase := ""
	ledger := func(auth string) ([]map[string]any, bool) {
		switch auth {
		case basicA:
			return []map[string]any{jiraIssuePayload("A-1", "成员A的任务", "进行中", "2026-09-25")}, true
		case basicB:
			return []map[string]any{jiraIssuePayload("B-1", "成员B的任务", "待办", "2026-09-26")}, true
		default:
			return nil, false
		}
	}
	base, _ := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if fakeJiraBase == "" {
			fakeJiraBase = "http://" + r.Host
		}
		issues, known := ledger(r.Header.Get("Authorization"))
		if !known {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/rest/api/3/myself":
			_ = json.NewEncoder(w).Encode(map[string]any{"emailAddress": "known@example.test", "displayName": "Member"})
		case "/rest/api/3/search/jql":
			_ = json.NewEncoder(w).Encode(map[string]any{"issues": issues})
		default:
			t.Errorf("unexpected jira path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})

	accessA, _ := testOAuthFlow(t, base, "https://client-a.example/callback", emailA, tokenA)
	accessB, _ := testOAuthFlow(t, base, "https://client-b.example/callback", emailB, tokenB)

	resultA, err := callToolWithToken(t, base, accessA)
	require.NoError(t, err)
	textA := toolResultText(t, resultA)
	require.Contains(t, textA, "[A-1] 成员A的任务")
	require.Contains(t, textA, fakeJiraBase+"/browse/A-1", "member A's link must point at the issue source")
	require.NotContains(t, textA, "B-1", "member A's token must never see member B's issues")

	resultB, err := callToolWithToken(t, base, accessB)
	require.NoError(t, err)
	textB := toolResultText(t, resultB)
	require.Contains(t, textB, "[B-1] 成员B的任务")
	require.Contains(t, textB, fakeJiraBase+"/browse/B-1")
	require.NotContains(t, textB, "A-1", "member B's token must never see member A's issues")
}

// TestJiraErrorsSurfaceWithoutFabrication Jira 403/超时/凭据失效必须如实以
// 错误浮出（错误文本携带状态码/超时语义），绝不伪装成空成功列表。
func TestJiraErrorsSurfaceWithoutFabrication(t *testing.T) {
	email, apiToken := testRandomFakeCredential(t)
	redirectURI := "https://client.example/callback"

	// searchDenies 返回一个 myself 接受成员凭据、/search/jql 按 statusCode
	// 拒绝的 fake Jira（403 = 无权限；401 = 授权后 token 失效）。
	searchDenies := func(statusCode int) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/rest/api/3/myself":
				_ = json.NewEncoder(w).Encode(map[string]any{"emailAddress": email, "displayName": "Member"})
			case "/rest/api/3/search/jql":
				w.WriteHeader(statusCode)
				_ = json.NewEncoder(w).Encode(map[string]any{"errorMessages": []string{"denied"}})
			default:
				t.Errorf("unexpected jira path: %s", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		}
	}

	// 403 / 401（token 失效）：错误文本携带状态码，且不得是空成功列表。
	for _, tc := range []struct {
		name string
		code int
		want string
	}{
		{"jira-403", http.StatusForbidden, "403"},
		{"jira-401-token-invalid", http.StatusUnauthorized, "401"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base, _ := newTestService(t, searchDenies(tc.code))
			accessToken, _ := testOAuthFlow(t, base, redirectURI, email, apiToken)
			result, err := callToolWithToken(t, base, accessToken)
			require.Error(t, err, "a Jira %s must surface as an error, not an empty success list", tc.want)
			require.Nil(t, result, "an error path must not fabricate an empty successful result")
			require.ErrorContains(t, err, tc.want)
		})
	}

	// 超时：fake Jira 的响应延迟长于工具调用整体时限（maxToolCallTimeout 是
	// 供测试改写的整体 deadline），错误必须携带超时语义浮出。
	t.Run("jira-timeout", func(t *testing.T) {
		old := maxToolCallTimeout
		maxToolCallTimeout = 150 * time.Millisecond
		t.Cleanup(func() { maxToolCallTimeout = old })
		base, _ := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/rest/api/3/myself":
				_ = json.NewEncoder(w).Encode(map[string]any{"emailAddress": email, "displayName": "Member"})
			case "/rest/api/3/search/jql":
				time.Sleep(400 * time.Millisecond) // 慢于整体 deadline
				_ = json.NewEncoder(w).Encode(map[string]any{"issues": []map[string]any{}})
			default:
				t.Errorf("unexpected jira path: %s", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		})
		accessToken, _ := testOAuthFlow(t, base, redirectURI, email, apiToken)
		result, err := callToolWithToken(t, base, accessToken)
		require.Error(t, err, "a slow Jira must surface as a timeout error, not an empty success list")
		require.Nil(t, result, "an error path must not fabricate an empty successful result")
		require.ErrorContains(t, err, "timeout")
	})
}

// TestEmptyWeekIsEmptySuccess 空结果是合法成功：fake Jira 返回
// {"issues":[]} 时工具成功且输出为空文本（而非错误或虚构条目）。
func TestEmptyWeekIsEmptySuccess(t *testing.T) {
	email, apiToken := testRandomFakeCredential(t)
	base, jiraCalls := newTestService(t, testJiraAuthOK(t, email, apiToken, nil))
	accessToken, _ := testOAuthFlow(t, base, "https://client.example/callback", email, apiToken)
	result, err := callToolWithToken(t, base, accessToken)
	require.NoError(t, err, "an empty week is a legal success, not an error")
	require.Empty(t, toolResultText(t, result), "empty week must render as empty output")
	// 搜索确实到达过 fake Jira（myself ≥1 次 + search ≥1 次）——空成功来自
	// 真实的空响应，而非根本没发请求。
	require.GreaterOrEqual(t, atomic.LoadInt32(jiraCalls), int32(2),
		"the tool must actually query Jira; emptiness must come from a real empty response")
}

// callToolWithTokenAndArgs 以指定 Bearer 与参数调用 search_my_week_issues。
func callToolWithTokenAndArgs(t *testing.T, base, token string, arguments map[string]any) (*mcp.CallToolResult, error) {
	t.Helper()
	c, err := client.NewStreamableHttpClient(base+"/mcp",
		transport.WithHTTPBasicClient(&http.Client{}),
		transport.WithHTTPHeaders(map[string]string{"Authorization": "Bearer " + token}),
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
	return c.CallTool(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "search_my_week_issues", Arguments: arguments},
	})
}

// TestToolRejectsExtraneousArguments 模型传参攻击：调用工具时携带
// token/user_id/jql/url 等多余参数 → 服务端 schema（additionalProperties:
// false）校验失败，绝不成功执行，且不为该次调用发起任何 Jira 请求。
func TestToolRejectsExtraneousArguments(t *testing.T) {
	email, apiToken := testRandomFakeCredential(t)
	issues := []map[string]any{jiraIssuePayload("A-1", "本周任务", "进行中", "2026-09-25")}
	base, jiraCalls := newTestService(t, testJiraAuthOK(t, email, apiToken, issues))
	accessToken, _ := testOAuthFlow(t, base, "https://client.example/callback", email, apiToken)
	before := atomic.LoadInt32(jiraCalls) // OAuth 流已触达 myself；从这里起计增量

	result, err := callToolWithTokenAndArgs(t, base, accessToken, map[string]any{
		"token":   "attacker-supplied-token",
		"user_id": "42",
		"jql":     "assignee = someone-else()",
		"url":     "http://evil.example",
	})
	// 拒绝形态二者取一：JSON-RPC 错误（err != nil）或 SEP-1303 工具执行错误
	//（result.IsError）；但绝不能是成功结果。
	if err == nil {
		require.NotNil(t, result)
		require.True(t, result.IsError,
			"extraneous arguments must be rejected by additionalProperties:false, not executed")
		require.NotContains(t, toolResultText(t, result), "[A-1]",
			"a rejected call must not fabricate or leak issue data")
	}
	// 该次调用不得发起任何 Jira 请求（fake Jira 新增计数为 0）。
	require.Equal(t, before, atomic.LoadInt32(jiraCalls),
		"a rejected call must not reach Jira at all")
}
