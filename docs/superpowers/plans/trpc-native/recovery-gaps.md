# tRPC 原生 Agent 恢复间隙

P0 Task 3 记录当前基线，不把现有自定义 durable graph 宣称为最终原生组合。所有下列目标均受 P0-2 产品执行门约束：固定 `trpc-agent-go v1.10.0` 的 native Runner/Session 组合仍有 race，未经选定 SDK/Session 配置通过 race probe 前不得进入产品执行。

## 当前证据边界

`internal/agent/trpc/compatibility_probe.go` 的临时 SQLite 文件只证明 GraphAgent checkpoint saver 能保存并重开本地 pending writes、interrupt state 和工具 ID。`internal/agent/runtime/tool_recovery_test.go` 证明未知写入默认 `wait_user`，有持久结果时复用、可查询时先查询、只有受证实的幂等键或只读调用才重试。它们没有证明外部副作用恰好一次。

普通 `LLMAgent` 的内部循环在固定 SDK v1.10.0 中是 `LLMAgent.Run`（`agent/llmagent/llm_agent.go:1412`）→ `llmflow.Flow.Run`（`internal/flow/llmflow/llmflow.go:162`）→ `FunctionCallResponseProcessor.ProcessResponse`（`internal/flow/processor/functioncall.go:175`）→ `executeToolWithCallbacks`（同文件:1859）→ callable/streamable tool。该路径有 plugin/local `BeforeTool` 和 `AfterTool` 回调（同文件:1666、1710、1749、1797），因此能插入应用适配器；SDK 源码没有把它们与 WeKnora 的工具 journal、审批决定、业务数据库 checkpoint 或 SSE outbox 组成一个原子协议。

当前 GraphAgent 探针把 plan、approval、tool、answer 显式设为图节点，并在 approval 调用 `graph.Interrupt`；GraphAgent 的 `Run`（`agent/graphagent/graph_agent.go:103`）进入 `graph.Executor.Execute`（`graph/executor.go:264`）。Executor 在节点前后运行 callbacks（`graph/executor.go:3825`、`3895`），checkpoint saver 可存 `PutFull` 和 pending writes。这个顺序提供恢复插点，不能单独证明外部调用恰好一次。

## 六个间隙

| 间隙 | 当前行为与证据 | 目标行为 | P1 / P2 / P3 / P7 责任 | 验证方式 |
| --- | --- | --- | --- | --- |
| 1. 模型结果 → 计划提交 | 当前 durable graph 的 model node 与 `NewToolExecutor` 已可承接计划；普通 Runner 只有 response processor/callback 边界，未把模型工具调用原子写入 `agent_tool_calls`。 | 在任何外部调用前，以稳定 provider tool-call ID、工具身份、参数摘要、模型 attempt 和 fence 提交可恢复计划；恢复只读取已提交计划。 | P1 定义 run/plan 状态与幂等键契约；P2 将模型工具调用转换并校验为 journal plan；P3 将其与 fenced run/checkpoint 写入组合；P7 做 crash、并发、升级与真实 provider 验收。 | 在 plan 提交后 SIGKILL，再启动同一 run；确认仅一条计划、ID/args 不漂移，SQLite 和 PostgreSQL 均可重放。真实 provider 版本另列环境证据。 |
| 2. 审批 → 调用 | 当前 approval graph node 可 `graph.Interrupt`；runtime 已把 MCP OAuth、MCP approval、OC action 分别停车，普通 Runner callbacks 本身不持久化人类决定。 | 以 plan ID、授权范围、批准人、版本和过期时间持久化批准；仅有效批准可跨重启释放同一 dispatch，拒绝/过期不可调用。 | P1 定义审批/决策记录和审计读取；P2 实现 approval gate、wait kind 与恢复规则；P3 把 gate 放在 dispatch 前且接入图恢复；P7 验证权限、重启、并发批准和审计保留。 | SIGKILL 在批准提交后、调用前；用不同主体/过期/撤销重放，检查恰有一次合规 dispatch 和正确 `waiting_user`。 |
| 3. 调用成功 → 结果落库 | `ToolExecutor` 可把工具调用经过 durable dispatch/journal；`RecoveryAction` 对未知非幂等调用保持 `wait_user`。仍存在外部成功与本地结果记录之间的不可消除窗口。 | provider 返回后用 call ID、provider receipt/查询锚点和结果摘要持久化；若结果未知，先查询，不能查询且非幂等则等待用户。 | P1 定义结果/receipt/unknown 数据模型；P2 适配 provider query、幂等与 wait-user；P3 实施 journal transition 与 fenced 事务；P7 用真实 provider/Connector 的 kill matrix 验证。 | 在副作用后、结果写前 SIGKILL；幂等和可查询 provider 证明复用或查询，非幂等且不可查询固定为 `wait_user`，不得自动重发。 |
| 4. 结果落库 → checkpoint | 当前 checkpoint saver 能重开 pending writes；compatibility probe 在 SQLite 验证其中一笔 pending write。它未证明业务工具结果、state 与 checkpoint 事务边界完整。 | 结果、应用状态、pending write 和下一节点 checkpoint 以可验证顺序提交；恢复验证结果引用完整后才继续。 | P1 定义 checkpoint/result 引用不变量；P2 维护 graph state 和恢复校验；P3 实现 repository saver、事务与 fence；P7 运行 SQLite/PostgreSQL SIGKILL/损坏快照矩阵。 | 在结果 commit 后与 checkpoint commit 后各 kill 一次；重启验证工具不重调、pending write 顺序与结果引用一致。 |
| 5. checkpoint → Session | 现有 probe 使用 `noop.NewService()`；Task 2 的 `inmemory.NewSessionService` 仅为 SDK 探针，且 v1.10.0 race gate 失败。生产 durable path 从 run store/repository checkpoint 恢复，不应把 session 当授权或持久化来源。 | 选定并验证的 SDK/Session 配置只能作为受 scope 约束的会话投影；业务 run、tenant/fence、checkpoint 是权威来源，恢复不能跨 tenant/session 注入状态。 | P1 定义 run/session 权威边界和迁移契约；P2 连接 SDK session 映射且不扩大权限；P3 在恢复时重建上下文并校验 checkpoint scope；P7 验证 race、跨进程、租户隔离、升级和回滚。 | 先选定 SDK/Session 修复或版本并让 native race probe 通过；再执行跨进程/双租户重开测试，确认 checkpoint 与 session key 不能交叉恢复。 |
| 6. 事件落库 → 客户端发送 | `AgentRunStore.AppendEvent` 和 `Finalize` 能写有序事件；当前 `ExecuteDurableRun` 的 `emit` 在 append 失败时只记录 warning（`agent_run_graph.go:422-425`），不能作为目标投影。 | 事件先耐久化，再由可重放投影向 SSE/客户端发送；断线以 sequence 恢复，投影失败可重试且不能伪称已送达。 | P1 定义事件/outbox、cursor 与保留契约；P2 产出 tool/approval/run 事件；P3 实现原子 append/finalize、投影 worker 与重放；P7 验证断线、重复、trim/cursor-expired、客户端和跨数据库行为。 | 在 append、投影发送、客户端 ack 前后分别 kill/断线；重连从 `Last-Event-ID` 读取有序无丢失事件，append 失败必须使 run 可见地失败或可恢复，不能只告警。 |

## 明确的恢复规则

不可查询且非幂等的调用没有可信结果时必须维持 `waiting_user`，由用户决定后续处理；不得借由重启、模型重试或 callback 重入自动再发。`emit` 吞掉 `AppendEvent` 错误的当前实现只是一项 P1/P3/P7 缺口记录，不能被作为目标事件投影或客户端送达证明。
