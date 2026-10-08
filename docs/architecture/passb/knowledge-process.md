# Pass B Brief — B-knowledge-process（knowledge 处理流水线域文件拆分）

> child plan：**24-knowledge-process**（K4；B0.3 裁定标注，framework:71）。
> Manifest：`docs/architecture/moves/knowledge.yaml`（模块 knowledge）。本 brief 只覆盖
> 其 legacy_files 的**处理流水线子集**（28 文件）。

## Scope（legacy_files 子集，28 文件）

repository（5）：knowledge.go、knowledge_span_repo.go、knowledge_tag.go、
knowledge_transfer.go、knowledgebase.go

> **消歧注记（B0.2，b0 notes 审校补充发现）**：上述 repository 侧 `knowledge.go`
> 即全路径 `internal/application/repository/knowledge.go`（knowledge.yaml:87 登记，
> 4 份 K brief 此前均未以全路径枚举）。它**显式归属本 brief / K4
> （24-knowledge-process）**，ownership-matrix 已登记同值。注意其内定义的
> `escapeLikeKeyword`（:22）被 identity 属主（repository/tenant.go:85,90）与
> conversation 属主（repository/message.go:215、session.go:209）文件跨 owner 调用：
> K4 搬出该文件时须按 package_private_couplings 裁定导出窄端口或留薄 shim，
> 跨 owner 调用点修改走 Integration Brief 由集成工程师执行，禁复制实现。

service（19）：knowledge.go、knowledge_auto_tag.go、knowledge_clone_move.go、
knowledge_create.go、knowledge_delete.go、knowledge_delete_plan.go、
knowledge_housekeeping.go、knowledge_index_content.go、knowledge_post_process.go、
knowledge_process.go、knowledge_process_config.go、knowledge_reparse_scope.go、
knowledge_replace.go、knowledge_span_tracker.go、knowledge_summary_refresh.go、
knowledge_task_options.go、knowledge_util.go、knowledge_write.go、knowledge_transfer.go

handler（4）：kb_access.go、knowledge.go、knowledge_download.go、task_progress_auth.go

> **Housekeeping 裁定（B0.3 Step 4，freeze:207-209）**：上述 service 侧
> `knowledge_housekeeping.go` 及其业务清扫规则**独占归 Knowledge / 本 brief
> （24-knowledge-process）**，ownership-matrix 已登记同值；System 侧
> （**42-system-policy**）只拥有调度/生命周期调用——经窄端口
> **`KnowledgeHousekeeping`**（模块门面暴露）触发，调度挂点
> `startHousekeepingService`（container.go:642，func at :2389，system.yaml:50）
> 随 B-container/bootstrap 收敛任务切到该端口。**不得出现第二套清扫实现**：
> System 不复制清扫规则，Knowledge 不自建调度循环；两侧装配变更走
> Integration Brief 由集成工程师执行。

（完整路径前缀：repository=`internal/application/repository/`、
service=`internal/application/service/`、handler=`internal/handler/`。）

## 边界目标

文档处理流水线（TypeDocumentProcess/TypeManualProcess/TypeKnowledgeListReparse →
解析→分块→向量化→索引）、删除/级联（TypeKnowledgeListDelete/TypeIndexDelete/TypeKBDelete
的删除计划与执行）、克隆/迁移（TypeKBClone/TypeKnowledgeMove）、后处理链
（TypeKnowledgePostProcess/TypeKnowledgeAutoTag/TypeSummaryGeneration/
TypeQuestionGeneration/TypeDataTableSummary）归入 `internal/knowledge`
（建议 `…/knowledge/process`）。**删除与索引语义是外部契约**：asynq 任务类型、
幂等/重试语义、Redis/Lite 双池注册行为零变化。

## 删除义务

- 上述 28 文件从横向包删除；
- 18 个 knowledge worker 处理器句柄改由模块暴露（`internal/router/task.go:266-320`、
  `sync_task.go:143-163` 的注册表切换为模块门面，禁改文件行数应只减不增语义不变）；
- container.go:40-64 的 13 行旧路径 import 与 18 个别名目录的最终删除
  （B-knowledge 主任务收口）。
