# Wave 11 — T20 后端 wave-report：站内待办与隐私通知（Issue #160）

- **Worktree**: `/Users/wuyongjun/.codex/worktrees/issue-140-t20-reminder/WeKnora-fork01`（独立，从集成 HEAD `ff5b2e843` 创建）
- **BASE**: `ff5b2e843` · **HEAD**: `1bccbec19` · **提交**: `1bccbec19 feat(career): dedupe privacy-safe inbox todos with optional reminders`（本地提交，未 push/merge；14 files, +1334/−26）
- 详版任务报告：worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t20-reminder/task-1-report.md`

## 1. RED 证据（实现前，真实输出）

```
$ go test ./internal/modules/career/... -count=1
# github.com/Tencent/WeKnora/internal/modules/career [github.com/Tencent/WeKnora/internal/modules/career.test]
internal/modules/career/reminder_test.go:25:11: undefined: ReminderNotice
internal/modules/career/reminder_test.go:61:77: undefined: SetReminderInput
internal/modules/career/handler_test.go:946:217: h.SetReminderHandler undefined (type *Handler has no field or method SetReminderHandler)
internal/modules/career/handler_test.go:952:41: undefined: ReminderNoticeBodies
internal/modules/career/handler_test.go:956:65: h.ListRemindersHandler undefined ...
FAIL	github.com/Tencent/WeKnora/internal/modules/career [build failed]

$ go test ./internal/database/... -count=1 -run TestReminderMigrationUpAndDown
--- FAIL: TestReminderMigrationUpAndDown (0.00s)
    career_migration_test.go:727: unable to find file ".../migrations/versioned/000207_career_reminders.up.sql"

$ go test ./internal/router/... -count=1 -run TestCareerReminderRoutesAreRegistered
--- FAIL: TestCareerReminderRoutesAreRegistered (0.01s)  routes_career_test.go:192 Should be true
```

## 2. GREEN 证据（简报 §4 验证命令原样照跑）

```
$ gofmt -w internal/modules/career internal/router        → clean（-l 无输出）
$ go test ./internal/modules/career/... -count=1          → ok   github.com/Tencent/WeKnora/internal/modules/career	11.459s
$ go test ./internal/database/... -count=1                → ok   github.com/Tencent/WeKnora/internal/database	10.366s
$ go test ./internal/router/... -count=1                  → ok   github.com/Tencent/WeKnora/internal/router	1.828s
$ go test ./tools/architectureguard/... -count=1          → ok   github.com/Tencent/WeKnora/tools/architectureguard	0.377s
$ git diff --check                                        → 无空白错误（exit 0）
$ go vet（四包）                                           → 无诊断
```

10 个命名 RED 测试 verbose 全 PASS：TestSameOpportunityAndEventProducesSingleTodo(0.05s)、TestInboxRecordIsAuthoritativeAndPushOnlyReminds(0.05s)、TestNotificationBodyContainsNoCompanyJobOrInterviewDetail(0.04s)、TestUnsubscribeStopsPushButTodosRemainReadable(0.04s)、TestPushDeliveryFailureKeepsInboxFactAndDoesNotMutateState(0.04s)、TestSetReminderExactReplayAndChangedIntentConflict(0.03s)、TestReminderScopeRejectsOtherTenantAndOwner(0.04s)、TestReminderRevisionConflictReturnsCurrentRevision(0.05s)、TestReminderTableIncludedInDeletionPurgeAndBoundary(0.09s)、TestReminderMigrationUpAndDown(0.43s)。另附聚焦测试：TestCareerReminderHandlerWritesTodoWithFrozenNotice、TestCareerReminderRoutesAreRegistered 均 PASS。

## 3. 验收对照（主计划 Task 20）

| 验收 | 实现 | 测试 |
|---|---|---|
| 同一机会和事件只产生一条待办 | 唯一索引 (tenant,user,source_kind,source_id)=确定性幂等键；重复触发返回同一条（deduplicated=true） | TestSameOpportunityAndEventProducesSingleTodo |
| 站内记录是权威，推送只提醒重新同步 | `career_reminders` 权威存储；推送=post-commit 注入 `ReminderNotifier` seam（生产 nil），表无推送列、持久回执无 push 字段 | TestInboxRecordIsAuthoritativeAndPushOnlyReminds |
| 通知正文无公司、岗位、面试细节 | 冻结模板表 `ReminderNoticeBodies`（progress_updated/discovery_found 两条字面量），推送仅收 TemplateKey+Body | TestNotificationBodyContainsNoCompanyJobOrInterviewDetail |
| 取消订阅后不再发送，已存在待办仍可读取 | 档案事实 `notifications.push`=unsubscribed（house confirm 写入）停推；待办照常产生、旧待办照常可读 | TestUnsubscribeStopsPushButTodosRemainReadable |
| 送达失败不丢站内事实，不视为状态更新 | 推送失败→写事务已提交，仅响应 `push.reason=delivery_failed`；行/回执零变更；dedupe 不重推 | TestPushDeliveryFailureKeepsInboxFactAndDoesNotMutateState |
| house：replay/冲突、expectedRevision、scope、未知恢复 | `career_reminder_receipts`（含 dedupe 回执）+ profile head CAS（409 带 currentRevision）+ 认证 scope + OutcomeUnknown | TestSetReminderExactReplayAndChangedIntentConflict、TestReminderRevisionConflictReturnsCurrentRevision、TestReminderScopeRejectsOtherTenantAndOwner |
| 迁移 | versioned 000207 / sqlite 000128 双表 up/down，既有 21 处 version 断言 127→128 | TestReminderMigrationUpAndDown |
| purge+boundary 纳入 | careerPurgeTables+边界 InSpace `reminders` 段 | TestReminderTableIncludedInDeletionPurgeAndBoundary |

## 4. 接口冻结（Web 子任务依据）

- 路由：`POST /api/v1/career/reminders`（set_reminder）、`GET /api/v1/career/reminders`（列表）、`GET /api/v1/career/reminders/receipt?requestId=`。`CareerRemote.act({kind:"set_reminder"...})` → POST /career/reminders。
- 请求：`{"requestId","sourceKind":"progress_event|discovery","sourceId","expectedRevision"}`（严格 JSON）。
- 响应：`kind="reminder_set"`、`reminderId`、`deduplicated`、`noticeKey/notice`（冻结正文）、`status="open"`、`revision`、`push{attempted,delivered,reason}`（仅响应）。
- 取消订阅：既有 `POST /career/act` confirm 档案事实 `notifications.push`=`unsubscribed`。

## 5. architectureguard（以当前实值为基）

- 路由基线 689 → **692**（literal 620→623，apiKeyRoute 69、handle 0 不变；Worker 23/23、hooks 58 不变）；discovery_test.go 历史注释补记 T19(689) 与 T20(692) 两个检查点；TestGuardCleanAtHead 零诊断通过。
- career.yaml manifest 无需改动（`RegisterCareerRoutes — internal/router/routes_career.go:8` 行号不变，函数体内加路由）。

## 6. 文件清单

新增 `internal/modules/career/reminder.go`、`reminder_test.go`、`migrations/versioned/000207_career_reminders.up/.down.sql`、`migrations/sqlite/000128_career_reminders.up/.down.sql`；
修改 `office.go`、`handler.go`、`handler_test.go`、`routes_career.go`、`routes_career_test.go`、`career_export.go`、`internal/database/career_migration_test.go`、`tools/architectureguard/discovery_test.go`。
（迁移目录被 .gitignore 忽略，沿用先例 `git add -f`。）

## 7. 自查与遗留

- **T19 遗留观察项核对（载明，未改）**：`career_preparations` 已在 purge 清单并随删除清除，但删除边界 InSpace 视图**无 preparations 段**（evaluations/discovery_todos/各 receipt 表同样未单列）。属 T19 既有披露缺口；本简报对 career_export.go 的授权仅"新表纳入 purge+boundary 两处小改"，故仅记录，未越权重构（补段为纯增量，现有 Contains 断言兼容，集成方可裁量）。
- dedupe 命中回执 `revision=0`（无写入即无 head CAS）；Web 需当前 head 走 `/open`。
- 推送一次性、无渠道状态机；生产 notifier=nil，真实渠道由端侧任务接入。
- T22 导出档案未含待办数据（未授权改导出清单，待办为低敏引用，可后续增量）。
- 本波唯一 career 后端任务，office.go/handler.go/routes_career.go/career_migration_test.go 无并行冲突。
