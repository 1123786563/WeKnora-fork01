package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSemanticModelPolicyRepositoryPreservesRevisions(t *testing.T) {
	db := newSemanticSQLiteTestDB(t)
	repo := NewSemanticModelPolicyRepository(db)
	policy := SemanticModelPolicy{
		TenantID: 1, KBID: "kb-policy", ModelCallsEnabled: true, ModelID: "owner-chat",
		Funding: "platform", PriceVersion: "price-v1", MaxInputTokensPerCall: 100,
		MaxOutputTokensPerCall: 200, MaxCallsPerTask: 3, MaxInputTokensPerTask: 300,
		MaxOutputTokensPerTask: 600, TaskUpperMicro: int64ptr(9), UpdatedBy: "owner-1",
	}
	first, err := repo.Put(context.Background(), policy)
	require.NoError(t, err)
	require.Equal(t, uint64(1), first.PolicyVersion)

	policy.ModelCallsEnabled = false
	second, err := repo.Put(context.Background(), policy)
	require.NoError(t, err)
	require.Equal(t, uint64(2), second.PolicyVersion)

	current, err := repo.Get(context.Background(), 1, "kb-policy")
	require.NoError(t, err)
	require.False(t, current.ModelCallsEnabled)
	revisions, err := repo.ListRevisions(context.Background(), 1, "kb-policy")
	require.NoError(t, err)
	require.Len(t, revisions, 2)
	require.True(t, revisions[0].ModelCallsEnabled)
}

func int64ptr(v int64) *int64 { return &v }
