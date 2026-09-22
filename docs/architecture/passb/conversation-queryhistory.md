# Pass B Brief — conversation-queryhistory（任务 B-conversation 之查询历史/审计面）

来源：`docs/architecture/moves/conversation.yaml`（scope 事实源）+ Pass A task A10 交付。
本文是 Pass B 义务拆分之一；会话面见
`docs/architecture/passb/conversation-session.md`。

## Scope（scope）

横向 host 包内归属 conversation 查询历史面的 legacy 文件（manifest `legacy_files` 子集，
共 4 个非测试文件；同目录 `_test.go` 随主题文件一并处理）：

- `internal/application/repository/query_history_export.go` — 导出作业持久化
- `internal/application/service/query_history_export.go` — Admin+ 异步 CSV 导出服务
  （列集合 `queryHistoryExportColumns`、job 状态机、FileService 落盘）
- `internal/application/service/query_history_policy.go` — 查询历史可见性/隐私策略
- `internal/handler/session/query_history_admin.go` — Admin+ 审计查询 handler

已就位：`internal/modules/conversation/chat_pipeline`（Pass A A10 整包搬迁）。
相关注册入口：`RegisterQueryHistoryAdminRoutes`（`internal/router/routes_query_history.go:16`，
挂载 router.go:378）与 worker `TypeQueryHistoryExport`（`internal/types/task.go:258`，
注册 router/task.go:326 + sync_task.go:165）。

## Goal（goal）

把查询历史导出/审计/隐私策略拆进 `internal/modules/conversation/`，使 SP13 引入的
Admin+ 异步查询历史 CSV 导出（审计 listing 平铺为 CSV）成为 conversation 模块内
自洽的纵向切片；与 session 面拆分共享模块门面
（`NewModule/RegisterRoutes/RegisterWorkers/Start/Stop`，见 module.go 契约声明）。

## Obligations（obligations）

1. **CSV 导出语义是外部契约**：表头列集合（`queryHistoryExportColumns`，
   `query_history_export.go:21`）、租户 scope 与审计 listing 等价性
   （"The export is the audit listing flattened to CSV" :100）、job 状态流转
   （enqueue→processing→done with file path）不得改变；回归必测导出端到端。
2. **审计隐私策略**：`query_history_policy.go` 的可见性判定（Admin+ 门槛、
   脱敏/授权范围）迁移时保持判定结果逐用例等价；该策略同时约束导出与在线查询，
   不得在拆分中出现双实现漂移。
3. **worker 注册不动**：`TypeQueryHistoryExport` 常量在 platform `internal/types`；
   注册点 router/task.go:326、sync_task.go:165 为禁改文件——拆分只动 handler/service/
   repository 归属，不动注册行（处理器参数 `params.QueryHistoryExport.ProcessExport`
   的类型路径若变，需在 Pass B 计划单列 router 装配评审）。
4. **删除别名**（与 session 面共用前置）：IA3 切换 `container.go:52` 后删除
   `internal/application/service/chat_pipeline/alias.go`（本面无直接引用，属
   B-conversation 共同义务，删除动作二选一执行、不得重复）。
5. **横向包纪律**：同 conversation-session.md 义务 4——拆出后 host 包不留转发声明；
   `internal/handler/session` 为多模块共享 host 包，拆分顺序需协调。
