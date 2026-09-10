package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
)

// Tool journal states and explicitly trusted recovery capabilities.
const (
	ToolStatusPlanned     = "planned"
	ToolStatusDispatching = "dispatching"
	ToolStatusSucceeded   = "succeeded"
	ToolStatusFailed      = "failed"
	ToolStatusUnknown     = "unknown"

	ToolRecoveryWaitUser   = "wait_user"
	ToolRecoveryQueryable  = "queryable"
	ToolRecoveryIdempotent = "idempotent"
	ToolRecoveryReadOnly   = "read_only"
)

var (
	// ErrToolWaitUser means a possibly dispatched tool has no trustworthy
	// outcome and cannot be retried from durable evidence.
	ErrToolWaitUser = errors.New("tool outcome unknown; waiting for user")
	// ErrToolRecoveryQuery means the provider-specific observation path must
	// resolve the recorded external task before execution can continue.
	ErrToolRecoveryQuery = errors.New("tool outcome requires provider query")
	// ErrToolNotDispatched rejects an execution adapter that bypassed the
	// durable dispatch boundary and therefore cannot prove its outcome.
	ErrToolNotDispatched = errors.New("tool execution did not enter the durable dispatch boundary")
)

// ToolPlan is the immutable, durable identity of one logical model tool call.
// RecoveryPolicy is empty for the safe default (wait_user). Capability values
// are accepted only from trusted application wiring, never inferred here.
type ToolPlan struct {
	Version        int64
	CallID         string
	Name           string
	Identity       string
	ArgsHash       string
	IdempotencyKey string
	Args           json.RawMessage

	RecoveryPolicy       string
	IdempotencyExpiresAt time.Time
}

// StoredToolResult preserves live-only output files outside ToolResult's JSON
// representation while keeping the normal result envelope intact.
type StoredToolResult struct {
	Result      types.ToolResult
	OutputFiles []string
	Source      string
}

// ToolAttempt identifies one append-only dispatch attempt for a logical call.
type ToolAttempt struct {
	RunKey
	Owner  string
	CallID string
	Number int
	Epoch  int64
}

// ToolRecord is the journal view needed to make a recovery decision.
type ToolRecord struct {
	Plan          ToolPlan
	CallSeq       int64
	Status        string
	UnknownReason string
	Result        *StoredToolResult
}

// RecoveryFacts derives safety only from immutable persisted metadata.
func (r ToolRecord) RecoveryFacts(now time.Time) RecoveryFacts {
	keyValid := r.Plan.IdempotencyKey != "" && !r.Plan.IdempotencyExpiresAt.IsZero() &&
		now.Before(r.Plan.IdempotencyExpiresAt)
	return RecoveryFacts{
		HasResult:     (r.Status == ToolStatusSucceeded || r.Status == ToolStatusFailed) && r.Result != nil,
		NotDispatched: r.Status == ToolStatusPlanned,
		Queryable:     r.Plan.RecoveryPolicy == ToolRecoveryQueryable,
		Idempotent:    r.Plan.RecoveryPolicy == ToolRecoveryIdempotent,
		KeyValid:      keyValid,
		ReadOnly:      r.Plan.RecoveryPolicy == ToolRecoveryReadOnly,
	}
}

// ToolJournal persists plan, attempt and outcome transitions under a fence.
type ToolJournal interface {
	EnsureToolPlan(context.Context, Fence, ToolPlan) (ToolRecord, error)
	BeginToolAttempt(context.Context, Fence, string, ...int64) (ToolAttempt, error)
	ReviseToolPlan(context.Context, Fence, string, int64, json.RawMessage) (ToolPlan, error)
	CommitToolResult(context.Context, Fence, ToolAttempt, StoredToolResult) error
	CommitToolRejection(context.Context, Fence, string, StoredToolResult) error
	MarkToolUnknown(context.Context, Fence, ToolAttempt, string) error
}

// ToolExecuteFunc matches ToolRegistry.ExecuteTool. Implementations must call
// BeforeToolDispatch after preflight, immediately before the external action.
type ToolExecuteFunc func(context.Context, string, json.RawMessage) (*types.ToolResult, error)

// ToolDispatch carries persisted metadata to trusted provider adapters. The
// adapter must actually propagate the key to the provider before declaring a
// tool idempotent. A key in this context alone does not confer that capability.
type ToolDispatch struct {
	RunKey
	CallID, Identity, IdempotencyKey string
	IdempotencyExpiresAt             time.Time
	Attempt                          int
	PlanVersion                      int64
	ArgsHash                         string
}

type toolDispatchContextKey struct{}

type toolDispatchState struct {
	mu       sync.Mutex
	metadata ToolDispatch
	attempt  ToolAttempt
	begin    func(context.Context, int64) (ToolAttempt, error)
	revise   func(context.Context, int64, json.RawMessage) (ToolPlan, error)
	finished bool
}

// BeforeToolDispatch is called after permission/schema/approval/OAuth checks.
// Without a durable executor it preserves ordinary builtin execution. With a
// durable executor it allows exactly one external call per dispatch attempt.
func BeforeToolDispatch(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	state, ok := ctx.Value(toolDispatchContextKey{}).(*toolDispatchState)
	if !ok {
		return nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.finished || state.attempt.Number != 0 {
		return ErrConflict
	}
	attempt, err := state.begin(ctx, state.metadata.PlanVersion)
	if err != nil {
		return err
	}
	state.attempt = attempt
	state.metadata.Attempt = attempt.Number
	return nil
}

type toolApprovalProjectionKey struct{}

// WithToolApprovalProjection binds a proxy's approved target args back to its
// logical call envelope. Only the trusted proxy adapter installs this mapping.
func WithToolApprovalProjection(
	ctx context.Context, project func(json.RawMessage) (json.RawMessage, error),
) context.Context {
	return context.WithValue(ctx, toolApprovalProjectionKey{}, project)
}

// ApproveToolArguments persists arguments returned by the server-side approval
// gate. Tool input itself must never be used as authorization to call this.
func ApproveToolArguments(ctx context.Context, args json.RawMessage) error {
	state, ok := ctx.Value(toolDispatchContextKey{}).(*toolDispatchState)
	if !ok {
		return nil
	}
	if project, ok := ctx.Value(toolApprovalProjectionKey{}).(func(json.RawMessage) (json.RawMessage, error)); ok {
		var err error
		args, err = project(args)
		if err != nil {
			return err
		}
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.finished || state.attempt.Number != 0 {
		return ErrConflict
	}
	plan, err := state.revise(ctx, state.metadata.PlanVersion, args)
	if err != nil {
		return err
	}
	state.metadata.PlanVersion, state.metadata.ArgsHash = plan.Version, plan.ArgsHash
	state.metadata.IdempotencyKey = plan.IdempotencyKey
	state.metadata.IdempotencyExpiresAt = plan.IdempotencyExpiresAt
	return nil
}

// CarryToolDispatch preserves durable metadata when approval/OAuth derives a
// fresh execution context from the request parent.
func CarryToolDispatch(from, to context.Context) context.Context {
	if state, ok := from.Value(toolDispatchContextKey{}).(*toolDispatchState); ok {
		to = context.WithValue(to, toolDispatchContextKey{}, state)
	}
	if project, ok := from.Value(toolApprovalProjectionKey{}).(func(json.RawMessage) (json.RawMessage, error)); ok {
		to = context.WithValue(to, toolApprovalProjectionKey{}, project)
	}
	return to
}

// ToolDispatchFromContext retrieves journal metadata without adding model-
// controlled arguments to the tool's schema.
func ToolDispatchFromContext(ctx context.Context) (ToolDispatch, bool) {
	state, ok := ctx.Value(toolDispatchContextKey{}).(*toolDispatchState)
	if !ok {
		return ToolDispatch{}, false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.metadata, true
}

// ToolExecutor wraps the existing registry with a durable tool journal.
type ToolExecutor struct {
	runs    RunStore
	journal ToolJournal
	execute ToolExecuteFunc
}

// NewToolExecutor creates a recoverable executor without replacing the
// registry's validation, permission, approval, OAuth or tool logic.
func NewToolExecutor(runs RunStore, journal ToolJournal, execute ToolExecuteFunc) *ToolExecutor {
	return &ToolExecutor{runs: runs, journal: journal, execute: execute}
}

// Execute journals planned -> dispatching -> result. The external call runs
// outside repository transactions. Any error after dispatch is conservatively
// recorded as unknown, including cancellation and timeout.
func (e *ToolExecutor) Execute(
	ctx context.Context, fence Fence, plan ToolPlan,
) (StoredToolResult, error) {
	if e == nil || e.runs == nil || e.journal == nil || e.execute == nil || !validToolPlan(plan) ||
		fence.TenantID == 0 || fence.RunID == "" || fence.Owner == "" || fence.Epoch <= 0 {
		return StoredToolResult{}, ErrConflict
	}
	run, err := e.runs.Get(ctx, fence.RunKey)
	if err != nil {
		return StoredToolResult{}, err
	}
	if run.Key != fence.RunKey || run.Owner != fence.Owner || run.Epoch != fence.Epoch ||
		(run.Status != "running" && run.Status != "recovering") {
		return StoredToolResult{}, ErrLeaseLost
	}
	record, err := e.journal.EnsureToolPlan(ctx, fence, plan)
	if err != nil {
		return StoredToolResult{}, err
	}
	action := RecoveryAction(record.RecoveryFacts(time.Now()))
	switch action {
	case "reuse":
		return normalizedStoredResult(*record.Result), nil
	case "wait_user":
		return StoredToolResult{}, fmt.Errorf("%w: %s", ErrToolWaitUser, record.UnknownReason)
	case "query":
		return StoredToolResult{}, ErrToolRecoveryQuery
	case "execute", "retry":
		// Continue below. BeginToolAttempt rechecks the durable recovery facts
		// and fence in the dispatch transaction.
	default:
		return StoredToolResult{}, ErrConflict
	}

	state := &toolDispatchState{
		metadata: ToolDispatch{
			RunKey: fence.RunKey, CallID: record.Plan.CallID, Identity: record.Plan.Identity,
			IdempotencyKey: record.Plan.IdempotencyKey, IdempotencyExpiresAt: record.Plan.IdempotencyExpiresAt,
			PlanVersion: record.Plan.Version, ArgsHash: record.Plan.ArgsHash,
		},
		begin: func(dispatchCtx context.Context, version int64) (ToolAttempt, error) {
			return e.journal.BeginToolAttempt(dispatchCtx, fence, plan.CallID, version)
		},
		revise: func(approvalCtx context.Context, version int64, args json.RawMessage) (ToolPlan, error) {
			return e.journal.ReviseToolPlan(approvalCtx, fence, plan.CallID, version, args)
		},
	}
	dispatchCtx := context.WithValue(ctx, toolDispatchContextKey{}, state)
	result, executeErr := e.execute(dispatchCtx, record.Plan.Name, append(json.RawMessage(nil), record.Plan.Args...))
	state.mu.Lock()
	state.finished = true
	attempt := state.attempt
	state.mu.Unlock()
	if attempt.Number == 0 {
		// A pending/failed preflight must never be classified as an unknown
		// external side effect. A definitive rejection may be checkpointed
		// without manufacturing a dispatch attempt.
		if executeErr != nil {
			return StoredToolResult{}, executeErr
		}
		if err := ctx.Err(); err != nil {
			return StoredToolResult{}, err
		}
		if result == nil || result.Success {
			return StoredToolResult{}, ErrToolNotDispatched
		}
		stored := normalizedStoredResult(StoredToolResult{Result: *result, Source: "preflight"})
		persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := e.journal.CommitToolRejection(persistCtx, fence, plan.CallID, stored); err != nil {
			return StoredToolResult{}, err
		}
		return stored, nil
	}
	if executeErr != nil || result == nil {
		if executeErr == nil {
			executeErr = errors.New("tool returned no result")
		}
		persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if markErr := e.journal.MarkToolUnknown(persistCtx, fence, attempt, executeErr.Error()); markErr != nil {
			return StoredToolResult{}, errors.Join(executeErr, markErr)
		}
		return StoredToolResult{}, executeErr
	}
	stored := normalizedStoredResult(StoredToolResult{
		Result: *result, OutputFiles: append([]string(nil), result.OutputFiles...), Source: "tool",
	})
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := e.journal.CommitToolResult(persistCtx, fence, attempt, stored); err != nil {
		return StoredToolResult{}, err
	}
	return stored, nil
}

func validToolPlan(plan ToolPlan) bool {
	if plan.CallID == "" || plan.Name == "" || plan.Identity == "" || plan.ArgsHash == "" ||
		len(plan.Args) == 0 || !json.Valid(plan.Args) {
		return false
	}
	switch plan.RecoveryPolicy {
	case "", ToolRecoveryWaitUser, ToolRecoveryQueryable, ToolRecoveryIdempotent, ToolRecoveryReadOnly:
		return true
	default:
		return false
	}
}

func normalizedStoredResult(result StoredToolResult) StoredToolResult {
	files := result.OutputFiles
	if len(files) == 0 {
		files = result.Result.OutputFiles
	}
	result.OutputFiles = append([]string(nil), files...)
	result.Result.OutputFiles = append([]string(nil), files...)
	result.Result.Images = append([]string(nil), result.Result.Images...)
	if result.Source == "" {
		result.Source = "tool"
	}
	return result
}
