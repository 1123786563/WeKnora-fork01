# appconnector — legacy 遗留文件索引（横向包内归属本模块的文件）

与 `docs/architecture/moves/appconnector.yaml` 的 `legacy_files` 逐条镜像（同路径、同 Pass B 任务）。
非测试 `.go` 文件；同目录 `_test.go` 随主题文件一并搬迁。

| 文件 | 导航标签 | Pass B 任务 |
|---|---|---|
| `internal/handler/app_connector.go` | App connector (handler) | `B-appconnector` |
| `internal/handler/app_connector_action.go` | App connector action (handler) | `B-appconnector` |
| `internal/handler/app_connector_connection.go` | App connector connection (handler) | `B-appconnector` |
| `internal/handler/app_connector_installation.go` | App connector installation (handler) | `B-appconnector` |
| `internal/handler/app_connector_oauth.go` | App connector oauth (handler) | `B-appconnector` |
| `internal/handler/app_connector_oc.go` | App connector oc (handler) | `B-appconnector` |
| `internal/handler/app_connector_sync.go` | App connector sync (handler) | `B-appconnector` |
