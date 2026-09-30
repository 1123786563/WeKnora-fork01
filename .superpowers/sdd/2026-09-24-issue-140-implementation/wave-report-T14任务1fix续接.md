# Wave 报告 — T14 任务1 fix 续接（fix-resume, round 1/5）

- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-t14-backend/WeKnora-fork01`（detached）
- Fix BASE: `c67cec29d26ff911e4254d4a2034ff390a4ebd73`
- 新 HEAD: `eb42c99b5`（`fix(workbench): reconcile concurrent application task creation`，2 files, +206/−2）
- 评审包: worktree 内 `.superpowers/sdd/2026-09-24-issue-140-implementation/review-c67cec29d..eb42c99b5.diff`（13883 bytes）
- worktree 内证据报告: `.superpowers/sdd/2026-09-24-issue-140-t14-backend/task-1-fix-r1-report.md`

## 续接语义执行

中断现场（application_task.go +49 / application_task_test.go +154，三个并发/规范化测试）完整保留，未 checkout/stash/重写。续接核对后补齐三处：

1. **重试条件收敛**（简报预告缺口）：`isApplicationTaskCreationRace` marker 移除 `deadlock detected` / `serialization failure` / `sqlstate 40001` / `sqlstate 40p01`（计划允许集合之外），保留 `gorm.ErrDuplicatedKey` + `unique constraint`/`duplicate key`/`23505`（覆盖 `uq_agent_runs_request`、`uq_workbench_application_tasks_request`）+ SQLite lock 三项。
2. **UUID 等价形态测试补齐**（计划 Review Focus 缺口）：braced 之外补 raw(32 hex) 与 `urn:uuid:` 形式，三者重放返回同一 canonical application ID。
3. **gofmt**：两文件无格式差异。

核对后判定无需改动：并发测试同步语义（start channel + sessions-create callback 暂停/放行）；go.mod 为 go 1.26，goroutine 循环变量每迭代独立（无共享捕获 bug）；`ensureOnce` 三次上限、重试映射重放、非 race 错误原样保留、ctx 取消保护均符合计划。

## RED 证据（真实运行）

方法：`git archive c67cec29d` 导出基线到 /tmp，同步现场测试文件入基线（基线实现 + 新测试），运行：

```
go test ./internal/modules/workbench/service/workbench -run 'TestEnsureCareerApplicationTaskConcurrentExactReplayReturnsOriginal|TestEnsureCareerApplicationTaskConcurrentChangedIntentIsTypedConflict|TestEnsureCareerApplicationTaskCanonicalizesEquivalentUUIDForms' -count=1
```

三个测试全 FAIL，输出节选：
- `TestEnsureCareerApplicationTaskConcurrentExactReplayReturnsOriginal`：`Error: Received unexpected error`
- `TestEnsureCareerApplicationTaskConcurrentChangedIntentIsTypedConflict`：`Target error should be in err chain: expected: "career application task conflict", in chain: "database is locked"`（HIGH finding 直接复现，日志含 `INSERT INTO sessions ... database is locked`）
- `TestEnsureCareerApplicationTaskCanonicalizesEquivalentUUIDForms`：`Received unexpected error: career application task conflict`（braced UUID 未规范化）

## GREEN / VERIFY 证据（真实运行，逐条为计划命令）

| 命令 | 结果 |
|---|---|
| `gofmt -w internal/modules/workbench/service/workbench/application_task.go internal/modules/workbench/service/workbench/application_task_test.go` | exit 0，无格式改动（`gofmt -l` 空输出） |
| `go test ./internal/modules/workbench/service/workbench -run 'Test.*ApplicationTask' -count=1` | `ok github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench 7.463s` |
| `go test ./internal/modules/workbench/service/workbench -count=1` | `ok ... 23.097s` |
| `go test ./internal/application/repository -run 'TestWorkbench.*Task|TestWorkbenchList|TestReadTaskFacts' -count=1` | `ok ... 7.608s` |
| `git diff --check` | exit 0 |

稳定性追加（自愿，非计划要求）：`go test ... -run 'TestEnsureCareerApplicationTaskConcurrent|TestEnsureCareerApplicationTaskCanonicalizes' -count=3` → `ok 4.618s`。

## 三条 finding → 修复映射

1. **HIGH 并发精确重放**：事务体提取为 `ensureOnce`；`EnsureCareerApplicationTask` 最多 3 次有界重试，仅 race 证据触发（短暂停 + ctx 取消保护）；败者重试重读映射 → 原 link（exact）或 `ErrApplicationTaskConflict`（changed）；重试耗尽保留原始错误。测试同步 goroutine 并拒绝任何非 typed 数据库错误。
2. **MEDIUM 投影授权未测**：`TestApplicationTaskProjectionIsListReadableArchivableAndRestorable` 扩展，真实 `AgentRunStore.GetOwnedRun`、`WorkbenchListStore.ReadTaskFactsForRun`（断言 run/session/request 关联与 facts 字段），跨 tenant/owner `ErrNotFound` ×4，跨 scope `SetTaskArchived` → `ErrWorkbenchTaskNotFound` ×2（均在有效归档/恢复之前），无 mock。
3. **MEDIUM UUID 规范化**：`normalizeApplicationTaskIntent` 采用 `parsed.String()`；测试覆盖 braced/raw/URN → 存储仅一份 canonical `0cd7ee38-03e5-45bf-a070-6a8675da7db3`。

## 提交列表

- `eb42c99b5` fix(workbench): reconcile concurrent application task creation（基于 c67cec29d，本地提交，未 push）

## 文件清单（实际改动）

- `internal/modules/workbench/service/workbench/application_task.go`（+48/−2：ensureOnce 提取、重试循环、race 判定收敛、UUID 规范化）
- `internal/modules/workbench/service/workbench/application_task_test.go`（+160：三个新测试 + 投影授权扩展 + raw/URN 形式）

接口签名逐字未动（`EnsureCareerApplicationTask` / `FindCareerApplicationTask` / `CareerApplicationTaskLinker` 断言保留）；未触碰 Career 文件、迁移 schema（未占新迁移编号）、路由清单、无关 Workbench 行为。

## 自查与遗留

- 自查通过：文件所有权（仅两文件）、接口冻结、参数绑定 SQL（全部 `?` 占位）、VERIFY 全绿、无 push/merge/子 agent、主仓库与集成工作区未动（git -C 只读 archive）。
- 环境备注：Mimosa hook 拦截 bash 直接写两源码文件（要求 Write/Edit 路径），续接写操作改经 Write/Edit 工具完成；commit 时 Mimosa scanner_enobufs（未得到完整扫描结论，按兼容策略放行）——本轮改动仅这两个文件且全部经工具链提交，如需可重跑完整审计。
- 遗留：无未修 finding；评审包在 worktree 内（未提交，属 .superpowers 流程目录）；等待控制器验证 + 独立 scoped 评审（范围 c67cec29d..eb42c99b5），不自行集成。
