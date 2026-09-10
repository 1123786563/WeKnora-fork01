package service

import (
	"context"
	"errors"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
)

type agentRunLifecycleStore interface {
	CancelRun(context.Context, agentruntime.RunKey, string) error
	DeleteSessionRuns(context.Context, uint64, string) error
}

// Cancel records a durable terminal cancellation before returning to the caller.
// The repository owns the CAS and active-slot cleanup so an old worker cannot
// recreate or take over the run after cancellation.
func (s *AgentRunService) Cancel(ctx context.Context, key agentruntime.RunKey) error {
	if s == nil || s.store == nil {
		return errors.New("agent run store is required")
	}
	if key.TenantID == 0 || key.RunID == "" {
		return agentruntime.ErrConflict
	}
	store, ok := s.store.(agentRunLifecycleStore)
	if !ok {
		return errors.New("agent run store does not support lifecycle")
	}
	err := store.CancelRun(ctx, key, "user_canceled")
	if err != nil {
		return err
	}
	if s.cancelHook != nil {
		return s.cancelHook(ctx, key)
	}
	return nil
}

// DeleteSessionRuns durably fences and removes all runs belonging to a session.
func (s *AgentRunService) DeleteSessionRuns(ctx context.Context, tenantID uint64, sessionID string) error {
	if s == nil || s.store == nil {
		return errors.New("agent run store is required")
	}
	if tenantID == 0 || sessionID == "" {
		return agentruntime.ErrConflict
	}
	store, ok := s.store.(agentRunLifecycleStore)
	if !ok {
		return errors.New("agent run store does not support lifecycle")
	}
	return store.DeleteSessionRuns(ctx, tenantID, sessionID)
}
