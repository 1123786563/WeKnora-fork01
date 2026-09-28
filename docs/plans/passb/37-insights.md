# Pass B 子计划 37 — B3 Insights（6 legacy 文件：只读分析/评估边界迁入模块 + metric 别名义务收口）

> 实施方式：superpowers:executing-plans / subagent-driven-development，按任务逐个执行，一个任务一个 commit（conventions §4）。
> 节点：`b3-insights`（DAG `docs/plans/passb/execution-dag.json`，phase=B3，role=work，execution_mode=parallel，depends_on=`[ib2]`；可与 R1/R2、conversation application 层并行，framework:186）。
> 本计划由「计划撰写-b3-insights」于 2026-09-28 在 worktree `codex/passb-b3-insights` 撰写；撰写时点分支 HEAD `a2fbcf55e`（**已含 ib2 全部产物，`git merge-base --is-ancestor a2fbcf55e HEAD` 实测 yes**——P-2 基线对齐已由分支拓扑完成，无需再 merge）。文中全部代码坐标、符号签名、命令输出均为该树实读/实跑结果；实施时若 HEAD 前移，行号按当时树复核（本节点 6 个 legacy 文件不在任何兄弟 B3 节点 owned_files 内，预期稳定）。

## 0. Spec 与事实源指针

| 事实源 | 位置 | 本计划取用的内容 |
|---|---|---|
| Spec | `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md` §5.14（Insights：Analytics、Evaluation、运营指标、报表、可重建分析投影；**拥有评测定义/运行和投影，不拥有 Feedback、Session、Usage 等源记录，只消费事件或受控读模型**）、§5.18 覆盖矩阵（analytics、evaluation、metric/report/export projections → Insights 行）、§4.1（小模块不创建空目录）、§4.2 依赖方向、§6.5（Insights 投影可从权威事件重建）、§11（B3 并行 + IB3 串行）、§12（Pass B 循环：特征测试→…→高风险差分→装配切换→legacy 删除）、§13（M1–M5 提交隔离与回滚）、§14.3（高风险差分：Knowledge 删除面）、§14.4（blocked-env 语义）、§17.2 完成标准 | 边界语义、READ-ONLY 所有权、差分与提交纪律 |
| Pass B 框架 | `docs/plans/2026-09-23-backend-modularization-pass-b-framework.md` :183（37 号计划=6 legacy 文件、只读 analytics/evaluation 边界、metric 别名清理）、:186（Channels/Insights 可与 R1/R2、Conversation application 层并行）、:192（IB3 必测含 analytics 方言测试）、:25-33（全局约束：manifest 属主、模块 worker 不改 router/container、`_test.go` 随迁 :29、无双写/双注册 :30、高风险差分 :31、计数奇偶 :32） | 任务边界、并行依据、IB3 交接面 |
| 执行公约 | `.superpowers/sdd/passb/conventions.md`（主 checkout，git-ignored） | §1 派发契约（不派生子 Agent、只改 owned_files、§1.3 跨 owner 调用点禁直改）、§1.1 报告/审查/证据/Brief 路径、§1.2 报告命令、§2 门禁、§3 禁改清单（**module.go 门面注释仅 b0 与该模块集成节点可写**）、§4 提交规范、§5 升级契约、§6 差分证据四要素、§7 package-private 耦合、§8 计数基线 633/23+23/58/537、§10 裁定族（Ruling 1 LEGACY-ROW-OWNERSHIP / Ruling 3 TEST-SUPPORT-SHIM / Ruling 7 TRANSITION-SHIM-ROW-REGISTRATION） |
| MOVE-MANIFEST | `docs/architecture/moves/insights.yaml` | legacy_files 6 条（:11-:34，passb_task=B-insights）、alias_obligations 1 条（:8-:9 `internal/application/service/metric`）、move_packages 1 条（:5-:6，Pass A 已执行）、owned_files.module_files（README/module.go/legacy/README）、integration_points.routes 2 项（RegisterAnalyticsRoutes — internal/router/routes_analytics.go:15；RegisterEvaluationRoutes — internal/router/routes_infra.go:94）、workers/lifecycle_hooks 均 `[]`、test_commands `go test ./internal/modules/insights/... -count=1`（:48-:49）、forbidden_shared_files（:59-:66） |
| ownership-matrix | `docs/architecture/passb/ownership-matrix.yaml`（本分支副本，与 passb-int 同源） | insights 7 行：`internal/application/repository/analytics.go`（:98-:103）→ `internal/modules/insights/analytics`；`internal/application/service/dataset.go`（:782-:787）、`evaluation.go`（:812-:817）、`metric_hook.go`（:1034-:1039）→ `internal/modules/insights/evaluation`；`internal/handler/analytics.go`（:1478-:1483）→ `internal/modules/insights/analytics`；`internal/handler/evaluation.go`（:1556-:1561）→ `internal/modules/insights/evaluation`；六行均 `plan: 37-insights, integration_owner: ib3, delete_barrier: ib3`；aliases 区 1 行（:2268-:2270 `old_import_path: internal/application/service/metric, delete_barrier: ib3`） | 精确 destination、行级删除权（Ruling 1）、别名删除义务 |
| contracts.yaml | 同目录 `contracts.yaml` :1555-:1599 | `insights.facade`（:1555 items=NewModule/RegisterRoutes/RegisterWorkers/Start/Stop，symbol=internal/modules/insights/module.go，stability=frozen）、`insights.routes`（:1578 consumers=routes_analytics.go+routes_infra.go，items=RegisterAnalyticsRoutes/RegisterEvaluationRoutes）、`insights.workers`/`insights.lifecycle`（items=[]）——四契约本节点**只读不改**；另：`internal/application/service/evaluation.go` 登记为 `airesource.model-service`（:358）与 `conversation.session-service`（:967）的消费方，迁移后该 consumer 行的路径更新属 barrier 回写范畴（§5 Brief (e)） |
| event-catalog | 同目录 `event-catalog.yaml` | `grep -i insights` 实测零命中——本节点无版本化事件面改动，required_contracts 中「event-catalog 投影事件」对本节点为只读知悉项（Workbench/Insights projections 的具体事件版本由 B0.5 冻结记录，本模块当前无事件生产/消费代码） |
| exception-ledger | 同目录 `exception-ledger.yaml` | 本分支 142 行（`grep -c "id: exc-"` 实测），`grep -n "37-insights"` 零命中——**本节点无属主例外行，且预计零新增**（§2.6 论证） |
| DAG 裁定 | `execution-dag.json` 节点 `b3-insights` | owned_files=`manifest:docs/architecture/moves/insights.yaml legacy_files 6 条` + `internal/modules/insights/**`；gates 四条（`go build ./...` / `go test ./internal/modules/insights/...` / `make check-backend-architecture` / `make verify-module-moves`）；notes：analytics 方言测试属 IB3 必测（framework:192）；未导出耦合 insights→knowledge `deleteReferencedKnowledge`（knowledge_delete_plan.go:36，package_private_couplings） |
| Knowledge 程序冻结产物 | `docs/plans/passb/20-knowledge-program.md` :193、`24-knowledge-process.md` :84/:119 | `knowledge_delete_plan.go` 属 K4 推迟批（根因类 A：7 个他属主宿主白盒测试构造 `&knowledgeService{}`）；`deleteReferencedKnowledge` **顺延符号**（本分支实测 :36 仍为未导出 `func deleteReferencedKnowledge(ctx context.Context, svc interfaces.KnowledgeService, expectedKB string, ids []string) error`，留守宿主 `internal/application/service`），导出义务登记于 K4 Integration Brief「推迟批解除窗口执行」——DAG notes「K4 已导出」与实测存在事实偏差，按 conventions §5 如实登记（§0.2.2），不构成阻塞（§2.3 seam 方案对导出前后两时点行为等价） |
| 先例计划 | `docs/plans/passb/26-datasource.md`（compat shim + cleanup seam 接线 + 治理行成对操作全流程先例）、`27-appconnector.md`（别名空义务核销）、`13-execution.md` :7/:61（module.go 门面实现「集成工程师独占，本节点交付 Brief 由 barrier 切换」原文先例，本分支实读） | 格式与裁定先例 |

### 0.1 前置条件（开工前逐条核验；任一不满足按 conventions §5 回 BLOCKED）

1. **P-1 前置节点终态**：DAG `ib2` `status=done`（撰写时点已确认：ib2 head=`a2fbcf55e`，2026-09-28 10:28 收口）；以开工时点 DAG 实测为准（notes 残留 BLOCKED 文本为历史登记，不构成停工依据）。
2. **P-2 基线对齐**（Ruling 2026-09-24-WAVE-DEP-BASELINE）：`git merge-base --is-ancestor a2fbcf55e HEAD` 为 yes（撰写时点实测成立，本分支即 ib2 谱系直系）；若实施时 ib3 前又有新的集成前进，按 Ruling 4 对齐后再开工。**ALIGN_SHA**：无额外 merge 时 = 本分支首个任务 commit 的父提交（即开工 HEAD），登记进报告，作为 §1.2 diff 检查的 `PASSB_BASE_SHA` 采用值。
3. **P-3 ib2 冻结门面在位**（required_contracts 自核）：`ls internal/modules/knowledge/{ingest,retrieval,process,wiki}` 四目录存在；`grep -c "^func RecordKBActivity" internal/modules/knowledge/retrieval/app/kb_activity.go` ≥1（K2 导出面就绪）；`grep -n "^func deleteReferencedKnowledge" internal/application/service/knowledge_delete_plan.go` 命中 :36（K4 推迟件留守——§2.3 seam 裁定前提）。
4. **P-4 门禁工具可用**：`Makefile:250 verify-module-moves`、`:255 check-backend-architecture` 两目标在位（本分支实读确认；`check-passb-readiness` 为 barrier 门禁，本节点不跑）。
5. **P-5 worktree 干净**：`git status` 干净，分支 `codex/passb-b3-insights`。
6. **P-6 基线门禁快照**（开工 HEAD 实跑留档，后续差分对照；以下为本会话实测值）：
   - `go build ./...` 退出码 0（cmd/desktop、cmd/server 各输出一条 `ld: warning: ignoring duplicate libraries: '-lc++'`，既有噪声）；
   - `make verify-module-moves` → `modulemove: OK (16 manifests verified)`；
   - `make check-backend-architecture` → `architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16` + `architectureguard: OK (0 violations)`；
   - `go test -count=1 ./internal/modules/insights/...` → `?  .../internal/modules/insights [no test files]` + `ok  .../internal/modules/insights/metric 0.585s`；
   - `go test ./internal/application/repository -run TestAnalyticsAggregations -count=1` → `ok  .../internal/application/repository 4.159s`；
   - `go test ./internal/handler -run TestAnalyticsHandler -count=1` → `ok  .../internal/handler 0.575s`。

### 0.2 节点裁定与 BLOCKED 链（调度方原文要旨）

1. **并行裁定**：可与 R1/R2、conversation application 层并行（framework:186）——本节点 6 文件与 R/conv 节点 owned_files 不相交，§2.5 无环论证。
2. **DAG notes 事实偏差登记**（conventions §5，非阻塞）：notes 称「K4 已导出」，实测 `deleteReferencedKnowledge` 仍未导出、留守宿主（P-3 第 3 条判据；K4 按 Ruling 2026-09-25-DEFERRED-FILE-SPLIT 推迟）。本计划采用**消费侧 seam + 宿主 compat 接线**（§2.3，datasource 同型先例：`internal/application/service/datasource_passb_compat.go` 把模块 cleanup seam 接到宿主现行 `withKnowledgeCleanup`）；seam 对「K4 补迁窗已导出」与「未导出」两种时点行为等价（接线源不同而已，§5 Brief (b)）。报告中如实登记此偏差。
3. **IB3 必测预告**：analytics 方言测试（sqlite/postgres 双方言，framework:192）——本节点把 `TestAnalyticsAggregations` 双方言随迁并在证据中落盘 sqlite 实跑 + postgres env 缺省 skip 记录（blocked-env 语义，Spec §14.4），IB3 复跑（§5 Brief (d)）。
4. BLOCKED 历史链（09-23 b0 → 09-24 b2-k0 → 09-25 ac-market/k-retrieval/k-process → 09-26 k-process/k-integration → 09-27 k-integration → 09-28 ib2）：各前置 done 后逐次恢复；以 DAG `status` 字段为准。

---

## 1. 目标与范围

**目标**：把困在水平宿主包的 6 个 insights legacy 文件迁入冻结 destination（`internal/modules/insights/analytics`、`internal/modules/insights/evaluation`），evaluation 的 K4 推迟符号 `deleteReferencedKnowledge` 以消费侧 seam + 宿主 compat 接线保持行为零变化，宿主装配面（container/router/usage.go）经 3 个过渡 compat 文件零改动编译，analytics 双方言聚合与 handler 用例组差分等价证据落盘，metric 别名义务行收口，节点四门禁绿色。

**范围内**（owned_files = manifest legacy_files 6 条 + `internal/modules/insights/**` + 本节点自身产出新文件；**例外：`internal/modules/insights/module.go` 按 conventions §3 仅 b0 与本模块集成节点（ib3）可写，本节点零触碰**——13-execution.md:7/:61「门面实现一概不动（集成工程师独占）」原文先例）：

| # | legacy 文件 | destination（ownership-matrix 冻结） | 体量（本分支实测） | 关键符号 |
|---|---|---|---|---|
| 1 | `internal/application/repository/analytics.go` | `internal/modules/insights/analytics/analytics_repository.go`（package analytics） | 148 行 | `analyticsRepository`（:16）、`NewAnalyticsRepository`（:21）、`dayExpr`（:27）、`countExpr`（:37）、`analyticsSourceExpr`（:49）、四聚合方法 `QueryTrend`/`ActiveUsers`/`ChannelSessions`/`AgentMessages`（:63/:94/:113/:132） |
| 2 | `internal/handler/analytics.go` | `internal/modules/insights/analytics/analytics_handler.go`（package analytics） | 188 行 | `analyticsDefaultLookbackDays`（:16）、`AnalyticsHandler`（:21-:23）、`NewAnalyticsHandler`（:26）、`parseAnalyticsRange`（:40）、`analyticsIsDateOnly`（:68）、`analyticsRows`（:90）、`tenantID`（:99）、四 HTTP 方法 |
| 3 | `internal/application/service/dataset.go` | `internal/modules/insights/evaluation/dataset.go`（package evaluation） | 243 行 | `DatasetService`（:15）、`NewDatasetService`（:18）、`TextInfo`/`RelsInfo`/`QaInfo`（:23/:29/:35）、`DefaultDataset`（:54）、`dataset`（:103）、`loadParquet`（:237）；运行时读 `./dataset/samples/*.parquet`（相对 CWD，随服务启动目录不变，零改动） |
| 4 | `internal/application/service/evaluation.go` | `internal/modules/insights/evaluation/evaluation.go`（package evaluation） | 479 行 | `EvaluationService`（:28-:37）、`NewEvaluationService`（:39-:57）、`evaluationMemoryStorage`（:60-:100）、`EvaluationResult`（:102）、`Evaluation`（:133）、`EvalDataset`（:333）、`getPassageList`（:463）；`deleteReferencedKnowledge` 调用点 :366-:369 |
| 5 | `internal/application/service/metric_hook.go` | `internal/modules/insights/evaluation/metric_hook.go`（package evaluation） | 194 行 | `MetricList`（:15）、`metricCalculators` 表（:20-:51）、`Append`（:54）、`Avg`（:66）、`HookMetric`（:86）、`NewHookMetric`（:101）、`recordFinish`（:135-:187） |
| 6 | `internal/handler/evaluation.go` | `internal/modules/insights/evaluation/evaluation_handler.go`（package evaluation） | 131 行 | `EvaluationHandler`（:15-:17）、`NewEvaluationHandler`（:20）、`EvaluationRequest`（:25-:30）、`Evaluation`（:44）、`GetEvaluationRequest`（:91）、`GetEvaluationResult`（:107） |

> 文件落名说明：`analytics.go` 两文件同迁一个包需改名（repository 侧 → `analytics_repository.go`、handler 侧 → `analytics_handler.go`）；evaluation 面 4 文件原名保留（`git mv` 后 `git mv` 二段式或 `git mv a b && git mv` 链均可，rename 检测以内容相似度成立；datasource 先例同型）。**包名取 `analytics` / `evaluation`**——`grep -rln "^package analytics\|^package evaluation" internal/` 实测零冲突。

随迁测试（framework:29；`internal/modules/insights/legacy/README.md` 口径「同目录 `_test.go` 随主题文件一并搬移」）：

| 测试文件 | 随迁至 | 内容 |
|---|---|---|
| `internal/application/repository/analytics_test.go`（131 行） | `internal/modules/insights/analytics/analytics_repository_test.go` | `TestAnalyticsAggregations`（:79-:131，sqlite/postgres 双方言子测试；`seedAnalyticsData` :18-:74）——依赖宿主共享测试装置 `openRunTestDB`（定义 `internal/application/repository/agent_run_test.go:29`，含 :82 `seedRunFixtures`、:87 `openPostgresRunTestDB`），**模块侧自带副本**（§2.4，agentcatalog 先例） |
| `internal/handler/analytics_test.go`（284 行） | `internal/modules/insights/analytics/analytics_handler_test.go` | `TestAnalyticsHandler_*` 10 个顶层用例（`grep -c "^func TestAnalyticsHandler_"` 实测 10；brief 计「11 例」为含子测试口径）：默认 30 天窗 :97、显式 range 透传 :126、非法时间参数 400 :141（2 子测试）、tenant 缺失 403 :166、agent 透传 :179、date-only end 全天覆盖 :205、带时刻 end 不加一天 :219、空行渲染 `[]` :233（4 子测试）、空 agent_id 400 :256、repo 错误 500 :270——自包含（stub repo + gin + middleware.ErrorHandler），零外部装置 |

**留守宿主（零）**：6 文件全部迁出，无 K4 式推迟件（本节点文件无 `&knowledgeService{}` 型他属主白盒测试依赖——`grep -rln "EvaluationService\|DatasetService\|HookMetric\|MetricList\|AnalyticsRepository\|AnalyticsHandler" internal/` 实测命中仅：6 legacy 文件本体、container.go、router 三文件、router_api_key_capabilities_test.go:344、types/interfaces 两契约文件；其中 container/router 消费面经 §2.2 compat 覆盖）。

**范围外**：`internal/modules/insights/metric/`（Pass A 已就位，零改动，仅被 evaluation 包同模块 import）；`internal/types/interfaces/{analytics,evaluation}.go`（契约类型属 platform 侧 `internal/types`，非本节点 owned_files）；`internal/router/routes_analytics.go`、`routes_infra.go`、`router.go`、`internal/container/container.go`（禁改/集成工程师独占，经 compat 零改动编译）；`internal/handler/usage.go`（commercial 属主 12-commercial legacy，本节点禁改——§2.3(c) 转发声明覆盖其编译）。

---

## 2. 现状与消费面分析（全部为本分支实读）

### 2.1 宿主消费面全集（迁移后必须仍编译的调用方）

| 消费点 | 位置 | 消费符号 | 迁移后覆盖方式 |
|---|---|---|---|
| dig Provide | `internal/container/container.go:217` | `repository.NewAnalyticsRepository` | repository 侧 compat wrapper（§4 B3-IN.3 Step 4） |
| dig Provide | `internal/container/container.go:388` | `service.NewDatasetService` | service 侧 compat wrapper（B3-IN.4） |
| dig Provide | `internal/container/container.go:389` | `service.NewEvaluationService`（6 参） | service 侧 compat wrapper（6 参原签名 → 模块 7 参构造器 + seam 接线，B3-IN.4） |
| dig Provide | `internal/container/container.go:746` | `handler.NewAnalyticsHandler` | handler 侧 compat wrapper（B3-IN.3） |
| dig Provide | `internal/container/container.go:761` | `handler.NewEvaluationHandler` | handler 侧 compat wrapper（B3-IN.4） |
| 路由参数 | `internal/router/router.go:74`（`params.AnalyticsHandler *handler.AnalyticsHandler`）、`:379` 挂载 | `*handler.AnalyticsHandler` 具体类型 | handler 侧 compat **type 别名** `type AnalyticsHandler = analytics.AnalyticsHandler`（真别名，方法集随型，路由文件零改动） |
| 路由参数 | `internal/router/router.go:82`、`:384` 挂载 | `*handler.EvaluationHandler` 具体类型 | 同上 `type EvaluationHandler = evaluation.EvaluationHandler` |
| 路由测试 | `internal/router/router_api_key_capabilities_test.go:344` `RegisterEvaluationRoutes(v1, &handler.EvaluationHandler{}, g)`；`:369` 断言 `POST /api/v1/evaluation → APIKeyCapabilityRunEvaluations`（属 `TestTenantInfrastructureRoutesDeclareSpecificCapabilities` :337） | `&handler.EvaluationHandler{}` 空字面量 | type 别名下空字面量合法（字段零值）；**迁移后该测试必须仍绿**（capability 奇偶差分，B3-IN.4 Step 8） |
| 同包宿主消费（**跨 owner**） | `internal/handler/usage.go` 共 5 个调用点：`:89/:114/:138` 调 `parseAnalyticsRange(c)`，`:107/:128` 调 `analyticsRows(data)`（usage.go 属 commercial，manifest `commercial.yaml:63`，12-commercial legacy 仍在宿主；`TestUsageHandler_*` 9 个顶层用例经转发链回归） | `parseAnalyticsRange`、`analyticsRows`（定义于本节点 analytics.go :40/:90） | handler 侧 compat **转发函数**（§2.3(c)）；usage_test.go :118/:270 仅注释引用，无调用 |

### 2.2 冻结契约面（本节点只读）

- `insights.routes`（contracts.yaml :1578-:1590）：`RegisterAnalyticsRoutes — internal/router/routes_analytics.go:15`（Admin+ + `apiKeyFullAccess`，`GET /analytics/{queries,users,channels,agents/:agent_id}`）与 `RegisterEvaluationRoutes — internal/router/routes_infra.go:94-:101`（`POST /evaluation` Admin、`GET /evaluation` Viewer，`apiKeyRunEvaluations(apiKeyFullAccess())`，rbac.go:319）——两注册函数在 platform 领地 `internal/router`，本节点零触碰，路由面经 type 别名零变化。
- `insights.facade`（:1555）：NewModule/RegisterRoutes/RegisterWorkers/Start/Stop——由 ib3 按 Brief (a) 落地。
- `internal/types/interfaces/analytics.go:36-:46` `AnalyticsRepository` 四方法与 `:7-:34` 四 Point 类型、`internal/types/interfaces/evaluation.go:10-:17` `EvaluationService`、`:19-:23` `Metrics`、`:32-:35` `DatasetService`——契约类型不动，模块实现继续满足。
- `airesource.model-service` / `conversation.session-service` 消费：`evaluation.go` 仅经 `interfaces.ModelService`/`interfaces.SessionService` 窄接口消费（contracts.yaml :358/:967 登记），迁移不改变依赖方向（模块→interfaces，非模块→模块内部）。

### 2.3 跨 owner 未导出耦合处置（本节点 3 项，全部有先例）

**(a) insights→knowledge `deleteReferencedKnowledge`（DAG ppc 登记项）**
定义：`internal/application/service/knowledge_delete_plan.go:36-:41`，未导出，K4 推迟批留守（§0 事实源表）。调用点（本节点唯一）：`evaluation.go:366-:369`，语义=评估临时知识库的 cleanup（defer 内删除 `knowledge.ID`、绑定 `knowledgeBaseID`）——**Spec §14.3 高风险面「Knowledge 删除」**。
处置（conventions §7.1「定义方搬迁时导出窄端口或留薄 shim」的消费侧镜像 + datasource 先例）：
1. 模块 `internal/modules/insights/evaluation` 新增 seam 类型（真实签名为准）：
   ```go
   // DeleteReferencedKnowledgeFunc 镜像宿主清理助手（internal/application/service/
   // knowledge_delete_plan.go:36，K4 推迟批未导出）。由宿主 compat 构造器注入；
   // 模块不 import 宿主包。K4 补迁窗导出后，Brief (b) 切换接线源，seam 形态不变。
   type DeleteReferencedKnowledgeFunc func(ctx context.Context, svc interfaces.KnowledgeService,
       expectedKB string, ids []string) error
   ```
2. `EvaluationService` 增字段 `deleteReferencedKnowledge DeleteReferencedKnowledgeFunc`；模块构造器 `NewEvaluationService` 增第 7 参 `cleanup DeleteReferencedKnowledgeFunc`（存前 6 参原序原型）；`evaluation.go:366` 调用点改为 `e.deleteReferencedKnowledge(ctx, e.knowledgeService, knowledgeBaseID, []string{knowledge.ID})`（实参序不变）。
3. 宿主 compat `internal/application/service/insights_passb_compat.go` 内 6 参 `NewEvaluationService`（原签名，dig 兼容）转发模块构造器并传宿主裸标识符 `deleteReferencedKnowledge`（同包可见，函数值到具名函数类型隐式可赋值）——**与迁移前同包直引同一目标函数，行为零变化**（datasource_passb_compat.go 头注同型论证）。

**(b) analytics handler→`parseFilterTime`（宿主 `internal/handler` 未导出，knowledge.go:2565-:2577，K4 legacy 留守）**
调用点：`handler/analytics.go:45/:53`（`parseAnalyticsRange` 内）。处置：**模块本地副本**（`internal/modules/insights/analytics/analytics_handler.go` 内 `parseFilterTime`，函数体逐字同 knowledge.go:2566-:2577：TrimSpace → 空 returning zero → `[]string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"}` 逐 layout `time.ParseInLocation(layout, raw, time.Local)` → 返回 lastErr）。先例：`internal/handler/session/handler.go:427-:437` `parseSessionFilterTime`（注释原文「accepts the same layouts as the knowledge list filters (parseFilterTime in the parent handler package; duplicated here because that package imports this one)」）——本仓对该 layout 族已确立**属主本地副本**惯例（另见 airesource/web_search 各 vendor 副本）。行为由 `TestAnalyticsHandler_ExplicitRangePassthrough`（`time.Local` 语义 :135/:198）与 `TestAnalyticsHandler_InvalidTimeParams`（400 文案含参数名）钉住；副本头注标明 parity 依据。**不构成「复制他模块实现」违规**：解析 layout 是 analytics 查询串自身 API 契约的组成部分（brief 契约节「显式 range 透传」），非 knowledge 业务规则；§5 升级通道保留（若审查裁定应收敛为共享原语，走 coordinator 重裁）。

**(c) analytics.go 迁出后宿主留守文件 `usage.go` 断链（commercial 属主，conventions §1.3 禁直改）**
处置：handler 侧 compat 文件保留两个**转发声明**（零逻辑）：
```go
func parseAnalyticsRange(c *gin.Context) (time.Time, time.Time, bool) { return analytics.ParseAnalyticsRange(c) }
func analyticsRows[T any](rows []T) []T                               { return analytics.Rows(rows) }
```
为此模块侧须**导出** `ParseAnalyticsRange`（原 `parseAnalyticsRange` :40，仅可见性变化）与 `Rows`（原 `analyticsRows` :90 改名导出——泛型标识符改名为避免 `analytics.analyticsRows` 冗余前缀，调用点 `analytics_handler.go` 内 4 处同步）；`analyticsIsDateOnly`/`analyticsDefaultLookbackDays` 保持未导出（仅包内消费）。ib3 终局：Brief (c) 请集成工程师把 `usage.go` 4 个调用点直改 `analytics.ParseAnalyticsRange`/`analytics.Rows`（§1.3 barrier 执行调用点修改的机制内动作）后删 compat——或按 commercial 侧 usage.go 迁移时点衔接（协调者裁定，Brief 两案并列）。

### 2.4 测试资产与差分口径

| 类别 | 内容 |
|---|---|
| 随迁既有 | `TestAnalyticsAggregations`（双方言聚合契约）、`TestAnalyticsHandler_*`（10 个顶层用例 HTTP 契约）——**即差分用例集**：P-6 基线（宿主位实跑 ok）vs 迁移后模块位同用例实跑，逐用例等价（conventions §6 四要素落 evidence） |
| 装置副本 | 模块 `analytics_repository_test.go` 自带 `openRunTestDB`/`seedRunFixtures`/`openPostgresRunTestDB` 副本（源：`agent_run_test.go:29-:125`；**含 postgres 分支**——agentcatalog 副本省略 postgres 是因其用例无 `/postgres` 子测试，本模块相反，`TestAnalyticsAggregations` :80 双方言必测）；repoRoot 深度 `../../../..`（agent_marketplace_test.go:376 同深度先例）；文件头注 Ruling 2026-09-24-TEST-SUPPORT-SHIM 家族 + remove_at: ib3 后首次全量复核（宿主原件随 agent_run 系留守，副本无独立删除义务，随模块存续） |
| 新写特征化（RED 先行于迁移） | `internal/application/service/evaluation_characterization_test.go`（宿主位，B3-IN.2）：① `getPassageList` 黄金用例（PIDs 稀疏映射与空槽零值）；② `MetricList.Append/Avg`（同输入两次 Append 的 Avg == 单次值；空表 Avg 返回零值 `&types.MetricResult{}`；受控检索输入 Precision=1.0/Recall=0.5）；③ `HookMetric.recordFinish` 内容回映射（检索 chunk Content 命中 passage 子串 → 映射回该 passage PID）。B3-IN.4 随 evaluation.go 同迁 |
| 新写 seam 钉住（迁移后模块位） | `internal/modules/insights/evaluation/evaluation_cleanup_test.go`（B3-IN.4）：stub `DatasetService`/`KnowledgeService`（`CreateKnowledgeFromPassageSync` 返回固定 `&types.Knowledge{ID:"k-eval-1"}`）/`KnowledgeBaseService`（捕获 `DeleteKnowledgeBase` 实参）/`SessionService`（`KnowledgeQAByEvent` 返回 nil）注入 `NewEvaluationService`，seam 传捕获函数；调 `EvalDataset` 后断言 seam 收到 `("kb-eval", ["k-eval-1"])` 且 `DeleteKnowledgeBase("kb-eval")` 被调——钉住 **Knowledge 删除高风险面**的 seam 语义 |
| 差分等价论证（构造性） | 迁移 commit 函数体零变化（除 §2.3 三处列明点：seam 字段/调用、导出改名、本地副本）；compat/shim 均纯转发；`git diff` 复核无行为面改动 |

### 2.5 无环与守卫论证

- 模块新包 import：`analytics` = stdlib + gin + gorm + `internal/errors`/`internal/logger`/`internal/types`/`internal/types/interfaces`（非 `internal/modules/*`）；`evaluation` = stdlib + errgroup + parquet-go + `internal/config`/`internal/logger`/`internal/utils`/`internal/types`/`internal/types/interfaces` + **`internal/modules/insights/metric`（同模块，guard 判定 owner==target 放行）**。→ **零新增 module→module import，零新增 importExceptions 数据行**（forbidden-import 判定域仅 `internal/modules/` 内文件，check.go:1446-:1473 实读；宿主 compat 文件 import 模块不在判定域——datasource/span_trace 先例已确立）。
- 宿主 `internal/application/service` import `internal/modules/insights/metric`（metric_hook.go:9 现状）与 compat import `internal/modules/insights/{analytics,evaluation}` 并存——模块侧零反向 import 宿主（seam 注入代替），无环。
- 兼容别名单源：compat wrapper/别名只委托模块真源，无第二份实现（Spec §13）。

### 2.6 治理行操作汇总（Ruling 1 + Ruling 7 成对纪律）

| 文件 | 操作 | 任务 |
|---|---|---|
| `docs/architecture/moves/insights.yaml` | legacy_files 删 2 行（analytics 面）+ 增 2 行 shim（reason「Pass B 过渡 shim，ib3 同 commit 随文件删行」） | B3-IN.3 |
| 同上 | legacy_files 再删 4 行（evaluation 面）+ 增 1 行 shim；alias_obligations 清空（`[]`）；move_packages 清空（`[]`，Pass A 义务已物理完成）；owned_files.move_sources/move_targets 清空（`[]`）；importers 更新为三 shim 宿主目录 | B3-IN.4/:5 |
| `docs/architecture/passb/ownership-matrix.yaml` | 与 manifest 逐行镜像：6 legacy 行删（同物理迁移 commit，Ruling 1「可早删不可晚删」）、3 shim 行增（Ruling 7）、1 alias 行删（:2268-:2270；物理删除已于 IA4 commit `4d12ba36e` 完成，本操作为行级簿记收口） | B3-IN.3/:4/:5 |
| `docs/architecture/passb/exception-ledger.yaml` | 零属主行、零新增（§2.5 论证）；B3-IN.5 实测复核 `grep -n "37-insights"` 零命中并在报告登记 | B3-IN.5 |
| `internal/modules/insights/legacy/README.md` | 6 行标 **已迁移**（B3-IN.x → 目的包）+ 3 行 shim 补登（datasource legacy/README 同型格式） | B3-IN.6 |
| `internal/modules/insights/README.md` | 搬迁状态回填（legacy 清零、analytics/evaluation 两包就位、compat 过渡登记、验收命令不变） | B3-IN.6 |

---

## 3. 写入清单（owned_files 逐条对照）

| 写入 | 路径 | 依据 |
|---|---|---|
| 迁移（git mv） | §1 表 6 文件 + 2 测试文件 + B3-IN.2 特征化测试（→ evaluation 包） | manifest legacy_files 6 条 + framework:29 |
| 新建（模块） | `internal/modules/insights/analytics/analytics_repository.go`、`analytics_handler.go`、`analytics_repository_test.go`、`analytics_handler_test.go`；`internal/modules/insights/evaluation/{evaluation.go,dataset.go,metric_hook.go,evaluation_handler.go,evaluation_characterization_test.go,evaluation_cleanup_test.go}` | DAG owned_files `internal/modules/insights/**` |
| 新建（宿主过渡 shim ×3） | `internal/application/repository/insights_passb_compat.go`、`internal/application/service/insights_passb_compat.go`、`internal/handler/insights_passb_compat.go` | Ruling 7 + §2.1/§2.3 消费面 |
| 修改（治理） | `docs/architecture/moves/insights.yaml`、`docs/architecture/passb/ownership-matrix.yaml` | Ruling 1/7 行级权限 |
| 修改（模块文档） | `internal/modules/insights/README.md`、`internal/modules/insights/legacy/README.md` | manifest module_files |
| 新建（报告包） | `docs/architecture/evidence/passb/b3-insights.md`、`docs/plans/passb/reports/b3-insights.md`、`docs/architecture/passb/briefs/b3-insights.md` | conventions §1.1 |
| **零触碰** | `internal/modules/insights/module.go`（§1 裁定）、`internal/modules/insights/metric/**`、`internal/router/**`、`internal/container/**`、`internal/handler/usage.go`、`internal/handler/knowledge.go`、`internal/application/service/knowledge_delete_plan.go`、`go.mod`/`go.sum`、`migrations/`、`tools/architectureguard/check.go`（零新例外行） | conventions §3 + §2.3 |

---

## 4. 实施任务（一个任务一个 commit，conventions §4）

### B3-IN.1 前置核验与基线快照（docs/test only）

**目标**：P-1..P-6 逐条核验留档；evidence 骨架落盘。

步骤：
1. 逐条执行 §0.1 P-1..P-5 命令并记录输出（P-2：`git merge-base --is-ancestor a2fbcf55e HEAD && echo aligned`；预期 `aligned`）。
2. P-6 六条命令实跑，输出原样摘录进 evidence 骨架。
3. 创建 `docs/architecture/evidence/passb/b3-insights.md`：骨架含「基线快照 / analytics 差分 / evaluation 特征化与 seam / 治理行操作 / 门禁记录」五节，本任务填基线节。

命令与预期（本会话实测值，复跑以当时树为准）：
```bash
go build ./...                                              # 退出码 0（ld 警告为既有噪声）
make verify-module-moves                                    # modulemove: OK (16 manifests verified)
make check-backend-architecture                             # total=633 | redis=23 lite=23 | hooks=58 | modules=16；OK (0 violations)
go test -count=1 ./internal/modules/insights/...            # metric ok（模块根 no test files）
go test ./internal/application/repository -run TestAnalyticsAggregations -count=1   # ok（sqlite 跑、postgres skip：TRPC_TEST_POSTGRES_DSN unset）
go test ./internal/handler -run TestAnalyticsHandler -count=1                        # ok
```

commit：`test(passb): b3-insights baseline snapshot and evidence skeleton`

### B3-IN.2 evaluation 面特征化测试（RED→GREEN 于宿主位）

**目标**：为无测试覆盖的 evaluation/dataset/metric_hook 纯逻辑与内容回映射写特征化测试（conventions §1.4「搬迁类任务先写特征化测试」），在**旧实现**上跑绿，随 B3-IN.4 迁移。

步骤：
1. 新建 `internal/application/service/evaluation_characterization_test.go`（package service，与被测同包——`getPassageList`/`recordFinish` 为未导出符号）：
   - `TestGetPassageListGolden`：`[]*types.QAPair{{PIDs: []int{2, 0}, Passages: []string{"p2", "p0"}}}` → 断言 `getPassageList` 返回 `[]string{"p0", "", "p2"}`（maxPID=2、槽 1 零值）；
   - `TestMetricListAppendAvg`：构造 `MetricInput{RetrievalGT: [][]int{{1, 2}}, RetrievalIDs: []int{1}, GeneratedGT: "answer", GeneratedTexts: "answer"}`，`Append` 两次同输入后 `Avg()` 与单次 `Append` 的 `RetrievalMetrics.Precision==1.0`、`Recall==0.5`（受控值，metric 包 Compute 契约）；空 `MetricList{}.Avg()` 返回非 nil 零值；
   - `TestHookMetricRecordFinishMapsContentToPID`：`NewHookMetric(1)`，`recordInit(0)`→`recordQaPair(0, &types.QAPair{PIDs: []int{7, 9}, Passages: []string{"alpha passage", "beta passage"}, Answer: "ans"})`→`recordSearchResult(0, []*types.SearchResult{{Content: "alpha"}})`→`recordFinish(0)`；`MetricResult().RetrievalMetrics.Precision==1.0` 且 Recall==0.5（retrievalIDs 恰为 [7]）——钉住 metric_hook.go:147-:167 的内容子串回映射。
2. `go test ./internal/application/service -run 'TestGetPassageListGolden|TestMetricListAppendAvg|TestHookMetricRecordFinishMapsContentToPID' -count=1 -v` → 全 PASS（特征化对旧实现必绿；若首跑 FAIL，按 conventions §5 停下分析——说明对现行为理解有误，禁止改断言迎合）。
3. `go vet ./internal/application/service/` → 无诊断。

commit：`test(passb): characterization for evaluation dataset and metric hook`

### B3-IN.3 analytics 面迁移（repository + handler + 双测试）

**目标**：文件 1、2（§1 表）及 2 个测试文件迁入 `internal/modules/insights/analytics`；repository/handler 两侧 compat 落位；治理行成对操作；analytics 差分证据。

步骤：
1. `mkdir -p internal/modules/insights/analytics`；`git mv internal/application/repository/analytics.go internal/modules/insights/analytics/analytics_repository.go`；`git mv internal/handler/analytics.go internal/modules/insights/analytics/analytics_handler.go`；`git mv internal/application/repository/analytics_test.go internal/modules/insights/analytics/analytics_repository_test.go`；`git mv internal/handler/analytics_test.go internal/modules/insights/analytics/analytics_handler_test.go`。
2. 两生产文件头改 `package analytics`；`analytics_repository.go` import 块不变（context/time/gorm/types/interfaces）；`analytics_handler.go`：
   - `parseAnalyticsRange` → 导出 `ParseAnalyticsRange`（:40，签名 `(c *gin.Context) (from, to time.Time, ok bool)` 不变，包内 4 调用点同步）；
   - `analyticsRows` → 导出改名 `Rows[T any]`（:90，签名不变，包内 4 调用点同步）；
   - 新增模块本地 `parseFilterTime(raw string) (time.Time, error)`（逐字副本 + parity 头注引 `internal/handler/knowledge.go:2565` 与 `internal/handler/session/handler.go:432` 先例），import 增 `strings`；
   - `analyticsIsDateOnly`/`analyticsDefaultLookbackDays`/`tenantID` 原名未导出保留。
3. `analytics_repository_test.go`：package analytics；新增装置副本 `openRunTestDB`/`seedRunFixtures`/`openPostgresRunTestDB`（源 agent_run_test.go:29-:125 逐字，repoRoot 改 `"../../../.."`；头注 Ruling TEST-SUPPORT-SHIM 家族；postgres 分支保留 `TRPC_TEST_POSTGRES_DSN` env-skip 语义）；`seedAnalyticsData`/`TestAnalyticsAggregations` 本体零改动。
4. `analytics_handler_test.go`：package analytics，本体零改动（stub + gin 路由 + `&AnalyticsHandler{...}` 同包直构）。
5. 新建 `internal/application/repository/insights_passb_compat.go`（package repository；头注「Pass B 过渡 shim（37-insights），删除点 ib3，Ruling TRANSITION-SHIM-ROW-REGISTRATION」）：
   ```go
   func NewAnalyticsRepository(db *gorm.DB) interfaces.AnalyticsRepository {
       return analytics.NewAnalyticsRepository(db)
   }
   ```
6. 新建 `internal/handler/insights_passb_compat.go`（package handler；同款头注）：`type AnalyticsHandler = analytics.AnalyticsHandler`、`func NewAnalyticsHandler(repo interfaces.AnalyticsRepository) *AnalyticsHandler`（转发）、`func parseAnalyticsRange(c *gin.Context) (time.Time, time.Time, bool)` 与 `func analyticsRows[T any](rows []T) []T`（§2.3(c) 转发，覆盖 usage.go 5 个调用点 :89/:107/:114/:128/:138）。
7. 治理成对（同 commit）：manifest legacy_files 删 analytics 2 行、增 2 行 shim（path=两个 compat 文件，reason「Pass B 过渡 shim，ib3 同 commit 随文件删行」，navigation_label「Insights host compat (…)」，passb_task=B-insights——行格式镜像 datasource.yaml:9-:12 现存 shim 行）；ownership-matrix 删 :98-:103、:1478-:1483 两行，增 2 行 shim（module: insights / plan: 37-insights / destination=`internal/modules/insights/analytics` / integration_owner: ib3 / delete_barrier: ib3——行格式镜像 ownership-matrix:158-:163 现存 `internal/application/repository/datasource_passb_compat.go` 行，destination 指模块目的包）。
8. 验证：
   ```bash
   go build ./...                                                        # 退出码 0
   go vet ./internal/modules/insights/analytics/ ./internal/application/repository/ ./internal/handler/   # 无诊断（含宿主测试编译）
   go test -count=1 ./internal/modules/insights/analytics/               # ok —— TestAnalyticsAggregations(sqlite) + TestAnalyticsHandler_* 10 顶层用例
   go test ./internal/modules/insights/... -count=1                      # ok（metric + analytics）
   go test ./internal/handler -run 'TestUsage' -count=1                  # ok —— usage.go 转发链回归（usage 面 HTTP 契约不因转发变化）
   make verify-module-moves                                              # modulemove: OK (16 manifests verified)
   make check-backend-architecture                                       # 633/23+23/58/16，0 violations
   git diff --stat HEAD~1..HEAD --follow -- internal/application/repository/analytics.go internal/modules/insights/analytics/analytics_repository.go   # rename 检测（内容相似度）
   ```
9. evidence 填 analytics 差分节：P-6 宿主位基线 vs 本步模块位同用例输出逐例比对结论（等价）；postgres 子测试无 env 时记录 `--- SKIP: postgres (TRPC_TEST_POSTGRES_DSN unset...)`，按 Spec §14.4 标 blocked-env 不计 PASS，移交 IB3（Brief (d)）。

commit：`refactor(insights): migrate analytics to module with host compat`

### B3-IN.4 evaluation 面迁移（4 文件 + seam + compat + 特征化随迁）

**目标**：文件 3-6 及 B3-IN.2 特征化测试迁入 `internal/modules/insights/evaluation`；`DeleteReferencedKnowledgeFunc` seam 落地（TDD：先写 seam 测试 RED）；service/handler 两侧 compat；治理行成对；capability 奇偶差分。

步骤：
1. `mkdir -p internal/modules/insights/evaluation`；`git mv internal/application/service/dataset.go internal/modules/insights/evaluation/dataset.go`；`git mv internal/application/service/evaluation.go internal/modules/insights/evaluation/evaluation.go`；`git mv internal/application/service/metric_hook.go internal/modules/insights/evaluation/metric_hook.go`；`git mv internal/handler/evaluation.go internal/modules/insights/evaluation/evaluation_handler.go`；`git mv internal/application/service/evaluation_characterization_test.go internal/modules/insights/evaluation/evaluation_characterization_test.go`。
2. 四生产文件头改 `package evaluation`；import 路径零变化（metric_hook.go:9 `internal/modules/insights/metric` 同模块直引不变；dataset.go parquet-go、evaluation.go config/utils/errgroup 不变）。
3. **seam（§2.3(a)）**：evaluation.go 增 `DeleteReferencedKnowledgeFunc` 类型 + `EvaluationService.deleteReferencedKnowledge` 字段；`NewEvaluationService` 增第 7 参 `cleanup DeleteReferencedKnowledgeFunc`（原 6 参序型不变，返回 `interfaces.EvaluationService` 不变）；:366-:369 调用改 `e.deleteReferencedKnowledge(ctx, e.knowledgeService, knowledgeBaseID, []string{knowledge.ID})`。
4. **TDD seam 测试**：先建 `internal/modules/insights/evaluation/evaluation_cleanup_test.go`（§2.4 规格：四 stub + seam 捕获 + `EvalDataset` 断言）→ `go test ./internal/modules/insights/evaluation -run TestEvalDatasetInvokesKnowledgeCleanupSeam -count=1` 首跑编译失败（构造器尚无第 7 参，RED 留档）→ 步骤 3 完成后复跑 PASS（GREEN 留档）。stub 以嵌入接口 + 覆写所需方法实现（`stubAnalyticsRepository` 同型）；`detail := &types.EvaluationDetail{Task: &types.EvaluationTask{ID: "t1", DatasetID: "ds1"}, Params: &types.ChatManage{}}`（`Clone()` 见 types/chat_manage.go:169）。
5. 新建 `internal/application/service/insights_passb_compat.go`（package service；头注同款，删除点 ib3）：
   ```go
   func NewDatasetService() interfaces.DatasetService { return evaluation.NewDatasetService() }
   // 6 参原签名（container.go:389 dig 兼容）；第 7 参接线宿主现行
   // deleteReferencedKnowledge（knowledge_delete_plan.go:36，K4 推迟批留守未导出）——
   // 与迁移前同包直引同一目标函数，行为零变化。
   func NewEvaluationService(config *config.Config, dataset interfaces.DatasetService,
       knowledgeBaseService interfaces.KnowledgeBaseService, knowledgeService interfaces.KnowledgeService,
       sessionService interfaces.SessionService, modelService interfaces.ModelService,
   ) interfaces.EvaluationService {
       return evaluation.NewEvaluationService(config, dataset, knowledgeBaseService,
           knowledgeService, sessionService, modelService, deleteReferencedKnowledge)
   }
   ```
6. handler 侧 compat（并入 B3-IN.3 的 `internal/handler/insights_passb_compat.go`，同 commit 追加）：`type EvaluationHandler = evaluation.EvaluationHandler`、`func NewEvaluationHandler(svc interfaces.EvaluationService) *EvaluationHandler`（转发，签名照 evaluation_handler.go:20）。
7. 治理成对（同 commit）：manifest legacy_files 删剩余 4 行、service 侧 compat 增 1 行 shim（reason/navigation_label 同 B3-IN.3 Step 7 格式）；matrix 删 :782-:787、:812-:817、:1034-:1039、:1556-:1561 四行、增 service compat 1 行（destination=`internal/modules/insights/evaluation`，integration_owner/delete_barrier=ib3）。
8. 验证：
   ```bash
   go build ./...                                                                     # 退出码 0
   go vet ./internal/modules/insights/... ./internal/application/service/ ./internal/handler/
   go test -count=1 ./internal/modules/insights/...                                   # metric + analytics + evaluation（特征化 3 例 + seam 1 例）全 ok
   go test ./internal/router -run TestTenantInfrastructureRoutesDeclareSpecificCapabilities -count=1   # ok —— &handler.EvaluationHandler{} 别名构造 + POST /evaluation capability 断言（§2.1 差分）
   make verify-module-moves                                                           # OK (16 manifests verified)
   make check-backend-architecture                                                    # 633/23+23/58/16，0 violations
   grep -rn "deleteReferencedKnowledge" internal/modules/insights/                    # 仅 seam 类型/字段/调用（无宿主 import）
   ```
9. evidence 填 evaluation 节：特征化宿主位/模块位双跑输出、seam RED→GREEN 记录、capability 奇偶结论；§0.2.2 DAG notes 偏差登记项落 report。

commit：`refactor(insights): migrate evaluation family with cleanup seam`

### B3-IN.5 别名义务与 manifest 三区核销（docs only）

**目标**：insights 的 alias 义务行收口（物理删除 IA4 `4d12ba36e` 已完成，本任务行级簿记）、move 区核销、exception-ledger 属主复核。

步骤：
1. manifest：`alias_obligations: []`（删 :8-:9 行）；`move_packages: []`；`owned_files.move_sources: []`、`move_targets: []`（datasource 终态同型，见其 manifest :4-:5/:23-:28）；`importers` 改为三 shim 宿主目录列表（`internal/application/repository`、`internal/application/service`、`internal/handler`）。
2. matrix：删 aliases 区 :2268-:2270 行。
3. `grep -n "37-insights" docs/architecture/passb/exception-ledger.yaml` → 零命中（预期；若非零，属 B0 冻结遗漏，停下按 §5 上报）；`grep -c "id: exc-"` 记录行数（撰写时点 142）供 B5 口径。
4. `make verify-module-moves` → `modulemove: OK (16 manifests verified)`；`make check-backend-architecture` → 0 violations。

commit：`docs(passb): close insights alias obligation and manifest regions`

### B3-IN.6 README / legacy-README 回填（docs only）

**目标**：模块文档反映迁移终态（datasource/K 节点同型格式）。

步骤：
1. `internal/modules/insights/legacy/README.md`：6 行各标 **已迁移**（analytics 2 行 → `insights/analytics/`（B3-IN.3）；evaluation 4 行 → `insights/evaluation/`（B3-IN.4））；3 行 compat 补登（Ruling TRANSITION-SHIM-ROW-REGISTRATION，ib3 删）；头段补「manifest 行已按 Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP 随物理迁移 commit 删除」说明。
2. `internal/modules/insights/README.md`：职责/非职责保持；「搬迁的包」表更新（metric Pass A 已迁 + analytics/evaluation Pass B 已迁）；「横向包遗留文件」改 0（列 3 个过渡 compat）；公开面补 `analytics.NewAnalyticsRepository`/`analytics.NewAnalyticsHandler`/`ParseAnalyticsRange`/`Rows`、`evaluation.NewDatasetService`/`NewEvaluationService`（7 参含 seam）/`NewEvaluationHandler`/`DeleteReferencedKnowledgeFunc`；集成点 routes 2 / workers 0 / hooks 0 不变；验收命令不变。
3. `go build ./...` 复核（纯 docs，应无影响）。

commit：`docs(insights): update module and legacy README migration status`

### B3-IN.7 差分证据收口 + Integration Brief + 实施报告 + 终局门禁

**目标**：conventions §1.1/§1.2 报告包三件套落盘，节点四门禁 + 消费者面全绿，owned_files 差集为零。

步骤：
1. **Integration Brief** `docs/architecture/passb/briefs/b3-insights.md`（ib3 切换申请，逐条点名）：
   - (a) 实装 `internal/modules/insights/module.go` 五操作门面（insights.facade 冻结 items；RegisterWorkers/Start/Stop 空实现——workers/hooks 均零）；
   - (b) evaluation seam 接线源切换：K4 补迁窗导出 `deleteReferencedKnowledge` 后，service compat（或门面构造）改指 knowledge 导出版，seam 形态不变；K4 推迟批未落地期间维持宿主裸标识符接线；
   - (c) usage.go（commercial legacy）5 个调用点（:89/:107/:114/:128/:138）直改 `analytics.ParseAnalyticsRange`/`analytics.Rows` 后删 handler compat——或按 commercial usage.go 迁移时点衔接，两案并列请协调者裁定；
   - (d) IB3 必测：`go test ./internal/modules/insights/analytics -count=1`（sqlite）；postgres 方言以 `TRPC_TEST_POSTGRES_DSN` 环境复跑 `TestAnalyticsAggregations/postgres`（framework:192；本节点 blocked-env 记录见 evidence）；
   - (e) container.go:217/:388/:389/:746/:761 切模块构造器、routes_analytics.go:15/routes_infra.go:94 参数类型切 `*analytics.AnalyticsHandler`/`*evaluation.EvaluationHandler`、router_api_key_capabilities_test.go:344 构造同步、contracts.yaml consumer 路径行（airesource.model-service :358、conversation.session-service :967）回写为模块路径；删 3 个 compat 文件 + manifest/matrix shim 行（同 commit）。
2. **evidence** `docs/architecture/evidence/passb/b3-insights.md` 终稿：五节齐备（基线/analytics 差分/evaluation 特征化与 seam/治理行/门禁），每命令原文+退出码+关键输出。
3. **报告** `docs/plans/passb/reports/b3-insights.md`：conventions §1.2 四条命令输出；变更文件清单 vs owned_files 逐条核对；§0.2.2 偏差登记；未完成项如实列出。
4. 终局门禁（conventions §1.2 + DAG gates 原文）：
   ```bash
   PASSB_BASE_SHA=<ALIGN_SHA（开工 HEAD）>
   git diff --stat "$PASSB_BASE_SHA"...HEAD
   go build ./...
   go test -count=1 ./internal/modules/insights/...
   make check-backend-architecture
   make verify-module-moves
   go test ./internal/router -run TestTenantInfrastructureRoutesDeclareSpecificCapabilities -count=1
   go test ./internal/handler -run 'TestUsage|TestAnalyticsHandler' -count=1   # 后者预期 no tests to run（已迁走）——以包编译+usage 回归为准
   git diff "$PASSB_BASE_SHA"...HEAD --name-only | sort                        # 与 §3 写入清单求差集，差集非空即失败
   ```
   预期：全绿；diff 文件清单 = §3 全集（含三报告件）。

commit：`docs(passb): b3-insights evidence brief and report`

---

## 5. 集成与回滚边界

- **集成边界**：本节点止于「模块就位 + 宿主 compat 零改动编译 + 差分证据」；module.go 门面、container/router 切换、compat 删除、contracts.yaml consumer 回写全部属 ib3（Brief 逐条点名）。兄弟 B3 节点（R1/R2、conversation application 层、channels）与本节点文件不相交（各自 manifest/matrix 行互斥，B0 校验在案）。
- **回滚边界**（Spec §13 提交隔离映射）：B3-IN.3/B3-IN.4 任一失败 → revert 该 commit 即回到宿主原状（模块新文件可留为未接线代码）；compat shim 为唯一宿主残留面，随 revert 消失；无 schema/数据面改动，回滚无需数据修复；治理行（manifest/matrix）与物理迁移同 commit，revert 自动回行。B3-IN.5/:6/:7 为 docs only，独立可回退。**禁止**为救编译修改 usage.go/knowledge.go/knowledge_delete_plan.go/container/router（§3 零触碰清单）——出现该需求即 §5 升级。
- **升级通道**（conventions §5）：seam 接线若因 K4 补迁窗与 ib3 时序冲突断链、或审查对 parseFilterTime 本地副本/usage 转发裁定改向，均在报告登记 + DAG 置 blocked，不现场改判。

## 6. 独立验收标准（本节点完成的充要判据）

1. 6 个 legacy 文件物理位于冻结 destination，宿主 6 路径不存在；`git log --follow` 链可追溯；`internal/modules/insights/{analytics,evaluation}` 包编译且测试全绿（含 B3-IN.2 特征化 3 例、seam 1 例、analytics 既有用例组：`TestAnalyticsAggregations` 双方言 + `TestAnalyticsHandler_*` 10 顶层用例）。
2. `TestAnalyticsAggregations`（sqlite）+ `TestAnalyticsHandler_*` 10 个顶层用例在模块位与 P-6 宿主位基线逐例等价（evidence 双跑输出在案）；postgres 方言按 blocked-env 如实登记并移交 Brief (d)。
3. `go build ./...`、`make check-backend-architecture`（633/23+23/58/16、0 violations）、`make verify-module-moves`（16 manifests OK）、`go test -count=1 ./internal/modules/insights/...` 四条 DAG gates 全绿；`TestTenantInfrastructureRoutesDeclareSpecificCapabilities`（POST /evaluation capability）与 `internal/handler` usage 面回归绿。
4. 治理成对纪律：manifest 与 matrix 的 6 legacy 行删除均与物理迁移同 commit；3 shim 行成对补登（Ruling 7）；alias 行/move 区核销完成；exception-ledger 零属主行实测在案。
5. 零触碰清单（§3）经 `git diff "$PASSB_BASE_SHA"...HEAD --name-only` 差集验证为零越界；module.go/metric 包 diff 为空。
6. 三件套（evidence/report/brief）路径与命名符合 conventions §1.1；brief 覆盖 (a)-(e) 五切换项。
7. 报告含 §0.2.2 DAG notes「K4 已导出」偏差登记及 seam 等价性论证。

## 7. 计划自检记录（撰写会话）

- **Spec 覆盖**：§5.14/§5.18 边界与 READ-ONLY（analytics 仅 SELECT 三源表——repository 迁移零 SQL 改动由差分钉住）、§11 B3 并行/IB3 串行、§12 循环（特征化 B3-IN.2 先行）、§13 提交隔离与回滚（§5）、§14.3 Knowledge 删除差分（seam 测试 + 构造等价）、§14.4 blocked-env（postgres）、§17.2 方向项（legacy 清零、别名清零）。
- **无占位符**：全部符号含 file:line 与实读签名（deleteReferencedKnowledge :36-:41、ParseAnalyticsRange/Rows 原型、compat 构造器逐参）；命令均可执行且有实测预期输出。
- **类型一致**：seam 类型与宿主函数签名逐参一致（ctx/svc/expectedKB/ids → error）；compat 6 参构造器与 container.go:389 消费面一致；type 别名覆盖 router.go:74/:82 具体类型消费。
- **跨任务接口一致**：B3-IN.2 测试文件名/用例名与 B3-IN.4 随迁步骤一致；B3-IN.3 产出的 handler compat 文件在 B3-IN.4 Step 6 追加（同文件两段，commit 归属各自任务段）；Brief (a)-(e) 与 §2.1/§2.3 消费面闭环。
- **事实核验**：基线六命令本会话实跑（P-6 值为实测）；usage.go/knowledge_delete_plan.go/knowledge.go/session 先例/agentcatalog 装置副本均为直读；`package analytics|evaluation` 零冲突、exception-ledger 零属主行、ledger 142 行均实测。
