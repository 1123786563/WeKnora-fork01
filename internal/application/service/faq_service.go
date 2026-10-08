package service
// FAQ 域承载（原 internal/modules/knowledge/faq 包文档；随上游对齐 round 2 归位 service 包）：承载知识域 FAQ 子域（Pass B 23-knowledge-wikifaq K3.2，
// 迁自横向宿主包 internal/application/service 的 5 个 service 文件与
// internal/handler/faq.go）。
//
// 本文件是模块侧 F0 声明（§6.1，非 ib2 删除物）：faq.Service 接收者结构体
// + NewService 构造器 + 对仍驻宿主 service 包（K2/K4 属主、ib2 前未迁出）
// 未导出符号的 seam 端口。字段面照录宿主 knowledgeService（knowledge.go:49-88）
// 中 FAQ 五文件实际消费的 17 个依赖字段（K3.2 撰写时 s.X 使用面全量 grep）；
// 本包禁止 import 宿主 internal/application/service（防与宿主委托文件 D1
// 反向成环，spec §4.2 接口优先定义在使用方）。生产接线由宿主侧
// knowledge_faq_k3_delegate.go 完成（seam 闭包捕获宿主 knowledgeService 实例）。


import (
	"context"
	"sync"

	"github.com/Tencent/WeKnora/internal/application/service/retriever"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/redis/go-redis/v9"
)

// faqImportBatchSize 真源在本包 knowledge.go:98（K4 属主留驻声明）；
// 随迁副本已随包合并删除。

// KBShareLookup 结构性镜像宿主 access.KBShareLookup
// （internal/policy/access knowledgebase.go:74，单方法）。faq 代码
// 只把 s.kbShareService（interfaces.KBShareService）原样传入 seam；宿主
// 接线侧把本接口值传给宿主 resolveKBReadTenant 的 access.KBShareLookup 形参，
// 方法集满足，行为等价（wiki 包 SpanTracker 窄端口同型，spec §4.2）。
type KBShareLookup interface {
	CheckTenantKBPermission(context.Context, string, uint64, types.TenantRole) (types.OrgMemberRole, bool, error)
}

// FAQSeams 汇集 faq 域对宿主 service 包（K2 属主 kb_activity.go /
// knowledgebase_access.go）未导出符号的依赖端口。签名逐条照录宿主定义；
// 由宿主委托文件 D1 以闭包接到宿主现行实现（与迁移前同包直引同一目标
// 函数/方法）。零值仅允许出现在不触达 seam 调用点的测试构造中。
type FAQSeams struct {
	// RecordKBActivity 照录宿主 recordKBActivity（kb_activity.go:95）：
	// best-effort 追加一条有界活动摘要。
	RecordKBActivity func(
		ctx context.Context,
		audit interfaces.AuditLogService,
		tenantID uint64,
		kbID string,
		action types.AuditAction,
		targetType string,
		targetID string,
		outcome types.AuditOutcome,
		details map[string]any,
	)

	// KBActivityTrigger 照录宿主 kbActivityTrigger（kb_activity.go:36）。
	KBActivityTrigger func(ctx context.Context) string

	// WithKBActivityTask 照录宿主 withKBActivityTask（kb_activity.go:27）。
	WithKBActivityTask func(ctx context.Context, taskID, trigger string) context.Context

	// KBActivityAppendSampleTitles 照录宿主 kbActivityAppendSampleTitles
	//（kb_activity.go:49）。
	KBActivityAppendSampleTitles func(details map[string]any, titles ...string)

	// ResolveKBReadTenant 照录宿主 resolveKBReadTenant
	//（knowledgebase_access.go:21；shares 形参以本包 KBShareLookup 镜像）。
	ResolveKBReadTenant func(ctx context.Context, kb *types.KnowledgeBase, shares KBShareLookup) (uint64, error)

	// WritableFAQKnowledgeBase 照录宿主 *knowledgeService 方法
	// writableFAQKnowledgeBase（knowledgebase_access.go:38）：validate +
	// requireKBWrite + withKBWriteTenantInfo 三段。生产接线闭包捕获宿主
	// knowledgeService 实例；链路终止于本包 ValidateFAQKnowledgeBase
	//（导出后经 D1(b) 委托进入），无递归。
	WritableFAQKnowledgeBase func(ctx context.Context, kbID string) (*types.KnowledgeBase, context.Context, error)
}

// Deps 是 NewService 的依赖面。字段与 FAQSeams 一并由宿主 D1（生产）或
// 测试构造注入；sync.Map 双字段必须传指针以共享状态（FAQ 导入进度与
// 运行中导入登记跨调用不丢）。
type Deps struct {
	Repo                interfaces.KnowledgeRepository
	ChunkRepo           interfaces.ChunkRepository
	ChunkService        interfaces.ChunkService
	TagRepo             interfaces.KnowledgeTagRepository
	TagService          interfaces.KnowledgeTagService
	TenantRepo          interfaces.TenantRepository
	KBService           interfaces.KnowledgeBaseService
	KBShareService      interfaces.KBShareService
	FileSvc             interfaces.FileService
	ModelService        interfaces.ModelService
	Task                interfaces.TaskEnqueuer
	Audit               interfaces.AuditLogService
	RedisClient         *redis.Client
	RetrieveEngine      interfaces.RetrieveEngineRegistry
	Ownership           retriever.TenantStoreOwnership
	MemFAQProgress      *sync.Map
	MemFAQRunningImport *sync.Map
	Seams                  FAQSeams
}

// Service 是 FAQ 域服务接收者：迁入五文件的全部 (s *Service) 方法承载于
// 本结构体。字段语义与宿主 knowledgeService 对应字段一致（零行为变化）。
type Service struct {
	repo           interfaces.KnowledgeRepository
	chunkRepo      interfaces.ChunkRepository
	chunkService   interfaces.ChunkService
	tagRepo        interfaces.KnowledgeTagRepository
	tagService     interfaces.KnowledgeTagService
	tenantRepo     interfaces.TenantRepository
	kbService      interfaces.KnowledgeBaseService
	kbShareService interfaces.KBShareService
	fileSvc        interfaces.FileService
	modelService   interfaces.ModelService
	task           interfaces.TaskEnqueuer
	audit          interfaces.AuditLogService
	redisClient    *redis.Client
	retrieveEngine interfaces.RetrieveEngineRegistry
	ownership      retriever.TenantStoreOwnership

	// memFAQProgress / memFAQRunningImport 以指针共享宿主
	// knowledgeService 的两个内存回落表（Lite 模式无 Redis 时的任务进度
	// 与运行中导入登记）——宿主每次 faqSvc() 重构造本包 Service 时状态
	// 不丢（计划 §6.2 D1(c)）。
	memFAQProgress      *sync.Map
	memFAQRunningImport *sync.Map

	seams FAQSeams
}

// NewService 构造 FAQ 域服务。nil 的内存回落表默认换成新表（对齐迁移前
// 宿主值字段语义：零值 knowledgeService 的 sync.Map 恒可用）；seams 不做
// 默认回落——生产路径恒经宿主 D1 全量接线，未接线仅允许出现在不触达
// seam 调用点的测试构造中。
func NewService(deps Deps) *Service {
	svc := &Service{
		repo:                deps.Repo,
		chunkRepo:           deps.ChunkRepo,
		chunkService:        deps.ChunkService,
		tagRepo:             deps.TagRepo,
		tagService:          deps.TagService,
		tenantRepo:          deps.TenantRepo,
		kbService:           deps.KBService,
		kbShareService:      deps.KBShareService,
		fileSvc:             deps.FileSvc,
		modelService:        deps.ModelService,
		task:                deps.Task,
		audit:               deps.Audit,
		redisClient:         deps.RedisClient,
		retrieveEngine:      deps.RetrieveEngine,
		ownership:           deps.Ownership,
		memFAQProgress:      deps.MemFAQProgress,
		memFAQRunningImport: deps.MemFAQRunningImport,
		seams:                  deps.Seams,
	}
	if svc.memFAQProgress == nil {
		svc.memFAQProgress = &sync.Map{}
	}
	if svc.memFAQRunningImport == nil {
		svc.memFAQRunningImport = &sync.Map{}
	}
	return svc
}
