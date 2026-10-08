# Integration Brief — b2-k-ingest（21-knowledge-ingest）

> **状态：K1.6 完整登记（2026-09-25）**。十类接线/删除项齐备（plan §10 验收 8）：
> ①临时测试垫片批 §1；②R1 宿主 shim 全清单 §6；③seam→生产接线表 §7；
> ④conversation 调用点 ib2 改写项 §8；⑤handler wrapper 删除批与 identity 去方法化
> 联动 §9；⑥exc-0088 删除批及 airesource 门面前置 §10；⑦搬迁显形 import 例外
> （exc-0106..0111）删除批 §11；⑧契约/事件区路径漂移回写批 §3；⑨过渡 shim 行
> ib2 删行闭环 §12；⑩K1.2–K1.5 分任务交付的增量义务 §2/§4/§5。
> 本文件为属主文件（plan §4.1），K1.6 在 K1.2–K1.5 分段登记基础上扩写汇总。

## 1. 临时测试装置垫片删除批（Ruling 2026-09-24-TEST-SUPPORT-SHIM）

| id | 垫片文件 | 保护的宿主测试（禁改） | 断链符号 | remove_at | 删除前置 |
|---|---|---|---|---|---|
| tshim-0001 | `internal/application/service/chunk_ingest_test_shim_test.go` | `internal/application/service/document_write_access_test.go`（K4 属主，:169/:247/:251 夹具） | `chunkService`（service/chunk.go 随 K1.2 迁出 ingest） | ib2 | K4 域改写该测试（或改经 `ingest` 导出构造），垫片随之删除；IB2 先到先删，最迟 B5 |
| tshim-0002 | 同上文件（K1.3 追加段） | `internal/application/service/knowledge_transfer_test.go`（K4 属主，:448/:450） | `DataTableSummaryService`/`extractionResources`/`cleanupOnFailure`（service/extract.go 随 K1.3 迁出 ingest） | ib2 | K4 域改写该测试（或改经 `ingest.NewDataTableSummaryService` + R1 窄端口 `CleanupOnFailure`/`ExtractionResources` 构造），垫片随之删除；IB2 先到先删，最迟 B5 |

追踪台账：`docs/architecture/passb/exception-ledger.yaml`「临时测试装置垫片台账」注释段
（ocr-r1-1 裁定：passbguard 对 ledger 严格 schema 解码，结构化追踪落本 Brief）。

## 2. K1.2 交付的 seam 接线义务（K5/ib2 集成工程师执行）

- 宿主 `internal/application/service/span_trace_seam_adapter.go` 提供（ib2 删除批同窗）：
  `NewSpanTraceSeamAdapter(tr SpanTracker) ingest.SpanTraceSeam`、
  `EnqueueSummaryRefreshProvider(tr SpanTracker)`、`KnowledgeWriteGuardProvider()`、
  `BuildKnowledgeIndexContentProvider()`——K4 搬迁 knowledge_write.go /
  knowledge_span_tracker.go / knowledge_summary_refresh.go / knowledge_index_content.go
  后各提供器指到其导出包装。
- 宿主 `internal/application/service/chunk_ingest_shim.go` 的 `NewChunkService` 转发保留
  dig 旧签名（末参 `spanTracker SpanTracker`），seam 新参体内适配；ib2 将
  container.go:374 的 `service.NewSpanTracker` 经适配器直供
  `ingest.NewChunkService` 新签名（含 `writeGuard`/`indexContentFn` 追加参）后删除转发。
- `ingest.SameChunkDocument` 导出（R1 增量）：宿主 knowledge_process.go:2256/:3002 经
  shim 消费，K4 搬迁后随其文件收敛。

## 3. 契约/事件区路径漂移回写批（ib2 专写；work 节点禁改 contracts.yaml/event-catalog.yaml，conventions §3）

背景：K1.1+K1.2 物理迁移后 `go run ./tools/passbguard -root .` 显形 12 条诊断（此前被
ledger 严格解码失败掩蔽；ocr-r1-1 修复后可见，2026-09-25 实测）。两类文件仅 barrier
可写，故全部移交 ib2 回写：

1. **contracts.yaml 消费方路径更新**（3 条 contract-consumer-file-missing）：
   `airesource.model-service` / `knowledge.chunk-service` / `workbench.task-enqueuer`
   三个契约的 consumers 中 `internal/application/service/chunk.go` →
   `internal/knowledge/ingest/chunk_service.go`。
2. **contracts.yaml 消费方补录**（6 条 contract-consumer-unrecorded）：新文件
   `internal/knowledge/ingest/chunk_service.go`（ModelService/ChunkService/TaskEnqueuer）、
   `internal/application/service/chunk_ingest_shim.go`（ModelService/ChunkService/TaskEnqueuer）、
   `internal/application/service/span_trace_seam_adapter.go`（TaskEnqueuer）按各契约
   consumers 登记（shim/适配器为过渡物，ib2 删除时同步去行）。
3. **event-catalog.yaml knowledge.index.completed 路径更新**（event-catalog.yaml:159/:166）：
   producer `internal/application/service/chunk.go:CreateChunks` →
   `internal/knowledge/ingest/chunk_service.go:CreateChunks`；consumers 中
   `internal/application/repository/chunk.go` → `internal/knowledge/ingest/chunk_repo.go`
   （后者由 K1.1 迁移，该诊断自 K1.1 起已存在）。

## 4. K1.3（extract.go 搬迁）交付的 shim/seam/窄端口义务（K5/ib2 集成工程师执行）

- **R1-3/R1-4/R1-6/R1-9 宿主转发**（`internal/application/service/chunk_ingest_shim.go`，
  ib2 删除批）：`NewChunkExtractService`（保 container.go:409 dig 旧签名，末参
  `spanTracker SpanTracker`）、`NewDataTableSummaryService`（保 container.go:410，
  原 10 参签名 + 体内注入 3 项增量 seam）、`NewChunkExtractTask`（保
  knowledge_post_process.go:418）、`enqueueDataTableSummaryIfNeeded`（保
  knowledge_create.go:295/:746、knowledge_process.go:2629/:2682，以本包
  normalizeFileExtension/isDataTableFileType/getFileType 函数值构造
  `ingest.NewDataTableSummaryEnqueuer` 注入）。
- **K1.3 增量 seam（plan §6.3 未枚举，按 R2 机制具体化）**，适配器在
  `span_trace_seam_adapter.go`（ib2 删除批）：`AttemptSupersededProvider(tr)`（nil
  tracker 恒 false，noop 语义）、`KnowledgeWriteKBProvider()`（lookup 接口结构等价
  适配）、`DataAnalysisToolSeamFactory()`、`GraphExtractorSeamFactory()`；直传函数值
  `previewText`/`finalizeSubtaskDetached`/`isFinalAsynqAttempt`/`ResolveProcessConfig`
  （K3/K4 搬迁导出后改指其导出形式）。
- **import 环裁决（K1.3 实测）**：ingest 直连 `agentruntime/agent/tools` 或
  `conversation/chat_pipeline` 均经传递依赖回到 `internal/application/repository`
  （wiki_route_resolver.go:10 / chat_pipeline 自带 data_analysis.go 消费
  repository.ErrWikiPageNotFound 等），与宿主 R1-8 shim 的
  `repository→ingest` 边构成环；且 `service/knowledge.go:38`（K4 属主）钉住
  `repository.ErrChunkNotFound` 转发，装配切换（Ruling 2026-09-25-CYCLE-FORCED-
  COMPOSITION）无法单独破环——故 tools/chat_pipeline 以消费侧 seam
  （`DataAnalysisToolSeam`/`GraphExtractorSeam`，spec §4.2）接入，extract.go 保持
  零 agentruntime/conversation import。**原 plan §7.2 E3 例外
  （agentruntime/agent/tools）因此不再需要登记**；E1/E2/E4（airesource chat/
  embedding、policy/access）仍需 K1.6 登记。
- **R1 窄端口导出（增量，超出 §6.2 字面清单，同 K1.2 SameChunkDocument 先例）**：
  `ingest.CleanupOnFailure`（原 cleanupOnFailure）+ `ingest.ExtractionResources`
  （原 extractionResources；导出 Knowledge/RetrieveEngine/EmbeddingModel 三字段，
  余字段包内私有）——供 tshim-0002 垫片委托真实现；K4 域改写后可回收敛。

## 5. K1.4（image_multimodal.go + ocr_sanitizer.go 搬迁）交付的 shim/seam/导出义务（K5/ib2 集成工程师执行）

- **R1-5 宿主转发**（`internal/application/service/chunk_ingest_shim.go`，ib2 删除批）：
  `NewImageMultimodalService` 保 container.go:411 dig 旧 14 参签名（末参
  `spanTracker SpanTracker`），体内以 `NewSpanTraceSeamAdapter` + 宿主
  `repository.ErrKnowledgeNotFound`/`ErrKnowledgeBaseNotFound` 哨兵 + 本包
  `previewText`/`ResolveProcessConfig`/`knowledgePostProcessTaskOptions` 函数值
  供给 ingest 新签名（哨兵注入发生在宿主边界——ingest 禁 import 宿主
  repository，R1-8 shim import 环实测裁决，plan §6.3）。
- **R1-7 宿主转发**：`isFinalAsynqAttempt`（真源 `ingest.IsFinalAsynqAttempt`）——
  保护 knowledge_process.go:1125/:1497/:1868（K4 属主）与
  knowledge_summary_test.go:230/:238；K4 搬迁后随其文件收敛。
- **R1-10 宿主转发（conversation ib2 改写项）**：`sanitizeOCRText` /
  `buildVLMCaptionPrompt`（真源 `ingest.SanitizeOCRText` /
  `ingest.BuildVLMCaptionPrompt`，K0 §6.2 组 D）——保护
  temporary_document.go:541/:560；**ib2 改写为直连 ingest 导出形式后删除转发**。
- **R1 增量常量导出（超出 §6.2 字面清单）**：`ingest.VlmOCRPrompt` /
  `ingest.VlmOCRScannedPDFPrompt` 常量别名——保护 temporary_document.go:513/:515
  （conversation 属主，ib2 改写项，同上随 R1-10 批收敛）。
- **K1.4 增量 seam（plan §6.3 未枚举，按 R2 机制具体化）**：
  ImageMultimodalService 的 `previewTextFn`（K3 wiki_ingest.go:1383，K1.3 同款）、
  `resolveProcessConfigFn`（K4 knowledge_process_config.go:40，K1.3 同款）、
  `postProcessTaskOptionsFn`（K4 knowledge_task_options.go:21，K1.4 新增——
  原包级调用 `knowledgePostProcessTaskOptions()` 随文件迁移断链，构造注入零复制）；
  K3/K4 搬迁导出后由集成工程师改指其导出形式。
- **随迁测试哨兵孪生**：ingest/image_multimodal_orphan_test.go 以测试本地
  `errors.New` 孪生值注入 `knowledgeNotFoundErr`/`knowledgeBaseNotFoundErr`
  同名字段（plan §6.3 末行；比对语义不变，字段即 errors.Is 比对目标）；
  `postProcessTaskOptionsFn` 测试桩返回 nil（用例仅断言任务类型与计数；
  生产代码对 nil 字段快速失败不设静默回退）——K1.2 no-op guard 先例同机制。
- **E5/E6 例外（K1.6 登记）**：ingest/image_multimodal.go import
  `airesource/models/utils/ollama` 与 `airesource/models/vlm`（Pass A 前宿主直连，
  plan §7.2 表）。


## 6. R1 宿主 shim 全清单（plan §6.2 十项 + K1.1–K1.5 增量；逐条宿主引用，2026-09-25 会话 grep 复核）

### 6.1 `internal/application/repository/chunk_ingest_shim.go`（ib2 删除批）

| # | shim 符号 | 被保护的宿主引用（实测） | 真源 |
|---|---|---|---|
| R1-8 | `NewChunkRepository(db *gorm.DB)` | container.go:205（dig Provide）；宿主测试 document_write_access_test.go / knowledge_write_access_test.go:113 / knowledge_caller_scope_test.go（K4 属主） | `ingest.NewChunkRepository` |
| R1-8 增量 | `var ErrChunkRevisionConflict` | errors.Is 链同实例语义（plan §5.3；service/chunk.go:23 旧别名已随 K1.2 删） | `ingest.ErrChunkRevisionConflict` |
| R1-8 增量 | `var ErrChunkNotFound` | service/knowledge.go:38（K4 属主，禁改）；document_write_access_test.go:568 | `ingest.ErrChunkNotFound` |

### 6.2 `internal/application/service/chunk_ingest_shim.go`（ib2 删除批）

| # | shim 符号 | 被保护的宿主引用（实测） | 真源 |
|---|---|---|---|
| R1-1 | `var ErrChunkRevisionConflict` | knowledge_process.go:2282（K4） | `ingest.ErrChunkRevisionConflict` |
| R1 增量 | `sameChunkDocument`（包内私有） | knowledge_process.go:2256/:3002（K4） | `ingest.SameChunkDocument` |
| R1-2 | `NewChunkService`（dig 旧签名，末参 spanTracker） | container.go:375（dig Provide） | `ingest.NewChunkService`（新签名，体内经 §7 适配器接线） |
| R1-3 | `NewChunkExtractService`（dig 旧签名） | container.go:409 | `ingest.NewChunkExtractService` |
| R1-4 | `NewDataTableSummaryService`（原 10 参签名） | container.go:410 | `ingest.NewDataTableSummaryService` |
| R1-6 | `NewChunkExtractTask`（8 参） | knowledge_post_process.go:418（K4） | `ingest.NewChunkExtractTask` |
| R1-9 | `enqueueDataTableSummaryIfNeeded`（8 参包级） | knowledge_create.go:295/:746、knowledge_process.go:2629/:2682（K4；本会话 grep 复核存活） | `ingest.EnqueueDataTableSummaryIfNeeded` + `ingest.NewDataTableSummaryEnqueuer(normalizeFileExtension, isDataTableFileType, getFileType)`（宿主函数值注入，零复制） |
| R1-5 | `NewImageMultimodalService`（dig 旧 14 参签名） | container.go:411 | `ingest.NewImageMultimodalService`（新签名：SpanTraceSeam + 哨兵 error 注入在宿主边界） |
| R1-7 | `isFinalAsynqAttempt` | knowledge_process.go:1125/:1497/:1868（K4）+ knowledge_summary_test.go:230/:238 | `ingest.IsFinalAsynqAttempt` |
| R1-10 | `sanitizeOCRText` | temporary_document.go:541（conversation，ib2 改写项，§8） | `ingest.SanitizeOCRText` |
| R1-10 | `buildVLMCaptionPrompt` | temporary_document.go:560（conversation，ib2 改写项，§8） | `ingest.BuildVLMCaptionPrompt` |
| R1 增量 | `vlmOCRPrompt` / `vlmOCRScannedPDFPrompt` 常量 | temporary_document.go:513/:515（conversation，ib2 改写项） | `ingest.VlmOCRPrompt` / `ingest.VlmOCRScannedPDFPrompt` |
| R1 增量 | `validateParserEngineOverrideURLs` | knowledge_process.go:3750（K4） | `ingest.ValidateParserEngineOverrideURLs` |

### 6.3 `internal/handler/chunk_ingest_shim.go`（wrapper，§9 删除批）

| # | shim 符号 | 被保护的宿主引用（实测） | 真源 |
|---|---|---|---|
| R1（§6.4） | `type ChunkHandler`（嵌入 `*ingest.ChunkHandler` + service/kgService 字段副本）+ 11 路由方法转发 | routes_knowledge.go:28-66（11 条 /chunks 路由）、container.go:719、rbac_lookups.go:103/:104/:130/:149（identity 属主，禁改） | `ingest.ChunkHandler`（唯一实现） |
| R1（§6.4） | `NewChunkHandler` | container.go:719（dig） | `ingest.NewChunkHandler` |
| R1（§6.4） | `PreviewChunking` | routes_knowledge.go:19（/chunker/preview） | `ingest.PreviewChunking` |

## 7. seam→生产接线总表（K5/ib2 集成工程师执行；适配器现位 `internal/application/service/span_trace_seam_adapter.go`，与 shim 同属 ib2 删除批）

| ingest 侧 seam（定义 `ingest/seams.go`） | 生产接线（ib2） | 现接线点 |
|---|---|---|
| `SpanTraceSeam` | `NewSpanTraceSeamAdapter(tr SpanTracker)`；container.go:374 `service.NewSpanTracker` 经适配器直供 R1-2/3/5 新参；K4 搬迁 knowledge_span_tracker.go 后指其导出包装 | span_trace_seam_adapter.go:30 |
| `KnowledgeWriteGuard` | `KnowledgeWriteGuardProvider()`；K4 搬迁 knowledge_write.go 后指其导出包装 | :99 |
| `KBByIDLookup` | `KnowledgeWriteKBProvider()`（结构等价适配；`interfaces.KnowledgeBaseRepository` 天然满足） | :146 |
| `enqueueSummaryRefresh` 闭包 | `EnqueueSummaryRefreshProvider(tr)`（参数 `summaryKnowledgeBaseReader` → KBByIDLookup） | :81 |
| `BuildKnowledgeIndexContent` 闭包 | `BuildKnowledgeIndexContentProvider()`（K4 knowledge_index_content.go 导出后改指） | :126 |
| `attemptSupersededFn` | `AttemptSupersededProvider(tr)`（nil tracker 恒 false，noop 语义；K4 knowledge.go:202 导出后改指） | :133 |
| `previewTextFn` | 直传宿主 `previewText`（K3 wiki_ingest.go:1383 导出 `PreviewText` 后改指） | R1-3/R1-5 shim 体内 |
| `resolveProcessConfigFn` | 直传宿主 `ResolveProcessConfig`（K4 knowledge_process_config.go:40 导出后改指） | R1-3/R1-5 shim 体内 |
| `postProcessTaskOptionsFn` | 直传宿主 `knowledgePostProcessTaskOptions`（K4 knowledge_task_options.go:21 导出后改指） | R1-5 shim 体内 |
| `DataAnalysisToolFactory` | `DataAnalysisToolSeamFactory()`（agentruntime/conversation 根门面端口就绪后切换直连） | :164 |
| `GraphExtractorFactory`（chat.Chat 参数） | `GraphExtractorSeamFactory()`（同上；K1.3 import 环裁决，§4） | :216 |
| 哨兵 error 字段（knowledgeNotFoundErr/knowledgeBaseNotFoundErr） | 宿主 `repository.ErrKnowledgeNotFound`/`ErrKnowledgeBaseNotFound` 注入（ingest 禁 import 宿主 repository——R1-8 环裁决；K4 搬迁后指其导出） | R1-5 shim 体内 |

## 8. conversation 调用点 ib2 改写项（跨 owner 调用点修改，集成工程师执行）

- temporary_document.go:541（`sanitizeOCRText`）与 :560（`buildVLMCaptionPrompt`）：ib2 改写为直连 `ingest.SanitizeOCRText` / `ingest.BuildVLMCaptionPrompt` 后删除 R1-10 转发。
- temporary_document.go:513/:515（`vlmOCRPrompt` / `vlmOCRScannedPDFPrompt`）：同批改写为直连 `ingest.VlmOCRPrompt` / `ingest.VlmOCRScannedPDFPrompt`。
- 本节点未改 conversation 属主文件一字（diff 零出现，plan §10 验收 5）。

## 9. handler wrapper 删除批与 identity 去方法化联动（ib2）

- `internal/handler/chunk_ingest_shim.go` 的包装类型（§6.3）：rbac_lookups.go:130/:149（identity 属主，禁改）在宿主 `*ChunkHandler` 上定义 `KBCreatorLookupFromKnowledgeIDParam`/`KBCreatorLookupFromChunkIDParam`，方法体访问 `h.kgService`（:135）与 `h.service`——包装的自有字段承载之。
- ib2 删除序：①identity 去方法化（B0.3 裁定：两 lookup 改包级函数或迁 identity 域）；②路由切模块门面（routes_knowledge.go 11+1 条改 `ingest` 直供）；③container.go:719 切 `ingest.NewChunkHandler`；④wrapper 文件删除。四步同窗，rbac_lookups.go 与路由计数 633 不变。
- 删除前置：rbac_lookups_test.go 经 wrapper 保绿的用例随去方法化同步改写。

## 10. exc-0088 删除批及 airesource 门面前置（plan §7.3）

- exc-0088（exception-ledger.yaml，owner=21-knowledge-ingest，remove_at=ib2）：`internal/knowledge/docparser/weknoracloud_http_reader.go` import `airesource/models/utils` 仅用 `utils.Sign`（:237 → signer.go:24）。
- 前置（属 airesource/ib2 契约动作，非本节点）：airesource 根门面现为零逻辑骨架（`internal/airesource/module.go` 无导出符号，K1.6 会话实测），无合法替代 import 可切。ib2 删除批 = airesource 根门面暴露 `Sign` 或等价端口 → docreader 消费切换 → 删 exc-0088 行。
- 本节点不删行、不改 docparser 文件（Pass A 已在模块内，非本节点 9 文件）。

## 11. 搬迁显形 import 例外删除批（exc-0106..0111，Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY；commit fc14f4c2e 登记）

| id | from → to | 删除前置（ib2） |
|---|---|---|
| exc-0106 | ingest/extract.go → airesource/models/chat | airesource 根门面/契约端口暴露所需符号后切换直连，删 check.go 数据行 + ledger 行（同窗） |
| exc-0107 | ingest/extract.go → airesource/models/embedding | 同上 |
| exc-0108 | ingest/extract.go → policy/access | policy 根门面端口就绪后切换 |
| exc-0109 | ingest/seams.go → airesource/models/chat | GraphExtractorFactory 生产接线（§7）改门面端口后，seams.go 类型签名随之收敛 |
| exc-0110 | ingest/image_multimodal.go → airesource/models/utils/ollama | airesource 门面端口（同 exc-0106 批） |
| exc-0111 | ingest/image_multimodal.go → airesource/models/vlm | 同上 |

注：plan §7.2 预测的 E3（agentruntime/agent/tools）经 K1.3 import 环裁决改走消费侧 seam，未登记（§4）；实测集与 guard 诊断逐条对齐（K1.6 报告 §1）。ib2 删除时与 §3 契约 consumers 回写同窗复核 passbguard 三方计数。

## 12. 过渡 shim 行 ib2 删行闭环（Ruling 2026-09-25-TRANSITION-SHIM-ROW-REGISTRATION 第 3 条）

- 4 个过渡 shim 文件（§6.1/§6.2/§6.3 四件）在 knowledge.yaml legacy_files 与 ownership-matrix.yaml 的成对行（commit 253497b1f 登记）：**ib2 收口时 shim 文件与其 manifest/matrix 行同 commit 删除**（Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP 删行语义衔接）。
- 届时 legacy 计数回落（391 → −4）走 conventions §8 基线变更（台账条目 + evidence），F5 三方一致复验。
- 删除前置：本 Brief §6 全部宿主引用清零（§7 接线 + §8 改写 + §9 wrapper 批完成后）；ib2 前任何残留 importer 未清零禁止删除（conventions §7.5）。
