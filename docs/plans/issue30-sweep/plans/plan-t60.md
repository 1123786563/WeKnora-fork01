# T30：Public Marketplace 审核与跨 Tenant Adoption（Issue #60）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在已合并的 #58（Tenant Release 闭环）与 #59（Tenant Adoption/Variant）之上实现 spec §2/§7/§13 的公共分发层：平台维护 Verified Publisher 名册，Verified Publisher 把本租户已发布的可移植 Release 提交公共目录，Platform Marketplace Reviewer（SystemAdmin）审核后进入 Public Marketplace 目录，另一 Tenant 从目录引入（Adopt）该公共 Release——服务端逐字节复制可移植 Release 摘要内容到采用方租户台账并建立/更新 Adoption，随后 #59 的 Variant→映射→测试→发布→移动 Available Agent 全链在采用方租户内原样跑通；发布者与源 Tenant 的任何可见面都不含采用方身份、映射或 Task 内容。

**Architecture:** 全部为 Go 后端新增层 + contracts 契约镜像，不修改任何移动 TS 业务代码。数据层新增六张表（平台级 `public_marketplace_verified_publishers` / `public_marketplace_listings` / `public_release_submissions` / `public_release_reviews` / `public_agent_releases` 与采用方台账 `tenant_introduced_releases`，双迁移流 versioned 000193 / sqlite 000114），并放宽 #59 治理表上三条指向 `agent_marketplace_listings`/`agent_releases` 的 FK（跨租户 Adoption 的 listing/release 是平台 lineage 身份，无法也绝不物化为本地 `agent_releases` 行——后者 FK 链强制要求本地 submission 与 agent_versions）；仓储层 `internal/application/repository/public_marketplace.go` 拥有全部 SQL（参数绑定），#59 的 `agentAdoptionRepository.GetMarketplaceListing/GetRelease` 增加引入台账回退（本地行优先），`AdoptListing` 的 upsert 核心抽为 `adoptListingTx(tx, …)` 供公共引入事务复用；服务层 `PublicMarketplaceService` 以 #58 的 `interfaces.AgentMarketplaceRepository` 为源 Release 只读来源（无回退，杜绝把引入的公共 Release 再冒名提交公共目录）；HTTP 层新增 `/api/v1/marketplace/public/*` 路由族（租户面 Viewer/Admin + apiKeyFullAccess；平台审核与发布者验证面 SystemAdmin、不对 API key 声明——沿 `routes_auth_tenant.go:290-292` promote/revoke 先例）；端到端证据落在真实 sqlite 迁移流 + 真实服务栈的 router 测试，AC1 以「发布者的 KB/模型绑定不随传播进入采用方本地 Agent、摘要校验失败零传播」钉死，AC2 以「发布者视角全部可见响应不含任何采用方标识 + 公共目录行严格字段解码」钉死。

**Tech Stack:** Go 1.26.0（`go.mod` module `github.com/Tencent/WeKnora`）、gin + gorm + golang-migrate（sqlite 测试流复用 `internal/router/routes_agent_marketplace_test.go:349-373` 的 `openTenantAgentMarketplaceHTTPTestDB` 与 `internal/application/service/agent_version_test.go:31-63` 的 `openAgentVersionServiceTestDB`）；TypeScript contracts（`packages/contracts`，node:test + tsx，与 `src/marketplace/agent-adoption.ts` 同款）。测试命令全部在 worktree 根执行。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-60.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-agent-marketplace-domain-model.md`（§2 统一目录、§5 Release 内容边界、§7 Publication 与 Review、§8 Adoption 与本地发布、§12 隐私、§13 权限能力、§15 验收场景、§16 非目标、§17 完成边界）
- ADR：`docs/adr/0011-agent-marketplace-release-adoption-boundary.md`（全文一句决策）
- 领域术语：`CONTEXT.md:119-152`（Agent Catalog / Agent 引入 / Verified Publisher / Marketplace Reviewer）
- Parent：Issue #30；Blocked by：#58（T28，已合并）、#59（T29，已合并）
- 本 Issue 阻塞：#61（升级建议）、#62（Fork）、#64（安全撤回阻断）、#65（信任信号与 Custody）

## Global Constraints

以下为批准 Spec / Issue 的项目级约束，逐字引用，所有任务隐含遵守：

- 「| Public Marketplace | Verified Publisher | Platform Marketplace Reviewer | 平台允许的 Tenant |」（agent-marketplace-domain-model.md §2 统一目录表）
- 「移动端只显示已完成映射、测试和本地版本发布的 Available Agent。Marketplace 搜索、Submission、Review、Adoption 和升级不在移动端执行。」（同上 §2——本计划不改任何 apps/mobile / mobile-core 代码）
- 「首版不支持任意分享链接、跨 Tenant 私发、消费者五星评论或付费 Agent。」（同上 §2）
- Agent Release 不得携带：「Tenant 知识库、知识条目或成员身份；模型 ID、API Key、Connection credential；Sandbox 绑定、Memory 内容、Task 历史、预算或审计；未获再分发许可的依赖；采用方遥测目标或绕过空间策略的回传配置。」以及「Public、Built-in 与 Tenant Release 遵守同一边界。」（同上 §5）
- 「Verified Publisher 只表示公共发布主体身份已验证，不表示每个 Release 安全或获批。」（同上 §7）
- 「审核记录保留 Author、Submitter、Reviewer、时间、理由和所审核摘要。审核后任何字节变化都必须创建新 Submission 和 Release。」（同上 §7）
- 「Publisher 默认不能获得 Tenant、成员、Prompt、输出、知识、Connection、工具参数、本地 Mapping 或 Task 内容。采用方主动提交诊断时，必须先展示并确认脱敏内容。Release、Skill 和 Connector 不得自带绕过 Tenant 策略的 Publisher telemetry。」（同上 §12）
- 「review_public_release：审核 Public Marketplace；」「adopt_agent：建立 Tenant Adoption；」（同上 §13 权限能力）
- 非目标：「任意链接分享或跨 Tenant 私发」「Publisher 访问采用方遥测或 Task 内容」「移动端 Submission、Review、Adoption、Mapping 或 Agent 编辑」「把 Marketplace Review 宣称为适合所有 Tenant 的保证。」（同上 §16）
- 「本规格确定统一语言、聚合关系、状态、权限和安全不变量，不定义数据库表、HTTP 字段、搜索引擎、扫描器供应商或 UI 视觉。」（同上 §17）
- CONTEXT.md 术语：「**Agent 引入（Agent Adoption）**：一个空间接受某个 Marketplace Listing 并管理其来源与升级关系的记录……它属于空间而不是执行操作的成员」「**Verified Publisher**：通过平台身份与责任验证、可以向 Public Marketplace 提交 Release 的发布主体」「**Marketplace Reviewer**：……Public Marketplace 由平台治理者负责。」（CONTEXT.md:127-152）
- 安全约束（会话注入）：数据库查询所有外部输入使用参数绑定，不得拼接、format 或 f-string 组装 SQL（本计划仓储层全部走 gorm 参数绑定，写 SQL 处均为常量字符串）；本计划不新增服务端外呼请求；不新增任何凭据，源码与测试不写入可用凭据字面量。
- 工作流约束：严格 RED→GREEN→REFACTOR；实现与已批准 Spec 冲突时升级而非静默重设计；**边界**——升级建议（Agent Upgrade Proposal）属 #61、Fork 属 #62、退出（Retire Variant / End Adoption / Unlist / Deprecate）属 #63、安全撤回与传递阻断属 #64、目录信任信号展示（审核徽章聚合、Evaluation、去标识指标）与 Publisher Custody 属 #65；本计划不实现这些行为，但表结构（state 列、lineage 列）不得封死它们的演进。

**Issue #60 验收标准原文（docs/plans/issue30-sweep/issues/issue-60.md）：**

1. 「跨 Tenant 只传播可移植 Release。」
2. 「发布者和源 Tenant 无法读取采用方身份、映射或 Task 内容。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

验收标准 3 的本地可验证性说明：本计划最高稳定 Interface 证据 = `internal/router/routes_public_marketplace_test.go` 的真实 HTTP 端到端测试——真实 sqlite 迁移流（`openTenantAgentMarketplaceHTTPTestDB`，含本计划新增迁移与 FK 放宽，DSN `_foreign_keys=on`）、真实仓储、真实 `CustomAgentService`/`AgentVersionService`/`AgentMarketplaceService`/`AgentAdoptionService`/`PublicMarketplaceService`、真实 `GET /api/v1/agents` 列表端点（#33 移动 Resource Shelf 的 wire 来源，服务端形状 `internal/handler/custom_agent.go:267-272`），仅以测试惯用的 RBAC 中间件注入租户/角色/SystemAdmin 上下文（与已合并的 #58/#59 测试同款）。无 blocked-env 项：本 Issue 不改移动 TS、不需要真机或外部凭据；服务测试（Task 5）与仓储测试（Task 3/4）如实标注为下层证据，不冒充 AC3。

## Review Focus

Spec 隐含但易咬人的五类输入/失效模式（每行后已由所属任务的测试钉死）：

1. **未验证发布者冒名提交公共目录**：任意租户 Admin 直接 `POST /marketplace/public/release-submissions`，必须 403 且零 submission 行（spec §7 Verified Publisher 是公共提交的唯一主体）。——Task 5 `TestPublicMarketplaceServiceSubmitRequiresVerifiedPublisher` + Task 6 `TestPublicMarketplacePublisherVerificationAndSubmissionAuthorization`（未验证 403、Contributor 403、验证后 Admin 201）。
2. **公共 Release 字节被篡改后仍可传播**（db 直改 bundle、digest 不再匹配）：提交侧与引入侧都必须在写库前拒绝（spec §5「同一边界」+ §7「审核后任何字节变化都必须创建新 Submission」），不产生 submission / introduction / adoption 行。——Task 5 `TestPublicMarketplaceServiceSubmitCopiesPortableReleaseVerbatim`（源篡改拒绝）+ Task 8 `TestPublicMarketplaceAdoptRejectsTamperedPublicRelease`（公共 release 篡改 → adopt 拒绝且 tenant_introduced_releases/agent_adoptions 零行）。
3. **采用方隐私泄漏**：公共目录 / 审核面 / 发布者自查询面出现采用方身份、adoption 计数、映射内容或 Task 痕迹（spec §12）。——Task 7 目录行 `DisallowUnknownFields` 严格解码钉死 wire 字段集（任何 adopter 指标字段会直接解码失败）+ Task 8 发布者视角全响应 NotContains 采用方标识（adoption/variant/introduction/本地 Agent/`kb-tenant-2`）与 `GET /marketplace/tenant/adoptions` 为空数组。
4. **平台审核端点被租户 Admin 冒用 / 审核摘要过期**：租户 Admin 调 verified-publishers / review-queue / review 必须 403（spec §13 review_public_release 是平台能力）；digest 与 submission 不一致必须 409 而非静默发布。——Task 6 `TestPublicMarketplaceReviewAuthorization`（非 SystemAdmin 403、stale digest 409、重复审核 409）。
5. **重复 / 并发引入**：同一 (tenant, public release) 二次 adopt、同一公共 listing 先后引入不同 release，必须幂等收敛到唯一 Adoption 且 accepted 指针前进，不得产生第二个 Adoption 或第二个 introduction 行（spec §4「Adoption 在一个 Tenant 内对一个 Listing 唯一」）。——Task 4 `TestPublicMarketplaceRepositoryIntroduceReleaseCopiesPortableBundleAndAdopts`（幂等 + 指针前进）+ Task 7 重复 adopt 200 幂等断言。

## 任务结构与文件地图

| # | 任务 | 主要交付 |
|---|---|---|
| 1 | 迁移版本号去重（HEAD 破窗修复） | adoption 迁移改名 000191→000192 / 000112→000113，`internal/database` 全绿 |
| 2 | Public Marketplace 持久化层 | 六张新表（versioned 000193 / sqlite 000114）+ 三条 FK 放宽 + 实体 |
| 3 | Adoption 仓储引入回退 + adoptListingTx 抽取 | GetMarketplaceListing/GetRelease 台账回退（本地行优先） |
| 4 | Public Marketplace 仓储 | 参数绑定 SQL + IntroduceRelease 事务（复制可移植 Release + Adoption upsert） |
| 5 | Public Marketplace 服务（AC1/AC2 领域逻辑） | Verified Publisher 门禁、提交、平台审核 CAS、目录、引入 |
| 6 | HTTP 面 I：Verified Publisher + 公共提交 + 平台审核 + 接线 | SystemAdmin/授权矩阵端点（Review Focus 1/4） |
| 7 | HTTP 面 II：公共目录读 + Adopt（引入）端点 | Viewer+/Admin 面与幂等（Review Focus 5 前半） |
| 8 | 端到端 AC 证据（AC1/AC2/AC3） | 跨租户全链 + 隐私隔离 + 篡改拒绝（Review Focus 2/3） |
| 9 | contracts 契约层 | 公共目录/提交/审核/引入 wire 契约与解析器 |

**前置条件（本计划作者实测，2026-09-24，worktree `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep`，HEAD fb5f6653a）：**
- `go test ./internal/database/ -count=1` → **FAIL**（`duplicate migration file: 000112_task_grants.down.sql`；`TestSQLiteMigrationsCreateVersionedSchema` 报 "SQLite fixture has duplicate version 112"）——这是 #42 与 #59 在 B3 合并时各自占用 versioned 000191 / sqlite 000112 造成的既有破窗，Task 1 修复。
- `go test ./internal/router/ -run 'TestTenantAgentMarketplaceLifecycleAndAuthorization' -count=1` → **FAIL**（同一 duplicate migration file 错误，`routes_agent_marketplace_test.go:361`）。
- `go build ./internal/...` → 通过（未另行执行全量测试套件）。
- `grep -n "AgentMarketplaceHandler" internal/router/router.go` → `:103`（参数）、`:403`（注册）；`internal/container/container.go:434-438` 为 adoption 接线段。
- `head -5 go.mod` → `go 1.26.0`。

## Consumes（前置批次已合并接口，本计划逐字依赖）

- **#58 Tenant Release**：`interfaces.AgentMarketplaceRepository`（`internal/types/interfaces/agent_marketplace.go:8-17`，本计划消费 `GetListing`/`GetRelease` 作为源 Release 的**无回退**只读来源）；`interfaces.AgentMarketplaceService`（`internal/application/service/agent_marketplace.go:28-42`）；`types.AgentReleaseReviewDecision{ReviewerID, Decision, Reason}`（`internal/types/agent_marketplace_persistence.go:69-73`）；`types.AgentMarketplaceListingEntity`/`AgentReleaseEntity`（`internal/types/agent_marketplace_persistence.go:5-67`）；迁移 `migrations/versioned/000188_tenant_agent_marketplace.up.sql`（`agent_marketplace_listings` 无外联 FK、`agent_releases` FK 链强制 submission+agent_versions——本计划 Task 2 设计依据）；服务包内 `validMarketplaceDigest`（`internal/application/service/agent_marketplace.go:218-224`）与仓储包内 `isUniqueViolation`（`internal/application/repository/voice_session.go:289-299`）同包复用。
- **#59 Tenant Adoption**：`interfaces.AgentAdoptionService`（`internal/types/interfaces/agent_adoption.go:13-24`，本计划不改其签名）；`repository.AgentAdoptionRepository`（`internal/application/repository/agent_adoption.go:39-52`，Task 3 修改其 `GetMarketplaceListing`/`GetRelease` 实现并抽取 `adoptListingTx`）；`service.NewAgentAdoptionService(repo, agents AdoptionAgentSource, versions interfaces.AgentVersionService)`（`internal/application/service/agent_adoption.go:57-59`）；发布链 `PublishVariant` 的 bundle digest 校验与 payload 反序列化（同文件 :270-279）；路由 `internal/router/routes_agent_adoption.go:18-36`；测试骨架 `internal/router/routes_agent_adoption_test.go:21-119`（`newAgentAdoptionTestApp`）与 `internal/application/service/agent_version_test.go:31-63`（`openAgentVersionServiceTestDB`）。
- **#33 移动 Resource Shelf wire**：`GET /api/v1/agents` 信封 `{ success, data: CustomAgent[], disabled_own_agent_ids }`（服务端 `internal/handler/custom_agent.go:267-272`；行含 `id/name/description/is_builtin/config`，`config` 携带 `system_prompt/knowledge_bases/model_id`——`internal/types/custom_agent.go:84,101,121`）。Task 8 以该 wire 断言跨租户传播产物进入移动消费面且不含发布者本地绑定。
- **#59 contracts 模式**：`packages/contracts/src/marketplace/agent-adoption.ts`（本地 `object/requiredString/requiredStringArray` 辅助 + ContractError）与根导出挂接点 `packages/contracts/src/index.ts:727-730`。

## Produces（本计划对外接口，供 #61-#65 消费）

- Go 实体（`internal/types/public_marketplace_persistence.go`）：`PublicMarketplaceListingEntity` / `PublicReleaseSubmissionEntity` / `PublicReleaseReviewEntity` / `PublicAgentReleaseEntity` / `VerifiedPublisherEntity` / `TenantIntroducedReleaseEntity`。
- 仓储接口 `repository.PublicMarketplaceRepository`（`internal/application/repository/public_marketplace.go`）：
  - `VerifyPublisher(ctx, row *types.VerifiedPublisherEntity) (*types.VerifiedPublisherEntity, bool, error)`（bool=是否新建；revoked→verified 幂等回升）
  - `RevokePublisher(ctx, tenantID uint64) error`（miss→`ErrPublicMarketplaceNotFound`）
  - `ListVerifiedPublishers(ctx) ([]types.VerifiedPublisherEntity, error)` / `GetVerifiedPublisher(ctx, tenantID uint64) (*types.VerifiedPublisherEntity, error)`
  - `CreatePublicSubmission(ctx, listing *types.PublicMarketplaceListingEntity, submission *types.PublicReleaseSubmissionEntity) (*types.PublicReleaseSubmissionEntity, error)`
  - `ListPublicSubmissions(ctx, publisherTenantID uint64) ([]types.PublicReleaseSubmissionEntity, error)` / `ListPublicReviewQueue(ctx) ([]types.PublicReleaseSubmissionEntity, error)` / `GetPublicSubmission(ctx, submissionID string) (*types.PublicReleaseSubmissionEntity, error)`
  - `GetPublicListing(ctx, listingID string) (*types.PublicMarketplaceListingEntity, error)` / `GetPublicRelease(ctx, releaseID string) (*types.PublicAgentReleaseEntity, error)`
  - `ReviewAndPublishPublicTx(ctx, expectedPriorReleaseID, submissionID, expectedDigest string, decision types.AgentReleaseReviewDecision) (*types.PublicReleaseReviewEntity, *types.PublicAgentReleaseEntity, error)`
  - `ListPublicCatalog(ctx) ([]PublicCatalogRow, error)`（`PublicCatalogRow{Listing types.PublicMarketplaceListingEntity; Release *types.PublicAgentReleaseEntity}`）
  - `IntroduceRelease(ctx, adopterTenantID uint64, actorID string, listing *types.PublicMarketplaceListingEntity, release *types.PublicAgentReleaseEntity) (*types.TenantIntroducedReleaseEntity, *types.AgentAdoptionEntity, bool, error)`（bool=是否新引入）
- 服务接口 `interfaces.PublicMarketplaceService`（`internal/types/interfaces/public_marketplace.go`，视图 `VerifiedPublisherView` / `PublicSubmissionView` / `PublicReviewResult` / `PublicCatalogEntryView` / `PublicListingDetailView` / `PublicAdoptionResult{Introduction types.TenantIntroducedReleaseEntity; Adoption types.AgentAdoptionEntity}`）：
  - `VerifyPublisher(ctx, actorID string, tenantID uint64, note string) (VerifiedPublisherView, bool, error)` / `RevokePublisher(ctx, actorID string, tenantID uint64) error` / `ListVerifiedPublishers(ctx) ([]VerifiedPublisherView, error)`
  - `SubmitPublicRelease(ctx, tenantID uint64, actorID, sourceListingID, releaseID string) (PublicSubmissionView, error)`
  - `ListPublicSubmissions(ctx, tenantID uint64) ([]PublicSubmissionView, error)` / `ListPublicReviewQueue(ctx) ([]PublicSubmissionView, error)`
  - `ReviewPublicSubmission(ctx, reviewerID, submissionID, expectedDigest string, decision types.AgentReleaseReviewDecision) (PublicReviewResult, error)`
  - `ListPublicCatalog(ctx) ([]PublicCatalogEntryView, error)` / `GetPublicListing(ctx, listingID string) (*PublicListingDetailView, error)`
  - `AdoptPublicListing(ctx, tenantID uint64, actorID, listingID, releaseID string) (PublicAdoptionResult, bool, error)`
- 构造器：`service.NewPublicMarketplaceService(repo repository.PublicMarketplaceRepository, listings interfaces.AgentMarketplaceRepository) *PublicMarketplaceService`；`repository.NewPublicMarketplaceRepository(db *gorm.DB) PublicMarketplaceRepository`；`handler.NewPublicMarketplaceHandler(public interfaces.PublicMarketplaceService) *PublicMarketplaceHandler`；路由 `RegisterPublicMarketplaceRoutes(r *gin.RouterGroup, publicHandler *handler.PublicMarketplaceHandler, g *rbacGuards)`。
- HTTP wire（`/api/v1/marketplace/public/*`）：`GET /catalog`（Viewer+）、`GET /listings/:id`（Viewer+）、`POST /listings/:id/adopt`（Admin+）、`POST|GET /release-submissions`（Admin+，GET 仅本租户提交）、`GET /release-submissions/review-queue`、`POST /release-submissions/:id/review`、`GET|POST /verified-publishers`、`DELETE /verified-publishers/:tenant_id`（后五者 SystemAdmin，不对 API key 声明）。
- contracts：`@weknora/contracts` 根导出 `VerifiedPublisher/PublicCatalogListing/PublicCatalogRelease/PublicReleaseSubmission/PublicReleaseReview/PublicAgentRelease/PublicIntroduction/PublicAdoption/AdoptPublicListingResult` 类型与 `parseVerifiedPublisherResponse/parseVerifiedPublisherListResponse/parsePublicCatalogListResponse/parsePublicListingResponse/parsePublicSubmissionResponse/parsePublicSubmissionListResponse/parsePublicReviewResponse/parseAdoptPublicListingResponse` 解析器（`packages/contracts/src/marketplace/public-marketplace.ts`）。
- 数据面（供 #61 升级对比、#64 撤回阻断、#65 信任信号）：`public_agent_releases`（不可变、digest 唯一）、`tenant_introduced_releases(tenant_id, public_release_id)` 台账、`public_marketplace_listings.current_release_id` 指针、`public_marketplace_verified_publishers.state`。

## 差异记录（调查结论 vs 代码现状，以代码现状为准）

1. **#59 已合并**：调查缺口称「依赖 #59 的 Adoption 机制本身 absent」。代码现状：#59 已完整合并（HEAD fb5f6653a，`internal/application/service/agent_adoption.go` 等 12 个文件在库，git log 含 `merge: integrate #59 (b3)`）。本计划直接消费其接口，不存在「无法形成端到端验证路径」的问题。
2. **HEAD 迁移破窗（Task 1 修复）**：`migrations/versioned/` 同时存在 `000191_task_grants.*` 与 `000191_agent_adoption_variants.*`，`migrations/sqlite/` 同时存在 `000112_task_grants.*` 与 `000112_agent_adoption_variants.*`（#42 与 #59 并行批次各自占用同一版本号）。本计划作者实测 `go test ./internal/database/ -count=1` 与 marketplace 路由测试均因 `duplicate migration file` 失败。修复=纯改名（内容逐字节不变）：#59 的 adoption 迁移让位为 versioned 000192 / sqlite 000113（#42 先合并，保留原号）。已应用于任一分支数据库的迁移版本号回放问题属部署侧手工事项，不在本计划范围。
3. **跨租户 Release 落地形态（Task 2 设计决策）**：调查只指出缺口未给设计。本计划选择「平台级公共表 + 采用方台账 `tenant_introduced_releases` + 放宽 `agent_adoptions`/`agent_adoption_variants` 三条 FK」，理由：`agent_releases` 的 FK 链（→ `agent_release_submissions` → `agent_versions`）使跨租户 Release 无法物化为本地 `agent_releases` 行，伪造 submission/agent_versions 行会制造假 lineage；而 #59 全链对 listing/release 的唯一读口是 `agentAdoptionRepository.GetMarketplaceListing/GetRelease`，在其内部加台账回退即可让 Variant/映射/测试/发布原样工作。租户隔离仍由每条查询的 `tenant_id` 绑定保证（FK 放宽不放宽任何跨租户读）。
4. **「平台允许的 Tenant」**：spec §2 将公共目录可见范围定义为「平台允许的 Tenant」。首版=本部署内全部已认证租户可读（无按租户允许/屏蔽清单）；该清单不在本 Issue 验收标准内，未实现、未静默替代。
5. **ListTenantCatalog 不聚合公共来源**：采用方 `GET /marketplace/tenant/catalog`（#58）维持只显示本租户自有 listing；引入关系经 `GET /marketplace/tenant/adoptions`（#59）可见。三来源目录聚合展示属 #65（目录信任信号）范围。
6. **发布者度量**：spec §12 列出「去标识化的 Adoption……指标」为首版信任信号，但其展示面属 #65；本计划的公共目录读模型结构性不含任何 adoption 计数字段（Task 7 严格解码钉死），即 AC2 在本计划内以「零采用方可观测面」满足。
7. **sqlite 000114 需要生产入口 NoTxWrap 三段式（Task 2 Step 4）**：计划初版误以为 sqlite 迁移流已全局 NoTxWrap——代码现状是生产入口 `internal/database/migration.go` 只对 v55（000055 workbench rebuild，`:196-218`）做 NoTxWrap 三段式，其余文件 per-file 事务包装；golang-migrate v4.19.1 非 NoTxWrap 时整文件包在 `BEGIN...COMMIT` 里，SQLite 事务内 `PRAGMA foreign_keys` 是 no-op，000114 的表重建（`DROP TABLE agent_adoptions`，被 `agent_adoption_variants` 引用）会在 FK-on 部署（先例 `cmd/connector-control/main.go:122` DSN `_foreign_keys=on`）带存量行时 FK violation。修复=镜像 v55：000114 文件自带 `PRAGMA OFF → BEGIN IMMEDIATE → … → COMMIT → PRAGMA ON`（v55 同款），migration.go 为其新增独立的 NoTxWrap 三段式块。**如实声明的残留限制**：生产 down 路径（`scripts/migrate.sh:75-77` 裸 migrate CLI）没有 NoTxWrap 机制，000114 down（与既有 000055 down 先例相同）在该路径下 PRAGMA 同样不生效——down 场景属运维回滚流程，不在本计划代码范围内解决；本地与 CI 的 down 覆盖（`TestSemanticMigrationSQLiteUpDownUp` 回放）走 `newSQLiteMigrator(..., true)` NoTxWrap 驱动，PRAGMA 真实生效。仓储测试覆盖边界（`uq_agent_adoptions_scope` 唯一索引、OnConflict 竞态分支）的如实声明见 Task 3 Step 2 与 Task 4 Step 1。

---

### Task 1: 迁移版本号去重（HEAD 破窗修复）

**Files:**
- Rename: `migrations/versioned/000191_agent_adoption_variants.up.sql` → `migrations/versioned/000192_agent_adoption_variants.up.sql`
- Rename: `migrations/versioned/000191_agent_adoption_variants.down.sql` → `migrations/versioned/000192_agent_adoption_variants.down.sql`
- Rename: `migrations/sqlite/000112_agent_adoption_variants.up.sql` → `migrations/sqlite/000113_agent_adoption_variants.up.sql`
- Rename: `migrations/sqlite/000112_agent_adoption_variants.down.sql` → `migrations/sqlite/000113_agent_adoption_variants.down.sql`
- Test: `internal/database/`（既有测试，无新文件）

**Interfaces:**
- Consumes: 无（纯仓库修复）。
- Produces: 双流迁移版本号唯一（versioned 下一可用号 000193、sqlite 下一可用号 000114，Task 2 占用）；所有依赖 sqlite 迁移流的测试（`openTenantAgentMarketplaceHTTPTestDB`、`openAgentVersionServiceTestDB`）恢复可跑。

- [ ] **Step 1: 运行迁移测试确认失败（RED）**

Run: `go test ./internal/database/ -count=1`
Expected: FAIL —— `TestSQLiteMigrationsIncludeAutoTagConfig`、`TestSQLiteMigrationsCreateVersionedSchema`（"SQLite fixture has duplicate version 112"）、`TestSemanticMigrationSQLiteUpDownUp`、`TestWorkbenchSQLite*` 因 `duplicate migration file: 000112_task_grants.down.sql` 失败（本计划作者已在 HEAD 实测同一输出）。

- [ ] **Step 2: 改名四个迁移文件（内容逐字节不变）**

```bash
git mv migrations/versioned/000191_agent_adoption_variants.up.sql migrations/versioned/000192_agent_adoption_variants.up.sql
git mv migrations/versioned/000191_agent_adoption_variants.down.sql migrations/versioned/000192_agent_adoption_variants.down.sql
git mv migrations/sqlite/000112_agent_adoption_variants.up.sql migrations/sqlite/000113_agent_adoption_variants.up.sql
git mv migrations/sqlite/000112_agent_adoption_variants.down.sql migrations/sqlite/000113_agent_adoption_variants.down.sql
```

注释更新：各文件首行注释 `-- T29 Tenant Adoption...` 与 `-- SQLite twin of versioned migration 000191.` 中引用的旧号改为 `000192`（仅注释文字，SQL 不动）。

- [ ] **Step 3: 运行迁移测试确认通过（GREEN）**

Run: `go test ./internal/database/ -count=1`
Expected: PASS（ok github.com/Tencent/WeKnora/internal/database）。

- [ ] **Step 4: 复跑依赖迁移流的既有回归**

Run: `go test ./internal/router/ -run 'TestTenantAgentMarketplaceLifecycleAndAuthorization|TestTenantAgentAdoptionRoutesAndAuthorization' -count=1`
Expected: PASS（此前因 duplicate migration file 失败的两个 #58/#59 回归恢复）。

- [ ] **Step 5: Commit**

```bash
git add migrations/versioned/ migrations/sqlite/
git commit -m "fix(migrations): dedupe adoption migration version (000191->000192 / 000112->000113) broken by b3 merge"
```

---

### Task 2: Public Marketplace 持久化层（六张新表 + 三条 FK 放宽）

**Files:**
- Create: `migrations/versioned/000193_public_agent_marketplace.up.sql`
- Create: `migrations/versioned/000193_public_agent_marketplace.down.sql`
- Create: `migrations/sqlite/000114_public_agent_marketplace.up.sql`
- Create: `migrations/sqlite/000114_public_agent_marketplace.down.sql`
- Create: `internal/types/public_marketplace_persistence.go`
- Modify: `internal/database/migration.go:27`（新增版本常量）、`:110-116`（新增 presence 检查）、`:196-218`（v55 NoTxWrap 三段式块之后插入 000114 同款三段式）
- Modify: `internal/database/migration_sqlite_versioned_schema_test.go:22-49`（`versionedSQLiteTables` 追加六个表名）
- Test: `internal/database/migration_sqlite_versioned_schema_test.go`（既有，改表清单；其 up 阶段走生产入口 `RunMigrationsWithOptions`，因此同时覆盖新增的 NoTxWrap 三段式）

**Interfaces:**
- Consumes: Task 1 的唯一版本号（versioned 000193 / sqlite 000114）；`migrations/sqlite/000113_agent_adoption_variants.up.sql`（重建所依据的原表 DDL）；`internal/database/migration.go` 的 v55（000055 workbench rebuild）NoTxWrap 三段式先例（`:191-218`）与 `newSQLiteMigrator(migrationsPath, dsn, table, noTxWrap)`（`:329-337`）。
- Produces: 表 `public_marketplace_verified_publishers` / `public_marketplace_listings` / `public_release_submissions` / `public_release_reviews` / `public_agent_releases` / `tenant_introduced_releases`；`agent_adoptions` 与 `agent_adoption_variants` 上指向 `agent_marketplace_listings`/`agent_releases` 的三条 FK 被移除（#59 治理链可引用平台 lineage 身份）；生产 sqlite 迁移入口对 000114 单文件 NoTxWrap 执行（PRAGMA 在事务外生效）；实体 `types.PublicMarketplaceListingEntity` / `types.PublicReleaseSubmissionEntity` / `types.PublicReleaseReviewEntity` / `types.PublicAgentReleaseEntity` / `types.VerifiedPublisherEntity` / `types.TenantIntroducedReleaseEntity`（Task 3-8 消费）。

- [ ] **Step 1: 在迁移对齐测试中登记六张新表（RED）**

修改 `internal/database/migration_sqlite_versioned_schema_test.go`，在 `versionedSQLiteTables` 切片的 `"agent_variant_capability_mappings",` 之后追加：

```go
	"public_marketplace_verified_publishers",
	"public_marketplace_listings",
	"public_release_submissions",
	"public_release_reviews",
	"public_agent_releases",
	"tenant_introduced_releases",
```

Run: `go test ./internal/database/ -run 'TestSQLiteMigrationsCreateVersionedSchema' -count=1`
Expected: FAIL —— 报六张表缺失（迁移尚未创建）。

- [ ] **Step 2: 写 versioned（PostgreSQL）迁移**

创建 `migrations/versioned/000193_public_agent_marketplace.up.sql`：

```sql
-- T30 Public Marketplace: platform-scope listings/submissions/reviews/releases,
-- the Verified Publisher registry and the adopter-side introduction ledger.
-- Cross-tenant Adoptions reference platform-lineage ids (the public listing
-- and the introduced release), so the three FKs from the #59 governance
-- tables to agent_marketplace_listings/agent_releases are relaxed here;
-- tenant isolation stays enforced by every query's tenant_id binding.
CREATE TABLE public_marketplace_verified_publishers (
 tenant_id BIGINT PRIMARY KEY,
 state VARCHAR(32) NOT NULL DEFAULT 'verified',
 verified_by VARCHAR(255) NOT NULL DEFAULT '',
 note TEXT NOT NULL DEFAULT '',
 verified_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE public_marketplace_listings (
 id VARCHAR(36) PRIMARY KEY,
 publisher_tenant_id BIGINT NOT NULL,
 source_listing_id VARCHAR(36) NOT NULL,
 display_name VARCHAR(255) NOT NULL,
 summary TEXT NOT NULL DEFAULT '',
 state VARCHAR(32) NOT NULL DEFAULT 'listed',
 current_release_id VARCHAR(36),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE (publisher_tenant_id, source_listing_id)
);
CREATE TABLE public_release_submissions (
 id VARCHAR(36) PRIMARY KEY,
 publisher_tenant_id BIGINT NOT NULL,
 public_listing_id VARCHAR(36) NOT NULL,
 source_listing_id VARCHAR(36) NOT NULL,
 source_release_id VARCHAR(36) NOT NULL,
 publisher_actor_id VARCHAR(255) NOT NULL DEFAULT '',
 semantic_version VARCHAR(64) NOT NULL,
 bundle_digest VARCHAR(64) NOT NULL,
 manifest_json TEXT NOT NULL,
 dependency_lock_json TEXT NOT NULL,
 bundle BYTEA NOT NULL,
 status VARCHAR(32) NOT NULL DEFAULT 'submitted',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 FOREIGN KEY (public_listing_id) REFERENCES public_marketplace_listings(id),
 FOREIGN KEY (publisher_tenant_id, source_release_id) REFERENCES agent_releases(tenant_id, id)
);
CREATE TABLE public_release_reviews (
 id VARCHAR(36) PRIMARY KEY,
 submission_id VARCHAR(36) NOT NULL,
 reviewer_id VARCHAR(255) NOT NULL,
 reviewed_digest VARCHAR(64) NOT NULL,
 decision VARCHAR(32) NOT NULL,
 reason TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 FOREIGN KEY (submission_id) REFERENCES public_release_submissions(id)
);
CREATE TABLE public_agent_releases (
 id VARCHAR(36) PRIMARY KEY,
 listing_id VARCHAR(36) NOT NULL,
 submission_id VARCHAR(36) NOT NULL,
 publisher_tenant_id BIGINT NOT NULL,
 release_number INTEGER NOT NULL CHECK (release_number >= 1),
 semantic_version VARCHAR(64) NOT NULL,
 bundle_digest VARCHAR(64) NOT NULL,
 manifest_json TEXT NOT NULL,
 dependency_lock_json TEXT NOT NULL,
 bundle BYTEA NOT NULL,
 published_by VARCHAR(255) NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 FOREIGN KEY (listing_id) REFERENCES public_marketplace_listings(id),
 FOREIGN KEY (submission_id) REFERENCES public_release_submissions(id)
);
ALTER TABLE public_marketplace_listings ADD CONSTRAINT fk_public_marketplace_current_release
 FOREIGN KEY (current_release_id) REFERENCES public_agent_releases(id);
CREATE INDEX idx_public_release_submissions_queue ON public_release_submissions(status, created_at);
CREATE INDEX idx_public_release_reviews_submission ON public_release_reviews(submission_id, created_at);
CREATE INDEX idx_public_agent_releases_listing ON public_agent_releases(listing_id, release_number);
CREATE UNIQUE INDEX uq_public_release_review_submission ON public_release_reviews(submission_id);
CREATE UNIQUE INDEX uq_public_agent_releases_number ON public_agent_releases(listing_id, release_number);
CREATE UNIQUE INDEX uq_public_agent_releases_semantic ON public_agent_releases(listing_id, semantic_version);
CREATE UNIQUE INDEX uq_public_agent_releases_digest ON public_agent_releases(listing_id, bundle_digest);
CREATE TABLE tenant_introduced_releases (
 id VARCHAR(36) NOT NULL,
 tenant_id BIGINT NOT NULL,
 public_listing_id VARCHAR(36) NOT NULL,
 public_release_id VARCHAR(36) NOT NULL,
 display_name VARCHAR(255) NOT NULL,
 summary TEXT NOT NULL DEFAULT '',
 semantic_version VARCHAR(64) NOT NULL,
 bundle_digest VARCHAR(64) NOT NULL,
 manifest_json TEXT NOT NULL,
 dependency_lock_json TEXT NOT NULL,
 bundle BYTEA NOT NULL,
 introduced_by VARCHAR(255) NOT NULL DEFAULT '',
 introduced_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY (id, tenant_id),
 FOREIGN KEY (public_release_id) REFERENCES public_agent_releases(id),
 FOREIGN KEY (public_listing_id) REFERENCES public_marketplace_listings(id)
);
CREATE UNIQUE INDEX uq_tenant_introduced_release_scope ON tenant_introduced_releases(tenant_id, public_release_id);
DO $$
DECLARE c text;
BEGIN
	SELECT conname INTO c FROM pg_constraint WHERE conrelid = 'agent_adoptions'::regclass AND pg_get_constraintdef(oid) ILIKE '%REFERENCES agent_marketplace_listings%';
	IF c IS NOT NULL THEN EXECUTE format('ALTER TABLE agent_adoptions DROP CONSTRAINT %I', c); END IF;
	SELECT conname INTO c FROM pg_constraint WHERE conrelid = 'agent_adoptions'::regclass AND pg_get_constraintdef(oid) ILIKE '%REFERENCES agent_releases%';
	IF c IS NOT NULL THEN EXECUTE format('ALTER TABLE agent_adoptions DROP CONSTRAINT %I', c); END IF;
	SELECT conname INTO c FROM pg_constraint WHERE conrelid = 'agent_adoption_variants'::regclass AND pg_get_constraintdef(oid) ILIKE '%REFERENCES agent_releases%';
	IF c IS NOT NULL THEN EXECUTE format('ALTER TABLE agent_adoption_variants DROP CONSTRAINT %I', c); END IF;
END $$;
```

创建 `migrations/versioned/000193_public_agent_marketplace.down.sql`：

```sql
-- Down: rebuild the relaxed FKs (fails if platform-lineage Adoption rows
-- exist, which is expected for a down migration over live data) and drop
-- the public marketplace tables in reverse dependency order.
ALTER TABLE agent_adoption_variants ADD CONSTRAINT fk_agent_adoption_variants_release
 FOREIGN KEY (release_id, tenant_id) REFERENCES agent_releases(id, tenant_id);
ALTER TABLE agent_adoptions ADD CONSTRAINT fk_agent_adoptions_release
 FOREIGN KEY (accepted_release_id, tenant_id) REFERENCES agent_releases(id, tenant_id);
ALTER TABLE agent_adoptions ADD CONSTRAINT fk_agent_adoptions_listing
 FOREIGN KEY (listing_id, tenant_id) REFERENCES agent_marketplace_listings(id, tenant_id);
DROP TABLE IF EXISTS tenant_introduced_releases;
DROP TABLE IF EXISTS public_agent_releases;
DROP TABLE IF EXISTS public_release_reviews;
DROP TABLE IF EXISTS public_release_submissions;
DROP TABLE IF EXISTS public_marketplace_listings;
DROP TABLE IF EXISTS public_marketplace_verified_publishers;
```

- [ ] **Step 3: 写 sqlite 孪生迁移（含 FK 放宽的表重建）**

创建 `migrations/sqlite/000114_public_agent_marketplace.up.sql`：

```sql
-- SQLite twin of versioned migration 000193.
--
-- This migration owns its transaction and MUST run with the driver's
-- NoTxWrap: PRAGMA foreign_keys only changes outside a transaction, and the
-- agent_adoptions/agent_adoption_variants rebuild below (DROP TABLE on a
-- referenced parent) would raise FK violations on live data if the PRAGMAs
-- were swallowed by a per-file transaction wrapper. The production entry
-- internal/database/migration.go runs exactly this file through the
-- dedicated NoTxWrap three-phase block added alongside it (mirroring the
-- 000055 workbench rebuild precedent); test harnesses that hand the whole
-- stream a NoTxWrap driver are equally safe.
PRAGMA foreign_keys = OFF;
BEGIN IMMEDIATE;

CREATE TABLE public_marketplace_verified_publishers (
 tenant_id INTEGER PRIMARY KEY,
 state VARCHAR(32) NOT NULL DEFAULT 'verified',
 verified_by VARCHAR(255) NOT NULL DEFAULT '',
 note TEXT NOT NULL DEFAULT '',
 verified_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE public_marketplace_listings (
 id VARCHAR(36) PRIMARY KEY,
 publisher_tenant_id INTEGER NOT NULL,
 source_listing_id VARCHAR(36) NOT NULL,
 display_name VARCHAR(255) NOT NULL,
 summary TEXT NOT NULL DEFAULT '',
 state VARCHAR(32) NOT NULL DEFAULT 'listed',
 current_release_id VARCHAR(36),
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE (publisher_tenant_id, source_listing_id)
);
CREATE TABLE public_release_submissions (
 id VARCHAR(36) PRIMARY KEY,
 publisher_tenant_id INTEGER NOT NULL,
 public_listing_id VARCHAR(36) NOT NULL,
 source_listing_id VARCHAR(36) NOT NULL,
 source_release_id VARCHAR(36) NOT NULL,
 publisher_actor_id VARCHAR(255) NOT NULL DEFAULT '',
 semantic_version VARCHAR(64) NOT NULL,
 bundle_digest VARCHAR(64) NOT NULL,
 manifest_json TEXT NOT NULL,
 dependency_lock_json TEXT NOT NULL,
 bundle BLOB NOT NULL,
 status VARCHAR(32) NOT NULL DEFAULT 'submitted',
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 FOREIGN KEY (public_listing_id) REFERENCES public_marketplace_listings(id),
 FOREIGN KEY (publisher_tenant_id, source_release_id) REFERENCES agent_releases(tenant_id, id)
);
CREATE TABLE public_release_reviews (
 id VARCHAR(36) PRIMARY KEY,
 submission_id VARCHAR(36) NOT NULL,
 reviewer_id VARCHAR(255) NOT NULL,
 reviewed_digest VARCHAR(64) NOT NULL,
 decision VARCHAR(32) NOT NULL,
 reason TEXT NOT NULL DEFAULT '',
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 FOREIGN KEY (submission_id) REFERENCES public_release_submissions(id)
);
CREATE TABLE public_agent_releases (
 id VARCHAR(36) PRIMARY KEY,
 listing_id VARCHAR(36) NOT NULL,
 submission_id VARCHAR(36) NOT NULL,
 publisher_tenant_id INTEGER NOT NULL,
 release_number INTEGER NOT NULL CHECK (release_number >= 1),
 semantic_version VARCHAR(64) NOT NULL,
 bundle_digest VARCHAR(64) NOT NULL,
 manifest_json TEXT NOT NULL,
 dependency_lock_json TEXT NOT NULL,
 bundle BLOB NOT NULL,
 published_by VARCHAR(255) NOT NULL DEFAULT '',
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 FOREIGN KEY (listing_id) REFERENCES public_marketplace_listings(id),
 FOREIGN KEY (submission_id) REFERENCES public_release_submissions(id)
);
CREATE INDEX idx_public_release_submissions_queue ON public_release_submissions(status, created_at);
CREATE INDEX idx_public_release_reviews_submission ON public_release_reviews(submission_id, created_at);
CREATE INDEX idx_public_agent_releases_listing ON public_agent_releases(listing_id, release_number);
CREATE UNIQUE INDEX uq_public_release_review_submission ON public_release_reviews(submission_id);
CREATE UNIQUE INDEX uq_public_agent_releases_number ON public_agent_releases(listing_id, release_number);
CREATE UNIQUE INDEX uq_public_agent_releases_semantic ON public_agent_releases(listing_id, semantic_version);
CREATE UNIQUE INDEX uq_public_agent_releases_digest ON public_agent_releases(listing_id, bundle_digest);
CREATE TABLE tenant_introduced_releases (
 id VARCHAR(36) NOT NULL,
 tenant_id INTEGER NOT NULL,
 public_listing_id VARCHAR(36) NOT NULL,
 public_release_id VARCHAR(36) NOT NULL,
 display_name VARCHAR(255) NOT NULL,
 summary TEXT NOT NULL DEFAULT '',
 semantic_version VARCHAR(64) NOT NULL,
 bundle_digest VARCHAR(64) NOT NULL,
 manifest_json TEXT NOT NULL,
 dependency_lock_json TEXT NOT NULL,
 bundle BLOB NOT NULL,
 introduced_by VARCHAR(255) NOT NULL DEFAULT '',
 introduced_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY (id, tenant_id),
 FOREIGN KEY (public_release_id) REFERENCES public_agent_releases(id),
 FOREIGN KEY (public_listing_id) REFERENCES public_marketplace_listings(id)
);
CREATE UNIQUE INDEX uq_tenant_introduced_release_scope ON tenant_introduced_releases(tenant_id, public_release_id);
-- FK relaxation: sqlite cannot DROP CONSTRAINT, so rebuild both #59 tables
-- from their 000113 DDL minus the dropped FKs. The PRAGMAs at the file top
-- are only effective because internal/database/migration.go executes THIS
-- file through the dedicated NoTxWrap three-phase block (Task 2 Step 4);
-- under a per-file transaction wrapper they would be silent no-ops and the
-- DROP TABLE below would break FK-on deployments carrying adoption rows.
CREATE TABLE agent_adoptions_rebuild (
 id VARCHAR(36) NOT NULL, tenant_id INTEGER NOT NULL, listing_id VARCHAR(36) NOT NULL,
 accepted_release_id VARCHAR(36) NOT NULL, state VARCHAR(32) NOT NULL DEFAULT 'active',
 created_by VARCHAR(255) NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY (id, tenant_id)
);
INSERT INTO agent_adoptions_rebuild (id, tenant_id, listing_id, accepted_release_id, state, created_by, created_at, updated_at)
 SELECT id, tenant_id, listing_id, accepted_release_id, state, created_by, created_at, updated_at FROM agent_adoptions;
DROP TABLE agent_adoptions;
ALTER TABLE agent_adoptions_rebuild RENAME TO agent_adoptions;
CREATE UNIQUE INDEX uq_agent_adoptions_scope ON agent_adoptions(tenant_id, listing_id);
CREATE TABLE agent_adoption_variants_rebuild (
 id VARCHAR(36) NOT NULL, tenant_id INTEGER NOT NULL, adoption_id VARCHAR(36) NOT NULL, release_id VARCHAR(36) NOT NULL,
 name VARCHAR(255) NOT NULL, state VARCHAR(32) NOT NULL DEFAULT 'draft',
 local_agent_id VARCHAR(36), local_agent_version_id VARCHAR(36),
 created_by VARCHAR(255) NOT NULL DEFAULT '', tested_by VARCHAR(255) NOT NULL DEFAULT '', tested_at DATETIME,
 published_by VARCHAR(255) NOT NULL DEFAULT '', published_at DATETIME,
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY (id, tenant_id),
 FOREIGN KEY (adoption_id, tenant_id) REFERENCES agent_adoptions(id, tenant_id)
);
INSERT INTO agent_adoption_variants_rebuild (id, tenant_id, adoption_id, release_id, name, state, local_agent_id, local_agent_version_id, created_by, tested_by, tested_at, published_by, published_at, created_at, updated_at)
 SELECT id, tenant_id, adoption_id, release_id, name, state, local_agent_id, local_agent_version_id, created_by, tested_by, tested_at, published_by, published_at, created_at, updated_at FROM agent_adoption_variants;
DROP TABLE agent_adoption_variants;
ALTER TABLE agent_adoption_variants_rebuild RENAME TO agent_adoption_variants;
CREATE INDEX idx_agent_adoption_variants_adoption ON agent_adoption_variants(tenant_id, adoption_id, created_at);
COMMIT;
PRAGMA foreign_keys = ON;
```

创建 `migrations/sqlite/000114_public_agent_marketplace.down.sql`：

```sql
-- Down twin: restore the #59 FKs by rebuilding both tables with their
-- original 000113 DDL, then drop the public marketplace tables. Like the up
-- twin this file owns its transaction and requires the driver's NoTxWrap
-- (PRAGMA foreign_keys only changes outside a transaction). Restoring the
-- FKs fails on live data when platform-lineage Adoption rows exist — the
-- same expectation documented on the versioned twin.
PRAGMA foreign_keys = OFF;
BEGIN IMMEDIATE;
DROP TABLE IF EXISTS tenant_introduced_releases;
DROP TABLE IF EXISTS public_agent_releases;
DROP TABLE IF EXISTS public_release_reviews;
DROP TABLE IF EXISTS public_release_submissions;
DROP TABLE IF EXISTS public_marketplace_listings;
DROP TABLE IF EXISTS public_marketplace_verified_publishers;
CREATE TABLE agent_adoptions_restore (
 id VARCHAR(36) NOT NULL, tenant_id INTEGER NOT NULL, listing_id VARCHAR(36) NOT NULL,
 accepted_release_id VARCHAR(36) NOT NULL, state VARCHAR(32) NOT NULL DEFAULT 'active',
 created_by VARCHAR(255) NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY (id, tenant_id),
 FOREIGN KEY (listing_id, tenant_id) REFERENCES agent_marketplace_listings(id, tenant_id),
 FOREIGN KEY (accepted_release_id, tenant_id) REFERENCES agent_releases(id, tenant_id)
);
INSERT INTO agent_adoptions_restore (id, tenant_id, listing_id, accepted_release_id, state, created_by, created_at, updated_at)
 SELECT id, tenant_id, listing_id, accepted_release_id, state, created_by, created_at, updated_at FROM agent_adoptions;
DROP TABLE agent_adoptions;
ALTER TABLE agent_adoptions_restore RENAME TO agent_adoptions;
CREATE UNIQUE INDEX uq_agent_adoptions_scope ON agent_adoptions(tenant_id, listing_id);
CREATE TABLE agent_adoption_variants_restore (
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
INSERT INTO agent_adoption_variants_restore (id, tenant_id, adoption_id, release_id, name, state, local_agent_id, local_agent_version_id, created_by, tested_by, tested_at, published_by, published_at, created_at, updated_at)
 SELECT id, tenant_id, adoption_id, release_id, name, state, local_agent_id, local_agent_version_id, created_by, tested_by, tested_at, published_by, published_at, created_at, updated_at FROM agent_adoption_variants;
DROP TABLE agent_adoption_variants;
ALTER TABLE agent_adoption_variants_restore RENAME TO agent_adoption_variants;
CREATE INDEX idx_agent_adoption_variants_adoption ON agent_adoption_variants(tenant_id, adoption_id, created_at);
COMMIT;
PRAGMA foreign_keys = ON;
```

- [ ] **Step 4: 为 sqlite 000114 在生产迁移入口启用 NoTxWrap 三段式（镜像 v55 先例）**

生产入口 `internal/database/migration.go` 目前只对 v55（000055 workbench rebuild，`:196-218`）启用 NoTxWrap 三段式，其余 sqlite 迁移走 per-file 事务包装（`:123` 与 `:214` 的 `newSQLiteMigrator(..., false)`）。golang-migrate v4.19.1 在非 NoTxWrap 时以 `tx.Begin()+tx.Exec` 执行整个迁移文件，而 SQLite 在事务内执行 `PRAGMA foreign_keys` 是 no-op——000114 的 `DROP TABLE agent_adoptions` 在 FK-on 部署（真实先例 `cmd/connector-control/main.go:122` 的 DSN 带 `_foreign_keys=on`；`sqliteMigrationDSN` `:287-314` 只剥 `x-` 参数、保留 `_foreign_keys=on`）且存在 #59 adoption/variant 存量行时会 FK violation。镜像 v55 三段式修复。

修改 `internal/database/migration.go:27`（常量段，`sqliteWorkbenchRunsMigrationVersion = 55` 之后追加）：

```go
// sqliteAdoptionFKRelaxationMigrationVersion is the sqlite twin of the
// public-marketplace adoption FK relaxation (000114). Like the workbench
// rebuild (v55) that file owns its transaction and PRAGMAs, so it must run
// with the driver's NoTxWrap while every other file keeps per-file wrapping.
const sqliteAdoptionFKRelaxationMigrationVersion = 114
```

修改 `internal/database/migration.go:110-116`（presence 检查段，整段替换为：）

```go
	workbenchSQLiteMigrationPresent := false
	adoptionFKRelaxationSQLiteMigrationPresent := false
	isSQLite := strings.HasPrefix(dsn, "sqlite3://")
	if isSQLite {
		migrationsPath = "file://migrations/sqlite"
		_, err := os.Stat("migrations/sqlite/000055_workbench_runs.up.sql")
		workbenchSQLiteMigrationPresent = err == nil
		_, err = os.Stat("migrations/sqlite/000114_public_agent_marketplace.up.sql")
		adoptionFKRelaxationSQLiteMigrationPresent = err == nil
	}
```

在 v55 三段式块（`:196-218`，`// Run all pending migrations` 注释之前）插入：

```go
	// Migration 000114 relaxes the three #59 adoption FKs via table rebuild.
	// Same constraint as v55: PRAGMA foreign_keys only changes outside a
	// transaction, so this one file runs NoTxWrap while every other
	// migration keeps its per-file transaction.
	if isSQLite && adoptionFKRelaxationSQLiteMigrationPresent &&
		(versionErr == migrate.ErrNilVersion || oldVersion < sqliteAdoptionFKRelaxationMigrationVersion) {
		if err := m.Migrate(sqliteAdoptionFKRelaxationMigrationVersion - 1); err != nil && err != migrate.ErrNoChange {
			return captureMigrationFailure(m, fmt.Errorf("failed to prepare SQLite adoption FK relaxation: %w", err))
		}
		if _, err := m.Close(); err != nil {
			return fmt.Errorf("failed to close SQLite migration driver before adoption FK relaxation: %w", err)
		}
		m, err = newSQLiteMigrator(migrationsPath, sqliteMigrationDSN(dsn, opts), sqliteMigrationTable(dsn), true)
		if err != nil {
			return captureMigrationFailure(m, fmt.Errorf("failed to create SQLite adoption FK relaxation migrator: %w", err))
		}
		if err := m.Steps(1); err != nil && err != migrate.ErrNoChange {
			return captureMigrationFailure(m, fmt.Errorf("failed to run SQLite adoption FK relaxation migration: %w", err))
		}
		if _, err := m.Close(); err != nil {
			return fmt.Errorf("failed to close SQLite adoption FK relaxation migrator: %w", err)
		}
		m, err = newSQLiteMigrator(migrationsPath, sqliteMigrationDSN(dsn, opts), sqliteMigrationTable(dsn), false)
		if err != nil {
			return captureMigrationFailure(m, fmt.Errorf("failed to restore SQLite migration driver: %w", err))
		}
	}
```

语义说明：v55 块先执行并推进到 55，本块再推进到 114——两块对 fresh 库（`ErrNilVersion`）与任意 `oldVersion` 均收敛（`Migrate(113)` 幂等补齐到 113、`Steps(1)` 恰好应用 000114、`m.Up()` 应用其余）；presence 检查使 legacy v4 fixture 根（仅含 000000-000004，`TestSQLiteMigrationsUpgradeV4PreservesData` 的 `copySQLiteMigrationsV4`）自动跳过本块。如实声明：Go 测试不会直接暴露本修复的必要性（测试库为空库且 DSN 未带 `_foreign_keys=on`），其正确性依据是 `migration.go:191-195` 既有注释对「SQLite 只在事务外改变 foreign_keys，而 stock driver 给每个文件包事务」的同一结论与 v55 先例；生产 down 路径（`scripts/migrate.sh:75-77` 走裸 migrate CLI，无 NoTxWrap）与既有 000055 down 先例同样受限，已在差异记录第 7 条如实声明，不在本计划内解决。

- [ ] **Step 5: 写持久化实体**

创建 `internal/types/public_marketplace_persistence.go`：

```go
package types

import "time"

// Public Marketplace persistence entities (T30, Ticket #60).
//
// The platform owns the public catalog (Verified Publisher registry,
// public listings/submissions/reviews/releases); the adopter tenant owns
// its introduction ledger. Cross-tenant Adoptions reference the PUBLIC
// listing id and the introduced local release id, which is why the three
// FKs from agent_adoptions/agent_adoption_variants to the tenant
// marketplace tables were relaxed in migration 000193/000114 — tenant
// isolation remains enforced by every query's tenant_id binding.

// VerifiedPublisherEntity is the platform registry row: one tenant that
// may submit releases to the Public Marketplace (CONTEXT.md「Verified
// Publisher」). state: verified | revoked (revocation propagation is #64).
type VerifiedPublisherEntity struct {
	TenantID    uint64    `gorm:"primaryKey"`
	State       string    `gorm:"type:varchar(32);not null;default:'verified'"`
	VerifiedBy  string    `gorm:"type:varchar(255);not null;default:''"`
	Note        string    `gorm:"type:text;not null;default:''"`
	VerifiedAt  time.Time
	UpdatedAt   time.Time
}

func (VerifiedPublisherEntity) TableName() string { return "public_marketplace_verified_publishers" }

// PublicMarketplaceListingEntity is the public catalog identity derived
// from one publisher tenant listing (UNIQUE (publisher_tenant_id,
// source_listing_id)). state stays "listed" in this ticket; unlist/
// deprecate belong to #63, security revocation to #64.
type PublicMarketplaceListingEntity struct {
	ID                string  `gorm:"type:varchar(36);primaryKey"`
	PublisherTenantID uint64  `gorm:"not null"`
	SourceListingID   string  `gorm:"type:varchar(36);not null"`
	DisplayName       string  `gorm:"type:varchar(255);not null"`
	Summary           string  `gorm:"type:text;not null;default:''"`
	State             string  `gorm:"type:varchar(32);not null;default:'listed'"`
	CurrentReleaseID  *string `gorm:"type:varchar(36)"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (PublicMarketplaceListingEntity) TableName() string { return "public_marketplace_listings" }

// PublicReleaseSubmissionEntity is a Verified Publisher promoting one of
// its own immutable tenant releases for platform review. The bundle is a
// verbatim copy of the tenant release's portable bundle; the digest is
// re-verified server-side on both submit and review.
type PublicReleaseSubmissionEntity struct {
	ID                 string `gorm:"type:varchar(36);primaryKey"`
	PublisherTenantID  uint64 `gorm:"not null"`
	PublicListingID    string `gorm:"type:varchar(36);not null"`
	SourceListingID    string `gorm:"type:varchar(36);not null"`
	SourceReleaseID    string `gorm:"type:varchar(36);not null"`
	PublisherActorID   string `gorm:"type:varchar(255);not null;default:''"`
	SemanticVersion    string `gorm:"type:varchar(64);not null"`
	BundleDigest       string `gorm:"type:varchar(64);not null"`
	ManifestJSON       string `gorm:"type:text;not null"`
	DependencyLockJSON string `gorm:"type:text;not null"`
	Bundle             []byte `gorm:"type:blob;not null"`
	Status             string `gorm:"type:varchar(32);not null;default:'submitted'"`
	CreatedAt          time.Time
}

func (PublicReleaseSubmissionEntity) TableName() string { return "public_release_submissions" }

// PublicReleaseReviewEntity records the platform review decision exactly
// like the tenant review rows: reviewer, reviewed digest, decision, reason
// (spec §7 审核记录保留要求). One review decides a submission.
type PublicReleaseReviewEntity struct {
	ID             string    `gorm:"type:varchar(36);primaryKey"`
	SubmissionID   string    `gorm:"type:varchar(36);not null"`
	ReviewerID     string    `gorm:"type:varchar(255);not null"`
	ReviewedDigest string    `gorm:"type:varchar(64);not null"`
	Decision       string    `gorm:"type:varchar(32);not null"`
	Reason         string    `gorm:"type:text;not null;default:''"`
	CreatedAt      time.Time
}

func (PublicReleaseReviewEntity) TableName() string { return "public_release_reviews" }

// PublicAgentReleaseEntity is an approved, immutable public release. Like
// the tenant release it pins semantic version, digest, Manifest, Dependency
// Lock and the portable bundle bytes.
type PublicAgentReleaseEntity struct {
	ID                 string    `gorm:"type:varchar(36);primaryKey"`
	ListingID          string    `gorm:"type:varchar(36);not null"`
	SubmissionID       string    `gorm:"type:varchar(36);not null"`
	PublisherTenantID  uint64    `gorm:"not null"`
	ReleaseNumber      int       `gorm:"not null"`
	SemanticVersion    string    `gorm:"type:varchar(64);not null"`
	BundleDigest       string    `gorm:"type:varchar(64);not null"`
	ManifestJSON       string    `gorm:"type:text;not null"`
	DependencyLockJSON string    `gorm:"type:text;not null"`
	Bundle             []byte    `gorm:"type:blob;not null"`
	PublishedBy        string    `gorm:"type:varchar(255);not null;default:''"`
	CreatedAt          time.Time
}

func (PublicAgentReleaseEntity) TableName() string { return "public_agent_releases" }

// TenantIntroducedReleaseEntity is the adopter-side introduction ledger:
// the verbatim portable copy of ONE public release inside one tenant,
// unique per (tenant, public release). The #59 chain reads it through the
// adoption repository fallback; the Adoption references the public listing
// id and this row's local id.
type TenantIntroducedReleaseEntity struct {
	ID                 string `gorm:"type:varchar(36);primaryKey"`
	TenantID           uint64 `gorm:"primaryKey"`
	PublicListingID    string `gorm:"type:varchar(36);not null"`
	PublicReleaseID    string `gorm:"type:varchar(36);not null"`
	DisplayName        string `gorm:"type:varchar(255);not null"`
	Summary            string `gorm:"type:text;not null;default:''"`
	SemanticVersion    string `gorm:"type:varchar(64);not null"`
	BundleDigest       string `gorm:"type:varchar(64);not null"`
	ManifestJSON       string `gorm:"type:text;not null"`
	DependencyLockJSON string `gorm:"type:text;not null"`
	Bundle             []byte `gorm:"type:blob;not null"`
	IntroducedBy       string `gorm:"type:varchar(255);not null;default:''"`
	IntroducedAt       time.Time
}

func (TenantIntroducedReleaseEntity) TableName() string { return "tenant_introduced_releases" }
```

- [ ] **Step 6: 运行迁移测试确认通过（GREEN）**

Run: `go test ./internal/database/ -count=1`
Expected: PASS（含 `TestSQLiteMigrationsCreateVersionedSchema` 新表清单与 `TestSemanticMigrationSQLiteUpDownUp` 的 up/down/up 回放——down 迁移因此必须真实可逆，Step 2/3 已给出；`TestSQLiteMigrationsCreateVersionedSchema` 的 up 阶段与语义测试的重放 up 阶段跨越 114 时均会执行 Step 4 新增的三段式块，`TestSQLiteMigrationsUpgradeV4PreservesData` 的 legacy fixture 根经 presence 检查自动跳过）。

- [ ] **Step 7: Commit**

```bash
git add migrations/versioned/000193_public_agent_marketplace.up.sql migrations/versioned/000193_public_agent_marketplace.down.sql migrations/sqlite/000114_public_agent_marketplace.up.sql migrations/sqlite/000114_public_agent_marketplace.down.sql internal/types/public_marketplace_persistence.go internal/database/migration.go internal/database/migration_sqlite_versioned_schema_test.go
git commit -m "feat(marketplace): public marketplace tables, verified publisher registry and introduction ledger (T30 #60 task 2)"
```

---

### Task 3: Adoption 仓储引入回退 + adoptListingTx 抽取

**Files:**
- Modify: `internal/application/repository/agent_adoption.go:67-114`（AdoptListing 抽取 adoptListingTx/reconcileAdoptionTx）
- Modify: `internal/application/repository/agent_adoption.go:311-333`（GetMarketplaceListing/GetRelease 增加台账回退）
- Test: `internal/application/repository/agent_adoption_test.go`（追加测试）

**Interfaces:**
- Consumes: Task 2 的 `types.TenantIntroducedReleaseEntity`。
- Produces: 包内（不入接口）`adoptListingTx(tx *gorm.DB, adoption *types.AgentAdoptionEntity) (*types.AgentAdoptionEntity, bool, error)` 与 `reconcileAdoptionTx(tx *gorm.DB, existing *types.AgentAdoptionEntity, acceptedReleaseID string) (*types.AgentAdoptionEntity, bool, error)`（Task 4 `IntroduceRelease` 复用）；`AgentAdoptionRepository.GetMarketplaceListing/GetRelease` 行为扩展——本地行 miss 时回退 `tenant_introduced_releases` 合成实体（本地行优先），使 #59 全链对平台 lineage listing/release 可用（Task 5/7/8 消费）。

- [ ] **Step 1: 写失败测试**

在 `internal/application/repository/agent_adoption_test.go` 末尾追加（该文件已有 `context`/`require`/`types`/`gorm`/`sqlite`/`logger` 导入，若缺则补）：

```go
func TestAgentAdoptionRepositoryResolvesIntroducedListingAndRelease(t *testing.T) {
	db := openAdoptionVariantDB(t)
	require.NoError(t, db.AutoMigrate(&types.AgentMarketplaceListingEntity{}, &types.AgentReleaseEntity{}, &types.TenantIntroducedReleaseEntity{}))
	repo := NewAgentAdoptionRepository(db)
	ctx := context.Background()
	bundle := []byte(`{"payload":{"system_prompt":"portable"}}`)

	// 引入台账：tenant 2 引入了 public listing 的一个 release
	require.NoError(t, db.Create(&types.TenantIntroducedReleaseEntity{
		ID: "introduced-r1", TenantID: 2, PublicListingID: "pub-listing-1", PublicReleaseID: "pub-release-1",
		DisplayName: "Public helper", Summary: "s", SemanticVersion: "1.0.0", BundleDigest: "d1",
		ManifestJSON: `{"capability_requirements":["knowledge"]}`, DependencyLockJSON: `{"dependencies":[]}`,
		Bundle: bundle, IntroducedBy: "admin-2",
	}).Error)

	listing, err := repo.GetMarketplaceListing(ctx, 2, "pub-listing-1")
	require.NoError(t, err)
	require.NotNil(t, listing, "引入台账应合成为可采用的 listing")
	require.Equal(t, "listed", listing.State)
	require.NotNil(t, listing.CurrentReleaseID)
	require.Equal(t, "introduced-r1", *listing.CurrentReleaseID)

	release, err := repo.GetRelease(ctx, 2, "introduced-r1")
	require.NoError(t, err)
	require.NotNil(t, release, "引入台账应合成为可读的 release")
	require.Equal(t, "pub-listing-1", release.ListingID)
	require.Equal(t, bundle, release.Bundle)
	require.Equal(t, `{"capability_requirements":["knowledge"]}`, release.ManifestJSON)

	// 本地真实行优先：同 id 存在本地 release 时不得走回退
	require.NoError(t, db.Create(&types.AgentReleaseEntity{
		ID: "introduced-r1", TenantID: 2, ListingID: "local-listing", SubmissionID: "s1",
		AgentVersionID: "v1", SourceAgentID: "a1", ReleaseNumber: 1, SemanticVersion: "9.9.9",
		BundleDigest: "d1", ManifestJSON: "{}", DependencyLockJSON: "{}", Bundle: []byte("local"),
	}).Error)
	local, err := repo.GetRelease(ctx, 2, "introduced-r1")
	require.NoError(t, err)
	require.Equal(t, "local-listing", local.ListingID)

	// 租户隔离：tenant 1 看不到 tenant 2 的引入
	other, err := repo.GetMarketplaceListing(ctx, 1, "pub-listing-1")
	require.NoError(t, err)
	require.Nil(t, other)
	otherRelease, err := repo.GetRelease(ctx, 1, "introduced-r1")
	require.NoError(t, err)
	require.Nil(t, otherRelease)
}
```

Run: `go test ./internal/application/repository/ -run 'TestAgentAdoptionRepositoryResolvesIntroducedListingAndRelease' -count=1`
Expected: FAIL —— `listing` 为 nil（回退未实现），`require.NotNil(t, listing)` 处失败。

- [ ] **Step 2: 最小实现（回退 + 事务化重构）**

修改 `internal/application/repository/agent_adoption.go`。先把 `AdoptListing` 与 `reconcileAdoption` 改为事务友好的包级函数（行为不变，SQL 与参数逐字保留）：

```go
func (r *agentAdoptionRepository) AdoptListing(ctx context.Context, adoption *types.AgentAdoptionEntity) (*types.AgentAdoptionEntity, bool, error) {
	return adoptListingTx(r.db.WithContext(ctx), adoption)
}

// adoptListingTx is the transaction-bound adopt upsert, shared with the
// public-marketplace IntroduceRelease transaction (T30 #60). Semantics are
// identical to the pre-refactor AdoptListing: an existing (tenant,
// listing) row reconciles its accepted pointer; a first insert races on
// uq_agent_adoptions_scope and converges to the winner.
func adoptListingTx(tx *gorm.DB, adoption *types.AgentAdoptionEntity) (*types.AgentAdoptionEntity, bool, error) {
	var existing types.AgentAdoptionEntity
	err := tx.Where("tenant_id = ? AND listing_id = ?", adoption.TenantID, adoption.ListingID).First(&existing).Error
	if err == nil {
		return reconcileAdoptionTx(tx, &existing, adoption.AcceptedReleaseID)
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
	inserted := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&created)
	if inserted.Error != nil {
		return nil, false, inserted.Error
	}
	if inserted.RowsAffected == 1 {
		return &created, true, nil
	}
	// Lost the race to uq_agent_adoptions_scope: re-read the winner's row
	// and reconcile, exactly like a sequential re-adopt.
	var winner types.AgentAdoptionEntity
	if err := tx.Where("tenant_id = ? AND listing_id = ?", adoption.TenantID, adoption.ListingID).First(&winner).Error; err != nil {
		return nil, false, err
	}
	return reconcileAdoptionTx(tx, &winner, adoption.AcceptedReleaseID)
}

// reconcileAdoption is the shared existing-row path: an Adoption accepted at
// the same Release returns as-is; a different Release advances the accepted
// pointer (last write wins, matching sequential adopt semantics).
func reconcileAdoptionTx(tx *gorm.DB, existing *types.AgentAdoptionEntity, acceptedReleaseID string) (*types.AgentAdoptionEntity, bool, error) {
	if existing.AcceptedReleaseID == acceptedReleaseID {
		return existing, false, nil
	}
	existing.AcceptedReleaseID = acceptedReleaseID
	existing.UpdatedAt = time.Now().UTC()
	if err := tx.Model(&types.AgentAdoptionEntity{}).
		Where("tenant_id = ? AND id = ?", existing.TenantID, existing.ID).
		Updates(map[string]any{"accepted_release_id": existing.AcceptedReleaseID, "updated_at": existing.UpdatedAt}).Error; err != nil {
		return nil, false, err
	}
	return existing, false, nil
}
```

（原 `reconcileAdoption` 方法体整体删除，由 `reconcileAdoptionTx` 取代；文件头 import 不变。）

再给两个 marketplace 只读代理加台账回退（替换 `internal/application/repository/agent_adoption.go:311-333` 的两个方法）：

```go
func (r *agentAdoptionRepository) GetMarketplaceListing(ctx context.Context, tenantID uint64, listingID string) (*types.AgentMarketplaceListingEntity, error) {
	var row types.AgentMarketplaceListingEntity
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(listingID)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return introducedListing(r.db.WithContext(ctx), tenantID, listingID)
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// introducedListing synthesizes the adoptable Listing view of a public
// listing this tenant introduced (T30 #60): local rows win, the fallback
// resolves the LATEST introduced release of that public listing as the
// current release. Tenant-scoped like every other read.
func introducedListing(tx *gorm.DB, tenantID uint64, listingID string) (*types.AgentMarketplaceListingEntity, error) {
	var latest types.TenantIntroducedReleaseEntity
	err := tx.Where("tenant_id = ? AND public_listing_id = ?", tenantID, strings.TrimSpace(listingID)).
		Order("introduced_at DESC, id DESC").First(&latest).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	current := latest.ID
	return &types.AgentMarketplaceListingEntity{
		ID: latest.PublicListingID, TenantID: tenantID, SourceAgentID: latest.PublicListingID,
		DisplayName: latest.DisplayName, Summary: latest.Summary, State: "listed",
		CurrentReleaseID: &current,
	}, nil
}

func (r *agentAdoptionRepository) GetRelease(ctx context.Context, tenantID uint64, releaseID string) (*types.AgentReleaseEntity, error) {
	var row types.AgentReleaseEntity
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(releaseID)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return introducedRelease(r.db.WithContext(ctx), tenantID, releaseID)
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// introducedRelease synthesizes the Release view of one introduced public
// release: the #59 chain (CreateVariant gating, manifest reads, publish
// digest verification and payload decode) consumes it unchanged. The
// synthesized row deliberately keeps the portable content ONLY — there is
// no local submission/agent-version lineage, which is exactly the point of
// the introduction ledger.
func introducedRelease(tx *gorm.DB, tenantID uint64, releaseID string) (*types.AgentReleaseEntity, error) {
	var row types.TenantIntroducedReleaseEntity
	err := tx.Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(releaseID)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &types.AgentReleaseEntity{
		ID: row.ID, TenantID: tenantID, ListingID: row.PublicListingID,
		SemanticVersion: row.SemanticVersion, BundleDigest: row.BundleDigest,
		ManifestJSON: row.ManifestJSON, DependencyLockJSON: row.DependencyLockJSON,
		Bundle: row.Bundle, PublishedBy: row.IntroducedBy, CreatedAt: row.IntroducedAt,
	}, nil
}
```

同时在同一文件（`internal/application/repository/agent_adoption_test.go`）的 `openAdoptionVariantDB`（`:248-256`）AutoMigrate 之后补建生产唯一索引——实体 `types.AgentAdoptionEntity` 无 uniqueIndex tag，AutoMigrate 不会创建 `uq_agent_adoptions_scope`，没有它 `adoptListingTx` 的 `OnConflict{DoNothing}`+`RowsAffected==0` 竞态收敛分支就从未被真实唯一索引驱动：

```go
	// AutoMigrate 不创建 uq_agent_adoptions_scope（实体无 uniqueIndex tag）；
	// 显式补建使 adoptListingTx 的 OnConflict 竞态分支由真实唯一索引驱动，
	// 与迁移 000113/000114 的生产 DDL 一致。
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uq_agent_adoptions_scope ON agent_adoptions(tenant_id, listing_id)").Error)
```

- [ ] **Step 3: 运行新测试与 #59 仓储回归**

Run: `go test ./internal/application/repository/ -run 'TestAgentAdoption|TestReplaceCapabilityMappings' -count=1`
Expected: PASS（新测试 + 既有 AdoptListing 竞态收敛/CAS 回归全部通过——重构不改行为）。

- [ ] **Step 4: Commit**

```bash
git add internal/application/repository/agent_adoption.go internal/application/repository/agent_adoption_test.go
git commit -m "feat(marketplace): adoption repository resolves introduced public listings/releases; extract tx-bound adopt upsert (T30 #60 task 3)"
```

---

### Task 4: Public Marketplace 仓储

**Files:**
- Create: `internal/application/repository/public_marketplace.go`
- Test: `internal/application/repository/public_marketplace_test.go`

**Interfaces:**
- Consumes: Task 2 实体；Task 3 的 `adoptListingTx`（同包）；`isUniqueViolation`（`internal/application/repository/voice_session.go:289`，同包）。
- Produces: `PublicMarketplaceRepository` 接口 + `NewPublicMarketplaceRepository(db *gorm.DB)` + 错误 `ErrPublicMarketplaceNotFound` / `ErrPublicMarketplaceReviewConflict` / `ErrPublicMarketplaceInvalidDecision` / `ErrPublicMarketplacePointerConflict` / `ErrPublicMarketplaceDigestMismatch` / `ErrPublicMarketplaceReleaseConflict` + `PublicCatalogRow{Listing types.PublicMarketplaceListingEntity; Release *types.PublicAgentReleaseEntity}`（Task 5/6/7 消费；错误 Task 8 的 db 直查断言复用表名）。

- [ ] **Step 1: 写失败测试**

创建 `internal/application/repository/public_marketplace_test.go`：

```go
package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openPublicMarketplaceDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&types.AgentMarketplaceListingEntity{}, &types.AgentReleaseSubmissionEntity{},
		&types.AgentReleaseReviewEntity{}, &types.AgentReleaseEntity{},
		&types.VerifiedPublisherEntity{}, &types.PublicMarketplaceListingEntity{},
		&types.PublicReleaseSubmissionEntity{}, &types.PublicReleaseReviewEntity{},
		&types.PublicAgentReleaseEntity{}, &types.TenantIntroducedReleaseEntity{},
		&types.AgentAdoptionEntity{}, &types.AgentAdoptionVariantEntity{},
	))
	// AutoMigrate 不创建 uq_agent_adoptions_scope（实体无 uniqueIndex tag）；
	// 显式补建使 adoptListingTx/IntroduceRelease 的 OnConflict 竞态分支由
	// 真实唯一索引驱动，与迁移 000113/000114 的生产 DDL 一致。
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uq_agent_adoptions_scope ON agent_adoptions(tenant_id, listing_id)").Error)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func digestOf(bundle []byte) string { sum := sha256.Sum256(bundle); return hex.EncodeToString(sum[:]) }

// sourceReleaseNumber derives a stable per-version release_number so
// repeated seeds against the same listing never collide with the real
// uq_agent_releases_number(listing_id, release_number) unique index
// (migrations/sqlite/000109_tenant_agent_marketplace.up.sql) that the
// AutoMigrate schema of this test does NOT carry.
func sourceReleaseNumber(semanticVersion string) int {
	fields := strings.Split(semanticVersion, ".")
	major, err := strconv.Atoi(fields[0])
	if err != nil || major < 1 {
		return 1
	}
	return major
}

// seedApprovedPublicRelease seeds one REAL tenant source release (the
// CreatePublicSubmission tx verifies it exists in agent_releases) and walks
// it through public submission + platform approval.
func seedApprovedPublicRelease(t *testing.T, db *gorm.DB, semanticVersion string) (listing types.PublicMarketplaceListingEntity, release *types.PublicAgentReleaseEntity) {
	t.Helper()
	ctx := context.Background()
	repo := NewPublicMarketplaceRepository(db)
	bundle := []byte(`{"payload":{"system_prompt":"Be portable."},"manifest":{"semantic_version":"` + semanticVersion + `"},"dependency_lock":{"dependencies":[]}}`)
	sourceReleaseID := "tenant-release-" + semanticVersion
	require.NoError(t, db.Create(&types.AgentReleaseEntity{
		ID: sourceReleaseID, TenantID: 1, ListingID: "tenant-listing-1", SubmissionID: "tenant-submission-" + semanticVersion,
		AgentVersionID: "version-x-" + semanticVersion, SourceAgentID: "agent-a", ReleaseNumber: sourceReleaseNumber(semanticVersion), SemanticVersion: semanticVersion,
		BundleDigest: digestOf(bundle), ManifestJSON: `{}`, DependencyLockJSON: `{"dependencies":[]}`, Bundle: bundle,
	}).Error)
	listing = types.PublicMarketplaceListingEntity{PublisherTenantID: 1, SourceListingID: "tenant-listing-1", DisplayName: "Public helper", Summary: "s", State: "listed"}
	submission, err := repo.CreatePublicSubmission(ctx, &listing, &types.PublicReleaseSubmissionEntity{
		PublisherTenantID: 1, SourceListingID: "tenant-listing-1", SourceReleaseID: sourceReleaseID,
		PublisherActorID: "publisher-admin", SemanticVersion: semanticVersion, BundleDigest: digestOf(bundle),
		ManifestJSON: `{}`, DependencyLockJSON: `{"dependencies":[]}`, Bundle: bundle, Status: "submitted",
	})
	require.NoError(t, err)
	_, created, err := repo.ReviewAndPublishPublicTx(ctx, "", submission.ID, digestOf(bundle), types.AgentReleaseReviewDecision{ReviewerID: "platform-reviewer", Decision: "approved"})
	require.NoError(t, err)
	require.NotNil(t, created)
	loaded, err := repo.GetPublicListing(ctx, created.ListingID)
	require.NoError(t, err)
	return *loaded, created
}

func TestPublicMarketplaceRepositoryVerifyPublisherLifecycle(t *testing.T) {
	db := openPublicMarketplaceDB(t)
	repo := NewPublicMarketplaceRepository(db)
	ctx := context.Background()

	created, isFirst, err := repo.VerifyPublisher(ctx, &types.VerifiedPublisherEntity{TenantID: 7, VerifiedBy: "sysadmin", Note: "identity checked"})
	require.NoError(t, err)
	require.True(t, isFirst)
	require.Equal(t, "verified", created.State)

	again, isFirstRepeat, err := repo.VerifyPublisher(ctx, &types.VerifiedPublisherEntity{TenantID: 7, VerifiedBy: "sysadmin"})
	require.NoError(t, err)
	require.False(t, isFirstRepeat)
	require.Equal(t, "verified", again.State)

	require.NoError(t, repo.RevokePublisher(ctx, 7))
	row, err := repo.GetVerifiedPublisher(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, "revoked", row.State)

	revived, revivedFirst, err := repo.VerifyPublisher(ctx, &types.VerifiedPublisherEntity{TenantID: 7, VerifiedBy: "sysadmin-2"})
	require.NoError(t, err)
	require.False(t, revivedFirst, "重验证是更新既有行，不是新建")
	require.Equal(t, "verified", revived.State)

	require.ErrorIs(t, repo.RevokePublisher(ctx, 42), ErrPublicMarketplaceNotFound)
}

func TestPublicMarketplaceRepositoryReviewAndPublishAdvancesPointerOnce(t *testing.T) {
	db := openPublicMarketplaceDB(t)
	repo := NewPublicMarketplaceRepository(db)
	ctx := context.Background()
	listing, release := seedApprovedPublicRelease(t, db, "1.0.0")
	require.NotNil(t, listing.CurrentReleaseID)
	require.Equal(t, release.ID, *listing.CurrentReleaseID)
	require.Equal(t, 1, release.ReleaseNumber)

	// 同一 submission 不得被二次审核决定（唯一 review 约束收敛为显式冲突）
	_, _, err := repo.ReviewAndPublishPublicTx(ctx, "", release.SubmissionID, release.BundleDigest, types.AgentReleaseReviewDecision{ReviewerID: "platform-reviewer-2", Decision: "rejected", Reason: "too risky"})
	require.ErrorIs(t, err, ErrPublicMarketplaceReviewConflict)

	// digest 与 submission 不一致必须拒绝
	bundle := []byte(`{"payload":{"system_prompt":"v2"},"manifest":{"semantic_version":"2.0.0"},"dependency_lock":{"dependencies":[]}}`)
	require.NoError(t, db.Create(&types.AgentReleaseEntity{
		ID: "tenant-release-2.0.0", TenantID: 1, ListingID: "tenant-listing-1", SubmissionID: "tenant-submission-2",
		AgentVersionID: "version-x-2.0.0", SourceAgentID: "agent-a", ReleaseNumber: 2, SemanticVersion: "2.0.0",
		BundleDigest: digestOf(bundle), ManifestJSON: `{}`, DependencyLockJSON: `{"dependencies":[]}`, Bundle: bundle,
	}).Error)
	sub2, err := repo.CreatePublicSubmission(ctx, nil, &types.PublicReleaseSubmissionEntity{
		PublisherTenantID: 1, PublicListingID: listing.ID, SourceListingID: "tenant-listing-1", SourceReleaseID: "tenant-release-2.0.0",
		SemanticVersion: "2.0.0", BundleDigest: digestOf(bundle), ManifestJSON: `{}`, DependencyLockJSON: `{"dependencies":[]}`, Bundle: bundle, Status: "submitted",
	})
	require.NoError(t, err)
	_, _, err = repo.ReviewAndPublishPublicTx(ctx, *listing.CurrentReleaseID, sub2.ID, "0"+"1", types.AgentReleaseReviewDecision{ReviewerID: "r", Decision: "approved"})
	require.ErrorIs(t, err, ErrPublicMarketplaceDigestMismatch)

	// 指针 CAS：prior 指针错误必须拒绝
	_, _, err = repo.ReviewAndPublishPublicTx(ctx, "", sub2.ID, digestOf(bundle), types.AgentReleaseReviewDecision{ReviewerID: "r", Decision: "approved"})
	require.ErrorIs(t, err, ErrPublicMarketplacePointerConflict)

	_, release2, err := repo.ReviewAndPublishPublicTx(ctx, *listing.CurrentReleaseID, sub2.ID, digestOf(bundle), types.AgentReleaseReviewDecision{ReviewerID: "r", Decision: "approved"})
	require.NoError(t, err)
	require.Equal(t, 2, release2.ReleaseNumber)
}

func TestPublicMarketplaceRepositoryIntroduceReleaseCopiesPortableBundleAndAdopts(t *testing.T) {
	db := openPublicMarketplaceDB(t)
	repo := NewPublicMarketplaceRepository(db)
	ctx := context.Background()
	listing, release := seedApprovedPublicRelease(t, db, "1.0.0")

	introduced, adoption, created, err := repo.IntroduceRelease(ctx, 2, "adopter-admin", &listing, release)
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, release.Bundle, introduced.Bundle, "引入=可移植 Release 逐字节复制")
	require.Equal(t, release.BundleDigest, introduced.BundleDigest)
	require.Equal(t, uint64(2), introduced.TenantID)
	require.NotNil(t, adoption)
	require.Equal(t, listing.ID, adoption.ListingID, "Adoption 引用公共 Listing 身份")
	require.Equal(t, introduced.ID, adoption.AcceptedReleaseID)
	require.Equal(t, "active", adoption.State)

	// 同一 (tenant, public release) 幂等：二次引入不新建行、不新建 Adoption
	again, againAdoption, againCreated, err := repo.IntroduceRelease(ctx, 2, "adopter-admin", &listing, release)
	require.NoError(t, err)
	require.False(t, againCreated)
	require.Equal(t, introduced.ID, again.ID)
	require.Equal(t, adoption.ID, againAdoption.ID)

	// 引入更新的公共 release：同一 Adoption，accepted 指针前进
	listing2, release2 := seedApprovedPublicRelease(t, db, "2.0.0")
	require.Equal(t, listing.ID, listing2.ID)
	_, advanced, advancedCreated, err := repo.IntroduceRelease(ctx, 2, "adopter-admin", &listing2, release2)
	require.NoError(t, err)
	require.True(t, advancedCreated)
	require.Equal(t, adoption.ID, advanced.ID, "Adoption 唯一，不产生第二个")
	require.Equal(t, release2.ID, advanced.AcceptedReleaseID)

	// 其他租户互不可见
	var count int64
	db.Table("tenant_introduced_releases").Where("tenant_id = ?", 3).Count(&count)
	require.Zero(t, count)
}
```

覆盖边界如实声明（Review Focus 第 5 条的精确含义）：补建唯一索引后，本任务测试对 `IntroduceRelease` 的「幂等 / 指针前进」断言走的是 `adoptListingTx` 内 **First 命中的顺序路径**（不是 OnConflict 丢失竞态分支）；竞态收敛分支本身由 #59 既有测试 `TestAgentAdoptionRepositoryAdoptListingLostRaceConverges` / `TestAgentAdoptionRepositoryAdoptListingLostRaceDifferentRelease`（真实迁移流 + 真实唯一索引 + gorm 查询回调注入竞胜者）经共享的 `adoptListingTx` 继续覆盖——Task 3 的重构不改变这两条测试驱动的代码路径。`IntroduceRelease` 自身对 introduced 行的丢失竞态分支（`OnConflict` `RowsAffected==0` 回读 winner）未被直接测试（仅覆盖顺序收敛），已知边界、如实记录，不声称覆盖。

Run: `go test ./internal/application/repository/ -run 'TestPublicMarketplaceRepository' -count=1`
Expected: FAIL —— 编译错误 `undefined: NewPublicMarketplaceRepository`。

- [ ] **Step 2: 写仓储实现**

创建 `internal/application/repository/public_marketplace.go`：

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
	"gorm.io/gorm/clause"

	"github.com/Tencent/WeKnora/internal/types"
)

var (
	ErrPublicMarketplaceNotFound        = errors.New("public marketplace resource not found")
	ErrPublicMarketplaceInvalidDecision = errors.New("invalid public marketplace review decision")
	ErrPublicMarketplaceReviewConflict  = errors.New("public marketplace submission already has a review")
	ErrPublicMarketplaceDigestMismatch  = errors.New("public marketplace reviewed digest does not match submission")
	ErrPublicMarketplacePointerConflict = errors.New("public marketplace listing pointer changed")
	ErrPublicMarketplaceReleaseConflict = errors.New("public marketplace release conflict")
)

// PublicCatalogRow pairs a discoverable public listing with its current
// release. It deliberately carries NO adopter-derived data (spec §12).
type PublicCatalogRow struct {
	Listing types.PublicMarketplaceListingEntity
	Release *types.PublicAgentReleaseEntity
}

// PublicMarketplaceRepository owns the public catalog and introduction
// ledger SQL. Platform tables are not tenant-scoped; the adopter-side
// introduction ledger always binds the adopter tenant id. Every statement
// is parameter-bound.
type PublicMarketplaceRepository interface {
	VerifyPublisher(ctx context.Context, row *types.VerifiedPublisherEntity) (*types.VerifiedPublisherEntity, bool, error)
	RevokePublisher(ctx context.Context, tenantID uint64) error
	ListVerifiedPublishers(ctx context.Context) ([]types.VerifiedPublisherEntity, error)
	GetVerifiedPublisher(ctx context.Context, tenantID uint64) (*types.VerifiedPublisherEntity, error)
	CreatePublicSubmission(ctx context.Context, listing *types.PublicMarketplaceListingEntity, submission *types.PublicReleaseSubmissionEntity) (*types.PublicReleaseSubmissionEntity, error)
	ListPublicSubmissions(ctx context.Context, publisherTenantID uint64) ([]types.PublicReleaseSubmissionEntity, error)
	ListPublicReviewQueue(ctx context.Context) ([]types.PublicReleaseSubmissionEntity, error)
	GetPublicSubmission(ctx context.Context, submissionID string) (*types.PublicReleaseSubmissionEntity, error)
	GetPublicListing(ctx context.Context, listingID string) (*types.PublicMarketplaceListingEntity, error)
	GetPublicRelease(ctx context.Context, releaseID string) (*types.PublicAgentReleaseEntity, error)
	ReviewAndPublishPublicTx(ctx context.Context, expectedPriorReleaseID, submissionID, expectedDigest string, decision types.AgentReleaseReviewDecision) (*types.PublicReleaseReviewEntity, *types.PublicAgentReleaseEntity, error)
	ListPublicCatalog(ctx context.Context) ([]PublicCatalogRow, error)
	IntroduceRelease(ctx context.Context, adopterTenantID uint64, actorID string, listing *types.PublicMarketplaceListingEntity, release *types.PublicAgentReleaseEntity) (*types.TenantIntroducedReleaseEntity, *types.AgentAdoptionEntity, bool, error)
}

type publicMarketplaceRepository struct{ db *gorm.DB }

func NewPublicMarketplaceRepository(db *gorm.DB) PublicMarketplaceRepository {
	return &publicMarketplaceRepository{db: db}
}

func (r *publicMarketplaceRepository) VerifyPublisher(ctx context.Context, row *types.VerifiedPublisherEntity) (*types.VerifiedPublisherEntity, bool, error) {
	if row == nil || row.TenantID == 0 {
		return nil, false, fmt.Errorf("invalid verified publisher row")
	}
	var existing types.VerifiedPublisherEntity
	err := r.db.WithContext(ctx).Where("tenant_id = ?", row.TenantID).First(&existing).Error
	if err == nil {
		if existing.State == "verified" && existing.VerifiedBy == row.VerifiedBy {
			return &existing, false, nil
		}
		existing.State = "verified"
		existing.VerifiedBy = row.VerifiedBy
		existing.Note = row.Note
		existing.VerifiedAt = row.VerifiedAt
		existing.UpdatedAt = time.Now().UTC()
		if err := r.db.WithContext(ctx).Model(&types.VerifiedPublisherEntity{}).Where("tenant_id = ?", row.TenantID).
			Updates(map[string]any{"state": existing.State, "verified_by": existing.VerifiedBy, "note": existing.Note, "verified_at": existing.VerifiedAt, "updated_at": existing.UpdatedAt}).Error; err != nil {
			return nil, false, err
		}
		return &existing, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	created := *row
	if created.State == "" {
		created.State = "verified"
	}
	created.UpdatedAt = created.VerifiedAt
	inserted := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&created)
	if inserted.Error != nil {
		return nil, false, inserted.Error
	}
	if inserted.RowsAffected == 1 {
		return &created, true, nil
	}
	var winner types.VerifiedPublisherEntity
	if err := r.db.WithContext(ctx).Where("tenant_id = ?", row.TenantID).First(&winner).Error; err != nil {
		return nil, false, err
	}
	return &winner, false, nil
}

func (r *publicMarketplaceRepository) RevokePublisher(ctx context.Context, tenantID uint64) error {
	updated := r.db.WithContext(ctx).Model(&types.VerifiedPublisherEntity{}).
		Where("tenant_id = ?", tenantID).
		Updates(map[string]any{"state": "revoked", "updated_at": time.Now().UTC()})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return ErrPublicMarketplaceNotFound
	}
	return nil
}

func (r *publicMarketplaceRepository) ListVerifiedPublishers(ctx context.Context) ([]types.VerifiedPublisherEntity, error) {
	rows := []types.VerifiedPublisherEntity{}
	err := r.db.WithContext(ctx).Order("verified_at ASC, tenant_id ASC").Find(&rows).Error
	return rows, err
}

func (r *publicMarketplaceRepository) GetVerifiedPublisher(ctx context.Context, tenantID uint64) (*types.VerifiedPublisherEntity, error) {
	var row types.VerifiedPublisherEntity
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *publicMarketplaceRepository) CreatePublicSubmission(ctx context.Context, listing *types.PublicMarketplaceListingEntity, submission *types.PublicReleaseSubmissionEntity) (*types.PublicReleaseSubmissionEntity, error) {
	if submission == nil || submission.PublisherTenantID == 0 || strings.TrimSpace(submission.SourceListingID) == "" || strings.TrimSpace(submission.SourceReleaseID) == "" || strings.TrimSpace(submission.BundleDigest) == "" || len(submission.Bundle) == 0 {
		return nil, fmt.Errorf("invalid public marketplace submission")
	}
	var result types.PublicReleaseSubmissionEntity
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var source types.AgentReleaseEntity
		if err := tx.Where("tenant_id = ? AND id = ?", submission.PublisherTenantID, submission.SourceReleaseID).First(&source).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPublicMarketplaceNotFound
			}
			return err
		}
		var row types.PublicMarketplaceListingEntity
		err := tx.Where("publisher_tenant_id = ? AND source_listing_id = ?", submission.PublisherTenantID, submission.SourceListingID).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if listing == nil {
				return fmt.Errorf("public listing details required for first submission")
			}
			row = types.PublicMarketplaceListingEntity{
				ID: uuid.NewString(), PublisherTenantID: submission.PublisherTenantID, SourceListingID: submission.SourceListingID,
				DisplayName: listing.DisplayName, Summary: listing.Summary, State: "listed",
				CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		result = *submission
		result.ID, result.PublicListingID = uuid.NewString(), row.ID
		if result.Status == "" {
			result.Status = "submitted"
		}
		result.CreatedAt = time.Now().UTC()
		return tx.Create(&result).Error
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *publicMarketplaceRepository) ListPublicSubmissions(ctx context.Context, publisherTenantID uint64) ([]types.PublicReleaseSubmissionEntity, error) {
	rows := []types.PublicReleaseSubmissionEntity{}
	err := r.db.WithContext(ctx).Where("publisher_tenant_id = ?", publisherTenantID).Order("created_at ASC, id ASC").Find(&rows).Error
	return rows, err
}

func (r *publicMarketplaceRepository) ListPublicReviewQueue(ctx context.Context) ([]types.PublicReleaseSubmissionEntity, error) {
	rows := []types.PublicReleaseSubmissionEntity{}
	err := r.db.WithContext(ctx).
		Where("status IN ?", []string{"submitted", "in_review"}).
		Where("NOT EXISTS (SELECT 1 FROM public_release_reviews reviews WHERE reviews.submission_id = public_release_submissions.id)").
		Order("created_at ASC, id ASC").Find(&rows).Error
	return rows, err
}

func (r *publicMarketplaceRepository) GetPublicSubmission(ctx context.Context, submissionID string) (*types.PublicReleaseSubmissionEntity, error) {
	var row types.PublicReleaseSubmissionEntity
	err := r.db.WithContext(ctx).Where("id = ?", strings.TrimSpace(submissionID)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *publicMarketplaceRepository) GetPublicListing(ctx context.Context, listingID string) (*types.PublicMarketplaceListingEntity, error) {
	var row types.PublicMarketplaceListingEntity
	err := r.db.WithContext(ctx).Where("id = ?", strings.TrimSpace(listingID)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *publicMarketplaceRepository) GetPublicRelease(ctx context.Context, releaseID string) (*types.PublicAgentReleaseEntity, error) {
	var row types.PublicAgentReleaseEntity
	err := r.db.WithContext(ctx).Where("id = ?", strings.TrimSpace(releaseID)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *publicMarketplaceRepository) ReviewAndPublishPublicTx(ctx context.Context, expectedPriorReleaseID, submissionID, expectedDigest string, decision types.AgentReleaseReviewDecision) (*types.PublicReleaseReviewEntity, *types.PublicAgentReleaseEntity, error) {
	if decision.Decision != "approved" && decision.Decision != "rejected" && decision.Decision != "changes_requested" {
		return nil, nil, ErrPublicMarketplaceInvalidDecision
	}
	var review *types.PublicReleaseReviewEntity
	var release *types.PublicAgentReleaseEntity
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var submission types.PublicReleaseSubmissionEntity
		if err := tx.Where("id = ?", strings.TrimSpace(submissionID)).First(&submission).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPublicMarketplaceNotFound
			}
			return err
		}
		if submission.BundleDigest != expectedDigest {
			return ErrPublicMarketplaceDigestMismatch
		}
		if submission.Status != "submitted" && submission.Status != "in_review" {
			return ErrPublicMarketplaceInvalidDecision
		}
		review = &types.PublicReleaseReviewEntity{ID: uuid.NewString(), SubmissionID: submission.ID, ReviewerID: decision.ReviewerID, ReviewedDigest: expectedDigest, Decision: decision.Decision, Reason: decision.Reason, CreatedAt: time.Now().UTC()}
		if err := tx.Create(review).Error; err != nil {
			return err
		}
		if decision.Decision != "approved" {
			return nil
		}
		var max int
		if err := tx.Model(&types.PublicAgentReleaseEntity{}).Where("listing_id = ?", submission.PublicListingID).Select("COALESCE(MAX(release_number), 0)").Scan(&max).Error; err != nil {
			return err
		}
		release = &types.PublicAgentReleaseEntity{
			ID: uuid.NewString(), ListingID: submission.PublicListingID, SubmissionID: submission.ID,
			PublisherTenantID: submission.PublisherTenantID, ReleaseNumber: max + 1,
			SemanticVersion: submission.SemanticVersion, BundleDigest: submission.BundleDigest,
			ManifestJSON: submission.ManifestJSON, DependencyLockJSON: submission.DependencyLockJSON,
			Bundle: append([]byte(nil), submission.Bundle...), PublishedBy: decision.ReviewerID, CreatedAt: time.Now().UTC(),
		}
		if err := tx.Create(release).Error; err != nil {
			return err
		}
		query := tx.Model(&types.PublicMarketplaceListingEntity{}).Where("id = ?", submission.PublicListingID)
		if expectedPriorReleaseID == "" {
			query = query.Where("current_release_id IS NULL")
		} else {
			query = query.Where("current_release_id = ?", expectedPriorReleaseID)
		}
		updated := query.Updates(map[string]any{"current_release_id": release.ID, "updated_at": time.Now().UTC()})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrPublicMarketplacePointerConflict
		}
		return nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, nil, ErrPublicMarketplaceReviewConflict
		}
		return nil, nil, err
	}
	return review, release, nil
}

func (r *publicMarketplaceRepository) ListPublicCatalog(ctx context.Context) ([]PublicCatalogRow, error) {
	rows := []types.PublicMarketplaceListingEntity{}
	if err := r.db.WithContext(ctx).Where("state = ? AND current_release_id IS NOT NULL", "listed").Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]PublicCatalogRow, 0, len(rows))
	for i := range rows {
		release, err := r.GetPublicRelease(ctx, *rows[i].CurrentReleaseID)
		if err != nil {
			return nil, err
		}
		out = append(out, PublicCatalogRow{Listing: rows[i], Release: release})
	}
	return out, nil
}

// IntroduceRelease is the cross-tenant propagation primitive: it copies ONE
// approved public release's portable content verbatim into the adopter
// tenant's introduction ledger and creates/updates the tenant's Adoption
// for that public listing, all in one transaction. Concurrent introduces of
// the same (tenant, public release) converge on the winner's row.
func (r *publicMarketplaceRepository) IntroduceRelease(ctx context.Context, adopterTenantID uint64, actorID string, listing *types.PublicMarketplaceListingEntity, release *types.PublicAgentReleaseEntity) (*types.TenantIntroducedReleaseEntity, *types.AgentAdoptionEntity, bool, error) {
	var introduced types.TenantIntroducedReleaseEntity
	var adoption *types.AgentAdoptionEntity
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Where("tenant_id = ? AND public_release_id = ?", adopterTenantID, release.ID).First(&introduced).Error
		if err == nil {
			adoption, _, err = adoptListingTx(tx, &types.AgentAdoptionEntity{
				TenantID: adopterTenantID, ListingID: listing.ID, AcceptedReleaseID: introduced.ID,
				State: "active", CreatedBy: actorID,
			})
			return err
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		introduced = types.TenantIntroducedReleaseEntity{
			ID: uuid.NewString(), TenantID: adopterTenantID, PublicListingID: listing.ID, PublicReleaseID: release.ID,
			DisplayName: listing.DisplayName, Summary: listing.Summary, SemanticVersion: release.SemanticVersion,
			BundleDigest: release.BundleDigest, ManifestJSON: release.ManifestJSON,
			DependencyLockJSON: release.DependencyLockJSON, Bundle: append([]byte(nil), release.Bundle...),
			IntroducedBy:       actorID, IntroducedAt: time.Now().UTC(),
		}
		inserted := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&introduced)
		if inserted.Error != nil {
			return inserted.Error
		}
		if inserted.RowsAffected == 1 {
			created = true
		} else {
			var winner types.TenantIntroducedReleaseEntity
			if err := tx.Where("tenant_id = ? AND public_release_id = ?", adopterTenantID, release.ID).First(&winner).Error; err != nil {
				return err
			}
			introduced = winner
		}
		adoption, _, err = adoptListingTx(tx, &types.AgentAdoptionEntity{
			TenantID: adopterTenantID, ListingID: listing.ID, AcceptedReleaseID: introduced.ID,
			State: "active", CreatedBy: actorID,
		})
		return err
	})
	if err != nil {
		return nil, nil, false, err
	}
	return &introduced, adoption, created, nil
}
```

- [ ] **Step 3: 运行仓储测试确认通过（GREEN）**

Run: `go test ./internal/application/repository/ -run 'TestPublicMarketplaceRepository' -count=1`
Expected: PASS。

- [ ] **Step 4: Commit**

```bash
git add internal/application/repository/public_marketplace.go internal/application/repository/public_marketplace_test.go
git commit -m "feat(marketplace): public marketplace repository with platform review CAS and cross-tenant introduction tx (T30 #60 task 4)"
```

---

### Task 5: Public Marketplace 服务（Verified Publisher 门禁 + 提交 + 平台审核 + 引入）

**Files:**
- Create: `internal/types/interfaces/public_marketplace.go`
- Create: `internal/application/service/public_marketplace.go`
- Test: `internal/application/service/public_marketplace_test.go`

**Interfaces:**
- Consumes: Task 4 的 `repository.PublicMarketplaceRepository` 与错误；#58 的 `interfaces.AgentMarketplaceRepository.GetListing/GetRelease`（无回退源读）；同包 `validMarketplaceDigest`（`internal/application/service/agent_marketplace.go:218`）；Task 3 的 adoption 回退（经 `repository.NewAgentAdoptionRepository`，链式验证消费）。
- Produces: `interfaces.PublicMarketplaceService` 全部方法与视图（见计划头部 Produces；Task 6/7/8 消费）；服务错误 `service.ErrPublicMarketplaceInvalidInput` / `ErrPublicMarketplaceNotFound` / `ErrPublicMarketplaceNotVerifiedPublisher` / `ErrPublicMarketplaceStaleDigest`；`service.NewPublicMarketplaceService(repo repository.PublicMarketplaceRepository, listings interfaces.AgentMarketplaceRepository) *PublicMarketplaceService`。

- [ ] **Step 1: 写失败测试**

创建 `internal/application/service/public_marketplace_test.go`：

```go
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newPublicMarketplaceServiceForTest(t *testing.T) (*PublicMarketplaceService, *gorm.DB) {
	t.Helper()
	db := openAgentVersionServiceTestDB(t)
	listings := repository.NewAgentMarketplaceRepository(db)
	return NewPublicMarketplaceService(repository.NewPublicMarketplaceRepository(db), listings), db
}

// differentDigest flips the last hex char of a valid digest, yielding a
// DIFFERENT value that still passes the sha-256 format check.
func differentDigest(digest string) string {
	last := digest[len(digest)-1]
	replacement := "0"
	if last == '0' {
		replacement = "1"
	}
	return digest[:len(digest)-1] + replacement
}

// seedTenantRelease publishes a REAL tenant release (tenant 1) whose
// Manifest requires the capability "knowledge", mirroring the #59 service
// test seeding (including the agent_versions row CreateSubmission verifies).
func seedTenantRelease(t *testing.T, db *gorm.DB) (listingID, releaseID, digest string) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO agent_versions (id, tenant_id, agent_id, version_number, snapshot, source_sha256, frozen_by) VALUES ('version-a', 1, 'agent-a', 1, '{}', 'sha', 'publisher-admin')`,
	).Error)
	manifest := `{"semantic_version":"1.0.0","display_name":"Public helper","summary":"Portable","supported_languages":["en"],"use_cases":["support"],"capability_requirements":["knowledge"],"minimum_weknora_capability":"1","license_id":"MIT","source":{"agent_version_id":"version-a","version_number":1,"source_sha256":"sha"}}`
	bundle := []byte(`{"payload":{"agent_mode":"smart-reasoning","system_prompt":"Be portable.","allowed_tools":[]},"manifest":` + manifest + `,"dependency_lock":{"dependencies":[]}}`)
	sum := sha256.Sum256(bundle)
	digest = hex.EncodeToString(sum[:])
	repo := repository.NewAgentMarketplaceRepository(db)
	submission, err := repo.CreateSubmission(context.Background(),
		&types.AgentMarketplaceListingEntity{TenantID: 1, SourceAgentID: "agent-a", DisplayName: "Public helper", Summary: "Portable", State: "listed"},
		&types.AgentReleaseSubmissionEntity{
			TenantID: 1, AgentVersionID: "version-a", SourceAgentID: "agent-a", AuthorID: "publisher-admin",
			SemanticVersion: "1.0.0", BundleDigest: digest, ManifestJSON: manifest,
			DependencyLockJSON: `{"dependencies":[]}`, Bundle: bundle, Status: "submitted",
		})
	require.NoError(t, err)
	_, release, err := repo.ReviewAndPublishTx(context.Background(), 1, "", submission.ID, digest, types.AgentReleaseReviewDecision{ReviewerID: "tenant-reviewer", Decision: "approved"})
	require.NoError(t, err)
	require.NotNil(t, release)
	return submission.ListingID, release.ID, digest
}

func TestPublicMarketplaceServiceSubmitRequiresVerifiedPublisher(t *testing.T) {
	svc, db := newPublicMarketplaceServiceForTest(t)
	listingID, _, _ := seedTenantRelease(t, db)
	ctx := context.Background()

	_, err := svc.SubmitPublicRelease(ctx, 1, "publisher-admin", listingID, "")
	require.ErrorIs(t, err, ErrPublicMarketplaceNotVerifiedPublisher)

	var count int64
	db.Table("public_release_submissions").Count(&count)
	require.Zero(t, count, "未验证发布者不得留下任何 submission 行")

	_, _, err = svc.VerifyPublisher(ctx, "sysadmin", 1, "identity checked")
	require.NoError(t, err)
	view, err := svc.SubmitPublicRelease(ctx, 1, "publisher-admin", listingID, "")
	require.NoError(t, err)
	require.Equal(t, "submitted", view.Status)
	require.Equal(t, listingID, view.SourceListingID)
	require.NotEmpty(t, view.PublicListingID)
	require.NotEmpty(t, view.BundleDigest)
}

func TestPublicMarketplaceServiceSubmitCopiesPortableReleaseVerbatim(t *testing.T) {
	svc, db := newPublicMarketplaceServiceForTest(t)
	listingID, releaseID, _ := seedTenantRelease(t, db)
	ctx := context.Background()
	_, _, err := svc.VerifyPublisher(ctx, "sysadmin", 1, "")
	require.NoError(t, err)

	view, err := svc.SubmitPublicRelease(ctx, 1, "publisher-admin", listingID, "")
	require.NoError(t, err)
	var source types.AgentReleaseEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", uint64(1), releaseID).First(&source).Error)
	require.Equal(t, source.Bundle, view.Bundle, "公共提交=源可移植 Release 逐字节复制")
	require.Equal(t, source.BundleDigest, view.BundleDigest)
	require.Equal(t, source.ManifestJSON, view.ManifestJSON)

	// 源 Release 字节被篡改（digest 列未变）：提交默认指向 current release，
	// 必须在写库前被 digest 校验拒绝（fail closed）
	tampered := []byte(`{"payload":{"system_prompt":"evil"}}`)
	require.NoError(t, db.Exec("UPDATE agent_releases SET bundle = ? WHERE tenant_id = ? AND id = ?", tampered, uint64(1), releaseID).Error)
	_, _, err = svc.SubmitPublicRelease(ctx, 1, "publisher-admin", listingID, "")
	require.Error(t, err, "digest 与 bundle 不一致必须拒绝")
	require.ErrorIs(t, err, ErrPublicMarketplaceStaleDigest)
}

func TestPublicMarketplaceServiceReviewApprovesAndFillsCatalog(t *testing.T) {
	svc, db := newPublicMarketplaceServiceForTest(t)
	listingID, _, _ := seedTenantRelease(t, db)
	ctx := context.Background()
	_, _, err := svc.VerifyPublisher(ctx, "sysadmin", 1, "")
	require.NoError(t, err)
	submission, err := svc.SubmitPublicRelease(ctx, 1, "publisher-admin", listingID, "")
	require.NoError(t, err)

	// 拒绝必须给理由；digest 必须与 submission 一致
	_, err = svc.ReviewPublicSubmission(ctx, "platform-reviewer", submission.ID, submission.BundleDigest, types.AgentReleaseReviewDecision{Decision: "rejected"})
	require.ErrorIs(t, err, ErrPublicMarketplaceInvalidInput)
	// stale digest：与记录摘要必然不同的合法 sha-256 形态（末位十六进制翻转）
	_, err = svc.ReviewPublicSubmission(ctx, "platform-reviewer", submission.ID, differentDigest(submission.BundleDigest), types.AgentReleaseReviewDecision{Decision: "approved"})
	require.ErrorIs(t, err, ErrPublicMarketplaceStaleDigest)

	result, err := svc.ReviewPublicSubmission(ctx, "platform-reviewer", submission.ID, submission.BundleDigest, types.AgentReleaseReviewDecision{Decision: "approved"})
	require.NoError(t, err)
	require.NotNil(t, result.Review)
	require.NotNil(t, result.Release)

	catalog, err := svc.ListPublicCatalog(ctx)
	require.NoError(t, err)
	require.Len(t, catalog, 1)
	require.Equal(t, result.Release.ID, catalog[0].CurrentRelease.ID)
	require.Equal(t, uint64(1), catalog[0].PublisherTenantID)
	require.True(t, catalog[0].PublisherVerified, "已验证发布者必须在目录可见")

	_, err = svc.ReviewPublicSubmission(ctx, "platform-reviewer-2", submission.ID, submission.BundleDigest, types.AgentReleaseReviewDecision{Decision: "rejected", Reason: "late"})
	require.ErrorIs(t, err, repository.ErrPublicMarketplaceReviewConflict, "同一 submission 只能被决定一次")
}

func TestPublicMarketplaceServiceAdoptPropagatesPortableReleaseAndFeedsVariantChain(t *testing.T) {
	svc, db := newPublicMarketplaceServiceForTest(t)
	listingID, _, _ := seedTenantRelease(t, db)
	ctx := context.Background()
	_, _, err := svc.VerifyPublisher(ctx, "sysadmin", 1, "")
	require.NoError(t, err)
	submission, err := svc.SubmitPublicRelease(ctx, 1, "publisher-admin", listingID, "")
	require.NoError(t, err)
	result, err := svc.ReviewPublicSubmission(ctx, "platform-reviewer", submission.ID, submission.BundleDigest, types.AgentReleaseReviewDecision{Decision: "approved"})
	require.NoError(t, err)

	// 跨租户引入：tenant 2（无任何本地 listing/release）引入公共 Listing
	adopted, created, err := svc.AdoptPublicListing(ctx, 2, "adopter-admin", result.Release.ListingID, "")
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, result.Release.ID, adopted.Introduction.PublicReleaseID)
	require.Equal(t, result.Release.Bundle, adopted.Introduction.Bundle)
	require.Equal(t, adopted.Introduction.ID, adopted.Adoption.AcceptedReleaseID)
	require.Equal(t, result.Release.ListingID, adopted.Adoption.ListingID)

	// 幂等：同一公共 release 二次引入不新建
	_, createdAgain, err := svc.AdoptPublicListing(ctx, 2, "adopter-admin", result.Release.ListingID, result.Release.ID)
	require.NoError(t, err)
	require.False(t, createdAgain)

	// #59 链条在引入 release 上原样可用：Variant 固定引入 release，Manifest 能力需求可读
	adoptions := NewAgentAdoptionService(repository.NewAgentAdoptionRepository(db), &fakeAdoptionAgentSource{db: db}, fakeAdoptionVersions{})
	variant, err := adoptions.CreateVariant(ctx, 2, "adopter-admin", adopted.Adoption.ID, interfaces.VariantDraftInput{Name: "Imported helper"})
	require.NoError(t, err)
	require.Equal(t, adopted.Introduction.ID, variant.ReleaseID)
	require.Equal(t, []string{"knowledge"}, variant.MissingCapabilities, "Manifest 能力需求经引入台账可读")

	// 未知公共 listing / release 一律 NotFound
	_, _, err = svc.AdoptPublicListing(ctx, 2, "adopter-admin", "no-such-listing", "")
	require.ErrorIs(t, err, ErrPublicMarketplaceNotFound)
	_, _, err = svc.AdoptPublicListing(ctx, 2, "adopter-admin", result.Release.ListingID, "no-such-release")
	require.ErrorIs(t, err, ErrPublicMarketplaceNotFound)

	// 隐私：目录读模型不含采用方痕迹（结构保证，此处断言行数不因引入而增长字段）
	entries, err := svc.ListPublicCatalog(ctx)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, result.Release.ListingID, entries[0].ListingID)
}
```

Run: `go test ./internal/application/service/ -run 'TestPublicMarketplaceService' -count=1`
Expected: FAIL —— 编译错误 `undefined: NewPublicMarketplaceService`（及 `ErrPublicMarketplaceNotVerifiedPublisher` 等）。

- [ ] **Step 2: 写服务接口**

创建 `internal/types/interfaces/public_marketplace.go`：

```go
package interfaces

import (
	"time"

	"github.com/Tencent/WeKnora/internal/types"
)

// PublicMarketplaceService coordinates the Public Marketplace governance
// workflow (spec §2/§7/§13): Verified Publisher submissions, platform
// review, the public catalog read model and cross-tenant Adoption of
// public releases. HTTP authorization remains the route boundary; the
// platform reviewer endpoints are gated SystemAdmin at the router, tenant
// identity always arrives from the authenticated request context.
type PublicMarketplaceService interface {
	// VerifyPublisher registers (or re-verifies) a tenant as a Verified
	// Publisher; the bool reports whether the registry row was created.
	VerifyPublisher(ctx context.Context, actorID string, tenantID uint64, note string) (VerifiedPublisherView, bool, error)
	RevokePublisher(ctx context.Context, actorID string, tenantID uint64) error
	ListVerifiedPublishers(ctx context.Context) ([]VerifiedPublisherView, error)
	// SubmitPublicRelease promotes one of the caller tenant's own immutable
	// tenant releases for platform review (verbatim portable copy).
	SubmitPublicRelease(ctx context.Context, tenantID uint64, actorID, sourceListingID, releaseID string) (PublicSubmissionView, error)
	// ListPublicSubmissions returns ONLY the caller tenant's own public
	// submissions (publisher isolation: no other tenants' rows).
	ListPublicSubmissions(ctx context.Context, tenantID uint64) ([]PublicSubmissionView, error)
	ListPublicReviewQueue(ctx context.Context) ([]PublicSubmissionView, error)
	ReviewPublicSubmission(ctx context.Context, reviewerID, submissionID, expectedDigest string, decision types.AgentReleaseReviewDecision) (PublicReviewResult, error)
	// ListPublicCatalog is the discoverable public catalog read model. It
	// carries listing/release/publisher trust data ONLY — no adopter
	// identity, counts, mappings or task data ever enter it (spec §12).
	ListPublicCatalog(ctx context.Context) ([]PublicCatalogEntryView, error)
	GetPublicListing(ctx context.Context, listingID string) (*PublicListingDetailView, error)
	// AdoptPublicListing is the cross-tenant adoption entry: it propagates
	// the chosen public release (default: the listing's current release)
	// into the adopter tenant and creates/updates the tenant's Adoption;
	// the bool reports whether the introduction row was created.
	AdoptPublicListing(ctx context.Context, tenantID uint64, actorID, listingID, releaseID string) (PublicAdoptionResult, bool, error)
}

type VerifiedPublisherView struct {
	types.VerifiedPublisherEntity
}

type PublicSubmissionView struct {
	types.PublicReleaseSubmissionEntity
}

type PublicReviewResult struct {
	Review  *types.PublicReleaseReviewEntity `json:"review"`
	Release *types.PublicAgentReleaseEntity  `json:"release,omitempty"`
}

// PublicReleaseSummary is the portable documentation of one public
// release: version, digest, Manifest and Dependency Lock. No bundle bytes
// cross the service boundary through views.
type PublicReleaseSummary struct {
	ID                 string
	SemanticVersion    string
	BundleDigest       string
	ManifestJSON       string
	DependencyLockJSON string
	CreatedAt          time.Time
}

// PublicCatalogEntryView is one public catalog row: listing identity,
// display data, publisher trust signal and the current release summary.
type PublicCatalogEntryView struct {
	ListingID         string
	DisplayName       string
	Summary           string
	State             string
	PublisherTenantID uint64
	PublisherVerified bool
	CurrentRelease    *PublicReleaseSummary
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type PublicListingDetailView struct {
	PublicCatalogEntryView
}

// PublicAdoptionResult pairs the adopter-side introduction with the
// resulting Adoption row. The full variant-bearing adoption view stays on
// GET /marketplace/tenant/adoptions (#59).
type PublicAdoptionResult struct {
	Introduction types.TenantIntroducedReleaseEntity `json:"introduction"`
	Adoption     types.AgentAdoptionEntity           `json:"adoption"`
}
```

- [ ] **Step 3: 写服务实现**

创建 `internal/application/service/public_marketplace.go`：

```go
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

var (
	ErrPublicMarketplaceInvalidInput       = errors.New("invalid public marketplace request")
	ErrPublicMarketplaceNotFound           = errors.New("public marketplace resource not found")
	ErrPublicMarketplaceNotVerifiedPublisher = errors.New("tenant is not a verified publisher")
	ErrPublicMarketplaceStaleDigest        = errors.New("public marketplace digest is stale or does not match the recorded bundle")
)

const (
	VerifiedPublisherStateVerified = "verified"
	PublicListingStateListed       = "listed"
)

type PublicMarketplaceService struct {
	repo     repository.PublicMarketplaceRepository
	listings interfaces.AgentMarketplaceRepository
	now      func() time.Time
}

var _ interfaces.PublicMarketplaceService = (*PublicMarketplaceService)(nil)

func NewPublicMarketplaceService(repo repository.PublicMarketplaceRepository, listings interfaces.AgentMarketplaceRepository) *PublicMarketplaceService {
	return &PublicMarketplaceService{repo: repo, listings: listings, now: time.Now}
}

func (s *PublicMarketplaceService) VerifyPublisher(ctx context.Context, actorID string, tenantID uint64, note string) (interfaces.VerifiedPublisherView, bool, error) {
	actorID = strings.TrimSpace(actorID)
	if actorID == "" || tenantID == 0 {
		return interfaces.VerifiedPublisherView{}, false, ErrPublicMarketplaceInvalidInput
	}
	row, created, err := s.repo.VerifyPublisher(ctx, &types.VerifiedPublisherEntity{
		TenantID: tenantID, State: VerifiedPublisherStateVerified, VerifiedBy: actorID, Note: strings.TrimSpace(note), VerifiedAt: s.now().UTC(),
	})
	if err != nil {
		return interfaces.VerifiedPublisherView{}, false, err
	}
	return interfaces.VerifiedPublisherView{VerifiedPublisherEntity: *row}, created, nil
}

func (s *PublicMarketplaceService) RevokePublisher(ctx context.Context, actorID string, tenantID uint64) error {
	if strings.TrimSpace(actorID) == "" || tenantID == 0 {
		return ErrPublicMarketplaceInvalidInput
	}
	return s.repo.RevokePublisher(ctx, tenantID)
}

func (s *PublicMarketplaceService) ListVerifiedPublishers(ctx context.Context) ([]interfaces.VerifiedPublisherView, error) {
	rows, err := s.repo.ListVerifiedPublishers(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]interfaces.VerifiedPublisherView, 0, len(rows))
	for _, row := range rows {
		views = append(views, interfaces.VerifiedPublisherView{VerifiedPublisherEntity: row})
	}
	return views, nil
}

func (s *PublicMarketplaceService) requireVerifiedPublisher(ctx context.Context, tenantID uint64) error {
	row, err := s.repo.GetVerifiedPublisher(ctx, tenantID)
	if err != nil {
		return err
	}
	if row == nil || row.State != VerifiedPublisherStateVerified {
		return ErrPublicMarketplaceNotVerifiedPublisher
	}
	return nil
}

func (s *PublicMarketplaceService) SubmitPublicRelease(ctx context.Context, tenantID uint64, actorID, sourceListingID, releaseID string) (interfaces.PublicSubmissionView, error) {
	actorID, sourceListingID, releaseID = strings.TrimSpace(actorID), strings.TrimSpace(sourceListingID), strings.TrimSpace(releaseID)
	if tenantID == 0 || actorID == "" || sourceListingID == "" {
		return interfaces.PublicSubmissionView{}, ErrPublicMarketplaceInvalidInput
	}
	if err := s.requireVerifiedPublisher(ctx, tenantID); err != nil {
		return interfaces.PublicSubmissionView{}, err
	}
	// 源读取走 #58 marketplace 仓储（无引入回退）：只有本租户自有 listing/release
	// 可以被提升为公共提交，杜绝把别人发布的内容冒名再提交。
	listing, err := s.listings.GetListing(ctx, tenantID, sourceListingID)
	if err != nil {
		return interfaces.PublicSubmissionView{}, err
	}
	if listing == nil {
		return interfaces.PublicSubmissionView{}, ErrPublicMarketplaceNotFound
	}
	if listing.State != "listed" {
		return interfaces.PublicSubmissionView{}, fmt.Errorf("%w: listing state is %q", ErrPublicMarketplaceInvalidInput, listing.State)
	}
	resolvedReleaseID := releaseID
	if resolvedReleaseID == "" {
		if listing.CurrentReleaseID == nil {
			return interfaces.PublicSubmissionView{}, fmt.Errorf("%w: listing has no current release", ErrPublicMarketplaceInvalidInput)
		}
		resolvedReleaseID = *listing.CurrentReleaseID
	}
	release, err := s.listings.GetRelease(ctx, tenantID, resolvedReleaseID)
	if err != nil {
		return interfaces.PublicSubmissionView{}, err
	}
	if release == nil || release.ListingID != listing.ID {
		return interfaces.PublicSubmissionView{}, fmt.Errorf("%w: release does not belong to the source listing", ErrPublicMarketplaceInvalidInput)
	}
	if err := verifyBundleDigest(release.Bundle, release.BundleDigest); err != nil {
		return interfaces.PublicSubmissionView{}, err
	}
	created, err := s.repo.CreatePublicSubmission(ctx,
		&types.PublicMarketplaceListingEntity{DisplayName: listing.DisplayName, Summary: listing.Summary},
		&types.PublicReleaseSubmissionEntity{
			PublisherTenantID: tenantID, SourceListingID: listing.ID, SourceReleaseID: release.ID,
			PublisherActorID: actorID, SemanticVersion: release.SemanticVersion, BundleDigest: release.BundleDigest,
			ManifestJSON: release.ManifestJSON, DependencyLockJSON: release.DependencyLockJSON,
			Bundle: append([]byte(nil), release.Bundle...), Status: "submitted",
		})
	if err != nil {
		return interfaces.PublicSubmissionView{}, err
	}
	return interfaces.PublicSubmissionView{PublicReleaseSubmissionEntity: *created}, nil
}

func (s *PublicMarketplaceService) ListPublicSubmissions(ctx context.Context, tenantID uint64) ([]interfaces.PublicSubmissionView, error) {
	if tenantID == 0 {
		return nil, ErrPublicMarketplaceInvalidInput
	}
	rows, err := s.repo.ListPublicSubmissions(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	views := make([]interfaces.PublicSubmissionView, 0, len(rows))
	for _, row := range rows {
		views = append(views, interfaces.PublicSubmissionView{PublicReleaseSubmissionEntity: row})
	}
	return views, nil
}

func (s *PublicMarketplaceService) ListPublicReviewQueue(ctx context.Context) ([]interfaces.PublicSubmissionView, error) {
	rows, err := s.repo.ListPublicReviewQueue(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]interfaces.PublicSubmissionView, 0, len(rows))
	for _, row := range rows {
		views = append(views, interfaces.PublicSubmissionView{PublicReleaseSubmissionEntity: row})
	}
	return views, nil
}

func (s *PublicMarketplaceService) ReviewPublicSubmission(ctx context.Context, reviewerID, submissionID, expectedDigest string, decision types.AgentReleaseReviewDecision) (interfaces.PublicReviewResult, error) {
	reviewerID, submissionID, expectedDigest = strings.TrimSpace(reviewerID), strings.TrimSpace(submissionID), strings.ToLower(strings.TrimSpace(expectedDigest))
	decision.Decision, decision.Reason = strings.TrimSpace(decision.Decision), strings.TrimSpace(decision.Reason)
	if reviewerID == "" || submissionID == "" || !validMarketplaceDigest(expectedDigest) {
		return interfaces.PublicReviewResult{}, ErrPublicMarketplaceInvalidInput
	}
	if decision.Decision != "approved" && decision.Decision != "rejected" && decision.Decision != "changes_requested" {
		return interfaces.PublicReviewResult{}, fmt.Errorf("%w: invalid review decision", ErrPublicMarketplaceInvalidInput)
	}
	if decision.Decision != "approved" && decision.Reason == "" {
		return interfaces.PublicReviewResult{}, fmt.Errorf("%w: a reason is required", ErrPublicMarketplaceInvalidInput)
	}
	decision.ReviewerID = reviewerID
	submission, err := s.repo.GetPublicSubmission(ctx, submissionID)
	if err != nil {
		return interfaces.PublicReviewResult{}, err
	}
	if submission == nil {
		return interfaces.PublicReviewResult{}, repository.ErrPublicMarketplaceNotFound
	}
	if submission.BundleDigest != expectedDigest {
		return interfaces.PublicReviewResult{}, ErrPublicMarketplaceStaleDigest
	}
	if err := verifyBundleDigest(submission.Bundle, submission.BundleDigest); err != nil {
		return interfaces.PublicReviewResult{}, err
	}
	priorReleaseID := ""
	if decision.Decision == "approved" {
		listing, err := s.repo.GetPublicListing(ctx, submission.PublicListingID)
		if err != nil {
			return interfaces.PublicReviewResult{}, err
		}
		if listing == nil {
			return interfaces.PublicReviewResult{}, repository.ErrPublicMarketplaceNotFound
		}
		if listing.CurrentReleaseID != nil {
			priorReleaseID = *listing.CurrentReleaseID
		}
	}
	review, release, err := s.repo.ReviewAndPublishPublicTx(ctx, priorReleaseID, submissionID, expectedDigest, decision)
	if err != nil {
		return interfaces.PublicReviewResult{}, err
	}
	return interfaces.PublicReviewResult{Review: review, Release: release}, nil
}

func (s *PublicMarketplaceService) ListPublicCatalog(ctx context.Context) ([]interfaces.PublicCatalogEntryView, error) {
	rows, err := s.repo.ListPublicCatalog(ctx)
	if err != nil {
		return nil, err
	}
	verified := map[uint64]bool{}
	publishers, err := s.repo.ListVerifiedPublishers(ctx)
	if err != nil {
		return nil, err
	}
	for _, publisher := range publishers {
		if publisher.State == VerifiedPublisherStateVerified {
			verified[publisher.TenantID] = true
		}
	}
	views := make([]interfaces.PublicCatalogEntryView, 0, len(rows))
	for _, row := range rows {
		if row.Listing.State != PublicListingStateListed || row.Release == nil {
			continue
		}
		views = append(views, interfaces.PublicCatalogEntryView{
			ListingID: row.Listing.ID, DisplayName: row.Listing.DisplayName, Summary: row.Listing.Summary,
			State: row.Listing.State, PublisherTenantID: row.Listing.PublisherTenantID,
			PublisherVerified: verified[row.Listing.PublisherTenantID],
			CurrentRelease: &interfaces.PublicReleaseSummary{
				ID: row.Release.ID, SemanticVersion: row.Release.SemanticVersion, BundleDigest: row.Release.BundleDigest,
				ManifestJSON: row.Release.ManifestJSON, DependencyLockJSON: row.Release.DependencyLockJSON, CreatedAt: row.Release.CreatedAt,
			},
			CreatedAt: row.Listing.CreatedAt, UpdatedAt: row.Listing.UpdatedAt,
		})
	}
	return views, nil
}

func (s *PublicMarketplaceService) GetPublicListing(ctx context.Context, listingID string) (*interfaces.PublicListingDetailView, error) {
	listingID = strings.TrimSpace(listingID)
	if listingID == "" {
		return nil, ErrPublicMarketplaceInvalidInput
	}
	listing, err := s.repo.GetPublicListing(ctx, listingID)
	if err != nil {
		return nil, err
	}
	if listing == nil {
		return nil, repository.ErrPublicMarketplaceNotFound
	}
	entry := interfaces.PublicCatalogEntryView{
		ListingID: listing.ID, DisplayName: listing.DisplayName, Summary: listing.Summary,
		State: listing.State, PublisherTenantID: listing.PublisherTenantID,
		CreatedAt: listing.CreatedAt, UpdatedAt: listing.UpdatedAt,
	}
	publisher, err := s.repo.GetVerifiedPublisher(ctx, listing.PublisherTenantID)
	if err != nil {
		return nil, err
	}
	entry.PublisherVerified = publisher != nil && publisher.State == VerifiedPublisherStateVerified
	if listing.CurrentReleaseID != nil {
		release, err := s.repo.GetPublicRelease(ctx, *listing.CurrentReleaseID)
		if err != nil {
			return nil, err
		}
		if release != nil {
			entry.CurrentRelease = &interfaces.PublicReleaseSummary{
				ID: release.ID, SemanticVersion: release.SemanticVersion, BundleDigest: release.BundleDigest,
				ManifestJSON: release.ManifestJSON, DependencyLockJSON: release.DependencyLockJSON, CreatedAt: release.CreatedAt,
			}
		}
	}
	return &interfaces.PublicListingDetailView{PublicCatalogEntryView: entry}, nil
}

func (s *PublicMarketplaceService) AdoptPublicListing(ctx context.Context, tenantID uint64, actorID, listingID, releaseID string) (interfaces.PublicAdoptionResult, bool, error) {
	actorID, listingID, releaseID = strings.TrimSpace(actorID), strings.TrimSpace(listingID), strings.TrimSpace(releaseID)
	if tenantID == 0 || actorID == "" || listingID == "" {
		return interfaces.PublicAdoptionResult{}, false, ErrPublicMarketplaceInvalidInput
	}
	listing, err := s.repo.GetPublicListing(ctx, listingID)
	if err != nil {
		return interfaces.PublicAdoptionResult{}, false, err
	}
	if listing == nil || listing.State != PublicListingStateListed {
		return interfaces.PublicAdoptionResult{}, false, ErrPublicMarketplaceNotFound
	}
	resolvedReleaseID := releaseID
	if resolvedReleaseID == "" {
		if listing.CurrentReleaseID == nil {
			return interfaces.PublicAdoptionResult{}, false, fmt.Errorf("%w: listing has no current release", ErrPublicMarketplaceInvalidInput)
		}
		resolvedReleaseID = *listing.CurrentReleaseID
	}
	release, err := s.repo.GetPublicRelease(ctx, resolvedReleaseID)
	if err != nil {
		return interfaces.PublicAdoptionResult{}, false, err
	}
	if release == nil || release.ListingID != listing.ID {
		return interfaces.PublicAdoptionResult{}, false, ErrPublicMarketplaceNotFound
	}
	// AC1 传播完整性：公共 release 的字节必须仍与记录摘要一致，否则零传播。
	if err := verifyBundleDigest(release.Bundle, release.BundleDigest); err != nil {
		return interfaces.PublicAdoptionResult{}, false, err
	}
	introduced, adoption, created, err := s.repo.IntroduceRelease(ctx, tenantID, actorID, listing, release)
	if err != nil {
		return interfaces.PublicAdoptionResult{}, false, err
	}
	return interfaces.PublicAdoptionResult{Introduction: *introduced, Adoption: *adoption}, created, nil
}

// verifyBundleDigest fails closed on any byte drift between the bundle and
// its recorded digest — the shared guard for submit, review and introduce.
func verifyBundleDigest(bundle []byte, digest string) error {
	sum := sha256.Sum256(bundle)
	if hex.EncodeToString(sum[:]) != digest {
		return ErrPublicMarketplaceStaleDigest
	}
	return nil
}
```

- [ ] **Step 4: 运行服务测试确认通过（GREEN）**

Run: `go test ./internal/application/service/ -run 'TestPublicMarketplaceService' -count=1`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/types/interfaces/public_marketplace.go internal/application/service/public_marketplace.go internal/application/service/public_marketplace_test.go
git commit -m "feat(marketplace): public marketplace service with verified-publisher gate, platform review and cross-tenant adoption (T30 #60 task 5)"
```

---

### Task 6: HTTP 面 I：Verified Publisher + 公共提交 + 平台审核 + 接线

**Files:**
- Create: `internal/handler/public_marketplace.go`
- Create: `internal/router/routes_public_marketplace.go`
- Modify: `internal/router/router.go:104`（params 追加一行）与 `internal/router/router.go:404`（注册追加一行）
- Modify: `internal/container/container.go:438`（追加三个 Provide）
- Test: `internal/router/routes_public_marketplace_test.go`（新文件，Task 7/8 复用其 harness）

**Interfaces:**
- Consumes: Task 5 的 `interfaces.PublicMarketplaceService` 全部方法与错误；#59 测试骨架 `newAgentAdoptionTestApp`（`internal/router/routes_agent_adoption_test.go:21-63`）与 `openTenantAgentMarketplaceHTTPTestDB`；`handler` 包内 `decodeAgentMarketplaceBody`/`invalidMarketplaceBody`/`sandboxConfigTenantID`/`agentMarketplaceMaxRequestBytes`（`internal/handler/agent_marketplace.go:21,139-157` 与 `internal/handler/sandbox_config.go:92`）与 `reviewAgentReleaseBody`（`internal/handler/agent_marketplace.go:50-54`）。
- Produces: `handler.NewPublicMarketplaceHandler(public interfaces.PublicMarketplaceService)`；`RegisterPublicMarketplaceRoutes(r *gin.RouterGroup, publicHandler *handler.PublicMarketplaceHandler, g *rbacGuards)`（Task 7 扩展同文件加目录/adopt 路由）；路由 `/api/v1/marketplace/public/verified-publishers*`、`/release-submissions*`（Task 7 消费 harness）；测试 harness `newPublicMarketplaceTestApp(t) (*gin.Engine, *rbacGuards, *gorm.DB)` 与 `publicCall(r, tenantID, systemAdmin bool, method, path, role, actor string, body any)`（Task 7/8 消费）。

- [ ] **Step 1: 写失败测试（含 Task 6/7 共用的 harness）**

创建 `internal/router/routes_public_marketplace_test.go`：

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

// newPublicMarketplaceTestApp mounts the REAL cross-tenant stack over the
// real migration stream: publisher tenant 1's release workflow, the
// platform public marketplace, tenant 2's adoption chain and the real
// GET /api/v1/agents list the mobile Resource Shelf consumes. The
// X-Test-System-Admin header injects the platform reviewer identity the
// same way the other tests inject tenant roles.
func newPublicMarketplaceTestApp(t *testing.T) (*gin.Engine, *rbacGuards, *gorm.DB) {
	t.Helper()
	db := openTenantAgentMarketplaceHTTPTestDB(t)
	require.NoError(t, db.Create(&types.CustomAgent{
		ID: "agent-owned", Name: "Release helper", TenantID: 1, CreatedBy: "contributor",
		Config: types.CustomAgentConfig{AgentMode: "smart-reasoning", SystemPrompt: "Be portable.", KnowledgeBases: []string{"kb-publisher"}, ModelID: "model-publisher"},
	}).Error)

	marketRepo := repository.NewAgentMarketplaceRepository(db)
	customAgents := service.NewCustomAgentService(repository.NewCustomAgentRepository(db), nil, nil, nil, nil, nil, nil)
	versions := service.NewAgentVersionService(customAgents, repository.NewAgentVersionRepository(db))
	market := service.NewAgentMarketplaceService(versions, marketplaceHTTPResolver{}, marketRepo, t.TempDir())
	adoptions := service.NewAgentAdoptionService(repository.NewAgentAdoptionRepository(db), customAgents, versions)
	public := service.NewPublicMarketplaceService(repository.NewPublicMarketplaceRepository(db), marketRepo)

	versionHandler := handler.NewAgentVersionHandler(versions)
	marketHandler := handler.NewAgentMarketplaceHandler(market, versions)
	adoptionHandler := handler.NewAgentAdoptionHandler(adoptions)
	publicHandler := handler.NewPublicMarketplaceHandler(public)
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
		if c.GetHeader("X-Test-System-Admin") == "1" {
			ctx = context.WithValue(ctx, types.SystemAdminContextKey, true)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Set(types.TenantIDContextKey.String(), tenantID)
		c.Next()
	})
	v1 := r.Group("/api/v1")
	RegisterAgentVersionRoutes(v1, versionHandler, g)
	RegisterAgentMarketplaceRoutes(v1, marketHandler, g)
	RegisterAgentAdoptionRoutes(v1, adoptionHandler, g)
	RegisterPublicMarketplaceRoutes(v1, publicHandler, g)
	RegisterCustomAgentRoutes(v1, agentListHandler, g)
	return r, g, db
}

func publicCall(r *gin.Engine, tenantID uint64, systemAdmin bool, method, path, role, actor string, body any) *httptest.ResponseRecorder {
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
	if systemAdmin {
		req.Header.Set("X-Test-System-Admin", "1")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// differentDigest flips the last hex char of a valid digest, yielding a
// DIFFERENT value that still passes the server's sha-256 format check.
func differentDigest(digest string) string {
	last := digest[len(digest)-1]
	replacement := "0"
	if last == '0' {
		replacement = "1"
	}
	return digest[:len(digest)-1] + replacement
}

// publishTenantRelease drives tenant 1's REAL tenant-release workflow and
// returns the listing id whose current release is portable.
func publishTenantRelease(t *testing.T, r *gin.Engine) string {
	t.Helper()
	frozen := publicCall(r, 1, false, http.MethodPost, "/api/v1/agents/agent-owned/versions", "contributor", "contributor", nil)
	require.Equal(t, http.StatusCreated, frozen.Code, frozen.Body.String())
	var frozenBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(frozen.Body.Bytes(), &frozenBody))
	require.NotEmpty(t, frozenBody.Data.ID)

	metadata := map[string]any{
		"semantic_version": "1.0.0", "display_name": "Public helper", "summary": "A portable helper",
		"supported_languages": []string{"en"}, "use_cases": []string{"support"},
		"capability_requirements":    []string{"knowledge"},
		"minimum_weknora_capability": "1", "license_id": "MIT",
	}
	submitted := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/tenant/release-submissions", "contributor", "contributor", map[string]any{"agent_version_id": frozenBody.Data.ID, "metadata": metadata})
	require.Equal(t, http.StatusCreated, submitted.Code, submitted.Body.String())
	var submissionBody struct {
		Data struct {
			ID           string `json:"id"`
			ListingID    string `json:"listing_id"`
			BundleDigest string `json:"bundle_digest"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(submitted.Body.Bytes(), &submissionBody))

	reviewed := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/tenant/release-submissions/"+submissionBody.Data.ID+"/review", "admin", "tenant-reviewer", map[string]any{"expected_digest": submissionBody.Data.BundleDigest, "decision": "approved"})
	require.Equal(t, http.StatusOK, reviewed.Code, reviewed.Body.String())
	return submissionBody.Data.ListingID
}

func TestPublicMarketplacePublisherVerificationAndSubmissionAuthorization(t *testing.T) {
	r, g, db := newPublicMarketplaceTestApp(t)
	_ = g
	listingID := publishTenantRelease(t, r)

	// Verified Publisher 名册：仅 SystemAdmin
	forbidden := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/public/verified-publishers", "admin", "tenant-admin", map[string]any{"tenant_id": 1})
	require.Equal(t, http.StatusForbidden, forbidden.Code, forbidden.Body.String())

	verified := publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/verified-publishers", "admin", "sysadmin", map[string]any{"tenant_id": 1, "note": "identity checked"})
	require.Equal(t, http.StatusCreated, verified.Code, verified.Body.String())
	reverified := publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/verified-publishers", "admin", "sysadmin", map[string]any{"tenant_id": 1})
	require.Equal(t, http.StatusOK, reverified.Code, reverified.Body.String())

	list := publicCall(r, 1, true, http.MethodGet, "/api/v1/marketplace/public/verified-publishers", "admin", "sysadmin", nil)
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	require.Contains(t, list.Body.String(), `"tenant_id":1`)

	revoked := publicCall(r, 1, true, http.MethodDelete, "/api/v1/marketplace/public/verified-publishers/1", "admin", "sysadmin", nil)
	require.Equal(t, http.StatusOK, revoked.Code, revoked.Body.String())
	revokedAgain := publicCall(r, 1, true, http.MethodDelete, "/api/v1/marketplace/public/verified-publishers/1", "admin", "sysadmin", nil)
	require.Equal(t, http.StatusOK, revokedAgain.Code, revokedAgain.Body.String(), "重复撤销幂等（行仍在，state=revoked）")

	// 未验证发布者不得提交（撤销后同样拒绝）
	denied := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/public/release-submissions", "admin", "tenant-admin", map[string]any{"source_listing_id": listingID})
	require.Equal(t, http.StatusForbidden, denied.Code, denied.Body.String())
	var count int64
	db.Table("public_release_submissions").Count(&count)
	require.Zero(t, count)

	// 重新验证后：Contributor 仍 403（Admin 门禁），Admin 201
	_, _ = publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/verified-publishers", "admin", "sysadmin", map[string]any{"tenant_id": 1})
	contributor := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/public/release-submissions", "contributor", "contributor", map[string]any{"source_listing_id": listingID})
	require.Equal(t, http.StatusForbidden, contributor.Code, contributor.Body.String())
	submitted := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/public/release-submissions", "admin", "tenant-admin", map[string]any{"source_listing_id": listingID})
	require.Equal(t, http.StatusCreated, submitted.Code, submitted.Body.String())
	var submissionBody struct {
		Data struct {
			ID              string `json:"id"`
			PublicListingID string `json:"public_listing_id"`
			BundleDigest    string `json:"bundle_digest"`
			Status          string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(submitted.Body.Bytes(), &submissionBody))
	require.Equal(t, "submitted", submissionBody.Data.Status)

	// 发布者自查询只含本租户提交
	own := publicCall(r, 1, false, http.MethodGet, "/api/v1/marketplace/public/release-submissions", "admin", "tenant-admin", nil)
	require.Equal(t, http.StatusOK, own.Code, own.Body.String())
	require.Contains(t, own.Body.String(), submissionBody.Data.ID)
}

func TestPublicMarketplaceReviewAuthorization(t *testing.T) {
	r, _, _ := newPublicMarketplaceTestApp(t)
	listingID := publishTenantRelease(t, r)
	_, _ = publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/verified-publishers", "admin", "sysadmin", map[string]any{"tenant_id": 1})
	submitted := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/public/release-submissions", "admin", "tenant-admin", map[string]any{"source_listing_id": listingID})
	require.Equal(t, http.StatusCreated, submitted.Code, submitted.Body.String())
	var submissionBody struct {
		Data struct {
			ID           string `json:"id"`
			BundleDigest string `json:"bundle_digest"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(submitted.Body.Bytes(), &submissionBody))

	// 审核队列与审核决定：仅 SystemAdmin（租户 Admin 403）
	tenantQueue := publicCall(r, 1, false, http.MethodGet, "/api/v1/marketplace/public/release-submissions/review-queue", "admin", "tenant-admin", nil)
	require.Equal(t, http.StatusForbidden, tenantQueue.Code, tenantQueue.Body.String())
	tenantReview := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/public/release-submissions/"+submissionBody.Data.ID+"/review", "admin", "tenant-admin", map[string]any{"expected_digest": submissionBody.Data.BundleDigest, "decision": "approved"})
	require.Equal(t, http.StatusForbidden, tenantReview.Code, tenantReview.Body.String())

	queue := publicCall(r, 1, true, http.MethodGet, "/api/v1/marketplace/public/release-submissions/review-queue", "admin", "sysadmin", nil)
	require.Equal(t, http.StatusOK, queue.Code, queue.Body.String())
	require.Contains(t, queue.Body.String(), submissionBody.Data.ID)

	// stale digest → 409
	stale := publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/release-submissions/"+submissionBody.Data.ID+"/review", "admin", "sysadmin", map[string]any{"expected_digest": differentDigest(submissionBody.Data.BundleDigest), "decision": "approved"})
	require.Equal(t, http.StatusConflict, stale.Code, stale.Body.String())

	// 无理由拒绝 → 400
	noReason := publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/release-submissions/"+submissionBody.Data.ID+"/review", "admin", "sysadmin", map[string]any{"expected_digest": submissionBody.Data.BundleDigest, "decision": "rejected"})
	require.Equal(t, http.StatusBadRequest, noReason.Code, noReason.Body.String())

	approved := publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/release-submissions/"+submissionBody.Data.ID+"/review", "admin", "sysadmin", map[string]any{"expected_digest": submissionBody.Data.BundleDigest, "decision": "approved"})
	require.Equal(t, http.StatusOK, approved.Code, approved.Body.String())

	// 重复审核 → 409
	duplicate := publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/release-submissions/"+submissionBody.Data.ID+"/review", "admin", "sysadmin", map[string]any{"expected_digest": submissionBody.Data.BundleDigest, "decision": "rejected", "reason": "late"})
	require.Equal(t, http.StatusConflict, duplicate.Code, duplicate.Body.String())
}
```

Run: `go test ./internal/router/ -run 'TestPublicMarketplacePublisherVerificationAndSubmissionAuthorization' -count=1`
Expected: FAIL —— 编译错误 `undefined: handler.NewPublicMarketplaceHandler` / `undefined: RegisterPublicMarketplaceRoutes`。

- [ ] **Step 2: 写 Handler**

创建 `internal/handler/public_marketplace.go`：

```go
package handler

import (
	stderrors "errors"
	"encoding/json"
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

// PublicMarketplaceHandler is the HTTP boundary for the Public Marketplace
// (T30 #60): Verified Publisher registry (platform), public submissions,
// platform review, the public catalog and cross-tenant adoption. Tenant
// and actor identity always come from the authenticated request context;
// request bodies never carry principals (strict decoding rejects spoof
// attempts). No response of this handler carries adopter-derived data.
type PublicMarketplaceHandler struct {
	public interfaces.PublicMarketplaceService
}

func NewPublicMarketplaceHandler(public interfaces.PublicMarketplaceService) *PublicMarketplaceHandler {
	return &PublicMarketplaceHandler{public: public}
}

type verifyPublisherBody struct {
	TenantID uint64 `json:"tenant_id"`
	Note     string `json:"note,omitempty"`
}

type submitPublicReleaseBody struct {
	SourceListingID string `json:"source_listing_id"`
	ReleaseID       string `json:"release_id,omitempty"`
}

type adoptPublicListingBody struct {
	ReleaseID string `json:"release_id,omitempty"`
}

type verifiedPublisherResponse struct {
	TenantID    uint64    `json:"tenant_id"`
	State       string    `json:"state"`
	VerifiedBy  string    `json:"verified_by"`
	Note        string    `json:"note"`
	VerifiedAt  time.Time `json:"verified_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type publicCatalogReleaseResponse struct {
	ID              string          `json:"id"`
	SemanticVersion string          `json:"semantic_version"`
	BundleDigest    string          `json:"bundle_digest"`
	Manifest        json.RawMessage `json:"manifest"`
	DependencyLock  json.RawMessage `json:"dependency_lock"`
	CreatedAt       time.Time       `json:"created_at"`
}

// publicCatalogListingResponse is the public catalog row wire. Its field
// set is pinned by the router tests with strict decoding: adding any
// adopter-derived field (adoption counts, adopter ids, metrics) breaks the
// privacy contract (spec §12) and the test.
type publicCatalogListingResponse struct {
	ID                string                       `json:"id"`
	DisplayName       string                       `json:"display_name"`
	Summary           string                       `json:"summary"`
	State             string                       `json:"state"`
	PublisherTenantID uint64                       `json:"publisher_tenant_id"`
	PublisherVerified bool                         `json:"publisher_verified"`
	CurrentRelease    *publicCatalogReleaseResponse `json:"current_release,omitempty"`
	CreatedAt         time.Time                    `json:"created_at"`
	UpdatedAt         time.Time                    `json:"updated_at"`
}

type publicSubmissionResponse struct {
	ID                string          `json:"id"`
	PublisherTenantID uint64          `json:"publisher_tenant_id"`
	PublicListingID   string          `json:"public_listing_id"`
	SourceListingID   string          `json:"source_listing_id"`
	SourceReleaseID   string          `json:"source_release_id"`
	PublisherActorID  string          `json:"publisher_actor_id"`
	SemanticVersion   string          `json:"semantic_version"`
	BundleDigest      string          `json:"bundle_digest"`
	Manifest          json.RawMessage `json:"manifest"`
	DependencyLock    json.RawMessage `json:"dependency_lock"`
	Status            string          `json:"status"`
	CreatedAt         time.Time       `json:"created_at"`
}

type publicReviewResponse struct {
	ID             string    `json:"id"`
	SubmissionID   string    `json:"submission_id"`
	ReviewerID     string    `json:"reviewer_id"`
	ReviewedDigest string    `json:"reviewed_digest"`
	Decision       string    `json:"decision"`
	Reason         string    `json:"reason"`
	CreatedAt      time.Time `json:"created_at"`
}

type publicReleaseResponse struct {
	ID                string          `json:"id"`
	ListingID         string          `json:"listing_id"`
	SubmissionID      string          `json:"submission_id"`
	PublisherTenantID uint64          `json:"publisher_tenant_id"`
	ReleaseNumber     int             `json:"release_number"`
	SemanticVersion   string          `json:"semantic_version"`
	BundleDigest      string          `json:"bundle_digest"`
	Manifest          json.RawMessage `json:"manifest"`
	DependencyLock    json.RawMessage `json:"dependency_lock"`
	PublishedBy       string          `json:"published_by"`
	CreatedAt         time.Time       `json:"created_at"`
}

type publicIntroductionResponse struct {
	ID              string    `json:"id"`
	PublicListingID string    `json:"public_listing_id"`
	PublicReleaseID string    `json:"public_release_id"`
	DisplayName     string    `json:"display_name"`
	Summary         string    `json:"summary"`
	SemanticVersion string    `json:"semantic_version"`
	BundleDigest    string    `json:"bundle_digest"`
	IntroducedBy    string    `json:"introduced_by"`
	IntroducedAt    time.Time `json:"introduced_at"`
}

type publicAdoptionSummaryResponse struct {
	ID                string    `json:"id"`
	ListingID         string    `json:"listing_id"`
	AcceptedReleaseID string    `json:"accepted_release_id"`
	State             string    `json:"state"`
	CreatedBy         string    `json:"created_by"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type adoptPublicListingResponse struct {
	Introduction publicIntroductionResponse    `json:"introduction"`
	Adoption     publicAdoptionSummaryResponse `json:"adoption"`
}

func verifiedPublisherDTO(row interfaces.VerifiedPublisherView) verifiedPublisherResponse {
	return verifiedPublisherResponse{
		TenantID: row.TenantID, State: row.State, VerifiedBy: row.VerifiedBy, Note: row.Note,
		VerifiedAt: row.VerifiedAt, UpdatedAt: row.UpdatedAt,
	}
}

func publicCatalogReleaseDTO(release *interfaces.PublicReleaseSummary) *publicCatalogReleaseResponse {
	if release == nil {
		return nil
	}
	return &publicCatalogReleaseResponse{
		ID: release.ID, SemanticVersion: release.SemanticVersion, BundleDigest: release.BundleDigest,
		Manifest: json.RawMessage(release.ManifestJSON), DependencyLock: json.RawMessage(release.DependencyLockJSON),
		CreatedAt: release.CreatedAt,
	}
}

func publicCatalogListingDTO(entry interfaces.PublicCatalogEntryView) publicCatalogListingResponse {
	return publicCatalogListingResponse{
		ID: entry.ListingID, DisplayName: entry.DisplayName, Summary: entry.Summary, State: entry.State,
		PublisherTenantID: entry.PublisherTenantID, PublisherVerified: entry.PublisherVerified,
		CurrentRelease: publicCatalogReleaseDTO(entry.CurrentRelease),
		CreatedAt:      entry.CreatedAt, UpdatedAt: entry.UpdatedAt,
	}
}

func publicSubmissionDTO(row interfaces.PublicSubmissionView) publicSubmissionResponse {
	return publicSubmissionResponse{
		ID: row.ID, PublisherTenantID: row.PublisherTenantID, PublicListingID: row.PublicListingID,
		SourceListingID: row.SourceListingID, SourceReleaseID: row.SourceReleaseID,
		PublisherActorID: row.PublisherActorID, SemanticVersion: row.SemanticVersion,
		BundleDigest: row.BundleDigest, Manifest: json.RawMessage(row.ManifestJSON),
		DependencyLock: json.RawMessage(row.DependencyLockJSON), Status: row.Status, CreatedAt: row.CreatedAt,
	}
}

func publicMarketplaceClientError(err error) error {
	switch {
	case stderrors.Is(err, marketservice.ErrPublicMarketplaceNotFound), stderrors.Is(err, marketrepo.ErrPublicMarketplaceNotFound):
		return apperrors.NewNotFoundError("public marketplace resource not found")
	case stderrors.Is(err, marketservice.ErrPublicMarketplaceNotVerifiedPublisher):
		return apperrors.NewForbiddenError("tenant is not a verified publisher")
	case stderrors.Is(err, marketservice.ErrPublicMarketplaceStaleDigest),
		stderrors.Is(err, marketrepo.ErrPublicMarketplaceDigestMismatch),
		stderrors.Is(err, marketrepo.ErrPublicMarketplaceReviewConflict),
		stderrors.Is(err, marketrepo.ErrPublicMarketplacePointerConflict),
		stderrors.Is(err, marketrepo.ErrPublicMarketplaceReleaseConflict):
		return apperrors.NewConflictError("public marketplace submission changed; reload and review again")
	case stderrors.Is(err, marketservice.ErrPublicMarketplaceInvalidInput),
		stderrors.Is(err, marketrepo.ErrPublicMarketplaceInvalidDecision):
		return apperrors.NewValidationError("invalid public marketplace request")
	default:
		return err
	}
}

func (h *PublicMarketplaceHandler) VerifyPublisher(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agentMarketplaceMaxRequestBytes)
	var body *verifyPublisherBody
	if err := decodeAgentMarketplaceBody(c.Request.Body, &body); err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	if body == nil || body.TenantID == 0 {
		invalidMarketplaceBody(c, stderrors.New("tenant_id is required"))
		return
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	view, created, err := h.public.VerifyPublisher(c.Request.Context(), actorID, body.TenantID, body.Note)
	if err != nil {
		_ = c.Error(publicMarketplaceClientError(err))
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, gin.H{"success": true, "data": verifiedPublisherDTO(view)})
}

func (h *PublicMarketplaceHandler) RevokePublisher(c *gin.Context) {
	targetTenantID, err := parseTenantIDParam(c.Param("tenant_id"))
	if err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	if err := h.public.RevokePublisher(c.Request.Context(), actorID, targetTenantID); err != nil {
		_ = c.Error(publicMarketplaceClientError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *PublicMarketplaceHandler) ListVerifiedPublishers(c *gin.Context) {
	views, err := h.public.ListVerifiedPublishers(c.Request.Context())
	if err != nil {
		_ = c.Error(publicMarketplaceClientError(err))
		return
	}
	data := make([]verifiedPublisherResponse, 0, len(views))
	for _, view := range views {
		data = append(data, verifiedPublisherDTO(view))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func (h *PublicMarketplaceHandler) SubmitPublicRelease(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agentMarketplaceMaxRequestBytes)
	var body *submitPublicReleaseBody
	if err := decodeAgentMarketplaceBody(c.Request.Body, &body); err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	if body == nil || strings.TrimSpace(body.SourceListingID) == "" {
		invalidMarketplaceBody(c, stderrors.New("source_listing_id is required"))
		return
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	view, err := h.public.SubmitPublicRelease(c.Request.Context(), sandboxConfigTenantID(c), actorID, body.SourceListingID, body.ReleaseID)
	if err != nil {
		_ = c.Error(publicMarketplaceClientError(err))
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": publicSubmissionDTO(view)})
}

func (h *PublicMarketplaceHandler) ListPublicSubmissions(c *gin.Context) {
	views, err := h.public.ListPublicSubmissions(c.Request.Context(), sandboxConfigTenantID(c))
	if err != nil {
		_ = c.Error(publicMarketplaceClientError(err))
		return
	}
	data := make([]publicSubmissionResponse, 0, len(views))
	for _, view := range views {
		data = append(data, publicSubmissionDTO(view))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func (h *PublicMarketplaceHandler) ListPublicReviewQueue(c *gin.Context) {
	views, err := h.public.ListPublicReviewQueue(c.Request.Context())
	if err != nil {
		_ = c.Error(publicMarketplaceClientError(err))
		return
	}
	data := make([]publicSubmissionResponse, 0, len(views))
	for _, view := range views {
		data = append(data, publicSubmissionDTO(view))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func (h *PublicMarketplaceHandler) ReviewPublicSubmission(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agentMarketplaceMaxRequestBytes)
	var body *reviewAgentReleaseBody
	if err := decodeAgentMarketplaceBody(c.Request.Body, &body); err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	if body == nil || strings.TrimSpace(body.ExpectedDigest) == "" || strings.TrimSpace(body.Decision) == "" {
		invalidMarketplaceBody(c, stderrors.New("expected_digest and decision are required"))
		return
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	result, err := h.public.ReviewPublicSubmission(c.Request.Context(), actorID, c.Param("id"), body.ExpectedDigest, types.AgentReleaseReviewDecision{Decision: body.Decision, Reason: body.Reason})
	if err != nil {
		_ = c.Error(publicMarketplaceClientError(err))
		return
	}
	var review *publicReviewResponse
	if result.Review != nil {
		review = &publicReviewResponse{
			ID: result.Review.ID, SubmissionID: result.Review.SubmissionID, ReviewerID: result.Review.ReviewerID,
			ReviewedDigest: result.Review.ReviewedDigest, Decision: result.Review.Decision, Reason: result.Review.Reason,
			CreatedAt: result.Review.CreatedAt,
		}
	}
	var release *publicReleaseResponse
	if result.Release != nil {
		release = &publicReleaseResponse{
			ID: result.Release.ID, ListingID: result.Release.ListingID, SubmissionID: result.Release.SubmissionID,
			PublisherTenantID: result.Release.PublisherTenantID, ReleaseNumber: result.Release.ReleaseNumber,
			SemanticVersion: result.Release.SemanticVersion, BundleDigest: result.Release.BundleDigest,
			Manifest: json.RawMessage(result.Release.ManifestJSON), DependencyLock: json.RawMessage(result.Release.DependencyLockJSON),
			PublishedBy: result.Release.PublishedBy, CreatedAt: result.Release.CreatedAt,
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"review": review, "release": release}})
}
```

（`parseTenantIDParam` 为本文件私有辅助，放在文件末尾；文件头 import 需补 `"strconv"`：）

```go
// parseTenantIDParam parses the numeric :tenant_id route param; a malformed
// value is a plain 400.
func parseTenantIDParam(raw string) (uint64, error) {
	value, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil || value == 0 {
		return 0, stderrors.New("tenant_id must be a positive integer")
	}
	return value, nil
}
```

- [ ] **Step 3: 写路由与接线**

创建 `internal/router/routes_public_marketplace.go`：

```go
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/handler"
)

// RegisterPublicMarketplaceRoutes mounts the Public Marketplace workflow
// (T30 #60) beside the tenant release/adoption routes.
//
// Tenant-facing surface (spec §2「平台允许的 Tenant」= 本部署全部已认证
// 租户，首版无按租户屏蔽清单):
//   - GET  /marketplace/public/catalog            Viewer+（发现）
//   - GET  /marketplace/public/listings/:id       Viewer+（采用前检视）
//   - POST /marketplace/public/listings/:id/adopt Admin+（spec §13 adopt_agent）
//   - POST /marketplace/public/release-submissions Admin+（Verified Publisher 门槛在服务层）
//   - GET  /marketplace/public/release-submissions Admin+（仅本租户提交）
// 平台面（spec §13 review_public_release / Verified Publisher 治理）全部
// SystemAdmin，且不对 API key 声明策略——沿 routes_auth_tenant.go 中
// promote/revoke 的先例：平台 API key 能力面属后续工作，默认拒绝。
func RegisterPublicMarketplaceRoutes(r *gin.RouterGroup, publicHandler *handler.PublicMarketplaceHandler, g *rbacGuards) {
	if publicHandler == nil {
		return
	}
	admin := apiKeyFullAccess()
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/public/release-submissions", admin, g.Admin(), publicHandler.SubmitPublicRelease)
	g.apiKeyRoute(r, http.MethodGet, "/marketplace/public/release-submissions", admin, g.Admin(), publicHandler.ListPublicSubmissions)
	g.apiKeyRoute(r, http.MethodGet, "/marketplace/public/release-submissions/review-queue", admin, g.SystemAdmin(), publicHandler.ListPublicReviewQueue)
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/public/release-submissions/:id/review", admin, g.SystemAdmin(), publicHandler.ReviewPublicSubmission)
	g.apiKeyRoute(r, http.MethodGet, "/marketplace/public/catalog", admin, g.Viewer(), publicHandler.ListPublicCatalog)
	g.apiKeyRoute(r, http.MethodGet, "/marketplace/public/listings/:id", admin, g.Viewer(), publicHandler.GetPublicListing)
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/public/listings/:id/adopt", admin, g.Admin(), publicHandler.AdoptPublicListing)
	r.GET("/marketplace/public/verified-publishers", g.SystemAdmin(), publicHandler.ListVerifiedPublishers)
	r.POST("/marketplace/public/verified-publishers", g.SystemAdmin(), publicHandler.VerifyPublisher)
	r.DELETE("/marketplace/public/verified-publishers/:tenant_id", g.SystemAdmin(), publicHandler.RevokePublisher)
}
```

注意：Task 6 落地时 Handler 还没有 `ListPublicCatalog`/`GetPublicListing`/`AdoptPublicListing` 三个方法（Task 7 补）——为保持 Task 6 可独立编译通过，Task 6 的路由文件先只注册已存在的方法（`release-submissions` 三条 + `verified-publishers` 三条），Task 7 再追加 catalog/listing/adopt 三行。Step 3 实际写入的初版：

```go
func RegisterPublicMarketplaceRoutes(r *gin.RouterGroup, publicHandler *handler.PublicMarketplaceHandler, g *rbacGuards) {
	if publicHandler == nil {
		return
	}
	admin := apiKeyFullAccess()
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/public/release-submissions", admin, g.Admin(), publicHandler.SubmitPublicRelease)
	g.apiKeyRoute(r, http.MethodGet, "/marketplace/public/release-submissions", admin, g.Admin(), publicHandler.ListPublicSubmissions)
	g.apiKeyRoute(r, http.MethodGet, "/marketplace/public/release-submissions/review-queue", admin, g.SystemAdmin(), publicHandler.ListPublicReviewQueue)
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/public/release-submissions/:id/review", admin, g.SystemAdmin(), publicHandler.ReviewPublicSubmission)
	r.GET("/marketplace/public/verified-publishers", g.SystemAdmin(), publicHandler.ListVerifiedPublishers)
	r.POST("/marketplace/public/verified-publishers", g.SystemAdmin(), publicHandler.VerifyPublisher)
	r.DELETE("/marketplace/public/verified-publishers/:tenant_id", g.SystemAdmin(), publicHandler.RevokePublisher)
}
```

修改 `internal/router/router.go`：第 104 行 `AgentAdoptionHandler *handler.AgentAdoptionHandler` 之后追加一行：

```go
	PublicMarketplaceHandler     *handler.PublicMarketplaceHandler
```

第 404 行 `RegisterAgentAdoptionRoutes(v1, params.AgentAdoptionHandler, rbacGuards)` 之后追加一行：

```go
		RegisterPublicMarketplaceRoutes(v1, params.PublicMarketplaceHandler, rbacGuards)
```

修改 `internal/container/container.go`：第 438 行 `must(container.Provide(handler.NewAgentAdoptionHandler))` 之后追加：

```go
	must(container.Provide(repository.NewPublicMarketplaceRepository))
	must(container.Provide(func(repo repository.PublicMarketplaceRepository, listings interfaces.AgentMarketplaceRepository) interfaces.PublicMarketplaceService {
		return service.NewPublicMarketplaceService(repo, listings)
	}))
	must(container.Provide(handler.NewPublicMarketplaceHandler))
```

- [ ] **Step 4: 运行 Task 6 测试确认通过（GREEN）**

Run: `go test ./internal/router/ -run 'TestPublicMarketplacePublisherVerificationAndSubmissionAuthorization|TestPublicMarketplaceReviewAuthorization' -count=1`
Expected: PASS。
Run: `go build ./internal/...`
Expected: exit 0。

- [ ] **Step 5: Commit**

```bash
git add internal/handler/public_marketplace.go internal/router/routes_public_marketplace.go internal/router/router.go internal/container/container.go internal/router/routes_public_marketplace_test.go
git commit -m "feat(marketplace): verified publisher registry, public submissions and platform review endpoints (T30 #60 task 6)"
```

---

### Task 7: HTTP 面 II：公共目录读 + Adopt（引入）端点

**Files:**
- Modify: `internal/handler/public_marketplace.go`（追加三个方法）
- Modify: `internal/router/routes_public_marketplace.go`（追加三行路由）
- Test: `internal/router/routes_public_marketplace_test.go`（追加测试）

**Interfaces:**
- Consumes: Task 6 的 handler/routes/harness（`publicCall`/`newPublicMarketplaceTestApp`/`publishTenantRelease`）与 Task 5 的 `AdoptPublicListing`/`ListPublicCatalog`/`GetPublicListing`。
- Produces: 端点 `GET /api/v1/marketplace/public/catalog`、`GET /api/v1/marketplace/public/listings/:id`、`POST /api/v1/marketplace/public/listings/:id/adopt`（Task 8 消费）；`adoptPublicListingResponse` wire（Task 9 contracts 镜像）。

- [ ] **Step 1: 写失败测试**

在 `internal/router/routes_public_marketplace_test.go` 末尾追加：

```go
// publicCatalogRow 是目录行 wire 的严格镜像：DisallowUnknownFields 解码
// 使任何新增 adopter 派生字段（adoption 计数、adopter id、metrics）直接
// 破坏测试（AC2 结构性隐私钉）。
type publicCatalogRow struct {
	ID                string    `json:"id"`
	DisplayName       string    `json:"display_name"`
	Summary           string    `json:"summary"`
	State             string    `json:"state"`
	PublisherTenantID uint64    `json:"publisher_tenant_id"`
	PublisherVerified bool      `json:"publisher_verified"`
	CurrentRelease    *struct {
		ID              string          `json:"id"`
		SemanticVersion string          `json:"semantic_version"`
		BundleDigest    string          `json:"bundle_digest"`
		Manifest        json.RawMessage `json:"manifest"`
		DependencyLock  json.RawMessage `json:"dependency_lock"`
		CreatedAt       time.Time       `json:"created_at"`
	} `json:"current_release"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func decodeStrictCatalogRows(t *testing.T, body []byte) []publicCatalogRow {
	t.Helper()
	var envelope struct {
		Success bool                `json:"success"`
		Data    []map[string]any    `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	require.True(t, envelope.Success)
	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err)
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var rows []publicCatalogRow
	require.NoError(t, dec.Decode(&rows))
	return rows
}

// approvePublicRelease walks verify -> submit -> review-approve and returns
// the public listing id + release id.
func approvePublicRelease(t *testing.T, r *gin.Engine, listingID string) (publicListingID, publicReleaseID string) {
	t.Helper()
	_, _ = publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/verified-publishers", "admin", "sysadmin", map[string]any{"tenant_id": 1})
	submitted := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/public/release-submissions", "admin", "tenant-admin", map[string]any{"source_listing_id": listingID})
	require.Equal(t, http.StatusCreated, submitted.Code, submitted.Body.String())
	var submissionBody struct {
		Data struct {
			ID              string `json:"id"`
			PublicListingID string `json:"public_listing_id"`
			BundleDigest    string `json:"bundle_digest"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(submitted.Body.Bytes(), &submissionBody))
	approved := publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/release-submissions/"+submissionBody.Data.ID+"/review", "admin", "sysadmin", map[string]any{"expected_digest": submissionBody.Data.BundleDigest, "decision": "approved"})
	require.Equal(t, http.StatusOK, approved.Code, approved.Body.String())
	var reviewBody struct {
		Data struct {
			Release struct {
				ID string `json:"id"`
			} `json:"release"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(approved.Body.Bytes(), &reviewBody))
	require.NotEmpty(t, reviewBody.Data.Release.ID)
	return submissionBody.Data.PublicListingID, reviewBody.Data.Release.ID
}

func TestPublicMarketplaceCatalogAndAdoptAuthorization(t *testing.T) {
	r, _, _ := newPublicMarketplaceTestApp(t)
	listingID := publishTenantRelease(t, r)
	publicListingID, publicReleaseID := approvePublicRelease(t, r, listingID)

	// 目录：Viewer+ 可读；严格解码钉死字段集
	catalog := publicCall(r, 2, false, http.MethodGet, "/api/v1/marketplace/public/catalog", "viewer", "viewer-2", nil)
	require.Equal(t, http.StatusOK, catalog.Code, catalog.Body.String())
	rows := decodeStrictCatalogRows(t, catalog.Body.Bytes())
	require.Len(t, rows, 1)
	require.Equal(t, publicListingID, rows[0].ID)
	require.True(t, rows[0].PublisherVerified)
	require.NotNil(t, rows[0].CurrentRelease)
	require.Equal(t, publicReleaseID, rows[0].CurrentRelease.ID)
	require.NotEmpty(t, rows[0].CurrentRelease.Manifest)

	// Listing 详情：Viewer+ 可读
	detail := publicCall(r, 2, false, http.MethodGet, "/api/v1/marketplace/public/listings/"+publicListingID, "viewer", "viewer-2", nil)
	require.Equal(t, http.StatusOK, detail.Code, detail.Body.String())

	// Adopt：Viewer 403，Admin 201；幂等重放 200
	viewerAdopt := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/public/listings/"+publicListingID+"/adopt", "viewer", "viewer-2", map[string]any{})
	require.Equal(t, http.StatusForbidden, viewerAdopt.Code, viewerAdopt.Body.String())
	adopted := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/public/listings/"+publicListingID+"/adopt", "admin", "adopter-admin", map[string]any{})
	require.Equal(t, http.StatusCreated, adopted.Code, adopted.Body.String())
	var adoptBody struct {
		Data struct {
			Introduction struct {
				ID              string `json:"id"`
				PublicListingID string `json:"public_listing_id"`
				PublicReleaseID string `json:"public_release_id"`
				BundleDigest    string `json:"bundle_digest"`
			} `json:"introduction"`
			Adoption struct {
				ID                string `json:"id"`
				ListingID         string `json:"listing_id"`
				AcceptedReleaseID string `json:"accepted_release_id"`
				State             string `json:"state"`
			} `json:"adoption"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(adopted.Body.Bytes(), &adoptBody))
	require.Equal(t, publicListingID, adoptBody.Data.Introduction.PublicListingID)
	require.Equal(t, publicReleaseID, adoptBody.Data.Introduction.PublicReleaseID)
	require.Equal(t, adoptBody.Data.Introduction.ID, adoptBody.Data.Adoption.AcceptedReleaseID)
	require.Equal(t, "active", adoptBody.Data.Adoption.State)

	readopt := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/public/listings/"+publicListingID+"/adopt", "admin", "adopter-admin", map[string]any{})
	require.Equal(t, http.StatusOK, readopt.Code, readopt.Body.String())

	// 采用方租户经 #59 端点看到引入关系
	adoptions := publicCall(r, 2, false, http.MethodGet, "/api/v1/marketplace/tenant/adoptions", "admin", "adopter-admin", nil)
	require.Equal(t, http.StatusOK, adoptions.Code, adoptions.Body.String())
	require.Contains(t, adoptions.Body.String(), adoptBody.Data.Adoption.ID)

	// 未知 listing / release → 404
	missing := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/public/listings/no-such-listing/adopt", "admin", "adopter-admin", map[string]any{})
	require.Equal(t, http.StatusNotFound, missing.Code, missing.Body.String())
	missingRelease := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/public/listings/"+publicListingID+"/adopt", "admin", "adopter-admin", map[string]any{"release_id": "no-such-release"})
	require.Equal(t, http.StatusNotFound, missingRelease.Code, missingRelease.Body.String())
}
```

（该文件 import 需补 `"time"`。）

Run: `go test ./internal/router/ -run 'TestPublicMarketplaceCatalogAndAdoptAuthorization' -count=1`
Expected: FAIL —— `catalog` 请求返回 404（`GET /api/v1/marketplace/public/catalog` 路由尚未注册；Step 2 注册三行并补齐 Handler 方法后才转绿）。

- [ ] **Step 2: 实现 Handler 三方法与路由三行**

在 `internal/handler/public_marketplace.go` 末尾追加：

```go
func (h *PublicMarketplaceHandler) ListPublicCatalog(c *gin.Context) {
	entries, err := h.public.ListPublicCatalog(c.Request.Context())
	if err != nil {
		_ = c.Error(publicMarketplaceClientError(err))
		return
	}
	data := make([]publicCatalogListingResponse, 0, len(entries))
	for _, entry := range entries {
		data = append(data, publicCatalogListingDTO(entry))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func (h *PublicMarketplaceHandler) GetPublicListing(c *gin.Context) {
	entry, err := h.public.GetPublicListing(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(publicMarketplaceClientError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": publicCatalogListingDTO(entry.PublicCatalogEntryView)})
}

func (h *PublicMarketplaceHandler) AdoptPublicListing(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agentMarketplaceMaxRequestBytes)
	var body *adoptPublicListingBody
	if err := decodeAgentMarketplaceBody(c.Request.Body, &body); err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	releaseID := ""
	if body != nil {
		releaseID = body.ReleaseID
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	result, created, err := h.public.AdoptPublicListing(c.Request.Context(), sandboxConfigTenantID(c), actorID, c.Param("id"), releaseID)
	if err != nil {
		_ = c.Error(publicMarketplaceClientError(err))
		return
	}
	response := adoptPublicListingResponse{
		Introduction: publicIntroductionResponse{
			ID: result.Introduction.ID, PublicListingID: result.Introduction.PublicListingID,
			PublicReleaseID: result.Introduction.PublicReleaseID, DisplayName: result.Introduction.DisplayName,
			Summary: result.Introduction.Summary, SemanticVersion: result.Introduction.SemanticVersion,
			BundleDigest: result.Introduction.BundleDigest, IntroducedBy: result.Introduction.IntroducedBy,
			IntroducedAt: result.Introduction.IntroducedAt,
		},
		Adoption: publicAdoptionSummaryResponse{
			ID: result.Adoption.ID, ListingID: result.Adoption.ListingID,
			AcceptedReleaseID: result.Adoption.AcceptedReleaseID, State: result.Adoption.State,
			CreatedBy: result.Adoption.CreatedBy, CreatedAt: result.Adoption.CreatedAt, UpdatedAt: result.Adoption.UpdatedAt,
		},
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, gin.H{"success": true, "data": response})
}
```

在 `internal/router/routes_public_marketplace.go` 的 `RegisterPublicMarketplaceRoutes` 中、`verified-publishers` 三行之前追加（即恢复 Step 3 设计注释中的完整版三行）：

```go
	g.apiKeyRoute(r, http.MethodGet, "/marketplace/public/catalog", admin, g.Viewer(), publicHandler.ListPublicCatalog)
	g.apiKeyRoute(r, http.MethodGet, "/marketplace/public/listings/:id", admin, g.Viewer(), publicHandler.GetPublicListing)
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/public/listings/:id/adopt", admin, g.Admin(), publicHandler.AdoptPublicListing)
```

- [ ] **Step 3: 运行测试确认通过（GREEN）**

Run: `go test ./internal/router/ -run 'TestPublicMarketplace' -count=1`
Expected: PASS（Task 6 两个 + Task 7 一个）。

- [ ] **Step 4: Commit**

```bash
git add internal/handler/public_marketplace.go internal/router/routes_public_marketplace.go internal/router/routes_public_marketplace_test.go
git commit -m "feat(marketplace): public catalog reads and cross-tenant adopt endpoint (T30 #60 task 7)"
```

---

### Task 8: 端到端 AC 证据（AC1/AC2/AC3）

**Files:**
- Test: `internal/router/routes_public_marketplace_test.go`（追加两个测试）

**Interfaces:**
- Consumes: Task 6/7 的 harness 与端点；#59 的 Variant 链端点（`POST /marketplace/tenant/adoptions/:id/variants`、`PUT /marketplace/tenant/variants/:id/capability-mapping`、`POST .../test`、`POST .../publish`）；#33 的 `GET /api/v1/agents` wire。
- Produces: AC1/AC2/AC3 的最高稳定 Interface 证据（本计划验收产物；不新增生产代码）。

- [ ] **Step 1: 写端到端测试（先跑，确认失败或通过均如实记录；本测试是验收证据，RED 阶段即应失败在缺失行为上——Task 3-7 已实现，预期直接通过；若失败则按 systematic-debugging 修复实现而非测试）**

在 `internal/router/routes_public_marketplace_test.go` 末尾追加（无需新增 import）：

```go
// TestPublicMarketplaceCrossTenantEndToEndAndPrivacy 是 Issue #60 三条验收
// 标准的最高稳定 Interface 证据（真实 sqlite 迁移流 + 真实服务栈 + 真实
// HTTP）：
//   AC1 跨 Tenant 只传播可移植 Release —— 发布者 Agent 的 KB/模型绑定
//        （kb-publisher / model-publisher）不进入采用方本地 Agent；采用方
//        的知识绑定完全来自本地映射（kb-tenant-2）。
//   AC2 发布者和源 Tenant 无法读取采用方身份、映射或 Task 内容 ——
//        发布者视角（tenant 1）全部可见响应不含任何采用方标识；公共目录
//        行字段集被严格解码钉死。
//   AC3 全链真实 HTTP：freeze → tenant release → tenant review → verify →
//        public submit → platform review → catalog → adopt → variant →
//        mapping → test → publish → GET /api/v1/agents（#33 移动 Resource
//        Shelf wire）→ available-agents。
func TestPublicMarketplaceCrossTenantEndToEndAndPrivacy(t *testing.T) {
	r, _, db := newPublicMarketplaceTestApp(t)
	listingID := publishTenantRelease(t, r)
	publicListingID, publicReleaseID := approvePublicRelease(t, r, listingID)

	// --- 采用方（tenant 2）发现并引入 ---
	catalog := publicCall(r, 2, false, http.MethodGet, "/api/v1/marketplace/public/catalog", "viewer", "viewer-2", nil)
	require.Equal(t, http.StatusOK, catalog.Code, catalog.Body.String())
	rows := decodeStrictCatalogRows(t, catalog.Body.Bytes())
	require.Len(t, rows, 1)

	adopted := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/public/listings/"+publicListingID+"/adopt", "admin", "adopter-admin", map[string]any{})
	require.Equal(t, http.StatusCreated, adopted.Code, adopted.Body.String())
	var adoptBody struct {
		Data struct {
			Introduction struct {
				ID string `json:"id"`
			} `json:"introduction"`
			Adoption struct {
				ID                string `json:"id"`
				AcceptedReleaseID string `json:"accepted_release_id"`
			} `json:"adoption"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(adopted.Body.Bytes(), &adoptBody))
	introductionID := adoptBody.Data.Introduction.ID
	adoptionID := adoptBody.Data.Adoption.ID
	require.NotEmpty(t, introductionID)
	require.Equal(t, introductionID, adoptBody.Data.Adoption.AcceptedReleaseID)

	// --- 采用方本地 Variant 链（#59 端点原样） ---
	variant := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionID+"/variants", "admin", "adopter-admin", map[string]any{"name": "Imported helper"})
	require.Equal(t, http.StatusCreated, variant.Code, variant.Body.String())
	var variantBody struct {
		Data struct {
			ID                  string   `json:"id"`
			ReleaseID           string   `json:"release_id"`
			State               string   `json:"state"`
			MissingCapabilities []string `json:"missing_capabilities"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(variant.Body.Bytes(), &variantBody))
	require.Equal(t, introductionID, variantBody.Data.ReleaseID, "Variant 固定引入 release")
	require.Equal(t, []string{"knowledge"}, variantBody.Data.MissingCapabilities)

	mapped := publicCall(r, 2, false, http.MethodPut, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/capability-mapping", "admin", "adopter-admin", map[string]any{"mappings": []map[string]any{{"capability": "knowledge", "knowledge_base_ids": []string{"kb-tenant-2"}}}})
	require.Equal(t, http.StatusOK, mapped.Code, mapped.Body.String())
	tested := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/test", "admin", "adopter-admin", nil)
	require.Equal(t, http.StatusOK, tested.Code, tested.Body.String())
	published := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/publish", "admin", "adopter-admin", nil)
	require.Equal(t, http.StatusOK, published.Code, published.Body.String())
	var publishedBody struct {
		Data struct {
			LocalAgentID string `json:"local_agent_id"`
			State        string `json:"state"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(published.Body.Bytes(), &publishedBody))
	require.Equal(t, "published", publishedBody.Data.State)
	localAgentID := publishedBody.Data.LocalAgentID
	require.NotEmpty(t, localAgentID)

	// --- AC1/AC3：移动 Resource Shelf wire（#33）中传播产物只含可移植内容 ---
	agents2 := publicCall(r, 2, false, http.MethodGet, "/api/v1/agents", "viewer", "viewer-2", nil)
	require.Equal(t, http.StatusOK, agents2.Code, agents2.Body.String())
	var agentsBody struct {
		Data []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Config struct {
				SystemPrompt   string   `json:"system_prompt"`
				KnowledgeBases []string `json:"knowledge_bases"`
				ModelID        string   `json:"model_id"`
			} `json:"config"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(agents2.Body.Bytes(), &agentsBody))
	var imported *struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Config struct {
			SystemPrompt   string   `json:"system_prompt"`
			KnowledgeBases []string `json:"knowledge_bases"`
			ModelID        string   `json:"model_id"`
		} `json:"config"`
	}
	for i := range agentsBody.Data {
		if agentsBody.Data[i].ID == localAgentID {
			imported = &agentsBody.Data[i]
		}
	}
	require.NotNil(t, imported, "发布后的本地 Agent 必须出现在 GET /api/v1/agents（移动 Resource Shelf wire）")
	require.Equal(t, "Be portable.", imported.Config.SystemPrompt, "可移植 system prompt 随传播")
	require.Equal(t, []string{"kb-tenant-2"}, imported.Config.KnowledgeBases, "AC1：知识绑定只来自采用方本地映射")
	require.Equal(t, "", imported.Config.ModelID, "AC1：发布者模型绑定（model-publisher）不得传播")
	require.NotContains(t, agents2.Body.String(), "kb-publisher", "AC1：发布者 KB 不外泄")
	require.NotContains(t, agents2.Body.String(), "model-publisher", "AC1：发布者模型不外泄")

	available := publicCall(r, 2, false, http.MethodGet, "/api/v1/marketplace/tenant/available-agents", "viewer", "viewer-2", nil)
	require.Equal(t, http.StatusOK, available.Code, available.Body.String())
	require.Contains(t, available.Body.String(), localAgentID)

	// --- AC2：发布者（tenant 1）与源租户可见面零采用方痕迹 ---
	publisherAgents := publicCall(r, 1, false, http.MethodGet, "/api/v1/agents", "admin", "tenant-admin", nil)
	require.Equal(t, http.StatusOK, publisherAgents.Code)
	require.NotContains(t, publisherAgents.Body.String(), localAgentID)
	require.NotContains(t, publisherAgents.Body.String(), "Imported helper")
	require.NotContains(t, publisherAgents.Body.String(), "kb-tenant-2")

	publisherAdoptions := publicCall(r, 1, false, http.MethodGet, "/api/v1/marketplace/tenant/adoptions", "admin", "tenant-admin", nil)
	require.Equal(t, http.StatusOK, publisherAdoptions.Code)
	var publisherAdoptionsEnvelope struct {
		Data []json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(publisherAdoptions.Body.Bytes(), &publisherAdoptionsEnvelope))
	require.Empty(t, publisherAdoptionsEnvelope.Data, "AC2：发布者视角无任何（含采用方的）Adoption 行")

	publisherCatalog := publicCall(r, 1, false, http.MethodGet, "/api/v1/marketplace/public/catalog", "viewer", "viewer-1", nil)
	require.Equal(t, http.StatusOK, publisherCatalog.Code)
	for _, secret := range []string{adoptionID, variantBody.Data.ID, introductionID, localAgentID, "kb-tenant-2", "Imported helper", "adopter-admin"} {
		require.NotContains(t, publisherCatalog.Body.String(), secret, "AC2：公共目录不得含采用方标识")
	}

	publisherSubmissions := publicCall(r, 1, false, http.MethodGet, "/api/v1/marketplace/public/release-submissions", "admin", "tenant-admin", nil)
	require.Equal(t, http.StatusOK, publisherSubmissions.Code)
	require.NotContains(t, publisherSubmissions.Body.String(), adoptionID)

	// 平台审核面也不含采用方数据（结构钉死：队列行=提交行字段集）
	queue := publicCall(r, 1, true, http.MethodGet, "/api/v1/marketplace/public/release-submissions/review-queue", "admin", "sysadmin", nil)
	require.Equal(t, http.StatusOK, queue.Code)
	require.NotContains(t, queue.Body.String(), adoptionID)
	require.NotContains(t, queue.Body.String(), localAgentID)

	// db 侧证：采用方行只存在于 tenant 2 作用域
	var adoptionRows int64
	db.Table("agent_adoptions").Where("listing_id = ?", publicListingID).Count(&adoptionRows)
	require.Equal(t, int64(1), adoptionRows)
	db.Table("agent_adoptions").Where("tenant_id = ? AND listing_id = ?", uint64(2), publicListingID).Count(&adoptionRows)
	require.Equal(t, int64(1), adoptionRows)
}

func TestPublicMarketplaceAdoptRejectsTamperedPublicRelease(t *testing.T) {
	r, _, db := newPublicMarketplaceTestApp(t)
	listingID := publishTenantRelease(t, r)
	publicListingID, publicReleaseID := approvePublicRelease(t, r, listingID)

	// 公共 release 字节被篡改（digest 不再匹配）：引入必须拒绝且零行落库
	require.NoError(t, db.Exec("UPDATE public_agent_releases SET bundle = ? WHERE id = ?", []byte(`{"payload":{"system_prompt":"evil"}}`), publicReleaseID).Error)
	rejected := publicCall(r, 2, false, http.MethodPost, "/api/v1/marketplace/public/listings/"+publicListingID+"/adopt", "admin", "adopter-admin", map[string]any{})
	require.Equal(t, http.StatusConflict, rejected.Code, rejected.Body.String())

	var introducedCount, adoptionCount int64
	db.Table("tenant_introduced_releases").Where("tenant_id = ?", uint64(2)).Count(&introducedCount)
	db.Table("agent_adoptions").Where("tenant_id = ?", uint64(2)).Count(&adoptionCount)
	require.Zero(t, introducedCount, "AC1：篡改的公共 release 不得产生引入行")
	require.Zero(t, adoptionCount, "AC1：篡改的公共 release 不得产生 Adoption")
}
```

注意：`publisherAdoptions` 用 `json.RawMessage` 切片反序列化断言 `data` 为空数组——不依赖 gin 信封的具体包裹方式（#59 `ListAdoptions` 以 `make([]adoptionResponse, 0, ...)` 初始化，空时 `data` 序列化为 `[]`；该断言对 `null` 与 `[]` 均判空，语义为「发布者视角零 Adoption 行」）。

Run: `go test ./internal/router/ -run 'TestPublicMarketplaceCrossTenantEndToEndAndPrivacy|TestPublicMarketplaceAdoptRejectsTamperedPublicRelease' -count=1`
Expected: PASS（Task 3-7 已实现全部行为；此测试为验收证据。若失败：按 superpowers:systematic-debugging 定位实现缺陷修复，不得修改断言语义）。

- [ ] **Step 2: 复跑本计划全部 router 测试与 #58/#59 回归**

Run: `go test ./internal/router/ -run 'TestPublicMarketplace|TestTenantAgentMarketplace|TestTenantAgentAdoption|TestTenantAgentVariant|TestAvailableAgentsAPIKeyFloor' -count=1`
Expected: PASS。

- [ ] **Step 3: Commit**

```bash
git add internal/router/routes_public_marketplace_test.go
git commit -m "test(marketplace): cross-tenant end-to-end adoption, portable-only propagation and adopter privacy evidence (T30 #60 task 8)"
```

---

### Task 9: contracts 契约层

**Files:**
- Create: `packages/contracts/src/marketplace/public-marketplace.ts`
- Create: `packages/contracts/src/marketplace/public-marketplace.test.ts`
- Modify: `packages/contracts/src/index.ts:730`（追加两行导出）

**Interfaces:**
- Consumes: Task 6/7 的 wire 形态（`publicCatalogListingResponse`/`publicSubmissionResponse`/`publicReviewResponse`/`verifiedPublisherResponse`/`adoptPublicListingResponse`，信封 `{ success, data }`）；`agent-adoption.ts` 的本地辅助函数模式（`packages/contracts/src/marketplace/agent-adoption.ts:28-56`）与 `ContractError`。
- Produces: `@weknora/contracts` 根导出类型与解析器（见计划头部 Produces；供 #61-#65 与 Web 管理端消费）。

- [ ] **Step 1: 写失败测试**

创建 `packages/contracts/src/marketplace/public-marketplace.test.ts`：

```ts
import assert from 'node:assert/strict';
import test from 'node:test';
import { ContractError } from '../index.ts';
import {
  parseVerifiedPublisherResponse,
  parseVerifiedPublisherListResponse,
  parsePublicCatalogListResponse,
  parsePublicListingResponse,
  parsePublicSubmissionResponse,
  parsePublicSubmissionListResponse,
  parsePublicReviewResponse,
  parseAdoptPublicListingResponse,
} from './public-marketplace.ts';

const publisher = { tenant_id: 1, state: 'verified', verified_by: 'sysadmin', note: 'identity checked', verified_at: '2026-09-24T00:00:00Z', updated_at: '2026-09-24T00:00:00Z' };
const catalogRow = {
  id: 'pub-listing-1', display_name: 'Public helper', summary: 'A portable helper', state: 'listed',
  publisher_tenant_id: 1, publisher_verified: true,
  current_release: {
    id: 'pub-release-1', semantic_version: '1.0.0', bundle_digest: 'a'.repeat(64),
    manifest: { capability_requirements: ['knowledge'] }, dependency_lock: { dependencies: [] },
    created_at: '2026-09-24T00:00:00Z',
  },
  created_at: '2026-09-24T00:00:00Z', updated_at: '2026-09-24T00:00:00Z',
};
const submission = {
  id: 'pub-sub-1', publisher_tenant_id: 1, public_listing_id: 'pub-listing-1',
  source_listing_id: 'tenant-listing-1', source_release_id: 'tenant-release-1',
  publisher_actor_id: 'tenant-admin', semantic_version: '1.0.0', bundle_digest: 'a'.repeat(64),
  manifest: {}, dependency_lock: {}, status: 'submitted', created_at: '2026-09-24T00:00:00Z',
};
const review = { id: 'rev-1', submission_id: 'pub-sub-1', reviewer_id: 'platform-reviewer', reviewed_digest: 'a'.repeat(64), decision: 'approved', reason: '', created_at: '2026-09-24T00:00:00Z' };
const adoptResult = {
  introduction: {
    id: 'introduced-1', public_listing_id: 'pub-listing-1', public_release_id: 'pub-release-1',
    display_name: 'Public helper', summary: 'A portable helper', semantic_version: '1.0.0',
    bundle_digest: 'a'.repeat(64), introduced_by: 'adopter-admin', introduced_at: '2026-09-24T00:00:00Z',
  },
  adoption: {
    id: 'adoption-1', listing_id: 'pub-listing-1', accepted_release_id: 'introduced-1',
    state: 'active', created_by: 'adopter-admin', created_at: '2026-09-24T00:00:00Z', updated_at: '2026-09-24T00:00:00Z',
  },
};

test('parses verified publishers, catalog rows and listing detail', () => {
  assert.equal(parseVerifiedPublisherResponse({ success: true, data: publisher }).state, 'verified');
  assert.deepEqual(parseVerifiedPublisherListResponse({ success: true, data: [publisher] }), [publisher]);
  const rows = parsePublicCatalogListResponse({ success: true, data: [catalogRow] });
  assert.equal(rows[0].current_release?.semantic_version, '1.0.0');
  assert.equal(rows[0].publisher_verified, true);
  assert.equal(parsePublicListingResponse({ success: true, data: catalogRow }).id, catalogRow.id);
  assert.deepEqual(parsePublicCatalogListResponse({ success: true, data: [] }), []);
});

test('parses submissions, review and adoption result', () => {
  assert.equal(parsePublicSubmissionResponse({ success: true, data: submission }).status, 'submitted');
  assert.deepEqual(parsePublicSubmissionListResponse({ success: true, data: [submission] }), [submission]);
  const parsed = parsePublicReviewResponse({ success: true, data: { review, release: null } });
  assert.equal(parsed.review.decision, 'approved');
  const adopted = parseAdoptPublicListingResponse({ success: true, data: adoptResult });
  assert.equal(adopted.adoption.accepted_release_id, adopted.introduction.id);
});

test('rejects malformed envelopes, ids, digests, states and revoked rows in list context', () => {
  assert.throws(() => parsePublicCatalogListResponse({ success: false, data: [catalogRow] }), ContractError);
  assert.throws(() => parsePublicCatalogListResponse({ success: true, data: catalogRow }), ContractError);
  assert.throws(() => parsePublicCatalogListResponse({ success: true, data: [{ ...catalogRow, id: '' }] }), ContractError);
  assert.throws(() => parsePublicCatalogListResponse({ success: true, data: [{ ...catalogRow, current_release: { ...catalogRow.current_release, bundle_digest: 'not-hex' } }] }), ContractError);
  assert.throws(() => parsePublicSubmissionResponse({ success: true, data: { ...submission, publisher_tenant_id: 0 } }), ContractError);
  assert.throws(() => parsePublicReviewResponse({ success: true, data: { review: { ...review, decision: 'maybe' }, release: null } }), ContractError);
  assert.throws(() => parseVerifiedPublisherResponse({ success: true, data: { ...publisher, state: 'unknown' } }), ContractError);
  assert.throws(() => parseAdoptPublicListingResponse({ success: true, data: { ...adoptResult, adoption: { ...adoptResult.adoption, listing_id: '' } } }), ContractError);
});
```

Run: `npx tsx --test packages/contracts/src/marketplace/public-marketplace.test.ts`
Expected: FAIL —— 模块不存在（Cannot find module './public-marketplace.ts'）。

- [ ] **Step 2: 写契约实现**

创建 `packages/contracts/src/marketplace/public-marketplace.ts`：

```ts
import { ContractError } from '../index.ts';

export interface VerifiedPublisher {
  tenant_id: number;
  state: 'verified' | 'revoked';
  verified_by: string;
  note: string;
  verified_at: string;
  updated_at: string;
  [key: string]: unknown;
}

export interface PublicCatalogRelease {
  id: string;
  semantic_version: string;
  bundle_digest: string;
  manifest: Record<string, unknown>;
  dependency_lock: Record<string, unknown>;
  created_at: string;
  [key: string]: unknown;
}

/**
 * Public catalog row. The field set mirrors the Go wire
 * (publicCatalogListingResponse) exactly: it carries listing, publisher
 * trust signal and current release documentation ONLY — never adopter
 * identity, adoption counts, mappings or task data (spec §12).
 */
export interface PublicCatalogListing {
  id: string;
  display_name: string;
  summary: string;
  state: string;
  publisher_tenant_id: number;
  publisher_verified: boolean;
  current_release?: PublicCatalogRelease;
  created_at: string;
  updated_at: string;
  [key: string]: unknown;
}

export interface PublicReleaseSubmission {
  id: string;
  publisher_tenant_id: number;
  public_listing_id: string;
  source_listing_id: string;
  source_release_id: string;
  publisher_actor_id: string;
  semantic_version: string;
  bundle_digest: string;
  manifest: Record<string, unknown>;
  dependency_lock: Record<string, unknown>;
  status: string;
  created_at: string;
  [key: string]: unknown;
}

export interface PublicReleaseReview {
  id: string;
  submission_id: string;
  reviewer_id: string;
  reviewed_digest: string;
  decision: 'approved' | 'rejected' | 'changes_requested';
  reason: string;
  created_at: string;
  [key: string]: unknown;
}

export interface PublicIntroduction {
  id: string;
  public_listing_id: string;
  public_release_id: string;
  display_name: string;
  summary: string;
  semantic_version: string;
  bundle_digest: string;
  introduced_by: string;
  introduced_at: string;
  [key: string]: unknown;
}

export interface PublicAdoption {
  id: string;
  listing_id: string;
  accepted_release_id: string;
  state: string;
  created_by: string;
  created_at: string;
  updated_at: string;
  [key: string]: unknown;
}

export interface AdoptPublicListingResult {
  introduction: PublicIntroduction;
  adoption: PublicAdoption;
  [key: string]: unknown;
}

const REVIEW_DECISIONS = ['approved', 'rejected', 'changes_requested'] as const;
const PUBLISHER_STATES = ['verified', 'revoked'] as const;

function object(value: unknown, path: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new ContractError(path, 'expected an object');
  return value as Record<string, unknown>;
}

function envelope(value: unknown): Record<string, unknown> {
  const row = object(value, 'response');
  if (row.success !== true) throw new ContractError('response.success', 'expected success to be true');
  return row;
}

function requiredString(row: Record<string, unknown>, key: string, path: string): string {
  const value = row[key];
  if (typeof value !== 'string' || value.trim() === '') throw new ContractError(`${path}.${key}`, 'expected a non-empty string');
  return value;
}

function requiredDocument(row: Record<string, unknown>, key: string, path: string): Record<string, unknown> {
  return object(row[key], `${path}.${key}`);
}

function requiredDigest(row: Record<string, unknown>, path: string): string {
  const digest = requiredString(row, 'bundle_digest', path);
  if (!/^[0-9a-f]{64}$/.test(digest)) throw new ContractError(`${path}.bundle_digest`, 'expected a lowercase sha-256 hex digest');
  return digest;
}

function oneOf<T extends readonly string[]>(row: Record<string, unknown>, key: string, values: T, path: string): T[number] {
  const value = row[key];
  if (typeof value !== 'string' || !(values as readonly string[]).includes(value)) {
    throw new ContractError(`${path}.${key}`, `expected one of ${values.join('|')}`);
  }
  return value as T[number];
}

function verifiedPublisher(row: Record<string, unknown>, path: string): VerifiedPublisher {
  return {
    tenant_id: row.tenant_id as number,
    state: oneOf(row, 'state', PUBLISHER_STATES, path),
    verified_by: requiredString(row, 'verified_by', path),
    note: typeof row.note === 'string' ? row.note : '',
    verified_at: requiredString(row, 'verified_at', path),
    updated_at: requiredString(row, 'updated_at', path),
  };
}

function publicCatalogRelease(row: Record<string, unknown>, path: string): PublicCatalogRelease {
  return {
    id: requiredString(row, 'id', path),
    semantic_version: requiredString(row, 'semantic_version', path),
    bundle_digest: requiredDigest(row, path),
    manifest: requiredDocument(row, 'manifest', path),
    dependency_lock: requiredDocument(row, 'dependency_lock', path),
    created_at: requiredString(row, 'created_at', path),
  };
}

function publicCatalogListing(row: Record<string, unknown>, path: string): PublicCatalogListing {
  const listing: PublicCatalogListing = {
    id: requiredString(row, 'id', path),
    display_name: requiredString(row, 'display_name', path),
    summary: typeof row.summary === 'string' ? row.summary : '',
    state: requiredString(row, 'state', path),
    publisher_tenant_id: row.publisher_tenant_id as number,
    publisher_verified: row.publisher_verified === true,
    created_at: requiredString(row, 'created_at', path),
    updated_at: requiredString(row, 'updated_at', path),
  };
  if (row.current_release !== null && row.current_release !== undefined) {
    listing.current_release = publicCatalogRelease(object(row.current_release, `${path}.current_release`), `${path}.current_release`);
  }
  return listing;
}

function publicSubmission(row: Record<string, unknown>, path: string): PublicReleaseSubmission {
  return {
    id: requiredString(row, 'id', path),
    publisher_tenant_id: row.publisher_tenant_id as number,
    public_listing_id: requiredString(row, 'public_listing_id', path),
    source_listing_id: requiredString(row, 'source_listing_id', path),
    source_release_id: requiredString(row, 'source_release_id', path),
    publisher_actor_id: typeof row.publisher_actor_id === 'string' ? row.publisher_actor_id : '',
    semantic_version: requiredString(row, 'semantic_version', path),
    bundle_digest: requiredDigest(row, path),
    manifest: requiredDocument(row, 'manifest', path),
    dependency_lock: requiredDocument(row, 'dependency_lock', path),
    status: requiredString(row, 'status', path),
    created_at: requiredString(row, 'created_at', path),
  };
}

function publicAdoption(row: Record<string, unknown>, path: string): PublicAdoption {
  return {
    id: requiredString(row, 'id', path),
    listing_id: requiredString(row, 'listing_id', path),
    accepted_release_id: requiredString(row, 'accepted_release_id', path),
    state: requiredString(row, 'state', path),
    created_by: typeof row.created_by === 'string' ? row.created_by : '',
    created_at: requiredString(row, 'created_at', path),
    updated_at: requiredString(row, 'updated_at', path),
  };
}

export function parseVerifiedPublisherResponse(value: unknown): VerifiedPublisher {
  return verifiedPublisher(object(envelope(value).data, 'response.data'), 'response.data');
}

export function parseVerifiedPublisherListResponse(value: unknown): VerifiedPublisher[] {
  const rows = envelope(value).data;
  if (!Array.isArray(rows)) throw new ContractError('response.data', 'expected an array');
  return rows.map((row, index) => verifiedPublisher(object(row, `response.data[${index}]`), `response.data[${index}]`));
}

export function parsePublicCatalogListResponse(value: unknown): PublicCatalogListing[] {
  const rows = envelope(value).data;
  if (!Array.isArray(rows)) throw new ContractError('response.data', 'expected an array');
  return rows.map((row, index) => publicCatalogListing(object(row, `response.data[${index}]`), `response.data[${index}]`));
}

export function parsePublicListingResponse(value: unknown): PublicCatalogListing {
  return publicCatalogListing(object(envelope(value).data, 'response.data'), 'response.data');
}

export function parsePublicSubmissionResponse(value: unknown): PublicReleaseSubmission {
  return publicSubmission(object(envelope(value).data, 'response.data'), 'response.data');
}

export function parsePublicSubmissionListResponse(value: unknown): PublicReleaseSubmission[] {
  const rows = envelope(value).data;
  if (!Array.isArray(rows)) throw new ContractError('response.data', 'expected an array');
  return rows.map((row, index) => publicSubmission(object(row, `response.data[${index}]`), `response.data[${index}]`));
}

export function parsePublicReviewResponse(value: unknown): { review: PublicReleaseReview } {
  const row = object(envelope(value).data, 'response.data');
  const reviewRow = object(row.review, 'response.data.review');
  return {
    review: {
      id: requiredString(reviewRow, 'id', 'response.data.review'),
      submission_id: requiredString(reviewRow, 'submission_id', 'response.data.review'),
      reviewer_id: requiredString(reviewRow, 'reviewer_id', 'response.data.review'),
      reviewed_digest: requiredDigest(reviewRow, 'response.data.review'),
      decision: oneOf(reviewRow, 'decision', REVIEW_DECISIONS, 'response.data.review'),
      reason: typeof reviewRow.reason === 'string' ? reviewRow.reason : '',
      created_at: requiredString(reviewRow, 'created_at', 'response.data.review'),
    },
  };
}

export function parseAdoptPublicListingResponse(value: unknown): AdoptPublicListingResult {
  const row = object(envelope(value).data, 'response.data');
  return {
    introduction: (() => {
      const introduction = object(row.introduction, 'response.data.introduction');
      return {
        id: requiredString(introduction, 'id', 'response.data.introduction'),
        public_listing_id: requiredString(introduction, 'public_listing_id', 'response.data.introduction'),
        public_release_id: requiredString(introduction, 'public_release_id', 'response.data.introduction'),
        display_name: requiredString(introduction, 'display_name', 'response.data.introduction'),
        summary: typeof introduction.summary === 'string' ? introduction.summary : '',
        semantic_version: requiredString(introduction, 'semantic_version', 'response.data.introduction'),
        bundle_digest: requiredDigest(introduction, 'response.data.introduction'),
        introduced_by: typeof introduction.introduced_by === 'string' ? introduction.introduced_by : '',
        introduced_at: requiredString(introduction, 'introduced_at', 'response.data.introduction'),
      };
    })(),
    adoption: publicAdoption(object(row.adoption, 'response.data.adoption'), 'response.data.adoption'),
  };
}
```

- [ ] **Step 3: 挂根导出**

修改 `packages/contracts/src/index.ts`，在 `:730`（adoption 导出行）之后追加：

```ts
export type { VerifiedPublisher, PublicCatalogListing, PublicCatalogRelease, PublicReleaseSubmission, PublicReleaseReview, PublicIntroduction, PublicAdoption, AdoptPublicListingResult } from './marketplace/public-marketplace.ts';
export { parseVerifiedPublisherResponse, parseVerifiedPublisherListResponse, parsePublicCatalogListResponse, parsePublicListingResponse, parsePublicSubmissionResponse, parsePublicSubmissionListResponse, parsePublicReviewResponse, parseAdoptPublicListingResponse } from './marketplace/public-marketplace.ts';
```

- [ ] **Step 4: 运行契约测试与共享类型检查（GREEN）**

Run: `npx tsx --test packages/contracts/src/marketplace/public-marketplace.test.ts packages/contracts/src/marketplace/agent-adoption.test.ts`
Expected: PASS（fail 0）。
Run: `npm run test:shared --silent`
Expected: **不要把 test:shared 当作本任务新文件的回归证据**——其 glob（`package.json:13`）只含 `packages/contracts/test/*.test.ts` 与 `test/mobile-*.test.ts`，不含 `packages/contracts/src/marketplace/*.test.ts`，因此既不运行本计划新增的 `public-marketplace.test.ts`，也不运行既有 `agent-adoption.test.ts`；这两个 marketplace 契约文件的回归证据就是上面的 `npx tsx --test` 直跑。test:shared 在此仅作共享包整体健康旁证；若因其它并行批次文件缺失而失败，如实记录并以本计划两个 marketplace 测试文件 + `npm run typecheck:shared --silent`（strict tsc 含 `packages/contracts/src/index.ts`，会真实检查新导出的类型与解析器）作为本任务证据即可。

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src/marketplace/public-marketplace.ts packages/contracts/src/marketplace/public-marketplace.test.ts packages/contracts/src/index.ts
git commit -m "feat(contracts): public marketplace wire contracts and parsers (T30 #60 task 9)"
```

---

## 计划级验证命令

在 worktree 根（`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep`）执行：

```bash
go build ./internal/... && go test ./internal/database/ -count=1 && go test ./internal/application/repository/ -run 'TestAgentAdoption|TestReplaceCapabilityMappings|TestPublicMarketplace' -count=1 && go test ./internal/application/service/ -run 'TestPublicMarketplace|TestAgentAdoption' -count=1 && go test ./internal/router/ -run 'TestPublicMarketplace|TestTenantAgentMarketplace|TestTenantAgentAdoption|TestTenantAgentVariant|TestAvailableAgentsAPIKeyFloor' -count=1 && npx tsx --test packages/contracts/src/marketplace/public-marketplace.test.ts packages/contracts/src/marketplace/agent-adoption.test.ts
```

覆盖说明：`internal/database` 全包（迁移对齐 + up/down/up 语义回放）；repository/service 以 `-run` 定向本计划与 #59 受影响测试；router 以 `-run` 定向本计划新测试 + #58/#59 回归；contracts 运行新契约文件 + #59 契约回归。全量 `go test ./...` 不在本计划门内（既有套件体量大且与本计划无关）。

## 并行批次冲突最小化说明（本计划独有文件 vs 共享文件）

- 本计划独有新增：Task 2/4/5/6/7/9 的全部新文件、四个新迁移、四个改名迁移。
- 共享文件修改点（合并时注意）：`internal/router/router.go`（仅 2 行：params 字段 + 注册调用）、`internal/container/container.go`（仅 3 个 Provide）、`internal/application/repository/agent_adoption.go`（AdoptListing 重构 + 两个 getter 回退，方法签名不变）、`internal/database/migration_sqlite_versioned_schema_test.go`（表清单追加 6 行）、`packages/contracts/src/index.ts`（2 行导出）。
- 迁移版本号占用：本计划占用 versioned 000193 / sqlite 000114。同批次若有其它计划也新增 Go 迁移，集成时以「下一个空闲号」机械重排解决（纯改名），不得再次产生重复版本号（Task 1 的教训）。
