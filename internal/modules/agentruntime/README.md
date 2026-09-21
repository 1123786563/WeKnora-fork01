# agentruntime 模块（Pass A 骨架）

Agent 执行引擎、Run/Attempt、工具、审批、Compaction、Memory、Model Context、native/tRPC/OpenCode 协议（§5.5/§5.18）

- **职责**：Agent 执行引擎、Run/Attempt 生命周期、工具调用、运行审批、Compaction、Memory 与 Model Context，native/tRPC/OpenCode 协议。
- **非职责**：Agent 定义读取归 Agent Catalog；MCP 服务定义与凭据归 AI Resource。
- **所有权依据**：`docs/architecture/backend-modules.yaml`（F0）；spec §5.18 覆盖矩阵。
- **Pass B 任务**：`B-agentruntime`（拆分横向包遗留文件、删除别名、收敛门面）。

## 搬迁的包（move_packages → 目标）

| 现路径 | 目标路径 |
|---|---|
| `internal/agent` | `internal/modules/agentruntime/agent` |
| `internal/agent/approval` | `internal/modules/agentruntime/agent/approval` |
| `internal/agent/compaction` | `internal/modules/agentruntime/agent/compaction` |
| `internal/agent/experts` | `internal/modules/agentruntime/agent/experts` |
| `internal/agent/native` | `internal/modules/agentruntime/agent/native` |
| `internal/agent/nativecontract` | `internal/modules/agentruntime/agent/nativecontract` |
| `internal/agent/nativeprobe` | `internal/modules/agentruntime/agent/nativeprobe` |
| `internal/agent/opencode` | `internal/modules/agentruntime/agent/opencode` |
| `internal/agent/persona` | `internal/modules/agentruntime/agent/persona` |
| `internal/agent/recoverytest` | `internal/modules/agentruntime/agent/recoverytest` |
| `internal/agent/recoverytest/provider` | `internal/modules/agentruntime/agent/provider` |
| `internal/agent/runtime` | `internal/modules/agentruntime/agent/runtime` |
| `internal/agent/skills` | `internal/modules/agentruntime/agent/skills` |
| `internal/agent/skills/skillhub` | `internal/modules/agentruntime/agent/skillhub` |
| `internal/agent/subagents` | `internal/modules/agentruntime/agent/subagents` |
| `internal/agent/token` | `internal/modules/agentruntime/agent/token` |
| `internal/agent/tools` | `internal/modules/agentruntime/agent/tools` |
| `internal/agent/trpc` | `internal/modules/agentruntime/agent/trpc` |
| `internal/application/service/memory` | `internal/modules/agentruntime/memory` |
| `internal/modelcontext` | `internal/modules/agentruntime/modelcontext` |

## 横向包遗留文件（legacy_files）

共 45 个文件（明细与 Pass B 任务见 [legacy/README.md](legacy/README.md)）：

- `internal/application/repository`：23
- `internal/application/service`：18
- `internal/handler`：1
- `internal/handler/session`：3

## 集成点（F0）

- **routes**：3 项 — 见 manifest `integration_points.routes`
- **workers**：1 项 — 见 manifest `integration_points.workers`
- **lifecycle hooks**：2 项 — 见 manifest `integration_points.lifecycle_hooks`

## 验收命令

- `go test ./internal/modules/agentruntime/... -count=1`
- `go test -race ./internal/modules/agentruntime/agent/recoverytest -count=1 -v  # mirrors .github/workflows/agent-recovery.yml`

导入方（import-path 修复对象，21 个）与禁改共享文件见同目录 manifest：
`docs/architecture/moves/agentruntime.yaml`。
