package execution

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

// ExecutionObservation is a fresh observation from the provider. A product
// cancellation is durable independently from this observation: only a fresh
// exited/destroyed observation permits a workspace to be reused.
type ExecutionObservation struct {
	ProcessState string
	Fresh        bool
	ObservedAt   time.Time
	Epoch        int64
}

func MayReleaseWorkspace(runStatus string, observation ExecutionObservation) bool {
	return MayReleaseWorkspaceAtEpoch(runStatus, 0, observation)
}

// MayReleaseWorkspaceAtEpoch only accepts an observation from the current
// provider epoch. Epoch zero is retained for legacy callers that do not yet
// have a fenced remote command; all fenced callers must pass a positive epoch.
func MayReleaseWorkspaceAtEpoch(runStatus string, currentEpoch int64, observation ExecutionObservation) bool {
	status := strings.ToLower(strings.TrimSpace(runStatus))
	if status != "canceled" && status != "failed" && status != "succeeded" {
		return false
	}
	if currentEpoch > 0 && observation.Epoch != currentEpoch {
		return false
	}
	if !observation.Fresh {
		return false
	}
	state := strings.ToLower(strings.TrimSpace(observation.ProcessState))
	return state == "exited" || state == "destroyed"
}

var (
	ErrWorkspaceLocked    = errors.New("execution_workspace_locked")
	ErrWorkspaceLeaseLost = errors.New("execution_workspace_lease_lost")
)

type StopState string

const (
	StopConfirmed   StopState = "confirmed"
	StopUnconfirmed StopState = "unconfirmed"
	StopUnknown     StopState = "unknown"
)

type StopResult struct {
	ExternalID  string
	State       StopState
	Observation ExecutionObservation
	Confirmed   bool
}

// RemoteControl is deliberately smaller than the Paseo SDK. Implementations
// must use the provider's public cancel and observe operations; a local process
// kill or shell fallback is not a valid implementation.
type RemoteControl interface {
	Cancel(context.Context, string) error
	Observe(context.Context, string) (ExecutionObservation, error)
}

// StopRemoteExecution sends one idempotent stop request and then observes the
// same external execution. A successful stop response is not confirmation: a
// running or unknown observation remains unconfirmed so the workspace stays
// fenced and callers can retry reconciliation with the same external ID.
func StopRemoteExecution(ctx context.Context, remote RemoteControl, externalID string, observeTimeout time.Duration) (StopResult, error) {
	return stopRemoteExecutionAtEpoch(ctx, remote, externalID, 0, observeTimeout)
}

// StopRemoteExecutionAtEpoch fences provider observations to the epoch that
// issued the stop. A response from a reused external ID can never release the
// current workspace.
func StopRemoteExecutionAtEpoch(ctx context.Context, remote RemoteControl, externalID string, epoch int64, observeTimeout time.Duration) (StopResult, error) {
	return stopRemoteExecutionAtEpoch(ctx, remote, externalID, epoch, observeTimeout)
}

func stopRemoteExecutionAtEpoch(ctx context.Context, remote RemoteControl, externalID string, epoch int64, observeTimeout time.Duration) (StopResult, error) {
	result := StopResult{ExternalID: strings.TrimSpace(externalID), State: StopUnknown}
	if remote == nil || result.ExternalID == "" {
		return result, ErrWorkspaceLeaseLost
	}
	if err := remote.Cancel(ctx, result.ExternalID); err != nil {
		return result, err
	}
	if observeTimeout <= 0 {
		observeTimeout = 10 * time.Second
	}
	deadline := time.NewTimer(observeTimeout)
	defer deadline.Stop()
	for {
		observation, err := remote.Observe(ctx, result.ExternalID)
		if err != nil {
			result.State = StopUnknown
			return result, err
		}
		result.Observation = observation
		if MayReleaseWorkspaceAtEpoch("canceled", epoch, observation) {
			result.State, result.Confirmed, result.Observation.Fresh = StopConfirmed, true, true
			return result, nil
		}
		if strings.EqualFold(observation.ProcessState, "unknown") || strings.EqualFold(observation.ProcessState, "not_found") {
			result.State = StopUnknown
			return result, nil
		}
		select {
		case <-ctx.Done():
			result.State = StopUnknown
			return result, ctx.Err()
		case <-deadline.C:
			result.State = StopUnconfirmed
			return result, nil
		case <-time.After(25 * time.Millisecond):
		}
	}
}

type WorkspaceLease struct {
	TenantID     uint64
	WorkspaceRef string
	RunID        string
	Owner        string
	Epoch        int64
	ExpiresAt    time.Time
}

type workspaceLeaseKey struct {
	tenantID     uint64
	workspaceRef string
}

// WorkspaceLocks is a process-local coordination primitive for hosts that do
// not yet have the durable lease table. It still enforces tenant scope and
// owner/epoch fencing. Production adapters should persist the same contract
// behind WorkspaceLeaseStore before enabling multi-replica execution.
type WorkspaceLocks struct {
	mu     sync.Mutex
	leases map[workspaceLeaseKey]WorkspaceLease
	epochs map[workspaceLeaseKey]int64
}

func NewWorkspaceLocks() *WorkspaceLocks {
	return &WorkspaceLocks{leases: make(map[workspaceLeaseKey]WorkspaceLease), epochs: make(map[workspaceLeaseKey]int64)}
}

func (l *WorkspaceLocks) Acquire(tenantID uint64, workspaceRef, runID, owner string, ttl time.Duration) (WorkspaceLease, error) {
	if l == nil || tenantID == 0 || strings.TrimSpace(workspaceRef) == "" || strings.TrimSpace(runID) == "" || strings.TrimSpace(owner) == "" || ttl <= 0 {
		return WorkspaceLease{}, ErrWorkspaceLeaseLost
	}
	key := workspaceLeaseKey{tenantID: tenantID, workspaceRef: strings.TrimSpace(workspaceRef)}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	previous, ok := l.leases[key]
	if ok && previous.ExpiresAt.After(now) {
		return WorkspaceLease{}, ErrWorkspaceLocked
	}
	epoch := l.epochs[key] + 1
	l.epochs[key] = epoch
	lease := WorkspaceLease{TenantID: tenantID, WorkspaceRef: key.workspaceRef, RunID: strings.TrimSpace(runID), Owner: strings.TrimSpace(owner), Epoch: epoch, ExpiresAt: now.Add(ttl)}
	l.leases[key] = lease
	return lease, nil
}
func (l *WorkspaceLocks) Renew(lease WorkspaceLease, owner string, ttl time.Duration) error {
	if l == nil || ttl <= 0 {
		return ErrWorkspaceLeaseLost
	}
	key := workspaceLeaseKey{tenantID: lease.TenantID, workspaceRef: lease.WorkspaceRef}
	l.mu.Lock()
	defer l.mu.Unlock()
	current, ok := l.leases[key]
	if !ok || current.RunID != lease.RunID || current.Owner != lease.Owner || current.Epoch != lease.Epoch || current.Owner != strings.TrimSpace(owner) || !current.ExpiresAt.After(time.Now()) {
		return ErrWorkspaceLeaseLost
	}
	current.ExpiresAt = time.Now().Add(ttl)
	l.leases[key] = current
	return nil
}
func (l *WorkspaceLocks) Release(lease WorkspaceLease, owner string) error {
	if l == nil {
		return ErrWorkspaceLeaseLost
	}
	key := workspaceLeaseKey{tenantID: lease.TenantID, workspaceRef: lease.WorkspaceRef}
	l.mu.Lock()
	defer l.mu.Unlock()
	current, ok := l.leases[key]
	if !ok || current.RunID != lease.RunID || current.Owner != lease.Owner || current.Epoch != lease.Epoch || current.Owner != strings.TrimSpace(owner) {
		return ErrWorkspaceLeaseLost
	}
	delete(l.leases, key)
	return nil
}
func (l *WorkspaceLocks) Held(tenantID uint64, workspaceRef string) (WorkspaceLease, bool) {
	if l == nil {
		return WorkspaceLease{}, false
	}
	key := workspaceLeaseKey{tenantID: tenantID, workspaceRef: strings.TrimSpace(workspaceRef)}
	l.mu.Lock()
	defer l.mu.Unlock()
	lease, ok := l.leases[key]
	if !ok || !lease.ExpiresAt.After(time.Now()) {
		if ok {
			delete(l.leases, key)
		}
		return WorkspaceLease{}, false
	}
	return lease, true
}
