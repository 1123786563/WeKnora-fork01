# Integration Brief — b2-k-wikifaq（23-knowledge-wikifaq）

> **状态：K3.3 产出（覆盖计划 §7 Task K3.3 (a)–(e) 全项 + 删除义务 2 核对结论）**。K3.4 在此基础上追加差分终稿与门禁收口，不另起文件。本 Brief 交付 b2-k-integration / ib2；执行属主：集成工程师（conventions §1.3、§10 Ruling 2026-09-25-CYCLE-FORCED-COMPOSITION 同律）。
> 行号基准：本节点分支 HEAD `b15dec368`（K3.2 审阅修复后）。文中行号均为 2026-09-25 在本 worktree 实测。

## (a) 装配切换表（ib2 逐行执行；container/router 集成工程师独占，conventions §3）

| # | 文件:行（当前实测） | 现状（经 shim） | ib2 切换目标 | 说明 |
|---|---|---|---|---|
| A1 | `internal/container/container.go:316` | `must(container.Provide(knowledgeWiki.NewWikiPageRepository))` | **已提前落地，无需再切**；IB2 转核验 | Ruling 2026-09-25-CYCLE-FORCED-COMPOSITION：repository 侧转发 shim 因 `wiki→agentruntime（prompt 常量）→agent/tools→repository` import 环不可编译（container.go:312-315 注释登记）。IB2 核验项：该行 + import 行 `knowledgeWiki`（container.go:93）确已指向 wiki 包，并删除 repository 侧 `NewWikiPageRepository` 转发（在 `wiki_k3_repo_compat.go`，见 (c) W3） |
| A2 | `internal/container/container.go:436` | `service.NewWikiPageService`（W2 转发） | `wiki.NewWikiPageService(..., wikiK3Seams 生产接线)` | W2 签名含 5 参；wiki 包构造器末参为 `wiki.Seams`。ib2 时 K2/K4 已落位，seam 改指落位包导出符号（见 (d) 同律）或直 import |
| A3 | `internal/container/container.go:437` | `service.NewWikiIngestService, dig.Name("wikiIngest")`（W2） | `wiki.NewWikiIngestService(..., spanTracker, seams)` | **`dig.Name("wikiIngest")` 必须保留**（同名多 provider 消歧，冻结面）。span 句柄：wiki 侧窄接口 + `wiki.NewSpan` 装卸；K4 `knowledge_span_tracker.go` 落位 process 包后，评估 `wiki.Span` 与 process Span 收口为直类型（届时 W2 `wikiK3SpanAdapter` 删除） |
| A4 | `internal/container/container.go:438` | `service.NewWikiLintService`（W2） | `wiki.NewWikiLintService(...)` | 返回 `*wiki.WikiLintService`；消费方 container.go:864 `NewWikiPageHandler` 形参同步 |
| A5 | `internal/container/container.go:725` | `handler.NewFAQHandler`（H2 转发） | `faq.NewFAQHandler(knowledgeService, kbService, parseTagIDs, requireTaskProgressTenant)` | 末两参为 helper seam（计划外断链，H2 现接宿主 `handler/knowledge.go:2546 parseCommaSeparatedTagIDs`、`handler/task_progress_auth.go:14 requireTaskProgressTenant`）；K4 handler/knowledge.go 落位后改指其导出形式或以 faq 包内私有实现收口 |
| A6 | `internal/container/container.go:864` | `handler.NewWikiPageHandler`（H1 wrapper） | `wiki.NewWikiPageHandler(...)`（去 wrapper） | 前置：rbac_lookups 收口（见 (c) H1 行）；`service.RecordWikiContentActivity` 参（H1 现传宿主导出函数）改指 K2 落位包导出 |
| A7 | `internal/router/routes_knowledge.go:143` | `func RegisterFAQRoutes(r *gin.RouterGroup, handler *handler.FAQHandler, g *rbacGuards)` | 形参类型改 `*faq.FAQHandler`（或改 import 别名） | 路由数 633 不变（conventions §8）；H2 为类型别名，router 测试 `&FAQHandler{}` 零字段字面量届时随之改指 faq 包 |
| A8 | `internal/router/routes_knowledge.go:309` | `func RegisterWikiPageRoutes(..., wikiHandler *handler.WikiPageHandler, ...)` | 形参类型改 `*wiki.WikiPageHandler` | H1 wrapper 删除批同窗 |
| A9 | `internal/container/recover_pending_wiki_tasks.go:75` | `json.Marshal(service.WikiIngestPayload{...})` | `wiki.WikiIngestPayload`（W1 类型别名改直引） | 同文件 `service.` 前缀其余引用不受影响；**hook 挂接行 `container.go:1058`（`must(container.Invoke(recoverPendingWikiTasks))`）零改动**；恢复语义（fail-closed :45-58、`asynq.TaskID("wiki-finalize-"+scope.ScopeID)`、MaxRetry 10、Timeout 60/30min）为冻结面不动 |
| A9′ | `internal/container/reset_pending_tasks_test.go:388/:390` | `map[string]service.WikiIngestPayload{}` 等 | 同步改 `wiki.WikiIngestPayload` | 测试文件（container 属主），ib2 与 A9 同窗，防悬空引用 W1 |
| A10 | `internal/router/task.go:124` | `errors.Is(e, service.ErrWikiIngestConcurrent)` | `errors.Is(e, wiki.ErrWikiIngestConcurrent)` | W1 是 `var` 别名（同一错误实例）；改直引后 `errors.Is` 语义不变。重试退避注释锚（task.go:117-124）不动 |
| A11 | `internal/handler/session/qa.go:205` | `h.resolveWikiFixerTenantScope(ctx, customAgent, tenantID, role, kbIDs)` | `wiki.ResolveBuiltinWikiFixerTenantScope(ctx, customAgent, tenantID, role, kbIDs, h.knowledgebaseService, h.kbShareService)` | B0.3 Step 3 去方法化收口（S1 删除批同窗）；两字段来自 `session/handler.go:30/:34`。qa.go 属 conversation 属主，本节点禁改（10-identity.md:79 同型推迟先例） |

注意：`container.go:2320/:2324` 的 `wiki.NewConnector` 属 docreader/connector 的同名 wiki 包（import 块可辨），与本域无关，ib2 勿误切。

## (b) 宿主调用点改写清单（§5.1 组 A 符号 + faq 面；ib2 属主：集成工程师）

W1（`internal/application/service/wiki_k3_compat.go`）转发的全部符号与其宿主调用点。ib2 逐行把调用点改为 wiki 包导出名后删除 W1；下表行号为 HEAD 实测。

| W1 现名（宿主 service 包） | wiki 导出名 | 宿主调用点（属主） |
|---|---|---|
| `previewText` | `wiki.PreviewText` | `extract.go:309`、`image_multimodal.go:280/:296`（K1→ingest）；`knowledge_process.go:1226/:1289/:1336/:1759/:2097`（K4→process） |
| `enqueueWikiIngestTrigger` | `wiki.EnqueueWikiIngestTrigger` | `knowledge_post_process.go:268/:435`（K4） |
| `enqueueWikiRetract` | `wiki.EnqueueWikiRetractWithError`（**导出名含 WithError 后缀**，计划 §5.1 名 `EnqueueWikiRetract` 未用） | `knowledge_delete.go:233`（K4） |
| `extractRealText` | `wiki.ExtractRealText` | `knowledge_post_process.go:729`（K4） |
| `newWikiIngestPendingOp` | `wiki.NewWikiIngestPendingOp` | `knowledge_post_process.go:317`（K4） |
| `minTextContentRunes`（var 双副本） | `wiki.MinTextContentRunes` | `knowledge_process.go:844/:847`（K4 读点）；宿主测试 `knowledge_summary_test.go`（写点） |
| `realTextRuneCount` | `wiki.RealTextRuneCount` | `knowledge_process.go:843/:942`（K4） |
| `uniqueWikiFolderIDs` | `wiki.UniqueWikiFolderIDs` | `knowledge_delete.go:236`（K4） |
| `EnqueueWikiIngest`（已导出） | `wiki.EnqueueWikiIngest` | `knowledge_clone_move.go:1237/:1284`（K4） |
| `ErrWikiIngestConcurrent`（var 别名） | `wiki.ErrWikiIngestConcurrent` | `router/task.go:124`（集成工程师文件，见 A10） |
| `WikiIngestPayload`（类型别名） | `wiki.WikiIngestPayload` | `container/recover_pending_wiki_tasks.go:75`（A9）、`container/reset_pending_tasks_test.go:388/:390`（A9′） |
| `WikiRetractPayload`（类型别名，计划外增量） | `wiki.WikiRetractPayload` | `knowledge_delete.go:233` 字面量构造（K4） |
| `WikiPendingOp`（类型别名，计划外增量） | `wiki.WikiPendingOp` | 宿主测试 `knowledge_move_wiki_test.go`、`knowledge_housekeeping_test.go`（K4 随迁面，ib2 由 K4 域处置） |
| `WikiDeletedTombstoneKey`（转发，计划外增量） | `wiki.WikiDeletedTombstoneKey` | `knowledgeService.cleanupWikiOnKnowledgeDelete`（K4 `knowledge_delete.go` 域，grep 实测同包直引） |
| `wikiDeletedTTL`（常量照录，计划外增量） | `wiki.WikiDeletedTTL` | `knowledge_delete.go:178`（K4 写墓碑 TTL） |
| `wikiTaskType`/`wikiTaskScope`/`WikiOpIngest`/`WikiOpRetract`（常量族，计划外增量） | `wiki.WikiTaskType`/`WikiTaskScope`/`WikiOpIngest`/`WikiOpRetract` | 宿主测试 `knowledge_housekeeping_test.go:105`（K4 随迁面） |

faq 面（D1 = `internal/application/service/knowledge_faq_k3_delegate.go`）：

| D1 现名（宿主 `*knowledgeService` 方法） | faq 导出名 | 宿主调用点（属主） |
|---|---|---|
| FAQ 接口面 15 方法（ListFAQEntries…UpdateLastFAQImportResultDisplayStatus，清单见 evidence §2.6(a)） | `*faq.Service` 同名方法 | 经 `interfaces.KnowledgeService` 接口分发：`router/task.go:277`、`sync_task.go:148`（worker 注册零改）、handler/knowledge.go FAQ 面、K4 各面 |
| `validateFAQKnowledgeBase` | `faq.(*Service).ValidateFAQKnowledgeBase` | `knowledgebase_access.go:42`（K2） |
| `buildFAQStatusSyncPlan` | `faq.(*Service).BuildFAQStatusSyncPlan`（+`faq.FAQStatusSyncPlan` 类型） | `knowledge_clone_move.go:700`（K4） |
| `indexFAQChunks` | `faq.(*Service).IndexFAQChunks` | `knowledge_clone_move.go:855`（K4） |
| `syncFAQChunkStatusBatch` | `faq.(*Service).SyncFAQChunkStatusBatch` | `knowledge_clone_move.go:885`（K4） |
| 测试垫片 6 符号（UpdateFAQEntryStatus/UpdateFAQEntryTag/resolveTagID/buildFAQTagResolver/faqImportCompletedOutcome/faqImportActivityDetails） | `faq` 包对应导出（含 `FAQTagResolver`/`BuildFAQTagResolver`/`ResolveTagID`/`FaqImportCompletedOutcome`/`FaqImportActivityDetails`） | 留驻宿主测试（`kb_activity_test.go` 等，Ruling 2026-09-24-TEST-SUPPORT-SHIM 垫片 `knowledge_faq_k3_test_support_shim_test.go` 消费）——K2/K4 域改写测试后垫片删除 |

W3（`internal/application/repository/wiki_k3_repo_compat.go`）面：

| W3 现名 | 归属 | 宿主消费点 |
|---|---|---|
| `EscapeLikePattern`（原 `escapeLikePattern`，repository/wiki_page.go:1258 拆出物理留驻） | ib2 评估迁往 wiki 或共享 util（文件头裁定） | `tenant_member.go`（identity 属主，同包裸引 `escapeLikePattern` var） |
| 4 哨兵 `ErrWikiPageNotFound`/`ErrWikiFolderNotFound`/`ErrWikiFolderConflict`/`ErrWikiFolderNotEmpty`（+`ErrWikiPageConflict`） | wiki 域语义；物理留驻 repository | `agentruntime/agent/tools/wiki_route_resolver.go:136`、`wiki_tools.go:644`（B3 迁移面，经 `repository.ErrWikiPageNotFound` errors.Is）；wiki 包以导出别名引用同一实例（`wiki.ErrWikiPageNotFound = repository.ErrWikiPageNotFound` 族）。ib2 与 B3 agentruntime 迁移同窗收口：哨兵单点定 wiki 包（或按 K 计划归属裁定），repository 侧转发行删除 |

## (c) 兼容文件删除批（7 文件；计划 §6.2 列 6 + 实施增量 W3）+ rbac_lookups 收口

全部文件头已注「Pass B (23-knowledge-wikifaq) 过渡 shim — 删除点 ib2」并登记 `docs/architecture/moves/knowledge.yaml` legacy_files 行（:343-369，7 行）。

| id | 文件 | 保护的不改方 | ib2 删除前置 |
|---|---|---|---|
| W1 | `internal/application/service/wiki_k3_compat.go` | (b) wiki 表全部宿主调用点 | (b) 表逐行改写完成（K1/K4 落位后其文件内改指 wiki 导出；router/container 见 (a)） |
| W2 | `internal/application/service/wiki_k3_ctor_compat.go` | `container.go:436/:437/:438` dig Provide | (a) A2–A4 切换；`wikiK3SpanAdapter` 待 K4 span 落位收口（A3 说明） |
| W3（计划外增量） | `internal/application/repository/wiki_k3_repo_compat.go` | `tenant_member.go`（identity）、agentruntime wiki 工具（B3）、wiki 包哨兵别名 | (b) W3 表：escapeLikePattern 收口裁定 + 哨兵单点化 + B3 调用点改写 |
| H1 | `internal/handler/wiki_page_k3_compat.go` | `routes_knowledge.go:309`、`container.go:864`、**rbac_lookups.go:105/:183 与 rbac_lookups_test.go:327/:344/:356（10-identity 推迟件，字节不变）** | (a) A6 + rbac_lookups 收口（下段） |
| H2 | `internal/handler/faq_k3_compat.go` | `routes_knowledge.go:143`、`container.go:725`、router 测试 `&FAQHandler{}` 字面量 | (a) A5/A7；helper seam 两参改指 K4 落位导出 |
| S1 | `internal/handler/session/wiki_fixer_scope_compat.go` | `handler/session/qa.go:205`（conversation 属主） | (a) A11 |
| D1 | `internal/application/service/knowledge_faq_k3_delegate.go` | 冻结接口 FAQ 面（编译器强制）、K2/K4 四调用点、worker 注册 | K4 `knowledge.go` 落位 process 后，KnowledgeService 实现体的 FAQ 面归宿按 24 计划收口（接口不变， delegates 换指向） |

**rbac_lookups 收口（H1 前置）**：`internal/handler/rbac_lookups.go:183` 在宿主 `*WikiPageHandler`（现为 H1 wrapper）上定义方法 `KBCreatorLookupFromKBPath`（读 wrapper 的 `kbService` 字段）；`:105` 以方法表达式 `(*WikiPageHandler)(nil).KBCreatorLookupFromKBPath` 断言 `middleware.CreatorLookup`；`rbac_lookups_test.go:327/:344/:356` 以字面量 `&WikiPageHandler{kbService: …}` 构造。三处均 10-identity 推迟件、本节点与 ib2 前字节不变。ib2 切 A6 时集成工程师：把方法定义移到 `*wiki.WikiPageHandler`（或以自由函数/接口断言重写 `:105`），测试字面量改 `&wiki.WikiPageHandler{...}` 等价构造，然后删 H1。10-identity.md:155 推迟件口径：该文件的最终落位随 10-identity 残差裁定，ib2 需与其协调同一窗口。

## (d) faq 包 seam 生产接线表（seam → 定义方符号 → 落位包）

当前生产接线全部在 D1 `faqSvc()`（闭包捕获宿主 receiver / 同包函数值）。K2/K4 合并后按 20 计划 §6.1 R1.3 shim 状态核对：K2/K4 落位包导出符号就绪后，闭包目标改指落位包（或 wiki/faq 直接 import 落位包，届时按 R1.3 判定 shim 是否可删）。

| faq.Seams 字段（service.go:41-76） | 现接线目标（宿主符号，file:line，属主） | ib2 后目标（落位包） |
|---|---|---|
| `RecordKBActivity` | `recordKBActivity`（kb_activity.go:95，K2） | `retrieval/app` 导出（20 计划 §6.2 组 B 行裁定：K2 按 R1 导出 + 宿主 shim） |
| `KBActivityTrigger` | `kbActivityTrigger`（kb_activity.go:36，K2） | 同上 |
| `WithKBActivityTask` | `withKBActivityTask`（kb_activity.go:27，K2） | 同上 |
| `KBActivityAppendSampleTitles` | `kbActivityAppendSampleTitles`（kb_activity.go:49，K2） | 同上 |
| `ResolveKBReadTenant` | `resolveKBReadTenant`（knowledgebase_access.go:21，K2；经 `faq.KBShareLookup` 镜像接口适配） | 同上 |
| `WritableFAQKnowledgeBase` | `(*knowledgeService).writableFAQKnowledgeBase`（knowledgebase_access.go:38，K2 方法；闭包捕获宿主 receiver，链路终止于 `faq.ValidateFAQKnowledgeBase`，无递归） | K2 落位后改指其导出包装；或随 KnowledgeService 实现体归宿一并收口 |
| （handler 侧）`parseTagIDs` | `parseCommaSeparatedTagIDs`（handler/knowledge.go:2546，K4） | `process` 落位导出 |
| （handler 侧）`requireTaskProgressTenant` | `requireTaskProgressTenant`（handler/task_progress_auth.go:14，留驻 handler 宿主） | 按其文件归属（非 K 计划 18 文件）裁定；最小动作 = faq_handler 构造参保留直引 |

wiki 包 `wiki.Seams`（seams.go:29-53）同律（W2 `wikiK3Seams()` 现接线）：`ResolveDeadSlug`→`resolveDeadSlug`（slug_fuzzy.go:91，K2→retrieval/app）；`FinalizeSubtaskDetached`→`finalizeSubtaskDetached`（knowledge.go:235，K4→process）；`IsLikelyRateLimitError`→`isLikelyRateLimitError`（knowledge_process.go:3902，K4→process）；`RemoveSourceRef`→`removeSourceRef`（knowledge_delete.go:346，K4→process）；`RecordWikiContentActivity`→宿主导出函数（kb_activity.go:181，K2；20 计划组 C :164 原裁定「经 K2 落位包导入，无需 seam」，实施选 seam 形态——ib2 核对时可改直 import 落位包）；`IsKnowledgeBaseNotFound`→`errors.Is(err, repository.ErrKnowledgeBaseNotFound)` 闭包（计划 §10.4 二选一的 seam 路径；ib2 若哨兵随 K4 落位可改直引）。

计划 §5.2 列出但实测未建的 seam（evidence §2.4-3、§1.6-5）：`hash`（K2 semantic_model_capability.go:147——调用点实为同名局部变量/注释，零命中）、`contains`（knowledge_span_tracker.go:702——wiki_ingest.go:2169 原判读为假阳性，零命中）。ib2 无对应动作。

## (e) 模块内同包合并就地重命名清单（实际发生项）

R1 导出改名（迁移 commit b981d13e3 / f6dfba041 内，行为零变化；调用点已随迁直改）：

| 旧名（宿主包内） | 新名（模块包） | 备注 |
|---|---|---|
| `previewText` | `wiki.PreviewText` | |
| `enqueueWikiIngestTrigger` | `wiki.EnqueueWikiIngestTrigger` | |
| `enqueueWikiRetract` | `wiki.EnqueueWikiRetractWithError` | 导出名带 WithError 后缀（与包内既有错误变体区分）；宿主 W1 转发名不变 |
| `extractRealText` | `wiki.ExtractRealText` | |
| `newWikiIngestPendingOp` | `wiki.NewWikiIngestPendingOp` | |
| `minTextContentRunes` | `wiki.MinTextContentRunes` | var；宿主留双副本（W1），ib2 收口单点 |
| `realTextRuneCount` | `wiki.RealTextRuneCount` | |
| `uniqueWikiFolderIDs` | `wiki.UniqueWikiFolderIDs` | |
| `wikiTaskType`/`wikiTaskScope`/`wikiOpIngest`/`wikiOpRetract`/`wikiDeletedTTL` | `wiki.WikiTaskType`/`WikiTaskScope`/`WikiOpIngest`/`WikiOpRetract`/`WikiDeletedTTL` | 常量族导出（wiki_ingest.go:173/:200/:204/:315-316） |
| `resolveBuiltinWikiFixerTenantScope` | `wiki.ResolveBuiltinWikiFixerTenantScope` | B0.3 Step 3 去方法化（计划 §5.4） |
| `validateFAQKnowledgeBase` | `faq.(*Service).ValidateFAQKnowledgeBase` | |
| `buildFAQStatusSyncPlan` | `faq.(*Service).BuildFAQStatusSyncPlan` | + 类型 `faqStatusSyncPlan`→`faq.FAQStatusSyncPlan`（faq_clone_sync.go:16，字段 Pairs/SrcByID/DstByID 导出面不变） |
| `indexFAQChunks` | `faq.(*Service).IndexFAQChunks` | |
| `syncFAQChunkStatusBatch` | `faq.(*Service).SyncFAQChunkStatusBatch` | |
| `UpdateFAQEntryStatus`/`UpdateFAQEntryTag`（已导出方法，接收者换 `*Service`） | 同名 `faq.(*Service)` | |
| `buildFAQTagResolver`/`resolveTagID`/`faqImportCompletedOutcome`/`faqImportActivityDetails` | `faq.BuildFAQTagResolver`/`ResolveTagID`/`FaqImportCompletedOutcome`/`FaqImportActivityDetails` | R1 导出，供宿主测试垫片消费（Ruling TEST-SUPPORT-SHIM） |

文件级更名（防同目录同名冲突，计划 §6.1 预定）：`repository/wiki_page.go`→`wiki/wiki_page_repository.go`；`handler/wiki_page.go`→`wiki/wiki_page_handler.go`；`handler/faq.go`→`faq/faq_handler.go`。接收者改写：`(s *knowledgeService)`→`(s *Service)` 57 处（evidence §2.7）。无其他机械重命名（K3.1 Step 3 重声明检查零命中，evidence §1.5）。

## 删除义务 2 核对结论（docparser 别名消费；brief knowledge-wikifaq.md 义务 2）

实测命令（2026-09-25，本 worktree HEAD）：

```
grep -rn 'IsImageFormat\|IsSimpleFormat\|SimpleFormatReader' internal/knowledge/wiki/ internal/knowledge/faq/
# 退出码 1（零命中）
```

结论：本节点 18 迁移文件（wiki 12 + faq 6）对 docparser `IsImageFormat`/`IsSimpleFormat`/`SimpleFormatReader` **零消费**，删除义务 2 对本节点无回收动作。防后续误判：wiki 摄取管线消费 docparser 的面是 `StripMarkdownImages` 等（如 wiki_ingest 引用面），不属 docparser 别名删除义务范围。

## 其它 ib2 登记（K3.3 汇总）

1. **import 例外删除批**：wiki 侧 11 条 + faq 侧 2 条 file→package 精确豁免（`tools/architectureguard/check.go` importExceptions :107-198；exception-ledger exc-0106..0116（wiki）+ exc-0117/0118（faq），owner=23-knowledge-wikifaq，RemoveAt=ib2）。涉及被导入包：`policy/access`、`agentruntime/agent`、`agentruntime/modelcontext`、`airesource/models/chat`、`airesource/models/embedding`。ib2 判定：K2/K4 落位后这些预存横向耦合是否已被模块间公共面取代，否则按裁定延续或改 seam。例外计数基线 105→118 已按 Ruling 机械修正并登记（§8 基线）。
2. **测试支撑垫片删除批**（Ruling 2026-09-24-TEST-SUPPORT-SHIM）：`wiki_k3_test_support_shim_test.go`（wikiGuardTaskQueue 最小重建）、`knowledge_faq_k3_test_support_shim_test.go`（6 符号委托）、宿主 `wiki_k3_compat_test.go` / `wiki_k3_span_adapter_test.go`（W1/W2 伴生测试，随 shim 删除）。IB2 先到先删，最迟 B5。
3. **双副本收口**：`minTextContentRunes`（W1 副本 vs `wiki.MinTextContentRunes`，宿主测试改写副本侧）、`faqImportBatchSize`（宿主 knowledge.go:96 未引用常量 vs faq/service.go 照录，K4 迁出时收口单点）。
4. **legacy/README.md 行同步**：K3.1 同 commit 删除 wiki 12 行；faq 6 行为 K3.2 机械遗漏，K3.3 已就地补删（conventions §10 共同原则：机械缺口发现节点就地补齐 + 留痕）。
5. **manifest 终态**：`docs/architecture/moves/knowledge.yaml` legacy_files 18 行已删（K3.1 12 行 + K3.2 6 行，与物理迁移同 commit，Ruling LEGACY-ROW-OWNERSHIP）；新增 7 行过渡登记（W1/W2/W3/H1/H2/S1/D1，reason 注明 ib2 删除）。ownership-matrix.yaml 18 行删除归 ib2（delete_barrier 口径，本节点只读）。
6. **container.go 提前落地核验**：见 (a) A1——本节点在分支上改动了 container.go 仅 1 处 provider + import（Ruling CYCLE-FORCED-COMPOSITION 授权范围），IB2 核验后无残留动作；集成分支 container.go 其余行本节点零触碰。
