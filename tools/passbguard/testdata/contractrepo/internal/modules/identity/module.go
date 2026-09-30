// Package identity 是 fixture 模块门面骨架（仅注释，B0.4 façade shape 检测样本）。
//
// 预期模块门面（façade）——零逻辑骨架，仅声明契约：
//
//	NewModule(deps Dependencies) (*Module, error)
//	    构造模块实例；依赖经窄端口注入，不直接引用其他模块内部。
//	(m *Module) RegisterRoutes(r RouteRegistrar)
//	    注册本模块 HTTP 路由（当前 1 项入口，见 manifest integration_points.routes）。
//	(m *Module) RegisterWorkers(mux WorkerRegistrar)
//	    注册本模块 asynq/Lite 任务处理器（当前 0 项，见 integration_points.workers）。
//	(m *Module) Start(ctx context.Context) error
//	    启动后台任务（当前 1 项生命周期挂点，见 integration_points.lifecycle_hooks）。
//	(m *Module) Stop(ctx context.Context) error
//	    优雅停止 Start 启动的后台任务。
package identity
