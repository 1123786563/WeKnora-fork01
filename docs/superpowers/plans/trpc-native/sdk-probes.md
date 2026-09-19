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
