package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/models/embedding"
	"github.com/Tencent/WeKnora/internal/models/vlm"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/hibiken/asynq"
)

// 本文件钉住 #3834 的回归面：图片 OCR/Caption 分块的索引写入失败必须让
// 任务失败（asynq 重试），处理轨迹不得再记 indexed=true。

// ---- 测试桩 ----

type indexFakeVLM struct {
	ocrText     string
	captionText string
}

func (f *indexFakeVLM) Predict(_ context.Context, _ [][]byte, prompt string) (string, error) {
	if strings.Contains(prompt, "OCR assistant") {
		return f.ocrText, nil
	}
	return f.captionText, nil
}
func (f *indexFakeVLM) GetModelName() string { return "fake-vlm" }
func (f *indexFakeVLM) GetModelID() string   { return "vlm-1" }

type fakeEmbedder struct{}

func (fakeEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return []float32{0.1}, nil
}
func (fakeEmbedder) BatchEmbed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{0.1}
	}
	return out, nil
}
func (fakeEmbedder) GetModelName() string { return "fake-embedder" }
func (fakeEmbedder) GetDimensions() int   { return 1 }
func (fakeEmbedder) GetModelID() string   { return "emb-1" }
func (fakeEmbedder) BatchEmbedWithPool(ctx context.Context, model embedding.Embedder, texts []string) ([][]float32, error) {
	return model.BatchEmbed(ctx, texts)
}

type fakeModelService struct {
	interfaces.ModelService
	vlmModel       vlm.VLM
	embeddingModel embedding.Embedder
	embeddingErr   error
}

func (s *fakeModelService) GetVLMModel(context.Context, string) (vlm.VLM, error) {
	return s.vlmModel, nil
}
func (s *fakeModelService) GetEmbeddingModel(context.Context, string) (embedding.Embedder, error) {
	if s.embeddingErr != nil {
		return nil, s.embeddingErr
	}
	return s.embeddingModel, nil
}

type fakeEngineService struct {
	interfaces.RetrieveEngineService
	batchIndexErr   error
	batchIndexCalls int
	lastIndexed     int
}

func (f *fakeEngineService) EngineType() types.RetrieverEngineType {
	return types.PostgresRetrieverEngineType
}
func (f *fakeEngineService) Support() []types.RetrieverType {
	return []types.RetrieverType{types.VectorRetrieverType, types.KeywordsRetrieverType}
}
func (f *fakeEngineService) Retrieve(context.Context, types.RetrieveParams) ([]*types.RetrieveResult, error) {
	return nil, nil
}
func (f *fakeEngineService) BatchIndex(_ context.Context, _ embedding.Embedder, list []*types.IndexInfo, _ []types.RetrieverType) error {
	f.batchIndexCalls++
	f.lastIndexed = len(list)
	return f.batchIndexErr
}

type fakeEngineRegistry struct {
	interfaces.RetrieveEngineRegistry
	engine interfaces.RetrieveEngineService
}

func (r *fakeEngineRegistry) GetRetrieveEngineService(types.RetrieverEngineType) (interfaces.RetrieveEngineService, error) {
	return r.engine, nil
}

type fakeTenantRepo struct {
	interfaces.TenantRepository
}

func (fakeTenantRepo) GetTenantByID(_ context.Context, id uint64) (*types.Tenant, error) {
	return &types.Tenant{
		ID: id,
		RetrieverEngines: types.RetrieverEngines{Engines: []types.RetrieverEngineParams{
			{RetrieverType: types.VectorRetrieverType, RetrieverEngineType: types.PostgresRetrieverEngineType},
		}},
	}, nil
}

type fakeChunkRepo struct {
	interfaces.ChunkRepository
	chunks  map[string]*types.Chunk
	created []*types.Chunk
	updated []*types.Chunk
	deleted []string
}

func (r *fakeChunkRepo) CreateChunks(_ context.Context, chunks []*types.Chunk) error {
	for _, c := range chunks {
		r.chunks[c.ID] = c
		r.created = append(r.created, c)
	}
	return nil
}
func (r *fakeChunkRepo) GetChunkByIDOnly(_ context.Context, id string) (*types.Chunk, error) {
	if c, ok := r.chunks[id]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("chunk %s not found", id)
}
func (r *fakeChunkRepo) UpdateChunk(_ context.Context, chunk *types.Chunk) error {
	r.chunks[chunk.ID] = chunk
	r.updated = append(r.updated, chunk)
	return nil
}
func (r *fakeChunkRepo) DeleteChunks(_ context.Context, _ uint64, ids []string) error {
	r.deleted = append(r.deleted, ids...)
	for _, id := range ids {
		delete(r.chunks, id)
	}
	return nil
}

type fakeChunkService struct {
	interfaces.ChunkService
	repo *fakeChunkRepo
}

func (s *fakeChunkService) GetRepository() interfaces.ChunkRepository { return s.repo }
func (s *fakeChunkService) GetChunkByIDOnly(ctx context.Context, id string) (*types.Chunk, error) {
	return s.repo.GetChunkByIDOnly(ctx, id)
}

type indexFakeFileService struct {
	interfaces.FileService
}

func (indexFakeFileService) GetFile(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader([]byte("fake-image-bytes"))), nil
}

// captureSpanTrace 记录 EndSpan/FailSpan 的调用，用于断言处理轨迹输出。
type captureSpanTrace struct {
	endOutputs []types.JSONMap
	failCodes  []string
}

func (t *captureSpanTrace) LookupStage(context.Context, string, int, string) any {
	return struct{}{}
}
func (t *captureSpanTrace) BeginSubSpan(_ context.Context, _ any, _, _ string, _ types.JSONMap) any {
	return struct{}{}
}
func (t *captureSpanTrace) EndSpan(_ context.Context, _ any, output types.JSONMap) {
	t.endOutputs = append(t.endOutputs, output)
}
func (t *captureSpanTrace) FailSpan(_ context.Context, _ any, code, _ string, _ error) {
	t.failCodes = append(t.failCodes, code)
}

// indexTestKBService 让 GetKnowledgeBaseByIDOnly 在第 failAfter+1 次调用起
// 返回 err：Handle 内该方法是第 3 次被调（orphan 检查 → resolveVLM →
// indexChunks），前两次必须成功任务才能走到索引阶段。
type indexTestKBService struct {
	interfaces.KnowledgeBaseService
	kb        *types.KnowledgeBase
	err       error
	failAfter int
	calls     int
}

func (s *indexTestKBService) GetKnowledgeBaseByIDOnly(_ context.Context, _ string) (*types.KnowledgeBase, error) {
	s.calls++
	if s.err != nil && s.failAfter > 0 && s.calls > s.failAfter {
		return nil, s.err
	}
	return s.kb, nil
}

type indexTestHarness struct {
	svc       *ImageMultimodalService
	kbSvc     *indexTestKBService
	modelSvc  *fakeModelService
	engine    *fakeEngineService
	chunkRepo *fakeChunkRepo
	trace     *captureSpanTrace
}

func indexTestEmbeddingKB() *types.KnowledgeBase {
	return &types.KnowledgeBase{
		ID:               "kb-1",
		EmbeddingModelID: "emb-1",
		IndexingStrategy: types.IndexingStrategy{VectorEnabled: true},
	}
}

func newIndexTestHarness(kb *types.KnowledgeBase) *indexTestHarness {
	repo := &fakeChunkRepo{chunks: map[string]*types.Chunk{}}
	engine := &fakeEngineService{}
	h := &indexTestHarness{
		kbSvc: &indexTestKBService{kb: kb},
		modelSvc: &fakeModelService{
			vlmModel: &indexFakeVLM{
				ocrText:     "发票号码：88012345\n开票日期：2026-10-06",
				captionText: "A scanned invoice image.",
			},
			embeddingModel: fakeEmbedder{},
		},
		engine:    engine,
		chunkRepo: repo,
		trace:     &captureSpanTrace{},
	}
	h.svc = &ImageMultimodalService{
		chunkService:   &fakeChunkService{repo: repo},
		modelService:   h.modelSvc,
		kbService:      h.kbSvc,
		knowledgeRepo:  &orphanKnowledgeRepo{knowledge: &types.Knowledge{ParseStatus: types.ParseStatusProcessing}},
		tenantRepo:     fakeTenantRepo{},
		retrieveEngine: &fakeEngineRegistry{engine: engine},
		fileSvc:        indexFakeFileService{},
		// 哨兵孪生值（同 image_multimodal_orphan_test.go）：宿主哨兵经构造注入，
		// 测试内以 errors.New 孪生值承载，避免 errors.Is(nil, nil) 误判为孤儿。
		knowledgeNotFoundErr:     testKnowledgeNotFound,
		knowledgeBaseNotFoundErr: testKnowledgeBaseNotFound,
		spanTrace:                h.trace,
		previewTextFn: func(s string, maxRunes int) string {
			r := []rune(s)
			if len(r) > maxRunes {
				return string(r[:maxRunes])
			}
			return s
		},
		resolveProcessConfigFn: func(_ *types.KnowledgeBase, _ *types.KnowledgeProcessOverrides) types.EffectiveProcessConfig {
			return types.EffectiveProcessConfig{VLMConfig: types.VLMConfig{Enabled: true, ModelID: "vlm-1"}}
		},
		postProcessTaskOptionsFn: testPostProcessTaskOptions,
	}
	return h
}

func newImageMultimodalIndexTask(t *testing.T) *asynq.Task {
	t.Helper()
	payload, err := json.Marshal(types.ImageMultimodalPayload{
		TenantID:        1,
		KnowledgeID:     "k-1",
		KnowledgeBaseID: "kb-1",
		ChunkID:         "parent-chunk-1",
		ImageURL:        "minio://bucket/img.png",
		EnableOCR:       true,
		EnableCaption:   true,
		Attempt:         1,
	})
	if err != nil {
		t.Fatal(err)
	}
	return asynq.NewTask(types.TypeImageMultimodal, payload)
}

// assertNoIndexedTrueInTrace 断言失败路径没有任何 EndSpan 输出携带 indexed=true
// —— 处理轨迹不能再对未索引进检索引擎的分块宣称成功。
func assertNoIndexedTrueInTrace(t *testing.T, trace *captureSpanTrace) {
	t.Helper()
	for _, out := range trace.endOutputs {
		if v, ok := out["indexed"]; ok && v == true {
			t.Fatalf("trace output must not claim indexed=true on failure: %v", out)
		}
	}
}

// ---- 索引失败必须让任务失败（#3834） ----

func TestImageMultimodalHandleIndexBatchIndexFailureFailsTask(t *testing.T) {
	h := newIndexTestHarness(indexTestEmbeddingKB())
	h.engine.batchIndexErr = errors.New("vector store down")

	err := h.svc.Handle(context.Background(), newImageMultimodalIndexTask(t))
	if err == nil {
		t.Fatal("BatchIndex failure must fail the task so asynq retries")
	}
	for _, want := range []string{"index multimodal chunks", "batch index", "vector store down", "2 chunks"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error should mention %q (stage + chunk count): %v", want, err)
		}
	}
	if h.engine.batchIndexCalls != 1 {
		t.Fatalf("engine BatchIndex should be attempted exactly once, got %d", h.engine.batchIndexCalls)
	}
	if len(h.chunkRepo.created) != 2 {
		t.Fatalf("OCR+Caption chunks should be persisted before indexing, got %d", len(h.chunkRepo.created))
	}
	// 与同文件 CreateChunks 失败的既有形态一致：上抛错误、不做已写分块清理。
	if len(h.chunkRepo.deleted) != 0 {
		t.Fatalf("no chunk cleanup expected (consistent with CreateChunks failure posture), deleted=%v", h.chunkRepo.deleted)
	}
	if len(h.trace.endOutputs) != 0 {
		t.Fatalf("EndSpan must not run for retryable failure, got %d outputs", len(h.trace.endOutputs))
	}
	assertNoIndexedTrueInTrace(t, h.trace)
}

func TestImageMultimodalHandleIndexKBLookupFailureFailsTask(t *testing.T) {
	h := newIndexTestHarness(indexTestEmbeddingKB())
	h.kbSvc.err = errors.New("kb db down")
	h.kbSvc.failAfter = 2 // 第 3 次调用（indexChunks 内）失败

	err := h.svc.Handle(context.Background(), newImageMultimodalIndexTask(t))
	if err == nil {
		t.Fatal("KB lookup failure at index stage must fail the task")
	}
	for _, want := range []string{"index multimodal chunks", "get knowledge base", "kb db down", "2 chunks"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error should mention %q: %v", want, err)
		}
	}
	if h.engine.batchIndexCalls != 0 {
		t.Fatalf("engine must not be touched when KB lookup fails, got %d calls", h.engine.batchIndexCalls)
	}
	assertNoIndexedTrueInTrace(t, h.trace)
}

func TestImageMultimodalHandleIndexEmbeddingModelFailureFailsTask(t *testing.T) {
	h := newIndexTestHarness(indexTestEmbeddingKB())
	h.modelSvc.embeddingErr = errors.New("embedding model gone")

	err := h.svc.Handle(context.Background(), newImageMultimodalIndexTask(t))
	if err == nil {
		t.Fatal("embedding model resolution failure must fail the task")
	}
	for _, want := range []string{"index multimodal chunks", "get embedding model", "embedding model gone", "2 chunks"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error should mention %q: %v", want, err)
		}
	}
	if h.engine.batchIndexCalls != 0 {
		t.Fatalf("engine must not be touched when embedding model resolution fails, got %d calls", h.engine.batchIndexCalls)
	}
	assertNoIndexedTrueInTrace(t, h.trace)
}

// ---- 成功路径不回归 ----

func TestImageMultimodalHandleIndexSuccessMarksIndexed(t *testing.T) {
	h := newIndexTestHarness(indexTestEmbeddingKB())

	if err := h.svc.Handle(context.Background(), newImageMultimodalIndexTask(t)); err != nil {
		t.Fatalf("successful index should not fail the task: %v", err)
	}
	if h.engine.batchIndexCalls != 1 || h.engine.lastIndexed != 2 {
		t.Fatalf("engine BatchIndex should index both chunks once, calls=%d indexed=%d",
			h.engine.batchIndexCalls, h.engine.lastIndexed)
	}
	if len(h.trace.endOutputs) != 1 {
		t.Fatalf("exactly one EndSpan expected on success, got %d", len(h.trace.endOutputs))
	}
	if v, ok := h.trace.endOutputs[0]["indexed"]; !ok || v != true {
		t.Fatalf("trace output should carry indexed=true on success, got %v", h.trace.endOutputs[0])
	}
	for _, c := range h.chunkRepo.created {
		if c.Status != int(types.ChunkStatusIndexed) {
			t.Fatalf("chunk %s should be marked indexed, got status %d", c.ID, c.Status)
		}
	}
}

// 无嵌入管线的 KB（如 wiki-only）维持既有跳过语义：不触引擎、分块仍标已索引、
// 任务成功且轨迹记 indexed=true。
func TestImageMultimodalHandleSkipsIndexForNonEmbeddingKB(t *testing.T) {
	h := newIndexTestHarness(&types.KnowledgeBase{ID: "kb-1"})

	if err := h.svc.Handle(context.Background(), newImageMultimodalIndexTask(t)); err != nil {
		t.Fatalf("non-embedding KB should skip indexing and succeed: %v", err)
	}
	if h.engine.batchIndexCalls != 0 {
		t.Fatalf("engine must not be touched for non-embedding KB, got %d calls", h.engine.batchIndexCalls)
	}
	if len(h.trace.endOutputs) != 1 || h.trace.endOutputs[0]["indexed"] != true {
		t.Fatalf("skip path should still record indexed=true, got %v", h.trace.endOutputs)
	}
	for _, c := range h.chunkRepo.created {
		if c.Status != int(types.ChunkStatusIndexed) {
			t.Fatalf("chunk %s should be marked indexed on skip path, got status %d", c.ID, c.Status)
		}
	}
}
