# Evidence — b2-k-wikifaq（Pass B 23-knowledge-wikifaq）

> 状态：K3.1+K3.2 草稿（§1 wiki 差分 / §2 faq 差分已建立，K3.4 终稿收口四行全表）。节点基线 BASE=7ffaf6cc4（K3.2 起点；K3.1 段基线 4649630df）；节点分支 codex/passb-b2-k-wikifaq。

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
| `go test -count=1 ./internal/modules/knowledge/wiki/...` | ok | 130 PASS / 0 FAIL / 0 SKIP | 集合 = T0 基线 85（repo 2 + service 83）+ session 5 + 测试类目展开（`-v` 顶层 RUN 计数口径：T0 顶层 92 → 迁入后同断言全 PASS；TestRepairContentLinks 移至宿主真实 seam 双跑，见 §1.3） |
| `go test -count=1 ./internal/handler/session/... ./internal/application/service/ -run 'Wiki'` | ok | — | K4 留驻（knowledge_post_process_wiki_enqueue_test.go、knowledge_move_wiki_test.go、knowledge_summary_test.go、knowledge_housekeeping_test.go）经 W1/W2 PASS |
| `go test -count=1 ./internal/modules/knowledge/kbfreeze/` | ok | 2 守卫 | R0 冻结面零违反 |
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
| `go test -count=1 ./internal/modules/knowledge/faq/` | ok | 8 顶层 PASS / 0 FAIL / 0 SKIP | = 3 守卫用例 + 3 filter 用例（同用例同断言随迁）+ 2 个 K3.2 Step 2 新锚定（TestNewService*，构造器接线/指针共享）；filter 第三用例断言面见 §2.4 |
| `go test -count=1 ./internal/application/service/ -run 'FAQ'` | ok | 15 顶层 PASS / 0 FAIL / 0 SKIP | = T0 18 − 随迁 3 守卫用例（精确差集，见 §2.5） |
| `go test -count=1 ./internal/application/service/ -run 'FAQ\|Knowledge'` | ok | — | K2/K4 留驻面（含 clone/move、semantic scope、replace）经 D1 委托 PASS |
| `go test -count=1 ./internal/handler/... ./internal/router/...` | ok（4 包） | — | H2 类型别名下 routes 注册与 `&handler.FAQHandler{}` 零字段字面量合法 |
| `go build ./...` | exit 0 | — | knowledgeService 仍实现 interfaces.KnowledgeService（冻结端口满足性=编译器强制；接口零改动） |
| `go vet ./internal/modules/knowledge/faq/... ./internal/application/service/` | exit 0 | — | sync.Map 指针共享无复制告警 |
| `go test -count=1 ./internal/modules/knowledge/...` | ok（全域） | — | 含 kbfreeze 2 守卫：faq 包无影子类型（R0） |
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
- **修复后复跑**：`go build ./...` exit0；`go test -count=1 ./internal/modules/knowledge/faq/...` ok；`go test -count=1 ./internal/application/service/ -run 'FAQ|Knowledge'` ok；`go test -count=1 ./internal/modules/knowledge/...` exit0；`make verify-module-moves` OK；`make check-backend-architecture` OK（633/23+23/58 不变）；`gofmt -l` 空。

## 3. K3.3/K3.4 章（占位，后续任务填充）

- Integration Brief、断链登记、终稿四行差分表与节点门禁收口：K3.3/K3.4 填充。

