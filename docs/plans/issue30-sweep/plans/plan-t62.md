# T32：Agent Fork lineage、许可证与再发布（Issue #62）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在已合并的 #58/#59/#60 Marketplace 体系之上实现 spec §9 的 Fork 治理层：从已引入 Release 派生的 Release Submission 由服务端推导并记录 lineage（来源 Listing/Release、fork 判定、修改说明、来源许可），纯本地能力映射修改永不误判为 Fork（AC1）；来源许可证禁止再分发（或未注册）时，Tenant 与 Public 两条 Submission 通道都由服务端拒绝（AC2）；全部行为以真实 sqlite 迁移 + 真实服务的 HTTP 端到端测试为最高 Interface 证据（AC3）。

**Architecture:** 全部为 Go 后端改动，零 TS / 零移动端改动（spec §2：Submission、Review 不在移动端执行）。数据层：`agent_release_submissions` 与 `agent_releases` 两表各加 5 个 lineage 列，新建平台级许可证注册表 `agent_licenses`（双迁移流 sqlite 000119 / versioned 000198）。仓储层：`agentMarketplaceRepository` 新增 4 个方法（`FindDerivation`/`GetLicense`/`UpsertLicense`/`ListLicenses`，落在新文件 `agent_marketplace_lineage.go`，复用 `introducedRelease` 引入台账回退），`ReviewAndPublishTx` 的 release 构造逐列传播 lineage。服务层：新文件 `agent_fork_lineage.go` 承载 fork 判定——基线不是源 Release 原始载荷，而是「源载荷经 `buildLocalAgent`→`PortablePayloadOf` 的发布管线重投影」，纯映射修改 therefore 字节相等 → `is_fork=false`；许可证门 fail closed（未注册 = 不允许）。导出层：`experts` 新增 `PortablePayloadOf` 与 `BuildAgentReleaseBundleWithLineage`，lineage 段进入 Manifest（digest 边界内），`lineage=nil` 时字节与既有 `BuildAgentReleaseBundle` 完全一致（存量 digest 零回归）。lineage 只由服务端从 `agent_adoption_variants.local_agent_id` 推导，请求体没有任何 lineage 字段（防伪造）。容器接线零改动（方法挂在既有 `AgentMarketplaceRepository` 上）。

**Tech Stack:** Go 1.26（`go.mod` module `github.com/Tencent/WeKnora`）、gin + gorm + golang-migrate。测试基建全部复用已合并批次：`internal/router/routes_agent_marketplace_test.go:349` 的 `openTenantAgentMarketplaceHTTPTestDB`（真实 sqlite 迁移流）、`internal/router/routes_public_marketplace_test.go:29` 的 `newPublicMarketplaceTestApp`/`publicCall`、`internal/application/service/agent_version_test.go` 的 `openAgentVersionServiceTestDB`（真实迁移流）、`internal/application/service/agent_marketplace_test.go:34` 的 `marketplaceResolverFake`。测试命令全部在 worktree 根执行。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-62.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-agent-marketplace-domain-model.md`（§5 Release 内容边界、§6 Manifest、§9 升级/Fork/退出、§15 验收场景 8、§16 首版非目标）
- ADR：`docs/adr/0011-agent-marketplace-release-adoption-boundary.md`（Marketplace 分发不可变 Release，Tenant 派生本地 Variant）
- 领域术语：`CONTEXT.md:134`（Agent Fork）、`CONTEXT.md:164`（Agent 许可）、`CONTEXT.md:122`（本地能力映射）
- Parent：Issue #30；Blocked by：#59（已实施并集成，见「调查差异记录」第 1 条）、#60（已实施并集成）

## Global Constraints

以下为批准 Spec / Issue 的项目级约束，逐字引用，所有任务隐含遵守：

- 「只修改本地资源和策略属于 Mapping。修改 portable core 时创建 Agent Fork。Fork 保留来源、许可和修改说明，不再跟随上游升级。许可证允许时，Fork 可经 Tenant Review 或 Verified Publisher 加 Platform Review 再发布；上游 Publisher 不对 Fork 质量负责。」（agent-marketplace-domain-model.md §9）
- 「Agent Release 可以携带：……来源 Listing、Release、Fork lineage 和修改说明；Agent Manifest、许可证与变更说明。」（同上 §5）
- 「Manifest 至少声明：……来源、Fork lineage、许可证与支持语言；……Dependency Lock；变更、弃用、安全撤回和替代版本。」（同上 §6）
- 「Marketplace 搜索、Submission、Review、Adoption 和升级不在移动端执行。」（同上 §2——本计划零 TS 改动）
- 「退出不删除 Listing、Release、许可证或 lineage。安全撤回是更强的独立流程。」（同上 §9——本计划不删除任何历史行）
- 「Agent Fork：从一个 Agent Release 派生、保留完整来源与许可关系，但已修改其可移植核心并作为独立 Agent Definition 维护的 Agent；Fork 不再自动跟随来源 Listing 的升级建议，只有来源许可允许时才能作为新的 Release 重新提交审核。」避免：「只修改本地能力映射、对来源 Release 的原地编辑、隐藏来源或许可的复制品。」（CONTEXT.md:134）
- 「首版免费分发不等于没有许可，也不等于运行模型、Sandbox 或 Connector 免费。」（CONTEXT.md:164）
- 「Review 至少检查 Manifest 完整性、声明与内容一致性、……许可证、恶意内容、……Fork lineage……」（同上 §7——本计划提供审核输入，不自动替代人工审核）
- 边界（spec §16 与 Issue 拓扑）：升级建议（Upgrade Proposal）属 #61；Retire/End Adoption/Unlist/Deprecate 属 #63；安全撤回传播属 #64；Evaluation/Custody 属 #65。本计划不实现这些行为，也不给已发布 Release 加撤销机制。
- 安全约束（会话注入）：数据库查询所有外部输入使用参数绑定，不得拼接 SQL；本计划不新增服务端外呼请求；配置凭据只从环境变量或密钥服务读取，源码与测试不写入可用凭据字面量（本计划无新凭据）。
- 工作流约束：严格 RED→GREEN→REFACTOR；实现与已批准 Spec 冲突时升级而非静默重设计。

**Issue #62 验收标准原文（docs/plans/issue30-sweep/issues/issue-62.md）：**

1. 「Mapping 修改不误判为 Fork。」
2. 「禁止再分发时 Submission 由服务端拒绝。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

AC3 的本地可验证性说明：本计划最高稳定 Interface 证据 = `internal/router/routes_agent_fork_lineage_test.go` 的真实 HTTP 端到端测试——真实 sqlite 迁移流（`openTenantAgentMarketplaceHTTPTestDB`，含本计划新增迁移 000119）、真实 `CustomAgentService`/`AgentVersionService`/`AgentMarketplaceService`/`AgentAdoptionService`/`PublicMarketplaceService`、真实 freeze/submit/review/adopt/variant/publish 端点链路，仅以测试惯用的 RBAC 中间件注入租户/角色上下文（与已合并的 #58/#59/#60 router 测试同款）。无 blocked-env 项：本 Issue 不需要真机、外部凭据或移动端。服务层（Task 3）与仓储层（Task 2）测试如实标注为下层证据，不冒充 AC3。

## 调查差异记录（调查摘要 vs 代码现状，以代码为准）

1. **调查摘要称「依赖 #59/#60 的 Adoption 与跨 Tenant 机制均 absent，无端到端验证路径」——已过时。** 实测（本 worktree HEAD `d52270a0f`）：#59/#60 已实施并集成——`internal/types/agent_adoption_persistence.go`、`internal/types/interfaces/agent_adoption.go`、`internal/application/service/agent_adoption.go`、`internal/application/repository/agent_adoption.go`、`internal/handler/agent_adoption.go`、`internal/router/routes_agent_adoption.go`、`internal/application/service/public_marketplace.go`、`internal/router/routes_public_marketplace.go` 均在。调查中仍然成立的部分：Fork 实体/lineage 逻辑/再分发检查确实不存在（`internal/types/agent_marketplace.go:55-57` 与 `:102-105` 的 `LicenseID` 仅是声明字段，全仓无 `agent fork/lineage` 判定代码，无许可证注册表）。本计划以代码现状为 Consumes 事实源。
2. **波级问题 1（迁移 000114/000193 同号双迁移损坏）在干净 HEAD 上存在，由本计划 Task 0 负责去重重编（待办，非已完成）。** 本会话实测时间线：(a) 初次审查时工作区曾带一组未提交改名（mobile_device_app 000114→000118 / 000193→000197），当时基线 PASS；(b) 随后工作区被重置回干净 HEAD `d52270a0f`——`git status --porcelain -- migrations/` 输出为空，`ls migrations/sqlite/` 最大号为 `000117_code_deliveries`（versioned 最大 `000196_code_deliveries`），`000114_mobile_device_app.*` 与 `000114_public_agent_marketplace.*` 同号并存（versioned 同为 000193 双迁移）；(c) 在干净 HEAD 实跑基线命令 `go test ./internal/router/ -run 'TestTenantAgentAdoptionRoutesAndAuthorization|TestTenantAgentMarketplaceLifecycleAndAuthorization|TestPublicMarketplace' -count=1` → **全部 FAIL**，报错 `failed to open source, "file://…/migrations/sqlite": duplicate migration file: 000114_public_agent_marketplace.down.sql`；(d) 按本计划 Task 0 的重编程序在工作区执行改名并同步 5 个测试文件的字面引用后，同一命令复跑 → **PASS**（实测 `ok github.com/Tencent/WeKnora/internal/router 10.525s`）。重编对象选择依据：`internal/database/migration.go:123` 以文件存在性检查钉住 `migrations/sqlite/000114_public_agent_marketplace.up.sql`（且 `:33` 的 `sqliteAdoptionFKRelaxationMigrationVersion = 114`），故只能移动 mobile_device_app → **sqlite 000118 / versioned 000197**（当前两套流的最大空闲号）。本计划自己的新迁移占用 **sqlite 000119 / versioned 000198**；若执行时被并行计划占用，按既有先例（t60 60122179f 整体顺延）重编两套迁移的四个文件（up/down 成对），DDL 零变化，并同步更新 Task 1 中的文件名。执行环境若再次被重置（Task 0 Step 2 的守卫会检测到旧号文件仍在），Task 0 幂等重做同一重编。
3. **`buildLocalAgent` 不回填 StarterPrompts 的发布保真缺口（`internal/application/service/agent_adoption.go:489-523` 未设置 `Config.QuestionSuggestions`，而 `releasePayload` 读取它）。** 若 fork 判定直接与源 Release 原始载荷比较，「带 starters 的源 + 纯映射派生」会被误判为 Fork——恰是 AC1 要防止的误判。裁决：**不改 `buildLocalAgent`**（发布保真属 #59 范畴，不越界顺手改），fork 判定基线改为「源载荷经发布管线（`buildLocalAgent`→`PortablePayloadOf`）的重投影」，使纯映射派生的基线与提交载荷结构对齐；任何对可移植核心（含 starters）的事后修改仍正确判为 Fork。
4. **许可证注册表作用域为部署级（平台级），非租户级。** 原因：`TenantIntroducedReleaseEntity`（`internal/types/public_marketplace_persistence.go:109`）不携带发布者租户 ID，且 `introducedRelease` 综合视图把 `TenantID` 改写为采用方（`internal/application/repository/agent_adoption.go:382`），租户级 `(publisherTenantID, licenseID)` 查找无法落地；自托管部署本身即一个治理域，部署级注册表使跨租户引入的许可证查找天然成立（按 `license_id` 全局唯一）。
5. **`LicenseID` 追溯门控的范围收窄。** 既有行为：原始内容提交只要求 `license_id` 非空（`internal/modules/agentruntime/agent/experts/agent_release.go:153`），不查注册表；已合并的 #58/#59/#60 测试以 `"MIT"` 等未注册 id 提交原始内容且通过。本计划**不**追溯门控原始内容提交（保住存量行为与测试），注册表仅在派生 lineage 路径上 fail closed。默认值安全：未注册的来源许可 = 不允许再分发。
6. **Fork「不再自动跟随上游升级建议」无实现可违反。** 升级建议机制属 #61（absent）；本计划交付的 lineage 记录（`fork_source_*` 列 + Manifest lineage 段）是 #61 区分「升级 vs Fork」的数据基础。

## Consumes / Produces

**Consumes（均已实测存在于当前 HEAD）：**
- #58：`interfaces.AgentMarketplaceService.SubmitRelease(ctx, tenantID uint64, actorID, versionID string, input SubmitReleaseInput) (ReleaseSubmissionView, error)`（`internal/types/interfaces/agent_marketplace.go:22`）、`experts.BuildAgentReleaseBundle(version types.AgentVersionSnapshot, input types.ReleaseMetadata, lock types.DependencyLock) (types.AgentReleaseBundle, error)`（`internal/modules/agentruntime/agent/experts/agent_release.go:60`）。
- #59：Adoption 链（`Adopt/CreateVariant/UpdateCapabilityMapping/TestVariant/PublishVariant`，HTTP `POST /marketplace/tenant/adoptions` 等，Admin+）、`buildLocalAgent(variant *types.AgentAdoptionVariantEntity, payload types.AgentReleasePayload, manifest types.AgentReleaseManifest, mappings []types.AgentVariantCapabilityMappingEntity) *types.CustomAgent`（service 包内）、`AgentAdoptionVariantEntity.LocalAgentID/LocalAgentVersionID`。
- #60：`SubmitPublicRelease(ctx, tenantID uint64, actorID, sourceListingID, releaseID string)`、`PublicMarketplaceService.listings interfaces.AgentMarketplaceRepository` 字段、引入台账回退 `introducedRelease(tx *gorm.DB, tenantID uint64, releaseID string) (*types.AgentReleaseEntity, error)`（repository 包内，`internal/application/repository/agent_adoption.go:372`）、HTTP `POST /marketplace/public/verified-publishers`（SystemAdmin）。
- 测试基建：`openTenantAgentMarketplaceHTTPTestDB`、`newPublicMarketplaceTestApp`、`publicCall`、`publishAdoptionRelease`、`openAgentVersionServiceTestDB`、`marketplaceResolverFake`。

**Produces（供 #61/#63/#64/#65/#71 及后续批次消费）：**
- Go 仓储：`interfaces.AgentMarketplaceRepository` 新增 `FindDerivation(ctx, tenantID uint64, localAgentID string) (*types.AgentForkDerivation, error)`、`GetLicense(ctx, licenseID string) (*types.AgentLicenseEntity, error)`、`UpsertLicense(ctx, *types.AgentLicenseEntity) (*types.AgentLicenseEntity, error)`、`ListLicenses(ctx) ([]types.AgentLicenseEntity, error)`。
- Go 类型：`types.AgentForkDerivation{Variant AgentAdoptionVariantEntity; ListingID string; Release *AgentReleaseEntity}`、`types.AgentLicenseEntity`（表 `agent_licenses`，PK `id`）、`types.AgentReleaseLineage{SourceListingID, SourceReleaseID string; IsFork bool}`、`AgentReleaseManifest.Lineage *AgentReleaseLineage`（json `lineage,omitempty`）、提交/发布实体 5 个 lineage 列（`is_fork`/`fork_source_listing_id`/`fork_source_release_id`/`fork_notes`/`lineage_license_id`）。
- Go 服务：哨兵 `service.ErrReleaseRedistributionForbidden`（HTTP 409，消息点名许可证 id）、`service.ErrReleaseLineageUnavailable`（500 fail closed）、`service.ErrAgentLicenseInvalid`（400）；`AgentMarketplaceService` 接口新增 `RegisterLicense(ctx, actorID string, input LicenseInput) (types.AgentLicenseEntity, error)` / `ListLicenses(ctx) ([]types.AgentLicenseEntity, error)`；`interfaces.LicenseInput{ID, Name string; AllowsRedistribution bool}`。
- experts 导出：`experts.PortablePayloadOf(agent *types.CustomAgent) (types.AgentReleasePayload, error)`、`experts.BuildAgentReleaseBundleWithLineage(version, metadata, lock, lineage *types.AgentReleaseLineage) (types.AgentReleaseBundle, error)`。
- HTTP wire：`POST/GET /api/v1/marketplace/licenses`（Admin+）；submission/release DTO 新增 `is_fork`/`fork_source_listing_id`/`fork_source_release_id`/`fork_notes`/`lineage_license_id`；Manifest 文档新增可选 `lineage` 段。
- 给 #61：`agent_releases` 的 lineage 列 = 「该 Release 是否 Fork 及来源」的权威读模型；给 #71：本计划的 e2e 测试即 Fork 场景的发布证据。

## Review Focus

Spec 隐含但易咬人的五类输入/失效模式（每行后在所属任务以测试钉死）：

1. **Mapping 修改被误判为 Fork**（AC1 反面，最高风险）：fork 判定若直接对比源 Release 原始载荷，「带 starters 的源 + 纯映射派生」「本地绑定修改」都会误判。必须以发布管线重投影为基线，且本地绑定（KnowledgeBases/ModelID）结构性不入载荷。——Task 3 纯函数测试（`forkVerdict` 对映射形态配置返回 false）+ Task 5 `TestAgentForkLineageMappingEditsAreNotForks`（重映射→发布→提交→`is_fork=false`；改 systemPrompt→`is_fork=true` 同测试对照）。
2. **禁止再分发从另一条 Submission 通道漏过**：租户 lane 放行过的派生 Release，许可证注册表翻转后经 Public lane 升公共提交必须被拒（live 查询，不缓存发布时结论）；未注册许可证与已注册但禁止同等拒绝（fail closed）。——Task 5 `TestAgentForkLineageRedistributionForbiddenRejectsSubmission`（租户 lane：未注册/已注册禁止均 409）+ `TestAgentForkLineagePublicSubmissionLicenseGate`（公共 lane：放行→翻转→409→恢复→201）。
3. **lineage 被请求体伪造**：lineage 是治理事实，只能由服务端从 variant 台账推导；请求体出现任何 lineage 字段必须被 strict decode 拒绝（400），响应中的 lineage 只读。——Task 5 同测试断言带 `fork_source_release_id` 的 body → 400；Task 3 服务测试断言无 variant 的 agent 提交响应无任何 lineage 字段。
4. **原始（非派生）提交的存量回归**：无 variant 台账的提交必须与既有行为逐字节一致——Manifest 无 `lineage` 段、digest 不漂移、不查许可证注册表——否则存量 Release 的 digest/评审断言全部断裂。——Task 3 experts 测试（`BuildAgentReleaseBundleWithLineage(..., nil)` 字节等同 `BuildAgentReleaseBundle`）+ Task 5 断言原始提交响应 `is_fork=false` 且无 lineage 字段 + Task 5 步骤复跑既有 lifecycle 测试。
5. **损坏 lineage 数据静默放行**：源 Release bundle/manifest 不可解码、派生 variant 指向缺失 Release 时，提交必须拒绝（fail closed），绝不能以 `is_fork=false` 放行。——Task 3 服务测试（db 直改 bundle 字节 → `ErrReleaseLineageUnavailable`；variant 指向不存在 Release → 同哨兵）。

## 任务结构与文件地图

| # | 任务 | 主要交付 |
|---|---|---|
| 0 | 前置修复 + 前置核对 | 双迁移去重重编（mobile_device_app → 000118/000197）+ 5 个测试文件引用同步 + 迁移编号占用预检 + 基线复跑 |
| 1 | 双流迁移 + 持久化实体 | lineage 列 ×2 表 + `agent_licenses` 表 + 实体字段 + 迁移对齐测试 |
| 2 | lineage/许可证仓储 | `FindDerivation`（含引入回退）+ license 三方法 + 接口与 fake 扩展 |
| 3 | Manifest lineage + experts 导出 + fork 判定/许可证门 | `AgentReleaseLineage`、`PortablePayloadOf`、`BuildAgentReleaseBundleWithLineage`、服务层判定与门 + license 管理 |
| 4 | 审批传播 + 公共 lane 门 + HTTP wire | `ReviewAndPublishTx` 传播、公共门、DTO/端点/路由、错误映射 |
| 5 | AC1/AC2/AC3 端到端证据 | 真实 HTTP 三场景测试 + 全量验证 |

**Files:**
- Modify（Task 0 改名）: `migrations/sqlite/000114_mobile_device_app.up.sql` → `000118_mobile_device_app.up.sql`（down 同）；`migrations/versioned/000193_mobile_device_app.up.sql` → `000197_mobile_device_app.up.sql`（down 同）
- Modify（Task 0 引用同步）: `internal/handler/mobile_device_test.go`、`internal/application/repository/mobile_device_test.go`、`internal/application/repository/mobile_push_isolation_test.go`、`internal/modules/workbench/service/workbench/notification_app_policy_test.go`、`internal/application/repository/mobile_device_app_test.go`
- Create: `migrations/sqlite/000119_agent_fork_lineage.up.sql`、`migrations/sqlite/000119_agent_fork_lineage.down.sql`、`migrations/versioned/000198_agent_fork_lineage.up.sql`、`migrations/versioned/000198_agent_fork_lineage.down.sql`
- Create: `internal/types/agent_fork_lineage.go`
- Modify: `internal/types/agent_marketplace.go`（追加 `AgentReleaseLineage` 与 `AgentReleaseManifest.Lineage` 字段）
- Modify: `internal/types/agent_marketplace_persistence.go`（两实体各追加 5 个 lineage 列）
- Modify: `internal/types/interfaces/agent_marketplace.go`（仓储接口 +4 方法；服务接口 +2 方法 + `LicenseInput`）
- Create: `internal/application/repository/agent_marketplace_lineage.go`
- Modify: `internal/application/repository/agent_marketplace.go:145`（release 构造传播 lineage，1 处 struct literal）
- Modify: `internal/application/service/agent_marketplace_test.go`（`marketplaceRepoFake` 补 4 个 stub 方法）
- Modify: `internal/modules/agentruntime/agent/experts/agent_release.go`（追加 2 个导出函数）
- Create: `internal/application/service/agent_fork_lineage.go`
- Modify: `internal/application/service/agent_marketplace.go`（`SubmitRelease` 门 + lineage 记录 + 2 个 license 委托方法 + 哨兵）
- Modify: `internal/application/service/public_marketplace.go`（`SubmitPublicRelease` 公共门）
- Modify: `internal/handler/agent_marketplace.go`（2 个 DTO + license 端点 + 错误映射）
- Modify: `internal/handler/public_marketplace.go:185`（`publicMarketplaceClientError` 加 409 分支）
- Modify: `internal/router/routes_agent_marketplace.go`（license 路由）
- Create: `internal/application/repository/agent_fork_lineage_test.go`
- Create: `internal/application/service/agent_fork_lineage_test.go`
- Create: `internal/modules/agentruntime/agent/experts/agent_release_lineage_test.go`
- Create: `internal/router/routes_agent_fork_lineage_test.go`
- 零改动：`internal/container/`、TS 全部（contracts/mobile-core/apps/mobile）

---

### Task 0: 前置修复（双迁移去重重编）+ 前置核对（迁移编号 + 基线）

**Files:**
- Modify（改名）: `migrations/sqlite/000114_mobile_device_app.up.sql` → `migrations/sqlite/000118_mobile_device_app.up.sql`（down 同）
- Modify（改名）: `migrations/versioned/000193_mobile_device_app.up.sql` → `migrations/versioned/000197_mobile_device_app.up.sql`（down 同）
- Modify: `internal/handler/mobile_device_test.go:28-33`、`internal/application/repository/mobile_device_test.go:27-31`、`internal/application/repository/mobile_push_isolation_test.go:67`、`internal/modules/workbench/service/workbench/notification_app_policy_test.go:50`、`internal/application/repository/mobile_device_app_test.go:37,179,188-189`（迁移文件名字面引用随改名同步）

背景：干净 HEAD `d52270a0f` 上 `000114_mobile_device_app` 与 `000114_public_agent_marketplace`（sqlite）同号、`000193_mobile_device_app` 与 `000193_public_agent_marketplace`（versioned）同号，golang-migrate 装载直接失败（`duplicate migration file: 000114_public_agent_marketplace.down.sql`，本会话在干净 HEAD 实测全部基线测试 FAIL），而本计划 Task 1/3/5 的测试都要经 `openTenantAgentMarketplaceHTTPTestDB`/`openAgentVersionServiceTestDB` 装载迁移流——不去重本计划无法验证。移动 mobile_device_app 而非 public_agent_marketplace：`internal/database/migration.go:123` 钉住 `migrations/sqlite/000114_public_agent_marketplace.up.sql`（`:33` 的 `sqliteAdoptionFKRelaxationMigrationVersion = 114`），000118/000197 是两套流当前最大空闲号（sqlite 现最大 000117、versioned 现最大 000196）。本会话已按下列步骤在工作区实做并复跑基线 PASS（`ok github.com/Tencent/WeKnora/internal/router 10.525s`）；执行时若守卫检测到重编已完成则跳过。

- [ ] **Step 1: 确认 000119/000198 未被占用**

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep
ls migrations/sqlite/ | grep -E "^000119" ; ls migrations/versioned/ | grep -E "^000198"
```

Expected: 两个 grep 均无输出（000119/000198 空闲）。若有输出（被并行计划占用），把本计划四个迁移文件整体重编为下一个空闲号（sqlite 从 000119 起顺延、versioned 从 000198 起顺延，up/down 成对、两套注释互指保持一致），Task 1/Task 5 中出现的文件名同步替换。

- [ ] **Step 2: 双迁移去重重编（幂等：旧号文件不存在即跳过）**

```bash
test ! -f migrations/sqlite/000114_mobile_device_app.up.sql || {
  mv migrations/sqlite/000114_mobile_device_app.up.sql   migrations/sqlite/000118_mobile_device_app.up.sql
  mv migrations/sqlite/000114_mobile_device_app.down.sql migrations/sqlite/000118_mobile_device_app.down.sql
  mv migrations/versioned/000193_mobile_device_app.up.sql   migrations/versioned/000197_mobile_device_app.up.sql
  mv migrations/versioned/000193_mobile_device_app.down.sql migrations/versioned/000197_mobile_device_app.down.sql
  sed -i '' 's/同 sqlite 000114/同 sqlite 000118/' migrations/versioned/000197_mobile_device_app.up.sql
  sed -i '' -e 's/与 sqlite 000114 down 对称/与 sqlite 000118 down 对称/' -e 's/回滚卡死在 000193/回滚卡死在 000197/' migrations/versioned/000197_mobile_device_app.down.sql
  sed -i '' 's/000114_mobile_device_app/000118_mobile_device_app/g; s/000193_mobile_device_app/000197_mobile_device_app/g; s/stuck at 000114 (sqlite) \/ 000193 (PostgreSQL)/stuck at 000118 (sqlite) \/ 000197 (PostgreSQL)/' \
    internal/handler/mobile_device_test.go \
    internal/application/repository/mobile_device_test.go \
    internal/application/repository/mobile_push_isolation_test.go \
    internal/modules/workbench/service/workbench/notification_app_policy_test.go \
    internal/application/repository/mobile_device_app_test.go
}
```

（注：迁移 SQL 文件的注释自指按上示 sed 更新；5 个 Go 测试文件是迁移文件名的字面引用（`ReadFile` 直读，非 golang-migrate 装载），必须随改名同步，否则这些测试本身因找不到文件而挂。若执行环境禁 Bash 写源码，则对 5 个测试文件改用编辑工具做同一替换。）验证无残留：

```bash
rg -rn "000114_mobile_device_app|000193_mobile_device_app" internal/ migrations/ ; echo "exit=$?（期望输出为空、exit=1）"
```

- [ ] **Step 3: 复跑基线测试确认迁移轨道可装载**

```bash
go test ./internal/router/ -run 'TestTenantAgentAdoptionRoutesAndAuthorization|TestTenantAgentMarketplaceLifecycleAndAuthorization|TestPublicMarketplace' -count=1
```

Expected: **PASS**（本会话已在工作区完成 Step 2 重编后实测 `ok github.com/Tencent/WeKnora/internal/router 10.525s`）。仍 FAIL 且报 duplicate migration file 则说明环境与本计划假设不符，先停下来核对迁移目录再继续，不带病实施。

- [ ] **Step 4: 复跑被改名波及的设备测试（确认引用更新无损）**

```bash
go test ./internal/handler/ -run 'TestMobileDevice' -count=1 && go test ./internal/application/repository/ -run 'TestMobileDevice|TestMobilePush' -count=1
```

Expected: PASS（本会话实测两者均 `ok`；`internal/modules/workbench/service/workbench` 包存在一个与本计划无关的 pre-existing 失败 `TestNotificationDeliveryRejectsResolvedInteractionAfterClaim`——干净 HEAD stash 实测同样失败——该包内被波及的 4 个策略测试函数实测 PASS）。

- [ ] **Step 5: 提交（重编 + 引用同步一起入库，供编排层合并）**

```bash
git add migrations/ internal/handler/mobile_device_test.go internal/application/repository/mobile_device_test.go internal/application/repository/mobile_push_isolation_test.go internal/modules/workbench/service/workbench/notification_app_policy_test.go internal/application/repository/mobile_device_app_test.go
git commit -m "fix(migrations): renumber mobile_device_app to 000118/000197 to dedup with public_agent_marketplace (#62 Task 0)"
```

---

### Task 1: 双流迁移 + 持久化实体 + 迁移对齐测试

**Files:**
- Create: `migrations/sqlite/000119_agent_fork_lineage.up.sql` / `.down.sql`
- Create: `migrations/versioned/000198_agent_fork_lineage.up.sql` / `.down.sql`
- Create: `internal/types/agent_fork_lineage.go`
- Modify: `internal/types/agent_marketplace.go`（文件末尾追加）
- Modify: `internal/types/agent_marketplace_persistence.go`（两个 struct 各追加字段）
- Test: `internal/application/service/agent_fork_lineage_test.go`（本任务只写迁移对齐一个测试）

**Interfaces:**
- Produces: 实体列 `is_fork`/`fork_source_listing_id`/`fork_source_release_id`/`fork_notes`/`lineage_license_id`（`AgentReleaseSubmissionEntity` 与 `AgentReleaseEntity` 同列集）；`types.AgentLicenseEntity`；`types.AgentForkDerivation`；`types.AgentReleaseLineage`；`AgentReleaseManifest.Lineage *AgentReleaseLineage`（json `lineage,omitempty`）。Task 2-4 按名消费。

- [ ] **Step 1: 写失败测试（迁移对齐）**

创建 `internal/application/service/agent_fork_lineage_test.go`：

```go
package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAgentForkLineageMigrationProvidesColumnsAndLicenseTable pins the
// migration↔projection alignment for T32 (#62): the lineage columns exist
// on both marketplace tables and the license registry table is present
// after the full migration stream (same pattern as the workbench
// notifications alignment test from #34).
func TestAgentForkLineageMigrationProvidesColumnsAndLicenseTable(t *testing.T) {
	db := openAgentVersionServiceTestDB(t)
	var count int64
	require.NoError(t, db.Raw(
		`SELECT COUNT(*) FROM agent_release_submissions WHERE is_fork = 0 AND fork_source_listing_id = '' AND fork_source_release_id = '' AND fork_notes = '' AND lineage_license_id = ''`,
	).Scan(&count).Error)
	require.NoError(t, db.Raw(
		`SELECT COUNT(*) FROM agent_releases WHERE is_fork = 0 AND fork_source_listing_id = '' AND fork_source_release_id = '' AND fork_notes = '' AND lineage_license_id = ''`,
	).Scan(&count).Error)
	require.NoError(t, db.Exec(`SELECT 1 FROM agent_licenses`).Error)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/application/service/ -run TestAgentForkLineageMigrationProvidesColumnsAndLicenseTable -count=1`
Expected: FAIL（sqlite 报 `no such table: agent_licenses` / `no such column: is_fork`）

- [ ] **Step 3: 写四个迁移文件**

`migrations/sqlite/000119_agent_fork_lineage.up.sql`：

```sql
-- Agent Fork lineage + license registry (T32, #62): derived submissions and
-- releases record their source lineage and fork verdict; the source
-- license's redistribution flag gates re-submission (spec §5, §9).
-- SQLite twin of versioned migration 000198.
ALTER TABLE agent_release_submissions ADD COLUMN is_fork BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE agent_release_submissions ADD COLUMN fork_source_listing_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE agent_release_submissions ADD COLUMN fork_source_release_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE agent_release_submissions ADD COLUMN fork_notes TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_release_submissions ADD COLUMN lineage_license_id VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN is_fork BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE agent_releases ADD COLUMN fork_source_listing_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN fork_source_release_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN fork_notes TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN lineage_license_id VARCHAR(64) NOT NULL DEFAULT '';
CREATE TABLE agent_licenses (
 id VARCHAR(64) NOT NULL,
 name VARCHAR(255) NOT NULL DEFAULT '',
 allows_redistribution BOOLEAN NOT NULL DEFAULT 0,
 created_by VARCHAR(255) NOT NULL DEFAULT '',
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY (id)
);
```

`migrations/sqlite/000119_agent_fork_lineage.down.sql`：

```sql
DROP TABLE IF EXISTS agent_licenses;
ALTER TABLE agent_releases DROP COLUMN lineage_license_id;
ALTER TABLE agent_releases DROP COLUMN fork_notes;
ALTER TABLE agent_releases DROP COLUMN fork_source_release_id;
ALTER TABLE agent_releases DROP COLUMN fork_source_listing_id;
ALTER TABLE agent_releases DROP COLUMN is_fork;
ALTER TABLE agent_release_submissions DROP COLUMN lineage_license_id;
ALTER TABLE agent_release_submissions DROP COLUMN fork_notes;
ALTER TABLE agent_release_submissions DROP COLUMN fork_source_release_id;
ALTER TABLE agent_release_submissions DROP COLUMN fork_source_listing_id;
ALTER TABLE agent_release_submissions DROP COLUMN is_fork;
```

`migrations/versioned/000198_agent_fork_lineage.up.sql`：

```sql
-- T32 Agent Fork lineage + license registry (#62): derived submissions and
-- releases record their source lineage and fork verdict; the source
-- license's redistribution flag gates re-submission (spec §5, §9).
-- Versioned twin of sqlite migration 000119.
ALTER TABLE agent_release_submissions ADD COLUMN is_fork BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE agent_release_submissions ADD COLUMN fork_source_listing_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE agent_release_submissions ADD COLUMN fork_source_release_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE agent_release_submissions ADD COLUMN fork_notes TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_release_submissions ADD COLUMN lineage_license_id VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN is_fork BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE agent_releases ADD COLUMN fork_source_listing_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN fork_source_release_id VARCHAR(36) NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN fork_notes TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_releases ADD COLUMN lineage_license_id VARCHAR(64) NOT NULL DEFAULT '';
CREATE TABLE agent_licenses (
 id VARCHAR(64) NOT NULL,
 name VARCHAR(255) NOT NULL DEFAULT '',
 allows_redistribution BOOLEAN NOT NULL DEFAULT FALSE,
 created_by VARCHAR(255) NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY (id)
);
```

`migrations/versioned/000198_agent_fork_lineage.down.sql`：

```sql
DROP TABLE IF EXISTS agent_licenses;
ALTER TABLE agent_releases DROP COLUMN lineage_license_id;
ALTER TABLE agent_releases DROP COLUMN fork_notes;
ALTER TABLE agent_releases DROP COLUMN fork_source_release_id;
ALTER TABLE agent_releases DROP COLUMN fork_source_listing_id;
ALTER TABLE agent_releases DROP COLUMN is_fork;
ALTER TABLE agent_release_submissions DROP COLUMN lineage_license_id;
ALTER TABLE agent_release_submissions DROP COLUMN fork_notes;
ALTER TABLE agent_release_submissions DROP COLUMN fork_source_release_id;
ALTER TABLE agent_release_submissions DROP COLUMN fork_source_listing_id;
ALTER TABLE agent_release_submissions DROP COLUMN is_fork;
```

- [ ] **Step 4: 追加持久化实体**

`internal/types/agent_fork_lineage.go`（新文件）：

```go
package types

import "time"

// Agent Fork lineage and license registry value types (T32, Ticket #62).
//
// spec §9: 只修改本地资源和策略属于 Mapping；修改 portable core 时创建
// Agent Fork。Fork 保留来源、许可和修改说明；许可证允许时才能作为新的
// Release 重新提交审核。The lineage columns live on the submission/release
// rows; the license registry is deployment-scoped (a self-hosted
// deployment is one governance domain — see plan-t62 差异记录 #4).

// AgentForkDerivation is the adoption lineage of a variant-published local
// agent: the latest AgentAdoptionVariantEntity whose LocalAgentID matches,
// its Adoption's Listing and the pinned source Release (local row first,
// the #60 introduction ledger as fallback). Release is nil only when the
// pinned release cannot be resolved — callers must fail closed.
type AgentForkDerivation struct {
	Variant   AgentAdoptionVariantEntity
	ListingID string
	Release   *AgentReleaseEntity
}

// AgentLicenseEntity is one deployment license term. ID is the license_id
// slug ReleaseMetadata declares; AllowsRedistribution is the flag the
// re-submission gate consults. Re-registering (upsert) updates the flags —
// that is how a license flip propagates to later submissions.
type AgentLicenseEntity struct {
	ID                   string `gorm:"type:varchar(64);primaryKey"`
	Name                 string `gorm:"type:varchar(255);not null;default:''"`
	AllowsRedistribution bool   `gorm:"not null;default:false"`
	CreatedBy            string `gorm:"type:varchar(255);not null;default:''"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (AgentLicenseEntity) TableName() string { return "agent_licenses" }
```

`internal/types/agent_marketplace_persistence.go`：在 `AgentReleaseSubmissionEntity` 的 `Status` 字段之后、`CreatedAt` 之前插入：

```go
	// Fork lineage (T32 #62): set when the submitted agent derives from an
	// adopted Release (spec §5/§9). Zero-value rows (original content or
	// pre-#62 history) carry no lineage and are never gated.
	IsFork              bool   `gorm:"not null;default:false"`
	ForkSourceListingID string `gorm:"type:varchar(36);not null;default:''"`
	ForkSourceReleaseID string `gorm:"type:varchar(36);not null;default:''"`
	ForkNotes           string `gorm:"type:text;not null;default:''"`
	LineageLicenseID    string `gorm:"type:varchar(64);not null;default:''"`
```

在 `AgentReleaseEntity` 的 `PublishedBy` 字段之前插入同一字段块（注释相同，逐字复制）。

- [ ] **Step 5: 追加 Manifest lineage 类型**

`internal/types/agent_marketplace.go` 文件末尾追加：

```go
// AgentReleaseLineage records, inside the Release digest boundary, where a
// derived Release came from and whether the portable core was modified
// (spec §5「来源 Listing、Release、Fork lineage 和修改说明」, §6, §9).
// Original (non-derived) Releases carry no lineage section at all.
type AgentReleaseLineage struct {
	// SourceListingID is the adopted Listing the derivation started from.
	SourceListingID string `json:"source_listing_id"`
	// SourceReleaseID is the pinned source Release.
	SourceReleaseID string `json:"source_release_id"`
	// IsFork reports the fork verdict: true exactly when the submitted
	// portable core differs from the source Release's publish-pipeline
	// projection (spec §9「修改 portable core 时创建 Agent Fork」). Local
	// capability mapping changes never flip this flag.
	IsFork bool `json:"is_fork"`
}
```

并把 `AgentReleaseManifest` 的 `Source` 字段之后追加：

```go
	// Lineage is set exactly for derived Releases; omitted (and therefore
	// digest-neutral) for original Releases, so pre-#62 submission bytes
	// never change.
	Lineage *AgentReleaseLineage `json:"lineage,omitempty"`
```

- [ ] **Step 6: 运行测试确认通过**

Run: `go test ./internal/application/service/ -run TestAgentForkLineageMigrationProvidesColumnsAndLicenseTable -count=1` 与 `go build ./...`
Expected: PASS + 构建通过

- [ ] **Step 7: 提交**

```bash
git add migrations/sqlite/000119_agent_fork_lineage.up.sql migrations/sqlite/000119_agent_fork_lineage.down.sql migrations/versioned/000198_agent_fork_lineage.up.sql migrations/versioned/000198_agent_fork_lineage.down.sql internal/types/agent_fork_lineage.go internal/types/agent_marketplace.go internal/types/agent_marketplace_persistence.go internal/application/service/agent_fork_lineage_test.go
git commit -m "feat(marketplace): fork lineage columns, license registry table and manifest lineage type (#62)"
```

---

### Task 2: lineage/许可证仓储（FindDerivation + license 三方法）

**Files:**
- Create: `internal/application/repository/agent_marketplace_lineage.go`
- Modify: `internal/types/interfaces/agent_marketplace.go`（`AgentMarketplaceRepository` 接口 +4 方法）
- Modify: `internal/application/service/agent_marketplace_test.go`（`marketplaceRepoFake` 补 4 个 stub）
- Test: `internal/application/repository/agent_fork_lineage_test.go`

**Interfaces:**
- Consumes: Task 1 的 `types.AgentForkDerivation`/`types.AgentLicenseEntity`；repository 包内既有 `introducedRelease(tx *gorm.DB, tenantID uint64, releaseID string) (*types.AgentReleaseEntity, error)`（`internal/application/repository/agent_adoption.go:372`，包级函数，可直接调用）。
- Produces: `interfaces.AgentMarketplaceRepository` 新增四方法（签名见下）；Task 3 的服务层按 `s.repo.FindDerivation/GetLicense/UpsertLicense/ListLicenses` 消费。

- [ ] **Step 1: 写失败测试**

创建 `internal/application/repository/agent_fork_lineage_test.go`：

```go
package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/driver/sqlite"

	"github.com/Tencent/WeKnora/internal/types"
)

// openForkLineageDB follows the fast in-memory convention of
// openAdoptionVariantDB (agent_adoption_test.go): AutoMigrate over the
// entities involved, plus the adoption scope unique index. The migration
// stream itself is exercised by the service/router tests.
func openForkLineageDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&types.AgentAdoptionEntity{}, &types.AgentAdoptionVariantEntity{},
		&types.AgentMarketplaceListingEntity{}, &types.AgentReleaseEntity{},
		&types.AgentLicenseEntity{}, &types.TenantIntroducedReleaseEntity{},
	))
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func seedDerivation(t *testing.T, db *gorm.DB, releaseID string) {
	t.Helper()
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{
		ID: "adopt-1", TenantID: 1, ListingID: "listing-1", AcceptedReleaseID: releaseID, State: "active",
	}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{
		ID: "variant-1", TenantID: 1, AdoptionID: "adopt-1", ReleaseID: releaseID,
		Name: "Sales Assistant", State: "published", LocalAgentID: "agent-local",
	}).Error)
}

func TestFindDerivationResolvesVariantAdoptionAndRelease(t *testing.T) {
	db := openForkLineageDB(t)
	require.NoError(t, db.Create(&types.AgentReleaseEntity{
		ID: "rel-1", TenantID: 1, ListingID: "listing-1", SubmissionID: "sub-1",
		AgentVersionID: "version-src", SourceAgentID: "agent-src", ReleaseNumber: 1,
		SemanticVersion: "1.0.0", BundleDigest: "d", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("{}"),
	}).Error)
	seedDerivation(t, db, "rel-1")
	repo := NewAgentMarketplaceRepository(db)

	derivation, err := repo.FindDerivation(context.Background(), 1, "agent-local")
	require.NoError(t, err)
	require.NotNil(t, derivation)
	require.Equal(t, "variant-1", derivation.Variant.ID)
	require.Equal(t, "listing-1", derivation.ListingID)
	require.NotNil(t, derivation.Release)
	require.Equal(t, "rel-1", derivation.Release.ID)

	// 未派生（无 variant 台账）的 agent 必须得到 nil, nil——原始内容没有 lineage。
	none, err := repo.FindDerivation(context.Background(), 1, "agent-original")
	require.NoError(t, err)
	require.Nil(t, none)

	// 跨租户读取必须读不到他租户的 variant。
	cross, err := repo.FindDerivation(context.Background(), 2, "agent-local")
	require.NoError(t, err)
	require.Nil(t, cross)
}

func TestFindDerivationFallsBackToIntroducedRelease(t *testing.T) {
	db := openForkLineageDB(t)
	require.NoError(t, db.Create(&types.TenantIntroducedReleaseEntity{
		ID: "public-rel-1", TenantID: 1, PublicListingID: "public-listing-1", PublicReleaseID: "upstream-1",
		DisplayName: "Public helper", SemanticVersion: "2.0.0", BundleDigest: "pd",
		ManifestJSON: `{"license_id":"community"}`, DependencyLockJSON: "{}", Bundle: []byte(`{"payload":{}}`),
	}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{
		ID: "adopt-p", TenantID: 1, ListingID: "public-listing-1", AcceptedReleaseID: "public-rel-1", State: "active",
	}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{
		ID: "variant-p", TenantID: 1, AdoptionID: "adopt-p", ReleaseID: "public-rel-1",
		Name: "Adopted helper", State: "published", LocalAgentID: "agent-adopted",
	}).Error)
	repo := NewAgentMarketplaceRepository(db)

	derivation, err := repo.FindDerivation(context.Background(), 1, "agent-adopted")
	require.NoError(t, err)
	require.NotNil(t, derivation)
	require.Equal(t, "public-listing-1", derivation.ListingID, "引入台账回退解析出公共 Listing id")
	require.NotNil(t, derivation.Release)
	require.Equal(t, "public-rel-1", derivation.Release.ID)
}

func TestLicenseUpsertGetAndList(t *testing.T) {
	db := openForkLineageDB(t)
	repo := NewAgentMarketplaceRepository(db)
	ctx := context.Background()

	created, err := repo.UpsertLicense(ctx, &types.AgentLicenseEntity{
		ID: "MIT", Name: "MIT License", AllowsRedistribution: true, CreatedBy: "admin",
	})
	require.NoError(t, err)
	require.True(t, created.AllowsRedistribution)

	// 翻转：重新注册即更新（许可证可收紧，后续提交 live 生效）。
	updated, err := repo.UpsertLicense(ctx, &types.AgentLicenseEntity{
		ID: "MIT", Name: "MIT License", AllowsRedistribution: false, CreatedBy: "admin",
	})
	require.NoError(t, err)
	require.False(t, updated.AllowsRedistribution)

	got, err := repo.GetLicense(ctx, "MIT")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.False(t, got.AllowsRedistribution)

	missing, err := repo.GetLicense(ctx, "ghost")
	require.NoError(t, err)
	require.Nil(t, missing, "未注册许可证返回 nil，由服务层 fail closed")

	rows, err := repo.ListLicenses(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "MIT", rows[0].ID)
}
```

注意：import 分组顺序按仓库惯例调整（`gorm.io/driver/sqlite` 与 `gorm.io/gorm` 同组）。

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/application/repository/ -run 'TestFindDerivation|TestLicenseUpsertGetAndList' -count=1`
Expected: FAIL（编译错误：`repo.FindDerivation undefined` / `repo.UpsertLicense undefined`）

- [ ] **Step 3: 扩展接口与 fake**

`internal/types/interfaces/agent_marketplace.go`：在 `AgentMarketplaceRepository` 接口的 `GetListing(...)` 之前插入：

```go
	// FindDerivation resolves the adoption lineage of a variant-published
	// local agent (T32 #62): nil when the agent is not derived from an
	// adopted Release — original content has no lineage.
	FindDerivation(context.Context, uint64, string) (*types.AgentForkDerivation, error)
	// GetLicense returns the deployment license row, nil when unregistered.
	GetLicense(context.Context, string) (*types.AgentLicenseEntity, error)
	UpsertLicense(context.Context, *types.AgentLicenseEntity) (*types.AgentLicenseEntity, error)
	ListLicenses(context.Context) ([]types.AgentLicenseEntity, error)
```

`internal/application/service/agent_marketplace_test.go`：在 `marketplaceRepoFake` 的最后一个方法之后追加：

```go
func (f *marketplaceRepoFake) FindDerivation(context.Context, uint64, string) (*types.AgentForkDerivation, error) {
	return nil, nil
}

func (f *marketplaceRepoFake) GetLicense(context.Context, string) (*types.AgentLicenseEntity, error) {
	return nil, nil
}

func (f *marketplaceRepoFake) UpsertLicense(_ context.Context, license *types.AgentLicenseEntity) (*types.AgentLicenseEntity, error) {
	return license, nil
}

func (f *marketplaceRepoFake) ListLicenses(context.Context) ([]types.AgentLicenseEntity, error) {
	return nil, nil
}
```

- [ ] **Step 4: 写仓储实现**

创建 `internal/application/repository/agent_marketplace_lineage.go`：

```go
package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Tencent/WeKnora/internal/types"
)

// Fork lineage + license registry SQL for the Marketplace repository
// (T32 #62). Methods live on agentMarketplaceRepository (declared in
// agent_marketplace.go) but sit in this file so the lineage feature stays
// one reviewable unit. All reads and writes are parameter-bound.

// FindDerivation resolves the adoption lineage of a variant-published
// local agent: the latest variant whose local_agent_id matches, its
// Adoption's Listing and the pinned source Release (tenant-local row
// first, the #60 introduction-ledger synthesis second). Returns
// (nil, nil) when the agent is not derived — original content has no
// lineage and is never gated.
func (r *agentMarketplaceRepository) FindDerivation(ctx context.Context, tenantID uint64, localAgentID string) (*types.AgentForkDerivation, error) {
	localAgentID = strings.TrimSpace(localAgentID)
	if tenantID == 0 || localAgentID == "" {
		return nil, nil
	}
	var variant types.AgentAdoptionVariantEntity
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND local_agent_id = ?", tenantID, localAgentID).
		Order("updated_at DESC, id DESC").First(&variant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var adoption types.AgentAdoptionEntity
	err = r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, variant.AdoptionID).First(&adoption).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// The governance rows were archived away (#63 owns that flow):
		// without the Adoption there is no lineage left to preserve.
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	release, err := r.getDerivationRelease(ctx, tenantID, variant.ReleaseID)
	if err != nil {
		return nil, err
	}
	return &types.AgentForkDerivation{Variant: variant, ListingID: adoption.ListingID, Release: release}, nil
}

// getDerivationRelease reads the pinned source Release: the tenant-local
// row first, the introduction-ledger fallback second (the same synthesis
// the #59 adoption chain consumes via GetRelease).
func (r *agentMarketplaceRepository) getDerivationRelease(ctx context.Context, tenantID uint64, releaseID string) (*types.AgentReleaseEntity, error) {
	var row types.AgentReleaseEntity
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(releaseID)).First(&row).Error
	if err == nil {
		return &row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return introducedRelease(r.db.WithContext(ctx), tenantID, releaseID)
}

// UpsertLicense registers (or re-registers) one deployment license term.
// The upsert is how a license flip propagates to later submissions.
func (r *agentMarketplaceRepository) UpsertLicense(ctx context.Context, license *types.AgentLicenseEntity) (*types.AgentLicenseEntity, error) {
	if license == nil || strings.TrimSpace(license.ID) == "" {
		return nil, ErrAgentLicenseIDRequired
	}
	now := time.Now().UTC()
	license.CreatedAt = now
	license.UpdatedAt = now
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "allows_redistribution", "created_by", "updated_at"}),
		}).
		Create(license).Error
	if err != nil {
		return nil, err
	}
	return license, nil
}

// GetLicense returns the license row, nil when unregistered (the service
// layer decides that fail closed).
func (r *agentMarketplaceRepository) GetLicense(ctx context.Context, licenseID string) (*types.AgentLicenseEntity, error) {
	var row types.AgentLicenseEntity
	err := r.db.WithContext(ctx).Where("id = ?", strings.TrimSpace(licenseID)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// ListLicenses returns the deployment license registry ordered by id.
func (r *agentMarketplaceRepository) ListLicenses(ctx context.Context) ([]types.AgentLicenseEntity, error) {
	rows := make([]types.AgentLicenseEntity, 0)
	err := r.db.WithContext(ctx).Order("id ASC").Find(&rows).Error
	return rows, err
}
```

同文件顶部已列出 import；`ErrAgentLicenseIDRequired` 定义在 `internal/application/repository/agent_marketplace.go` 既有 `var (...)` 块中追加：

```go
	ErrAgentLicenseIDRequired = errors.New("agent marketplace license id is required")
```

（先运行 `goimports -w internal/application/repository/agent_marketplace_lineage.go` 或按编译器提示整理 import。）

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/application/repository/ -run 'TestFindDerivation|TestLicenseUpsertGetAndList' -count=1` 与 `go build ./...`
Expected: PASS + 构建通过

- [ ] **Step 6: 提交**

```bash
git add internal/application/repository/agent_marketplace_lineage.go internal/application/repository/agent_marketplace.go internal/types/interfaces/agent_marketplace.go internal/application/service/agent_marketplace_test.go internal/application/repository/agent_fork_lineage_test.go
git commit -m "feat(marketplace): fork derivation lookup and license registry repository (#62)"
```

---

### Task 3: Manifest lineage + experts 导出 + 服务层 fork 判定/许可证门

**Files:**
- Modify: `internal/modules/agentruntime/agent/experts/agent_release.go`（追加 2 个导出函数）
- Create: `internal/application/service/agent_fork_lineage.go`
- Modify: `internal/application/service/agent_marketplace.go`（`SubmitRelease` 门 + lineage 记录 + 2 个委托方法）
- Modify: `internal/types/interfaces/agent_marketplace.go`（`AgentMarketplaceService` +2 方法 + `LicenseInput`）
- Test: `internal/modules/agentruntime/agent/experts/agent_release_lineage_test.go`（新）
- Test: `internal/application/service/agent_fork_lineage_test.go`（追加）

**Interfaces:**
- Consumes: Task 2 的 `FindDerivation/GetLicense/UpsertLicense/ListLicenses`；#59 的 `buildLocalAgent`（service 包内）；`experts.BuildAgentReleaseBundle`。
- Produces: `experts.PortablePayloadOf(agent *types.CustomAgent) (types.AgentReleasePayload, error)`、`experts.BuildAgentReleaseBundleWithLineage(version types.AgentVersionSnapshot, input types.ReleaseMetadata, lock types.DependencyLock, lineage *types.AgentReleaseLineage) (types.AgentReleaseBundle, error)`；哨兵 `ErrReleaseRedistributionForbidden`/`ErrReleaseLineageUnavailable`/`ErrAgentLicenseInvalid`；`interfaces.LicenseInput`；`AgentMarketplaceService.RegisterLicense/ListLicenses`。Task 4/5 按名消费。

- [ ] **Step 1: 写失败测试（experts 层）**

创建 `internal/modules/agentruntime/agent/experts/agent_release_lineage_test.go`：

```go
package experts

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/types"
)

func lineageTestVersion(prompt string) types.AgentVersionSnapshot {
	return types.AgentVersionSnapshot{
		AgentVersionView: types.AgentVersionView{ID: "version-1"},
		Agent: &types.CustomAgent{
			ID: "agent-1", Name: "Helper",
			Config: types.CustomAgentConfig{AgentMode: "quick-answer", SystemPrompt: prompt},
		},
	}
}

func lineageTestMetadata() types.ReleaseMetadata {
	return types.ReleaseMetadata{
		SemanticVersion: "1.0.1", DisplayName: "Helper", Summary: "A helper",
		SupportedLanguages: []string{"en"}, UseCases: []string{"support"},
		MinimumWeKnoraCapability: "1", LicenseID: "MIT",
	}
}

// AC1 的结构性前提：本地映射绑定（知识库/模型）绝不进入可移植载荷，
// 因此纯映射修改在载荷层面与基线字节相等。
func TestPortablePayloadOfProjectsPortableCoreOnly(t *testing.T) {
	version := lineageTestVersion("Be portable.")
	version.Agent.Config.KnowledgeBases = []string{"kb-sales"}
	version.Agent.Config.ModelID = "gpt-x"

	payload, err := PortablePayloadOf(version.Agent)
	require.NoError(t, err)
	require.Equal(t, "quick-answer", payload.AgentMode)
	require.Equal(t, "Be portable.", payload.SystemPrompt)
	encoded, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "kb-sales", "本地映射绑定绝不进入可移植载荷")
	require.NotContains(t, string(encoded), "gpt-x")
}

// 存量回归卫兵：lineage=nil 时字节与既有 BuildAgentReleaseBundle 完全
// 一致——原始内容的 digest 不漂移（Review Focus 4）。
func TestBuildAgentReleaseBundleWithLineageNilKeepsLegacyBytes(t *testing.T) {
	version := lineageTestVersion("Be portable.")
	lock := types.DependencyLock{}
	legacy, err := BuildAgentReleaseBundle(version, lineageTestMetadata(), lock)
	require.NoError(t, err)
	neutral, err := BuildAgentReleaseBundleWithLineage(version, lineageTestMetadata(), lock, nil)
	require.NoError(t, err)
	require.Equal(t, legacy.Bytes, neutral.Bytes)
	require.Equal(t, legacy.SHA256, neutral.SHA256)
}

// Fork 的 lineage 段进入 Manifest（digest 边界内），载荷可 round-trip 读回。
func TestBuildAgentReleaseBundleWithLineageEmbedsLineageInManifest(t *testing.T) {
	version := lineageTestVersion("Be portable, but sharper.")
	lineage := &types.AgentReleaseLineage{SourceListingID: "listing-1", SourceReleaseID: "rel-1", IsFork: true}
	bundle, err := BuildAgentReleaseBundleWithLineage(version, lineageTestMetadata(), types.DependencyLock{}, lineage)
	require.NoError(t, err)

	var manifest types.AgentReleaseManifest
	require.NoError(t, json.Unmarshal(bundle.Manifest, &manifest))
	require.NotNil(t, manifest.Lineage)
	require.Equal(t, "listing-1", manifest.Lineage.SourceListingID)
	require.Equal(t, "rel-1", manifest.Lineage.SourceReleaseID)
	require.True(t, manifest.Lineage.IsFork)

	// 无 lineage 的 manifest 序列化不得出现 "lineage" 键。
	neutral, err := BuildAgentReleaseBundleWithLineage(version, lineageTestMetadata(), types.DependencyLock{}, nil)
	require.NoError(t, err)
	require.NotContains(t, string(neutral.Manifest), `"lineage"`)
}
```

注：`AgentVersionView` 的字段名以 `internal/types/agent_version.go` 为准（本计划实测含 `ID`；若字段为小写未导出组合，按实际类型补齐字面量）。

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/modules/agentruntime/agent/experts/ -run 'TestPortablePayloadOf|TestBuildAgentReleaseBundleWithLineage' -count=1`
Expected: FAIL（`PortablePayloadOf`/`BuildAgentReleaseBundleWithLineage` undefined）

- [ ] **Step 3: 实现 experts 两个导出函数**

`internal/modules/agentruntime/agent/experts/agent_release.go`：把既有 `BuildAgentReleaseBundle` 函数体重命名为内部核心并追加两个导出函数（既有函数签名不变，直接在文件中修改为如下三个函数；`BuildAgentReleaseBundle` 保持原签名成为委托）：

```go
// BuildAgentReleaseBundle exports the immutable AgentVersion snapshot as a
// canonical sanitized Release bundle (T28 behavior, unchanged). Original
// (non-derived) submissions take this path: the manifest carries no
// lineage section and the bytes are identical to the pre-#62 exporter.
func BuildAgentReleaseBundle(
	version types.AgentVersionSnapshot,
	input types.ReleaseMetadata,
	lock types.DependencyLock,
) (types.AgentReleaseBundle, error) {
	return buildAgentReleaseBundle(version, input, lock, nil)
}

// BuildAgentReleaseBundleWithLineage is the lineage-aware exporter
// (T32 #62): lineage is embedded in the Manifest — inside the digest
// boundary, so a fork's bytes differ from both the source release and the
// lineage-free projection. A nil lineage produces exactly the legacy bytes.
func BuildAgentReleaseBundleWithLineage(
	version types.AgentVersionSnapshot,
	input types.ReleaseMetadata,
	lock types.DependencyLock,
	lineage *types.AgentReleaseLineage,
) (types.AgentReleaseBundle, error) {
	return buildAgentReleaseBundle(version, input, lock, lineage)
}

// buildAgentReleaseBundle is the shared canonical exporter; lineage is nil
// for original content.
func buildAgentReleaseBundle(
	version types.AgentVersionSnapshot,
	input types.ReleaseMetadata,
	lock types.DependencyLock,
	lineage *types.AgentReleaseLineage,
) (types.AgentReleaseBundle, error) {
	if version.Agent == nil {
		return types.AgentReleaseBundle{}, fmt.Errorf("experts: release bundle: the agent snapshot is nil")
	}
	manifest, err := releaseManifest(version, input)
	if err != nil {
		return types.AgentReleaseBundle{}, err
	}
	manifest.Lineage = lineage
	canonicalLock, err := canonicalDependencyLock(lock)
	if err != nil {
		return types.AgentReleaseBundle{}, err
	}
	payload, err := releasePayload(version.Agent)
	if err != nil {
		return types.AgentReleaseBundle{}, err
	}

	raw, err := canonicalReleaseJSON(agentReleaseEnvelope{
		Payload:        payload,
		Manifest:       manifest,
		DependencyLock: canonicalLock,
	})
	if err != nil {
		return types.AgentReleaseBundle{}, fmt.Errorf("experts: release bundle: serialize the canonical envelope: %w", err)
	}
	sum := sha256.Sum256(raw)
	return types.AgentReleaseBundle{
		Manifest: manifest,
		Lock:     canonicalLock,
		Payload:  payload,
		Bytes:    raw,
		SHA256:   hex.EncodeToString(sum[:]),
	}, nil
}

// PortablePayloadOf projects a live agent onto the portable allow-list —
// the exact projection a Release bundle's payload uses (T32 #62). The fork
// verdict compares the submitted snapshot's payload against the source
// Release's publish-pipeline baseline through this function.
func PortablePayloadOf(agent *types.CustomAgent) (types.AgentReleasePayload, error) {
	return releasePayload(agent)
}
```

（原 `BuildAgentReleaseBundle` 函数体被 `buildAgentReleaseBundle` 取代，注释头保留在 `buildAgentReleaseBundle` 上方；实现时保持包内其它引用不变。）

- [ ] **Step 4: 运行 experts 测试确认通过 + 全包回归**

Run: `go test ./internal/modules/agentruntime/agent/experts/ -count=1`
Expected: PASS（新测试 + 既有 agent_release_test.go 全绿——digest 零回归的直接证据）

- [ ] **Step 5: 写失败测试（服务层）**

在 `internal/application/service/agent_fork_lineage_test.go` 追加（文件已有 Task 1 的迁移对齐测试）：

```go
// ---------- 下层证据（服务层）：fork 判定与再分发门 ----------

func lineageSnapshot(t *testing.T, id, agentID, prompt string, localBindings bool) (versionID, snapshotJSON string) {
	t.Helper()
	config := map[string]any{"agent_mode": "quick-answer", "system_prompt": prompt}
	if localBindings {
		config["knowledge_bases"] = []string{"kb-sales"}
		config["model_id"] = "gpt-x"
	}
	raw, err := json.Marshal(map[string]any{"id": agentID, "name": "Sales Assistant", "config": config})
	require.NoError(t, err)
	return id, string(raw)
}

func seedFrozenAgentVersion(t *testing.T, db *gorm.DB, id, agentID, snapshot string) {
	t.Helper()
	sum := sha256.Sum256([]byte(snapshot))
	require.NoError(t, db.Exec(
		`INSERT INTO agent_versions (id, tenant_id, agent_id, version_number, snapshot, source_sha256, frozen_by) VALUES (?, 1, ?, 1, ?, ?, 'admin')`,
		id, agentID, snapshot, hex.EncodeToString(sum[:]),
	).Error)
}

// seedLineageSource publishes a REAL tenant release (listing+submission+
// release rows via the real repository, mirroring the #60 service-test
// seeding) whose manifest declares licenseID and whose payload carries
// systemPrompt. Returns the listing and release ids.
func seedLineageSource(t *testing.T, db *gorm.DB, suffix, licenseID, systemPrompt string) (string, string) {
	t.Helper()
	repo := NewAgentMarketplaceRepository(db)
	payload := map[string]any{"agent_mode": "quick-answer", "system_prompt": systemPrompt, "allowed_tools": []string{}}
	manifest := map[string]any{
		"semantic_version": "1.0.0", "display_name": "Source " + suffix, "summary": "Source",
		"supported_languages": []string{"en"}, "use_cases": []string{"support"},
		"minimum_weknora_capability": "1", "license_id": licenseID,
		"source": map[string]any{"agent_version_id": "version-src-" + suffix, "version_number": 1, "source_sha256": "sha"},
	}
	payloadJSON, err := json.Marshal(payload)
	require.NoError(t, err)
	manifestJSON, err := json.Marshal(manifest)
	require.NoError(t, err)
	bundle, err := json.Marshal(map[string]any{
		"payload": json.RawMessage(payloadJSON), "manifest": json.RawMessage(manifestJSON),
		"dependency_lock": map[string]any{"dependencies": []any{}},
	})
	require.NoError(t, err)
	sum := sha256.Sum256(bundle)

	seedFrozenAgentVersion(t, db, "version-src-"+suffix, "agent-src-"+suffix,
		`{"id":"agent-src-`+suffix+`","name":"Source","config":{"agent_mode":"quick-answer","system_prompt":"`+systemPrompt+`"}}`)
	submission, err := repo.CreateSubmission(context.Background(),
		&types.AgentMarketplaceListingEntity{TenantID: 1, SourceAgentID: "agent-src-" + suffix, DisplayName: "Source " + suffix, Summary: "Source", State: "listed"},
		&types.AgentReleaseSubmissionEntity{
			TenantID: 1, AgentVersionID: "version-src-" + suffix, SourceAgentID: "agent-src-" + suffix,
			AuthorID: "admin", SemanticVersion: "1.0.0", BundleDigest: hex.EncodeToString(sum[:]),
			ManifestJSON: string(manifestJSON), DependencyLockJSON: `{"dependencies":[]}`, Bundle: bundle, Status: "submitted",
		})
	require.NoError(t, err)
	_, release, err := repo.ReviewAndPublishTx(context.Background(), 1, "", submission.ID, submission.BundleDigest,
		types.AgentReleaseReviewDecision{ReviewerID: "reviewer", Decision: "approved"})
	require.NoError(t, err)
	return submission.ListingID, release.ID
}

func newLineageMarketplaceService(t *testing.T, db *gorm.DB) *AgentMarketplaceService {
	t.Helper()
	customAgents := NewCustomAgentService(repository.NewCustomAgentRepository(db), nil, nil, nil, nil, nil, nil)
	versions := NewAgentVersionService(customAgents, repository.NewAgentVersionRepository(db))
	return NewAgentMarketplaceService(versions, marketplaceResolverFake{}, repository.NewAgentMarketplaceRepository(db), t.TempDir())
}

func seedPublishedVariantAgent(t *testing.T, db *gorm.DB, listingID, releaseID, variantID, localAgentID string) {
	t.Helper()
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{
		ID: "adopt-" + variantID, TenantID: 1, ListingID: listingID, AcceptedReleaseID: releaseID, State: "active",
	}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{
		ID: variantID, TenantID: 1, AdoptionID: "adopt-" + variantID, ReleaseID: releaseID,
		Name: "Sales Assistant", State: "published", LocalAgentID: localAgentID,
	}).Error)
}

func lineageMetadataFor(licenseID, changeNotes string) interfaces.SubmitReleaseInput {
	return interfaces.SubmitReleaseInput{Metadata: types.ReleaseMetadata{
		SemanticVersion: "2.0.0", DisplayName: "Derived helper", Summary: "Derived",
		SupportedLanguages: []string{"en"}, UseCases: []string{"support"},
		MinimumWeKnoraCapability: "1", LicenseID: licenseID, ChangeNotes: changeNotes,
	}}
}

// AC1（下层）：纯映射派生（本地绑定不同、可移植核心相同）→ is_fork=false
// 且 lineage 完整；改可移植核心 → is_fork=true。基线是发布管线重投影，
// 不是源 Release 原始载荷（差异记录 #3）。
func TestSubmitReleaseLineageVerdict(t *testing.T) {
	db := openAgentVersionServiceTestDB(t)
	repo := repository.NewAgentMarketplaceRepository(db)
	require.NoError(t, repo.UpsertLicense(context.Background(), &types.AgentLicenseEntity{
		ID: "MIT", Name: "MIT License", AllowsRedistribution: true, CreatedBy: "admin",
	}))
	listingID, releaseID := seedLineageSource(t, db, "mit", "MIT", "Be portable.")
	seedPublishedVariantAgent(t, db, listingID, releaseID, "variant-1", "agent-local")
	svc := newLineageMarketplaceService(t, db)

	// 纯映射派生：本地绑定（knowledge_bases/model_id）在快照里，可移植核心不变。
	mappingOnly, snapshot := lineageSnapshot(t, "version-local", "agent-local", "Be portable.", true)
	seedFrozenAgentVersion(t, db, mappingOnly, "agent-local", snapshot)
	view, err := svc.SubmitRelease(context.Background(), 1, "admin", mappingOnly, lineageMetadataFor("MIT", "re-mapped knowledge binding"))
	require.NoError(t, err)
	require.False(t, view.IsFork, "纯映射修改绝不判为 Fork（AC1）")
	require.Equal(t, listingID, view.ForkSourceListingID)
	require.Equal(t, releaseID, view.ForkSourceReleaseID)
	require.Equal(t, "MIT", view.LineageLicenseID)
	require.Equal(t, "re-mapped knowledge binding", view.ForkNotes, "修改说明 = ChangeNotes")
	require.Contains(t, view.ManifestJSON, `"lineage"`)
	require.Contains(t, view.ManifestJSON, `"is_fork":false`)

	// Fork：改可移植核心（system prompt）。
	forked, forkSnapshot := lineageSnapshot(t, "version-fork", "agent-local", "Be portable, but sharper.", true)
	seedFrozenAgentVersion(t, db, forked, "agent-local", forkSnapshot)
	forkView, err := svc.SubmitRelease(context.Background(), 1, "admin", forked, lineageMetadataFor("MIT", "sharper prompt"))
	require.NoError(t, err)
	require.True(t, forkView.IsFork)
	require.Contains(t, forkView.ManifestJSON, `"is_fork":true`)

	// 原始内容（无 variant 台账）：无 lineage、不查注册表、manifest 无 lineage 段。
	original, originalSnapshot := lineageSnapshot(t, "version-orig", "agent-orig", "Brand new.", false)
	seedFrozenAgentVersion(t, db, original, "agent-orig", originalSnapshot)
	origView, err := svc.SubmitRelease(context.Background(), 1, "admin", original, lineageMetadataFor("unregistered-raw", ""))
	require.NoError(t, err, "原始内容提交不查许可证注册表（差异记录 #5）")
	require.False(t, origView.IsFork)
	require.Empty(t, origView.ForkSourceListingID)
	require.Empty(t, origView.LineageLicenseID)
	require.NotContains(t, origView.ManifestJSON, `"lineage"`)
}

// AC2（下层）：来源许可证禁止/未注册 → 服务端拒绝；许可证翻转后 live 生效；
// 损坏 lineage 数据 fail closed。
func TestSubmitReleaseRedistributionGate(t *testing.T) {
	db := openAgentVersionServiceTestDB(t)
	repo := repository.NewAgentMarketplaceRepository(db)
	require.NoError(t, repo.UpsertLicense(context.Background(), &types.AgentLicenseEntity{
		ID: "tenant-private", Name: "Internal", AllowsRedistribution: false, CreatedBy: "admin",
	}))
	svc := newLineageMarketplaceService(t, db)

	// 已注册但禁止再分发。
	listingID, releaseID := seedLineageSource(t, db, "private", "tenant-private", "Be portable.")
	seedPublishedVariantAgent(t, db, listingID, releaseID, "variant-p", "agent-private")
	mappingOnly, snapshot := lineageSnapshot(t, "version-private", "agent-private", "Be portable.", true)
	seedFrozenAgentVersion(t, db, mappingOnly, "agent-private", snapshot)
	_, err := svc.SubmitRelease(context.Background(), 1, "admin", mappingOnly, lineageMetadataFor("tenant-private", ""))
	require.ErrorIs(t, err, ErrReleaseRedistributionForbidden, "禁止再分发时派生 Submission 被拒（AC2），与 fork 判定无关")
	require.Contains(t, err.Error(), "tenant-private")

	// 未注册许可证：fail closed。
	ghostListing, ghostRelease := seedLineageSource(t, db, "ghost", "ghost-license", "Be portable.")
	seedPublishedVariantAgent(t, db, ghostListing, ghostRelease, "variant-g", "agent-ghost")
	ghostVersion, ghostSnapshot := lineageSnapshot(t, "version-ghost", "agent-ghost", "Be portable.", true)
	seedFrozenAgentVersion(t, db, ghostVersion, "agent-ghost", ghostSnapshot)
	_, err = svc.SubmitRelease(context.Background(), 1, "admin", ghostVersion, lineageMetadataFor("ghost-license", ""))
	require.ErrorIs(t, err, ErrReleaseRedistributionForbidden)
	require.Contains(t, err.Error(), "not registered")

	// 许可证翻转后 live 放行（同一派生链）。
	require.NoError(t, repo.UpsertLicense(context.Background(), &types.AgentLicenseEntity{
		ID: "tenant-private", Name: "Internal", AllowsRedistribution: true, CreatedBy: "admin",
	}))
	allowed, err := svc.SubmitRelease(context.Background(), 1, "admin", mappingOnly, lineageMetadataFor("tenant-private", ""))
	require.NoError(t, err)
	require.False(t, allowed.IsFork)

	// 损坏的源 bundle：fail closed，绝不静默放行（Review Focus 5）。
	require.NoError(t, db.Model(&types.AgentReleaseEntity{}).Where("id = ?", releaseID).
		Update("bundle", []byte(`{broken`)).Error)
	_, err = svc.SubmitRelease(context.Background(), 1, "admin", mappingOnly, lineageMetadataFor("tenant-private", ""))
	require.ErrorIs(t, err, ErrReleaseLineageUnavailable)

	// variant 指向缺失 Release：fail closed。
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{
		ID: "adopt-dangling", TenantID: 1, ListingID: listingID, AcceptedReleaseID: releaseID, State: "active",
	}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{
		ID: "variant-dangling", TenantID: 1, AdoptionID: "adopt-dangling", ReleaseID: "release-missing",
		Name: "Dangling", State: "published", LocalAgentID: "agent-dangling",
	}).Error)
	dangling, danglingSnapshot := lineageSnapshot(t, "version-dangling", "agent-dangling", "Be portable.", true)
	seedFrozenAgentVersion(t, db, dangling, "agent-dangling", danglingSnapshot)
	_, err = svc.SubmitRelease(context.Background(), 1, "admin", dangling, lineageMetadataFor("tenant-private", ""))
	require.ErrorIs(t, err, ErrReleaseLineageUnavailable)
}

// 许可证注册表管理：校验与翻转。
func TestRegisterLicenseValidationAndList(t *testing.T) {
	db := openAgentVersionServiceTestDB(t)
	svc := newLineageMarketplaceService(t, db)

	_, err := svc.RegisterLicense(context.Background(), "admin", interfaces.LicenseInput{ID: "  "})
	require.ErrorIs(t, err, ErrAgentLicenseInvalid)
	_, err = svc.RegisterLicense(context.Background(), "", interfaces.LicenseInput{ID: "MIT"})
	require.ErrorIs(t, err, ErrAgentLicenseInvalid)

	created, err := svc.RegisterLicense(context.Background(), "admin", interfaces.LicenseInput{ID: "MIT", Name: "MIT", AllowsRedistribution: true})
	require.NoError(t, err)
	require.True(t, created.AllowsRedistribution)
	flipped, err := svc.RegisterLicense(context.Background(), "admin", interfaces.LicenseInput{ID: "MIT", Name: "MIT", AllowsRedistribution: false})
	require.NoError(t, err)
	require.False(t, flipped.AllowsRedistribution)

	rows, err := svc.ListLicenses(context.Background())
	require.NoError(t, err)
	require.Len(t, rows, 1)
}
```

同时在该文件顶部把 import 补齐为：

```go
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)
```

- [ ] **Step 6: 运行确认失败**

Run: `go test ./internal/application/service/ -run 'TestSubmitReleaseLineageVerdict|TestSubmitReleaseRedistributionGate|TestRegisterLicense' -count=1`
Expected: FAIL（`ErrReleaseRedistributionForbidden`/`svc.RegisterLicense` undefined 等）

- [ ] **Step 7: 扩展服务接口**

`internal/types/interfaces/agent_marketplace.go`：`AgentMarketplaceService` 接口 `GetRelease(...)` 之后追加：

```go
	// RegisterLicense records (or re-records) one deployment license term;
	// re-registering is how a redistribution flip propagates (T32 #62).
	RegisterLicense(ctx context.Context, actorID string, input LicenseInput) (types.AgentLicenseEntity, error)
	ListLicenses(ctx context.Context) ([]types.AgentLicenseEntity, error)
```

并在文件末尾追加：

```go
// LicenseInput registers one deployment license term. AllowsRedistribution
// is the flag the fork re-submission gate consults (T32 #62).
type LicenseInput struct {
	ID                   string
	Name                 string
	AllowsRedistribution bool
}
```

- [ ] **Step 8: 实现服务层判定与门**

创建 `internal/application/service/agent_fork_lineage.go`：

```go
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/experts"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// Agent Fork lineage + re-distribution gate (T32 #62, spec §5/§6/§9).
//
// The lineage is a SERVER-side fact: it is resolved from the adoption
// variant ledger via the frozen version's source Agent alone — the request
// body never carries lineage fields (strict decoding rejects them). The
// fork verdict compares the submitted portable core against the source
// Release's publish-pipeline projection; the redistribution gate consults
// the deployment license registry live and fails closed.

var (
	// ErrReleaseRedistributionForbidden rejects a derived Submission whose
	// source license does not (or does not yet) permit redistribution.
	ErrReleaseRedistributionForbidden = errors.New("release redistribution is forbidden by the source lineage license")
	// ErrReleaseLineageUnavailable marks unresolvable/corrupt lineage data;
	// the submission fails closed instead of passing as non-fork.
	ErrReleaseLineageUnavailable = errors.New("release derivation lineage could not be resolved")
	// ErrAgentLicenseInvalid rejects malformed license registry writes.
	ErrAgentLicenseInvalid = errors.New("invalid agent license request")
)

// resolveSubmissionLineage derives the submission's lineage and fork
// verdict, gate included. Returns (nil, "", nil) for original content:
// no lineage columns, no license lookup, a lineage-free manifest.
func (s *AgentMarketplaceService) resolveSubmissionLineage(ctx context.Context, tenantID uint64, version types.AgentVersionSnapshot) (*types.AgentReleaseLineage, string, error) {
	derivation, err := s.repo.FindDerivation(ctx, tenantID, version.AgentID)
	if err != nil {
		return nil, "", err
	}
	if derivation == nil {
		return nil, "", nil
	}
	release := derivation.Release
	if release == nil {
		return nil, "", fmt.Errorf("%w: derivation variant %s pins a missing release", ErrReleaseLineageUnavailable, derivation.Variant.ID)
	}
	var sourceManifest types.AgentReleaseManifest
	if err := json.Unmarshal([]byte(release.ManifestJSON), &sourceManifest); err != nil {
		return nil, "", fmt.Errorf("%w: decode the source release manifest: %v", ErrReleaseLineageUnavailable, err)
	}
	if err := s.requireRedistributable(ctx, sourceManifest.LicenseID); err != nil {
		return nil, "", err
	}
	baseline, err := derivationBaselinePayload(derivation)
	if err != nil {
		return nil, "", err
	}
	submitted, err := experts.PortablePayloadOf(version.Agent)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrReleaseLineageUnavailable, err)
	}
	isFork, err := forkVerdict(submitted, baseline)
	if err != nil {
		return nil, "", err
	}
	return &types.AgentReleaseLineage{
		SourceListingID: derivation.ListingID,
		SourceReleaseID: release.ID,
		IsFork:          isFork,
	}, strings.TrimSpace(sourceManifest.LicenseID), nil
}

// requireRedistributable fails closed unless the deployment license
// registry records the source license as redistribution-permitted
// (spec §9「许可证允许时，Fork 可经 Tenant Review 或 Verified Publisher 加
// Platform Review 再发布」). An unregistered license is NOT permission.
func (s *AgentMarketplaceService) requireRedistributable(ctx context.Context, licenseID string) error {
	licenseID = strings.TrimSpace(licenseID)
	license, err := s.repo.GetLicense(ctx, licenseID)
	if err != nil {
		return err
	}
	if license == nil {
		return fmt.Errorf("%w: source license %q is not registered in the license registry", ErrReleaseRedistributionForbidden, licenseID)
	}
	if !license.AllowsRedistribution {
		return fmt.Errorf("%w: source license %q does not permit redistribution", ErrReleaseRedistributionForbidden, licenseID)
	}
	return nil
}

// derivationBaselinePayload recomputes the portable payload the publish
// pipeline would produce from the source Release (buildLocalAgent ->
// PortablePayloadOf). Comparing against THIS baseline — not the raw source
// payload — keeps pure capability-mapping work from reading as a
// portable-core change despite the publish projection's round-trip shape
// (spec §9「只修改本地资源和策略属于 Mapping」; plan-t62 差异记录 #3).
func derivationBaselinePayload(derivation *types.AgentForkDerivation) (types.AgentReleasePayload, error) {
	var envelope struct {
		Payload types.AgentReleasePayload `json:"payload"`
	}
	if err := json.Unmarshal(derivation.Release.Bundle, &envelope); err != nil {
		return types.AgentReleasePayload{}, fmt.Errorf("%w: decode the source release bundle: %v", ErrReleaseLineageUnavailable, err)
	}
	var manifest types.AgentReleaseManifest
	// buildLocalAgent reads only the Summary; the manifest was already
	// strictly decoded for the license gate above, so a tolerated decode
	// here cannot bypass the gate.
	_ = json.Unmarshal([]byte(derivation.Release.ManifestJSON), &manifest)
	agent := buildLocalAgent(&derivation.Variant, envelope.Payload, manifest, nil)
	baseline, err := experts.PortablePayloadOf(agent)
	if err != nil {
		return types.AgentReleasePayload{}, fmt.Errorf("%w: project the source baseline: %v", ErrReleaseLineageUnavailable, err)
	}
	return baseline, nil
}

// forkVerdict decides whether a lineage-derived submission is a Fork
// (CONTEXT.md「Agent Fork」): only a portable-core change is a Fork. Local
// capability mapping bindings never enter the portable payload and
// therefore never flip the verdict (AC1).
func forkVerdict(submitted, baseline types.AgentReleasePayload) (bool, error) {
	submittedJSON, err := json.Marshal(submitted)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrReleaseLineageUnavailable, err)
	}
	baselineJSON, err := json.Marshal(baseline)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrReleaseLineageUnavailable, err)
	}
	return !bytes.Equal(submittedJSON, baselineJSON), nil
}

// RegisterLicense records (or re-records) one deployment license term. The
// deployment is the governance domain: Admin+ members manage the same
// registry the redistribution gate consults live.
func (s *AgentMarketplaceService) RegisterLicense(ctx context.Context, actorID string, input interfaces.LicenseInput) (types.AgentLicenseEntity, error) {
	actorID, input.ID, input.Name = strings.TrimSpace(actorID), strings.TrimSpace(input.ID), strings.TrimSpace(input.Name)
	if actorID == "" || input.ID == "" || len(input.ID) > 64 || len(input.Name) > 255 {
		return types.AgentLicenseEntity{}, ErrAgentLicenseInvalid
	}
	return s.repo.UpsertLicense(ctx, &types.AgentLicenseEntity{
		ID: input.ID, Name: input.Name, AllowsRedistribution: input.AllowsRedistribution,
		CreatedBy: actorID, UpdatedAt: time.Now().UTC(),
	})
}

// ListLicenses returns the deployment license registry.
func (s *AgentMarketplaceService) ListLicenses(ctx context.Context) ([]types.AgentLicenseEntity, error) {
	return s.repo.ListLicenses(ctx)
}
```

- [ ] **Step 9: 接入 SubmitRelease**

`internal/application/service/agent_marketplace.go` 的 `SubmitRelease`：在 `lock, err := s.resolver.Resolve(...)` 错误检查之后、`bundle, err := experts.BuildAgentReleaseBundle(...)` 之前插入：

```go
	lineage, lineageLicenseID, err := s.resolveSubmissionLineage(ctx, tenantID, version)
	if err != nil {
		return interfaces.ReleaseSubmissionView{}, err
	}
```

并把 bundle 构造改为 `experts.BuildAgentReleaseBundleWithLineage(version, input.Metadata, lock, lineage)`；在 `submission := &types.AgentReleaseSubmissionEntity{...}` 字面量中 `Status: "submitted"` 之前追加：

```go
		IsFork:              lineage != nil && lineage.IsFork,
		ForkSourceListingID: lineageSourceValue(lineage, func(l *types.AgentReleaseLineage) string { return l.SourceListingID }),
		ForkSourceReleaseID: lineageSourceValue(lineage, func(l *types.AgentReleaseLineage) string { return l.SourceReleaseID }),
		ForkNotes:           lineageNotes(lineage, bundle.Manifest.ChangeNotes),
		LineageLicenseID:    lineageLicenseID,
```

并在 `ensurePayloadReferencesLocked` 函数之前追加两个小助手：

```go
// lineageSourceValue keeps the original-content path all-zero: derived
// submissions copy the resolved lineage, original ones stay empty.
func lineageSourceValue(lineage *types.AgentReleaseLineage, pick func(*types.AgentReleaseLineage) string) string {
	if lineage == nil {
		return ""
	}
	return pick(lineage)
}

// lineageNotes records the modification notes (修改说明) exactly for
// derived submissions — the author's ChangeNotes at submit time (spec §9).
func lineageNotes(lineage *types.AgentReleaseLineage, changeNotes string) string {
	if lineage == nil {
		return ""
	}
	return changeNotes
}
```

- [ ] **Step 10: 运行服务层测试确认通过**

Run: `go test ./internal/application/service/ -run 'TestSubmitReleaseLineageVerdict|TestSubmitReleaseRedistributionGate|TestRegisterLicense|TestAgentForkLineageMigration' -count=1` 与 `go test ./internal/application/service/ -count=1`（全包回归）
Expected: PASS（含既有 `TestAgentMarketplace*` 全绿）

- [ ] **Step 11: 提交**

```bash
git add internal/modules/agentruntime/agent/experts/agent_release.go internal/modules/agentruntime/agent/experts/agent_release_lineage_test.go internal/application/service/agent_fork_lineage.go internal/application/service/agent_marketplace.go internal/types/interfaces/agent_marketplace.go internal/application/service/agent_fork_lineage_test.go
git commit -m "feat(marketplace): server-side fork verdict and redistribution gate on derived submissions (#62)"
```

---

### Task 4: 审批传播 + 公共 lane 门 + HTTP wire

**Files:**
- Modify: `internal/application/repository/agent_marketplace.go:145`（`ReviewAndPublishTx` 的 release 字面量）
- Modify: `internal/application/service/public_marketplace.go`（`SubmitPublicRelease` 公共门）
- Modify: `internal/handler/agent_marketplace.go`（DTO 字段 + license 端点 + 错误映射）
- Modify: `internal/handler/public_marketplace.go`（`publicMarketplaceClientError` 加分支）
- Modify: `internal/router/routes_agent_marketplace.go`（license 路由）
- Test: 复用 Task 3/5（本任务无独立新测试文件；行为由 Task 5 端到端钉死）

**Interfaces:**
- Consumes: Task 1-3 全部；既有 `marketplaceClientError`/`publicMarketplaceClientError` 错误映射函数。
- Produces: HTTP `POST/GET /api/v1/marketplace/licenses`（Admin+）；submission/release wire 的 5 个 lineage 字段；`ErrReleaseRedistributionForbidden` → 409（两条通道）。

- [ ] **Step 1: ReviewAndPublishTx 传播 lineage 列**

`internal/application/repository/agent_marketplace.go:145` 的 `release = &types.AgentReleaseEntity{...}` 字面量中，`PublishedBy: decision.ReviewerID,` 之前追加：

```go
				IsFork: submission.IsFork, ForkSourceListingID: submission.ForkSourceListingID,
				ForkSourceReleaseID: submission.ForkSourceReleaseID, ForkNotes: submission.ForkNotes,
				LineageLicenseID: submission.LineageLicenseID,
```

- [ ] **Step 2: 公共 lane 门**

`internal/application/service/public_marketplace.go` 的 `SubmitPublicRelease`：在 `verifyBundleDigest(release.Bundle, release.BundleDigest)` 错误检查之后、`created, err := s.repo.CreatePublicSubmission(...)` 之前插入：

```go
	// T32 #62 AC2（公共 lane）：带 lineage 的 Release 再次分发前必须 live
	// 通过许可证注册表——租户 lane 发布时的放行不缓存（注册表可翻转）。
	if strings.TrimSpace(release.LineageLicenseID) != "" {
		license, err := s.listings.GetLicense(ctx, release.LineageLicenseID)
		if err != nil {
			return interfaces.PublicSubmissionView{}, err
		}
		if license == nil {
			return interfaces.PublicSubmissionView{}, fmt.Errorf("%w: source lineage license %q is not registered in the license registry", ErrReleaseRedistributionForbidden, release.LineageLicenseID)
		}
		if !license.AllowsRedistribution {
			return interfaces.PublicSubmissionView{}, fmt.Errorf("%w: source lineage license %q does not permit redistribution", ErrReleaseRedistributionForbidden, release.LineageLicenseID)
		}
	}
```

（`strings`/`fmt` 已在该文件 import 中。）

- [ ] **Step 3: handler wire**

`internal/handler/agent_marketplace.go`：

(a) `marketplaceSubmissionResponse` 的 `Status` 字段之后追加：

```go
	IsFork              bool        `json:"is_fork"`
	ForkSourceListingID string      `json:"fork_source_listing_id,omitempty"`
	ForkSourceReleaseID string      `json:"fork_source_release_id,omitempty"`
	ForkNotes           string      `json:"fork_notes,omitempty"`
	LineageLicenseID    string      `json:"lineage_license_id,omitempty"`
```

`marketplaceReleaseResponse` 的 `PublishedBy` 字段之前追加同一字段块。

(b) `marketplaceSubmissionDTO` 的 return 字面量 `Status: row.Status,` 之后追加：

```go
IsFork: row.IsFork, ForkSourceListingID: row.ForkSourceListingID, ForkSourceReleaseID: row.ForkSourceReleaseID, ForkNotes: row.ForkNotes, LineageLicenseID: row.LineageLicenseID,
```

`marketplaceReleaseDTO` 的 `PublishedBy: row.PublishedBy,` 之前追加同一映射。

(c) `marketplaceClientError` 的 switch 中，`ErrAgentMarketplaceMissingDependency` case 之前插入：

```go
	case stderrors.Is(err, marketservice.ErrReleaseRedistributionForbidden):
		// The refusal message IS the governance reason: it names the
		// source license and why it refuses (409, reviewable).
		return apperrors.NewConflictError(err.Error())
```

(d) 文件末尾追加 license 端点：

```go
type registerLicenseBody struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	AllowsRedistribution bool   `json:"allows_redistribution"`
}

type licenseResponse struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	AllowsRedistribution bool      `json:"allows_redistribution"`
	CreatedBy            string    `json:"created_by"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func licenseDTO(row types.AgentLicenseEntity) licenseResponse {
	return licenseResponse{ID: row.ID, Name: row.Name, AllowsRedistribution: row.AllowsRedistribution, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

// RegisterLicense records (or re-records) one deployment license term
// (T32 #62). The redistribution gate reads this registry live.
func (h *AgentMarketplaceHandler) RegisterLicense(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agentMarketplaceMaxRequestBytes)
	var body *registerLicenseBody
	if err := decodeAgentMarketplaceBody(c.Request.Body, &body); err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	if body == nil || strings.TrimSpace(body.ID) == "" {
		invalidMarketplaceBody(c, stderrors.New("id is required"))
		return
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	row, err := h.market.RegisterLicense(c.Request.Context(), actorID, interfaces.LicenseInput{
		ID: body.ID, Name: body.Name, AllowsRedistribution: body.AllowsRedistribution,
	})
	if err != nil {
		if stderrors.Is(err, marketservice.ErrAgentLicenseInvalid) {
			_ = c.Error(apperrors.NewValidationError("invalid agent license request"))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": licenseDTO(row)})
}

// ListLicenses returns the deployment license registry (Admin+).
func (h *AgentMarketplaceHandler) ListLicenses(c *gin.Context) {
	rows, err := h.market.ListLicenses(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	data := make([]licenseResponse, 0, len(rows))
	for _, row := range rows {
		data = append(data, licenseDTO(row))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}
```

`internal/handler/public_marketplace.go` 的 `publicMarketplaceClientError` switch 中 `ErrPublicMarketplaceStaleDigest` case 之前插入：

```go
	case stderrors.Is(err, marketservice.ErrReleaseRedistributionForbidden):
		return apperrors.NewConflictError(err.Error())
```

- [ ] **Step 4: 路由**

`internal/router/routes_agent_marketplace.go` 的 `RegisterAgentMarketplaceRoutes` 中 `tenantCatalog := ...` 之后追加：

```go
	// T32 #62: the deployment license registry the fork re-submission gate
	// consults. Admin+ (deployment governance domain).
	licenses := g.apiKeyGroup(r.Group("/marketplace/licenses"), apiKeyFullAccess())
	licenses.POST("", g.Admin(), marketHandler.RegisterLicense)
	licenses.GET("", g.Admin(), marketHandler.ListLicenses)
```

- [ ] **Step 5: 构建 + 既有全量回归**

Run: `go build ./...` 与 `go test ./internal/application/service/ ./internal/application/repository/ ./internal/modules/agentruntime/agent/experts/ -count=1`
Expected: PASS（既有 marketplace/adoption/public 服务与仓储测试全绿——本任务不改变任何既有行为）

- [ ] **Step 6: 提交**

```bash
git add internal/application/repository/agent_marketplace.go internal/application/service/public_marketplace.go internal/handler/agent_marketplace.go internal/handler/public_marketplace.go internal/router/routes_agent_marketplace.go
git commit -m "feat(marketplace): propagate fork lineage through approval and gate the public lane (#62)"
```

---

### Task 5: AC1/AC2/AC3 端到端证据（真实 HTTP）

**Files:**
- Test: `internal/router/routes_agent_fork_lineage_test.go`（新）

**Interfaces:**
- Consumes: `newPublicMarketplaceTestApp(t) (*gin.Engine, *rbacGuards, *gorm.DB)`（`internal/router/routes_public_marketplace_test.go:29`）、`publicCall(r *gin.Engine, tenantID uint64, systemAdmin bool, method, path, role, actor string, body any) *httptest.ResponseRecorder`（同文件 :83）、`publishAdoptionRelease(t, r)`（`routes_agent_adoption_test.go`，以 `"MIT"` 为 license_id 发布源 Release——测试先注册 MIT 为允许再分发即可）。
- Produces: AC1/AC2/AC3 的最高稳定 Interface 证据（真实 sqlite 迁移流 + 真实服务链路，唯一测试替身是测试自带的 RBAC 上下文注入中间件）。

- [ ] **Step 1: 写完整端到端测试文件**

创建 `internal/router/routes_agent_fork_lineage_test.go`，内容如下（完整最终形态）：

```go
package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// forkLineageBody 是端到端断言用的 submission wire 形状。manifest 是内嵌
// JSON 对象，解码为 RawMessage 后做字符串断言。
type forkLineageBody struct {
	Data struct {
		ID                  string          `json:"id"`
		ListingID           string          `json:"listing_id"`
		BundleDigest        string          `json:"bundle_digest"`
		IsFork              bool            `json:"is_fork"`
		ForkSourceListingID string          `json:"fork_source_listing_id"`
		ForkSourceReleaseID string          `json:"fork_source_release_id"`
		ForkNotes           string          `json:"fork_notes"`
		LineageLicenseID    string          `json:"lineage_license_id"`
		Manifest            json.RawMessage `json:"manifest"`
	} `json:"data"`
}

// forkReviewBody 是 review 端点的 release 解码形状。
type forkReviewBody struct {
	Data struct {
		Release *struct {
			ID string `json:"id"`
		} `json:"release"`
	} `json:"data"`
}

// registerForkLicense 注册（或翻转）一条部署许可证（Admin+ 端点）。
func registerForkLicense(t *testing.T, r *gin.Engine, id string, allowsRedistribution bool) {
	t.Helper()
	registered := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/licenses", "admin", "admin", map[string]any{
		"id": id, "name": "License " + id, "allows_redistribution": allowsRedistribution,
	})
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, registered.Code, registered.Body.String())
}

// seedForkChain 走真实 HTTP adoption 链：adopt → variant → 完整映射（含一次
// 重映射，即 AC1 的「Mapping 修改」）→ test → publish →（editedPrompt 非空时
// 修改可移植核心并 freeze 新版本）。返回 (localAgentID, 可提交的 versionID)：
// editedPrompt 为空时是发布时冻结的版本（纯映射派生），非空时是 fork 冻结版本。
func seedForkChain(t *testing.T, r *gin.Engine, listingID, editedPrompt string) (string, string) {
	t.Helper()
	call := func(method, path string, body any) *httptest.ResponseRecorder {
		return publicCall(r, 1, false, method, path, "admin", "admin", body)
	}

	adopted := call(http.MethodPost, "/api/v1/marketplace/tenant/adoptions", map[string]any{"listing_id": listingID})
	// 首次 201（created）；同一 listing 再派生时是幂等 re-adopt → 200，
	// accepted 指针指向 listing 当前 Release（201/200 都合法）。
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, adopted.Code, adopted.Body.String())
	var adoption struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(adopted.Body.Bytes(), &adoption))

	variant := call(http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoption.Data.ID+"/variants", map[string]any{"name": "Sales Assistant"})
	require.Equal(t, http.StatusCreated, variant.Code, variant.Body.String())
	var variantBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(variant.Body.Bytes(), &variantBody))

	// 完整映射后再重映射一次（换知识库绑定）：AC1 的「Mapping 修改」。
	for _, kb := range []string{"kb-sales", "kb-legal"} {
		mapped := call(http.MethodPut, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/capability-mapping", map[string]any{"mappings": []map[string]any{
			{"capability": "model", "model_id": "gpt-x"},
			{"capability": "knowledge", "knowledge_base_ids": []string{kb}},
		}})
		require.Equal(t, http.StatusOK, mapped.Code, mapped.Body.String())
	}

	tested := call(http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/test", nil)
	require.Equal(t, http.StatusOK, tested.Code, tested.Body.String())
	published := call(http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/publish", nil)
	require.Equal(t, http.StatusOK, published.Code, published.Body.String())
	var publishBody struct {
		Data struct {
			LocalAgentID        string `json:"local_agent_id"`
			LocalAgentVersionID string `json:"local_agent_version_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(published.Body.Bytes(), &publishBody))
	require.NotEmpty(t, publishBody.Data.LocalAgentID)
	require.NotEmpty(t, publishBody.Data.LocalAgentVersionID)

	if editedPrompt == "" {
		return publishBody.Data.LocalAgentID, publishBody.Data.LocalAgentVersionID
	}
	edited := call(http.MethodPut, "/api/v1/agents/"+publishBody.Data.LocalAgentID, map[string]any{
		"name": "Sales Assistant", "description": "forked",
		"config": map[string]any{"agent_mode": "smart-reasoning", "system_prompt": editedPrompt},
	})
	require.Equal(t, http.StatusOK, edited.Code, edited.Body.String())
	frozen := call(http.MethodPost, "/api/v1/agents/"+publishBody.Data.LocalAgentID+"/versions", nil)
	require.Equal(t, http.StatusCreated, frozen.Code, frozen.Body.String())
	var frozenBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(frozen.Body.Bytes(), &frozenBody))
	return publishBody.Data.LocalAgentID, frozenBody.Data.ID
}

// submitForkVersionRaw 提交一个冻结版本，返回原始响应（供 409 断言）。
func submitForkVersionRaw(r *gin.Engine, role, actor, versionID, licenseID, version, notes string) *httptest.ResponseRecorder {
	return publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/tenant/release-submissions", role, actor, map[string]any{
		"agent_version_id": versionID,
		"metadata": map[string]any{
			"semantic_version": version, "display_name": "Derived helper", "summary": "Derived",
			"supported_languages": []string{"en"}, "use_cases": []string{"support"},
			"minimum_weknora_capability": "1", "license_id": licenseID,
			"change_notes": notes,
		},
	})
}

// submitForkVersion 提交并断言 201，返回解码后的 submission。
func submitForkVersion(t *testing.T, r *gin.Engine, role, actor, versionID, licenseID, version, notes string) forkLineageBody {
	t.Helper()
	submitted := submitForkVersionRaw(r, role, actor, versionID, licenseID, version, notes)
	require.Equal(t, http.StatusCreated, submitted.Code, submitted.Body.String())
	var body forkLineageBody
	require.NoError(t, json.Unmarshal(submitted.Body.Bytes(), &body))
	return body
}

// approveForkSubmission 以 reviewer 身份批准，返回新 Release 的 id。
func approveForkSubmission(t *testing.T, r *gin.Engine, submissionID, digest string) string {
	t.Helper()
	approved := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/tenant/release-submissions/"+submissionID+"/review", "admin", "reviewer", map[string]any{
		"expected_digest": digest, "decision": "approved",
	})
	require.Equal(t, http.StatusOK, approved.Code, approved.Body.String())
	var review forkReviewBody
	require.NoError(t, json.Unmarshal(approved.Body.Bytes(), &review))
	require.NotNil(t, review.Data.Release)
	return review.Data.Release.ID
}

// AC1：Mapping 修改不误判为 Fork；可移植核心修改判为 Fork（同链对照）；
// lineage 由服务端推导、请求体伪造被拒；原始内容无 lineage（Review Focus 3/4）。
func TestAgentForkLineageMappingEditsAreNotForks(t *testing.T) {
	r, _, _ := newPublicMarketplaceTestApp(t)
	registerForkLicense(t, r, "MIT", true)
	listingID, releaseID := publishAdoptionRelease(t, r)

	// 原始内容（agent-owned 的冻结版本，无 variant 台账）：无 lineage、
	// 不查许可证注册表（license 未注册仍 201）。
	origFrozen := publicCall(r, 1, false, http.MethodPost, "/api/v1/agents/agent-owned/versions", "contributor", "contributor", nil)
	require.Equal(t, http.StatusCreated, origFrozen.Code, origFrozen.Body.String())
	var origVersion struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(origFrozen.Body.Bytes(), &origVersion))
	original := submitForkVersion(t, r, "contributor", "contributor", origVersion.Data.ID, "unregistered-raw", "9.0.0", "")
	require.False(t, original.Data.IsFork)
	require.Empty(t, original.Data.ForkSourceListingID, "原始内容没有 lineage（Review Focus 4）")
	require.Empty(t, original.Data.LineageLicenseID)
	require.NotContains(t, string(original.Data.Manifest), `"lineage"`)

	// 纯映射派生（两次映射修改后直接提交发布冻结的版本）→ is_fork=false。
	_, mappingVersionID := seedForkChain(t, r, listingID, "")
	mapping := submitForkVersion(t, r, "admin", "admin", mappingVersionID, "MIT", "2.0.0", "re-mapped knowledge binding")
	require.False(t, mapping.Data.IsFork, "AC1：Mapping 修改不误判为 Fork")
	require.Equal(t, listingID, mapping.Data.ForkSourceListingID)
	require.Equal(t, releaseID, mapping.Data.ForkSourceReleaseID)
	require.Equal(t, "MIT", mapping.Data.LineageLicenseID)
	require.Equal(t, "re-mapped knowledge binding", mapping.Data.ForkNotes, "修改说明 = ChangeNotes")
	require.Contains(t, string(mapping.Data.Manifest), `"lineage"`)
	require.Contains(t, string(mapping.Data.Manifest), `"is_fork":false`)

	// 伪造 lineage 的请求体 → strict decode 400（Review Focus 3）。
	forge := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/tenant/release-submissions", "admin", "admin", map[string]any{
		"agent_version_id":       mappingVersionID,
		"metadata":               map[string]any{},
		"fork_source_release_id": "fake",
	})
	require.Equal(t, http.StatusBadRequest, forge.Code)

	// 对照：修改可移植核心 → is_fork=true。
	_, forkVersionID := seedForkChain(t, r, listingID, "Be portable, but sharper.")
	fork := submitForkVersion(t, r, "admin", "admin", forkVersionID, "MIT", "2.1.0", "sharper prompt")
	require.True(t, fork.Data.IsFork, "可移植核心修改 = Fork（对照）")
	require.Contains(t, string(fork.Data.Manifest), `"is_fork":true`)
}

// AC2（租户 lane）：来源许可证禁止再分发/未注册 → Submission 被服务端拒绝
// （409，消息点名许可证）；翻转注册表后同形态派生链 live 放行。
func TestAgentForkLineageRedistributionForbiddenRejectsSubmission(t *testing.T) {
	r, _, _ := newPublicMarketplaceTestApp(t)
	registerForkLicense(t, r, "tenant-private", false)

	// publishSourceThenFork：以给定许可证发布一条新源 Release（真实
	// 提交+审批，listing 复用累积 Release），跑完整派生链并提交 fork。
	// sourceSemVer 必须逐次递增（1.0.0/1.0.1/1.0.2）：同一 listing 的
	// agent_releases 受 uq_agent_releases_semantic(listing_id, semantic_version)
	// 唯一索引约束，源审批版本重复会使第二次审批撞唯一冲突而非 200。
	// fork 提交三次同为 "2.0.0" 无碍——前两次 409 未落任何行。
	publishSourceThenFork := func(metadataLicense, sourceSemVer, version string) *httptest.ResponseRecorder {
		frozen := publicCall(r, 1, false, http.MethodPost, "/api/v1/agents/agent-owned/versions", "contributor", "contributor", nil)
		require.Equal(t, http.StatusCreated, frozen.Code, frozen.Body.String())
		var sourceVersion struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(frozen.Body.Bytes(), &sourceVersion))
		source := submitForkVersion(t, r, "contributor", "contributor", sourceVersion.Data.ID, metadataLicense, sourceSemVer, "")
		approveForkSubmission(t, r, source.Data.ID, source.Data.BundleDigest)
		_, forkVersionID := seedForkChain(t, r, source.Data.ListingID, "Be portable, forked.")
		return submitForkVersionRaw(r, "admin", "admin", forkVersionID, metadataLicense, version, "forked")
	}

	// 已注册但禁止再分发 → 409，消息点名许可证。
	forbidden := publishSourceThenFork("tenant-private", "1.0.0", "2.0.0")
	require.Equal(t, http.StatusConflict, forbidden.Code, forbidden.Body.String())
	require.Contains(t, forbidden.Body.String(), "tenant-private")
	require.Contains(t, forbidden.Body.String(), "redistribution")

	// 未注册 → 409，fail closed。
	unregistered := publishSourceThenFork("ghost-license", "1.0.1", "2.0.0")
	require.Equal(t, http.StatusConflict, unregistered.Code, unregistered.Body.String())
	require.Contains(t, unregistered.Body.String(), "not registered")

	// 翻转注册表为允许 → 同形态派生链 live 放行，并如实记为 Fork。
	registerForkLicense(t, r, "tenant-private", true)
	allowed := publishSourceThenFork("tenant-private", "1.0.2", "2.0.0")
	require.Equal(t, http.StatusCreated, allowed.Code, allowed.Body.String())
	var allowedBody forkLineageBody
	require.NoError(t, json.Unmarshal(allowed.Body.Bytes(), &allowedBody))
	require.True(t, allowedBody.Data.IsFork)
}

// AC2（公共 lane）：放行期审批的派生 Release 可升公共提交；许可证翻转后，
// 同形态新 Release 的公共提交被 live 拒绝；恢复后放行——租户 lane 的历史
// 放行不被缓存。
func TestAgentForkLineagePublicSubmissionLicenseGate(t *testing.T) {
	r, _, _ := newPublicMarketplaceTestApp(t)
	registerForkLicense(t, r, "MIT", true)
	listingID, _ := publishAdoptionRelease(t, r)

	// F1：fork 链 → 提交 → 审批（放行期）。
	_, forkOneVersion := seedForkChain(t, r, listingID, "Be portable, forked.")
	forkOne := submitForkVersion(t, r, "admin", "admin", forkOneVersion, "MIT", "2.0.0", "forked once")
	require.True(t, forkOne.Data.IsFork)
	forkOneReleaseID := approveForkSubmission(t, r, forkOne.Data.ID, forkOne.Data.BundleDigest)

	// F2：第二条 fork 链（仍在放行期完成租户 lane 提交与审批）。
	_, forkTwoVersion := seedForkChain(t, r, listingID, "Be portable, forked twice.")
	forkTwo := submitForkVersion(t, r, "admin", "admin", forkTwoVersion, "MIT", "3.0.0", "forked twice")
	require.True(t, forkTwo.Data.IsFork)
	forkTwoReleaseID := approveForkSubmission(t, r, forkTwo.Data.ID, forkTwo.Data.BundleDigest)

	// Verified Publisher 注册（SystemAdmin 面）。
	verified := publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/verified-publishers", "admin", "sysadmin", map[string]any{"tenant_id": 1})
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, verified.Code, verified.Body.String())

	// 放行期：F1 升公共提交 → 201。
	promotedOne := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/public/release-submissions", "admin", "admin", map[string]any{
		"source_listing_id": listingID, "release_id": forkOneReleaseID,
	})
	require.Equal(t, http.StatusCreated, promotedOne.Code, promotedOne.Body.String())

	// 翻转：MIT 禁止再分发。
	registerForkLicense(t, r, "MIT", false)

	// F2 升公共提交 → 409（live 查询，历史放行不缓存）。
	promotedTwo := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/public/release-submissions", "admin", "admin", map[string]any{
		"source_listing_id": listingID, "release_id": forkTwoReleaseID,
	})
	require.Equal(t, http.StatusConflict, promotedTwo.Code, promotedTwo.Body.String())
	require.Contains(t, promotedTwo.Body.String(), "redistribution")

	// 恢复允许后，同一 F2 放行（被拒的尝试未落任何行，可重试）。
	registerForkLicense(t, r, "MIT", true)
	retried := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/public/release-submissions", "admin", "admin", map[string]any{
		"source_listing_id": listingID, "release_id": forkTwoReleaseID,
	})
	require.Equal(t, http.StatusCreated, retried.Code, retried.Body.String())
}
```

- [ ] **Step 2: 运行端到端测试**

Run: `go test ./internal/router/ -run 'TestAgentForkLineage' -count=1 -v`
Expected: 3 个测试全部 PASS（AC1 + AC2 租户 lane + AC2 公共 lane）

- [ ] **Step 3: 既有 marketplace/adoption/public 全量回归**

Run: `go test ./internal/router/ -run 'TestTenantAgent|TestPublicMarketplace|TestAvailableAgents' -count=1`
Expected: PASS——既有 #58/#59/#60 行为零回归（原始内容 digest 与既有 lifecycle 不漂移，Review Focus 4 的接口级证据）。

- [ ] **Step 4: 提交**

```bash
git add internal/router/routes_agent_fork_lineage_test.go
git commit -m "test(marketplace): end-to-end fork lineage, license gate and public lane evidence (#62 AC1/AC2/AC3)"
```

---

## 计划级验证命令

在 worktree 根执行（覆盖本计划全部测试与受影响包回归）：

```bash
go build ./... && go test ./internal/application/service/ -run 'TestAgentForkLineageMigration|TestSubmitReleaseLineageVerdict|TestSubmitReleaseRedistributionGate|TestRegisterLicense' -count=1 && go test ./internal/application/repository/ -run 'TestFindDerivation|TestLicenseUpsertGetAndList' -count=1 && go test ./internal/modules/agentruntime/agent/experts/ -count=1 && go test ./internal/router/ -run 'TestAgentForkLineage|TestTenantAgentAdoption|TestTenantAgentMarketplace|TestTenantAgentVariant|TestPublicMarketplace|TestAvailableAgents' -count=1
```

（各包用定向 `-run` 模式避免全量 flaky 套件；`go build ./...` 兜底接线完整性；experts 包全量因为既有 exporter 测试即 digest 零回归证据。）

## 自我审查记录（writing-plans 四项检查）

1. **Spec 覆盖**：AC1「Mapping 修改不误判为 Fork」→ Task 3 基线重投影判定（差异记录 #3 说明为何不改 `buildLocalAgent`）+ Task 5 `TestAgentForkLineageMappingEditsAreNotForks`（两次映射修改→发布→提交→`is_fork=false`，改 prompt→`is_fork=true` 同测试对照）。AC2「禁止再分发时 Submission 由服务端拒绝」→ Task 3 租户 lane 门 + Task 4 公共 lane 门 + Task 5 Test 2（未注册/已注册禁止均 409、翻转 live 放行）与 Test 3（公共 lane 翻转 409/恢复 201）。「Fork 保留来源、许可和修改说明」→ lineage 5 列 + `ForkNotes=ChangeNotes` + Manifest `lineage` 段（Task 1/3/4），断言在 Task 5 Test 1。AC3 → Task 5 真实 HTTP 三场景 + 迁移流装载。无遗漏验收项。
2. **占位符扫描**：全文无 TBD/TODO/「类似 Task N」/「实现细节略」；Task 5 测试代码为完整可落盘最终形态（含全部 helper 与三个测试函数）；占位类词仅出现在禁止事项的反例说明中。
3. **类型/签名一致性**：`FindDerivation(ctx, tenantID uint64, localAgentID string)` 在接口（Task 2 Step 3）、实现（Task 2 Step 4）、fake（Task 2 Step 3）、服务消费（Task 3 Step 8 `s.repo.FindDerivation`）四处一致；`GetLicense(ctx, licenseID string)` 单参数（部署级注册表，无 tenant 维度）与公共门 `s.listings.GetLicense(ctx, release.LineageLicenseID)`（Task 4 Step 2）一致；`BuildAgentReleaseBundleWithLineage` 第 4 参 `*types.AgentReleaseLineage` 与 Task 3 Step 9 传入的 `lineage` 一致；实体列名（snake_case）与迁移 DDL、wire json tag 三方对齐（`is_fork`/`fork_source_listing_id`/`fork_source_release_id`/`fork_notes`/`lineage_license_id`）；`forkLineageBody.Data` 同时携带 `ListingID`/`BundleDigest`（Test 2/3 的源链与审批依赖）；`seedForkChain` 返回 `(localAgentID, versionID)` 与三处调用消费一致。
4. **Review Focus 落实**：5 条全部有钉死测试——①Task 3 `TestSubmitReleaseLineageVerdict`（纯映射 false + 对照 true）+ Task 5 Test 1；②Task 5 Test 2/3；③Task 5 Test 1 伪造 body 400 + 原始内容无 lineage 断言；④Task 3 Step 4 experts 全包回归（`WithLineage(nil)` 字节等同 legacy）+ Task 5 Test 1 原始内容段 + Task 5 Step 3 既有 lifecycle 回归；⑤Task 3 `TestSubmitReleaseRedistributionGate`（损坏 bundle + dangling release 两个 fail closed 分支）。

## 审查修复记录（独立计划审查轮，逐项修复）

1. **【阻断】基线前提为假 → 已修复为 Task 0 待办并实做验证。** 原稿差异记录 #2 声称「迁移流可装载（本会话已在 HEAD 实跑确认）」——该结论来自带未提交改名的工作区快照，随后工作区被重置回干净 HEAD `d52270a0f`，在干净 HEAD 实跑基线命令 `go test ./internal/router/ -run 'TestTenantAgentAdoptionRoutesAndAuthorization|TestTenantAgentMarketplaceLifecycleAndAuthorization|TestPublicMarketplace' -count=1` 全部 FAIL（`duplicate migration file: 000114_public_agent_marketplace.down.sql`，本会话复现）。已把双迁移去重重编改写为 Task 0 的可执行待办（Step 2，幂等守卫 + 5 个测试文件字面引用同步 + Step 3/4 复跑验证），差异记录 #2 相应改写为本会话实测时间线。本会话已在工作区按 Task 0 实做：改名后基线复跑 `ok github.com/Tencent/WeKnora/internal/router 10.525s`，波及的设备测试（`go test ./internal/handler/ -run 'TestMobileDevice'`、`go test ./internal/application/repository/ -run 'TestMobileDevice|TestMobilePush'`）均 `ok`。
2. **【阻断】差异记录 #2 的 000118/000197 文件描述不实 → 已改写。** 干净 HEAD 上 `git status --porcelain -- migrations/` 为空、两套流最大号为 `000117_code_deliveries`/`000196_code_deliveries`、不存在任何 000118/000197 文件（本会话 `ls` 实证）；差异记录 #2 现以实测时间线陈述，并说明占用 000119/000198 不留空洞（mobile_device_app 恰好补位 118/197）。
3. **【阻断】`seedLineageSource` 按 3 返回值调用 `CreateSubmission` → 已修复。** 实际签名 `CreateSubmission(ctx, listing, submission) (*types.AgentReleaseSubmissionEntity, error)`（`internal/application/repository/agent_marketplace.go:57`，且 `:82` 回填 `result.ListingID`）；计划已改为 `submission, err := ...` 并 `return submission.ListingID, release.ID`（本会话读源码核实回填）。
4. **【阻断】仓储测试误用 `internal/logger` → 已修复。** `internal/logger` 包无 `Default`/`Silent` 导出；已改为 `"gorm.io/gorm/logger"`（与 `openAdoptionVariantDB`、`mobile_device_app_test.go` 的既有 import 一致，本会话 rg 实证两文件均 import `gorm.io/gorm/logger`）。
5. **【逻辑】Test 2 源审批撞 `uq_agent_releases_semantic` 唯一索引 → 已修复。** `migrations/sqlite/000109_tenant_agent_marketplace.up.sql:42` 实证 `CREATE UNIQUE INDEX uq_agent_releases_semantic ON agent_releases(listing_id, semantic_version)`；`publishSourceThenFork` 增加逐次递增的 `sourceSemVer` 参数（1.0.0/1.0.1/1.0.2），fork 提交版本恒 "2.0.0" 无碍（前两次 409 未落行）。Test 1/3 与服务层测试经核查无同号审批（Test 3 的 2.0.0/3.0.0 相异；服务层 seed 按 suffix 分 listing）。
6. **【非阻断】三处行号偏差 → 已修正。** `introducedRelease` :394→:372、`newPublicMarketplaceTestApp` :30→:29、`publicCall` :88→:83；顺带把差异记录 #4 的 `TenantID` 改写行修正为 `agent_adoption.go:382`（本会话 sed 实测行号）。
