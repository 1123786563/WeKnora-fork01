# T06：通用目标输入与耐久 Task 创建（Issue #36）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 移动端获得一个统一 New 入口：用户描述目标、接受或改选系统推荐的主理 Agent（Lead Agent）、附加资源引用与预算后创建 Task；request_id 在网络发送前持久化，ACK 丢失用同一 request_id 对账，相同意图绝不重复创建 Task 或预算预占，附件未就绪/离线/输入冲突一律保留草稿且零危险重放。

**Architecture:** Go 服务端不改代码——`(tenant, actor, requestID)` 持久幂等准入（`admission.go` 的 `CreatePending` 冲突 → `resumeExisting` 原请求）与 `GET /api/v1/workbench/executions/requests/:request_id` 对账端点已在当前 HEAD，本计划只复跑其测试作为 AC1 的服务端证据。客户端把既有但零消费者的 domain 资产接入深模块：`packages/domain` 的 MX-006 提交协调器（`submission.ts`，本计划补一个 `resume` 受控重入方法：已有持久 entry 时先 lookup 对账，服务端明确 `unknown` 才以同一 request_id 重发）与 MX-015 表单模型（`task-form.ts` 的就绪裁决/七字段产出）成为 Task Office 的内部策略。持久化分两层，边界与 domain 契约严格对齐：domain `SubmissionStore` 是**同步**端口（`save(entry): void`，`submission.ts:51-56`——"实现必须先于网络调用完成写入"），只有进程内实现（office 缺省 in-memory）能真正满足它；**耐久**层由 Task Office 新增的异步意图日志端口 `SubmissionIntentLog`（`save/load/listScope/remove?`，office `await`）承载——持久化 `requestId → { sessionId, goal, scope }`：`start(goal, options)` 在无意图记录时先 `createSession`（无 Task/预算副作用的前置网络调用，与小程序 `AgentPage` 的 `sessions.create → startTask` 先例同序，`features/home/pages.tsx:26-28`），拿到 sessionId 后 **`await intentLog.save(...)`（Start POST 前的耐久落盘，磁盘满等失败上抛且零 Start 派发）**，再交给 domain 协调器；重入（`options.requestId` 已有意图记录）**不新建 session**，复用原 sessionId 重建 digest 一致的七字段输入（`inputDigest` 覆盖 `session_id`，`submission.ts:65-74`）——这是「同一意图保留相同 request_id」在跨 session/跨重启下成立的前提；goal 不一致（借旧 ID 发新意图）零网络拒绝。`packages/mobile-core` 的 `TaskOffice` 新增 `start(goal)` 与 `reconcilePending()`（module-seams §5.2 第三个外部 Interface；`reconcilePending` 从意图日志恢复并预建 entry 后逐个 lookup 对账）；`TaskBackendPort` 追加 `createSession`/`start`/`lookup`——taskId = sessionId，ADR-0004。`packages/api-client` 的 `createTaskOfficeRemote` 补三个 wire 方法（全部复用既有 `createExecutionsApi`/`createChatSessionsApi`，零新端点）。apps/mobile 新增 `/new` 路由与 `NewTaskScreen`（推荐来自 `recommendLeadAgent` 纯函数：supported 中 kind=general 优先）、基于 expo-secure-store 的持久 `SubmissionIntentLog` Adapter（App 重启后恢复意图）与 Scoped Vault 加密草稿通道（离线保留草稿；跨进程加密持久化属 #40）。

**持久化两层边界的声明（回应审查 F1/F2/F3）：** (1) 「网络前先持久化 request_id 与输入摘要」的不变量作用点是 **Start POST**（创建 Task 与预算预占的唯一副作用点）；`createSession` 是无副作用的前置网络调用，允许发生在意图落盘之前——与小程序已批准实现同序（`AgentPage` submit：`client.sessions.create` 成功后才进 `startTask` → `intent.begin()` 落盘 → `executions.start`）。「store 失败零后端调用」的测试口径因此收敛为「零 Start POST」。(2) 耐久意图记录在 **Start POST 之前** `await` 写入（`intentLog.save` 失败 → 上抛 → 零 Start 派发），这是 RN 平台上「网络前持久化」的忠实实现——SecureStore 是异步 API，进程内同步 store 只保证同进程语义，跨进程耐久必须由 office 显式 `await` 的异步端口承担，绝不把异步 save 伪装成 domain 同步端口（那会变成 fire-and-forget，破坏「save 失败不发送」）。

**Tech Stack:** TypeScript（`packages/domain`、`packages/mobile-core`、`packages/api-client`、`apps/mobile` Expo RN）、node:test + tsx（TS 测试运行器，与 `task-office.test.ts` 一致）、Go 1.26（仅复跑既有 `go test`，不改 Go 代码）、expo-secure-store（持久 Adapter）。所有测试命令在 worktree 根（`.worktrees/issue30-sweep`）执行；前置：已执行过 `pnpm install`（本计划作者已实跑：`npx tsx --test packages/mobile-core/src/task-office/task-office.test.ts` 8 pass、`npx tsx --test packages/domain/src/mobile/submission.test.ts` 5 pass、`pnpm --filter @weknora/mobile test` 72 pass、`pnpm --filter @weknora/mobile typecheck` 均绿、`go test ./internal/modules/workbench/service/workbench/...` ok、`node --experimental-strip-types --test apps/miniprogram/tests/assembly.test.mjs` 11/11 pass）。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-36.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（User Stories 9/10/11/61/62、Information Architecture「移动端采用首页、任务、新建、资源、我的五个一级入口」、Implementation Decisions、Testing Decisions）
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§5 Task Office Module——「start(goal)：持久化意图并返回 TaskStartReceipt」、§5.3 不变量、§5.4「现有 submission.ts、execution-cache.ts、session-list.ts、task-form.ts、execution-presentation.ts 和 compatibility.ts 应成为 Module 内部实现或纯策略」、§6 Resource Shelf、§10 App Shell）
- ADR：`docs/adr/0004-task-is-session.md`（taskId = sessionId，创建 Task 先有目标会话）、`docs/adr/0012-mobile-business-logic-lives-behind-deep-modules.md`（按需读取）
- 领域术语：`CONTEXT.md`（「任务（Task）」：每个从新建入口提交的初始目标都创建一个任务；「主理 Agent（Lead Agent）」：对一个任务的过程与结果承担统一责任的 Agent；系统可以推荐，用户也可以显式选择）
- Parent：Issue #30；Blocked by：#33（T03，已合并）、#35（T05，已合并）
- 前序批次产出（本计划 Consumes，全部在当前 HEAD 亲眼核实）：#34 的 `createTaskOffice`/`TaskOffice`/`TaskBackendPort`/`TaskOfficeError`（`packages/mobile-core/src/task-office/task-office.ts:142`、`task-office-errors.ts:9`）、`MobileRuntime.authorizedRequest`（`packages/mobile-core/src/runtime/mobile-runtime.ts:341` 附近，`scopeLease: () => lease` 在 `:413`）；#35 的 `createTaskOfficeRemote({ origin, request, stream? })`（`packages/api-client/src/mobile/task-office.ts:55`）、`taskOfficeFor(runtime, origin)` 工厂（`apps/mobile/src/composition.ts:109`）、`activeTaskOffice()`（`composition.ts:139`）、`RuntimeScopeLease`/`leaseActive`/`leaseScopeOf`（`packages/mobile-core/src/runtime/scope-lease.ts:11-28`）；#32 的 Scoped Vault（`packages/mobile-core/src/vault/scoped-vault.ts:56`，`ScopedStore.drafts` 的 `put/get/list/remove`）与 apps/mobile 的 `createSecureVaultKeyStore`/`createSecureVaultStorage`（`apps/mobile/src/adapters/vault-adapters.ts:6`/`:27`）；#33 的 `runtime.resourceShelf()`（`mobile-runtime.ts:414`）与 `ResourcePage.agents: readonly AgentOption[]`（`packages/mobile-core/src/shelf/types.ts:15`）。

## Global Constraints

以下为批准 Spec / ADR 的项目级约束，逐字引用，所有任务隐含遵守：

- 「移动端采用首页、任务、新建、资源、我的五个一级入口。」（mobile-ai-office-design.md · Information Architecture）
- 「9. As a member, I want one universal New entry, so that I can describe a goal without classifying it first.」（同上 · User Story 9）
- 「10. As an advanced member, I want optional model, reasoning and budget controls, so that I can override safe defaults when authorized.」（同上 · User Story 10——本计划交付其中的预算控件；模型/推理控件属后续 Issue）
- 「11. As a member, I want the system to recommend a Lead Agent and relevant resources, so that common tasks require little setup.」（同上 · User Story 11）
- 「61. As a member, I want the app to preserve drafts while offline, so that interrupted mobile work is not lost.」（同上 · User Story 61）
- 「62. As a member, I want offline drafts submitted only after my confirmation, so that reconnect does not replay stale intent.」（同上 · User Story 62）
- 「Task Office owns Home/Task projections, durable submission identity, reconciliation, Snapshot/SSE recovery, intervention, decisions, budget and Task lifecycle.」（同上 · Implementation Decisions）
- 「Mobile core does not depend on React Native, DOM or concrete transport. Remote and native details are injected as Adapters.」（同上）
- 「Offline mode permits approved reads, drafts and annotations. It prohibits Run commands, approval, budget expansion and external Actions.」（同上）
- 「Tests target observable behavior at the highest stable Interface. Screen tests verify rendering and navigation; they do not duplicate Module internals.」（同上 · Testing Decisions）
- 「Task Office Interface tests cover durable request identity, lost acknowledgements, unknown reconciliation, Snapshot hydration, SSE gaps, cursor expiry, single-writer admission, intervention routing, decision CAS and three-dimensional state projection.」（同上）
- 「- start(goal)：持久化意图并返回 TaskStartReceipt；」（mobile-module-seams.md §5.2）
- 「- 网络前先持久化 request_id 与输入摘要；」「- unknown 不自动重发或换 request_id；」「- Scope Lease 失效后丢弃迟到结果；」（mobile-module-seams.md §5.3 不变量）
- 「现有 submission.ts、execution-cache.ts、session-list.ts、task-form.ts、execution-presentation.ts 和 compatibility.ts 应成为 Module 内部实现或纯策略，不再各自成为 Screen 可见 Interface。」（mobile-module-seams.md §5.4）
- 「移动 AI Office 将现有 WeKnora Session 呈现为 Task，并保持 `taskId = sessionId`，不新增第二个 Task 聚合身份。」（ADR-0004——`start(goal)` 先创建目标会话再 Start，sessionId 即 taskId）
- MX-006 冻结规则（`packages/domain/src/mobile/submission.ts:19-27`，已批准实现）：「一次用户意图一个 request_id；网络发送前必须先落盘（request_id+输入摘要+scope）」「ACK 丢失/杀进程后重启：同一 store 中恢复 entry，用 lookup 对账，绝不换 ID 重建任务」「相同 request_id 不同输入 → 本地冲突（零网络请求）」「lookup unknown/pending 不自动重新 POST、不新建预算预占」
- MX-015 冻结规则（`packages/domain/src/mobile/task-form.ts:4-12`，已批准实现）：「草稿优先：取消输入/未就绪提交/离线一律保留草稿文本（不丢字）」「附件未就绪（scanning/pending/failed）禁止提交——startCount=0、草稿保留，并给出原因；附件字段绝不擅加进 StartInput（7 字段冻结，附件经会话准备接口解析）」「校验：文本非空、预算非负安全整数」
- 安全约束（Mimosa）：本计划不改服务端请求代码与 SQL（无新查询）；凭据只从环境变量读取，源码与测试不写入可用凭据字面量；request_id 是意图关联键而非凭据/签名/token（与 `apps/miniprogram/src/core/intent.ts:25` 注释同一纪律）。
- 工作流约束：严格 RED→GREEN→REFACTOR（每个任务先写失败测试、实跑确认失败、最小实现、通过、提交）；实现不与已批准 Spec 冲突，冲突时升级而非静默重设计。

**Issue #36 验收标准原文（docs/plans/issue30-sweep/issues/issue-36.md）：**

1. 「相同意图不会重复创建 Task 或预算预占。」
2. 「附件未就绪、离线和输入冲突均保留草稿且零危险重放。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

验收标准 3 的本地可验证性说明（blocked-env 声明）：真实端到端（生产 JSON transport + 真实 WeKnora Deployment 的 `/api/v1/sessions`、`POST /api/v1/workbench/executions`、lookup、agents 目录 + Runtime 授权通道 + Task Office `start(goal)` 编排 + 持久提交 store）沿用 T01–T05 已合并的 opt-in 真实 HTTP 模式，需要「一个真实 WeKnora Deployment（HTTPS origin）+ 一个测试账号 + 该部署至少一个可用 Agent」（`WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD` 环境变量）。本地无此环境时 Task 7 的真实 HTTP 用例以 `t.skip` 跳过（**不得伪造通过**）。本地替代证据：Task Office Interface 级场景测试（Task 2，真实模块编排 + in-memory scenario Adapter，覆盖 AC1/AC2 全部三类保留场景与幂等/对账）+ domain MX-006 `resume` 语义测试（Task 1）+ Go 服务端幂等既有测试复跑（Task 7 Step 1，真实 sqlite 库）+ api-client wire 契约测试（Task 3，真实序列化字节）。凡具备环境的运行都自动产出端到端证据。

**与调查结论的差异记录（以代码现状为准）：**

1. 调查称 `apps/miniprogram/tests/assembly.test.mjs:149/182`（unknown 后同一 request_id 重提交 D5、未决期间重复点击不建第二 intent）「在本 worktree 因未安装依赖无法执行，本次未能复跑」。本计划作者在当前 worktree 实跑 `node --experimental-strip-types --test apps/miniprogram/tests/assembly.test.mjs`：**11/11 全部通过**（依赖已安装）。小程序端的 D5 与重复点击语义已有绿色证据，本计划不为小程序改代码；原生端由 Task 1/Task 2 落同语义测试（`resume` 的 unknown-才-重发 + 同 ID 幂等重入）。
2. 调查称「`packages/domain` 的 MX-006 持久提交协调器（submission.ts）与 MX-015 表单模型（task-form.ts）当前无任何消费者」——属实（`grep -rn "createSubmissionCoordinator\|createTaskForm" packages apps --include='*.ts' --include='*.tsx'` 除 domain 自身导出外零引用）。本计划将其接入 Task Office（module-seams §5.4 的既定方向），并补齐 `task-form.ts` 至今缺失的测试文件（Task 1 Step 5，表征测试）。
3. 调查称「原生统一新建入口不存在：apps/mobile 无 New 屏」——属实（`apps/mobile/src/screens/` 无 New 屏、`apps/mobile/src/app/` 无 new 路由）。本计划 Task 6 交付。
4. 调查称「附件走 chat 会话上传通道（features/chat/page.tsx:24），task-form.ts 草稿模型的 attachments/knowledgeIds 未接提交链」——属实。边界声明：本计划把 `attachments`/`knowledgeIds` 接入草稿模型与就绪裁决链（未就绪附件阻塞提交并保留草稿，AC2），但**不实现附件上传/扫描通道**——Resource Shelf 的 `prepare(TaskResourceDraft)`（module-seams §6.2）尚无对应已交付 API，附件/知识的会话准备落点属后续 Issue（#45 知识闭环、#40 离线确认等被本 Issue 阻塞的后续工作）；StartInput 七字段冻结不变，附件/知识绝不进入 Start body。
5. Go 服务端（`internal/modules/workbench/service/workbench/admission.go` 的持久幂等 + `internal/handler/session/workbench_start.go` 的 Start/Lookup handler + `internal/router/routes_workbench.go:133-135` 的路由）本计划**零代码改动**：作者已实跑 `go test ./internal/modules/workbench/service/workbench/ -run 'TestAdmissionTwentyConcurrentIdenticalRequestsCreateOneRun|TestBudgetEnsureRetryAfterUnknownResponseKeepsOneReservation|TestAdmissionPublishFailureIsRetryable' -count=1` → ok（2.5s），作为 AC1 的服务端证据基线，Task 7 Step 1 复跑。
6. 独立计划审查修订记录（本轮已修复，执行者按修订后语义实现）：(F1/F2) `inputDigest` 覆盖 `session_id`（`submission.ts:65-74`）——原实现「无条件先 `createSession` 再以新 sessionId 重入」会使全部重入用例撞 `SubmissionConflictError`；已改为「意图日志端口持久化 `requestId → { sessionId, goal, scope }`，重入复用原 session、绝不新建」，且「store 失败零后端调用」的口径收敛为「零 Start POST」（`createSession` 是无副作用前置网络调用，与小程序先例同序）。(F3) SecureStore 是异步 API，不能伪装 domain `SubmissionStore` 的同步 `save(entry): void` 契约（TS 允许 `Promise<void>` 赋 `void` 返回、typecheck 不报，但 coordinator 不 await 会变 fire-and-forget，破坏「save 失败不发送」）——耐久层改由 office 显式 `await` 的 `SubmissionIntentLog` 异步端口承载。(F4) `/new` 生命周期宿主的卸载 cleanup 原为 stale closure，已改为 effect 作用域持有 `createdController` 并配套 mount/unmount 回归测试。(F5) `intent.ts` 注释行号修正为 `:25`。(F6) Task 4 Step 2 命令只保留根目录可执行的一条。

## Review Focus

Spec 隐含但无任务测试覆盖、最可能咬到真实用户的五类输入/失效模式（每行后在所属任务落地测试）：

1. **同意图重复 POST（双击/重试风暴）**：用户在 ACK 丢失后连点提交，若每次都换 request_id 或每次都重发 POST，服务端会创建多个 Task 与多次预算预占。——Task 2 测试「a repeated start with the same request id never dispatches a second POST」+ Task 1 domain 测试「resume resubmits the same request id only after an explicit unknown lookup」。
2. **ACK 丢失后换新 request_id 重建**：恢复路径若悄悄生成新 ID，旧请求在服务端已 admitted 时会出现两个 Task（旧 run 无人监督继续烧预算）。——Task 2 测试「a lost acknowledgement reconciles onto the original request and run」。
3. **附件就绪竞态**：附件仍 scanning/pending 时提交被放行，会发出一个缺少附件上下文的 Task 且草稿被清空。——Task 2 测试「attachments that are not ready block submission with zero backend calls」+ Task 5 控制器测试（未就绪提交不清草稿）。
4. **离线误清草稿与后台自动重放**：断网时提交失败若清空草稿或后台自动重发，违反 User Story 61/62（reconnect does not replay stale intent）。——Task 5 控制器测试「a failed submit keeps the draft and the intent request id; retry re-enters with the SAME id (D5)」+ Task 2 测试「a repeated start with the same request id never dispatches a second POST while unresolved」。
5. **跨 scope 的持久意图泄漏**：App 重启后另一部署/租户的 New 屏读到别 scope 的意图记录并重试，会把意图绑定到错误的 tenant 请求上；跨 scope 的持久 entry 同理。——Task 1 domain 测试「resume rejects an entry persisted under a different scope with zero network」+ Task 2 测试「the same request id with a different input conflicts with zero network」（scope 不一致分支同码拒绝）+ Task 4 Adapter 测试「intent records survive a simulated restart and stay scope-tagged」。

## 任务结构与文件地图

| # | 任务 | 主要交付 |
|---|---|---|
| 1 | domain：主理 Agent 推荐 + `resume` 受控重入 | `lead-agent.ts` 纯函数、`submission.ts` 的 `resume`、`task-form.test.ts` 表征 |
| 2 | mobile-core：`TaskOffice.start(goal)` + `reconcilePending()` | 端口扩展、错误码、in-memory 场景、全场景 Interface 测试 |
| 3 | api-client：remote 的 `createSession`/`start`/`lookup` | wire 适配（复用既有 executions/sessions API） |
| 4 | apps/mobile：耐久意图日志 Adapter + request_id 工厂 | `adapters/intent-log.ts`（expo-secure-store，重启恢复）、`adapters/request-id.ts` |
| 5 | apps/mobile：草稿持久化通道 + New 控制器 | `new-task-drafts.ts`、`new-task-view.ts`（AC2 的保留语义） |
| 6 | apps/mobile：New 屏、路由与接线 | `NewTaskScreen`、`/new`、composition、Home 入口、app-smoke |
| 7 | apps/mobile：集成证据 + 服务端幂等复跑 | `task-start-integration-smoke.ts`（AC3）、Go 测试复跑 |

执行门控：无——前置 #33/#35 已全部合入当前 HEAD（亲眼核实）。Task 2 依赖 Task 1 的 `resume`/`recommendLeadAgent`；Task 3 依赖 Task 2 的端口类型；Task 5 依赖 Task 2；Task 6 依赖 Task 4/5；Task 7 依赖 Task 3/6。按序执行。

并行合并注意（本计划与同批次其余 6 个计划独立 worktree 后合并）：共享文件改动收敛为——`packages/domain/src/mobile/submission.ts` 与 `submission.test.ts`（只追加一个方法与四个测试，不动 `submit`/`reconcile` 语义）、`packages/mobile-core/src/task-office/task-office.ts`（只追加类型/两个方法/三个可选 port，不动既有方法）、`task-office-errors.ts`（union 追加两个码）、`in-memory-task-backend.ts`（handlers/calls 追加三个条目）、`packages/mobile-core/src/index.ts`（追加导出）、`packages/api-client/src/mobile/task-office.ts` 与 `task-office.test.ts`（追加三个方法与测试）、`apps/mobile/src/composition.ts`（三处：vault 单例化、intentLog/newRequestId 注入 `taskOfficeFor`、导出 `openScopedDraftStore`，位置在任务内逐处标注）、`apps/mobile/src/screens/HomeScreen.tsx`（仅加一个按钮）、`apps/mobile/src/app-smoke.test.tsx`（一处 deepEqual 数组更新 + 文件末尾追加测试块）。新增文件全部为本计划独有。

---

### Task 1: domain——主理 Agent 推荐纯函数与提交协调器 `resume` 受控重入

**Files:**
- Create: `packages/domain/src/mobile/lead-agent.ts`
- Test: `packages/domain/src/mobile/lead-agent.test.ts`
- Modify: `packages/domain/src/mobile/submission.ts:102-175`（`createSubmissionCoordinator` 返回对象内追加 `resume` 方法，不动 `submit`/`reconcile`/`retryEntry`）
- Test: `packages/domain/src/mobile/submission.test.ts`（文件末尾追加两个测试）
- Test: `packages/domain/src/mobile/task-form.test.ts`（新文件，表征测试）

**Interfaces:**
- Consumes: 既有 `AgentOption`（`packages/domain/src/mobile/agent-options.ts:14`，`capability.state ∈ 'supported' | 'unavailable' | 'forbidden'`、`kind ∈ 'general' | 'coding' | 'analysis' | 'custom'`）；既有 `createSubmissionCoordinator(store, transport)` / `SubmissionEntry` / `SubmitOutcome` / `SubmissionConflictError`（`submission.ts:102`/`:41`/`:92`/`:58`）；既有 `createTaskForm`/`evaluateSubmitReadiness`/`toStartInput`/`EMPTY_DRAFT`（`task-form.ts:76`/`:39`/`:52`/`:30`）。
- Produces: `recommendLeadAgent(agents: readonly AgentOption[]): LeadAgentRecommendation`（`LeadAgentRecommendation = { agent: AgentOption; basis: 'kind-general' | 'first-supported' } | { agent: undefined; reason: 'no_supported_agent' }`）；`createSubmissionCoordinator(...)` 返回对象新增方法 `resume(input: MobileStartInput, scope: SubmissionScope): Promise<SubmitOutcome>`——同一意图的任意重入：无 entry 等同 `submit`；有 entry 且摘要/scope 不一致抛 `SubmissionConflictError`（零网络）；`bound` 直接返回；`awaiting_*` 先 `transport.lookup`，仅当 lookup 明确 `unknown`（服务端无持久记录）才以同一 request_id 重新 `transport.start`，否则走 `reconcile` 结果。Task 2/Task 5 消费。

- [ ] **Step 1: 写失败测试（lead-agent）**

`packages/domain/src/mobile/lead-agent.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { recommendLeadAgent } from './lead-agent.ts';
import type { AgentOption } from './agent-options.ts';

function agent(id: string, overrides: Partial<AgentOption> = {}): AgentOption {
  return {
    id,
    name: `Agent ${id}`,
    summary: '',
    kind: 'general',
    capability: { state: 'supported', reason: '' },
    ...overrides,
  };
}

test('recommends the first supported general agent (universal New entry default lead)', () => {
  const coding = agent('a-coding', { kind: 'coding' });
  const general = agent('a-general', { kind: 'general' });
  const later = agent('a-later', { kind: 'general' });
  const recommendation = recommendLeadAgent([coding, general, later]);
  assert.deepEqual(recommendation, { agent: general, basis: 'kind-general' });
});

test('falls back to the first supported agent of any kind when no general agent is supported', () => {
  const analysis = agent('a-analysis', { kind: 'analysis' });
  const recommendation = recommendLeadAgent([agent('a-coding', { kind: 'coding' }), analysis]);
  assert.deepEqual(recommendation, { agent: analysis, basis: 'first-supported' });
});

test('unavailable and forbidden agents are never recommended; absence is explicit', () => {
  const unavailable = agent('a-1', { capability: { state: 'unavailable', reason: 'capability_not_reported' } });
  const forbidden = agent('a-2', { capability: { state: 'forbidden', reason: 'policy' } });
  assert.deepEqual(recommendLeadAgent([unavailable, forbidden]), { agent: undefined, reason: 'no_supported_agent' });
  assert.deepEqual(recommendLeadAgent([]), { agent: undefined, reason: 'no_supported_agent' });
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/domain/src/mobile/lead-agent.test.ts`
Expected: FAIL——`Cannot find module './lead-agent.ts'`（模块不存在）。

- [ ] **Step 3: 最小实现**

`packages/domain/src/mobile/lead-agent.ts`（新文件，完整内容）：

```ts
import type { AgentOption } from './agent-options.ts';

/**
 * 主理 Agent（Lead Agent）推荐（MX-016 目录上的纯策略，CONTEXT.md「主理 Agent」：
 * 系统可以推荐，用户也可以显式选择）。
 * 规则（不猜测）：
 * - 候选 = capability.state === 'supported'（unavailable/forbidden 绝不推荐）；
 * - 通用目标入口（User Story 9）的默认主理优先 kind='general' 的首个 supported；
 * - 无 supported general 时取任意首个 supported；
 * - 无任何 supported → 显式 reason，表单展示原因而不是静默 fallback。
 */
export type LeadAgentRecommendation =
  | { agent: AgentOption; basis: 'kind-general' | 'first-supported' }
  | { agent: undefined; reason: 'no_supported_agent' };

export function recommendLeadAgent(agents: readonly AgentOption[]): LeadAgentRecommendation {
  const supported = agents.filter((candidate) => candidate.capability.state === 'supported');
  if (supported.length === 0) return { agent: undefined, reason: 'no_supported_agent' };
  const general = supported.find((candidate) => candidate.kind === 'general');
  if (general) return { agent: general, basis: 'kind-general' };
  return { agent: supported[0]!, basis: 'first-supported' };
}
```

并在 `packages/domain/src/mobile/index.ts` 追加一行导出（与既有行并列，放在 `export * from './agent-options.ts';` 之后）：

```ts
export * from './lead-agent.ts';
```

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/domain/src/mobile/lead-agent.test.ts`
Expected: PASS（3/3）。

- [ ] **Step 5: 写失败测试（submission.resume + task-form 表征）**

`packages/domain/src/mobile/submission.test.ts` 文件末尾追加（保持既有 import 不变，追加到文件尾）：

```ts
test('resume resubmits the same request id only after an explicit unknown lookup', async () => {
  const starts: MobileStartInput[] = [];
  const lookups: string[] = [];
  let failFirstStart = true;
  const transport: SubmissionTransport = {
    start: async (input) => {
      starts.push(input);
      if (failFirstStart) {
        failFirstStart = false;
        throw new Error('request lost');
      }
      return { run_id: 'run-resumed', request_id: input.request_id, status: 'admitted' };
    },
    lookup: async (requestId) => { lookups.push(requestId); return { state: 'unknown' }; },
  };
  const store = createInMemorySubmissionStore();
  const coordinator = createSubmissionCoordinator(store, transport);
  // 首次提交：网络失败 → entry 落在 awaiting_reconciliation，同 ID 保留
  const first = await coordinator.submit(input(), scope);
  assert.equal(first.entry.phase, 'awaiting_reconciliation');
  assert.equal(starts.length, 1);
  // 同一意图重入：先 lookup；unknown → 同 ID 重发（不换 ID）
  const resumed = await coordinator.resume(input(), scope);
  assert.equal(resumed.entry.phase, 'bound');
  assert.equal(resumed.entry.run_id, 'run-resumed');
  assert.equal(resumed.dispatched, true);
  assert.equal(starts.length, 2, 'exactly one resubmission');
  assert.equal(starts[1]!.request_id, input().request_id, 'the SAME request id is reused');
  assert.deepEqual(lookups, [input().request_id]);
  // 再次重入（已 bound）：直接返回，零网络
  const again = await coordinator.resume(input(), scope);
  assert.equal(again.entry.run_id, 'run-resumed');
  assert.equal(again.dispatched, false);
  assert.equal(starts.length, 2);
  assert.equal(lookups.length, 1);
});

test('resume reconciles instead of resubmitting when the server knows the request', async () => {
  const starts: MobileStartInput[] = [];
  const transport: SubmissionTransport = {
    start: async (input) => { starts.push(input); throw new Error('request lost'); },
    lookup: async () => ({ state: 'admitted' as const, run_id: 'run-original' }),
  };
  const store = createInMemorySubmissionStore();
  const coordinator = createSubmissionCoordinator(store, transport);
  await coordinator.submit(input(), scope);
  const resumed = await coordinator.resume(input(), scope);
  assert.equal(resumed.entry.phase, 'bound');
  assert.equal(resumed.entry.run_id, 'run-original');
  assert.equal(resumed.dispatched, false, 'a server-known request is reconciled, never re-POSTed');
  assert.equal(starts.length, 1);
});

test('resume rejects an entry persisted under a different scope with zero network', async () => {
  let network = 0;
  const transport: SubmissionTransport = {
    start: async () => { network += 1; throw new Error('unused'); },
    lookup: async () => { network += 1; throw new Error('unused'); },
  };
  const store = createInMemorySubmissionStore();
  const coordinator = createSubmissionCoordinator(store, transport);
  await coordinator.submit(input(), scope);
  await assert.rejects(
    coordinator.resume(input(), { origin: 'https://other.example', tenantID: 't1', userID: 'u1' }),
    SubmissionConflictError,
  );
  assert.equal(network, 0);
});

test('resume with a changed input digest conflicts without network', async () => {
  let network = 0;
  const transport: SubmissionTransport = {
    start: async () => { network += 1; throw new Error('unused'); },
    lookup: async () => { network += 1; throw new Error('unused'); },
  };
  const store = createInMemorySubmissionStore();
  const coordinator = createSubmissionCoordinator(store, transport);
  await coordinator.submit(input(), scope);
  await assert.rejects(coordinator.resume(input({ text: '不同的目标' })), SubmissionConflictError);
  assert.equal(network, 0);
});
```

`packages/domain/src/mobile/task-form.test.ts`（新文件，完整内容；**表征测试**——`task-form.ts` 实现已在 HEAD 但从未有测试文件，本测试钉住 MX-015 冻结语义供 #36 提交链引用，预期直接 PASS，若失败说明实现与冻结规则冲突、必须升级而非改测试）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createTaskForm, EMPTY_DRAFT, evaluateSubmitReadiness, toStartInput, type TaskFormSubmitPorts } from './task-form.ts';

function ports(log: string[] = []): TaskFormSubmitPorts {
  return {
    submit: async (input) => { log.push(`start:${input.request_id}`); return { dispatched: true }; },
    saveDraft: async (draft) => { log.push(`save:${draft.text}|${draft.agentId ?? ''}`); },
  };
}

test('readiness blocks empty text, missing agent, invalid budget and not-ready attachments', () => {
  assert.deepEqual(evaluateSubmitReadiness(EMPTY_DRAFT), { ready: false, reason: 'text_required', blockingAttachments: [] });
  assert.equal(evaluateSubmitReadiness({ ...EMPTY_DRAFT, text: '整理周报' }).reason, 'agent_required');
  assert.equal(evaluateSubmitReadiness({ ...EMPTY_DRAFT, text: '整理周报', agentId: 'a-1', budgetUpper: -1 }).reason, 'budget_invalid');
  assert.equal(evaluateSubmitReadiness({ ...EMPTY_DRAFT, text: '整理周报', agentId: 'a-1', budgetUpper: 1.5 }).reason, 'budget_invalid');
  const blocked = evaluateSubmitReadiness({ ...EMPTY_DRAFT, text: '整理周报', agentId: 'a-1', attachments: [{ id: 'f-1', name: 'a.pdf', readiness: 'scanning' }] });
  assert.equal(blocked.reason, 'attachments_not_ready');
  assert.deepEqual(blocked.blockingAttachments.map((item) => item.id), ['f-1']);
});

test('attemptSubmit with a not-ready attachment keeps the draft and never touches the network', async () => {
  const log: string[] = [];
  const form = createTaskForm({ ...EMPTY_DRAFT, text: '整理周报', agentId: 'a-1', attachments: [{ id: 'f-1', name: 'a.pdf', readiness: 'pending' }] }, ports(log));
  const result = await form.attemptSubmit({ requestID: 'req-1', sessionId: 's-1', targetId: 'platform', workspaceRef: '' });
  assert.equal(result.submitted, false);
  assert.equal(result.readiness.reason, 'attachments_not_ready');
  assert.equal(log.filter((entry) => entry.startsWith('start:')).length, 0, 'zero submissions');
  assert.equal(form.draft.text, '整理周报', 'the draft survives');
});

test('toStartInput produces exactly the frozen seven fields; attachments and knowledge never enter the body', () => {
  const input = toStartInput(
    { text: '  整理周报  ', agentId: 'a-1', budgetUpper: 200, attachments: [{ id: 'f-1', name: 'a.pdf', readiness: 'ready' }], knowledgeIds: ['kb-1'] },
    { requestID: 'req-1', sessionId: 's-1', targetId: 'platform', workspaceRef: 'ws' },
  );
  assert.deepEqual(Object.keys(input).sort(), ['agent_id', 'budget_upper', 'request_id', 'session_id', 'target_id', 'text', 'workspace_ref']);
  assert.equal(input.text, '整理周报');
});

test('a successful dispatch clears the editable fields but keeps agent and budget; a failed one keeps everything', async () => {
  const log: string[] = [];
  const form = createTaskForm({ ...EMPTY_DRAFT, text: '整理周报', agentId: 'a-1', budgetUpper: 200 }, ports(log));
  await form.attemptSubmit({ requestID: 'req-1', sessionId: 's-1', targetId: 'platform', workspaceRef: '' });
  assert.equal(form.draft.text, '');
  assert.equal(form.draft.agentId, 'a-1');
  assert.equal(form.draft.budgetUpper, 200);
  const failing = createTaskForm({ ...EMPTY_DRAFT, text: '离线目标', agentId: 'a-1' }, { submit: async () => ({ dispatched: false }), saveDraft: async () => {} });
  await failing.attemptSubmit({ requestID: 'req-2', sessionId: 's-2', targetId: 'platform', workspaceRef: '' });
  assert.equal(failing.draft.text, '离线目标', 'an unresolved submission keeps the draft');
});

test('cancelKeepingDraft persists the draft explicitly', async () => {
  const log: string[] = [];
  const form = createTaskForm({ ...EMPTY_DRAFT, text: '草稿', agentId: null }, ports(log));
  await form.cancelKeepingDraft();
  assert.equal(log.some((entry) => entry === 'save:草稿|'), true);
});
```

- [ ] **Step 6: 运行确认失败（resume 用例）/ 通过（表征用例）**

Run: `npx tsx --test packages/domain/src/mobile/submission.test.ts packages/domain/src/mobile/task-form.test.ts`
Expected: submission.test.ts 新增 4 个用例 FAIL——`coordinator.resume is not a function`；task-form.test.ts 5 个用例 PASS（表征基线，若此处 FAIL 停止并升级：实现与 MX-015 冻结规则冲突）。

- [ ] **Step 7: 最小实现（resume）**

`packages/domain/src/mobile/submission.ts`——在 `createSubmissionCoordinator` 返回对象内、`reconcile` 方法定义之后追加（保持既有方法不动）：

```ts
    /**
     * 同一意图的受控重入（显式驱动，区别于「unknown 不自动重发」的自动语义）：
     * - 无持久 entry：等同 submit（新意图）；
     * - 有 entry 且 scope/摘要不一致：SubmissionConflictError（零网络）；
     * - bound：直接返回原 run（零网络）；
     * - awaiting_*：先 lookup 对账原请求；仅当服务端明确 unknown（无持久记录，
     *   即该 request_id 从未 CreatePending 成功、无 Task/预算预占）才以同一
     *   request_id 重发——绝不换 ID 重建任务。
     */
    async resume(input: MobileStartInput, scope: SubmissionScope): Promise<SubmitOutcome> {
      const digest = inputDigest(input);
      const existing = store.load(input.request_id);
      if (!existing) return this.submit(input, scope);
      if (!sameScope(existing.scope, scope)) {
        throw new SubmissionConflictError(input.request_id, existing.input_digest, `${digest} (scope mismatch)`);
      }
      if (existing.input_digest !== digest) {
        throw new SubmissionConflictError(input.request_id, existing.input_digest, digest);
      }
      if (existing.phase === 'bound' && existing.run_id) {
        return { entry: existing, dispatched: false };
      }
      const lookup = await transport.lookup(input.request_id);
      if (lookup.state !== 'unknown') {
        return { entry: await this.reconcile(input.request_id, scope), dispatched: false };
      }
      try {
        const ack = await transport.start(input);
        if (ack.request_id !== input.request_id) {
          const entry: SubmissionEntry = { request_id: input.request_id, input_digest: digest, scope, phase: 'awaiting_reconciliation', updated_at: new Date().toISOString() };
          store.save(entry);
          return { entry, dispatched: true };
        }
        const entry: SubmissionEntry = { request_id: input.request_id, input_digest: digest, scope, phase: 'bound', run_id: ack.run_id, updated_at: new Date().toISOString() };
        store.save(entry);
        return { entry, dispatched: true };
      } catch {
        const entry: SubmissionEntry = { request_id: input.request_id, input_digest: digest, scope, phase: 'awaiting_reconciliation', updated_at: new Date().toISOString() };
        store.save(entry);
        return { entry, dispatched: true };
      }
    },
```

- [ ] **Step 8: 运行确认通过**

Run: `npx tsx --test packages/domain/src/mobile/submission.test.ts packages/domain/src/mobile/task-form.test.ts packages/domain/src/mobile/lead-agent.test.ts packages/domain/src/mobile/index.test.ts`
Expected: PASS（submission 9、task-form 5、lead-agent 3、index 既有全绿）。

- [ ] **Step 9: Commit**

```bash
git add packages/domain/src/mobile/lead-agent.ts packages/domain/src/mobile/lead-agent.test.ts packages/domain/src/mobile/submission.ts packages/domain/src/mobile/submission.test.ts packages/domain/src/mobile/task-form.test.ts packages/domain/src/mobile/index.ts
git commit -m "feat(domain): lead-agent recommendation, MX-006 resume entry and MX-015 characterization tests (T06)"
```

---

### Task 2: mobile-core——`TaskOffice.start(goal)` 与 `reconcilePending()`

**Files:**
- Modify: `packages/mobile-core/src/task-office/task-office.ts`
- Modify: `packages/mobile-core/src/task-office/task-office-errors.ts:8-15`（`TaskOfficeErrorCode` union 追加两个码）
- Modify: `packages/mobile-core/src/task-office/in-memory-task-backend.ts`
- Test: `packages/mobile-core/src/task-office/task-office-start.test.ts`（新文件）
- Modify: `packages/mobile-core/src/index.ts`

**Interfaces:**
- Consumes: Task 1 的 `resume`/`recommendLeadAgent` 与 domain 的 `inputDigest`；既有 `createTaskOffice(ports: TaskOfficePorts): TaskOffice`、`TaskBackendPort`、`requireLease`/`callBackend` 模式（`task-office.ts:142-238`）；`leaseScopeOf`（`../runtime/scope-lease.ts:21`，返回 `LeaseScope { deploymentOrigin, userId, tenantId }`——包内私有，mobile-core 可直接 import）；domain 的 `createSubmissionCoordinator`/`createInMemorySubmissionStore`/`evaluateSubmitReadiness`/`toStartInput`/`inputDigest`/`SubmissionStore`/`SubmissionEntry`/`SubmissionScope`/`SubmissionTransport`/`SubmissionConflictError`/`TaskAttachmentRef`/`NewTaskDraft`（`@weknora/domain/mobile`，mobile-core 已依赖该包——`shelf/types.ts:1` 同模式）；`createScenarioTaskBackend(handlers)`（`in-memory-task-backend.ts:19`）。
- Produces:
  - `TaskBackendPort` 追加：`createSession(input: { title: string }): Promise<{ sessionId: string }>`、`start(input: TaskBackendStartInput): Promise<TaskBackendStartAck>`、`lookup(requestId: string): Promise<TaskBackendLookup>`。
  - `TaskBackendStartInput`（wire 七字段冻结：`{ request_id: string; session_id: string; agent_id: string; target_id: string; workspace_ref: string; text: string; budget_upper: number }`）、`TaskBackendStartAck`（`{ run_id: string; request_id: string; status: string }`）、`TaskBackendLookup`（`{ state: 'pending' | 'dispatching' | 'admitted' | 'rejected' | 'unknown'; run_id?: string; reason?: string }`）——与 api-client `StartExecutionInput`/`StartAck`/`RequestLookup` 结构逐字一致（Task 3 实现同构、`pnpm --filter @weknora/mobile typecheck` 证明可赋值）。
  - `TaskOfficeGoal`（`{ text: string; agentId: string; budgetUpper: number; knowledgeIds?: string[]; attachments?: TaskAttachmentRef[] }`——附件/知识只参与就绪裁决与草稿，绝不进入 Start body）。
  - `TaskStartReceipt`（`{ requestId: string; phase: 'awaiting_ack' | 'awaiting_reconciliation' | 'bound' | 'rejected'; runId?: string; dispatched: boolean }`）。
  - `SubmissionIntentRecord`（`{ requestId: string; sessionId: string; goal: TaskOfficeGoal; scope: SubmissionScope; persistedAt: string }`）与 `SubmissionIntentLog`（`{ save(record): Promise<void>; load(requestId): Promise<SubmissionIntentRecord | undefined>; listScope(scope): Promise<SubmissionIntentRecord[]>; remove?(requestId): Promise<void> }`）及 `createInMemoryIntentLog(): SubmissionIntentLog`——耐久意图日志端口（office `await`；Task 4 的 secure-store Adapter 实现它）。goal 一致性比较键 `goalKeyOf(goal)` 只覆盖影响意图的字段（text/agentId/budgetUpper/knowledgeIds 排序；attachments 的 readiness 是状态不是意图，不进比较键——附件从 scanning 变 ready 不构成「换意图」）。
  - `TaskOffice` 追加：`start(goal: TaskOfficeGoal, options?: { requestId?: string }): Promise<TaskStartReceipt>`、`reconcilePending(): Promise<TaskStartReceipt[]>`。
  - `TaskOfficePorts` 追加：`submissionStore?: SubmissionStore`（**进程内同步契约**——domain `save(entry): void`，缺省 office 内 in-memory；不得注入异步实现，见 Architecture 的两层边界声明）、`intentLog?: SubmissionIntentLog`（耐久意图日志；缺省 office 内 in-memory——持久化是组合根显式决策）、`newRequestId?: () => string`（缺省 `crypto.randomUUID()`；平台无实现时抛错，组合根必须注入）。
  - 错误码追加：`TASK_OFFICE_ATTACHMENTS_NOT_READY`（附件未就绪：零网络、草稿保留由调用方持久化）、`TASK_OFFICE_SUBMISSION_CONFLICT`（相同 request_id 不同输入/跨 scope/意图记录缺失：零网络）。

**start(goal, options) 的顺序语义（固定，测试按此口径）：**
1. lease 有效 + 就绪裁决（未就绪零网络抛错）。
2. `requestId = options.requestId ?? nextRequestId()`；`await intentLog.load(requestId)`。
3. 无意图记录（新意图）：`backend.createSession`（前置网络，无 Task 副作用）→ `await intentLog.save({ requestId, sessionId, goal, scope })`（**Start POST 前耐久落盘；失败上抛且零 Start 派发**）。
4. 有意图记录（重入/重启恢复）：goal（`goalKeyOf`）或 scope 与记录不一致 → `TASK_OFFICE_SUBMISSION_CONFLICT` 零网络；一致则**复用原 sessionId**（不 `createSession`），保证 `inputDigest` 与首次落盘 entry 一致（`inputDigest` 覆盖 `session_id`——`submission.ts:65-74`）。
5. `submissions.resume(input, scope)`（domain：无 entry → submit 的 save→start；有 entry → conflict 检查 → lookup → unknown 才同 ID 重发）。
6. lease 迟到撤销拒绝；`bound` 后 `intentLog.remove?.(requestId)` 清理。

- [ ] **Step 1: 写失败测试**

`packages/mobile-core/src/task-office/task-office-start.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createInMemorySubmissionStore, type SubmissionStore } from '@weknora/domain/mobile';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import { createScenarioTaskBackend } from './in-memory-task-backend.ts';
import { createInMemoryIntentLog, createTaskOffice, TaskOfficeError, type SubmissionIntentLog, type TaskBackendStartInput } from './task-office.ts';

function leased() {
  const revocable = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.test', userId: 'user-1', tenantId: 'tenant-1' });
  return { revocable, lease: revocable.asScopeLease() };
}

const goal = { text: '整理本周反馈并生成周报', agentId: 'agent-1', budgetUpper: 200 };
const scope = { origin: 'https://weknora.example.test', tenantID: 'tenant-1', userID: 'user-1' };

function startOffice(handlers: Parameters<typeof createScenarioTaskBackend>[0], options: { ids?: string[]; store?: SubmissionStore; log?: SubmissionIntentLog } = {}) {
  const backend = createScenarioTaskBackend(handlers);
  const { lease } = leased();
  const office = createTaskOffice({
    backend,
    lease: () => lease,
    ...(options.store === undefined ? {} : { submissionStore: options.store }),
    ...(options.log === undefined ? {} : { intentLog: options.log }),
    ...(options.ids === undefined ? {} : { newRequestId: (() => { const queue = [...options.ids!]; return () => queue.shift() ?? 'req-fallback'; })() }),
  });
  return { backend, office };
}

const startPosts = (backend: ReturnType<typeof createScenarioTaskBackend>): TaskBackendStartInput[] =>
  backend.calls.flatMap((call) => call.kind === 'start' ? [call.input] : []);

test('a failing in-process store prevents the start POST (the session may already exist)', async () => {
  const { backend, office } = startOffice({}, {
    store: { load: () => undefined, save: () => { throw new Error('disk full'); }, listScope: () => [] },
  });
  await assert.rejects(office.start(goal), /disk full/);
  assert.equal(startPosts(backend).length, 0, 'MX-006: store save 失败不得发送 Start');
  // createSession 允许发生在意图落盘之前：它是无 Task/预算副作用的前置网络调用，
  // 与小程序 AgentPage 的 sessions.create → startTask 先例同序（features/home/pages.tsx:26-28）
  assert.equal(backend.calls.filter((call) => call.kind === 'createSession').length, 1);
});

test('an intent log failure prevents the start POST after the session exists', async () => {
  const { backend, office } = startOffice({}, {
    log: {
      save: async () => { throw new Error('intent log disk full'); },
      load: async () => undefined,
      listScope: async () => [],
    },
  });
  await assert.rejects(office.start(goal), /intent log disk full/);
  assert.equal(startPosts(backend).length, 0, 'the durable intent record must precede the start POST');
  assert.equal(backend.calls.filter((call) => call.kind === 'createSession').length, 1);
});

test('a lost acknowledgement reconciles onto the original request and run (no second task)', async () => {
  const posts: string[] = [];
  const { office } = startOffice({
    start: async (input) => { posts.push(input.request_id); if (posts.length === 1) throw new Error('request lost'); return { run_id: 'run-original', request_id: input.request_id, status: 'queued' }; },
    lookup: async () => ({ state: 'admitted', run_id: 'run-original' }),
  }, { ids: ['req-1'] });
  const first = await office.start(goal);
  assert.equal(first.phase, 'awaiting_reconciliation');
  assert.equal(first.requestId, 'req-1');
  assert.equal(posts.length, 1);
  // 用户以同一 request_id 重入（重复点击）：复用原 session，对账命中原 run，零新 POST
  const second = await office.start(goal, { requestId: 'req-1' });
  assert.equal(second.phase, 'bound');
  assert.equal(second.runId, 'run-original');
  assert.equal(second.dispatched, false);
  assert.equal(posts.length, 1, 'the same intent never creates a second task');
});

test('a repeated start with the same request id never dispatches a second POST while unresolved', async () => {
  const posts: string[] = [];
  const { office } = startOffice({
    start: async (input) => { posts.push(input.request_id); throw new Error('offline'); },
    lookup: async () => ({ state: 'pending' }),
  }, { ids: ['req-1'] });
  const first = await office.start(goal);
  assert.equal(first.phase, 'awaiting_reconciliation');
  const second = await office.start(goal, { requestId: first.requestId });
  assert.equal(second.phase, 'awaiting_reconciliation');
  assert.equal(second.dispatched, false, 'pending on the server means: wait, never re-POST');
  assert.equal(posts.length, 1);
});

test('an unknown lookup is the only path that resubmits, and it reuses the same request id AND session', async () => {
  const posts: TaskBackendStartInput[] = [];
  let lookupState: 'unknown' | 'admitted' = 'admitted';
  const { office } = startOffice({
    start: async (input) => { posts.push(input); if (posts.length === 1) throw new Error('request lost'); return { run_id: 'run-9', request_id: input.request_id, status: 'queued' }; },
    lookup: async () => ({ state: lookupState, ...(lookupState === 'admitted' ? { run_id: 'run-9' } : {}) }),
  }, { ids: ['req-1'] });
  await office.start(goal);
  lookupState = 'unknown';
  const resumed = await office.start(goal, { requestId: 'req-1' });
  assert.equal(resumed.phase, 'bound');
  assert.equal(resumed.runId, 'run-9');
  assert.equal(resumed.dispatched, true);
  assert.equal(posts.length, 2);
  assert.equal(posts[1]!.request_id, posts[0]!.request_id, 'the SAME request id is reused (D5)');
  assert.equal(posts[1]!.session_id, posts[0]!.session_id, 'the SAME session is reused — a fresh session would change the digest and fake a new intent');
});

test('attachments that are not ready block submission with zero backend calls', async () => {
  const { backend, office } = startOffice({});
  await assert.rejects(
    office.start({ ...goal, attachments: [{ id: 'f-1', name: 'a.pdf', readiness: 'scanning' }] }),
    (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_ATTACHMENTS_NOT_READY',
  );
  assert.equal(backend.calls.length, 0, 'not-ready attachments must not even create a session');
});

test('invalid text, agent or budget surface as TASK_OFFICE_INVALID_INPUT', async () => {
  const { backend, office } = startOffice({});
  for (const bad of [{ ...goal, text: '   ' }, { ...goal, agentId: '' }, { ...goal, budgetUpper: -1 }, { ...goal, budgetUpper: 1.5 }]) {
    await assert.rejects(office.start(bad), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_INVALID_INPUT');
  }
  assert.equal(backend.calls.length, 0);
});

test('the same request id with a different input conflicts with zero network', async () => {
  let posts = 0;
  const { office } = startOffice({
    start: async (input) => { posts += 1; return { run_id: 'r-1', request_id: input.request_id, status: 'queued' }; },
  }, { ids: ['req-1'] });
  await office.start(goal);
  await assert.rejects(
    office.start({ ...goal, text: '另一个不同的目标' }, { requestId: 'req-1' }),
    (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SUBMISSION_CONFLICT',
  );
  assert.equal(posts, 1, 'a conflicting replay performs no second POST');
});

test('a revoked scope rejects start before any persistence or network', async () => {
  const { revocable, lease } = leased();
  let backendCalls = 0;
  const office = createTaskOffice({
    backend: createScenarioTaskBackend({
      createSession: async () => { backendCalls += 1; return { sessionId: 's-1' }; },
    }),
    lease: () => lease,
    newRequestId: () => 'req-1',
  });
  revocable.revoke();
  await assert.rejects(office.start(goal), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED');
  assert.equal(backendCalls, 0);
});

test('a scope revoked mid-start rejects the late receipt', async () => {
  const { revocable, lease } = leased();
  let releaseSession!: (value: { sessionId: string }) => void;
  const office = createTaskOffice({
    backend: createScenarioTaskBackend({ createSession: () => new Promise<{ sessionId: string }>((resolve) => { releaseSession = resolve; }) }),
    lease: () => lease,
    newRequestId: () => 'req-1',
  });
  const pending = office.start(goal);
  revocable.revoke();
  releaseSession({ sessionId: 's-1' });
  await assert.rejects(pending, (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED');
});

test('reconcilePending restores unresolved submissions from the durable intent log after a restart', async () => {
  const sharedLog = createInMemoryIntentLog();
  const { revocable, lease } = leased();
  await createTaskOffice({
    backend: createScenarioTaskBackend({ start: async () => { throw new Error('request lost'); } }),
    lease: () => lease,
    intentLog: sharedLog,
    newRequestId: () => 'req-restart',
  }).start(goal);
  // 模拟 App 重启：进程内 store 随内存丢失，耐久意图日志跨实例共享，新 office
  const second = createTaskOffice({
    backend: createScenarioTaskBackend({ lookup: async () => ({ state: 'admitted', run_id: 'run-recovered' }) }),
    lease: () => lease,
    intentLog: sharedLog,
    newRequestId: () => 'req-unused',
  });
  const receipts = await second.reconcilePending();
  assert.equal(receipts.length, 1);
  assert.equal(receipts[0]!.requestId, 'req-restart');
  assert.equal(receipts[0]!.phase, 'bound');
  assert.equal(receipts[0]!.runId, 'run-recovered');
  assert.equal(receipts[0]!.dispatched, false);
  assert.equal((await sharedLog.listScope(scope)).length, 0, 'bound 意图记录被清理，列表不无限增长');
  revocable.revoke();
});

test('after a restart, retrying with the same request id reuses the original session and resubmits only on unknown', async () => {
  const sharedLog = createInMemoryIntentLog();
  const { lease } = leased();
  await createTaskOffice({
    backend: createScenarioTaskBackend({ start: async () => { throw new Error('request lost'); } }),
    lease: () => lease,
    intentLog: sharedLog,
    newRequestId: () => 'req-1',
  }).start(goal);
  // 重启：新 office、新进程内 store；lookup 明确 unknown → 同 ID 同 session 重发
  const posts: TaskBackendStartInput[] = [];
  const secondBackend = createScenarioTaskBackend({
    lookup: async () => ({ state: 'unknown' }),
    start: async (input) => { posts.push(input); return { run_id: 'run-9', request_id: input.request_id, status: 'queued' }; },
  });
  const second = createTaskOffice({ backend: secondBackend, lease: () => lease, intentLog: sharedLog, newRequestId: () => 'req-unused' });
  const receipt = await second.start(goal, { requestId: 'req-1' });
  assert.equal(receipt.phase, 'bound');
  assert.equal(receipt.runId, 'run-9');
  assert.equal(secondBackend.calls.filter((call) => call.kind === 'createSession').length, 0, 'a retry NEVER creates a second session');
  assert.equal(posts.length, 1);
  assert.equal(posts[0]!.request_id, 'req-1');
  assert.equal(posts[0]!.session_id, 'session-scenario-1', 'the original session id is restored from the intent log');
  assert.equal((await sharedLog.listScope(scope)).length, 0, 'the intent record is cleaned up once bound');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/mobile-core/src/task-office/task-office-start.test.ts`
Expected: FAIL——`office.start is not a function`（以及 `TaskOfficeError` code 断言失败：`TASK_OFFICE_ATTACHMENTS_NOT_READY` 不在 union）。

- [ ] **Step 3: 最小实现**

1. `packages/mobile-core/src/task-office/task-office-errors.ts`——`TaskOfficeErrorCode` union 追加两行（放在 `'TASK_OFFICE_DETAIL_CLOSED'` 之后）：

```ts
  | 'TASK_OFFICE_ATTACHMENTS_NOT_READY'
  | 'TASK_OFFICE_SUBMISSION_CONFLICT'
```

2. `packages/mobile-core/src/task-office/task-office.ts`——四处修改：

(a) 文件头 import 区追加（与既有 import 并列）：

```ts
import {
  createInMemorySubmissionStore,
  createSubmissionCoordinator,
  evaluateSubmitReadiness,
  inputDigest,
  toStartInput,
  SubmissionConflictError,
  type NewTaskDraft,
  type SubmissionEntry,
  type SubmissionScope,
  type SubmissionStore,
  type SubmissionTransport,
  type TaskAttachmentRef,
} from '@weknora/domain/mobile';
import { leaseScopeOf } from '../runtime/scope-lease.ts';
```

(b) 类型区追加（`TaskBackendPort` 定义之前，`TaskBackendListInput` 之后）：

```ts
/** T06 Start wire 输入：七字段冻结（与 api-client StartExecutionInput / domain MobileStartInput 逐字一致）。 */
export interface TaskBackendStartInput {
  request_id: string;
  session_id: string;
  agent_id: string;
  target_id: string;
  workspace_ref: string;
  text: string;
  budget_upper: number;
}

export interface TaskBackendStartAck {
  run_id: string;
  request_id: string;
  status: string;
}

export interface TaskBackendLookup {
  state: 'pending' | 'dispatching' | 'admitted' | 'rejected' | 'unknown';
  run_id?: string;
  reason?: string;
}

/** 一次用户目标的完整形态：附件/知识只参与就绪裁决与草稿，绝不进入 Start body。 */
export interface TaskOfficeGoal {
  text: string;
  agentId: string;
  budgetUpper: number;
  knowledgeIds?: string[];
  attachments?: TaskAttachmentRef[];
}

/** module-seams §5.2：start(goal) 持久化意图并返回 TaskStartReceipt。 */
export interface TaskStartReceipt {
  requestId: string;
  phase: SubmissionEntry['phase'];
  runId?: string;
  dispatched: boolean;
}
```

(c) `TaskBackendPort` 追加三个方法（`restore(taskId: string): Promise<void>;` 之后）：

```ts
  /** T06：一个初始目标创建 Task 前的目标会话（taskId = sessionId，ADR-0004）。 */
  createSession(input: { title: string }): Promise<{ sessionId: string }>;
  /** T06：持久幂等 Start（POST /api/v1/workbench/executions；七字段 wire）。 */
  start(input: TaskBackendStartInput): Promise<TaskBackendStartAck>;
  /** T06：request 对账（GET /api/v1/workbench/executions/requests/:request_id）。 */
  lookup(requestId: string): Promise<TaskBackendLookup>;
```

`TaskOfficePorts` 追加三个可选端口（`store?: TaskProjectionStore;` 之后）：

```ts
  /** T06 进程内提交存储（domain SubmissionStore 的同步契约：save 失败必须同步抛出且不发送）；
   *  缺省 office 内 in-memory。不得注入异步实现——耐久层是下面的 intentLog。 */
  submissionStore?: SubmissionStore;
  /** T06 耐久意图日志：office 在 Start POST 前 await save；重入/重启恢复时 load/listScope
   *  复用原 sessionId 与 goal。缺省 office 内 in-memory（进程内）——持久化是组合根的显式决策。 */
  intentLog?: SubmissionIntentLog;
  /** T06：一次用户意图的新 request_id 生成器；缺省 crypto.randomUUID（平台无实现时必须显式注入）。 */
  newRequestId?: () => string;
```

`TaskOffice` 接口追加（`open(input: { taskId: string; runId: string }): TaskHandle;` 之后）：

```ts
  start(goal: TaskOfficeGoal, options?: { requestId?: string }): Promise<TaskStartReceipt>;
  reconcilePending(): Promise<TaskStartReceipt[]>;
```

(d) `createTaskOffice` 实现体内追加（`const defaultDetailStore = ...` 之后）：

```ts
  const submissionStore = ports.submissionStore ?? createInMemorySubmissionStore();
  const intentLog = ports.intentLog ?? createInMemoryIntentLog();
  const submissionTransport: SubmissionTransport = {
    start: (input) => ports.backend.start(input),
    lookup: async (requestId) => {
      try {
        return await ports.backend.lookup(requestId);
      } catch (error) {
        if (error instanceof TaskOfficeError) throw error;
        throw new TaskOfficeError('TASK_OFFICE_BACKEND', { cause: error });
      }
    },
  };
  const submissions = createSubmissionCoordinator(submissionStore, submissionTransport);
  const submissionScopeOf = (lease: ScopeLease): SubmissionScope => {
    const scope = leaseScopeOf(lease);
    if (!scope) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
    return { origin: scope.deploymentOrigin, tenantID: scope.tenantId, userID: scope.userId };
  };
  const receiptOf = (entry: SubmissionEntry, dispatched: boolean): TaskStartReceipt => ({
    requestId: entry.request_id,
    phase: entry.phase,
    ...(entry.run_id === undefined ? {} : { runId: entry.run_id }),
    dispatched,
  });
  const nextRequestId = (): string => {
    if (ports.newRequestId) return ports.newRequestId();
    if (typeof globalThis.crypto?.randomUUID === 'function') return globalThis.crypto.randomUUID();
    throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT', { cause: new Error('newRequestId port is required on platforms without crypto.randomUUID') });
  };
  const goalDraftOf = (goal: TaskOfficeGoal): NewTaskDraft => ({
    text: goal.text,
    agentId: goal.agentId,
    budgetUpper: goal.budgetUpper,
    attachments: goal.attachments ?? [],
    knowledgeIds: goal.knowledgeIds ?? [],
  });
  // 意图一致性比较键：覆盖影响意图的字段（text/agentId/budgetUpper/knowledgeIds 排序）。
  // attachments 的 readiness 是状态不是意图（scanning→ready 不构成「换意图」），不进比较键。
  const goalKeyOf = (goal: TaskOfficeGoal): string => JSON.stringify({
    agentId: goal.agentId,
    budgetUpper: goal.budgetUpper,
    knowledgeIds: [...(goal.knowledgeIds ?? [])].sort(),
    text: goal.text,
  });
```

返回对象追加（`open(...)` 之后）：

```ts
    async start(startGoal: TaskOfficeGoal, options: { requestId?: string } = {}): Promise<TaskStartReceipt> {
      const lease = requireLease();
      const scope = submissionScopeOf(lease);
      const draft = goalDraftOf(startGoal);
      const readiness = evaluateSubmitReadiness(draft);
      if (!readiness.ready) {
        if (readiness.reason === 'attachments_not_ready') throw new TaskOfficeError('TASK_OFFICE_ATTACHMENTS_NOT_READY');
        throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');
      }
      const requestId = options.requestId ?? nextRequestId();
      const record = await intentLog.load(requestId);
      let sessionId: string;
      if (record === undefined) {
        // 新意图：目标会话是前置网络调用（无 Task/预算副作用，与 miniprogram AgentPage 同序），
        // 随后在 Start POST 之前耐久落盘意图记录——intentLog 写失败（磁盘满等）上抛且零 Start 派发。
        const session = await callBackend(() => ports.backend.createSession({ title: startGoal.text.trim().slice(0, 60) }));
        if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
        sessionId = session.sessionId;
        await intentLog.save({ requestId, sessionId, goal: startGoal, scope, persistedAt: new Date().toISOString() });
      } else {
        // 重入/重启恢复：绝不新建 session——inputDigest 覆盖 session_id（submission.ts:65-74），
        // 换 session 会伪造成新意图并撞 digest 冲突。借旧 ID 发新意图零网络拒绝。
        if (record.scope.origin !== scope.origin || record.scope.tenantID !== scope.tenantID || record.scope.userID !== scope.userID) {
          throw new TaskOfficeError('TASK_OFFICE_SUBMISSION_CONFLICT', { cause: new Error(`intent ${requestId} belongs to a different scope`) });
        }
        if (goalKeyOf(record.goal) !== goalKeyOf(startGoal)) {
          throw new TaskOfficeError('TASK_OFFICE_SUBMISSION_CONFLICT', { cause: new Error(`intent ${requestId} was persisted with a different goal`) });
        }
        sessionId = record.sessionId;
      }
      const input = toStartInput(draft, { requestID: requestId, sessionId, targetId: 'platform', workspaceRef: '' });
      try {
        const outcome = await submissions.resume(input, scope);
        if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
        if (outcome.entry.phase === 'bound') await intentLog.remove?.(requestId);
        return receiptOf(outcome.entry, outcome.dispatched);
      } catch (error) {
        if (error instanceof TaskOfficeError) throw error;
        if (error instanceof SubmissionConflictError) throw new TaskOfficeError('TASK_OFFICE_SUBMISSION_CONFLICT', { cause: error });
        throw error;
      }
    },
    async reconcilePending(): Promise<TaskStartReceipt[]> {
      const lease = requireLease();
      const scope = submissionScopeOf(lease);
      const records = await intentLog.listScope(scope);
      const receipts: TaskStartReceipt[] = [];
      for (const record of records) {
        // 重启恢复：进程内 store 为空时按意图记录预建 awaiting_reconciliation entry
        // （digest 与原提交一致——同一 sessionId + 同一 goal），再走 lookup 对账。
        if (submissionStore.load(record.requestId) === undefined) {
          const input = toStartInput(goalDraftOf(record.goal), { requestID: record.requestId, sessionId: record.sessionId, targetId: 'platform', workspaceRef: '' });
          submissionStore.save({ request_id: record.requestId, input_digest: inputDigest(input), scope, phase: 'awaiting_reconciliation', updated_at: new Date().toISOString() });
        }
        const entry = await submissions.reconcile(record.requestId, scope);
        if (entry.phase === 'bound') await intentLog.remove?.(record.requestId);
        receipts.push(receiptOf(entry, false));
      }
      if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
      return receipts;
    },
```

（在 (b) 类型区 `TaskStartReceipt` 之后追加以下公共类型与模块级函数——`goalDraftOf`/`goalKeyOf` 是 `createTaskOffice` 内的私有函数，已包含在上面 (d) 的实现体内：）

```ts
/** T06 耐久意图记录：重启后用原 session 与原 goal 重建 digest 一致的 Start 输入。 */
export interface SubmissionIntentRecord {
  requestId: string;
  sessionId: string;
  goal: TaskOfficeGoal;
  scope: SubmissionScope;
  persistedAt: string;
}

/** 耐久意图日志（office await；RN SecureStore 等异步持久层的忠实契约）。 */
export interface SubmissionIntentLog {
  save(record: SubmissionIntentRecord): Promise<void>;
  load(requestId: string): Promise<SubmissionIntentRecord | undefined>;
  listScope(scope: SubmissionScope): Promise<SubmissionIntentRecord[]>;
  /** bound/rejected 后清理；不实现则记录留存（reconcile 幂等无害，仅列表增长）。 */
  remove?(requestId: string): Promise<void>;
}

export function createInMemoryIntentLog(): SubmissionIntentLog {
  const records = new Map<string, SubmissionIntentRecord>();
  const sameScope = (a: SubmissionScope, b: SubmissionScope): boolean => a.origin === b.origin && a.tenantID === b.tenantID && a.userID === b.userID;
  return {
    async save(record) { records.set(record.requestId, record); },
    async load(requestId) { return records.get(requestId); },
    async listScope(scope) { return [...records.values()].filter((record) => sameScope(record.scope, scope)); },
    async remove(requestId) { records.delete(requestId); },
  };
}
```

3. `packages/mobile-core/src/task-office/in-memory-task-backend.ts`——完整替换为（保留既有四方法语义，追加三方法；`calls` 数组类型扩展）：

```ts
import type { TaskBackendListInput, TaskBackendLookup, TaskBackendOverview, TaskBackendPage, TaskBackendPort, TaskBackendStartAck, TaskBackendStartInput } from './task-office.ts';

/** Scriptable scenario Adapter（module-seams §12：remote-owned 依赖的 in-memory 场景）。 */
export interface ScenarioTaskBackendHandlers {
  overview?: () => Promise<TaskBackendOverview>;
  list?: (input: TaskBackendListInput) => Promise<TaskBackendPage>;
  archive?: (taskId: string) => Promise<void>;
  restore?: (taskId: string) => Promise<void>;
  createSession?: (input: { title: string }) => Promise<{ sessionId: string }>;
  start?: (input: TaskBackendStartInput) => Promise<TaskBackendStartAck>;
  lookup?: (requestId: string) => Promise<TaskBackendLookup>;
}

export interface ScenarioTaskBackend extends TaskBackendPort {
  calls: Array<
    | { kind: 'overview' }
    | { kind: 'list'; input: TaskBackendListInput }
    | { kind: 'archive' | 'restore'; taskId: string }
    | { kind: 'createSession'; title: string }
    | { kind: 'start'; input: TaskBackendStartInput }
    | { kind: 'lookup'; requestId: string }
  >;
}

export function emptyOverview(asOf = '2026-09-23T00:00:00Z'): TaskBackendOverview {
  return { needsMe: [], running: [], recentlyCompleted: [], unreadNotifications: 0, asOf };
}

export function createScenarioTaskBackend(handlers: ScenarioTaskBackendHandlers = {}): ScenarioTaskBackend {
  const calls: ScenarioTaskBackend['calls'] = [];
  let sessionSeq = 0;
  let runSeq = 0;
  const admitted = new Map<string, TaskBackendStartAck>();
  const scenario: ScenarioTaskBackend = {
    calls,
    async overview() {
      calls.push({ kind: 'overview' });
      return handlers.overview ? handlers.overview() : emptyOverview();
    },
    async list(input) {
      calls.push({ kind: 'list', input });
      return handlers.list ? handlers.list(input) : { items: [] };
    },
    async archive(taskId) {
      calls.push({ kind: 'archive', taskId });
      await handlers.archive?.(taskId);
    },
    async restore(taskId) {
      calls.push({ kind: 'restore', taskId });
      await handlers.restore?.(taskId);
    },
    async createSession(input) {
      calls.push({ kind: 'createSession', title: input.title });
      return handlers.createSession ? handlers.createSession(input) : { sessionId: `session-scenario-${sessionSeq += 1}` };
    },
    async start(input) {
      calls.push({ kind: 'start', input });
      const ack = handlers.start ? await handlers.start(input) : { run_id: `run-scenario-${runSeq += 1}`, request_id: input.request_id, status: 'queued' };
      admitted.set(input.request_id, ack);
      return ack;
    },
    async lookup(requestId) {
      calls.push({ kind: 'lookup', requestId });
      if (handlers.lookup) return handlers.lookup(requestId);
      const ack = admitted.get(requestId);
      return ack ? { state: 'admitted', run_id: ack.run_id } : { state: 'unknown' };
    },
  };
  return scenario;
}
```

4. `packages/mobile-core/src/index.ts`——追加导出（既有 `export type { ... } from './task-office/task-office.ts'` 块的标识符列表中加入新类型；另加一个值导出行）：

```ts
export { createInMemoryIntentLog } from './task-office/task-office.ts';
export type {
  AttentionState, HomeView, InteractionCard, SubmissionIntentLog, SubmissionIntentRecord, TaskBackendListInput, TaskBackendLookup, TaskBackendOverview, TaskBackendPage,
  TaskBackendPort, TaskBackendRun, TaskBackendStartAck, TaskBackendStartInput, TaskCard, TaskListPage, TaskOffice, TaskOfficeErrorCode,
  TaskOfficeGoal, TaskOfficePorts, TaskOfficeQuery, TaskStartReceipt, TaskStatusFilter,
} from './task-office/task-office.ts';
```

（即：type 块新增 `SubmissionIntentLog`、`SubmissionIntentRecord`、`TaskBackendLookup`、`TaskBackendStartAck`、`TaskBackendStartInput`、`TaskOfficeGoal`、`TaskStartReceipt` 七个标识符，字母序插入；值导出 `createInMemoryIntentLog` 单独一行，放在既有 `export { createTaskOffice, TaskOfficeError } ...` 行之后。）

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test 'packages/mobile-core/src/task-office/*.test.ts' packages/mobile-core/src/runtime/mobile-runtime.test.ts`
Expected: PASS（新增 12 个用例 + 既有全部用例绿——`task-office.test.ts`、`task-detail.test.ts`、`in-memory-task-detail.test.ts`、`task-timeline.test.ts`、`mobile-runtime.test.ts` 不回归）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/task-office/task-office.ts packages/mobile-core/src/task-office/task-office-errors.ts packages/mobile-core/src/task-office/in-memory-task-backend.ts packages/mobile-core/src/task-office/task-office-start.test.ts packages/mobile-core/src/index.ts
git commit -m "feat(mobile-core): TaskOffice.start(goal) durable submission and reconcilePending (T06)"
```

---

### Task 3: api-client——`createTaskOfficeRemote` 的 `createSession`/`start`/`lookup`

**Files:**
- Modify: `packages/api-client/src/mobile/task-office.ts`
- Test: `packages/api-client/src/mobile/task-office.test.ts`（文件末尾追加测试块）

**Interfaces:**
- Consumes: 既有 `createExecutionsApi(request)` 的 `start(input: StartExecutionInput, signal?): Promise<StartAck>`（`executions.ts:285`，POST `/api/v1/workbench/executions`）与 `lookup(requestID, signal?): Promise<RequestLookup>`（`executions.ts:291`，GET `/api/v1/workbench/executions/requests/:id`）；既有 `createChatSessionsApi(request).create(input: { title?: string }): Promise<ChatSession>`（`../chat/sessions.ts:62`，POST `/api/v1/sessions`，`ChatSession.id`）；Task 2 的 `TaskBackendStartInput`/`TaskBackendStartAck`/`TaskBackendLookup`（结构同构，typecheck 证明）。
- Produces: `createTaskOfficeRemote(...)` 返回对象新增 `createSession`/`start`/`lookup` 三方法，使该 remote 同时满足 Task 2 扩展后的 `TaskBackendPort`（与 `detail`/`stream` 共存于同一对象——`backend: remote, detail: remote` 装配模式继续成立）。

- [ ] **Step 1: 写失败测试**

`packages/api-client/src/mobile/task-office.test.ts` 文件末尾追加（沿用该文件既有的 request-recorder 测试模式——先读文件确认 recorder 形态，再按下述内容对齐变量名；核心断言如下）：

```ts
test('task office remote creates the goal session, starts the durable run and reconciles by request id', async () => {
  const requests: Array<{ method: string; path: string; body?: unknown }> = [];
  const request = async (input: ClientRequest) => {
    requests.push({ method: input.method, path: input.path, body: input.body });
    if (input.method === 'POST' && input.path === '/api/v1/sessions') {
      return { success: true, data: { id: 'session-77', title: '整理本周反馈并生成周报', is_pinned: false } };
    }
    if (input.method === 'POST' && input.path === '/api/v1/workbench/executions') {
      return { success: true, data: { run_id: 'run-77', request_id: (input.body as { request_id: string }).request_id, status: 'queued' } };
    }
    if (input.method === 'GET' && input.path === '/api/v1/workbench/executions/requests/req-77') {
      return { success: true, data: { state: 'admitted', run_id: 'run-77' } };
    }
    throw new Error(`unexpected ${input.method} ${input.path}`);
  };
  const remote = createTaskOfficeRemote({ origin: 'https://weknora.example.test', request });

  const session = await remote.createSession({ title: '整理本周反馈并生成周报' });
  assert.equal(session.sessionId, 'session-77');

  const ack = await remote.start({ request_id: 'req-77', session_id: 'session-77', agent_id: 'agent-1', target_id: 'platform', workspace_ref: '', text: '整理本周反馈并生成周报', budget_upper: 200 });
  assert.deepEqual(ack, { run_id: 'run-77', request_id: 'req-77', status: 'queued' });

  const lookup = await remote.lookup('req-77');
  assert.deepEqual(lookup, { state: 'admitted', run_id: 'run-77' });

  assert.deepEqual(requests, [
    { method: 'POST', path: '/api/v1/sessions', body: { title: '整理本周反馈并生成周报' } },
    { method: 'POST', path: '/api/v1/workbench/executions', body: { request_id: 'req-77', session_id: 'session-77', agent_id: 'agent-1', target_id: 'platform', workspace_ref: '', text: '整理本周反馈并生成周报', budget_upper: 200 } },
    { method: 'GET', path: '/api/v1/workbench/executions/requests/req-77', body: undefined },
  ]);
});

test('task office remote start validates the frozen seven fields before any request', async () => {
  let calls = 0;
  const remote = createTaskOfficeRemote({ origin: 'https://weknora.example.test', request: async () => { calls += 1; return {}; } });
  await assert.rejects(
    remote.start({ request_id: '', session_id: 's', agent_id: 'a', target_id: 'platform', workspace_ref: '', text: 't', budget_upper: 1 }),
    /request_id/,
  );
  await assert.rejects(
    remote.start({ request_id: 'r', session_id: 's', agent_id: 'a', target_id: 'platform', workspace_ref: '', text: 't', budget_upper: -1 }),
    /budget_upper/,
  );
  assert.equal(calls, 0, 'validation must reject before any HTTP traffic');
});
```

（`ClientRequest` 若该测试文件尚未导入，则在文件头 import 区补 `import type { ClientRequest } from '../client.ts';`；`createTaskOfficeRemote` 已有导入。）

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/api-client/src/mobile/task-office.test.ts`
Expected: FAIL——`remote.createSession is not a function`。

- [ ] **Step 3: 最小实现**

`packages/api-client/src/mobile/task-office.ts` 两处修改：

(a) import 区追加（与 `createExecutionsApi` 同级）：

```ts
import { createChatSessionsApi } from '../chat/sessions.ts';
```

构造区（`const executionsApi = createExecutionsApi(request);` 之后）追加：

```ts
  const sessionsApi = createChatSessionsApi(request);
```

(b) 返回对象内追加（`async archive(taskId: string): Promise<void>` 之前，紧随 `list` 之后）：

```ts
    async createSession(input: { title: string }): Promise<{ sessionId: string }> {
      const session = await sessionsApi.create({ title: input.title });
      return { sessionId: session.id };
    },
    async start(input: StartExecutionInput): Promise<StartAck> {
      return executionsApi.start(input);
    },
    async lookup(requestId: string): Promise<RequestLookup> {
      return executionsApi.lookup(requestId);
    },
```

（`StartExecutionInput`/`StartAck`/`RequestLookup` 与 Task 2 的 `TaskBackendStartInput`/`TaskBackendStartAck`/`TaskBackendLookup` 字段逐字一致——结构可赋值由 Task 6 的 `pnpm --filter @weknora/mobile typecheck` 证明，api-client 不依赖 mobile-core。）

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/api-client/src/mobile/task-office.test.ts packages/api-client/src/mobile/executions.test.ts`
Expected: PASS（新增 2 个用例 + 既有全绿）。

- [ ] **Step 5: Commit**

```bash
git add packages/api-client/src/mobile/task-office.ts packages/api-client/src/mobile/task-office.test.ts
git commit -m "feat(api-client): task office remote goal session, durable start and lookup (T06)"
```

---

### Task 4: apps/mobile——耐久意图日志 Adapter 与 request_id 工厂

**Files:**
- Create: `apps/mobile/src/adapters/intent-log.ts`
- Test: `apps/mobile/src/adapters/intent-log.test.ts`
- Create: `apps/mobile/src/adapters/request-id.ts`

**Interfaces:**
- Consumes: Task 2 的 `SubmissionIntentLog`/`SubmissionIntentRecord`（`@weknora/mobile-core` 公共导出——异步端口，office 在 Start POST 前 `await save`）；domain `SubmissionScope`（`@weknora/domain/mobile`）；`SecureStorePort`（`apps/mobile/src/adapters/secure-store.ts:6-10`，`getItemAsync/setItemAsync/deleteItemAsync`）。
- Produces: `createSecureIntentLog(store: SecureStorePort): SubmissionIntentLog`（持久键 `weknora.mobile.intents.v1`，JSON 数组全量重写；损坏 JSON 读空表不炸，与 `deployment-registry.ts` 同策略；行级结构校验坏行跳过；`remove` 实现）；`createNativeSecureIntentLog(): SubmissionIntentLog`（`require('expo-secure-store')`，与 `createNativeSecurePendingOidcStore` 同模式——app-smoke 已 stub 该模块）。`createNativeRequestId(): () => string`（`adapters/request-id.ts`，UUID v4：`crypto.randomUUID` → `getRandomValues` v4 → `Math.random` 兜底；request_id 是意图关联键，非凭据/签名/token，与 `apps/miniprogram/src/core/intent.ts:25` 注释同一纪律）。Task 6 composition 消费。**本 Adapter 不实现 domain `SubmissionStore`**（同步契约 `save(entry): void` 只能由进程内实现满足——异步 save 伪装同步端口会变成 fire-and-forget，破坏「save 失败不发送」；见计划 Architecture 的两层边界声明）。

- [ ] **Step 1: 写失败测试**

`apps/mobile/src/adapters/intent-log.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createSecureIntentLog } from './intent-log.ts';
import type { SecureStorePort } from './secure-store.ts';

function memoryStore(): SecureStorePort & { dump(): string | null; corrupt(body: string): void } {
  let value: string | null = null;
  return {
    async getItemAsync() { return value; },
    async setItemAsync(_key: string, next: string) { value = next; },
    async deleteItemAsync() { value = null; },
    dump: () => value,
    corrupt: (body: string) => { value = body; },
  };
}

const scope = { origin: 'https://weknora.example.test', tenantID: 'tenant-1', userID: 'user-1' };
const goal = { text: '整理周报', agentId: 'a-1', budgetUpper: 200 };
const record = { requestId: 'req-1', sessionId: 'session-77', goal, scope, persistedAt: '2026-09-24T00:00:00Z' };

test('intent records survive a simulated restart and stay scope-tagged', async () => {
  const secure = memoryStore();
  await createSecureIntentLog(secure).save(record);
  // 模拟 App 重启：全新实例、同一持久键——load/listScope 直接读盘，无需 hydrate
  const second = createSecureIntentLog(secure);
  const loaded = await second.load('req-1');
  assert.deepEqual(loaded, record);
  assert.deepEqual((await second.listScope(scope)).map((row) => row.requestId), ['req-1']);
  assert.deepEqual(await second.listScope({ ...scope, tenantID: 'tenant-2' }), [], 'intent records never leak across scopes');
});

test('a missing record reads as undefined without failing', async () => {
  const log = createSecureIntentLog(memoryStore());
  assert.equal(await log.load('req-x'), undefined);
  assert.deepEqual(await log.listScope(scope), []);
});

test('corrupted persisted JSON reads as an empty log instead of crashing the New flow', async () => {
  const secure = memoryStore();
  secure.corrupt('{not json');
  const log = createSecureIntentLog(secure);
  assert.deepEqual(await log.listScope(scope), []);
  await log.save(record);
  assert.equal((await createSecureIntentLog(secure).load('req-1'))?.sessionId, 'session-77', 'a fresh write repairs the log');
});

test('remove deletes exactly one record', async () => {
  const secure = memoryStore();
  const log = createSecureIntentLog(secure);
  await log.save(record);
  await log.save({ ...record, requestId: 'req-2', sessionId: 'session-78' });
  await log.remove?.('req-1');
  assert.equal(await log.load('req-1'), undefined);
  assert.equal((await log.load('req-2'))?.sessionId, 'session-78');
});

test('malformed rows are skipped while healthy rows survive', async () => {
  const secure = memoryStore();
  secure.corrupt(JSON.stringify([record, 'garbage-string', { requestId: 42 }]));
  const log = createSecureIntentLog(secure);
  assert.deepEqual((await log.listScope(scope)).map((row) => row.requestId), ['req-1']);
});
```

- [ ] **Step 2: 运行确认失败**

Run: `cd apps/mobile && npx tsx --test src/adapters/intent-log.test.ts && cd ../..`
Expected: FAIL——`Cannot find module './intent-log.ts'`。

- [ ] **Step 3: 最小实现**

`apps/mobile/src/adapters/intent-log.ts`（新文件，完整内容）：

```ts
import type { SubmissionScope } from '@weknora/domain/mobile';
import type { SubmissionIntentLog, SubmissionIntentRecord } from '@weknora/mobile-core';
import type { SecureStorePort } from './secure-store.ts';

const INTENTS_KEY = 'weknora.mobile.intents.v1';

function sameScope(a: SubmissionScope, b: SubmissionScope): boolean {
  return a.origin === b.origin && a.tenantID === b.tenantID && a.userID === b.userID;
}

function isRecord(value: unknown): value is SubmissionIntentRecord {
  if (typeof value !== 'object' || value === null) return false;
  const row = value as Partial<SubmissionIntentRecord>;
  return typeof row.requestId === 'string' && row.requestId !== ''
    && typeof row.sessionId === 'string' && row.sessionId !== ''
    && typeof row.persistedAt === 'string'
    && typeof row.goal === 'object' && row.goal !== null
    && typeof row.scope === 'object' && row.scope !== null;
}

function parseRecords(raw: string | null): Map<string, SubmissionIntentRecord> {
  const map = new Map<string, SubmissionIntentRecord>();
  if (!raw) return map;
  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    return map; // 损坏 JSON 读空表（不炸创建流），下一次 save 覆写修复
  }
  if (!Array.isArray(value)) return map;
  for (const row of value) {
    if (isRecord(row)) map.set(row.requestId, row);
  }
  return map;
}

/**
 * 耐久意图日志（Task Office 的 SubmissionIntentLog 端口实现）：
 * office 在 Start POST 前 await save——SecureStore 是异步 API，这正是耐久层
 * 必须由 office 显式 await 的原因（进程内 domain SubmissionStore 是同步契约，
 * 只有 in-memory 实现能真正满足它；见 plan-t36 的两层持久化边界声明）。
 */
export function createSecureIntentLog(store: SecureStorePort): SubmissionIntentLog {
  const readAll = async (): Promise<Map<string, SubmissionIntentRecord>> => parseRecords(await store.getItemAsync(INTENTS_KEY));
  return {
    async save(record) {
      const records = await readAll();
      records.set(record.requestId, record);
      await store.setItemAsync(INTENTS_KEY, JSON.stringify([...records.values()]));
    },
    async load(requestId) {
      return (await readAll()).get(requestId);
    },
    async listScope(scope) {
      return [...(await readAll()).values()].filter((record) => sameScope(record.scope, scope));
    },
    async remove(requestId) {
      const records = await readAll();
      if (!records.delete(requestId)) return;
      await store.setItemAsync(INTENTS_KEY, JSON.stringify([...records.values()]));
    },
  };
}

/** Loads Expo SecureStore only in the native composition path. */
export function createNativeSecureIntentLog(): SubmissionIntentLog {
  return createSecureIntentLog(require('expo-secure-store') as SecureStorePort);
}
```

`apps/mobile/src/adapters/request-id.ts`（新文件，完整内容）：

```ts
/**
 * 一次用户意图的 request_id（持久幂等关联键，NOT a password, credential,
 * signature or auth token——与 miniprogram core/intent.ts:25 同一纪律）。
 * 不可碰撞是正确性要求（碰撞 = 两个意图被服务端当成同一个幂等键），因此
 * 优先使用平台 CSPRNG；Math.random 仅为最后兜底。
 */
export function createNativeRequestId(): () => string {
  return (): string => {
    if (typeof globalThis.crypto?.randomUUID === 'function') return globalThis.crypto.randomUUID();
    const bytes = new Uint8Array(16);
    if (typeof globalThis.crypto?.getRandomValues === 'function') {
      globalThis.crypto.getRandomValues(bytes);
    } else {
      for (let index = 0; index < bytes.length; index += 1) bytes[index] = Math.floor(Math.random() * 256);
    }
    bytes[6] = (bytes[6]! & 0x0f) | 0x40;
    bytes[8] = (bytes[8]! & 0x3f) | 0x80;
    const hex = [...bytes].map((byte) => byte.toString(16).padStart(2, '0')).join('');
    return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
  };
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd apps/mobile && npx tsx --test src/adapters/intent-log.test.ts && cd ../..`
Expected: PASS（5/5）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/adapters/intent-log.ts apps/mobile/src/adapters/intent-log.test.ts apps/mobile/src/adapters/request-id.ts
git commit -m "feat(mobile): durable intent log adapter and request id factory (T06)"
```

---

### Task 5: apps/mobile——Scoped Vault 草稿通道与 New 控制器（AC2 保留语义）

**Files:**
- Create: `apps/mobile/src/new-task-drafts.ts`
- Test: `apps/mobile/src/new-task-drafts.test.ts`
- Create: `apps/mobile/src/new-task-view.ts`
- Test: `apps/mobile/src/new-task-view.test.ts`

**Interfaces:**
- Consumes: Task 2 的 `TaskOffice.start(goal, options?)`/`reconcilePending()`/`TaskStartReceipt`/`TaskOfficeGoal`；Task 1 的 `recommendLeadAgent`/`LeadAgentRecommendation` 与 domain 的 `NewTaskDraft`/`EMPTY_DRAFT`/`evaluateSubmitReadiness`/`SubmitReadiness`/`TaskAttachmentRef`/`AgentOption`/`KnowledgeResource`（`@weknora/domain/mobile`，`KnowledgeResource = { id: string; title: string; scanStatus: ...; documentCount: number; updatedAt: string }`，`resource-presentation.ts:14`）；#32 的 `ScopedStore.drafts`（`put/get`，`packages/mobile-core/src/vault/scoped-vault.ts:17-23`——`createScopedNewTaskDrafts` 消费其结构子集）。
- Produces:
  - `NewTaskDraftPersistence`（`{ load(): Promise<NewTaskDraft | undefined>; save(draft: NewTaskDraft): Promise<void> }`）与 `createScopedNewTaskDrafts(store: { get(id: string): Promise<{ body: string } | undefined>; put(input: { id: string; body: string }): Promise<void> }): NewTaskDraftPersistence`（草稿 id `new-task`；损坏 body 读 `undefined`）。
  - `NewTaskViewState`/`NewTaskController` 与 `createNewTaskController(ports: NewTaskControllerPorts): NewTaskController`；`NewTaskControllerPorts = { office: Pick<TaskOffice, 'start' | 'reconcilePending'>; agents(): Promise<readonly AgentOption[]>; knowledge?(): Promise<readonly KnowledgeResource[]>; drafts?: NewTaskDraftPersistence; newRequestId(): string }`。控制器状态机：初始化并行加载草稿与 agent 目录并推荐默认主理（draft.agentId 为空时采纳推荐），同时加载可附加知识列表（缺省空）；每次 update 持久化草稿；`toggleKnowledge(id)` 增删 `draft.knowledgeIds` 并持久化；未就绪提交零 `office.start` 调用且草稿保留；`bound` 成功后才清空可编辑字段（保留 agentId/budgetUpper 作为下次默认）；未决（`awaiting_*`）提交保留草稿并记住 `requestId`，用户重试时以同一 `requestId` 重入（D5）；`dispose()` 防迟到发布。Task 6 的屏与路由消费。

- [ ] **Step 1: 写失败测试**

`apps/mobile/src/new-task-drafts.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createScopedNewTaskDrafts } from './new-task-drafts.ts';

function memoryDrafts() {
  const rows = new Map<string, string>();
  return {
    async get(id: string) { const body = rows.get(id); return body === undefined ? undefined : { body }; },
    async put(input: { id: string; body: string }) { rows.set(input.id, input.body); },
    dump: (id: string) => rows.get(id),
  };
}

test('drafts round-trip through the scoped store under the fixed id', async () => {
  const backing = memoryDrafts();
  const drafts = createScopedNewTaskDrafts(backing);
  await drafts.save({ text: '整理周报', agentId: 'a-1', budgetUpper: 200, attachments: [], knowledgeIds: [] });
  const loaded = await drafts.load();
  assert.deepEqual(loaded, { text: '整理周报', agentId: 'a-1', budgetUpper: 200, attachments: [], knowledgeIds: [] });
  assert.equal(backing.dump('new-task'), JSON.stringify(loaded));
  await drafts.save({ text: '', agentId: null, budgetUpper: 0, attachments: [], knowledgeIds: [] });
  assert.equal((await drafts.load())?.text, '');
});

test('a corrupted or missing draft body reads as undefined, never crashes the New flow', async () => {
  const backing = memoryDrafts();
  await backing.put({ id: 'new-task', body: '{not json' });
  const drafts = createScopedNewTaskDrafts(backing);
  assert.equal(await drafts.load(), undefined);
  assert.equal(await createScopedNewTaskDrafts(memoryDrafts()).load(), undefined);
});
```

`apps/mobile/src/new-task-view.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import type { AgentOption, KnowledgeResource } from '@weknora/domain/mobile';
import { recommendLeadAgent } from '@weknora/domain/mobile';
import type { TaskOffice, TaskStartReceipt } from '@weknora/mobile-core';
import { createNewTaskController, type NewTaskDraftPersistence } from './new-task-view.ts';

function agents(): AgentOption[] {
  return [
    { id: 'a-coding', name: '编码', summary: '', kind: 'coding', capability: { state: 'supported', reason: '' } },
    { id: 'a-general', name: '通用', summary: '', kind: 'general', capability: { state: 'supported', reason: '' } },
  ];
}

const knowledge: KnowledgeResource[] = [
  { id: 'kb-1', title: '团队知识库', scanStatus: 'indexed', documentCount: 3, updatedAt: '2026-09-24T00:00:00Z' },
];

function memoryDrafts(): NewTaskDraftPersistence & { saved: string[] } {
  const saved: string[] = [];
  let current: string | undefined;
  return {
    saved,
    async load() { return current === undefined ? undefined : JSON.parse(current); },
    async save(draft) { current = JSON.stringify(draft); saved.push(draft.text); },
  };
}

function officeDouble(startImpl?: (goal: unknown, options?: { requestId?: string }) => Promise<TaskStartReceipt>) {
  const calls: Array<{ goal: unknown; options?: { requestId?: string } }> = [];
  const double = {
    calls,
    async start(goal: never, options?: { requestId?: string }) {
      calls.push({ goal, options });
      return startImpl ? startImpl(goal, options) : { requestId: 'req-1', phase: 'bound', runId: 'run-1', dispatched: true };
    },
    async reconcilePending() { return []; },
  };
  return double as unknown as Pick<TaskOffice, 'start' | 'reconcilePending'> & { calls: typeof calls };
}

test('initialization adopts the recommended lead agent and surfaces the recommendation basis', async () => {
  const controller = createNewTaskController({ office: officeDouble(), agents: async () => agents(), drafts: memoryDrafts(), newRequestId: () => 'req-9' });
  await controller.whenInitialized();
  const state = controller.state();
  assert.equal(state.loading, false);
  assert.deepEqual(state.recommendation, recommendLeadAgent(agents()));
  assert.equal(state.draft.agentId, 'a-general', 'a draft without an agent adopts the recommended lead');
  controller.dispose();
});

test('a stored draft overrides the recommendation (explicit choice wins)', async () => {
  const drafts = memoryDrafts();
  await drafts.save({ text: '既有草稿', agentId: 'a-coding', budgetUpper: 50, attachments: [], knowledgeIds: [] });
  const controller = createNewTaskController({ office: officeDouble(), agents: async () => agents(), drafts, newRequestId: () => 'req-9' });
  await controller.whenInitialized();
  assert.equal(controller.state().draft.agentId, 'a-coding');
  assert.equal(controller.state().draft.text, '既有草稿');
  controller.dispose();
});

test('a not-ready submission (attachments scanning) never calls office.start and keeps the draft', async () => {
  const drafts = memoryDrafts();
  const office = officeDouble();
  const controller = createNewTaskController({ office, agents: async () => agents(), drafts, newRequestId: () => 'req-1' });
  await controller.whenInitialized();
  controller.update({ text: '整理周报' });
  controller.setAttachments([{ id: 'f-1', name: 'a.pdf', readiness: 'scanning' }]);
  const receipt = await controller.submit();
  assert.equal(receipt, undefined);
  assert.equal(office.calls.length, 0, 'zero submissions for a not-ready draft');
  assert.equal(controller.state().draft.text, '整理周报', 'the draft survives (AC2)');
  assert.equal(drafts.saved[drafts.saved.length - 1], '整理周报');
  assert.equal(controller.state().readiness.reason, 'attachments_not_ready');
  controller.dispose();
});

test('a bound receipt clears the editable fields but keeps agent and budget for the next task', async () => {
  const drafts = memoryDrafts();
  const controller = createNewTaskController({ office: officeDouble(), agents: async () => agents(), drafts, newRequestId: () => 'req-1' });
  await controller.whenInitialized();
  controller.update({ text: '整理周报', budgetUpper: 300 });
  const receipt = await controller.submit();
  assert.equal(receipt?.phase, 'bound');
  const state = controller.state();
  assert.equal(state.draft.text, '');
  assert.equal(state.draft.agentId, 'a-general');
  assert.equal(state.draft.budgetUpper, 300);
  assert.equal(drafts.saved[drafts.saved.length - 1], '', 'the cleared draft is persisted');
  controller.dispose();
});

test('a failed submit keeps the draft and the intent request id; retry re-enters with the SAME id (D5)', async () => {
  const drafts = memoryDrafts();
  const office = officeDouble(async () => ({ requestId: 'req-first', phase: 'awaiting_reconciliation', dispatched: true }));
  const controller = createNewTaskController({ office, agents: async () => agents(), drafts, newRequestId: () => 'req-first' });
  await controller.whenInitialized();
  controller.update({ text: '离线目标' });
  await controller.submit();
  assert.equal(controller.state().draft.text, '离线目标', 'an unresolved submission keeps the draft (AC2 offline)');
  assert.deepEqual(office.calls.map((call) => call.options), [{}]);
  await controller.submit();
  assert.equal(office.calls[1]!.options?.requestId, 'req-first', 'the retry re-enters with the same request id');
  assert.equal(office.calls.length, 2);
  controller.dispose();
});

test('an input conflict surfaces the typed error and keeps the draft (zero replay)', async () => {
  const drafts = memoryDrafts();
  const office = officeDouble(async () => { throw new Error('TASK_OFFICE_SUBMISSION_CONFLICT'); });
  const controller = createNewTaskController({ office, agents: async () => agents(), drafts, newRequestId: () => 'req-1' });
  await controller.whenInitialized();
  controller.update({ text: '冲突目标' });
  const receipt = await controller.submit();
  assert.equal(receipt, undefined);
  assert.equal(controller.state().error, 'TASK_OFFICE_SUBMISSION_CONFLICT');
  assert.equal(controller.state().draft.text, '冲突目标', 'the draft survives a conflict (AC2)');
  controller.dispose();
});

test('cancelKeepingDraft persists the draft explicitly', async () => {
  const drafts = memoryDrafts();
  const controller = createNewTaskController({ office: officeDouble(), agents: async () => agents(), drafts, newRequestId: () => 'req-1' });
  await controller.whenInitialized();
  controller.update({ text: '取消时的草稿' });
  await controller.cancelKeepingDraft();
  assert.equal(drafts.saved.includes('取消时的草稿'), true);
  controller.dispose();
});

test('attached knowledge references toggle into the draft and ride along to start', async () => {
  const office = officeDouble();
  const controller = createNewTaskController({ office, agents: async () => agents(), knowledge: async () => knowledge, drafts: memoryDrafts(), newRequestId: () => 'req-1' });
  await controller.whenInitialized();
  assert.deepEqual(controller.state().knowledge.map((item) => item.id), ['kb-1'], 'the attachable knowledge list is projected');
  controller.update({ text: '整理周报' });
  controller.toggleKnowledge('kb-1');
  assert.deepEqual(controller.state().draft.knowledgeIds, ['kb-1']);
  controller.toggleKnowledge('kb-1');
  assert.deepEqual(controller.state().draft.knowledgeIds, [], 'toggling again detaches it');
  controller.toggleKnowledge('kb-1');
  await controller.submit();
  assert.deepEqual((office.calls[0]!.goal as { knowledgeIds?: string[] }).knowledgeIds, ['kb-1'], 'attached knowledge rides along to start(goal) — never into the Start body');
  controller.dispose();
});

test('unresolved submissions from before the restart are surfaced on initialization', async () => {
  const office = officeDouble();
  const recoverable = { ...office, reconcilePending: async () => [{ requestId: 'req-old', phase: 'awaiting_reconciliation', dispatched: false }] };
  const controller = createNewTaskController({ office: recoverable as unknown as Pick<TaskOffice, 'start' | 'reconcilePending'>, agents: async () => agents(), drafts: memoryDrafts(), newRequestId: () => 'req-1' });
  await controller.whenInitialized();
  assert.deepEqual(controller.state().inFlight, { requestId: 'req-old', phase: 'awaiting_reconciliation', dispatched: false });
  controller.dispose();
});
```

- [ ] **Step 2: 运行确认失败**

Run: `cd apps/mobile && npx tsx --test src/new-task-drafts.test.ts src/new-task-view.test.ts && cd ../..`
Expected: FAIL——`Cannot find module './new-task-drafts.ts'` / `Cannot find module './new-task-view.ts'`。

- [ ] **Step 3: 最小实现**

`apps/mobile/src/new-task-drafts.ts`（新文件，完整内容）：

```ts
import type { NewTaskDraft } from '@weknora/domain/mobile';

export const NEW_TASK_DRAFT_ID = 'new-task';

/** Scoped Vault drafts 的结构子集（put/get 与 ScopedStore.drafts 签名一致，测试可注入内存替身）。 */
export interface ScopedDraftLike {
  get(id: string): Promise<{ body: string } | undefined>;
  put(input: { id: string; body: string }): Promise<void>;
}

export interface NewTaskDraftPersistence {
  load(): Promise<NewTaskDraft | undefined>;
  save(draft: NewTaskDraft): Promise<void>;
}

/** 加密离线草稿通道（#32 Scoped Vault 之上的 New 屏适配；跨进程持久化证据属 #40）。 */
export function createScopedNewTaskDrafts(store: ScopedDraftLike): NewTaskDraftPersistence {
  return {
    async load() {
      const entry = await store.get(NEW_TASK_DRAFT_ID).catch(() => undefined);
      if (!entry) return undefined;
      try {
        const value: unknown = JSON.parse(entry.body);
        if (typeof value !== 'object' || value === null) return undefined;
        return value as NewTaskDraft; // 字段级校验由 evaluateSubmitReadiness 在投影时兜底
      } catch {
        return undefined;
      }
    },
    async save(draft) {
      await store.put({ id: NEW_TASK_DRAFT_ID, body: JSON.stringify(draft) });
    },
  };
}
```

`apps/mobile/src/new-task-view.ts`（新文件，完整内容）：

```ts
import { EMPTY_DRAFT, evaluateSubmitReadiness, recommendLeadAgent, type AgentOption, type KnowledgeResource, type LeadAgentRecommendation, type NewTaskDraft, type SubmitReadiness, type TaskAttachmentRef } from '@weknora/domain/mobile';
import type { TaskOffice, TaskStartReceipt } from '@weknora/mobile-core';
import type { NewTaskDraftPersistence } from './new-task-drafts.ts';

export interface NewTaskControllerPorts {
  office: Pick<TaskOffice, 'start' | 'reconcilePending'>;
  agents(): Promise<readonly AgentOption[]>;
  /** 可附加知识（Resource Shelf browse 投影）；缺省空列表（附加资源是可选输入）。 */
  knowledge?(): Promise<readonly KnowledgeResource[]>;
  /** 加密离线草稿（Scoped Vault）；缺省时草稿仅存活于本控制器（组合根显式决定）。 */
  drafts?: NewTaskDraftPersistence;
  newRequestId(): string;
}

export interface NewTaskViewState {
  draft: NewTaskDraft;
  agents: readonly AgentOption[];
  knowledge: readonly KnowledgeResource[];
  recommendation: LeadAgentRecommendation;
  readiness: SubmitReadiness;
  loading: boolean;
  submitting: boolean;
  /** 上一次未决提交（本会话失败或重启恢复），驱动「同一意图同 request_id」重入。 */
  inFlight?: TaskStartReceipt;
  error?: string;
}

export interface NewTaskController {
  state(): NewTaskViewState;
  subscribe(listener: (state: NewTaskViewState) => void): () => void;
  update(patch: Partial<Omit<NewTaskDraft, 'attachments' | 'knowledgeIds'>>): void;
  setAttachments(attachments: TaskAttachmentRef[]): void;
  toggleKnowledge(knowledgeId: string): void;
  refreshAgents(): Promise<void>;
  submit(): Promise<TaskStartReceipt | undefined>;
  cancelKeepingDraft(): Promise<void>;
  whenInitialized(): Promise<void>;
  dispose(): void;
}

const emptyDraft = (): NewTaskDraft => ({ ...EMPTY_DRAFT, attachments: [], knowledgeIds: [] });

/**
 * New 屏控制器（module-seams §5.4：task-form 纯策略的宿主编排，Screen 只见状态与意图）。
 * AC2 的保留语义全部落在这里：未就绪/离线/冲突一律不清草稿；只有 bound 才清空可编辑
 * 字段；未决意图记住 requestId，用户显式重试时以同一 id 重入（D5：绝不换 ID 重建）。
 */
export function createNewTaskController(ports: NewTaskControllerPorts): NewTaskController {
  let state: NewTaskViewState = {
    draft: emptyDraft(),
    agents: [],
    knowledge: [],
    recommendation: { agent: undefined, reason: 'no_supported_agent' },
    readiness: evaluateSubmitReadiness(emptyDraft()),
    loading: true,
    submitting: false,
  };
  let intentRequestId: string | undefined;
  let disposed = false;
  const listeners = new Set<(state: NewTaskViewState) => void>();
  const publish = (next: NewTaskViewState): void => {
    state = next;
    if (!disposed) for (const listener of [...listeners]) listener(state);
  };
  const persistDraft = (): void => {
    void ports.drafts?.save(state.draft).catch(() => undefined);
  };
  const project = (patch: Partial<NewTaskViewState> & { draft?: NewTaskDraft }): void => {
    const draft = patch.draft ?? state.draft;
    publish({ ...state, ...patch, draft, readiness: evaluateSubmitReadiness(draft), recommendation: recommendLeadAgent(state.agents) });
  };
  const initialized = (async (): Promise<void> => {
    try {
      const [stored, agents, knowledge] = await Promise.all([
        ports.drafts?.load().catch(() => undefined),
        ports.agents(),
        ports.knowledge?.().catch(() => [] as const) ?? ([] as const),
      ]);
      let pending: TaskStartReceipt[] = [];
      try { pending = await ports.office.reconcilePending(); } catch { /* 恢复失败不阻塞新建流 */ }
      if (disposed) return;
      const draft = stored ?? emptyDraft();
      if (draft.agentId === null) draft.agentId = recommendLeadAgent(agents).agent?.id ?? null;
      const unresolved = pending.find((receipt) => receipt.phase !== 'bound' && receipt.phase !== 'rejected');
      publish({
        ...state,
        draft,
        agents,
        knowledge,
        loading: false,
        readiness: evaluateSubmitReadiness(draft),
        recommendation: recommendLeadAgent(agents),
        ...(unresolved === undefined ? {} : { inFlight: unresolved }),
      });
    } catch (cause) {
      if (disposed) return;
      publish({ ...state, loading: false, error: cause instanceof Error ? cause.message : String(cause) });
    }
  })();
  return {
    state: () => state,
    subscribe(listener) { listeners.add(listener); return () => { listeners.delete(listener); }; },
    update(patch) {
      project({ draft: { ...state.draft, ...patch } });
      persistDraft();
    },
    setAttachments(attachments) {
      project({ draft: { ...state.draft, attachments: [...attachments] } });
      persistDraft();
    },
    toggleKnowledge(knowledgeId) {
      const attached = state.draft.knowledgeIds.includes(knowledgeId);
      project({ draft: { ...state.draft, knowledgeIds: attached ? state.draft.knowledgeIds.filter((id) => id !== knowledgeId) : [...state.draft.knowledgeIds, knowledgeId] } });
      persistDraft();
    },
    async refreshAgents() {
      try {
        const agents = await ports.agents();
        if (!disposed) project({ agents });
      } catch {
        /* 目录刷新失败保持现状（recommendation 不回退） */
      }
    },
    async submit() {
      const readiness = evaluateSubmitReadiness(state.draft);
      if (!readiness.ready || state.submitting) {
        project({ readiness });
        persistDraft();
        return undefined;
      }
      publish({ ...state, submitting: true, error: undefined });
      try {
        const receipt = await ports.office.start(
          {
            text: state.draft.text,
            agentId: state.draft.agentId!,
            budgetUpper: state.draft.budgetUpper,
            ...(state.draft.knowledgeIds.length === 0 ? {} : { knowledgeIds: [...state.draft.knowledgeIds] }),
            ...(state.draft.attachments.length === 0 ? {} : { attachments: [...state.draft.attachments] }),
          },
          intentRequestId === undefined ? {} : { requestId: intentRequestId },
        );
        if (receipt.phase === 'bound' && receipt.runId !== undefined) {
          intentRequestId = undefined;
          const cleared: NewTaskDraft = { ...emptyDraft(), agentId: state.draft.agentId, budgetUpper: state.draft.budgetUpper };
          publish({ ...state, draft: cleared, readiness: evaluateSubmitReadiness(cleared), recommendation: state.recommendation, submitting: false, inFlight: undefined });
          await ports.drafts?.save(cleared).catch(() => undefined);
        } else {
          intentRequestId = receipt.requestId;
          publish({ ...state, submitting: false, inFlight: receipt });
        }
        return receipt;
      } catch (cause) {
        publish({ ...state, submitting: false, error: cause instanceof Error ? cause.message : String(cause) });
        return undefined;
      }
    },
    async cancelKeepingDraft() {
      persistDraft();
    },
    whenInitialized: () => initialized,
    dispose() {
      disposed = true;
      listeners.clear();
    },
  };
}
```

- [ ] **Step 4: 运行确认通过**

Run: `cd apps/mobile && npx tsx --test src/new-task-drafts.test.ts src/new-task-view.test.ts && cd ../..`
Expected: PASS（2 + 8 用例）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/new-task-drafts.ts apps/mobile/src/new-task-drafts.test.ts apps/mobile/src/new-task-view.ts apps/mobile/src/new-task-view.test.ts
git commit -m "feat(mobile): scoped vault draft channel and new task controller with durable intent reentry (T06)"
```

---

### Task 6: apps/mobile——NewTaskScreen、`/new` 路由与组合根接线

**Files:**
- Create: `apps/mobile/src/screens/NewTaskScreen.tsx`
- Create: `apps/mobile/src/app/new.tsx`
- Modify: `apps/mobile/src/composition.ts`（三处，位置逐处标注）
- Modify: `apps/mobile/src/screens/HomeScreen.tsx:46`（一个按钮）
- Test: `apps/mobile/src/app-smoke.test.tsx`（一处 deepEqual 更新 + 文件末尾追加测试块）

**Interfaces:**
- Consumes: Task 4 的 `createNativeSecureIntentLog` 与 `createNativeRequestId`；Task 5 的 `createNewTaskController`/`NewTaskViewState`/`createScopedNewTaskDrafts`（含 `knowledge?()` 端口与 `toggleKnowledge`）；既有 `activeTaskOffice()`（`composition.ts:139`）、`activeMobileRuntime()`（`composition.ts:182`）、`runtime.resourceShelf()`（`ResourcePage.agents`/`ResourcePage.knowledge`）、`createNativeScopedVaultIfAvailable()`（`composition.ts:35`）；`ScopedStore`（`@weknora/mobile-core`）。
- Produces: `/new` 路由（expo-router 文件路由，`Stack` 已由 `_layout.tsx` 提供）；`NewTaskScreen`（受控组件：`props: { state: NewTaskViewState; onUpdate(patch): void; onSetAttachments(attachments): void; onToggleKnowledge(knowledgeId): void; onSubmit(): void; onCancel(): void; onRefreshAgents(): void }`）；composition 新导出 `openScopedDraftStore(): Promise<ScopedStore | undefined>`（授权 scope 的加密 drafts；无 vault/无 lease 返回 undefined，fail soft——草稿持久化缺失不阻塞创建流）；HomeScreen 的 `/new` 一级入口。（意图日志经 `taskOfficeFor` 注入 office，New 层不直接接触——`reconcilePending`/重试路径由 office 内部读日志，无 hydrate 步骤。）

- [ ] **Step 1: 写失败测试**

`apps/mobile/src/app-smoke.test.tsx` 两处修改：

(a) 既有断言更新（测试 `the home header activates any listed tenant through the runtime callback` 内的 deepEqual 数组，原为 `['Acme', 'Beta', 'Sign out', 'View all tasks', 'Open Resources', 'Load home']`）：

```ts
  assert.deepEqual(buttons, ['Acme', 'Beta', 'Sign out', 'New task', 'View all tasks', 'Open Resources', 'Load home'], 'with more than one tenant every tenant is a header switch button');
```

（'New task' 插在 'View all tasks' 之前——按钮出现顺序与 HomeScreen 渲染顺序一致。）

(b) 文件末尾追加测试块（沿用文件既有的 `render`/`descendants` helper）：

```ts
test('the universal New entry renders the recommended lead agent, budget control and a blocked-submit reason', async () => {
  const { NewTaskScreen } = await import('./screens/NewTaskScreen.tsx');
  const state = {
    draft: { text: '整理周报', agentId: 'a-general', budgetUpper: 200, attachments: [{ id: 'f-1', name: 'a.pdf', readiness: 'scanning' }], knowledgeIds: ['kb-1'] },
    agents: [
      { id: 'a-general', name: '通用主理', summary: '', kind: 'general', capability: { state: 'supported', reason: '' } },
      { id: 'a-coding', name: '编码', summary: '', kind: 'coding', capability: { state: 'supported', reason: '' } },
    ],
    knowledge: [{ id: 'kb-1', title: '团队知识库', scanStatus: 'indexed', documentCount: 3, updatedAt: '2026-09-24T00:00:00Z' }],
    recommendation: { agent: { id: 'a-general', name: '通用主理', summary: '', kind: 'general', capability: { state: 'supported', reason: '' } }, basis: 'kind-general' },
    readiness: { ready: false, reason: 'attachments_not_ready', blockingAttachments: [{ id: 'f-1', name: 'a.pdf', readiness: 'scanning' }] },
    loading: false,
    submitting: false,
    inFlight: { requestId: 'req-old', phase: 'awaiting_reconciliation', dispatched: false },
  };
  const events: string[] = [];
  const element = render(NewTaskScreen, {
    state,
    onUpdate: () => { events.push('update'); },
    onSetAttachments: () => { events.push('attachments'); },
    onToggleKnowledge: () => { events.push('knowledge'); },
    onSubmit: () => { events.push('submit'); },
    onCancel: () => { events.push('cancel'); },
    onRefreshAgents: () => { events.push('refresh'); },
  });
  const text = descendants(element).filter(({ type }) => type === 'Text').flatMap(({ props }) => props.children).join(' ');
  assert.equal(text.includes('通用主理'), true, 'the recommended lead agent is visible');
  assert.equal(text.includes('attachments_not_ready'), true, 'a blocked submit surfaces its reason, never a fake success');
  assert.equal(text.includes('req-old'), true, 'an unresolved intent from before is surfaced');
  const buttons = descendants(element).filter(({ type }) => type === 'Button').map(({ props }) => props.title);
  assert.equal(buttons.some((title) => String(title).includes('✓ 团队知识库')), true, 'attached knowledge is visibly selected');
  assert.equal(buttons.includes('Submit task'), true);
  assert.equal(buttons.includes('Keep draft'), true);
});

test('the /new route reaches the Task Office through the composition root, never the wire directly', async () => {
  const { readFileSync } = await import('node:fs');
  const source = readFileSync(resolve(workspaceRoot, 'apps/mobile/src/app/new.tsx'), 'utf8');
  assert.equal(/activeTaskOffice\(\)/.test(source), true, 'the route must obtain the office via activeTaskOffice()');
  assert.equal(/workbench\/executions/.test(source), false, 'screens never call wire paths directly (module-seams §10)');
});

test('the home screen exposes the universal New entry', async () => {
  const { HomeScreen } = await import('./screens/HomeScreen.tsx');
  const element = render(HomeScreen, {
    deploymentLabel: 'Acme',
    tenants: [{ id: 'tenant-1', name: 'Acme' }],
    activeTenantId: 'tenant-1',
    onActivateTenant: () => {},
    onSignOut: async () => {},
    taskOffice: {
      home: async () => ({ needsMe: [], running: [], recentlyCompleted: [], unreadNotifications: 0, asOf: '' }),
      tasks: async () => ({ items: [], duplicateRunIds: [] }),
      moreTasks: async () => ({ items: [], duplicateRunIds: [] }),
      archive: async () => {},
      restore: async () => {},
      open: () => { throw new Error('unused'); },
      start: async () => { throw new Error('unused'); },
      reconcilePending: async () => [],
    },
  });
  const buttons = descendants(element).filter(({ type }) => type === 'Button').map(({ props }) => props.title);
  assert.equal(buttons.includes('New task'), true);
});

test('the /new lifecycle host disposes its controller on unmount, including after async creation', async () => {
  const route = await import('./app/new.tsx');
  const calls: string[] = [];
  const office = {
    start: async () => { calls.push('start'); throw new Error('unused'); },
    reconcilePending: async () => { calls.push('reconcilePending'); return []; },
  };
  // mount → 等待异步创建完成（init 会调用 office.reconcilePending，证明 controller 已创建）
  // → unmount → 再等一轮 microtask：卸载后不得再有任何 office 调用（cleanup 必须拿到
  // 已创建的 controller 并 dispose，而不是首帧 state 的 stale closure）。
  hooks().__beginRender();
  void route.NewTaskRouteLifecycle({ office: office as unknown as Parameters<typeof route.NewTaskRouteLifecycle>[0]['office'] });
  hooks().__mount();
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(calls.filter((entry) => entry === 'reconcilePending').length >= 1, true, 'initialization reconciled during the mounted period');
  hooks().__unmount();
  const afterUnmount = calls.length;
  await new Promise((resolve) => setImmediate(resolve));
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(calls.length, afterUnmount, 'no office calls may happen after unmount');
  // 第二轮 mount/unmount 验证循环稳定（重复挂载不残留）
  hooks().__beginRender();
  void route.NewTaskRouteLifecycle({ office: office as unknown as Parameters<typeof route.NewTaskRouteLifecycle>[0]['office'] });
  hooks().__mount();
  await new Promise((resolve) => setImmediate(resolve));
  hooks().__unmount();
  const afterSecond = calls.length;
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(calls.length, afterSecond);
});
```

（`taskOffice` 替身已补齐 Task 2 扩展后的 `start`/`reconcilePending` 两个方法；`open`/`start` 在该用例中不被调用。最后一个用例的行为级断言：controller 创建由挂载期的 `reconcilePending` 调用证明，dispose 的直接效果（迟到发布不触达 listener）在 `new-task-view.test.ts` 的 dispose 语义中验证；本用例回归「异步创建后 unmount 的 cleanup 路径」可执行且无泄漏调用。）

- [ ] **Step 2: 运行确认失败**

Run: `pnpm --filter @weknora/mobile test`
Expected: FAIL——四处：deepEqual 数组不等（HomeScreen 无 'New task'）；`Cannot find module './screens/NewTaskScreen.tsx'`；读取 `apps/mobile/src/app/new.tsx` ENOENT（源级断言用例）；`Cannot find module './app/new.tsx'`（lifecycle 用例）。

- [ ] **Step 3: 最小实现**

1. `apps/mobile/src/screens/NewTaskScreen.tsx`（新文件，完整内容）：

```tsx
import { Button, Text, TextInput, View } from 'react-native';
import type { NewTaskDraft, TaskAttachmentRef } from '@weknora/domain/mobile';
import type { NewTaskViewState } from '../new-task-view.ts';

export interface NewTaskScreenProps {
  state: NewTaskViewState;
  onUpdate(patch: Partial<Omit<NewTaskDraft, 'attachments' | 'knowledgeIds'>>): void;
  onSetAttachments(attachments: TaskAttachmentRef[]): void;
  onToggleKnowledge(knowledgeId: string): void;
  onSubmit(): void;
  onCancel(): void;
  onRefreshAgents(): void;
}

/** 统一 New 入口屏（受控组件）：目标输入 + 推荐/改选主理 Agent + 附加知识 + 预算 + 就绪裁决（module-seams §10：Screen 不见 wire）。 */
export function NewTaskScreen({ state, onUpdate, onSetAttachments, onToggleKnowledge, onSubmit, onCancel, onRefreshAgents }: NewTaskScreenProps) {
  return (
    <View>
      <Text>{state.loading ? 'Loading' : 'New task'}</Text>
      <Text>{state.recommendation.agent ? `Lead Agent: ${state.recommendation.agent.name}` : 'No supported lead agent'}</Text>
      {state.agents.map((agent) => (
        <Button
          key={agent.id}
          title={`${agent.name}${state.draft.agentId === agent.id ? ' ✓' : ''}`}
          onPress={() => { onUpdate({ agentId: agent.id }); }}
        />
      ))}
      <Button title="Refresh agents" onPress={() => { void onRefreshAgents(); }} />
      <TextInput value={state.draft.text} onChangeText={(text) => { onUpdate({ text }); }} placeholder="今天想完成什么？" multiline />
      <TextInput
        value={state.draft.budgetUpper === 0 ? '' : String(state.draft.budgetUpper)}
        onChangeText={(text) => { onUpdate({ budgetUpper: /^\d+$/.test(text) ? Number(text) : 0 }); }}
        placeholder="预算上限（Credits，可留空）"
        keyboardType="numeric"
      />
      {state.knowledge.map((item) => (
        <Button
          key={item.id}
          title={`${state.draft.knowledgeIds.includes(item.id) ? '✓ ' : ''}${item.title}`}
          onPress={() => { onToggleKnowledge(item.id); }}
        />
      ))}
      {state.draft.attachments.map((attachment) => (
        <Text key={attachment.id}>{`${attachment.name} · ${attachment.readiness}`}</Text>
      ))}
      {!state.readiness.ready && state.readiness.reason !== undefined && <Text>{state.readiness.reason}</Text>}
      {state.inFlight !== undefined && <Text>{`Unresolved submission ${state.inFlight.requestId} (${state.inFlight.phase}) — retrying keeps the same request id`}</Text>}
      {state.error !== undefined && <Text>{state.error}</Text>}
      <Button title="Submit task" disabled={state.submitting || !state.readiness.ready} onPress={() => { void onSubmit(); }} />
      <Button title="Keep draft" onPress={() => { void onCancel(); }} />
    </View>
  );
}
```

2. `apps/mobile/src/app/new.tsx`（新文件，完整内容）：

```tsx
import { useEffect, useState } from 'react';
import { Text, View } from 'react-native';
import type { AgentOption, KnowledgeResource } from '@weknora/domain/mobile';
import { activeMobileRuntime, activeTaskOffice, openScopedDraftStore } from '../composition.ts';
import { createNativeRequestId } from '../adapters/request-id.ts';
import { createScopedNewTaskDrafts } from '../new-task-drafts.ts';
import { createNewTaskController, type NewTaskController, type NewTaskViewState } from '../new-task-view.ts';
import { NewTaskScreen } from '../screens/NewTaskScreen.tsx';

/** /new 的挂载生命周期宿主：controller 与加密草稿在 effect 内创建，卸载时 dispose。 */
export function NewTaskRouteLifecycle({ office }: { office: NonNullable<ReturnType<typeof activeTaskOffice>> }) {
  const runtime = activeMobileRuntime();
  const [state, setState] = useState<NewTaskViewState | undefined>(undefined);
  const [controller, setController] = useState<NewTaskController | undefined>(undefined);
  useEffect(() => {
    let disposed = false;
    // cleanup 必须能拿到异步创建完成后的 controller：在 effect 作用域持有引用，
    // 而不是读取首帧渲染的 state（那是 undefined 的 stale closure）。
    let createdController: NewTaskController | undefined;
    void (async () => {
      const draftsStore = await openScopedDraftStore();
      const created = createNewTaskController({
        office,
        agents: async () => {
          const handle = runtime.resourceShelf();
          if (!handle) return [];
          return (await handle.browse()).agents as AgentOption[];
        },
        knowledge: async () => {
          const handle = runtime.resourceShelf();
          if (!handle) return [];
          return (await handle.browse()).knowledge as KnowledgeResource[];
        },
        ...(draftsStore === undefined ? {} : { drafts: createScopedNewTaskDrafts(draftsStore.drafts) }),
        newRequestId: createNativeRequestId(),
      });
      if (disposed) { created.dispose(); return; }
      createdController = created;
      setController(created);
      setState(created.state());
    })();
    return () => {
      disposed = true;
      createdController?.dispose();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- office/runtime 是 app 生命周期单例
  }, [office, runtime]);
  useEffect(() => (controller === undefined ? undefined : controller.subscribe(setState)), [controller]);
  if (controller === undefined || state === undefined) {
    return (
      <View>
        <Text>Loading</Text>
      </View>
    );
  }
  return (
    <NewTaskScreen
      state={state}
      onUpdate={(patch) => { controller.update(patch); }}
      onSetAttachments={(attachments) => { controller.setAttachments(attachments); }}
      onToggleKnowledge={(knowledgeId) => { controller.toggleKnowledge(knowledgeId); }}
      onSubmit={() => { void controller.submit(); }}
      onCancel={() => { void controller.cancelKeepingDraft(); }}
      onRefreshAgents={() => { void controller.refreshAgents(); }}
    />
  );
}

/** Expo Router 文件路由：/new（统一 New 入口）。只消费 Task Office 与 Resource Shelf Interface。 */
export default function NewTaskRoute() {
  const office = activeTaskOffice();
  if (!office) {
    return (
      <View>
        <Text>Sign in to create a task.</Text>
      </View>
    );
  }
  return <NewTaskRouteLifecycle office={office} />;
}
```

3. `apps/mobile/src/composition.ts` 三处修改：

(a) import 区追加（与 `createNativeSecureDeploymentRegistry` 同级）：

```ts
import type { ScopedStore } from '@weknora/mobile-core';
import { createNativeRequestId } from './adapters/request-id.ts';
import { createNativeSecureIntentLog } from './adapters/intent-log.ts';
```

(b) vault 单例化——把 `createNativeMobileRuntime` 内的 `scopedVault: createNativeScopedVaultIfAvailable(),` 改为引用模块级单例，并在模块级（`export const OIDC_REDIRECT_URI` 之前）新增：

```ts
/** App 生命周期单例：Runtime 撤销 scope 时 revoke 的就是这把 vault（#32）。 */
const nativeScopedVault = createNativeScopedVaultIfAvailable();
let nativeIntentLog: ReturnType<typeof createNativeSecureIntentLog> | undefined;
/** 惰性解析 expo-secure-store（与 pendingOidcStore 的函数体内 require 同模式；app-smoke 环境有 stub）。 */
const intentLogOf = (): ReturnType<typeof createNativeSecureIntentLog> => (nativeIntentLog ??= createNativeSecureIntentLog());

/** 授权 scope 的加密 drafts（New 屏离线草稿）；无 vault 或无授权 scope 返回 undefined（fail soft）。 */
export async function openScopedDraftStore(): Promise<ScopedStore | undefined> {
  const lease = runtime().scopeLease();
  if (!nativeScopedVault || !lease) return undefined;
  try {
    return await nativeScopedVault.open(lease);
  } catch {
    return undefined;
  }
}
```

（`createNativeMobileRuntime` 内 `scopedVault: createNativeScopedVaultIfAvailable(),` 同步改为 `scopedVault: nativeScopedVault,`。意图日志是异步端口（office 内部 `await`，load/listScope 直接读盘），无 hydrate 步骤。）

(c) `taskOfficeFor`（`composition.ts:109-124`）的 `createTaskOffice({...})` 增加两个参数（`store: createInMemoryTaskProjectionStore(),` 之后）：

```ts
      intentLog: intentLogOf(),
      newRequestId: createNativeRequestId(),
```

4. `apps/mobile/src/screens/HomeScreen.tsx`——在 `<Button title="View all tasks" onPress={() => router.push('/tasks')} />`（`:46`）之前插入一行：

```tsx
      <Button title="New task" onPress={() => router.push('/new')} />
```

- [ ] **Step 4: 运行确认通过**

Run: `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`
Expected: PASS（含既有 app-smoke 全部用例与更新后的三处断言；typecheck 证明 Task 3 remote 与 Task 2 `TaskBackendPort` 扩展结构逐字一致、`composition.ts` 新接线类型正确）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/screens/NewTaskScreen.tsx apps/mobile/src/app/new.tsx apps/mobile/src/composition.ts apps/mobile/src/screens/HomeScreen.tsx apps/mobile/src/app-smoke.test.tsx
git commit -m "feat(mobile): universal New entry screen, /new route and durable composition wiring (T06)"
```

---

### Task 7: apps/mobile——真实集成证据（AC3）与服务端幂等复跑

**Files:**
- Create: `apps/mobile/src/task-start-integration-smoke.ts`
- Test: `apps/mobile/src/task-start-integration-smoke.test.ts`

**Interfaces:**
- Consumes: Task 2/3 的完整链路（`createTaskOffice` + `createTaskOfficeRemote` 三新方法 + `MobileRuntime.authorizedRequest`）；既有 opt-in 模式 `taskOfficeIntegrationConfig`（`apps/mobile/src/task-office-integration-smoke.ts:22`——本计划自包含复刻同语义，不跨文件 import）；Task 4 的 `createNativeRequestId`；Go 侧 `TestAdmissionTwentyConcurrentIdenticalRequestsCreateOneRun`/`TestBudgetEnsureRetryAfterUnknownResponseKeepsOneReservation`/`TestAdmissionPublishFailureIsRetryable`（`internal/modules/workbench/service/workbench/admission_concurrency_test.go:69`/`:56`/`:121`——作者已实跑 ok）。
- Produces: `TaskStartIntegrationEvidence`（`{ deploymentOrigin: string; start: 'admitted' | 'pending' | 'rejected' | 'failed'; requestId?: string; runId?: string; repeatSubmitSameRequest: 'no-second-dispatch' | 'second-dispatch' | 'failed'; runVisibleInTasks: boolean | 'unavailable'; errorReason?: string; timestamp: string }`）与 `runTaskStartIntegration(config)` / `taskStartIntegrationConfig(env)` / `emitTaskStartIntegrationEvidence(evidence, emit)`——AC3 的真实端到端证据契约（真 transport、真 Runtime 授权、真 Task Office 编排、真服务端创建 Task）。

- [ ] **Step 1: 复跑服务端幂等基线（AC1 服务端证据，零代码改动）**

Run: `go test ./internal/modules/workbench/service/workbench/ -run 'TestAdmissionTwentyConcurrentIdenticalRequestsCreateOneRun|TestBudgetEnsureRetryAfterUnknownResponseKeepsOneReservation|TestAdmissionPublishFailureIsRetryable' -count=1`
Expected: PASS `ok github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench`（作者基线实跑：ok 2.5s。这三个测试钉住 `(tenant, actor, requestID)` 持久幂等：20 并发同 requestID 只建一个 run、unknown 后预算 Ensure 重试保留唯一预占、publish 失败可重试且不二次唤醒）。

- [ ] **Step 2: 写集成证据模块与 opt-in 测试**

`apps/mobile/src/task-start-integration-smoke.ts`（新文件，完整内容）：

```ts
import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createInMemoryCredentialStore, createMobileRuntime, createTaskOffice, type TaskOffice } from '@weknora/mobile-core';
import { createNativeRequestId } from './adapters/request-id.ts';

export type TaskStartIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface TaskStartIntegrationEvidence {
  deploymentOrigin: string;
  start: 'admitted' | 'pending' | 'rejected' | 'failed';
  requestId?: string;
  runId?: string;
  repeatSubmitSameRequest: 'no-second-dispatch' | 'second-dispatch' | 'failed';
  runVisibleInTasks: boolean | 'unavailable';
  errorReason?: string;
  timestamp: string;
}

/** 与 task-office-integration-smoke.ts 相同的 opt-in 语义（自包含，不跨计划 import）。 */
export function taskStartIntegrationConfig(env: Record<string, string | undefined>): TaskStartIntegrationConfig {
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

/**
 * 真实端到端（AC3）：生产 JSON transport + Runtime 授权通道 + 具体 Remote Adapter +
 * Task Office start(goal) 编排，在真实部署上创建一个真 Task，并验证同一 request_id
 * 重入不产生第二次派发。agent 目录经授权通道读 GET /api/v1/agents。
 */
export async function runTaskStartIntegration(config: Extract<TaskStartIntegrationConfig, { enabled: true }>): Promise<TaskStartIntegrationEvidence> {
  const evidence: TaskStartIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    start: 'failed',
    repeatSubmitSameRequest: 'failed',
    runVisibleInTasks: 'unavailable',
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
  const newRequestId = createNativeRequestId();
  // intentLog 用 office 缺省的进程内实现（同一 office 实例内的重入语义即可支撑本证据；
  // 跨进程耐久由 Task 4 的 secure Adapter 在组合根承载，真机重启证据属 #40）
  const office: TaskOffice = createTaskOffice({
    backend: createTaskOfficeRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) }),
    lease: () => runtime.scopeLease(),
    newRequestId,
  });
  const goal = { text: `T06 集成验证：${new Date().toISOString()}`, agentId, budgetUpper: 10 };
  const first = await office.start(goal);
  evidence.requestId = first.requestId;
  evidence.runId = first.runId;
  evidence.start = first.phase === 'bound' ? 'admitted' : first.phase === 'rejected' ? 'rejected' : 'pending';
  if (first.phase !== 'bound' || first.runId === undefined) {
    evidence.errorReason = `first receipt phase ${first.phase}`;
    return evidence;
  }
  // 同一意图重入（真服务端）：不得产生第二次派发
  const second = await office.start(goal, { requestId: first.requestId });
  evidence.repeatSubmitSameRequest = second.dispatched === false && second.runId === first.runId ? 'no-second-dispatch' : 'second-dispatch';
  // 创建的 run 可在任务列表观测（端到端闭环）
  try {
    const page = await office.tasks({});
    evidence.runVisibleInTasks = page.items.some((card) => card.runId === first.runId);
  } catch {
    evidence.runVisibleInTasks = 'unavailable';
  }
  return evidence;
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitTaskStartIntegrationEvidence(evidence: TaskStartIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
```

`apps/mobile/src/task-start-integration-smoke.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { emitTaskStartIntegrationEvidence, runTaskStartIntegration, taskStartIntegrationConfig } from './task-start-integration-smoke.ts';

const env = () => process.env as Record<string, string | undefined>;

test('integration stays opt-in: missing credentials skip, never fake a pass', () => {
  delete process.env.WEKNORA_MOBILE_TEST_DEPLOYMENT_URL;
  const config = taskStartIntegrationConfig(env());
  assert.equal(config.enabled, false);
  assert.equal(config.disposition, 'skip');
  const invalid = taskStartIntegrationConfig({ WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'http://insecure.example', WEKNORA_MOBILE_TEST_EMAIL: 'e', WEKNORA_MOBILE_TEST_PASSWORD: 'p' });
  assert.equal(invalid.enabled, false);
  assert.equal(invalid.disposition, 'invalid');
});

test('live end-to-end task creation through the highest stable interface (opt-in)', async (t) => {
  const config = taskStartIntegrationConfig(env());
  if (!config.enabled) {
    t.skip(config.reason);
    return;
  }
  const evidence = await runTaskStartIntegration(config);
  const emitted: string[] = [];
  emitTaskStartIntegrationEvidence(evidence, (record) => { emitted.push(record); });
  assert.equal(emitted.length, 1);
  assert.equal(JSON.parse(emitted[0]!).start, evidence.start);
  assert.equal(evidence.start, 'admitted', 'a live deployment with one agent must admit the goal');
  assert.equal(evidence.repeatSubmitSameRequest, 'no-second-dispatch', 'AC1: the same intent never dispatches twice');
  assert.notEqual(evidence.runVisibleInTasks, false, 'when the list is reachable, the created run must be observable in it');
});
```

- [ ] **Step 3: 运行确认（本地 skip + 语义用例通过）**

Run: `cd apps/mobile && npx tsx --test src/task-start-integration-smoke.test.ts && cd ../..`
Expected: PASS——2 个用例：第 1 个 PASS（opt-in/invalid 语义）；第 2 个 **skip**（本地无 `WEKNORA_MOBILE_TEST_*` 环境变量，`t.skip(reason)`——不伪造通过）。具备真实部署与账号时同一命令产出 live 证据。

- [ ] **Step 4: 全量收口（计划级验证）**

Run（worktree 根）：

```bash
npx tsx --test packages/domain/src/mobile/lead-agent.test.ts packages/domain/src/mobile/submission.test.ts packages/domain/src/mobile/task-form.test.ts packages/domain/src/mobile/index.test.ts && \
npx tsx --test 'packages/mobile-core/src/task-office/*.test.ts' packages/mobile-core/src/runtime/mobile-runtime.test.ts && \
npx tsx --test packages/api-client/src/mobile/task-office.test.ts packages/api-client/src/mobile/executions.test.ts && \
pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck && \
go test ./internal/modules/workbench/service/workbench/ -run 'TestAdmissionTwentyConcurrentIdenticalRequestsCreateOneRun|TestBudgetEnsureRetryAfterUnknownResponseKeepsOneReservation|TestAdmissionPublishFailureIsRetryable' -count=1 && \
cd apps/miniprogram && node --experimental-strip-types --test tests/assembly.test.mjs
```

Expected: 全绿（miniprogram assembly 11/11——差异记录第 1 条的既有 D5/重复点击证据保持绿色）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/task-start-integration-smoke.ts apps/mobile/src/task-start-integration-smoke.test.ts
git commit -m "test(mobile): opt-in real-transport task start integration evidence (T06 AC3)"
```

---

## 计划级验证命令（worktree 根执行）

```bash
npx tsx --test packages/domain/src/mobile/lead-agent.test.ts packages/domain/src/mobile/submission.test.ts packages/domain/src/mobile/task-form.test.ts packages/domain/src/mobile/index.test.ts && \
npx tsx --test 'packages/mobile-core/src/task-office/*.test.ts' packages/mobile-core/src/runtime/mobile-runtime.test.ts && \
npx tsx --test packages/api-client/src/mobile/task-office.test.ts packages/api-client/src/mobile/executions.test.ts && \
pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck && \
go test ./internal/modules/workbench/service/workbench/ -run 'TestAdmissionTwentyConcurrentIdenticalRequestsCreateOneRun|TestBudgetEnsureRetryAfterUnknownResponseKeepsOneReservation|TestAdmissionPublishFailureIsRetryable' -count=1 && \
cd apps/miniprogram && node --experimental-strip-types --test tests/assembly.test.mjs
```

（作者基线实跑：`npx tsx --test packages/domain/src/mobile/submission.test.ts` 5 pass、`npx tsx --test packages/mobile-core/src/task-office/task-office.test.ts` 8 pass、`pnpm --filter @weknora/mobile test` 72 pass、`pnpm --filter @weknora/mobile typecheck` exit 0、`go test ./internal/modules/workbench/service/workbench/...` ok 11.5s、`node --experimental-strip-types --test apps/miniprogram/tests/assembly.test.mjs` 11/11 pass。）

## Consumes / Produces 汇总（供后续计划 #37–#40/#45/#52/#56/#68 引用）

**Produces（本计划新增的对外接口）：**
- domain：`recommendLeadAgent(agents): LeadAgentRecommendation`（`packages/domain/src/mobile/lead-agent.ts`）；`createSubmissionCoordinator(...).resume(input, scope): Promise<SubmitOutcome>`（同 ID 受控重入：unknown 才重发）；MX-015 表征测试基线（`task-form.test.ts`）。
- mobile-core：`TaskOffice.start(goal: TaskOfficeGoal, options?: { requestId?: string }): Promise<TaskStartReceipt>`（顺序语义：意图日志先于 Start POST；重入复用原 sessionId，绝不新建 session）；`TaskOffice.reconcilePending(): Promise<TaskStartReceipt[]>`（从意图日志恢复并预建 entry 后对账）；`TaskBackendPort.createSession/start/lookup` 与 `TaskBackendStartInput`/`TaskBackendStartAck`/`TaskBackendLookup`；`TaskOfficePorts.submissionStore?`（进程内同步契约）`/intentLog?`（耐久意图日志，office await）`/newRequestId?`；`SubmissionIntentRecord`/`SubmissionIntentLog`/`createInMemoryIntentLog`（公共导出）；错误码 `TASK_OFFICE_ATTACHMENTS_NOT_READY`/`TASK_OFFICE_SUBMISSION_CONFLICT`；`ScenarioTaskBackendHandlers.createSession/start/lookup`。
- api-client：`createTaskOfficeRemote` 的 `createSession`/`start`/`lookup`（与 mobile-core 端口结构逐字一致）。
- apps/mobile：`/new` 路由与 `NewTaskScreen`；`createNewTaskController`（AC2 保留语义宿主，含 `knowledge?()`/`toggleKnowledge`）；`createScopedNewTaskDrafts`（Scoped Vault 加密草稿通道）；`createSecureIntentLog(store)`/`createNativeSecureIntentLog`（持久键 `weknora.mobile.intents.v1`，实现 `SubmissionIntentLog` 异步端口——不实现 domain 同步 `SubmissionStore`）；`createNativeRequestId`；composition 的 `openScopedDraftStore()` 与 `taskOfficeFor` 的 `intentLog/newRequestId` 注入；HomeScreen `/new` 一级入口；`task-start-integration-smoke.ts` 证据契约（opt-in `WEKNORA_MOBILE_TEST_*`）。

**明确不在本计划（后续 Issue）：** `act(TaskIntent)`（steer/queue-next/stop/decision——#37/#38）；Task 预算展示/扩额/恢复（#39）；跨进程加密持久化与联网确认提交队列（#40——本计划的意图日志已是设备持久（Start POST 前 await 写入）、草稿走 Scoped Vault 加密，但跨进程重启的真机持久化证据与「离线队列联网确认」属 #40）；附件上传/扫描通道与 `prepare(TaskResourceDraft)`（module-seams §6.2 的 Resource Shelf 第四入口——无对应已交付 API，附件在本计划只参与就绪裁决与草稿）；知识引用的真实检索闭环（#45）；语音转写草稿（#56）；小程序复用深 Module（#68——小程序现有两步流保持不动，其 D5/重复点击语义已有 assembly 测试绿色证据）。
