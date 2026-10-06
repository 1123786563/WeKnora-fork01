package service

import (
	"encoding/json"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/stretchr/testify/require"
)

func TestValidateDecisionRejectsMissingResult(t *testing.T) {
	err := ValidateDecision(agentruntime.Decision{PendingID: "p1", DecisionID: "d1", Action: "provide_result"})
	require.Error(t, err)
}

func TestValidateDecisionRequiresRetryReason(t *testing.T) {
	err := ValidateDecision(agentruntime.Decision{PendingID: "p1", DecisionID: "d1", Action: "retry"})
	require.Error(t, err)
}

func TestValidateDecisionBoundsResultEnvelope(t *testing.T) {
	err := ValidateDecision(agentruntime.Decision{PendingID: "p1", DecisionID: "d1", Action: "provide_result", Reason: "ok", Result: json.RawMessage(`{"success":true,"output":"ok"}`)})
	require.NoError(t, err)
}
