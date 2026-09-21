# craft 模块（Pass A 骨架）

开发工作区、会话/运行、Interaction、Snapshot、Artifact Version/Preview、用量、Scheduled Task（§5.7/§5.18）

- **职责**：开发工作区、Craft 会话/运行、Interaction、Snapshot、Artifact Version/Preview、用量视图与 Scheduled Task。
- **非职责**：Sandbox 与 Execution Target 归 Execution；预算与计量归 Commercial。
- **所有权依据**：`docs/architecture/backend-modules.yaml`（F0）；spec §5.18 覆盖矩阵。
- **Pass B 任务**：`B-craft`（拆分横向包遗留文件、删除别名、收敛门面）。

## 搬迁的包（move_packages → 目标）

| 现路径 | 目标路径 |
|---|---|
| `internal/craft` | `internal/modules/craft` |

## 横向包遗留文件（legacy_files）

共 30 个文件（明细与 Pass B 任务见 [legacy/README.md](legacy/README.md)）：

- `internal/application/repository`：6
- `internal/application/service`：18
- `internal/handler`：1
- `internal/handler/session`：5

## 集成点（F0）

- **routes**：6 项 — 见 manifest `integration_points.routes`
- **workers**：0 项 — 见 manifest `integration_points.workers`
- **lifecycle hooks**：9 项 — 见 manifest `integration_points.lifecycle_hooks`

## 验收命令

- `go test ./internal/modules/craft/... -count=1`

导入方（import-path 修复对象，7 个）与禁改共享文件见同目录 manifest：
`docs/architecture/moves/craft.yaml`。
