package knowledge

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"

	"github.com/Tencent/WeKnora/internal/bootstrap"
	"github.com/Tencent/WeKnora/internal/handler"
	kbhandler "github.com/Tencent/WeKnora/internal/modules/knowledge/retrieval/app/handler"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// stubTaskHandler 满足 interfaces.TaskHandler（task_handler.go:10，单方法）。
type stubTaskHandler struct{}

func (stubTaskHandler) Handle(ctx context.Context, t *asynq.Task) error { return nil }

// stubXXXService 以「嵌入 nil 接口」满足巨型冻结端口：本包测试只做非空校验、
// 方法值创建与登记（bootstrap.WorkerRegistry.Register 以 any 承载 handler、
// 不调用），永不调用服务方法。语义依据：Go 方法值创建不解引用嵌入的 nil
// 接口（2026-09-26 已以独立 go test 实证，见 20 计划 §13 自检记录）。
type (
	stubKnowledgeService     struct{ interfaces.KnowledgeService }
	stubKnowledgeBaseService struct {
		interfaces.KnowledgeBaseService
	}
	stubKnowledgeTagService struct{ interfaces.KnowledgeTagService }
)

// knowledgeWorkerTypes 是 18 个 knowledge 任务类型的期望清单
// （knowledge.yaml integration_points.workers / contracts.yaml:1893）。
var knowledgeWorkerTypes = []string{
	types.TypeChunkExtract, types.TypeDataTableSummary, types.TypeDocumentProcess,
	types.TypeManualProcess, types.TypeFAQImport, types.TypeQuestionGeneration,
	types.TypeSummaryGeneration, types.TypeKBClone, types.TypeKnowledgeMove,
	types.TypeKnowledgeListDelete, types.TypeKnowledgeListReparse, types.TypeIndexDelete,
	types.TypeKBDelete, types.TypeImageMultimodal, types.TypeKnowledgePostProcess,
	types.TypeKnowledgeAutoTag, types.TypeWikiIngest, types.TypeWikiFinalize,
}

func fullDependencies(recovery func(ctx context.Context)) Dependencies {
	return Dependencies{
		KnowledgeService:     stubKnowledgeService{},
		KnowledgeBaseService: stubKnowledgeBaseService{},
		TagService:           stubKnowledgeTagService{},
		ChunkExtractor:       stubTaskHandler{},
		DataTableSummary:     stubTaskHandler{},
		ImageMultimodal:      stubTaskHandler{},
		KnowledgePostProcess: stubTaskHandler{},
		KnowledgeAutoTag:     stubTaskHandler{},
		WikiIngest:           stubTaskHandler{},
		Chunk:                &handler.ChunkHandler{},
		ChunkerDebug:         func(c *gin.Context) {},
		WikiPage:             &handler.WikiPageHandler{},
		FAQ:                  &handler.FAQHandler{},
		Tag:                  &handler.TagHandler{},
		SemanticModelPolicy:  &kbhandler.SemanticModelPolicyHandler{},
		SemanticInternal:     &kbhandler.SemanticInternalHandler{},
		PendingWikiRecovery:  recovery,
	}
}

func TestNewModuleRejectsMissingAssemblyDependencies(t *testing.T) {
	deps := fullDependencies(nil)
	deps.KnowledgeService = nil
	deps.Chunk = nil
	_, err := NewModule(deps)
	if err == nil ||
		!strings.Contains(err.Error(), "KnowledgeService") ||
		!strings.Contains(err.Error(), "Chunk") {
		t.Fatalf("want missing-deps error naming KnowledgeService and Chunk, got %v", err)
	}
	if _, err := NewModule(fullDependencies(nil)); err != nil {
		t.Fatalf("full dependencies must construct, got %v", err)
	}
}

func TestRegisterWorkersDualStackParity(t *testing.T) {
	m, err := NewModule(fullDependencies(nil))
	if err != nil {
		t.Fatal(err)
	}
	redis := bootstrap.NewWorkerRegistry("redis", nil)
	lite := bootstrap.NewWorkerRegistry("lite", nil)
	if err := m.RegisterWorkers(redis, lite); err != nil {
		t.Fatalf("RegisterWorkers: %v", err)
	}
	if err := bootstrap.VerifyWorkerParity(redis, lite); err != nil {
		t.Fatalf("parity: %v", err)
	}
	want := append([]string(nil), knowledgeWorkerTypes...)
	sort.Strings(want)
	if got := redis.TaskTypes(); !reflect.DeepEqual(got, want) {
		t.Fatalf("registered task types drift: got %d %v, want %d", len(got), got, len(want))
	}
	// 同一模块重复登记必须被 bootstrap 拒绝（already-registered 前缀契约）。
	if err := m.RegisterWorkers(redis, lite); err == nil ||
		!strings.HasPrefix(err.Error(), "knowledge module: register worker") {
		t.Fatalf("re-registration must be rejected, got %v", err)
	}
}

func TestRegisterRoutesReturnsHandlerSupply(t *testing.T) {
	m, err := NewModule(fullDependencies(nil))
	if err != nil {
		t.Fatal(err)
	}
	set, err := m.RegisterRoutes()
	if err != nil {
		t.Fatalf("RegisterRoutes: %v", err)
	}
	if set.Chunk == nil || set.WikiPage == nil || set.FAQ == nil || set.Tag == nil ||
		set.SemanticModelPolicy == nil || set.SemanticInternal == nil || set.ChunkerDebug == nil {
		t.Fatal("handler supply must be fully populated")
	}
}

func TestStartInvokesPendingWikiRecovery(t *testing.T) {
	called := false
	m, _ := NewModule(fullDependencies(func(ctx context.Context) { called = true }))
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !called {
		t.Fatal("Start must invoke PendingWikiRecovery when wired")
	}
	mNil, _ := NewModule(fullDependencies(nil))
	if err := mNil.Start(context.Background()); err != nil {
		t.Fatalf("Start with nil recovery must be a no-op, got %v", err)
	}
}

func TestStopReturnsNil(t *testing.T) {
	m, _ := NewModule(fullDependencies(nil))
	if err := m.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}
