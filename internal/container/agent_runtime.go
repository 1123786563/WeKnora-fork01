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
)

func newAgentRuntime(cfg *config.Config, store *repository.AgentRunStore, resources *service.GormAgentRunResourceRepository) (*AgentRuntime, error) {
	r, err := NewAgentRuntime(cfg, store)
	if err != nil || r == nil || r.Worker == nil {
		return r, err
	}
	// The provider-specific adapter is selected by the sandbox manager at
	// execution time. Until it is available, park the claimed run durably;
	// never let the graph fabricate a successful result.
	r.Runs.SetCancelHook(func(_ context.Context, key agentruntime.RunKey) error { return r.Worker.Cancel(key) })
	r.SetRecoveryHook(func(ctx context.Context, fence agentruntime.Fence) error {
		_ = resources // used by the provider-backed adapter when it is configured
		_ = r.Runs.WaitForDecision(ctx, fence, "sandbox_unavailable")
		return service.ErrSandboxUnavailable
	})
	return r, nil
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
	c.Enabled = r.Enabled
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
	// Graph construction is injected by the tRPC graph task. Keeping this
	// executor explicit makes an enabled but unwired deployment fail runs
	// durably instead of pretending they completed.
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

func (r *AgentRuntime) Start(ctx context.Context) {
	if r == nil || r.Worker == nil {
		return
	}
	go func() {
		if err := r.Worker.Run(ctx); err != nil {
			_ = fmt.Errorf("agent worker stopped: %w", err)
		}
	}()
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
	return cfg != nil && cfg.Agent != nil && cfg.Agent.Recovery.Enabled && cfg.Agent.Recovery.AdmissionEnabled
}

// ValidateAgentRuntimeConfig is called by startup wiring before constructing
// any tRPC graph resources.
func ValidateAgentRuntimeConfig(cfg *config.Config) error {
	if cfg == nil || cfg.Agent == nil {
		return nil
	}
	r := cfg.Agent.Recovery
	if r.AdmissionEnabled && !r.Enabled {
		return errors.New("tRPC recovery admission requires the recovery worker to be enabled")
	}
	if !r.Enabled {
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
