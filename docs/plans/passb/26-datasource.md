# Pass B 子计划 26 — B2 Data Source（4 legacy 文件：service/repo/2×handler 迁入模块 + kbActivity 直连改写 + cleanup seam + 别名/例外收口）

> 实施方式：superpowers:executing-plans / subagent-driven-development，按任务逐个执行，一个任务一个 commit（conventions §4）。
> 节点：`b2-datasource`（DAG `docs/plans/passb/execution-dag.json`，phase=B2，role=work，execution_mode=serial，depends_on=`[ib1, b2-k-integration]`）。
> 本计划由「计划撰写-b2-datasource」于 2026-09-27 在 worktree `codex/passb-b2-datasource`（分支 HEAD `74f454527`，**K 生产分支尚未合入的集成谱系**）撰写；文中全部代码坐标、符号签名、命令输出均为该树实读/实跑结果。**实施前必须完成 §0.1 P-2 基线对齐 merge（Ruling 2026-09-24-WAVE-DEP-BASELINE），对齐后行号按当时树复核**（本节点 4 个 legacy 文件不在任何 K 节点 owned_files 内，其行号预期稳定；5 符号定义方位置随 K 合并变化，已在 §2.2 标注对齐后复验点）。
> **R1 修订**（2026-09-27，「计划修订-b2-datasource-R1」按审校 findings 5 项在本 worktree 同分支完成，修订时分支 HEAD `b76d0fb8b`）：① 补全 repository 侧 2 个关联测试文件随迁（`datasource_repo_test.go` 6 用例 + `datasource_repo_synclog_test.go` 4 用例，framework:29），基线口径 105→**115** 用例；② 全部特征化/差分 `-run` 模式重写并本分支实跑验证（service 85 / repo 10 / handler 20 / purge 单独 9 顶层用例）；③ §2.1 消费面全集补 router 三处（router.go:118-119/:408、router_api_key_capabilities_test.go:353）；④ §2.3 无环论证更正——K 分支无 `app/knowledgebase.go`、modules 内零 datasource 反向 import 边；⑤ 行号漂移修正（DataSourceService struct :30、import :18/:21、syncBindingStore :46、dataSourceBindingCleaner :823、module.go 22 行、routes_infra 签名 :301-306、purge_test 引用行）。

## 0. Spec 与事实源指针

| 事实源 | 位置 | 本计划取用的内容 |
|---|---|---|
| Spec | `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md` §5.9（Data Source 职责：外部知识源、连接器注册、同步、取消、重试、进度、凭据元数据、调度；**抓取后调用 Knowledge 摄取接口，不直接写 Knowledge 表**）、§5.18 覆盖矩阵（datasource service/connector registry/sync scheduler→Data Source 行）、§6.2（Data Source→Knowledge 数据流：双方不直接更新对方表）、§11（B2 并行 + IB2 串行）、§12（Pass B 循环：特征测试→domain/ports/adapters→高风险差分→装配切换→legacy 删除）、§13（M1–M5 提交隔离与回滚）、§14.3（高风险差分：Worker 状态机/重试/幂等 + Knowledge 删除）、§17.2 完成标准 | 边界语义、差分义务、提交纪律 |
| Pass B 框架 | `docs/plans/2026-09-23-backend-modularization-pass-b-framework.md` :146（26 号计划=4 legacy 文件、sync scheduler/worker 门面、别名删除）、:149（与 Knowledge/AgentCatalog 的并行依据与现 DAG 串行裁定，§0.2）、:25-33（全局约束：manifest 属主、模块 worker 不改 router/container、`_test.go` 随迁 :29、无双写/双注册 :30、高风险差分 :31）、:153-155（IB2 职责） | 任务边界、随迁与差分纪律 |
| 执行公约 | `.superpowers/sdd/passb/conventions.md`（主 checkout，git-ignored） | §1 派发契约（不派生子 Agent、只改 owned_files、跨 owner 调用点不直接改对方文件）、§1.1 报告路径、§1.2 报告命令、§2 门禁、§3 禁改清单、§4 提交规范、§5 升级、§6 差分四要素、§7 package-private 耦合处理、§8 计数基线、§10 裁定族（Ruling LEGACY-ROW-OWNERSHIP / IMPORT-EXCEPTION-REGISTRY / WAVE-DEP-BASELINE / TRANSITION-SHIM-ROW-REGISTRATION） |
| MOVE-MANIFEST | `docs/architecture/moves/datasource.yaml` | legacy_files 4 条（:55-:70，passb_task=B-datasource）、alias_obligations 12 条（:30-:53）、integration_points（routes 1：RegisterDataSourceRoutes—routes_infra.go:301；workers 2：TypeDataSourceSync/TypeDataSourcePurge；lifecycle_hooks 2：injectDataSourceTaskInspector container.go:636、startDataSourceScheduler :637）、owned_files、test_commands（:130-131 `go test ./internal/modules/datasource/... -count=1`） |
| ownership-matrix | `docs/architecture/passb/ownership-matrix.yaml`（本分支副本；B0 冻结，集成 worktree 同源）:164-168（datasource_repo.go→`internal/modules/datasource/repository`）、:842-846（datasource_service.go→`internal/modules/datasource/service`）、:1808-1812（handler/datasource.go→`internal/modules/datasource/handler`）、:1814-1818（handler/datasource_credentials.go→同上；四行均 `plan: 26-datasource, integration_owner: ib2, delete_barrier: ib2`）、:2547-2583（aliases 区 12 行 `plan: 26-datasource, delete_barrier: ib2`） | 精确 destination、行级删除权（Ruling LEGACY-ROW-OWNERSHIP）、别名删除义务 |
| contracts.yaml | 同目录 `contracts.yaml` :1119-1169（`datasource.facade` :1119 items=NewModule/RegisterRoutes/RegisterWorkers/Start/Stop、`datasource.lifecycle` :1133 items=injectDataSourceTaskInspector/startDataSourceScheduler、`datasource.routes` :1145 items=RegisterDataSourceRoutes、`datasource.workers` :1156 items=TypeDataSourceSync/TypeDataSourcePurge；四契约均 `stability: frozen`，本节点只读）。消费面登记：`identity.audit-log-service` consumers 含 datasource_service.go（:1279）；`knowledge.knowledge-base-service`（:1636/:1654/:1684-1685）、`knowledge.service`（:1781/:1826）、`knowledge.tag-service`（:1850/:1857）；`workbench.task-enqueuer` consumers 含 datasource_service.go + `internal/modules/datasource/scheduler.go`（:2092/:2108）；`workbench.task-inspector` consumers 含 datasource_service.go、characterization 含 datasource_cancel_enqueue_test.go（:2123/:2131） | 冻结契约（本节点不改四契约面）、依赖端口的合规依据、随迁测试的契约登记定位 |
| event-catalog | 同目录 `event-catalog.yaml` | `grep -i datasource` 零命中——本模块无版本化事件，本节点零事件面改动 |
| exception-ledger | 同目录 `exception-ledger.yaml` | 本分支 105 行（exc-0001..0105，`grep -c "id: exc-"` 实测）中 `plan: 26-datasource` 零行——本节点无属主例外行，**B2-DS.5 新增 3 行**（§2.6）；行格式见 :602-607 |
| DAG 裁定 | `execution-dag.json` 节点 `b2-datasource`：owned_files（manifest legacy_files 4 条 + `internal/modules/datasource/**`；**F7 修正：Pass A 已完成 12 个 move_packages 搬迁、internal/datasource 目录已不存在，本节点仅经手 4 条 legacy 文件与别名删除，不含目录级再搬迁**）、shared_resources（service 宿主包对 kb_activity.go 未导出函数的调用）、required_contracts（knowledge 活动端口、Data Source sync scheduler/worker 契约）、gates（`go build ./...` / `go test ./internal/modules/datasource/...` / `make check-backend-architecture` / `make verify-module-moves`）、produced_artifacts、notes（CORR-2 + BLOCKED 链，§0.2） | 节点职责、门禁 argv、5 符号 24 调用点裁定 |
| Knowledge 程序冻结产物 | `docs/plans/passb/20-knowledge-program.md`（b2-k-integration worktree 副本；K0 §6.2 组 B/组 C 联动裁定表、K5.1–K5.3 任务）、`22-knowledge-retrieval.md` §2 导出面表（:76-:80）、`24-knowledge-process.md` §3.3 推迟件（knowledge_delete_plan.go 属推迟批 2-12；§:119 顺延符号 withKnowledgeCleanup） | 4+1 符号的导出名、签名、`withKnowledgeCleanup` 推迟未导出的事实（§2.2 裁定前提） |
| 先例计划 | `docs/plans/passb/27-appconnector.md`（§2.2 forbidden-import 判定域实读、§2.4 别名空义务核销、shim 最小化先例）、`13-execution.md` §2（module.go 门面零实现=集成节点职责先例）、`24-knowledge-process.md`（kbprocess_passb_compat 成对补行先例，§0.4 引用实测） | 格式与裁定先例 |

### 0.1 前置条件（开工前逐条核验；任一不满足按 conventions §5 回 BLOCKED）

1. **P-1 前置节点终态**：DAG `b2-k-integration` `status=done, review_status=approved` 且 `head_sha` 已回填；`ib1` `status=done`。以开工时点 DAG 实测为准（撰写时点该节点仍 blocked，notes 残留 BLOCKED 文本为历史登记，不构成停工依据——27 计划 §0 同口径）。
2. **P-2 基线对齐 merge**（Ruling 2026-09-24-WAVE-DEP-BASELINE，K5 P-K5-2 同型）：先判 `git merge-base --is-ancestor codex/passb-b2-k-integration HEAD`：
   - **Case A（NO，当前实测拓扑：本分支自集成谱系分出，k-integration 非祖先）**：`git merge --no-ff codex/passb-b2-k-integration` 产生独立 merge commit；"Already up to date" 不可能出现，若出现转 Case B 复核。冲突面预判：`docs/plans/passb/`（K 系列 plan 文件 add/add 或双方新增——本侧保留本计划文件 `git checkout --ours docs/plans/passb/26-datasource.md`；其余 K 文件取 theirs）；`internal/modules/knowledge/module.go` 若冲突按「两侧注册全保留」处理；**涉冻结签名取舍的冲突停下升级**（conventions §10 Ruling 4）。
   - **Case B（YES，重派/中断恢复）**：merge 输出 "Already up to date" 不是前置失败；判据=`git log --merges --ancestry-path codex/passb-b2-k-integration..HEAD --oneline` 最近一条即既有对齐 merge commit；无法指认则上报。
   - 共同收尾判据：`go build ./...` 绿、`git status` 干净；**ALIGN_SHA**（=Case A 新 merge commit / Case B 指认的既有 merge commit）登记进报告——本节点全部 diff 检查的 `PASSB_BASE_SHA` 采用值（K5.3 Step 2 同裁定：缺省 merge-base origin/main 公式会把 K0–K4 全部产物算进本节点 diff，与 owned_files 求差必非空，禁用）。
3. **P-3 K 面产物在位**（对齐后实测）：`ls docs/architecture/passb/briefs/b2-k-integration.md docs/architecture/evidence/passb/b2-k-integration.md` 存在；`grep -c "^func RecordKBActivity\|^func KBActivityTrigger\|^func WithKBActivityTask\|^func WithKBActivitySuppressed" internal/modules/knowledge/retrieval/app/kb_activity.go` = **4**（K2 导出面就绪）；`grep -n "^func withKnowledgeCleanup" internal/application/service/knowledge_delete_plan.go` 仍在（K4 推迟件未迁，§2.2 裁定前提）。
4. **P-4 门禁工具可用**：`Makefile:250 verify-module-moves`、`:255 check-backend-architecture`、`:262 check-passb-readiness` 三目标在位（本分支实读确认）。
5. **P-5 worktree 干净**：`git status` 干净，分支 `codex/passb-b2-datasource`。
6. **P-6 基线门禁快照**（对齐后在 ALIGN_SHA 树实跑并留档，后续差分对照）：`go build ./...` 退出码 0；`go test -count=1 ./internal/modules/datasource/...` 全 ok；`make check-backend-architecture` 输出 `total=633 | redis=23 lite=23 | hooks=58 | modules=16`、`OK (0 violations)`；`make verify-module-moves` 输出 `OK (16 manifests verified)`（以上为本分支 HEAD 实测值，K 合并后计数不变——K 节点均按奇偶交付）。

### 0.2 节点裁定与 BLOCKED 链（调度方原文要旨）

1. **CORR-2 骨架修正**：由 [ib1] 增加 `b2-k-integration` 前置。datasource→knowledge 5 符号 24 调用点同宿主包未导出调用（调用方 datasource_service.go 归属 datasource.yaml:59；定义方分两处：kb_activity 4 函数在 kb_activity.go=K2 retrieval（knowledge-retrieval.md:11）、withKnowledgeCleanup 在 knowledge_delete_plan.go:22=K4 process（knowledge-process.md:12））——依赖 b2-k-integration 同时覆盖 K2 与 K4。若 IB1 裁定共享 shim/提前导出，可回写本 DAG 降级为并行（基线变更流程）。
2. **实测补充（本撰写会话，供实施者对齐后复核）**：K4 按 Ruling 2026-09-25-DEFERRED-FILE-SPLIT 把 `knowledge_delete_plan.go` 列入推迟批（24 计划 §3.3 批 2-12），**对齐后 `withKnowledgeCleanup` 仍未导出、仍留守宿主**（P-3 第 3 条判据）。因此本计划对 5 符号采用**分裂处置**（§2.2）：4 个 K2 已导出符号直连改写；1 个 K4 推迟符号走消费侧 seam + 宿主 compat 接线（K0 §6.1 R2 裁定的 datasource 侧兑现，先例=K3 faq `Seams` + 宿主委托 D1）。**这与 DAG notes「依赖 b2-k-integration 同时覆盖 K2 与 K4」的预期存在事实偏差（K4 侧以推迟+补迁窗兑现），按 conventions §5 在报告中如实登记，不构成阻塞**——seam 方案对「补迁窗已导出」与「未导出」两种时点均行为等价（接线源不同而已，§4.2）。
3. BLOCKED 历史链（2026-09-23 b0 → 09-24 b2-k0 → 09-25 b2-k-retrieval → 09-25/26 b2-k-process×2 → 09-26/27 b2-k-integration×3）：各前置 done 后逐次恢复；以 DAG `status` 字段为准。

---

## 1. 目标与范围

**目标**：把困在水平宿主包的 4 个 datasource legacy 文件迁入 `internal/modules/datasource` 冻结 destination，24 个 kbActivity 调用点按「4 符号直连 + 1 符号 seam」完成 knowledge 活动端口接线，宿主装配面经 3 个过渡 compat 文件零改动编译，sync 取消/重试/进度与 purge 级联差分等价证据落盘，12 条别名义务行与 3 条新例外行收口登记，节点四门禁绿色。

**范围内（owned_files = manifest legacy_files 4 条 + `internal/modules/datasource/**` + 本节点自身产出新文件）**：

| # | legacy 文件 | destination（ownership-matrix :164/:842/:1808/:1814 冻结） | 体量（本分支实测） |
|---|---|---|---|
| 1 | `internal/application/repository/datasource_repo.go` | `internal/modules/datasource/repository/datasource_repo.go`（package repository） | 505 行：`DataSourceRepository`（:14）+`NewDataSourceRepository`（:19）+`AppDataSourceBindingRow`（:156）+`SyncLogRepository`（:264）+`NewSyncLogRepository`（:269） |
| 2 | `internal/application/service/datasource_service.go` | `internal/modules/datasource/service/datasource_service.go`（package service） | 112,510 字节 / 62 func（:54 `NewDataSourceService`… :2654 `sweepStaleSubtree`；哨兵 `ErrReindexDuplicateRequest` :1157、`ErrSyncLogNotFound` :1353；常量 `dataSourcePurgeBatchSize=200` :816；窄接口 `dataSourceBindingCleaner` :823；`SyncSpaceStateResolver` :83） |
| 3 | `internal/handler/datasource.go` | `internal/modules/datasource/handler/datasource.go`（package handler） | 24,525 字节：`DataSourceHandler`（:19）+`NewDataSourceHandler`（:25）+20 方法/3 请求类型 |
| 4 | `internal/handler/datasource_credentials.go` | `internal/modules/datasource/handler/datasource_credentials.go` | 4,123 字节：`DataSourceCredentialsHandler`（:23）+`NewDataSourceCredentialsHandler`（:28）+Put/DeleteField |

随迁测试（framework:29 每搬一个生产文件随迁其 `_test.go`；分类依据 §2.4；**R1 修订补全 repository 侧 2 文件**——随迁 17 文件 + 留守 1 文件 = 18 文件 / **115 用例**）：

- **repository 侧 2 文件 → `internal/modules/datasource/repository/`**：`datasource_repo_test.go`（6 用例：TestDataSourceRepository×5 + TestSyncLogRepositoryUpdateResultClearsErrorMessage，全名清单见 §5 T1 Step 2 模式；构造器引用 :25/:56/:86/:114/:135/:172）、`datasource_repo_synclog_test.go`（4 用例 TestSyncLogLifecycle×4；构造器引用 :17/:46/:69/:114）——共享 helper `setupDataSourceRepoTestDB`（定义于 datasource_repo_test.go:15），仅引同包导出构造器，零宿主未导出符号 → **纯随迁零改写**（package repository 同名包，构造器直接解析到模块包，import 零修正）。
- **service 侧 11 文件 → `internal/modules/datasource/service/`**：`datasource_cancel_enqueue_test.go`（6 用例）、`datasource_credential_refresh_test.go`（4）、`datasource_credential_refresh_trigger_test.go`（14）、`datasource_delete_sqlite_test.go`（3）、`datasource_reindex_test.go`（8）、`datasource_result_cap_test.go`（3）、`datasource_service_test.go`（16）、`datasource_stream_test.go`（6）、`datasource_sweep_wiring_test.go`（7）、`datasource_sync_cancel_test.go`（7）、`datasource_sync_heartbeat_test.go`（2）——合计 76 用例随迁。
- **handler 侧 4 文件 → `internal/modules/datasource/handler/`**：`datasource_test.go`（9；`stubDataSourceService` 定义处 :17）、`datasource_documents_count_test.go`（6；`newOwnedKBStub`/`stubKBServiceForDS` 定义处 :30）、`datasource_reindex_test.go`（4）、`datasource_credentials_test.go`（1；`newCredentialsTestRouter` 定义处 :22）——helper 全部定义于本组 4 文件内（grep 实测），**自包含可纯随迁**。
- **留守宿主 1 文件（不随迁，B2-DS.3 同 commit 重写构造点）**：`internal/application/service/datasource_purge_test.go`（9 用例）——`:225` 白盒构造 `&knowledgeService{}`（K4 属主类型，推迟批留守），迁移不可行；K4 计划 §3.3 已登记其归属「→ 26-datasource/ib2」。

加上本节点自身产出：`internal/application/service/datasource_passb_compat.go`、`internal/application/repository/datasource_passb_compat.go`、`internal/handler/datasource_passb_compat.go`（3 个宿主过渡 shim，Ruling TRANSITION-SHIM-ROW-REGISTRATION 成对补行）、`internal/modules/datasource/README.md` 与 `internal/modules/datasource/legacy/README.md` 迁移状态回填、`docs/architecture/evidence/passb/b2-datasource.md`、`docs/plans/passb/reports/b2-datasource.md`、`docs/architecture/passb/briefs/b2-datasource.md`、`tools/architectureguard/check.go` importExceptions **数据行 3 条**（Ruling IMPORT-EXCEPTION-REGISTRY：数据行非逻辑，禁改判定逻辑）、`docs/architecture/passb/exception-ledger.yaml` 3 新行、`docs/architecture/moves/datasource.yaml` 与 `docs/architecture/passb/ownership-matrix.yaml` 行级收口（4 legacy 行删 + 12 alias 行删 + 3 shim 行成对补）。

**范围外（一律不动）**：

- `internal/router/router.go`、`internal/router/routes_infra.go`（:301 `RegisterDataSourceRoutes` 形参经 handler compat 别名继续解析）、`internal/router/task.go`（:295/:314）、`internal/router/sync_task.go`（:160-161）、`internal/router/task_inspector.go`（:100）、`internal/container/**`（:77-87 import、:309-310/:632-637/:760/:857 Provide/Invoke、:2311 `initConnectorRegistry`、:2362-2380 `injectDataSourceTaskInspector`/`startDataSourceScheduler`）、`internal/bootstrap/**`——全部集成工程师独占（conventions §3）。
- `internal/modules/datasource/module.go`（22 行零逻辑骨架，passbguard 冻结其注释形态：五操作 `//\t` 声明 + 计数句式 1/2/2；**门面实现归 IB2**——contracts.yaml datasource.facade + 27-appconnector §1 先例 + 13-execution §2 先例；本节点零触碰）。
- K 系列属主文件：`internal/modules/knowledge/**`、宿主 `internal/application/service/{kb_activity*,kbretrieval_passb_compat,knowledge_delete_plan,knowledge*,wiki*,tag*,kbshare*}.go`、`internal/application/repository/knowledge*.go`（compat 仅消费其编译产物，不改一字）。
- appconnector 属主：`internal/modules/appconnector/**`（service 文件消费其导出面 `appconnector.SyncBindingStore`/`BindingState`）；policy 属主：`internal/modules/policy/access`（消费 `access.WithKBTaskWrite`）。
- `internal/types/**`（含 interfaces/datasource.go 22 方法端口——**非 contracts.yaml 冻结契约，但为稳定共享端口，零改动**）、`internal/handler/dto`（platform 包，check.go:89 `platformPackageDirs` 整包豁免，模块 handler 继续导入不改）、migration 编号、`go.mod`/`go.sum`、生产 SQL、既有迁移文件、`cmd/desktop`、`docreader`、`client`。
- `docs/architecture/passb/{contracts,event-catalog}.yaml`（barrier 回写）；`ownership-matrix.yaml` 仅限本节点属主行的行级增删（Ruling LEGACY-ROW-OWNERSHIP/TRANSITION-SHIM-ROW-REGISTRATION）；`execution-dag.json`（协调者独占）。

## 2. 现状断链面（真实代码证据，本分支 HEAD 实测）

### 2.1 宿主装配层对本组符号的消费面（compat 必须覆盖的全集；shim 最小化——零消费符号不留转发）

| 消费方 | 符号 | 位置（实测） | compat 覆盖方式 |
|---|---|---|---|
| `internal/container/container.go` | `repository.NewDataSourceRepository` / `repository.NewSyncLogRepository`（dig Provide） | :309 / :310 | repository compat **var 别名** |
| 同上 | `service.NewDataSourceService`（dig Provide） | :633 | service compat **wrapper 函数**（§4.1，同签名+seam 接线） |
| 同上 | `handler.NewDataSourceCredentialsHandler` / `handler.NewDataSourceHandler`（dig Provide） | :760 / :857 | handler compat **var 别名** |
| 同上 | `svc.(*service.DataSourceService)` 类型断言（injectDataSourceTaskInspector） | :2368 | service compat **type 别名**（别名保型同一性，断言继续成立） |
| 同上 | `*datasource.ConnectorRegistry` / `*datasource.Scheduler` / 11 个 connector import | :77-87、:632、:2311-2380 | 模块根包既有导出，**零改动** |
| `internal/router/routes_infra.go` | `RegisterDataSourceRoutes(r, handler *handler.DataSourceHandler, credHandler *handler.DataSourceCredentialsHandler, g *rbacGuards)` 形参类型 | 签名 :301-306（形参类型 :303-304） | handler compat **type 别名**（路由体/RBAC guard 零改动，633 计数不动） |
| `internal/router/router.go`（R1 修订补列） | `RouterParams` 结构体字段 `DataSourceHandler *handler.DataSourceHandler` / `DataSourceCredentialsHandler *handler.DataSourceCredentialsHandler` 与消费行 `RegisterDataSourceRoutes(v1, params.DataSourceHandler, params.DataSourceCredentialsHandler, rbacGuards)` | :118-119 / :408 | handler compat **type 别名**（字段类型与实参零改动） |
| `internal/router/router_api_key_capabilities_test.go`（R1 修订补列） | `RegisterDataSourceRoutes(v1, &handler.DataSourceHandler{}, &handler.DataSourceCredentialsHandler{}, g)` | :353 | 同上（test-only，零改动自证） |
| `internal/router/task.go` / `sync_task.go` / `task_inspector.go` | `params.DataSourceService.ProcessSync` / `.ProcessDataSourcePurge`（接口方法值）；`types.TypeDataSourceSync/Purge` 常量集 | task.go:295/:314、sync_task.go:160-161、task_inspector.go:100 | 经 `interfaces.DataSourceService` 接口 + `internal/types` 常量，**零改动** |
| 宿主留守测试 `datasource_purge_test.go` | `NewDataSourceRepository`/`NewSyncLogRepository`/`repository.AppDataSourceBindingRow`/`datasource.NewScheduler` | 构造器 :142/:143、`AppDataSourceBindingRow` :139/:215/:219、`datasource.NewScheduler` :239 | repo compat var+type 别名；`datasource.NewScheduler` 为模块既有导出 |
| `internal/modules/appconnector/service/appconnector/sync_test.go` | `interfaces.DataSourceService`（接口形态） | （grep 实测，test-only） | 接口零改动，天然兼容 |
| `internal/application/service/knowledgebase.go`、`knowledgebase_delete_datasource_test.go`（R1 修订补列） | `interfaces.DataSourceRepository` / `interfaces.SyncLogRepository`（接口端口类型标注/`var _` 断言） | knowledgebase.go:50-51/:73-74；test :75/:114 | **零改动**——经 `internal/types/interfaces` 端口解析，非宿主 repository 包符号 |

宿主消费面全集以全量 sweep 实测封闭（R1 修订，本分支 HEAD）：`grep -rn "DataSourceHandler|DataSourceCredentialsHandler|NewDataSourceService|service.DataSourceService|NewDataSourceRepository|NewSyncLogRepository|AppDataSourceBindingRow|DataSourceRepository|SyncLogRepository"` 于 `internal/{router,container,handler,application,bootstrap}` + `cmd`（剔除本组 4 legacy 文件与其测试）——命中仅上表所列 router 三文件（routes_infra.go/router.go/router_api_key_capabilities_test.go）、container.go 五处、interfaces 端口标注两类（knowledgebase.go、knowledgebase_delete_datasource_test.go）、appconnector sync_test.go，别无其他。哨兵 `service.ErrReindexDuplicateRequest`/`service.ErrSyncLogNotFound` 的宿主消费者仅 `internal/handler/datasource.go:525/:761` 与随迁测试（`datasource_test.go:364`、`datasource_reindex_test.go:126`、`datasource_sync_cancel_test.go:117/:119`）——**全部随本节点迁移，宿主无需 var 别名**（shim 最小化）。

### 2.2 datasource→knowledge 5 符号 24 调用点（DAG ppc pair #1；本会话双向 grep 实证）

定义方状态（**P-2 对齐后复验**；下表「定义方（对齐后）」按 K2 已交付、K4 推迟留守描述）：

| 符号 | 定义（base 实测 → 对齐后） | 调用点（datasource_service.go，本分支行号） | 处置 |
|---|---|---|---|
| `recordKBActivity` | service/kb_activity.go:95 → `internal/modules/knowledge/retrieval/app/kb_activity.go:103` `func RecordKBActivity(ctx context.Context, audit interfaces.AuditLogService, tenantID uint64, kbID string, action types.AuditAction, targetType string, targetID string, outcome types.AuditOutcome, details map[string]any)`（K2 导出，20 计划 §6.2 组 B） | **17 处**：:258、:374、:420、:462、:525、:778、:915、:1065、:1111、:1127、:1201、:1301、:1325、:1461、:1529、:1853、:2425 | **直连改写** `app.RecordKBActivity`（K2 裁定「调用点 ib2 直连改写」的本节点提前兑现——本节点为文件属主且导出面已就绪） |
| `withKBActivityTask` | kb_activity.go:27 → app/kb_activity.go:35 `func WithKBActivityTask(ctx context.Context, taskID, trigger string) context.Context` | **2 处**：:837、:1405 | 直连 `app.WithKBActivityTask` |
| `kbActivityTrigger` | kb_activity.go:36 → app/kb_activity.go:44 `func KBActivityTrigger(ctx context.Context) string` | **1 处**：:837（与上行同语句） | 直连 `app.KBActivityTrigger` |
| `withKBActivitySuppressed` | kb_activity.go:87 → app/kb_activity.go:95 `func WithKBActivitySuppressed(ctx context.Context) context.Context` | **3 处**：:1642、:1790、:2106（全仓仅此 3 处，22 计划 :79 同结论） | 直连 `app.WithKBActivitySuppressed` |
| `withKnowledgeCleanup` | service/knowledge_delete_plan.go:22 `func withKnowledgeCleanup(ctx context.Context, tenant uint64, bindings map[string]string) context.Context`——**K4 推迟件，对齐后仍未导出**（24 计划 §3.3 批 2-12 + §:119 顺延符号） | **1 处**：:885（`PurgeDataSourceDocuments` 内 `s.knowledgeService.DeleteKnowledgeList(withKnowledgeCleanup(ctx, payload.TenantID, bindings), ids)`） | **消费侧 seam**（§4.2）：新增字段+`SetKnowledgeCleanup` 导出 setter；宿主 compat 构造包装器以同包 `withKnowledgeCleanup` 接线——与迁移前同目标函数，行为零变化（K0 §6.1 R2；先例 K3 faq Seams + D1 委托） |

计 17+2+1+3+1=**24 调用点 / 23 行**（:837 一行双符号）。改写示例（:885）见 §4.2。

### 2.3 搬迁显形跨模块 import（B2-DS.5 按 Ruling IMPORT-EXCEPTION-REGISTRY 登记精确豁免）

architectureguard forbidden-import 仅扫 `internal/modules/**` 文件对 `internal/modules/<他模块>` 的导入（check.go:1175-1220 WalkDir modRoot 实读；`_test.go` 跳过 :1186；host→module 与 module→host 横向方向均不在判定域——27 计划 §2.2 同结论）。搬迁使下列既有 import 变为 module→module，须登记精确 file→package 豁免（种子对，importer 均为落位后精确路径）：

| # | importer（落位后） | imported | 现消费点（base 行号） | 用途 |
|---|---|---|---|---|
| 1 | `internal/modules/datasource/service/datasource_service.go` | `internal/modules/knowledge/retrieval/app` | 新增 import（24 调用点直连，§2.2） | knowledge 活动端口（K2 导出面） |
| 2 | 同上 | `internal/modules/appconnector` | :18 import；`syncBindingStore appconnector.SyncBindingStore`（:46）、`SyncSpaceStateResolver.SpaceBindingState(...) *appconnector.BindingState`（:83-86） | A07 scoped-sync 绑定存储 |
| 3 | 同上 | `internal/modules/policy/access` | :21 import；`access.WithKBTaskWrite(ctx, kb, ds.TenantID)`（:1440，全文件唯一调用） | KB 写入任务上下文门控 |

同模块导入（`internal/modules/datasource` 根包 `ConnectorRegistry`/`Scheduler`、`connector/ima` 哨兵 `ima.ErrTargetedRefetchUnsupported` :1808）owner==datasource 不触发判定。横向导入（`internal/logger`、`internal/tracing/langfuse`、`internal/types`、`internal/utils`、`internal/handler/dto`）不在判定域。handler 侧：`internal/application/service` import（哨兵错误）随两文件同时迁移消亡，改为同模块 `internal/modules/datasource/service` 导入（合法）。**无 import 环**（R1 修订更正——原文「K2 已登记例外 app/knowledgebase.go:15」为不实引用，该文件不存在）：对齐目标分支 `codex/passb-b2-k-integration` 实测（`git ls-tree --name-only codex/passb-b2-k-integration -- internal/modules/knowledge/retrieval/app/` 列 14 项，无 knowledgebase.go，实为 kb_activity.go、knowledgebase_access.go、graph.go、audit_actor_seam.go、semantic_*×5、slug_fuzzy*×2、handler/、repository/；`git grep -l "internal/modules/datasource" codex/passb-b2-k-integration -- internal/modules` 剔除 datasource 模块自身后**零命中**）——modules 内不存在任何指向 datasource 的反向 import 边，datasource/service → knowledge/retrieval/app 为**单向**文件级有向边，无包级环（`go build ./...` 实证）。K2 已登记的 retrieval/app 例外实为 5 条（K 分支 ledger exc-0112..0116 / check.go :987-:1021：semantic_model_capability.go→commercial、semantic_model_policy.go→commercial、knowledgebase_access.go→policy/access、graph.go→airesource/models/chat、graph.go→airesource/models/utils），无一指向 datasource，与本节点新增 3 条例外无交叠。

### 2.4 测试夹具分类（18 文件 / 115 用例逐文件实测；R1 修订补全 repository 侧 2 文件）

- **repository 侧 2 文件（纯随迁零改写）**：`datasource_repo_test.go`（6）+ `datasource_repo_synclog_test.go`（4）经逐行核验仅引同包导出构造器（`NewDataSourceRepository`/`NewSyncLogRepository`）与自有共享 helper `setupDataSourceRepoTestDB`（datasource_repo_test.go:15，两文件共享），无任何宿主未导出符号、无 repo 生产文件内部方法白盒——git mv 后同名包 `package repository` 内构造器直接解析到模块包自身，**零 import 修正零改写**（此前漏列为计划缺陷，R1 补全）。同目录 `knowledge_datasource_test.go`/`knowledge_datasource_external_id_test.go` 不引用本组构造器（grep 实测零命中），属 K 属主文件，不在本节点范围。

- **随迁判定依据**：service 侧 11 文件对宿主符号的引用经 grep 逐文件核验，仅为 ①`&DataSourceService{knowledgeService: ks}` 白盒复合字面量（字段名，非 K4 类型标注——`grep -n "knowledgeService" ` 实测 :460-:756 等 21 处全为字段名；K4 计划 Step 4 同结论「经接口形态，compat var 别名即可保护，确认无 *knowledgeService 类型标注」）②`NewDataSourceRepository`/`NewSyncLogRepository`（本节点 repo 构造器，随迁后改 import 模块路径）③`applyFetchedItem` 白盒方法（:1927 定义，reindex 1/result_cap 5/service_test 1/sweep_wiring 3 处调用——必须同包，**构成随迁必要性**）。复合字面量设未导出字段只能在定义包内合法（Go 语言规则），留守不可行。
- **留守判定依据**：`datasource_purge_test.go` `:225` `knowledgeService := &knowledgeService{repo:…, kbService:…, tenantRepo:…, chunkRepo:…, graphEngine:…}`（K4 属主未导出类型的白盒复合字面量，字段全未导出）——无法迁出宿主；其 9 用例是 purge 级联（drain/墓碑/批次边界取消/孤儿 tag/app_datasource_bindings 清理）的**唯一覆盖**（全仓 `grep -ln PurgeDataSourceDocuments *_test.go` 仅命中此文件），必须保活。留守重写点：`:243` 与 `:467` 两处 `&DataSourceService{…}` 白盒字面量 → 改经 compat `NewDataSourceService` 构造（§4.3）；`:469` `f.svc.knowledgeService` 未导出字段读 → fixture 持有 `f.ks` 字段引用。
- handler 侧 4 文件 helper 自包含（§1 已列定义位置）；`datasource_test.go`/`datasource_reindex_test.go` 对 `service.ErrSyncLogNotFound`/`service.ErrReindexDuplicateRequest` 的引用随迁后改 import 模块 service 路径。

### 2.5 别名现状：12 条 alias 行为空义务（Pass A 集成期已物理删除）

本分支实测：`ls internal/datasource` → No such file or directory；`grep -rn "WeKnora/internal/datasource" --include="*.go" internal cmd` 排除 `modules/datasource` 后**零命中**。`datasource.yaml` alias_obligations 12 行（:30-:53）与 ownership-matrix :2547-:2583 对应行为**空义务**（27 计划 §2.4 同型事实）。处置：B2-DS.6 成对删行核销（Ruling LEGACY-ROW-OWNERSHIP「可早删不可晚删」+ K5.2 先例；与 27 计划「不改治理 YAML 留 IB2」的差异以 K5.2 后例为准——本节点为行属主且 K5 已树立成对删行先例）。

### 2.6 例外现状：本节点零属主行，B2-DS.5 新增 3 行

本分支 ledger 105 行（exc-0001..0105）中 `plan: 26-datasource` 零行（grep 实测）。新增 3 行（§2.3 三对）：owner=`26-datasource`、remove_at=`ib2`、reason=「预存横向包耦合（Pass A 前宿主文件→模块导入，26 号搬迁后显形 module→module），Pass B 不改边界」；**id 分配在对齐后的 ledger 现值上顺延**（K 节点在对齐分支已增 exc-0106..0131 区段，实施时以 `grep -o "id: exc-[0-9]*" | sort | tail -1` 实测 max 顺延，不硬编码）。机械计数修正：105（对齐后实值）→+3，§8 基线登记（台账+原因+独立 commit 窗口内完成，conventions §8）。

## 3. 目标包结构与写入所有权

```text
internal/modules/datasource/          # 既有（Pass A）：connector.go、scheduler.go、errors.go、
                                      # filename.go、httpclient.go、connector/**、module.go(禁改)、
                                      # README.md(回填)、legacy/README.md(回填)
  repository/datasource_repo.go       # 新增（B2-DS.2 git mv）
  repository/*_test.go ×2             # 随迁（B2-DS.2 同 commit git mv，纯随迁零改写）
  service/datasource_service.go       # 新增（B2-DS.3 git mv + 24 调用点改写 + seam）
  service/*_test.go ×11               # 随迁
  handler/datasource.go               # 新增（B2-DS.4 git mv）
  handler/datasource_credentials.go   # 同上
  handler/*_test.go ×4                # 随迁
internal/application/repository/datasource_passb_compat.go   # 新增（B2-DS.2，过渡 shim）
internal/application/service/datasource_service.go           # 删除（B2-DS.3 物理迁移同 commit）
internal/application/service/datasource_passb_compat.go     # 新增（B2-DS.3，过渡 shim + seam 接线）
internal/application/service/datasource_purge_test.go       # 保留 + 重写 3 处构造点（B2-DS.3）
internal/handler/datasource*.go                               # 删除（B2-DS.4）
internal/handler/datasource_passb_compat.go                  # 新增（B2-DS.4，过渡 shim）
docs/architecture/moves/datasource.yaml                      # legacy 4 行删 + shim 3 行增 + alias 12 行删
docs/architecture/passb/ownership-matrix.yaml                # 镜像同窗成对增删（legacy 4+3、aliases 12）
docs/architecture/passb/exception-ledger.yaml                # +3 行（B2-DS.5）
tools/architectureguard/check.go                             # importExceptions +3 数据行（B2-DS.5）
docs/architecture/passb/briefs/b2-datasource.md              # 新增（B2-DS.8）
docs/architecture/evidence/passb/b2-datasource.md            # 新增（B2-DS.1 起累积，B2-DS.7 定稿）
docs/plans/passb/reports/b2-datasource.md                    # 新增（B2-DS.8）
```

**禁改**（conventions §3，违者节点失败）：`internal/router/**`、`internal/container/**`、`internal/bootstrap/**`、migration 编号、`go.mod`/`go.sum`、生产 SQL、既有迁移文件、`internal/modules/datasource/module.go`、`docs/architecture/passb/{contracts,event-catalog}.yaml`、K 系列属主文件、appconnector/policy/airesource 属主文件、`internal/types/**`、`execution-dag.json`、guard 判定逻辑（check.go 仅限 importExceptions 数据行追加）。

## 4. 迁移设计（真实签名，不发明）

### 4.1 宿主过渡 compat（3 文件，全部 no-logic forwarding；文件头注释 `// Pass B 过渡 shim：删除点 ib2（Integration Brief 登记）`）

**(a) `internal/application/repository/datasource_passb_compat.go`**（B2-DS.2）：

```go
package repository

// Pass B 过渡 shim（26-datasource）：datasource_repo.go 已物理迁移至
// internal/modules/datasource/repository。本文件为留守宿主消费方
// （container.go:309-310 dig Provide、datasource_purge_test.go）提供
// type/var 别名，调用点零改动。删除点：ib2 集成屏障直连后（Brief 指令）。

import (
	"github.com/Tencent/WeKnora/internal/modules/datasource/repository"
	"gorm.io/gorm"
)

type DataSourceRepository = repository.DataSourceRepository
type SyncLogRepository = repository.SyncLogRepository
type AppDataSourceBindingRow = repository.AppDataSourceBindingRow

var (
	NewDataSourceRepository = repository.NewDataSourceRepository
	NewSyncLogRepository    = repository.NewSyncLogRepository
)
```

（`var` 别名保函数值可作 dig Provide；`AppDataSourceBindingRow` 别名保护留守 purge_test :139/:215/:219 复合字面量——字段全导出，跨包合法。gorm import 若未直接使用则省略。repo 侧 2 个测试文件随 B2-DS.2 同 commit 迁至模块包后，宿主 repository 包**无测试消费者**，本 compat 服务面=container.go:309/:310 + 留守 purge_test :142/:143——R1 修订口径。）

**(b) `internal/application/service/datasource_passb_compat.go`**（B2-DS.3）：

```go
package service

// Pass B 过渡 shim（26-datasource）：datasource_service.go 已物理迁移至
// internal/modules/datasource/service。本文件为留守宿主消费方
// （container.go:633 dig Provide、:2368 类型断言、datasource_purge_test.go）
// 提供 type 别名与构造包装器，并把模块侧 cleanup seam 接到宿主现行
// withKnowledgeCleanup（knowledge_delete_plan.go:22，K4 推迟批留守未导出）——
// 与迁移前同包直引同一目标函数，行为零变化。删除点：ib2（含 K4 补迁窗
// 导出后的直连改写，见 Brief (b)）。

import (
	"github.com/Tencent/WeKnora/internal/modules/datasource"
	"github.com/Tencent/WeKnora/internal/modules/datasource/service"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type DataSourceService = service.DataSourceService

// NewDataSourceService 与模块构造器同签名（10 参，逐参照录 :54-66 实测），dig 兼容；
// 构造后立即接线 knowledge cleanup seam（生产装配唯一入口）。断言写法镜像
// container.go:2368 既有模式（SetKnowledgeCleanup 在具体类型上，接口断言后调用）。
func NewDataSourceService(
	dsRepo interfaces.DataSourceRepository,
	syncLogRepo interfaces.SyncLogRepository,
	knowledgeService interfaces.KnowledgeService,
	kbService interfaces.KnowledgeBaseService,
	taskEnqueuer interfaces.TaskEnqueuer,
	connectorRegistry *datasource.ConnectorRegistry,
	scheduler *datasource.Scheduler,
	tenantRepo interfaces.TenantRepository,
	tagService interfaces.KnowledgeTagService,
	audit interfaces.AuditLogService,
) interfaces.DataSourceService {
	svc := service.NewDataSourceService(dsRepo, syncLogRepo, knowledgeService, kbService,
		taskEnqueuer, connectorRegistry, scheduler, tenantRepo, tagService, audit)
	if impl, ok := svc.(*service.DataSourceService); ok && impl != nil {
		impl.SetKnowledgeCleanup(withKnowledgeCleanup)
	}
	return svc
}
```

（宿主→模块根包/模块 service 子包导入为 host→module 方向，不在 forbidden-import 判定域；`withKnowledgeCleanup` 为同包 K4 推迟件符号，编译期锚定；断言失败分支不可达——模块构造器恒返回具体类型，防御式写法仅为与 container.go:2368 同型。）

**(c) `internal/handler/datasource_passb_compat.go`**（B2-DS.4）：

```go
package handler

// Pass B 过渡 shim（26-datasource）：datasource.go / datasource_credentials.go
// 已物理迁移至 internal/modules/datasource/handler。本文件为 routes_infra.go:301-306
// （形参 :303-304）、router.go:118-119（RouterParams 字段）/:408（消费行）、
// router_api_key_capabilities_test.go:353 与 container.go:760/:857 dig Provide
// 提供 type/var 别名，router 侧零改动。
// 删除点：ib2 集成屏障直连后（Brief 指令）。

import (
	"github.com/Tencent/WeKnora/internal/modules/datasource/handler"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type DataSourceHandler = handler.DataSourceHandler
type DataSourceCredentialsHandler = handler.DataSourceCredentialsHandler

var (
	NewDataSourceHandler           = handler.NewDataSourceHandler
	NewDataSourceCredentialsHandler = handler.NewDataSourceCredentialsHandler
)
```

（interfaces import 若仅 var 别名不需要则省略。）

**成对补行**（Ruling TRANSITION-SHIM-ROW-REGISTRATION；K4 kbprocess 先例逐字格式，本分支 git show 实测）：manifest `datasource.yaml` legacy_files 增 3 行（`reason: Pass B 过渡 shim，ib2 同 commit 随文件删行`、`navigation_label: Datasource host compat (application/repository|application/service|handler)`、`passb_task: B-datasource`）；ownership-matrix legacy 区增 3 行（`module: datasource`、`plan: 26-datasource`、`destination: internal/modules/datasource/{repository|service|handler}`、`integration_owner: ib2`、`delete_barrier: ib2`）。**与 shim 文件落地同 commit**。

### 4.2 模块侧改写（B2-DS.3 主体；全部为机械等价变换，无行为变化）

1. **import 追加**：`kactivity "github.com/Tencent/WeKnora/internal/modules/knowledge/retrieval/app"`（别名按实施者惯例，下文以 `app.` 示意）。
2. **23 行直连改写**（§2.2 表）：`recordKBActivity(` → `app.RecordKBActivity(`（17 处）；`:837` `ctx = withKBActivityTask(ctx, taskID, kbActivityTrigger(ctx))` → `ctx = app.WithKBActivityTask(ctx, taskID, app.KBActivityTrigger(ctx))`；`:1405` `ctx = withKBActivityTask(ctx, taskID, payload.Trigger)` → `app.WithKBActivityTask(ctx, taskID, payload.Trigger)`；`:1642/:1790/:2106` `withKBActivitySuppressed(ctx)` → `app.WithKBActivitySuppressed(ctx)`。
3. **cleanup seam**（:885 唯一 K4 推迟符号）：

```go
// struct 新增字段（DataSourceService 定义 :30-52 区段末尾——type 行 :30、
// 闭合 } :52；:33 为 knowledgeService 字段行，非 struct 起点）：
	// knowledgeCleanup mirrors host withKnowledgeCleanup (knowledge_delete_plan.go:22,
	// K4 deferred batch — unexported). Wired by the host compat constructor
	// (datasource_passb_compat.go) to the same function the pre-migration code
	// called in-package. nil keeps ctx unchanged; allowed only in test
	// constructions that never exercise the purge-cleanup path (faq.Seams 同口径).
	knowledgeCleanup func(ctx context.Context, tenant uint64, bindings map[string]string) context.Context

// 新增导出 setter（紧邻 SetTaskInspector :107 之后，注入式 setter 先例同文件 :91/:107）：
// SetKnowledgeCleanup installs the knowledge-domain delete-cleanup context hook.
func (s *DataSourceService) SetKnowledgeCleanup(fn func(ctx context.Context, tenant uint64, bindings map[string]string) context.Context) {
	s.knowledgeCleanup = fn
}

// :885 调用点改写（PurgeDataSourceDocuments 内）：
		cleanupCtx := ctx
		if s.knowledgeCleanup != nil {
			cleanupCtx = s.knowledgeCleanup(ctx, payload.TenantID, bindings)
		}
		if err := s.knowledgeService.DeleteKnowledgeList(cleanupCtx, ids); err != nil {
```

（签名逐字照录 knowledge_delete_plan.go:22 实测 `func withKnowledgeCleanup(ctx context.Context, tenant uint64, bindings map[string]string) context.Context`；nil 分支仅测试构造可达——生产唯一装配入口=compat wrapper 必接线；purge 级联的绑定清理断言在留守 purge_test 内经 compat 装配运行，即 seam 等价性的差分锚点，§6。）
4. **随迁 import 修正**：文件内 `internal/modules/datasource` / `connector/ima` 导入相对路径不变（同模块跨包）；宿主 package 声明 `package service` → 保持 `package service`（新目录同名包）。
5. **随迁测试修正**：11 文件中 `repository.NewDataSourceRepository/NewSyncLogRepository`（cancel_enqueue/delete_sqlite/sync_cancel/sync_heartbeat/credential_refresh_trigger 等，grep 实测仅此两符号）→ import `internal/modules/datasource/repository`；handler 侧 2 文件的 `service.ErrX` → import `internal/modules/datasource/service`。

### 4.3 留守 purge_test 重写（B2-DS.3 同 commit；仅构造点，断言零变化）

1. `:240-:248` 字面量（7 字段：dsRepo/syncLogRepo/knowledgeService/tagService/taskEnqueuer/scheduler/audit）→ `f.svc = NewDataSourceService(dsRepo, syncLogRepo, knowledgeService, nil, f.enqueuer, nil, f.scheduler, nil, tagService, f.audit)`（参数序照录构造器实测签名 :54-66；kbService/connectorRegistry/tenantRepo 原字面量未设置=零值，传 nil 等价；taskInspector/syncBindingStore 等其余字段构造器本就不设，与原字面量零值一致）。fixture 增存 `f.ks = knowledgeService`（供 2 用）。
2. `:467-:470` 字面量+字段读 → `svc := NewDataSourceService(f.dsRepo, nil, &cancelAfterBatchKS{KnowledgeService: f.ks, cancel: cancel}, nil, nil, nil, nil, nil, nil, nil)`（原字面量仅设 dsRepo/knowledgeService 两字段，其余 nil 等价）。
3. 断言、fixture 其余部分零改动。留守文件对 K 系列宿主符号的引用经 K 宿主 compat 继续解析（本节点不触碰）：`:83` `recordKBActivity` 为注释；`:225` `&knowledgeService{…}`（K4 推迟件 knowledge.go 留守宿主，直引成立）；`:226` `repository.NewKnowledgeRepository` / `:234` `repository.NewKnowledgeTagRepository` / `:232` `&knowledgeTagService{…}`——对齐后经 K4 宿主 compat `kbprocess_passb_compat.go`（repository 侧已迁，compat 一行委托）与 K2 推迟留守 `tag.go`（`knowledgeTagService` 定义处）解析；`:239` `datasource.NewScheduler` 为模块既有导出。实施时以对齐树 `go build`/`go test` 实证为准，任一断链按 conventions §5 登记而非现场改 K 属主文件。

### 4.4 公共可观察行为与兼容要求（不变式清单）

1. **HTTP 面**：633 路由、方法、路径、RBAC/APIKey 门、状态码映射零变化——`RegisterDataSourceRoutes`（routes_infra.go:301）经 handler compat type 别名解析到同一实现；本节点零 router 改动。
2. **Worker 面**：`TypeDataSourceSync`/`TypeDataSourcePurge` 双栈注册（task.go:295/:314 asynq mux + sync_task.go:160-161 Lite executor）经 `params.DataSourceService` 接口方法值——接口与注册行零改动；redis=23 lite=23 奇偶不变。
3. **生命周期面**：`injectDataSourceTaskInspector`（container.go:636→:2362）经 compat type 别名断言继续命中；`startDataSourceScheduler`（:637→:2373）模块 Scheduler 零改动；58 挂点计数不变。
4. **审计活动流**：`app.RecordKBActivity` 与宿主旧 `recordKBActivity` 为 K2 R1 裁定的同一实现体（kb_activity.go 已随 K2 整体迁至 app 包，宿主 compat 为一行委托），17 调用点的入参/时机/outcome 逐字不变——purge_test `purgeAuditSink.findByAction` 断言即为等价证据。
5. **purge 级联语义**（spec §6.2 数据流边界：Data Source 不直接写 Knowledge 表）：drain 批次 200（`dataSourcePurgeBatchSize`）、墓碑→硬删两段、ctx 取消在批次边界终止（重试幂等）、孤儿 tag tail 清理、`app_datasource_bindings` 行删除、`:885` cleanup-context 绑定清理——全部由 purge_test 9 用例锚定，改写后同用例重跑等价（§6 差分）。
6. **错误契约**：`ErrReindexDuplicateRequest`/`ErrSyncLogNotFound` 哨兵值随文件迁移**同一变量**（错误文本 `errors.Is` 语义跨包不变）；`asynq.SkipRetry` 语义零变化。
7. **事件面**：event-catalog 零 datasource 事件，零改动。

## 5. 任务分解（业务完整、可独立审阅；严格按序执行；一任务一 commit）

> 任务编号 `B2-DS.n` 即 DAG `task_ids` 回填单位。全部命令在 worktree 根、P-2 对齐后的分支执行。

### T1 — B2-DS.1 前置核验 + 基线对齐 + 特征化基线【commit: `test(passb): b2-datasource 前置核验与特征化基线`】

- [ ] **Step 1**：逐条执行 §0.1 P-1..P-6，每条命令原文+退出码入报告；P-2 按 Case A/B 完成对齐 merge 并登记 `ALIGN_SHA`（merge commit 本身即基线对齐产物，Ruling WAVE-DEP-BASELINE；`git log --oneline -3` 留档）。
- [ ] **Step 2 特征化基线（conventions §1.4 搬迁类先特征化；复用现有 115 用例，不新写；R1 修订：模式全部经本分支实跑验证）**。计数口径：顶层用例 = `-v` 输出中不含 `/` 的 `=== RUN` 行（子测试行含 `/`，不计入用例数）。

```bash
# ① service 侧（11 随迁文件 + purge 留守文件 = 85 顶层用例；本分支实跑 85/85 ok）
go test -count=1 ./internal/application/service/ -run 'TestDeleteDataSourceH|TestDeleteDataSourceP|TestDeleteDataSourceW|TestDeleteKnowledgeBaseCleansUpSQLiteDataSources|TestSetTaskInspector|TestPauseDataSource|TestManualSync|TestRefreshDataSourceCredential|TestProcessSync|TestIncrementAppDataSourceBindingAuthVersion|TestCursorAuthVersionStale|TestReindexItems|TestApplyFetchedItem|TestAllFetchedItemsFailedError|TestIngestItem|TestStream|TestSyncHeartbeat|TestCancelSyncLog|TestCheckpoint|TestCheckCancelRequested|TestPurgeWorker|TestProcessDataSourcePurge|TestDataSourceService|TestDataSourcePurgeQueueTopology|TestCountDataSourceDocumentsScopesToTenantKbDataSource' -v 2>&1 | tail -20
# 预期：85 顶层用例全 PASS；逐用例清单（PASS/FAIL/SKIP+原因）写入 evidence §基线；出现 FAIL 即停，按 conventions §5 上报（不得继续搬迁）
#   模式设计说明（勿「简化」）：裸 TestDeleteDataSource/TestDataSource/TestSync/TestDeleteKnowledgeBase 前缀会碰撞 K 属主
#   留守用例——TestDeleteDataSourcesForKnowledgeBase(ContinuesOnDeleteError)（knowledgebase_delete_datasource_test.go）、
#   TestDataSourceTagCreationReceivesOnlyItsTaskKBGrant（knowledge_write_access_test.go）、
#   TestSyncEditedChunkImagesDisablesAndRestoresImageChildren（chunk_edit_parent_test.go）、
#   TestDeleteKnowledgeBaseForwardsDataSourceTaskScope|...CancelsQueuedTasksBestEffort（knowledgebase_task_cancel_test.go）
#   ——故用 TestDeleteDataSourceH|P|W 收窄（K 属主复数形为 ...DataSources...，H/P/W 后缀必不匹配）。
# ② repository 侧（2 文件 10 顶层用例；本分支实跑 10/10 ok，2.667s）
go test -count=1 ./internal/application/repository/ -run 'TestDataSourceRepository|TestSyncLogRepository|TestSyncLogLifecycle' -v 2>&1 | tail -5
# 预期：10 顶层用例全 PASS（TestDataSourceRepository×5|TestSyncLogRepository×1|TestSyncLogLifecycle×4；R1 补全）
# ③ handler 侧（4 文件 20 顶层用例；本分支实跑 ok，29 RUN 行 = 20 顶层 + 9 子测试[Delete_PurgeDocumentsParamMatrix×4、ManualSync_ForceFullBodyMatrix×5]）
go test -count=1 ./internal/handler/ -run 'TestDataSource' -v 2>&1 | tail -5
# 预期：20 顶层用例全 PASS（handler 包内 TestDataSource 前缀无碰撞，全量 grep 实测）
go test -count=1 ./internal/modules/datasource/...   # 预期：全 ok（模块既有面基线）
```

- 合计 **115 用例基线**（85 + 10 + 20），与 §1 随迁/留守清单一一对应。

- [ ] **Step 3 覆盖面核对（高风险差分锚点定位）**：确认以下行为均有既有用例锚定（grep 用例名实证，无则**在宿主新写最小特征化测试并随 B2-DS.3 随迁**——本会话核对结论：全覆盖，预期零新增）：purge drain 跨批（`TestPurgeWorkerDrainsAcrossBatches`）、ctx 取消批次边界（`TestPurgeWorkerStopsBetweenBatchesWhenContextCanceled`）、墓碑/硬删（`TestDeleteDataSourcePurgeEnqueuesTaskAndWorkerDrainsDocuments`）、孤儿 tag（`TestPurgeWorkerRemovesOrphanAutoTag`）、不 purge 保留（`TestDeleteDataSourceWithoutPurgeKeepsDocuments`）、绑定清理 + 兄弟行存活（:355-:356 `bindingRows` 断言）、硬取消顺序/无 inspector 降级（`TestDeleteDataSourceHardCancelsQueuedSyncTasksBeforeSweep`/`...WithoutInspectorDegradesToSweep`/`TestPauseDataSourceCancelsRunningAndQueuedSyncs`）、asynq TaskID 记录与 force-full 载荷（`TestManualSyncRecordsAsynqTaskID`/`...PassesForceFullToPayload`）、租户隔离 404（sync_cancel :117-:119 `ErrSyncLogNotFound` 双断言）、心跳（sync_heartbeat 2 用例 + repo 侧 `TestSyncLogLifecycleUpdateHeartbeat`）、SyncLog cancel 标志/stall 窗口（repo 侧 `TestSyncLogLifecycleRequestCancel`/`...HasRunningSyncExcludesStalledRuns`——R1 补全的 repo 10 用例即 worker 状态机底层语义锚点）、凭据轮换/auth-version 游标（credential_refresh* 18 用例）。
- **产出**：`docs/architecture/evidence/passb/b2-datasource.md` 骨架 + §前置核验 + §特征化基线（用例清单+命令+退出码）。
- **验收**：P-1..P-6 全绿留档；**115 用例**基线清单落盘；ALIGN_SHA 登记。

### T2 — B2-DS.2 repository 搬迁 + 宿主 compat + 行级收口【commit: `refactor(datasource): passb B2-DS.2 repository 迁入模块与宿主 compat`】

- [ ] **Step 1**：`git mv internal/application/repository/datasource_repo.go internal/modules/datasource/repository/datasource_repo.go`（目录新建；package repository；文件体零改动——imports 均为 gorm/types/interfaces，无宿主符号）；**同 commit `git mv internal/application/repository/datasource_repo_test.go internal/application/repository/datasource_repo_synclog_test.go internal/modules/datasource/repository/`**（framework:29 随迁；纯随迁零改写——同名包 + 导出构造器 + 自有 helper `setupDataSourceRepoTestDB`，R1 修订补全）。
- [ ] **Step 2**：落盘 §4.1(a) compat 文件。
- [ ] **Step 3 行级收口（同 commit，Ruling LEGACY-ROW-OWNERSHIP）**：`docs/architecture/moves/datasource.yaml` 删 legacy_files 行 `internal/application/repository/datasource_repo.go`（:55-:58）+ 增 shim 行；`docs/architecture/passb/ownership-matrix.yaml` 删 :164-:168 行 + 增 shim 行（§4.1 成对格式）。
- [ ] **Step 4 验证**：

```bash
go build ./...                                    # 预期：退出码 0
go test -count=1 ./internal/modules/datasource/repository/   # 预期：ok，10 顶层用例全 PASS（T1 repo 基线同模式复跑；R1 修订后不再是无测试包）
go test -count=1 ./internal/application/service/ -run 'TestDeleteDataSourceWithoutPurgeKeepsDocuments|TestProcessSync'   # 预期：PASS（留守测试经 compat 编译运行）
make verify-module-moves                          # 预期：OK (16 manifests verified)（行集增删成对）
git diff --summary HEAD~1..HEAD | grep -c rename  # 预期：≥1（git 识别 rename）
```

- **验收**：`git diff --name-only HEAD~1` ⊆ {datasource_repo.go 新旧路径、datasource_repo_test.go 新旧路径、datasource_repo_synclog_test.go 新旧路径、repo compat、datasource.yaml、ownership-matrix.yaml、（空目录清理）}。

### T3 — B2-DS.3 service 搬迁：kbActivity 直连 23 行 + cleanup seam + 11 测试随迁 + purge_test 重写【commit: `refactor(datasource): passb B2-DS.3 service 迁入模块（kbActivity 直连 + cleanup seam）`】

- [ ] **Step 1**：`git mv internal/application/service/datasource_service.go internal/modules/datasource/service/datasource_service.go`；按 §4.2.1-4 改写（import 追加、23 行直连、seam 字段+setter+:885 调用点）。
- [ ] **Step 2**：`git mv` 11 个 service 测试文件至 `internal/modules/datasource/service/`；按 §4.2.5 修 import（repository 两构造器→模块 repo 包）。
- [ ] **Step 3**：落盘 §4.1(b) service compat + §4.3 purge_test 重写（同 commit——service 离开宿主的同一变更集内完成留守测试保活）。
- [ ] **Step 4 行级收口（同 commit）**：manifest 删 :59-:62 行 + 增 service shim 行；matrix 删 :842-:846 行 + 增 shim 行。
- [ ] **Step 5 验证（例外登记前的编译白检——forbidden-import 此时预期 FAIL，属已知窗口，B2-DS.5 收口）**：

```bash
go build ./...     # 预期：退出码 0（module→module import 编译合法，guard 判定独立于编译）
go vet ./internal/modules/datasource/...   # 预期：退出码 0
go test -count=1 ./internal/modules/datasource/service/   # 预期：76 用例全 PASS（与 T1 基线逐用例一致）
go test -count=1 ./internal/application/service/ -run 'TestDeleteDataSourcePurge|TestDeleteDataSourceWithoutPurge|TestPurgeWorker|TestProcessDataSourcePurge|TestDataSourcePurgeQueueTopology|TestCountDataSourceDocumentsScopesToTenantKbDataSource' -v   # 预期：purge_test 9 顶层用例全 PASS（模式本分支实跑=9；R1 修订——原 3-alternates 模式漏 TestProcessDataSourcePurgeRejectsInvalidPayload/TestDataSourcePurgeQueueTopology/TestCountDataSourceDocumentsScopesToTenantKbDataSource 三例）
make check-backend-architecture 2>&1 | grep forbidden-import   # 预期：恰 3 条（§2.3 三对；B2-DS.5 登记后归零——此为登机的先决证据，非节点失败）
```

- **验收**：24 调用点全部改写，逐项 grep 对账（§2.2 表）：`grep -c "app.RecordKBActivity(" …/service/datasource_service.go` = 17、`grep -c "app.WithKBActivityTask("` = 2、`grep -c "app.KBActivityTrigger("` = 1、`grep -c "app.WithKBActivitySuppressed("` = 3、`grep -c "s.knowledgeCleanup("` = 1（seam 调用点）+ 残留检查 `grep -n "recordKBActivity(\|withKBActivityTask(\|kbActivityTrigger(\|withKBActivitySuppressed(\|withKnowledgeCleanup(" …/service/datasource_service.go` = 0；purge_test 断言零变化（diff 仅构造点 3 处 + f.ks 字段）。

### T4 — B2-DS.4 handler 搬迁 + 宿主 compat + 行级收口【commit: `refactor(datasource): passb B2-DS.4 handler 迁入模块与宿主 compat`】

- [ ] **Step 1**：`git mv internal/handler/datasource.go internal/handler/datasource_credentials.go internal/modules/datasource/handler/`；改写：删除 `internal/application/service` import，`service.ErrReindexDuplicateRequest`/`service.ErrSyncLogNotFound`（:525/:761）→ import `internal/modules/datasource/service` 同名引用；`internal/modules/datasource`（根包）import 若仅为既有类型则保留（同模块合法）；`internal/handler/dto` import 保留（platform 包豁免，check.go:89）。
- [ ] **Step 2**：`git mv` 4 个 handler 测试文件至 `internal/modules/datasource/handler/`；修 `service.ErrX` import（§2.4）。
- [ ] **Step 3**：落盘 §4.1(c) handler compat。
- [ ] **Step 4 行级收口（同 commit）**：manifest 删 :63-:70 两行 + 增 handler shim 行；matrix 删 :1808-:1818 两行 + 增 shim 行。
- [ ] **Step 5 验证**：

```bash
go build ./...                                                        # 预期：退出码 0
go test -count=1 ./internal/modules/datasource/...                    # 预期：全 ok（service 76 + handler 20 + 既有模块面）
go test -count=1 ./internal/handler/ -run 'TestDataSource' -v 2>&1 | grep -c "^=== RUN"   # 预期：0（datasource 20 用例已随迁；宿主 handler 包 TestDataSource 前缀仅本组使用，全量 grep 实测，零残留自证）
grep -rn "DataSourceHandler\|DataSourceCredentialsHandler" internal/router/ internal/container/ | grep -v "_test" | wc -l   # 预期：≥6（routes_infra:303-304、router.go:118-119/:408、container:760/:857——消费点仍指向 handler.* 别名，零改动自证）
```

- **验收**：宿主 `internal/handler/` 零 datasource 残留（`ls internal/handler/datasource*` 仅剩 compat 一件）。

### T5 — B2-DS.5 跨模块例外登记（3 对数据行 + ledger 3 行 + 计数 + 基线登记）【commit: `chore(passb): b2-datasource 登记跨模块 import 例外（数据行 + 台账）`】

- [ ] **Step 1**：`tools/architectureguard/check.go` importExceptions 追加 3 条（结构体字段照录 §2.3 表：`ImporterFile`×3 同值、`ImportedPath` 分别为 `github.com/Tencent/WeKnora/internal/modules/knowledge/retrieval/app` / `.../internal/modules/appconnector` / `.../internal/modules/policy/access`、`Reason: "预存横向包耦合（Pass A 前宿主文件→模块导入，26 号搬迁显形 module→module），Pass B 不改边界"`、`PassBTask: "B2-DS.5"`）——**仅数据行，禁改判定逻辑**（Ruling 原文）。
- [ ] **Step 2**：`exception-ledger.yaml` 末尾同序追加 3 行（§2.6 格式与 id 顺延规则；`from: internal/modules/datasource/service/datasource_service.go`、`to: <对应全路径>`、`plan: 26-datasource`、`remove_at: ib2`、reason 同上）。
- [ ] **Step 3 计数与基线登记**：ledger 行数断言基值（对齐后实测值 X）→ X+3 机械修正（若存在计数断言/台账数字，conventions §8；台账 `docs/architecture/evidence/pass-a-acceptance.md` 不改——B2 阶段登记进本节点 evidence §计数基线）；`make check-backend-architecture` 归零复证（见 Step 4）。
- [ ] **Step 4 验证**：

```bash
make check-backend-architecture   # 预期：total=633 | redis=23 lite=23 | hooks=58 | modules=16、OK (0 violations)（T3 Step 5 的 3 条 forbidden-import 全部被精确豁免吸收）
make verify-module-moves          # 预期：OK (16 manifests verified)
go run ./tools/architectureguard 2>&1 | tail -3   # 预期：无例外诊断新增
```

- **验收**：guard 双绿；diff ⊆ {check.go, exception-ledger.yaml, evidence}。

### T6 — B2-DS.6 别名 12 行成对删除（空义务核销）【commit: `refactor(passb): b2-datasource 删除 12 条别名义务行（manifest+matrix 同 commit）`】

- [ ] **Step 1 逐条零 importer 复检**：`python3` 读 `datasource.yaml` alias_obligations 12 条 old_import_path，对每条 `grep -rln "\"<path>\"" --include="*.go" . | grep -v _test.go || echo "(zero non-test importer)"`——预期 12 条全 zero；`ls internal/datasource` → No such file or directory（本分支已实测，复跑留档）。
- [ ] **Step 2 成对删行**：manifest alias_obligations 12 行（:30-:53）+ matrix aliases 区 12 行（:2547-:2583）同 commit 删除。
- [ ] **Step 3 验证**：`make verify-module-moves` OK（双侧奇偶，check.go:167-206 键为行集对照）；`make check-backend-architecture` 保持绿；`grep -c "internal/datasource" docs/architecture/moves/datasource.yaml` = 0。
- **验收**：12 条逐条结论（zero importer + 行删除）入 evidence §别名。

### T7 — B2-DS.7 差分复跑比对 + evidence 定稿【commit: `test(passb): b2-datasource 差分复跑比对与证据定稿`】

- [ ] **Step 1 新旧同用例双跑**（conventions §6 四要素：用例清单、双跑输出、比对结论、命令与退出码）：

```bash
# 新实现（模块包：repository 10 + service 76 + handler 20 + 既有模块面）
go test -count=1 ./internal/modules/datasource/... -v 2>&1 | tee /tmp/ds-new.txt
# 旧锚点（留守 purge_test——经 compat 装配运行同一实现，兼作 compat/seam 接线等价证据；模式=R1 修订后 9 顶层用例全模式，本分支实跑=9）
go test -count=1 ./internal/application/service/ -run 'TestDeleteDataSourcePurge|TestDeleteDataSourceWithoutPurge|TestPurgeWorker|TestProcessDataSourcePurge|TestDataSourcePurgeQueueTopology|TestCountDataSourceDocumentsScopesToTenantKbDataSource' -v 2>&1 | tee /tmp/ds-old-anchor.txt
```

- [ ] **Step 2 逐用例比对**：**115 用例** T1 基线清单 vs Step 1 结果——**PASS/FAIL/SKIP 逐用例一致**为通过判据（差分失败只能修正新实现，conventions §6；本节点为纯搬迁+机械改写，预期零偏差）。高风险两面的专门登记：
  - **knowledge 删除/purge 级联**（framework §14.3）：purge 9 用例（drain/墓碑/取消边界/孤儿 tag/绑定清理/兄弟行存活）——绑定清理断言（:355-:356）同时证明 `:885` seam 接线与迁移前 `withKnowledgeCleanup` 直引等价；
  - **Worker 状态机/取消/重试/幂等**（framework §14.3）：cancel_enqueue 6（硬取消顺序、降级、TaskID、force-full）+ sync_cancel 7（租户隔离、取消标志）+ sync_heartbeat 2 + stream 6；
  - **审计活动流**：purgeAuditSink.findByAction + service_test 内 audit 断言（17 直连改写点的行为面）。
- [ ] **Step 3 evidence 定稿**：`docs/architecture/evidence/passb/b2-datasource.md` 补 §差分（四面×四要素）、§计数基线（105→105+3 例外行登记；633/23+23/58/537 三方一致复核记录——`make check-backend-architecture` 输出 + manifest 发现值对照）、§别名（B2-DS.6 指针）。
- **验收**：差分四要素齐备；**115 用例**零偏差登记；任一偏差未归因即节点不完成。

### T8 — B2-DS.8 Integration Brief + 实施报告 + 节点门禁收口【commit: `docs(passb): b2-datasource Brief、报告与节点门禁收口`】

- [ ] **Step 1 Brief**（`docs/architecture/passb/briefs/b2-datasource.md`，交付 IB2）：
  - **(a) 装配直连切换申请**：container.go :309/:310/:633/:760/:857 五处 Provide 与 :2368 断言、routes_infra.go:301-306（形参 :303-304）与 router.go:118-119（RouterParams 字段）/:408（消费行）及 router_api_key_capabilities_test.go:353 的 `handler.*` 引用一并切到模块路径（`repository.`→`dsrepo.`、`service.NewDataSourceService`→`dsservice.NewDataSourceService`（**切换后必须补接 `SetKnowledgeCleanup`（见 (b)）或等待 K4 补迁窗直连**）、`handler.*`→`dshandler.*`）；切换后删 3 个 compat 文件 + manifest/matrix 3 shim 行（同 commit）。
  - **(b) cleanup seam 终局**：K4 补迁窗导出 `withKnowledgeCleanup`（24 计划 §5.3 蓝本）后，`:885` 改直连 `app.WithKnowledgeCleanup`（或 K 面裁定的最终门面位）+ 删 `SetKnowledgeCleanup`/字段 + 删 compat 内接线；若 IB2 先于补迁窗切换装配，IB2 须在 container 侧以等价闭包接线（与 Ruling CYCLE-FORCED-COMPOSITION 的 Brief 点名口径衔接）。
  - **(c) 3 条例外行收口编排**：remove_at=ib2；收口前置=knowledge 根门面暴露活动端口或经 ADR 修订（K2 Brief §7 同族裁定——app/knowledgebase.go→datasource 例外与本节点 datasource→app 例外在 ib2 一并裁决方向）。
  - **(d) 门面实装申请**：`datasource.facade` 五操作按 K5.1 同法实装（真实 seam：`RegisterWorkers` 双栈 2 类型=ProcessSync/ProcessDataSourcePurge 接口方法值；`Start`=startDataSourceScheduler 等价入口；`RegisterRoutes`=RegisterDataSourceRoutes 形参供给；Dependencies 字段从 container Invoke 面推导——IB2 时点实装，不在本节点）。
  - **(e) 留守件处置**：purge_test 最终随 K4 补迁窗处置（宿主 knowledgeService 消亡时其白盒不可编译，届时迁入模块并以 fake/公开构造重建——K4 Brief (f) 已登记对应义务）；`datasource.yaml`/matrix 剩余行状态快照。
  - **(f) 差分与计数证据指针** + worker/route/hook 奇偶声明（2/1/2 与 manifest integration_points 一致）。
- [ ] **Step 2 报告**（`docs/plans/passb/reports/b2-datasource.md`）：执行命令台账（原文+退出码）、变更清单 vs owned_files 逐条核对、§0.2.2 事实偏差登记（K4 推迟致 seam 方案）、未完成项如实列出。执行 conventions §1.2 报告包：

```bash
PASSB_BASE_SHA="$ALIGN_SHA"                                   # §0.1 P-2 裁定，禁用 merge-base origin/main 缺省公式
git diff --stat "$PASSB_BASE_SHA"...HEAD
git diff "$PASSB_BASE_SHA"...HEAD --name-only | sort          # 与 §3 写入所有权求差集，差集非空即失败
```

- [ ] **Step 3 节点 DAG gates 全量执行（不得以更快等价检查替代，conventions §2）**：

```bash
go build ./...                                            # 预期：退出码 0
go test -count=1 ./internal/modules/datasource/...        # 预期：全 PASS（repository 10 + service 76 + handler 20 + 既有 connector/scheduler/根包）
make check-backend-architecture                           # 预期：total=633 | redis=23 lite=23 | hooks=58 | modules=16、OK (0 violations)
make verify-module-moves                                  # 预期：OK (16 manifests verified)
```

- [ ] **Step 4**：DAG `b2-datasource` 建议置 `review`（回填归协调者，conventions §9）；审查者产出 `docs/plans/passb/reviews/b2-datasource.md`。
- **验收**：四 gates 绿（命令+退出码在报告）；差集为空；Brief (a)-(f) 齐备。

## 6. 高风险差分证据要求（conventions §6；framework §14.3 的 knowledge deletion + Worker 状态机两面）

**旧实现特征化 → 搬迁/改写 → 新实现同用例运行 → 逐用例等价比对**；legacy 删除（T2-T4 的旧路径删除）前差分必须已通过——本节点时序满足：T1 基线在搬迁前、T7 比对在全部删除后、T3 Step 5/T4 Step 5 的包级复跑为每个删除 commit 的即时门。四面登记（§5 T7 Step 2）+ 双跑输出与比对结论写入 evidence §差分。禁止改断言迎合新结果（conventions §5/§6）。

## 7. Integration Brief（交付 IB2 的装配变更申请，T8 定稿）

见 §5 T8 Step 1 (a)-(f)。核心：装配直连五处 + compat 3 文件删除 + seam 终局（K4 补迁窗联动）+ 例外 3 行收口 + 门面实装申请 + purge_test 终局。

## 8. 必须删除的 legacy/alias/例外（本节点口径）

| 项 | 动作 | 时点 | 依据 |
|---|---|---|---|
| `internal/application/repository/datasource_repo.go`（manifest :55-:58 + matrix :164-:168 行） | 物理迁移同 commit 删行 | B2-DS.2 | Ruling LEGACY-ROW-OWNERSHIP |
| `internal/application/service/datasource_service.go`（:59-:62 + :842-:846） | 同上 | B2-DS.3 | 同上 |
| `internal/handler/datasource.go` + `datasource_credentials.go`（:63-:70 + :1808-:1818） | 同上 | B2-DS.4 | 同上 |
| alias_obligations 12 行（manifest :30-:53）+ matrix aliases 12 行（:2547-:2583） | 成对删行（空义务核销） | B2-DS.6 | Ruling LEGACY-ROW-OWNERSHIP「可早删不可晚删」+ K5.2 先例 |
| `internal/datasource` 12 个旧路径 | 已物理不存在（Pass A 集成期），零重建 | — | §2.5 实测 |
| 3 个宿主 compat 文件 + 对应 manifest/matrix shim 行 | **本节点不删**（ib2 删除批，Brief (a) 登记；删除前置=装配直连切换） | ib2 | Ruling TRANSITION-SHIM-ROW-REGISTRATION |
| 3 条 importExceptions 数据行 + ledger 3 行 | **本节点不删**（remove_at=ib2；ib2 随知识门面/ADR 裁决收口，Brief (c)） | ib2 | Ruling IMPORT-EXCEPTION-REGISTRY |

**例外计数**：105（对齐后实值）→ +3（B2-DS.5，§8 基线登记）。**legacy 计数**：396 基线上 −4（迁移）+3（shim 补行）净 −1，随各任务同 commit 机械修正并入 evidence §计数基线登记（conventions §8 三方一致口径，禁静默改断言）。

## 9. 独立验收标准（全部满足才算完成；审查者逐条核验）

1. 四 DAG gates 命令原样执行且绿（§5 T8 Step 3；退出码在报告）。
2. `git diff "$ALIGN_SHA"...HEAD --name-only` 与 §3 写入所有权求差集为空（conventions §1.2）。
3. 4 个 legacy 文件物理落位 3 个 destination 包，`git diff --summary` 识别 rename，除 §4.2/§4.3/§5 T4 Step 1 列明的机械改写外函数体零变化（reviewer 抽查 ≥10 函数）。
4. 24 调用点对账：17+2+1+3 直连 `app.*` + 1 seam（`grep` 清单与 §2.2 表一致）；purge_test 9 用例经 compat+seam 全 PASS（seam 等价性证据）。
5. **115 用例** T1 基线 vs T7 终态逐用例一致（差分四要素在 evidence；其中 repository 10 用例经 T2 随迁后在新包内同名同断言复跑——R1 修订口径）。
6. manifest/matrix：4 legacy 行删、12 alias 行删、3 shim 行成对增——`make verify-module-moves` 绿（16 manifests）。
7. 3 条 importExceptions 数据行 + ledger 3 行（id 顺延、owner=26-datasource、remove_at=ib2）；`make check-backend-architecture` `OK (0 violations)` 且 633/23+23/58/16 不变。
8. 宿主零业务残留：`internal/application/service/datasource_service.go` 等 4 旧路径不存在；宿主仅余 3 compat + 留守 purge_test；`internal/handler/dto` 等 platform 导入不改。
9. Brief (a)-(f) 齐备且 (b) 与 K4 Brief (f) 的补迁窗义务对得上；报告含 §0.2.2 偏差登记与 ALIGN_SHA 来源。
10. 禁改清单零触碰（router/container/bootstrap/module.go/types/migrations/go.mod；check.go 仅数据行）。

## 10. 升级路径（conventions §5）

- 门禁不可能通过时：禁止改断言/删测试/扩例外/塞 common/复制实现/绕 guard/伪造证据；在报告登记未过项+根因+复现命令+建议裁定，DAG 置 blocked。
- **预期升级点 1**：P-3 复验发现 `withKnowledgeCleanup` 已被补迁导出（协调者提前执行 K4 补迁窗）→ §4.2.3 seam 仍合法（接线源改为导出包装），或经基线变更流程改直连（报告登记后择简者，不改行为）。
- **预期升级点 2**：P-2 对齐 merge 出现冻结签名取舍冲突 → 停下升级（conventions §10 Ruling 4）。
- **预期升级点 3**：留守 purge_test 在 K4 补迁窗时点（非本节点）宿主 knowledgeService 消亡而断链 → 属 K4 Brief (f) 已登记义务，本节点仅在 Brief (e) 复述，不代处置。
- 契约签名变化（contracts.yaml 四 datasource 契约）→ ADR/Spec 修订 + 串行契约任务（framework:26）。

## 11. 计划自检记录（撰写者已执行；R1 修订者复核并增补）

- **Spec 覆盖**：§5.9/§5.18/§6.2（不直接写 Knowledge 表——purge 经 `knowledgeService.DeleteKnowledgeList` 端口，:885 仅附上下文）→ §1/§2.2/§4.4；§12 Pass B 循环 → §5 T1→T7；§13 提交隔离 → 一任务一 commit + 同 commit 行级收口；§14.3 差分 → §6；§11 B2/IB2 → §0.2/§7。
- **无占位符/TBD**：全部文件路径、行号、签名为本分支 HEAD `74f454527` 实读（§0 头注声明对齐后复核点）；R1 修订所涉全部行号/清单于修订时分支 HEAD `b76d0fb8b` 重读复核；无发明接口——`app.*` 4 符号签名取自 20 计划 §6.2 组 B 冻结表 + 22 计划 :76-:80 导出面表 + k-process 分支 `retrieval/app/kb_activity.go` 实读；seam 签名逐字照录 `knowledge_delete_plan.go:22`。
- **类型一致**：compat wrapper 10 参签名与 `datasource_service.go:54-66` 逐参一致（struct :30-52 十个接口/指针字段一一对应；R1 勘误：原文「11 参」计数有误）；`SetKnowledgeCleanup` 与 seam 字段/调用点三方同签名；repo/handler 别名与定义处一致（`:19`/`:269`/`:25`/`:28`，R1 复验）。
- **跨任务接口一致**：T2 产出的 repo 10 用例随迁 = T1 repo 基线（10 顶层用例同模式）；T3 产出的 `SetKnowledgeCleanup` 被 T3 的 compat wrapper 消费；T5 的例外行 importer 路径=T3 落位路径；T8 Brief (a) 的切换点=T2/T3/T4 的 compat 覆盖面（§2.1 表，R1 补 router 三行）；T7 差分锚点=T1 基线清单（115 用例）。
- **R1 修订实测留痕**（2026-09-27，worktree 分支 HEAD `b76d0fb8b`）：①service 25-alternates 模式实跑 = 85 顶层用例、0 FAIL（36.1s）；②repo 模式实跑 = 10 用例 ok；③handler `TestDataSource` 模式实跑 = 20 顶层 + 9 子测试 ok；④purge 6-alternates 模式实跑 = 9 顶层；⑤K 分支反向边 grep 零命中 + retrieval/app 14 项清单（无 knowledgebase.go）+ exc-0112..0116 五条例外 ImportedPath 逐一读出；⑥§2.1 全集 sweep 命令与命中清单（见 §2.1 正文）。
- **事实偏差如实登记**：DAG notes「b2-k-integration 同时覆盖 K2 与 K4」vs K4 推迟现实 → §0.2.2 + §10 升级点 1；R1 勘误（原文不实引用/漏列）→ §0 头注 R1 修订条 + §2.3 无环论证更正 + §1/§2.4 repo 测试补全。
