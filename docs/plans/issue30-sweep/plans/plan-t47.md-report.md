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
