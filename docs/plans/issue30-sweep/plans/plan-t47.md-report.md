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

---

# Task 4：contracts——research wire 类型与解析器

> 执行者：实现员-t47-任务4（subagent-driven-development）。工作目录：`.worktrees/issue30-sweep-t47`（分支 `codex/issue30-t47`，起点 `30ca6645d` 即 Task 3 提交）。

## T4.1 任务与交付物

按 `plan-t47.md` Task 4（contracts——research wire 类型与解析器）实施，严格 RED→GREEN→提交：

**新建：**
- `packages/contracts/src/mobile/research.ts`（计划原文逐字）——`ResearchStatusWire`/`ResearchDelegationWire`/`ResearchListWire`/`AnnotationWire`/`AnnotationListWire` 五个 wire 类型 + `parseResearchListResponse`/`parseAnnotationListResponse` 两个整体拒绝解析器（信封 `{success:true, data:{items:[...]}}`，任何字段缺失/类型不符即抛错，不部分渲染）。
- `packages/contracts/src/mobile/research.test.ts`（计划原文逐字）——3 个 node:test 测试：真实信封接受（含 completed+summary）、15 种畸形信封整体拒绝、批注信封接受 + 4 种畸形行拒绝。

**修改（纯追加 2 行，未动他人导出）：**
- `packages/contracts/src/index.ts:740-741` —— 末尾追加计划指定的两行根导出（`parseResearchListResponse`/`parseAnnotationListResponse` 与 5 个 type 导出）。

**键集核对（逐字）：** 与 Task 2 实际落盘 handler 的 JSON tag 逐字一致——`researchDelegationView`（`internal/handler/session/workbench_research.go:119-127`：`delegation_id`/`run_id`/`session_id`/`objective`/`sources`/`status`/`summary,omitempty`/`created_at`）与 `researchAnnotationView`（同文件 :144-152：`annotation_id`/`run_id`/`material_id`/`base_version`/`body`/`author_id`/`created_at`）。本 worktree HEAD 上亲眼核实（sed -n '110,175p'）。

## T4.2 TDD RED 证据

先写测试、实跑确认预期失败（实现文件尚不存在）：

```
$ pnpm exec tsx --test packages/contracts/src/mobile/research.test.ts 2>&1 | tail -25
undefined
 ERR_PNPM_RECURSIVE_EXEC_FIRST_FAIL  Command "tsx" not found
```

（首跑暴露 worktree 环境前提差异，见 T4.4 第 1 条；`pnpm install` 后重跑：）

```
$ pnpm exec tsx --test packages/contracts/src/mobile/research.test.ts 2>&1 | tail -25
#   code: 'ERR_MODULE_NOT_FOUND',
#   url: 'file:///Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t47/packages/contracts/src/mobile/research.ts'
# }
# Node.js v22.22.3
# Subtest: packages/contracts/src/mobile/research.test.ts
not ok 1 - packages/contracts/src/mobile/research.test.ts
  ---
  duration_ms: 2021.935875
  type: 'test'
  location: '.../packages/contracts/src/mobile/research.test.ts:1:1'
  failureType: 'testCodeFailure'
  exitCode: 1
  error: 'test failed'
  ...
1..1
# tests 1
# pass 0
# fail 1
```

失败形态与计划 Step 2 预期一致（模块不存在，`ERR_MODULE_NOT_FOUND` 指向 `./research.ts`）。注：本段输出取自 `tail -25`，错误块最前一两行（`Cannot find module` 文本行）未入捕获，但错误码、目标 URL 与 fail=1 计数完整。事后曾尝试临时移走实现文件复现完整 RED 头部，被 Mimosa PreToolUse hook 拒绝（Bash 直接操作源文件绕过安全扫描）；RED 时序真实（实现在测试失败之后才落盘），不影响证据效力。

## T4.3 GREEN 与提交前验证（全部实跑）

**GREEN（计划 Step 4 指定命令，完整输出）：**

```
$ pnpm exec tsx --test packages/contracts/src/mobile/research.test.ts
TAP version 13
# Subtest: parseResearchListResponse accepts a real envelope
ok 1 - parseResearchListResponse accepts a real envelope
# Subtest: parseResearchListResponse rejects malformed envelopes wholesale
ok 2 - parseResearchListResponse rejects malformed envelopes wholesale
# Subtest: parseAnnotationListResponse accepts a real envelope and rejects malformed rows
ok 3 - parseAnnotationListResponse accepts a real envelope and rejects malformed rows
1..3
# tests 3
# pass 3
# fail 0
```

**提交前包级全量（实现员模板要求；test/ 目录 16 个既有测试文件 + 本任务 src 测试一并跑）：**

```
$ pnpm exec tsx --test packages/contracts/test/*.test.ts packages/contracts/src/mobile/research.test.ts
1..88
# tests 88
# pass 88
# fail 0
```

**类型检查（确认 index.ts 追加未破坏 barrel，全入口 tsc）：**

```
$ pnpm exec tsc --noEmit --allowImportingTsExtensions --module nodenext --moduleResolution nodenext --target es2022 --strict --skipLibCheck packages/contracts/src/index.ts
（无输出，exit=0）
```

**定点回归（index.ts 改动前的双保险，后被 88 全量覆盖）：**

```
$ pnpm exec tsx --test packages/contracts/test/mobile-knowledge-evidence.test.ts packages/contracts/test/mobile-read-models.test.ts
# tests 9 / # pass 9 / # fail 0
```

## T4.4 自检发现

1. **环境前提差异（非代码改动）**：计划头部声明「前置 pnpm install 已就绪」针对的是主 sweep worktree；本任务 worktree（issue30-sweep-t47）无 node_modules，首次测试命令以 `Command "tsx" not found` 失败。实跑 `pnpm install --prefer-offline`（Done in 1m 26.6s）后恢复。node_modules 被 gitignore（`.gitignore:21`），不产生 tracked 改动；后续 TS 任务（5–8）在本 worktree 不必重装。
2. **summary 语义与服务端闭环**：解析器对 `summary` 为「缺省合法、present 但空串拒绝」。与服务端实际行为闭环：assigned 委派 summary="" 被 handler 的 `omitempty`（workbench_research.go:126）省略键 → 客户端读作 undefined（合法）；completed 委派的 summary 必非空（CompleteResearch 在 :256 拒绝空 summary）→ present 且非空（合法）。「present 但 ''」不可能由服务端产生，解析器拒绝属 fail-closed，无真实误伤面。
3. **纯追加边界**：三处改动全部为新建文件或文件末尾追加（index.ts 739→741 行），未触碰任何既有导出与他人逻辑；`git show --stat HEAD` 确认 3 files changed, 166 insertions(+), 0 deletions。
4. **迁移号/task 编号无关性**：本任务不触 Go 侧，Task 3 报告提到的迁移号顺延（000121/000201）对 contracts 无影响。

## T4.5 提交

- 代码提交：`a7e56dc80` `feat(contracts): research delegation and annotation wire contracts (T17 #47 task 4)`（3 文件：research.ts 新建 111 行、research.test.ts 新建 53 行、index.ts 追加 2 行）。
- 报告追加：本 T4 章节，随后的 docs 提交（SHA 见 submit_result）。


## T5.1 任务与交付物（实现员-t47-任务5）

按 `plan-t47.md` Task 5（api-client——createMobileResearchRemote）实施，严格 RED→GREEN→验证→提交：

**新建：**
- `packages/api-client/src/mobile/research.ts` —— `createMobileResearchRemote`（计划逐字代码）：
  - `ResearchRemote` 五方法：`delegate`（POST `/api/v1/workbench/executions/:runId/research`，body `{ objective, sources }`）、`list`（GET 同路径）、`complete`（POST `.../research/:delegationId/summary`，body `{ summary }`）、`annotate`（POST `.../annotations`，body `{ material_id, base_version, body }` snake wire）、`annotations`（GET 同路径）。
  - 行类型 `ResearchDelegationRow` / `ResearchAnnotationRow`（camelCase 语义行，与计划 Produces 块逐字一致；与 mobile-core `ResearchBackendPort` 的结构一致性由 Task 7 apps/mobile typecheck 证明，本包不反向依赖 mobile-core）。
  - 行投影经 Task 4 真实解析器 `parseResearchListResponse` / `parseAnnotationListResponse`（`@weknora/contracts` 根导出，index.ts:740），任何字段缺失/类型不符整体抛错——不部分渲染。
  - 错误语义：`annotate` 的 `ApiError.status === 409` → `Error('RESEARCH_BASE_VERSION_CONFLICT')`；其余失败（非 2xx、畸形信封、传输失败）统一 `Error('RESEARCH_BACKEND')`；message 即裸错误码且 `.code` 属性同值（task-office.ts:256-266 范式）；catch 一律重抛（`rethrow` 返回 `never`），不会把 rejection 变 resolved。
  - 构造期 `requireDeploymentOrigin` 强校验 origin；runId/delegationId 经 `encodeURIComponent` 进路径；不新建传输、不持有 token（授权通道 `request: (ClientRequest) => Promise<unknown>` 注入）。
- `packages/api-client/src/mobile/research.test.ts` —— 5 个契约测试（计划原文逐字），请求替身捕获 `ClientRequest` 断言 `method` / 相对 `path` / `body`（materials.test.ts:27-52 同款形态）。

**修改（追加一行，未动他人逻辑）：**
- `packages/api-client/package.json` —— exports 表在 `"./mobile/code-delivery"` 行后追加 `"./mobile/research": "./src/mobile/research.ts"`（package.json:19）。

**消费的前置接口（全部亲眼核实后才动工）：**
- Task 4 产出：`packages/contracts/src/mobile/research.ts` 的两个解析器 + 根导出 `packages/contracts/src/index.ts:740-741`。
- `ClientRequest`（`packages/api-client/src/client.ts:38-47`：`method`/`path`/`body` 可选字段）。
- `ApiError`（`packages/api-client/src/errors.ts:24-38`：`status?: number` + `code: string`）。
- `requireDeploymentOrigin`（`packages/api-client/src/mobile/deployment-origin.ts:7-17`）。

## T5.2 TDD RED 证据

Step 1 写测试（文件创建后）Step 2 实跑：

```
$ pnpm exec tsx --test packages/api-client/src/mobile/research.test.ts

  code: 'ERR_MODULE_NOT_FOUND',
  url: 'file:///Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t47/packages/api-client/src/mobile/research.ts'
}

Node.js v26.7.0
✖ packages/api-client/src/mobile/research.test.ts (1564.182625ms)
ℹ tests 1
ℹ pass 0
ℹ fail 1
```

失败形态 = `ERR_MODULE_NOT_FOUND`（research.ts 不存在），与计划 Step 2「Expected: FAIL（模块不存在）」逐字一致。

## T5.3 GREEN 与提交前验证（全部实跑）

计划 Step 4 命令（新测试 + materials 回归）：

```
$ pnpm exec tsx --test packages/api-client/src/mobile/research.test.ts && pnpm exec tsx --test packages/api-client/src/mobile/materials.test.ts
✔ delegate POSTs to the research endpoint and projects a semantic row (7.216083ms)
✔ complete posts the summary to the delegation summary endpoint (0.465958ms)
✔ list/annotations GET the run-scoped endpoints and map rows (6.4465ms)
✔ annotate maps a 409 stale base version to RESEARCH_BASE_VERSION_CONFLICT (0.776541ms)
✔ generic failures and malformed envelopes map to RESEARCH_BACKEND with the code attached (1.710334ms)
ℹ tests 5  ℹ pass 5  ℹ fail 0  ℹ skipped 0
✔ material remote maps the artifact list wire and degrades without the terminal flag (2.06175ms)
✔ material remote posts signed-url with the numeric index and maps the grant (0.289416ms)
✔ terminalLog reports undefined when the cursor did not advance (end of log) (0.2595ms)
✔ material remote pages the terminal log with after/limit query facets (0.153208ms)
✔ material remote projects evidence events from the run snapshot (0.326084ms)
✔ material remote requires a credential-free absolute https origin (0.497625ms)
ℹ tests 6  ℹ pass 6  ℹ fail 0  ℹ skipped 0
```

提交前包级全量（implementer 模板要求，非计划命令的替代——计划命令原样先跑过）：

```
$ pnpm exec tsx --test "packages/api-client/src/mobile/*.test.ts"
ℹ tests 112
ℹ pass 108
ℹ fail 0
ℹ skipped 4      # opt-in 集成用例（runtime/task-office integration，缺真实凭据显式 skip）
ℹ duration_ms 9866.197791
```

定向类型检查（根 `typecheck:shared` 为固定文件列表不含新文件，故以同 flags 对新文件单独取证）：

```
$ pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2022 --lib ES2022,DOM \
    --module NodeNext --moduleResolution NodeNext --allowImportingTsExtensions \
    packages/api-client/src/mobile/research.ts && echo "TYPECHECK OK"
TYPECHECK OK
```

## T5.4 自检发现

- 测试真实行为而非 mock 行为：wire 形态（method/path/body 键集）经捕获的 `ClientRequest` 断言；camelCase 投影一律经真实 contracts 解析器（替身只提供 wire 字节形态），畸形信封分支也走真实 `unwrap`。
- `ResearchRemote.status` 类型为字面量 `'assigned' | 'completed'`（源自 contracts `ResearchStatusWire`），非宽化 `string`。
- 依赖方向核对：api-client 不 import mobile-core（结构一致性留给 Task 7 的 apps/mobile typecheck，计划注释原文如此声明）；research.ts 对 contracts 仅依赖根导出，不深入包内部路径。
- `getEnvelope` 中 RESEARCH_BACKEND 二次包装的短路判断按 `(error as { code?: unknown } | null)?.code` 判——unwrap 已 coded 的错误不再套壳，其余（含解析器裸 Error）包成 RESEARCH_BACKEND；5 个测试全部覆盖到这两条路径。
- 无越权改动：`git status` 提交前仅 3 个授权文件；未触碰他人文件；未派发任何子代理；未推送远端。
- 未运行 apps/mobile / mobile-core 侧任何测试（不在本任务授权范围）；Task 6/7 消费本适配器的结构可赋值性由后续任务验证。

## T5.5 提交

- 代码提交：`39799662b` `feat(api-client): mobile research remote adapter (T17 #47 task 5)`（3 文件：research.ts 新建 145 行、research.test.ts 新建 96 行、package.json 追加 1 行）。
- 报告追加：本 T5 章节，随后的 docs 提交（SHA 见 submit_result）。

---

# Task 6 报告：mobile-core——research 深模块（句柄、离线批注草稿、修订组合）

- worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t47`（分支 `codex/issue30-t47`）
- 需求来源：`docs/plans/issue30-sweep/plans/plan-t47.md` Task 6（行 2426–3104）
- 报告契约模板：任务指定 6.4.1 路径不存在；实读同内容 6.4.2 版
  `/Users/wuyongjun/.codex/plugins/cache/openai-curated-remote/superpowers/6.4.2/skills/subagent-driven-development/implementer-prompt.md`

## T6.1 任务与交付物

按计划逐字落盘，全部位于任务授权文件清单内（5 文件，+574 行）：

| 文件 | 内容 |
|---|---|
| `packages/mobile-core/src/research/types.ts`（新建） | `ResearchDelegationRow`/`ResearchAnnotationRow`（camelCase 语义行，与 Task 5 `ResearchRemote` 行结构逐字一致）、`ResearchBackendPort`（delegate/list/complete/annotate/annotations 5 方法）、`ResearchAnnotationDraft`/`ResearchDraftsPort`、`TaskResearchPorts`（remote 必选；commands/gate/drafts 可选并注明各自 fail-closed 语义）、`TaskResearch.open({lease})`、`TaskResearchHandle`（10 方法）、`ResearchAnnotationReceipt`/`ResearchRevisionInput`/`ResearchRevisionReceipt`/`ResearchEvent`/`ResearchStatus` |
| `packages/mobile-core/src/research/task-research.ts`（新建） | `createTaskResearch(ports)` + `ResearchError`/`ResearchErrorCode`（8 码）。行为不变量（plan-t47.md:2481）全部钉死：每个异步方法入口 `check()`（closed 或 `!leaseActive(lease)` → `RESEARCH_SCOPE_CHANGED`，零网络）；delegate/annotate/requestRevision 输入校验在任何网络派发之前；离线批注落 `drafts.put`（id `research-ann-<n>` 符合 Scoped Vault 白名单 `^[A-Za-z0-9._-]{1,64}$`）+ `annotation-drafted` 事件；`backendFailure` 把 `RESEARCH_BASE_VERSION_CONFLICT` → `RESEARCH_CONFLICT`（`failureCode` 优先读 `Error.code`、回退 message，两种错误形态都覆盖）；requestRevision 组合确定性版本钉定文本 `请基于版本 ${baseVersion} 修订材料 ${materialId}：${note}` 委托 `TaskCommandPort`，`TASK_COMMAND_CONFLICT` → `RESEARCH_COMMAND_CONFLICT`、无 commands → `RESEARCH_COMMAND_UNAVAILABLE`；`flushAnnotationDrafts` 按序重放同 run 草稿：成功移除 + `draft-flushed` 事件、conflict 保留草稿标 `conflict`、其他失败标 `failed` 继续、每草稿重放前再 `check()`、`RESEARCH_SCOPE_CHANGED` 立即中止抛出；`close(reason)` 幂等 + `scope-closed` 事件 |
| `packages/mobile-core/src/research/in-memory-research-remote.ts`（新建） | `createScenarioResearchRemote(script)` 场景 Adapter（module-seams §4.4/§7.3 同款）：脚本化委派/批注行、`conflictOnAnnotate` 冲突分支、`baseVersion==='stale'` 恒冲突、complete 未知 id 抛 `RESEARCH_NOT_FOUND`；头注释声明绝不冒充真实集成证据 |
| `packages/mobile-core/src/research/task-research.test.ts`（新建） | 5 个 Interface 级测试（计划逐字 + 两处类型修正，见 T6.2） |
| `packages/mobile-core/src/index.ts`（修改，末尾 +10 行） | barrel 导出：值 `ResearchError`/`createTaskResearch`/`createScenarioResearchRemote`，类型 `ResearchErrorCode` + types.ts 全部 13 个类型 + `ScenarioResearchRemoteScript` |

平台纯净：模块只 import 包内相对路径（runtime/offline/task-office 类型），零 RN/DOM/transport 依赖。

## T6.2 与计划的偏差（3 处，均为类型层面，运行时行为零变化）

1. **测试夹具 `newLease()` 改经 `asScopeLease()`**。计划测试直接 `open({ lease: newLease() })` 传裸 `RuntimeScopeLease`；但 `ScopeLease` 是 branded 不透明类型（`runtime/types.ts:12-18`，`readonly [scopeLeaseBrand]: never`），裸类实例在严格 tsc 下不可赋值（TS2741）。仓库既有先例都经转换：`task-office.test.ts:15-16`（`lease: revocable.asScopeLease()` + 保留 revocable 引用做 revoke）、`task-material.test.ts:36`。因此 `newLease()` 签名改为 `{ lease: ScopeLease; revoke(): void }`，各测试点改用解构/`.lease`。运行时 `leaseActive` 以 `instanceof RuntimeScopeLease` 判定（`scope-lease.ts:26-28`），`asScopeLease()` 是同一对象原样出手，可撤销性与计划语义完全一致。
2. **commands stub 的 `action` 参数改精确 union `'steer' | 'queue_next' | 'cancel'`**。计划的 `action: string` 使 stub 返回类型不可赋给 `TaskCommandPort`（TS2322）；对齐 `task-detail.test.ts:701-718` 的 `scriptedCommandPort` 先例。断言内容（`action === 'queue_next'`、钉定文本逐字相等）不变。
3. **`task-research.ts` 增加 3 个类型 re-export**（`export type { ResearchBackendPort, ResearchDraftsPort, ResearchEvent } from './types.ts';`）。计划测试第 4 行从 `./task-research.ts` 以 `import type` 导入这三个类型，而计划实现代码只 import 未 re-export——tsx 擦除 type import 故测试仍绿，但类型层面 import 会失败。加 re-export 是满足计划测试导入要求的最小改动，与 index.ts 的 barrel 导出不冲突。

## T6.3 TDD 过程与测试命令完整输出（全部本会话实跑）

### RED（计划 Step 2）

```
$ pnpm exec tsx --test packages/mobile-core/src/research/task-research.test.ts
# Error [ERR_MODULE_NOT_FOUND]: Cannot find module '.../packages/mobile-core/src/research/in-memory-research-remote.ts'
#   imported from .../packages/mobile-core/src/research/task-research.test.ts
not ok 1 - packages/mobile-core/src/research/task-research.test.ts
# tests 1 / # pass 0 / # fail 1
```

失败形态为模块不存在（`ERR_MODULE_NOT_FOUND`），与计划 Step 2 预期（「FAIL（模块不存在）」）一致。

### GREEN + 回归（计划 Step 4 原样串联命令，实现修正后复跑为最终证据）

```
$ pnpm exec tsx --test packages/mobile-core/src/research/task-research.test.ts && \
  pnpm exec tsx --test packages/mobile-core/src/material/task-material.test.ts packages/mobile-core/src/task-office/task-office.test.ts
ok 1 - delegate validates scope and input before any network dispatch
ok 2 - annotations record, conflict-map and draft offline
ok 3 - flush replays drafts in order; conflict keeps the draft; revocation aborts the rest
ok 4 - requestRevision composes a version-pinned text over the command port
ok 5 - closed handles reject every path with SCOPE_CHANGED and emit scope-closed
# tests 5 / # pass 5 / # fail 0          ← research 新测试

（material 16 项 + task-office 8 项，逐项 ok）
# tests 24 / # pass 24 / # fail 0        ← material + task-office 回归
```

### 补充自检（计划外，非计划命令替代）

```
$ pnpm exec tsx --test packages/mobile-core/src/platform-purity.test.ts
# tests 2 / # pass 2 / # fail 0

$ pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2022 --lib ES2022,DOM \
    --module NodeNext --moduleResolution NodeNext --allowImportingTsExtensions --types node \
    packages/mobile-core/src/research/types.ts packages/mobile-core/src/research/task-research.ts \
    packages/mobile-core/src/research/in-memory-research-remote.ts \
    packages/mobile-core/src/research/task-research.test.ts packages/mobile-core/src/index.ts
tsc-exit=0   （零类型错误）
```

说明：最初一次裸 tsc（未带 `--types node`）报测试文件 `node:assert/strict`/`node:test` 找不到——是调用方式缺 `--types node` 所致，非代码问题；加参数并完成 T6.2 偏差 1/2 修正后归零（偏差 1 即该轮 tsc 的真实类型发现）。

## T6.4 自检发现（供主控/后续任务知悉）

- **Completeness**：Produces 契约（plan-t47.md:2439-2479）逐项核对——错误码 8 个、Port 5 方法、句柄 10 方法、receipt/event 类型、两个工厂签名逐字一致；行为不变量（plan-t47.md:2481）9 条全部有测试钉死（零网络断言靠计数 remote 与 `assert.rejects` 前后 `calls` 不变证明）。
- **边界确认**：离线修订请求不做草稿（属 Run 命令，计划边界声明）——`requestRevision` 无任何草稿路径，fail closed，与 spec「Offline mode … prohibits Run commands」一致；离线批注草稿 id 过白名单校验（`research-ann-1` 长度 14，全合法字符）。
- **Testing**：测试验证可观察行为（错误码、事件、草稿保留/移除、网络调用计数），非 mock 内部；`recordingDrafts` 用 `structuredClone` 防引用共享假阳性；输出 pristine。
- **依赖对接**：`ResearchBackendPort` 与 Task 5 `ResearchRemote`（`packages/api-client/src/mobile/research.ts:38-44`）方法签名与行结构逐字同构，Task 7 composition 可直接把 `createMobileResearchRemote` 产物注入 `TaskResearchPorts.remote`（结构可赋值性由 Task 7 的 apps/mobile typecheck 最终证明，本模块不 import api-client，依赖方向不变）。
- **无越权改动**：提交前 `git status` 仅 5 个授权文件；未触碰他人文件；未派发任何子代理；未推送远端。
- **未运行的检查**：apps/mobile 侧 typecheck/e2e（属 Task 7/8 范围）；包级 `test:shared` 不含 mobile-core（脚本清单核实，根 `package.json:13`），故以计划指定范围 + 平台纯净性 + 严格 tsc 为准。

## T6.5 提交

- 代码提交：`257b50e2e` `feat(mobile-core): task research deep module with offline annotation drafts (T17 #47 task 6)`（5 文件 +574 行：research/ 四文件新建、index.ts 追加导出块）。
- 报告追加：本 T6 章节，随后的 docs 提交（SHA 见 submit_result）。

---

# T7：apps/mobile——ResearchScreen、/tasks/research 路由与 composition 接线（实现员报告）

执行者：T7 实现员（subagent）；worktree：`.worktrees/issue30-sweep-t47`（分支 `codex/issue30-t47`，起点 HEAD `68434694b`）。
授权文件：`apps/mobile/src/research-view.ts`、`apps/mobile/src/research-view.test.ts`、`apps/mobile/src/screens/ResearchScreen.tsx`、`apps/mobile/src/app/tasks/research.tsx`（新建）；`apps/mobile/src/composition.ts`、`apps/mobile/src/screens/MaterialsScreen.tsx`、`apps/mobile/src/app/tasks/materials.tsx`（修改）。7 个文件全部与计划 File 地图逐字一致，无越权改动。

## T7.1 实现内容

- **`apps/mobile/src/research-view.ts`**（新建，逐字按计划）：`ResearchViewState`（loading/delegations/annotations/materials/pendingDrafts/error/notice）、`RESEARCH_ERROR_COPY`（`Record<ResearchErrorCode, string>` 全 8 码文案表——键类型为 mobile-core 的 `ResearchErrorCode`，新码缺文案即类型错，与 materials-view B3-F55 同款穷尽性手法）、`createResearchController(handle, { runId, materials? })`。控制器 `load()` 用 `Promise.all` 聚合委派/批注/离线草稿计数（pendingDrafts 按 `runId` 过滤，读取失败回退 0），materials 投影仅当注入了 `materials()` 才加载（失败包含为 undefined）；`annotate` 按 receipt.status 区分在线已记录/离线加密草稿双文案；`flushDrafts` 按 conflict 计数给诚实文案；`requestRevision` 透传修订纪律参数（materialId/baseVersion/note/action/expectedRevision）；`dispose` 以 `handle.close('research-route-unmount')` 关闭句柄。
- **`apps/mobile/src/screens/ResearchScreen.tsx`**（新建，逐字按计划）：演示态研究屏，只消费 `ResearchViewState` + 回调 props；委派表单、委派投影（状态/objective/来源/summary）、材料列表点选自动填充 materialId+baseVersion（版本身份从材料索引取得，用户手填错版本会得到服务端 409 → `RESEARCH_CONFLICT` 文案）、批注表单与批注投影、修订请求表单（steer/queue_next 通道切换 + expectedRevision）。文件不 import `@weknora/contracts` / `@weknora/api-client`（测试源级断言钉死）。
- **`apps/mobile/src/app/tasks/research.tsx`**（新建，逐字按计划）：`ResearchRouteLifecycle`（effect 内 `activeTaskResearch().open({ lease })` 开句柄、卸载即 dispose，与 materials.tsx 同款生命周期宿主模式；materials 投影经 `activeTaskMaterial().open({ lease }).index({ runId })` 取材料索引用于批注表单版本身份点选）+ 默认导出 Expo Router 文件路由 `/tasks/research?runId=..`。路由文件不 import api-client，只经 composition 工厂取模块。
- **`apps/mobile/src/composition.ts`**（两处最小改动，逐字按计划）：import 区末尾（foreground-sync 之后）追加 `createTaskResearch`/`ResearchDraftsPort`/`TaskResearch` 类型与 `createMobileResearchRemote`（`@weknora/api-client/mobile/research`）导入；文件末尾（`completeNativeOidcCallback` 之后）追加 `taskResearchFor`（`cachePut(taskResearches, deploymentScopeKey(origin, tenantId))` 记忆化，同 taskMaterialFor 模式——remote 经 `activeRuntime.authorizedRequest`，`gate: nativeOfflineGate`，draftsPort 适配 `openScopedDraftStore()` 的 Scoped Vault drafts 命名空间：put 失败抛 `RESEARCH_DRAFT_UNAVAILABLE`、list 对 JSON.parse 失败行跳过、无 vault 时 undefined 由模块内 fail closed）与导出 `activeTaskResearch(): TaskResearch | undefined`（无授权面 undefined，同 activeTaskMaterial 口径）。lease 不在端口持有——唯一来源是路由处 `research.open({ lease })`。
- **`apps/mobile/src/screens/MaterialsScreen.tsx`**（最小 diff）：`MaterialsScreenProps` 增可选 `onOpenResearch?(): void`；「刷新材料」按钮旁增入口行（仅当 prop 存在渲染，`testID: 'materials-open-research'`）。计划片段以 `createElement` 书写，本文件既有风格是 JSX——按同文件 JSX 风格落地（语义逐字等价：条件渲染 + onPress + testID + 「研究与批注 →」文案）。
- **`apps/mobile/src/app/tasks/materials.tsx`**（一行）：`MaterialsScreen` 调用处传入 `onOpenResearch={runId.trim() === '' ? undefined : () => router.push(\`/tasks/research?runId=${encodeURIComponent(runId)}\`)}`。

## T7.2 TDD 过程与测试命令完整输出（全部本会话实跑）

### RED（计划 Step 2）

```
$ pnpm --filter @weknora/mobile exec tsx --test src/research-view.test.ts
Error: Cannot find module './research-view.ts'
✖ src/research-view.test.ts (13060.410959ms)
ℹ tests 1 / ℹ pass 0 / ℹ fail 1
ERR_PNPM_RECURSIVE_EXEC_FIRST_FAIL Command failed with exit code 1: tsx --test src/research-view.test.ts
```

失败形态为 `./research-view.ts` 不存在（模块解析失败），与计划 Step 2 预期逐字一致。

### GREEN + typecheck + 回归（计划 Step 4 原样串联命令，最终证据）

```
$ pnpm --filter @weknora/mobile exec tsx --test src/research-view.test.ts && \
  pnpm --filter @weknora/mobile typecheck && \
  pnpm --filter @weknora/mobile exec tsx --test src/materials-view.test.ts
✔ controller loads delegations, annotations and pending drafts (2.586875ms)
✔ controller annotate records online and drafts offline with honest copy (52.810208ms)
✔ research route and composition wiring stay at the Interface boundary (6.384167ms)
ℹ tests 3 / ℹ pass 3 / ℹ fail 0

> @weknora/mobile@0.0.0 typecheck … > tsc --noEmit        （零输出 = 通过）

✔ the controller loads the index, opens views and records grants (18.181416ms)
✔ material error codes map to user copy, never raw internals (1.315584ms)
✔ dispose closes the handle exactly once (1.666166ms)
✔ MATERIAL_ERROR_COPY covers every MaterialErrorCode (exhaustiveness) (0.281833ms)
✔ a slower stale open does not overwrite a newer state (generation token) (0.353792ms)
ℹ tests 5 / ℹ pass 5 / ℹ fail 0
```

三段全绿：research-view 3/3、typecheck 干净、materials-view 回归 5/5。

### 补充自检（计划外，非计划命令替代）

```
$ pnpm --filter @weknora/mobile exec tsx --test src/app-smoke.test.tsx
ℹ tests 66 / ℹ pass 66 / ℹ fail 0
```

改动触及 `composition.ts` / `MaterialsScreen.tsx` / `materials.tsx`，app-smoke 是它们的行为级回归面（66 项全过）。

## T7.3 自检发现（供主控/后续任务知悉）

- **Consumes 对接核实（执行前逐项亲眼核实）**：mobile-core `index.ts:117-126` 导出 `ResearchError`/`createTaskResearch`/`ResearchErrorCode` 与 `TaskResearchHandle`/`ResearchAnnotationDraft` 等全部类型；`api-client` `./mobile/research` 存在 `createMobileResearchRemote`；`MaterialEntry` 含 `materialId`/`version`；`TaskMaterialHandle.index({ runId })` 返回 `{ materials }`；composition 既有 `cachePut`（:153-160）/`deploymentScopeKey`（:166-168）/`runtime()`/`activeMobileRuntime()`/`activeTaskMaterial`（:309-314）/`openScopedDraftStore`（:65-73）/`nativeOfflineGate`。测试 stub 对 `TaskResearchHandle` 的 10 方法结构逐字可赋值（typecheck 证明）。
- **边界确认**：路由/Screen 零 wire 导入（测试源级断言 3/3）；composition 是唯一 join api-client 的位置（module-seams §3/§10）；lease 唯一来源是路由 `open({ lease })`；离线批注草稿 id `research-ann-<n>` 命中 vault `DRAFT_ID_PATTERN` 白名单；draftsPort 无 vault 时 undefined → 模块内 `RESEARCH_DRAFT_UNAVAILABLE` fail closed，不静默丢批注。
- **计划微偏差（语义等价，如实声明）**：MaterialsScreen 入口行按该文件既有 JSX 风格书写（计划片段为 createElement 形态），渲染条件/testID/文案逐字一致；`research.tsx` 保留计划原样双导出（默认路由 + `ResearchRouteLifecycle` 具名宿主），JSX 与同目录 `materials.tsx` 同款可用。
- **依赖对接**：`activeTaskResearch()` 与 `/tasks/research?runId=..` 路由即 Task 8 `research-integration-smoke` 的消费面；`expectedRevision` 在演示屏由用户手输（详情页可见），真实集成证据在 Task 8 由测试自取权威值。
- **未运行的检查**：iOS/Android 真机渲染、Expo bundler 构建不在本任务范围（与既有各屏同口径，无独立可跑命令）；端到端真实 HTTP 属 Task 8（AC3）。
- **无越权改动**：提交前 `git status --short` 仅列 7 个授权文件（3 修改 + 4 新建）；未触碰他人文件；未派发任何子代理；未推送远端。

## T7.4 提交

- 代码提交：`57cea7469` `feat(mobile): research and annotation screen with /tasks/research route (T17 #47 task 7)`（7 文件 +407/-1）。
- 报告追加：本 T7 章节，随后的 docs 提交（SHA 见 submit_result）。

## T8.1 实现内容（实现员-t47-任务8）

- **`apps/mobile/src/research-integration-smoke.ts`**（新建，逐字按计划）：`ResearchIntegrationConfig`（enabled 双态：真凭据 + 无凭据 skip / 非法 origin invalid）、`ResearchIntegrationEvidence`（`listed: 'delegated'|'no-tasks'|'no-materials'|'failed'` + 委派/批注计数 + `revision: 'not-dispatched'` 恒定）、`researchIntegrationConfig(env)`（缺凭据如实 skip；非 HTTPS / 带 path/query/凭据的 origin 拒绝；复用 `runtime-integration-smoke.ts:118` 的 `disallowedDeploymentHost` 主机防线）、`runResearchIntegration(config)`（真实 JSON transport + `runtime.signIn` 授权通道 + Task Office 列任务 + `createMobileResearchRemote` 委派/列表/批注 + `createMobileMaterialRemote` 取真实材料版本身份；委派源 'probe-kb' 被租户围栏拒时记 `delegationCreated:false` + `failure` 前缀 `delegation-rejected-by-scope-fence`——AC1 真实证据；任何步骤异常 → `listed:'failed'` + failure 摘要，从不 reject；`finally` 内 `runtime.dispose()`）、`emitResearchIntegrationEvidence`（JSONL，`kind: 'research-integration'`，无凭据字段）。修订请求恒 `'not-dispatched'`——对真实部署不触发新 Run，副作用只保留 append-only 的委派与批注记录，如实声明不伪装。
- **`apps/mobile/src/research-integration-smoke.test.ts`**（新建）：计划 Step 1 的 3 个证据契约测试逐字落地（config 缺凭据 skip + 非公网 origin 拒绝；无凭据时诚实 skip 形态；证据记录不携带凭据 + revision 恒 not-dispatched），另加第 4 个 opt-in 实跑测试（逐字沿用同目录 `material-integration-smoke.test.ts:40-49` 的 skip 门控模式：`WEKNORA_MOBILE_TEST_DEPLOYMENT_URL` 缺失即 SKIP，不断言伪造通过）——计划 Step 4 的 opt-in 命令（设凭据后重跑本测试文件）因此有真实消费点，断言 `revision === 'not-dispatched'` 与 delegated/annotated 计数类型。

## T8.2 TDD 过程与测试命令完整输出（全部本会话实跑）

### RED（计划 Step 2）

```
$ pnpm --filter @weknora/mobile exec tsx --test src/research-integration-smoke.test.ts
Error: Cannot find module './research-integration-smoke.ts'
Require stack: apps/mobile/src/research-integration-smoke.test.ts
ℹ pass 0 / ℹ fail 1
ERR_PNPM_RECURSIVE_EXEC_FIRST_FAIL Command failed with exit code 1
```

失败形态为 `./research-integration-smoke.ts` 不存在（模块解析失败），与计划 Step 2 预期（「模块不存在」）逐字一致。

### GREEN（计划 Step 4 第一段）

```
$ pnpm --filter @weknora/mobile exec tsx --test src/research-integration-smoke.test.ts
✔ researchIntegrationConfig skips without credentials and rejects non-public origins (5.623125ms)
✔ runResearchIntegration without credentials reports an honest skip shape (0.397167ms)
✔ evidence records never contain credential material (1.654167ms)
﹣ runResearchIntegration executes against a real deployment when credentials are provided (0.104291ms) # SKIP
ℹ tests 4 / ℹ pass 3 / ℹ fail 0 / ℹ skipped 1
```

opt-in 实跑测试因本机无 `WEKNORA_MOBILE_TEST_*` 凭据如实 SKIP（不伪造 AC3 真实环境证据）。

### typecheck + 全应用回归（计划 Step 4 原样串联）

```
$ pnpm --filter @weknora/mobile typecheck
> tsc --noEmit        （零输出，exit 0 = 通过）

$ pnpm --filter @weknora/mobile test
ℹ tests 233 / ℹ pass 221 / ℹ fail 0 / ℹ cancelled 0 / ℹ skipped 12
```

全应用 233 项全绿（0 fail）；12 个 skip 全部为 opt-in 真实 HTTP 用例缺凭据的诚实跳过（与本计划同模式的既有各集成证据测试一致）。

## T8.3 自检发现（供主控/后续任务知悉）

- **Consumes 对接核实（执行前逐项亲眼核实）**：`createMobileResearchRemote`（`packages/api-client/src/mobile/research.ts:109`，`delegate/list/complete/annotate/annotations` 五行签名与计划用法逐字一致）；`createMobileMaterialRemote().list(runId)` 返回 `artifacts[0].id/.version`（`materials.ts:13-17/26`）；`disallowedDeploymentHost(hostname, variable)`（`runtime-integration-smoke.ts:118`）；api-client `package.json:19` 已导出 `./mobile/research`；mobile-core 导出 `createMobileRuntime`/`createInMemoryCredentialStore`/`createTaskOffice`（`index.ts:1/2/15`）。runtime 引导段与 `material-integration-smoke.ts:55-69` 逐字同构。
- **计划微偏差（1 处，如实声明）**：测试文件在计划 3 测试之外追加了第 4 个 opt-in 实跑测试（`material-integration-smoke.test.ts:40-49` 同款 skip 门控）。依据是计划 Step 4 明文「有真实环境时另加 opt-in 实跑（缺环境如实 skip，不伪造）」及其给出的设凭据重跑命令——无该测试则该命令无真实消费点。缺凭据时该测试 SKIP，不影响证据契约 3 测试的判定。
- **AC3 诚实性声明**：本机无 `WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD`，真实 HTTP 端到端证据未在本会话产出（opt-in 测试 SKIP）。替代证据链：Task 3 Go 真实迁移 E2E + Task 6 mobile-core Interface 场景 + Task 4/5 wire 契约 + Task 7 控制器/路由 + 本任务的证据契约与全应用回归。凡具备真实环境的运行设上述环境变量重跑本测试文件即自动产出 AC3 证据。
- **副作用声明**：真实环境实跑会在探测到的首个任务上留下 1 条只读委派 + 1 条版本钉定批注（append-only，无删除端点是设计事实），委派源 'probe-kb' 预期被租户围栏 400 拒绝并如实记录为 AC1 证据；修订请求不派发。
- **无越权改动**：提交前 `git status --short` 仅列 2 个授权新建文件；未触碰他人文件；未派发任何子代理；未推送远端。

## T8.4 提交

- 代码提交：`8821bdf6d` `test(mobile): research integration evidence contract with opt-in real HTTP (T17 #47 task 8)`（2 文件 +163）。
- 报告追加：本 T8 章节，随后的 docs 提交（SHA 见 submit_result）。

---

# T8 复核轮（实现员-t47-任务9，第 9/9 任务）

## T8R.1 派发现状核实（本会话亲眼核实）

按任务定位「第 9/9 个任务」= 计划任务结构表第 9 项 = **Task 8（apps/mobile opt-in 真实集成证据 AC3）**。派发时核实（git log + git status --short + 文件读取）：

- Task 8 实现提交已存在：`8821bdf6d` `test(mobile): research integration evidence contract with opt-in real HTTP (T17 #47 task 8)`（2 文件 +163）。
- Task 8 报告追加提交已存在：`3107a6511`（上方 T8.1–T8.4 章节）。
- 工作区 clean（`git status --short` 零输出），无未提交改动、无本任务之外文件可动。

**结论**：Task 8 已由前序实现员实例完整交付。本会话不重复实现（不撤销/不覆盖他人交付），转为**独立复核轮**：逐字比对交付物与计划 Produces 契约 + 亲自重跑计划 Step 4 全部验证命令。

## T8R.2 交付物契约逐字比对（本会话实读）

对照计划 Task 8 Produces（plan-t47.md:3638-3663），逐项核实 `apps/mobile/src/research-integration-smoke.ts`：

| 契约项 | 核实位置 | 结果 |
|---|---|---|
| `ResearchIntegrationConfig` 双态（enabled true/false + disposition skip/invalid） | research-integration-smoke.ts:11-13 | 逐字一致 |
| `ResearchIntegrationEvidence` 全字段（listed 四态 + delegation/annotation 计数 + `revision: 'not-dispatched'` 恒定 + commandTimestamp） | 同文件 :15-26 | 逐字一致 |
| `researchIntegrationConfig`：缺凭据 skip；非 HTTPS / 带 path/query/凭据 origin invalid；`disallowedDeploymentHost` 主机防线 | 同文件 :29-46（防线 import 自 runtime-integration-smoke.ts:9） | 逐字一致 |
| `runResearchIntegration`：真实 JSON transport + runtime.signIn 授权通道 + Task Office 列任务 + `createMobileResearchRemote` 委派/列表/批注 + `createMobileMaterialRemote` 取真实版本身份；委派被围栏拒记 `delegationCreated:false` + `delegation-rejected-by-scope-fence` 前缀；异常 → `listed:'failed'` 从不 reject；`finally` dispose | 同文件 :56-115 | 逐字一致 |
| `emitResearchIntegrationEvidence`：JSONL，`kind:'research-integration'`，无凭据字段 | 同文件 :117-120 | 逐字一致 |
| 修订请求恒 `'not-dispatched'`（不触发新 Run） | 同文件 :23（类型恒定）+ 测试 :38 断言 | 一致 |
| 测试文件：计划 3 测试 + 第 4 个 opt-in 实跑测试（skip 门控） | research-integration-smoke.test.ts:5-30（计划 3 测试逐字）+ :32-43（opt-in） | 一致 |

## T8R.3 验证命令重跑（全部本会话实跑，非转抄前轮输出）

### ① 计划 Step 4 第一段——证据契约测试

```
$ pnpm --filter @weknora/mobile exec tsx --test src/research-integration-smoke.test.ts
✔ researchIntegrationConfig skips without credentials and rejects non-public origins (1.525875ms)
✔ runResearchIntegration without credentials reports an honest skip shape (0.266459ms)
✔ evidence records never contain credential material (0.765843ms)
﹣ runResearchIntegration executes against a real deployment when credentials are provided (0.26ms) # SKIP
ℹ tests 4 / ℹ pass 3 / ℹ fail 0 / ℹ skipped 1
```

3 pass + 1 诚实 SKIP。与上一轮 GREEN 输出一致。

### ② 计划 Step 4 第二段——typecheck

```
$ pnpm --filter @weknora/mobile typecheck
> tsc --noEmit        （零输出）
exit=0
```

### ③ 计划 Step 4 第三段——全应用回归

```
$ pnpm --filter @weknora/mobile test
ℹ tests 233 / ℹ pass 221 / ℹ fail 0 / ℹ cancelled 0 / ℹ skipped 12
```

233 项全绿（0 fail）；12 个 skip 全部为 opt-in 真实 HTTP 用例缺凭据的诚实跳过（与本计划 Task 8 同模式的既有集成证据测试一致）。三项数字与上一轮（T8.2）完全一致，无回归漂移。

## T8R.4 AC3 opt-in 真实 HTTP 证据（未产出，如实声明）

本会话核实环境：`env | grep -c WEKNORA_MOBILE_TEST` → 0（无任何 opt-in 凭据）。计划 Step 4 的设凭据重跑命令**未运行**（无凭据可设，不伪造）：真实部署端到端 AC3 证据在本会话与上一轮均未产出，依赖具备环境时重跑 `research-integration-smoke.test.ts` 的 opt-in 测试自动产出。替代证据链（全部已提交并经本会话/前轮实跑）：Task 3 Go 真实迁移 E2E + Task 6 mobile-core Interface 场景 + Task 4/5 wire 契约 + Task 7 控制器/路由 + 本任务证据契约与全应用回归。

## T8R.5 自检发现

- **本会话零代码改动**：`git status --short` 在报告追加前仅将列本报告文件；未触碰任何源码/测试文件；未派发子代理；未推送远端。
- **派发措辞观察（供主控知悉）**：派发单「前置接口」行描述的是 research-integration-smoke 三导出（`researchIntegrationConfig/runResearchIntegration/emitResearchIntegrationEvidence`）——与 Task 8 自身 Produces 逐字重合，疑为编排方复制错位或对已完成任务的重派。无论哪种，计划 9 个任务（Task 0–8）的代码与报告提交均已在树上，本复核轮确认全链有效。
- **计划级验证命令未在本会话整体重跑**：计划「计划级验证命令」一节为整计划收口命令（含 Go 侧与三 TS 包），超出本任务（Task 8）授权范围；本会话只重跑 Task 8 Step 4 的三条命令。如需计划级收口，建议由主控或终局任务执行。

## T8R.6 提交

- 本轮仅追加本报告章节（docs 提交，SHA 见 submit_result）。代码零改动——Task 8 交付维持 `8821bdf6d` 原样。

---

# T8 第二轮复核（实现员-t47-任务9，同一任务再次派发）

## T8R2.1 派发现状核实（本会话亲眼核实）

本会话再次收到「第 9/9 个任务」派发（任务定位、需求文件、前置接口行与 T8R 轮所收派发单同形——前置接口行仍是 research-integration-smoke 三导出，与 Task 8 自身 Produces 逐字重合，维持 T8R.5 的重派判定）。本会话核实（git log / git status --short / 文件读取，均为本会话实跑）：

- Task 8 代码交付 `8821bdf6d`（2 文件 +163）、报告 `3107a6511`、第一轮复核 `6a1227ce2` 均已在树上；工作区 clean（`git status --short` 零输出）。
- 本会话不重复实现（不撤销/不覆盖他人交付，TDD 的 RED 无法对已交付代码重演），执行**第二轮独立复核**：①交付物与计划代码块逐字 diff；②计划 Step 4 三段串联命令本会话亲自重跑。

## T8R2.2 交付物逐字 diff（本会话实跑）

- **实现文件**：`diff <(sed -n '3688,3807p' plan-t47.md) apps/mobile/src/research-integration-smoke.ts` → **exit=0，与计划 Task 8 Step 3 代码块逐字一致**（零差异）。
- **测试文件**：`diff <(sed -n '3643,3672p' plan-t47.md) apps/mobile/src/research-integration-smoke.test.ts` → 仅一处追加：计划 3 测试（前 30 行）逐字一致 + 第 4 个 opt-in 实跑测试（31-43 行，skip 门控）。该微偏差已由 T8.3 首次声明（依据计划 Step 4 明文「有真实环境时另加 opt-in 实跑（缺环境如实 skip，不伪造）」）并经 T8R.2 复核确认，本轮 diff 再次实证其边界精确：无其它任何改动。

## T8R2.3 验证命令重跑（本会话实跑，一条串联命令原样执行）

```
$ pnpm --filter @weknora/mobile exec tsx --test src/research-integration-smoke.test.ts && pnpm --filter @weknora/mobile typecheck && pnpm --filter @weknora/mobile test
✔ researchIntegrationConfig skips without credentials and rejects non-public origins (17.739917ms)
✔ runResearchIntegration without credentials reports an honest skip shape (0.932292ms)
✔ evidence records never contain credential material (1.712ms)
﹣ runResearchIntegration executes against a real deployment when credentials are provided (0.252625ms) # SKIP
ℹ tests 4 / ℹ pass 3 / ℹ fail 0 / ℹ skipped 1

> @weknora/mobile@0.0.0 typecheck
> tsc --noEmit        （零输出，串联继续）

ℹ tests 233 / ℹ pass 221 / ℹ fail 0 / ℹ cancelled 0 / ℹ skipped 12 / ℹ duration_ms 367314.74975
chain-exit=0
```

三段全过：证据契约 4 项（3 pass + 1 诚实 SKIP，0 fail）；typecheck 零输出；全应用 233 项（221 pass + 12 skip，0 fail）。数字与前两轮（T8.2 / T8R.3）完全一致，无回归漂移。12 个 skip 均为 opt-in 真实 HTTP 用例缺凭据的诚实跳过。

## T8R2.4 AC3 opt-in 声明（与 T8R.4 同口径）

本会话 `env | grep -c WEKNORA_MOBILE_TEST` → 0（无凭据）。设凭据重跑命令未运行（无凭据可设，不伪造）。真实部署 AC3 证据仍依赖具备环境时重跑本测试文件自动产出；替代证据链不变（Task 3 Go 真实迁移 E2E + Task 6 Interface 场景 + Task 4/5 wire 契约 + Task 7 控制器/路由 + 本任务证据契约与全应用回归，均已在树上并经本/前会话实跑）。

## T8R2.5 自检发现

- 本会话零代码改动：验证命令跑完 `git status --short` 仍零输出；报告追加前工作区仅本报告文件待提交；未触碰源码/测试；未派发子代理；未推送远端。
- 三轮独立验证（T8 实现 / T8R / T8R2）输出零漂移，Task 8 交付稳定性充分确认。
- 重申 T8R.5 建议：计划级收口命令（含 Go 侧）属主控/终局任务职责，本任务授权范围内不执行。

## T8R2.6 提交

- 本轮仅追加本报告章节（docs 提交，SHA 见 submit_result）。代码零改动——Task 8 交付维持 `8821bdf6d` 原样。

---

# T8 终局收口轮（实现员-t47-任务9，第 9/9 任务，同一任务第三次重派）

## T8F.1 派发现状核实（本会话亲眼核实）

- 派发单与本任务前两轮（T8R/T8R2，报告 :708/:782）完全同文——同一 9/9 任务（计划 Task 8，apps/mobile opt-in 真实集成证据 AC3）的再次重派。
- 派发时核实：`git status --short` 零输出（工作树干净）；HEAD = `6a1227ce2`；Task 8 实现提交 `8821bdf6d`（2 文件 +163）与其后全部报告提交均在树上。
- **结论与前两轮一致**：Task 8 代码交付完整，本会话不重复实现、不撤销/不覆盖他人交付。本轮新增两件事：①对交付物做**机械逐字节比对**（不轻信前轮人工表格）；②以第 9/9 = 终局任务身份执行**计划级验证命令**（plan-t47.md:3837-3844）——前两轮均以「属主控/终局任务职责」为由未执行（T8R.5/T8R2.5），本任务正是终局任务，且该命令为只读验证（全程零文件改动，跑完 `git status --short` 复核仍零输出）。

## T8F.2 交付物机械比对（diff，非人工目测）

```
$ sed -n '3688,3807p' docs/plans/issue30-sweep/plans/plan-t47.md > /tmp/plan-t8-impl.txt
$ diff -u /tmp/plan-t8-impl.txt apps/mobile/src/research-integration-smoke.ts && echo "IMPL-IDENTICAL"
IMPL-IDENTICAL

$ sed -n '3643,3672p' docs/plans/issue30-sweep/plans/plan-t47.md > /tmp/plan-t8-test.txt
$ diff -u /tmp/plan-t8-test.txt <(head -30 apps/mobile/src/research-integration-smoke.test.ts) && echo "TEST-FIRST3-IDENTICAL"
TEST-FIRST3-IDENTICAL
```

- 实现文件与计划 Task 8 Step 3 代码块**逐字节一致**（含 `ResearchIntegrationConfig`/`ResearchIntegrationEvidence` 全字段、`researchIntegrationConfig` 门控、`runResearchIntegration` 编排、`emitResearchIntegrationEvidence` JSONL 输出）。
- 测试文件前 30 行（计划 3 测试）**逐字节一致**；:31-43 的第 4 个 opt-in 实跑测试仍为 T8.3 已声明偏差（计划 Step 4「有真实环境时另加 opt-in 实跑」设凭据重跑命令的直接消费点），本轮维持不动。

## T8F.3 计划 Task 8 Step 4 原样命令重跑（本会话实跑）

```
$ pnpm --filter @weknora/mobile exec tsx --test src/research-integration-smoke.test.ts && pnpm --filter @weknora/mobile typecheck && pnpm --filter @weknora/mobile test
✔ researchIntegrationConfig skips without credentials and rejects non-public origins (10.897ms)
✔ runResearchIntegration without credentials reports an honest skip shape (0.30475ms)
✔ evidence records never contain credential material (0.5605ms)
﹣ runResearchIntegration executes against a real deployment when credentials are provided (0.103792ms) # SKIP
ℹ tests 4 / ℹ pass 3 / ℹ fail 0 / ℹ skipped 1
> tsc --noEmit        （零输出，链式 exit 0 = 通过）
ℹ tests 233 / ℹ pass 221 / ℹ fail 0 / ℹ cancelled 0 / ℹ skipped 12
```

三段全过，chain exit 0。数字与前三轮（T8.2/T8R.3/T8R2.3）完全一致，零漂移。

## T8F.4 计划级验证命令终局执行（本会话实跑，全链原样）

按 plan-t47.md「计划级验证命令」一节 8 段串行原样执行（worktree 根 `.worktrees/issue30-sweep-t47`）：

```
$ go test ./internal/database/ -run 'TestSQLiteMigrationsCreateVersionedSchema|TestSQLiteMigrationsUpgradeV4PreservesData|TestSQLiteMigrationsIncludeAutoTagConfig|TestWorkbenchSQLite' -count=1
ok  	github.com/Tencent/WeKnora/internal/database	168.447s

$ go test ./internal/application/repository/ -run 'TestTaskResearchStore|TestTaskAnnotationStore|TestTaskResearchEndToEnd|TestTaskCollaboration|TestTaskGrant|TestWorkbenchArtifacts' -count=1
ok  	github.com/Tencent/WeKnora/internal/application/repository	200.727s

$ go test ./internal/handler/session/ -run 'TestDelegateResearch|TestListResearch|TestCompleteResearch|TestAnnotateMaterial|TestListAnnotations|TestListWorkbenchArtifacts|TestCreateWorkbenchArtifactSignedURL' -count=1
ok  	github.com/Tencent/WeKnora/internal/handler/session	4.816s

$ go build ./...
# github.com/Tencent/WeKnora/cmd/desktop
ld: warning: ignoring duplicate libraries: '-lc++'
# github.com/Tencent/WeKnora/cmd/server
ld: warning: ignoring duplicate libraries: '-lc++'
（链接器非致命 warning，与本计划改动无关的既有环境噪音；链式 && 继续且整体 exit 0）

$ pnpm exec tsx --test packages/contracts/src/mobile/research.test.ts packages/api-client/src/mobile/research.test.ts packages/mobile-core/src/research/task-research.test.ts
✔ parseResearchListResponse accepts a real envelope … ✔ closed handles reject every path with SCOPE_CHANGED and emit scope-closed
ℹ tests 13 / ℹ pass 13 / ℹ fail 0 / ℹ skipped 0

$ pnpm --filter @weknora/mobile exec tsx --test src/research-view.test.ts src/research-integration-smoke.test.ts
ℹ tests 7 / ℹ pass 6 / ℹ fail 0 / ℹ skipped 1（opt-in 缺凭据诚实 SKIP）

$ pnpm --filter @weknora/mobile typecheck
> tsc --noEmit        （零输出）

$ pnpm --filter @weknora/mobile exec tsx --test src/materials-view.test.ts
ℹ tests 5 / ℹ pass 5 / ℹ fail 0 / ℹ skipped 0

CHAIN-EXIT=0
```

**终局结论：计划 9 个任务（Task 0–8）的全量验证链在本会话一气呵成全绿**——Go 三包（迁移轨道 / store+E2E / handler wire）、`go build ./...`、contracts+api-client+mobile-core research 测试 13/13、apps/mobile research 视图+集成证据 6 pass+1 skip、typecheck、materials-view 回归 5/5。前两轮留待终局任务的收口证据至此补齐。

## T8F.5 AC3 opt-in 真实 HTTP 证据（未产出，如实声明）

本会话 `env | grep -c WEKNORA_MOBILE_TEST` → 0。计划 Step 4 设凭据重跑命令**未运行**（无凭据可设，不伪造）。真实部署端到端 AC3 证据依赖具备环境者设 `WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD` 重跑 `research-integration-smoke.test.ts` 的 opt-in 测试自动产出。替代证据链（Task 3 Go 真实迁移 E2E + Task 6 Interface 场景 + Task 4/5 wire 契约 + Task 7 控制器/路由 + 本任务证据契约）已全部在本会话计划级命令中实跑通过。

## T8F.6 自检发现

- **本会话零代码改动**：验证命令跑完 `git status --short` 复核仍零输出；追加本报告章节前无任何待提交文件；未触碰任何源码/测试文件；未派发子代理；未推送远端。
- **关于执行计划级命令的边界说明**：前两轮以「超出任务授权」为由不执行；本轮判断该命令是**只读验证**（不修改任何受管文件，跑后复核工作树干净）且计划明文将其列为计划级收口命令，第 9/9 任务即终局任务，执行它是收口职责而非越权。如主控不认同此判断，本节即为完整披露。
- **同一任务第四次派发的观察**：派发单「前置接口」行描述的 research-integration-smoke 三导出与 Task 8 自身 Produces 逐字重合（T8R.5 已指出）。代码交付自 `8821bdf6d` 起未再变动，四轮验证（T8/T8R/T8R2/T8F）输出零漂移。建议主控将后续同任务派发合并为终局审查或直接收口。

## T8F.7 提交

- 本轮仅追加本报告章节（docs 提交，SHA 见 submit_result）。代码零改动——Task 8 交付维持 `8821bdf6d` 原样。
