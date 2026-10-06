package ingest

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/access"
	"github.com/Tencent/WeKnora/internal/application/service/retriever"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/models/embedding"
	"github.com/Tencent/WeKnora/internal/tracing/langfuse"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const (
	// tableDescriptionPromptTemplate is the prompt template for generating table descriptions
	tableDescriptionPromptTemplate = `You are a data analysis expert. Based on the following table structure information and data samples, generate a concise table metadata description (200-300 words).

Table name: %s

%s

%s

Please describe the table from the following dimensions:
1. **Data Subject**: What type of data does this table record? (e.g., user information, sales records, log data, etc.)
2. **Core Fields**: List 3-5 most important fields and their meanings
3. **Data Scale**: Total number of rows and columns
4. **Business Scenarios**: What business analysis or application scenarios might this table be used for?
5. **Key Characteristics**: What notable features does the data have? (e.g., contains geographic locations, has category labels, has hierarchical relationships, etc.)

**Important Notes**:
- Do not output specific data values or sample content
- Use general descriptions so users can quickly determine if this table contains the information they need
- Use concise and professional language for easy retrieval and understanding
- Write the description in the same language as the data content`

	// columnDescriptionsPromptTemplate is the prompt template for generating column descriptions
	columnDescriptionsPromptTemplate = `You are a data analysis expert. Based on the following table structure information and data samples, generate structured description information for each column.

Table name: %s

%s

%s

Please generate a detailed description for each column, including the following information:
1. **Field Meaning**: What information does this column store? (e.g., user ID, order amount, creation time, etc.)
2. **Data Type**: The type and format of the data (e.g., integer, string, datetime, boolean, etc.)
3. **Business Purpose**: The role of this field in business (e.g., for user identification, amount calculation, time sorting, etc.)
4. **Data Characteristics**: Notable features of the data (e.g., unique identifier, nullable, has enum values, has units, etc.)

Please output in the following format (one paragraph per column):

**Column1** (data type)
- Field Meaning: xxx
- Business Purpose: xxx
- Data Characteristics: xxx

**Column2** (data type)
- Field Meaning: xxx
- Business Purpose: xxx
- Data Characteristics: xxx

**Important Notes**:
- Do not output specific data values, only describe the field metadata
- Use clear business terms for easy user understanding and search
- If enum value ranges can be inferred from sample data, provide a summary (e.g., status field contains pending/in-progress/completed states)
- Write descriptions in the same language as the data content`
)

// NewChunkExtractTask creates a new chunk extract task. It returns
// (enqueued, err): enqueued is true only when a task was actually placed on
// the queue. When NEO4J is disabled the call is a no-op and returns
// (false, nil) — callers that seeded a pending-subtask counter for this chunk
// MUST release that slot, otherwise the parent knowledge stays stuck in
// "finalizing" forever (the graph subtask it's waiting on was never enqueued).
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
	if strings.ToLower(os.Getenv("NEO4J_ENABLE")) != "true" {
		logger.Warn(ctx, "NEO4J is not enabled, skip chunk extract task")
		return false, nil
	}
	taskPayload := types.ExtractChunkPayload{
		TenantID:    tenantID,
		ChunkID:     chunkID,
		ModelID:     modelID,
		KnowledgeID: knowledgeID,
		Attempt:     attempt,
		ChunkIndex:  chunkIndex,
	}
	langfuse.InjectTracing(ctx, &taskPayload)
	payload, err := json.Marshal(taskPayload)
	if err != nil {
		return false, err
	}
	task := asynq.NewTask(types.TypeChunkExtract, payload,
		asynq.Queue(types.QueueGraph), asynq.MaxRetry(3), asynq.Timeout(30*time.Minute))
	info, err := client.Enqueue(task)
	if err != nil {
		logger.Errorf(ctx, "failed to enqueue task: %v", err)
		return false, fmt.Errorf("failed to enqueue task: %v", err)
	}
	logger.Infof(ctx, "enqueued task: id=%s queue=%s chunk=%s", info.ID, info.Queue, chunkID)
	return true, nil
}

// NewTableExtractTask creates a new table extract task
func NewDataTableSummaryTask(
	ctx context.Context,
	client interfaces.TaskEnqueuer,
	tenantID uint64,
	knowledgeID string,
	summaryModel string,
	embeddingModel string,
) error {
	taskPayload := DataTableSummaryPayload{
		TenantID:       tenantID,
		KnowledgeID:    knowledgeID,
		SummaryModel:   summaryModel,
		EmbeddingModel: embeddingModel,
	}
	langfuse.InjectTracing(ctx, &taskPayload)
	payload, err := json.Marshal(taskPayload)
	if err != nil {
		return err
	}
	task := asynq.NewTask(types.TypeDataTableSummary, payload,
		asynq.Queue(types.QueueSummary), asynq.MaxRetry(3), asynq.Timeout(30*time.Minute))
	info, err := client.Enqueue(task)
	if err != nil {
		logger.Errorf(ctx, "failed to enqueue data table summary task: %v", err)
		return fmt.Errorf("failed to enqueue data table summary task: %v", err)
	}
	logger.Infof(ctx, "enqueued data table summary task: id=%s queue=%s knowledge=%s",
		info.ID, info.Queue, knowledgeID)
	return nil
}

// ChunkExtractService is a service for extracting chunks
type ChunkExtractService struct {
	template          *types.PromptTemplateStructured
	modelService      interfaces.ModelService
	knowledgeBaseRepo interfaces.KnowledgeBaseRepository
	knowledgeRepo     interfaces.KnowledgeRepository
	chunkRepo         interfaces.ChunkRepository
	graphEngine       interfaces.RetrieveGraphRepository
	// spanTrace records this graph-extract task's subspan under the
	// parent attempt's postprocess stage so the trace viewer shows real
	// per-chunk graph extraction time rather than the upstream's enqueue.
	// Pass B K1.3 R2 seam（plan 21 §6.3）：原 spanTracker SpanTracker（K4 属主
	// knowledge_span_tracker.go）改经 SpanTraceSeam 投影，构造注入，禁包级 var。
	spanTrace SpanTraceSeam
	// attemptSupersededFn 承载宿主 attemptSuperseded（knowledge.go:202，K4 属主；
	// 原 4 参吸收 tracker 为闭包）。plan §6.3。
	attemptSupersededFn func(context.Context, string, int) bool
	// previewTextFn 承载宿主 previewText（wiki_ingest.go:1383，K3 属主）。
	// plan §6.3 组 A；生产接线 K5 接 K3 导出 PreviewText。
	previewTextFn func(s string, maxRunes int) string
	// 以下三项为 K1.3 增量 seam（plan §6.3 未枚举，按既有 R2 机制具体化，
	// 节点报告登记）：finalizeSubtaskFn 承载宿主 finalizeSubtaskDetached
	// （knowledge.go:235，K4）；isFinalAttemptFn 承载宿主 isFinalAsynqAttempt
	// （image_multimodal.go:413，K1 自有，K1.4 随文件入包后同包收敛）；
	// resolveProcessConfigFn 承载宿主 ResolveProcessConfig
	// （knowledge_process_config.go:40，K4）。
	finalizeSubtaskFn      func(ctx context.Context, repo interfaces.KnowledgeRepository, knowledgeID, source string, retErr error, superseded, final bool)
	isFinalAttemptFn       func(ctx context.Context) bool
	resolveProcessConfigFn func(kb *types.KnowledgeBase, overrides *types.KnowledgeProcessOverrides) types.EffectiveProcessConfig
	// newGraphExtractor 承载 conversation/chat_pipeline.NewExtractor（K1.3
	// 增量 seam：chat_pipeline 传递依赖 repository 成环，见 seams.go 注释）。
	newGraphExtractor GraphExtractorFactory
}

// NewChunkExtractService creates a new chunk extract service
// Pass B K1.3：末参 spanTracker SpanTracker 改为 spanTrace SpanTraceSeam +
// attemptSupersededFn（plan §6.1）；previewTextFn/finalizeSubtaskFn/
// isFinalAttemptFn/resolveProcessConfigFn/newGraphExtractor 为增量 seam
// 闭包（R2 构造注入）。
// OCR f3：六项 seam 闭包在 Handle/defer 期被无条件调用（:253 attemptSupersededFn；
// defer :284-286 finalizeSubtaskFn/isFinalAttemptFn 每个终端路径必经），漏注 nil
// 将 panic 于 defer 且 pending_subtasks_count 永不递减——构造期 fail-fast
// （格式对齐宿主先例 service/session.go:176）。spanTrace 为唯一允许 nil 的
// seam：trace() 回退 noopSpanTraceSeam，零值语义与原 noopSpanTracker{} 一致
// （plan §6.3），不参与 fail-fast。
func NewChunkExtractService(
	config *config.Config,
	modelService interfaces.ModelService,
	knowledgeBaseRepo interfaces.KnowledgeBaseRepository,
	knowledgeRepo interfaces.KnowledgeRepository,
	chunkRepo interfaces.ChunkRepository,
	graphEngine interfaces.RetrieveGraphRepository,
	spanTrace SpanTraceSeam,
	attemptSupersededFn func(context.Context, string, int) bool,
	previewTextFn func(s string, maxRunes int) string,
	finalizeSubtaskFn func(ctx context.Context, repo interfaces.KnowledgeRepository, knowledgeID, source string, retErr error, superseded, final bool),
	isFinalAttemptFn func(ctx context.Context) bool,
	resolveProcessConfigFn func(kb *types.KnowledgeBase, overrides *types.KnowledgeProcessOverrides) types.EffectiveProcessConfig,
	newGraphExtractor GraphExtractorFactory,
) interfaces.TaskHandler {
	// 漏注任何一项 seam 闭包都是接线 bug 而非运行期条件：构造期点名 panic，
	// 而不是等到 Handle/defer 期 nil 调用（OCR f3，先例 session.go:176）。
	if attemptSupersededFn == nil {
		panic("NewChunkExtractService: attemptSupersededFn is required (stale-attempt short-circuit runs unconditionally in Handle)")
	}
	if previewTextFn == nil {
		panic("NewChunkExtractService: previewTextFn is required (graph extract input truncation)")
	}
	if finalizeSubtaskFn == nil {
		panic("NewChunkExtractService: finalizeSubtaskFn is required (defer decrements pending_subtasks_count on every terminal path)")
	}
	if isFinalAttemptFn == nil {
		panic("NewChunkExtractService: isFinalAttemptFn is required (dead-letter decision in the finalize defer)")
	}
	if resolveProcessConfigFn == nil {
		panic("NewChunkExtractService: resolveProcessConfigFn is required (per-KB effective process config)")
	}
	if newGraphExtractor == nil {
		panic("NewChunkExtractService: newGraphExtractor is required (chat_pipeline extractor seam)")
	}
	return &ChunkExtractService{
		template:               config.ExtractManager.ExtractGraph,
		modelService:           modelService,
		knowledgeBaseRepo:      knowledgeBaseRepo,
		knowledgeRepo:          knowledgeRepo,
		chunkRepo:              chunkRepo,
		graphEngine:            graphEngine,
		spanTrace:              spanTrace,
		attemptSupersededFn:    attemptSupersededFn,
		previewTextFn:          previewTextFn,
		finalizeSubtaskFn:      finalizeSubtaskFn,
		isFinalAttemptFn:       isFinalAttemptFn,
		resolveProcessConfigFn: resolveProcessConfigFn,
		newGraphExtractor:      newGraphExtractor,
	}
}

// trace 返回 seam 句柄；nil 回退 noopSpanTraceSeam，零值语义与原
// tracker() 的 noopSpanTracker{} 一致（plan §6.3）。
func (s *ChunkExtractService) trace() SpanTraceSeam {
	if s.spanTrace == nil {
		return noopSpanTraceSeam{}
	}
	return s.spanTrace
}

// Handle handles the chunk extraction task
func (s *ChunkExtractService) Handle(ctx context.Context, t *asynq.Task) error {
	var p types.ExtractChunkPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		logger.Errorf(ctx, "failed to unmarshal task payload: %v", err)
		return err
	}
	ctx = logger.WithRequestID(ctx, uuid.New().String())
	ctx = logger.WithField(ctx, "extract", p.ChunkID)
	ctx = context.WithValue(ctx, types.TenantIDContextKey, p.TenantID)

	// A newer attempt (re-upload / edit / reparse) has superseded this one:
	// skip before opening the span or registering the FinalizeSubtask defer.
	// The chunk this task references was deleted by the new attempt's cleanup,
	// and decrementing here would drain the new attempt's counter.
	if s.attemptSupersededFn(ctx, p.KnowledgeID, p.Attempt) {
		logger.Infof(ctx, "graph extract: attempt %d superseded for %s, skipping stale enrichment",
			p.Attempt, p.KnowledgeID)
		return nil
	}

	// Open a postprocess subspan keyed by chunk ordinal so the trace
	// shows real per-chunk graph extraction time. Skipped silently when
	// upstream didn't pass the parent attempt (legacy in-flight tasks)
	// or when the postprocess stage span isn't found.
	var gSpan any
	if p.KnowledgeID != "" && p.Attempt > 0 {
		parent := s.trace().LookupStage(ctx, p.KnowledgeID, p.Attempt, types.StagePostProcess)
		if parent != nil {
			gSpan = s.trace().BeginSubSpan(ctx, parent,
				fmt.Sprintf("postprocess.graph.chunk[%d]", p.ChunkIndex),
				types.SpanKindSubSpan,
				types.JSONMap{
					"chunk_id":    p.ChunkID,
					"chunk_index": p.ChunkIndex,
					"model_id":    p.ModelID,
				})
		}
	}
	var handleErr error
	graphOut := types.JSONMap{}
	defer func() {
		// Decrement the parent's enrichment counter on terminal exit so a
		// completed (or terminally-failed) per-chunk extract releases its
		// slot in pending_subtasks_count. KnowledgeID is the new (post-#? )
		// payload field; legacy in-flight tasks without it are skipped.
		s.finalizeSubtaskFn(ctx, s.knowledgeRepo, p.KnowledgeID,
			fmt.Sprintf("graph_chunk[%d]", p.ChunkIndex),
			handleErr, false, s.isFinalAttemptFn(ctx))
		if gSpan == nil {
			return
		}
		if handleErr != nil {
			s.trace().FailSpan(ctx, gSpan, "GRAPH_EXTRACT_FAILED", handleErr.Error(), handleErr)
		} else {
			s.trace().EndSpan(ctx, gSpan, graphOut)
		}
	}()

	// Short-circuit when the parent knowledge has been cancelled / deleted.
	// Each graph extract is per-chunk and runs one LLM call — the most
	// expensive enrichment fan-out in the pipeline. Skipping on cancel
	// is the whole point of the finalizing-state machinery above.
	if p.KnowledgeID != "" && s.knowledgeRepo != nil {
		if k, kerr := s.knowledgeRepo.GetKnowledgeByIDOnly(ctx, p.KnowledgeID); kerr == nil && k != nil {
			switch k.ParseStatus {
			case types.ParseStatusCancelled, types.ParseStatusDeleting:
				logger.Infof(ctx, "graph extract: knowledge %s aborted (%s), skipping chunk %s",
					p.KnowledgeID, k.ParseStatus, p.ChunkID)
				graphOut["skipped"] = "knowledge_" + k.ParseStatus
				return nil
			}
		}
	}

	chunk, err := s.chunkRepo.GetChunkByID(ctx, p.TenantID, p.ChunkID)
	if err != nil {
		logger.Errorf(ctx, "failed to get chunk: %v", err)
		handleErr = err
		return err
	}
	// Capture chunk content shape on output — lets traces answer "WHAT
	// did the LLM call see?" without joining back to the chunk store.
	// Preview is truncated to keep span rows reasonable.
	if gSpan != nil {
		graphOut["chunk_chars"] = len([]rune(chunk.Content))
		graphOut["chunk_preview"] = s.previewTextFn(chunk.Content, 200)
	}
	kb, err := s.knowledgeBaseRepo.GetKnowledgeBaseByID(ctx, chunk.KnowledgeBaseID)
	if err != nil {
		logger.Errorf(ctx, "failed to get knowledge base: %v", err)
		handleErr = err
		return err
	}

	var processOverrides *types.KnowledgeProcessOverrides
	knowledgeID := p.KnowledgeID
	if knowledgeID == "" {
		knowledgeID = chunk.KnowledgeID
	}
	if knowledgeID != "" && s.knowledgeRepo != nil {
		if k, kerr := s.knowledgeRepo.GetKnowledgeByIDOnly(ctx, knowledgeID); kerr == nil && k != nil {
			processOverrides, _ = k.ProcessOverrides()
		}
	}
	extractCfg := s.resolveProcessConfigFn(kb, processOverrides).ExtractConfig
	if !extractCfg.Enabled {
		logger.Warnf(ctx, "extract config not enabled")
		graphOut["skipped"] = "extract_disabled"
		return nil
	}

	chatModel, err := s.modelService.GetChatModel(ctx, p.ModelID)
	if err != nil {
		logger.Errorf(ctx, "failed to get chat model: %v", err)
		handleErr = err
		return err
	}

	template := &types.PromptTemplateStructured{
		Description: types.AppendCustomPromptInstructions(
			s.template.Description, extractCfg.CustomInstructions, "graph_extraction"),
		Tags: extractCfg.Tags,
		Examples: []types.GraphData{
			{
				Text:     extractCfg.Text,
				Node:     extractCfg.Nodes,
				Relation: extractCfg.Relations,
			},
		},
	}
	extractor := s.newGraphExtractor(chatModel, template)
	graph, err := extractor.Extract(ctx, chunk.Content)
	if err != nil {
		handleErr = err
		return err
	}

	chunk, err = s.chunkRepo.GetChunkByID(ctx, p.TenantID, p.ChunkID)
	if err != nil {
		logger.Warnf(ctx, "graph ignore chunk %s: %v", p.ChunkID, err)
		graphOut["skipped"] = "chunk_disappeared"
		return nil
	}

	for _, node := range graph.Node {
		node.Chunks = []string{chunk.ID}
	}
	if err = s.graphEngine.AddGraph(ctx,
		types.NameSpace{KnowledgeBase: chunk.KnowledgeBaseID, Knowledge: chunk.KnowledgeID},
		[]*types.GraphData{graph},
	); err != nil {
		logger.Errorf(ctx, "failed to add graph: %v", err)
		handleErr = err
		return err
	}
	graphOut["nodes_added"] = len(graph.Node)
	graphOut["relations_added"] = len(graph.Relation)
	// Capture a couple of sample nodes/relations so the trace viewer can
	// answer "what did the LLM actually extract?" without round-tripping
	// to the graph store. Cap to two each — anything more bloats span
	// rows and the full graph is queryable elsewhere.
	if len(graph.Node) > 0 {
		samples := graph.Node
		if len(samples) > 2 {
			samples = samples[:2]
		}
		names := make([]string, 0, len(samples))
		for _, n := range samples {
			names = append(names, n.Name)
		}
		graphOut["sample_nodes"] = names
	}
	if len(graph.Relation) > 0 {
		samples := graph.Relation
		if len(samples) > 2 {
			samples = samples[:2]
		}
		out := make([]string, 0, len(samples))
		for _, r := range samples {
			out = append(out, fmt.Sprintf("%s --[%s]--> %s", r.Node1, r.Type, r.Node2))
		}
		graphOut["sample_relations"] = out
	}
	return nil
}

// DataTableExtractPayload represents the table extract task payload
type DataTableSummaryPayload struct {
	types.TracingContext
	TenantID       uint64 `json:"tenant_id"`
	KnowledgeID    string `json:"knowledge_id"`
	SummaryModel   string `json:"summary_model"`
	EmbeddingModel string `json:"embedding_model"`
}

// DataTableSummaryService is a service for extracting tables
type DataTableSummaryService struct {
	modelService         interfaces.ModelService
	knowledgeBaseService interfaces.KnowledgeBaseService
	knowledgeService     interfaces.KnowledgeService
	fileService          interfaces.FileService
	chunkService         interfaces.ChunkService
	tenantService        interfaces.TenantService
	retrieveEngine       interfaces.RetrieveEngineRegistry
	ownership            retriever.TenantStoreOwnership
	sqlDB                *sql.DB
	storageResolver      interfaces.StorageBackendResolver
	// Pass B K1.3 增量 seam（plan §6.3 未枚举，按既有 R2 机制具体化，节点报告
	// 登记）：knowledgeWriteKBFn 承载宿主 knowledgeWriteKB（knowledge_write.go:42，
	// K4 属主；lookup 参数结构等价 KBByIDLookup）；resolveProcessConfigFn 承载宿主
	// ResolveProcessConfig（knowledge_process_config.go:40，K4 属主）。
	knowledgeWriteKBFn     func(ctx context.Context, lookup KBByIDLookup, knowledge *types.Knowledge) (*types.KnowledgeBase, error)
	resolveProcessConfigFn func(kb *types.KnowledgeBase, overrides *types.KnowledgeProcessOverrides) types.EffectiveProcessConfig
	// newDataAnalysisTool 承载 agentruntime/agent/tools 的 DuckDB 分析工具工厂
	// （K1.3 增量 seam：直连 tools 构成 repository→ingest→tools→repository
	// import 环，见 seams.go DataAnalysisToolSeam 注释）。
	newDataAnalysisTool DataAnalysisToolFactory
}

// NewDataTableSummaryService creates a new DataTableSummaryService
// Pass B K1.3：原 10 参签名追加 knowledgeWriteKBFn/resolveProcessConfigFn/
// newDataAnalysisTool 三项增量 seam（R2 构造注入）；宿主旧装配经 R1-4 shim
// 以本包函数值/提供器供给。
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
	knowledgeWriteKBFn func(ctx context.Context, lookup KBByIDLookup, knowledge *types.Knowledge) (*types.KnowledgeBase, error),
	resolveProcessConfigFn func(kb *types.KnowledgeBase, overrides *types.KnowledgeProcessOverrides) types.EffectiveProcessConfig,
	newDataAnalysisTool DataAnalysisToolFactory,
) interfaces.TaskHandler {
	return &DataTableSummaryService{
		modelService:           modelService,
		knowledgeBaseService:   knowledgeBaseService,
		knowledgeService:       knowledgeService,
		fileService:            fileService,
		chunkService:           chunkService,
		tenantService:          tenantService,
		retrieveEngine:         retrieveEngine,
		ownership:              ownership,
		sqlDB:                  sqlDB,
		storageResolver:        storageResolver,
		knowledgeWriteKBFn:     knowledgeWriteKBFn,
		resolveProcessConfigFn: resolveProcessConfigFn,
		newDataAnalysisTool:    newDataAnalysisTool,
	}
}

// Handle implements the TaskHandler interface for table extraction
// 整体流程：初始化 -> 准备资源 -> 加载数据 -> 生成摘要 -> 创建索引
func (s *DataTableSummaryService) Handle(ctx context.Context, t *asynq.Task) error {
	// 1. 解析任务并初始化上下文
	var payload DataTableSummaryPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		logger.Errorf(ctx, "failed to unmarshal table extract task payload: %v", err)
		return err
	}

	ctx = logger.WithRequestID(ctx, uuid.New().String())
	ctx = logger.WithField(ctx, "knowledge", payload.KnowledgeID)
	ctx = types.WithExecutionTenant(ctx, payload.TenantID)

	logger.Infof(ctx, "Processing table extraction for knowledge: %s", payload.KnowledgeID)

	_, err := s.knowledgeService.GetRepository().GetKnowledgeByID(ctx, payload.TenantID, payload.KnowledgeID)
	if isKnowledgeNotFound(err) {
		logger.Infof(ctx, "Skipping orphaned table summary task: knowledge %s not found in tenant %d",
			payload.KnowledgeID, payload.TenantID)
		return nil
	}
	if err != nil {
		logger.Errorf(ctx, "failed to get knowledge: %v", err)
		return err
	}

	// 2. 准备所有必需的资源（知识、模型、引擎等）
	resources, err := s.prepareResources(ctx, payload)
	if err != nil {
		return err
	}

	ctx, err = access.WithKBTaskWrite(ctx, resources.knowledgeBase, payload.TenantID)
	if err != nil {
		return err
	}
	ctx = context.WithValue(ctx, types.TenantInfoContextKey, resources.tenant)

	// 3. 加载表格数据并生成摘要
	chunks, err := s.processTableData(ctx, resources)
	if err != nil {
		return err
	}

	// 4. 索引到向量数据库
	if err := s.indexToVectorDB(ctx, chunks, resources.RetrieveEngine, resources.EmbeddingModel); err != nil {
		s.CleanupOnFailure(ctx, resources, chunks, err)
		return err
	}

	logger.Infof(ctx, "Table extraction completed for knowledge: %s", payload.KnowledgeID)
	return nil
}

// ExtractionResources 封装提取过程所需的所有资源。
// Pass B K1.3 R1 窄端口（增量，超出 §6.2 字面清单）：cleanup 失败清理路径的
// 三字段（Knowledge/RetrieveEngine/EmbeddingModel）导出，供宿主孤儿测试夹具
// （knowledge_transfer_test.go，Ruling 2026-09-24-TEST-SUPPORT-SHIM 垫片）构造
// 投影调用真实现；knowledgeBase/tenant/chatModel 保持包内私有。
type ExtractionResources struct {
	Knowledge      *types.Knowledge
	knowledgeBase  *types.KnowledgeBase
	tenant         *types.Tenant
	chatModel      chat.Chat
	EmbeddingModel embedding.Embedder
	RetrieveEngine *retriever.CompositeRetrieveEngine
}

// prepareResources 准备提取所需的所有资源
// 思路：集中加载所有依赖，统一错误处理，避免分散的资源获取逻辑
func (s *DataTableSummaryService) prepareResources(ctx context.Context, payload DataTableSummaryPayload) (*ExtractionResources, error) {
	// 获取并验证知识文件
	knowledge, err := s.knowledgeService.GetRepository().GetKnowledgeByID(ctx, payload.TenantID, payload.KnowledgeID)
	if err != nil {
		logger.Errorf(ctx, "failed to get knowledge: %v", err)
		return nil, err
	}

	if knowledge == nil || knowledge.ID != payload.KnowledgeID || knowledge.TenantID != payload.TenantID {
		return nil, fmt.Errorf("invalid table summary knowledge scope")
	}
	kb, err := s.knowledgeWriteKBFn(ctx, s.knowledgeBaseService, knowledge)
	if err != nil {
		return nil, err
	}

	// 验证文件类型
	fileType := strings.ToLower(knowledge.FileType)
	if fileType != "csv" && fileType != "xlsx" && fileType != "xls" {
		logger.Warnf(ctx, "knowledge %s is not a CSV or Excel file, skipping table summary", payload.KnowledgeID)
		return nil, fmt.Errorf("unsupported file type: %s", fileType)
	}

	// 获取空间信息
	tenantInfo, err := s.tenantService.GetTenantByID(ctx, payload.TenantID)
	if err != nil {
		logger.Errorf(ctx, "failed to get tenant: %v", err)
		return nil, err
	}

	// 获取聊天模型（用于生成摘要）
	chatModel, err := s.modelService.GetChatModel(ctx, payload.SummaryModel)
	if err != nil {
		logger.Errorf(ctx, "failed to get chat model: %v", err)
		return nil, err
	}

	// 获取嵌入模型（用于向量化）
	embeddingModel, err := s.modelService.GetEmbeddingModel(ctx, payload.EmbeddingModel)
	if err != nil {
		logger.Errorf(ctx, "failed to get embedding model: %v", err)
		return nil, err
	}

	var vectorStoreID *string
	if kb != nil {
		vectorStoreID = kb.VectorStoreID
	}

	// The factory's unbound path reads TenantInfo from ctx.
	ctx = context.WithValue(ctx, types.TenantInfoContextKey, tenantInfo)

	// Resolve the engine via the factory using the KB's VectorStore binding
	// (nil -> tenant effective engines fallback; verified tenant ownership otherwise).
	retrieveEngine, err := retriever.CreateRetrieveEngineForKB(
		ctx, s.retrieveEngine, s.ownership, payload.TenantID, vectorStoreID)
	if err != nil {
		logger.Errorf(ctx, "failed to get retrieve engine: %v", err)
		return nil, err
	}

	return &ExtractionResources{
		Knowledge:      knowledge,
		knowledgeBase:  kb,
		tenant:         tenantInfo,
		chatModel:      chatModel,
		EmbeddingModel: embeddingModel,
		RetrieveEngine: retrieveEngine,
	}, nil
}

// resolveFileServiceForKnowledge resolves a provider-specific file service for the current knowledge file.
// It falls back to the global service when tenant storage config is unavailable.
func (s *DataTableSummaryService) resolveFileServiceForKnowledge(ctx context.Context, resources *ExtractionResources) interfaces.FileService {
	if resources == nil || resources.Knowledge == nil {
		return s.fileService
	}
	if resources.tenant == nil {
		return s.fileService
	}

	provider := types.InferStorageFromFilePath(resources.Knowledge.FilePath)
	if provider == "" && resources.tenant.StorageEngineConfig != nil {
		provider = strings.ToLower(strings.TrimSpace(resources.tenant.StorageEngineConfig.DefaultProvider))
	}

	baseDir := strings.TrimSpace(os.Getenv("LOCAL_STORAGE_BASE_DIR"))
	backendID, _, _ := types.ParseStorageBackendPath(resources.Knowledge.FilePath)
	if backendID == "" && resources.knowledgeBase != nil && resources.knowledgeBase.StorageBackendID != nil {
		backendID = strings.TrimSpace(*resources.knowledgeBase.StorageBackendID)
	}

	// New-model workspaces resolve via DefaultStorageBackendID even when no
	// legacy StorageEngineConfig / provider is present, so gate on the resolver
	// and a usable backendID/provider rather than requiring a non-empty provider.
	if s.storageResolver == nil || (backendID == "" && provider == "") {
		return s.fileService
	}

	resolvedSvc, resolvedProvider, err := s.storageResolver.ResolveFileService(ctx, resources.tenant, backendID, provider, baseDir)
	if err != nil {
		logger.Warnf(ctx, "[TableSummary] Failed to resolve file service for provider=%s, fallback to default: %v", provider, err)
		return s.fileService
	}
	logger.Infof(ctx, "[TableSummary] Resolved file service for knowledge=%s provider=%s", resources.Knowledge.ID, resolvedProvider)
	return resolvedSvc
}

// processTableData 处理表格数据：加载 -> 分析 -> 生成摘要 -> 创建chunks
// 思路：将数据处理的核心流程集中在一起，保持逻辑连贯性
func (s *DataTableSummaryService) processTableData(ctx context.Context, resources *ExtractionResources) ([]*types.Chunk, error) {
	// 创建DuckDB会话并加载数据
	sessionID := fmt.Sprintf("table_summary_%s", resources.Knowledge.ID)
	fileSvc := s.resolveFileServiceForKnowledge(ctx, resources)
	duckdbTool := s.newDataAnalysisTool(s.knowledgeBaseService, s.knowledgeService, s.tenantService, fileSvc, s.sqlDB, sessionID, s.storageResolver)
	defer duckdbTool.Cleanup(ctx)

	// 使用knowledge.ID作为表名，根据文件类型自动加载数据
	tableSchema, err := duckdbTool.LoadFromKnowledge(ctx, resources.Knowledge)
	if err != nil {
		logger.Errorf(ctx, "failed to load data into DuckDB: %v", err)
		return nil, err
	}

	logger.Infof(ctx, "Loaded table %s with %d columns and %d rows", tableSchema.TableName, tableSchema.ColumnCount, tableSchema.RowCount)

	// 获取样本数据用于生成摘要（DataAnalysisInput 的 JSON 序列化由 seam 适配侧承载）
	sampleResult, err := duckdbTool.Execute(ctx, resources.Knowledge.ID,
		fmt.Sprintf("SELECT * FROM \"%s\" LIMIT 10", tableSchema.TableName))
	if err != nil {
		logger.Errorf(ctx, "failed to get sample data: %v", err)
		return nil, err
	}

	// 构建共用的schema和样本数据描述
	schemaDesc := tableSchema.Description
	sampleDesc := s.buildSampleDataDescription(ctx, sampleResult, 10)

	// 使用AI生成表格摘要和列描述
	customInstructions := ""
	if resources.knowledgeBase != nil {
		var processOverrides *types.KnowledgeProcessOverrides
		if resources.Knowledge != nil {
			processOverrides, _ = resources.Knowledge.ProcessOverrides()
		}
		customInstructions = s.resolveProcessConfigFn(resources.knowledgeBase, processOverrides).ChunkingConfig.TableMetadataInstructions
	}
	// The stored summary is later shown to the model by data_schema, so it
	// must name the model-facing table, never the physical knowledge-ID table.
	tableDescription, err := s.generateTableDescription(ctx, resources.chatModel, dataAnalysisTableName,
		schemaDesc, sampleDesc, customInstructions)
	if err != nil {
		logger.Errorf(ctx, "failed to generate table description: %v", err)
		return nil, err
	}
	logger.Debugf(ctx, "table describe of knowledge %s: %s", resources.Knowledge.ID, tableDescription)

	columnDescription, err := s.generateColumnDescriptions(ctx, resources.chatModel, dataAnalysisTableName,
		schemaDesc, sampleDesc, customInstructions)
	if err != nil {
		logger.Errorf(ctx, "failed to generate column descriptions: %v", err)
		return nil, err
	}
	logger.Debugf(ctx, "column describe of knowledge %s: %s", resources.Knowledge.ID, columnDescription)

	// 构建chunks：一个表格摘要chunk + 多个列描述chunks
	chunks := s.buildChunks(resources, tableDescription, columnDescription)
	return chunks, nil
}

// buildChunks 构建chunk对象
// tableDescription和columnDescriptions分别生成一个chunk
func (s *DataTableSummaryService) buildChunks(resources *ExtractionResources, tableDescription string, columnDescription string) []*types.Chunk {
	chunks := make([]*types.Chunk, 0, 2)

	// 表格摘要chunk
	summaryChunk := &types.Chunk{
		ID:              uuid.New().String(),
		TenantID:        resources.Knowledge.TenantID,
		KnowledgeID:     resources.Knowledge.ID,
		KnowledgeBaseID: resources.Knowledge.KnowledgeBaseID,
		Content:         tableDescription,
		ChunkIndex:      0,
		IsEnabled:       true,
		ChunkType:       types.ChunkTypeTableSummary,
		Status:          int(types.ChunkStatusStored),
	}
	chunks = append(chunks, summaryChunk)

	// 列描述chunk（所有列的描述合并为一个chunk）
	columnChunk := &types.Chunk{
		ID:              uuid.New().String(),
		TenantID:        resources.Knowledge.TenantID,
		KnowledgeID:     resources.Knowledge.ID,
		KnowledgeBaseID: resources.Knowledge.KnowledgeBaseID,
		Content:         columnDescription,
		ChunkIndex:      1,
		IsEnabled:       true,
		ChunkType:       types.ChunkTypeTableColumn,
		ParentChunkID:   summaryChunk.ID,
		Status:          int(types.ChunkStatusStored),
	}
	chunks = append(chunks, columnChunk)

	summaryChunk.NextChunkID = columnChunk.ID
	columnChunk.PreChunkID = summaryChunk.ID

	return chunks
}

// indexToVectorDB 将chunks索引到向量数据库
// 思路：批量构建索引信息，统一索引，更新状态
func (s *DataTableSummaryService) indexToVectorDB(
	ctx context.Context,
	chunks []*types.Chunk,
	engine *retriever.CompositeRetrieveEngine,
	embedder embedding.Embedder,
) error {
	// 构建索引信息列表
	indexInfoList := make([]*types.IndexInfo, 0, len(chunks))
	for _, chunk := range chunks {
		indexInfoList = append(indexInfoList, &types.IndexInfo{
			Content:         chunk.Content,
			SourceID:        chunk.ID,
			SourceType:      types.ChunkSourceType,
			ChunkID:         chunk.ID,
			KnowledgeID:     chunk.KnowledgeID,
			KnowledgeBaseID: chunk.KnowledgeBaseID,
			IsEnabled:       true,
		})
	}

	// 保存到数据库
	if err := s.chunkService.CreateChunks(ctx, chunks); err != nil {
		logger.Errorf(ctx, "failed to create chunks: %v", err)
		return err
	}
	logger.Infof(ctx, "Created %d chunks for data table", len(chunks))

	// 批量索引
	if err := engine.BatchIndex(ctx, embedder, indexInfoList); err != nil {
		logger.Errorf(ctx, "failed to index chunks: %v", err)
		return err
	}

	// 更新chunk状态为已索引
	for _, chunk := range chunks {
		chunk.Status = int(types.ChunkStatusIndexed)
	}
	if err := s.chunkService.UpdateChunks(ctx, chunks); err != nil {
		logger.Errorf(ctx, "failed to update chunk status: %v", err)
		return err
	}

	return nil
}

// CleanupOnFailure 索引失败时的清理工作。
// Pass B K1.3 R1 窄端口（增量，超出 §6.2 字面清单）：原 cleanupOnFailure 导出，
// 供宿主孤儿测试夹具经垫片委托到本真实现（同 K1.2 SameChunkDocument 先例），
// 唯一定义仍在 ingest。
// 思路：删除已创建的chunk和对应的向量索引，避免脏数据残留
func (s *DataTableSummaryService) CleanupOnFailure(ctx context.Context, resources *ExtractionResources, chunks []*types.Chunk, indexErr error) {
	logger.Warnf(ctx, "Starting cleanup due to failure: %v", indexErr)

	// 1. 更新知识状态为失败
	before, after := *resources.Knowledge, *resources.Knowledge
	after.ParseStatus = types.ParseStatusFailed
	after.ErrorMessage = indexErr.Error()
	if err := s.knowledgeService.GetRepository().UpdateKnowledgeForTransfer(ctx, &before, &after); err != nil {
		logger.Warnf(ctx, "Table summary cleanup skipped after knowledge changed: %v", err)
		return
	}

	// 提取chunk IDs
	chunkIDs := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		chunkIDs = append(chunkIDs, chunk.ID)
	}

	// 删除已创建的chunks
	if len(chunkIDs) > 0 {
		if err := s.chunkService.GetRepository().DeleteChunks(ctx, resources.Knowledge.TenantID, chunkIDs); err != nil {
			logger.Errorf(ctx, "Failed to delete chunks: %v", err)
		} else {
			logger.Infof(ctx, "Deleted %d chunks", len(chunkIDs))
		}
	}

	// 删除对应的向量索引
	if len(chunkIDs) > 0 {
		if err := resources.RetrieveEngine.DeleteBySourceIDList(
			ctx, chunkIDs, resources.EmbeddingModel.GetDimensions(), types.KnowledgeBaseTypeDocument,
		); err != nil {
			logger.Errorf(ctx, "Failed to delete vector index: %v", err)
		} else {
			logger.Infof(ctx, "Deleted vector index for %d chunks", len(chunkIDs))
		}
	}

	logger.Infof(ctx, "Cleanup completed")
}

// generateTableDescription generates a summary description for the entire table
func (s *DataTableSummaryService) generateTableDescription(ctx context.Context, chatModel chat.Chat,
	tableName, schemaDesc, sampleDesc, customInstructions string,
) (string, error) {
	prompt := fmt.Sprintf(tableDescriptionPromptTemplate, tableName, schemaDesc, sampleDesc)
	prompt = types.AppendCustomPromptInstructions(prompt, customInstructions, "table_metadata")
	// logger.Debugf(ctx, "generateTableDescription prompt: %s", prompt)

	thinking := false
	response, err := chatModel.Chat(ctx, []chat.Message{
		{Role: "user", Content: prompt},
	}, &chat.ChatOptions{
		Temperature: 0.3,
		MaxTokens:   512,
		Thinking:    &thinking,
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate table description: %w", err)
	}
	if _, err := validateSummaryOutput(response); err != nil {
		return "", fmt.Errorf("failed to generate table description: %w", err)
	}

	return fmt.Sprintf("# Table Summary\n\nTable name: %s\n\n%s", tableName, response.Content), nil
}

// dataAnalysisTableName mirrors tools.DataAnalysisTableName; importing the
// tools package here would close an import cycle (tools reaches this package
// via the chunk ingest shim).
const dataAnalysisTableName = "dataset"

// isKnowledgeNotFound matches the repository sentinel by text: importing the
// repository package here would close an import cycle (repository imports this
// package via the chunk ingest shim).
func isKnowledgeNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "knowledge not found")
}

// errEmptySummaryOutput marks a summary model response with no user-visible
// text; Asynq retries the task instead of persisting description="".
var errEmptySummaryOutput = errors.New("summary model returned empty output")

// validateSummaryOutput rejects successful model responses that contain no
// user-visible text. Treating whitespace-only output as an error lets Asynq
// retry the summary task instead of persisting description="" as completed.
func validateSummaryOutput(response *types.ChatResponse) (string, error) {
	if response == nil {
		return "", errEmptySummaryOutput
	}
	content := strings.TrimSpace(response.Content)
	if content == "" {
		return "", errEmptySummaryOutput
	}
	return content, nil
}

// generateColumnDescriptions generates descriptions for each column in batch
func (s *DataTableSummaryService) generateColumnDescriptions(ctx context.Context, chatModel chat.Chat,
	tableName, schemaDesc, sampleDesc, customInstructions string,
) (string, error) {
	// Build batch prompt for all columns
	prompt := fmt.Sprintf(columnDescriptionsPromptTemplate, tableName, schemaDesc, sampleDesc)
	prompt = types.AppendCustomPromptInstructions(prompt, customInstructions, "table_metadata")
	// logger.Debugf(ctx, "generateColumnDescriptions prompt: %s", prompt)

	// Call LLM once for all columns
	thinking := false
	response, err := chatModel.Chat(ctx, []chat.Message{
		{Role: "user", Content: prompt},
	}, &chat.ChatOptions{
		Temperature: 0.3,
		MaxTokens:   2048,
		Thinking:    &thinking,
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate column descriptions: %w", err)
	}
	if _, err := validateSummaryOutput(response); err != nil {
		return "", fmt.Errorf("failed to generate column descriptions: %w", err)
	}

	return fmt.Sprintf("# Table Column Information\n\nTable name: %s\n\n%s", tableName, response.Content), nil
}

// buildSampleDataDescription builds a formatted sample data description
func (s *DataTableSummaryService) buildSampleDataDescription(ctx context.Context, sampleData *types.ToolResult, maxRows int) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Sample data (first %d rows):\n", maxRows))

	if sampleData == nil || sampleData.Data == nil {
		return builder.String()
	}

	rawRows, exists := sampleData.Data["rows"]
	if !exists || rawRows == nil {
		return builder.String()
	}

	// DataAnalysisTool returns []map[string]string. A decoded ToolResult can
	// instead contain []map[string]interface{}, so normalize both shapes before
	// serializing the sample rows.
	var rows []interface{}
	switch typedRows := rawRows.(type) {
	case []map[string]string:
		rows = make([]interface{}, len(typedRows))
		for i, row := range typedRows {
			rows[i] = row
		}
	case []map[string]interface{}:
		rows = make([]interface{}, len(typedRows))
		for i, row := range typedRows {
			rows[i] = row
		}
	default:
		logger.Warnf(ctx, "[TableSummary] Unsupported sample rows type: %T", rawRows)
		return builder.String()
	}

	for i, row := range rows {
		if i >= maxRows {
			break
		}
		jsonBytes, err := json.Marshal(row)
		if err != nil {
			continue
		}
		builder.WriteString(string(jsonBytes))
		builder.WriteString("\n")
	}

	return builder.String()
}
