# Pass B Brief — B-knowledge-ingest（knowledge 摄取域文件拆分）

Manifest：`docs/architecture/moves/knowledge.yaml`（模块 knowledge）。本 brief 只覆盖
其 legacy_files 的**摄取子集**（9 文件）。已搬入模块的引擎包
（`internal/modules/knowledge/chunker`、`docparser(+anydoc)`）不再动。

## Scope（legacy_files 子集，9 文件）

- internal/application/repository/chunk.go
- internal/application/service/chunk.go
- internal/application/service/chunk_write.go
- internal/application/service/extract.go
- internal/application/service/image_multimodal.go
- internal/application/service/ocr_sanitizer.go
- internal/application/service/parser_url_security.go
- internal/handler/chunk.go
- internal/handler/chunker_debug.go

## 边界目标

把"上传 → 解析（docparser）→ 分块（chunker）→ chunk 持久化/内容索引"的应用层入口
从横向包 `internal/application/{repository,service}` 与 `internal/handler` 拆出，
归入 `internal/modules/knowledge`（建议落位 `…/knowledge/ingest`，与 A9 已就位的
chunker/docparser/searchutil 同模块互联），handler 薄层改为模块路由注册。

## 删除义务

- 上述 9 文件从横向包移除后，`internal/application/repository`、`internal/application/service`、
  `internal/handler` 中对应文件删除；
- 摄取路径对 `internal/modules/airesource/storageurl`、`policy/access` 等的消费经模块
  公共门面（消除可预见的跨模块内部包 import）；
- 完成后回收 §A9-integration 第 2 节 18 个别名中与本域相关的 alias（docparser 一族，
  若届时仍被禁改文件以外代码引用）。
