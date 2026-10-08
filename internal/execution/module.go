// Package execution 是 WeKnora 后端模块化改造（Pass A）的 execution 模块占位骨架。
//
// 职责（spec §5.18 / F0）：Sandbox、Execution Target/Registration、Workspace Lease、Terminal、Browser Skill、环境变量与资源传输。
//
// 非职责：通用 blob 存储与文件服务归 Platform；Docker/E2B 等驱动放 adapters。
//
// 搬迁清单：docs/architecture/moves/execution.yaml（本模块唯一 scope 事实源）。
//
// 预期模块门面（façade）——本文件为零逻辑骨架，仅声明契约，不含任何实现与导入；
// 后续任务按装配契约填充：
//
//	NewModule(deps Dependencies) (*Module, error)
//	    构造模块实例；依赖经窄端口注入，不直接引用其他模块内部。
//	(m *Module) RegisterRoutes(r RouteRegistrar)
//	    注册本模块 HTTP 路由（当前 8 项入口，见 manifest integration_points.routes）。
//	(m *Module) RegisterWorkers(mux WorkerRegistrar)
//	    注册本模块 asynq/Lite 任务处理器（当前 0 项，见 integration_points.workers）。
//	(m *Module) Start(ctx context.Context) error
//	    启动后台任务（当前 0 项生命周期挂点，见 integration_points.lifecycle_hooks）。
//	(m *Module) Stop(ctx context.Context) error
//	    优雅停止 Start 启动的后台任务。
package execution
