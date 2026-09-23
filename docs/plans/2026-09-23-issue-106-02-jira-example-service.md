# 插件 02｜提供只读 Jira MCP 示例服务 实施计划（Issue #109）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付可独立部署的远程 MCP 示例插件服务：提供"查询本人本周 Jira 待办"只读工具；工具 schema 固定、不接受模型传入令牌/用户 ID/JQL/URL；按成员授权映射 Jira 账号；返回事项标识、标题、状态、截止日期与来源链接；配套 `weknora.plugin/1` 清单与自托管部署文档。

**Architecture:** Go 独立 main 包 `examples/plugins/jira-todo-mcp`（与仓库 go module 同根，进 `go test ./...`）：`main.go` 装配 streamable-HTTP MCP 端点 + 简化 OAuth 2.0 授权码服务器（`oauth.go`：`/authorize`、`/token`、`/register`、RFC 9728 `/.well-known/oauth-protected-resource` 与 `/.well-known/oauth-authorization-server`、WWW-Authenticate 401）；`jira.go` 用固定 JQL 模板调 Jira REST（base URL 从 `PLUGIN_JIRA_BASE_URL` 环境变量读取）。行为契约——**MCP 目录公开、执行鉴权**（未认证 `ListTools` 可用；`CallTool` 校验 Bearer token），这是 #108 安装前核验能 `ListTools` 的前提，也是 T13 受控替身必须复刻的契约。

**Tech Stack:** Go 1.26；`mark3labs/mcp-go` server（`sdkserver.NewMCPServer` + `NewStreamableHTTPServer`，与测试基建同栈）；标准库 `net/http`（OAuth 端点手写，无第三方 OAuth 依赖）；Jira REST v3（`/rest/api/3/myself`、`/rest/api/3/search/jql`）。

**Spec:** `docs/specs/2026-09-23-self-hosted-plugins-spec.md`（User Stories 1-6、13、17、20-21；Implementation Decisions 56 行）；设计文档「首个纵向案例」。范围裁决 R10：本周范围由服务端工具语义锁定。

**Issue:** https://github.com/1123786563/WeKnora-fork01/issues/109 （无 blocked_by）

**边界（ADR-0001）:** 示例服务独立部署，不使用 open-connector 共享运行时、不持有其管理凭据；Jira 凭据由成员在授权页运行时输入或经环境变量注入插件服务自身配置，源码/测试不写可用凭据字面量。

## Global Constraints

见总索引。本切片特别注意：测试与示例中的 Jira 一律使用内嵌 fake（`httptest`），不访问真实 Atlassian；`PLUGIN_JIRA_BASE_URL`、`PLUGIN_JIRA_API_TOKEN`、`PLUGIN_JIRA_ACCOUNT_EMAIL`（部署自测用）只从环境变量读取。

## Review Focus（本切片）

- 模型传参攻击：调用工具时携带 `token`/`user_id`/`jql`/`url` 等多余参数 → schema `additionalProperties:false` 拒绝，且工具逻辑不读任何账号性参数（T05）。
- 无 token 调用 / 失效 token → 401 且不发生 Jira 请求（T05）。
- 两成员 token 隔离：A 的 token 只查到 A 的事项（T05）。
- Jira 403/超时/失效 → 明确错误不虚构空结果（T05）。
- 空结果返回空数组而非错误（T05）。

---

### Task 4: 示例插件服务本体

**Files:**
- Create: `examples/plugins/jira-todo-mcp/main.go`
- Create: `examples/plugins/jira-todo-mcp/oauth.go`
- Create: `examples/plugins/jira-todo-mcp/jira.go`
- Create: `examples/plugins/jira-todo-mcp/manifest.json`
- Create: `examples/plugins/jira-todo-mcp/README.md`
- Test: `examples/plugins/jira-todo-mcp/server_test.go`（本任务先建骨架编译，T05 补全场景）

**Interfaces:**
- Consumes: `github.com/mark3labs/mcp-go/server`（sdkserver，先例 manager_startup_test.go:21-91）、`github.com/mark3labs/mcp-go/mcp`（sdkmcp）。
- Produces（T13 受控替身与部署文档依赖的行为契约，全部落在代码常量/结构上）:
  - 工具名 `search_my_week_issues`；输入 schema 恒为 `{"type":"object","properties":{},"additionalProperties":false}`；输出文本块数组，每行 `[{key}] {summary} · 状态 {status} · 截止 {due} · {url}`（字段齐备时；缺失字段省略对应段）。
  - OAuth 端点集：`GET /.well-known/oauth-protected-resource`（`{"authorization_servers": ["<base>"]}`）、`GET /.well-known/oauth-authorization-server`（`authorization_endpoint`/`token_endpoint`/`registration_endpoint`/`grant_types: ["authorization_code","refresh_token"]`）、`POST /register`（动态客户端注册，返回 `client_id`）、`GET /authorize`（HTML 表单：Jira 邮箱 + API token → 验证 `myself` → 302 `redirect_uri?code=...&state=...`，PKCE `S256` 校验）、`POST /token`（`grant_type=authorization_code` 换 access token；`refresh_token` 续期）。
  - MCP 端点 `/mcp`：未认证 `initialize`/`ListTools` 放行；`CallTool` 无有效 Bearer → 401 + `WWW-Authenticate: Bearer resource_metadata="<base>/.well-known/oauth-protected-resource"`（触发 WeKnora 客户端 OAuth 流，client.go:119-136 语义）。
  - `manifest.go` 内嵌 `Manifest()`：`tools[0].input_schema_digest = sha256(无参 canonical schema)`——**源码中不写死 digest 字面量**，由 `manifest.Calculate()` 在构建期计算并生成 `/manifest.json` 路由（服务同时自托管清单，部署者可直接引用该 URL）。
  - `func Run(ctx context.Context, opts Options) error`（`Options{BaseURL string; JiraBaseURL string; ListenAddr string}`——测试用它注入 fake Jira 与 httptest 监听）。
  - JQL 模板（服务端固定）：`assignee = currentUser() AND resolution = Unresolved AND due >= startOfWeek() ORDER BY due ASC`。

- [ ] **Step 1: 写失败测试（骨架契约：目录公开 + schema 固定 + 401）**

`examples/plugins/jira-todo-mcp/server_test.go`：

```go
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/utils"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/stretchr/testify/require"
)

// newTestService 启动完整示例服务（OAuth + MCP + 自托管清单），Jira 指向 fake。
func newTestService(t *testing.T, jira http.HandlerFunc) (baseURL string, jiraCalls *int32) { /* httptest 组装 Run() */ }

func TestListToolsIsPublicAndSchemaIsFixed(t *testing.T) {
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	t.Cleanup(utils.ResetSSRFWhitelistForTest)
	base, _ := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("ListTools must not reach Jira")
	})
	c := client.NewMCPClient("preview", transport.NewStreamableHttpClient(base+"/mcp", transport.WithHTTPHeaders(nil)))
	require.NoError(t, c.Start(context.Background()))
	defer func() { _ = c.Close() }()
	initRes, err := c.Initialize(context.Background(), mcp.InitializeRequest{})
	require.NoError(t, err)
	require.Equal(t, "jira-todo-mcp", initRes.ServerInfo.Name)
	tools, err := c.ListTools(context.Background(), mcp.ListToolsRequest{})
	require.NoError(t, err)
	require.Len(t, tools.Tools, 1)
	require.Equal(t, "search_my_week_issues", tools.Tools[0].Name)
	require.JSONEq(t, `{"type":"object","properties":{},"additionalProperties":false}`, string(tools.Tools[0].InputSchema.Raw()))
}

func TestCallToolWithoutTokenReturns401(t *testing.T) {
	// 无 Bearer 调用 search_my_week_issues → 错误/401，且 fake Jira 调用计数为 0。
}

func TestSelfHostedManifestMatchesContract(t *testing.T) {
	// GET base+"/manifest.json" → 解析 types.PluginManifest → ValidateManifest 通过
	// 且 tools[0].input_schema_digest == ToolSchemaDigest(无参 canonical schema)。
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./examples/plugins/jira-todo-mcp/ -v`
Expected: FAIL —— 包不存在（编译错误）。

- [ ] **Step 3: 实现服务本体**

`main.go`：`Options`/`Run`（mux 挂 `/mcp`、OAuth 端点、`/manifest.json`；`PLUGIN_JIRA_BASE_URL` 缺失时启动即报错退出——fail-closed，不猜测默认）；`jira.go`：`type JiraClient struct{ BaseURL string; Token, Email string }`（`SearchMyWeek(ctx)` 发固定 JQL，解析 `issues[].{key,fields.summary,fields.status.name,fields.duedate}`，构造 `url = BaseURL + "/browse/" + key`；HTTP 状态 ≥400 → 带状态码的错误；`context.WithTimeout` 30s）；`oauth.go`：内存会话/码本（`map[string]oauthSession`，code 一次性、state 一次性、token→会话映射；PKCE `S256` 验证；授权表单 POST 的邮箱+token 先经 `JiraClient.Myself` 验证成功才发码——凭据不落盘、不写日志）；MCP 服务器 handler：`sdkserver.NewMCPServer("jira-todo-mcp", ...)` 注册 `search_my_week_issues`（handler 内从请求上下文取已验证会话——sdkserver 工具 handler 不透传 HTTP 头时，改用 `NewStreamableHTTPServer(server, WithStateLess(false))` + `WithToolHandlerMiddleware` 从 `r.Header` 提取 Bearer 注入 context；执行时以该会话构造 JiraClient）。`manifest.json` 构建产物由 `/manifest.json` 路由从 `Manifest()` 动态序列化（digest 由 `plugins.ToolSchemaDigest` 计算——examples 包可以 import `github.com/Tencent/WeKnora/internal/modules/plugins`，同 module `internal/` 规则允许根模块内引用）。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./examples/plugins/jira-todo-mcp/ -run 'TestListTools|TestCallToolWithoutToken|TestSelfHostedManifest' -v`
Expected: PASS。

- [ ] **Step 5: 写 README 与部署说明**

`README.md`：自托管步骤（环境变量表：`PLUGIN_JIRA_BASE_URL` 必填、`PLUGIN_LISTEN_ADDR` 默认 `:8020`；Jira API token 创建指引链接；清单 URL 即 `https://<host>/manifest.json`）、开发者约定（版本端点稳定：不同版本部署在不同路径或实例、旧版保持可用）、信任边界声明（设计文档：平台核验可观察目录/schema，不能锁住远端内部代码）。

- [ ] **Step 6: Commit**

```bash
git add examples/plugins/jira-todo-mcp/
git commit -m "feat(plugins): 只读 Jira 待办示例 MCP 服务（目录公开/执行鉴权/OAuth/PKCE）[T04]"
```

---

### Task 5: 示例服务自测（fake Jira 全场景）

**Files:**
- Modify: `examples/plugins/jira-todo-mcp/server_test.go`（补全场景测试）
- Test（同一文件内）：OAuth 全流程 + 隔离 + 403/超时/失效/空结果 + 多余参数拒绝

**Interfaces:**
- Consumes: T04 的 `Run(Options)`、OAuth 端点契约、`JiraClient`。
- Produces: 行为契约的证据测试（T13 替身必须复刻同样断言集）。

- [ ] **Step 1: 写失败测试（补全场景）**

在 `server_test.go` 追加（fake Jira 按成员凭据返回不同事项集）：

```go
func TestOAuthCodeFlowIssuesMemberScopedToken(t *testing.T) {
	// 1) POST /register → client_id。
	// 2) GET /authorize?...&code_challenge=S256 → 表单 POST 成员 A 凭据（fake Jira 接受）→ 302 带 code。
	// 3) POST /token（code + code_verifier）→ access_token。
	// 4) 断言：错误凭据（fake Jira 401）→ /authorize 返回错误页且无 code。
	// 5) 断言：code 二次使用 → /token 拒绝。
}

func TestTwoMembersIsolated(t *testing.T) {
	// 成员 A、B 各自走 OAuth；用各自 token 调 search_my_week_issues：
	// fake Jira 按 Authorization 区分返回 A={A-1}、B={B-1}。
	// 断言 A 的结果只含 A-1 且带 <base>/browse/A-1 链接；B 同理。
}

func TestJiraErrorsSurfaceWithoutFabrication(t *testing.T) {
	// fake Jira 403 → 工具结果错误含 "403"；超时（handler sleep > 客户端超时）→ 错误含 "timeout"；
	// 401（token 失效）→ 错误含 "401"；三者都不得返回空成功列表。
}

func TestEmptyWeekIsEmptySuccess(t *testing.T) {
	// fake Jira 返回 {"issues":[]} → 工具成功且输出空。
}

func TestToolRejectsExtraneousArguments(t *testing.T) {
	// 带 {"token":"x","jql":"...","url":"http://evil"} 参数调用 → 服务端校验失败（additionalProperties），
	// 且 fake Jira 计数为 0。
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./examples/plugins/jira-todo-mcp/ -run 'TestOAuthCodeFlow|TestTwoMembersIsolated|TestJiraErrors|TestEmptyWeek|TestToolRejects' -v`
Expected: FAIL —— 场景未全部实现（按 T04 完成度，至少 403/超时/多余参数路径缺实现）。

- [ ] **Step 3: 补齐实现**（错误透传、超时上下文、参数严格校验、空结果路径；不虚构数据）。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./examples/plugins/jira-todo-mcp/ -v && go build ./...`
Expected: PASS + build 成功。

- [ ] **Step 5: Commit**

```bash
git add examples/plugins/jira-todo-mcp/server_test.go
git commit -m "test(plugins): Jira 示例服务全场景自测（隔离/403/超时/失效/空结果/拒多余参数）[T05]"
```
