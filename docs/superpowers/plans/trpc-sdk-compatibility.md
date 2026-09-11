# tRPC SDK checkpoint 兼容性实证（Task 01）

日期：2026-09-10。结论：固定版本下，本任务的 checkpoint/interrupt/文件重开门禁通过。此结论不代表业务工具幂等、租约接管、Sandbox 或进程 SIGKILL 验收已经完成。

## 版本与安装

- 根模块：`trpc.group/trpc-go/trpc-agent-go v1.10.0`，发布时间 `2026-06-05T01:35:05Z`。
- 模块校验和：`h1:0pY2ee7tc6+3e+I7CgnkdGY2z4lrmQwQ4sq7C1dulQc=`。
- go.mod 校验和：`h1:lksOlht6E+LR7AKOA0XoKrkI3AJNsfukTVes9BFmTow=`。
- `graph/checkpoint/sqlite` 在本版本属于根模块，没有额外 saver 子模块版本。
- 实测环境：`go version go1.26.3 darwin/arm64`，SQLite 驱动 `github.com/mattn/go-sqlite3 v1.14.32`（需要 CGO）。
- SDK 最低依赖使 `github.com/go-ego/gse` 从 `v0.80.3` 升为 `v1.0.0`，SQLite 驱动从 `v1.14.24` 升为 `v1.14.32`；这是后续全仓回归需要覆盖的影响。

执行过的安装命令：

```sh
GOWORK=off go get trpc.group/trpc-go/trpc-agent-go@v1.10.0 trpc.group/trpc-go/trpc-agent-go/graph/checkpoint/sqlite@v1.10.0
GOPROXY=https://proxy.golang.org,direct GOWORK=off go get trpc.group/trpc-go/trpc-agent-go@v1.10.0 trpc.group/trpc-go/trpc-agent-go/graph/checkpoint/sqlite@v1.10.0
GOPROXY=https://goproxy.cn,direct GOWORK=off go get trpc.group/trpc-go/trpc-agent-go@v1.10.0 trpc.group/trpc-go/trpc-agent-go/graph/checkpoint/sqlite@v1.10.0
GOPROXY=https://goproxy.cn,direct GOWORK=off go get trpc.group/trpc-go/trpc-agent-go/agent/graphagent@v1.10.0 trpc.group/trpc-go/trpc-agent-go/runner@v1.10.0
GOWORK=off go list -m -json trpc.group/trpc-go/trpc-agent-go
```

环境默认 `GOPROXY=off`，首次缺少 `trpc-a2a-go v0.2.5`。公共代理下载返回 `unexpected EOF`，切换 `goproxy.cn` 后成功；最终测试使用默认离线环境通过。

## 实际 API 与官方版本代码

依据固定模块缓存中的源码、`agent/graphagent/example_test.go`、`graph/executor_checkpoint_test.go` 和 `graph/checkpoint/sqlite/sqlite_test.go`。对应官方源码：[v1.10.0 GraphAgent](https://github.com/trpc-group/trpc-agent-go/blob/v1.10.0/agent/graphagent/graph_agent.go)、[checkpoint](https://github.com/trpc-group/trpc-agent-go/blob/v1.10.0/graph/checkpoint.go)、[SQLite saver](https://github.com/trpc-group/trpc-agent-go/blob/v1.10.0/graph/checkpoint/sqlite/sqlite.go)。

从 SDK 模块目录执行 `GOWORK=off go doc ./graph CheckpointSaver`、`go doc ./agent/graphagent New`、`go doc ./graph Interrupt`、`go doc ./graph/checkpoint/sqlite NewSaver` 得到以下签名。`go doc` 同时输出了 SDK 其他未缓存依赖的离线警告；下列接口已由本项目实际编译和执行确认。

```go
type CheckpointSaver interface {
    Get(context.Context, map[string]any) (*Checkpoint, error)
    GetTuple(context.Context, map[string]any) (*CheckpointTuple, error)
    List(context.Context, map[string]any, *CheckpointFilter) ([]*CheckpointTuple, error)
    Put(context.Context, PutRequest) (map[string]any, error)
    PutWrites(context.Context, PutWritesRequest) error
    PutFull(context.Context, PutFullRequest) (map[string]any, error)
    DeleteLineage(context.Context, string) error
    Close() error
}
func graphagent.New(name string, g *graph.Graph, opts ...Option) (*GraphAgent, error)
func graphagent.WithCheckpointSaver(saver graph.CheckpointSaver) Option
func sqlite.NewSaver(db *sql.DB) (*Saver, error)
func graph.Interrupt(ctx context.Context, state State, key string, prompt any) (any, error)
```

恢复配置通过 `agent.WithRuntimeState` 传入 `lineage_id`、`checkpoint_ns` 和原 `checkpoint_id`；`__command__` 使用 `&graph.Command{ResumeMap: map[string]any{toolCallID: true}}`。`CheckpointManager.Latest(ctx, lineageID, namespace)` 只负责定位原 checkpoint，实际恢复节点前沿由 SDK executor 完成。

## 实证行为

探针每次构建新的 `runner.NewRunner`、GraphAgent、graph、SQLite saver 和数据库句柄。Session service 使用 `noop`，没有会话内存缓存。模型调用数和工具调用数均存入同一个临时 SQLite 文件。

图为 `plan(AddLLMNode) → approval(graph.Interrupt) → tool(AddToolsNode) → answer(AddLLMNode)`。Fake model 首次返回固定工具计划；最终模型验证工具结果携带原 `call-probe-1`，随后回答 `probe complete`。图执行及恢复均由 SDK 处理，探针没有自行重放节点或保存替代 checkpoint。

| 检查 | 观察 |
| --- | --- |
| 首次工具前中断 | `Completed=false`，模型调用 1 次，工具调用 0 次 |
| 关闭全部句柄后重开 | 指定原 lineage/namespace/checkpoint 恢复；最终模型累计 2 次，工具累计 1 次 |
| 计划节点去重 | 重开后没有重跑已提交的计划模型节点 |
| Interrupt payload | 重开 saver 后仍有工具名、原 tool-call ID；SDK InterruptState.TaskID 保持该 ID |
| 工具 ID | 从 checkpoint 消息恢复，在工具结果和最终模型输入中保持原值 |
| Pending writes | `PutFull` 写入后关闭/重开，`GetTuple` 读回 TaskID、Channel、Sequence=7 和 JSON Value |
| 事件终态 | 完整排空事件通道；仅 `graph.execution` 的 Done 且含最终回答计为完成；中断不会被模型 Done 或 runner 完成事件误报成功 |

`PendingRestored` 表示探针发现并恢复了持久化的中断任务；独立 pending-writes 测试覆盖 saver 的存储往返，不宣称已验证任意并发图的 pending-writes 重放。

## 实测发现与后续边界

1. SDK `StateKeyLastResponse` 是文本，不是 `*model.Response`；工具计划来自 `StateKeyMessages`。探针按模型消息 JSON schema 解码后读取原 ID，不能依赖 JSON 重开后的 Go 动态类型。
2. SQLite saver `List → processSingleRow → GetTuple` 在列表游标仍占连接时再次查询。连接池只有 1 个连接会阻塞；`-timeout=10s` 的栈停在 `database/sql.(*DB).conn`。探针明确使用 2 个连接，随后测试通过。业务 saver 实现也需检查嵌套查询与连接池配置。
3. SDK interrupt 默认会重新执行中断节点。approval 中断放在真正工具节点之前，所以本门禁没有重复副作用。业务工具执行中断后的不确定窗口仍须由工具执行记录、幂等/查询能力和 `waiting_user` 处理。
4. 本任务只验证 SQLite 文件重开，未做真实进程崩溃、生产数据库事务、租约 fencing、多实例和 Sandbox 测试；这些仍属于后续任务。

## 测试命令与结果

实现前，在依赖安装完成后运行计划指定命令，确认 RED：

```text
GOWORK=off go test ./internal/agent/trpc -run TestCheckpointProbeReopen -count=1
compatibility_probe_test.go:18:16: undefined: RunCheckpointProbe
FAIL github.com/Tencent/WeKnora/internal/agent/trpc [build failed]
```

实现后：

```text
GOWORK=off go test ./internal/agent/trpc -run TestCheckpointProbeReopen -count=1
ok github.com/Tencent/WeKnora/internal/agent/trpc 3.248s

GOWORK=off go test ./internal/agent/trpc -count=1 -v -timeout=30s
--- PASS: TestCheckpointProbeReopen (0.01s)
--- PASS: TestCheckpointProbeStreamTerminal (0.01s)
    --- PASS: TestCheckpointProbeStreamTerminal/interrupted (0.00s)
    --- PASS: TestCheckpointProbeStreamTerminal/completed (0.00s)
--- PASS: TestCheckpointProbeInterruptPayloadAndToolIDAreStable (0.01s)
--- PASS: TestSQLiteSaverPendingWritesSurviveReopen (0.00s)
PASS
ok github.com/Tencent/WeKnora/internal/agent/trpc 0.255s
```
