# Integration Brief — b2-k-ingest（21-knowledge-ingest）

> **状态：部分草稿（K1.2 OCR 修复 ocr-r1-1 先行登记 1 项）**。完整十类接线/删除项
> （R1 shim 全清单、seam→生产接线表、conversation 调用点改写、handler wrapper
> 删除批、exc-0088、E1-E6 例外删除批等）由 Task K1.6 按 plan
> `docs/plans/passb/21-knowledge-ingest.md` §8 K1.6 补齐——本文件为属主文件，
> K1.6 在此基础上扩写，不另起文件。

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
   `internal/modules/knowledge/ingest/chunk_service.go`。
2. **contracts.yaml 消费方补录**（6 条 contract-consumer-unrecorded）：新文件
   `internal/modules/knowledge/ingest/chunk_service.go`（ModelService/ChunkService/TaskEnqueuer）、
   `internal/application/service/chunk_ingest_shim.go`（ModelService/ChunkService/TaskEnqueuer）、
   `internal/application/service/span_trace_seam_adapter.go`（TaskEnqueuer）按各契约
   consumers 登记（shim/适配器为过渡物，ib2 删除时同步去行）。
3. **event-catalog.yaml knowledge.index.completed 路径更新**（event-catalog.yaml:159/:166）：
   producer `internal/application/service/chunk.go:CreateChunks` →
   `internal/modules/knowledge/ingest/chunk_service.go:CreateChunks`；consumers 中
   `internal/application/repository/chunk.go` → `internal/modules/knowledge/ingest/chunk_repo.go`
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
