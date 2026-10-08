// Package identity 是 WeKnora 后端模块化改造（Pass A）的 identity 模块占位骨架。
//
// 职责（spec §5.18 / F0）：认证、用户、Tenant、成员、邀请、API Key、Organization、RBAC 与审计日志的领域规则与授权事实。
//
// 非职责：HTTP 中间件归 Platform；跨域执行门归 Policy；进程装配归 bootstrap。
//
// 搬迁清单：docs/architecture/moves/identity.yaml（本模块唯一 scope 事实源）。
//
// 预期模块门面（façade）——本文件为零逻辑骨架，仅声明契约，不含任何实现与导入；
// 后续任务按装配契约填充：
//
//	NewModule(deps Dependencies) (*Module, error)
//	    构造模块实例；依赖经窄端口注入，不直接引用其他模块内部。
//	(m *Module) RegisterRoutes(r RouteRegistrar)
//	    注册本模块 HTTP 路由（当前 5 项入口，见 manifest integration_points.routes）。
//	(m *Module) RegisterWorkers(mux WorkerRegistrar)
//	    注册本模块 asynq/Lite 任务处理器（当前 0 项，见 integration_points.workers）。
//	(m *Module) Start(ctx context.Context) error
//	    启动后台任务（当前 2 项生命周期挂点，见 integration_points.lifecycle_hooks）。
//	(m *Module) Stop(ctx context.Context) error
//	    优雅停止 Start 启动的后台任务。
package identity
