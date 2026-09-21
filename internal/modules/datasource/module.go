// Package datasource 是 WeKnora 后端模块化改造（Pass A）的 datasource 模块占位骨架。
//
// 职责（spec §5.18 / F0）：外部知识源连接、连接器注册、同步调度、取消/重试、进度与凭据元数据。
//
// 非职责：文档与索引状态归 Knowledge；同步结果经 Knowledge 摄取端口进入知识域。
//
// 搬迁清单：docs/architecture/moves/datasource.yaml（本模块唯一 scope 事实源）。
//
// 预期模块门面（façade）——本文件为零逻辑骨架，仅声明契约，不含任何实现与导入；
// 后续任务按装配契约填充：
//
//	NewModule(deps Dependencies) (*Module, error)
//	    构造模块实例；依赖经窄端口注入，不直接引用其他模块内部。
//	(m *Module) RegisterRoutes(r RouteRegistrar)
//	    注册本模块 HTTP 路由（当前 1 项入口，见 manifest integration_points.routes）。
//	(m *Module) RegisterWorkers(mux WorkerRegistrar)
//	    注册本模块 asynq/Lite 任务处理器（当前 2 项，见 integration_points.workers）。
//	(m *Module) Start(ctx context.Context) error
//	    启动后台任务（当前 2 项生命周期挂点，见 integration_points.lifecycle_hooks）。
//	(m *Module) Stop(ctx context.Context) error
//	    优雅停止 Start 启动的后台任务。
package datasource
