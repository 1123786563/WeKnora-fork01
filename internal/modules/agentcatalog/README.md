# agentcatalog 模块（Pass A 骨架）

Agent 定义/版本、Persona、Expert、Subagent 目录、Skill、Marketplace、收藏（§5.4/§5.18）

- **职责**：Custom Agent、Agent 版本、Persona、Expert、Subagent 目录、Skill、Marketplace 与收藏的定义管理。
- **非职责**：Agent 运行态、审批与工具调用归 Agent Runtime。
- **所有权依据**：`docs/architecture/backend-modules.yaml`（F0）；spec §5.18 覆盖矩阵。
- **Pass B 任务**：`B-agentcatalog`（拆分横向包遗留文件、删除别名、收敛门面）。

## 搬迁的包（move_packages → 目标）

本模块 Pass A 无整包搬迁；代码目前位于横向包内（见 legacy 索引）。

## 横向包遗留文件（legacy_files）

共 55 个文件（明细与 Pass B 任务见 [legacy/README.md](legacy/README.md)）：

- `internal/application/repository`：11
- `internal/application/service`：31
- `internal/handler`：13

## 集成点（F0）

- **routes**：11 项 — 见 manifest `integration_points.routes`
- **workers**：0 项 — 见 manifest `integration_points.workers`
- **lifecycle hooks**：1 项 — 见 manifest `integration_points.lifecycle_hooks`

## 验收命令

- `go test ./internal/modules/agentcatalog/... -count=1`

导入方（import-path 修复对象，0 个）与禁改共享文件见同目录 manifest：
`docs/architecture/moves/agentcatalog.yaml`。
