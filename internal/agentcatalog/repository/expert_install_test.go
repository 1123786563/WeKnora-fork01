package repository

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newExpertInstallTestRepo(t *testing.T) ExpertInstallRepository {
	t.Helper()
	repo, _ := newExpertInstallTestRepoWithDB(t)
	return repo
}

// newExpertInstallTestRepoWithDB also hands back the handle, for the tests
// that must read a row raw to prove a delete was soft rather than destructive.
func newExpertInstallTestRepoWithDB(t *testing.T) (ExpertInstallRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.ExpertInstallEntity{}))
	// AutoMigrate cannot express the partial unique index, so add it here to
	// match the production migration.
	require.NoError(t, db.Exec(
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_expert_installs_scope
		 ON expert_installs (tenant_id, slug) WHERE deleted_at IS NULL`).Error)
	return NewExpertInstallRepository(db), db
}

func expertInstallRow(id, slug, ref, digest string) *types.ExpertInstallEntity {
	return &types.ExpertInstallEntity{
		ID: id, TenantID: 7, Slug: slug,
		StorageRef: ref, SnapshotSHA256: digest,
		CreatedBy: "user-1",
	}
}

func TestExpertInstallUpsertIsIdempotentPerTenantSlug(t *testing.T) {
	repo := newExpertInstallTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Upsert(ctx, expertInstallRow("ei-a", "pdf-tools",
		"/data/expert-market/7/pdf-tools", "digest-1")))
	require.NoError(t, repo.Upsert(ctx, expertInstallRow("ei-b", "pdf-tools",
		"/data/expert-market/7/pdf-tools", "digest-2")))

	list, err := repo.ListByTenant(ctx, 7)
	require.NoError(t, err)
	require.Len(t, list, 1, "a reinstall of the same (tenant, slug) replaces the row, it does not add one")
	require.Equal(t, "digest-2", list[0].SnapshotSHA256)
	require.Equal(t, "ei-a", list[0].ID, "the original row key stays stable across reinstalls")
}

func TestExpertInstallListIsTenantScopedAndOrdered(t *testing.T) {
	repo := newExpertInstallTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Upsert(ctx, expertInstallRow("ei-z", "zeta-skillset", "r-z", "d")))
	require.NoError(t, repo.Upsert(ctx, expertInstallRow("ei-a", "alpha-skillset", "r-a", "d")))
	other := expertInstallRow("ei-x", "alpha-skillset", "r-x", "d")
	other.TenantID = 8
	require.NoError(t, repo.Upsert(ctx, other))

	list, err := repo.ListByTenant(ctx, 7)
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, "alpha-skillset", list[0].Slug, "ordered by slug")
	require.Equal(t, "zeta-skillset", list[1].Slug)
}

func TestExpertInstallGetByTenantAndSlug(t *testing.T) {
	repo := newExpertInstallTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Upsert(ctx, expertInstallRow("ei-a", "pdf-tools", "ref", "digest")))

	got, err := repo.GetByTenantAndSlug(ctx, 7, "pdf-tools")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "ref", got.StorageRef)
	require.Equal(t, "digest", got.SnapshotSHA256)

	got, err = repo.GetByTenantAndSlug(ctx, 7, "ghost")
	require.NoError(t, err)
	require.Nil(t, got, "a missing install is nil, not an error")

	got, err = repo.GetByTenantAndSlug(ctx, 8, "pdf-tools")
	require.NoError(t, err)
	require.Nil(t, got, "installs never leak across tenants")
}

func TestExpertInstallDeleteIsSoftAndSlotFrees(t *testing.T) {
	repo, db := newExpertInstallTestRepoWithDB(t)
	ctx := context.Background()

	require.NoError(t, repo.Upsert(ctx, expertInstallRow("ei-a", "pdf-tools", "ref", "digest")))
	require.NoError(t, repo.Delete(ctx, 7, "pdf-tools"))

	var live, dead int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM expert_installs WHERE deleted_at IS NULL`).Scan(&live).Error)
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM expert_installs WHERE deleted_at IS NOT NULL`).Scan(&dead).Error)
	require.EqualValues(t, 0, live, "no live row survives the delete")
	require.EqualValues(t, 1, dead, "the row is soft-deleted, not dropped")

	got, err := repo.GetByTenantAndSlug(ctx, 7, "pdf-tools")
	require.NoError(t, err)
	require.Nil(t, got, "a soft-deleted install is invisible to reads")

	// The freed slot accepts a fresh row under a new key.
	require.NoError(t, repo.Upsert(ctx, expertInstallRow("ei-b", "pdf-tools", "ref-2", "digest-2")))
	list, err := repo.ListByTenant(ctx, 7)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "ei-b", list[0].ID)

	// Deleting a slot that matches nothing is a no-op, not an error.
	require.NoError(t, repo.Delete(ctx, 7, "ghost"))
}
