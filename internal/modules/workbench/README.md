# workbench 模块（Pass A 骨架）

Task 视图、Timeline、Inbox/Attention、通用 Artifact、通知、移动任务投影（§5.6/§5.18）

- **职责**：Task 视图、Timeline、Inbox/Attention、通用 Artifact 汇总、通知与移动任务投影。
- **非职责**：Craft 专用源记录归 Craft；通用文件/HMAC grant 机制归 Platform。
- **所有权依据**：`docs/architecture/backend-modules.yaml`（F0）；spec §5.18 覆盖矩阵。
- **Pass B 任务**：`B-workbench`（拆分横向包遗留文件、删除别名、收敛门面）。

## 搬迁的包（move_packages → 目标）

| 现路径 | 目标路径 |
|---|---|
| `internal/application/service/workbench` | `internal/modules/workbench/service/workbench` |
| `internal/notification` | `internal/modules/workbench/notification` |
| `internal/voice` | `internal/modules/workbench/voice` |
| `internal/workbench` | `internal/modules/workbench` |

## 横向包遗留文件（legacy_files）

共 19 个文件（明细与 Pass B 任务见 [legacy/README.md](legacy/README.md)）：

- `internal/application/repository`：7
- `internal/handler`：3
- `internal/handler/session`：9

## 集成点（F0）

- **routes**：10 项 — 见 manifest `integration_points.routes`
- **workers**：0 项 — 见 manifest `integration_points.workers`
- **lifecycle hooks**：2 项 — 见 manifest `integration_points.lifecycle_hooks`

## 验收命令

- `go test ./internal/modules/workbench/... -count=1`

导入方（import-path 修复对象，10 个）与禁改共享文件见同目录 manifest：
`docs/architecture/moves/workbench.yaml`。
