// Package plugintest 提供受控的远程 MCP 插件服务替身——仅供测试使用
// （test-only double，绝不进入生产装配）。它是 T10 起后续集成任务
// （T11/T13/T16-T19）共用的受控远端：
//
//   - MCP /mcp 端点（Streamable HTTP，目录公开、执行鉴权——与示例服务
//     examples/plugins/jira-todo-mcp 的行为契约一致）；
//   - SetTools 运行时改目录、Calls/WriteCalls 调用计数；
//   - Manifest() 同步产出合法 weknora.plugin/1 清单（digest 由
//     plugins.ToolSchemaDigest 从 live schema 计算），并经 /manifest.json
//     动态序列化；
//   - EnableOAuth(users) 启用完整 OAuth 端点集（与 T04 oauth.go 同构）：
//     401+WWW-Authenticate(resource_metadata)、RFC 9728 protected-resource、
//     RFC 8414 authorization-server、RFC 7591 动态注册、/authorize
//     （PKCE S256 + 授权码一次性）、/token（code_verifier 校验 + refresh
//     轮换）、CallTool 的 Bearer 归属（tok-<user> → 成员数据）。
//
// 与教学级示例不同，这里刻意不做容量上限/清扫节流：替身生命周期是单个
// 测试，不受未认证流量放大面约束；PKCE/一次性语义则完整保留，否则真实
// 客户端流（oauthManager.StartAuthorization → CompleteAuthorization）会
// 在发现/注册/交换阶段失真。
package plugintest

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/plugins"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/utils"
	sdkmcp "github.com/mark3labs/mcp-go/mcp"
	sdkserver "github.com/mark3labs/mcp-go/server"
)

// Tool describes one programmable tool of the stand-in server.
type Tool struct {
	Name                 string
	Description          string
	ReadOnly             bool // ReadOnly=false 的调用计入 WriteCalls（写工具计数）
	RequiresPersonalAuth bool
	Scopes               []string
	// InputSchema 是工具的原始 JSON schema bytes——清单声明的
	// input_schema_digest 即由它计算，live ListTools 返回相同 bytes。
	InputSchema string
	// Call 返回该工具的结果文本；member 是 Bearer 归属的成员名（未启用
	// OAuth 或未认证调用时为 ""）。返回 error 时以 MCP 工具错误面呈现。
	Call func(member string) (string, error)
}

const (
	// fixedRegisteredClientID 是 /register 的固定 client_id（计划 Task 11
	// 规定的替身契约）。WeKnora 侧注册键为 (tenant, service)，不会碰撞。
	fixedRegisteredClientID = "plugintest-client"

	authCodeTTL    = 10 * time.Minute
	accessTokenTTL = time.Hour
	// refreshTokenTTL 远长于 access——「access 过期收到 401 后刷新」是
	// refresh 的核心场景（与 T04 同构的语义）。
	refreshTokenTTL = 30 * 24 * time.Hour
	// maxRPCBodyBytes 封顶 /mcp 的 JSON-RPC 体（与 gate 的读取缓冲一致）。
	maxRPCBodyBytes = 1 << 20
)

// Server is the controlled remote plugin stand-in. Assemble with New,
// configure (SetTools / EnableOAuth / 可写 PluginID·Name·Version 字段)，
// then Start(t) binds the HTTP server.
type Server struct {
	// 可配置字段（Start 前设置；Start 后只读）。
	PluginID string
	Name     string
	Version  string

	mu          sync.Mutex
	tools       []Tool
	mcpServer   *sdkserver.MCPServer
	transport   *sdkserver.StreamableHTTPServer
	srv         *httptest.Server
	baseURL     string
	calls       atomic.Int64
	writeCalls  atomic.Int64
	oauth       oauthStub
	metadataURL string // baseURL 下的 protected-resource 文档地址（Start 时定型）
	// manifestMutations 是经 ManifestHandler_mutate 登记的清单改写钩子
	//（升级差异场景：只改清单声明——版本/端点——不动 live 目录）。
	manifestMutations []func(*types.PluginManifest)
}

// New returns an unstarted server with default identity.
func New() *Server {
	return &Server{
		PluginID: "com.example.plugintest",
		Name:     "Plugintest 插件",
		Version:  "1.0.0",
	}
}

// EnableOAuth turns on the full OAuth endpoint set. users maps member name →
// credential (the /authorize consent form selects the member by these).
// Callable before or after Start.
func (s *Server) EnableOAuth(users map[string]string) {
	s.oauth.enable(users)
}

// BaseURL returns the bound base URL (empty before Start).
func (s *Server) BaseURL() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.baseURL
}

// ManifestURL returns the stand-in's manifest address (Start 后可用).
func (s *Server) ManifestURL() string { return s.BaseURL() + "/manifest.json" }

// Calls returns the number of read-tool invocations.
func (s *Server) Calls() int64 { return s.calls.Load() }

// WriteCalls returns the number of write-tool invocations.
func (s *Server) WriteCalls() int64 { return s.writeCalls.Load() }

// Start binds the HTTP server (MCP + OAuth + manifest), allows 127.0.0.1 in
// the SSRF whitelist for this test, and registers cleanup. Returns s for
// chaining.
func (s *Server) Start(t testing.TB) *Server {
	t.Helper()
	// httptest 监听 127.0.0.1——快照恢复式放行（-count>=2 安全）。
	t.Cleanup(utils.SnapshotSSRFWhitelistForTest())
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")

	s.mu.Lock()
	s.assembleLocked()
	transport := s.transport
	s.mu.Unlock()

	mux := http.NewServeMux()
	mux.Handle("/mcp", s.gateMCP(transport))
	mux.HandleFunc("/.well-known/oauth-protected-resource", s.handleProtectedResource)
	mux.HandleFunc("/.well-known/oauth-authorization-server", s.handleAuthorizationServer)
	mux.HandleFunc("/register", s.handleRegister)
	mux.HandleFunc("/authorize", s.handleAuthorize)
	mux.HandleFunc("/token", s.handleToken)
	mux.HandleFunc("/manifest.json", s.handleManifest)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	s.mu.Lock()
	s.srv = srv
	s.baseURL = srv.URL
	s.metadataURL = srv.URL + "/.well-known/oauth-protected-resource"
	s.mu.Unlock()
	return s
}

// SetTools replaces the live tool directory at runtime.
func (s *Server) SetTools(tools []Tool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.assembleLocked()
	if names := toolNames(s.tools); len(names) > 0 {
		s.mcpServer.DeleteTools(names...)
	}
	s.tools = append([]Tool(nil), tools...)
	for _, tool := range s.tools {
		s.addToolLocked(tool)
	}
}

// assembleLocked lazily builds the MCP server + transport (s.mu held).
func (s *Server) assembleLocked() {
	if s.transport != nil {
		return
	}
	s.mcpServer = sdkserver.NewMCPServer("plugintest", "1.0.0", sdkserver.WithInputSchemaValidation())
	s.transport = sdkserver.NewStreamableHTTPServer(s.mcpServer,
		sdkserver.WithStateLess(true),
		// 从每个入站 HTTP 请求解析 Bearer 归属并注入工具 handler 的
		// context（SDK 工具 handler 本身不透传 HTTP 头）。
		sdkserver.WithHTTPContextFunc(s.contextFunc),
	)
}

func toolNames(tools []Tool) []string {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Name)
	}
	return names
}

func (s *Server) addToolLocked(t Tool) {
	tool := sdkmcp.NewToolWithRawSchema(t.Name, t.Description, json.RawMessage(t.InputSchema))
	s.mcpServer.AddTool(tool, func(ctx context.Context, req sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		member := memberFromContext(ctx)
		if t.ReadOnly {
			s.calls.Add(1)
		} else {
			s.writeCalls.Add(1)
		}
		call := t.Call
		if call == nil {
			return sdkmcp.NewToolResultText("ok:" + t.Name + ":" + member), nil
		}
		result, err := call(member)
		if err != nil {
			return sdkmcp.NewToolResultError(err.Error()), nil
		}
		return sdkmcp.NewToolResultText(result), nil
	})
}

// Manifest synchronously derives the weknora.plugin/1 manifest from the
// CURRENT tool directory（digest 从 live schema bytes 计算，与远端
// ListTools 一致）；transport.endpoint 指向已绑定的 /mcp。
func (s *Server) Manifest() *types.PluginManifest {
	s.mu.Lock()
	defer s.mu.Unlock()
	endpoint := ""
	if s.baseURL != "" {
		endpoint = s.baseURL + "/mcp"
	}
	decls := make([]types.PluginToolDecl, 0, len(s.tools))
	anyAuth := false
	scopeSeen := map[string]bool{}
	var authScopes []string
	for _, t := range s.tools {
		if t.RequiresPersonalAuth {
			anyAuth = true
			for _, scope := range t.Scopes {
				if !scopeSeen[scope] {
					scopeSeen[scope] = true
					authScopes = append(authScopes, scope)
				}
			}
		}
		decls = append(decls, types.PluginToolDecl{
			Name:                 t.Name,
			ReadOnly:             t.ReadOnly,
			RequiresPersonalAuth: t.RequiresPersonalAuth,
			Scopes:               t.Scopes,
			InputSchemaDigest:    plugins.ToolSchemaDigest([]byte(t.InputSchema)),
		})
	}
	m := &types.PluginManifest{
		Protocol:  "weknora.plugin/1",
		PluginID:  s.PluginID,
		Version:   s.Version,
		Name:      s.Name,
		Transport: types.PluginTransport{Type: "http-streamable", Endpoint: endpoint},
		Tools:     decls,
	}
	if anyAuth {
		m.Auth = &types.PluginAuth{PersonalOAuth: true, Scopes: authScopes}
	}
	// 改写钩子在持锁态逐个应用——钩子必须是纯改写，不得回调 Server 的
	// 加锁方法（会自锁）；每次序列化都重放，因此 /manifest.json 始终反映
	// 改写后的清单。
	for _, mutate := range s.manifestMutations {
		if mutate != nil {
			mutate(m)
		}
	}
	return m
}

// ManifestHandler_mutate 登记一个清单改写钩子：此后的每次 Manifest() 与
// /manifest.json 序列化都会重放它（计划 Task 10 Produces——升级差异场景
// 用它发布候选版本号或改写端点声明，而保持 live 目录不变）。
func (s *Server) ManifestHandler_mutate(fn func(*types.PluginManifest)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.manifestMutations = append(s.manifestMutations, fn)
}

func (s *Server) handleManifest(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.Manifest())
}

// ---------------------------------------------------------------------------
// MCP gate：目录公开、执行鉴权（与 T04 gateCallToolAuth 同构）
// ---------------------------------------------------------------------------

type memberContextKey struct{}

func memberFromContext(ctx context.Context) string {
	member, _ := ctx.Value(memberContextKey{}).(string)
	return member
}

func bearerToken(header string) string {
	scheme, value, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(value)
}

// gateMCP passes non-tools/call traffic through; a tools/call POST must carry
// a Bearer token resolving to a live member session when OAuth is enabled,
// otherwise 401 + WWW-Authenticate (RFC 9728) — the WeKnora client's
// discovery trigger.
func (s *Server) gateMCP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.oauth.enabled() || r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRPCBodyBytes))
		if err != nil {
			http.Error(w, "unable to read request body", http.StatusBadRequest)
			return
		}
		var rpc struct {
			Method string `json:"method"`
		}
		// fail-closed：只接受单个 JSON-RPC 对象——绝不把鉴权决策建立在
		// 「自行解析失败即放行」上。
		if err := json.Unmarshal(body, &rpc); err != nil {
			http.Error(w, "request body must be a single JSON-RPC object", http.StatusBadRequest)
			return
		}
		if rpc.Method == "tools/call" && s.oauth.lookupMember(bearerToken(r.Header.Get("Authorization"))) == "" {
			s.mu.Lock()
			metadataURL := s.metadataURL
			s.mu.Unlock()
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer resource_metadata=%q`, metadataURL))
			http.Error(w, "unauthorized: this MCP tool call requires a personal OAuth bearer token", http.StatusUnauthorized)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

// contextFunc resolves the request's Bearer into a member for tool handlers.
func (s *Server) contextFunc(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, memberContextKey{}, s.oauth.lookupMember(bearerToken(r.Header.Get("Authorization"))))
}

// ---------------------------------------------------------------------------
// OAuth 端点集（与 T04 oauth.go 同构的测试替身实现）
// ---------------------------------------------------------------------------

type authzRequest struct {
	State               string
	ClientID            string
	RedirectURI         string
	CodeChallenge       string
	CodeChallengeMethod string
}

type issuedCode struct {
	Request   authzRequest
	Member    string
	ExpiresAt time.Time
}

type tokenEntry struct {
	Member    string
	ExpiresAt time.Time
}

type refreshEntry struct {
	Member    string
	ExpiresAt time.Time
}

// credentialCheckFunc replaces oauthStub's in-table credential comparison:
// the Jira-shaped stand-in (T13) uses it to verify the submitted credentials
// against the fake Jira /rest/api/3/myself over real HTTP (same semantics as
// the T04 example service's authorize page) before a one-time code is issued.
type credentialCheckFunc func(username, password string) error

// oauthStub is the in-memory authorization server: users gates the whole
// endpoint set (nil = disabled); codes/tokens are one-time; access tokens
// are deterministic "tok-<user>" so CallTool attribution is observable.
type oauthStub struct {
	mu            sync.Mutex
	users         map[string]string
	clients       map[string][]string // client_id -> registered redirect_uris
	pendingAuths  map[string]authzRequest
	codes         map[string]issuedCode
	tokens        map[string]tokenEntry
	refreshTokens map[string]refreshEntry
	// credentialCheck 非 nil 时（enableWithCheck），凭据验证以它为权威，
	// users 仅标记端点集启用；sessionCredentials 记录各成员授权时提交的
	// 凭据（member → password），供 Jira 形替身的工具执行取出站调用。
	credentialCheck    credentialCheckFunc
	sessionCredentials map[string]string
}

func (o *oauthStub) enable(users map[string]string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.users = users
	if o.clients == nil {
		o.clients = map[string][]string{}
	}
	if o.pendingAuths == nil {
		o.pendingAuths = map[string]authzRequest{}
	}
	if o.codes == nil {
		o.codes = map[string]issuedCode{}
	}
	if o.tokens == nil {
		o.tokens = map[string]tokenEntry{}
	}
	if o.refreshTokens == nil {
		o.refreshTokens = map[string]refreshEntry{}
	}
	if o.sessionCredentials == nil {
		o.sessionCredentials = map[string]string{}
	}
}

// enableWithCheck turns on the endpoint set with an external credential
// authority: authorization succeeds iff check accepts the submitted
// (username, password) — the stand-in then records the credential for the
// member's tool-side egress (sessionCredential).
func (o *oauthStub) enableWithCheck(check credentialCheckFunc) {
	o.enable(map[string]string{}) // 非 nil users 标记启用；权威在 check
	o.mu.Lock()
	defer o.mu.Unlock()
	o.credentialCheck = check
}

func (o *oauthStub) enabled() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.users != nil
}

// lookupMember resolves a Bearer to its member, "" when unknown/expired.
func (o *oauthStub) lookupMember(token string) string {
	if token == "" {
		return ""
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	entry, ok := o.tokens[token]
	if !ok || time.Now().After(entry.ExpiresAt) {
		return ""
	}
	return entry.Member
}

// register validates and binds the dynamic client registration.
func (o *oauthStub) register(uris []string) error {
	if len(uris) == 0 {
		return fmt.Errorf("at least one absolute http(s) redirect_uri is required")
	}
	for _, uri := range uris {
		parsed, err := url.Parse(uri)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" ||
			(parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("redirect_uri must be an absolute http(s) URL")
		}
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.clients[fixedRegisteredClientID] = uris
	return nil
}

// startAuthorization validates GET /authorize params and registers the
// pending state（发码前的请求参数登记）。
func (o *oauthStub) startAuthorization(req authzRequest) error {
	if req.CodeChallengeMethod != "S256" {
		return fmt.Errorf("code_challenge_method must be S256")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	uris, ok := o.clients[req.ClientID]
	if !ok {
		return fmt.Errorf("unknown client_id")
	}
	registered := false
	for _, uri := range uris {
		if req.RedirectURI == uri {
			registered = true
			break
		}
	}
	if !registered {
		return fmt.Errorf("redirect_uri does not match the registered redirect_uris for this client")
	}
	if req.CodeChallenge == "" {
		return fmt.Errorf("code_challenge is required (PKCE)")
	}
	o.pendingAuths[req.State] = req
	return nil
}

// submitCredentials consumes the pending state on successful credential
// validation and mints a ONE-TIME code, returning the redirect location.
// Credentials failure keeps the state (retry-able), mirroring T04. When a
// credentialCheck is configured (enableWithCheck) it is the authority;
// otherwise the in-table users comparison applies. On success the submitted
// credential is recorded in sessionCredentials for tool-side egress.
func (o *oauthStub) submitCredentials(state, username, password string) (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	req, ok := o.pendingAuths[state]
	if !ok {
		return "", fmt.Errorf("unknown or already-consumed state")
	}
	if o.credentialCheck != nil {
		if err := o.credentialCheck(username, password); err != nil {
			return "", err
		}
	} else {
		credential, known := o.users[username]
		if !known || credential != password {
			return "", fmt.Errorf("invalid credentials")
		}
	}
	delete(o.pendingAuths, state)
	code := randomToken()
	o.codes[code] = issuedCode{Request: req, Member: username, ExpiresAt: time.Now().Add(authCodeTTL)}
	if o.sessionCredentials == nil {
		o.sessionCredentials = map[string]string{}
	}
	o.sessionCredentials[username] = password
	redirect, err := url.Parse(req.RedirectURI)
	if err != nil {
		return "", fmt.Errorf("registered redirect_uri is no longer parsable: %w", err)
	}
	q := redirect.Query()
	q.Set("code", code)
	q.Set("state", state)
	redirect.RawQuery = q.Encode()
	return redirect.String(), nil
}

// sessionCredential returns the credential a member submitted at
// authorization time ("" when the member never authorized) — the Jira-shaped
// stand-in's tool handler uses it as the egress credential.
func (o *oauthStub) sessionCredential(member string) string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.sessionCredentials[member]
}

// exchangeCode validates + consumes the one-time code（PKCE S256）and
// issues the member's deterministic access token.
func (o *oauthStub) exchangeCode(code, verifier, clientID, redirectURI string) (string, string, error) {
	o.mu.Lock()
	issued, ok := o.codes[code]
	delete(o.codes, code) // code 一次性：无论后续校验成败都不再可用
	o.mu.Unlock()
	if !ok || time.Now().After(issued.ExpiresAt) {
		return "", "", fmt.Errorf("unknown or expired code")
	}
	if clientID != issued.Request.ClientID || redirectURI != issued.Request.RedirectURI {
		return "", "", fmt.Errorf("client_id/redirect_uri mismatch")
	}
	if !verifyPKCES256(issued.Request.CodeChallenge, verifier) {
		return "", "", fmt.Errorf("PKCE verification failed")
	}
	accessToken := "tok-" + issued.Member
	refreshToken := "rt-" + randomToken()
	now := time.Now()
	o.mu.Lock()
	o.tokens[accessToken] = tokenEntry{Member: issued.Member, ExpiresAt: now.Add(accessTokenTTL)}
	o.refreshTokens[refreshToken] = refreshEntry{Member: issued.Member, ExpiresAt: now.Add(refreshTokenTTL)}
	o.mu.Unlock()
	return accessToken, refreshToken, nil
}

// refresh rotates a refresh token（会话成员不变，旧 token 一次性消费）。
func (o *oauthStub) refresh(refreshToken string) (string, string, error) {
	o.mu.Lock()
	entry, ok := o.refreshTokens[refreshToken]
	valid := ok && !time.Now().After(entry.ExpiresAt)
	if valid {
		delete(o.refreshTokens, refreshToken)
	}
	o.mu.Unlock()
	if !valid {
		return "", "", fmt.Errorf("unknown or expired refresh_token")
	}
	accessToken := "tok-" + entry.Member
	newRefresh := "rt-" + randomToken()
	now := time.Now()
	o.mu.Lock()
	o.tokens[accessToken] = tokenEntry{Member: entry.Member, ExpiresAt: now.Add(accessTokenTTL)}
	o.refreshTokens[newRefresh] = refreshEntry{Member: entry.Member, ExpiresAt: now.Add(refreshTokenTTL)}
	o.mu.Unlock()
	return accessToken, newRefresh, nil
}

func (s *Server) handleProtectedResource(w http.ResponseWriter, r *http.Request) {
	if !s.oauth.enabled() {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"resource":              s.BaseURL(),
		"authorization_servers": []string{s.BaseURL()},
	})
}

func (s *Server) handleAuthorizationServer(w http.ResponseWriter, r *http.Request) {
	if !s.oauth.enabled() {
		http.NotFound(w, r)
		return
	}
	base := s.BaseURL()
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                 base,
		"authorization_endpoint": base + "/authorize",
		"token_endpoint":         base + "/token",
		"registration_endpoint":  base + "/register",
		"grant_types":            []string{"authorization_code", "refresh_token"},
		"response_types":         []string{"code"},
		"code_challenge_methods": []string{"S256"},
	})
}

// handleRegister is RFC 7591 dynamic client registration（简化）：注册体
// 至少携带一个绝对 http(s) redirect_uri 并绑定到固定 client_id。
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if !s.oauth.enabled() {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var registration struct {
		RedirectURIs []string `json:"redirect_uris"`
		ClientName   string   `json:"client_name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRPCBodyBytes)).Decode(&registration); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invalid_client_metadata", "error_description": "request body must be a valid JSON registration document",
		})
		return
	}
	if err := s.oauth.register(registration.RedirectURIs); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invalid_redirect_uri", "error_description": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"client_id":           fixedRegisteredClientID,
		"client_id_issued_at": time.Now().Unix(),
		"client_name":         registration.ClientName,
		"redirect_uris":       registration.RedirectURIs,
	})
}

// handleAuthorize：GET 校验并登记授权请求（渲染应答面）；POST 凭据提交
// → 校验 users 表成员 → 302 redirect_uri?code=...&state=...（code 一次性）。
func (s *Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	if !s.oauth.enabled() {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query()
		req := authzRequest{
			ClientID:            q.Get("client_id"),
			RedirectURI:         q.Get("redirect_uri"),
			CodeChallenge:       q.Get("code_challenge"),
			CodeChallengeMethod: q.Get("code_challenge_method"),
		}
		if q.Get("response_type") != "code" {
			http.Error(w, `response_type must be "code"`, http.StatusBadRequest)
			return
		}
		if req.State = q.Get("state"); req.State == "" {
			http.Error(w, "state is required", http.StatusBadRequest)
			return
		}
		if err := s.oauth.startAuthorization(req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// 应答面（真实世界是凭据表单；测试直接 POST 凭据）。
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, "<!doctype html><title>plugintest authorize</title>")
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, maxRPCBodyBytes)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		state := r.PostFormValue("state")
		username := strings.TrimSpace(r.PostFormValue("username"))
		password := r.PostFormValue("password")
		location, err := s.oauth.submitCredentials(state, username, password)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, location, http.StatusFound)
	default:
		http.Error(w, "GET or POST required", http.StatusMethodNotAllowed)
	}
}

// handleToken：authorization_code（code_verifier 校验）/ refresh_token。
func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	if !s.oauth.enabled() {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRPCBodyBytes)
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_request"})
		return
	}
	switch r.PostFormValue("grant_type") {
	case "authorization_code":
		access, refresh, err := s.oauth.exchangeCode(
			r.PostFormValue("code"), r.PostFormValue("code_verifier"),
			r.PostFormValue("client_id"), r.PostFormValue("redirect_uri"),
		)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant", "error_description": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"access_token":  access,
			"token_type":    "Bearer",
			"expires_in":    int(accessTokenTTL.Seconds()),
			"refresh_token": refresh,
			"scope":         "read:demo",
		})
	case "refresh_token":
		access, refresh, err := s.oauth.refresh(r.PostFormValue("refresh_token"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant", "error_description": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"access_token":  access,
			"token_type":    "Bearer",
			"expires_in":    int(accessTokenTTL.Seconds()),
			"refresh_token": refresh,
			"scope":         "read:demo",
		})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unsupported_grant_type"})
	}
}

// verifyPKCES256: BASE64URL(SHA256(code_verifier)) == code_challenge.
func verifyPKCES256(challenge, verifier string) bool {
	if verifier == "" || challenge == "" {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:]) == challenge
}

func randomToken() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("plugintest: rand: %v", err))
	}
	return hex.EncodeToString(b[:])
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// ---------------------------------------------------------------------------
// Jira 形替身（T13）：复刻 examples/plugins/jira-todo-mcp 的行为契约
// ---------------------------------------------------------------------------

const (
	// JiraTodoToolName 与示例服务唯一工具同名（T04 契约）。
	JiraTodoToolName = "search_my_week_issues"
	// JiraTodoToolSchema 与示例服务 canonicalToolInputSchema 逐字一致：
	// 无参数、拒绝一切额外属性——模型注入 token/jql/url/user_id 在服务端
	// schema 层（additionalProperties:false）被拒。
	JiraTodoToolSchema = `{"type":"object","properties":{},"additionalProperties":false}`
	// jiraTodoDefaultTimeout 是替身工具出站 HTTP 客户端的缺省超时（与 T04
	// jiraHTTPClient 的 30s 一致）。
	jiraTodoDefaultTimeout = 30 * time.Second
	// jiraTodoMyWeekJQL 与示例服务 jqlMyWeek（examples/plugins/jira-todo-
	// mcp/jira.go）逐字一致：JQL 由替身服务端固定模板构造，绝不接受调用
	// 方输入拼接。fake Jira 按同一语义过滤账本（未解决 + due 在本周内）。
	jiraTodoMyWeekJQL = `assignee = currentUser() AND resolution = Unresolved AND due >= startOfWeek() AND due < startOfWeek("+1w") ORDER BY due ASC`
)

// JiraTodoOption 配置 Jira 形替身。
type JiraTodoOption func(*jiraTodoConfig)

type jiraTodoConfig struct {
	timeout time.Duration
}

// WithJiraToolTimeout 覆盖替身工具出站 HTTP 客户端超时（缺省 30s；超时
// 注入场景测试传 500ms，慢 fake Jira 在此被确定性切断——计划 Task 13
// Step 3 的 Option JiraTimeout）。
func WithJiraToolTimeout(d time.Duration) JiraTodoOption {
	return func(c *jiraTodoConfig) { c.timeout = d }
}

// NewJiraTodoPlugin 返回未 Start 的 Jira 形替身：恰好一个工具
// search_my_week_issues（目录公开、执行鉴权，与示例服务同契约）；授权
// 凭据即成员 Jira 凭据（email + APIToken），经 fake Jira
// /rest/api/3/myself 真实验证后才发一次性 code；工具执行经真实 HTTP 调
// fake Jira 的 /rest/api/3/search/jql，输出行格式与示例服务 formatIssues
// 一致（[{key}] {summary} · 状态 {status} · 截止 {due} · {base}/browse/{key}）。
// HTTP ≥400 与超时都以携带语义的错误浮出，绝不伪装成空成功列表。
func NewJiraTodoPlugin(t testing.TB, jira *FakeJira, opts ...JiraTodoOption) *Server {
	t.Helper()
	cfg := jiraTodoConfig{timeout: jiraTodoDefaultTimeout}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	s := New()
	s.PluginID = "jira-todo-mcp"
	s.Name = "Jira 本周待办（测试替身）"
	// 授权凭据权威在 fake Jira：/myself 验证通过才发码（T04 授权页同语义）。
	s.oauth.enableWithCheck(func(username, password string) error {
		return jira.VerifyCredentials(username, password)
	})
	s.SetTools([]Tool{{
		Name:                 JiraTodoToolName,
		Description:          "查询当前授权成员本周内未解决的 Jira 待办事项（测试替身，与示例服务同契约；工具不接受任何参数，JQL 由服务端固定）",
		ReadOnly:             true,
		RequiresPersonalAuth: true,
		InputSchema:          JiraTodoToolSchema,
		Call: func(member string) (string, error) {
			apiToken := s.oauth.sessionCredential(member)
			if apiToken == "" {
				return "", fmt.Errorf("unauthorized: no jira credentials bound to member %s", member)
			}
			// member 即授权时提交的 Jira 邮箱：Basic(email:APIToken) 出站。
			return searchFakeJiraWeek(jira.BaseURL(), member, apiToken, cfg.timeout)
		},
	}})
	return s
}

// searchFakeJiraWeek 以成员 Jira 凭据（Basic 认证，与 T04 JiraClient 一致）
// 经 SSRF-safe client 调 fake Jira 的固定 JQL 搜索并渲染输出行。错误浮出
// 语义与 T04 jira.go 对齐：HTTP ≥400 携带状态码；超时（ctx deadline/
// Client.Timeout）以 timeout 措辞包装。
func searchFakeJiraWeek(baseURL, email, apiToken string, timeout time.Duration) (string, error) {
	cfg := utils.DefaultSSRFSafeHTTPClientConfig()
	cfg.Timeout = timeout
	client := utils.NewSSRFSafeHTTPClient(cfg)
	payload, err := json.Marshal(map[string]any{
		"jql":        jiraTodoMyWeekJQL,
		"fields":     []string{"summary", "status", "duedate"},
		"maxResults": 100,
	})
	if err != nil {
		return "", fmt.Errorf("encode jira search request: %w", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		strings.TrimRight(baseURL, "/")+"/rest/api/3/search/jql", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build jira request POST /rest/api/3/search/jql: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	// Basic 认证只进请求头，不进任何日志。
	req.Header.Set("Authorization", jiraBasicAuth(email, apiToken))
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("jira request POST /rest/api/3/search/jql timeout: %w", err)
		}
		return "", fmt.Errorf("jira request POST /rest/api/3/search/jql failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("jira POST /rest/api/3/search/jql returned HTTP %d", resp.StatusCode)
	}
	var result fakeJiraSearchResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return "", fmt.Errorf("decode jira /rest/api/3/search/jql response: %w", err)
	}
	return formatFakeJiraIssues(baseURL, result.Issues), nil
}

// fakeJiraSearchResponse 是 fake Jira 搜索响应的最小映射（字段结构与 T04
// jiraSearchResponse 一致；fake 不分页，无 nextPageToken）。
type fakeJiraSearchResponse struct {
	Issues []fakeJiraSearchIssue `json:"issues"`
}

// fakeJiraSearchIssue 是搜索结果的单条事项（fields 白名单：summary/status/
// duedate）。
type fakeJiraSearchIssue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string `json:"summary"`
		Status  struct {
			Name string `json:"name"`
		} `json:"status"`
		DueDate string `json:"duedate"`
	} `json:"fields"`
}

// formatFakeJiraIssues 与示例服务 formatIssues（jira.go）同格式渲染：每行
// [{key}] {summary} · 状态 {status} · 截止 {due} · {url}；空列表输出空
// 字符串（空数组语义，而非错误）。
func formatFakeJiraIssues(baseURL string, issues []fakeJiraSearchIssue) string {
	lines := make([]string, 0, len(issues))
	for _, issue := range issues {
		var b strings.Builder
		fmt.Fprintf(&b, "[%s]", issue.Key)
		if issue.Fields.Summary != "" {
			fmt.Fprintf(&b, " %s", issue.Fields.Summary)
		}
		if issue.Fields.Status.Name != "" {
			fmt.Fprintf(&b, " · 状态 %s", issue.Fields.Status.Name)
		}
		if issue.Fields.DueDate != "" {
			fmt.Fprintf(&b, " · 截止 %s", issue.Fields.DueDate)
		}
		fmt.Fprintf(&b, " · %s", strings.TrimRight(baseURL, "/")+"/browse/"+issue.Key)
		lines = append(lines, b.String())
	}
	return strings.Join(lines, "\n")
}
