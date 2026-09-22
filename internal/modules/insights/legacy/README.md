# insights — legacy 遗留文件索引（横向包内归属本模块的文件）

与 `docs/architecture/moves/insights.yaml` 的 `legacy_files` 逐条镜像（同路径、同 Pass B 任务）。
非测试 `.go` 文件；同目录 `_test.go` 随主题文件一并搬迁。

| 文件 | 导航标签 | Pass B 任务 |
|---|---|---|
| `internal/application/repository/analytics.go` | Analytics (application/repository) | `B-insights` |
| `internal/application/service/dataset.go` | Dataset (application/service) | `B-insights` |
| `internal/application/service/evaluation.go` | Evaluation (application/service) | `B-insights` |
| `internal/application/service/metric_hook.go` | Metric hook (application/service) | `B-insights` |
| `internal/handler/analytics.go` | Analytics (handler) | `B-insights` |
| `internal/handler/evaluation.go` | Evaluation (handler) | `B-insights` |
