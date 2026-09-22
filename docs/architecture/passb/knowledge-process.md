# Pass B Brief — B-knowledge-process（knowledge 处理流水线域文件拆分）

Manifest：`docs/architecture/moves/knowledge.yaml`（模块 knowledge）。本 brief 只覆盖
其 legacy_files 的**处理流水线子集**（28 文件）。

## Scope（legacy_files 子集，28 文件）

repository（5）：knowledge.go、knowledge_span_repo.go、knowledge_tag.go、
knowledge_transfer.go、knowledgebase.go

service（19）：knowledge.go、knowledge_auto_tag.go、knowledge_clone_move.go、
knowledge_create.go、knowledge_delete.go、knowledge_delete_plan.go、
knowledge_housekeeping.go、knowledge_index_content.go、knowledge_post_process.go、
knowledge_process.go、knowledge_process_config.go、knowledge_reparse_scope.go、
knowledge_replace.go、knowledge_span_tracker.go、knowledge_summary_refresh.go、
knowledge_task_options.go、knowledge_util.go、knowledge_write.go、knowledge_transfer.go

handler（4）：kb_access.go、knowledge.go、knowledge_download.go、task_progress_auth.go

（完整路径前缀：repository=`internal/application/repository/`、
service=`internal/application/service/`、handler=`internal/handler/`。）

## 边界目标

文档处理流水线（TypeDocumentProcess/TypeManualProcess/TypeKnowledgeListReparse →
解析→分块→向量化→索引）、删除/级联（TypeKnowledgeListDelete/TypeIndexDelete/TypeKBDelete
的删除计划与执行）、克隆/迁移（TypeKBClone/TypeKnowledgeMove）、后处理链
（TypeKnowledgePostProcess/TypeKnowledgeAutoTag/TypeSummaryGeneration/
TypeQuestionGeneration/TypeDataTableSummary）归入 `internal/modules/knowledge`
（建议 `…/knowledge/process`）。**删除与索引语义是外部契约**：asynq 任务类型、
幂等/重试语义、Redis/Lite 双池注册行为零变化。

## 删除义务

- 上述 28 文件从横向包删除；
- 18 个 knowledge worker 处理器句柄改由模块暴露（`internal/router/task.go:266-320`、
  `sync_task.go:143-163` 的注册表切换为模块门面，禁改文件行数应只减不增语义不变）；
- container.go:40-64 的 13 行旧路径 import 与 18 个别名目录的最终删除
  （B-knowledge 主任务收口）。
