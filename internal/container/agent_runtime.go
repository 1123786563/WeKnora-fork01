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
	workbenchservice "github.com/Tencent/WeKnora/internal/application/service/workbench"
	"github.com/Tencent/WeKnora/internal/config"
)

// AgentRuntime owns the durable worker lifecycle independently from HTTP.
type AgentRuntime struct {
	Runs   *service.AgentRunService
	Worker *service.AgentRunWorker
	once   sync.Once
}

func NewAgentRuntime(cfg *config.Config, store *repository.AgentRunStore) (*AgentRuntime, error) {
	return newAgentRuntime(cfg, store, nil, nil)
}

// NewAgentRuntimeWithRemoteProvider is the production assembly point for a
// Paseo-backed worker. The provider and durable dispatch store are explicit
// dependencies so an enabled runtime cannot accidentally invoke a provider
// outside the W20 intent/receipt fence.
func NewAgentRuntimeWithRemoteProvider(
	cfg *config.Config,
	store *repository.AgentRunStore,
	dispatch *repository.ExecutionDispatchStore,
	provider workbenchservice.RemoteProvider,
) (*AgentRuntime, error) {
	return newAgentRuntime(cfg, store, dispatch, provider)
}

func newAgentRuntime(cfg *config.Config, store *repository.AgentRunStore, dispatch *repository.ExecutionDispatchStore, provider workbenchservice.RemoteProvider) (*AgentRuntime, error) {
	if store == nil {
		return nil, errors.New("agent run store is required")
	}
	if err := ValidateAgentRuntimeConfig(cfg); err != nil {
		return nil, err
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
	execute := func(context.Context, agentruntime.Fence) error { return errors.New("trpc graph executor is not wired") }
	var worker *service.AgentRunWorker
	if provider != nil {
		if dispatch == nil {
			return nil, errors.New("remote dispatch store is required when a provider is configured")
		}
		worker, err = service.NewAgentRunWorkerWithRemoteDispatch(store, execute, service.RemoteDispatchConfig{
			Dispatcher: workbenchservice.NewRemoteDispatcher(dispatch), Provider: provider,
			CommandID: func(fence agentruntime.Fence) (string, string) {
				return fence.RunID + "/" + fmt.Sprint(fence.Epoch), ""
			},
		}, c)
	} else {
		worker, err = service.NewAgentRunWorker(store, execute, c)
	}
	if err != nil {
		return nil, err
	}
	var wake func()
	wake = func() {}
	return &AgentRuntime{Runs: service.NewAgentRunService(store, wake), Worker: worker}, nil
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

// ValidateAgentRuntimeConfig is called by startup wiring before constructing
// any tRPC graph resources.
func ValidateAgentRuntimeConfig(cfg *config.Config) error {
	if cfg == nil || cfg.Agent == nil || !cfg.Agent.Recovery.Enabled {
		return nil
	}
	c := service.DefaultWorkerConfig()
	r := cfg.Agent.Recovery
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
