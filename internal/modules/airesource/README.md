# airesource 模块（Pass A 骨架）

模型、Provider 凭据、MCP、Web Search、Vector Store、Storage Backend、租户能力配置（§5.12/§5.18）

- **职责**：模型与 Provider 凭据、MCP 服务定义/OAuth 元数据、Web Search Provider、Vector Store、Storage Backend 与租户能力配置。
- **非职责**：MCP 运行工具位于 Agent Runtime；模型调用方不在本模块内复制凭据。
- **所有权依据**：`docs/architecture/backend-modules.yaml`（F0）；spec §5.18 覆盖矩阵。
- **Pass B 任务**：`B-airesource`（拆分横向包遗留文件、删除别名、收敛门面）。

## 搬迁的包（move_packages → 目标）

| 现路径 | 目标路径 |
|---|---|
| `internal/infrastructure/web_search` | `internal/modules/airesource/web_search` |
| `internal/mcp` | `internal/modules/airesource/mcp` |
| `internal/models/asr` | `internal/modules/airesource/models/asr` |
| `internal/models/chat` | `internal/modules/airesource/models/chat` |
| `internal/models/embedding` | `internal/modules/airesource/models/embedding` |
| `internal/models/limiter` | `internal/modules/airesource/models/limiter` |
| `internal/models/provider` | `internal/modules/airesource/models/provider` |
| `internal/models/rerank` | `internal/modules/airesource/models/rerank` |
| `internal/models/utils` | `internal/modules/airesource/models/utils` |
| `internal/models/utils/ollama` | `internal/modules/airesource/models/utils/ollama` |
| `internal/models/vlm` | `internal/modules/airesource/models/vlm` |
| `internal/storageurl` | `internal/modules/airesource/storageurl` |

## 横向包遗留文件（legacy_files）

共 33 个文件（明细与 Pass B 任务见 [legacy/README.md](legacy/README.md)）：

- `internal/application/repository`：9
- `internal/application/service`：11
- `internal/handler`：13

## 集成点（F0）

- **routes**：7 项 — 见 manifest `integration_points.routes`
- **workers**：0 项 — 见 manifest `integration_points.workers`
- **lifecycle hooks**：3 项 — 见 manifest `integration_points.lifecycle_hooks`

## 验收命令

- `go test ./internal/modules/airesource/... -count=1`

导入方（import-path 修复对象，23 个）与禁改共享文件见同目录 manifest：
`docs/architecture/moves/airesource.yaml`。
