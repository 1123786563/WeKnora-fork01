# Integration Brief — b2-k-integration（K5 knowledge 模块门面实装与装配切换汇总）

> 提交方：work 节点 `b2-k-integration`（plan `docs/plans/passb/20-knowledge-program.md` Task K5.1）。
> 消费方：ib2 集成屏障（集成工程师按本 Brief 串行执行共享装配切换；framework:103「IB 只从已评审 Brief 切换共享装配」）。
> 事实源：K1–K4 四 Brief（`b2-k-ingest.md` / `b2-k-retrieval.md` / `b2-k-wikifaq.md` / `b2-k-process.md`，随本分支基线对齐 merge 可见）+ 门面实装 `internal/knowledge/module.go`（K5.1）+ `module_test.go`（五测试）。
> 写权限边界：本 Brief 只登记装配变更申请；`internal/router/**`、`internal/container/**`、`internal/bootstrap/**`、`contracts.yaml`、`ownership-matrix.yaml` 的实际写入由集成工程师/barrier 独占（conventions §3；K5.1 门面 commit 零触碰这些文件，M4 预备态纯新增）。
> 行号基准：本分支 ALIGN_SHA=b9c09f524（P-K5-2 Case A 对齐 merge，含 codex/passb-b2-k-process K1–K4 终态）实测；实施时点复核为准。

## 头注：container.go 已提前落地项（IB2 转核验清单）

| 项 | 内容 | 出处 |
|---|---|---|
| K3 A1 | `internal/container/container.go:316` `must(container.Provide(knowledgeWiki.NewWikiPageRepository))` —— **已提前落地，无需再切**（Ruling 2026-09-25-CYCLE-FORCED-COMPOSITION：repository 侧转发 shim 因 `wiki→agentruntime→agent/tools→repository` import 环不可编译，container.go:312-315 注释登记）。IB2 核验项：该行 + import 行 `knowledgeWiki`（container.go:93）确已指向 wiki 包，并删除 repository 侧 `NewWikiPageRepository` 转发（`wiki_k3_repo_compat.go`，(e) W3） | K3 Brief (a) A1 |

## (f) 门面五操作精确签名（K5.1 已实装，`internal/knowledge/module.go`）

```
func NewModule(deps Dependencies) (*Module, error)
func (m *Module) RegisterRoutes() (HandlerSet, error)
func (m *Module) RegisterWorkers(redis, lite *bootstrap.WorkerRegistry) error
func (m *Module) Start(ctx context.Context) error
func (m *Module) Stop(ctx context.Context) error
```

`Dependencies` 字段表（16 必填 + 1 可选；类型与 `router.AsynqTaskParams` / `router.SyncTaskParams` knowledge 子集逐一对应，构造仍在 dig 容器、门面只收已构造实例）：

| 字段 | 类型 | dig 供给（ib2 注入源） |
|---|---|---|
| KnowledgeService | interfaces.KnowledgeService | 现容器同名类型（实现体 K4 推迟件留守宿主，接口满足性=现状即满足） |
| KnowledgeBaseService | interfaces.KnowledgeBaseService | 同上（K2 推迟件） |
| TagService | interfaces.KnowledgeTagService | 同上（K2 落位 `retrieval/app`） |
| ChunkExtractor | interfaces.TaskHandler | container.go dig name `"chunkExtractor"`（经 K1 shim/ingest 构造器） |
| DataTableSummary | interfaces.TaskHandler | dig name `"dataTableSummary"` |
| ImageMultimodal | interfaces.TaskHandler | dig name `"imageMultimodal"` |
| KnowledgePostProcess | interfaces.TaskHandler | dig name `"knowledgePostProcess"`（container.go:417，K4 推迟件切换见 K4 Brief (a)） |
| KnowledgeAutoTag | interfaces.TaskHandler | dig name `"knowledgeAutoTag"`（:418） |
| WikiIngest | interfaces.TaskHandler | dig name `"wikiIngest"`（container.go:437，`dig.Name` 保留——K3 Brief (a) A3 冻结面） |
| Chunk | *ingest.ChunkHandler | container.go:719 `NewChunkHandler`（K1 §6.3 wrapper，ib2 切 `ingest.NewChunkHandler`） |
| ChunkerDebug | gin.HandlerFunc | 生产值 `ingest.PreviewChunking`（chunker_debug.go:123；K1 §6.3 R1） |
| WikiPage | *wiki.WikiPageHandler | container.go:864（K3 H1 wrapper，ib2 去 wrapper） |
| FAQ | *faq.FAQHandler | container.go:725（K3 H2，helper seam 两参见 K3 Brief (a) A5） |
| Tag | *kbhandler.TagHandler | K2 落位 `retrieval/app/handler`（container 直连点见 K2 Brief §3.1） |
| SemanticModelPolicy | *kbhandler.SemanticModelPolicyHandler | 同上 |
| SemanticInternal | *kbhandler.SemanticInternalHandler | 同上（container.go:197 区段） |
| PendingWikiRecovery（可选） | func(ctx context.Context) | 生产值 = container 侧 `recoverPendingWikiTasks` 等价闭包（(c)）；nil 时 Start no-op |

`HandlerSet` 字段与 `Dependencies` 路由供给段一一对应（7 组已落位；其余 4 项为宿主推迟件：3 组 handler——RegisterKnowledgeRoutes/RegisterKnowledgeBaseRoutes/RegisterKnowledgeBaseActivityRoutes 的 KnowledgeHandler/KnowledgeBaseHandler/AuditLogHandler——与 serveKBScopedFiles 文件服务面（非 handler 供给，无字段）；ib2/补迁窗后增补 `Dependencies`/`HandlerSet` 字段，装配面扩展非契约变更）。

## (a) 11 路由注册切换表（§7.1 全表；宿主→模块 HandlerSet 供给；RBAC/路由计数 633 零变化声明）

路由体与 `rbacGuards` guard 语义全部**留驻 internal/router**（B2 无平台端口可消费，spec §4.3；architectureguard 路由扫描面=`internal/router`+`internal/handler`，模块内路由代码不入 633 计数——K5 门面只交付 handler 供给，禁止双写路由表）。ib2 切换 = 把各注册函数的 handler 形参类型从宿主 `*handler.X` 切到模块供给类型；`g *rbacGuards` 形参与路由体零改动。

| # | 注册函数（现态行号） | 形参现状 | ib2 切换目标（HandlerSet 字段） | 依据 |
|---|---|---|---|---|
| 1 | `RegisterChunkerDebugRoutes`（routes_knowledge.go:18） | `r *gin.RouterGroup, g *rbacGuards`（体内 :19 `handler.PreviewChunking`） | 体内改 `set.ChunkerDebug`（gin.HandlerFunc，生产值同 `ingest.PreviewChunking`） | K1 §6.3 R1；K1 Brief §9 ② |
| 2 | `RegisterChunkRoutes`（:28） | `handler *handler.ChunkHandler`（wrapper） | `*ingest.ChunkHandler`（set.Chunk） | K1 §6.3；K1 Brief §9 ②③（11 条 /chunks 路由 + container.go:719 同窗） |
| 3 | `RegisterKnowledgeRoutes`（:67） | `handler *handler.KnowledgeHandler`（宿主原生，K4 推迟件 #16 留守） | **无即时切换**（HandlerSet 暂无该字段；#16 补迁后增补字段+切换，见 (h)） | K4 Brief (a) 第 6 行 |
| 4 | `RegisterFAQRoutes`（:143） | `handler *handler.FAQHandler`（H2 类型别名） | `*faq.FAQHandler`（set.FAQ） | K3 Brief (a) A7 |
| 5 | `RegisterKnowledgeBaseRoutes`（:185） | `handler *handler.KnowledgeBaseHandler`（K2 推迟件 #2 留守宿主） | **无即时切换**（补迁后增补；rbac_lookups 去方法化前置见 K2 Brief §5 #2） | K2 Brief §5 #2 |
| 6 | `RegisterSemanticModelPolicyRoutes`（:254） | `policyHandler *handler.SemanticModelPolicyHandler`（compat type） | `*kbhandler.SemanticModelPolicyHandler`（set.SemanticModelPolicy） | K2 Brief §3.2 |
| 7 | `RegisterKnowledgeBaseActivityRoutes`（:266） | `auditHandler *handler.AuditLogHandler`（非 K 计划属主文件） | **无切换**（不属 knowledge 18 文件；零动作登记） | 20 计划 §7.1 注 |
| 8 | `RegisterKnowledgeTagRoutes`（:279） | `tagHandler *handler.TagHandler`（compat type） | `*kbhandler.TagHandler`（set.Tag） | K2 Brief §3.2 |
| 9 | `RegisterWikiPageRoutes`（:309） | `wikiHandler *handler.WikiPageHandler`（H1 wrapper） | `*wiki.WikiPageHandler`（set.WikiPage） | K3 Brief (a) A8（H1 wrapper 删除批同窗） |
| 10 | `RegisterSemanticInternalRoutes`（routes_infra.go:14） | `h *handler.SemanticInternalHandler`（compat type） | `*kbhandler.SemanticInternalHandler`（set.SemanticInternal） | K2 Brief §3.2 |
| 11 | `serveKBScopedFiles`（files.go:325；router.go:332 调用） | router 包私有 func（参数经 handler 面） | **无即时切换**（路由体耦合 rbacGuards/files 平台面；K2/K4 handler 推迟件补迁后随其形参面评估） | 20 计划 §7.1；K3 Brief (a) 并发项已并入 4/9 行 |

并发项合并声明（K1 Brief §9 ②③ + K3 Brief (a) A7/A8）：container.go:719（ChunkHandler）/ :725（FAQHandler）/ :864（WikiPageHandler）三行 Provide 与上表 2/4/9 同窗切换；router 侧测试字面量（`internal/router/router_api_key_capabilities_test.go:301` `&handler.TagHandler{}`、`semantic_internal_test.go:18`、FAQ `&FAQHandler{}` 零字段字面量）随 compat 删除同窗改指模块包。**RBAC 语义（rbacGuards 闭包族）与路由计数 633 零变化（conventions §8；K5.1 门禁实测 total=633 复证）。**

## (b) 18 worker 双栈装配切换申请

**现状**（ALIGN_SHA 实测）：Redis 栈 `internal/router/task.go` 18 行 `mux.HandleFunc(types.TypeX, params.Y)`（:266/:267/:270/:274/:277/:280/:283/:286/:289/:292/:298/:301/:304/:307/:310/:311/:319/:320）；Lite 栈 `internal/router/sync_task.go` 18 行 `params.Executor.RegisterHandler(types.TypeX, params.Y)`（:143-:163 区段 knowledge 子集）。两栈第二参同形 `func(context.Context, *asynq.Task) error`，与方法值映射逐条同构（K5.1 `workerHandlers()` 18 映射 = 同类型同方法，module_test.go parity 测试锚定精确集）。

**ib2 切换序**（注册行本体集成工程师独占，conventions §3；K0 §7.2「注册行禁改」指 K1–K4 不得改——ib2 装配切换属集成工程师职权）：

1. 适配 `internal/bootstrap.WorkerSink` 两枚（`WorkerSink.RegisterTaskHandler(taskType string, handler any)`，workers.go:21-22）：
   - Redis 栈 sink：包 `*asynq.ServeMux`，委托 `mux.HandleFunc(taskType, handler)`；
   - Lite 栈 sink：包 `*router.SyncTaskExecutor`，委托 `RegisterHandler(taskType, handler)`。
2. 在装配点构造 `redis := bootstrap.NewWorkerRegistry("redis", redisSink)`、`lite := bootstrap.NewWorkerRegistry("lite", liteSink)`（workers.go:28）。
3. 以 (f) 表供给构造 `knowledge.NewModule(deps)`，调用 `mod.RegisterWorkers(redis, lite)`——模块内已按稳定序逐类型双栈登记并以 `bootstrap.VerifyWorkerParity` 收尾（同 registry 重复登记返回 `already registered` 错误，幂等保护）。
4. 删除 task.go/sync_task.go 上述 18+18 行原注册行（`AsynqTaskParams`/`SyncTaskParams` 的 9 个 knowledge 分发面字段保留为 `Dependencies` 注入源；非 knowledge 行零触碰）。
5. 切换后计数基线 23+23 不变（Redis/Lite 各 18 类型仍在双栈各登记一次；architectureguard redis/lite 计数复验）。

**等价性声明**：18 类型→处理器映射与现行注册行逐条同构（同类型同方法值；`TypeWikiIngest`/`TypeWikiFinalize` 共享 `params.WikiIngest.Handle` 保持）；重试退避注释锚（task.go:117-124，K3 Brief (a) A10 所指 `errors.Is(e, service.ErrWikiIngestConcurrent)` 行随 W1 删除同窗改 `wiki.ErrWikiIngestConcurrent` 直引，`errors.Is` 语义不变）。

## (c) `recoverPendingWikiTasks` 切模块 Start 的等价性说明

- **现状**：`internal/container/container.go:1058` `must(container.Invoke(recoverPendingWikiTasks))`；func 定义 `internal/container/recover_pending_wiki_tasks.go:32`，签名 `func recoverPendingWikiTasks(db *gorm.DB, task interfaces.TaskEnqueuer)`（包私有，K5 禁改该文件）。
- **ib2 切换申请**：把 container.go:1058 挂接行改为经模块门面——`PendingWikiRecovery` 注入 `func(ctx context.Context)` 等价闭包（体内调用同一恢复函数或其导出包装），在模块 Start 序 `mod.Start(ctx)` 触发；**单一注册点原则（spec §4.3「同一钩子只注册一次」）**：原挂接行与门面注入二选一，切换 commit 内撤销原行。
- **过渡期无害**：恢复函数幂等（recover_pending_wiki_tasks.go:27-30「Duplicate triggers are harmless」）；双重触发窗口内行为等价。
- **恢复语义冻结面照录（K3 Brief (a) A9）**：fail-closed 清扫（:45-58）、`asynq.TaskID("wiki-finalize-"+scope.ScopeID)`、MaxRetry 10、Timeout 60/30min——切换零改动；同文件 A9（`service.WikiIngestPayload` → `wiki.WikiIngestPayload`，:75）与 A9′（`reset_pending_tasks_test.go:388/:390`）随 W1 删除同窗改直引。
- `Stop(ctx)` 预留对称面：知识域当前无模块自持后台 goroutine（housekeeping 调度经 `KnowledgeHousekeeping` 窄端口由 42-system-policy 消费方调度，24 计划 §5.3——**不得出现第二套清扫实现**）；ib2 切换 Start 后 Stop 暂为 no-op 登记。

## (d) seam 装配接线表（K1 Brief §7 全表 + K3 Brief (d) faq/wiki Seams 合并；K4 Brief (g) 已就绪/随推迟批分列）

**K1 ingest 侧**（适配器现位 `internal/application/service/span_trace_seam_adapter.go`，与 shim 同属 ib2 删除批；定义 `ingest/seams.go`）：

| seam | 生产接线（ib2） | 现接线点 | 就绪状态 |
|---|---|---|---|
| `SpanTraceSeam` | `NewSpanTraceSeamAdapter(tr SpanTracker)`；container.go:374 `service.NewSpanTracker` 经适配器直供 R1-2/3/5 新参；K4 搬迁 knowledge_span_tracker.go 后指其导出包装 | span_trace_seam_adapter.go:30 | 随 K4 推迟批 |
| `KnowledgeWriteGuard` | `KnowledgeWriteGuardProvider()` → **`process` 包已导出 write-family 4 函数**（K4 Brief (g) 已就绪） | :99 | **已就绪**（WriteResourceIDs/WriteExecutionTenant/LoadKnowledgeWrite/LoadKnowledgeWriteBatch，`internal/knowledge/process`） |
| `KBByIDLookup` | `KnowledgeWriteKBProvider()` → `process.KnowledgeWriteKB`+`KnowledgeBaseWriteLookup`（已导出） | :146 | **已就绪** |
| `enqueueSummaryRefresh` 闭包 | `EnqueueSummaryRefreshProvider(tr)`（参数 `summaryKnowledgeBaseReader` → KBByIDLookup） | :81 | 随推迟批 |
| `BuildKnowledgeIndexContent` 闭包 | `BuildKnowledgeIndexContentProvider()` → `process.BuildKnowledgeIndexContent`（已导出） | :126 | **已就绪** |
| `attemptSupersededFn` | `AttemptSupersededProvider(tr)`（nil 恒 false noop；K4 knowledge.go:202 导出后改指） | :133 | 随推迟批 |
| `previewTextFn` | 直传宿主 `previewText` → **改指 `wiki.PreviewText`（K3 已导出）** | R1-3/R1-5 shim 体内 | **已就绪** |
| `resolveProcessConfigFn` | 直传宿主 `ResolveProcessConfig`（knowledge_process_config.go 留守，K4.1 §4.3「比原方案更简」口径） | R1-3/R1-5 shim 体内 | 随推迟批 |
| `postProcessTaskOptionsFn` | 直传宿主 `knowledgePostProcessTaskOptions` → `process.KnowledgePostProcessTaskOptions`（已导出） | R1-5 shim 体内 | **已就绪** |
| `DataAnalysisToolFactory` | `DataAnalysisToolSeamFactory()`（agentruntime/conversation 根门面端口就绪后切直连） | :164 | 随 B3/35 计划 |
| `GraphExtractorFactory` | `GraphExtractorSeamFactory()`（K1.3 import 环裁决） | :216 | 同上 |
| 哨兵 error 字段（knowledgeNotFoundErr/knowledgeBaseNotFoundErr） | 宿主 `repository.ErrKnowledgeNotFound`/`ErrKnowledgeBaseNotFound` 注入（ingest 禁 import 宿主 repository；K4 搬迁后指其导出） | R1-5 shim 体内 | 随 K4 推迟批 |

**K3 wiki/faq 侧**（`faq.Seams` service.go:41-76 / `wiki.Seams` seams.go:29-53；现接线在 D1 `faqSvc()` 闭包与 W2 `wikiK3Seams()`）：

| seam 字段 | 现接线目标（宿主符号） | ib2 后目标 | 就绪状态 |
|---|---|---|---|
| faq.RecordKBActivity / KBActivityTrigger / WithKBActivityTask / KBActivityAppendSampleTitles | kb_activity.go:95/:36/:27/:49（K2） | `retrieval/app` 导出（app.RecordKBActivity 等） | **已就绪** |
| faq.ResolveKBReadTenant | knowledgebase_access.go:21（K2，经 `faq.KBShareLookup` 镜像接口适配） | `app.ResolveKBReadTenant` | **已就绪** |
| faq.WritableFAQKnowledgeBase | `(*knowledgeService).writableFAQKnowledgeBase`（knowledgebase_access.go:38，K2 方法） | K2 落位导出包装；或随 KnowledgeService 实现体归宿收口（K4 Brief (c) 蓝本第 2 点） | 随推迟批 |
| faq（handler 侧）parseTagIDs | `parseCommaSeparatedTagIDs`（handler/knowledge.go:2546，K4） | `process` 落位导出 | 随 K4 #16 |
| faq（handler 侧）requireTaskProgressTenant | `requireTaskProgressTenant`（handler/task_progress_auth.go:14，留驻 handler 宿主） | 按其文件归属裁定；最小动作=构造参保留直引 | 非本程序 |
| wiki.ResolveDeadSlug | slug_fuzzy.go:91（K2） | `app.ResolveDeadSlug` | **已就绪** |
| wiki.FinalizeSubtaskDetached | knowledge.go:235（K4） | `process` 落位导出 | 随推迟批 |
| wiki.IsLikelyRateLimitError | knowledge_process.go:3902（K4） | 同上 | 随推迟批 |
| wiki.RemoveSourceRef | knowledge_delete.go:346（K4） | 同上 | 随推迟批 |
| wiki.RecordWikiContentActivity | 宿主导出函数（kb_activity.go:181，K2） | `app.RecordWikiContentActivity` 直 import 落位包（K0 组 C 原裁定形态） | **已就绪** |
| wiki.IsKnowledgeBaseNotFound | `errors.Is(err, repository.ErrKnowledgeBaseNotFound)` 闭包 | K4 哨兵落位后直引 | 随推迟批 |

计划列出的假阳性 seam（`hash`/`contains`，K3 evidence §2.4-3/§1.6-5 零命中）无 ib2 动作。**三族 seam 收口**（K2 Brief §6）：`semanticScopeGuard` 三份统一 identity 导出版本；`auditActor/auditActorRole` 接 identity 导出 `AuditActor`/`AuditActorRole` 后删 `app/audit_actor_seam.go`；`escapeLikeKeyword` 与 K4 `process/repository.EscapeLikeKeyword` **合并处置**为直连（K4 Brief (d)）。

## (e) 宿主薄 shim/compat 删除批清单（K5 盘点；删除归 ib2，逐一「删除前置=残留 importer 清零」）

Ruling TRANSITION-SHIM-ROW-REGISTRATION 成对删行口径：下表过渡物文件与其 `knowledge.yaml` legacy_files / `ownership-matrix.yaml` 行**同 commit 成对删除**；legacy 计数回落走 conventions §8 基线变更（台账 + evidence + F5 三方一致复验）。

| 属主 | id | 文件 | 内容摘要 | 删除前置（残留 importer 清零） |
|---|---|---|---|---|
| K1 | — | `internal/application/repository/chunk_ingest_shim.go` | NewChunkRepository + ErrChunkRevisionConflict/ErrChunkNotFound var（R1-8 族） | container.go:205 dig 直连 + K4 宿主测试面（document_write_access_test 等）随 #2-12 补迁改写 |
| K1 | — | `internal/application/service/chunk_ingest_shim.go` | R1-1..R1-10 转发 13 符号（NewChunkService/NewChunkExtractService/NewDataTableSummaryService/NewImageMultimodalService dig 旧签名 + isFinalAsynqAttempt/sameChunkDocument/enqueueDataTableSummaryIfNeeded 等） | (d) K1 表接线完成 + K4 留守文件改直连 + conversation R1-10 改写（K1 Brief §8） |
| K1 | — | `internal/application/service/span_trace_seam_adapter.go` | seam 适配器族（SpanTraceSeam 等 7 提供器） | (d) K1 表全部改直连落位包导出 |
| K1 | — | `internal/handler/chunk_ingest_shim.go` | ChunkHandler wrapper + NewChunkHandler + PreviewChunking | K1 Brief §9 四步序（identity 去方法化 → 路由切门面 → container.go:719 切 ingest.NewChunkHandler → 删文件） |
| K2 | — | `internal/application/repository/kbretrieval_passb_compat.go` | var/type 别名（NewSemanticControlRepository 等 6） | K2 Brief §3.1 container 直连完成 |
| K2 | — | `internal/application/service/kbretrieval_passb_compat.go` | §2 表 12 符号一行委托 + semantic 面 + semanticScopeGuard 同形 + writableFAQKnowledgeBase 再归置 | §3.1 直连 + §4 跨 owner 改写 + guard 三族收口；writableFAQKnowledgeBase 随 K4 补迁/属主接收 |
| K2 | — | `internal/handler/kbretrieval_passb_compat.go` | 3 type + 3 var 别名 | §3.2 直连完成 |
| K2 | — | `internal/application/repository/kbretrieval_passb_compat_test.go` | 测试垫片（Ruling TEST-SUPPORT-SHIM，服务 K4 knowledge_tag_test.go:219/:241） | knowledge_tag_test.go 补迁/直连改写（K4 Brief (f) ⑤：消费者未清零，ib2 删除前置=补迁） |
| K3 | W1 | `internal/application/service/wiki_k3_compat.go` | wiki 面转发 17 符号（previewText 族/WikiIngestPayload 类型别名等） | K3 Brief (b) wiki 表逐行改写（K1/K4 落位 + A9/A9′/A10） |
| K3 | W2 | `internal/application/service/wiki_k3_ctor_compat.go` | NewWikiPageService/NewWikiIngestService/NewWikiLintService 转发（5 参 + seams） | K3 Brief (a) A2-A4；wikiK3SpanAdapter 待 K4 span 落位收口 |
| K3 | W3 | `internal/application/repository/wiki_k3_repo_compat.go` | NewWikiPageRepository 转发（A1 已提前落地）+ EscapeLikePattern + 4+1 哨兵 | escapeLikePattern 收口裁定 + 哨兵单点化（wiki 包）+ B3 调用点改写 |
| K3 | H1 | `internal/handler/wiki_page_k3_compat.go` | WikiPageHandler wrapper | A6 + rbac_lookups 收口（方法移 `*wiki.WikiPageHandler` 或自由函数化；与 10-identity 同窗协调） |
| K3 | H2 | `internal/handler/faq_k3_compat.go` | FAQHandler 类型别名 | A5/A7；helper seam 两参改指 K4 落位导出 |
| K3 | S1 | `internal/handler/session/wiki_fixer_scope_compat.go` | resolveWikiFixerTenantScope 方法化保持 | A11（qa.go:205 改 `wiki.ResolveBuiltinWikiFixerTenantScope`） |
| K3 | D1 | `internal/application/service/knowledge_faq_k3_delegate.go` | FAQ 接口面 15 方法委托 + 4 helper | K4 knowledge.go 落位 process 后按 24 计划收口（接口不变，delegates 换指向） |
| K4 | — | `internal/application/service/kbprocess_passb_compat.go` | 8 委托（write-family/indexContent/task-options） | K4 Brief (e)：推迟件 #2-12 补迁 + K1 adapter/shim 改直连 + 留守测试改写 |
| K4 | — | `internal/handler/kbprocess_passb_compat.go` | 5 委托（resolveHandlerKBAccess 族） | 同上 + kb_access_test.go 随 #16 补迁窗 + faq_k3_compat.go 同窗 |

测试垫片批（Ruling 2024-09-24-TEST-SUPPORT-SHIM；IB2 先到先删，最迟 B5）：K1 `chunk_ingest_test_shim_test.go`（tshim-0001/0002）、K3 `wiki_k3_test_support_shim_test.go` + `knowledge_faq_k3_test_support_shim_test.go` + W1/W2 伴生测试（`wiki_k3_compat_test.go`/`wiki_k3_span_adapter_test.go`）；K4 垫片未创建（成因文件留守，前提不成立——重评时点见 K4 Brief (h)）。

## (g) K5.2 的 18 别名删除结论与例外 30 行收口编排（ib2 删除批输入）

- **18 别名**（knowledge.yaml alias_obligations）：K5.2 按 20 计划 §7.5 预检结论执行成对删行（manifest + ownership-matrix aliases 区同 commit；2026-09-26 预检=18 旧路径物理目录已全部不存在、全仓 import 零命中，唯一文本残留 `internal/knowledge/docparser/anydoc/convert_linked_test.go:19` build 指令注释就地修正）；passbguard alias 双侧奇偶以 manifest↔matrix 行集对照为键，无 18 字面计数断言（check.go:167-206 实读）。**K5.2 交付逐条结论，本 Brief 登记删除编排指针。**
- **例外 30 行**（exception-ledger.yaml，owner=21/22/23/24 计划，`remove_at: ib2`）：K1 7（exc-0088+0106..0111）、K2 8（0089-0091+0112..0116）、K3 13（0117..0129）、K4 2（0130/0131）。**K5 不删行**（conventions §3：行属主是 21/22/23/24）；ib2 删除批前置：
  - exc-0088：airesource 根门面暴露 `Sign` 或等价端口 → docparser 消费切换 → 删行（K1 Brief §10）；
  - exc-0089/0090/0091：两分支择一——①随 ib2 与 airesource 门面任务（embedding/utils 经模块根公共门面导出）收口后删除；②ADR 修订 `RetrieveEngineService` 签名后删除（K2 Brief §7）；
  - exc-0106..0131：各被导入包（airesource/models/{chat,embedding,utils/ollama,vlm}、policy/access、agentruntime/{agent,modelcontext}）门面/端口合法化后切换直连，删 check.go importExceptions 数据行 + ledger 行同窗（K1 Brief §11、K3 Brief 其它登记 1）。
  - **`container.go:36-38` 三行旧路径 import 改写申请**（K0 §7.5/K1 Brief §12 义务）：ib2 把 knowledge 相关旧包 import 行改指 `internal/knowledge/*` 落位路径（与 (a)/(b) 切换同窗评估）。
- 治理文件回写请求（barrier 窗口，K5 禁改）：contracts.yaml knowledge 区 consumers/characterization 路径漂移回写（K1 Brief §3 12 条 + K2 Brief §8 tag_delete_test 路径等）与 status 字段（F2 补落盘后）；ownership-matrix legacy 行删除按各属主补迁窗口。

## (h) 推迟件汇总裁定请求（协调者在 ib2 前裁定补迁窗口归属）

| 来源 | 数量 | 内容 | 请求 |
|---|---|---|---|
| K2 Brief §5 | 14 件 | semantic_model.go（+budget 切割前置）、handler/knowledgebase.go（rbac 去方法化前置）、knowledgebase.go 族 8 件（K4 knowledge_delete.go 前置）、kbshare.go（identity 哨兵前置）、tag.go/tag_access.go（测试拆分前置）、repository/kbshare.go（**计划空位**） | 裁定补迁窗口归属与前置兑现编排（K2 Brief §5 表逐行） |
| K4 Brief (f) | 17 件 + 环境阻断新增 | §3.3 表 17 文件（repository/knowledgebase.go + service 核心 11 + 独立面 3 + handler 2）+ 环境阻断件（repository 4 生产宿主原件删除、11 测试——其中 3 DDL 已获用户裁定放行待 world.run 通道、span_tracker/housekeeping 两对） | 同上（24 计划 §5.3 六项蓝本为执行依据；ENV-UNBLOCK 依赖闭包规则=生产先迁后测试，ib2 推迟批窗口执行） |
| 空位 | 1 件 | `internal/application/repository/kbshare.go`（K2.5 报告 §7.3 计划空位） | 建议并入 kbshare.go 补迁同窗 |

推迟件 396/B5 口径不变（Ruling DEFERRED-FILE-SPLIT 第 4 点）：仍计入未迁集，B5 清零时必须有去向；manifest/matrix 行保留未删。**HandlerSet/Dependencies 增补联动**：#16（handler/knowledge.go）与 K2 #2（handler/knowledgebase.go）补迁落位后，本门面按 (f) 增补字段并扩 `RegisterRoutes` 供给（装配面扩展，非契约变更；passbguard facade 计数句式 11/18/1 以 manifest integration_points 冻结值为准不受影响）。

其它 ib2 裁定请求（K2 Brief §8.4 遗留）：`app/graph.go` `NewGraphBuilder` 全仓零消费——若属死装配面请裁定导出或删除。
