# knowledge — legacy 遗留文件索引（横向包内归属本模块的文件）

与 `docs/architecture/moves/knowledge.yaml` 的 `legacy_files` 逐条镜像（同路径、同 Pass B 任务）。
非测试 `.go` 文件；同目录 `_test.go` 随主题文件一并搬迁。

| 文件 | 导航标签 | Pass B 任务 |
|---|---|---|
| `internal/application/repository/chunk.go` | Chunk (application/repository) | `B-knowledge` |
| `internal/application/repository/kbshare.go` | Kbshare (application/repository) | `B-knowledge` |
| `internal/application/repository/knowledge.go` | Knowledge (application/repository) | `B-knowledge` |
| `internal/application/repository/knowledge_span_repo.go` | Knowledge span repo (application/repository) | `B-knowledge` |
| `internal/application/repository/knowledge_tag.go` | Knowledge tag (application/repository) | `B-knowledge` |
| `internal/application/repository/knowledge_transfer.go` | Knowledge transfer (application/repository) | `B-knowledge` |
| `internal/application/repository/knowledgebase.go` | Knowledgebase (application/repository) | `B-knowledge` |
| `internal/application/repository/semantic_model_invocation.go` | Semantic model invocation (application/repository) | `B-knowledge` |
| `internal/application/repository/semantic_model_policy.go` | Semantic model policy (application/repository) | `B-knowledge` |
| `internal/application/repository/semantic_outbox.go` | Semantic outbox (application/repository) | `B-knowledge` |
| `internal/application/repository/semantic_scope_epoch.go` | Semantic scope epoch (application/repository) | `B-knowledge` |
| `internal/application/repository/tag.go` | Tag (application/repository) | `B-knowledge` |
| `internal/application/repository/wiki_page.go` | Wiki page (application/repository) | `B-knowledge` |
| `internal/application/service/chunk.go` | Chunk (application/service) | `B-knowledge` |
| `internal/application/service/chunk_write.go` | Chunk write (application/service) | `B-knowledge` |
| `internal/application/service/extract.go` | Extract (application/service) | `B-knowledge` |
| `internal/application/service/faq_clone_sync.go` | Faq clone sync (application/service) | `B-knowledge` |
| `internal/application/service/graph.go` | Graph (application/service) | `B-knowledge` |
| `internal/application/service/image_multimodal.go` | Image multimodal (application/service) | `B-knowledge` |
| `internal/application/service/kb_activity.go` | Kb activity (application/service) | `B-knowledge` |
| `internal/application/service/kbshare.go` | Kbshare (application/service) | `B-knowledge` |
| `internal/application/service/knowledge.go` | Knowledge (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_auto_tag.go` | Knowledge auto tag (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_clone_move.go` | Knowledge clone move (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_create.go` | Knowledge create (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_delete.go` | Knowledge delete (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_delete_plan.go` | Knowledge delete plan (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_faq.go` | Knowledge faq (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_faq_batch.go` | Knowledge faq batch (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_faq_create_guard.go` | Knowledge faq create guard (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_faq_import.go` | Knowledge faq import (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_housekeeping.go` | Knowledge housekeeping (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_index_content.go` | Knowledge index content (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_post_process.go` | Knowledge post process (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_process.go` | Knowledge process (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_process_config.go` | Knowledge process config (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_reparse_scope.go` | Knowledge reparse scope (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_replace.go` | Knowledge replace (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_span_tracker.go` | Knowledge span tracker (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_summary_refresh.go` | Knowledge summary refresh (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_task_options.go` | Knowledge task options (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_transfer.go` | Knowledge transfer (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_util.go` | Knowledge util (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_write.go` | Knowledge write (application/service) | `B-knowledge` |
| `internal/application/service/knowledgebase.go` | Knowledgebase (application/service) | `B-knowledge` |
| `internal/application/service/knowledgebase_access.go` | Knowledgebase access (application/service) | `B-knowledge` |
| `internal/application/service/knowledgebase_search.go` | Knowledgebase search (application/service) | `B-knowledge` |
| `internal/application/service/knowledgebase_search_fanout.go` | Knowledgebase search fanout (application/service) | `B-knowledge` |
| `internal/application/service/knowledgebase_search_faq.go` | Knowledgebase search faq (application/service) | `B-knowledge` |
| `internal/application/service/knowledgebase_search_fusion.go` | Knowledgebase search fusion (application/service) | `B-knowledge` |
| `internal/application/service/knowledgebase_search_results.go` | Knowledgebase search results (application/service) | `B-knowledge` |
| `internal/application/service/knowledgebase_search_shared.go` | Knowledgebase search shared (application/service) | `B-knowledge` |
| `internal/application/service/knowledgebase_search_storegroup.go` | Knowledgebase search storegroup (application/service) | `B-knowledge` |
| `internal/application/service/ocr_sanitizer.go` | Ocr sanitizer (application/service) | `B-knowledge` |
| `internal/application/service/parser_url_security.go` | Parser url security (application/service) | `B-knowledge` |
| `internal/application/service/semantic_model.go` | Semantic model (application/service) | `B-knowledge` |
| `internal/application/service/semantic_model_capability.go` | Semantic model capability (application/service) | `B-knowledge` |
| `internal/application/service/semantic_model_policy.go` | Semantic model policy (application/service) | `B-knowledge` |
| `internal/application/service/semantic_scope.go` | Semantic scope (application/service) | `B-knowledge` |
| `internal/application/service/slug_fuzzy.go` | Slug fuzzy (application/service) | `B-knowledge` |
| `internal/application/service/tag.go` | Tag (application/service) | `B-knowledge` |
| `internal/application/service/tag_access.go` | Tag access (application/service) | `B-knowledge` |
| `internal/application/service/wiki_ingest.go` | Wiki ingest (application/service) | `B-knowledge` |
| `internal/application/service/wiki_ingest_batch.go` | Wiki ingest batch (application/service) | `B-knowledge` |
| `internal/application/service/wiki_ingest_cite.go` | Wiki ingest cite (application/service) | `B-knowledge` |
| `internal/application/service/wiki_ingest_dedup.go` | Wiki ingest dedup (application/service) | `B-knowledge` |
| `internal/application/service/wiki_ingest_taxonomy.go` | Wiki ingest taxonomy (application/service) | `B-knowledge` |
| `internal/application/service/wiki_linkify.go` | Wiki linkify (application/service) | `B-knowledge` |
| `internal/application/service/wiki_lint.go` | Wiki lint (application/service) | `B-knowledge` |
| `internal/application/service/wiki_page.go` | Wiki page (application/service) | `B-knowledge` |
| `internal/application/service/wiki_slug_handles.go` | Wiki slug handles (application/service) | `B-knowledge` |
| `internal/handler/chunk.go` | Chunk (handler) | `B-knowledge` |
| `internal/handler/chunker_debug.go` | Chunker debug (handler) | `B-knowledge` |
| `internal/handler/faq.go` | Faq (handler) | `B-knowledge` |
| `internal/handler/kb_access.go` | Kb access (handler) | `B-knowledge` |
| `internal/handler/knowledge.go` | Knowledge (handler) | `B-knowledge` |
| `internal/handler/knowledge_download.go` | Knowledge download (handler) | `B-knowledge` |
| `internal/handler/knowledgebase.go` | Knowledgebase (handler) | `B-knowledge` |
| `internal/handler/semantic_internal.go` | Semantic internal (handler) | `B-knowledge` |
| `internal/handler/semantic_model_policy.go` | Semantic model policy (handler) | `B-knowledge` |
| `internal/handler/session/wiki_fixer_scope.go` | Wiki fixer scope (handler/session) | `B-knowledge` |
| `internal/handler/tag.go` | Tag (handler) | `B-knowledge` |
| `internal/handler/task_progress_auth.go` | Task progress auth (handler) | `B-knowledge` |
| `internal/handler/wiki_page.go` | Wiki page (handler) | `B-knowledge` |
