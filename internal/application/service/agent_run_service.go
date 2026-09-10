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
type AgentRunService struct {
	store agentruntime.RunStore
	wake  func()
	mu    sync.Mutex
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
