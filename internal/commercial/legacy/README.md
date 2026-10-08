# commercial — legacy 遗留文件索引（横向包内归属本模块的文件）

与 `docs/architecture/moves/commercial.yaml` 的 `legacy_files` 逐条镜像（同路径、同 Pass B 任务）。
非测试 `.go` 文件；同目录 `_test.go` 随主题文件一并搬迁。

| 文件 | 导航标签 | Pass B 任务 |
|---|---|---|
| `internal/application/repository/model_usage.go` | Model usage (application/repository) | `B-commercial` |
| `internal/application/repository/user_usage.go` | User usage (application/repository) | `B-commercial` |
| `internal/application/service/semantic_model_budget.go` | Semantic model budget (application/service) | `B-commercial` |
| `internal/application/service/usage_recorder.go` | Usage recorder (application/service) | `B-commercial` |
| `internal/handler/commercial.go` | Commercial (handler) | `B-commercial` |
| `internal/handler/commercial_task_budget.go` | Commercial task budget (handler) | `B-commercial` |
| `internal/handler/payment_callbacks.go` | Payment callbacks (handler) | `B-commercial` |
| `internal/handler/usage.go` | Usage (handler) | `B-commercial` |
