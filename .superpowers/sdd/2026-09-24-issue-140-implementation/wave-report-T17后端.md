# Wave 5 报告：T17 后端——申请进展事件与阶段投影（Issue #157, implement）

- 执行者：backend_implementer（#140 wave 5，本波唯一 career 后端任务）
- 现场：worktree `/Users/wuyongjun/.codex/worktrees/issue-140-t17-progress/WeKnora-fork01`
- BASE `05122c146` → HEAD `648e0db88`，提交 `feat(career): project application progress from immutable events`（本地提交，未 push、未碰集成分支）
- 详细接口冻结与已知局限见 worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t17-progress/task-1-report.md`

## RED/GREEN 证据

### RED（先写失败测试并运行）

| 命令 | 真实输出（节选） |
| --- | --- |
| `go test ./internal/modules/career/ -run 'TestAppendProgress\|TestCorrectProgress\|TestProgressProjection\|TestProgressEvents\|TestProgressScope\|TestProgressRevision\|TestProgressUnknown' -count=1` | `undefined: AppendProgressInput` / `undefined: progressEventRecord` / `h.AppendProgress undefined` … `FAIL [build failed]` |
| `go test ./internal/database/ -run 'TestProgressMigrationUpAndDown' -count=1` | `--- FAIL: TestProgressMigrationUpAndDown`（`unable to find file .../000201_career_progress_events.up.sql`） |
| `go test ./internal/router/ -run 'TestCareerProgressRoutesAreRegistered' -count=1` | `--- FAIL: TestCareerProgressRoutesAreRegistered`（`Should be true`） |

### GREEN（实现后，简报第 4 节命令全跑）

| 命令 | 输出 |
| --- | --- |
| `gofmt -w internal/modules/career internal/router` | 无输出；`gofmt -l` 复查无差异 |
| `go test ./internal/modules/career/... -count=1` | `ok  github.com/Tencent/WeKnora/internal/modules/career  9.007s` |
| `go test ./internal/database/... -count=1` | `ok  github.com/Tencent/WeKnora/internal/database  10.109s` |
| `go test ./internal/router/... -count=1` | `ok  github.com/Tencent/WeKnora/internal/router  2.709s` |
| `go test ./tools/architectureguard/... -count=1` | `ok  github.com/Tencent/WeKnora/tools/architectureguard  0.769s` |
| `git diff --check` | 无输出（clean） |
| 额外：`go vet ./internal/modules/career/ ./internal/router/ ./internal/database/` | clean |

10 个命名 RED 测试全部落地通过：append 不可变+replay 幂等、纠错不覆盖原行、确定性投影、重开一致、来源与确认者可追溯、跨申请不串联、scope（跨租户/跨用户/缺失应用统一 ErrApplicationNotFound 不泄漏）、revision 冲突返回当前值、未知结果经原 request ID 恢复、迁移 up/down。

## 提交列表

- `648e0db88` feat(career): project application progress from immutable events（单提交，BASE `05122c146`）

## 文件清单（简报所有权内）

新增：
- `internal/modules/career/progress.go`（核心实现：封闭 intent、append-only 写路径、确定性投影、回执）
- `internal/modules/career/progress_test.go`（9 个命名 office 级测试）
- `migrations/versioned/000201_career_progress_events.up.sql` / `.down.sql`（本轮分配编号）
- `migrations/sqlite/000122_career_progress_events.up.sql` / `.down.sql`

修改：
- `internal/modules/career/office.go`（模型清单 +progressEventRecord；SQLite 启动校验新表列与两条唯一约束；测试钩子 afterProgressEventPersist）
- `internal/modules/career/handler.go`（4 处理器、ErrProgressEventNotFound→404、source 客户端白名单 manual/user、路径参数权威）
- `internal/modules/career/handler_test.go`（聚焦 TestCareerProgressHTTPContract）
- `internal/router/routes_career.go`（4 条路由）
- `internal/router/routes_career_test.go`（TestCareerProgressRoutesAreRegistered）
- `internal/database/career_migration_test.go`（TestProgressMigrationUpAndDown + 既有头版本断言 121→122——新增 000122 的机械后果）
- `tools/architectureguard/discovery_test.go`（597→601、666→670 按精确新值 + 历史注释）

## 接口冻结摘要（供 Web/小程序下游）

- `POST /api/v1/career/applications/:applicationId/progress`（append）| `POST .../progress/correct`（纠错）| `GET .../progress`（历史+投影）| `GET /api/v1/career/progress/receipt?requestId=`（回执重放）
- 事件字典（9，封闭）：pending_submission、submitted、resubmitted、assessment、interview、offer、rejected、withdrawn、retracted
- 阶段词汇（7，封闭）：preparing（零事件默认）、pending_submission、submitted、assessment、interview、offer、closed
- 投影：更正就地替换被更正事件的贡献（最新更正胜出），当前阶段 = 生效时间线 seq 最大事件的阶段；纯函数、重开一致
- expectedRevision = 该申请当前事件数；收据字段/错误码（409 revision_conflict 携 currentRevision、409 idempotency_conflict、504 outcome_unknown 携 requestId、404 not_found）见 worktree 报告全文

## 自查与遗留

- 自查通过：所有权文件之外零改动；SQL 全部参数绑定（GORM 占位符）；confirmer 恒取认证 scope（请求体无此字段且 DisallowUnknownFields）；无自动投递/邮件/点击推断路径；迁移编号为唯一分配的 000201/000122；architectureguard 精确新值 601/670。
- 迁移目录被 `.gitignore:96` 的 `migrations/` 覆盖，按既有先例 `git add -f` 纳入跟踪。
- 遗留（非阻塞）：事件字典/阶段词汇为首版冻结，扩充需同步投影映射与测试；`occurredAt` 为用户自述仅展示用；并发依赖应用行锁 + SQLite busy 有界重试（与模块既有先例一致）。
- 环境说明：Go 1.26.3；纯 Go 任务未涉及 node_modules/pnpm。
