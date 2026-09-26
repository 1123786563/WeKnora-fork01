// Package ingest 承载知识摄取域（K1）的实现：上传 → 解析 → 分块 → chunk 持久化/内容索引。
//
// 本包由 Pass B 节点 b2-k-ingest 按 docs/plans/passb/21-knowledge-ingest.md 从
// internal/application/{repository,service} 与 internal/handler 的 9 个 legacy
// 摄取域文件搬迁而来（边界目标见 docs/architecture/passb/knowledge-ingest.md）。
// 宿主包仅保留 ib2 删除点的薄 shim；装配切换由集成工程师按 Integration Brief 执行。
package ingest
