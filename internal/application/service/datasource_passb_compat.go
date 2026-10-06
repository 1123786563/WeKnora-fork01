package service

// Pass B 过渡 shim（26-datasource）：datasource_service.go 已物理迁移至
// internal/modules/datasource/service。本文件为留守宿主消费方
// （container.go:633 dig Provide、:2368 类型断言、datasource_purge_test.go
// 及其余留守 datasource 测试文件）提供 type 别名与构造包装器，并把模块侧
// cleanup seam 接到宿主现行 withKnowledgeCleanup（knowledge_delete_plan.go:22，
// K4 推迟批留守未导出）——与迁移前同包直引同一目标函数，行为零变化。
// 删除点：ib2（含 K4 补迁窗导出后的直连改写，见 Brief (b)）。

import (
	"github.com/Tencent/WeKnora/internal/datasource"
	"github.com/Tencent/WeKnora/internal/modules/datasource/service"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type DataSourceService = service.DataSourceService

// 哨兵错误 var 别名（同一变量引用，errors.Is 语义跨包不变）：T3 时点
// internal/handler/datasource.go:525/:761 仍在宿主（B2-DS.4 才迁移），
// 其哨兵消费需经本 shim 解析到模块侧同一变量；B2-DS.4 迁移后宿主
// 消费面消失，随本文件 ib2 同删。
var (
	ErrReindexDuplicateRequest = service.ErrReindexDuplicateRequest
	ErrSyncLogNotFound         = service.ErrSyncLogNotFound
)

// NewDataSourceService 与模块构造器同签名（10 参，逐参照录 :54-66 实测），dig 兼容；
// 构造后立即接线 knowledge cleanup seam（生产装配唯一入口）。断言写法镜像
// container.go:2368 既有模式（SetKnowledgeCleanup 在具体类型上，接口断言后调用）。
func NewDataSourceService(
	dsRepo interfaces.DataSourceRepository,
	syncLogRepo interfaces.SyncLogRepository,
	knowledgeService interfaces.KnowledgeService,
	kbService interfaces.KnowledgeBaseService,
	taskEnqueuer interfaces.TaskEnqueuer,
	connectorRegistry *datasource.ConnectorRegistry,
	scheduler *datasource.Scheduler,
	tenantRepo interfaces.TenantRepository,
	tagService interfaces.KnowledgeTagService,
	audit interfaces.AuditLogService,
) interfaces.DataSourceService {
	svc := service.NewDataSourceService(dsRepo, syncLogRepo, knowledgeService, kbService,
		taskEnqueuer, connectorRegistry, scheduler, tenantRepo, tagService, audit)
	if impl, ok := svc.(*service.DataSourceService); ok && impl != nil {
		impl.SetKnowledgeCleanup(withKnowledgeCleanup)
	}
	return svc
}
