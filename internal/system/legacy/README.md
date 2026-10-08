# system — legacy 遗留文件索引（横向包内归属本模块的文件）

与 `docs/architecture/moves/system.yaml` 的 `legacy_files` 逐条镜像（同路径、同 Pass B 任务）。
非测试 `.go` 文件；同目录 `_test.go` 随主题文件一并搬迁。

| 文件 | 导航标签 | Pass B 任务 |
|---|---|---|
| `internal/application/repository/system_setting.go` | System setting (application/repository) | `B-system` |
| `internal/application/service/system_setting.go` | System setting (application/service) | `B-system` |
| `internal/handler/deployment_capabilities.go` | Deployment capabilities (handler) | `B-system` |
| `internal/handler/initialization.go` | Initialization (handler) | `B-system` |
| `internal/handler/system.go` | System (handler) | `B-system` |
