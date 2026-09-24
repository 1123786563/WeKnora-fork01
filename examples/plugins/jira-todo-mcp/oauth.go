package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"
)

// 令牌与一次性凭据的生命周期（测试可临时改写以覆盖过期路径）。
var (
	// accessTokenTTL 是 access_token 的有效期，与 /token 响应的 expires_in
	// 一致（OCR T04-R1-2：声明与实现不得脱节）。
	accessTokenTTL = time.Hour
	// refreshTokenTTL 是 refresh_token 的独立有效期，远长于 access_token——
	// 「access 过期收到 401 后刷新」是 refresh 的核心场景（OCR T04-R2-1），
	// refresh 条目必须独立于 access 条目存活。
	refreshTokenTTL = 30 * 24 * time.Hour
	// pendingAuthTTL 是授权表单 state 的有效期。
	pendingAuthTTL = 10 * time.Minute
	// authCodeTTL 是授权码的有效期。
	authCodeTTL = 10 * time.Minute
)

const (
	// maxStateBytes 限制 state 长度：state 是未认证输入的存储键，必须封顶
	// （OCR T04-R1-4：无界内存增长面）。
	maxStateBytes = 1024
	// maxRedirectURIsPerRegistration 封顶单次注册的 redirect_uris 条数
	//（OCR T04-R2-8：聚合内存大小必须有界）。
	maxRedirectURIsPerRegistration = 16
	// maxRedirectURIBytes 封顶单条注册 redirect_uri 长度（OCR T04-R2-8）。
	maxRedirectURIBytes = 2048
	// maxClientNameBytes 封顶注册 client_name 长度（整分支 OCR 一轮 F7）：
	// /register 无认证，1MiB 注册体可携任意长度名字——不封顶则 4096 条
	// 注册可把聚合内存界「4096×16×2KiB」击穿到 GiB 级。字节计（内存单位）。
	maxClientNameBytes = 256
	// maxFormBodyBytes 封顶 /authorize 与 /token 的表单体大小（整分支终评
	// r3-006/r3-007/r4-006/r4-007）：两个端点都是未认证输入面，ParseForm
	// 前必须套 http.MaxBytesReader——与 /register 的 1MiB JSON 注册体对齐。
	maxFormBodyBytes = 1 << 20
	// maxCodeChallengeBytes 封顶 code_challenge 长度（跨任务转交 T01-R2-F9）：
	// 它与 state 同为未认证输入的存储键成分；RFC 7636 S256 的 challenge 恰为
	// 43 个 base64url 字符，128 字节余量充分。
	maxCodeChallengeBytes = 128
)

var (
	// maxRegisteredClients 封顶动态注册客户端数量（OCR T04-R1-4）。var 以便
	// 测试改写：配合 clientRegistrationTTL 淘汰验证容量释放。
	maxRegisteredClients = 4096
	// clientRegistrationTTL 是动态注册客户端的存活期（整分支 OCR 一轮
	// F5）：/register 无认证，无淘汰时攻击者可在重启前把上限永久填满
	// （此后合法客户端一律 429）。过期条目由 sweepExpiredLocked 惰性淘汰，
	// 淘汰即时释放容量。合法流程注册后数分钟内完成授权，24h 远充裕。
	clientRegistrationTTL = 24 * time.Hour
)

// maxPendingAuths 封顶窗口内未消费的授权表单 state 数量（OCR T04-R2-3：
// TTL 清扫只约束时间维度，窗口内总量必须有界）。var 以便测试改写。
var maxPendingAuths = 1024

// maxActiveTokens 封顶窗口内未过期 access_token 条目数（终评 R5 F8，闭环
// T01 账本第八轮转交的 T01-R4-F1）：持有任一有效 refresh_token 的调用方
// 可零门槛高频 /token——每次刷新令一个新 access_token 存活 accessTokenTTL，
// 窗口内可把 tokens 表推至任意大；pendingAuths 已有同款不变量（OCR
// T04-R2-3）。var 以便测试改写。
var maxActiveTokens = 8192

// maxRefreshTokens 封顶窗口内未过期 refresh_token 条目数（跨任务转交 R7
// T01-R4-F1 补充）：refreshTokens 只在授权码交换时净增（refresh 轮换 1 删
// 1 增），与 pendingAuths/clients 的「未认证/半认证写入面聚合内存有界」
// 不变量对齐。var 以便测试改写。
var maxRefreshTokens = 4096

// maxActiveCodes 封顶窗口内未消费授权码条目数（OCR 一轮 F9）：codes 曾是
// 唯一只有 TTL 没有容量上限的令牌 map——发码即消费 state 释放
// pendingAuths 名额，持有有效 Jira 凭据者可循环「登记→发码」在 authCodeTTL
// 窗口内按吞吐无界堆积（issuedCode 含 Email/APIToken/authorizationRequest）。
// var 以便测试改写。
var maxActiveCodes = 4096

// oauthServer 是一个简化的 OAuth 2.0 授权码服务器（内存态）：
//   - RFC 9728 protected-resource / RFC 8414 authorization-server metadata；
//   - RFC 7591 动态客户端注册（POST /register，注册时绑定 redirect_uris）；
//   - 授权码 + PKCE S256（GET/POST /authorize）；
//   - 换发与续期（POST /token：authorization_code / refresh_token）。
//
// 会话数据（成员 Jira 邮箱 + API token）仅驻留内存；code、state、
// refresh_token 一次性使用，access_token 按 accessTokenTTL 过期；凭据不落盘、
// 不写日志、不出现在任何重定向或错误页。注册客户端、待授权 state 与授权码
// 均在写入/读取时惰性清扫过期条目（OCR T04-R1-4）；动态注册客户端按
// clientRegistrationTTL（24h）淘汰并即时释放注册容量（整分支 OCR 一轮
// F5——/register 无认证，无淘汰则上限可被未认证方永久占满）。这是示例
// 服务的教学级
// 实现：单实例、无持久化、无后台清理协程——生产部署应替换为专业 OAuth
// 实现（见 README）。
type oauthServer struct {
	baseURL     string
	jiraBaseURL string
	// allowedRedirectHosts 非空时（装配项 Options.AllowedRedirectHosts，env
	// PLUGIN_ALLOWED_REDIRECT_HOSTS），/register 仅接受 host 在名单内的
	// redirect_uri（整分支终评 r2-012/r4-008 部署侧缓解：切断「任意注册方 +
	// 任意跳转地」的钓鱼组合；缺省空 = 教学示例语义，不限制）。
	allowedRedirectHosts map[string]struct{}

	mu sync.Mutex
	// clients: 动态注册的 client_id -> 注册信息（含绑定的 redirect_uris）。
	clients map[string]registeredClient
	// pendingAuths: state -> 授权请求参数（进入表单页时登记，凭据验证成功
	// 发码前一次性消费；凭据失败可同 state 重试，见 submitAuthorizeForm）。
	pendingAuths map[string]pendingAuth
	// codes: 授权码 -> 发码上下文（一次性消费）。
	codes map[string]issuedCode
	// tokens: access_token -> 会话条目（按 ExpiresAt 过期）。
	tokens map[string]tokenEntry
	// refreshTokens: refresh_token -> 会话条目（独立 TTL，一次性轮换）。
	refreshTokens map[string]refreshEntry
	// lastSweep 上次全量清扫时刻（s.mu 保护；OCR 二轮 F2 节流用）。
	lastSweep time.Time
}

// registeredClient 记录一次动态注册（ClientName 供同意页展示请求方——
// 整分支终评 r2-012/r4-008：成员必须在提交凭据前识别请求方）。ExpiresAt
// 是注册存活边界（整分支 OCR 一轮 F5）：/register 无认证，无淘汰时未认证
// 攻击者可在重启前把 maxRegisteredClients 上限永久填满，合法 WeKnora 客户端
// 将一律 429——过期注册由 sweepExpiredLocked 惰性淘汰并即时释放容量。
type registeredClient struct {
	ClientName   string
	RedirectURIs []string
	ExpiresAt    time.Time
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

// pendingAuth 是登记中的授权请求（含过期时间）。
type pendingAuth struct {
	Request   authorizationRequest
	ExpiresAt time.Time
}

// issuedCode 是授权码及其发码上下文。
type issuedCode struct {
	Session   oauthSession
	Request   authorizationRequest
	ExpiresAt time.Time
}

// tokenEntry 是 access_token 条目：会话 + 过期时间（OCR T04-R1-2）。
type tokenEntry struct {
	Session   *oauthSession
	ExpiresAt time.Time
}

// refreshEntry 是 refresh_token 条目：直接持有会话与独立过期时间，不依赖
// 对应 access_token 的存活（OCR T04-R2-1：access 过期后刷新必须可用）。
type refreshEntry struct {
	Session   *oauthSession
	ExpiresAt time.Time
}

// sessionContextKey 是会话注入 context 的键类型（不与 SDK 冲突）。
type sessionContextKey struct{}

func newOAuthServer(baseURL, jiraBaseURL string, allowedRedirectHosts []string) *oauthServer {
	allowlist := make(map[string]struct{}, len(allowedRedirectHosts))
	for _, host := range allowedRedirectHosts {
		allowlist[strings.ToLower(strings.TrimSpace(host))] = struct{}{}
	}
	return &oauthServer{
		baseURL:              baseURL,
		jiraBaseURL:          jiraBaseURL,
		allowedRedirectHosts: allowlist,
		clients:              make(map[string]registeredClient),
		pendingAuths:         make(map[string]pendingAuth),
		codes:                make(map[string]issuedCode),
		tokens:               make(map[string]tokenEntry),
		refreshTokens:        make(map[string]refreshEntry),
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

// sweepExpiredLocked 惰性清扫已过期的 clients / pendingAuths / codes /
// tokens / refreshTokens（调用方需持锁；OCR T04-R1-4 与 T04-R2-7：所有
// 令牌类 map 都必须有 TTL 退出路径，不留孤儿条目；clients 的淘汰见
// 整分支 OCR 一轮 F5——否则未认证 /register 可把注册容量永久占满）。
func (s *oauthServer) sweepExpiredLocked(now time.Time) {
	for clientID, client := range s.clients {
		if now.After(client.ExpiresAt) {
			delete(s.clients, clientID)
		}
	}
	for state, pending := range s.pendingAuths {
		if now.After(pending.ExpiresAt) {
			delete(s.pendingAuths, state)
		}
	}
	for code, issued := range s.codes {
		if now.After(issued.ExpiresAt) {
			delete(s.codes, code)
		}
	}
	for token, entry := range s.tokens {
		if now.After(entry.ExpiresAt) {
			delete(s.tokens, token)
		}
	}
	for refreshToken, entry := range s.refreshTokens {
		if now.After(entry.ExpiresAt) {
			delete(s.refreshTokens, refreshToken)
		}
	}
}

// validateClientName 校验动态注册的请求方显示名（OCR 二轮 F6）：拒绝
// Cc 控制字符、Zl/Zp 分隔符与 Cf 格式字符（RLO/isolate/BOM/软连字符等
// 可视觉重排或隐藏文本），豁免 U+200C/U+200D（复合 emoji 序列的必要
// 组成，与 internal/modules/plugins 的 validateDescription 同款双重标准）；
// Co 私用区有普通可见字形、按 validateDescription 先例保留合法。
func validateClientName(name string) error {
	for _, r := range name {
		invisible := unicode.IsControl(r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r) ||
			(unicode.Is(unicode.Cf, r) && r != '\u200c' && r != '\u200d')
		if invisible {
			return fmt.Errorf("client_name must not contain control, format or separator characters (U+%04X)", r)
		}
	}
	return nil
}

// maybeSweepLocked 是写路径的节流清扫入口（OCR 二轮 F2）：5 个未认证
// 写路径都曾每请求全量清扫（达上限 21504 条目、全程持 s.mu，与 /mcp
// 每请求两次的 lookupSession 争抢同一把锁）——按 sweepMinInterval 节流，
// 窗口内直接返回。过期判定不受影响（各查找路径都有单条过期判定）；
// 过期条目的容量释放从「即时」弱化为「≤窗口」。var 供测试改写。
var sweepMinInterval = time.Second

func (s *oauthServer) maybeSweepLocked(now time.Time) {
	if now.Sub(s.lastSweep) < sweepMinInterval {
		return
	}
	s.lastSweep = now
	s.sweepExpiredLocked(now)
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

// lookupSession 返回 access_token 对应会话；无效、缺失或已过期的 token
// 返回 nil。只做单条 O(1) 查找（终评 R5 F8：原实现持锁调 sweepExpiredLocked
// 对 5 个 map 全量清扫，而每个 /mcp 请求经 gate 与 contextFunc 触发两次
// ——tokens 表被推大后形成请求串行化 O(n)×2 放大）。全量清扫职责收敛到
// 写路径（/register、/authorize、/token 经 maybeSweepLocked 节流执行）；
// 查到的过期条目仍即时删除并拒绝（OCR T04-R1-2 过期即拒语义不变）。
func (s *oauthServer) lookupSession(token string) *oauthSession {
	if token == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.tokens[token]
	if !ok {
		return nil
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(s.tokens, token)
		return nil
	}
	return entry.Session
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

// handleRegister 是 RFC 7591 动态客户端注册（简化）：要求注册文档携带至少
// 一个绝对 http(s) redirect_uri 并存储绑定（OCR T04-R1-1：authorize 时与
// 注册值精确匹配，杜绝授权码被 302 到任意地址的劫持路径）。
func (s *oauthServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var registration struct {
		RedirectURIs []string `json:"redirect_uris"`
		ClientName   string   `json:"client_name"`
	}
	// 解析失败一律拒绝（跨任务转交 T01-R2-F5）：encoding/json 在类型不匹配
	// 等错误下会保留已成功解码的字段（截断形态则不发生部分 unmarshal），
	// 「err!=nil 且 redirect_uris==nil 才拒」会让部分解码的注册体带着空
	// client_name 入库——与 RFC 7591 严格解析不符。
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&registration); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invalid_client_metadata", "error_description": "request body must be a valid JSON registration document with redirect_uris",
		})
		return
	}
	if len(registration.RedirectURIs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invalid_redirect_uri", "error_description": "at least one absolute http(s) redirect_uri is required",
		})
		return
	}
	// 整分支 OCR 一轮 F7：client_name 封顶——注册体原样入 map，聚合内存
	// 必须有界（与 redirect_uri 的条数/单条封顶同一不变量；字节计）。
	if len(registration.ClientName) > maxClientNameBytes {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invalid_client_metadata", "error_description": "client_name is too long",
		})
		return
	}
	// OCR 二轮 F6：client_name 是同意页的「请求方标识」，渲染只经
	// html.EscapeString（仅覆盖 5 个 ASCII 字符）——Unicode Cf（RLO、
	// isolate、BOM、软连字符）原样穿透，可视觉重排/隐藏伪装可信请求方
	//（深链钓鱼面，/register 缺省无认证）。注册期 fail-closed 拒绝该
	// 字符类；ZWJ/ZWNJ 复合 emoji 按 manifest validateDescription 的双重
	// 标准豁免（显示文本），Co 私用区有可见字形按同先例保留。
	if err := validateClientName(registration.ClientName); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invalid_client_metadata", "error_description": err.Error(),
		})
		return
	}
	// OCR T04-R2-8：条数与单条长度封顶——注册体原样入 map，聚合内存大小
	// 必须有界（4096 客户端 × 16 条 × 2KiB）。
	if len(registration.RedirectURIs) > maxRedirectURIsPerRegistration {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "invalid_redirect_uri", "error_description": "too many redirect_uris",
		})
		return
	}
	for _, uri := range registration.RedirectURIs {
		if len(uri) > maxRedirectURIBytes {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "invalid_redirect_uri", "error_description": "redirect_uri is too long",
			})
			return
		}
		parsed, err := url.Parse(uri)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" ||
			(parsed.Scheme != "http" && parsed.Scheme != "https") {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "invalid_redirect_uri", "error_description": "redirect_uri must be an absolute http(s) URL",
			})
			return
		}
		// 整分支终评 r2-012/r4-008：装配了 AllowedRedirectHosts 时，注册即
		// 拒绝名单外 host——授权码的目的地在注册期收敛，而非到同意页才告警。
		if len(s.allowedRedirectHosts) > 0 {
			if _, ok := s.allowedRedirectHosts[strings.ToLower(parsed.Hostname())]; !ok {
				writeJSON(w, http.StatusBadRequest, map[string]any{
					"error": "invalid_redirect_uri", "error_description": "redirect_uri host is not in the operator allowlist",
				})
				return
			}
		}
	}
	clientID := "jtm-" + randomToken()
	now := time.Now()
	s.mu.Lock()
	// 先清扫再查容量（整分支 OCR 一轮 F5）：过期注册即时释放名额——否则
	// 未认证 /register 填满上限后，合法客户端到重启前都拿不到注册位。
	s.maybeSweepLocked(now)
	if len(s.clients) >= maxRegisteredClients {
		s.mu.Unlock()
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"error": "registration_limit_exceeded",
		})
		return
	}
	s.clients[clientID] = registeredClient{
		ClientName:   registration.ClientName,
		RedirectURIs: registration.RedirectURIs,
		ExpiresAt:    now.Add(clientRegistrationTTL),
	}
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
	client, err := s.validateAuthorizationRequest(req, q.Get("response_type"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	state := q.Get("state")
	if state == "" {
		http.Error(w, "state is required", http.StatusBadRequest)
		return
	}
	if len(state) > maxStateBytes {
		http.Error(w, "state is too long", http.StatusBadRequest)
		return
	}
	now := time.Now()
	s.mu.Lock()
	s.maybeSweepLocked(now)
	// OCR T04-R2-4：对未过期的同 state 条目拒绝覆盖——否则攻击者获知成员
	// state 后可用自己的 client/redirect/PKCE 重新绑定（授权请求固定）。
	// 合法客户端每次授权流生成新 state（OAuth 语义），POST 重试不受影响。
	if _, exists := s.pendingAuths[state]; exists {
		s.mu.Unlock()
		http.Error(w, "state is already in use; start a new authorization request", http.StatusConflict)
		return
	}
	// OCR T04-R2-3：窗口内总量封顶——TTL 清扫只约束时间维度，未认证攻击者
	// 高频 GET 可在窗口内无界填充。
	if len(s.pendingAuths) >= maxPendingAuths {
		s.mu.Unlock()
		http.Error(w, "too many pending authorizations", http.StatusTooManyRequests)
		return
	}
	s.pendingAuths[state] = pendingAuth{Request: req, ExpiresAt: now.Add(pendingAuthTTL)}
	s.mu.Unlock()
	// 同意页透明化（整分支终评 r2-012/r4-008）：开放动态注册意味着任何人
	// 都能成为「请求方」——成员提交凭据前必须看到请求方（注册名，缺失回落
	// client_id）与授权码跳转目的地，才能识别深链诱导的伪造授权请求。
	// client_name 是不可信输入，渲染前一律 html.EscapeString。
	displayName := client.ClientName
	if strings.TrimSpace(displayName) == "" {
		displayName = req.ClientID
	}
	redirectDisplay := ""
	if parsed, err := url.Parse(req.RedirectURI); err == nil {
		redirectDisplay = parsed.Scheme + "://" + parsed.Host
	}
	// 表单把授权参数（含 state）藏在隐藏字段；凭据字段仅在提交瞬间经
	// HTTPS 到达本服务，不进入任何存储。
	setHTMLPageHeaders(w)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="zh-CN"><head><meta charset="utf-8"><title>Jira 授权</title></head>
<body>
<h1>授权访问你的 Jira 账号</h1>
<p>请求方应用：<strong>%s</strong></p>
<p>授权通过后结果将跳转至：<code>%s</code></p>
<p>⚠ 请核对以上请求方与跳转地址；不认识请勿输入凭据。</p>
<p>输入你的 Jira 邮箱与 API token（仅用于本次授权验证，服务不保存凭据）。</p>
<form method="POST" action="/authorize">
<input type="hidden" name="state" value="%s">
<label>邮箱 <input type="email" name="email" required autocomplete="off"></label><br>
<label>API token <input type="password" name="api_token" required autocomplete="off"></label><br>
<button type="submit">授权</button>
</form>
</body></html>`,
		html.EscapeString(displayName), html.EscapeString(redirectDisplay), html.EscapeString(state))
}

// setHTMLPageHeaders 写凭据录入面 HTML 页前的统一响应头：凭据同意页与
// 凭据错误/上游故障页都可能承载成员凭据交互，必须拒绝第三方 iframe 嵌入
// （整分支 OCR 二轮 F6：X-Frame-Options: DENY + CSP frame-ancestors 'none'，
// 双头并存以覆盖新旧浏览器）。
func setHTMLPageHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
}

// validateAuthorizationRequest 校验授权请求：response_type=code、client 已
// 注册、redirect_uri 与该 client 注册值**精确匹配**（RFC 6749 §3.1.2.3 公共
// 客户端语义，OCR T04-R1-1）、PKCE S256。返回注册信息（含 ClientName，供
// 同意页展示请求方——整分支终评 r2-012/r4-008）。
func (s *oauthServer) validateAuthorizationRequest(req authorizationRequest, responseType string) (registeredClient, error) {
	if responseType != "code" {
		return registeredClient{}, fmt.Errorf("response_type must be \"code\"")
	}
	s.mu.Lock()
	// 单条 O(1) 过期判定（OCR 一轮 F8）：读路径不得全量清扫——sweep 会
	// 遍历 5 个 map（达上限 ≈1.8 万条目）且这把锁被 /mcp 每请求两次的
	// lookupSession 依赖，未认证垃圾 client_id 可造成锁 convoy。查到的
	// 过期注册即时删除拒绝（lookupSession 同款语义）；全量清扫职责保留
	// 在写路径（/register、/authorize 写入段、/token）。
	client, registered := s.clients[req.ClientID]
	if registered && time.Now().After(client.ExpiresAt) {
		delete(s.clients, req.ClientID)
		registered = false
	}
	s.mu.Unlock()
	if !registered {
		return registeredClient{}, fmt.Errorf("unknown client_id")
	}
	redirect, err := url.Parse(req.RedirectURI)
	if err != nil || redirect.Scheme == "" || redirect.Host == "" {
		return registeredClient{}, fmt.Errorf("redirect_uri must be an absolute http(s) URL")
	}
	if redirect.Scheme != "http" && redirect.Scheme != "https" {
		return registeredClient{}, fmt.Errorf("redirect_uri must use http or https")
	}
	// 精确匹配注册值：杜绝注册后把授权码发给任意第三方地址的劫持路径。
	registeredURI := false
	for _, uri := range client.RedirectURIs {
		if req.RedirectURI == uri {
			registeredURI = true
			break
		}
	}
	if !registeredURI {
		return registeredClient{}, fmt.Errorf("redirect_uri does not match the registered redirect_uris for this client")
	}
	if req.CodeChallenge == "" {
		return registeredClient{}, fmt.Errorf("code_challenge is required (PKCE)")
	}
	// 跨任务转交 T01-R2-F9：code_challenge 与 state 同为未认证 GET
	// /authorize 的存储键成分（随 pendingAuths 驻留 TTL），必须同样封顶——
	// 否则 1024 × ~1MiB 击穿聚合内存界。RFC 7636 S256 的 challenge 恰为
	// 43 个 base64url 字符，字符集一并收紧。
	if len(req.CodeChallenge) > maxCodeChallengeBytes {
		return registeredClient{}, fmt.Errorf("code_challenge is too long")
	}
	if !isBase64URL(req.CodeChallenge) {
		return registeredClient{}, fmt.Errorf("code_challenge must contain base64url characters only")
	}
	if req.CodeChallengeMethod != "S256" {
		return registeredClient{}, fmt.Errorf("code_challenge_method must be S256")
	}
	return client, nil
}

// isBase64URL 报告 s 是否只含 base64url 字母表字符（RFC 4648 §5：A-Z、
// a-z、0-9、-、_，无填充）——PKCE S256 challenge 的规范字符集。
func isBase64URL(s string) bool {
	for _, c := range s {
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

// submitAuthorizeForm 处理凭据提交：凭据经 Jira /rest/api/3/myself 验证成功
// 才消费 state 并发码（OCR T04-R1-3：凭据输错后同 state 可重试，避免成员被
// 迫重启整条授权流）。凭据性失败（401/403）渲染凭据错误页；上游故障渲染
// 502 服务不可用页（OCR T04-R1-10：不得把 Jira 故障伪装成凭据错误）。
//
// 已知限制（OCR T04-R2-9）：本端点无速率限制/失败锁定，可被用作成员 Jira
// 凭据的在线猜测代理与出站请求放大器——教学级实现，生产部署必须置于
// 限流/防护之后（见 README）。
func (s *oauthServer) submitAuthorizeForm(w http.ResponseWriter, r *http.Request) {
	// 整分支终评 r3-006/r4-007：表单体必须封顶后再解析（未认证输入面）。
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBodyBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	state := r.PostFormValue("state")
	email := strings.TrimSpace(r.PostFormValue("email"))
	apiToken := r.PostFormValue("api_token")
	now := time.Now()
	s.mu.Lock()
	pending, ok := s.pendingAuths[state]
	if ok && now.After(pending.ExpiresAt) {
		delete(s.pendingAuths, state)
		ok = false
	}
	s.mu.Unlock()
	if !ok {
		http.Error(w, "unknown or expired state", http.StatusBadRequest)
		return
	}
	// 凭据只进内存变量；Myself 失败信息不含凭据本身。
	client := &JiraClient{BaseURL: s.jiraBaseURL, Email: email, APIToken: apiToken}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if _, err := client.Myself(ctx); err != nil {
		var httpErr *jiraHTTPError
		credentialFailure := errors.As(err, &httpErr) &&
			(httpErr.StatusCode == http.StatusUnauthorized || httpErr.StatusCode == http.StatusForbidden)
		setHTMLPageHeaders(w)
		if credentialFailure {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `<!DOCTYPE html>
<html lang="zh-CN"><head><meta charset="utf-8"><title>授权失败</title></head>
<body><h1>授权失败</h1><p>Jira 凭据验证未通过。请返回重试。</p></body></html>`)
			return
		}
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprint(w, `<!DOCTYPE html>
<html lang="zh-CN"><head><meta charset="utf-8"><title>Jira 服务不可用</title></head>
<body><h1>Jira 服务暂不可用</h1><p>授权验证依赖的 Jira 服务当前不可达或出错，请稍后重试或联系部署管理员。</p></body></html>`)
		return
	}
	// 验证成功：此刻才一次性消费 state 并签发授权码。消费必须原子——上面的
	// 存在性检查与这里的消费之间隔着一次跨网络调用（Myself，最长 30s），
	// 并发同 state 双提交会双双通过检查（整分支终评 r2-004/r3-008/r4-009）：
	// 复查存在性并消费放进同一临界区，输者 409，保证一个 state 恰发一个码。
	// 过期判定与发码 TTL 都必须用消费时刻的时钟（整分支 OCR 二轮 F3）——
	// 复用进入前的旧 now 会放行调用期间跨过 TTL 边界的 state，且授权码
	// ExpiresAt 以回溯时间签发（实际存活期短于声明）。
	code := randomToken()
	s.mu.Lock()
	now = time.Now()
	s.maybeSweepLocked(now)
	// OCR 一轮 F9：先清扫再查容量（过期 code 即时释放名额）。满容不消费
	// state——容量是暂时态，成员可稍后重试提交而非重启授权流（与
	// exchangeCode 满容语义一致）。
	if len(s.codes) >= maxActiveCodes {
		s.mu.Unlock()
		http.Error(w, "too many pending codes; retry later", http.StatusTooManyRequests)
		return
	}
	consumed, still := s.pendingAuths[state]
	if still && now.After(consumed.ExpiresAt) {
		delete(s.pendingAuths, state)
		still = false
	}
	if still {
		delete(s.pendingAuths, state)
		s.codes[code] = issuedCode{
			Session:   oauthSession{Email: email, APIToken: apiToken},
			Request:   consumed.Request,
			ExpiresAt: now.Add(authCodeTTL),
		}
	}
	s.mu.Unlock()
	if !still {
		http.Error(w, "state is already consumed or expired; start a new authorization request", http.StatusConflict)
		return
	}
	pending = consumed
	redirect, err := url.Parse(pending.Request.RedirectURI)
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
	// 整分支终评 r3-007/r4-006：表单体必须封顶后再解析（未认证输入面）。
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBodyBytes)
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
	now := time.Now()
	s.mu.Lock()
	s.maybeSweepLocked(now)
	// 终评 R5 F8 + 跨任务转交 R7（T01-R4-F1 补充）：先清扫再查容量（过期
	// 条目即时释放名额）。tokens 与 refreshTokens 任一满容即 429——满容时
	// 不消费 code，容量是暂时态，客户端可稍后重试而非重启整条授权流。检查
	// 与后续写入跨临界区的并发窗口仅允许临时超出并发请求数条，稳态上界
	// = 上限 + 并发数，非无界（refresh 轮换对 refreshTokens 1 删 1 增，不受
	// 该上限影响）。
	if len(s.tokens) >= maxActiveTokens || len(s.refreshTokens) >= maxRefreshTokens {
		s.mu.Unlock()
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"error": "temporarily_unavailable", "error_description": "too many active tokens; retry later",
		})
		return
	}
	issued, ok := s.codes[code]
	delete(s.codes, code) // code 一次性：无论后续校验成败都不再可用
	s.mu.Unlock()
	if !ok || now.After(issued.ExpiresAt) {
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
	s.tokens[accessToken] = tokenEntry{Session: &session, ExpiresAt: now.Add(accessTokenTTL)}
	s.refreshTokens[refreshToken] = refreshEntry{Session: &session, ExpiresAt: now.Add(refreshTokenTTL)}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    int(accessTokenTTL.Seconds()),
		"refresh_token": refreshToken,
		"scope":         "jira:read",
	})
}

// refresh 用 refresh_token 换新 access_token（授权会话保持不变）。
// refreshEntry 直接持有会话与独立 TTL（OCR T04-R2-1：access_token 自然
// 过期后刷新必须成功——这是 WeKnora 客户端 401 后的标准续期路径）；
// 先校验再消费，只有校验通过才轮换旧 refresh_token（失败不烧毁有效条目）。
// 旧 access_token 不立即烧毁——保留到自然过期（OCR T04-R1-2 宽限语义）。
func (s *oauthServer) refresh(w http.ResponseWriter, r *http.Request) {
	refreshToken := r.PostFormValue("refresh_token")
	now := time.Now()
	newAccess := "at-" + randomToken()
	newRefresh := "rt-" + randomToken()
	s.mu.Lock()
	s.maybeSweepLocked(now)
	// 终评 R5 F8：满容时保留调用方现有 refresh_token（与「失败不烧毁有效
	// 条目」语义一致），429 后稍后重试即可。
	if len(s.tokens) >= maxActiveTokens {
		s.mu.Unlock()
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"error": "temporarily_unavailable", "error_description": "too many active tokens; retry later",
		})
		return
	}
	entry, ok := s.refreshTokens[refreshToken]
	valid := ok && !now.After(entry.ExpiresAt)
	if valid {
		// 轮换：旧 refresh_token 一次性消费，签发新对（会话不变）。
		delete(s.refreshTokens, refreshToken)
		s.tokens[newAccess] = tokenEntry{Session: entry.Session, ExpiresAt: now.Add(accessTokenTTL)}
		s.refreshTokens[newRefresh] = refreshEntry{Session: entry.Session, ExpiresAt: now.Add(refreshTokenTTL)}
	}
	s.mu.Unlock()
	if !valid {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant", "error_description": "unknown or expired refresh_token"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  newAccess,
		"token_type":    "Bearer",
		"expires_in":    int(accessTokenTTL.Seconds()),
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
