# Pass B Brief — B-knowledge-wikifaq（knowledge Wiki/FAQ 域文件拆分）

Manifest：`docs/architecture/moves/knowledge.yaml`（模块 knowledge）。本 brief 只覆盖
其 legacy_files 的 **Wiki+FAQ 子集**（18 文件）。

## Scope（legacy_files 子集，18 文件）

repository（1）：wiki_page.go

service（14）：faq_clone_sync.go、knowledge_faq.go、knowledge_faq_batch.go、
knowledge_faq_create_guard.go、knowledge_faq_import.go、wiki_ingest.go、
wiki_ingest_batch.go、wiki_ingest_cite.go、wiki_ingest_dedup.go、
wiki_ingest_taxonomy.go、wiki_linkify.go、wiki_lint.go、wiki_page.go、
wiki_slug_handles.go

handler（3）：faq.go、wiki_page.go、session/wiki_fixer_scope.go

（前缀：repository=`internal/application/repository/`、service=`internal/application/service/`、
handler=`internal/handler/`。）

## 边界目标

Wiki 域（批量摄取 TypeWikiIngest/TypeWikiFinalize、citation、去重、taxonomy、linkify、
lint、slug 体系、页面存储）与 FAQ 域（FAQ 导入 TypeFAQImport、批量、创建守卫、克隆同步）
归入 `internal/modules/knowledge`（建议 `…/knowledge/wiki` 与 `…/knowledge/faq` 两个包）。
`internal/handler/session/wiki_fixer_scope.go` 在横向 session handler 包内（plurality owner
conversation），Pass B 细分时随本域拆出。生命周期挂点 `recoverPendingWikiTasks`
（container/recover_pending_wiki_tasks.go:32，container.go:1052 Invoke）的挂接改为模块
Start/Stop 语义时保持恢复行为不变。

## 删除义务

- 上述 18 文件从横向包删除（含 handler/session 内该文件）；
- `internal/modules/knowledge/docparser` 的 `IsImageFormat`/`IsSimpleFormat`/
  `SimpleFormatReader` 别名面中与 wiki/faq 摄取相关消费切至新路径后回收；
- FAQ 导入对 `internal/application/service` 内非本域符号的引用消除（改模块内依赖）。
