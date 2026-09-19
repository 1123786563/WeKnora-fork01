# tRPC 原生 SDK 探针

探针日期：2026-09-19。初始固定依赖为 `trpc.group/trpc-go/trpc-agent-go v1.10.0`；Task 1 以 `go get trpc.group/trpc-go/trpc-agent-go@v1.11.0` 和 `GOWORK=off go mod tidy` 精确升级根模块至 `v1.11.0`。未使用浮动版本或本地路径 `replace`。v1.10.0 证据保留为 RED 基线；v1.11.0 只是在本文件记录的候选，不构成产品执行许可。

## Task 1 candidate probe — v1.11.0

`internal/agent/nativeprobe/runner_test.go` 现在先创建并读取同一个 `inmemory.SessionService` 中的 Session，再经真实 `runner.Run` 工具往返读取它。除“工具恰好调用一次”和最终 `finished` 外，它断言 Runner 已把事件同步到 `Session.Events`，并且 `after.UpdatedAt` 严格晚于预建 Session 的 `before.UpdatedAt`。这覆盖真实 Runner 的已有 Session mutation，而不是只证明 Runner 创建了一个 `UpdatedAt` 非零的新 Session。

在升级前，该断言的普通运行通过，而以下 v1.10.0 RED 命令以退出码 1 失败并报告 SDK 内部 race：

```sh
GOWORK=off go test -race ./internal/agent/nativeprobe -count=20 -v
```

race 路径是 `session.(*Session).Clone` 与 `session.(*Session).UpdateUserSession` 并发访问 `UpdatedAt`，分别经 function-call state-delta snapshot 与 Runner 的 in-memory `AppendEvent` 到达；报告源码均为 `trpc-agent-go@v1.10.0`。

升级后下列命令均以退出码 0 通过：

```sh
GOWORK=off go test ./internal/agent/nativeprobe -count=1 -v -timeout 45s
GOWORK=off go test -race ./internal/agent/nativeprobe -count=20 -v
GOWORK=off go test ./internal/agent/trpc -count=1 -v
```

最后一个直接 consumer（`internal/agent/trpc`）通过。依赖清单由以下命令获得：

```sh
GOWORK=off go list -deps -f '{{with .Module}}{{if eq .Path "github.com/Tencent/WeKnora"}}{{$.ImportPath}} {{join $.Imports " "}}{{end}}{{end}}' ./... | rg 'trpc\\.group/trpc-go/trpc-agent-go'
```

它还列出 `internal/application/service` 与 `internal/agent/recoverytest/provider`。当前候选的 `GOWORK=off go test ./internal/application/service -run TestExecuteDurableRunPersistsBudgetExhaustionForNotification -count=1 -v` 以退出码 1 失败：持久事件只有 `run_started` 和 `run_failed`，没有预算耗尽通知所需记录。在临时 detached worktree 的精确 `75523c9c9` 基线运行同一命令，同样以退出码 1 和相同事件缺口失败。因此这是阻断完整直接消费者验收的**既有失败**，不是 v1.11.0 回归；在它被单独修复并回归验证前，不得将完整 consumer 验收记为通过。PostgreSQL、真实 Provider、持久 Session/Memory、append-failure barrier、恢复和客户端门禁同样保持 **NO-GO**。

## 历史 v1.10.0 基线：确定性 SDK 往返

`internal/agent/nativeprobe/runner_test.go` 使用只在测试包内存在的 `scriptedModel`。`llmagent.New` 接收该 `model.Model` 和 `function.NewFunctionTool`，`runner.NewRunner` 使用 `inmemory.NewSessionService`，再经 `Runner.Run` 执行一次工具调用并在工具结果返回后得到明确的 `finished` assistant 内容。非 race 运行 `GOWORK=off go test ./internal/agent/nativeprobe -count=1 -v -timeout 45s` 通过：工具调用计数恰为一次、事件没有被静默吞错，并在读取完成后确认请求 context 没有超时。

同一探针还创建两个仅 `session.Key.AppName` 不同的键（`weknora/tenant/1` 与 `weknora/tenant/2`），并验证它们在 in-memory session service 中保有不同状态。这证明 SDK session key 空间能够表达租户范围。

## v1.10.0 RED → v1.11.0 GREEN：重复 race gate

历史 v1.10.0 的 repeated command 以退出码 1 失败。race detector 报告 `session.(*Session).Clone`（`session/session.go:95`）与 `session.(*Session).UpdateUserSession`（`session/session.go:476`）并发访问同一 session。调用路径分别来自 function-call processor 的 state-delta snapshot 和 runner 的 in-memory `AppendEvent` 持久化。该 RED 结果来自固定 SDK 内部代码，产品代码没有改动来掩盖它。

**产品执行门：** 在明确批准一个 SDK 版本和完整 Session/Memory service 配置后，必须由受审任务执行且全部通过以下固定多次 gate，才能启用任何原生 Runner 产品执行任务：

```sh
GOWORK=off go test -race ./internal/agent/nativeprobe -count=20 -v
```

`-count=20` 要求同一已固定组合连续二十次独立测试运行均以零退出且没有 race report；一次通过不能解除门禁。当前根 `v1.11.0` 候选已以此命令退出 0。它仅将 v1.10.0 的 SDK race 从当前候选中排除，不能解除持久化、恢复、Provider、PostgreSQL、客户端或完整消费者验收门禁；产品执行仍为 **NO-GO**。

## 未验证的主张

此结果不验证真实模型 Provider、Provider 的工具调用格式或流式/失败语义。它也不证明服务端授权、生产数据库隔离、租户身份来源、历史访问控制或跨进程持久化；session 名称隔离仅是 SDK key-space 行为。

## 当前 v1.11.0 接口核对

当前 `v1.11.0` 的 `agent/llmagent.New` 接受名称和选项；`runner.NewRunner` 返回 `Runner`，其 `Run` 接受 `context.Context`、user ID、session ID 与 `model.Message`，并产生 event channel；`runner.WithSessionService` 接受 `session.Service`。本探针引用的 `model.Model`、`session.Key`、`session.StateMap`、`inmemory.NewSessionService` 与 function tool API 均在该当前根模块中编译使用；不需要产品源代码调整。

## P0-3：普通 Runner 与 checkpoint 图的恢复边界

普通 `LLMAgent` 的工具循环是 SDK 内部的 `LLMAgent.Run` → `llmflow.Flow.Run` → `FunctionCallResponseProcessor.ProcessResponse` → `executeToolWithCallbacks`。历史 v1.10.0 源码和当前 v1.11.0 编译均显示工具前后 callbacks 是应用写入计划、审批和结果的可插入点；它们没有同 WeKnora 的 journal、数据库事务、checkpoint 和客户端事件组成原子恢复协议。源码和当前候选 race green 都不能由此证明外部效果恰好一次。

GraphAgent 走 `GraphAgent.Run` → `graph.Executor.Execute`，在节点前后运行 graph callbacks，并由 checkpoint saver 存储 checkpoint 和 pending writes。`internal/agent/trpc/compatibility_probe.go` 的显式 plan → approval → tool → answer 图在 approval 使用 `graph.Interrupt`，基线已证明 SQLite 的 interrupted state、pending write 与工具 ID 能重开恢复。完整缺口、责任与环境限制见 [recovery-gaps.md](recovery-gaps.md)。

## P0-4：能力矩阵与版本选择结论

[八列能力矩阵](sdk-capabilities.tsv) 覆盖 Task 1 的全部功能 ID，并展开 9 个能力类别、26 个远程 chat Provider、本地 Ollama，以及只做 embedding/rerank 的 Jina 非 Agent 消费者。共 90 行：4 行 `verified`（仅继承 Task 2/3 各行指定的确定性/SQLite/策略证据）、79 行 `source-only`、2 行 `blocked-env`、5 行 `incompatible`。Task 1 只新增当前 v1.11.0 的 bounded Runner/Session/race evidence；没有新增真实 Provider、数据库或客户端运行验收。

当前版本证据为根 `go.mod` 和 `go.sum`：`v1.11.0` 模块校验和 `h1:LwMxQwT2l6hqWUVARfVA/ef2tq8gJCzbImSHurpaPIo=`，go.mod 校验和 `h1:bIZcN4N9sGpA42sWfE98XCPl9ZgMi6fMDLGYOaXNe9A=`。当前源码根为 `$(go env GOMODCACHE)/trpc.group/trpc-go/trpc-agent-go@v1.11.0`；历史 v1.10.0 checksum/source 只服务于上述 RED 记录。

**选择结论：当前根 v1.11.0 是已验证 repeated-race green 的候选，不是已批准的产品目标。** 历史 `RUN-02` v1.10.0 race 已有明确 RED，当前候选已通过相同 20 次 gate；但 `SESSION-04` 的 PostgreSQL/SQLite Session 实现与 Memory 持久化实现尚未固定版本/配置，且预算耗尽通知的直接 consumer 在 v1.10.0 基线和当前候选均失败。P0-5 继续记录 no-go；后续必须修复既有 consumer 失败，并完成存储、失败、恢复、Provider、PostgreSQL 与客户端验收。

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

### P0-4 review round 1：决策交互与准入行为补齐

新增 `TOOLS-04`：依据现有 `approval.PendingRequest/OAuthPendingRequest`、durable OAuth park、`ownedRun`、`ValidateDecision/Resolve/ApplyDecision` 和 OAuth handler，固定 PendingDecisionService 的授权 List/Get/BeginOAuth/Resolve、完整展示 DTO、service identity、operation description、当前权限脱敏参数、等待/决定状态、OAuth token 重新确认和同一 run 的恢复语义。`decision.required` 必须引用该新鲜详情，不能仅靠事件里的 pending ID 完成审批。

新增 `RUN-06` 并从 RUN-05 分离 ENG-006：RunControl.Admit 明确读取服务端 enabled/admission_enabled/worker_drain 配置及 revision；关闭开关或 drain 拒绝新准入，已有工作安全排空，读/取消/cleanup 不受准入开关屏蔽，child/followup 也不得绕过；P2/P7 负责组合、竞态、双库、重启和维护窗口验证。两行仍为 source-only；本轮没有运行产品行为测试，原生 race no-go 原样保留。
