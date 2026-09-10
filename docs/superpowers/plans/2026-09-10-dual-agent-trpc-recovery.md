# 双 Agent 与 tRPC 持久化恢复 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 保留现有 ReAct，新增分会话使用的 tRPC Agent，复用现有能力，并仅为 tRPC 提供进程崩溃恢复与持久化用户等待。

**Architecture:** 提取共享能力装配；现有 ReAct 继续原执行路径。tRPC 使用显式 GraphAgent 节点，应用负责 Run、工具日志、租约、用户决策和事件投影，SDK saver 负责完整图状态与 pending writes 的适配。所有外部调用先记录意图，恢复时按证据重用结果、查询、幂等重试或等待用户。

**Tech Stack:** Go 1.26.0、Gin、GORM、现有 PostgreSQL/SQLite 迁移、tRPC-Agent-Go、现有 Vue 3/TypeScript 客户端。Redis 仅为可选投递加速，不能作为恢复事实来源。

**Spec:** [已批准设计](../specs/2026-09-10-dual-agent-trpc-recovery-design.md)

## Global Constraints

- 同一后端保留现有自研 ReAct，并新增 tRPC-Agent-Go。
- 两种引擎用于不同会话；一个会话固定一种引擎，不做同会话双引擎执行或中途切换。
- 复用现有 MCP、Skills、Sandbox、Tools、模型配置、权限、消息存储与事件协议。
- 仅 tRPC 首版支持后端重启、进程崩溃后的未完成运行恢复；现有 ReAct 不增加恢复保证。
- 工具执行结果不明且不能安全查询或幂等重试时，持久化暂停，等待用户。
- 本设计覆盖后端进程故障；不承诺数据库丢失恢复、任意外部操作 exactly-once 或任意 Sandbox 内存快照恢复。
- 会话 `engine_type` 为 `builtin` / `trpc`，缺省 builtin；同会话最多一个非终态 Run，包括 waiting_user。
- 首版同一 Run 工具串行执行；一次模型响应的全部 tool_calls 有结果后才进行下一次模型调用。
- 固定 SDK 版本和 graph/schema 版本；禁止浮动版本和个人绝对路径 replace；已运行配置与当前权限共同约束恢复。
- 新功能默认关闭；不支持的数据库方言拒绝启用。首版验收 PostgreSQL 与 SQLite，不因目录中存在 MySQL 初始化 SQL 就宣称支持 MySQL 恢复。
- HTTP 断线不取消 tRPC Run；取消必须持久化。旧引擎和已有客户端缺省行为不变。
- 不修改其他任务的 dirty 文件。代码实施时使用隔离 worktree；本计划不创建工作分支或改动依赖。

---

## 基线、依赖与执行约定

规划基线为 `e91f8af`。当前迁移最大号：PostgreSQL `000092`，SQLite `000013`；计划为该功能预留 `000093` / `000014`。执行前若编号已占用，先原子更新本计划及两套迁移文件名，不能覆盖已有迁移。

2026-09-10 读取的[官方 release 页面](https://github.com/trpc-group/trpc-agent-go/releases/tag/v1.10.0)显示 v1.10.0，因此 Task 01 以此为**候选验证版本**，不宣称它已通过本仓库兼容验证。滚动文档与该 tag 可能不同；SDK 精确接口在 Task 01 固定源码中核对后写入证据文件，禁止凭滚动文档猜签名。

每项任务：先写列出的失败测试并确认失败原因，再实现、验证、审查和提交。下文 Go 片段置于指定包，导入标准库及本仓库包；片段定义关键断言和接口，不替代完整实现。所有执行命令从仓库根目录运行，前端命令单独指定 `frontend`。

每项完成只提交该任务明确拥有的文件：`git add -- <本任务明确文件列表>`，检查 `git diff --cached --check` 后使用该任务给出的提交信息；不得 `git add .`。迁移向前和向后测试仅使用隔离测试数据库。

| 任务 | 交付 | 依赖 |
|---|---|---|
| 01 | SDK 单步执行、真实 checkpoint 和重开恢复验证 | 无 |
| 02 | 共享装配、引擎类型校验 | 无 |
| 03 | Run 存储、会话唯一活跃运行、租约 | 02 |
| 04 | 模型适配与图状态保存器 | 01、03 |
| 05 | 工具执行日志及安全恢复策略 | 03 |
| 06 | tRPC 图执行纵向链路 | 02、04、05 |
| 07 | 请求受理、后台执行与重启接管 | 03、06 |
| 08 | 持久化审批与未知结果决策 | 05、07 |
| 09 | Sandbox 绑定恢复与资源保护 | 05、07、08 |
| 10 | MCP/Skills/记忆/多模态等能力完整复用 | 06、08、09 |
| 11 | 事件回放、消息完成事务与 steering | 07、08、10 |
| 12 | API、权限、取消与删除生命周期 | 08、09、11 |
| 13 | 最小客户端引擎选择与恢复交互 | 12 |
| 14 | 跨进程故障验收及启用门禁 | 01–13 |

进度账本：[trpc-recovery-progress.md](trpc-recovery-progress.md)。失败测试、通过命令、提交 SHA、审查结论分别记录；未运行不得标记通过。01 是 SDK 门禁，14 是发布门禁；01 失败时不得推进 04/06，不能绕成自研聊天循环后仍称为 tRPC GraphAgent。

## 文件与责任映射

| 路径 | 责任 / 归属任务 |
|---|---|
| `internal/agent/trpc/compatibility_*` | SDK 实证（01） |
| `internal/application/service/agent_capabilities.go` | 共享装配（02），能力完整性（10） |
| `internal/types/agent_engine.go` | 引擎枚举与校验（02） |
| `internal/agent/runtime/contracts.go` | Run、lease、结果等共享契约（03）；后续新增字段先更新接口清单 |
| `internal/application/repository/agent_run*.go` | Run、租约、事务、工具、事件和输入持久化（03/05/08/11） |
| `internal/agent/trpc/model.go`、`checkpoint.go`、`graph.go` | 模型与图适配（04/06） |
| `internal/agent/runtime/tool_executor.go` | 工具日志协议（05） |
| `internal/application/service/agent_run_*.go` | 受理、恢复、用户决策与投影（07/08/11/12） |
| `internal/sandbox/execution_recovery.go` | 可选任务恢复协议（09） |
| `internal/handler/session/agent_run.go` | HTTP 契约（12） |
| `frontend/src/api/chat/runs.ts`、`utils/agentRunState.ts` | 客户端契约及重放状态（13） |
| `internal/agent/recoverytest/` | 黑盒杀进程与外部副作用观测（14） |

### Task 01：固定 SDK 与 checkpoint 恢复实证

**Files:** 修改 `go.mod`、`go.sum`；创建 `internal/agent/trpc/compatibility_probe.go`、`compatibility_probe_test.go`、`docs/superpowers/plans/trpc-sdk-compatibility.md`。

**Interfaces:** 本任务定义 `ProbeReport { ModelCalls int; ToolCalls int; Completed bool; PendingRestored bool }` 和 `RunCheckpointProbe(context.Context, string, bool) (ProbeReport, error)`。路径参数是临时 SQLite 文件；bool 表示在工具计划保存后以 SDK interrupt 停止。每次调用必须创建新的 runner/graph/saver 并关闭全部句柄；计数保存在测试数据库中，禁止跨调用内存缓存。

- [ ] 写失败测试，首次停在工具前，重新打开文件后完成；第二次不能重新调用已提交的计划模型节点。

```go
func TestCheckpointProbeReopen(t *testing.T) {
    path := filepath.Join(t.TempDir(), "probe.db")
    first, err := RunCheckpointProbe(context.Background(), path, true)
    require.NoError(t, err)
    require.False(t, first.Completed)
    require.Equal(t, 0, first.ToolCalls)
    next, err := RunCheckpointProbe(context.Background(), path, false)
    require.NoError(t, err)
    require.True(t, next.Completed)
    require.True(t, next.PendingRestored)
    require.Equal(t, 1, next.ToolCalls)
    require.Equal(t, 2, next.ModelCalls) // 计划一次，工具结果后的回答一次
}
```

- [ ] 执行 `GOWORK=off go test ./internal/agent/trpc -run TestCheckpointProbeReopen -count=1`，记录未实现导致的失败。
- [ ] 在隔离代码工作树执行 `GOWORK=off go get trpc.group/trpc-go/trpc-agent-go@v1.10.0`；用 `go doc` 查看 `graph.CheckpointSaver`、`agent/graphagent`、`graph` 的实际接口与官方 tag 示例。若 saver 为独立 Go module，只安装与该版本相符的已发布版本并记录根/子模块组合。
- [ ] 使用真实 SDK 构建 `model → persisted plan → one tool → model` 图；fake model 返回固定工具调用和最终回答，工具递增 SQLite 计数。saver 使用固定版本提供的持久化实现；恢复指定原 namespace/checkpoint，不创建新会话重演。
- [ ] 追加 pending writes round-trip、interrupt payload、流式终态、工具 ID 稳定性测试；证据文件记录真实签名、版本/校验和、命令和观察结果。不兼容则记录实际 API 缺口，停止依赖任务，不靠关闭 checkpoint 过门禁。
- [ ] 重跑上述测试，确认 PASS；提交 `test(agent): characterize pinned trpc checkpoint recovery`。

### Task 02：共享装配与固定引擎类型

**Files:** 创建 `internal/types/agent_engine.go`、`agent_engine_test.go`、`internal/application/service/agent_capabilities.go`、`agent_capabilities_test.go`；修改 `internal/application/service/agent_service.go`。

**Interfaces:** `types.AgentEngineType string`，常量 `AgentEngineBuiltin="builtin"`、`AgentEngineTRPC="trpc"`；`ParseAgentEngine(string) (AgentEngineType,error)`。共享装配返回 `AgentCapabilities`，包含 `Config *types.AgentConfig`、`Chat chat.Chat`、`Rerank rerank.Reranker`、`Tools *tools.ToolRegistry`、`Skills *skills.Manager`、`KnowledgeBases []*agent.KnowledgeBaseInfo`、`Documents []*agent.SelectedDocumentInfo`、`SystemPrompt string`、`EventBus *event.EventBus`；其他 pinned/VLM 字段沿当前构造代码逐项迁移，类型直接复用现有声明，不另定义同义结构。

- [ ] 添加类型失败测试，并为共享装配添加两个不同 session 的独立 registry 测试；一个 registry 激活延迟 MCP 定义不得影响另一个。

```go
func TestParseAgentEngine(t *testing.T) {
    for _, raw := range []string{"", "builtin"} {
        got, err := ParseAgentEngine(raw)
        require.NoError(t, err)
        require.Equal(t, AgentEngineBuiltin, got)
    }
    _, err := ParseAgentEngine("typo")
    require.Error(t, err)
}
```

- [ ] 执行 `GOWORK=off go test ./internal/types ./internal/application/service -run 'TestParseAgentEngine|TestAgentCapabilities' -count=1`，确认失败。
- [ ] 将 CreateAgentEngine 中工具/MCP/Sandbox/Skills/提示词准备提取到 `prepareAgentCapabilities`；参数保留原 CreateAgentEngine 的 context、config、模型、eventBus、session/assistant ID，返回 `(*AgentCapabilities,error)`。旧 CreateAgentEngine 只消费该结构并构造原引擎。

```go
func ParseAgentEngine(raw string) (AgentEngineType, error) {
    switch raw {
    case "", "builtin": return AgentEngineBuiltin, nil
    case "trpc": return AgentEngineTRPC, nil
    default: return "", fmt.Errorf("unsupported agent engine: %q", raw)
    }
}
```

- [ ] 保留原 first-wins、MCP 动态公开、Skill 安装模式、租户 env、知识权限和共享 Agent 写限制。先校验可用引擎，再创建有副作用的 Sandbox，避免拒绝请求仍创建资源。
- [ ] 执行上面的测试和 `GOWORK=off go test ./internal/application/service -run 'TestAgent|TestSharedAgent' -count=1`；提交 `refactor(agent): separate reusable capability assembly`。

### Task 03：Run 受理存储与租约事务

**Files:** 创建 `internal/agent/runtime/contracts.go`、`internal/application/repository/agent_run.go`、`agent_run_test.go`；修改 `internal/types/session.go`；创建 `migrations/versioned/000093_agent_runs.up.sql`、`.down.sql`、`migrations/sqlite/000014_agent_runs.up.sql`、`.down.sql`。

**Interfaces:** 下列契约放在 runtime 包；repository 提供 `NewAgentRunStore(*gorm.DB) *AgentRunStore`。JSON 字段保存版本化值，所有持久化记录携带 tenant/run。

同文件定义哨兵错误 `ErrConflict`、`ErrRunActive`、`ErrLeaseLost`、`ErrNotFound`、`ErrCursorExpired`，均使用 `errors.New`，调用方用 `errors.Is` 判断；不依赖字符串匹配。存储私有实体可比 Run 查询视图拥有更多列，但必须覆盖规格第 6 节全部字段。

```go
type RunKey struct { TenantID uint64; RunID string }
type Fence struct { RunKey; Owner string; Epoch int64 }
type Run struct {
    Key RunKey
    SessionID, UserID, RequestID, AssistantMessageID string
    Status, WaitReason, Owner string
    Revision, Epoch int64
    LeaseUntil, Deadline time.Time
    Snapshot json.RawMessage
}
type Admission struct {
    Key RunKey
    SessionID, UserID, RequestID, AssistantMessageID, RequestHash string
    Snapshot, UserMessage, AssistantMessage json.RawMessage
    Deadline time.Time
}
type CheckpointRecord struct {
    Namespace, ID, ParentID string
    Seq int64
    State, PendingWrites json.RawMessage
}
type RunStore interface {
    Admit(context.Context, Admission) (Run, error)
    Get(context.Context, RunKey) (Run, error)
    Claim(context.Context, RunKey, string, time.Duration) (Fence, error)
    Renew(context.Context, Fence, time.Duration) error
    Scan(context.Context, int) ([]RunKey, error)
    SaveCheckpoint(context.Context, Fence, CheckpointRecord) error
    LoadCheckpoint(context.Context, RunKey) (CheckpointRecord, error)
}
```

- [ ] 创建 repository 测试辅助 `openRunTestDB(t *testing.T) *gorm.DB`：临时文件 SQLite，执行完整迁移而非 AutoMigrate，创建满足当前外键的 tenant/user/session fixture（tenant=1，owner=u1，session=s1/s2，两个 session 的 engine_type 均为 trpc）。定义下列 fixture；同请求键重复返回第一次 Run，同键不同 hash 返回 `ErrConflict`，第二个请求占用同会话返回 `ErrRunActive`。

```go
func TestAgentRunAdmissionIdempotent(t *testing.T) {
    store := NewAgentRunStore(openRunTestDB(t))
    in := runtime.Admission{
        Key: runtime.RunKey{TenantID: 1, RunID: "r1"}, SessionID: "s1",
        UserID: "u1", RequestID: "q1", AssistantMessageID: "a1", RequestHash: "h1",
        Snapshot: json.RawMessage(`{"version":1}`),
        UserMessage: json.RawMessage(`{"role":"user","content":"hello"}`),
        AssistantMessage: json.RawMessage(`{"role":"assistant","content":""}`),
        Deadline: time.Now().Add(time.Hour),
    }
    first, err := store.Admit(context.Background(), in)
    require.NoError(t, err)
    in.Key.RunID = "r2"
    again, err := store.Admit(context.Background(), in)
    require.NoError(t, err)
    require.Equal(t, first.Key, again.Key)
}
```

- [ ] 执行 `GOWORK=off go test ./internal/application/repository -run TestAgentRun -count=1`，确认失败。
- [ ] 实现迁移：sessions 增加 engine_type 默认 builtin 和 active_agent_run_id；新增 agent_runs/checkpoints/tool_calls/tool_attempts/events/decisions/inputs。主键及外键覆盖 tenant；请求幂等唯一键绑定 tenant+owner+request_id；保存请求 hash。ToolCall 唯一键 run+call_id；event 唯一键 run+seq；decision 唯一键 run+decision_id，pending 处理用 CAS。
- [ ] Admit 一次事务校验 session owner/type、CAS active slot、写两条消息与 Run；existing key 检索先做 tenant/owner 校验。同 session waiting_user 不释放 active slot。
- [ ] Claim 使用 DB 时间和条件 UPDATE 获取过期/queued Run，并 epoch+1；Renew 必须核对 owner/epoch/未过期。追加两个独立 DB 连接竞争 claim、过期旧 epoch 保存失败、跨租户加载拒绝、事务失败不残留消息的测试。
- [ ] SQLite 文件重开及 PostgreSQL 隔离 schema 执行相同验收；测试 PostgreSQL 通过专用 `TRPC_TEST_POSTGRES_DSN`，未设置时显式报告该组合未验收。两组迁移 up/down/up 后验证默认 builtin 和索引；提交 `feat(agent): persist trpc runs and fenced leases`。

### Task 04：模型适配与完整 checkpoint 保存

**Files:** 创建 `internal/agent/trpc/model.go`、`model_test.go`、`checkpoint.go`、`checkpoint_test.go`、`state.go`；扩展 `internal/application/repository/agent_run.go`。

**Interfaces:** `NewModel(chat.Chat)` 返回固定 SDK 的 model.Model；`NewCheckpointSaver(runtime.RunStore, runtime.Fence)` 返回 Task 01 证实的 saver 接口。`State` 定义 Version、Messages（SDK message 类型）、PendingCallIDs、NextCallIndex、AppliedCallIDs、CompactionState、ModelAttemptID、InputCursor、UsageAttempts；所有字段可序列化，禁止放连接、函数和 context。

State 字段类型：Version/NextCallIndex 为 int，PendingCallIDs 为 []string，AppliedCallIDs 为 map[string]bool，ModelAttemptID 为 string，InputCursor 为 int64；CompactionState 为 json.RawMessage，UsageAttempts 为 map[string]json.RawMessage，Messages 为 []model.Message。两个 RawMessage 的 JSON schema 版本必须固定，未知版本拒绝恢复，不宽松解码成空值。

- [ ] 模型测试用现有 Chat 接口 fake，记录收到的消息和 opts；验证工具 schema、stream reasoning、完整/分片工具参数、多模态、usage 与错误均保留。checkpoint 测试至少验证尚未完成的调用及 pending writes。

```go
func TestStateRoundTripPendingCalls(t *testing.T) {
    before := State{Version: 1, PendingCallIDs: []string{"c1", "c2"}, NextCallIndex: 1,
        AppliedCallIDs: map[string]bool{"c1": true}, ModelAttemptID: "m1"}
    raw, err := json.Marshal(before)
    require.NoError(t, err)
    var after State
    require.NoError(t, json.Unmarshal(raw, &after))
    require.Equal(t, before.PendingCallIDs, after.PendingCallIDs)
    require.Equal(t, 1, after.NextCallIndex)
    require.True(t, after.AppliedCallIDs["c1"])
}
```

- [ ] 执行 `GOWORK=off go test ./internal/agent/trpc -run 'TestState|TestModel|TestCheckpoint' -count=1`，确认失败。
- [ ] 依据 Task 01 真实接口实现 saver 所有必要操作，完整保存 state/metadata/pending writes，Save 必须经过 fenced transaction；namespace 由 tenant/run/graph version 构造。拒绝 graph/schema 版本不兼容，恢复时验证引用的工具日志。
- [ ] NewModel 把 SDK 请求转换给现有 Chat/ChatStream，不新建 provider 凭据链。累积工具参数到完整响应后才允许产生计划；context 取消和 provider 错误不转为空成功回答。
- [ ] 将模型完成响应和 attempt 身份随 checkpoint 保存，未完成文本事件标记 attempt，不生成可执行工具计划。验证 saver 句柄重建后加载完全相同状态、旧 epoch 拒写和多 session 隔离；提交 `feat(agent): adapt existing models and persist graph checkpoints`。

### Task 05：工具日志与恢复策略

**Files:** 创建 `internal/agent/runtime/tool_executor.go`、`tool_executor_test.go`、`tool_recovery.go`、`tool_recovery_test.go`、`internal/application/repository/agent_run_tools.go`、`agent_run_tools_test.go`。

**Interfaces:** `RecoveryFacts` 与策略函数如下；runtime 增加 `ToolPlan { CallID, Name, Identity, ArgsHash, IdempotencyKey string; Args json.RawMessage }`、`StoredToolResult { Result types.ToolResult; OutputFiles []string; Source string }`、`ToolExecutor.Execute(context.Context,Fence,ToolPlan) (StoredToolResult,error)`。构造器注入 RunStore、日志 repository 和现有 `ToolRegistry.ExecuteTool` 函数。

```go
type RecoveryFacts struct {
    HasResult, NotDispatched, Queryable, Idempotent, KeyValid, ReadOnly bool
}
func RecoveryAction(f RecoveryFacts) string {
    switch {
    case f.HasResult: return "reuse"
    case f.NotDispatched: return "execute"
    case f.Queryable: return "query"
    case f.Idempotent && f.KeyValid: return "retry"
    case f.ReadOnly: return "retry"
    default: return "wait_user"
    }
}
```

- [ ] 写策略失败测试，以及结果提交后进程重建不再次调用工具的 repository 测试。尤其验证 OutputFiles 独立序列化：现有 types.ToolResult.OutputFiles 使用 `json:"-"`。

```go
func TestUnknownWriteWaits(t *testing.T) {
    require.Equal(t, "wait_user", RecoveryAction(RecoveryFacts{}))
    require.Equal(t, "wait_user", RecoveryAction(RecoveryFacts{Idempotent: true}))
    require.Equal(t, "reuse", RecoveryAction(RecoveryFacts{HasResult: true}))
    require.Equal(t, "query", RecoveryAction(RecoveryFacts{Queryable: true}))
}
```

- [ ] 执行 `GOWORK=off go test ./internal/agent/runtime ./internal/application/repository -run 'TestUnknown|TestTool|TestAgentRunTool' -count=1`，确认失败。
- [ ] 实现 planned → dispatching → result：计划与参数摘要先提交，dispatch 事务检查有效 fence/权限/取消，外部调用在事务外执行，结果事务再次检查 fence。ToolAttempt append-only；同 call_id 不同 args/identity 报冲突。
- [ ] 为不可确定失败保存 unknown outcome，不能将 timeout/cancel 当未执行。复用结果不重新跑权限之外的外部逻辑；无可信分类默认 wait_user。原幂等键和有效期持久化，人工重试增加 attempt 不抹旧记录。
- [ ] 强制测试外部计数为 1、结果读回含 OutputFiles/Images、已登记副作用后故障进入等待、旧 epoch 不能提交；通过后提交 `feat(agent): journal tool calls and classify recovery`。

### Task 06：显式图执行纵向链路

**Files:** 创建 `internal/agent/trpc/graph.go`、`graph_test.go`、`engine.go`、`engine_test.go`。

**Interfaces:** `GraphRunner.Run(context.Context,runtime.Fence) error`；构造参数为共享能力、model adapter、RunStore、ToolExecutor、checkpoint saver。不让 runtime 包导入 application service；在 container/service 中组合依赖。`NextAfterTool(index,count int) string` 是图路由的单一实现。

为避免循环导入，trpc 包也不得导入 application/service。service 将 AgentCapabilities 映射为 trpc 自己定义的 GraphBindings：`Model model.Model`、`Store runtime.RunStore`、`Tools *runtime.ToolExecutor`、`Finalize func(context.Context,runtime.Fence,json.RawMessage)error`、`InitialState State`。`NewGraphRunner(GraphBindings) (*GraphRunner,error)` 在每次 Run 中用 fence 创建对应 saver；共享提示词/工具定义等由初始状态和模型适配配置传入。Finalize 在本任务测试中注入记录回调，Task 11 绑定事务实现；生产启用前必须完成该绑定。

- [ ] 写多工具批次测试，验证 c1 已保存、c2 未执行时恢复仅执行 c2，全部结果齐备才调用后续模型。

```go
func TestNextAfterToolWaitsForBatch(t *testing.T) {
    require.Equal(t, "dispatch_one_tool", NextAfterTool(1, 2))
    require.Equal(t, "model", NextAfterTool(2, 2))
}
```

- [ ] 执行 `GOWORK=off go test ./internal/agent/trpc -run 'TestNextAfterTool|TestGraph' -count=1`，确认失败。
- [ ] 使用 Task 01 已验证的 SDK 图构建 API，节点为 prepare/model/persist_tool_plan/dispatch_one_tool/apply_result/finalize；图循环代码必须调用真正的 SDK graph runner，不调用旧 AgentEngine.executeLoop。

```go
func NextAfterTool(index, count int) string {
    if index < count { return "dispatch_one_tool" }
    return "model"
}
```

- [ ] persist_tool_plan 固定完整响应和全部调用；apply_result 用 call_id 去重更新消息和游标，先核对工具日志完整性。初次 prepare 导入业务历史，恢复 prepare 使用原 checkpoint，不再次导入原 query。
- [ ] 增加一次完整问答、工具结果后中断、空工具列表、坏参数、模型中断、预算耗尽及跨 session 的测试。正式 finalize 通过可注入回调连接 Task 11，不在此处写第二套业务消息模型；在生产 wiring 未完成前保持功能关闭。
- [ ] 通过后提交 `feat(agent): execute recoverable trpc tool graph`。

### Task 07：后台受理与故障接管

**Files:** 创建 `internal/application/service/agent_run_service.go`、`agent_run_worker.go`、`agent_run_worker_test.go`、`internal/container/agent_runtime.go`；修改 `internal/application/service/session_agent_qa.go`、`internal/container/container.go`、`internal/config/config.go`、`cmd/server/main.go`。

**Interfaces:** service 提供 `Submit(ctx context.Context,in runtime.Admission)(runtime.Run,error)`；Worker 提供 `Run(ctx context.Context) error` 与 `Tick(ctx context.Context) error`，构造器接收 RunStore、`func(context.Context,runtime.Fence)error` 执行器和配置。配置 `enabled=false, lease=60s, heartbeat=15s, scan_interval=5s, max_workers=4`；启动验证 heartbeat < lease/2。

`WorkerConfig` 定义 `Lease, Heartbeat, ScanInterval time.Duration` 和 `MaxWorkers int`，方法 `Validate()error` 校验全部值为正并验证心跳比例。enabled 是配置入口控制项；将 admission_enabled 与 worker drain 生命周期分开，关闭新请求不能使已有 Run 永久失去恢复者。

- [ ] 使用 fake RunStore（实现 Task 03 的全部方法）和执行计数回调，测试两个 Tick 不重复执行、waiting_user 不扫描、过期 epoch 不继续。数据库实际抢占由 Task 03/14 验证。

```go
func TestWorkerConfigRejectsUnsafeLease(t *testing.T) {
    cfg := WorkerConfig{Lease: time.Minute, Heartbeat: time.Minute,
        ScanInterval: time.Second, MaxWorkers: 1}
    require.Error(t, cfg.Validate())
}
```

- [ ] 执行 `GOWORK=off go test ./internal/application/service -run 'TestWorker|TestAgentRunWorker' -count=1`，确认失败。
- [ ] Submit 在请求已完成访问校验后构造 immutable snapshot，执行 Admit 再唤醒 Worker；业务 query、scope、owner 不能只保存在 HTTP context。worker 由应用生命周期 context 管理，不继承浏览器请求取消。
- [ ] Tick 查 queued/过期运行，Claim 后重建当前权限/模型凭据/资源引用、加载图，按 max_workers 限流并持续 Renew。回收 goroutine 前记录对应状态，不能把普通 goroutine 退出当业务 completed。
- [ ] SIGTERM 停止受理、停止新 dispatch、有界收尾；无法完成的 Run 保留非终态等租约超时。暂时错误有界退避，版本/权限问题留明确原因，预算从持久化读取。
- [ ] 测试提交后 HTTP context 取消而 Run 可继续、旧引擎仍走原入口、disabled 不创建 tRPC 资源；提交 `feat(agent): dispatch and reclaim durable trpc runs`。

### Task 08：持久化审批与用户决策

**Files:** 创建 `internal/application/repository/agent_run_decisions.go`、`agent_run_decisions_test.go`、`internal/application/service/agent_run_decisions.go`、`agent_run_decisions_test.go`；修改 `internal/agent/approval/gate.go`、`internal/application/service/agent_capabilities.go`。

**Interfaces:** runtime 定义 `Decision { PendingID, DecisionID, Action, ArgsHash, Reason string; ExpectedRevision int64; Result json.RawMessage }`；service `Resolve(ctx context.Context,key runtime.RunKey,in runtime.Decision)(runtime.Run,error)`。action 为 retry/provide_result/terminate；执行前 approval 使用独立 approve/reject 路径，不能混用未知结果动作。

- [ ] 定义 `ValidateDecision(runtime.Decision) error`，先写拒绝空结果和无原因重试的失败测试；repository 测试两个并发处理只有一个改变 revision，同 decision_id 相同 payload 返回原结果，不同 payload 冲突。

```go
func TestDecisionRequiresSuppliedResult(t *testing.T) {
    err := ValidateDecision(runtime.Decision{PendingID: "p1", DecisionID: "d1",
        Action: "provide_result", ExpectedRevision: 1})
    require.Error(t, err)
}
```

- [ ] 执行 `GOWORK=off go test ./internal/application/service ./internal/application/repository -run 'TestDecision|TestAgentRunDecision' -count=1`，确认失败。
- [ ] 将审批策略 Checker 与 waiter 载体分离：builtin 使用原 Gate；tRPC 写 pending 和 waiting_user 后释放 Worker，通过图 interrupt 返回。OAuth 重连是独立 reason，保存授权资源引用不保存临时 token。
- [ ] Resolve 事务校验 owner、pending、revision、当前权限、参数摘要；记录决策并将 Run 排队或取消。provide_result 校验 ToolResult JSON envelope 和 1 MiB 请求上限，标记 source=user；retry 创建新 attempt 并记录可能重复风险原因。
- [ ] 模拟等待落库后所有对象重建，仍能处理且只继续一次；参数修改使旧审批失效；提交 `feat(agent): persist human decisions across restarts`。

### Task 09：Sandbox 任务恢复与资源引用

**Files:** 创建 `internal/sandbox/execution_recovery.go`、`execution_recovery_test.go`、`internal/application/service/agent_run_resources.go`、`agent_run_resources_test.go`；修改 `internal/sandbox/session_manager.go`、`session_binding.go`、`docker_idle_sweeper.go`。

**Interfaces:** 可选接口放 sandbox 包，不破坏现有 Execute。

```go
type ExecutionRef struct {
    TenantID uint64
    SessionID, Provider, ConfigID, InstanceID, Generation, TaskID, WorkspaceID string
}
type ExecutionObservation struct {
    State string // running, succeeded, failed, unknown, missing
    Result json.RawMessage
}
type ExecutionRecovery interface {
    Observe(context.Context, ExecutionRef) (ExecutionObservation, error)
    CancelExecution(context.Context, ExecutionRef) error
}
func CanImportObservation(o ExecutionObservation) bool {
    return (o.State == "succeeded" || o.State == "failed") && len(o.Result) > 0
}
```

- [ ] 写缺失实例不能导入成功结果的测试，以及 fake provider 仅 Observe、从未 Create 的恢复测试。

```go
func TestMissingSandboxCannotResumeAsSuccess(t *testing.T) {
    require.False(t, CanImportObservation(ExecutionObservation{State: "missing"}))
    require.False(t, CanImportObservation(ExecutionObservation{State: "succeeded"}))
}
```

- [ ] 执行 `GOWORK=off go test ./internal/sandbox ./internal/application/service -run 'TestMissingSandbox|TestExecutionRecovery|TestAgentRunResource' -count=1`，确认失败。
- [ ] 保存 provider/instance/task/workspace/generation；用现有 session binding 接回原实例并验证归属。没有 task 查询能力时不伪造实现，返回 unknown 进入 Task 08 等待；PID 和容器存在均不作为完成证据。
- [ ] 资源 repository 记录非终态引用，sweeper 删除前查询保护；不可续期 TTL 或已丢失 workspace 进入 sandbox_unavailable。仅恢复绑定，不自动从空镜像重建工作区。引用清理与终态/会话删除协调。
- [ ] 验证 running/complete/unknown/missing、跨租户 ref 拒绝、旧 generation 拒绝、保护引用后 sweeper 不删；提交 `feat(agent): reconcile sandbox resources during recovery`。

### Task 10：现有能力完整复用与兼容矩阵

**Files:** 创建 `internal/agent/trpc/capabilities.go`、`capabilities_test.go`、`docs/superpowers/plans/trpc-capability-parity.md`；修改 `internal/application/service/agent_capabilities.go`、`internal/agent/trpc/model.go`、`graph.go`、`state.go`。

**Interfaces:** `CapabilitySnapshot { ToolIdentities []string; DeferredNames []string; SkillDigests map[string]string; MemoryPrompt string; ImageReferences []string }` 序列化进 Run snapshot/State。MCP 身份使用稳定 service ID+原工具名；恢复通过现有 register/prepare 路径重建，再恢复已公开集合。

- [ ] 写 capability round-trip 测试；覆盖两 session 不共享延迟工具公开集合，Skill digest 变化不能无提示继续。

```go
func TestCapabilitySnapshotKeepsDeferredTools(t *testing.T) {
    before := CapabilitySnapshot{ToolIdentities: []string{"svc1/search"},
        DeferredNames: []string{"svc1/search"}, SkillDigests: map[string]string{"skill1":"sha256:a"}}
    raw, err := json.Marshal(before)
    require.NoError(t, err)
    var after CapabilitySnapshot
    require.NoError(t, json.Unmarshal(raw, &after))
    require.Equal(t, before, after)
}
```

- [ ] 执行 `GOWORK=off go test ./internal/agent/trpc -run 'TestCapability|TestModel' -count=1`，确认失败。
- [ ] 建立兼容矩阵逐行测试：RAG scope/rerank、MCP deferred describe/call/审批/OAuth、Skills 读取/安装/env、Shell/file、模型流/工具参数/图片、VLM fallback、memory、compaction、引用、产物、模型使用量。每行记录复用入口、fixture、观察与恢复行为，不能仅列名字打勾。
- [ ] 复用原 system prompt 构造与 memory envelope；compaction 结果写入图，不重复压缩同一已提交边界。提供商 tool output image 处理、模型配置与限流复用现有代码，禁止同时开启 tRPC 自带同名业务工具。
- [ ] 关闭与释放按 Run 所有权：registry cleanup 不删除尚有非终态引用的产物/Sandbox。工具原生输出与模型裁剪结果分别保留。
- [ ] 运行兼容矩阵中的聚焦测试和既有 agent service 回归；提交 `feat(agent): preserve shared capabilities in trpc execution`。

### Task 11：持久化事件、终态事务与 steering

**Files:** 创建 `internal/application/repository/agent_run_events.go`、`agent_run_inputs.go` 及同名测试；创建 `internal/application/service/agent_run_projection.go`、`agent_run_projection_test.go`；修改 `internal/handler/session/agent_stream_handler.go`、`steer.go`。

**Interfaces:** runtime 增加 `RunEvent { Seq int64; AttemptID, Type string; Payload json.RawMessage }`、`RunInput { SteerID, Mode string; Message json.RawMessage }`。repository 提供 `AppendEvent(ctx,Fence,RunEvent)(RunEvent,error)`、`ReadEvents(ctx,RunKey,after int64,limit int)([]RunEvent,error)`、`Finalize(ctx,Fence,answer json.RawMessage)error`、`AppendInput(ctx,RunKey,RunInput)error`、`ApplyInput(ctx,Fence,steerID string,CheckpointRecord)error`。

- [ ] 写重放过滤测试和 repository 测试：finalize 两次只出现一个业务完成事件；输入接受后重启，message 与 input cursor 原子提交，不重复应用。

```go
func TestEventCursorDropsDeliveredRows(t *testing.T) {
    events := []runtime.RunEvent{{Seq:1}, {Seq:2}, {Seq:3}}
    got := EventsAfter(events, 2)
    require.Equal(t, []runtime.RunEvent{{Seq:3}}, got)
}
```

- [ ] 定义 `EventsAfter([]runtime.RunEvent,int64) []runtime.RunEvent` 为有序过滤辅助；执行 `GOWORK=off go test ./internal/application/service ./internal/application/repository -run 'TestEvent|TestAgentRunEvent|TestAgentRunInput|TestAgentRunFinalize' -count=1`，确认失败。
- [ ] event seq 在 Run 行锁/CAS 下分配；先提交再投递。finalize 同事务写消息、终态、完成事件并释放 session active slot；引用/usage 等投影以 run/event 身份去重。
- [ ] 图开始新 model attempt 时先写 attempt_replaced 事件，禁止把废弃半段拼接到答案。外部发送/产物收集等无法进入数据库事务的后续动作使用带去重身份的持久化 outbox，不放回 SSE handler goroutine 作为唯一执行者。
- [ ] steering 按 mode 保存：inject 在安全节点边界与 checkpoint 同事务确认；after 保持待处理，只有当前 Run 终态后受理下一 Run。waiting_user 的普通消息不能当决策消费；清理消息前走 Task 12 生命周期。
- [ ] 回放游标早于保留起点返回明确过期错误；验证断线/重连、finalize 提交后进程退出、注入中崩溃、queued follow-up 不抢占等待；提交 `feat(agent): persist run events and message projections`。

### Task 12：HTTP 契约、权限与取消删除

**Files:** 创建 `internal/handler/session/agent_run.go`、`agent_run_test.go`、`internal/application/service/agent_run_lifecycle.go`、`agent_run_lifecycle_test.go`；修改 `internal/handler/session/types.go`、`handler.go`、`internal/router/routes_chat.go`、`internal/router/router_api_key_capabilities_test.go`。

**Interfaces:** 在 `/api/v1/sessions/:session_id/runs/:run_id` 下提供 GET 状态、GET `/events`、POST `/decisions`、POST `/cancel`。创建 session 接受 engine_type；已有 session UPDATE 拒绝引擎变更。service 复用 Get/Resolve，新增 `Cancel(ctx,RunKey)error`、`DeleteSessionRuns(ctx,tenantID uint64,sessionID string)error`。

- [ ] 创建 Gin route fixture，使用 fake service 统计执行次数；访问其他 tenant/owner 的 run 返回 404 且调用次数为零。状态决策用下面的 JSON 作为契约 fixture。

```json
{"pending_id":"p1","decision_id":"d1","expected_revision":7,"action":"retry","reason":"已检查外部系统，允许重新执行"}
```

```go
func TestEngineCannotChangeAfterSessionCreation(t *testing.T) {
    err := ValidateEngineUpdate(types.AgentEngineBuiltin, types.AgentEngineTRPC)
    require.Error(t, err)
}
```

- [ ] 定义 `ValidateEngineUpdate(current,next types.AgentEngineType)error` 仅允许相等；执行 `GOWORK=off go test ./internal/handler/session ./internal/router -run 'TestEngine|TestAgentRun|TestAPIKey' -count=1`，确认失败。
- [ ] 在受理前校验 session owner、API key scope、共享 Agent 来源和当前工具权限；run tenant/owner 从数据库校验，不能从 body 接受。错误映射：参数 400，超限 413，不可见 404，版本/并发冲突 409，功能关闭 503。
- [ ] events 使用 Last-Event-ID 解析 seq，补读已提交事件再持续查询/订阅，避免补读与订阅之间丢失；重复投递由 seq 去重。
- [ ] Cancel 先写 durable cancellation intent，阻止新 dispatch，尝试取消外部任务；无法确认终止时保留说明。删除/清空/batch delete 先阻止接管与新执行，再清理状态；不得删除记录后放任旧 Worker 重建 Run。用户终止不等同外部副作用回滚。
- [ ] 覆盖所有创建会话入口缺省 builtin，包括 API/embed；tRPC 受理仍由统一业务 service 承担。未提供可恢复 UI 的客户端可使用 API 决策，不能永久堵在进程审批通道。
- [ ] 通过后提交 `feat(api): expose durable agent run controls`。

### Task 13：最小客户端交互

**Files:** 创建 `frontend/src/api/chat/runs.ts`、`frontend/src/utils/agentRunState.ts`、`agentRunState.test.ts`、`frontend/src/views/chat/components/AgentRunRecovery.vue`；修改 `frontend/src/api/chat/index.ts`、`streame.ts`、`frontend/src/components/Input-field.vue`、`frontend/src/views/chat/index.vue`，按现有 locale 结构添加对应文案。

**Interfaces:** TypeScript `RunView { run_id:string; status:string; revision:number; seq:number; pending_id?:string; wait_reason?:string }`、`RunEvent { seq:number; attempt_id:string; type:string; payload:Record<string,unknown> }`；`applyRunEvent(state:ClientRunState,event:RunEvent):ClientRunState`。`ClientRunState {seq:number;attemptId:string;text:string}`。

- [ ] 按现有 `tsx --test` 使用 node:test，先写重复 seq 丢弃与 attempt 替换测试，不引入另一套测试框架。

```ts
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { applyRunEvent } from './agentRunState';
test('replacement clears incomplete text', () => {
  const old = { seq: 4, attemptId: 'm1', text: '未完成' };
  const next = applyRunEvent(old, {seq:5, attempt_id:'m2', type:'attempt_replaced', payload:{}});
  assert.equal(next.text, '');
  assert.equal(next.attemptId, 'm2');
});
```

- [ ] 在 frontend 执行 `npm test -- src/utils/agentRunState.test.ts`，确认失败。
- [ ] 新会话显示 builtin/trpc 选择，仅启用受后端能力允许的选项；既有会话显示只读引擎身份。恢复卡展示工具、脱敏参数、时间和原因，分别调用 retry/provide_result/terminate；按钮防重复点击，409 刷新最新 revision。

```ts
if (event.seq <= state.seq) return state;
if (event.type === 'attempt_replaced') {
  return { seq:event.seq, attemptId:event.attempt_id, text:'' };
}
```

- [ ] 普通增量仅应用当前 attempt；reconnect 使用已消费 seq，等待状态重新打开页面仍显示。提供结果是结构化 ToolResult，不只一个“成功”按钮；OAuth/执行前审批卡与不明结果卡区分。
- [ ] 执行聚焦测试、`npm run type-check`、`npm run build`，浏览器验证创建两类会话、等待后刷新、重复决策冲突、半段文本替换。若此时 React 迁移已合入，先将本任务文件逐项映射到当前客户端并提交计划修订，不新建第二套 UI。
- [ ] 通过后提交 `feat(web): add trpc run recovery controls`。

### Task 14：真实崩溃验收与启用门禁

**Files:** 创建 `internal/agent/recoverytest/harness_test.go`、`process_test.go`、`http_test.go`、`.github/workflows/agent-recovery.yml`、`docs/superpowers/plans/trpc-recovery-acceptance.md`；修改 `internal/container/agent_runtime.go` 的功能门禁。

**Interfaces:** 测试 harness 在独立进程启动真实 Run service/graph/saver，外部副作用服务由父进程持有，数据库使用文件 SQLite 或隔离 PostgreSQL schema。测试 helper 提供 `runCrashCase(t *testing.T, point string) CrashReport`；`CrashReport {ExternalCalls int; FinalStatus string; AssistantRows int; LostEvents int}`。子进程通过管道报告 barrier，由父进程 `Process.Kill()`；不让测试故障点成为生产可远程调用接口。

- [ ] 写结果已提交后杀进程的验收测试；Task 01 的重开对象测试不能替代这里的 SIGKILL。

```go
func TestCrashAfterToolResult(t *testing.T) {
    r := runCrashCase(t, "after_tool_result_before_checkpoint")
    require.Equal(t, 1, r.ExternalCalls)
    require.Equal(t, "succeeded", r.FinalStatus)
    require.Equal(t, 1, r.AssistantRows)
    require.Zero(t, r.LostEvents)
}
```

- [ ] 执行 `GOWORK=off go test ./internal/agent/recoverytest -run TestCrash -count=1`，先记录真实缺口；harness 创建/清理资源只能限于测试命名空间。
- [ ] 实现 barrier：admission 后、plan 后、外部副作用后结果前、result 后 checkpoint 前、waiting 后、finalize 后。未知写入 case 断言调用 1 次且 waiting_user，用户明确 retry 后计数才变 2；幂等提供商重发请求但副作用计数仍 1。
- [ ] 两 Worker 使用真实 DB 并发；暂停旧 Worker 至 lease 过期，启动接管后恢复旧 Worker，验证旧 epoch checkpoint/result 拒写、不重复启动 unknown 工具。
- [ ] API 进程重启后重连与决策、权限撤销、取消/删除、预算不重置、SDK/schema 不兼容均有黑盒断言。Sandbox 用存活/失联/销毁三类 fixture；具体 Docker/Cube/E2B 实网结果单列，未运行组合不写“通过”。
- [ ] CI 执行 SQLite 全矩阵和 PostgreSQL 服务矩阵，前端聚焦测试与 type-check；聚焦 Go 包执行 `go test -race`。CI 没有云 Sandbox 凭据时只报告其契约测试通过，不代表云实网验收。
- [ ] 功能门禁验证：disabled 时拒绝新 tRPC 会话/运行；unsupported dialect 拒绝启用；关闭 admission 仍允许已存在 Run 被受控 drain，不能回退 builtin 执行 tRPC checkpoint。数据库回滚不得删除非终态数据；升级前检查 graph/schema 兼容，保留旧版本 worker 或暂停受影响运行。
- [ ] 验收文档逐项记录 OS、DB/SDK/graph 版本、命令、预期/实测、外部计数、提交 SHA。更新账本后提交 `test(agent): verify crash recovery and gate rollout`。只在真实矩阵通过后标记功能完成。

## 规格覆盖自查

| 规格章节 | 实施任务 |
|---|---|
| 1/3 范围与选型 | 全局约束、01、02、14 |
| 4 共享能力与隔离 | 02、06、10 |
| 5 会话与受理 | 02、03、07、12 |
| 6 数据契约、快照、权限 | 03、04、05、08、11、12 |
| 7 图与 checkpoint | 01、04、06、10 |
| 8 工具与用户等待 | 05、08、12、13 |
| 9 租约与接管 | 03、07、14 |
| 10 Sandbox | 09、14 |
| 11 事件、决策、取消、steering | 08、11、12、13 |
| 12 验收 | 每任务聚焦测试、14 黑盒矩阵 |

## 执行交接

计划自查完成后，选择逐任务子代理实施与双阶段审查，或在当前会话按 executing-plans 顺序实施。两种模式均使用隔离 worktree，保留本账本；未执行的任务均保持 pending。设计批准不等同任何恢复能力已实现或已验收。
