# Integration Brief — ib2（IB2 核心能力集成 barrier，集成侧）

> 提交方：ib2 barrier 执行（Pass B 总集成工程师，2026-09-27）。
> 消费方：B3 面各子计划（b3-r-* / b3-conv-* / b3-channels / b3-insights）与 ib3。
> 事实源：四入边子分支 Integration Brief（b2-k-integration.md 含 K1-K4 四 Brief 汇总、b2-ac-market.md、b2-datasource.md、b2-appconnector.md）+ 本屏障 evidence（docs/architecture/evidence/passb/ib2.md）。

## B3 可消费门面状态（IB2 产出）

- **knowledge**：`internal/knowledge/module.go` 五操作门面（NewModule/RegisterRoutes/RegisterWorkers/Start/Stop）已在 integration 生产装配——18 worker 双栈经 `mod.RegisterWorkers`（router/workers_knowledge.go 装配点）、`recoverPendingWikiTasks` 经 `mod.Start` 单一注册点、ChunkerDebug 直引 ingest。HandlerSet 7 项供给在位；KnowledgeHandler/KnowledgeBaseHandler/AuditLogHandler 3 组为宿主推迟件（K4 #16/K2 #2 补迁后增补字段）。
- **agentcatalog**（25a+25b+25c 全量并入）：模块树 `internal/modules/agentcatalog/**` 与宿主 compat 并存；B2-AC 面 8 条 import 例外（agentcatalog→execution/sandbox）在册（exc-0132..0139）。
- **datasource**：模块树 `internal/datasource/connector/moauth/**`；3 条例外（→knowledge retrieval app / appconnector / policy access）在册（exc-0140..0142）。
- **appconnector**：模块树 `internal/modules/appconnector/**`；legacy 行仅余 shim 1 行（matrix appconnector=1）。

## B3 前置提醒

1. Chunk/WikiPage 路由形参完整切换待 identity 去方法化（rbac_lookups.go:103/:105/:130/:183 方法迁移，K1 Brief §9 四步序第 1 步）——b3-r-* 开工前应先协调 10-identity。
2. K4 推迟件（K2 §5 14 件 + K4 (f) 17 件 + 环境阻断件）窗口裁定归协调者（K5 Brief (h)）；span_tracker/housekeeping 的 env_unblock 对在生产迁移后执行。
3. IB2 属主 45 条例外删除前置=被导入包门面合法化契约任务（airesource models/policy access/agentruntime 等），非 B3 单方面可删。
4. 计数基线（B3 消费）：路由 633 / Redis 23 / Lite 23 / hooks 58 / matrix legacy 358 / manifest legacy 358 / exceptions 142 / aliases 69——三方一致复验在案（evidence §6）。
