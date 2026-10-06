// Pass B 过渡 shim：删除点 ib2（Integration Brief 登记，plan 21-knowledge-ingest §6.2/§7.5）。
// K1（b2-k-ingest）定义的 chunk 摄取域符号已随 service/chunk.go + chunk_write.go +
// service/extract.go + service/image_multimodal.go + service/ocr_sanitizer.go 搬迁至
// internal/modules/knowledge/ingest；本文件为宿主包仍被引用的调用方保留无逻辑转发
// 声明（spec §13 M3 兼容别名，真源唯一在 ingest 包）。
package service

import (
	"context"
	"database/sql"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service/retriever"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/models/utils/ollama"
	"github.com/Tencent/WeKnora/internal/modules/knowledge/ingest"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/redis/go-redis/v9"
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

// NewChunkExtractService 转发（plan §6.2 R1-3）：保护 container.go:409（dig
// Provide，dig.Name("chunkExtractor")）。签名保持搬迁前原样（末参
// spanTracker SpanTracker）——dig 容器在 K1→K4 期间仍按旧装配解析；seam 新参
// （spanTrace/attemptSupersededFn）经适配器就地提供，previewTextFn/
// finalizeSubtaskFn/isFinalAttemptFn/resolveProcessConfigFn 以本包函数值直供
// （K1.3 增量 seam，零复制）。ib2 由集成工程师切换 container 直供后删除。
func NewChunkExtractService(
	config *config.Config,
	modelService interfaces.ModelService,
	knowledgeBaseRepo interfaces.KnowledgeBaseRepository,
	knowledgeRepo interfaces.KnowledgeRepository,
	chunkRepo interfaces.ChunkRepository,
	graphEngine interfaces.RetrieveGraphRepository,
	spanTracker SpanTracker,
) interfaces.TaskHandler {
	return ingest.NewChunkExtractService(
		config,
		modelService,
		knowledgeBaseRepo,
		knowledgeRepo,
		chunkRepo,
		graphEngine,
		NewSpanTraceSeamAdapter(spanTracker),
		AttemptSupersededProvider(spanTracker),
		previewText,
		finalizeSubtaskDetached,
		isFinalAsynqAttempt,
		ResolveProcessConfig,
		GraphExtractorSeamFactory(),
	)
}

// NewDataTableSummaryService 转发（plan §6.2 R1-4）：保护 container.go:410
// （dig Provide，dig.Name("dataTableSummary")）。原 10 参签名不变；增量 seam
// （knowledgeWriteKBFn/resolveProcessConfigFn）在本函数体内以本包函数值/提供器
// 供给，零复制。ib2 由集成工程师切换 container 直供后删除。
func NewDataTableSummaryService(
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
	return ingest.NewDataTableSummaryService(
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

// NewChunkExtractTask 转发（plan §6.2 R1-6）：保护 knowledge_post_process.go:418
// （K4 属主，禁改）；签名与搬迁前原样一致。
func NewChunkExtractTask(
	ctx context.Context,
	client interfaces.TaskEnqueuer,
	tenantID uint64,
	chunkID string,
	modelID string,
	knowledgeID string,
	attempt int,
	chunkIndex int,
) (bool, error) {
	return ingest.NewChunkExtractTask(ctx, client, tenantID, chunkID, modelID, knowledgeID, attempt, chunkIndex)
}

// enqueueDataTableSummaryIfNeeded 转发（plan §6.2 R1-9）：保护
// knowledge_create.go:295/:746、knowledge_process.go:2629/:2682（K4 属主，禁改）。
// knowledge_util.go 三 helper（K4 属主）以本包函数值构造 enqueuer seam 注入，
// 零复制；K4 搬迁 knowledge_util.go 导出后由其直接传导出形式。
func enqueueDataTableSummaryIfNeeded(
	ctx context.Context,
	client interfaces.TaskEnqueuer,
	tenantID uint64,
	knowledgeID string,
	fileName, fileType, summaryModelID, embeddingModelID string,
) {
	ingest.EnqueueDataTableSummaryIfNeeded(
		ingest.NewDataTableSummaryEnqueuer(normalizeFileExtension, isDataTableFileType, getFileType),
		ctx, client, tenantID, knowledgeID, fileName, fileType, summaryModelID, embeddingModelID)
}

// NewImageMultimodalService 转发（plan §6.2 R1-5）：保护 container.go:411
// （dig Provide，dig.Name("imageMultimodal")）。签名保持搬迁前原样（末参
// spanTracker SpanTracker）——dig 容器在 K1→K4 期间仍按旧装配解析；seam 化
// 新参（spanTrace/两哨兵 error/previewTextFn/resolveProcessConfigFn/
// postProcessTaskOptionsFn）在本函数体内以适配器与本包/repository 函数值就地
// 供给，零逻辑复制。哨兵注入发生在宿主边界（ingest 禁 import 宿主
// repository，plan §6.3）；ib2 由集成工程师切换 container 直供后删除。
func NewImageMultimodalService(
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
	return ingest.NewImageMultimodalService(
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
		previewText,
		ResolveProcessConfig,
		knowledgePostProcessTaskOptions,
	)
}

// isFinalAsynqAttempt 转发（plan §6.2 R1-7）：保护 knowledge_process.go
// :1125/:1497/:1868（K4 属主，禁改）与本文件内 NewChunkExtractService 的
// 函数值直供；真源唯一在 ingest（isFinalAsynqAttempt 随 image_multimodal.go
// 搬迁，K1.4）。
func isFinalAsynqAttempt(ctx context.Context) bool {
	return ingest.IsFinalAsynqAttempt(ctx)
}

// sanitizeOCRText 转发（plan §6.2 R1-10，K0 §6.2 组 D）：保护 conversation
// 属主 temporary_document.go:541（禁改，ib2 改写项走 Integration Brief）。
func sanitizeOCRText(raw string) string {
	return ingest.SanitizeOCRText(raw)
}

// buildVLMCaptionPrompt 转发（plan §6.2 R1-10，K0 §6.2 组 D）：保护
// conversation 属主 temporary_document.go:560（禁改，ib2 改写项走
// Integration Brief）。
func buildVLMCaptionPrompt(ctx context.Context, cfg types.VLMConfig) string {
	return ingest.BuildVLMCaptionPrompt(ctx, cfg)
}

// vlmOCRPrompt / vlmOCRScannedPDFPrompt 转发常量（K1.4 R1 增量，超出 §6.2
// 字面清单）：保护 conversation 属主 temporary_document.go:513/:515（禁改，
// ib2 改写项走 Integration Brief）；真源唯一在 ingest。
const (
	vlmOCRPrompt           = ingest.VlmOCRPrompt
	vlmOCRScannedPDFPrompt = ingest.VlmOCRScannedPDFPrompt
)

// validateParserEngineOverrideURLs 转发（K1.5 R1 增量，超出 §6.2 字面清单）：
// 保护 knowledge_process.go:3750（K4 属主，禁改）；真源唯一在 ingest
// （parser_url_security.go 随 K1.5 搬迁，导出包装 ValidateParserEngineOverrideURLs）。
func validateParserEngineOverrideURLs(overrides map[string]string) error {
	return ingest.ValidateParserEngineOverrideURLs(overrides)
}
