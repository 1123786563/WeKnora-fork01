package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
)

type WorkerConfig struct {
	Enabled                        bool
	Lease, Heartbeat, ScanInterval time.Duration
	MaxWorkers                     int
	Driver                         string
}

// RemoteDispatchConfig makes the durable W20 dispatch boundary part of the
// worker execution path. The worker claims the run first; this adapter then
// persists the provider intent and receipt around exactly one provider start.
// A nil provider is rejected by RemoteDispatcher and cannot silently fall
// back to the legacy executor.
type RemoteDispatchConfig struct {
	Dispatcher FencedRemoteDispatcher
	Provider   RemoteProvider
	CommandID  func(agentruntime.Fence) (commandID, payloadHash string)
}

// RemoteProvider and RemoteDispatcher are local structural ports. Keeping
// the worker dependent on ports avoids importing the workbench service package
// (and its HTTP-admission dependencies) into the generic worker package.
type RemoteProvider = agentruntime.RemoteProvider

type RemoteDispatcher interface {
	Dispatch(context.Context, agentruntime.RunKey, string, string, string, time.Duration, int64, RemoteProvider) (string, error)
}
type FencedRemoteDispatcher interface {
	DispatchFence(context.Context, agentruntime.Fence, string, string, time.Duration, RemoteProvider) (string, error)
}

func (c RemoteDispatchConfig) validate() error {
	if c.Dispatcher == nil || c.Provider == nil || c.CommandID == nil {
		return errors.New("remote dispatcher, provider, and command builder are required")
	}
	return nil
}

func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{Lease: time.Minute, Heartbeat: 15 * time.Second, ScanInterval: 5 * time.Second, MaxWorkers: 4, Driver: "platform"}
}

func (c WorkerConfig) Validate() error {
	if c.Lease <= 0 || c.Heartbeat <= 0 || c.ScanInterval <= 0 || c.MaxWorkers <= 0 {
		return errors.New("agent worker durations and max_workers must be positive")
	}
	if c.Heartbeat*2 >= c.Lease {
		return fmt.Errorf("agent worker heartbeat must be less than half the lease")
	}
	return nil
}

type AgentRunWorker struct {
	store     agentruntime.RunStore
	execute   func(context.Context, agentruntime.Fence) error
	reconcile func(context.Context, agentruntime.Fence) error
	cfg       WorkerConfig
	owner     string
	remote    *RemoteDispatchConfig
	mu        sync.Mutex
	active    map[string]context.CancelFunc
	done      chan struct{}
}

// SetRecoveryHook installs the post-claim sandbox reconciliation step. The
// hook runs before graph execution and may durably park the run.
func (w *AgentRunWorker) SetRecoveryHook(hook func(context.Context, agentruntime.Fence) error) {
	if w != nil {
		w.reconcile = hook
	}
}

// Config exposes the worker configuration for startup gates.
func (w *AgentRunWorker) Config() WorkerConfig {
	if w == nil {
		return WorkerConfig{}
	}
	return w.cfg
}

func NewAgentRunWorker(store agentruntime.RunStore, execute func(context.Context, agentruntime.Fence) error, cfg WorkerConfig) (*AgentRunWorker, error) {
	if store == nil || execute == nil {
		return nil, errors.New("agent worker store and executor are required")
	}
	if !cfg.Enabled {
		return &AgentRunWorker{store: store, execute: execute, cfg: cfg, owner: fmt.Sprintf("worker-%d", time.Now().UnixNano()), active: make(map[string]context.CancelFunc), done: make(chan struct{})}, nil
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &AgentRunWorker{store: store, execute: execute, cfg: cfg, owner: fmt.Sprintf("worker-%d", time.Now().UnixNano()), active: make(map[string]context.CancelFunc), done: make(chan struct{})}, nil
}

// NewAgentRunWorkerWithRemoteDispatch is the production worker entrypoint for
// Paseo-backed runs. It retains the normal executor for post-start result
// handling, while making provider invocation pass through the durable intent
// and receipt log.
func NewAgentRunWorkerWithRemoteDispatch(
	store agentruntime.RunStore,
	execute func(context.Context, agentruntime.Fence) error,
	remote RemoteDispatchConfig,
	cfg WorkerConfig,
) (*AgentRunWorker, error) {
	if err := remote.validate(); err != nil {
		return nil, err
	}
	w, err := NewAgentRunWorker(store, execute, cfg)
	if err != nil {
		return nil, err
	}
	w.remote = &remote
	if w.cfg.Driver == "" || w.cfg.Driver == "platform" {
		w.cfg.Driver = "paseo"
	}
	return w, nil
}

func (w *AgentRunWorker) Run(ctx context.Context) (err error) {
	if w == nil {
		return errors.New("agent worker is nil")
	}
	defer close(w.done)
	if !w.cfg.Enabled {
		return nil
	}
	if err := w.cfg.Validate(); err != nil {
		return err
	}
	ticker := time.NewTicker(w.cfg.ScanInterval)
	defer ticker.Stop()
	// Recover immediately on process startup, then continue polling.
	if err := w.Tick(ctx); err != nil && ctx.Err() == nil {
		w.backoff(ctx)
	}
	for {
		select {
		case <-ctx.Done():
			w.drain()
			return nil
		case <-ticker.C:
			if err := w.Tick(ctx); err != nil && ctx.Err() == nil {
				w.backoff(ctx)
			}
		}
	}
}

func (w *AgentRunWorker) backoff(ctx context.Context) {
	d := w.cfg.ScanInterval / 2
	if d < 25*time.Millisecond {
		d = 25 * time.Millisecond
	}
	if d > time.Second {
		d = time.Second
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func (w *AgentRunWorker) Tick(ctx context.Context) error {
	if w == nil {
		return errors.New("agent worker is nil")
	}
	if !w.cfg.Enabled {
		return nil
	}
	keys, err := w.scan(ctx)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		run, getErr := w.store.Get(ctx, key)
		if getErr != nil || run.Status == "waiting_user" || run.Status == "canceled" || run.Status == "succeeded" || run.Status == "failed" {
			continue
		}
		id := fmt.Sprintf("%d/%s", key.TenantID, key.RunID)
		w.mu.Lock()
		if len(w.active) >= w.cfg.MaxWorkers {
			w.mu.Unlock()
			break
		}
		if _, ok := w.active[id]; ok {
			w.mu.Unlock()
			continue
		}
		runCtx, cancel := context.WithCancel(context.Background()) // independent of HTTP/request ctx
		w.active[id] = cancel
		w.mu.Unlock()
		fence, claimErr := w.claim(ctx, key)
		if claimErr != nil {
			cancel()
			w.mu.Lock()
			delete(w.active, id)
			w.mu.Unlock()
			continue
		}
		go w.runOne(runCtx, id, fence)
	}
	return nil
}

type driverRunStore interface {
	ScanDriver(context.Context, string, int) ([]agentruntime.RunKey, error)
	ClaimDriver(context.Context, agentruntime.RunKey, string, string, time.Duration) (agentruntime.Fence, error)
}

func (w *AgentRunWorker) scan(ctx context.Context) ([]agentruntime.RunKey, error) {
	if w.cfg.Driver == "platform" {
		return w.store.Scan(ctx, w.cfg.MaxWorkers)
	}
	if store, ok := w.store.(driverRunStore); ok {
		return store.ScanDriver(ctx, w.cfg.Driver, w.cfg.MaxWorkers)
	}
	return nil, fmt.Errorf("worker store does not support driver %q", w.cfg.Driver)
}

func (w *AgentRunWorker) claim(ctx context.Context, key agentruntime.RunKey) (agentruntime.Fence, error) {
	if w.cfg.Driver == "platform" {
		return w.store.Claim(ctx, key, w.owner, w.cfg.Lease)
	}
	if store, ok := w.store.(driverRunStore); ok {
		return store.ClaimDriver(ctx, key, w.cfg.Driver, w.owner, w.cfg.Lease)
	}
	return agentruntime.Fence{}, fmt.Errorf("worker store does not support driver %q", w.cfg.Driver)
}

func (w *AgentRunWorker) runOne(ctx context.Context, id string, fence agentruntime.Fence) {
	defer func() { w.mu.Lock(); delete(w.active, id); w.mu.Unlock() }()
	// The persisted absolute deadline bounds this worker's claim: a run past
	// its deadline fails with the explicit reason instead of looping forever
	// while renewing its lease (spec: 预算与绝对截止时间 not reset on restart).
	if run, err := w.store.Get(ctx, fence.RunKey); err == nil && !run.Deadline.IsZero() &&
		time.Now().After(run.Deadline) {
		_ = w.store.SetStatus(context.WithoutCancel(ctx), fence, "failed", "deadline_exceeded")
		return
	}
	done := make(chan struct{})
	defer close(done)
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	executionCtx := workerCtx
	if run, getErr := w.store.Get(ctx, fence.RunKey); getErr == nil && !run.Deadline.IsZero() {
		if until := time.Until(run.Deadline); until > 0 {
			var deadlineCancel context.CancelFunc
			executionCtx, deadlineCancel = context.WithDeadline(workerCtx, run.Deadline)
			defer deadlineCancel()
		}
	}
	go func() {
		ticker := time.NewTicker(w.cfg.Heartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-workerCtx.Done():
				return
			case <-ticker.C:
				if err := w.store.Renew(workerCtx, fence, w.cfg.Lease); err != nil {
					cancel()
					return
				}
			}
		}
	}()
	if w.reconcile != nil {
		if err := w.reconcile(executionCtx, fence); err != nil {
			if current, getErr := w.store.Get(context.Background(), fence.RunKey); getErr == nil && current.Status == "waiting_user" {
				return
			}
			return
		}
	}
	// A persisted deadline also caps the execution context so a slow graph
	// cannot outlive the budget; the heartbeat renewer gives up with it.
	// The durable run identity is trusted only after Claim has returned a
	// fence. Carry it through the worker boundary so AgentEngine.runToolCall
	// can populate ToolExecContext.RunID for approval projection. RequestID is
	// deliberately not used as a substitute: it is an idempotency key, not the
	// execution identity.
	if w.remote != nil {
		commandID, payloadHash := w.remote.CommandID(fence)
		_, dispatchErr := w.remote.Dispatcher.DispatchFence(executionCtx, fence, commandID, payloadHash, w.cfg.Lease, w.remote.Provider)
		if dispatchErr != nil {
			return
		}
	}
	executeCtx := context.WithValue(executionCtx, types.RunIDContextKey, fence.RunID)
	err := w.execute(executeCtx, fence)
	if executionCtx.Err() != nil {
		return
	}
	if current, getErr := w.store.Get(context.Background(), fence.RunKey); getErr == nil && current.Status == "waiting_user" {
		return
	}
	status, reason := "succeeded", ""
	if err != nil {
		// Wait-class failures park durably instead of terminating: the run
		// stays recoverable and only an explicit user decision resumes it.
		// If the park itself loses the fence, the run remains non-terminal
		// and the expiring lease triggers a bounded reclaim.
		if reason, wait := durableWaitReason(err); wait {
			if parkErr := w.store.SetStatus(context.Background(), fence, "waiting_user", reason); parkErr == nil {
				return
			}
			return
		}
		status, reason = "failed", err.Error()
	}
	// A terminal state is durable and fenced; a cancelled/draining worker
	// leaves the run non-terminal for lease based takeover.
	if err := w.store.SetStatus(context.Background(), fence, status, reason); err != nil {
		return
	}
}

// durableWaitReason classifies executor failures that must park a run at
// waiting_user instead of terminating it. Unknown tool outcomes and sandbox
// unavailability are durable waits; provider-query gaps park as well until the
// provider adapter integrates task-level observation.
func durableWaitReason(err error) (string, bool) {
	switch {
	case errors.Is(err, agentruntime.ErrToolWaitUser):
		return "tool_outcome_unknown", true
	case errors.Is(err, agentruntime.ErrToolRecoveryQuery):
		return "tool_outcome_query_required", true
	case errors.Is(err, ErrSandboxUnavailable):
		return "sandbox_unavailable", true
	default:
		return "", false
	}
}

// Cancel requests best-effort cancellation of an active worker for a durable run.
func (w *AgentRunWorker) Cancel(key agentruntime.RunKey) error {
	if w == nil || key.TenantID == 0 || key.RunID == "" {
		return agentruntime.ErrConflict
	}
	id := fmt.Sprintf("%d/%s", key.TenantID, key.RunID)
	w.mu.Lock()
	cancel, ok := w.active[id]
	w.mu.Unlock()
	if ok {
		cancel()
	}
	return nil
}

func (w *AgentRunWorker) drain() {
	w.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(w.active))
	for _, c := range w.active {
		cancels = append(cancels, c)
	}
	w.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for {
		w.mu.Lock()
		n := len(w.active)
		w.mu.Unlock()
		if n == 0 {
			return
		}
		select {
		case <-deadline.C:
			return
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// Wait blocks until the worker loop exits, bounded by timeout.
func (w *AgentRunWorker) Wait(timeout time.Duration) bool {
	if w == nil {
		return true
	}
	if timeout <= 0 {
		<-w.done
		return true
	}
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case <-w.done:
		return true
	case <-t.C:
		return false
	}
}
