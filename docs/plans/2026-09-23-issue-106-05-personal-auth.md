# 插件 05｜成员个人授权与撤销 实施计划（Issue #112）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 成员从已安装插件入口查看本人授权状态，完成个人 OAuth 授权，可撤销、可重授权；两名成员凭据不混用；跨空间不可读；未授权/过期时工具不可用；撤销不影响其他成员。

**Architecture:** GAP-4 裁决（总索引）：个人连接 = 既有 per-principal MCP OAuth（token 存 `mcp_oauth_tokens(tenant, principal, service_id)`，`service_id` 为插件物化服务 ID）。本切片新增聚合视图 API `GET /api/v1/plugins/installations/:id/connections/me`（Viewer+）复用 `mcpManager`/`oauthManager` 的 `IsAuthorized`/状态查询，把授权动作（`POST /mcp-services/:service_id/oauth/authorize-url`）与撤销（`DELETE /mcp-services/:service_id/oauth/token`）映射到插件安装语义（响应体内带 `service_id` 供前端直调既有端点）；不改 token 存储、不新建表。

**Tech Stack:** Go 1.26；`interfaces.MCPOAuthService`（AuthorizeURL/Status/Revoke，handler mcp_oauth.go）；gin；真 PG 集成测试（`PLUGIN_TEST_DATABASE_URL` Fatal 模式 + AutoMigrate 冻结模型，先例 oc_integration_test.go:408-455）。

**Spec:** `docs/specs/2026-09-23-self-hosted-plugins-spec.md`（User Stories 17/18/19/26；Implementation Decisions 53 行）；设计文档差距表「成员个人连接」行；范围裁决 GAP-4。

**Issue:** https://github.com/1123786563/WeKnora-fork01/issues/112 （blocked_by #110 → 任务级前置 T06）

## Global Constraints

见总索引。本切片不新增迁移；OAuth 凭据路径全走既有 AES 加密存储与 DTO 脱敏（types/mcp.go:75-107 注释约束）。

## Review Focus（本切片）

- 成员 B 查询成员 A 的授权状态：API 无此入参（仅 `me`），越权面由 `principal` 来自会话上下文保证——集成测试用两名 principal 分别调用断言互不影响（T11）。
- 撤销后其他成员 token 仍在（T11）。
- 跨空间：空间 2 成员查空间 1 安装的 `connections/me` → not found（T11）。
- 未授权时调用需授权工具 → `OAuthRequiredError` 引导（T11）。
- token 过期（`ExpiresAt` 过去 + 无 refresh）→ 状态 `expired`，工具调用引导重授权（T11）。

---

### Task 11: connections/me API、授权映射与隔离测试

**Files:**
- Modify: `internal/types/interfaces/plugin.go`（`PluginService` 扩展）
- Modify: `internal/application/service/plugin_install_service.go`（新增 `GetMyConnectionStatus`）
- Modify: `internal/handler/dto/plugin.go`、`internal/handler/plugin.go`、`internal/router/routes_plugins.go`
- Test: `internal/modules/plugins/connection_status_test.go`（fake 层）
- Test: `internal/modules/plugins/plugin_pg_integration_test.go`（新建，`//go:build integration`，真 PG：`PLUGIN_TEST_DATABASE_URL`）

**Interfaces:**
- Consumes: T07 的 `GetInstallation`；`mcp.MCPManager`；`interfaces.MCPOAuthRepository.GetTokenForPrincipal(ctx, tenantID, principal, serviceID)`（mcp_oauth.go:70-80）；`types.MCPOAuthToken`（`AccessToken`/`ExpiresAt`/`RefreshToken` 等，types/mcp_oauth.go）。
- Produces:
  - `interfaces.PluginService.GetMyConnectionStatus(ctx context.Context, tenantID uint64, installationID string, principal types.Principal) (*dto.PluginMyConnection, error)`
  - `dto.PluginMyConnection{InstallationID, PluginID, Name, ServiceID string; RequiresPersonalAuth bool; Authorized bool; State string /* authorized|expired|unauthorized */; AuthorizeURLPath string /* "/api/v1/mcp-services/"+ServiceID+"/oauth/authorize-url" 固定拼接 */; RevokePath string; RequiresAuthTools []string}`
  - 路由：`GET /api/v1/plugins/installations/:id/connections/me`（Viewer+）
  - `internal/modules/plugins/pluginpg` 测试基建（integration 包内）：`openPluginDB(t)`（isolated schema + AutoMigrate 冻结模型 `types.MCPService/MCPToolApproval/MCPOAuthToken/MCPOAuthClient/MCPMetadata/PluginPreview/PluginInstallation/TenantMember`）、`newPluginStack(t, db)`（真实 repo + 真实 service + 受控远端）——**后续 T13/T16-T19 全部复用**

- [ ] **Step 1: 写失败测试（fake 层）**

`connection_status_test.go`：

```go
func TestGetMyConnectionStatusStates(t *testing.T) {
	// 安装含 requires_personal_auth 工具：
	// a) 无 token → Authorized=false, State=unauthorized, AuthorizeURLPath 指向物化 service_id。
	// b) 有效 token（ExpiresAt 未来）→ State=authorized。
	// c) token 过期且无 RefreshToken → State=expired。
	// d) 清单无 personal_oauth → RequiresPersonalAuth=false, State 恒 authorized（无账号插件）。
}

func TestGetMyConnectionStatusTenantScoped(t *testing.T) {
	// 租户 2 成员查租户 1 安装 → not found。
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/modules/plugins/ -run 'TestGetMyConnectionStatus' -v`
Expected: FAIL —— 方法未定义。

- [ ] **Step 3: 实现**（`GetMyConnectionStatus`：`GetInstallation` 校验租户 → 读快照聚合 `RequiresAuthTools` → `oauthRepo.GetTokenForPrincipal` 判定状态；`RequiresPersonalAuth = manifest Auth.PersonalOAuth`——由安装快照的工具声明推导：任一工具 `RequiresPersonalAuth` 即 true；DTO 不含任何 token 材料）。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/modules/plugins/ -run 'TestGetMyConnectionStatus' -v && go build ./...`
Expected: PASS。

- [ ] **Step 5: 写失败测试（真 PG 应用边界：授权/隔离/撤销/过期）**

`plugin_pg_integration_test.go`：

```go
//go:build integration
// 环境契约：PLUGIN_TEST_DATABASE_URL（一次性 PostgreSQL DSN）。缺失 → Fatal "blocked-env"，绝不 Skip 通过。

func TestMemberConnectionsIsolatedAndRevocable(t *testing.T) {
	// 受控插件服务带 OAuth 端点（plugintest 扩展：两成员凭据 user-a/user-b，
	// token 换发后 CallTool 校验 Bearer）。真实 stack：repo+service+manager+oauthManager。
	// 1) 管理员预览+确认安装（真实 ConfirmInstallation）。
	// 2) 成员 A 走真实 OAuth 流（AuthorizeURL→受控 /authorize→token 交换）→ connections/me State=authorized。
	// 3) 成员 A call search 工具成功且只命中 A 的数据；成员 B 未授权 → OAuthRequiredError。
	// 4) B 完成授权 → B 只命中 B 的数据（凭据不混用）。
	// 5) A Revoke（DELETE token 语义经 oauthManager.Revoke）→ A unauthorized、B 仍 authorized。
	// 6) 空间 2 成员查同安装 → not found；空间 2 另装同插件各自 token 互不可见。
}

func TestExpiredTokenGuidesReauthorization(t *testing.T) {
	// 直接在 repo 中把 A 的 token ExpiresAt 置过去（无 refresh）→
	// connections/me State=expired；调用工具 → 引导重授权错误。
}
```

（plugintest 扩展 OAuth——`Server.EnableOAuth(users map[string]string)` 必须实现**完整端点集**以支撑真实客户端流 `oauthManager.StartAuthorization`（oauth_manager.go:80-165：先做 RFC 9728 保护资源发现与授权服务器元数据发现，再 `RegisterClient` 动态客户端注册 + PKCE，最后授权码交换）：
- MCP `/mcp` 端点对未认证请求返回 401 + `WWW-Authenticate: Bearer resource_metadata="<base>/.well-known/oauth-protected-resource"`（触发客户端发现）；
- `GET /.well-known/oauth-protected-resource` → `{"authorization_servers": ["<base>"]}`；
- `GET /.well-known/oauth-authorization-server` → `{"authorization_endpoint": "<base>/authorize", "token_endpoint": "<base>/token", "registration_endpoint": "<base>/register", "grant_types": ["authorization_code", "refresh_token"]}`；
- `POST /register`（动态客户端注册）→ `{"client_id": "plugintest-client"}`；
- `GET /authorize`（带 `code_challenge` S256）→ 按 POST 凭据选择成员（`users` 表：成员名→凭据）→ 302 `redirect_uri?code=...&state=...`；code 一次性；
- `POST /token`（`grant_type=authorization_code` + `code_verifier` PKCE 校验）→ `{"access_token": "tok-<user>", "token_type": "Bearer"}`；
- `CallTool` 校验 Bearer 并按 `tok-<user>` 归属返回该成员数据。
端点清单与 T04 `examples/plugins/jira-todo-mcp/oauth.go` 同构（替身复刻示例服务契约）。）

- [ ] **Step 6: 运行确认失败**

Run: `go test ./internal/modules/plugins/ -tags integration -run 'TestMemberConnections|TestExpiredToken' -v`
Expected: 无 `PLUGIN_TEST_DATABASE_URL` 时 `blocked-env` Fatal（预期阻塞）；配置后 FAIL（plugintest OAuth 扩展未实现）。

- [ ] **Step 7: 实现 plugintest OAuth 扩展与集成测试，运行通过**

Run: `PLUGIN_TEST_DATABASE_URL=postgres://... go test ./internal/modules/plugins/ -tags integration -run 'TestMemberConnections|TestExpiredToken' -v`
Expected: PASS（执行者用本地一次性 PG，如 `docker run -e POSTGRES_PASSWORD=... -p 127.0.0.1:<freeport>:5432 postgres:16`；端口避开 5432，先例 R8 模式）。

- [ ] **Step 8: Commit**

```bash
git add internal/types/ internal/modules/plugins/ internal/application/ internal/handler/ internal/router/
git commit -m "feat(plugins): 成员个人授权状态视图与两成员/跨空间隔离集成测试 [T11]"
```

---

### Task 12: 前端成员授权/撤销入口

**Files:**
- Modify: `packages/api-client/src/plugins.ts` / `plugins.test.ts`（`getMyConnection` + 解析器）
- Modify: `apps/web/src/integrations/PluginsPanel.tsx` / `PluginsPanel.test.tsx`（每插件行：授权状态徽标 authorized/expired/unauthorized；"去授权"按钮 → `window.open(authorize_url)`；"撤销"按钮 → `DELETE /mcp-services/:service_id/oauth/token`（复用 `client.configuration` 既有方法，configuration.ts:231-249 解析器对应方法——执行时以该文件实际方法名为准）→ 刷新状态）
- Test: 同两测试文件追加用例

**Interfaces:**
- Consumes: T11 的 `dto.PluginMyConnection` JSON；既有 mcp oauth api-client 方法。
- Produces: `client.plugins.getMyConnection(installationId)`。

- [ ] **Step 1: 写失败测试**（`parsePluginMyConnection` 字段/非法拒绝；面板：unauthorized 显示"去授权"、authorized 显示"撤销"、无 personal_oauth 不显示授权区；撤销后状态刷新为 unauthorized）。
- [ ] **Step 2: 运行确认失败**：`pnpm gates` → FAIL。
- [ ] **Step 3: 实现**。
- [ ] **Step 4: 运行确认通过 + Commit**

Run: `pnpm gates`（Expected 全过）
```bash
git add packages/api-client/src/plugins.ts packages/api-client/src/plugins.test.ts apps/web/src/integrations/
git commit -m "feat(plugins): 成员授权状态徽标与授权/撤销入口 [T12]"
```
