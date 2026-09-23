# 插件 03｜确认空间安装并供成员发现 实施计划（Issue #110）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 管理员从经核验的预览确认安装一个确定版本：确认时重新核对预览身份与版本，预览已变或过期则拒绝、确认幂等；安装后所有空间成员可发现插件、跨空间不可见；管理员可停用/启用；固定安装 ID、已接受版本、空间归属与状态的对外语义；现有手工 MCP 服务完全兼容；迁移不把未审阅旧工具视为已批准插件版本。

**Architecture:** 新表 `plugin_installations`（`(tenant_id, plugin_id)` 唯一；`accepted_version`/`endpoint_url`/`tools_snapshot`/`tools_digest`/`state`/`drift_state`/`service_id`）+ `mcp_services` 新列 `plugin_installation_id`。确认安装在一个事务里：重核预览（存在/未消费/未过期/重抓远端 digest 一致）→ 写安装行 → 物化 `mcp_services` 行（`Name="plugin:"+plugin_id`、URL=版本端点、OAuth 声明时 `AuthType=oauth`）→ 为快照内每个工具写显式 `MCPToolApproval` 行（只读 `Enabled=true`、写 `Enabled=false`）→ 标记预览 consumed。停用/启用同步安装状态与物化服务 `Enabled`。Agent 侧沿用 `registerMCPTools` 只注册 `Enabled` 服务的现状（agent_service.go:279-283），停用即不可见。

**Tech Stack:** Go 1.26；gorm 事务 + PostgreSQL `000190`/SQLite `000111` 迁移；gin；`interfaces.MCPServiceService.CreateMCPService`、`interfaces.MCPToolApprovalService.SetPolicy`。

**Spec:** `docs/specs/2026-09-23-self-hosted-plugins-spec.md`（User Stories 10/11/16/27；Implementation Decisions 51/53/55 行）；范围裁决 GAP-3（迁移与回退）落实。

**Issue:** https://github.com/1123786563/WeKnora-fork01/issues/110 （blocked_by #108 → 任务级前置 T02）

## Global Constraints

见总索引。本切片新增迁移 PG `000190_plugin_installations`（含 `mcp_services` 加列）/ SQLite `000111` twin；up 零回填、down 反序清理（GAP-3）。

## Review Focus（本切片）

- 预览过期/已消费/身份指纹变化 → 安装拒绝且零写入（T06）。
- 幂等：同 preview 重复确认、同 `(tenant, plugin)` 重复安装 → 语义明确不产生第二行（T06）。
- 跨空间：空间 2 无法读/调空间 1 的安装与物化服务（T07）。
- 手工 MCP 兼容：既有手工服务行为与其 `MCPToolApproval` 缺省语义不变（T07）。
- 回退安全：down 迁移清干净插件派生行、手工服务原样（T06）。

---

### Task 6: 安装确认、物化与停用/启用（后端核心）

**Files:**
- Create: `migrations/versioned/000190_plugin_installations.up.sql` / `.down.sql`
- Create: `migrations/sqlite/000111_plugin_installations.up.sql` / `.down.sql`
- Modify: `internal/types/mcp.go`（`MCPService` 追加 `PluginInstallationID *string` 字段，gorm tag `type:varchar(36);index`，json `plugin_installation_id,omitempty`）
- Modify: `internal/types/plugin.go`（追加 `PluginInstallation` 模型）
- Modify: `internal/types/interfaces/plugin.go`（`PluginRepository` 扩展安装方法）
- Modify: `internal/application/repository/plugin.go`
- Create: `internal/application/service/plugin_install_service.go`（实现挂在 `pluginService` 上，文件独立便于审查）
- Modify: `internal/handler/dto/plugin.go`、`internal/handler/plugin.go`、`internal/router/routes_plugins.go`
- Modify: `internal/database/migration_sqlite_versioned_schema_test.go`（tables 加 `plugin_installations`；columns 加 `"mcp_services": {"plugin_installation_id"}`）
- Test: `internal/modules/plugins/install_service_test.go`
- Test: `internal/handler/plugin_install_test.go`

**Interfaces:**
- Consumes: T01/T02 全部产出；`interfaces.MCPServiceService.CreateMCPService(ctx, service *types.MCPService) error`（mcp_service.go:36）；`interfaces.MCPToolApprovalService.SetPolicy(ctx context.Context, tenantID uint64, serviceID, toolName string, requireApproval, enabled *bool) error`（真实签名见 internal/types/interfaces/mcp_tool_approval.go:20）；`types.MCPAuthConfig{AuthType: types.MCPAuthOAuth, Scopes}`。
- Produces（后续任务依赖的精确签名）:
  - `types.PluginInstallation`（字段：`ID string`；`TenantID uint64`；`PluginID string`；`Name string`；`Description string`；`AcceptedVersion string`；`TransportType string`；`EndpointURL string`；`ToolsSnapshot datatypes.JSON`（存 `[]types.PluginToolSnapshot` 序列化；若项目未用 datatypes 包则以 `json.RawMessage`+gorm `type:jsonb` 对齐既有 `MCPTool.InputSchema` 惯例）；`ToolsDigest string`；`ServiceID string`；`DriftState string`（`none|detected`）；`DriftDetail json.RawMessage`；`State string`（`active|disabled`）；`CreatedBy string`；`CreatedAt/UpdatedAt time.Time`）
  - 状态常量：`types.PluginInstallationActive = "active"`、`types.PluginInstallationDisabled = "disabled"`、`types.PluginDriftNone = "none"`、`types.PluginDriftDetected = "detected"`
  - `interfaces.PluginService.ConfirmInstallation(ctx context.Context, tenantID uint64, actorID, previewID string) (*dto.PluginInstallationResponse, error)`
  - `interfaces.PluginService.SetInstallationState(ctx context.Context, tenantID uint64, installationID, state string) (*dto.PluginInstallationResponse, error)`（state ∈ {active, disabled}）
  - `dto.PluginInstallationResponse{InstallationID, PluginID, Name, Description, Version, State, TransportType, EndpointURL, ServiceID, DriftState, Tools []dto.PluginInstallationTool{Name, Description, ReadOnly, RequiresPersonalAuth, Scopes, Enabled *bool}}`
  - 路由：`POST /api/v1/plugins/installations`（Admin，body `{"preview_id": "..."}`）、`POST /api/v1/plugins/installations/:id/disable`（Admin）、`POST /api/v1/plugins/installations/:id/enable`（Admin）

- [ ] **Step 1: 写失败测试（确认安装的核验/幂等/物化/策略行，fake repo + 受控远端）**

`internal/modules/plugins/install_service_test.go`：

```go
package plugins

func TestConfirmInstallationRejectsStalePreview(t *testing.T) {
	// 场景 a：preview 不存在 / 属于其他 tenant → 错误 "preview not found"。
	// 场景 b：preview.ConsumedAt != nil → 错误 "preview already consumed"。
	// 场景 c：preview.ExpiresAt 过去 → 错误 "preview expired"。
	// 场景 d：重抓远端（受控服务切换工具 schema 后）tools_digest != preview.ToolsDigest
	//          → 错误 "preview content changed"。
	// 四场景断言：installations/mcp_services/approvals 均零写入。
}

func TestConfirmInstallationCreatesAndMaterializes(t *testing.T) {
	// 预览只读工具 + 写工具各一 → 确认成功：
	// 断言 1：plugin_installations 行 state=active、accepted_version/endpoint 与预览一致。
	// 断言 2：mcp_services 物化行 Name="plugin:com.example.jira-todo"、
	//          PluginInstallationID=&installationID、AuthConfig.AuthType=oauth（清单声明 personal_oauth 时）。
	// 断言 3：MCPToolApproval 显式行 = 快照工具数；只读 Enabled=true、写 Enabled=false。
	// 断言 4：preview.ConsumedAt 已置位。
}

func TestConfirmInstallationIdempotent(t *testing.T) {
	// 同一 preview 二次确认 → 错误（consumed），不产生第二行；
	// 同 (tenant, plugin_id) 新 preview 再确认 → 错误 "plugin already installed"。
}

func TestSetInstallationStateSyncsService(t *testing.T) {
	// disable → installation.State=disabled 且物化 mcp_services.Enabled=false；enable 反向。
	// 未知 state → 参数错误。
}
```

（测试组装：T01 的受控远端 + T02 fake repo 扩展 + fake `MCPServiceRepository`/`MCPToolApprovalRepository`（实现 `interfaces` 方法集，内存 map）；服务构造 `NewPluginService(pluginRepo, mcpServiceService, toolApprovalService, mcpManager, previewTTL)`——`mcpServiceService` 用真实 `service.NewMCPServiceService(fakeRepo, manager, nil)` 以复用 `CreateMCPService` 的 URL 校验与默认配置逻辑。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/modules/plugins/ -run 'TestConfirmInstallation|TestSetInstallationState' -v`
Expected: FAIL —— `ConfirmInstallation`/`SetInstallationState`/`types.PluginInstallation` 未定义。

- [ ] **Step 3: 实现迁移与模型**

`migrations/versioned/000190_plugin_installations.up.sql`：

```sql
-- Issue #110: tenant-scoped plugin installations. accepted_version +
-- tools_snapshot form the runtime verification baseline (issue #116).
-- service_id points at the materialized mcp_services row. up() writes NO
-- mcp_tool_approvals rows: legacy manual tools are never auto-promoted into
-- approved plugin versions by migration (spec line 55).
CREATE TABLE plugin_installations (
 id VARCHAR(36) PRIMARY KEY,
 tenant_id BIGINT NOT NULL,
 plugin_id VARCHAR(128) NOT NULL,
 name VARCHAR(255) NOT NULL,
 description TEXT NOT NULL DEFAULT '',
 accepted_version VARCHAR(64) NOT NULL,
 transport_type VARCHAR(50) NOT NULL,
 endpoint_url VARCHAR(512) NOT NULL,
 tools_snapshot JSONB NOT NULL,
 tools_digest VARCHAR(64) NOT NULL,
 service_id VARCHAR(36) NOT NULL,
 drift_state VARCHAR(16) NOT NULL DEFAULT 'none',
 drift_detail JSONB,
 state VARCHAR(16) NOT NULL DEFAULT 'active',
 created_by VARCHAR(255) NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX uq_plugin_installations_tenant_plugin ON plugin_installations(tenant_id, plugin_id);
CREATE INDEX idx_plugin_installations_service ON plugin_installations(service_id);

ALTER TABLE mcp_services ADD COLUMN plugin_installation_id VARCHAR(36);
CREATE INDEX idx_mcp_services_plugin_installation ON mcp_services(plugin_installation_id);
```

down（反序、GAP-3 回退方案——手工服务不受影响）：

```sql
DO $$ BEGIN RAISE NOTICE '[Migration 000190 DOWN] removing plugin installations'; END $$;
DELETE FROM mcp_tool_approvals WHERE service_id IN (SELECT id FROM mcp_services WHERE plugin_installation_id IS NOT NULL);
DELETE FROM mcp_oauth_tokens WHERE service_id IN (SELECT id FROM mcp_services WHERE plugin_installation_id IS NOT NULL);
DELETE FROM mcp_metadata WHERE service_id IN (SELECT id FROM mcp_services WHERE plugin_installation_id IS NOT NULL);
DELETE FROM mcp_services WHERE plugin_installation_id IS NOT NULL;
DROP TABLE IF EXISTS plugin_installations;
ALTER TABLE mcp_services DROP COLUMN IF EXISTS plugin_installation_id;
```

SQLite twin `000111`（类型对照惯例：`JSONB`→`TEXT`、`TIMESTAMPTZ`→`DATETIME`、`BIGINT`→`INTEGER`；`ALTER TABLE` 同款）。`types/mcp.go` 的 `MCPService` 追加字段（json `plugin_installation_id,omitempty`；注释说明 NULL=手工服务，行为不变）。

- [ ] **Step 4: 实现确认/停用服务**

`ConfirmInstallation` 流程（单事务或补偿式两段——gorm 跨 repo 事务经 `PluginRepository.WithTx` 暴露 `func(tx *gorm.DB) error` 执行体，repo 方法集增加 `WithTx(ctx, fn)`；物化服务创建与策略行写入都在 tx 句柄上执行）：
1. 取 preview（tenant 归属校验）→ `Expired(now)` 拒绝。
2. `plugins.FetchAndVerify(preview.ManifestURL, lister)` 重抓 → `res.ToolsDigest == preview.ToolsDigest && res.IdentityFingerprint == preview.IdentityFingerprint`，不符拒绝 "preview content changed"。
3. `(tenant, plugin_id)` 已存在 → "plugin already installed"（幂等语义：同插件重装需先走升级路径）。
4. 创建安装行（uuid）→ `CreateMCPService`（构造 `types.MCPService{Name: "plugin:" + pluginID, Description: manifest.Description, Enabled: true, TransportType, URL: &endpoint, AuthConfig: oauth?{AuthType: types.MCPAuthOAuth, Scopes: manifest.Auth.Scopes} : nil, PluginInstallationID: &installationID}`）→ 回填 `installation.ServiceID`。
5. 逐快照工具 `SetPolicy(ctx, tenant, serviceID, tool.Name, nil /* requireApproval */, &enabled /* true（只读）或 false（写） */)`——**写工具 Enabled=false**。
6. `MarkPreviewConsumed`。
`SetInstallationState`：更新安装 `state` + `mcpServiceRepo.Update` 物化行 `Enabled`（`updateFields["enabled"]=true`，走 `UpdateMCPService` 或 repo 直更，保持 `UpdatedAt` 刷新触发客户端重建连接，manager.go 按 `UpdatedAt` 缓存失效）。

- [ ] **Step 5: 运行确认通过**

Run: `go test ./internal/modules/plugins/ -run 'TestConfirmInstallation|TestSetInstallationState' -v`
Expected: PASS。

- [ ] **Step 6: 写失败测试（handler + 迁移执行 + 回退）**

`internal/handler/plugin_install_test.go`（stub service，模式同 T02 Step 5）：确认 200/400（缺 preview_id）/404（服务层 not found 错误映射）；disable/enable 200。
迁移执行测试（新增 `internal/modules/plugins/migration_pg_integration_test.go`，`//go:build integration`，环境 `PLUGIN_TEST_DATABASE_URL` 缺失 `t.Fatal("blocked-env: PLUGIN_TEST_DATABASE_URL required")`——先例 oc_integration_test.go:83）：isolated schema 内 `db.Exec` 依次执行 `000189.up`、`000190.up`，断言 `plugin_previews`/`plugin_installations` 表与 `mcp_services.plugin_installation_id` 列存在；插入一行物化 `mcp_services`（`plugin_installation_id` 非空）+ 派生 approvals/tokens 行后依次执行 `000190.down`、`000189.down`，断言表/列已删、派生行已清、预置的一行**手工** `mcp_services` 仍在（回退安全）。

- [ ] **Step 7: 运行确认失败**

Run: `go test ./internal/handler/ -run 'TestConfirmInstallationHandler|TestSetInstallationStateHandler' -v && go test ./internal/modules/plugins/ -run TestMigration -tags integration -v`
Expected: handler FAIL（未实现）；integration 测试在无 `PLUGIN_TEST_DATABASE_URL` 时以 `blocked-env` Fatal（属预期阻塞而非通过——有 DB 时应 FAIL 于断言，因为 handler/迁移文件未全）。

- [ ] **Step 8: 实现 handler/DTO/路由/登记，运行全部**

实现 `POST /plugins/installations`（Admin）、`/disable`、`/enable`（Admin）；`migration_sqlite_versioned_schema_test.go` 登记。
Run: `go test ./internal/handler/ -run 'TestConfirmInstallationHandler|TestSetInstallationStateHandler' -v && go test ./internal/modules/plugins/ -v && go test ./internal/database/ -run TestSQLiteMigrationsCreateVersionedSchema -v && go build ./...`
Expected: 全部 PASS/build 成功。

- [ ] **Step 9: Commit**

```bash
git add migrations/ internal/types/ internal/modules/plugins/ internal/application/ internal/handler/ internal/router/ internal/database/migration_sqlite_versioned_schema_test.go
git commit -m "feat(plugins): 空间安装确认/物化 MCPService/停用启用与 plugin_installations 迁移回退 [T06]"
```

---

### Task 7: 成员发现 API 与跨空间隔离

**Files:**
- Modify: `internal/types/interfaces/plugin.go`（发现方法）
- Modify: `internal/application/repository/plugin.go`、`internal/application/service/plugin_install_service.go`
- Modify: `internal/handler/dto/plugin.go`、`internal/handler/plugin.go`、`internal/router/routes_plugins.go`
- Test: `internal/modules/plugins/install_discover_test.go`
- Test: `internal/handler/plugin_discover_test.go`

**Interfaces:**
- Consumes: T06 的 `types.PluginInstallation`/常量/DTO；`types.TenantRole` 权限模型（tenant_member.go:18-32——路由侧 `g.Viewer()` 已承载，服务层只按 tenantID 隔离）。
- Produces:
  - `interfaces.PluginService.ListInstallations(ctx context.Context, tenantID uint64) ([]*dto.PluginInstallationSummary, error)`（`PluginInstallationSummary{InstallationID, PluginID, Name, Version, State, DriftState, RequiresPersonalAuth bool, ToolCount int}`）
  - `interfaces.PluginService.GetInstallation(ctx context.Context, tenantID uint64, installationID string) (*dto.PluginInstallationResponse, error)`
  - 路由：`GET /api/v1/plugins/installations`（Viewer+）、`GET /api/v1/plugins/installations/:id`（Viewer+）

- [ ] **Step 1: 写失败测试**

`internal/modules/plugins/install_discover_test.go`（fake repo 预置两空间数据）：

```go
func TestListInstallationsTenantScoped(t *testing.T) {
	// 租户 1 安装 2 个插件、租户 2 安装 1 个：
	// ListInstallations(ctx, 1) 恰好返回租户 1 的 2 个（含 state/drift_state 字段）；
	// ListInstallations(ctx, 2) 返回租户 2 的 1 个；互不可见。
}

func TestGetInstallationRejectsForeignTenant(t *testing.T) {
	// 租户 2 取租户 1 的 installation id → "not found"（不泄露存在性）。
}

func TestManualMCPServiceUnaffectedByInstallations(t *testing.T) {
	// 预置手工 mcp_services 行（plugin_installation_id NULL，无 approvals 行）：
	// ListMCPServices 行为不变（数量/字段/Enabled 语义），
	// 且手工服务的工具在无 approval 行时仍视为启用（types/mcp.go:148-152 缺省语义保持）。
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/modules/plugins/ -run 'TestListInstallations|TestGetInstallationRejects|TestManualMCP' -v`
Expected: FAIL —— `ListInstallations`/`GetInstallation` 未定义。

- [ ] **Step 3: 实现**

repo：`ListByTenant(ctx, tenantID)`、`GetByID(ctx, tenantID, id)`（参数绑定 `WHERE tenant_id = ?`）。DTO 映射注意不返回 `ToolsSnapshot` 原文（成员列表用摘要；详情经 `GET /:id` 返回 `Tools` 数组但同样不含 schema 原文，只含元数据——schema 大且属远端不可信数据，透传给前端无必要）。handler/路由按 T02 模式。

- [ ] **Step 4: 运行确认通过 + Commit**

Run: `go test ./internal/modules/plugins/ -run 'TestListInstallations|TestGetInstallationRejects|TestManualMCP' -v && go test ./internal/handler/ -run 'TestListInstallationsHandler' -v && go build ./...`
Expected: PASS。
```bash
git add internal/types/ internal/modules/plugins/ internal/application/ internal/handler/ internal/router/
git commit -m "feat(plugins): 成员发现安装列表与跨空间隔离 [T07]"
```

---

### Task 8: 前端——安装确认、插件管理与成员发现页

**Files:**
- Modify: `packages/api-client/src/plugins.ts` / `plugins.test.ts`（新增 `confirmInstallation`/`listInstallations`/`getInstallation`/`setInstallationState` + 解析器）
- Modify: `apps/web/src/settings/PluginsSettingsPanel.tsx` / `PluginsSettingsPanel.test.tsx`（确认安装按钮（消费 T03 预览卡的 `preview_id`）+ 已安装列表（版本/状态/漂移徽标）+ 停用/启用 + 删除不需要——卸载不在 Spec 范围）
- Create: `apps/web/src/integrations/PluginsPanel.tsx`
- Test: `apps/web/src/integrations/PluginsPanel.test.tsx`
- Modify: `packages/views/src/integrations/registry.ts`（`IntegrationKey` 联合类型追加 `'plugins'`；`INTEGRATION_SECTIONS` 追加 `{ key: 'plugins', viewId: 'PluginDiscoverPanel', apiDomain: null, minRole: 'viewer', external: false, operations: ['manage'] }`——不设 capability，随 integrations 页默认开放，与 im/embed 同级）

**Interfaces:**
- Consumes: T06/T07 的 API 响应 JSON；T03 的 `client.plugins.*` 客户端；`IntegrationsPage` 现有 tab/section 结构（IntegrationsPage.tsx:1-4 import 面）。
- Produces: `client.plugins.confirmInstallation(previewId)`、`listInstallations()`、`getInstallation(id)`、`setInstallationState(id, state)`；成员发现面板组件。

- [ ] **Step 1: 写失败测试（api-client + 两面板）**

`plugins.test.ts` 追加 `parsePluginInstallations`（列表 envelope）/`parsePluginInstallation`（详情）字段映射与非法 envelope 拒绝。
`PluginsSettingsPanel.test.tsx` 追加：预览卡出现"确认安装"按钮；点击后（stub resolve）列表出现该插件且版本正确；列表行有停用按钮（admin）。
`PluginsPanel.test.tsx`（新，模式 McpSettingsPanel.test.tsx）：

```tsx
test('成员插件面板列出本空间已安装插件与状态', () => {
  // stub listInstallations 返回 2 项 → SSR html 匹配名称、版本、active 徽标。
});
test('成员面板不显示管理操作', () => {
  // 同数据 → html 不匹配 /停用|启用|确认安装/。
});
```

- [ ] **Step 2: 运行确认失败**

Run: `pnpm gates`
Expected: FAIL —— 新方法/组件未实现。

- [ ] **Step 3: 实现**（api-client 方法 + 面板；PluginsPanel 走 `INTEGRATION_SECTIONS` 的 `plugins` section 渲染，管理操作按 `role` 隐藏——面板 props 不带 role 时默认只读展示，与 registry `minRole: 'viewer'` 一致）。

- [ ] **Step 4: 运行确认通过 + Commit**

Run: `pnpm gates`
Expected: 全部通过。
```bash
git add packages/api-client/src/plugins.ts packages/api-client/src/plugins.test.ts packages/views/src/integrations/registry.ts apps/web/src/settings/ apps/web/src/integrations/
git commit -m "feat(plugins): 安装确认/插件管理面板与成员发现页 [T08]"
```
