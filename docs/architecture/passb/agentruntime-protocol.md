# Pass B Brief — B-agentruntime：协议边界（native/tRPC/OpenCode）

> Pass B 任务：`B-agentruntime`。child plan：**34-agentruntime-protocol**（B0.3
> Step 2 裁定标注，framework:71）。本 brief 覆盖 native 会话协议、tRPC durable run、
> OpenCode 本地代理协议与 recovery 验收工具的边界收敛。范围事实源：
> `docs/architecture/moves/agentruntime.yaml` 与
> `docs/architecture/integration/agentruntime.md`。

## Goal（目标）

`…/agent/native`、`…/agent/nativecontract`、`…/agent/nativeprobe`、`…/agent/trpc`、
`…/agent/opencode`、`…/agent/recoverytest(+provider)` 六个协议包收敛为模块对外协议面：
协议 wire 契约冻结，横向 host 包中的 native/archive 处理器与 tRPC 消费方拆入模块或
改为门面消费。

## Scope（范围）

**模块内已有包（边界收敛对象，B0.3 裁定归本计划/34）**：`agent/native`、
`agent/nativecontract`、`agent/nativeprobe`、`agent/trpc`、`agent/opencode`、
`agent/recoverytest`、`agent/recoverytest/provider`（package-main，嵌套结构保留）。

**迁入（legacy_files 中归协议侧的文件，B0.3 Step 2 裁定：archive 全链路独占归
34-agentruntime-protocol，engine brief 不认领）**：
- `internal/handler/session/native_archive.go`（native archive 处理器）；
- `internal/application/service/native_archive.go`（archive 传输服务）。

native recovery 与其余 native 持久化/状态文件（repository 侧 commit/events/lease/
memory/oauth/pending/schema/session/tool-journal/usage 及 service 侧
admission/oauth/pending/usage/recovery）**独占归 33-agentruntime-engine**
（见 agentruntime-engine.md scope；引擎侧审批、Run/Attempt 状态机同归 33）。

**模块外消费方（切换对象）**：`internal/application/service/agent_run_graph.go:22`、
`agent_capabilities.go:15`（`trpcagent` import；两文件均为 legacy 且属
33-agentruntime-engine，其拆分由 engine brief 承接，本计划只收敛 tRPC 消费面）、
`internal/container/craft_*.go` 的 opencode 消费（craft 模块集成侧，仅改 import 面）。

## Obligations（义务）

1. **协议契约冻结**：native/tRPC wire 格式、事件序号、lease/fence 语义、opencode
   协议 pin（`opencode/testdata/protocol-lock.json`）为外部契约，不得变更。
2. **tRPC 消费面收敛**：`agent_run_graph.go`、`agent_capabilities.go` 拆入模块后，
   application/service 层不得再 import `…/agent/trpc` 内部包。
3. **Recovery 验收保持**：`go test -race ./internal/agentruntime/agent/recoverytest
   -count=1 -v` 持续绿；PG 依赖用例维持 blocked-env skip 语义
   （`TRPC_RECOVERY_GRAPH_PROVIDER` 未设时 skip，不得改为 PASS 或删除）。
4. **路由不变**：`RegisterNativeArchiveRoutes`（router.go:365）与
   agent-run session 子路由 ×4（routes_chat.go:125-128）挂载点与路径不变。
5. **验证**：`go test ./internal/agentruntime/agent/{native,nativecontract,
   nativeprobe,trpc,opencode}/... -count=1`、recovery race、`go build ./...`、
   `make check-backend-architecture` 全绿。
