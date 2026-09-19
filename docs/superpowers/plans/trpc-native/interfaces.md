# 原生 Agent 业务接口 v1（P0-4）

2026-09-19；源码基线 `ba6dfcc69f167ef6250aa983484930b462ce0900`。这是后续 P1–P7 的契约草案交付，不是已实现 API。P0-4 的 `sdk:` 源码核对固定在历史 `trpc.group/trpc-go/trpc-agent-go v1.10.0`；根模块当前固定 `v1.11.0`，但它只有 Task 1 的候选 race slice 证据，尚非获批产品组合。`sdk:` 路径相对于被核对模块根，`repo:` 相对于仓库根。[能力矩阵](sdk-capabilities.tsv) 的 `source-only` 表示只读了源码，`verified` 仅适用于行内命名的具体探针。本文所有新类型均是待实现的 `nativecontract` v1，不能冒充当前包导出类型。

**执行门仍关闭。** 历史 `v1.10.0` 在 Runner + in-memory Session 的 repeated race gate 上是 RED；Task 1 的当前 `v1.11.0` 候选在同一 `GOWORK=off go test -race ./internal/agent/nativeprobe -count=20 -v` 命令是 GREEN（零退出、无 race report）。这只排除了该候选的已复现 SDK race，不能解除整体 **NO-GO**。`internal/application/service` 的预算耗尽通知 consumer 在当前候选和精确 `75523c9c9` 历史基线都失败，故它是既有失败而非候选回归；直接 consumers 因此尚未完整验收。Session/Memory 持久化组合、Session append 错误传播、逐工具恢复边界、SQLite/PostgreSQL 持久化、真实 Provider 与客户端仍须分别验收。文档交付可完成，不能据此解锁实现接线或发布。

## 阶段责任映射

本契约与清单使用总计划的固定责任：P1 是数据存储与唯一权威；P2 是治理、准入、审批、工具副作用与预算；P3 是原生 Runner、模型、Session 与 Memory 执行链；P6 是只读归档与业务配置迁移；P7 是已集成组合的完整故障/功能验收；P8 是切换演练与具体发布。跨阶段条目只在前置、消费或验收确有责任时列出，不把 P7 或 P8 写成实现所有者。

## 1. 当前类型与权威边界

| 领域 | 当前真实类型/位置 | v1 处理 |
| --- | --- | --- |
| 身份 | `types.Principal`、`TenantIDFromContext`、`PrincipalFromContext`、`SessionOwnerIDFromContext`，`internal/types/{principal,context_helpers}.go`；认证入口 `internal/middleware/auth.go` | tenant 来自服务端认证后的当前空间；actor/RBAC、终端 principal、session owner 分开，不能只取请求 JSON 的 user ID |
| Memory scope | `interfaces.MemoryScope`、`MemoryService`，`internal/types/interfaces/memory.go` | `(TenantID, SubjectID)` 来自认证上下文；保留现有开关、拒绝标记和检索偏好，不把账户 ID 当所有 principal |
| Session | SDK `session.Service/Key/UserKey/Options`，`sdk:session/session.go` | 新执行历史唯一权威；完整 CRUD、状态、历史、summary API 采用该固定接口；UI 读模型可重建 |
| Memory | SDK `memory.Service/Key/UserKey/Entry/Metadata`，`sdk:memory/memory.go` | 新长期记忆唯一权威，不承担审批审计与任务恢复 |
| 执行日志 | `runtime.RunKey/Fence/Run/Admission/Decision/RunEvent`，`internal/agent/runtime/contracts.go` | 保留必要业务职责；v1 新字段见下文，不能把 SDK InvocationID 当租约/授权依据 |
| 工具日志 | `runtime.ToolPlan/ToolAttempt/StoredToolResult/ToolJournal`，`internal/agent/runtime/tool_executor.go` | 旧字段为迁移参考；稳定逻辑调用 ID、独立 attempt、未知效果持有原则保留 |
| 检查点 | SDK `graph.CheckpointSaver/PutFullRequest/CheckpointTuple`；现有 `internal/agent/trpc/checkpoint.go` | 图状态与位置；保存 pending writes、SDK/graph/schema 版本和结果引用，不证明外部效果 |
| 模型 | SDK `model.Model/Request/Response`；旧 `chat.ChatOptions/ChatConfig`，`internal/models/chat/chat.go` | 原生 Model 协议；Provider 参数/鉴权扩展逐项删除旧桥，不复制整个旧 Chat 桥作为最终架构 |
| 工具/Skills | SDK `tool.Tool/CallableTool/StreamableTool`、`skill.Repository`、`codeexecutor.Engine/InteractiveProgramRunner` | 协议原生；授权安装、租户、工作区、审批和耐久外部调用为业务边界 |
| 旧数据 | `types.Session`、现有消息/附件/审批/审计记录；已确认规格 §8 | 独立归档读取，保留历史；不能恢复旧 run 或自动注入新 Session/Memory |

SDK Session service 的数据库实现尚未固定。P0-4 的 v1.10.0 源码核对不覆盖当前 v1.11.0 候选的持久化子模块；Task 1 的 in-memory candidate probe 也不选择 SQLite/PostgreSQL Session 或 Memory backend。P1 必须固定每个子模块的版本/校验和、其 root SDK 依赖与 Session 实例并发语义；也可在既有业务数据库上实现固定 `session.Service` 的最小存储适配器，但需同样通过 race、分页、幂等、故障测试。选型未完成前此处为**明确阻塞**，不能只靠 root `go.mod` 声称 SQL 存储已可用。Memory 持久化实现有相同选型与版本门槛。

## 2. 认证范围与键映射（P1 权威；P2 治理、P3 执行消费）

认证入口已有 JWT、tenant API key、API external user、IM、Embed 等 principal；`types.Principal` 不代表 RBAC 用户。v1 由 `ScopeResolver` 使用当前认证上下文与资源服务生成范围。worker 恢复从准入记录取得身份，再重新验证成员资格、key/channel 状态、resource grants 与策略版本；不能重建一个永久可信的旧 request context。

以下字符串编码均为 UTF-8 bytes 的 base64url 无 padding，记为 `b64(x)`。服务端拒绝空 tenant/owner/subject/session ID，先校验 owner 归属，再构造键：

- `AppName = "weknora/native-v1/tenant/" + tenantID 的十进制`。这里 tenant 即业务空间，不新增与现有 tenant 不一致的第二空间 ID。
- `session.Key.UserID = "owner/" + b64(SessionOwnerID)`；`SessionID = "session/" + b64(服务端生成的新会话 ID)`。owner 采用现有 `SessionOwnerIDFromContext` 的来源规则，API tenant key 必须带 key 维度。
- `memory.UserKey.UserID = "subject/" + b64(MemorySubjectID)`；MemorySubjectID 采用现有 scope resolver 的 subject，而非直接重用 Session owner。Embed 的 OAuth visitor principal 映射继续由 `MCPOAuthPrincipalFromContext` 处理，不能拿 OAuth visitor token scope 扩大 Session 读取范围。
- Session/Memory 的 `AppName` 相同，但数据库表/集合分别归各服务。旧键没有 `native-v1` 命名空间，禁止 fallback 查询或按旧 engine 默认值猜测。
- Checkpoint lineage 为新 RunID；namespace 为 `native-v1/tenant/<tenantID>/run/<b64(runID)>/graph/<b64(graphVersion)>`。envelope 必须校验 SDK、graph 与 schema 版本。配置 hash 相同不代表凭据仍有权访问。

读、列表、删除、summary worker、memory worker、archive/attachment 读取均校验当前 scope。SDK 只检查非空键，不是授权层。App/User state 广域方法只允许相应服务权限，普通模型工具不得取得任意 `AppName` 的 service 原对象。共享 Agent 的资源 grant 可只读；子 Agent grant 为父 grant 与当前授权交集，不能凭名称重新授予。

## 3. 完整 Go 契约定义

以下单一代码块依赖标准库及已固定 SDK，无未定义的业务跨任务类型。字符串 ID 由服务端生成；enum 以后的扩展必须升级 schema 或显式定义兼容语义。`SDKServices` 暴露给内部装配层，面向客户端的方法一律经过认证 facade。类型导出不构成允许客户端填充 scope、fence、price 或凭据的授权。

```go
package nativecontract

import (
	"context"
	"encoding/json"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	"trpc.group/trpc-go/trpc-agent-go/memory"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

const ContractVersion = 1
const EventProtocol = "weknora.agent.v1"

type Principal struct{ Type, ID string }
type ResourceGrant struct{ ResourceType, ResourceID, Action string }
type Scope struct {
	TenantID                         uint64
	ActorUserID                      string // Empty only for authenticated non-account principals.
	Principal                        Principal
	SessionOwnerID, MemorySubjectID  string
	PolicyRevision, MemoryGeneration int64
	Grants                           []ResourceGrant
}
type ScopeResolver interface {
	Resolve(context.Context) (Scope, error)
	Recheck(context.Context, Scope, []ResourceGrant) (Scope, error)
}
type SDKServices struct {
	Session session.Service
	Memory  memory.Service
}
type SessionBinding struct {
	Scope         Scope
	SessionID     string
	Key           session.Key
	MemoryKey     memory.UserKey
	SchemaVersion int
}

type RunStatus string

const (
	RunQueued     RunStatus = "queued"
	RunRunning    RunStatus = "running"
	RunWaiting    RunStatus = "waiting_user"
	RunCancelling RunStatus = "cancelling"
	RunSucceeded  RunStatus = "succeeded"
	RunFailed     RunStatus = "failed"
	RunCancelled  RunStatus = "cancelled"
)

type WaitKind string

const (
	WaitApproval  WaitKind = "tool_approval"
	WaitOAuth     WaitKind = "mcp_oauth"
	WaitConnector WaitKind = "connector_approval"
	WaitUnknown   WaitKind = "unknown_result"
)

type RunIdentity struct {
	TenantID                                                  uint64
	SessionID, RunID, ParentRunID, BudgetRootRunID, RequestID string
}
type Fence struct {
	Run        RunIdentity
	Owner      string
	Epoch      int64
	LeaseUntil time.Time
}
type ConfigBinding struct {
	SchemaVersion                                                     int
	SDKVersion, GraphVersion, ConfigHash, ModelID, ModelConfigVersion string
	CredentialRef                                                     string // Reference only; never the credential value.
	CredentialVersion                                                 int64
	ToolSetHash, SkillSetHash, ExecutionTargetID, WorkspaceRef        string
}
type FundingBinding struct {
	BudgetRef, BudgetRootRunID, Source, Funding, Service, PriceVersion string
	CredentialVersion                                                  int64
	MaxUnits                                                           int64
}
type Admission struct {
	Scope     Scope
	Run       RunIdentity
	Session   session.Key
	Config    ConfigBinding
	Funding   FundingBinding
	Input     model.Message
	InputHash string
	Deadline  time.Time
}
type RunRecord struct {
	Admission Admission
	Status    RunStatus
	Wait      WaitKind
	Revision  int64
	Fence     Fence
}
type SteerInput struct {
	InputID, Mode    string
	Message          model.Message
	ExpectedRevision int64
}

// AdmissionControls is server-owned rollout state, never request data.
type AdmissionControls struct {
	Revision                                       int64
	RecoveryEnabled, AdmissionEnabled, WorkerDrain bool
	NativeExecutionApproved, DependenciesReady     bool
}
type AdmissionControlSource interface {
	Current(context.Context) (AdmissionControls, error)
}

type RunControl interface {
	Admit(context.Context, Admission) (RunRecord, error)
	Get(context.Context, Scope, RunIdentity) (RunRecord, error)
	Cancel(context.Context, Scope, RunIdentity, int64) error
	Steer(context.Context, Scope, RunIdentity, SteerInput) error
}

type AttemptKind string

const (
	ModelAttempt AttemptKind = "model"
	ToolAttempt  AttemptKind = "tool"
)

type Attempt struct {
	ID                                             string
	Run                                            RunIdentity
	Kind                                           AttemptKind
	LogicalCallID, InvocationID, ReplacesAttemptID string
	Number                                         int
	Epoch                                          int64
	ProviderRequestID                              string
	StartedAt                                      time.Time
}
type AttemptJournal interface {
	Begin(context.Context, Fence, Attempt) (Attempt, error)
	Finish(context.Context, Fence, string, EffectState, *Failure) error
	Get(context.Context, Scope, RunIdentity, string) (Attempt, error)
}

type RecoveryPolicy string

const (
	RecoveryHold       RecoveryPolicy = "hold"
	RecoveryQuery      RecoveryPolicy = "query"
	RecoveryIdempotent RecoveryPolicy = "idempotent"
	RecoveryReadOnly   RecoveryPolicy = "read_only"
)

type ToolIdentity struct {
	Kind, ServiceID, InstallationID, Name, SchemaHash, ConfigVersion string
}
type ToolPlan struct {
	Version                                    int
	Run                                        RunIdentity
	CallID, ProviderToolCallID, ModelAttemptID string
	Tool                                       ToolIdentity
	Args                                       json.RawMessage
	ArgsHash                                   string
	RequiredGrants                             []ResourceGrant
	Policy                                     RecoveryPolicy
	IdempotencyKey                             string
	IdempotencyExpiresAt                       time.Time
}
type UserDecision struct {
	Reason, ResourceRef           string
	Version                       int
	DecisionID, PendingID, CallID string
	Run                           RunIdentity
	Actor                         Principal
	PlanVersion                   int
	ArgsHash, Action              string
	ExpectedRevision              int64
	ExpiresAt                     time.Time
	ProvidedResult                json.RawMessage
}
type DecisionAction string

const (
	DecisionRetry         DecisionAction = "retry"
	DecisionProvideResult DecisionAction = "provide_result"
	DecisionTerminate     DecisionAction = "terminate"
)

type PendingStatus string

const (
	PendingOpen      PendingStatus = "pending"
	PendingResolved  PendingStatus = "resolved"
	PendingExpired   PendingStatus = "expired"
	PendingRevoked   PendingStatus = "revoked"
	PendingCancelled PendingStatus = "cancelled"
)

type PendingKey struct {
	Run       RunIdentity
	PendingID string
}
type PendingReference struct {
	PendingID  string `json:"pending_id"`
	DetailPath string `json:"detail_path"`
	Revision   string `json:"revision"`
}
type PendingServiceIdentity struct {
	Kind               string `json:"kind"` // builtin, mcp, connector, sandbox, delegate
	ServiceID          string `json:"service_id,omitempty"`
	ServiceName        string `json:"service_name"`
	InstallationID     string `json:"installation_id,omitempty"`
	ResourceRef        string `json:"resource_ref,omitempty"`
	ToolName           string `json:"tool_name"`
	RegisteredToolName string `json:"registered_tool_name"`
	SchemaHash         string `json:"schema_hash"`
}
type PendingOAuth struct {
	ServiceID string     `json:"service_id"`
	State     string     `json:"state"` // required, pending, authorized, expired, revoked
	BeginPath string     `json:"begin_path"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}
type PendingDecisionDetail struct {
	Version              int                    `json:"version"`
	Ref                  PendingReference       `json:"ref"`
	SessionID            string                 `json:"session_id"`
	RunID                string                 `json:"run_id"`
	CallID               string                 `json:"call_id"`
	WaitKind             WaitKind               `json:"wait_kind"`
	Status               PendingStatus          `json:"status"`
	RunStatus            RunStatus              `json:"run_status"`
	RunRevision          string                 `json:"run_revision"`
	PlanVersion          int                    `json:"plan_version"`
	ArgsHash             string                 `json:"args_hash"`
	Service              PendingServiceIdentity `json:"service"`
	OperationDescription string                 `json:"operation_description"`
	RedactedArgs         json.RawMessage        `json:"redacted_args"`
	RedactedPaths        []string               `json:"redacted_paths"`
	RedactionVersion     string                 `json:"redaction_version"`
	ExpiresAt            time.Time              `json:"expires_at"`
	AllowedActions       []DecisionAction       `json:"allowed_actions"`
	ResolvePath          string                 `json:"resolve_path"`
	OAuth                *PendingOAuth          `json:"oauth,omitempty"`
	ExternalActionID     string                 `json:"external_action_id,omitempty"`
	ExternalActionPath   string                 `json:"external_action_path,omitempty"`
	ExternalActionState  string                 `json:"external_action_state,omitempty"`
	ResolvedDecisionID   string                 `json:"resolved_decision_id,omitempty"`
	ResolvedAction       DecisionAction         `json:"resolved_action,omitempty"`
	ResolvedAt           *time.Time             `json:"resolved_at,omitempty"`
}
type PendingDecisionPage struct {
	Items      []PendingDecisionDetail `json:"items"`
	NextCursor string                  `json:"next_cursor,omitempty"`
}
type ResolvePendingRequest struct {
	DecisionID       string          `json:"decision_id"`
	CallID           string          `json:"call_id"`
	ExpectedRevision string          `json:"expected_revision"` // Run revision, decimal int64.
	PendingRevision  string          `json:"pending_revision"`
	PlanVersion      int             `json:"plan_version"`
	ArgsHash         string          `json:"args_hash"`
	ResourceRef      string          `json:"resource_ref,omitempty"`
	Action           DecisionAction  `json:"action"`
	Reason           string          `json:"reason"`
	ProvidedResult   json.RawMessage `json:"provided_result,omitempty"`
}
type PendingResolution struct {
	Detail      PendingDecisionDetail `json:"detail"`
	RunStatus   RunStatus             `json:"run_status"`
	RunRevision string                `json:"run_revision"`
	ResumeState string                `json:"resume_state"` // queued, held, terminated; never implies dispatch completed.
}
type OAuthStartRequest struct {
	RedirectURI string `json:"redirect_uri"`
}
type OAuthStartResult struct {
	AuthorizationURL     string    `json:"authorization_url"`
	AuthorizationAttempt string    `json:"authorization_attempt"`
	ExpiresAt            time.Time `json:"expires_at"`
}
type PendingDecisionService interface {
	List(context.Context, Scope, RunIdentity, string, int) (PendingDecisionPage, error)
	Get(context.Context, Scope, PendingKey) (PendingDecisionDetail, error)
	BeginOAuth(context.Context, Scope, PendingKey, OAuthStartRequest) (OAuthStartResult, error)
	Resolve(context.Context, Scope, PendingKey, ResolvePendingRequest) (PendingResolution, error)
}

type EffectState string

const (
	EffectNotDispatched EffectState = "not_dispatched"
	EffectConfirmed     EffectState = "confirmed"
	EffectUnknown       EffectState = "unknown"
)

type ArtifactRef struct {
	ID        string `json:"id"`
	MediaType string `json:"media_type"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"size_bytes"`
}
type ToolOutcome struct {
	AttemptID       string          `json:"attempt_id"`
	CallID          string          `json:"call_id"`
	ProviderReceipt string          `json:"provider_receipt,omitempty"`
	QueryAnchor     string          `json:"query_anchor,omitempty"`
	ResultHash      string          `json:"result_hash"`
	Effect          EffectState     `json:"effect"`
	IsError         bool            `json:"is_error"`
	Truncated       bool            `json:"truncated"`
	Content         json.RawMessage `json:"content"`
	Artifacts       []ArtifactRef   `json:"artifacts,omitempty"`
	Failure         *Failure        `json:"failure,omitempty"`
}
type ToolBoundary interface {
	Plan(context.Context, Fence, ToolPlan) (ToolPlan, error)
	Decide(context.Context, Scope, UserDecision) error
	Wrap(context.Context, Scope, ToolIdentity, tool.Tool) (tool.Tool, error)
	LookupResult(context.Context, Scope, RunIdentity, string) (ToolOutcome, error)
}

type ErrorCode string

const (
	ErrUnauthorized    ErrorCode = "unauthorized"
	ErrForbidden       ErrorCode = "forbidden"
	ErrInvalid         ErrorCode = "invalid_request"
	ErrConflict        ErrorCode = "conflict"
	ErrNotFound        ErrorCode = "not_found"
	ErrLeaseLost       ErrorCode = "lease_lost"
	ErrCancelled       ErrorCode = "cancelled"
	ErrDeadline        ErrorCode = "deadline_exceeded"
	ErrBudget          ErrorCode = "budget_exhausted"
	ErrProvider        ErrorCode = "provider_error"
	ErrIncomplete      ErrorCode = "incomplete_stream"
	ErrUnknownEffect   ErrorCode = "unknown_effect"
	ErrStore           ErrorCode = "durable_store_unavailable"
	ErrCheckpoint      ErrorCode = "checkpoint_incompatible"
	ErrCursor          ErrorCode = "cursor_expired"
	ErrArchiveReadOnly ErrorCode = "archive_read_only"
	ErrUpgrade         ErrorCode = "client_upgrade_required"
)

const (
	ErrAdmissionClosed ErrorCode = "admission_closed"
	ErrExecutionGate   ErrorCode = "execution_gate_closed"
)

type Failure struct {
	Code      ErrorCode   `json:"code"`
	Message   string      `json:"message"` // Sanitized; never raw provider secrets.
	Retryable bool        `json:"retryable"`
	Effect    EffectState `json:"effect"`
	AttemptID string      `json:"attempt_id,omitempty"`
}

func (f Failure) Error() string { return string(f.Code) + ": " + f.Message }

type ModelBinding struct {
	Model   model.Model
	Config  ConfigBinding
	Funding FundingBinding
}
type ModelResolver interface {
	Resolve(context.Context, Scope, ConfigBinding) (ModelBinding, error)
}

type UsageObservation struct {
	Version                                          int
	Run                                              RunIdentity
	AttemptID, ObservationID, ProviderRequestID      string
	Funding                                          FundingBinding
	Revision                                         int64
	PromptTokens, CompletionTokens, TotalTokens      int64
	CachedTokens, CacheReadTokens, CacheCreateTokens int64
	AccountingStatus                                 string // known, partial, unknown
	Dimensions                                       map[string]int64
	OccurredAt                                       time.Time
}
type UsageLedger interface {
	Observe(context.Context, Fence, UsageObservation) error
}

type EventKind string

const (
	EventRunStatus        EventKind = "run.status"
	EventAttemptStarted   EventKind = "attempt.started"
	EventAttemptReplaced  EventKind = "attempt.replaced"
	EventAttemptFinished  EventKind = "attempt.finished"
	EventTextDelta        EventKind = "text.delta"
	EventReasoningDelta   EventKind = "reasoning.delta"
	EventToolPlanned      EventKind = "tool.planned"
	EventToolResult       EventKind = "tool.result"
	EventDecisionRequired EventKind = "decision.required"
	EventUsage            EventKind = "usage.observed"
	EventArtifact         EventKind = "artifact.available"
	EventFailure          EventKind = "error"
)

type PublicUsage struct {
	ObservationID     string `json:"observation_id"`
	Revision          string `json:"revision"`
	PromptTokens      string `json:"prompt_tokens"`
	CompletionTokens  string `json:"completion_tokens"`
	TotalTokens       string `json:"total_tokens"`
	CachedTokens      string `json:"cached_tokens"`
	CacheReadTokens   string `json:"cache_read_tokens"`
	CacheCreateTokens string `json:"cache_create_tokens"`
	AccountingStatus  string `json:"accounting_status"`
}
type EventPayload struct {
	Pending           *PendingReference `json:"pending,omitempty"`
	Status            RunStatus         `json:"status,omitempty"`
	Wait              WaitKind          `json:"wait_kind,omitempty"`
	Text              string            `json:"text,omitempty"`
	Offset            *int64            `json:"offset,omitempty"`
	ReplacesAttemptID string            `json:"replaces_attempt_id,omitempty"`
	CallID            string            `json:"call_id,omitempty"`
	PlanVersion       int               `json:"plan_version,omitempty"`
	ToolName          string            `json:"tool_name,omitempty"`
	PendingID         string            `json:"pending_id,omitempty"`
	ArgsHash          string            `json:"args_hash,omitempty"`
	ExpiresAt         *time.Time        `json:"expires_at,omitempty"`
	Outcome           *ToolOutcome      `json:"outcome,omitempty"`
	Usage             *PublicUsage      `json:"usage,omitempty"`
	Artifact          *ArtifactRef      `json:"artifact,omitempty"`
	Failure           *Failure          `json:"failure,omitempty"`
}
type BusinessEvent struct {
	Protocol      string       `json:"protocol"`
	SchemaVersion int          `json:"schema_version"`
	EventID       string       `json:"event_id"`
	TenantID      string       `json:"tenant_id"` // Decimal uint64, no JS precision loss.
	SessionID     string       `json:"session_id"`
	RunID         string       `json:"run_id"`
	ParentRunID   string       `json:"parent_run_id,omitempty"`
	AttemptID     string       `json:"attempt_id,omitempty"`
	Sequence      string       `json:"seq"` // Positive decimal int64 after commit.
	Kind          EventKind    `json:"kind"`
	Payload       EventPayload `json:"payload"`
}
type EventPage struct {
	Events      []BusinessEvent
	NextCursor  string
	MinSequence int64
}
type EventReader interface {
	Read(context.Context, Scope, RunIdentity, string, int) (EventPage, error)
}

type SessionAppend struct {
	Key                        session.Key
	StableEventID, PayloadHash string
	Event                      *event.Event
}
type CheckpointWrite struct {
	SchemaVersion                                  int
	SDKVersion, GraphVersion, Namespace, LineageID string
	Request                                        graph.PutFullRequest
	ResultCallIDs                                  []string
}
type CommitIntent struct {
	Version         int
	ID, PayloadHash string
	Fence           Fence
	Results         []ToolOutcome
	Checkpoint      *CheckpointWrite
	SessionAppends  []SessionAppend
	Events          []BusinessEvent
	Usage           []UsageObservation
	TerminalStatus  RunStatus // Empty unless finalizing.
}
type CommitReceipt struct {
	IntentID     string
	Applied      bool
	LastSequence int64
}
type CommitCoordinator interface {
	Commit(context.Context, CommitIntent) (CommitReceipt, error)
	Reconcile(context.Context, Fence, string) (CommitReceipt, error)
	Barrier(context.Context, Fence, string) error
}

type MemoryJob struct {
	ID                         string
	Scope                      Scope
	SessionKey                 session.Key
	ThroughEventID             string
	Generation, PolicyRevision int64
}
type MemoryGovernance interface {
	SetEnabled(context.Context, Scope, bool) error
	Delete(context.Context, Scope, string) error
	Clear(context.Context, Scope) error
	Enqueue(context.Context, MemoryJob) error
	Execute(context.Context, MemoryJob) error
}
type DelegateRequest struct {
	Parent     Fence
	Child      Admission
	ToolCallID string
}
type DelegateResult struct {
	ChildRunID string
	Status     RunStatus
	Result     ToolOutcome
}
type DelegateService interface {
	Start(context.Context, Scope, DelegateRequest) (RunIdentity, error)
	Result(context.Context, Scope, RunIdentity) (DelegateResult, error)
	Cancel(context.Context, Scope, RunIdentity) error
}

type ArchiveQuery struct {
	SessionID, Kind, Cursor string
	Limit                   int
}
type ArchiveRecord struct {
	ID, SessionID, Kind string
	Data                json.RawMessage // Existing archived record schema; read-only.
	Artifacts           []ArtifactRef
}
type ArchivePage struct {
	Records    []ArchiveRecord
	NextCursor string
}
type ArchiveReader interface {
	List(context.Context, Scope, ArchiveQuery) (ArchivePage, error)
	Read(context.Context, Scope, string) (ArchiveRecord, error)
	ReadArtifact(context.Context, Scope, string) (ArtifactRef, error)
}
```

## 3A. 待决策详情、读取与解决（P1 持久化；P2 治理；P4/P5 消费；P7 验收）

来源是 `internal/agent/approval/gate.go` 的 `PendingRequest`/`OAuthPendingRequest`（服务、工具、说明、参数与资源关联），`internal/application/service/session.go installDurableOAuthPark`（持久 service_id/service_name/mcp_tool/call/hash/resource_ref），`internal/handler/session/agent_run.go ownedRun/PostAgentRunDecision`、`internal/application/service/agent_run_decisions.go ValidateDecision/Resolve` 与 repository `ApplyDecision`（归属、当前策略、revision/hash、幂等消费），以及 `internal/handler/mcp_oauth.go AuthorizeURL/Status/ResolveMCPOAuth`（服务与 principal 的 OAuth 确认）。旧 in-process Gate 只作交互语义来源，不充当新系统的耐久权威。

v1 新路由固定在 `/api/v1/native/sessions/:session_id/runs/:run_id/pending-decisions`：GET 列表映射 List，GET `/:pending_id` 映射 Get，POST `/:pending_id/resolve` 映射 Resolve，POST `/:pending_id/oauth/authorize-url` 映射 BeginOAuth。这是待实现路由契约，不宣称当前 handler 已提供；每个相对 `DetailPath/ResolvePath/BeginPath` 由服务器根据绑定身份生成，不接受模型生成 URL。列表 cursor 按该 run 的持久待决策顺序分页，上限 100；列表及单项都能查询已解决状态，用于断线后同步。

Get/List/BeginOAuth/Resolve 每次从认证 Scope 检查 tenant、Session owner、run/session 关系、待决策归属和当前资源权限；详情对不属于调用者的项返回 not_found，已知资源被撤权返回 forbidden。获准读取不自动获准解决，AllowedActions 由当前策略生成并在 Resolve 再检查。现有 API key/Embed 的 Session owner 与 OAuth principal 区分继续遵守第 2 节。仅持有 pending ID、过去的 SSE 帧或曾为成员不能读取参数或解决等待。

`PendingDecisionDetail` 足以展示当前操作：真实 service ID/name、安装/resource ref、服务内与注册工具名、operation description、完整计划的 ArgsHash/PlanVersion，以及**按当前读取权限生成**的 RedactedArgs（JSON object）、RedactedPaths（JSON Pointer 列表）和 redaction version。不能直接发送原始 journal Args、令牌或密钥。ArgsHash 针对持久原始规范化参数，不针对脱敏显示；必要安全影响无法向该主体说明时，AllowedActions 不包含 retry。工具/服务说明和参数作为不可信显示内容，不解释为客户端命令。Revoked/expired/resolved/cancelled 均可明确显示，不能继续呈现为可批准。

`decision.required` 的 `payload.pending` 为必需的 PendingReference，指向该授权详情；它与 envelope 的 run/session 和既有 pending_id/call_id/plan_version/args_hash 必须一致。事件只作通知，客户端先 GET 详情才展示和操作；不得仅靠历史 SSE 内容批准。无参数权限时不在重放中泄露历史原文。事件 revision 落后时客户端使用最新详情，不擅自修改请求 revision 以重试。Resolve 请求不含 actor/tenant/service/token 或批准人；服务端从 Scope 和持久 pending 绑定生成内部 UserDecision（ToolBoundary.Decide 仅由该服务内部调用）。Reason 必填，DecisionID 在 run 内幂等；同 ID 同载荷返回已保存结果，同 ID 不同载荷 conflict。ExpectedRevision/PendingRevision/PlanVersion/ArgsHash/CallID/ResourceRef 全部与当前绑定匹配并 CAS 消费一次，过期/撤权/策略不可用均 fail closed。

| wait_kind | 展示与允许操作 | Resolve 与恢复条件 |
| --- | --- | --- |
| tool_approval | 展示上述服务/操作/脱敏参数；retry 文案为“批准并继续”，terminate 为“拒绝并结束”；禁止 provide_result | retry 持久记录对精确 plan/hash 的批准后仅将同一 run 排队；dispatch 前重查授权、参数与 fence；不会在 HTTP 处理器内直接调用工具 |
| mcp_oauth | OAuth 非 nil，包含被绑定的 ServiceID、当前授权 state、BeginPath；不会提供 API key/token；retry 文案为“授权完成后继续” | BeginOAuth 固定使用 pending 绑定的 tenant/principal/service，校验 redirect allowlist，并沿用现有单次 state/authorization_attempt；客户端弹窗回调不是完成证据。GET 详情刷新服务端授权状态，Resolve retry 必须再次查询同一 scope/service 的有效令牌；失败保留 pending。授权完成仅解除 OAuth 条件，若还需工具审批则产生独立新 pending，不能视作工具批准 |
| connector_approval | 包含 ExternalActionID/Path/State，详情从当前有权 action 查询；`awaiting_approval` 引导现有 action approve/execute 面，`unknown` 仅引导只读结果核对 | action 生命周期继续由现有业务 action 服务管理，路径依据 `internal/router/routes_app_connectors.go`；retry 只重读同一 logical call/action 的耐久状态，不能批准另一 action 或自动再次 dispatch；未确认仍 held，不把 action wait 当 MCP OAuth |
| unknown_result | 展示操作和结果不明状态；按当前恢复政策列 retry/provide_result/terminate，retry 必须明确说明可能重复外部效果 | retry 是用户针对该 call 的显式新 attempt 授权；provide_result 只接受不超过 1 MiB 的有效 JSON object，与现有 ValidateDecision 一致并按工具结果 schema 校验，来源标记为 user；不可把用户陈述当 Provider 实测。terminate 停止后续执行但不撤销已发生外部效果 |

terminate 适用于所有开放等待（包括前置 OAuth/审批），统一通过受权取消终结事务处理，而不依赖旧 repository 对 planned-wait terminate 的分支限制。Resolve 成功返回最新详情、run revision/status 和 resume_state；queued 表示同一 run 等待 worker，held 表示已记录但因 worker/执行门或外部结果仍不能推进，terminated 表示不再恢复。运行未执行、工具未完成时绝不返回“工具成功”。SDK race 门未解除时不得 BeginOAuth/Resolve 的成功触发产品 native Runner；已有合法等待的决定仍可保存并 held，terminate/read 仍允许。所有批准/拒绝/提供结果审计保留，过期 worker 不得消费决定。P1/P2 做双数据库持久/CAS/幂等与撤权测试，P4 做真实 OAuth/service 绑定、失败/过期/取消、外部 action 核对，P5 做详情展示和重连同步；P7 复验跨进程、双 worker 和所有客户端。

## 3B. 新准入开关与排空（ENG-006；P2 实现，P7 验收）

`RunControl.Admit` 必须读取服务端 `AdmissionControlSource.Current`，不是调用者 Admission 字段。来源对应当前 `config.AgentRecoveryConfig.RecoveryEnabled/RecoveryAdmissionEnabled`（未设置默认 false）、`Config.IsWorkbenchWorkerDraining`，及容器 `AgentRecoveryAdmissionEnabled/ValidateAgentRuntimeConfig`。部署环境输入为 `WEKNORA_AGENT_RECOVERY_ENABLED`、`WEKNORA_AGENT_RECOVERY_ADMISSION_ENABLED`、`WEKNORA_WORKBENCH_WORKER_DRAIN`；解析后的受控配置 revision 才是准入判断来源，不接受客户端覆盖。NativeExecutionApproved 同时要求本文件首段的批准 SDK/Session 配置与 clean race gate，DependenciesReady 要求 run store、checkpoint/journal/事件/Session、模型与执行器所需服务可用；开关打开本身不能绕过这些门槛。

| 服务端状态 | Admit 及现有工作行为 |
| --- | --- |
| 配置缺失/读取失败、RecoveryEnabled=false，或任一必须依赖不可用 | fail closed，零新 run/预算预留/模型或工具 dispatch；返回 admission_closed 或 durable_store_unavailable。Enabled 控制 worker，不等于 admission 开关；admission=true 且 worker=false 为启动配置错误 |
| RecoveryEnabled=true、AdmissionEnabled=false | 拒绝新准入 admission_closed；worker 继续处理已准入工作直至终态/安全等待，允许取消、只读查询、事件重放、外部结果核对和清理 |
| WorkerDrain=true（无论 admission 配置值） | 所有新准入 lane 拒绝 admission_closed，包括新 child/delegate run 与 after-followup；不得用隐式新 run 绕过。既有 run 和已创建 child 在其他安全门通过时继续排空；必要已有等待的 Resolve 可恢复同一 run，不能顺带创建新任务；已准入父任务若必须新建 child，则安全 held 至 gate 重开 |
| 三个开关允许、NativeExecutionApproved=false | 返回 execution_gate_closed，任何 native 产品 dispatch 禁止；不能因 drain 希望完成而越过 race no-go |
| RecoveryEnabled=true、AdmissionEnabled=true、WorkerDrain=false 且所有安全门通过 | 才可执行幂等准入；仍校验当前授权、预算、配置与 session slot；不是无条件接受 |

准入事务的线性化点必须校验配置 revision，配置在提交前转为关闭/drain 则拒绝并释放本次未提交预算预留；P2 实现需协调配置源与 admission commit，不能“请求开始读一次”后跨关闭窗口提交。同 request key 的已提交且同 hash 重复请求允许受权读取原 RunRecord，不算新准入；载荷冲突仍拒绝。关闭准入不屏蔽 Get、PendingDecisionService.Get/List、EventReader、授权归档读取、Cancel/terminate 和受控 cleanup/reconcile；RecoveryEnabled=false 不启动/领取执行工作，已经派发的外部动作仍由受控取消/核对路径确认，不声称停止 worker 已撤销外部效果。Steer inject 仅可追加到仍允许继续的已准入 run；after 输入可保存为未准入意图，但不得创建下一 run，直到 gate 重开。原生执行门关闭时所有会造成 native dispatch 的恢复同样保持 held。

源码依据为 `internal/application/service/agent_run_graph.go submitDurableAgentRun`、`internal/container/agent_runtime.go` 及 `internal/application/service/agent_run_admission_gate_test.go`/`internal/config/agent_recovery_default_test.go`。这是行为契约补齐，不新增这些测试的通过声明。P2 必须测试 nil/default、enabled/admission_enabled 的四种组合、drain 覆盖、新 child/followup lane、配置关闭与提交竞态、幂等请求、读/取消/cleanup 可达和已有 run 排空；P7 在 SQLite/PostgreSQL、重启/双 worker/维护窗口验证“新准入数为零、既有 run 可终结或安全 held、无失租 dispatch”，并保留原生 race no-go。

## 4. ID、错误、重试与用量（P2 治理；P3 产生执行输入）

`RequestID` 在 `(tenant, Session owner, session, client request key)` 内幂等；同键不同 InputHash 返回 conflict。新 RunID 不等于 SDK RequestID，但装配层明确将 SDK `agent.WithRequestID` 绑定 RunID，保留 client request key。`InvocationID` 用于 SDK branch/追踪；业务 Run/Attempt 不从它推导授权。所有模型 dispatch、tool dispatch、failover/hedge 实际请求均需独立持久 attempt；同一次崩溃恢复复用已提交的 attempt/result，不凭新随机 ID 逃过幂等检查。预算根由准入生成，子 run 不能改它。

逻辑 CallID 由 `(tenant, run, model attempt, provider tool-call ID)` 唯一；provider 无 ID 时，在**完整响应计划首次耐久提交**时生成并保存，不在恢复时重新生成。ToolIdentity 固定服务/安装/工具/schema/config 版本；最终参数在所有合法转换后规范化并 SHA-256，审批覆盖该版本/hash。参数重写必须新 PlanVersion 并失效旧审批，禁止 hash 对 A、实际执行 B。幂等 token 是凭据外的 provider 能力，必须有经测试的有效期，不能凭 SDK 注释判定可安全重试。

`ToolBoundary.Wrap` 返回保持 callable/streamable 能力的受控工具。所有原生 MCP、子 Agent 工具、Skills 执行、代码响应处理器与外部 delegate 路径都必须经过相同 dispatch guard。SDK before callbacks 可提前拒绝或复用结果，但回调之后仍可能发生参数或身份变化，所以 journal/approval/当前授权/fence 的最终比对在真实调用前进行。后置 callback 只用于结果提交，不能代替前置准入。未经验证的 SDK tool retry、MCP reconnect 重放、model failover/hedge 不启用。

| 情况 | v1 处理 |
| --- | --- |
| 未认证/无权/撤权 | unauthorized/forbidden；不 dispatch；恢复也重新检查 |
| 同 ID 载荷不符/过期 revision | conflict；拒绝覆盖；审批消费 CAS |
| lease/epoch 不符 | lease_lost；旧 worker 停止后续 dispatch/提交；外部效果另行核对 |
| HTTP 断开 | 不改变 Run 状态；SSE 可重连；显式 Cancel 才产生持久取消请求 |
| ctx cancel/timeout | cancelled/deadline_exceeded；已派发外部动作若无法确认停止则 EffectUnknown，不能直接当工具成功/已撤销 |
| 模型 stream 中途 EOF、reasoning-only Done、未完成工具参数 | incomplete_stream；废弃当前展示 attempt，禁止发布工具计划；已发生用量仍记录 |
| 不可查询、非幂等且结果未知 | unknown_effect + waiting_user；禁止自动重发；批准重试是新的 decision/attempt |
| append/checkpoint/Session store 失败 | durable_store_unavailable；记录可见故障并停止推进；恢复日志/存储本身不可写时 supervisor 报故障且保持准入关闭，不声称已落库 |
| checkpoint 版本/引用不符 | checkpoint_incompatible；隔离并阻止自动恢复，不能把状态当空历史继续 |
| 预算耗尽 | budget_exhausted 耐久 run 终态；已派发动作仍核对，不能伪装工具正常成功 |
| 过期事件 cursor | cursor_expired，明确重新同步；不静默从尾部接入 |

`Retryable` 只表示可进入重试决策，不授权再次派发；必须同时满足恢复策略、预算和新鲜授权。SDK `GenerateContent` 的返回 error 和 `Response.Error` 两层都映射 Failure。用户可见 Message 脱敏；日志保留 run/call/attempt/epoch/intent/错误码和摘要，不存 API key、OAuth token、AppSecret、原始授权头；工具参数与 Provider 元数据按敏感数据策略处理。

`AfterModel` 在 `sdk:internal/flow/llmflow/llmflow.go` 的 response 序列中调用；不是“每次模型调用只触发一次”的结算保证。ObservationID/Revision 对 cumulative usage 做 CAS 更新，账本只收差额；重复 stream、Session 重放、checkpoint 恢复均不重复记费。失败或取消可能已有真实成本；缺失 usage 为 unknown，等待核对。`FundingBinding` 仅由服务端现有商业归属解析，platform/BYOK、价格与 credential 版本不得由模型或 Provider observation 决定。父子预算在同一预算根上原子预留/释放，SDK MaxLLMCalls 不能替代商业预算。

## 5. 六间隙提交协调（P1 定义存储权威；P2 定义工具/准入治理；P3 实现执行链；P7 故障验收）

采用**业务数据库内提交意图 + Session 幂等应用 + 显式推进屏障**契约。SQLite/PostgreSQL 分别验收，不假定 SDK Session、checkpoint、事件天然共享事务。即使物理同库，只有实现证实处于同一事务才能优化合并；否则始终执行以下协议。

1. 在事务中校验当前 fence/lease、计划版本和授权，固化 plan/approval/attempt，再派发。审批/模型计划/日志写失败时外部调用计数必须为零。
2. 外部结果确认后，以 `(run, call, attempt)` 写不可变结果和 CommitIntent。事务同时写 checkpoint 数据（含 pending writes）、result 引用和待应用的 Session append/业务事件/usage 意图；尚未应用的 checkpoint 标记为**不可推进**，不被正常恢复选作可运行头。
3. Session adapter 按 `(AppName, UserID, SessionID, StableEventID)` 唯一应用事件；SDK Event.ID 必须等于 StableEventID，第一次接收时固化其映射，不能恢复时换 ID；同 ID 同 hash 返回成功，同 ID 不同 hash 为 conflict。Session 是唯一历史权威；intent 仅为未完成提交的恢复指令，不提供另一可写历史 API。应用成功持久化每项确认，再将 commit intent 标为 Applied。
4. `Barrier` 必须检查 intent Applied、当前 fence、所有工具 result 引用齐全和 Session 确认；此后 checkpoint 才成为可运行头，业务事件取得单调序号并进入可发送日志，终态才可 Finalize。恢复从未完成 intent 幂等 reconcile，不重派已确认外部动作。文本 delta 也须先提交业务事件后对外发送，但不会把未完成工具参数暴露为可执行计划。
5. SDK Runner `handleEventPersistence` 当前只 log append 错误；`processSingleAgentEvent` 随后仍可能 NotifyCompletion。因此 SDK completion 不能充当上述 barrier。P3 必须用每次运行的不可清除故障 latch、Session 适配确认与下一模型/工具/图节点前置屏障验证“持久化失败后没有后续 dispatch”。若所选原生组合不能保证，该组合保持 incompatible，固定修复版本或回到规格决策，不能靠 goroutine 取消竞速宣称安全。
6. SSE 只读取已提交事件。发送失败重读相同 event ID/sequence；发送成功不等于业务确认，不参与工具结果推断。projection 可重建；trim watermark 外的 cursor 返回 resync。

数据库内部幂等/CAS/租约不构成外部恰好一次。结果成功到落库之间仍是 [recovery-gaps.md](recovery-gaps.md) 的不可消除间隙；未知非幂等动作固定等待用户。两个 worker 同时 reconcile、跨数据库故障、append 失败、kill 在每一提交边界、摘要/Memory 后台任务与 Session 并发都是目标组合验收，而非本次已通过证据。

## 6. 事件 wire 版本与读取约束（P5）

`BusinessEvent` 的 JSON tag 为 v1 wire 权威；SDK event.Version 与业务 SchemaVersion 分离。内嵌 outcome/usage/artifact/failure 同样由本类型的 JSON tag 固定；对外使用 PublicUsage，不能泄露 FundingBinding、凭据引用或未授权 Provider receipt/query anchor。P5 序列化前清除敏感 receipt/anchor 和结果内容，不直接转发任意 SDK JSON。事件含 run/parent/attempt 与序号；tenant/seq 和 PublicUsage 的 int64 计数用十进制字符串避免 JS 精度丢失。`Last-Event-ID` 为 `v1:<b64(runID)>:<seq>`，解析后仍验证当前 scope 和 run；cursor 不是访问令牌。

| kind | 必需 payload / 客户端语义 |
| --- | --- |
| run.status | status；waiting_user 时另有 wait_kind/pending_id；终态唯一 |
| attempt.started / attempt.replaced / attempt.finished | envelope attempt_id；replaced 含 replaces_attempt_id；客户端替换被废弃尝试的文本，不拼接两次回答 |
| text.delta / reasoning.delta | attempt_id、text、offset；offset 必须非 nil（首块为 0），为该 attempt 对应流 UTF-8 bytes 的起点；重复 event_id 不再次追加，缺口必须 resync |
| tool.planned | call_id、plan_version、tool_name；脱敏展示，不暴露完整未授权参数 |
| tool.result | call_id、outcome；IsError/EffectUnknown 与成功分开 |
| decision.required | 必需 pending 引用（pending_id/detail_path/revision）及 call_id、plan_version、args_hash、expires_at、wait_kind；先授权读取第 3A 节完整详情；OAuth 服务由详情绑定；Resolve 返回状态前不乐观显示已批准 |
| usage.observed | usage；允许 partial/unknown；账单不是客户端累加 delta |
| artifact.available | artifact 引用；读取时单独重新授权，事件中不嵌长期签名下载 URL |
| error | failure；持久失败事件不自动等于最终 run 终态 |

客户端列表不缩减：Go/CLI、Web/Desktop/Embed/小程序、Flutter mobile、独立 Expo mobile-next、DSH。完整功能与原生运行证据由 P5/P7 各自交付；本契约不声称任何客户端已经适配。

## 7. Memory、委派与只读归档

Memory switch、成员/key 撤销、clear/delete 均递增范围 generation 或写对应 tombstone；enqueue、读取 transcript/抽取、最终写入都必须重新检查。旧 job 在删除/关闭之后提交应失败，重新开启也不能自动复活旧 generation 的抽取结果。SDK `Tools()`、`EnqueueAutoMemoryJob` 必须走同一受控 service，不能只在 UI 隐藏开关。长期记忆存入 SDK Memory service；现有主题计数、document affinity、拒绝/确认、驻留/检索选择、合并保留为业务元数据/可重建索引，字段来源见 `interfaces.MemoryService` 和 `types.MemoryItem/MemoryTopicView/MemoryDocView/MemoryConsolidationResult/MemorySettings`，不是第二份可写长期记忆。P1 若所选后端不能保存这些元数据，需最小业务扩展并测试，不能删功能。

Delegate Start 在一个准入事务固定 child run、parent、budget root、scope 子集、target/workspace 和触发 call ID；重复 Start 返回同一 child。结果必须携带 child 身份，父取消先持久化并传播至 child/Sandbox，再核对已派发动作。SDK branch/state merge 的顺序不是业务租约：并行写入以独立 branch ID 和单一 Session commit 序列协调，冲突不能 last-writer-wins 静默覆盖。现有 Craft 工作区限制和远程 provider 身份必须保留。

归档 API 只暴露 ArchiveReader，Kind 仅为 session/message/tool_call/approval/audit/memory/artifact 的已有记录类别，默认分页上限 100。查询/附件访问每次使用当前空间与资源权限；不因曾是成员而允许读取。对旧 session 的 RunControl/steer/resume/model-context 操作返回 archive_read_only；旧协议客户端在切换后收到 client_upgrade_required。新协议的自动上下文组装禁止读取归档。历史主动检索工具若业务允许，可返回明确来源、当前授权的只读结果，不能冒充新 Session 历史。归档保留记录、附件和产物引用，不新增清除策略；P6 实现归档与配置迁移，P7 验证数量/关系/权限/备份恢复，P8 仅在 P7 通过后演练切换。

## 8. 每类扩展的理由、测试与删除条件

下表是矩阵所有“最小业务扩展”行的生命周期契约。原生采用仅指框架职责；任何受上方执行门影响的行仍不能直接启用。

| 扩展族/矩阵行 | 保留理由 | 必须新增的行为证据 | 删除条件 |
| --- | --- | --- | --- |
| 身份/Run/Fence（RUN、GRAPH、SESSION） | tenant/成员/准入/租约属于产品；SDK 没有该业务授权 | SQLite/PostgreSQL scope、撤权、双 worker、失租、断线与取消 | 同等受审业务服务接管且调用者迁完才删旧适配器；不能只因 SDK 同名类型删除治理 |
| Provider（MODEL、PROVIDER） | 协议特殊字段/签名/图像/缓存/超时/并发保真；旧桥显式拒绝部分原生字段 | 每配置的真实调用与参数捕获；对应旧 model_test 用例迁到原生路径；错误/费用/取消 | 每 provider 原生实现已验证所有差异后单独删 shim；非 Agent chat 消费者清零前保留共享旧代码 |
| 工具/MCP/Connector（TOOLS、MCP、CONNECTOR） | 外部副作用/授权/OAuth/审批/查询恢复不是 SDK 协议职责 | dispatch 计数、参数 hash、撤权、OAuth 重启、结果未知、幂等到期、重连重放 | 原生组件或统一业务 gateway 实现同等门槛并移除旧旁路后删重复包装，治理本身保留 |
| 知识/Web（KNOWLEDGE、WEB） | 现有知识权限、引用、SQL、Wiki 写策略与供应商开关 | 每注册工具允许/拒绝/来源/共享只读；不能仅测 registry 数量 | 原生工具完整调用同一业务服务且逐项等价后删旧工具壳；不替换知识服务本体 |
| Skills/Sandbox（SKILL、SANDBOX） | 安装 entitlement、bundle/path/workspace/长任务；阻止 local fallback | 无能力时零执行、越界文件拒绝、撤权、长任务 kill/query/cancel、产物访问 | 原生 executor 能保持现有 sandbox 能力且测试通过后删重复适配；授权边界不能删 |
| Session/commit/event（GRAPH、SESSION、EVENT、CLIENT） | 幂等历史、跨提交修复、耐久 replay、版本化 UI | append 失败后零后续 dispatch、六间隙 kill、projection 重建、cursor 去重和全部端 | 原生存储/协调机制固定版本证明同等保证后删 glue；业务事件协议仍保留 |
| Memory（MEMORY） | 用户 opt-out、撤权、generation/tombstone、主题/文档偏好与拒绝语义 | 关闭/删除与后台 commit 竞态、双空间 principal、拒绝不重提、可重建索引 | 原生 Memory 服务+元数据满足完整产品语义后删重复抽取/索引；不可维护双权威过渡 |
| 委派/预算/计费（MULTI、USAGE） | 身份不扩大、预算根共享、商业归属与实际成本 | 父子取消/预算竞争、attempt retry/failover 去重、BYOK/平台、unknown usage 核对 | 统一业务协调器/计费服务承接后删重复适配；SDK callback 永不直接替代账本 |
| 归档（ARCHIVE） | 用户明确要求只读留存且新执行从零 | 跨 tenant、撤权、附件、旧会话写拒绝、完整备份恢复 | 只有另行批准保留/删除策略并迁完合法查询消费者，不能在本次迁移自动删除 |

本次不选择“固定候选升级”：没有对新 tag/commit 做相同探针，因此没有声称升级能消除 race、SQL 或恢复问题。矩阵中的“阻塞并修订规格”要求 P0-5 明示 no-go，并针对固定修复版本/支持配置/最小受控组合补充批准与证据；若需要改变已确认职责或功能才回到用户规格决策。它不授权静默降级、关闭恢复或删除功能。
