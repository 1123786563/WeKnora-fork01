# Wave 11 — T20 后端子任务：站内待办与隐私通知（implement）

你是 backend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** 本任务只做后端；Web 子任务等你的合同 reviewed+integrated 后另行派发。T20 无专用计划——本简报即权威任务书。**本波唯一的 career 后端任务**（office.go/handler.go/routes_career.go/career_migration_test.go 独占）。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t20-reminder/WeKnora-fork01 --detach ff5b2e843
cd /Users/wuyongjun/.codex/worktrees/issue-140-t20-reminder/WeKnora-fork01
```
BASE = `ff5b2e843`（当前集成 HEAD）。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 20 文本（verbatim）

### Task 20: T20/#160 站内待办与隐私通知
**Depends:** #154（T13 verified）、#157（T17 verified）。**Produces:** `CareerRemote.act({kind: "set_reminder", payload}, requestId, expectedRevision)`。
**验收：** **同一机会和事件只产生一条待办**；**站内记录是权威，推送只提醒重新同步**；**通知正文无公司、岗位、面试细节**；**取消订阅后不再发送，已存在待办仍可读取**
**原始证据：** Inbox/通知契约测试去重、隐私文案和取消订阅。**失败处理：** **通知送达失败不丢站内事实，也不将推送视为状态更新。**
（注：主计划 `internal/modules/career/service/` 为旧稿路径，实际在 `internal/modules/career/` 包根——沿用先例。）

## 3. 设计约定（遵循既有先例，结构由你实现并在报告冻结）

**Files**：create `internal/modules/career/reminder.go`、`reminder_test.go`；modify `internal/modules/career/office.go`、`handler.go`、`handler_test.go`（聚焦）；modify `internal/router/routes_career.go` + 聚焦 route tests；create `migrations/versioned/000207_career_reminders.up/.down.sql`、`migrations/sqlite/000128_career_reminders.up/.down.sql`；extend `internal/database/career_migration_test.go`；更新 architectureguard Career 清单。**另**：`career_export.go` 的 `careerPurgeTables`/删除边界（boundary）需纳入新表（T19 F1 集成修复先例——新持久表必须进 purge 清单与边界视图，本任务所有权含此两处小改）。

**迁移编号（本轮唯一分配）**：versioned **000207** / sqlite **000128**（调度员实核空闲：目录现有最大 206/127）。

**Consumes**：T13 发现待办（career_search_rules 域的 discovery todos）；T17 progress 事件（面试/进展类待办来源）；T22 删除 purge 模式（本任务新表纳入）。

**行为要点**：
1. **待办去重权威**：同一（机会/事件）只产生一条待办（确定性幂等键：scope+来源事件 ID；重复产生/触发返回同一条）；站内待办表是权威存储。
2. **推送只是提醒**：推送（通知）为 best-effort 提醒渠道（注入式 notifier seam，生产可空实现）；**送达失败不丢站内事实**、**绝不将推送回执视为待办状态更新**。
3. **隐私正文**：通知/推送正文**不含公司、岗位、面试细节**（如"你有新的求职进展待查看"级别；站内详情仅登录后可见）；隐私文案为冻结枚举/模板。
4. **取消订阅**：unsubscribe 后不再产生推送（站内待办继续产生且**已存在待办仍可读取**）。
5. **house 语义**：request ID replay/conflict、expectedRevision、scope、未知恢复。

**命名 RED 测试**：
```go
func TestSameOpportunityAndEventProducesSingleTodo(t *testing.T)
func TestInboxRecordIsAuthoritativeAndPushOnlyReminds(t *testing.T)
func TestNotificationBodyContainsNoCompanyJobOrInterviewDetail(t *testing.T)
func TestUnsubscribeStopsPushButTodosRemainReadable(t *testing.T)
func TestPushDeliveryFailureKeepsInboxFactAndDoesNotMutateState(t *testing.T)
func TestSetReminderExactReplayAndChangedIntentConflict(t *testing.T)
func TestReminderScopeRejectsOtherTenantAndOwner(t *testing.T)
func TestReminderRevisionConflictReturnsCurrentRevision(t *testing.T)
func TestReminderMigrationUpAndDown(t *testing.T)
func TestReminderTableIncludedInDeletionPurgeAndBoundary(t *testing.T)
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
architectureguard 基线以 tools/architectureguard 清单当前实值为准（T19 后），新路由按精确值更新。

## 5. 全局约束（verbatim 摘录，违反即 Spec FAIL）

- 只在 assigned isolated Worktree 内改本任务所有权文件；local commits authorized, no push/merge/deploy/GitHub action。
- 通知正文无公司、岗位、面试细节；站内记录是权威。
- 所有写入使用 request ID 与 expected revision；scope 从认证上下文派生；数据库查询一律参数绑定。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 6. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t20-reminder/` 下写 `task-1-report.md`（mkdir -p）：BASE/HEAD、隐私文案模板冻结、RED/GREEN 证据、接口冻结说明、已知局限。COMMIT：`feat(career): dedupe privacy-safe inbox todos with optional reminders`。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成。
