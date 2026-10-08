# 假成功家族二轮五票（#3832 / #3833 / #3775 / #3834 / #3835）

日期：2026-10-06 · 分支：fix/fake-success2-3832-3833-3834-3835-3775（基于 merge/upstream-20261003）· 模式：4 并行实现者 + 验证门

## 背景

开放列表 51-100 名单里的同题族（此前误判「bug 已清尽」，本轮纠正）：存储/索引层吞错返回假成功——第八轮（#3880/#3872/#3854）的延续。五票均带票面精确行号定位。fork 路径：检索器在 `internal/knowledge/retriever/`。

- #3832：ES v8 丢弃 _bulk 响应（`res.Errors`/`Items` 全仓零使用），v7 降级为告警仍返回 nil——逐条被拒的文档静默缺失。
- #3833：sqlite 三个索引 helper（insertVec/FTS insert/copyVec）void 吞 Exec 错误——入库成功但检索永远召不回；CopyIndices/copyVec 同病。
- #3775：sqlite BatchUpdateChunkStatus/TagID 丢弃 .Error 永远 return nil（postgres 同名函数会 return）+ knowledge_faq.go 成功审计在 return err 之后。
- #3834：image_multimodal.go indexChunks 无返回值，五处失败只日志；`out["indexed"]=true` 无条件写入并持久化到处理轨迹。
- #3835：qdrant/milvus KeywordsRetrieve 逐 collection 失败只 Warn+continue，整批失败→空结果+nil error，上层判「库里没有」。

## 分工（4 代理，文件域不重叠）

- **ES（#3832）**：retriever/elasticsearch/v8/repository.go:209 形态 + v7/repository.go:391 形态。v8：检查 res.Errors，为真则收集 items[].<op>.error 汇总返回错误（含数量与首例明细）；v7：把 errors:true 告警升级为返回错误。测试：假 bulk 响应（200+errors:true+item error）→ BatchSave 返回错误；全成功 → nil 不回归。
- **SQL（#3833+#3775）**：retriever/sqlite/repository.go。三 helper 改返回 error 并穿透调用方（BatchSave/CopyIndices/copyVec 路径）；BatchUpdateChunkStatus/TagID 对齐 postgres 语义（result.Error 返回；匹配 0 行按 postgres 同口径处理）；service/knowledge_faq.go :841 成功审计移到 return err 之前。测试：注入 Exec 失败（SQLITE_BUSY 形态）→ BatchSave/CopyIndices/批更返回错误；正常路径不回归。
- **QM（#3835）**：retriever/{qdrant,milvus}/repository.go 的 KeywordsRetrieve。逐 collection 失败计数：全部失败且零结果 → 返回错误（区分「没这个词」与「检索没执行成功」）；部分失败有结果 → 结果照常 + Warn 记失败比例。测试：全 collection 失败 → error；部分成功 → 结果+无 error；全成功 → 不回归。
- **IM（#3834）**：application/service/image_multimodal.go。indexChunks 改返回 error（五处失败分支上抛）；processImage 失败时按同文件 :413-416 CreateChunks 的既有严格模式处理（handleErr 返回错误→asynq 重试）；`out["indexed"]` 只在索引成功后写。对齐 knowledge_process.go:729-753 / extract.go:508 的「索引失败必须失败」惯例。测试：engine.BatchIndex 失败 → processImage 返回错误且 out 无 indexed:true；成功 → indexed:true 不回归。

## 验收

1. 四域定向 go test 绿；涉及包基线比对（sqlite 包、service 338/90、elasticsearch 包、qdrant/milvus 包——后三者此前无预存失败记录，出现即须解释）。
2. `go build ./...`；diff 四域互不越界。
3. 语义不变式：任何索引/批量写入失败不得再返回 nil 或标 indexed:true。

## 交付

按域 4 commit（本地不 push）；issue 回帖并入草稿文档（下轮统一补）。
