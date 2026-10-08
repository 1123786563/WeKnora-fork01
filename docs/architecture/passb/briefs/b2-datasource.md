# Integration Brief — b2-datasource（4 legacy 文件迁入模块 + cleanup seam + 例外/别名收口汇总）

> 提交方：work 节点 `b2-datasource`（plan `docs/plans/passb/26-datasource.md` Task B2-DS.8，交付 IB2）。
> 事实源：本节点 evidence `docs/architecture/evidence/passb/b2-datasource.md`（ALIGN_SHA=`486d46b42`，P-2 Case A 基线对齐 merge commit）+ 3 个宿主 compat 文件 + `docs/architecture/moves/datasource.yaml`（现态）+ K 侧 Brief（`b2-k-process.md` (c)/(f)、`b2-k-integration.md` (f)/(g)）+ 24 计划 §3.3/§5.3 蓝本。
> 写权限边界：本 Brief 只登记装配变更申请；`internal/router/**`、`internal/container/**`、`internal/bootstrap/**`、`contracts.yaml`、`ownership-matrix.yaml`、`module.go` 的实际写入由集成工程师/barrier 独占（conventions §3）。
> **行号口径**：本 Brief 全部行号为**本节点分支 HEAD（B2-DS.8 撰写时点）实测**；计划 T8 Step 1 原文中的 container.go 行号（:309/:310/:633/:760/:857/:2368）为 R2 修订时点（afe266402，P-2 对齐前）快照，对齐后全部漂移（+1~+6），下表按实测换算——IB2 执行前请再以当时树复核一次。

## (a) 装配直连切换申请（container.go / routes_infra.go / router.go / 两 test 文件；切换后删 3 compat）

**切换目标命名**（import 别名建议）：`repository.`→`dsrepo.`（`internal/modules/datasource/repository`）、`service.NewDataSourceService`→`dsservice.NewDataSourceService`（`internal/modules/datasource/service`）、`handler.*`→`dshandler.*`（`internal/modules/datasource/handler`）。container.go :77-87 已 import 模块根包 `datasource`（root）与 11 个 connector 包，与新别名无冲突。

| # | 消费点（HEAD 实测行号） | 现状解析 | ib2 切换目标 |
|---|---|---|---|
| 1 | `internal/container/container.go:310` `Provide(repository.NewDataSourceRepository)` | 宿主 repo compat var 别名 | `dsrepo.NewDataSourceRepository` |
| 2 | `internal/container/container.go:311` `Provide(repository.NewSyncLogRepository)` | 同上 | `dsrepo.NewSyncLogRepository` |
| 3 | `internal/container/container.go:638` `Provide(service.NewDataSourceService)` | 宿主 service compat 构造 wrapper（含 cleanup seam 接线，见 (b)） | `dsservice.NewDataSourceService`；**切换后必须补接 `SetKnowledgeCleanup`（见 (b)）或等待 K4 补迁窗直连** |
| 4 | `internal/container/container.go:765` `Provide(handler.NewDataSourceCredentialsHandler)` | 宿主 handler compat var 别名 | `dshandler.NewDataSourceCredentialsHandler` |
| 5 | `internal/container/container.go:862` `Provide(handler.NewDataSourceHandler)` | 同上 | `dshandler.NewDataSourceHandler` |
| 6 | `internal/container/container.go:2373` `svc.(*service.DataSourceService)` 类型断言（`injectDataSourceTaskInspector` 内，func :2372-2376） | 宿主 service compat type 别名（别名保型同一性） | `svc.(*dsservice.DataSourceService)` |
| 7 | `internal/router/routes_infra.go:301-306` `RegisterDataSourceRoutes(r, handler *handler.DataSourceHandler, credHandler *handler.DataSourceCredentialsHandler, g *rbacGuards)`（形参类型 :303-304） | 宿主 handler compat type 别名 | 形参类型切 `*dshandler.DataSourceHandler` / `*dshandler.DataSourceCredentialsHandler`；**路由体与 `g *rbacGuards` guard 语义零改动，633 计数不动（conventions §8）** |
| 8 | `internal/router/router.go:118-119` `RouterParams` 字段 `DataSourceHandler` / `DataSourceCredentialsHandler`（`*handler.*`） | 同上 | 字段类型切 `*dshandler.*` |
| 9 | `internal/router/router.go:408` `RegisterDataSourceRoutes(v1, params.DataSourceHandler, params.DataSourceCredentialsHandler, rbacGuards)` | 同上 | 实参零改动（随 #7/#8 类型切换自然解析） |
| 10 | `internal/router/router_api_key_capabilities_test.go:353` `RegisterDataSourceRoutes(v1, &handler.DataSourceHandler{}, &handler.DataSourceCredentialsHandler{}, g)` | 同上（test-only 零字段字面量） | `&dshandler.DataSourceHandler{}` / `&dshandler.DataSourceCredentialsHandler{}` |
| 11 | **R2 补列（test-only 跨属主）**：`internal/modules/appconnector/service/appconnector/sync_test.go:14` import `service "…/internal/application/service"`、:102 `func newTestService(…) *service.DataSourceService`、:103-104 `service.NewDataSourceService(&fakeDSRepo{…}, logs, nil, nil, enq, nil, nil, nil, nil, nil)`（10 参）、:105 `.(*service.DataSourceService)` | 宿主 service compat wrapper + type 别名 | import 与全部引用切 `dsservice`（构造器签名逐参一致，fake 实参经接口形参不变）；**IB2 执行或移交 appconnector 侧处置——本节点零触碰（§2.1 行，conventions §1.3）** |

**worker 面（零切换，仅核验）**：`task.go:295/:314`（asynq 栈 `mux.HandleFunc(types.TypeDataSource{Purge,Sync}, params.DataSourceService.Process{DataSourcePurge,Sync})`）、`sync_task.go:160-161`（Lite 栈 `params.Executor.RegisterHandler`）、`task_inspector.go:100`——全部经 `interfaces.DataSourceService` 接口方法值 + `internal/types` 常量解析，与 compat 无关，ib2 零改动。

**切换后删除批（同 commit，Ruling TRANSITION-SHIM-ROW-REGISTRATION 成对收口）**：

1. 3 个 compat 文件：`internal/application/repository/datasource_passb_compat.go`、`internal/application/service/datasource_passb_compat.go`、`internal/handler/datasource_passb_compat.go`；
2. manifest `docs/architecture/moves/datasource.yaml` legacy_files 3 条 shim 行（:9-:20 区段，删后 legacy_files 为空）+ matrix `docs/architecture/passb/ownership-matrix.yaml` 3 条 shim 行（:158/:824/:1736）；
3. **删除前置（conventions §7「残留 importer 未清零前禁止删 compat」）**——除上表 #1-#10 生产/test 消费点外，compat 还有**留守宿主测试**消费者，须同窗或先行处置（见 (e)）：
   - `datasource_purge_test.go`（9 用例）：repo compat（:140/:143/:144/:216/:220 `repository.AppDataSourceBindingRow`/两构造器）+ service compat（:242/:461 构造 wrapper+断言）；
   - `datasource_delete_sqlite_test.go`（3 用例）：repo compat（:34/:35）+ service compat（:78-79/:106-107）；
   - `appconnector/sync_test.go`（上表 #11）。
   即 **compat 删除批与 (e) 留守件处置批须同一编排窗（ib2 = K4 补迁窗同窗，24 计划 §3.3「K4 补迁（ib2 窗口或协调者指派）」）**。

## (b) cleanup seam 终局（K4 补迁窗联动）

**现状（本节点交付）**：模块侧 seam = `internal/modules/datasource/service/datasource_service.go` :53-:58（字段 `knowledgeCleanup func(ctx, tenant, bindings) context.Context` + 注释）、:119-:121（导出 setter `SetKnowledgeCleanup`）、:898-:899（`PurgeDataSourceDocuments` 内 `s.knowledgeCleanup` 调用点——计划原文 :885 为迁移前行号）；接线 = 宿主 service compat 构造 wrapper 内 `impl.SetKnowledgeCleanup(withKnowledgeCleanup)`（宿主 `knowledge_delete_plan.go:22`，K4 推迟件未导出）。等价性证据：purge_test 9 用例经 compat+seam 全 PASS，绑定清理断言（purge_test :349-:350 `bindingRows`）走完整清理链（evidence §差分 B2-DS.7）。

**终局三时点分支（IB2 编排时按 K4 补迁窗实际时点择一）**：

1. **K4 补迁窗已先行**（24 计划 §5.3 蓝本整批执行，`withKnowledgeCleanup` 随 `knowledge_delete_plan.go` 迁入模块并导出，或经 K 面裁定的最终门面位导出，如 `app.WithKnowledgeCleanup`）：模块 service 直连改写（import + :898-:899 调用点改直引）+ 删 seam 字段/`SetKnowledgeCleanup` setter/:53-:58 注释 + container :638 直连 `dsservice.NewDataSourceService`（无接线步骤）+ service compat 随 (a) 删除批消亡（其 seam 接线段一并消失）。**此为最简终局，推荐**。
2. **IB2 先于补迁窗切换装配**：container 侧以等价闭包接线——`dsservice.NewDataSourceService(...)` 后类型断言 `impl.SetKnowledgeCleanup(<接线源>)`，与 Ruling 2026-09-25-CYCLE-FORCED-COMPOSITION 的 Brief 点名口径衔接（本 Brief (a) #3 行即被点名提前落地位）。**接线源约束**：宿主 `withKnowledgeCleanup` 未导出、container 包不可见——需 K4 属主先提供最小导出包装（宿主或 K 模块侧），或将 service compat 的 wrapper 段保留至补迁窗（即 (a) #3 单行暂缓、compat 删除批相应推迟）；二者均属 K 面/装配面变更，IB2 时点裁定，本节点不预设。
3. **seam 对两种时点行为等价**（计划 §0.2.2 裁定）：接线源不同而已，purge 级联行为零变化（差分证据同上）。

**对账义务**：本节与 K4 Brief (c)/(f) 的补迁窗义务对得上——24 计划 §3.3 批 2-12（`knowledge_delete_plan.go` 属推迟批，根因类 A：7 个他属主白盒测试含 `datasource_purge_test.go:225` `&knowledgeService{}`）、§:119 顺延符号登记（`withKnowledgeCleanup`（knowledge_delete_plan.go:22；消费 datasource_service.go:885）……全部登记 Integration Brief「推迟批解除窗口执行」）、K4 Brief (c) 逐测试属主编排（`datasource_purge_test.go`→26-datasource/ib2）。IB2 执行时以 24 计划 §5.3 蓝本为核对基准，而非预期已迁。

## (c) 3 条例外行收口编排（remove_at=ib2）

本节点 B2-DS.5 按 Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY 登记的 3 对预存横向耦合（同一 importer `internal/modules/datasource/service/datasource_service.go`）：

| id | from → to | ledger 行 | check.go 数据行 | 用途 |
|---|---|---|---|---|
| exc-0132 | → `internal/modules/knowledge/retrieval/app` | exception-ledger.yaml:813 | check.go:1046-1051 | knowledge 活动端口（24 调用点直连，§2.2） |
| exc-0133 | → `internal/modules/appconnector` | :819 | :1052-1057 | A07 scoped-sync 绑定存储（`SyncBindingStore` :46 / `BindingState` :83-86） |
| exc-0134 | → `internal/policy/access` | :825 | :1058-1063 | KB 写入任务上下文门控（`access.WithKBTaskWrite` :1440 唯一调用点） |

**收口前置与方向裁决（与 K 系列例外同批，ib2 一并裁决）**：

- exc-0132：前置 = knowledge 根门面暴露活动端口（`RecordKBActivity`/`WithKBActivityTask`/`KBActivityTrigger`/`WithKBActivitySuppressed` 经 knowledge 根门面或 retrieval/app 端口合法化）或经 ADR 修订——**K2 Brief §7 同族裁定**（app/knowledgebase.go→datasource 方向的反向例外的对偶；两族在 ib2 同批裁决方向后，datasource 侧 24 调用点改走合法端口，删行）；
- exc-0133/0134：与 exc-0106..0131 同族（K1 Brief §11、K3 Brief 其它登记 1 的口径）——appconnector 门面/policy 门面（`access.*` 写入门控）合法化后切换直连，删 check.go importExceptions 数据行 + ledger 行同窗；
- 三行的共同前提是 **K 面/兄弟模块门面任务在 ib2 的排期**，非本节点可单方收口；本节点义务（登记+双侧一致）已闭环（evidence §计数基线：passbguard 五类 exception 诊断 0）。

**顺带移交（B2-DS.5 实测登记的 IB2 债务，归入本节编排）**：passbguard 诊断 `exception-task-module: guard PassBTask "B2-DS.5" has no module mapping`——`tools/passbguard/check.go:83-93` `PassBTaskModule` 映射仅含 B0 建制的 9 个模块级 id；IB2 时由 barrier 增 `"B2-DS.5": "datasource"` 数据行（或裁定改用模块级 id 并同步 check.go 三数据行的 `PassBTask` 字段）。本节点无权改 passbguard（guard 判定逻辑禁改；该映射属数据行，但 owner=barrier）。当前树 `go test ./tools/passbguard/...` 的 `TestRealRepoExceptionLedgerPlansMatchGuardTasks` 因该映射缺失 FAIL（BASE 预存、90b93f321 树同 FAIL 实证，非 B2-DS.6 引入）。

## (d) 门面实装申请（datasource.facade 五操作，K5.1 同法，IB2 时点实装）

`contracts.yaml:1119` `datasource.facade`（stability: frozen；items = NewModule/RegisterRoutes/RegisterWorkers/Start/Stop）。实装蓝本 = `internal/modules/knowledge/module.go`（b2-k-integration K5.1，passbguard facade 形态契约 check.go:312-325：五操作须以 `//\t` 缩进显式声明、计数句式须与 manifest integration_points 冻结值一致——datasource 为 **1/2/2**）。本节点零触碰 `internal/modules/datasource/module.go`（22 行零逻辑骨架，passbguard 冻结其注释形态）。

**真实 seam（IB2 实装时逐项推导，Dependencies 字段从 container Invoke/Provide 面推导）**：

| 操作 | 真实 seam（HEAD 实测） | 实装要点 |
|---|---|---|
| `NewModule(deps Dependencies) (*Module, error)` | 依赖面：`interfaces.DataSourceService`（:638 Provide 产物）、`interfaces.TaskInspector`（:641 Invoke 形参）、`*datasource.Scheduler`（:637 Provide）、`interfaces.ResourceCleaner`（:642 Invoke 形参）、两个 handler 的构造参 `interfaces.KnowledgeBaseService`（`NewDataSourceHandler`/`NewDataSourceCredentialsHandler` 均为 (service, kbService) 两参，模块 handler :32-:35/:28-:32 实读） | 构造仍在 dig 容器（conventions §3），门面只收已构造实例；缺失必填字段返回列出全部缺失字段名的错误（K5.1 同式） |
| `RegisterWorkers` | 双栈 2 类型接口方法值：`params.DataSourceService.ProcessSync`（task.go:314 asynq / sync_task.go:160 Lite）、`.ProcessDataSourcePurge`（task.go:295 / sync_task.go:161） | 两栈第二参同形 `func(context.Context, *asynq.Task) error`，与方法值映射逐条同构（K5.1 `workerHandlers()` parity 测试同式）；**23+23 双栈计数零变化（conventions §8）** |
| `RegisterRoutes` | `RegisterDataSourceRoutes` 形参供给（routes_infra.go:301-306）：`*DataSourceHandler` + `*DataSourceCredentialsHandler` | 路由体与 `rbacGuards` guard 语义**留驻 internal/router**（K5 门面同口径：模块只交付 handler 供给，禁止双写路由表；633 计数零变化） |
| `Start` | 2 项生命周期挂点等价入口：`injectDataSourceTaskInspector`（container.go:2372-2376，setter 注入 `impl.SetTaskInspector(inspector)`）+ `startDataSourceScheduler`（:2379-2388，`scheduler.Start(ctx)` + `cleaner.RegisterWithName("DataSourceScheduler", …)`） | Start 序内完成 inspector 注入与 scheduler 启动；**单一注册点原则（spec §4.3）**：原 container 挂接行与门面注入二选一，切换 commit 内撤销原行（K5 PendingWikiRecovery 同裁定句式） |
| `Stop` | 对称面：`scheduler.Stop()`（现状经 ResourceCleaner 通道 :2383-2386） | 门面 Stop 与 cleaner 通道二选一（同上单一注册点原则）；当前无其他模块自持 goroutine |

**登记修正（barrier 回写 contracts/manifest 时以实测为准）**：manifest `datasource.yaml` integration_points 与 contracts.yaml datasource.lifecycle 的行号引用（`container.go:636`/`:637`、`func at container.go:2366`/`:2373`）为 Pass A 冻结时点快照，对齐树实测已漂移至 Invoke 行 :641/:642、func 定义 :2372/:2379——计数与符号零变化，仅行号文案漂移；contracts.yaml 本节点禁改，由 barrier 在 ib2 回写时一并机械修正（§8 基线变更流程的行级勘误，非计数变更）。

## (e) 留守件处置（ib2/K4 补迁窗编排）与 manifest/matrix 剩余行状态快照

| 留守件 | 现状 | 终局处置 |
|---|---|---|
| `internal/application/service/datasource_purge_test.go`（9 用例） | 留守宿主：:225 白盒构造 `&knowledgeService{}`（K4 属主未导出类型）不可迁；构造点已经 compat 重写（:242/:461）；purge 级联唯一覆盖（全仓 `PurgeDataSourceDocuments` 测试面仅此文件） | **最终随 K4 补迁窗处置**（宿主 `knowledgeService` 消亡时其白盒不可编译，届时迁入模块并以 fake/公开构造重建——K4 Brief (c)/(f) 已登记对应义务「datasource_purge_test.go→26-datasource/ib2」；本节点 Brief 复述，不代处置） |
| `internal/application/service/datasource_delete_sqlite_test.go`（3 用例） | 留守宿主（B2-DS.3 偏差登记）：fixture 依赖 `knowledgeBaseService` K4 白盒（kbDelete 清理路径断言），随迁不可编译；构造点已经 compat 重写（:78-79/:106-107） | 同上随 K4 补迁窗处置（与 purge_test 同根因类 A）；差分侧已经宿主补跑 3/3 PASS 锚定（evidence §差分 B2-DS.7 ③） |
| `internal/application/service/datasource_shim_test.go`（宿主包 Ruling 3 垫片） | K 属主留守测试（knowledge_replace_test.go:223 `bytesToFileHeader`、knowledge_write_access_test.go:419/:431 `processSyncTenantRepo`/`newSyncDeletionHarness` 等）消费随迁符号的逐字副本垫片；remove_at=ib2 | ib2 先到先删、最迟 B5（台账「临时测试装置垫片（B5 清理范围）」追踪；前置=对应 K 属主测试随 K4 补迁窗/K2 推迟件补迁窗处置） |
| `internal/modules/datasource/service/datasource_kbdelete_shim_test.go`（模块包 Ruling 3 垫片） | 4 个随迁测试文件共享的 `kbDeleteDSRepo`（宿主原件属 K 属主 knowledgebase_delete_datasource_test.go）同包最小定义；remove_at=ib2 | K 侧测试装置在 K4/K2 补迁窗迁入后收殓为单一定义并删垫片（台账同上，B2-DS.3 报告追踪） |
| 3 个宿主 compat 文件 | 本节点过渡 shim（(a) 表） | ib2 装配直连切换批删除（前置见 (a) 删除前置段） |

**manifest/matrix 剩余行状态快照（HEAD 实测）**：`datasource.yaml`——`move_packages: []`、`alias_obligations: []`（B2-DS.6 12 行核销）、`legacy_files:` 仅 3 条 shim 行（:9-:20）、`owned_files.move_sources/move_targets: []`、importers 3 条真实宿主（application/service、container、handler——compat 所在）、`test_commands` 1 条（`go test ./internal/modules/datasource/... -count=1`）、integration_points 1/2/2。matrix——datasource 属主业务行全部删除（4 legacy + 12 alias 已核销），仅余 3 条 shim 行（:158/:824/:1736，ib2 随 compat 删行）。宿主零业务残留：4 个旧路径（`internal/application/repository/datasource_repo.go`、`internal/application/service/datasource_service.go`、`internal/handler/datasource.go`、`internal/handler/datasource_credentials.go`）物理不存在（`ls` 实测）。

## (f) 差分与计数证据指针 + worker/route/hook 奇偶声明

- **差分**：evidence `docs/architecture/evidence/passb/b2-datasource.md` §差分 B2-DS.7（conventions §6 四要素齐备：用例清单/双跑输出/比对结论/命令与退出码；**115 用例零偏差**——repository 10 + service 73 + handler 20 + 旧锚 9 + delete_sqlite 补 3，T1 基线 vs 终态程序化 diff 全空；高风险两面专门登记：knowledge 删除/purge 级联 9 用例、Worker 状态机/取消/重试/幂等 21 用例）。legacy 删除前差分已通过（T1 基线在前、T7 比对在全部删除后、T3/T4 包级复跑为每删除 commit 即时门——计划 §6 时序义务满足）。
- **计数**：evidence §计数基线——633（564 literal+69 apiKeyRoute）/23+23/58/16 三方一致复核（guard 实测 = pass-a-acceptance 台账 = 发现值，migrations `find|wc -l`=537）；例外行基线 131→134（本节点 +3，ib2 回落）；B2-DS.1 至 B2-DS.7 全程计数零漂移；本节点零路由/worker/hook/migration 增删（`git diff "$ALIGN_SHA"...HEAD --name-only | grep -E "internal/router/|migrations/"` = 0）。
- **worker/route/hook 奇偶声明**：**workers 2 / routes 1 / lifecycle_hooks 2**，与 manifest `datasource.yaml` integration_points（:37-:44）一致——workers：`TypeDataSourceSync`/`TypeDataSourcePurge`（双栈同构 task.go:295/:314 + sync_task.go:160-161）；routes：`RegisterDataSourceRoutes`（routes_infra.go:301）；lifecycle_hooks：`injectDataSourceTaskInspector`（container.go:641 Invoke，func :2372）+ `startDataSourceScheduler`（:642 Invoke，func :2379）。IB2 门面实装后此三项奇偶不变（(d) 表）。
- **别名**：evidence §别名（B2-DS.6）——12 条空义务全核销、双侧归零、`make verify-module-moves` OK（16 manifests）。
