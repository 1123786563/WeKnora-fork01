# system 模块（Pass A 骨架）

初始化、System Setting、部署能力、系统管理、健康、housekeeping（§5.15/§5.18）

- **职责**：初始化、System Setting、部署能力、系统管理与 housekeeping。
- **非职责**：config 读取与进程启动机制归 Platform/bootstrap。
- **所有权依据**：`docs/architecture/backend-modules.yaml`（F0）；spec §5.18 覆盖矩阵。
- **Pass B 任务**：`B-system`（拆分横向包遗留文件、删除别名、收敛门面）。

## 搬迁的包（move_packages → 目标）

本模块 Pass A 无整包搬迁；代码目前位于横向包内（见 legacy 索引）。

## 横向包遗留文件（legacy_files）

共 5 个文件（明细与 Pass B 任务见 [legacy/README.md](legacy/README.md)）：

- `internal/application/repository`：1
- `internal/application/service`：1
- `internal/handler`：3

## 集成点（F0）

- **routes**：3 项 — 见 manifest `integration_points.routes`
- **workers**：0 项 — 见 manifest `integration_points.workers`
- **lifecycle hooks**：1 项 — 见 manifest `integration_points.lifecycle_hooks`

## 验收命令

- `go test ./internal/modules/system/... -count=1`

导入方（import-path 修复对象，0 个）与禁改共享文件见同目录 manifest：
`docs/architecture/moves/system.yaml`。
