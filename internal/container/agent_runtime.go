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

// AgentRuntime owns the durable worker lifecycle independently from HTTP.
type AgentRuntime struct {
	Runs   *service.AgentRunService
	Worker *service.AgentRunWorker
	once   sync.Once
}

func NewAgentRuntime(cfg *config.Config, store *repository.AgentRunStore) (*AgentRuntime, error) {
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
	worker, err := service.NewAgentRunWorker(store, func(context.Context, agentruntime.Fence) error { return errors.New("trpc graph executor is not wired") }, c)
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
