# agentruntime — legacy 遗留文件索引（横向包内归属本模块的文件）

与 `docs/architecture/moves/agentruntime.yaml` 的 `legacy_files` 逐条镜像（同路径、同 Pass B 任务）。
非测试 `.go` 文件；同目录 `_test.go` 随主题文件一并搬迁。

| 文件 | 导航标签 | Pass B 任务 |
|---|---|---|
| `internal/application/repository/agent_checkpoint.go` | Agent checkpoint (application/repository) | `B-agentruntime` |
| `internal/application/repository/agent_run.go` | Agent run (application/repository) | `B-agentruntime` |
| `internal/application/repository/agent_run_decisions.go` | Agent run decisions (application/repository) | `B-agentruntime` |
| `internal/application/repository/agent_run_events.go` | Agent run events (application/repository) | `B-agentruntime` |
| `internal/application/repository/agent_run_inputs.go` | Agent run inputs (application/repository) | `B-agentruntime` |
| `internal/application/repository/agent_run_lifecycle.go` | Agent run lifecycle (application/repository) | `B-agentruntime` |
| `internal/application/repository/agent_run_snapshot.go` | Agent run snapshot (application/repository) | `B-agentruntime` |
| `internal/application/repository/agent_run_tools.go` | Agent run tools (application/repository) | `B-agentruntime` |
| `internal/application/repository/mcp_tool_approval_repository.go` | Mcp tool approval repository (application/repository) | `B-agentruntime` |
| `internal/application/repository/memory.go` | Memory (application/repository) | `B-agentruntime` |
| `internal/application/repository/memory_extraction.go` | Memory extraction (application/repository) | `B-agentruntime` |
| `internal/application/repository/memory_lifecycle.go` | Memory lifecycle (application/repository) | `B-agentruntime` |
| `internal/application/repository/memory_vector.go` | Memory vector (application/repository) | `B-agentruntime` |
| `internal/application/repository/native_commit.go` | Native commit (application/repository) | `B-agentruntime` |
| `internal/application/repository/native_events.go` | Native events (application/repository) | `B-agentruntime` |
| `internal/application/repository/native_lease.go` | Native lease (application/repository) | `B-agentruntime` |
| `internal/application/repository/native_memory.go` | Native memory (application/repository) | `B-agentruntime` |
| `internal/application/repository/native_oauth.go` | Native oauth (application/repository) | `B-agentruntime` |
| `internal/application/repository/native_pending.go` | Native pending (application/repository) | `B-agentruntime` |
| `internal/application/repository/native_schema.go` | Native schema (application/repository) | `B-agentruntime` |
| `internal/application/repository/native_session.go` | Native session (application/repository) | `B-agentruntime` |
| `internal/application/repository/native_tool_journal.go` | Native tool journal (application/repository) | `B-agentruntime` |
| `internal/application/repository/native_usage.go` | Native usage (application/repository) | `B-agentruntime` |
| `internal/application/service/agent_capabilities.go` | Agent capabilities (application/service) | `B-agentruntime` |
| `internal/application/service/agent_history.go` | Agent history (application/service) | `B-agentruntime` |
| `internal/application/service/agent_run_decisions.go` | Agent run decisions (application/service) | `B-agentruntime` |
| `internal/application/service/agent_run_graph.go` | Agent run graph (application/service) | `B-agentruntime` |
| `internal/application/service/agent_run_lifecycle.go` | Agent run lifecycle (application/service) | `B-agentruntime` |
| `internal/application/service/agent_run_projection.go` | Agent run projection (application/service) | `B-agentruntime` |
| `internal/application/service/agent_run_resources.go` | Agent run resources (application/service) | `B-agentruntime` |
| `internal/application/service/agent_run_service.go` | Agent run service (application/service) | `B-agentruntime` |
| `internal/application/service/agent_run_worker.go` | Agent run worker (application/service) | `B-agentruntime` |
| `internal/application/service/agent_web_pages.go` | Agent web pages (application/service) | `B-agentruntime` |
| `internal/application/service/mcp_tool_approval_service.go` | Mcp tool approval service (application/service) | `B-agentruntime` |
| `internal/application/service/native_admission.go` | Native admission (application/service) | `B-agentruntime` |
| `internal/application/service/native_archive.go` | Native archive (application/service) | `B-agentruntime` |
| `internal/application/service/native_oauth.go` | Native oauth (application/service) | `B-agentruntime` |
| `internal/application/service/native_pending.go` | Native pending (application/service) | `B-agentruntime` |
| `internal/application/service/native_recovery.go` | Native recovery (application/service) | `B-agentruntime` |
| `internal/application/service/native_usage.go` | Native usage (application/service) | `B-agentruntime` |
| `internal/application/service/subagent_delegate.go` | Subagent delegate (application/service) | `B-agentruntime` |
| `internal/handler/memory.go` | Memory (handler) | `B-agentruntime` |
| `internal/handler/session/agent_run.go` | Agent run (handler/session) | `B-agentruntime` |
| `internal/handler/session/agent_stream_handler.go` | Agent stream handler (handler/session) | `B-agentruntime` |
| `internal/handler/session/native_archive.go` | Native archive (handler/session) | `B-agentruntime` |
