# execution — legacy 遗留文件索引（横向包内归属本模块的文件）

与 `docs/architecture/moves/execution.yaml` 的 `legacy_files` 逐条镜像（同路径、同 Pass B 任务）。
非测试 `.go` 文件；同目录 `_test.go` 随主题文件一并搬迁。

| 文件 | 导航标签 | Pass B 任务 |
|---|---|---|
| `internal/application/repository/execution_cleanup.go` | Execution cleanup (application/repository) | `B-execution` |
| `internal/application/repository/execution_dispatch.go` | Execution dispatch (application/repository) | `B-execution` |
| `internal/application/repository/execution_observation.go` | Execution observation (application/repository) | `B-execution` |
| `internal/application/repository/execution_target.go` | Execution target (application/repository) | `B-execution` |
| `internal/application/repository/tenant_sandbox_config.go` | Tenant sandbox config (application/repository) | `B-execution` |
| `internal/application/service/sandbox_terminal_auth.go` | Sandbox terminal auth (application/service) | `B-execution` |
| `internal/application/service/sandbox_terminal_service.go` | Sandbox terminal service (application/service) | `B-execution` |
| `internal/application/service/sandbox_terminal_ticket.go` | Sandbox terminal ticket (application/service) | `B-execution` |
| `internal/application/service/tenant_sandbox_config.go` | Tenant sandbox config (application/service) | `B-execution` |
| `internal/application/service/tenant_sandbox_resolve.go` | Tenant sandbox resolve (application/service) | `B-execution` |
| `internal/application/service/user_env.go` | User env (application/service) | `B-execution` |
| `internal/application/service/user_env_resolver.go` | User env resolver (application/service) | `B-execution` |
| `internal/handler/execution_registration.go` | Execution registration (handler) | `B-execution` |
| `internal/handler/execution_target.go` | Execution target (handler) | `B-execution` |
| `internal/handler/me_env_var.go` | Me env var (handler) | `B-execution` |
| `internal/handler/sandbox_check.go` | Sandbox check (handler) | `B-execution` |
| `internal/handler/sandbox_config.go` | Sandbox config (handler) | `B-execution` |
| `internal/handler/sandbox_skill.go` | Sandbox skill (handler) | `B-execution` |
| `internal/handler/session/browserskill.go` | Browserskill (handler/session) | `B-execution` |
| `internal/handler/session/sandbox_terminal_bridge.go` | Sandbox terminal bridge (handler/session) | `B-execution` |
| `internal/handler/session/sandbox_terminal_ws.go` | Sandbox terminal ws (handler/session) | `B-execution` |
