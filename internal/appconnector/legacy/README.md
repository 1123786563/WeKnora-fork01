# appconnector — legacy 遗留文件索引（横向包内归属本模块的文件）

与 `docs/architecture/moves/appconnector.yaml` 的 `legacy_files` 逐条镜像（同路径、同 Pass B 任务）。
非测试 `.go` 文件；同目录 `_test.go` 随主题文件一并搬迁。

## 迁移状态（B2-AC.2 回填，2026-09-23）

全部 7 个 legacy handler 文件已在节点 `b2-appconnector` 任务 B2-AC.2 迁出宿主包
`internal/handler/`，落位 `internal/appconnector/handler/`（ownership-matrix 冻结
destination）。迁移提交：`refactor(appconnector): move app connector handlers into module
with alias shims`（分支 `codex/passb-b2-appconnector`；commit SHA 按 Ruling
2026-09-23-LEGACY-ROW-OWNERSHIP §4 登记于 `docs/architecture/evidence/pass-a-acceptance.md`
基线变更条目与任务报告——本文件为提交内容，无法自指自身 SHA）。

| 原宿主路径 | 导航标签 | 目标包（已迁） | 随迁测试 |
|---|---|---|---|
| `internal/handler/app_connector.go` | App connector (handler) | `internal/appconnector/handler/app_connector.go`（helpers；宿主路径重写为过渡 shim，IB2 切装配后删除） | — |
| `internal/handler/app_connector_action.go` | App connector action (handler) | `internal/appconnector/handler/app_connector_action.go` | `app_connector_oc_test.go`（部分）、`app_connector_lifecycle_test.go`（部分） |
| `internal/handler/app_connector_connection.go` | App connector connection (handler) | `internal/appconnector/handler/app_connector_connection.go` | `app_connector_oauth_test.go`、`app_connector_lifecycle_test.go`（部分） |
| `internal/handler/app_connector_installation.go` | App connector installation (handler) | `internal/appconnector/handler/app_connector_installation.go` | `app_connector_lifecycle_test.go`（部分） |
| `internal/handler/app_connector_oauth.go` | App connector oauth (handler) | `internal/appconnector/handler/app_connector_oauth.go` | `app_connector_oauth_test.go` |
| `internal/handler/app_connector_oc.go` | App connector oc (handler) | `internal/appconnector/handler/app_connector_oc.go` | `app_connector_oc_test.go` |
| `internal/handler/app_connector_sync.go` | App connector sync (handler) | `internal/appconnector/handler/app_connector_sync.go` | `app_connector_lifecycle_test.go`（部分） |

治理行核销：moves/appconnector.yaml 与 ownership-matrix.yaml 中 6 条已迁行
（action/connection/installation/oauth/oc/sync）已按 Ruling
2026-09-23-LEGACY-ROW-OWNERSHIP 在迁移提交同窗删除；`app_connector.go` 行保留
（过渡 shim 仍在宿主路径，IB2 删除 shim 时一并删行）。
