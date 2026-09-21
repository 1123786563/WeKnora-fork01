# Pass B Brief — B-knowledge-retrieval（knowledge 检索域文件拆分）

Manifest：`docs/architecture/moves/knowledge.yaml`（模块 knowledge）。本 brief 只覆盖
其 legacy_files 的**检索/KB/标签/语义模型子集**（29 文件）。

## Scope（legacy_files 子集，29 文件）

repository（6）：kbshare.go、semantic_model_invocation.go、semantic_model_policy.go、
semantic_outbox.go、semantic_scope_epoch.go、tag.go

service（19）：graph.go、kb_activity.go、kbshare.go、knowledgebase.go、
knowledgebase_access.go、knowledgebase_search.go、knowledgebase_search_fanout.go、
knowledgebase_search_faq.go、knowledgebase_search_fusion.go、
knowledgebase_search_results.go、knowledgebase_search_shared.go、
knowledgebase_search_storegroup.go、semantic_model.go、semantic_model_capability.go、
semantic_model_policy.go、semantic_scope.go、slug_fuzzy.go、tag.go、tag_access.go

handler（4）：knowledgebase.go、semantic_internal.go、semantic_model_policy.go、tag.go

（前缀：repository=`internal/application/repository/`、service=`internal/application/service/`、
handler=`internal/handler/`。）

## 边界目标

检索面（TypeIndexDelete 索引清理的 tag 侧、KB 搜索 fanout/fusion/FAQ 混排/分组、
知识图谱 graph、语义模型目录与能力/策略、semantic outbox/scope epoch 一致性、
标签体系与访问判定、KB 活动与共享）归入 `internal/modules/knowledge`
（建议 `…/knowledge/retrieval/app` 应用层，与 A9 已搬入的
`…/knowledge/retriever`（引擎仓+service）衔接成完整检索栈）。
注意：`knowledgebase_search_*` 对 `searchutil`、`retriever`（新路径）的消费已在 A9
修好 import；Pass B 搬文件本身，不改语义。**检索/融合/排序行为是外部契约。**

## 删除义务

- 上述 29 文件从横向包删除；
- A9 guard 移交的预存耦合中归本域的两条改走 airesource 模块门面或登记后于本任务删除：
  `internal/modules/knowledge/retriever/service/composite.go` 与
  `keywords_vector_hybrid_indexer.go` → `modules/airesource/models/{embedding,utils}`；
- `semantic_internal` 内部路由（routes_infra.go:14）委托改经模块门面。
