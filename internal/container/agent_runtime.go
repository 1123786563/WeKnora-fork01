package container

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/sandbox"
)

// newAgentRuntime assembles the durable tRPC runtime. The graph executor is
// resolved through service.RegisteredGraphExecutor: the session service that
// owns it is constructed after this provider in the dependency graph, while
// the worker only executes after the container has fully booted.
func newAgentRuntime(
	cfg *config.Config,
	store *repository.AgentRunStore,
	resources *service.GormAgentRunResourceRepository,
	pinner *service.SessionSandboxPinner,
	resolver sandbox.TenantSandboxResolver,
) (*AgentRuntime, error) {
	executor := func(ctx context.Context, fence agentruntime.Fence) error {
		run := service.RegisteredGraphExecutor()
		if run == nil {
			return errors.New("tRPC graph executor provider is not registered")
		}
		return run(ctx, fence)
	}
	r, err := NewAgentRuntime(cfg, store, executor)
	if err != nil || r == nil || r.Worker == nil {
		return r, err
	}
	r.Runs.SetCancelHook(func(_ context.Context, key agentruntime.RunKey) error { return r.Worker.Cancel(key) })
	r.SetRecoveryHook(newSandboxRecoveryHook(store, resources, r.Runs, pinner, resolver))
	return r, nil
}

// sessionBoundRecovery adapts the session-bound manager to the recovery
// contract. Observe performs the durable binding check plus a real provider
// sandbox-list query; CancelExecution is a best-effort release.
type sessionBoundRecovery struct{ mgr *sandbox.SessionBoundManager }

func (a sessionBoundRecovery) Observe(
	ctx context.Context, ref sandbox.ExecutionRef,
) (sandbox.ExecutionObservation, error) {
	return a.mgr.ObserveInstance(ctx, ref)
}

func (a sessionBoundRecovery) CancelExecution(ctx context.Context, _ sandbox.ExecutionRef) error {
	return a.mgr.Cleanup(ctx)
}

// newSandboxRecoveryHook reconciles sandbox-bound runs after a worker claims
// them. Runs without a durable resource reference proceed directly; runs with
// one are checked against the provider: an alive instance continues (tool
// outcomes are decided by the journal), a lost or unobservable instance parks
// the run durably as sandbox_unavailable instead of executing against a
// workspace that no longer exists.
func newSandboxRecoveryHook(
	store *repository.AgentRunStore,
	resources *service.GormAgentRunResourceRepository,
	runs *service.AgentRunService,
	pinner *service.SessionSandboxPinner,
	resolver sandbox.TenantSandboxResolver,
) func(context.Context, agentruntime.Fence) error {
	return func(ctx context.Context, fence agentruntime.Fence) error {
		resource, err := resources.Get(ctx, fence.TenantID, fence.RunID)
		if errors.Is(err, sandbox.ErrNotFound) {
			// No durable sandbox resource was recorded for this run yet.
			return nil
		}
		if err != nil {
			// Lookup failures are transient: leave the run non-terminal so the
			// expiring lease triggers a bounded reclaim.
			return err
		}
		run, err := store.Get(ctx, fence.RunKey)
		if err != nil {
			return err
		}
		recovery, err := resolveExecutionRecovery(ctx, resolver, pinner, resource, run.SessionID)
		if err != nil {
			_ = runs.WaitForDecision(ctx, fence, "sandbox_unavailable")
			return service.ErrSandboxUnavailable
		}
		if _, err := service.ReconcileExecution(ctx, resources, recovery, resource); err != nil {
			if errors.Is(err, service.ErrSandboxUnavailable) {
				_ = runs.WaitForDecision(ctx, fence, "sandbox_unavailable")
				return err
			}
			return err
		}
		return nil
	}
}

// resolveExecutionRecovery rebuilds the session-bound manager for the
// session's pinned sandbox config. A nil resolver (binding store unavailable
// at boot) or a non-session-bound backend is an explicit unavailable result,
// never a silent continue.
func resolveExecutionRecovery(
	ctx context.Context,
	resolver sandbox.TenantSandboxResolver,
	pinner *service.SessionSandboxPinner,
	resource service.AgentRunResource,
	sessionID string,
) (sandbox.ExecutionRecovery, error) {
	if resolver == nil {
		return nil, errors.New("sandbox resolver is unavailable")
	}
	if pinner == nil {
		return nil, errors.New("session sandbox pinner is unavailable")
	}
	configID, err := pinner.Read(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if configID == "" && resource.Ref.ConfigID != "" {
		configID = resource.Ref.ConfigID
	}
	mgr, err := resolver.Resolve(ctx, resource.Ref.TenantID, configID)
	if err != nil {
		return nil, err
	}
	bound, ok := mgr.(*sandbox.SessionBoundManager)
	if !ok {
		return nil, errors.New("sandbox backend does not support execution recovery")
	}
	return sessionBoundRecovery{mgr: bound}, nil
}

// AgentRuntime owns the durable worker lifecycle independently from HTTP.
type AgentRuntime struct {
	Runs   *service.AgentRunService
	Worker *service.AgentRunWorker
	once   sync.Once
}

// SetRecoveryHook wires sandbox reconciliation into the worker before it
// starts. The hook is intentionally an interface-free callback so sandbox
// adapters do not import the application container.
func (r *AgentRuntime) SetRecoveryHook(hook func(context.Context, agentruntime.Fence) error) {
	if r != nil && r.Worker != nil {
		r.Worker.SetRecoveryHook(hook)
	}
}

func NewAgentRuntime(cfg *config.Config, store *repository.AgentRunStore, executors ...func(context.Context, agentruntime.Fence) error) (*AgentRuntime, error) {
	if store == nil {
		return nil, errors.New("agent run store is required")
	}
	if err := ValidateAgentRuntimeConfig(cfg); err != nil {
		return nil, err
	}
	if cfg == nil || cfg.Agent == nil {
		return &AgentRuntime{Runs: service.NewAgentRunService(store)}, nil
	}
	r := cfg.Agent.Recovery
	c := service.DefaultWorkerConfig()
	c.Enabled = r.RecoveryEnabled()
	if r.Lease > 0 {
		c.Lease = r.Lease
	}
	if r.Heartbeat > 0 {
		c.Heartbeat = r.Heartbeat
	}
	if r.ScanInterval > 0 {
		c.ScanInterval = r.ScanInterval
	}
	if r.MaxWorkers > 0 {
		c.MaxWorkers = r.MaxWorkers
	}
	// Graph construction is injected by the session service through
	// RegisterGraphExecutor; keeping this executor explicit makes an enabled
	// but unwired deployment fail runs durably instead of pretending they
	// completed.
	var execute func(context.Context, agentruntime.Fence) error
	if len(executors) > 0 {
		execute = executors[0]
	}
	if c.Enabled && execute == nil {
		return nil, errors.New("tRPC recovery enabled but no graph executor provider is registered")
	}
	if execute == nil {
		execute = func(context.Context, agentruntime.Fence) error { return nil }
	}
	worker, err := service.NewAgentRunWorker(store, execute, c)
	if err != nil {
		return nil, err
	}
	var wake func()
	wake = func() {}
	runs := service.NewAgentRunService(store, wake)
	service.RegisterAgentRunService(runs)
	return &AgentRuntime{Runs: runs, Worker: worker}, nil
}

// Start launches the durable worker. An enabled runtime without a registered
// graph executor is a boot failure: the container has fully constructed every
// service by the time Start runs, so a missing executor means the durable path
// can never execute.
func (r *AgentRuntime) Start(ctx context.Context) error {
	if r == nil || r.Worker == nil {
		return nil
	}
	if r.Worker.Config().Enabled && service.RegisteredGraphExecutor() == nil {
		return errors.New("tRPC recovery enabled but the graph executor provider was never registered")
	}
	go func() {
		if err := r.Worker.Run(ctx); err != nil {
			_ = fmt.Errorf("agent worker stopped: %w", err)
		}
	}()
	return nil
}
func (r *AgentRuntime) Drain() {
	r.once.Do(func() {
		if r != nil && r.Worker != nil {
			r.Worker.Wait(time.Second)
		}
	})
}

// AgentRecoveryAdmissionEnabled reports whether new tRPC runs may be admitted.
// Enabled controls the worker; AdmissionEnabled controls new work, so an
// operator can close admission while allowing existing runs to drain.
func AgentRecoveryAdmissionEnabled(cfg *config.Config) bool {
	return cfg != nil && cfg.Agent != nil &&
		cfg.Agent.Recovery.RecoveryEnabled() && cfg.Agent.Recovery.RecoveryAdmissionEnabled()
}

// ValidateAgentRuntimeConfig is called by startup wiring before constructing
// any tRPC graph resources.
func ValidateAgentRuntimeConfig(cfg *config.Config) error {
	if cfg == nil || cfg.Agent == nil {
		return nil
	}
	r := cfg.Agent.Recovery
	if r.RecoveryAdmissionEnabled() && !r.RecoveryEnabled() {
		return errors.New("tRPC recovery admission requires the recovery worker to be enabled")
	}
	if !r.RecoveryEnabled() {
		return nil
	}
	c := service.DefaultWorkerConfig()
	if r.Lease > 0 {
		c.Lease = r.Lease
	}
	if r.Heartbeat > 0 {
		c.Heartbeat = r.Heartbeat
	}
	if r.ScanInterval > 0 {
		c.ScanInterval = r.ScanInterval
	}
	if r.MaxWorkers > 0 {
		c.MaxWorkers = r.MaxWorkers
	}
	if err := c.Validate(); err != nil {
		return fmt.Errorf("invalid agent recovery config: %w", err)
	}
	return nil
}
