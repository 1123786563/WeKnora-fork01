# T31：Agent Upgrade Proposal 与渐进升级（Issue #61）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在已合并的 #59（Tenant Adoption / Agent Variant）与 #60（Public Marketplace / 跨 Tenant 引入台账）之上实现 spec §9 的升级层：Listing 指向新 Release 时只为既有 Adoption 生成**可审阅的 Agent Upgrade Proposal**（差异覆盖行为、依赖、安全和许可四维）；管理员接受建议只是以新 Release 创建一个新的 Variant 草稿，随后走既有 #59 的重新映射→测试→发布流程——被接受的 Release、其他 Variant、既有本地 Agent Version 与既有 Task 永不因新 Release 的出现而改变。

**Architecture:** 全部为 Go 后端新增层，不修改任何 TS 代码（spec §2：升级不在移动端执行，同 #59 判例）。数据层新增一张表 `agent_upgrade_proposals`（双迁移流 versioned 000198 / sqlite 000119）；仓储层 `internal/application/repository/agent_upgrade.go` 通过**接口嵌入**复用 `AgentAdoptionRepository`（从而继承 #60 的引入台账回退读取 `GetMarketplaceListing`/`GetRelease`），只新增 proposal 行的 find-or-create / CAS / 列表三个原语；服务层 `internal/application/service/agent_upgrade.go` 以**读取时对账（reconcile）**统一覆盖 Tenant 本地发布与引入式两条链：Listing 当前 Release（本地行或引入台账合成行）落后于 Adoption 已接受 Release 时，经真实仓储解析两份 Release，由纯函数 `diffUpgradeBundles`（`agent_upgrade_diff.go`）计算行为/依赖/安全/许可四维差异并随 proposal 落盘（Release 不可变 → 差异记录可审计）；接受/驳回经 CAS 状态机（open→accepted|dismissed，均为终态）。HTTP 层新增 `/marketplace/tenant/upgrade-proposals` 路由族（Admin+ full-access 地板，与 Adoption 治理面同款）；端到端证据落在真实迁移流上的 router 级 HTTP 测试（与 #59 同判的最高稳定 Interface）。

**Tech Stack:** Go 1.26（`go.mod` `go 1.26.0`，module `github.com/Tencent/WeKnora`）、gin + gorm + golang-migrate（测试流复用 `internal/router/routes_agent_marketplace_test.go:349-373` 的 `openTenantAgentMarketplaceHTTPTestDB`、`internal/application/service/agent_version_test.go:31-59` 的 `openAgentVersionServiceTestDB`，均为真实 sqlite 全量迁移）；测试断言 `github.com/stretchr/testify/require`。本计划无 TS/移动端改动、无外部凭据依赖。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-61.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-agent-marketplace-domain-model.md`（§9 升级、Fork 与退出；§4 聚合身份；§2 移动只读边界；§10 目录与 Release 状态；§17 完成边界）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（Testing Decisions：最高稳定 Interface 口径）
- 领域术语：`CONTEXT.md:137`（Agent 升级建议，逐字见 Global Constraints）、`CONTEXT.md:122/128`（Agent 引入 / Agent 变体）
- 相关 ADR：`docs/adr/0011-agent-marketplace-release-adoption-boundary.md`（Release 与 Adoption 边界——本计划 proposal 只引用两侧不可变 Release id，不复制内容）
- Parent：Issue #30；Blocked by：#59（已合并）、#60（已合并）；本 Issue 阻塞 #63（End Adoption 时禁止新升级建议——本计划的状态过滤已为其留好钩子）、#64（安全撤回传播）

## Global Constraints

以下为批准 Spec / Issue 的项目级约束，逐字引用，所有任务隐含遵守：

- 「新 Release 只生成可审阅升级建议，各 Variant 可独立重新映射、测试和发布。」（issue-61.md 正文 What to build）
- 「Listing 指向新 Release 时只生成 Agent Upgrade Proposal。管理员比较行为、依赖、能力、安全和许可变化，以新 Release 创建 Variant 升级草稿，重新映射、测试和发布。当前 Agent Version、Task 和其他 Variant不变化。」（agent-marketplace-domain-model.md §9）
- 「**Agent 升级建议（Agent Upgrade Proposal）**：当 Listing 提供新 Release 时，为现有 Agent Adoption 展示行为、能力需求、安全与许可差异的候选升级；接受建议只创建新的本地草稿，完成重新映射、测试和发布前不影响当前可用版本或既有 Task。」（CONTEXT.md:137）
- _避免_：「自动升级、覆盖当前 Agent Version、要求删除旧 Adoption 后重装。」（CONTEXT.md:138）
- 「移动端只显示已完成映射、测试和本地版本发布的 Available Agent。Marketplace 搜索、Submission、Review、Adoption 和升级不在移动端执行。」（同上 §2——本计划零 TS 改动）
- 「End Adoption：全部 Variant 退役后，禁止新 Variant 和升级建议，并归档 Adoption。」（同上 §9——End Adoption 本体属 #63，本计划只保证对账仅作用于 active Adoption）
- 「本地 Agent Version 的退出（Retire Variant / End Adoption）属 #63，安全撤回（Security Revoked / Dependency Security Blocked）属 #64，Fork 属 #62——本计划不得实现这些边界外的行为；差异计算的安全维在此指 Manifest 声明面（数据类别/外部副作用/能力需求）的扩大，不是扫描类安全状态。」
- 安全约束（会话注入）：数据库查询所有外部输入使用参数绑定，不得拼接 SQL；服务端请求 URL 仅允许 http/https 且发出前校验 host（本计划不新增外呼请求）；配置凭据只从环境变量或密钥服务读取，源码与测试不写入可用凭据字面量（本计划无新凭据）。
- 工作流约束：严格 RED→GREEN→REFACTOR；实现与已批准 Spec 冲突时升级而非静默重设计；「Tests target observable behavior at the highest stable Interface. Screen tests verify rendering and navigation; they do not duplicate Module internals.」（mobile-ai-office-design.md · Testing Decisions）。

**Issue #61 验收标准原文（docs/plans/issue30-sweep/issues/issue-61.md）：**

1. 「未接受的 Variant 和既有 Task 保持旧版本。」
2. 「升级差异覆盖行为、依赖、安全和许可。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

验收标准 3 的本地可验证性说明：本计划最高稳定 Interface 证据 = `internal/router/routes_agent_upgrade_test.go` 的真实 HTTP 端到端测试——真实 sqlite 迁移流（`openTenantAgentMarketplaceHTTPTestDB`，含本计划新增迁移 000119 与 Task 0 重编）、真实仓储、真实 `CustomAgentService`/`AgentVersionService`/`AgentMarketplaceService`/`AgentAdoptionService`/`AgentUpgradeService`、真实 `GET /api/v1/agents`（#33 移动 Resource Shelf 的 wire 来源）与 `GET /marketplace/tenant/available-agents`，仅以测试惯用的 RBAC 上下文中间件注入租户/角色（与已合并 #58/#59 同款）。无 blocked-env 验收项：本计划不改移动 TS、不需要真机或外部凭据。仓储测试（Task 2）、差异纯函数测试（Task 3）与服务测试（Task 4）在计划中如实标注为下层证据，不冒充 AC3。

## Review Focus

Spec 隐含但易咬人的五类输入/失效模式（每行后在所属任务以测试钉死）：

1. **零静默升级**（AC1 的反面）：Listing 前移后的任何路径——列建议、读建议、甚至发布 v2/v3——都不得改动既有 Variant 行、既有本地 Agent Definition 或既有冻结版本；`available-agents` 不得出现内容被替换的旧行。——Task 6 e2e（新 Release 落地 + 接受 + 新 Variant 全流程发布后，旧 agent 行 config 逐字段断言不变、旧 Variant state/release_id 不变、旧冻结版本 GET 仍 200）+ Task 2 仓储测试（proposal 行写入不触碰 adoption 行）。
2. **对账幂等 / 并发重复建行**：多次列建议或并发请求必须收敛为同一行（(tenant, adoption, to_release) 唯一索引 + find-or-create 竞态收敛），不得每次 GET 长出新行。——Task 2 `TestAgentUpgradeRepositoryFindOrCreateProposalIsIdempotent`（唯一索引真实驱动竞态分支）+ Task 6 e2e（连续两次 list 仍恰好 1 条 open）。
3. **跨租户 ID 枚举**：用他租户的 proposal_id / adoption_id 打本租户端点必须与不存在同形（404），他租户的建议永不出现在本租户列表。——Task 2 `TestAgentUpgradeRepositoryProposalTenantScope` + Task 6 e2e（tenant-2 列表为空、跨租户读单条 404）。
4. **状态机外的重复操作**：accept 已 accepted/dismissed、dismiss 已 resolved、对已终态 proposal 重复 materialize，必须显式 409 拒绝且不产生第二行/第二个 Variant 草稿。——Task 4 `TestAgentUpgradeServiceResolvesAndRefusesOutOfStateOperations` + Task 6 e2e（accept 后重复 accept 409、dismiss accepted 409、dismissed 行不复活）。
5. **损坏的 Release 载荷**：bundle/manifest/lock 任一 JSON 畸形时，差异计算 fail closed——不落半成品 proposal、列表端点不 500（跳过 + 日志，已存在行不动）；已落盘行的 DiffJSON 畸形属存储缺陷，读取必须显式报错而非静默空差异。——Task 3 `TestDiffUpgradeBundlesRejectsMalformedPayload` + Task 4 `TestAgentUpgradeServiceSkipsCorruptReleasesWithoutFailingTheListing` + Task 3 `TestDiffUpgradeBundlesIsDeterministic`。

## 任务结构与文件地图

| # | 任务 | 主要交付 |
|---|---|---|
| 0 | 迁移双号去重重编（波级前置修复） | mobile_device_app 迁移 000114→000118 / 000193→000197，恢复全量迁移流装载 |
| 1 | 双流迁移 + 持久化实体 + 迁移同步测试 | `agent_upgrade_proposals`（versioned 000198 / sqlite 000119）与实体、差异值类型 |
| 2 | Upgrade 仓储（嵌入 Adoption 仓储） | find-or-create（唯一索引幂等）+ CAS 状态迁移 + 租户作用域读取 |
| 3 | 四维差异纯函数 | 行为/依赖/安全/许可差异计算（确定性排序、fail closed） |
| 4 | Upgrade 服务（读取时对账 + 接受/驳回） | reconcile/list/get/accept/dismiss（含引入台账回退覆盖） |
| 5 | Handler + 路由 + 容器装配 | `/marketplace/tenant/upgrade-proposals` 四端点 + Admin 地板 + dig 装配 |
| 6 | 端到端 HTTP 证据（AC1/AC2/AC3） | 真实迁移流上完整生命周期：生成→审阅→接受→独立映射/测试/发布→旧版本不变 |

---

### Task 0: 迁移双号去重重编（波级前置修复，必须先行）

波起点 HEAD 存在同号双迁移：`migrations/sqlite/` 下 `000114_mobile_device_app`（#67，后落，fb9037710）与 `000114_public_agent_marketplace`（#60，先落，34565aa41）同号，`migrations/versioned/` 下 `000193_mobile_device_app` 与 `000193_public_agent_marketplace` 同号。golang-migrate 装载报 `duplicate migration file`，**实测复现**（本计划写作时在 worktree 执行）：

```
go test ./internal/router/ -run TestTenantAgentAdoptionRoutesAndAuthorization -count=1
→ Error: failed to open source, ".../migrations/sqlite": duplicate migration file: 000114_public_agent_marketplace.down.sql
```

全量迁移流是 Task 6 端到端测试的装载前提，因此本计划必须修复。按既有重编先例（60122179f：后到者前移），**后落的 mobile_device_app 前移**。序号占用核对（本计划写作时刻）：sqlite 000115/000116/000117 已被 app_publications（#48）/ task_compliance（#43）/ code_deliveries（#52）占用，000118 空闲；versioned 000194/000195/000196 同样已占用，000197 空闲。`public_agent_marketplace` 保持 000114/000193 不动——`internal/database/migration.go:33` 的 `sqliteAdoptionFKRelaxationMigrationVersion = 114` 与 `:123` 的 `os.Stat("migrations/sqlite/000114_public_agent_marketplace.up.sql")` 均钉住它，生产代码零改动。mobile_device_app 迁移只依赖 000058-000060 的 mobile 表（与 000115-000117 无交互），挪到其后顺序安全。**集成注记**：本批兄弟计划并行写作，若合并时 000118/000197 已被先集成者占用，按 plan-t48 既有约定「整体顺延、DDL 零变化」处理（重编本计划四个迁移文件并同步五处测试路径字符串），不改变本计划语义。

**Files:**
- Modify（重命名）: `migrations/sqlite/000114_mobile_device_app.up.sql` → `migrations/sqlite/000118_mobile_device_app.up.sql`
- Modify（重命名）: `migrations/sqlite/000114_mobile_device_app.down.sql` → `migrations/sqlite/000118_mobile_device_app.down.sql`
- Modify（重命名）: `migrations/versioned/000193_mobile_device_app.up.sql` → `migrations/versioned/000197_mobile_device_app.up.sql`
- Modify（重命名）: `migrations/versioned/000193_mobile_device_app.down.sql` → `migrations/versioned/000197_mobile_device_app.down.sql`
- Modify: `internal/handler/mobile_device_test.go:27-33`（cherry-pick 路径字符串）
- Modify: `internal/application/repository/mobile_device_test.go:26-31`（同上）
- Modify: `internal/application/repository/mobile_device_app_test.go:37,188,189`（同上）
- Modify: `internal/application/repository/mobile_push_isolation_test.go:61-67`（同上）
- Modify: `internal/modules/workbench/service/workbench/notification_app_policy_test.go:50`（同上）

**Interfaces:**
- Consumes: 无（纯修复）。
- Produces: 可装载的全量迁移流（Task 1 的 `TestSQLiteMigrationsCreateVersionedSchema` 与 Task 6 的 `openTenantAgentMarketplaceHTTPTestDB` 依赖它）；`000119`（sqlite）/`000198`（versioned）成为下一个空闲迁移号。

- [ ] **Step 1: 确认破损（RED）**

```sh
go test ./internal/router/ -run TestTenantAgentAdoptionRoutesAndAuthorization -count=1
```

Expected: FAIL，错误含 `duplicate migration file: 000114_public_agent_marketplace.down.sql`。

- [ ] **Step 2: 重命名四个迁移文件（git mv，保历史）**

```sh
git mv migrations/sqlite/000114_mobile_device_app.up.sql migrations/sqlite/000118_mobile_device_app.up.sql
git mv migrations/sqlite/000114_mobile_device_app.down.sql migrations/sqlite/000118_mobile_device_app.down.sql
git mv migrations/versioned/000193_mobile_device_app.up.sql migrations/versioned/000197_mobile_device_app.up.sql
git mv migrations/versioned/000193_mobile_device_app.down.sql migrations/versioned/000197_mobile_device_app.down.sql
```

- [ ] **Step 3: 更新五处测试文件中的路径字符串**

`internal/handler/mobile_device_test.go:27-33`，将注释与文件名列表改为：

```go
	// mobileDeviceRow 携带 AppID 后，旧 schema 库上的 INSERT 会缺 app_id 列；
	// 000118 的 intents 重建段要求 mobile_notification_intents 已存在，
	// 故必须 000059 → 000060 → 000118 三文件成组、顺序固定。
	for _, file := range []string{
		"000059_mobile_notifications.up.sql",
		"000060_mobile_notification_delivery.up.sql",
		"000118_mobile_device_app.up.sql",
	} {
```

`internal/application/repository/mobile_device_test.go:26-31`，同样改为：

```go
	// mobileDeviceRow 自带 AppID 后，旧 000058 schema 库上的 INSERT 会缺 app_id 列；
	// 与 openMobileHandlerDB 同构成组补齐 000059 → 000060 → 000118（顺序固定）。
	for _, file := range []string{
		"000059_mobile_notifications.up.sql",
		"000060_mobile_notification_delivery.up.sql",
		"000118_mobile_device_app.up.sql",
	} {
```

`internal/application/repository/mobile_device_app_test.go`，`:37` 改为 `"migrations/sqlite/000118_mobile_device_app.up.sql",`；`:188-189` 两行改为：

```go
		{file: "migrations/sqlite/000118_mobile_device_app.down.sql", deviceRebuild: "CREATE TABLE mobile_devices_rebuilt"},
		{file: "migrations/versioned/000197_mobile_device_app.down.sql", deviceRebuild: "ADD PRIMARY KEY (tenant_id, owner_id, device_id, environment)"},
```

`internal/application/repository/mobile_push_isolation_test.go:61-67`，改为：

```go
	// 顺序固定：000058 → 000059 → 000060 → 000118（000118 的重建段 INSERT...SELECT
	// 依赖前序迁移建出的 mobile_devices 与 mobile_notification_intents）。
	for _, file := range []string{
		"migrations/sqlite/000058_mobile_devices.up.sql",
		"migrations/sqlite/000059_mobile_notifications.up.sql",
		"migrations/sqlite/000060_mobile_notification_delivery.up.sql",
		"migrations/sqlite/000118_mobile_device_app.up.sql",
	} {
```

`internal/modules/workbench/service/workbench/notification_app_policy_test.go:50`，改为 `"migrations/sqlite/000118_mobile_device_app.up.sql",`。

- [ ] **Step 4: 全组受影响测试实跑确认通过（GREEN）**

```sh
go build ./internal/... && go test ./internal/database/ -run 'TestSQLiteMigrations|TestSemanticMigrationSQLiteUpDownUp' -count=1 && go test ./internal/application/repository/ -run 'TestAgentAdoption|TestMobileDevice|TestMobilePush|TestMobileDeviceApp' -count=1 && go test ./internal/application/service/ -run TestAgentAdoption -count=1 && go test ./internal/handler/ -run TestMobileDevice -count=1 && go test ./internal/router/ -run 'TestTenantAgentAdoption|TestTenantAgentVariant|TestTenantAgentAdoptionPublishes|TestAvailableAgentsAPIKeyFloor' -count=1 && go test ./internal/modules/workbench/service/workbench/ -run 'TestPushPayloadPolicy|TestAppRouting|TestHTTPNotificationProvider|TestDisabledNotificationProvider' -count=1
```

Expected: 全部 `ok`。（注：`internal/modules/workbench/service/workbench` 包内 `TestNotificationDeliveryRejectsResolvedInteractionAfterClaim` 在 pristine HEAD 上即失败，已用 `git stash` 实跑复核为既有失败、与本修复无关——它不引用本迁移，不纳入验证面，见「差异记录」第 4 条。）

- [ ] **Step 5: Commit**

```bash
git add migrations/sqlite/000118_mobile_device_app.up.sql migrations/sqlite/000118_mobile_device_app.down.sql migrations/versioned/000197_mobile_device_app.up.sql migrations/versioned/000197_mobile_device_app.down.sql internal/handler/mobile_device_test.go internal/application/repository/mobile_device_test.go internal/application/repository/mobile_device_app_test.go internal/application/repository/mobile_push_isolation_test.go internal/modules/workbench/service/workbench/notification_app_policy_test.go
git commit -m "fix(migrations): dedupe mobile_device_app version (000114->000118 / 000193->000197) broken by b4 merge (T31 #61 task 0)"
```

---

### Task 1: 双流迁移 + 持久化实体 + 迁移同步测试

**Files:**
- Create: `migrations/sqlite/000119_agent_upgrade_proposals.up.sql`
- Create: `migrations/sqlite/000119_agent_upgrade_proposals.down.sql`
- Create: `migrations/versioned/000198_agent_upgrade_proposals.up.sql`
- Create: `migrations/versioned/000198_agent_upgrade_proposals.down.sql`
- Create: `internal/types/agent_upgrade.go`
- Create: `internal/types/agent_upgrade_persistence.go`
- Modify: `internal/database/migration_sqlite_versioned_schema_test.go`（`versionedSQLiteTables` 列表尾部、索引断言列表尾部各加一行）

**Interfaces:**
- Consumes: Task 0 释放的空闲迁移号 000119/000198；既有实体风格（`internal/types/agent_adoption_persistence.go` 的 gorm tag 与复合主键约定）。
- Produces（后续任务逐字使用）:
  - `types.AgentUpgradeProposalEntity`（表 `agent_upgrade_proposals`，复合主键 `id+tenant_id`）：字段 `ID, TenantID, AdoptionID, ListingID, FromReleaseID, ToReleaseID, ToSemanticVersion, DiffJSON, State, AcceptedVariantID, ResolvedBy string, CreatedAt, UpdatedAt time.Time`
  - `types.UpgradeFieldChange{Field, From, To string}`（json: `field/from/to`）
  - `types.UpgradeDependencyChange{Type, ID, Change, FromVersion, ToVersion string}`（json: `type/id/change/from_version/to_version`；`change ∈ added|removed|version_changed|digest_changed`）
  - `types.UpgradeLicenseChange{Scope, ID, From, To string}`（json: `scope/id/from/to`；`scope ∈ release|dependency`）
  - `types.AgentUpgradeDiff{Behavior []UpgradeFieldChange; Dependencies []UpgradeDependencyChange; Security []UpgradeFieldChange; License []UpgradeLicenseChange}`（json: `behavior/dependencies/security/license`）

**FK 设计决策（与 #60 放宽迁移同理）：** `to_release_id` **不建** `agent_releases` 外键——跨 Tenant 引入的 Release 位于 `tenant_introduced_releases`，不在 `agent_releases`（这正是 #60 在 000114/000193 放宽 adoption 三条 FK 的原因）；`adoption_id` 保留对 `agent_adoptions(id, tenant_id)` 的复合外键（adoption 行在两条链上都真实存在）。租户隔离由每条查询的 `tenant_id` 绑定保持。

- [ ] **Step 1: 先写失败断言（RED）——把新表与唯一索引进迁移同步测试**

`internal/database/migration_sqlite_versioned_schema_test.go`：在 `versionedSQLiteTables` 列表末行 `"tenant_introduced_releases",`（:68）之后追加：

```go
	"agent_upgrade_proposals",
```

在索引断言列表 `"uq_agent_adoptions_scope", "uq_agent_variant_capability",`（:109）之后追加：

```go
		"uq_agent_upgrade_proposals_scope",
```

（该测试的期望头版本由 `sqliteMigrationHead` 从夹具根动态推导，新增迁移文件后自动跟随，无需改期望值。）

- [ ] **Step 2: 运行确认失败**

```sh
go test ./internal/database/ -run TestSQLiteMigrationsCreateVersionedSchema -count=1
```

Expected: FAIL，`SQLite migrations must create table agent_upgrade_proposals`。

- [ ] **Step 3: 写迁移 SQL（GREEN 第一半）**

`migrations/sqlite/000119_agent_upgrade_proposals.up.sql` 全文：

```sql
-- SQLite twin of versioned migration 000198.
--
-- T31 Agent Upgrade Proposal review table (ticket #61, spec §9). Listing
-- 指向新 Release 时为既有 Adoption 生成可审阅升级建议；接受建议只创建固定
-- 到 to_release_id 的新 Variant 草稿，既有 Variant / 本地 Agent Version /
-- Task 永不因此改变。
--
-- to_release_id 不建 agent_releases 外键：跨 Tenant 引入的 Release 位于
-- tenant_introduced_releases，不在 agent_releases（#60 在 000114/000193 放宽
-- adoption 三条 FK 的同一原因）；租户隔离由每条查询的 tenant_id 绑定保持。
CREATE TABLE agent_upgrade_proposals (
 id VARCHAR(36) NOT NULL, tenant_id INTEGER NOT NULL,
 adoption_id VARCHAR(36) NOT NULL, listing_id VARCHAR(36) NOT NULL,
 from_release_id VARCHAR(36) NOT NULL, to_release_id VARCHAR(36) NOT NULL,
 to_semantic_version VARCHAR(64) NOT NULL DEFAULT '',
 diff_json TEXT NOT NULL DEFAULT '{}', state VARCHAR(32) NOT NULL DEFAULT 'open',
 accepted_variant_id VARCHAR(36), resolved_by VARCHAR(255) NOT NULL DEFAULT '',
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY (id, tenant_id),
 FOREIGN KEY (adoption_id, tenant_id) REFERENCES agent_adoptions(id, tenant_id)
);
CREATE INDEX idx_agent_upgrade_proposals_adoption ON agent_upgrade_proposals(tenant_id, adoption_id, created_at);
CREATE UNIQUE INDEX uq_agent_upgrade_proposals_scope ON agent_upgrade_proposals(tenant_id, adoption_id, to_release_id);
```

`migrations/sqlite/000119_agent_upgrade_proposals.down.sql` 全文：

```sql
DROP INDEX IF EXISTS uq_agent_upgrade_proposals_scope;
DROP INDEX IF EXISTS idx_agent_upgrade_proposals_adoption;
DROP TABLE IF EXISTS agent_upgrade_proposals;
```

`migrations/versioned/000198_agent_upgrade_proposals.up.sql` 全文：

```sql
-- T31 Agent Upgrade Proposal review table (ticket #61, spec §9). Listing
-- 指向新 Release 时为既有 Adoption 生成可审阅升级建议；接受建议只创建固定
-- 到 to_release_id 的新 Variant 草稿。to_release_id 不建 agent_releases 外键
-- （引入式 Release 位于 tenant_introduced_releases，与 #60 放宽 adoption FK
-- 同理）；租户隔离由每条查询的 tenant_id 绑定保持。
CREATE TABLE agent_upgrade_proposals (
 id VARCHAR(36) NOT NULL, tenant_id BIGINT NOT NULL,
 adoption_id VARCHAR(36) NOT NULL, listing_id VARCHAR(36) NOT NULL,
 from_release_id VARCHAR(36) NOT NULL, to_release_id VARCHAR(36) NOT NULL,
 to_semantic_version VARCHAR(64) NOT NULL DEFAULT '',
 diff_json TEXT NOT NULL DEFAULT '{}', state VARCHAR(32) NOT NULL DEFAULT 'open',
 accepted_variant_id VARCHAR(36), resolved_by VARCHAR(255) NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY (id, tenant_id),
 FOREIGN KEY (adoption_id, tenant_id) REFERENCES agent_adoptions(id, tenant_id)
);
CREATE INDEX idx_agent_upgrade_proposals_adoption ON agent_upgrade_proposals(tenant_id, adoption_id, created_at);
CREATE UNIQUE INDEX uq_agent_upgrade_proposals_scope ON agent_upgrade_proposals(tenant_id, adoption_id, to_release_id);
```

`migrations/versioned/000198_agent_upgrade_proposals.down.sql` 全文：

```sql
DROP INDEX IF EXISTS uq_agent_upgrade_proposals_scope;
DROP INDEX IF EXISTS idx_agent_upgrade_proposals_adoption;
DROP TABLE IF EXISTS agent_upgrade_proposals;
```

- [ ] **Step 4: 写持久化实体与差异值类型（GREEN 第二半）**

`internal/types/agent_upgrade_persistence.go` 全文：

```go
package types

import "time"

// Agent Upgrade Proposal persistence entity (T31, Ticket #61; spec §9).
//
// A proposal is generated when a Listing's current Release advances past an
// Adoption's accepted Release (tenant-local publish or #60 introduction
// ledger — both resolve through the adoption repository's read fallbacks).
// Acceptance only ever creates a NEW Variant draft pinned to ToReleaseID;
// the previously accepted release, other Variants and existing Tasks never
// change (CONTEXT.md「Agent 升级建议」_避免_: 自动升级 / 覆盖当前 Agent
// Version). ToReleaseID carries no FK to agent_releases for the same reason
// migration 000114/000193 relaxed the adoption FKs: introduced releases live
// in tenant_introduced_releases.
type AgentUpgradeProposalEntity struct {
	ID                string `gorm:"type:varchar(36);primaryKey"`
	TenantID          uint64 `gorm:"primaryKey"`
	AdoptionID        string `gorm:"type:varchar(36);not null"`
	ListingID         string `gorm:"type:varchar(36);not null"`
	FromReleaseID     string `gorm:"type:varchar(36);not null"`
	ToReleaseID       string `gorm:"type:varchar(36);not null"`
	ToSemanticVersion string `gorm:"type:varchar(64);not null;default:''"`
	// DiffJSON is the canonical serialization of AgentUpgradeDiff, computed
	// once at materialization from the two immutable releases. Releases are
	// immutable, so the stored diff is the stable review record.
	DiffJSON          string `gorm:"type:text;not null;default:'{}'"`
	State             string `gorm:"type:varchar(32);not null;default:'open'"`
	AcceptedVariantID string `gorm:"type:varchar(36)"`
	ResolvedBy        string `gorm:"type:varchar(255);not null;default:''"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (AgentUpgradeProposalEntity) TableName() string { return "agent_upgrade_proposals" }
```

`internal/types/agent_upgrade.go` 全文：

```go
package types

// Agent Upgrade Proposal portable diff value types (T31, Ticket #61;
// spec §9, Issue #61 AC2).
//
// diffUpgradeBundles（internal/application/service）把两份不可变 Release 的
// 可审阅差异归入四个维度，逐字对应 Issue #61 AC2「升级差异覆盖行为、依赖、
// 安全和许可」：
//   - 行为 Behavior：便携 payload 字段级变化（模式/提示词/工具/技能/子代理/
//     开场提示），列表字段排序后拼接比较；
//   - 依赖 Dependencies：DependencyLock 按 (type, id) 对齐的增/删/版本变化/
//     摘要变化；
//   - 安全 Security：Manifest 声明面的扩大——能力需求、数据类别、外部副作用
//     （扫描类安全状态属 #64，不在本维度）；
//   - 许可 License：Release 许可与依赖许可的变化。
//
// 所有切片在生成时保持确定性顺序（固定字段序 + 排序键），同一对 Release
// 永远产出字节相同的差异。

// UpgradeFieldChange is one scalar or sorted-list field difference.
type UpgradeFieldChange struct {
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// UpgradeDependencyChange is one DependencyLock difference keyed by
// (type, id). Change ∈ added | removed | version_changed | digest_changed.
type UpgradeDependencyChange struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Change      string `json:"change"`
	FromVersion string `json:"from_version"`
	ToVersion   string `json:"to_version"`
}

// UpgradeLicenseChange is one license difference. Scope ∈ release |
// dependency; added/removed dependencies surface their license here with an
// empty From/To respectively.
type UpgradeLicenseChange struct {
	Scope string `json:"scope"`
	ID    string `json:"id"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// AgentUpgradeDiff is the four-dimension reviewable difference between the
// currently accepted Release and the proposed Release.
type AgentUpgradeDiff struct {
	Behavior     []UpgradeFieldChange      `json:"behavior"`
	Dependencies []UpgradeDependencyChange `json:"dependencies"`
	Security     []UpgradeFieldChange      `json:"security"`
	License      []UpgradeLicenseChange    `json:"license"`
}
```

- [ ] **Step 5: 运行迁移同步测试确认通过**

```sh
go test ./internal/database/ -run 'TestSQLiteMigrationsCreateVersionedSchema|TestSQLiteMigrationsUpgradeV4PreservesData' -count=1
```

Expected: PASS（2 个测试，含 000119 表 + `uq_agent_upgrade_proposals_scope` 索引 + Up/Down/Up 幂等）。

- [ ] **Step 6: Commit**

```bash
git add migrations/sqlite/000119_agent_upgrade_proposals.up.sql migrations/sqlite/000119_agent_upgrade_proposals.down.sql migrations/versioned/000198_agent_upgrade_proposals.up.sql migrations/versioned/000198_agent_upgrade_proposals.down.sql internal/types/agent_upgrade.go internal/types/agent_upgrade_persistence.go internal/database/migration_sqlite_versioned_schema_test.go
git commit -m "feat(marketplace): agent_upgrade_proposals migration and upgrade diff types (T31 #61 task 1)"
```

---

### Task 2: Upgrade 仓储（嵌入 Adoption 仓储）

**Files:**
- Create: `internal/application/repository/agent_upgrade.go`
- Test: `internal/application/repository/agent_upgrade_test.go`

**Interfaces:**
- Consumes: Task 1 的 `types.AgentUpgradeProposalEntity`；#59 仓储 `AgentAdoptionRepository`（`internal/application/repository/agent_adoption.go:39-52`，嵌入它即继承 `GetMarketplaceListing`/`GetRelease` 的引入台账回退、`ListAdoptions`、`GetAdoption`、`CreateVariant`、`ListCapabilityMappings`）；先例 `adoptListingTx` 的 find-or-create 竞态收敛模式（agent_adoption.go:76-106）与 `UpdateVariantState` 的 CAS 模式（agent_adoption.go:234-269）。
- Produces（Task 4/5/6 逐字使用）:

```go
type AgentUpgradeRepository interface {
	AgentAdoptionRepository // 嵌入：listing/release/adoption/variant/mapping 读取（含 #60 引入台账回退）
	FindOrCreateProposal(context.Context, *types.AgentUpgradeProposalEntity) (*types.AgentUpgradeProposalEntity, bool, error)
	GetProposal(context.Context, uint64, string) (*types.AgentUpgradeProposalEntity, error)
	ListProposals(context.Context, uint64) ([]types.AgentUpgradeProposalEntity, error)
	TransitionProposal(context.Context, uint64, string, []string, string, map[string]any) (*types.AgentUpgradeProposalEntity, error)
}
func NewAgentUpgradeRepository(db *gorm.DB) AgentUpgradeRepository
var ErrAgentUpgradeProposalTransition = errors.New("agent upgrade proposal state transition failed")
```

> 下层证据标注：本任务是仓储层 SQL 语义测试（AutoMigrate 直建表 + 真实唯一索引），如实为 Task 4/6 的下层支撑，不冒充 AC3。

- [ ] **Step 1: 写失败测试（RED）**

`internal/application/repository/agent_upgrade_test.go` 全文：

```go
package repository

// Agent-upgrade repository tests (T31 #61): find-or-create idempotence on
// the real unique index, CAS transitions, tenant scoping. Store-level SQL
// semantics under test — direct-DDL setup mirrors openAdoptionVariantDB.

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openUpgradeProposalDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.AgentUpgradeProposalEntity{}, &types.AgentAdoptionEntity{}))
	// AutoMigrate 不创建 uq_agent_upgrade_proposals_scope（实体无 uniqueIndex
	// tag）；显式补建使 FindOrCreateProposal 的竞态分支由真实唯一索引驱动，
	// 与迁移 000119/000198 的生产 DDL 一致。
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uq_agent_upgrade_proposals_scope ON agent_upgrade_proposals(tenant_id, adoption_id, to_release_id)").Error)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func TestAgentUpgradeRepositoryFindOrCreateProposalIsIdempotent(t *testing.T) {
	db := openUpgradeProposalDB(t)
	repo := NewAgentUpgradeRepository(db)
	ctx := context.Background()

	// 夹具基线：adoption 行存在；FindOrCreateProposal 只写 proposal 行，
	// 绝不触碰 adoption（AC1 的存储侧半边，HTTP 侧另一半在 Task 6 e2e）。
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{
		TenantID: 1, ID: "a1", ListingID: "l1", AcceptedReleaseID: "r1", State: "active", CreatedBy: "admin",
	}).Error)

	first, created, err := repo.FindOrCreateProposal(ctx, &types.AgentUpgradeProposalEntity{
		TenantID: 1, AdoptionID: "a1", ListingID: "l1", FromReleaseID: "r1", ToReleaseID: "r2",
		ToSemanticVersion: "1.1.0", DiffJSON: `{"behavior":[]}`, State: "open",
	})
	require.NoError(t, err)
	require.True(t, created)
	require.NotEmpty(t, first.ID)
	require.Equal(t, "open", first.State)

	// 同键重复 materialize：返回既有行，绝不新行。
	second, created, err := repo.FindOrCreateProposal(ctx, &types.AgentUpgradeProposalEntity{
		TenantID: 1, AdoptionID: "a1", ListingID: "l1", FromReleaseID: "r1", ToReleaseID: "r2", State: "open",
	})
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, first.ID, second.ID)

	// 同一 adoption 的新目标 Release → 新行。
	third, created, err := repo.FindOrCreateProposal(ctx, &types.AgentUpgradeProposalEntity{
		TenantID: 1, AdoptionID: "a1", ListingID: "l1", FromReleaseID: "r1", ToReleaseID: "r3", State: "open",
	})
	require.NoError(t, err)
	require.True(t, created)
	require.NotEqual(t, first.ID, third.ID)

	// 跨租户同键 → 独立行（租户隔离由复合主键承载）。
	fourth, created, err := repo.FindOrCreateProposal(ctx, &types.AgentUpgradeProposalEntity{
		TenantID: 2, AdoptionID: "a1", ListingID: "l1", FromReleaseID: "r1", ToReleaseID: "r2", State: "open",
	})
	require.NoError(t, err)
	require.True(t, created)

	// adoption 行原封未动（Review Focus 1 的存储侧断言）。
	var adoptionAfter types.AgentAdoptionEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", uint64(1), "a1").First(&adoptionAfter).Error)
	require.Equal(t, "r1", adoptionAfter.AcceptedReleaseID, "proposal 写入不触碰 adoption 行")
	require.Equal(t, "active", adoptionAfter.State)

	rows, err := repo.ListProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, rows, 2, "tenant 1 恰好两条：r2 与 r3")
	rows2, err := repo.ListProposals(ctx, 2)
	require.NoError(t, err)
	require.Len(t, rows2, 1)
	require.Equal(t, fourth.ID, rows2[0].ID)
}

func TestAgentUpgradeRepositoryTransitionProposalCAS(t *testing.T) {
	db := openUpgradeProposalDB(t)
	repo := NewAgentUpgradeRepository(db)
	ctx := context.Background()
	require.NoError(t, db.Create(&types.AgentUpgradeProposalEntity{
		TenantID: 1, ID: "p1", AdoptionID: "a1", ListingID: "l1",
		FromReleaseID: "r1", ToReleaseID: "r2", State: "open",
	}).Error)

	updated, err := repo.TransitionProposal(ctx, 1, "p1", []string{"open"}, "accepted",
		map[string]any{"accepted_variant_id": "v9", "resolved_by": "admin"})
	require.NoError(t, err)
	require.Equal(t, "accepted", updated.State)
	require.Equal(t, "v9", updated.AcceptedVariantID)
	require.Equal(t, "admin", updated.ResolvedBy)

	// 重放 open→accepted：期望集不再匹配，CAS 显式拒绝，不静默二次应用。
	_, err = repo.TransitionProposal(ctx, 1, "p1", []string{"open"}, "dismissed", nil)
	require.ErrorIs(t, err, ErrAgentUpgradeProposalTransition)

	// 终态之间互斥：accepted 不得再 dismissed。
	_, err = repo.TransitionProposal(ctx, 1, "p1", []string{"dismissed"}, "dismissed", nil)
	require.ErrorIs(t, err, ErrAgentUpgradeProposalTransition)

	// 不存在的行：明确 not found 哨兵。
	_, err = repo.TransitionProposal(ctx, 1, "missing", []string{"open"}, "accepted", nil)
	require.ErrorIs(t, err, ErrAgentUpgradeProposalNotFound)
}

func TestAgentUpgradeRepositoryProposalTenantScope(t *testing.T) {
	db := openUpgradeProposalDB(t)
	repo := NewAgentUpgradeRepository(db)
	ctx := context.Background()
	require.NoError(t, db.Create(&types.AgentUpgradeProposalEntity{
		TenantID: 1, ID: "p1", AdoptionID: "a1", ListingID: "l1",
		FromReleaseID: "r1", ToReleaseID: "r2", State: "open",
	}).Error)

	other, err := repo.GetProposal(ctx, 2, "p1")
	require.NoError(t, err)
	require.Nil(t, other, "他租户的同形 id 读取为不存在（不泄露存在性）")

	mine, err := repo.GetProposal(ctx, 1, "p1")
	require.NoError(t, err)
	require.NotNil(t, mine)
	require.Equal(t, "p1", mine.ID)
}
```

- [ ] **Step 2: 运行确认失败**

```sh
go test ./internal/application/repository/ -run TestAgentUpgradeRepository -count=1
```

Expected: FAIL（编译错误 `undefined: NewAgentUpgradeRepository` / `undefined: ErrAgentUpgradeProposalTransition` / `undefined: ErrAgentUpgradeProposalNotFound`）。

- [ ] **Step 3: 写最小实现（GREEN）**

`internal/application/repository/agent_upgrade.go` 全文：

```go
package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Tencent/WeKnora/internal/types"
)

var (
	ErrAgentUpgradeProposalNotFound    = errors.New("agent upgrade proposal not found")
	// ErrAgentUpgradeProposalTransition marks a TransitionProposal whose
	// guarded state UPDATE missed (already resolved, or a concurrent
	// resolution landed first): the move fails loudly instead of
	// double-applying. Mirrors ErrAgentAdoptionVariantTransition.
	ErrAgentUpgradeProposalTransition = errors.New("agent upgrade proposal state transition failed")
)

// AgentUpgradeRepository owns the upgrade-proposal rows. It embeds
// AgentAdoptionRepository so the service reads listings/releases/adoption/
// variant/mapping data through ONE seam that already carries the #60
// introduction-ledger fallbacks (introducedListing/introducedRelease); the
// proposal primitives below are plain tenant-scoped, parameter-bound SQL.
type AgentUpgradeRepository interface {
	AgentAdoptionRepository
	FindOrCreateProposal(context.Context, *types.AgentUpgradeProposalEntity) (*types.AgentUpgradeProposalEntity, bool, error)
	GetProposal(context.Context, uint64, string) (*types.AgentUpgradeProposalEntity, error)
	ListProposals(context.Context, uint64) ([]types.AgentUpgradeProposalEntity, error)
	TransitionProposal(context.Context, uint64, string, []string, string, map[string]any) (*types.AgentUpgradeProposalEntity, error)
}

type agentUpgradeRepository struct {
	AgentAdoptionRepository
	db *gorm.DB
}

func NewAgentUpgradeRepository(db *gorm.DB) AgentUpgradeRepository {
	return &agentUpgradeRepository{AgentAdoptionRepository: NewAgentAdoptionRepository(db), db: db}
}

// FindOrCreateProposal inserts the (tenant, adoption, to_release)-unique
// proposal or returns the existing row — the same race-convergent shape as
// adoptListingTx: the insert losing to uq_agent_upgrade_proposals_scope
// re-reads the winner so both callers get an idempotent result.
func (r *agentUpgradeRepository) FindOrCreateProposal(ctx context.Context, proposal *types.AgentUpgradeProposalEntity) (*types.AgentUpgradeProposalEntity, bool, error) {
	key := func() (*types.AgentUpgradeProposalEntity, error) {
		var existing types.AgentUpgradeProposalEntity
		err := r.db.WithContext(ctx).Where(
			"tenant_id = ? AND adoption_id = ? AND to_release_id = ?",
			proposal.TenantID, proposal.AdoptionID, proposal.ToReleaseID,
		).First(&existing).Error
		if err != nil {
			return nil, err
		}
		return &existing, nil
	}
	if existing, err := key(); err == nil {
		return existing, false, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	created := *proposal
	created.ID = uuid.NewString()
	created.CreatedAt = time.Now().UTC()
	created.UpdatedAt = created.CreatedAt
	if created.State == "" {
		created.State = "open"
	}
	inserted := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&created)
	if inserted.Error != nil {
		return nil, false, inserted.Error
	}
	if inserted.RowsAffected == 1 {
		return &created, true, nil
	}
	// 输给了 uq_agent_upgrade_proposals_scope：重读赢家行，与顺序重放同收敛。
	winner, err := key()
	if err != nil {
		return nil, false, err
	}
	return winner, false, nil
}

func (r *agentUpgradeRepository) GetProposal(ctx context.Context, tenantID uint64, proposalID string) (*types.AgentUpgradeProposalEntity, error) {
	var row types.AgentUpgradeProposalEntity
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(proposalID)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *agentUpgradeRepository) ListProposals(ctx context.Context, tenantID uint64) ([]types.AgentUpgradeProposalEntity, error) {
	rows := []types.AgentUpgradeProposalEntity{}
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at ASC, id ASC").Find(&rows).Error
	return rows, err
}

// TransitionProposal is the compare-and-set state move: the update applies
// only when the current state is one of expectedFrom, mirroring
// UpdateVariantState. Paired stamps: naming an actor (resolved_by) also
// stamps updated_at.
func (r *agentUpgradeRepository) TransitionProposal(ctx context.Context, tenantID uint64, proposalID string, expectedFrom []string, nextState string, updates map[string]any) (*types.AgentUpgradeProposalEntity, error) {
	set := map[string]any{"state": nextState, "updated_at": time.Now().UTC()}
	for key, value := range updates {
		set[key] = value
	}
	result := r.db.WithContext(ctx).Model(&types.AgentUpgradeProposalEntity{}).
		Where("tenant_id = ? AND id = ? AND state IN ?", tenantID, strings.TrimSpace(proposalID), expectedFrom).
		Updates(set)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		var current types.AgentUpgradeProposalEntity
		err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(proposalID)).First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentUpgradeProposalNotFound
		}
		if err != nil {
			return nil, err
		}
		return nil, ErrAgentUpgradeProposalTransition
	}
	return r.GetProposal(ctx, tenantID, proposalID)
}
```

- [ ] **Step 4: 运行测试确认通过**

```sh
go test ./internal/application/repository/ -run 'TestAgentUpgradeRepository' -count=1
```

Expected: PASS（3 个测试）。

- [ ] **Step 5: Commit**

```bash
git add internal/application/repository/agent_upgrade.go internal/application/repository/agent_upgrade_test.go
git commit -m "feat(marketplace): upgrade proposal repository with race-convergent find-or-create and CAS (T31 #61 task 2)"
```

---

### Task 3: 四维差异纯函数

**Files:**
- Create: `internal/application/service/agent_upgrade_diff.go`
- Test: `internal/application/service/agent_upgrade_diff_test.go`

**Interfaces:**
- Consumes: Task 1 的 `types.AgentUpgradeDiff` 及四类变更结构；#58 的 `types.AgentReleaseEntity`（`ManifestJSON`/`DependencyLockJSON`/`Bundle` 列，`internal/types/agent_marketplace_persistence.go:50-67`）、`types.AgentReleaseManifest`/`AgentReleasePayload`/`DependencyLock`（`internal/types/agent_marketplace.go`）。
- Produces（Task 4 逐字使用）:

```go
func diffUpgradeBundles(from, to *types.AgentReleaseEntity) (types.AgentUpgradeDiff, error)
var ErrAgentUpgradeInvalidInput = errors.New("invalid agent upgrade proposal request") // 定义在 Task 4 的 agent_upgrade.go，本任务测试以 errors.Is 断言
```

> 下层证据标注：纯函数级测试（真实结构体 + 被测函数本体，无 mock），承载 AC2「四维全覆盖」的精确断言；e2e（Task 6）因生产依赖解析器 `tenantReleaseDependencyResolver` fail-closed（`internal/container/container.go:120-137`——生产路径无法产生非空 DependencyLock）只能断言依赖段存在为空，四维全量覆盖由本任务承载，见「差异记录」第 3 条。

- [ ] **Step 1: 写失败测试（RED）**

`internal/application/service/agent_upgrade_diff_test.go` 全文：

```go
package service

// Upgrade-diff pure-function tests (T31 #61, AC2): the four review
// dimensions over REAL AgentReleaseEntity rows (real manifests, real
// dependency locks, real bundle envelopes — the same shapes the exporter
// produces). No DB, no mocks of the function under test.

import (
	"encoding/json"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

const (
	diffManifestV1 = `{"semantic_version":"1.0.0","display_name":"Helper","summary":"Portable helper","supported_languages":["en"],"use_cases":["support"],"capability_requirements":["model","knowledge"],"minimum_weknora_capability":"1","license_id":"MIT","source":{"agent_version_id":"version-a","version_number":1,"source_sha256":"sha"}}`
	diffManifestV2 = `{"semantic_version":"1.1.0","display_name":"Helper","summary":"Portable helper","supported_languages":["en"],"use_cases":["support"],"capability_requirements":["knowledge","model","sandbox"],"data_categories":["chat_content"],"external_side_effects":["web_search"],"minimum_weknora_capability":"1","license_id":"Apache-2.0","source":{"agent_version_id":"version-b","version_number":2,"source_sha256":"sha"}}`
	diffLockV1     = `{"dependencies":[{"type":"skill","id":"calendar","version":"2.0.0","digest":"bb","license_id":"Apache-2.0"},{"type":"skill","id":"weather","version":"1.0.0","digest":"aa","license_id":"MIT"}]}`
	diffLockV2     = `{"dependencies":[{"type":"skill","id":"calendar","version":"2.1.0","digest":"cc","license_id":"Apache-2.0"},{"type":"skill","id":"translate","version":"1.0.0","digest":"dd","license_id":"MIT"}]}`
	diffBundleV1   = `{"payload":{"agent_mode":"smart-reasoning","system_prompt":"Be useful.","allowed_tools":["search"],"starter_prompts":["Help me"]},"manifest":` + diffManifestV1 + `,"dependency_lock":` + diffLockV1 + `}`
	diffBundleV2   = `{"payload":{"agent_mode":"smart-reasoning","system_prompt":"Be extra useful.","allowed_tools":["search","mail"],"starter_prompts":["Help me now"]},"manifest":` + diffManifestV2 + `,"dependency_lock":` + diffLockV2 + `}`
)

func diffRelease(id, manifest, lock, bundle string) *types.AgentReleaseEntity {
	return &types.AgentReleaseEntity{ID: id, ManifestJSON: manifest, DependencyLockJSON: lock, Bundle: []byte(bundle)}
}

func TestDiffUpgradeBundlesCoversAllFourDimensions(t *testing.T) {
	diff, err := diffUpgradeBundles(
		diffRelease("r1", diffManifestV1, diffLockV1, diffBundleV1),
		diffRelease("r2", diffManifestV2, diffLockV2, diffBundleV2),
	)
	require.NoError(t, err)

	// 行为维：提示词、工具表、开场提示变化；agent_mode/persona 不变 → 不出现。
	require.Equal(t, []types.UpgradeFieldChange{
		{Field: "system_prompt", From: "Be useful.", To: "Be extra useful."},
		{Field: "allowed_tools", From: "search", To: "mail,search"},
		{Field: "starter_prompts", From: "Help me", To: "Help me now"},
	}, diff.Behavior)

	// 依赖维：按 (type, id) 对齐且键序确定——calendar 版本+摘要变化，translate 新增，weather 移除。
	require.Equal(t, []types.UpgradeDependencyChange{
		{Type: "skill", ID: "calendar", Change: "version_changed", FromVersion: "2.0.0", ToVersion: "2.1.0"},
		{Type: "skill", ID: "calendar", Change: "digest_changed", FromVersion: "2.0.0", ToVersion: "2.1.0"},
		{Type: "skill", ID: "translate", Change: "added", ToVersion: "1.0.0"},
		{Type: "skill", ID: "weather", Change: "removed", FromVersion: "1.0.0"},
	}, diff.Dependencies)

	// 安全维：能力需求（排序拼接）、数据类别、外部副作用三项变化。
	require.Equal(t, []types.UpgradeFieldChange{
		{Field: "capability_requirements", From: "knowledge,model", To: "knowledge,model,sandbox"},
		{Field: "data_categories", From: "", To: "chat_content"},
		{Field: "external_side_effects", From: "", To: "web_search"},
	}, diff.Security)

	// 许可维：依赖许可变化按依赖键序在前（calendar 许可未变 → 不产生条目；
	// translate 新增 → 空 From；weather 移除 → 空 To），Release 许可收尾。
	require.Equal(t, []types.UpgradeLicenseChange{
		{Scope: "dependency", ID: "translate", From: "", To: "MIT"},
		{Scope: "dependency", ID: "weather", From: "MIT", To: ""},
		{Scope: "release", ID: "license_id", From: "MIT", To: "Apache-2.0"},
	}, diff.License)
}

func TestDiffUpgradeBundlesIdenticalReleasesProduceEmptySections(t *testing.T) {
	diff, err := diffUpgradeBundles(
		diffRelease("r1", diffManifestV1, diffLockV1, diffBundleV1),
		diffRelease("r1-copy", diffManifestV1, diffLockV1, diffBundleV1),
	)
	require.NoError(t, err)
	require.NotNil(t, diff.Behavior)
	require.Len(t, diff.Behavior, 0)
	require.NotNil(t, diff.Dependencies)
	require.Len(t, diff.Dependencies, 0)
	require.NotNil(t, diff.Security)
	require.Len(t, diff.Security, 0)
	require.NotNil(t, diff.License)
	require.Len(t, diff.License, 0)
}

func TestDiffUpgradeBundlesRejectsMissingReleasesAndMalformedPayload(t *testing.T) {
	_, err := diffUpgradeBundles(nil, diffRelease("r2", diffManifestV2, diffLockV2, diffBundleV2))
	require.ErrorIs(t, err, ErrAgentUpgradeInvalidInput)

	_, err = diffUpgradeBundles(diffRelease("r1", diffManifestV1, diffLockV1, diffBundleV1), nil)
	require.ErrorIs(t, err, ErrAgentUpgradeInvalidInput)

	// bundle JSON 畸形：fail closed，绝不产出半成品差异。
	_, err = diffUpgradeBundles(
		diffRelease("r1", diffManifestV1, diffLockV1, `{"payload":"not-an-object"`),
		diffRelease("r2", diffManifestV2, diffLockV2, diffBundleV2),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "r1")

	// manifest JSON 畸形：同样 fail closed。
	_, err = diffUpgradeBundles(
		diffRelease("r1", `{broken`, diffLockV1, diffBundleV1),
		diffRelease("r2", diffManifestV2, diffLockV2, diffBundleV2),
	)
	require.Error(t, err)

	// 依赖锁 JSON 畸形：同样 fail closed。
	_, err = diffUpgradeBundles(
		diffRelease("r1", diffManifestV1, `{broken`, diffBundleV1),
		diffRelease("r2", diffManifestV2, diffLockV2, diffBundleV2),
	)
	require.Error(t, err)
}

func TestDiffUpgradeBundlesIsDeterministic(t *testing.T) {
	run := func() types.AgentUpgradeDiff {
		diff, err := diffUpgradeBundles(
			diffRelease("r1", diffManifestV1, diffLockV1, diffBundleV1),
			diffRelease("r2", diffManifestV2, diffLockV2, diffBundleV2),
		)
		require.NoError(t, err)
		return diff
	}
	first, second := run(), run()
	firstRaw, err := json.Marshal(first)
	require.NoError(t, err)
	secondRaw, err := json.Marshal(second)
	require.NoError(t, err)
	require.JSONEq(t, string(firstRaw), string(secondRaw), "同一对 Release 永远产出相同差异")
}
```

- [ ] **Step 2: 运行确认失败**

```sh
go test ./internal/application/service/ -run TestDiffUpgradeBundles -count=1
```

Expected: FAIL（编译错误 `undefined: diffUpgradeBundles`；`ErrAgentUpgradeInvalidInput` 亦未定义）。

- [ ] **Step 3: 写最小实现（GREEN）**

`internal/application/service/agent_upgrade_diff.go` 全文：

```go
package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
)

// diffUpgradeBundles computes the four review dimensions (behavior /
// dependencies / security / license — Issue #61 AC2) between the currently
// accepted Release and the proposed Release. Deterministic: fixed field
// order, sorted list joins, dependency keys sorted; identical releases
// produce byte-identical (all-empty) diffs. Malformed stored JSON fails
// closed with an error — the caller must not materialize a half-built
// proposal.
func diffUpgradeBundles(from, to *types.AgentReleaseEntity) (types.AgentUpgradeDiff, error) {
	diff := types.AgentUpgradeDiff{
		Behavior:     []types.UpgradeFieldChange{},
		Dependencies: []types.UpgradeDependencyChange{},
		Security:     []types.UpgradeFieldChange{},
		License:      []types.UpgradeLicenseChange{},
	}
	if from == nil || to == nil {
		return diff, ErrAgentUpgradeInvalidInput
	}
	fromManifest, fromPayload, fromLock, err := decodeUpgradeBundleParts(from)
	if err != nil {
		return diff, err
	}
	toManifest, toPayload, toLock, err := decodeUpgradeBundleParts(to)
	if err != nil {
		return diff, err
	}

	// 行为维：便携 payload 字段级差异（列表字段排序拼接后比较）。
	appendFieldChange(&diff.Behavior, "agent_mode", fromPayload.AgentMode, toPayload.AgentMode)
	appendFieldChange(&diff.Behavior, "system_prompt", fromPayload.SystemPrompt, toPayload.SystemPrompt)
	appendFieldChange(&diff.Behavior, "persona_mbti", fromPayload.PersonaMBTI, toPayload.PersonaMBTI)
	appendFieldChange(&diff.Behavior, "persona_style", fromPayload.PersonaStyle, toPayload.PersonaStyle)
	appendListChange(&diff.Behavior, "allowed_tools", fromPayload.AllowedTools, toPayload.AllowedTools)
	appendListChange(&diff.Behavior, "skills", fromPayload.Skills, toPayload.Skills)
	appendListChange(&diff.Behavior, "subagents", fromPayload.Subagents, toPayload.Subagents)
	appendListChange(&diff.Behavior, "starter_prompts", fromPayload.StarterPrompts, toPayload.StarterPrompts)

	// 安全维：Manifest 声明面的扩大（数据类别 / 外部副作用 / 能力需求）。
	appendListChange(&diff.Security, "capability_requirements", fromManifest.CapabilityRequirements, toManifest.CapabilityRequirements)
	appendListChange(&diff.Security, "data_categories", fromManifest.DataCategories, toManifest.DataCategories)
	appendListChange(&diff.Security, "external_side_effects", fromManifest.ExternalSideEffects, toManifest.ExternalSideEffects)

	// 依赖维：DependencyLock 按 (type, id) 对齐；键序确定。
	fromDeps := make(map[string]types.AgentReleaseDependency, len(fromLock.Dependencies))
	toDeps := make(map[string]types.AgentReleaseDependency, len(toLock.Dependencies))
	keys := make([]string, 0, len(fromLock.Dependencies)+len(toLock.Dependencies))
	for _, dep := range fromLock.Dependencies {
		key := dep.Type + "\x00" + dep.ID
		fromDeps[key] = dep
		keys = append(keys, key)
	}
	for _, dep := range toLock.Dependencies {
		key := dep.Type + "\x00" + dep.ID
		toDeps[key] = dep
		if _, ok := fromDeps[key]; !ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		fromDep, hadFrom := fromDeps[key]
		toDep, hasTo := toDeps[key]
		switch {
		case !hadFrom:
			diff.Dependencies = append(diff.Dependencies, types.UpgradeDependencyChange{
				Type: toDep.Type, ID: toDep.ID, Change: "added", ToVersion: toDep.Version,
			})
			diff.License = append(diff.License, types.UpgradeLicenseChange{
				Scope: "dependency", ID: toDep.ID, From: "", To: toDep.LicenseID,
			})
		case !hasTo:
			diff.Dependencies = append(diff.Dependencies, types.UpgradeDependencyChange{
				Type: fromDep.Type, ID: fromDep.ID, Change: "removed", FromVersion: fromDep.Version,
			})
			diff.License = append(diff.License, types.UpgradeLicenseChange{
				Scope: "dependency", ID: fromDep.ID, From: fromDep.LicenseID, To: "",
			})
		default:
			if fromDep.Version != toDep.Version {
				diff.Dependencies = append(diff.Dependencies, types.UpgradeDependencyChange{
					Type: fromDep.Type, ID: fromDep.ID, Change: "version_changed",
					FromVersion: fromDep.Version, ToVersion: toDep.Version,
				})
			}
			if fromDep.Digest != toDep.Digest {
				diff.Dependencies = append(diff.Dependencies, types.UpgradeDependencyChange{
					Type: fromDep.Type, ID: fromDep.ID, Change: "digest_changed",
					FromVersion: fromDep.Version, ToVersion: toDep.Version,
				})
			}
			if fromDep.LicenseID != toDep.LicenseID {
				diff.License = append(diff.License, types.UpgradeLicenseChange{
					Scope: "dependency", ID: fromDep.ID, From: fromDep.LicenseID, To: toDep.LicenseID,
				})
			}
		}
	}

	// 许可维收尾：Release 自身许可（依赖许可已按键序并入其前）。
	if fromManifest.LicenseID != toManifest.LicenseID {
		diff.License = append(diff.License, types.UpgradeLicenseChange{
			Scope: "release", ID: "license_id", From: fromManifest.LicenseID, To: toManifest.LicenseID,
		})
	}
	return diff, nil
}

// decodeUpgradeBundleParts decodes the three stored projections of one
// immutable release: ManifestJSON → AgentReleaseManifest, Bundle → payload
// envelope, DependencyLockJSON → DependencyLock.
func decodeUpgradeBundleParts(release *types.AgentReleaseEntity) (types.AgentReleaseManifest, types.AgentReleasePayload, types.DependencyLock, error) {
	var manifest types.AgentReleaseManifest
	if err := json.Unmarshal([]byte(release.ManifestJSON), &manifest); err != nil {
		return manifest, types.AgentReleasePayload{}, types.DependencyLock{}, fmt.Errorf("decode release %s manifest: %w", release.ID, err)
	}
	var envelope struct {
		Payload types.AgentReleasePayload `json:"payload"`
	}
	if err := json.Unmarshal(release.Bundle, &envelope); err != nil {
		return manifest, types.AgentReleasePayload{}, types.DependencyLock{}, fmt.Errorf("decode release %s bundle payload: %w", release.ID, err)
	}
	var lock types.DependencyLock
	if err := json.Unmarshal([]byte(release.DependencyLockJSON), &lock); err != nil {
		return manifest, types.AgentReleasePayload{}, types.DependencyLock{}, fmt.Errorf("decode release %s dependency lock: %w", release.ID, err)
	}
	return manifest, envelope.Payload, lock, nil
}

func sortedJoin(values []string) string {
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return strings.Join(sorted, ",")
}

func appendFieldChange(changes *[]types.UpgradeFieldChange, field, from, to string) {
	if from == to {
		return
	}
	*changes = append(*changes, types.UpgradeFieldChange{Field: field, From: from, To: to})
}

func appendListChange(changes *[]types.UpgradeFieldChange, field string, from, to []string) {
	appendFieldChange(changes, field, sortedJoin(from), sortedJoin(to))
}
```

- [ ] **Step 4: 运行测试确认通过**

```sh
go test ./internal/application/service/ -run TestDiffUpgradeBundles -count=1
```

Expected: FAIL（编译错误 `undefined: ErrAgentUpgradeInvalidInput` —— 该哨兵在 Task 4 的 `agent_upgrade.go` 定义；本步先在 `agent_upgrade_diff.go` 顶部临时声明 `var ErrAgentUpgradeInvalidInput = errors.New("invalid agent upgrade proposal request")` 并 `import "errors"`，Task 4 落盘时移入 `agent_upgrade.go` 的哨兵组并从此处删除）。

Expected（补哨兵后重跑）: PASS（4 个测试）。

- [ ] **Step 5: Commit**

```bash
git add internal/application/service/agent_upgrade_diff.go internal/application/service/agent_upgrade_diff_test.go
git commit -m "feat(marketplace): four-dimension upgrade diff over immutable releases (T31 #61 task 3)"
```

---

### Task 4: Upgrade 服务（读取时对账 + 接受/驳回）

**Files:**
- Create: `internal/types/interfaces/agent_upgrade.go`
- Create: `internal/application/service/agent_upgrade.go`
- Test: `internal/application/service/agent_upgrade_test.go`
- Modify: `internal/application/service/agent_upgrade_diff.go`（删除 Task 3 步骤 4 的临时哨兵，改由本任务文件提供）

**Interfaces:**
- Consumes: Task 2 `AgentUpgradeRepository`/`ErrAgentUpgradeProposalTransition`/`ErrAgentUpgradeProposalNotFound`；Task 3 `diffUpgradeBundles`；同包既有符号 `AgentAdoptionStateActive`、`AgentVariantStateDraft`（agent_adoption.go:31-39）、`releaseManifest(ctx, repo, tenantID, releaseID)`（agent_adoption.go:414-427，参数类型 `adoptionReleaseReader` 只要 `GetRelease`——嵌入接口满足）、`missingCapabilities(requirements, mappings)`（agent_adoption.go:432-447）、`openAgentVersionServiceTestDB`（agent_version_test.go:31）。
- Produces（Task 5/6 逐字使用）:

```go
// internal/types/interfaces/agent_upgrade.go
type AgentUpgradeService interface {
	ListUpgradeProposals(ctx context.Context, tenantID uint64) ([]UpgradeProposalView, error)
	GetUpgradeProposal(ctx context.Context, tenantID uint64, proposalID string) (UpgradeProposalView, error)
	AcceptUpgradeProposal(ctx context.Context, tenantID uint64, actorID, proposalID string, input UpgradeVariantInput) (AdoptionVariantView, UpgradeProposalView, error)
	DismissUpgradeProposal(ctx context.Context, tenantID uint64, actorID, proposalID string) (UpgradeProposalView, error)
}
type UpgradeVariantInput struct{ Name string }
type UpgradeProposalView struct {
	types.AgentUpgradeProposalEntity
	Diff types.AgentUpgradeDiff `json:"diff"`
}

// internal/application/service/agent_upgrade.go
func NewAgentUpgradeService(repo repository.AgentUpgradeRepository) *AgentUpgradeService
var (
	ErrAgentUpgradeInvalidInput  = errors.New("invalid agent upgrade proposal request")
	ErrAgentUpgradeNotFound      = errors.New("agent upgrade proposal not found")
	ErrAgentUpgradeStateConflict = errors.New("agent upgrade proposal state conflict")
)
const (
	AgentUpgradeProposalStateOpen      = "open"
	AgentUpgradeProposalStateAccepted  = "accepted"
	AgentUpgradeProposalStateDismissed = "dismissed"
)
```

> 下层证据标注：服务级测试跑真实仓储 + 真实迁移流 DB（复用 `openAgentVersionServiceTestDB`），承载 reconcile 语义与 #60 引入台账回退；仍属 Task 6 之下的证据层。

- [ ] **Step 1: 写失败测试（RED）**

`internal/application/service/agent_upgrade_test.go` 全文：

```go
package service

// Agent-upgrade service tests (T31 #61): reconcile-on-read materialization
// over the REAL repository (real migration-stream DB via
// openAgentVersionServiceTestDB), acceptance/dismissal state machine, and
// the #60 introduced-ledger fallback. Lower-interface evidence below the
// Task 6 HTTP e2e — labeled as such, not passed off as AC3.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	upgradeManifestV1 = `{"semantic_version":"1.0.0","display_name":"Helper","summary":"Portable helper","supported_languages":["en"],"use_cases":["support"],"capability_requirements":["model","knowledge"],"minimum_weknora_capability":"1","license_id":"MIT","source":{"agent_version_id":"version-1.0.0","version_number":1,"source_sha256":"sha"}}`
	upgradeManifestV2 = `{"semantic_version":"1.1.0","display_name":"Helper","summary":"Portable helper","supported_languages":["en"],"use_cases":["support"],"capability_requirements":["model","knowledge","sandbox"],"data_categories":["chat_content"],"external_side_effects":["web_search"],"minimum_weknora_capability":"1","license_id":"Apache-2.0","source":{"agent_version_id":"version-1.1.0","version_number":2,"source_sha256":"sha"}}`
	upgradeManifestV3 = `{"semantic_version":"1.2.0","display_name":"Helper","summary":"Portable helper","supported_languages":["en"],"use_cases":["support"],"capability_requirements":["model","knowledge"],"minimum_weknora_capability":"1","license_id":"MIT","source":{"agent_version_id":"version-1.2.0","version_number":3,"source_sha256":"sha"}}`
	upgradeLockV1     = `{"dependencies":[]}`
	upgradeLockV2     = `{"dependencies":[{"type":"skill","id":"weather","version":"1.0.0","digest":"aa","license_id":"MIT"}]}`
	upgradeBundleV1   = `{"payload":{"agent_mode":"smart-reasoning","system_prompt":"Be useful.","allowed_tools":["search"]},"manifest":` + upgradeManifestV1 + `,"dependency_lock":` + upgradeLockV1 + `}`
	upgradeBundleV2   = `{"payload":{"agent_mode":"smart-reasoning","system_prompt":"Be extra useful.","allowed_tools":["search","mail"],"skills":["weather"]},"manifest":` + upgradeManifestV2 + `,"dependency_lock":` + upgradeLockV2 + `}`
	upgradeBundleV3   = `{"payload":{"agent_mode":"smart-reasoning","system_prompt":"Be useful v3."},"manifest":` + upgradeManifestV3 + `,"dependency_lock":{"dependencies":[]}}`
)

func newAgentUpgradeServiceForTest(t *testing.T) (*AgentUpgradeService, *gorm.DB) {
	t.Helper()
	db := openAgentVersionServiceTestDB(t)
	return NewAgentUpgradeService(repository.NewAgentUpgradeRepository(db)), db
}

// publishUpgradeServiceRelease publishes ONE release on the agent-a listing
// through the real repository path (CreateSubmission + ReviewAndPublishTx —
// the same shape seedAdoptionServiceRelease uses). The listing is
// find-or-created by SourceAgentID; expectedPriorReleaseID is read from the
// listing row, mirroring the service's ReviewSubmission. Returns listing id
// and release id.
func publishUpgradeServiceRelease(t *testing.T, db *gorm.DB, versionNumber int, semanticVersion, manifest, lock, bundle string) (listingID, releaseID string) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, db.Exec(
		`INSERT INTO agent_versions (id, tenant_id, agent_id, version_number, snapshot, source_sha256, frozen_by) VALUES (?, 1, 'agent-a', ?, '{}', 'sha', 'author')`,
		"version-"+semanticVersion, versionNumber).Error)
	raw := []byte(bundle)
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])

	priorRelease := ""
	var prior types.AgentMarketplaceListingEntity
	err := db.Where("tenant_id = ? AND source_agent_id = ?", uint64(1), "agent-a").First(&prior).Error
	if err == nil && prior.CurrentReleaseID != nil {
		priorRelease = *prior.CurrentReleaseID
	} else if err != nil {
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	}

	repo := repository.NewAgentMarketplaceRepository(db)
	sub, err := repo.CreateSubmission(ctx,
		&types.AgentMarketplaceListingEntity{TenantID: 1, SourceAgentID: "agent-a", DisplayName: "Helper", Summary: "s", State: "listed"},
		&types.AgentReleaseSubmissionEntity{
			TenantID: 1, AgentVersionID: "version-" + semanticVersion, SourceAgentID: "agent-a", AuthorID: "author",
			SemanticVersion: semanticVersion, BundleDigest: digest, ManifestJSON: manifest,
			DependencyLockJSON: lock, Bundle: raw, Status: "submitted",
		})
	require.NoError(t, err)
	_, rel, err := repo.ReviewAndPublishTx(ctx, 1, priorRelease, sub.ID, digest, types.AgentReleaseReviewDecision{ReviewerID: "reviewer", Decision: "approved"})
	require.NoError(t, err)
	require.NotNil(t, rel)
	return sub.ListingID, rel.ID
}

func adoptUpgradeRelease(t *testing.T, db *gorm.DB, listingID, releaseID string) *types.AgentAdoptionEntity {
	t.Helper()
	repo := repository.NewAgentAdoptionRepository(db)
	adoption, _, err := repo.AdoptListing(context.Background(), &types.AgentAdoptionEntity{
		TenantID: 1, ListingID: listingID, AcceptedReleaseID: releaseID, State: "active", CreatedBy: "admin",
	})
	require.NoError(t, err)
	return adoption
}

func TestAgentUpgradeServiceMaterializesFourDimensionDiff(t *testing.T) {
	svc, db := newAgentUpgradeServiceForTest(t)
	listingID, v1 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", upgradeManifestV1, upgradeLockV1, upgradeBundleV1)
	adoptUpgradeRelease(t, db, listingID, v1)
	ctx := context.Background()

	// Listing 指针仍在 v1、Adoption 已接受 v1：无升级建议。
	before, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, before)

	// v2 发布（ReviewAndPublishTx 前移指针）：读取时对账物化建议。
	_, v2 := publishUpgradeServiceRelease(t, db, 2, "1.1.0", upgradeManifestV2, upgradeLockV2, upgradeBundleV2)
	proposals, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, proposals, 1)
	p := proposals[0]
	require.Equal(t, listingID, p.ListingID)
	require.Equal(t, v1, p.FromReleaseID)
	require.Equal(t, v2, p.ToReleaseID)
	require.Equal(t, "1.1.0", p.ToSemanticVersion)
	require.Equal(t, AgentUpgradeProposalStateOpen, p.State)

	// 四维差异（AC2）：
	require.Equal(t, []types.UpgradeFieldChange{
		{Field: "system_prompt", From: "Be useful.", To: "Be extra useful."},
		{Field: "allowed_tools", From: "search", To: "mail,search"},
		{Field: "skills", From: "", To: "weather"},
	}, p.Diff.Behavior)
	require.Equal(t, []types.UpgradeDependencyChange{
		{Type: "skill", ID: "weather", Change: "added", ToVersion: "1.0.0"},
	}, p.Diff.Dependencies)
	require.Equal(t, []types.UpgradeFieldChange{
		{Field: "capability_requirements", From: "knowledge,model", To: "knowledge,model,sandbox"},
		{Field: "data_categories", From: "", To: "chat_content"},
		{Field: "external_side_effects", From: "", To: "web_search"},
	}, p.Diff.Security)
	require.Equal(t, []types.UpgradeLicenseChange{
		{Scope: "dependency", ID: "weather", From: "", To: "MIT"},
		{Scope: "release", ID: "license_id", From: "MIT", To: "Apache-2.0"},
	}, p.Diff.License)

	// 对账幂等：再列一次仍恰好一条（唯一索引收敛，无重复行）。
	again, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, again, 1)
	require.Equal(t, p.ID, again[0].ID)

	// 单条读取与列表一致。
	single, err := svc.GetUpgradeProposal(ctx, 1, p.ID)
	require.NoError(t, err)
	require.Equal(t, p.ID, single.ID)
	require.Equal(t, p.Diff, single.Diff)
}

func TestAgentUpgradeServiceResolvesAndRefusesOutOfStateOperations(t *testing.T) {
	svc, db := newAgentUpgradeServiceForTest(t)
	listingID, v1 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", upgradeManifestV1, upgradeLockV1, upgradeBundleV1)
	adoption := adoptUpgradeRelease(t, db, listingID, v1)
	_, v2 := publishUpgradeServiceRelease(t, db, 2, "1.1.0", upgradeManifestV2, upgradeLockV2, upgradeBundleV2)
	ctx := context.Background()

	proposals, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, proposals, 1)
	proposalID := proposals[0].ID

	// 非法输入：空名字拒绝。
	_, _, err = svc.AcceptUpgradeProposal(ctx, 1, "admin", proposalID, interfaces.UpgradeVariantInput{Name: "  "})
	require.ErrorIs(t, err, ErrAgentUpgradeInvalidInput)
	// 未知 proposal：not found。
	_, _, err = svc.AcceptUpgradeProposal(ctx, 1, "admin", "missing-proposal", interfaces.UpgradeVariantInput{Name: "X"})
	require.ErrorIs(t, err, ErrAgentUpgradeNotFound)
	_, err = svc.DismissUpgradeProposal(ctx, 1, "admin", "missing-proposal")
	require.ErrorIs(t, err, ErrAgentUpgradeNotFound)

	// 接受：只创建固定到 to_release 的草稿 Variant，proposal 落 accepted 终态。
	variant, accepted, err := svc.AcceptUpgradeProposal(ctx, 1, "admin", proposalID, interfaces.UpgradeVariantInput{Name: "Sales v1.1"})
	require.NoError(t, err)
	require.Equal(t, AgentVariantStateDraft, variant.State)
	require.Equal(t, v2, variant.ReleaseID)
	require.Equal(t, adoption.ID, variant.AdoptionID)
	require.Equal(t, AgentUpgradeProposalStateAccepted, accepted.State)
	require.Equal(t, variant.ID, accepted.AcceptedVariantID)
	require.Equal(t, "admin", accepted.ResolvedBy)
	// 新 Release 要求三项能力而新草稿尚无映射：missing 逐项点名（复用 #59 裁决）。
	require.Equal(t, []string{"knowledge", "model", "sandbox"}, variant.MissingCapabilities)

	// 终态互斥：重复 accept / dismiss 已 accepted，一律显式冲突。
	_, _, err = svc.AcceptUpgradeProposal(ctx, 1, "admin", proposalID, interfaces.UpgradeVariantInput{Name: "Again"})
	require.ErrorIs(t, err, ErrAgentUpgradeStateConflict)
	_, err = svc.DismissUpgradeProposal(ctx, 1, "admin", proposalID)
	require.ErrorIs(t, err, ErrAgentUpgradeStateConflict)

	// 已 resolved 的建议不复活：再列仍恰好一条，state=accepted。
	after, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, after, 1)
	require.Equal(t, AgentUpgradeProposalStateAccepted, after[0].State)

	// dismiss 路径：新 Release v3 → 新 open 建议 → dismiss → 终态保留。
	_, v3 := publishUpgradeServiceRelease(t, db, 3, "1.2.0", upgradeManifestV3, `{"dependencies":[]}`, upgradeBundleV3)
	reopened, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, reopened, 2)
	var openID string
	for _, row := range reopened {
		if row.State == AgentUpgradeProposalStateOpen {
			openID = row.ID
			require.Equal(t, v3, row.ToReleaseID)
		}
	}
	require.NotEmpty(t, openID)
	dismissed, err := svc.DismissUpgradeProposal(ctx, 1, "admin", openID)
	require.NoError(t, err)
	require.Equal(t, AgentUpgradeProposalStateDismissed, dismissed.State)
	require.Equal(t, "admin", dismissed.ResolvedBy)
	still, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, still, 2, "dismissed 行保留且不复活")
	for _, row := range still {
		require.NotEqual(t, AgentUpgradeProposalStateOpen, row.State)
	}
	_, err = svc.DismissUpgradeProposal(ctx, 1, "admin", openID)
	require.ErrorIs(t, err, ErrAgentUpgradeStateConflict)
}

func TestAgentUpgradeServiceCoversIntroducedLedgerUpgrades(t *testing.T) {
	svc, db := newAgentUpgradeServiceForTest(t)
	ctx := context.Background()

	// #60 引入台账：同一 public listing 的两个引入 Release（旧→新），
	// Adoption 停留在旧引入版；listing 读取经 introducedListing 合成、
	// 两侧 Release 读取经 introducedRelease 合成。
	base := time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(&types.TenantIntroducedReleaseEntity{
		ID: "intro-rel-1", TenantID: 1, PublicListingID: "pub-listing-1", PublicReleaseID: "public-rel-1",
		DisplayName: "Helper", Summary: "s", SemanticVersion: "1.0.0",
		BundleDigest: "d1", ManifestJSON: upgradeManifestV1, DependencyLockJSON: upgradeLockV1,
		Bundle: []byte(upgradeBundleV1), IntroducedBy: "admin", IntroducedAt: base,
	}).Error)
	require.NoError(t, db.Create(&types.TenantIntroducedReleaseEntity{
		ID: "intro-rel-2", TenantID: 1, PublicListingID: "pub-listing-1", PublicReleaseID: "public-rel-2",
		DisplayName: "Helper", Summary: "s", SemanticVersion: "2.0.0",
		BundleDigest: "d2", ManifestJSON: upgradeManifestV2, DependencyLockJSON: upgradeLockV2,
		Bundle: []byte(upgradeBundleV2), IntroducedBy: "admin", IntroducedAt: base.Add(time.Hour),
	}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{
		TenantID: 1, ID: "adopt-intro", ListingID: "pub-listing-1",
		AcceptedReleaseID: "intro-rel-1", State: "active", CreatedBy: "admin",
	}).Error)

	proposals, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, proposals, 1)
	p := proposals[0]
	require.Equal(t, "pub-listing-1", p.ListingID)
	require.Equal(t, "intro-rel-1", p.FromReleaseID)
	require.Equal(t, "intro-rel-2", p.ToReleaseID, "listing 当前指针 = 最新引入行")
	require.Equal(t, "2.0.0", p.ToSemanticVersion)
	require.Len(t, p.Diff.Behavior, 3, "引入链与本地链共用同一差异计算")
}

func TestAgentUpgradeServiceSkipsInactiveAdoptionsAndCorruptReleases(t *testing.T) {
	svc, db := newAgentUpgradeServiceForTest(t)
	listingID, v1 := publishUpgradeServiceRelease(t, db, 1, "1.0.0", upgradeManifestV1, upgradeLockV1, upgradeBundleV1)
	adoption := adoptUpgradeRelease(t, db, listingID, v1)
	_, v2 := publishUpgradeServiceRelease(t, db, 2, "1.1.0", upgradeManifestV2, upgradeLockV2, upgradeBundleV2)
	ctx := context.Background()

	// 非 active Adoption（End Adoption 归 #63，此处只钉状态过滤）。
	require.NoError(t, db.Model(&types.AgentAdoptionEntity{}).
		Where("tenant_id = ? AND id = ?", uint64(1), adoption.ID).Update("state", "ended").Error)
	proposals, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, proposals, "非 active Adoption 不生成升级建议")

	// 损坏的 to-Release bundle：fail closed——跳过物化并记日志，列表端点不炸。
	require.NoError(t, db.Model(&types.AgentReleaseEntity{}).
		Where("tenant_id = ? AND id = ?", uint64(1), v2).Update("bundle", []byte(`{broken`)).Error)
	corrupt, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, corrupt)

	// 修复后（模拟恢复）建议照常出现。
	require.NoError(t, db.Model(&types.AgentReleaseEntity{}).
		Where("tenant_id = ? AND id = ?", uint64(1), v2).Update("bundle", []byte(upgradeBundleV2)).Error)
	healed, err := svc.ListUpgradeProposals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, healed, 1)
}
```

- [ ] **Step 2: 运行确认失败**

```sh
go test ./internal/application/service/ -run TestAgentUpgradeService -count=1
```

Expected: FAIL（编译错误 `undefined: NewAgentUpgradeService` / `interfaces.UpgradeVariantInput` / `interfaces.UpgradeProposalView` 未定义）。

- [ ] **Step 3: 写最小实现（GREEN）**

`internal/types/interfaces/agent_upgrade.go` 全文：

```go
package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// AgentUpgradeService is the reviewable upgrade-proposal surface (T31,
// Ticket #61; spec §9). A Listing advancing to a new Release only ever
// GENERATES a proposal; acceptance creates a new Variant draft pinned to
// the proposal's release and the draft then flows through the #59
// mapping/test/publish state machine unchanged. The previously accepted
// release, other Variants and existing Tasks never change (CONTEXT.md
// 「Agent 升级建议」). HTTP authorization stays at the route boundary;
// Tenant and actor identity always come from the authenticated request
// context.
type AgentUpgradeService interface {
	// ListUpgradeProposals reconciles open proposals for the tenant's active
	// adoptions against each listing's current release, then lists all
	// proposals (open + resolved) ordered created_at ASC.
	ListUpgradeProposals(ctx context.Context, tenantID uint64) ([]UpgradeProposalView, error)
	GetUpgradeProposal(ctx context.Context, tenantID uint64, proposalID string) (UpgradeProposalView, error)
	// AcceptUpgradeProposal creates the upgrade Variant draft (state draft,
	// Release = proposal.ToReleaseID) and CAS-marks the proposal accepted
	// with the draft's id.
	AcceptUpgradeProposal(ctx context.Context, tenantID uint64, actorID, proposalID string, input UpgradeVariantInput) (AdoptionVariantView, UpgradeProposalView, error)
	// DismissUpgradeProposal CAS-marks the proposal dismissed. Both states
	// are terminal: reconcile never resurrects a resolved proposal.
	DismissUpgradeProposal(ctx context.Context, tenantID uint64, actorID, proposalID string) (UpgradeProposalView, error)
}

// UpgradeVariantInput names the upgrade Variant draft to create on
// acceptance (the proposal pins the release; the admin names the draft).
type UpgradeVariantInput struct{ Name string }

// UpgradeProposalView is the reviewable wire projection: the persisted
// proposal row plus its decoded four-dimension diff.
type UpgradeProposalView struct {
	types.AgentUpgradeProposalEntity
	Diff types.AgentUpgradeDiff `json:"diff"`
}
```

`internal/application/service/agent_upgrade.go` 全文：

```go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

var (
	ErrAgentUpgradeInvalidInput  = errors.New("invalid agent upgrade proposal request")
	ErrAgentUpgradeNotFound      = errors.New("agent upgrade proposal not found")
	ErrAgentUpgradeStateConflict = errors.New("agent upgrade proposal state conflict")
)

// Proposal lifecycle states (CONTEXT.md「Agent 升级建议」). accepted 和
// dismissed 都是终态：materialize 对已存在行幂等，resolved 建议不会被
// reconcile 复活（End Adoption 的「禁止新升级建议」由 #63 在 adoption 状态
// 上落闸，本服务的对账只作用于 active Adoption）。
const (
	AgentUpgradeProposalStateOpen      = "open"
	AgentUpgradeProposalStateAccepted  = "accepted"
	AgentUpgradeProposalStateDismissed = "dismissed"
)

type AgentUpgradeService struct {
	repo repository.AgentUpgradeRepository
	now  func() time.Time
}

var _ interfaces.AgentUpgradeService = (*AgentUpgradeService)(nil)

func NewAgentUpgradeService(repo repository.AgentUpgradeRepository) *AgentUpgradeService {
	return &AgentUpgradeService{repo: repo, now: time.Now}
}

func (s *AgentUpgradeService) ListUpgradeProposals(ctx context.Context, tenantID uint64) ([]interfaces.UpgradeProposalView, error) {
	if tenantID == 0 {
		return nil, ErrAgentUpgradeInvalidInput
	}
	if err := s.reconcileProposals(ctx, tenantID); err != nil {
		return nil, err
	}
	rows, err := s.repo.ListProposals(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	views := make([]interfaces.UpgradeProposalView, 0, len(rows))
	for i := range rows {
		view, err := decodeUpgradeProposal(&rows[i])
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *AgentUpgradeService) GetUpgradeProposal(ctx context.Context, tenantID uint64, proposalID string) (interfaces.UpgradeProposalView, error) {
	if tenantID == 0 || strings.TrimSpace(proposalID) == "" {
		return interfaces.UpgradeProposalView{}, ErrAgentUpgradeInvalidInput
	}
	if err := s.reconcileProposals(ctx, tenantID); err != nil {
		return interfaces.UpgradeProposalView{}, err
	}
	row, err := s.repo.GetProposal(ctx, tenantID, strings.TrimSpace(proposalID))
	if err != nil {
		return interfaces.UpgradeProposalView{}, err
	}
	if row == nil {
		return interfaces.UpgradeProposalView{}, ErrAgentUpgradeNotFound
	}
	return decodeUpgradeProposal(row)
}

func (s *AgentUpgradeService) AcceptUpgradeProposal(ctx context.Context, tenantID uint64, actorID, proposalID string, input interfaces.UpgradeVariantInput) (interfaces.AdoptionVariantView, interfaces.UpgradeProposalView, error) {
	proposalID, input.Name, actorID = strings.TrimSpace(proposalID), strings.TrimSpace(input.Name), strings.TrimSpace(actorID)
	if tenantID == 0 || actorID == "" || proposalID == "" || input.Name == "" {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, ErrAgentUpgradeInvalidInput
	}
	row, err := s.repo.GetProposal(ctx, tenantID, proposalID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, err
	}
	if row == nil {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, ErrAgentUpgradeNotFound
	}
	if row.State != AgentUpgradeProposalStateOpen {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{},
			fmt.Errorf("%w: proposal is %q, not %q", ErrAgentUpgradeStateConflict, row.State, AgentUpgradeProposalStateOpen)
	}
	adoption, err := s.repo.GetAdoption(ctx, tenantID, row.AdoptionID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, err
	}
	if adoption == nil {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, ErrAgentUpgradeNotFound
	}
	if adoption.State != AgentAdoptionStateActive {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{},
			fmt.Errorf("%w: adoption state is %q", ErrAgentUpgradeStateConflict, adoption.State)
	}
	toRelease, err := s.repo.GetRelease(ctx, tenantID, row.ToReleaseID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, err
	}
	if toRelease == nil || toRelease.ListingID != row.ListingID {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{},
			fmt.Errorf("%w: proposed release does not belong to the listing", ErrAgentUpgradeInvalidInput)
	}
	// 接受 = 以新 Release 创建一个新的草稿 Variant（spec §9）。随后走 #59
	// 既有 mapping/test/publish 流程；本方法绝不触碰其他 Variant 或既有
	// 本地 Agent Version。
	variant, err := s.repo.CreateVariant(ctx, &types.AgentAdoptionVariantEntity{
		TenantID: tenantID, AdoptionID: row.AdoptionID, ReleaseID: row.ToReleaseID,
		Name: input.Name, State: AgentVariantStateDraft, CreatedBy: actorID,
	})
	if err != nil {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, err
	}
	manifest, err := releaseManifest(ctx, s.repo, tenantID, row.ToReleaseID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, err
	}
	mappings, err := s.repo.ListCapabilityMappings(ctx, tenantID, variant.ID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, err
	}
	updated, err := s.repo.TransitionProposal(ctx, tenantID, row.ID, []string{AgentUpgradeProposalStateOpen}, AgentUpgradeProposalStateAccepted,
		map[string]any{"accepted_variant_id": variant.ID, "resolved_by": actorID})
	if err != nil {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, err
	}
	proposal, err := decodeUpgradeProposal(updated)
	if err != nil {
		return interfaces.AdoptionVariantView{}, interfaces.UpgradeProposalView{}, err
	}
	return interfaces.AdoptionVariantView{
		AgentAdoptionVariantEntity: *variant,
		MissingCapabilities:        missingCapabilities(manifest.CapabilityRequirements, mappings),
	}, proposal, nil
}

func (s *AgentUpgradeService) DismissUpgradeProposal(ctx context.Context, tenantID uint64, actorID, proposalID string) (interfaces.UpgradeProposalView, error) {
	proposalID, actorID = strings.TrimSpace(proposalID), strings.TrimSpace(actorID)
	if tenantID == 0 || actorID == "" || proposalID == "" {
		return interfaces.UpgradeProposalView{}, ErrAgentUpgradeInvalidInput
	}
	row, err := s.repo.GetProposal(ctx, tenantID, proposalID)
	if err != nil {
		return interfaces.UpgradeProposalView{}, err
	}
	if row == nil {
		return interfaces.UpgradeProposalView{}, ErrAgentUpgradeNotFound
	}
	if row.State != AgentUpgradeProposalStateOpen {
		return interfaces.UpgradeProposalView{},
			fmt.Errorf("%w: proposal is %q, not %q", ErrAgentUpgradeStateConflict, row.State, AgentUpgradeProposalStateOpen)
	}
	updated, err := s.repo.TransitionProposal(ctx, tenantID, row.ID, []string{AgentUpgradeProposalStateOpen}, AgentUpgradeProposalStateDismissed,
		map[string]any{"resolved_by": actorID})
	if err != nil {
		return interfaces.UpgradeProposalView{}, err
	}
	return decodeUpgradeProposal(updated)
}

// reconcileProposals materializes one open proposal per (active adoption,
// newer listing release) pair. Tenant-local and introduced listings both
// resolve through the embedded adoption repository (local rows win, the
// #60 introduction ledger synthesizes the rest). A release that fails to
// decode is skipped with a log line — fail closed, never a half-built
// proposal, never a failing listing read.
func (s *AgentUpgradeService) reconcileProposals(ctx context.Context, tenantID uint64) error {
	adoptions, err := s.repo.ListAdoptions(ctx, tenantID)
	if err != nil {
		return err
	}
	for i := range adoptions {
		adoption := adoptions[i]
		if adoption.State != AgentAdoptionStateActive {
			continue
		}
		listing, err := s.repo.GetMarketplaceListing(ctx, tenantID, adoption.ListingID)
		if err != nil {
			return err
		}
		if listing == nil || listing.CurrentReleaseID == nil {
			continue
		}
		toReleaseID := *listing.CurrentReleaseID
		if toReleaseID == adoption.AcceptedReleaseID {
			continue
		}
		fromRelease, err := s.repo.GetRelease(ctx, tenantID, adoption.AcceptedReleaseID)
		if err != nil {
			return err
		}
		if fromRelease == nil {
			continue
		}
		toRelease, err := s.repo.GetRelease(ctx, tenantID, toReleaseID)
		if err != nil {
			return err
		}
		if toRelease == nil {
			continue
		}
		diff, err := diffUpgradeBundles(fromRelease, toRelease)
		if err != nil {
			logger.WarnWithFields(ctx, logger.Fields{
				"tenant_id": tenantID, "adoption_id": adoption.ID,
				"from_release_id": adoption.AcceptedReleaseID, "to_release_id": toReleaseID,
				"reason": err.Error(),
			}, "agent upgrade: skip proposal materialization, release payload unreadable")
			continue
		}
		raw, err := json.Marshal(diff)
		if err != nil {
			return err
		}
		if _, _, err := s.repo.FindOrCreateProposal(ctx, &types.AgentUpgradeProposalEntity{
			TenantID: tenantID, AdoptionID: adoption.ID, ListingID: adoption.ListingID,
			FromReleaseID: adoption.AcceptedReleaseID, ToReleaseID: toReleaseID,
			ToSemanticVersion: toRelease.SemanticVersion, DiffJSON: string(raw),
			State: AgentUpgradeProposalStateOpen,
		}); err != nil {
			return err
		}
	}
	return nil
}

// decodeUpgradeProposal decodes the stored DiffJSON. A malformed payload is
// a storage defect: it surfaces as an error (500 path) instead of a silent
// empty diff.
func decodeUpgradeProposal(row *types.AgentUpgradeProposalEntity) (interfaces.UpgradeProposalView, error) {
	view := interfaces.UpgradeProposalView{
		AgentUpgradeProposalEntity: *row,
		Diff: types.AgentUpgradeDiff{
			Behavior:     []types.UpgradeFieldChange{},
			Dependencies: []types.UpgradeDependencyChange{},
			Security:     []types.UpgradeFieldChange{},
			License:      []types.UpgradeLicenseChange{},
		},
	}
	if strings.TrimSpace(row.DiffJSON) != "" && row.DiffJSON != "{}" {
		if err := json.Unmarshal([]byte(row.DiffJSON), &view.Diff); err != nil {
			return interfaces.UpgradeProposalView{}, fmt.Errorf("decode upgrade proposal %s diff: %w", row.ID, err)
		}
	}
	if view.Diff.Behavior == nil {
		view.Diff.Behavior = []types.UpgradeFieldChange{}
	}
	if view.Diff.Dependencies == nil {
		view.Diff.Dependencies = []types.UpgradeDependencyChange{}
	}
	if view.Diff.Security == nil {
		view.Diff.Security = []types.UpgradeFieldChange{}
	}
	if view.Diff.License == nil {
		view.Diff.License = []types.UpgradeLicenseChange{}
	}
	return view, nil
}
```

同时：把 Task 3 步骤 4 加在 `internal/application/service/agent_upgrade_diff.go` 的临时哨兵 `var ErrAgentUpgradeInvalidInput = ...`（含 `import "errors"`）删除——哨兵现由本文件提供。

- [ ] **Step 4: 运行测试确认通过**

```sh
go test ./internal/application/service/ -run 'TestAgentUpgradeService|TestDiffUpgradeBundles|TestAgentAdoption' -count=1
```

Expected: PASS（本计划 4 个新服务测试 + Task 3 的 4 个差异测试 + #59 存量 adoption 服务测试回归全绿）。

- [ ] **Step 5: Commit**

```bash
git add internal/types/interfaces/agent_upgrade.go internal/application/service/agent_upgrade.go internal/application/service/agent_upgrade_test.go internal/application/service/agent_upgrade_diff.go
git commit -m "feat(marketplace): upgrade proposal service with reconcile-on-read and terminal states (T31 #61 task 4)"
```

---

### Task 5: Handler + 路由 + 容器装配

**Files:**
- Create: `internal/handler/agent_upgrade.go`
- Create: `internal/router/routes_agent_upgrade.go`
- Modify: `internal/container/container.go`（在 `:444` `must(container.Provide(handler.NewAgentAdoptionHandler))` 之后插入三行 Provide）
- Modify: `internal/router/router.go`（`:106` 的 params 字段后加一行；`:413` 的注册行后加一行）
- Test: `internal/router/routes_agent_upgrade_test.go`（本任务先只放路由地板/nil 熔断测试；e2e 在 Task 6 同文件追加）

**Interfaces:**
- Consumes: Task 4 的 `interfaces.AgentUpgradeService`/`UpgradeVariantInput`/`UpgradeProposalView` 与哨兵错误；既有 handler 共享件 `decodeAgentMarketplaceBody`/`invalidMarketplaceBody`/`agentMarketplaceMaxRequestBytes`（`internal/handler/agent_marketplace.go:21,139,155`）、`sandboxConfigTenantID`（`internal/handler/sandbox_config.go:92`）、`types.UserIDFromContext`、`adoptionVariantDTO`/`adoptionClientError` 同款错误映射模式（`internal/handler/agent_adoption.go`）；router 侧 `g.apiKeyRoute`/`apiKeyFullAccess`（`internal/router/rbac.go:230`）/`rbacGuards`。
- Produces（wire 契约，Task 6 e2e 与 #63/#64 消费）:
  - `GET  /api/v1/marketplace/tenant/upgrade-proposals` → `{"success":true,"data":[upgradeProposalResponse...]}`（读时对账后全量，created_at ASC）
  - `GET  /api/v1/marketplace/tenant/upgrade-proposals/:id` → `{"success":true,"data":upgradeProposalResponse}`
  - `POST /api/v1/marketplace/tenant/upgrade-proposals/:id/accept`，body `{"name":"..."}` → `{"success":true,"data":{"proposal":upgradeProposalResponse,"variant":adoptionVariantResponse}}`
  - `POST /api/v1/marketplace/tenant/upgrade-proposals/:id/dismiss` → `{"success":true,"data":upgradeProposalResponse}`（state=dismissed）
  - `upgradeProposalResponse`：`id/adoption_id/listing_id/from_release_id/to_release_id/to_semantic_version/state/accepted_variant_id?/resolved_by?/diff{behavior,dependencies,security,license}/created_at/updated_at`
  - 错误映射：NotFound→404、`ErrAgentUpgradeStateConflict`/`ErrAgentUpgradeProposalTransition`→409（message 即原因，与 Adoption 治理面同款透传）、InvalidInput→400
  - 四条路由全部 Admin+ full-access 地板；handler 为 nil 时 `RegisterAgentUpgradeRoutes` 不挂载任何路由（fail closed）

- [ ] **Step 1: 写失败测试（RED）**

`internal/router/routes_agent_upgrade_test.go` 全文（Task 6 将向此文件追加 e2e 测试与 helper）：

```go
package router

// Agent-upgrade route tests (T31 #61). This file hosts the governance-floor
// assertions (Task 5) and the full-lifecycle HTTP e2e (Task 6).

import (
	"net/http"
	"testing"

	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAgentUpgradeRoutesRequireAdminAndFullAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// 升级建议审阅是治理写面（spec §9 管理员比较/接受/驳回）：与 Adoption
	// 治理路由同款 Admin+ full-access 地板（spec §13 adopt_agent /
	// configure_variant）。
	g := &rbacGuards{}
	v1 := gin.New().Group("/api/v1")
	RegisterAgentUpgradeRoutes(v1, handler.NewAgentUpgradeHandler(nil), g)

	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals"},
		{http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals/:id"},
		{http.MethodPost, "/api/v1/marketplace/tenant/upgrade-proposals/:id/accept"},
		{http.MethodPost, "/api/v1/marketplace/tenant/upgrade-proposals/:id/dismiss"},
	}
	for _, route := range routes {
		policy := mustLookupAPIKeyPolicy(t, g, route.method, route.path)
		require.Truef(t, policy.RequireFullAccess, "%s %s 必须要求 full-access", route.method, route.path)
		require.Falsef(t, policyHasCapability(policy, types.APIKeyCapabilityIngest),
			"%s %s 不得被无关能力放行", route.method, route.path)
	}

	// nil handler fail closed：不挂载任何路由。
	bare := gin.New()
	RegisterAgentUpgradeRoutes(bare.Group("/api/v1"), nil, &rbacGuards{})
	require.Empty(t, bare.Routes())
}
```

- [ ] **Step 2: 运行确认失败**

```sh
go test ./internal/router/ -run TestAgentUpgradeRoutesRequireAdminAndFullAccess -count=1
```

Expected: FAIL（编译错误 `undefined: RegisterAgentUpgradeRoutes` / `undefined: handler.NewAgentUpgradeHandler`）。

- [ ] **Step 3: 写 Handler（GREEN 第一半）**

`internal/handler/agent_upgrade.go` 全文：

```go
package handler

import (
	stderrors "errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	marketrepo "github.com/Tencent/WeKnora/internal/application/repository"
	marketservice "github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// AgentUpgradeHandler is the HTTP boundary for reviewable upgrade proposals
// (T31 #61, spec §9). Same governance floor as the adoption routes: listing
// proposals, accepting (creating the upgrade draft) and dismissing are
// admin-grade acts, so every endpoint is Admin+ with the full-access
// API-key floor. Tenant and actor identity always come from the
// authenticated request context; request bodies never carry principals
// (strict decoding rejects spoof attempts).
type AgentUpgradeHandler struct {
	upgrades interfaces.AgentUpgradeService
}

func NewAgentUpgradeHandler(upgrades interfaces.AgentUpgradeService) *AgentUpgradeHandler {
	return &AgentUpgradeHandler{upgrades: upgrades}
}

type acceptUpgradeProposalBody struct {
	Name string `json:"name"`
}

type upgradeProposalResponse struct {
	ID                string                 `json:"id"`
	AdoptionID        string                 `json:"adoption_id"`
	ListingID         string                 `json:"listing_id"`
	FromReleaseID     string                 `json:"from_release_id"`
	ToReleaseID       string                 `json:"to_release_id"`
	ToSemanticVersion string                 `json:"to_semantic_version"`
	State             string                 `json:"state"`
	AcceptedVariantID string                 `json:"accepted_variant_id,omitempty"`
	ResolvedBy        string                 `json:"resolved_by,omitempty"`
	Diff              types.AgentUpgradeDiff `json:"diff"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

func upgradeProposalDTO(view interfaces.UpgradeProposalView) upgradeProposalResponse {
	return upgradeProposalResponse{
		ID: view.ID, AdoptionID: view.AdoptionID, ListingID: view.ListingID,
		FromReleaseID: view.FromReleaseID, ToReleaseID: view.ToReleaseID,
		ToSemanticVersion: view.ToSemanticVersion, State: view.State,
		AcceptedVariantID: view.AcceptedVariantID, ResolvedBy: view.ResolvedBy,
		Diff: view.Diff, CreatedAt: view.CreatedAt, UpdatedAt: view.UpdatedAt,
	}
}

// acceptUpgradeResponse pairs the resolved proposal with the freshly
// created upgrade draft. Deliberately NOT the adoption publish envelope
// (B3-F86 applies to variant-state endpoints): acceptance creates a draft
// AND resolves the proposal, so the response names both.
type acceptUpgradeResponse struct {
	Proposal upgradeProposalResponse     `json:"proposal"`
	Variant  adoptionVariantResponse `json:"variant"`
}

func upgradeClientError(err error) error {
	switch {
	case stderrors.Is(err, marketservice.ErrAgentUpgradeNotFound),
		stderrors.Is(err, marketrepo.ErrAgentAdoptionNotFound),
		stderrors.Is(err, marketrepo.ErrAgentUpgradeProposalNotFound):
		return apperrors.NewNotFoundError("agent upgrade proposal not found")
	case stderrors.Is(err, marketservice.ErrAgentUpgradeStateConflict),
		stderrors.Is(err, marketrepo.ErrAgentUpgradeProposalTransition),
		stderrors.Is(err, marketrepo.ErrAgentAdoptionVariantTransition):
		// The refusal message IS the explicit reason (terminal state,
		// conflicting adoption state) — pass it through verbatim.
		return apperrors.NewConflictError(err.Error())
	case stderrors.Is(err, marketservice.ErrAgentUpgradeInvalidInput):
		return apperrors.NewValidationError("invalid agent upgrade proposal request")
	default:
		return err
	}
}

func (h *AgentUpgradeHandler) ListUpgradeProposals(c *gin.Context) {
	views, err := h.upgrades.ListUpgradeProposals(c.Request.Context(), sandboxConfigTenantID(c))
	if err != nil {
		_ = c.Error(upgradeClientError(err))
		return
	}
	data := make([]upgradeProposalResponse, 0, len(views))
	for _, view := range views {
		data = append(data, upgradeProposalDTO(view))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func (h *AgentUpgradeHandler) GetUpgradeProposal(c *gin.Context) {
	view, err := h.upgrades.GetUpgradeProposal(c.Request.Context(), sandboxConfigTenantID(c), c.Param("id"))
	if err != nil {
		_ = c.Error(upgradeClientError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": upgradeProposalDTO(view)})
}

func (h *AgentUpgradeHandler) AcceptUpgradeProposal(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agentMarketplaceMaxRequestBytes)
	var body *acceptUpgradeProposalBody
	if err := decodeAgentMarketplaceBody(c.Request.Body, &body); err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	if body == nil || strings.TrimSpace(body.Name) == "" {
		invalidMarketplaceBody(c, stderrors.New("name is required"))
		return
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	variant, proposal, err := h.upgrades.AcceptUpgradeProposal(c.Request.Context(), sandboxConfigTenantID(c), actorID, c.Param("id"), interfaces.UpgradeVariantInput{Name: body.Name})
	if err != nil {
		_ = c.Error(upgradeClientError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": acceptUpgradeResponse{
		Proposal: upgradeProposalDTO(proposal), Variant: adoptionVariantDTO(variant),
	}})
}

func (h *AgentUpgradeHandler) DismissUpgradeProposal(c *gin.Context) {
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	view, err := h.upgrades.DismissUpgradeProposal(c.Request.Context(), sandboxConfigTenantID(c), actorID, c.Param("id"))
	if err != nil {
		_ = c.Error(upgradeClientError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": upgradeProposalDTO(view)})
}
```

- [ ] **Step 4: 写路由 + 容器装配（GREEN 第二半）**

`internal/router/routes_agent_upgrade.go` 全文：

```go
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/handler"
)

// RegisterAgentUpgradeRoutes mounts the reviewable upgrade-proposal surface
// beside the adoption routes (T31 #61, spec §9). Proposal review,
// acceptance and dismissal are admin-grade governance acts, so every route
// is Admin+ with the full-access API-key floor — mirroring
// RegisterAgentAdoptionRoutes. A nil handler mounts nothing (fail closed).
func RegisterAgentUpgradeRoutes(r *gin.RouterGroup, upgradeHandler *handler.AgentUpgradeHandler, g *rbacGuards) {
	if upgradeHandler == nil {
		return
	}
	admin := apiKeyFullAccess()
	g.apiKeyRoute(r, http.MethodGet, "/marketplace/tenant/upgrade-proposals", admin, g.Admin(), upgradeHandler.ListUpgradeProposals)
	g.apiKeyRoute(r, http.MethodGet, "/marketplace/tenant/upgrade-proposals/:id", admin, g.Admin(), upgradeHandler.GetUpgradeProposal)
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/tenant/upgrade-proposals/:id/accept", admin, g.Admin(), upgradeHandler.AcceptUpgradeProposal)
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/tenant/upgrade-proposals/:id/dismiss", admin, g.Admin(), upgradeHandler.DismissUpgradeProposal)
}
```

`internal/container/container.go`——在 `must(container.Provide(handler.NewAgentAdoptionHandler))`（:444）之后插入：

```go
	must(container.Provide(repository.NewAgentUpgradeRepository))
	must(container.Provide(func(repo repository.AgentUpgradeRepository) interfaces.AgentUpgradeService {
		return service.NewAgentUpgradeService(repo)
	}))
	must(container.Provide(handler.NewAgentUpgradeHandler))
```

`internal/router/router.go`——在 `:106` `AgentAdoptionHandler           *handler.AgentAdoptionHandler` 之后加一行：

```go
	AgentUpgradeHandler            *handler.AgentUpgradeHandler
```

在 `:413` `RegisterAgentAdoptionRoutes(v1, params.AgentAdoptionHandler, rbacGuards)` 之后加一行：

```go
		RegisterAgentUpgradeRoutes(v1, params.AgentUpgradeHandler, rbacGuards)
```

- [ ] **Step 5: 运行路由测试 + 生产装配编译确认通过**

```sh
go build ./internal/... && go test ./internal/router/ -run TestAgentUpgradeRoutesRequireAdminAndFullAccess -count=1
```

Expected: build exit 0，测试 PASS。

- [ ] **Step 6: Commit**

```bash
git add internal/handler/agent_upgrade.go internal/router/routes_agent_upgrade.go internal/router/routes_agent_upgrade_test.go internal/container/container.go internal/router/router.go
git commit -m "feat(marketplace): upgrade proposal HTTP surface with admin floor and wiring (T31 #61 task 5)"
```

---

### Task 6: 端到端 HTTP 证据（AC1 / AC2 / AC3）

**Files:**
- Modify: `internal/router/routes_agent_upgrade_test.go`（追加 helper 与 e2e 测试）

**Interfaces:**
- Consumes: 同包既有测试件 `openTenantAgentMarketplaceHTTPTestDB`（routes_agent_marketplace_test.go:349）、`adoptionCall`（routes_agent_adoption_test.go:74）、`marketplaceHTTPResolver`（routes_agent_marketplace_test.go）、`mustLookupAPIKeyPolicy`/`policyHasCapability`（router_api_key_capabilities_test.go:664,678）；Task 1-5 全部产出；真实端点 `POST /api/v1/agents/:id/versions`（冻结）、`POST /api/v1/marketplace/tenant/release-submissions`、`.../review`（发布）、`POST /marketplace/tenant/adoptions`、`POST .../adoptions/:id/variants`、`PUT .../variants/:id/capability-mapping`、`POST .../variants/:id/test`、`POST .../variants/:id/publish`、`GET /api/v1/agents`、`GET /marketplace/tenant/available-agents`、`GET /api/v1/agents/:agentId/versions/:versionId`。
- Produces: AC1/AC2/AC3 的最高稳定 Interface 证据（真实迁移流 + 真实服务栈 + 真实 HTTP）。

- [ ] **Step 1: 向 `routes_agent_upgrade_test.go` 追加 helper（写失败测试第一半）**

文件末尾追加：

```go
// ---------- Task 6: end-to-end lifecycle over the real stack ----------

// newAgentUpgradeTestApp mounts the REAL release -> adoption -> upgrade
// stack over the real migration stream: real CustomAgentService /
// AgentVersionService / AgentMarketplaceService / AgentAdoptionService /
// AgentUpgradeService and the real agent list endpoint (GET /api/v1/agents)
// that the mobile Resource Shelf consumes. Setup mirrors
// newAgentAdoptionTestApp without modifying it (parallel-batch merge safety).
func newAgentUpgradeTestApp(t *testing.T) (*gin.Engine, *rbacGuards, *gorm.DB) {
	t.Helper()
	db := openTenantAgentMarketplaceHTTPTestDB(t)
	require.NoError(t, db.Create(&types.CustomAgent{
		ID: "agent-owned", Name: "Upgrade helper", TenantID: 1, CreatedBy: "contributor",
		Config: types.CustomAgentConfig{AgentMode: "smart-reasoning", SystemPrompt: "Be useful."},
	}).Error)

	marketRepo := repository.NewAgentMarketplaceRepository(db)
	customAgents := service.NewCustomAgentService(repository.NewCustomAgentRepository(db), nil, nil, nil, nil, nil, nil)
	versions := service.NewAgentVersionService(customAgents, repository.NewAgentVersionRepository(db))
	market := service.NewAgentMarketplaceService(versions, marketplaceHTTPResolver{}, marketRepo, t.TempDir())
	adoptions := service.NewAgentAdoptionService(repository.NewAgentAdoptionRepository(db), customAgents, versions)
	upgrades := service.NewAgentUpgradeService(repository.NewAgentUpgradeRepository(db))

	versionHandler := handler.NewAgentVersionHandler(versions)
	marketHandler := handler.NewAgentMarketplaceHandler(market, versions)
	adoptionHandler := handler.NewAgentAdoptionHandler(adoptions)
	upgradeHandler := handler.NewAgentUpgradeHandler(upgrades)
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
	RegisterAgentUpgradeRoutes(v1, upgradeHandler, g)
	RegisterCustomAgentRoutes(v1, agentListHandler, g)
	return r, g, db
}

// freezeAndPublishUpgradeRelease drives the real HTTP release workflow for
// ONE semantic version of agent-owned: freeze a fresh AgentVersion of the
// CURRENT agent row, submit a Release with the given metadata and approve
// it. metadata 必须自带 supported_languages/use_cases/capability_requirements/
// minimum_weknora_capability/license_id（semantic_version/display_name/summary
// 由本 helper 注入）。返回 listing id 与 release id。
func freezeAndPublishUpgradeRelease(t *testing.T, r *gin.Engine, semanticVersion string, metadata map[string]any) (listingID, releaseID string) {
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

	metadata["semantic_version"] = semanticVersion
	metadata["display_name"] = "Upgrade helper"
	metadata["summary"] = "Portable upgrade helper"
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

// publishUpgradeVariant drives the #59 variant lifecycle to published over
// real HTTP: create draft, map both capabilities, test, publish. Returns
// the variant id and the instantiated local agent id.
func publishUpgradeVariant(t *testing.T, r *gin.Engine, adoptionID, name, modelID, knowledgeID string) (variantID, localAgentID string) {
	t.Helper()
	variant := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionID+"/variants", "admin", "admin", map[string]any{"name": name})
	require.Equal(t, http.StatusCreated, variant.Code, variant.Body.String())
	var variantBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(variant.Body.Bytes(), &variantBody))
	mapping := adoptionCall(r, 1, http.MethodPut, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/capability-mapping", "admin", "admin", map[string]any{"mappings": []map[string]any{
		{"capability": "model", "model_id": modelID},
		{"capability": "knowledge", "knowledge_base_ids": []string{knowledgeID}},
	}})
	require.Equal(t, http.StatusOK, mapping.Code, mapping.Body.String())
	tested := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/test", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, tested.Code, tested.Body.String())
	published := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/publish", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, published.Code, published.Body.String())
	var publishBody struct {
		Data struct {
			LocalAgentID string `json:"local_agent_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(published.Body.Bytes(), &publishBody))
	require.NotEmpty(t, publishBody.Data.LocalAgentID)
	return variantBody.Data.ID, publishBody.Data.LocalAgentID
}

// agentsRowByName reads the REAL GET /api/v1/agents wire and returns the
// config of the named agent (the projection the mobile Resource Shelf maps).
func agentsRowByName(t *testing.T, r *gin.Engine, name string) (found bool, systemPrompt string, modelID string) {
	t.Helper()
	resp := adoptionCall(r, 1, http.MethodGet, "/api/v1/agents", "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	var body struct {
		Data []struct {
			Name string `json:"name"`
			Config struct {
				SystemPrompt string `json:"system_prompt"`
				ModelID      string `json:"model_id"`
			} `json:"config"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	for _, row := range body.Data {
		if row.Name == name {
			return true, row.Config.SystemPrompt, row.Config.ModelID
		}
	}
	return false, "", ""
}

// upgradeDiffWire mirrors the four-dimension diff on the wire.
type upgradeDiffWire struct {
	Behavior []struct {
		Field string `json:"field"`
		From  string `json:"from"`
		To    string `json:"to"`
	} `json:"behavior"`
	Dependencies []struct {
		Type        string `json:"type"`
		ID          string `json:"id"`
		Change      string `json:"change"`
		FromVersion string `json:"from_version"`
		ToVersion   string `json:"to_version"`
	} `json:"dependencies"`
	Security []struct {
		Field string `json:"field"`
		From  string `json:"from"`
		To    string `json:"to"`
	} `json:"security"`
	License []struct {
		Scope string `json:"scope"`
		ID    string `json:"id"`
		From  string `json:"from"`
		To    string `json:"to"`
	} `json:"license"`
}
```

并在文件 import 块补齐：`"context"`, `"encoding/json"`, `"github.com/Tencent/WeKnora/internal/application/repository"`, `"github.com/Tencent/WeKnora/internal/application/service"`, `"github.com/Tencent/WeKnora/internal/config"`, `"github.com/Tencent/WeKnora/internal/middleware"`, `"gorm.io/gorm"`。

- [ ] **Step 2: 追加 e2e 主测试（写失败测试第二半，RED）**

继续在文件末尾追加：

```go
func TestAgentUpgradeProposalLifecycleKeepsOldVariantsAndTasks(t *testing.T) {
	r, _, db := newAgentUpgradeTestApp(t)

	// ---- v1 发布（真实 HTTP：冻结 → 提交 → 审核）----
	upgradeMetadataV1 := func() map[string]any {
		return map[string]any{
			"supported_languages": []string{"en"}, "use_cases": []string{"support"},
			"capability_requirements": []string{"model", "knowledge"},
			"minimum_weknora_capability": "1", "license_id": "MIT",
		}
	}
	listingID, v1 := freezeAndPublishUpgradeRelease(t, r, "1.0.0", upgradeMetadataV1())

	// ---- Adoption + 第一个 Variant 到 published（走 #59 既有端点）----
	adopted := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID})
	require.Equal(t, http.StatusCreated, adopted.Code, adopted.Body.String())
	var adoptionBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(adopted.Body.Bytes(), &adoptionBody))
	oldVariantID, oldAgentID := publishUpgradeVariant(t, r, adoptionBody.Data.ID, "Sales Assistant", "gpt-x", "kb-sales")

	// 记录旧世界：旧本地 agent 行已入库（新 Release 落地前后逐字段比对）。
	var oldAgent types.CustomAgent
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", uint64(1), oldAgentID).First(&oldAgent).Error)

	// ---- 新 Release 落地前：无建议 ----
	before := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, before.Code, before.Body.String())
	require.JSONEq(t, `{"success":true,"data":[]}`, before.Body.String())

	// ---- v2 发布：真实行为差异（改源 agent 配置后重新冻结）----
	var sourceAgent types.CustomAgent
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", uint64(1), "agent-owned").First(&sourceAgent).Error)
	sourceAgent.Config.SystemPrompt = "Be extra useful and cite sources."
	require.NoError(t, db.Save(&sourceAgent).Error)
	metadataV2 := upgradeMetadataV1()
	metadataV2["data_categories"] = []string{"chat_content"}
	metadataV2["external_side_effects"] = []string{"web_search"}
	metadataV2["license_id"] = "Apache-2.0"
	_, v2 := freezeAndPublishUpgradeRelease(t, r, "1.1.0", metadataV2)

	// ---- AC2：建议出现，四维差异可审阅 ----
	listed := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, listed.Code, listed.Body.String())
	var listBody struct {
		Data []struct {
			ID                string          `json:"id"`
			AdoptionID        string          `json:"adoption_id"`
			ListingID         string          `json:"listing_id"`
			FromReleaseID     string          `json:"from_release_id"`
			ToReleaseID       string          `json:"to_release_id"`
			ToSemanticVersion string          `json:"to_semantic_version"`
			State             string          `json:"state"`
			Diff              upgradeDiffWire `json:"diff"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listed.Body.Bytes(), &listBody))
	require.Len(t, listBody.Data, 1)
	proposal := listBody.Data[0]
	require.Equal(t, adoptionBody.Data.ID, proposal.AdoptionID)
	require.Equal(t, listingID, proposal.ListingID)
	require.Equal(t, v1, proposal.FromReleaseID)
	require.Equal(t, v2, proposal.ToReleaseID)
	require.Equal(t, "1.1.0", proposal.ToSemanticVersion)
	require.Equal(t, "open", proposal.State)

	// 行为维：系统提示词（真实冻结→导出链产生的差异）。
	require.Len(t, proposal.Diff.Behavior, 1)
	require.Equal(t, "system_prompt", proposal.Diff.Behavior[0].Field)
	require.Equal(t, "Be useful.", proposal.Diff.Behavior[0].From)
	require.Equal(t, "Be extra useful and cite sources.", proposal.Diff.Behavior[0].To)

	// 安全维：数据类别 + 外部副作用扩大（capability_requirements 未变 → 不出现）。
	require.Len(t, proposal.Diff.Security, 2)
	require.Equal(t, "data_categories", proposal.Diff.Security[0].Field)
	require.Equal(t, "", proposal.Diff.Security[0].From)
	require.Equal(t, "chat_content", proposal.Diff.Security[0].To)
	require.Equal(t, "external_side_effects", proposal.Diff.Security[1].Field)
	require.Equal(t, "", proposal.Diff.Security[1].From)
	require.Equal(t, "web_search", proposal.Diff.Security[1].To)

	// 许可维：Release 许可 MIT → Apache-2.0（依赖段为空——生产依赖解析器
	// tenantReleaseDependencyResolver fail-closed，真实发布路径无法产生非空
	// DependencyLock；四维全量覆盖由 Task 3 纯函数测试承载）。
	require.Len(t, proposal.Diff.Dependencies, 0)
	require.Len(t, proposal.Diff.License, 1)
	require.Equal(t, "release", proposal.Diff.License[0].Scope)
	require.Equal(t, "license_id", proposal.Diff.License[0].ID)
	require.Equal(t, "MIT", proposal.Diff.License[0].From)
	require.Equal(t, "Apache-2.0", proposal.Diff.License[0].To)

	// 对账幂等（HTTP 面）：再列一次仍恰好一条。
	relisted := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, relisted.Code)
	var relistedBody struct {
		Data []json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(relisted.Body.Bytes(), &relistedBody))
	require.Len(t, relistedBody.Data, 1)

	// ---- 治理边界：viewer 403、跨租户空列表 + 404、畸形 body 400 ----
	viewerList := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals", "viewer", "viewer", nil)
	require.Equal(t, http.StatusForbidden, viewerList.Code)
	tenant2List := adoptionCall(r, 2, http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, tenant2List.Code)
	require.JSONEq(t, `{"success":true,"data":[]}`, tenant2List.Body.String(), "他租户看不到 tenant-1 的建议")
	tenant2Get := adoptionCall(r, 2, http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals/"+proposal.ID, "admin", "admin", nil)
	require.Equal(t, http.StatusNotFound, tenant2Get.Code, "跨租户枚举与不存在同形")
	missingGet := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals/missing-proposal", "admin", "admin", nil)
	require.Equal(t, http.StatusNotFound, missingGet.Code)
	emptyName := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/upgrade-proposals/"+proposal.ID+"/accept", "admin", "admin", map[string]any{"name": "  "})
	require.Equal(t, http.StatusBadRequest, emptyName.Code)
	spoofed := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/upgrade-proposals/"+proposal.ID+"/accept", "admin", "admin", map[string]any{"name": "X", "tenant_id": 999})
	require.Equal(t, http.StatusBadRequest, spoofed.Code, "严格解码拒绝 body 冒充 principal")

	// ---- 接受建议：只创建新 Release 上的草稿 Variant（AC1 的接受路径）----
	accepted := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/upgrade-proposals/"+proposal.ID+"/accept", "admin", "admin", map[string]any{"name": "Sales Assistant v1.1"})
	require.Equal(t, http.StatusOK, accepted.Code, accepted.Body.String())
	var acceptBody struct {
		Data struct {
			Proposal struct {
				ID                string `json:"id"`
				State             string `json:"state"`
				AcceptedVariantID string `json:"accepted_variant_id"`
			} `json:"proposal"`
			Variant struct {
				ID                  string   `json:"id"`
				ReleaseID           string   `json:"release_id"`
				State               string   `json:"state"`
				MissingCapabilities []string `json:"missing_capabilities"`
			} `json:"variant"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(accepted.Body.Bytes(), &acceptBody))
	require.Equal(t, "accepted", acceptBody.Data.Proposal.State)
	require.Equal(t, acceptBody.Data.Proposal.AcceptedVariantID, acceptBody.Data.Variant.ID)
	require.Equal(t, v2, acceptBody.Data.Variant.ReleaseID, "草稿固定到新 Release")
	require.Equal(t, "draft", acceptBody.Data.Variant.State)
	require.Equal(t, []string{"knowledge", "model"}, acceptBody.Data.Variant.MissingCapabilities, "新草稿按新 Release 重算缺失能力")

	// 重复 accept：终态互斥，409。
	reAccept := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/upgrade-proposals/"+proposal.ID+"/accept", "admin", "admin", map[string]any{"name": "Again"})
	require.Equal(t, http.StatusConflict, reAccept.Code)
	// draft 不得重复：同一 adoption 的草稿数 = 旧 published 1 + 新 draft 1。
	var draftCount int64
	require.NoError(t, db.Model(&types.AgentAdoptionVariantEntity{}).
		Where("tenant_id = ? AND adoption_id = ? AND state = ?", uint64(1), adoptionBody.Data.ID, "draft").Count(&draftCount).Error)
	require.Equal(t, int64(1), draftCount)

	// ---- AC1：未接受升级的此刻，旧 Variant / 旧本地 Agent / 旧冻结版本原封不动 ----
	var oldVariantRow types.AgentAdoptionVariantEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", uint64(1), oldVariantID).First(&oldVariantRow).Error)
	require.Equal(t, "published", oldVariantRow.State)
	require.Equal(t, v1, oldVariantRow.ReleaseID)
	found, prompt, modelID := agentsRowByName(t, r, "Sales Assistant")
	require.True(t, found)
	require.Equal(t, "Be useful.", prompt, "旧本地 agent 的行为保持旧版本")
	require.Equal(t, "gpt-x", modelID)
	availableNow := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/available-agents", "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, availableNow.Code)
	var availableBody struct {
		Data []struct {
			AgentID string `json:"agent_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(availableNow.Body.Bytes(), &availableBody))
	require.Len(t, availableBody.Data, 1, "草稿不进入移动可用面")
	require.Equal(t, oldAgentID, availableBody.Data[0].AgentID)

	// ---- 各 Variant 可独立重新映射、测试和发布（新草稿走 #59 既有流程）----
	_, newAgentID := publishUpgradeVariant(t, r, adoptionBody.Data.ID, "Sales Assistant v1.1", "gpt-new", "kb-new")
	require.NotEqual(t, oldAgentID, newAgentID, "升级发布实例化新的本地 agent，不覆盖旧的")

	// ---- AC1 终局断言：全流程后旧世界原封不动 ----
	found, prompt, modelID = agentsRowByName(t, r, "Sales Assistant")
	require.True(t, found)
	require.Equal(t, "Be useful.", prompt, "旧 agent 行为逐字节不变")
	require.Equal(t, "gpt-x", modelID)
	found, newPrompt, _ := agentsRowByName(t, r, "Sales Assistant v1.1")
	require.True(t, found)
	require.Equal(t, "Be extra useful and cite sources.", newPrompt, "新 agent 承载新版本行为")
	availableAfter := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/available-agents", "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, availableAfter.Code)
	var availableAfterBody struct {
		Data []struct {
			AgentID string `json:"agent_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(availableAfter.Body.Bytes(), &availableAfterBody))
	require.Len(t, availableAfterBody.Data, 2, "旧变体仍在可用面，新变体并行加入")
	require.Equal(t, oldAgentID, availableAfterBody.Data[0].AgentID, "旧变体保持在前（created_at 序）")

	// 旧冻结版本仍可经 #58 端点读取。
	var oldVariantAfter types.AgentAdoptionVariantEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", uint64(1), oldVariantID).First(&oldVariantAfter).Error)
	require.Equal(t, "published", oldVariantAfter.State)
	require.Equal(t, v1, oldVariantAfter.ReleaseID)
	oldVersionRead := adoptionCall(r, 1, http.MethodGet, "/api/v1/agents/"+oldAgentID+"/versions/"+oldVariantAfter.LocalAgentVersionID, "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, oldVersionRead.Code, oldVersionRead.Body.String())

	// ---- dismiss 路径：v3 → 新建议 → dismiss 终态保留、accepted 不可 dismiss ----
	metadataV3 := upgradeMetadataV1()
	_, v3 := freezeAndPublishUpgradeRelease(t, r, "1.2.0", metadataV3)
	afterV3 := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, afterV3.Code)
	var afterV3Body struct {
		Data []struct {
			ID    string `json:"id"`
			State string `json:"state"`
			To    string `json:"to_release_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(afterV3.Body.Bytes(), &afterV3Body))
	require.Len(t, afterV3Body.Data, 2)
	var openID string
	for _, row := range afterV3Body.Data {
		if row.State == "open" {
			openID = row.ID
			require.Equal(t, v3, row.To)
		}
	}
	require.NotEmpty(t, openID)
	dismissed := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/upgrade-proposals/"+openID+"/dismiss", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, dismissed.Code, dismissed.Body.String())
	require.Contains(t, dismissed.Body.String(), `"state":"dismissed"`)
	require.Contains(t, dismissed.Body.String(), `"resolved_by":"admin"`)
	reDismiss := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/upgrade-proposals/"+openID+"/dismiss", "admin", "admin", nil)
	require.Equal(t, http.StatusConflict, reDismiss.Code)
	acceptedDismiss := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/upgrade-proposals/"+proposal.ID+"/dismiss", "admin", "admin", nil)
	require.Equal(t, http.StatusConflict, acceptedDismiss.Code)
	final := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, final.Code)
	require.Contains(t, final.Body.String(), `"state":"dismissed"`)
	require.NotContains(t, final.Body.String(), `"state":"open"`, "已 resolved 的建议不复活")
}
```

- [ ] **Step 3: 运行确认失败（RED）**

```sh
go test ./internal/router/ -run TestAgentUpgradeProposalLifecycleKeepsOldVariantsAndTasks -count=1
```

Expected: FAIL（首跑可能因 helper 缺 import 或断言细节报编译/断言错误——按错误信息修正测试自身的笔误；实现层已在 Task 1-5 就绪，此测试是它们的集成验收门）。

- [ ] **Step 4: 修至全部通过（GREEN）**

```sh
go test ./internal/router/ -run 'TestAgentUpgradeProposalLifecycleKeepsOldVariantsAndTasks|TestAgentUpgradeRoutesRequireAdminAndFullAccess' -count=1 -v
```

Expected: PASS（2 个测试全部 `--- PASS`）。若 `freezeAndPublishUpgradeRelease` 对 semantic_version 唯一性或冻结版本号断言失败，检查 metadata 是否漏注 `semantic_version`（helper 内已注入）；若 available-agents 顺序断言失败，确认断言使用 `created_at ASC` 序（仓储 `PublishedAvailableAgents` 的既有排序）而非假设。

- [ ] **Step 5: Commit**

```bash
git add internal/router/routes_agent_upgrade_test.go
git commit -m "test(marketplace): upgrade proposal end-to-end lifecycle over real migrations (T31 #61 task 6)"
```

---

## 计划级验证

在 worktree 根执行（覆盖本计划全部定向测试 + 生产装配编译 + Task 0 修复的回归面；避免全量 flaky 套件）：

```sh
go build ./internal/... && go test ./internal/database/ -run 'TestSQLiteMigrations|TestSemanticMigrationSQLiteUpDownUp' -count=1 && go test ./internal/application/repository/ -run 'TestAgentUpgrade|TestAgentAdoption|TestMobileDevice|TestMobilePush' -count=1 && go test ./internal/application/service/ -run 'TestAgentUpgrade|TestDiffUpgradeBundles|TestAgentAdoption' -count=1 && go test ./internal/handler/ -run 'TestMobileDevice|TestPublishVariant' -count=1 && go test ./internal/modules/workbench/service/workbench/ -run 'TestPushPayloadPolicy|TestAppRouting|TestHTTPNotificationProvider|TestDisabledNotificationProvider' -count=1 && go test ./internal/router/ -run 'TestAgentUpgrade|TestTenantAgentAdoption|TestTenantAgentVariant|TestTenantAgentAdoptionPublishes|TestAvailableAgentsAPIKeyFloor' -count=1
```

预期：全部 `ok`（database 4 测试；repository 本计划 3 + adoption/mobile 回归；service 本计划 8 + adoption 回归；handler 回归；workbench policy 8；router 本计划 2 + adoption e2e 回归 4）。已知与本计划无关的既有失败：`internal/modules/workbench/service/workbench` 包的 `TestNotificationDeliveryRejectsResolvedInteractionAfterClaim`（pristine HEAD 实跑复现，见差异记录第 4 条），不在上述 -run 过滤内。

## 验收标准 → 证据映射

| 验收标准 | 证据 |
|---|---|
| AC1 「未接受的 Variant 和既有 Task 保持旧版本。」 | Task 6 e2e：v2 发布→建议出现→旧 Variant 行（state=published、release_id=v1）与旧本地 agent（`GET /api/v1/agents` 行 system_prompt="Be useful."、model_id=gpt-x）逐字段不变；接受只是新草稿（available-agents 仍 1 行）；新 Variant 独立映射/测试/发布后旧 agent 仍原样、available-agents 2 行且旧变体在前；旧冻结版本 `GET /api/v1/agents/:oldAgent/versions/:oldVersion` 仍 200。既有 Task 固定本地 Agent Version（spec §4），本地 agent 行不变即 Task 的版本不变。Review Focus 1 钉死于此。 |
| AC2 「升级差异覆盖行为、依赖、安全和许可。」 | Task 3 纯函数测试（真实 Release 结构体 + 真实函数）：行为 3 项、依赖 added/removed/version_changed/digest_changed 四形态、安全 3 项、许可（依赖许可 + Release 许可）全断言；Task 4 服务测试经真实仓储对账后断言同一四维差异；Task 6 e2e 经 wire 断言行为/安全/许可三维 + 依赖段在场（生产依赖解析器 fail-closed 边界如实声明，见差异记录第 3 条）。 |
| AC3 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」 | Task 6 `TestAgentUpgradeProposalLifecycleKeepsOldVariantsAndTasks`：真实 sqlite 迁移流（Task 0 修复后含 000119）+ 真实五层服务栈 + 真实 HTTP 全生命周期（v1 发布→adopt→variant 发布→v2 发布→建议审阅→接受→新 Variant 发布→v3→dismiss），移动可用面经真实 `GET /api/v1/agents` 与 `available-agents` 验证；Task 2/3/4 如实标注为下层证据。 |

## 差异记录（调查结论 vs 代码现状，以代码现状为准）

1. **缺口核实一致**：`rg "UpgradeProposal|upgrade_proposal" internal/ packages/ apps/` 零匹配；`internal/types/agent_marketplace_persistence.go:5-15` 的 Listing 实体仅有 `State`/`CurrentReleaseID`；`internal/types/agent_marketplace.go:63-64` 的 `ChangeNotes` 仅为 Release 内变更说明字段。本计划从零新增 Upgrade Proposal 实体、差异模型与路由。
2. **波级问题 #1 实测复现并必须修复**：波起点 HEAD 的 sqlite `000114` 双迁移（`000114_mobile_device_app` vs `000114_public_agent_marketplace`）与 versioned `000193` 双迁移实测使 golang-migrate 报 `duplicate migration file`，全量迁移流不可装载——而 Task 6 的 AC3 证据依赖该流（`openTenantAgentMarketplaceHTTPTestDB`、`openAgentVersionServiceTestDB` 均走 `migrator.Up()` 全量）。因此 Task 0 属本计划必需而非顺手修复。重编对象为**后落**的 mobile_device_app（fb9037710 晚于 34565aa41，先例 60122179f 即"后到者前移"）；可用序号经与既有重编历史核对：sqlite 000115-000117 / versioned 000194-000196 已被 #48/#43/#52 占用，故取 000118/000197；本计划新迁移取 000119/000198。修复方案已在本 worktree 实跑验证（`internal/database`、repository/service/router 的 adoption 组、mobile device 组、workbench policy 组全绿），随后已还原工作区、由执行者按 Task 0 步骤重做。
3. **依赖维的生产边界**：`internal/container/container.go:120-137` 的 `tenantReleaseDependencyResolver` 对任何 SelectedSkills/Subagents fail closed（无稳定可再分发版本源），生产发布路径无法产生非空 `DependencyLock`。因此 e2e 的依赖段如实断言为空数组；四维（含依赖增/删/版本/摘要/许可变化）的全量覆盖由 Task 3 以真实 `types.AgentReleaseEntity` 结构体驱动真实 `diffUpgradeBundles` 承载，并在计划内明示这是函数级而非 HTTP 级证据——不存在以 mock 冒充集成的情形。
4. **与本计划无关的既有测试失败**：`internal/modules/workbench/service/workbench` 的 `TestNotificationDeliveryRejectsResolvedInteractionAfterClaim`（notification_delivery_test.go:489）在 pristine HEAD 上即失败（已用 `git stash` 前后对比实跑确认）；它不引用本计划触碰的任何迁移或代码。不纳入计划级验证面，不伪造修复。
5. **零 TS 面**：spec §2「……Adoption 和升级不在移动端执行」（agent-marketplace-domain-model.md）+ CONTEXT.md:137 将升级建议定位为 Web 管理端治理。与 #59 同判：无 contracts/api-client/apps/mobile 改动，天然规避同批次 TS 共享文件合并冲突。
6. **接受响应信封**：`accept` 返回 `{proposal, variant}` 成对结构，**有意偏离** #59 的 B3-F86「data 即 variant 本体」约定——B3-F86 约束的是 variant 状态端点（publish/test 等），accept 同时产生两个可观察结果（proposal 终态 + 新草稿），双键命名是最诚实的投影；已在 `acceptUpgradeResponse` 注释中说明。
7. **无 blocked-env 验收项**：本计划不需要真机、外部凭据或公网主机；全部证据本地可复现。
