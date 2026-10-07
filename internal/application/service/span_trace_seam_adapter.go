// Pass B 过渡 seam 适配器：删除点 ib2（Integration Brief 登记，plan 21-knowledge-ingest §6.3 接线义务）。
// 宿主 K4 属主实现（SpanTracker / enqueueSummaryRefresh / write-family /
// buildKnowledgeIndexContent）→ ingest R2 seam 的纯转发提供器，零逻辑复制。
// K1→K4 期间由本包 chunk_ingest_shim.go 内部消费；K4 搬迁 knowledge_write.go /
// knowledge_span_tracker.go / knowledge_summary_refresh.go / knowledge_index_content.go
// 后指到其导出包装，ib2 由集成工程师切换 container 直供并删除本文件。
package service

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/Tencent/WeKnora/internal/agent/tools"
	chatpipeline "github.com/Tencent/WeKnora/internal/application/service/chat_pipeline"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// spanTraceSeamAdapter 把宿主 SpanTracker（knowledge_span_tracker.go:85）投影为
// SpanTraceSeam；*Span 句柄经 any 不透明传递（ingest 不得 import 宿主，
// service→ingest→service import 环实测裁决，plan §6.3）。
type spanTraceSeamAdapter struct {
	tr SpanTracker
}

// NewSpanTraceSeamAdapter 返回四方法纯转发适配器。
func NewSpanTraceSeamAdapter(tr SpanTracker) SpanTraceSeam {
	return spanTraceSeamAdapter{tr: tr}
}

// spanPtr 从 seam 的 any 句柄还原 *Span（nil 安全）。
func spanPtr(span any) *Span {
	if span == nil {
		return nil
	}
	return span.(*Span)
}

func (a spanTraceSeamAdapter) LookupStage(ctx context.Context, knowledgeID string, attempt int, stage string) any {
	if a.tr == nil {
		return nil
	}
	// nil *Span 归一化为 untyped nil：seam 消费方（ingest/extract.go）以
	// `if parent != nil` 判空，装箱后的 nil 指针会破坏原语义（K1.3 修正）。
	if sp := a.tr.LookupStage(ctx, knowledgeID, attempt, stage); sp != nil {
		return sp
	}
	return nil
}

func (a spanTraceSeamAdapter) BeginSubSpan(ctx context.Context, parent any, name, kind string, input types.JSONMap) any {
	if a.tr == nil {
		return nil
	}
	if sp := a.tr.BeginSubSpan(ctx, spanPtr(parent), name, kind, input); sp != nil {
		return sp
	}
	return nil
}

func (a spanTraceSeamAdapter) EndSpan(ctx context.Context, span any, output types.JSONMap) {
	if a.tr == nil {
		return
	}
	a.tr.EndSpan(ctx, spanPtr(span), output)
}

func (a spanTraceSeamAdapter) FailSpan(ctx context.Context, span any, code, message string, err error) {
	if a.tr == nil {
		return
	}
	a.tr.FailSpan(ctx, spanPtr(span), code, message, err)
}

// EnqueueSummaryRefreshProvider 返回捕获 tracker 的 summary 刷新入队闭包；
// kbReader 参数类型 KBByIDLookup 与宿主 summaryKnowledgeBaseReader
// 结构等价（单方法 GetKnowledgeBaseByID），Go 结构化接口直接传递。
func EnqueueSummaryRefreshProvider(tr SpanTracker) func(context.Context, interfaces.KnowledgeRepository, interfaces.TaskEnqueuer, KBByIDLookup, *types.Knowledge) error {
	return func(
		ctx context.Context,
		repo interfaces.KnowledgeRepository,
		taskEnqueuer interfaces.TaskEnqueuer,
		kbReader KBByIDLookup,
		knowledge *types.Knowledge,
	) error {
		return enqueueSummaryRefresh(ctx, repo, taskEnqueuer, kbReader, tr, knowledge)
	}
}

// hostKnowledgeWriteGuard 以宿主 knowledge_write.go 包级函数（K4 属主）实现
// KnowledgeWriteGuard，方法名一一对应，纯转发零复制。
type hostKnowledgeWriteGuard struct{}

// KnowledgeWriteGuardProvider 返回宿主写授权 seam 实现；K4 搬迁
// knowledge_write.go 后指到其导出包装（plan §6.3 接线义务）。
func KnowledgeWriteGuardProvider() KnowledgeWriteGuard {
	return hostKnowledgeWriteGuard{}
}

func (hostKnowledgeWriteGuard) WriteResourceIDs(ids []string) ([]string, error) {
	return writeResourceIDs(ids)
}

func (hostKnowledgeWriteGuard) WriteExecutionTenant(ctx context.Context) (uint64, error) {
	return writeExecutionTenant(ctx)
}

func (hostKnowledgeWriteGuard) LoadKnowledgeWrite(
	ctx context.Context, repo interfaces.KnowledgeRepository, lookup KBByIDLookup, id string,
) (*types.Knowledge, *types.KnowledgeBase, error) {
	return loadKnowledgeWrite(ctx, repo, lookup, id)
}

func (hostKnowledgeWriteGuard) LoadKnowledgeWriteBatch(
	ctx context.Context, repo interfaces.KnowledgeRepository, lookup KBByIDLookup, ids []string,
) ([]*types.Knowledge, error) {
	return loadKnowledgeWriteBatch(ctx, repo, lookup, ids)
}

// BuildKnowledgeIndexContentProvider 返回宿主 buildKnowledgeIndexContent
// （knowledge_index_content.go:12，K4 属主）的函数值；K1.2 增量 seam
// （plan §6.3 未枚举，按既有 R2 机制具体化，节点报告登记）。
func BuildKnowledgeIndexContentProvider() func(knowledge *types.Knowledge, content string) string {
	return buildKnowledgeIndexContent
}

// AttemptSupersededProvider 返回捕获 tracker 的 attempt 超前判定闭包（K1.3，
// plan §6.3 接线义务：NewChunkExtractService 消费）；nil tracker 按
// noopSpanTracker 语义（LatestAttempt→0）恒 false，与原 tracker() 回退一致。
func AttemptSupersededProvider(tr SpanTracker) func(context.Context, string, int) bool {
	return func(ctx context.Context, knowledgeID string, attempt int) bool {
		if tr == nil {
			return false
		}
		return attemptSuperseded(ctx, tr, knowledgeID, attempt)
	}
}

// KnowledgeWriteKBProvider 返回宿主 knowledgeWriteKB（knowledge_write.go:42，
// K4 属主）的适配闭包（K1.3 增量 seam）：宿主参数类型 knowledgeBaseWriteLookup
// 与 KBByIDLookup 结构等价（单方法 GetKnowledgeBaseByID），接口到接口
// 直接赋值，零逻辑复制。
func KnowledgeWriteKBProvider() func(context.Context, KBByIDLookup, *types.Knowledge) (*types.KnowledgeBase, error) {
	return func(ctx context.Context, lookup KBByIDLookup, knowledge *types.Knowledge) (*types.KnowledgeBase, error) {
		return knowledgeWriteKB(ctx, lookup, knowledge)
	}
}

// dataAnalysisToolSeam 把 agentruntime/agent/tools 的 DuckDB 分析工具投影为
// DataAnalysisToolSeam（K1.3 增量 seam）：ingest 直连 tools 与宿主 R1-8
// shim 构成 repository→ingest→tools→repository import 环（tools 侧
// wiki_route_resolver.go 消费 repository.ErrWikiPageNotFound，且
// service/knowledge.go:38 钉住 repository→ingest），消费侧 seam 化破环。
// TableSchema 投影与 DataAnalysisInput 序列化在本适配器完成，零逻辑复制。
type dataAnalysisToolSeam struct {
	tool *tools.DataAnalysisTool
}

// DataAnalysisToolSeamFactory 返回 DataAnalysisToolFactory 的宿主实现
// （K1.3 增量 seam）。
func DataAnalysisToolSeamFactory() DataAnalysisToolFactory {
	return func(
		knowledgeBaseService interfaces.KnowledgeBaseService,
		knowledgeService interfaces.KnowledgeService,
		tenantService interfaces.TenantService,
		fileService interfaces.FileService,
		db *sql.DB,
		sessionID string,
		storageResolver interfaces.StorageBackendResolver,
	) DataAnalysisToolSeam {
		return dataAnalysisToolSeam{
			tool: tools.NewDataAnalysisTool(knowledgeBaseService, knowledgeService, tenantService, fileService, db, sessionID, storageResolver),
		}
	}
}

func (s dataAnalysisToolSeam) LoadFromKnowledge(ctx context.Context, knowledge *types.Knowledge) (*TableSchemaSummary, error) {
	schema, err := s.tool.LoadFromKnowledge(ctx, knowledge)
	if err != nil {
		return nil, err
	}
	return &TableSchemaSummary{
		TableName:   schema.TableName,
		ColumnCount: len(schema.Columns),
		RowCount:    schema.RowCount,
		Description: schema.Description(),
	}, nil
}

func (s dataAnalysisToolSeam) Execute(ctx context.Context, knowledgeID, sql string) (*types.ToolResult, error) {
	input := tools.DataAnalysisInput{KnowledgeID: knowledgeID, SQL: sql}
	jsonData, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	return s.tool.Execute(ctx, jsonData)
}

func (s dataAnalysisToolSeam) Cleanup(ctx context.Context) {
	s.tool.Cleanup(ctx)
}

// graphExtractorSeam 把 conversation/chat_pipeline.Extractor 投影为
// GraphExtractorSeam（K1.3 增量 seam：chat_pipeline 自身 data_analysis.go
// 经 agentruntime/agent/tools 传递依赖 repository，与宿主 R1-8 shim 构成
// import 环，消费侧 seam 化破环），纯转发零逻辑复制。
type graphExtractorSeam struct {
	extractor chatpipeline.Extractor
}

// GraphExtractorSeamFactory 返回 GraphExtractorFactory 的宿主实现
// （K1.3 增量 seam）。
func GraphExtractorSeamFactory() GraphExtractorFactory {
	return func(chatModel chat.Chat, template *types.PromptTemplateStructured) GraphExtractorSeam {
		return graphExtractorSeam{extractor: chatpipeline.NewExtractor(chatModel, template)}
	}
}

func (g graphExtractorSeam) Extract(ctx context.Context, content string) (*types.GraphData, error) {
	return g.extractor.Extract(ctx, content)
}
