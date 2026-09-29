# T21：多操作 Action Plan 与部分成功恢复（Issue #51）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 一个 Task 组合多个有序外部副作用为一份 Action Plan：Owner 可整体批准或排除单项（排除单项），计划内容变化使旧批准失效（AC1），部分成功只恢复确认未完成的动作、绝不重复已成功的动作（AC2），端到端行为在生产迁移库 + 真实 HTTP 处理器链路上以最高稳定 Interface 验证（AC3）。

**Architecture:** #48 已交付冻结的单操作 A03 审批管线（`internal/modules/appconnector/service/appconnector/action.go:160` `ActionService`，store 为权威）与 Notion 发布 seam（`internal/modules/appconnector/publish/plan.go:143` `NotionPublishService.FormPlan`：Artifact 版本→段落块→快照→Prepare→planned 回执行）。本计划在其上加一层**计划层深模块**：①`repository/appconnector` 新增 `app_action_plans` + `app_action_plan_items` 两表与 `PlanStore`（不动冻结的 `ActionRow`——与 #48 同判例）；②新包 `internal/modules/appconnector/plan`：`PlanDigest`（结构化 JSON sha256，绑定租户/actor/有序项的 action id+digest+连接+目标+风险——集合、顺序、内容任一变化即新 digest，AC1 的纯函数锚）、`Service.FormPlan`（逐项复用 #48 单操作 formation，计划 digest 绑定权威行动行）、`Service.Approve`（整体批准 + 排除单项；排除项永不批准永不派发）、`Service.Execute`（顺序执行；succeeded→跳过、authorized→执行（恢复路径）、awaiting_approval→跳过（fail closed）、failed/unknown→settled（确认终态绝不重派））、`Service.Status`（逐项权威状态 + 回执投影）；③HTTP 面 `POST/GET /api/v1/apps/action-plans[/:id(/approve|/execute)]` + 路由 + 容器接线（扩展现有 `newNotionPublishHandler` dig 构造器为双输出，同一 `ActionService`/bridge 实例，冻结面零改动）；④Task 0 先修复波级迁移同号双占（sqlite 000114 / versioned 000193），否则 AC3 的全量迁移 e2e 在当前 HEAD 不可运行。

**Tech Stack:** Go 1.26（gin + gorm + golang-migrate + testify + httptest），单模块 `github.com/Tencent/WeKnora`，全部命令在 worktree 根 `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep` 执行。本计划作者已在本环境实跑基线（2026-09-26）：`go test ./internal/modules/appconnector/... -count=1` 6 个包全部 ok；`go test ./internal/handler/ -run TestNotionPublish -count=1` **3 个 e2e 全 FAIL**，报错 `failed to open source, "file://...migrations/sqlite": duplicate migration file: 000114_public_agent_marketplace.down.sql`——波级问题 1 属实（sqlite 000114 被 `mobile_device_app`(#67) 与 `public_agent_marketplace`(#60) 双占、versioned 000193 同样双占），Task 0 修复。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-51.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（Implementation Decisions——尤其「Action Plans and candidate code commits are immutable approval anchors. Changes invalidate prior approvals.」与「Go integration tests verify authorization, Tenant/Owner/Collaborator predicates, revision and digest CAS, idempotency, **partial external success**, unknown outcomes and durable checkpoints.」；Testing Decisions「Tests target observable behavior at the highest stable Interface.」）；`docs/specs/2026-09-20-mobile-module-seams.md`（按需）
- 领域术语：`CONTEXT.md`——「**操作计划（Action Plan）**：提交审批的一组确定外部操作，包含每项操作的连接、目标、内容摘要、顺序与版本；成员可整体批准或排除单项，执行结果仍逐项持久记录。任何目标、内容、连接或操作集合变化都会使既有批准失效。」（避免：批准整个运行中的未知未来操作、一次批准后的永久自动执行）、「外部发布（External Publication）」、「任务协作者」（不授予批准外部副作用权限）
- ADR（按需）：`docs/adr/0004-task-is-session.md`（Task=Session）、`docs/adr/0009-cloud-data-trust-boundary.md`（凭据边界）
- Parent：Issue #30；Blocked by：#48（B4 已交付——本计划作者在当前 HEAD 亲眼核实：`internal/modules/appconnector/publish/` 四文件、`app_publications` 迁移 sqlite 000115 / versioned 000194、`internal/handler/app_connector_notion_publish.go`、`internal/container/notion_publish.go` 均已集成）
- 下游（本计划 Produces 供其消费）：#71（被本 Issue 阻塞的下游）消费计划层 wire 契约与 `plan.Service`；#49/#50 飞书/Confluence 接入时逐项复用同一条计划 seam（Provider 列已预留）

## Consumes（前四批已集成的接口，作者均在当前 HEAD 亲眼核实）

- `appconnectorsvc.NewActionService(store, guard, gate, dispatcher, unknown) *ActionService`（`service/appconnector/action.go:192`）；其 `Approve(ctx, id, actor, digest) error`（`:284`，digest 不匹配→`ErrActionDigestMismatch`）、`Execute(ctx, id) error`（`:318`）、`ErrActionState`/`ErrNoDispatcher`；`ActionStoreSource.FindAction(ctx, id) (repoappconn.ActionRow, error)`（`:142`）
- `publish.NewNotionPublishService(actions, store, pubs, artifacts, content, remote, scopes) *NotionPublishService`（`publish/plan.go:120`）；`FormPlan(ctx, PublishPlanInput) (PublishPlanView, error)`（`:143`，逐项：scope 校验→destination 权威规则→artifact 读取→Notion 段落块→**外部版本预读**→A03 Prepare→planned 回执行）；`Execute(ctx, tenantID, actionID) (PublishExecuteOutcome, error)`（`:252`）；`Receipt(ctx, tenantID, actionID) (PublishReceiptView, error)`（`:271`）；`PublishExecuteOutcome{ActionState, Conflict, Receipt}`（`:98`，Conflict=ProviderResult 前缀 `PublishVersionConflictResult`）；哨兵 `ErrPublishInvalidInput/ArtifactNotReady/UnsupportedArtifact/ContentTooLarge/EmptyContent/DestinationOutOfScope/DestinationUnreadable/UpdateTargetNotPublished`（`plan.go:18` + `blocks.go:28`）
- `repoappconn.NewActionStore(db)/*ActionStore`（`repository/appconnector/action.go:93`：`CreateAction/FindAction/SetActionState/ClaimDispatch/FinishDispatch/FinishUnknown`）；`ActionRow`（`:31`，字段 `ID/TenantID/ActorID/ConnectionID/AppVersion/Target/Risk/AuthVersion/ArgsSnapshot/ArgsDigest/State/Fence/...`）；`NewPublicationStore(db)/*PublicationStore`（`publication.go:66`）；`ErrActionNotFound`
- `appconn`（root 包）：`Action{ID,TenantID,ActorID,ConnectionID,Version,Target,Risk,AuthVersion,Args,...}`、状态常量 `ActionAwaitingApproval/Authorized/Queued/Dispatched/Succeeded/Failed/Unknown`（`action.go:37-45`）、`RiskWrite`、`ActionDigest`（`:141`，结构化 JSON 绑定含 `a.ID`——两个不同 action 永不共享 digest）、`CanDriveActionWrites(role) bool`
- HTTP 辅助：`appTenantScope(c) (tenantID uint64, role string, userID string, ok bool)`（`internal/handler/app_connector.go:46`）、`appOK/appFail`（`:28/:32`）、`appRequireWriteCapability`（`:60`）；既有审批端点 `POST /api/v1/apps/actions/:id/approve`（`app_connector_action.go:211`，本计划不动）
- 容器/路由：`internal/container/notion_publish.go:28` `newNotionPublishHandler`（dig Provide，`container.go:1013`）；`internal/router/router.go:145` `RouterParams.AppNotionPublishHandler` 与 `:436` 注册点
- e2e 夹具（同包复用）：`openNotionPublishE2EDB`/`newE2ENotion`/`e2ePolicyProvider`/`e2ePassGuard`/`e2eLocalContent`（`internal/handler/app_connector_notion_publish_e2e_test.go`）
- 迁移轨道现状：sqlite 最新 `000117_code_deliveries`、versioned 最新 `000196_code_deliveries`（重编后本计划新表落位 sqlite 000119 / versioned 000198；编号为执行时刻变量，强制核查与顺延条款见 Task 0 Step 0）

## Global Constraints

以下为批准 Spec / CONTEXT.md / Issue 的项目级约束，逐字引用，所有任务隐含遵守：

- 「计划内容变化使旧批准失效。」（Issue #51 验收标准 1 原文）
- 「部分成功只恢复确认未完成的动作。」（Issue #51 验收标准 2 原文）
- 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」（Issue #51 验收标准 3 原文）
- CONTEXT.md「操作计划（Action Plan）」全文：「提交审批的一组确定外部操作，包含每项操作的连接、目标、内容摘要、顺序与版本；成员可整体批准或排除单项，执行结果仍逐项持久记录。任何目标、内容、连接或操作集合变化都会使既有批准失效。」与「_避免_：批准整个运行中的未知未来操作、一次批准后的永久自动执行。」——计划 digest 在批准与执行两个时刻都核对；不存在"批准后自动追加项"。
- mobile-ai-office-design.md：「Lead Agent Version, Artifact versions, Action Plans and candidate code commits are immutable approval anchors. Changes invalidate prior approvals.」
- mobile-ai-office-design.md：「Go integration tests verify authorization, Tenant/Owner/Collaborator predicates, revision and digest CAS, idempotency, partial external success, unknown outcomes and durable checkpoints.」「Tests target observable behavior at the highest stable Interface.」
- CONTEXT.md「任务协作者」：「不会授予使用任务所有者个人连接或批准其外部副作用的权限」——计划审批谓词 = 计划发起者或租户 owner/admin（发起者必然拥有计划用到的全部个人连接：#48 formation 的连接检查已强制，`internal/handler/app_connector_notion_publish.go:77`）。
- #48 冻结判例延续：`ActionRow`/`ActionService` 生命周期/`NotionBridge`/既有 `/apps/actions/*` 与 `/apps/notion-publish/*` 端点零行为改动；回执永远是行动行的投影（`publish/plan.go:287` `project` 语义），计划层的逐项结果同样是行动行的投影。
- 安全约束（Mimosa，与本需求相关者视为验收条件）：服务端出站请求仅 http/https 且发请求前校验 host、拒绝 localhost/环回/私网/保留地址——既有 `HTTPPolicy` + `AuthorizedNetworks` 通道承担，测试契约双打走 127.0.0/8 documented test hook；数据库查询全部参数绑定（本计划新查询一律 gorm `Where("col = ?", v)` 绑定，无字符串拼接 SQL）；凭据只从环境变量读取（本计划零新增凭据；测试 token `secret_test_token` 为契约双打专用假值，不是可用凭据）。
- 工作流约束：严格 RED→GREEN→REFACTOR（先写失败测试、实跑确认失败、最小实现、通过、提交）；skip 不是 pass——凡 blocked-env 证据必须显式标注，不得伪造。

**Issue #51 验收标准原文（docs/plans/issue30-sweep/issues/issue-51.md）：**

1. 「计划内容变化使旧批准失效。」
2. 「部分成功只恢复确认未完成的动作。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

**验收标准 3 的本地可验证性说明（blocked-env 声明）：** 真实 Notion API 验收需要 `NOTION_TOKEN`/`NOTION_PARENT_PAGE_ID`，本环境不存在——#48 的真实 Provider 循环 `TestNotionRealPublishLoop`（`notion_publish_real_test.go:20`，NOTION_TOKEN 门控）在单操作层继续承担真实凭据证据且本地如实 SKIP；本计划**不新增伪造的真实凭据测试**。本地最高稳定 Interface 替代证据 = **Task 6 端到端集成测试**：生产 sqlite 迁移库（golang-migrate 全量 `migrations/sqlite`）+ 真实 `ActionService`/`PlanStore`/`NotionPublishService`/`NotionBridge`/gin 处理器/既有审批端点 + 本地契约双打 Notion HTTP 服务（httptest 实现官方页面/块契约形状）——只有 Notion 网络端点被替换，其余全真；双打在测试注释中明示「NOT the real-provider acceptance」。多操作计划的真实 Provider 验收项列为 **blocked-env**：有凭据的环境应将计划端点对真实 Notion 父页面跑通后再关闭本 Issue 的 AC3。

**与调查结论的差异记录（以代码现状为准）：**

1. 调查称「前置依赖 #48 为 partial，多操作计划被其阻塞」——issue 状态滞后于批次事实：B4 已交付并集成（`docs/plans/issue30-sweep/FINAL-REPORT.md`；本计划作者实核 `internal/modules/appconnector/publish/plan.go:143` FormPlan、`repository/appconnector/publication.go:36` PublicationRow、`internal/container/notion_publish.go:28` 装配、#48 三个 HTTP e2e 均在），本计划直接 Consumes。`publish/blocks.go:8` 甚至预写了「#51 extends the single-action plan record」。
2. 调查称「ActionRow 是单操作行，无计划分组/顺序/排除单项/逐项聚合结构」属实——本计划不扩展冻结的 `ActionRow`（OC 派发/恢复链共用投影，与 #48 差异记录第 2 条同判例），计划分组落在新表 `app_action_plans`/`app_action_plan_items`；逐项结果不另建聚合列——行动行 + 回执行就是「逐项持久记录」的权威，计划层只做投影（`Service.Status`）。
3. **波级迁移双占现状与调查指引一致但占用对已变**：当前 HEAD 双占对是 `mobile_device_app`(#67) × `public_agent_marketplace`(#60)（plan-t43/t60 当年声明的 114/193 槽位已被后续落库的 app_publications 115/194、task_compliance 116/195、code_deliveries 117/196 用掉）。重编对象选 **public_agent_marketplace**：作者实核全仓库 Go 引用——它仅被 `internal/database/migration.go` 两处引用（`:33` 常量 `sqliteAdoptionFKRelaxationMigrationVersion = 114` 与 `:123` `os.Stat` 探测串），而 `mobile_device_app` 被 6 个测试文件的路径字符串引用（`internal/handler/mobile_device_test.go:33`、`internal/application/repository/mobile_device_test.go:31`、`mobile_push_isolation_test.go:67`、`mobile_device_app_test.go:37/:188-189`、`internal/modules/workbench/service/workbench/notification_app_policy_test.go:50`）。挪 marketplace 后 NoTxWrap 门控常量改 118，该文件（PRAGMA foreign_keys 表重建）移动到 117 之后运行——其 DDL 只触碰 `agent_adoptions`/`agent_adoption_variants`（000112/113 建）与自有五表，与 115-117 的 app_publications/task_compliance/code_deliveries 无表级交集，顺序交换安全。自引用修正：sqlite up 文件第 1 行注释「SQLite twin of versioned migration 000193.」→ 000197。
4. 调查称「Owner 整体批准或排除单项的交互不存在」属实：全仓库无 `ActionPlan` 域类型（作者 rg 复核 internal/ 与 packages/ 零命中，唯一的 plan 命名是 `publish` 包的单操作 FormPlan 与 `internal/modules/codedelivery` 的代码交付 plan——后者是 #52 的交付计划，与本 Issue 的多操作计划无关，不合并）。本计划补齐服务端全链；移动端 UI 消费（Task Office 面板呈现计划批准/排除）不在本计划范围——移动侧既有 app-connector 动作面均由 Go wire 先行、TS 后续接入（#48 同模式，TS 侧当前零消费面，作者已核实 packages/mobile-core、packages/api-client、apps/mobile 对 notion-publish 零引用）。

**执行语义裁定（写计划时的设计决策，实施者不得擅改）：**

- **有序 + 逐项独立**：计划项按 seq 升序顺序执行；一项失败/未知**不阻断**后续项（各项是独立外部副作用，CONTEXT.md 明文「执行结果仍逐项持久记录」）；"有序"保证 N+1 在 N 终态之后才开始。
- **AC2 的"确认未完成"**：唯一会被（重新）派发的状态是 `authorized`（已批准未派发——崩溃恢复窗口）。`succeeded`→skipped_succeeded（永不重发）；`failed`/`unknown`→settled（已是确认终态，重新派发需要新计划——重 Prepared 内容已变，AC1 使旧批准失效）；`awaiting_approval`→skipped_unapproved（fail closed）；`queued`/`dispatched`→skipped_in_flight（活跃写者持有；单项批准计数 + 状态 CAS 是既有护栏）。
- **排除单项是批准时决定**：记录在计划行 `excluded_json`，被排除项永不批准、永不派发；批准后再改排除集是状态冲突（`ErrPlanState`）。同 digest 幂等重批准是恢复路径（为 partial-approve 窗口补批剩余项）。
- **AC1 在两个时刻生效**：批准时 digest 不匹配→拒绝（镜像 `service/appconnector/action_test.go:194-197` 的单操作锚）；执行时 digest 不匹配→拒绝。计划行 digest 不可变——"计划内容变化" = 形成新计划行、新 digest；旧计划的批准永远只绑定旧内容（冻结快照纪律，与单操作「Modified args later simply mean a NEW Prepare」一致，`service/appconnector/action.go:213-214`）。

## Review Focus

Spec/领域定义隐含但无任务测试覆盖、最可能咬到真实用户的五类输入/失效模式（每行后在所属任务落地测试）：

1. **部分成功后盲目重跑全部（重复外发）**：用户第一次执行到一半（1 成功 1 失败 1 未知），恢复时天真地全部重跑会把已成功的副作用再发一次。合理行为：同 digest 重入只派发 `authorized` 项，已确认 outcome 零重发。——Task 4 `TestPlanExecuteSkipsConfirmedOutcomesOnResume`（dispatch 计数断言）+ Task 6 `TestActionPlanEndToEndPartialSuccessResumesUnfinishedOnly`（fake Notion 写计数断言）。
2. **计划内容变化后旧批准复用**：内容变化形成新计划后，拿着旧 digest 批准/执行新计划。合理行为：批准面与执行面都以 409 `ACTION_PLAN_DIGEST_MISMATCH` 拒绝且零写入。——Task 3 `TestPlanApproveRejectsForeignDigest` + Task 4 `TestPlanExecuteRefusesUnapprovedOrForeignDigest` + Task 6 `TestActionPlanEndToEndContentChangeInvalidatesOldApproval`。
3. **排除单项后仍被派发 / 排除集被事后改写**：Owner 排除了第 2 项，执行却把第 2 项也发了；或批准后重批时偷换排除集。合理行为：排除项永久停在 `awaiting_approval`，排除集在首次批准后冻结。——Task 3 `TestPlanApproveExcludesItemNeverApprovesIt` + Task 4 `TestPlanApproveRecoveryExclusionFrozen` + Task 6 `TestActionPlanEndToEndExcludeItem`。
4. **未批准项被计划级执行派发**：计划已 authorized 但某项的单项批准缺失（partial-approve 窗口）。合理行为：该项 skipped_unapproved，零派发；未批准计划整体拒绝执行。——Task 4 `TestPlanExecuteSkipsUnapprovedItemFailClosed`/`TestPlanExecuteRefusesUnapprovedOrForeignDigest`。
5. **跨租户 plan id 侦查（存在性泄漏）**：用他租户的计划 id 探测。合理行为：与本域所有端点一致——跨租户与不存在统一 404，绝不 403 泄漏存在性。——Task 1 `TestPlanCreateFindRoundTrip`（store 层 not-found）+ Task 5 `TestActionPlanHandlerValidationAndNotFound`（wire 层 404）。

---

### Task 0: 迁移轨道去重重编（波级前置修复）

当前 HEAD 实测：`go test ./internal/handler/ -run TestNotionPublish -count=1` 的 3 个 e2e 全部 FAIL，报 `duplicate migration file: 000114_public_agent_marketplace.down.sql`（golang-migrate 打开源目录即失败）。本任务把 `public_agent_marketplace` 两个轨道的四个文件重编号到下一可用号（sqlite 000114→**000118**、versioned 000193→**000197**），并同步 `internal/database/migration.go` 的两处引用。重编后 118/197 被 marketplace 占用，**本计划新表迁移落位 sqlite 000119 / versioned 000198**（Task 1）。

**编号是执行时刻变量，不是常量**：同批并行计划在本 worktree 落盘各自的新迁移（作者修复轮实跑时曾观察到瞬时存在后消失的未跟踪 `000118_mobile_device_app`/`000119_task_research`/`000197_mobile_device_app`）。因此 Step 0 的占用核查是本任务的**强制第一步**：若下文的 118/197（及 Task 1 的 119/198）在执行时刻已被占用，整组顺延为当时的下一空闲号（两个任务两轮占用保持先后顺序），并同步下方「联动点」清单中的每一处；DDL、测试断言与测试逻辑零变化。

**Files:**
- Modify（重命名）: `migrations/sqlite/000114_public_agent_marketplace.up.sql` → `migrations/sqlite/000118_public_agent_marketplace.up.sql`（若 118 已被占用则顺延为执行时刻空闲号，下同）
- Modify（重命名）: `migrations/sqlite/000114_public_agent_marketplace.down.sql` → `migrations/sqlite/000118_public_agent_marketplace.down.sql`
- Modify（重命名）: `migrations/versioned/000193_public_agent_marketplace.up.sql` → `migrations/versioned/000197_public_agent_marketplace.up.sql`
- Modify（重命名）: `migrations/versioned/000193_public_agent_marketplace.down.sql` → `migrations/versioned/000197_public_agent_marketplace.down.sql`
- Modify: `internal/database/migration.go:29-33`（注释 + 常量）与 `:123`（os.Stat 探测串）
- Modify: `migrations/sqlite/000118_public_agent_marketplace.up.sql:1`（自引用版本注释）

**联动点（顺延时必须逐一同步，缺一即装载失败或门控错位）**：①本任务 Files 的四个 git mv 目标号；②`internal/database/migration.go:33` 常量 `sqliteAdoptionFKRelaxationMigrationVersion`；③`internal/database/migration.go:123` os.Stat 探测串；④sqlite up 文件第 1 行自引用注释（指向 versioned 伴生号）；⑤Task 1 四个迁移文件号及其 commit 路径；⑥本段与 Task 1/交付边界中出现的编号文字。

**Interfaces:**
- Consumes: 无
- Produces: 可装载的全量 sqlite 迁移轨道（#48 三个 e2e 恢复可运行）；`sqliteAdoptionFKRelaxationMigrationVersion = 118`（NoTxWrap 三段式门控锚，顺延时同号联动）

- [ ] **Step 0: 执行时刻占用核查（强制，先于一切改动）**

```bash
git status --short migrations/
ls migrations/sqlite/ | sort | tail -6
ls migrations/versioned/ | sort | tail -6
```

判定规则：以两轨道当前最大号为准——本文写作时（作者修复轮实跑 2026-09-26）为 sqlite `000117_code_deliveries`、versioned `000196_code_deliveries`，故 118/197（marketplace）与 119/198（Task 1 新表）空闲可用。若 Step 0 输出显示这些号已被任何文件（含未跟踪落盘文件）占用，把 marketplace 顺延为「当前最大号+1」、Task 1 新表顺延为其后再 +1，并按上方「联动点」清单同步全部六处，然后才执行 Step 1 以下步骤。

- [ ] **Step 1: 实跑确认预存在损坏（RED 基线）**

- [ ] **Step 1: 实跑确认预存在损坏（RED 基线）**

Run: `go test ./internal/handler/ -run TestNotionPublish -count=1 2>&1 | grep -E "duplicate migration|FAIL|ok " | head -8`
Expected: 3 个 `--- FAIL`，含 `duplicate migration file: 000114_public_agent_marketplace.down.sql`（作者已实测，此输出是重编依据）

- [ ] **Step 2: git mv 四个迁移文件**

```bash
git mv migrations/sqlite/000114_public_agent_marketplace.up.sql migrations/sqlite/000118_public_agent_marketplace.up.sql
git mv migrations/sqlite/000114_public_agent_marketplace.down.sql migrations/sqlite/000118_public_agent_marketplace.down.sql
git mv migrations/versioned/000193_public_agent_marketplace.up.sql migrations/versioned/000197_public_agent_marketplace.up.sql
git mv migrations/versioned/000193_public_agent_marketplace.down.sql migrations/versioned/000197_public_agent_marketplace.down.sql
```

- [ ] **Step 3: 修正 sqlite up 文件的自引用注释**

`migrations/sqlite/000118_public_agent_marketplace.up.sql` 第 1 行：

```sql
-- SQLite twin of versioned migration 000197.
```

（原文为 `-- SQLite twin of versioned migration 000193.`；文件其余内容零改动。）

- [ ] **Step 4: 更新 internal/database/migration.go 三处**

`internal/database/migration.go:29-33` 原文：

```go
// sqliteAdoptionFKRelaxationMigrationVersion is the sqlite twin of the
// public-marketplace adoption FK relaxation (000114). Like the workbench
// rebuild (v55) that file owns its transaction and PRAGMAs, so it must run
// with the driver's NoTxWrap while every other file keeps per-file wrapping.
const sqliteAdoptionFKRelaxationMigrationVersion = 114
```

改为：

```go
// sqliteAdoptionFKRelaxationMigrationVersion is the sqlite twin of the
// public-marketplace adoption FK relaxation (000118; renumbered from
// 000114 in T21 #51 Task 0 to de-duplicate the mobile_device_app
// collision). Like the workbench rebuild (v55) that file owns its
// transaction and PRAGMAs, so it must run with the driver's NoTxWrap
// while every other file keeps per-file wrapping.
const sqliteAdoptionFKRelaxationMigrationVersion = 118
```

`internal/database/migration.go:123` 原文：

```go
		_, err = os.Stat("migrations/sqlite/000114_public_agent_marketplace.up.sql")
```

改为：

```go
		_, err = os.Stat("migrations/sqlite/000118_public_agent_marketplace.up.sql")
```

- [ ] **Step 5: 实跑验证修复（GREEN）**

Run: `go build ./... && go test ./internal/handler/ -run 'TestNotionPublish|TestAppPublicationsTableExists' -count=1 && go test ./internal/database/ -count=1`
Expected: 全部 PASS（#48 三个 e2e + publications 迁移对齐 + database 包全绿）

- [ ] **Step 6: Commit**

```bash
git add migrations/ internal/database/migration.go
git commit -m "fix(migrations): 去重 sqlite 000114/versioned 000193 双占——public_agent_marketplace 重编 000118/000197（T21 #51 Task 0）"
```

---

### Task 1: PlanStore 与 app_action_plans 迁移

**Files:**
- Create: `migrations/sqlite/000119_app_action_plans.up.sql`
- Create: `migrations/sqlite/000119_app_action_plans.down.sql`
- Create: `migrations/versioned/000198_app_action_plans.up.sql`
- Create: `migrations/versioned/000198_app_action_plans.down.sql`
- Create: `internal/modules/appconnector/repository/appconnector/plan.go`
- Test: `internal/modules/appconnector/repository/appconnector/plan_test.go`

**Interfaces:**
- Consumes: gorm（`Where("col = ?", v)` 参数绑定纪律）；Task 0 的可装载迁移轨道
- Produces: `repoappconn.ActionPlanRow{ID, TenantID, ActorID, Digest, State, ExcludedJSON string, ApprovedBy string, ApprovedAt *time.Time, CreatedAt, UpdatedAt time.Time}`（TableName `app_action_plans`，复合 PK tenant_id+id）；`repoappconn.ActionPlanItemRow{TenantID uint64, PlanID string, Seq int, ActionID string, CreatedAt time.Time}`（TableName `app_action_plan_items`，复合 PK tenant_id+plan_id+seq）；常量 `PlanStateAwaitingApproval = "awaiting_approval"`、`PlanStateAuthorized = "authorized"`；哨兵 `ErrPlanNotFound`/`ErrPlanState`；`NewPlanStore(db) *PlanStore` 与 `CreatePlan(ctx, plan ActionPlanRow, items []ActionPlanItemRow) error`（单事务，零项拒绝）、`FindPlan(ctx, tenantID uint64, planID string) (ActionPlanRow, error)`（跨租户统一 `ErrPlanNotFound`）、`ListPlanItems(ctx, tenantID uint64, planID string) ([]ActionPlanItemRow, error)`（seq 升序）、`ApprovePlan(ctx, tenantID uint64, planID, digest, actor, excludedJSON string, now time.Time) error`（CAS：digest + 状态∈{awaiting,authorized}，0 行→`ErrPlanState`）

- [ ] **Step 1: 写迁移文件（四件）**

`migrations/sqlite/000119_app_action_plans.up.sql`：

```sql
-- Multi-action Action Plans (T21, #51) — sqlite track. Same shape as the
-- versioned migration. A plan binds an ORDERED set of already-prepared
-- actions under ONE plan digest (CONTEXT.md 操作计划): any content,
-- connection, target or set change forms a NEW plan with a NEW digest, so
-- an old approval can never authorize new content. Exclusions are an
-- approval-time decision recorded on the plan row; per-item results stay
-- on the authoritative app_actions rows and the plan only projects them.
CREATE TABLE app_action_plans (
    tenant_id INTEGER NOT NULL,
    id TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    digest TEXT NOT NULL,
    state TEXT NOT NULL,
    excluded_json TEXT NOT NULL DEFAULT '',
    approved_by TEXT NOT NULL DEFAULT '',
    approved_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, id)
);

CREATE INDEX idx_app_action_plans_digest ON app_action_plans (tenant_id, digest);

CREATE TABLE app_action_plan_items (
    tenant_id INTEGER NOT NULL,
    plan_id TEXT NOT NULL,
    seq INTEGER NOT NULL,
    action_id TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, plan_id, seq)
);

CREATE INDEX idx_app_action_plan_items_action ON app_action_plan_items (tenant_id, action_id);
```

`migrations/sqlite/000119_app_action_plans.down.sql`：

```sql
DROP INDEX IF EXISTS idx_app_action_plan_items_action;
DROP TABLE IF EXISTS app_action_plan_items;
DROP INDEX IF EXISTS idx_app_action_plans_digest;
DROP TABLE IF EXISTS app_action_plans;
```

`migrations/versioned/000198_app_action_plans.up.sql`：

```sql
-- Multi-action Action Plans (T21, #51): an ORDERED set of already-prepared
-- external actions approved as ONE decision under ONE plan digest
-- (CONTEXT.md 操作计划). Any content, connection, target or set change
-- forms a NEW plan with a NEW digest, so an old approval can never
-- authorize new content; exclusions are an approval-time decision
-- recorded on the plan row; per-item results stay on the authoritative
-- app_actions rows and the plan only projects them.
CREATE TABLE app_action_plans (
    tenant_id BIGINT NOT NULL,
    id VARCHAR(64) NOT NULL,
    actor_id VARCHAR(255) NOT NULL,
    digest VARCHAR(64) NOT NULL,
    state VARCHAR(16) NOT NULL,
    excluded_json TEXT NOT NULL DEFAULT '',
    approved_by VARCHAR(255) NOT NULL DEFAULT '',
    approved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, id)
);

CREATE INDEX idx_app_action_plans_digest ON app_action_plans (tenant_id, digest);

CREATE TABLE app_action_plan_items (
    tenant_id BIGINT NOT NULL,
    plan_id VARCHAR(64) NOT NULL,
    seq INTEGER NOT NULL,
    action_id VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, plan_id, seq)
);

CREATE INDEX idx_app_action_plan_items_action ON app_action_plan_items (tenant_id, action_id);
```

`migrations/versioned/000198_app_action_plans.down.sql`：

```sql
DROP INDEX IF EXISTS idx_app_action_plan_items_action;
DROP TABLE IF EXISTS app_action_plan_items;
DROP INDEX IF EXISTS idx_app_action_plans_digest;
DROP TABLE IF EXISTS app_action_plans;
```

（编号执行时刻核查：开工前重跑 Task 0 Step 0 的三条命令。若 000119/000198 已被同批兄弟计划先占——含未跟踪落盘文件——按四方一致约定整体顺延为当时的下一空闲编号（须排在 Task 0 的 marketplace 新号之后）；顺延时同步四个迁移文件名、本任务 commit 的 git add 路径与 DDL 注释文字，DDL 本体与测试断言零变化。迁移↔投影对齐测试（Task 6）只断言表/列名，不引用编号，无需改动。）

- [ ] **Step 2: 写失败测试**

`internal/modules/appconnector/repository/appconnector/plan_test.go`：

```go
package appconnector

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openPlanStoreDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&ActionPlanRow{}, &ActionPlanItemRow{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func planRow(id string) ActionPlanRow {
	return ActionPlanRow{
		ID: id, TenantID: 7, ActorID: "user-a",
		Digest: "digest-" + id, State: PlanStateAwaitingApproval,
	}
}

func planItems(tenant uint64, planID string, actionIDs ...string) []ActionPlanItemRow {
	items := make([]ActionPlanItemRow, 0, len(actionIDs))
	for i, a := range actionIDs {
		items = append(items, ActionPlanItemRow{TenantID: tenant, PlanID: planID, Seq: i + 1, ActionID: a})
	}
	return items
}

// TestPlanCreateFindRoundTrip: the plan + ordered items round-trip; a
// zero-item plan is structurally impossible; a cross-tenant lookup is
// indistinguishable from a missing one (existence never leaks).
func TestPlanCreateFindRoundTrip(t *testing.T) {
	db := openPlanStoreDB(t)
	store := NewPlanStore(db)
	ctx := context.Background()
	if err := store.CreatePlan(ctx, planRow("plan-1"), planItems(7, "plan-1", "act-1", "act-2")); err != nil {
		t.Fatal(err)
	}
	got, err := store.FindPlan(ctx, 7, "plan-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Digest != "digest-plan-1" || got.State != PlanStateAwaitingApproval || got.ActorID != "user-a" {
		t.Fatalf("round trip drift: %+v", got)
	}
	items, err := store.ListPlanItems(ctx, 7, "plan-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Seq != 1 || items[0].ActionID != "act-1" || items[1].Seq != 2 || items[1].ActionID != "act-2" {
		t.Fatalf("items must come back in seq order: %+v", items)
	}
	if err := store.CreatePlan(ctx, planRow("plan-2"), nil); !errors.Is(err, ErrPlanState) {
		t.Fatalf("zero-item plan must be refused, got %v", err)
	}
	if _, err := store.FindPlan(ctx, 8, "plan-1"); !errors.Is(err, ErrPlanNotFound) {
		t.Fatalf("cross-tenant lookup must be not-found, got %v", err)
	}
	if _, err := store.FindPlan(ctx, 7, "plan-x"); !errors.Is(err, ErrPlanNotFound) {
		t.Fatalf("missing plan must be not-found, got %v", err)
	}
}

// TestPlanApproveBindsDigestAndExclusions: the approval CAS binds the
// plan digest AND the state — a foreign digest moves nothing; the
// matching CAS records state/exclusions/approver/time and stays
// idempotent for the recovery re-approval path.
func TestPlanApproveBindsDigestAndExclusions(t *testing.T) {
	db := openPlanStoreDB(t)
	store := NewPlanStore(db)
	ctx := context.Background()
	if err := store.CreatePlan(ctx, planRow("plan-1"), planItems(7, "plan-1", "act-1", "act-2", "act-3")); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := store.ApprovePlan(ctx, 7, "plan-1", "digest-other", "boss", `[]`, now); !errors.Is(err, ErrPlanState) {
		t.Fatalf("approve with foreign digest must be refused, got %v", err)
	}
	got, _ := store.FindPlan(ctx, 7, "plan-1")
	if got.State != PlanStateAwaitingApproval {
		t.Fatalf("refused approve must not move state: %s", got.State)
	}
	if err := store.ApprovePlan(ctx, 7, "plan-1", "digest-plan-1", "boss", `[2]`, now); err != nil {
		t.Fatal(err)
	}
	got, _ = store.FindPlan(ctx, 7, "plan-1")
	if got.State != PlanStateAuthorized || got.ExcludedJSON != `[2]` || got.ApprovedBy != "boss" || got.ApprovedAt == nil {
		t.Fatalf("approval must record state/exclusions/approver/time: %+v", got)
	}
	// Re-approval (recovery path) stays legal from authorized.
	if err := store.ApprovePlan(ctx, 7, "plan-1", "digest-plan-1", "boss", `[2]`, now); err != nil {
		t.Fatalf("idempotent re-approve must stay legal, got %v", err)
	}
}
```

- [ ] **Step 3: 运行确认失败**

Run: `go test ./internal/modules/appconnector/repository/appconnector/ -run TestPlan -count=1`
Expected: 编译失败（`undefined: ActionPlanRow` 等——plan.go 尚不存在）

- [ ] **Step 4: 最小实现 PlanStore**

`internal/modules/appconnector/repository/appconnector/plan.go`：

```go
package appconnector

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

var (
	// ErrPlanNotFound: no plan row for the (tenant, id) pair — a foreign
	// tenant's plan and a missing one are indistinguishable by design.
	ErrPlanNotFound = errors.New("plan_not_found")
	// ErrPlanState: an invalid plan shape or lifecycle transition —
	// zero-item create, a CAS lost race, an approval whose digest does
	// not match the plan row's digest.
	ErrPlanState = errors.New("plan_state_conflict")
)

// Plan lifecycle states persisted on app_action_plans.state. There is no
// stored terminal plan state: completion is PROJECTED from the per-item
// action rows, which are the authority.
const (
	PlanStateAwaitingApproval = "awaiting_approval"
	PlanStateAuthorized       = "authorized"
)

// ActionPlanRow is the durable multi-action Action Plan (CONTEXT.md
// 操作计划): one approval decision binding an ORDERED set of already-
// prepared actions under ONE plan digest. ExcludedJSON carries the
// approval-time exclusions (排除单项) as a JSON int array; per-item
// results stay on the authoritative app_actions rows — this row never
// duplicates them.
type ActionPlanRow struct {
	ID       string `gorm:"primaryKey;column:id"`
	TenantID uint64 `gorm:"primaryKey;column:tenant_id"`
	ActorID  string `gorm:"column:actor_id;not null"`
	Digest   string `gorm:"column:digest;not null"`
	State    string `gorm:"column:state;not null"`
	// ExcludedJSON is the approval-time exclusion set ('' or "[]" = none);
	// it is frozen at first approval — a rewrite after execution started
	// is refused at the service layer.
	ExcludedJSON string `gorm:"column:excluded_json;not null;default:''"`
	ApprovedBy   string     `gorm:"column:approved_by;not null;default:''"`
	ApprovedAt   *time.Time `gorm:"column:approved_at"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (ActionPlanRow) TableName() string { return "app_action_plans" }

// ActionPlanItemRow is one ordered member of a plan: seq positions the
// item (1-based, contiguous); action_id references the authoritative
// app_actions row the item dispatches through.
type ActionPlanItemRow struct {
	TenantID uint64 `gorm:"primaryKey;column:tenant_id"`
	PlanID   string `gorm:"primaryKey;column:plan_id"`
	Seq      int    `gorm:"primaryKey;column:seq"`
	ActionID string `gorm:"column:action_id;not null"`
	CreatedAt time.Time
}

func (ActionPlanItemRow) TableName() string { return "app_action_plan_items" }

// PlanStore persists action plans and their ordered items.
type PlanStore struct{ db *gorm.DB }

// NewPlanStore builds a PlanStore over a gorm DB.
func NewPlanStore(db *gorm.DB) *PlanStore { return &PlanStore{db: db} }

// CreatePlan inserts the plan row and its ordered items in ONE
// transaction. A plan without items is structurally impossible — an
// approval decision over nothing is refused, never persisted.
func (s *PlanStore) CreatePlan(ctx context.Context, plan ActionPlanRow, items []ActionPlanItemRow) error {
	if plan.ID == "" || plan.TenantID == 0 || plan.ActorID == "" || plan.Digest == "" ||
		plan.State != PlanStateAwaitingApproval || len(items) == 0 {
		return ErrPlanState
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&plan).Error; err != nil {
			return err
		}
		return tx.Create(&items).Error
	})
}

// FindPlan loads a plan scoped to the tenant.
func (s *PlanStore) FindPlan(ctx context.Context, tenantID uint64, planID string) (ActionPlanRow, error) {
	var row ActionPlanRow
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, planID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ActionPlanRow{}, ErrPlanNotFound
		}
		return ActionPlanRow{}, err
	}
	return row, nil
}

// ListPlanItems returns the plan's items in seq order.
func (s *PlanStore) ListPlanItems(ctx context.Context, tenantID uint64, planID string) ([]ActionPlanItemRow, error) {
	var rows []ActionPlanItemRow
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND plan_id = ?", tenantID, planID).
		Order("seq ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ApprovePlan is the guarded approval CAS: it moves the plan to
// authorized ONLY when the presented digest equals the row's digest and
// the row is still in an approvable state. Zero rows affected (foreign
// digest or a lost race) is ErrPlanState — the service layer reads the
// row first to give a foreign digest its own mismatch sentinel.
func (s *PlanStore) ApprovePlan(ctx context.Context, tenantID uint64, planID, digest, actor, excludedJSON string, now time.Time) error {
	if planID == "" || digest == "" || actor == "" {
		return ErrPlanState
	}
	res := s.db.WithContext(ctx).Model(&ActionPlanRow{}).
		Where("tenant_id = ? AND id = ? AND digest = ? AND state IN ?", tenantID, planID, digest,
			[]string{PlanStateAwaitingApproval, PlanStateAuthorized}).
		Updates(map[string]interface{}{
			"state":         PlanStateAuthorized,
			"excluded_json": excludedJSON,
			"approved_by":   actor,
			"approved_at":   now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrPlanState
	}
	return nil
}
```

- [ ] **Step 5: 运行确认通过**

Run: `go test ./internal/modules/appconnector/repository/appconnector/ -run TestPlan -count=1`
Expected: PASS（2 个测试）

- [ ] **Step 6: Commit**

```bash
git add migrations/sqlite/000119_app_action_plans.up.sql migrations/sqlite/000119_app_action_plans.down.sql migrations/versioned/000198_app_action_plans.up.sql migrations/versioned/000198_app_action_plans.down.sql internal/modules/appconnector/repository/appconnector/plan.go internal/modules/appconnector/repository/appconnector/plan_test.go
git commit -m "feat(appconnector): app_action_plans 计划表 + PlanStore（T21 #51 Task 1）"
```

---

### Task 2: plan 包——PlanDigest 纯函数与 FormPlan

**Files:**
- Create: `internal/modules/appconnector/plan/plan.go`
- Test: `internal/modules/appconnector/plan/plan_test.go`

**Interfaces:**
- Consumes: Task 1 的 `PlanStore`/行类型；#48 的 `publish.NotionPublishService.FormPlan/PublishPlanInput/PublishPlanView`（`publish/plan.go:143/:49/:72`）、`repoappconn.ActionStoreSource.FindAction`、`repoappconn.ActionRow.ArgsDigest/ConnectionID/Target/Risk`、`github.com/google/uuid`
- Produces（Task 3-6 消费，逐字签名）:
  - `plan.PlanDigest(tenantID uint64, actorID string, items []DigestItem) (string, error)`；`plan.DigestItem{Seq int; ActionID, ActionDigest, Connection, Target, Risk string}`
  - `plan.NewService(plans *repoappconn.PlanStore, actions appconnectorsvc.ActionStoreSource, approver PlanApprover, pubs *publish.NotionPublishService) *Service`；`plan.PlanApprover` interface `{ Approve(ctx context.Context, id, actor, digest string) error }`（*appconnectorsvc.ActionService 天然满足）
  - `plan.ItemInput{ConnectionID, SessionID, ArtifactVersionID, Title, ParentPageID, PageID string}`；`plan.FormInput{TenantID uint64, ActorID string, Items []ItemInput}`
  - `(s *Service) FormPlan(ctx, in FormInput) (PlanView, error)`；`plan.PlanView{ID, State, Digest string, Items []ItemView}`；`plan.ItemView{Seq int, ActionID, Digest, Mode, Destination, ExpectedExternalVersion, Title string}`（json 标签 `id/state/digest/items` 与 `seq/action_id/digest/mode/destination/expected_external_version/title`）
  - 哨兵 `plan.ErrPlanInvalidInput`/`plan.ErrPlanDigestMismatch`/`plan.ErrPlanState`；状态常量 `plan.PlanStateAwaitingApproval/PlanStateAuthorized`

- [ ] **Step 1: 写失败测试**

`internal/modules/appconnector/plan/plan_test.go`：

```go
package plan

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/publish"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ---- scenario doubles (self-contained). The publish seam's own tests
// double the Notion WIRE; plan-level unit tests script the DISPATCH
// outcomes directly, so no Notion wire is needed here. ----

type stubArtifacts struct{}

func (stubArtifacts) ReadableArtifactVersion(ctx context.Context, tenantID uint64, sessionID, versionID string) (repository.ArtifactVersion, error) {
	return repository.ArtifactVersion{ID: "ver-1", Digest: "d1", MIME: "text/plain", Size: 26}, nil
}

type stubContent struct{}

func (stubContent) ReadArtifactContent(ctx context.Context, tenantID uint64, version repository.ArtifactVersion) ([]byte, error) {
	return []byte("第一段。\n\n第二段。"), nil
}

type stubRemote struct{ versions map[string]string }

func (f stubRemote) ReadPageVersion(ctx context.Context, connectionID, pageID string) (string, error) {
	return f.versions[pageID], nil
}

type stubScopes struct{}

func (stubScopes) NotionScope(ctx context.Context, connectionID string) (publish.NotionConnectionScope, error) {
	return publish.NotionConnectionScope{
		AppID: "notion", ConnectionKind: "personal", OwnerID: "user-a", AuthVersion: 1,
		ApprovedParents: []string{"parent-1"}, InsertCapability: true,
	}, nil
}

type stubGuard struct{}

func (stubGuard) Check(ctx context.Context, subject appconn.OCSubject, connectionID string, expectedVersion int64) error {
	return nil
}

// scriptedDispatcher answers Dispatch by snap.ID and COUNTS every call —
// AC2's zero-re-dispatch assertions count here.
type scriptedDispatcher struct {
	outcomes map[string]appconnectorsvc.DispatchOutcome
	errs     map[string]error
	calls    map[string]int
}

func newScriptedDispatcher() *scriptedDispatcher {
	return &scriptedDispatcher{
		outcomes: map[string]appconnectorsvc.DispatchOutcome{},
		errs:     map[string]error{},
		calls:    map[string]int{},
	}
}

func (d *scriptedDispatcher) Dispatch(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	d.calls[snap.ID]++
	if err := d.errs[snap.ID]; err != nil {
		return appconnectorsvc.DispatchOutcome{}, err
	}
	return d.outcomes[snap.ID], nil
}

// okReceipt is a parseable succeeded ProviderResult (NotionPageReceipt
// shape) so the publication settles to published.
func okReceipt(id string) string {
	return `{"object":"page","id":"` + id + `","last_edited_time":"2026-09-25T08:00:00.000Z"}`
}

func openPlanSvcDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repoappconn.ActionRow{}, &repoappconn.ApprovalRow{},
		&repoappconn.PreAuthorizationRow{}, &repoappconn.PublicationRow{},
		&repoappconn.ActionPlanRow{}, &repoappconn.ActionPlanItemRow{}); err != nil {
		t.Fatal(err)
	}
	return db
}

type planSvcEnv struct {
	svc      *Service
	store    *repoappconn.ActionStore
	plans    *repoappconn.PlanStore
	pubs     *repoappconn.PublicationStore
	dispatch *scriptedDispatcher
}

func newPlanSvcEnv(t *testing.T) *planSvcEnv {
	t.Helper()
	db := openPlanSvcDB(t)
	store := repoappconn.NewActionStore(db)
	plans := repoappconn.NewPlanStore(db)
	pubs := repoappconn.NewPublicationStore(db)
	dispatch := newScriptedDispatcher()
	actions := appconnectorsvc.NewActionService(store, stubGuard{}, nil, dispatch, nil)
	publishSvc := publish.NewNotionPublishService(actions, store, pubs,
		stubArtifacts{}, stubContent{},
		stubRemote{versions: map[string]string{
			"parent-1": "2026-09-24T08:00:00.000Z",
			"page-9":   "2026-09-24T09:00:00.000Z",
		}},
		stubScopes{})
	svc := NewService(plans, store, actions, publishSvc)
	return &planSvcEnv{svc: svc, store: store, plans: plans, pubs: pubs, dispatch: dispatch}
}

func item(title string) ItemInput {
	return ItemInput{ConnectionID: "conn-notion", SessionID: "sess-1",
		ArtifactVersionID: "ver-1", Title: title, ParentPageID: "parent-1"}
}

func (e *planSvcEnv) formTwo(t *testing.T) PlanView {
	t.Helper()
	pv, err := e.svc.FormPlan(context.Background(), FormInput{TenantID: 7, ActorID: "user-a",
		Items: []ItemInput{item("Doc A"), item("Doc B")}})
	if err != nil {
		t.Fatal(err)
	}
	return pv
}

// TestPlanDigestBindsSetOrderAndContent pins the AC1 anchor at the pure
// function level: the digest changes when the set, the order, any item's
// content digest, the connection or the actor changes — and only then.
func TestPlanDigestBindsSetOrderAndContent(t *testing.T) {
	base := []DigestItem{
		{Seq: 1, ActionID: "act-1", ActionDigest: "d1", Connection: "conn-notion", Target: "parent-1", Risk: "write"},
		{Seq: 2, ActionID: "act-2", ActionDigest: "d2", Connection: "conn-notion", Target: "parent-1", Risk: "write"},
	}
	want, err := PlanDigest(7, "user-a", base)
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := PlanDigest(7, "user-a", base); again != want {
		t.Fatal("same material must hash identically")
	}
	if actor, _ := PlanDigest(7, "user-b", base); actor == want {
		t.Fatal("actor identity must bind the digest")
	}
	reordered := []DigestItem{base[1], base[0]}
	if re, _ := PlanDigest(7, "user-a", reordered); re == want {
		t.Fatal("order must bind the digest（有序外部副作用）")
	}
	dropped := []DigestItem{base[0]}
	if fewer, _ := PlanDigest(7, "user-a", dropped); fewer == want {
		t.Fatal("the set must bind the digest（操作集合变化失效）")
	}
	edited := []DigestItem{base[0], base[1]}
	edited[1].ActionDigest = "d2-edited"
	if ed, _ := PlanDigest(7, "user-a", edited); ed == want {
		t.Fatal("any item's content digest must bind the plan digest（内容变化失效）")
	}
	reconn := []DigestItem{base[0], base[1]}
	reconn[0].Connection = "conn-other"
	if rc, _ := PlanDigest(7, "user-a", reconn); rc == want {
		t.Fatal("connection must bind the digest（连接变化失效）")
	}
	retarget := []DigestItem{base[0], base[1]}
	retarget[0].Target = "parent-2"
	if rt, _ := PlanDigest(7, "user-a", retarget); rt == want {
		t.Fatal("target must bind the digest（目标变化失效）")
	}
	if _, err := PlanDigest(7, "user-a", nil); !errors.Is(err, ErrPlanInvalidInput) {
		t.Fatalf("zero-item plan must be refused, got %v", err)
	}
}

// TestPlanFormBuildsOrderedDigestBoundPlan: formation runs every item
// through the single-action publish seam, records each planned
// publication row, and returns the plan digest bound to the
// AUTHORITATIVE stored rows.
func TestPlanFormBuildsOrderedDigestBoundPlan(t *testing.T) {
	e := newPlanSvcEnv(t)
	ctx := context.Background()
	pv := e.formTwo(t)
	if pv.State != PlanStateAwaitingApproval || len(pv.Items) != 2 {
		t.Fatalf("formed plan: %+v", pv)
	}
	if pv.Items[0].Seq != 1 || pv.Items[1].Seq != 2 || pv.Items[0].ActionID == pv.Items[1].ActionID {
		t.Fatalf("items must be distinct and ordered: %+v", pv.Items)
	}
	if pv.Items[0].Digest == pv.Items[1].Digest {
		t.Fatal("different titles must produce different action digests")
	}
	if pv.Items[0].Mode != "create" || pv.Items[0].Destination != "parent-1" || pv.Items[0].ExpectedExternalVersion == "" {
		t.Fatalf("per-item view must carry the #48 formation facts: %+v", pv.Items[0])
	}
	// The plan digest binds the stored rows — recompute from the store.
	items, err := e.plans.ListPlanItems(ctx, 7, pv.ID)
	if err != nil {
		t.Fatal(err)
	}
	var digests []DigestItem
	for i, it := range items {
		row, ferr := e.store.FindAction(ctx, it.ActionID)
		if ferr != nil {
			t.Fatal(ferr)
		}
		digests = append(digests, DigestItem{Seq: i + 1, ActionID: row.ID,
			ActionDigest: row.ArgsDigest, Connection: row.ConnectionID,
			Target: row.Target, Risk: row.Risk})
	}
	recomputed, err := PlanDigest(7, "user-a", digests)
	if err != nil {
		t.Fatal(err)
	}
	if recomputed != pv.Digest {
		t.Fatalf("plan digest must bind the stored rows: %s != %s", recomputed, pv.Digest)
	}
	// Every item recorded its planned publication row（逐项持久记录）.
	for _, it := range pv.Items {
		if _, err := e.pubs.FindByAction(ctx, 7, it.ActionID); err != nil {
			t.Fatalf("item %s must have its planned publication row: %v", it.ActionID, err)
		}
	}
	// Every item's action is parked awaiting_approval: nothing is
	// approved or dispatched by formation.
	for _, it := range pv.Items {
		row, ferr := e.store.FindAction(ctx, it.ActionID)
		if ferr != nil {
			t.Fatal(ferr)
		}
		if row.State != appconn.ActionAwaitingApproval {
			t.Fatalf("formation must never authorize: %s = %s", it.ActionID, row.State)
		}
	}
	if len(e.dispatch.calls) != 0 {
		t.Fatalf("formation must not dispatch: %v", e.dispatch.calls)
	}
}

// TestPlanFormMidItemFailureLeavesNoPlanRow: a failing item (an update
// destination with no prior publication receipt → the #48
// ErrPublishUpdateTargetNotPublished authority rule) aborts formation
// AFTER earlier items were prepared — the orphans stay awaiting_approval
// (harmless, TTL expiry) and NO plan row exists, so nothing can ever be
// approved or dispatched.
func TestPlanFormMidItemFailureLeavesNoPlanRow(t *testing.T) {
	e := newPlanSvcEnv(t)
	_, err := e.svc.FormPlan(context.Background(), FormInput{TenantID: 7, ActorID: "user-a",
		Items: []ItemInput{item("Doc A"), item("Doc B"), {
			// An update destination the tenant never published through
			// this connection — refused by LatestPublishedByDestination.
			ConnectionID: "conn-notion", SessionID: "sess-1",
			ArtifactVersionID: "ver-1", Title: "Doc C", PageID: "page-404",
		}}})
	if !errors.Is(err, publish.ErrPublishUpdateTargetNotPublished) {
		t.Fatalf("the unpublished update target must abort formation: %v", err)
	}
	if err == nil {
		t.Fatal("the unreadable destination must abort formation")
	}
	plans, _ := e.plans.ListPlanItems(context.Background(), 7, "nonexistent")
	if len(plans) != 0 {
		t.Fatalf("no plan items may exist: %+v", plans)
	}
}

func TestPlanFormRejectsInvalidInput(t *testing.T) {
	e := newPlanSvcEnv(t)
	if _, err := e.svc.FormPlan(context.Background(), FormInput{TenantID: 7, ActorID: "user-a"}); !errors.Is(err, ErrPlanInvalidInput) {
		t.Fatalf("zero-item plan refused, got %v", err)
	}
	if _, err := e.svc.FormPlan(context.Background(), FormInput{TenantID: 0, ActorID: "user-a", Items: []ItemInput{item("A")}}); !errors.Is(err, ErrPlanInvalidInput) {
		t.Fatalf("missing tenant refused, got %v", err)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/modules/appconnector/plan/ -count=1`
Expected: 编译失败（包 `plan` 不存在）

- [ ] **Step 3: 最小实现——plan.go 骨架 + PlanDigest + FormPlan**

`internal/modules/appconnector/plan/plan.go`（本任务先落地文件头、哨兵、常量、类型、PlanDigest、Service 构造器与 FormPlan；Approve/Execute/Status 的方法体 Task 3/4 再补——先用如下占位保证编译，**占位实现必须在 Task 3/4 被真实实现替换，不得留到计划完成**）：

```go
// Package plan owns the multi-action Action Plan seam (T21 #51): an
// ORDERED set of already-prepared external actions approved as ONE
// decision under ONE plan digest (CONTEXT.md 操作计划). Any content,
// connection, target or set change forms a NEW plan with a NEW digest,
// so an old approval can never authorize new content (AC1); exclusion is
// an approval-time decision; per-item results stay on the authoritative
// app_actions rows and the plan only projects them. Partial-success
// recovery re-dispatches ONLY authorized (approved, unsent) items — a
// confirmed outcome is never repeated (AC2).
package plan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/publish"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
)

// Sentinel errors surfaced by the plan service.
var (
	// ErrPlanInvalidInput: malformed form/approve input (zero items,
	// unknown or duplicated exclusion seqs, empty digest/actor).
	ErrPlanInvalidInput = errors.New("plan_invalid_input")
	// ErrPlanDigestMismatch: the presented digest was issued for
	// DIFFERENT plan content — the AC1 refusal. An approval (or an
	// execute request) bound to one plan can never authorize another.
	ErrPlanDigestMismatch = errors.New("plan_digest_mismatch")
	// ErrPlanState: invalid plan lifecycle transition (executing an
	// unapproved plan, an exclusion-set rewrite after approval, a lost
	// approval CAS).
	ErrPlanState = errors.New("plan_state_conflict")
)

// Plan lifecycle states (persisted on app_action_plans.state).
const (
	PlanStateAwaitingApproval = "awaiting_approval"
	PlanStateAuthorized       = "authorized"
)

// Per-item dispositions of one plan execution pass.
const (
	ItemExecuted          = "executed"           // dispatched through the publish pipeline this pass
	ItemSkippedSucceeded  = "skipped_succeeded"  // AC2: already succeeded — never re-dispatched
	ItemSkippedUnapproved = "skipped_unapproved" // plan authorized but this item's approval absent
	ItemSkippedInFlight   = "skipped_in_flight"  // queued/dispatched — a live writer owns it
	ItemSettled           = "settled"            // failed/unknown — a confirmed outcome resume must not redo
	ItemExcluded          = "excluded"           // excluded at approval time — never dispatched
)

// planDigestVersion is the plan digest layout generation.
const planDigestVersion = 1

// DigestItem is one plan item's identity as bound by PlanDigest.
type DigestItem struct {
	Seq          int    `json:"seq"`
	ActionID     string `json:"action_id"`
	ActionDigest string `json:"action_digest"`
	Connection   string `json:"connection"`
	Target       string `json:"target"`
	Risk         string `json:"risk"`
}

// PlanDigest hashes the plan's full approval material as ONE structured
// JSON document (the ActionDigest layout discipline): the layout
// version, the tenant and actor identity, and the ORDERED items with
// their action ids, action digests, connections, targets and risks.
// Changing any item's content (→ a NEW action digest), connection,
// target, risk, the order or the set changes the digest — an approval
// for one exact plan can never authorize another (AC1).
func PlanDigest(tenantID uint64, actorID string, items []DigestItem) (string, error) {
	if len(items) == 0 {
		return "", fmt.Errorf("%w: a plan binds at least one item", ErrPlanInvalidInput)
	}
	material := struct {
		Version int          `json:"version"`
		Tenant  uint64       `json:"tenant"`
		Actor   string       `json:"actor"`
		Items   []DigestItem `json:"items"`
	}{planDigestVersion, tenantID, actorID, items}
	b, err := json.Marshal(material)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// ItemInput is one plan item's formation input — exactly the
// single-action publish plan input (publish.PublishPlanInput minus the
// tenant/actor the plan carries once for all items).
type ItemInput struct {
	ConnectionID      string
	SessionID         string
	ArtifactVersionID string
	Title             string
	ParentPageID      string // exactly one of parent/page per item
	PageID            string
}

// FormInput forms one multi-action plan.
type FormInput struct {
	TenantID uint64
	ActorID  string
	Items    []ItemInput
}

// ItemView is one item's formation view.
type ItemView struct {
	Seq                     int    `json:"seq"`
	ActionID                string `json:"action_id"`
	Digest                  string `json:"digest"`
	Mode                    string `json:"mode"`
	Destination             string `json:"destination"`
	ExpectedExternalVersion string `json:"expected_external_version"`
	Title                   string `json:"title"`
}

// PlanView is the formed plan as returned to the caller: the plan digest
// an approval binds, plus the ordered per-item views. In Status
// projections the items carry identity only (Seq/ActionID) — the details
// live in the per-item outcomes.
type PlanView struct {
	ID     string     `json:"id"`
	State  string     `json:"state"`
	Digest string     `json:"digest"`
	Items  []ItemView `json:"items"`
}

// ApproveInput is the owner's whole-plan decision: the digest shown at
// approval time and the optional excluded seqs (排除单项). Excluding item
// N approves the plan WITHOUT it: the excluded action stays
// awaiting_approval forever and is never dispatched.
type ApproveInput struct {
	Digest      string
	ExcludeSeqs []int
}

// ItemOutcome is one item's result of one execution pass.
type ItemOutcome struct {
	Seq         int                        `json:"seq"`
	ActionID    string                     `json:"action_id"`
	Disposition string                     `json:"disposition"`
	ActionState string                     `json:"action_state"`
	Conflict    bool                       `json:"conflict"`
	Publication publish.PublishReceiptView `json:"publication"`
}

// ExecuteOutcome is one ordered execution pass over the plan.
type ExecuteOutcome struct {
	PlanID string        `json:"plan_id"`
	Digest string        `json:"digest"`
	Items  []ItemOutcome `json:"items"`
}

// PlanStatus is the durable projection: plan + per-item authoritative
// states + receipts (计划/逐项结果查询).
type PlanStatus struct {
	PlanView
	Excluded []int         `json:"excluded"`
	Outcomes []ItemOutcome `json:"outcomes"`
}

// PlanApprover is the per-item approval face of the A03 ActionService.
type PlanApprover interface {
	Approve(ctx context.Context, id, actor, digest string) error
}

// Service drives the persisted plan lifecycle on top of the #48 publish
// seam. Per-item actions stay the A03 authority (digests, approvals,
// dispatch claims); the plan layer adds the set digest (AC1), exclusions
// and ordered partial-success execution (AC2).
type Service struct {
	plans    *repoappconn.PlanStore
	actions  appconnectorsvc.ActionStoreSource
	approver PlanApprover
	publish  *publish.NotionPublishService
}

// NewService builds the plan service.
func NewService(plans *repoappconn.PlanStore, actions appconnectorsvc.ActionStoreSource,
	approver PlanApprover, pubs *publish.NotionPublishService) *Service {
	return &Service{plans: plans, actions: actions, approver: approver, publish: pubs}
}

// FormPlan forms one multi-action plan: every item goes through the #48
// single-action formation (server-derived snapshot, external baseline
// read, A03 Prepare, planned publication row); the plan digest then
// binds the ORDERED action identities read back from the AUTHORITATIVE
// rows (never the formation echo). A mid-formation failure leaves
// already-prepared actions orphaned in awaiting_approval (never
// approved, never dispatched, approval TTL expiry) and creates NO plan
// row — fail closed, mirror of the single-action failure mode.
func (s *Service) FormPlan(ctx context.Context, in FormInput) (PlanView, error) {
	if in.TenantID == 0 || in.ActorID == "" || len(in.Items) == 0 {
		return PlanView{}, fmt.Errorf("%w: tenant, actor and at least one item are required", ErrPlanInvalidInput)
	}
	items := make([]DigestItem, 0, len(in.Items))
	views := make([]ItemView, 0, len(in.Items))
	rows := make([]repoappconn.ActionPlanItemRow, 0, len(in.Items))
	for i, item := range in.Items {
		pv, err := s.publish.FormPlan(ctx, publish.PublishPlanInput{
			TenantID: in.TenantID, ActorID: in.ActorID, ConnectionID: item.ConnectionID,
			SessionID: item.SessionID, ArtifactVersionID: item.ArtifactVersionID,
			Title: item.Title, ParentPageID: item.ParentPageID, PageID: item.PageID,
		})
		if err != nil {
			return PlanView{}, err
		}
		row, err := s.actions.FindAction(ctx, pv.ActionID)
		if err != nil {
			return PlanView{}, err
		}
		items = append(items, DigestItem{
			Seq: i + 1, ActionID: row.ID, ActionDigest: row.ArgsDigest,
			Connection: row.ConnectionID, Target: row.Target, Risk: row.Risk,
		})
		views = append(views, ItemView{
			Seq: i + 1, ActionID: pv.ActionID, Digest: pv.Digest, Mode: pv.Mode,
			Destination: pv.Destination, ExpectedExternalVersion: pv.ExpectedExternalVersion,
			Title: pv.Title,
		})
		rows = append(rows, repoappconn.ActionPlanItemRow{TenantID: in.TenantID, Seq: i + 1, ActionID: row.ID})
	}
	digest, err := PlanDigest(in.TenantID, in.ActorID, items)
	if err != nil {
		return PlanView{}, err
	}
	planID := "plan_" + uuid.NewString()
	for i := range rows {
		rows[i].PlanID = planID
	}
	if err := s.plans.CreatePlan(ctx, repoappconn.ActionPlanRow{
		ID: planID, TenantID: in.TenantID, ActorID: in.ActorID,
		Digest: digest, State: PlanStateAwaitingApproval,
	}, rows); err != nil {
		return PlanView{}, err
	}
	return PlanView{ID: planID, State: PlanStateAwaitingApproval, Digest: digest, Items: views}, nil
}

// Approve records the owner's whole-plan decision. IMPLEMENT IN TASK 3.
// Execute runs one ordered pass over an approved plan. IMPLEMENT IN TASK 4.
// Status returns the durable plan projection. IMPLEMENT IN TASK 4.

// normalizeExclusions validates the caller's exclusion seqs: within
// 1..n, no duplicates; nil means approve everything. The result is
// sorted so the stored/compared form is canonical.
func normalizeExclusions(seqs []int, n int) ([]int, error) {
	if len(seqs) == 0 {
		return []int{}, nil
	}
	seen := map[int]bool{}
	out := make([]int, 0, len(seqs))
	for _, seq := range seqs {
		if seq < 1 || seq > n {
			return nil, fmt.Errorf("%w: exclude_seq %d outside plan items 1..%d", ErrPlanInvalidInput, seq, n)
		}
		if seen[seq] {
			return nil, fmt.Errorf("%w: duplicate exclude_seq %d", ErrPlanInvalidInput, seq)
		}
		seen[seq] = true
		out = append(out, seq)
	}
	sort.Ints(out)
	return out, nil
}

// parseExclusions reads the plan row's frozen exclusion record.
func parseExclusions(raw string) ([]int, error) {
	if raw == "" {
		return []int{}, nil
	}
	var out []int
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("%w: corrupt exclusion record: %v", ErrPlanState, err)
	}
	return out, nil
}

func equalSeqs(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// view rebuilds the plan identity view from the store.
func (s *Service) view(ctx context.Context, tenantID uint64, planID string) (PlanView, error) {
	row, err := s.plans.FindPlan(ctx, tenantID, planID)
	if err != nil {
		return PlanView{}, err
	}
	items, err := s.plans.ListPlanItems(ctx, tenantID, planID)
	if err != nil {
		return PlanView{}, err
	}
	views := make([]ItemView, 0, len(items))
	for _, item := range items {
		views = append(views, ItemView{Seq: item.Seq, ActionID: item.ActionID})
	}
	return PlanView{ID: row.ID, State: row.State, Digest: row.Digest, Items: views}, nil
}

var _ = appconn.ActionAwaitingApproval // used from Task 3 onward
var _ = time.Now                       // used from Task 3 onward
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/modules/appconnector/plan/ -count=1 && go build ./...`
Expected: PASS（4 个测试）+ 全仓编译通过

- [ ] **Step 5: Commit**

```bash
git add internal/modules/appconnector/plan/plan.go internal/modules/appconnector/plan/plan_test.go
git commit -m "feat(appconnector): plan 包——PlanDigest 纯函数 + FormPlan（T21 #51 Task 2）"
```

---

### Task 3: plan 包——Approve（整体批准 / 排除单项 / AC1 批准面）

**Files:**
- Modify: `internal/modules/appconnector/plan/plan.go`（替换 `// Approve ... IMPLEMENT IN TASK 3.` 占位为真实实现）
- Test: `internal/modules/appconnector/plan/plan_test.go`（追加）

**Interfaces:**
- Consumes: Task 1 `PlanStore.ApprovePlan/FindPlan/ListPlanItems`；Task 2 的类型与 helpers；`appconnectorsvc.ActionService.Approve`（经 `PlanApprover`）；`appconn.ActionAwaitingApproval/ActionAuthorized`
- Produces: `(s *Service) Approve(ctx context.Context, tenantID uint64, planID, actor string, in ApproveInput) (PlanView, error)`——digest 不匹配→`ErrPlanDigestMismatch`；非法排除→`ErrPlanInvalidInput`；授权后改排除集→`ErrPlanState`；被排除项/已终态项跳过单项批准；返回恢复后的 `PlanView`

- [ ] **Step 1: 追加失败测试**（`internal/modules/appconnector/plan/plan_test.go` 末尾）

```go
func (e *planSvcEnv) approveAll(t *testing.T, pv PlanView) {
	t.Helper()
	if _, err := e.svc.Approve(context.Background(), 7, pv.ID, "user-a", ApproveInput{Digest: pv.Digest}); err != nil {
		t.Fatal(err)
	}
}

// TestPlanApproveWholeApprovesEveryIncludedItem: the whole-plan decision
// approves every included item with its own digest-bound A03 approval.
func TestPlanApproveWholeApprovesEveryIncludedItem(t *testing.T) {
	e := newPlanSvcEnv(t)
	pv := e.formTwo(t)
	e.approveAll(t, pv)
	for _, it := range pv.Items {
		row, err := e.store.FindAction(context.Background(), it.ActionID)
		if err != nil {
			t.Fatal(err)
		}
		if row.State != appconn.ActionAuthorized {
			t.Fatalf("included item %s must be authorized, got %s", it.ActionID, row.State)
		}
	}
	got, _ := e.plans.FindPlan(context.Background(), 7, pv.ID)
	if got.State != PlanStateAuthorized || got.ApprovedBy != "user-a" || got.ApprovedAt == nil {
		t.Fatalf("plan approval must be recorded: %+v", got)
	}
}

// TestPlanApproveExcludesItemNeverApprovesIt (排除单项): the excluded
// item stays awaiting_approval forever and can never be dispatched by
// this plan; the exclusion is recorded on the plan row.
func TestPlanApproveExcludesItemNeverApprovesIt(t *testing.T) {
	e := newPlanSvcEnv(t)
	pv := e.formTwo(t)
	if _, err := e.svc.Approve(context.Background(), 7, pv.ID, "user-a",
		ApproveInput{Digest: pv.Digest, ExcludeSeqs: []int{2}}); err != nil {
		t.Fatal(err)
	}
	first, _ := e.store.FindAction(context.Background(), pv.Items[0].ActionID)
	second, _ := e.store.FindAction(context.Background(), pv.Items[1].ActionID)
	if first.State != appconn.ActionAuthorized {
		t.Fatalf("included item must be authorized, got %s", first.State)
	}
	if second.State != appconn.ActionAwaitingApproval {
		t.Fatalf("excluded item must stay awaiting_approval, got %s", second.State)
	}
	got, _ := e.plans.FindPlan(context.Background(), 7, pv.ID)
	if got.ExcludedJSON != "[2]" {
		t.Fatalf("exclusion must be recorded on the plan row: %q", got.ExcludedJSON)
	}
}

// TestPlanApproveRejectsForeignDigest is the AC1 approval-side anchor: a
// digest issued for DIFFERENT plan content (here: another plan's digest —
// 计划内容变化) authorizes nothing, and a refusal moves nothing.
func TestPlanApproveRejectsForeignDigest(t *testing.T) {
	e := newPlanSvcEnv(t)
	p1 := e.formTwo(t)
	// A second plan over the same connection/version but CHANGED content
	// (different titles → different items → different digests).
	p2, err := e.svc.FormPlan(context.Background(), FormInput{TenantID: 7, ActorID: "user-a",
		Items: []ItemInput{item("Doc A v2"), item("Doc B v2")}})
	if err != nil {
		t.Fatal(err)
	}
	if p1.Digest == p2.Digest {
		t.Fatal("计划内容变化必须产生新 plan digest")
	}
	// The OLD digest must not approve the NEW content — the plan-level
	// mirror of TestApprovalLifecycleHappyPath's per-action refusal.
	if _, err := e.svc.Approve(context.Background(), 7, p2.ID, "user-a", ApproveInput{Digest: p1.Digest}); !errors.Is(err, ErrPlanDigestMismatch) {
		t.Fatalf("old plan digest approved new content: %v", err)
	}
	if _, err := e.svc.Approve(context.Background(), 7, p1.ID, "user-a", ApproveInput{Digest: "deadbeef"}); !errors.Is(err, ErrPlanDigestMismatch) {
		t.Fatalf("wrong digest accepted: %v", err)
	}
	for _, pv := range []PlanView{p1, p2} {
		got, _ := e.plans.FindPlan(context.Background(), 7, pv.ID)
		if got.State != PlanStateAwaitingApproval {
			t.Fatalf("refused approve must not move state: %s", got.State)
		}
	}
}

func TestPlanApproveRejectsBadExclusions(t *testing.T) {
	e := newPlanSvcEnv(t)
	pv := e.formTwo(t)
	if _, err := e.svc.Approve(context.Background(), 7, pv.ID, "user-a",
		ApproveInput{Digest: pv.Digest, ExcludeSeqs: []int{3}}); !errors.Is(err, ErrPlanInvalidInput) {
		t.Fatalf("out-of-range exclude seq refused, got %v", err)
	}
	if _, err := e.svc.Approve(context.Background(), 7, pv.ID, "user-a",
		ApproveInput{Digest: pv.Digest, ExcludeSeqs: []int{1, 1}}); !errors.Is(err, ErrPlanInvalidInput) {
		t.Fatalf("duplicate exclude seq refused, got %v", err)
	}
}
```

（本任务的测试只依赖 Approve；`TestPlanApproveRecoveryExclusionFrozen` 需要 Task 4 的 Execute，归入 Task 4 Step 1。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/modules/appconnector/plan/ -run TestPlanApprove -count=1`
Expected: 编译失败（`e.svc.Approve`/`ApproveInput` 未定义）

- [ ] **Step 3: 最小实现——替换占位注释为真实 Approve**

在 `internal/modules/appconnector/plan/plan.go` 中，把：

```go
// Approve records the owner's whole-plan decision. IMPLEMENT IN TASK 3.
```

替换为：

```go
// Approve records the owner's whole-plan decision: the digest must match
// the plan's CURRENT content (AC1 — a digest issued for different content
// is refused, never migrated), the exclusion set is recorded on the plan
// row, and every INCLUDED still-awaiting item receives its own
// digest-bound A03 approval. Excluded items are never approved and never
// dispatched. Re-approval is the recovery path: the exclusion set is
// frozen at first approval (a different set after approval is a state
// conflict), already-authorized items are idempotent no-ops, and items
// still awaiting approval (a prior partial approve) are approved now.
func (s *Service) Approve(ctx context.Context, tenantID uint64, planID, actor string, in ApproveInput) (PlanView, error) {
	if actor == "" || in.Digest == "" {
		return PlanView{}, fmt.Errorf("%w: actor and digest are required", ErrPlanInvalidInput)
	}
	row, err := s.plans.FindPlan(ctx, tenantID, planID)
	if err != nil {
		return PlanView{}, err
	}
	if row.Digest != in.Digest {
		return PlanView{}, fmt.Errorf("%w: approval digest does not match plan content %s", ErrPlanDigestMismatch, planID)
	}
	items, err := s.plans.ListPlanItems(ctx, tenantID, planID)
	if err != nil {
		return PlanView{}, err
	}
	excluded, err := normalizeExclusions(in.ExcludeSeqs, len(items))
	if err != nil {
		return PlanView{}, err
	}
	if row.State == PlanStateAuthorized {
		// Recovery re-approval: the exclusion set is frozen at first
		// approval — a different set after approval is a state conflict,
		// never a silent rewrite.
		recorded, perr := parseExclusions(row.ExcludedJSON)
		if perr != nil {
			return PlanView{}, perr
		}
		if !equalSeqs(recorded, excluded) {
			return PlanView{}, fmt.Errorf("%w: plan already approved with exclusions %v", ErrPlanState, recorded)
		}
	}
	excludedJSON, err := json.Marshal(excluded)
	if err != nil {
		return PlanView{}, err
	}
	if err := s.plans.ApprovePlan(ctx, tenantID, planID, in.Digest, actor, string(excludedJSON), time.Now().UTC()); err != nil {
		return PlanView{}, err
	}
	exSet := map[int]bool{}
	for _, seq := range excluded {
		exSet[seq] = true
	}
	for _, item := range items {
		if exSet[item.Seq] {
			continue // 排除单项：永不批准、永不派发
		}
		action, aerr := s.actions.FindAction(ctx, item.ActionID)
		if aerr != nil {
			return PlanView{}, aerr
		}
		if action.State != appconn.ActionAwaitingApproval {
			// authorized: already approved (idempotent); queued/dispatched/
			// terminal: settled — an approval is no longer applicable.
			continue
		}
		if aerr := s.approver.Approve(ctx, item.ActionID, actor, action.ArgsDigest); aerr != nil {
			return PlanView{}, aerr
		}
	}
	return s.view(ctx, tenantID, planID)
}
```

并删除文件尾部两行占位卫兵 `var _ = appconn.ActionAwaitingApproval` 与 `var _ = time.Now`（`appconn` 与 `time` 从本任务起被真实使用；`sort`/`json` 已被 helpers 使用）。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/modules/appconnector/plan/ -count=1 && go build ./...`
Expected: PASS（全部 plan 测试）+ 编译通过

- [ ] **Step 5: Commit**

```bash
git add internal/modules/appconnector/plan/plan.go internal/modules/appconnector/plan/plan_test.go
git commit -m "feat(appconnector): plan Approve——整体批准/排除单项/AC1 批准面（T21 #51 Task 3）"
```

---

### Task 4: plan 包——Execute 与 Status（AC2 部分成功恢复）

**Files:**
- Modify: `internal/modules/appconnector/plan/plan.go`（替换 `// Execute ...` 与 `// Status ...` 占位为真实实现）
- Test: `internal/modules/appconnector/plan/plan_test.go`（追加）

**Interfaces:**
- Consumes: Task 3 的 `Approve`；`publish.NotionPublishService.Execute(ctx, tenantID, actionID) (PublishExecuteOutcome, error)` 与 `.Receipt`；`appconn` 全部状态常量
- Produces: `(s *Service) Execute(ctx context.Context, tenantID uint64, planID, digest string) (ExecuteOutcome, error)`（disposition 语义见 Task 2 常量）；`(s *Service) Status(ctx context.Context, tenantID uint64, planID string) (PlanStatus, error)`

- [ ] **Step 1: 追加失败测试**（`plan_test.go` 末尾）

```go
// TestPlanExecuteRunsIncludedItemsInOrderAndSettlesReceipts: one pass
// runs every included authorized item in seq order and each item
// settles its own published receipt.
func TestPlanExecuteRunsIncludedItemsInOrderAndSettlesReceipts(t *testing.T) {
	e := newPlanSvcEnv(t)
	pv := e.formTwo(t)
	e.dispatch.outcomes[pv.Items[0].ActionID] = appconnectorsvc.DispatchOutcome{
		Status: appconn.ActionSucceeded, ProviderResult: okReceipt("page-a")}
	e.dispatch.outcomes[pv.Items[1].ActionID] = appconnectorsvc.DispatchOutcome{
		Status: appconn.ActionSucceeded, ProviderResult: okReceipt("page-b")}
	e.approveAll(t, pv)
	out, err := e.svc.Execute(context.Background(), 7, pv.ID, pv.Digest)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 2 || out.Items[0].Seq != 1 || out.Items[1].Seq != 2 {
		t.Fatalf("outcomes must be ordered: %+v", out.Items)
	}
	for i, oc := range out.Items {
		if oc.Disposition != ItemExecuted || oc.ActionState != appconn.ActionSucceeded {
			t.Fatalf("item %d must execute to success: %+v", i+1, oc)
		}
		if oc.Publication.State != "published" || oc.Publication.ExternalID == "" || oc.Publication.ExternalVersion == "" {
			t.Fatalf("receipt must settle published with external version: %+v", oc.Publication)
		}
	}
	if e.dispatch.calls[pv.Items[0].ActionID] != 1 || e.dispatch.calls[pv.Items[1].ActionID] != 1 {
		t.Fatalf("each item dispatched exactly once: %v", e.dispatch.calls)
	}
}

// TestPlanExecuteSkipsConfirmedOutcomesOnResume is the AC2 core: a
// second pass over a partially-successful plan re-dispatches NOTHING
// already confirmed (succeeded/failed/unknown) — the dispatch counts
// prove it.
func TestPlanExecuteSkipsConfirmedOutcomesOnResume(t *testing.T) {
	e := newPlanSvcEnv(t)
	pv, err := e.svc.FormPlan(context.Background(), FormInput{TenantID: 7, ActorID: "user-a",
		Items: []ItemInput{item("Doc A"), item("Doc B"), item("Doc C")}})
	if err != nil {
		t.Fatal(err)
	}
	e.dispatch.outcomes[pv.Items[0].ActionID] = appconnectorsvc.DispatchOutcome{
		Status: appconn.ActionSucceeded, ProviderResult: okReceipt("page-a")}
	e.dispatch.outcomes[pv.Items[1].ActionID] = appconnectorsvc.DispatchOutcome{
		Status: appconn.ActionFailed, ProviderResult: publish.PublishVersionConflictResult + ": expected v1 remote v2"}
	e.dispatch.errs[pv.Items[2].ActionID] = errors.New("transport lost")
	e.approveAll(t, pv)
	out, err := e.svc.Execute(context.Background(), 7, pv.ID, pv.Digest)
	if err != nil {
		t.Fatal(err)
	}
	if out.Items[0].Disposition != ItemExecuted || out.Items[0].ActionState != appconn.ActionSucceeded {
		t.Fatalf("item 1: %+v", out.Items[0])
	}
	if out.Items[1].Disposition != ItemExecuted || out.Items[1].ActionState != appconn.ActionFailed || !out.Items[1].Conflict {
		t.Fatalf("item 2 must record the conflict failure: %+v", out.Items[1])
	}
	if out.Items[2].Disposition != ItemExecuted || out.Items[2].ActionState != appconn.ActionUnknown {
		t.Fatalf("item 3 must park unknown: %+v", out.Items[2])
	}
	// Resume with the SAME digest: zero new dispatches anywhere.
	out2, err := e.svc.Execute(context.Background(), 7, pv.ID, pv.Digest)
	if err != nil {
		t.Fatal(err)
	}
	if out2.Items[0].Disposition != ItemSkippedSucceeded {
		t.Fatalf("AC2: succeeded item must be skipped, got %+v", out2.Items[0])
	}
	if out2.Items[1].Disposition != ItemSettled || out2.Items[1].ActionState != appconn.ActionFailed {
		t.Fatalf("failed item must stay settled, never re-dispatched: %+v", out2.Items[1])
	}
	if out2.Items[2].Disposition != ItemSettled || out2.Items[2].ActionState != appconn.ActionUnknown {
		t.Fatalf("unknown item must stay parked: %+v", out2.Items[2])
	}
	for _, it := range pv.Items {
		if e.dispatch.calls[it.ActionID] != 1 {
			t.Fatalf("AC2: item %s dispatched %d times, want exactly 1", it.ActionID, e.dispatch.calls[it.ActionID])
		}
	}
}

// TestPlanExecuteRefusesUnapprovedOrForeignDigest: the plan-level gates —
// an awaiting_approval plan never executes; a foreign digest never
// executes (AC1 at execute time); a missing plan is not-found.
func TestPlanExecuteRefusesUnapprovedOrForeignDigest(t *testing.T) {
	e := newPlanSvcEnv(t)
	pv := e.formTwo(t)
	if _, err := e.svc.Execute(context.Background(), 7, pv.ID, pv.Digest); !errors.Is(err, ErrPlanState) {
		t.Fatalf("unapproved plan must not execute, got %v", err)
	}
	e.approveAll(t, pv)
	if _, err := e.svc.Execute(context.Background(), 7, pv.ID, "stale-digest"); !errors.Is(err, ErrPlanDigestMismatch) {
		t.Fatalf("execute with foreign digest must be refused, got %v", err)
	}
	if _, err := e.svc.Execute(context.Background(), 7, "plan-x", pv.Digest); !errors.Is(err, repoappconn.ErrPlanNotFound) {
		t.Fatalf("unknown plan must be not-found, got %v", err)
	}
}

// TestPlanExecuteSkipsUnapprovedItemFailClosed: an included item whose
// single-item approval is absent (partial-approve window) is never
// dispatched by the plan pass.
func TestPlanExecuteSkipsUnapprovedItemFailClosed(t *testing.T) {
	e := newPlanSvcEnv(t)
	pv := e.formTwo(t)
	e.dispatch.outcomes[pv.Items[0].ActionID] = appconnectorsvc.DispatchOutcome{
		Status: appconn.ActionSucceeded, ProviderResult: okReceipt("page-a")}
	e.approveAll(t, pv)
	// Simulate the partial-approval window: item 2 back to awaiting.
	if err := e.store.SetActionState(context.Background(), pv.Items[1].ActionID,
		appconn.ActionAuthorized, appconn.ActionAwaitingApproval); err != nil {
		t.Fatal(err)
	}
	out, err := e.svc.Execute(context.Background(), 7, pv.ID, pv.Digest)
	if err != nil {
		t.Fatal(err)
	}
	if out.Items[1].Disposition != ItemSkippedUnapproved {
		t.Fatalf("unapproved item must be skipped fail-closed: %+v", out.Items[1])
	}
	if e.dispatch.calls[pv.Items[1].ActionID] != 0 {
		t.Fatalf("unapproved item dispatched: %v", e.dispatch.calls)
	}
}

// TestPlanStatusProjectsPerItemResults: the durable projection — plan
// identity, the frozen exclusion set, and each item's authoritative
// action state + receipt.
func TestPlanStatusProjectsPerItemResults(t *testing.T) {
	e := newPlanSvcEnv(t)
	pv, err := e.svc.FormPlan(context.Background(), FormInput{TenantID: 7, ActorID: "user-a",
		Items: []ItemInput{item("Doc A"), item("Doc B")}})
	if err != nil {
		t.Fatal(err)
	}
	e.dispatch.outcomes[pv.Items[0].ActionID] = appconnectorsvc.DispatchOutcome{
		Status: appconn.ActionSucceeded, ProviderResult: okReceipt("page-a")}
	if _, err := e.svc.Approve(context.Background(), 7, pv.ID, "user-a",
		ApproveInput{Digest: pv.Digest, ExcludeSeqs: []int{2}}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Execute(context.Background(), 7, pv.ID, pv.Digest); err != nil {
		t.Fatal(err)
	}
	st, err := e.svc.Status(context.Background(), 7, pv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if st.ID != pv.ID || st.State != PlanStateAuthorized || len(st.Excluded) != 1 || st.Excluded[0] != 2 {
		t.Fatalf("plan projection: %+v excluded=%v", st.PlanView, st.Excluded)
	}
	if len(st.Outcomes) != 2 {
		t.Fatalf("per-item outcomes: %+v", st.Outcomes)
	}
	if st.Outcomes[0].ActionState != appconn.ActionSucceeded || st.Outcomes[0].Publication.State != "published" {
		t.Fatalf("item 1 projection: %+v", st.Outcomes[0])
	}
	if st.Outcomes[1].ActionState != appconn.ActionAwaitingApproval {
		t.Fatalf("excluded item projects its untouched action state: %+v", st.Outcomes[1])
	}
}

// TestPlanApproveRecoveryExclusionFrozen: after execution started the
// recorded exclusion set is frozen — a different set is a state
// conflict; the same-set re-approve stays legal (recovery path).
// （Review Focus 3 的排除集冻结腿；依赖本任务的 Execute，故归此。）
func TestPlanApproveRecoveryExclusionFrozen(t *testing.T) {
	e := newPlanSvcEnv(t)
	pv := e.formTwo(t)
	e.approveAll(t, pv)
	e.dispatch.outcomes[pv.Items[0].ActionID] = appconnectorsvc.DispatchOutcome{
		Status: appconn.ActionSucceeded, ProviderResult: okReceipt("page-1")}
	if _, err := e.svc.Execute(context.Background(), 7, pv.ID, pv.Digest); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Approve(context.Background(), 7, pv.ID, "user-a",
		ApproveInput{Digest: pv.Digest, ExcludeSeqs: []int{2}}); !errors.Is(err, ErrPlanState) {
		t.Fatalf("exclusion rewrite after execution must be refused, got %v", err)
	}
	if _, err := e.svc.Approve(context.Background(), 7, pv.ID, "user-a", ApproveInput{Digest: pv.Digest}); err != nil {
		t.Fatalf("idempotent re-approve refused: %v", err)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/modules/appconnector/plan/ -run 'TestPlanExecute|TestPlanStatus|TestPlanApproveRecovery' -count=1`
Expected: 编译失败（`Execute`/`Status` 未定义——含从 Task 3 挪入的 `TestPlanApproveRecoveryExclusionFrozen`）

- [ ] **Step 3: 最小实现——替换两个占位注释**

把 `// Execute runs one ordered pass over an approved plan. IMPLEMENT IN TASK 4.` 与 `// Status returns the durable plan projection. IMPLEMENT IN TASK 4.` 替换为：

```go
// Execute runs one ordered pass over an APPROVED plan whose digest the
// caller presents (AC1 holds at execute time too). Per item, in seq
// order:
//
//   - excluded → recorded excluded, never dispatched;
//   - succeeded → skipped_succeeded: the confirmed outcome is carried,
//     the dispatch count stays untouched (AC2 — 部分成功只恢复确认未完成
//     的动作, never a re-run);
//   - authorized → dispatched through the publish pipeline (the resume
//     case: approved but not yet sent);
//   - awaiting_approval → skipped_unapproved (fail closed: no dispatch);
//   - queued/dispatched → skipped_in_flight (a live writer owns it — the
//     single-action approval count + state CAS are the deeper guards);
//   - failed/unknown → settled (confirmed outcomes of their own kind; a
//     re-send needs a NEW plan — re-preparing changes the content and the
//     AC1 digest with it).
//
// A failed or unknown item does NOT stop the pass: items are independent
// external effects and every one records its own result (CONTEXT.md:
// 执行结果仍逐项持久记录). Later passes are idempotent.
func (s *Service) Execute(ctx context.Context, tenantID uint64, planID, digest string) (ExecuteOutcome, error) {
	row, err := s.plans.FindPlan(ctx, tenantID, planID)
	if err != nil {
		return ExecuteOutcome{}, err
	}
	if row.Digest != digest {
		return ExecuteOutcome{}, fmt.Errorf("%w: execute digest does not match plan content %s", ErrPlanDigestMismatch, planID)
	}
	if row.State != PlanStateAuthorized {
		return ExecuteOutcome{}, fmt.Errorf("%w: plan is %s, execute requires authorized", ErrPlanState, row.State)
	}
	items, err := s.plans.ListPlanItems(ctx, tenantID, planID)
	if err != nil {
		return ExecuteOutcome{}, err
	}
	excluded, err := parseExclusions(row.ExcludedJSON)
	if err != nil {
		return ExecuteOutcome{}, err
	}
	exSet := map[int]bool{}
	for _, seq := range excluded {
		exSet[seq] = true
	}
	out := ExecuteOutcome{PlanID: planID, Digest: row.Digest, Items: []ItemOutcome{}}
	for _, item := range items {
		action, aerr := s.actions.FindAction(ctx, item.ActionID)
		if aerr != nil {
			return ExecuteOutcome{}, aerr
		}
		oc := ItemOutcome{Seq: item.Seq, ActionID: item.ActionID, ActionState: action.State}
		switch {
		case exSet[item.Seq]:
			oc.Disposition = ItemExcluded
		case action.State == appconn.ActionSucceeded:
			oc.Disposition = ItemSkippedSucceeded
		case action.State == appconn.ActionAuthorized:
			exec, xerr := s.publish.Execute(ctx, tenantID, item.ActionID)
			if xerr != nil {
				return ExecuteOutcome{}, xerr
			}
			oc.Disposition = ItemExecuted
			oc.ActionState = exec.ActionState
			oc.Conflict = exec.Conflict
			oc.Publication = exec.Receipt
		case action.State == appconn.ActionAwaitingApproval:
			oc.Disposition = ItemSkippedUnapproved
		case action.State == appconn.ActionQueued || action.State == appconn.ActionDispatched:
			oc.Disposition = ItemSkippedInFlight
		default: // failed / unknown: confirmed outcomes of their own kind
			oc.Disposition = ItemSettled
		}
		if oc.Disposition != ItemExecuted {
			// Project the durable receipt for display; items without a
			// publication row carry an empty view.
			if rcpt, rerr := s.publish.Receipt(ctx, tenantID, item.ActionID); rerr == nil {
				oc.Publication = rcpt
			}
		}
		out.Items = append(out.Items, oc)
	}
	return out, nil
}

// Status returns the durable plan projection: plan identity, the frozen
// exclusion set, and each item's authoritative action state + receipt.
// There is no stored terminal plan state — completion is projected from
// the items.
func (s *Service) Status(ctx context.Context, tenantID uint64, planID string) (PlanStatus, error) {
	st, err := s.view(ctx, tenantID, planID)
	if err != nil {
		return PlanStatus{}, err
	}
	row, err := s.plans.FindPlan(ctx, tenantID, planID)
	if err != nil {
		return PlanStatus{}, err
	}
	excluded, err := parseExclusions(row.ExcludedJSON)
	if err != nil {
		return PlanStatus{}, err
	}
	items, err := s.plans.ListPlanItems(ctx, tenantID, planID)
	if err != nil {
		return PlanStatus{}, err
	}
	outcomes := make([]ItemOutcome, 0, len(items))
	for _, item := range items {
		action, aerr := s.actions.FindAction(ctx, item.ActionID)
		if aerr != nil {
			return PlanStatus{}, aerr
		}
		oc := ItemOutcome{Seq: item.Seq, ActionID: item.ActionID, ActionState: action.State}
		if rcpt, rerr := s.publish.Receipt(ctx, tenantID, item.ActionID); rerr == nil {
			oc.Publication = rcpt
		}
		outcomes = append(outcomes, oc)
	}
	return PlanStatus{PlanView: st, Excluded: excluded, Outcomes: outcomes}, nil
}
```

- [ ] **Step 4: 运行确认通过 + 全包回归**

Run: `go test ./internal/modules/appconnector/... -count=1`
Expected: 全部 7 个包 ok（appconnector root/connectorcontrol/openconnector/publish/repository/service/plan）

- [ ] **Step 5: Commit**

```bash
git add internal/modules/appconnector/plan/plan.go internal/modules/appconnector/plan/plan_test.go
git commit -m "feat(appconnector): plan Execute/Status——部分成功恢复 AC2（T21 #51 Task 4）"
```

---

### Task 5: HTTP 面——handler、路由与容器接线

**Files:**
- Create: `internal/handler/app_connector_action_plan.go`
- Create: `internal/router/routes_app_action_plan.go`
- Modify: `internal/router/router.go:145-147`（RouterParams 增加字段）与 `:436` 后（注册调用）
- Modify: `internal/container/notion_publish.go`（构造器扩为双输出）
- Test: `internal/handler/app_connector_action_plan_test.go`
- Test: `internal/router/routes_app_action_plan_test.go`（路由守卫/存在性测试——router 包函数只能在 router 包内测，handler 包不能反向 import router）

**Interfaces:**
- Consumes: Task 2-4 的 `plan.Service/ItemInput/FormInput/ApproveInput`；`appTenantScope/appOK/appFail/appRequireWriteCapability`；`appconnector.CanDriveActionWrites`；`repoappconn.ErrPlanNotFound`；#48 的 `publish` 哨兵（failForm 映射）；`appconnectorsvc.ErrActionDigestMismatch/ErrActionState/ErrNoDispatcher`
- Produces:
  - `handler.NewAppActionPlanHandler(db *gorm.DB) *AppActionPlanHandler`；`(h) SetActionPlanService(s *plan.Service)`；`(h) RequireActionCapabilityForWrites() gin.HandlerFunc`
  - wire：`POST /api/v1/apps/action-plans`（body `{"items":[{connection_id, session_id, artifact_version_id, title, parent_page_id|page_id}...]}`→201 PlanView）；`POST /api/v1/apps/action-plans/:id/approve`（`{"digest","exclude_seqs"?}`→200 PlanView）；`POST /api/v1/apps/action-plans/:id/execute`（`{"digest"}`→200 ExecuteOutcome）；`GET /api/v1/apps/action-plans/:id`→200 PlanStatus
  - wire 错误码：`ACTION_PLAN_NOT_FOUND`(404)/`ACTION_PLAN_DIGEST_MISMATCH`(409)/`ACTION_PLAN_STATE_CONFLICT`(409)/`ACTION_DIGEST_MISMATCH`(409)/`ACTION_STATE_CONFLICT`(409)/`ACTION_PLAN_PIPELINE_NOT_CONFIGURED`(501)/`ACTION_APPROVAL_FORBIDDEN`(403)，formation 侧沿用 #48 的 `ARTIFACT_VERSION_NOT_ACCESSIBLE`/`PUBLISH_*` 表
  - `router.RegisterAppActionPlanRoutes(r *gin.RouterGroup, h *handler.AppActionPlanHandler)`（nil handler 静默返回，镜像 #48）；`RouterParams.AppActionPlanHandler *handler.AppActionPlanHandler`

- [ ] **Step 1: 写失败测试**

`internal/handler/app_connector_action_plan_test.go`：

```go
package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openActionPlanHandlerDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repoappconn.ActionPlanRow{}, &repoappconn.ActionPlanItemRow{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func actionPlanTestEngine(t *testing.T, h *AppActionPlanHandler) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(7))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "user-a")
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleAdmin)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	v1 := engine.Group("/api/v1")
	// Mirror of RegisterAppActionPlanRoutes (internal/router) — the
	// router package is not importable from this handler-package test.
	v1.POST("/apps/action-plans", h.FormActionPlan)
	v1.POST("/apps/action-plans/:id/approve", h.ApproveActionPlan)
	v1.POST("/apps/action-plans/:id/execute", h.ExecuteActionPlan)
	v1.GET("/apps/action-plans/:id", h.GetActionPlan)
	return engine
}

func doActionPlan(t *testing.T, engine *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

// TestActionPlanHandlerFailClosedWithoutService: until the container
// wires the plan service every endpoint refuses with 501 — the edge
// never fabricates plans, approvals or dispatches.
func TestActionPlanHandlerFailClosedWithoutService(t *testing.T) {
	db := openActionPlanHandlerDB(t)
	h := NewAppActionPlanHandler(db)
	engine := actionPlanTestEngine(t, h)
	w := doActionPlan(t, engine, http.MethodPost, "/api/v1/apps/action-plans",
		`{"items":[{"connection_id":"c","session_id":"s","artifact_version_id":"v","title":"T","parent_page_id":"p"}]}`)
	if w.Code != http.StatusNotImplemented || !strings.Contains(w.Body.String(), "ACTION_PLAN_PIPELINE_NOT_CONFIGURED") {
		t.Fatalf("form must fail closed: %d %s", w.Code, w.Body.String())
	}
	if err := db.Create(&repoappconn.ActionPlanRow{ID: "plan-1", TenantID: 7, ActorID: "user-a",
		Digest: "d", State: repoappconn.PlanStateAwaitingApproval}).Error; err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/apps/action-plans/plan-1/approve", `{"digest":"d"}`},
		{http.MethodPost, "/api/v1/apps/action-plans/plan-1/execute", `{"digest":"d"}`},
		{http.MethodGet, "/api/v1/apps/action-plans/plan-1", ""},
	} {
		w := doActionPlan(t, engine, tc.method, tc.path, tc.body)
		if w.Code != http.StatusNotImplemented || !strings.Contains(w.Body.String(), "ACTION_PLAN_PIPELINE_NOT_CONFIGURED") {
			t.Fatalf("%s %s must fail closed: %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}

// TestActionPlanHandlerValidationAndNotFound: malformed input is a 400
// before the service matters; missing plans are 404 with the plan code.
func TestActionPlanHandlerValidationAndNotFound(t *testing.T) {
	db := openActionPlanHandlerDB(t)
	h := NewAppActionPlanHandler(db)
	engine := actionPlanTestEngine(t, h)
	w := doActionPlan(t, engine, http.MethodPost, "/api/v1/apps/action-plans", `{"items":[]}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "INVALID_REQUEST") {
		t.Fatalf("zero items must be 400: %d %s", w.Code, w.Body.String())
	}
	w = doActionPlan(t, engine, http.MethodPost, "/api/v1/apps/action-plans", `not-json`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("malformed body must be 400: %d %s", w.Code, w.Body.String())
	}
	w = doActionPlan(t, engine, http.MethodPost, "/api/v1/apps/action-plans/plan-x/approve", `{"digest":"d"}`)
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "ACTION_PLAN_NOT_FOUND") {
		t.Fatalf("unknown plan must be 404: %d %s", w.Code, w.Body.String())
	}
	w = doActionPlan(t, engine, http.MethodGet, "/api/v1/apps/action-plans/plan-x", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown plan GET must be 404: %d %s", w.Code, w.Body.String())
	}
}
```

（路由注册的守卫测试属于 `package router` 自身——handler 包不能反向 import router（循环依赖），故放在本任务 Step 4 的新 router 测试文件中。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/handler/ -run TestActionPlan -count=1`
Expected: 编译失败（`AppActionPlanHandler` 未定义——handler 侧两测试）

- [ ] **Step 3: 实现 handler**

`internal/handler/app_connector_action_plan.go`：

```go
package handler

import (
	"errors"
	"net/http"

	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/plan"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/publish"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AppActionPlanHandler serves the T21 (#51) multi-action Action Plan
// endpoints under /api/v1/apps/action-plans: formation (server-derived
// per-item snapshots through the #48 publish seam), whole-plan approval
// with 排除单项, ordered partial-success execution and the per-item
// result projection. The handler is nil-service fail-closed like its
// siblings.
type AppActionPlanHandler struct {
	db    *gorm.DB
	plans *plan.Service
}

// NewAppActionPlanHandler constructs the handler over the business DB.
func NewAppActionPlanHandler(db *gorm.DB) *AppActionPlanHandler {
	return &AppActionPlanHandler{db: db}
}

// SetActionPlanService wires the plan service (container injection
// point). Until called every endpoint fails closed with 501
// ACTION_PLAN_PIPELINE_NOT_CONFIGURED.
func (h *AppActionPlanHandler) SetActionPlanService(s *plan.Service) { h.plans = s }

// RequireActionCapabilityForWrites mirrors the action write gate
// (CanDriveActionWrites): plan formation, approval and execution are
// action writes.
func (h *AppActionPlanHandler) RequireActionCapabilityForWrites() gin.HandlerFunc {
	return appRequireWriteCapability(appconnector.CanDriveActionWrites,
		"FORBIDDEN_ACTION_WRITE",
		"plan writes require owner or admin role")
}

type actionPlanItemInput struct {
	ConnectionID      string `json:"connection_id"`
	SessionID         string `json:"session_id"`
	ArtifactVersionID string `json:"artifact_version_id"`
	Title             string `json:"title"`
	ParentPageID      string `json:"parent_page_id"`
	PageID            string `json:"page_id"`
}

type actionPlanFormInput struct {
	Items []actionPlanItemInput `json:"items"`
}

type actionPlanApproveInput struct {
	Digest      string `json:"digest"`
	ExcludeSeqs []int  `json:"exclude_seqs"`
}

type actionPlanExecuteInput struct {
	Digest string `json:"digest"`
}

// planByID resolves a plan inside the tenant — a cross-tenant id and a
// missing one are both 404, never a 403 that leaks existence.
func (h *AppActionPlanHandler) planByID(c *gin.Context, tenantID uint64, id string) (repoappconn.ActionPlanRow, bool) {
	var row repoappconn.ActionPlanRow
	if err := h.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error; err != nil {
		appFail(c, http.StatusNotFound, "ACTION_PLAN_NOT_FOUND", "action plan not found")
		return row, false
	}
	return row, true
}

// FormActionPlan POST /apps/action-plans — forms the multi-action plan
// through the #48 per-item formation; the response carries the plan
// digest an approval binds.
func (h *AppActionPlanHandler) FormActionPlan(c *gin.Context) {
	tenantID, _, userID, ok := appTenantScope(c)
	if !ok {
		return
	}
	var input actionPlanFormInput
	if err := c.ShouldBindJSON(&input); err != nil || len(input.Items) == 0 {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "at least one plan item is required")
		return
	}
	if h.plans == nil {
		appFail(c, http.StatusNotImplemented, "ACTION_PLAN_PIPELINE_NOT_CONFIGURED",
			"the action plan pipeline is not wired in this environment; refusing to fabricate a plan")
		return
	}
	items := make([]plan.ItemInput, 0, len(input.Items))
	for _, it := range input.Items {
		items = append(items, plan.ItemInput{
			ConnectionID: it.ConnectionID, SessionID: it.SessionID,
			ArtifactVersionID: it.ArtifactVersionID, Title: it.Title,
			ParentPageID: it.ParentPageID, PageID: it.PageID,
		})
	}
	view, err := h.plans.FormPlan(c.Request.Context(), plan.FormInput{
		TenantID: tenantID, ActorID: userID, Items: items})
	if err != nil {
		h.failForm(c, err)
		return
	}
	appOK(c, http.StatusCreated, view)
}

// ApproveActionPlan POST /apps/action-plans/:id/approve — the owner's
// whole-plan decision: approve everything or exclude individual items
// (排除单项). Approval authority: the plan's initiator or a tenant
// owner/admin — the initiator necessarily owns every personal connection
// the plan uses (the formation connection check enforces it,
// app_connector_notion_publish.go:77), and sharing a task never
// delegates approval of its owner's side effects (CONTEXT.md 任务协作者).
func (h *AppActionPlanHandler) ApproveActionPlan(c *gin.Context) {
	tenantID, role, userID, ok := appTenantScope(c)
	if !ok {
		return
	}
	row, ok := h.planByID(c, tenantID, c.Param("id"))
	if !ok {
		return
	}
	var input actionPlanApproveInput
	if err := c.ShouldBindJSON(&input); err != nil || input.Digest == "" {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "digest is required")
		return
	}
	if row.ActorID != userID && !appconnector.CanDriveActionWrites(role) {
		appFail(c, http.StatusForbidden, "ACTION_APPROVAL_FORBIDDEN",
			"approving this plan requires its initiator or tenant owner/admin")
		return
	}
	if h.plans == nil {
		appFail(c, http.StatusNotImplemented, "ACTION_PLAN_PIPELINE_NOT_CONFIGURED",
			"the action plan pipeline is not wired in this environment; refusing to fabricate an approval")
		return
	}
	view, err := h.plans.Approve(c.Request.Context(), tenantID, row.ID, userID,
		plan.ApproveInput{Digest: input.Digest, ExcludeSeqs: input.ExcludeSeqs})
	if err != nil {
		h.failPlan(c, err)
		return
	}
	appOK(c, http.StatusOK, view)
}

// ExecuteActionPlan POST /apps/action-plans/:id/execute — one ordered
// pass over the approved plan; the presented digest must match the plan
// content (AC1 at execute time). Per-item outcomes ride the 200 payload.
func (h *AppActionPlanHandler) ExecuteActionPlan(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	row, ok := h.planByID(c, tenantID, c.Param("id"))
	if !ok {
		return
	}
	var input actionPlanExecuteInput
	if err := c.ShouldBindJSON(&input); err != nil || input.Digest == "" {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "digest is required")
		return
	}
	if h.plans == nil {
		appFail(c, http.StatusNotImplemented, "ACTION_PLAN_PIPELINE_NOT_CONFIGURED",
			"the action plan pipeline is not wired in this environment; refusing to dispatch")
		return
	}
	out, err := h.plans.Execute(c.Request.Context(), tenantID, row.ID, input.Digest)
	if err != nil {
		h.failPlan(c, err)
		return
	}
	appOK(c, http.StatusOK, out)
}

// GetActionPlan GET /apps/action-plans/:id — the durable plan projection
// with per-item authoritative states and receipts.
func (h *AppActionPlanHandler) GetActionPlan(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	row, ok := h.planByID(c, tenantID, c.Param("id"))
	if !ok {
		return
	}
	if h.plans == nil {
		appFail(c, http.StatusNotImplemented, "ACTION_PLAN_PIPELINE_NOT_CONFIGURED",
			"the action plan pipeline is not wired in this environment")
		return
	}
	st, err := h.plans.Status(c.Request.Context(), tenantID, row.ID)
	if err != nil {
		h.failPlan(c, err)
		return
	}
	appOK(c, http.StatusOK, st)
}

// failForm maps one plan-formation refusal onto the fixed code table —
// the per-item publish sentinels keep their #48 wire meanings.
func (h *AppActionPlanHandler) failForm(c *gin.Context, err error) {
	switch {
	case errors.Is(err, plan.ErrPlanInvalidInput):
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid action plan input")
	case errors.Is(err, publish.ErrPublishInvalidInput):
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid publish plan input in one of the items")
	case errors.Is(err, publish.ErrPublishArtifactNotReady):
		appFail(c, http.StatusNotFound, "ARTIFACT_VERSION_NOT_ACCESSIBLE", "an artifact version is not readable for this session")
	case errors.Is(err, publish.ErrPublishUnsupportedArtifact):
		appFail(c, http.StatusUnsupportedMediaType, "PUBLISH_UNSUPPORTED_ARTIFACT", "only text artifacts are publishable in this version")
	case errors.Is(err, publish.ErrPublishContentTooLarge):
		appFail(c, http.StatusRequestEntityTooLarge, "PUBLISH_CONTENT_TOO_LARGE", "an item's artifact exceeds the publish bounds")
	case errors.Is(err, publish.ErrPublishEmptyContent):
		appFail(c, http.StatusBadRequest, "PUBLISH_EMPTY_CONTENT", "an item's artifact carries no publishable text")
	case errors.Is(err, publish.ErrPublishDestinationOutOfScope):
		appFail(c, http.StatusForbidden, "PUBLISH_DESTINATION_OUT_OF_SCOPE", "a destination is not in the reviewed scope for its connection")
	case errors.Is(err, publish.ErrPublishDestinationUnreadable):
		appFail(c, http.StatusBadGateway, "PUBLISH_DESTINATION_UNREADABLE", "an external destination could not be read; no plan was formed")
	case errors.Is(err, publish.ErrPublishUpdateTargetNotPublished):
		appFail(c, http.StatusConflict, "PUBLISH_UPDATE_TARGET_NOT_PUBLISHED", "only pages published through this connection may be updated")
	default:
		appFail(c, http.StatusInternalServerError, "ACTION_PLAN_FORM_FAILED", "failed to form the action plan")
	}
}

// failPlan maps one approve/execute/read refusal onto the fixed code
// table. Messages are static on purpose: no upstream error text or
// provider detail ever crosses the wire.
func (h *AppActionPlanHandler) failPlan(c *gin.Context, err error) {
	switch {
	case errors.Is(err, plan.ErrPlanInvalidInput):
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid action plan input")
	case errors.Is(err, repoappconn.ErrPlanNotFound):
		appFail(c, http.StatusNotFound, "ACTION_PLAN_NOT_FOUND", "action plan not found")
	case errors.Is(err, plan.ErrPlanDigestMismatch):
		// AC1 on the wire: an approval/request issued for different plan
		// content is a conflict with its OWN code, never a generic 500.
		appFail(c, http.StatusConflict, "ACTION_PLAN_DIGEST_MISMATCH",
			"the digest was issued for different plan content; form and approve the plan again")
	case errors.Is(err, plan.ErrPlanState):
		appFail(c, http.StatusConflict, "ACTION_PLAN_STATE_CONFLICT", "action plan is not in the state this operation requires")
	case errors.Is(err, appconnectorsvc.ErrActionDigestMismatch):
		appFail(c, http.StatusConflict, "ACTION_DIGEST_MISMATCH", "an item's approval was issued for different content")
	case errors.Is(err, appconnectorsvc.ErrActionState):
		appFail(c, http.StatusConflict, "ACTION_STATE_CONFLICT", "an item is not in the state this operation requires")
	case errors.Is(err, appconnectorsvc.ErrNoDispatcher):
		appFail(c, http.StatusServiceUnavailable, "OC_DISPATCH_NOT_CONFIGURED",
			"no outbound dispatcher is configured in this deployment; refusing to fabricate a dispatch")
	default:
		appFail(c, http.StatusInternalServerError, "ACTION_PLAN_EXECUTE_FAILED", "action plan operation failed")
	}
}
```

- [ ] **Step 4: 实现路由并运行确认通过**

`internal/router/routes_app_action_plan.go`：

```go
package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterAppActionPlanRoutes registers the T21 (#51) multi-action
// Action Plan surface under /api/v1/apps/action-plans. Like the
// notion-publish routes, these are intentionally NOT declared in the
// API-key route authorizer: the /api/v1 gate default-denies every
// X-API-Key principal. The group carries the action write gate; the
// approval authority predicate lives in the handler (initiator or
// tenant owner/admin).
func RegisterAppActionPlanRoutes(r *gin.RouterGroup, h *handler.AppActionPlanHandler) {
	if h == nil {
		return
	}
	g := r.Group("/apps/action-plans", h.RequireActionCapabilityForWrites())
	{
		g.POST("", h.FormActionPlan)
		g.POST("/:id/approve", h.ApproveActionPlan)
		g.POST("/:id/execute", h.ExecuteActionPlan)
		g.GET("/:id", h.GetActionPlan)
	}
}
```

新增路由自身的测试文件（router 包函数只能在 router 包内测——handler 包反向 import router 会构成循环依赖）：

`internal/router/routes_app_action_plan_test.go`：

```go
package router

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// TestActionPlanRoutesNilHandlerRegistersSilently: the router guard —
// nil handler registers nothing and panics nowhere (mirror of #48).
func TestActionPlanRoutesNilHandlerRegistersSilently(t *testing.T) {
	gin.SetMode(gin.TestMode)
	RegisterAppActionPlanRoutes(gin.New().Group("/api/v1"), nil)
}

// TestActionPlanRoutesRegisterWithHandler: the four endpoints exist on
// the engine after registration (route-existence smoke).
func TestActionPlanRoutesRegisterWithHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := handler.NewAppActionPlanHandler(nil) // constructor only stores the field
	r := gin.New()
	RegisterAppActionPlanRoutes(r.Group("/api/v1"), h)
	found := map[string]bool{}
	for _, ri := range r.Routes() {
		found[ri.Method+" "+ri.Path] = true
	}
	for _, want := range []string{
		"POST /api/v1/apps/action-plans",
		"POST /api/v1/apps/action-plans/:id/approve",
		"POST /api/v1/apps/action-plans/:id/execute",
		"GET /api/v1/apps/action-plans/:id",
	} {
		if !found[want] {
			t.Fatalf("route missing: %s (registered: %v)", want, found)
		}
	}
}
```

Run: `go test ./internal/handler/ -run TestActionPlan -count=1 && go test ./internal/router/ -run TestActionPlanRoutes -count=1`
Expected: PASS（handler 侧 2 个 + router 侧 2 个）

- [ ] **Step 5: 接线 router 与 container**

`internal/router/router.go`——在 `RouterParams` 的 `AppNotionPublishHandler *handler.AppNotionPublishHandler`（`:145`）之后追加：

```go
	// T21 (#51): the multi-action Action Plan surface (form / approve
	// with exclusions / ordered execute / per-item projection) — its own
	// handler over the same publish seam authority.
	AppActionPlanHandler *handler.AppActionPlanHandler
```

并在 `RegisterAppNotionPublishRoutes(v1, params.AppNotionPublishHandler)`（`:436`）之后追加：

```go
		RegisterAppActionPlanRoutes(v1, params.AppActionPlanHandler)
```

`internal/container/notion_publish.go`——构造器扩为双输出（dig 原生支持多输出；`:1013` 的 `container.Provide(newNotionPublishHandler)` 调用点零改动）。在文件头 import 块加入 `"github.com/Tencent/WeKnora/internal/modules/appconnector/plan"`，并把 `newNotionPublishHandler` 尾部：

```go
	h := handler.NewAppNotionPublishHandler(db)
	h.SetNotionPublishService(svc)
	return h, nil
}
```

改为：

```go
	h := handler.NewAppNotionPublishHandler(db)
	h.SetNotionPublishService(svc)
	// T21 (#51): the plan layer over the SAME publish ActionService —
	// per-item digests/approvals/dispatch claims stay on the A03
	// authority; the plan adds the set digest, exclusions and ordered
	// partial-success execution.
	planSvc := plan.NewService(repoappconn.NewPlanStore(db), store, actions, svc)
	planHandler := handler.NewAppActionPlanHandler(db)
	planHandler.SetActionPlanService(planSvc)
	return h, planHandler, nil
}
```

并把签名 `func newNotionPublishHandler(...) (*handler.AppNotionPublishHandler, error)` 改为 `func newNotionPublishHandler(...) (*handler.AppNotionPublishHandler, *handler.AppActionPlanHandler, error)`。

- [ ] **Step 6: 全量编译 + 回归**

Run: `go build ./... && go test ./internal/handler/ -run 'TestActionPlan' -count=1 && go test ./internal/router/ -run TestActionPlanRoutes -count=1 && go vet ./internal/container/ ./internal/router/ ./internal/handler/`
Expected: 编译通过、handler 2 + router 2 测试 PASS、vet 干净

- [ ] **Step 7: Commit**

```bash
git add internal/handler/app_connector_action_plan.go internal/handler/app_connector_action_plan_test.go internal/router/routes_app_action_plan.go internal/router/routes_app_action_plan_test.go internal/router/router.go internal/container/notion_publish.go
git commit -m "feat(appconnector): action-plans HTTP 面 + 路由 + 容器双输出接线（T21 #51 Task 5）"
```

---

### Task 6: 端到端证据（AC1/AC2/AC3，生产迁移库 + 契约双打 Notion）

**Files:**
- Modify: `internal/handler/app_connector_notion_publish_e2e_test.go`（给 `e2eNotion` 双打加一个**加法式** title 定向丢回复钩子，不影响既有三个测试）
- Create: `internal/handler/app_connector_action_plan_e2e_test.go`

**Interfaces:**
- Consumes: `openNotionPublishE2EDB/newE2ENotion/e2ePolicyProvider/e2ePassGuard/e2eLocalContent`（同包复用）；Task 5 全部 wire 面；既有 `POST /apps/notion-publish/actions/:id/reconcile` 与 `GET /apps/actions/:id`
- Produces: AC1/AC2/排除单项/全链四条 e2e + 迁移↔投影对齐测试（TestAppActionPlansTablesExistAfterMigrations）

- [ ] **Step 1: 给 #48 契约双打加 title 定向丢回复钩子（加法式修改）**

`internal/handler/app_connector_notion_publish_e2e_test.go`——`e2eNotion` 结构体（`:91-99`）追加两个字段：

```go
type e2eNotion struct {
	mu             sync.Mutex
	token          string
	pages          map[string]*e2ePage
	nextID         int
	patchCalls     int
	appendCalls    int
	dropNextAppend bool
	// T21 (#51) additive hook: when a page is CREATED with this title,
	// its first append-children reply is lost AFTER the effect applied
	// (blocks land, the connection dies) — the plan-level unknown leg.
	// The pre-existing dropNextAppend behavior is untouched.
	dropAppendForTitle string
	droppedOnce        map[string]bool
}
```

`newE2ENotion`（`:106`）改为：

```go
func newE2ENotion(token string) *e2eNotion {
	return &e2eNotion{token: token, pages: map[string]*e2ePage{}, droppedOnce: map[string]bool{}}
}
```

`/v1/pages` POST 处理器中，创建页面后（`e.pages[id] = ...` 与 `e.unlock()` 之后、`writeJSON` 之前）插入：

```go
		e.lock()
		if e.dropAppendForTitle != "" && req.Properties.Title.Title[0].Text.Content == e.dropAppendForTitle {
			e.droppedOnce[id] = true
		}
		e.unlock()
```

`/v1/blocks/` PATCH 处理器中，把：

```go
			drop := e.dropNextAppend
			if drop {
				e.dropNextAppend = false
			}
```

改为：

```go
			drop := e.dropNextAppend
			if drop {
				e.dropNextAppend = false
			}
			if e.droppedOnce[id] {
				delete(e.droppedOnce, id)
				drop = true
			}
```

- [ ] **Step 2: 写端到端测试文件**

`internal/handler/app_connector_action_plan_e2e_test.go`：

```go
package handler

// End-to-end evidence for T21 (#51). Everything here runs on a FULLY
// MIGRATED sqlite database (the production migrations/sqlite track) with
// the REAL ActionService / PlanStore / NotionPublishService / NotionBridge
// / handlers and the REAL approval machinery. The ONLY replaced piece is
// the Notion wire endpoint (the #48 contract double) — this is the
// highest stable Interface evidence available without provider
// credentials; it is NOT the real-provider acceptance, which stays
// NOTION_TOKEN-gated in notion_publish_real_test.go (single-action leg)
// and is blocked-env for the plan-level loop in this environment.

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service/file"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/plan"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/publish"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestAppActionPlansTablesExistAfterMigrations: the plan tables must be
// created by the PRODUCTION migrations (columns aligned with the Go rows
// — the workbench-notifications precedent).
func TestAppActionPlansTablesExistAfterMigrations(t *testing.T) {
	db := openNotionPublishE2EDB(t)
	require.True(t, db.Migrator().HasTable("app_action_plans"),
		"app_action_plans must be created by the production migrations")
	for _, column := range []string{"tenant_id", "id", "actor_id", "digest", "state", "excluded_json", "approved_by", "approved_at"} {
		require.True(t, db.Migrator().HasColumn("app_action_plans", column),
			"app_action_plans.%s must exist (aligned with ActionPlanRow)", column)
	}
	require.True(t, db.Migrator().HasTable("app_action_plan_items"),
		"app_action_plan_items must be created by the production migrations")
	for _, column := range []string{"tenant_id", "plan_id", "seq", "action_id"} {
		require.True(t, db.Migrator().HasColumn("app_action_plan_items", column),
			"app_action_plan_items.%s must exist (aligned with ActionPlanItemRow)", column)
	}
}

// ---- the full-stack environment ----

type actionPlanE2EEnv struct {
	engine *gin.Engine
	db     *gorm.DB
	fake   *e2eNotion
}

// newActionPlanE2E assembles the SAME production composition as
// newNotionPublishE2E plus the plan service and routes. The seeding is
// deliberately duplicated (not refactored out of the frozen #48 fixture)
// so this file stays merge-isolated.
func newActionPlanE2E(t *testing.T) *actionPlanE2EEnv {
	t.Helper()
	db := openNotionPublishE2EDB(t)
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (7, 't7', 'test')`).Error)
	for _, u := range []string{"user-a", "user-b"} {
		require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES (?, ?, ?, 'x', 7)`, u, u, u+"@example.test").Error)
		require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id, user_id, role, status) VALUES (7, ?, 'contributor', 'active')`, u).Error)
	}
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('sess-1', 7, 'task-1', 'user-a', 'trpc')`).Error)
	_, err := repository.NewAgentRunStore(db).Admit(context.Background(), agentruntime.Admission{
		Key: agentruntime.RunKey{TenantID: 7, RunID: "run-1"}, SessionID: "sess-1", UserID: "user-a",
		RequestID: "q1", AssistantMessageID: "a1", RequestHash: "hash-1",
		Snapshot: json.RawMessage(`{"version":1}`), UserMessage: json.RawMessage(`{"role":"user","content":"hello"}`),
		AssistantMessage: json.RawMessage(`{"role":"assistant","content":""}`), Deadline: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	baseDir := t.TempDir()
	digest := strings.Repeat("a", 64)
	objectKey := fmt.Sprintf("artifact-versions/7/run-1/%s", digest)
	require.NoError(t, os.MkdirAll(filepath.Join(baseDir, filepath.Dir(objectKey)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, objectKey), []byte("第一段。\n\n第二段。"), 0o644))
	require.NoError(t, db.Exec(`INSERT INTO artifact_versions (tenant_id, id, run_id, session_id, digest, object_key, mime, scan_state, size)
		VALUES (7, 'ver-1', 'run-1', 'sess-1', ?, ?, 'text/plain', 'ready', 15)`, digest, objectKey).Error)
	require.NoError(t, db.Exec(`INSERT INTO installations (id, tenant_id, app_id, app_version, state, version) VALUES ('inst-notion', 7, 'notion', 'v1', 'active', 1)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO app_versions (app_id, version, schema_json, risk_json) VALUES ('notion', 'v1', '{"scopes":["insert_content"],"approved_parents":["parent-1"]}', '{}')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO connections (tenant_id, id, installation_id, kind, owner_id, credential_ref, state, auth_version)
		VALUES (7, 'conn-notion', 'inst-notion', 'personal', 'user-a', 'mcp_oauth_token:notion', 'active', 1)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO mcp_services (id, tenant_id, name, transport_type) VALUES ('notion', 7, 'notion', 'http')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO mcp_oauth_tokens (id, tenant_id, user_id, principal_type, principal_id, service_id, access_token, token_type, expires_at)
		VALUES ('tok-1', 7, 'user-a', 'web_user', 'user-a', 'notion', 'secret_test_token', 'bearer', '2099-01-01 00:00:00')`).Error)
	fake := newE2ENotion("secret_test_token")
	fake.addPage("parent-1", "root", "Workspace")
	srv := fake.server(t)

	pubs := repoappconn.NewPublicationStore(db)
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	_, loopback, _ := net.ParseCIDR("127.0.0.0/8")
	pol := appconn.HTTPPolicy{
		Scheme: "http", Host: host, Port: port,
		Methods: []string{"GET", "POST", "PATCH"}, PathPrefix: "/v1/",
		AuthorizedNetworks: []*net.IPNet{loopback}, Timeout: 10 * time.Second,
	}
	scopeSrc := publish.NewDBNotionScopeSource(db)
	bridge := publish.NewNotionBridge(scopeSrc, &e2ePolicyProvider{pol: pol},
		publish.NewCredentialTokenSource(appconnectorsvc.NewCredentialResolver(repository.NewMCPOAuthBindingStore(db))), pubs)
	store := repoappconn.NewActionStore(db)
	publishActions := appconnectorsvc.NewActionService(store, &e2ePassGuard{}, nil, bridge, bridge)
	content := &e2eLocalContent{svc: file.NewLocalFileService(baseDir, "")}
	publishSvc := publish.NewNotionPublishService(publishActions, store, pubs,
		repository.NewArtifactVersionStore(db), content, bridge, scopeSrc)
	planSvc := plan.NewService(repoappconn.NewPlanStore(db), store, publishActions, publishSvc)

	publishHandler := NewAppNotionPublishHandler(db)
	publishHandler.SetNotionPublishService(publishSvc)
	actionHandler := NewAppActionHandler(db)
	actionHandler.SetActionService(publishActions)
	planHandler := NewAppActionPlanHandler(db)
	planHandler.SetActionPlanService(planSvc)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		user := "user-a"
		if c.GetHeader("X-Test-User") != "" {
			user = c.GetHeader("X-Test-User")
		}
		role := types.TenantRoleAdmin
		if c.GetHeader("X-Test-Role") != "" {
			role = types.TenantRole(c.GetHeader("X-Test-Role"))
		}
		ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(7))
		ctx = context.WithValue(ctx, types.UserIDContextKey, user)
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	v1 := engine.Group("/api/v1")
	v1.GET("/apps/actions/:id", actionHandler.GetAction)
	gp := v1.Group("/apps/notion-publish", publishHandler.RequireActionCapabilityForWrites())
	{
		gp.POST("/actions/:id/reconcile", publishHandler.ReconcileNotionAction)
		gp.GET("/actions/:id", publishHandler.GetNotionPublication)
	}
	g := v1.Group("/apps/action-plans", planHandler.RequireActionCapabilityForWrites())
	{
		g.POST("", planHandler.FormActionPlan)
		g.POST("/:id/approve", planHandler.ApproveActionPlan)
		g.POST("/:id/execute", planHandler.ExecuteActionPlan)
		g.GET("/:id", planHandler.GetActionPlan)
	}
	return &actionPlanE2EEnv{engine: engine, db: db, fake: fake}
}

func (e *actionPlanE2EEnv) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader = strings.NewReader("")
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

func planCreateItem(title string) string {
	return fmt.Sprintf(`{"connection_id":"conn-notion","session_id":"sess-1","artifact_version_id":"ver-1","title":%q,"parent_page_id":"parent-1"}`, title)
}

func planUpdateItem(title, pageID string) string {
	return fmt.Sprintf(`{"connection_id":"conn-notion","session_id":"sess-1","artifact_version_id":"ver-1","title":%q,"page_id":%q}`, title, pageID)
}

type formedPlan struct {
	id, digest string
}

func (e *actionPlanE2EEnv) formPlanItems(t *testing.T, items ...string) formedPlan {
	t.Helper()
	w := e.do(t, http.MethodPost, "/api/v1/apps/action-plans", `{"items":[`+strings.Join(items, ",")+`]}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var parsed struct {
		Data struct {
			ID     string `json:"id"`
			Digest string `json:"digest"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &parsed))
	return formedPlan{id: parsed.Data.ID, digest: parsed.Data.Digest}
}

func (e *actionPlanE2EEnv) formPlan(t *testing.T, titles ...string) formedPlan {
	t.Helper()
	items := make([]string, 0, len(titles))
	for _, title := range titles {
		items = append(items, planCreateItem(title))
	}
	return e.formPlanItems(t, items...)
}

func (e *actionPlanE2EEnv) approve(t *testing.T, p formedPlan) {
	t.Helper()
	w := e.do(t, http.MethodPost, "/api/v1/apps/action-plans/"+p.id+"/approve",
		fmt.Sprintf(`{"digest":%q}`, p.digest))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func (e *actionPlanE2EEnv) approveExcluding(t *testing.T, p formedPlan, seqs []int) {
	t.Helper()
	raw, _ := json.Marshal(seqs)
	w := e.do(t, http.MethodPost, "/api/v1/apps/action-plans/"+p.id+"/approve",
		fmt.Sprintf(`{"digest":%q,"exclude_seqs":%s}`, p.digest, raw))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

type e2eItemOutcome struct {
	Seq         int    `json:"seq"`
	ActionID    string `json:"action_id"`
	Disposition string `json:"disposition"`
	ActionState string `json:"action_state"`
	Conflict    bool   `json:"conflict"`
	Publication struct {
		State           string `json:"state"`
		ExternalID      string `json:"external_id"`
		ExternalVersion string `json:"external_version"`
	} `json:"publication"`
}

func (e *actionPlanE2EEnv) execute(t *testing.T, p formedPlan) []e2eItemOutcome {
	t.Helper()
	w := e.do(t, http.MethodPost, "/api/v1/apps/action-plans/"+p.id+"/execute",
		fmt.Sprintf(`{"digest":%q}`, p.digest))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out struct {
		Data struct {
			Items []e2eItemOutcome `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return out.Data.Items
}

// publishOne creates exactly one page through a ONE-item plan and
// returns its external id — a receipt-backed update target minted
// through the plan seam itself.
func (e *actionPlanE2EEnv) publishOne(t *testing.T, title string) string {
	t.Helper()
	p := e.formPlan(t, title)
	e.approve(t, p)
	items := e.execute(t, p)
	require.Len(t, items, 1)
	require.Equal(t, "succeeded", items[0].ActionState, fmt.Sprintf("%+v", items))
	require.NotEmpty(t, items[0].Publication.ExternalID)
	return items[0].Publication.ExternalID
}

// TestActionPlanEndToEndApproveExecutePerItemResults: the whole-plan
// chain — form a THREE-item plan through the real publish seam, approve
// the WHOLE plan once, execute, and read per-item receipts. Each item is
// an independent page creation and records its own result.
func TestActionPlanEndToEndApproveExecutePerItemResults(t *testing.T) {
	env := newActionPlanE2E(t)
	p := env.formPlan(t, "Report A", "Report B", "Report C")

	w := env.do(t, http.MethodPost, "/api/v1/apps/action-plans/"+p.id+"/approve",
		fmt.Sprintf(`{"digest":%q}`, p.digest))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	out := env.execute(t, p)
	require.Len(t, out, 3)
	for i, it := range out {
		require.Equal(t, i+1, it.Seq)
		require.Equal(t, "executed", it.Disposition, "item %d: %+v", i+1, it)
		require.Equal(t, "succeeded", it.ActionState)
		require.Equal(t, "published", it.Publication.State)
		require.NotEmpty(t, it.Publication.ExternalID)
		require.NotEmpty(t, it.Publication.ExternalVersion, "each receipt saves the external version")
	}
	// Three REAL pages exist remotely with the derived blocks.
	env.fake.lock()
	require.Len(t, env.fake.pages, 4, "parent + three created pages")
	env.fake.unlock()
	// The durable projection agrees.
	w = env.do(t, http.MethodGet, "/api/v1/apps/action-plans/"+p.id, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"action_state":"succeeded"`)
	require.Contains(t, w.Body.String(), `"state":"published"`)
}

// TestActionPlanEndToEndContentChangeInvalidatesOldApproval (AC1): a
// plan whose content changed (different items → different digest) can
// never be approved or executed with the OLD plan's digest — the old
// approval authorizes only its own frozen content, and every refusal
// happens BEFORE any dispatch.
func TestActionPlanEndToEndContentChangeInvalidatesOldApproval(t *testing.T) {
	env := newActionPlanE2E(t)
	oldPlan := env.formPlan(t, "Report A", "Report B")
	newPlan := env.formPlan(t, "Report A v2", "Report B v2")
	require.NotEqual(t, oldPlan.digest, newPlan.digest, "计划内容变化必须产生新 plan digest")

	w := env.do(t, http.MethodPost, "/api/v1/apps/action-plans/"+newPlan.id+"/approve",
		fmt.Sprintf(`{"digest":%q}`, oldPlan.digest))
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "ACTION_PLAN_DIGEST_MISMATCH")

	w = env.do(t, http.MethodPost, "/api/v1/apps/action-plans/"+newPlan.id+"/execute",
		fmt.Sprintf(`{"digest":%q}`, oldPlan.digest))
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "ACTION_PLAN_DIGEST_MISMATCH")

	// The new content can never ride the old plan's approval either:
	// approve the OLD plan, then present ITS digest against the NEW
	// plan — the same refusal.
	env.approve(t, oldPlan)
	w = env.do(t, http.MethodPost, "/api/v1/apps/action-plans/"+newPlan.id+"/execute",
		fmt.Sprintf(`{"digest":%q}`, oldPlan.digest))
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "ACTION_PLAN_DIGEST_MISMATCH")

	// Zero dispatches happened anywhere (fail closed before any send).
	env.fake.lock()
	require.Len(t, env.fake.pages, 1, "parent only — no page was ever created")
	env.fake.unlock()
}

// TestActionPlanEndToEndPartialSuccessResumesUnfinishedOnly (AC2): a
// three-item ordered plan [create-ok, update-stale, create-unknown]:
// pass 1 records executed/succeeded + executed/failed(version conflict,
// zero writes) + executed/unknown (blocks landed, reply lost); pass 2
// (resume, same digest) re-dispatches NOTHING; the unknown item resolves
// through the provider query; pass 3 reads skipped_succeeded for both
// recovered items. External write counts prove the recovery never
// repeats a confirmed action.
func TestActionPlanEndToEndPartialSuccessResumesUnfinishedOnly(t *testing.T) {
	env := newActionPlanE2E(t)
	seed := env.publishOne(t, "Seed")
	env.fake.lock()
	appendsAfterSeed := env.fake.appendCalls
	env.fake.unlock()

	// The plan's update item binds the CURRENT external version at
	// formation; the collaborator edit comes AFTER approval.
	items := []string{planCreateItem("Doc A"), planUpdateItem("Seed v2", seed), planCreateItem("Doc Ghost")}
	p := env.formPlanItems(t, items...)
	env.fake.dropAppendForTitle = "Doc Ghost"
	env.approve(t, p)
	env.fake.touch(seed, "2026-09-24T10:30:00.000Z")

	// Pass 1: partial success — succeeded + failed(conflict) + unknown.
	out := env.execute(t, p)
	require.Len(t, out, 3)
	require.Equal(t, "executed", out[0].Disposition)
	require.Equal(t, "succeeded", out[0].ActionState)
	require.Equal(t, "executed", out[1].Disposition)
	require.Equal(t, "failed", out[1].ActionState)
	require.True(t, out[1].Conflict, "the stale update must be a definitive version conflict: %+v", out[1])
	require.Equal(t, "executed", out[2].Disposition)
	require.Equal(t, "unknown", out[2].ActionState)

	env.fake.lock()
	pagesAfterPass1 := len(env.fake.pages)    // parent + Seed + Doc A + Doc Ghost
	appendsAfterPass1 := env.fake.appendCalls // Seed + Doc A + Doc Ghost(applied)
	patchesAfterPass1 := env.fake.patchCalls  // 0: the conflict refused BEFORE any write
	env.fake.unlock()
	require.Equal(t, 4, pagesAfterPass1, "Seed, Doc A and Doc Ghost must all exist")
	require.Equal(t, 0, patchesAfterPass1, "the conflicted update must write NOTHING")

	// Pass 2 (resume, same digest): confirmed outcomes never re-sent.
	out2 := env.execute(t, p)
	require.Len(t, out2, 3)
	require.Equal(t, "skipped_succeeded", out2[0].Disposition)
	require.Equal(t, "settled", out2[1].Disposition)
	require.Equal(t, "failed", out2[1].ActionState)
	require.Equal(t, "settled", out2[2].Disposition)
	require.Equal(t, "unknown", out2[2].ActionState)
	env.fake.lock()
	require.Equal(t, appendsAfterPass1, env.fake.appendCalls, "AC2: resume re-dispatched nothing")
	require.Equal(t, pagesAfterPass1, len(env.fake.pages), "AC2: no duplicate page")
	env.fake.unlock()

	// Reconcile the unknown item through the EXISTING per-action endpoint
	// (provider query first): the effect HAD applied remotely → succeeded.
	w := env.do(t, http.MethodPost, "/api/v1/apps/notion-publish/actions/"+out[2].ActionID+"/reconcile", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"action_state":"succeeded"`)
	require.Contains(t, w.Body.String(), `"state":"published"`)

	// Pass 3: both recovered legs read as confirmed outcomes.
	out3 := env.execute(t, p)
	require.Len(t, out3, 3)
	require.Equal(t, "skipped_succeeded", out3[0].Disposition)
	require.Equal(t, "settled", out3[1].Disposition)
	require.Equal(t, "skipped_succeeded", out3[2].Disposition)
	env.fake.lock()
	require.Equal(t, appendsAfterPass1, env.fake.appendCalls, "reconcile + resume never re-send blocks")
	require.Equal(t, 0, env.fake.patchCalls, "the failed conflict update is NEVER re-dispatched")
	env.fake.unlock()
}

// TestActionPlanEndToEndExcludeItem (Owner 排除单项): the approval
// excludes item 2; execution runs 1 and 3; the excluded item's action
// stays awaiting_approval — queryable through the frozen single-action
// GET, never dispatched by the plan.
func TestActionPlanEndToEndExcludeItem(t *testing.T) {
	env := newActionPlanE2E(t)
	p := env.formPlan(t, "Doc A", "Doc B", "Doc C")
	env.approveExcluding(t, p, []int{2})
	out := env.execute(t, p)
	require.Len(t, out, 3)
	require.Equal(t, "executed", out[0].Disposition)
	require.Equal(t, "excluded", out[1].Disposition)
	require.Equal(t, "executed", out[2].Disposition)
	// The excluded action stayed untouched.
	w := env.do(t, http.MethodGet, "/api/v1/apps/actions/"+out[1].ActionID, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"state":"awaiting_approval"`)
	env.fake.lock()
	require.Len(t, env.fake.pages, 3, "parent + two created pages — the excluded item never dispatched")
	env.fake.unlock()
	// The plan GET projects the exclusion.
	w = env.do(t, http.MethodGet, "/api/v1/apps/action-plans/"+p.id, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"excluded":[2]`)
}
```

- [ ] **Step 3: 首跑并如实记录**

Run: `go test ./internal/handler/ -run 'TestActionPlanEndToEnd|TestAppActionPlansTables' -count=1`
Expected: Task 0/1 已合入时（迁移可装载且新表已建）全部应直接 PASS——本任务是前五个任务的红绿证据收口，不是新的 RED；任何 FAIL 先定位夹具装配或前置任务缺陷，修复后重跑，如实记录首跑输出

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/handler/ -run 'TestActionPlan|TestNotionPublish|TestAppPublications' -count=1`
Expected: 全部 PASS（本计划 4+3 条 e2e + #48 既有 3 条 e2e + publications 对齐测试——#48 既有测试必须零回归）

- [ ] **Step 5: 计划级全量验证**

Run: `go build ./... && go test ./internal/database/ -count=1 && go test ./internal/modules/appconnector/... -count=1 && go test ./internal/handler/ -run 'TestActionPlan|TestNotionPublish|TestAppPublications' -count=1`
Expected: 全绿

- [ ] **Step 6: Commit**

```bash
git add internal/handler/app_connector_notion_publish_e2e_test.go internal/handler/app_connector_action_plan_e2e_test.go
git commit -m "test(appconnector): 多操作计划 AC1/AC2/排除单项 端到端证据（T21 #51 Task 6）"
```

---

## 计划级验证命令（testCommand）

在 worktree 根 `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep` 执行：

```bash
go build ./... && go test ./internal/database/ -count=1 && go test ./internal/modules/appconnector/... -count=1 && go test ./internal/handler/ -run 'TestActionPlan|TestAppActionPlans|TestNotionPublish|TestAppPublications' -count=1 && go test ./internal/router/ -run TestActionPlanRoutes -count=1
```

覆盖：迁移轨道装载（database 包）+ plan/repository/publish/service 全部单测（appconnector 树 7 包）+ handler 层全部本计划测试与 #48 零回归 + router 层路由守卫/存在性。不触碰全量 flaky 套件。
（终审 Finding 1 修订：handler 段正则原为 `'TestActionPlan|TestNotionPublish|TestAppPublications'`，实测不匹配迁移对齐测试 `TestAppActionPlansTablesExistAfterMigrations`——该测试此前仅由 `go test ./internal/modules/appconnector/...` 与 Task 6 Step 3 的 `'TestActionPlanEndToEnd|TestAppActionPlansTables'` 覆盖，计划级门控未直跑；现补 `TestAppActionPlans` 分支使其纳入计划级门控。Task 6 Step 4/5 的历史命令保持原样，其实跑证据已入册。）

## 交付边界（如实声明）

- **blocked-env**：真实 Notion 多操作计划验收（`NOTION_TOKEN` + 真实父页面）本环境不可运行——单操作层的 `TestNotionRealPublishLoop`（NOTION_TOKEN 门控）继续承担真实凭据证据，本地 SKIP 不是 pass；计划级真实 Provider 循环留待有凭据环境执行，不得伪造。
- **不做**：移动端 UI 消费面（TS 侧当前对 notion-publish 零消费，作者已核实；后续由 Task Office 面板接入）；代码交付（#52 已有独立 `codedelivery` plan 域，与本计划的多操作 Action Plan 无关不合并）；计划项间依赖模型（一项失败阻断后续——本裁定为逐项独立记录，见「执行语义裁定」）；计划删除/取消端点（YAGNI，等下游 #71 需求落地）。
- **并行批次避让**：共享文件改动仅四处且位置精确——`internal/database/migration.go`（两处引用）、`internal/router/router.go`（两处插入）、`internal/container/notion_publish.go`（构造器尾部）、`internal/handler/app_connector_notion_publish_e2e_test.go`（双打钩子）；其余全部为新文件。全部迁移编号（Task 0 的 marketplace 重编号与 Task 1 的新表号）以 Task 0 Step 0 的执行时刻核查为准，被占即整组顺延（DDL 零变化）。
