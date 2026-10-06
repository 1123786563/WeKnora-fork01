package ingest

// 本文件承载 K1 摄取域的 R2 消费侧 seam 类型定义（plan 21-knowledge-ingest §6.3，
// spec §4.2「接口优先定义在使用方模块」）。seam 一律构造注入，禁包级 var 注入
// （spec §4.3）；生产接线由集成工程师按 Integration Brief 在 K5/ib2 执行。

import (
	"context"
	"database/sql"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// KBByIDLookup 是宿主 knowledge_write.go 的 knowledgeBaseWriteLookup 与
// knowledge_summary_refresh.go 的 summaryKnowledgeBaseReader 的共同结构
// （单方法 GetKnowledgeBaseByID）；interfaces.KnowledgeBaseRepository 天然满足。
type KBByIDLookup interface {
	GetKnowledgeBaseByID(context.Context, string) (*types.KnowledgeBase, error)
}

// KnowledgeWriteGuard 收敛 K4 属主（internal/application/service 包）的写路径
// 授权族：writeResourceIDs / writeExecutionTenant / loadKnowledgeWrite /
// loadKnowledgeWriteBatch（knowledge_write.go:17/:32/:60/:93）。方法名与宿主
// 包级函数一一对应；chunk_write.go 与 chunk.go 的调用点经本 seam 走宿主实现，
// 零逻辑复制。
type KnowledgeWriteGuard interface {
	// WriteResourceIDs 对应宿主 writeResourceIDs（去空/去重）。
	WriteResourceIDs(ids []string) ([]string, error)
	// WriteExecutionTenant 对应宿主 writeExecutionTenant（写侧租户解析）。
	WriteExecutionTenant(ctx context.Context) (uint64, error)
	// LoadKnowledgeWrite 对应宿主 loadKnowledgeWrite（单文档写授权装载）。
	LoadKnowledgeWrite(ctx context.Context, repo interfaces.KnowledgeRepository, lookup KBByIDLookup, id string) (*types.Knowledge, *types.KnowledgeBase, error)
	// LoadKnowledgeWriteBatch 对应宿主 loadKnowledgeWriteBatch（批量写授权装载）。
	LoadKnowledgeWriteBatch(ctx context.Context, repo interfaces.KnowledgeRepository, lookup KBByIDLookup, ids []string) ([]*types.Knowledge, error)
}

// SpanTraceSeam 是 K4 属主 SpanTracker（knowledge_span_tracker.go:85，11 方法）
// 的最小投影：ingest 侧消费 LookupStage/BeginSubSpan/EndSpan/FailSpan 四方法。
// 句柄以 any 传递（宿主 *Span 的不透明引用）——ingest 若 import 宿主 service 包
// 将与 R1 shim 形成 service→ingest→service import 环，实测裁决不可行（plan §6.3）。
type SpanTraceSeam interface {
	LookupStage(ctx context.Context, knowledgeID string, attempt int, stage string) any
	BeginSubSpan(ctx context.Context, parent any, name, kind string, input types.JSONMap) any
	EndSpan(ctx context.Context, span any, output types.JSONMap)
	FailSpan(ctx context.Context, span any, code, message string, err error)
}

// noopSpanTraceSeam 对照宿主 noopSpanTracker（knowledge_span_tracker.go:858-879）
// 的零值语义：LookupStage/BeginSubSpan 返回 nil，EndSpan/FailSpan 空操作。
// nil seam 的回退行为与原 tracker() 的 noopSpanTracker{} 一致（plan §6.3）。
type noopSpanTraceSeam struct{}

func (noopSpanTraceSeam) LookupStage(context.Context, string, int, string) any { return nil }

func (noopSpanTraceSeam) BeginSubSpan(context.Context, any, string, string, types.JSONMap) any {
	return nil
}

func (noopSpanTraceSeam) EndSpan(context.Context, any, types.JSONMap) {}

func (noopSpanTraceSeam) FailSpan(context.Context, any, string, string, error) {}

// DataTableSummaryEnqueuer 承载 K4 属主（knowledge_util.go:47/:67/:89）三 helper
// 的构造注入（plan §6.3）：normalizeFileExtension / isDataTableFileType /
// getFileType。宿主经 R1-9 shim 以本包函数值构造，K4 搬迁 knowledge_util.go
// 导出后由其直接传导出形式。
type DataTableSummaryEnqueuer struct {
	normalizeFileExtension func(string) string
	isDataTableFileType    func(string) bool
	getFileType            func(string) string
}

// NewDataTableSummaryEnqueuer 构造 enqueuer seam（plan §6.1）。
func NewDataTableSummaryEnqueuer(
	normalizeFileExtension func(string) string,
	isDataTableFileType func(string) bool,
	getFileType func(string) string,
) *DataTableSummaryEnqueuer {
	return &DataTableSummaryEnqueuer{
		normalizeFileExtension: normalizeFileExtension,
		isDataTableFileType:    isDataTableFileType,
		getFileType:            getFileType,
	}
}

// EnqueueIfNeeded 承载原宿主 enqueueDataTableSummaryIfNeeded 函数体（三 helper
// 改经字段，函数体其余零改动）。fileName is a fallback for older records whose
// FileType is empty.
func (e *DataTableSummaryEnqueuer) EnqueueIfNeeded(
	ctx context.Context,
	client interfaces.TaskEnqueuer,
	tenantID uint64,
	knowledgeID string,
	fileName, fileType, summaryModelID, embeddingModelID string,
) {
	ft := e.normalizeFileExtension(fileType)
	if ft == "" && fileName != "" {
		ft = e.getFileType(fileName)
	}
	if !e.isDataTableFileType(ft) {
		return
	}
	if err := NewDataTableSummaryTask(ctx, client, tenantID, knowledgeID, summaryModelID, embeddingModelID); err != nil {
		logger.Warnf(ctx, "Failed to enqueue data table summary task for knowledge %s: %v", knowledgeID, err)
	}
}

// EnqueueDataTableSummaryIfNeeded 导出包装（plan §6.1 签名）：转发到
// enqueuer seam 的 EnqueueIfNeeded。
func EnqueueDataTableSummaryIfNeeded(
	e *DataTableSummaryEnqueuer,
	ctx context.Context,
	client interfaces.TaskEnqueuer,
	tenantID uint64,
	knowledgeID string,
	fileName, fileType, summaryModelID, embeddingModelID string,
) {
	e.EnqueueIfNeeded(ctx, client, tenantID, knowledgeID, fileName, fileType, summaryModelID, embeddingModelID)
}

// TableSchemaSummary 是 agentruntime/agent/tools.TableSchema 在 ingest 侧的
// 最小投影（K1.3 增量 seam）：processTableData 仅消费表名、列数、行数与
// schema 描述四项，Description() 由适配侧调用后内联为字符串。
type TableSchemaSummary struct {
	TableName   string
	ColumnCount int
	RowCount    int64
	Description string
}

// DataAnalysisToolSeam 投象 agentruntime/agent/tools 的 DuckDB 分析工具
// （data_analysis.go:143 NewDataAnalysisTool）。K1.3 增量 seam：ingest 直连
// tools 将与宿主 R1-8 shim 构成 repository→ingest→tools→repository import 环
// （tools/wiki_route_resolver.go:10 消费 repository.ErrWikiPageNotFound，且
// service/knowledge.go:38 钉住 repository→ingest，装配切换无法单独破环），
// 故按 spec §4.2「接口优先定义在使用方模块」消费侧 seam 化，工厂由宿主
// 适配器（span_trace_seam_adapter.go）注入，零逻辑复制。
type DataAnalysisToolSeam interface {
	// LoadFromKnowledge 对应 tools.(*DataAnalysisTool).LoadFromKnowledge。
	LoadFromKnowledge(ctx context.Context, knowledge *types.Knowledge) (*TableSchemaSummary, error)
	// Execute 对应 tools.(*DataAnalysisTool).Execute，输入以
	// (knowledgeID, sql) 承载 DataAnalysisInput 的 JSON 序列化（适配侧完成）。
	Execute(ctx context.Context, knowledgeID, sql string) (*types.ToolResult, error)
	// Cleanup 对应 tools.(*DataAnalysisTool).Cleanup。
	Cleanup(ctx context.Context)
}

// DataAnalysisToolFactory 是 DuckDB 分析工具的构造注入工厂（K1.3 增量 seam）。
// 参数与 tools.NewDataAnalysisTool 前 6 参一一对应，末位 resolver 对应其
// variadic storageResolvers 的单值调用形态（原 extract.go:630 调用形态）。
type DataAnalysisToolFactory func(
	knowledgeBaseService interfaces.KnowledgeBaseService,
	knowledgeService interfaces.KnowledgeService,
	tenantService interfaces.TenantService,
	fileService interfaces.FileService,
	db *sql.DB,
	sessionID string,
	storageResolver interfaces.StorageBackendResolver,
) DataAnalysisToolSeam

// GraphExtractorSeam 投象 conversation/chat_pipeline.Extractor 的实体抽取
// （K1.3 增量 seam：chat_pipeline 自身 data_analysis.go 经
// agentruntime/agent/tools 传递依赖 application/repository，与宿主 R1-8 shim
// 构成 repository→ingest→chat_pipeline→tools→repository import 环，
// 消费侧 seam 化破环，工厂由宿主适配器注入，零逻辑复制）。
type GraphExtractorSeam interface {
	// Extract 对应 chatpipeline.(*Extractor).Extract。
	Extract(ctx context.Context, content string) (*types.GraphData, error)
}

// GraphExtractorFactory 对应 chatpipeline.NewExtractor（chat.Chat 参数以
// airesource models/chat 接口承载——ingest 对该模块为合法直连，E1 例外）。
type GraphExtractorFactory func(chatModel chat.Chat, template *types.PromptTemplateStructured) GraphExtractorSeam
