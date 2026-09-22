# identity 模块（Pass A 骨架）

认证、用户、Tenant、成员、邀请、API Key、Organization、RBAC、审计日志（§5.1/§5.18）

- **职责**：认证、用户、Tenant、成员、邀请、API Key、Organization、RBAC 与审计日志的领域规则与授权事实。
- **非职责**：HTTP 中间件归 Platform；跨域执行门归 Policy；进程装配归 bootstrap。
- **所有权依据**：`docs/architecture/backend-modules.yaml`（F0）；spec §5.18 覆盖矩阵。
- **Pass B 任务**：`B-identity`（拆分横向包遗留文件、删除别名、收敛门面）。

## 搬迁的包（move_packages → 目标）

本模块 Pass A 无整包搬迁；代码目前位于横向包内（见 legacy 索引）。

## 横向包遗留文件（legacy_files）

共 27 个文件（明细与 Pass B 任务见 [legacy/README.md](legacy/README.md)）：

- `internal/application/repository`：8
- `internal/application/service`：9
- `internal/handler`：10

## 集成点（F0）

- **routes**：5 项 — 见 manifest `integration_points.routes`
- **workers**：0 项 — 见 manifest `integration_points.workers`
- **lifecycle hooks**：2 项 — 见 manifest `integration_points.lifecycle_hooks`

## 验收命令

- `go test ./internal/modules/identity/... -count=1`

导入方（import-path 修复对象，0 个）与禁改共享文件见同目录 manifest：
`docs/architecture/moves/identity.yaml`。
