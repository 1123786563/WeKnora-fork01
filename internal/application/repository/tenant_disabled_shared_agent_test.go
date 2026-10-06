package repository

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Characterization tests for the tenant disabled-shared-agent repository,
// anchored on the legacy host-package implementation before the Pass B (25a)
// move to internal/modules/agentcatalog/repository.

func newDisabledSharedAgentTestRepo(t *testing.T) (interfaces.TenantDisabledSharedAgentRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.TenantDisabledSharedAgent{}))
	return NewTenantDisabledSharedAgentRepository(db), db
}

func TestDisabledSharedAgentAddIsIdempotent(t *testing.T) {
	repo, db := newDisabledSharedAgentTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Add(ctx, 7, "agent-a", 42))
	require.NoError(t, repo.Add(ctx, 7, "agent-a", 42),
		"re-adding the same (tenant, agent, source) must not error nor duplicate")

	var count int64
	require.NoError(t, db.Model(&types.TenantDisabledSharedAgent{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestDisabledSharedAgentListDisabledOwnAgentIDsScopesToOwnSource(t *testing.T) {
	repo, _ := newDisabledSharedAgentTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Add(ctx, 7, "own-agent", 7))
	require.NoError(t, repo.Add(ctx, 7, "foreign-agent", 84))

	ids, err := repo.ListDisabledOwnAgentIDs(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, []string{"own-agent"}, ids,
		"only rows the tenant disabled for itself (source_tenant_id == tenant_id) are listed")
}

func TestDisabledSharedAgentListByTenantIDIsTenantScoped(t *testing.T) {
	repo, _ := newDisabledSharedAgentTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Add(ctx, 7, "agent-a", 42))
	require.NoError(t, repo.Add(ctx, 8, "agent-b", 42))

	list, err := repo.ListByTenantID(ctx, 7)
	require.NoError(t, err)
	require.Len(t, list, 1, "another tenant's rows must not leak into the listing")
	require.Equal(t, "agent-a", list[0].AgentID)
	require.Equal(t, uint64(42), list[0].SourceTenantID)
}

func TestDisabledSharedAgentRemoveDeletesOnlyTheScopedRow(t *testing.T) {
	repo, db := newDisabledSharedAgentTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Add(ctx, 7, "agent-a", 42))
	require.NoError(t, repo.Add(ctx, 7, "agent-a", 84))
	require.NoError(t, repo.Add(ctx, 8, "agent-a", 42))

	require.NoError(t, repo.Remove(ctx, 7, "agent-a", 42))

	var remaining []types.TenantDisabledSharedAgent
	require.NoError(t, db.Find(&remaining).Error)
	require.Len(t, remaining, 2, "remove deletes exactly the (tenant, agent, source) scoped row")
	for _, rec := range remaining {
		notTheRemovedRow := rec.TenantID != 7 || rec.SourceTenantID != 42
		require.True(t, notTheRemovedRow, "the removed (7, agent-a, 42) row must be gone")
	}

	list, err := repo.ListByTenantID(ctx, 7)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, uint64(84), list[0].SourceTenantID)
}
