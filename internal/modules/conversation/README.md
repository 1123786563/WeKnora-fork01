# conversation 模块（Pass A 骨架）

Session、Message、Chat pipeline、Feedback、分享、建议、Query History、临时文档（§5.3/§5.18）

- **职责**：Session、Message、chat pipeline、Feedback、分享、建议、Query History 与临时文档。
- **非职责**：Agent 执行引擎与工具调用归 Agent Runtime；不在 Conversation 内复制运行引擎。
- **所有权依据**：`docs/architecture/backend-modules.yaml`（F0）；spec §5.18 覆盖矩阵。
- **Pass B 任务**：`B-conversation`（拆分横向包遗留文件、删除别名、收敛门面）。

## 搬迁的包（move_packages → 目标）

| 现路径 | 目标路径 |
|---|---|
| `internal/application/service/chat_pipeline` | `internal/modules/conversation/chat_pipeline` |

## 横向包遗留文件（legacy_files）

共 44 个文件（明细与 Pass B 任务见 [legacy/README.md](legacy/README.md)）：

- `internal/application/repository`：6
- `internal/application/service`：20
- `internal/handler`：3
- `internal/handler/session`：15

## 集成点（F0）

- **routes**：7 项 — 见 manifest `integration_points.routes`
- **workers**：2 项 — 见 manifest `integration_points.workers`
- **lifecycle hooks**：3 项 — 见 manifest `integration_points.lifecycle_hooks`

## 验收命令

- `go test ./internal/modules/conversation/... -count=1`

导入方（import-path 修复对象，3 个）与禁改共享文件见同目录 manifest：
`docs/architecture/moves/conversation.yaml`。
