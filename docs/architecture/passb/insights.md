# Pass B Brief — B-insights（insights 横向包遗留文件拆分 + 别名回收）

Manifest：`docs/architecture/moves/insights.yaml`（模块 insights）。
已搬入模块的 `internal/modules/insights/metric`（指标计算库，Pass A task A14）
不再动；本 brief 覆盖其 legacy_files 全集（6 文件）与别名删除义务（1 目录）。

## Scope（legacy_files 全集，6 文件）

- internal/application/repository/analytics.go
- internal/application/service/dataset.go
- internal/application/service/evaluation.go
- internal/application/service/metric_hook.go
- internal/handler/analytics.go
- internal/handler/evaluation.go

同目录 `_test.go` 随主题文件一并搬移（既有
`internal/application/repository/analytics_test.go`、`internal/handler/analytics_test.go`）。

## 边界目标

把"分析投影（analytics 只读聚合）、评估（evaluation/dataset/metric_hook）"的应用层、
handler 层从横向包 `internal/application/{repository,service}` 与 `internal/handler`
拆出，归入 `internal/modules/insights`（建议落位 `…/insights/analytics`、
`…/insights/evaluation`，与已就位的 `…/insights/metric` 同模块互联），并按
`internal/modules/insights/module.go` 骨架声明的门面契约收口
（`NewModule(deps)` / `RegisterRoutes(r)`；本模块 0 worker、0 hook）。

## 契约（外部语义，拆分不得改变）

- **Analytics 聚合语义**：QueryTrend / ActiveUsers / ChannelSessions / AgentMessages
  的按日聚合、LEFT JOIN `message_feedback` fan-out 去重（同消息双评分不重复计数）、
  soft-delete 过滤、tenant 隔离 —— 由 `TestAnalyticsAggregations`（sqlite/postgres
  双方言）与 `TestAnalyticsHandler_*`（默认 30 天窗、显式 range 透传、date-only
  end 覆盖全天、空行渲染空数组、repo 错误映射 500 等 11 例）钉住。
  路由面 `GET /analytics/{queries,users,channels,agents/:agent_id}`（Admin+ +
  `apiKeyFullAccess`）不得变更。
- **Evaluation 语义**：`POST /evaluation`（Admin，驱动 LLM/检索评估）与
  `GET /evaluation`（Viewer，读结果）；评估参数对
  `config.Config.Conversation.*` 的只读消费口径不变。
- **READ-ONLY 所有权**：insights 对源表 `messages` / `sessions` / `message_feedback`
  仅 SELECT（spec §5.18：源记录归产生事实的业务模块，Insights 只拥有投影与报表）。
  拆分时不得新增写路径或扩大读取面。
- **指标库**：`internal/modules/insights/metric` 的 `New*Metric` 构造器签名与
  Compute 行为（BLEU/ROUGE/MAP/MRR/NDCG/Precision/Recall）是 `metric_hook.go`
  `metricCalculators` 表的输入契约，包内测试钉住。

## 删除义务

- 上述 6 文件从横向包移除后，`internal/application/repository`、
  `internal/application/service`、`internal/handler` 中对应文件删除；
- **别名回收**：删除空壳别名包 `internal/application/service/metric/`（仅
  `alias.go`，零转发面；IA4 可先行删除，见
  `docs/architecture/integration/insights.md` §3 切换步骤）；
- 完成后 `go build ./...` + `go test ./internal/modules/insights/... -count=1` +
  `go run ./tools/modulemove verify --all` 全绿。
