# Wave 2 — T14 子任务2：Career Application Evidence And Link Workflow（implement）

你是 backend_implementer。本简报自包含：只读本简报 + 引用的计划文件，不依赖任何父会话。**禁止派发子 agent。**

## 0. Worktree 与换基（先做）

```bash
cd /Users/wuyongjun/.codex/worktrees/issue-140-t14-backend/WeKnora-fork01
git status --short        # 必须 clean（调度员 2026-09-25 实核为空，HEAD eb42c99b5）
git checkout --detach 7cbad8941
```
- **BASE = `7cbad8941`（当前集成 HEAD）**。换基理由：T09 后端已修改 `internal/modules/career/office.go`、`handler.go`、`opportunity.go`、`internal/router/routes_career.go`，在旧基线 eb42c99b5 上继续会在集成时冲突；eb42c99b5 的内容已由集成员以 `2a171b389`、`d17c8b121` 集成，无价值损失（reflog 仍可达）。
- detached HEAD 有意为之；本地 commit 允许，绝不 push/merge、绝不改集成分支。

## 1. Step 0（先于主任务，单独 commit）：偿还 T14 子任务1 评审遗留 medium

集成台账载明（不阻塞但须修）：**重试 3 次耗尽仍向调用方泄漏原始 unique/lock 错误**（`internal/modules/workbench/service/workbench/application_task.go` 约 :73-74），且约 :82 的 `ErrApplicationTaskConflict` 不可达；评审建议"下轮 attempt==2 仍 race 时返回 typed conflict"。另有 2 low（race marker 集合宽于计划允许、并发测试 1ms+2ms 预算在慢 CI 理论 flake）——一并收敛，low 若判断不值得动代码须在报告说明理由。

要求：先 RED（耗尽重试仍 race → 断言返回 typed conflict 而非原始错误；marker 集合收敛到 fix-r1 计划允许的三类）→ GREEN 最小实现 → 单独 commit `fix(workbench): return typed conflict after exhausted application task retries`。验证：
```bash
go test ./internal/modules/workbench/service/workbench -run 'Test.*ApplicationTask' -count=1
go test ./internal/modules/workbench/service/workbench -count=1
```

## 2. 主任务：Career Application Evidence And Link Workflow

权威执行文件（全文遵守，冲突以其为准）：`/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01/.superpowers/sdd/2026-09-24-issue-140-t14-backend/task-2-brief.md`。要点复述：

**Files**：create `internal/modules/career/application.go`、`application_test.go`；modify `internal/modules/career/office.go`、`handler.go`、`handler_test.go`；modify `internal/container/container.go`；modify `internal/router/routes_career.go` + 聚焦 route tests；create `migrations/versioned/000197_career_applications.up/.down.sql`、`migrations/sqlite/000118_career_applications.up/.down.sql`；extend `internal/database/career_migration_test.go`。

**迁移编号（本轮唯一分配）**：versioned **000197** / sqlite **000118**（调度员实核空闲：目录现有 196/117 与 198/119，无 197/118）。注意基线里终版本是 198/119——你的 197/118 在文件序上位于其前，这与既有 career_migration_test 的显式 Migrate(版本) 断言模式兼容；不要改 196-119 的既有编号。

**Consumes**：已集成的 Task 1 `interfaces.CareerApplicationTaskLinker`（`internal/types/interfaces/career_application_task.go`，集成于 2a171b389/d17c8b121；`EnsureCareerApplicationTask`/`FindCareerApplicationTask` 签名冻结）；`OpportunityEvidence`、`Evaluation`（T08/T10 已集成）；scoped opportunity/evaluation repositories；owner-only Career 中间件。

**Produces**（task-2-brief.md 冻结，含 `CreateApplicationInput`/`ApplicationReceipt`/`ApplicationEvidencePin` 结构与字段 json tag——以该文件代码块为准逐字实现）。

**HTTP 四路由**：`POST /api/v1/career/applications`→`Office.CreateApplication`；`GET /api/v1/career/applications/receipt?requestId=...`→`FindApplicationReceipt`；`GET /api/v1/career/applications/:applicationId`→`GetApplication`；`POST /api/v1/career/applications/link/reconcile`（body `{"requestId":"..."}`）→`ReconcileApplicationLink`。

**十个命名 RED 测试**（名字逐字，见 task-2-brief.md Step 1）：PinsEvidenceAndLinksTask / HardFailureRequiresExplicitContinue / ExactReplayReturnsOriginalReceipt / SameRequestChangedIntentConflicts / SameJobAndBatchHasOneApplicationAndTask / DistinctBatchesCreateDistinctTasks / LinkerUnknownLeavesLinkingAndReconciles / CareerLinkUpdateFailureReconcilesSameTask / ScopeRejectsOtherTenantAndOwner / RevisionConflictDoesNotCreateSideEffects。

**行为要点**（task-2-brief.md Step 3 全文遵守）：evidence 先查后 replay 已存在收据（fingerprint 精确匹配才返回存储态）；profile revision 须等于 `ExpectedRevision`（exact replay 除外）；hard `ineligible` 无 `ContinueDespiteHardFailure` 拒绝，显式继续则 persist warning + `Qualified=false`；Career 事务内先落 `link_state=linking`+收据再调外部 linker（**外部调用在事务外**）；unknown/timeout 返回 typed `ErrOutcomeUnknown` 且保持 `linking`、绝不再分配新 request ID；确定拒绝置 `link_failed` 保留申请。`ReconcileApplicationLink` 不改 pinned intent，用原 request ID 查 linker，找到则同一行更新为 `ready`。

**Step 5 VERIFY（全跑附原始输出）**：
```bash
gofmt -w internal/types/interfaces/career_application_task.go internal/modules/career internal/modules/workbench/service/workbench internal/container internal/router
go test ./internal/modules/career/... -count=1
go test ./internal/modules/workbench/service/workbench/... -count=1
go test ./internal/database/... -count=1
go test ./internal/router/... -count=1
go test ./tools/architectureguard/... -count=1
git diff --check
```
**COMMIT**：`feat(career): link applications to durable tasks`（add 范围见 task-2-brief.md Step 6）。

## 3. architectureguard 义务

T09 落地后基线为 **583 路由 / 652 计数**（T09 报告载明 581/650→583/652）。你的 4 条新路由落地后应更新为 **587/656** 并保持 `go test ./tools/architectureguard/...` 全绿；Career 清单 ownership 按 T08/T09/T10 先例。

## 4. 全局约束（verbatim，违反即 Spec FAIL）

- Work only in the assigned isolated Worktree；local commits authorized, no push/merge/deploy/GitHub action。
- Career owns application evidence and link state；Career must not import Workbench repositories or write `sessions`/`agent_runs`/Workbench tables directly——只经 `interfaces.CareerApplicationTaskLinker`。
- 一个岗位+批次只有一个申请与 Task；未知 Task 创建结果必须用同一 request ID 恢复；同 request ID 内容变化拒绝。
- 所有查询带 authenticated tenant/owner；origin request ID 本身不是授权。数据库查询一律参数绑定。
- A hard-ineligible evaluation can only create an application when `continueDespiteHardFailure=true`；原状态与 warning 不可变、`qualified=false`。
- 严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`（尤其 `.worktrees/issue30-sweep`）。
- 报告只写事实与实测输出；环境不满足时如实 blocked 附证据，绝不伪造。

## 5. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t14-backend/` 下写 `task-2-report.md`：BASE/HEAD、Step 0 与主任务各自的 RED/GREEN 证据、设计取舍、已知局限。最终消息报告两个 commit 的 SHA 与全部验证结果。留在 worktree 等独立评审与集成；不宣布 T14 完成（Web 子任务仍待派发）。
