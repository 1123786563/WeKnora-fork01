# insights — Integration Brief（Pass A → IA4）

事实源：`docs/architecture/moves/insights.yaml`。本文是 Pass A worker A14 交给集成任务
IA4 的交接说明：模块搬到了哪里、剩下哪些旧路径引用、IA4 需要按什么清单切换。
行为零变更（spec Pass A 全局约束）：路径/导入之外没有任何改动。

## 1. 搬迁结果（from → to）

| 旧导入路径 | 新导入路径 |
|---|---|
| `internal/application/service/metric` | `internal/modules/insights/metric` |

1 包 / 13 文件（9 非测试 + 4 测试）`git mv` 纯改名，move commit 13/13 rename @100%、
0 插入 0 删除；包名 `package metric` 不变。evidence 见
`docs/architecture/evidence/insights.md`。

move 树零跨模块 import：metric 只依赖 stdlib + `internal/types`（platform），
不触碰 batch-a2 冻结面，也不新增任何对其他模块内部的引用。

## 2. 模块能力速览（IA4 装配需要的全部触点）

### 2.1 指标计算库（已入模块）

`internal/modules/insights/metric`：Precision / Recall / NDCG / MRR / MAP / BLEU /
ROUGE 计算器（`New*Metric` 构造器 + `BLEU*N Gram` 常量），纯函数、无 I/O、无 config、
无 worker。行为契约由包内 4 个测试文件钉住（MRR/MAP/Precision/Recall Compute）。

### 2.2 评估服务（legacy，Pass B `B-insights` 拆分）

`internal/application/service/evaluation.go`：`EvaluationService`（LLM/检索评估，
`config.Config.Conversation.{VectorThreshold,KeywordThreshold,EmbeddingTopK,MaxRounds,
RerankTopK,RerankThreshold}` 只读消费）；`dataset.go` 评估数据集构造；
`metric_hook.go` `MetricList` 聚合器 + `metricCalculators` 表（对 metric 包的唯一
生产消费点，Pass A 已翻转 import 至新路径）。

### 2.3 分析投影（legacy，read-only，Pass B `B-insights` 拆分）

`internal/application/repository/analytics.go`：`AnalyticsRepository` 四个只读聚合
（QueryTrend / ActiveUsers / ChannelSessions / AgentMessages），**仅 SELECT**
`messages`、`sessions`、`message_feedback` 三表（0 条写语句；LEFT JOIN fan-out 去重
语义由 `TestAnalyticsAggregations` 钉住）。Insights 对源表是 READ-ONLY 所有者：
不得扩大读写面（spec §5.18）。

### 2.4 路由（2 个注册入口，6 条路由）

- `RegisterAnalyticsRoutes` — internal/router/routes_analytics.go:15（挂载点
  internal/router/router.go:379）：`GET /analytics/queries`、`GET /analytics/users`、
  `GET /analytics/channels`、`GET /analytics/agents/:agent_id`；
  Admin+（JWT roles）+ `apiKeyFullAccess`（tenant-wide 聚合，scoped key 不可读）。
- `RegisterEvaluationRoutes` — internal/router/routes_infra.go:94（挂载点
  internal/router/router.go:384）：`POST /evaluation`（Admin，驱动 LLM 调用）、
  `GET /evaluation`（Viewer，读结果）；`apiKeyRunEvaluations(apiKeyFullAccess())`。

两个注册函数本体都在 bootstrap 侧 `internal/router`（platform 领地），insights 的
handler legacy 文件只提供方法；Pass A 未动。

### 2.5 workers / lifecycle hooks / 配置键

- workers：**0**（manifest `integration_points.workers: []`；task.go/sync_task.go
  零引用）。
- lifecycle hooks：**0**（manifest `integration_points.lifecycle_hooks: []`；
  container.go 零 Invoke 挂点）。
- 配置键：已搬移的 metric 包读 0 个配置；legacy 的 evaluation 只读消费 platform
  `config.Config.Conversation.*`（上表 §2.2），无 `INSIGHTS_*`/`ANALYTICS_*` 专属
  环境变量键。

## 3. 旧路径残留引用清单（IA4 切换指引）

非禁改文件的导入已在 Pass A 修复完毕（唯一 importer
`internal/application/service/metric_hook.go:8` 已翻转 + gofmt 重排该 import 块，
再 grep 零命中）。**禁改共享文件对 insights 旧路径的引用面 = 空**：

| 禁改文件 | 旧引用 |
|---|---|
| internal/router/router.go | 无（:379/:384 挂载的是 routes_*.go 内的注册函数，不引用 insights 符号） |
| internal/router/task.go | 无（0 workers） |
| internal/router/sync_task.go | 无（0 workers） |
| internal/container/container.go | 无（从未 import metric 包；EvaluationHandler/AnalyticsHandler 构造引用的是 platform `internal/handler` 横向包，非本模块搬移面） |
| go.mod / go.sum / migrations/ | 无（未触碰） |

### IA4 切换步骤（机械操作，仅 1 步）

1. 删除空壳别名包 `internal/application/service/metric/`（仅 `alias.go`，零转发面，
   moauth 先例；manifest `alias_obligations` 一对一义务，Pass B 任务 `B-insights`）。
   当前零消费者，删除后跑 `go build ./...` +
   `go test ./internal/modules/insights/... -count=1` +
   `go run ./tools/modulemove verify --all`（别名全删后 insights 清单的 alias 1:1
   校验按"已执行"语义放行）。

无 import 行需要改写（A6 可变 var 翻转先例不适用：本模块无任何禁改文件引用）。

## 4. Guard 移交（IA4）

`go run ./tools/architectureguard` 后置结果：`literal=564 apiKeyRoute=69 handle=0
total=633 | redis=23 lite=23 | hooks=58 | modules=16`，**0 violations** —— 无新增
forbidden-import，无需加例外（落盘 `/tmp/a14-guard.log`）。

## 5. 验收命令（后搬迁路径）

- `go test ./internal/modules/insights/... -count=1`（manifest test_commands）
- 直接消费方：`go test ./internal/application/service -count=1`（metric 唯一生产
  importer 所在包）、`go test ./internal/handler -run TestAnalyticsHandler -count=1`、
  `go test ./internal/application/repository -run TestAnalyticsAggregations -count=1`
- `go run ./tools/modulemove verify --module insights`（及 `--all`）
- `go build ./...`、`go vet ./internal/modules/insights/... ./internal/application/service/`
