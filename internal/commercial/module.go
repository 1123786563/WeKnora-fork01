// Package commercial 是 WeKnora 后端模块化改造（Pass A）的 commercial 模块占位骨架。
//
// 职责（spec §5.18 / F0）：套餐、权益、预算、Usage、账单、支付、履约与商业网关。
//
// 非职责：各业务模块只发布用量事实或调用准入/结算接口；计量产生方归各业务所有者。
//
// 搬迁清单：docs/architecture/moves/commercial.yaml（本模块唯一 scope 事实源）。
//
// 预期模块门面（façade）——本文件为零逻辑骨架，仅声明契约，不含任何实现与导入；
// 后续任务按装配契约填充：
//
//	NewModule(deps Dependencies) (*Module, error)
//	    构造模块实例；依赖经窄端口注入，不直接引用其他模块内部。
//	(m *Module) RegisterRoutes(r RouteRegistrar)
//	    注册本模块 HTTP 路由（当前 2 项入口，见 manifest integration_points.routes）。
//	(m *Module) RegisterWorkers(mux WorkerRegistrar)
//	    注册本模块 asynq/Lite 任务处理器（当前 0 项，见 integration_points.workers）。
//	(m *Module) Start(ctx context.Context) error
//	    启动后台任务（当前 6 项生命周期挂点，见 integration_points.lifecycle_hooks）。
//	(m *Module) Stop(ctx context.Context) error
//	    优雅停止 Start 启动的后台任务。
package commercial
