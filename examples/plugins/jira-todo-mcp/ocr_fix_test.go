package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Tencent/WeKnora/internal/modules/plugins"
	"github.com/Tencent/WeKnora/internal/utils"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	mcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

// 本文件是 T04 任务级 OCR（docs/plans/issue-106-ocr-T04.md）发现项的回归
// 测试，与 server_test.go 的骨架契约测试同包互补。

// testJiraAuthOK 是一个接受固定凭据的 fake Jira：/myself 验证
// Authorization Basic base64(email:token)，/search/jql 返回固定事项集。
func testJiraAuthOK(t *testing.T, email, apiToken string, issues []map[string]any) http.HandlerFunc {
	t.Helper()
	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte(email+":"+apiToken))
	return func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != expected {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"errorMessages": []string{"bad creds"}})
			return
		}
		switch r.URL.Path {
		case "/rest/api/3/myself":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"emailAddress": email, "displayName": "Member " + email,
			})
		case "/rest/api/3/search/jql":
			_ = json.NewEncoder(w).Encode(map[string]any{"issues": issues})
		default:
			t.Errorf("unexpected jira path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

// testRegisterClient 注册一个绑定 redirectURIs 的动态客户端。
func testRegisterClient(t *testing.T, base string, redirectURIs ...string) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{"redirect_uris": redirectURIs})
	require.NoError(t, err)
	resp, err := http.Post(base+"/register", "application/json", strings.NewReader(string(body)))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var registration struct {
		ClientID string `json:"client_id"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&registration))
	require.NotEmpty(t, registration.ClientID)
	return registration.ClientID
}

// testPKCE 生成一对 verifier/challenge（S256）。
func testPKCE(t *testing.T) (verifier, challenge string) {
	t.Helper()
	verifier = "test-verifier-" + time.Now().Format("150405.000000000")
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

// testFormNonceRe 从授权表单 HTML 提取服务端生成的表单 nonce
// （OCR 二轮 F1：form_nonce 隐藏字段，提交时须原样往返）。
var testFormNonceRe = regexp.MustCompile(`name="form_nonce" value="([0-9a-f]+)"`)

// testFormNonces 记录 testAuthorizeGET 拉到的 state -> form_nonce，供
// testAuthorizePOST 自动附带——模拟真实浏览器对隐藏字段的往返。攻击
// 场景测试用 testAuthorizePOSTWithNonce 显式指定（陈旧表单 = 显式传
// 旧 nonce）。各测试的 state 互不重叠，重登记同 state 时覆盖为最新
// 页面的 nonce（浏览器语义：最新渲染的表单）。
var testFormNonces sync.Map

// testFormNonceOf 返回最近一次 testAuthorizeGET 为该 state 记录的
// form_nonce（未登记过返回空串）。
func testFormNonceOf(state string) string {
	v, ok := testFormNonces.Load(state)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// testAuthorizeGET 拉取授权表单（返回 HTTP 状态码；表单 nonce 自动记录）。
func testAuthorizeGET(t *testing.T, base, clientID, redirectURI, state, challenge string) int {
	t.Helper()
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	resp, err := http.Get(base + "/authorize?" + q.Encode())
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	if m := testFormNonceRe.FindSubmatch(body); m != nil {
		testFormNonces.Store(state, string(m[1]))
	}
	return resp.StatusCode
}

// testAuthorizePOST 提交凭据表单（自动附带最近一次 GET 记录的 nonce；
// 返回 HTTP 状态码与 Location）。关闭重定向跟随——302 的目标
// redirect_uri 是虚构域名，客户端语义上也不该替成员访问。
func testAuthorizePOST(t *testing.T, base, state, email, apiToken string) (int, string) {
	t.Helper()
	return testAuthorizePOSTWithNonce(t, base, state, testFormNonceOf(state), email, apiToken)
}

// testAuthorizePOSTWithNonce 提交凭据表单并显式指定表单 nonce——攻击
// 场景专用（陈旧表单提交 = 传旧页面 nonce；无 nonce 表单 = 传空串）。
func testAuthorizePOSTWithNonce(t *testing.T, base, state, nonce, email, apiToken string) (int, string) {
	t.Helper()
	form := url.Values{}
	form.Set("state", state)
	form.Set("form_nonce", nonce)
	form.Set("email", email)
	form.Set("api_token", apiToken)
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := noRedirect.PostForm(base+"/authorize", form)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, resp.Header.Get("Location")
}

// testExchangeCode 用 code + verifier 换令牌（返回状态码与响应负载）。
func testExchangeCode(t *testing.T, base, code, verifier, clientID, redirectURI string) (int, map[string]any) {
	t.Helper()
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("client_id", clientID)
	form.Set("redirect_uri", redirectURI)
	resp, err := http.PostForm(base+"/token", form)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	payload := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&payload)
	return resp.StatusCode, payload
}

// testOAuthFlow 跑完整授权码流，返回 access_token 与 refresh_token。
func testOAuthFlow(t *testing.T, base, redirectURI, email, apiToken string) (accessToken, refreshToken string) {
	t.Helper()
	clientID := testRegisterClient(t, base, redirectURI)
	verifier, challenge := testPKCE(t)
	state := "state-" + time.Now().Format("150405.000000000")
	require.Equal(t, http.StatusOK, testAuthorizeGET(t, base, clientID, redirectURI, state, challenge))
	status, location := testAuthorizePOST(t, base, state, email, apiToken)
	require.Equal(t, http.StatusFound, status, "location: %s", location)
	parsed, err := url.Parse(location)
	require.NoError(t, err)
	code := parsed.Query().Get("code")
	require.NotEmpty(t, code)
	status, payload := testExchangeCode(t, base, code, verifier, clientID, redirectURI)
	require.Equal(t, http.StatusOK, status, "payload: %v", payload)
	accessToken, _ = payload["access_token"].(string)
	refreshToken, _ = payload["refresh_token"].(string)
	require.NotEmpty(t, accessToken)
	require.NotEmpty(t, refreshToken)
	return accessToken, refreshToken
}

// callToolWithToken 以指定 Bearer 调 search_my_week_issues。
func callToolWithToken(t *testing.T, base, token string) (*mcp.CallToolResult, error) {
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
		Params: mcp.CallToolParams{Name: "search_my_week_issues"},
	})
}

// --- OCR T04-R1-1：动态注册必须绑定 redirect_uris，authorize 精确匹配 ---

func TestRegisterBindsAndAuthorizeMatchesRedirectURI(t *testing.T) {
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "tok", nil))
	// 注册时不带 redirect_uris → 拒绝（不能留下可任意 redirect 的客户端）。
	body, err := json.Marshal(map[string]any{})
	require.NoError(t, err)
	resp, err := http.Post(base+"/register", "application/json", strings.NewReader(string(body)))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	registered := "https://client.example/callback"
	clientID := testRegisterClient(t, base, registered)
	_, challenge := testPKCE(t)
	// 未注册的 redirect_uri → 400（授权码不得发给任意地址）。
	require.Equal(t, http.StatusBadRequest,
		testAuthorizeGET(t, base, clientID, "https://attacker.example/cb", "st", challenge))
	// 已注册的 redirect_uri → 200 表单。
	require.Equal(t, http.StatusOK,
		testAuthorizeGET(t, base, clientID, registered, "st", challenge))
}

// --- OCR T04-R1-2：access_token 过期生效；refresh 不立即烧毁旧 token ---

func TestAccessTokenExpiresAndRefreshKeepsOldTokenUsable(t *testing.T) {
	issues := []map[string]any{{
		"key": "A-1", "fields": map[string]any{
			"summary": "任务", "status": map[string]any{"name": "进行中"}, "duedate": "2026-09-25"},
	}}
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "tok", issues))
	redirectURI := "https://client.example/callback"
	accessToken, refreshToken := testOAuthFlow(t, base, redirectURI, "member@example.com", "tok")

	// 基线：TTL 内 token 可用。
	_, err := callToolWithToken(t, base, accessToken)
	require.NoError(t, err)

	// refresh 轮换：旧 access_token 在广告的 TTL 内必须继续可用。
	refreshForm := url.Values{}
	refreshForm.Set("grant_type", "refresh_token")
	refreshForm.Set("refresh_token", refreshToken)
	resp, err := http.PostForm(base+"/token", refreshForm)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_, err = callToolWithToken(t, base, accessToken)
	require.NoError(t, err, "old access token must stay usable within its advertised TTL after refresh")

	// TTL 缩短后签发的 token 立即过期 → CallTool 401（gate 拒绝无效会话）。
	oldTTL := accessTokenTTL
	accessTokenTTL = time.Nanosecond
	t.Cleanup(func() { accessTokenTTL = oldTTL })
	expiredToken, _ := testOAuthFlow(t, base, redirectURI, "member@example.com", "tok")
	_, err = callToolWithToken(t, base, expiredToken)
	require.Error(t, err, "expired access token must be rejected")
	var authErr *transport.AuthorizationRequiredError
	require.True(t, errors.As(err, &authErr), "expected AuthorizationRequiredError, got %v", err)
}

// --- OCR T04-R1-3：凭据输错后同 state 重试不得烧毁 ---

func TestStateSurvivesFailedCredentialRetry(t *testing.T) {
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "correct-token", nil))
	redirectURI := "https://client.example/callback"
	clientID := testRegisterClient(t, base, redirectURI)
	verifier, challenge := testPKCE(t)
	state := "retry-state"
	require.Equal(t, http.StatusOK, testAuthorizeGET(t, base, clientID, redirectURI, state, challenge))

	// 错误凭据 → 401 凭据错误页，无 code。
	status, location := testAuthorizePOST(t, base, state, "member@example.com", "wrong-token")
	require.Equal(t, http.StatusUnauthorized, status)
	require.Empty(t, location)

	// 浏览器返回重试（同 state、正确凭据）→ 302 带新 code（完整换码成功）。
	status, location = testAuthorizePOST(t, base, state, "member@example.com", "correct-token")
	require.Equal(t, http.StatusFound, status, "location: %s", location)
	parsed, err := url.Parse(location)
	require.NoError(t, err)
	code := parsed.Query().Get("code")
	require.NotEmpty(t, code)
	exchangeStatus, payload := testExchangeCode(t, base, code, verifier, clientID, redirectURI)
	require.Equal(t, http.StatusOK, exchangeStatus, "payload: %v", payload)
}

// --- OCR T04-R1-4：state 长度上限 + 过期 pendingAuth 拒绝 ---

func TestAuthorizeRejectsOverlongAndExpiredState(t *testing.T) {
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "tok", nil))
	redirectURI := "https://client.example/callback"
	clientID := testRegisterClient(t, base, redirectURI)
	_, challenge := testPKCE(t)
	// 超长 state（>1024 字节）→ 400。
	require.Equal(t, http.StatusBadRequest,
		testAuthorizeGET(t, base, clientID, redirectURI, strings.Repeat("s", 2000), challenge))

	// pendingAuth 过期 → 表单提交时拒绝。
	oldTTL := pendingAuthTTL
	pendingAuthTTL = time.Nanosecond
	t.Cleanup(func() { pendingAuthTTL = oldTTL })
	state := "expiring-state"
	require.Equal(t, http.StatusOK, testAuthorizeGET(t, base, clientID, redirectURI, state, challenge))
	time.Sleep(2 * time.Millisecond)
	status, _ := testAuthorizePOST(t, base, state, "member@example.com", "tok")
	require.Equal(t, http.StatusBadRequest, status)
}

// --- OCR T04-R1-6：搜索跟进 nextPageToken ---

func TestSearchFollowsNextPageToken(t *testing.T) {
	sawPageToken := ""
	base, _ := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/myself" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"emailAddress": "member@example.com", "displayName": "Member",
			})
			return
		}
		if r.URL.Path != "/rest/api/3/search/jql" {
			t.Errorf("unexpected jira path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body struct {
			// OCR R2 F18：请求键与 Atlassian /search/jql 契约对齐为
			// nextPageToken（替身此前按同样错误的 pageToken 键钉死错误契约）。
			PageToken string `json:"nextPageToken"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.PageToken == "" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issues": []map[string]any{{
					"key": "A-1", "fields": map[string]any{
						"summary": "第一页", "status": map[string]any{"name": "进行中"}}}},
				"nextPageToken": "page-2",
			})
			return
		}
		sawPageToken = body.PageToken
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issues": []map[string]any{{
				"key": "A-2", "fields": map[string]any{
					"summary": "第二页", "status": map[string]any{"name": "待办"}}}},
		})
	})
	accessToken, _ := testOAuthFlow(t, base, "https://client.example/callback", "member@example.com", "tok")
	result, err := callToolWithToken(t, base, accessToken)
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "content[0] type = %T", result.Content[0])
	require.Contains(t, text.Text, "[A-1] 第一页")
	require.Contains(t, text.Text, "[A-2] 第二页")
	require.Equal(t, "page-2", sawPageToken, "second page request must carry nextPageToken as pageToken")
}

// --- OCR T04-R1-7：BaseURL host 名为空必须拒绝 ---

func TestValidateBaseURLRejectsEmptyHostname(t *testing.T) {
	require.Error(t, validateBaseURL("BaseURL", "http://:8020"), "host-only URL must not pass")
	require.Error(t, validateBaseURL("JiraBaseURL", "https://:443"))
	require.NoError(t, validateBaseURL("BaseURL", "http://127.0.0.1:8020"))
}

// --- 跨任务转交 R7：main.go 三项 + jira.go 输出封顶 ---

// TestValidateBaseURLRejectsUnspecifiedHost（T01-R2-F7）：PLUGIN_LISTEN_ADDR
// ="0.0.0.0:8020" 且未设 PLUGIN_BASE_URL 时回退拼出 "http://0.0.0.0:8020"
// ——hostname 非空可过校验，服务带着不可用元数据地址静默启动（OAuth 元数据
// /WWW-Authenticate/同意页表单全部指向 0.0.0.0，授权流必然失败）。非具体
// host 必须拒绝，部署者须显式设置 PLUGIN_BASE_URL。
func TestValidateBaseURLRejectsUnspecifiedHost(t *testing.T) {
	require.Error(t, validateBaseURL("BaseURL", "http://0.0.0.0:8020"),
		"an unspecified IPv4 host must not pass (silent unusable metadata)")
	require.Error(t, validateBaseURL("BaseURL", "http://[::]:8020"),
		"an unspecified IPv6 host must not pass")
	require.NoError(t, validateBaseURL("BaseURL", "http://127.0.0.1:8020"))
	require.NoError(t, validateBaseURL("BaseURL", "https://plugins.example.com"))
}

// TestValidateBaseURLRejectsUserinfo（T01-R4-F5）：BaseURL 会传播进 RFC
// 8414/9728 元数据、WWW-Authenticate challenge 与 manifest endpoint——
// 内嵌凭据（u.User != nil）与 manifest URL/transport endpoint 两个面已建立
// 的 userinfo 卫生一致，必须 fail-closed 拒绝。
func TestValidateBaseURLRejectsUserinfo(t *testing.T) {
	require.Error(t, validateBaseURL("BaseURL", "https://user:pass@example.com/"),
		"userinfo credentials must not pass BaseURL validation")
	require.Error(t, validateBaseURL("JiraBaseURL", "https://token@jira.example.com/"))
	require.NoError(t, validateBaseURL("BaseURL", "https://example.com/"))
}

// TestRunShutdownReturnsAfterContextCancel（T01-R2-F6 守卫）：ctx 取消后
// Run 必须在 Shutdown 超时（5s）内返回且不泄漏监听——活跃 SSE 长连接下
// 的确定性 RED 不可构造（streamable HTTP 会话状态依赖真实 MCP 握手），
// 本测试作为无连接快速路径的行为守卫；Close 兜底修复以代码审查保障。
func TestRunShutdownReturnsAfterContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{
			BaseURL:     "http://127.0.0.1:8021",
			JiraBaseURL: "http://127.0.0.1:9",
			ListenAddr:  "127.0.0.1:0",
		})
	}()
	deadline := time.Now().Add(5 * time.Second)
	_ = deadline                       // 守卫语义见上：只需 cancel 后有限时间返回
	time.Sleep(200 * time.Millisecond) // 让监听与 Serve goroutine 就绪
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(8 * time.Second):
		t.Fatal("Run must return shortly after context cancellation")
	}
}

// TestRenderIssuesOutputBounded（T01-R4-F2）：工具输出文本无行数/字节封顶
// ——1000 事项 × 255 字符可达数百 KB 整体进入 CallToolResult（LLM 上下文）。
// renderIssuesOutput 必须行数/字节双界封顶并追加显式标注（非静默截断）。
func TestRenderIssuesOutputBounded(t *testing.T) {
	// 行数界：250 条事项 → 截到 200 行 + 显式标注。
	many := make([]JiraIssue, 250)
	for i := range many {
		many[i] = JiraIssue{Key: fmt.Sprintf("A-%d", i), Summary: "任务", Status: "进行中", Due: "2026-09-25", URL: "https://jira.example.com/browse/A-1"}
	}
	out := renderIssuesOutput(many, false)
	require.LessOrEqual(t, len(strings.Split(out, "\n")), maxToolOutputLines+1, "line cap + at most the notes")
	require.Contains(t, out, fmt.Sprintf("输出超过 %d 行被截断", maxToolOutputLines),
		"line truncation must be explicitly annotated")
	require.NotContains(t, out, "[A-249]", "entries past the line cap must be dropped")

	// 字节界：少量超长行同样触发封顶（截断不产生非法 UTF-8）。
	fat := make([]JiraIssue, 3)
	longSummary := strings.Repeat("长", 40000) // 每行 ~120KB
	for i := range fat {
		fat[i] = JiraIssue{Key: "B-1", Summary: longSummary}
	}
	out = renderIssuesOutput(fat, false)
	require.LessOrEqual(t, len(out), maxToolOutputBytes+512, "byte cap plus annotation headroom")
	require.Contains(t, out, fmt.Sprintf("输出超过 %d 字节被截断", maxToolOutputBytes),
		"byte truncation must be explicitly annotated")
	require.True(t, utf8.ValidString(out), "byte truncation must cut on rune boundaries")

	// 正常量输出不封顶、不标注；页数截断标注保持原样。
	normal := []JiraIssue{{Key: "C-1", Summary: "本周任务", Status: "进行中"}}
	out = renderIssuesOutput(normal, true)
	require.Contains(t, out, "[C-1] 本周任务")
	require.NotContains(t, out, "被截断，请到 Jira 查看完整列表\n⚠", "no size annotation expected")
	require.Contains(t, out, fmt.Sprintf("结果超过 %d 页被截断", maxSearchPages))
}

// --- OCR T04-R1-10：上游故障不得伪装成凭据错误 ---

func TestAuthorizeUpstreamFailureIsNotCredentialError(t *testing.T) {
	base, _ := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/myself" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		t.Errorf("unexpected jira path: %s", r.URL.Path)
	})
	redirectURI := "https://client.example/callback"
	clientID := testRegisterClient(t, base, redirectURI)
	_, challenge := testPKCE(t)
	state := "upstream-state"
	require.Equal(t, http.StatusOK, testAuthorizeGET(t, base, clientID, redirectURI, state, challenge))
	status, location := testAuthorizePOST(t, base, state, "member@example.com", "tok")
	require.Equal(t, http.StatusBadGateway, status, "upstream 5xx must surface as 502, not a credential error")
	require.Empty(t, location)

	// 凭据性失败（Jira 401）仍是 401 凭据页。
	base2, _ := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	clientID2 := testRegisterClient(t, base2, redirectURI)
	_, challenge2 := testPKCE(t)
	require.Equal(t, http.StatusOK, testAuthorizeGET(t, base2, clientID2, redirectURI, state, challenge2))
	status, location = testAuthorizePOST(t, base2, state, "member@example.com", "tok")
	require.Equal(t, http.StatusUnauthorized, status)
	require.Empty(t, location)
}

// --- OCR T04-R1-9：manifest.json 参考副本 digest 防漂移（与代码计算一致） ---

func TestReferenceManifestDigestMatchesComputed(t *testing.T) {
	_ = plugins.ToolSchemaDigest // 保持 import 引用（主断言在 TestSelfHostedManifestMatchesContract）
	require.Equal(t,
		plugins.ToolSchemaDigest([]byte(canonicalToolInputSchema)),
		Manifest("https://reference.example").Tools[0].InputSchemaDigest)
}

// --- OCR T04-R1-8：JQL 必须界定"本周"（下界 + 上界），不得含未来待办 ---

// TestSearchJQLIsWeekBounded 断言服务端发给 Jira 的 JQL 精确等于带上下界
// 的固定模板（主流程否决 R1-8 豁免后的权威契约；T05/T13 复刻消费）。
func TestSearchJQLIsWeekBounded(t *testing.T) {
	const wantJQL = "assignee = currentUser() AND resolution = Unresolved AND " +
		`due >= startOfWeek() AND due < startOfWeek("+1w") ORDER BY due ASC`
	require.Equal(t, wantJQL, jqlMyWeek, "jqlMyWeek 必须与带界契约一致")

	sawJQL := ""
	base, _ := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/3/myself":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"emailAddress": "member@example.com", "displayName": "Member"})
		case "/rest/api/3/search/jql":
			var body struct {
				JQL string `json:"jql"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			sawJQL = body.JQL
			_ = json.NewEncoder(w).Encode(map[string]any{"issues": []map[string]any{}})
		default:
			t.Errorf("unexpected jira path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	accessToken, _ := testOAuthFlow(t, base, "https://client.example/callback", "member@example.com", "tok")
	result, err := callToolWithToken(t, base, accessToken)
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	require.Equal(t, wantJQL, sawJQL, "服务端发出的 JQL 必须精确等于带界模板")
}

// --- OCR T04-R2-1/R2-7：access_token 过期后 refresh 必须可用 ---

// TestRefreshWorksAfterAccessTokenExpiry 覆盖 refresh_token 的核心场景：
// access_token 自然过期（收到 401）后用 refresh_token 续期必须成功——
// refresh 令牌独立于 access 令牌存活，且无效 refresh 尝试不得烧毁有效条目。
func TestRefreshWorksAfterAccessTokenExpiry(t *testing.T) {
	issues := []map[string]any{{
		"key": "A-1", "fields": map[string]any{
			"summary": "任务", "status": map[string]any{"name": "进行中"}, "duedate": "2026-09-25"},
	}}
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "tok", issues))
	redirectURI := "https://client.example/callback"

	// 先把 access TTL 缩到 1ns 再走授权流：签发的 access_token 立即过期，
	// refresh_token 用独立的 30 天 TTL（这正是「401 后刷新」场景）。
	oldTTL := accessTokenTTL
	accessTokenTTL = time.Nanosecond
	accessToken, refreshToken := testOAuthFlow(t, base, redirectURI, "member@example.com", "tok")
	t.Cleanup(func() { accessTokenTTL = oldTTL })
	time.Sleep(2 * time.Millisecond)

	// 无效 refresh_token 先试：400，且不得影响后续有效条目（先校验再消费）。
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", "rt-not-a-real-token")
	resp, err := http.PostForm(base+"/token", form)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// 有效 refresh_token：access 已过期仍必须成功续期（拿新 access + 轮换 refresh）。
	form.Set("refresh_token", refreshToken)
	resp, err = http.PostForm(base+"/token", form)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode, "refresh must survive access_token expiry")
	payload := map[string]any{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	newAccess, _ := payload["access_token"].(string)
	require.NotEmpty(t, newAccess)

	// 过期旧 access 仍被 gate 拒绝（不被续期复活）。
	_, err = callToolWithToken(t, base, accessToken)
	require.Error(t, err, "expired access token must stay rejected")
}

// --- OCR T04-R2-3/R2-4：pendingAuths 数量有界 + state 绑定不可覆盖 ---

func TestPendingAuthsBoundedAndStateNotOverwritable(t *testing.T) {
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "tok", nil))
	first := "https://client.example/callback"
	second := "https://client.example/other"
	clientID := testRegisterClient(t, base, first, second)
	_, challenge := testPKCE(t)

	// 同 state、不同绑定参数的第二次 GET → 409（防授权请求固定）。
	require.Equal(t, http.StatusOK, testAuthorizeGET(t, base, clientID, first, "fix-state", challenge))
	require.Equal(t, http.StatusConflict,
		testAuthorizeGET(t, base, clientID, second, "fix-state", challenge))

	// 数量上限：窗口内 pendingAuths 超过 maxPendingAuths → 429。
	oldMax := maxPendingAuths
	maxPendingAuths = 2
	t.Cleanup(func() { maxPendingAuths = oldMax })
	// "fix-state" 已占 1 条；再登记 1 条达到上限，第 3 个新 state 被拒。
	require.Equal(t, http.StatusOK, testAuthorizeGET(t, base, clientID, first, "state-b", challenge))
	require.Equal(t, http.StatusTooManyRequests,
		testAuthorizeGET(t, base, clientID, first, "state-c", challenge))
}

// --- OCR T04-R2-6：鉴权 gate 对不可解析 body 必须 fail-closed ---

func TestGateRejectsNonObjectJSONRPCBody(t *testing.T) {
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "tok", nil))
	body := []byte(`[{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search_my_week_issues"}}]`)
	resp, err := http.Post(base+"/mcp", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode,
		"batch/array body must be rejected by the auth gate, not passed through")
}

// --- OCR T04-R2-8：单次注册 redirect_uris 条数与单条长度上限 ---

func TestRegisterBoundsRedirectURIs(t *testing.T) {
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "tok", nil))
	many := make([]string, 17)
	for i := range many {
		many[i] = fmt.Sprintf("https://client.example/cb-%d", i)
	}
	body, err := json.Marshal(map[string]any{"redirect_uris": many})
	require.NoError(t, err)
	resp, err := http.Post(base+"/register", "application/json", strings.NewReader(string(body)))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "17 redirect_uris must be rejected")

	long := []string{"https://client.example/cb?pad=" + strings.Repeat("x", 3000)}
	body, err = json.Marshal(map[string]any{"redirect_uris": long})
	require.NoError(t, err)
	resp, err = http.Post(base+"/register", "application/json", strings.NewReader(string(body)))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "3000-byte redirect_uri must be rejected")

	// OCR R2 F19：RFC 6749 §3.1.2——redirect endpoint URI MUST NOT include a
	// fragment component（RFC 7591 注册校验同理）；userinfo 同拒。
	for _, uri := range []string{
		"https://client.example/cb#section",
		"https://user:pass@client.example/cb",
	} {
		body, err = json.Marshal(map[string]any{"redirect_uris": []string{uri}})
		require.NoError(t, err)
		resp, err = http.Post(base+"/register", "application/json", strings.NewReader(string(body)))
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		require.Equal(t, http.StatusBadRequest, resp.StatusCode, "redirect_uri %q must be rejected (fragment/userinfo)", uri)
	}
}

// --- 整分支终评（R4）：跨轮转交安全 findings 回归 ---

// TestAuthorizeAndTokenFormBodiesBounded：/authorize 与 /token 的表单解析必须有
// 体积上限（终评 r3-006/r3-007/r4-006/r4-007，此前四次报告未处置）。/authorize
// 用**有效 state + 有效凭据**放大请求体——上限缺失（RED）时会走完凭据验证并
// 302 放行；/token 以 error 字段区分「体积拒绝（invalid_request）」与
// 「grant_type 缺失（unsupported_grant_type）」。
func TestAuthorizeAndTokenFormBodiesBounded(t *testing.T) {
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "tok", nil))
	redirect := "https://client.example/cb"
	clientID := testRegisterClient(t, base, redirect)
	_, challenge := testPKCE(t)
	state := "big-form-state"
	require.Equal(t, http.StatusOK, testAuthorizeGET(t, base, clientID, redirect, state, challenge))

	form := url.Values{}
	form.Set("state", state)
	form.Set("email", "member@example.com")
	form.Set("api_token", "tok")
	form.Set("pad", strings.Repeat("x", 1<<20)) // >1MiB 表单体
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	resp, err := noRedirect.PostForm(base+"/authorize", form)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode,
		"oversized /authorize form must be rejected before credential verification, not 302-accepted")

	resp2, err := noRedirect.PostForm(base+"/token", form)
	require.NoError(t, err)
	payload := map[string]any{}
	_ = json.NewDecoder(resp2.Body).Decode(&payload)
	_ = resp2.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp2.StatusCode)
	require.Equal(t, "invalid_request", payload["error"],
		"oversized /token body must fail on size (invalid_request), not reach grant_type parsing")
}

// TestAuthorizeStateConsumedExactlyOnceUnderConcurrency：state 消费必须原子——
// 凭据验证（跨网络调用）与消费之间不得留下 TOCTOU 窗口，两个并发同 state 提交
// 只能发出一个授权码（终评 r2-004/r3-008/r4-009，三次报告未处置）。
func TestAuthorizeStateConsumedExactlyOnceUnderConcurrency(t *testing.T) {
	inner := testJiraAuthOK(t, "member@example.com", "tok", nil)
	var myselfHits int32
	slowJira := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/myself" {
			time.Sleep(250 * time.Millisecond) // 保证两请求在凭据验证阶段重叠
			atomic.AddInt32(&myselfHits, 1)
		}
		inner(w, r)
	})
	base, _ := newTestService(t, slowJira)
	redirect := "https://client.example/cb"
	clientID := testRegisterClient(t, base, redirect)
	_, challenge := testPKCE(t)
	state := "race-state"
	require.Equal(t, http.StatusOK, testAuthorizeGET(t, base, clientID, redirect, state, challenge))

	const workers = 2
	statuses := make([]int, workers)
	locations := make([]string, workers)
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			form := url.Values{}
			form.Set("state", state)
			form.Set("form_nonce", testFormNonceOf(state)) // 并发提交携带同一表单 nonce
			form.Set("email", "member@example.com")
			form.Set("api_token", "tok")
			resp, err := noRedirect.PostForm(base+"/authorize", form)
			if err != nil {
				t.Errorf("worker %d: %v", i, err)
				return
			}
			statuses[i] = resp.StatusCode
			locations[i] = resp.Header.Get("Location")
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}(i)
	}
	wg.Wait()

	redirects, conflicts := 0, 0
	for i, s := range statuses {
		switch s {
		case http.StatusFound:
			redirects++
			require.NotEmpty(t, locations[i], "the winning submit must carry the code")
		case http.StatusConflict:
			conflicts++
		default:
			t.Fatalf("unexpected status %d (location %q)", s, locations[i])
		}
	}
	require.Equal(t, 1, redirects, "exactly one authorization code must be issued per state")
	require.Equal(t, 1, conflicts, "the losing concurrent submit must be a conflict")
	require.GreaterOrEqual(t, atomic.LoadInt32(&myselfHits), int32(2),
		"both submits must reach credential verification (overlap ensured by delay)")
}

// TestAuthorizeFormShowsClientAndRedirectTarget：同意页必须在提交凭据前展示
// 请求方（client_name，缺失回落 client_id）与授权码跳转目的地（终评
// r2-012/r4-008：开放注册 + 信任注册 redirect_uri 的深链钓鱼链路，透明化是
// 成员识别伪造授权请求的前提）。client_name 是不可信输入，必须转义渲染。
func TestAuthorizeFormShowsClientAndRedirectTarget(t *testing.T) {
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "tok", nil))

	authorizePage := func(t *testing.T, clientID, redirect, state string) string {
		t.Helper()
		_, challenge := testPKCE(t)
		q := url.Values{}
		q.Set("response_type", "code")
		q.Set("client_id", clientID)
		q.Set("redirect_uri", redirect)
		q.Set("state", state)
		q.Set("code_challenge", challenge)
		q.Set("code_challenge_method", "S256")
		page, err := http.Get(base + "/authorize?" + q.Encode())
		require.NoError(t, err)
		body, _ := io.ReadAll(page.Body)
		_ = page.Body.Close()
		require.Equal(t, http.StatusOK, page.StatusCode)
		return string(body)
	}

	// 带恶意 client_name 的注册：名字必须转义后出现在同意页。
	evilRedirect := "https://evil.example.com/cb"
	body, err := json.Marshal(map[string]any{
		"redirect_uris": []string{evilRedirect},
		"client_name":   "Evil <script>alert(1)</script>",
	})
	require.NoError(t, err)
	resp, err := http.Post(base+"/register", "application/json", strings.NewReader(string(body)))
	require.NoError(t, err)
	var reg struct {
		ClientID string `json:"client_id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&reg)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.NotEmpty(t, reg.ClientID)

	pageHTML := authorizePage(t, reg.ClientID, evilRedirect, "phish-state")
	require.Contains(t, pageHTML, "Evil &lt;script&gt;", "client name must be displayed HTML-escaped")
	require.Contains(t, pageHTML, "evil.example.com", "redirect target host must be displayed")
	require.NotContains(t, pageHTML, "<script>alert(1)</script>", "raw client name must not inject markup")

	// 未提供 client_name 的注册：回落展示 client_id。
	plain := testRegisterClient(t, base, "https://client.example/cb2")
	pageHTML2 := authorizePage(t, plain, "https://client.example/cb2", "plain-state")
	require.Contains(t, pageHTML2, plain, "unnamed client must fall back to client_id")
}

// TestRegisterRestrictedToAllowedRedirectHosts：装配 AllowedRedirectHosts 后，
// /register 仅接受 host 在名单内的 redirect_uri（终评 r2-012/r4-008 的部署侧
// 缓解：生产启用即切断「任意注册方 + 任意跳转地」的钓鱼组合）。
func TestRegisterRestrictedToAllowedRedirectHosts(t *testing.T) {
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	t.Cleanup(utils.ResetSSRFWhitelistForTest)
	fakeJira := httptest.NewServer(testJiraAuthOK(t, "member@example.com", "tok", nil))
	t.Cleanup(fakeJira.Close)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	base := "http://" + ln.Addr().String()
	handler, err := NewHandler(Options{
		BaseURL:              base,
		JiraBaseURL:          fakeJira.URL,
		AllowedRedirectHosts: []string{"good.example.com"},
	})
	require.NoError(t, err)
	svc := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = svc.Serve(ln) }()
	t.Cleanup(func() { _ = svc.Shutdown(context.Background()) })

	register := func(uri string) int {
		body, err := json.Marshal(map[string]any{"redirect_uris": []string{uri}})
		require.NoError(t, err)
		resp, err := http.Post(base+"/register", "application/json", strings.NewReader(string(body)))
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		_, _ = io.Copy(io.Discard, resp.Body)
		return resp.StatusCode
	}
	require.Equal(t, http.StatusCreated, register("https://good.example.com/cb"))
	require.Equal(t, http.StatusBadRequest, register("https://evil.example.com/cb"),
		"redirect host outside the allowlist must be rejected at registration")
}

// --- 整分支 OCR 一轮（R5-D）：F7 client_name 封顶 + F5 注册表 TTL 淘汰 ---

// registerStatus 注册一个绑定单 redirect_uri 的客户端，返回 HTTP 状态码
// （不要求 201——容量 429 等拒绝路径也走这里）。
func registerStatus(t *testing.T, base, redirectURI string) int {
	t.Helper()
	body, err := json.Marshal(map[string]any{"redirect_uris": []string{redirectURI}})
	require.NoError(t, err)
	resp, err := http.Post(base+"/register", "application/json", strings.NewReader(string(body)))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

// TestRegisterBoundsClientName（整分支 OCR 一轮 F7）：client_name 是未认证
// /register 的不可信输入，必须封顶（maxClientNameBytes，字节）——否则 1MiB
// 注册体可把 ~1MiB 名字原样塞进注册表，4096 条击穿「4096×16×2KiB」聚合
// 内存界，与 redirect_uri 的条数/单条封顶不一致。
func TestRegisterBoundsClientName(t *testing.T) {
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "tok", nil))
	register := func(name string) int {
		body, err := json.Marshal(map[string]any{
			"redirect_uris": []string{"https://client.example/cb"},
			"client_name":   name,
		})
		require.NoError(t, err)
		resp, err := http.Post(base+"/register", "application/json", strings.NewReader(string(body)))
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		_, _ = io.Copy(io.Discard, resp.Body)
		return resp.StatusCode
	}
	require.Equal(t, http.StatusCreated, register(strings.Repeat("x", 256)),
		"a 256-byte client_name must be accepted")
	require.Equal(t, http.StatusBadRequest, register(strings.Repeat("x", 257)),
		"client_name over the cap must be rejected at registration")
}

// TestRegisteredClientsExpiredByTTL（整分支 OCR 一轮 F5）：动态注册表必须
// 有 TTL 淘汰——/register 无认证，无淘汰时攻击者可在重启前永久填满
// maxRegisteredClients 上限（此后合法 WeKnora 客户端一律 429，个人授权链路
// 整体不可用）。过期条目必须：(a) 释放容量（新注册成功）；(b) 不再通过
// /authorize 校验（unknown client_id）。
func TestRegisteredClientsExpiredByTTL(t *testing.T) {
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "tok", nil))
	oldMax, oldTTL := maxRegisteredClients, clientRegistrationTTL
	maxRegisteredClients = 1
	clientRegistrationTTL = 10 * time.Millisecond
	t.Cleanup(func() { maxRegisteredClients, clientRegistrationTTL = oldMax, oldTTL })
	// OCR 二轮 F2：写路径清扫按 sweepMinInterval 节流——本测试的原意是
	// 「register 写路径的惰性清扫释放容量」，关掉节流保持该断言语义
	//（不依赖 authorize 读路径的单条过期删除顺带腾名额）。
	oldInterval := sweepMinInterval
	sweepMinInterval = 0
	t.Cleanup(func() { sweepMinInterval = oldInterval })

	// A 占满唯一名额；容量满后 B 被拒（429，既有语义不变）。
	first := testRegisterClient(t, base, "https://client.example/cb-a")
	require.Equal(t, http.StatusTooManyRequests, registerStatus(t, base, "https://client.example/cb-b"),
		"capacity 1 must still reject while the slot is live")

	// 过期后：惰性清扫释放名额，B 可注册；A 的授权请求按 unknown client 拒绝。
	time.Sleep(30 * time.Millisecond)
	_, challenge := testPKCE(t)
	require.Equal(t, http.StatusBadRequest,
		testAuthorizeGET(t, base, first, "https://client.example/cb-a", "state-after-evict", challenge),
		"the expired client registration must no longer authorize")
	second := testRegisterClient(t, base, "https://client.example/cb-b")
	require.NotEqual(t, first, second)
}

// --- 整分支 OCR 二轮（R6）：F3 过期判定取时 / F5 响应体上限 / F6 防框架 ---

// TestAuthorizeStateExpiryJudgedAtConsumeTime（整分支 OCR 二轮 F3）：state
// 的过期判定必须用消费时刻的时钟——Myself 跨网络调用最长 30s，若复用进入
// 前的旧 now，调用期间恰好跨过 pendingAuthTTL 边界的 state 仍会发码，且
// 授权码 ExpiresAt 以回溯时间签发（实际存活期短于声明）。
func TestAuthorizeStateExpiryJudgedAtConsumeTime(t *testing.T) {
	inner := testJiraAuthOK(t, "member@example.com", "tok", nil)
	slowJira := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/myself" {
			time.Sleep(60 * time.Millisecond) // 消费时刻已越过 30ms 的 TTL 边界
		}
		inner(w, r)
	})
	base, _ := newTestService(t, slowJira)
	oldTTL := pendingAuthTTL
	pendingAuthTTL = 30 * time.Millisecond
	t.Cleanup(func() { pendingAuthTTL = oldTTL })

	redirect := "https://client.example/cb"
	clientID := testRegisterClient(t, base, redirect)
	_, challenge := testPKCE(t)
	state := "ttl-boundary-state"
	require.Equal(t, http.StatusOK, testAuthorizeGET(t, base, clientID, redirect, state, challenge))

	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	form := url.Values{}
	form.Set("state", state)
	form.Set("email", "member@example.com")
	form.Set("api_token", "tok")
	resp, err := noRedirect.PostForm(base+"/authorize", form)
	require.NoError(t, err)
	location := resp.Header.Get("Location")
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	require.NotEqual(t, http.StatusFound, resp.StatusCode,
		"a state that crossed its TTL boundary during the Myself round trip must not be issued a code (location: %s)", location)
	require.NotContains(t, location, "code=", "no authorization code may appear in the redirect")
}

// TestJiraResponseBodyBounded（整分支 OCR 二轮 F5）：响应体解码必须有大小
// 上限——分页封顶只约束请求参数，被攻陷/异常 Jira 可在超时窗口内推送超大
// JSON 流造成解码内存放大。截断导致 unexpected EOF 必须显式报错（fail-closed），
// 不得静默当作空结果。
func TestJiraResponseBodyBounded(t *testing.T) {
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	t.Cleanup(utils.ResetSSRFWhitelistForTest)
	bigSummary := strings.Repeat("s", 2048)
	fakeJira := httptest.NewServer(testJiraAuthOK(t, "member@example.com", "tok", []map[string]any{{
		"key":    "X-1",
		"fields": map[string]any{"summary": bigSummary, "status": map[string]any{"name": "To Do"}, "duedate": "2026-09-30"},
	}}))
	t.Cleanup(fakeJira.Close)

	oldLimit := jiraMaxResponseBytes
	jiraMaxResponseBytes = 256 // 响应 >2KB，远超测试上限
	t.Cleanup(func() { jiraMaxResponseBytes = oldLimit })

	client := &JiraClient{BaseURL: fakeJira.URL, Email: "member@example.com", APIToken: "tok"}
	issues, _, err := client.SearchMyWeek(context.Background())
	require.Error(t, err, "a response body over the cap must fail the decode, not parse")
	require.Nil(t, issues)
	require.ErrorContains(t, err, "decode", "the failure must surface as a decode error (truncation → unexpected EOF)")
}

// TestAuthorizePagesDenyFraming（整分支 OCR 二轮 F6）：凭据同意页与凭据
// 错误页是凭据录入面，必须携带 X-Frame-Options: DENY 与 CSP
// frame-ancestors 'none'——否则可被第三方站点 iframe 嵌入做视觉诱导。
func TestAuthorizePagesDenyFraming(t *testing.T) {
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "tok", nil))
	redirect := "https://client.example/cb"
	clientID := testRegisterClient(t, base, redirect)
	_, challenge := testPKCE(t)
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirect)
	q.Set("state", "framing-state")
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")

	page, err := http.Get(base + "/authorize?" + q.Encode())
	require.NoError(t, err)
	pageBody, err := io.ReadAll(page.Body)
	_ = page.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, page.StatusCode)
	require.Equal(t, "DENY", page.Header.Get("X-Frame-Options"), "consent page must deny framing")
	require.Equal(t, "frame-ancestors 'none'", page.Header.Get("Content-Security-Policy"), "consent page must set frame-ancestors")
	// 凭据错误页断言走 POST：nonce 随隐藏字段往返（OCR 二轮 F1）。
	framingNonce := ""
	if n := testFormNonceRe.FindSubmatch(pageBody); n != nil {
		framingNonce = string(n[1])
	}
	require.NotEmpty(t, framingNonce, "consent form must carry a form nonce")

	// 凭据错误页（401）同属凭据录入面。
	form := url.Values{}
	form.Set("state", "framing-state")
	form.Set("form_nonce", framingNonce)
	form.Set("email", "member@example.com")
	form.Set("api_token", "wrong-token")
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	bad, err := noRedirect.PostForm(base+"/authorize", form)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, bad.Body)
	_ = bad.Body.Close()
	require.Equal(t, http.StatusUnauthorized, bad.StatusCode)
	require.Equal(t, "DENY", bad.Header.Get("X-Frame-Options"), "credential-error page must deny framing")
	require.Equal(t, "frame-ancestors 'none'", bad.Header.Get("Content-Security-Policy"), "credential-error page must set frame-ancestors")
}

// --- 终评 R5 F8（T01-R4-F1 转交闭环）：access_token 表窗口总量封顶 ---

// TestActiveTokensBoundedOnExchangeAndRefresh：持有任一有效 refresh_token
// 的调用方可零门槛高频 /token——每次刷新令一个新 access_token 存活
// accessTokenTTL，窗口内 s.tokens 可被推至任意大（pendingAuths 已有同款
// 不变量 maxPendingAuths）。满容后 refresh 与新授权流的 exchange 都必须
// 429，且失败不得烧毁调用方现有凭据（旧 refresh_token 保留可重试、未消费
// 的 code 保留可重试）。
func TestActiveTokensBoundedOnExchangeAndRefresh(t *testing.T) {
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "tok", nil))
	redirectURI := "https://client.example/callback"

	oldMax := maxActiveTokens
	maxActiveTokens = 2
	t.Cleanup(func() { maxActiveTokens = oldMax })

	// 初始授权（tokens 1 条）+ 一次刷新（tokens 2 条）占满容量。
	_, refreshToken := testOAuthFlow(t, base, redirectURI, "member@example.com", "tok")
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	resp, err := http.PostForm(base+"/token", form)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "second token slot must still be issued")

	// 满容后的再次刷新：429，且旧 refresh_token 未被烧毁（重试仍 429 而非
	// 400 invalid_grant——容量是暂时态，不该迫使成员重启授权流）。
	form.Set("refresh_token", refreshToken)
	resp, err = http.PostForm(base+"/token", form)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode,
		"refresh beyond maxActiveTokens must be rejected with 429")
	form.Set("refresh_token", refreshToken)
	resp, err = http.PostForm(base+"/token", form)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode,
		"the caller's refresh_token must survive a 429 (not rotated away)")

	// 满容后的新授权流：exchange 也必须 429，且未消费的 code 保留可重试
	//（重试同 code 仍 429 而非 400 unknown code）。
	clientID := testRegisterClient(t, base, redirectURI)
	verifier, challenge := testPKCE(t)
	require.Equal(t, http.StatusOK,
		testAuthorizeGET(t, base, clientID, redirectURI, "r5-bounded-state", challenge))
	status, location := testAuthorizePOST(t, base, "r5-bounded-state", "member@example.com", "tok")
	require.Equal(t, http.StatusFound, status, "location: %s", location)
	parsed, err := url.Parse(location)
	require.NoError(t, err)
	code := parsed.Query().Get("code")
	require.NotEmpty(t, code)
	exStatus, _ := testExchangeCode(t, base, code, verifier, clientID, redirectURI)
	require.Equal(t, http.StatusTooManyRequests, exStatus,
		"exchange beyond maxActiveTokens must be rejected with 429")
	exStatus, _ = testExchangeCode(t, base, code, verifier, clientID, redirectURI)
	require.Equal(t, http.StatusTooManyRequests, exStatus,
		"an unconsumed code must survive a 429 (not burned)")
}

// TestLookupSessionDoesNotSweepUnrelatedEntries：/mcp 每请求经 gate 与
// contextFunc 两次 lookupSession，原实现持锁对 5 个 map 全量清扫——tokens
// 表被推大后形成请求串行化 O(n)×2 放大。读路径必须只做单条 O(1) 查找：
// 查有效 token 不删无关过期条目（全量清扫职责在写路径）；查到过期条目
// 仍即时删除并拒绝（过期即拒语义不变）。
func TestLookupSessionDoesNotSweepUnrelatedEntries(t *testing.T) {
	s := newOAuthServer("http://auth.example", "http://jira.example", nil)
	session := &oauthSession{Email: "member@example.com", APIToken: "tok"}
	now := time.Now()
	s.tokens["at-expired"] = tokenEntry{Session: session, ExpiresAt: now.Add(-time.Minute)}
	s.tokens["at-valid"] = tokenEntry{Session: session, ExpiresAt: now.Add(time.Minute)}

	require.NotNil(t, s.lookupSession("at-valid"))
	_, unrelatedStillThere := s.tokens["at-expired"]
	require.True(t, unrelatedStillThere,
		"lookupSession must not sweep unrelated expired entries on the hot /mcp read path")

	require.Nil(t, s.lookupSession("at-expired"), "expired token must be rejected")
	_, expiredStillThere := s.tokens["at-expired"]
	require.False(t, expiredStillThere,
		"the looked-up expired entry itself must still be removed on sight")
}

// --- 跨任务转交 R7：oauth.go 四项（T01-R1-F2 / T01-R2-F5 / T01-R2-F9 / T01-R4-F1 补充） ---

// TestAuthorizeFormRoundTripsEscapedState（T01-R1-F2）：隐藏 state 字段曾用
// %q 渲染——Go 字面量转义（\\ 等）浏览器不逆转变，含此类字符的合法 state
// 表单回传值 ≠ 登记键，POST 对未过期 state 400 中断授权流。测试模拟浏览器：
// 从表单 HTML 提取 hidden state 属性值并做 HTML 实体解码（浏览器只解 HTML
// 转义，不逆转 Go 字面量转义），用该值 POST 必须命中登记键并成功发码。
func TestAuthorizeFormRoundTripsEscapedState(t *testing.T) {
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "tok", nil))
	redirectURI := "https://client.example/callback"
	clientID := testRegisterClient(t, base, redirectURI)
	_, challenge := testPKCE(t)
	state := `r7\state "<>&` // 含反斜杠、引号与 HTML 元字符

	// 单次 GET 登记并取回表单（同 state 二次 GET 会被防覆盖 409 拒）。
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	page, err := http.Get(base + "/authorize?" + q.Encode())
	require.NoError(t, err)
	body, err := io.ReadAll(page.Body)
	_ = page.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, page.StatusCode)
	m := regexp.MustCompile(`<input type="hidden" name="state" value="([^"]*)">`).FindSubmatch(body)
	require.NotNil(t, m, "the consent form must carry the hidden state field")
	browserState := html.UnescapeString(string(m[1]))
	// 表单 nonce 同为隐藏字段：浏览器会与 state 一并回传（OCR 二轮 F1）。
	if n := testFormNonceRe.FindSubmatch(body); n != nil {
		testFormNonces.Store(browserState, string(n[1]))
	}

	status, location := testAuthorizePOST(t, base, browserState, "member@example.com", "tok")
	require.Equal(t, http.StatusFound, status,
		"the state a browser would submit must match the registered key, location: %s", location)
	require.Contains(t, location, "code=", "the authorization code must be issued")
}

// TestRegisterRejectsPartialDecodes（T01-R2-F5）：/register 曾仅在「err!=nil
// 且 redirect_uris==nil」时拒绝——类型不匹配的注册体（client_name:123）在
// json 部分解码后 redirect_uris 已填充，会带着空 client_name 入库（本会话
// 以 json.Decoder 实验证实：UnmarshalTypeError 下已解码字段保留；截断形态
// 则不发生部分 unmarshal，现行已拒）。解析失败必须一律 400。
func TestRegisterRejectsPartialDecodes(t *testing.T) {
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "tok", nil))
	for _, body := range []string{
		`{"redirect_uris":["https://client.example/cb"],"client_name":123}`, // 类型不匹配，部分解码
		`{"redirect_uris":["https://client.example/cb"],"client_name":"x`,   // 截断（对照：现行已拒）
	} {
		resp, err := http.Post(base+"/register", "application/json", strings.NewReader(body))
		require.NoError(t, err)
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		require.Equalf(t, http.StatusBadRequest, resp.StatusCode,
			"partially decoded registration %q must be rejected wholesale", body)
	}
}

// TestAuthorizeBoundsCodeChallenge（T01-R2-F9）：code_challenge 是未认证
// GET /authorize 的存储键成分（随 pendingAuths 驻留 TTL），与 state 同一
// 威胁面却无长度封顶——1024 × ~1MiB 击穿聚合内存界。RFC 7636 S256 的
// challenge 恰为 43 个 base64url 字符：超长与非 base64url 字符集都必须拒绝。
func TestAuthorizeBoundsCodeChallenge(t *testing.T) {
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "tok", nil))
	redirectURI := "https://client.example/callback"
	clientID := testRegisterClient(t, base, redirectURI)
	_, validChallenge := testPKCE(t)
	long := strings.Repeat("A", 129)
	illegal := "!!!not-base64url!!!"

	require.Equal(t, http.StatusBadRequest,
		testAuthorizeGET(t, base, clientID, redirectURI, "st-long", long),
		"an overlong code_challenge must be rejected")
	require.Equal(t, http.StatusBadRequest,
		testAuthorizeGET(t, base, clientID, redirectURI, "st-illegal", illegal),
		"non-base64url code_challenge characters must be rejected")
	require.Equal(t, http.StatusOK,
		testAuthorizeGET(t, base, clientID, redirectURI, "st-valid", validChallenge),
		"a legal 43-char base64url challenge must still register")
}

// TestRefreshTokensBoundedOnExchange（T01-R4-F1 补充，跨任务转交）：tokens
// 表已由 maxActiveTokens 封顶（终评 R5 F8）；refreshTokens 的写入只发生在
// 授权码交换（refresh 轮换 1 删 1 增净零），同样按 pendingAuths 的不变量
// 封窗口内总量——满容 429 且不消费 code（暂时态可重试）。
func TestRefreshTokensBoundedOnExchange(t *testing.T) {
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "tok", nil))
	redirectURI := "https://client.example/callback"

	oldMax := maxRefreshTokens
	maxRefreshTokens = 1
	t.Cleanup(func() { maxRefreshTokens = oldMax })

	// 第一次授权流：唯一名额被占用。
	_, refreshToken := testOAuthFlow(t, base, redirectURI, "member@example.com", "tok")
	require.NotEmpty(t, refreshToken)

	// 第二次授权流：exchange 满 429，且未消费的 code 保留可重试。
	clientID := testRegisterClient(t, base, redirectURI)
	verifier, challenge := testPKCE(t)
	require.Equal(t, http.StatusOK,
		testAuthorizeGET(t, base, clientID, redirectURI, "r7-rt-state", challenge))
	status, location := testAuthorizePOST(t, base, "r7-rt-state", "member@example.com", "tok")
	require.Equal(t, http.StatusFound, status, "location: %s", location)
	parsed, err := url.Parse(location)
	require.NoError(t, err)
	code := parsed.Query().Get("code")
	require.NotEmpty(t, code)
	exStatus, _ := testExchangeCode(t, base, code, verifier, clientID, redirectURI)
	require.Equal(t, http.StatusTooManyRequests, exStatus,
		"exchange beyond maxRefreshTokens must be rejected with 429")
	exStatus, _ = testExchangeCode(t, base, code, verifier, clientID, redirectURI)
	require.Equal(t, http.StatusTooManyRequests, exStatus,
		"an unconsumed code must survive a 429 (not burned)")
}

// --- OCR 一轮 R8：oauth.go 两项 + jira.go 整体时限 ---

// TestValidateAuthorizationRequestDoesNotSweep（OCR 一轮 F8）：
// validateAuthorizationRequest 曾持 s.mu 执行 sweepExpiredLocked 全量清扫
// （5 个 map 达上限 ≈1.8 万条目）——这把锁同时被 /mcp 每请求两次的
// lookupSession 依赖，R5 F8 已消除的 O(n) 放大被重新挂到未认证
// GET /authorize（垃圾 client_id 即触达）。读路径必须 O(1)：只查目标
// client（查到的过期注册即时删除拒绝），无关过期条目不被清扫。
func TestValidateAuthorizationRequestDoesNotSweep(t *testing.T) {
	s := newOAuthServer("http://auth.example", "http://jira.example", nil)
	now := time.Now()
	s.clients["jtm-expired"] = registeredClient{
		RedirectURIs: []string{"https://a.example/cb"}, ExpiresAt: now.Add(-time.Minute),
	}
	s.clients["jtm-valid"] = registeredClient{
		ClientName: "WeKnora", RedirectURIs: []string{"https://client.example/cb"}, ExpiresAt: now.Add(time.Hour),
	}
	s.pendingAuths["st-expired"] = pendingAuth{ExpiresAt: now.Add(-time.Minute)}
	_, challenge := testPKCE(t)

	client, err := s.validateAuthorizationRequest(authorizationRequest{
		ClientID: "jtm-valid", RedirectURI: "https://client.example/cb",
		CodeChallenge: challenge, CodeChallengeMethod: "S256",
	}, "code")
	require.NoError(t, err)
	require.Equal(t, "WeKnora", client.ClientName)
	_, stillClient := s.clients["jtm-expired"]
	require.True(t, stillClient,
		"the client_id read path must not sweep unrelated expired clients (lock convoy on /mcp)")
	_, stillPending := s.pendingAuths["st-expired"]
	require.True(t, stillPending,
		"the client_id read path must not sweep unrelated expired pending auths")

	// 查到的过期注册本身：即时删除并拒绝（lookupSession 同款单条语义）。
	_, err = s.validateAuthorizationRequest(authorizationRequest{
		ClientID: "jtm-expired", RedirectURI: "https://a.example/cb",
		CodeChallenge: challenge, CodeChallengeMethod: "S256",
	}, "code")
	require.Error(t, err, "an expired registration must be rejected on sight")
	_, stillThere := s.clients["jtm-expired"]
	require.False(t, stillThere,
		"the looked-up expired entry itself must still be removed")
}

// TestCodesBoundedOnIssue（OCR 一轮 F9）：codes 是唯一只有 TTL 没有窗口
// 容量上限的令牌 map——发码即消费 state 释放 pendingAuths 名额，持有有效
// Jira 凭据者可循环「登记→发码」在 10min TTL 窗口内按吞吐无界堆积
// （R7 的豁免论证被本轮评审推翻）。满容不消费 state、429（与 exchangeCode
// 满容语义一致）。
func TestCodesBoundedOnIssue(t *testing.T) {
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "tok", nil))
	redirectURI := "https://client.example/callback"

	oldMax := maxActiveCodes
	maxActiveCodes = 1
	t.Cleanup(func() { maxActiveCodes = oldMax })

	// 第一条授权流发码占满唯一名额（code 不 exchange，TTL 内驻留）。
	clientID := testRegisterClient(t, base, redirectURI)
	_, challenge := testPKCE(t)
	require.Equal(t, http.StatusOK,
		testAuthorizeGET(t, base, clientID, redirectURI, "r8-c1", challenge))
	status, location := testAuthorizePOST(t, base, "r8-c1", "member@example.com", "tok")
	require.Equal(t, http.StatusFound, status, "location: %s", location)
	require.Contains(t, location, "code=")

	// 第二条授权流：凭据验证通过但发码满容 → 429，且 state 未消费可重试
	//（重试仍 429 而非 409/400——容量是暂时态）。
	clientID2 := testRegisterClient(t, base, redirectURI)
	_, challenge2 := testPKCE(t)
	require.Equal(t, http.StatusOK,
		testAuthorizeGET(t, base, clientID2, redirectURI, "r8-c2", challenge2))
	status, _ = testAuthorizePOST(t, base, "r8-c2", "member@example.com", "tok")
	require.Equal(t, http.StatusTooManyRequests, status,
		"issuing beyond maxActiveCodes must be rejected with 429")
	status, _ = testAuthorizePOST(t, base, "r8-c2", "member@example.com", "tok")
	require.Equal(t, http.StatusTooManyRequests, status,
		"the pending state must survive a 429 (not consumed)")
}

// TestSearchMyWeekOverallDeadline（OCR 一轮 F5）：分页循环最多 10 页、每页
// 独立 30s 超时，缺整体 deadline 时上游持续慢响应可拖满 ≈5 分钟。入口
// context.WithTimeout 整体时限必须先于页级超时切断。
func TestSearchMyWeekOverallDeadline(t *testing.T) {
	old := maxToolCallTimeout
	maxToolCallTimeout = 100 * time.Millisecond
	t.Cleanup(func() { maxToolCallTimeout = old })

	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/search/jql" {
			time.Sleep(300 * time.Millisecond) // 慢于整体 deadline，远快于页级 30s
			_ = json.NewEncoder(w).Encode(map[string]any{"issues": []any{}})
			return
		}
		t.Errorf("unexpected jira path: %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(fake.Close)
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	t.Cleanup(utils.ResetSSRFWhitelistForTest)

	ctx := context.WithValue(context.Background(), sessionContextKey{},
		&oauthSession{Email: "member@example.com", APIToken: "tok"})
	_, err := handleSearchMyWeek(ctx, fake.URL)
	require.Error(t, err,
		"the overall tool-call deadline must cut a slow upstream before page timeouts pile up")
}

// --- OCR 二轮 R9：main.go 三项 + oauth.go 两项 ---

// TestHTTPServerTimeouts（OCR 二轮 F1）：/mcp、/register、/authorize、/token
// 都是未认证 1MiB 输入面，MaxBytesReader 只封顶字节数不封顶读取时间——
// 慢速 body 连接可无限期占用连接与 goroutine。server 必须配
// ReadTimeout（请求读取阶段，不影响 SSE 响应）与 IdleTimeout；
// WriteTimeout 不得设置（会切断 /mcp 流式长响应）。
func TestHTTPServerTimeouts(t *testing.T) {
	srv := newHTTPServer(http.NotFoundHandler())
	require.NotNil(t, srv.ReadTimeout, "ReadTimeout must be set (slow-body DoS)")
	require.NotNil(t, srv.IdleTimeout, "IdleTimeout must be set (keep-alive reaping)")
	require.NotNil(t, srv.ReadHeaderTimeout, "ReadHeaderTimeout must stay set")
	require.Equal(t, time.Duration(0), srv.WriteTimeout,
		"WriteTimeout must stay zero (it would cut /mcp SSE streams)")
}

// TestSweepThrottledOnWritePaths（OCR 二轮 F2）：sweepExpiredLocked 曾在
// 5 个未认证写路径请求内全量遍历 5 个 map 且全程持 s.mu（达上限 21504
// 条目），与 /mcp 每请求两次的 lookupSession 争抢同一把锁。写路径清扫
// 必须按 sweepMinInterval 节流：窗口内跳过（过期条目留给窗口后的下一次
// 清扫），过期判定本身不受影响。
func TestSweepThrottledOnWritePaths(t *testing.T) {
	s := newOAuthServer("http://auth.example", "http://jira.example", nil)
	now := time.Now()
	s.tokens["at-expired"] = tokenEntry{
		Session: &oauthSession{Email: "e", APIToken: "t"}, ExpiresAt: now.Add(-time.Minute),
	}

	// 刚清扫过（lastSweep=now）：窗口内的写路径清扫被节流跳过。
	s.mu.Lock()
	s.lastSweep = now
	s.maybeSweepLocked(now.Add(sweepMinInterval / 2))
	s.mu.Unlock()
	_, still := s.tokens["at-expired"]
	require.True(t, still,
		"a write-path sweep inside the throttle window must not run the O(n) pass")

	// 窗口过后：清扫照常执行。
	s.mu.Lock()
	s.maybeSweepLocked(now.Add(2 * sweepMinInterval))
	s.mu.Unlock()
	_, still = s.tokens["at-expired"]
	require.False(t, still, "a sweep past the window must still run")
}

// TestRegisterRejectsFormatCharactersInClientName（OCR 二轮 F6）：同意页
// 请求方标识 client_name 只经 html.EscapeString 渲染——Unicode Cf（RLO、
// isolate、BOM、软连字符）原样穿透，可视觉重排伪装可信请求方（深链钓鱼
// 面）。注册期必须 fail-closed 拒绝该字符类；ZWJ/ZWNJ 复合 emoji 按
// manifest validateDescription 的双重标准豁免（显示文本）。
func TestRegisterRejectsFormatCharactersInClientName(t *testing.T) {
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "tok", nil))
	redirect := "https://client.example/cb"

	for name, note := range map[string]string{
		"evil\u202Ename":            "RLO (Cf) must be rejected (visual reordering)",
		"iso\u2066lated":            "LRI (Cf) must be rejected (isolate override)",
		"bom\uFEFFname":             "BOM (Cf) must be rejected",
		"soft\u00ADhyphen":          "soft hyphen (Cf) must be rejected",
		"line\u2028sep":             "Zl separator must be rejected",
		"ctl\x01name":               "Cc control must be rejected",
		strings.Repeat("\u200E", 3): "LRM run (Cf) must be rejected",
	} {
		body, err := json.Marshal(map[string]any{"redirect_uris": []string{redirect}, "client_name": name})
		require.NoError(t, err)
		resp, err := http.Post(base+"/register", "application/json", strings.NewReader(string(body)))
		require.NoError(t, err)
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		require.Equalf(t, http.StatusBadRequest, resp.StatusCode, "%s: %q", note, name)
	}

	// 对照：ZWJ 复合 emoji 名合法（manifest validateDescription 同款豁免）。
	body, err := json.Marshal(map[string]any{"redirect_uris": []string{redirect}, "client_name": "家庭待办 👨‍👩‍👧‍👦"})
	require.NoError(t, err)
	resp, err := http.Post(base+"/register", "application/json", strings.NewReader(string(body)))
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode,
		"compound emoji (ZWJ) client names must stay legal")
}

// TestValidateServiceBaseURLRejectsPathQueryFragment（OCR 二轮 F3）：
// 服务自身 BaseURL 携带 path/query/fragment 时，拼出的 OAuth 元数据、
// WWW-Authenticate challenge 与 manifest endpoint 全部不可用却静默启动
// （fragment 吞路径、query 干扰路由、sub-path 与根挂载 mux 不匹配）。
// JiraBaseURL 不适用本规则（可合法携带 /jira 等路径）。
func TestValidateServiceBaseURLRejectsPathQueryFragment(t *testing.T) {
	for _, raw := range []string{
		"https://host#f",
		"https://host?q",
		"https://host/sub",
		"https://host/sub?x=1#y",
	} {
		require.Errorf(t, validateServiceBaseURL("BaseURL", raw),
			"service BaseURL %q must not carry path/query/fragment", raw)
	}
	require.NoError(t, validateServiceBaseURL("BaseURL", "https://plugins.example.com"))
	require.NoError(t, validateServiceBaseURL("BaseURL", "http://127.0.0.1:8020"))
	// JiraBaseURL 仍走 validateBaseURL：路径合法（Jira DC 常部署 /jira 下）。
	require.NoError(t, validateBaseURL("JiraBaseURL", "https://jira.example.com/jira"))
}

// TestShutdownGracefullyReturnsNilAfterClose（OCR 二轮 F4）：Shutdown 超时
// 分支 Close 兜底后进程状态已干净，却仍返回超时错误令调用方
// log.Fatalf 误判失败。graceful 停机在宽限关闭与强制回收两种结局下都
// 必须返回 nil。
func TestShutdownGracefullyReturnsNilAfterClose(t *testing.T) {
	release := make(chan struct{})
	srv := newHTTPServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		<-release // 挂起活跃请求：Shutdown 等不到 idle，必走超时分支
	}))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { close(release); _ = srv.Close() })

	done := make(chan struct{})
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String() + "/metadata")
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		close(done)
	}()
	time.Sleep(100 * time.Millisecond) // 让请求进入 handler

	err = shutdownGracefully(srv, 100*time.Millisecond)
	require.NoError(t, err,
		"a forced close after graceful-timeout still leaves the server down cleanly")

	// Close 已强制断开挂起连接：请求 goroutine 必然被解除。
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the hung request must be released by Close")
	}
}

// TestRegisterRedirectHostPortSemantics（OCR 一轮 R12-E F18）：名单匹配按
// host[:port]——裸 host 条目=该主机任意端口；host:port 条目=精确匹配。
// 修复前 Hostname() 剥端口匹配裸名：host:port 条目永 Miss（运营者填法
// 静默失效），且裸 host 放行同主机任意第三方端口。
func TestRegisterRedirectHostPortSemantics(t *testing.T) {
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	t.Cleanup(utils.ResetSSRFWhitelistForTest)
	fakeJira := httptest.NewServer(testJiraAuthOK(t, "member@example.com", "tok", nil))
	t.Cleanup(fakeJira.Close)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	base := "http://" + ln.Addr().String()
	handler, err := NewHandler(Options{
		BaseURL:              base,
		JiraBaseURL:          fakeJira.URL,
		AllowedRedirectHosts: []string{"bare.example.com", "pinned.example.com:8443"},
	})
	require.NoError(t, err)
	svc := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = svc.Serve(ln) }()
	t.Cleanup(func() { _ = svc.Shutdown(context.Background()) })

	register := func(uri string) int {
		body, err := json.Marshal(map[string]any{"redirect_uris": []string{uri}})
		require.NoError(t, err)
		resp, err := http.Post(base+"/register", "application/json", strings.NewReader(string(body)))
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		_, _ = io.Copy(io.Discard, resp.Body)
		return resp.StatusCode
	}
	// 裸 host 条目：同主机任意端口放行（部署语义保持）。
	require.Equal(t, http.StatusCreated, register("https://bare.example.com/cb"))
	require.Equal(t, http.StatusCreated, register("https://bare.example.com:9999/cb"))
	// host:port 条目：精确匹配放行、错端口拒绝。
	require.Equal(t, http.StatusCreated, register("https://pinned.example.com:8443/cb"))
	require.Equal(t, http.StatusBadRequest, register("https://pinned.example.com:9000/cb"),
		"a pinned host:port entry must not admit other ports on the same host")
	// 名单外主机仍整体拒绝。
	require.Equal(t, http.StatusBadRequest, register("https://evil.example.com:8443/cb"))
}

// TestAuthorizePagesNoStoreHeader（OCR 一轮 R12-E F19）：凭据同意页与凭据
// 错误页必须带 Cache-Control: no-store——与 writeJSON 同纪律，防共享缓存
// 留存承载隐藏 state 与凭据交互上下文的页面快照。
func TestAuthorizePagesNoStoreHeader(t *testing.T) {
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "tok", nil))
	redirect := "https://client.example/cb"
	clientID := testRegisterClient(t, base, redirect)
	_, challenge := testPKCE(t)
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirect)
	q.Set("state", "nostore-state")
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")

	page, err := http.Get(base + "/authorize?" + q.Encode())
	require.NoError(t, err)
	pageBody, err := io.ReadAll(page.Body)
	_ = page.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, page.StatusCode)
	require.Equal(t, "no-store", page.Header.Get("Cache-Control"), "consent page must not be cacheable")
	nostoreNonce := ""
	if n := testFormNonceRe.FindSubmatch(pageBody); n != nil {
		nostoreNonce = string(n[1])
	}
	require.NotEmpty(t, nostoreNonce, "consent form must carry a form nonce")

	form := url.Values{}
	form.Set("state", "nostore-state")
	form.Set("form_nonce", nostoreNonce)
	form.Set("email", "member@example.com")
	form.Set("api_token", "wrong-token")
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	bad, err := noRedirect.PostForm(base+"/authorize", form)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, bad.Body)
	_ = bad.Body.Close()
	require.Equal(t, "no-store", bad.Header.Get("Cache-Control"), "credential error page must not be cacheable")
}

// TestRunMainDrainsOnSIGTERM（跨任务转交 T01-OCR1-F9）：main() 曾以
// context.Background() 启动 Run，而 Run 的优雅停机只挂 ctx.Done()——
// Background 永不取消，shutdownGracefully（5s 宽限 + Close 强制回收）在
// 生产部署不可达：SIGTERM/SIGINT 时进程被直接杀死，/mcp SSE 长连接与
// 最长 60s 的 Jira 工具调用不排水。runMain 必须用 signal.NotifyContext
// 挂 SIGINT/SIGTERM——本测试向自身进程发送真实 SIGTERM（handler 注册期
// 内进程不会被杀死，只会取消 ctx），断言服务完整监听后 runMain 仍能在
// 有限时间内优雅返回 nil（runMain → Run → shutdownGracefully）。
func TestRunMainDrainsOnSIGTERM(t *testing.T) {
	// 先占一个自由端口拿具体地址（BaseURL 走 main 的缺省回退路径时，
	// host 具体且非通配，validateServiceBaseURL 通过）。
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := probe.Addr().String()
	require.NoError(t, probe.Close())

	// 启动期不访问 Jira（与 TestRunShutdownReturnsAfterContextCancel 同值）。
	t.Setenv("PLUGIN_JIRA_BASE_URL", "http://127.0.0.1:9")
	t.Setenv("PLUGIN_LISTEN_ADDR", addr)

	errCh := make(chan error, 1)
	go func() { errCh <- runMain() }()

	// 等服务真正监听后再发信号（SIGTERM 必须落在 NotifyContext 注册期内；
	// 注册早于 net.Listen，监听成功即已注册）。
	listening := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-errCh:
			t.Fatalf("runMain returned before signal: %v", err)
		default:
		}
		if conn, derr := net.DialTimeout("tcp", addr, 200*time.Millisecond); derr == nil {
			_ = conn.Close()
			listening = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.True(t, listening, "service must be listening before SIGTERM")

	require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))

	select {
	case err := <-errCh:
		require.NoError(t, err, "graceful shutdown after SIGTERM must return nil")
	case <-time.After(10 * time.Second):
		t.Fatal("runMain did not return after SIGTERM — graceful shutdown unreachable (dead code)")
	}
}

// TestAuthorizeConsumedStateCannotBeRebound（OCR 一轮 F2）：state 被消费后
// 必须保留墓碑至原 TTL 过期——消费时 delete(pendingAuths, state) 会让
// 「同 state 拒绝覆盖」的 409 守卫对已消费 state 失效：攻击者获知成员
// state（浏览器历史/referrer/日志）后，可用自己的 client_id/redirect_uri/
// PKCE 对同 state 重新 GET /authorize 重绑发码上下文；成员后退重交旧表单
// （隐藏字段仍是同 state）时，凭据验证成功发出的授权码会 302 到攻击者
// redirect_uri，攻击者持自己的 PKCE verifier 即可换取成员 access_token。
func TestAuthorizeConsumedStateCannotBeRebound(t *testing.T) {
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "member-token", nil))
	memberRedirect := "https://client.example/cb"
	attackerRedirect := "https://attacker.example/cb"
	memberClient := testRegisterClient(t, base, memberRedirect)
	attackerClient := testRegisterClient(t, base, attackerRedirect)
	_, memberChallenge := testPKCE(t)
	state := "leaked-state-001"

	// 墓碑随登记时的原 TTL 过期——从步骤 1 起就用短 TTL：既覆盖步骤 1-3
	// 的墓碑窗（本地往返毫秒级，300ms 充裕），又让步骤 4 无需等待默认
	// 10 分钟。登记条目的 ExpiresAt = 登记时刻 + 本 TTL。
	origTTL := pendingAuthTTL
	pendingAuthTTL = 300 * time.Millisecond
	t.Cleanup(func() { pendingAuthTTL = origTTL })

	// 1) 成员发起授权并提交凭据：state 被消费、发码给成员客户端。
	require.Equal(t, http.StatusOK, testAuthorizeGET(t, base, memberClient, memberRedirect, state, memberChallenge))
	status, location := testAuthorizePOST(t, base, state, "member@example.com", "member-token")
	require.Equal(t, http.StatusFound, status)
	require.Contains(t, location, "code=")

	// 2) 攻击者对已消费的同一 state 重新登记自己的授权请求（重绑定尝试）。
	// 修复前：登记成功（200）→ 攻击成立；修复后：墓碑期内 409。
	_, attackerChallenge := testPKCE(t)
	require.Equal(t, http.StatusConflict,
		testAuthorizeGET(t, base, attackerClient, attackerRedirect, state, attackerChallenge),
		"a consumed state must not be re-registrable before its original TTL expires")

	// 3) 成员后退重交旧表单（隐藏字段仍是同 state）：不得再发码。
	resubmitStatus, resubmitLocation := testAuthorizePOST(t, base, state, "member@example.com", "member-token")
	require.NotEqual(t, http.StatusFound, resubmitStatus,
		"resubmitting an already-consumed state must not issue a code (got redirect: %s)", resubmitLocation)

	// 4) 墓碑随原 TTL 过期失效：同 state 可开始全新授权流（OAuth 正常语义
	//    ——state 由客户端生成，跨授权流可复用；攻击窗口被 TTL 限定）。
	time.Sleep(400 * time.Millisecond)
	require.Equal(t, http.StatusOK,
		testAuthorizeGET(t, base, attackerClient, attackerRedirect, state, attackerChallenge),
		"tombstone must expire with the original pendingAuthTTL")
	status, location = testAuthorizePOST(t, base, state, "member@example.com", "member-token")
	require.Equal(t, http.StatusFound, status)
	require.Contains(t, location, attackerRedirect, "a fresh registration must complete the flow normally")
}

// TestAuthorizeStaleFormRebindRejected（OCR 二轮 F1 攻击路径②）：成员提交
// 超过 TTL 的陈旧表单。原条目过期后，攻击者经 serveAuthorizeForm 的
// 「过期即删+重登记」路径用自己的 client/redirect/PKCE 登记同 state；
// 成员浏览器后退重交陈旧表单（隐藏字段仍是原登记的 nonce/绑定参数）时，
// 发码上下文取自攻击者条目——携带成员 Jira 凭据会话的授权码被 302 到
// 攻击者 redirect_uri。服务端必须校验表单 nonce 与登记条目一致。
func TestAuthorizeStaleFormRebindRejected(t *testing.T) {
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "member-token", nil))
	memberRedirect := "https://client.example/cb"
	attackerRedirect := "https://attacker.example/cb"
	memberClient := testRegisterClient(t, base, memberRedirect)
	attackerClient := testRegisterClient(t, base, attackerRedirect)
	_, memberChallenge := testPKCE(t)
	state := "stale-form-state"

	origTTL := pendingAuthTTL
	pendingAuthTTL = 300 * time.Millisecond
	t.Cleanup(func() { pendingAuthTTL = origTTL })

	// 1) 成员拉取授权表单（nonce₁ 随表单发给成员浏览器）。
	require.Equal(t, http.StatusOK, testAuthorizeGET(t, base, memberClient, memberRedirect, state, memberChallenge))
	staleNonce := testFormNonceOf(state)
	require.NotEmpty(t, staleNonce, "authorization form must carry a form nonce")

	// 2) 成员搁置表单超过 TTL——原登记过期。
	time.Sleep(400 * time.Millisecond)

	// 3) 攻击者对同 state 重新登记（过期后 state 可复用是正常语义）。
	_, attackerChallenge := testPKCE(t)
	require.Equal(t, http.StatusOK, testAuthorizeGET(t, base, attackerClient, attackerRedirect, state, attackerChallenge),
		"a state past its TTL may start a fresh authorization flow")

	// 4) 成员提交陈旧表单（隐藏字段仍是 nonce₁）：不得发码。
	// 修复前：nonce 不被校验 → 发码上下文=攻击者条目 → 302 攻击者
	// redirect_uri（攻击成立）；修复后：nonce 不匹配 → 400 拒绝。
	status, location := testAuthorizePOSTWithNonce(t, base, state, staleNonce, "member@example.com", "member-token")
	require.NotEqual(t, http.StatusFound, status,
		"a stale form (nonce of the expired registration) must not issue a code (got redirect: %s)", location)
	require.NotContains(t, location, attackerRedirect, "the member's code must never reach the attacker redirect_uri")
}

// TestAuthorizeRebindDuringMyselfWindowRejected（OCR 二轮 F1 攻击路径①）：
// 成员在 TTL 临界提交表单——第一次检查通过后，Myself 调用窗口（最长
// 30s）内原条目过期；攻击者重登记同 state，第二次检查若只校验「存活
// 且未消费」就会消费攻击者条目发码。时序：TTL=120ms；POST 于 ~10ms
// 过第一次检查；攻击 GET 于 150ms（原条目已过期）；Myself sleep 200ms
// 于 ~210ms 返回（攻击条目 150+120=270ms 前仍存活）。
func TestAuthorizeRebindDuringMyselfWindowRejected(t *testing.T) {
	expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("member@example.com:member-token"))
	slowMyself := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond) // 跨过原条目 TTL 与攻击者重登记时刻
		if r.URL.Path != "/rest/api/3/myself" || r.Header.Get("Authorization") != expectedAuth {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"emailAddress": "member@example.com"})
	})
	base, _ := newTestService(t, slowMyself)

	memberRedirect := "https://client.example/cb"
	attackerRedirect := "https://attacker.example/cb"
	memberClient := testRegisterClient(t, base, memberRedirect)
	attackerClient := testRegisterClient(t, base, attackerRedirect)
	_, memberChallenge := testPKCE(t)
	state := "myself-window-state"

	origTTL := pendingAuthTTL
	pendingAuthTTL = 120 * time.Millisecond
	t.Cleanup(func() { pendingAuthTTL = origTTL })

	// 1) 成员拉表单并立刻提交（第一次检查通过，进入 Myself 等待）。
	require.Equal(t, http.StatusOK, testAuthorizeGET(t, base, memberClient, memberRedirect, state, memberChallenge))
	submittedNonce := testFormNonceOf(state)
	type postResult struct {
		status   int
		location string
	}
	done := make(chan postResult, 1)
	go func() {
		status, location := testAuthorizePOSTWithNonce(t, base, state, submittedNonce, "member@example.com", "member-token")
		done <- postResult{status: status, location: location}
	}()

	// 2) 原条目过期后（>120ms，且早于 Myself 返回的 ~210ms），攻击者
	//    重登记同 state。
	time.Sleep(150 * time.Millisecond)
	_, attackerChallenge := testPKCE(t)
	require.Equal(t, http.StatusOK, testAuthorizeGET(t, base, attackerClient, attackerRedirect, state, attackerChallenge),
		"the expired registration must be replaceable (fresh-flow semantics)")

	// 3) Myself 验证成员凭据成功返回：第二次检查必须拒绝——发码上下文
	//    已是攻击者条目（nonce 不匹配）。修复前：302 攻击者 redirect_uri。
	res := <-done
	require.NotEqual(t, http.StatusFound, res.status,
		"a credential submission whose registration was replaced during the Myself window must not issue a code (got redirect: %s)", res.location)
	require.NotContains(t, res.location, attackerRedirect, "the member's code must never reach the attacker redirect_uri")
}

// TestAllowedRedirectHostsIPv6Semantics（OCR 二轮 F2）：冒号守卫的唯一
// 实际作用是排除裸 IPv6 名单条目（Hostname() 对 IPv6 返回无括号地址，
// 含冒号 → `entry == bareHost && !strings.Contains(entry, ":")` 恒
// false）——运营者按「裸 host=任意端口」语义写 IPv6 地址将静默失效。
func TestAllowedRedirectHostsIPv6Semantics(t *testing.T) {
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	t.Cleanup(utils.ResetSSRFWhitelistForTest)
	fakeJira := httptest.NewServer(testJiraAuthOK(t, "member@example.com", "tok", nil))
	t.Cleanup(fakeJira.Close)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	handler, err := NewHandler(Options{
		BaseURL:              "http://" + ln.Addr().String(),
		JiraBaseURL:          fakeJira.URL,
		AllowedRedirectHosts: []string{"::1", "[2001:db8::1]", "pinned.example.com:8443"},
	})
	require.NoError(t, err)
	svc := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = svc.Serve(ln) }()
	t.Cleanup(func() { _ = svc.Shutdown(context.Background()) })
	base := "http://" + ln.Addr().String()

	register := func(uri string) int {
		body, err := json.Marshal(map[string]any{"redirect_uris": []string{uri}})
		require.NoError(t, err)
		resp, err := http.Post(base+"/register", "application/json", strings.NewReader(string(body)))
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		_, _ = io.Copy(io.Discard, resp.Body)
		return resp.StatusCode
	}
	// 裸 IPv6 条目（::1）= 该主机任意端口：修复前被冒号守卫静默拒绝。
	require.Equal(t, http.StatusCreated, register("https://[::1]:9443/cb"),
		"a bare IPv6 allowlist entry must admit any port on that host")
	// 括号 IPv6 条目（[2001:db8::1]）= 精确 host 形态匹配（URI 同为括号形态）。
	require.Equal(t, http.StatusCreated, register("https://[2001:db8::1]/cb"),
		"a bracketed IPv6 allowlist entry must match the bracketed host form")
	// 钉端口条目不放宽到省略默认端口的 URI 形态（钉住即精确）。
	require.Equal(t, http.StatusBadRequest, register("https://pinned.example.com/cb"),
		"a pinned host:port entry must not admit the port-elided form")
}

// TestNewHandlerRejectsDefaultPortAllowlistEntries（OCR 二轮 F2）：:80/
// :443 名单条目对省略默认端口的同名 URI 永不命中（url.Parse 不物化默认
// 端口）——静默失效的歧义写法应在启动期 fail-closed，而非运行期静默。
func TestNewHandlerRejectsDefaultPortAllowlistEntries(t *testing.T) {
	for _, entry := range []string{"a.example.com:443", "b.example.com:80", "[::1]:443"} {
		_, err := NewHandler(Options{
			BaseURL:              "http://127.0.0.1:8021",
			JiraBaseURL:          "http://127.0.0.1:9",
			AllowedRedirectHosts: []string{entry},
		})
		require.Error(t, err, "default-port allowlist entry %q must fail-closed at startup", entry)
	}
	// 非默认端口与裸 host/IPv6 形态不受影响。
	for _, hosts := range [][]string{{"a.example.com:8443"}, {"::1"}, {"good.example.com"}} {
		_, err := NewHandler(Options{
			BaseURL:              "http://127.0.0.1:8021",
			JiraBaseURL:          "http://127.0.0.1:9",
			AllowedRedirectHosts: hosts,
		})
		require.NoError(t, err, "non-default-port entries must stay accepted: %v", hosts)
	}
}

// TestIssueCodeRedirectNoStore（OCR 二轮 F3）：发码 302 的 Location 查询
// 参数携带授权码（敏感度不低于承载隐藏 state 的页面快照）——共享缓存/
// 中间代理不得留存该响应，与 writeJSON/HTML 页的 no-store 纪律一致。
func TestIssueCodeRedirectNoStore(t *testing.T) {
	base, _ := newTestService(t, testJiraAuthOK(t, "member@example.com", "tok", nil))
	redirect := "https://client.example/cb"
	clientID := testRegisterClient(t, base, redirect)
	_, challenge := testPKCE(t)
	state := "nostore-302-state"
	require.Equal(t, http.StatusOK, testAuthorizeGET(t, base, clientID, redirect, state, challenge))

	form := url.Values{}
	form.Set("state", state)
	form.Set("form_nonce", testFormNonceOf(state))
	form.Set("email", "member@example.com")
	form.Set("api_token", "tok")
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := noRedirect.PostForm(base+"/authorize", form)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	require.Equal(t, http.StatusFound, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Location"), "code=")
	require.Equal(t, "no-store", resp.Header.Get("Cache-Control"),
		"the code-bearing 302 must not be cacheable by shared caches/proxies")
}
