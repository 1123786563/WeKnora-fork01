# Pass B Brief — B-agentruntime：引擎与 Run/审批边界（engine）

> Pass B 任务：`B-agentruntime`。child plan：**33-agentruntime-engine**（B0.3
> Step 2 裁定标注，framework:71）。本 brief 覆盖引擎/Run 状态机/审批/人群包（experts、
> persona、skills、subagents、token、compaction）的边界收敛。范围事实源：
> `docs/architecture/moves/agentruntime.yaml`（legacy_files、alias_obligations）与
> `docs/architecture/integration/agentruntime.md`。

## Goal（目标）

把 agentruntime 的执行引擎与其 Run/Attempt 状态机、审批闸门收敛为模块内清晰边界：
`internal/agentruntime/agent`（引擎根包）与 `…/agent/runtime`、`…/agent/approval`
不再依赖横向 host 包的内部符号；横向包中的 38 个引擎侧 agent/native 遗留文件按文件拆入模块；
删除 Pass A 遗留的 4 个零逻辑别名与 container.go 的 4 行旧 import。
（archive 双文件归 34-agentruntime-protocol，B0.3 Step 2 裁定，本 brief 不认领。）

## Scope（范围）

**迁入（legacy_files 中归引擎侧的文件）**：
- `internal/application/repository/`：agent_checkpoint.go、agent_run.go、
  agent_run_decisions.go、agent_run_events.go、agent_run_inputs.go、
  agent_run_lifecycle.go、agent_run_snapshot.go、agent_run_tools.go、
  mcp_tool_approval_repository.go、native_commit.go、native_events.go、native_lease.go、
  native_memory.go、native_oauth.go、native_pending.go、native_schema.go、
  native_session.go、native_tool_journal.go、native_usage.go（19）
- `internal/application/service/`：agent_capabilities.go、agent_history.go、
  agent_run_decisions.go、agent_run_graph.go、agent_run_lifecycle.go、
  agent_run_projection.go、agent_run_resources.go、agent_run_service.go、
  agent_run_worker.go、agent_web_pages.go、mcp_tool_approval_service.go、
  native_admission.go、native_oauth.go、native_pending.go、
  native_recovery.go、native_usage.go、subagent_delegate.go（17）
- `internal/handler/session/`：agent_run.go、agent_stream_handler.go（2）

**模块内已有包（边界收敛对象，不重复搬迁）**：`agent`（根引擎包）、`agent/runtime`、
`agent/approval`、`agent/compaction`、`agent/experts`、`agent/persona`、`agent/skills`、
`agent/skills/skillhub`、`agent/subagents`、`agent/token`、`agent/commercial_adapter.go`
的 commercial 依赖面。

## Obligations（义务）

1. **别名删除**：完成 integration/agentruntime.md §3 的 4 行 container.go import 切换后
   `git rm` `internal/agent/{approval,experts,subagents}/alias.go` 与
   `internal/application/service/memory/alias.go`（engine brief 负责确认 approval/experts/
   subagents 三个；memory 别名由 memory brief 确认后同批删除）。
2. **文件拆分**：上述 38 个引擎侧遗留文件自横向包拆出时不得改动行为；拆出后
   `internal/application/{repository,service}`、`internal/handler/session` 中不得残留
   本模块归属文件（guard legacy-guard 为零容忍）。
   **\*Handler 去方法化（B0.3 Step 3 + IB1 裁定，DAG b3 相关节点 required_contracts）**：
   `agent_run.go`、`agent_stream_handler.go` 挂在 conversation 属主 `*Handler`
   （handler.go:24，67 字段）上的方法，拆出时必须去方法化——自带 receiver/独立构造，
   不得继续给共享 `*Handler` 增方法；确无法当场消除的残差登记 Integration Brief
   推迟 IB1 收口，禁止遗留为 conversation handler 的常驻方法。
3. **跨模块耦合收敛**：删除 `agent/commercial_adapter.go` → `modules/commercial` 与
   `agent/tools/app_connector.go` → `modules/appconnector` 两条预存 import
   （integration §10 未豁免清单），改为模块根门面或回调注入。
4. **状态机外部契约不变**：Run/Attempt 生命周期、审批 Gate 语义、fence/lease 协议
   （`agent/runtime`）为外部契约，签名冻结；引擎入口 `Engine.Execute` 调用方
   （`internal/container/agent_runtime.go`）经模块根或 runtime 包公共面消费。
5. **验证**：`go test ./internal/agentruntime/... -count=1`、
   `go test -race ./internal/agentruntime/agent/recoverytest -count=1`、
   `go build ./...`、`make check-backend-architecture` 全绿；route/worker/hook 计数
   与 F0 基线一致（633/23+23/58）。
