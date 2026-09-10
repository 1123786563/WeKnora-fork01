package container

import (
	"fmt"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/config"
)

// ValidateAgentRuntimeConfig is called by startup wiring before constructing
// any tRPC graph resources. Disabled recovery deliberately requires no
// runtime dependencies.
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

// AgentRuntime is the lifecycle boundary for the durable worker. Keeping the
// worker behind this small type makes shutdown ownership explicit and avoids
// coupling it to HTTP request contexts.
type AgentRuntime struct{ Worker *service.AgentRunWorker }
