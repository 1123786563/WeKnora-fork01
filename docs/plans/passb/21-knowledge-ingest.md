# Pass B B2 — 21-knowledge-ingest（b2-k-ingest：knowledge 摄取域 9 文件搬迁）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**节点：** `b2-k-ingest`（DAG `docs/plans/passb/execution-dag.json`，`depends_on=[b2-k0]`，gates=`go build ./...` / `go test ./internal/modules/knowledge/... -count=1` / `make check-backend-architecture` / `make verify-module-moves`，`execution_mode=parallel`）。本计划只承载 b2-k-ingest 一个节点的任务（K1.0–K1.7）；同程序其他节点的义务只在 §2/§10 以「属主节点」注明，不构成本计划任务。

**Goal:** 把 `internal/application/{repository,service}` 与 `internal/handler` 中归属 K1 的 9 个摄取域 legacy 文件（repository/chunk.go、service/chunk.go、chunk_write.go、extract.go、image_multimodal.go、ocr_sanitizer.go、parser_url_security.go、handler/chunk.go、chunker_debug.go）搬迁至 `internal/modules/knowledge/ingest`（ownership-matrix 逐行登记的 destination），宿主包零转发业务声明；K1 定义、跨 owner 消费的符号按 K0 R1 导出 + 宿主薄 shim；K1 消费、他 plan 定义的未导出符号按 K0 R2 建消费侧 seam；`TypeChunkExtract` 与 `knowledge.index.completed` 高风险差分双跑留证；节点 gates 四项全绿。

**Architecture:** 纯搬迁 + 编译修复 + 兼容 shim（spec §13 M2/M3/M1 提交隔离），不改任何业务语义、不改路由/worker 注册行、不改 schema。9 文件按 matrix 同一 destination 合并为单包 `internal/modules/knowledge/ingest`；装配切换（router/container）不属本节点——由 K5/ib2 按本节点交付的 Integration Brief 执行。

**Tech Stack:** Go 1.26、Gin、GORM、dig、asynq、SQLite 内存测试、Testify 风格裸 testing、Git worktrees。

**Spec:** `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md`（§4.2 依赖方向 :86-96、§4.3 模块装配 :103-107、§5.2 Knowledge :145-148、§11 Pass B 串并图 :386-396、§13 提交隔离和回滚 :411-424、§14.2 纯移动验证 :440-443、§14.3 高风险差分 :444-457、§17.2 完成标准 :488-500）。

---

## 1. Spec 与事实源指针（全部只读输入；行号为 2026-09-24 会话实测）

| 事实源 | 本计划引用点 |
|---|---|
| `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md` | §4.2「接口优先定义在使用方模块」（seam 依据）、§13 M2/M3 隔离、§14.3 差分、§17.2 完成标准 |
| `docs/plans/2026-09-23-backend-modularization-pass-b-framework.md` | :25 manifest 唯一所有权、:28 一次一支审后合并、:29 生产文件随迁 `_test.go`、:30 禁双写/复制、:31 legacy 删除前差分必过、:33 禁改范围、:40 knowledge deletion/indexing 高风险面、:118-119 K1(9)|K2(29)|K3(18) 并行、:132 K1-K3 并行前提 = K0 分配共享类型 |
| `.superpowers/sdd/passb/conventions.md`（主 checkout，git-ignored） | §1 派发契约（只改 owned_files、跨 owner 调用点走 Brief）、§2 门禁、§3 禁改清单、§4 提交规范、§5 升级契约、§6 高风险差分证据、§7 package-private 耦合、§8 计数基线、§9 DAG 回填 |
| `docs/plans/passb/20-knowledge-program.md`（b2-k0 冻结产物，节点分支 `codex/passb-b2-k0` head `5bcb7986`） | §5 共享类型分配表（R0：`internal/types` 单一事实源）、§6.1 R1/R2/R3 通用裁定、§6.2 组 A/D 联动符号表、§7.1-7.7 契约/路由/worker/事件/别名/例外清单、§9 K1 行义务、§10/§11 测试与回滚 |
| B0 冻结产物（`.worktrees/passb-int/docs/architecture/passb/`，本分支合并 b2-k0 后以 `git ls-tree` 同名核验） | `ownership-matrix.yaml` 9 行（`plan: 21-knowledge-ingest` 锚点 :118/:712/:718/:892/:922/:1216/:1228/:1774/:1780，destination 全为 `internal/modules/knowledge/ingest`，integration_owner/delete_barrier 全 ib2）；`exception-ledger.yaml:530-535` exc-0088；`event-catalog.yaml:137-146/:157-167` knowledge 事件 v1；`contracts.yaml:1576` chunk-service 端口、`:1613` facade、`:1750` routes、`:1893` workers、`:1685/:1857` characterization 登记 |
| `docs/architecture/moves/knowledge.yaml` | K1 的 9 条 legacy_files 行（:79/:131/:135/:139/:151/:291/:295/:363/:367）、18 条 alias_obligations（:41-77，删除归 K5 非本节点）、integration_points |
| `docs/architecture/passb/knowledge-ingest.md` | K1 brief：9 文件清单 :8-16、边界目标 :19-24、删除义务 :26-33 |
| DAG `b2-k-ingest` notes/required_contracts | F1 修正（kb_activity 归 K2）、组 D 两符号导出义务、差分面、BLOCKED 历史见 §2 |
| 实测代码（本 worktree base `6bda27b1d`） | 本计划全部 file:line 与签名出处（§4–§7 各表，撰写时逐条 grep/读源码实证） |

## 2. 节点裁定（调度方原文照录 + 状态澄清）

1. **审校 F1 修正**：`kb_activity.go` 不在 K1 brief 9 文件清单（knowledge-ingest.md:8-16），归 K2 retrieval（knowledge-retrieval.md:11）——kb_activity 4 函数的导出义务在本 DAG 移至 `b2-k-retrieval`，`withKnowledgeCleanup` 归 `b2-k-process`。**本节点义务**：image_multimodal.go（buildVLMCaptionPrompt）与 ocr_sanitizer.go:29（sanitizeOCRText）搬出宿主包时为 conversation 调用方导出端口/留 shim，跨 owner 调用点修改走 Integration Brief，集成工程师执行。knowledge deletion/indexing 属高风险差分面（framework:40）。
2. **BLOCKED 历史**：BLOCKED（2026-09-23）：前置 b0 阻塞；解除条件：修复并 done b0 后恢复。BLOCKED（2026-09-24）：前置 b2-k0 阻塞；解除条件：修复并 done b2-k0 后恢复。**当前状态（2026-09-24 会话实测）**：DAG `b2-k0` `status=done`、`review_status=approved`、`head_sha=5bcb7986`——两轮 BLOCKED 均已解除；b2-k0 分支产出（20 计划 §5-§7 冻结表 + `internal/modules/knowledge/kbfreeze` 守卫包）在本节点分支基线 `6bda27b1d` **之外**，须按裁定 3 先做基线对齐（K1.0）。
3. **Ruling 2026-09-24-WAVE-DEP-BASELINE**（conventions §10.4）：波内依赖节点开工前先做**基线对齐 merge**——把 b2-k0 分支按框架顺序合入本节点分支（独立 merge commit）；性质=基线对齐非集成合并；对齐后重跑前置核验再开工。
4. **Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP**（conventions §10.1）：物理迁移 commit 须**同 commit** 删除 manifest legacy_files 行 + ownership-matrix 行（`manifest:`/`matrix:` 前缀=行级删除权；DeleteBarrier=ib2 为最后期限，可早删）。
5. **Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY**（conventions §10.2）：搬迁显形横向耦合 → 本节点以**独立 commit** 在 `tools/architectureguard/check.go` importExceptions 登记精确 file→package 豁免，**同窗**更新 exception-ledger.yaml 对应行（owner=21-knowledge-ingest，RemoveAt=ib2）+ 机械计数修正 + §8 基线登记。
6. **跨节点义务（非本计划任务，注明属主）**：`kb_activity.go` 4 函数（kbActivityTrigger:36/withKBActivityTask:27/kbActivityAppendSampleTitles:49/recordKBActivity:95）导出义务属 **b2-k-retrieval**（DAG b2-k-retrieval notes F1）；`withKnowledgeCleanup`（knowledge_delete_plan.go:22）导出义务属 **b2-k-process**（DAG b2-k-process required_contracts F1 补）。本节点 9 文件与上述符号无调用关系（会话 grep 实证：K1 文件中零命中）。

## 3. 前置条件（K1.0 开工门禁，逐条实证）

- [ ] **P0 基线对齐**：`codex/passb-b2-k0`（head `5bcb798621b856f7ff6497986c7ca79dba8c338e`）合入本节点分支，merge commit 独立；对齐后 `ls docs/plans/passb/20-knowledge-program.md` 与 `ls internal/modules/knowledge/kbfreeze/` 均存在（2026-09-24 实测本分支 `6bda27b1d` 尚无二者——`git merge-base --is-ancestor 5bcb7986… HEAD` 退出码 1）。
- [ ] **P1 前置核验（对齐后重跑）**：DAG `b2-k0.status=done`、`review_status=approved`；`go test -count=1 ./internal/modules/knowledge/kbfreeze/` PASS（K0.2 守卫，2 测试）。
- [ ] **P2 工作树干净**：`.worktrees/passb-b2-k-ingest` `git status` 干净；分支 `codex/passb-b2-k-ingest`。
- [ ] **P3 门禁工具可用**：`Makefile:250 verify-module-moves`、`Makefile:255 check-backend-architecture`、`Makefile:262 check-passb-readiness` 目标存在（实测 grep 命中）。
- [ ] **P4 基线测试双跑锚点**：K1.0 中按 §8.1 采集 11 个随迁测试 + 差分用例在**旧位置**的通过清单（T0 基线，spec §14.1）。
- [ ] **P5 计划评审通过**：本文件获独立评审 `docs/plans/passb/reviews/b2-k-ingest.md` approved 后方可派发实施。

## 4. 精确文件与写入所有权

### 4.1 本节点可写（owned_files 逐条落实）

**迁移（`git mv`，随迁测试，framework:29；M2 纯移动 commit）：**

| 源（实测行数） | 目标 | 随迁 `_test.go`（实测存在） |
|---|---|---|
| `internal/application/repository/chunk.go`（1317 行） | `internal/modules/knowledge/ingest/chunk_repo.go`① | `repository/chunk_faq_diff_test.go`、`chunk_fields_test.go`、`chunk_revision_test.go`、`chunk_sqlite_test.go`（4 件，package repository 白盒，仅引用 `NewChunkRepository`，会话 grep 实证自包含） |
| `internal/application/service/chunk.go`（878 行） | `internal/modules/knowledge/ingest/chunk_service.go` | `service/chunk_edit_parent_test.go`（`&chunkService{…}` 具名字段字面量，实测 :129） |
| `internal/application/service/chunk_write.go`（143 行） | `internal/modules/knowledge/ingest/chunk_write.go` | 无 |
| `internal/application/service/extract.go`（917 行） | `internal/modules/knowledge/ingest/extract.go` | `service/extract_data_table_summary_test.go`（仅测 `buildSampleDataDescription`） |
| `internal/application/service/image_multimodal.go`（706 行） | `internal/modules/knowledge/ingest/image_multimodal.go` | `service/image_multimodal_orphan_test.go`、`image_multimodal_prompt_test.go` |
| `internal/application/service/ocr_sanitizer.go`（109 行） | `internal/modules/knowledge/ingest/ocr_sanitizer.go` | `service/ocr_sanitizer_test.go` |
| `internal/application/service/parser_url_security.go`（33 行） | `internal/modules/knowledge/ingest/parser_url_security.go` | `service/parser_url_security_test.go` |
| `internal/handler/chunk.go`（472 行） | `internal/modules/knowledge/ingest/chunk_handler.go` | 无（`handler/chunk_test.go` 不存在，实测） |
| `internal/handler/chunker_debug.go`（292 行） | `internal/modules/knowledge/ingest/chunker_debug.go` | `handler/chunker_debug_test.go` |

① 目标文件名：`git mv` 后与源同名为缺省；`chunk_repo.go`/`chunk_service.go`/`chunk_handler.go` 的改名仅为消除同包同名歧义所需的**唯一**例外（repository/chunk.go 与 service/chunk.go 同落 `ingest` 包，同名文件冲突）；除此之外保持源文件名。M2 commit 中 `git diff --summary` 须全部识别为 rename（spec §14.2）。

**新增（本节点产出的新文件）：**

| 文件 | 内容 |
|---|---|
| `internal/modules/knowledge/ingest/doc.go` | `// Package ingest …` 包注释：摄取域边界（上传→解析→分块→chunk 持久化/内容索引），指向 brief 与本计划 |
| `internal/modules/knowledge/ingest/seams.go` | §6.3 全部消费侧 seam 类型定义（`KBByIDLookup`、`KnowledgeWriteGuard`、`SpanTraceSeam`、`noopSpanTraceSeam`、`DataTableSummaryEnqueuer`） |
| `internal/application/repository/chunk_ingest_shim.go` | 宿主薄 shim：`NewChunkRepository` 转发（§6.2 R1-8） |
| `internal/application/service/chunk_ingest_shim.go` | 宿主薄 shim：`ErrChunkRevisionConflict`、`NewChunkService`、`NewChunkExtractService`、`NewDataTableSummaryService`、`NewImageMultimodalService`、`NewChunkExtractTask`、`isFinalAsynqAttempt`、`enqueueDataTableSummaryIfNeeded`、`sanitizeOCRText`、`buildVLMCaptionPrompt` 转发（§6.2 R1-1..7、R1-9..10） |
| `internal/application/service/span_trace_seam_adapter.go` | 宿主 seam 适配：`SpanTracker`→`ingest.SpanTraceSeam` 纯转发适配器 + `attemptSuperseded`/`enqueueSummaryRefresh` 闭包提供器（§6.3 接线，零逻辑复制） |
| `internal/handler/chunk_ingest_shim.go` | 宿主薄 shim：`ChunkHandler` 包装类型、`NewChunkHandler`、`PreviewChunking` 转发（§6.4；保 `rbac_lookups.go:103/:104/:130/:149` 不改一字可编译） |
| `tools/architectureguard/check.go`（importExceptions 数据行） | §7.1 的 6 条新例外（Ruling 2 授权的独立 commit） |
| `docs/architecture/passb/exception-ledger.yaml`（同窗新增行） | §7.1 的 6 条（owner=21-knowledge-ingest，remove_at=ib2） |
| `docs/architecture/passb/briefs/b2-k-ingest.md` | Integration Brief（§9） |
| `docs/architecture/evidence/passb/b2-k-ingest.md`、`docs/plans/passb/reports/b2-k-ingest.md` | 证据与报告（conventions §1.1） |

**治理行删除（同迁移 commit，Ruling 4）**：`docs/architecture/moves/knowledge.yaml` 9 条 legacy_files 行（:79/:131/:135/:139/:151/:291/:295/:363/:367）+ `docs/architecture/passb/ownership-matrix.yaml` 9 行（锚点见 §1）。分批迁移时各 commit 删除对应行。

### 4.2 禁改清单（违者节点失败，conventions §1.2/§3）

- `internal/router/router.go`、`internal/router/routes_knowledge.go`、`internal/router/rbac.go`、`internal/router/task.go`、`internal/router/sync_task.go`、`internal/container/container.go`、`internal/bootstrap/**`（集成工程师独占；本节点以 shim 保其可编译）；
- `internal/handler/rbac_lookups.go`（identity 属主，10-identity，matrix :2000；其 :103/:104/:130/:149 的 ChunkHandler 方法**一字不改**，由 §6.4 wrapper shim 保编译）；
- `internal/application/service/temporary_document.go`（conversation 属主，:541/:560 调用点不改，由 §6.2 shim 保编译）；
- 其余 K2/K3/K4 属主宿主文件（含 `knowledge_write.go`、`knowledge_span_tracker.go`、`knowledge.go`、`knowledge_summary_refresh.go`、`knowledge_util.go`、`knowledge_process.go`、`knowledge_create.go`、`knowledge_post_process.go`）——K1 消费其未导出符号一律走 §6.3 seam，不改对方文件；
- `internal/types/**`（K0 R0：共享类型唯一事实源，只读）、`internal/modules/knowledge/module.go`（门面注释仅 b0 与集成节点）、`internal/modules/knowledge/{chunker,docparser,retriever,searchutil,semantic,kbfreeze,legacy}`（Pass A 已就位/K0 产出，非 ingest 面）；
- `go.mod`、`go.sum`、migration 编号、`cmd/desktop`、`docreader`、`client`、生产 SQL、既有迁移文件（framework:33）；
- 18 条 alias_obligations 与 `internal/infrastructure/*` 别名目录（删除归 K5/b2-k-integration，knowledge.yaml:41-77）。

## 5. 公共可观察行为与兼容要求

1. **路由零变化**：`/chunker/preview`（routes_knowledge.go:19→`handler.PreviewChunking`）与 `/chunks` 组 11 条（routes_knowledge.go:28-66→`*handler.ChunkHandler` 方法）的路径、方法、RBAC 链、错误码逐条不变；路由计数基线 633 不变（conventions §8）。
2. **Worker 零变化**：`TypeChunkExtract`（task.go:266/sync_task.go:143）、`TypeDataTableSummary`（:267/:144）、`TypeImageMultimodal`（:307/:157）在 Redis/Lite 双栈各注册一次；asynq 队列（`types.QueueGraph`/`types.QueueSummary`）、`MaxRetry(3)`、超时（30m）与 payload 结构（`types.ExtractChunkPayload`/`DataTableSummaryPayload`）逐字段不变。
3. **错误哨兵身份不变**：`ingest.ErrChunkRevisionConflict` 即原 `repository.ErrChunkRevisionConflict`（chunk.go:17 `errors.New("chunk revision conflict")` 同一实例语义）；`errors.Is` 链（handler/chunk.go:230/:292、knowledge_process.go:2282 经 shim var）不因搬迁断链。
4. **事件语义不变**：`knowledge.index.completed` v1（event-catalog.yaml:157-167，producer=`service/chunk.go:CreateChunks`，`ChunkStatusIndexed` 可见性、per-knowledge 分批写入序、required_metadata 七键）；`knowledge.processing.completed/failed/deletion.completed` 本节点零触及。
5. **纯移动验证**（spec §14.2）：`git diff --summary` 全 rename；除 package/import/接线与 §6 seam/shim 外无函数体变化；不新增 DB 写入、goroutine、包级可变状态（spec §4.3：seam 一律构造注入，禁包级 var 注入）。
6. **OCR/VLM 文本清洗行为不变**：`SanitizeOCRText`/`BuildVLMCaptionPrompt` 搬迁前后对 `ocr_sanitizer_test.go`/`image_multimodal_prompt_test.go` 全用例输出逐字节一致（conversation 调用方 temporary_document.go:541/:560 经 shim 拿到同一行为）。

## 6. 输入/输出与 Go 接口签名（全部为 2026-09-24 会话对 base `6bda27b1d` 实测；不发明）

### 6.1 搬迁后 ingest 包导出面（R1 三件套之 1/2：实现体随 `git mv`，落位包内导出薄包装）

| ingest 导出符号 | 真实签名（源定义实测） | 源定义 |
|---|---|---|
| `NewChunkRepository` | `func NewChunkRepository(db *gorm.DB) interfaces.ChunkRepository` | repository/chunk.go:32 |
| `ErrChunkRevisionConflict` | `var ErrChunkRevisionConflict = errors.New("chunk revision conflict")` | repository/chunk.go:17 |
| `NewChunkService` | `func NewChunkService(chunkRepository interfaces.ChunkRepository, knowledgeRepo interfaces.KnowledgeRepository, kbRepository interfaces.KnowledgeBaseRepository, modelService interfaces.ModelService, retrieveEngine interfaces.RetrieveEngineRegistry, ownership retriever.TenantStoreOwnership, task interfaces.TaskEnqueuer, spanTrace SpanTraceSeam, enqueueSummaryRefresh func(context.Context, interfaces.KnowledgeRepository, interfaces.TaskEnqueuer, KBByIDLookup, *types.Knowledge) error) interfaces.ChunkService` | service/chunk.go:46（原 `spanTracker SpanTracker` 参数按 §6.3 改为 seam 两参数——M3 编译修复，seam 属 R2 授权的构造注入接口） |
| `EnqueueDataTableSummaryIfNeeded` | `func EnqueueDataTableSummaryIfNeeded(e *DataTableSummaryEnqueuer, ctx context.Context, client interfaces.TaskEnqueuer, tenantID uint64, knowledgeID string, fileName, fileType, summaryModelID, embeddingModelID string)` | service/extract.go:162（原 8 参包级函数 + §6.3 三 helper seam） |
| `NewChunkExtractService` | 原 7 参（config *config.Config, modelService, knowledgeBaseRepo, knowledgeRepo, chunkRepo, graphEngine interfaces.RetrieveGraphRepository, spanTracker SpanTracker——extract.go:196）→ `spanTracker SpanTracker` 改为 `spanTrace SpanTraceSeam, attemptSupersededFn func(context.Context, string, int) bool`，返回 `interfaces.TaskHandler` | extract.go:196 |
| `NewDataTableSummaryService` | `func NewDataTableSummaryService(modelService interfaces.ModelService, knowledgeBaseService interfaces.KnowledgeBaseService, knowledgeService interfaces.KnowledgeService, fileService interfaces.FileService, chunkService interfaces.ChunkService, tenantService interfaces.TenantService, retrieveEngine interfaces.RetrieveEngineRegistry, ownership retriever.TenantStoreOwnership, sqlDB *sql.DB, storageResolver interfaces.StorageBackendResolver) interfaces.TaskHandler`（签名零变化） | extract.go:434 |
| `NewImageMultimodalService` | 原 14 参（image_multimodal.go:93，含 `spanTracker SpanTracker`）→ `spanTracker` 改 `spanTrace SpanTraceSeam`，追加 `knowledgeNotFoundErr, knowledgeBaseNotFoundErr error` 两参，返回 `interfaces.TaskHandler` | image_multimodal.go:93 |
| `BuildVLMCaptionPrompt` | `func BuildVLMCaptionPrompt(ctx context.Context, cfg types.VLMConfig) string` | image_multimodal.go:53（K0 §6.2 组 D） |
| `SanitizeOCRText` | `func SanitizeOCRText(raw string) string` | ocr_sanitizer.go:29（K0 §6.2 组 D） |
| `IsFinalAsynqAttempt` | `func IsFinalAsynqAttempt(ctx context.Context) bool` | image_multimodal.go:413 |
| `NewDataTableSummaryEnqueuer` | `func NewDataTableSummaryEnqueuer(normalizeFileExtension func(string) string, isDataTableFileType func(string) bool, getFileType func(string) string) *DataTableSummaryEnqueuer` | §6.3 |
| `ChunkHandler` / `NewChunkHandler` | `type ChunkHandler struct { service interfaces.ChunkService; kgService interfaces.KnowledgeService }`；`func NewChunkHandler(service interfaces.ChunkService, kgService interfaces.KnowledgeService) *ChunkHandler` | handler/chunk.go:30/:36 |
| `PreviewChunking` | `func PreviewChunking(c *gin.Context)` | handler/chunker_debug.go:123 |

未导出面（chunkRepository/chunkService/ChunkExtractService/DataTableSummaryService/ImageMultimodalService 结构体及其全部方法、`computeChunkSizeStats`、`shouldDropOrphanedMultimodal`、`diffFAQChunkIDsByContentHash`、`chunkIDHash`、`FAQChunkDiff` 等）随文件原样进入 ingest 包，跨包不可见即天然收敛（R3 同属主调用点直接改）。

### 6.2 R1 宿主薄 shim 清单（K1 定义、宿主仍被引用；shim 仅转发声明，文件头注 `// Pass B 过渡 shim：删除点 ib2（Integration Brief 登记）`）

| # | 宿主 shim（文件：`internal/application/service/chunk_ingest_shim.go`，除注明外） | 被保护的宿主引用（实测） |
|---|---|---|
| R1-1 | `var ErrChunkRevisionConflict = ingest.ErrChunkRevisionConflict` | `knowledge_process.go:2282`（K4） |
| R1-2 | `func NewChunkService(…同 §6.1 新签名…) interfaces.ChunkService { return ingest.NewChunkService(…) }` | container.go:375（dig Provide） |
| R1-3 | `func NewChunkExtractService(…) interfaces.TaskHandler` 转发 | container.go:409 |
| R1-4 | `func NewDataTableSummaryService(…) interfaces.TaskHandler` 转发 | container.go:410 |
| R1-5 | `func NewImageMultimodalService(…) interfaces.TaskHandler` 转发 | container.go:411 |
| R1-6 | `func NewChunkExtractTask(ctx context.Context, client interfaces.TaskEnqueuer, tenantID uint64, chunkID, modelID, knowledgeID string, attempt, chunkIndex int) (bool, error)` 转发 | knowledge_post_process.go:418（K4） |
| R1-7 | `func isFinalAsynqAttempt(ctx context.Context) bool { return ingest.IsFinalAsynqAttempt(ctx) }` | knowledge_process.go:1125/:1497/:1868（K4） |
| R1-8 | `internal/application/repository/chunk_ingest_shim.go`：`func NewChunkRepository(db *gorm.DB) interfaces.ChunkRepository { return ingest.NewChunkRepository(db) }` | container.go:205；宿主测试 `document_write_access_test.go`/`knowledge_caller_scope_test.go`（K4 属主测试，留在宿主） |
| R1-9 | `func isFinalAsynqAttempt` 同 R1-7；`func enqueueDataTableSummaryIfNeeded(ctx context.Context, client interfaces.TaskEnqueuer, tenantID uint64, knowledgeID string, fileName, fileType, summaryModelID, embeddingModelID string) { ingest.EnqueueDataTableSummaryIfNeeded(ingest.NewDataTableSummaryEnqueuer(normalizeFileExtension, isDataTableFileType, getFileType), ctx, client, tenantID, knowledgeID, fileName, fileType, summaryModelID, embeddingModelID) }`（host 包内以函数值传入本包 K4 helper，零复制） | knowledge_create.go:295/:746、knowledge_process.go:2629/:2682（K4） |
| R1-10 | `func sanitizeOCRText(raw string) string { return ingest.SanitizeOCRText(raw) }`；`func buildVLMCaptionPrompt(ctx context.Context, cfg types.VLMConfig) string { return ingest.BuildVLMCaptionPrompt(ctx, cfg) }` | **conversation**：temporary_document.go:541/:560（ib2 改写项，Brief 登记） |

注：`service/chunk.go:23` 的 `var ErrChunkRevisionConflict = repository.ErrChunkRevisionConflict` 别名随文件搬迁后与 ingest 包内 repository 侧定义同包重名——M3 中**删除别名行**，全包唯一定义为 R1-1 的真源（禁双写，framework:30）。

### 6.3 R2 消费侧 seam（K1 消费、他 plan 定义的未导出符号；定义于 `ingest/seams.go`，构造注入，spec §4.2）

| 符号（定义实测） | K1 调用点 | seam 具体化 |
|---|---|---|
| `writeExecutionTenant`（knowledge_write.go:32，K4）、`writeResourceIDs`（:17）、`loadKnowledgeWrite`（:60）、`loadKnowledgeWriteBatch`（:93）；其 `knowledgeBaseWriteLookup`（:13，单方法 `GetKnowledgeBaseByID(context.Context, string) (*types.KnowledgeBase, error)`） | chunk_write.go:11/:22/:48/:88；chunk.go（loadKnowledgeWriteBatch） | `type KBByIDLookup interface { GetKnowledgeBaseByID(context.Context, string) (*types.KnowledgeBase, error) }`（同时覆盖 knowledge_summary_refresh.go:16 `summaryKnowledgeBaseReader`）；`type KnowledgeWriteGuard interface { WriteResourceIDs([]string) ([]string, error); WriteExecutionTenant(context.Context) (uint64, error); LoadKnowledgeWrite(ctx context.Context, interfaces.KnowledgeRepository, KBByIDLookup, string) (*types.Knowledge, *types.KnowledgeBase, error); LoadKnowledgeWriteBatch(ctx context.Context, interfaces.KnowledgeRepository, KBByIDLookup, []string) ([]*types.Knowledge, error) }`；chunkService 增字段 `writeGuard KnowledgeWriteGuard`，chunk_write.go 四处调用改经 `s.writeGuard.X`（方法名一一对应，函数体其余零改动） |
| `SpanTracker`（knowledge_span_tracker.go:85，11 方法，句柄 `*Span` :72）、`noopSpanTracker`（:858） | extract.go:216-278（LookupStage/BeginSubSpan/FailSpan/EndSpan + `*Span` 局部变量 :248）；image_multimodal.go:129-222/:427 前 | `type SpanTraceSeam interface { LookupStage(ctx context.Context, knowledgeID string, attempt int, stage string) any; BeginSubSpan(ctx context.Context, parent any, name, kind string, input types.JSONMap) any; EndSpan(ctx context.Context, span any, out types.JSONMap); FailSpan(ctx context.Context, span any, code, message string, err error) }`（`any`=宿主 `*Span` 不透明句柄——`Span` 虽导出于宿主包，但 ingest 若 import 宿主 service 包将与 R1 shim 形成 `service→ingest→service` import 环，实测裁决不可行）；`noopSpanTraceSeam` 四方法实现零值语义（对照 knowledge_span_tracker.go:858-879 的 noop 行为：LookupStage→nil、BeginSubSpan→nil、EndSpan/FailSpan→空操作）；两服务字段 `spanTrace SpanTraceSeam`，`tracker()` nil 回退语义与原 `noopSpanTracker{}` 一致 |
| `attemptSuperseded`（knowledge.go:202，K4） | extract.go:238 | ChunkExtractService 增字段 `attemptSupersededFn func(ctx context.Context, knowledgeID string, attempt int) bool` |
| `enqueueSummaryRefresh`（knowledge_summary_refresh.go:83，参数 `summaryKnowledgeBaseReader` :16） | chunk.go:511-515 | NewChunkService 闭包参数（§6.1），chunkService 不再持有 spanTracker 字段 |
| `previewText`（wiki_ingest.go:1383，K3；K0 §6.2 组 A） | extract.go:309、image_multimodal.go:280/:296 | 两服务增字段 `previewTextFn func(s string, maxRunes int) string`（K0 §6.1 R2 明文裁定项；生产接线 K5 接 K3 导出 `PreviewText`） |
| `getFileType`（knowledge_util.go:89）、`normalizeFileExtension`（:47）、`isDataTableFileType`（:67）（均 K4） | extract.go:171（`enqueueDataTableSummaryIfNeeded` 体内） | `type DataTableSummaryEnqueuer struct { normalizeFileExtension func(string) string; isDataTableFileType func(string) bool; getFileType func(string) string }` + `EnqueueIfNeeded` 方法承载原函数体（三 helper 改经字段）；宿主经 R1-9 shim 传本包函数值，K4 搬 `knowledge_util.go` 导出后由其直接传导出形式 |
| `repository.ErrKnowledgeNotFound`（repository/knowledge.go:15，K4）、`repository.ErrKnowledgeBaseNotFound`（repository/knowledgebase.go:13，K4） | image_multimodal.go:377/:390（`errors.Is`） | ImageMultimodalService 增字段 `knowledgeNotFoundErr, knowledgeBaseNotFoundErr error`，调用点改 `errors.Is(err, s.knowledgeNotFoundErr)`；**禁 import 宿主 repository 包**（R1-8 shim 将形成 `repository→ingest→repository` import 环，实测裁决）；随迁的 image_multimodal_orphan_test.go 中 4 处 `repository.ErrKnowledgeNotFound/…BaseNotFound`（:54/:71/:84/:123）改用测试本地 `errors.New` 孪生值注入同名字段——比对语义不变（字段即比对目标），双跑差分锚定 |

**接线义务（写进 Brief，集成工程师在 K5/ib2 执行）**：`internal/application/service/span_trace_seam_adapter.go` 提供 `NewSpanTraceSeamAdapter(tr SpanTracker) ingest.SpanTraceSeam`（四方法纯转发）、`AttemptSupersededProvider(tr SpanTracker) func(context.Context, string, int) bool`、`EnqueueSummaryRefreshProvider(tr SpanTracker) func(context.Context, interfaces.KnowledgeRepository, interfaces.TaskEnqueuer, KBByIDLookup, *types.Knowledge) error`、`KnowledgeWriteGuardProvider() ingest.KnowledgeWriteGuard`（K4 搬迁后指到其导出包装）；container.go:374 的 `service.NewSpanTracker` 经上述适配器供给 R1-2/3/5 新参。K1→K4 期间 ingest 新路径未接线不影响运行（plan 20 §11 回滚边界同律）。

### 6.4 handler wrapper shim（跨 owner 方法保编译；`internal/handler/chunk_ingest_shim.go`）

`rbac_lookups.go:130/:149`（identity 属主，禁改）在 `*ChunkHandler` 上定义 `KBCreatorLookupFromKnowledgeIDParam`/`KBCreatorLookupFromChunkIDParam`，方法体访问字段 `h.kgService`（:135/:149 内）与 `h.service`——ChunkHandler 类型随 K1 搬出宿主包后，宿主包内方法定义即非法（Go：non-local type）。**alias 不可救**（alias 到外部类型同样禁定义方法），故 shim 采用**本包定义类型包装**：

```go
type ChunkHandler struct {
    *ingest.ChunkHandler                       // 路由调用的 14 方法经嵌入提升，行为同一实现
    service   interfaces.ChunkService          // rbac_lookups.go 方法体同名字段
    kgService interfaces.KnowledgeService
}
func NewChunkHandler(service interfaces.ChunkService, kgService interfaces.KnowledgeService) *ChunkHandler {
    return &ChunkHandler{ChunkHandler: ingest.NewChunkHandler(service, kgService), service: service, kgService: kgService}
}
func PreviewChunking(c *gin.Context) { ingest.PreviewChunking(c) }
```

rbac_lookups.go **一字不改**可编译（:103/:104 断言、:130/:149 方法定义、字段访问全部落在包装类型的自有字段）；router.go:55/:304、rbac.go:158、routes_knowledge.go:19/:28、container.go:719 全部保持编译。零复制实现：处理逻辑唯一存在于 ingest；包装仅持有同一注入实例的两个指针；rbac 方法逻辑仍唯一存在于 identity 属主文件。包装与字段副本的删除点=ib2（路由切模块门面 + identity 去方法化，Brief 登记）。

## 7. 必须删除/登记的 legacy、别名、例外

1. **manifest/matrix 行删除**：9 条 legacy_files 行 + 9 条 matrix 行随各自迁移 commit 删除（Ruling 4；`git diff <base>...HEAD` 与 owned_files 差集为空的核对含此二文件）。
2. **新例外登记（6 条，Ruling 2 独立 commit）**：9 文件迁入 `internal/modules/knowledge/ingest` 后落入 architectureguard forbidden-import 扫描集（check.go:1175-1224：模块文件跨模块 import 非 owner 模块子包即诊断），实测搬迁文件跨模块 import 清单：
   | # | ImporterFile（新） | ImportedPath | 现状等价物 |
   |---|---|---|---|
   | E1 | `internal/modules/knowledge/ingest/extract.go` | `…/internal/modules/airesource/models/chat` | extract.go:15（Pass A 直连，plan 20 §7.4 合法消费清单） |
   | E2 | 同上 | `…/internal/modules/airesource/models/embedding` | extract.go:16 |
   | E3 | 同上 | `…/internal/modules/agentruntime/agent/tools` | extract.go:13（`types.ToolResult` 消费） |
   | E4 | 同上 | `…/internal/modules/policy/access` | extract.go:19 |
   | E5 | `internal/modules/knowledge/ingest/image_multimodal.go` | `…/internal/modules/airesource/models/utils/ollama` | image_multimodal.go:15 |
   | E6 | 同上 | `…/internal/modules/airesource/models/vlm` | image_multimodal.go:16 |
   每条：`check.go` importExceptions 数据行（`PassBTask: "21-knowledge-ingest"`）+ `exception-ledger.yaml` 同窗新行（owner=21-knowledge-ingest，remove_at=ib2，reason=「搬迁显形横向耦合（Pass A 前宿主包直连，plan 20 §7.4 合法消费清单）」）。**禁**通配/前缀豁免、禁把依赖塞进 common、禁复制实现（framework:38）。
3. **exc-0088（exception-ledger.yaml:530-535，removal owner=本计划）**：`docparser/weknoracloud_http_reader.go`（Pass A 已在模块内，非本节点 9 文件）import `airesource/models/utils` 仅用 `utils.Sign`（:237，signer.go:24）。airesource 根门面现为零逻辑骨架（实测 `internal/modules/airesource/module.go` 无导出符号），本节点**无合法替代 import 可切**——处置：Brief 登记其 ib2 删除批（前置=airesource 根门面暴露 Sign 或等价端口，属 airesource/ib2 契约动作），报告按 conventions §5 如实登记该前置依赖；本节点不删行、不改 docparser 文件。
4. **别名 18 条（knowledge.yaml:41-77）**：删除归 K5（b2-k-integration），本节点零动作、零 import 变更涉别名。
5. **宿主 shim 的最终删除**：§4.1 全部 shim 文件（repository/service/handler 三处 + span_trace_seam_adapter.go）+ §6.3 seam 的生产接线替换，删除点=ib2，逐条入 Brief 删除批；ib2 前任何残留 importer 未清零禁止删除（conventions §8/§11 回滚边界同律）。

## 8. 实施步骤（一任务一逻辑单元；搬迁类任务按 spec §13 拆 M2+M3 两个 commit，commit message 模板见各任务；先 RED 后 GREEN）

### Task K1.0 — 基线对齐与特征化基线（M1）

- [ ] `git merge codex/passb-b2-k0`（独立 merge commit；冲突时 module.go 等冻结面停下升级，Ruling 3/conventions §10.4）。
- [ ] 前置条件 P0-P4 逐条核验，命令与退出码记入报告。
- [ ] **T0 特征化基线（RED 前置）**：在搬迁前 HEAD 记录 11 个随迁测试 + 差分用例的旧行为：

```bash
go test -count=1 -v ./internal/application/repository/ -run 'TestDiffFAQChunkIDsByContentHash|TestFAQChunkDiff|TestTagFieldUpdatesReturnAllAffectedChunks|TestSaveChunkRevisionIsAtomicAndOptimistic|TestCreateChunks|TestKnowledgeTag_SQLite'
go test -count=1 -v ./internal/application/service/ -run 'TestValidateEditedChunkImages|TestImageChildMatchesEditedContent|TestSyncEditedChunkImages|TestBuildSampleDataDescription|TestShouldDropOrphanedMultimodal|TestImageMultimodalHandle|TestSanitizeOCRText|TestBuildVLMCaptionPrompt|TestValidateParserOverrideURLs'
go test -count=1 -v ./internal/handler/ -run 'TestComputeChunkSizeStats|TestPreviewChunking'
```
  预期：全 PASS，用例清单+输出写入 evidence 差分章节「旧实现」栏。
- 验收：merge 后 `go build ./...` 退出码 0；`go test -count=1 ./internal/modules/knowledge/kbfreeze/` PASS。commit：`chore(passb): b2-k-ingest 基线对齐 b2-k0（merge）`、`test(passb): b2-k-ingest T0 特征化基线`。

### Task K1.1 — 搬迁 repository/chunk.go（M2+M3）

- [ ] M2：`git mv internal/application/repository/chunk.go internal/modules/knowledge/ingest/chunk_repo.go`（新建包 doc.go 同 commit）+ 4 个 `chunk_*_test.go` 随迁（同包改名 `package ingest`）+ 同 commit 删 knowledge.yaml:79 行与 matrix 对应行。
- [ ] M3：修 import（`internal/common`、`internal/types`、`internal/types/interfaces`、gorm 路径不变）；新建 `internal/application/repository/chunk_ingest_shim.go`（R1-8）；`go build ./...` 退出码 0。
- [ ] 预期：`go test -count=1 ./internal/modules/knowledge/ingest/ -run 'TestDiffFAQChunkIDsByContentHash|TestFAQChunkDiff|TestTagFieldUpdates|TestSaveChunkRevision|TestCreateChunks|TestKnowledgeTag_SQLite'` 全 PASS 且用例数与 T0 一致。commit：`refactor(knowledge): b2-k-ingest M2 纯移动 repository/chunk.go`、`refactor(knowledge): b2-k-ingest M3 chunk repo 编译修复与宿主 shim`。

### Task K1.2 — 搬迁 service/chunk.go + chunk_write.go（M2+M3）

- [ ] M2：两文件 `git mv` 入 ingest（chunk_service.go）+ `chunk_edit_parent_test.go` 随迁 + 删 knowledge.yaml:131/:135 行与 matrix 行。
- [ ] M3：新建 `ingest/seams.go`（`KBByIDLookup`、`KnowledgeWriteGuard`）；chunkService 增 `writeGuard` 字段、删 `spanTracker` 字段改 `spanTrace SpanTraceSeam`+`enqueueSummaryRefresh` 闭包字段；NewChunkService 按 §6.1 新签名；chunk_write.go 四调用点改经 `s.writeGuard`；删 :23 别名行（ErrChunkRevisionConflict 唯一化）；新建 `internal/application/service/chunk_ingest_shim.go`（R1-1/R1-2）。
- [ ] 预期：`go build ./...` 0；随迁测试 PASS（`TestValidateEditedChunkImages` 族，chunk_edit_parent_test 具名字段字面量不触及新字段，零修改可编译——若编译器证明需要注入，注入 no-op guard 并在报告登记）。commit：`refactor(knowledge): b2-k-ingest M2 纯移动 chunk service`、`refactor(knowledge): b2-k-ingest M3 chunk service seam 与 shim`。

### Task K1.3 — 搬迁 service/extract.go（M2+M3）

- [ ] M2：`git mv` + `extract_data_table_summary_test.go` 随迁 + 删 knowledge.yaml:139 行与 matrix 行。
- [ ] M3：`DataTableSummaryEnqueuer` 入 seams.go，`enqueueDataTableSummaryIfNeeded` 改 `EnqueueDataTableSummaryIfNeeded(e, …)` 导出；ChunkExtractService 改 `spanTrace`+`attemptSupersededFn`+`previewTextFn` 字段（:238/:250-252/:276/:278/:309 五调用点改经字段）；NewChunkExtractService/NewDataTableSummaryTask 按 §6.1；shim 追加 R1-3/R1-6/R1-9。
- [ ] 预期：`go build ./...` 0；`TestBuildSampleDataDescription*` 3 用例 PASS。commit：`refactor(knowledge): b2-k-ingest M2 纯移动 extract`、`refactor(knowledge): b2-k-ingest M3 extract 导出与 seam`。

### Task K1.4 — 搬迁 image_multimodal.go + ocr_sanitizer.go（M2+M3）

- [ ] M2：两文件 `git mv` + orphan/prompt/ocr 三测试随迁 + 删 knowledge.yaml:151/:291 行与 matrix 行。
- [ ] M3：`BuildVLMCaptionPrompt`/`SanitizeOCRText`/`IsFinalAsynqAttempt` 导出薄包装（落位包内一行委托，R1 之 2）；ImageMultimodalService 改 `spanTrace`+`previewTextFn`+两哨兵 error 字段（:276/:280/:289/:296/:377/:390 调用点改经字段/包装）；NewImageMultimodalService 按 §6.1；orphan 测试 4 处哨兵改本地孪生值注入（§6.3 末行）；shim 追加 R1-5/R1-7/R1-10。
- [ ] 预期：`go build ./...` 0；`TestShouldDropOrphanedMultimodal|TestImageMultimodalHandle*|TestSanitizeOCRText|TestBuildVLMCaptionPrompt` 用例数与 T0 一致且 PASS。commit：`refactor(knowledge): b2-k-ingest M2 纯移动 image_multimodal 与 ocr_sanitizer`、`refactor(knowledge): b2-k-ingest M3 组 D 符号导出与 conversation shim`。

### Task K1.5 — 搬迁 parser_url_security.go + handler/chunk.go + chunker_debug.go（M2+M3）

- [ ] M2：三文件 `git mv`（handler/chunk.go→chunk_handler.go、chunker_debug.go 同名）+ parser_url_security_test.go、chunker_debug_test.go 随迁 + 删 knowledge.yaml:295/:363/:367 行与 matrix 行。
- [ ] M3：parser_url_security 测试同包零改；handler wrapper shim `internal/handler/chunk_ingest_shim.go`（§6.4）；ingest 内 handler 文件引用 `service.ErrChunkRevisionConflict` 改同包裸标识符（:230/:292）；`go vet` 级自检 `go build ./...` 0。
- [ ] 预期：`go test -count=1 ./internal/modules/knowledge/ingest/ -run 'TestValidateParserOverrideURLs|TestComputeChunkSizeStats|TestPreviewChunking'` PASS；`go test -count=1 ./internal/handler/ ./internal/application/... ./internal/router/...` 编译通过（rbac_lookups_test.go 经 wrapper 保绿）。commit：`refactor(knowledge): b2-k-ingest M2 纯移动 parser_url_security 与 chunk handler`、`refactor(knowledge): b2-k-ingest M3 handler wrapper shim`。

### Task K1.6 — 例外登记（独立 commit）+ Integration Brief

- [ ] 独立 commit：§7.2 的 E1-E6 写入 `tools/architectureguard/check.go` importExceptions（数据行、`PassBTask: "21-knowledge-ingest"`）；同窗 `exception-ledger.yaml` 追加 6 行；`make check-backend-architecture` 退出码 0（禁改 guard 判定逻辑，仅数据行）。commit：`chore(passb): b2-k-ingest 登记 6 条搬迁显形 import 例外（Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY）`。
- [ ] 撰写 `docs/architecture/passb/briefs/b2-k-ingest.md`：①R1 shim 全清单（§6.2 十项，逐条列宿主引用 file:line）；②seam→生产接线表（§6.3，K5/ib2 由集成工程师执行：SpanTraceSeam 适配器、KnowledgeWriteGuard→K4 导出包装、previewTextFn→K3 `PreviewText`、哨兵 error→`repository.ErrKnowledgeNotFound/…BaseNotFound`、attemptSuperseded/enqueueSummaryRefresh 闭包）；③conversation 调用点 ib2 改写项（temporary_document.go:541/:560）；④handler wrapper 删除批与 identity 去方法化联动（rbac_lookups.go:130/:149）；⑤exc-0088 删除批及 airesource 门面前置；⑥E1-E6 例外 ib2 删除批。commit：`docs(passb): b2-k-ingest Integration Brief`。

### Task K1.7 — 差分双跑、证据、报告与门禁收口

- [ ] **同用例双跑比对（conventions §6，spec §14.3）**：以 K1.0 T0 用例清单为准，对搬迁后 ingest 包重跑同清单，逐用例等价比对（用例数、PASS/FAIL、关键断言输出），任何失败只修新实现、不改期望（spec §14.3）。高风险面锚点：`TestCreateChunks_SQLite_SeqID*`（SeqID 语义）、`TestSaveChunkRevisionIsAtomicAndOptimistic`（revision 原子性）、`TestTagFieldUpdatesReturnAllAffectedChunks`（TypeIndexDelete tag 侧）、`TestDiffFAQChunkIDsByContentHash*`（FAQ diff）、`TestImageMultimodalHandle*`（finalize-once/死信语义）。
- [ ] 节点 gates 四项（原文执行、退出码入报告）：

```bash
go build ./...
go test -count=1 ./internal/modules/knowledge/... 
make check-backend-architecture
make verify-module-moves
PASSB_BASE_SHA=$(git merge-base origin/main HEAD)
git diff --stat "$PASSB_BASE_SHA"...HEAD
git diff "$PASSB_BASE_SHA"...HEAD --name-only | sort   # 与 §4.1 可写清单求差集，非空即失败
```
- [ ] 写 `docs/architecture/evidence/passb/b2-k-ingest.md`（差分章节：用例清单、双跑输出、比对结论、命令与退出码）与 `docs/plans/passb/reports/b2-k-ingest.md`（命令台账、变更 vs owned_files 逐条核对、未完成项如实列出、§2 裁定 6 的跨节点义务转办确认、seam 具体化若经评审修订的偏差登记）。commit：`docs(passb): b2-k-ingest 差分证据与节点报告`。

## 9. 集成与回滚边界

- **集成边界**：本节点分支独立评审（`docs/plans/passb/reviews/b2-k-ingest.md`）后按缺省序 K1→K2→K3→K4 一次一支合入集成分支（framework:28）；装配切换（container.go:205/:375/:409-411/:719 的 Provide 换成 seam 接线版、router 路由切模块门面）仅集成工程师按 K5 Brief/IB2 执行；contracts.yaml knowledge 区状态回写仅 ib2（conventions §3）。
- **回滚边界**（spec §13）：M1（基线/测试/证据）revert 无装配影响；M2 纯移动与 M3 编译修复为分支内 revert；已并入集成分支的 M2/M3 保留（新路径未接线不影响运行——container 仍经宿主 shim 走旧装配）；本节点无 schema/migration 变更，回滚无需数据修复；例外数据行（E1-E6）revert 随其独立 commit。
- **升级边界**：门禁不可能通过、或 §6 seam 具体化与 K0 冻结表冲突时，按 conventions §5 在报告记录（根因/复现/建议裁定）并将 DAG notes 上报，禁止现场改判所有权、删测试、扩例外。

## 10. 独立验收标准

1. DAG b2-k-ingest gates 四项全绿，命令原文+退出码记录于 `docs/plans/passb/reports/b2-k-ingest.md`；
2. `git diff --name-only <base>...HEAD | sort` 与 §4.1 可写清单差集为空（含 knowledge.yaml/matrix 9+9 行删除、6 例外登记、brief/evidence/report）；
3. 9 个 legacy 文件在宿主目录不存在（`ls internal/application/repository/chunk.go internal/application/service/chunk.go internal/application/service/chunk_write.go internal/application/service/extract.go internal/application/service/image_multimodal.go internal/application/service/ocr_sanitizer.go internal/application/service/parser_url_security.go internal/handler/chunk.go internal/handler/chunker_debug.go` 全部 No such file），且 `internal/application/{repository,service}`、`internal/handler` 无本域转发业务声明（仅 §6.2/§6.4 薄 shim）；
4. 11 个随迁测试双跑（旧位置 T0 vs 新位置 ingest）逐用例等价，证据入 evidence 差分章节；`go test -count=1 ./internal/modules/knowledge/ingest/` PASS；
5. 宿主全绿不靠改对方文件：`rbac_lookups.go`、`temporary_document.go`、全部 K2/K3/K4 属主文件在 diff 中零出现；
6. `go test -count=1 ./internal/handler/ ./internal/application/... ` PASS（shim 保编译实证）；
7. `make check-backend-architecture` 0（E1-E6 生效、零通配例外）；
8. 计划评审 approved；报告含未完成项如实清单；Integration Brief 十类接线/删除项齐全。

## 11. 计划自检记录（撰写时执行，2026-09-24，base `6bda27b1d`）

- **Spec 覆盖**：ask 九要素 → §1（Spec 指针）、§3（前置）、§4（文件与写权）、§5-§7（接口签名/行为兼容/删除义务）、§8（步骤/命令/预期）、§9（集成与回滚）、§10（验收）——全覆盖；无 TBD/占位符。
- **实测锚点**：9 文件行数与符号定义行（NewChunkRepository:32、chunkService:28、NewChunkService:46、buildVLMCaptionPrompt:53、sanitizeOCRText:29、validateParserEngineOverrideURLs:22、ChunkHandler:30、PreviewChunking:123 等）、全部 K4 定义（knowledge_write.go:13/:17/:32/:60/:93、knowledge_span_tracker.go:72/:85/:172/:858、knowledge.go:202、knowledge_summary_refresh.go:16/:83、knowledge_util.go:47/:67/:89、knowledge.go:38 ErrChunkNotFound）、全部宿主引用（container.go:205/:374/:375/:409-411/:719、router.go:55/:304/:362、rbac.go:158、routes_knowledge.go:19/:28、task.go:266/:267/:307、sync_task.go:143/:144/:157、rbac_lookups.go:103/:104/:130/:149、temporary_document.go:541/:560、knowledge_process.go:1125/:1497/:1868/:2282、knowledge_create.go:295/:746、knowledge_post_process.go:418、knowledge_process.go:2629/:2682）逐条 grep/读源码命中。
- **类型一致性**：§6.1 全部签名转录自源码；两处构造签名变化（NewChunkService/NewChunkExtractService/NewImageMultimodalService 的 seam 参数化）均为 R2 授权的构造注入，真实参数类型（interfaces.*/retriever.TenantStoreOwnership/types.*）逐一核对。
- **跨任务/跨节点接口一致性**：§2 裁定 6 与 DAG b2-k-retrieval/b2-k-process notes 逐一对应（kb_activity 4 函数、withKnowledgeCleanup 不在本节点任何任务）；§6.2/§6.3 与 K0 §6.2 组 A/D、§9 K1 行义务（destination、组 D R1、组 A R2 seam、随迁测试、差分面、exc-0088）一致；§7.2 与 Ruling 2 机制一致；与 K0 表的**增量**（write-family、SpanTracker 族、attemptSuperseded、enqueueSummaryRefresh、getFileType 族——K0 §6.2 未枚举）均按既有 R2 机制具体化并在 §6.3 注明，不构成新裁定。
- **环依赖裁决（实测推导）**：ingest 禁 import 宿主 `internal/application/{service,repository}`——R1 shim 使两宿主包 import ingest，反向 import 即环；故 ErrKnowledgeNotFound/ErrKnowledgeBaseNotFound 走 error 字段注入、ErrChunkRevisionConflict 以 ingest 为唯一真源。
- **测试归属裁决**：`embed_channel_chunk_test.go` 被测为 `embedChannelService.chunkAllowedForEmbed`（embed_channel.go:285，36-channels 属主；对 ChunkService 仅接口 stub）——**不随迁**，plan 20 §10 粗粒度表述与 framework:29 定义文件属主细则的偏差在报告登记上报；`document_write_access_test.go`/`knowledge_caller_scope_test.go`（被测 knowledgeService，K4）与 `rbac_lookups_test.go`（被测 identity 属主 lookup，经 wrapper shim 保编译）**留在宿主**；随迁 11 件见 §4.1。
