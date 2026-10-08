# insights 模块（Pass A 骨架 + Pass B B3 搬迁完成）

Analytics、Evaluation、运营指标、报表、可重建分析投影（§5.14/§5.18）

- **职责**：Analytics、Evaluation、运营指标、报表与可重建分析投影。
- **非职责**：源记录归产生事实的业务模块；Insights 只拥有投影与报表。
- **所有权依据**：`docs/architecture/backend-modules.yaml`（F0）；spec §5.18 覆盖矩阵。
- **Pass B 任务**：`B-insights`（拆分横向包遗留文件、删除别名、收敛门面）——B3-IN.1~IN.6 已完成（2026-09-29）；门面实装（module.go 五操作）归 IB3。

## 搬迁的包（move_packages → 目标）

`move_packages` 唯一义务（metric）已于 Pass A 集成期物理完成，B3-IN.5 按空义务核销（manifest 现值 `[]`）；Pass B 另将 6 个横向包 legacy 文件拆为 analytics/evaluation 两包（见下节）。下表保留为迁移轨迹记录：

| 原路径 | 目标路径 | 状态 |
|---|---|---|
| `internal/application/service/metric` | `internal/insights/metric` | Pass A 已迁 |
| `internal/application/repository/analytics.go`、`internal/handler/analytics.go` | `internal/insights/analytics` | Pass B 已迁（B3-IN.3） |
| `internal/application/service/{dataset,evaluation,metric_hook}.go`、`internal/handler/evaluation.go` | `internal/insights/evaluation` | Pass B 已迁（B3-IN.4） |

## 横向包遗留文件（legacy_files）

原 6 个 legacy 文件已随 B3-IN.3/B3-IN.4 物理迁入本模块（`insights/analytics`、`insights/evaluation`），manifest 行同 commit 删除；现 manifest `legacy_files` 仅余 **3 个 Pass B 过渡 shim**（宿主 compat，ib3 装配直连切换后随文件删行，明细与迁移轨迹见 [legacy/README.md](legacy/README.md)）：

- `internal/application/repository`：1（compat）
- `internal/application/service`：1（compat，含 cleanup seam 接线）
- `internal/handler`：1（compat）

## 公开面（Pass B 搬迁后新增导出）

- `analytics` 包：`NewAnalyticsRepository(db *gorm.DB) interfaces.AnalyticsRepository`、`NewAnalyticsHandler(repo interfaces.AnalyticsRepository) *AnalyticsHandler`、`ParseAnalyticsRange(c *gin.Context) (from, to time.Time, ok bool)`、`Rows[T any](rows []T) []T`。
- `evaluation` 包：`NewDatasetService() interfaces.DatasetService`、`NewEvaluationService`（7 参：config/dataset/knowledgeBaseService/knowledgeService/sessionService/modelService + cleanup seam）、`NewEvaluationHandler(svc interfaces.EvaluationService) *EvaluationHandler`、`DeleteReferencedKnowledgeFunc`（Knowledge 删除高风险面的 cleanup seam 类型，evaluation.go:30）。
- 宿主消费方暂经 `internal/{application/repository,application/service,handler}/insights_passb_compat.go` type 别名/转发过渡，IB3 装配直连切换后删除（见 Integration Brief）。

## 集成点（F0）

- **routes**：2 项 — 见 manifest `integration_points.routes`
- **workers**：0 项 — 见 manifest `integration_points.workers`
- **lifecycle hooks**：0 项 — 见 manifest `integration_points.lifecycle_hooks`

## 验收命令

- `go test ./internal/insights/... -count=1`

导入方（importers，现为 3 个真实宿主——compat shim 所在：`internal/application/repository`、`internal/application/service`、`internal/handler`）与禁改共享文件见同目录 manifest：
`docs/architecture/moves/insights.yaml`。
