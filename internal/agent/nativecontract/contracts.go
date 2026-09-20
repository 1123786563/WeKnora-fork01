// Package nativecontract contains the server-owned boundary shared by native
// agent tracks. Exported structs are transport-neutral; callers never obtain
// authority to choose a Scope, Fence, price, or credential from their values.
package nativecontract

import (
	"context"
	"encoding/json"
	"strconv"
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
	Kind               string `json:"kind"`
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
	State     string     `json:"state"`
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
	ExpectedRevision string          `json:"expected_revision"`
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
	ResumeState string                `json:"resume_state"`
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
	Pending           *PendingReference  `json:"pending,omitempty"`
	Status            RunStatus          `json:"status,omitempty"`
	Wait              WaitKind           `json:"wait_kind,omitempty"`
	Text              string             `json:"text,omitempty"`
	Offset            *int64             `json:"offset,omitempty"`
	ReplacesAttemptID string             `json:"replaces_attempt_id,omitempty"`
	CallID            string             `json:"call_id,omitempty"`
	PlanVersion       int                `json:"plan_version,omitempty"`
	ToolName          string             `json:"tool_name,omitempty"`
	PendingID         string             `json:"pending_id,omitempty"`
	ArgsHash          string             `json:"args_hash,omitempty"`
	ExpiresAt         *time.Time         `json:"expires_at,omitempty"`
	Outcome           *ToolOutcome       `json:"outcome,omitempty"`
	Usage             *PublicUsage       `json:"usage,omitempty"`
	Artifact          *PublicArtifactRef `json:"artifact,omitempty"`
	Failure           *Failure           `json:"failure,omitempty"`
}
type BusinessEvent struct {
	Protocol      string       `json:"protocol"`
	SchemaVersion int          `json:"schema_version"`
	EventID       string       `json:"event_id"`
	TenantID      string       `json:"tenant_id"`
	SessionID     string       `json:"session_id"`
	RunID         string       `json:"run_id"`
	ParentRunID   string       `json:"parent_run_id,omitempty"`
	AttemptID     string       `json:"attempt_id,omitempty"`
	Sequence      string       `json:"seq"`
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

type ArtifactRef struct {
	ID        string `json:"id"`
	MediaType string `json:"media_type"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"size_bytes"`
}

// PublicArtifactRef is the browser/client wire projection. ArtifactRef remains
// an internal storage value, while public int64 counters are decimal strings.
type PublicArtifactRef struct {
	ID        string `json:"id"`
	MediaType string `json:"media_type"`
	SHA256    string `json:"sha256"`
	SizeBytes string `json:"size_bytes"`
}

func PublicArtifact(ref ArtifactRef) PublicArtifactRef {
	return PublicArtifactRef{ID: ref.ID, MediaType: ref.MediaType, SHA256: ref.SHA256, SizeBytes: strconv.FormatInt(ref.SizeBytes, 10)}
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
	// Attempt is assigned by the durable claim. It fences a crashed worker
	// after another worker has reclaimed the expired running job.
	Attempt int64
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
	ID        string              `json:"id"`
	SessionID string              `json:"session_id"`
	Kind      string              `json:"kind"`
	Data      json.RawMessage     `json:"data"`
	Artifacts []PublicArtifactRef `json:"artifacts"`
}
type ArchivePage struct {
	Records    []ArchiveRecord `json:"records"`
	NextCursor string          `json:"next_cursor,omitempty"`
}
type ArchiveReader interface {
	List(context.Context, Scope, ArchiveQuery) (ArchivePage, error)
	Read(context.Context, Scope, string) (ArchiveRecord, error)
	ReadArtifact(context.Context, Scope, string) (ArtifactRef, error)
}
