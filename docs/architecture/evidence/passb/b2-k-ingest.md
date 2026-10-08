# b2-k-ingest 差分证据（docs/architecture/evidence/passb/b2-k-ingest.md）

> 节点：`b2-k-ingest`（plan `docs/plans/passb/21-knowledge-ingest.md`）。本文件按 conventions §6 / spec §14.3 记录高风险差分（knowledge deletion/indexing 面）双跑证据：旧实现（T0，搬迁前）→ 新实现（ingest 包）→ 逐用例等价比对。
> 基线：节点分支 BASE=`741cdb517490fffbd45325a7cdef16569dac7968`；K1.0 基线对齐 merge `codex/passb-b2-k0`（head `5bcb798621b856f7ff6497986c7ca79dba8c338e`）后 HEAD 上采集（merge 仅引入 K0 冻结产物，9 文件尚未搬迁，故即「旧实现」语义）。

## 差分章节

### 旧实现（T0 特征化基线，2026-09-24 采集）

命令原文（plan §8 Task K1.0，逐字执行）：

```bash
go test -count=1 -v ./internal/application/repository/ -run 'TestDiffFAQChunkIDsByContentHash|TestFAQChunkDiff|TestTagFieldUpdatesReturnAllAffectedChunks|TestSaveChunkRevisionIsAtomicAndOptimistic|TestCreateChunks|TestKnowledgeTag_SQLite'
go test -count=1 -v ./internal/application/service/ -run 'TestValidateEditedChunkImages|TestImageChildMatchesEditedContent|TestSyncEditedChunkImages|TestBuildSampleDataDescription|TestShouldDropOrphanedMultimodal|TestImageMultimodalHandle|TestSanitizeOCRText|TestBuildVLMCaptionPrompt|TestValidateParserOverrideURLs'
go test -count=1 -v ./internal/handler/ -run 'TestComputeChunkSizeStats|TestPreviewChunking'
```

结果摘要（退出码 / 顶层用例数 / 失败数 / 包级 `ok` 行）：

| 包 | 退出码 | 顶层 PASS | FAIL | `ok` 行 |
|---|---|---|---|---|
| `internal/application/repository` | 0 | 13 | 0 | `ok  github.com/Tencent/WeKnora/internal/application/repository  1.426s` |
| `internal/application/service` | 0 | 14 | 0 | `ok  github.com/Tencent/WeKnora/internal/application/service  5.352s` |
| `internal/handler` | 0 | 13 | 0 | `ok  github.com/Tencent/WeKnora/internal/handler  1.491s` |
| **合计** | — | **40** | **0** | 全 PASS |

用例清单（顶层 40 + 子用例 25，`=== RUN` 逐条转录；K1.7 双跑比对锚点）：

**repository（13 顶层 / 2 子用例）**
- TestCreateChunks_CleansContextHeaderBeforePersistence
- TestCreateChunks_SQLite_SeqIDAfterSoftDelete
- TestCreateChunks_SQLite_SeqIDAutoAssigned
- TestCreateChunks_SQLite_SeqIDContinuesFromExisting
- TestCreateChunks_SQLite_SeqIDUniqueAcrossKBs
- TestDiffFAQChunkIDsByContentHash
- TestDiffFAQChunkIDsByContentHash_dstDuplicateHash
- TestDiffFAQChunkIDsByContentHash_dstDuplicateHash_notInSrc
- TestDiffFAQChunkIDsByContentHash_emptySide
- TestFAQChunkDiff_SQLite
- TestKnowledgeTag_SQLite_SeqIDAutoAssigned
- TestSaveChunkRevisionIsAtomicAndOptimistic
- TestTagFieldUpdatesReturnAllAffectedChunks（子用例：flags_only、tag_only）

**service（14 顶层 / 21 子用例）**
- TestBuildSampleDataDescriptionIncludesDataAnalysisRows
- TestBuildSampleDataDescriptionLimitsRows
- TestBuildSampleDataDescriptionSupportsDecodedRows
- TestBuildVLMCaptionPrompt（子用例：defaults_to_context_language、uses_configured_language_and_custom_instructions）
- TestImageChildMatchesEditedContent
- TestImageMultimodalHandleDropFinalizesPendingCounter
- TestImageMultimodalHandleDropsMissingKnowledge
- TestImageMultimodalHandlePropagatesTransientKnowledgeError
- TestSanitizeOCRText（子用例 14：HTML_document_converted_to_markdown、HTML_with_only_whitespace_text、HTML_with_substantial_text_content_is_converted、code_block_wrapper_stripped、empty_string、html_code_block_wrapper_stripped、known_empty_reply_-_Chinese、known_empty_reply_-_no_text、known_empty_reply_-_图片中没有文字、multiple_blank_lines_collapsed、plain_text_with_minimal_HTML_not_converted、pure_HTML_skeleton_with_no_text、valid_markdown_passes_through、whitespace_only）
- TestShouldDropOrphanedMultimodal
- TestSyncEditedChunkImagesDisablesAndRestoresImageChildren
- TestValidateEditedChunkImages
- TestValidateParserOverrideURLsIgnoresNonURLKeys
- TestValidateParserOverrideURLsRejectsEverySupportedURLKey（子用例 5：mineru_endpoint、mineru_vlm_server_url、odl_hybrid_url、paddleocr_vl_cloud_base_url、paddleocr_vl_endpoint）

**handler（13 顶层 / 2 子用例）**
- TestComputeChunkSizeStats_Empty
- TestComputeChunkSizeStats_NoVarianceUnderflow
- TestComputeChunkSizeStats_SingleChunk
- TestComputeChunkSizeStats_VaryingSizes
- TestPreviewChunking_ChunkTruncation
- TestPreviewChunking_HappyPath_AutoStrategy
- TestPreviewChunking_LegacyStrategy_NoProfile
- TestPreviewChunking_LineEndingsMatchUpload（子用例：pasted_LF、uploaded_CRLF）
- TestPreviewChunking_ParentChildDefaultSizes
- TestPreviewChunking_ParentChildMatchesIngestion
- TestPreviewChunking_RejectsEmptyText
- TestPreviewChunking_RejectsOversizedText
- TestPreviewChunking_SingleLevelUnchanged

高风险面锚点（plan §8 K1.7 指定，旧位置 T0 状态）：

| 锚点用例 | T0 状态 |
|---|---|
| TestCreateChunks_SQLite_SeqID*（SeqID 语义，4 用例） | PASS |
| TestSaveChunkRevisionIsAtomicAndOptimistic（revision 原子性） | PASS |
| TestTagFieldUpdatesReturnAllAffectedChunks（TypeIndexDelete tag 侧） | PASS |
| TestDiffFAQChunkIDsByContentHash*（FAQ diff，4 用例） | PASS |
| TestImageMultimodalHandle*（finalize-once/死信语义，3 用例） | PASS |

### 新实现（ingest 包，搬迁后重跑同清单，2026-09-25 K1.7 采集）

命令原文（T0 三条命令的 -run 模式合并为一条——9 文件 + 11 随迁测试已同包落位 `internal/knowledge/ingest`，逐字保留全部用例名模式）：

```bash
go test -count=1 -v ./internal/knowledge/ingest/ -run 'TestDiffFAQChunkIDsByContentHash|TestFAQChunkDiff|TestTagFieldUpdatesReturnAllAffectedChunks|TestSaveChunkRevisionIsAtomicAndOptimistic|TestCreateChunks|TestKnowledgeTag_SQLite|TestValidateEditedChunkImages|TestImageChildMatchesEditedContent|TestSyncEditedChunkImages|TestBuildSampleDataDescription|TestShouldDropOrphanedMultimodal|TestImageMultimodalHandle|TestSanitizeOCRText|TestBuildVLMCaptionPrompt|TestValidateParserOverrideURLs|TestComputeChunkSizeStats|TestPreviewChunking'
```

结果摘要（退出码 0；`ok  github.com/Tencent/WeKnora/internal/knowledge/ingest  1.391s`）：

| 指标 | T0（三包分跑） | K1.7（ingest 单包） | 等价 |
|---|---|---|---|
| 顶层用例 | 40（repository 13 + service 14 + handler 13） | 40（同包合并） | ✅ |
| 子用例 | 25（2 + 21 + 2） | 25 | ✅ |
| FAIL | 0 | 0（`grep -c FAIL` = 0） | ✅ |
| 退出码 | 0 ×3 | 0 | ✅ |

顶层用例清单逐名比对：`grep '^--- PASS'` 提取 40 名排序后与 T0 清单（本文件上文逐条转录）**完全一致**（含 TestCreateChunks_SQLite_SeqID* 4 用例、TestDiffFAQChunkIDsByContentHash 族 4、TestImageMultimodalHandle* 3、TestPreviewChunking_* 9、TestComputeChunkSizeStats_* 4）；子用例 25 名（`grep '^    --- PASS'`）与 T0 转录逐一对应：TestBuildVLMCaptionPrompt 2、TestPreviewChunking_LineEndingsMatchUpload 2（pasted_LF/uploaded_CRLF）、TestSanitizeOCRText 14、TestTagFieldUpdatesReturnAllAffectedChunks 2（flags_only/tag_only）、TestValidateParserOverrideURLsRejectsEverySupportedURLKey 5。

高风险面锚点（plan §8 K1.7 指定，新实现状态 + 关键断言输出摘录）：

| 锚点用例 | K1.7 状态 | 关键输出摘录 |
|---|---|---|
| TestCreateChunks_SQLite_SeqID*（4 用例：AutoAssigned/ContinuesFromExisting/AfterSoftDelete/UniqueAcrossKBs） | PASS ×4 | `--- PASS: TestCreateChunks_SQLite_SeqIDAutoAssigned (0.00s)` 等 4 行 |
| TestSaveChunkRevisionIsAtomicAndOptimistic（revision 原子性） | PASS | `--- PASS: TestSaveChunkRevisionIsAtomicAndOptimistic (0.00s)` |
| TestTagFieldUpdatesReturnAllAffectedChunks（TypeIndexDelete tag 侧） | PASS（含 flags_only/tag_only 子用例） | `--- PASS: TestTagFieldUpdatesReturnAllAffectedChunks/flags_only (0.00s)` |
| TestDiffFAQChunkIDsByContentHash*（FAQ diff，4 用例） | PASS ×4 | `--- PASS: TestDiffFAQChunkIDsByContentHash (0.00s)` 等 |
| TestImageMultimodalHandle*（finalize-once/死信语义，3 用例） | PASS ×3 | 日志链完整复现：`image_multimodal.go:187 [Handle] Processing image` → `:203 Dropping task chunk= knowledge=missing` → `:724 [checkAndFinalizeAllImages] All images processed for knowledge missing. Finalizing...` |

### 逐用例等价比对（K1.7 结论）

- **用例数等价**：40 顶层 + 25 子用例，双跑两侧计数一致（T0 见上文章节；K1.7 见上表）。
- **清单等价**：顶层 40 名、子用例 25 名逐名一致（两侧均为 `=== RUN`/`--- PASS` 全集转录比对，非抽样）。
- **结果等价**：两侧 0 FAIL；无 skip、无 timeout。
- **行为等价（关键断言输出）**：TestImageMultimodalHandle* 三用例的 Handle→Drop→Finalize 日志链在 K1.7 输出中逐行复现（file:line 锚点随迁后仍为 image_multimodal.go:187/:203/:724，纯移动验证旁证）；其余用例为断言型测试（失败即 FAIL），全 PASS 即行为锚定。
- **结论**：同用例双跑**逐用例等价**，高风险面（knowledge deletion/indexing，framework:40）五个锚点族全部双侧 PASS——spec §14.3 差分要求满足，legacy 删除（K1.1–K1.5 各 M2 commit）前置于本比对的时序约束在 K1.1–K1.5 会话已按任务内双跑履行，本节为节点级收口重证。

## §8 计数基线登记：例外台账 105 → 111（K1.6，Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY）

- **变更**：`docs/architecture/passb/exception-ledger.yaml` 追加 6 条 K1 搬迁显形 import 例外（exc-0106..exc-0111，owner=21-knowledge-ingest，remove_at=ib2），同窗 `tools/architectureguard/check.go` importExceptions 数据行（PassBTask=B-knowledge——模块级口径，沿 K2.3/K2.4 先例；plan §7.2 字面的 `21-knowledge-ingest` 不在 passbguard PassBTaskModule 映射表（tools/passbguard/check.go:83-93），字面采用将在 ib2 触发 `exception-task-module` 诊断，偏差如实登记待协调者确认）。头计数注释 105→111。
- **实测集 vs 计划预测集偏差**：plan §7.2 预测 E1-E6 中 **E3（extract.go→agentruntime/agent/tools）无需登记**——K1.3 import 环裁决（CYCLE-FORCED-COMPOSITION 同律）将 tools/chat_pipeline 消费改走 `DataAnalysisToolSeam`/`GraphExtractorSeam` 消费侧 seam，extract.go 保持零 agentruntime import（Brief §4 已留痕）；**新增 1 条计划未预测**：`ingest/seams.go→airesource/models/chat`（seam 类型签名承载 chat.Chat，K1.3 产物）。实测 6 条 = E1/E2/E4/E5/E6 + seams 行，与 `make check-backend-architecture` 诊断逐条对齐。
- **exc-id 跨分支续号说明**：并行兄弟分支（retrieval 至 exc-0110、wikifaq 至 exc-0118、ac-skills/ac-market 至 exc-0113）各自从本分支上界独立续号，集成侧合并如撞号按 (from,to) 边键重排（passbguard exception-overlap 以边为键，不以 id）。
- **验证命令与退出码**：登记前 `make check-backend-architecture` 退出码 1（6 forbidden-import + 4 legacy-guard）；登记后退出码 1（仅余 4 legacy-guard 过渡 shim 诊断——K1.1 报告遗留 1 的已升级项，见 Brief/K1.6 报告），6 条 forbidden-import 全消解；`make verify-module-moves` 退出码 0（16 manifests OK）；`go run ./tools/passbguard -root .` 零 `exception-*` 新增诊断（既有契约/事件漂移项不变，Brief §3 ib2 回写批）。

## §8 计数基线登记：legacy 台账 396 → 391（K1.6，Ruling 2026-09-25-TRANSITION-SHIM-ROW-REGISTRATION）

- **变更**：4 个宿主过渡 shim 以「knowledge.yaml legacy_files 行 + ownership-matrix.yaml 行」成对补行（Ruling 方案 a，K1.1 报告遗留 1 升级、协调者 2026-09-25 批准）：`internal/application/repository/chunk_ingest_shim.go`、`internal/application/service/chunk_ingest_shim.go`、`internal/application/service/span_trace_seam_adapter.go`、`internal/handler/chunk_ingest_shim.go`（manifest reason「Pass B 过渡 shim，ib2 同 commit 随文件删行」/passb_task=B-knowledge；matrix module=knowledge/plan=21-knowledge-ingest/destination=internal/knowledge/ingest/integration_owner=delete_barrier=ib2）。
- **计数构成**：396 − 9（K1.1–K1.5 迁移删行，Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP 范畴，本窗口补台账登记）+ 4（shim 行）= **391**；`tools/passbguard/ownership_test.go` wantPerModule knowledge 84→79 纯机械修正（判定逻辑不变，沿 Ruling 1 的 appconnector 7→1 先例）。台账条目落 `docs/architecture/evidence/pass-a-acceptance.md` §6（本分支新增该节；并行兄弟分支各自登记本分支基线，ib2 集成侧按 F5 口径合并复核）。
- **ib2 闭环义务**：k-integration/ib2 收口时 shim 文件与其 manifest/matrix 行同 commit 删除，届时计数回落同样走 §8 登记（已写入 Brief 删除批）。
- **验证命令与退出码**：见 K1.6 报告 §2——`make check-backend-architecture` 退出码 **0**（4 条 legacy-guard 全消解）、`make verify-module-moves` 退出码 0、`go run ./tools/passbguard -root .` 零新增诊断类别。
