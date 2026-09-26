# Issue #106 自行托管插件与清单安装 — 代码基线勘察

日期：2026-09-23。分支：`codex/issue-106-self-hosted-plugins`（worktree `.worktrees/issue106`，与 `main` 同为 `4bcad69ba`，工作树干净）。
所有路径相对 worktree 根 `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue106`。

输入材料：`CONTEXT.md`（插件/插件版本/插件清单/应用/应用安装/连接/个人连接/操作/任务授权 术语，301-340 行）、`docs/specs/2026-09-23-self-hosted-plugins-spec.md`、`docs/specs/2026-09-23-self-hosted-plugins-design.md`、`website-docs/03-features/08-mcp.md`（设计文档引用的 MCP 功能说明，已确认存在）。

---

## 1. 现有 MCP 接入路径（WeKnora 作为 MCP 客户端）

### 1.1 数据模型 — `internal/types/mcp.go`

- `MCPService`（24-45 行）：`TenantID+Name` 唯一索引；`URL *string` 可变、`TransportType`（`sse`/`http-streamable`/`stdio`，18-22 行）；`AuthConfig *MCPAuthConfig`（none/`api_key`/`bearer`/`oauth`，62-73 行）；`IsBuiltin`（跨空间可见的内置服务）。**没有版本、没有端点快照** —— 这是 Spec 指出的核心差距。
- `MCPAuthConfig`（87-107 行）：OAuth 模式不在该结构存密钥，`Scopes []string`、`AuthServerMetadataURL`（RFC 9728/8414 自动发现）；`APIKey/Token` 经 `Value()/Scan()` AES-256-GCM 加解密（232-282 行，`SYSTEM_AES_KEY`）。
- `MCPTool`（131-137 行）：`Name/Description/InputSchema json.RawMessage/RequireApproval`。
- `MCPToolApproval`（142-155 行）：`(TenantID, ServiceID, ToolName)` 唯一；`RequireApproval` 默认 false、**`Enabled` 默认 true，缺失行视为启用**（149-152 行注释明确 backwards compatibility）—— 与 Spec「新增写工具默认关闭」相反，实施必须处理该默认值的迁移语义。
- `MCPToolPolicyPatch`（160-163 行）：`RequireApproval *bool` / `Enabled *bool` 部分更新。
- 凭据脱敏在 DTO 层：`internal/handler/dto/mcp.go` 的 `NewMCPServiceResponse`（类型文件 354-359 行注释指认）。

### 1.2 MCP 客户端与连接管理 — `internal/modules/airesource/mcp/`

- `client.go:25-52` 接口：
  ```go
  type MCPClient interface {
      Connect(ctx) error; Disconnect() error
      Initialize(ctx) (*InitializeResult, error)
      ListTools(ctx) ([]*types.MCPTool, error)
      ListResources(ctx) ([]*types.MCPResource, error)
      CallTool(ctx, name string, args map[string]interface{}) (*CallToolResult, error)
      ReadResource(ctx, uri string) (*ReadResourceResult, error)
      IsConnected() bool; GetServiceID() string
  }
  ```
  封装 `github.com/mark3labs/mcp-go`（client/transport/mcp 三个子包）。`ClientConfig`（55-67 行）携带 `TenantID/Principal`，OAuth token 按 `(TenantID, Principal, Service.ID)` 作用域。OAuth 未授权时返回 `OAuthRequiredError`（124-136 行）。
- `manager.go`：`MCPManager.GetOrCreateClient(ctx, service)`（88 行）按 `cacheKey(service, principal)`（75 行）缓存连接；配置 `UpdatedAt` 变化即重建连接（`manager_startup_test.go:85-90` 验证）。
- OAuth：`oauth_manager.go`（`StartAuthorization/CompleteAuthorization/IsAuthorized/Revoke` 等）、`oauth_state.go`（一次性 state）、`oauth_lifecycle.go`（`ensureFresh/refreshWithLease`，74 行 refresh lease 防并发刷新）、`oauth_tokenstore.go`。
- Token 模型 `internal/types/mcp_oauth.go`：`MCPOAuthToken` 唯一索引 `idx_mcp_oauth_tokens_tenant_principal_svc`（79-82 行，`PrincipalType/PrincipalID`），表 `mcp_oauth_tokens`；`MCPOAuthClient` 表 `mcp_oauth_clients`。

### 1.3 Service / Handler / 路由

- `internal/application/service/mcp_service.go`：`CreateMCPService/GetMCPServiceByID/ListMCPServices/UpdateMCPService/DeleteMCPService/TestMCPService/GetMCPServiceTools/UpdateMCPCredentials/ClearMCPCredential/GetMCPServiceResources`。`GetMCPServiceTools`（454-481 行）实时 `GetOrCreateClient + ListTools`。
- `internal/application/service/mcp_tool_approval_service.go`：`ListByService/SetPolicy/IsRequired/IsEnabled`。
- `internal/application/service/mcp_metadata.go`：`GetMCPMetadata/ListMCPMetadataSummaries/PersistMCPMetadata/RefreshMCPMetadata`。快照模型 `internal/types/mcp_metadata.go:28-40`：`MCPMetadata` 主键 `(TenantID, ServiceID, Principal)` + `ConfigFingerprint`（57-68 行，对 transport/URL/headers/auth/stdio/env 的 SHA-256）—— **已有按 principal 隔离的工具目录快照先例，可被版本快照复用**。
- Handler：`internal/handler/mcp_service.go`（CRUD/test/tools/tool-approvals/ResolveToolApproval）、`mcp_oauth.go`（AuthorizeURL/Callback/Status/Revoke/ResolveMCPOAuth/CancelMCPOAuth）、`mcp_credentials.go`（Put/DeleteField）、`mcp_metadata.go`、`mcp_usage_instructions.go`。
- 路由 `internal/router/routes_infra.go:152-210`：
  - `/api/v1/mcp-services` 组：写操作 `g.Admin()`（创建/更新/删除/test/credentials/tool-approvals 写），读 `g.Viewer()`（list/get/tools/metadata/refresh）。
  - **成员个人 OAuth 已是 Viewer 可用**：`POST /:id/oauth/authorize-url`、`GET /:id/oauth/status`、`DELETE /:id/oauth/token`（192-194 行）。
  - OAuth 回调挂组外：`GET /mcp-oauth/callback`（156 行）。
  - 会话内审批/OAuth pending：`agent-tool` 组 `POST /mcp-oauth-resolutions/:pending_id[...]`（209-210 行）+ `routes_agent.go:248-251`（embed 侧同款）。
  - API-key 能力：`GET /api/v1/mcp-services` 需 `types.APIKeyCapabilityManageMCPServices`（`router_api_key_capabilities_test.go:371`）。

### 1.4 既有「MCP OAuth ↔ 应用安装」桥梁（重要，Spec Further Notes 要求明确共用语义）

- `internal/application/repository/mcp_oauth.go`：`MCPOAuthBindingStore`（247 行起）——`IssueBindingState/GetBindingState/CompleteBinding/RevokePersonalConnections/RevokeConnection/FindConnectionByID/LoadCredential/MemberActive/TryAcquireRefreshLease`。`CompleteBinding`（327 行起）在事务中校验 `installations` 表内安装存在且 `InstallationActive`（367-376 行）后把凭据绑定成 `connections` 行。
- 迁移 `migrations/versioned/000118_mcp_oauth_binding_states.up.sql`：`mcp_oauth_binding_states(state PK, tenant_id, actor_id, installation_id, service_id, expires_at, used, ...)` —— **「成员授权关联到具体安装」已有表级先例**。
- 相关迁移：`000042_mcp_tool_approval`、`000091_mcp_tool_enabled`、`000092_mcp_metadata`、`000074_mcp_oauth_refresh_lease`、`000118_mcp_oauth_binding_states`（versioned 目录，最新头为 `000188_tenant_agent_marketplace`）。

### 1.5 不要误用的两个「Server 方向」

- `mcp-server/`（Python）：WeKnora 对外提供 MCP Server（设计文档 49 行明确禁止误用为插件安装运行时）。
- `cli/internal/mcp/server.go`：`weknora mcp serve` 把 WeKnora 工具暴露为 stdio MCP server（`modelcontextprotocol/go-sdk`）。两者与本次「WeKnora 作为客户端调用开发者服务」方向相反。

## 2. 空间 / 成员 / 角色模型与 Agent 工具调用链路

### 2.1 空间与角色

- `internal/types/tenant.go:86` `Tenant`（ID uint64 主键）。
- `internal/types/tenant_member.go`：`TenantRole`（18-32 行）= `owner(40)/admin(30)/contributor(20)/viewer(10)`；`HasPermission(required)`（59 行）层级比较；`TenantMember`（86 行）默认角色 `contributor`。路由侧 `g.Admin()/g.Viewer()` 中间件即消费该模型（`internal/middleware/auth.go`）。
- 跨空间共享组织：`internal/types/organization.go`（`Organization/OrganizationTenantMember`）——与空间内安装无关，仅作边界说明。

### 2.2 Agent 会话内 MCP 工具链路（手工配置现状）

1. 会话组装：`internal/application/service/agent_service.go:289` `tools.RegisterMCPTools(ctx, toolRegistry, enabledServices, s.mcpManager, s.toolApprovalGate, config.MCPAuthWaitTimeout, s.mcpServiceService.GetMCPServiceByID, &tools.MCPMetadataIO{...})`（240-299 行按 agent config 的 MCP 模式选择服务，只注册 `Enabled` 的服务）。
2. 目录与两段式调用：`internal/modules/agentruntime/agent/tools/mcp_tool.go:539` `RegisterMCPTools(ctx, registry, services, mcpManager, gate, authWaitTimeoutSeconds, lookup, metadata) (int, error)` 安装 `discover_mcp_tools` + `call_mcp_tool` 代理（不预载全 schema）。目录实现在 `tools/mcp_catalog.go:132` `MCPCatalog{tenantID, principal, oauthPrincipal, servers, load, lookup, gate}`；describe 返回 `tool_ref`（`mcpToolRef` 对 schema 的哈希引用），call 前必须重新 `snapshot` 核对。
3. 调用时再验证：`tools/mcp_exposure.go:44` `MCPRegisteredTool.Execute` 重新 `catalog.snapshot` + `checkEnabled`，工具定义漂移即报 `unavailable`（68-71 行「MCP tool definition changed or was removed; rediscover before calling」）—— **已有运行时防漂移的单服务行为，但没有跨版本的管理员确认流程**。
4. 审批闸门：`tools/mcp_tool.go:114` `MCPTool.Execute` —— 调用时重查 `gate.IsEnabled`；`gate.NeedsApproval` 为真则 `gate.RequestAndWait(ctx, approval.PendingRequest{...})` 阻塞等待成员决定；`Decision{Approved, ModifiedArgs, Reason, TimedOut, ContextCanceled}`（`internal/modules/agentruntime/agent/approval/gate.go:78-84`）；拒绝/超时不派发。`MCPApproval` 接口（gate.go:106-110）：`NeedsApproval/IsEnabled/RequestAndWait`。
5. 跨实例审批：`Gate`（gate.go:153-162）内存 waiter + Redis Pub/Sub fan-out（`Resolve` 任意副本可达）；`DurableGate`（durable_gate.go:37）包装持久 pending（`RequestOAuthAndWait` 对话中 OAuth park）。
6. 成员侧审批 UI API：`packages/api-client/src/chat/approvals.ts:45` `POST /api/v1/agent/tool-approvals/:pendingId`。

### 2.3 ADR-0013 服务端预检（native 链路，当前与 MCP 链路并存）

- 契约 `internal/modules/agentruntime/agent/nativecontract/contracts.go`：`ToolDispatchRequest{Scope, Fence, Plan, Attempt, DecisionReference, Funding, ReservationUnits}`（203-214 行）、`ToolDispatchPreflight.Authorize`（220-222 行，fail-closed）、`ToolDispatchReservation.ReserveAndConsume`（228-230 行）、`ToolPlan/ToolIdentity{Kind, ServiceID, InstallationID, Name, SchemaHash, ConfigVersion}`（180-198 行 —— **identity 已含 InstallationID 与 SchemaHash 字段，可承载插件版本 pin**）。
- 实现 `internal/modules/agentruntime/agent/native/tool.go:45` `ServerToolDispatchPreflight.Authorize`（重查 scope/grants）+ `WrapCallableTool/WrapStreamableTool`（107/123 行，preflight 包装的 callable）。journal：`internal/application/repository/native_tool_journal.go`。
- 现状边界：ADR-0013/0014 明确该 seam 只授权 fail-closed 适配器与确定性 fake，**尚未接 MCP/connector 生产派发**；插件远程调用若走此 seam 属新增集成，需在实施计划中明确是否复用还是继续走 2.2 的 Gate 链路。

## 3. AppConnector（应用安装/连接/操作）现状 — Spec 要求共用的领域模型

- 领域模型 `internal/modules/appconnector/model.go`：
  ```go
  type Installation struct { ID, AppID, Version, State string; TenantID uint64 }   // active/disabled/reauthorization_required
  type Connection struct { ID, InstallationID, Kind, OwnerID, CredentialRef, State string; TenantID uint64; AuthVersion int64 } // personal/space
  ```
- 表：`internal/modules/appconnector/repository/appconnector/install.go`（`app_versions/installations/connections`）、`action.go`（`app_actions/app_action_approvals/app_action_preauthorizations`）、`oc_dispatch.go`（`connector_dispatch_records/connector_dispatch_leases`）。
- 路由 `internal/router/routes_app_connectors.go:20-95`：`/apps/installations`（List/Create/Upgrade/Disable，owner/admin 写闸 `RequireInstallCapabilityForWrites`）、`/apps/catalog`（T13 成员可读的已审阅 action 目录）、`/apps/connections`（CreateConnection 走 A02 OAuth，Revoke）、`/apps/authorization-attempts`、`/apps/oc/actions/prepare`（T13 服务端派生 risk/version）、`/apps/actions/:id/approve|execute`（A03 审批管线）。
- 远程定义 pin：`internal/modules/appconnector/service/appconnector/oc_catalog.go` —— `OCSchemaDigest(schema []byte) string`（79 行 SHA-256）、`ValidateOCDefinition`（98 行，自报 digest 不被信任，111-112 行 mismatch 报错）、`PublishOCDefinition`（199 行，无 digest 时从 schema 字节 pin，200-204 行）—— **Spec Testing Decisions 点名的「远程定义 pin」先例**。
- open-connector 共享运行时客户端：`internal/modules/appconnector/openconnector/`（scoped HTTP client）；绑定 `oc_binding.go`、动作 `action.go`。ADR-0001 约束：WeKnora 校验连接归属 + 按连接限制的运行 Token，管理凭据不给终端用户/Agent；本 Spec 57 行明确「不要求迁入 open-connector，也不让插件直接获得其管理凭据」。

## 4. 测试基建

### 4.1 命令与版本

- Go：`make test` = `go test -v ./...`（`Makefile:107-108`）；`go.mod` 声明 `go 1.26.0`，本机 `go version go1.26.3 darwin/arm64`（已验证）。
- 前端/共享包：`package.json` scripts —— `test:shared`、`typecheck:shared`、`test:web`、`typecheck:web`、`check:integrity`、`build:web`；组合入口 `pnpm gates` = `node scripts/run-gates.mjs`（6 个 gate 串行；`MIN_MAJOR = 26`，PATH node 不足 26 时自动从 `WEKNORA_NODE_BIN`/homebrew/`~/.nvm` 找 ≥26 重执行）。本机 PATH node 为 **v22.22.3**（已验证），直接跑 `test:shared` 会踩已知 tsx/CJS createPortal false-red，须走 `pnpm gates` 或显式 node≥26。`engines.node >= 26`。
- 本次实际运行（均在 worktree 内）：
  - `go test ./internal/modules/airesource/mcp/ -run 'TestManagerCloseRetiresPendingConnection' -count=1` → `ok ... 0.780s`
  - `go test ./internal/modules/agentruntime/agent/tools/ -run 'TestMCPProxyHTTPApprovalArgumentsAndImages' -count=1` → `ok ... 1.892s`（仅 ld duplicate-library 警告）
  - 未运行：全量 `go test ./...`、`pnpm gates`（勘察任务未要求全量，实施计划首轮应跑）。

### 4.2 受控远程 MCP 测试服务先例（Spec Testing Decisions 直接点名的基础）

1. `internal/modules/airesource/mcp/manager_startup_test.go:21-91`：`sdkserver.NewMCPServer` + `sdkserver.NewStreamableHTTPServer(server, sdkserver.WithStateLess(true))` 挂 `httptest.NewServer`，前置 `utils.SetSSRFWhitelistFromRaw("127.0.0.1")` + `t.Cleanup(utils.ResetSSRFWhitelistForTest)`（127.0.0.1 直连 IP 默认被 SSRF 防护拦截，`internal/utils/security.go:324` `isSSRFSafeURL`、`SSRF_WHITELIST` env）。
2. `internal/modules/agentruntime/agent/tools/mcp_catalog_integration_test.go:34-105`（**最贴近本 Spec 验收 seam 的先例**）：同样 SDK server + httptest，另配 `proxyApprovalGate`（`approval.MCPApproval` 接口的测试替身，记录 `PendingRequest`、可拒绝/禁用），走 `RegisterMCPTools → discover(list_tools) → describe → call_mcp_tool` 全链，断言审批参数修改、图片内容、外部调用计数。

### 4.3 应用边界集成测试先例（管理员安装→成员授权→Agent 调用 对应物）

- `internal/modules/appconnector/service/appconnector/oc_integration_test.go`（`//go:build integration`）：环境契约 `OC_TEST_DATABASE_URL`（真实一次性 PostgreSQL，迁移 000117→000124），缺失时 `TestOCIntegrationEnvironment` **Fatal（blocked-env），绝不 Skip 通过**；两空间、跨空间成员、共享运行时、HTTP fake provider（fake provider ≠ fake OC，grants/OAuth 关联/审批 digest/幂等/恢复全走真实代码）；acceptance journal 输出 `OC17-EVIDENCE` JSON 行。本 worktree 未配置该 env，未运行。
- handler 层模式：`internal/handler/mcp_tool_approval_test.go` —— `gin.SetMode(gin.TestMode)` + 最小路由 + stub `interfaces.MCPToolApprovalService` + `httptest.NewRecorder`（该模式用于 422 语义边界，非应用边界）。

## 5. 外部依赖核查（git 证据）

在 worktree 内执行 `git log` 核验：

- **open-connector 成果已在 main**：`c10405a06 Merge branch 'codex/open-connector-integration'`、`144ecb8b0 feat(connectors): add scoped open-connector HTTP client`、`00aeceef6 refactor(appconnector): move packages to internal/modules/appconnector`、`b50ee126a merge: pass-a worker appconnector (task A1)`、`638d3eb60 merge: pass-a worker commercial (task A2)`、`8f5d3caa3/851d7cd78`（craft 迁移重编号 / sqlite migration head 跟踪）。本分支与 main 同为 `4bcad69ba`（`git merge-base --is-ancestor main HEAD` 通过，双向无差集），**当前分支已具备全部 open-connector/AppConnector 能力**。
- `/private/tmp/weknora-head`（detached `83ce05bd2`）：已含于 main，无未集成内容。
- **其他 worktree 未集成成果（不算本分支能力，均经 `git log main..<branch> --oneline | wc -l` 核验）**：
  - `.worktrees/backend-mod-wave01`（backend-mod-wave01，21 commits：queryhistory 后端模块化 + architecture guard）
  - `.worktrees/bm-t10`（sdd-bm-t10，22）、`.worktrees/bm-t5`（sdd-bm-t5，5）
  - `.worktrees/issue30-sweep*`（codex/issue30-mobile-office 32 + t35/t66）
  - `.worktrees/issue72-lago`（codex/issue-72-lago，21：Lago OCR 修复）
  - `.worktrees/passb-b0`（18）、`.worktrees/passb-int`（codex/passb-integration，3：文档）
  - `.worktrees/tdm-int`（feat/tdesign-react-migration，**68 commits**：settings/integrations 等 Tailwind 清零与 parity 迁移）及其子 worktree tdm-w2-s4/s5/s6/s7、tdm-s2-chat、tdm-s3-kb
  - `~/.codex/worktrees/craft-107-*`（均 detached 于 `4bcad69ba`，与 main 同，无领先提交）
  - 上述分支无 MCP/插件/连接器功能性重叠；**主要合并冲突面是 tdm-int 的前端 settings/integrations 区域**（若先合入会改动 `apps/web/src/settings`、`apps/web/src/integrations` 的样式与结构）。

## 6. 前端挂载点（按仓库现状确定目标端）

双前端并存：**目标端为 React（`apps/web` + `packages/*`）**，老 Vue（`frontend/`）仍在运行 MCP 设置页但处于 parity 迁移退出期（近期提交均为 `feat(parity): migrate ... to Vue geometry/tdesign` 方向的对照迁移）。

### 6.1 React 端（目标端）

- **管理员 MCP 设置页（现有挂载点）**：`apps/web/src/settings/McpSettingsPanel.tsx`（1383 行，服务 CRUD/测试/OAuth 状态/工具目录抽屉）+ `McpToolsDirectory.tsx`（98 行）。挂载于 `apps/web/src/settings/SettingsPage.tsx`：31 行 `lazy(() => import('./McpSettingsPanel.tsx'))`，444-466 行按 `key === 'mcp'` 渲染（`PARTIALLY_PORTED_SECTIONS` 含 `mcp`，105 行；`client.configuration.mcp.list()` 预取，157 行）；导航分组 621 行（dataExtensions 组含 `mcp`）。
- **注意不一致**：`packages/views/src/settings/registry.ts:41` 声明 `{ key: 'mcp', viewId: 'McpServiceSettings', ..., ported: false }` —— registry 的 `ported` 标志与 apps/web 已存在的面板状态有出入（registry 描述的是共享 views 层通用映射），实施时以 `apps/web/src/settings/SettingsPage.tsx` 的实际挂载为准，并同步更新 registry 条目避免误导。
- **成员侧发现入口（待新增的挂载点候选）**：`packages/views/src/integrations/registry.ts` `INTEGRATION_SECTIONS`（im/embed/api/cli/chrome/claw 六节，含 `minRole`/`apiDomain`）+ `apps/web/src/integrations/IntegrationsPage.tsx`/`IntegrationsRoutePage.tsx`。插件发现/个人授权页可作为新 section 或独立路由挂入。
- API 客户端：`packages/api-client/src/configuration.ts`（mcp-services CRUD/test/oauth envelope 解析，231-249/558-570 行）；`packages/api-client/src/appconnector.ts`（apps 路由）。
- 部署能力开关：`settings.mcp` capability（`internal/handler/deployment_capabilities.go`，`router/deployment_capabilities_test.go:68` 引用）。

### 6.2 老 Vue 端（保留兼容，不作为新功能目标端）

- `frontend/src/views/settings/McpSettings.vue`（588 行，`Settings.vue:211-212` 按 `currentSection === 'mcp'` 挂载）+ `components/McpServiceDialog.vue`（1429 行）、`McpToolsList.vue`、`McpMetadataPanel.vue`、`McpTestResultBody.vue`（均带 `.test.ts`）。

## 7. 风险与歧义（供实施计划处理）

1. **默认启用语义冲突**：`MCPToolApproval` 缺行 = 启用（`internal/types/mcp.go:149-152`）；Spec 要求新写工具默认关闭且「迁移不得把未审阅的旧工具自动视为已批准插件版本」（spec 55 行）。需要显式迁移策略与读写分类来源。
2. **无版本模型**：`MCPService.URL` 可变、无端点快照/内容摘要；`MCPMetadata` 快照按 `(tenant, service, principal)` 存在但无版本维度。新表需要与 `MCPConfigFingerprint`、`mcpToolRef`、`OC OCSchemaDigest` 的既有哈希语义对齐而不是另造一套。
3. **两套授权 seam 并存**：MCP 走 `approval.Gate`（2.2），native 走 `ToolDispatchPreflight`（ADR-0013，未接生产 MCP）；`ToolIdentity` 已有 `InstallationID/SchemaHash` 字段但无消费者。实施计划必须裁决插件远程调用挂在哪条链（或以 Gate 为主、preflight 为后续集成）。
4. **registry `ported:false` 与实际面板不一致**（见 6.1），改 settings 注册表时须核实，避免误导验收。
5. **node ≥26 硬要求**：PATH 默认 v22.22.3；必须经 `pnpm gates`（自动 re-exec）或显式 ≥26 二进制，否则 `test:shared` 有已知 false-red。
6. **集成测试环境**：`oc_integration_test.go` 模式要求真实 PG（`OC_TEST_DATABASE_URL`，缺失即 Fatal）。本 Spec 的应用边界测试若沿用该模式需准备一次性 DB 与迁移头（当前 versioned 头 000188）；受控远程 MCP 服务用 httptest + `SetSSRFWhitelistFromRaw("127.0.0.1")` 模式即可，无外部依赖。
7. **前端冲突面**：`tdm-int` worktree 68 commits 深耕 `apps/web/src/settings`/`apps/web/src/integrations`；若其先合入 main，本需求前端改动需要 rebase 适配（后端无此风险）。
8. **凭据写入约束**（Mimosa 安全约束，与本需求直接相关）：清单抓取/连接配置只从环境变量或密钥服务读取凭据；源码、示例与测试不得写入可用凭据字面量。既有代码已符合该模式（`MCPAuthConfig` AES 加密存储、DTO 脱敏、OAuth token per-principal）。
