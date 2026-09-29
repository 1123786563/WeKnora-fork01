# T09：Task Budget 达限、扩额与恢复（Issue #39）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让一个 Task 的预算（预计/已用/预占/剩余，含委派 Run）可读、达限后运行**持久暂停**（`waiting_user`/`budget_exhausted`，而非终态 failed），Owner/账单管理员授权扩额后**同一 Run 恢复执行**（不重复历史消耗、预占或在途费用），且「Collaborator 不能扩额、权限由服务端强制」与端到端行为全部在最高稳定 Interface（真实 HTTP + 真实 sqlite store/worker + 移动端授权通道）上可验证。

**Architecture:** 服务端三处最小增量：①worker 的 `durableWaitReason` 增加 budget 达限 wait 类（与 `durableRunFailureEvent` 同一错误集，`agent_run_graph.go:496-501` 已先行落盘 `budget_exhausted` 持久事件），达限由终态 failed 改为 `waiting_user`/`budget_exhausted` 持久暂停；②仓储新增 `AgentRunStore.RequeueBudgetPausedRuns`（同一预算根下 parked 行 `waiting_user→queued` 的受护栏 UPDATE，镜像 `ApplyDecision` 的恢复先例 `agent_run_decisions.go:255-271`），挂在既有扩额端点成功路径上（幂等重放同样 requeue，治愈竞态）；③新增只读端点 `GET /api/v1/commercial/tasks/:id/budget`（四数字来自 `commercial_task_budgets` 根行——子 Run 预占天然计入根行，`budget_reservation.go:89-96`；读门 = 账单权威 OR 任务 Owner OR #42 task grant 持有者，`can_extend` 只对前两者为 true）。客户端按 module-seams §5（「预算扩展」归 Task Office）新建 `packages/mobile-core/src/task-office/task-budget.ts` 深模块（scope 守卫 + 每逻辑扩额一个幂等键、失败保留、成功清除——对齐 Web `TaskBudget.tsx:38-40` 纪律），经 `TaskOfficePorts.budget?` 以两个方法最小接入 `TaskOffice`；api-client 新增 `createMobileTaskBudgetRemote`（授权通道，无新传输）；apps/mobile 新增预算屏（四数字 + 委派 + 暂停横幅 + 独立扩额操作，`can_extend=false` 时禁用）与 opt-in 真实 HTTP 集成证据；Web `TaskBudget.tsx` 补派生「剩余」行。

**Tech Stack:** Go 1.26（gin + gorm + testify，`go test`）、TypeScript（`packages/contracts`、`packages/api-client`、`packages/mobile-core`、`apps/mobile` Expo RN、`apps/web` React）、node:test + tsx（TS 测试运行器）。所有命令在 worktree 根（`.worktrees/issue30-sweep`）执行；前置 `pnpm install` 已就绪。本计划作者在当前 HEAD 实跑基线：`go test ./internal/handler/ -run 'TestExtendTaskBudget' -count=1` ok（1.848s）；`go test ./internal/modules/commercial/service/commercial/ ./internal/modules/commercial/repository/commercial/ -count=1` ok（3.981s/2.571s）；`pnpm exec tsx --test packages/mobile-core/src/task-office/task-office.test.ts` 8 pass 0 fail；`pnpm exec tsx --test packages/api-client/src/mobile/task-office.test.ts` 13 pass 0 fail；`pnpm --filter @weknora/mobile test` 154 tests / 149 pass / 0 fail（5 skip 为既有 opt-in）。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-39.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（User Stories 57/58/59/60、Implementation Decisions、Testing Decisions——尤其「Task Office owns Home/Task projections, durable submission identity, reconciliation, Snapshot/SSE recovery, intervention, decisions, budget and Task lifecycle.」）
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§5 Task Office Module——「运行干预、Interaction decision、预算扩展」归 Task Office 所有；§3 依赖方向；§10 App Shell 禁止事项；§13 Interface 测试面）
- 领域术语：`CONTEXT.md:274-275`「任务预算（Task Budget）：空间授权一个任务及其委派执行合计可消耗的 Credits 上限；达到上限后运行持久暂停，只有任务所有者或获授权的账单管理员可以增加上限。_避免_：整个空间的余额、每个子任务各自获得一份完整额度、任务协作者可自行提高的预算。」；`CONTEXT.md:87`（重要预算事件的行动通知）；「额度预占」「结算待核对」条目
- ADR：`docs/adr/0004-task-is-session.md`（Owner 权威 = `sessions.user_id`，扩额门禁 B3-F83 同源）、`docs/adr/0006-mobile-transport-by-semantics.md`（授权 REST 读、无新增推送通道）、`docs/adr/0012-mobile-business-logic-lives-behind-deep-modules.md`
- Parent：Issue #30；Blocked by：#36、#38（均已合并——`TaskOffice.start`/`inbox()/decide()` 在当前 HEAD 亲眼核实）
- 前序批次产出（本计划 Consumes，全部在当前 HEAD 亲眼核实）：#34 的 `MobileRuntime.authorizedRequest` 授权读通道与 `TaskOfficePorts`（`packages/mobile-core/src/runtime/types.ts`、`task-office.ts:167-186`）；#35 的 `TaskDetailScreen` 与 `/tasks/detail` 路由（`apps/mobile/src/screens/TaskDetailScreen.tsx`、`apps/mobile/src/app/tasks/detail.tsx`）；#32 的 `RuntimeScopeLease`/`leaseActive`（`packages/mobile-core/src/runtime/scope-lease.ts`，包内可见）；#36 的 `TaskOfficePorts.newRequestId?`（缺省 `globalThis.crypto.randomUUID`，`task-office.ts:290-295`）；#42 的扩额 Owner-or-billing 门禁与 `taskRunOwner`（`internal/handler/commercial_task_budget.go:53-70,99-121`）、task_grants 授予读谓词（`internal/application/repository/agent_run.go:123-147` `GetRunForGrantedReader` 的 EXISTS SQL 形态）、`NewAgentRunStore(db *gorm.DB)` 单参构造器（`agent_run.go:29`）；#33/#41/#46 的深模块 + 场景 Adapter + 记忆化 composition 工厂 + opt-in 集成证据范式（`device-registry.ts`、`material/task-material.ts`、`composition.ts:266-279`、`material-integration-smoke.ts`）

## Global Constraints

以下为批准 Spec / ADR / Issue 的项目级约束，逐字引用，所有任务隐含遵守：

- 「As a Task Owner, I want every Task to have a cumulative budget including delegated work, so that subagents and retries cannot multiply costs invisibly.」（mobile-ai-office-design.md · Story 57）——四数字读模型必须来自**根预算行**（委派子 Run 的预占在 `Reserve` 内解析到根行，`budget_reservation.go:89-96`），不得按子 Run 各自累计。
- 「As a Task Owner, I want estimated, used, reserved and remaining cost shown distinctly, so that I understand budget state.」（同上 · Story 58）——预计=批准上限（`limit_credits`）、已用（`used_credits`）、预占（`held_credits`）、剩余（`remaining_credits = limit - used - held`）四数必须**分立展示**。
- 「As a billing administrator, I want only authorized actors to raise Task Budget, so that Collaborators cannot expand spend.」（同上 · Story 59）
- 「As a member, I want budget exhaustion to pause durably without deleting the Workspace, so that authorized continuation is possible.」（同上 · Story 60）——达限是**持久暂停**：不删 Workspace、不清 checkpoint、run 行保持非终态。
- 「Task Office owns Home/Task projections, durable submission identity, reconciliation, Snapshot/SSE recovery, intervention, decisions, budget and Task lifecycle.」（同上 · Implementation Decisions）+ module-seams §5.1「运行干预、Interaction decision、预算扩展；」——移动端预算扩展经 Task Office Interface（`office.budget()/extendBudget()`），Screen 不直接调 wire。
- 「REST submits commands and loads authoritative Snapshots. Cursored SSE carries durable Task/Run events. WebSocket or WebRTC is reserved for real-time voice. Push is a synchronization hint.」（同上；ADR-0006 同义）——预算读/扩额走授权 REST，**不新增任何 WebSocket**。
- 「Offline mode permits approved reads, drafts and annotations. It prohibits Run commands, approval, budget expansion and external Actions.」（同上）+ Non-goal「Offline Run execution, offline approval, offline budget changes or automatic command replay.」（同上）——扩额必须在线经授权通道；离线时自然失败，不做本地队列/自动重放。
- 「Mobile core does not depend on React Native, DOM or concrete transport. Remote and native details are injected as Adapters.」（同上）
- 「Cache identity includes Deployment, user, Tenant, Task and Run where applicable. Scope changes invalidate subscriptions and reject late responses.」（同上）——scope lease 每次异步提交前检查，迟到结果不落地。
- 「Tests target observable behavior at the highest stable Interface. Screen tests verify rendering and navigation; they do not duplicate Module internals.」（同上 · Testing Decisions）
- 「Screen 不调用 start、lookup、snapshot、events、interaction、command 等多个 wire 方法。Module 内部决定顺序、幂等、revision 和错误呈现。」（mobile-module-seams.md §5.2）+ 「禁止：Screen 直接导入 packages/contracts 或 packages/api-client；Screen 自己维护 request_id、cursor、revision、scope generation；每个 Screen 建独立 query cache 或 token refresh」（§10）
- 「Interface 不暴露 token、query key、generation number 或 SecureStore key。Scope Lease 是不透明、可撤销的能力对象，子 Module 每次异步提交前检查其有效性。」（§4.2）
- 「任务预算（Task Budget）……达到上限后运行持久暂停，只有任务所有者或获授权的账单管理员可以增加上限。_避免_：……任务协作者可自行提高的预算。」（CONTEXT.md:274-275）
- 「额度预占……_避免_：最终消费、延长额度有效期。」「结算待核对……对应预占继续保留，直到结果被确认、修正或进入人工处置。_避免_：超时后自动释放预占。」（CONTEXT.md）——扩额永不延长 deadline（既有 `TASK_BUDGET_EXPIRED` 409 契约不变）；恢复不得提前释放未确认预占。
- 追加预算 ≠ 外部操作授权：沿 Web `TaskBudget.tsx:21-27,79` 既有纪律——「预算追加与外部写操作审批相互独立」，移动屏文案与确认控件同口径（默认不勾选，无任何自动授权）。
- 安全约束（会话注入）：服务端 SQL 一律参数绑定（本计划新查询全部 `?` 占位 + gorm 绑定，不拼接外部输入）；凭据只从环境变量读取，源码与测试不写入可用凭据字面量；移动 remote 仅经 `authorizedRequest` 既有 HTTPS 通道，origin 构造即强校验（复用 `requireDeploymentOrigin`）。
- 工作流约束：严格 RED→GREEN→REFACTOR（每个任务先写失败测试、实跑确认失败、最小实现、通过、提交）；实现不与已批准 Spec 冲突，冲突时升级而非静默重设计。

**Issue #39 验收标准原文（docs/plans/issue30-sweep/issues/issue-39.md）：**

1. 「Collaborator 不能扩额，Owner/账单管理员权限由服务端强制。」
2. 「恢复不重复历史消耗、预占或外部在途费用。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

验收标准 3 的本地可验证性说明（blocked-env 声明）：「移动端真实达限（真实模型调用耗尽真实预算）→ 暂停 → 扩额 → 恢复」的全链路需要「真实 WeKnora Deployment（HTTPS origin + 已配置商业计费与模型上游）+ 一个测试账号 + 一个可被真实耗尽的任务预算」，属真环境门槛。本地替代证据链（不冒充、不伪造）：①Go 端到端——真实 sqlite 库 + 真实 `AgentRunStore`/`AgentRunWorker`/`BudgetService`/handler/route（Task 1/2/4：达限错误→持久暂停→授权扩额→同一 run 行 requeue；Task 5：恢复后同 callID 重放不新增预占）；②移动端 opt-in 真实 HTTP 集成证据（Task 10，`WEKNORA_MOBILE_TEST_*` 环境变量，含主机防线）：真实 JSON transport + 授权通道 + 具体 Remote Adapter + Task Office 编排读取四数字并（`WEKNORA_MOBILE_TEST_EXTEND_BUDGET=1` 门控）执行一次真实扩额 + 幂等重放断言不加倍；本地无环境时该用例如实 skip。凡具备环境的运行自动产出端到端证据。真机推送面板/原生键盘属真机验收门槛（spec Testing Decisions：「real-device acceptance separately」），本地不伪造真机结论。

**与调查结论的差异记录（以代码现状为准，全部亲眼核实于当前 HEAD `fb5f6653a`）：**

1. 调查称「POST /commercial/tasks/:id/budget/extend 挂 RequireManageBillingForWrites 中间件（routes_commercial.go:29-31,64），非空间 owner 的 Task Owner 也被 403」。**代码现状**：#42（T12）已落地——该路由已移出组级 billing 写门禁、直挂父组的显式商业能力门（`internal/router/routes_commercial.go:62-73`），handler 内先查 `CanManageBilling`，不通过再以 `sessions.user_id` 为权威解析任务 Owner 放行（`internal/handler/commercial_task_budget.go:53-70`），且角色门禁先于幂等逻辑、owner 先应用幂等键后 collaborator 重放同键仍 403（既有测试 `commercial_task_budget_test.go:88-147` 四个用例实跑全绿）。**缺口 1「权限谓词是空间级而非任务级」已由 #42 关闭**；本计划不重复实现，只补：读面（grant 持有者可读、`can_extend=false`）与「collaborator 扩额不得触发 requeue」的端到端证据（Task 3/4）。
2. 调查称扩额 handler「entitlement check 未接线（nil 即 allow 的已披露缺口）」（`commercial_task_budget.go:16-19,44`）。**代码现状属实**：`NewBudgetService(h.db, nil, nil)` 第三参 nil 按 U04 约定 = allow，注释自我披露（当前 HEAD 行 18-26/47）。这是 F02「空间/套餐 entitlement」缺口，**不属于 #39 三条验收标准**（AC1 的角色谓词已强制、AC2/AC3 不涉及空间降级），本计划保持披露、不伪造门禁、不静默接线（空间降级语义需独立决策，牵动 Summary/Usage/Extend 一致性）。
3. 调查称「agent_runs 状态机无 paused→resume 路径（状态集 queued/running/waiting_user/reconciling/recovering）」。**代码现状核实**：`waiting_user→queued` 的恢复先例已存在（决定端点 `ApplyDecision`，`agent_run_decisions.go:255-271`，受护栏 UPDATE + revision+1 + 清 lease）；worker 对 wait 类错误本就 park（`agent_run_worker.go:315-327` `durableWaitReason`：tool_outcome_unknown / tool_outcome_query_required / sandbox_unavailable），且 graph 执行器对「resumed runs restore the checkpointed message list and never re-import or re-recall」已有实现（`agent_run_graph.go:376-383`）。**真正缺口仅是**：budget 达限错误不在 wait 类（落到终态 failed），且无人把 parked 行翻回 queued。本计划 Task 1/2/4 补齐这两点，不新增状态值（复用 `waiting_user` + `wait_reason='budget_exhausted'`，`wait_reason` 列宽 VARCHAR(64) 足以容纳）。
4. 调查称「预算四数字展示缺『预计』与『剩余』，且为 React Web 组件非移动端」。属实：`TaskBudget.tsx:84-88` 仅三数字；apps/mobile 无预算界面。本计划 Task 3-11 交付读端点 + 移动四数字屏 + Web「剩余」派生行。miniprogram 不在本移动 AI Office 批次范围（前序 #32-#46 均未触及 miniprogram），如实记录为范围外。
5. **迁移目录序号冲突（已知升级项，本计划不修）**：`migrations/sqlite/` 同时存在 `000112_task_grants.*`（#42，ca9b66ee1）与 `000112_agent_adoption_variants.*`（#59，a3132eaa0），`migrations/versioned/` 同号冲突（双 000191），golang-migrate 报 `duplicate migration file`，导致**所有走 migrations/sqlite 的既有测试在当前 HEAD 失败**（本作者实跑：`internal/application/service` 的 `TestWorkerParksWaitClassFailureDurable`、`TestCraftBudgetAuthorizeCallReservesBudgetAndCountsCalls` 等失败，根因均为此；`internal/application/repository` 同理）。既有决策（`docs/plans/issue30-sweep/plans/ocr-fix-increment-b3.md:38,2064`）：「重编已发布迁移序号有部署影响，升级为独立决策，不在本批次顺手修改」，验收口径为「失败集合不扩大」。**因此本计划全部新 Go 测试一律手建表**（`newBudgetGateDB`（`commercial_task_budget_test.go:24-54`）与 `task_grant_store_test.go:141` 注释所立的当前惯例），不读 migrations 目录，保证本计划测试命令在当前 HEAD 可跑且不扩大失败集合。迁移序号重编落地后这些测试不依赖迁移文件、继续有效。

## Review Focus

Spec 隐含但无任务测试覆盖、最可能咬到真实用户的五类输入/失效模式（每行后在所属任务落地测试）：

1. **Collaborator 用 owner 已应用的幂等键重放扩额被误放行/误唤醒**：若门禁顺序在幂等之后，协作者重放他人 key 会得到 200 并触发 requeue。——Task 4 测试 `TestExtendTaskBudgetCollaboratorReplayDoesNotResume`（collaborator 重放已应用 key：403 且 parked run 保持 `waiting_user`）+ Task 7 测试「wire 403 映射为 `TASK_BUDGET_FORBIDDEN` 且不消耗幂等键位」+ Task 9 屏级断言 `canExtend=false` 时扩额控件禁用。
2. **恢复重复计费**：requeue 后重放已授权的历史 callID 产生第二笔预占/消耗（双倍扣费）。——Task 5 测试 `TestCraftBudgetResumeAfterExtensionDoesNotDoubleCharge`（达限→扩额→恢复：历史 callID 重放幂等、预占总数不变、同 key 扩额不加倍）。
3. **跨任务误唤醒**：requeue 的子查询把**其它预算根**下 parked 的 run 一并翻回 queued（跨任务串扰、他人预算被悄悄恢复消耗）。——Task 2 测试 `TestRequeueBudgetPausedRunsResumesOnlySameBudgetRoot`（异根 parked / 异 wait_reason / running 行全部不动，二次调用 0 行）。
4. **达限-过期循环**：预算 deadline 已过（或 grant 已过期/吊销）的 run 被扩额重排后无限 park（`ErrGrantExpired`/`ErrTaskBudgetExpired` 也被误判为可恢复 wait）。——Task 1 测试 `TestDurableWaitReasonClassifiesBudgetExhaustionAsDurablePark`（过期类错误**不**进 wait 集 → 保持终态 failed；只有 Exhausted/Denied/GrantExhausted 三哨兵 park）。
5. **读面越权与存在性泄漏**：无 grant 的普通成员枚举他人 run_id 读预算（探查他人任务是否存在/花销）；或 grant 持有者被误标 `can_extend=true` 诱导扩额 UI。——Task 3 测试 `TestGetTaskBudgetReadGateArms`（bystander 403、未知 run 对任何人都 404 不泄漏、grant 协作者可读但 `can_extend=false`、跨租户 404）。

---

## 任务结构与文件地图

| # | 任务 | 主要交付 |
|---|---|---|
| 1 | Go：达限持久暂停（worker budget wait 类） | `durableWaitReason` 增 budget 分支 + 真实 worker park 测试（自建 schema） |
| 2 | Go：`RequeueBudgetPausedRuns` 仓储恢复原语 | 同预算根 parked→queued 受护栏 UPDATE + 仅同根/同因翻转测试 |
| 3 | Go：`GET /commercial/tasks/:id/budget` 读端点 | 四数字（根行）+ 委派/暂停清单 + `can_extend` + 三臂读门测试 |
| 4 | Go：扩额成功路径接线 resume | extend 后 requeue + `resumed_runs` 字段 + collaborator 不唤醒 + 幂等重放治愈竞态（e2e） |
| 5 | Go：恢复不重复消耗（craft 级场景） | 达限→扩额→恢复：历史 callID 幂等、预占不双倍、同 key 扩额不加倍 |
| 6 | contracts：预算 wire 契约 | `parseTaskBudgetFacts`/`parseTaskBudgetExtension`（四数字算术一致性校验） |
| 7 | mobile-core：task-budget 深模块 + TaskOffice 接线 | `createTaskBudgetOps`（scope 守卫/幂等键保持/typed 错误）+ `Ports.budget?` + `office.budget()/extendBudget()` + 场景 Adapter |
| 8 | api-client：`createMobileTaskBudgetRemote` | facts/extend wire 适配 + 错误码翻译 + `./mobile/task-budget` exports |
| 9 | apps/mobile：预算屏与接线 | `TaskBudgetScreen` + `/tasks/budget` 路由 + 详情入口 + composition 装配 + app-smoke 断言 |
| 10 | apps/mobile：真实 HTTP 集成证据 | `task-budget-integration-smoke.ts`（opt-in，AC3）+ 证据契约测试 |
| 11 | Web：`TaskBudget` 补「剩余」 | `remainingCreditsOf` 派生 + 四数分立展示 + 纯函数测试 |

**并行批次注意（本计划与同批其余计划并行实施，独立 worktree 后合并）**：新增文件全部为本计划独有（上表 Create 项）。共享文件修改清单与位置：`internal/application/service/agent_run_worker.go`（`durableWaitReason` 一个 case + 2 个 import）；`internal/application/repository/agent_run.go`（`SetStatus` 之后追加一个方法）；`internal/handler/commercial_task_budget.go`（追加 `GetTaskBudget`/`taskBudgetRoot`/`taskBudgetGrantedReader` + `ExtendTaskBudget` 成功路径 6 行）；`internal/router/routes_commercial.go`（budgetGroup 内 1 行路由）；`packages/contracts/src/index.ts`（2 行 re-export）；`packages/mobile-core/src/task-office/task-office.ts`（Ports 1 行 + 接口 2 方法 + 工厂 10 行）；`packages/mobile-core/src/task-office/task-office-errors.ts`（错误码联合 +1 值）；`packages/mobile-core/src/index.ts`（末尾追加导出块）；`packages/api-client/package.json`（exports +1 行）；`apps/mobile/src/composition.ts`（`taskOfficeFor` 工厂内 +1 行）；`apps/mobile/src/screens/TaskDetailScreen.tsx`（1 可选 prop + 1 按钮）；`apps/mobile/src/app/tasks/detail.tsx`（Lifecycle 1 prop + 默认导出 1 处透传）；`apps/mobile/src/app-smoke.test.tsx`（末尾追加 2 测试）；`apps/web/src/commercial/TaskBudget.tsx`（导出 1 纯函数 + 列表 1 行）。均为最小、位置明确的追加，便于合并。

---
---

### Task 1: Go——达限持久暂停（worker budget wait 类）

**Files:**
- Modify: `internal/application/service/agent_run_worker.go`（`durableWaitReason` 函数，当前 HEAD 行 336-351；文件头 import 块）
- Test: `internal/application/service/agent_run_worker_budget_test.go`（Create）

**Interfaces:**
- Consumes: `durableRunFailureEvent` 的达限错误集（`agent_run_graph.go:496-501`：`repocommercial.ErrTaskBudgetExhausted` / `craft.ErrBudgetDenied` / `craft.ErrGrantExhausted`——达限时 `budget_exhausted` 持久事件已由执行器先行落盘）；`AgentRunStore`（`repository.NewAgentRunStore(db)`，`agent_run.go:29`）；`NewAgentRunWorker(store, execute, cfg)`（`agent_run_worker.go:93`）与 `WorkerConfig` 校验规则（`Heartbeat*2 < Lease`，`:55-63`）。
- Produces: wait 理由常量语义 `waiting_user` + `wait_reason='budget_exhausted'`（Task 2 的 requeue 谓词、Task 3 的 `paused_run_ids` 查询、Task 4 的 e2e 断言均以此为准）。

- [ ] **Step 1: 写失败测试**

创建 `internal/application/service/agent_run_worker_budget_test.go`（**自建 schema，不读 migrations/sqlite 目录**——见差异记录第 5 条）：

```go
package service

// T09 (#39) 达限持久暂停：预算耗尽的执行错误必须把 run 持久停靠在
// waiting_user/budget_exhausted（可经授权扩额恢复同一 Run），而不是终态
// failed。与 durableRunFailureEvent（agent_run_graph.go:496-501）同一错误集；
// 过期类错误（ErrGrantExpired/ErrTaskBudgetExpired）不进 wait 集——否则扩额
// 重排后会形成 park/失败循环。本文件 schema 手工创建，不依赖 migrations/sqlite
// （当前 HEAD 的 000112 序号冲突是已升级的独立决策，见 ocr-fix-increment-b3.md）。

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openBudgetParkDB 手工创建 agent_runs/sessions 的完整列集（agentRunRow，
// internal/application/repository/agent_run.go:40-58），使真实 AgentRunStore 的
// Scan/Claim/Get/Renew/SetStatus 全链路可跑而无需迁移目录。
func openBudgetParkDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE agent_runs (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL,
		session_id TEXT NOT NULL DEFAULT '', owner_id TEXT NOT NULL DEFAULT '',
		request_id TEXT NOT NULL DEFAULT '', assistant_message_id TEXT NOT NULL DEFAULT '',
		request_hash TEXT NOT NULL DEFAULT '', engine_type TEXT NOT NULL DEFAULT '',
		driver TEXT NOT NULL DEFAULT 'platform', target_id TEXT NOT NULL DEFAULT '',
		budget_ref TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'queued',
		wait_reason TEXT NOT NULL DEFAULT '', snapshot TEXT NOT NULL DEFAULT '{}',
		graph_version TEXT NOT NULL DEFAULT '', sdk_version TEXT NOT NULL DEFAULT '',
		schema_version INTEGER NOT NULL DEFAULT 0, lease_owner TEXT NOT NULL DEFAULT '',
		lease_until DATETIME, epoch INTEGER NOT NULL DEFAULT 0, revision INTEGER NOT NULL DEFAULT 0,
		max_rounds INTEGER NOT NULL DEFAULT 0, max_tool_calls INTEGER NOT NULL DEFAULT 0,
		token_budget INTEGER NOT NULL DEFAULT 0, deadline DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (tenant_id, run_id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE sessions (
		id TEXT PRIMARY KEY, tenant_id INTEGER NOT NULL, user_id TEXT,
		active_agent_run_id TEXT)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, user_id) VALUES ('s1', 7, 'u1')`).Error)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func seedClaimableBudgetRun(t *testing.T, db *gorm.DB, runID string) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO agent_runs
		(tenant_id, run_id, session_id, owner_id, driver, status, deadline)
		VALUES (7, ?, 's1', 'u1', 'platform', 'queued', ?)`,
		runID, time.Now().UTC().Add(time.Hour)).Error)
}

func runStatusReason(t *testing.T, db *gorm.DB, runID string) (string, string) {
	t.Helper()
	var row struct{ Status, WaitReason string }
	require.NoError(t, db.Table("agent_runs").
		Select("status, wait_reason").Where("tenant_id = ? AND run_id = ?", 7, runID).
		Take(&row).Error)
	return row.Status, row.WaitReason
}

// TestDurableWaitReasonClassifiesBudgetExhaustionAsDurablePark 是纯分类器表测：
// 三个预算哨兵 park；过期/吊销/普通错误不 park（防达限-过期循环，Review Focus #4）。
func TestDurableWaitReasonClassifiesBudgetExhaustionAsDurablePark(t *testing.T) {
	for _, err := range []error{
		repocommercial.ErrTaskBudgetExhausted,
		craft.ErrBudgetDenied,
		craft.ErrGrantExhausted,
	} {
		reason, wait := durableWaitReason(err)
		require.True(t, wait, "budget sentinel %v must park durably", err)
		require.Equal(t, "budget_exhausted", reason)
	}
	for _, err := range []error{
		craft.ErrGrantExpired,             // 预算/grant 过期：终态，扩额不延长 deadline
		craft.ErrGrantRevoked,             // 吊销：终态
		errors.New("ordinary model failure"), // 普通失败：终态 failed
	} {
		_, wait := durableWaitReason(err)
		require.False(t, wait, "non-budget error %v must stay terminal", err)
	}
}

// TestWorkerParksBudgetExhaustedRunDurable 走真实 worker 全链路（Scan→Claim→
// execute→SetStatus）：预算耗尽错误使 run 停靠 waiting_user/budget_exhausted；
// 对照组普通错误保持终态 failed。
func TestWorkerParksBudgetExhaustedRunDurable(t *testing.T) {
	db := openBudgetParkDB(t)
	seedClaimableBudgetRun(t, db, "park-r1")
	store := repository.NewAgentRunStore(db)
	worker, err := NewAgentRunWorker(store, func(context.Context, agentruntime.Fence) error {
		return repocommercial.ErrTaskBudgetExhausted
	}, WorkerConfig{Enabled: true, Lease: time.Minute, Heartbeat: 15 * time.Second,
		ScanInterval: 5 * time.Second, MaxWorkers: 4})
	require.NoError(t, err)
	require.NoError(t, worker.Tick(context.Background()))
	require.Eventually(t, func() bool {
		status, reason := runStatusReason(t, db, "park-r1")
		return status == "waiting_user" && reason == "budget_exhausted"
	}, 5*time.Second, 20*time.Millisecond, "budget exhaustion must durably park the run")

	// 对照组：普通错误 → 终态 failed（不可经扩额恢复）。
	seedClaimableBudgetRun(t, db, "fail-r2")
	worker2, err := NewAgentRunWorker(store, func(context.Context, agentruntime.Fence) error {
		return errors.New("provider exploded")
	}, WorkerConfig{Enabled: true, Lease: time.Minute, Heartbeat: 15 * time.Second,
		ScanInterval: 5 * time.Second, MaxWorkers: 4})
	require.NoError(t, err)
	require.NoError(t, worker2.Tick(context.Background()))
	require.Eventually(t, func() bool {
		status, _ := runStatusReason(t, db, "fail-r2")
		return status == "failed"
	}, 5*time.Second, 20*time.Millisecond, "ordinary failures stay terminal")
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/application/service/ -run 'TestDurableWaitReasonClassifiesBudgetExhaustionAsDurablePark|TestWorkerParksBudgetExhaustedRunDurable' -count=1 -v`
Expected: FAIL——`durableWaitReason` 对三个预算哨兵返回 `wait=false`（纯分类器用例断言失败：`budget sentinel ... must park durably`）；worker 用例随后超时/断言失败（run 落到 `failed`）。

- [ ] **Step 3: 写最小实现**

`internal/application/service/agent_run_worker.go`——import 块增加两行（与现有 import 分组对齐）：

```go
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	"github.com/Tencent/WeKnora/internal/modules/craft"
```

`durableWaitReason`（当前 HEAD 行 340-351）在 `ErrSandboxUnavailable` 分支后追加一个 case：

```go
	case errors.Is(err, repocommercial.ErrTaskBudgetExhausted),
		errors.Is(err, craft.ErrBudgetDenied),
		errors.Is(err, craft.ErrGrantExhausted):
		// T09 (#39)：达限是持久暂停（CONTEXT.md「任务预算」），不是终态失败。
		// 与 durableRunFailureEvent（agent_run_graph.go）同一错误集；此时
		// budget_exhausted 持久事件已先行落盘，授权扩额后由
		// RequeueBudgetPausedRuns 把同一 Run 翻回 queued 继续执行。
		// 过期类错误（ErrGrantExpired/ErrTaskBudgetExpired）刻意不进此集：
		// 扩额不延长 deadline，重排只会形成 park 循环。
		return "budget_exhausted", true
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/application/service/ -run 'TestDurableWaitReasonClassifiesBudgetExhaustionAsDurablePark|TestWorkerParksBudgetExhaustedRunDurable' -count=1 -v`
Expected: PASS（2 tests）。

- [ ] **Step 5: 确认既有失败集合未扩大**

Run: `go test ./internal/application/service/ -run 'TestWorker|TestDurableWaitReason|TestCraftBudgetExtendRaisesCallCapAndCommercialLimit' -count=1 2>&1 | grep -E '^--- (PASS|FAIL)'`
Expected: 本计划两个新用例 PASS；`TestWorker...` 系列中既有失败（迁移序号冲突所致，差异记录第 5 条）与基线一致、不新增。

- [ ] **Step 6: 提交**

```bash
git add internal/application/service/agent_run_worker.go internal/application/service/agent_run_worker_budget_test.go
git commit -m "feat(budget): park budget-exhausted runs durably at waiting_user/budget_exhausted (T09 #39)"
```

---

### Task 2: Go——`RequeueBudgetPausedRuns` 仓储恢复原语

**Files:**
- Modify: `internal/application/repository/agent_run.go`（`SetStatus` 之后追加方法，当前 HEAD 行 531-557 之后）
- Test: `internal/application/repository/agent_run_budget_requeue_test.go`（Create）

**Interfaces:**
- Consumes: `ApplyDecision` 的 `waiting_user→queued` 恢复形态（`agent_run_decisions.go:255-271`：`status='waiting_user' AND wait_reason=?` 受护栏 UPDATE + `revision+1` + 清 lease）；`commercial_task_budgets.root_run_id` 的父子映射（`budget_task.go:24-58`）。
- Produces: `func (s *AgentRunStore) RequeueBudgetPausedRuns(ctx context.Context, tenantID uint64, budgetRootRunID string) (int64, error)`——返回翻转行数；Task 4 的 handler 与 Task 3 的 `paused_run_ids` 查询共用其谓词语义（`status='waiting_user' AND wait_reason='budget_exhausted' AND (run_id=root OR run_id IN children)`）。

- [ ] **Step 1: 写失败测试**

创建 `internal/application/repository/agent_run_budget_requeue_test.go`（自建 schema，不读 migrations 目录）：

```go
package repository

// T09 (#39) RequeueBudgetPausedRuns：授权扩额后把同一任务预算（根行 + 全部委派
// 子 Run）下停靠的 budget_exhausted run 翻回 claimable 的 queued。只有同根、同
// wait 理由的行翻转；异根/异因/非停靠行一律不动（Review Focus #3 跨任务误唤醒）。

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openRequeueTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE agent_runs (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL,
		session_id TEXT NOT NULL DEFAULT '', owner_id TEXT NOT NULL DEFAULT '',
		request_id TEXT NOT NULL DEFAULT '', assistant_message_id TEXT NOT NULL DEFAULT '',
		request_hash TEXT NOT NULL DEFAULT '', engine_type TEXT NOT NULL DEFAULT '',
		driver TEXT NOT NULL DEFAULT 'platform', target_id TEXT NOT NULL DEFAULT '',
		budget_ref TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'queued',
		wait_reason TEXT NOT NULL DEFAULT '', snapshot TEXT NOT NULL DEFAULT '{}',
		graph_version TEXT NOT NULL DEFAULT '', sdk_version TEXT NOT NULL DEFAULT '',
		schema_version INTEGER NOT NULL DEFAULT 0, lease_owner TEXT NOT NULL DEFAULT '',
		lease_until DATETIME, epoch INTEGER NOT NULL DEFAULT 0, revision INTEGER NOT NULL DEFAULT 0,
		max_rounds INTEGER NOT NULL DEFAULT 0, max_tool_calls INTEGER NOT NULL DEFAULT 0,
		token_budget INTEGER NOT NULL DEFAULT 0, deadline DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (tenant_id, run_id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE commercial_task_budgets (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, root_run_id TEXT NOT NULL DEFAULT '',
		limit_micro BIGINT NOT NULL DEFAULT 0, spent_micro BIGINT NOT NULL DEFAULT 0,
		held_micro BIGINT NOT NULL DEFAULT 0, deadline DATETIME NOT NULL,
		version BIGINT NOT NULL DEFAULT 1, PRIMARY KEY (tenant_id, run_id))`).Error)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func seedParkedRun(t *testing.T, db *gorm.DB, runID, status, waitReason string) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO agent_runs
		(tenant_id, run_id, session_id, owner_id, driver, status, wait_reason, revision, deadline)
		VALUES (7, ?, 's1', 'u1', 'platform', ?, ?, 3, ?)`,
		runID, status, waitReason, time.Now().UTC().Add(time.Hour)).Error)
}

func TestRequeueBudgetPausedRunsResumesOnlySameBudgetRoot(t *testing.T) {
	db := openRequeueTestDB(t)
	store := NewAgentRunStore(db)
	// 预算根 r1：自身 + 委派子 c1 均因达限停靠 → 两者都恢复。
	seedParkedRun(t, db, "r1", "waiting_user", "budget_exhausted")
	seedParkedRun(t, db, "c1", "waiting_user", "budget_exhausted")
	require.NoError(t, db.Exec(`INSERT INTO commercial_task_budgets
		(tenant_id, run_id, root_run_id, deadline) VALUES (7, 'r1', '', ?)`,
		time.Now().UTC().Add(time.Hour)).Error)
	require.NoError(t, db.Exec(`INSERT INTO commercial_task_budgets
		(tenant_id, run_id, root_run_id, deadline) VALUES (7, 'c1', 'r1', ?)`,
		time.Now().UTC().Add(time.Hour)).Error)
	// 异预算根 other-root 的停靠 run：不得唤醒。
	seedParkedRun(t, db, "x1", "waiting_user", "budget_exhausted")
	require.NoError(t, db.Exec(`INSERT INTO commercial_task_budgets
		(tenant_id, run_id, root_run_id, deadline) VALUES (7, 'x1', 'other-root', ?)`,
		time.Now().UTC().Add(time.Hour)).Error)
	// 同根但异因停靠（工具结果未知）：不得唤醒。
	seedParkedRun(t, db, "w1", "waiting_user", "tool_outcome_unknown")
	// 非停靠态：不得触碰。
	seedParkedRun(t, db, "run-running", "running", "")

	n, err := store.RequeueBudgetPausedRuns(context.Background(), 7, "r1")
	require.NoError(t, err)
	require.EqualValues(t, 2, n, "only the root and its delegated child resume")

	for _, runID := range []string{"r1", "c1"} {
		var row struct {
			Status, WaitReason, LeaseOwner string
			Revision                       int64
		}
		require.NoError(t, db.Table("agent_runs").Select("status, wait_reason, lease_owner, revision").
			Where("tenant_id = ? AND run_id = ?", 7, runID).Take(&row).Error)
		require.Equal(t, "queued", row.Status, runID)
		require.Equal(t, "", row.WaitReason, runID)
		require.Equal(t, "", row.LeaseOwner, runID)
		require.EqualValues(t, 4, row.Revision, "revision must advance exactly once (%s)", runID)
	}
	for _, runID := range []string{"x1", "w1"} {
		var row struct{ Status, WaitReason string }
		require.NoError(t, db.Table("agent_runs").Select("status, wait_reason").
			Where("tenant_id = ? AND run_id = ?", 7, runID).Take(&row).Error)
		require.Equal(t, "waiting_user", row.Status, "unrelated parked run %s must stay parked", runID)
		require.NotEqual(t, "", row.WaitReason)
	}
	var running struct{ Status string }
	require.NoError(t, db.Table("agent_runs").Select("status").
		Where("tenant_id = ? AND run_id = ?", 7, "run-running").Take(&running).Error)
	require.Equal(t, "running", running.Status)

	// 幂等：再次调用无行可翻（已 queued 不再匹配谓词）。
	n2, err := store.RequeueBudgetPausedRuns(context.Background(), 7, "r1")
	require.NoError(t, err)
	require.EqualValues(t, 0, n2)

	// 输入护栏：空 root / 零租户拒绝。
	_, err = store.RequeueBudgetPausedRuns(context.Background(), 0, "r1")
	require.Error(t, err)
	_, err = store.RequeueBudgetPausedRuns(context.Background(), 7, "")
	require.Error(t, err)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/application/repository/ -run 'TestRequeueBudgetPausedRuns' -count=1 -v`
Expected: FAIL——`store.RequeueBudgetPausedRuns undefined (type *AgentRunStore has no field or method RequeueBudgetPausedRuns)`（编译失败即失败确认）。

- [ ] **Step 3: 写最小实现**

`internal/application/repository/agent_run.go` 在 `SetStatus` 方法结束后追加（`agentruntime`/`gorm` 已在文件 import 中）：

```go
// RequeueBudgetPausedRuns resumes the runs durably parked at
// waiting_user/budget_exhausted under one task budget — the root run plus
// every attached child run — flipping them back to claimable queued. The
// guarded UPDATE mirrors ApplyDecision's waiting_user→queued transition
// (revision advances; lease fields clear); rows parked for any other wait
// reason, other budget roots, or non-parked states are untouched, and a
// replay affecting already-queued rows is a no-op. T09 (#39): called from
// the authorized budget-extension success path.
func (s *AgentRunStore) RequeueBudgetPausedRuns(ctx context.Context, tenantID uint64, budgetRootRunID string) (int64, error) {
	if tenantID == 0 || budgetRootRunID == "" {
		return 0, agentruntime.ErrConflict
	}
	result := s.db.WithContext(ctx).Table("agent_runs").
		Where("tenant_id = ? AND status = ? AND wait_reason = ?",
			tenantID, "waiting_user", "budget_exhausted").
		Where("run_id = ? OR run_id IN (SELECT run_id FROM commercial_task_budgets WHERE tenant_id = ? AND root_run_id = ?)",
			budgetRootRunID, tenantID, budgetRootRunID).
		Updates(map[string]any{
			"status": "queued", "wait_reason": "", "lease_owner": "", "lease_until": nil,
			"revision": gorm.Expr("revision + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/application/repository/ -run 'TestRequeueBudgetPausedRuns' -count=1 -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/application/repository/agent_run.go internal/application/repository/agent_run_budget_requeue_test.go
git commit -m "feat(budget): RequeueBudgetPausedRuns resumes same-budget-root parked runs (T09 #39)"
```

---

### Task 3: Go——`GET /commercial/tasks/:id/budget` 预算读端点

**Files:**
- Modify: `internal/handler/commercial_task_budget.go`（追加 `GetTaskBudget`、`taskBudgetRoot`、`taskBudgetGrantedReader`；import 增加 `time`）
- Modify: `internal/router/routes_commercial.go`（budgetGroup 内 1 行，当前 HEAD 行 72-73）
- Test: `internal/handler/commercial_task_budget_read_test.go`（Create）

**Interfaces:**
- Consumes: `commercialTenantScope`/`commercialUserID`/`hasBillingGrant`（`commercial.go:100-134`）、`commercial.CanManageBilling`（`access.go:6-8`）、`taskRunOwner`（`commercial_task_budget.go:99-121`）、#42 `task_grants × tenant_members(active)` 授予读谓词（`agent_run.go:123-147` 的 EXISTS 形态）、`appOK`/`appFail`（`app_connector.go:28-36`）。
- Produces: `GET /api/v1/commercial/tasks/:id/budget` → `appOK` 信封 `{"success":true,"data":{task_id, root_run_id, limit_credits, used_credits, held_credits, remaining_credits, deadline, delegated_run_ids, paused_run_ids, can_extend}}`（`*_credits` 为 millionths-of-a-Credit 整数，与既有 `additional_credits` wire 单位一致——`commercial.Credits` 定义见 `amount.go:11`）。Task 6 契约解析、Task 8 remote、Task 10 集成证据消费此形状。`resumed_runs` 字段由 Task 4 加入 extend 响应。

- [ ] **Step 1: 写失败测试**

创建 `internal/handler/commercial_task_budget_read_test.go`：

```go
package handler

// T09 (#39) 预算读端点：四数字来自根预算行（委派子 Run 预占天然计入根行），
// 读门 = 账单权威 OR 任务 Owner（sessions.user_id，ADR-0004）OR #42 task grant
// 持有者（active 成员）；can_extend 只对账单权威/Owner 为 true——grant 协作者可读
// 不可扩（AC1 在 UI 面的如实投影）。未知/跨租户 run 对任何调用者都是 404，不向
// 无权者泄漏 run 存在性（Review Focus #5）。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newBudgetReadDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newBudgetGateDB(t) // sessions/commercial_budget_accounts/commercial_task_budgets 已备（commercial_task_budget_test.go:24-54）
	// 重建 agent_runs：读端点需要 status/wait_reason 列（newBudgetGateDB 的最小表没有）。
	require.NoError(t, db.Exec(`DROP TABLE agent_runs`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE agent_runs (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, owner_id TEXT NOT NULL,
		session_id TEXT, status TEXT NOT NULL DEFAULT 'queued',
		wait_reason TEXT NOT NULL DEFAULT '', driver TEXT NOT NULL DEFAULT 'platform',
		revision INTEGER NOT NULL DEFAULT 0, lease_owner TEXT NOT NULL DEFAULT '',
		lease_until DATETIME, epoch INTEGER NOT NULL DEFAULT 0)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, owner_id, session_id)
		VALUES (7, 'r1', 'u1', 's1')`).Error)
	// newBudgetGateDB 的 commercial_task_budgets 没有 root_run_id 列（父/子映射），
	// 追加后再种委派子行。
	require.NoError(t, db.Exec(`ALTER TABLE commercial_task_budgets ADD COLUMN root_run_id TEXT NOT NULL DEFAULT ''`).Error)
	// 追加读门与投影需要的表：#42 grants + 成员表 + 扩展记录（extend 落 TaskBudgetExtensionRow）。
	require.NoError(t, db.Exec(`CREATE TABLE task_grants (
		tenant_id INTEGER NOT NULL, task_id TEXT NOT NULL, grantee_id TEXT NOT NULL,
		role TEXT NOT NULL, granted_by TEXT NOT NULL DEFAULT '', granted_at DATETIME,
		PRIMARY KEY (tenant_id, task_id, grantee_id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE tenant_members (
		tenant_id INTEGER NOT NULL, user_id TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'active',
		deleted_at DATETIME, PRIMARY KEY (tenant_id, user_id))`).Error)
	require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id, user_id, status) VALUES
		(7, 'u1', 'active'), (7, 'u2', 'active'), (7, 'u3', 'active')`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE commercial_task_budget_extensions (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, key TEXT NOT NULL,
		extra_micro BIGINT NOT NULL, applied_at DATETIME NOT NULL,
		PRIMARY KEY (tenant_id, run_id, key))`).Error)
	// 根行 r1 带真实四数字；委派子 c1 挂根（零 limit 映射行）；r1 因达限停靠。
	require.NoError(t, db.Exec(`UPDATE commercial_task_budgets
		SET limit_micro = 1000, spent_micro = 400, held_micro = 100 WHERE tenant_id = 7 AND run_id = 'r1'`).Error)
	// 委派子行：零 limit 的映射行，仅指向根（budget_task.go AttachChildRun 语义）。
	require.NoError(t, db.Exec(`INSERT INTO commercial_task_budgets
		(tenant_id, run_id, root_run_id, limit_micro, deadline) VALUES (7, 'c1', 'r1', 0, ?)`,
		time.Now().Add(time.Hour).UTC()).Error)
	require.NoError(t, db.Exec(`UPDATE agent_runs SET status = 'waiting_user', wait_reason = 'budget_exhausted'
		WHERE tenant_id = 7 AND run_id = 'r1'`).Error)
	return db
}

func getTaskBudget(t *testing.T, db *gorm.DB, role, userID, runID string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewCommercialHandler(db)
	r := gin.New()
	r.GET("/commercial/tasks/:id/budget", h.GetTaskBudget)
	req := httptest.NewRequest(http.MethodGet, "/commercial/tasks/"+runID+"/budget", nil)
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, uint64(7))
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	if role != "" {
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRole(role))
	}
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var body map[string]any
	if w.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	}
	return w, body
}

func TestGetTaskBudgetReadGateArms(t *testing.T) {
	db := newBudgetReadDB(t)

	// 任务 Owner（普通 contributor，非空间 owner）：可读，can_extend=true，
	// 四数字来自根行（1000/400/100/500），委派与停靠清单如实。
	w, data := getTaskBudget(t, db, "contributor", "u1", "r1")
	require.Equal(t, http.StatusOK, w.Code, "task owner may read: %s", w.Body.String())
	require.Equal(t, "r1", data["task_id"])
	require.Equal(t, "r1", data["root_run_id"])
	require.EqualValues(t, 1000, data["limit_credits"])
	require.EqualValues(t, 400, data["used_credits"])
	require.EqualValues(t, 100, data["held_credits"])
	require.EqualValues(t, 500, data["remaining_credits"])
	require.Equal(t, true, data["can_extend"])
	require.Equal(t, []any{"c1"}, data["delegated_run_ids"])
	require.Equal(t, []any{"r1"}, data["paused_run_ids"])

	// grant 协作者（#42 真实 grant 行，active 成员）：可读，但 can_extend=false。
	require.NoError(t, db.Exec(`INSERT INTO task_grants (tenant_id, task_id, grantee_id, role)
		VALUES (7, 's1', 'u2', 'collaborator')`).Error)
	w2, data2 := getTaskBudget(t, db, "contributor", "u2", "r1")
	require.Equal(t, http.StatusOK, w2.Code, "granted collaborator may READ the budget")
	require.Equal(t, false, data2["can_extend"], "a grant never carries budget authority (AC1)")

	// 账单权威（空间 owner 角色）：可读，can_extend=true。
	w3, data3 := getTaskBudget(t, db, "owner", "boss", "r1")
	require.Equal(t, http.StatusOK, w3.Code)
	require.Equal(t, true, data3["can_extend"])

	// 无 grant 的普通成员（bystander）：403，不泄漏数字。
	w4, _ := getTaskBudget(t, db, "contributor", "u3", "r1")
	require.Equal(t, http.StatusForbidden, w4.Code)
	require.Contains(t, w4.Body.String(), "BUDGET_FORBIDDEN")

	// 未知 run 对任何人都 404（与写门同口径，不泄漏存在性）。
	w5, _ := getTaskBudget(t, db, "contributor", "u1", "missing")
	require.Equal(t, http.StatusNotFound, w5.Code)
	require.Contains(t, w5.Body.String(), "TASK_BUDGET_NOT_FOUND")
	// 跨租户 run 同样 404。
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, owner_id) VALUES (8, 'r8', 'u1')`).Error)
	w6, _ := getTaskBudget(t, db, "contributor", "u1", "r8")
	require.Equal(t, http.StatusNotFound, w6.Code)

	// 以委派子 Run 寻址：返回根行数字 + root_run_id（cumulative budget）。
	w7, data7 := getTaskBudget(t, db, "contributor", "u1", "c1")
	require.Equal(t, http.StatusOK, w7.Code)
	require.Equal(t, "r1", data7["root_run_id"])
	require.EqualValues(t, 1000, data7["limit_credits"])
	require.EqualValues(t, 500, data7["remaining_credits"])
}
```

（import 集合在文件头：`context`/`encoding/json`/`net/http`/`net/http/httptest`/`testing`/`time` + `github.com/Tencent/WeKnora/internal/types` + `github.com/gin-gonic/gin` + `github.com/stretchr/testify/require` + `gorm.io/gorm`——与 `commercial_task_budget_test.go` 同款。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/handler/ -run 'TestGetTaskBudgetReadGateArms' -count=1 -v`
Expected: FAIL——`h.GetTaskBudget undefined (type *CommercialHandler has no field or method GetTaskBudget)`（编译失败即失败确认）。

- [ ] **Step 3: 写最小实现**

`internal/router/routes_commercial.go`：budgetGroup（当前 HEAD 行 72-73）内追加 1 行：

```go
	// T09 (#39): the task budget readout (estimated/used/reserved/remaining
	// from the ROOT budget row — delegated runs charge it exactly once) plus
	// delegated/paused run lists and the caller's own can_extend verdict.
	// Read gate lives in the handler: billing authority, the task owner
	// (sessions.user_id) or a #42 task-grant holder.
	budgetGroup.GET("/tasks/:id/budget", commercialHandler.GetTaskBudget)
```

`internal/handler/commercial_task_budget.go` 文件尾追加（import 增加 `"time"`；`context`/`database/sql`/`strings` 已在）：

```go
// taskBudgetRoot resolves the budget ROOT of one run row: an empty
// RootRunID marks the owner row itself. found=false covers unknown and
// cross-tenant runs alike.
func (h *CommercialHandler) taskBudgetRoot(ctx context.Context, tenantID uint64, runID string) (string, bool, error) {
	if h == nil || h.db == nil || tenantID == 0 || runID == "" {
		return "", false, nil
	}
	var row struct {
		RunID     string
		RootRunID string
	}
	err := h.db.WithContext(ctx).Table("commercial_task_budgets").
		Select("run_id, root_run_id").
		Where("tenant_id = ? AND run_id = ?", tenantID, runID).Scan(&row).Error
	if err != nil {
		return "", false, err
	}
	if row.RunID == "" {
		return "", false, nil
	}
	if row.RootRunID == "" {
		return row.RunID, true, nil
	}
	return row.RootRunID, true, nil
}

// taskBudgetGrantedReader reports whether userID holds ANY #42 task grant on
// the task that owns the budget root (task_id = the root run's session) while
// being an active member. A grant never carries budget authority — it admits
// the READ only (can_extend stays false).
func (h *CommercialHandler) taskBudgetGrantedReader(ctx context.Context, tenantID uint64, rootRunID, userID string) bool {
	if h == nil || h.db == nil || tenantID == 0 || rootRunID == "" || userID == "" {
		return false
	}
	var n int64
	err := h.db.WithContext(ctx).Table("task_grants tg").
		Joins("JOIN agent_runs root ON root.tenant_id = tg.tenant_id AND root.session_id = tg.task_id").
		Joins("JOIN tenant_members tm ON tm.tenant_id = tg.tenant_id AND tm.user_id = tg.grantee_id AND tm.status = 'active' AND tm.deleted_at IS NULL").
		Where("root.tenant_id = ? AND root.run_id = ? AND tg.grantee_id = ?", tenantID, rootRunID, userID).
		Count(&n).Error
	return err == nil && n > 0
}

// GetTaskBudget GET /commercial/tasks/:id/budget (T09 #39). The four numbers
// come from the ROOT budget row — a delegated child run charges its parent's
// budget exactly once (G4 Reserve resolves the root), so the root row already
// aggregates delegated spend. Estimates never fabricate: an absent row is the
// explicit 404, never zero-filled numbers.
func (h *CommercialHandler) GetTaskBudget(c *gin.Context) {
	tenantID, role, ok := commercialTenantScope(c)
	if !ok {
		appFail(c, http.StatusForbidden, "MISSING_TENANT_SCOPE", ErrMissingTenantScope.Error())
		return
	}
	if h.db == nil {
		appFail(c, http.StatusServiceUnavailable, "BUDGET_DATABASE_MISSING", "budget storage is unavailable")
		return
	}
	runID := c.Param("id")
	root, found, err := h.taskBudgetRoot(c.Request.Context(), tenantID, runID)
	if err != nil {
		appFail(c, http.StatusInternalServerError, "BUDGET_READ_FAILED", "failed to resolve the task budget")
		return
	}
	if !found {
		appFail(c, http.StatusNotFound, "TASK_BUDGET_NOT_FOUND", "task budget not found")
		return
	}
	userID := commercialUserID(c)
	billing := commercial.CanManageBilling(role, true, h.hasBillingGrant(c, tenantID))
	owner := false
	if !billing {
		var ownerErr error
		owner, ownerErr = h.ownerMatches(c.Request.Context(), tenantID, root, userID)
		if ownerErr != nil {
			appFail(c, http.StatusInternalServerError, "BUDGET_OWNER_LOOKUP_FAILED", "failed to resolve the task owner")
			return
		}
		if !owner && !h.taskBudgetGrantedReader(c.Request.Context(), tenantID, root, userID) {
			appFail(c, http.StatusForbidden, "BUDGET_FORBIDDEN",
				"reading a task budget requires the task owner, a task grant or billing authority")
			return
		}
	}
	var row struct {
		LimitMicro int64
		SpentMicro int64
		HeldMicro  int64
		Deadline   time.Time
	}
	if err := h.db.WithContext(c.Request.Context()).Table("commercial_task_budgets").
		Select("limit_micro, spent_micro, held_micro, deadline").
		Where("tenant_id = ? AND run_id = ?", tenantID, root).Scan(&row).Error; err != nil {
		appFail(c, http.StatusInternalServerError, "BUDGET_READ_FAILED", "failed to read the task budget")
		return
	}
	var delegated []string
	if err := h.db.WithContext(c.Request.Context()).Table("commercial_task_budgets").
		Where("tenant_id = ? AND root_run_id = ?", tenantID, root).
		Order("run_id").Pluck("run_id", &delegated).Error; err != nil {
		appFail(c, http.StatusInternalServerError, "BUDGET_READ_FAILED", "failed to read delegated runs")
		return
	}
	var paused []string
	if err := h.db.WithContext(c.Request.Context()).Table("agent_runs").
		Where("tenant_id = ? AND status = ? AND wait_reason = ?", tenantID, "waiting_user", "budget_exhausted").
		Where("run_id = ? OR run_id IN (SELECT run_id FROM commercial_task_budgets WHERE tenant_id = ? AND root_run_id = ?)",
			root, tenantID, root).
		Order("run_id").Pluck("run_id", &paused).Error; err != nil {
		appFail(c, http.StatusInternalServerError, "BUDGET_READ_FAILED", "failed to read paused runs")
		return
	}
	data := gin.H{
		"task_id":           runID,
		"root_run_id":       root,
		"limit_credits":     row.LimitMicro,
		"used_credits":      row.SpentMicro,
		"held_credits":      row.HeldMicro,
		"remaining_credits": row.LimitMicro - row.SpentMicro - row.HeldMicro,
		"delegated_run_ids": delegated,
		"paused_run_ids":    paused,
		"can_extend":        billing || owner,
	}
	if !row.Deadline.IsZero() {
		data["deadline"] = row.Deadline.UTC().Format(time.RFC3339)
	}
	if delegated == nil {
		data["delegated_run_ids"] = []string{}
	}
	if paused == nil {
		data["paused_run_ids"] = []string{}
	}
	appOK(c, http.StatusOK, data)
}

// ownerMatches reports whether userID is the business owner of the run's task
// (sessions.user_id authority, ADR-0004 — taskRunOwner returning the verdict).
func (h *CommercialHandler) ownerMatches(ctx context.Context, tenantID uint64, runID, userID string) (bool, error) {
	ownerID, found, err := h.taskRunOwner(ctx, tenantID, runID)
	if err != nil {
		return false, err
	}
	return found && ownerID == userID, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/handler/ -run 'TestGetTaskBudgetReadGateArms' -count=1 -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/handler/commercial_task_budget.go internal/handler/commercial_task_budget_read_test.go internal/router/routes_commercial.go
git commit -m "feat(budget): GET /commercial/tasks/:id/budget four-number readout with owner/grant/billing gate (T09 #39)"
```

---

### Task 4: Go——扩额成功路径接线 resume（授权扩额后恢复同一 Run）

**Files:**
- Modify: `internal/handler/commercial_task_budget.go`（`ExtendTaskBudget` 成功路径，当前 HEAD 行 71-96）
- Test: `internal/handler/commercial_task_budget_resume_test.go`（Create）

**Interfaces:**
- Consumes: Task 2 的 `RequeueBudgetPausedRuns(ctx, tenantID, rootRunID)`；Task 3 的 `taskBudgetRoot`；既有 `svc.Extend`（`budget.go:68-75`，exactly-once per key）；`repository.NewAgentRunStore(h.db)`。
- Produces: `POST /api/v1/commercial/tasks/:id/budget/extend` 成功响应新增 `resumed_runs`（int64，本次被唤醒的 run 行数；幂等重放同样执行 requeue——治愈「扩额已提交但 requeue 失败」的竞态，谓词护栏保证重放是 no-op）。Web 既有解析 `parseTaskBudgetExtensionResult`（`packages/contracts/src/appconnector.ts:246-253`）为结构性择取，对新字段天然容忍，实测无需改动。

- [ ] **Step 1: 写失败测试**

创建 `internal/handler/commercial_task_budget_resume_test.go`（复用 Task 3 测试的建表——直接再次手建，避免测试间耦合；两个测试文件同包 `handler`，helper 名不得冲突，前缀 `resume`）：

```go
package handler

// T09 (#39) 授权扩额 → 恢复同一 Run：extend 成功后，同一预算根下停靠的
// budget_exhausted run 被翻回 queued（resumed_runs 如实计数）；幂等重放同键
// 不再加额但同样执行 requeue（治愈竞态）；collaborator 403 不触发任何唤醒
// （Review Focus #1：角色门禁先于幂等与 requeue）。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newBudgetResumeDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE agent_runs (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, owner_id TEXT NOT NULL,
		session_id TEXT, status TEXT NOT NULL DEFAULT 'queued',
		wait_reason TEXT NOT NULL DEFAULT '', driver TEXT NOT NULL DEFAULT 'platform',
		revision INTEGER NOT NULL DEFAULT 0, lease_owner TEXT NOT NULL DEFAULT '',
		lease_until DATETIME, epoch INTEGER NOT NULL DEFAULT 0)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE sessions (id TEXT PRIMARY KEY, tenant_id INTEGER NOT NULL, user_id TEXT)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE commercial_budget_accounts (
		tenant_id INTEGER PRIMARY KEY, verified_micro BIGINT NOT NULL, unreflected_micro BIGINT NOT NULL DEFAULT 0,
		held_micro BIGINT NOT NULL DEFAULT 0, refund_locked_micro BIGINT NOT NULL DEFAULT 0,
		watermark TEXT NOT NULL DEFAULT 'w0', verified_until DATETIME NOT NULL, version BIGINT NOT NULL DEFAULT 0)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE commercial_task_budgets (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, root_run_id TEXT NOT NULL DEFAULT '',
		limit_micro BIGINT NOT NULL, spent_micro BIGINT NOT NULL DEFAULT 0,
		held_micro BIGINT NOT NULL DEFAULT 0, deadline DATETIME NOT NULL,
		version BIGINT NOT NULL DEFAULT 1, PRIMARY KEY (tenant_id, run_id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE commercial_task_budget_extensions (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, key TEXT NOT NULL,
		extra_micro BIGINT NOT NULL, applied_at DATETIME NOT NULL,
		PRIMARY KEY (tenant_id, run_id, key))`).Error)
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, user_id) VALUES ('s1', 7, 'u1')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, owner_id, session_id, status, wait_reason)
		VALUES (7, 'r1', 'u1', 's1', 'waiting_user', 'budget_exhausted')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, owner_id, session_id, status, wait_reason)
		VALUES (7, 'c1', 'u1', 's1', 'waiting_user', 'budget_exhausted')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO commercial_budget_accounts
		(tenant_id, verified_micro, verified_until, version) VALUES (7, 100000000, ?, 0)`,
		time.Now().Add(time.Hour).UTC()).Error)
	require.NoError(t, db.Exec(`INSERT INTO commercial_task_budgets
		(tenant_id, run_id, limit_micro, deadline, version) VALUES (7, 'r1', 1000, ?, 1)`,
		time.Now().Add(time.Hour).UTC()).Error)
	require.NoError(t, db.Exec(`INSERT INTO commercial_task_budgets
		(tenant_id, run_id, root_run_id, deadline) VALUES (7, 'c1', 'r1', ?)`,
		time.Now().Add(time.Hour).UTC()).Error)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func postBudgetExtendResume(t *testing.T, db *gorm.DB, role, userID, runID, key string) (int, map[string]any, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewCommercialHandler(db)
	r := gin.New()
	r.POST("/commercial/tasks/:id/budget/extend", h.ExtendTaskBudget)
	body := `{"additional_credits": 10, "idempotency_key": "` + key + `"}`
	req := httptest.NewRequest(http.MethodPost, "/commercial/tasks/"+runID+"/budget/extend", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, uint64(7))
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	if role != "" {
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRole(role))
	}
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var data map[string]any
	if w.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &data))
	}
	return w.Code, data, w.Body.String()
}

func runStatusOf(t *testing.T, db *gorm.DB, runID string) string {
	t.Helper()
	var status string
	require.NoError(t, db.Table("agent_runs").Select("status").
		Where("tenant_id = ? AND run_id = ?", 7, runID).Scan(&status).Error)
	return status
}

func TestExtendTaskBudgetResumesParkedRunsEndToEnd(t *testing.T) {
	db := newBudgetResumeDB(t)

	// Collaborator（非 Owner、无 grant 的普通成员）重放 owner 尚未应用的键：403，
	// 且不唤醒任何 run（角色门禁在幂等与 requeue 之前）。
	code, _, body := postBudgetExtendResume(t, db, "contributor", "u2", "r1", "k-e2e-1")
	require.Equal(t, http.StatusForbidden, code, "collaborator refused: %s", body)
	require.Equal(t, "waiting_user", runStatusOf(t, db, "r1"))
	require.Equal(t, "waiting_user", runStatusOf(t, db, "c1"))

	// 任务 Owner 扩额：200 + resumed_runs=2（根 + 委派子），两行翻回 queued。
	code, data, body := postBudgetExtendResume(t, db, "contributor", "u1", "r1", "k-e2e-1")
	require.Equal(t, http.StatusOK, code, "owner extends and resumes: %s", body)
	require.EqualValues(t, 2, data["resumed_runs"])
	require.Equal(t, "queued", runStatusOf(t, db, "r1"))
	require.Equal(t, "queued", runStatusOf(t, db, "c1"))
	var limit int64
	require.NoError(t, db.Table("commercial_task_budgets").Select("limit_micro").
		Where("tenant_id = ? AND run_id = ?", 7, "r1").Scan(&limit).Error)
	require.EqualValues(t, 1010, limit, "limit raised exactly once")

	// 幂等重放同键：200、limit 不再加倍（exactly-once），requeue 谓词不再匹配
	// （已 queued）→ resumed_runs=0。
	code, data, body = postBudgetExtendResume(t, db, "contributor", "u1", "r1", "k-e2e-1")
	require.Equal(t, http.StatusOK, code)
	require.EqualValues(t, 0, data["resumed_runs"])
	require.NoError(t, db.Table("commercial_task_budgets").Select("limit_micro").
		Where("tenant_id = ? AND run_id = ?", 7, "r1").Scan(&limit).Error)
	require.EqualValues(t, 1010, limit, "idempotent replay never raises twice")

	// 竞态治愈：若 run 在重放前又被停靠（模拟 requeue 失败后的重新 park），
	// 同键重放把它翻回 queued 而不加额。
	require.NoError(t, db.Exec(`UPDATE agent_runs SET status='waiting_user', wait_reason='budget_exhausted'
		WHERE tenant_id=7 AND run_id='r1'`).Error)
	code, data, body = postBudgetExtendResume(t, db, "contributor", "u1", "r1", "k-e2e-1")
	require.Equal(t, http.StatusOK, code)
	require.EqualValues(t, 1, data["resumed_runs"], "replay heals a stranded park without re-raising")
	require.EqualValues(t, 1010, limitAfter(t, db))
}

func limitAfter(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var limit int64
	require.NoError(t, db.Table("commercial_task_budgets").Select("limit_micro").
		Where("tenant_id = ? AND run_id = ?", 7, "r1").Scan(&limit).Error)
	return limit
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/handler/ -run 'TestExtendTaskBudgetResumesParkedRunsEndToEnd' -count=1 -v`
Expected: 第一个断言即 FAIL——collaborator 403 通过（既有门禁），但 owner 200 后 `data["resumed_runs"]` 为 nil（`invalid memory address` 或 `EqualValues` 失败：期望 2、实际 nil），且两行 run 仍 `waiting_user`。

- [ ] **Step 3: 写最小实现**

`internal/handler/commercial_task_budget.go`：import 增加 `"github.com/Tencent/WeKnora/internal/application/repository"`；`ExtendTaskBudget` 的成功返回（当前 HEAD 行 93-96）替换为：

```go
	// T09 (#39): an AUTHORIZED raise also resumes the runs this budget parked
	// — the durable same-Run continuation. The requeue runs on EVERY success,
	// idempotent replays included: the guarded predicate makes the replay a
	// no-op unless a run stranded parked (healing a lost first requeue)
	// without ever raising the limit twice. A refused caller never reaches
	// here, so a collaborator can never resume anything.
	resumed := int64(0)
	if root, found, rerr := h.taskBudgetRoot(c.Request.Context(), tenantID, runID); rerr == nil && found {
		if n, qerr := repository.NewAgentRunStore(h.db).
			RequeueBudgetPausedRuns(c.Request.Context(), tenantID, root); qerr == nil {
			resumed = n
		}
	}
	appOK(c, http.StatusOK, gin.H{
		"task_id":            runID,
		"additional_credits": *input.AdditionalCredits,
		"resumed_runs":       resumed,
	})
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/handler/ -run 'TestExtendTaskBudget|TestGetTaskBudget' -count=1 -v`
Expected: PASS——本任务 1 个用例 + #42 既有 4 个扩额门禁用例 + Task 3 读端点用例全绿（既有用例不因 `resumed_runs` 新字段失败：它们只断言状态码与 `BUDGET_FORBIDDEN`/`TASK_BUDGET_NOT_FOUND` 文案）。

- [ ] **Step 5: 提交**

```bash
git add internal/handler/commercial_task_budget.go internal/handler/commercial_task_budget_resume_test.go
git commit -m "feat(budget): authorized extension resumes same-budget paused runs with resumed_runs evidence (T09 #39)"
```

---

### Task 5: Go——恢复不重复消耗、预占或在途费用（craft 级场景，AC2）

**Files:**
- Test: `internal/application/service/craft_budget_resume_test.go`（Create；无生产代码改动——本任务用测试钉住既有幂等原语在「达限→扩额→恢复」序列下的端到端不变量）

**Interfaces:**
- Consumes: `NewCraftBudgetService(db, nil, policy)`（`craft_budget.go:164-185`）、`AuthorizeBinding`/`AuthorizeCall`（`:277-367`，同 callID 幂等 + 拒绝补偿）、`Extend`（`:600-637`，双 fence 整体幂等 per key）、`Admit`；commercial 行的 gorm AutoMigrate 集合（既有 `craftBudgetEnv` 同款，`craft_budget_test.go:46-51`）——但**不使用** `openCraftBudgetTestDB`（其走 migrations/sqlite，当前 HEAD 因 000112 冲突不可跑，差异记录第 5 条）。
- Produces: AC2 的 Go 侧权威证据：`TestCraftBudgetResumeAfterExtensionDoesNotDoubleCharge`。

- [ ] **Step 1: 写失败测试（本任务为「钉住既有行为」型：测试先行，若既有实现已满足则直接转绿——此时失败确认步骤改为「删除测试中对既有行为的误述」并如实记录；若红则修复 craft_budget.go）**

创建 `internal/application/service/craft_budget_resume_test.go`：

```go
package service

// T09 (#39) AC2 权威场景（达限→授权扩额→恢复同一 Run 的不重复计费不变量）：
//  1. 历史 callID 重放幂等：恢复后重发 call A 的 AuthorizeCall 不产生第二笔
//     预占/消耗（reservations 总数不变）；
//  2. 扩额 exactly-once：同 idempotency key 重放不再加额；
//  3. 恢复只对「新调用」放行：call C 在扩额前被拒、扩额后成功，历史 call A/B
//     的预占原样保留（未被释放、未被重复）。
// 自建库（AutoMigrate），不读 migrations/sqlite（当前 HEAD 序号冲突，见计划差异记录）。

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openResumeCraftDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	if pool, err := db.DB(); err == nil {
		pool.SetMaxOpenConns(1)
	}
	require.NoError(t, db.AutoMigrate(
		&repocommercial.BudgetAccountRow{}, &repocommercial.TaskBudgetRow{},
		&repocommercial.ReservationRow{}, &repocommercial.BudgetLotRow{},
		&repocommercial.BudgetLotAllocationRow{}, &repocommercial.TaskBudgetExtensionRow{},
		&commercialsvc.SettlementRecord{},
		&CraftBudgetGrantRow{}, &CraftBudgetCallRow{},
	))
	return db
}

func TestCraftBudgetResumeAfterExtensionDoesNotDoubleCharge(t *testing.T) {
	db := openResumeCraftDB(t)
	// TaskLimit 2000、CallUpper 1000：两次调用即耗尽任务预算（limit-spent-held=0）。
	svc, err := NewCraftBudgetService(db, nil, CraftBudgetPolicy{
		GrantWindow: time.Hour, MaxCalls: 5,
		CallUpper: commercial.Credits(1000), TaskLimit: commercial.Credits(2000),
	})
	require.NoError(t, err)
	ctx := context.Background()
	seedCraftFundedTenant(t, db, 7, 100000) // 既有 helper（craft_budget_test.go:72-81）

	g, err := svc.Admit(ctx, craft.Scope{TenantID: 7, UserID: "u1", SessionID: "s1"}, "resume-run-1")
	require.NoError(t, err)
	binding := CraftCallBinding{ModelID: "m1", Funding: "platform"}

	callA, err := svc.AuthorizeBinding(ctx, g.ID, binding)
	require.NoError(t, err)
	callB, err := svc.AuthorizeBinding(ctx, g.ID, binding)
	require.NoError(t, err)
	require.Len(t, craftReservations(t, db, 7), 2)

	// 达限：第三次调用被拒（task headroom=0），此刻 run 持久暂停（Task 1）。
	_, err = svc.AuthorizeBinding(ctx, g.ID, binding)
	require.ErrorIs(t, err, craft.ErrBudgetDenied)

	// 授权扩额（+2000）。
	require.NoError(t, svc.Extend(ctx, g.ID, "k-resume-1", 5, commercial.Credits(2000)))

	// 恢复后：新调用成功；历史 callID 重放幂等（不产生第二笔预占）。
	callC, err := svc.AuthorizeBinding(ctx, g.ID, binding)
	require.NoError(t, err)
	require.NotEmpty(t, callC)
	require.NoError(t, svc.AuthorizeCall(ctx, g.ID, callA))
	require.NoError(t, svc.AuthorizeCall(ctx, g.ID, callA)) // 再重放仍幂等
	require.Len(t, craftReservations(t, db, 7), 3, "resume replays never add reservations")

	// 任务行不变量：limit 恰好 +2000 一次；held=3×1000；spent 仍 0（未结算）。
	var task repocommercial.TaskBudgetRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 7, "resume-run-1").First(&task).Error)
	require.EqualValues(t, 4000, task.LimitMicro)
	require.EqualValues(t, 3000, task.HeldMicro)
	require.EqualValues(t, 0, task.SpentMicro)

	// 同 key 扩额重放：整体幂等，双 fence 均不再动。
	require.NoError(t, svc.Extend(ctx, g.ID, "k-resume-1", 5, commercial.Credits(2000)))
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 7, "resume-run-1").First(&task).Error)
	require.EqualValues(t, 4000, task.LimitMicro, "same-key replay never raises twice")
	var grants int64
	require.NoError(t, db.Model(&CraftBudgetGrantRow{}).Where("grant_id = ?", g.ID).Count(&grants).Error)
	require.EqualValues(t, 1, grants)
	var calls int64
	require.NoError(t, db.Model(&CraftBudgetCallRow{}).Where("grant_id = ?", g.ID).Count(&calls).Error)
	require.EqualValues(t, 3, calls, "denied call was compensated; only accepted calls ledger")
}
```

- [ ] **Step 2: 运行测试确认当前行为**

Run: `go test ./internal/application/service/ -run 'TestCraftBudgetResumeAfterExtensionDoesNotDoubleCharge' -count=1 -v`
Expected: PASS（既有原语已满足不变量：`AuthorizeCall` 幂等重放 `resolveReplayedReservation`、`Extend` 整体幂等、拒绝补偿 `compensateCall`）。**若任何断言失败**：按失败点修复 `internal/application/service/craft_budget.go` 的对应原语（禁止改断言迁就实现），并把修复写入本任务提交说明。作者判断预期直接转绿——本任务的价值是把 AC2 序列钉在最高可用 Interface 上，防回归。

- [ ] **Step 3: 运行并确认（RED→GREEN 语义说明）**

本任务属 writing-plans 允许的「先写测试钉住既有行为」情形：Step 2 的运行即为确认。若绿：直接进入 Step 4。若红：最小实现修复后重跑至绿。

- [ ] **Step 4: 提交**

```bash
git add internal/application/service/craft_budget_resume_test.go
git commit -m "test(budget): pin resume-after-extension no-double-charge invariants (AC2, T09 #39)"
```

---

### Task 6: contracts——预算 wire 契约（`packages/contracts`）

**Files:**
- Create: `packages/contracts/src/mobile/task-budget.ts`
- Modify: `packages/contracts/src/index.ts`（`parseInteractionWithRun` 导出块（当前 HEAD 行 661-662）之后追加 2 行）
- Test: `packages/contracts/test/mobile-task-budget.test.ts`

**Interfaces:**
- Consumes: `ContractError`（`packages/contracts/src/index.ts` 根导出，用法同 `mobile/interaction-inbox.ts:23-31`）；Task 3/4 定义的 wire 形状。
- Produces: `TaskBudgetWireFacts`（`task_id`/`root_run_id`/`limit_credits`/`used_credits`/`held_credits`/`remaining_credits`/`deadline?`/`delegated_run_ids`/`paused_run_ids`/`can_extend`）与 `TaskBudgetWireExtension`（`task_id`/`additional_credits`/`resumed_runs`）类型 + `parseTaskBudgetFacts(value: unknown): TaskBudgetWireFacts` + `parseTaskBudgetExtension(value: unknown): TaskBudgetWireExtension`——Task 8 的 remote 消费；`@weknora/contracts` 根导出。

- [ ] **Step 1: 写失败测试**

创建 `packages/contracts/test/mobile-task-budget.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { parseTaskBudgetFacts, parseTaskBudgetExtension, ContractError } from '@weknora/contracts';

const factsWire = {
  task_id: 'r1', root_run_id: 'r1',
  limit_credits: 1000, used_credits: 400, held_credits: 100, remaining_credits: 500,
  deadline: '2026-09-24T12:00:00Z',
  delegated_run_ids: ['c1'], paused_run_ids: ['r1'], can_extend: true,
};

test('parseTaskBudgetFacts accepts the full wire row and keeps the four numbers distinct', () => {
  const row = parseTaskBudgetFacts(factsWire);
  assert.equal(row.task_id, 'r1');
  assert.equal(row.root_run_id, 'r1');
  assert.equal(row.limit_credits, 1000);
  assert.equal(row.used_credits, 400);
  assert.equal(row.held_credits, 100);
  assert.equal(row.remaining_credits, 500);
  assert.deepEqual(row.delegated_run_ids, ['c1']);
  assert.deepEqual(row.paused_run_ids, ['r1']);
  assert.equal(row.can_extend, true);
});

test('parseTaskBudgetFacts tolerates absent deadline and empty arrays', () => {
  const row = parseTaskBudgetFacts({ ...factsWire, deadline: undefined, delegated_run_ids: [], paused_run_ids: [] });
  assert.equal(row.deadline, undefined);
  assert.deepEqual(row.delegated_run_ids, []);
});

test('parseTaskBudgetFacts rejects arithmetic inconsistency and malformed numbers (Review Focus #2/#5 面)', () => {
  // 剩余与三数不一致：服务端序列化缺陷必须 fail-closed，客户端不得展示幻影数字。
  assert.throws(() => parseTaskBudgetFacts({ ...factsWire, remaining_credits: 499 }), ContractError);
  for (const bad of [1.5, NaN, '1000', -1, undefined]) {
    assert.throws(() => parseTaskBudgetFacts({ ...factsWire, limit_credits: bad }), ContractError,
      `limit_credits=${String(bad)}`);
  }
  for (const bad of [undefined, '', '   ', 7]) {
    assert.throws(() => parseTaskBudgetFacts({ ...factsWire, task_id: bad }), ContractError);
    assert.throws(() => parseTaskBudgetFacts({ ...factsWire, root_run_id: bad }), ContractError);
  }
  assert.throws(() => parseTaskBudgetFacts({ ...factsWire, delegated_run_ids: 'c1' }), ContractError);
  assert.throws(() => parseTaskBudgetFacts({ ...factsWire, delegated_run_ids: [''] }), ContractError);
  assert.throws(() => parseTaskBudgetFacts({ ...factsWire, paused_run_ids: [3] }), ContractError);
  assert.throws(() => parseTaskBudgetFacts({ ...factsWire, can_extend: 'yes' }), ContractError);
  assert.throws(() => parseTaskBudgetFacts(null), ContractError);
});

test('parseTaskBudgetExtension requires positive credits and a non-negative resume count', () => {
  const row = parseTaskBudgetExtension({ task_id: 'r1', additional_credits: 10, resumed_runs: 2 });
  assert.equal(row.task_id, 'r1');
  assert.equal(row.additional_credits, 10);
  assert.equal(row.resumed_runs, 2);
  const zero = parseTaskBudgetExtension({ task_id: 'r1', additional_credits: 10, resumed_runs: 0 });
  assert.equal(zero.resumed_runs, 0);
  assert.throws(() => parseTaskBudgetExtension({ task_id: 'r1', additional_credits: 0, resumed_runs: 0 }), ContractError);
  assert.throws(() => parseTaskBudgetExtension({ task_id: 'r1', additional_credits: 1.5, resumed_runs: 0 }), ContractError);
  assert.throws(() => parseTaskBudgetExtension({ task_id: 'r1', additional_credits: 10, resumed_runs: -1 }), ContractError);
  assert.throws(() => parseTaskBudgetExtension({ task_id: '', additional_credits: 10, resumed_runs: 0 }), ContractError);
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test packages/contracts/test/mobile-task-budget.test.ts`
Expected: FAIL——`parseTaskBudgetFacts` 不存在（import 报错：模块没有该导出）。

- [ ] **Step 3: 写最小实现**

创建 `packages/contracts/src/mobile/task-budget.ts`：

```ts
import { ContractError } from '../index.ts';

/**
 * Task Budget wire 契约（T09 #39）。四个数字全部是 millionths-of-a-Credit 的
 * 安全整数（与既有 additional_credits wire 单位一致，commercial.Credits 定义见
 * internal/modules/commercial/amount.go:11）；remaining 与三数的算术一致性由
 * 解析器强制——序列化缺陷 fail-closed，客户端永不展示幻影数字。delegated 与
 * paused 清单来自根预算行聚合（子 Run 预占天然计入根行）；can_extend 是服务端
 * 对「账单权威 OR 任务 Owner」的判定投影，grant 协作者恒 false。
 */
export interface TaskBudgetWireFacts {
  task_id: string;
  root_run_id: string;
  limit_credits: number;
  used_credits: number;
  held_credits: number;
  remaining_credits: number;
  deadline?: string;
  delegated_run_ids: string[];
  paused_run_ids: string[];
  can_extend: boolean;
}

export interface TaskBudgetWireExtension {
  task_id: string;
  additional_credits: number;
  resumed_runs: number;
}

function nonEmptyString(value: unknown, field: string): string {
  if (typeof value !== 'string' || value.trim() === '') {
    throw new ContractError(field, 'expected a non-empty string');
  }
  return value;
}

function microCredits(value: unknown, field: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value)) {
    throw new ContractError(field, 'expected a safe integer (millionths of a Credit)');
  }
  return value;
}

function idList(value: unknown, field: string): string[] {
  if (!Array.isArray(value)) throw new ContractError(field, 'expected an array of run ids');
  return value.map((entry) => nonEmptyString(entry, field));
}

export function parseTaskBudgetFacts(value: unknown): TaskBudgetWireFacts {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError('task budget', 'expected an object');
  }
  const row = value as Record<string, unknown>;
  const limit = microCredits(row.limit_credits, 'limit_credits');
  const used = microCredits(row.used_credits, 'used_credits');
  const held = microCredits(row.held_credits, 'held_credits');
  const remaining = microCredits(row.remaining_credits, 'remaining_credits');
  if (remaining !== limit - used - held) {
    throw new ContractError('remaining_credits', `expected ${limit - used - held} (limit - used - held), got ${remaining}`);
  }
  if (used < 0 || held < 0) {
    throw new ContractError('credits', 'used/held must be non-negative');
  }
  if (row.deadline !== undefined && typeof row.deadline !== 'string') {
    throw new ContractError('deadline', 'must be a string when present');
  }
  if (typeof row.can_extend !== 'boolean') {
    throw new ContractError('can_extend', 'expected a boolean');
  }
  const base: TaskBudgetWireFacts = {
    task_id: nonEmptyString(row.task_id, 'task_id'),
    root_run_id: nonEmptyString(row.root_run_id, 'root_run_id'),
    limit_credits: limit,
    used_credits: used,
    held_credits: held,
    remaining_credits: remaining,
    delegated_run_ids: idList(row.delegated_run_ids, 'delegated_run_ids'),
    paused_run_ids: idList(row.paused_run_ids, 'paused_run_ids'),
    can_extend: row.can_extend,
  };
  return row.deadline === undefined ? base : { ...base, deadline: row.deadline };
}

export function parseTaskBudgetExtension(value: unknown): TaskBudgetWireExtension {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError('task budget extension', 'expected an object');
  }
  const row = value as Record<string, unknown>;
  const additional = microCredits(row.additional_credits, 'additional_credits');
  if (additional <= 0) {
    throw new ContractError('additional_credits', 'must be positive');
  }
  const resumed = microCredits(row.resumed_runs, 'resumed_runs');
  if (resumed < 0) {
    throw new ContractError('resumed_runs', 'must be non-negative');
  }
  return { task_id: nonEmptyString(row.task_id, 'task_id'), additional_credits: additional, resumed_runs: resumed };
}
```

`packages/contracts/src/index.ts` 在 `parseInteractionWithRun` 导出块（行 661-662）后追加：

```ts
export { parseTaskBudgetFacts, parseTaskBudgetExtension } from './mobile/task-budget.ts';
export type { TaskBudgetWireFacts, TaskBudgetWireExtension } from './mobile/task-budget.ts';
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test packages/contracts/test/mobile-task-budget.test.ts && pnpm run test:shared 2>&1 | tail -5`
Expected: 新测试 PASS；`test:shared`（含 `packages/contracts/test/mobile-*.test.ts` 全量 glob）不出现新失败。

- [ ] **Step 5: 提交**

```bash
git add packages/contracts/src/mobile/task-budget.ts packages/contracts/src/index.ts packages/contracts/test/mobile-task-budget.test.ts
git commit -m "feat(contracts): task budget wire contract with four-number arithmetic consistency (T09 #39)"
```

---

### Task 7: mobile-core——task-budget 深模块与 Task Office 接线

**Files:**
- Create: `packages/mobile-core/src/task-office/task-budget.ts`
- Create: `packages/mobile-core/src/task-office/in-memory-task-budget.ts`
- Modify: `packages/mobile-core/src/task-office/task-office.ts`（Ports 1 行 + 接口 2 方法 + 工厂装配；当前 HEAD 行 167-186/188-260/261-）
- Modify: `packages/mobile-core/src/task-office/task-office-errors.ts`（错误码联合 +1 值）
- Modify: `packages/mobile-core/src/index.ts`（末尾追加导出块）
- Test: `packages/mobile-core/src/task-office/task-budget.test.ts`

**Interfaces:**
- Consumes: `ScopeLease`（`../runtime/types.ts`）与 `leaseActive`（`../runtime/scope-lease.ts`，#32 包内 seam）；`TaskOfficePorts.lease()/newRequestId?`（`task-office.ts:167-186,290-295`）；`TaskOfficeError`（`task-office-errors.ts`）；#38 `attention-inbox.ts` 的跨包契约码模式（`error.code` 字符串）。
- Produces（Task 8/9/10 消费，签名逐字）：
  - `interface TaskBudgetFacts { taskId: string; rootRunId: string; limitCredits: number; usedCredits: number; heldCredits: number; remainingCredits: number; deadline?: string; delegatedRunIds: string[]; pausedRunIds: string[]; canExtend: boolean }`
  - `interface TaskBudgetExtendInput { taskId: string; additionalCredits: number }`
  - `interface TaskBudgetExtendReceipt { additionalCredits: number; resumedRuns: number }`
  - `interface TaskBudgetBackendPort { facts(taskId: string): Promise<TaskBudgetFacts>; extend(input: { taskId: string; additionalCredits: number; idempotencyKey: string }): Promise<TaskBudgetExtendReceipt> }`
  - `type TaskBudgetErrorCode = 'TASK_BUDGET_SCOPE_CHANGED' | 'TASK_BUDGET_INVALID_INPUT' | 'TASK_BUDGET_FORBIDDEN' | 'TASK_BUDGET_NOT_FOUND' | 'TASK_BUDGET_INSUFFICIENT' | 'TASK_BUDGET_EXPIRED' | 'TASK_BUDGET_BACKEND'`；`class TaskBudgetError extends Error { constructor(readonly code: TaskBudgetErrorCode, options?: { cause?: unknown }) }`
  - `function createTaskBudgetOps(deps: { backend: TaskBudgetBackendPort; lease(): ScopeLease | undefined; newIdempotencyKey?: () => string }): { budget(taskId: string): Promise<TaskBudgetFacts>; extendBudget(input: TaskBudgetExtendInput): Promise<TaskBudgetExtendReceipt> }`
  - `createScenarioTaskBudgetBackend(script)`（场景 Adapter，测试用）
  - TaskOffice 增量：`TaskOfficePorts.budget?: TaskBudgetBackendPort`；`TaskOffice.budget(taskId: string): Promise<TaskBudgetFacts>` 与 `extendBudget(input: TaskBudgetExtendInput): Promise<TaskBudgetExtendReceipt>`（缺端口 fail closed：`TaskOfficeError('TASK_OFFICE_BUDGET_UNAVAILABLE')`，新错误码入 `TaskOfficeErrorCode` 联合）。

- [ ] **Step 1: 写失败测试**

创建 `packages/mobile-core/src/task-office/task-budget.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import {
  createTaskBudgetOps, TaskBudgetError,
  createScenarioTaskBudgetBackend, type TaskBudgetBackendPort, type TaskBudgetFacts,
} from './task-budget.ts';
import { createTaskOffice, TaskOfficeError } from './task-office.ts';
import { createScenarioTaskBackend } from './in-memory-task-backend.ts';

function leased() {
  const revocable = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.test', userId: 'user-1', tenantId: '7' });
  return { revocable, lease: revocable.asScopeLease() };
}

const facts: TaskBudgetFacts = {
  taskId: 'r1', rootRunId: 'r1',
  limitCredits: 1000, usedCredits: 400, heldCredits: 100, remainingCredits: 500,
  delegatedRunIds: ['c1'], pausedRunIds: ['r1'], canExtend: true,
};

function opsWith(backend: TaskBudgetBackendPort, leaseRef: { lease?: ScopeLease }) {
  return createTaskBudgetOps({ backend, lease: () => leaseRef.lease, newIdempotencyKey: () => `idem-${Math.random().toString(36).slice(2)}` });
}

test('budget() reads the four numbers through the backend under a live lease', async () => {
  const backend = createScenarioTaskBudgetBackend({ facts });
  const { lease } = leased();
  const ops = opsWith(backend, { lease });
  const row = await ops.budget('r1');
  assert.equal(row.remainingCredits, 500);
  assert.deepEqual(row.pausedRunIds, ['r1']);
});

test('extendBudget keeps ONE idempotency key per logical extension and clears it after success', async () => {
  const seen: string[] = [];
  let failFirst = true;
  const backend: TaskBudgetBackendPort = {
    facts: async () => facts,
    extend: async (input) => {
      seen.push(input.idempotencyKey);
      if (failFirst) { failFirst = false; throw Object.assign(new Error('flaky transport'), { name: 'ApiError', status: 500 }); }
      return { additionalCredits: input.additionalCredits, resumedRuns: 2 };
    },
  };
  const { lease } = leased();
  const ops = opsWith(backend, { lease });
  await assert.rejects(() => ops.extendBudget({ taskId: 'r1', additionalCredits: 10 }), (error: unknown) => error instanceof TaskBudgetError && error.code === 'TASK_BUDGET_BACKEND');
  const receipt = await ops.extendBudget({ taskId: 'r1', additionalCredits: 10 });
  assert.equal(receipt.resumedRuns, 2);
  assert.equal(seen.length, 2);
  assert.equal(seen[0], seen[1], 'retry of the same logical extension reuses the key (exactly-once server-side)');
  const receipt2 = await ops.extendBudget({ taskId: 'r1', additionalCredits: 10 });
  assert.equal(seen.length, 3);
  assert.notEqual(seen[2], seen[0], 'a NEW logical action gets a fresh key');
});

test('wire-coded refusals map to typed budget errors and never look like success', async () => {
  for (const [code, expected] of [
    ['TASK_BUDGET_FORBIDDEN', 'TASK_BUDGET_FORBIDDEN'],
    ['TASK_BUDGET_NOT_FOUND', 'TASK_BUDGET_NOT_FOUND'],
    ['TASK_BUDGET_INSUFFICIENT', 'TASK_BUDGET_INSUFFICIENT'],
    ['TASK_BUDGET_EXPIRED', 'TASK_BUDGET_EXPIRED'],
    ['TASK_BUDGET_INVALID_INPUT', 'TASK_BUDGET_INVALID_INPUT'],
  ] as const) {
    const backend: TaskBudgetBackendPort = {
      facts: async () => { throw codedError(code); },
      extend: async () => { throw codedError(code); },
    };
    const { lease } = leased();
    const ops = opsWith(backend, { lease });
    await assert.rejects(() => ops.budget('r1'), (error: unknown) => error instanceof TaskBudgetError && error.code === expected, code);
    await assert.rejects(() => ops.extendBudget({ taskId: 'r1', additionalCredits: 10 }), (error: unknown) => error instanceof TaskBudgetError && error.code === expected, code);
  }
});

function codedError(code: string): Error {
  const error = new Error(code);
  (error as unknown as { code?: string }).code = code;
  return error;
}

test('a revoked lease fails closed before dispatch and rejects late results (scope guard)', async () => {
  const { revocable, lease } = leased();
  let dispatched = 0;
  const backend: TaskBudgetBackendPort = {
    facts: async () => { dispatched += 1; return facts; },
    extend: async () => { dispatched += 1; return { additionalCredits: 10, resumedRuns: 0 }; },
  };
  const leaseRef = { lease: lease as ScopeLease | undefined };
  const ops = opsWith(backend, leaseRef);
  revocable.revoke();
  await assert.rejects(() => ops.budget('r1'), (error: unknown) => error instanceof TaskBudgetError && error.code === 'TASK_BUDGET_SCOPE_CHANGED');
  await assert.rejects(() => ops.extendBudget({ taskId: 'r1', additionalCredits: 10 }), (error: unknown) => error instanceof TaskBudgetError && error.code === 'TASK_BUDGET_SCOPE_CHANGED');
  assert.equal(dispatched, 0);
  // 迟到成功不越 scope 原样返回：dispatch 进行中撤销 lease → SCOPE_CHANGED。
  const again = leased();
  const slow: TaskBudgetBackendPort = {
    facts: async () => { await new Promise((resolve) => setTimeout(resolve, 5)); return facts; },
    extend: async () => ({ additionalCredits: 10, resumedRuns: 0 }),
  };
  const ops2 = createTaskBudgetOps({ backend: slow, lease: () => again.lease });
  const pending = ops2.budget('r1');
  again.revocable.revoke();
  await assert.rejects(() => pending, (error: unknown) => error instanceof TaskBudgetError && error.code === 'TASK_BUDGET_SCOPE_CHANGED');
});

test('invalid input is rejected locally with zero backend calls', async () => {
  let calls = 0;
  const backend: TaskBudgetBackendPort = {
    facts: async () => { calls += 1; return facts; },
    extend: async () => { calls += 1; return { additionalCredits: 1, resumedRuns: 0 }; },
  };
  const { lease } = leased();
  const ops = opsWith(backend, { lease });
  for (const bad of [0, -5, 1.5, NaN]) {
    await assert.rejects(() => ops.extendBudget({ taskId: 'r1', additionalCredits: bad }),
      (error: unknown) => error instanceof TaskBudgetError && error.code === 'TASK_BUDGET_INVALID_INPUT', `credits=${String(bad)}`);
  }
  await assert.rejects(() => ops.extendBudget({ taskId: '  ', additionalCredits: 10 }),
    (error: unknown) => error instanceof TaskBudgetError && error.code === 'TASK_BUDGET_INVALID_INPUT');
  await assert.rejects(() => ops.budget(''), (error: unknown) => error instanceof TaskBudgetError && error.code === 'TASK_BUDGET_INVALID_INPUT');
  assert.equal(calls, 0);
});

test('TaskOffice exposes budget()/extendBudget() and fails closed without the port', async () => {
  const office = createTaskOffice({ backend: createScenarioTaskBackend(), lease: () => leased().lease });
  await assert.rejects(() => office.budget('r1'), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_BUDGET_UNAVAILABLE');
  await assert.rejects(() => office.extendBudget({ taskId: 'r1', additionalCredits: 10 }),
    (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_BUDGET_UNAVAILABLE');

  const wired = createTaskOffice({
    backend: createScenarioTaskBackend(),
    lease: () => leased().lease,
    budget: createScenarioTaskBudgetBackend({ facts }),
  });
  const row = await wired.budget('r1');
  assert.equal(row.remainingCredits, 500);
  const receipt = await wired.extendBudget({ taskId: 'r1', additionalCredits: 10 });
  assert.equal(receipt.additionalCredits, 10);
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/task-office/task-budget.test.ts`
Expected: FAIL——`./task-budget.ts` 模块不存在（Cannot find module）。

- [ ] **Step 3: 写最小实现**

创建 `packages/mobile-core/src/task-office/task-budget.ts`：

```ts
import { leaseActive } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';

/**
 * Task Budget 深模块（T09 #39，module-seams §5「预算扩展」归 Task Office 所有）：
 * - 四数字读（预计/已用/预占/剩余，根预算行聚合含委派 Run）与授权扩额；
 * - 幂等纪律（对齐 Web TaskBudget.tsx:38-40）：每个「待完成的逻辑扩额」持有一个
 *   idempotency key——失败（含 typed 拒绝前的传输失败）保留同键重试，成功后清除，
 *   下一次用户动作取新键；服务端 exactly-once per key，重试永不加倍；
 * - scope 围栏：每次提交前与完成后检查 lease（切租户/换部署/登出即拒，迟到成功
 *   不越 scope 原样返回）；token 经 authorizedRequest 通道，不入本模块任何返回值；
 * - wire 错误经跨包契约码（api-client remote 写入 error.code，#38 INTERACTION_* 先例）
 *   翻译为 typed TaskBudgetError；扩额是纯预算决定，永不携带外部操作授权。
 */

export interface TaskBudgetFacts {
  taskId: string;
  rootRunId: string;
  limitCredits: number;
  usedCredits: number;
  heldCredits: number;
  remainingCredits: number;
  deadline?: string;
  delegatedRunIds: string[];
  pausedRunIds: string[];
  canExtend: boolean;
}

export interface TaskBudgetExtendInput { taskId: string; additionalCredits: number }
export interface TaskBudgetExtendReceipt { additionalCredits: number; resumedRuns: number }

export interface TaskBudgetBackendPort {
  facts(taskId: string): Promise<TaskBudgetFacts>;
  extend(input: { taskId: string; additionalCredits: number; idempotencyKey: string }): Promise<TaskBudgetExtendReceipt>;
}

export type TaskBudgetErrorCode =
  | 'TASK_BUDGET_SCOPE_CHANGED'
  | 'TASK_BUDGET_INVALID_INPUT'
  | 'TASK_BUDGET_FORBIDDEN'
  | 'TASK_BUDGET_NOT_FOUND'
  | 'TASK_BUDGET_INSUFFICIENT'
  | 'TASK_BUDGET_EXPIRED'
  | 'TASK_BUDGET_BACKEND';

const TASK_BUDGET_WIRE_CODES = new Set([
  'TASK_BUDGET_FORBIDDEN', 'TASK_BUDGET_NOT_FOUND', 'TASK_BUDGET_INSUFFICIENT',
  'TASK_BUDGET_EXPIRED', 'TASK_BUDGET_INVALID_INPUT',
]);

export class TaskBudgetError extends Error {
  constructor(readonly code: TaskBudgetErrorCode, options?: { cause?: unknown }) {
    super(code, options);
    this.name = 'TaskBudgetError';
  }
}

function errorCodeOf(error: unknown): string | undefined {
  if (typeof error !== 'object' || error === null) return undefined;
  const code = (error as { code?: unknown }).code;
  return typeof code === 'string' ? code : undefined;
}

function toBudgetError(error: unknown): TaskBudgetError {
  if (error instanceof TaskBudgetError) return error;
  const code = errorCodeOf(error);
  if (code !== undefined && TASK_BUDGET_WIRE_CODES.has(code)) {
    return new TaskBudgetError(code as TaskBudgetErrorCode, { cause: error });
  }
  return new TaskBudgetError('TASK_BUDGET_BACKEND', { cause: error });
}

export function createTaskBudgetOps(deps: {
  backend: TaskBudgetBackendPort;
  lease(): ScopeLease | undefined;
  newIdempotencyKey?: () => string;
}): { budget(taskId: string): Promise<TaskBudgetFacts>; extendBudget(input: TaskBudgetExtendInput): Promise<TaskBudgetExtendReceipt> } {
  let pendingKey: string | undefined;
  const nextKey = (): string => {
    if (deps.newIdempotencyKey) return deps.newIdempotencyKey();
    if (typeof globalThis.crypto?.randomUUID === 'function') return globalThis.crypto.randomUUID();
    throw new TaskBudgetError('TASK_BUDGET_INVALID_INPUT', { cause: new Error('newIdempotencyKey port is required on platforms without crypto.randomUUID') });
  };
  const requireLease = (): ScopeLease => {
    const lease = deps.lease();
    if (lease === undefined || !leaseActive(lease)) throw new TaskBudgetError('TASK_BUDGET_SCOPE_CHANGED');
    return lease;
  };
  return {
    async budget(taskId) {
      const id = taskId.trim();
      if (id === '') throw new TaskBudgetError('TASK_BUDGET_INVALID_INPUT');
      const lease = requireLease();
      try {
        const facts = await deps.backend.facts(id);
        if (!leaseActive(lease)) throw new TaskBudgetError('TASK_BUDGET_SCOPE_CHANGED');
        return facts;
      } catch (error) {
        if (error instanceof TaskBudgetError) throw error;
        throw toBudgetError(error);
      }
    },
    async extendBudget(input) {
      const id = input.taskId.trim();
      const credits = input.additionalCredits;
      if (id === '') throw new TaskBudgetError('TASK_BUDGET_INVALID_INPUT');
      if (!Number.isSafeInteger(credits) || credits <= 0) throw new TaskBudgetError('TASK_BUDGET_INVALID_INPUT');
      const lease = requireLease();
      if (pendingKey === undefined) pendingKey = nextKey();
      const key = pendingKey;
      try {
        const receipt = await deps.backend.extend({ taskId: id, additionalCredits: credits, idempotencyKey: key });
        if (!leaseActive(lease)) throw new TaskBudgetError('TASK_BUDGET_SCOPE_CHANGED');
        pendingKey = undefined; // 成功才清除：失败重试沿用同键（服务端 exactly-once）
        return receipt;
      } catch (error) {
        if (error instanceof TaskBudgetError) throw error;
        throw toBudgetError(error);
      }
    },
  };
}
```

创建 `packages/mobile-core/src/task-office/in-memory-task-budget.ts`：

```ts
import type { TaskBudgetBackendPort, TaskBudgetExtendReceipt, TaskBudgetFacts } from './task-budget.ts';

/** 场景 Adapter（测试/演示）：可脚本化 facts、extend 应答与拒绝码。 */
export interface ScenarioTaskBudgetScript {
  facts: TaskBudgetFacts;
  extend?: (input: { taskId: string; additionalCredits: number; idempotencyKey: string }) =>
    Promise<TaskBudgetExtendReceipt> | TaskBudgetExtendReceipt;
}

export function createScenarioTaskBudgetBackend(script: ScenarioTaskBudgetScript): TaskBudgetBackendPort {
  const appliedKeys = new Set<string>();
  const limitByTask = new Map<string, number>([[script.facts.taskId, script.facts.limitCredits]]);
  return {
    async facts(taskId) {
      const base = script.facts;
      if (taskId !== base.taskId) {
        const error = new Error('TASK_BUDGET_NOT_FOUND');
        (error as unknown as { code?: string }).code = 'TASK_BUDGET_NOT_FOUND';
        throw error;
      }
      return { ...base, limitCredits: limitByTask.get(taskId) ?? base.limitCredits };
    },
    async extend(input) {
      if (appliedKeys.has(input.idempotencyKey)) {
        return { additionalCredits: input.additionalCredits, resumedRuns: 0 }; // exactly-once per key
      }
      appliedKeys.add(input.idempotencyKey);
      if (script.extend) return await script.extend(input);
      limitByTask.set(input.taskId, (limitByTask.get(input.taskId) ?? 0) + input.additionalCredits);
      return { additionalCredits: input.additionalCredits, resumedRuns: script.facts.pausedRunIds.length };
    },
  };
}
```

`task-office-errors.ts`：`TaskOfficeErrorCode` 联合追加 `'TASK_OFFICE_BUDGET_UNAVAILABLE'`（在 `'TASK_OFFICE_LEGACY_UNAVAILABLE';` 之前插入一行 `| 'TASK_OFFICE_BUDGET_UNAVAILABLE'`）。

`task-office.ts`：
1. import 区追加：`import { createTaskBudgetOps } from './task-budget.ts';` 与 `import type { TaskBudgetBackendPort, TaskBudgetExtendInput, TaskBudgetExtendReceipt, TaskBudgetFacts } from './task-budget.ts';`
2. `TaskOfficePorts`（行 167-186）`legacy?: LegacyTaskBackendPort;` 之后追加：

```ts
  /** T09（#39）预算端口：四数字读 + 授权扩额。缺失时 budget()/extendBudget()
   *  fail closed（TASK_OFFICE_BUDGET_UNAVAILABLE）——预算扩展归 Task Office 所有
   *  （module-seams §5.1），幂等键由 office 内 task-budget 深模块持有。 */
  budget?: TaskBudgetBackendPort;
```

3. `TaskOffice` 接口（行 188 起）`decide(...)` 声明之后追加两方法声明：

```ts
  /** T09（#39）：一个 Task 的预算四数字（预计/已用/预占/剩余，根行聚合含委派 Run）。 */
  budget(taskId: string): Promise<TaskBudgetFacts>;
  /** T09（#39）：授权扩额（幂等键模块内保持，失败重试同键）；receipt 携带本次唤醒的 run 数。 */
  extendBudget(input: TaskBudgetExtendInput): Promise<TaskBudgetExtendReceipt>;
```

4. `createTaskOffice` 工厂（行 261 起，`lease`/`newRequestId` 等装配之后）追加：

```ts
  const budgetOps = ports.budget === undefined ? undefined : createTaskBudgetOps({
    backend: ports.budget,
    lease: ports.lease,
    newIdempotencyKey: ports.newRequestId,
  });
```

返回对象中追加：

```ts
    budget(taskId: string): Promise<TaskBudgetFacts> {
      if (budgetOps === undefined) return Promise.reject(new TaskOfficeError('TASK_OFFICE_BUDGET_UNAVAILABLE'));
      return budgetOps.budget(taskId);
    },
    extendBudget(input: TaskBudgetExtendInput): Promise<TaskBudgetExtendReceipt> {
      if (budgetOps === undefined) return Promise.reject(new TaskOfficeError('TASK_OFFICE_BUDGET_UNAVAILABLE'));
      return budgetOps.extendBudget(input);
    },
```

`packages/mobile-core/src/index.ts` 末尾追加：

```ts
export { createTaskBudgetOps, TaskBudgetError } from './task-office/task-budget.ts';
export type {
  TaskBudgetBackendPort, TaskBudgetErrorCode, TaskBudgetExtendInput, TaskBudgetExtendReceipt, TaskBudgetFacts,
} from './task-office/task-budget.ts';
export { createScenarioTaskBudgetBackend } from './task-office/in-memory-task-budget.ts';
export type { ScenarioTaskBudgetScript } from './task-office/in-memory-task-budget.ts';
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test packages/mobile-core/src/task-office/task-budget.test.ts && pnpm exec tsx --test packages/mobile-core/src/task-office/task-office.test.ts && pnpm exec tsx --test packages/mobile-core/src/task-office/task-office-start.test.ts`
Expected: 新测试 PASS；既有 task-office 系列不回归。

- [ ] **Step 5: 提交**

```bash
git add packages/mobile-core/src/task-office/task-budget.ts packages/mobile-core/src/task-office/in-memory-task-budget.ts packages/mobile-core/src/task-office/task-budget.test.ts packages/mobile-core/src/task-office/task-office.ts packages/mobile-core/src/task-office/task-office-errors.ts packages/mobile-core/src/index.ts
git commit -m "feat(mobile-core): task budget ops behind Task Office with idempotency-key retention and scope guard (T09 #39)"
```

---

### Task 8: api-client——`createMobileTaskBudgetRemote`

**Files:**
- Create: `packages/api-client/src/mobile/task-budget.ts`
- Modify: `packages/api-client/package.json`（exports 追加 `"./mobile/task-budget": "./src/mobile/task-budget.ts"`，置于 `"./mobile/task-office"` 行之后）
- Test: `packages/api-client/src/mobile/task-budget.test.ts`

**Interfaces:**
- Consumes: `ClientRequest`（`../client.ts:38-47`）、`requireDeploymentOrigin`（`./deployment-origin.ts`）、`ApiError`（`../errors.ts:24-36`）、contracts 的 `parseTaskBudgetFacts`/`parseTaskBudgetExtension`（Task 6）、materials.ts 的 `unwrap` 信封解包模式（`materials.ts:41-50`——本文件自带同语义 unwrap，不跨文件导入私有函数）。
- Produces: `createMobileTaskBudgetRemote(options: { origin: string; request: (input: ClientRequest) => Promise<unknown> }): MobileTaskBudgetRemote`，本文件自带语义行类型 `MobileTaskBudgetFacts`（与 mobile-core `TaskBudgetFacts` 字段逐字同构）与 `MobileTaskBudgetExtendReceipt`（同构 `TaskBudgetExtendReceipt`）——api-client 不依赖 mobile-core（沿 materials 先例）；`MobileTaskBudgetRemote` 与 mobile-core `TaskBudgetBackendPort` 结构逐字一致（可赋值性由 Task 9 的 `pnpm --filter @weknora/mobile typecheck` 证明）；wire 错误按 Task 7 的跨包契约码写入 `error.code`。

- [ ] **Step 1: 写失败测试**

创建 `packages/api-client/src/mobile/task-budget.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createMobileTaskBudgetRemote } from './task-budget.ts';
import type { ClientRequest } from '../client.ts';

const factsData = {
  task_id: 'r1', root_run_id: 'r1',
  limit_credits: 1000, used_credits: 400, held_credits: 100, remaining_credits: 500,
  deadline: '2026-09-24T12:00:00Z',
  delegated_run_ids: ['c1'], paused_run_ids: ['r1'], can_extend: true,
};

function remoteWith(handler: (input: ClientRequest) => Promise<unknown>) {
  return createMobileTaskBudgetRemote({ origin: 'https://weknora.example.test', request: handler });
}

function apiError(status: number, code: string): Error {
  return Object.assign(new Error(code), { name: 'ApiError', status, code });
}

test('facts() GETs the budget endpoint and maps the wire row to semantic fields', async () => {
  let seen: ClientRequest | undefined;
  const remote = remoteWith(async (input) => {
    seen = input;
    return { success: true, data: factsData };
  });
  const row = await remote.facts('r1');
  assert.equal(seen?.method, 'GET');
  assert.equal(seen?.path, '/api/v1/commercial/tasks/r1/budget');
  assert.equal(row.remainingCredits, 500);
  assert.deepEqual(row.delegatedRunIds, ['c1']);
  assert.equal(row.canExtend, true);
});

test('extend() POSTs the idempotent body and returns the receipt', async () => {
  let seen: ClientRequest | undefined;
  const remote = remoteWith(async (input) => {
    seen = input;
    return { success: true, data: { task_id: 'r1', additional_credits: 10, resumed_runs: 2 } };
  });
  const receipt = await remote.extend({ taskId: 'r1', additionalCredits: 10, idempotencyKey: 'k-1' });
  assert.equal(seen?.method, 'POST');
  assert.equal(seen?.path, '/api/v1/commercial/tasks/r1/budget/extend');
  assert.deepEqual(seen?.body, { additional_credits: 10, idempotency_key: 'k-1' });
  assert.equal(receipt.resumedRuns, 2);
});

test('wire refusals translate to cross-package contract codes on error.code', async () => {
  const cases: Array<[number, string, string]> = [
    [403, 'BUDGET_FORBIDDEN', 'TASK_BUDGET_FORBIDDEN'],
    [403, 'BUDGET_UNAUTHORIZED', 'TASK_BUDGET_FORBIDDEN'],
    [404, 'TASK_BUDGET_NOT_FOUND', 'TASK_BUDGET_NOT_FOUND'],
    [409, 'BUDGET_INSUFFICIENT', 'TASK_BUDGET_INSUFFICIENT'],
    [409, 'TASK_BUDGET_EXPIRED', 'TASK_BUDGET_EXPIRED'],
    [400, 'INVALID_REQUEST', 'TASK_BUDGET_INVALID_INPUT'],
    [400, 'INVALID_BUDGET_REQUEST', 'TASK_BUDGET_INVALID_INPUT'],
  ];
  for (const [status, wireCode, expected] of cases) {
    const remote = remoteWith(async () => { throw apiError(status, wireCode); });
    await assert.rejects(() => remote.facts('r1'), (error: unknown) =>
      (error as unknown as { code?: string }).code === expected, `${status} ${wireCode}`);
    await assert.rejects(() => remote.extend({ taskId: 'r1', additionalCredits: 10, idempotencyKey: 'k' }), (error: unknown) =>
      (error as unknown as { code?: string }).code === expected, `${status} ${wireCode}`);
  }
  // 未知形态（如 500 非 ApiError）原样透传：不吞成假成功。
  const remote = remoteWith(async () => { throw new Error('socket exploded'); });
  await assert.rejects(() => remote.facts('r1'), /socket exploded/);
});

test('malformed success payloads fail closed instead of fabricating numbers', async () => {
  for (const data of [{}, { ...factsData, remaining_credits: 499 }, null]) {
    const remote = remoteWith(async () => ({ success: true, data }));
    await assert.rejects(() => remote.facts('r1'));
  }
  const badEnvelope = remoteWith(async () => ({ success: false, error: { code: 'X', message: 'm' } }));
  await assert.rejects(() => badEnvelope.facts('r1'), /success/);
});

test('the remote rejects an invalid deployment origin at construction', () => {
  assert.throws(() => createMobileTaskBudgetRemote({ origin: 'http://insecure.example.test', request: async () => ({}) }));
  assert.throws(() => createMobileTaskBudgetRemote({ origin: 'https://example.test/path', request: async () => ({}) }));
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test packages/api-client/src/mobile/task-budget.test.ts`
Expected: FAIL——`Cannot find module './task-budget.ts'`。

- [ ] **Step 3: 写最小实现**

创建 `packages/api-client/src/mobile/task-budget.ts`：

```ts
import type { ClientRequest } from '../client.ts';
import { ApiError } from '../errors.ts';
import { parseTaskBudgetExtension, parseTaskBudgetFacts } from '@weknora/contracts';
import { requireDeploymentOrigin } from './deployment-origin.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface TaskBudgetRemoteOptions {
  /** 部署 Origin：构造即强校验（绝对 HTTPS、无 path/query/fragment、无内嵌凭据）。 */
  origin: string;
  /** 授权通道（MobileRuntime.authorizedRequest 或测试替身）；本适配器不新建传输、不持有 token。 */
  request: Request;
}

/** 语义行（api-client 不依赖 mobile-core——依赖方向沿 materials 先例；与 mobile-core
 * TaskBudgetBackendPort 的 TaskBudgetFacts/TaskBudgetExtendReceipt 结构逐字一致，
 * 结构可赋值由 composition 装配处的 apps/mobile typecheck 证明）。 */
export interface MobileTaskBudgetFacts {
  taskId: string;
  rootRunId: string;
  limitCredits: number;
  usedCredits: number;
  heldCredits: number;
  remainingCredits: number;
  deadline?: string;
  delegatedRunIds: string[];
  pausedRunIds: string[];
  canExtend: boolean;
}

export interface MobileTaskBudgetExtendReceipt {
  additionalCredits: number;
  resumedRuns: number;
}

/** 与 mobile-core TaskBudgetBackendPort 结构逐字一致（结构可赋值由 apps/mobile typecheck 证明）。 */
export interface MobileTaskBudgetRemote {
  facts(taskId: string): Promise<MobileTaskBudgetFacts>;
  extend(input: { taskId: string; additionalCredits: number; idempotencyKey: string }): Promise<MobileTaskBudgetExtendReceipt>;
}

function unwrap(value: unknown): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new Error('task budget response must be a success envelope');
  }
  const envelope = value as { success?: unknown; data?: unknown };
  if (envelope.success !== true || !Object.prototype.hasOwnProperty.call(envelope, 'data')) {
    throw new Error('task budget response.success must be true with data');
  }
  const data = envelope.data;
  if (typeof data !== 'object' || data === null || Array.isArray(data)) {
    throw new Error('task budget response data must be an object');
  }
  return data as Record<string, unknown>;
}

/** 跨包契约码（沿 #38 INTERACTION_* 先例）：mobile-core 按 error.code 分类，不得改名。 */
function coded(cause: unknown, code: string): Error {
  const translated = new Error(code, { cause });
  (translated as unknown as { code?: string }).code = code;
  return translated;
}

function translate(error: unknown): unknown {
  if (!(error instanceof ApiError)) return error;
  switch (error.status) {
    case 400: return coded(error, 'TASK_BUDGET_INVALID_INPUT');
    case 403: return coded(error, 'TASK_BUDGET_FORBIDDEN');
    case 404: return coded(error, 'TASK_BUDGET_NOT_FOUND');
    case 409: return error.code === 'TASK_BUDGET_EXPIRED'
      ? coded(error, 'TASK_BUDGET_EXPIRED')
      : coded(error, 'TASK_BUDGET_INSUFFICIENT');
    default: return error;
  }
}

export function createMobileTaskBudgetRemote(options: TaskBudgetRemoteOptions): MobileTaskBudgetRemote {
  requireDeploymentOrigin(options.origin);
  const request = options.request;
  return {
    async facts(taskId) {
      const path = `/api/v1/commercial/tasks/${encodeURIComponent(taskId)}/budget`;
      try {
        const row = parseTaskBudgetFacts(unwrap(await request({ method: 'GET', path })));
        return {
          taskId: row.task_id,
          rootRunId: row.root_run_id,
          limitCredits: row.limit_credits,
          usedCredits: row.used_credits,
          heldCredits: row.held_credits,
          remainingCredits: row.remaining_credits,
          ...(row.deadline === undefined ? {} : { deadline: row.deadline }),
          delegatedRunIds: row.delegated_run_ids,
          pausedRunIds: row.paused_run_ids,
          canExtend: row.can_extend,
        };
      } catch (error) {
        throw translate(error);
      }
    },
    async extend(input) {
      const path = `/api/v1/commercial/tasks/${encodeURIComponent(input.taskId)}/budget/extend`;
      try {
        const row = parseTaskBudgetExtension(unwrap(await request({
          method: 'POST', path,
          body: { additional_credits: input.additionalCredits, idempotency_key: input.idempotencyKey },
        })));
        return { additionalCredits: row.additional_credits, resumedRuns: row.resumed_runs };
      } catch (error) {
        throw translate(error);
      }
    },
  };
}
```

`packages/api-client/package.json` exports 在 `"./mobile/task-office"` 行后追加：

```json
    "./mobile/task-budget": "./src/mobile/task-budget.ts",
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test packages/api-client/src/mobile/task-budget.test.ts && pnpm run test:shared 2>&1 | tail -5`
Expected: 新测试 PASS（含 `packages/api-client/src/mobile/*.test.ts` glob 全量无新失败）。

- [ ] **Step 5: 提交**

```bash
git add packages/api-client/src/mobile/task-budget.ts packages/api-client/src/mobile/task-budget.test.ts packages/api-client/package.json
git commit -m "feat(api-client): createMobileTaskBudgetRemote on the authorized channel with typed refusal codes (T09 #39)"
```

---

### Task 9: apps/mobile——预算屏、路由与接线

**Files:**
- Create: `apps/mobile/src/task-budget-view.ts`
- Create: `apps/mobile/src/task-budget-view.test.ts`
- Create: `apps/mobile/src/screens/TaskBudgetScreen.tsx`
- Create: `apps/mobile/src/app/tasks/budget.tsx`
- Modify: `apps/mobile/src/screens/TaskDetailScreen.tsx`（`TaskDetailScreenProps` 加 1 可选 prop + 1 按钮，当前 HEAD 行 11/20/68）
- Modify: `apps/mobile/src/app/tasks/detail.tsx`（`TaskDetailRouteLifecycle` 加 1 prop + 默认导出 1 处透传，当前 HEAD 行 11/41-53）
- Modify: `apps/mobile/src/composition.ts`（`taskOfficeFor` 工厂内 +1 行，当前 HEAD 行 172-193）
- Modify: `apps/mobile/src/app-smoke.test.tsx`（末尾追加 2 测试）

**Interfaces:**
- Consumes: Task 7 的 `TaskOffice.budget()/extendBudget()` 与 `TaskBudgetFacts`/`TaskBudgetError`；`activeTaskOffice()`（`composition.ts`）；`#46 materials` 的 Screen/入口模式（`TaskDetailScreen.tsx:68`）；`MATERIAL_ERROR_COPY`→`TASK_BUDGET_COPY` 模式（`materials-view.ts:13`）；Task 8 的 remote（经 composition 注入）。
- Produces: `/tasks/budget?taskId=` 路由与 `TaskBudgetScreen`（四数字 + 委派/暂停 + 独立扩额操作）；`createTaskBudgetController`（`/tasks/budget` 路由的视图层状态机）。〔终审修订 #4：原文「Task 10 复用其状态机做集成断言的视图层对照」与实现不符——Task 10 的集成证据直接对照 `office.budget()` 的 wire 事实投影，未引用该控制器；已按实际产出修订。〕

- [ ] **Step 1: 写失败测试**

创建 `apps/mobile/src/task-budget-view.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createTaskBudgetController, TASK_BUDGET_COPY } from './task-budget-view.ts';
import { TaskBudgetError } from '@weknora/mobile-core';

type OfficeBudgetFace = Pick<import('@weknora/mobile-core').TaskOffice, 'budget' | 'extendBudget'>;

function officeWith(overrides: Partial<Record<'budget' | 'extendBudget', unknown>> = {}): OfficeBudgetFace {
  return {
    budget: overrides.budget as OfficeBudgetFace['budget'] ??
      (async () => ({
        taskId: 'r1', rootRunId: 'r1',
        limitCredits: 1000, usedCredits: 400, heldCredits: 100, remainingCredits: 500,
        delegatedRunIds: ['c1'], pausedRunIds: ['r1'], canExtend: true,
      })),
    extendBudget: overrides.extendBudget as OfficeBudgetFace['extendBudget'] ??
      (async () => ({ additionalCredits: 10, resumedRuns: 2 })),
  };
}

test('refresh loads the four numbers and the paused/delegated projection', async () => {
  const controller = createTaskBudgetController(officeWith(), { taskId: 'r1' });
  await controller.refresh();
  const state = controller.state();
  assert.equal(state.loading, false);
  assert.equal(state.error, undefined);
  assert.equal(state.facts?.limitCredits, 1000);
  assert.equal(state.facts?.remainingCredits, 500);
  assert.equal(state.facts?.pausedRunIds.length, 1);
  controller.dispose();
});

test('extend succeeds once, shows the resume count, and refetches fresh numbers', async () => {
  const calls: number[] = [];
  const office = officeWith({
    budget: async () => ({
      taskId: 'r1', rootRunId: 'r1',
      limitCredits: 1000 + calls.length * 10, usedCredits: 400, heldCredits: 100,
      remainingCredits: 500 + calls.length * 10,
      delegatedRunIds: [], pausedRunIds: calls.length === 0 ? ['r1'] : [], canExtend: true,
    }),
    extendBudget: async (input: { additionalCredits: number }) => {
      calls.push(input.additionalCredits);
      return { additionalCredits: input.additionalCredits, resumedRuns: 2 };
    },
  });
  const controller = createTaskBudgetController(office, { taskId: 'r1' });
  await controller.refresh();
  await controller.extend(10);
  const state = controller.state();
  assert.deepEqual(calls, [10]);
  assert.equal(state.message, '已追加 10 额度，恢复 2 个运行；追加预算不等于外部操作批准。');
  assert.equal(state.facts?.limitCredits, 1010);
  assert.equal(state.facts?.pausedRunIds.length, 0, 'refetch reflects the resumed state');
  controller.dispose();
});

test('a typed refusal surfaces honest copy per code and keeps the last facts', async () => {
  const office = officeWith({
    extendBudget: async () => { throw new TaskBudgetError('TASK_BUDGET_FORBIDDEN'); },
  });
  const controller = createTaskBudgetController(office, { taskId: 'r1' });
  await controller.refresh();
  await controller.extend(10);
  const state = controller.state();
  assert.equal(state.error, TASK_BUDGET_COPY.TASK_BUDGET_FORBIDDEN);
  assert.equal(state.facts?.limitCredits, 1000, 'refused extend never mutates local numbers');
  controller.dispose();
});

test('invalid local input is rejected without touching the office', async () => {
  let extendCalls = 0;
  const office = officeWith({
    extendBudget: async () => { extendCalls += 1; return { additionalCredits: 1, resumedRuns: 0 }; },
  });
  const controller = createTaskBudgetController(office, { taskId: 'r1' });
  await controller.refresh();
  await controller.extend(0);
  await controller.extend(1.5);
  assert.equal(extendCalls, 0);
  assert.equal(controller.state().error, TASK_BUDGET_COPY.TASK_BUDGET_INVALID_INPUT);
  controller.dispose();
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/task-budget-view.test.ts`
Expected: FAIL——`Cannot find module './task-budget-view.ts'`。

- [ ] **Step 3: 写最小实现**

创建 `apps/mobile/src/task-budget-view.ts`：

```ts
import type { TaskBudgetErrorCode, TaskBudgetFacts, TaskOffice } from '@weknora/mobile-core';
import { TaskBudgetError } from '@weknora/mobile-core';

/** T09（#39）预算屏状态机：四数字 + 委派/暂停投影 + 独立扩额操作（对齐 Web
 * TaskBudget.tsx 的「预算决定与外部操作审批相互独立」纪律——文案不承诺任何外部授权）。 */
export interface TaskBudgetViewState {
  loading: boolean;
  facts?: TaskBudgetFacts;
  error?: string;
  message?: string;
  extending: boolean;
}

export const TASK_BUDGET_COPY: Record<TaskBudgetErrorCode, string> = {
  TASK_BUDGET_SCOPE_CHANGED: '登录空间已切换，请重新打开任务预算。',
  TASK_BUDGET_INVALID_INPUT: '追加额度必须为正整数。',
  TASK_BUDGET_FORBIDDEN: '只有任务所有者或获授权的账单管理员可以增加上限。',
  TASK_BUDGET_NOT_FOUND: '未找到该任务的预算。',
  TASK_BUDGET_INSUFFICIENT: '空间余额不足以追加该额度。',
  TASK_BUDGET_EXPIRED: '预算有效期已过，追加不延长截止时间。',
  TASK_BUDGET_BACKEND: '预算服务暂不可用，请稍后重试。',
};

export interface TaskBudgetController {
  state(): TaskBudgetViewState;
  subscribe(listener: (state: TaskBudgetViewState) => void): () => void;
  refresh(): Promise<void>;
  extend(additionalCredits: number): Promise<void>;
  dispose(): void;
}

export function createTaskBudgetController(
  office: Pick<TaskOffice, 'budget' | 'extendBudget'>,
  input: { taskId: string },
): TaskBudgetController {
  let state: TaskBudgetViewState = { loading: true, extending: false };
  const listeners = new Set<(next: TaskBudgetViewState) => void>();
  const emit = (): void => {
    const snapshot = { ...state };
    for (const listener of listeners) listener(snapshot);
  };
  const failWith = (error: unknown): void => {
    state = { ...state, loading: false, extending: false, error: error instanceof TaskBudgetError ? TASK_BUDGET_COPY[error.code] : '预算读取失败，请稍后重试。' };
    emit();
  };
  return {
    state: () => ({ ...state }),
    subscribe(listener) { listeners.add(listener); return () => { listeners.delete(listener); }; },
    async refresh() {
      state = { ...state, loading: true, error: undefined };
      emit();
      try {
        const facts = await office.budget(input.taskId);
        state = { ...state, loading: false, facts };
      } catch (error) {
        failWith(error);
      }
    },
    async extend(additionalCredits: number) {
      if (!Number.isSafeInteger(additionalCredits) || additionalCredits <= 0) {
        state = { ...state, error: TASK_BUDGET_COPY.TASK_BUDGET_INVALID_INPUT };
        emit();
        return;
      }
      state = { ...state, extending: true, error: undefined, message: undefined };
      emit();
      try {
        const receipt = await office.extendBudget({ taskId: input.taskId, additionalCredits });
        const facts = await office.budget(input.taskId); // 权威重取，绝不本地加数
        state = {
          ...state, extending: false, facts,
          message: `已追加 ${receipt.additionalCredits} 额度，恢复 ${receipt.resumedRuns} 个运行；追加预算不等于外部操作批准。`,
        };
      } catch (error) {
        failWith(error);
      }
    },
    dispose() { listeners.clear(); },
  };
}
```

创建 `apps/mobile/src/screens/TaskBudgetScreen.tsx`：

```tsx
import { Button, CheckBox, Text, TextInput, View } from 'react-native';
import type { TaskBudgetViewState } from '../task-budget-view.ts';

export interface TaskBudgetScreenProps {
  taskId: string;
  state: TaskBudgetViewState;
  /** 独立确认（默认不勾选）：预算追加与外部写操作审批相互独立，永不自动勾选。 */
  confirmed: boolean;
  credits: string;
  onToggleConfirmed: (value: boolean) => void;
  onChangeCredits: (value: string) => void;
  onRefresh: () => void;
  onExtend: () => void;
}

/** 任务预算屏（T09 #39）：预计/已用/预占/剩余 四数分立（Story 58），委派与达限暂停
 * 如实投影；扩额入口在 canExtend=false 时禁用（AC1 的 UI 投影——权限权威在服务端）。 */
export function TaskBudgetScreen(props: TaskBudgetScreenProps) {
  const { state } = props;
  const facts = state.facts;
  return (
    <View>
      <Text accessibilityRole="header">任务预算</Text>
      <Text>预算追加与外部写操作审批相互独立：追加预算不会、也不能授权任何外部发送或删除操作。</Text>
      {state.message !== undefined ? <Text accessibilityRole="status">{state.message}</Text> : null}
      {state.error !== undefined ? <Text accessibilityRole="alert">{state.error}</Text> : null}
      {state.loading && facts === undefined ? <Text>正在读取预算…</Text> : null}
      {facts !== undefined ? (
        <View>
          <Text testID="budget-paused">
            {facts.pausedRunIds.length > 0 ? '预算已达限，运行已持久暂停；授权扩额后将恢复同一运行。' : '没有因预算暂停的运行。'}
          </Text>
          <Text testID="budget-limit">预计（批准上限）：{facts.limitCredits} 额度</Text>
          <Text testID="budget-used">已用：{facts.usedCredits} 额度</Text>
          <Text testID="budget-held">预占：{facts.heldCredits} 额度</Text>
          <Text testID="budget-remaining">剩余：{facts.remainingCredits} 额度</Text>
          <Text testID="budget-delegated">委派运行：{facts.delegatedRunIds.length > 0 ? facts.delegatedRunIds.join(', ') : '无'}</Text>
          <Button title="刷新" onPress={props.onRefresh} />
          <Text>追加额度（正整数，含委派运行的合计上限）</Text>
          <TextInput
            testID="budget-credits"
            keyboardType="numeric"
            value={props.credits}
            onChangeText={props.onChangeCredits}
          />
          <CheckBox
            testID="budget-confirm"
            value={props.confirmed}
            onValueChange={props.onToggleConfirmed}
          />
          <Text>我确认追加此任务预算（仅预算决定，不包含任何外部发送授权）</Text>
        </View>
      ) : null}
      {!state.loading && facts === undefined ? <Text>暂无预算数据。</Text> : null}
      <Button
        testID="budget-extend"
        title={state.extending ? '正在追加…' : '追加预算'}
        disabled={!facts?.canExtend || state.extending || !props.confirmed}
        onPress={props.onExtend}
      />
      {!facts?.canExtend && facts !== undefined ? <Text>当前身份不能追加预算：只有任务所有者或获授权的账单管理员可以增加上限。</Text> : null}
    </View>
  );
}
```

（expo stub 环境缺 `TextInput`/`CheckBox` 导出时，在 `app-smoke.test.tsx` 的 `NATIVE_MODULE_STUBS['react-native']` 追加这两个键（保留既有键，参照 #46 追加 `Image` 的先例）。Screen 不 import api-client/contracts（module-seams §10）。）

创建 `apps/mobile/src/app/tasks/budget.tsx`：

```tsx
import { useEffect, useRef, useState } from 'react';
import { useLocalSearchParams } from 'expo-router';
import { TaskBudgetScreen } from '../../screens/TaskBudgetScreen.tsx';
import { createTaskBudgetController, type TaskBudgetController, type TaskBudgetViewState } from '../../task-budget-view.ts';
import { activeTaskOffice } from '../../composition.ts';

/** /tasks/budget?taskId=.. 挂载生命周期宿主：控制器在 effect 内创建、卸载即 dispose——
 * 与 /tasks/detail 同一模式；只消费 Task Office Interface（module-seams §5.2/§10）。 */
export function TaskBudgetRouteLifecycle({ taskId }: { taskId: string }) {
  const [state, setState] = useState<TaskBudgetViewState>({ loading: true, extending: false });
  const [credits, setCredits] = useState('100');
  const [confirmed, setConfirmed] = useState(false);
  const controllerRef = useRef<TaskBudgetController | undefined>(undefined);
  useEffect(() => {
    const office = activeTaskOffice();
    if (office === undefined || taskId === '') {
      setState({ loading: false, extending: false, error: '请先登录并激活空间，再查看任务预算。' });
      return;
    }
    const controller = createTaskBudgetController(office, { taskId });
    controllerRef.current = controller;
    setState(controller.state());
    const unsubscribe = controller.subscribe(setState);
    void controller.refresh();
    return () => {
      unsubscribe();
      controller.dispose();
      controllerRef.current = undefined;
    };
  }, [taskId]);
  return (
    <TaskBudgetScreen
      taskId={taskId}
      state={state}
      confirmed={confirmed}
      credits={credits}
      onToggleConfirmed={setConfirmed}
      onChangeCredits={setCredits}
      onRefresh={() => { void controllerRef.current?.refresh(); }}
      onExtend={() => { void controllerRef.current?.extend(Number(credits)); }}
    />
  );
}

/** Expo Router 文件路由：/tasks/budget?taskId=..。 */
export default function TaskBudgetRoute() {
  const params = useLocalSearchParams<{ taskId?: string }>();
  return <TaskBudgetRouteLifecycle taskId={String(params.taskId ?? '')} />;
}
```

`apps/mobile/src/screens/TaskDetailScreen.tsx`：
- `TaskDetailScreenProps`（行 11 附近）追加 `onOpenBudget?: () => void;`
- 组件签名解构加入 `onOpenBudget`；
- 「任务材料」按钮（行 68）之后追加一行：

```tsx
      {onOpenBudget !== undefined && <Button title="任务预算" onPress={onOpenBudget} />}
```

`apps/mobile/src/app/tasks/detail.tsx`：
- `TaskDetailRouteLifecycle` 参数（行 11）追加 `onOpenBudget?: () => void`；
- 末尾 `<TaskDetailScreen ... />` 透传 `onOpenBudget={onOpenBudget}`；
- 默认导出（行 41-53）追加：

```tsx
      onOpenBudget={() => { router.push({ pathname: '/tasks/budget', params: { taskId: String(params.taskId ?? '') } }); }}
```

`apps/mobile/src/composition.ts`：`taskOfficeFor` 工厂的 `createTaskOffice({...})` 入参（行 176-192）`legacy: ...` 之后追加一行（文件头补 import `import { createMobileTaskBudgetRemote } from '@weknora/api-client/mobile/task-budget';`）：

```ts
      budget: createMobileTaskBudgetRemote({ origin, request: (input) => activeRuntime.authorizedRequest(input) }),
```

`apps/mobile/src/app-smoke.test.tsx` 末尾追加：

```tsx
test('the task budget route keeps an Expo Router screen consuming the Task Office interface only', async () => {
  const route = await import('./app/tasks/budget.tsx');
  assert.equal(typeof route.default, 'function', 'src/app/tasks/budget.tsx must default-export the Expo Router screen');
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  for (const relative of ['screens/TaskBudgetScreen.tsx', 'task-budget-view.ts', 'app/tasks/budget.tsx']) {
    const source = readFileSync(join(here, relative), 'utf8');
    assert.equal(/@weknora\/(api-client|contracts)/.test(source), false, `${relative} must consume the Task Office Interface only (module-seams §10)`);
  }
});

test('composition wires the concrete budget remote into the Task Office (source-level, parallel-batch guard)', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const source = readFileSync(join(here, 'composition.ts'), 'utf8');
  assert.match(source, /budget:\s*createMobileTaskBudgetRemote/, 'taskOfficeFor must assemble the budget remote on the authorized channel');
});
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`
Expected: 全部 PASS / typecheck 无输出（含既有 154 项无回归；typecheck 同时证明 `MobileTaskBudgetRemote` 与 `TaskBudgetBackendPort` 结构可赋值）。

- [ ] **Step 5: 提交**

```bash
git add apps/mobile/src/task-budget-view.ts apps/mobile/src/task-budget-view.test.ts apps/mobile/src/screens/TaskBudgetScreen.tsx apps/mobile/src/app/tasks/budget.tsx apps/mobile/src/screens/TaskDetailScreen.tsx apps/mobile/src/app/tasks/detail.tsx apps/mobile/src/composition.ts apps/mobile/src/app-smoke.test.tsx
git commit -m "feat(mobile): task budget screen with four distinct numbers, paused banner and gated extend (T09 #39)"
```

---

### Task 10: apps/mobile——真实 HTTP 集成证据（AC3，opt-in）

**Files:**
- Create: `apps/mobile/src/task-budget-integration-smoke.ts`
- Create: `apps/mobile/src/task-budget-integration-smoke.test.ts`

**Interfaces:**
- Consumes: Task 7 的 `createTaskOffice({ budget: createMobileTaskBudgetRemote(...) })`；Task 8 的 remote；`createMobileRuntime`/`createInMemoryCredentialStore`（`@weknora/mobile-core`）；`disallowedDeploymentHost`（`./runtime-integration-smoke.ts:118` 起，#32 主机防线——拒绝 localhost/环回/私网/链路本地/保留地址）；`material-integration-smoke.ts` 的 opt-in 范式（`materialIntegrationConfig`）。
- Produces: `TaskBudgetIntegrationConfig`（`{enabled:true; deploymentOrigin; email; password; extendBudget:boolean; extendCredits:number}` | `{enabled:false; disposition:'skip'|'invalid'; reason}`）；`TaskBudgetIntegrationEvidence`（`{deploymentOrigin; budgetFacts:'read'|'no-tasks'|'failed'; fourNumbersDistinct?:boolean; remainingConsistent?:boolean; pausedRunCount?:number（终审修订 #3：原 pausedListed?:boolean 为同义反复，改为计数承载「达限暂停清单非空/为空」语义）; extend?:'skipped'|'extended'|'refused'|'failed'; limitRaisedBy?:number; replayNeverDoubled?:boolean; failure?; commandTimestamp}`）；`taskBudgetIntegrationConfig(env)`、`runTaskBudgetIntegration(config)`、`emitTaskBudgetIntegrationEvidence(evidence, emit)`、测试套内 opt-in 钩子（终审修订 #1：仿 material-integration-smoke，具备 `WEKNORA_MOBILE_TEST_*` 环境的常规套件运行自动产出预算端到端证据）。

- [ ] **Step 1: 写失败测试**

创建 `apps/mobile/src/task-budget-integration-smoke.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { taskBudgetIntegrationConfig } from './task-budget-integration-smoke.ts';

test('taskBudgetIntegrationConfig skips without credentials and never fabricates enabled', () => {
  const config = taskBudgetIntegrationConfig({});
  assert.equal(config.enabled, false);
  assert.equal(config.disposition, 'skip');
});

test('taskBudgetIntegrationConfig validates the HTTPS origin and host line', () => {
  for (const url of ['http://weknora.example.test', 'https://user:pw@weknora.example.test', 'https://127.0.0.1:8080', 'https://weknora.example.test/path']) {
    const config = taskBudgetIntegrationConfig({
      WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: url,
      WEKNORA_MOBILE_TEST_EMAIL: 'e@example.test',
      WEKNORA_MOBILE_TEST_PASSWORD: 'p',
    });
    assert.equal(config.enabled, false, url);
    assert.equal(config.disposition, 'invalid', url);
  }
});

test('the extend arm is opt-in via WEKNORA_MOBILE_TEST_EXTEND_BUDGET=1 with a positive credit amount', () => {
  const config = taskBudgetIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://weknora.example.test',
    WEKNORA_MOBILE_TEST_EMAIL: 'e@example.test',
    WEKNORA_MOBILE_TEST_PASSWORD: 'p',
  });
  assert.equal(config.enabled, true);
  assert.equal(config.extendBudget, false);
  const extended = taskBudgetIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://weknora.example.test',
    WEKNORA_MOBILE_TEST_EMAIL: 'e@example.test',
    WEKNORA_MOBILE_TEST_PASSWORD: 'p',
    WEKNORA_MOBILE_TEST_EXTEND_BUDGET: '1',
    WEKNORA_MOBILE_TEST_EXTEND_CREDITS: '5',
  });
  assert.equal(extended.enabled, true);
  assert.ok(extended.enabled);
  assert.equal((extended as { extendBudget: boolean }).extendBudget, true);
  assert.equal((extended as { extendCredits: number }).extendCredits, 5);
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/task-budget-integration-smoke.test.ts`
Expected: FAIL——`Cannot find module './task-budget-integration-smoke.ts'`。

- [ ] **Step 3: 写最小实现**

创建 `apps/mobile/src/task-budget-integration-smoke.ts`（对齐 `material-integration-smoke.ts` 的自包含 opt-in 语义）：

```ts
import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createMobileTaskBudgetRemote } from '@weknora/api-client/mobile/task-budget';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createInMemoryCredentialStore, createMobileRuntime, createTaskOffice } from '@weknora/mobile-core';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';

export type TaskBudgetIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string; extendBudget: boolean; extendCredits: number }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface TaskBudgetIntegrationEvidence {
  deploymentOrigin: string;
  budgetFacts: 'read' | 'no-tasks' | 'failed';
  /** 四数分立（Story 58）与算术一致性（remaining === limit-used-held）在真实 wire 上的投影。 */
  fourNumbersDistinct?: boolean;
  remainingConsistent?: boolean;
  /** 达限暂停清单的实况计数（终审修复 #3：parse 已保证数组，>0 即存在暂停 Run、
   * ===0 即清单为空——比 Array.isArray 复述类型更有区分度）。 */
  pausedRunCount?: number;
  extend?: 'skipped' | 'extended' | 'refused' | 'failed';
  limitRaisedBy?: number;
  replayNeverDoubled?: boolean;
  failure?: string;
  commandTimestamp: string;
}

/** opt-in 语义与 T04/T05/T16 相同：真实 Deployment（HTTPS 公网主机）+ 测试账号；
 * 扩额臂额外需 WEKNORA_MOBILE_TEST_EXTEND_BUDGET=1（真实改账动作必须显式开门）。 */
export function taskBudgetIntegrationConfig(env: Record<string, string | undefined>): TaskBudgetIntegrationConfig {
  const deploymentOrigin = env.WEKNORA_MOBILE_TEST_DEPLOYMENT_URL?.trim();
  const email = env.WEKNORA_MOBILE_TEST_EMAIL?.trim();
  const password = env.WEKNORA_MOBILE_TEST_PASSWORD;
  if (!deploymentOrigin || !email || !password) {
    return { enabled: false, disposition: 'skip', reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD' };
  }
  let parsed: URL;
  try { parsed = new URL(deploymentOrigin); } catch {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL is not an absolute URL' };
  }
  if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.pathname !== '/' || parsed.search || parsed.hash) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must be a credential-free HTTPS origin' };
  }
  const hostRejection = disallowedDeploymentHost(parsed.hostname, 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL');
  if (hostRejection) return { enabled: false, disposition: 'invalid', reason: hostRejection };
  const rawCredits = env.WEKNORA_MOBILE_TEST_EXTEND_CREDITS?.trim() ?? '1';
  const extendCredits = Number(rawCredits);
  if (!Number.isSafeInteger(extendCredits) || extendCredits <= 0) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_EXTEND_CREDITS must be a positive integer' };
  }
  return {
    enabled: true, deploymentOrigin: parsed.origin, email, password,
    extendBudget: env.WEKNORA_MOBILE_TEST_EXTEND_BUDGET === '1',
    extendCredits,
  };
}

/**
 * 真实 JSON transport + 授权通道 + 具体 Remote Adapter + Task Office 编排：读取
 * 第一个任务的预算四数字（含委派/暂停投影）；opt-in 扩额臂执行一次真实追加 +
 * 同键幂等重放断言不加倍。账号无任务时如实记 'no-tasks'；任何步骤异常 →
 * budgetFacts:'failed' + failure 摘要（无凭据字段），从不 reject。
 */
export async function runTaskBudgetIntegration(config: Extract<TaskBudgetIntegrationConfig, { enabled: true }>): Promise<TaskBudgetIntegrationEvidence> {
  const evidence: TaskBudgetIntegrationEvidence = { deploymentOrigin: config.deploymentOrigin, budgetFacts: 'failed', commandTimestamp: new Date().toISOString() };
  const fetcher: FetchLike = (input, init) => fetch(input, init as RequestInit);
  const runtime = createMobileRuntime({
    credentialStore: createInMemoryCredentialStore(),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    remoteFor(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      return createMobileRuntimeRemote({ origin, request: client.request });
    },
    authorizedTransport(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      return (input, accessToken) => client.request({ ...input, headers: { ...input.headers, authorization: `Bearer ${accessToken}` } });
    },
  });
  try {
    const snapshot = await runtime.signIn({ deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' }, email: config.email, password: config.password });
    if (snapshot.surface !== 'authorized' || !snapshot.deployment) return evidence;

    const office = createTaskOffice({
      backend: createTaskOfficeRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) }),
      lease: () => runtime.scopeLease(),
      budget: createMobileTaskBudgetRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) }),
    });
    const page = await office.tasks({});
    if (page.items.length === 0) { evidence.budgetFacts = 'no-tasks'; return evidence; }
    const taskId = page.items[0]!.taskId;

    const facts = await office.budget(taskId);
    evidence.budgetFacts = 'read';
    evidence.fourNumbersDistinct =
      facts.limitCredits !== facts.usedCredits || facts.usedCredits !== facts.heldCredits || facts.heldCredits !== facts.remainingCredits;
    evidence.remainingConsistent = facts.remainingCredits === facts.limitCredits - facts.usedCredits - facts.heldCredits;
    evidence.pausedRunCount = facts.pausedRunIds.length;

    if (!config.extendBudget) { evidence.extend = 'skipped'; return evidence; }
    if (!facts.canExtend) { evidence.extend = 'refused'; return evidence; }

    const before = facts.limitCredits;
    await office.extendBudget({ taskId, additionalCredits: config.extendCredits });
    // 同键幂等重放走 remote 层（office 的键在成功后清除，第二次 office 调用是新键、
    // 会真实加额——不能用作重放断言）：固定 key 两次 POST，断言只计一次。
    const remote = createMobileTaskBudgetRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) });
    const replayKey = `smoke-replay-${Date.now()}`;
    await remote.extend({ taskId, additionalCredits: config.extendCredits, idempotencyKey: replayKey });
    await remote.extend({ taskId, additionalCredits: config.extendCredits, idempotencyKey: replayKey });
    const after = await office.budget(taskId);
    evidence.extend = 'extended';
    evidence.limitRaisedBy = after.limitCredits - before;
    // office 一次（+N）+ remote 同键两次（只计一次 +N）= 恰好 +2N：重放不加倍。
    evidence.replayNeverDoubled = evidence.limitRaisedBy === config.extendCredits * 2;
    return evidence;
  } catch (error) {
    evidence.budgetFacts = 'failed';
    evidence.failure = error instanceof Error ? error.message : String(error);
    return evidence;
  } finally {
    runtime.dispose();
  }
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitTaskBudgetIntegrationEvidence(evidence: TaskBudgetIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test apps/mobile/src/task-budget-integration-smoke.test.ts && pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`
Expected: 配置契约测试 PASS；全套无回归。真环境执行（本地产出端到端证据，blocked-env 见计划头声明）：

```bash
WEKNORA_MOBILE_TEST_DEPLOYMENT_URL=https://<真实部署> WEKNORA_MOBILE_TEST_EMAIL=<账号> \
WEKNORA_MOBILE_TEST_PASSWORD=<密码> WEKNORA_MOBILE_TEST_EXTEND_BUDGET=1 \
WEKNORA_MOBILE_TEST_EXTEND_CREDITS=1 \
pnpm exec tsx -e "import('./apps/mobile/src/task-budget-integration-smoke.ts').then(async (m) => { const config = m.taskBudgetIntegrationConfig(process.env); if (config.enabled) m.emitTaskBudgetIntegrationEvidence(await m.runTaskBudgetIntegration(config), (r) => console.log(r)); else console.log(JSON.stringify(config)); })"
```

Expected（有环境时）：`budgetFacts:'read'`、`fourNumbersDistinct:true`、`remainingConsistent:true`、`extend:'extended'`、`replayNeverDoubled:true`；无环境时输出 `{enabled:false,...}` 如实 skip，不得伪造。

- [ ] **Step 5: 提交**

```bash
git add apps/mobile/src/task-budget-integration-smoke.ts apps/mobile/src/task-budget-integration-smoke.test.ts
git commit -m "test(mobile): opt-in real-HTTP task budget evidence with same-key replay guard (AC3, T09 #39)"
```

---

### Task 11: Web——`TaskBudget` 补「剩余」派生行（Story 58 四数分立）

**Files:**
- Modify: `apps/web/src/commercial/TaskBudget.tsx`（导出纯函数 + 列表 1 行；当前 HEAD 行 5-12/84-88）
- Test: `apps/web/src/commercial/TaskBudget.test.ts`（Create）

**Interfaces:**
- Consumes: 既有 `TaskBudgetSnapshot { limit; used; held }`（`TaskBudget.tsx:5-12`）与「不编造数字」纪律（snapshot 缺失显示「暂无数据」）。
- Produces: `export function remainingCreditsOf(snapshot: { limit: number; used: number; held: number }): number`（`limit - used - held`，派生算术非编造数据）；Web 面板四数分立（批准上限/已用/占用/剩余）。

- [ ] **Step 1: 写失败测试**

创建 `apps/web/src/commercial/TaskBudget.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { remainingCreditsOf } from './TaskBudget.tsx';

test('remainingCreditsOf derives the fourth number from the three facts', () => {
  assert.equal(remainingCreditsOf({ limit: 1000, used: 400, held: 100 }), 500);
  assert.equal(remainingCreditsOf({ limit: 100, used: 0, held: 0 }), 100);
  assert.equal(remainingCreditsOf({ limit: 100, used: 100, held: 0 }), 0);
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @weknora/web exec node --import tsx --test src/commercial/TaskBudget.test.ts`
Expected: FAIL——`remainingCreditsOf` 未导出（SyntaxError/undefined export）。

- [ ] **Step 3: 写最小实现**

`apps/web/src/commercial/TaskBudget.tsx`：
1. `TaskBudgetSnapshot` 接口后追加导出纯函数：

```tsx
/** 剩余 = 批准上限 − 已用 − 占用（Story 58 四数分立；对既有三事实的派生算术，
 * 不是新数据源——snapshot 缺失时 UI 仍显示「暂无数据」，不编造数字）。 */
export function remainingCreditsOf(snapshot: { limit: number; used: number; held: number }): number {
  return snapshot.limit - snapshot.used - snapshot.held;
}
```

2. 列表（行 84-88）「占用」行之后追加：

```tsx
        <li><strong>剩余</strong><span>{limit === undefined || used === undefined || held === undefined ? '暂无数据' : remainingCreditsOf({ limit, used, held }) + ' 额度'}</span></li>
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm --filter @weknora/web exec node --import tsx --test src/commercial/TaskBudget.test.ts && pnpm --filter @weknora/web exec node --import tsx --test 'src/commercial/*.test.ts'`
Expected: PASS，commercial 目录既有测试（BillingPage/order-state/refund-state）无回归。

- [ ] **Step 5: 提交**

```bash
git add apps/web/src/commercial/TaskBudget.tsx apps/web/src/commercial/TaskBudget.test.ts
git commit -m "feat(web): derive the remaining-credits line on TaskBudget (Story 58, T09 #39)"
```

---

## 计划级验证（testCommand）

在 worktree 根（`.worktrees/issue30-sweep`）执行（覆盖本计划全部新增测试，定向到受影响包/目录，不触发已知迁移冲突套件）：

```bash
go test ./internal/application/service/ -run 'TestDurableWaitReasonClassifiesBudgetExhaustionAsDurablePark|TestWorkerParksBudgetExhaustedRunDurable|TestCraftBudgetResumeAfterExtensionDoesNotDoubleCharge' -count=1 && go test ./internal/application/repository/ -run 'TestRequeueBudgetPausedRuns' -count=1 && go test ./internal/handler/ -run 'TestGetTaskBudget|TestExtendTaskBudget' -count=1 && pnpm exec tsx --test packages/contracts/test/mobile-task-budget.test.ts && pnpm exec tsx --test packages/mobile-core/src/task-office/task-budget.test.ts && pnpm exec tsx --test packages/mobile-core/src/task-office/task-office.test.ts && pnpm exec tsx --test packages/api-client/src/mobile/task-budget.test.ts && pnpm exec tsx --test apps/mobile/src/task-budget-view.test.ts && pnpm exec tsx --test apps/mobile/src/task-budget-integration-smoke.test.ts && pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck && pnpm --filter @weknora/web exec node --import tsx --test src/commercial/TaskBudget.test.ts
```

补充回归门（建议在批次验收时一并跑）：`pnpm run test:shared 2>&1 | tail -5`（contracts/api-client/domain glob 全量，确认无新失败）与 `go test ./internal/modules/commercial/service/commercial/ ./internal/modules/commercial/repository/commercial/ -count=1`（预算域全绿基线）。

## 自我审查记录（writing-plans 四项检查）

1. **Spec 覆盖**：AC1（Collaborator 不能扩额、服务端强制）→ Task 4（collaborator 403 且不 requeue，真实 HTTP e2e）+ Task 3（grant 读面 `can_extend=false`）+ Task 7/8（typed 拒绝映射）+ Task 9（UI 禁用投影）；服务端谓词本体已由 #42 落地（差异记录 1），本计划补端到端证据。AC2（恢复不重复历史消耗/预占/在途）→ Task 5（历史 callID 幂等重放、预占不双倍、同键扩额不加倍、拒绝补偿）+ Task 4（exactly-once 重放不加额）+ 既有 Reconcile 保留未确认结算的绿色测试引用（`budget.go:77-117`、`TestBudgetRecoveryDowngradeBlocksNewExtendButNotReconcile`）。AC3（最高稳定 Interface）→ Go：真实 sqlite 库 + 真实 store/worker/handler/route（Task 1/2/4）；移动：真实 JSON transport + 授权通道 + 具体 Remote + Task Office（Task 10，opt-in）；blocked-env 声明在计划头。Story 57（含委派的合计预算）→ 根行聚合（Task 3 delegated/paused 清单 + Task 5 子 Run 断言）。Story 58（四数分立）→ Task 3 wire + Task 6 契约 + Task 9 移动屏 + Task 11 Web。Story 59/60 → Task 1/3/4。「持久暂停不删 Workspace」→ park 复用 `waiting_user` 非终态，checkpoint 保留（`agent_run_graph.go:376-383`）。Spec「预算扩展归 Task Office」→ Task 7 经 `office.extendBudget()`。离线禁止扩额 → 不做本地队列（Global Constraints 声明）。**缺口检查：无未覆盖条款。**
2. **占位符扫描**：初稿在 Task 3（GET 实现混入无效的 `ShouldBindJSON` 两行）、Task 9（Screen 用了非法的 `<input-like />`）、Task 10（重放臂误用 office 二次调用且混入 `EXTEND_CREDENTS` 拼写变体）各发现一处撰写期草稿残留——已在定稿中**直接清除并写为正确正文**（GET 实现无 body 绑定；Screen 使用 `TextInput`/`CheckBox` 受控组件并附 stub 追加说明；重放臂走 remote 层显式同键两次 POST）。终稿全文无 TBD/TODO/「适当处理」/「类似 Task N」类占位；所有测试与实现代码完整给出。
3. **类型/签名一致性**：`RequeueBudgetPausedRuns(ctx, tenantID uint64, budgetRootRunID string) (int64, error)` 在 Task 2 定义、Task 4 消费一致；`TaskBudgetFacts`/`TaskBudgetExtendInput`/`TaskBudgetExtendReceipt`/`TaskBudgetBackendPort` 在 Task 7（mobile-core）定义，Task 8 因依赖方向（api-client 不依赖 mobile-core）自带同构语义行 `MobileTaskBudgetFacts`/`MobileTaskBudgetExtendReceipt`（字段与 mobile-core 版逐字一致，结构可赋值由 composition 装配 + apps/mobile typecheck 证明），Task 9 控制器消费 office 同名方法 `budget()/extendBudget()`；跨包契约码（`TASK_BUDGET_*`）Task 7 定义集合 = Task 8 写入集合 = Task 9 `TASK_BUDGET_COPY` 键集合；`TASK_OFFICE_BUDGET_UNAVAILABLE` 同时入 `TaskOfficeErrorCode`（Task 7）；wire 字段名（`*_credits`/`delegated_run_ids`/`paused_run_ids`/`can_extend`/`resumed_runs`）在 Task 3/4 产出与 Task 6 解析、Task 8 请求/映射逐字一致。
4. **Review Focus 落实**：五条各已在对应任务落测试（#1→Task 4/7/9；#2→Task 5/6；#3→Task 2；#4→Task 1；#5→Task 3）。

## Consumes-Produces 摘要（供同批/后续计划）

**Consumes（既有）**：#34 `authorizedRequest`/`TaskOfficePorts`；#35 `TaskDetailScreen`/`/tasks/detail`；#32 `leaseActive`/`RuntimeScopeLease`；#36 `newRequestId`；#42 扩额 Owner-or-billing 门禁、`taskRunOwner`、task_grants 授予读谓词形态、`NewAgentRunStore(db)`；#38 跨包契约码先例；#33/#41/#46 深模块 + 场景 Adapter + composition 记忆化 + opt-in 集成证据范式。

**Produces（本计划新增，后续计划可依赖）**：
1. Go：`AgentRunStore.RequeueBudgetPausedRuns(ctx, tenantID, rootRunID) (int64, error)`；达限 park 语义 `waiting_user`+`wait_reason='budget_exhausted'`（`durableWaitReason` budget 分支）；`GET /api/v1/commercial/tasks/:id/budget`（四数字 + delegated/paused + can_extend）；extend 响应 `resumed_runs`。
2. contracts：`parseTaskBudgetFacts`/`parseTaskBudgetExtension` + `TaskBudgetWireFacts`/`TaskBudgetWireExtension`（`@weknora/contracts` 根导出）。
3. mobile-core：`TaskOffice.budget(taskId)/extendBudget(input)`、`TaskOfficePorts.budget?: TaskBudgetBackendPort`、`TaskBudgetError`（7 码）、`TaskOfficeErrorCode` 新值 `TASK_OFFICE_BUDGET_UNAVAILABLE`、`createTaskBudgetOps`、`createScenarioTaskBudgetBackend`。
4. api-client：`createMobileTaskBudgetRemote`（exports `./mobile/task-budget`）。
5. apps/mobile：`/tasks/budget` 路由、`TaskBudgetScreen`、`createTaskBudgetController` + `TASK_BUDGET_COPY`、composition 装配行、`TaskBudgetIntegrationEvidence` 证据契约（opt-in `WEKNORA_MOBILE_TEST_EXTEND_BUDGET=1` + `WEKNORA_MOBILE_TEST_EXTEND_CREDITS`）。
6. Web：`remainingCreditsOf`。
