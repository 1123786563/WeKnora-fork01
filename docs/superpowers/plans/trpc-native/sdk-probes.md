# tRPC 原生 SDK 探针

探针日期：2026-09-19。固定依赖为 `trpc.group/trpc-go/trpc-agent-go v1.10.0`（`go.mod`）；源码从本机 `go env GOMODCACHE` 的 `trpc.group/trpc-go/trpc-agent-go@v1.10.0` 读取。未使用浮动版本或本机同时存在的 v1.11.x 源码。

## 已观察到的确定性 SDK 往返

`internal/agent/nativeprobe/runner_test.go` 使用只在测试包内存在的 `scriptedModel`。`llmagent.New` 接收该 `model.Model` 和 `function.NewFunctionTool`，`runner.NewRunner` 使用 `inmemory.NewSessionService`，再经 `Runner.Run` 执行一次工具调用并在工具结果返回后得到明确的 `finished` assistant 内容。非 race 运行 `GOWORK=off go test ./internal/agent/nativeprobe -count=1 -v -timeout 45s` 通过：工具调用计数恰为一次、事件没有被静默吞错，并在读取完成后确认请求 context 没有超时。

同一探针还创建两个仅 `session.Key.AppName` 不同的键（`weknora/tenant/1` 与 `weknora/tenant/2`），并验证它们在 in-memory session service 中保有不同状态。这证明 SDK session key 空间能够表达租户范围。

## Race gate（未通过）

任务指定的 `GOWORK=off go test -race ./internal/agent/nativeprobe -count=1 -v` 在 v1.10.0 失败。race detector 报告 `session.(*Session).Clone`（`session/session.go:95`）与 `session.(*Session).UpdateUserSession`（`session/session.go:476`）并发访问同一 session。调用路径分别来自 function-call processor 的 state-delta snapshot 和 runner 的 in-memory `AppendEvent` 持久化。该结果是固定 SDK 内部代码的竞态，产品代码没有改动来掩盖它；因此 P0-2 的 race-quality gate 不能标记为通过，需由后续 SDK 升级/上游修复决策处理。

**产品执行门：** 在明确批准一个 SDK 版本和 Session-service 配置，并且 `GOWORK=off go test -race ./internal/agent/nativeprobe -count=1 -v` 无 race、以成功退出前，不得启用任何原生 Runner 产品执行任务。当前 v1.10.0 受上述 SDK 内部 race 阻断，不能满足此门。

## 未验证的主张

此结果不验证真实模型 Provider、Provider 的工具调用格式或流式/失败语义。它也不证明服务端授权、生产数据库隔离、租户身份来源、历史访问控制或跨进程持久化；session 名称隔离仅是 SDK key-space 行为。

## 固定版本接口核对

v1.10.0 的 `agent/llmagent.New` 接受名称和选项；`runner.NewRunner` 返回 `Runner`，其 `Run` 接受 `context.Context`、user ID、session ID 与 `model.Message`，并产生 event channel；`runner.WithSessionService` 接受 `session.Service`。本探针引用的 `model.Model`、`session.Key`、`session.StateMap`、`inmemory.NewSessionService` 与 function tool API 均在该固定模块中编译使用。未发现相对任务基线的接口漂移。

## P0-3：普通 Runner 与 checkpoint 图的恢复边界

普通 `LLMAgent` 的工具循环是 SDK 内部的 `LLMAgent.Run` → `llmflow.Flow.Run` → `FunctionCallResponseProcessor.ProcessResponse` → `executeToolWithCallbacks`。固定 v1.10.0 在工具实际调用前后提供 `BeforeTool`/`AfterTool` plugin 与 local callbacks；这些是应用写入计划、审批和结果的可插入点，但没有把它们同 WeKnora 的 journal、数据库事务、checkpoint 和客户端事件组成原子恢复协议。源码不能由此证明外部效果恰好一次。

GraphAgent 走 `GraphAgent.Run` → `graph.Executor.Execute`，在节点前后运行 graph callbacks，并由 checkpoint saver 存储 checkpoint 和 pending writes。`internal/agent/trpc/compatibility_probe.go` 的显式 plan → approval → tool → answer 图在 approval 使用 `graph.Interrupt`，基线已证明 SQLite 的 interrupted state、pending write 与工具 ID 能重开恢复。完整缺口、责任与环境限制见 [recovery-gaps.md](recovery-gaps.md)。

## P0-4：能力矩阵与版本选择结论

[八列能力矩阵](sdk-capabilities.tsv) 覆盖 Task 1 的全部功能 ID，并展开 9 个能力类别、26 个远程 chat Provider、本地 Ollama，以及只做 embedding/rerank 的 Jina 非 Agent 消费者。共 88 行：4 行 `verified`（仅继承 Task 2/3 各行指定的确定性/SQLite/策略证据）、77 行 `source-only`、2 行 `blocked-env`、5 行 `incompatible`。本任务只执行文档结构与 Go 契约编译检查；没有新增真实 Provider、数据库或客户端运行验收。

版本证据为根 `go.mod:99` 和 `go.sum`：v1.10.0 模块校验和 `h1:0pY2ee7tc6+3e+I7CgnkdGY2z4lrmQwQ4sq7C1dulQc=`，go.mod 校验和 `h1:lksOlht6E+LR7AKOA0XoKrkI3AJNsfukTVes9BFmTow=`。源码根固定为 `$(go env GOMODCACHE)/trpc.group/trpc-go/trpc-agent-go@v1.10.0`。本任务没有修改模块缓存、go.mod、go.sum 或产品代码，没有选用其他本机版本，也没有声称新版本已修复问题。

**选择结论：v1.10.0 仅作为本轮分析和探针基线，尚无已批准的产品目标版本。** `RUN-02` race 失败直接阻断原生 Runner 产品执行；`SESSION-04` 的 PostgreSQL/SQLite Session 实现与 Memory 持久化实现也尚未固定版本/配置。没有经同样探针验证的新 tag/commit，所以不凭新版功能文档给出升级通过结论。P0-5 应记录 no-go，并明确修复/配置复验责任；后续选候选时先固定官方 tag/commit 与所有子模块依赖，再执行 race、存储、失败和恢复探针。

### 固定源码新增发现（尚未行为验收）

- `sdk:runner/runner.go handleEventPersistence` 在 `Session.AppendEvent` 返回错误时只记录日志并返回 false；`processSingleAgentEvent` 后续仍可通知 completion/转发。因此“收到 SDK final event”不能证明业务日志、Session 和 checkpoint 已耐久提交。`interfaces.md` 定义 intent、幂等 Session apply 与推进 barrier；若 callback/plugin 组合无法保证持久化失败后零后续 dispatch，保持阻塞，不用取消竞速代替证据。
- `sdk:agent/llmagent/option.go WithSkills` 未配置显式 executor 或显式 knowledge-only/fine-grained opt-out 时会自动使用 local executor。此默认配置不符合已有 Sandbox 隔离，必须显式受控配置并覆盖 code execution response processor 等所有执行面。
- `sdk:tool/mcp/tool.go Call` 本身调用一次，但 `toolset.go callTool` 包装在 `executeWithSessionReconnect` 中；启用 reconnect 后会重新执行 operation。默认未设置 reconnect 时不启用。`WithSessionReconnect(0)` 会 clamp 至至少一次、`WithSessionReconnectConfig` 会强制 enable，不能用这些选项假装关闭重放。未知有副作用操作必须由业务恢复策略决定，不得让传输重连自动重发。
- `sdk:internal/flow/processor/functioncall.go executeToolWithCallbacks` 为 plugin BeforeTool → local BeforeTool → permission → execute → after；可插前置边界，但 callbacks 与业务 journal/approval 无天然事务。参数经过所有合法变换后仍要在真实 dispatch 处验证 hash、授权和 fence。
- `sdk:internal/flow/llmflow/llmflow.go` 在 response 序列中调用 AfterModel；不能按回调次数收费。必须绑定真实 Provider dispatch attempt，并去重 cumulative usage/重复流；模型 failover/hedge 的每次请求也属于独立成本。
- `sdk:memory/memory.go Service` 有 Tools、删除、清空、后台抽取入口；Runner 会 enqueue auto memory job。现有产品的 opt-out、撤权、删除 tombstone、topics/document affinity、confirm/reject/consolidation 需业务语义扩展，不能仅隐藏工具。SDK key 也不提供租户授权。
- `sdk:session/session.go` 提供 CRUD、history filters、summary；严格 event-page 的注释限定 postgres/mysql，不能据此声称 SQLite 实现已支持。Task 2 仍仅证明 in-memory 的 AppName 键空间。

### 旧模型特殊字段的逐项继承

矩阵 `MODEL-03` 至 `MODEL-09` 和每个 `PROVIDER-*` 分别覆盖：采样/temperature/top_p/max token aliases、seed、tool_choice、parallel_tool_calls、format/StructuredOutput、ThinkingEnabled、image URL/data/detail/format、ReasoningContent、ToolID/ToolName、thought signature/google metadata、prompt/completion/total 与 cache read/create usage、stream terminal、无效/标量/重复工具参数 delta。依据是 `internal/agent/trpc/model.go` 与全部 5 个 `model_test.go` 测试的静态阅读，没有把旧 fake 测试当新路径通过。

旧桥明确拒绝 Stop/ReasoningEffort/ThinkingTokens/ThinkingLevel/Headers/ReasoningSignature 等，不能继续包装旧桥后宣称完成原生迁移。P3 使用原生 `model.Model` 和必要 Provider 小扩展，逐项迁移并真实调用验收；非 Agent 的 chat/embedding/rerank 消费者不在无证据删除范围。

完整新 Go 接口、键映射、身份、提交协调、事件 v1、归档只读、扩展测试与删除条件见 [interfaces.md](interfaces.md)。`阻塞并修订规格` 不授权降低功能：先补版本/配置决策；若不能满足已确认边界，再提交明确规格修订，而不是静默删恢复、权限或旧能力。
