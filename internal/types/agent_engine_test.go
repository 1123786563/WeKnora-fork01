package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAgentEngine(t *testing.T) {
	for _, raw := range []string{"", "builtin"} {
		got, err := ParseAgentEngine(raw)
		require.NoError(t, err)
		require.Equal(t, AgentEngineBuiltin, got)
	}

	got, err := ParseAgentEngine("trpc")
	require.NoError(t, err)
	require.Equal(t, AgentEngineTRPC, got)

	_, err = ParseAgentEngine("typo")
	require.Error(t, err)
}
