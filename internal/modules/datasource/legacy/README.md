# datasource — legacy 遗留文件索引（横向包内归属本模块的文件）

与 `docs/architecture/moves/datasource.yaml` 的 `legacy_files` 逐条镜像（同路径、同 Pass B 任务）。
非测试 `.go` 文件；同目录 `_test.go` 随主题文件一并搬迁。

| 文件 | 导航标签 | Pass B 任务 |
|---|---|---|
| `internal/application/repository/datasource_repo.go` | Datasource repo (application/repository) | `B-datasource` |
| `internal/application/service/datasource_service.go` | Datasource service (application/service) | `B-datasource` |
| `internal/handler/datasource.go` | Datasource (handler) | `B-datasource` |
| `internal/handler/datasource_credentials.go` | Datasource credentials (handler) | `B-datasource` |
