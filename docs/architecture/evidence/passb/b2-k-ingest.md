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

### 新实现（ingest 包，搬迁后重跑同清单）

（K1.7 填写：同命令对新包路径重跑的用例清单、输出、退出码。）

### 逐用例等价比对

（K1.7 填写：用例数、PASS/FAIL、关键断言输出比对结论。）
