# Wave 10 — T19 后端子任务：求职信与基于投递版的面试准备（implement）

你是 backend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** 本任务只做后端；Web 子任务等你的合同 reviewed+integrated 后另行派发。T19 无专用计划——本简报即权威任务书。**本波唯一的 career 后端任务**（office.go/handler.go/routes_career.go/career_migration_test.go 独占）。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t19-preparation/WeKnora-fork01 --detach f6b14cf95
cd /Users/wuyongjun/.codex/worktrees/issue-140-t19-preparation/WeKnora-fork01
```
BASE = `f6b14cf95`（当前集成 HEAD）。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 19 文本（verbatim）

### Task 19: T19/#156 求职信与基于投递版的面试准备
**Depends:** #159（T18 verified）。**Produces:** `CareerRemote.act({kind: "edit_material", payload}, requestId, expectedRevision)`（沿用 edit_material 封闭 intent——准备材料是材料域的新正文类型）。
**验收：** **求职信与回答只引用已确认事实和岗位快照**；**面试准备优先固定实际投递版本，版本未知必须提示**；**生成结果可审阅、修订且有来源**；**不自动发送或替用户承诺事实**
**原始证据：** Career Office 测试证明 **V2 已投递时引用 V2、未知时不猜 V3**。**失败处理：** 模型失败保留请求和可恢复状态，**不展示空白成功产物**。
（注：主计划 `internal/modules/career/service/` 为旧稿路径，实际在 `internal/modules/career/` 包根——沿用先例。）

## 3. 设计约定（遵循既有先例，结构由你实现并在报告冻结）

**Files**：create `internal/modules/career/preparation.go`、`preparation_test.go`；modify `internal/modules/career/office.go`、`handler.go`、`handler_test.go`（聚焦）；modify `internal/router/routes_career.go` + 聚焦 route tests；如需持久表 create `migrations/versioned/000206_career_preparations.up/.down.sql`、`migrations/sqlite/000127_career_preparations.up/.down.sql`（若复用 materials 域结构不建新表，报告说明取舍并归还编号）；extend `internal/database/career_migration_test.go`（如建表）；更新 architectureguard Career 清单。

**迁移编号（本轮唯一分配，如需）**：versioned **000206** / sqlite **000127**（调度员实核空闲：目录现有最大 205/126）。

**Consumes**：T18 `career_submissions`（投递绑定的版本引用或显式 unconfirmed——`internal/modules/career/submission.go`）；T15 materials（结构化正文/claim 引用语义）；T10 评估/T08 岗位快照。

**行为要点**：
1. **版本锚定**：面试准备/求职信生成时，**优先固定该申请实际投递的版本**（submissions 的版本引用）；投递为 unconfirmed/无投递 → **明确返回"版本未知必须提示"状态**（typed 可见，不猜最新版——V2 已投递时引用 V2、未知时不猜 V3）。
2. **只引用已确认事实和岗位快照**：正文 claim 沿用 materials 的确认事实引用校验（T15 先例）；不得引入未确认事实。
3. **可审阅、修订、有来源**：生成结果是草稿（可修订、带来源引用链：事实/快照/投递版本）；修订产生新草稿/版本，不覆盖。
4. **不自动发送、不替用户承诺**：纯生成+审阅闭环，零外部发送动作；正文不得虚构用户承诺/经历（缺失去"缺失/待补充"占位——T15 语义）。
5. **模型失败**（若引入生成 seam 则注入式，生产无外部 LLM 依赖——报告披露）：失败保留请求与可恢复状态，typed error，**不产生空白成功产物**。
6. **house 语义**：request ID replay/conflict、expectedRevision、scope、未知恢复。

**命名 RED 测试**：
```go
func TestPreparationAnchorsToActuallySubmittedVersion(t *testing.T)      // V2 已投递 → 引用 V2
func TestPreparationUnknownVersionRequiresPromptNotLatestGuess(t *testing.T) // 未知 → 提示态，不猜 V3
func TestPreparationClaimsOnlyConfirmedFactsAndSnapshot(t *testing.T)
func TestPreparationDraftReviewableRevisionableWithSources(t *testing.T)
func TestPreparationNeverAutoSendsOrPromisesFacts(t *testing.T)
func TestPreparationGenerationFailurePreservesRecoverableRequest(t *testing.T)
func TestPreparationExactReplayAndChangedIntentConflict(t *testing.T)
func TestPreparationScopeRejectsOtherTenantAndOwner(t *testing.T)
func TestPreparationRevisionConflictReturnsCurrentRevision(t *testing.T)
func TestPreparationMigrationUpAndDown(t *testing.T)                     // 若建表；否则以等价持久测试替代并说明
```

## 4. 验证（全跑附原始输出）

```bash
gofmt -w internal/modules/career internal/router
go test ./internal/modules/career/... -count=1
go test ./internal/database/... -count=1
go test ./internal/router/... -count=1
go test ./tools/architectureguard/... -count=1
git diff --check
```
architectureguard：当前基线以 `git log` 最近 guard 提交为准（T22 后，读 tools/architectureguard 清单实值），你的新路由按精确新值更新。

## 5. 全局约束（verbatim 摘录，违反即 Spec FAIL）

- 只在 assigned isolated Worktree 内改本任务所有权文件；local commits authorized, no push/merge/deploy/GitHub action。
- 所有写入使用 request ID 与 expected revision；同一 request ID 内容变化拒绝；scope 从认证上下文派生；数据库查询一律参数绑定。
- 缺失不得由模型补造；不自动发送；外部副作用用收据对账。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 6. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t19-preparation/` 下写 `task-1-report.md`（mkdir -p）：BASE/HEAD、版本锚定语义冻结、RED/GREEN 证据、接口冻结说明、生成 seam 取舍披露、已知局限。COMMIT：`feat(career): anchor interview preparation to submitted versions`。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成。
