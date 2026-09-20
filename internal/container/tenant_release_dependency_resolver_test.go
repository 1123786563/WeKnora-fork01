package container

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestTenantReleaseDependencyResolverNamesMissingDependencies(t *testing.T) {
	version := types.AgentVersionSnapshot{
		Agent: &types.CustomAgent{Config: types.CustomAgentConfig{
			SelectedSkills: []string{"tenant-skill-a"},
			Subagents:      []string{"tenant-subagent-b"},
		}},
	}
	_, err := (tenantReleaseDependencyResolver{}).Resolve(context.Background(), 1, version)
	require.Error(t, err)
	require.Contains(t, err.Error(), `skill "tenant-skill-a"`)
	require.Contains(t, err.Error(), `subagent "tenant-subagent-b"`)
}
