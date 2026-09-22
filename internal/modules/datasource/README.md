# datasource 模块（Pass A 骨架）

外部知识源、连接器注册、同步、取消、重试、进度、凭据元数据、调度（§5.9/§5.18）

- **职责**：外部知识源连接、连接器注册、同步调度、取消/重试、进度与凭据元数据。
- **非职责**：文档与索引状态归 Knowledge；同步结果经 Knowledge 摄取端口进入知识域。
- **所有权依据**：`docs/architecture/backend-modules.yaml`（F0）；spec §5.18 覆盖矩阵。
- **Pass B 任务**：`B-datasource`（拆分横向包遗留文件、删除别名、收敛门面）。

## 搬迁的包（move_packages → 目标）

| 现路径 | 目标路径 |
|---|---|
| `internal/datasource` | `internal/modules/datasource` |
| `internal/datasource/connector/confluence` | `internal/modules/datasource/connector/confluence` |
| `internal/datasource/connector/dingtalk` | `internal/modules/datasource/connector/dingtalk` |
| `internal/datasource/connector/feishu/core` | `internal/modules/datasource/connector/feishu/core` |
| `internal/datasource/connector/feishu/drive` | `internal/modules/datasource/connector/feishu/drive` |
| `internal/datasource/connector/feishu/wiki` | `internal/modules/datasource/connector/feishu/wiki` |
| `internal/datasource/connector/gitlab` | `internal/modules/datasource/connector/gitlab` |
| `internal/datasource/connector/ima` | `internal/modules/datasource/connector/ima` |
| `internal/datasource/connector/moauth` | `internal/modules/datasource/connector/moauth` |
| `internal/datasource/connector/notion` | `internal/modules/datasource/connector/notion` |
| `internal/datasource/connector/rss` | `internal/modules/datasource/connector/rss` |
| `internal/datasource/connector/yuque` | `internal/modules/datasource/connector/yuque` |

## 横向包遗留文件（legacy_files）

共 4 个文件（明细与 Pass B 任务见 [legacy/README.md](legacy/README.md)）：

- `internal/application/repository`：1
- `internal/application/service`：1
- `internal/handler`：2

## 集成点（F0）

- **routes**：1 项 — 见 manifest `integration_points.routes`
- **workers**：2 项 — 见 manifest `integration_points.workers`
- **lifecycle hooks**：2 项 — 见 manifest `integration_points.lifecycle_hooks`

## 验收命令

- `go test ./internal/modules/datasource/... -count=1`

导入方（import-path 修复对象，14 个）与禁改共享文件见同目录 manifest：
`docs/architecture/moves/datasource.yaml`。
