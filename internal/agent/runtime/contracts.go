// Package runtime defines the durable execution contracts used by the tRPC
// agent runtime. It intentionally contains no database or application-service
// dependencies so workers can be assembled without import cycles.
package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	// ErrConflict reports an incompatible idempotent request or state change.
	ErrConflict = errors.New("agent runtime conflict")
	// ErrRunActive indicates that a session's durable run slot is occupied.
	ErrRunActive = errors.New("agent run already active")
	// ErrLeaseLost rejects workers without a current owner/epoch lease.
	ErrLeaseLost = errors.New("agent run lease lost")
	// ErrNotFound indicates no record exists in the requested scope.
	ErrNotFound = errors.New("agent run not found")
	// ErrCursorExpired reports an event cursor outside retained history.
	ErrCursorExpired = errors.New("agent event cursor expired")
)

// RunKey identifies a run within its tenant.
type RunKey struct {
	TenantID uint64
	RunID    string
}

// Fence identifies a worker's exclusive, expiring claim on a run.
type Fence struct {
	RunKey
	Owner string
	Epoch int64
}

// RunEvent is an append-only durable event in a run stream. Seq is assigned by the store.
type RunEvent struct {
	Seq       int64           `json:"seq"`
	AttemptID string          `json:"attempt_id,omitempty"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

// RunInput is a durable steering message. Mode is inject or after.
type RunInput struct {
	SteerID string          `json:"steer_id"`
	Mode    string          `json:"mode"`
	Message json.RawMessage `json:"message"`
}

// Run is the durable execution query view.
type Run struct {
	Key                RunKey
	SessionID          string
	UserID             string
	RequestID          string
	AssistantMessageID string
	Status             string
	WaitReason         string
	Owner              string
	Revision           int64
	Epoch              int64
	LeaseUntil         time.Time
	Deadline           time.Time
	Snapshot           json.RawMessage
}

// Admission contains the immutable request and initial business messages.
type Admission struct {
	Key                RunKey
	SessionID          string
	UserID             string
	RequestID          string
	AssistantMessageID string
	RequestHash        string
	Snapshot           json.RawMessage
	UserMessage        json.RawMessage
	AssistantMessage   json.RawMessage
	Deadline           time.Time
}

// Decision is a durable user resolution for a run waiting on an external
// result or tool action. Result is used only by provide_result.
type Decision struct {
	PendingID        string
	DecisionID       string
	ToolCallID       string
	Action           string
	ArgsHash         string
	Reason           string
	ExpectedRevision int64
	Result           json.RawMessage
	ResourceRef      string
}

// CheckpointRecord holds a complete graph checkpoint and its pending writes.
type CheckpointRecord struct {
	Namespace     string
	ID            string
	ParentID      string
	Seq           int64
	State         json.RawMessage
	PendingWrites json.RawMessage
}

// RunStore persists admission and fences all executing-worker writes.
type RunStore interface {
	Admit(context.Context, Admission) (Run, error)
	Get(context.Context, RunKey) (Run, error)
	Claim(context.Context, RunKey, string, time.Duration) (Fence, error)
	Renew(context.Context, Fence, time.Duration) error
	Scan(context.Context, int) ([]RunKey, error)
	SaveCheckpoint(context.Context, Fence, CheckpointRecord) error
	SetStatus(context.Context, Fence, string, string) error
	LoadCheckpoint(context.Context, RunKey) (CheckpointRecord, error)
}

// RunEventStore is the durable event and steering projection boundary.
type RunEventStore interface {
	AppendEvent(context.Context, Fence, RunEvent) (RunEvent, error)
	ReadEvents(context.Context, RunKey, int64, int) ([]RunEvent, error)
	Finalize(context.Context, Fence, json.RawMessage) error
}

// RunInputStore persists and atomically applies steering inputs with a checkpoint.
type RunInputStore interface {
	AppendInput(context.Context, RunKey, RunInput) error
	ApplyInput(context.Context, Fence, string, CheckpointRecord) error
}

// RunInputReader lists steering inputs not yet consumed by the graph. The
// durable worker reads pending inject inputs at safe node boundaries.
type RunInputReader interface {
	ListPendingInputs(context.Context, RunKey, string) ([]RunInput, error)
}

// RunInputConsumer marks steering inputs consumed once their message is
// checkpointed in AppliedSteerIDs; a crash between mark and checkpoint is
// safe because the resumed state re-filters by AppliedSteerIDs.
type RunInputConsumer interface {
	MarkInputsProcessed(context.Context, RunKey, ...string) error
}
