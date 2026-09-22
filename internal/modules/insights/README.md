# insights 模块（Pass A 骨架）

Analytics、Evaluation、运营指标、报表、可重建分析投影（§5.14/§5.18）

- **职责**：Analytics、Evaluation、运营指标、报表与可重建分析投影。
- **非职责**：源记录归产生事实的业务模块；Insights 只拥有投影与报表。
- **所有权依据**：`docs/architecture/backend-modules.yaml`（F0）；spec §5.18 覆盖矩阵。
- **Pass B 任务**：`B-insights`（拆分横向包遗留文件、删除别名、收敛门面）。

## 搬迁的包（move_packages → 目标）

| 现路径 | 目标路径 |
|---|---|
| `internal/application/service/metric` | `internal/modules/insights/metric` |

## 横向包遗留文件（legacy_files）

共 6 个文件（明细与 Pass B 任务见 [legacy/README.md](legacy/README.md)）：

- `internal/application/repository`：1
- `internal/application/service`：3
- `internal/handler`：2

## 集成点（F0）

- **routes**：2 项 — 见 manifest `integration_points.routes`
- **workers**：0 项 — 见 manifest `integration_points.workers`
- **lifecycle hooks**：0 项 — 见 manifest `integration_points.lifecycle_hooks`

## 验收命令

- `go test ./internal/modules/insights/... -count=1`

导入方（import-path 修复对象，1 个）与禁改共享文件见同目录 manifest：
`docs/architecture/moves/insights.yaml`。
