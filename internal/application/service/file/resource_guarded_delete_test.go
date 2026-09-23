package file_test

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"strings"
	"sync"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	file "github.com/Tencent/WeKnora/internal/application/service/file"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type guardedDeleteAPI interface {
	DeleteUnbound(context.Context, uint64, string) (bool, error)
}

type guardedPhysicalFiles struct {
	interfaces.FileService
	mu        sync.Mutex
	deletes   int
	deleteErr error
	entered   chan struct{}
	resume    chan struct{}
	enterOnce sync.Once
}

func (f *guardedPhysicalFiles) SaveBytes(context.Context, []byte, uint64, string, bool) (string, error) {
	return "local://7/career-source.txt", nil
}
func (f *guardedPhysicalFiles) DeleteFile(_ context.Context, path string) error {
	f.mu.Lock()
	f.deletes++
	err := f.deleteErr
	f.mu.Unlock()
	if f.entered != nil {
		f.enterOnce.Do(func() { close(f.entered) })
		<-f.resume
	}
	return err
}
func (f *guardedPhysicalFiles) GetFile(context.Context, string) (io.ReadCloser, error) {
	return nil, nil
}
func (f *guardedPhysicalFiles) SaveFile(context.Context, *multipart.FileHeader, uint64, string) (string, error) {
	return "", nil
}

func guardedFixture(t *testing.T) (interfaces.ResourceCatalog, interfaces.FileService, *guardedPhysicalFiles, *gorm.DB, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.StoredResource{}, &types.ResourceBinding{}, &types.ResourceAccessGrant{}))
	catalog := service.NewResourceCatalog(repository.NewResourceRepository(db))
	physical := &guardedPhysicalFiles{}
	files := file.NewResourceCatalogFileService(physical, catalog)
	ref, err := files.SaveBytes(context.Background(), []byte("resume"), 7, "career-source.txt", false)
	require.NoError(t, err)
	return catalog, files, physical, db, ref
}

func TestGuardedDeletePreservesNewBindingDuringPhysicalDelete(t *testing.T) {
	catalog, files, physical, db, ref := guardedFixture(t)
	guarded, ok := files.(guardedDeleteAPI)
	require.True(t, ok, "catalog-backed files must expose guarded deletion")
	ctx := context.Background()
	deleted, err := guarded.DeleteUnbound(ctx, 9, ref)
	require.NoError(t, err)
	require.False(t, deleted, "another tenant cannot claim physical deletion")
	require.NoError(t, catalog.Bind(ctx, ref, "career_source", "source-1", types.ResourceRelationSourceFile))
	_, err = catalog.Release(ctx, ref, "career_source", "source-1")
	require.NoError(t, err)
	require.NoError(t, catalog.Bind(ctx, ref, "other", "owner-2", types.ResourceRelationAttachment))
	deleted, err = guarded.DeleteUnbound(ctx, 7, ref)
	require.NoError(t, err)
	require.False(t, deleted)
	require.Equal(t, 0, physical.deletes)
	_, err = catalog.Release(ctx, ref, "other", "owner-2")
	require.NoError(t, err)
	physical.entered, physical.resume = make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, deleteErr := guarded.DeleteUnbound(ctx, 7, ref)
		done <- deleteErr
	}()
	<-physical.entered
	deleted, err = guarded.DeleteUnbound(ctx, 7, ref)
	require.NoError(t, err)
	require.False(t, deleted, "second cleaner must not enter physical deletion")
	require.ErrorIs(t, catalog.Bind(ctx, ref, "other", "owner-3", types.ResourceRelationAttachment), interfaces.ErrResourceUnavailable)
	close(physical.resume)
	require.NoError(t, <-done)
	require.Equal(t, 1, physical.deletes)
	var resource types.StoredResource
	require.NoError(t, db.Unscoped().Where("handle=?", ref[len(types.ResourceScheme):]).First(&resource).Error)
	require.Equal(t, types.ResourceStateDeleted, resource.State)
	deleted, err = guarded.DeleteUnbound(ctx, 7, ref)
	require.NoError(t, err)
	require.True(t, deleted, "same tenant may complete cleanup from terminal catalog state")
	require.Equal(t, 1, physical.deletes, "terminal cleanup must not repeat physical deletion")
}

func TestGuardedDeleteRetainsRetryableStateAfterPhysicalFailure(t *testing.T) {
	catalog, files, physical, db, ref := guardedFixture(t)
	guarded, ok := files.(guardedDeleteAPI)
	require.True(t, ok)
	ctx := context.Background()
	physical.deleteErr = errors.New("disk busy")
	deleted, err := guarded.DeleteUnbound(ctx, 7, ref)
	require.ErrorContains(t, err, "disk busy")
	require.False(t, deleted)
	var resource types.StoredResource
	require.NoError(t, db.Where("handle=?", ref[len(types.ResourceScheme):]).First(&resource).Error)
	require.Equal(t, "deleting", resource.State)
	require.ErrorIs(t, catalog.Bind(ctx, ref, "other", "new-owner", types.ResourceRelationAttachment), interfaces.ErrResourceUnavailable)
	physical.deleteErr = nil
	deleted, err = guarded.DeleteUnbound(ctx, 7, ref)
	require.NoError(t, err)
	require.True(t, deleted)
	require.Equal(t, 2, physical.deletes)
	require.NoError(t, db.Unscoped().Where("id=?", resource.ID).First(&resource).Error)
	require.Equal(t, types.ResourceStateDeleted, resource.State)
	deleted, err = guarded.DeleteUnbound(ctx, 9, ref)
	require.NoError(t, err)
	require.False(t, deleted, "other tenant cannot delete")
	deleted, err = guarded.DeleteUnbound(ctx, 7, types.BuildResourcePath(strings.Repeat("x", types.ResourceHandleLength)))
	require.NoError(t, err)
	require.False(t, deleted, "unknown handle has no terminal catalog state")
	require.Equal(t, 2, physical.deletes)
}

func TestGuardedDeleteDoesNotTreatActiveOrUnknownDatabaseStateAsDeleted(t *testing.T) {
	_, files, physical, db, ref := guardedFixture(t)
	guarded := files.(guardedDeleteAPI)
	ctx := context.Background()
	// The active resource has a binding, so no physical deletion is allowed.
	require.NoError(t, db.Create(&types.ResourceBinding{ResourceID: resourceIDForRef(t, db, ref), TenantID: 7, OwnerType: "other", OwnerID: "owner", Relation: types.ResourceRelationAttachment}).Error)
	deleted, err := guarded.DeleteUnbound(ctx, 7, ref)
	require.NoError(t, err)
	require.False(t, deleted)
	require.Equal(t, 0, physical.deletes)

	// A catalog query failure is unknown, never proof that bytes are gone.
	require.NoError(t, db.Migrator().DropTable(&types.StoredResource{}))
	deleted, err = guarded.DeleteUnbound(ctx, 7, ref)
	require.Error(t, err)
	require.False(t, deleted)
	require.Equal(t, 0, physical.deletes)
}

func resourceIDForRef(t *testing.T, db *gorm.DB, ref string) string {
	t.Helper()
	var resource types.StoredResource
	require.NoError(t, db.Where("handle=?", strings.TrimPrefix(ref, types.ResourceScheme)).First(&resource).Error)
	return resource.ID
}
