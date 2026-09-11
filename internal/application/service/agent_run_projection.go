package service

import (
	"context"
	"encoding/json"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
)

// EventsAfter filters an already ordered event page for a reconnecting client.
// Keeping this pure makes replay cursor handling deterministic in HTTP and SSE.
func EventsAfter(events []agentruntime.RunEvent, after int64) []agentruntime.RunEvent {
	out := make([]agentruntime.RunEvent, 0, len(events))
	for _, e := range events {
		if e.Seq > after {
			out = append(out, e)
		}
	}
	return out
}

type runEventStore interface {
	ReadEvents(context.Context, agentruntime.RunKey, int64, int) ([]agentruntime.RunEvent, error)
}
type runFinalizer interface {
	Finalize(context.Context, agentruntime.Fence, json.RawMessage) error
}

// AgentRunProjection is the application boundary used by graph runners and
// reconnect handlers. Persistence is injected so projection remains testable.
type AgentRunProjection struct {
	events    runEventStore
	finalizer runFinalizer
}

func NewAgentRunProjection(events runEventStore, finalizer ...runFinalizer) *AgentRunProjection {
	p := &AgentRunProjection{events: events}
	if len(finalizer) > 0 {
		p.finalizer = finalizer[0]
	}
	return p
}
func (p *AgentRunProjection) Replay(ctx context.Context, key agentruntime.RunKey, after, limit int) ([]agentruntime.RunEvent, error) {
	if p == nil || p.events == nil {
		return nil, agentruntime.ErrNotFound
	}
	return p.events.ReadEvents(ctx, key, int64(after), limit)
}
func (p *AgentRunProjection) Finalize(ctx context.Context, fence agentruntime.Fence, answer json.RawMessage) error {
	if p == nil || p.finalizer == nil {
		return agentruntime.ErrConflict
	}
	return p.finalizer.Finalize(ctx, fence, answer)
}
