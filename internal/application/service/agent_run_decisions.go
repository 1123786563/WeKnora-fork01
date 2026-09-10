package service

import (
	"context"
	"encoding/json"
	"fmt"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
)

const maxDecisionResultEnvelope = 1 << 20

type decisionResolver interface {
	ApplyDecision(context.Context, agentruntime.RunKey, string, agentruntime.Decision) (agentruntime.Run, error)
}

// ValidateDecision checks the user action before it reaches durable storage.
func ValidateDecision(d agentruntime.Decision) error {
	if d.PendingID == "" || d.DecisionID == "" {
		return fmt.Errorf("pending_id and decision_id are required")
	}
	switch d.Action {
	case "retry":
		if d.Reason == "" {
			return fmt.Errorf("retry reason is required")
		}
		if len(d.Result) != 0 {
			return fmt.Errorf("retry cannot include result")
		}
	case "provide_result":
		if d.Reason == "" {
			return fmt.Errorf("result reason is required")
		}
		if len(d.Result) == 0 || !json.Valid(d.Result) {
			return fmt.Errorf("provide_result requires valid JSON result")
		}
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(d.Result, &envelope); err != nil || envelope == nil {
			return fmt.Errorf("provide_result requires a JSON object envelope")
		}
		if len(d.Result) > maxDecisionResultEnvelope {
			return fmt.Errorf("result envelope exceeds 1 MiB")
		}
	case "terminate":
		if d.Reason == "" {
			return fmt.Errorf("termination reason is required")
		}
		if len(d.Result) != 0 {
			return fmt.Errorf("terminate cannot include result")
		}
	default:
		return fmt.Errorf("unsupported decision action: %q", d.Action)
	}
	return nil
}

// Resolve validates and durably applies a user decision. Actor identity is
// supplied by the authenticated caller and is checked again by the repository.
func (s *AgentRunService) Resolve(ctx context.Context, key agentruntime.RunKey, in agentruntime.Decision) (agentruntime.Run, error) {
	if s == nil || s.store == nil {
		return agentruntime.Run{}, fmt.Errorf("agent run store is required")
	}
	if err := ValidateDecision(in); err != nil {
		return agentruntime.Run{}, err
	}
	if s.decisionPolicy != nil {
		if err := s.decisionPolicy(ctx, in); err != nil {
			return agentruntime.Run{}, err
		}
	} else if in.Action == "retry" || in.Action == "provide_result" {
		return agentruntime.Run{}, fmt.Errorf("current approval policy is unavailable")
	}
	resolver, ok := s.store.(decisionResolver)
	if !ok {
		return agentruntime.Run{}, fmt.Errorf("agent run store does not support decisions")
	}
	principal, ok := types.PrincipalFromContext(ctx)
	if !ok || principal.StorageID() == "" {
		return agentruntime.Run{}, fmt.Errorf("authenticated decision actor is required")
	}
	return resolver.ApplyDecision(ctx, key, principal.StorageID(), in)
}
