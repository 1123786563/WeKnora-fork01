# Wave 8 — T18 后端子任务：本人投递确认与实际材料绑定（implement）

你是 backend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** 本任务只做后端；Web 子任务等你的合同 reviewed+integrated 后另行派发。T18 无专用计划——本简报即权威任务书。**本波唯一的 career 后端任务**（office.go/handler.go/routes_career.go/career_migration_test.go 独占）。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t18-submission/WeKnora-fork01 --detach 24708145c
cd /Users/wuyongjun/.codex/worktrees/issue-140-t18-submission/WeKnora-fork01
```
BASE = `24708145c`（当前集成 HEAD）。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 18 文本（verbatim）

### Task 18: T18/#159 本人投递确认与实际材料绑定
**Depends:** #158（T16 verified）、#157（T17 verified）。**Produces:** `CareerRemote.act({kind: "record_submission", payload}, requestId, expectedRevision): Promise<CareerReceipt>`。
**验收：** **系统不点击外部提交或自动发信**；实际版本可选择已验证版，未知时明确记录未确认；**投递事件绑定渠道、时间、版本或未知标记**；重复确认不生成第二次投递记录，后续准备不引用错误版本
**原始证据：** Career Office seam 测试已知/未知版本与重复回执。**失败处理：** **外部提交是否成功由用户确认；产品不得从点击下载推断投递。**
（注：主计划 `internal/modules/career/service/` 为旧稿路径，实际在 `internal/modules/career/` 包根——沿用先例。）

## 3. 设计约定（遵循既有先例，结构由你实现并在报告冻结）

**Files**：create `internal/modules/career/submission.go`、`submission_test.go`；modify `internal/modules/career/office.go`、`handler.go`、`handler_test.go`（聚焦）；modify `internal/router/routes_career.go` + 聚焦 route tests；create `migrations/versioned/000204_career_submissions.up/.down.sql`、`migrations/sqlite/000125_career_submissions.up/.down.sql`；extend `internal/database/career_migration_test.go`；更新 architectureguard Career 清单。

**迁移编号（本轮唯一分配）**：versioned **000204** / sqlite **000125**（调度员实核空闲：目录现有最大 203/124）。

**Consumes**：T16 `career_material_exports`（submittable 的导出版本——`internal/modules/career/rendering.go`）；T14 `career_applications`；T17 progress 事件语义（投递记录可投影为进展事件，设计取舍报告冻结）。

**行为要点**：
1. **纯记录**：record_submission 只持久化"用户声明的投递事实"——渠道（封闭枚举：如 email/web/other 由用户选择）、时间（用户声明）、版本（引用 material version/export）或**显式未知标记**；**绝不发起任何外部提交/发信/点击**，也**绝不从下载行为推断投递**（服务端无此推断代码，报告确认）。
2. **版本选择**：可绑定"已验证版"（submittable export/material version）；用户选择"未知版本"时明确记录 `unconfirmed`——不虚构版本引用。
3. **重复确认**：同一 application 的投递确认幂等（同 request ID replay 返回原记录；**重复确认不生成第二次投递记录**——同 application 已有有效投递时，再次确认按业务语义返回既有记录或 typed conflict，冻结取舍）。
4. **后续准备不引用错误版本**：面试准备等下游消费投递绑定的版本引用必须解析到记录时的确切版本（不可变引用；版本不可变性由 T15/T16 保证，本任务保证引用不被改写）。
5. **house 语义**：request ID replay/conflict、expectedRevision、tenant/owner scope、未知回执恢复。

**命名 RED 测试**：
```go
func TestRecordSubmissionBindsChannelTimeAndVersion(t *testing.T)
func TestRecordSubmissionUnknownVersionRecordsExplicitUnconfirmed(t *testing.T)
func TestRecordSubmissionOnlyAcceptsSubmittableVersions(t *testing.T)
func TestRecordSubmissionRepeatConfirmationDoesNotDuplicate(t *testing.T)
func TestRecordSubmissionImmutableVersionReferenceResolvesExactly(t *testing.T)
func TestRecordSubmissionNeverPerformsOrInfersExternalAction(t *testing.T)
func TestRecordSubmissionExactReplayAndChangedIntentConflict(t *testing.T)
func TestRecordSubmissionScopeRejectsOtherTenantAndOwner(t *testing.T)
func TestRecordSubmissionRevisionConflictReturnsCurrentRevision(t *testing.T)
func TestSubmissionMigrationUpAndDown(t *testing.T)
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
architectureguard：当前基线 **609/678**（T13 后），你的新路由按精确新值更新。

## 5. 全局约束（verbatim 摘录，违反即 Spec FAIL）

- 不自动投递、发送邮件、跨站填表、绕过 CAPTCHA/登录或读取邮箱推断进展。
- 外部提交是否成功由用户确认；产品不得从点击下载推断投递。
- 只在 assigned isolated Worktree 内改本任务所有权文件；local commits authorized, no push/merge/deploy/GitHub action。
- 所有写入使用 request ID 与 expected revision；同一 request ID 内容变化拒绝；scope 从认证上下文派生；数据库查询一律参数绑定。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 6. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t18-submission/` 下写 `task-1-report.md`（mkdir -p）：BASE/HEAD、RED/GREEN 证据、接口冻结说明（HTTP+JSON 供 Web/小程序消费）、与 progress 事件的投影取舍、已知局限。COMMIT：`feat(career): record user-confirmed submissions with bound versions`。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成。
