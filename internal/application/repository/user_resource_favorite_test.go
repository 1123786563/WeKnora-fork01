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

// Characterization tests for the user resource favorite repository,
// anchored on the legacy host-package implementation before the Pass B (25a)
// move to internal/agentcatalog/repository.

func newFavoriteTestRepo(t *testing.T) (interfaces.UserResourceFavoriteRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.UserResourceFavorite{}))
	return NewUserResourceFavoriteRepository(db), db
}

func TestFavoriteAddIsIdempotentAndReportsCreated(t *testing.T) {
	repo, db := newFavoriteTestRepo(t)
	ctx := context.Background()

	created, err := repo.Add(ctx, "user-1", 7, types.ResourceTypeKB, "kb-1")
	require.NoError(t, err)
	require.True(t, created, "the first add inserts a row")

	created, err = repo.Add(ctx, "user-1", 7, types.ResourceTypeKB, "kb-1")
	require.NoError(t, err)
	require.False(t, created, "a repeated add collapses into the existing row, created=false")

	var count int64
	require.NoError(t, db.Model(&types.UserResourceFavorite{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestFavoriteListFiltersByResourceTypeAndTenant(t *testing.T) {
	repo, _ := newFavoriteTestRepo(t)
	ctx := context.Background()

	_, err := repo.Add(ctx, "user-1", 7, types.ResourceTypeKB, "kb-1")
	require.NoError(t, err)
	_, err = repo.Add(ctx, "user-1", 7, types.ResourceTypeAgent, "agent-1")
	require.NoError(t, err)
	_, err = repo.Add(ctx, "user-1", 8, types.ResourceTypeKB, "kb-other-tenant")
	require.NoError(t, err)

	list, err := repo.List(ctx, "user-1", 7, types.ResourceTypeKB)
	require.NoError(t, err)
	require.Len(t, list, 1, "list filters by resource type and must not leak another tenant's rows")
	require.Equal(t, "kb-1", list[0].ResourceID)
	require.Equal(t, types.ResourceTypeKB, list[0].ResourceType)
}

func TestFavoriteIsFavoriteReportsExistenceWithoutError(t *testing.T) {
	repo, _ := newFavoriteTestRepo(t)
	ctx := context.Background()

	ok, err := repo.IsFavorite(ctx, "user-1", 7, types.ResourceTypeKB, "kb-1")
	require.NoError(t, err)
	require.False(t, ok, "an absent favorite is (false, nil), not an error")

	_, err = repo.Add(ctx, "user-1", 7, types.ResourceTypeKB, "kb-1")
	require.NoError(t, err)
	ok, err = repo.IsFavorite(ctx, "user-1", 7, types.ResourceTypeKB, "kb-1")
	require.NoError(t, err)
	require.True(t, ok)
}

func TestFavoriteRemoveIsScopedAndReportsDeleted(t *testing.T) {
	repo, db := newFavoriteTestRepo(t)
	ctx := context.Background()

	_, err := repo.Add(ctx, "user-1", 7, types.ResourceTypeKB, "kb-1")
	require.NoError(t, err)
	_, err = repo.Add(ctx, "user-1", 7, types.ResourceTypeKB, "kb-2")
	require.NoError(t, err)

	deleted, err := repo.Remove(ctx, "user-1", 7, types.ResourceTypeKB, "kb-1")
	require.NoError(t, err)
	require.True(t, deleted, "removing an existing favorite reports deleted=true")

	deleted, err = repo.Remove(ctx, "user-1", 7, types.ResourceTypeKB, "kb-1")
	require.NoError(t, err)
	require.False(t, deleted, "removing an absent favorite reports deleted=false, still no error")

	deleted, err = repo.Remove(ctx, "user-1", 8, types.ResourceTypeKB, "kb-2")
	require.NoError(t, err)
	require.False(t, deleted, "another tenant's remove must not reach this row")

	var count int64
	require.NoError(t, db.Model(&types.UserResourceFavorite{}).Count(&count).Error)
	require.EqualValues(t, 1, count, "only kb-2 remains")
}
