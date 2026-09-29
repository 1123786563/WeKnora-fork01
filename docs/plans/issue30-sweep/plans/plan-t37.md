# T07：运行中调整、排队下一 Run 与停止重启（Issue #37）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Task Owner 在移动端对当前 Run 发出三类显式干预——调整当前 Run（steer）、为下一 Run 排队一条指令（queue-next）、停止后重启（stop）——停止请求/停止确认/结果未知三态分别呈现，命令结果未知期间阻止冲突写 Run，所有控制命令携带真实观察的 revision，且每次干预回执呈现指令实际绑定的 Run。

**Architecture:** 服务端把命令闭集 `cancel|steer` 扩为 `cancel|steer|queue_next`，并补齐取消的保真度：`GormCancelPort` 当前只把 `agent_runs.status` 单条 UPDATE 成 `canceled`（`internal/modules/workbench/service/workbench/interaction.go:383-395`），既不写 `cancellation_requested` Run 事件（时间线看不到停止请求这一事实），也不释放 `sessions.active_agent_run_id` 会话槽（`Admit` 的槽位谓词 `active_agent_run_id IS NULL` 见 `internal/application/repository/agent_run.go:222-225`，槽残留会令同 Task 的下一 Run 准入永远 `ErrRunActive`——「停止后重启」在当前代码下无机制可用）。本计划新增 `AgentRunStore.CancelOwnedRun`（tenant+owner+revision CAS → status=canceled + `cancellation_requested` 事件 + 会话槽释放，同一事务）并由 `GormCancelPort` 委托。`queue_next` 的服务端语义经代码核定：引擎终态后排空 backlog 的机制（`admitAfterFollowUps`，`internal/application/service/agent_run_graph.go:534`）只服务 graph 执行的 `DurableRunSnapshot` 形态 Run，而 workbench 准入的 Run 快照是另一形态（`admission.go:377-380` 的简单 map）且执行走 Paseo remote dispatch（`agent_run_worker.go:117-131`），该排空对 workbench Run 不适用——因此 `queue_next` 定为「终态 Run 上经同一 `AdmissionCoordinator` 重准入下一 Run」（同 session/agent/target/workspace/budget，文本为排队指令，幂等 request_id 派生自命令 idempotency id），活动 Run 上明确 409（Spec 单写者规则），绝不静默排队。停止三态不在服务端引入无人消费的 `stopping` 状态（workbench 的 durable row 即权威，`CancelRun` 先例同为单步 CAS）：三态由客户端在事实之上推导——`requested`=202 已接受且未观察到终态、`confirmed`=快照/SSE 观察到 `canceled`（或自然终态获胜、从不改写）、`unknown`=命令投递结果未知（传输失败/5xx/502 `command_recovery_unknown`），unknown 进入模块门（阻止同句柄一切后续写意图）直到快照核对 resolves：`canceled` ⇒ 实已落地；非 canceled ⇒ 取消 CAS 不可能已落地（取消不可逆）⇒ 未落地、解除门。移动端在 `packages/mobile-core` 落 `TaskHandle.act(TaskIntent)`（module-seams §5.2 第 4 个入口的最后一块），`TaskCommandPort` 为命令 seam（`createTaskOfficeRemote` 同一对象实现 detail/stream/command 三端口，#35 先例）；Run 活动期间的 queue-next 意图由模块本地持有（单写者规则），观察到终态后发出（本地持有、非耐久——跨进程耐久属 #40 且 SQLite 选型 ADR 未决，本计划不引入任何新持久化）。

**Tech Stack:** Go 1.26（`internal/modules/workbench`、`internal/application/repository`、`internal/handler/session`、`internal/container`；golang-migrate sqlite 全量迁移 + httptest 集成测试）、TypeScript（`packages/contracts`、`packages/mobile-core`、`packages/api-client`、`apps/mobile` Expo RN）、node:test + tsx（`tsx --test`）、`pnpm --filter @weknora/mobile test/typecheck`。所有测试命令在 worktree 根（`.worktrees/issue30-sweep`）执行，前置已 `pnpm install`。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-37.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（User Stories 15/16/21/22/23、Implementation Decisions、Testing Decisions）
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§5.2 Interface「act(TaskIntent)：执行 steer、queue-next、stop、decision、share、archive 等受控意图」、§5.3 不变量、§13 Interface 测试面「Task Office：……intervention routing……stop pending」）
- ADR：`docs/adr/0004-task-is-session.md`（taskId = sessionId；queue_next 重准入同 session）、`docs/adr/0012-mobile-business-logic-lives-behind-deep-modules.md`（干预编排位于 Task Office 深模块后，Screen 不维护 revision）
- 领域术语：`CONTEXT.md`（「任务（Task）」「运行（Run）」——一个 Task 包含多个 Run；重试与追问保留同一目标与历史）
- Parent：Issue #30；Blocked by：#35（T05，已合并）、#36（T06，已合并）
- 前序批次产出（本计划 Consumes，全部在当前 HEAD 亲眼核实）：#35 的 `TaskHandle`（hydrate/view/updates/resync/close，`packages/mobile-core/src/task-office/task-detail.ts:72-78`）、`TaskDetailBackendPort`（detail/stream，`task-detail.ts:35-39`）、`createTaskDetail`（`task-detail.ts:95`）、`isTerminalRunStatus/terminalRunStatusOf`（`task-timeline.ts:36-55`）、`createTaskOfficeRemote`（`packages/api-client/src/mobile/task-office.ts:70`）、composition 的 `taskOfficeFor`（`apps/mobile/src/composition.ts:153`，`detail: remote` 同对象装配在 `:158`）、`createTaskDetailController`（`apps/mobile/src/task-detail-view.ts:33`）、`TaskDetailScreen`（`apps/mobile/src/screens/TaskDetailScreen.tsx:20`）、`/tasks/detail` 路由（`apps/mobile/src/app/tasks/detail.tsx`）；#36 的 `TaskOffice.start(goal)` 与 `TaskOfficePorts.intentLog/newRequestId`（`task-office.ts:167-186`）、`office.open()` 转发（`task-office.ts:460-466`）、集成冒烟先例 `task-start-integration-smoke.ts`（opt-in 环境变量 + total 化证据）；#38 的跨包契约码先例（`error.code === 'TASK_STREAM_CURSOR_EXPIRED'` / `INTERACTION_DELIVERY_UNKNOWN`，`task-office.ts:182-235`）；Go 侧 `AdmissionCoordinator.Start` 的持久幂等准入（`admission.go:278-328`）、`Service.Command` 的 cancel/steer 路由与 `expected_revision` 栅栏（`interaction.go(service):608-633`）、`writeWorkbenchCommandError`（`workbench_commands.go:120-152`）、`openWorkbenchHTTPDB`/`withIdentity` 集成测试基建（`workbench_start_integration_test.go:118-147`）、`openAdmissionConcurrencyDB`/`countingBudget`（`admission_concurrency_test.go:25-31,163-190`）。

## Global Constraints

以下为批准 Spec / ADR 的项目级约束，逐字引用，所有任务隐含遵守：

- 「21. As a Task Owner, I want to steer the current Run at a safe point, queue an instruction for the next Run, or stop and restart, so that each intervention has explicit semantics.」（mobile-ai-office-design.md · User Story 21）
- 「22. As a Task Owner, I want the app to show which Run accepted an intervention, so that I know whether it affected current or future work.」（同上 · User Story 22）
- 「23. As a Task Owner, I want a stop request distinguished from confirmed process termination, so that unknown effects are not hidden.」（同上 · User Story 23）
- 「One Task permits at most one write Run. Independent read-only delegation may run concurrently and is combined by the single writer.」（同上 · Implementation Decisions——queue_next 在活动 Run 上必须拒绝而非静默排队）
- 「Each command that can have an unknown outcome uses a durable idempotency identity. Network failure triggers lookup or reconciliation, not silent replay.」（同上 · Implementation Decisions——queue_next 以 idempotency id 派生准入 request_id；停止 unknown 以快照核对收敛，绝不盲目重发）
- 「REST submits commands and loads authoritative Snapshots.」（同上——三态推导只消费权威快照/SSE 事实）
- 「Task Office owns Home/Task projections, durable submission identity, reconciliation, Snapshot/SSE recovery, intervention, decisions, budget and Task lifecycle.」（同上）
- 「Mobile core does not depend on React Native, DOM or concrete transport. Remote and native details are injected as Adapters.」（同上）
- 「Tests target observable behavior at the highest stable Interface. Screen tests verify rendering and navigation; they do not duplicate Module internals.」（同上 · Testing Decisions）
- 「Task Office Interface tests cover durable request identity, lost acknowledgements, unknown reconciliation, Snapshot hydration, SSE gaps, cursor expiry, single-writer admission, intervention routing, decision CAS and three-dimensional state projection.」（同上）
- 「Go integration tests verify authorization, Tenant/Owner/Collaborator predicates, revision and digest CAS, idempotency, partial external success, unknown outcomes and durable checkpoints.」（同上）
- 「- act(TaskIntent)：执行 steer、queue-next、stop、decision、share、archive 等受控意图；」（mobile-module-seams.md §5.2——decision 属 #38 已交付的 office.decide，share 属后续 Issue）
- 「- cancel ACK 不等于停止完成或退款；」「- 同一 Task 不产生第二个写 Run；」「- capability 缺失和未知 schema 一律 fail closed；」「- Scope Lease 失效后丢弃迟到结果；」（mobile-module-seams.md §5.3 不变量）
- 「Screen 不调用 start、lookup、snapshot、events、interaction、command 等多个 wire 方法。Module 内部决定顺序、幂等、重连、revision 和错误呈现。」（mobile-module-seams.md §5.2）
- 「移动 AI Office 将现有 WeKnora Session 呈现为 Task，并保持 `taskId = sessionId`，不新增第二个 Task 聚合身份。」（ADR-0004——queue_next 重准入复用原 session）
- 安全约束（Mimosa）：服务端新 SQL 一律参数绑定（本计划新增 SQL 全部走 gorm 参数化/事务，无字符串拼接）；凭据只从环境变量读取，源码与测试不写入可用凭据字面量；集成冒烟的部署 origin 校验复用既有防线语义（仅 HTTPS、无内嵌凭据，`taskStartIntegrationConfig` 同款）。
- 工作流约束：严格 RED→GREEN→REFACTOR（每个任务先写失败测试、实跑确认失败、最小实现、通过、提交）；实现不与已批准 Spec 冲突，冲突时升级而非静默重设计。
- 测试归属：Go 服务语义 → `internal/modules/workbench/service/workbench`；仓储语义 → `internal/application/repository`；HTTP 集成（AC3 服务端证据）→ `internal/handler/session`；契约冻结规则 → `packages/contracts/test/mobile-execution.test.ts`；wire 适配 → `packages/api-client/src/mobile`；模块行为（AC1/AC2 主证据）→ `packages/mobile-core/src/task-office`；Screen/控制器 → `apps/mobile`；真实部署端到端 → `apps/mobile` opt-in 冒烟（无环境 t.skip，不伪造）。

**Issue #37 验收标准原文（docs/plans/issue30-sweep/issues/issue-37.md）：**

1. 「停止请求、停止确认、结果未知分别呈现。」
2. 「未知结果阻止冲突写 Run，所有控制命令带真实 revision。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

验收标准 3 的本地可验证性说明（blocked-env 声明）：「最高稳定 Interface」在本地可执行的层级为——(a) mobile-core `TaskHandle.act()` Interface 级场景测试（真实模块编排 + 场景 Adapter，AC1/AC2 主证据，Task 6）；(b) Go HTTP 级集成测试（真实全量迁移 sqlite + 真实 `AdmissionCoordinator`/`Service`/handler + httptest，Task 4，AC3 服务端证据）；(c) api-client wire 契约测试（真实序列化字节，Task 7）。真实移动端→真实部署的最终端到端沿用 T01–T06 已合并的 opt-in 真实 HTTP 模式（`WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD`，需一个真实 WeKnora Deployment + 测试账号 + 至少一个可用 Agent）：本地无此环境时 Task 9 的 live 用例 `t.skip`（**不得伪造通过**），凡具备环境的运行自动产出证据。真实引擎/LLM 执行链路（Paseo remote dispatch）不在本 Issue 验收范围（run 的执行属平台 worker，干预面只承诺 durable 事实与准入语义）。

**与调查结论的差异记录（以代码现状为准）：**

1. **迁移编号冲突（当前 worktree 测试基线红的根因，必须前置修复）**：调查称「已运行 `go test ./internal/modules/workbench/service/workbench/`（ok 13.634s）」。本计划作者在当前 HEAD 实跑该命令 → **FAIL**，根因 `duplicate migration file: 000112_task_grants.down.sql`：`migrations/sqlite/000112_agent_adoption_variants.*`（#59，commit a3132eaa0）与 `migrations/sqlite/000112_task_grants.*`（#42，commit ca9b66ee1）同号，`migrations/versioned/000191_*` 同样双文件（golang-migrate 拒绝同号不同名）。受影响测试（实跑核实）：workbench service 包 9 个 admission/notification 测试、`TestWorkbenchStartHTTPIntegrationAndIdentityIsolation`（handler/session）。Task 1 把 #59 的迁移改号为 `000113`（sqlite）/`000192`（versioned）（两号均空闲，实查 `ls migrations/sqlite | grep 000113` → 空）。引用该号的仅有两处注释（`internal/application/repository/agent_adoption_test.go:247`、`task_grant_store_test.go:141`）。
2. **调查缺口 2 暗示服务端需要「独立停止请求态」**：以代码现状核定——workbench 取消是单条原子 CAS（durable row 即权威；仓储 `CancelRun` 先例同为单步），三态在客户端呈现层成立（由 202 ack、快照终态、投递结果三个事实推导，见 Architecture），服务端不引入无人消费的 `stopping` 状态。但调查未发现的**真实缺口**是：`GormCancelPort` 不写 `cancellation_requested` 事件、不释放会话槽（`agent_run.go:552` 只有 failed/succeeded 路径清槽；canceled run 的槽仅 `CancelRun`/`DeleteSessionRuns` 清，二者均不在 workbench 命令路径上）——槽残留令「停止后重启」的同 session 准入永远 `ErrRunActive`。Task 3 修复。
3. **queue_next 语义核定**：调查称「排队下一 Run 目前仅是引擎在终态后自动排空 backlog 的行为」。实读代码：该排空（`admitAfterFollowUps`，`agent_run_graph.go:534-592`）要求父 Run 快照可解析为 `DurableRunSnapshot`（`version/query/model_id/agent_config`，`agent_run_graph.go:53-61`），而 workbench 准入写入的快照是 `{session_id, agent_id, target_id, workspace_ref, space_id, request_id, text, budget_upper, ...}` 简单 map（`admission.go:377-380`），且平台 Run 执行走 Paseo remote dispatch worker（`agent_run_worker.go:117-131`）——排空机制对 workbench Run 不适用。故 queue_next 服务端语义定为终态 Run 上经同一 `AdmissionCoordinator` 重准入（快照字段同源、幂等 request_id），活动 Run 上 409（Spec 单写者规则），不发明服务端排队存储。
4. 调查称「移动端 UI 完全缺失……无 Task/Run 干预界面」「CommandAck 回显 run_id 无客户端呈现」——属实（`TaskHandle` 无 `act`，`TaskDetailScreen` 无干预区）。Task 6/8 交付。
5. **B2-F23 延期项边界（SQLite/持久化 TaskProjectionStore 选型 ADR 未决）**：本计划**不引入任何新持久化**。Run 活动期间被模块持有的 queue-next 意图是进程内状态（App 被杀即失，重开后用户可见 Run 终态并重新排队——诚实的最小可行语义）；如执行者倾向持久化 parked 意图，那属 #40 且被该 ADR 阻塞（blocked-env/延后），不得在本计划内静默做存储选型。
6. **写计划时新发现的既有缺陷（owner 投影不一致，本计划必须修复否则交付物在生产不可用）**：命令服务的 `identity()` 取 `Principal.StorageID()`（web 用户 ⇒ `web_user:u1`，`interaction.go(service):478-488` + `principal.go:62-66`），而 `agent_runs.owner_id` 由准入的 `contextIdentity()` 以 `UserIDFromContext` 写入（原始 id `u1`，`admission.go:266-276`），且全部 workbench 读面（list `workbench_list.go:44`、read `workbench_read.go:148`）同样以 `UserIDFromContext` 为 owner 谓词——即 `GormCancelPort`/`GormSteerPort` 的 `owner_id = ?` 谓词在 web 用户（移动 App 的登录形态）下**永远不匹配自己准入的 Run**（API-key principal 下两投影恰好同值，故既有测试未暴露）。Task 2 在 `Service.Command` 引入 `runOwnerID(ctx)`（`UserIDFromContext` 优先、Principal 兜底）供三个 Run 键控命令分支使用；`Decide`/`List`（交互行，owner 恒为 storage id，`workbench_commands_test.go:92` 证实）保持 `identity()` 不变。

## Review Focus

Spec 隐含但无任务测试覆盖、最可能咬到真实用户的五类输入/失效模式（每行后在所属任务落地测试）：

1. **停止命令网络层失败（无状态码）被当作「已失败」呈现并允许立即重发**：断网时点停止，若把传输失败当确定失败，用户连点会产生重发风暴，且一次可能已落地的取消被谎报为未发。——Task 6 测试「an unknown stop enters the gate; a second act is rejected with TASK_OFFICE_COMMAND_UNKNOWN until a resync reconciles」+「a resync that still shows the run active resolves unknown to not-landed and lifts the gate」。
2. **停止后立即重启被会话槽卡死**：cancel 202 与下一 Run 准入之间 `sessions.active_agent_run_id` 未释放，`Admit` 永远 `ErrRunActive`，停止后重启形同虚设。——Task 3 测试「CancelOwnedRun releases the session's active-run slot」+ Task 4 HTTP 测试「queue_next after cancel admits a follow-up run on the same session」。
3. **过期 revision 重放命令引发 409-重试风暴**：用户停留在过期页面点停止，服务端 revision 已前进；409（确定性冲突）若被并入 unknown 门会永久卡死干预，或被当 unknown 无限重发。——Task 2 测试「queue_next with a stale revision is a conflict, never an admission」+ Task 6 测试「a 409-coded conflict returns a conflict receipt and does NOT enter the unknown gate」。
4. **queue_next 在活动 Run 上被谎报为「已排队」**：服务端从无排队事实，客户端若把活动 Run 上的 queue-next 回执当 accepted，用户以为下一 Run 会执行他的指令。——Task 2 测试「queue_next on an active run is a conflict with zero new runs」+ Task 6 测试「queue-next on an active run parks locally and the receipt says 'parked', not accepted」。
5. **跨 owner/跨租户命令命中他人的 Run**：run_id 猜测或串号必须统一 404 且零副作用（含零准入、零事件、零槽变更）。——Task 3 测试「CancelOwnedRun answers a foreign owner with not-found and mutates nothing」+ Task 2 测试「Restart with a foreign owner is a uniform not-found」+ Task 4 HTTP 测试「a foreign owner's queue_next/cancel is 404」。

补：**幂等重放 queue_next（同 intentId 网络重试）不得创建第二个 Run**——Task 2 测试「replaying the same idempotency id reconciles onto the same follow-up run」+ Task 4 HTTP 测试「the same external_pending_id replay returns the same next_run_id」。

## 任务结构与文件地图

| # | 任务 | 主要交付 |
|---|---|---|
| 1 | 前置：迁移编号冲突修复 | `000112→000113`（sqlite）/`000191→000192`（versioned）改号，迁移链恢复可跑 |
| 2 | Go：`queue_next` 闭集成员 + `CommandAck` + 终态重准入端口 | `command_queue_next.go`、`interaction.go`（模块+服务）、container 接线 |
| 3 | Go：全保真取消 `CancelOwnedRun`（事件+槽释放） | `agent_run_lifecycle.go`、`GormCancelPort` 委托、container 接线 |
| 4 | Go：HTTP 集成证据（AC3 服务端面） | `workbench_commands_restart_test.go`（全量迁移 sqlite 全链） |
| 5 | contracts：`CommandAction` 扩 `queue_next` + 冻结准入规则 `evaluateQueueNext` | `mobile/execution.ts`、`index.ts` 导出 |
| 6 | mobile-core：`task-intent.ts` + `TaskHandle.act()`（三态/unknown 门/revision 纪律/parked queue-next） | Interface 级 AC1/AC2 主证据 |
| 7 | api-client：`executions.ts` queue_next wire + `remote.command`（契约码） | 跨包 `TASK_COMMAND_CONFLICT/UNKNOWN` |
| 8 | apps/mobile：干预 UI（三态停止卡 + 绑定 Run + 排队意图）+ 控制器 + 装配 | `TaskDetailScreen` 干预区、composition `commands: remote` |
| 9 | apps/mobile：opt-in 真实部署集成冒烟 | `task-intervention-integration-smoke.ts`（AC3 live 证据） |

---

### Task 1: 前置修复——迁移编号冲突（恢复迁移链可跑）

**Files:**
- Rename: `migrations/sqlite/000112_agent_adoption_variants.up.sql` → `migrations/sqlite/000113_agent_adoption_variants.up.sql`（`git mv`，内容不变）
- Rename: `migrations/sqlite/000112_agent_adoption_variants.down.sql` → `migrations/sqlite/000113_agent_adoption_variants.down.sql`
- Rename: `migrations/versioned/000191_agent_adoption_variants.up.sql` → `migrations/versioned/000192_agent_adoption_variants.up.sql`
- Rename: `migrations/versioned/000191_agent_adoption_variants.down.sql` → `migrations/versioned/000192_agent_adoption_variants.down.sql`
- Modify: `internal/application/repository/agent_adoption_test.go:247`（注释改号说明）
- Modify: `internal/application/repository/task_grant_store_test.go:141`（注释改号说明）

**Interfaces:**
- Consumes: 无（纯修复）。
- Produces: 可运行的 `migrations/sqlite` / `migrations/versioned` 迁移链（`000113`/`000192` 空闲已实查：`ls migrations/sqlite | grep 000113` 与 `ls migrations/versioned | grep 000192` 均无输出）。Task 2/4 的迁移测试依赖本任务。

背景：#59（`agent_adoption_variants`，commit a3132eaa0）与 #42（`task_grants`，commit ca9b66ee1）并行批次各占用了 `000112`（sqlite）/`000191`（versioned），golang-migrate 以「版本号唯一」为合法前提（`duplicate migration file: 000112_task_grants.down.sql`，实跑输出）。改号不改变任何 DDL 内容；引用该号的仅两处注释。

- [ ] **Step 1: 确认现状（失败基线）**

Run: `go test ./internal/modules/workbench/service/workbench/ -run TestAdmissionPublishFailureIsRetryable -count=1`
Expected: FAIL，错误含 `duplicate migration file: 000112_task_grants.down.sql`

- [ ] **Step 2: 改号**

```bash
git mv migrations/sqlite/000112_agent_adoption_variants.up.sql migrations/sqlite/000113_agent_adoption_variants.up.sql
git mv migrations/sqlite/000112_agent_adoption_variants.down.sql migrations/sqlite/000113_agent_adoption_variants.down.sql
git mv migrations/versioned/000191_agent_adoption_variants.up.sql migrations/versioned/000192_agent_adoption_variants.up.sql
git mv migrations/versioned/000191_agent_adoption_variants.down.sql migrations/versioned/000192_agent_adoption_variants.down.sql
```

- [ ] **Step 3: 更新两处注释**

`internal/application/repository/agent_adoption_test.go:247` 一带，把「pre-existing migrations/sqlite 000112 duplicate-number conflict」的表述改为指向新号（如实描述：本测试曾因 000112/000191 双号冲突绕开全量迁移；冲突已于本提交改号为 000113/000192 修复，此处保留直接 DDL 的做法不变）。`internal/application/repository/task_grant_store_test.go:141` 注释中「pre-existing migrations/sqlite 000112」改为「migrations/sqlite 000112（task_grants，本计划改号后唯一占用者）」。

- [ ] **Step 4: 验证迁移链恢复**

Run: `go test ./internal/modules/workbench/service/workbench/ -count=1 && go test ./internal/handler/session/ -run 'TestWorkbenchStartHTTPIntegrationAndIdentityIsolation' -count=1 && go test ./internal/application/repository/ -run 'TestAgentAdoption' -count=1`
Expected: 全部 ok（本计划作者实跑：改号前第一/二条分别 FAIL 于 duplicate migration；第三条不受影响）

- [ ] **Step 5: Commit**

```bash
git add migrations/ internal/application/repository/agent_adoption_test.go internal/application/repository/task_grant_store_test.go
git commit -m "fix(migrations): renumber agent_adoption_variants to 000113/000192 — restore unique migration versions (T37 precondition)"
```

---

### Task 2: Go：`queue_next` 命令闭集成员、`CommandAck` 与终态 Run 重准入端口

**Files:**
- Create: `internal/modules/workbench/service/workbench/command_queue_next.go`
- Create: `internal/modules/workbench/service/workbench/command_queue_next_test.go`
- Modify: `internal/modules/workbench/interaction.go:78-105`（`ExecutionCommand` 闭集扩 `queue_next` + 新增 `CommandAck`）
- Modify: `internal/modules/workbench/service/workbench/interaction.go:444-460,608-633`（`Service` 增 `restart` 字段与 `NewInteractionServiceWithRestart`；`Command` 返回 ack、路由 `queue_next`、Run 键控分支改用 `runOwnerID(ctx)`——修复差异记录 6 的 owner 投影不一致）
- Modify: `internal/container/workbench.go:77-82`（`NewWorkbenchInteractionService` 增 `runs`/`admission` 参数并装配重准入端口）
- Modify: `internal/modules/workbench/service/workbench/interaction_test.go:308-316`（`Command` 改双返回值的机械适配）
- Test: `internal/modules/workbench/service/workbench/command_queue_next_test.go`、`internal/modules/workbench/interaction_test.go`（既有闭集测试如断言 cancel|steer 集合需同步扩员——实跑确认后按同样语义补 `queue_next` 用例）

**Interfaces:**
- Consumes: `AdmissionCoordinator.Start(ctx, StartInput) (agentruntime.Run, error)`（同包，`admission.go:278`）；`openAdmissionConcurrencyDB(t)` 与 `countingBudget`（同包测试基建，`admission_concurrency_test.go:25-31,163-190`）；`agentruntime.ErrConflict/ErrNotFound`；handler 的 `writeWorkbenchCommandError` 错误映射（`workbench_commands.go:120-152`）。
- Produces（后续任务逐字消费）:
  - `workbench.ExecutionCommand` 闭集新增 `queue_next`（`Text` 必填、`ExpectedRevision >= 0`、可选 `ExternalPendingID` 作为幂等 id）。
  - `workbench.CommandAck struct { RunID string `json:"run_id"`; Action string `json:"action"`; NextRunID string `json:"next_run_id,omitempty"` }`。
  - `(*Service).Command(ctx, runID, command) (workbench.CommandAck, error)`（签名变更：原返回 `error`）。
  - `RunRestartPort interface { Restart(ctx context.Context, tenantID uint64, ownerID, runID, text, requestID string, expectedRevision int64) (string, error) }`。
  - `NewGormRunRestartPort(db *gorm.DB, admission *AdmissionCoordinator) *GormRunRestartPort`。
  - `NewInteractionServiceWithRestart(store InteractionStore, steer SteerPort, cancel CancelPort, gate *approval.Gate, restart RunRestartPort) *Service`。
  - `runOwnerID(ctx context.Context) string`（包内私有）——Run 键控命令（cancel/steer/queue_next）的 owner 投影：`UserIDFromContext` 优先（与 `agent_runs.owner_id` 写入投影一致），Principal.StorageID 兜底。
  - wire：`POST /api/v1/workbench/executions/:run_id/commands` body `{"action":"queue_next","text":"...","expected_revision":N,"external_pending_id":"..."}` → 202 `{"success":true,"data":{"run_id":"...","action":"queue_next","next_run_id":"..."}}`；活动 Run → 409；revision 不符 → 409；非本人/不存在 → 404；未接线 → 501 `capability_unavailable`。

- [ ] **Step 1: 写失败测试（服务语义）**

创建 `internal/modules/workbench/service/workbench/command_queue_next_test.go`：

```go
package workbench

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	contract "github.com/Tencent/WeKnora/internal/modules/workbench"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// queueNextEnv 在全量迁移 sqlite 上装真实的准入协调器与命令服务（与 admission
// 并发测试同一基建）：queue_next 的语义是「经同一 AdmissionCoordinator 重准入」，
// 只有用真协调器才能证明幂等与单写者。
func queueNextEnv(t *testing.T) (*Service, *AdmissionCoordinator, *gorm.DB) {
	t.Helper()
	db := openAdmissionConcurrencyDB(t) // tenant 1 / u1 / s1(trpc)
	runs := repository.NewAgentRunStore(db)
	coordinator := NewAdmissionCoordinator(db, runs, &countingBudget{}, nil)
	svc := NewInteractionServiceWithRestart(nil, nil, nil, nil, NewGormRunRestartPort(db, coordinator))
	return svc, coordinator, db
}

// runOwnerProjection：命令服务的 Run 键控 owner 投影必须与 agent_runs.owner_id 的
// 写入投影（UserIDFromContext）一致——web 用户 Principal.StorageID() 是
// "web_user:u1" 形态，与准入写入的 "u1" 不同（差异记录 6）。本测试用真实准入行证明。
func TestQueueNextMatchesTheAdmissionOwnerProjection(t *testing.T) {
	svc, coordinator, db := queueNextEnv(t)
	ctx := queueNextContext()
	first, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "a1", TargetID: "platform", RequestID: "r-owner", Text: "goal", BudgetUpper: 100})
	require.NoError(t, err)
	var owner string
	require.NoError(t, db.Raw(`SELECT owner_id FROM agent_runs WHERE run_id = ?`, first.Key.RunID).Scan(&owner).Error)
	require.NotEqual(t, "web_user:u1", owner, "admission writes the raw user id; the command predicate must match it")
	require.NoError(t, db.Exec(`UPDATE agent_runs SET status = 'succeeded', revision = revision + 1 WHERE run_id = ?`, first.Key.RunID).Error)
	require.NoError(t, db.Exec(`UPDATE sessions SET active_agent_run_id = NULL WHERE id = 's1'`).Error)
	var revision int64
	require.NoError(t, db.Raw(`SELECT revision FROM agent_runs WHERE run_id = ?`, first.Key.RunID).Scan(&revision).Error)
	// 同一身份（ctx 只有 TenantID+UserID）经 Service.Command 必须 202 命中自己的 Run：
	// 若 Command 仍用 identity() 的 storage id 投影，这里会 404。
	ack, err := svc.Command(ctx, first.Key.RunID, contract.ExecutionCommand{Action: "queue_next", Text: "next", ExpectedRevision: revision, ExternalPendingID: "q-owner"})
	require.NoError(t, err)
	require.NotEmpty(t, ack.NextRunID)
}

func TestQueueNextValidatesAsAClosedUnionMember(t *testing.T) {
	require.NoError(t, contract.ExecutionCommand{Action: "queue_next", Text: "next", ExpectedRevision: 3}.Validate())
	require.ErrorIs(t, contract.ExecutionCommand{Action: "queue_next", ExpectedRevision: 3}.Validate(), contract.ErrCommandActionMismatch)
	require.ErrorIs(t, contract.ExecutionCommand{Action: "restart", Text: "x", ExpectedRevision: 3}.Validate(), contract.ErrCommandActionMismatch)
}

func TestQueueNextOnTerminalRunAdmitsFollowUpOnSameSession(t *testing.T) {
	svc, coordinator, db := queueNextEnv(t)
	ctx := queueNextContext()
	first, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "a1", TargetID: "platform", RequestID: "r1", Text: "goal", BudgetUpper: 100})
	require.NoError(t, err)
	// 父 Run 置终态并释放会话槽（模拟引擎 finalize 的既成事实）。
	require.NoError(t, db.Exec(`UPDATE agent_runs SET status = 'succeeded', revision = revision + 1 WHERE run_id = ?`, first.Key.RunID).Error)
	require.NoError(t, db.Exec(`UPDATE sessions SET active_agent_run_id = NULL WHERE id = 's1'`).Error)
	var revision int64
	require.NoError(t, db.Raw(`SELECT revision FROM agent_runs WHERE run_id = ?`, first.Key.RunID).Scan(&revision).Error)

	ack, err := svc.Command(ctx, first.Key.RunID, contract.ExecutionCommand{Action: "queue_next", Text: "do the next thing", ExpectedRevision: revision, ExternalPendingID: "qid-1"})
	require.NoError(t, err)
	require.Equal(t, "queue_next", ack.Action)
	require.Equal(t, first.Key.RunID, ack.RunID)
	require.NotEmpty(t, ack.NextRunID)
	require.NotEqual(t, first.Key.RunID, ack.NextRunID)
	// 同 session、同租户、同 owner 的下一 Run 已排队。
	var follow struct {
		SessionID string
		OwnerID   string
		Status    string
	}
	require.NoError(t, db.Raw(`SELECT session_id, owner_id, status FROM agent_runs WHERE run_id = ?`, ack.NextRunID).Scan(&follow).Error)
	require.Equal(t, "s1", follow.SessionID)
	require.Equal(t, "u1", follow.OwnerID)
	require.Equal(t, "queued", follow.Status)

	// 幂等重放（网络重试语义）：同 idempotency id + 已前进后的真实 revision 也不得建第二个 Run。
	replay, err := svc.Command(ctx, first.Key.RunID, contract.ExecutionCommand{Action: "queue_next", Text: "do the next thing", ExpectedRevision: revision, ExternalPendingID: "qid-1"})
	require.NoError(t, err)
	require.Equal(t, ack.NextRunID, replay.NextRunID)
}

func TestQueueNextOnActiveRunIsASingleWriterConflict(t *testing.T) {
	svc, coordinator, db := queueNextEnv(t)
	ctx := queueNextContext()
	first, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "a1", TargetID: "platform", RequestID: "r1", Text: "goal", BudgetUpper: 100})
	require.NoError(t, err)
	var revision int64
	require.NoError(t, db.Raw(`SELECT revision FROM agent_runs WHERE run_id = ?`, first.Key.RunID).Scan(&revision).Error)

	_, err = svc.Command(ctx, first.Key.RunID, contract.ExecutionCommand{Action: "queue_next", Text: "queue me", ExpectedRevision: revision})
	require.ErrorIs(t, err, agentruntime.ErrConflict)
	var runs int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM agent_runs WHERE session_id = 's1'`).Scan(&runs).Error)
	require.EqualValues(t, 1, runs, "a conflict must never admit a second run")
}

func TestQueueNextWithStaleRevisionIsAConflict(t *testing.T) {
	svc, coordinator, db := queueNextEnv(t)
	ctx := queueNextContext()
	first, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "a1", TargetID: "platform", RequestID: "r1", Text: "goal", BudgetUpper: 100})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`UPDATE agent_runs SET status = 'canceled', revision = revision + 1 WHERE run_id = ?`, first.Key.RunID).Error)
	require.NoError(t, db.Exec(`UPDATE sessions SET active_agent_run_id = NULL WHERE id = 's1'`).Error)
	var revision int64
	require.NoError(t, db.Raw(`SELECT revision FROM agent_runs WHERE run_id = ?`, first.Key.RunID).Scan(&revision).Error)

	_, err = svc.Command(ctx, first.Key.RunID, contract.ExecutionCommand{Action: "queue_next", Text: "stale view", ExpectedRevision: revision - 1})
	require.ErrorIs(t, err, agentruntime.ErrConflict)
}

func TestQueueNextForeignOwnerIsUniformNotFound(t *testing.T) {
	svc, coordinator, db := queueNextEnv(t)
	ctx := queueNextContext()
	first, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "a1", TargetID: "platform", RequestID: "r1", Text: "goal", BudgetUpper: 100})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`UPDATE agent_runs SET status = 'succeeded', revision = revision + 1 WHERE run_id = ?`, first.Key.RunID).Error)
	require.NoError(t, db.Exec(`UPDATE sessions SET active_agent_run_id = NULL WHERE id = 's1'`).Error)
	var revision int64
	require.NoError(t, db.Raw(`SELECT revision FROM agent_runs WHERE run_id = ?`, first.Key.RunID).Scan(&revision).Error)

	foreign := queueNextContextFor("u2")
	_, err = svc.Command(foreign, first.Key.RunID, contract.ExecutionCommand{Action: "queue_next", Text: "steal", ExpectedRevision: revision})
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
	var runs int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM agent_runs WHERE session_id = 's1'`).Scan(&runs).Error)
	require.EqualValues(t, 1, runs, "a foreign command must have zero side effects")
}

func TestQueueNextWithoutRestartPortFailsClosed(t *testing.T) {
	svc := NewInteractionService(nil, nil, nil)
	_, err := svc.Command(queueNextContext(), "run-1", contract.ExecutionCommand{Action: "queue_next", Text: "x", ExpectedRevision: 0})
	require.ErrorIs(t, err, ErrCapabilityUnavailable)
}

func queueNextContext() context.Context { return queueNextContextFor("u1") }

func queueNextContextFor(actor string) context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	return context.WithValue(ctx, types.UserIDContextKey, actor)
}
```

注意（owner 投影，差异记录 6）：`queueNextContext()`/`queueNextContextFor(actor)` 与既有 `interactionContext()` 同构（TenantID + UserID 两个 context key）。准入经 `contextIdentity()` 把 `agent_runs.owner_id` 写为原始 `u1`；命令服务的 Run 键控分支必须以 `runOwnerID(ctx)`（UserID 优先）取同一投影——`TestQueueNextMatchesTheAdmissionOwnerProjection` 正是钉死这一致性的用例（若 `Command` 误用 `identity()` 的 storage id 投影 `web_user:u1`，该用例 404 失败）。`u2` 的隔离用例因此比较 `u2` vs 行上 `u1`——不相等即统一 404，零副作用。`follow.OwnerID` 的 `u1` 断言若因未来准入投影变化失败，以实查值为准修正（先查后断，不猜测）。

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/modules/workbench/service/workbench/ -run 'TestQueueNext' -count=1`
Expected: FAIL（编译错误：`queue_next` 不是合法 action / `CommandAck` 未定义 / `NewInteractionServiceWithRestart` 未定义）

- [ ] **Step 3: 最小实现**

3a. `internal/modules/workbench/interaction.go`——闭集与 ack（替换 78-105 行的 `ExecutionCommand` 定义与 `Validate`，紧随其后新增 `CommandAck`）：

```go
// ExecutionCommand is a closed union. cancel has no payload; steer carries a
// text payload which is delivered through the existing steer queue; queue_next
// carries a text payload for the NEXT run of the same task and is only
// admissible on a terminal run (one task permits at most one write run — an
// active run owns the write lane, so queueing on it is a conflict, never a
// silent queue).
type ExecutionCommand struct {
	Action            string `json:"action"`
	Text              string `json:"text,omitempty"`
	ExpectedRevision  int64  `json:"expected_revision"`
	ExternalPendingID string `json:"external_pending_id,omitempty"`
	CredentialVersion int64  `json:"credential_version,omitempty"`
}

func (c ExecutionCommand) Validate() error {
	switch strings.TrimSpace(c.Action) {
	case "cancel":
		if strings.TrimSpace(c.Text) != "" {
			return ErrCommandActionMismatch
		}
	case "steer", "queue_next":
		if strings.TrimSpace(c.Text) == "" {
			return ErrCommandActionMismatch
		}
	default:
		return ErrCommandActionMismatch
	}
	if c.ExpectedRevision < 0 {
		return ErrCommandActionMismatch
	}
	return nil
}

// CommandAck is the honest command receipt: the run the command bound to, the
// echoed action, and — for queue_next — the follow-up run admitted for the
// queued instruction. Clients present bound Run + NextRunID so the owner sees
// which run the intervention actually attached to (T07).
type CommandAck struct {
	RunID     string `json:"run_id"`
	Action    string `json:"action"`
	NextRunID string `json:"next_run_id,omitempty"`
}
```

3b. `internal/modules/workbench/service/workbench/interaction.go`——`Service` 结构体（`:444-450`）增 `restart RunRestartPort` 字段；新增构造器（放在 `NewInteractionServiceWithApproval` 之后）：

```go
// NewInteractionServiceWithRestart installs the queue_next re-admission port.
// A nil restart keeps queue_next fail-closed (capability_unavailable) — the
// same discipline as a missing steer or cancel port.
func NewInteractionServiceWithRestart(store InteractionStore, steer SteerPort, cancel CancelPort, gate *approval.Gate, restart RunRestartPort) *Service {
	svc := NewInteractionServiceWithApproval(store, steer, cancel, gate)
	if svc != nil {
		svc.restart = restart
	}
	return svc
}
```

`Command`（`:608-633`）签名改为返回 `(workbench.CommandAck, error)`、路由 `queue_next`，并且三个 Run 键控分支改用 `runOwnerID(ctx)` 作为 owner 谓词投影（差异记录 6——`agent_runs.owner_id` 由 `UserIDFromContext` 写入，`identity()` 的 storage id 投影与之不匹配；`Decide`/`List` 保持 `identity()` 不变，交互行两投影一致）：

```go
// runOwnerID is the owner projection for run-keyed commands (cancel/steer/
// queue_next): agent_runs.owner_id is written by admission's contextIdentity
// via UserIDFromContext, so the command predicate must read the same value.
// The principal storage id (web_user:<id>) belongs to the interaction rows,
// not to run rows — mixing them made cancel/steer never match web-admitted
// runs (T37 difference record 6).
func runOwnerID(ctx context.Context) string {
	if uid, ok := types.UserIDFromContext(ctx); ok && strings.TrimSpace(uid) != "" {
		return strings.TrimSpace(uid)
	}
	return canonicalOwnerID(ctx, "")
}

func (s *Service) Command(ctx context.Context, runID string, command workbench.ExecutionCommand) (workbench.CommandAck, error) {
	tenant, _, err := identity(ctx) // tenant + actor existence (web_user:<id> synthesized from UserID when no principal)
	if err != nil {
		return workbench.CommandAck{}, err
	}
	owner := runOwnerID(ctx)
	if err := command.Validate(); err != nil {
		return workbench.CommandAck{}, err
	}
	if s == nil || strings.TrimSpace(runID) == "" {
		return workbench.CommandAck{}, ErrInteractionNotFound
	}
	runID = strings.TrimSpace(runID)
	switch command.Action {
	case "cancel":
		if s.cancel == nil {
			return workbench.CommandAck{}, ErrCapabilityUnavailable
		}
		if err := s.cancel.Cancel(ctx, tenant, owner, runID, command.ExpectedRevision); err != nil {
			return workbench.CommandAck{}, err
		}
		return workbench.CommandAck{RunID: runID, Action: "cancel"}, nil
	case "steer":
		if s.steer == nil {
			return workbench.CommandAck{}, ErrCapabilityUnavailable
		}
		if err := s.steer.Steer(ctx, tenant, owner, runID, command.Text, command.ExpectedRevision); err != nil {
			return workbench.CommandAck{}, err
		}
		return workbench.CommandAck{RunID: runID, Action: "steer"}, nil
	case "queue_next":
		if s.restart == nil {
			return workbench.CommandAck{}, ErrCapabilityUnavailable
		}
		next, err := s.restart.Restart(ctx, tenant, owner, runID, command.Text, queueNextRequestID(command), command.ExpectedRevision)
		if err != nil {
			return workbench.CommandAck{}, err
		}
		return workbench.CommandAck{RunID: runID, Action: "queue_next", NextRunID: next}, nil
	default:
		return workbench.CommandAck{}, workbench.ErrCommandActionMismatch
	}
}

// queueNextRequestID derives the admission request id from the command's
// idempotency id, so a network-retried queue_next reconciles onto the same
// follow-up run (spec: every unknown-outcome command carries a durable
// idempotency identity). The "queue-" prefix keeps the namespace disjoint
// from start request ids in workbench_requests.
func queueNextRequestID(command workbench.ExecutionCommand) string {
	if id := strings.TrimSpace(command.ExternalPendingID); id != "" {
		return "queue-" + id
	}
	return "queue-" + uuid.NewString()
}
```

3c. 新文件 `internal/modules/workbench/service/workbench/command_queue_next.go`：

```go
package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	workbench "github.com/Tencent/WeKnora/internal/modules/workbench"
	"gorm.io/gorm"
)

// RunRestartPort admits the next run of a task with an explicitly queued
// instruction (T07 queue-next / stop-restart). It is a port so deployments
// without an admission coordinator fail closed instead of improvising a
// second admission path.
type RunRestartPort interface {
	Restart(ctx context.Context, tenantID uint64, ownerID, runID, text, requestID string, expectedRevision int64) (string, error)
}

// GormRunRestartPort re-admits through the same AdmissionCoordinator that
// created the parent run: same session, agent, target, workspace and budget
// ceiling are read from the parent's immutable admission snapshot; only the
// text is the queued instruction. The engine-side after-input drain
// (admitAfterFollowUps) does not apply to workbench-admitted runs — their
// snapshot is the admission map, not a graph DurableRunSnapshot — so the
// command surface owns the follow-up admission itself.
type GormRunRestartPort struct {
	db        *gorm.DB
	admission *AdmissionCoordinator
}

func NewGormRunRestartPort(db *gorm.DB, admission *AdmissionCoordinator) *GormRunRestartPort {
	return &GormRunRestartPort{db: db, admission: admission}
}

type restartRunRow struct {
	SessionID, OwnerID, TargetID, Status string
	Revision                             int64
	Snapshot                             string
}

func (restartRunRow) TableName() string { return "agent_runs" }

func (p *GormRunRestartPort) Restart(ctx context.Context, tenantID uint64, ownerID, runID, text, requestID string, expectedRevision int64) (string, error) {
	if p == nil || p.db == nil || p.admission == nil {
		return "", ErrCapabilityUnavailable
	}
	if strings.TrimSpace(text) == "" || strings.TrimSpace(requestID) == "" {
		return "", workbench.ErrCommandActionMismatch
	}
	var row restartRunRow
	err := p.db.WithContext(ctx).
		Where("tenant_id = ? AND run_id = ?", tenantID, strings.TrimSpace(runID)).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", agentruntime.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	// A foreign run id is indistinguishable from a missing one: uniform 404.
	if row.OwnerID != strings.TrimSpace(ownerID) {
		return "", agentruntime.ErrNotFound
	}
	// Single writer (spec: one task permits at most one write run): while the
	// parent is active the next run cannot be admitted — the honest answer is
	// a conflict, never a silent queue.
	if row.Status != "succeeded" && row.Status != "failed" && row.Status != "canceled" {
		return "", agentruntime.ErrConflict
	}
	// The revision read-check is the "real revision" contract: a stale view
	// must never admit a restart on top of facts it has not observed. The
	// parent row itself is not mutated here — the follow-up admission is the
	// write, fenced separately by the session slot and the request id.
	if row.Revision != expectedRevision {
		return "", agentruntime.ErrConflict
	}
	var parent struct {
		AgentID      string `json:"agent_id"`
		TargetID     string `json:"target_id"`
		WorkspaceRef string `json:"workspace_ref"`
		SpaceID      string `json:"space_id"`
		BudgetUpper  int64  `json:"budget_upper"`
	}
	if err := json.Unmarshal([]byte(row.Snapshot), &parent); err != nil {
		// Not a coordinator-admitted run: fail closed instead of guessing.
		return "", agentruntime.ErrConflict
	}
	run, err := p.admission.Start(ctx, StartInput{
		SessionID: row.SessionID, AgentID: parent.AgentID, TargetID: parent.TargetID,
		WorkspaceRef: parent.WorkspaceRef, SpaceID: parent.SpaceID,
		RequestID: strings.TrimSpace(requestID), Text: strings.TrimSpace(text), BudgetUpper: parent.BudgetUpper,
	})
	if err != nil {
		return "", err
	}
	return run.Key.RunID, nil
}
```

3d. `internal/handler/session/workbench_commands.go`——`Command`（`:103-118`）改用 ack；`writeWorkbenchCommandError` 的 switch 增一行 `ErrRunActive → 409`（重准入撞会话槽时的确定性冲突）：

```go
func (h *WorkbenchCommandHandler) Command(c *gin.Context) {
	if h == nil || h.interactions == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	var command workbench.ExecutionCommand
	if err := c.ShouldBindJSON(&command); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request"})
		return
	}
	ack, err := h.interactions.Command(commandContext(c), c.Param("run_id"), command)
	if err != nil {
		writeWorkbenchCommandError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"success": true, "data": ack})
}
```

`writeWorkbenchCommandError` 内、`agentruntime.ErrConflict` 分支旁新增：

```go
	case errors.Is(err, agentruntime.ErrRunActive):
		// The follow-up admission raced another write run on the session: a
		// deterministic conflict, not a server error.
		status = http.StatusConflict
```

3e. `internal/container/workbench.go`——`NewWorkbenchInteractionService` 增参并装配（fx 已提供 `*repository.AgentRunStore` 与 `*workbenchservice.AdmissionCoordinator`，`container.go:276`）：

```go
func NewWorkbenchInteractionService(store *workbenchservice.GormInteractionStore, gate *approval.Gate, streams interfaces.StreamManager, runs *repository.AgentRunStore, admission *workbenchservice.AdmissionCoordinator) *workbenchservice.Service {
	return workbenchservice.NewInteractionServiceWithRestart(
		store,
		workbenchservice.NewGormSteerPort(storeDB(store), streams),
		workbenchservice.NewGormCancelPort(storeDB(store)),
		gate,
		workbenchservice.NewGormRunRestartPort(storeDB(store), admission),
	)
}
```

（Task 3 会把 `NewGormCancelPort(storeDB(store))` 换成 `NewGormCancelPort(runs)`，本步保持现状。）

3f. 既有测试机械适配：`internal/modules/workbench/service/workbench/interaction_test.go:308-316` 两处 `err := svc.Command(...)` 改 `_, err := svc.Command(...)`；`internal/modules/workbench/interaction_test.go` 若存在对闭集成员的断言（实跑 `go test ./internal/modules/workbench/ -count=1` 确认），按同语义补 `queue_next` 用例（text 必填、`cancel` 带 text 拒绝不变）。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/modules/workbench/service/workbench/ -run 'TestQueueNext|TestInteractionService' -count=1 && go test ./internal/modules/workbench/... -count=1 && go build ./...`
Expected: 全部 ok / 编译通过

- [ ] **Step 5: Commit**

```bash
git add internal/modules/workbench/ internal/modules/workbench/service/workbench/ internal/handler/session/workbench_commands.go internal/container/workbench.go
git commit -m "feat(workbench): queue_next command — re-admit the next run on a terminal run (T07 #37)"
```

---

### Task 3: Go：全保真取消 `CancelOwnedRun`（`cancellation_requested` 事件 + 会话槽释放）

**Files:**
- Modify: `internal/application/repository/agent_run_lifecycle.go`（新增 `CancelOwnedRun`，紧随 `CancelRun` 之后）
- Create: `internal/application/repository/agent_run_cancel_owned_test.go`
- Modify: `internal/modules/workbench/service/workbench/interaction.go:377-395`（`GormCancelPort` 持 `*repository.AgentRunStore` 并委托）
- Modify: `internal/container/workbench.go`（`NewGormCancelPort(storeDB(store))` → `NewGormCancelPort(runs)`）
- Test: `internal/application/repository/agent_run_cancel_owned_test.go`

**Interfaces:**
- Consumes: `appendRunEventLocked`（同文件 `agent_run_events.go:166`，repository 独占 seq 分配）；`agentRunRow`/`runScope`（`agent_run.go:41`）；`agentruntime.ErrConflict/ErrNotFound`。
- Produces（Task 4 消费）:
  - `(*AgentRunStore).CancelOwnedRun(ctx context.Context, tenantID uint64, ownerID, runID string, expectedRevision int64, reason string) error`——语义：事务内校验 tenant+run 存在（缺失→`ErrNotFound`）、owner 一致（不一致→统一 `ErrNotFound`）、status 非终态且 `revision == expectedRevision`（否则→`ErrConflict`），然后原子执行 `status='canceled'` + `wait_reason=reason` + 清 lease + `revision+1` + 追加 `cancellation_requested` Run 事件（payload `{"reason":...}`）+ 释放 `sessions.active_agent_run_id`（仅当其指向本 Run）。已是 `canceled` → 幂等返回 `nil`（与 `CancelRun` 同语义）。
  - `NewGormCancelPort(runs *repository.AgentRunStore) *GormCancelPort`（构造签名变更；`CancelPort` 接口不变）。

- [ ] **Step 1: 写失败测试**

创建 `internal/application/repository/agent_run_cancel_owned_test.go`：

```go
package repository

import (
	"context"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 与 steerRunRow 先例同法：本测试只证明 CancelOwnedRun 的单事务语义，
// 直接 AutoMigrate 三张最小行集（全量迁移链由 Task 4 的 HTTP 集成测试覆盖）。
type cancelOwnedRunRow struct {
	TenantID                            uint64
	RunID, SessionID, OwnerID           string
	Status, WaitReason, LeaseOwner      string
	LeaseUntil                          *time.Time
	Revision                            int64
	UpdatedAt                           time.Time
}

func (cancelOwnedRunRow) TableName() string { return "agent_runs" }

type cancelOwnedSessionRow struct {
	TenantID        uint64
	ID              string `gorm:"column:id"`
	ActiveAgentRunID *string `gorm:"column:active_agent_run_id"`
}

func (cancelOwnedSessionRow) TableName() string { return "sessions" }

type cancelOwnedEventRow struct {
	TenantID                      uint64
	RunID                         string
	Seq                           int64
	AttemptID, EventType, Payload string
}

func (cancelOwnedEventRow) TableName() string { return "agent_run_events" }

func openCancelOwnedDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:cancel_owned_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&cancelOwnedRunRow{}, &cancelOwnedSessionRow{}, &cancelOwnedEventRow{}))
	require.NoError(t, db.Exec(`INSERT INTO sessions (tenant_id, id, active_agent_run_id) VALUES (1, 's1', NULL)`).Error)
	require.NoError(t, db.Exec(`UPDATE sessions SET active_agent_run_id = 'run-1' WHERE id = 's1'`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, status, wait_reason, revision, lease_owner, lease_until, updated_at) VALUES (1, 'run-1', 's1', 'u1', 'running', '', 0, '', NULL, CURRENT_TIMESTAMP)`).Error)
	return db
}

func TestCancelOwnedRunWritesEventReleasesSlotAndFencesRevision(t *testing.T) {
	db := openCancelOwnedDB(t)
	runs := NewAgentRunStore(db)
	ctx := context.Background()

	require.NoError(t, runs.CancelOwnedRun(ctx, 1, "u1", "run-1", 0, "user_requested"))

	var status, reason string
	var revision int64
	require.NoError(t, db.Raw(`SELECT status, wait_reason, revision FROM agent_runs WHERE run_id = 'run-1'`).Scan(&status, &reason, &revision).Error)
	require.Equal(t, "canceled", status)
	require.Equal(t, "user_requested", reason)
	require.EqualValues(t, 1, revision)

	var eventType string
	require.NoError(t, db.Raw(`SELECT event_type FROM agent_run_events WHERE run_id = 'run-1' ORDER BY seq DESC LIMIT 1`).Scan(&eventType).Error)
	require.Equal(t, "cancellation_requested", eventType)

	var slot *string
	require.NoError(t, db.Raw(`SELECT active_agent_run_id FROM sessions WHERE id = 's1'`).Scan(&slot).Error)
	require.Nil(t, slot, "the session slot must be released so a restart can be admitted")

	// 幂等：对已取消的 run 重复取消返回 nil，不再推进 revision。
	require.NoError(t, runs.CancelOwnedRun(ctx, 1, "u1", "run-1", 1, "user_requested"))
	require.NoError(t, db.Raw(`SELECT revision FROM agent_runs WHERE run_id = 'run-1'`).Scan(&revision).Error)
	require.EqualValues(t, 1, revision)
}

func TestCancelOwnedRunStaleRevisionIsAConflict(t *testing.T) {
	db := openCancelOwnedDB(t)
	runs := NewAgentRunStore(db)
	err := runs.CancelOwnedRun(context.Background(), 1, "u1", "run-1", 7, "user_requested")
	require.ErrorIs(t, err, agentruntime.ErrConflict)
	var status string
	require.NoError(t, db.Raw(`SELECT status FROM agent_runs WHERE run_id = 'run-1'`).Scan(&status).Error)
	require.Equal(t, "running", status, "a fenced-out cancel must mutate nothing")
}

func TestCancelOwnedRunForeignOwnerIsUniformNotFound(t *testing.T) {
	db := openCancelOwnedDB(t)
	runs := NewAgentRunStore(db)
	err := runs.CancelOwnedRun(context.Background(), 1, "u2", "run-1", 0, "user_requested")
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
	var count int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM agent_run_events WHERE run_id = 'run-1'`).Scan(&count).Error)
	require.EqualValues(t, 0, count, "a foreign cancel must write no events")
	var slot string
	require.NoError(t, db.Raw(`SELECT COALESCE(active_agent_run_id, '') FROM sessions WHERE id = 's1'`).Scan(&slot).Error)
	require.Equal(t, "run-1", slot, "a foreign cancel must not release the slot")
}
```

（owner 用原始 id `u1`——与准入写入 `agent_runs.owner_id` 的投影一致，见 Task 2 差异记录 6；`cancelOwnedEventRow` 逐字镜像生产 `agentRunEventRow`（`agent_run_events.go:14-20`），`appendRunEventLocked` 以 `ORDER BY seq DESC` 取最大 seq 后 Create——行集列缺失会令 INSERT 失败，测试即红。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/application/repository/ -run 'TestCancelOwnedRun' -count=1`
Expected: FAIL（编译错误：`CancelOwnedRun` 未定义）

- [ ] **Step 3: 最小实现**

`internal/application/repository/agent_run_lifecycle.go`（紧随 `CancelRun` 之后）：

```go
// CancelOwnedRun is the workbench command-surface cancellation: a revision-CAS
// transition to canceled that also writes the durable cancellation_requested
// run event and releases the session's active-run slot in the same
// transaction. The slot release is what makes a later restart (queue_next on
// the terminal run) admissible; the run event is what makes the stop request a
// timeline fact clients can present. A foreign owner is indistinguishable from
// a missing run: uniform not-found, zero side effects.
func (s *AgentRunStore) CancelOwnedRun(ctx context.Context, tenantID uint64, ownerID, runID string, expectedRevision int64, reason string) error {
	if tenantID == 0 || strings.TrimSpace(ownerID) == "" || strings.TrimSpace(runID) == "" {
		return agentruntime.ErrConflict
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run agentRunRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND run_id = ?", tenantID, strings.TrimSpace(runID)).Take(&run).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return agentruntime.ErrNotFound
			}
			return err
		}
		if run.OwnerID != strings.TrimSpace(ownerID) {
			return agentruntime.ErrNotFound
		}
		if run.Status == "canceled" {
			return nil // idempotent, same as CancelRun
		}
		if run.Status == "succeeded" || run.Status == "failed" || run.Revision != expectedRevision {
			return agentruntime.ErrConflict
		}
		if err := tx.Model(&agentRunRow{}).
			Where("tenant_id = ? AND run_id = ? AND revision = ?", tenantID, run.RunID, run.Revision).
			Updates(map[string]any{"status": "canceled", "wait_reason": reason, "lease_owner": "", "lease_until": nil,
				"revision": gorm.Expr("revision + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP")}).Error; err != nil {
			return err
		}
		payloadBytes, err := json.Marshal(map[string]string{"reason": reason})
		if err != nil {
			return err
		}
		if err := appendRunEventLocked(tx, agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: tenantID, RunID: run.RunID}}, "cancellation_requested", string(payloadBytes)); err != nil {
			return err
		}
		return tx.Table("sessions").Where("tenant_id = ? AND id = ? AND active_agent_run_id = ?",
			tenantID, run.SessionID, run.RunID).Update("active_agent_run_id", nil).Error
	})
}
```

文件头 import 增 `"strings"`、`"errors"`、`"gorm.io/gorm/clause"`（保留既有）。SQLite 忽略 `FOR UPDATE`，事务 + revision CAS 是实际栅栏——与 `GormInteractionStore.Decide` 的既有形态一致（`interaction.go(service):270-272`）。

`internal/modules/workbench/service/workbench/interaction.go`——`GormCancelPort`（`:377-395`）改为委托：

```go
// GormCancelPort is the durable cancel command. It delegates to the
// repository's CancelOwnedRun so cancellation keeps its full fidelity: the
// revision-CAS terminal transition, the cancellation_requested run event and
// the session-slot release land in one transaction. Unknown or already-terminal
// runs are conflicts and never mutate a different run.
type GormCancelPort struct{ runs *repository.AgentRunStore }

func NewGormCancelPort(runs *repository.AgentRunStore) *GormCancelPort { return &GormCancelPort{runs: runs} }

func (p *GormCancelPort) Cancel(ctx context.Context, tenantID uint64, ownerID, runID string, expectedRevision int64) error {
	if p == nil || p.runs == nil {
		return ErrCapabilityUnavailable
	}
	return p.runs.CancelOwnedRun(ctx, tenantID, ownerID, runID, expectedRevision, "user_requested")
}
```

（该文件 import 增 `"github.com/Tencent/WeKnora/internal/application/repository"`——同包 `admission.go` 已 import，无环。）

`internal/container/workbench.go` Task 2 Step 3e 的装配中把 `workbenchservice.NewGormCancelPort(storeDB(store))` 改为 `workbenchservice.NewGormCancelPort(runs)`。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/application/repository/ -run 'TestCancelOwnedRun' -count=1 && go test ./internal/modules/workbench/service/workbench/ -count=1 && go build ./...`
Expected: 全部 ok（`TestInteractionServiceDoesNotFallbackForUnavailableCommand` 的 nil-port 分支不受影响；`GormSteerPort` 未动）

- [ ] **Step 5: Commit**

```bash
git add internal/application/repository/agent_run_lifecycle.go internal/application/repository/agent_run_cancel_owned_test.go internal/modules/workbench/service/workbench/interaction.go internal/container/workbench.go
git commit -m "feat(workbench): full-fidelity owned-run cancel — event, slot release, revision CAS (T07 #37)"
```

---

### Task 4: Go：HTTP 集成证据——停止→确认事实→重启全链（AC3 服务端面）

**Files:**
- Create: `internal/handler/session/workbench_commands_restart_test.go`
- Test: `internal/handler/session/workbench_commands_restart_test.go`

**Interfaces:**
- Consumes: `openWorkbenchHTTPDB(t)` / `withIdentity(tenant, actor)`（同包 `workbench_start_integration_test.go:118-147`，全量迁移 sqlite + tenant1/u1/s1(trpc) 种子）；`integrationBudget`（同包 `:30-49`）；Task 2/3 的 `NewInteractionServiceWithRestart`、`NewGormCancelPort(runs)`、`NewGormRunRestartPort(db, coordinator)`；`NewWorkbenchCommandHandler`；`NewWorkbenchStartHandler`。
- Produces: AC3 的服务端证据（本计划验证命令的组成部分）：真实迁移库上 `POST /commands` 的 cancel（202 + 快照事件含 `cancellation_requested` + 会话槽释放）与 queue_next（终态重准入 + 幂等 + 单写者 409 + revision 409 + 跨 owner 404）。

- [ ] **Step 1: 写失败测试**

（若 Task 2/3 已实现，本测试会直接绿——这是允许的：本任务把已实现语义钉在 HTTP 层。若要维持 RED 先行，可先写测试跑一次确认因断言缺事件/缺 next_run_id 而 FAIL，再进入 Task 2/3；执行者按实际顺序处理，最终以本任务命令绿为准。）

创建 `internal/handler/session/workbench_commands_restart_test.go`：

```go
package session

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	workbenchservice "github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// T07 服务端集成证据：全量迁移 sqlite + 真实准入协调器 + 真实命令服务 + 真实 handler。
// stop→确认事实→restart 全链经 HTTP 字节验证（AC3：不以单测/mock 冒充集成证据）。
func newCommandRestartRouter(t *testing.T) (*gin.Engine, *repository.AgentRunStore, *gorm.DB) {
	t.Helper()
	db := openWorkbenchHTTPDB(t)
	runs := repository.NewAgentRunStore(db)
	coordinator := workbenchservice.NewAdmissionCoordinator(db, runs, &integrationBudget{}, nil)
	svc := workbenchservice.NewInteractionServiceWithRestart(
		workbenchservice.NewGormInteractionStore(db),
		nil, // steer 不在本链路；缺端口时 steer 命令 fail closed 501
		workbenchservice.NewGormCancelPort(runs),
		nil,
		workbenchservice.NewGormRunRestartPort(db, coordinator),
	)
	commandHandler := NewWorkbenchCommandHandler(svc)
	startHandler := NewWorkbenchStartHandler(coordinator)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1", withIdentity(1, "u1"))
	v1.POST("/workbench/executions", startHandler.Start)
	v1.POST("/workbench/executions/:run_id/commands", commandHandler.Command)
	return r, runs, db
}

func postCommand(r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestWorkbenchStopThenRestartHTTPIntegration(t *testing.T) {
	r, _, db := newCommandRestartRouter(t)

	// 1. 创建 Task 的首个 Run（真实准入）。
	start := postCommand(r, "/api/v1/workbench/executions", `{"session_id":"s1","agent_id":"a1","target_id":"platform","request_id":"t37-r1","text":"goal","budget_upper":100}`)
	require.Equal(t, http.StatusAccepted, start.Code)
	var admitted struct {
		Data struct {
			RunID string `json:"run_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(start.Body.Bytes(), &admitted))
	runID := admitted.Data.RunID
	require.NotEmpty(t, runID)

	// 2. queue_next 在活动 Run 上是确定性冲突（单写者），零准入。
	active := postCommand(r, "/api/v1/workbench/executions/"+runID+"/commands", `{"action":"queue_next","text":"too early","expected_revision":0}`)
	require.Equal(t, http.StatusConflict, active.Code)

	// 3. 停止：202 + ack 绑定本 Run。
	stop := postCommand(r, "/api/v1/workbench/executions/"+runID+"/commands", `{"action":"cancel","expected_revision":0}`)
	require.Equal(t, http.StatusAccepted, stop.Code)
	require.Contains(t, stop.Body.String(), `"action":"cancel"`)
	require.Contains(t, stop.Body.String(), runID)

	// 4. 重复停止（revision 已前进）→ 409：旧视图不得再次改写。
	staleStop := postCommand(r, "/api/v1/workbench/executions/"+runID+"/commands", `{"action":"cancel","expected_revision":0}`)
	require.Equal(t, http.StatusConflict, staleStop.Code)

	// 5. 停止后重启：queue_next 带真实（终态后）revision → 202 + next_run_id。
	var revision int64
	require.NoError(t, db.Raw(`SELECT revision FROM agent_runs WHERE run_id = ?`, runID).Scan(&revision).Error)
	revisionText := strconv.FormatInt(revision, 10)
	restart := postCommand(r, "/api/v1/workbench/executions/"+runID+"/commands", `{"action":"queue_next","text":"restart with this","expected_revision":`+revisionText+`,"external_pending_id":"t37-q1"}`)
	require.Equal(t, http.StatusAccepted, restart.Code)
	var restartAck struct {
		Data struct {
			RunID     string `json:"run_id"`
			Action    string `json:"action"`
			NextRunID string `json:"next_run_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(restart.Body.Bytes(), &restartAck))
	require.Equal(t, "queue_next", restartAck.Data.Action)
	require.Equal(t, runID, restartAck.Data.RunID)
	require.NotEmpty(t, restartAck.Data.NextRunID)
	require.NotEqual(t, runID, restartAck.Data.NextRunID)

	// 6. 幂等重放（网络重试语义）：同 idempotency id → 同一个 next_run_id。
	replay := postCommand(r, "/api/v1/workbench/executions/"+runID+"/commands", `{"action":"queue_next","text":"restart with this","expected_revision":`+revisionText+`,"external_pending_id":"t37-q1"}`)
	require.Equal(t, http.StatusAccepted, replay.Code)
	var replayAck struct {
		Data struct {
			NextRunID string `json:"next_run_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(replay.Body.Bytes(), &replayAck))
	require.Equal(t, restartAck.Data.NextRunID, replayAck.Data.NextRunID)
}

func TestWorkbenchCancelHTTPWritesEventReleasesSlot(t *testing.T) {
	r, _, db := newCommandRestartRouter(t)
	start := postCommand(r, "/api/v1/workbench/executions", `{"session_id":"s1","agent_id":"a1","target_id":"platform","request_id":"t37-r2","text":"goal","budget_upper":100}`)
	require.Equal(t, http.StatusAccepted, start.Code)
	var admitted struct {
		Data struct {
			RunID string `json:"run_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(start.Body.Bytes(), &admitted))
	runID := admitted.Data.RunID

	stop := postCommand(r, "/api/v1/workbench/executions/"+runID+"/commands", `{"action":"cancel","expected_revision":0}`)
	require.Equal(t, http.StatusAccepted, stop.Code)

	db := db
	var events int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM agent_run_events WHERE run_id = ? AND event_type = 'cancellation_requested'`, runID).Scan(&events).Error)
	require.EqualValues(t, 1, events, "the stop request must be a durable timeline fact")
	var slot *string
	require.NoError(t, db.Raw(`SELECT active_agent_run_id FROM sessions WHERE id = 's1'`).Scan(&slot).Error)
	require.Nil(t, slot)
}

func TestWorkbenchCommandHTTPForeignOwnerIsIsolated(t *testing.T) {
	r, _, db := newCommandRestartRouter(t)
	start := postCommand(r, "/api/v1/workbench/executions", `{"session_id":"s1","agent_id":"a1","target_id":"platform","request_id":"t37-r3","text":"goal","budget_upper":100}`)
	require.Equal(t, http.StatusAccepted, start.Code)
	var admitted struct {
		Data struct {
			RunID string `json:"run_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(start.Body.Bytes(), &admitted))
	runID := admitted.Data.RunID

	foreign := gin.New()
	// 同一 handler、不同身份（u2）：跨 owner 命令统一 404。
	db := db
	runs := repository.NewAgentRunStore(db)
	coordinator := workbenchservice.NewAdmissionCoordinator(db, runs, &integrationBudget{}, nil)
	svc := workbenchservice.NewInteractionServiceWithRestart(workbenchservice.NewGormInteractionStore(db), nil, workbenchservice.NewGormCancelPort(runs), nil, workbenchservice.NewGormRunRestartPort(db, coordinator))
	fh := NewWorkbenchCommandHandler(svc)
	foreign.POST("/api/v1/workbench/executions/:run_id/commands", withIdentity(1, "u2"), fh.Command)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/executions/"+runID+"/commands", strings.NewReader(`{"action":"cancel","expected_revision":0}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	foreign.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
	var status string
	require.NoError(t, db.Raw(`SELECT status FROM agent_runs WHERE run_id = ?`, runID).Scan(&status).Error)
	require.Equal(t, "queued", status, "a foreign cancel must not mutate the run")
}
```

owner 投影说明：`withIdentity` 只设置 UserID → 命令服务 `runOwnerID` 与准入 `contextIdentity` 均取原始 `u1`，两侧一致（Task 2 差异记录 6 的修复正覆盖此链路；若断言因未来投影变化失败，以实跑查询值为准修正——先 SELECT 打印再断言，不猜测）。

- [ ] **Step 2: 运行（Task 2/3 未合入时应失败；合入后必须绿）**

Run: `go test ./internal/handler/session/ -run 'TestWorkbenchStopThenRestartHTTPIntegration|TestWorkbenchCancelHTTPWritesEventReleasesSlot|TestWorkbenchCommandHTTPForeignOwnerIsIsolated' -count=1`
Expected: PASS（若 Task 2/3 尚未实现则为编译失败——按任务顺序执行时本任务在 Task 3 之后，直接绿）

- [ ] **Step 3: 全量回归**

Run: `go test ./internal/handler/session/ -run 'TestWorkbench' -count=1 && go test ./internal/modules/workbench/... -count=1`
Expected: 全部 ok

- [ ] **Step 4: Commit**

```bash
git add internal/handler/session/workbench_commands_restart_test.go
git commit -m "test(workbench): HTTP integration evidence for stop→restart and queue_next fences (T07 #37 AC3)"
```

---

### Task 5: contracts：`CommandAction` 扩 `queue_next` + 冻结准入规则 `evaluateQueueNext`

**Files:**
- Modify: `packages/contracts/src/mobile/execution.ts:73-97`（`CommandAction` 扩员 + 新增 `evaluateQueueNext`）
- Modify: `packages/contracts/src/index.ts:640` 附近（导出 `evaluateQueueNext` 与 `QueueNextDecision`）
- Test: `packages/contracts/test/mobile-execution.test.ts`（追加用例）

**Interfaces:**
- Consumes: `ExecutionDTO`（含 `run_status`/`revision`，同文件）；`TERMINAL_RUN_STATUS`（同文件 `:76`）；`evaluateCommand` 冻结规则（MX-003，`:81-97`——**不改**）。
- Produces（Task 6/7 消费）: `CommandAction = 'cancel' | 'steer' | 'queue_next'`；`QueueNextDecision { allowed: boolean; reason: string }`；`evaluateQueueNext(execution: ExecutionDTO): QueueNextDecision`——冻结规则：活动 Run → 拒绝（单写者）；非终态未知状态 → 拒绝（没有事实不猜测）；`revision <= 0` → 拒绝；终态 + 有 revision → 放行。

- [ ] **Step 1: 写失败测试**

`packages/contracts/test/mobile-execution.test.ts` 追加（该文件为 node:test 的 `test(...)` 风格；`evaluateQueueNext` 并入顶部 `import { ... } from '../src/mobile/execution.ts'`）：

```ts
import { evaluateQueueNext } from '../src/mobile/execution.ts'; // 并入既有 import 行

const queueNextBase = { schema_version: 1, run_id: 'r1', session_id: 's1', revision: 3, driver: 'platform', run_status: 'queued', execution_status: 'queued', settlement_status: 'pending', seq: 1, capabilities: {} };
const queueNextExecutionOf = (runStatus: string, revision = 3) => ({ ...queueNextBase, run_status: runStatus, execution_status: runStatus, revision });

test('evaluateQueueNext rejects an active run: one task permits at most one write run', () => {
  for (const status of ['queued', 'running', 'waiting_user', 'reconciling', 'recovering'] as const) {
    const decision = evaluateQueueNext(queueNextExecutionOf(status));
    assert.equal(decision.allowed, false);
    assert.match(decision.reason, /still/);
  }
});

test('evaluateQueueNext allows a terminal run carrying a real revision', () => {
  for (const status of ['succeeded', 'failed', 'canceled'] as const) {
    assert.deepEqual(evaluateQueueNext(queueNextExecutionOf(status)), { allowed: true, reason: '' });
  }
});

test('evaluateQueueNext rejects a terminal run without a snapshot revision', () => {
  const decision = evaluateQueueNext(queueNextExecutionOf('canceled', 0));
  assert.equal(decision.allowed, false);
  assert.match(decision.reason, /revision/);
});

test('evaluateQueueNext rejects an unknown run status without guessing', () => {
  assert.equal(evaluateQueueNext(queueNextExecutionOf('warping')).allowed, false);
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/contracts/test/mobile-execution.test.ts`
Expected: FAIL（`evaluateQueueNext` 未定义）

- [ ] **Step 3: 最小实现**

`packages/contracts/src/mobile/execution.ts`——`:73` 的 `CommandAction` 扩员，`evaluateCommand` 之后新增：

```ts
/** 命令闭集，镜像 internal/workbench/interaction.go ExecutionCommand（cancel 无载荷，steer/queue_next 带文本）。 */
export type CommandAction = 'cancel' | 'steer' | 'queue_next';

export interface QueueNextDecision {
  allowed: boolean;
  reason: string;
}

/**
 * 冻结的 queue_next 准入规则（T07）：
 * 1. 活动 Run（queued/running/waiting_user/reconciling/recovering）拒绝——一个 Task 至多一个写
 *    Run，下一 Run 的准入必须等当前 Run 终态；
 * 2. 未知 run_status 不默认放行（没有事实不猜测）；
 * 3. revision<=0（无快照 revision）拒绝——命令必须携带真实 expected_revision；
 * 4. 终态 + 真实 revision 放行。与 evaluateCommand（MX-003）不同：queue_next 不查
 *    capability——它走准入（budget/槽位），不是 Run 生命周期能力。
 */
export function evaluateQueueNext(execution: ExecutionDTO): QueueNextDecision {
  if (TERMINAL_RUN_STATUS.includes(execution.run_status)) {
    if (execution.revision <= 0) {
      return { allowed: false, reason: 'command requires a snapshot revision (expected_revision)' };
    }
    return { allowed: true, reason: '' };
  }
  return { allowed: false, reason: `run still ${execution.run_status}; one task permits at most one write run` };
}
```

`packages/contracts/src/index.ts` 的 mobile 导出区（`:640` 附近）追加 `evaluateQueueNext,` 与 `type QueueNextDecision,`。

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/contracts/test/mobile-execution.test.ts`
Expected: PASS（既有 evaluateCommand 用例不受影响）

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src/mobile/execution.ts packages/contracts/src/index.ts packages/contracts/test/mobile-execution.test.ts
git commit -m "feat(contracts): queue_next command action + frozen admission rule (T07 #37)"
```

---

### Task 6: mobile-core：`TaskIntent` 合同 + `TaskHandle.act()`——三态停止、unknown 门、revision 纪律、parked queue-next

**Files:**
- Create: `packages/mobile-core/src/task-office/task-intent.ts`
- Create: `packages/mobile-core/src/task-office/task-intent.test.ts`
- Modify: `packages/mobile-core/src/task-office/task-detail.ts`（`TaskCommandPort`、`TaskDetailPorts.commands?`、`TaskHandle.act/flushQueuedIntents`、视图字段、unknown 解析）
- Modify: `packages/mobile-core/src/task-office/task-office-errors.ts:10-21`（4 个错误码）
- Modify: `packages/mobile-core/src/task-office/task-timeline.ts:67-82`（`cancellation_requested` 分类与摘要）
- Modify: `packages/mobile-core/src/task-office/task-office.ts:167-186,460-466`（`TaskOfficePorts.commands?` 与 `open()` 转发）
- Modify: `packages/mobile-core/src/index.ts`（导出新类型/函数）
- Test: `packages/mobile-core/src/task-office/task-intent.test.ts`、`packages/mobile-core/src/task-office/task-detail.test.ts`（追加）、`packages/mobile-core/src/task-office/task-timeline.test.ts`（追加一行用例）

**Interfaces:**
- Consumes: `TaskHandle`/`TaskDetailView`/`TaskDetailPorts`（`task-detail.ts:72-84`）；`isTerminalRunStatus`/`terminalRunStatusOf`（`task-timeline.ts:36-55`）；`leaseActive`（`runtime/scope-lease.ts`）；`TaskOfficeError`（`task-office-errors.ts:23`）；#38 的跨包契约码先例（`error.code` 字符串）。
- Produces（Task 7/8/9 逐字消费）:
  - `TaskIntent = { kind: 'steer'; text: string } | { kind: 'queue-next'; text: string; intentId?: string } | { kind: 'stop' }`。
  - `StopPhase = 'requested' | 'confirmed' | 'unknown'`；`InterventionOutcome = 'accepted' | 'parked' | 'conflict' | 'unknown'`。
  - `InterventionReceipt { intent: TaskIntent; outcome: InterventionOutcome; boundRunId: string; revision: number; nextRunId?: string; note?: string; at: string }`。
  - `resolveUnknownStop(observedRunStatus: string): 'confirmed' | 'not-landed'`（纯函数）。
  - `TaskCommandPort { command(input: { runId: string; action: 'steer' | 'queue_next' | 'cancel'; text?: string; expectedRevision: number; intentId?: string }): Promise<{ runId: string; action: 'steer' | 'queue_next' | 'cancel'; nextRunId?: string }> }`。
  - `TaskHandle` 增：`act(intent: TaskIntent): Promise<InterventionReceipt>`、`flushQueuedIntents(): Promise<void>`。
  - `TaskDetailView` 增可选字段：`stop?: { phase: StopPhase; since: string; note?: string }`、`queuedNext?: Array<{ intentId: string; text: string; queuedAt: string }>`、`interventions?: InterventionReceipt[]`。
  - `TaskOfficePorts` 增 `commands?: TaskCommandPort`；`TaskOffice.open` 转发。
  - 错误码：`TASK_OFFICE_COMMAND_UNAVAILABLE`、`TASK_OFFICE_NO_SNAPSHOT`、`TASK_OFFICE_COMMAND_CONFLICT`、`TASK_OFFICE_COMMAND_UNKNOWN`。
  - 跨包契约码（api-client 抛出、本模块识别，沿 `TASK_STREAM_CURSOR_EXPIRED` 先例）：`TASK_COMMAND_CONFLICT`（HTTP 409/404 的确定性冲突）、`TASK_COMMAND_UNKNOWN`（传输失败/5xx/502 `command_recovery_unknown` 等投递结果未知）。

- [ ] **Step 1: 写失败测试（纯函数）**

创建 `packages/mobile-core/src/task-office/task-intent.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { resolveUnknownStop } from './task-intent.ts';

test('resolveUnknownStop: canceled is the only proof a stop landed (cancellation is irreversible)', () => {
  assert.equal(resolveUnknownStop('canceled'), 'confirmed');
  for (const status of ['queued', 'running', 'waiting_user', 'reconciling', 'recovering', 'succeeded', 'failed', 'anything']) {
    assert.equal(resolveUnknownStop(status), 'not-landed', status);
  }
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/mobile-core/src/task-office/task-intent.test.ts`
Expected: FAIL（模块不存在）

- [ ] **Step 3: 实现 task-intent.ts**

```ts
/**
 * TaskHandle.act 的意图合同（module-seams §5.2：act(TaskIntent) 执行 steer、queue-next、
 * stop 等受控意图；decision 已由 #38 的 office.decide 承载，share 属后续 Issue）。
 * 三个意图对应三种用户可明确表达的干预：
 * - steer：调整当前 Run（安全点注入）；
 * - queue-next：为下一 Run 排队一条指令——Run 仍在运行时由模块本地持有（单写者规则：
 *   活动 Run 占据该 Task 的写通道，服务端对活动 Run 的 queue_next 是 409），观察到终态
 *   后发出；本地持有非耐久（跨进程耐久属 #40，且持久化选型 ADR 未决，本模块不引入）；
 * - stop：停止当前 Run——请求、确认、结果未知三态分别呈现（Issue #37 AC1）。
 */
export type TaskIntent =
  | { kind: 'steer'; text: string }
  | { kind: 'queue-next'; text: string; intentId?: string }
  | { kind: 'stop' };

/** 停止三态：requested=请求已被服务端接受（202）、尚未观察到终态；confirmed=已观察到
 * canceled（或自然终态获胜——正常完成从不被改写为取消）；unknown=命令投递结果未知，
 * 核对前阻止同句柄一切后续写意图（Issue #37 AC2）。 */
export type StopPhase = 'requested' | 'confirmed' | 'unknown';

export type InterventionOutcome = 'accepted' | 'parked' | 'conflict' | 'unknown';

/** 一次干预的诚实回执：boundRunId 是指令实际绑定的 Run（AC「看到指令实际绑定的 Run」）；
 * queue-next 已被服务端准入下一 Run 时携带 nextRunId；revision 是命令携带的真实观察
 * revision（202 后的 +1 是服务端 CAS 证明，同样真实，绝无编造值）。 */
export interface InterventionReceipt {
  intent: TaskIntent;
  outcome: InterventionOutcome;
  boundRunId: string;
  revision: number;
  nextRunId?: string;
  note?: string;
  at: string;
}

/** unknown 的核对规则（快照事实驱动，取消不可逆）：观察到 canceled ⇒ 停止其实已落地；
 * 其余任何状态 ⇒ 取消 CAS 不可能已落地 ⇒ 未落地（门解除，用户可重试）。 */
export function resolveUnknownStop(observedRunStatus: string): 'confirmed' | 'not-landed' {
  return observedRunStatus === 'canceled' ? 'confirmed' : 'not-landed';
}
```

运行 `npx tsx --test packages/mobile-core/src/task-office/task-intent.test.ts` → PASS。

- [ ] **Step 4: 写失败测试（act() Interface 级——AC1/AC2 主证据）**

`packages/mobile-core/src/task-office/task-detail.test.ts` 追加（文件既有 import/setup 风格保持；下方为完整新增用例与所需 stub，执行者按文件头部实际 helper 命名对齐——本文件既有测试用 `createTaskDetail` + `createInMemoryTaskProjectionStore` + scripted backend）：

```ts
// —— 测试用命令端口 stub（内联于本测试文件；script 可变以便用例中途切换行为） ——
interface RecordedCommand { runId: string; action: 'steer' | 'queue_next' | 'cancel'; text?: string; expectedRevision: number; intentId?: string }
interface CommandScript { result?: { runId: string; action: RecordedCommand['action']; nextRunId?: string }; error?: unknown }
function scriptedCommandPort(initial: CommandScript = {}): { port: { command(input: RecordedCommand): Promise<{ runId: string; action: RecordedCommand['action']; nextRunId?: string }> }; calls: RecordedCommand[]; script: CommandScript } {
  const calls: RecordedCommand[] = [];
  const script: CommandScript = { ...initial };
  return {
    calls,
    script,
    port: {
      async command(input) {
        calls.push(input);
        if (script.error !== undefined) throw script.error;
        return script.result ?? { runId: input.runId, action: input.action };
      },
    },
  };
}
const conflictError = (): Error => { const e = new Error('TASK_COMMAND_CONFLICT'); (e as unknown as { code?: string }).code = 'TASK_COMMAND_CONFLICT'; return e; };
const unknownError = (): Error => { const e = new Error('TASK_COMMAND_UNKNOWN'); (e as unknown as { code?: string }).code = 'TASK_COMMAND_UNKNOWN'; return e; };

// —— handle 工厂：可变 detail stub + in-memory 投影 + 活跃 lease ——
function detailOf(execution: { runStatus: string; revision: number }) {
  return {
    taskId: 's1', runId: 'run-1', title: 't', attention: 'none' as const,
    execution: { runStatus: execution.runStatus, executionStatus: execution.runStatus, settlementStatus: 'pending', revision: execution.revision, seq: 0 },
    watermark: 0, incomplete: false, events: [],
  };
}
function newHandleForIntervention(execution: { runStatus: string; revision: number }, commands?: TaskCommandPort, options: { skipHydrate?: boolean } = {}): { handle: TaskHandle; detailBackend: { detailResult: TaskBackendDetail; detail(runId: string): Promise<TaskBackendDetail>; stream(input: never): Promise<void> } } {
  const detailBackend = {
    detailResult: detailOf(execution),
    async detail(_runId: string) { return this.detailResult; },
    async stream(_input: never): Promise<void> { /* 立即结束的空流：终态 detail 走 hydrate 的 drained 分支 */ },
  };
  const handle = createTaskDetail(
    { taskId: 's1', runId: 'run-1' },
    { backend: detailBackend, store: createInMemoryTaskProjectionStore(), lease: () => activeLease, ...(commands === undefined ? {} : { commands }) },
  );
  if (options.skipHydrate !== true) void handle.hydrate().catch(() => undefined);
  return { handle, detailBackend };
}
// activeLease 复用本文件既有用例的活跃 ScopeLease 构造（既有 #35 用例已有同构 helper——执行者对齐其命名；若无则内联一个 { active: true } 形态满足 leaseActive 的最小 lease）。
```

（上方 `newHandleForIntervention` 中 `handle.hydrate()` 的调用以测试用例内显式 `await handle.hydrate()` 为准——工厂内不 await、用例自行 await，保证 RED 阶段断言时序确定。`TaskCommandPort`/`TaskBackendDetail`/`TaskHandle` 从 `./task-detail.ts` import。）

```ts
test('act(stop) presents requested, then confirmed when the projection observes canceled', async () => {
  const commands = scriptedCommandPort();
  const { handle, detailBackend } = newHandleForIntervention({ runStatus: 'running', revision: 4 }, commands.port);
  await handle.hydrate();
  const receipt = await handle.act({ kind: 'stop' });
  assert.equal(receipt.outcome, 'accepted');
  assert.equal(receipt.boundRunId, 'run-1');
  assert.equal(receipt.revision, 4, 'the command must carry the observed revision');
  assert.equal(commands.calls[0]?.action, 'cancel');
  assert.equal(commands.calls[0]?.expectedRevision, 4);
  assert.equal(handle.view()?.stop?.phase, 'requested');
  // 快照/SSE 观察到 canceled → confirmed（AC1）。
  detailBackend.detailResult = detailOf({ runStatus: 'canceled', revision: 5 });
  await handle.resync();
  assert.equal(handle.view()?.stop?.phase, 'confirmed');
  handle.close('done');
});

test('act(stop) with an unknown delivery outcome gates further writes until a resync reconciles', async () => {
  const commands = scriptedCommandPort({ error: unknownError() });
  const { handle, detailBackend } = newHandleForIntervention({ runStatus: 'running', revision: 2 }, commands.port);
  await handle.hydrate();
  const receipt = await handle.act({ kind: 'stop' });
  assert.equal(receipt.outcome, 'unknown');
  assert.equal(handle.view()?.stop?.phase, 'unknown');
  // AC2：unknown 未核对前，同句柄后续写意图一律拒绝。
  await assert.rejects(() => handle.act({ kind: 'steer', text: 'x' }), /TASK_OFFICE_COMMAND_UNKNOWN/);
  await assert.rejects(() => handle.act({ kind: 'stop' }), /TASK_OFFICE_COMMAND_UNKNOWN/);
  // 核对：仍 running ⇒ 取消未落地 ⇒ 门解除、停止卡清除。
  detailBackend.detailResult = detailOf({ runStatus: 'running', revision: 2 });
  await handle.resync();
  assert.equal(handle.view()?.stop, undefined);
  const retry = await handle.act({ kind: 'stop' });
  assert.equal(retry.outcome, 'accepted');
  handle.close('done');
});

test('act(stop) unknown reconciles to confirmed when the run was actually canceled', async () => {
  const commands = scriptedCommandPort({ error: unknownError() });
  const { handle, detailBackend } = newHandleForIntervention({ runStatus: 'running', revision: 2 }, commands.port);
  await handle.hydrate();
  await handle.act({ kind: 'stop' });
  detailBackend.detailResult = detailOf({ runStatus: 'canceled', revision: 3 });
  await handle.resync();
  assert.equal(handle.view()?.stop?.phase, 'confirmed');
  handle.close('done');
});

test('a 409 conflict is a receipt, not the unknown gate', async () => {
  const commands = scriptedCommandPort({ error: conflictError() });
  const { handle } = newHandleForIntervention({ runStatus: 'running', revision: 1 }, commands.port);
  await handle.hydrate();
  const receipt = await handle.act({ kind: 'stop' });
  assert.equal(receipt.outcome, 'conflict');
  assert.equal(handle.view()?.stop, undefined, 'a conflict leaves no stop card');
  // 冲突不进门：后续意图仍可发出。
  commands.script.error = undefined;
  const next = await handle.act({ kind: 'steer', text: 'go' });
  assert.equal(next.outcome, 'accepted');
  handle.close('done');
});

test('act(queue-next) on an active run parks locally and flushes on terminal observation', async () => {
  const commands = scriptedCommandPort();
  const { handle, detailBackend } = newHandleForIntervention({ runStatus: 'running', revision: 7 }, commands.port);
  await handle.hydrate();
  const parked = await handle.act({ kind: 'queue-next', text: 'next instruction' });
  assert.equal(parked.outcome, 'parked', 'never claim a server queue that does not exist');
  assert.equal(commands.calls.length, 0);
  assert.deepEqual(handle.view()?.queuedNext?.map((q) => q.text), ['next instruction']);
  // 观察到终态 → flush 发出 queue_next，服务端准入下一 Run。
  detailBackend.detailResult = detailOf({ runStatus: 'succeeded', revision: 7 });
  await handle.resync();
  await handle.flushQueuedIntents();
  assert.equal(commands.calls.length, 1);
  assert.equal(commands.calls[0]?.action, 'queue_next');
  assert.equal(commands.calls[0]?.intentId !== undefined, true, 'parked intents fire with a stable idempotency id');
  const receipt = handle.view()?.interventions?.at(-1);
  assert.equal(receipt?.outcome, 'accepted');
  assert.equal(handle.view()?.queuedNext?.length, 0);
  handle.close('done');
});

test('act(queue-next) on a terminal run dispatches immediately', async () => {
  const commands = scriptedCommandPort({ result: { runId: 'run-1', action: 'queue_next', nextRunId: 'run-2' } });
  const { handle } = newHandleForIntervention({ runStatus: 'canceled', revision: 6 }, commands.port);
  await handle.hydrate();
  const receipt = await handle.act({ kind: 'queue-next', text: 'restart now' });
  assert.equal(receipt.outcome, 'accepted');
  assert.equal(receipt.boundRunId, 'run-1');
  assert.equal(receipt.nextRunId, 'run-2');
  handle.close('done');
});

test('act before hydrate and a missing commands port fail closed', async () => {
  const noPort = newHandleForIntervention({ runStatus: 'running', revision: 1 });
  await assert.rejects(() => noPort.handle.act({ kind: 'stop' }), /TASK_OFFICE_COMMAND_UNAVAILABLE/);
  const withPort = newHandleForIntervention({ runStatus: 'running', revision: 1 }, scriptedCommandPort().port, { skipHydrate: true });
  await assert.rejects(() => withPort.handle.act({ kind: 'stop' }), /TASK_OFFICE_NO_SNAPSHOT/);
  noPort.handle.close('done'); withPort.handle.close('done');
});

test('a command landing while the lease died is rejected, never silently recorded', async () => {
  // Scope Lease 失效后丢弃迟到结果（module-seams §5.3）：lease 在命令在途时撤销 → SCOPE_CHANGED。
  let release: (() => void) | undefined;
  const slowPort = { command: () => new Promise<{ runId: string; action: 'cancel' as const }>((resolve) => { release = () => resolve({ runId: 'run-1', action: 'cancel' }); }) };
  const { handle } = newHandleForIntervention({ runStatus: 'running', revision: 9 }, slowPort);
  await handle.hydrate();
  const pending = handle.act({ kind: 'stop' });
  revokeLease(); // 复用本文件既有的 lease 撤销 helper（#35 用例已有；无则把 activeLease 换为失效对象）
  release!();
  await assert.rejects(() => pending, /TASK_OFFICE_SCOPE_CHANGED/);
  assert.equal(handle.view()?.interventions?.length, 0, 'a late result must not be recorded as an intervention');
  handle.close('done');
});
```

- [ ] **Step 5: 运行确认失败**

Run: `npx tsx --test packages/mobile-core/src/task-office/task-detail.test.ts`
Expected: FAIL（`act`/`flushQueuedIntents`/`stop` 字段不存在——编译/断言失败）

- [ ] **Step 6: 最小实现（task-detail.ts 等）**

6a. `task-office-errors.ts` 错误码联合追加 4 值：

```ts
  | 'TASK_OFFICE_COMMAND_UNAVAILABLE'
  | 'TASK_OFFICE_NO_SNAPSHOT'
  | 'TASK_OFFICE_COMMAND_CONFLICT'
  | 'TASK_OFFICE_COMMAND_UNKNOWN'
```

6b. `task-timeline.ts`——`KIND_OF_TYPE` 增 `'cancellation_requested': 'run_status'`；`EVENT_SUMMARIES` 增 `'cancellation_requested': '已请求停止'`。（`task-timeline.test.ts` 追加一行：`projectTimeline([{ seq: 1, type: 'cancellation_requested', occurredAt: '...', payload: {} }])[0].summary === '已请求停止'`。）

6c. `task-detail.ts`——import 区增 `import type { InterventionReceipt, StopPhase, TaskIntent } from './task-intent.ts';` 与 `import { resolveUnknownStop } from './task-intent.ts';`。类型与端口：

```ts
export type TaskCommandAction = 'steer' | 'queue_next' | 'cancel';

/** T07 命令 seam：实现方（api-client remote）以结构化 `code` 错误表达确定性冲突
 * （TASK_COMMAND_CONFLICT）与投递结果未知（TASK_COMMAND_UNKNOWN）。 */
export interface TaskCommandPort {
  command(input: { runId: string; action: TaskCommandAction; text?: string; expectedRevision: number; intentId?: string }): Promise<{ runId: string; action: TaskCommandAction; nextRunId?: string }>;
}

export interface TaskDetailPorts {
  backend: TaskDetailBackendPort;
  store: TaskProjectionStore;
  lease(): ScopeLease | undefined;
  /** T07 干预通道；缺失时 act() fail closed（TASK_OFFICE_COMMAND_UNAVAILABLE）。 */
  commands?: TaskCommandPort;
}
```

`TaskDetailView` 追加可选字段（可选保证既有构造点零破坏）：

```ts
  stop?: { phase: StopPhase; since: string; note?: string };
  queuedNext?: Array<{ intentId: string; text: string; queuedAt: string }>;
  interventions?: InterventionReceipt[];
```

`TaskHandle` 接口追加两方法：`act(intent: TaskIntent): Promise<InterventionReceipt>;` 与 `flushQueuedIntents(): Promise<void>;`。

`createTaskDetail` 内部新增状态与逻辑（放在既有状态声明区之后；`buildView` 返回对象追加三个字段）：

```ts
  let stopState: TaskDetailView['stop'];
  let unknownGate: { revision: number } | undefined;
  let revisionFloor = 0; // 202 后服务端 CAS 证明的 revision+1（steer/cancel 各 +1）
  const queuedNext: Array<{ intentId: string; text: string; queuedAt: string }> = [];
  const interventions: InterventionReceipt[] = [];
  const commandRevision = (): number => Math.max(detail?.execution.revision ?? 0, revisionFloor);
  const currentRunStatus = (): string => (detail === undefined ? '' : terminalRunStatusOf(detail.execution.runStatus, events));
  const nextIntentId = (): string => {
    if (typeof globalThis.crypto?.randomUUID === 'function') return globalThis.crypto.randomUUID();
    throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT', { cause: new Error('no id generator on this platform') });
  };
  const trimInterventions = (): void => { if (interventions.length > 20) interventions.splice(0, interventions.length - 20); };
  const messageOf = (failure: unknown): string => (failure instanceof Error ? failure.message : String(failure));
  const wrapCommand = async <T>(action: () => Promise<T>): Promise<T> => {
    try {
      return await action();
    } catch (error) {
      const code = (error as { code?: unknown } | null)?.code;
      if (code === 'TASK_COMMAND_CONFLICT' || code === 'TASK_COMMAND_UNKNOWN') throw error; // 跨包契约码透传（#38 先例）
      if (error instanceof TaskOfficeError) throw error;
      throw new TaskOfficeError('TASK_OFFICE_BACKEND', { cause: error });
    }
  };
  const stopProjection = (runStatus: string): TaskDetailView['stop'] => {
    if (stopState === undefined) return undefined;
    if (stopState.phase === 'unknown') return stopState;
    if (isTerminalRunStatus(runStatus)) {
      return { ...stopState, phase: 'confirmed', ...(runStatus === 'canceled' ? {} : { note: `run ended as ${runStatus} before the stop landed` }) };
    }
    return stopState;
  };
```

`buildView` 的返回对象追加：

```ts
      ...(stopProjection(runStatus) === undefined ? {} : { stop: stopProjection(runStatus) }),
      ...(queuedNext.length === 0 ? {} : { queuedNext: [...queuedNext] }),
      ...(interventions.length === 0 ? {} : { interventions: [...interventions] }),
```

（`buildView` 内已有局部 `runStatus` 变量，直接复用。）`hydrate` 在 `detail = fetched;` 与事件合并之后、终态判定之前插入 unknown 解析：

```ts
    if (unknownGate !== undefined) {
      const resolved = resolveUnknownStop(fetched.execution.runStatus);
      if (resolved === 'confirmed') {
        stopState = { phase: 'confirmed', since: stopState?.since ?? new Date().toISOString(), note: 'reconciled: canceled' };
      } else {
        stopState = undefined; // 取消未落地：门解除，用户可重试。
      }
      unknownGate = undefined;
    }
```

`hydrate` 的终态分支（既有 `if (isTerminalRunStatus(...)) { notify('drained'); return current!; }`）在 return 前追加 `void flushQueuedIntents().catch(() => undefined);`。返回对象追加两个方法（放在 `close` 之前）：

```ts
    async act(intent: TaskIntent): Promise<InterventionReceipt> {
      requireOpen();
      const lease = requireLease(); // 捕获在途 lease：命令飞行期间撤销 → 迟到结果按 SCOPE_CHANGED 拒绝（§5.3）
      const commands = ports.commands;
      if (commands === undefined) throw new TaskOfficeError('TASK_OFFICE_COMMAND_UNAVAILABLE');
      if (detail === undefined) throw new TaskOfficeError('TASK_OFFICE_NO_SNAPSHOT');
      if (unknownGate !== undefined) throw new TaskOfficeError('TASK_OFFICE_COMMAND_UNKNOWN');
      const text = intent.kind === 'stop' ? '' : intent.text.trim();
      if ((intent.kind === 'steer' || intent.kind === 'queue-next') && text === '') throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');
      const revision = commandRevision();
      const at = new Date().toISOString();
      if (intent.kind === 'queue-next' && !isTerminalRunStatus(currentRunStatus())) {
        const parked = { intentId: intent.intentId ?? nextIntentId(), text, queuedAt: at };
        queuedNext.push(parked);
        const receipt: InterventionReceipt = { intent, outcome: 'parked', boundRunId: input.runId, revision, at };
        interventions.push(receipt); trimInterventions();
        notify(current?.connection === undefined ? 'syncing' : current.connection);
        return receipt;
      }
      const intentId = intent.kind === 'queue-next' ? intent.intentId ?? nextIntentId() : undefined;
      const dispatch = intent.kind === 'stop'
        ? { runId: input.runId, action: 'cancel' as const, expectedRevision: revision }
        : intent.kind === 'steer'
          ? { runId: input.runId, action: 'steer' as const, text, expectedRevision: revision }
          : { runId: input.runId, action: 'queue_next' as const, text, expectedRevision: revision, ...(intentId === undefined ? {} : { intentId }) };
      try {
        const ack = await wrapCommand(() => commands.command(dispatch));
        if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED'); // 迟到结果拒绝（§5.3）
        if (dispatch.action === 'steer' || dispatch.action === 'cancel') revisionFloor = revision + 1; // 服务端 CAS 证明
        if (dispatch.action === 'cancel') stopState = { phase: 'requested', since: at };
        if (dispatch.action === 'queue_next') {
          const index = queuedNext.findIndex((q) => q.intentId === intentId);
          if (index >= 0) queuedNext.splice(index, 1);
        }
        const receipt: InterventionReceipt = {
          intent, outcome: 'accepted', boundRunId: ack.runId === '' ? input.runId : ack.runId, revision,
          ...(ack.nextRunId === undefined ? {} : { nextRunId: ack.nextRunId }), at,
        };
        interventions.push(receipt); trimInterventions();
        void hydrate().catch(() => undefined); // 重观察（一次；失败由流/下次 hydrate 兜底）
        return receipt;
      } catch (error) {
        if (error instanceof TaskOfficeError) throw error;
        const code = (error as { code?: unknown } | null)?.code;
        if (code === 'TASK_COMMAND_CONFLICT') {
          const receipt: InterventionReceipt = { intent, outcome: 'conflict', boundRunId: input.runId, revision, note: messageOf(error), at };
          interventions.push(receipt); trimInterventions();
          void hydrate().catch(() => undefined);
          return receipt;
        }
        // 结果未知（传输失败/5xx/502 command_recovery_unknown）：进入 unknown 门（AC2）。
        if (intent.kind === 'stop') stopState = { phase: 'unknown', since: at, note: messageOf(error) };
        unknownGate = { revision };
        const receipt: InterventionReceipt = { intent, outcome: 'unknown', boundRunId: input.runId, revision, note: messageOf(error), at };
        interventions.push(receipt); trimInterventions();
        return receipt;
      }
    },
    async flushQueuedIntents(): Promise<void> {
      requireOpen();
      if (ports.commands === undefined) throw new TaskOfficeError('TASK_OFFICE_COMMAND_UNAVAILABLE');
      while (queuedNext.length > 0 && unknownGate === undefined) {
        if (!isTerminalRunStatus(currentRunStatus())) return;
        const next = queuedNext[0]!;
        const dispatch = { runId: input.runId, action: 'queue_next' as const, text: next.text, expectedRevision: commandRevision(), intentId: next.intentId };
        try {
          const ack = await wrapCommand(() => ports.commands!.command(dispatch));
          queuedNext.shift();
          interventions.push({ intent: { kind: 'queue-next', text: next.text, intentId: next.intentId }, outcome: 'accepted', boundRunId: input.runId, revision: dispatch.expectedRevision, ...(ack.nextRunId === undefined ? {} : { nextRunId: ack.nextRunId }), at: new Date().toISOString() });
        } catch (error) {
          // one-shot：一次失败的 flush 不重试、不静默丢弃——以回执如实呈现后移除。
          queuedNext.shift();
          const code = (error as { code?: unknown } | null)?.code;
          interventions.push({ intent: { kind: 'queue-next', text: next.text, intentId: next.intentId }, outcome: code === 'TASK_COMMAND_CONFLICT' ? 'conflict' : 'unknown', boundRunId: input.runId, revision: dispatch.expectedRevision, note: messageOf(error), at: new Date().toISOString() });
          if (code !== 'TASK_COMMAND_CONFLICT') unknownGate = { revision: dispatch.expectedRevision };
        }
        trimInterventions();
      }
      if (current !== undefined) notify(current.connection);
    },
```

6d. `task-office.ts`——`TaskOfficePorts` 增 `commands?: TaskCommandPort;`（import type 自 `./task-detail.ts`），`open()` 内 `createTaskDetail` 调用（`:465`）的 ports 对象追加 `...(ports.commands === undefined ? {} : { commands: ports.commands })`。

6e. `index.ts` 追加导出：

```ts
export { resolveUnknownStop } from './task-office/task-intent.ts';
export type { InterventionOutcome, InterventionReceipt, StopPhase, TaskIntent } from './task-office/task-intent.ts';
```

并把 `TaskCommandAction, TaskCommandPort` 并入既有 `task-detail.ts` 的 type 导出行（`:36-39`）。

- [ ] **Step 7: 运行确认通过**

Run: `npx tsx --test packages/mobile-core/src/task-office/task-intent.test.ts packages/mobile-core/src/task-office/task-detail.test.ts packages/mobile-core/src/task-office/task-timeline.test.ts packages/mobile-core/src/task-office/task-office.test.ts packages/mobile-core/src/task-office/task-office-start.test.ts`
Expected: 全部 PASS（既有 #35/#36 用例零回归）

- [ ] **Step 8: Commit**

```bash
git add packages/mobile-core/src/task-office/ packages/mobile-core/src/index.ts
git commit -m "feat(mobile-core): TaskHandle.act — steer/queue-next/stop with three-phase stop and unknown gating (T07 #37)"
```

---

### Task 7: api-client：queue_next wire + `createTaskOfficeRemote.command`（跨包契约码）

**Files:**
- Modify: `packages/api-client/src/mobile/executions.ts:27-35,215-221,241-249`（输入联合/ack/校验/解析扩 queue_next）
- Modify: `packages/api-client/src/mobile/task-office.ts`（新增 `command` 方法与类型）
- Test: `packages/api-client/src/mobile/executions.test.ts`、`packages/api-client/src/mobile/task-office.test.ts`（追加）

**Interfaces:**
- Consumes: `createExecutionsApi.command`（`executions.ts:297-302`）；`ApiError`（`../errors.ts`）；`#38 decide` 的 coded-error 先例（`task-office.ts:225-238`）；Task 6 的 `TaskCommandPort` 结构（remote 以结构化类型满足，无需 import）。
- Produces（Task 8/9 消费）:
  - `ExecutionCommandInput` 联合增 `{ action: 'queue_next'; text: string; expected_revision: number; external_pending_id?: string }`；`CommandAck.action: 'cancel' | 'steer' | 'queue_next'`、`CommandAck.next_run_id?: string`。
  - `createTaskOfficeRemote` 新增 `command(input: RemoteTaskCommandInput): Promise<RemoteTaskCommandAck>`；`RemoteTaskCommandInput = { runId: string; action: 'steer' | 'queue_next' | 'cancel'; text?: string; expectedRevision: number; intentId?: string }`；`RemoteTaskCommandAck = { runId: string; action: 'steer' | 'queue_next' | 'cancel'; nextRunId?: string }`。
  - 错误翻译（跨包契约码，不得改名）：ApiError 409/404 → `code: 'TASK_COMMAND_CONFLICT'`；其余 ApiError（含 502 `command_recovery_unknown`）与非 ApiError 传输错误 → `code: 'TASK_COMMAND_UNKNOWN'`。

- [ ] **Step 1: 写失败测试**

`packages/api-client/src/mobile/task-office.test.ts` 追加（沿用该文件既有的 fake `request` 构造风格）：

```ts
test('task office remote command: 202 ack maps to runId/nextRunId; 409 and transport failures map to contract codes', async () => {
  const seen: ClientRequest[] = [];
  const request = async (input: ClientRequest): Promise<unknown> => {
    seen.push(input);
    const action = typeof input.body === 'object' && input.body !== null && (input.body as { action?: string }).action === 'queue_next' ? 'queue_next' : 'cancel';
    return { success: true, data: { run_id: 'run-1', action, next_run_id: 'run-2' } };
  };
  const remote = createTaskOfficeRemote({ origin: 'https://weknora.example', request });
  const ack = await remote.command({ runId: 'run-1', action: 'queue_next', text: 'next', expectedRevision: 5, intentId: 'qid-1' });
  assert.equal(ack.nextRunId, 'run-2');
  assert.equal(seen[0]?.path, '/api/v1/workbench/executions/run-1/commands');
  assert.deepEqual(seen[0]?.body, { action: 'queue_next', text: 'next', expected_revision: 5, external_pending_id: 'qid-1' });

  const conflictRemote = createTaskOfficeRemote({ origin: 'https://weknora.example', request: async () => { throw new ApiError({ status: 409, code: 'HTTP_409', message: 'conflict' }); } });
  await assert.rejects(
    () => conflictRemote.command({ runId: 'run-1', action: 'cancel', expectedRevision: 5 }),
    (error: unknown) => (error as { code?: string }).code === 'TASK_COMMAND_CONFLICT',
  );

  const transportRemote = createTaskOfficeRemote({ origin: 'https://weknora.example', request: async () => { throw new Error('network down'); } });
  await assert.rejects(
    () => transportRemote.command({ runId: 'run-1', action: 'cancel', expectedRevision: 5 }),
    (error: unknown) => (error as { code?: string }).code === 'TASK_COMMAND_UNKNOWN',
  );
});
```

（`ClientRequest` 已是该文件既有 import；`ApiError({ status, code, message })` 构造形态与该文件 `:259` 既有用例一致。）`executions.test.ts` 的既有 command 用例（`:61-82,142` 附近）追加 queue_next 分支：POST body 携带 `external_pending_id`、ack 透传 `next_run_id`、活动 Run 的 409 不在 api-client 层翻译（原始 ApiError 透传给上层 remote——由 remote.command 统一翻译）。

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/api-client/src/mobile/task-office.test.ts packages/api-client/src/mobile/executions.test.ts`
Expected: FAIL（`command` 方法不存在 / queue_next 输入被 `validateCommand` 拒绝）

- [ ] **Step 3: 最小实现**

`executions.ts`：

```ts
export interface CommandAck {
  run_id: string;
  action: 'cancel' | 'steer' | 'queue_next';
  next_run_id?: string;
}

/** Commands are intentionally a closed union: arbitrary method/body pairs are not exposed. */
export type ExecutionCommandInput =
  | { action: 'cancel'; expected_revision: number }
  | { action: 'steer'; text: string; expected_revision: number }
  | { action: 'queue_next'; text: string; expected_revision: number; external_pending_id?: string };
```

`validateCommand` 追加分支（替换 `:241-249` 的 action 判定）：

```ts
function validateCommand(input: ExecutionCommandInput): void {
  if (input.action !== 'cancel' && input.action !== 'steer' && input.action !== 'queue_next') throw new Error('command action is invalid');
  positiveSafeInteger(input.expected_revision, 'expected_revision');
  if (input.action === 'cancel') {
    if ('text' in input) throw new Error('cancel command cannot include text');
  } else if (typeof input.text !== 'string' || input.text.trim() === '') {
    throw new Error(`${input.action} command text must not be empty`);
  }
}
```

`parseCommandAck`（`:215-221`）返回值追加 `...(typeof row.next_run_id === 'string' && row.next_run_id !== '' ? { next_run_id: row.next_run_id } : {})`。

`task-office.ts`——类型与 `createTaskOfficeRemote` 返回对象内新增方法（放在 `decide` 之后）：

```ts
export interface RemoteTaskCommandInput { runId: string; action: 'steer' | 'queue_next' | 'cancel'; text?: string; expectedRevision: number; intentId?: string }
export interface RemoteTaskCommandAck { runId: string; action: 'steer' | 'queue_next' | 'cancel'; nextRunId?: string }

    async command(input: RemoteTaskCommandInput): Promise<RemoteTaskCommandAck> {
      const wire = input.action === 'cancel'
        ? { action: 'cancel' as const, expected_revision: input.expectedRevision }
        : { action: input.action, text: input.text ?? '', expected_revision: input.expectedRevision, ...(input.intentId === undefined ? {} : { external_pending_id: input.intentId }) };
      try {
        const ack = await executionsApi.command(input.runId, wire);
        return { runId: ack.run_id, action: ack.action, ...(ack.next_run_id === undefined ? {} : { nextRunId: ack.next_run_id }) };
      } catch (error) {
        // 跨包契约码（沿 TASK_STREAM_CURSOR_EXPIRED / INTERACTION_* 先例，不得改名）：
        // 409/404 是确定性冲突；其余（5xx、502 command_recovery_unknown、传输失败）投递结果未知。
        const coded = (message: string): Error => {
          const translated = new Error(message);
          (translated as unknown as { code?: string }).code = message;
          throw translated;
        };
        if (error instanceof ApiError) {
          if (error.status === 409 || error.status === 404) coded('TASK_COMMAND_CONFLICT');
          coded('TASK_COMMAND_UNKNOWN');
        }
        coded('TASK_COMMAND_UNKNOWN');
      }
    },
```

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/api-client/src/mobile/task-office.test.ts packages/api-client/src/mobile/executions.test.ts && npx tsx --test packages/contracts/test/mobile-execution.test.ts`
Expected: 全部 PASS

- [ ] **Step 5: Commit**

```bash
git add packages/api-client/src/mobile/executions.ts packages/api-client/src/mobile/task-office.ts packages/api-client/src/mobile/executions.test.ts packages/api-client/src/mobile/task-office.test.ts
git commit -m "feat(api-client): queue_next wire + task office command channel with contract codes (T07 #37)"
```

---

### Task 8: apps/mobile：干预 UI（三态停止卡 + 绑定 Run + 排队意图）与装配

**Files:**
- Modify: `apps/mobile/src/task-detail-view.ts`（控制器 `act` + 新错误文案）
- Modify: `apps/mobile/src/screens/TaskDetailScreen.tsx`（干预区：steer/queue-next/stop + 三态停止卡 + 回执列表）
- Modify: `apps/mobile/src/app/tasks/detail.tsx`（路由生命周期把 `onAct` 接入 Screen）
- Modify: `apps/mobile/src/composition.ts:153-173`（`taskOfficeFor` 装配 `commands: remote`）
- Test: `apps/mobile/src/task-detail-view.test.ts`（追加）

**Interfaces:**
- Consumes: Task 6 的 `TaskHandle.act/flushQueuedIntents`、`TaskIntent/InterventionReceipt/StopPhase`（`@weknora/mobile-core` 导出）；Task 7 的 `remote.command`（remote 对象结构化满足 `TaskCommandPort`）；既有 `createTaskDetailController`（`task-detail-view.ts:33`）与 `TaskDetailScreen` props（`screens/TaskDetailScreen.tsx:6-12`）。
- Produces（Task 9 消费）: `TaskDetailController` 增 `act(intent: TaskIntent): Promise<InterventionReceipt>`；`TaskDetailScreenProps` 增 `onAct?: (intent: TaskIntent) => Promise<InterventionReceipt>`；`TASK_OFFICE_ERROR_COPY` 增 4 条新码文案；三态停止卡文案常量 `STOP_PHASE_COPY: Record<StopPhase, string>`。

- [ ] **Step 1: 写失败测试**

`apps/mobile/src/task-detail-view.test.ts` 追加（沿用既有 fake handle 构造风格）：

```ts
test('controller.act forwards intents to the handle and surfaces receipts through state', async () => {
  const { controller, handle } = newControllerWithCommands(); // 既有 fake handle 构造 + act stub（按文件内既有 helper 风格补齐）
  const receipt = await controller.act({ kind: 'stop' });
  assert.equal(receipt.outcome, 'accepted');
  assert.equal(handle.acts.length, 1);
  assert.deepEqual(handle.acts[0], { kind: 'stop' });
});

test('controller.act maps TaskOfficeError codes to user copy instead of raw codes', async () => {
  const { controller } = newControllerWithCommands({ actError: new TaskOfficeError('TASK_OFFICE_COMMAND_UNKNOWN') });
  await assert.rejects(() => controller.act({ kind: 'stop' }));
  // 文案映射表必须覆盖新码（Screen 兜底渲染用）。
  assert.match(TASK_OFFICE_ERROR_COPY.TASK_OFFICE_COMMAND_UNKNOWN, /核对/);
});
```

（`newControllerWithCommands` 为本文件新增 helper：在既有 fake `TaskHandle` 上加 `acts` 记录数组与可注入 `actError`；`TASK_OFFICE_ERROR_COPY` 从 `./task-detail-view.ts` import 已存在于既有用例。）另在 `apps/mobile/src/app-smoke.test.tsx`（若既有源级断言文件风格允许）追加一条源级断言：composition 的 taskOfficeFor 装配含 `commands:`（grep 源码字符串，防回归——与 #35 `detail:\s*remote` 守卫同法；若该文件不存在此机制则跳过此断言，以 typecheck 兜底）。

- [ ] **Step 2: 运行确认失败**

Run: `pnpm --filter @weknora/mobile test`
Expected: FAIL（`controller.act` 不存在 / 文案缺新码）

- [ ] **Step 3: 最小实现**

`task-detail-view.ts`——`TASK_OFFICE_ERROR_COPY` 追加：

```ts
  TASK_OFFICE_COMMAND_UNAVAILABLE: '当前部署未提供运行干预通道。',
  TASK_OFFICE_NO_SNAPSHOT: '任务快照尚未同步，请稍候再试。',
  TASK_OFFICE_COMMAND_CONFLICT: '任务状态已变化，正在刷新最新状态。',
  TASK_OFFICE_COMMAND_UNKNOWN: '指令结果未知，正在与服务端核对；核对完成前暂不能下达新指令。',
```

`TaskDetailController` 接口追加 `act(intent: TaskIntent): Promise<InterventionReceipt>;`，`createTaskDetailController` 返回对象追加：

```ts
    act(intent: TaskIntent) { return handle.act(intent); },
```

（import type `TaskIntent, InterventionReceipt` 自 `@weknora/mobile-core`。）

`TaskDetailScreen.tsx`——props 追加 `onAct?: (intent: TaskIntent) => Promise<InterventionReceipt>`；组件内新增输入态与干预区（放在状态卡之后、时间线之前）：

```tsx
const STOP_PHASE_COPY: Record<StopPhase, string> = {
  requested: '停止请求已发出，等待运行确认停止',
  confirmed: '停止已确认：运行已取消',
  unknown: '停止结果未知：正在与服务端核对，核对完成前不能下达新指令',
};
const OUTCOME_COPY: Record<InterventionReceipt['outcome'], string> = {
  accepted: '已受理', parked: '已排队（等待当前 Run 结束后发出）', conflict: '状态冲突，请刷新后重试', unknown: '结果未知，核对中',
};

// 组件体内：
const [draft, setDraft] = useState('');
const run = (intent: TaskIntent): void => {
  if (onAct === undefined) return;
  void onAct(intent).then(() => setDraft(''), () => undefined);
};
// 渲染（view !== undefined 分支内）：
{view.stop !== undefined && <Text>{STOP_PHASE_COPY[view.stop.phase]}</Text>}
{onAct !== undefined && (
  <View>
    <Text>运行干预</Text>
    <TextInput value={draft} onChangeText={setDraft} placeholder="调整或排队下一 Run 的指令" />
    <Button title="调整当前运行" onPress={() => run({ kind: 'steer', text: draft })} disabled={draft.trim() === ''} />
    <Button title="排队下一 Run" onPress={() => run({ kind: 'queue-next', text: draft })} disabled={draft.trim() === ''} />
    <Button title="停止运行" onPress={() => run({ kind: 'stop' })} />
  </View>
)}
{(view.interventions ?? []).slice(-5).reverse().map((receipt, index) => (
  <View key={`${receipt.at}-${index}`}>
    <Text>{OUTCOME_COPY[receipt.outcome]} · 绑定 Run {receipt.boundRunId}{receipt.nextRunId === undefined ? '' : ` · 下一 Run ${receipt.nextRunId}`}</Text>
  </View>
))}
{(view.queuedNext ?? []).length > 0 && <Text>已排队待发：{view.queuedNext.map((q) => q.text).join('；')}</Text>}
```

（`TextInput` 从 `react-native` import 并入既有 import 行；`TaskIntent/InterventionReceipt/StopPhase` type import 自 `@weknora/mobile-core`。）

`app/tasks/detail.tsx`——`TaskDetailRouteLifecycle` 把 `onAct` 传入：`<TaskDetailScreen ... onAct={controllerRef.current === undefined ? undefined : (intent) => controllerRef.current!.act(intent)} />`（引用稳定性以 controllerRef 存续为保证；不新建闭包状态）。

`composition.ts:158` 一带的 `createTaskOffice` 调用追加 `commands: remote,`（与 `detail: remote, interactions: remote` 同对象装配，#35/#38 先例）。

- [ ] **Step 4: 运行确认通过**

Run: `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`
Expected: 全部 PASS / typecheck 零错误（typecheck 同时证明 `remote` 结构化满足 `TaskCommandPort`）

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/task-detail-view.ts apps/mobile/src/screens/TaskDetailScreen.tsx apps/mobile/src/app/tasks/detail.tsx apps/mobile/src/composition.ts apps/mobile/src/task-detail-view.test.ts
git commit -m "feat(mobile): intervention UI — steer/queue-next/stop with three-phase stop card and bound-run receipts (T07 #37)"
```

---

### Task 9: apps/mobile：opt-in 真实部署集成冒烟（AC3 live 证据）

**Files:**
- Create: `apps/mobile/src/task-intervention-integration-smoke.ts`
- Create: `apps/mobile/src/task-intervention-integration-smoke.test.ts`
- Test: `apps/mobile/src/task-intervention-integration-smoke.test.ts`

**Interfaces:**
- Consumes: `taskStartIntegrationConfig` 同款 env 语义（本文件自包含重声明，`task-start-integration-smoke.ts:13-33` 先例）；`createMobileRuntime`/`createTaskOfficeRemote`/`createTaskOffice` 装配（同文件 `:38-110`）；Task 6/7/8 的 `office.open`+`commands: remote`+`act`。
- Produces: `TaskInterventionIntegrationEvidence`（如实记录，不伪造）+ `taskInterventionIntegrationConfig(env)` + `runTaskInterventionIntegration(config)` + `emitTaskInterventionIntegrationEvidence(evidence, emit)`。

- [ ] **Step 1: 写测试（opt-in 门 + total 化结构断言）**

创建 `apps/mobile/src/task-intervention-integration-smoke.test.ts`（镜像 `task-start-integration-smoke.test.ts` 的三段结构）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { emitTaskInterventionIntegrationEvidence, runTaskInterventionIntegration, taskInterventionIntegrationConfig } from './task-intervention-integration-smoke.ts';

const env = () => process.env as Record<string, string | undefined>;

test('integration stays opt-in: missing credentials skip, never fake a pass', () => {
  const config = taskInterventionIntegrationConfig({ ...env(), WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: undefined });
  assert.equal(config.enabled, false);
  assert.equal(config.disposition, 'skip');
  const invalid = taskInterventionIntegrationConfig({ WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'http://insecure.example', WEKNORA_MOBILE_TEST_EMAIL: 'e', WEKNORA_MOBILE_TEST_PASSWORD: 'p' });
  assert.equal(invalid.enabled, false);
  assert.equal(invalid.disposition, 'invalid');
});

test('live intervention through the highest stable interface (opt-in)', async (t) => {
  const config = taskInterventionIntegrationConfig(env());
  if (!config.enabled) {
    t.skip(config.reason);
    return;
  }
  const evidence = await runTaskInterventionIntegration(config);
  const emitted: string[] = [];
  emitTaskInterventionIntegrationEvidence(evidence, (record) => { emitted.push(record); });
  assert.equal(emitted.length, 1);
  assert.equal(JSON.parse(emitted[0]!).start, evidence.start);
  assert.notEqual(evidence.steer, 'failed');
  assert.notEqual(evidence.stop, 'failed');
  assert.notEqual(evidence.queueNext, 'failed');
});

test('the integration runner is total: failures still yield evidence, never a rejection', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const source = readFileSync(join(here, 'task-intervention-integration-smoke.ts'), 'utf8');
  assert.match(source, /catch \(error\)/);
  assert.match(source, /finally\s*\{/);
});
```

- [ ] **Step 2: 运行确认失败**

Run: `pnpm --filter @weknora/mobile test`
Expected: FAIL（smoke 模块不存在）

- [ ] **Step 3: 实现 smoke**

`apps/mobile/src/task-intervention-integration-smoke.ts`（config 校验与 runtime/remote 装配逐字镜像 `task-start-integration-smoke.ts:13-84`；差异只在 goal 之后——打开句柄并按观察到的 runStatus 如实干预）：

```ts
import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createInMemoryCredentialStore, createMobileRuntime, createTaskOffice, type TaskOffice } from '@weknora/mobile-core';
import { createNativeRequestId } from './adapters/request-id.ts';

export type TaskInterventionIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface TaskInterventionIntegrationEvidence {
  deploymentOrigin: string;
  start: 'admitted' | 'pending' | 'rejected' | 'failed';
  runId?: string;
  observedRunStatus?: string;
  steer: 'accepted' | 'conflict' | 'unknown' | 'skipped-terminal' | 'failed';
  stop: 'requested-then-confirmed' | 'requested-only' | 'unknown-then-reconciled' | 'conflict' | 'skipped-terminal' | 'failed';
  queueNext: 'admitted' | 'conflict' | 'unknown' | 'skipped' | 'failed';
  boundRunId?: string;
  nextRunId?: string;
  revisionCarried?: number;
  errorReason?: string;
  timestamp: string;
}

/** 与 task-start-integration-smoke.ts 相同的 opt-in 语义（自包含，不跨计划 import）。 */
export function taskInterventionIntegrationConfig(env: Record<string, string | undefined>): TaskInterventionIntegrationConfig {
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
  return { enabled: true, deploymentOrigin: parsed.origin, email, password };
}

const TERMINAL_STATUSES = new Set(['succeeded', 'failed', 'canceled']);
const sleep = (ms: number): Promise<void> => new Promise((resolve) => { setTimeout(resolve, ms); });

/**
 * 真实端到端（AC3 live）：生产 JSON transport + Runtime 授权通道 + 具体 Remote Adapter +
 * TaskHandle.act 编排。创建一个真 Task 后按观察到的 runStatus 如实干预——活动 Run：
 * steer → stop → 有界等待 canceled → queue-next 重启；已终态 Run：queue-next 直接重启。
 * 每一步如实记录（含冲突与未知），失败落 errorReason，绝不伪造通过。
 */
export async function runTaskInterventionIntegration(config: Extract<TaskInterventionIntegrationConfig, { enabled: true }>): Promise<TaskInterventionIntegrationEvidence> {
  const evidence: TaskInterventionIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    start: 'failed', steer: 'failed', stop: 'failed', queueNext: 'skipped',
    timestamp: new Date().toISOString(),
  };
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
    const snapshot = await runtime.signIn({
      deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' },
      email: config.email,
      password: config.password,
    });
    if (snapshot.surface !== 'authorized' || !snapshot.deployment) {
      evidence.errorReason = `surface ${snapshot.surface}`;
      return evidence;
    }
    const agentsEnvelope = await runtime.authorizedRequest({ method: 'GET', path: '/api/v1/agents' }) as { success?: boolean; data?: Array<{ id?: unknown }> };
    const agentId = typeof agentsEnvelope?.data?.[0]?.id === 'string' ? agentsEnvelope.data[0].id : undefined;
    if (!agentId) {
      evidence.errorReason = 'no agent available on the deployment';
      return evidence;
    }
    const remote = createTaskOfficeRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input), stream: (input, onChunk) => runtime.authorizedEventStream(input, onChunk) });
    const office: TaskOffice = createTaskOffice({
      backend: remote, detail: remote, commands: remote,
      lease: () => runtime.scopeLease(), newRequestId: createNativeRequestId(),
    });
    const requestId = createNativeRequestId();
    const receipt = await office.start({ text: `T07 集成验证：${new Date().toISOString()}`, agentId, budgetUpper: 10 }, { requestId });
    evidence.start = receipt.phase === 'bound' ? 'admitted' : receipt.phase === 'rejected' ? 'rejected' : 'pending';
    if (receipt.phase !== 'bound' || receipt.runId === undefined) {
      evidence.errorReason = `start receipt phase ${receipt.phase}`;
      return evidence;
    }
    evidence.runId = receipt.runId;
    evidence.boundRunId = receipt.runId;
    // taskId（= sessionId，ADR-0004）：`TaskStartReceipt` 不携带会话 id，用任务列表反查（零新接口）。
    const page = await office.tasks({});
    const card = page.items.find((item) => item.runId === receipt.runId);
    if (card === undefined) {
      evidence.errorReason = 'created run not visible in the task list';
      return evidence;
    }
    const handle = office.open({ taskId: card.taskId, runId: receipt.runId });
    const view = await handle.hydrate();
    evidence.observedRunStatus = view.runStatus;
    evidence.revisionCarried = view.revision;
    if (!TERMINAL_STATUSES.has(view.runStatus)) {
      const steer = await handle.act({ kind: 'steer', text: '集成验证：继续' });
      evidence.steer = steer.outcome === 'accepted' ? 'accepted' : steer.outcome === 'conflict' ? 'conflict' : 'unknown';
      const stop = await handle.act({ kind: 'stop' });
      if (stop.outcome === 'conflict') {
        evidence.stop = 'conflict';
      } else {
        const stopPhaseAfterAck = handle.view()?.stop?.phase ?? 'requested';
        if (stopPhaseAfterAck === 'unknown') {
          // 投递结果未知：以真实 resync 核对（AC2 核对路径必须真实走通）。
          const reconciled = await handle.resync();
          evidence.stop = reconciled.runStatus === 'canceled' ? 'unknown-then-reconciled' : 'conflict';
        } else {
          let confirmed = false;
          for (let attempt = 0; attempt < 10 && !confirmed; attempt += 1) {
            await sleep(1000);
            const observed = await handle.resync();
            if (observed.runStatus === 'canceled') confirmed = true;
          }
          evidence.stop = confirmed ? 'requested-then-confirmed' : 'requested-only';
        }
      }
      const terminal = (await handle.resync());
      if (TERMINAL_STATUSES.has(terminal.runStatus)) {
        const restart = await handle.act({ kind: 'queue-next', text: '集成验证：重启' });
        evidence.queueNext = restart.outcome === 'accepted' ? 'admitted' : restart.outcome === 'conflict' ? 'conflict' : 'unknown';
        if (restart.nextRunId !== undefined) evidence.nextRunId = restart.nextRunId;
        if (restart.outcome === 'unknown') {
          const reconciled = await handle.resync(); // 核对后重试一次仍失败则如实记录
          evidence.queueNext = reconciled.runStatus === 'canceled' || TERMINAL_STATUSES.has(reconciled.runStatus) ? 'admitted' : 'unknown';
        }
      } else {
        evidence.queueNext = 'skipped';
      }
    } else {
      evidence.steer = 'skipped-terminal';
      evidence.stop = 'skipped-terminal';
      const restart = await handle.act({ kind: 'queue-next', text: '集成验证：重启' });
      evidence.queueNext = restart.outcome === 'accepted' ? 'admitted' : restart.outcome === 'conflict' ? 'conflict' : 'unknown';
      if (restart.nextRunId !== undefined) evidence.nextRunId = restart.nextRunId;
    }
    handle.close('integration-done');
    return evidence;
  } catch (error) {
    evidence.errorReason = error instanceof Error ? error.message : String(error); // 失败仍产出证据（不含凭据）
    return evidence;
  } finally {
    runtime.dispose();
  }
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitTaskInterventionIntegrationEvidence(evidence: TaskInterventionIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
```

（`handle.view()` 在停止回执后同步可读——`act` 内部已 notify；示例中的 `handle.view()` 均为同步调用。）

- [ ] **Step 4: 运行确认通过（本地无环境 → skip；有环境 → live）**

Run: `pnpm --filter @weknora/mobile test`
Expected: PASS（live 用例在无环境时 skip 并给出 reason；本计划作者环境无真实部署凭据，本地证据为 skip——不伪造）

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/task-intervention-integration-smoke.ts apps/mobile/src/task-intervention-integration-smoke.test.ts
git commit -m "test(mobile): opt-in live intervention smoke through the highest stable interface (T07 #37 AC3)"
```

---

## 计划级验证命令

在 worktree 根（`.worktrees/issue30-sweep`）执行，覆盖本计划全部测试（定向到受影响包/目录）：

```bash
go test ./internal/modules/workbench/service/workbench/ -count=1 && go test ./internal/modules/workbench/... -count=1 && go test ./internal/application/repository/ -run 'TestCancelOwnedRun|TestAgentAdoption' -count=1 && go test ./internal/handler/session/ -run 'TestWorkbench' -count=1 && npx tsx --test packages/contracts/test/mobile-execution.test.ts && npx tsx --test packages/api-client/src/mobile/executions.test.ts packages/api-client/src/mobile/task-office.test.ts && npx tsx --test packages/mobile-core/src/task-office/task-intent.test.ts packages/mobile-core/src/task-office/task-detail.test.ts packages/mobile-core/src/task-office/task-timeline.test.ts packages/mobile-core/src/task-office/task-office.test.ts packages/mobile-core/src/task-office/task-office-start.test.ts && pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck && go build ./...
```

基线说明（本计划作者在当前 HEAD 实跑）：`go test ./internal/modules/workbench/service/workbench/ -count=1` 与 `go test ./internal/handler/session/ -run 'TestWorkbench' -count=1` 在 Task 1 之前因迁移编号冲突 FAIL（见差异记录 1，Task 1 修复后必须 ok）；TS 侧基线全绿（`task-detail.test.ts`、`task-office*.test.ts`、`pnpm --filter @weknora/mobile test`（5 skipped 为既有 opt-in 冒烟）、`packages/api-client/src/mobile/{executions,task-office}.test.ts`、`packages/contracts/test/mobile-execution.test.ts` 均实跑通过）。

## 验收标准 → 证据映射

| 验收标准 | 证据 |
|---|---|
| 1. 停止请求、停止确认、结果未知分别呈现 | Task 6 Interface 测试（requested→confirmed、unknown→门+核对双路径、conflict 不进停止卡）+ Task 8 三态停止卡文案与渲染 |
| 2. 未知结果阻止冲突写 Run，所有控制命令带真实 revision | Task 6（unknown 门拒绝后续 act；revision 断言 = 观察值；202 后 CAS 证明的 +1；conflict 收据）+ Task 2/3 服务端 revision 栅栏（stale 409）+ Task 4 HTTP 全链 |
| 3. 端到端行为通过最高稳定 Interface 验证 | Task 4（Go HTTP 集成：真实迁移 sqlite + 真实协调器/handler）+ Task 6（模块 Interface 场景）+ Task 7（wire 契约字节）+ Task 9（opt-in 真实部署 live；无环境 t.skip 不伪造，本地替代证据如上） |

## Consumes-Produces 总览（供并行批次与后续计划）

- **Consumes（既有 HEAD）**：#35 TaskHandle/详情端口/SSE 恢复；#36 start/intentLog/office 装配与冒烟模式；#38 跨包契约码先例与 decide；#42/#59 迁移（Task 1 改号）；`AdmissionCoordinator` 幂等准入；`writeWorkbenchCommandError` 错误映射。
- **Produces**：上述各任务 Produces 列全体；关键对外接口——Go wire `POST /workbench/executions/:run_id/commands` 的 `queue_next`（202 ack 含 `next_run_id`；活动 Run 409；revision/owner 栅栏）、`cancellation_requested` Run 事件与取消时会话槽释放；mobile-core `TaskHandle.act(TaskIntent)`/`flushQueuedIntents()`/`TaskCommandPort`/`resolveUnknownStop`；api-client `remote.command` 与 `TASK_COMMAND_CONFLICT/UNKNOWN` 契约码；apps/mobile `TaskDetailController.act` 与三态停止卡。后续计划（#39 预算扩展、#47 批注等）经 `TaskCommandPort`/契约码复用命令通道；`share` 意图仍属后续 Issue。
