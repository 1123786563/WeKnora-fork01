// Command jira-todo-mcp 是 WeKnora 自托管插件体系的只读示例服务：
// 提供单一工具 search_my_week_issues（查询本人本周 Jira 待办）。
//
// 行为契约（#108 安装前核验与 T13 受控替身都依赖）：
//   - MCP 端点 /mcp 目录公开、执行鉴权——未认证 initialize/ListTools 放行，
//     未认证 CallTool 返回 401 + WWW-Authenticate resource_metadata（RFC 9728）；
//   - 工具 schema 恒为 {"type":"object","properties":{},"additionalProperties":false}，
//     模型无法传入 token/user_id/jql/url 等任何参数；
//   - JQL 由服务端固定模板构造，绝不接受输入拼接；
//   - OAuth 2.0 授权码 + PKCE S256 + 动态客户端注册（oauth.go）；
//   - /manifest.json 由 Manifest() 动态序列化自托管（digest 由代码计算）。
//
// 边界（ADR-0001）：独立部署，不使用 open-connector 运行时、不持有其管理
// 凭据；成员 Jira 凭据仅在授权页运行时输入并驻留内存，不落盘、不写日志。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/plugins"
	"github.com/Tencent/WeKnora/internal/types"
	mcp "github.com/mark3labs/mcp-go/mcp"
	sdkserver "github.com/mark3labs/mcp-go/server"
)

const (
	// serverName 同时是 MCP initialize 响应中的 ServerInfo.Name 与 plugin_id。
	serverName = "jira-todo-mcp"
	// pluginVersion 是本示例服务的语义化版本（manifest.version）。
	pluginVersion = "1.0.0"
	// toolName 是本服务唯一工具名。
	toolName = "search_my_week_issues"
	// toolDescription 描述工具用途；不含任何凭据。
	toolDescription = "查询当前授权成员本周内（due >= startOfWeek() 且 due < startOfWeek(\"+1w\")）未解决的 Jira 待办事项，" +
		"返回事项标识、标题、状态、截止日期与来源链接。工具不接受任何参数；" +
		"JQL 由服务端固定，成员身份来自个人 OAuth 授权。"
	// canonicalToolInputSchema 是工具唯一合法输入 schema：无参数、拒绝一切
	// 额外属性。任何 token/jql/url/user_id 之类的模型传参都会被
	// additionalProperties:false 拒绝。manifest 的 input_schema_digest
	// 由 plugins.ToolSchemaDigest 对该常量计算，源码不写死 digest 字面量。
	canonicalToolInputSchema = `{"type":"object","properties":{},"additionalProperties":false}`

	// defaultListenAddr 是 PLUGIN_LISTEN_ADDR 缺省监听地址。
	defaultListenAddr = ":8020"

	// maxRPCBodyBytes 限制单个 JSON-RPC 请求体大小（鉴权 gate 需要读 body
	// 判断 method，读取必须有边界）。
	maxRPCBodyBytes = 1 << 20
)

// Options 是 Run/NewHandler 的装配参数。测试用 NewHandler 注入 fake Jira
// 与 httptest 监听；main() 从环境变量填充。
type Options struct {
	// BaseURL 是本服务对外可达的 base URL（含 scheme，无尾斜杠），用于
	// OAuth metadata、WWW-Authenticate resource_metadata 与 manifest 端点。
	BaseURL string
	// JiraBaseURL 是 Jira REST API 的 base URL（如 https://xxx.atlassian.net）。
	// 必填——缺失时启动 fail-closed，不猜测默认值。
	JiraBaseURL string
	// ListenAddr 是监听地址；Run() 中为空时使用 defaultListenAddr。
	ListenAddr string
	// AllowedRedirectHosts 非空时，/register 仅接受 host 在名单内的
	// redirect_uri（整分支终评 r2-012/r4-008：开放动态注册下，任何人都能
	// 注册 client 并深链诱导成员提交 Jira 凭据；生产部署设置本名单即把
	// 授权码的目的地收敛到运营者认可的主机）。缺省空 = 不限制（教学示例
	// 语义；同意页仍会展示请求方与跳转目的地）。
	AllowedRedirectHosts []string
}

// Run 启动示例服务并阻塞直到 ctx 取消。校验失败（如 JiraBaseURL 缺失）
// 返回错误——fail-closed。
func Run(ctx context.Context, opts Options) error {
	handler, err := NewHandler(opts)
	if err != nil {
		return err
	}
	listenAddr := opts.ListenAddr
	if listenAddr == "" {
		listenAddr = defaultListenAddr
	}
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", listenAddr, err)
	}
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ln) }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-done:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

// validateBaseURL 校验一个配置方提供的 URL：仅 http/https 且带非空
// hostname（"http://:8020" 这类 host 为空的 URL 会通过 u.Host!="" 却把
// 元数据指向不可达地址——OCR T04-R1-7）。它不是 SSRF 判定（本服务出站
// 请求统一走 SSRF-safe client），只是装配期的 fail-closed 结构校验。
func validateBaseURL(where, raw string) error {
	if strings.TrimSpace(raw) == "" {
		return fmt.Errorf("%s is required (fail-closed): got empty", where)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s %q: %w", where, raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%s must use http or https, got %q", where, u.Scheme)
	}
	if u.Hostname() == "" {
		return fmt.Errorf("%s must include a host, got %q", where, raw)
	}
	return nil
}

// parseHostList 解析逗号分隔的 host 名单（PLUGIN_ALLOWED_REDIRECT_HOSTS）：
// 逐项去空白并小写化；空串或全空白 → nil（不限制）。名单项不含 scheme/path
// ——与 /register 的 redirect_uri host 匹配（小写主机名比较）。
func parseHostList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	hosts := make([]string, 0, len(parts))
	for _, part := range parts {
		if host := strings.ToLower(strings.TrimSpace(part)); host != "" {
			hosts = append(hosts, host)
		}
	}
	return hosts
}

// NewHandler 校验 Options 并组装完整 HTTP handler：/mcp（MCP 端点）、
// OAuth 端点集与 /manifest.json。
func NewHandler(opts Options) (http.Handler, error) {
	if err := validateBaseURL("BaseURL", opts.BaseURL); err != nil {
		return nil, err
	}
	// fail-closed：PLUGIN_JIRA_BASE_URL 缺失时拒绝启动，不猜测默认值。
	if err := validateBaseURL("JiraBaseURL", opts.JiraBaseURL); err != nil {
		return nil, err
	}
	oauthSrv := newOAuthServer(opts.BaseURL, opts.JiraBaseURL, opts.AllowedRedirectHosts)

	mcpServer := sdkserver.NewMCPServer(serverName, pluginVersion)
	tool := mcp.NewToolWithRawSchema(toolName, toolDescription, json.RawMessage(canonicalToolInputSchema))
	mcpServer.AddTool(tool, func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleSearchMyWeek(ctx, opts.JiraBaseURL)
	})

	streamable := sdkserver.NewStreamableHTTPServer(mcpServer,
		sdkserver.WithStateLess(false),
		// 从每个入站 HTTP 请求提取 Bearer 并解析为已授权会话，注入工具
		// handler 的 context（SDK 工具 handler 本身不透传 HTTP 头）。
		sdkserver.WithHTTPContextFunc(oauthSrv.contextFunc),
	)

	mux := http.NewServeMux()
	// MCP 端点：目录公开、执行鉴权。
	mux.Handle("/mcp", gateCallToolAuth(streamable, oauthSrv, opts.BaseURL))
	mux.HandleFunc("/.well-known/oauth-protected-resource", oauthSrv.handleProtectedResource)
	mux.HandleFunc("/.well-known/oauth-authorization-server", oauthSrv.handleAuthorizationServer)
	mux.HandleFunc("/register", oauthSrv.handleRegister)
	mux.HandleFunc("/authorize", oauthSrv.handleAuthorize)
	mux.HandleFunc("/token", oauthSrv.handleToken)
	// 自托管清单：每次请求动态序列化 Manifest()（digest 由代码计算）。
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Manifest(opts.BaseURL))
	})
	return mux, nil
}

// gateCallToolAuth 实现「目录公开、执行鉴权」：POST 到 /mcp 的 JSON-RPC
// 若是 tools/call，必须携带映射到已授权会话的 Bearer token，否则返回
// 401 + WWW-Authenticate: Bearer resource_metadata="<base>/.well-known/oauth-protected-resource"
// （RFC 9728；WeKnora 客户端据此触发 OAuth 流）。initialize/ListTools 等
// 目录操作未认证放行。非 POST 与非 tools/call 的 POST 原样透传。
func gateCallToolAuth(next http.Handler, oauthSrv *oauthServer, baseURL string) http.Handler {
	metadataURL := baseURL + "/.well-known/oauth-protected-resource"
	challenge := fmt.Sprintf(`Bearer resource_metadata=%q`, metadataURL)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
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
		// fail-closed（OCR T04-R2-6）：本端点只接受单个 JSON-RPC 对象；
		// 解析失败（批量数组、拼接文档等）一律 400，绝不把鉴权决策建立在
		// 「自行解析失败即放行」上——否则 SDK 未来一旦接受多消息形态，
		// 其中的 tools/call 将绕过 401+WWW-Authenticate challenge。
		if err := json.Unmarshal(body, &rpc); err != nil {
			http.Error(w, "request body must be a single JSON-RPC object", http.StatusBadRequest)
			return
		}
		if rpc.Method == "tools/call" {
			if !oauthSrv.hasValidSession(bearerToken(r.Header.Get("Authorization"))) {
				w.Header().Set("WWW-Authenticate", challenge)
				http.Error(w, "unauthorized: this MCP tool call requires a personal OAuth bearer token", http.StatusUnauthorized)
				return
			}
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

// bearerToken 从 Authorization 头提取 Bearer 凭据；非 Bearer 方案返回空。
func bearerToken(header string) string {
	scheme, value, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(value)
}

func main() {
	opts := Options{
		BaseURL:              strings.TrimRight(os.Getenv("PLUGIN_BASE_URL"), "/"),
		JiraBaseURL:          strings.TrimRight(os.Getenv("PLUGIN_JIRA_BASE_URL"), "/"),
		ListenAddr:           os.Getenv("PLUGIN_LISTEN_ADDR"),
		AllowedRedirectHosts: parseHostList(os.Getenv("PLUGIN_ALLOWED_REDIRECT_HOSTS")),
	}
	// PLUGIN_BASE_URL 缺省用监听地址拼一个 base（仅当监听地址含具体 host
	// 如 127.0.0.1:8020 时可得到合法 URL；通配 ":8020" 拼出的
	// "http://:8020" 会被 validateBaseURL 拒绝——fail-closed，部署者必须
	// 显式设置 PLUGIN_BASE_URL）。
	if opts.BaseURL == "" {
		listenAddr := opts.ListenAddr
		if listenAddr == "" {
			listenAddr = defaultListenAddr
		}
		opts.BaseURL = "http://" + listenAddr
	}
	if err := Run(context.Background(), opts); err != nil {
		log.Fatalf("jira-todo-mcp: %v", err)
	}
}

// Manifest 构造本服务的 weknora.plugin/1 清单。input_schema_digest 与
// content_digest 全部由 plugins 包在运行时计算——源码不写死 digest 字面量；
// manifest.json 参考副本的 digest 值由同一函数生成，并有测试
// （TestSelfHostedManifestMatchesContract）断言副本与代码计算值一致以防
// 漂移；/manifest.json 路由每次请求动态序列化本函数结果（部署后以它为权威）。
func Manifest(baseURL string) types.PluginManifest {
	m := types.PluginManifest{
		Protocol:    plugins.PluginProtocolV1,
		PluginID:    serverName,
		Version:     pluginVersion,
		Name:        "Jira 本周待办（只读示例）",
		Description: "查询授权成员本周未解决的 Jira 待办事项；只读、固定 JQL、不接受模型传参。",
		Transport: types.PluginTransport{
			Type:     "http-streamable",
			Endpoint: baseURL + "/mcp",
		},
		Auth: &types.PluginAuth{PersonalOAuth: true},
		Tools: []types.PluginToolDecl{{
			Name:                 toolName,
			ReadOnly:             true,
			RequiresPersonalAuth: true,
			InputSchemaDigest:    plugins.ToolSchemaDigest([]byte(canonicalToolInputSchema)),
		}},
	}
	m.ContentDigest = plugins.ManifestContentDigest(&m)
	return m
}
