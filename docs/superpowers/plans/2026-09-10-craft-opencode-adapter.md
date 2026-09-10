# Craft P1 OpenCode Adapter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现独立 Go OpenCode Adapter，以可控 HTTP/SSE 测试服务和固定版本真实 OpenCode 验证委派执行；不接管现有聊天或 React 迁移。

**Architecture:** `internal/craft/delegation` 定义内部契约，`internal/craft/opencode` 隔离 OpenCode wire 协议。产品持久化和 tRPC-Agent-Go 工具接线在后续阶段，P1 不宣称具备生产恢复与前端闭环。

**Tech Stack:** 当前仓库 Go 工具链、net/http、encoding/json、bufio、httptest；OpenCode 版本由 T01 验证后锁定。P1 不添加 Eino、tRPC-Agent-Go 或前端依赖。

**Spec:** [Craft 双 Runtime 规格](../specs/2026-09-10-craft-runtime-design.md)，重点第 5–8、10 节。

## Global Constraints

- tRPC-Agent-Go 是主 Agent Runtime，拥有主 Agent 的推理、上下文、工具选择和执行循环。
- 主 Agent 通过 OpenCode Agent Adapter 委派任务；OpenCode 在沙箱中拥有独立的编码执行循环。
- 产品后端管理用户可见的会话、Run、权限、审批和产物；两个 Runtime 的内部状态不替代产品权威记录。
- 首版同一工作区串行执行。
- 执行完成保留工作区与 session；显式会话删除或保留策略触发清理，不能照搬 wanwu AfterRun 删除行为。
- SSE EOF：只说明连接结束，不能当作成功；重新查询消息和状态，无法证明结果则保持 reconciling。
- 子任务 succeeded：只结束该 delegation；主 Agent 仍可检查产物或继续委派，只有主 Runtime 的结果能结束主 Run。

## 执行边界

当前只生成计划。实施从独立工作树开始，核对 HEAD 与脏文件；不暂存或覆盖 `2026-09-10-react-*` 和其他任务的文件。每个任务记录命令、退出码与 fixture/运行证据，完成范围内提交，不自动推送。使用 [进度记录](craft-runtime-progress.md)。

T01 → T02 → T03 → T04 → T05。T01 是协议事实门槛，后面的方法名是本项目内部接口，并非声称上游已经提供同名 SDK。HTTP 端点和 payload 以 T01 冻结的 profile 实现，不能根据未锁定 main 猜测兼容性。

## T01：冻结 OpenCode 协议与参考证据

**文件**

- 新建 `docs/superpowers/specs/2026-09-10-craft-opencode-wire-profile.md`。
- 新建 `internal/craft/opencode/testdata/profile.json` 与 `testdata/events/*.json`。
- 新建 `internal/craft/opencode/profile_test.go`。
- 只读参考 wanwu `pkg/wga-sandbox/internal/runner/opencode/opencode.go`、`api_question.go`、`pkg/ag-ui-util/translator_opencode.go`。

**消费**：规格第 6–8 节；选定发行版的 OpenAPI、版本命令输出和脱敏实测事件。

**产出**：唯一固定版本 profile，包括二进制版本、来源提交/发行版、接口与事件样本；记录 session 创建/查询、prompt_async、SSE、status、abort、question/permission、最终消息查询是否可用。

- [ ] 读取选定版本官方 schema 与服务端源码，记录以上操作的 verb/path/request/response/错误码。确定是否存在服务端幂等键、事件游标和 message correlation，不支持的项明确写 `unsupported`。
- [ ] 添加 fixture 自检，保证基础 profile 不缺版本和 fixture，而不是先生成伪造上游事件：

```go
package opencode

import (
    "encoding/json"
    "os"
    "path/filepath"
    "testing"
)

func TestWireProfileComplete(t *testing.T) {
    raw, err := os.ReadFile("testdata/profile.json")
    if err != nil { t.Fatal(err) }
    var p struct {
        Version string `json:"version"`
        Fixtures []string `json:"fixtures"`
    }
    if err := json.Unmarshal(raw, &p); err != nil { t.Fatal(err) }
    if p.Version == "" || len(p.Fixtures) < 6 { t.Fatal("incomplete wire profile") }
    for _, name := range p.Fixtures {
        data, err := os.ReadFile(filepath.Join("testdata", name))
        if err != nil { t.Fatal(err) }
        if !json.Valid(data) { t.Fatalf("invalid fixture %s", name) }
    }
}
```

- [ ] 运行 `go test ./internal/craft/opencode -run TestWireProfileComplete -count=1`，缺 fixture 时应失败。
- [ ] 在隔离测试沙箱捕获创建、文本、工具、idle、error、abort 的至少六类真实脱敏响应/事件；问题和权限分别记录支持与样本。不得提交模型凭证或用户文件内容。
- [ ] 重新运行自检并人工比对 schema/样本；fixture 自检成功只证明文件完整，协议真实性需要来源记录和真实调用输出。
- [ ] 提交仅本任务文件。若无法访问真实 OpenCode，将任务标记 blocked 并记录原因；可编写后续纯契约，但不能标记真实兼容验收通过。

## T02：内部身份、状态与事件契约

**文件**：新建 `internal/craft/delegation/types.go`、`identity.go`、`state.go`、`identity_test.go`、`state_test.go`。

**消费**：规格身份与终态规则，不消费 OpenCode SDK 类型。

**产出**：以下命名接口，所有导出类型由本任务定义；后续不能重复创建近似结构。

```go
type Identity struct {
    TenantID uint64
    ConversationID, RunID, DelegationID, WorkspaceID string
    SandboxGeneration uint64
}
func (id Identity) Validate() error

type Status string
const (
    Queued Status = "queued"
    Running Status = "running"
    WaitingInput Status = "waiting_input"
    WaitingApproval Status = "waiting_approval"
    Cancelling Status = "cancelling"
    Reconciling Status = "reconciling"
    Succeeded Status = "succeeded"
    Failed Status = "failed"
    Cancelled Status = "cancelled"
)
func CanTransition(from, to Status) bool

type Request struct {
    Identity Identity
    Task string
    InputRefs []string
    Deadline time.Time
}
type Verification struct { Name, Status, EvidenceRef string }
type Usage struct {
    CallID, Provider, Model string
    InputTokens, OutputTokens, CachedTokens int64
}
type Result struct {
    DelegationID string
    Status Status
    Summary string
    ArtifactRefs []string
    Verifications []Verification
    Usage []Usage
}
type Event struct {
    Version int
    Identity Identity
    Kind, SourceID string
    Payload json.RawMessage
}
type Sink func(context.Context, Event) error
type InteractionResponse struct {
    InteractionID, Kind string
    Answers [][]string
    Decision string
}
type Executor interface {
    Execute(context.Context, Request, Sink) (Result, error)
    Cancel(context.Context, Identity) error
    Respond(context.Context, Identity, InteractionResponse) error
}
```

导入 `context`、`encoding/json`、`time`。这里的 Event 是 Adapter 到应用层的事件；持久化 EventID/Sequence 由 P2 发布层分配，不制造伪游标。Identity 的六项均必填且非零，字符串空白无效。

- [ ] 添加表驱动测试，每次只清空一个 Identity 字段，全部应报错；完整 Identity 通过。
- [ ] 添加终态吸收测试：

```go
func TestTerminalStateCannotRestart(t *testing.T) {
    for _, s := range []Status{Succeeded, Failed, Cancelled} {
        if CanTransition(s, Running) { t.Fatalf("restarted %s", s) }
    }
    if !CanTransition(Running, Reconciling) { t.Fatal("lost ambiguous state") }
    if CanTransition(Cancelling, Succeeded) { t.Fatal("cancel became success") }
}
```

- [ ] 运行 `go test ./internal/craft/delegation -count=1`，先确认编译/断言失败。
- [ ] 实现 Validate 与显式状态迁移表：queued→running/failed/cancelled；running→等待态/cancelling/reconciling/succeeded/failed；等待态→running/cancelling/reconciling/failed；cancelling→cancelled/reconciling；reconciling→running/等待态/cancelling/三个终态；同态幂等，未知状态拒绝。
- [ ] 运行上述测试；增加 reconciling 转移需有上层核对证据的文档说明，状态函数本身不替代授权/核对。
- [ ] 提交本任务文件并更新进度证据。

## T03：HTTP 客户端、连接建立与 session 复用

**文件**：新建 `internal/craft/opencode/client.go`、`session.go`、`stream.go`、`client_test.go`、`stream_test.go`。

**消费**：T01 profile；标准库 `*http.Client`。

**产出**：包内项目接口（参数中的 endpoint 只能由可信沙箱解析器提供）：

```go
type Client struct { config ClientConfig }
type ClientConfig struct {
    BaseURL, Directory, Username, Password string
    HTTP *http.Client
}
type Session struct { ID string }
type Snapshot struct {
    SessionID, ActiveMessageID, State string
    Message json.RawMessage
}
type WireStream interface {
    Next(context.Context) (json.RawMessage, error)
    Close() error
}
func NewClient(ClientConfig) (*Client, error)
func (c *Client) EnsureSession(context.Context, string) (Session, error)
func (c *Client) OpenEvents(context.Context) (WireStream, error)
func (c *Client) Prompt(context.Context, string, string) error
func (c *Client) Snapshot(context.Context, string) (Snapshot, error)
func (c *Client) Abort(context.Context, string) error
```

导入 `context`、`encoding/json`、`net/http`。`EnsureSession(ctx, existingID)` 仅空 ID 创建；非空验证存在，不存在返回明确错误，不悄悄创建新会话。P2 持久化层提供 existingID。

- [ ] 用 `httptest.Server` 编写复用会话和失败状态码测试。测试服务记录 POST /session 次数，同一既有 ID 再次 EnsureSession 必须保持 0；查询失败应返回错误，不创建替代 session。
- [ ] 添加握手测试：测试服务发出 SSE 响应头并 Flush 后才允许 OpenEvents 返回；不接受 content-type 的响应立即失败；401/403 不进入事件解析。
- [ ] 运行 `go test ./internal/craft/opencode -run 'Test(Client|Stream|EnsureSession)' -count=1`，确认失败。
- [ ] 按 T01 schema 实现请求和响应校验，directory 使用 URL 编码；HTTP 非 2xx 报错，错误正文截断脱敏。禁止打印 ClientConfig 密码。禁止配置 InsecureSkipVerify。
- [ ] 实现 SSE 分帧：支持跨 read、CRLF、多行 data、注释；事件最大 1 MiB，超过限制返回明确错误；不以 Scanner 默认 64 KiB 偶然截断。断流返回 EOF，不返回合成成功事件。
- [ ] 配置握手与普通请求有界超时，持续流不套全局短超时；ctx 取消关闭响应体并解除阻塞。Prompt 超时返回可识别的“不确定提交”错误，不自动重试。
- [ ] 运行测试及 `go test -race ./internal/craft/opencode -count=1`；复用 session、握手失败、长帧、读取消、401、提交超时均需断言。
- [ ] 提交本任务文件及记录。

## T04：来源过滤、事件标准化与终态核对

**文件**：新建 `internal/craft/opencode/normalize.go`、`normalize_test.go`、`terminal.go`、`terminal_test.go`。

**消费**：T01 真实 fixture、T02 Event/Identity/Status、T03 Snapshot。

**产出**：

```go
type Normalizer struct {
    identity delegation.Identity
    sessionID string
    partSnapshots map[string]json.RawMessage
}
func NewNormalizer(delegation.Identity, string) *Normalizer
func (n *Normalizer) Push(json.RawMessage) ([]delegation.Event, error)
func VerifyTerminal(submittedMessageID string, snapshot Snapshot) (delegation.Status, error)
```

Normalizer 绑定 Identity + OpenCode session ID。Push 只输出内部事件，不写数据库或创建主 Run 终态。归属不明的控制事件不得路由到当前任务。

- [ ] 使用 T01 fixture 构造另一 session、旧 generation 的 envelope、重复工具快照、同一文本累计快照与 delta 混用测试。跨 session 输出为空，重复快照不重复累计文本。
- [ ] 验证终态不得由裸 idle 推断：

```go
func TestIdleWithoutMessageIsUncertain(t *testing.T) {
    got, err := VerifyTerminal("submitted-message", Snapshot{
        SessionID: "session", State: "idle",
    })
    if err == nil || got != delegation.Reconciling {
        t.Fatalf("idle was treated as completion: %s %v", got, err)
    }
}
```

- [ ] 运行 `go test ./internal/craft/opencode -run 'Test(Normalize|Idle|Terminal)' -count=1`，确认失败。
- [ ] 实现文本、工具、交互、错误与 usage 的字段映射。模型 usage 无稳定 call ID 时标记不可可靠归并，不用随机 ID 假装可去重。
- [ ] 实现最终消息与本轮关联核对；只有 T01 已证实的成功完成标志与对应消息存在时返回 succeeded。error、取消和工具失败按实际最终结果区分；没有提交消息关联证据返回 reconciling。
- [ ] 在提交前保存消息基线，结合该 session 串行执行约束识别本轮新用户消息及其 assistant 响应；若 T01 支持客户端 message ID 则优先使用。`submittedMessageID` 必须来自该关联证据，不得凭最新消息或任意第一条 SSE 推断。
- [ ] 对未知事件仅记录类型；必需控制类型不支持则显式兼容错误。不得从未检查的 payload 中直接生成产物下载地址。
- [ ] 重新运行测试；补成功最终消息、旧消息 idle、error 与 EOF 样本，确认没有任何路径生成主 RUN_FINISHED。
- [ ] 提交本任务文件及证据。

## T05：可取消的委派执行器与真实协议验收

**文件**：新建 `internal/craft/opencode/executor.go`、`executor_test.go`、`executor_integration_test.go`。

**消费**：T02 Executor、T03 Client、T04 Normalizer/VerifyTerminal。

**产出**：一个绑定工作区身份的执行器实现。P1 仅用内存生命周期承载测试；跨进程幂等、锁、持久 session 和审批属于 P2，不以此实现对外路由。

```go
type ExecutorConfig struct {
    Client *Client
    Identity delegation.Identity
    ExistingSessionID string
    AbortTimeout time.Duration
}
func NewExecutor(ExecutorConfig) (delegation.Executor, error)
```

- [ ] 先写以下命名测试，各自用 httptest 模拟 wire 操作而不是调用模型：`TestExecuteSubscribesBeforePrompt`、`TestExecuteRetainsSession`、`TestExecuteEOFReconciles`、`TestExecutePromptTimeoutDoesNotResubmit`、`TestCancelCallsRemoteAbort`、`TestCancelWrongIdentityRejected`、`TestSinkFailureDoesNotSucceed`。
- [ ] 测试通过 HTTP 请求日志断言：提交前完成事件握手；成功后不存在 DELETE session；EOF 调用快照核对；不确定提交最多一次 prompt；Cancel 实际调用 abort 且超时未证明停止时不报告 cancelled。
- [ ] 运行 `go test ./internal/craft/opencode -run 'Test(Execute|Cancel|Sink)' -count=1`，确认失败。
- [ ] 按以下顺序实现 Execute：Validate → 匹配绑定身份 → 获取本执行器活跃锁 → EnsureSession → OpenEvents → Prompt → 消费并写 Sink → 最终快照核对 → 返回 Result；所有退出分支关闭流并释放锁，保留 session。
- [ ] 相同执行器第二个活跃 Execute 返回冲突；不同 Identity 的 Cancel/Respond 拒绝。Cancel 使用独立且最长 AbortTimeout 的上下文执行 abort，随后查询远端。不能用已经取消的 Execute ctx 发送停止命令。
- [ ] P1 Respond 返回明确 unsupported 错误；遇到 question/permission 发出交互事件并请求停止，返回非成功状态，不自动回复或默许执行。P2 完成持久化人工交互后才启用这两种能力。该限制必须进入阶段验收记录。
- [ ] 添加 `//go:build integration` 的真实协议测试：从测试专用环境读取 endpoint/凭证，固定 profile 版本；连续两次修改同一测试文件，确认 session 未删除；调用 abort 并查询停止。禁止记录凭证，集成测试清理由测试自身显式删除测试资源。
- [ ] 运行 `go test ./internal/craft/... -count=1` 和 `go test -race ./internal/craft/... -count=1`。
- [ ] 配置测试沙箱后运行 `go test -tags=integration ./internal/craft/opencode -run TestOpenCodeLifecycle -count=1 -v`。测试函数必须命名 `TestOpenCodeLifecycle`；环境缺失应 Skip 并记录为未验收，不能当作通过。
- [ ] 更新进度：记录上游版本、fixture 来源、命令退出码、真实 lifecycle 证据和限制；范围内提交。

## P1 完成定义与后续交接

P1 accepted 要求契约测试、race、固定版本真实 lifecycle 均通过；实现只通过 fake server 则状态为 review，不写兼容已验收。P1 仍不代表双 Runtime 产品完成。

P2 的输入是 P1 Executor 和 wire profile；新增数据库委派记录、session 映射、幂等锁、事件序号、审批与恢复扫描，并把 P1 unsupported Respond 替换成持久化交互流程。P3 再冻结 tRPC-Agent-Go API 并注册委派工具，P4 对接 React 迁移共享包，P5 做真实恢复/隔离与发布。分期验收见规格第 10 节。

## 计划自检

- [x] P1 覆盖 session、HTTP/SSE、归属过滤、终态核对、取消与明确交互限制。
- [x] 产品持久化/主 Runtime/React/生产恢复明确属于 P2–P5，未伪报覆盖。
- [x] 类型与方法在消费任务之前定义，外部协议由 T01 固定。
- [x] 测试区分 fixture 自检、mock 协议测试、真实兼容验收。
- [x] 不修改 React 迁移文档、现有聊天实现、根依赖或相邻 wanwu 仓库。
