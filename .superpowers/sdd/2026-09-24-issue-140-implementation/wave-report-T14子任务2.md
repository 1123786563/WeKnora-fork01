# T14 子任务2 报告：Career Application Evidence And Link Workflow

- **BASE**: `7cbad8941`（调度员换基指令执行：`git checkout --detach 7cbad8941`，弃 eb42c99b5 旧基线）
- **HEAD**: `83c47198b`
- **提交**（本地，未 push）：
  1. `6488b250e` fix(workbench): return typed conflict after exhausted application task retries（Step 0，单独 commit）
  2. `83c47198b` feat(career): link applications to durable tasks（主任务）

## Step 0：偿还子任务1评审遗留 medium（+2 low）

### RED（真实输出）

命令：`go test ./internal/modules/workbench/service/workbench -run 'TestEnsureApplicationTaskRetryExhaustionReturnsTypedConflict|TestEnsureApplicationTaskNonRaceErrorIsReturnedAsIs|TestIsApplicationTaskCreationRaceMatchesOnlyPlannedMarkers' -count=1`

```
internal/modules/workbench/service/workbench/application_task_test.go:501:27: coordinator.ensureWithRetry undefined (type *ApplicationTaskCoordinator has no field or method ensureWithRetry)
internal/modules/workbench/service/workbench/application_task_test.go:519:27: coordinator.ensureWithRetry undefined (type *ApplicationTaskCoordinator has no field or method ensureWithRetry)
FAIL	github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench [build failed]
```

（marker 收敛测试为行为级 RED：旧实现匹配 generic "unique constraint"/"duplicate key"/"23505"，`sqlstate 23505 without a request constraint name` 等用例会误判为 race；编译错误先暴露。）

### GREEN

- 重试循环提取为 `ensureWithRetry(ctx, tenant, owner, intent, once applicationTaskEnsure)` 可测缝；`EnsureCareerApplicationTask` 委托之。
- **medium 修复**：attempt==2 仍 race 时 `break` 出循环，统一 `return fmt.Errorf("%w: creation still racing after %d attempts: %v", ErrApplicationTaskConflict, 3, lastRace)`——调用方拿到 typed conflict（wrap 保留末次 race 证据），原 :82 不可达 return 变为可达出口，原始 unique/lock 错误不再泄漏。
- **low-1 修复（marker 收敛）**：`isApplicationTaskCreationRace` 仅保留 fix-r1 计划允许三类——`gorm.ErrDuplicatedKey`、两个 request 唯一约束名（`uq_agent_runs_request`/`uq_workbench_application_tasks_request`）、SQLite 锁（`database is locked`/`database table is locked`/`sqlite_busy`）；移除 generic `unique constraint`/`duplicate key`/`23505`。
- **low-2 修复（backoff）**：重试间隔 1ms+2ms → 10ms+20ms（`applicationTaskRetryPause`）。理由：并发测试 ctx 预算 5s，30ms 总退避仍是"短暂停"，显著降低慢 CI 上锁竞争越过退避窗口的理论 flake。

### 验证（真实输出）

```
$ go test ./internal/modules/workbench/service/workbench -run 'Test.*ApplicationTask' -count=1
ok  	github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench	19.169s
$ go test ./internal/modules/workbench/service/workbench -count=1
ok  	github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench	122.663s
```

## 主任务：10 个命名 RED 测试 → 实现 → GREEN

### RED（真实输出）

命令：`go test ./internal/modules/career -run 'Test.*Application' -v -count=1`

```
application_test.go:63:70: undefined: CreateApplicationInput
application_test.go:174:4: o.SetApplicationTaskLinker undefined
application_test.go:177:20: o.CreateApplication undefined
application_test.go:230:19: o.Application undefined
application_test.go:243:26: undefined: ErrApplicationHardIneligible
... (too many errors)
FAIL	github.com/Tencent/WeKnora/internal/modules/career	[build failed]
```

### 实现要点（design 取舍）

- **类型**：`CreateApplicationInput`/`ApplicationReceipt`/`ApplicationEvidencePin` 逐字按 task-2-brief 代码块（json tag 一致）。
- **存储**：`career_applications` 表（versioned 000197 / sqlite 000118，编号为调度员唯一分配，实核空闲）：UUID id、request+fingerprint(SHA-256 of normalized input JSON)、immutable opportunity/snapshot/evaluation、profile_revision+evidence_body、canonical batch（`strings.ToLower(strings.Join(strings.Fields(x)," "))`）、continue 标志、evaluation_status、qualified、warning_body、link_state、task_id/run_id（NOT NULL DEFAULT ''）、receipt_body、created/updated；非空唯一 `(tenant_id,user_id,request_id)` 与 `(tenant_id,user_id,opportunity_id,batch_identity)` + scope/link_state 索引。down = DROP TABLE。
- **CreateApplication 顺序**：scope+requireSpace → 输入校验 → 单事务内（行锁）：exact replay（fingerprint 精确匹配返回存储态，不同内容 ErrIdempotencyConflict）→ 同 scope 加载 snapshot+evaluation 并校验 evaluation 引用一致 → profile head 锁读且 revision 必须等于 ExpectedRevision → hard ineligible 无显式继续即拒（ErrApplicationHardIneligible），显式继续则 Qualified=false+warning（hardRuleId/reasonCode/evaluationId）→ 同 job/batch 唯一检查 → 落 `linking` 行 → commit。事务提交后（外部调用在事务外）以**原 request ID** 调 linker：成功→同一行更新 `ready`+TaskID/RunID；`ErrCareerApplicationTaskConflict`（definite）→同一行 `link_failed`+返回 typed ErrApplicationConflict；其余（含 ctx 取消/超时）→ `OutcomeUnknownError{原 request ID}` 且保持 `linking`。事务内唯一冲突竞争路径：按 request 重读（指纹匹配回放/不匹配 conflict），否则按 job/batch 重读→ErrApplicationConflict。
- **ReconcileApplicationLink**：scope 读行（不改 pinned intent），以原 request ID 调 `FindCareerApplicationTask`：找到→同一行更新 `ready`；`ErrCareerApplicationTaskNotFound`→保持 `linking` 返回当前 receipt；其他错误→`OutcomeUnknownError`。
- **跨模块 typed error**：Career 不得 import workbench 包 → 在 `internal/types/interfaces/career_application_task.go` 增加共享 sentinel `ErrCareerApplicationTaskConflict`/`ErrCareerApplicationTaskNotFound`（消息与 workbench 原值逐字相同），workbench 侧原 `ErrApplicationTaskConflict`/`ErrApplicationTaskNotFound` 改为别名赋值（`errors.Is` 语义不变，既有测试无需改动）。
- **HTTP**：4 路由 `POST /career/applications`、`GET /career/applications/receipt?requestId=`、`GET /career/applications/:applicationId`、`POST /career/applications/link/reconcile`（body `{"requestId"}`）；`NewHandler` 增加 linker 参数；writeError 映射 application not_found(404)/application_conflict(409)/hard_ineligible_requires_continue(409)/outcome_unknown(504 带 requestId)。
- **container**：`Provide(workbenchservice.NewApplicationTaskCoordinator)` + 显式 interface provider（仿既有 initFileService 先例，不依赖 dig 隐式接口匹配）。
- **Office**：`linker` 字段 + `SetApplicationTaskLinker`；`failApplicationReadyUpdate` 测试钩子（沿用 Office 既有 hook 先例）模拟"Workbench 已持久化但 Career ready 更新失败"。
- **architectureguard**：`wantRouteLiteral 583→587`、`wantRouteTotal 652→656`（注释更新注明 T14 4 条）；career.yaml 无需改（routes 条目按文件覆盖，routes_career.go 仍被 `RegisterCareerRoutes — internal/router/routes_career.go:8` 覆盖）。
- **migration 编号核对**：终版本仍是 versioned 198 / sqlite 119（197/118 位于其前），既有 `require.Equal(t, 119, version)` 断言不变；`Migrate(114/115/116/117)` 下行路径实测通过。

### 测试夹具

`fakeCareerApplicationLinker`（可配 Ensure/Find 错误与 Find link、调用计数、记录 intent/scope/requestID）；`seedApplicationEvaluation`（真实 Office API：Confirm→ImportJD→EvaluateOpportunity）；行级断言直查 `career_applications`。

### GREEN（真实输出，Step 5 VERIFY 全跑）

```
$ gofmt -w internal/types/interfaces/career_application_task.go internal/modules/career internal/modules/workbench/service/workbench internal/container internal/router   # 无输出
$ go test ./internal/modules/career/... -count=1
ok  	github.com/Tencent/WeKnora/internal/modules/career	4.206s
$ go test ./internal/modules/workbench/service/workbench/... -count=1
ok  	github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench	142.805s
$ go test ./internal/database/... -count=1
ok  	github.com/Tencent/WeKnora/internal/database	63.239s
$ go test ./internal/router/... -count=1
ok  	github.com/Tencent/WeKnora/internal/router	13.087s
$ go test ./tools/architectureguard/... -count=1
ok  	github.com/Tencent/WeKnora/tools/architectureguard	2.130s
$ git diff --check        # exit 0，无输出
```

中途一次真实失败与修复：首跑 career 包 `TestSlowConcurrentMutationAcrossSQLiteConnections` 4 子用例失败——AutoMigrate 路径下 gorm tag 只把 job/batch 唯一索引建在 (opportunity_id,batch_identity)（缺 tenant/user 列），第二连接 NewOffice 的 schema 校验报 `career_applications uniqueness ... missing`。修复：tag 用 priority 1-4 显式纳入四列后全绿。

## 文件清单

新增：`internal/modules/career/application.go`、`application_test.go`、`migrations/versioned/000197_career_applications.{up,down}.sql`、`migrations/sqlite/000118_career_applications.{up,down}.sql`
修改：`internal/modules/career/{office,handler,handler_test}.go`、`internal/types/interfaces/career_application_task.go`、`internal/modules/workbench/service/workbench/application_task.go`（Step 0 + sentinel 别名）、`internal/container/container.go`、`internal/router/{routes_career.go,routes_career_test.go}`、`internal/database/career_migration_test.go`、`tools/architectureguard/discovery_test.go`

## 自查与已知局限

- 十个命名测试逐字落地并全绿；receipt JSON tag、linker 调用次数/参数（原 request ID、确定性 Title `Career application <opportunityID>`、scope）均有断言。
- hard-continue 的 warning 取自 evaluation body 的 hard rule[0]（ruleId/reasonCode）；若未来 rules 多条，仅取首条（graduation_year 是当前唯一 hard 规则）。
- `Qualified` 定义为 `evaluationStatus != ineligible`（unknown 不阻塞，非硬性冲突在显式继续语义之外保持可见）。
- handler 层新增 1 个映射测试（409/504/404 + requestId body）；未覆盖 Reconcile 的 HTTP 层（office 层已覆盖，handler 方法为薄封装）。
- 迁移 up/down/up 与双唯一约束由 `TestCareerApplicationSQLiteMigrationUpDownUp` 实测；PostgreSQL versioned 000197 仅静态落盘（与 196 同模式，本环境无 PG 实例）。
- 遗留注释陈旧（非本任务所有权）：`workbench_application_task_migration_test.go:63-65` 与 `career_migration_test.go` 中"117/118 reserved"注释在 118 落地后过时，行为不受影响。
- 不宣布 T14 完成：Web 子任务仍待派发；本 worktree 留待独立评审与集成。
