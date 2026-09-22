# appconnector 模块（Pass A 骨架）

应用目录、安装、OAuth、连接、授权、动作、同步、Open Connector（§5.10/§5.18）

- **职责**：应用目录、安装、OAuth、连接、授权、动作、同步与 Open Connector 适配/控制。
- **非职责**：Agent 工具只调用本模块公开动作接口，不读取连接表；MCP 定义归 AI Resource。
- **所有权依据**：`docs/architecture/backend-modules.yaml`（F0）；spec §5.18 覆盖矩阵。
- **Pass B 任务**：`B-appconnector`（拆分横向包遗留文件、删除别名、收敛门面）。

## 搬迁的包（move_packages → 目标）

| 现路径 | 目标路径 |
|---|---|
| `internal/appconnector` | `internal/modules/appconnector` |
| `internal/appconnector/openconnector` | `internal/modules/appconnector/openconnector` |
| `internal/application/repository/appconnector` | `internal/modules/appconnector/repository/appconnector` |
| `internal/application/service/appconnector` | `internal/modules/appconnector/service/appconnector` |
| `internal/connectorcontrol` | `internal/modules/appconnector/connectorcontrol` |

## 横向包遗留文件（legacy_files）

共 7 个文件（明细与 Pass B 任务见 [legacy/README.md](legacy/README.md)）：

- `internal/handler`：7

## 集成点（F0）

- **routes**：1 项 — 见 manifest `integration_points.routes`
- **workers**：0 项 — 见 manifest `integration_points.workers`
- **lifecycle hooks**：5 项 — 见 manifest `integration_points.lifecycle_hooks`

## 验收命令

- `go test ./internal/modules/appconnector/... -count=1`

导入方（import-path 修复对象，9 个）与禁改共享文件见同目录 manifest：
`docs/architecture/moves/appconnector.yaml`。
