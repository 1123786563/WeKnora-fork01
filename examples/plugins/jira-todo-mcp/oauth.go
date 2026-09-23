package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// oauthServer 是一个简化的 OAuth 2.0 授权码服务器（内存态）：
//   - RFC 9728 protected-resource / RFC 8414 authorization-server metadata；
//   - RFC 7591 动态客户端注册（POST /register）；
//   - 授权码 + PKCE S256（GET/POST /authorize）；
//   - 换发与续期（POST /token：authorization_code / refresh_token）。
//
// 会话数据（成员 Jira 邮箱 + API token）仅驻留内存：code、state、token 均
// 一次性使用；凭据不落盘、不写日志、不出现在任何重定向或错误页。
// 这是示例服务的教学级实现：单实例、无持久化、无过期清理协程——生产部署
// 应替换为专业 OAuth 实现（见 README）。
type oauthServer struct {
	baseURL     string
	jiraBaseURL string

	mu sync.Mutex
	// clients: 动态注册的 client_id 集合。
	clients map[string]struct{}
	// pendingAuths: state -> 授权请求参数（进入表单页时登记，提交时一次性消费）。
	pendingAuths map[string]authorizationRequest
	// codes: 授权码 -> 发码上下文（一次性消费）。
	codes map[string]issuedCode
	// tokens: access_token -> 授权会话。
	tokens map[string]*oauthSession
	// refreshTokens: refresh_token -> 对应 access_token。
	refreshTokens map[string]string
}

// oauthSession 是一次成功授权后的成员会话（内存驻留）。
type oauthSession struct {
	Email    string
	APIToken string
}

// authorizationRequest 记录 GET /authorize 携带的参数，供 POST 提交时核对。
type authorizationRequest struct {
	ClientID            string
	RedirectURI         string
	CodeChallenge       string
	CodeChallengeMethod string
}

// issuedCode 是授权码及其发码上下文。
type issuedCode struct {
	Session   oauthSession
	Request   authorizationRequest
	ExpiresAt time.Time
}

// sessionContextKey 是会话注入 context 的键类型（不与 SDK 冲突）。
type sessionContextKey struct{}

func newOAuthServer(baseURL, jiraBaseURL string) *oauthServer {
	return &oauthServer{
		baseURL:       baseURL,
		jiraBaseURL:   jiraBaseURL,
		clients:       make(map[string]struct{}),
		pendingAuths:  make(map[string]authorizationRequest),
		codes:         make(map[string]issuedCode),
		tokens:        make(map[string]*oauthSession),
		refreshTokens: make(map[string]string),
	}
}

// randomToken 用 crypto/rand 生成 URL 安全的随机标识（code/state/token/
// client_id 共用）。
func randomToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		// 加密随机源不可用时 fail-closed：授权基础设施不能降级到可预测标识。
		panic(fmt.Sprintf("crypto/rand unavailable: %v", err))
	}
	return hex.EncodeToString(buf)
}

// contextFunc 是 StreamableHTTPServer 的 HTTPContextFunc：从每个入站请求
// 提取 Bearer 并解析为已授权会话，注入工具 handler 的 context。无效或缺失
// token 注入 nil 会话（工具执行防御性拒绝；HTTP 层 gate 已先行拦截）。
func (s *oauthServer) contextFunc(ctx context.Context, r *http.Request) context.Context {
	session := s.lookupSession(bearerToken(r.Header.Get("Authorization")))
	return context.WithValue(ctx, sessionContextKey{}, session)
}

// sessionFromContext 取出 contextFunc 注入的会话（可能为 nil）。
func sessionFromContext(ctx context.Context) *oauthSession {
	session, _ := ctx.Value(sessionContextKey{}).(*oauthSession)
	return session
}

// lookupSession 返回 access_token 对应会话；无效 token 返回 nil。
func (s *oauthServer) lookupSession(token string) *oauthSession {
	if token == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tokens[token]
}

// hasValidSession 报告 Bearer token 是否映射到已授权会话（gate 用）。
func (s *oauthServer) hasValidSession(token string) bool {
	return s.lookupSession(token) != nil
}

// handleProtectedResource 服务 RFC 9728 元数据：authorization_servers 指向
// 本服务自身。WeKnora 客户端 401 后拉取该文档以发现授权服务器。
func (s *oauthServer) handleProtectedResource(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"resource":              s.baseURL,
		"authorization_servers": []string{s.baseURL},
	})
}

// handleAuthorizationServer 服务 RFC 8414 授权服务器元数据。
func (s *oauthServer) handleAuthorizationServer(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                 s.baseURL,
		"authorization_endpoint": s.baseURL + "/authorize",
		"token_endpoint":         s.baseURL + "/token",
		"registration_endpoint":  s.baseURL + "/register",
		"grant_types":            []string{"authorization_code", "refresh_token"},
		"response_types":         []string{"code"},
		"code_challenge_methods": []string{"S256"},
	})
}

// handleRegister 是 RFC 7591 动态客户端注册（简化）：接受任意注册文档，
// 签发随机 client_id。redirect_uris 原样回显（token 交换时不做二次比对，
// authorize 时以注册时提交的 redirect_uri 为准——示例级实现）。
func (s *oauthServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var registration struct {
		RedirectURIs []string `json:"redirect_uris"`
		ClientName   string   `json:"client_name"`
	}
	// 注册文档解析失败也放行——本简化实现只关心 client_id 签发。
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&registration)
	clientID := "jtm-" + randomToken()
	s.mu.Lock()
	s.clients[clientID] = struct{}{}
	s.mu.Unlock()
	writeJSON(w, http.StatusCreated, map[string]any{
		"client_id":           clientID,
		"client_id_issued_at": time.Now().Unix(),
		"client_name":         registration.ClientName,
		"redirect_uris":       registration.RedirectURIs,
	})
}

// handleAuthorize 服务 GET（渲染表单，登记 state）与 POST（验证成员 Jira
// 凭据 + PKCE，发一次性授权码并 302 回 redirect_uri）。
func (s *oauthServer) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.serveAuthorizeForm(w, r)
	case http.MethodPost:
		s.submitAuthorizeForm(w, r)
	default:
		http.Error(w, "GET or POST required", http.StatusMethodNotAllowed)
	}
}

// serveAuthorizeForm 校验授权请求参数并渲染凭据输入表单。
func (s *oauthServer) serveAuthorizeForm(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := authorizationRequest{
		ClientID:            q.Get("client_id"),
		RedirectURI:         q.Get("redirect_uri"),
		CodeChallenge:       q.Get("code_challenge"),
		CodeChallengeMethod: q.Get("code_challenge_method"),
	}
	if err := s.validateAuthorizationRequest(req, q.Get("response_type")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	state := q.Get("state")
	if state == "" {
		http.Error(w, "state is required", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.pendingAuths[state] = req
	s.mu.Unlock()
	// 表单把授权参数（含 state）藏在隐藏字段；凭据字段仅在提交瞬间经
	// HTTPS 到达本服务，不进入任何存储。
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="zh-CN"><head><meta charset="utf-8"><title>Jira 授权</title></head>
<body>
<h1>授权 WeKnora 查询你的本周 Jira 待办</h1>
<p>输入你的 Jira 邮箱与 API token（仅用于本次授权验证，服务不保存凭据）。</p>
<form method="POST" action="/authorize">
<input type="hidden" name="state" value=%q>
<label>邮箱 <input type="email" name="email" required autocomplete="off"></label><br>
<label>API token <input type="password" name="api_token" required autocomplete="off"></label><br>
<button type="submit">授权</button>
</form>
</body></html>`, html.EscapeString(state))
}

// validateAuthorizationRequest 校验授权请求：response_type=code、client 已
// 注册、redirect_uri 为 http/https 绝对地址、PKCE S256。
func (s *oauthServer) validateAuthorizationRequest(req authorizationRequest, responseType string) error {
	if responseType != "code" {
		return fmt.Errorf("response_type must be \"code\"")
	}
	s.mu.Lock()
	_, registered := s.clients[req.ClientID]
	s.mu.Unlock()
	if !registered {
		return fmt.Errorf("unknown client_id")
	}
	redirect, err := url.Parse(req.RedirectURI)
	if err != nil || redirect.Scheme == "" || redirect.Host == "" {
		return fmt.Errorf("redirect_uri must be an absolute http(s) URL")
	}
	if redirect.Scheme != "http" && redirect.Scheme != "https" {
		return fmt.Errorf("redirect_uri must use http or https")
	}
	if req.CodeChallenge == "" {
		return fmt.Errorf("code_challenge is required (PKCE)")
	}
	if req.CodeChallengeMethod != "S256" {
		return fmt.Errorf("code_challenge_method must be S256")
	}
	return nil
}

// submitAuthorizeForm 处理凭据提交：state 一次性消费，凭据经 Jira
// /rest/api/3/myself 验证成功才发码；失败渲染错误页（无 code、无凭据回显）。
func (s *oauthServer) submitAuthorizeForm(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	state := r.PostFormValue("state")
	email := strings.TrimSpace(r.PostFormValue("email"))
	apiToken := r.PostFormValue("api_token")
	s.mu.Lock()
	req, ok := s.pendingAuths[state]
	delete(s.pendingAuths, state) // state 一次性
	s.mu.Unlock()
	if !ok {
		http.Error(w, "unknown or already-used state", http.StatusBadRequest)
		return
	}
	// 凭据只进内存变量；Myself 失败信息不含凭据本身。
	client := &JiraClient{BaseURL: s.jiraBaseURL, Email: email, APIToken: apiToken}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if _, err := client.Myself(ctx); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `<!DOCTYPE html>
<html lang="zh-CN"><head><meta charset="utf-8"><title>授权失败</title></head>
<body><h1>授权失败</h1><p>Jira 凭据验证未通过。请返回重试。</p></body></html>`)
		return
	}
	code := randomToken()
	s.mu.Lock()
	s.codes[code] = issuedCode{
		Session:   oauthSession{Email: email, APIToken: apiToken},
		Request:   req,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	s.mu.Unlock()
	redirect, err := url.Parse(req.RedirectURI)
	if err != nil {
		http.Error(w, "invalid redirect_uri", http.StatusBadRequest)
		return
	}
	query := redirect.Query()
	query.Set("code", code)
	if state != "" {
		query.Set("state", state)
	}
	redirect.RawQuery = query.Encode()
	http.Redirect(w, r, redirect.String(), http.StatusFound)
}

// handleToken 服务 POST /token：
//   - grant_type=authorization_code：code 一次性消费 + PKCE S256 校验，
//     签发 access_token 与 refresh_token；
//   - grant_type=refresh_token：续期 access_token（会话不变）。
func (s *oauthServer) handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_request"})
		return
	}
	switch r.PostFormValue("grant_type") {
	case "authorization_code":
		s.exchangeCode(w, r)
	case "refresh_token":
		s.refresh(w, r)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unsupported_grant_type"})
	}
}

// exchangeCode 校验 code + code_verifier（PKCE S256）并签发令牌。
func (s *oauthServer) exchangeCode(w http.ResponseWriter, r *http.Request) {
	code := r.PostFormValue("code")
	verifier := r.PostFormValue("code_verifier")
	clientID := r.PostFormValue("client_id")
	redirectURI := r.PostFormValue("redirect_uri")
	s.mu.Lock()
	issued, ok := s.codes[code]
	delete(s.codes, code) // code 一次性：无论后续校验成败都不再可用
	s.mu.Unlock()
	if !ok || time.Now().After(issued.ExpiresAt) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant", "error_description": "unknown or expired code"})
		return
	}
	if clientID != issued.Request.ClientID || redirectURI != issued.Request.RedirectURI {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant", "error_description": "client_id/redirect_uri mismatch"})
		return
	}
	if !verifyPKCES256(issued.Request.CodeChallenge, verifier) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant", "error_description": "PKCE verification failed"})
		return
	}
	accessToken := "at-" + randomToken()
	refreshToken := "rt-" + randomToken()
	session := issued.Session
	s.mu.Lock()
	s.tokens[accessToken] = &session
	s.refreshTokens[refreshToken] = accessToken
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": refreshToken,
		"scope":         "jira:read",
	})
}

// refresh 用 refresh_token 换新 access_token（授权会话保持不变）。
func (s *oauthServer) refresh(w http.ResponseWriter, r *http.Request) {
	refreshToken := r.PostFormValue("refresh_token")
	s.mu.Lock()
	accessToken, ok := s.refreshTokens[refreshToken]
	if ok {
		// 轮换：旧 refresh_token 一次性消费，签发新对。
		delete(s.refreshTokens, refreshToken)
	}
	s.mu.Unlock()
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant", "error_description": "unknown refresh_token"})
		return
	}
	newAccess := "at-" + randomToken()
	newRefresh := "rt-" + randomToken()
	s.mu.Lock()
	if session := s.tokens[accessToken]; session != nil {
		delete(s.tokens, accessToken)
		s.tokens[newAccess] = session
	} else {
		// 原会话已不可考：拒绝续期。
		s.mu.Unlock()
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant", "error_description": "session no longer exists"})
		return
	}
	s.refreshTokens[newRefresh] = newAccess
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  newAccess,
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": newRefresh,
		"scope":         "jira:read",
	})
}

// verifyPKCES256 校验 BASE64URL-ENCODE(SHA256(ASCII(code_verifier))) ==
// code_challenge（RFC 7636 S256）。
func verifyPKCES256(challenge, verifier string) bool {
	if verifier == "" || challenge == "" {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	encoded := base64.RawURLEncoding.EncodeToString(sum[:])
	return encoded == challenge
}

// writeJSON 写一个 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
