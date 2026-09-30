# Wave 13 — T12 后端子任务：多来源去重、岗位更新与覆盖说明（implement）

你是 backend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** 本任务只做后端；Web 子任务等你的合同 reviewed+integrated 后另行派发。T12 无专用计划——本简报即权威任务书。**本波唯一的 career 后端任务**，也是 **#140 最后一个 career 后端票**。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t12-reconciliation/WeKnora-fork01 --detach 38c22abe9
cd /Users/wuyongjun/.codex/worktrees/issue-140-t12-reconciliation/WeKnora-fork01
```
BASE = `38c22abe9`（当前集成 HEAD）。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 12 文本（verbatim）

### Task 12: T12/#151 多来源去重、岗位更新与覆盖说明
**Depends:** #152（T11 verified）。**Produces:** `CareerRemote.act({kind: "import_url", payload}, ...)`（去重/合并语义扩展于 import 链路）。
**验收：** **仅有充分岗位编号、企业、地点与批次证据时合并**；**不确定重复并列**，**所有原始链接和检查时间保留**；**过期、下架和要求变化显式标注，旧申请仍展示旧快照**；**可查看已接入来源和实际覆盖城市**
**原始证据：** Career Office 合同测试覆盖**同岗双来源、同名不同批次和 JD 更新**。**失败处理：** **来源检查失败保留最后成功观察并显示陈旧时间。**
（注：主计划 `internal/modules/career/service/` 为旧稿路径，实际在 `internal/modules/career/` 包根——沿用先例。）

## 3. 设计约定（遵循既有先例，结构由你实现并在报告冻结）

**Files**：create `internal/modules/career/reconciliation.go`、`reconciliation_test.go`；modify `internal/modules/career/office.go`、`handler.go`、`handler_test.go`（聚焦）；modify `internal/router/routes_career.go` + 聚焦 route tests；如需持久合并证据表 create `migrations/versioned/000209_career_reconciliations.up/.down.sql`、`migrations/sqlite/000130_career_reconciliations.up/.down.sql`；extend `internal/database/career_migration_test.go`（如建表，**纳入 career_export.go purge 清单与 boundary 视图——T19/T20/T21 先例**）；更新 architectureguard Career 清单。

**迁移编号（本轮唯一分配，如建表）**：versioned **000209** / sqlite **000130**（调度员实核空闲：目录现有最大 208/129）。

**Consumes**：T09 observations（不可变观察历史——合并判断的证据输入）；T08 opportunities/snapshots；T11 searches 结果。

**行为要点**：
1. **合并门槛**：仅当有**充分证据**（岗位编号+企业+地点+批次 的确定性匹配——冻结匹配规则）时才合并两条岗位记录；**不确定的重复并列**（不合并、双条可见+标注疑似重复）。
2. **不可变痕迹**：合并/并列决策**不删除任何原始观察**；所有原始链接与检查时间保留可查。
3. **状态显式**：岗位快照的 过期/下架/要求变化 为显式状态标注（来自后续观察对比）；**旧申请仍展示旧快照**（申请 pinned evidence 不变——T14 语义，测试断言）。
4. **覆盖说明**：已接入来源与实际覆盖城市可查（复用 T11 coverage 语义的持久化查询或聚合端点，形态冻结）。
5. **失败处理**：来源检查失败 → 保留最后成功观察 + **显示陈旧时间**（stale 标注，不静默丢弃）。
6. **house 语义**：request ID replay/conflict、expectedRevision、scope、未知恢复。

**命名 RED 测试**：
```go
func TestReconcileMergesOnlyWithSufficientIdentityEvidence(t *testing.T)
func TestReconcileUncertainDuplicatesStaySideBySide(t *testing.T)
func TestReconcilePreservesAllOriginalLinksAndCheckTimes(t *testing.T)
func TestReconcileMarksExpiryDelistingAndRequirementChanges(t *testing.T)
func TestReconcileOldApplicationsStillShowOldSnapshots(t *testing.T)
func TestReconcileExposesSourceCoverageAndCities(t *testing.T)
func TestReconcileCheckFailureKeepsLastObservationWithStaleTime(t *testing.T)
func TestReconcileSameJobDualSourcesAndDistinctBatches(t *testing.T)
func TestReconcileExactReplayAndChangedIntentConflict(t *testing.T)
func TestReconciliationMigrationUpAndDown(t *testing.T) // 如建表；否则等价持久测试
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
architectureguard 基线以清单当前实值为准（T21 后），新路由按精确值更新。

## 5. 全局约束（verbatim 摘录，违反即 Spec FAIL）

- 只在 assigned isolated Worktree 内改本任务所有权文件；local commits authorized, no push/merge/deploy/GitHub action。
- 所有写入使用 request ID 与 expected revision；scope 从认证上下文派生；数据库查询一律参数绑定（外部输入绝不拼接 SQL）。
- 不确定重复不得静默合并；原始观察不可删除。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 6. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t12-reconciliation/` 下写 `task-1-report.md`（mkdir -p）：BASE/HEAD、合并匹配规则冻结、RED/GREEN 证据、接口冻结说明、已知局限。COMMIT：`feat(career): reconcile duplicate opportunities with explicit evidence`。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成。
