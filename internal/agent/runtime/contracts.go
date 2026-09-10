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
	LoadCheckpoint(context.Context, RunKey) (CheckpointRecord, error)
}
