# execution 模块（Pass A 骨架）

Sandbox、Execution Target/Registration、Workspace Lease、Terminal、Browser Skill、环境变量、资源传输（§5.8/§5.18）

- **职责**：Sandbox、Execution Target/Registration、Workspace Lease、Terminal、Browser Skill、环境变量与资源传输。
- **非职责**：通用 blob 存储与文件服务归 Platform；Docker/E2B 等驱动放 adapters。
- **所有权依据**：`docs/architecture/backend-modules.yaml`（F0）；spec §5.18 覆盖矩阵。
- **Pass B 任务**：`B-execution`（拆分横向包遗留文件、删除别名、收敛门面）。

## 搬迁的包（move_packages → 目标）

| 现路径 | 目标路径 |
|---|---|
| `internal/browserskill` | `internal/modules/execution/browserskill` |
| `internal/execution` | `internal/modules/execution` |
| `internal/sandbox` | `internal/modules/execution/sandbox` |

## 横向包遗留文件（legacy_files）

共 21 个文件（明细与 Pass B 任务见 [legacy/README.md](legacy/README.md)）：

- `internal/application/repository`：5
- `internal/application/service`：7
- `internal/handler`：6
- `internal/handler/session`：3

## 集成点（F0）

- **routes**：8 项 — 见 manifest `integration_points.routes`
- **workers**：0 项 — 见 manifest `integration_points.workers`
- **lifecycle hooks**：0 项 — 见 manifest `integration_points.lifecycle_hooks`

## 验收命令

- `go test ./internal/modules/execution/... -count=1`

导入方（import-path 修复对象，9 个）与禁改共享文件见同目录 manifest：
`docs/architecture/moves/execution.yaml`。
