# Wave 5 — T17 后端子任务：申请进展事件与阶段投影（implement）

你是 backend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** 本任务只做后端；Web 子任务等你的合同 reviewed+integrated 后另行派发。T17 无专用计划——本简报即权威任务书。**本波唯一的 career 后端任务**（office.go/handler.go/routes_career.go/career_migration_test.go 独占）。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t17-progress/WeKnora-fork01 --detach 05122c146
cd /Users/wuyongjun/.codex/worktrees/issue-140-t17-progress/WeKnora-fork01
```
BASE = `05122c146`（当前集成 HEAD）。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 17 文本（verbatim）

### Task 17: T17/#157 申请进展事件与阶段投影
**Depends:** #155（T14 已 verified）。**Produces:** `CareerRemote.act({kind: "append_progress", payload}, requestId, expectedRevision): Promise<CareerReceipt>`（封闭 intent 集另含 `correct_progress`）。
**验收：** 事件追加保存，**纠错追加更正事件而非覆盖原记录**；当前阶段由已确认事件投影，**重新打开结果一致**；同一请求重放不出现第二个事件；**事件来源和确认者可追溯，跨申请记录不串联**
**原始证据：** Career Office 合同测试覆盖事件顺序、更正、幂等、阶段投影。**失败处理：** 写入结果未知先查回执，不用新 ID 盲目重试。
（注：主计划 `internal/modules/career/service/` 为旧稿路径，实际在 `internal/modules/career/` 包根——沿用先例。Step 1-4 框架：RED 公共 seam 测试覆盖验收+request ID+revision+Tenant 隔离 → 最小实现封闭 intent → 验证。）

## 3. 设计约定（遵循既有先例，结构由你实现并在报告冻结）

**Files**：create `internal/modules/career/progress.go`、`progress_test.go`；modify `internal/modules/career/office.go`、`handler.go`、`handler_test.go`（聚焦）；modify `internal/router/routes_career.go` + 聚焦 route tests；create `migrations/versioned/000201_career_progress_events.up/.down.sql`、`migrations/sqlite/000122_career_progress_events.up/.down.sql`；extend `internal/database/career_migration_test.go`；更新 architectureguard Career 清单。

**迁移编号（本轮唯一分配）**：versioned **000201** / sqlite **000122**（调度员实核空闲：目录现有最大 200/121）。

**Consumes**：已集成的 `career_applications`（T14）——进展事件绑定单一 application（scope 校验：application 必须属于当前 tenant/user，否则 ErrNotFound 不泄漏存在性）。

**HTTP（三件套约定，形态由你设计并在报告冻结）**：如 `POST /applications/:applicationId/progress`（append）、`POST /applications/:applicationId/progress/correct`（纠错）、`GET /progress/receipt?requestId=...`、`GET /applications/:applicationId/progress`（事件历史+当前阶段投影）。

**行为要点**：
1. **追加不可变**：事件 append-only；`correct_progress` 追加一条"更正事件"引用被更正事件 ID，**原事件永不修改/删除**；更正后投影采用更正语义但历史完整可读。
2. **确定性投影**：当前阶段 = 已确认事件的确定性折叠（纯函数：同事件集 → 同阶段），重复计算/重新打开结果一致；投影不落可变状态（或落缓存但可从事件重建并测试一致）。
3. **幂等**：request ID exact replay 返回原收据不产生第二事件；同 ID 变更 payload typed conflict；expectedRevision 冲突返回当前 revision。
4. **可追溯**：每事件记录来源（source：用户录入/系统导入等封闭枚举）与确认者（confirmer 身份来自认证 scope，不信任客户端）。
5. **不串联**：事件查询/投影严格 application-scoped；跨 application 查询拒绝。
6. **未知恢复**：写入结果未知 → typed unknown + 原 request ID 查回执恢复（house 语义；SQLite busy 分类参照 office.go 既有 isSQLiteBusy 模式，含 requireSpace 级别——Wave 4 集成修复 2edced621 先例）。

**命名 RED 测试**：
```go
func TestAppendProgressPersistsImmutableEventAndReplayIsIdempotent(t *testing.T)
func TestCorrectProgressAppendsCorrectionWithoutOverwritingOriginal(t *testing.T)
func TestProgressProjectionDerivesStageDeterministicallyFromConfirmedEvents(t *testing.T)
func TestProgressProjectionReopenYieldsSameResult(t *testing.T)
func TestProgressEventsTraceSourceAndConfirmer(t *testing.T)
func TestProgressEventsDoNotChainAcrossApplications(t *testing.T)
func TestProgressScopeRejectsOtherTenantAndOwner(t *testing.T)
func TestProgressRevisionConflictReturnsCurrentRevision(t *testing.T)
func TestProgressUnknownOutcomeRecoversViaReceipt(t *testing.T)
func TestProgressMigrationUpAndDown(t *testing.T)
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
architectureguard：当前基线 **597/666**（T15 后），你的新路由按精确新值更新。

## 5. 全局约束（verbatim 摘录，违反即 Spec FAIL）

- 只在 assigned isolated Worktree 内改本任务所有权文件；local commits authorized, no push/merge/deploy/GitHub action。
- 所有写入使用 request ID 与 expected revision；同一 request ID 内容变化拒绝；scope 从认证上下文派生；数据库查询一律参数绑定。
- 事件来源和确认者可追溯，跨申请记录不串联；纠错不覆盖原记录。
- 不自动投递、不发送邮件、不从点击下载推断进展（本任务纯事件记录）。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 6. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t17-progress/` 下写 `task-1-report.md`（mkdir -p）：BASE/HEAD、RED/GREEN 证据、接口冻结说明（HTTP+JSON 供 Web/小程序消费）、已知局限。COMMIT：`feat(career): project application progress from immutable events`。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成。
