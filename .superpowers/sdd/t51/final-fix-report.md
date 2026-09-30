# plan-t51 整计划最终审查终修报告（final fix round）

- 日期：2026-09-27
- worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t51`（分支 `codex/issue30-t51`，基线 `76cc5d369`）
- 输入：整计划最终审查 3 项 minor findings，一次批次全部处置
- 结论：Finding 1 已修（文档+台账）、Finding 2 已修（补直接覆盖单测）、Finding 3 维持不修（在案记录 + 机制实证勘误）；终修后整计划门控（更新版 testCommand）五段全绿

---

## Finding 1（minor）：计划级 testCommand 正则不含迁移对齐测试 —— 已修

**审查原文要点**：`-run 'TestActionPlan|TestNotionPublish|TestAppPublications'` 不匹配 `TestAppActionPlansTablesExistAfterMigrations`；`progress.md:24`「非锚定正则可匹配」的自洽判断与事实不符；建议补 `TestAppActionPlans` 分支。

**本 ask 独立复核（实跑证据）**：

```
$ go test ./internal/handler/ -list 'TestActionPlan|TestNotionPublish|TestAppPublications'
→ 12 条（TestActionPlan* 6 条 + TestNotionPublish* 5 条 + TestAppPublicationsTableExistsAfterMigrations），
  不含 TestAppActionPlansTablesExistAfterMigrations —— 审查事实成立
$ go test ./internal/handler/ -list 'TestActionPlan|TestAppActionPlans|TestNotionPublish|TestAppPublications'
→ 13 条，新增 TestAppActionPlansTablesExistAfterMigrations
```

原因：该测试名含子串 `TestAppActionPlans`，与 `TestActionPlan` 不匹配（差一个 `App`）。

**修复内容**：

1. `docs/plans/issue30-sweep/plans/plan-t51.md:2890`：计划级 testCommand handler 段正则改为 `'TestActionPlan|TestAppActionPlans|TestNotionPublish|TestAppPublications'`，并在 testCommand 下方追加修订说明（含修订缘由与 Task 6 历史命令不回写的声明）。
   - Task 6 Step 4/5（plan-t51.md:2868、:2873）的历史命令保持原样：其为已执行完毕步骤，实跑证据已入册（Task 6 报告），不回写历史；该测试此前由 `go test ./internal/modules/appconnector/...`（appconnector 树全量）与 Task 6 Step 3 `'TestActionPlanEndToEnd|TestAppActionPlansTables'` 覆盖，非无覆盖。
2. `.superpowers/sdd/plan-t51/progress.md:24`：原「非锚定正则可匹配 TestAppActionPlansTablesExistAfterMigrations」就地勘误标注（保留原文+插入【勘误 2026-09-27 终审 Finding 1】段，附 -list 实跑复核结果），不抹历史。

**回归证据（新正则实跑）**：

```
$ go test ./internal/handler/ -run 'TestActionPlan|TestAppActionPlans|TestNotionPublish|TestAppPublications' -count=1 -v
→ 13 RUN 全 PASS，首个即 TestAppActionPlansTablesExistAfterMigrations (2.61s)；
  其余 12 个与旧枚举一致（#48 零回归）
ok  github.com/Tencent/WeKnora/internal/handler  30.713s
```

---

## Finding 2（minor）：Execute queued/dispatched→skipped_in_flight 分支零直接覆盖 —— 已修（补覆盖单测）

**审查原文要点**：`internal/modules/appconnector/plan/plan.go:389-390` 的 `ActionQueued || ActionDispatched → ItemSkippedInFlight` 分支在 15 个 plan 单测 + 4 条 e2e 中均无直接覆盖腿，语义由 action 层 ClaimDispatch CAS 兜底，风险低。

**修复内容**：新增 `TestPlanExecuteSkipsInFlightItemsOwnedByLiveWriter`（`internal/modules/appconnector/plan/plan_test.go`，置于 `TestPlanExecuteSkipsUnapprovedItemFailClosed` 之后）。覆盖腿设计：

- 3 项计划 approveAll 后，用既有 CAS helper `SetActionState(ctx, id, from, to)` 把 item1 置 `queued`、item2 置 `dispatched`（模拟并发 live writer 占有窗口），item3 留 `authorized` 作控制腿；
- 断言：item1/item2 disposition == `ItemSkippedInFlight` 且 ActionState 原样反映；item3 照常 `ItemExecuted→succeeded`（in-flight 不阻断逐项独立执行）；`dispatch.calls[act1]==0 && dispatch.calls[act2]==0`（永不重复派发）；Execute 后三行状态逐一回读（`FindAction`）确认 plan pass 未改动 live writer 状态机。

**回归证据（实跑）**：

```
$ go test ./internal/modules/appconnector/plan/ -run TestPlanExecuteSkipsInFlightItemsOwnedByLiveWriter -count=1 -v
--- PASS: TestPlanExecuteSkipsInFlightItemsOwnedByLiveWriter (0.02s)

$ go test ./internal/modules/appconnector/plan/ -count=1 -v
→ 16 个测试全 PASS（15 既有 + 1 新增），go vet 同包无告警
```

---

## Finding 3（minor）：GREEN 输出 gorm 红色 record not found 噪音 —— 维持不修，在案记录 + 机制实证

**审查原文要点**：噪音来自 publish seam `LatestPublishedByDestination`/`Receipt` 对无 publication 行项的探测（`publication.go:79` 探测点），定性为预期路径噪音、非失败、与 #48 同款；维持不修，仅记录。

**本 ask 实证（两轮对照实验，临时测试文件已删）**：

1. 直接单发 `publishSvc.Receipt(ctx, 7, "act-missing")`（→ `FindByAction`，`publication.go:93`）→ trace 归因 `publication.go:93 record not found`；直接单发 `CreatePublication` → 归因 `:79`。gorm v1.31.1 `utils.CallerFrame` 归因本身准确。
2. 在 plan env 内以 Info 级 logger 跑完整 FormPlan→Approve→Execute 流程：三条 `publication.go:79 record not found`（rows:0）全部出现在 **FormPlan 阶段**，且每条之后紧跟同 action 的 `INSERT INTO app_publications ... state="planned"`；Execute 阶段的 Receipt 探测全部命中（rows:1，formation 已建 planned 行），零 not-found trace。
3. 机制勘误：**噪音源是 `CreatePublication` 的存在性守卫预检**（`internal/modules/appconnector/repository/appconnector/publication.go:79`，由 `publish/plan.go:225` 在计划 formation 时对每个新项调用）——rows:0 是该守卫的**预期分支**（无既有行 → 放行受保护插入）；finding 所述 Receipt/LatestPublishedByDestination 探测（`:93`/`:159`）在本计划流程中因 formation 已建行而命中，不产生噪音。finding 引用的探测点行号 `:79` 正确，机制描述存在偏差，以本实证为准。
4. 定性不变：该红色仅为 gorm `logger.Default`（`IgnoreRecordNotFoundError=false`）把 `ErrRecordNotFound` 按 Error 级 trace 打印，测试结果不受影响（门控全绿），与 #48 GREEN 输出同款。**维持不修**——消灭它需定制 gorm logger 或改守卫写法，两者皆属噪音治理而非缺陷修复，收益不抵夹具扰动。

**本 ask 附加实证**：新增的 Finding 2 单测 `-v` 输出（`go test ./internal/modules/appconnector/plan/ -run TestPlanExecuteSkipsInFlightItemsOwnedByLiveWriter -count=1 -v`）含 3 条同款 `publication.go:79 record not found`（rows:0），PASS 不受影响——即为在案活样本。

---

## 终修后整计划门控复跑（更新版 testCommand，本 worktree 根执行）

```
$ go build ./... && go test ./internal/database/ -count=1 && go test ./internal/modules/appconnector/... -count=1 \
  && go test ./internal/handler/ -run 'TestActionPlan|TestAppActionPlans|TestNotionPublish|TestAppPublications' -count=1 \
  && go test ./internal/router/ -run TestActionPlanRoutes -count=1
== build OK ==（cmd/desktop、cmd/server 各一条 ld duplicate-libraries warning，既有噪音非错误）
ok  github.com/Tencent/WeKnora/internal/database       82.559s
ok  github.com/Tencent/WeKnora/internal/modules/appconnector                    2.175s
ok  github.com/Tencent/WeKnora/internal/modules/appconnector/connectorcontrol   7.877s
ok  github.com/Tencent/WeKnora/internal/modules/appconnector/openconnector      3.248s
ok  github.com/Tencent/WeKnora/internal/modules/appconnector/plan               3.914s   ← 含新增覆盖单测
ok  github.com/Tencent/WeKnora/internal/modules/appconnector/publish            3.330s
ok  github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector  1.917s
ok  github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector     6.948s
ok  github.com/Tencent/WeKnora/internal/handler   53.036s   ← 13 RUN（含 TestAppActionPlansTablesExistAfterMigrations）
ok  github.com/Tencent/WeKnora/internal/router    2.954s
```

计划 gate：**PASS**（更新版 testCommand 全链一次通过）。

如实声明：plan-t51.md testCommand 注明的执行根是兄弟 worktree `…/.worktrees/issue30-sweep`（计划成文时路径），本终修在该计划的执行 worktree `…/.worktrees/issue30-sweep-t51` 复跑——内容等价（同一计划分支），未改写计划中的路径原文。

## 变更清单

| 文件 | 变更 |
| --- | --- |
| `docs/plans/issue30-sweep/plans/plan-t51.md` | testCommand handler 段正则补 `TestAppActionPlans`（:2890）+ 修订说明段 |
| `.superpowers/sdd/plan-t51/progress.md` | :24 就地勘误标注；文末追加终修轮台账 |
| `internal/modules/appconnector/plan/plan_test.go` | 新增 `TestPlanExecuteSkipsInFlightItemsOwnedByLiveWriter`（Finding 2 直接覆盖腿） |
| `.superpowers/sdd/t51/final-fix-report.md` | 本报告（新建） |

无生产代码改动（Finding 2 修的是覆盖缺口，测试文件；Finding 3 明确不修）。
