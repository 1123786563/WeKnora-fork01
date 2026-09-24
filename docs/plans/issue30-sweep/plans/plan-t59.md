# T29：Tenant Adoption、Agent Variant 与移动 Available Agent（Issue #59）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在已合并的 #58 Tenant Release 闭环之上实现 spec §8 的 Tenant 引入层：Tenant 对 Listing 建立 Adoption、同一 Adoption 派生多个独立 Agent Variant、完成本地能力映射与测试后发布本地 Agent Version，发布产物进入移动 Resource Shelf 实际消费的 `GET /api/v1/agents` 投影与新的领域一致 Available Agent 读模型；缺少必需能力时 Variant 不可运行且拒绝原因逐项点名。

**Architecture:** 全部为 Go 后端新增层 + contracts 契约镜像，不修改任何移动 TS 代码。数据层新增三张表（`agent_adoptions` / `agent_adoption_variants` / `agent_variant_capability_mappings`，双迁移流 000191/versioned 与 000112/sqlite）；仓储层 `internal/application/repository/agent_adoption.go` 拥有全部 SQL（参数绑定）并代理 marketplace 表只读；服务层 `internal/application/service/agent_adoption.go` 实现 `draft → mapped → tested → published` 状态机与能力覆盖裁决（Release Manifest 的 `capability_requirements` 减去已绑定本地资源的映射 = `missing_capabilities`，排序保证拒绝消息确定性），发布编排经两个窄 seam（`AdoptionAgentSource.CreateAgent`、`interfaces.AgentVersionService.FreezeAgentVersion`）实例化本地 Agent Definition 并冻结本地 Agent Version；HTTP 层沿用 #58 的 `/marketplace/tenant` 路由族（Admin 门禁 + apiKeyRoute 策略），移动侧证据落在端到端 router 测试：发布后的本地 Agent 出现在 #33 移动 Resource Shelf 实际消费的 `GET /api/v1/agents` 响应中（`internal/handler/custom_agent.go:268-271` 信封），且 Viewer 可读 `GET /marketplace/tenant/available-agents` 领域读模型。

**Tech Stack:** Go 1.26（`go.mod` module `github.com/Tencent/WeKnora`）、gin + gorm + golang-migrate（sqlite 测试流复用 `internal/router/routes_agent_marketplace_test.go:349-373` 的 `openTenantAgentMarketplaceHTTPTestDB` 与 `internal/application/service/agent_version_test.go:31-63` 的 `openAgentVersionServiceTestDB`）；TypeScript contracts（`packages/contracts`，node:test + tsx，与 `src/marketplace/tenant-releases.test.ts` 同款）。测试命令全部在 worktree 根执行。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-59.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-agent-marketplace-domain-model.md`（§3 聚合关系、§4 身份规则、§8 Adoption 与本地发布、§13 权限能力、§15 验收场景 1/2、§17 完成边界）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（Implementation/Testing Decisions；:195 已知缺口自认）
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§6 Resource Shelf、:383「后端尚需提供符合新领域模型的投影」）
- ADR：`docs/adr/0011-agent-marketplace-release-adoption-boundary.md`（Release 与 Adoption 边界，全文一句决策）
- 领域术语：`CONTEXT.md:122-135`（Agent 引入 / 本地能力映射 / Agent 变体 / 可用 Agent）
- Parent：Issue #30；Blocked by：#33（T03，已合并）、#58（T28，已合并）

## Global Constraints

以下为批准 Spec / Issue 的项目级约束，逐字引用，所有任务隐含遵守：

- 「移动端只显示已完成映射、测试和本地版本发布的 Available Agent。Marketplace 搜索、Submission、Review、Adoption 和升级不在移动端执行。」（agent-marketplace-domain-model.md §2）
- 「Agent Adoption 是 Tenant 接受 Listing 的治理关系，不是成员个人安装，也不是直接运行。」（同上 §8）
- 「采用流程：1. 管理员选择确定 Release；2. 检查 Deployment capability、许可、依赖和安全状态；3. Tenant 接受 Release，并建立或更新 Adoption；4. 创建固定到该 Release 的 Agent Variant 草稿；5. 管理员完成 Local Capability Mapping；6. Tenant Binding Preset 只作为可接受、替换或忽略的建议；7. 运行兼容性检查和 Tenant 测试；8. 发布本地 Agent Version；9. 成员获授权后，移动资源页才显示 Available Agent。」（同上 §8；第 2/6 步的 Deployment capability 检查器与 Binding Preset 实体不在本 Issue 范围，见「差异记录」）
- 「Local Capability Mapping 解析模型、知识、Skill、连接、Sandbox、审批和成员范围。它不能回写 Release，也不能让 Release 自带能力获得权限。」（同上 §8）
- 「同一 Adoption 可建立销售、法务等不同 Variant，并逐个采用不同 Release。每个 Variant 的来源、Release、映射和本地 Agent Version必须可追溯。」（同上 §8）
- 「Listing 是 Marketplace Agent 产品的稳定身份」「Adoption 在一个 Tenant 内对一个 Listing 唯一，保存该 Tenant 接受过的 Release」「一个 Adoption 可以派生多个 Agent Variant」「每个 Variant 固定一个来源 Release，拥有独立本地 Definition、映射、成员范围和发布节奏」「Task 固定本地 Agent Version，不直接运行 Marketplace Release」（同上 §4）
- 「Resource Shelf owns Available Agent, knowledge, connection and attachment selection. Marketplace governance remains on Web.」（mobile-ai-office-design.md · Implementation Decisions——本计划不在 apps/mobile 做任何 Adoption/Mapping 操作界面）
- 「Marketplace distributes immutable, sanitized Agent Releases with Manifest and Dependency Lock. Tenant Adoption creates local Variants that require mapping, testing and local Agent Version publication.」（同上）
- 「Tests target observable behavior at the highest stable Interface. Screen tests verify rendering and navigation; they do not duplicate Module internals.」（同上 · Testing Decisions——本计划的 Interface 测试是 Go router 级真实 HTTP）
- 安全约束（会话注入）：数据库查询所有外部输入使用参数绑定，不得拼接 SQL；服务端请求 URL 仅允许 http/https 且发出前校验 host（本计划不新增外呼请求）；配置凭据只从环境变量或密钥服务读取，源码与测试不写入可用凭据字面量（本计划无新凭据）。
- 工作流约束：严格 RED→GREEN→REFACTOR；实现与已批准 Spec 冲突时升级而非静默重设计；退出（Retire Variant / End Adoption / Unlist / Deprecate）属 #63、升级建议属 #61、跨 Tenant 公共目录属 #60、Fork 属 #62，本计划不得实现这些边界外的行为。

**Issue #59 验收标准原文（docs/plans/issue30-sweep/issues/issue-59.md）：**

1. 「同一 Adoption 支持多个独立 Variant。」
2. 「缺少必需能力时 Variant 不可运行且原因明确。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

验收标准 3 的本地可验证性说明：本计划最高稳定 Interface 证据 = `internal/router/routes_agent_adoption_test.go` 的真实 HTTP 端到端测试——真实 sqlite 迁移流（`openTenantAgentMarketplaceHTTPTestDB`，含本计划新增迁移）、真实仓储、真实 `CustomAgentService`/`AgentVersionService`/`AgentMarketplaceService`、真实 `GET /api/v1/agents` 列表端点（#33 移动 Resource Shelf 的 wire 来源），仅以测试惯用的 RBAC 中间件注入租户/角色上下文（与已合并的 #58 `TestTenantAgentMarketplaceLifecycleAndAuthorization` 同款）。无 blocked-env 项：本 Issue 不改移动 TS、不需要真机或外部凭据；服务测试（Task 3）与仓储测试（Task 2）如实标注为下层证据，不冒充 AC3。

## Review Focus

Spec 隐含但易咬人的五类输入/失效模式（每行后在所属任务以测试钉死）：

1. **跨租户 ID 枚举**：用他租户的 listing_id / adoption_id / variant_id 打本租户端点，必须与不存在同形（404），不得 403 泄露存在性。——Task 4 `TestTenantAgentAdoptionRoutesAndAuthorization`（cross-tenant adopt/variant 均 404）+ Task 2 仓储租户作用域测试。
2. **状态机外的重复操作**（重复 publish/test、并发 CAS 竞态）：第二次 `publish` 必须拒绝且不得再实例化第二个本地 Agent（否则移动投影出现重复 Agent）。——Task 3 服务测试（re-publish → `ErrAgentAdoptionStateConflict` 且 `agents.created` 长度不变）+ Task 6 HTTP 测试（重复 publish 409、`GET /api/v1/agents` 中该变体 Agent 恰好 1 行）。
3. **映射与 Manifest 不一致**：为 Release 未声明的 capability 提交映射、同一 capability 重复提交，必须 400 拒绝而非静默丢弃；空绑定（无 model/KB/connection）不算覆盖。——Task 3 `TestAgentAdoptionServiceRejectsUnknownAndDuplicateCapabilities` + Task 5 HTTP 测试。
4. **Release 字节被篡改**（digest 不匹配）：publish 必须在实例化本地 Agent 前拒绝（fail closed）。——Task 6 HTTP 测试（db 直改 bundle 字节 → publish → 500 拒绝，不产生本地 Agent）。
5. **已发布 Variant 被静默重映射 / 本地 Agent 被删后读模型残留**：published 状态拒绝 PUT mapping（409）；软删本地 Agent 后 `available-agents` 立即少一行、不渲染幽灵行。——Task 3 服务测试（published 再映射 409）+ Task 6 HTTP 测试（软删后 available-agents 为空）+ Task 2 仓储测试（软删 join 剔除）。

## 任务结构与文件地图

| # | 任务 | 主要交付 |
|---|---|---|
| 1 | 双流迁移 + 持久化实体 + 迁移同步测试 | 三张新表（versioned 000191 / sqlite 000112）与实体 |
| 2 | Adoption 仓储（含 marketplace 只读代理） | 参数绑定 SQL + upsert 式 AdoptListing + 状态 CAS |
| 3 | Adoption 服务（状态机 + 能力裁决 + 发布编排） | AC1/AC2 的领域逻辑（真实仓储 + 假 agent/version seam） |
| 4 | HTTP 面 I：Adopt / ListAdoptions / CreateVariant + 接线 | 治理写入端点与授权矩阵 |
| 5 | HTTP 面 II：CapabilityMapping / Test（AC2 拒绝与原因） | 映射替换 + 测试门禁端点 |
| 6 | HTTP 面 III：Publish / available-agents（AC1/AC3 端到端） | 发布端点 + 领域读模型 + `GET /api/v1/agents` 投影证据 |
| 7 | contracts 契约层 | Adoption/Variant/AvailableAgent wire 契约与解析器 |

**Consumes（前置批次已合并接口，本计划逐字依赖）：**
- #58：`interfaces.AgentMarketplaceRepository`（`internal/types/interfaces/agent_marketplace.go:8-17`，`GetRelease`/`GetListing` 签名）；`service.NewAgentMarketplaceService`（`internal/application/service/agent_marketplace.go:40`）；`interfaces.AgentVersionService.FreezeAgentVersion(ctx, tenantID, actorID, agentID) (AgentVersionView, error)`（`internal/types/interfaces/agent_version.go:16-28`）；路由/测试骨架 `internal/router/routes_agent_marketplace_test.go:49-373`（`openTenantAgentMarketplaceHTTPTestDB`、`marketplaceHTTPResolver`）；迁移模式 `migrations/versioned/000188_tenant_agent_marketplace.up.sql` 与 `migrations/sqlite/000109_tenant_agent_marketplace.up.sql`。
- #33：移动 Resource Shelf 的 Agent wire 事实——`packages/api-client/src/mobile/resources.ts:69-82` `availableAgents` 消费 `GET /api/v1/agents` 信封 `{ success, data: CustomAgent[], disabled_own_agent_ids }`（服务端形状 `internal/handler/custom_agent.go:268-271`），行字段 `id/name/description/is_builtin`。本计划 Task 6 端到端测试以该 wire 断言「进入移动 Resource Shelf」。
- #58 前置的 Agent 域：`service.NewCustomAgentService`（`internal/application/service/custom_agent.go:49-67`，`CreateAgent` 仅用 `repo.CreateAgent` + ctx，`:70-125`）与 `repository.NewCustomAgentRepository`。

**Produces（本计划对外接口，供 #60-#63 消费）：**
- Go 实体：`types.AgentAdoptionEntity` / `types.AgentAdoptionVariantEntity` / `types.AgentVariantCapabilityMappingEntity`（`internal/types/agent_adoption_persistence.go`）。
- 服务接口 `interfaces.AgentAdoptionService`（`internal/types/interfaces/agent_adoption.go`）：
  - `Adopt(ctx, tenantID uint64, actorID string, input AdoptInput) (AdoptionView, bool, error)`（`AdoptInput{ListingID, ReleaseID string}`；bool=是否新建）
  - `ListAdoptions(ctx, tenantID uint64) ([]AdoptionView, error)`
  - `CreateVariant(ctx, tenantID uint64, actorID, adoptionID string, input VariantDraftInput) (AdoptionVariantView, error)`（`VariantDraftInput{Name, ReleaseID string}`）
  - `UpdateCapabilityMapping(ctx, tenantID uint64, actorID, variantID string, mappings []CapabilityMapping) (AdoptionVariantView, error)`（`CapabilityMapping{Capability, ModelID string; KnowledgeBaseIDs, ConnectionIDs []string}`）
  - `TestVariant(ctx, tenantID uint64, actorID, variantID string) (AdoptionVariantView, error)`
  - `PublishVariant(ctx, tenantID uint64, actorID, variantID string) (PublishVariantResult, error)`
  - `ListAvailableAgents(ctx, tenantID uint64) ([]AvailableAgentView, error)`
  - 视图：`AdoptionView{types.AgentAdoptionEntity; Variants []AdoptionVariantView}`、`AdoptionVariantView{types.AgentAdoptionVariantEntity; MissingCapabilities []string}`、`PublishVariantResult{Variant AdoptionVariantView}`、`AvailableAgentView{AgentID, VariantID, AdoptionID, ReleaseID, Name, Description string; IsBuiltin bool; Capability AgentCapabilityVerdict}`、`AgentCapabilityVerdict{State, Reason string}`。
- Variant 状态常量（`internal/application/service/agent_adoption.go`）：`draft` / `mapped` / `tested` / `published`（spec §15 场景 1 的 "mapping-required" 由 `state="draft"` 且 `missing_capabilities≠[]` 表达；`retired` 属 #63）。
- HTTP：`POST|GET /api/v1/marketplace/tenant/adoptions`、`POST /api/v1/marketplace/tenant/adoptions/:id/variants`、`PUT /api/v1/marketplace/tenant/variants/:id/capability-mapping`、`POST /api/v1/marketplace/tenant/variants/:id/test`、`POST /api/v1/marketplace/tenant/variants/:id/publish`（以上 Admin+，apiKey full-access）、`GET /api/v1/marketplace/tenant/available-agents`（Viewer+，apiKey full-access）。
- contracts：`packages/contracts/src/marketplace/agent-adoption.ts` 导出 `AgentAdoption` / `AgentAdoptionVariant` / `AvailableAgent` 类型与 `parseAdoptionResponse` / `parseAdoptionListResponse` / `parseVariantResponse` / `parseAvailableAgentListResponse`。

**并行批次注意（B3 七计划并行，独立 worktree 后合并）：** 本计划对共享文件的修改共 4 处且均为最小增量——`internal/router/router.go`（Params 字段 + 1 行注册）、`internal/container/container.go`（3 行 Provide）、`internal/database/migration_sqlite_versioned_schema_test.go`（表/索引清单追加）、`packages/contracts/src/index.ts`（2 行 re-export）。迁移编号 versioned **000191** / sqlite **000112** 为本计划撰写时流头（`ls migrations/versioned | tail` 实测 000190、`ls migrations/sqlite | tail` 实测 000111）——若并行计划已占用同号，合并时由编排层统一重命名（两流内部各自单调即可，golang-migrate 只要求序列无洞）。

**前置条件：** worktree 根已可运行（本计划作者实测：`go test ./internal/router/ -run 'TestTenantAgentMarketplaceLifecycleAndAuthorization' -count=1` → ok 2.347s；`go build ./internal/...` → exit 0；`npx tsx --test packages/domain/src/mobile/agent-options.test.ts` → fail 0）。所有命令在 `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep` 根执行。

**差异记录（调查结论 vs 代码现状，以代码为准）：**
1. Issue 调查摘要称「移动 Available Agent 投影不存在：apps/mobile 无资源页」。代码现状：#33 已交付并合并 `apps/mobile/src/screens/ResourcesScreen.tsx`、`apps/mobile/src/app/resources.tsx`、`apps/mobile/src/resources-view.ts` 与 composition 接线（本计划作者亲读全文）。真实缺口是 Adoption/Variant 语义层（后端零代码），非页面缺失；本计划因此不动任何 apps/mobile 文件。
2. 调查摘要引用 spec:195「移动 Available Agent 投影未反映新 Marketplace 模型」。现状核实：投影链路存在（`GET /api/v1/agents` → `createMobileResourceRemote.availableAgents` → Resource Shelf），但其语义是「本租户全部 Agent」。本计划交付领域一致读模型端点 `GET /marketplace/tenant/available-agents` 并以端到端测试证明「发布后的 Variant 进入 #33 投影实际消费的 `GET /api/v1/agents`」；**不**翻转 `GET /api/v1/agents` 为仅 Adoption 派生 Agent——该端点同时服务 Web 会话下拉与 #33 已合并测试（`internal/router/routes_agent.go:39`、`packages/api-client/src/mobile/resources.ts:70`），单方面翻转会破坏 Web 现有功能，完整翻转属后续批次（#60 合入 Public 目录后统一迁移），此处按 Spec §17「完成边界」记为非目标。
3. spec §8 步骤 2「检查 Deployment capability、许可、依赖和安全状态」与步骤 6「Tenant Binding Preset」：部署能力检查器（许可/安全状态源）与 Binding Preset 实体在仓库尚不存在（`rg -l "BindingPreset|binding_preset" --type go` 无匹配）。本计划以「Release 必须真实存在于本租户 Listing 且 digest 完整」为发布前检查子集实现，Deployment capability 比对与 Preset 建议列为后续（依赖 #60/#61 的目录状态与升级差异面）。

---

### Task 1: 双流迁移与持久化实体

**Files:**
- Create: `migrations/versioned/000191_agent_adoption_variants.up.sql`
- Create: `migrations/versioned/000191_agent_adoption_variants.down.sql`
- Create: `migrations/sqlite/000112_agent_adoption_variants.up.sql`
- Create: `migrations/sqlite/000112_agent_adoption_variants.down.sql`
- Create: `internal/types/agent_adoption_persistence.go`
- Modify: `internal/database/migration_sqlite_versioned_schema_test.go:20-44`（`versionedSQLiteTables` 追加三表）、`:98-105`（首个索引清单追加两个唯一索引）

**Interfaces:**
- Consumes: #58 迁移模式（`migrations/versioned/000188_tenant_agent_marketplace.up.sql` 的表/索引/FK 写法与 `migrations/sqlite/000109_tenant_agent_marketplace.up.sql` 的 SQLite 方言：`BIGINT→INTEGER`、`TIMESTAMPTZ→DATETIME`、`NOW()→CURRENT_TIMESTAMP`、`BYTEA→BLOB`）；迁移同步测试清单（`internal/database/migration_sqlite_versioned_schema_test.go`，`agent_marketplace_listings` 等四表已在 `:40-43`）。
- Produces: 表 `agent_adoptions`（PK `(id, tenant_id)`，`uq_agent_adoptions_scope(tenant_id, listing_id)`）、`agent_adoption_variants`（PK `(id, tenant_id)`，FK→adoptions/releases，无 FK→custom_agents——本地 Agent 软删时读模型自然剔除）、`agent_variant_capability_mappings`（PK `(id, tenant_id)`，`uq_agent_variant_capability(tenant_id, variant_id, capability)`）；Go 实体三枚（含 `TableName()`，模式同 `internal/types/agent_marketplace_persistence.go`）。

- [ ] **Step 1: 写失败测试——迁移同步清单先要求新表**

修改 `internal/database/migration_sqlite_versioned_schema_test.go`：在 `versionedSQLiteTables` 切片（`:20` 起）的 `"agent_releases",` 之后追加：

```go
	"agent_adoptions",
	"agent_adoption_variants",
	"agent_variant_capability_mappings",
```

在 `TestSQLiteMigrationsCreateVersionedSchema` 的索引断言清单（`:98` 起，`"uq_agent_releases_digest",` 之后）追加：

```go
		"uq_agent_adoptions_scope", "uq_agent_variant_capability",
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/database/ -run 'TestSQLiteMigrationsCreateVersionedSchema' -count=1`
Expected: FAIL，断言消息形如 `SQLite migrations must create table agent_adoptions`（新迁移尚不存在）。

- [ ] **Step 3: 写迁移与实体（最小实现）**

`migrations/versioned/000191_agent_adoption_variants.up.sql`：

```sql
-- T29 Tenant Adoption, Agent Variant and local capability mapping governance tables.
CREATE TABLE agent_adoptions (
 id VARCHAR(36) NOT NULL, tenant_id BIGINT NOT NULL, listing_id VARCHAR(36) NOT NULL,
 accepted_release_id VARCHAR(36) NOT NULL, state VARCHAR(32) NOT NULL DEFAULT 'active',
 created_by VARCHAR(255) NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY (id, tenant_id),
 FOREIGN KEY (listing_id, tenant_id) REFERENCES agent_marketplace_listings(id, tenant_id),
 FOREIGN KEY (accepted_release_id, tenant_id) REFERENCES agent_releases(id, tenant_id)
);
CREATE TABLE agent_adoption_variants (
 id VARCHAR(36) NOT NULL, tenant_id BIGINT NOT NULL, adoption_id VARCHAR(36) NOT NULL, release_id VARCHAR(36) NOT NULL,
 name VARCHAR(255) NOT NULL, state VARCHAR(32) NOT NULL DEFAULT 'draft',
 local_agent_id VARCHAR(36), local_agent_version_id VARCHAR(36),
 created_by VARCHAR(255) NOT NULL DEFAULT '', tested_by VARCHAR(255) NOT NULL DEFAULT '', tested_at TIMESTAMPTZ,
 published_by VARCHAR(255) NOT NULL DEFAULT '', published_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY (id, tenant_id),
 FOREIGN KEY (adoption_id, tenant_id) REFERENCES agent_adoptions(id, tenant_id),
 FOREIGN KEY (release_id, tenant_id) REFERENCES agent_releases(id, tenant_id)
);
CREATE TABLE agent_variant_capability_mappings (
 id VARCHAR(36) NOT NULL, tenant_id BIGINT NOT NULL, variant_id VARCHAR(36) NOT NULL,
 capability VARCHAR(255) NOT NULL, model_id VARCHAR(255) NOT NULL DEFAULT '',
 knowledge_base_ids TEXT NOT NULL DEFAULT '[]', connection_ids TEXT NOT NULL DEFAULT '[]',
 updated_by VARCHAR(255) NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY (id, tenant_id),
 FOREIGN KEY (variant_id, tenant_id) REFERENCES agent_adoption_variants(id, tenant_id)
);
CREATE INDEX idx_agent_adoption_variants_adoption ON agent_adoption_variants(tenant_id, adoption_id, created_at);
CREATE INDEX idx_agent_variant_capability_variant ON agent_variant_capability_mappings(tenant_id, variant_id);
CREATE UNIQUE INDEX uq_agent_adoptions_scope ON agent_adoptions(tenant_id, listing_id);
CREATE UNIQUE INDEX uq_agent_variant_capability ON agent_variant_capability_mappings(tenant_id, variant_id, capability);
```

`migrations/versioned/000191_agent_adoption_variants.down.sql`：

```sql
DROP TABLE IF EXISTS agent_variant_capability_mappings;
DROP TABLE IF EXISTS agent_adoption_variants;
DROP TABLE IF EXISTS agent_adoptions;
```

`migrations/sqlite/000112_agent_adoption_variants.up.sql`（SQLite twin，方言对齐 000109）：

```sql
-- SQLite twin of versioned migration 000191.
CREATE TABLE agent_adoptions (
 id VARCHAR(36) NOT NULL, tenant_id INTEGER NOT NULL, listing_id VARCHAR(36) NOT NULL,
 accepted_release_id VARCHAR(36) NOT NULL, state VARCHAR(32) NOT NULL DEFAULT 'active',
 created_by VARCHAR(255) NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY (id, tenant_id),
 FOREIGN KEY (listing_id, tenant_id) REFERENCES agent_marketplace_listings(id, tenant_id),
 FOREIGN KEY (accepted_release_id, tenant_id) REFERENCES agent_releases(id, tenant_id)
);
CREATE TABLE agent_adoption_variants (
 id VARCHAR(36) NOT NULL, tenant_id INTEGER NOT NULL, adoption_id VARCHAR(36) NOT NULL, release_id VARCHAR(36) NOT NULL,
 name VARCHAR(255) NOT NULL, state VARCHAR(32) NOT NULL DEFAULT 'draft',
 local_agent_id VARCHAR(36), local_agent_version_id VARCHAR(36),
 created_by VARCHAR(255) NOT NULL DEFAULT '', tested_by VARCHAR(255) NOT NULL DEFAULT '', tested_at DATETIME,
 published_by VARCHAR(255) NOT NULL DEFAULT '', published_at DATETIME,
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY (id, tenant_id),
 FOREIGN KEY (adoption_id, tenant_id) REFERENCES agent_adoptions(id, tenant_id),
 FOREIGN KEY (release_id, tenant_id) REFERENCES agent_releases(id, tenant_id)
);
CREATE TABLE agent_variant_capability_mappings (
 id VARCHAR(36) NOT NULL, tenant_id INTEGER NOT NULL, variant_id VARCHAR(36) NOT NULL,
 capability VARCHAR(255) NOT NULL, model_id VARCHAR(255) NOT NULL DEFAULT '',
 knowledge_base_ids TEXT NOT NULL DEFAULT '[]', connection_ids TEXT NOT NULL DEFAULT '[]',
 updated_by VARCHAR(255) NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY (id, tenant_id),
 FOREIGN KEY (variant_id, tenant_id) REFERENCES agent_adoption_variants(id, tenant_id)
);
CREATE INDEX idx_agent_adoption_variants_adoption ON agent_adoption_variants(tenant_id, adoption_id, created_at);
CREATE INDEX idx_agent_variant_capability_variant ON agent_variant_capability_mappings(tenant_id, variant_id);
CREATE UNIQUE INDEX uq_agent_adoptions_scope ON agent_adoptions(tenant_id, listing_id);
CREATE UNIQUE INDEX uq_agent_variant_capability ON agent_variant_capability_mappings(tenant_id, variant_id, capability);
```

`migrations/sqlite/000112_agent_adoption_variants.down.sql`：

```sql
DROP TABLE IF EXISTS agent_variant_capability_mappings;
DROP TABLE IF EXISTS agent_adoption_variants;
DROP TABLE IF EXISTS agent_adoptions;
```

`internal/types/agent_adoption_persistence.go`：

```go
package types

import "time"

// Agent Adoption persistence entities (T29, Ticket #59).
//
// The Marketplace owns Adoption, Variant and the local capability mapping
// rows (docs/specs/2026-09-20-agent-marketplace-domain-model.md §4, §8).
// The local Agent Definition and Agent Version stay in the Agent domain:
// agent_adoption_variants references them by ID only, with no FK, so a
// soft-deleted local agent simply disappears from the available-agent read
// model instead of blocking governance history.

// AgentAdoptionEntity is the Tenant's governance relationship to one
// Listing: unique per (tenant, listing), holding the accepted Release.
type AgentAdoptionEntity struct {
	ID                string `gorm:"type:varchar(36);primaryKey"`
	TenantID          uint64 `gorm:"primaryKey"`
	ListingID         string `gorm:"type:varchar(36);not null"`
	AcceptedReleaseID string `gorm:"type:varchar(36);not null"`
	State             string `gorm:"type:varchar(32);not null;default:'active'"`
	CreatedBy         string `gorm:"type:varchar(255);not null;default:''"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (AgentAdoptionEntity) TableName() string { return "agent_adoptions" }

// AgentAdoptionVariantEntity is one local Variant derived from an Adoption,
// pinned to one immutable Release. State machine: draft -> mapped -> tested
// -> published (retire belongs to #63). LocalAgentID/LocalAgentVersionID are
// set exactly once, at publication.
type AgentAdoptionVariantEntity struct {
	ID                  string `gorm:"type:varchar(36);primaryKey"`
	TenantID            uint64 `gorm:"primaryKey"`
	AdoptionID          string `gorm:"type:varchar(36);not null"`
	ReleaseID           string `gorm:"type:varchar(36);not null"`
	Name                string `gorm:"type:varchar(255);not null"`
	State               string `gorm:"type:varchar(32);not null;default:'draft'"`
	LocalAgentID        string `gorm:"type:varchar(36)"`
	LocalAgentVersionID string `gorm:"type:varchar(36)"`
	CreatedBy           string `gorm:"type:varchar(255);not null;default:''"`
	TestedBy            string `gorm:"type:varchar(255);not null;default:''"`
	TestedAt            *time.Time
	PublishedBy         string `gorm:"type:varchar(255);not null;default:''"`
	PublishedAt         *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (AgentAdoptionVariantEntity) TableName() string { return "agent_adoption_variants" }

// AgentVariantCapabilityMappingEntity is one local capability binding of a
// Variant: the Manifest capability name mapped to local model / knowledge /
// connection choices. ID lists are canonical JSON arrays ("[]"/["id",...]).
type AgentVariantCapabilityMappingEntity struct {
	ID               string `gorm:"type:varchar(36);primaryKey"`
	TenantID         uint64 `gorm:"primaryKey"`
	VariantID        string `gorm:"type:varchar(36);not null"`
	Capability       string `gorm:"type:varchar(255);not null"`
	ModelID          string `gorm:"type:varchar(255);not null;default:''"`
	KnowledgeBaseIDs string `gorm:"type:text;not null;default:'[]'"`
	ConnectionIDs    string `gorm:"type:text;not null;default:'[]'"`
	UpdatedBy        string `gorm:"type:varchar(255);not null;default:''"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (AgentVariantCapabilityMappingEntity) TableName() string { return "agent_variant_capability_mappings" }
```

- [ ] **Step 4: 运行确认通过（含 up/down/up 与升级路径）**

Run: `go test ./internal/database/ -run 'TestSQLiteMigrations|TestSemanticMigrationSQLiteUpDownUp' -count=1`
Expected: PASS（三表、两索引在全新库与 v4 升级库均存在；down 迁移可回放）。

- [ ] **Step 5: Commit**

```bash
git add migrations/versioned/000191_agent_adoption_variants.up.sql migrations/versioned/000191_agent_adoption_variants.down.sql migrations/sqlite/000112_agent_adoption_variants.up.sql migrations/sqlite/000112_agent_adoption_variants.down.sql internal/types/agent_adoption_persistence.go internal/database/migration_sqlite_versioned_schema_test.go
git commit -m "feat(marketplace): agent adoption/variant/capability-mapping tables (T29 #59 task 1)"
```

---

### Task 2: Adoption 仓储

**Files:**
- Create: `internal/application/repository/agent_adoption.go`
- Test: `internal/application/repository/agent_adoption_test.go`

**Interfaces:**
- Consumes: Task 1 三实体；包内既有 `openRunTestDB`（`internal/application/repository/agent_run_test.go:29-61`，完整 sqlite 迁移流）与 marketplace 仓储 `NewAgentMarketplaceRepository`（`internal/application/repository/agent_marketplace.go:53`，用于种出真实 Release）。
- Produces: `repository.AgentAdoptionRepository` 接口（见下方实现顶部，含 `AdoptListing`/`GetAdoption`/`ListAdoptions`/`CreateVariant`/`GetVariant`/`ListVariantsByAdoption`/`ReplaceCapabilityMappings`/`ListCapabilityMappings`/`UpdateVariantState`/`PublishedAvailableAgents`/`GetMarketplaceListing`/`GetRelease`）；哨兵 `repository.ErrAgentAdoptionNotFound`、`repository.ErrAgentAdoptionVariantTransition`；行类型 `repository.AgentAdoptionPublishedRow{Variant types.AgentAdoptionVariantEntity; Agent *types.CustomAgent}`。

- [ ] **Step 1: 写失败测试**

`internal/application/repository/agent_adoption_test.go`：

```go
package repository

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedAdoptionRelease drives the REAL marketplace repository to publish one
// immutable Release whose Manifest declares capability requirements, so the
// adoption rows under test bind to genuine FK-respecting marketplace data.
func seedAdoptionRelease(t *testing.T, db *gorm.DB, tenantID uint64, agentID, semanticVersion string) (listingID, releaseID string) {
	t.Helper()
	versionID := agentID + "-v1"
	require.NoError(t, db.Exec(
		`INSERT INTO agent_versions (id, tenant_id, agent_id, version_number, snapshot, source_sha256, frozen_by) VALUES (?, ?, ?, 1, '{}', 'sha', 'author')`,
		versionID, tenantID, agentID,
	).Error)
	manifest := `{"semantic_version":"` + semanticVersion + `","display_name":"Listing ` + agentID + `","summary":"s","supported_languages":["en"],"use_cases":["u"],"capability_requirements":["model","knowledge"],"minimum_weknora_capability":"1","license_id":"MIT","source":{"agent_version_id":"` + versionID + `","version_number":1,"source_sha256":"sha"}}`
	bundle := []byte(`{"payload":{"agent_mode":"quick-answer","system_prompt":"p"},"manifest":` + manifest + `,"dependency_lock":{"dependencies":[]}}`)
	repo := NewAgentMarketplaceRepository(db)
	submission, err := repo.CreateSubmission(context.Background(),
		&types.AgentMarketplaceListingEntity{TenantID: tenantID, SourceAgentID: agentID, DisplayName: "Listing " + agentID, Summary: "s", State: "listed"},
		&types.AgentReleaseSubmissionEntity{
			TenantID: tenantID, AgentVersionID: versionID, SourceAgentID: agentID, AuthorID: "author",
			SemanticVersion: semanticVersion, BundleDigest: "digest-" + agentID,
			ManifestJSON: manifest, DependencyLockJSON: `{"dependencies":[]}`,
			Bundle: bundle, Status: "submitted",
		})
	require.NoError(t, err)
	_, release, err := repo.ReviewAndPublishTx(context.Background(), tenantID, "", submission.ID, "digest-"+agentID, types.AgentReleaseReviewDecision{ReviewerID: "reviewer", Decision: "approved"})
	require.NoError(t, err)
	require.NotNil(t, release)
	return submission.ListingID, release.ID
}

func TestAgentAdoptionRepositoryLifecycle(t *testing.T) {
	db := openRunTestDB(t)
	listingID, releaseID := seedAdoptionRelease(t, db, 1, "agent-a", "1.0.0")
	repo := NewAgentAdoptionRepository(db)
	ctx := context.Background()

	adopted, created, err := repo.AdoptListing(ctx, &types.AgentAdoptionEntity{TenantID: 1, ListingID: listingID, AcceptedReleaseID: releaseID, State: "active", CreatedBy: "admin"})
	require.NoError(t, err)
	require.True(t, created)
	require.NotEmpty(t, adopted.ID)
	again, createdAgain, err := repo.AdoptListing(ctx, &types.AgentAdoptionEntity{TenantID: 1, ListingID: listingID, AcceptedReleaseID: releaseID, State: "active", CreatedBy: "admin"})
	require.NoError(t, err)
	require.False(t, createdAgain, "re-adopting the same listing is idempotent")
	require.Equal(t, adopted.ID, again.ID)

	// Marketplace read proxies are tenant-scoped: another tenant reads absence.
	listing, err := repo.GetMarketplaceListing(ctx, 1, listingID)
	require.NoError(t, err)
	require.NotNil(t, listing)
	listing, err = repo.GetMarketplaceListing(ctx, 2, listingID)
	require.NoError(t, err)
	require.Nil(t, listing)
	release, err := repo.GetRelease(ctx, 2, releaseID)
	require.NoError(t, err)
	require.Nil(t, release)

	variant, err := repo.CreateVariant(ctx, &types.AgentAdoptionVariantEntity{TenantID: 1, AdoptionID: adopted.ID, ReleaseID: releaseID, Name: "Sales", State: "draft", CreatedBy: "admin"})
	require.NoError(t, err)
	secondVariant, err := repo.CreateVariant(ctx, &types.AgentAdoptionVariantEntity{TenantID: 1, AdoptionID: adopted.ID, ReleaseID: releaseID, Name: "Legal", State: "draft", CreatedBy: "admin"})
	require.NoError(t, err)
	require.NotEqual(t, variant.ID, secondVariant.ID, "one adoption owns multiple independent variants")
	variants, err := repo.ListVariantsByAdoption(ctx, 1, adopted.ID)
	require.NoError(t, err)
	require.Len(t, variants, 2)
	stored, err := repo.GetVariant(ctx, 1, variant.ID)
	require.NoError(t, err)
	require.Equal(t, "draft", stored.State)
	foreignVariant, err := repo.GetVariant(ctx, 2, variant.ID)
	require.NoError(t, err)
	require.Nil(t, foreignVariant)

	require.NoError(t, repo.ReplaceCapabilityMappings(ctx, 1, variant.ID, []types.AgentVariantCapabilityMappingEntity{
		{TenantID: 1, VariantID: variant.ID, Capability: "model", ModelID: "gpt-x", KnowledgeBaseIDs: "[]", ConnectionIDs: "[]", UpdatedBy: "admin"},
	}, "draft"))
	require.NoError(t, repo.ReplaceCapabilityMappings(ctx, 1, variant.ID, []types.AgentVariantCapabilityMappingEntity{
		{TenantID: 1, VariantID: variant.ID, Capability: "model", ModelID: "gpt-x", KnowledgeBaseIDs: "[]", ConnectionIDs: "[]", UpdatedBy: "admin"},
		{TenantID: 1, VariantID: variant.ID, Capability: "knowledge", KnowledgeBaseIDs: `["kb-1"]`, ConnectionIDs: "[]", UpdatedBy: "admin"},
	}, "mapped"))
	rows, err := repo.ListCapabilityMappings(ctx, 1, variant.ID)
	require.NoError(t, err)
	require.Len(t, rows, 2, "replacement must not accumulate stale rows")
	stored, err = repo.GetVariant(ctx, 1, variant.ID)
	require.NoError(t, err)
	require.Equal(t, "mapped", stored.State)
	err = repo.ReplaceCapabilityMappings(ctx, 1, "missing-variant", nil, "draft")
	require.ErrorIs(t, err, ErrAgentAdoptionNotFound, "replacing mappings of an unknown variant must fail as not-found")

	tested, err := repo.UpdateVariantState(ctx, 1, variant.ID, []string{"mapped"}, "tested", map[string]any{"tested_by": "admin"})
	require.NoError(t, err)
	require.Equal(t, "tested", tested.State)
	require.Equal(t, "admin", tested.TestedBy)
	require.NotNil(t, tested.TestedAt)
	_, err = repo.UpdateVariantState(ctx, 1, variant.ID, []string{"mapped"}, "tested", nil)
	require.ErrorIs(t, err, ErrAgentAdoptionVariantTransition, "a stale from-state must refuse the transition")
	_, err = repo.UpdateVariantState(ctx, 1, "missing-variant", []string{"mapped"}, "tested", nil)
	require.ErrorIs(t, err, ErrAgentAdoptionNotFound)

	adoptions, err := repo.ListAdoptions(ctx, 1)
	require.NoError(t, err)
	require.Len(t, adoptions, 1)
	foreignAdoption, err := repo.GetAdoption(ctx, 2, adopted.ID)
	require.NoError(t, err)
	require.Nil(t, foreignAdoption)
}

func TestAgentAdoptionPublishedAvailableAgentsJoinsLocalAgents(t *testing.T) {
	db := openRunTestDB(t)
	listingID, releaseID := seedAdoptionRelease(t, db, 1, "agent-b", "1.0.0")
	repo := NewAgentAdoptionRepository(db)
	ctx := context.Background()
	adopted, _, err := repo.AdoptListing(ctx, &types.AgentAdoptionEntity{TenantID: 1, ListingID: listingID, AcceptedReleaseID: releaseID, State: "active", CreatedBy: "admin"})
	require.NoError(t, err)
	published, err := repo.CreateVariant(ctx, &types.AgentAdoptionVariantEntity{TenantID: 1, AdoptionID: adopted.ID, ReleaseID: releaseID, Name: "Sales", State: "published", LocalAgentID: "agent-sales", CreatedBy: "admin"})
	require.NoError(t, err)

	rows, err := repo.PublishedAvailableAgents(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, rows, "a published variant whose local agent row is absent yields no available agent")

	require.NoError(t, db.Create(&types.CustomAgent{ID: "agent-sales", TenantID: 1, Name: "Sales Assistant", CreatedBy: "admin", Config: types.CustomAgentConfig{AgentMode: "quick-answer"}}).Error)
	rows, err = repo.PublishedAvailableAgents(ctx, 1)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, published.ID, rows[0].Variant.ID)
	require.NotNil(t, rows[0].Agent)
	require.Equal(t, "agent-sales", rows[0].Agent.ID)

	require.NoError(t, db.Delete(&types.CustomAgent{}, "tenant_id = ? AND id = ?", 1, "agent-sales").Error)
	rows, err = repo.PublishedAvailableAgents(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, rows, "a soft-deleted local agent disappears from the read model")

	rows, err = repo.PublishedAvailableAgents(ctx, 2)
	require.NoError(t, err)
	require.Empty(t, rows, "another tenant never sees tenant-1 availability")
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/application/repository/ -run 'TestAgentAdoption' -count=1`
Expected: FAIL（构建失败）——`undefined: NewAgentAdoptionRepository`。

- [ ] **Step 3: 写最小实现**

`internal/application/repository/agent_adoption.go`：

```go
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Tencent/WeKnora/internal/types"
)

var (
	ErrAgentAdoptionNotFound           = errors.New("agent adoption resource not found")
	ErrAgentAdoptionVariantTransition  = errors.New("agent adoption variant state transition failed")
)

// AgentAdoptionPublishedRow pairs one published Variant with its live local
// agent row. Agent is nil when the local agent is gone (soft-deleted): the
// available-agent read model then simply omits the row.
type AgentAdoptionPublishedRow struct {
	Variant types.AgentAdoptionVariantEntity
	Agent   *types.CustomAgent
}

// AgentAdoptionRepository owns the Adoption/Variant/mapping SQL. All reads
// and writes are tenant-scoped and parameter-bound; the two marketplace
// getters are read-only proxies so the adoption service never re-implements
// marketplace queries.
type AgentAdoptionRepository interface {
	AdoptListing(context.Context, *types.AgentAdoptionEntity) (*types.AgentAdoptionEntity, bool, error)
	GetAdoption(context.Context, uint64, string) (*types.AgentAdoptionEntity, error)
	ListAdoptions(context.Context, uint64) ([]types.AgentAdoptionEntity, error)
	CreateVariant(context.Context, *types.AgentAdoptionVariantEntity) (*types.AgentAdoptionVariantEntity, error)
	GetVariant(context.Context, uint64, string) (*types.AgentAdoptionVariantEntity, error)
	ListVariantsByAdoption(context.Context, uint64, string) ([]types.AgentAdoptionVariantEntity, error)
	ReplaceCapabilityMappings(context.Context, uint64, string, []types.AgentVariantCapabilityMappingEntity, string) error
	ListCapabilityMappings(context.Context, uint64, string) ([]types.AgentVariantCapabilityMappingEntity, error)
	UpdateVariantState(context.Context, uint64, string, []string, string, map[string]any) (*types.AgentAdoptionVariantEntity, error)
	PublishedAvailableAgents(context.Context, uint64) ([]AgentAdoptionPublishedRow, error)
	GetMarketplaceListing(context.Context, uint64, string) (*types.AgentMarketplaceListingEntity, error)
	GetRelease(context.Context, uint64, string) (*types.AgentReleaseEntity, error)
}

type agentAdoptionRepository struct{ db *gorm.DB }

func NewAgentAdoptionRepository(db *gorm.DB) AgentAdoptionRepository {
	return &agentAdoptionRepository{db: db}
}

// AdoptListing inserts the (tenant, listing)-unique Adoption or returns the
// existing row; re-adopting with a different release advances the accepted
// pointer (spec §8 step 3 "建立或更新 Adoption").
func (r *agentAdoptionRepository) AdoptListing(ctx context.Context, adoption *types.AgentAdoptionEntity) (*types.AgentAdoptionEntity, bool, error) {
	var existing types.AgentAdoptionEntity
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND listing_id = ?", adoption.TenantID, adoption.ListingID).First(&existing).Error
	if err == nil {
		if existing.AcceptedReleaseID == adoption.AcceptedReleaseID {
			return &existing, false, nil
		}
		existing.AcceptedReleaseID = adoption.AcceptedReleaseID
		existing.UpdatedAt = time.Now().UTC()
		if err := r.db.WithContext(ctx).Model(&types.AgentAdoptionEntity{}).
			Where("tenant_id = ? AND id = ?", existing.TenantID, existing.ID).
			Updates(map[string]any{"accepted_release_id": existing.AcceptedReleaseID, "updated_at": existing.UpdatedAt}).Error; err != nil {
			return nil, false, err
		}
		return &existing, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	created := *adoption
	created.ID = uuid.NewString()
	created.CreatedAt = time.Now().UTC()
	created.UpdatedAt = created.CreatedAt
	if created.State == "" {
		created.State = "active"
	}
	if err := r.db.WithContext(ctx).Create(&created).Error; err != nil {
		return nil, false, err
	}
	return &created, true, nil
}

func (r *agentAdoptionRepository) GetAdoption(ctx context.Context, tenantID uint64, adoptionID string) (*types.AgentAdoptionEntity, error) {
	var row types.AgentAdoptionEntity
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(adoptionID)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *agentAdoptionRepository) ListAdoptions(ctx context.Context, tenantID uint64) ([]types.AgentAdoptionEntity, error) {
	rows := []types.AgentAdoptionEntity{}
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at ASC, id ASC").Find(&rows).Error
	return rows, err
}

func (r *agentAdoptionRepository) CreateVariant(ctx context.Context, variant *types.AgentAdoptionVariantEntity) (*types.AgentAdoptionVariantEntity, error) {
	if variant == nil || variant.TenantID == 0 || strings.TrimSpace(variant.AdoptionID) == "" || strings.TrimSpace(variant.ReleaseID) == "" || strings.TrimSpace(variant.Name) == "" {
		return nil, fmt.Errorf("invalid agent adoption variant")
	}
	created := *variant
	created.ID = uuid.NewString()
	created.Name = strings.TrimSpace(created.Name)
	if created.State == "" {
		created.State = "draft"
	}
	created.CreatedAt = time.Now().UTC()
	created.UpdatedAt = created.CreatedAt
	if err := r.db.WithContext(ctx).Create(&created).Error; err != nil {
		return nil, err
	}
	return &created, nil
}

func (r *agentAdoptionRepository) GetVariant(ctx context.Context, tenantID uint64, variantID string) (*types.AgentAdoptionVariantEntity, error) {
	var row types.AgentAdoptionVariantEntity
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(variantID)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *agentAdoptionRepository) ListVariantsByAdoption(ctx context.Context, tenantID uint64, adoptionID string) ([]types.AgentAdoptionVariantEntity, error) {
	rows := []types.AgentAdoptionVariantEntity{}
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND adoption_id = ?", tenantID, strings.TrimSpace(adoptionID)).Order("created_at ASC, id ASC").Find(&rows).Error
	return rows, err
}

// ReplaceCapabilityMappings atomically swaps the Variant's mapping rows and
// records the recomputed state (draft while incomplete, mapped once every
// required capability is bound). One transaction, all parameters bound.
func (r *agentAdoptionRepository) ReplaceCapabilityMappings(ctx context.Context, tenantID uint64, variantID string, mappings []types.AgentVariantCapabilityMappingEntity, nextState string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tenant_id = ? AND variant_id = ?", tenantID, strings.TrimSpace(variantID)).Delete(&types.AgentVariantCapabilityMappingEntity{}).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		for i := range mappings {
			mappings[i].ID = uuid.NewString()
			mappings[i].TenantID = tenantID
			mappings[i].VariantID = strings.TrimSpace(variantID)
			mappings[i].CreatedAt = now
			mappings[i].UpdatedAt = now
			if err := tx.Create(&mappings[i]).Error; err != nil {
				return err
			}
		}
		updated := tx.Model(&types.AgentAdoptionVariantEntity{}).
			Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(variantID)).
			Updates(map[string]any{"state": nextState, "updated_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrAgentAdoptionNotFound
		}
		return nil
	})
}

func (r *agentAdoptionRepository) ListCapabilityMappings(ctx context.Context, tenantID uint64, variantID string) ([]types.AgentVariantCapabilityMappingEntity, error) {
	rows := []types.AgentVariantCapabilityMappingEntity{}
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND variant_id = ?", tenantID, strings.TrimSpace(variantID)).Order("capability ASC").Find(&rows).Error
	return rows, err
}

// UpdateVariantState is the compare-and-set transition: the update applies
// only when the current state is one of expectedFrom, so a concurrent or
// repeated transition fails loudly instead of double-applying.
func (r *agentAdoptionRepository) UpdateVariantState(ctx context.Context, tenantID uint64, variantID string, expectedFrom []string, nextState string, updates map[string]any) (*types.AgentAdoptionVariantEntity, error) {
	set := map[string]any{"state": nextState, "updated_at": time.Now().UTC()}
	for key, value := range updates {
		set[key] = value
	}
	result := r.db.WithContext(ctx).Model(&types.AgentAdoptionVariantEntity{}).
		Where("tenant_id = ? AND id = ? AND state IN ?", tenantID, strings.TrimSpace(variantID), expectedFrom).
		Updates(set)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		var current types.AgentAdoptionVariantEntity
		err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(variantID)).First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentAdoptionNotFound
		}
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: variant %s is %q, expected one of %v", ErrAgentAdoptionVariantTransition, variantID, current.State, expectedFrom)
	}
	return r.GetVariant(ctx, tenantID, variantID)
}

// PublishedAvailableAgents returns the adoption-aware available-agent read
// model: published Variants of active Adoptions, joined with their live
// local agent rows (soft-deleted agents drop out via gorm's DeletedAt
// default scope).
func (r *agentAdoptionRepository) PublishedAvailableAgents(ctx context.Context, tenantID uint64) ([]AgentAdoptionPublishedRow, error) {
	adoptions := []types.AgentAdoptionEntity{}
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND state = ?", tenantID, "active").Find(&adoptions).Error; err != nil {
		return nil, err
	}
	active := make(map[string]bool, len(adoptions))
	for _, adoption := range adoptions {
		active[adoption.ID] = true
	}
	variants := []types.AgentAdoptionVariantEntity{}
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND state = ?", tenantID, "published").Order("created_at ASC, id ASC").Find(&variants).Error; err != nil {
		return nil, err
	}
	agentIDs := make([]string, 0, len(variants))
	for _, variant := range variants {
		if active[variant.AdoptionID] && variant.LocalAgentID != "" {
			agentIDs = append(agentIDs, variant.LocalAgentID)
		}
	}
	agents := map[string]*types.CustomAgent{}
	if len(agentIDs) > 0 {
		rows := []types.CustomAgent{}
		if err := r.db.WithContext(ctx).Where("tenant_id = ? AND id IN ?", tenantID, agentIDs).Find(&rows).Error; err != nil {
			return nil, err
		}
		for i := range rows {
			agents[rows[i].ID] = &rows[i]
		}
	}
	out := []AgentAdoptionPublishedRow{}
	for _, variant := range variants {
		if active[variant.AdoptionID] {
			out = append(out, AgentAdoptionPublishedRow{Variant: variant, Agent: agents[variant.LocalAgentID]})
		}
	}
	return out, nil
}

func (r *agentAdoptionRepository) GetMarketplaceListing(ctx context.Context, tenantID uint64, listingID string) (*types.AgentMarketplaceListingEntity, error) {
	var row types.AgentMarketplaceListingEntity
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(listingID)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *agentAdoptionRepository) GetRelease(ctx context.Context, tenantID uint64, releaseID string) (*types.AgentReleaseEntity, error) {
	var row types.AgentReleaseEntity
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(releaseID)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/application/repository/ -run 'TestAgentAdoption' -count=1 -v`
Expected: PASS（2 个测试）。

- [ ] **Step 5: Commit**

```bash
git add internal/application/repository/agent_adoption.go internal/application/repository/agent_adoption_test.go
git commit -m "feat(marketplace): agent adoption repository with tenant-scoped CAS transitions (T29 #59 task 2)"
```

---

### Task 3: Adoption 服务（状态机、能力裁决与发布编排）

**Files:**
- Create: `internal/types/interfaces/agent_adoption.go`
- Create: `internal/application/service/agent_adoption.go`
- Test: `internal/application/service/agent_adoption_test.go`

**Interfaces:**
- Consumes: Task 2 `repository.AgentAdoptionRepository` 全部方法与哨兵；`interfaces.AgentVersionService.FreezeAgentVersion`（#58，`internal/types/interfaces/agent_version.go:22`）；`types.AgentReleasePayload`（`internal/types/agent_marketplace.go:122-140`）与 `types.AgentReleaseManifest`（`:81-85`，嵌入 `ReleaseMetadata.CapabilityRequirements`，wire key `capability_requirements`，`:46`）；服务包内既有 `openAgentVersionServiceTestDB`（`internal/application/service/agent_version_test.go:31-63`）。
- Produces: 见计划头部 Produces 的 `interfaces.AgentAdoptionService` 全部签名与视图类型；哨兵 `service.ErrAgentAdoptionInvalidInput` / `ErrAgentAdoptionNotFound` / `ErrAgentAdoptionVariantNotRunnable` / `ErrAgentAdoptionStateConflict`；状态常量 `AgentVariantStateDraft/Mapped/Tested/Published` 与 `AgentAdoptionStateActive`；构造器 `NewAgentAdoptionService(repo repository.AgentAdoptionRepository, agents AdoptionAgentSource, versions interfaces.AgentVersionService) *AgentAdoptionService`（`AdoptionAgentSource` 仅 `CreateAgent(ctx, *types.CustomAgent) (*types.CustomAgent, error)`，`interfaces.CustomAgentService` 结构性满足）。

- [ ] **Step 1: 写失败测试**

`internal/application/service/agent_adoption_test.go`：

```go
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// fakeAdoptionAgentSource stands in for the Agent domain on the publish
// flow: it records every instantiated local Agent Definition.
type fakeAdoptionAgentSource struct{ created []*types.CustomAgent }

func (f *fakeAdoptionAgentSource) CreateAgent(_ context.Context, agent *types.CustomAgent) (*types.CustomAgent, error) {
	agent.ID = fmt.Sprintf("agent-%d", len(f.created)+1)
	agent.TenantID = 1
	f.created = append(f.created, agent)
	return agent, nil
}

type fakeAdoptionVersions struct{}

func (fakeAdoptionVersions) FreezeAgentVersion(_ context.Context, _ uint64, _, agentID string) (interfaces.AgentVersionView, error) {
	return interfaces.AgentVersionView{ID: "version-of-" + agentID, AgentID: agentID, VersionNumber: 1}, nil
}
func (fakeAdoptionVersions) GetAgentVersion(context.Context, uint64, string) (types.AgentVersionSnapshot, error) {
	return types.AgentVersionSnapshot{}, nil
}
func (fakeAdoptionVersions) ListAgentVersions(context.Context, uint64, string) ([]interfaces.AgentVersionView, error) {
	return nil, nil
}

func newAgentAdoptionServiceForTest(t *testing.T) (*AgentAdoptionService, *gorm.DB, *fakeAdoptionAgentSource) {
	t.Helper()
	db := openAgentVersionServiceTestDB(t)
	agents := &fakeAdoptionAgentSource{}
	svc := NewAgentAdoptionService(repository.NewAgentAdoptionRepository(db), agents, fakeAdoptionVersions{})
	return svc, db, agents
}

// seedAdoptionServiceRelease publishes a real Release whose Manifest
// requires the capabilities "model" and "knowledge".
func seedAdoptionServiceRelease(t *testing.T, db *gorm.DB) (listingID, releaseID string) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO agent_versions (id, tenant_id, agent_id, version_number, snapshot, source_sha256, frozen_by) VALUES ('version-a', 1, 'agent-a', 1, '{}', 'sha', 'author')`,
	).Error)
	manifest := `{"semantic_version":"1.0.0","display_name":"Helper","summary":"Portable helper","supported_languages":["en"],"use_cases":["support"],"capability_requirements":["model","knowledge"],"minimum_weknora_capability":"1","license_id":"MIT","source":{"agent_version_id":"version-a","version_number":1,"source_sha256":"sha"}}`
	bundle := []byte(`{"payload":{"agent_mode":"smart-reasoning","system_prompt":"Be useful.","allowed_tools":["search"]},"manifest":` + manifest + `,"dependency_lock":{"dependencies":[]}}`)
	sum := sha256.Sum256(bundle)
	digest := hex.EncodeToString(sum[:])
	repo := repository.NewAgentMarketplaceRepository(db)
	submission, err := repo.CreateSubmission(context.Background(),
		&types.AgentMarketplaceListingEntity{TenantID: 1, SourceAgentID: "agent-a", DisplayName: "Helper", Summary: "s", State: "listed"},
		&types.AgentReleaseSubmissionEntity{
			TenantID: 1, AgentVersionID: "version-a", SourceAgentID: "agent-a", AuthorID: "author",
			SemanticVersion: "1.0.0", BundleDigest: digest, ManifestJSON: manifest,
			DependencyLockJSON: `{"dependencies":[]}`, Bundle: bundle, Status: "submitted",
		})
	require.NoError(t, err)
	_, release, err := repo.ReviewAndPublishTx(context.Background(), 1, "", submission.ID, digest, types.AgentReleaseReviewDecision{ReviewerID: "reviewer", Decision: "approved"})
	require.NoError(t, err)
	require.NotNil(t, release)
	return submission.ListingID, release.ID
}

func TestAgentAdoptionServiceVariantsMappingTestPublish(t *testing.T) {
	svc, db, agents := newAgentAdoptionServiceForTest(t)
	listingID, releaseID := seedAdoptionServiceRelease(t, db)
	ctx := context.Background()

	adoption, created, err := svc.Adopt(ctx, 1, "admin", interfaces.AdoptInput{ListingID: listingID})
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, releaseID, adoption.AcceptedReleaseID)
	reAdopted, createdAgain, err := svc.Adopt(ctx, 1, "admin", interfaces.AdoptInput{ListingID: listingID})
	require.NoError(t, err)
	require.False(t, createdAgain)
	require.Equal(t, adoption.ID, reAdopted.ID)

	sales, err := svc.CreateVariant(ctx, 1, "admin", adoption.ID, interfaces.VariantDraftInput{Name: "Sales Assistant"})
	require.NoError(t, err)
	require.Equal(t, "draft", sales.State)
	require.Equal(t, []string{"knowledge", "model"}, sales.MissingCapabilities)

	// AC2: a variant missing required capabilities is not runnable and the
	// refusal names every missing capability.
	_, err = svc.TestVariant(ctx, 1, "admin", sales.ID)
	require.ErrorIs(t, err, ErrAgentAdoptionVariantNotRunnable)
	require.Contains(t, err.Error(), "knowledge")
	require.Contains(t, err.Error(), "model")
	_, err = svc.PublishVariant(ctx, 1, "admin", sales.ID)
	require.ErrorIs(t, err, ErrAgentAdoptionVariantNotRunnable)

	sales, err = svc.UpdateCapabilityMapping(ctx, 1, "admin", sales.ID, []interfaces.CapabilityMapping{{Capability: "model", ModelID: "gpt-x"}})
	require.NoError(t, err)
	require.Equal(t, "draft", sales.State, "partial mapping keeps the variant unmapped")
	require.Equal(t, []string{"knowledge"}, sales.MissingCapabilities)
	_, err = svc.TestVariant(ctx, 1, "admin", sales.ID)
	require.ErrorIs(t, err, ErrAgentAdoptionVariantNotRunnable)
	require.Contains(t, err.Error(), "knowledge")

	sales, err = svc.UpdateCapabilityMapping(ctx, 1, "admin", sales.ID, []interfaces.CapabilityMapping{
		{Capability: "model", ModelID: "gpt-x"},
		{Capability: "knowledge", KnowledgeBaseIDs: []string{"kb-sales"}},
	})
	require.NoError(t, err)
	require.Equal(t, "mapped", sales.State)
	require.Empty(t, sales.MissingCapabilities)
	sales, err = svc.TestVariant(ctx, 1, "admin", sales.ID)
	require.NoError(t, err)
	require.Equal(t, "tested", sales.State)
	require.Equal(t, "admin", sales.TestedBy)

	// AC1: the SAME adoption derives a second, independent variant.
	legal, err := svc.CreateVariant(ctx, 1, "admin", adoption.ID, interfaces.VariantDraftInput{Name: "Legal Assistant"})
	require.NoError(t, err)
	require.Equal(t, "draft", legal.State)
	legal, err = svc.UpdateCapabilityMapping(ctx, 1, "admin", legal.ID, []interfaces.CapabilityMapping{
		{Capability: "model", ModelID: "gpt-legal"},
		{Capability: "knowledge", KnowledgeBaseIDs: []string{"kb-legal"}},
	})
	require.NoError(t, err)
	legal, err = svc.TestVariant(ctx, 1, "admin", legal.ID)
	require.NoError(t, err)
	require.Equal(t, "tested", legal.State)

	publishedSales, err := svc.PublishVariant(ctx, 1, "admin", sales.ID)
	require.NoError(t, err)
	require.Equal(t, "published", publishedSales.Variant.State)
	require.NotEmpty(t, publishedSales.Variant.LocalAgentID)
	require.Equal(t, "version-of-agent-1", publishedSales.Variant.LocalAgentVersionID)
	publishedLegal, err := svc.PublishVariant(ctx, 1, "admin", legal.ID)
	require.NoError(t, err)
	require.Equal(t, "published", publishedLegal.Variant.State)
	require.NotEqual(t, publishedSales.Variant.LocalAgentID, publishedLegal.Variant.LocalAgentID)
	require.Len(t, agents.created, 2, "one adoption with two variants instantiates two independent local agents")
	require.Equal(t, []string{"kb-sales"}, agents.created[0].Config.KnowledgeBases)
	require.Equal(t, "gpt-x", agents.created[0].Config.ModelID)
	require.Equal(t, []string{"kb-legal"}, agents.created[1].Config.KnowledgeBases)
	require.Equal(t, "gpt-legal", agents.created[1].Config.ModelID)
	require.Equal(t, "Be useful.", agents.created[0].Config.SystemPrompt, "portable payload behavior survives the round trip")

	available, err := svc.ListAvailableAgents(ctx, 1)
	require.NoError(t, err)
	require.Len(t, available, 2)
	require.Equal(t, "supported", available[0].Capability.State)
	listed, err := svc.ListAdoptions(ctx, 1)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Len(t, listed[0].Variants, 2)

	// A published variant refuses silent re-mapping and re-publication.
	_, err = svc.UpdateCapabilityMapping(ctx, 1, "admin", sales.ID, []interfaces.CapabilityMapping{{Capability: "model", ModelID: "other"}})
	require.ErrorIs(t, err, ErrAgentAdoptionStateConflict)
	_, err = svc.PublishVariant(ctx, 1, "admin", sales.ID)
	require.ErrorIs(t, err, ErrAgentAdoptionStateConflict)
	require.Len(t, agents.created, 2, "a refused re-publish must not instantiate another local agent")

	// Testing an untested-again path: publish from mapped (not tested) refuses.
	_, err = svc.PublishVariant(ctx, 1, "admin", "missing-variant")
	require.ErrorIs(t, err, ErrAgentAdoptionNotFound)
}

func TestAgentAdoptionServiceRejectsUnknownAndDuplicateCapabilities(t *testing.T) {
	svc, db, _ := newAgentAdoptionServiceForTest(t)
	listingID, _ := seedAdoptionServiceRelease(t, db)
	ctx := context.Background()
	adoption, _, err := svc.Adopt(ctx, 1, "admin", interfaces.AdoptInput{ListingID: listingID})
	require.NoError(t, err)
	variant, err := svc.CreateVariant(ctx, 1, "admin", adoption.ID, interfaces.VariantDraftInput{Name: "Sales"})
	require.NoError(t, err)

	_, err = svc.UpdateCapabilityMapping(ctx, 1, "admin", variant.ID, []interfaces.CapabilityMapping{{Capability: "sandbox"}})
	require.ErrorIs(t, err, ErrAgentAdoptionInvalidInput)
	_, err = svc.UpdateCapabilityMapping(ctx, 1, "admin", variant.ID, []interfaces.CapabilityMapping{{Capability: "model", ModelID: "a"}, {Capability: "model", ModelID: "b"}})
	require.ErrorIs(t, err, ErrAgentAdoptionInvalidInput)

	// Cross-tenant ids read as not-found; an empty binding never covers.
	_, err = svc.CreateVariant(ctx, 2, "admin", adoption.ID, interfaces.VariantDraftInput{Name: "X"})
	require.ErrorIs(t, err, ErrAgentAdoptionNotFound)
	variant, err = svc.UpdateCapabilityMapping(ctx, 1, "admin", variant.ID, []interfaces.CapabilityMapping{{Capability: "model"}})
	require.NoError(t, err)
	require.Equal(t, []string{"knowledge", "model"}, variant.MissingCapabilities, "an empty binding leaves the capability missing")
	require.Equal(t, "draft", variant.State)
}

func TestAgentAdoptionServicePublishRefusesTamperedRelease(t *testing.T) {
	svc, db, _ := newAgentAdoptionServiceForTest(t)
	listingID, releaseID := seedAdoptionServiceRelease(t, db)
	ctx := context.Background()
	adoption, _, err := svc.Adopt(ctx, 1, "admin", interfaces.AdoptInput{ListingID: listingID})
	require.NoError(t, err)
	variant, err := svc.CreateVariant(ctx, 1, "admin", adoption.ID, interfaces.VariantDraftInput{Name: "Sales"})
	require.NoError(t, err)
	_, err = svc.UpdateCapabilityMapping(ctx, 1, "admin", variant.ID, []interfaces.CapabilityMapping{
		{Capability: "model", ModelID: "gpt-x"},
		{Capability: "knowledge", KnowledgeBaseIDs: []string{"kb-1"}},
	})
	require.NoError(t, err)
	_, err = svc.TestVariant(ctx, 1, "admin", variant.ID)
	require.NoError(t, err)

	require.NoError(t, db.Model(&types.AgentReleaseEntity{}).Where("tenant_id = ? AND id = ?", 1, releaseID).Update("bundle", []byte(`{"payload":"tampered"}`)).Error)
	_, err = svc.PublishVariant(ctx, 1, "admin", variant.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not match its recorded digest")
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/application/service/ -run 'TestAgentAdoption' -count=1`
Expected: FAIL（构建失败）——`undefined: NewAgentAdoptionService`、`undefined: ErrAgentAdoptionVariantNotRunnable` 等。

- [ ] **Step 3: 写最小实现**

`internal/types/interfaces/agent_adoption.go`：

```go
package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// AgentAdoptionService coordinates the Tenant Adoption and Variant
// governance workflow (docs/specs/2026-09-20-agent-marketplace-domain-model.md
// §8). HTTP authorization remains the route boundary; Tenant and actor
// identity always arrive from the authenticated request context.
type AgentAdoptionService interface {
	// Adopt accepts a Listing (and by default its current Release) for the
	// Tenant. Idempotent per (tenant, listing); the bool reports whether the
	// Adoption row was created by this call.
	Adopt(ctx context.Context, tenantID uint64, actorID string, input AdoptInput) (AdoptionView, bool, error)
	ListAdoptions(ctx context.Context, tenantID uint64) ([]AdoptionView, error)
	CreateVariant(ctx context.Context, tenantID uint64, actorID, adoptionID string, input VariantDraftInput) (AdoptionVariantView, error)
	UpdateCapabilityMapping(ctx context.Context, tenantID uint64, actorID, variantID string, mappings []CapabilityMapping) (AdoptionVariantView, error)
	TestVariant(ctx context.Context, tenantID uint64, actorID, variantID string) (AdoptionVariantView, error)
	PublishVariant(ctx context.Context, tenantID uint64, actorID, variantID string) (PublishVariantResult, error)
	ListAvailableAgents(ctx context.Context, tenantID uint64) ([]AvailableAgentView, error)
}

type AdoptInput struct{ ListingID, ReleaseID string }

type VariantDraftInput struct{ Name, ReleaseID string }

// CapabilityMapping is one local binding of a Manifest capability
// requirement: a local model, knowledge bases and/or connections. At least
// one non-empty binding must be present for the capability to count as
// covered.
type CapabilityMapping struct {
	Capability       string
	ModelID          string
	KnowledgeBaseIDs []string
	ConnectionIDs    []string
}

type AdoptionView struct {
	types.AgentAdoptionEntity
	Variants []AdoptionVariantView `json:"variants"`
}

type AdoptionVariantView struct {
	types.AgentAdoptionVariantEntity
	// MissingCapabilities names every Manifest capability requirement that
	// no mapping binds to a local resource, sorted for determinism.
	MissingCapabilities []string `json:"missing_capabilities"`
}

type PublishVariantResult struct {
	Variant AdoptionVariantView `json:"variant"`
}

type AgentCapabilityVerdict struct {
	State  string `json:"state"`
	Reason string `json:"reason"`
}

// AvailableAgentView is the adoption-aware mobile read model row: one
// published Variant's live local agent. Field names mirror the wire the
// mobile Resource Shelf already consumes (GET /api/v1/agents rows) plus the
// governance lineage ids.
type AvailableAgentView struct {
	AgentID     string                `json:"agent_id"`
	VariantID   string                `json:"variant_id"`
	AdoptionID  string                `json:"adoption_id"`
	ReleaseID   string                `json:"release_id"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	IsBuiltin   bool                  `json:"is_builtin"`
	Capability  AgentCapabilityVerdict `json:"capability"`
}
```

`internal/application/service/agent_adoption.go`：

```go
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

var (
	ErrAgentAdoptionInvalidInput       = errors.New("invalid agent adoption request")
	ErrAgentAdoptionNotFound           = errors.New("agent adoption resource not found")
	ErrAgentAdoptionVariantNotRunnable = errors.New("agent adoption variant is not runnable")
	ErrAgentAdoptionStateConflict      = errors.New("agent adoption variant state conflict")
)

// Variant lifecycle states (CONTEXT.md「Agent 变体」「可用 Agent」; the spec's
// "mapping-required" is state draft with a non-empty missing_capabilities).
// retire belongs to #63 and is deliberately absent.
const (
	AgentVariantStateDraft     = "draft"
	AgentVariantStateMapped    = "mapped"
	AgentVariantStateTested    = "tested"
	AgentVariantStatePublished = "published"
)

// AgentAdoptionStateActive is the only Adoption state in this ticket;
// end_adoption belongs to #63.
const AgentAdoptionStateActive = "active"

// AdoptionAgentSource is the Agent-domain seam the publish flow needs:
// instantiating the Variant's local Agent Definition.
// interfaces.CustomAgentService satisfies it structurally.
type AdoptionAgentSource interface {
	CreateAgent(ctx context.Context, agent *types.CustomAgent) (*types.CustomAgent, error)
}

type AgentAdoptionService struct {
	repo     repository.AgentAdoptionRepository
	agents   AdoptionAgentSource
	versions interfaces.AgentVersionService
	now      func() time.Time
}

var _ interfaces.AgentAdoptionService = (*AgentAdoptionService)(nil)

func NewAgentAdoptionService(repo repository.AgentAdoptionRepository, agents AdoptionAgentSource, versions interfaces.AgentVersionService) *AgentAdoptionService {
	return &AgentAdoptionService{repo: repo, agents: agents, versions: versions, now: time.Now}
}

func (s *AgentAdoptionService) Adopt(ctx context.Context, tenantID uint64, actorID string, input interfaces.AdoptInput) (interfaces.AdoptionView, bool, error) {
	input.ListingID, input.ReleaseID = strings.TrimSpace(input.ListingID), strings.TrimSpace(input.ReleaseID)
	actorID = strings.TrimSpace(actorID)
	if tenantID == 0 || actorID == "" || input.ListingID == "" {
		return interfaces.AdoptionView{}, false, ErrAgentAdoptionInvalidInput
	}
	listing, err := s.repo.GetMarketplaceListing(ctx, tenantID, input.ListingID)
	if err != nil {
		return interfaces.AdoptionView{}, false, err
	}
	if listing == nil {
		return interfaces.AdoptionView{}, false, ErrAgentAdoptionNotFound
	}
	if listing.State != "listed" || listing.CurrentReleaseID == nil {
		return interfaces.AdoptionView{}, false, fmt.Errorf("%w: listing %s has no adoptable release", ErrAgentAdoptionInvalidInput, listing.ID)
	}
	releaseID := input.ReleaseID
	if releaseID == "" {
		releaseID = *listing.CurrentReleaseID
	}
	release, err := s.repo.GetRelease(ctx, tenantID, releaseID)
	if err != nil {
		return interfaces.AdoptionView{}, false, err
	}
	if release == nil || release.ListingID != listing.ID {
		return interfaces.AdoptionView{}, false, fmt.Errorf("%w: release does not belong to listing", ErrAgentAdoptionInvalidInput)
	}
	row, created, err := s.repo.AdoptListing(ctx, &types.AgentAdoptionEntity{
		TenantID: tenantID, ListingID: listing.ID, AcceptedReleaseID: releaseID,
		State: AgentAdoptionStateActive, CreatedBy: actorID,
	})
	if err != nil {
		return interfaces.AdoptionView{}, false, err
	}
	view, err := s.adoptionView(ctx, tenantID, row)
	return view, created, err
}

func (s *AgentAdoptionService) ListAdoptions(ctx context.Context, tenantID uint64) ([]interfaces.AdoptionView, error) {
	rows, err := s.repo.ListAdoptions(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	views := make([]interfaces.AdoptionView, 0, len(rows))
	for i := range rows {
		view, err := s.adoptionView(ctx, tenantID, &rows[i])
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *AgentAdoptionService) CreateVariant(ctx context.Context, tenantID uint64, actorID, adoptionID string, input interfaces.VariantDraftInput) (interfaces.AdoptionVariantView, error) {
	adoptionID, input.Name, input.ReleaseID = strings.TrimSpace(adoptionID), strings.TrimSpace(input.Name), strings.TrimSpace(input.ReleaseID)
	actorID = strings.TrimSpace(actorID)
	if tenantID == 0 || actorID == "" || adoptionID == "" || input.Name == "" {
		return interfaces.AdoptionVariantView{}, ErrAgentAdoptionInvalidInput
	}
	adoption, err := s.repo.GetAdoption(ctx, tenantID, adoptionID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	if adoption == nil {
		return interfaces.AdoptionVariantView{}, ErrAgentAdoptionNotFound
	}
	if adoption.State != AgentAdoptionStateActive {
		return interfaces.AdoptionVariantView{}, fmt.Errorf("%w: adoption state is %q", ErrAgentAdoptionStateConflict, adoption.State)
	}
	releaseID := input.ReleaseID
	if releaseID == "" {
		releaseID = adoption.AcceptedReleaseID
	}
	release, err := s.repo.GetRelease(ctx, tenantID, releaseID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	if release == nil || release.ListingID != adoption.ListingID {
		return interfaces.AdoptionVariantView{}, fmt.Errorf("%w: release does not belong to the adopted listing", ErrAgentAdoptionInvalidInput)
	}
	created, err := s.repo.CreateVariant(ctx, &types.AgentAdoptionVariantEntity{
		TenantID: tenantID, AdoptionID: adoption.ID, ReleaseID: releaseID,
		Name: input.Name, State: AgentVariantStateDraft, CreatedBy: actorID,
	})
	if err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	return s.variantView(ctx, tenantID, created)
}

func (s *AgentAdoptionService) UpdateCapabilityMapping(ctx context.Context, tenantID uint64, actorID, variantID string, mappings []interfaces.CapabilityMapping) (interfaces.AdoptionVariantView, error) {
	variantID, actorID = strings.TrimSpace(variantID), strings.TrimSpace(actorID)
	if tenantID == 0 || actorID == "" || variantID == "" {
		return interfaces.AdoptionVariantView{}, ErrAgentAdoptionInvalidInput
	}
	variant, err := s.repo.GetVariant(ctx, tenantID, variantID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	if variant == nil {
		return interfaces.AdoptionVariantView{}, ErrAgentAdoptionNotFound
	}
	switch variant.State {
	case AgentVariantStateDraft, AgentVariantStateMapped, AgentVariantStateTested:
	default:
		return interfaces.AdoptionVariantView{}, fmt.Errorf("%w: state %q cannot be re-mapped", ErrAgentAdoptionStateConflict, variant.State)
	}
	manifest, err := releaseManifest(ctx, s.repo, tenantID, variant.ReleaseID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	required := make(map[string]bool, len(manifest.CapabilityRequirements))
	for _, capability := range manifest.CapabilityRequirements {
		required[capability] = true
	}
	rows := make([]types.AgentVariantCapabilityMappingEntity, 0, len(mappings))
	seen := make(map[string]bool, len(mappings))
	for _, mapping := range mappings {
		capability := strings.TrimSpace(mapping.Capability)
		if capability == "" || !required[capability] {
			return interfaces.AdoptionVariantView{}, fmt.Errorf("%w: capability %q is not required by release %s", ErrAgentAdoptionInvalidInput, mapping.Capability, variant.ReleaseID)
		}
		if seen[capability] {
			return interfaces.AdoptionVariantView{}, fmt.Errorf("%w: duplicate mapping for capability %q", ErrAgentAdoptionInvalidInput, capability)
		}
		seen[capability] = true
		rows = append(rows, types.AgentVariantCapabilityMappingEntity{
			TenantID: tenantID, VariantID: variant.ID, Capability: capability,
			ModelID:          strings.TrimSpace(mapping.ModelID),
			KnowledgeBaseIDs: encodeIDList(mapping.KnowledgeBaseIDs),
			ConnectionIDs:    encodeIDList(mapping.ConnectionIDs),
			UpdatedBy:        actorID,
		})
	}
	nextState := AgentVariantStateDraft
	if len(missingCapabilities(manifest.CapabilityRequirements, rows)) == 0 {
		nextState = AgentVariantStateMapped
	}
	if err := s.repo.ReplaceCapabilityMappings(ctx, tenantID, variant.ID, rows, nextState); err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	updated, err := s.repo.GetVariant(ctx, tenantID, variant.ID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	if updated == nil {
		return interfaces.AdoptionVariantView{}, ErrAgentAdoptionNotFound
	}
	return s.variantView(ctx, tenantID, updated)
}

func (s *AgentAdoptionService) TestVariant(ctx context.Context, tenantID uint64, actorID, variantID string) (interfaces.AdoptionVariantView, error) {
	variantID, actorID = strings.TrimSpace(variantID), strings.TrimSpace(actorID)
	if tenantID == 0 || actorID == "" || variantID == "" {
		return interfaces.AdoptionVariantView{}, ErrAgentAdoptionInvalidInput
	}
	variant, missing, err := s.variantWithMissing(ctx, tenantID, variantID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	if variant == nil {
		return interfaces.AdoptionVariantView{}, ErrAgentAdoptionNotFound
	}
	if len(missing) > 0 {
		return interfaces.AdoptionVariantView{}, notRunnable(missing)
	}
	if variant.State != AgentVariantStateMapped {
		return interfaces.AdoptionVariantView{}, fmt.Errorf("%w: state is %q; complete capability mapping first", ErrAgentAdoptionStateConflict, variant.State)
	}
	testedAt := s.now().UTC()
	updated, err := s.repo.UpdateVariantState(ctx, tenantID, variant.ID, []string{AgentVariantStateMapped}, AgentVariantStateTested, map[string]any{
		"tested_by": actorID, "tested_at": testedAt,
	})
	if err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	return s.variantView(ctx, tenantID, updated)
}

func (s *AgentAdoptionService) PublishVariant(ctx context.Context, tenantID uint64, actorID, variantID string) (interfaces.PublishVariantResult, error) {
	variantID, actorID = strings.TrimSpace(variantID), strings.TrimSpace(actorID)
	if tenantID == 0 || actorID == "" || variantID == "" {
		return interfaces.PublishVariantResult{}, ErrAgentAdoptionInvalidInput
	}
	variant, missing, err := s.variantWithMissing(ctx, tenantID, variantID)
	if err != nil {
		return interfaces.PublishVariantResult{}, err
	}
	if variant == nil {
		return interfaces.PublishVariantResult{}, ErrAgentAdoptionNotFound
	}
	if len(missing) > 0 {
		return interfaces.PublishVariantResult{}, notRunnable(missing)
	}
	if variant.State != AgentVariantStateTested {
		return interfaces.PublishVariantResult{}, fmt.Errorf("%w: state is %q; complete capability mapping and test first", ErrAgentAdoptionStateConflict, variant.State)
	}
	manifest, err := releaseManifest(ctx, s.repo, tenantID, variant.ReleaseID)
	if err != nil {
		return interfaces.PublishVariantResult{}, err
	}
	release, err := s.repo.GetRelease(ctx, tenantID, variant.ReleaseID)
	if err != nil {
		return interfaces.PublishVariantResult{}, err
	}
	if release == nil {
		return interfaces.PublishVariantResult{}, ErrAgentAdoptionNotFound
	}
	sum := sha256.Sum256(release.Bundle)
	if hex.EncodeToString(sum[:]) != release.BundleDigest {
		return interfaces.PublishVariantResult{}, fmt.Errorf("release bundle does not match its recorded digest")
	}
	var envelope struct {
		Payload types.AgentReleasePayload `json:"payload"`
	}
	if err := json.Unmarshal(release.Bundle, &envelope); err != nil {
		return interfaces.PublishVariantResult{}, fmt.Errorf("decode release bundle payload: %w", err)
	}
	mappings, err := s.repo.ListCapabilityMappings(ctx, tenantID, variant.ID)
	if err != nil {
		return interfaces.PublishVariantResult{}, err
	}
	created, err := s.agents.CreateAgent(ctx, buildLocalAgent(variant, envelope.Payload, *manifest, mappings))
	if err != nil {
		return interfaces.PublishVariantResult{}, fmt.Errorf("create the variant local agent: %w", err)
	}
	view, err := s.versions.FreezeAgentVersion(ctx, tenantID, actorID, created.ID)
	if err != nil {
		return interfaces.PublishVariantResult{}, fmt.Errorf("freeze the variant local agent version: %w", err)
	}
	publishedAt := s.now().UTC()
	updated, err := s.repo.UpdateVariantState(ctx, tenantID, variant.ID, []string{AgentVariantStateTested}, AgentVariantStatePublished, map[string]any{
		"local_agent_id": created.ID, "local_agent_version_id": view.ID,
		"published_by": actorID, "published_at": publishedAt,
	})
	if err != nil {
		return interfaces.PublishVariantResult{}, err
	}
	variantView, err := s.variantView(ctx, tenantID, updated)
	if err != nil {
		return interfaces.PublishVariantResult{}, err
	}
	return interfaces.PublishVariantResult{Variant: variantView}, nil
}

func (s *AgentAdoptionService) ListAvailableAgents(ctx context.Context, tenantID uint64) ([]interfaces.AvailableAgentView, error) {
	if tenantID == 0 {
		return nil, ErrAgentAdoptionInvalidInput
	}
	rows, err := s.repo.PublishedAvailableAgents(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	views := make([]interfaces.AvailableAgentView, 0, len(rows))
	for _, row := range rows {
		if row.Agent == nil {
			continue // the local agent row is gone: the variant simply is not available
		}
		views = append(views, interfaces.AvailableAgentView{
			AgentID:     row.Agent.ID,
			VariantID:   row.Variant.ID,
			AdoptionID:  row.Variant.AdoptionID,
			ReleaseID:   row.Variant.ReleaseID,
			Name:        row.Agent.Name,
			Description: row.Agent.Description,
			IsBuiltin:   row.Agent.IsBuiltin,
			Capability:  interfaces.AgentCapabilityVerdict{State: "supported", Reason: ""},
		})
	}
	return views, nil
}

// notRunnable builds the explicit refusal: every missing capability is
// named, sorted (missingCapabilities already sorts).
func notRunnable(missing []string) error {
	return fmt.Errorf("%w: missing required capabilities: %s", ErrAgentAdoptionVariantNotRunnable, strings.Join(missing, ", "))
}

func (s *AgentAdoptionService) adoptionView(ctx context.Context, tenantID uint64, row *types.AgentAdoptionEntity) (interfaces.AdoptionView, error) {
	view := interfaces.AdoptionView{AgentAdoptionEntity: *row, Variants: []interfaces.AdoptionVariantView{}}
	variants, err := s.repo.ListVariantsByAdoption(ctx, tenantID, row.ID)
	if err != nil {
		return interfaces.AdoptionView{}, err
	}
	for i := range variants {
		variant, err := s.variantView(ctx, tenantID, &variants[i])
		if err != nil {
			return interfaces.AdoptionView{}, err
		}
		view.Variants = append(view.Variants, variant)
	}
	return view, nil
}

func (s *AgentAdoptionService) variantView(ctx context.Context, tenantID uint64, variant *types.AgentAdoptionVariantEntity) (interfaces.AdoptionVariantView, error) {
	missing, err := s.missingCapabilitiesOf(ctx, tenantID, variant)
	if err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	return interfaces.AdoptionVariantView{AgentAdoptionVariantEntity: *variant, MissingCapabilities: missing}, nil
}

func (s *AgentAdoptionService) variantWithMissing(ctx context.Context, tenantID uint64, variantID string) (*types.AgentAdoptionVariantEntity, []string, error) {
	variant, err := s.repo.GetVariant(ctx, tenantID, variantID)
	if err != nil || variant == nil {
		return nil, nil, err
	}
	manifest, err := releaseManifest(ctx, s.repo, tenantID, variant.ReleaseID)
	if err != nil {
		return nil, nil, err
	}
	mappings, err := s.repo.ListCapabilityMappings(ctx, tenantID, variant.ID)
	if err != nil {
		return nil, nil, err
	}
	return variant, missingCapabilities(manifest.CapabilityRequirements, mappings), nil
}

func (s *AgentAdoptionService) missingCapabilitiesOf(ctx context.Context, tenantID uint64, variant *types.AgentAdoptionVariantEntity) ([]string, error) {
	manifest, err := releaseManifest(ctx, s.repo, tenantID, variant.ReleaseID)
	if err != nil {
		return nil, err
	}
	mappings, err := s.repo.ListCapabilityMappings(ctx, tenantID, variant.ID)
	if err != nil {
		return nil, err
	}
	return missingCapabilities(manifest.CapabilityRequirements, mappings), nil
}

type adoptionReleaseReader interface {
	GetRelease(context.Context, uint64, string) (*types.AgentReleaseEntity, error)
}

func releaseManifest(ctx context.Context, repo adoptionReleaseReader, tenantID uint64, releaseID string) (*types.AgentReleaseManifest, error) {
	release, err := repo.GetRelease(ctx, tenantID, releaseID)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, ErrAgentAdoptionNotFound
	}
	var manifest types.AgentReleaseManifest
	if err := json.Unmarshal([]byte(release.ManifestJSON), &manifest); err != nil {
		return nil, fmt.Errorf("decode release manifest: %w", err)
	}
	return &manifest, nil
}

// missingCapabilities computes the Manifest capability requirements that no
// stored mapping binds to a local resource. Sorting keeps the refusal
// message deterministic.
func missingCapabilities(requirements []string, mappings []types.AgentVariantCapabilityMappingEntity) []string {
	bound := make(map[string]bool, len(mappings))
	for _, mapping := range mappings {
		if capabilityBound(mapping) {
			bound[mapping.Capability] = true
		}
	}
	missing := make([]string, 0, len(requirements))
	for _, capability := range requirements {
		if !bound[capability] {
			missing = append(missing, capability)
		}
	}
	sort.Strings(missing)
	return missing
}

func capabilityBound(mapping types.AgentVariantCapabilityMappingEntity) bool {
	if strings.TrimSpace(mapping.ModelID) != "" {
		return true
	}
	return len(decodeIDList(mapping.KnowledgeBaseIDs)) > 0 || len(decodeIDList(mapping.ConnectionIDs)) > 0
}

func decodeIDList(encoded string) []string {
	ids := []string{}
	_ = json.Unmarshal([]byte(encoded), &ids)
	return ids
}

func encodeIDList(ids []string) string {
	if len(ids) == 0 {
		return "[]"
	}
	raw, err := json.Marshal(ids)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

// buildLocalAgent projects the Release's portable payload plus the
// Variant's local capability mapping into a local Agent Definition draft.
// The payload never carries KB/model bindings (T28 allow-list boundary);
// every local binding here comes from the mapping rows.
func buildLocalAgent(variant *types.AgentAdoptionVariantEntity, payload types.AgentReleasePayload, manifest types.AgentReleaseManifest, mappings []types.AgentVariantCapabilityMappingEntity) *types.CustomAgent {
	knowledgeBases := []string{}
	modelID := ""
	for _, mapping := range mappings {
		knowledgeBases = append(knowledgeBases, decodeIDList(mapping.KnowledgeBaseIDs)...)
		if modelID == "" {
			modelID = strings.TrimSpace(mapping.ModelID)
		}
	}
	config := types.CustomAgentConfig{
		AgentMode:      payload.AgentMode,
		SystemPrompt:   payload.SystemPrompt,
		PersonaMBTI:    payload.PersonaMBTI,
		PersonaStyle:   payload.PersonaStyle,
		AllowedTools:   payload.AllowedTools,
		Subagents:      payload.Subagents,
		KnowledgeBases: knowledgeBases,
		ModelID:        modelID,
	}
	if len(payload.Skills) > 0 {
		config.SkillsSelectionMode = "selected"
		config.SelectedSkills = payload.Skills
	}
	return &types.CustomAgent{
		Name:        variant.Name,
		Description: manifest.Summary,
		Config:      config,
	}
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/application/service/ -run 'TestAgentAdoption' -count=1 -v`
Expected: PASS（3 个测试）。

- [ ] **Step 5: Commit**

```bash
git add internal/types/interfaces/agent_adoption.go internal/application/service/agent_adoption.go internal/application/service/agent_adoption_test.go
git commit -m "feat(marketplace): adoption service state machine with capability gating and publish orchestration (T29 #59 task 3)"
```

---

### Task 4: HTTP 面 I——Adopt / ListAdoptions / CreateVariant 与接线

**Files:**
- Create: `internal/handler/agent_adoption.go`
- Create: `internal/router/routes_agent_adoption.go`
- Modify: `internal/router/router.go:101`（Params 结构体 `AgentMarketplaceHandler` 字段后加 `AgentAdoptionHandler`）、`:398`（`RegisterAgentMarketplaceRoutes(...)` 行后加注册行）
- Modify: `internal/container/container.go:431`（`must(container.Provide(handler.NewAgentMarketplaceHandler))` 之后追加三段 Provide）
- Test: `internal/router/routes_agent_adoption_test.go`

**Interfaces:**
- Consumes: Task 3 `interfaces.AgentAdoptionService` 全部签名；#58 handler 惯例——`decodeAgentMarketplaceBody` / `invalidMarketplaceBody` / `agentMarketplaceMaxRequestBytes`（`internal/handler/agent_marketplace.go:21,139-157`）与 `sandboxConfigTenantID`（`internal/handler/sandbox_config.go:92`；同包可直接复用）；路由惯例 `g.apiKeyRoute(r, method, path, apiKeyFullAccess(), g.Admin(), handler)`（`internal/router/routes_agent_marketplace.go:26-33`）；测试骨架 `openTenantAgentMarketplaceHTTPTestDB` / `marketplaceHTTPResolver`（`internal/router/routes_agent_marketplace_test.go:43-47,349-373`，同包复用）与 `mustLookupAPIKeyPolicy`（`internal/router/router_api_key_capabilities_test.go:664`）。
- Produces: `handler.AgentAdoptionHandler`（本任务方法 `Adopt`/`ListAdoptions`/`CreateVariant`；Task 5/6 追加其余方法）；`RegisterAgentAdoptionRoutes(r *gin.RouterGroup, adoptionHandler *handler.AgentAdoptionHandler, g *rbacGuards)`；路由 `POST|GET /api/v1/marketplace/tenant/adoptions`、`POST /api/v1/marketplace/tenant/adoptions/:id/variants`；测试脚手架 `newAgentAdoptionTestApp(t) (*gin.Engine, *rbacGuards, *gorm.DB)` 与 `adoptionCall(...)`（Task 5/6 复用）。

- [ ] **Step 1: 写失败测试**

`internal/router/routes_agent_adoption_test.go`：

```go
package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newAgentAdoptionTestApp mounts the REAL release->adoption stack over the
// real migration stream: real CustomAgentService/AgentVersionService/
// AgentMarketplaceService/AgentAdoptionService and the real agent list
// endpoint (GET /api/v1/agents) that the mobile Resource Shelf consumes.
func newAgentAdoptionTestApp(t *testing.T) (*gin.Engine, *rbacGuards, *gorm.DB) {
	t.Helper()
	db := openTenantAgentMarketplaceHTTPTestDB(t)
	require.NoError(t, db.Create(&types.CustomAgent{
		ID: "agent-owned", Name: "Release helper", TenantID: 1, CreatedBy: "contributor",
		Config: types.CustomAgentConfig{AgentMode: "smart-reasoning", SystemPrompt: "Be useful."},
	}).Error)

	marketRepo := repository.NewAgentMarketplaceRepository(db)
	customAgents := service.NewCustomAgentService(repository.NewCustomAgentRepository(db), nil, nil, nil, nil, nil, nil)
	versions := service.NewAgentVersionService(customAgents, repository.NewAgentVersionRepository(db))
	market := service.NewAgentMarketplaceService(versions, marketplaceHTTPResolver{}, marketRepo, t.TempDir())
	adoptions := service.NewAgentAdoptionService(repository.NewAgentAdoptionRepository(db), customAgents, versions)

	versionHandler := handler.NewAgentVersionHandler(versions)
	marketHandler := handler.NewAgentMarketplaceHandler(market, versions)
	adoptionHandler := handler.NewAgentAdoptionHandler(adoptions)
	agentListHandler := handler.NewCustomAgentHandler(customAgents, nil, repository.NewTenantDisabledSharedAgentRepository(db), nil, nil)

	enabled := true
	g := &rbacGuards{cfg: &config.Config{Tenant: &config.TenantConfig{EnableRBAC: &enabled}}, agentCreator: func(c *gin.Context) (string, error) {
		if c.Param("id") == "agent-owned" {
			return "contributor", nil
		}
		return "", nil
	}}
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		tenantID := uint64(1)
		if c.GetHeader("X-Test-Tenant") == "2" {
			tenantID = 2
		}
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, tenantID)
		ctx = context.WithValue(ctx, types.UserIDContextKey, c.GetHeader("X-Test-Actor"))
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRole(c.GetHeader("X-Test-Role")))
		c.Request = c.Request.WithContext(ctx)
		c.Set(types.TenantIDContextKey.String(), tenantID)
		c.Next()
	})
	v1 := r.Group("/api/v1")
	RegisterAgentVersionRoutes(v1, versionHandler, g)
	RegisterAgentMarketplaceRoutes(v1, marketHandler, g)
	RegisterAgentAdoptionRoutes(v1, adoptionHandler, g)
	RegisterCustomAgentRoutes(v1, agentListHandler, g)
	return r, g, db
}

func adoptionCall(r *gin.Engine, tenantID uint64, method, path, role, actor string, body any) *httptest.ResponseRecorder {
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Role", role)
	req.Header.Set("X-Test-Actor", actor)
	if tenantID == 2 {
		req.Header.Set("X-Test-Tenant", "2")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// publishAdoptionRelease drives the real HTTP release workflow and returns
// the listing id plus the published release id.
func publishAdoptionRelease(t *testing.T, r *gin.Engine) (listingID, releaseID string) {
	t.Helper()
	frozen := adoptionCall(r, 1, http.MethodPost, "/api/v1/agents/agent-owned/versions", "contributor", "contributor", nil)
	require.Equal(t, http.StatusCreated, frozen.Code, frozen.Body.String())
	var frozenBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(frozen.Body.Bytes(), &frozenBody))
	require.NotEmpty(t, frozenBody.Data.ID)

	metadata := map[string]any{
		"semantic_version": "1.0.0", "display_name": "Release helper", "summary": "A helpful agent",
		"supported_languages": []string{"en"}, "use_cases": []string{"support"},
		"capability_requirements": []string{"model", "knowledge"},
		"minimum_weknora_capability": "1", "license_id": "MIT",
	}
	submitted := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/release-submissions", "contributor", "contributor", map[string]any{"agent_version_id": frozenBody.Data.ID, "metadata": metadata})
	require.Equal(t, http.StatusCreated, submitted.Code, submitted.Body.String())
	var submissionBody struct {
		Data struct {
			ID           string `json:"id"`
			ListingID    string `json:"listing_id"`
			BundleDigest string `json:"bundle_digest"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(submitted.Body.Bytes(), &submissionBody))

	approved := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/release-submissions/"+submissionBody.Data.ID+"/review", "admin", "reviewer", map[string]any{"expected_digest": submissionBody.Data.BundleDigest, "decision": "approved"})
	require.Equal(t, http.StatusOK, approved.Code, approved.Body.String())
	var result struct {
		Data struct {
			Release *struct {
				ID string `json:"id"`
			} `json:"release"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(approved.Body.Bytes(), &result))
	require.NotNil(t, result.Data.Release)
	return submissionBody.Data.ListingID, result.Data.Release.ID
}

func TestTenantAgentAdoptionRoutesAndAuthorization(t *testing.T) {
	r, g, _ := newAgentAdoptionTestApp(t)
	listingID, releaseID := publishAdoptionRelease(t, r)

	viewerAdopt := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "viewer", "viewer", map[string]any{"listing_id": listingID})
	require.Equal(t, http.StatusForbidden, viewerAdopt.Code)
	crossTenantAdopt := adoptionCall(r, 2, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID})
	require.Equal(t, http.StatusNotFound, crossTenantAdopt.Code, "another tenant's listing must read as absent")
	unknownListing := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": "missing-listing"})
	require.Equal(t, http.StatusNotFound, unknownListing.Code)
	badBody := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID, "unexpected": true})
	require.Equal(t, http.StatusBadRequest, badBody.Code)
	spoofed := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID, "tenant_id": 999})
	require.Equal(t, http.StatusBadRequest, spoofed.Code)

	adopted := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID})
	require.Equal(t, http.StatusCreated, adopted.Code, adopted.Body.String())
	var adoptionBody struct {
		Data struct {
			ID                string `json:"id"`
			ListingID         string `json:"listing_id"`
			AcceptedReleaseID string `json:"accepted_release_id"`
			State             string `json:"state"`
			Variants          []struct {
				ID string `json:"id"`
			} `json:"variants"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(adopted.Body.Bytes(), &adoptionBody))
	require.Equal(t, listingID, adoptionBody.Data.ListingID)
	require.Equal(t, releaseID, adoptionBody.Data.AcceptedReleaseID)
	require.Equal(t, "active", adoptionBody.Data.State)
	require.Empty(t, adoptionBody.Data.Variants)

	reAdopt := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID})
	require.Equal(t, http.StatusOK, reAdopt.Code)
	require.Contains(t, reAdopt.Body.String(), adoptionBody.Data.ID, "re-adopting is idempotent")

	variant := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionBody.Data.ID+"/variants", "admin", "admin", map[string]any{"name": "Sales Assistant"})
	require.Equal(t, http.StatusCreated, variant.Code, variant.Body.String())
	var variantBody struct {
		Data struct {
			ID                  string   `json:"id"`
			AdoptionID          string   `json:"adoption_id"`
			ReleaseID           string   `json:"release_id"`
			Name                string   `json:"name"`
			State               string   `json:"state"`
			MissingCapabilities []string `json:"missing_capabilities"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(variant.Body.Bytes(), &variantBody))
	require.Equal(t, adoptionBody.Data.ID, variantBody.Data.AdoptionID)
	require.Equal(t, releaseID, variantBody.Data.ReleaseID)
	require.Equal(t, "Sales Assistant", variantBody.Data.Name)
	require.Equal(t, "draft", variantBody.Data.State)
	require.Equal(t, []string{"knowledge", "model"}, variantBody.Data.MissingCapabilities)

	viewerVariant := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionBody.Data.ID+"/variants", "viewer", "viewer", map[string]any{"name": "Nope"})
	require.Equal(t, http.StatusForbidden, viewerVariant.Code)
	crossTenantVariant := adoptionCall(r, 2, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionBody.Data.ID+"/variants", "admin", "admin", map[string]any{"name": "Nope"})
	require.Equal(t, http.StatusNotFound, crossTenantVariant.Code)
	emptyName := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionBody.Data.ID+"/variants", "admin", "admin", map[string]any{"name": "  "})
	require.Equal(t, http.StatusBadRequest, emptyName.Code)

	list := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	require.Contains(t, list.Body.String(), variantBody.Data.ID)
	viewerList := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/adoptions", "viewer", "viewer", nil)
	require.Equal(t, http.StatusForbidden, viewerList.Code)

	adoptPolicy := mustLookupAPIKeyPolicy(t, g, http.MethodPost, "/api/v1/marketplace/tenant/adoptions")
	if !adoptPolicy.RequireFullAccess || len(adoptPolicy.Capabilities) != 0 {
		t.Fatalf("adopt policy = %#v, want full-access only", adoptPolicy)
	}
	variantPolicy := mustLookupAPIKeyPolicy(t, g, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/:id/variants")
	if !variantPolicy.RequireFullAccess {
		t.Fatalf("variant policy = %#v, want full-access", variantPolicy)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/router/ -run 'TestTenantAgentAdoptionRoutesAndAuthorization' -count=1`
Expected: FAIL（构建失败）——`undefined: RegisterAgentAdoptionRoutes` / `undefined: handler.NewAgentAdoptionHandler`。

- [ ] **Step 3: 写最小实现**

`internal/handler/agent_adoption.go`：

```go
package handler

import (
	"net/http"
	stderrors "errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	marketrepo "github.com/Tencent/WeKnora/internal/application/repository"
	marketservice "github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// AgentAdoptionHandler is the HTTP boundary for the Tenant Adoption and
// Variant governance workflow (spec §8). Tenant and actor identity always
// come from the authenticated request context; request bodies never carry
// principals (strict decoding rejects spoof attempts).
type AgentAdoptionHandler struct {
	adoptions interfaces.AgentAdoptionService
}

func NewAgentAdoptionHandler(adoptions interfaces.AgentAdoptionService) *AgentAdoptionHandler {
	return &AgentAdoptionHandler{adoptions: adoptions}
}

type adoptAgentBody struct {
	ListingID string `json:"listing_id"`
	ReleaseID string `json:"release_id,omitempty"`
}

type createAdoptionVariantBody struct {
	Name      string `json:"name"`
	ReleaseID string `json:"release_id,omitempty"`
}

type adoptionVariantResponse struct {
	ID                  string    `json:"id"`
	AdoptionID          string    `json:"adoption_id"`
	ReleaseID           string    `json:"release_id"`
	Name                string    `json:"name"`
	State               string    `json:"state"`
	LocalAgentID        string    `json:"local_agent_id,omitempty"`
	LocalAgentVersionID string    `json:"local_agent_version_id,omitempty"`
	MissingCapabilities []string  `json:"missing_capabilities"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type adoptionResponse struct {
	ID                string                    `json:"id"`
	ListingID         string                    `json:"listing_id"`
	AcceptedReleaseID string                    `json:"accepted_release_id"`
	State             string                    `json:"state"`
	CreatedBy         string                    `json:"created_by"`
	CreatedAt         time.Time                 `json:"created_at"`
	UpdatedAt         time.Time                 `json:"updated_at"`
	Variants          []adoptionVariantResponse `json:"variants"`
}

func adoptionVariantDTO(view interfaces.AdoptionVariantView) adoptionVariantResponse {
	return adoptionVariantResponse{
		ID: view.ID, AdoptionID: view.AdoptionID, ReleaseID: view.ReleaseID,
		Name: view.Name, State: view.State,
		LocalAgentID: view.LocalAgentID, LocalAgentVersionID: view.LocalAgentVersionID,
		MissingCapabilities: view.MissingCapabilities,
		CreatedAt:           view.CreatedAt, UpdatedAt: view.UpdatedAt,
	}
}

func adoptionDTO(view interfaces.AdoptionView) adoptionResponse {
	variants := make([]adoptionVariantResponse, 0, len(view.Variants))
	for _, variant := range view.Variants {
		variants = append(variants, adoptionVariantDTO(variant))
	}
	return adoptionResponse{
		ID: view.ID, ListingID: view.ListingID, AcceptedReleaseID: view.AcceptedReleaseID,
		State: view.State, CreatedBy: view.CreatedBy,
		CreatedAt: view.CreatedAt, UpdatedAt: view.UpdatedAt, Variants: variants,
	}
}

func adoptionClientError(err error) error {
	switch {
	case stderrors.Is(err, marketservice.ErrAgentAdoptionNotFound), stderrors.Is(err, marketrepo.ErrAgentAdoptionNotFound):
		return apperrors.NewNotFoundError("agent adoption resource not found")
	case stderrors.Is(err, marketservice.ErrAgentAdoptionVariantNotRunnable),
		stderrors.Is(err, marketservice.ErrAgentAdoptionStateConflict),
		stderrors.Is(err, marketrepo.ErrAgentAdoptionVariantTransition):
		// The refusal message IS the explicit reason (missing capability
		// names, conflicting state) — pass it through verbatim.
		return apperrors.NewConflictError(err.Error())
	case stderrors.Is(err, marketservice.ErrAgentAdoptionInvalidInput):
		return apperrors.NewValidationError("invalid agent adoption request")
	default:
		return err
	}
}

func (h *AgentAdoptionHandler) Adopt(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agentMarketplaceMaxRequestBytes)
	var body *adoptAgentBody
	if err := decodeAgentMarketplaceBody(c.Request.Body, &body); err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	if body == nil || strings.TrimSpace(body.ListingID) == "" {
		invalidMarketplaceBody(c, stderrors.New("listing_id is required"))
		return
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	view, created, err := h.adoptions.Adopt(c.Request.Context(), sandboxConfigTenantID(c), actorID, interfaces.AdoptInput{ListingID: body.ListingID, ReleaseID: body.ReleaseID})
	if err != nil {
		_ = c.Error(adoptionClientError(err))
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, gin.H{"success": true, "data": adoptionDTO(view)})
}

func (h *AgentAdoptionHandler) ListAdoptions(c *gin.Context) {
	views, err := h.adoptions.ListAdoptions(c.Request.Context(), sandboxConfigTenantID(c))
	if err != nil {
		_ = c.Error(adoptionClientError(err))
		return
	}
	data := make([]adoptionResponse, 0, len(views))
	for _, view := range views {
		data = append(data, adoptionDTO(view))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func (h *AgentAdoptionHandler) CreateVariant(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agentMarketplaceMaxRequestBytes)
	var body *createAdoptionVariantBody
	if err := decodeAgentMarketplaceBody(c.Request.Body, &body); err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	if body == nil || strings.TrimSpace(body.Name) == "" {
		invalidMarketplaceBody(c, stderrors.New("name is required"))
		return
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	view, err := h.adoptions.CreateVariant(c.Request.Context(), sandboxConfigTenantID(c), actorID, c.Param("id"), interfaces.VariantDraftInput{Name: body.Name, ReleaseID: body.ReleaseID})
	if err != nil {
		_ = c.Error(adoptionClientError(err))
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": adoptionVariantDTO(view)})
}
```

`internal/router/routes_agent_adoption.go`：

```go
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/handler"
)

// RegisterAgentAdoptionRoutes mounts the Tenant Adoption governance
// workflow beside the tenant release routes. Writing an Adoption, deriving
// Variants, mapping capabilities, testing and publishing are admin-grade
// governance acts (spec §13 adopt_agent / configure_variant /
// test_variant / publish_agent_version), so every mutation is Admin+ with
// the full-access API-key floor; only the available-agent read model is
// Viewer+ (spec §2 mobile read-only surface).
func RegisterAgentAdoptionRoutes(r *gin.RouterGroup, adoptionHandler *handler.AgentAdoptionHandler, g *rbacGuards) {
	if adoptionHandler == nil {
		return
	}
	admin := apiKeyFullAccess()
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/tenant/adoptions", admin, g.Admin(), adoptionHandler.Adopt)
	g.apiKeyRoute(r, http.MethodGet, "/marketplace/tenant/adoptions", admin, g.Admin(), adoptionHandler.ListAdoptions)
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/tenant/adoptions/:id/variants", admin, g.Admin(), adoptionHandler.CreateVariant)
}
```

接线一——`internal/router/router.go`：在 Params 结构体 `AgentMarketplaceHandler      *handler.AgentMarketplaceHandler`（`:101`）下一行加：

```go
	AgentAdoptionHandler         *handler.AgentAdoptionHandler
```

在 `RegisterAgentMarketplaceRoutes(v1, params.AgentMarketplaceHandler, rbacGuards)`（`:398`）下一行加：

```go
		RegisterAgentAdoptionRoutes(v1, params.AgentAdoptionHandler, rbacGuards)
```

接线二——`internal/container/container.go`：在 `must(container.Provide(handler.NewAgentMarketplaceHandler))`（`:431`）之后追加：

```go
	must(container.Provide(repository.NewAgentAdoptionRepository))
	must(container.Provide(func(repo repository.AgentAdoptionRepository, agents interfaces.CustomAgentService, versions interfaces.AgentVersionService) interfaces.AgentAdoptionService {
		return service.NewAgentAdoptionService(repo, agents, versions)
	}))
	must(container.Provide(handler.NewAgentAdoptionHandler))
```

- [ ] **Step 4: 运行确认通过（含生产装配编译）**

Run: `go build ./internal/... && go test ./internal/router/ -run 'TestTenantAgentAdoptionRoutesAndAuthorization' -count=1 -v`
Expected: build exit 0；测试 PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/handler/agent_adoption.go internal/router/routes_agent_adoption.go internal/router/routes_agent_adoption_test.go internal/router/router.go internal/container/container.go
git commit -m "feat(marketplace): adoption/variant HTTP governance endpoints with admin authorization (T29 #59 task 4)"
```

---

### Task 5: HTTP 面 II——CapabilityMapping 与 Test 门禁（AC2）

**Files:**
- Modify: `internal/handler/agent_adoption.go`（文件末尾追加 `UpdateCapabilityMapping` / `TestVariant` 方法与请求体类型）
- Modify: `internal/router/routes_agent_adoption.go`（追加两条路由）
- Test: `internal/router/routes_agent_adoption_test.go`（追加测试函数）

**Interfaces:**
- Consumes: Task 4 的 `adoptionClientError`/`adoptionVariantDTO`/`decodeAgentMarketplaceBody`/`invalidMarketplaceBody`/`sandboxConfigTenantID`、测试脚手架 `newAgentAdoptionTestApp`/`adoptionCall`/`publishAdoptionRelease`；Task 3 服务方法 `UpdateCapabilityMapping`/`TestVariant`。
- Produces: `PUT /api/v1/marketplace/tenant/variants/:id/capability-mapping`（请求体 `{ "mappings": [{ "capability", "model_id?", "knowledge_base_ids?", "connection_ids?" }] }`，响应 `data = adoptionVariantResponse`）；`POST /api/v1/marketplace/tenant/variants/:id/test`（响应同上）。AC2 的 HTTP 证据：409 冲突体 message 逐项点名缺失能力。

- [ ] **Step 1: 写失败测试**

在 `internal/router/routes_agent_adoption_test.go` 末尾追加：

```go
func TestTenantAgentVariantCapabilityMappingAndTestGate(t *testing.T) {
	r, _, _ := newAgentAdoptionTestApp(t)
	listingID, _ := publishAdoptionRelease(t, r)
	adopted := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID})
	require.Equal(t, http.StatusCreated, adopted.Code, adopted.Body.String())
	var adoptionBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(adopted.Body.Bytes(), &adoptionBody))
	variant := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionBody.Data.ID+"/variants", "admin", "admin", map[string]any{"name": "Sales Assistant"})
	require.Equal(t, http.StatusCreated, variant.Code, variant.Body.String())
	var variantBody struct {
		Data struct {
			ID                  string   `json:"id"`
			State               string   `json:"state"`
			MissingCapabilities []string `json:"missing_capabilities"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(variant.Body.Bytes(), &variantBody))

	assertConflict := func(response *httptest.ResponseRecorder, message string) {
		t.Helper()
		require.Equal(t, http.StatusConflict, response.Code, response.Body.String())
		var envelope struct {
			Success bool `json:"success"`
			Error   struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
		require.False(t, envelope.Success)
		require.Equal(t, 1005, envelope.Error.Code)
		require.Equal(t, message, envelope.Error.Message)
	}

	// AC2 over HTTP: missing required capabilities -> 409 with every missing
	// capability named in the message.
	assertConflict(
		adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/test", "admin", "admin", nil),
		"agent adoption variant is not runnable: missing required capabilities: knowledge, model",
	)

	partial := adoptionCall(r, 1, http.MethodPut, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/capability-mapping", "admin", "admin", map[string]any{"mappings": []map[string]any{{"capability": "model", "model_id": "gpt-x"}}})
	require.Equal(t, http.StatusOK, partial.Code, partial.Body.String())
	require.NoError(t, json.Unmarshal(partial.Body.Bytes(), &variantBody))
	require.Equal(t, "draft", variantBody.Data.State)
	require.Equal(t, []string{"knowledge"}, variantBody.Data.MissingCapabilities)
	assertConflict(
		adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/test", "admin", "admin", nil),
		"agent adoption variant is not runnable: missing required capabilities: knowledge",
	)

	unknownCapability := adoptionCall(r, 1, http.MethodPut, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/capability-mapping", "admin", "admin", map[string]any{"mappings": []map[string]any{{"capability": "sandbox"}}})
	require.Equal(t, http.StatusBadRequest, unknownCapability.Code, unknownCapability.Body.String())
	duplicateCapability := adoptionCall(r, 1, http.MethodPut, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/capability-mapping", "admin", "admin", map[string]any{"mappings": []map[string]any{{"capability": "model", "model_id": "a"}, {"capability": "model", "model_id": "b"}}})
	require.Equal(t, http.StatusBadRequest, duplicateCapability.Code)
	emptyBinding := adoptionCall(r, 1, http.MethodPut, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/capability-mapping", "admin", "admin", map[string]any{"mappings": []map[string]any{{"capability": "model"}, {"capability": "knowledge"}}})
	require.Equal(t, http.StatusOK, emptyBinding.Code)
	require.NoError(t, json.Unmarshal(emptyBinding.Body.Bytes(), &variantBody))
	require.Equal(t, []string{"knowledge", "model"}, variantBody.Data.MissingCapabilities, "empty bindings never cover a capability")
	viewerMapping := adoptionCall(r, 1, http.MethodPut, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/capability-mapping", "viewer", "viewer", map[string]any{"mappings": []map[string]any{}})
	require.Equal(t, http.StatusForbidden, viewerMapping.Code)
	crossTenantMapping := adoptionCall(r, 2, http.MethodPut, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/capability-mapping", "admin", "admin", map[string]any{"mappings": []map[string]any{}})
	require.Equal(t, http.StatusNotFound, crossTenantMapping.Code)
	viewerTest := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/test", "viewer", "viewer", nil)
	require.Equal(t, http.StatusForbidden, viewerTest.Code)

	complete := adoptionCall(r, 1, http.MethodPut, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/capability-mapping", "admin", "admin", map[string]any{"mappings": []map[string]any{
		{"capability": "model", "model_id": "gpt-x"},
		{"capability": "knowledge", "knowledge_base_ids": []string{"kb-sales"}},
	}})
	require.Equal(t, http.StatusOK, complete.Code, complete.Body.String())
	require.NoError(t, json.Unmarshal(complete.Body.Bytes(), &variantBody))
	require.Equal(t, "mapped", variantBody.Data.State)
	require.Empty(t, variantBody.Data.MissingCapabilities)

	tested := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/test", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, tested.Code, tested.Body.String())
	require.NoError(t, json.Unmarshal(tested.Body.Bytes(), &variantBody))
	require.Equal(t, "tested", variantBody.Data.State)
	// Re-testing a tested variant is a state conflict, not a silent re-run.
	assertConflict(
		adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/test", "admin", "admin", nil),
		"agent adoption variant state conflict: state is \"tested\"; complete capability mapping first",
	)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/router/ -run 'TestTenantAgentVariantCapabilityMappingAndTestGate' -count=1`
Expected: FAIL——首个 `assertConflict` 收到 404（路由未注册），断言 `Expected status code 409 ... 404`。

- [ ] **Step 3: 写最小实现**

在 `internal/handler/agent_adoption.go` 的 `createAdoptionVariantBody` 类型后追加请求体类型：

```go
type capabilityMappingEntryBody struct {
	Capability       string   `json:"capability"`
	ModelID          string   `json:"model_id,omitempty"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids,omitempty"`
	ConnectionIDs    []string `json:"connection_ids,omitempty"`
}

type updateCapabilityMappingBody struct {
	Mappings []capabilityMappingEntryBody `json:"mappings"`
}
```

在文件末尾追加方法：

```go
func (h *AgentAdoptionHandler) UpdateCapabilityMapping(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agentMarketplaceMaxRequestBytes)
	var body *updateCapabilityMappingBody
	if err := decodeAgentMarketplaceBody(c.Request.Body, &body); err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	if body == nil || body.Mappings == nil {
		invalidMarketplaceBody(c, stderrors.New("mappings are required"))
		return
	}
	mappings := make([]interfaces.CapabilityMapping, 0, len(body.Mappings))
	for _, entry := range body.Mappings {
		mappings = append(mappings, interfaces.CapabilityMapping{
			Capability: entry.Capability, ModelID: entry.ModelID,
			KnowledgeBaseIDs: entry.KnowledgeBaseIDs, ConnectionIDs: entry.ConnectionIDs,
		})
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	view, err := h.adoptions.UpdateCapabilityMapping(c.Request.Context(), sandboxConfigTenantID(c), actorID, c.Param("id"), mappings)
	if err != nil {
		_ = c.Error(adoptionClientError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": adoptionVariantDTO(view)})
}

func (h *AgentAdoptionHandler) TestVariant(c *gin.Context) {
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	view, err := h.adoptions.TestVariant(c.Request.Context(), sandboxConfigTenantID(c), actorID, c.Param("id"))
	if err != nil {
		_ = c.Error(adoptionClientError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": adoptionVariantDTO(view)})
}
```

在 `internal/router/routes_agent_adoption.go` 的 `RegisterAgentAdoptionRoutes` 内（`CreateVariant` 路由之后）追加：

```go
	g.apiKeyRoute(r, http.MethodPut, "/marketplace/tenant/variants/:id/capability-mapping", admin, g.Admin(), adoptionHandler.UpdateCapabilityMapping)
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/tenant/variants/:id/test", admin, g.Admin(), adoptionHandler.TestVariant)
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/router/ -run 'TestTenantAgentVariantCapabilityMappingAndTestGate|TestTenantAgentAdoptionRoutesAndAuthorization' -count=1 -v`
Expected: PASS（2 个测试）。

- [ ] **Step 5: Commit**

```bash
git add internal/handler/agent_adoption.go internal/router/routes_agent_adoption.go internal/router/routes_agent_adoption_test.go
git commit -m "feat(marketplace): variant capability-mapping and test gate endpoints with named missing capabilities (T29 #59 task 5)"
```

---

### Task 6: HTTP 面 III——Publish 与移动 Available Agent 读模型（AC1/AC3 端到端）

**Files:**
- Modify: `internal/handler/agent_adoption.go`（末尾追加 `PublishVariant` / `ListAvailableAgents` 方法与 `availableAgentResponse` 类型）
- Modify: `internal/router/routes_agent_adoption.go`（追加两条路由）
- Test: `internal/router/routes_agent_adoption_test.go`（追加测试函数）

**Interfaces:**
- Consumes: Task 3 `PublishVariant`/`ListAvailableAgents`；Task 4/5 脚手架与 DTO；#33 wire 事实（`GET /api/v1/agents` 信封 `{success, data: CustomAgent[], disabled_own_agent_ids}`，行字段 `id/name/description/is_builtin`——`internal/handler/custom_agent.go:268-271`、`packages/api-client/src/mobile/resources.ts:69-82`）；真实 `GET /api/v1/agents/:id/versions/:versionId`（#58，`internal/router/routes_agent_versions.go:43-44`）。
- Produces: `POST /api/v1/marketplace/tenant/variants/:id/publish`（响应 `{data: {variant: adoptionVariantResponse}}`）；`GET /api/v1/marketplace/tenant/available-agents`（Viewer+，响应 `{success, data: [{agent_id, variant_id, adoption_id, release_id, name, description, is_builtin, capability: {state, reason}}]}`）——Task 7 contracts 逐字镜像该形状。

- [ ] **Step 1: 写失败测试**

在 `internal/router/routes_agent_adoption_test.go` 末尾追加：

```go
func TestTenantAgentAdoptionPublishesIndependentVariantsIntoMobileAvailableAgents(t *testing.T) {
	r, _, db := newAgentAdoptionTestApp(t)
	listingID, releaseID := publishAdoptionRelease(t, r)
	adopted := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID})
	require.Equal(t, http.StatusCreated, adopted.Code, adopted.Body.String())
	var adoptionBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(adopted.Body.Bytes(), &adoptionBody))

	// AC2 over HTTP (publish gate): an un-mapped variant refuses publication
	// with the missing capabilities named.
	draft := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionBody.Data.ID+"/variants", "admin", "admin", map[string]any{"name": "Draft Gate Probe"})
	require.Equal(t, http.StatusCreated, draft.Code, draft.Body.String())
	var draftBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(draft.Body.Bytes(), &draftBody))
	draftPublish := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+draftBody.Data.ID+"/publish", "admin", "admin", nil)
	require.Equal(t, http.StatusConflict, draftPublish.Code, draftPublish.Body.String())
	require.Contains(t, draftPublish.Body.String(), "missing required capabilities: knowledge, model")

	mapAndTest := func(name, model, kb string) string {
		variant := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionBody.Data.ID+"/variants", "admin", "admin", map[string]any{"name": name})
		require.Equal(t, http.StatusCreated, variant.Code, variant.Body.String())
		var variantBody struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(variant.Body.Bytes(), &variantBody))
		mapping := adoptionCall(r, 1, http.MethodPut, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/capability-mapping", "admin", "admin", map[string]any{"mappings": []map[string]any{
			{"capability": "model", "model_id": model},
			{"capability": "knowledge", "knowledge_base_ids": []string{kb}},
		}})
		require.Equal(t, http.StatusOK, mapping.Code, mapping.Body.String())
		tested := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/test", "admin", "admin", nil)
		require.Equal(t, http.StatusOK, tested.Code, tested.Body.String())
		return variantBody.Data.ID
	}

	salesID := mapAndTest("Sales Assistant", "gpt-x", "kb-sales")
	legalID := mapAndTest("Legal Assistant", "gpt-legal", "kb-legal")

	// Nothing is published yet: the mobile agent projection carries no
	// variant agents and the read model is empty (the "not runnable" side
	// of the shelf gate — the agents simply do not exist as runnable).
	agentsBefore := adoptionCall(r, 1, http.MethodGet, "/api/v1/agents", "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, agentsBefore.Code, agentsBefore.Body.String())
	require.NotContains(t, agentsBefore.Body.String(), "Sales Assistant")
	require.NotContains(t, agentsBefore.Body.String(), "Legal Assistant")
	availableBefore := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/available-agents", "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, availableBefore.Code, availableBefore.Body.String())
	var availableBeforeBody struct {
		Data []struct {
			AgentID string `json:"agent_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(availableBefore.Body.Bytes(), &availableBeforeBody))
	require.Empty(t, availableBeforeBody.Data)

	publish := func(variantID string) (localAgentID, localVersionID string) {
		published := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantID+"/publish", "admin", "admin", nil)
		require.Equal(t, http.StatusOK, published.Code, published.Body.String())
		var publishBody struct {
			Data struct {
				Variant struct {
					ID                  string `json:"id"`
					State               string `json:"state"`
					LocalAgentID        string `json:"local_agent_id"`
					LocalAgentVersionID string `json:"local_agent_version_id"`
				} `json:"variant"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(published.Body.Bytes(), &publishBody))
		require.Equal(t, "published", publishBody.Data.Variant.State)
		require.NotEmpty(t, publishBody.Data.Variant.LocalAgentID)
		require.NotEmpty(t, publishBody.Data.Variant.LocalAgentVersionID)
		return publishBody.Data.Variant.LocalAgentID, publishBody.Data.Variant.LocalAgentVersionID
	}
	salesAgentID, salesVersionID := publish(salesID)
	legalAgentID, legalVersionID := publish(legalID)
	require.NotEqual(t, salesAgentID, legalAgentID)

	// AC3 (mobile entry): the REAL GET /api/v1/agents — the endpoint the #33
	// mobile Resource Shelf consumes — now carries both published local
	// agents with the wire fields the api-client maps.
	agentsAfter := adoptionCall(r, 1, http.MethodGet, "/api/v1/agents", "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, agentsAfter.Code, agentsAfter.Body.String())
	type mobileAgentRow struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		IsBuiltin   bool   `json:"is_builtin"`
		Config      struct {
			KnowledgeBases []string `json:"knowledge_bases"`
			ModelID        string   `json:"model_id"`
			SystemPrompt   string   `json:"system_prompt"`
		} `json:"config"`
	}
	var agentsList struct {
		Data []mobileAgentRow `json:"data"`
	}
	require.NoError(t, json.Unmarshal(agentsAfter.Body.Bytes(), &agentsList))
	byID := map[string]struct{}{}
	var salesRow, legalRow *mobileAgentRow
	for i := range agentsList.Data {
		byID[agentsList.Data[i].ID] = struct{}{}
		switch agentsList.Data[i].ID {
		case salesAgentID:
			salesRow = &agentsList.Data[i]
		case legalAgentID:
			legalRow = &agentsList.Data[i]
		}
	}
	require.Contains(t, byID, salesAgentID)
	require.Contains(t, byID, legalAgentID)
	// AC1 (independence): the two variants' local agents carry different
	// local mappings from one shared adoption.
	require.NotNil(t, salesRow)
	require.NotNil(t, legalRow)
	require.Equal(t, "Sales Assistant", salesRow.Name)
	require.Equal(t, "Legal Assistant", legalRow.Name)
	require.Equal(t, []string{"kb-sales"}, salesRow.Config.KnowledgeBases)
	require.Equal(t, "gpt-x", salesRow.Config.ModelID)
	require.Equal(t, []string{"kb-legal"}, legalRow.Config.KnowledgeBases)
	require.Equal(t, "gpt-legal", legalRow.Config.ModelID)
	require.Equal(t, "Be useful.", salesRow.Config.SystemPrompt, "portable payload behavior reaches the local agent")
	require.False(t, salesRow.IsBuiltin)

	// The published local Agent Version is real and readable (#58 endpoint).
	frozenVersion := adoptionCall(r, 1, http.MethodGet, "/api/v1/agents/"+salesAgentID+"/versions/"+salesVersionID, "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, frozenVersion.Code, frozenVersion.Body.String())
	require.Contains(t, frozenVersion.Body.String(), salesVersionID)
	_ = legalVersionID

	// Domain read model: two available agents with full lineage.
	available := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/available-agents", "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, available.Code, available.Body.String())
	var availableBody struct {
		Data []struct {
			AgentID    string `json:"agent_id"`
			VariantID  string `json:"variant_id"`
			AdoptionID string `json:"adoption_id"`
			ReleaseID  string `json:"release_id"`
			Name       string `json:"name"`
			Capability struct {
				State  string `json:"state"`
				Reason string `json:"reason"`
			} `json:"capability"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(available.Body.Bytes(), &availableBody))
	require.Len(t, availableBody.Data, 2)
	require.Equal(t, salesAgentID, availableBody.Data[0].AgentID)
	require.Equal(t, adoptionBody.Data.ID, availableBody.Data[0].AdoptionID)
	require.Equal(t, releaseID, availableBody.Data[0].ReleaseID)
	require.Equal(t, "supported", availableBody.Data[0].Capability.State)

	// Governance view: one adoption, two published variants.
	listed := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, listed.Code, listed.Body.String())
	require.Contains(t, listed.Body.String(), `"state":"published"`)

	// Review Focus 2: re-publishing refuses and does not duplicate the agent.
	rePublish := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+salesID+"/publish", "admin", "admin", nil)
	require.Equal(t, http.StatusConflict, rePublish.Code, rePublish.Body.String())
	var count int64
	require.NoError(t, db.Model(&types.CustomAgent{}).Where("tenant_id = ? AND name IN ?", 1, []string{"Sales Assistant", "Legal Assistant"}).Count(&count).Error)
	require.Equal(t, int64(2), count, "a refused re-publish must not create another local agent")

	// Review Focus 4: a tampered release bundle refuses publication (500,
	// fail closed) before any local agent is instantiated.
	tamperedID := mapAndTest("Compliance Assistant", "gpt-c", "kb-c")
	require.NoError(t, db.Model(&types.AgentReleaseEntity{}).Where("tenant_id = ? AND id = ?", 1, releaseID).Update("bundle", []byte(`{"payload":"tampered"}`)).Error)
	tamperedPublish := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+tamperedID+"/publish", "admin", "admin", nil)
	require.Equal(t, http.StatusInternalServerError, tamperedPublish.Code, tamperedPublish.Body.String())
	require.NoError(t, db.Model(&types.CustomAgent{}).Where("tenant_id = ? AND name = ?", 1, "Compliance Assistant").Count(&count).Error)
	require.Equal(t, int64(0), count, "a tampered release must not instantiate a local agent")

	// Review Focus 5: soft-deleting a published local agent removes it from
	// the read model immediately.
	require.NoError(t, db.Delete(&types.CustomAgent{}, "tenant_id = ? AND id = ?", 1, legalAgentID).Error)
	afterDelete := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/available-agents", "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, afterDelete.Code)
	require.NotContains(t, afterDelete.Body.String(), legalAgentID)

	viewerPublish := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+salesID+"/publish", "viewer", "viewer", nil)
	require.Equal(t, http.StatusForbidden, viewerPublish.Code)
	crossTenantAvailable := adoptionCall(r, 2, http.MethodGet, "/api/v1/marketplace/tenant/available-agents", "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, crossTenantAvailable.Code)
	var crossTenantAvailableBody struct {
		Data []struct {
			AgentID string `json:"agent_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(crossTenantAvailable.Body.Bytes(), &crossTenantAvailableBody))
	require.Empty(t, crossTenantAvailableBody.Data, "another tenant never sees tenant-1 availability")
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/router/ -run 'TestTenantAgentAdoptionPublishesIndependentVariantsIntoMobileAvailableAgents' -count=1`
Expected: FAIL——首个 publish 调用收到 404（路由未注册）。

- [ ] **Step 3: 写最小实现**

在 `internal/handler/agent_adoption.go` 的 `adoptionResponse` 类型后追加：

```go
type availableAgentCapability struct {
	State  string `json:"state"`
	Reason string `json:"reason"`
}

type availableAgentResponse struct {
	AgentID     string                   `json:"agent_id"`
	VariantID   string                   `json:"variant_id"`
	AdoptionID  string                   `json:"adoption_id"`
	ReleaseID   string                   `json:"release_id"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	IsBuiltin   bool                     `json:"is_builtin"`
	Capability  availableAgentCapability `json:"capability"`
}
```

在文件末尾追加方法：

```go
func (h *AgentAdoptionHandler) PublishVariant(c *gin.Context) {
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	result, err := h.adoptions.PublishVariant(c.Request.Context(), sandboxConfigTenantID(c), actorID, c.Param("id"))
	if err != nil {
		_ = c.Error(adoptionClientError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"variant": adoptionVariantDTO(result.Variant)}})
}

func (h *AgentAdoptionHandler) ListAvailableAgents(c *gin.Context) {
	views, err := h.adoptions.ListAvailableAgents(c.Request.Context(), sandboxConfigTenantID(c))
	if err != nil {
		_ = c.Error(adoptionClientError(err))
		return
	}
	data := make([]availableAgentResponse, 0, len(views))
	for _, view := range views {
		data = append(data, availableAgentResponse{
			AgentID: view.AgentID, VariantID: view.VariantID, AdoptionID: view.AdoptionID,
			ReleaseID: view.ReleaseID, Name: view.Name, Description: view.Description,
			IsBuiltin: view.IsBuiltin,
			Capability: availableAgentCapability{State: view.Capability.State, Reason: view.Capability.Reason},
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}
```

在 `internal/router/routes_agent_adoption.go` 的 `RegisterAgentAdoptionRoutes` 内（test 路由之后）追加：

```go
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/tenant/variants/:id/publish", admin, g.Admin(), adoptionHandler.PublishVariant)
	g.apiKeyRoute(r, http.MethodGet, "/marketplace/tenant/available-agents", admin, g.Viewer(), adoptionHandler.ListAvailableAgents)
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/router/ -run 'TestTenantAgentAdoption|TestTenantAgentVariantCapabilityMappingAndTestGate|TestTenantAgentMarketplaceLifecycleAndAuthorization' -count=1 -v`
Expected: PASS（4 个测试——含 #58 既有回归）。

- [ ] **Step 5: Commit**

```bash
git add internal/handler/agent_adoption.go internal/router/routes_agent_adoption.go internal/router/routes_agent_adoption_test.go
git commit -m "feat(marketplace): variant publish endpoint and mobile available-agent read model (T29 #59 task 6)"
```

---

### Task 7: contracts 契约层

**Files:**
- Create: `packages/contracts/src/marketplace/agent-adoption.ts`
- Test: `packages/contracts/src/marketplace/agent-adoption.test.ts`
- Modify: `packages/contracts/src/index.ts:726`（既有 marketplace re-export 块之后追加 2 行）

**Interfaces:**
- Consumes: `ContractError`（`packages/contracts/src/index.ts`）与 `tenant-releases.ts` 的解析器惯例（`packages/contracts/src/marketplace/tenant-releases.ts:66-110` 的 `object`/`requiredString`/`envelopeData` 模式）。
- Produces: `AgentAdoption` / `AgentAdoptionVariant` / `AvailableAgent` 类型与 `parseAdoptionResponse` / `parseAdoptionListResponse` / `parseVariantResponse` / `parseAvailableAgentListResponse`（从 `@weknora/contracts` 根导出）。

- [ ] **Step 1: 写失败测试**

`packages/contracts/src/marketplace/agent-adoption.test.ts`：

```ts
import assert from 'node:assert/strict';
import test from 'node:test';
import { ContractError } from '../index.ts';
import { parseAdoptionResponse, parseAdoptionListResponse, parseVariantResponse, parseAvailableAgentListResponse } from './agent-adoption.ts';

const variant = { id: 'variant-1', adoption_id: 'adoption-1', release_id: 'release-1', name: 'Sales Assistant', state: 'draft', missing_capabilities: ['knowledge', 'model'], created_at: '2026-09-24T00:00:00Z', updated_at: '2026-09-24T00:00:00Z' };
const adoption = { id: 'adoption-1', listing_id: 'listing-1', accepted_release_id: 'release-1', state: 'active', created_by: 'admin', created_at: '2026-09-24T00:00:00Z', updated_at: '2026-09-24T00:00:00Z', variants: [variant] };
const available = { agent_id: 'agent-1', variant_id: 'variant-1', adoption_id: 'adoption-1', release_id: 'release-1', name: 'Sales Assistant', description: 'A helpful agent', is_builtin: false, capability: { state: 'supported', reason: '' } };

test('parses adoption with nested variants and missing capabilities', () => {
  const parsed = parseAdoptionResponse({ success: true, data: adoption });
  assert.equal(parsed.id, adoption.id);
  assert.equal(parsed.variants[0].missing_capabilities.join(','), 'knowledge,model');
  const published = parseVariantResponse({ success: true, data: { ...variant, state: 'published', local_agent_id: 'agent-1', local_agent_version_id: 'version-1', missing_capabilities: [] } });
  assert.equal(published.local_agent_id, 'agent-1');
  assert.equal(published.state, 'published');
  assert.deepEqual(parseAdoptionListResponse({ success: true, data: [adoption] }), [adoption]);
});

test('available agents carry lineage and a supported verdict', () => {
  const rows = parseAvailableAgentListResponse({ success: true, data: [available] });
  assert.equal(rows[0].agent_id, 'agent-1');
  assert.equal(rows[0].capability.state, 'supported');
  assert.deepEqual(parseAvailableAgentListResponse({ success: true, data: [] }), []);
});

test('rejects malformed ids, states, verdicts and envelopes', () => {
  assert.throws(() => parseAdoptionResponse({ success: true, data: { ...adoption, id: '' } }), ContractError);
  assert.throws(() => parseAdoptionResponse({ success: true, data: { ...adoption, listing_id: '' } }), ContractError);
  assert.throws(() => parseAdoptionResponse({ success: false, data: adoption }), ContractError);
  assert.throws(() => parseAdoptionListResponse({ success: true, data: adoption }), ContractError);
  assert.throws(() => parseVariantResponse({ success: true, data: { ...variant, state: 'unknown-state' } }), ContractError);
  assert.throws(() => parseVariantResponse({ success: true, data: { ...variant, missing_capabilities: 'knowledge' } }), ContractError);
  assert.throws(() => parseAvailableAgentListResponse({ success: true, data: [{ ...available, agent_id: '' }] }), ContractError);
  assert.throws(() => parseAvailableAgentListResponse({ success: true, data: [{ ...available, capability: { state: 'maybe', reason: '' } }] }), ContractError);
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/contracts/src/marketplace/agent-adoption.test.ts`
Expected: FAIL——`Cannot find module '.../agent-adoption.ts'`。

- [ ] **Step 3: 写最小实现**

`packages/contracts/src/marketplace/agent-adoption.ts`：

```ts
import { ContractError } from '../index.ts';

export interface AgentAdoptionVariant {
  id: string;
  adoption_id: string;
  release_id: string;
  name: string;
  state: 'draft' | 'mapped' | 'tested' | 'published';
  local_agent_id?: string;
  local_agent_version_id?: string;
  missing_capabilities: string[];
  created_at: string;
  updated_at: string;
  [key: string]: unknown;
}

export interface AgentAdoption {
  id: string;
  listing_id: string;
  accepted_release_id: string;
  state: string;
  created_by: string;
  created_at: string;
  updated_at: string;
  variants: AgentAdoptionVariant[];
  [key: string]: unknown;
}

export interface AvailableAgent {
  agent_id: string;
  variant_id: string;
  adoption_id: string;
  release_id: string;
  name: string;
  description: string;
  is_builtin: boolean;
  capability: { state: 'supported' | 'unavailable' | 'forbidden'; reason: string };
  [key: string]: unknown;
}

const VARIANT_STATES = ['draft', 'mapped', 'tested', 'published'];
const CAPABILITY_STATES = ['supported', 'unavailable', 'forbidden'];

function object(value: unknown, path: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new ContractError(path, 'expected an object');
  return value as Record<string, unknown>;
}

function requiredString(row: Record<string, unknown>, key: string, path: string): string {
  const value = row[key];
  if (typeof value !== 'string' || value.trim() === '') throw new ContractError(`${path}.${key}`, 'expected a non-empty string');
  return value;
}

function requiredStringArray(row: Record<string, unknown>, key: string, path: string): string[] {
  const value = row[key];
  if (!Array.isArray(value) || value.some((item) => typeof item !== 'string')) throw new ContractError(`${path}.${key}`, 'expected an array of strings');
  return value as string[];
}

function enumValue<T extends string>(row: Record<string, unknown>, key: string, allowed: readonly T[], path: string): T {
  const value = row[key];
  if (typeof value !== 'string' || !(allowed as readonly string[]).includes(value)) {
    throw new ContractError(`${path}.${key}`, `expected one of ${allowed.join(', ')}`);
  }
  return value as T;
}

function envelopeData(value: unknown): unknown {
  const envelope = object(value, '');
  if (envelope.success !== true) throw new ContractError('success', 'expected true');
  if (!Object.prototype.hasOwnProperty.call(envelope, 'data')) throw new ContractError('data', 'is required');
  return envelope.data;
}

function optionalString(row: Record<string, unknown>, key: string): string | undefined {
  const value = row[key];
  return typeof value === 'string' && value !== '' ? value : undefined;
}

function parseVariantRow(value: unknown, path: string): AgentAdoptionVariant {
  const row = object(value, path);
  const variant: AgentAdoptionVariant = {
    ...row,
    id: requiredString(row, 'id', path),
    adoption_id: requiredString(row, 'adoption_id', path),
    release_id: requiredString(row, 'release_id', path),
    name: requiredString(row, 'name', path),
    state: enumValue(row, 'state', VARIANT_STATES, path),
    missing_capabilities: requiredStringArray(row, 'missing_capabilities', path),
    created_at: requiredString(row, 'created_at', path),
    updated_at: requiredString(row, 'updated_at', path),
  };
  const localAgentId = optionalString(row, 'local_agent_id');
  if (localAgentId !== undefined) variant.local_agent_id = localAgentId;
  const localVersionId = optionalString(row, 'local_agent_version_id');
  if (localVersionId !== undefined) variant.local_agent_version_id = localVersionId;
  return variant;
}

function parseAdoptionRow(value: unknown, path: string): AgentAdoption {
  const row = object(value, path);
  const variants = row.variants;
  if (!Array.isArray(variants)) throw new ContractError(`${path}.variants`, 'expected an array');
  return {
    ...row,
    id: requiredString(row, 'id', path),
    listing_id: requiredString(row, 'listing_id', path),
    accepted_release_id: requiredString(row, 'accepted_release_id', path),
    state: requiredString(row, 'state', path),
    created_by: typeof row.created_by === 'string' ? row.created_by : '',
    created_at: requiredString(row, 'created_at', path),
    updated_at: requiredString(row, 'updated_at', path),
    variants: variants.map((variant, index) => parseVariantRow(variant, `${path}.variants[${index}]`)),
  } as AgentAdoption;
}

export function parseAdoptionResponse(value: unknown): AgentAdoption {
  return parseAdoptionRow(envelopeData(value), 'data');
}

export function parseAdoptionListResponse(value: unknown): AgentAdoption[] {
  const data = envelopeData(value);
  if (!Array.isArray(data)) throw new ContractError('data', 'expected an array');
  return data.map((row, index) => parseAdoptionRow(row, `data[${index}]`));
}

export function parseVariantResponse(value: unknown): AgentAdoptionVariant {
  return parseVariantRow(envelopeData(value), 'data');
}

export function parseAvailableAgentListResponse(value: unknown): AvailableAgent[] {
  const data = envelopeData(value);
  if (!Array.isArray(data)) throw new ContractError('data', 'expected an array');
  return data.map((row, index) => {
    const path = `data[${index}]`;
    const recordRow = object(row, path);
    const capability = object(recordRow.capability, `${path}.capability`);
    return {
      ...recordRow,
      agent_id: requiredString(recordRow, 'agent_id', path),
      variant_id: requiredString(recordRow, 'variant_id', path),
      adoption_id: requiredString(recordRow, 'adoption_id', path),
      release_id: requiredString(recordRow, 'release_id', path),
      name: requiredString(recordRow, 'name', path),
      description: typeof recordRow.description === 'string' ? recordRow.description : '',
      is_builtin: recordRow.is_builtin === true,
      capability: {
        state: enumValue(capability, 'state', CAPABILITY_STATES, `${path}.capability`),
        reason: typeof capability.reason === 'string' ? capability.reason : '',
      },
    } as AvailableAgent;
  });
}
```

`packages/contracts/src/index.ts`：在既有 marketplace re-export 两行（`:725-726`）之后追加：

```ts
export type { AgentAdoption, AgentAdoptionVariant, AvailableAgent } from './marketplace/agent-adoption.ts';
export { parseAdoptionResponse, parseAdoptionListResponse, parseVariantResponse, parseAvailableAgentListResponse } from './marketplace/agent-adoption.ts';
```

- [ ] **Step 4: 运行确认通过（含既有 contracts 测试回归）**

Run: `npx tsx --test packages/contracts/src/marketplace/agent-adoption.test.ts packages/contracts/src/marketplace/tenant-releases.test.ts`
Expected: PASS（两文件全部用例，fail 0）。

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src/marketplace/agent-adoption.ts packages/contracts/src/marketplace/agent-adoption.test.ts packages/contracts/src/index.ts
git commit -m "feat(contracts): adoption/variant/available-agent wire contracts (T29 #59 task 7)"
```

---

## 计划级验证

在 worktree 根执行（覆盖本计划全部定向测试 + 生产装配编译；避免全量 flaky 套件）：

```sh
go build ./internal/... && go test ./internal/database/ -run 'TestSQLiteMigrations|TestSemanticMigrationSQLiteUpDownUp' -count=1 && go test ./internal/application/repository/ -run 'TestAgentAdoption' -count=1 && go test ./internal/application/service/ -run 'TestAgentAdoption' -count=1 && go test ./internal/router/ -run 'TestTenantAgentAdoption|TestTenantAgentVariantCapabilityMappingAndTestGate|TestTenantAgentMarketplaceLifecycleAndAuthorization' -count=1 && npx tsx --test packages/contracts/src/marketplace/agent-adoption.test.ts
```

预期：全部通过（`go build` exit 0；database 3 测试；repository 2 测试；service 3 测试；router 4 测试；contracts 3 用例组）。

## 验收标准 → 证据映射

| 验收标准 | 证据 |
|---|---|
| AC1 同一 Adoption 支持多个独立 Variant | Task 3 服务测试（`agents.created` 2 个、KB/model 各异、ID 不同）+ Task 6 HTTP 测试（同一 adoption 的 Sales/Legal 均发布、`GET /api/v1/agents` 两行、`available-agents` 两行、lineage 齐全） |
| AC2 缺少必需能力时 Variant 不可运行且原因明确 | Task 3（`ErrAgentAdoptionVariantNotRunnable` + 逐项点名）+ Task 5 HTTP（409 message `"agent adoption variant is not runnable: missing required capabilities: knowledge, model"`，部分映射时精确到 `knowledge`）+ Task 6（发布前移动投影中不存在任何 Variant Agent、available-agents 为空——不可运行=不可进入投影） |
| AC3 端到端行为通过最高稳定 Interface 验证 | Task 6 `TestTenantAgentAdoptionPublishesIndependentVariantsIntoMobileAvailableAgents`：真实迁移 DB + 真实服务栈 + 真实 HTTP（release→adopt→variant→mapping→test→publish→`GET /api/v1/agents`（#33 移动 Resource Shelf 的 wire 来源）→`GET /api/v1/agents/:id/versions/:versionId`→`GET /marketplace/tenant/available-agents`）；Task 2/3 的下层测试在计划中如实标注为下层证据 |
