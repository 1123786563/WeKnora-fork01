// Package nativecontract contains the server-owned boundary shared by native
// agent tracks. Exported structs are transport-neutral; callers never obtain
// authority to choose a Scope, Fence, price, or credential from their values.
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
	ActorUserID                      string
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
	CredentialRef                                                     string
	CredentialVersion                                                 int64
	ToolSetHash, SkillSetHash, ExecutionTargetID, WorkspaceRef        string
}
type FundingBinding struct {
	BudgetRef, BudgetRootRunID, Source, Funding, Service, PriceVersion string
	CredentialVersion, MaxUnits                                        int64
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

type AdmissionControls struct {
	Revision                                                                                   int64
	RecoveryEnabled, AdmissionEnabled, WorkerDrain, NativeExecutionApproved, DependenciesReady bool
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

type EffectState string

const (
	EffectNotDispatched EffectState = "not_dispatched"
	EffectConfirmed     EffectState = "confirmed"
	EffectUnknown       EffectState = "unknown"
)

type ToolIdentity struct{ Kind, ServiceID, InstallationID, Name, SchemaHash, ConfigVersion string }
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
type ToolOutcome struct {
	AttemptID, CallID, ProviderReceipt, QueryAnchor, ResultHash string
	Effect                                                      EffectState
	IsError, Truncated                                          bool
	Content                                                     json.RawMessage
	Artifacts                                                   []ArtifactRef
	Failure                                                     *Failure
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
	ErrAdmissionClosed ErrorCode = "admission_closed"
	ErrExecutionGate   ErrorCode = "execution_gate_closed"
)

type Failure struct {
	Code      ErrorCode   `json:"code"`
	Message   string      `json:"message"`
	Retryable bool        `json:"retryable"`
	Effect    EffectState `json:"effect"`
	AttemptID string      `json:"attempt_id,omitempty"`
}

func (f Failure) Error() string { return string(f.Code) + ": " + f.Message }

// Fault is retained as the cross-track name for a sanitized Failure.
type Fault = Failure

type ModelBinding struct {
	Model   model.Model
	Config  ConfigBinding
	Funding FundingBinding
}
type ModelResolver interface {
	Resolve(context.Context, Scope, ConfigBinding) (ModelBinding, error)
}
type UsageObservation struct {
	Version                                                                                       int
	Run                                                                                           RunIdentity
	AttemptID, ObservationID, ProviderRequestID                                                   string
	Funding                                                                                       FundingBinding
	Revision                                                                                      int64
	PromptTokens, CompletionTokens, TotalTokens, CachedTokens, CacheReadTokens, CacheCreateTokens int64
	AccountingStatus                                                                              string
	Dimensions                                                                                    map[string]int64
	OccurredAt                                                                                    time.Time
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

type PendingReference struct {
	PendingID  string `json:"pending_id"`
	DetailPath string `json:"detail_path"`
	Revision   string `json:"revision"`
}
type PublicUsage struct{ ObservationID, Revision, PromptTokens, CompletionTokens, TotalTokens, CachedTokens, CacheReadTokens, CacheCreateTokens, AccountingStatus string }
type EventPayload struct {
	Pending                       *PendingReference
	Status                        RunStatus
	Wait                          WaitKind
	Text                          string
	Offset                        *int64
	ReplacesAttemptID, CallID     string
	PlanVersion                   int
	ToolName, PendingID, ArgsHash string
	ExpiresAt                     *time.Time
	Outcome                       *ToolOutcome
	Usage                         *PublicUsage
	Artifact                      *ArtifactRef
	Failure                       *Failure
}
type BusinessEvent struct {
	Protocol                                                              string
	SchemaVersion                                                         int
	EventID, TenantID, SessionID, RunID, ParentRunID, AttemptID, Sequence string
	Kind                                                                  EventKind
	Payload                                                               EventPayload
}
type EventPage struct {
	Events      []BusinessEvent
	NextCursor  string
	MinSequence int64
}
type EventReader interface {
	Read(context.Context, Scope, RunIdentity, string, int) (EventPage, error)
}

type ArtifactRef struct {
	ID        string `json:"id"`
	MediaType string `json:"media_type"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"size_bytes"`
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
	TerminalStatus  RunStatus
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
	Data                json.RawMessage
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
