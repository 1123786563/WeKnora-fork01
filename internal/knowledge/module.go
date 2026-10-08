// Package knowledge 是 WeKnora 后端 knowledge 模块（Pass B B2 集成态门面）。
//
// 职责（spec §5.18 / F0）：知识库、文档、Chunk、Tag、FAQ、Wiki、知识图谱、
// 检索与语义知识索引。
//
// 门面五操作（contracts.yaml knowledge.facade:1613 冻结五操作名单）由
// b2-k-integration（20 计划 K5.1）按真实 seam 实装，声明如下（passbguard
// facade 形态契约 check.go:312-325：五操作须以 //\t 缩进显式声明、计数
// 句式须与 manifest integration_points 冻结值一致）：
//
//	NewModule(deps Dependencies) (*Module, error)
//	    构造模块实例；16 个必填装配依赖（9 worker 分发面 + 7 路由 handler 供给）
//	    缺失时返回列出全部缺失字段名的错误。
//	(m *Module) RegisterRoutes() (HandlerSet, error)
//	    交付路由块模块侧 handler 供给（当前 11 项入口，见 manifest integration_points.routes）。
//	(m *Module) RegisterWorkers(redis, lite *bootstrap.WorkerRegistry) error
//	    把任务处理器登记进 Redis/Lite 双栈注册表（当前 18 项，见 integration_points.workers）。
//	(m *Module) Start(ctx context.Context) error
//	    启动生命周期挂点（当前 1 项生命周期挂点，见 manifest integration_points.lifecycle_hooks）。
//	(m *Module) Stop(ctx context.Context) error
//	    优雅停止；当前无模块自持后台 goroutine，预留对称面。
//
// worker 面 = router/task.go 与 router/sync_task.go 的 knowledge 子集（18 类型
// 双栈同构）；生命周期面 = container.go:1058 挂接的 recoverPendingWikiTasks
// 等价入口；路由供给面 = 11 组注册函数的模块侧 handler 供给（路由体与
// rbacGuards guard 语义留驻 internal/router，ib2 按 Brief 切换形参——RBAC
// 语义与路由计数 633 零变化，conventions §8）。
package knowledge

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"

	"github.com/Tencent/WeKnora/internal/bootstrap"
	"github.com/Tencent/WeKnora/internal/handler"
	kbhandler "github.com/Tencent/WeKnora/internal/knowledge/retrieval/app/handler"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// Dependencies 是 knowledge 模块的装配依赖（窄端口注入，spec §4.3）。
// 字段类型与 router.AsynqTaskParams / router.SyncTaskParams 的 knowledge 子集
// 逐一对应：18 个 worker 的全部分发面经 interfaces 冻结端口注入，构造仍在
// dig 容器（conventions §3 集成工程师独占），门面只收已构造实例。
type Dependencies struct {
	// 18 workers 的分发面（9 个；task.go:37-63 / sync_task.go:115-131 knowledge 子集）。
	KnowledgeService     interfaces.KnowledgeService
	KnowledgeBaseService interfaces.KnowledgeBaseService
	TagService           interfaces.KnowledgeTagService
	ChunkExtractor       interfaces.TaskHandler // dig name "chunkExtractor"
	DataTableSummary     interfaces.TaskHandler // dig name "dataTableSummary"
	ImageMultimodal      interfaces.TaskHandler // dig name "imageMultimodal"
	KnowledgePostProcess interfaces.TaskHandler // dig name "knowledgePostProcess"
	KnowledgeAutoTag     interfaces.TaskHandler // dig name "knowledgeAutoTag"
	WikiIngest           interfaces.TaskHandler // dig name "wikiIngest"

	// 路由块 handler 供给面（7 个；§7.1 全表 11 项中已落位模块包的组，其余
	// 4 项为宿主推迟件：3 组 handler（RegisterKnowledgeRoutes/
	// RegisterKnowledgeBaseRoutes/RegisterKnowledgeBaseActivityRoutes）+
	// serveKBScopedFiles 文件服务面（非 handler 供给，无字段）；ib2/补迁窗后
	// 增补字段——装配面扩展，非契约变更）。
	Chunk               *handler.ChunkHandler                 // RegisterChunkRoutes（routes_knowledge.go:28）
	ChunkerDebug        gin.HandlerFunc                       // RegisterChunkerDebugRoutes（:18）；生产值 handler.PreviewChunking（chunker_debug.go:123）
	WikiPage            *handler.WikiPageHandler              // RegisterWikiPageRoutes（:309）
	FAQ                 *handler.FAQHandler                   // RegisterFAQRoutes（:143）
	Tag                 *handler.TagHandler                   // RegisterKnowledgeTagRoutes（:279）
	SemanticModelPolicy *kbhandler.SemanticModelPolicyHandler // RegisterSemanticModelPolicyRoutes（:254）
	SemanticInternal    *kbhandler.SemanticInternalHandler    // RegisterSemanticInternalRoutes（routes_infra.go:14）

	// 生命周期挂点（1 个）：container.go:1058 挂接的 recoverPendingWikiTasks
	// 等价入口（恢复语义冻结面见 K3 Brief (a) A9——fail-closed 清扫、
	// asynq.TaskID("wiki-finalize-"+scope)、MaxRetry 10、Timeout 60/30min、
	// 幂等）。生产值由集成工程师按 Brief 注入；nil 时 Start 为 no-op。
	PendingWikiRecovery func(ctx context.Context)
}

// Module 是 knowledge 模块装配门面实例。
type Module struct {
	deps Dependencies
}

// NewModule 构造模块实例；16 个必填装配依赖（9 分发面 + 7 handler 供给）缺失时
// 返回列出全部缺失字段名的错误（装配错误在注册前暴露，快速失败）。
func NewModule(deps Dependencies) (*Module, error) {
	var missing []string
	if deps.KnowledgeService == nil {
		missing = append(missing, "KnowledgeService")
	}
	if deps.KnowledgeBaseService == nil {
		missing = append(missing, "KnowledgeBaseService")
	}
	if deps.TagService == nil {
		missing = append(missing, "TagService")
	}
	if deps.ChunkExtractor == nil {
		missing = append(missing, "ChunkExtractor")
	}
	if deps.DataTableSummary == nil {
		missing = append(missing, "DataTableSummary")
	}
	if deps.ImageMultimodal == nil {
		missing = append(missing, "ImageMultimodal")
	}
	if deps.KnowledgePostProcess == nil {
		missing = append(missing, "KnowledgePostProcess")
	}
	if deps.KnowledgeAutoTag == nil {
		missing = append(missing, "KnowledgeAutoTag")
	}
	if deps.WikiIngest == nil {
		missing = append(missing, "WikiIngest")
	}
	if deps.Chunk == nil {
		missing = append(missing, "Chunk")
	}
	if deps.ChunkerDebug == nil {
		missing = append(missing, "ChunkerDebug")
	}
	if deps.WikiPage == nil {
		missing = append(missing, "WikiPage")
	}
	if deps.FAQ == nil {
		missing = append(missing, "FAQ")
	}
	if deps.Tag == nil {
		missing = append(missing, "Tag")
	}
	if deps.SemanticModelPolicy == nil {
		missing = append(missing, "SemanticModelPolicy")
	}
	if deps.SemanticInternal == nil {
		missing = append(missing, "SemanticInternal")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("knowledge module: missing assembly dependencies: %s", strings.Join(missing, ", "))
	}
	return &Module{deps: deps}, nil
}

// workerHandlers 返回 18 个任务类型的处理器映射——与 router/task.go、
// router/sync_task.go 的 knowledge 子集逐条同构（同类型同方法）。
func (m *Module) workerHandlers() map[string]func(context.Context, *asynq.Task) error {
	return map[string]func(context.Context, *asynq.Task) error{
		types.TypeChunkExtract:         m.deps.ChunkExtractor.Handle,
		types.TypeDataTableSummary:     m.deps.DataTableSummary.Handle,
		types.TypeDocumentProcess:      m.deps.KnowledgeService.ProcessDocument,
		types.TypeManualProcess:        m.deps.KnowledgeService.ProcessManualUpdate,
		types.TypeFAQImport:            m.deps.KnowledgeService.ProcessFAQImport,
		types.TypeQuestionGeneration:   m.deps.KnowledgeService.ProcessQuestionGeneration,
		types.TypeSummaryGeneration:    m.deps.KnowledgeService.ProcessSummaryGeneration,
		types.TypeKBClone:              m.deps.KnowledgeService.ProcessKBClone,
		types.TypeKnowledgeMove:        m.deps.KnowledgeService.ProcessKnowledgeMove,
		types.TypeKnowledgeListDelete:  m.deps.KnowledgeService.ProcessKnowledgeListDelete,
		types.TypeKnowledgeListReparse: m.deps.KnowledgeService.ProcessKnowledgeListReparse,
		types.TypeIndexDelete:          m.deps.TagService.ProcessIndexDelete,
		types.TypeKBDelete:             m.deps.KnowledgeBaseService.ProcessKBDelete,
		types.TypeImageMultimodal:      m.deps.ImageMultimodal.Handle,
		types.TypeKnowledgePostProcess: m.deps.KnowledgePostProcess.Handle,
		types.TypeKnowledgeAutoTag:     m.deps.KnowledgeAutoTag.Handle,
		types.TypeWikiIngest:           m.deps.WikiIngest.Handle,
		types.TypeWikiFinalize:         m.deps.WikiIngest.Handle,
	}
}

// RegisterWorkers 把 18 个任务处理器登记到 Redis 与 Lite 双栈注册表并校验
// 双栈集合一致（bootstrap.VerifyWorkerParity）。本模块是 internal/bootstrap
// 契约的首个生产消费方（该包零内部 import、方向合法，见 20 计划 §8 K5.1
// 输入段）。ib2 生产适配（Brief (b)）：WorkerSink 两枚——Redis 栈包
// *asynq.ServeMux（委托 mux.HandleFunc）、Lite 栈包 *router.SyncTaskExecutor
// （委托 RegisterHandler；二者第二参同形 func(context.Context, *asynq.Task) error）。
func (m *Module) RegisterWorkers(redis, lite *bootstrap.WorkerRegistry) error {
	handlers := m.workerHandlers()
	names := make([]string, 0, len(handlers))
	for tt := range handlers {
		names = append(names, tt)
	}
	sort.Strings(names)
	for _, tt := range names {
		h := handlers[tt]
		if err := redis.Register(tt, h); err != nil {
			return fmt.Errorf("knowledge module: register worker %q (redis): %w", tt, err)
		}
		if err := lite.Register(tt, h); err != nil {
			return fmt.Errorf("knowledge module: register worker %q (lite): %w", tt, err)
		}
	}
	return bootstrap.VerifyWorkerParity(redis, lite)
}

// HandlerSet 是 knowledge 11 组路由注册（§7.1）的模块侧 handler 供给
// （已落位 7 组；字段与 Dependencies 对应段一一对应）。
type HandlerSet struct {
	Chunk               *handler.ChunkHandler
	ChunkerDebug        gin.HandlerFunc
	WikiPage            *handler.WikiPageHandler
	FAQ                 *handler.FAQHandler
	Tag                 *handler.TagHandler
	SemanticModelPolicy *kbhandler.SemanticModelPolicyHandler
	SemanticInternal    *kbhandler.SemanticInternalHandler
}

// RegisterRoutes 交付 knowledge 路由块的模块侧 handler 供给。
// B2 实装口径：11 组注册函数留驻 internal/router（路由体耦合包私有
// rbacGuards 平台面）；ib2 按 Brief (a) 把路由块 handler 形参切到本供给
// （RBAC 语义与路由计数 633 零变化）。禁止在本包复刻路由表（双写）。
func (m *Module) RegisterRoutes() (HandlerSet, error) {
	return HandlerSet{
		Chunk:               m.deps.Chunk,
		ChunkerDebug:        m.deps.ChunkerDebug,
		WikiPage:            m.deps.WikiPage,
		FAQ:                 m.deps.FAQ,
		Tag:                 m.deps.Tag,
		SemanticModelPolicy: m.deps.SemanticModelPolicy,
		SemanticInternal:    m.deps.SemanticInternal,
	}, nil
}

// Start 启动模块生命周期挂点。当前唯一挂点=PendingWikiRecovery
// （container.go:1058 的模块侧等价入口）；nil 时 no-op。
// ib2 切换（Brief (c)）：原挂接行改经本门面（单一注册点，spec §4.3）；
// 过渡期双重触发无害（恢复函数幂等，recover_pending_wiki_tasks.go:27-30）。
func (m *Module) Start(ctx context.Context) error {
	if m.deps.PendingWikiRecovery != nil {
		m.deps.PendingWikiRecovery(ctx)
	}
	return nil
}

// Stop 优雅停止。知识域当前无模块自持后台 goroutine（housekeeping 调度经
// KnowledgeHousekeeping 窄端口由 42-system-policy 消费方调度，24 计划 §5.3；
// 不得出现第二套清扫实现）；预留对称面。
func (m *Module) Stop(ctx context.Context) error {
	return nil
}
