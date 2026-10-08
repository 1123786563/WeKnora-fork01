// Pass B 过渡 shim（ib2 后收口）：K1（b2-k-ingest）的 chunk 摄取域 service 侧
// 实现已随上游对齐 round 2 归位至本包（chunk.go + chunk_write.go + extract.go +
// image_multimodal.go + ocr_sanitizer.go + parser_url_security.go +
// ingest_seams.go）；chunk 持久化层（repository 侧 chunk.go/chunk_image_assets.go）
// 仍驻 internal/knowledge/ingest。本文件保留：
//   - 两枚 repository 哨兵的包级别名（errors.Is 链身份不变，真源在 ingest）；
//   - 未导出调用面（knowledge_process.go 等）的同名薄包装；
//   - dig 装配面（container.go）的 -DI 适配构造器：以旧装配签名接缝化新签名
//     （seam 参数经本包提供器就地供给，零逻辑复制）。
package service

import (
	"context"
	"database/sql"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service/retriever"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/models/utils/ollama"
	"github.com/Tencent/WeKnora/internal/knowledge/ingest"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/redis/go-redis/v9"
)

// ErrChunkRevisionConflict 哨兵别名：真源唯一在 ingest（repository 侧
// chunk_repo.go 随迁残留），errors.Is 链身份不变（plan §5.3）。
// ErrChunkNotFound 的包级别名在 knowledge.go（= repository.ErrChunkNotFound，
// 同一实例），不在此重复。
var ErrChunkRevisionConflict = ingest.ErrChunkRevisionConflict

// sameChunkDocument 薄包装（R1 增量）：保护 knowledge_process.go:2256/:3002
// 的未导出调用面；真源已随 chunk_write.go 归位本包（SameChunkDocument）。
func sameChunkDocument(a, b *types.Chunk) bool {
	return SameChunkDocument(a, b)
}

// enqueueDataTableSummaryIfNeeded 薄包装：保护 knowledge_create.go:295/:746、
// knowledge_process.go:2629/:2682 的未导出调用面；三 helper（K4 属主
// knowledge_util.go）以本包函数值构造 enqueuer seam，零复制。
func enqueueDataTableSummaryIfNeeded(
	ctx context.Context,
	client interfaces.TaskEnqueuer,
	tenantID uint64,
	knowledgeID string,
	fileName, fileType, summaryModelID, embeddingModelID string,
) {
	EnqueueDataTableSummaryIfNeeded(
		NewDataTableSummaryEnqueuer(normalizeFileExtension, isDataTableFileType, getFileType),
		ctx, client, tenantID, knowledgeID, fileName, fileType, summaryModelID, embeddingModelID)
}

// NewChunkServiceDI 是 dig 装配面（container.go）的适配构造器：保持搬迁前的
// 8 参旧签名（末参 spanTracker SpanTracker），seam 化新参（writeGuard/
// spanTrace/enqueueSummaryRefresh/indexContentFn）经本包提供器就地适配，
// 零逻辑复制（原 ib2 接线表 R1-2）。
func NewChunkServiceDI(
	chunkRepository interfaces.ChunkRepository,
	knowledgeRepo interfaces.KnowledgeRepository,
	kbRepository interfaces.KnowledgeBaseRepository,
	modelService interfaces.ModelService,
	retrieveEngine interfaces.RetrieveEngineRegistry,
	ownership retriever.TenantStoreOwnership,
	task interfaces.TaskEnqueuer,
	spanTracker SpanTracker,
) interfaces.ChunkService {
	return NewChunkService(
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

// NewChunkExtractServiceDI 是 dig 装配面（dig.Name("chunkExtractor")）的适配
// 构造器：7 参旧签名不变；seam 新参（spanTrace/attemptSupersededFn）经适配器
// 就地提供，previewTextFn/finalizeSubtaskFn/isFinalAttemptFn/
// resolveProcessConfigFn/newGraphExtractor 以本包函数值直供（原 R1-3）。
func NewChunkExtractServiceDI(
	config *config.Config,
	modelService interfaces.ModelService,
	knowledgeBaseRepo interfaces.KnowledgeBaseRepository,
	knowledgeRepo interfaces.KnowledgeRepository,
	chunkRepo interfaces.ChunkRepository,
	graphEngine interfaces.RetrieveGraphRepository,
	spanTracker SpanTracker,
) interfaces.TaskHandler {
	return NewChunkExtractService(
		config,
		modelService,
		knowledgeBaseRepo,
		knowledgeRepo,
		chunkRepo,
		graphEngine,
		NewSpanTraceSeamAdapter(spanTracker),
		AttemptSupersededProvider(spanTracker),
		PreviewText,
		finalizeSubtaskDetached,
		isFinalAsynqAttempt,
		ResolveProcessConfig,
		GraphExtractorSeamFactory(),
	)
}

// NewDataTableSummaryServiceDI 是 dig 装配面（dig.Name("dataTableSummary")）
// 的适配构造器：原 10 参签名不变；增量 seam（knowledgeWriteKBFn/
// resolveProcessConfigFn/newDataAnalysisTool）以本包函数值/提供器供给（原 R1-4）。
func NewDataTableSummaryServiceDI(
	modelService interfaces.ModelService,
	knowledgeBaseService interfaces.KnowledgeBaseService,
	knowledgeService interfaces.KnowledgeService,
	fileService interfaces.FileService,
	chunkService interfaces.ChunkService,
	tenantService interfaces.TenantService,
	retrieveEngine interfaces.RetrieveEngineRegistry,
	ownership retriever.TenantStoreOwnership,
	sqlDB *sql.DB,
	storageResolver interfaces.StorageBackendResolver,
) interfaces.TaskHandler {
	return NewDataTableSummaryService(
		modelService,
		knowledgeBaseService,
		knowledgeService,
		fileService,
		chunkService,
		tenantService,
		retrieveEngine,
		ownership,
		sqlDB,
		storageResolver,
		KnowledgeWriteKBProvider(),
		ResolveProcessConfig,
		DataAnalysisToolSeamFactory(),
	)
}

// NewImageMultimodalServiceDI 是 dig 装配面（dig.Name("imageMultimodal")）的
// 适配构造器：14 参旧签名不变；seam 化新参（spanTrace/两哨兵 error/
// previewTextFn/resolveProcessConfigFn/postProcessTaskOptionsFn）在本函数体内
// 以适配器与本包/repository 函数值就地供给（原 R1-5）。
func NewImageMultimodalServiceDI(
	chunkService interfaces.ChunkService,
	modelService interfaces.ModelService,
	kbService interfaces.KnowledgeBaseService,
	knowledgeRepo interfaces.KnowledgeRepository,
	tenantRepo interfaces.TenantRepository,
	retrieveEngine interfaces.RetrieveEngineRegistry,
	ownership retriever.TenantStoreOwnership,
	ollamaService *ollama.OllamaService,
	taskEnqueuer interfaces.TaskEnqueuer,
	redisClient *redis.Client,
	fileSvc interfaces.FileService,
	storageResolver interfaces.StorageBackendResolver,
	resourceCatalog interfaces.ResourceCatalog,
	spanTracker SpanTracker,
) interfaces.TaskHandler {
	return NewImageMultimodalService(
		chunkService,
		modelService,
		kbService,
		knowledgeRepo,
		tenantRepo,
		retrieveEngine,
		ownership,
		ollamaService,
		taskEnqueuer,
		redisClient,
		fileSvc,
		storageResolver,
		resourceCatalog,
		NewSpanTraceSeamAdapter(spanTracker),
		repository.ErrKnowledgeNotFound,
		repository.ErrKnowledgeBaseNotFound,
		PreviewText,
		ResolveProcessConfig,
		knowledgePostProcessTaskOptions,
	)
}
