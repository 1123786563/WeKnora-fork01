// Pass B 过渡 shim：删除点 ib2（Integration Brief 登记，plan 21-knowledge-ingest §6.2/§7.5）。
// K1（b2-k-ingest）定义的 chunk 摄取域符号已随 service/chunk.go + chunk_write.go
// 搬迁至 internal/modules/knowledge/ingest；本文件为宿主包仍被引用的调用方保留
// 无逻辑转发声明（spec §13 M3 兼容别名，真源唯一在 ingest 包）。
package service

import (
	"github.com/Tencent/WeKnora/internal/modules/knowledge/ingest"
	"github.com/Tencent/WeKnora/internal/modules/knowledge/retriever"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// ErrChunkRevisionConflict 哨兵转发（plan §6.2 R1-1）：保护 knowledge_process.go:2282
// （K4 属主）与 handler/chunk.go:230/:292（经 service.ErrChunkRevisionConflict）。
// 原 service/chunk.go:23 的 repository 别名行已随 M3 删除，本包唯一定义即此转发，
// 与 ingest.ErrChunkRevisionConflict 同一实例，errors.Is 链身份不变（plan §5.3）。
var ErrChunkRevisionConflict = ingest.ErrChunkRevisionConflict

// sameChunkDocument 转发（R1 增量，超出 §6.2 字面清单）：保护
// knowledge_process.go:2256/:3002（K4 属主，禁改）；真源唯一在 ingest。
func sameChunkDocument(a, b *types.Chunk) bool {
	return ingest.SameChunkDocument(a, b)
}

// NewChunkService 转发（plan §6.2 R1-2）：保护 container.go:375（dig Provide）。
// 签名保持搬迁前原样（末参 spanTracker SpanTracker）——dig 容器在 K1→K4 期间
// 仍按旧装配解析（plan §9「container 仍经宿主 shim 走旧装配」）；seam 化新参
// （writeGuard/spanTrace/enqueueSummaryRefresh/indexContentFn）在函数体内经
// span_trace_seam_adapter.go 的提供器就地适配，零逻辑复制。ib2 由集成工程师
// 将 container.go:374 的 service.NewSpanTracker 经适配器直供 ingest 新签名后
// 删除本转发（Brief 接线表）。
func NewChunkService(
	chunkRepository interfaces.ChunkRepository,
	knowledgeRepo interfaces.KnowledgeRepository,
	kbRepository interfaces.KnowledgeBaseRepository,
	modelService interfaces.ModelService,
	retrieveEngine interfaces.RetrieveEngineRegistry,
	ownership retriever.TenantStoreOwnership,
	task interfaces.TaskEnqueuer,
	spanTracker SpanTracker,
) interfaces.ChunkService {
	return ingest.NewChunkService(
		chunkRepository,
		knowledgeRepo,
		kbRepository,
		modelService,
		retrieveEngine,
		ownership,
		task,
		KnowledgeWriteGuardProvider(),
		NewSpanTraceSeamAdapter(spanTracker),
		EnqueueSummaryRefreshProvider(spanTracker),
		BuildKnowledgeIndexContentProvider(),
	)
}
