package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Tencent/WeKnora/internal/types"
)

func newPublishedExpertTestRepo(t *testing.T) PublishedExpertRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.PublishedExpertEntity{}))
	// AutoMigrate cannot express the partial unique index, so add it here to
	// match the production migration (the published_skills pattern).
	require.NoError(t, db.Exec(
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_published_experts_scope
		 ON published_experts (tenant_id, agent_id) WHERE deleted_at IS NULL`).Error)
	return NewPublishedExpertRepository(db)
}

func publishedExpertRow(tenantID uint64, agentID, name, by string) *types.PublishedExpertEntity {
	return &types.PublishedExpertEntity{
		ID: uuid.NewString(), TenantID: tenantID, AgentID: agentID,
		Name: name, SnapshotRef: "/snap/" + agentID, SnapshotSHA256: "sha-" + agentID,
		PublishedBy: by,
	}
}

func TestPublishedExpertRepoUpsertIsIdempotentPerAgent(t *testing.T) {
	repo := newPublishedExpertTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Upsert(ctx, publishedExpertRow(7, "agent-a", "First name", "user-1")))
	first, err := repo.GetByTenantAndAgent(ctx, 7, "agent-a")
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Equal(t, "First name", first.Name)
	require.NotEmpty(t, first.ID)

	// Re-publishing the same agent must reuse the row (new ID ignored) and
	// refresh the snapshot stamp and metadata at publish time.
	secondRow := publishedExpertRow(7, "agent-a", "Renamed", "user-2")
	secondRow.SnapshotSHA256 = "sha-v2"
	require.NoError(t, repo.Upsert(ctx, secondRow))
	second, err := repo.GetByTenantAndAgent(ctx, 7, "agent-a")
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, first.ID, second.ID, "re-publish must keep the same row key")
	require.Equal(t, "Renamed", second.Name)
	require.Equal(t, "user-2", second.PublishedBy)
	require.Equal(t, "sha-v2", second.SnapshotSHA256)
}

func TestPublishedExpertRepoListIsTenantScoped(t *testing.T) {
	repo := newPublishedExpertTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Upsert(ctx, publishedExpertRow(7, "agent-a", "A", "user-1")))
	require.NoError(t, repo.Upsert(ctx, publishedExpertRow(7, "agent-b", "B", "user-1")))
	require.NoError(t, repo.Upsert(ctx, publishedExpertRow(9, "agent-c", "C", "user-9")))

	rows, err := repo.ListByTenant(ctx, 7)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	names := []string{rows[0].Name, rows[1].Name}
	require.ElementsMatch(t, []string{"A", "B"}, names)
}

func TestPublishedExpertRepoGetByIDIsTenantScoped(t *testing.T) {
	repo := newPublishedExpertTestRepo(t)
	ctx := context.Background()

	inserted := publishedExpertRow(7, "agent-a", "A", "user-1")
	require.NoError(t, repo.Upsert(ctx, inserted))

	got, err := repo.GetByTenantAndID(ctx, 7, inserted.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "agent-a", got.AgentID)

	// Another tenant's publish row must read as absent, not as an error.
	got, err = repo.GetByTenantAndID(ctx, 9, inserted.ID)
	require.NoError(t, err)
	require.Nil(t, got)

	got, err = repo.GetByTenantAndID(ctx, 7, uuid.NewString())
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestPublishedExpertRepoDeleteSoftDeletesAndFreesScope(t *testing.T) {
	repo := newPublishedExpertTestRepo(t)
	ctx := context.Background()

	inserted := publishedExpertRow(7, "agent-a", "A", "user-1")
	require.NoError(t, repo.Upsert(ctx, inserted))
	require.NoError(t, repo.Delete(ctx, 7, inserted.ID))

	got, err := repo.GetByTenantAndAgent(ctx, 7, "agent-a")
	require.NoError(t, err)
	require.Nil(t, got, "unpublished rows must not surface")

	rows, err := repo.ListByTenant(ctx, 7)
	require.NoError(t, err)
	require.Empty(t, rows)

	// The soft-deleted slot frees the unique scope: a later publish of the
	// same agent inserts fresh (the published_skills reinstall semantics).
	require.NoError(t, repo.Upsert(ctx, publishedExpertRow(7, "agent-a", "A2", "user-2")))
	got, err = repo.GetByTenantAndAgent(ctx, 7, "agent-a")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "A2", got.Name)

	// Deleting when nothing matches succeeds as a no-op.
	require.NoError(t, repo.Delete(ctx, 9, "missing"))
}
