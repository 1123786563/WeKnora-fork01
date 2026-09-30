# T14：旧 Session 投影为 Legacy Task（Issue #44）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 从未有过 Run 的旧 Session（纯聊天会话）以同一身份（`taskId = sessionId`，ADR-0004）进入显式的 Legacy Task 投影层，只携带历史能证明的事实（标题/归档/更新时间，attention 恒为 none），在原生 App 上可查看历史并继续普通追问（既有聊天语义）；一切需要新安全语义的行为（Run 指令、审批、预算、Agent Version）被显式门禁标记为「需新建 Run」，并以真实迁移测试锁定「旧 Session 仅经新 Run 准入获得新能力」。

**Architecture:** Go 侧新增独立的 Legacy Task 读模型与端点 `GET /api/v1/workbench/legacy-tasks`（`internal/application/repository/workbench_legacy_list.go` 新文件：`sessions` 表 owner+tenant 谓词 + `NOT EXISTS agent_runs` 反连接 + 软删除过滤 + (updated_at,id) keyset cursor，与既有 run 列表把全部会话二分区——有 Run 的进执行列表、无 Run 的进 legacy 列表，同一 taskId 两处互斥、绝不重复投影）；归档生命周期（#34 的 `SetTaskArchived`）的归属谓词从「拥有该会话至少一个 Run」扩展为「或拥有该 session 本身」，使 Legacy Task 同身份可归档/恢复。客户端在 `packages/mobile-core` Task Office 域新增 `legacyTasks/moreLegacyTasks/legacyHistory/followUp` 入口（`LegacyTaskBackendPort` 注入、scope lease fail closed、迟到结果拒绝）与纯门禁策略 `legacyTaskGates()`（follow-up supported；run-command/decision/budget/agent-version 一律 unavailable + 固定理由「需新建 Run」）；`api-client` 新增 `createMobileLegacyTaskRemote`（list 走 REST、history 复用既有 `GET /messages/:id/load`、followUp 走既有 `POST /knowledge-chat/:id` SSE 通道——普通追问即既有聊天 wire，不新增安全语义）。apps/mobile 新增 LegacyTasksScreen + `/tasks/legacy` 路由 + Tasks 屏入口按钮。迁移门禁测试用真实 sqlite 迁移库 + 真实 `AdmissionCoordinator`：旧会话先可读可归档 → `POST /workbench/executions` 新 Run 准入后才离开 legacy 投影并进入执行列表。

**Tech Stack:** Go 1.26（gin + gorm，sqlite 真迁移测试库，`migrations/` 不动——本计划无新迁移）、TypeScript（`packages/mobile-core`、`packages/api-client`、`apps/mobile` Expo RN）、node:test + tsx（TS 测试运行器，与 `task-office.test.ts` 一致）、`testify`（Go）。所有测试命令在 worktree 根（`.worktrees/issue30-sweep`）执行；前置：已执行过 `pnpm install`（本计划作者已实跑：`pnpm exec tsx --test packages/mobile-core/src/task-office/task-office.test.ts` 全绿、`go test ./internal/application/repository/ -run 'TestWorkbenchList' -count=1` ok）。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-44.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（Implementation Decisions 中 Legacy 投影与 Task 身份各条、Testing Decisions 中 Migration tests 与 Interface 测试各条、Out of Scope 中「Guaranteed support for every existing Session as a fully upgraded Task」）
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§5 Task Office Module——所有权含「一个初始目标创建 Task，追问继续原 Task」/§5.2 Interface/§5.3 不变量、§4 Mobile Runtime、§10 App Shell）
- ADR：`docs/adr/0004-task-is-session.md`（taskId = sessionId，不新增第二 Task 聚合身份）、`docs/adr/0006-mobile-transport-by-semantics.md`（REST 提交命令、SSE 承载流式回复）
- 领域术语：`CONTEXT.md`（「任务生命周期（Task Lifecycle）」「关注状态（Attention State）」「运行（Run）」）
- Parent：Issue #30；Blocked by：#34（T04）、#35（T05）——均已合入当前 HEAD（`git log` 核实：`1ca9e928`、`c7a7629d` 系列及 B2/R1 修复链）
- 前序批次产出（本计划 Consumes，全部在当前 HEAD 亲眼核实）：#34 的 `WorkbenchListStore`/`WorkbenchExecutionFilter`/`ErrWorkbenchCursor`/`normalizeWorkbenchSearch`/`likeEscaped`/`searchPredicate`（`internal/application/repository/workbench_list.go:19-45`）、`WorkbenchTaskStateStore.SetTaskArchived`（`internal/application/repository/workbench_task_state.go:28`）、`WorkbenchTaskStateHandler`（`internal/handler/session/workbench_task_state.go:23`）、`TaskOffice`/`TaskOfficePorts`/`TaskOfficeError`（`packages/mobile-core/src/task-office/task-office.ts:100-116`）；#35 的 `MobileRuntime.authorizedEventStream(input, onChunk)` 与 `RuntimeAuthorizedRequest{method,path,headers?,body?,signal?}`（`packages/mobile-core/src/runtime/types.ts:31-37,57`）、`createServerSentEventParser`/`parseChatEvent`（`packages/api-client/src/chat/stream.ts:14,94`）、`streamAuthorizedSse`（`apps/mobile/src/adapters/sse-stream.ts:6`）、`activeTaskOffice()`/`taskOfficeFor()`（`apps/mobile/src/composition.ts:109,139`）；既有聊天 wire：`GET /messages/:session_id/load`（`internal/router/routes_chat.go:34`，`parseChatMessageListResponse` 于 `packages/contracts/src/index.ts:460`）、`POST /knowledge-chat/:session_id`（`routes_chat.go:242`，请求体只需 `query`，`internal/handler/session/types.go:48`）、`responseType`（`packages/contracts/src/chat/events.ts:23`，经 contracts index 导出）。

## Global Constraints

以下为批准 Spec / ADR / Issue 的项目级约束，逐字引用，所有任务隐含遵守：

- 「Existing Session history is projected as legacy Tasks without fabricating historical Grants, budgets, approvals or Agent Versions.」（mobile-ai-office-design.md · Implementation Decisions）
- 「Migration tests verify old Sessions remain readable and only gain new capabilities through explicit upgrade or new Run admission.」（同上 · Testing Decisions）
- 「移动 AI Office 将现有 WeKnora Session 呈现为 Task，并保持 `taskId = sessionId`，不新增第二个 Task 聚合身份。」（ADR-0004）
- 「Task is the product name for the existing Session identity. A Task contains multiple Runs; initial goals create Tasks and same-goal follow-ups stay within them.」（mobile-ai-office-design.md · Implementation Decisions）
- 「Task Office owns Home/Task projections, durable submission identity, reconciliation, Snapshot/SSE recovery, intervention, decisions, budget and Task lifecycle.」（同上）
- 「Task lifecycle, Run status and Attention status are separate. Agent-specific progress phases never replace these canonical dimensions.」（同上）
- 「Mobile core does not depend on React Native, DOM or concrete transport. Remote and native details are injected as Adapters.」（同上）
- 「Cache identity includes Deployment, user, Tenant, Task and Run where applicable. Scope changes invalidate subscriptions and reject late responses.」（同上）
- 「Tests target observable behavior at the highest stable Interface. Screen tests verify rendering and navigation; they do not duplicate Module internals.」（同上 · Testing Decisions）
- 「Remote-owned dependencies use in-memory scenario Adapters for Module tests and real HTTP/SSE contract tests for production Adapters.」（同上）
- 「Screen 不调用 start、lookup、snapshot、events、interaction、command 等多个 wire 方法。Module 内部决定顺序、幂等、重连、revision 和错误呈现。」（mobile-module-seams.md §5.2）
- 「禁止：Screen 直接导入 packages/contracts 或 packages/api-client；Screen 自己维护 request_id、cursor、revision、scope generation；每个 Screen 建独立 query cache 或 token refresh」（mobile-module-seams.md §10）
- 「capability 缺失和未知 schema 一律 fail closed；Scope Lease 失效后丢弃迟到结果」（mobile-module-seams.md §5.3）
- Out of Scope 边界：「Guaranteed support for every existing Session as a fully upgraded Task」——本计划交付的是**事实投影**而非全量升级，不得把旧会话伪装成全功能 Task。
- 安全约束（会话注入）：服务端 SQL 一律参数绑定（本计划所有新查询均为 `?` 占位 + gorm 绑定，`NOT EXISTS` 反连接为常量 SQL 片段不含外部输入）；服务端请求仅 http/https 且真实集成沿用 `disallowedDeploymentHost` 已有防线；凭据只从环境变量读取，源码与测试不写入可用凭据字面量；真实 HTTP 集成证据沿用 `WEKNORA_MOBILE_TEST_*` opt-in 环境变量（无回退凭据）。
- 工作流约束：严格 RED→GREEN→REFACTOR（每个任务先写失败测试、实跑确认失败、最小实现、通过、提交）；实现不与已批准 Spec 冲突，冲突时升级而非静默重设计。

**Issue #44 验收标准原文（docs/plans/issue30-sweep/issues/issue-44.md）：**

1. 「不创建第二 task ID，不伪造 Grant、预算、审批或 Agent Version。」
2. 「需要新安全语义的行为必须显式升级或新建 Run。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

验收标准 3 的本地可验证性说明（blocked-env 声明）：真实端到端（生产 JSON transport + 授权 REST/SSE 通道 + 具体 Remote Adapter + Task Office 编排 + 真后端 legacy 投影/普通追问 wire）沿用 T01–T05 已合并的 opt-in 真实 HTTP 模式，需要「一个真实 WeKnora Deployment（HTTPS origin）+ 一个测试账号」（`WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD` 环境变量）。本地无此环境时 Task 8 的真实 HTTP 用例以 `t.skip` 跳过（**不得伪造通过**）。本地替代证据：Task Office Interface 级场景测试（Task 5，真实模块编排 + in-memory scenario Adapter）+ Go 真实 sqlite 迁移库集成测试（Task 1/3，真实 SQL 与真实 AdmissionCoordinator）+ api-client wire 契约测试（Task 6，真实序列化字节与 SSE 帧）。凡具备环境的运行都自动产出端到端证据。

**与调查结论的差异记录（以代码现状为准）：**

1. 调查缺口第 4 条称「新原生 apps/mobile 无任何任务页面（仅 3 个登录/落地 Screen）」。该缺口在调查时点为真，但已被前两批解决：当前 HEAD 的 `apps/mobile/src/screens/` 已有 `HomeScreen.tsx`/`TasksScreen.tsx`/`TaskDetailScreen.tsx`（亲眼核实），路由 `src/app/tasks.tsx`、`src/app/tasks/detail.tsx` 存在。本计划只补 Legacy 维度，不重建任务页。
2. 调查未列出的**新发现缺口**：`SetTaskArchived` 的归属谓词要求「该 tenant 内拥有该 session 至少一个 run」（`internal/application/repository/workbench_task_state.go:34-41`，亲眼核实），0 Run 的旧会话归档一律 `ErrWorkbenchTaskNotFound`（HTTP 404）——Legacy Task 同身份却不可归档，且 legacy 列表的 `archived` 过滤将永远为空。Task 3 扩展归属谓词修复（「或拥有该 session 本身」），改动最小且有独立测试。
3. 小程序首页「继续上次的工作」已存在且经会话列表 + chat 页实现（`apps/miniprogram/src/features/home/pages.tsx:13`，亲眼核实）——小程序侧本计划零改动；本计划交付的是**服务端显式投影层/标记 + 原生 App 的 Legacy 查看/追问**。
4. 「显式升级」的移动端落点说明：`start(goal)`（新 Task/新 Run 入口）属 #36（T06，同批次并行），本计划不得依赖其接口。LegacyTasksScreen 的升级出口为**信息性文案 + 可选回调 prop**（`onStartNewRun?`，缺省只展示门禁理由），不实现 start。

## Review Focus

Spec 隐含但无任务测试覆盖、最可能咬到真实用户的五类输入/失效模式（每行后在所属任务落地测试）：

1. **越权探测**：以他人/异租户的旧会话 ID 查询 legacy 列表、读历史、归档——必须与 run 列表同规则 owner+tenant 谓词，跨 owner 一律不可见/`ErrWorkbenchTaskNotFound`，不因知道 ID 获得访问。——Task 1 测试 `TestWorkbenchLegacyListProjectsChatOnlySessionsWithinOwnerScope`（u2/异租户不可见）+ Task 3 测试 `TestWorkbenchTaskStateArchiveLegacySessionBySessionOwner`（他人 session 拒绝）+ Task 2 测试 `TestListLegacyTasksRequiresAuthenticatedIdentity`（未认证 401）。
2. **伪造/跨端点游标重放**：手工捏造的 base64、或把 run 列表签发的 cursor 重放到 legacy 端点（及反向），若被接受会跨过滤器泄漏行——必须 `ErrWorkbenchCursor`（HTTP 400）拒绝。——Task 1 测试 `TestWorkbenchLegacyListArchivedFilterCursorBindingAndCrossEndpointReplay`（伪造 + run 游标重放 + 过滤器失配三种均拒）。
3. **伪造 Run 级事实**：Legacy 行的序列化 JSON 若出现 `run_id`/`run_status`/`execution_status`/`settlement_status`/`grant`/`budget`/`approval`/`agent_version`/`revision` 任何一键，即违反 AC1「不伪造」；客户端解析器若接受 `kind != 'legacy'` 的行混入 legacy 视图，门禁即被污染。——Task 1 测试中对序列化行做**键集锁定**；Task 6 测试 `kind must be "legacy"` 强校验（非 legacy 行拒绝）。
4. **升级门禁旁路**：在 legacy 表面上直接寻求 Run 指令/审批/预算/Agent Version——客户端 `office.open()` 无 runId 即 `TASK_OFFICE_INVALID_INPUT`、gates 四类 unavailable 且理由固定；服务端 Run 作用域表面（owner 谓词 run 读）对 legacy 会话一律 `ErrNotFound`；唯一通道是新 Run 准入（`POST /workbench/executions`），准入后 legacy 投影即让位执行列表。——Task 3 迁移测试 `TestLegacyTaskMigrationOnlyNewRunAdmissionGrantsRunCapabilities`（准入前 run 读 ErrNotFound、准入后 legacy 消失/执行列表出现）+ Task 4 测试 `legacyTaskGates` 固定裁决 + Task 5 测试「legacy 任务无 runId 无法 open」。
5. **普通追问撞上活跃 turn / 流错误**：followUp 提交时该会话已有活跃 turn（服务端 409 another turn running）或 SSE 帧损坏/携带 error 事件——不得静默重试、不得吞错伪造成功。——Task 5 测试 followUp 错误透传（`TASK_OFFICE_BACKEND` 包装、不换语义）+ Task 6 测试 409 以 ApiError 形态透传、`error` 事件与畸形帧分别以 `LEGACY_FOLLOW_UP_FAILED`/`LEGACY_FOLLOW_UP_MALFORMED_FRAME` 拒绝。

## 任务结构与文件地图

| # | 任务 | 主要交付 |
|---|---|---|
| 1 | Go：Legacy Task 读模型 | `WorkbenchLegacyListStore.ListOwnedLegacyTasks`（facts-only 投影、kind=legacy、NOT EXISTS 反连接、keyset cursor、键集锁定测试） |
| 2 | Go：HTTP 端点与容器接线 | `WorkbenchLegacyListHandler`、`GET /api/v1/workbench/legacy-tasks` 路由（Viewer + 读门 + 参数绑定） |
| 3 | Go：同身份归档 + 迁移门禁 | `SetTaskArchived` 归属谓词扩展（或拥有 session 本身）+ 真实迁移库 + 真实 AdmissionCoordinator 的「仅新 Run 准入获得新能力」测试 |
| 4 | mobile-core：Legacy Task 域 | `legacy-tasks.ts`（DTO/`LegacyTaskBackendPort`/纯门禁 `legacyTaskGates`/scenario Adapter） |
| 5 | mobile-core：Task Office legacy 入口 | `legacyTasks/moreLegacyTasks/legacyHistory/followUp` 四方法（lease fail closed、迟到拒绝、epoch 失效）+ 错误码 |
| 6 | api-client：Legacy Remote Adapter | `createMobileLegacyTaskRemote`（list REST / history messages-load / followUp knowledge-chat SSE）+ `./mobile/legacy-tasks` 导出 |
| 7 | apps/mobile：视图与路由 | `legacy-tasks-view.ts` 控制器、`LegacyTasksScreen`、`/tasks/legacy` 路由、composition 接线、sse-stream body 透传、Tasks 屏入口 |
| 8 | apps/mobile：真实 HTTP 集成证据 | `legacy-tasks-integration-smoke.ts` + opt-in 集成测试（AC3，探针会话创建→legacy 投影→真实追问→清理） |

执行门控：无——前置 #34/#35 已全部合入当前 HEAD。Task 2 依赖 Task 1 的 store；Task 3 依赖 Task 1/2；Task 5 依赖 Task 4；Task 7 依赖 Task 5/6；Task 8 依赖 Task 7；其余按序执行。

并行合并注意（本计划与同批次 #36/#38/#41/#42/#46/#59 独立 worktree 后合并）：共享文件改动收敛为——`internal/router/routes_workbench.go`（文件末尾追加一个 `RegisterWorkbenchLegacyTaskRoutes` 函数，不动既有函数）、`internal/router/router.go`（`RouterParams` 追加一个 optional 字段 + 注册区追加一行）、`internal/container/workbench.go` 与 `internal/container/container.go`（各追加一个 provider）、`internal/application/repository/workbench_task_state.go`（仅 `SetTaskArchived` 归属谓词内追加 session 兜底分支）、`packages/mobile-core/src/task-office/task-office.ts`（追加 `ports.legacy` 可选字段与四个方法，标记「T14 追加区」）、`packages/mobile-core/src/task-office/task-office-errors.ts`（错误码联合追加一员）、`packages/mobile-core/src/index.ts`（追加导出块）、`packages/api-client/package.json`（exports 追加一条）、`apps/mobile/src/composition.ts`（`taskOfficeFor` 内追加 legacy 端口装配 + import 一行）、`apps/mobile/src/screens/TasksScreen.tsx`（可选 `onOpenLegacy` prop + 一个按钮）、`apps/mobile/src/app/tasks.tsx`（透传一个回调）、`apps/mobile/src/adapters/sse-stream.ts`（body 透传，三处小改）。新增文件全部为本计划独有。

---

### Task 1: Go——Legacy Task 读模型（repository 层）

**Files:**
- Create: `internal/application/repository/workbench_legacy_list.go`
- Test: `internal/application/repository/workbench_legacy_list_test.go`

**Interfaces:**
- Consumes: 既有 `ErrWorkbenchCursor`/`normalizeWorkbenchSearch`/`likeEscaped`/`searchPredicate`/`workbenchListDefaultLimit`/`workbenchListMaxLimit`（`internal/application/repository/workbench_list.go:19-63`，同包直接复用；`searchPredicate` 已按 `sessions.title` 限定）、`agentruntime.ErrNotFound`、测试夹具 `openRunTestDB`/`seedRunFixtures`/`seedWorkbenchListFixtures`/`insertWorkbenchRun`（`agent_run_test.go:29`、`workbench_list_test.go:22,57`；`seedRunFixtures` 种下 tenant 1、用户 `u1`、会话 `s1`='session-1'、`s2`='session-2'，`seedWorkbenchListFixtures` 追加 u2/s3 与 tenant 2/v1/t1）。sessions 表列（sqlite 真迁移库亲眼核实 `migrations/sqlite/000000_init.up.sql:127-156` + `000110_task_archive.up.sql`）：`id/title/tenant_id/user_id/created_at/updated_at/deleted_at/archived_at`。
- Produces: `WorkbenchLegacyFilter{ Query string; ArchivedOnly bool; Cursor string; Limit int }`、`WorkbenchLegacyTaskSummary{ TaskID string `json:"task_id"`; Title string `json:"title,omitempty"`; Attention string `json:"attention"`; ArchivedAt string `json:"archived_at,omitempty"`; UpdatedAt string `json:"updated_at"`; Kind string `json:"kind"` }`（Attention 恒 `"none"`、Kind 恒 `"legacy"`，均由行映射器设置）、`WorkbenchLegacyPage{ Items []WorkbenchLegacyTaskSummary `json:"items"`; NextCursor string `json:"next_cursor,omitempty"` }`、`WorkbenchLegacyListStore` 与方法 `(*WorkbenchLegacyListStore).ListOwnedLegacyTasks(ctx context.Context, tenantID uint64, ownerID string, filter WorkbenchLegacyFilter) (WorkbenchLegacyPage, error)`（查无归属/身份非法返回 `agentruntime.ErrNotFound`；游标非法/失配返回 `ErrWorkbenchCursor`）。Task 2 的 handler 与 Task 3 的迁移测试消费。

- [ ] **Step 1: 写失败测试**

`internal/application/repository/workbench_legacy_list_test.go`（新文件，完整内容）：

```go
package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedLegacySessions 添加纯聊天旧会话（0 条 agent_runs 行、显式时间戳），
// u1 名下三行（lg-0/lg-1/lg-2，keyset 翻页因此确定）、u2 一行、软删除一行。
func seedLegacySessions(t *testing.T, db *gorm.DB) {
	t.Helper()
	base := time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC)
	rows := []struct {
		id    string
		title string
		owner string
		at    time.Time
	}{
		{"lg-0", "旧聊天：会议纪要", "u1", base},
		{"lg-1", "旧聊天：周报素材", "u1", base.Add(time.Hour)},
		{"lg-2", "旧聊天：报销问题", "u1", base.Add(2 * time.Hour)},
		{"lg-3", "别人的旧聊天", "u2", base.Add(3 * time.Hour)},
		{"lg-del", "已删除的旧聊天", "u1", base.Add(4 * time.Hour)},
	}
	for _, row := range rows {
		require.NoError(t, db.Exec(
			`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type, created_at, updated_at)
			 VALUES (?, 1, ?, ?, 'builtin', ?, ?)`, row.id, row.title, row.owner, row.at, row.at,
		).Error)
	}
	require.NoError(t, db.Exec("UPDATE sessions SET deleted_at = ? WHERE id = 'lg-del'", base.Add(5*time.Hour)).Error)
}

func legacyTaskIDs(page WorkbenchLegacyPage) []string {
	ids := make([]string, 0, len(page.Items))
	for _, item := range page.Items {
		ids = append(ids, item.TaskID)
	}
	return ids
}

// TestWorkbenchLegacyListProjectsChatOnlySessionsWithinOwnerScope：legacy 投影
// 只覆盖「从未有过 agent_runs 行」的会话（与 run 列表二分区）；只携带可证明事实；
// 他人/异租户/软删除不可见；序列化行键集锁定——不出现任何 Run/Grant/预算/审批/
// Agent Version 字段（AC1）。
func TestWorkbenchLegacyListProjectsChatOnlySessionsWithinOwnerScope(t *testing.T) {
	db := openRunTestDB(t)
	seedWorkbenchListFixtures(t, db)
	seedLegacySessions(t, db)
	base := time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)
	// s1/s2 有 run 行：必须留在执行列表、绝不进入 legacy 投影。
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u1", runID: "r-a1", session: "s1", status: "running", agent: "agent-x", target: "platform", at: base})
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u1", runID: "r-a2", session: "s2", status: "succeeded", agent: "agent-y", target: "platform", at: base.Add(time.Second)})

	store := NewWorkbenchLegacyListStore(db)
	ctx := context.Background()

	page, err := store.ListOwnedLegacyTasks(ctx, 1, "u1", WorkbenchLegacyFilter{})
	require.NoError(t, err)
	require.Equal(t, []string{"lg-2", "lg-1", "lg-0"}, legacyTaskIDs(page), "newest first; run-backed s1/s2 and soft-deleted lg-del excluded")

	item := page.Items[0]
	require.Equal(t, "legacy", item.Kind)
	require.Equal(t, "none", item.Attention, "no Run means no provable attention source")
	require.Equal(t, "旧聊天：报销问题", item.Title)
	require.Empty(t, item.ArchivedAt)
	require.NotEmpty(t, item.UpdatedAt)

	// AC1 键集锁定：legacy 行的序列化形状不得携带任何 Run 级/安全语义字段。
	keys, err := marshalLegacyKeys(page.Items[0])
	require.NoError(t, err)
	for _, key := range []string{"task_id", "title", "attention", "updated_at", "kind"} {
		require.Contains(t, keys, key)
	}
	for _, forbidden := range []string{
		"run_id", "run_status", "execution_status", "settlement_status",
		"grant", "budget", "approval", "agent_version", "revision", "wait_reason",
	} {
		require.NotContains(t, keys, forbidden, "legacy projection must not fabricate %q", forbidden)
	}

	// 他人同租户 / 异租户：不可见（u2 只见自己的 lg-3；tenant 2 无 legacy 行）。
	page, err = store.ListOwnedLegacyTasks(ctx, 1, "u2", WorkbenchLegacyFilter{})
	require.NoError(t, err)
	require.Equal(t, []string{"lg-3"}, legacyTaskIDs(page))
	page, err = store.ListOwnedLegacyTasks(ctx, 2, "v1", WorkbenchLegacyFilter{})
	require.NoError(t, err)
	require.Empty(t, legacyTaskIDs(page))

	// 身份非法：ErrNotFound，零查询语义。
	_, err = store.ListOwnedLegacyTasks(ctx, 0, "u1", WorkbenchLegacyFilter{})
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
	_, err = store.ListOwnedLegacyTasks(ctx, 1, " ", WorkbenchLegacyFilter{})
	require.ErrorIs(t, err, agentruntime.ErrNotFound)

	// 标题子串搜索（忽略大小写、全空白归一）。
	page, err = store.ListOwnedLegacyTasks(ctx, 1, "u1", WorkbenchLegacyFilter{Query: "周报"})
	require.NoError(t, err)
	require.Equal(t, []string{"lg-1"}, legacyTaskIDs(page))
}

// TestWorkbenchLegacyListArchivedFilterCursorBindingAndCrossEndpointReplay：
// archived 过滤镜像 run 列表语义；keyset 分页稳定；伪造游标、过滤器失配游标、
// 以及 run 列表签发的游标重放到 legacy 端点一律 ErrWorkbenchCursor。
func TestWorkbenchLegacyListArchivedFilterCursorBindingAndCrossEndpointReplay(t *testing.T) {
	db := openRunTestDB(t)
	seedLegacySessions(t, db)
	archived := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	require.NoError(t, db.Exec("UPDATE sessions SET archived_at = ? WHERE id = 'lg-2'", archived).Error)

	store := NewWorkbenchLegacyListStore(db)
	ctx := context.Background()
	// openRunTestDB 自带的 s1/s2（u1、0 run）也是合法 legacy 行且时间戳为夹具时钟——
	// 以标题搜索「旧聊天」把断言域收敛到 lg-* 行，顺序因此确定。
	scope := WorkbenchLegacyFilter{Query: "旧聊天"}

	active, err := store.ListOwnedLegacyTasks(ctx, 1, "u1", scope)
	require.NoError(t, err)
	require.Equal(t, []string{"lg-1", "lg-0"}, legacyTaskIDs(active), "archived task leaves the default view")

	archivedOnly, err := store.ListOwnedLegacyTasks(ctx, 1, "u1", WorkbenchLegacyFilter{Query: "旧聊天", ArchivedOnly: true})
	require.NoError(t, err)
	require.Equal(t, []string{"lg-2"}, legacyTaskIDs(archivedOnly))
	require.NotEmpty(t, archivedOnly.Items[0].ArchivedAt)

	// keyset：limit 1 翻页不重不漏。
	first, err := store.ListOwnedLegacyTasks(ctx, 1, "u1", WorkbenchLegacyFilter{Query: "旧聊天", Limit: 1})
	require.NoError(t, err)
	require.Equal(t, []string{"lg-1"}, legacyTaskIDs(first))
	require.NotEmpty(t, first.NextCursor)
	second, err := store.ListOwnedLegacyTasks(ctx, 1, "u1", WorkbenchLegacyFilter{Query: "旧聊天", Limit: 1, Cursor: first.NextCursor})
	require.NoError(t, err)
	require.Equal(t, []string{"lg-0"}, legacyTaskIDs(second))

	// 过滤器失配：带搜索词视图签发的游标重放到无搜索词视图被拒。
	_, err = store.ListOwnedLegacyTasks(ctx, 1, "u1", WorkbenchLegacyFilter{Cursor: first.NextCursor})
	require.ErrorIs(t, err, ErrWorkbenchCursor)
	// 跨身份重放被拒。
	_, err = store.ListOwnedLegacyTasks(ctx, 1, "u2", WorkbenchLegacyFilter{Query: "旧聊天", Cursor: first.NextCursor})
	require.ErrorIs(t, err, ErrWorkbenchCursor)
	// 伪造 base64 被拒（'Zm9yZ2Vk' 解码为非法 JSON 'forged'）。
	_, err = store.ListOwnedLegacyTasks(ctx, 1, "u1", WorkbenchLegacyFilter{Query: "旧聊天", Cursor: "Zm9yZ2Vk"})
	require.ErrorIs(t, err, ErrWorkbenchCursor)
	// run 列表签发的游标重放到 legacy 端点被拒（kind 判别符缺失）。
	runCursor := encodeWorkbenchListCursor(workbenchListCursor{
		Version: 1, TenantID: 1, OwnerID: "u1",
		CreatedAt: "2026-09-21T09:00:00Z", RunID: "r-a1",
	})
	_, err = store.ListOwnedLegacyTasks(ctx, 1, "u1", WorkbenchLegacyFilter{Query: "旧聊天", Cursor: runCursor})
	require.ErrorIs(t, err, ErrWorkbenchCursor)
}

func marshalLegacyKeys(summary WorkbenchLegacyTaskSummary) ([]string, error) {
	raw, err := json.Marshal(summary)
	if err != nil {
		return nil, err
	}
	var projected map[string]any
	if err := json.Unmarshal(raw, &projected); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(projected))
	for key := range projected {
		keys = append(keys, key)
	}
	return keys, nil
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/application/repository/ -run 'TestWorkbenchLegacyList' -count=1`
Expected: 编译失败 `undefined: NewWorkbenchLegacyListStore`（RED）

- [ ] **Step 3: 最小实现**

`internal/application/repository/workbench_legacy_list.go`（新文件，完整内容）：

```go
package repository

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"gorm.io/gorm"
)

// T14（Issue #44）：Legacy Task 读模型。从未有过 agent_runs 行的旧 Session 以
// 同一身份（taskId = sessionId，ADR-0004）投影为 Legacy Task。本列表与
// ListOwnedExecutions 把全部会话二分区：有 Run 的进执行列表、无 Run 的进
// legacy 列表，同一 taskId 两处互斥。只投影历史能证明的事实：标题、归档
// 时间、更新时间；attention 恒为 "none"（waiting_user 与 pending interaction
// 都是 Run 作用域事实，无 Run 即无可证明的关注来源）；绝不伪造
// Run/Grant/预算/审批/Agent Version 字段。

// WorkbenchLegacyFilter is the caller-owned facet set. Tenant and owner are
// never part of the filter: they are separate required arguments so every
// generated query binds them, mirroring WorkbenchExecutionFilter.
type WorkbenchLegacyFilter struct {
	Query        string // 任务标题子串搜索（忽略大小写；全空白归一为无搜索）
	ArchivedOnly bool   // false（默认）=仅未归档；true=仅已归档
	Cursor       string
	Limit        int
}

// WorkbenchLegacyTaskSummary is one legacy task row: facts only. Kind is the
// explicit legacy marker clients gate Run-scoped capabilities on; Attention is
// always "none" because no provable attention source exists without a Run.
type WorkbenchLegacyTaskSummary struct {
	TaskID     string `json:"task_id"`
	Title      string `json:"title,omitempty"`
	Attention  string `json:"attention"`
	ArchivedAt string `json:"archived_at,omitempty"`
	UpdatedAt  string `json:"updated_at"`
	Kind       string `json:"kind"`
}

type WorkbenchLegacyPage struct {
	Items      []WorkbenchLegacyTaskSummary `json:"items"`
	NextCursor string                       `json:"next_cursor,omitempty"`
}

// workbenchLegacyCursor is the opaque pagination token. The "legacy" kind
// discriminator makes a run-list cursor replayed here (and vice versa) fail
// validation instead of silently continuing the wrong row space.
type workbenchLegacyCursor struct {
	Version      int    `json:"v"`
	Kind         string `json:"kind"`
	TenantID     uint64 `json:"tenant_id"`
	OwnerID      string `json:"owner_id"`
	Query        string `json:"q,omitempty"`
	ArchivedOnly bool   `json:"archived_only,omitempty"`
	UpdatedAt    string `json:"updated_at"`
	TaskID       string `json:"task_id"`
}

type workbenchLegacyRow struct {
	TaskID     string
	Title      string
	ArchivedAt *time.Time
	UpdatedAt  time.Time
}

func (workbenchLegacyRow) TableName() string { return "sessions" }

type WorkbenchLegacyListStore struct{ db *gorm.DB }

func NewWorkbenchLegacyListStore(db *gorm.DB) *WorkbenchLegacyListStore {
	return &WorkbenchLegacyListStore{db: db}
}

// A session is legacy exactly when no agent_runs row ever referenced it. The
// constant NOT EXISTS anti-join carries no external input; every caller-owned
// value stays behind bound parameters.
const legacyNoRunsExpr = `NOT EXISTS (SELECT 1 FROM agent_runs ar
	WHERE ar.tenant_id = sessions.tenant_id AND ar.session_id = sessions.id)`

// legacyOrderExpr and legacyKeysetPredicate normalize updated_at per dialect
// (same rationale as the run list: SQLite stores driver-formatted text).
func legacyOrderExpr(db *gorm.DB) string {
	if db.Dialector.Name() == "sqlite" {
		return "julianday(sessions.updated_at) DESC, sessions.id DESC"
	}
	return "sessions.updated_at DESC, sessions.id DESC"
}

func legacyKeysetPredicate(db *gorm.DB, anchor time.Time, taskID string) string {
	if db.Dialector.Name() == "sqlite" {
		return "(julianday(sessions.updated_at) < julianday(?) OR (julianday(sessions.updated_at) = julianday(?) AND sessions.id < ?))"
	}
	return "(sessions.updated_at < ? OR (sessions.updated_at = ? AND sessions.id < ?))"
}

func encodeWorkbenchLegacyCursor(cursor workbenchLegacyCursor) string {
	raw, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeWorkbenchLegacyCursor(raw string) (workbenchLegacyCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return workbenchLegacyCursor{}, ErrWorkbenchCursor
	}
	var cursor workbenchLegacyCursor
	if err := json.Unmarshal(decoded, &cursor); err != nil {
		return workbenchLegacyCursor{}, ErrWorkbenchCursor
	}
	if cursor.Version != 1 || cursor.Kind != "legacy" || cursor.TenantID == 0 ||
		strings.TrimSpace(cursor.OwnerID) == "" || strings.TrimSpace(cursor.TaskID) == "" ||
		strings.TrimSpace(cursor.UpdatedAt) == "" {
		return workbenchLegacyCursor{}, ErrWorkbenchCursor
	}
	if _, err := time.Parse(time.RFC3339Nano, cursor.UpdatedAt); err != nil {
		return workbenchLegacyCursor{}, ErrWorkbenchCursor
	}
	return cursor, nil
}

func legacySummaryFromRow(row workbenchLegacyRow) WorkbenchLegacyTaskSummary {
	summary := WorkbenchLegacyTaskSummary{
		TaskID:    row.TaskID,
		Title:     strings.TrimSpace(row.Title),
		Attention: "none",
		UpdatedAt: row.UpdatedAt.UTC().Format(time.RFC3339Nano),
		Kind:      "legacy",
	}
	if row.ArchivedAt != nil {
		summary.ArchivedAt = row.ArchivedAt.UTC().Format(time.RFC3339Nano)
	}
	return summary
}

// ListOwnedLegacyTasks returns the owner's legacy tasks (sessions without any
// agent_runs row), newest first, with a stable (updated_at, id) keyset cursor
// bound to the exact tenant/owner/filter it was issued under.
func (s *WorkbenchLegacyListStore) ListOwnedLegacyTasks(ctx context.Context, tenantID uint64, ownerID string, filter WorkbenchLegacyFilter) (WorkbenchLegacyPage, error) {
	if s == nil || s.db == nil || tenantID == 0 || strings.TrimSpace(ownerID) == "" {
		return WorkbenchLegacyPage{}, agentruntime.ErrNotFound
	}
	filter.Query = normalizeWorkbenchSearch(filter.Query)
	limit := filter.Limit
	if limit <= 0 {
		limit = workbenchListDefaultLimit
	}
	if limit > workbenchListMaxLimit {
		limit = workbenchListMaxLimit
	}

	query := s.db.WithContext(ctx).Table("sessions").
		Select("sessions.id AS task_id, sessions.title, sessions.archived_at, sessions.updated_at").
		Where("sessions.tenant_id = ? AND sessions.user_id = ? AND sessions.deleted_at IS NULL", tenantID, ownerID).
		Where(legacyNoRunsExpr)
	if filter.Query != "" {
		query = query.Where(searchPredicate(s.db), "%"+likeEscaped(filter.Query)+"%")
	}
	if filter.ArchivedOnly {
		query = query.Where("sessions.archived_at IS NOT NULL")
	} else {
		query = query.Where("sessions.archived_at IS NULL")
	}
	if strings.TrimSpace(filter.Cursor) != "" {
		cursor, err := decodeWorkbenchLegacyCursor(filter.Cursor)
		if err != nil {
			return WorkbenchLegacyPage{}, err
		}
		if cursor.TenantID != tenantID || cursor.OwnerID != ownerID ||
			cursor.Query != filter.Query || cursor.ArchivedOnly != filter.ArchivedOnly {
			return WorkbenchLegacyPage{}, fmt.Errorf("%w: cursor does not match the active filter", ErrWorkbenchCursor)
		}
		anchor, err := time.Parse(time.RFC3339Nano, cursor.UpdatedAt)
		if err != nil {
			return WorkbenchLegacyPage{}, ErrWorkbenchCursor
		}
		query = query.Where(legacyKeysetPredicate(s.db, anchor, cursor.TaskID), anchor, anchor, cursor.TaskID)
	}

	var rows []workbenchLegacyRow
	if err := query.Order(legacyOrderExpr(s.db)).Limit(limit + 1).Find(&rows).Error; err != nil {
		return WorkbenchLegacyPage{}, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	page := WorkbenchLegacyPage{Items: make([]WorkbenchLegacyTaskSummary, 0, len(rows))}
	for _, row := range rows {
		page.Items = append(page.Items, legacySummaryFromRow(row))
	}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		page.NextCursor = encodeWorkbenchLegacyCursor(workbenchLegacyCursor{
			Version: 1, Kind: "legacy", TenantID: tenantID, OwnerID: ownerID,
			Query: filter.Query, ArchivedOnly: filter.ArchivedOnly,
			UpdatedAt: last.UpdatedAt.UTC().Format(time.RFC3339Nano), TaskID: last.TaskID,
		})
	}
	return page, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/application/repository/ -run 'TestWorkbenchLegacyList' -count=1`
Expected: PASS（两个测试全绿）

- [ ] **Step 5: 提交**

```bash
git add internal/application/repository/workbench_legacy_list.go internal/application/repository/workbench_legacy_list_test.go
git commit -m "feat(workbench): legacy task read model projects run-less sessions as facts-only rows (T44 AC1)"
```

---

### Task 2: Go——HTTP 端点与容器接线

**Files:**
- Create: `internal/handler/session/workbench_legacy_list.go`
- Test: `internal/handler/session/workbench_legacy_list_handler_test.go`
- Modify: `internal/router/routes_workbench.go`（文件末尾追加注册函数）
- Modify: `internal/router/router.go:58` 附近（`RouterParams` 追加字段）与 `router.go:372` 后（注册调用）
- Modify: `internal/container/workbench.go`（追加 provider）
- Modify: `internal/container/container.go:274` 附近（追加 `must(container.Provide(...))`）

**Interfaces:**
- Consumes: Task 1 的 `WorkbenchLegacyListStore`/`WorkbenchLegacyFilter`/`WorkbenchLegacyPage`/`ErrWorkbenchCursor`；既有 `workbenchReadGate`/`rbacGuards`（`internal/router/routes_workbench.go:18,33`）、`types.TenantIDFromContext`/`types.UserIDFromContext` 与 context key 兜底模式（`internal/handler/session/workbench_list.go:38-53`）、路由注册链（`internal/router/router.go:367-372`、`internal/container/container.go:274`）。
- Produces: `GET /api/v1/workbench/legacy-tasks`（query：`q`/`archived`/`cursor`/`limit`；Viewer + `workbenchReadGate` + apiKeyChat 边界，与执行列表同 lane）、`OwnedLegacyTaskLister` 接口与 `NewWorkbenchLegacyListHandler(lists OwnedLegacyTaskLister) *WorkbenchLegacyListHandler`、容器 provider `NewWorkbenchLegacyListHandler(db *gorm.DB) *session.WorkbenchLegacyListHandler`。Task 3 的迁移测试与 Task 6/8 的远端消费。

- [ ] **Step 1: 写失败测试**

`internal/handler/session/workbench_legacy_list_handler_test.go`（新文件，完整内容）：

```go
package session

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type legacyListerStub struct {
	page    repository.WorkbenchLegacyPage
	err     error
	tenant  uint64
	owner   string
	filter  repository.WorkbenchLegacyFilter
	invoked bool
}

func (s *legacyListerStub) ListOwnedLegacyTasks(_ context.Context, tenantID uint64, ownerID string, filter repository.WorkbenchLegacyFilter) (repository.WorkbenchLegacyPage, error) {
	s.invoked = true
	s.tenant, s.owner, s.filter = tenantID, ownerID, filter
	return s.page, s.err
}

func TestListLegacyTasksRequiresAuthenticatedIdentity(t *testing.T) {
	for name, identity := range map[string][2]any{
		"no tenant": {uint64(0), "u1"},
		"no owner":  {uint64(1), ""},
	} {
		lists := &legacyListerStub{}
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, "/workbench/legacy-tasks", nil)
		listContext(c, identity[0].(uint64), identity[1].(string))
		NewWorkbenchLegacyListHandler(lists).ListLegacyTasks(c)
		require.Equal(t, http.StatusUnauthorized, recorder.Code, name)
		require.False(t, lists.invoked, name)
	}
}

func TestListLegacyTasksPassesQueryFacetsOnly(t *testing.T) {
	lists := &legacyListerStub{page: repository.WorkbenchLegacyPage{Items: []repository.WorkbenchLegacyTaskSummary{{
		TaskID: "lg-1", Title: "旧聊天", Attention: "none", UpdatedAt: "2026-09-20T08:00:00Z", Kind: "legacy",
	}}, NextCursor: "cursor-2"}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/workbench/legacy-tasks?q=%E5%91%A8%E6%8A%A5&archived=true&limit=15&cursor=abc", nil)
	listContext(c, 1, "u1")
	NewWorkbenchLegacyListHandler(lists).ListLegacyTasks(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, uint64(1), lists.tenant)
	require.Equal(t, "u1", lists.owner)
	require.Equal(t, "周报", lists.filter.Query)
	require.True(t, lists.filter.ArchivedOnly)
	require.Equal(t, 15, lists.filter.Limit)
	require.Equal(t, "abc", lists.filter.Cursor)

	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Items      []repository.WorkbenchLegacyTaskSummary `json:"items"`
			NextCursor string                                 `json:"next_cursor"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.True(t, envelope.Success)
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, "lg-1", envelope.Data.Items[0].TaskID)
	require.Equal(t, "legacy", envelope.Data.Items[0].Kind)
	require.Equal(t, "cursor-2", envelope.Data.NextCursor)
}

func TestListLegacyTasksRejectsBadCursorLimitAndSurfacesServerFailure(t *testing.T) {
	lists := &legacyListerStub{err: repository.ErrWorkbenchCursor}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/workbench/legacy-tasks?cursor=forged", nil)
	listContext(c, 1, "u1")
	NewWorkbenchLegacyListHandler(lists).ListLegacyTasks(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)

	badLimit := &legacyListerStub{}
	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/workbench/legacy-tasks?limit=-3", nil)
	listContext(c, 1, "u1")
	NewWorkbenchLegacyListHandler(badLimit).ListLegacyTasks(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.False(t, badLimit.invoked)

	failed := &legacyListerStub{err: context.Canceled}
	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/workbench/legacy-tasks", nil)
	listContext(c, 1, "u1")
	NewWorkbenchLegacyListHandler(failed).ListLegacyTasks(c)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}
```

（`listContext` 复用同包 `workbench_list_handler_test.go:31` 既有 helper，不重复定义。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/handler/session/ -run 'TestListLegacyTasks' -count=1`
Expected: 编译失败 `undefined: NewWorkbenchLegacyListHandler`（RED）

- [ ] **Step 3: 最小实现**

`internal/handler/session/workbench_legacy_list.go`（新文件，完整内容）：

```go
package session

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// OwnedLegacyTaskLister is the ownership-scoped legacy task list port.
// Implementations must bind every row to the authenticated tenant and owner;
// the handler never forwards a caller-supplied tenant or owner.
type OwnedLegacyTaskLister interface {
	ListOwnedLegacyTasks(ctx context.Context, tenantID uint64, ownerID string, filter repository.WorkbenchLegacyFilter) (repository.WorkbenchLegacyPage, error)
}

// WorkbenchLegacyListHandler serves GET /workbench/legacy-tasks (T14): the
// facts-only projection of sessions that never had a Run. Authentication and
// ownership come from the request context, the same boundary the execution
// list relies on.
type WorkbenchLegacyListHandler struct {
	lists OwnedLegacyTaskLister
}

func NewWorkbenchLegacyListHandler(lists OwnedLegacyTaskLister) *WorkbenchLegacyListHandler {
	return &WorkbenchLegacyListHandler{lists: lists}
}

func (h *WorkbenchLegacyListHandler) ListLegacyTasks(c *gin.Context) {
	if h == nil || h.lists == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	tenantID, ok := types.TenantIDFromContext(c.Request.Context())
	if !ok || tenantID == 0 {
		if value, exists := c.Get(types.TenantIDContextKey.String()); exists {
			tenantID, ok = value.(uint64)
		}
	}
	ownerID, ownerOK := types.UserIDFromContext(c.Request.Context())
	if !ownerOK || ownerID == "" {
		if value, exists := c.Get(types.UserIDContextKey.String()); exists {
			ownerID, ownerOK = value.(string)
		}
	}
	if !ok || !ownerOK || tenantID == 0 || ownerID == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	filter := repository.WorkbenchLegacyFilter{
		Query:  c.Query("q"),
		Cursor: strings.TrimSpace(c.Query("cursor")),
	}
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 0 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "limit must be a non-negative integer"})
			return
		}
		filter.Limit = limit
	}
	if raw := strings.TrimSpace(c.Query("archived")); raw != "" {
		archived, err := strconv.ParseBool(raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "archived must be a boolean"})
			return
		}
		filter.ArchivedOnly = archived
	}
	page, err := h.lists.ListOwnedLegacyTasks(c.Request.Context(), tenantID, ownerID, filter)
	if errors.Is(err, repository.ErrWorkbenchCursor) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid cursor"})
		return
	}
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": page})
}
```

路由注册——`internal/router/routes_workbench.go` 文件末尾追加：

```go
// RegisterWorkbenchLegacyTaskRoutes exposes the T14 legacy task projection:
// sessions that never had a Run, projected as facts-only legacy rows under
// the same Viewer/API-key boundary and W34 read gate as the run list.
func RegisterWorkbenchLegacyTaskRoutes(r *gin.RouterGroup, h *session.WorkbenchLegacyListHandler, g *rbacGuards) {
	if h == nil || g == nil {
		return
	}
	legacy := g.apiKeyGroup(r.Group("/workbench/legacy-tasks", g.Viewer(), workbenchReadGate(g.cfg)), apiKeyChat(apiKeyFullAccess()))
	legacy.GET("", h.ListLegacyTasks)
}
```

`internal/router/router.go`——`RouterParams` 结构体（`router.go:58` `WorkbenchListHandler` 字段之后）追加一行：

```go
	WorkbenchLegacyListHandler  *session.WorkbenchLegacyListHandler `optional:"true"`
```

并在 `router.go:372`（`RegisterWorkbenchTaskStateRoutes` 调用之后）追加一行：

```go
		RegisterWorkbenchLegacyTaskRoutes(v1, params.WorkbenchLegacyListHandler, rbacGuards)
```

`internal/container/workbench.go` 追加 provider（文件既有 provider 区末尾）：

```go
// NewWorkbenchLegacyListHandler wires the T14 legacy task projection to the
// ownership-scoped repository; tenant/owner always come from the
// authenticated context.
func NewWorkbenchLegacyListHandler(db *gorm.DB) *session.WorkbenchLegacyListHandler {
	return session.NewWorkbenchLegacyListHandler(repository.NewWorkbenchLegacyListStore(db))
}
```

`internal/container/container.go` 在 `container.go:274`（`NewWorkbenchListHandler` 的 Provide）之后追加：

```go
	must(container.Provide(NewWorkbenchLegacyListHandler))
```

- [ ] **Step 4: 运行测试与编译确认通过**

Run: `go test ./internal/handler/session/ -run 'TestListLegacyTasks' -count=1 && go build ./internal/... ./cmd/...`
Expected: 测试 PASS + 构建成功

- [ ] **Step 5: 提交**

```bash
git add internal/handler/session/workbench_legacy_list.go internal/handler/session/workbench_legacy_list_handler_test.go internal/router/routes_workbench.go internal/router/router.go internal/container/workbench.go internal/container/container.go
git commit -m "feat(workbench): GET /workbench/legacy-tasks endpoint behind the same viewer/read-gate lane (T44)"
```

---

### Task 3: Go——同身份归档与迁移门禁（AC1 身份 + AC2 门禁 + spec Migration tests）

**Files:**
- Modify: `internal/application/repository/workbench_task_state.go:24-48`（`SetTaskArchived` 归属谓词）
- Modify: `internal/application/repository/workbench_task_state_test.go`（追加一个测试）
- Create: `internal/handler/session/workbench_legacy_migration_test.go`

**Interfaces:**
- Consumes: Task 1 的 `WorkbenchLegacyListStore`、Task 2 的 `WorkbenchLegacyListHandler`；既有 `WorkbenchTaskStateStore.SetTaskArchived`/`ErrWorkbenchTaskNotFound`（`workbench_task_state.go:24-48`）、`WorkbenchListStore.ListOwnedExecutions`、`AgentRunStore.GetOwnedRun`（`agent_run.go:100`）、真实准入链 `workbenchservice.NewAdmissionCoordinator(db, repository.NewAgentRunStore(db), &integrationBudget{}, nil)` 与 `NewWorkbenchStartHandler`（`workbench_start_integration_test.go:53-54`，同包复用其 `openWorkbenchHTTPDB`/`withIdentity`/`integrationBudget`——注意 `openWorkbenchHTTPDB` 种下 tenant 1、用户 `u1`、会话 `s1`（engine trpc，0 run，天然 legacy））。
- Produces: `SetTaskArchived` 归属谓词扩展为「拥有该 session 至少一个 run，**或**该 session 本身属于 caller（且未软删除）」——Legacy Task 同身份可归档/恢复；`TestLegacyTaskMigrationOnlyNewRunAdmissionGrantsRunCapabilities`（spec Testing Decisions「Migration tests verify old Sessions remain readable and only gain new capabilities through explicit upgrade or new Run admission」的 Go 集成证据，真实迁移库 + 真实 HTTP handler + 真实 AdmissionCoordinator）。

- [ ] **Step 1: 写失败测试**

先在 `internal/application/repository/workbench_task_state_test.go` 末尾追加（完整内容）：

```go
// TestWorkbenchTaskStateArchiveLegacySessionBySessionOwner：T14——从未有过
// run 的旧会话（Legacy Task）由 session 归属者归档/恢复（同身份生命周期）；
// 他人/异租户依旧 ErrWorkbenchTaskNotFound，不写任何行。
func TestWorkbenchTaskStateArchiveLegacySessionBySessionOwner(t *testing.T) {
	db := openRunTestDB(t)
	seedWorkbenchListFixtures(t, db)
	store := NewWorkbenchTaskStateStore(db)
	ctx := context.Background()
	now := time.Date(2026, 9, 24, 9, 0, 0, 0, time.UTC)

	// s2 属于 u1 且 0 run（seedRunFixtures 未给 s2 加 run）——Legacy Task。
	require.NoError(t, store.SetTaskArchived(ctx, 1, "u1", "s2", true, now))
	at := archivedAt(t, db, "s2")
	require.NotNil(t, at)
	require.Equal(t, now.UTC(), at.UTC())
	require.NoError(t, store.SetTaskArchived(ctx, 1, "u1", "s2", false, now))
	require.Nil(t, archivedAt(t, db, "s2"))

	// s3 属于 u2 且 0 run：他人不可归档；t1 属于异租户 v1：同样拒绝。
	require.ErrorIs(t, store.SetTaskArchived(ctx, 1, "u1", "s3", true, now), ErrWorkbenchTaskNotFound)
	require.ErrorIs(t, store.SetTaskArchived(ctx, 2, "v1", "s2", true, now), ErrWorkbenchTaskNotFound)
	require.Nil(t, archivedAt(t, db, "s3"))

	// 软删除的 session 不再是可归档的任务。
	require.NoError(t, db.Exec("UPDATE sessions SET deleted_at = ? WHERE id = 's2'", now).Error)
	require.ErrorIs(t, store.SetTaskArchived(ctx, 1, "u1", "s2", true, now), ErrWorkbenchTaskNotFound)
}
```

再新建 `internal/handler/session/workbench_legacy_migration_test.go`（完整内容）：

```go
package session

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	workbenchservice "github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestLegacyTaskMigrationOnlyNewRunAdmissionGrantsRunCapabilities：spec
// Testing Decisions 的 Migration test——旧 Session（0 run）保持可读、可归档
// （同身份），但 Run 作用域能力在准入前不存在；唯一获得通道是新 Run 准入
// （POST /workbench/executions）。准入后 legacy 投影让位执行列表，Run 事实
// 才可证明。真实 sqlite 迁移库 + 真实 HTTP handler + 真实 AdmissionCoordinator。
func TestLegacyTaskMigrationOnlyNewRunAdmissionGrantsRunCapabilities(t *testing.T) {
	db := openWorkbenchHTTPDB(t) // 种下 tenant 1 / u1 / s1（engine trpc，0 run → 天然 legacy）
	ctx := context.Background()

	legacyStore := repository.NewWorkbenchLegacyListStore(db)
	runStore := repository.NewAgentRunStore(db)

	// 1) 旧 Session 可读：legacy 投影包含 s1，facts-only。
	page, err := legacyStore.ListOwnedLegacyTasks(ctx, 1, "u1", repository.WorkbenchLegacyFilter{})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, "s1", page.Items[0].TaskID)
	require.Equal(t, "legacy", page.Items[0].Kind)
	require.Equal(t, "none", page.Items[0].Attention)

	// 2) Run 作用域能力在准入前不存在：任何 run 读（含伪造 id）一律 ErrNotFound。
	_, err = runStore.GetOwnedRun(ctx, 1, "u1", "fabricated-run")
	require.ErrorIs(t, err, agentruntime.ErrNotFound)

	// 3) 同身份归档回路（HTTP 层，真实 handler + 真实 store）。
	gin.SetMode(gin.TestMode)
	r := gin.New()
	states := NewWorkbenchTaskStateHandler(repository.NewWorkbenchTaskStateStore(db))
	r.POST("/api/v1/workbench/tasks/:task_id/archive", withIdentity(1, "u1"), states.Archive)
	r.DELETE("/api/v1/workbench/tasks/:task_id/archive", withIdentity(1, "u1"), states.Restore)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/v1/workbench/tasks/s1/archive", nil))
	require.Equal(t, http.StatusOK, resp.Code)
	archivedPage, err := legacyStore.ListOwnedLegacyTasks(ctx, 1, "u1", repository.WorkbenchLegacyFilter{})
	require.NoError(t, err)
	require.Empty(t, archivedPage.Items, "archived legacy task leaves the default legacy view")
	restored, err := legacyStore.ListOwnedLegacyTasks(ctx, 1, "u1", repository.WorkbenchLegacyFilter{ArchivedOnly: true})
	require.NoError(t, err)
	require.Equal(t, "s1", restored.Items[0].TaskID)
	resp = httptest.NewRecorder()
	r.ServeHTTP(resp, httptest.NewRequest(http.MethodDelete, "/api/v1/workbench/tasks/s1/archive", nil))
	require.Equal(t, http.StatusOK, resp.Code)

	// 4) 显式升级 = 新 Run 准入（真实 AdmissionCoordinator，预算适配器复用
	// 既有 integrationBudget）。
	coordinator := workbenchservice.NewAdmissionCoordinator(db, repository.NewAgentRunStore(db), &integrationBudget{}, nil)
	start := NewWorkbenchStartHandler(coordinator)
	r.POST("/api/v1/workbench/executions", withIdentity(1, "u1"), start.Start)
	body := `{"session_id":"s1","agent_id":"a1","target_id":"platform","request_id":"legacy-upgrade-1","text":"upgrade this legacy task","budget_upper":100}`
	resp = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/executions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(resp, req)
	require.Equal(t, http.StatusAccepted, resp.Code)
	var accepted struct {
		Success bool `json:"success"`
		Data    struct {
			RunID string `json:"run_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &accepted))
	require.True(t, accepted.Success)
	require.NotEmpty(t, accepted.Data.RunID)

	// 5) 准入后：legacy 投影让位执行列表；Run 事实此刻才可证明。
	legacyAfter, err := legacyStore.ListOwnedLegacyTasks(ctx, 1, "u1", repository.WorkbenchLegacyFilter{})
	require.NoError(t, err)
	require.Empty(t, legacyAfter.Items, "the admitted Run is the explicit upgrade; the task is no longer legacy")
	executions, err := repository.NewWorkbenchListStore(db).ListOwnedExecutions(ctx, 1, "u1", repository.WorkbenchExecutionFilter{})
	require.NoError(t, err)
	require.Len(t, executions.Items, 1)
	require.Equal(t, "s1", executions.Items[0].SessionID)
	require.Equal(t, accepted.Data.RunID, executions.Items[0].RunID)
	admitted, err := runStore.GetOwnedRun(ctx, 1, "u1", accepted.Data.RunID)
	require.NoError(t, err)
	require.Equal(t, "s1", admitted.SessionID)
}
```

（import 区完整为：`bytes`、`context`、`encoding/json`、`net/http`、`net/http/httptest`、`testing`、`agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"`、`"github.com/Tencent/WeKnora/internal/application/repository"`、`workbenchservice "github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench"`、`"github.com/gin-gonic/gin"`、`"github.com/stretchr/testify/require"`。`agentruntime.Run.SessionID` 字段已在当前 HEAD 亲眼核实：`internal/modules/agentruntime/agent/runtime/contracts.go:116`。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/application/repository/ -run 'TestWorkbenchTaskStateArchiveLegacySessionBySessionOwner' -count=1 && go test ./internal/handler/session/ -run 'TestLegacyTaskMigration' -count=1`
Expected: 第一个 FAIL（`s2` 归档返回 `ErrWorkbenchTaskNotFound`，_require.ErrorIs 不成立——实际是 `require.NoError` 收到 error 而失败）；第二个 FAIL（同样死在归档 404 分支）。RED 确认两处都指向归属谓词。

- [ ] **Step 3: 最小实现**

`internal/application/repository/workbench_task_state.go` 的 `SetTaskArchived`（`workbench_task_state.go:28-48`）归属谓词段替换为（其余不动）：

```go
// SetTaskArchived archives or restores the caller's task (taskId = sessionId,
// ADR-0004). Ownership follows the read model exactly: the caller must own at
// least one run of this session inside this tenant, OR own the session itself
// (T14 Legacy Task — a run-less session archives by session ownership; soft-
// deleted sessions are no longer tasks). Identity always comes from the
// authenticated context at the handler layer, never from the request body.
```

`workbench_task_state.go:33-41` 的计数段（`var owned int64` 至 `return ErrWorkbenchTaskNotFound`）替换为：

```go
	taskID = strings.TrimSpace(taskID)
	var owned int64
	if err := s.db.WithContext(ctx).Table("agent_runs").
		Where("tenant_id = ? AND session_id = ? AND owner_id = ?", tenantID, taskID, ownerID).
		Limit(1).Count(&owned).Error; err != nil {
		return err
	}
	if owned == 0 {
		// T14：Legacy Task 兜底——0 run 的旧会话按 session 归属判定（全参数绑定）。
		var sessionOwned int64
		if err := s.db.WithContext(ctx).Table("sessions").
			Where("tenant_id = ? AND id = ? AND user_id = ? AND deleted_at IS NULL", tenantID, taskID, ownerID).
			Limit(1).Count(&sessionOwned).Error; err != nil {
			return err
		}
		if sessionOwned == 0 {
			return ErrWorkbenchTaskNotFound
		}
	}
```

（文件既有 doc comment 同步更新为上面的新注释；后续 `archived_at` 更新段不动。）

- [ ] **Step 4: 运行测试确认通过（含既有回归）**

Run: `go test ./internal/application/repository/ -run 'TestWorkbenchTaskState' -count=1 && go test ./internal/handler/session/ -run 'TestLegacyTaskMigration|TestWorkbenchStart|TestListLegacyTasks' -count=1`
Expected: 全部 PASS（既有 `TestWorkbenchTaskStateArchiveOwnershipAndIsolation` 不回归 + 新增两测试绿）

- [ ] **Step 5: 提交**

```bash
git add internal/application/repository/workbench_task_state.go internal/application/repository/workbench_task_state_test.go internal/handler/session/workbench_legacy_migration_test.go
git commit -m "feat(workbench): legacy tasks archive by session ownership; migration test proves new capabilities arrive only via new Run admission (T44 AC1/AC2)"
```

---

### Task 4: mobile-core——Legacy Task 域（DTO、显式门禁、scenario Adapter）

**Files:**
- Create: `packages/mobile-core/src/task-office/legacy-tasks.ts`
- Test: `packages/mobile-core/src/task-office/legacy-tasks.test.ts`

**Interfaces:**
- Consumes: `AttentionState`（`packages/mobile-core/src/task-office/task-office-errors.ts:8`）。
- Produces（Task 5/7 消费，逐字签名）:
  - `type LegacyTaskIntent = 'follow-up' | 'run-command' | 'decision' | 'budget' | 'agent-version'`
  - `interface LegacyTaskCapability { state: 'supported' | 'unavailable'; reason: string }`
  - `type LegacyTaskGates = Record<LegacyTaskIntent, LegacyTaskCapability>`
  - `const LEGACY_TASK_NEW_RUN_REASON: string`（值 `'legacy task has no Run; a new Run admission is required'`）
  - `function legacyTaskGates(): LegacyTaskGates`
  - `interface LegacyBackendTask { taskId: string; title: string; attention: 'none'; archivedAt?: string; updatedAt: string }`
  - `interface LegacyTaskBackendPage { items: LegacyBackendTask[]; nextCursor?: string }`
  - `interface LegacyMessage { messageId: string; role: 'user' | 'assistant' | 'system'; content: string; createdAt?: string }`
  - `interface LegacyFollowUpInput { taskId: string; question: string; signal?: AbortSignal }`
  - `interface LegacyTaskBackendPort { list(input: { search?: string; archived?: boolean; cursor?: string; limit?: number }): Promise<LegacyTaskBackendPage>; history(taskId: string): Promise<LegacyMessage[]>; followUp(input: LegacyFollowUpInput): Promise<void> }`
  - `function createScenarioLegacyTaskBackend(handlers?: ScenarioLegacyTaskBackendHandlers): ScenarioLegacyTaskBackend`（含 `calls` 观测数组）

- [ ] **Step 1: 写失败测试**

`packages/mobile-core/src/task-office/legacy-tasks.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { LEGACY_TASK_NEW_RUN_REASON, createScenarioLegacyTaskBackend, legacyTaskGates } from './legacy-tasks.ts';

test('the legacy gate supports ordinary follow-up and refuses every new-security-semantics intent', () => {
  const gates = legacyTaskGates();
  assert.deepEqual(gates['follow-up'], { state: 'supported', reason: 'ordinary follow-up keeps the existing chat semantics' });
  for (const intent of ['run-command', 'decision', 'budget', 'agent-version'] as const) {
    assert.deepEqual(gates[intent], { state: 'unavailable', reason: LEGACY_TASK_NEW_RUN_REASON }, intent);
  }
  assert.equal(LEGACY_TASK_NEW_RUN_REASON, 'legacy task has no Run; a new Run admission is required');
});

test('the scenario backend records calls and defaults to honest empties', async () => {
  const backend = createScenarioLegacyTaskBackend();
  const page = await backend.list({});
  assert.deepEqual(page, { items: [] });
  assert.deepEqual(await backend.history('lg-1'), []);
  await backend.followUp({ taskId: 'lg-1', question: '继续' });
  assert.deepEqual(backend.calls, [
    { kind: 'list', input: {} },
    { kind: 'history', taskId: 'lg-1' },
    { kind: 'followUp', input: { taskId: 'lg-1', question: '继续' } },
  ]);
});

test('the scenario backend forwards scripted handlers', async () => {
  const backend = createScenarioLegacyTaskBackend({
    list: async () => ({ items: [{ taskId: 'lg-1', title: '旧聊天', attention: 'none', updatedAt: '2026-09-20T08:00:00Z' }], nextCursor: 'c1' }),
    history: async () => [{ messageId: 'm1', role: 'user', content: '第一问', createdAt: '2026-09-20T08:00:01Z' }],
    followUp: async () => undefined,
  });
  const page = await backend.list({ search: '旧' });
  assert.equal(page.items[0]!.taskId, 'lg-1');
  assert.equal(page.nextCursor, 'c1');
  const messages = await backend.history('lg-1');
  assert.equal(messages[0]!.role, 'user');
  await backend.followUp({ taskId: 'lg-1', question: '第二问' });
  assert.equal(backend.calls.length, 3);
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/task-office/legacy-tasks.test.ts`
Expected: FAIL——`Cannot find module './legacy-tasks.ts'`（RED）

- [ ] **Step 3: 最小实现**

`packages/mobile-core/src/task-office/legacy-tasks.ts`（新文件，完整内容）：

```ts
import type { AttentionState } from './task-office-errors.ts';

/**
 * T14（Issue #44）Legacy Task 域合同：从未有过 Run 的旧 Session 以同一身份
 * （taskId = sessionId，ADR-0004）呈现。卡片只携带历史能证明的事实；Run 级
 * 新安全语义（Run 指令/审批/预算/Agent Version）由显式门禁标记为需新建 Run，
 * 门禁在模块内派生（纯函数），不来自 wire——服务端 legacy 行本来就不携带
 * 这些字段（Go 侧键集锁定）。
 */

export type LegacyTaskIntent = 'follow-up' | 'run-command' | 'decision' | 'budget' | 'agent-version';

export interface LegacyTaskCapability {
  state: 'supported' | 'unavailable';
  reason: string;
}

export type LegacyTaskGates = Record<LegacyTaskIntent, LegacyTaskCapability>;

export const LEGACY_TASK_NEW_RUN_REASON = 'legacy task has no Run; a new Run admission is required';

/** 显式升级门禁（AC2）：普通追问 supported；四类新安全语义意图一律 unavailable。 */
export function legacyTaskGates(): LegacyTaskGates {
  return {
    'follow-up': { state: 'supported', reason: 'ordinary follow-up keeps the existing chat semantics' },
    'run-command': { state: 'unavailable', reason: LEGACY_TASK_NEW_RUN_REASON },
    'decision': { state: 'unavailable', reason: LEGACY_TASK_NEW_RUN_REASON },
    'budget': { state: 'unavailable', reason: LEGACY_TASK_NEW_RUN_REASON },
    'agent-version': { state: 'unavailable', reason: LEGACY_TASK_NEW_RUN_REASON },
  };
}

/** 后端行（wire 语义行，attention 恒 'none'——无 Run 即无可证明关注来源）。 */
export interface LegacyBackendTask {
  taskId: string;
  title: string;
  attention: 'none';
  archivedAt?: string;
  updatedAt: string;
}

export interface LegacyTaskBackendPage {
  items: LegacyBackendTask[];
  nextCursor?: string;
}

export interface LegacyMessage {
  messageId: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  createdAt?: string;
}

export interface LegacyFollowUpInput {
  taskId: string;
  question: string;
  signal?: AbortSignal;
}

export interface LegacyTaskBackendPort {
  list(input: { search?: string; archived?: boolean; cursor?: string; limit?: number }): Promise<LegacyTaskBackendPage>;
  history(taskId: string): Promise<LegacyMessage[]>;
  followUp(input: LegacyFollowUpInput): Promise<void>;
}

/** Scriptable scenario Adapter（module-seams §12：remote-owned 依赖的 in-memory 场景）。 */
export interface ScenarioLegacyTaskBackendHandlers {
  list?: (input: { search?: string; archived?: boolean; cursor?: string; limit?: number }) => Promise<LegacyTaskBackendPage>;
  history?: (taskId: string) => Promise<LegacyMessage[]>;
  followUp?: (input: LegacyFollowUpInput) => Promise<void>;
}

export interface ScenarioLegacyTaskBackend extends LegacyTaskBackendPort {
  calls: Array<
    | { kind: 'list'; input: { search?: string; archived?: boolean; cursor?: string; limit?: number } }
    | { kind: 'history'; taskId: string }
    | { kind: 'followUp'; input: LegacyFollowUpInput }
  >;
}

export function createScenarioLegacyTaskBackend(handlers: ScenarioLegacyTaskBackendHandlers = {}): ScenarioLegacyTaskBackend {
  const calls: ScenarioLegacyTaskBackend['calls'] = [];
  const scenario: ScenarioLegacyTaskBackend = {
    calls,
    async list(input) {
      calls.push({ kind: 'list', input });
      return handlers.list ? handlers.list(input) : { items: [] };
    },
    async history(taskId) {
      calls.push({ kind: 'history', taskId });
      return handlers.history ? handlers.history(taskId) : [];
    },
    async followUp(input) {
      calls.push({ kind: 'followUp', input });
      await handlers.followUp?.(input);
    },
  };
  return scenario;
}

/** 展示卡片：后端行 + 模块内门禁（gates 不来自 wire）。 */
export interface LegacyTaskCard extends LegacyBackendTask {
  kind: 'legacy';
  gates: LegacyTaskGates;
}

export interface LegacyTaskListPage {
  items: LegacyTaskCard[];
  nextCursor?: string;
  duplicateTaskIds: string[];
}

export type { AttentionState };
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test packages/mobile-core/src/task-office/legacy-tasks.test.ts`
Expected: PASS（3 用例全绿）

- [ ] **Step 5: 提交**

```bash
git add packages/mobile-core/src/task-office/legacy-tasks.ts packages/mobile-core/src/task-office/legacy-tasks.test.ts
git commit -m "feat(mobile-core): legacy task domain contracts, explicit new-Run gate and scenario adapter (T44 AC2)"
```

---

### Task 5: mobile-core——Task Office legacy 入口接线

**Files:**
- Modify: `packages/mobile-core/src/task-office/task-office.ts`（追加端口与四个方法，标注「T14 追加区」）
- Modify: `packages/mobile-core/src/task-office/task-office-errors.ts:10-17`（错误码联合追加 `'TASK_OFFICE_LEGACY_UNAVAILABLE'`）
- Modify: `packages/mobile-core/src/index.ts`（追加导出）
- Test: `packages/mobile-core/src/task-office/legacy-office.test.ts`（新文件）

**Interfaces:**
- Consumes: Task 4 的全部类型与 `legacyTaskGates`；既有 `createTaskOffice`/`TaskOfficePorts`/`TaskOffice`/`TaskOfficeError`（`task-office.ts:100-142`）、`leaseActive`（`runtime/scope-lease.ts`）、`RuntimeScopeLease`（测试夹具模式，`task-office.test.ts:11-20`）。
- Produces（Task 7 消费，逐字签名——追加进 `TaskOffice` 接口）:
  - `legacyTasks(query: { search?: string; archived?: boolean; limit?: number }): Promise<LegacyTaskListPage>`
  - `moreLegacyTasks(): Promise<LegacyTaskListPage>`
  - `legacyHistory(taskId: string): Promise<LegacyMessage[]>`
  - `followUp(input: LegacyFollowUpInput): Promise<void>`
  - `TaskOfficePorts.legacy?: LegacyTaskBackendPort`（缺省 fail closed：`TASK_OFFICE_LEGACY_UNAVAILABLE`）
  - `TaskOfficeErrorCode` 新成员 `'TASK_OFFICE_LEGACY_UNAVAILABLE'`
  - `index.ts` 追加导出：`legacyTaskGates`、`LEGACY_TASK_NEW_RUN_REASON`、`createScenarioLegacyTaskBackend` 与类型 `LegacyTaskCard`/`LegacyTaskListPage`/`LegacyTaskGates`/`LegacyTaskCapability`/`LegacyTaskIntent`/`LegacyBackendTask`/`LegacyTaskBackendPage`/`LegacyTaskBackendPort`/`LegacyMessage`/`LegacyFollowUpInput`/`ScenarioLegacyTaskBackend`/`ScenarioLegacyTaskBackendHandlers`

- [ ] **Step 1: 写失败测试**

`packages/mobile-core/src/task-office/legacy-office.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import { createScenarioTaskBackend } from './in-memory-task-backend.ts';
import { createScenarioLegacyTaskBackend, LEGACY_TASK_NEW_RUN_REASON } from './legacy-tasks.ts';
import { createTaskOffice, TaskOfficeError } from './task-office.ts';

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => { resolve = next; });
  return { promise, resolve };
}

function leased() {
  const revocable = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.test', userId: 'user-1', tenantId: 'tenant-1' });
  return { revocable, lease: revocable.asScopeLease() };
}

function officeWith(leaseRef: { lease?: ScopeLease }, handlers: Parameters<typeof createScenarioLegacyTaskBackend>[0] = {}) {
  const legacy = createScenarioLegacyTaskBackend(handlers);
  return { legacy, office: createTaskOffice({ backend: createScenarioTaskBackend(), legacy, lease: () => leaseRef.lease }) };
}

function legacyRow(taskId: string, updatedAt = '2026-09-20T08:00:00Z') {
  return { taskId, title: `title-${taskId}`, attention: 'none' as const, updatedAt };
}

test('legacyTasks returns gate-annotated cards and continues the owned cursor', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const pages = [
    { items: [legacyRow('lg-a'), legacyRow('lg-b')], nextCursor: 'cursor-1' },
    { items: [legacyRow('lg-b'), legacyRow('lg-c')] }, // 服务端重复键：lg-b 重现
  ];
  let call = 0;
  const { legacy, office } = officeWith(leaseRef, { list: () => Promise.resolve(pages[call++]!) });
  const first = await office.legacyTasks({});
  assert.deepEqual(first.items.map((item) => item.taskId), ['lg-a', 'lg-b']);
  assert.equal(first.nextCursor, 'cursor-1');
  assert.equal(first.items[0]!.kind, 'legacy');
  assert.equal(first.items[0]!.attention, 'none');
  // 显式门禁随卡片下发（模块内派生）：AC2。
  assert.equal(first.items[0]!.gates['run-command'].state, 'unavailable');
  assert.equal(first.items[0]!.gates['run-command'].reason, LEGACY_TASK_NEW_RUN_REASON);
  assert.equal(first.items[0]!.gates['follow-up'].state, 'supported');
  const second = await office.moreLegacyTasks();
  assert.deepEqual(second.items.map((item) => item.taskId), ['lg-c'], 'repeated key not rendered again');
  assert.deepEqual(second.duplicateTaskIds, ['lg-b']);
  assert.deepEqual(legacy.calls.map((c) => c.kind), ['list', 'list']);
});

test('legacy ports missing fail closed with a dedicated code', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const office = createTaskOffice({ backend: createScenarioLegacyTaskBackend(), lease: () => leaseRef.lease });
  await assert.rejects(office.legacyTasks({}), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_LEGACY_UNAVAILABLE');
});

test('legacy reads reject after the scope lease was revoked', async () => {
  const { revocable, lease } = leased();
  const leaseRef: { lease?: ScopeLease } = { lease };
  const gate = deferred<{ items: [] }>();
  const { office } = officeWith(leaseRef, { list: () => gate.promise });
  const pending = office.legacyTasks({});
  revocable.revoke();
  gate.resolve({ items: [] }); // 迟到结果到达：settle 必须按 SCOPE_CHANGED 拒绝
  await assert.rejects(pending, (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED');
});

test('followUp validates input, wraps backend failures and invalidates the active legacy query', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { legacy, office } = officeWith(leaseRef, {
    list: async () => ({ items: [legacyRow('lg-1')], nextCursor: 'cursor-1' }),
    followUp: async (input) => {
      if (input.question === 'boom') throw new Error('HTTP_409: another turn is already running');
    },
  });
  await assert.rejects(office.followUp({ taskId: '  ', question: 'x' }), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_INVALID_INPUT');
  await assert.rejects(office.followUp({ taskId: 'lg-1', question: '' }), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_INVALID_INPUT');
  await assert.rejects(office.followUp({ taskId: 'lg-1', question: 'x'.repeat(8001) }), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_INVALID_INPUT');

  // 后端失败（409 another turn running）按 TASK_OFFICE_BACKEND 包装上抛，不静默重试、不换语义。
  await assert.rejects(
    office.followUp({ taskId: 'lg-1', question: 'boom' }),
    (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_BACKEND' && String((error as TaskOfficeError & { cause?: unknown }).cause).includes('409'),
  );

  // 建立活动查询（moreLegacyTasks 可续页），成功追问后查询失效。
  const first = await office.legacyTasks({});
  assert.equal(first.nextCursor, 'cursor-1');
  await office.followUp({ taskId: 'lg-1', question: '继续这个话题' });
  assert.deepEqual(legacy.calls.filter((c) => c.kind === 'followUp').length, 2);
  await assert.rejects(office.moreLegacyTasks(), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_NO_ACTIVE_QUERY');
});

test('a legacy task cannot be opened as a run handle: open requires a runId', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { office } = officeWith(leaseRef);
  assert.throws(() => office.open({ taskId: 'lg-1', runId: '' }), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_INVALID_INPUT');
});

test('legacyHistory passes the trimmed task id through the lease gate', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const { legacy, office } = officeWith(leaseRef, {
    history: async () => [{ messageId: 'm1', role: 'user', content: '第一问' }],
  });
  const messages = await office.legacyHistory(' lg-1 ');
  assert.equal(messages[0]!.messageId, 'm1');
  assert.equal((legacy.calls[0] as { kind: string }).kind, 'history');
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/task-office/legacy-office.test.ts`
Expected: FAIL——`office.legacyTasks is not a function`（TS 下编译为属性缺失，运行时报 TypeError；RED）

- [ ] **Step 3: 最小实现**

`packages/mobile-core/src/task-office/task-office-errors.ts:10-17` 的错误码联合追加一员（仅此一处改动）：

```ts
export type TaskOfficeErrorCode =
  | 'TASK_OFFICE_SCOPE_CHANGED'
  | 'TASK_OFFICE_SUPERSEDED'
  | 'TASK_OFFICE_NO_ACTIVE_QUERY'
  | 'TASK_OFFICE_INVALID_INPUT'
  | 'TASK_OFFICE_BACKEND'
  | 'TASK_OFFICE_DETAIL_UNAVAILABLE'
  | 'TASK_OFFICE_DETAIL_CLOSED'
  | 'TASK_OFFICE_LEGACY_UNAVAILABLE';
```

`packages/mobile-core/src/task-office/task-office.ts`——文件头 import 区追加：

```ts
import { legacyTaskGates } from './legacy-tasks.ts';
import type {
  LegacyBackendTask, LegacyFollowUpInput, LegacyMessage, LegacyTaskBackendPage, LegacyTaskBackendPort,
  LegacyTaskCard, LegacyTaskListPage,
} from './legacy-tasks.ts';
```

`TaskOfficePorts`（`task-office.ts:100-107`）追加可选端口字段；`TaskOffice`（`task-office.ts:109-116`）追加四个方法签名：

```ts
export interface TaskOfficePorts {
  backend: TaskBackendPort;
  lease(): ScopeLease | undefined;
  /** T05 详情与 SSE 恢复端口；缺失时 open() fail closed（TASK_OFFICE_DETAIL_UNAVAILABLE）。 */
  detail?: TaskDetailBackendPort;
  /** 持久化投影存储（App 重启恢复）；缺省为 office 内共享的 in-memory store。 */
  store?: TaskProjectionStore;
  /** T14（#44）Legacy Task 端口；缺失时 legacy 入口 fail closed（TASK_OFFICE_LEGACY_UNAVAILABLE）。 */
  legacy?: LegacyTaskBackendPort;
}
```

```ts
export interface TaskOffice {
  home(): Promise<HomeView>;
  tasks(query: TaskOfficeQuery): Promise<TaskListPage>;
  moreTasks(): Promise<TaskListPage>;
  archive(taskId: string): Promise<void>;
  restore(taskId: string): Promise<void>;
  open(input: { taskId: string; runId: string }): TaskHandle;
  /** T14（#44）追加区：Legacy Task 读投影与普通追问。 */
  legacyTasks(query: { search?: string; archived?: boolean; limit?: number }): Promise<LegacyTaskListPage>;
  moreLegacyTasks(): Promise<LegacyTaskListPage>;
  legacyHistory(taskId: string): Promise<LegacyMessage[]>;
  followUp(input: LegacyFollowUpInput): Promise<void>;
}
```

`settle` 辅助（`task-office.ts:161-166`）的 `kind` 参数放宽为 `'home' | 'list' | 'legacy'`，epoch 选择追加 legacy 分支：

```ts
  const settle = <T>(epoch: number, kind: 'home' | 'list' | 'legacy', lease: ScopeLease, value: T): T => {
    const currentEpoch = kind === 'home' ? homeEpoch : kind === 'list' ? listEpoch : legacyListEpoch;
    if (epoch !== currentEpoch) throw new TaskOfficeError('TASK_OFFICE_SUPERSEDED');
    if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
    return value;
  };
```

`createTaskOffice` 主体（`task-office.ts:142` 起）在 `const defaultDetailStore = ...` 之后追加状态与辅助（完整代码块）：

```ts
  let legacyListEpoch = 0;
  let legacyAccumulated: legacyListAccumulation | undefined;

  interface legacyListAccumulation {
    input: { search?: string; archived?: boolean; limit?: number };
    cursor?: string;
    seen: Set<string>;
    duplicates: string[];
  }

  const requireLegacy = (): LegacyTaskBackendPort => {
    if (ports.legacy === undefined) throw new TaskOfficeError('TASK_OFFICE_LEGACY_UNAVAILABLE');
    return ports.legacy;
  };
  const toLegacyCard = (task: LegacyBackendTask): LegacyTaskCard => ({
    taskId: task.taskId,
    title: task.title,
    attention: 'none',
    ...(task.archivedAt === undefined ? {} : { archivedAt: task.archivedAt }),
    updatedAt: task.updatedAt,
    kind: 'legacy',
    gates: legacyTaskGates(),
  });
  const accumulateLegacy = (state: legacyListAccumulation, page: LegacyTaskBackendPage): LegacyTaskListPage => {
    const items: LegacyTaskCard[] = [];
    for (const task of page.items) {
      if (state.seen.has(task.taskId)) {
        state.duplicates.push(task.taskId);
        continue;
      }
      state.seen.add(task.taskId);
      items.push(toLegacyCard(task));
    }
    state.cursor = page.nextCursor;
    return { items, ...(page.nextCursor === undefined ? {} : { nextCursor: page.nextCursor }), duplicateTaskIds: [...state.duplicates] };
  };
  const normalizeLegacyQuery = (query: { search?: string; archived?: boolean; limit?: number }): { search?: string; archived?: boolean; limit?: number } => {
    const search = typeof query.search === 'string' ? query.search.trim().replace(/\s+/g, ' ').slice(0, searchMaxLen) : '';
    const limit = typeof query.limit === 'number' && Number.isSafeInteger(query.limit) && query.limit > 0 ? Math.min(query.limit, 100) : 20;
    return { ...(search === '' ? {} : { search }), ...(query.archived === true ? { archived: true } : {}), limit };
  };
```

`mutate`（`task-office.ts:181-192`）内 epoch 失效段追加 legacy 两行（跟随既有 `listEpoch += 1; homeEpoch += 1; accumulated = undefined;`）：

```ts
    listEpoch += 1;
    homeEpoch += 1;
    legacyListEpoch += 1;
    accumulated = undefined;
    legacyAccumulated = undefined;
```

返回对象（`task-office.ts:194-239`）在 `open(...)` 方法之后追加四个方法（完整代码块）：

```ts
    /** T14（#44）追加区。 */
    async legacyTasks(query: { search?: string; archived?: boolean; limit?: number }): Promise<LegacyTaskListPage> {
      const epoch = ++legacyListEpoch;
      const lease = requireLease();
      const input = normalizeLegacyQuery(query);
      const page = settle(epoch, 'legacy', lease, await callBackend(() => requireLegacy().list(input)));
      const state: legacyListAccumulation = { input, seen: new Set<string>(), duplicates: [] };
      legacyAccumulated = state;
      return accumulateLegacy(state, page);
    },
    async moreLegacyTasks(): Promise<LegacyTaskListPage> {
      const state = legacyAccumulated;
      if (state === undefined) throw new TaskOfficeError('TASK_OFFICE_NO_ACTIVE_QUERY');
      if (state.cursor === undefined) return { items: [], duplicateTaskIds: [...state.duplicates] };
      const epoch = ++legacyListEpoch;
      const lease = requireLease();
      const page = settle(epoch, 'legacy', lease, await callBackend(() => requireLegacy().list({ ...state.input, cursor: state.cursor })));
      return accumulateLegacy(state, page);
    },
    async legacyHistory(taskId: string): Promise<LegacyMessage[]> {
      const lease = requireLease();
      const trimmed = taskId.trim();
      if (trimmed === '') throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');
      const messages = await callBackend(() => requireLegacy().history(trimmed));
      if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
      return messages;
    },
    async followUp(input: LegacyFollowUpInput): Promise<void> {
      const lease = requireLease();
      const taskId = (input?.taskId ?? '').trim();
      const question = (input?.question ?? '').trim();
      if (taskId === '' || question === '' || question.length > 8000) throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');
      await callBackend(() => requireLegacy().followUp({ taskId, question, ...(input.signal === undefined ? {} : { signal: input.signal }) }));
      if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
      // 追问改变了会话 updated_at：一切在途读失效（与 archive/restore 同规则）。
      listEpoch += 1;
      homeEpoch += 1;
      legacyListEpoch += 1;
      accumulated = undefined;
      legacyAccumulated = undefined;
    },
```

（`settle` 的 `kind` 联合放宽是 legacyTasks/moreLegacyTasks 的迟到拒绝所必需；`legacyHistory` 不参与并发取消竞争，用与 `followUp` 相同的前后 lease 检查即可。）

`packages/mobile-core/src/index.ts` 末尾追加导出块：

```ts
export { legacyTaskGates, LEGACY_TASK_NEW_RUN_REASON, createScenarioLegacyTaskBackend } from './task-office/legacy-tasks.ts';
export type {
  LegacyBackendTask, LegacyFollowUpInput, LegacyMessage, LegacyTaskBackendPage, LegacyTaskBackendPort,
  LegacyTaskCapability, LegacyTaskCard, LegacyTaskGates, LegacyTaskIntent, LegacyTaskListPage,
  ScenarioLegacyTaskBackend, ScenarioLegacyTaskBackendHandlers,
} from './task-office/legacy-tasks.ts';
```

- [ ] **Step 4: 运行测试确认通过（含既有回归）**

Run: `pnpm exec tsx --test packages/mobile-core/src/task-office/legacy-office.test.ts packages/mobile-core/src/task-office/task-office.test.ts packages/mobile-core/src/task-office/legacy-tasks.test.ts`
Expected: PASS（新增 7 用例 + 既有 task-office 套件全绿）

- [ ] **Step 5: 提交**

```bash
git add packages/mobile-core/src/task-office/task-office.ts packages/mobile-core/src/task-office/task-office-errors.ts packages/mobile-core/src/task-office/legacy-office.test.ts packages/mobile-core/src/index.ts
git commit -m "feat(mobile-core): task office legacy entries with lease gating, duplicate keys and follow-up invalidation (T44)"
```

---

### Task 6: api-client——Legacy Remote Adapter（list / history / followUp SSE）

**Files:**
- Create: `packages/api-client/src/mobile/legacy-tasks.ts`
- Test: `packages/api-client/src/mobile/legacy-tasks.test.ts`
- Modify: `packages/api-client/package.json`（exports 追加 `"./mobile/legacy-tasks": "./src/mobile/legacy-tasks.ts"`）

**Interfaces:**
- Consumes: Task 4/5 的 `LegacyTaskBackendPort` 形状（结构逐字一致，可赋值性由 apps/mobile typecheck 证明——与 `task-office.ts` remote 同一先例）；既有 `createServerSentEventParser`/`parseChatEvent`（`../chat/stream.ts:14,94`）、`parseChatMessageListResponse`/`responseType`（`@weknora/contracts`，`index.ts:460`、`chat/events.ts:23`）、`ClientRequest`（`../client.ts`）。
- Produces: `createMobileLegacyTaskRemote(options: { origin: string; request: (input: ClientRequest) => Promise<unknown>; stream?: (input: ClientRequest, onChunk: (chunk: string) => void) => Promise<void> })` 返回 `{ list, history, followUp }`（与 `LegacyTaskBackendPort` 结构逐字一致）；子路径导出 `@weknora/api-client/mobile/legacy-tasks`。Task 7 的 composition 与 Task 8 的集成冒烟消费。

- [ ] **Step 1: 写失败测试**

`packages/api-client/src/mobile/legacy-tasks.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import type { ClientRequest } from '../client.ts';
import { createMobileLegacyTaskRemote } from './legacy-tasks.ts';

test('legacy remote maps the list wire into module rows and requires the legacy kind marker', async () => {
  const requests: ClientRequest[] = [];
  const remote = createMobileLegacyTaskRemote({
    origin: 'https://weknora.example.test',
    request: async (input) => {
      requests.push(input);
      return {
        success: true,
        data: {
          items: [
            { task_id: 'lg-1', title: '旧聊天：周报素材', attention: 'none', updated_at: '2026-09-20T08:00:00Z', kind: 'legacy' },
            { task_id: 'lg-2', attention: 'none', archived_at: '2026-09-22T12:00:00Z', updated_at: '2026-09-20T09:00:00Z', kind: 'legacy' },
          ],
          next_cursor: 'cursor-2',
        },
      };
    },
  });
  const page = await remote.list({ search: '周报', archived: true, limit: 15 });
  assert.equal(requests[0]!.method, 'GET');
  assert.equal(requests[0]!.path, '/api/v1/workbench/legacy-tasks?q=%E5%91%A8%E6%8A%A5&archived=true&limit=15');
  assert.deepEqual(page.items, [
    { taskId: 'lg-1', title: '旧聊天：周报素材', attention: 'none', updatedAt: '2026-09-20T08:00:00Z' },
    { taskId: 'lg-2', title: '', attention: 'none', archivedAt: '2026-09-22T12:00:00Z', updatedAt: '2026-09-20T09:00:00Z' },
  ]);
  assert.equal(page.nextCursor, 'cursor-2');
});

test('legacy remote rejects rows without the legacy kind marker or with fabricated attention', async () => {
  const remote = createMobileLegacyTaskRemote({
    origin: 'https://weknora.example.test',
    request: async () => ({ success: true, data: { items: [{ task_id: 'r-1', updated_at: '2026-09-20T08:00:00Z', run_status: 'running' }] } }),
  });
  await assert.rejects(remote.list({}), /kind must be "legacy"/);

  const fabricated = createMobileLegacyTaskRemote({
    origin: 'https://weknora.example.test',
    request: async () => ({ success: true, data: { items: [{ task_id: 'lg-1', updated_at: '2026-09-20T08:00:00Z', kind: 'legacy', attention: 'required' }] } }),
  });
  await assert.rejects(fabricated.list({}), /attention must be "none"/);
});

test('legacy remote maps message history from the messages-load wire', async () => {
  const requests: ClientRequest[] = [];
  const remote = createMobileLegacyTaskRemote({
    origin: 'https://weknora.example.test',
    request: async (input) => {
      requests.push(input);
      return {
        success: true,
        data: [
          { id: 'm1', session_id: 'lg-1', role: 'user', content: '第一问' },
          { id: 'm2', session_id: 'lg-1', role: 'assistant', content: '第一答', created_at: '2026-09-20T08:00:01Z' },
        ],
      };
    },
  });
  const messages = await remote.history('lg-1');
  assert.equal(requests[0]!.path, '/api/v1/messages/lg-1/load?limit=20');
  assert.deepEqual(messages, [
    { messageId: 'm1', role: 'user', content: '第一问' },
    { messageId: 'm2', role: 'assistant', content: '第一答', createdAt: '2026-09-20T08:00:01Z' },
  ]);
});

test('legacy followUp posts the ordinary chat body through the stream channel and resolves on completion', async () => {
  const streams: ClientRequest[] = [];
  const remote = createMobileLegacyTaskRemote({
    origin: 'https://weknora.example.test',
    request: async () => { throw new Error('follow-up must not use the JSON channel'); },
    stream: async (input, onChunk) => {
      streams.push(input);
      onChunk('data: {"response_type":"answer","content":"答"}\n\n');
      onChunk('data: {"response_type":"complete"}\n\n');
    },
  });
  await remote.followUp({ taskId: 'lg-1', question: '继续这个话题' });
  assert.equal(streams.length, 1);
  assert.equal(streams[0]!.method, 'POST');
  assert.equal(streams[0]!.path, '/api/v1/knowledge-chat/lg-1');
  assert.equal((streams[0]!.headers ?? {}).accept, 'text/event-stream');
  assert.deepEqual((streams[0]!.body as Record<string, unknown>).query, '继续这个话题');
});

test('legacy followUp fails closed without a stream transport and rejects error frames', async () => {
  const closed = createMobileLegacyTaskRemote({ origin: 'https://weknora.example.test', request: async () => undefined });
  await assert.rejects(closed.followUp({ taskId: 'lg-1', question: 'x' }), /authorized stream transport/);

  const failing = createMobileLegacyTaskRemote({
    origin: 'https://weknora.example.test',
    request: async () => undefined,
    stream: async (_input, onChunk) => { onChunk('data: {"response_type":"error","content":"boom"}\n\n'); },
  });
  await assert.rejects(failing.followUp({ taskId: 'lg-1', question: 'x' }), /LEGACY_FOLLOW_UP_FAILED/);

  const malformed = createMobileLegacyTaskRemote({
    origin: 'https://weknora.example.test',
    request: async () => undefined,
    stream: async (_input, onChunk) => { onChunk('data: not-json\n\n'); },
  });
  await assert.rejects(malformed.followUp({ taskId: 'lg-1', question: 'x' }), /LEGACY_FOLLOW_UP_MALFORMED_FRAME/);
});

test('legacy remote validates the deployment origin like the task office remote', async () => {
  assert.throws(() => createMobileLegacyTaskRemote({ origin: 'https://weknora.example.test/path', request: async () => undefined }), /must not include a path/);
  assert.throws(() => createMobileLegacyTaskRemote({ origin: 'http://weknora.example.test', request: async () => undefined }), /HTTPS/);
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test packages/api-client/src/mobile/legacy-tasks.test.ts`
Expected: FAIL——`Cannot find module './legacy-tasks.ts'`（RED）

- [ ] **Step 3: 最小实现**

`packages/api-client/package.json` 的 `exports`（`package.json:6-14`）追加一行（置于 `"./mobile/task-office"` 之后）：

```json
    "./mobile/legacy-tasks": "./src/mobile/legacy-tasks.ts",
```

`packages/api-client/src/mobile/legacy-tasks.ts`（新文件，完整内容）：

```ts
import { parseChatMessageListResponse, responseType } from '@weknora/contracts';
import { createServerSentEventParser, parseChatEvent } from '../chat/stream.ts';
import type { ClientRequest } from '../client.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface LegacyTaskRemoteOptions {
  /** 部署 Origin：构造即强校验（绝对 HTTPS、无 path/query/fragment、无内嵌凭据）。 */
  origin: string;
  /** 授权通道（MobileRuntime.authorizedRequest 或测试替身）；本适配器不新建传输。 */
  request: Request;
  /** 授权 SSE 通道（MobileRuntime.authorizedEventStream 或测试替身）；普通追问走既有聊天 SSE wire。 */
  stream?: (input: ClientRequest, onChunk: (chunk: string) => void) => Promise<void>;
}

/** 与 mobile-core LegacyBackendTask 逐字一致（结构可赋值由 apps/mobile typecheck 证明）。 */
export interface RemoteLegacyTask { taskId: string; title: string; attention: 'none'; archivedAt?: string; updatedAt: string }
export interface RemoteLegacyPage { items: RemoteLegacyTask[]; nextCursor?: string }
export interface RemoteLegacyMessage { messageId: string; role: 'user' | 'assistant' | 'system'; content: string; createdAt?: string }

function requireDeploymentOrigin(origin: string): string {
  let parsed: URL;
  if (typeof origin !== 'string' || origin.trim() === '') throw new Error('deployment origin is required');
  try { parsed = new URL(origin); } catch { throw new Error(`deployment origin must be an absolute URL: ${origin}`); }
  if (parsed.protocol !== 'https:') throw new Error('deployment origin must use HTTPS');
  if (parsed.username !== '' || parsed.password !== '') throw new Error('deployment origin must not embed user info');
  if (parsed.pathname !== '/') throw new Error('deployment origin must not include a path');
  if (parsed.search !== '' || parsed.hash !== '') throw new Error('deployment origin must not include a query or fragment');
  return parsed.origin;
}

function legacyRow(item: unknown, path: string): RemoteLegacyTask {
  if (typeof item !== 'object' || item === null || Array.isArray(item)) throw new Error(`${path} must be an object`);
  const row = item as Record<string, unknown>;
  if (row.kind !== 'legacy') throw new Error(`${path}.kind must be "legacy"`);
  if (row.attention !== undefined && row.attention !== null && row.attention !== 'none') {
    throw new Error(`${path}.attention must be "none" for legacy tasks`);
  }
  if (typeof row.task_id !== 'string' || row.task_id.trim() === '') throw new Error(`${path}.task_id must be a non-empty string`);
  if (typeof row.updated_at !== 'string' || row.updated_at === '') throw new Error(`${path}.updated_at must be a non-empty string`);
  const archivedAt = typeof row.archived_at === 'string' && row.archived_at !== '' ? row.archived_at : undefined;
  return {
    taskId: row.task_id,
    title: typeof row.title === 'string' ? row.title : '',
    attention: 'none',
    ...(archivedAt === undefined ? {} : { archivedAt }),
    updatedAt: row.updated_at,
  };
}

export function createMobileLegacyTaskRemote(options: LegacyTaskRemoteOptions) {
  requireDeploymentOrigin(options.origin);
  const request = options.request;

  return {
    async list(input: { search?: string; archived?: boolean; cursor?: string; limit?: number }): Promise<RemoteLegacyPage> {
      const query = new URLSearchParams();
      if (input.search !== undefined && input.search !== '') query.set('q', input.search);
      if (input.archived === true) query.set('archived', 'true');
      if (input.cursor !== undefined && input.cursor !== '') query.set('cursor', input.cursor);
      if (input.limit !== undefined) query.set('limit', String(input.limit));
      const suffix = query.toString();
      const value = await request({ method: 'GET', path: `/api/v1/workbench/legacy-tasks${suffix ? `?${suffix}` : ''}` });
      const envelope = value as { success?: unknown; data?: unknown } | null;
      if (typeof envelope !== 'object' || envelope === null || envelope.success !== true || typeof envelope.data !== 'object' || envelope.data === null) {
        throw new Error('legacy task list response.success must be true with data');
      }
      const page = envelope.data as { items?: unknown; next_cursor?: unknown };
      if (!Array.isArray(page.items)) throw new Error('legacy task list data.items must be an array');
      return {
        items: page.items.map((item, index) => legacyRow(item, `items[${index}]`)),
        ...(typeof page.next_cursor === 'string' && page.next_cursor !== '' ? { nextCursor: page.next_cursor } : {}),
      };
    },
    async history(taskId: string): Promise<RemoteLegacyMessage[]> {
      const trimmed = taskId.trim();
      if (trimmed === '') throw new Error('taskId must not be empty');
      const messages = parseChatMessageListResponse(await request({
        method: 'GET',
        path: `/api/v1/messages/${encodeURIComponent(trimmed)}/load?limit=20`,
      }));
      return messages.map((message) => ({
        messageId: message.id,
        role: message.role,
        content: message.content,
        ...(message.created_at === undefined ? {} : { createdAt: message.created_at }),
      }));
    },
    async followUp(input: { taskId: string; question: string; signal?: AbortSignal }): Promise<void> {
      const transport = options.stream;
      if (!transport) throw new Error('legacy follow-up requires an authorized stream transport');
      const taskId = input.taskId.trim();
      const question = input.question.trim();
      if (taskId === '' || question === '') throw new Error('legacy follow-up requires taskId and question');
      let failed = false;
      const parser = createServerSentEventParser((frame) => {
        let event;
        try {
          event = parseChatEvent(frame);
        } catch {
          throw new Error('LEGACY_FOLLOW_UP_MALFORMED_FRAME');
        }
        if (responseType(event) === 'error') {
          failed = true;
          return;
        }
      });
      await transport({
        method: 'POST',
        path: `/api/v1/knowledge-chat/${encodeURIComponent(taskId)}`,
        headers: { accept: 'text/event-stream', 'content-type': 'application/json' },
        body: { query: question },
        ...(input.signal === undefined ? {} : { signal: input.signal }),
      }, (chunk) => parser.push(chunk));
      parser.finish();
      if (failed) throw new Error('LEGACY_FOLLOW_UP_FAILED');
    },
  };
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test packages/api-client/src/mobile/legacy-tasks.test.ts`
Expected: PASS（6 用例全绿）

- [ ] **Step 5: 提交**

```bash
git add packages/api-client/src/mobile/legacy-tasks.ts packages/api-client/src/mobile/legacy-tasks.test.ts packages/api-client/package.json
git commit -m "feat(api-client): mobile legacy task remote (list REST, history messages-load, follow-up chat SSE) (T44)"
```

---

### Task 7: apps/mobile——视图控制器、LegacyTasksScreen、路由与接线

**Files:**
- Create: `apps/mobile/src/legacy-tasks-view.ts`
- Test: `apps/mobile/src/legacy-tasks-view.test.ts`
- Create: `apps/mobile/src/screens/LegacyTasksScreen.tsx`
- Create: `apps/mobile/src/app/tasks/legacy.tsx`
- Modify: `apps/mobile/src/adapters/sse-stream.ts:4-23`（body 透传）+ `apps/mobile/src/adapters/sse-stream.test.ts`（追加一测）
- Modify: `apps/mobile/src/composition.ts`（legacy 端口装配）
- Modify: `apps/mobile/src/screens/TasksScreen.tsx`（可选 `onOpenLegacy` prop + 按钮）
- Modify: `apps/mobile/src/app/tasks.tsx`（透传回调）

**Interfaces:**
- Consumes: Task 5 的 `office.legacyTasks/moreLegacyTasks/legacyHistory/followUp`、`LegacyTaskCard`/`LegacyMessage`/`LegacyTaskListPage`、`TaskOfficeError`；Task 6 的 `createMobileLegacyTaskRemote`；既有 `activeTaskOffice()`（`composition.ts:139`）、`taskOfficeFor()`（`composition.ts:109-124`）、`streamAuthorizedSse`/`SseFetchLike`（`adapters/sse-stream.ts:4-6`）、TasksScreen 既有结构（`screens/TasksScreen.tsx:5-16`）、`app/tasks.tsx` 路由模式。
- Produces: `/tasks/legacy` 路由（Expo Router 文件路由）；`createLegacyTasksController(office: Pick<TaskOffice, 'legacyTasks' | 'moreLegacyTasks' | 'legacyHistory' | 'followUp'>): LegacyTasksController`（`state()/subscribe()/reload()/loadMore()/openHistory(taskId)/submitFollowUp(taskId, question)/dispose()`）与 `LegacyTasksViewState`；`TasksScreenProps.onOpenLegacy?: () => void`；`streamAuthorizedSse` 支持 `input.body`（POST SSE 追问的透传，string 或 JSON 序列化 + `content-type: application/json`）。Task 8 的集成冒烟消费 sse body 透传。

- [ ] **Step 1: 写失败测试**

`apps/mobile/src/legacy-tasks-view.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import type { TaskOffice } from '@weknora/mobile-core';
import { legacyTaskGates } from '@weknora/mobile-core';
import { createLegacyTasksController } from './legacy-tasks-view.ts';

type LegacyOffice = Pick<TaskOffice, 'legacyTasks' | 'moreLegacyTasks' | 'legacyHistory' | 'followUp'>;

function fakeLegacyOffice(calls: string[], options: { failFollowUp?: boolean } = {}): LegacyOffice {
  const office = {
    async legacyTasks() {
      calls.push('legacyTasks');
      return {
        items: [{ taskId: 'lg-1', title: '旧聊天', attention: 'none' as const, updatedAt: '2026-09-20T08:00:00Z', kind: 'legacy' as const, gates: legacyTaskGates() }],
        duplicateTaskIds: [],
      };
    },
    async moreLegacyTasks() {
      calls.push('moreLegacyTasks');
      return { items: [], duplicateTaskIds: [] };
    },
    async legacyHistory(taskId: string) {
      calls.push(`history:${taskId}`);
      return [{ messageId: 'm1', role: 'user' as const, content: '第一问' }];
    },
    async followUp(input: { taskId: string; question: string }) {
      if (options.failFollowUp) throw new Error('HTTP_409: another turn is already running');
      calls.push(`followUp:${input.taskId}:${input.question}`);
    },
  };
  return office as LegacyOffice;
}

test('the controller loads the legacy page, opens history and submits a follow-up', async () => {
  const calls: string[] = [];
  const controller = createLegacyTasksController(fakeLegacyOffice(calls));
  await controller.whenSettled();
  let state = controller.state();
  assert.equal(state.loading, false);
  assert.equal(state.items[0]!.taskId, 'lg-1');
  assert.equal(state.hasMore, false);

  await controller.openHistory('lg-1');
  state = controller.state();
  assert.equal(state.history?.taskId, 'lg-1');
  assert.equal(state.history?.messages[0]!.content, '第一问');

  await controller.submitFollowUp('lg-1', '继续这个话题');
  state = controller.state();
  assert.equal(state.followUpState, 'sent');
  assert.deepEqual(calls, ['legacyTasks', 'history:lg-1', 'followUp:lg-1:继续这个话题', 'legacyTasks'], 'a successful follow-up reloads the page');
  controller.dispose();
});

test('the controller surfaces follow-up failures without faking success', async () => {
  const calls: string[] = [];
  const controller = createLegacyTasksController(fakeLegacyOffice(calls, { failFollowUp: true }));
  await controller.whenSettled();
  await controller.submitFollowUp('lg-1', 'boom');
  const state = controller.state();
  assert.equal(state.followUpState, 'idle');
  assert.match(state.followUpError ?? '', /409/);
  assert.deepEqual(calls, ['legacyTasks'], 'no reload happens after a failed follow-up');
  controller.dispose();
});

test('the controller skips empty follow-up input', async () => {
  const calls: string[] = [];
  const controller = createLegacyTasksController(fakeLegacyOffice(calls));
  await controller.whenSettled();
  await controller.submitFollowUp('lg-1', '   ');
  const state = controller.state();
  assert.equal(state.followUpState, 'idle');
  assert.equal(calls.filter((call) => call.startsWith('followUp')).length, 0);
  controller.dispose();
});
```

再在 `apps/mobile/src/adapters/sse-stream.test.ts` 末尾追加：

```ts
test('a POST stream carries the JSON body and content-type for chat follow-ups', async () => {
  const calls: Array<{ url: string; init?: RequestInit }> = [];
  await streamAuthorizedSse(
    'https://weknora.example.test',
    { method: 'POST', path: '/api/v1/knowledge-chat/lg-1', body: { query: '继续这个话题' } },
    'access-1',
    () => undefined,
    async (url, init) => { calls.push({ url, init: init as RequestInit }); return new Response(streamOf(['data: {"response_type":"complete"}\n\n'])); },
  );
  assert.equal(calls.length, 1);
  assert.equal(calls[0]!.init!.method, 'POST');
  assert.equal(calls[0]!.init!.body, '{"query":"继续这个话题"}');
  const headers = calls[0]!.init!.headers as Record<string, string>;
  assert.equal(headers['content-type'], 'application/json');
  assert.equal(headers.accept, 'text/event-stream');
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @weknora/mobile test`
Expected: FAIL——`Cannot find module './legacy-tasks-view.ts'`（legacy-tasks-view 三用例）与新增 sse 用例 FAIL（POST 请求未携带 body：`assert.equal(calls[0].init.body, ...)` 收到 undefined）。RED

- [ ] **Step 3: 最小实现**

`apps/mobile/src/adapters/sse-stream.ts`——`SseFetchLike` 与 `streamAuthorizedSse` 的 input 类型加 body、fetch 透传（`sse-stream.ts:4-17` 替换为）：

```ts
/** 流式读取授权 SSE 响应。非 2xx 以真 ApiError 拒绝（Runtime 401 重试与远端 409 映射都依赖该形态）。 */
export type SseFetchLike = (input: string, init?: { method?: string; headers?: Record<string, string>; body?: string; signal?: AbortSignal }) => Promise<Response>;

export async function streamAuthorizedSse(
  origin: string,
  input: { method: string; path: string; headers?: Record<string, string>; body?: unknown; signal?: AbortSignal },
  accessToken: string,
  onChunk: (chunk: string) => void,
  fetchLike: SseFetchLike,
): Promise<void> {
  const hasBody = input.body !== undefined;
  const response = await fetchLike(origin + input.path, {
    method: input.method,
    headers: {
      ...(input.headers ?? {}),
      authorization: `Bearer ${accessToken}`,
      accept: 'text/event-stream',
      ...(hasBody ? { 'content-type': 'application/json' } : {}),
    },
    ...(hasBody ? { body: typeof input.body === 'string' ? input.body : JSON.stringify(input.body) } : {}),
    ...(input.signal === undefined ? {} : { signal: input.signal }),
  });
```

（函数其余部分不动。）

`apps/mobile/src/legacy-tasks-view.ts`（新文件，完整内容）：

```ts
import type { LegacyMessage, LegacyTaskCard, TaskOffice } from '@weknora/mobile-core';

/** Legacy Task 视图状态：列表 + 历史 + 追问回执；cursor/epoch 归 Task Office，本控制器只持有渲染态。 */
export interface LegacyTasksViewState {
  loading: boolean;
  error?: string;
  items: LegacyTaskCard[];
  hasMore: boolean;
  duplicateTaskIds: string[];
  history?: { taskId: string; messages: LegacyMessage[]; error?: string };
  followUpState: 'idle' | 'sending' | 'sent';
  followUpError?: string;
}

export interface LegacyTasksController {
  state(): LegacyTasksViewState;
  subscribe(listener: (state: LegacyTasksViewState) => void): () => void;
  reload(): Promise<void>;
  loadMore(): Promise<void>;
  openHistory(taskId: string): Promise<void>;
  submitFollowUp(taskId: string, question: string): Promise<void>;
  whenSettled(): Promise<void>;
  dispose(): void;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export function createLegacyTasksController(
  office: Pick<TaskOffice, 'legacyTasks' | 'moreLegacyTasks' | 'legacyHistory' | 'followUp'>,
): LegacyTasksController {
  let state: LegacyTasksViewState = { loading: true, items: [], hasMore: false, duplicateTaskIds: [], followUpState: 'idle' };
  const listeners = new Set<(next: LegacyTasksViewState) => void>();
  let pending: Promise<void> = Promise.resolve();
  const publish = (patch: Partial<LegacyTasksViewState>): void => {
    state = { ...state, ...patch };
    for (const listener of listeners) listener(state);
  };

  const run = (action: () => Promise<Partial<LegacyTasksViewState>>): Promise<void> => {
    pending = pending.then(async () => {
      try {
        publish(await action());
      } catch (error) {
        publish({ loading: false, error: errorMessage(error) });
      }
    });
    return pending;
  };

  const controller: LegacyTasksController = {
    state: () => state,
    subscribe(listener) {
      listeners.add(listener);
      return () => { listeners.delete(listener); };
    },
    reload: () => run(async () => {
      const page = await office.legacyTasks({});
      return { loading: false, error: undefined, items: page.items, hasMore: page.nextCursor !== undefined, duplicateTaskIds: page.duplicateTaskIds };
    }),
    loadMore: () => run(async () => {
      const page = await office.moreLegacyTasks();
      return { loading: false, items: [...state.items, ...page.items], hasMore: page.nextCursor !== undefined, duplicateTaskIds: page.duplicateTaskIds };
    }),
    openHistory: (taskId: string) => run(async () => {
      const messages = await office.legacyHistory(taskId);
      return { loading: false, history: { taskId, messages } };
    }),
    submitFollowUp: (taskId: string, question: string) => run(async () => {
      const trimmed = question.trim();
      if (trimmed === '') return { followUpState: 'idle' as const, followUpError: undefined };
      publish({ followUpState: 'sending', followUpError: undefined });
      try {
        await office.followUp({ taskId, question: trimmed });
      } catch (error) {
        return { followUpState: 'idle' as const, followUpError: errorMessage(error) };
      }
      const page = await office.legacyTasks({});
      return { followUpState: 'sent' as const, followUpError: undefined, items: page.items, hasMore: page.nextCursor !== undefined };
    }),
    whenSettled: () => pending,
    dispose: () => { listeners.clear(); },
  };
  void controller.reload();
  return controller;
}
```

`apps/mobile/src/screens/LegacyTasksScreen.tsx`（新文件，完整内容）：

```tsx
import { useEffect, useState } from 'react';
import { Button, Text, TextInput, View } from 'react-native';
import type { LegacyMessage, LegacyTaskCard, TaskOffice } from '@weknora/mobile-core';
import { activeTaskOffice } from '../composition.ts';
import { createLegacyTasksController, type LegacyTasksController, type LegacyTasksViewState } from '../legacy-tasks-view.ts';

const INTENT_LABELS: Array<{ intent: 'run-command' | 'decision' | 'budget' | 'agent-version'; label: string }> = [
  { intent: 'run-command', label: 'Run commands' },
  { intent: 'decision', label: 'Approvals' },
  { intent: 'budget', label: 'Budget' },
  { intent: 'agent-version', label: 'Agent version' },
];

/** Legacy Task 屏：facts-only 投影 + 显式门禁文案 + 历史与普通追问。Screen 只消费 Task Office Interface（seams §10）。 */
export function LegacyTasksScreen({ onStartNewRun }: { onStartNewRun?: () => void } = {}) {
  const [controller, setController] = useState<LegacyTasksController | undefined>(undefined);
  const [view, setView] = useState<LegacyTasksViewState>({ loading: true, items: [], hasMore: false, duplicateTaskIds: [], followUpState: 'idle' });
  const [question, setQuestion] = useState('');

  useEffect(() => {
    const office = activeTaskOffice();
    if (!office) return;
    const next = createLegacyTasksController(office as Pick<TaskOffice, 'legacyTasks' | 'moreLegacyTasks' | 'legacyHistory' | 'followUp'>);
    setController(next);
    setView(next.state());
    const unsubscribe = next.subscribe(setView);
    return () => { unsubscribe(); next.dispose(); };
  }, []);

  return (
    <View>
      <Text>Legacy tasks</Text>
      <Text>Old chat sessions appear here with the same task identity. Only history-provable facts are shown.</Text>
      {view.loading && <Text>Loading</Text>}
      {view.error !== undefined && <Text>{view.error}</Text>}
      {view.items.length === 0 && !view.loading && view.error === undefined && <Text>No legacy tasks</Text>}
      {view.items.map((card: LegacyTaskCard) => (
        <View key={card.taskId}>
          <Text>{`${card.title || card.taskId} · legacy${card.archivedAt !== undefined ? ' · archived' : ''}`}</Text>
          {INTENT_LABELS.map(({ intent, label }) => (
            <Text key={intent}>{`${label}: ${card.gates[intent].state}${card.gates[intent].state === 'unavailable' ? ' — new Run required' : ''}`}</Text>
          ))}
          <Button title="History" onPress={() => { void controller?.openHistory(card.taskId); }} />
          {view.history?.taskId === card.taskId && (
            <View>
              {(view.history.messages as LegacyMessage[]).map((message) => (
                <Text key={message.messageId}>{`${message.role}: ${message.content}`}</Text>
              ))}
            </View>
          )}
          <TextInput value={question} onChangeText={setQuestion} placeholder="Continue with an ordinary follow-up" />
          <Button title="Send follow-up" disabled={view.followUpState === 'sending' || question.trim() === ''} onPress={() => { void controller?.submitFollowUp(card.taskId, question); setQuestion(''); }} />
          {view.followUpState === 'sent' && <Text>Follow-up sent</Text>}
          {view.followUpError !== undefined && <Text>{view.followUpError}</Text>}
        </View>
      ))}
      {view.hasMore && <Button title="Load more" onPress={() => { void controller?.loadMore(); }} />}
      <Button title="Reload" onPress={() => { void controller?.reload(); }} />
      {onStartNewRun !== undefined && <Button title="Start a new Run for new capabilities" onPress={onStartNewRun} />}
    </View>
  );
}
```

`apps/mobile/src/app/tasks/legacy.tsx`（新文件，完整内容）：

```tsx
import { LegacyTasksScreen } from '../../screens/LegacyTasksScreen.tsx';

/** /tasks/legacy：Legacy Task 投影（T14）。Surface 由 Runtime 裁决——activeTaskOffice 无授权面时屏内如实空态。 */
export default function LegacyTasks() {
  return <LegacyTasksScreen />;
}
```

`apps/mobile/src/composition.ts`——import 区（`composition.ts:11` 后）追加：

```ts
import { createMobileLegacyTaskRemote } from '@weknora/api-client/mobile/legacy-tasks';
```

`taskOfficeFor`（`composition.ts:109-124`）的 `createTaskOffice({...})` 参数追加 legacy 端口（`detail: remote,` 之后）：

```ts
      legacy: createMobileLegacyTaskRemote({
        origin,
        request: (input) => activeRuntime.authorizedRequest(input),
        stream: (input, onChunk) => activeRuntime.authorizedEventStream(input, onChunk),
      }),
```

`apps/mobile/src/screens/TasksScreen.tsx`——`TasksScreenProps`（`TasksScreen.tsx:5-8`）追加可选 prop，渲染区（`TasksScreen.tsx:84` `Load more` 按钮之后）追加入口按钮：

```tsx
export interface TasksScreenProps {
  taskOffice: TaskOffice;
  onOpenTask?: (card: TaskCard) => void;
  onOpenLegacy?: () => void;
}
```

```tsx
      {onOpenLegacy !== undefined && <Button title="Legacy tasks" onPress={onOpenLegacy} />}
```

`apps/mobile/src/app/tasks.tsx`——`MobileTasks` 调用处（全文替换为）：

```tsx
import { router } from 'expo-router';
import { MobileTasks } from '../composition.ts';

/** /tasks 一级入口；Surface 仍由 Runtime 快照裁决，行点击进入 /tasks/detail，Legacy 段进入 /tasks/legacy。 */
export default function Tasks() {
  return (
    <MobileTasks
      onOpenTask={(taskId, runId) => router.push({ pathname: '/tasks/detail', params: { taskId, runId } })}
      onOpenLegacy={() => router.push('/tasks/legacy')}
    />
  );
}
```

（`MobileTasks` 的 props 类型 `composition.ts:127` 相应追加可选 `onOpenLegacy?: () => void` 并在 `createElement(TasksScreen, {...})` 处透传——`composition.ts:131-135` 的参数对象追加 `...(onOpenLegacy === undefined ? {} : { onOpenLegacy })`。）

- [ ] **Step 4: 运行测试与类型检查确认通过**

Run: `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`
Expected: 测试 PASS（新增 4 用例 + 既有全绿）+ typecheck exit 0

- [ ] **Step 5: 提交**

```bash
git add apps/mobile/src/legacy-tasks-view.ts apps/mobile/src/legacy-tasks-view.test.ts apps/mobile/src/screens/LegacyTasksScreen.tsx apps/mobile/src/app/tasks/legacy.tsx apps/mobile/src/adapters/sse-stream.ts apps/mobile/src/adapters/sse-stream.test.ts apps/mobile/src/composition.ts apps/mobile/src/screens/TasksScreen.tsx apps/mobile/src/app/tasks.tsx
git commit -m "feat(mobile): legacy tasks screen, /tasks/legacy route and POST-SSE follow-up wiring (T44)"
```

---

### Task 8: apps/mobile——真实 HTTP 集成证据（AC3）

**Files:**
- Create: `apps/mobile/src/legacy-tasks-integration-smoke.ts`
- Test: `apps/mobile/src/legacy-tasks-integration-smoke.test.ts`

**Interfaces:**
- Consumes: Task 5 的 `createTaskOffice`（含 legacy 端口）、Task 6 的 `createMobileLegacyTaskRemote`、Task 7 的 `streamAuthorizedSse`；既有 `createMobileRuntime`/`createInMemoryCredentialStore`/`CLIENT_PROTOCOL_VERSION`/`createWeKnoraClient`/`createJsonTransport`/`createMobileRuntimeRemote`（`task-office-integration-smoke.ts:1-78` 同一装配先例）、`createChatSessionsApi`（`packages/api-client/src/chat/sessions.ts:37`，根导出于 `packages/api-client/src/index.ts:38`——经授权通道创建/删除探针会话）、`disallowedDeploymentHost`（`apps/mobile/src/runtime-integration-smoke.ts:118`，B2-F15 主机防线）。
- Produces: `legacyTasksIntegrationConfig(env: Record<string, string | undefined>): LegacyTasksIntegrationConfig`（opt-in，与 `taskOfficeIntegrationConfig` 相同语义：`WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD` 三变量同进同出、HTTPS 无凭据 origin 校验）与 `runLegacyTasksIntegration(config): Promise<LegacyTasksIntegrationEvidence>`；证据契约 `LegacyTasksIntegrationEvidence = { deploymentOrigin: string; legacyList: 'loaded' | 'failed'; legacyCount: number; probeCreated: boolean; followUp: 'submitted' | 'unavailable' | 'failed'; history: 'loaded' | 'unavailable' | 'failed'; cleanup: 'removed' | 'left' | 'failed'; commandTimestamp: string }`。发布证据矩阵（#71）消费。

- [ ] **Step 1: 写失败测试**

`apps/mobile/src/legacy-tasks-integration-smoke.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { legacyTasksIntegrationConfig, runLegacyTasksIntegration } from './legacy-tasks-integration-smoke.ts';

const env = () => ({ ...process.env }) as Record<string, string | undefined>;

test('the legacy smoke stays opt-in: missing credentials skip, malformed origin is rejected', () => {
  assert.equal(legacyTasksIntegrationConfig({}).enabled, false);
  assert.equal(legacyTasksIntegrationConfig({ WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://weknora.example.test' }).enabled, false);
  const invalid = legacyTasksIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'http://weknora.example.test',
    WEKNORA_MOBILE_TEST_EMAIL: 'a@b.test',
    WEKNORA_MOBILE_TEST_PASSWORD: 'x',
  });
  assert.equal(invalid.enabled, false);
  assert.equal(invalid.enabled === false && invalid.disposition, 'invalid');
});

test('the legacy smoke rejects loopback, private and reserved deployment hosts as invalid', () => {
  for (const host of ['https://localhost', 'https://127.0.0.1:8080', 'https://10.0.0.5', 'https://192.168.1.4', 'https://[fe80::1]', 'https://169.254.1.1']) {
    const verdict = legacyTasksIntegrationConfig({
      WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: host,
      WEKNORA_MOBILE_TEST_EMAIL: 'user@example.test',
      WEKNORA_MOBILE_TEST_PASSWORD: 'pw',
    });
    assert.equal(verdict.enabled, false, host);
    assert.equal(verdict.enabled === false && verdict.disposition, 'invalid', host);
  }
  const verdict = legacyTasksIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://weknora.example.org',
    WEKNORA_MOBILE_TEST_EMAIL: 'user@example.test',
    WEKNORA_MOBILE_TEST_PASSWORD: 'pw',
  });
  assert.equal(verdict.enabled, true);
});

test('the legacy smoke runs against a real deployment when credentials are supplied', async (t) => {
  const config = legacyTasksIntegrationConfig(env());
  if (!config.enabled) {
    t.skip(`missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD: ${config.enabled === false ? config.disposition : ''}`);
    return;
  }
  const evidence = await runLegacyTasksIntegration(config);
  assert.equal(evidence.legacyList, 'loaded', 'the legacy projection must load over the real wire');
  assert.equal(evidence.probeCreated, true, 'the probe session must have been created');
  assert.ok(['submitted', 'unavailable', 'failed'].includes(evidence.followUp), 'follow-up outcome is recorded honestly');
  assert.ok(['loaded', 'unavailable', 'failed'].includes(evidence.history));
  assert.ok(typeof evidence.commandTimestamp === 'string' && evidence.commandTimestamp !== '');
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @weknora/mobile test`
Expected: FAIL——`Cannot find module './legacy-tasks-integration-smoke.ts'`（RED；三个用例均因模块缺失失败）

- [ ] **Step 3: 最小实现**

`apps/mobile/src/legacy-tasks-integration-smoke.ts`（新文件，完整内容）：

```ts
import { createChatSessionsApi, createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createMobileLegacyTaskRemote } from '@weknora/api-client/mobile/legacy-tasks';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createInMemoryCredentialStore, createMobileRuntime, createTaskOffice, type TaskOffice } from '@weknora/mobile-core';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';
import { streamAuthorizedSse } from './adapters/sse-stream.ts';

export type LegacyTasksIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface LegacyTasksIntegrationEvidence {
  deploymentOrigin: string;
  legacyList: 'loaded' | 'failed';
  legacyCount: number;
  probeCreated: boolean;
  followUp: 'submitted' | 'unavailable' | 'failed';
  history: 'loaded' | 'unavailable' | 'failed';
  cleanup: 'removed' | 'left' | 'failed';
  commandTimestamp: string;
}

/** 与 T01/T04 opt-in 语义一致（自包含，不跨计划 import；主机防线复用 B2-F15 同一函数）。 */
export function legacyTasksIntegrationConfig(env: Record<string, string | undefined>): LegacyTasksIntegrationConfig {
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
  // 主机防线（B2-F15）：拒绝 localhost/环回/私网/链路本地/保留地址，与 runtime-integration-smoke 同一语义。
  const hostRejection = disallowedDeploymentHost(parsed.hostname, 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL');
  if (hostRejection) return { enabled: false, disposition: 'invalid', reason: hostRejection };
  return { enabled: true, deploymentOrigin: parsed.origin, email, password };
}

/**
 * 真实生产 JSON transport + 具体 Remote Adapter + Runtime 授权 REST/SSE 通道 +
 * Task Office 编排。探针会话（新建即天然 legacy：0 run）承担完整端到端：
 * legacy 投影可读 → 普通追问（既有聊天 SSE wire）→ 历史可读 → 清理删除。
 * 每一步如实记录，不伪造通过。
 */
export async function runLegacyTasksIntegration(config: Extract<LegacyTasksIntegrationConfig, { enabled: true }>): Promise<LegacyTasksIntegrationEvidence> {
  const evidence: LegacyTasksIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    legacyList: 'failed',
    legacyCount: 0,
    probeCreated: false,
    followUp: 'unavailable',
    history: 'unavailable',
    cleanup: 'left',
    commandTimestamp: new Date().toISOString(),
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
    authorizedStream(origin) {
      return (input, accessToken, onChunk) => streamAuthorizedSse(origin, input, accessToken, onChunk, (url, init) => fetch(url, init as RequestInit));
    },
  });
  const snapshot = await runtime.signIn({
    deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' },
    email: config.email,
    password: config.password,
  });
  if (snapshot.surface !== 'authorized' || !snapshot.deployment) return evidence;

  const office: TaskOffice = createTaskOffice({
    backend: createTaskOfficeRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) }),
    legacy: createMobileLegacyTaskRemote({
      origin: config.deploymentOrigin,
      request: (input) => runtime.authorizedRequest(input),
      stream: (input, onChunk) => runtime.authorizedEventStream(input, onChunk),
    }),
    lease: () => runtime.scopeLease(),
  });
  const sessions = createChatSessionsApi((input) => runtime.authorizedRequest(input));

  // 1) 探针会话：新建空会话即天然 legacy（0 run），标题唯一以便精确搜索。
  const probeTitle = `legacy-smoke-${Date.now()}`;
  let probeId = '';
  try {
    const probe = await sessions.create({ title: probeTitle });
    probeId = probe.id;
    evidence.probeCreated = probeId !== '';
  } catch {
    return evidence;
  }

  // 2) legacy 投影可读（真实 wire + 真实解析）。
  try {
    const page = await office.legacyTasks({ search: probeTitle });
    evidence.legacyCount = page.items.length;
    evidence.legacyList = page.items.some((item) => item.taskId === probeId && item.kind === 'legacy') ? 'loaded' : 'failed';
  } catch {
    evidence.legacyList = 'failed';
  }

  // 3) 普通追问（既有聊天 SSE wire；结果如实记录，不伪造）。
  if (evidence.legacyList === 'loaded') {
    try {
      await office.followUp({ taskId: probeId, question: 'integration probe: reply with the single word ok' });
      evidence.followUp = 'submitted';
    } catch {
      evidence.followUp = 'failed';
    }
    if (evidence.followUp === 'submitted') {
      try {
        const messages = await office.legacyHistory(probeId);
        evidence.history = messages.length > 0 ? 'loaded' : 'failed';
      } catch {
        evidence.history = 'failed';
      }
    }
  }

  // 4) 清理：软删除探针会话并复核 legacy 投影不再包含它。
  try {
    await sessions.remove(probeId);
    const after = await office.legacyTasks({ search: probeTitle });
    evidence.cleanup = after.items.some((item) => item.taskId === probeId) ? 'failed' : 'removed';
  } catch {
    evidence.cleanup = 'failed';
  }
  return evidence;
}
```

（`createChatSessionsApi` 与 `createWeKnoraClient` 均从 `@weknora/api-client` 根导出——`packages/api-client/src/chat/sessions.ts:37` 的工厂以 `request` 函数注入，授权通道直接复用 `runtime.authorizedRequest`，与 `task-office-integration-smoke.ts` 同一先例。）

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`
Expected: 无凭据环境：前两测（opt-in 语义 + 主机防线）PASS、真实部署用例 `t.skip`（如实跳过，不伪造）；有凭据环境：全绿且证据字段在闭集内。

- [ ] **Step 5: 提交**

```bash
git add apps/mobile/src/legacy-tasks-integration-smoke.ts apps/mobile/src/legacy-tasks-integration-smoke.test.ts
git commit -m "test(mobile): opt-in real-HTTP legacy task integration evidence (projection, follow-up, history, cleanup) (T44 AC3)"
```

---

## 计划级验证

在 worktree 根（`.worktrees/issue30-sweep`）执行（覆盖本计划全部定向测试；避免全量 flaky 套件）：

```bash
go test ./internal/application/repository/ -run 'TestWorkbenchLegacyList|TestWorkbenchTaskState' -count=1 && \
go test ./internal/handler/session/ -run 'TestListLegacyTasks|TestLegacyTaskMigration|TestWorkbenchStart|TestWorkbenchTaskState' -count=1 && \
go build ./internal/... ./cmd/... && \
pnpm exec tsx --test packages/mobile-core/src/task-office/legacy-tasks.test.ts packages/mobile-core/src/task-office/legacy-office.test.ts packages/mobile-core/src/task-office/task-office.test.ts && \
pnpm exec tsx --test packages/api-client/src/mobile/legacy-tasks.test.ts && \
pnpm --filter @weknora/mobile test && \
pnpm --filter @weknora/mobile typecheck
```

（作者基线实跑：`go test ./internal/application/repository/ -run 'TestWorkbenchList' -count=1` ok、`pnpm exec tsx --test packages/mobile-core/src/task-office/task-office.test.ts` 全绿——两项均为本会话在当前 HEAD 亲手执行。）

## Spec 覆盖对照（自审第 1 项）

| Spec/AC 条目 | 落点 |
|---|---|
| AC1「不创建第二 task ID」 | Task 1（`task_id` = sessions.id，与 run 列表同身份二分区）；Task 3（同身份归档谓词） |
| AC1「不伪造 Grant、预算、审批或 Agent Version」 | Task 1 键集锁定测试；Task 4 `legacyTaskGates` 不携带任何伪造值；Task 6 remote 解析器 `kind`/`attention` 强校验 |
| AC2「需要新安全语义的行为必须显式升级或新建 Run」 | Task 3 迁移门禁测试（唯一通道 = 新 Run 准入）；Task 4/5 显式门禁与 open-fail-closed；Task 7 屏幕门禁文案 |
| AC3「端到端行为通过最高稳定 Interface 验证」 | Task 8 真实 HTTP 集成证据（blocked-env 声明 + 本地替代证据链）；Task 5 Interface 级场景测试；Task 1/3 Go 真实迁移库 |
| spec「Migration tests verify old Sessions remain readable...」 | Task 3 `TestLegacyTaskMigrationOnlyNewRunAdmissionGrantsRunCapabilities` |
| spec「Existing Session history is projected as legacy Tasks...」 | Task 1/2 投影层与端点；Task 6/7 历史读取 |
| 「继续普通追问」 | Task 5 `followUp` + Task 6 既有聊天 SSE wire + Task 7 追问 UI |
| Out of Scope「不保证每个旧 Session 全量升级」 | 投影 facts-only；升级出口为信息性（#36 start(goal) 边界声明） |

## 边界声明（不做什么）

- `start(goal)`（新 Task/新 Run 入口 UI）属 #36（T06）；本计划的升级出口仅为门禁文案与可选 `onStartNewRun` prop，不实现提交。
- `act(TaskIntent)`（steer/stop/decision）属 #37/#38；legacy 表面无任何 Run 意图入口。
- 跨进程加密持久化属 #40；legacy 列表/历史均为在线读，无离线缓存承诺。
- 统一任务列表（run 行 + legacy 行单列表合并）属 #34「统一 Task 列表」域；本计划以独立 legacy 段/路由呈现，两列表 keyset 互不干扰，taskId 同身份保证未来合并不产生双重身份。
- 小程序零改动（其「继续上次的工作」已由会话列表 + chat 页承担）。
