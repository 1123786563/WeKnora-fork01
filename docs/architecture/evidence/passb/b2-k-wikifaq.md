# Evidence — b2-k-wikifaq（Pass B 23-knowledge-wikifaq）

> 状态：**终稿**（K3.1/K3.2 差分 §1–§2 + K3.4 §3 收口：计划 §8 四行差分终表、hook 恢复手工差分双跑、T0 vs T1/T2 终态对照、门禁终跑）。节点基线 BASE=7ffaf6cc4（K3.2 起点；K3.1 段基线 4649630df）；节点分支 codex/passb-b2-k-wikifaq；差分对照基线（节点工作起点）= 基线对齐 merge `1c9d812d0`。

## 0. T0 旧实现基线（迁移前，2026-09-25 实跑）

| 命令 | 结果 | 计数 |
|---|---|---|
| `go test -count=1 ./internal/application/repository/ -run 'Wiki'` | ok | 2 PASS |
| `go test -count=1 ./internal/application/service/ -run 'Wiki\|Linkify\|Slug'` | ok | 85 PASS / 0 FAIL / 0 SKIP |
| `go test -count=1 ./internal/handler/session/ -run 'WikiFixer\|ResolveBuiltin'` | ok | 5 PASS |
| `go test -count=1 ./internal/handler/session/... ./internal/application/service/ -run 'Wiki'` | ok（两包） | service 侧 48 PASS |

## 1. K3.1 差分（Wiki 摄取/Finalize/页面操作 + wiki fixer，双跑比对）

### 1.1 用例清单与双跑结果

迁移后命令（新实现同用例运行）：

| 命令 | 结果 | 计数 | 与 T0 比对 |
|---|---|---|---|
| `go test -count=1 ./internal/knowledge/wiki/...` | ok | 130 PASS / 0 FAIL / 0 SKIP | 集合 = T0 基线 85（repo 2 + service 83）+ session 5 + 测试类目展开（`-v` 顶层 RUN 计数口径：T0 顶层 92 → 迁入后同断言全 PASS；TestRepairContentLinks 移至宿主真实 seam 双跑，见 §1.3） |
| `go test -count=1 ./internal/handler/session/... ./internal/application/service/ -run 'Wiki'` | ok | — | K4 留驻（knowledge_post_process_wiki_enqueue_test.go、knowledge_move_wiki_test.go、knowledge_summary_test.go、knowledge_housekeeping_test.go）经 W1/W2 PASS |
| `go test -count=1 ./internal/knowledge/kbfreeze/` | ok | 2 守卫 | R0 冻结面零违反 |
| `go test -count=1 ./internal/application/repository/`（全包） | ok（357s） | — | escapeLikePattern/哨兵留驻拓扑下 tenant_member 等全量绿 |
| `make verify-module-moves` | `modulemove: OK (16 manifests verified)` | — | 12 wiki 行已删、5 兼容行已登记 |
| `make check-backend-architecture` | `OK (0 violations)`，`total=633 \| redis=23 lite=23 \| hooks=58` | — | 路由/worker/hook 计数与 pass-a-acceptance 台账一致（§8 三方一致） |

### 1.2 TypeWikiIngest / TypeWikiFinalize 状态机逐用例等价

- **claim/peek/finalize 合并（`wiki-finalize-<kbID>` TaskID 去重）**：`TestEnqueueWikiFinalizeOnlySchedulesAcceptedRows`（accepted=true 断言恰 1 个 TypeWikiFinalize 任务入队）+ `TestProcessWikiFinalizeDefersFolderPruneWhileIngestIsPending`（ingest 未排空→不剪枝、durable prune 行保留、恰 1 个重排任务）双跑 PASS；`scheduleFinalize` 的 `asynq.TaskID("wiki-finalize-"+kbID)` 与 ErrTaskIDConflict 吸收分支未改动（纯移动，diff 仅 package/import）。
- **5-docs-per-batch fan-out**：`WikiMaxDocsPerBatch=5` 常量与 `claimPendingList`/`peekPendingList` 调用点零变化（wiki_ingest.go 常量区逐字保留）。
- **MaxRetry 10 / Timeout 60/30 分钟**：`wikiIngestMaxRetry=10`、`asynq.Timeout(60*time.Minute)`（ingest）/`30*time.Minute`（finalize）逐字保留；`TestIsTransientLLMError_*` 5 用例（含 RateLimit403 网关语义）PASS。
- **ErrWikiIngestConcurrent 指数退避重试语义**：`TestWikiDeletedKnowledgeBaseCleanupFailureRetries`、guard 双任务类型 drain 用例 PASS；哨兵经 W1 `var ErrWikiIngestConcurrent = wiki.ErrWikiIngestConcurrent` 保持同一实例（router/task.go:124 `errors.Is` 语义不变——同变量引用）。

### 1.3 TestRepairContentLinks 宿主真实 seam 双跑（跨包迁移声明）

原 service/wiki_page_test.go:176-228（断言 resolveDeadSlug 的 bigram 模糊复活真实行为）。迁移后落在 `internal/application/service/wiki_k3_compat_test.go`（宿主包可引用未导出 `resolveDeadSlug` 与 W2 `wikiK3Seams()` 生产同构接线），驱动 `wiki.NewWikiPageService(..., wikiK3Seams()).RepairContentLinks`：

```
=== RUN   TestRepairContentLinks
--- PASS: TestRepairContentLinks (0.00s)   [3/3 子用例 PASS：mangled-uuid bigram 修复 / live links 不动 / 不可解析死链保留]
```

被测实现 = wiki 包生产代码路径；seam 目标函数 = 迁移前同包直引的同一 `resolveDeadSlug`（slug_fuzzy.go:91）。等价成立。

### 1.4 wiki fixer 共享租户作用域

`wiki_fixer_scope_test.go` 5 用例随迁 package wiki，调用点改导出名 `ResolveBuiltinWikiFixerTenantScope`，断言逐字未动：5/5 PASS（T0 基线 5 PASS）。`qa.go:205` 经 S1 薄委托同一实现——方法体仅转发，逐分支行为不变。

### 1.5 纯移动验收（spec §14.2）

`git diff --summary` 全部识别 rename（26 个 R 标记：12 生产 + 14 测试，含 wiki_page.go→wiki_page_repository.go、handler/wiki_page.go→wiki_page_handler.go 两处防冲突更名）；生产文件 diff 除 package 子句、import 块、§5.1 导出改名（8 符号 + 2 常量族）、seam 接线点（6 处调用改 `s.seams.X`/`s.prompts 不涉`）外零函数体变化。逐文件 `diff <(git show BASE:<old>) <new>` 复核：wiki_ingest.go 86 行差异全部落于上述类别（详见审查包）；dedup/cite/linkify/taxonomy/lint/slug_handles 仅 package 行差异。

### 1.6 已知拓扑裁定（非行为差异，登记供 IB2 收口）

1. **Ruling 2026-09-25-CYCLE-FORCED-COMPOSITION**：container.go:311 provider 目标切至 `knowledgeWiki.NewWikiPageRepository`（import 环强制，Brief (a) 同款切换提前落地；IB2 转核验项）。
2. **minTextContentRunes 双副本**：wiki.MinTextContentRunes 与 W1 `minTextContentRunes` 初始值同为 10；生产零改写；宿主测试（knowledge_summary_test）改写副本与 K4 读点同侧，行为不变；IB2 K4 迁出时收口单一变量。
3. **Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY**：wiki 包 11 条 file→package 豁免登记（check.go importExceptions + exception-ledger exc-0106..0116，owner=23-knowledge-wikifaq，RemoveAt=ib2）；例外计数 105→116 机械修正。
4. **Ruling 2026-09-24-TEST-SUPPORT-SHIM**：宿主 `wiki_k3_test_support_shim_test.go` 垫片（wikiGuardTaskQueue 最小重建，remove_at ib2）。
5. **§5.2 实测修订**：`contains` seam 无调用点（grep 零命中）未建；`IsKnowledgeBaseNotFound` seam 替代 wiki→repository 哨兵 import（计划 §10.4 二选一备选路径，已选 seam 注入）。

## 2. K3.2 差分（FAQ 域迁移 + 冻结端口委托，2026-09-25 实跑）

### 2.1 T0 旧实现基线（FAQ 面，迁移前）

| 命令 | 结果 | 计数 |
|---|---|---|
| `go test -count=1 ./internal/application/service/ -run 'FAQ'` | ok | 18 顶层 PASS / 0 FAIL / 0 SKIP |
| `go test -count=1 ./internal/handler/ -run 'FAQ'` | ok | 3 顶层 PASS（faq_enabled_filter_test.go） |

T0 用例集合：service 18 = `TestFAQImportCompletedOutcome`、`TestFAQImportActivityDetails`、`TestAcquireFAQCreateGuard{RejectsConcurrentSameQuestion,IsolatesUnrelatedCreates,FallsBackWithoutRedis}`、`TestReplaceKnowledgeFileRejectsFAQKnowledgeBase`、`TestFAQAndTagWritesRejectUnscopedServiceCalls`、`TestFAQBatchPreflightRejectsBeforeAnyWrite`、`TestFAQDeleteValidatesEntireSelectionAndParents`、`TestFAQFieldGroupsAndExplicitUpdatesPreservePrecedence`、`TestFAQTagOnlyBatchUsesValidatedPlanAndSynchronizesIndex`、`TestFAQImportRejectsForeignTagsBeforeEnqueue`、`TestFAQImportWorkerRejectsMismatchedScopeBeforeSideEffects`、`TestSharedFAQWriteLoadsOwnerTenantInfoWithoutReplacingCaller`、`TestApplyFAQPostProcessing_PropagatesError`、`TestFAQCloneDoesNotInheritTransferState`、`TestSemanticScopeTransferFAQFailsBeforeCheckpoint`、`TestBuildSearchTargets_FAQTagScopeKeepsIndexTagFilter`；handler 3 = `TestParseOptionalFAQEnabled`、`TestFAQListEntriesPassesEnabledFilter`、`TestFAQListEntriesRejectsInvalidEnabledFilter`。

### 2.2 迁移后双跑结果

| 命令 | 结果 | 计数 | 与 T0 比对 |
|---|---|---|---|
| `go test -count=1 ./internal/knowledge/faq/` | ok | 8 顶层 PASS / 0 FAIL / 0 SKIP | = 3 守卫用例 + 3 filter 用例（同用例同断言随迁）+ 2 个 K3.2 Step 2 新锚定（TestNewService*，构造器接线/指针共享）；filter 第三用例断言面见 §2.4 |
| `go test -count=1 ./internal/application/service/ -run 'FAQ'` | ok | 15 顶层 PASS / 0 FAIL / 0 SKIP | = T0 18 − 随迁 3 守卫用例（精确差集，见 §2.5） |
| `go test -count=1 ./internal/application/service/ -run 'FAQ\|Knowledge'` | ok | — | K2/K4 留驻面（含 clone/move、semantic scope、replace）经 D1 委托 PASS |
| `go test -count=1 ./internal/handler/... ./internal/router/...` | ok（4 包） | — | H2 类型别名下 routes 注册与 `&handler.FAQHandler{}` 零字段字面量合法 |
| `go build ./...` | exit 0 | — | knowledgeService 仍实现 interfaces.KnowledgeService（冻结端口满足性=编译器强制；接口零改动） |
| `go vet ./internal/knowledge/faq/... ./internal/application/service/` | exit 0 | — | sync.Map 指针共享无复制告警 |
| `go test -count=1 ./internal/knowledge/...` | ok（全域） | — | 含 kbfreeze 2 守卫：faq 包无影子类型（R0） |
| `make verify-module-moves` | `modulemove: OK (16 manifests verified)` | — | faq 侧 6 legacy 行已删、D1/H2 已登记 |
| `make check-backend-architecture` | `OK (0 violations)`，`total=633 \| redis=23 lite=23 \| hooks=58` | — | 计数奇偶不变（§8 三方一致） |

### 2.3 TypeFAQImport 处理路径逐用例等价

- **dry-run 零写入**：`TestFAQImportRejectsForeignTagsBeforeEnqueue`（foreign tag 在 enqueue 前拒绝）+ `TestFAQImportWorkerRejectsMismatchedScopeBeforeSideEffects`（租户 7/8 两分支：scope 不匹配 → `asynq.SkipRetry` 且 chunk 零写入）双跑 PASS；`UpsertFAQEntries` 的 DryRun 分支与 `ProcessFAQImport` 的 finalize 路径为纯移动（diff 仅 package/接收者/seam 前缀）。
- **进度 memFAQProgress 跨调用共享（指针）**：宿主 D1 `faqSvc()` 每次以 `&s.memFAQProgress`/`&s.memFAQRunningImport` 重建 `*faq.Service`——指针共享使 Lite 模式回落状态跨重构造不丢；`TestNewServiceWiresDependencies` 锚定指针直传语义，`go vet` 无复制告警；Redis 路径（`getFAQImportRunningKey`/`faqImportProgressTTL=3h`/SetNX-Del 清理链）逐字未动。
- **失败 CSV 生成一致**：`generateFailedEntriesCSV`（BOM+8 列表头+csvEscape）纯移动；`TestFAQImportCompletedOutcome`/`TestFAQImportActivityDetails`（kb_activity_test.go，经垫片委托）PASS。
- **断点续跑/幂等**：`existingProgress.ValidEntryIndices` 复用、`DeleteUnindexedChunks` 清理、completed 幂等返回、Replace→Append 重试切换（executeFAQImport/ProcessFAQImport）纯移动。
- **executeFAQImport @ knowledge_faq_import.go:1373**：接收者改写外零函数体变化（git rename 相似度 96%）。

### 2.4 迁移适配（计划外、逐条等价论证）

1. **filter 第三用例断言面迁移**：原经 `middleware.ErrorHandler()` 断言 HTTP 400；faq 包测试 import middleware 构成测试期 import 环（middleware→service→faq）。改为错误边界断言 `errors.IsAppError(...).HTTPCode == 400`——ErrorHandler 对 AppError 原样渲染 HTTPCode（middleware/error_handler.go:22-27），可观察契约等价。
2. **接口 FAQ 面第 15 方法**：计划 §4 表列 14；编译器枚举出 `UpdateLastFAQImportResultDisplayStatus`（interfaces/knowledge.go:235）——按 K3.2 Step 4 预定机制补一行委托并补录本表（§2.6(a)）。
3. **hash seam 不需要**：计划 §5.2 列 `hash`（semantic_model_capability.go:147）调用点 :1177/:1221-1224 实为同名局部变量 `hash := types.CalculateFAQContentHash(...)` 与注释文本，精确 grep `(^|[^.\w])hash\(` 对 K2 hash 函数零命中——未建 seam（计划实测偏差，报告上报）。
4. **faqImportBatchSize 常量**：定义在 K4 knowledge.go:96（禁改文件），唯一消费者随迁——faq/service.go 照录声明 `const faqImportBatchSize = 50`（值同源；宿主声明留驻暂成未引用常量，合法；IB2 K4 迁出时收口）。
5. **handler 双 helper seam**：`parseCommaSeparatedTagIDs`（knowledge.go:2546，K4）/`requireTaskProgressTenant`（task_progress_auth.go:14）为计划未列的 faq.go 断链——按 R2 消费侧 seam 处理（FAQHandler 构造注入，H2 接线宿主现行函数）；未接线回落：tag 解析→nil（与空查询同行为）、租户校验→fail-closed 未授权（生产恒接线）。
6. **测试支撑垫片扩展**：Ruling 2026-09-24-TEST-SUPPORT-SHIM 垫片新增 6 符号委托（UpdateFAQEntryStatus/UpdateFAQEntryTag/resolveTagID/buildFAQTagResolver/faqImportCompletedOutcome/faqImportActivityDetails，消费方全部为留驻宿主测试）；faq 侧对应 R1 导出 FAQTagResolver/BuildFAQTagResolver/ResolveTagID/FaqImportCompletedOutcome/FaqImportActivityDetails。
7. **Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY**：faq/knowledge_faq_import.go → airesource/models/embedding 与 policy/access 两条精确豁免（check.go + exception-ledger exc-0117/0118，RemoveAt=ib2）；例外计数 116→118 机械修正。

### 2.5 集合一致性（逐用例）

T0 service 18 − {TestAcquireFAQCreateGuard×3}（随迁 faq 包，3/3 PASS）= 迁移后 service 15，逐一比对无缺漏、零新增失败、零 Skip；handler 3 → faq 包 3/3 PASS（同文件同断言，除 §2.4-1 的等价断言面）。守卫用例集合在 faq 包与迁移前逐字同构（newGuardService 构造面等价：`&knowledgeService{redisClient}` → `NewService(Deps{RedisClient})`）。

### 2.6 冻结端口委托清单（D1）

- (a) 接口面 15 方法：ListFAQEntries、UpsertFAQEntries、CreateFAQEntry、GetFAQEntry、UpdateFAQEntry、AddSimilarQuestions、UpdateFAQEntryFieldsBatch、DeleteFAQEntries、SearchFAQEntries、ExportFAQEntries、ExportFAQEntriesJSON、UpdateFAQEntryTagBatch、ProcessFAQImport、GetFAQImportProgress、UpdateLastFAQImportResultDisplayStatus（末项为计划表外补录）。
- (b) 未导出 4 方法：validateFAQKnowledgeBase（K2 knowledgebase_access.go:42）、buildFAQStatusSyncPlan（K4 :700）、indexFAQChunks（K4 :855）、syncFAQChunkStatusBatch（K4 :885）；faq 侧 R1 导出 ValidateFAQKnowledgeBase/BuildFAQStatusSyncPlan(+FAQStatusSyncPlan 类型)/IndexFAQChunks/SyncFAQChunkStatusBatch。
- (c) faqSvc()：17 依赖字段 + 6 seam 闭包（recordKBActivity/kbActivityTrigger/withKBActivityTask/kbActivityAppendSampleTitles 直引同包函数值；ResolveKBReadTenant 经 faq.KBShareLookup 镜像接口适配；WritableFAQKnowledgeBase 捕获宿主 receiver，链路终止于 ValidateFAQKnowledgeBase，无递归）。
- worker 注册零变化：router/task.go:277、sync_task.go:148 经 `params.KnowledgeService.ProcessFAQImport` 接口调用 → D1 委托 → faq 实现。

### 2.7 纯移动验收（spec §14.2）

`git diff --cached -M --summary`：8 个 rename 全识别（faq_clone_sync 84%、knowledge_faq 93%、knowledge_faq_batch 96%、knowledge_faq_create_guard 98%、knowledge_faq_create_guard_test 84%、knowledge_faq_import 96%、handler/faq.go→faq_handler.go 86%、faq_enabled_filter_test 70%）。生产文件 diff 类别：package 子句、接收者 `(s *knowledgeService)`→`(s *Service)`（57 处）、R1 导出改名（6 方法 + 2 类型 + 2 纯函数）、seam 调用前缀（22 处：writable 10 / recordKBActivity 7 / kbActivityTrigger 2 / withKBActivityTask 1 / appendSampleTitles 1 / resolveKBReadTenant 1）、faq_handler 双 helper seam 点（2）。

### 2.8 审阅轮 R1 修复（review finding，2026-09-25）

- **finding（critical）**：knowledge_faq_import.go `deleteFAQChunkVectors` 的 `knowledge.StorageSize` 扣减块在迁移 commit f6dfba041 中被误移入 `AdjustStorageUsed` 的 `err == nil` 分支内（BASE：tenant 记账失败仍扣减并经 UpdateKnowledge 持久化）——手写转录 2882 行文件时引入的大括号层级错误，未申报的行为变化（纯移动违规）。
- **修复**：恢复 BASE 结构（`git show BASE:internal/application/service/knowledge_faq_import.go | sed -n '2160,2179p'` 与 HEAD 逐行比对确认）；fix commit 见分支 log。
- **全面自查（防同类）**：对 5 个 service 侧迁移文件做「HEAD 逆向机械变换（package/接收者/seam 前缀/R1 改名）→ 与 BASE diff」——knowledge_faq_batch.go、knowledge_faq_create_guard.go 0 差异；faq_clone_sync.go/knowledge_faq.go/knowledge_faq_import.go 仅剩 22 行 R1 导出注释；faq_handler.go 仅剩 seam 机制声明差异（字段/构造器/访问器/2 调用点）；knowledge_faq_create_guard_test.go 仅剩构造面等价改写（`&knowledgeService{redisClient}`→`NewService(Deps{RedisClient})`）+注释。**无其他行为差异。**
- **修复后复跑**：`go build ./...` exit0；`go test -count=1 ./internal/knowledge/faq/...` ok；`go test -count=1 ./internal/application/service/ -run 'FAQ|Knowledge'` ok；`go test -count=1 ./internal/knowledge/...` exit0；`make verify-module-moves` OK；`make check-backend-architecture` OK（633/23+23/58 不变）；`gofmt -l` 空。

## 3. K3.4 终稿 — 计划 §8 高风险差分四行终表 + hook 恢复手工差分（2026-09-25 实跑）

### 3.1 四行差分终表

| 面（计划 §8） | 锚定用例清单 | 双跑命令与结果 | 等价比对结论 |
|---|---|---|---|
| ① `TypeWikiIngest`/`TypeWikiFinalize` 状态机 | `TestEnqueueWikiFinalizeOnlySchedulesAcceptedRows`、`TestProcessWikiFinalizeDefersFolderPruneWhileIngestIsPending`、`TestIsTransientLLMError_*` 5 用例、`TestWikiDeletedKnowledgeBaseCleanupFailureRetries`、guard 双任务类型 drain 用例（wiki_ingest_test.go / wiki_ingest_retry_test.go / wiki_ingest_dedup_test.go / wiki_deleted_kb_guard_test.go） | T0：§0（repo 2 + service 85/92 顶层口径）；T1/T2 终态：`go test -count=1 -v ./internal/knowledge/wiki/` → **132 顶层 PASS / 0 FAIL / 0 SKIP**（exit 0） | **等价**（§1.2 逐用例）：`wiki-finalize-<kbID>` TaskID 去重、5-docs-per-batch fan-out（`WikiMaxDocsPerBatch=5`）、MaxRetry 10、Timeout 60/30min、ErrWikiIngestConcurrent 同实例退避全部保留；132 = K3.1 记录 130 + OCR R1 回归 2（commit `7ffaf6cc4` seams_test.go 增 `TestNewSpanNormalizesNilAndTypedNil`/`TestSpanWrapperControlFlowParity`，git log 实证） |
| ② FAQ 导入 `TypeFAQImport` | `TestAcquireFAQCreateGuard{RejectsConcurrentSameQuestion,IsolatesUnrelatedCreates,FallsBackWithoutRedis}`（随迁）、`TestParseOptionalFAQEnabled`/`TestFAQListEntries{Passes,Rejects}…`（随迁）、`TestNewService*` 2 锚定、留驻 `TestFAQImport{CompletedOutcome,ActivityDetails}` 等 15（经垫片委托） | T0：§2.1（service 18 + handler 3）；T1/T2 终态：`go test -count=1 -v ./internal/knowledge/faq/` → **8 顶层 PASS / 0 FAIL / 0 SKIP**（exit 0）；`go test -count=1 ./internal/application/service/ -run 'FAQ'` → 15 顶层 PASS（§2.2） | **等价**（§2.3/§2.5 逐用例）：dry-run 零写入、memFAQProgress 指针共享、失败 CSV 生成、断点续跑/幂等；集合差 = 随迁 3 守卫用例，逐一比对无缺漏 |
| ③ `recoverPendingWikiTasks` 重启恢复（hook 恢复手工差分，**K3.4 新增双跑**） | `TestRecoverPendingWikiTasks_RecreatesOneTriggerPerLaneAndKB`（reset_pending_tasks_test.go:355，构造 knowledge_bases 3 行 active×2+deleted×1 与 task_pending_ops 6 行：同 lane 重复 ×2、finalize lane、跨租户、kb-deleted、kb-missing）、`TestResetPendingTasks_DurableWikiOpSurvivesLiteRestart`(:191)、`TestResetPendingTasks_LiteWikiDoesNotHideOtherLostSubtasks`(:216) | **双跑**（同命令 `go test -count=1 -v ./internal/container/ -run 'TestRecoverPendingWikiTasks_RecreatesOneTriggerPerLaneAndKB|TestResetPendingTasks_DurableWikiOpSurvivesLiteRestart|TestResetPendingTasks_LiteWikiDoesNotHideOtherLostSubtasks'`）：旧侧 = 临时 worktree `git worktree add --detach /tmp/k34-base-1c9d812d 1c9d812d0`（迁移前基线），新侧 = 本分支 HEAD。两侧输出**逐字节一致**：`[WikiRecovery] removed 2 pending row(s) for deleted knowledge bases` + `recreated 3 trigger(s) from durable pending queues`，3/3 PASS，exit 0 | **等价**（§3.2 逐项论证）：重建数 3=3（同 lane 单 trigger）、fail-closed 清除数 2=2（deleted+missing）、payload 租户路由断言（7/7/8）双侧 PASS；Timeout 60/30min、MaxRetry 10、`asynq.TaskID("wiki-finalize-"+scopeID)` 去重——recover_pending_wiki_tasks.go 本节点零改动（§3.2） |
| ④ Wiki 页面/文件夹操作 | wiki_page_test.go、wiki_page_revision_test.go、wiki_folder_prune_finalize_test.go、wiki_slug_handles_test.go、wiki_linkify_test.go、wiki_page_repository_test.go（4 哨兵） | 同①包内运行（132/132）；`TestRepairContentLinks` 宿主真实 seam 双跑见 §1.3（3/3 子用例 PASS） | **等价**（§1.2/§1.3）：冻结 WikiPageService 方法集逐用例一致；`ErrWikiPageNotFound/ErrWikiFolderNotFound/ErrWikiFolderNotEmpty/ErrWikiFolderConflict` 4 哨兵标识不变（随迁包内符号，断言未动） |

### 3.2 hook 恢复逐项等价论证（行 ③ 支撑证据）

1. **实现零改动**：`git diff 1c9d812d0..HEAD -- internal/container/` 仅 container.go 一个文件——`knowledgeWiki.NewWikiPageRepository` provider 行 + 1 import 行 + 4 注释行（Ruling 2026-09-25-CYCLE-FORCED-COMPOSITION 提前落地，IB2 转核验）；`recover_pending_wiki_tasks.go` 与 `reset_pending_tasks_test.go` 均不在 diff 中（字节不变）。`asynq.Timeout(60*time.Minute)`/finalize 分支 `30*time.Minute`、`asynq.MaxRetry(10)`、`asynq.TaskID("wiki-finalize-"+scope.ScopeID)` 与 ErrTaskIDConflict/ErrDuplicateTask 吸收分支逐字保留。
2. **唯一依赖变化的等价性**：`recover_pending_wiki_tasks.go:75` 的 `service.WikiIngestPayload` 现经宿主 W1（wiki_k3_compat.go:77 `type WikiIngestPayload = wiki.WikiIngestPayload`）解析——**真类型别名**（非新类型）；wiki 包 `type WikiIngestPayload struct`（含 `types.TracingContext` 内嵌与 3 字段 JSON tag）与 BASE `git show 1c9d812d0:internal/application/service/wiki_ingest.go` 同段**逐字节一致**（本任务 diff 实证）→ `json.Marshal` 输出字节不变、测试侧 `json.Unmarshal` 同构。
3. **可观察行为双跑**：见 §3.1 行③——旧侧（迁移前 `1c9d812d0`）与新侧（HEAD）日志输出（removed 2 / recreated 3）、断言集（3 用例）、退出码（0）全部一致；`TestResetPendingTasks_DurableWikiOpSurvivesLiteRestart`（Lite 重启 durable wiki op 存活）与 `TestResetPendingTasks_LiteWikiDoesNotHideOtherLostSubtasks`（Lite wiki 不遮蔽其它丢失子任务，日志 `Reset 1 stuck knowledge parsing tasks`）双侧同输出。

### 3.3 T0 基线 vs T1/T2 终态对照（K3.4 终跑）

| 面 | T0（迁移前，§0/§2.1 记录） | T1/T2 终态（K3.4 实跑） | 结论 |
|---|---|---|---|
| repository Wiki 面 | 2 PASS | 随迁 wiki 包（wiki_page_repository_test.go，包内 132 之一） | 集合一致 |
| service Wiki\|Linkify\|Slug | 85 PASS / 0 SKIP（顶层 92 口径） | wiki 包 132 PASS / 0 FAIL / 0 SKIP（= 130 集合 + OCR R1 2 回归，行①） | 集合一致（增量有 commit 实证） |
| session WikiFixer 面 | 5 PASS | wiki 包 wiki_fixer_scope_test.go 5/5（行④/§1.4） | 逐用例一致 |
| service FAQ 面 | 18 顶层 PASS | faq 包 8（含守卫 3+filter 3+锚定 2）+ 宿主留驻 15（§2.2） | 18 = 3 随迁 + 15 留驻，精确差集（§2.5） |
| handler FAQ 面 | 3 顶层 PASS | faq 包 3/3（断言面等价改写 §2.4-1） | 等价 |
| container hook 恢复 | （旧侧双跑）3 PASS | 3 PASS，输出逐字节一致（§3.1 行③） | 等价 |

### 3.4 K3.4 门禁终跑（conventions §1.2，2026-09-25，基线=节点工作起点 `1c9d812d0`）

| 命令（原文） | 退出码 | 关键输出 |
|---|---|---|
| `go build ./...` | 0 | 仅 `ld: warning: ignoring duplicate libraries: '-lc++'`（cmd/desktop、cmd/server 链接器警告，与迁移无关） |
| `go test -count=1 ./internal/knowledge/...` | 0 | 18 个有测试包全 `ok`、0 `FAIL`（含 `…/knowledge/faq`、`…/kbfreeze`、`…/wiki`） |
| `make check-backend-architecture` | 0 | `architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16`；`OK (0 violations)` |
| `make verify-module-moves` | 0 | `modulemove: OK (16 manifests verified)` |
| `git diff --stat 1c9d812d0..HEAD` | 0 | `57 files changed, 2326 insertions(+), 556 deletions(-)`（K3.4 commit 仅追加修改已列 2 产物文件，路径集合不变） |
| `git diff --name-only 1c9d812d0..HEAD \| sort` | 0 | 57 路径，与 §3.2 可写清单求差集=**空**（归类核对见节点报告 §2；禁改文件 pattern grep 零命中 exit 1） |

计数奇偶（conventions §8 三方一致）：guard 实测 633（564 literal+69 apiKeyRoute）/ 23+23 / 58 == `docs/architecture/evidence/pass-a-acceptance.md` 台账（:22 路由 633 同分解、:23 Redis/Lite worker 23/23、:24 hooks 58）== manifests 发现值（`modulemove verify` 16 manifests OK）。本节点零路由/worker/钩子增删。

18 条 legacy 路径零残留终验（验收 #3）：逐条 `git ls-files --error-unmatch` → 全部无 RESIDUE（K3.4 复跑）。

