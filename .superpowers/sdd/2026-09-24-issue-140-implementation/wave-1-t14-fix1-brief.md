# Wave 1 — T14 子任务1 修复续接（fix-resume, round 1/5）

你是 backend_implementer。本简报自包含：只读本简报 + 下述计划文件，不依赖任何父会话。**禁止派发子 agent。**

## 0. 续接语义（最重要，先读）

- Worktree：`/Users/wuyongjun/.codex/worktrees/issue-140-t14-backend/WeKnora-fork01`（已存在，detached HEAD `c67cec29d26ff911e4254d4a2034ff390a4ebd73`，提交信息 `feat(workbench): ensure idempotent career application tasks`）。这是已通过控制器验证的 checkpoint，**绝对不得丢弃或重置**。
- **现场遗留（不得覆盖）**：该 worktree 有未提交修改，恰是上一位实现者被中断的修复现场：
  - `M internal/modules/workbench/service/workbench/application_task.go`（+49：提取 `ensureOnce`、UUID 规范化、重试骨架）
  - `M internal/modules/workbench/service/workbench/application_task_test.go`（+154：三个新测试 `TestEnsureCareerApplicationTaskConcurrentExactReplayReturnsOriginal`、`TestEnsureCareerApplicationTaskConcurrentChangedIntentIsTypedConflict`、`TestEnsureCareerApplicationTaskCanonicalizesEquivalentUUIDForms`，以及用真实 `GetOwnedRun`/`ReadTaskFactsForRun` 的跨 scope 断言）
  - 调度员 2026-09-25 实测：`go test ./internal/modules/workbench/service/workbench -run 'Test.*ApplicationTask' -count=1` → `ok github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench 19.271s`（含未提交修改的当前状态）。
- 续接动作：先 `git status --short && git diff` 完整阅读未提交现场 → 对照下方修复计划核对覆盖度 → 补齐缺口（如并发测试同步语义、重试条件收敛、gofmt）→ 完整跑 VERIFY → 补报告证据 → 提交。**禁止 `git checkout -- .` / `git stash` 丢弃现场后从零重写**；若判定现场某部分方向错误，先在报告中说明理由再以最小 diff 修正。
- 修复轮上限 5 轮，本轮为 round 1/5。若你认为剩余 medium/high 无法在本轮修完，如实报告，不得伪装通过。

## 1. 被修复的三条独立评审 findings（裁定：三条均 valid）

来源：T14 子任务1 独立 Spec/quality 评审（checkpoint c67cec29d）；裁定记录在 `/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01/.superpowers/sdd/2026-09-24-issue-140-t14-backend-review-fix-r1/progress.md`：

- **HIGH 并发精确重放**：`agent_runs` 有 `(tenant_id, owner_id, request_id)` 唯一约束，当前事务在 reconciling 映射冲突之前先插入 run，并发同请求竞争败者会把原始唯一性/锁错误直接泄漏给调用方。
- **MEDIUM 投影读取与跨范围归档授权未测**：真实 `GetOwnedRun`、`ReadTaskFactsForRun`、跨 scope `SetTaskArchived` 拒绝路径没有针对该投影的测试（此前仅 mock 层面）。
- **MEDIUM 等价 UUID 形态未规范化**：`uuid.Parse` 结果被丢弃，braced/raw/URN 等等价拼写会变成不同 intent 或超出 PostgreSQL 存储假设。

## 2. 修复计划（权威执行文件，全文遵守）

`/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01/docs/plans/2026-09-24-issue-140-t14-backend-review-fix-r1.md`

该计划要点（与其冲突时以计划文件为准）：

**Goal**：并发 Workbench application-task 重放返回原 link 或 typed conflict；规范化 application ID；证明真实 owner-scoped 读取/归档路径。

**文件所有权（仅此两个文件）**：`internal/modules/workbench/service/workbench/application_task.go`、`application_task_test.go`。不得改 Career 文件、迁移 schema、路由清单或无关 Workbench 行为。

**接口冻结（签名逐字保留）**：
```go
// internal/types/interfaces/career_application_task.go（已存在于 checkpoint，只读）
type CareerApplicationTaskIntent struct { ApplicationID string; RequestID string; Title string }
type CareerApplicationTaskLink struct { TaskID string; RunID string }
type CareerApplicationTaskLinker interface {
    EnsureCareerApplicationTask(ctx context.Context, tenantID uint64, ownerID string, intent CareerApplicationTaskIntent) (CareerApplicationTaskLink, error)
    FindCareerApplicationTask(ctx context.Context, tenantID uint64, ownerID, requestID string) (CareerApplicationTaskLink, error)
}
// workbench 包（签名逐字保留）
func (c *ApplicationTaskCoordinator) EnsureCareerApplicationTask(ctx context.Context, tenantID uint64, ownerID string, intent interfaces.CareerApplicationTaskIntent) (interfaces.CareerApplicationTaskLink, error)
func (c *ApplicationTaskCoordinator) FindCareerApplicationTask(ctx context.Context, tenantID uint64, ownerID, requestID string) (interfaces.CareerApplicationTaskLink, error)
```

**行为约束（计划 Global Constraints verbatim）**：
- Concurrent exact replay must return the same Task/Run; concurrent changed intent must return `ErrApplicationTaskConflict`; neither may leak a raw unique/lock error.
- Retry only evidence of concurrent creation (`gorm.ErrDuplicatedKey`, the two request uniqueness constraint names, or SQLite database-lock contention), with a small bounded attempt count. Other errors remain original errors.
- Canonicalize accepted UUID spellings to `uuid.UUID.String()`.
- Exercise `AgentRunStore.GetOwnedRun`, `WorkbenchListStore.ReadTaskFactsForRun`, and cross-scope archive denial; no mock may replace these repository calls.

**GREEN 实现要点**：把当前事务体提取为未导出 `ensureOnce`；`uuid.Parse` 后用 `parsed.String()` 规范化；`EnsureCareerApplicationTask` 最多调用 `ensureOnce` 三次；仅当回滚后的错误是可归因于并发 request 创建的 duplicate/lock race 时重试（短暂停）；重试时既有映射重放返回原 link 或 typed conflict；保留所有非 race 错误。

**测试要求（RED，部分已在现场存在）**：并发测试必须同步 goroutine 启动点并拒绝任何非 typed 数据库错误；扩展 `TestApplicationTaskProjectionIsListReadableArchivableAndRestorable` 用真实 `AgentRunStore.GetOwnedRun` 与 `WorkbenchListStore.ReadTaskFactsForRun`，断言跨 tenant/跨 owner 返回 `ErrNotFound`，断言跨 scope `SetTaskArchived` 在有效归档/恢复之前返回 `ErrWorkbenchTaskNotFound`。

**VERIFY（必须全跑并在报告附原始输出）**：
```bash
gofmt -w internal/modules/workbench/service/workbench/application_task.go internal/modules/workbench/service/workbench/application_task_test.go
go test ./internal/modules/workbench/service/workbench -run 'Test.*ApplicationTask' -count=1
go test ./internal/modules/workbench/service/workbench -count=1
go test ./internal/application/repository -run 'TestWorkbench.*Task|TestWorkbenchList|TestReadTaskFacts' -count=1
git diff --check
```

**COMMIT**：先把精确 RED/GREEN 证据追加到既有 Task 1 报告（worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t14-backend/` 下的 task-1 报告文件；若不存在则在同目录新建 `task-1-fix-r1-report.md`），然后：
```bash
git add internal/modules/workbench/service/workbench/application_task.go internal/modules/workbench/service/workbench/application_task_test.go
git commit -m 'fix(workbench): reconcile concurrent application task creation'
```

## 3. 全局约束（verbatim，违反即 Spec FAIL）

- Work only in `/Users/wuyongjun/.codex/worktrees/issue-140-t14-backend/WeKnora-fork01`；Fix BASE 是 `c67cec29d26ff911e4254d4a2034ff390a4ebd73`；local commit authorized, no push/merge。
- Do not alter Career files, migration schema, route manifests, or unrelated Workbench behavior.
- 严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`（尤其 `.worktrees/issue30-sweep`）与集成工作区。
- 禁止 push、merge、deploy、GitHub 操作；禁止派发子 agent。
- 数据库查询一律参数绑定，不得拼接 SQL。
- 报告只写事实与实测输出；环境不满足时如实写 blocked 并附证据，绝不伪造通过。

## 4. 完成后

留在 worktree 等待控制器验证 + 独立 scoped 评审（评审范围 `c67cec29d..<你的新 HEAD>` 的 diff）。不自行集成、不宣布 T14 完成。最终消息报告：新 HEAD SHA、VERIFY 各命令结果、相对中断现场你补齐/修正了什么。
