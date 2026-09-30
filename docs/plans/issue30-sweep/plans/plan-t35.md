# T05：Task 详情 Snapshot、Timeline 与 SSE 恢复（Issue #35）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 移动端可以打开一个 Task，看到结果优先的详情、Task/Run/Attention 三层状态与规范 Timeline，并通过「权威 Snapshot 水合 → 游标 SSE → 缺口/裁剪补洞 → 重同步」的 Task Office 深模块 Interface 完成 App 重启、断线与终态 drain 的可验证恢复。

**Architecture:** Go 侧在既有 run 作用域读路径上补齐任务层事实：`GET /workbench/executions/:run_id/snapshot` 响应新增可选 `task` 段（title/archived_at/attention，读时从 `sessions` LEFT JOIN 与 pending interaction 派生，谓词与列表行同规则），不新增第二套 Task 聚合读模型（ADR-0004：`taskId = sessionId`）。客户端在 `packages/mobile-core` 的 Task Office 上新增 `open({ taskId, runId })` → `TaskHandle`（`hydrate/view/updates/resync/close`）：模块内部拥有 Snapshot 水合、持久化投影合并（App 重启恢复）、串行事件提交（持久化失败不推游标）、重复 seq 幂等、缺口与游标裁剪的不静默中断 + 有界自动重同步、终态 drain 判定；Timeline 是从持久事件投影出的规范事实流（已知类型分类、未知类型不丢弃、原始证据按需展开）。`MobileRuntime` 新增 `authorizedEventStream` 授权 SSE 通道（token 不出 Runtime，pre-stream 401 refresh-once）；`api-client` 的 `createTaskOfficeRemote` 增加 `detail`/`stream`（v2 SSE 帧 → 模块事件，409 → `TASK_STREAM_CURSOR_EXPIRED`，control 帧透传）。apps/mobile 新增详情屏（结果优先 + 三层状态 + 折叠证据）与 `/tasks/detail` 路由。

**Tech Stack:** Go 1.26（gin + gorm，sqlite/postgres 双方言，`migrations/` 不动——本计划无新迁移）、TypeScript（`packages/contracts`、`packages/mobile-core`、`packages/api-client`、`apps/mobile` Expo RN）、node:test + tsx（TS 测试运行器，与 `task-office.test.ts` 一致）、`testify`（Go）。所有测试命令在 worktree 根（`.worktrees/issue30-sweep`）执行；前置：已执行过 `pnpm install`（本计划作者已实跑：`pnpm --filter @weknora/mobile test`、`pnpm --filter @weknora/mobile typecheck` 均绿）。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-35.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（User Stories 16/17/18/19/20、Implementation Decisions、Testing Decisions）
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§5 Task Office Module——所有权/Interface/不变量/seam、§4 Mobile Runtime、§10 App Shell、§13 Interface 测试面）
- ADR：`docs/adr/0004-task-is-session.md`（taskId = sessionId）、`docs/adr/0006-mobile-transport-by-semantics.md`（REST 取 Snapshot、游标 SSE 承载持久事件）、`docs/adr/0012-mobile-business-logic-lives-behind-deep-modules.md`
- 领域术语：`CONTEXT.md`（「任务生命周期（Task Lifecycle）」：进行中/已完成/已取消/已归档；「关注状态（Attention State）」；「任务时间线（Task Timeline）」：按权威顺序记录任务目标、成员输入、Agent 结论、工具活动、审批、运行状态、产物和外部操作结果的可恢复事实流；「运行（Run）」）
- Parent：Issue #30；Blocked by：#32（T02，已合并——`MobileRuntime.activateTenant`、scope-lease、`authorizedTransport` 均在当前 HEAD 核实）
- 前序批次产出（本计划 Consumes，全部在当前 HEAD 亲眼核实）：#34 的 `createTaskOffice`/`TaskBackendPort`/`TaskOfficeError`（`packages/mobile-core/src/task-office/task-office.ts`）、`MobileRuntime.authorizedRequest(input: RuntimeAuthorizedRequest): Promise<unknown>` 与 `MobileRuntimePorts.authorizedTransport?: (deploymentOrigin: string) => AuthorizedTransport`（`packages/mobile-core/src/runtime/mobile-runtime.ts:341`、`ports.ts:82`）、`createTaskOfficeRemote({ origin, request })`（`packages/api-client/src/mobile/task-office.ts`）、`taskOfficeFor(runtime, origin)` 工厂（`apps/mobile/src/composition.ts:91`）、`taskOfficeIntegrationConfig(env)` opt-in 模式（`apps/mobile/src/task-office-integration-smoke.ts`）、`createScenarioTaskBackend`（`packages/mobile-core/src/task-office/in-memory-task-backend.ts:19`）；#32 的 `RuntimeScopeLease`/`leaseActive`（`packages/mobile-core/src/runtime/scope-lease.ts:26`）。

## Global Constraints

以下为批准 Spec / ADR 的项目级约束，逐字引用，所有任务隐含遵守：

- 「Task Office owns Home/Task projections, durable submission identity, reconciliation, Snapshot/SSE recovery, intervention, decisions, budget and Task lifecycle.」（mobile-ai-office-design.md · Implementation Decisions）
- 「Task lifecycle, Run status and Attention status are separate. Agent-specific progress phases never replace these canonical dimensions.」（同上）
- 「REST submits commands and loads authoritative Snapshots. Cursored SSE carries durable Task/Run events. WebSocket or WebRTC is reserved for real-time voice. Push is a synchronization hint.」（同上）
- 「Cache identity includes Deployment, user, Tenant, Task and Run where applicable. Scope changes invalidate subscriptions and reject late responses.」（同上）
- 「Mobile core does not depend on React Native, DOM or concrete transport. Remote and native details are injected as Adapters.」（同上）
- 「Tests target observable behavior at the highest stable Interface. Screen tests verify rendering and navigation; they do not duplicate Module internals.」（同上 · Testing Decisions）
- 「Task Office Interface tests cover durable request identity, lost acknowledgements, unknown reconciliation, Snapshot hydration, SSE gaps, cursor expiry, single-writer admission, intervention routing, decision CAS and three-dimensional state projection.」（同上）
- 「Remote-owned dependencies use in-memory scenario Adapters for Module tests and real HTTP/SSE contract tests for production Adapters.」（同上）
- 「移动 AI Office 将现有 WeKnora Session 呈现为 Task，并保持 `taskId = sessionId`，不新增第二个 Task 聚合身份。」（ADR-0004）
- 「移动端使用 REST 提交命令和取得权威 Snapshot，使用带游标的 SSE 接收 Task 与 Run 的持久事件，使用 WebSocket 或 WebRTC 承载实时语音等双向低延迟媒体，并把 APNs/FCM 推送仅视为重新同步的提示。」（ADR-0006）
- 「Screen 不调用 start、lookup、snapshot、events、interaction、command 等多个 wire 方法。Module 内部决定顺序、幂等、重连、revision 和错误呈现。」（mobile-module-seams.md §5.2）
- 「禁止：Screen 直接导入 packages/contracts 或 packages/api-client；Screen 自己维护 request_id、cursor、revision、scope generation；每个 Screen 建独立 query cache 或 token refresh」（mobile-module-seams.md §10）
- 「Interface 不暴露 token、query key、generation number 或 SecureStore key。Scope Lease 是不透明、可撤销的能力对象，子 Module 每次异步提交前检查其有效性。」（mobile-module-seams.md §4.2）
- 「TaskHandle 只暴露：snapshot()：返回 TaskView；act(TaskIntent)：执行 steer、queue-next、stop、decision、share、archive 等受控意图；updates(listener)：订阅规范 TaskDelta；close()：释放订阅。」（mobile-module-seams.md §5.2）——本计划交付其中的 `hydrate/snapshot 视图`、`updates`、`close` 与恢复语义；`act(TaskIntent)` 属 #37/#38（运行干预与决定 CAS），`start(goal)` 属 #36，本计划不实现并在任务边界注明。
- 安全约束（会话注入）：服务端 SQL 一律参数绑定（本计划所有新查询均为 `?` 占位 + gorm 绑定，不拼接外部输入）；凭据只从环境变量读取，源码与测试不写入可用凭据字面量；真实 HTTP 集成证据沿用 `WEKNORA_MOBILE_TEST_*` opt-in 环境变量（无回退凭据）。
- 工作流约束：严格 RED→GREEN→REFACTOR（每个任务先写失败测试、实跑确认失败、最小实现、通过、提交）；实现不与已批准 Spec 冲突，冲突时升级而非静默重设计。

**Issue #35 验收标准原文（docs/plans/issue30-sweep/issues/issue-35.md）：**

1. 「重复事件幂等，缺口或游标裁剪不会被静默忽略。」
2. 「App 重启、断线和终态 drain 均有 Interface-level scenario 证据。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

验收标准 3 的本地可验证性说明（blocked-env 声明）：真实端到端（生产 JSON transport + 授权 SSE 通道 + 具体 Remote Adapter + Task Office 编排 + 真后端 snapshot/task 段/游标 SSE）沿用 T01–T04 已合并的 opt-in 真实 HTTP 模式，需要「一个真实 WeKnora Deployment（HTTPS origin）+ 一个测试账号」（`WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD` 环境变量）。本地无此环境时 Task 10 的真实 HTTP 用例以 `t.skip` 跳过（**不得伪造通过**）。本地替代证据：Task Office Interface 级场景测试（Task 6，真实模块编排 + in-memory scenario Adapter，覆盖 AC1/AC2 全部三类场景）+ Go 集成测试（真实 sqlite 迁移库，Task 1–2）+ contracts/api-client wire 契约测试（真实序列化字节与 SSE 帧，Task 3/8）。凡具备环境的运行都自动产出端到端证据。

**与调查结论的差异记录（以代码现状为准）：**

1. 调查称 `apps/miniprogram/tests/assembly.test.mjs:130`（watchExecution snapshot→watermark 追加流）「因未安装 workspace 依赖无法执行（ERR_MODULE_NOT_FOUND @weknora/api-client）」。本计划作者在当前 worktree 实跑 `node --experimental-strip-types --test tests/assembly.test.mjs`：依赖已安装、测试可执行，但 **2 个用例预存失败**（watchExecution 用例与 wire-paths 用例，错误 `incomplete: expected a boolean`，来自 `packages/contracts/src/mobile/execution.ts:184` 的 MX-003 冻结字段 `incomplete`/`confirmed_watermark` 未进入测试 fixture）。Task 11 修复 fixture 使该证据恢复绿色，并实跑为证。
2. 调查称「原生 App 的 Task 详情屏不存在；App 重启场景当前无代码与测试（旧 `apps/mobile/sources/weknora/executions/recovery.ts` 随旧树删除）」——属实（`find apps/mobile -name '*.tsx'` 无详情屏）。本计划以 mobile-core `TaskProjectionStore` + `TaskHandle` 重建该能力，落地在深模块而非 Screen。
3. 跨进程重启的持久化边界：composition 当前以模块内共享 in-memory store 承载 `TaskProjectionStore`（Interface 级重启场景证据成立）；接上 Scoped Vault 加密持久化属 #40（原生加密 seam），真机重启持久化证据属真机验收门槛——本计划不伪造该证据，在 Task 9 边界注明。

## Review Focus

Spec 隐含但无任务测试覆盖、最可能咬到真实用户的五类输入/失效模式（每行后在所属任务落地测试）：

1. **非业务帧污染业务游标**：SSE 心跳注释行（`: heartbeat`）与 `event: control` 控制帧若被当作业务事件解析，会推进游标或把控制语义写进时间线；控制帧必须单独分类且永不推进业务 seq。——Task 8 测试「the stream resumes from the cursor, routes control frames separately and never parses them as business events」+ Task 6 场景「cursor trimming (control frame) resyncs from a fresh snapshot」。
2. **外来 run 的事件与畸形 seq**：流里混入属于其他 run 的事件、seq 为 0/负数/非整数的事件，若静默入时间线会破坏「事件属于本 run 且 seq 严格递增」的不变量。——Task 6 测试「a foreign-run event or a malformed sequence interrupts the stream instead of entering the timeline」。
3. **损坏的持久缓存高于权威水位**：App 重启合并时，持久化投影中 seq 大于服务端 snapshot watermark 的行（本地缓存损坏/回滚）若被回放，会呈现服务端从未确认的"事实"。——Task 4 测试「mergeEventHistory dedupes by seq, prefers snapshot facts and clamps corrupted cache rows」。
4. **详情读路径的越权探测**：以他人 runId/taskId 打开详情必须与列表/归档同一 owner+tenant 谓词（`GetOwnedRun` + 事实读 `ErrNotFound`），不得因知道 ID 而获得访问。——Task 1 测试 `TestReadTaskFactsForRunDerivesThreeLayerFactsWithinOwnerScope`（跨 owner/跨租户/不存在一律 ErrNotFound）+ Task 2 测试 `TestGetWorkbenchSnapshotFactsReadIsOwnerScoped`。
5. **断线重连风暴**：服务端不可用时的自动重同步若无界，模块会打转并放大故障；必须有界停面并保留显式 `resync()` 恢复路径。——Task 6 测试「repeated stream failures stop automatic resync after the bound; explicit resync recovers」。

## 任务结构与文件地图

| # | 任务 | 主要交付 |
|---|---|---|
| 1 | Go：task 事实读 | `WorkbenchListStore.ReadTaskFactsForRun`（title/archived_at/attention，owner+tenant+run 谓词，全参数绑定） |
| 2 | Go：snapshot wire `task` 段 | `workbench.TaskSnapshotFacts` 类型、`ExecutionSnapshot.Task`、`GetWorkbenchSnapshot` 注入、容器装配 |
| 3 | contracts TS：`task` 段解析 | `SnapshotTaskFacts` + `parseExecutionSnapshot` 可选解析与校验 |
| 4 | mobile-core：纯投影 | `taskLifecycleOf`/`terminalRunStatusOf`/`mergeEventHistory`/`projectTimeline` |
| 5 | mobile-core：详情端口类型与 in-memory 场景 Adapter | `task-detail.ts` 类型区、`in-memory-task-detail.ts`（scripted stream + 投影 store + scenario backend） |
| 6 | mobile-core：`createTaskDetail` + `open()` + 全部恢复场景（AC1/AC2） | TaskHandle 完整实现与水合/幂等/缺口/裁剪/持久化失败/有界重连/重启/断线/终态 drain 场景测试、index exports |
| 7 | mobile-core Runtime：授权 SSE 通道 | `AuthorizedStreamTransport` port + `MobileRuntime.authorizedEventStream`（401 refresh-once、迟到 scope 拒绝） |
| 8 | api-client：detail/stream 适配 | `createTaskOfficeRemote` 新增 `detail`/`stream`（v2 帧→模块事件、409 映射、control 透传） |
| 9 | apps/mobile：详情屏与接线 | 原生 SSE adapter、composition 接线、`TaskDetailScreen` + 控制器 + `/tasks/detail` 路由 + 冒烟 |
| 10 | apps/mobile：真实 HTTP 集成证据 | `task-detail-integration-smoke.ts` + opt-in 集成测试（AC3） |
| 11 | miniprogram：SSE 证据修复 | `assembly.test.mjs` 两个预存 RED 用例的 fixture 补齐 MX-003 字段 |

执行门控：无——前置 #32/#34 已全部合入当前 HEAD（`git log` 核实：`7418e1b56`、`9ad9c2f98`、`028f72b11` 等）。Task 6 是单一 RED→GREEN 周期的大任务（全部场景测试先行、一次完整实现），评审作为一个单元把守；Task 8 依赖 Task 7 类型；Task 9 依赖 Task 5–8；Task 10 依赖 Task 9；其余按序执行。

并行合并注意（本计划与同批次其它计划独立 worktree 后合并）：共享文件改动收敛为——`apps/mobile/src/composition.ts`（三处：`authorizedStream` port 接线、`taskOfficeFor` 的 remote 增参、导出区追加 `activeTaskOffice` 与 `MobileTasks` 的 `onOpenTask` 透传，位置明确标注）、`apps/mobile/src/app-smoke.test.tsx`（两处：`NATIVE_MODULE_STUBS` 内 `expo-router`/`react-native` 两条既有条目各追加一个具名导出；文件末尾追加两个测试块）、`apps/mobile/src/screens/TasksScreen.tsx`（仅新增可选 `onOpenTask` prop 与行内按钮）、`apps/mobile/src/app/tasks.tsx`（仅注入 `onOpenTask`）、`packages/mobile-core/src/index.ts`（追加导出）、`packages/mobile-core/src/task-office/task-office.ts`（新增可选 port、`open` 方法与两个错误码，不动既有方法）、`packages/mobile-core/src/runtime/{ports,types,mobile-runtime}.ts` 与其测试（仅追加流通道，不动既有方法）、`packages/api-client/src/mobile/task-office.{ts,test.ts}`（追加方法与测试）、`packages/contracts/src/mobile/execution.ts` 与其测试（追加可选字段解析）、`internal/handler/session/workbench_read.go`（新增 port 字段/`WithTaskFacts`/`GetWorkbenchSnapshot` 改用 `resolveOwnedRun`，其余不动）、`internal/container/workbench.go`（提供者加一个参数）、`internal/modules/workbench/contracts.go`（追加类型与字段）。新增文件全部为本计划独有。

---

### Task 1: Go——`ReadTaskFactsForRun` 任务层事实读

**Files:**
- Create: `internal/application/repository/workbench_task_facts.go`
- Test: `internal/application/repository/workbench_task_facts_test.go`

**Interfaces:**
- Consumes: 既有 `WorkbenchListStore`（`internal/application/repository/workbench_list.go:135`）、`attentionPendingExpr`/`attentionOf`（同文件 `:49`/`:53`——waiting_user 或该 run 存在 pending interaction 派生 attention，与列表行同规则）、`agentruntime.ErrNotFound`、测试夹具 `openRunTestDB`/`seedWorkbenchListFixtures`/`insertWorkbenchRun`（`agent_run_test.go:29`、`workbench_list_test.go:15`/`:46`；`seedRunFixtures` 种下 tenant 1、用户 `u1`、会话 `s1`='session-1'、`s2`='session-2'，`agent_run_test.go:63`）。
- Produces: `WorkbenchTaskFacts{ TaskID string; Title string; Attention string; ArchivedAt string }`（Attention ∈ "none"|"required"；ArchivedAt 为空串表示未归档）与方法 `(*WorkbenchListStore).ReadTaskFactsForRun(ctx context.Context, tenantID uint64, ownerID, runID string) (WorkbenchTaskFacts, error)`——锚定一条已通过 owner 谓词的 run 行，LEFT JOIN sessions 取 title/archived_at；查无此 run 返回 `agentruntime.ErrNotFound`。Task 2 的 handler 注入消费。

- [ ] **Step 1: 写失败测试**

`internal/application/repository/workbench_task_facts_test.go`（新文件，完整内容）：

```go
package repository

import (
	"context"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/stretchr/testify/require"
)

// TestReadTaskFactsForRunDerivesThreeLayerFactsWithinOwnerScope：任务层事实
// （title/archived_at/attention）沿 owner+tenant+run 谓词读取；跨 owner、跨
// 租户与不存在的 run 一律 ErrNotFound，不泄露任何事实。
func TestReadTaskFactsForRunDerivesThreeLayerFactsWithinOwnerScope(t *testing.T) {
	db := openRunTestDB(t)
	seedWorkbenchListFixtures(t, db)
	base := time.Date(2026, 9, 23, 8, 0, 0, 0, time.UTC)
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u1", runID: "r-live", session: "s1", status: "running", agent: "agent-x", target: "platform", at: base})
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u1", runID: "r-wait", session: "s2", status: "waiting_user", agent: "agent-y", target: "platform", at: base.Add(time.Second)})
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u1", runID: "r-pend", session: "s1", status: "running", agent: "agent-z", target: "platform", at: base.Add(2 * time.Second)})
	archived := base.Add(time.Hour)
	require.NoError(t, db.Exec("UPDATE sessions SET archived_at = ? WHERE id = 's2'", archived).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO workbench_interactions (tenant_id, id, run_id, owner_id, kind, args_hash, status, expected_revision, created_at, updated_at)
		 VALUES (1, 'ix-t5', 'r-pend', 'u1', 'tool_approval', 'h-t5', 'pending', 1, ?, ?)`, archived, archived,
	).Error)
	store := NewWorkbenchListStore(db)
	ctx := context.Background()

	live, err := store.ReadTaskFactsForRun(ctx, 1, "u1", "r-live")
	require.NoError(t, err)
	require.Equal(t, "s1", live.TaskID)
	require.Equal(t, "session-1", live.Title)
	require.Equal(t, "none", live.Attention)
	require.Empty(t, live.ArchivedAt)

	waiting, err := store.ReadTaskFactsForRun(ctx, 1, "u1", "r-wait")
	require.NoError(t, err)
	require.Equal(t, "required", waiting.Attention, "waiting_user derives attention")
	require.Equal(t, archived.UTC().Format(time.RFC3339Nano), waiting.ArchivedAt)

	pending, err := store.ReadTaskFactsForRun(ctx, 1, "u1", "r-pend")
	require.NoError(t, err)
	require.Equal(t, "required", pending.Attention, "a pending interaction derives attention")

	_, err = store.ReadTaskFactsForRun(ctx, 1, "u2", "r-live")
	require.ErrorIs(t, err, agentruntime.ErrNotFound, "another owner in the same tenant sees nothing")
	_, err = store.ReadTaskFactsForRun(ctx, 2, "v1", "r-live")
	require.ErrorIs(t, err, agentruntime.ErrNotFound, "a tenant neighbour sees nothing")
	_, err = store.ReadTaskFactsForRun(ctx, 1, "u1", "r-none")
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/application/repository/ -run TestReadTaskFactsForRunDerivesThreeLayerFactsWithinOwnerScope -count=1`
Expected: FAIL——`store.ReadTaskFactsForRun undefined (type *repository.WorkbenchListStore has no field or method ReadTaskFactsForRun)`（编译失败即 RED）。

- [ ] **Step 3: 最小实现**

`internal/application/repository/workbench_task_facts.go`（新文件，完整内容）：

```go
package repository

import (
	"context"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"gorm.io/gorm"
)

// WorkbenchTaskFacts carries the task-level projection facts (T05 three-layer
// state) resolved for one run: title and archive lifecycle come from the
// session row; attention follows the exact rule used by list rows (a
// waiting_user run or a pending interaction on that run).
type WorkbenchTaskFacts struct {
	TaskID     string
	Title      string
	Attention  string // "none" | "required"
	ArchivedAt string // "" = not archived
}

type workbenchTaskFactsRow struct {
	SessionID     string
	Title         *string
	ArchivedAt    *time.Time
	RunStatus     string
	AttentionPend bool
}

func (workbenchTaskFactsRow) TableName() string { return "agent_runs" }

// ReadTaskFactsForRun resolves task facts anchored on one owned run. The run
// row is the anchor (the HTTP caller already proved ownership through
// GetOwnedRun); the session row is a LEFT JOIN so a drifted session cannot
// 404 an otherwise readable run — it only degrades title/archive facts to
// unknown. Tenant, owner and run id always arrive as bound parameters.
func (s *WorkbenchListStore) ReadTaskFactsForRun(ctx context.Context, tenantID uint64, ownerID, runID string) (WorkbenchTaskFacts, error) {
	if s == nil || s.db == nil || tenantID == 0 || ownerID == "" || runID == "" {
		return WorkbenchTaskFacts{}, agentruntime.ErrNotFound
	}
	var row workbenchTaskFactsRow
	err := s.db.WithContext(ctx).Table("agent_runs").
		Select("agent_runs.session_id AS session_id, agent_runs.status AS run_status, sessions.title AS title, sessions.archived_at AS archived_at, "+attentionPendingExpr+" AS attention_pending").
		Joins("LEFT JOIN sessions ON sessions.tenant_id = agent_runs.tenant_id AND sessions.id = agent_runs.session_id").
		Where("agent_runs.tenant_id = ? AND agent_runs.owner_id = ? AND agent_runs.run_id = ?", tenantID, ownerID, runID).
		Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return WorkbenchTaskFacts{}, agentruntime.ErrNotFound
		}
		return WorkbenchTaskFacts{}, err
	}
	facts := WorkbenchTaskFacts{
		TaskID:    row.SessionID,
		Attention: attentionOf(row.RunStatus, row.AttentionPend),
	}
	if row.Title != nil {
		facts.Title = strings.TrimSpace(*row.Title)
	}
	if row.ArchivedAt != nil {
		facts.ArchivedAt = row.ArchivedAt.UTC().Format(time.RFC3339Nano)
	}
	return facts, nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/application/repository/ -run TestReadTaskFactsForRunDerivesThreeLayerFactsWithinOwnerScope -count=1`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/application/repository/workbench_task_facts.go internal/application/repository/workbench_task_facts_test.go
git commit -m "feat(workbench): owner-scoped task facts read for the run snapshot (T05)"
```

---

### Task 2: Go——`ExecutionSnapshot.Task` wire 段与 handler 注入

**Files:**
- Modify: `internal/modules/workbench/contracts.go`（`ExecutionSnapshot` 定义 `:52` 追加字段，其后追加类型与 Validate）
- Modify: `internal/handler/session/workbench_read.go`（新增 port 与 `WithTaskFacts`；`GetWorkbenchSnapshot`（`:155`）改用 `resolveOwnedRun` 并注入 `task` 段）
- Modify: `internal/container/workbench.go`（`NewWorkbenchReadHandler` 提供者追加 `lists *repository.WorkbenchListStore` 参数并链式 `.WithTaskFacts(lists)`）
- Test: `internal/handler/session/workbench_read_task_facts_test.go`（新文件）

**Interfaces:**
- Consumes: Task 1 的 `ReadTaskFactsForRun`/`repository.WorkbenchTaskFacts`；既有 `resolveOwnedRun`（`workbench_read.go:108`——返回的 `Run` 带 `Owner`/`SessionID` 字段，`agent/runtime/contracts.go:114`）、测试助手 `workbenchRequest`/`workbenchRunReaderStub`/`workbenchSnapshotReaderStub`（`workbench_read_test.go:107`/`:27`/`:41`）；容器已 Provide `repository.NewWorkbenchListStore`（`internal/container/container.go:273`）。
- Produces: wire 形状 `GET /api/v1/workbench/executions/:run_id/snapshot` 响应 data 新增可选 `task: { task_id, title?, attention: "none"|"required", archived_at? }`（`omitempty`；`GetWorkbenchExecution` 不变）。Task 3 的 TS parser 与 Task 8 的 remote 消费；`(*WorkbenchReadHandler).WithTaskFacts(facts OwnedTaskFactsReader) *WorkbenchReadHandler` 构建器。

- [ ] **Step 1: 写失败测试**

`internal/handler/session/workbench_read_task_facts_test.go`（新文件，完整内容）：

```go
package session

import (
	"context"
	"net/http"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/workbench"
	"github.com/stretchr/testify/require"
)

type stubTaskFactsReader struct {
	facts repository.WorkbenchTaskFacts
	err   error
	calls int
}

func (s *stubTaskFactsReader) ReadTaskFactsForRun(context.Context, uint64, string, string) (repository.WorkbenchTaskFacts, error) {
	s.calls++
	return s.facts, s.err
}

func snapshotHandlerWithFacts(facts OwnedTaskFactsReader) *WorkbenchReadHandler {
	runs := &workbenchRunReaderStub{run: agentruntime.Run{Key: agentruntime.RunKey{TenantID: 1, RunID: "r1"}, Owner: "u1", SessionID: "s1"}}
	snapshots := &workbenchSnapshotReaderStub{snapshot: workbench.ExecutionSnapshot{Execution: workbench.ExecutionDTO{SchemaVersion: 1, RunID: "r1", SessionID: "s1", Driver: "platform", RunStatus: "running", ExecutionStatus: "running", SettlementStatus: "pending", Capabilities: map[string]workbench.Capability{}}}}
	h := NewWorkbenchReadHandler(runs, snapshots)
	if facts != nil {
		h = h.WithTaskFacts(facts)
	}
	return h
}

func TestGetWorkbenchSnapshotCarriesTaskFacts(t *testing.T) {
	facts := &stubTaskFactsReader{facts: repository.WorkbenchTaskFacts{TaskID: "s1", Title: "session-1", Attention: "required", ArchivedAt: "2026-09-23T00:00:00Z"}}
	c, w := workbenchRequest(t, "u1")
	snapshotHandlerWithFacts(facts).GetWorkbenchSnapshot(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"task":{"task_id":"s1"`)
	require.Contains(t, w.Body.String(), `"title":"session-1"`)
	require.Contains(t, w.Body.String(), `"attention":"required"`)
	require.Contains(t, w.Body.String(), `"archived_at":"2026-09-23T00:00:00Z"`)
	require.Equal(t, 1, facts.calls)
}

func TestGetWorkbenchSnapshotOmitsTaskSectionWithoutFactsReader(t *testing.T) {
	c, w := workbenchRequest(t, "u1")
	snapshotHandlerWithFacts(nil).GetWorkbenchSnapshot(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), `"task"`)
}

func TestGetWorkbenchSnapshotSurfacesFactsReadFailure(t *testing.T) {
	facts := &stubTaskFactsReader{err: context.DeadlineExceeded}
	c, w := workbenchRequest(t, "u1")
	snapshotHandlerWithFacts(facts).GetWorkbenchSnapshot(c)
	require.Equal(t, http.StatusInternalServerError, w.Code, "enrichment failure must fail honestly, not silently drop the task section")
}

func TestGetWorkbenchSnapshotFactsReadIsOwnerScoped(t *testing.T) {
	facts := &stubTaskFactsReader{facts: repository.WorkbenchTaskFacts{TaskID: "s1", Attention: "none"}}
	c, w := workbenchRequest(t, "other-user")
	snapshotHandlerWithFacts(facts).GetWorkbenchSnapshot(c)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Zero(t, facts.calls, "ownership is checked before any facts read")
}
```

（`workbenchRequest` 已在 `workbench_read_test.go:107` 设置 `c.Params` 与身份 context，直接复用。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/handler/session/ -run 'TestGetWorkbenchSnapshot' -count=1`
Expected: FAIL——`h.WithTaskFacts undefined` / `undefined: OwnedTaskFactsReader`（编译失败即 RED）。

- [ ] **Step 3: 最小实现**

3a. `internal/modules/workbench/contracts.go`——`ExecutionSnapshot` 结构体追加字段，并在其后追加类型：

```go
type ExecutionSnapshot struct {
	Execution          ExecutionDTO     `json:"execution"`
	Watermark          int64            `json:"watermark"`
	Incomplete         bool             `json:"incomplete"`
	ConfirmedWatermark int64            `json:"confirmed_watermark"`
	Events             []ExecutionEvent `json:"events"`
	// Task carries the task-level projection facts (T05) resolved at read
	// time. Optional: deployments without the facts reader omit the section
	// and the run section stays unchanged.
	Task *TaskSnapshotFacts `json:"task,omitempty"`
}

// TaskSnapshotFacts mirrors repository.WorkbenchTaskFacts on the wire. Title
// and ArchivedAt are omitempty; Attention is always emitted ("none"|"required").
type TaskSnapshotFacts struct {
	TaskID     string `json:"task_id"`
	Title      string `json:"title,omitempty"`
	Attention  string `json:"attention"`
	ArchivedAt string `json:"archived_at,omitempty"`
}

func (f TaskSnapshotFacts) Validate() error {
	if strings.TrimSpace(f.TaskID) == "" {
		return invalid("task.task_id", "required")
	}
	if f.Attention != "none" && f.Attention != "required" {
		return invalid("task.attention", `expected "none" or "required"`)
	}
	if f.ArchivedAt != "" && !iso8601.MatchString(f.ArchivedAt) {
		return invalid("task.archived_at", "expected an ISO-8601 timestamp")
	}
	return nil
}
```

（`strings`/`iso8601`/`invalid` 均为该文件既有标识符：`strings` 导入于 `contracts.go:10`、`iso8601` 定义于 `:60`、`invalid` 定义于 `:62`——行号为本计划作者实测。）

3b. `internal/handler/session/workbench_read.go`——接口区追加（`WorkbenchSourceIngestor` 之后，`:34` 附近）：

```go
// OwnedTaskFactsReader resolves task-level facts for one owned run (T05).
// Implementations bind every read to the authenticated tenant and owner.
type OwnedTaskFactsReader interface {
	ReadTaskFactsForRun(ctx context.Context, tenantID uint64, ownerID, runID string) (repository.WorkbenchTaskFacts, error)
}
```

`WorkbenchReadHandler` 结构体追加字段 `taskFacts OwnedTaskFactsReader`；构造函数（`:57`）之后追加：

```go
// WithTaskFacts attaches the task-facts reader used to enrich run snapshots
// with the task layer (title/archive/attention). Nil facts keep the legacy
// snapshot shape.
func (h *WorkbenchReadHandler) WithTaskFacts(facts OwnedTaskFactsReader) *WorkbenchReadHandler {
	h.taskFacts = facts
	return h
}
```

`GetWorkbenchSnapshot`（原 `:155`）整体替换为：

```go
func (h *WorkbenchReadHandler) GetWorkbenchSnapshot(c *gin.Context) {
	run, ok := resolveOwnedRun(c, h.runs)
	if !ok || h.snapshots == nil {
		return
	}
	snapshot, err := h.snapshots.ReadRunSnapshot(c.Request.Context(), run.Key)
	if err != nil {
		writeWorkbenchError(c, err)
		return
	}
	if h.taskFacts != nil {
		facts, factsErr := h.taskFacts.ReadTaskFactsForRun(c.Request.Context(), run.Key.TenantID, run.Owner, run.Key.RunID)
		if factsErr != nil {
			writeWorkbenchError(c, factsErr)
			return
		}
		task := workbench.TaskSnapshotFacts{TaskID: facts.TaskID, Title: facts.Title, Attention: facts.Attention, ArchivedAt: facts.ArchivedAt}
		if err := task.Validate(); err != nil {
			writeWorkbenchError(c, err)
			return
		}
		snapshot.Task = &task
	}
	writeWorkbenchJSON(c, snapshot)
}
```

3c. `internal/container/workbench.go`——`NewWorkbenchReadHandler` 签名与返回改为：

```go
func NewWorkbenchReadHandler(
	runs *repository.AgentRunStore,
	snapshots *repository.AgentRunSnapshotRepository,
	ingestor *repository.ExecutionObservationStore,
	lists *repository.WorkbenchListStore,
) *session.WorkbenchReadHandler {
	return session.NewWorkbenchReadHandler(runs, snapshots, ingestor).WithTaskFacts(lists)
}
```

（dig 容器已注册 `repository.NewWorkbenchListStore`，无需改 `container.go`。）

- [ ] **Step 4: 运行确认通过 + 构建**

Run: `go test ./internal/handler/session/ -run 'TestGetWorkbenchSnapshot|TestWorkbenchRead|TestStreamWorkbench|TestIngestWorkbenchSourceEvent' -count=1 && go build ./internal/... ./cmd/...`
Expected: PASS + 构建成功（既有读路径用例不回归）。

- [ ] **Step 5: Commit**

```bash
git add internal/modules/workbench/contracts.go internal/handler/session/workbench_read.go internal/handler/session/workbench_read_task_facts_test.go internal/container/workbench.go
git commit -m "feat(workbench): run snapshots carry owner-scoped task facts on the wire (T05)"
```

---

### Task 3: contracts TS——`ExecutionSnapshot.task` 解析

**Files:**
- Modify: `packages/contracts/src/mobile/execution.ts`（`ExecutionSnapshot` 接口 `:35` 与 `parseExecutionSnapshot` `:181`）
- Test: `packages/contracts/test/mobile-execution.test.ts`（追加用例）

**Interfaces:**
- Consumes: 既有 `parseExecutionSnapshot`/`object`/`nonEmpty`/`isoTime`（`execution.ts:81`–`:122`、`:181`）。
- Produces: `export interface SnapshotTaskFacts { task_id: string; title?: string; attention: SnapshotAttentionState; archived_at?: string }` 与 `export type SnapshotAttentionState = 'none' | 'required'`；`ExecutionSnapshot.task?: SnapshotTaskFacts`（缺省 = 旧服务端，合法）。Task 8 的 remote 消费。

- [ ] **Step 1: 写失败测试**

在 `packages/contracts/test/mobile-execution.test.ts` 末尾追加（`execution`/`event` 为该文件既有 fixture，`parseExecutionSnapshot` 已导入）：

```ts
test('snapshot task facts parse optionally and reject malformed values', () => {
  const base = { execution, watermark: 1, incomplete: false, confirmed_watermark: 0, events: [event] };
  const task = { task_id: 's', title: '季度报告', attention: 'required', archived_at: '2026-09-23T00:00:00Z' };
  assert.deepEqual(parseExecutionSnapshot({ ...base, task }).task, task);
  assert.equal(parseExecutionSnapshot({ ...base }).task, undefined, 'a legacy server omits the section');
  assert.equal(parseExecutionSnapshot({ ...base, task: { task_id: 's', attention: 'none' } }).task?.title, undefined, 'title is optional');
  assert.throws(() => parseExecutionSnapshot({ ...base, task: { task_id: '', attention: 'none' } }), /task.task_id/);
  assert.throws(() => parseExecutionSnapshot({ ...base, task: { task_id: 's', attention: 'maybe' } }), /task.attention/);
  assert.throws(() => parseExecutionSnapshot({ ...base, task: { task_id: 's', attention: 'none', archived_at: 'yesterday' } }), /task.archived_at/);
  assert.throws(() => parseExecutionSnapshot({ ...base, task: { task_id: 's', attention: 'none', title: 7 } }), /task.title/);
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/contracts/test/mobile-execution.test.ts`
Expected: FAIL——`(...).task` 为 `undefined`，`assert.deepEqual` 不等（tsx 只转译不查类型；类型层由 `pnpm --filter @weknora/mobile typecheck` 在 Task 9 收口）。

- [ ] **Step 3: 最小实现**

`packages/contracts/src/mobile/execution.ts`——`ExecutionSnapshot` 接口追加 `task?: SnapshotTaskFacts;`，并在该接口后追加：

```ts
/** 与 read-models 的 AttentionState 同构；独立定义避免 execution↔read-models 循环导入。 */
export type SnapshotAttentionState = 'none' | 'required';

/** Go 侧 TaskSnapshotFacts（读时派生的任务层事实，MX-003 之后新增的可选段）。 */
export interface SnapshotTaskFacts {
  task_id: string;
  title?: string;
  attention: SnapshotAttentionState;
  archived_at?: string;
}

function snapshotTaskFacts(value: unknown): SnapshotTaskFacts {
  const row = object(value, 'task');
  const attention = row.attention;
  if (attention !== 'none' && attention !== 'required') throw new ContractError('task.attention', 'expected "none" or "required"');
  const title = row.title;
  if (title !== undefined && title !== null && typeof title !== 'string') throw new ContractError('task.title', 'expected a string when present');
  const archivedAt = row.archived_at;
  if (archivedAt !== undefined && archivedAt !== null) isoTime(archivedAt, 'task.archived_at');
  return {
    task_id: nonEmpty(row.task_id, 'task.task_id'),
    ...(title === undefined || title === null ? {} : { title }),
    attention,
    ...(archivedAt === undefined || archivedAt === null ? {} : { archived_at: archivedAt }),
  };
}
```

`parseExecutionSnapshot` 返回对象追加一行（`events,` 之后）：

```ts
    ...(row.task === undefined || row.task === null ? {} : { task: snapshotTaskFacts(row.task) }),
```

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/contracts/test/mobile-execution.test.ts packages/contracts/test/mobile-read-models.test.ts packages/contracts/test/mobile-interactions.test.ts`
Expected: PASS（含既有用例）。

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src/mobile/execution.ts packages/contracts/test/mobile-execution.test.ts
git commit -m "feat(contracts): parse the optional snapshot task facts section (T05)"
```

---

### Task 4: mobile-core——三层状态与规范 Timeline 纯投影

**Files:**
- Create: `packages/mobile-core/src/task-office/task-timeline.ts`
- Test: `packages/mobile-core/src/task-office/task-timeline.test.ts`

**Interfaces:**
- Consumes: 无包外依赖（纯函数，不 import 任何文件——参数为结构类型）。
- Produces（Task 6/9 消费；Task 9 屏消费 `timelineKindLabel`）:
  - `export type TaskLifecycleState = 'active' | 'completed' | 'canceled' | 'archived'`
  - `export function taskLifecycleOf(archivedAt: string | undefined, runStatus: string): TaskLifecycleState`
  - `export function terminalRunStatusOf(base: string, events: ReadonlyArray<{ type: string }>): string`（逐字镜像服务端规则 `agent_run_snapshot.go:104-134`：仅当 base ∈ {queued, running, reconciling} 时由终态事件推进，优先级 canceled > failed > succeeded）
  - `export function isTerminalRunStatus(status: string): boolean`
  - `export function mergeEventHistory<E extends { seq: number }>(persisted: readonly E[], snapshot: readonly E[], watermark: number): E[]`（按 seq 去重升序；同 seq 时 snapshot 覆盖 persisted；seq > watermark 的持久行按权威上界丢弃）
  - `export type TaskTimelineKind = 'run_status' | 'conclusion' | 'tool_activity' | 'approval' | 'artifact' | 'activity'`
  - `export interface TaskTimelineEntry { seq: number; occurredAt: string; kind: TaskTimelineKind; type: string; summary: string; evidence: { payload: Record<string, unknown> } }`
  - `export interface TaskTimelineSourceEvent { seq: number; type: string; occurredAt: string; payload: Record<string, unknown> }`
  - `export function projectTimeline(events: readonly TaskTimelineSourceEvent[]): TaskTimelineEntry[]`
  - `export function timelineKindLabel(kind: TaskTimelineKind): string`

- [ ] **Step 1: 写失败测试**

`packages/mobile-core/src/task-office/task-timeline.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  isTerminalRunStatus, mergeEventHistory, projectTimeline, taskLifecycleOf, terminalRunStatusOf, timelineKindLabel,
} from './task-timeline.ts';

const source = (seq: number, type: string, payload: Record<string, unknown> = {}) =>
  ({ seq, type, occurredAt: '2026-09-23T00:00:00Z', payload });

test('task lifecycle keeps archive, completion and cancellation separate from run status', () => {
  assert.equal(taskLifecycleOf(undefined, 'running'), 'active');
  assert.equal(taskLifecycleOf(undefined, 'queued'), 'active');
  assert.equal(taskLifecycleOf(undefined, 'waiting_user'), 'active');
  assert.equal(taskLifecycleOf(undefined, 'reconciling'), 'active');
  assert.equal(taskLifecycleOf(undefined, 'failed'), 'active', 'a failed run leaves the goal in progress');
  assert.equal(taskLifecycleOf(undefined, 'succeeded'), 'completed');
  assert.equal(taskLifecycleOf(undefined, 'canceled'), 'canceled');
  assert.equal(taskLifecycleOf('2026-09-23T00:00:00Z', 'succeeded'), 'archived', 'archive wins over completion');
});

test('terminal run status mirrors the server projection rule and precedence', () => {
  assert.equal(terminalRunStatusOf('running', [source(1, 'run.completed')]), 'succeeded');
  assert.equal(terminalRunStatusOf('queued', [source(1, 'execution.succeeded')]), 'succeeded');
  assert.equal(terminalRunStatusOf('reconciling', [source(1, 'status.succeeded')]), 'succeeded');
  assert.equal(terminalRunStatusOf('running', [source(1, 'run.completed'), source(2, 'run.canceled')]), 'canceled', 'canceled beats succeeded, mirroring agent_run_snapshot.go');
  assert.equal(terminalRunStatusOf('running', [source(1, 'run.failed'), source(2, 'run.completed')]), 'succeeded', 'terminal facts follow the server precedence, not recency');
  assert.equal(terminalRunStatusOf('succeeded', [source(1, 'run.failed')]), 'succeeded', 'a settled base never regresses');
  assert.equal(terminalRunStatusOf('waiting_user', []), 'waiting_user');
  assert.equal(isTerminalRunStatus('succeeded'), true);
  assert.equal(isTerminalRunStatus('waiting_user'), false);
});

test('mergeEventHistory dedupes by seq, prefers snapshot facts and clamps corrupted cache rows', () => {
  const merged = mergeEventHistory(
    [source(1, 'run.started'), source(2, 'tool.started', { tool: 'cached' }), source(9, 'text.delta')],
    [source(2, 'tool.started', { tool: 'authoritative' }), source(3, 'run.completed')],
    3,
  );
  assert.deepEqual(merged.map((event) => event.seq), [1, 2, 3]);
  assert.deepEqual(merged[1]!.payload, { tool: 'authoritative' }, 'the same seq prefers the authoritative snapshot payload');
  assert.equal(merged.some((event) => event.seq === 9), false, 'a persisted row above the watermark is corrupted cache, never replayed');
});

test('projectTimeline classifies known kinds, preserves unknown types and keeps raw evidence', () => {
  const entries = projectTimeline([
    source(3, 'artifact.available', { artifact_id: 'a-1' }),
    source(1, 'run.started'),
    source(2, 'tool.started', { name: 'web.search', arguments: { q: '竞品' } }),
    source(4, 'future.receipt', { external: 'stub' }),
    source(5, 'interaction.required', {}),
  ]);
  assert.deepEqual(entries.map((entry) => entry.seq), [1, 2, 3, 4, 5], 'entries render in authoritative seq order');
  assert.deepEqual(entries.map((entry) => entry.kind), ['run_status', 'tool_activity', 'artifact', 'activity', 'approval']);
  assert.equal(entries[3]!.type, 'future.receipt', 'unknown types are preserved, never dropped or guessed');
  assert.equal(entries[3]!.summary, '执行状态已更新');
  assert.deepEqual(entries[1]!.evidence.payload, { name: 'web.search', arguments: { q: '竞品' } }, 'raw evidence stays available for expansion');
  assert.equal(timelineKindLabel('conclusion'), 'Agent 结论');
  assert.equal(timelineKindLabel('activity'), '活动');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/mobile-core/src/task-office/task-timeline.test.ts`
Expected: FAIL——`Cannot find module './task-timeline.ts'`。

- [ ] **Step 3: 最小实现**

`packages/mobile-core/src/task-office/task-timeline.ts`（新文件，完整内容）：

```ts
/**
 * Task 详情纯投影（module-seams §5：Task Office 拥有三层状态与 Timeline 投影）。
 * 三层状态依据 CONTEXT.md：任务生命周期（进行中/已完成/已取消/已归档）、
 * Run 状态（run_status）与关注状态（attention）分别存在；终态推进规则逐字
 * 镜像服务端 agent_run_snapshot.go projectExecutionEvents，不自行发明。
 * Timeline 是持久事件的事实流投影：已知类型分类，未知类型保留原文归入
 * activity（durable ingestion 不丢未知类型，投影同样不丢）；原始证据默认
 * 折叠、按需展开（CONTEXT.md「任务时间线」）。
 */
export type TaskLifecycleState = 'active' | 'completed' | 'canceled' | 'archived';
export type TaskTimelineKind = 'run_status' | 'conclusion' | 'tool_activity' | 'approval' | 'artifact' | 'activity';

export interface TaskTimelineSourceEvent {
  seq: number;
  type: string;
  occurredAt: string;
  payload: Record<string, unknown>;
}

export interface TaskTimelineEntry {
  seq: number;
  occurredAt: string;
  kind: TaskTimelineKind;
  type: string;
  summary: string;
  evidence: { payload: Record<string, unknown> };
}

export function taskLifecycleOf(archivedAt: string | undefined, runStatus: string): TaskLifecycleState {
  if (archivedAt !== undefined && archivedAt !== '') return 'archived';
  if (runStatus === 'succeeded') return 'completed';
  if (runStatus === 'canceled') return 'canceled';
  return 'active';
}

export function isTerminalRunStatus(status: string): boolean {
  return status === 'succeeded' || status === 'failed' || status === 'canceled';
}

/** 镜像 internal/application/repository/agent_run_snapshot.go:104-134 的推进与优先级。 */
export function terminalRunStatusOf(base: string, events: ReadonlyArray<{ type: string }>): string {
  if (base !== 'queued' && base !== 'running' && base !== 'reconciling') return base;
  let succeeded = false;
  let failed = false;
  let canceled = false;
  for (const event of events) {
    if (event.type === 'run.completed' || event.type === 'execution.succeeded' || event.type === 'status.succeeded') succeeded = true;
    else if (event.type === 'run.failed' || event.type === 'execution.failed' || event.type === 'status.failed') failed = true;
    else if (event.type === 'run.canceled' || event.type === 'execution.canceled' || event.type === 'status.canceled') canceled = true;
  }
  if (canceled) return 'canceled';
  if (failed) return 'failed';
  if (succeeded) return 'succeeded';
  return base;
}

/** App 重启合并：按 seq 去重升序；同 seq 以权威 snapshot 载荷为准；高于水位的持久行是损坏缓存，不得回放。 */
export function mergeEventHistory<E extends { seq: number }>(persisted: readonly E[], snapshot: readonly E[], watermark: number): E[] {
  const unique = new Map<number, E>();
  for (const event of [...persisted, ...snapshot]) {
    if (event.seq > watermark) continue;
    unique.set(event.seq, event);
  }
  return [...unique.values()].sort((a, b) => a.seq - b.seq);
}

const KIND_OF_TYPE: Record<string, TaskTimelineKind> = {
  'run.started': 'run_status', 'run.status': 'run_status', 'run.failed': 'run_status', 'run.canceled': 'run_status',
  'attempt.started': 'run_status', 'attempt.replaced': 'run_status', 'attempt.finished': 'run_status',
  'execution.failed': 'run_status', 'status.failed': 'run_status', 'execution.canceled': 'run_status', 'status.canceled': 'run_status',
  'run.completed': 'conclusion', 'execution.succeeded': 'conclusion', 'status.succeeded': 'conclusion',
  'tool.started': 'tool_activity', 'tool.planned': 'tool_activity', 'tool.completed': 'tool_activity', 'tool.result': 'tool_activity',
  'interaction.required': 'approval', 'decision.required': 'approval',
  'artifact.created': 'artifact', 'artifact.available': 'artifact',
};

const EVENT_SUMMARIES: Record<string, string> = {
  'run.started': '任务已开始', 'run.completed': '任务已完成', 'run.failed': '任务未能完成', 'run.canceled': '任务已取消',
  'tool.started': '正在使用工具', 'tool.completed': '工具处理完成',
  'interaction.required': '需要你的确认', 'decision.required': '需要一次决定',
  'artifact.created': '已生成产物', 'artifact.available': '产物已就绪',
};

const KIND_LABELS: Record<TaskTimelineKind, string> = {
  run_status: '运行状态', conclusion: 'Agent 结论', tool_activity: '工具活动', approval: '审批', artifact: '产物', activity: '活动',
};

export function timelineKindLabel(kind: TaskTimelineKind): string {
  return KIND_LABELS[kind];
}

export function projectTimeline(events: readonly TaskTimelineSourceEvent[]): TaskTimelineEntry[] {
  return [...events].sort((a, b) => a.seq - b.seq).map((event) => ({
    seq: event.seq,
    occurredAt: event.occurredAt,
    kind: KIND_OF_TYPE[event.type] ?? 'activity',
    type: event.type,
    summary: EVENT_SUMMARIES[event.type] ?? '执行状态已更新',
    evidence: { payload: event.payload },
  }));
}
```

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/mobile-core/src/task-office/task-timeline.test.ts`
Expected: PASS（4 用例）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/task-office/task-timeline.ts packages/mobile-core/src/task-office/task-timeline.test.ts
git commit -m "feat(mobile-core): three-layer state and canonical task timeline projections (T05)"
```

---

### Task 5: mobile-core——详情端口类型与 in-memory 场景 Adapter

**Files:**
- Create: `packages/mobile-core/src/task-office/task-detail.ts`（本任务仅落类型区与常量；`createTaskDetail` 实现由 Task 6 在同文件追加）
- Create: `packages/mobile-core/src/task-office/in-memory-task-detail.ts`
- Test: `packages/mobile-core/src/task-office/in-memory-task-detail.test.ts`

**Interfaces:**
- Consumes: `AttentionState`（`task-office.ts:15`）、`ScopeLease`（`runtime/types.ts:17`）、`TaskLifecycleState`/`TaskTimelineEntry`（Task 4）。
- Produces: `TaskConnectionState`/`TaskInterruptionReason`/`TaskBackendEvent`/`TaskBackendDetail`/`TaskStreamControlFrame`/`TaskDetailBackendPort`/`PersistedTaskProjection`/`TaskProjectionStore`/`TaskDetailView`/`TaskHandle`/`TaskDetailPorts` 类型与 `TASK_DETAIL_HISTORY_LIMIT = 200`（完整定义见下方文件内容，Task 6/8/9 消费）；Adapter 三件套：
  - `createInMemoryTaskProjectionStore(): TaskProjectionStore & { snapshot(): PersistedTaskProjection[] }`
  - `createScriptedTaskStream(): ScriptedTaskStream`（`opened`/`emit`/`control`/`end`/`fail`/`attach`；abort 后静默并 resolve）
  - `createScenarioTaskDetailBackend(handlers?): TaskDetailBackendPort & { detailCalls: string[]; streams: ScriptedTaskStream[] }`

`task-detail.ts` 类型区（本任务写入的完整文件内容）：

```ts
import type { ScopeLease } from '../runtime/types.ts';
import type { TaskLifecycleState, TaskTimelineEntry } from './task-timeline.ts';
import type { AttentionState } from './task-office.ts';

export const TASK_DETAIL_HISTORY_LIMIT = 200;

export type TaskConnectionState = 'syncing' | 'live' | 'interrupted' | 'drained';
export type TaskInterruptionReason = 'gap' | 'cursor-expired' | 'stream-error' | 'stream-ended-nonterminal' | 'persist-failed';

export interface TaskBackendEvent {
  runId: string;
  seq: number;
  type: string;
  occurredAt: string;
  payload: Record<string, unknown>;
}

export interface TaskBackendDetail {
  taskId: string;
  runId: string;
  title: string;
  attention: AttentionState;
  archivedAt?: string;
  execution: { runStatus: string; executionStatus: string; settlementStatus: string; revision: number; seq: number };
  watermark: number;
  incomplete: boolean;
  events: TaskBackendEvent[];
}

export interface TaskStreamControlFrame { code: string; message: string }

export interface TaskDetailBackendPort {
  detail(runId: string): Promise<TaskBackendDetail>;
  /** Resolves when the server ends the stream; rejects on transport error. Cursor trimming rejects with an Error whose code is 'TASK_STREAM_CURSOR_EXPIRED'. */
  stream(input: { runId: string; cursor: number; signal: AbortSignal; onEvent(event: TaskBackendEvent): void; onControl(frame: TaskStreamControlFrame): void }): Promise<void>;
}

export interface PersistedTaskProjection {
  taskId: string;
  runId: string;
  cursor: number;
  events: TaskBackendEvent[];
  savedAt: string;
}

export interface TaskProjectionStore {
  load(runId: string): Promise<PersistedTaskProjection | undefined>;
  save(projection: PersistedTaskProjection): Promise<void>;
}

export interface TaskDetailView {
  taskId: string;
  runId: string;
  title: string;
  lifecycle: TaskLifecycleState;
  runStatus: string;
  attention: AttentionState;
  executionStatus: string;
  settlementStatus: string;
  revision: number;
  cursor: number;
  incomplete: boolean;
  connection: TaskConnectionState;
  interruption?: { reason: TaskInterruptionReason; message?: string };
  timeline: TaskTimelineEntry[];
  duplicateSeqs: number[];
}

export interface TaskHandle {
  hydrate(): Promise<TaskDetailView>;
  view(): TaskDetailView | undefined;
  updates(listener: (view: TaskDetailView) => void): () => void;
  resync(): Promise<TaskDetailView>;
  close(reason?: string): void;
}

export interface TaskDetailPorts {
  backend: TaskDetailBackendPort;
  store: TaskProjectionStore;
  lease(): ScopeLease | undefined;
}
```

- [ ] **Step 1: 写失败测试**

`packages/mobile-core/src/task-office/in-memory-task-detail.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createInMemoryTaskProjectionStore, createScenarioTaskDetailBackend, createScriptedTaskStream } from './in-memory-task-detail.ts';
import type { TaskBackendDetail, TaskBackendEvent } from './task-detail.ts';

const event$ = (seq: number): TaskBackendEvent => ({ runId: 'run-1', seq, type: 'text.delta', occurredAt: '2026-09-23T00:00:00Z', payload: {} });
const detail$ = (): TaskBackendDetail => ({
  taskId: 'task-1', runId: 'run-1', title: 't', attention: 'none',
  execution: { runStatus: 'running', executionStatus: 'running', settlementStatus: 'pending', revision: 1, seq: 2 },
  watermark: 2, incomplete: false, events: [],
});

test('the projection store round-trips per run and overwrites on save', async () => {
  const store = createInMemoryTaskProjectionStore();
  assert.equal(await store.load('run-1'), undefined);
  await store.save({ taskId: 'task-1', runId: 'run-1', cursor: 2, events: [event$(1)], savedAt: '2026-09-23T00:00:00Z' });
  await store.save({ taskId: 'task-1', runId: 'run-1', cursor: 3, events: [event$(1), event$(2)], savedAt: '2026-09-23T00:00:01Z' });
  const loaded = await store.load('run-1');
  assert.equal(loaded?.cursor, 3);
  assert.equal(store.snapshot().length, 1);
});

test('a scripted stream delivers frames until end, fail, or abort', async () => {
  const first = createScriptedTaskStream();
  const events: number[] = [];
  const controls: string[] = [];
  await new Promise<void>((resolve, reject) => {
    const controller = new AbortController();
    first.attach({ signal: controller.signal, onEvent: (event) => events.push(event.seq), onControl: (frame) => controls.push(frame.code), resolve, reject });
    first.opened.push({ runId: 'run-1', cursor: 0 });
    first.emit(event$(1));
    first.control({ code: 'stream_error', message: 'x' });
    first.end();
  });
  assert.deepEqual(events, [1]);
  assert.deepEqual(controls, ['stream_error']);

  const second = createScriptedTaskStream();
  const failure = new Promise<void>((resolve, reject) => {
    const controller = new AbortController();
    second.attach({ signal: controller.signal, onEvent: () => {}, onControl: () => {}, resolve, reject });
    second.fail(new Error('HTTP 503'));
  });
  await assert.rejects(failure, /HTTP 503/);

  const third = createScriptedTaskStream();
  let abortedSeen = 0;
  await new Promise<void>((resolve) => {
    const controller = new AbortController();
    third.attach({ signal: controller.signal, onEvent: () => { abortedSeen += 1; }, onControl: () => {}, resolve, reject: () => {} });
    controller.abort();
    third.emit(event$(9)); // abort 之后静默
  });
  assert.equal(abortedSeen, 0, 'frames after an abort are never delivered');
});

test('the scenario backend records detail calls and hands out scripted streams', async () => {
  const scripted = createScriptedTaskStream();
  const backend = createScenarioTaskDetailBackend({ detail: async () => detail$(), stream: () => scripted });
  assert.deepEqual(await backend.detail('run-1'), detail$());
  assert.deepEqual(backend.detailCalls, ['run-1']);
  const streamPromise = backend.stream({ runId: 'run-1', cursor: 5, signal: new AbortController().signal, onEvent: () => {}, onControl: () => {} });
  assert.equal(backend.streams[0], scripted);
  assert.deepEqual(scripted.opened, [{ runId: 'run-1', cursor: 5 }]);
  scripted.end();
  await streamPromise;
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/mobile-core/src/task-office/in-memory-task-detail.test.ts`
Expected: FAIL——`Cannot find module './in-memory-task-detail.ts'`（`./task-detail.ts` 与之同时创建，否则第一个导入即失败）。

- [ ] **Step 3: 最小实现**

先写入上文 `task-detail.ts` 类型区，再创建 `packages/mobile-core/src/task-office/in-memory-task-detail.ts`（完整内容）：

```ts
import type { PersistedTaskProjection, TaskBackendDetail, TaskBackendEvent, TaskDetailBackendPort, TaskProjectionStore, TaskStreamControlFrame } from './task-detail.ts';

/** 进程内持久化投影（module-seams §5.4 Task Store Port 的 in-memory Adapter；生产为 Scoped Vault Adapter）。 */
export function createInMemoryTaskProjectionStore(): TaskProjectionStore & { snapshot(): PersistedTaskProjection[] } {
  const rows = new Map<string, PersistedTaskProjection>();
  return {
    snapshot: () => [...rows.values()],
    async load(runId) { return rows.get(runId); },
    async save(projection) { rows.set(projection.runId, projection); },
  };
}

interface ScriptedTaskStreamSink {
  signal: AbortSignal;
  onEvent(event: TaskBackendEvent): void;
  onControl(frame: TaskStreamControlFrame): void;
  resolve(): void;
  reject(error: unknown): void;
}

export interface ScriptedTaskStream {
  opened: Array<{ runId: string; cursor: number }>;
  emit(event: TaskBackendEvent): void;
  control(frame: TaskStreamControlFrame): void;
  end(): void;
  fail(error: unknown): void;
  attach(sink: ScriptedTaskStreamSink): void;
}

/** 供测试驱动的单连接 SSE 脚本（module-seams §12 in-memory scenario Adapter）。 */
export function createScriptedTaskStream(): ScriptedTaskStream {
  const opened: Array<{ runId: string; cursor: number }> = [];
  let sink: ScriptedTaskStreamSink | undefined;
  return {
    opened,
    emit(event) { if (sink !== undefined && !sink.signal.aborted) sink.onEvent(event); },
    control(frame) { if (sink !== undefined && !sink.signal.aborted) sink.onControl(frame); },
    end() { if (sink !== undefined) { const active = sink; sink = undefined; active.resolve(); } },
    fail(error) { if (sink !== undefined) { const active = sink; sink = undefined; active.reject(error); } },
    attach(next) {
      sink = next;
      next.signal.addEventListener('abort', () => { if (sink === next) { sink = undefined; next.resolve(); } });
    },
  };
}

export interface ScenarioTaskDetailHandlers {
  detail?: (runId: string) => Promise<TaskBackendDetail>;
  /** 每次模块开流调用一次；缺省返回新空脚本（挂起直到测试驱动）。 */
  stream?: (input: { runId: string; cursor: number }) => ScriptedTaskStream | undefined;
}

export function createScenarioTaskDetailBackend(handlers: ScenarioTaskDetailHandlers = {}): TaskDetailBackendPort & { detailCalls: string[]; streams: ScriptedTaskStream[] } {
  const detailCalls: string[] = [];
  const streams: ScriptedTaskStream[] = [];
  return {
    detailCalls,
    streams,
    async detail(runId) {
      detailCalls.push(runId);
      if (handlers.detail === undefined) throw new Error(`scenario detail not scripted for ${runId}`);
      return handlers.detail(runId);
    },
    async stream({ runId, cursor, signal, onEvent, onControl }) {
      const scripted = handlers.stream?.({ runId, cursor }) ?? createScriptedTaskStream();
      streams.push(scripted);
      scripted.opened.push({ runId, cursor });
      return new Promise<void>((resolve, reject) => {
        scripted.attach({ signal, onEvent, onControl, resolve, reject });
      });
    },
  };
}
```

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/mobile-core/src/task-office/in-memory-task-detail.test.ts`
Expected: PASS（3 用例）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/task-office/task-detail.ts packages/mobile-core/src/task-office/in-memory-task-detail.ts packages/mobile-core/src/task-office/in-memory-task-detail.test.ts
git commit -m "feat(mobile-core): detail ports and scripted in-memory scenario adapters (T05)"
```

---

### Task 6: mobile-core——`createTaskDetail` + `open()` + 全部恢复场景（AC1/AC2）

本任务是单一 RED→GREEN 周期：先写**全部**场景测试（水合、幂等、缺口、裁剪、持久化、有界重连、重启、断线、终态 drain、租约失效），实跑确认全部失败，再一次交付完整 `createTaskDetail` 实现与 `open()` 接线。这是有意的大任务——恢复语义不可拆分交付（半成品句柄即占位符），评审作为一个单元把守。

**Files:**
- Modify: `packages/mobile-core/src/task-office/task-detail.ts`（追加实现函数）
- Modify: `packages/mobile-core/src/task-office/task-office.ts`（`TaskOfficePorts` 追加可选 `detail`/`store`；`TaskOffice` 接口追加 `open`；`TaskOfficeErrorCode` 追加两个码；`createTaskOffice` 追加默认 store 与 `open` 实现）
- Modify: `packages/mobile-core/src/index.ts`（追加导出）
- Test: `packages/mobile-core/src/task-office/task-detail.test.ts`（新文件，本任务完整写入）

**Interfaces:**
- Consumes: Task 4 全部纯投影；Task 5 的类型与 Adapter；既有 `TaskOfficeError`/`TaskOfficePorts`/`createScenarioTaskBackend`（`task-office.ts:98`/`:93`；`in-memory-task-backend.ts:19`）、`leaseActive`/`ScopeLease`（`scope-lease.ts:26`、`types.ts:17`）。
- Produces:
  - `export function createTaskDetail(input: { taskId: string; runId: string }, ports: TaskDetailPorts): TaskHandle`
  - `TaskOffice.open(input: { taskId: string; runId: string }): TaskHandle`；错误码 `TASK_OFFICE_DETAIL_UNAVAILABLE`（未接线 detail port）与 `TASK_OFFICE_DETAIL_CLOSED`（close 后再 hydrate/resync）；`TaskOfficePorts.store?: TaskProjectionStore`（缺省为 office 内共享 in-memory store——同一 office 内两次 `open` 共享投影，App 重启场景由此承载）
  - 行为契约（AC1）：重复 seq 幂等跳过且 `duplicateSeqs` 可观测；`seq > cursor+1` 的缺口、`TASK_STREAM_CURSOR_EXPIRED` 拒绝与 `cursor_expired` 控制帧、持久化失败，均先以 `connection:'interrupted'` + `interruption.reason` 通知订阅者（不静默），再有界自动重同步（连续失败上限 2 次，显式 `resync()` 重置上限）
  - 行为契约（AC2）：App 重启（持久投影合并服务端裁剪窗口后续流）、断线（流错误 → 自动重同步恢复 live）、终态 drain（流结束 + 终态 → `drained` 不再重连）
  - 边界声明：`act(TaskIntent)` 属 #37/#38、`start(goal)` 属 #36；task 级「当前 run」服务端解析待 #36 新增运行后提供，当前 wire 为 run 作用域（`open` 需同时携带 taskId 与 runId，与列表行/深链恢复的既有事实一致）。

- [ ] **Step 1: 写全部失败测试**

`packages/mobile-core/src/task-office/task-detail.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import { createScenarioTaskBackend } from './in-memory-task-backend.ts';
import { createInMemoryTaskProjectionStore, createScenarioTaskDetailBackend, createScriptedTaskStream } from './in-memory-task-detail.ts';
import type { TaskBackendDetail, TaskBackendEvent, TaskProjectionStore } from './task-detail.ts';
import { createTaskOffice, TaskOfficeError } from './task-office.ts';

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => { resolve = next; });
  return { promise, resolve };
}

const settle = async (rounds = 12): Promise<void> => {
  for (let index = 0; index < rounds; index += 1) await new Promise<void>((resolve) => setImmediate(resolve));
};

function leased() {
  const revocable = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.test', userId: 'user-1', tenantId: 'tenant-1' });
  return { revocable, lease: revocable.asScopeLease() };
}

const event$ = (seq: number, type = 'text.delta'): TaskBackendEvent => ({ runId: 'run-1', seq, type, occurredAt: '2026-09-23T00:00:00Z', payload: { note: seq } });

function detail$(overrides: Partial<TaskBackendDetail> = {}): TaskBackendDetail {
  return {
    taskId: 'task-1', runId: 'run-1', title: '季度竞品报告', attention: 'required',
    execution: { runStatus: 'running', executionStatus: 'running', settlementStatus: 'pending', revision: 4, seq: 2 },
    watermark: 2, incomplete: false, events: [event$(1, 'run.started'), event$(2, 'tool.started')],
    ...overrides,
  };
}

function officeWithDetail(leaseRef: { lease?: ScopeLease }, handlers: Parameters<typeof createScenarioTaskDetailBackend>[0] = {}, store?: TaskProjectionStore) {
  const detailBackend = createScenarioTaskDetailBackend(handlers);
  const sharedStore = store ?? createInMemoryTaskProjectionStore();
  const office = createTaskOffice({
    backend: createScenarioTaskBackend({}),
    lease: () => leaseRef.lease,
    detail: detailBackend,
    store: sharedStore,
  });
  return { backend: detailBackend, store: sharedStore, office };
}

test('open rejects blank ids, a missing detail port, and use after close', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { office } = officeWithDetail(leaseRef, { detail: async () => detail$() });
  assert.throws(() => office.open({ taskId: ' ', runId: 'run-1' }), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_INVALID_INPUT');
  const plain = createTaskOffice({
    backend: createScenarioTaskBackend({}),
    lease: () => leaseRef.lease,
  });
  assert.throws(() => plain.open({ taskId: 'task-1', runId: 'run-1' }), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_DETAIL_UNAVAILABLE');
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  handle.close('test');
  await assert.rejects(handle.hydrate(), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_DETAIL_CLOSED');
});

test('hydrate renders the three layers result-first and keeps the timeline factual', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { office } = officeWithDetail(leaseRef, { detail: async () => detail$() });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  assert.equal(handle.view(), undefined, 'no view before hydration');
  const views: unknown[] = [];
  const unsubscribe = handle.updates((view) => views.push(view));
  const view = await handle.hydrate();
  assert.equal(view.taskId, 'task-1');
  assert.equal(view.title, '季度竞品报告');
  assert.equal(view.lifecycle, 'active');
  assert.equal(view.runStatus, 'running');
  assert.equal(view.attention, 'required');
  assert.equal(view.executionStatus, 'running');
  assert.equal(view.settlementStatus, 'pending');
  assert.equal(view.revision, 4);
  assert.equal(view.cursor, 2);
  assert.equal(view.incomplete, false);
  assert.deepEqual(view.timeline.map((entry) => entry.seq), [1, 2]);
  assert.equal(view.timeline[0]!.kind, 'run_status');
  assert.equal(view.timeline[1]!.kind, 'tool_activity');
  assert.equal(views.length, 1, 'hydration notifies subscribers exactly once with the settled view');
  assert.equal(view.connection, 'live', 'a non-terminal run opens the event stream');
  assert.deepEqual(view.duplicateSeqs, []);
  unsubscribe();
  handle.close();
});

test('a terminal or archived task drains without opening a stream', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { backend, office } = officeWithDetail(leaseRef, {
    detail: async () => detail$({
      archivedAt: '2026-09-23T01:00:00Z',
      attention: 'none',
      execution: { runStatus: 'succeeded', executionStatus: 'succeeded', settlementStatus: 'settled', revision: 9, seq: 2 },
    }),
  });
  const view = await office.open({ taskId: 'task-1', runId: 'run-1' }).hydrate();
  assert.equal(view.lifecycle, 'archived', 'archive wins over completed');
  assert.equal(view.runStatus, 'succeeded');
  assert.equal(view.connection, 'drained');
  assert.equal(backend.streams.length, 0, 'terminal runs never subscribe');
});

test('hydrate merges the persisted projection with the authoritative snapshot', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const store = createInMemoryTaskProjectionStore();
  await store.save({ taskId: 'task-1', runId: 'run-1', cursor: 2, events: [event$(1, 'run.started'), event$(2, 'tool.started')], savedAt: '2026-09-23T00:00:00Z' });
  const { office } = officeWithDetail(leaseRef, {
    detail: async () => detail$({ watermark: 3, events: [event$(3, 'run.completed')] }),
  }, store);
  const view = await office.open({ taskId: 'task-1', runId: 'run-1' }).hydrate();
  assert.deepEqual(view.timeline.map((entry) => entry.seq), [1, 2, 3], 'hydration unions persisted and snapshot history');
  assert.equal(view.cursor, 3);
  assert.equal(store.snapshot().find((row) => row.runId === 'run-1')!.cursor, 3, 'hydration persists the merged authoritative projection');
});

test('a revoked scope lease rejects hydration with foreign data never rendered', async () => {
  const { revocable, lease } = leased();
  const leaseRef: { lease?: ScopeLease } = { lease };
  const gate = deferred<TaskBackendDetail>();
  const { office } = officeWithDetail(leaseRef, { detail: () => gate.promise });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  const pending = handle.hydrate();
  revocable.revoke();
  gate.resolve(detail$({ title: 'foreign' }));
  await assert.rejects(pending, (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED');
  assert.equal(handle.view(), undefined);
});

test('live events append serially, duplicates are idempotent and observable', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { backend, store, office } = officeWithDetail(leaseRef, { detail: async () => detail$() });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate();
  const stream = backend.streams[0]!;
  assert.equal(stream.opened[0]!.cursor, 2, 'the stream resumes from the snapshot watermark');
  stream.emit(event$(3));
  await settle();
  assert.equal(handle.view()!.cursor, 3);
  assert.deepEqual(handle.view()!.timeline.map((entry) => entry.seq), [1, 2, 3]);
  assert.equal(store.snapshot().find((row) => row.runId === 'run-1')!.cursor, 3, 'each commit persists before the cursor advances');
  stream.emit(event$(3)); // 服务端重复投递
  stream.emit(event$(2));
  await settle();
  assert.equal(handle.view()!.cursor, 3, 'duplicates never advance the cursor');
  assert.deepEqual(handle.view()!.duplicateSeqs, [3, 2], 'idempotent skips stay observable');
  assert.equal(handle.view()!.timeline.filter((entry) => entry.seq === 3).length, 1, 'no double render');
  handle.close();
});

test('a foreign-run event or a malformed sequence interrupts the stream instead of entering the timeline', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { backend, office } = officeWithDetail(leaseRef, { detail: async () => detail$() });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  const reasons: Array<string | undefined> = [];
  handle.updates((view) => reasons.push(view.interruption?.reason));
  await handle.hydrate();
  backend.streams[0]!.emit({ ...event$(3), runId: 'run-other' });
  await settle();
  assert.ok(reasons.includes('stream-error'), 'a foreign-run event is surfaced, never merged');
  assert.deepEqual(handle.view()!.timeline.map((entry) => entry.seq), [1, 2], 'the foreign event never enters the timeline');
  handle.close();

  const malformedRef: { lease?: ScopeLease } = {};
  malformedRef.lease = leased().lease;
  const second = officeWithDetail(malformedRef, { detail: async () => detail$() });
  const secondHandle = second.office.open({ taskId: 'task-1', runId: 'run-1' });
  const malformedReasons: Array<string | undefined> = [];
  secondHandle.updates((view) => malformedReasons.push(view.interruption?.reason));
  await secondHandle.hydrate();
  second.backend.streams[0]!.emit({ ...event$(0), type: 'run.started' });
  await settle();
  assert.ok(malformedReasons.includes('stream-error'), 'a zero sequence is surfaced, never appended');
  assert.equal(secondHandle.view()!.cursor, 2);
  secondHandle.close();
});

test('a sequence gap is never silently skipped: interrupt, then authoritative resync', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const details = [detail$(), detail$({ watermark: 5, events: [event$(4, 'text.delta'), event$(5, 'run.completed')] })];
  const { backend, office } = officeWithDetail(leaseRef, { detail: async () => details.shift()! });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  const seen: Array<{ connection: string; reason?: string }> = [];
  handle.updates((view) => seen.push({ connection: view.connection, reason: view.interruption?.reason }));
  await handle.hydrate();
  backend.streams[0]!.emit(event$(5)); // 3 缺失
  await settle();
  assert.ok(seen.some((state) => state.connection === 'interrupted' && state.reason === 'gap'), 'the gap is surfaced, never silent');
  const view = handle.view()!;
  assert.equal(view.cursor, 5, 'resync adopts the authoritative watermark');
  assert.deepEqual(view.timeline.map((entry) => entry.seq), [1, 2, 4, 5], 'the gapped event arrives via snapshot exactly once');
  assert.equal(backend.streams.length, 2);
  assert.equal(backend.streams[1]!.opened[0]!.cursor, 5, 'the reopened stream continues from the new watermark');
  handle.close();
});

test('cursor trimming (409 or control frame) resyncs from a fresh snapshot', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  // 场景 A：开流即 409（error.code = TASK_STREAM_CURSOR_EXPIRED）
  {
    const details = [detail$(), detail$({ watermark: 3, events: [event$(3, 'text.delta')] })];
    const { office } = officeWithDetail(leaseRef, {
      detail: async () => details.shift()!,
      stream: ({ cursor }) => {
        if (cursor === 2) {
          const rejected = createScriptedTaskStream();
          queueMicrotask(() => rejected.fail(Object.assign(new Error('cursor expired'), { code: 'TASK_STREAM_CURSOR_EXPIRED' })));
          return rejected;
        }
        return undefined;
      },
    });
    const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
    const reasons: Array<string | undefined> = [];
    handle.updates((view) => reasons.push(view.interruption?.reason));
    await handle.hydrate();
    await settle();
    assert.ok(reasons.includes('cursor-expired'), 'the 409 surfaces as cursor-expired, never silent');
    assert.equal(handle.view()!.cursor, 3);
    handle.close();
  }
  // 场景 B：流中控制帧 cursor_expired
  {
    const details = [detail$(), detail$({ watermark: 4, events: [event$(4, 'text.delta')] })];
    const { backend, office } = officeWithDetail(leaseRef, { detail: async () => details.shift()! });
    const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
    const reasons: Array<string | undefined> = [];
    handle.updates((view) => reasons.push(view.interruption?.reason));
    await handle.hydrate();
    backend.streams[0]!.emit(event$(3));
    await settle();
    backend.streams[0]!.control({ code: 'cursor_expired', message: 'history trimmed' });
    await settle();
    assert.ok(reasons.includes('cursor-expired'));
    assert.equal(handle.view()!.cursor, 4);
    assert.equal(backend.streams[1]!.opened[0]!.cursor, 4);
    handle.close();
  }
});

test('a persist failure never advances the committed cursor', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const baseStore = createInMemoryTaskProjectionStore();
  let failedOnce = false;
  const store: TaskProjectionStore = {
    load: (runId) => baseStore.load(runId),
    save: (projection) => {
      if (!failedOnce && projection.cursor === 3) { failedOnce = true; return Promise.reject(new Error('quota exceeded')); }
      return baseStore.save(projection);
    },
  };
  const resyncGate = deferred<TaskBackendDetail>();
  const details = [detail$(), resyncGate.promise];
  const { backend, office } = officeWithDetail(leaseRef, { detail: async () => details.shift()! }, store);
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  const reasons: Array<string | undefined> = [];
  handle.updates((view) => reasons.push(view.interruption?.reason));
  await handle.hydrate();
  backend.streams[0]!.emit(event$(3));
  await settle();
  assert.ok(reasons.includes('persist-failed'), 'the failure is surfaced, never silent');
  assert.equal(handle.view()!.cursor, 2, 'the committed cursor stays at the last persisted value while the resync is gated');
  resyncGate.resolve(detail$({ watermark: 3, events: [event$(3, 'text.delta')] }));
  await settle();
  assert.equal(handle.view()!.cursor, 3, 'the automatic resync re-persists authoritatively');
  handle.close();
});

test('repeated stream failures stop automatic resync after the bound; explicit resync recovers', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const details = [detail$(), detail$(), detail$()];
  const { backend, office } = officeWithDetail(leaseRef, {
    detail: async () => details.shift() ?? detail$(),
    stream: () => {
      const failing = createScriptedTaskStream();
      queueMicrotask(() => failing.fail(new Error('HTTP 503')));
      return failing;
    },
  });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate();
  await settle(30);
  assert.equal(backend.streams.length, 3, 'bounded: initial stream + two automatic resyncs, no storm');
  assert.equal(handle.view()!.connection, 'interrupted');
  handle.close();
  // 显式 resync 解除上限并恢复（独立句柄验证恢复路径本身）
  const second = officeWithDetail(leaseRef, { detail: async () => detail$() });
  const secondHandle = second.office.open({ taskId: 'task-1', runId: 'run-1' });
  const recovered = await secondHandle.resync();
  second.backend.streams[0]!.emit(event$(3));
  await settle();
  assert.equal(recovered.connection, 'live');
  assert.equal(secondHandle.view()!.cursor, 3);
  secondHandle.close();
});

test('scenario: a network disconnect recovers through automatic resync and continues live', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const details = [detail$(), detail$({ watermark: 3, events: [event$(3, 'text.delta')] })];
  const { backend, office } = officeWithDetail(leaseRef, {
    detail: async () => details.shift()!,
    stream: ({ cursor }) => {
      if (cursor === 2) {
        const dropped = createScriptedTaskStream();
        queueMicrotask(() => dropped.fail(new Error('network unreachable')));
        return dropped;
      }
      return undefined;
    },
  });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  const reasons: Array<string | undefined> = [];
  handle.updates((view) => reasons.push(view.interruption?.reason));
  await handle.hydrate();
  await settle();
  assert.ok(reasons.includes('stream-error'), 'the disconnect is surfaced');
  assert.equal(handle.view()!.connection, 'live', 'automatic resync recovers the stream');
  assert.equal(handle.view()!.cursor, 3);
  backend.streams[1]!.emit(event$(4));
  await settle();
  assert.equal(handle.view()!.cursor, 4);
  handle.close();
});

test('scenario: app restart resumes from the persisted projection across server-side trimming', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const details = [
    detail$(), // 第一次水合：watermark 2
    detail$({ watermark: 5, events: [event$(5, 'text.delta')] }), // 重启后：服务端窗口只剩 5
  ];
  const { backend, store, office } = officeWithDetail(leaseRef, { detail: async () => details.shift()! });
  // 会话 1：水合并接收 3、4，随后 App 被杀（close 中止流）
  const first = office.open({ taskId: 'task-1', runId: 'run-1' });
  await first.hydrate();
  backend.streams[0]!.emit(event$(3));
  backend.streams[0]!.emit(event$(4));
  await settle();
  first.close('app-killed');
  assert.equal(store.snapshot().find((row) => row.runId === 'run-1')!.cursor, 4, 'the killed session leaves a durable projection');
  // 会话 2（同一 office 默认共享 store，模拟重启后读回持久化投影）
  const second = office.open({ taskId: 'task-1', runId: 'run-1' });
  const view = await second.hydrate();
  assert.deepEqual(view.timeline.map((entry) => entry.seq), [1, 2, 3, 4, 5], 'trimmed server history is backfilled from the persisted projection');
  assert.equal(backend.streams[1]!.opened[0]!.cursor, 5, 'the reopened stream resumes from the merged watermark');
  assert.equal(backend.detailCalls.length, 2);
  backend.streams[1]!.emit(event$(6, 'text.delta'));
  await settle();
  assert.equal(second.view()!.cursor, 6);
  second.close();
});

test('scenario: terminal drain ends the stream and never reconnects', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { backend, office } = officeWithDetail(leaseRef, { detail: async () => detail$() });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate();
  const stream = backend.streams[0]!;
  stream.emit(event$(3, 'run.completed'));
  await settle();
  stream.end(); // 服务端终态收流（Go StreamWorkbenchEvents 终态 drain 语义）
  await settle(20);
  const view = handle.view()!;
  assert.equal(view.runStatus, 'succeeded', 'terminal events advance the run status by the server rule');
  assert.equal(view.lifecycle, 'completed');
  assert.equal(view.connection, 'drained');
  assert.equal(backend.detailCalls.length, 1, 'drain is decided locally from durable events, no extra snapshot');
  assert.equal(backend.streams.length, 1, 'a drained task never reconnects');
  handle.close();
});

test('scenario: a non-terminal stream end resyncs once and continues live', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const details = [detail$(), detail$({ watermark: 3, events: [event$(3, 'text.delta')] })];
  const { backend, office } = officeWithDetail(leaseRef, { detail: async () => details.shift()! });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  const reasons: Array<string | undefined> = [];
  handle.updates((view) => reasons.push(view.interruption?.reason));
  await handle.hydrate();
  backend.streams[0]!.end(); // 服务器中途收流，运行仍非终态
  await settle();
  assert.ok(reasons.includes('stream-ended-nonterminal'), 'a non-terminal end is surfaced');
  assert.equal(handle.view()!.connection, 'live', 'resync reopens the stream');
  assert.equal(backend.streams[1]!.opened[0]!.cursor, 3);
  handle.close();
});

test('a revoked lease after hydration stops notifications and rejects resync', async () => {
  const { revocable, lease } = leased();
  const leaseRef: { lease?: ScopeLease } = { lease };
  const { office } = officeWithDetail(leaseRef, { detail: async () => detail$() });
  const handle = office.open({ taskId: 'task-1', runId: 'run-1' });
  await handle.hydrate();
  const views: number[] = [];
  handle.updates(() => views.push(1));
  revocable.revoke();
  await settle();
  assert.deepEqual(views, [], 'no notifications after the scope died');
  await assert.rejects(handle.resync(), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED');
  handle.close();
});
```

- [ ] **Step 2: 运行确认全部失败**

Run: `npx tsx --test packages/mobile-core/src/task-office/task-detail.test.ts`
Expected: FAIL——全部用例失败（`office.open is not a function`；`createTaskDetail` 尚不存在）。

- [ ] **Step 3: 一次交付完整实现**

3a. `packages/mobile-core/src/task-office/task-detail.ts` 头部 import 区调整为（类型区不动）：

```ts
import { leaseActive } from '../runtime/scope-lease.ts';
import { TaskOfficeError } from './task-office.ts';
import { isTerminalRunStatus, mergeEventHistory, projectTimeline, taskLifecycleOf, terminalRunStatusOf } from './task-timeline.ts';
```

（`AttentionState`/`ScopeLease`/`TaskLifecycleState`/`TaskTimelineEntry` 保持 `import type`，与 Task 5 的类型 import 合并为完整 import 区。**刻意循环导入警示**：`task-detail.ts` 值导入 `TaskOfficeError`（`task-office.ts`），而 `task-office.ts` 值导入 `createTaskDetail`（`task-detail.ts`）——两处均只在函数体内使用对方导出，tsx/esbuild 的 CJS 转译为惰性属性访问，可正常运行；实现者不得把对方导出用于模块顶层（如顶层常量初始化），否则循环会在初始化期爆炸。）

文件末尾追加实现（完整内容）：

```ts
const AUTO_RESYNC_LIMIT = 2;

export function createTaskDetail(input: { taskId: string; runId: string }, ports: TaskDetailPorts): TaskHandle {
  let closed = false;
  let current: TaskDetailView | undefined;
  let detail: TaskBackendDetail | undefined;
  let events: TaskBackendEvent[] = [];
  let committedCursor = 0;
  let duplicateSeqs: number[] = [];
  let interruption: TaskDetailView['interruption'];
  let controller: AbortController | undefined;
  let streamEpoch = 0;
  let chain: Promise<void> = Promise.resolve();
  let autoResyncs = 0;
  const listeners = new Set<(view: TaskDetailView) => void>();

  const requireOpen = (): void => {
    if (closed) throw new TaskOfficeError('TASK_OFFICE_DETAIL_CLOSED');
  };
  const requireLease = (): ScopeLease => {
    const lease = ports.lease();
    if (!lease || !leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
    return lease;
  };
  const wrap = async <T>(action: () => Promise<T>): Promise<T> => {
    try {
      return await action();
    } catch (error) {
      if (error instanceof TaskOfficeError) throw error;
      throw new TaskOfficeError('TASK_OFFICE_BACKEND', { cause: error });
    }
  };
  const buildView = (connection: TaskConnectionState): TaskDetailView => {
    const base = detail!;
    const runStatus = terminalRunStatusOf(base.execution.runStatus, events);
    return {
      taskId: base.taskId,
      runId: base.runId,
      title: base.title,
      lifecycle: taskLifecycleOf(base.archivedAt, runStatus),
      runStatus,
      attention: base.attention,
      executionStatus: base.execution.executionStatus,
      settlementStatus: base.execution.settlementStatus,
      revision: base.execution.revision,
      cursor: committedCursor,
      incomplete: base.incomplete,
      connection,
      ...(interruption === undefined ? {} : { interruption }),
      timeline: projectTimeline(events),
      duplicateSeqs: [...duplicateSeqs],
    };
  };
  const notify = (connection: TaskConnectionState): void => {
    if (closed || detail === undefined) return;
    current = buildView(connection);
    for (const listener of [...listeners]) listener(current);
  };
  const abortStream = (): void => {
    streamEpoch += 1;
    controller?.abort();
    controller = undefined;
  };
  const persist = async (cursor: number, history: TaskBackendEvent[]): Promise<void> => {
    await ports.store.save({ taskId: detail!.taskId, runId: input.runId, cursor, events: history.slice(-TASK_DETAIL_HISTORY_LIMIT), savedAt: new Date().toISOString() });
  };
  const hydrate = async (): Promise<TaskDetailView> => {
    requireOpen();
    const lease = requireLease();
    if (detail !== undefined) notify('syncing');
    const fetched = await wrap(() => ports.backend.detail(input.runId));
    if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
    detail = fetched;
    const persisted = await ports.store.load(input.runId).catch(() => undefined);
    events = mergeEventHistory(persisted?.events ?? [], fetched.events, fetched.watermark).slice(-TASK_DETAIL_HISTORY_LIMIT);
    committedCursor = fetched.watermark;
    duplicateSeqs = [];
    interruption = undefined;
    try {
      await persist(committedCursor, events);
    } catch (cause) {
      interruption = { reason: 'persist-failed', message: cause instanceof Error ? cause.message : String(cause) };
    }
    if (isTerminalRunStatus(terminalRunStatusOf(fetched.execution.runStatus, events))) {
      notify('drained');
      return current!;
    }
    if (interruption !== undefined) {
      notify('interrupted');
      return current!;
    }
    startStream();
    notify('live');
    return current!;
  };
  const interrupt = async (reason: TaskInterruptionReason, message?: string): Promise<void> => {
    abortStream();
    interruption = { reason, ...(message === undefined ? {} : { message }) };
    notify('interrupted'); // 缺口/裁剪/持久化失败先可见，绝不静默
    if (autoResyncs >= AUTO_RESYNC_LIMIT) return;
    autoResyncs += 1;
    try {
      await hydrate();
    } catch {
      /* 保持 interrupted 已通知状态；显式 resync() 可再试 */
    }
  };
  const startStream = (): void => {
    abortStream();
    const epoch = ++streamEpoch;
    controller = new AbortController();
    void ports.backend.stream({
      runId: input.runId,
      cursor: committedCursor,
      signal: controller.signal,
      onEvent: (event) => { chain = chain.then(() => processEvent(event)).catch(() => undefined); },
      onControl: (frame) => { chain = chain.then(() => processControl(frame)).catch(() => undefined); },
    }).then(
      () => { if (epoch === streamEpoch) chain = chain.then(() => streamEnded()).catch(() => undefined); },
      (error) => { if (epoch === streamEpoch) chain = chain.then(() => streamFailed(error)).catch(() => undefined); },
    );
  };
  const processEvent = async (event: TaskBackendEvent): Promise<void> => {
    if (closed || detail === undefined) return;
    if (event.runId !== input.runId) {
      await interrupt('stream-error', `event belongs to run ${event.runId}`);
      return;
    }
    if (!Number.isSafeInteger(event.seq) || event.seq < 1) {
      await interrupt('stream-error', `invalid event sequence ${String(event.seq)}`);
      return;
    }
    if (event.seq <= committedCursor) {
      duplicateSeqs = [...duplicateSeqs, event.seq]; // 重复事件幂等跳过，但可观测
      notify(current?.connection === 'drained' ? 'drained' : 'live');
      return;
    }
    if (event.seq !== committedCursor + 1) {
      await interrupt('gap', `expected seq ${committedCursor + 1}, received ${event.seq}`);
      return;
    }
    const nextEvents = [...events, event].slice(-TASK_DETAIL_HISTORY_LIMIT);
    try {
      await persist(event.seq, nextEvents); // 先持久化，后推进已提交游标
    } catch (cause) {
      await interrupt('persist-failed', cause instanceof Error ? cause.message : String(cause));
      return;
    }
    events = nextEvents;
    committedCursor = event.seq;
    autoResyncs = 0;
    notify('live');
  };
  const processControl = async (frame: TaskStreamControlFrame): Promise<void> => {
    if (closed) return;
    if (frame.code === 'cursor_expired') {
      await interrupt('cursor-expired', frame.message);
      return;
    }
    await interrupt('stream-error', `${frame.code}: ${frame.message}`);
  };
  const streamEnded = async (): Promise<void> => {
    if (closed || detail === undefined) return;
    if (isTerminalRunStatus(terminalRunStatusOf(detail.execution.runStatus, events))) {
      interruption = undefined;
      notify('drained');
      return;
    }
    await interrupt('stream-ended-nonterminal', 'the event stream ended before a terminal status');
  };
  const streamFailed = async (error: unknown): Promise<void> => {
    if (closed) return;
    const code = (error as { code?: unknown } | null)?.code;
    const message = error instanceof Error ? error.message : String(error);
    if (code === 'TASK_STREAM_CURSOR_EXPIRED') {
      await interrupt('cursor-expired', message);
      return;
    }
    await interrupt('stream-error', message);
  };
  return {
    hydrate: () => { requireOpen(); return hydrate(); },
    view: () => current,
    updates(listener) {
      if (closed) return () => undefined;
      listeners.add(listener);
      return () => { listeners.delete(listener); };
    },
    async resync(): Promise<TaskDetailView> {
      requireOpen();
      autoResyncs = 0; // 显式重同步解除有界自动重连的上限
      abortStream();
      return hydrate();
    },
    close(reason?: string) {
      closed = true;
      abortStream();
      listeners.clear();
      void reason;
    },
  };
}
```

3b. `packages/mobile-core/src/task-office/task-office.ts` 修改：

- 头部 import 追加：
```ts
import { createInMemoryTaskProjectionStore } from './in-memory-task-detail.ts';
import { createTaskDetail } from './task-detail.ts';
import type { TaskDetailBackendPort, TaskHandle, TaskProjectionStore } from './task-detail.ts';
```
- `TaskOfficeErrorCode` 联合追加 `| 'TASK_OFFICE_DETAIL_UNAVAILABLE' | 'TASK_OFFICE_DETAIL_CLOSED'`。
- `TaskOfficePorts` 追加：
```ts
  /** T05 详情与 SSE 恢复端口；缺失时 open() fail closed（TASK_OFFICE_DETAIL_UNAVAILABLE）。 */
  detail?: TaskDetailBackendPort;
  /** 持久化投影存储（App 重启恢复）；缺省为 office 内共享的 in-memory store。 */
  store?: TaskProjectionStore;
```
- `TaskOffice` 接口追加 `open(input: { taskId: string; runId: string }): TaskHandle;`
- `createTaskOffice` 闭包顶部追加 `const defaultDetailStore = createInMemoryTaskProjectionStore();`，返回对象追加：
```ts
    open(taskOpen: { taskId: string; runId: string }): TaskHandle {
      const taskId = taskOpen.taskId.trim();
      const runId = taskOpen.runId.trim();
      if (taskId === '' || runId === '') throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');
      if (ports.detail === undefined) throw new TaskOfficeError('TASK_OFFICE_DETAIL_UNAVAILABLE');
      return createTaskDetail({ taskId, runId }, { backend: ports.detail, store: ports.store ?? defaultDetailStore, lease: ports.lease });
    },
```

3c. `packages/mobile-core/src/index.ts` 追加导出：

```ts
export { createScriptedTaskStream, createScenarioTaskDetailBackend, createInMemoryTaskProjectionStore } from './task-office/in-memory-task-detail.ts';
export { createTaskDetail, TASK_DETAIL_HISTORY_LIMIT } from './task-office/task-detail.ts';
export type {
  PersistedTaskProjection, TaskBackendDetail, TaskBackendEvent, TaskConnectionState, TaskDetailView,
  TaskDetailBackendPort, TaskDetailPorts, TaskHandle, TaskInterruptionReason, TaskProjectionStore, TaskStreamControlFrame,
} from './task-office/task-detail.ts';
export type { TaskLifecycleState, TaskTimelineEntry, TaskTimelineKind, TaskTimelineSourceEvent } from './task-office/task-timeline.ts';
export { isTerminalRunStatus, mergeEventHistory, projectTimeline, taskLifecycleOf, terminalRunStatusOf, timelineKindLabel } from './task-office/task-timeline.ts';
export type { ScenarioTaskDetailHandlers, ScriptedTaskStream } from './task-office/in-memory-task-detail.ts';
```

- [ ] **Step 4: 运行确认全部通过（含既有套件）**

Run: `npx tsx --test 'packages/mobile-core/src/task-office/*.test.ts' packages/mobile-core/src/runtime/mobile-runtime.test.ts`
Expected: PASS（Task 4/5 既有用例 + 本任务全部新用例绿）。

- [ ] **Step 5: REFACTOR 检查**

确认 `interrupt`/`persist` 单一出口、全部 floating promise 均 `.then(onOk, onErr)` + `chain.catch`（无 unhandled rejection）、无重复逻辑。重跑 Step 4 命令保持 PASS。

- [ ] **Step 6: Commit**

```bash
git add packages/mobile-core/src/task-office/task-detail.ts packages/mobile-core/src/task-office/task-office.ts packages/mobile-core/src/task-office/task-detail.test.ts packages/mobile-core/src/index.ts
git commit -m "feat(mobile-core): task handle with snapshot hydration, sse recovery and restart scenarios (T05 AC1+AC2)"
```

---

### Task 7: mobile-core Runtime——授权 SSE 通道

**Files:**
- Modify: `packages/mobile-core/src/runtime/ports.ts`（新增 `AuthorizedStreamTransport` 类型与 `MobileRuntimePorts.authorizedStream?`）
- Modify: `packages/mobile-core/src/runtime/types.ts`（`MobileRuntime` 接口追加 `authorizedEventStream`）
- Modify: `packages/mobile-core/src/runtime/mobile-runtime.ts`（实现，紧随 `authorizedRequest` `:341` 之后）
- Test: `packages/mobile-core/src/runtime/mobile-runtime.test.ts`（追加用例）

**Interfaces:**
- Consumes: 既有 `authorizedRequest` 实现模式（`mobile-runtime.ts:341-369`：`unauthorizedStatus`/`refreshedCredential`/`current` scope 守卫）、`RuntimeAuthorizedRequest`（`types.ts:31`）。
- Produces:
  - `export type AuthorizedStreamTransport = (input: RuntimeAuthorizedRequest, accessToken: string, onChunk: (chunk: string) => void) => Promise<void>`（**契约**：pre-stream 401 以 `name === 'ApiError' && status === 401` 的错误拒绝且不产生 chunk；正常收流 resolve；传输错误 reject）
  - `MobileRuntime.authorizedEventStream(input: RuntimeAuthorizedRequest, onChunk: (chunk: string) => void): Promise<void>`（未授权或部署未提供流通道 → `RUNTIME_UNAUTHORIZED`；scope 变化 → `RUNTIME_SCOPE_CHANGED`；401 refresh 恰好一次后重放）；`MobileRuntimePorts.authorizedStream?: (deploymentOrigin: string) => AuthorizedStreamTransport | undefined`
  - Task 8/9/10 消费。

- [ ] **Step 1: 写失败测试**

在 `packages/mobile-core/src/runtime/mobile-runtime.test.ts` 末尾追加（复用文件既有 `DEPLOYMENT`/`fakeStore`/`remote`/`ports`/`deferred` 助手，镜像 `:624`–`:655` 的 `authorizedRequest` 用例惯例）：

```ts
test('authorizedEventStream rejects before any transport call when unauthorized', async () => {
  const runtime = createMobileRuntime(ports(fakeStore(), () => remote()));
  await assert.rejects(runtime.authorizedEventStream({ method: 'GET', path: '/api/v1/workbench/executions/r1/events?version=2' }, () => {}), /RUNTIME_UNAUTHORIZED/);
});

test('authorizedEventStream delivers chunks with the active token and refreshes exactly once on a pre-stream 401', async () => {
  const sentTokens: string[] = [];
  const chunks: string[] = [];
  const rotated = { token: 'access-2', refreshToken: 'refresh-2' };
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => remote({ refresh: async () => ({ access_token: rotated.token, refresh_token: rotated.refreshToken }) }),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    authorizedStream: () => async (_input, accessToken, onChunk) => {
      sentTokens.push(accessToken);
      if (accessToken === 'access-1') {
        const error = new Error('HTTP 401');
        error.name = 'ApiError';
        (error as unknown as { status?: number }).status = 401;
        throw error; // pre-stream 401：未产出任何 chunk
      }
      onChunk('id: 3\nevent: run.started\n');
      onChunk('data: {}\n\n');
    },
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  await runtime.authorizedEventStream({ method: 'GET', path: '/api/v1/workbench/executions/r1/events?version=2' }, (chunk) => chunks.push(chunk));
  assert.deepEqual(sentTokens, ['access-1', 'access-2']);
  assert.equal(chunks.join(''), 'id: 3\nevent: run.started\ndata: {}\n\n');
});

test('a stream opened before a scope change is dropped, and its chunks never flush', async () => {
  const release = deferred<void>();
  const chunks: string[] = [];
  const runtime = createMobileRuntime({
    credentialStore: fakeStore(),
    remoteFor: () => remote(),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    authorizedStream: () => async (_input, _accessToken, onChunk) => {
      await release.promise;
      onChunk('late frame');
    },
  });
  await runtime.signIn({ deployment: DEPLOYMENT, email: 'member@example.test', password: 'password' });
  const pending = runtime.authorizedEventStream({ method: 'GET', path: '/api/v1/workbench/executions/r1/events?version=2' }, (chunk) => chunks.push(chunk));
  await runtime.signOut();
  release.resolve();
  await assert.rejects(pending, /RUNTIME_SCOPE_CHANGED/);
  assert.deepEqual(chunks, [], 'no late frame is delivered after the scope died');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts`
Expected: FAIL——`runtime.authorizedEventStream is not a function`。

- [ ] **Step 3: 最小实现**

3a. `packages/mobile-core/src/runtime/ports.ts`——在 `AuthorizedTransport`（`:44`）后追加：

```ts
/** Authorized SSE read channel. Contract: a pre-stream 401 rejects (ApiError, status 401) with no chunks emitted; normal end resolves. */
export type AuthorizedStreamTransport = (
  input: RuntimeAuthorizedRequest,
  accessToken: string,
  onChunk: (chunk: string) => void,
) => Promise<void>;
```

`MobileRuntimePorts`（`:82` 附近）追加字段：

```ts
  /** Authorized SSE channel for child modules; same token discipline as authorizedTransport. May return undefined when the platform has no streaming fetch (fail closed). */
  authorizedStream?: (deploymentOrigin: string) => AuthorizedStreamTransport | undefined;
```

（允许工厂返回 `undefined`：composition 在 `expo/fetch` 不可用时返回 `undefined`，Runtime 侧 `transport` 为空即以 `RUNTIME_UNAUTHORIZED` fail closed——与「没有能力事实就不得放行」一致。）

3b. `packages/mobile-core/src/runtime/types.ts`——`MobileRuntime` 接口在 `authorizedRequest` 后追加：

```ts
  /** Streams one authorized SSE endpoint through the active deployment (refresh-once on a pre-stream 401). */
  authorizedEventStream(input: RuntimeAuthorizedRequest, onChunk: (chunk: string) => void): Promise<void>;
```

3c. `packages/mobile-core/src/runtime/mobile-runtime.ts`——在 `authorizedRequest` 实现后追加（镜像其 token 纪律）：

```ts
    async authorizedEventStream(input: RuntimeAuthorizedRequest, onChunk: (chunk: string) => void): Promise<void> {
      const deployment = activeDeployment;
      const transport = deployment && state.surface === 'authorized' ? ports.authorizedStream?.(deployment.origin) : undefined;
      if (!deployment || !transport) throw new Error('RUNTIME_UNAUTHORIZED');
      const requestEpoch = epoch;
      const send = async (token: string): Promise<void> => transport(input, token, onChunk);
      const credential = await ports.credentialStore.read(deployment.origin);
      if (!current(requestEpoch, deployment)) throw new Error('RUNTIME_SCOPE_CHANGED');
      if (!credential) throw new Error('RUNTIME_UNAUTHORIZED');
      try {
        await send(credential.token);
        if (!current(requestEpoch, deployment)) throw new Error('RUNTIME_SCOPE_CHANGED');
        return;
      } catch (error) {
        if (!unauthorizedStatus(error)) throw error;
        const refreshed = await refreshedCredential(requestEpoch, deployment, credential);
        if (!refreshed) throw new Error('RUNTIME_UNAUTHORIZED');
        if (!current(requestEpoch, deployment)) throw new Error('RUNTIME_SCOPE_CHANGED');
        await send(refreshed.token);
        if (!current(requestEpoch, deployment)) throw new Error('RUNTIME_SCOPE_CHANGED');
      }
    },
```

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts`
Expected: PASS（既有用例 + 新 3 用例）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/runtime/ports.ts packages/mobile-core/src/runtime/types.ts packages/mobile-core/src/runtime/mobile-runtime.ts packages/mobile-core/src/runtime/mobile-runtime.test.ts
git commit -m "feat(mobile-core): runtime-authorized sse channel with single-flight refresh and late-scope drop (T05)"
```

---

### Task 8: api-client——`createTaskOfficeRemote` 的 detail/stream

**Files:**
- Modify: `packages/api-client/src/mobile/task-office.ts`（新选项 `stream`、新方法 `detail`/`stream`；包 exports 不变——`./mobile/task-office` 已在 `packages/api-client/package.json`）
- Test: `packages/api-client/src/mobile/task-office.test.ts`（追加用例）

**Interfaces:**
- Consumes: 既有 `createExecutionsApi(request).snapshot`（`executions.ts:279`）、`executionEventsRequest(runID, lastEventID)`（`:251`——`version=2` + `Last-Event-ID`）、`createServerSentEventParser`（`chat/stream.ts:14`，帧含 `id/event/data`，注释行跳过）、`parseExecutionEvent`（contracts）；Task 3 的 `SnapshotTaskFacts`；Task 5 的 `TaskBackendDetail`/`TaskBackendEvent`/`TaskStreamControlFrame` 结构（remote 返回结构逐字一致，可赋值由 Task 9 的 `pnpm --filter @weknora/mobile typecheck` 证明）。
- Produces: `createTaskOfficeRemote(options: { origin: string; request: (input: ClientRequest) => Promise<unknown>; stream?: (input: ClientRequest, onChunk: (chunk: string) => void) => Promise<void> })` 返回对象新增：
  - `detail(runId: string): Promise<RemoteTaskDetail>`（`RemoteTaskDetail` 与 mobile-core `TaskBackendDetail` 结构逐字一致；事件映射 `run_id→runId`、`occurred_at→occurredAt`；`task` 段缺省 title `''`、attention `'none'`——与 contracts「旧读模型可缺省（展示层按 'none' 兜底）」既有决定一致，`read-models.ts:43`）
  - `stream(input: { runId: string; cursor: number; signal: AbortSignal; onEvent(event): void; onControl(frame): void }): Promise<void>`（`event: control` 帧 → `onControl`，**永不解析为业务事件、永不推进游标**；业务帧 → `parseExecutionEvent` → `onEvent`；流传输以 `ApiError`+`status===409` 拒绝时转译为 `code === 'TASK_STREAM_CURSOR_EXPIRED'` 的 Error；缺 `stream` 选项调用即抛错——fail closed）
  - 类型 `RemoteTaskStreamEvent`/`RemoteTaskDetail`/`TaskOfficeStreamOption`。Task 9/10 消费。

- [ ] **Step 1: 写失败测试**

在 `packages/api-client/src/mobile/task-office.test.ts` 末尾追加（该文件已导入 `createTaskOfficeRemote`）：

```ts
test('detail maps the snapshot wire (with task section) onto the module DTO', async () => {
  const execution = {
    schema_version: 1, run_id: 'r1', session_id: 's1', revision: 7, driver: 'platform',
    run_status: 'waiting_user', execution_status: 'waiting_user', settlement_status: 'pending', seq: 4, capabilities: {},
  };
  const wireEvent = { schema_version: 1, run_id: 'r1', attempt_id: 'a1', seq: 4, type: 'interaction.required', occurred_at: '2026-09-23T00:00:00Z', payload: { kind: 'tool_approval' } };
  const remote = createTaskOfficeRemote({
    origin: 'https://weknora.example.test',
    request: async () => ({ success: true, data: { execution, watermark: 4, incomplete: false, confirmed_watermark: 4, events: [wireEvent], task: { task_id: 's1', title: '报告', attention: 'required', archived_at: '2026-09-23T01:00:00Z' } } }),
  });
  const detail = await remote.detail('r1');
  assert.deepEqual(detail, {
    taskId: 's1', runId: 'r1', title: '报告', attention: 'required', archivedAt: '2026-09-23T01:00:00Z',
    execution: { runStatus: 'waiting_user', executionStatus: 'waiting_user', settlementStatus: 'pending', revision: 7, seq: 4 },
    watermark: 4, incomplete: false,
    events: [{ runId: 'r1', seq: 4, type: 'interaction.required', occurredAt: '2026-09-23T00:00:00Z', payload: { kind: 'tool_approval' } }],
  });
});

test('detail defaults the task layer for a legacy server without the task section', async () => {
  const execution = { schema_version: 1, run_id: 'r1', session_id: 's1', revision: 0, driver: 'platform', run_status: 'running', execution_status: 'running', settlement_status: 'pending', seq: 1, capabilities: {} };
  const remote = createTaskOfficeRemote({
    origin: 'https://weknora.example.test',
    request: async () => ({ success: true, data: { execution, watermark: 1, incomplete: false, confirmed_watermark: 1, events: [] } }),
  });
  const detail = await remote.detail('r1');
  assert.equal(detail.title, '');
  assert.equal(detail.attention, 'none');
  assert.equal('archivedAt' in detail, false);
});

test('the stream resumes from the cursor, routes control frames separately and never parses them as business events', async () => {
  const business = { schema_version: 1, run_id: 'r1', attempt_id: 'a1', seq: 6, type: 'run.completed', occurred_at: '2026-09-23T00:00:00Z', payload: {} };
  const seenRequests: Array<{ path: string; headers?: Record<string, string> }> = [];
  const wire = [
    ': heartbeat\n\n',
    `id: 6\nevent: run.completed\ndata: ${JSON.stringify(business)}\n\n`,
    `event: control\ndata: ${JSON.stringify({ code: 'cursor_expired', message: 'history trimmed' })}\n\n`,
  ].join('');
  const remote = createTaskOfficeRemote({
    origin: 'https://weknora.example.test',
    request: async () => { throw new Error('no JSON call expected'); },
    stream: async (input, onChunk) => {
      seenRequests.push({ path: input.path, headers: input.headers });
      onChunk(wire.slice(0, 20));
      onChunk(wire.slice(20));
    },
  });
  const events: unknown[] = [];
  const controls: Array<{ code: string; message: string }> = [];
  await remote.stream({ runId: 'r1', cursor: 5, signal: new AbortController().signal, onEvent: (event) => events.push(event), onControl: (frame) => controls.push(frame) });
  assert.equal(seenRequests[0]!.path, '/api/v1/workbench/executions/r1/events?version=2');
  assert.equal(seenRequests[0]!.headers?.['Last-Event-ID'], '5', 'resume continues from the module cursor');
  assert.deepEqual(events, [{ runId: 'r1', seq: 6, type: 'run.completed', occurredAt: '2026-09-23T00:00:00Z', payload: {} }]);
  assert.deepEqual(controls, [{ code: 'cursor_expired', message: 'history trimmed' }], 'control frames classify separately');
});

test('a 409 stream failure is translated to TASK_STREAM_CURSOR_EXPIRED and missing transport fails closed', async () => {
  const failing = async (): Promise<void> => {
    const error = new Error('HTTP 409');
    error.name = 'ApiError';
    (error as unknown as { status?: number }).status = 409;
    throw error;
  };
  const remote = createTaskOfficeRemote({ origin: 'https://weknora.example.test', request: async () => ({}), stream: failing });
  await assert.rejects(
    remote.stream({ runId: 'r1', cursor: 9, signal: new AbortController().signal, onEvent: () => {}, onControl: () => {} }),
    (error: unknown) => (error as { code?: string }).code === 'TASK_STREAM_CURSOR_EXPIRED',
  );
  const unstreamed = createTaskOfficeRemote({ origin: 'https://weknora.example.test', request: async () => ({}) });
  await assert.rejects(
    unstreamed.stream({ runId: 'r1', cursor: 9, signal: new AbortController().signal, onEvent: () => {}, onControl: () => {} }),
    /stream transport is required/,
  );
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/api-client/src/mobile/task-office.test.ts`
Expected: FAIL——`remote.detail is not a function` / `remote.stream is not a function`。

- [ ] **Step 3: 最小实现**

`packages/api-client/src/mobile/task-office.ts`——头部 import 调整为：

```ts
import { parseExecutionEvent } from '@weknora/contracts';
import { createServerSentEventParser } from '../chat/stream.ts';
import { createExecutionsApi, executionEventsRequest } from './executions.ts';
import { createOverviewApi } from './overview.ts';
import type { ClientRequest } from '../client.ts';
```

`TaskOfficeRemoteOptions` 追加：

```ts
  /** T05 authorized SSE channel（MobileRuntime.authorizedEventStream 或测试替身）；本适配器不新建传输。 */
  stream?: (input: ClientRequest, onChunk: (chunk: string) => void) => Promise<void>;
```

类型别名追加（与 mobile-core `TaskBackendDetail`/`TaskBackendEvent`/`TaskStreamControlFrame` 结构逐字一致——由 Task 9 的 typecheck 证明）：

```ts
export interface RemoteTaskStreamEvent { runId: string; seq: number; type: string; occurredAt: string; payload: Record<string, unknown> }
export interface RemoteTaskDetail {
  taskId: string; runId: string; title: string; attention: 'none' | 'required'; archivedAt?: string;
  execution: { runStatus: string; executionStatus: string; settlementStatus: string; revision: number; seq: number };
  watermark: number; incomplete: boolean; events: RemoteTaskStreamEvent[];
}
export type TaskOfficeStreamOption = NonNullable<TaskOfficeRemoteOptions['stream']>;
```

返回对象追加两个方法（`restore` 之后）：

```ts
    async detail(runId: string): Promise<RemoteTaskDetail> {
      const snapshot = await executionsApi.snapshot(runId);
      const task = snapshot.task;
      return {
        taskId: snapshot.execution.session_id,
        runId: snapshot.execution.run_id,
        title: task?.title ?? '',
        ...(task?.archived_at === undefined ? {} : { archivedAt: task.archived_at }),
        attention: task?.attention ?? 'none',
        execution: {
          runStatus: snapshot.execution.run_status,
          executionStatus: snapshot.execution.execution_status,
          settlementStatus: snapshot.execution.settlement_status,
          revision: snapshot.execution.revision,
          seq: snapshot.execution.seq,
        },
        watermark: snapshot.watermark,
        incomplete: snapshot.incomplete,
        events: snapshot.events.map((event) => ({ runId: event.run_id, seq: event.seq, type: event.type, occurredAt: event.occurred_at, payload: event.payload })),
      };
    },
    async stream(input: { runId: string; cursor: number; signal: AbortSignal; onEvent(event: RemoteTaskStreamEvent): void; onControl(frame: { code: string; message: string }): void }): Promise<void> {
      const transport = options.stream;
      if (!transport) throw new Error('task office stream transport is required for event streams');
      const request = executionEventsRequest(input.runId, String(input.cursor));
      const parser = createServerSentEventParser((frame) => {
        if (frame.event === 'control') {
          const payload = JSON.parse(frame.data) as { code?: string; message?: string };
          input.onControl({ code: payload.code ?? 'stream_error', message: payload.message ?? '' });
          return;
        }
        const event = parseExecutionEvent(JSON.parse(frame.data));
        input.onEvent({ runId: event.run_id, seq: event.seq, type: event.type, occurredAt: event.occurred_at, payload: event.payload });
      });
      await transport({ method: request.method, path: request.path, headers: request.headers, signal: input.signal }, (chunk) => parser.push(chunk)).catch((error: unknown) => {
        const shape = error as { name?: unknown; status?: unknown } | null;
        if (typeof shape === 'object' && shape !== null && shape.name === 'ApiError' && shape.status === 409) {
          const expired = new Error('workbench event stream cursor expired');
          (expired as unknown as { code?: string }).code = 'TASK_STREAM_CURSOR_EXPIRED';
          throw expired;
        }
        throw error;
      });
      parser.finish();
    },
```

（`snapshot.task` 来自 Task 3 的 contracts 类型；`parseExecutionEvent` 的强校验承担业务帧防线。）

- [ ] **Step 4: 运行确认通过**

Run: `npx tsx --test packages/api-client/src/mobile/task-office.test.ts packages/api-client/src/mobile/executions.test.ts packages/api-client/src/mobile/interactions.test.ts`
Expected: PASS（含既有用例）。

- [ ] **Step 5: Commit**

```bash
git add packages/api-client/src/mobile/task-office.ts packages/api-client/src/mobile/task-office.test.ts
git commit -m "feat(api-client): task office remote detail and v2 sse stream adapter (T05)"
```

---

### Task 9: apps/mobile——详情屏、原生 SSE adapter 与接线

**Files:**
- Create: `apps/mobile/src/adapters/sse-stream.ts`
- Create: `apps/mobile/src/adapters/sse-stream.test.ts`
- Create: `apps/mobile/src/task-detail-view.ts`
- Create: `apps/mobile/src/task-detail-view.test.ts`
- Create: `apps/mobile/src/screens/TaskDetailScreen.tsx`
- Create: `apps/mobile/src/app/tasks/detail.tsx`
- Modify: `apps/mobile/src/composition.ts`（三处：`authorizedStream` 接线进 `createNativeMobileRuntime` ports；`taskOfficeFor` 的 remote 增加 `stream`；`MobileTasks` 透传 `onOpenTask` 并新增导出 `activeTaskOffice`）
- Modify: `apps/mobile/src/screens/TasksScreen.tsx`（新增可选 `onOpenTask` prop 与行内详情按钮）
- Modify: `apps/mobile/src/app/tasks.tsx`（注入 `onOpenTask` → `router.push('/tasks/detail')`）
- Modify: `apps/mobile/src/app-smoke.test.tsx`（文件末尾追加一个测试块）

**Interfaces:**
- Consumes: Task 6 的 `TaskOffice.open`/`TaskHandle`/`TaskDetailView`；Task 7 的 `MobileRuntime.authorizedEventStream`；Task 8 的 `createTaskOfficeRemote({ origin, request, stream })`；既有 `taskOfficeFor`（`composition.ts:91`）、`activeMobileRuntime`（`:138`）、ResourcesRouteLifecycle 挂载模式（`app/resources.tsx`）、`createResourceShelfController` 控制器模式（`resources-view.ts`）、`ResourcesScreen` 纯 props 屏模式（`screens/ResourcesScreen.tsx`）。
- Produces:
  - `apps/mobile/src/adapters/sse-stream.ts`：`export type SseFetchLike = (input: string, init?: { method?: string; headers?: Record<string, string>; signal?: AbortSignal }) => Promise<Response>` 与 `export async function streamAuthorizedSse(origin: string, input: { method: string; path: string; headers?: Record<string, string>; signal?: AbortSignal }, accessToken: string, onChunk: (chunk: string) => void, fetchLike: SseFetchLike): Promise<void>`（非 2xx 以 `name:'ApiError'`+`status` 拒绝；UTF-8 流式解码）
  - `apps/mobile/src/task-detail-view.ts`：`createTaskDetailController(handle: TaskHandle): TaskDetailController`（`state()/subscribe(listener)/refresh()/whenSettled()/dispose()`，与 `resources-view.ts` 同构；`refresh()` 调 `handle.resync()`；`dispose()` 关闭句柄）
  - `apps/mobile/src/screens/TaskDetailScreen.tsx`：`export interface TaskDetailScreenProps { view?: TaskDetailView; loading: boolean; error?: string; onRefresh(): void }`——结果优先（状态卡 + 三层状态行 + attention 横幅）、Timeline 分节、**原始证据默认折叠**（按 seq 展开开关，仅渲染键值文本）
  - 路由 `/tasks/detail?taskId=..&runId=..`（expo-router 文件 `app/tasks/detail.tsx`，经 `activeTaskOffice().open(...)`，屏不触碰 wire 层）
  - Task 10 消费 `streamAuthorizedSse`。
  - 已知边界：composition 的 `authorizedStream` 惰性 `require('expo/fetch')`（Node 测试环境无此模块 → 返回 undefined → `authorizedEventStream` 拒绝，详情流表现为 `interrupted`——fail closed，不伪造成 live）；`TaskProjectionStore` 用 Task 6 的 office 内共享 in-memory store，跨进程加密持久化属 #40。

- [ ] **Step 1: 写失败测试（adapter + 控制器）**

`apps/mobile/src/adapters/sse-stream.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { streamAuthorizedSse } from './sse-stream.ts';

const encoder = new TextEncoder();
const streamOf = (parts: string[]) => new ReadableStream<Uint8Array>({
  start(controller) { for (const part of parts) controller.enqueue(encoder.encode(part)); controller.close(); },
});

test('streams sse bytes as text chunks and sends the bearer token', async () => {
  const calls: Array<{ url: string; init?: RequestInit }> = [];
  const chunks: string[] = [];
  await streamAuthorizedSse(
    'https://weknora.example.test',
    { method: 'GET', path: '/api/v1/workbench/executions/r1/events?version=2', headers: { 'Last-Event-ID': '5' } },
    'access-1',
    (chunk) => chunks.push(chunk),
    async (url, init) => { calls.push({ url, init: init as RequestInit }); return new Response(streamOf(['id: 6\neve', 'nt: run.started\ndata: {}\n\n'])); },
  );
  assert.equal(calls.length, 1);
  assert.equal(calls[0]!.url, 'https://weknora.example.test/api/v1/workbench/executions/r1/events?version=2');
  const headers = calls[0]!.init!.headers as Record<string, string>;
  assert.equal(headers.authorization, 'Bearer access-1');
  assert.equal(headers['Last-Event-ID'], '5');
  assert.equal(headers.accept, 'text/event-stream');
  assert.equal(chunks.join(''), 'id: 6\nevent: run.started\ndata: {}\n\n');
});

test('a non-200 stream rejects in the ApiError shape (401 refresh / 409 cursor mapping depend on it)', async () => {
  await assert.rejects(
    streamAuthorizedSse('https://weknora.example.test', { method: 'GET', path: '/p' }, 't', () => {}, async () => new Response('gone', { status: 409 })),
    (error: unknown) => (error as { name?: string; status?: number }).name === 'ApiError' && (error as { status?: number }).status === 409,
  );
});
```

`apps/mobile/src/task-detail-view.test.ts`（新文件，完整内容；手写最小 `TaskHandle` 替身，验证控制器只依赖 Interface）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import type { TaskDetailView, TaskHandle } from '@weknora/mobile-core';
import { createTaskDetailController } from './task-detail-view.ts';

const view$ = (connection: TaskDetailView['connection'], seqs: number[] = [1]): TaskDetailView => ({
  taskId: 'task-1', runId: 'run-1', title: '报告', lifecycle: 'active', runStatus: 'running', attention: 'none',
  executionStatus: 'running', settlementStatus: 'pending', revision: 1, cursor: seqs.at(-1) ?? 0, incomplete: false,
  connection,
  timeline: seqs.map((seq) => ({ seq, occurredAt: '2026-09-23T00:00:00Z', kind: 'run_status' as const, type: 'run.started', summary: '任务已开始', evidence: { payload: {} } })),
  duplicateSeqs: [],
});

function handleFake(views: TaskDetailView[]): TaskHandle & { resyncCount(): number; push(view: TaskDetailView): void } {
  let index = 0;
  let resyncs = 0;
  let listener: ((view: TaskDetailView) => void) | undefined;
  return {
    push(view: TaskDetailView) { listener?.(view); },
    resyncCount: () => resyncs,
    async hydrate() { return views[index++] ?? views[views.length - 1]!; },
    view: () => views[Math.max(index - 1, 0)],
    updates(next) { listener = next; return () => { listener = undefined; }; },
    async resync() { resyncs += 1; return views[Math.min(index, views.length - 1)]!; },
    close() {},
  };
}

test('the controller publishes the hydrated view, streams updates and refresh resyncs the handle', async () => {
  const fake = handleFake([view$('live'), view$('drained', [1, 2])]);
  const controller = createTaskDetailController(fake);
  await controller.whenSettled();
  assert.deepEqual(controller.state().view?.timeline.map((entry) => entry.seq), [1]);
  assert.equal(controller.state().loading, false);
  fake.push(view$('live', [1, 2])); // updates 增量
  assert.deepEqual(controller.state().view?.timeline.map((entry) => entry.seq), [1, 2]);
  await controller.refresh();
  assert.equal(controller.state().view?.connection, 'drained');
  assert.equal(fake.resyncCount(), 1);
  controller.dispose();
});
```

- [ ] **Step 2: 运行确认失败**

Run: `pnpm --filter @weknora/mobile test`
Expected: FAIL——`Cannot find module './sse-stream.ts'` / `'./task-detail-view.ts'`。

- [ ] **Step 3: 最小实现**

3a. `apps/mobile/src/adapters/sse-stream.ts`（完整内容）：

```ts
/** 流式读取授权 SSE 响应。非 2xx 以 ApiError 形态拒绝（Runtime 401 重试与远端 409 映射都依赖该形态）。 */
export type SseFetchLike = (input: string, init?: { method?: string; headers?: Record<string, string>; signal?: AbortSignal }) => Promise<Response>;

export async function streamAuthorizedSse(
  origin: string,
  input: { method: string; path: string; headers?: Record<string, string>; signal?: AbortSignal },
  accessToken: string,
  onChunk: (chunk: string) => void,
  fetchLike: SseFetchLike,
): Promise<void> {
  const response = await fetchLike(origin + input.path, {
    method: input.method,
    headers: { ...(input.headers ?? {}), authorization: `Bearer ${accessToken}`, accept: 'text/event-stream' },
    ...(input.signal === undefined ? {} : { signal: input.signal }),
  });
  if (!response.ok || !response.body) {
    const error = new Error(`workbench event stream failed with HTTP ${response.status}`);
    error.name = 'ApiError';
    (error as unknown as { status?: number }).status = response.status;
    throw error;
  }
  const reader = (response.body as ReadableStream<Uint8Array>).getReader();
  const decoder = new TextDecoder();
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    if (value !== undefined && value.length > 0) onChunk(decoder.decode(value, { stream: true }));
  }
  onChunk(decoder.decode());
}
```

3b. `apps/mobile/src/task-detail-view.ts`（完整内容）：

```ts
import type { TaskDetailView, TaskHandle } from '@weknora/mobile-core';

export interface TaskDetailViewState {
  view?: TaskDetailView;
  loading: boolean;
  error?: string;
}

export interface TaskDetailController {
  state(): TaskDetailViewState;
  subscribe(listener: (state: TaskDetailViewState) => void): () => void;
  refresh(): Promise<void>;
  whenSettled(): Promise<void>;
  dispose(): void;
}

/** 详情页控制器：hydrate 驱动首帧，updates 驱动增量，refresh 走显式 resync；dispose 摘除订阅并关闭句柄。 */
export function createTaskDetailController(handle: TaskHandle): TaskDetailController {
  let state: TaskDetailViewState = { loading: true };
  let disposed = false;
  const listeners = new Set<(state: TaskDetailViewState) => void>();
  const publish = (next: TaskDetailViewState): void => {
    state = next;
    for (const listener of [...listeners]) listener(state);
  };
  const messageOf = (failure: unknown): string => (failure instanceof Error ? failure.message : String(failure));
  const unsubscribe = handle.updates((view) => { if (!disposed) publish({ view, loading: false }); });
  const tail: Promise<void> = handle.hydrate().then(undefined, (failure: unknown) => {
    if (!disposed) publish({ view: undefined, loading: false, error: messageOf(failure) });
  });
  return {
    state: () => state,
    subscribe(listener) { listeners.add(listener); return () => { listeners.delete(listener); }; },
    refresh(): Promise<void> {
      publish({ ...state, loading: true, error: undefined });
      const attempt = handle.resync().then(undefined, (failure: unknown) => {
        if (!disposed) publish({ view: handle.view(), loading: false, error: messageOf(failure) });
      });
      return attempt;
    },
    whenSettled: () => tail,
    dispose() {
      if (disposed) return;
      disposed = true;
      unsubscribe();
      listeners.clear();
      handle.close('controller-disposed');
    },
  };
}
```

3c. `apps/mobile/src/screens/TaskDetailScreen.tsx`（完整内容）：

```tsx
import { useState } from 'react';
import { Button, ScrollView, Text, View } from 'react-native';
import type { TaskDetailView } from '@weknora/mobile-core';
import { timelineKindLabel } from '@weknora/mobile-core';

export interface TaskDetailScreenProps {
  view?: TaskDetailView;
  loading: boolean;
  error?: string;
  onRefresh(): void;
}

const CONNECTION_LABELS: Record<TaskDetailView['connection'], string> = { syncing: '同步中', live: '已连接', interrupted: '连接中断，可恢复', drained: '已同步' };
const LIFECYCLE_LABELS: Record<TaskDetailView['lifecycle'], string> = { active: '进行中', completed: '已完成', canceled: '已取消', archived: '已归档' };

/** 结果优先详情屏：状态卡 + 三层状态 + attention 横幅在前，时间线事实流在后；原始证据默认折叠、按需展开。 */
export function TaskDetailScreen({ view, loading, error, onRefresh }: TaskDetailScreenProps) {
  const [expanded, setExpanded] = useState<ReadonlySet<number>>(new Set());
  if (view === undefined) {
    return (
      <View>
        <Text>{loading ? '正在读取服务端快照…' : '无法读取该任务'}</Text>
        {error !== undefined && <Text>{error}</Text>}
      </View>
    );
  }
  const toggle = (seq: number): void => {
    setExpanded((current) => {
      const next = new Set(current);
      if (next.has(seq)) next.delete(seq);
      else next.add(seq);
      return next;
    });
  };
  return (
    <ScrollView>
      <View>
        <Text>{view.title === '' ? view.taskId : view.title}</Text>
        <Text>{CONNECTION_LABELS[view.connection]}{view.interruption !== undefined ? ` · ${view.interruption.reason}` : ''}</Text>
        <View>
          <Text>任务：{LIFECYCLE_LABELS[view.lifecycle]}</Text>
          <Text>运行：{view.runStatus}</Text>
          <Text>关注：{view.attention === 'required' ? '需要你处理' : '无需处理'}</Text>
          <Text>执行：{view.executionStatus} · 结算：{view.settlementStatus} · 已接收事件：{view.cursor}</Text>
        </View>
        {view.attention === 'required' && <Text>任务需要你的确认，请查看时间线中的审批条目。</Text>}
      </View>
      <Text>任务时间线</Text>
      {view.timeline.map((entry) => (
        <View key={entry.seq}>
          <Text>{entry.summary}</Text>
          <Text>{timelineKindLabel(entry.kind)} · {formatTime(entry.occurredAt)} · #{entry.seq}</Text>
          <Button title={expanded.has(entry.seq) ? '收起证据' : '展开证据'} onPress={() => toggle(entry.seq)} />
          {expanded.has(entry.seq) && (
            <Text>{Object.entries(entry.evidence.payload).map(([key, value]) => `${key}: ${safeText(value)}`).join('\n')}</Text>
          )}
        </View>
      ))}
      {view.duplicateSeqs.length > 0 && <Text>已忽略重复事件：{view.duplicateSeqs.join(', ')}</Text>}
      <Button title="重新同步快照" onPress={onRefresh} />
      {error !== undefined && <Text>{error}</Text>}
    </ScrollView>
  );
}

function formatTime(value: string): string {
  return value.replace('T', ' ').replace('Z', ' UTC');
}

/** 原始证据只按文本呈现键值；不执行、不猜测结构。 */
function safeText(value: unknown): string {
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean' || value === null) return String(value);
  try { return JSON.stringify(value) ?? String(value); } catch { return '[unserializable]'; }
}
```

3d. `apps/mobile/src/app/tasks/detail.tsx`（完整内容）：

```tsx
import { useEffect, useRef, useState } from 'react';
import { useLocalSearchParams } from 'expo-router';
import { activeTaskOffice } from '../../composition.ts';
import { createTaskDetailController, type TaskDetailController, type TaskDetailViewState } from '../../task-detail-view.ts';
import { TaskDetailScreen } from '../../screens/TaskDetailScreen.tsx';

/** /tasks/detail 挂载生命周期宿主：handle 在 effect 内创建、卸载即 dispose——与 /resources 同一模式。 */
export function TaskDetailRouteLifecycle({ taskId, runId }: { taskId: string; runId: string }) {
  const [state, setState] = useState<TaskDetailViewState>({ loading: true });
  const controllerRef = useRef<TaskDetailController | undefined>(undefined);
  useEffect(() => {
    let controller: TaskDetailController | undefined;
    try {
      const handle = activeTaskOffice()?.open({ taskId, runId });
      controller = handle === undefined ? undefined : createTaskDetailController(handle);
    } catch {
      controller = undefined;
    }
    if (controller === undefined) {
      setState({ loading: false, error: '请先登录并激活空间，再打开任务详情。' });
      return;
    }
    controllerRef.current = controller;
    setState(controller.state());
    const unsubscribe = controller.subscribe(setState);
    return () => {
      unsubscribe();
      controller?.dispose();
      controllerRef.current = undefined;
    };
  }, [taskId, runId]);
  return <TaskDetailScreen view={state.view} loading={state.loading} error={state.error} onRefresh={() => { void controllerRef.current?.refresh(); }} />;
}

/** Expo Router 文件路由：/tasks/detail?taskId=..&runId=..。只消费 Task Office Interface。 */
export default function TaskDetailRoute() {
  const params = useLocalSearchParams<{ taskId?: string; runId?: string }>();
  return <TaskDetailRouteLifecycle taskId={String(params.taskId ?? '')} runId={String(params.runId ?? '')} />;
}
```

3e. `apps/mobile/src/composition.ts` 修改（三处，位置明确）：

- 头部 import 追加：`import { streamAuthorizedSse, type SseFetchLike } from './adapters/sse-stream.ts';`
- `createNativeMobileRuntime` 的 ports 内（`authorizedTransport` 之后）追加：

```ts
    authorizedStream(origin) {
      // 惰性解析 expo/fetch（Node 测试环境无此模块；解析失败即无流通道，fail closed）
      let streamFetch: SseFetchLike | undefined;
      try { streamFetch = (require('expo/fetch') as { fetch: SseFetchLike }).fetch; } catch { streamFetch = undefined; }
      return streamFetch === undefined ? undefined : (input, accessToken, onChunk) => streamAuthorizedSse(origin, input, accessToken, onChunk, streamFetch);
    },
```

- `taskOfficeFor` 内 remote 构造改为（同一 remote 同时作为 `backend` 与 `detail`——remote 结构上实现两个端口，漏传 `detail` 会使 `open()` 永远 fail closed）：

```ts
    const remote = createTaskOfficeRemote({ origin, request: (input) => activeRuntime.authorizedRequest(input), stream: (input, onChunk) => activeRuntime.authorizedEventStream(input, onChunk) });
    office = createTaskOffice({
      backend: remote,
      detail: remote,
      lease: () => activeRuntime.scopeLease(),
    });
```

- `MobileTasks` 改为（替换原函数体）：

```ts
export function MobileTasks({ onOpenTask }: { onOpenTask?: (taskId: string; runId: string) => void } = {}) {
  const activeRuntime = runtime();
  const snapshot = useSyncExternalStore(activeRuntime.subscribe, activeRuntime.snapshot, activeRuntime.snapshot);
  if (snapshot.surface !== 'authorized' || !snapshot.deployment || !snapshot.identity?.userId) return null;
  return createElement(TasksScreen, {
    key: snapshot.identity.activeTenantId,
    taskOffice: taskOfficeFor(activeRuntime, snapshot.deployment.origin),
    ...(onOpenTask === undefined ? {} : { onOpenTask: (card: { taskId: string; runId: string }) => onOpenTask(card.taskId, card.runId) }),
  });
}
```

- 其后追加：

```ts
/** 详情路由经此取当前授权 scope 的 Task Office（无授权面返回 undefined）。 */
export function activeTaskOffice(): TaskOffice | undefined {
  const activeRuntime = runtime();
  const snapshot = activeRuntime.snapshot();
  if (snapshot.surface !== 'authorized' || !snapshot.deployment || !snapshot.identity?.userId) return undefined;
  return taskOfficeFor(activeRuntime, snapshot.deployment.origin);
}
```

3f. `apps/mobile/src/screens/TasksScreen.tsx`：`TasksScreenProps` 追加 `onOpenTask?: (card: TaskCard) => void;`；组件解构加入 `onOpenTask`；`items.map` 的每个行卡片内追加：

```tsx
  {onOpenTask !== undefined && <Button title="详情" onPress={() => onOpenTask(item)} />}
```

3g. `apps/mobile/src/app/tasks.tsx` 整体替换为：

```tsx
import { router } from 'expo-router';
import { MobileTasks } from '../composition.ts';

/** /tasks 一级入口；Surface 仍由 Runtime 快照裁决，行点击进入 /tasks/detail。 */
export default function Tasks() {
  return <MobileTasks onOpenTask={(taskId, runId) => router.push({ pathname: '/tasks/detail', params: { taskId, runId } })} />;
}
```

3h. `apps/mobile/src/app-smoke.test.tsx` 共享文件改动（两处，位置明确）：

其一，`NATIVE_MODULE_STUBS`（`:24` 附近）两条既有条目扩展具名导出（新路由/新屏依赖 `useLocalSearchParams` 与 `ScrollView`；当前 tsx 按 CJS 处理缺失具名导出不抛错，但补齐后不再依赖该脆弱行为，包改 ESM 亦安全）：

```ts
  'expo-router': "module.exports = { Stack: function Stack() { return null; }, router: { replace() {}, push() {} }, useLocalSearchParams() { return {}; } }",
  'react-native': "module.exports = { View: 'View', Text: 'Text', TextInput: 'TextInput', Button: 'Button', ScrollView: 'ScrollView' }",
```

其二，文件末尾追加一个测试块（沿用 `:317-326` 的动态 `node:fs` 源级断言惯例；**detail 接线守卫是本块核心**——Task 6 的 `open()` 在 `ports.detail` 缺失时 fail closed，生产接线漏传 `detail` 会使 `/tasks/detail` 永远报错且本地测试无法发现，故以源级断言把该缺口类别钉进冒烟）：

```tsx
test('the task detail route and screen consume the task office interface with evidence collapsed by default', async () => {
  const route = await import('./app/tasks/detail.tsx');
  assert.equal(typeof route.default, 'function', 'src/app/tasks/detail.tsx must default-export the detail route');
  assert.equal(typeof route.TaskDetailRouteLifecycle, 'function');
  const screen = await import('./screens/TaskDetailScreen.tsx');
  assert.equal(typeof screen.TaskDetailScreen, 'function');
  const view = {
    taskId: 'task-1', runId: 'run-1', title: '报告', lifecycle: 'active', runStatus: 'running', attention: 'none',
    executionStatus: 'running', settlementStatus: 'pending', revision: 1, cursor: 2, incomplete: false, connection: 'live',
    timeline: [{ seq: 2, occurredAt: '2026-09-23T00:00:00Z', kind: 'tool_activity', type: 'tool.started', summary: '正在使用工具', evidence: { payload: { secretArgument: 'raw' } } }],
    duplicateSeqs: [],
  };
  const element = screen.TaskDetailScreen({ view, loading: false, onRefresh: () => {} });
  const json = JSON.stringify(element);
  assert.ok(json.includes('任务时间线'), 'the timeline section renders');
  assert.ok(!json.includes('secretArgument'), 'raw evidence stays collapsed by default');
});

test('the composition wires the task office detail port and detail files stay off wire adapters', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const composition = readFileSync(join(here, 'composition.ts'), 'utf8');
  assert.match(composition, /detail:\s*remote/, 'taskOfficeFor must pass the remote as the detail port; open() fails closed without it (T05)');
  for (const relative of ['screens/TaskDetailScreen.tsx', 'task-detail-view.ts', 'app/tasks/detail.tsx']) {
    const source = readFileSync(join(here, relative), 'utf8');
    assert.equal(/@weknora\/(api-client|contracts)/.test(source), false, `${relative} must consume the Task Office Interface only`);
  }
});
```

- [ ] **Step 4: 运行确认通过**

Run: `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`
Expected: PASS（含既有 app-smoke 全部用例；typecheck 证明 Task 8 remote 与 mobile-core `TaskDetailBackendPort` 结构逐字一致）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/adapters/sse-stream.ts apps/mobile/src/adapters/sse-stream.test.ts apps/mobile/src/task-detail-view.ts apps/mobile/src/task-detail-view.test.ts apps/mobile/src/screens/TaskDetailScreen.tsx apps/mobile/src/app/tasks/detail.tsx apps/mobile/src/composition.ts apps/mobile/src/screens/TasksScreen.tsx apps/mobile/src/app/tasks.tsx apps/mobile/src/app-smoke.test.tsx
git commit -m "feat(mobile): task detail screen with collapsed evidence and authorized sse wiring (T05)"
```

---

### Task 10: apps/mobile——opt-in 真实 HTTP 详情集成证据（AC3）

**Files:**
- Create: `apps/mobile/src/task-detail-integration-smoke.ts`
- Create: `packages/api-client/src/mobile/task-office-detail.integration.test.ts`

**Interfaces:**
- Consumes: Task 6/7/8/9 产出（`createTaskOffice`+`detail` port、`authorizedEventStream`、`createTaskOfficeRemote({ origin, request, stream })`、`streamAuthorizedSse`）；既有 `taskOfficeIntegrationConfig` 环境变量语义（`WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD`，`task-office-integration-smoke.ts:22`）。
- Produces: `TaskDetailIntegrationConfig`、`TaskDetailIntegrationEvidence`（`{ deploymentOrigin, opened: 'hydrated'|'no-tasks'|'failed', connection?, lifecycle?, runStatus?, attention?, timelineEntries?, cursor?, resync: 'resynced'|'failed'|'skipped', commandTimestamp }`——只含脱敏事实，绝不含 token/凭据/业务内容）、`taskDetailIntegrationConfig(env)`、`runTaskDetailIntegration(config)`、`emitTaskDetailIntegrationEvidence(evidence, emit)`。

- [ ] **Step 1: 写失败测试（opt-in 集成 + 本地必跑的 config 校验）**

`packages/api-client/src/mobile/task-office-detail.integration.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { emitTaskDetailIntegrationEvidence, runTaskDetailIntegration, taskDetailIntegrationConfig } from '../../../../apps/mobile/src/task-detail-integration-smoke.ts';

/**
 * Opt-in real HTTP check（AC3 端到端）。无回退凭据、无 mock：
 *
 * WEKNORA_MOBILE_TEST_DEPLOYMENT_URL=https://deployment.example \
 * WEKNORA_MOBILE_TEST_EMAIL=mobile-test@example.test \
 * WEKNORA_MOBILE_TEST_PASSWORD=<short-lived-secret> \
 * pnpm exec tsx --test packages/api-client/src/mobile/task-office-detail.integration.test.ts
 */
test('real HTTP task detail hydrates, streams and resyncs through the task office', async (t) => {
  const config = taskDetailIntegrationConfig(process.env);
  if (!config.enabled) {
    if (config.disposition === 'skip') t.skip(`TASK_DETAIL_HTTP_SKIPPED: ${config.reason}`);
    else assert.fail(`TASK_DETAIL_HTTP_INVALID: ${config.reason}`);
    return;
  }
  const evidence = await runTaskDetailIntegration(config);
  emitTaskDetailIntegrationEvidence(evidence, (record) => t.diagnostic(record));
  assert.equal(evidence.opened, 'hydrated', 'opening a real task must hydrate end to end');
  assert.equal(evidence.connection !== undefined && evidence.connection !== 'interrupted', true, 'a healthy deployment ends live or drained');
  assert.equal(typeof evidence.timelineEntries, 'number');
  assert.equal(evidence.resync, 'resynced', 'the explicit resync path must work against the real backend');
  assert.doesNotMatch(JSON.stringify(evidence), /short-lived-secret|password|token|email/i);
});

test('a path-bearing deployment URL is invalid, not skippable', () => {
  const probeSecret = ['short-lived', 'secret'].join('-');
  const config = taskDetailIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://deployment.example/api/v1',
    WEKNORA_MOBILE_TEST_EMAIL: 'mobile-test@example.test',
    WEKNORA_MOBILE_TEST_PASSWORD: probeSecret,
  });
  assert.equal(config.enabled, false);
  assert.equal(config.disposition, 'invalid');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `npx tsx --test packages/api-client/src/mobile/task-office-detail.integration.test.ts`
Expected: FAIL——`Cannot find module .../task-detail-integration-smoke.ts`。

- [ ] **Step 3: 最小实现**

`apps/mobile/src/task-detail-integration-smoke.ts`（完整内容）：

```ts
import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createInMemoryCredentialStore, createMobileRuntime, createTaskOffice, type TaskOffice } from '@weknora/mobile-core';
import { streamAuthorizedSse } from './adapters/sse-stream.ts';

export type TaskDetailIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface TaskDetailIntegrationEvidence {
  deploymentOrigin: string;
  opened: 'hydrated' | 'no-tasks' | 'failed';
  connection?: 'syncing' | 'live' | 'interrupted' | 'drained';
  lifecycle?: string;
  runStatus?: string;
  attention?: string;
  timelineEntries?: number;
  cursor?: number;
  resync: 'resynced' | 'failed' | 'skipped';
  commandTimestamp: string;
}

/** 与 T04 taskOfficeIntegrationConfig 相同的 opt-in 语义（自包含，不跨计划 import）。 */
export function taskDetailIntegrationConfig(env: Record<string, string | undefined>): TaskDetailIntegrationConfig {
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

const settle = async (rounds = 10): Promise<void> => {
  for (let index = 0; index < rounds; index += 1) await new Promise<void>((resolve) => setTimeout(resolve, 25));
};

/** 真实 JSON transport + 授权 SSE 通道 + 具体 Remote Adapter + Task Office 详情编排。账号无任务时如实记 'no-tasks'。 */
export async function runTaskDetailIntegration(config: Extract<TaskDetailIntegrationConfig, { enabled: true }>): Promise<TaskDetailIntegrationEvidence> {
  const evidence: TaskDetailIntegrationEvidence = { deploymentOrigin: config.deploymentOrigin, opened: 'failed', resync: 'skipped', commandTimestamp: new Date().toISOString() };
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
    authorizedStream(origin) {
      return (input, accessToken, onChunk) => streamAuthorizedSse(origin, input, accessToken, onChunk, fetcher);
    },
  });
  const snapshot = await runtime.signIn({ deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' }, email: config.email, password: config.password });
  if (snapshot.surface !== 'authorized' || !snapshot.deployment) return evidence;

  // 同一 remote 同时作为 backend 与 detail：漏传 detail 会让 open() fail closed，
  // 具备真实环境时 AC3 用例将以 opened:'failed' 如实暴露（本处由 Task 9 的
  // app-smoke 源级断言 `detail:\s*remote` 同类防护）。
  const remote = createTaskOfficeRemote({
    origin: config.deploymentOrigin,
    request: (input) => runtime.authorizedRequest(input),
    stream: (input, onChunk) => runtime.authorizedEventStream(input, onChunk),
  });
  const office: TaskOffice = createTaskOffice({ backend: remote, detail: remote, lease: () => runtime.scopeLease() });
  const page = await office.tasks({});
  if (page.items.length === 0) { evidence.opened = 'no-tasks'; return evidence; }
  const target = page.items[0]!;
  const handle = office.open({ taskId: target.taskId, runId: target.runId });
  const view = await handle.hydrate();
  await settle();
  evidence.opened = 'hydrated';
  const settled = handle.view() ?? view;
  evidence.connection = settled.connection;
  evidence.lifecycle = settled.lifecycle;
  evidence.runStatus = settled.runStatus;
  evidence.attention = settled.attention;
  evidence.timelineEntries = settled.timeline.length;
  evidence.cursor = settled.cursor;
  try {
    const resynced = await handle.resync();
    evidence.resync = resynced.connection === 'interrupted' ? 'failed' : 'resynced';
  } catch {
    evidence.resync = 'failed';
  }
  handle.close('integration-complete');
  runtime.dispose();
  return evidence;
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitTaskDetailIntegrationEvidence(evidence: TaskDetailIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
```

- [ ] **Step 4: 运行确认通过（本地 = config 校验 + skip；有环境 = 真实 HTTP）**

Run: `npx tsx --test packages/api-client/src/mobile/task-office-detail.integration.test.ts`
Expected: PASS——本地无环境变量时第 1 用例 `skip`（输出 `TASK_DETAIL_HTTP_SKIPPED`，不伪造通过）、第 2 用例 PASS。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/task-detail-integration-smoke.ts packages/api-client/src/mobile/task-office-detail.integration.test.ts
git commit -m "test(mobile): opt-in real http task detail hydration, stream and resync evidence (T05 AC3)"
```

---

### Task 11: miniprogram——assembly SSE 证据修复

**Files:**
- Modify: `apps/miniprogram/tests/assembly.test.mjs`（仅两处 snapshot fixture）

**Interfaces:**
- Consumes: 既有 `execDto`/`execEvent` fixture（`:22`–`:23`）与 MX-003 冻结契约（`packages/contracts/src/mobile/execution.ts:184` 要求 `incomplete: boolean` 与 `confirmed_watermark`）。
- Produces: `assembly.test.mjs` 11/11 绿——恢复「watchExecution 安装 snapshot 后从 watermark 追加流事件」与「wire paths 与 Go 路由表一致」两条端到端证据（T05 调查所列缺口）。

- [ ] **Step 1: 实跑确认当前 RED（预存失败）**

Run: `cd apps/miniprogram && node --experimental-strip-types --test tests/assembly.test.mjs`
Expected: FAIL——`not ok 7 - assembly: watchExecution installs the snapshot then appends streamed events from the watermark` 与 `not ok 10 - assembly: wire paths used by the miniprogram match the Go route table`，错误 `incomplete: expected a boolean`（本计划作者已在当前 worktree 实跑复现：9 pass / 2 fail）。

- [ ] **Step 2: 修复 fixture（GREEN）**

`apps/miniprogram/tests/assembly.test.mjs` 两处修改（按内容定位，均在测试的路由表内）：

1. watchExecution 用例内：
```js
    'GET /api/v1/workbench/executions/run-1/snapshot': call => stub.succeed(call, { data: { success: true, data: { execution: execDto(5), watermark: 5, incomplete: false, confirmed_watermark: 5, events: [execEvent(5)] } } }),
```
2. wire-paths 用例内：
```js
    'GET /api/v1/workbench/executions/run-1/snapshot': call => stub.succeed(call, { data: { success: true, data: { execution: execDto(5), watermark: 5, incomplete: false, confirmed_watermark: 5, events: [] } } }),
```

（只补 `incomplete: false` 与 `confirmed_watermark: 5` 两个字段，对齐 MX-003 冻结契约；不改断言语义。）

- [ ] **Step 3: 运行确认通过**

Run: `cd apps/miniprogram && node --experimental-strip-types --test tests/assembly.test.mjs tests/core.test.mjs`
Expected: PASS——assembly 11/11、core 9/9。

- [ ] **Step 4: Commit**

```bash
git add apps/miniprogram/tests/assembly.test.mjs
git commit -m "test(miniprogram): align assembly snapshot fixtures with the frozen MX-003 contract (T05)"
```

---

## 计划级验证命令（worktree 根执行）

```bash
go test ./internal/application/repository/ -run 'TestReadTaskFactsForRun|TestWorkbenchList' -count=1 && \
go test ./internal/handler/session/ -run 'TestGetWorkbenchSnapshot|TestWorkbenchRead|TestStreamWorkbench|TestNormalizeWorkbench|TestIngestWorkbenchSourceEvent' -count=1 && \
go build ./internal/... ./cmd/... && \
npx tsx --test packages/contracts/test/mobile-execution.test.ts packages/contracts/test/mobile-read-models.test.ts packages/contracts/test/mobile-interactions.test.ts && \
npx tsx --test 'packages/mobile-core/src/task-office/*.test.ts' packages/mobile-core/src/runtime/mobile-runtime.test.ts && \
npx tsx --test packages/api-client/src/mobile/task-office.test.ts packages/api-client/src/mobile/executions.test.ts packages/api-client/src/mobile/task-office-detail.integration.test.ts && \
pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck && \
cd apps/miniprogram && node --experimental-strip-types --test tests/assembly.test.mjs tests/core.test.mjs
```

（作者基线实跑：`go test ./internal/handler/session/ -run 'TestStreamWorkbench|TestNormalizeWorkbench'` ok、`go test ./internal/application/repository/ -run 'TestWorkbenchListArchiveFilterAndAttentionProjection'` ok、`npx tsx --test packages/mobile-core/src/task-office/task-office.test.ts` 7 pass、`npx tsx --test packages/contracts/test/mobile-execution.test.ts packages/api-client/src/mobile/task-office.test.ts packages/api-client/src/mobile/executions.test.ts` 16 pass、`pnpm --filter @weknora/mobile test` 全绿、`pnpm --filter @weknora/mobile typecheck` exit 0、miniprogram core 9/9、assembly 9 pass 2 fail——该 2 fail 即 Task 11 修复对象。）

## Consumes / Produces 汇总（供后续计划 #36–#38/#40 引用）

**Produces（本计划新增的对外接口）：**
- Go wire：`ExecutionSnapshot.Task *TaskSnapshotFacts`（`internal/modules/workbench/contracts.go`）；`WorkbenchListStore.ReadTaskFactsForRun`；`WorkbenchReadHandler.WithTaskFacts`。
- contracts：`SnapshotTaskFacts`/`SnapshotAttentionState`（`packages/contracts/src/mobile/execution.ts`）。
- mobile-core：`TaskOffice.open({ taskId, runId }): TaskHandle`；`TaskHandle{hydrate/view/updates/resync/close}`；`TaskDetailView`（三层状态 + 规范 Timeline + connection/interruption/duplicateSeqs）；`TaskDetailBackendPort{detail, stream}`；`TaskProjectionStore`；错误码 `TASK_OFFICE_DETAIL_UNAVAILABLE`/`TASK_OFFICE_DETAIL_CLOSED`；`MobileRuntime.authorizedEventStream(input, onChunk)` 与 `MobileRuntimePorts.authorizedStream?`；`task-timeline.ts` 全部纯投影（`taskLifecycleOf`/`terminalRunStatusOf`/`mergeEventHistory`/`projectTimeline`/`timelineKindLabel`/`isTerminalRunStatus`）；`createScenarioTaskDetailBackend`/`createScriptedTaskStream`/`createInMemoryTaskProjectionStore`。
- api-client：`createTaskOfficeRemote` 的 `detail`/`stream` 与 `stream` 选项；`TASK_STREAM_CURSOR_EXPIRED` 错误码契约。
- apps/mobile：`streamAuthorizedSse` adapter、`createTaskDetailController`、`TaskDetailScreen`、`/tasks/detail` 路由、`activeTaskOffice()`、`MobileTasks({ onOpenTask })`、`task-detail-integration-smoke.ts` 证据契约。

**明确不在本计划（后续 Issue）：** `act(TaskIntent)`（steer/queue-next/stop/decision/share——#37/#38）、`start(goal)` 与 request_id 对账（#36）、task 级「当前 run」服务端解析（#36 新增运行后）、Task Material（产物内容/Diff/终端——后续）、原生加密持久化 Scoped Vault Adapter 与跨进程重启持久化（#40；当前 composition 用 office 内共享 in-memory store，Interface 级重启场景证据成立，真机重启持久化证据属 #40/真机验收门槛）。
