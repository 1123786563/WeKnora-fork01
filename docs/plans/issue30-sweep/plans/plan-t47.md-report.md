# plan-t47（T17 / Issue #47）实施报告 —— Task 2：Go wire 委派与批注端点 + 路由 + 容器

> 执行者：实现员-t47-任务2（subagent-driven-development）。工作目录：`.worktrees/issue30-sweep-t47`。
> 本文件此前不存在（Task 1 报告未落此路径），由 Task 2 创建；后续任务请在其后追加。

## 1. 任务与交付物

按 `plan-t47.md` Task 2（Go wire——委派与批注端点 + 路由 + 容器）实施，严格 RED→GREEN→装配→验证→提交：

**新建：**
- `internal/handler/session/workbench_research.go` —— `WorkbenchResearchHandler`：
  - 五个端点方法：`DelegateResearch`（POST research，owner-only，源先经 `ResearchSourceAuthorizer` 租户校验、任何落库之前拒绝越权源）、`ListResearch`（GET，owner + granted 回退）、`CompleteResearch`（POST summary，owner-only，委派须绑定本 run 的 session，CAS 完成、重放 409）、`AnnotateMaterial`（POST annotations，owner/collaborator 经 `TaskRoleCanRun` 门，`base_version` 必须等于 `artifactVersionOf(当前工件)`，否则 409）、`ListAnnotations`（GET，owner + granted）。
  - 四个 seam 接口：`ResearchStore` / `AnnotationStore` / `ResearchSourceAuthorizer` / `TaskAccessResolver`（生产实现分别为 `*repository.TaskResearchStore` / `*repository.TaskAnnotationStore` / 容器 `researchSourceAuthorizer` / `*service.TaskGrantService`）。
  - 结构性只读保证：handler 不持有任何 run 准入写路径（无 `AgentRunStore.Admit` 依赖），委派不可能占用/释放 `sessions.active_agent_run_id` 单写槽。
  - 错误码表逐字对齐计划：`research_invalid_request`(400)/`research_source_out_of_task_grant`(400)/`unauthorized`(401)/`research_forbidden`(403)/`run_not_found`(404)/`research_not_found`(404)/`material_not_found`(404)/`research_delegation_state`(409)/`annotation_base_version_conflict`(409)。
- `internal/handler/session/workbench_research_test.go` —— 12 个 handler 测试（计划原文逐字），复用包内既有 `workbenchRunReaderStub`（workbench_read_test.go:27）/`artifactRefReaderStub`（workbench_artifacts_test.go:23）。

**修改（均为追加式，未动他人逻辑）：**
- `internal/router/routes_workbench.go` —— 文件末尾追加 `RegisterWorkbenchResearchRoutes`（计划逐字）：reads 树挂 GET research/annotations（Viewer + workbenchReadGate + API-key 边界），writes 树挂 POST research、research/:delegation_id/summary、annotations。
- `internal/router/router.go` —— ① `RouterParams` 增加 `WorkbenchResearchHandler *session.WorkbenchResearchHandler optional:"true"` 字段（router.go:63）；② `RegisterWorkbenchDeliveryRoutes` 调用行之后追加 `RegisterWorkbenchResearchRoutes(v1, params.WorkbenchResearchHandler, rbacGuards)`（router.go:390）。
- `internal/container/workbench.go` —— 追加 `researchSourceAuthorizer`（租户绑定 KB 校验）+ `NewResearchSourceAuthorizer(db)` + `NewWorkbenchResearchHandler(...)`。
- `internal/container/container.go` —— `Provide(NewWorkbenchDeliveryHandler)` 之后追加两行：`must(container.Provide(NewResearchSourceAuthorizer))`、`must(container.Provide(NewWorkbenchResearchHandler))`。

## 2. TDD 过程与测试命令完整输出

### RED（Step 1→2）

先写测试，实跑确认预期编译失败：

```
$ go test ./internal/handler/session/ -run 'TestDelegateResearch|TestListResearch|TestCompleteResearch|TestAnnotateMaterial|TestListAnnotations' -count=1
# github.com/Tencent/WeKnora/internal/handler/session [github.com/Tencent/WeKnora/internal/handler/session.test]
internal/handler/session/workbench_research_test.go:145:98: undefined: WorkbenchResearchHandler
internal/handler/session/workbench_research_test.go:149:7: undefined: NewWorkbenchResearchHandler
internal/handler/session/workbench_research_test.go:158:54: undefined: WorkbenchResearchHandler
internal/handler/session/workbench_research_test.go:217:15: undefined: researchDelegationView
FAIL	github.com/Tencent/WeKnora/internal/handler/session [build failed]
FAIL
```

失败形态与计划 Step 2 的预期一致（`undefined: NewWorkbenchResearchHandler` 编译错误，非迁移装载错误）。

### GREEN（Step 3→4 验证 1）

```
$ go test ./internal/handler/session/ -run 'TestDelegateResearch|TestListResearch|TestCompleteResearch|TestAnnotateMaterial|TestListAnnotations' -count=1 -v
=== RUN   TestDelegateResearchPersistsReadOnlyDelegation
--- PASS: TestDelegateResearchPersistsReadOnlyDelegation (0.00s)
=== RUN   TestDelegateResearchRejectsSourceOutsideTenantScope
--- PASS: TestDelegateResearchRejectsSourceOutsideTenantScope (0.00s)
=== RUN   TestDelegateResearchRequiresObjectiveAndSources
--- PASS: TestDelegateResearchRequiresObjectiveAndSources (0.00s)
=== RUN   TestListResearchFallsBackToGrantedReader
--- PASS: TestListResearchFallsBackToGrantedReader (0.00s)
=== RUN   TestCompleteResearchCASConflictsOnCompleted
--- PASS: TestCompleteResearchCASConflictsOnCompleted (0.00s)
=== RUN   TestCompleteResearchCrossTaskDelegationIsUniform404
--- PASS: TestCompleteResearchCrossTaskDelegationIsUniform404 (0.00s)
=== RUN   TestAnnotateMaterialPinsCurrentArtifactVersion
--- PASS: TestAnnotateMaterialPinsCurrentArtifactVersion (0.00s)
=== RUN   TestAnnotateMaterialRejectsCollaboratorRoleForViewer
--- PASS: TestAnnotateMaterialRejectsCollaboratorRoleForViewer (0.00s)
=== RUN   TestAnnotateMaterialRejectsStaleBaseVersion
--- PASS: TestAnnotateMaterialRejectsStaleBaseVersion (0.00s)
=== RUN   TestAnnotateMaterialUnknownMaterialIsUniform404
--- PASS: TestAnnotateMaterialUnknownMaterialIsUniform404 (0.00s)
=== RUN   TestListAnnotationsReadableByGrantedViewer
--- PASS: TestListAnnotationsReadableByGrantedViewer (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/handler/session	2.059s
```

11 个测试函数全部 PASS（计划预估「8 项」，-run 正则实际匹配 11 个测试函数，以实跑为准；无失败、无跳过）。

### Step 4 三条验证（独立串行执行）

```
$ go test ./internal/handler/session/ -run 'TestDelegateResearch|TestListResearch|TestCompleteResearch|TestAnnotateMaterial|TestListAnnotations' -count=1
ok  	github.com/Tencent/WeKnora/internal/handler/session	3.743s

$ go build ./...
# github.com/Tencent/WeKnora/cmd/desktop
ld: warning: ignoring duplicate libraries: '-lc++'
# github.com/Tencent/WeKnora/cmd/server
ld: warning: ignoring duplicate libraries: '-lc++'
（exit=0；两行 ld warning 为本机既有链接器噪音，非错误）

$ go vet ./internal/router/ ./internal/container/
（无输出，exit=0）
```

### 额外回归（超出计划要求的取证）

```
$ gofmt -l <6 个改动文件>          → 无输出（全部已格式化）
$ go test ./internal/router/ ./internal/container/ -count=1
ok  github.com/Tencent/WeKnora/internal/router     17.854s
ok  github.com/Tencent/WeKnora/internal/container   7.002s
$ go test ./internal/handler/session/ -count=1
ok  github.com/Tencent/WeKnora/internal/handler/session   76.555s
$ go test ./internal/application/repository/ -run 'TestTaskResearchStore|TestTaskAnnotationStore' -count=1
ok  github.com/Tencent/WeKnora/internal/application/repository   5.644s   （Task 1 前置回归）
```

## 3. 与计划的两处容器装配偏差（均有实证，非猜测）

计划 Step 3 给出的容器代码存在两处会破坏生产启动的缺陷，已按最小偏差修正并在 `workbench.go` 注释中留档：

**偏差 1：`NewWorkbenchResearchHandler` 只返回 handler，不返回 `(handler, ResearchSourceAuthorizer)` 双元组。**
- 计划原文让该 provider 双返回 `session.ResearchSourceAuthorizer`，同时 `container.go` 又 `Provide(NewResearchSourceAuthorizer)` —— 同一类型两个 provider。
- 实证（dig v1.19.0，本仓 go.mod:79；最小复现程序 `/tmp/digdup`，`go run` 实跑输出）：
  ```
  provide NewA err: <nil>
  provide NewB (tuple with duplicate Iface) err: cannot provide function "main".NewB (...): cannot provide main.Iface from [1]: already provided by "main".NewA (...)
  ```
  dig 源码 `provide.go:658/667`（checkKey）证实重复提供在 **Provide 时**即报错。经 `must()`（container.go:1273）包装会在容器构建（应用启动）时 panic。计划所称「双返回值 provider 由 dig 编译期接受」不成立——dig 是运行时图，且会拒绝。
- 修正：去掉第二个返回值；handler 的第 6 参仍按计划内联 `NewResearchSourceAuthorizer(db)`，语义不变。

**偏差 2：grant service 内联构造，provider 签名取 `(db, runs, messages, sessions, members)` 而非 `(db, runs, messages, grants *service.TaskGrantService)`。**
- 全容器包 grep 证实 `*service.TaskGrantService` 与 `*repository.TaskGrantStore` 均从未被 Provide（`grep -rn 'Provide(.*TaskGrantStore)|Provide(.*TaskGrantService)' internal/ --include='*.go'` → 零命中）；既有 `NewWorkbenchTaskGrantsHandler` 是内联构造该 service（workbench.go:145-153）。
- 实证（同一 `/tmp/digdup` 程序）：`dig.In` 的 `optional:"true"` 字段在「provider 已注册但其传递依赖缺失」时 Invoke 成功但字段注入 **nil**（实跑输出 `Use result: handler is nil`）。若按计划签名取 `*service.TaskGrantService`，本 provider 不可构建 → RouterParams 字段静默为 nil → `RegisterWorkbenchResearchRoutes` 因 `h == nil` 直接 return → 研究端点在生产装配中静默不存在，且一切测试仍绿（测试不经容器）。
- 修正：与 `NewWorkbenchTaskGrantsHandler` 同款内联形态——取容器已提供的 `interfaces.SessionRepository`（repository.NewSessionRepository，container.go:207）与 `interfaces.TenantMemberRepository`（container.go:191），从 `db` 内联 `repository.NewTaskGrantStore(db)` + `service.NewTaskGrantService(...)`，provider 全链可解析，路由真实挂载。

**router.go 的必要机械补全**：计划 Files 清单只写了「`RegisterWorkbenchDeliveryRoutes` 调用行之后加一行」，但 `params.WorkbenchResearchHandler` 需要 `RouterParams` 字段才可编译——已在同文件 `WorkbenchDeliveryHandler` 字段旁补 `WorkbenchResearchHandler *session.WorkbenchResearchHandler optional:"true"` 一行。

## 4. 自检发现（供主控/后续任务知悉）

1. **既有潜在装配缺口（非本任务引入，未改动）**：`*repository.TaskGrantStore` 无独立 Provide，故既有 `NewWorkbenchTaskGrantsHandler` 在生产装配中同样静默为 nil（`/workbench/tasks/:task_id/grants` 路由依赖 optional 字段，实际不挂载）。本任务**没有**顺手补 `Provide(repository.NewTaskGrantStore)`——那会改变 #42 既有特性的装配行为，超出本任务授权；若主控裁决要修，是独立一行 `must(container.Provide(repository.NewTaskGrantStore))` 并同时让 `NewWorkbenchTaskGrantsHandler`/`NewWorkbenchResearchHandler` 收敛为注入同一实例（消除内联重复构造）。
2. gin 通配符一致性核实：`/:run_id` 在 writes 树与既有 delivery 树同名；`:delegation_id` 仅出现在 `/research/:delegation_id/summary` 子路径，与 delivery 的 `/:run_id/delivery/:delivery_id/*` 分属不同静态子树，无 gin 冲突；router 包全量测试（含真实 engine 注册）17.854s 全绿佐证。
3. `interfaces.MessageService` 确认含 `GetSessionArtifactRefs`（internal/types/interfaces/message.go:69），满足 `session.ArtifactRefReader`，容器把 `messages` 作为 refs 注入与既有 `NewWorkbenchArtifactHandler` 同款。
4. 迁移轨道：本 worktree 的两张表迁移已被主控改号为 sqlite `000121` / versioned `000201`（Task 0 代执行时裁决），DDL 与计划逐字一致；本任务未触碰迁移。

## 5. 提交

- 代码 + 报告：`git add` 上述 6 个 Go 文件与本报告后一次提交（SHA 见 submit_result）。
- 提交信息：`feat(workbench): read-only research delegation and version-pinned annotation endpoints (T17 #47 task 2)`

---

# 修复轮 1/5 报告（审查发现：TaskGrantStore 全仓零 Provide → grants 路由静默不挂载）

> 主控裁决：Task 2 自检发现第 1 条的装配缺口定为 important，由本任务修复轮独立修复。审查引用位置（container.go:292 Provide、workbench.go:147-157 store 参数、router.go:71 optional 字段、routes_workbench.go:233-236 nil early-return）均逐一核实属实。

## R1. 修复内容（TDD：RED → GREEN）

**RED（先写失败测试）**：新建 `internal/container/task_grant_wiring_test.go`，仿既有 `retrieve_registry_wiring_test.go` 的「小 dig 容器 + 内存 sqlite + Invoke 断言」pin 装配先例。测试以**非 optional** 的 `dig.In` 参数结构请求 `*session.WorkbenchTaskGrantsHandler` 与 `*session.WorkbenchResearchHandler`（生产 RouterParams 两字段都是 `optional:"true"`，会把不可构建静默成 nil——测试有意去掉 optionality，让缺失 provider 变成响亮失败）。实跑（不含修复行）：

```
$ go test ./internal/container/ -run TestTaskGrantsAndResearchHandlersBuildable -count=1
--- FAIL: TestTaskGrantsAndResearchHandlersBuildable (0.00s)
    task_grant_wiring_test.go:74: container could not build the workbench collaboration handlers: could not build arguments for function ...task_grant_wiring_test.go:65: failed to build *session.WorkbenchTaskGrantsHandler: missing dependencies for function ...NewWorkbenchTaskGrantsHandler (workbench.go:150): missing type: *repository.TaskGrantStore
FAIL	github.com/Tencent/WeKnora/internal/container	5.030s
```

失败根因与审查发现逐字对应：`missing type: *repository.TaskGrantStore`。

**GREEN（最小修复，3 处）**：
1. `internal/container/container.go` —— 在 `must(container.Provide(NewWorkbenchTaskGrantsHandler))` 之前新增 `must(container.Provide(repository.NewTaskGrantStore))`（含注释说明根因与 pin 测试名）。这一行同时修复两件事：#42 的 grants 三路由（`POST|GET /workbench/tasks/:task_id/grants`、`DELETE /:task_id/grants/:grantee_id`，routes_workbench.go:238-240）从「静默不挂载」恢复为真实挂载；本任务 research provider 的 grant service 依赖可解析。
2. `internal/container/workbench.go` —— `NewWorkbenchResearchHandler` 收敛为注入 `grants *repository.TaskGrantStore`（与 `NewWorkbenchTaskGrantsHandler` 同款参数形态），共享同一 store 实例，消除此前内联 `repository.NewTaskGrantStore(db)` 的双实例构造；函数注释同步改写（原「容器无 TaskGrantStore Provide 故内联」的偏差 2 说明更新为收敛后形态；偏差 1 的单返回值说明保留，仍然成立）。
3. `internal/container/task_grant_wiring_test.go` —— 提供集补上 `provide(repository.NewTaskGrantStore)`（与生产提供集镜像），并把该子集 pin 住：今后任何人删掉生产 Provide 行导致装配退化时，此测试虽不能直接红（它自带提供集），但 container 包内该子图与生产行为的一致性由注释锚定；真正防止回归的是本测试对「两 handler 必须可构建且非 nil」的持续断言。

## R2. 修复后测试与回归（全部实跑取证）

```
$ go test ./internal/container/ -run TestTaskGrantsAndResearchHandlersBuildable -count=1 -v
PASS
ok  	github.com/Tencent/WeKnora/internal/container	8.322s

$ gofmt -l internal/container/{task_grant_wiring_test.go,container.go,workbench.go}
（无输出，全部已格式化）

$ go build ./...
（exit=0；仅既有 cmd/desktop、cmd/server 的 ld: warning 噪音）

$ go vet ./internal/router/ ./internal/container/
（无输出，vet_ok）

$ go test ./internal/container/ ./internal/router/ -count=1
ok  	github.com/Tencent/WeKnora/internal/container	10.828s
ok  	github.com/Tencent/WeKnora/internal/router	35.150s

$ go test ./internal/handler/session/ -run 'TestDelegateResearch|TestListResearch|TestCompleteResearch|TestAnnotateMaterial|TestListAnnotations' -count=1
ok  	github.com/Tencent/WeKnora/internal/handler/session	1.999s
```

## R3. 行为变化声明（供主控与后续审查知悉）

1. **生产行为变化（即审查要求的目标行为）**：修复后 `RouterParams.WorkbenchTaskGrantsHandler` 在完整容器装配下非 nil，`RegisterWorkbenchTaskGrantRoutes` 真实执行——#42 的三条 grants 路由从「从未挂载」变为挂载。gin 通配符安全：`/workbench/tasks` 组的 archive（routes_workbench.go:221）、grants（:237）、compliance purge（:270）各 verb 树统一 `:task_id` 命名（grants DELETE 树的 `:grantee_id` 为更深层唯一名），router 包全量测试 35.150s 全绿佐证真实 engine 注册无冲突。
2. grants 路由挂载后的请求面行为由既有 `internal/handler/session/workbench_task_grants_test.go` 等覆盖（owner-only 写、跨租户统一 404），本次修复只恢复装配，不改任何 handler/service/store 语义。
3. `NewWorkbenchResearchHandler` 签名从 `(db, runs, messages, sessions, members)` 变为 `(db, runs, messages, grants, sessions, members)`——该函数是 Task 2 本轮新引入的容器私有装配，无外部调用方受影响（全仓 `go build ./...` exit=0 佐证）。

## R4. 提交

- 修复轮提交：`internal/container/container.go` + `internal/container/workbench.go` + `internal/container/task_grant_wiring_test.go` + 本报告追加，一次提交（SHA 见 submit_result）。

---

# Task 3 报告：Go 端到端证据——AC1/AC2 服务端全链

> 执行者：实现员-t47-任务3（subagent-driven-development）。工作目录：`.worktrees/issue30-sweep-t47`，分支 `codex/issue30-t47`，起点 HEAD `979fce038`（Task 2 修复轮 1 之后，工作树干净）。

## T3.1 任务与交付物

按 `plan-t47.md` Task 3 实施。本任务无新实现，测试即交付：

**新建（唯一授权文件）：**
- `internal/application/repository/task_research_http_test.go` —— 三条 E2E 测试（计划正文写「四条 AC1×2+AC2×2」，实际为 3 个测试函数：`TestTaskResearchEndToEndParallelReadOnlyDelegations`、`TestTaskResearchEndToEndSourceOutsideTenantScope`、`TestTaskResearchEndToEndAnnotationVersionImmutability`，与计划代码块逐字一致）：
  - 装配完全走真实链路：`openTaskGrantDB(t)`（全量 sqlite 迁移真实库）+ 真实 `AgentRunStore.Admit`（占住 r1 写槽）+ 真实 `TaskGrantStore.UpsertGrant`（u2 viewer / u3 collaborator grant 行）+ 真实 `TaskGrantService` / `TaskResearchStore` / `TaskAnnotationStore` / `MessageRepository` / `KnowledgeBaseRepository`，经真实 gin engine + httptest 全链，无任何 mock service。
  - `researchGateAdapter` 镜像容器生产 gate（`container.researchSourceAuthorizer`，workbench.go:206-214）的租户绑定 KB 校验行为。
  - AC1 证据：第二个写 admission 撞单写槽 `ErrRunActive`（既有语义对照）的同时，两个只读委派 201 共存；granted viewer u2 经真实 SQL JOIN 读到委派列表；owner CAS 完成摘要、重放 409；协作者 u3 写 grants 被 403（委派不放宽 task_grants 面）。
  - AC1 证据（源越权）：不存在源与「存在于租户 2 的源」均在任何落库前被 400 `research_source_out_of_task_grant` 拒绝；租户 2 探测统一 404。
  - AC2 证据：批注钉定当前版本身份（ContentHash 前 16 hex，`artifactVersionOf`）；写 Run 产出 m2 新版本后，携带过期 `base_version` 的批注 409 `annotation_base_version_conflict`；已批注 m1:0 的 version/digest 字节不变；批注列表只含 v1 身份。

## T3.2 与计划代码的一处偏差（编译必然性，非设计变更）

计划代码块中有一行 `readHandler := session.NewWorkbenchReadHandler(runs, repository.NewAgentRunSnapshotRepository(db)).WithGrantedRuns(runs)`，构造后从未使用——Go 对未使用的局部变量是编译错误（`declared and not used`），且 `NewWorkbenchReadHandler` 不在 Task 3 声明的 Interfaces/Consumes 清单（计划 1588 行）中，三条测试也不触达 read face。落盘时**删除该行**，其余逐字保留。同包先例 `task_collaboration_http_test.go:53-65` 中 readHandler 之所以合法，是因为它注册了 `GetWorkbenchExecution`/`GetWorkbenchSnapshot` 两条路由；本测试不需要 read face，注册未使用路由属于噪音，故选择删除而非补路由。

## T3.3 测试命令与完整输出（全部实跑）

**Step 2 主验证（计划指定命令）：**
```
$ go test ./internal/application/repository/ -run 'TestTaskResearchEndToEnd' -count=1 -v
=== RUN   TestTaskResearchEndToEndParallelReadOnlyDelegations
--- PASS: TestTaskResearchEndToEndParallelReadOnlyDelegations (2.25s)
=== RUN   TestTaskResearchEndToEndSourceOutsideTenantScope
--- PASS: TestTaskResearchEndToEndSourceOutsideTenantScope (1.81s)
=== RUN   TestTaskResearchEndToEndAnnotationVersionImmutability
--- PASS: TestTaskResearchEndToEndAnnotationVersionImmutability (1.74s)
PASS
ok  	github.com/Tencent/WeKnora/internal/application/repository	7.780s
```

**防伪检查（计划 Step 2 要求）：** 临时把 `TestTaskResearchEndToEndParallelReadOnlyDelegations` 循环内第二个委派源改为 `kb-secret`，实跑确认断言真实生效：
```
$ go test ./internal/application/repository/ -run 'TestTaskResearchEndToEndParallelReadOnlyDelegations' -count=1
    Error:      	Not equal:
    	            	expected: 201
    	            	actual  : 400
    	Messages:    只读委派 #2 必须与活跃写 Run 并行（AC1）: {"code":"research_source_out_of_task_grant","error":"research source is outside the task's tenant knowledge scope","success":false}
FAIL
FAIL	github.com/Tencent/WeKnora/internal/application/repository	3.120s
```
改回后复跑：3/3 PASS（`ok ... 8.000s`，输出同上形态）。防伪闭环：越权源确实被服务端在任何落库前拒绝。

**Step 2 回归（计划指定命令）：**
```
$ go test ./internal/application/repository/ -run 'TestTaskCollaboration|TestTaskGrant' -count=1
ok  	github.com/Tencent/WeKnora/internal/application/repository	9.893s

$ go test ./internal/handler/session/ -run TestListWorkbenchArtifacts -count=1
ok  	github.com/Tencent/WeKnora/internal/handler/session	2.662s
```

**提交前全量（实现员模板要求，触碰包全量套件 + vet）：**
```
$ go vet ./internal/application/repository/
（无输出，VET_OK）

$ go test ./internal/application/repository/ -count=1
ok  	github.com/Tencent/WeKnora/internal/application/repository	509.541s
```
未跑整个仓库 `./...` 套件（唯一改动是 repository 包新增一个 _test.go 文件，不影响其它包编译产物；`./internal/handler/session/`、`./internal/container/` 等已由上面定点回归与 Task 2 轮全量覆盖）。

## T3.4 自检发现

1. 迁移号差异（非本任务造成）：task_research 两表迁移在本 worktree 实际为 sqlite `000121` / versioned `000201`（Task 0 代执行时按计划差异记录第 6 条「被同批其它计划先占则整体顺延，DDL 零变化」裁决），计划正文写的 000119/000198 号段已被占。E2E 经 `openTaskGrantDB` 装载真实迁移轨道，对号段无感知，测试只依赖表结构。
2. `TestTaskResearchEndToEndParallelReadOnlyDelegations` 中两个委派的完成断言取 `list.Data.Items[0]`：列表按 `created_at ASC, id ASC` 排序，两次 POST 间隔毫秒级、id 为 uuid，`items[0]` 是哪一个不确定——但两个委派都是 assigned，断言（200 后重放 409）与顺序无关，属确定性断言。
3. AC2 中 m1/m2 种子消息的 `created_at` 若同刻 tie，`GetSessionArtifactRefs` 的 `created_at ASC` 顺序不影响任何断言：长度断言与 `byID` 映射都与顺序无关。
4. `Message.BeforeCreate` 无条件重生成 ID（message.go:509）——种子消息按计划要求走 `SkipHooks: true`，保住 (message_id, index) 材料寻址；实跑通过佐证该处理正确。

## T3.5 提交

- `internal/application/repository/task_research_http_test.go` + 本报告追加，一次提交（SHA 见 submit_result）。
- 提交信息：`test(workbench): T17 research delegation and annotation version immutability E2E (T17 #47 task 3)`
