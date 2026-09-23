package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

// testAuthorizeGET 拉取授权表单（返回 HTTP 状态码）。
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
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

// testAuthorizePOST 提交凭据表单（返回 HTTP 状态码与 Location）。关闭重定向
// 跟随——302 的目标 redirect_uri 是虚构域名，客户端语义上也不该替成员访问。
func testAuthorizePOST(t *testing.T, base, state, email, apiToken string) (int, string) {
	t.Helper()
	form := url.Values{}
	form.Set("state", state)
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
			PageToken string `json:"pageToken"`
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
	_, _ = io.Copy(io.Discard, page.Body)
	_ = page.Body.Close()
	require.Equal(t, http.StatusOK, page.StatusCode)
	require.Equal(t, "DENY", page.Header.Get("X-Frame-Options"), "consent page must deny framing")
	require.Equal(t, "frame-ancestors 'none'", page.Header.Get("Content-Security-Policy"), "consent page must set frame-ancestors")

	// 凭据错误页（401）同属凭据录入面。
	form := url.Values{}
	form.Set("state", "framing-state")
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
