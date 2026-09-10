package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
)

// AgentRunService admits durable tRPC runs. The admission payload is copied
// before it is handed to storage so request-owned buffers cannot be changed
// after admission.
var defaultAgentRunService *AgentRunService

// RegisterAgentRunService makes the durable admission boundary available to
// the session service after container construction.
func RegisterAgentRunService(s *AgentRunService)  { defaultAgentRunService = s }
func RegisteredAgentRunService() *AgentRunService { return defaultAgentRunService }

type AgentRunService struct {
	store          agentruntime.RunStore
	wake           func()
	decisionPolicy func(context.Context, agentruntime.Decision) error
	cancelHook     func(context.Context, agentruntime.RunKey) error
	mu             sync.Mutex
}

// SetDecisionPolicy installs the current authorization check used immediately
// before a durable decision is consumed.
func (s *AgentRunService) SetDecisionPolicy(check func(context.Context, agentruntime.Decision) error) {
	if s != nil {
		s.decisionPolicy = check
	}
}

// WaitForDecision durably parks a leased run. Resolve later queues it, allowing
// the worker to resume after a restart without an in-process waiter.
func (s *AgentRunService) WaitForDecision(ctx context.Context, fence agentruntime.Fence, pendingID string) error {
	if s == nil || s.store == nil || pendingID == "" {
		return agentruntime.ErrConflict
	}
	return s.store.SetStatus(ctx, fence, "waiting_user", pendingID)
}

func NewAgentRunService(store agentruntime.RunStore, wake ...func()) *AgentRunService {
	var notify func()
	if len(wake) > 0 {
		notify = wake[0]
	}
	return &AgentRunService{store: store, wake: notify}
}

func (s *AgentRunService) Submit(ctx context.Context, in agentruntime.Admission) (agentruntime.Run, error) {
	if s == nil || s.store == nil {
		return agentruntime.Run{}, fmt.Errorf("agent run store is required")
	}
	// Deep-copy every JSON value. Admission is a durable boundary and must
	// never retain a caller's mutable backing array.
	copyJSON := func(v json.RawMessage) (json.RawMessage, error) {
		if len(v) == 0 || !json.Valid(v) {
			return nil, agentruntime.ErrConflict
		}
		return append(json.RawMessage(nil), v...), nil
	}
	var err error
	if in.Snapshot, err = copyJSON(in.Snapshot); err != nil {
		return agentruntime.Run{}, err
	}
	if in.UserMessage, err = copyJSON(in.UserMessage); err != nil {
		return agentruntime.Run{}, err
	}
	if in.AssistantMessage, err = copyJSON(in.AssistantMessage); err != nil {
		return agentruntime.Run{}, err
	}
	run, err := s.store.Admit(ctx, in)
	if err == nil && s.wake != nil {
		s.wake()
	}
	return run, err
}

// Store exposes the durable store to transport adapters that only need the
// read-side event contract; mutations remain on this service.
func (s *AgentRunService) Store() agentruntime.RunStore {
	if s == nil {
		return nil
	}
	return s.store
}

// Get returns the durable run view for transport read paths.
func (s *AgentRunService) Get(ctx context.Context, key agentruntime.RunKey) (agentruntime.Run, error) {
	if s == nil || s.store == nil {
		return agentruntime.Run{}, fmt.Errorf("agent run store is required")
	}
	return s.store.Get(ctx, key)
}

// SetCancelHook installs best-effort active execution cancellation. Durable cancellation is recorded first.
func (s *AgentRunService) SetCancelHook(hook func(context.Context, agentruntime.RunKey) error) {
	if s != nil {
		s.cancelHook = hook
	}
}
