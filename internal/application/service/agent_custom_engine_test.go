package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// TestAgentQARejectsCustomAgentOnBuiltinSession pins the routing rule:
// custom agents run exclusively on the tRPC engine. A custom agent
// arriving on a builtin-engine session is rejected with an actionable
// error instead of silently falling back to the ReAct loop.
func TestAgentQARejectsCustomAgentOnBuiltinSession(t *testing.T) {
	// The builtin-agent check reads the builtin registry; load it the way
	// the server does so builtin ids classify correctly in the test too.
	if err := types.LoadBuiltinAgentsConfig("../../config"); err != nil {
		t.Skipf("builtin agent config unavailable: %v", err)
	}
	svc := &sessionService{}
	req := &types.QARequest{
		Query:       "custom agent on builtin engine",
		Session:     &types.Session{TenantID: 1, ID: "s1", UserID: "u1", EngineType: "builtin"},
		CustomAgent: &types.CustomAgent{ID: "agent-custom-1"},
	}
	err := svc.AgentQA(context.Background(), req, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "custom agents run on the tRPC engine")

	// A builtin agent id never trips the guard: the discriminator matches
	// the client rule, so builtin agents keep both engines available.
	require.True(t, isBuiltinAgentID("builtin-smart-reasoning"))
	require.True(t, isBuiltinAgentID("builtin-quick-answer"))
	require.False(t, isBuiltinAgentID("agent-custom-1"))
}
