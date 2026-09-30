# Task 1 实施报告：PlanStore 与 app_action_plans 迁移（T21 #51）

- Worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t51`（分支 `codex/issue30-t51`）
- Brief：`.superpowers/sdd/plan-t51/task-1-brief.md`（plan：`docs/plans/issue30-sweep/plans/plan-t51.md` Task 1）
- 状态：**DONE**

## 1. 接手状态说明（如实记录）

本轮接手时，Task 1 的实现提交**已由先前轮次落盘**：`4276e36ac feat(appconnector): app_action_plans 计划表 + PlanStore（T21 #51 Task 1）`（2026-09-26 14:32:22 +0800，父提交 `11a067674`），工作树 clean。本轮工作为：①逐字核查已提交实现与 brief 的一致性；②真实运行全部检查并取证（含 RED 复现）；③报告入册。

## 2. 实现内容（逐字核查结论）

Brief（`task-1-brief.md`）要求 6 个文件，全部在提交 `4276e36ac` 中，与 brief 文本**逐字一致，无需任何修改**：

| 文件 | 核查结论 |
|---|---|
| `migrations/sqlite/000119_app_action_plans.up.sql`（33 行） | 与 brief 17–53 行逐字一致：`app_action_plans`（复合 PK `tenant_id+id`、`excluded_json/approved_by DEFAULT ''`）+ `idx_app_action_plans_digest` + `app_action_plan_items`（复合 PK `tenant_id+plan_id+seq`）+ `idx_app_action_plan_items_action` |
| `migrations/sqlite/000119_app_action_plans.down.sql`（4 行） | 与 brief 57–62 行逐字一致（先索引后表，逆序 drop） |
| `migrations/versioned/000198_app_action_plans.up.sql`（33 行） | 与 brief 66–100 行逐字一致（BIGINT/VARCHAR/TIMESTAMPTZ 方言） |
| `migrations/versioned/000198_app_action_plans.down.sql`（4 行） | 与 brief 104–109 行逐字一致 |
| `internal/modules/appconnector/repository/appconnector/plan.go`（137 行） | 与 brief 237–375 行逐字一致：哨兵 `ErrPlanNotFound`/`ErrPlanState`、常量 `PlanStateAwaitingApproval="awaiting_approval"`/`PlanStateAuthorized="authorized"`、`ActionPlanRow`/`ActionPlanItemRow`（TableName `app_action_plans`/`app_action_plan_items`）、`NewPlanStore`、`CreatePlan`（参数校验+单事务，零项拒绝）、`FindPlan`（`gorm.ErrRecordNotFound`→`ErrPlanNotFound`，跨租户统一）、`ListPlanItems`（`Order("seq ASC")`）、`ApprovePlan`（CAS：`digest = ? AND state IN ?`，0 行→`ErrPlanState`） |
| `internal/modules/appconnector/repository/appconnector/plan_test.go`（108 行） | 与 brief 117–226 行逐字一致：`TestPlanCreateFindRoundTrip`（round-trip/seq 升序/零项拒绝/跨租户与缺失统一 not-found）+ `TestPlanApproveBindsDigestAndExclusions`（异 digest 拒绝且不动状态/匹配 CAS 记录 state+exclusions+approver+time/authorized 态幂等重批） |

### 迁移编号核查（brief 第 111 行强制条款，开工时刻实查）

- `ls migrations/sqlite/ | sort`：现有最大为 `000117_code_deliveries`，另有波级双占 `000114_mobile_device_app` + `000114_public_agent_marketplace`（Task 0 待修）；`000118` 空闲（Task 0 marketplace 新号预留位），本任务占 `000119` ✓
- `ls migrations/versioned/ | sort`：现有最大为 `000196_code_deliveries`，另有双占 `000193_mobile_device_app` + `000193_public_agent_marketplace`；`000197` 空闲（Task 0 预留位），本任务占 `000198` ✓
- 结论：**未发生占用冲突，无需顺延**；119>118、198>197，排在 Task 0 marketplace 新号之后的顺序约束满足。DDL 注释文字因此保持 brief 原文。

## 3. TDD 证据

先前的实现轮次未留存 RED/GREEN 输出。本轮以**可复现的方式**补齐证据：

### RED（brief Step 3 预期失败，本轮在父提交上复现）

方法：`git worktree add --detach /tmp/t51-red-check 11a067674`（父提交，无 plan.go——`git ls-tree 11a067674 -- .../plan.go` 为空），将 brief 的 plan_test.go 写入后运行：

```
$ cd /tmp/t51-red-check && go test ./internal/modules/appconnector/repository/appconnector/ -run TestPlan -count=1
# github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector [...test]
internal/modules/appconnector/repository/appconnector/plan_test.go:22:28: undefined: ActionPlanRow
internal/modules/appconnector/repository/appconnector/plan_test.go:22:46: undefined: ActionPlanItemRow
internal/modules/appconnector/repository/appconnector/plan_test.go:31:34: undefined: PlanStateAwaitingApproval
internal/modules/appconnector/repository/appconnector/plan_test.go:48:11: undefined: NewPlanStore
...(too many errors)
FAIL	github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector [build failed]
```

失败原因符合预期：plan.go 尚不存在（brief Step 3 Expected：编译失败 `undefined: ActionPlanRow` 等）。验证后临时 worktree 已 `git worktree remove --force` 清理（worktree list 复查为 0）。

### GREEN（brief Step 5 命名检查，本轮实跑）

```
$ go test ./internal/modules/appconnector/repository/appconnector/ -run TestPlan -count=1 -v
=== RUN   TestPlanCreateFindRoundTrip
--- PASS: TestPlanCreateFindRoundTrip (0.11s)
=== RUN   TestPlanApproveBindsDigestAndExclusions
--- PASS: TestPlanApproveBindsDigestAndExclusions (0.01s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector	0.667s
```

（`-v` 日志中 gorm 红色 `record not found` 行是跨租户/缺失 not-found 用例的预期查询日志，非测试噪音。）不带 `-v` 的首跑同样 `ok ... 2.977s`。

## 4. 其他检查（本轮实跑）

1. **sqlite 迁移 up→down round trip**：`sqlite3 /tmp/t51-mig-check.db < migrations/sqlite/000119_app_action_plans.up.sql` 建表成功，`.schema` 显示复合 PK `(tenant_id, id)` 与索引 `idx_app_action_plans_digest`/`idx_app_action_plan_items_action` 齐备；再灌 down.sql 后 `.tables` 为空（回滚干净）。
2. **appconnector 全包回归**（计划作者基线为 6 包全 ok）：
   ```
   $ go test ./internal/modules/appconnector/... -count=1
   ok  .../internal/modules/appconnector                          0.584s
   ok  .../internal/modules/appconnector/connectorcontrol         3.231s
   ok  .../internal/modules/appconnector/openconnector            0.507s
   ok  .../internal/modules/appconnector/publish                  1.906s
   ok  .../internal/modules/appconnector/repository/appconnector  1.638s
   ok  .../internal/modules/appconnector/service/appconnector     4.304s
   ```
   零回归。本任务 6 文件均为新增，不触碰任何共享文件。

## 5. 自检发现与波级观察（非本任务范围，如实移交）

1. **Task 0 尚未执行**：本 worktree HEAD 上 `go test ./internal/handler/ -run TestNotionPublish -count=1` 中 3 个走迁移装载的 e2e FAIL（`TestNotionPublishEndToEndCreateApprovePublishReceipt`/`...UpdateConflict`/`...UnknownReconcilesRemoteFirst`），错误均为 `failed to open source, "file://...migrations/sqlite": duplicate migration file: 000114_public_agent_marketplace.down.sql`；2 个不装迁移的测试 PASS，handler 包编译正常。这与计划文档（plan-t51.md 第 9、79–83 行）描述的 Task 0 前置问题完全一致。Task 0 属独立任务授权（marketplace 四文件重编 + `internal/database/migration.go` 两处引用），不在本任务 6 文件范围内，未越权处理。本任务的新表迁移在 Task 0 落地后即可被装载轨道拾取（Task 6 收口验证）。
2. **versioned（Postgres 方言）迁移未实跑**：本环境无 PG 实例，`000198` 的 DDL 仅做了与 brief 的逐字比对（一致）。计划中该轨道的可装载性由 Task 6 的迁移↔投影对齐测试与 CI 的 PG 环境收口。
3. **主仓库 main 分支存在与本任务无关的编译破损**：取证期间一次 shell cwd 意外回落到主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`（main @ `ab082ad26`），在其上观察到 `internal/handler` 编译失败（`interaction.go:348` 引用 `AgentRunStore.CancelRunOwnedAtRevision` 而该方法在 main 上不存在）。已确认该现象与本任务 worktree 无关（本 worktree handler 编译通过），不处理、仅备忘。
4. **报告路径说明**：ask 中报告路径为乱码插值（`task-1-brief.md: 390 lines-report.md`），按同目录先例（`plan-t60/task-2-report.md`）落位为 `.superpowers/sdd/plan-t51/task-1-report.md` 并随本轮提交强制入库。

## 6. 提交

- `4276e36ac` `feat(appconnector): app_action_plans 计划表 + PlanStore（T21 #51 Task 1）`——先前轮次产出的 6 个授权文件（本轮逐字核查通过）
- 本轮新增提交：报告入册（`.superpowers/sdd/plan-t51/task-1-report.md`）
