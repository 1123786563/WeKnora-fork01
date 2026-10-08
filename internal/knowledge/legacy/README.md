# knowledge — legacy 遗留文件索引（横向包内归属本模块的文件）

与 `docs/architecture/moves/knowledge.yaml` 的 `legacy_files` 逐条镜像（同路径、同 Pass B 任务）。
非测试 `.go` 文件；同目录 `_test.go` 随主题文件一并搬迁。
标 **已迁移** 的行是已物理落位 `internal/knowledge/...`（retrieval/app、process 等模块内包）的迁移轨迹记录（manifest 行已按 Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP 随物理迁移 commit 删除，此镜像行保留至 ib2 收口）。

| 文件 | 导航标签 | Pass B 任务 |
|---|---|---|
| `internal/application/repository/kbshare.go` | Kbshare (application/repository) | `B-knowledge` |
| `internal/application/repository/knowledge.go` | Knowledge (application/repository) | `B-knowledge` |
| `internal/application/repository/knowledge_span_repo.go` | Knowledge span repo (application/repository) | `B-knowledge` |
| `internal/application/repository/knowledge_tag.go` | Knowledge tag (application/repository) | `B-knowledge` |
| `internal/application/repository/knowledge_transfer.go` | Knowledge transfer (application/repository) | `B-knowledge` |
| `internal/application/repository/knowledgebase.go` | Knowledgebase (application/repository) | `B-knowledge` |
| `internal/application/repository/kbretrieval_passb_compat.go` | Knowledge retrieval host compat (application/repository) | `B-knowledge`（K2.2 新增 compat） |
| `internal/application/repository/semantic_model_invocation.go` | Semantic model invocation (application/repository) | `B-knowledge` — **已迁移**（K2.2 → `retrieval/app/repository/`） |
| `internal/application/repository/semantic_model_policy.go` | Semantic model policy (application/repository) | `B-knowledge` — **已迁移**（K2.2 → `retrieval/app/repository/`） |
| `internal/application/repository/semantic_outbox.go` | Semantic outbox (application/repository) | `B-knowledge` — **已迁移**（K2.2 → `retrieval/app/repository/`） |
| `internal/application/repository/semantic_scope_epoch.go` | Semantic scope epoch (application/repository) | `B-knowledge` — **已迁移**（K2.2 → `retrieval/app/repository/`） |
| `internal/application/repository/tag.go` | Tag (application/repository) | `B-knowledge` — **已迁移**（K2.2 → `retrieval/app/repository/`） |
| `internal/application/service/graph.go` | Graph (application/service) | `B-knowledge` — **已迁移**（K2.4 → `retrieval/app/`） |
| `internal/application/service/kb_activity.go` | Kb activity (application/service) | `B-knowledge` — **已迁移**（K2.5 → `retrieval/app/`） |
| `internal/application/service/kbshare.go` | Kbshare (application/service) | `B-knowledge` |
| `internal/application/service/kbretrieval_passb_compat.go` | Knowledge retrieval host compat (application/service) | `B-knowledge`（K2.3 新增 compat） |
| `internal/application/service/knowledge.go` | Knowledge (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_auto_tag.go` | Knowledge auto tag (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_clone_move.go` | Knowledge clone move (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_create.go` | Knowledge create (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_delete.go` | Knowledge delete (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_delete_plan.go` | Knowledge delete plan (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_housekeeping.go` | Knowledge housekeeping (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_index_content.go` | Knowledge index content (application/service) | `B-knowledge` — **已迁移**（K4.2 → `process/`） |
| `internal/application/service/knowledge_post_process.go` | Knowledge post process (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_process.go` | Knowledge process (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_process_config.go` | Knowledge process config (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_reparse_scope.go` | Knowledge reparse scope (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_replace.go` | Knowledge replace (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_span_tracker.go` | Knowledge span tracker (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_summary_refresh.go` | Knowledge summary refresh (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_task_options.go` | Knowledge task options (application/service) | `B-knowledge` — **已迁移**（K4.2 → `process/`） |
| `internal/application/service/knowledge_transfer.go` | Knowledge transfer (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_util.go` | Knowledge util (application/service) | `B-knowledge` |
| `internal/application/service/knowledge_write.go` | Knowledge write (application/service) | `B-knowledge` — **已迁移**（K4.2 → `process/`） |
| `internal/application/service/knowledgebase.go` | Knowledgebase (application/service) | `B-knowledge` |
| `internal/application/service/knowledgebase_access.go` | Knowledgebase access (application/service) | `B-knowledge` — **已迁移**（K2.4 → `retrieval/app/`） |
| `internal/application/service/knowledgebase_search.go` | Knowledgebase search (application/service) | `B-knowledge` |
| `internal/application/service/knowledgebase_search_fanout.go` | Knowledgebase search fanout (application/service) | `B-knowledge` |
| `internal/application/service/knowledgebase_search_faq.go` | Knowledgebase search faq (application/service) | `B-knowledge` |
| `internal/application/service/knowledgebase_search_fusion.go` | Knowledgebase search fusion (application/service) | `B-knowledge` |
| `internal/application/service/knowledgebase_search_results.go` | Knowledgebase search results (application/service) | `B-knowledge` |
| `internal/application/service/knowledgebase_search_shared.go` | Knowledgebase search shared (application/service) | `B-knowledge` |
| `internal/application/service/knowledgebase_search_storegroup.go` | Knowledgebase search storegroup (application/service) | `B-knowledge` |
| `internal/application/service/semantic_model.go` | Semantic model (application/service) | `B-knowledge` |
| `internal/application/service/semantic_model_capability.go` | Semantic model capability (application/service) | `B-knowledge` — **已迁移**（K2.3 → `retrieval/app/`） |
| `internal/application/service/semantic_model_policy.go` | Semantic model policy (application/service) | `B-knowledge` — **已迁移**（K2.3 → `retrieval/app/`） |
| `internal/application/service/semantic_scope.go` | Semantic scope (application/service) | `B-knowledge` — **已迁移**（K2.3 → `retrieval/app/`） |
| `internal/application/service/slug_fuzzy.go` | Slug fuzzy (application/service) | `B-knowledge` — **已迁移**（K2.4 → `retrieval/app/`） |
| `internal/application/service/tag.go` | Tag (application/service) | `B-knowledge` |
| `internal/application/service/tag_access.go` | Tag access (application/service) | `B-knowledge` |
| `internal/handler/kb_access.go` | Kb access (handler) | `B-knowledge` — **已迁移**（K4.3 → `process/handler/`） |
| `internal/handler/kbretrieval_passb_compat.go` | Knowledge retrieval host compat (handler) | `B-knowledge`（K2.6 新增 compat） |
| `internal/handler/knowledge.go` | Knowledge (handler) | `B-knowledge` |
| `internal/handler/knowledge_download.go` | Knowledge download (handler) | `B-knowledge` |
| `internal/handler/knowledgebase.go` | Knowledgebase (handler) | `B-knowledge` |
| `internal/handler/semantic_internal.go` | Semantic internal (handler) | `B-knowledge` — **已迁移**（K2.6 → `retrieval/app/handler/`） |
| `internal/handler/semantic_model_policy.go` | Semantic model policy (handler) | `B-knowledge` — **已迁移**（K2.6 → `retrieval/app/handler/`） |
| `internal/handler/tag.go` | Tag (handler) | `B-knowledge` — **已迁移**（K2.6 → `retrieval/app/handler/`） |
| `internal/handler/task_progress_auth.go` | Task progress auth (handler) | `B-knowledge` — **已迁移**（K4.3 → `process/handler/`） |
| `internal/application/repository/chunk_ingest_shim.go` | Chunk ingest shim (application/repository) | `B-knowledge` |
| `internal/application/service/chunk_ingest_shim.go` | Chunk ingest shim (application/service) | `B-knowledge` |
| `internal/application/service/span_trace_seam_adapter.go` | Span trace seam adapter (application/service) | `B-knowledge` |
| `internal/handler/chunk_ingest_shim.go` | Chunk ingest shim (handler) | `B-knowledge` |
| `internal/application/service/kbprocess_passb_compat.go` | Knowledge process host compat (application/service) | `B-knowledge`（K4.2 新增 compat） |
| `internal/handler/kbprocess_passb_compat.go` | Knowledge process host compat (handler) | `B-knowledge`（K4.3 新增 compat） |
