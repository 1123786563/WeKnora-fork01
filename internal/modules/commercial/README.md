# commercial 模块（Pass A 骨架）

套餐、权益、预算、Usage、账单、支付、履约、商业网关（§5.13/§5.18）

- **职责**：套餐、权益、预算、Usage、账单、支付、履约与商业网关。
- **非职责**：各业务模块只发布用量事实或调用准入/结算接口；计量产生方归各业务所有者。
- **所有权依据**：`docs/architecture/backend-modules.yaml`（F0）；spec §5.18 覆盖矩阵。
- **Pass B 任务**：`B-commercial`（拆分横向包遗留文件、删除别名、收敛门面）。

## 搬迁的包（move_packages → 目标）

| 现路径 | 目标路径 |
|---|---|
| `internal/application/repository/commercial` | `internal/modules/commercial/repository/commercial` |
| `internal/application/service/commercial` | `internal/modules/commercial/service/commercial` |
| `internal/commercial` | `internal/modules/commercial` |
| `internal/infrastructure/commercialplatform` | `internal/modules/commercial/commercialplatform` |
| `internal/infrastructure/openmeter` | `internal/modules/commercial/openmeter` |
| `internal/payment` | `internal/modules/commercial/payment` |
| `internal/usage` | `internal/modules/commercial/usage` |

## 横向包遗留文件（legacy_files）

共 8 个文件（明细与 Pass B 任务见 [legacy/README.md](legacy/README.md)）：

- `internal/application/repository`：2
- `internal/application/service`：2
- `internal/handler`：4

## 集成点（F0）

- **routes**：2 项 — 见 manifest `integration_points.routes`
- **workers**：0 项 — 见 manifest `integration_points.workers`
- **lifecycle hooks**：6 项 — 见 manifest `integration_points.lifecycle_hooks`

## 验收命令

- `go test ./internal/modules/commercial/... -count=1`

导入方（import-path 修复对象，18 个）与禁改共享文件见同目录 manifest：
`docs/architecture/moves/commercial.yaml`。
