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
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/plugins"
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
