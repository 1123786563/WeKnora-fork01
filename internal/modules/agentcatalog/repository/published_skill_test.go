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

func newPublishedSkillTestRepo(t *testing.T) PublishedSkillRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.PublishedSkillEntity{}))
	// AutoMigrate cannot express the partial unique index, so add it here to
	// match the production migration (the expert_installs pattern).
	require.NoError(t, db.Exec(
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_published_skills_scope
		 ON published_skills (tenant_id, catalog_id) WHERE deleted_at IS NULL`).Error)
	return NewPublishedSkillRepository(db)
}

func publishedRow(tenantID uint64, catalogID, by string) *types.PublishedSkillEntity {
	return &types.PublishedSkillEntity{
		ID: uuid.NewString(), TenantID: tenantID, CatalogID: catalogID, PublishedBy: by,
	}
}

func TestPublishedSkillRepoUpsertIsIdempotentPerScope(t *testing.T) {
	repo := newPublishedSkillTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Upsert(ctx, publishedRow(7, "cat-a", "user-1")))
	first, err := repo.GetByTenantAndCatalog(ctx, 7, "cat-a")
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Equal(t, "user-1", first.PublishedBy)
	require.NotEmpty(t, first.ID)

	// Re-publishing the same scope must reuse the row (new ID ignored) and
	// only refresh the publisher stamp.
	require.NoError(t, repo.Upsert(ctx, publishedRow(7, "cat-a", "user-2")))
	second, err := repo.GetByTenantAndCatalog(ctx, 7, "cat-a")
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, first.ID, second.ID, "re-publish must keep the same row key")
	require.Equal(t, "user-2", second.PublishedBy)
}

func TestPublishedSkillRepoListIsTenantScoped(t *testing.T) {
	repo := newPublishedSkillTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Upsert(ctx, publishedRow(7, "cat-a", "user-1")))
	require.NoError(t, repo.Upsert(ctx, publishedRow(7, "cat-b", "user-1")))
	require.NoError(t, repo.Upsert(ctx, publishedRow(9, "cat-a", "user-9")))

	rows, err := repo.ListByTenant(ctx, 7)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	ids := []string{rows[0].CatalogID, rows[1].CatalogID}
	require.ElementsMatch(t, []string{"cat-a", "cat-b"}, ids)
}

func TestPublishedSkillRepoGetIsTenantScoped(t *testing.T) {
	repo := newPublishedSkillTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Upsert(ctx, publishedRow(7, "cat-a", "user-1")))

	got, err := repo.GetByTenantAndCatalog(ctx, 9, "cat-a")
	require.NoError(t, err)
	require.Nil(t, got, "another tenant's publish row must read as absent, not as an error")

	got, err = repo.GetByTenantAndCatalog(ctx, 7, "cat-b")
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestPublishedSkillRepoDeleteSoftDeletesAndFreesScope(t *testing.T) {
	repo := newPublishedSkillTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Upsert(ctx, publishedRow(7, "cat-a", "user-1")))
	require.NoError(t, repo.Delete(ctx, 7, "cat-a"))

	got, err := repo.GetByTenantAndCatalog(ctx, 7, "cat-a")
	require.NoError(t, err)
	require.Nil(t, got, "unpublished rows must not surface")

	rows, err := repo.ListByTenant(ctx, 7)
	require.NoError(t, err)
	require.Empty(t, rows)

	// The soft-deleted slot frees the unique scope: a later publish of the
	// same catalog inserts fresh (the expert_installs reinstall semantics).
	require.NoError(t, repo.Upsert(ctx, publishedRow(7, "cat-a", "user-2")))
	got, err = repo.GetByTenantAndCatalog(ctx, 7, "cat-a")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "user-2", got.PublishedBy)

	// Deleting when nothing is live succeeds as a no-op.
	require.NoError(t, repo.Delete(ctx, 9, "nope"))
}
