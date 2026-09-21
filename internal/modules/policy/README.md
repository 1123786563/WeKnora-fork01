# policy 模块（Pass A 骨架）

跨域配额、限流、存储白名单、embed/access policy、capability gate（§5.16/§5.18）

- **职责**：跨域配额、限流、存储白名单、embed/access policy 与 capability gate 的一致执行。
- **非职责**：具体业务规则由各业务所有者定义；Policy 只负责一致执行。
- **所有权依据**：`docs/architecture/backend-modules.yaml`（F0）；spec §5.18 覆盖矩阵。
- **Pass B 任务**：`B-policy`（拆分横向包遗留文件、删除别名、收敛门面）。

## 搬迁的包（move_packages → 目标）

| 现路径 | 目标路径 |
|---|---|
| `internal/application/access` | `internal/modules/policy/access` |
| `internal/embedpolicy` | `internal/modules/policy/embedpolicy` |
| `internal/ipclass` | `internal/modules/policy/ipclass` |
| `internal/ratelimit` | `internal/modules/policy/ratelimit` |
| `internal/storageallowlist` | `internal/modules/policy/storageallowlist` |

## 横向包遗留文件（legacy_files）

共 1 个文件（明细与 Pass B 任务见 [legacy/README.md](legacy/README.md)）：

- `internal/handler`：1

## 集成点（F0）

- **routes**：1 项 — 见 manifest `integration_points.routes`
- **workers**：0 项 — 见 manifest `integration_points.workers`
- **lifecycle hooks**：0 项 — 见 manifest `integration_points.lifecycle_hooks`

## 验收命令

- `go test ./internal/modules/policy/... -count=1`

导入方（import-path 修复对象，11 个）与禁改共享文件见同目录 manifest：
`docs/architecture/moves/policy.yaml`。
