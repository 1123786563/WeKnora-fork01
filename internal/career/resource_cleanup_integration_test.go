package career

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/application/service/file"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type careerDiskFiles struct {
	interfaces.FileService
	path string
}

func (f *careerDiskFiles) SaveBytes(_ context.Context, data []byte, _ uint64, _ string, _ bool) (string, error) {
	if err := os.WriteFile(f.path, data, 0600); err != nil {
		return "", err
	}
	return "local://" + f.path, nil
}

func (f *careerDiskFiles) DeleteFile(_ context.Context, path string) error {
	return os.Remove(strings.TrimPrefix(path, "local://"))
}

func TestCareerReconcileClearsSourceAfterCatalogAlreadyDeleted(t *testing.T) {
	o, ctx := testOffice(t)
	catalog := service.NewResourceCatalog(repository.NewResourceRepository(o.db))
	physical := &careerDiskFiles{path: filepath.Join(t.TempDir(), "resume.txt")}
	files := file.NewResourceCatalogFileService(physical, catalog)
	ref, err := files.SaveBytes(ctx, []byte("resume"), 1, "resume.txt", false)
	require.NoError(t, err)
	source, _, err := o.ClaimUpload(ctx, SourceUpload{FileName: "resume.txt", MIMEType: "text/plain", Size: 6, Digest: "digest", RequestID: "cleanup-retry", IntentHash: "intent"})
	require.NoError(t, err)
	require.NoError(t, o.PersistUploadResource(ctx, source.ID, source.ClaimToken, ref))
	require.NoError(t, catalog.Bind(ctx, ref, careerSourceOwner, source.ID, types.ResourceRelationSourceFile))
	source, err = o.FinishSourceClaim(ctx, source.ID, source.ClaimToken, "", nil, nil, errors.New("parser unavailable"))
	require.NoError(t, err)
	require.Equal(t, "cleanup_pending_parse_failed", source.ErrorCategory)

	failClear := true
	const callback = "career_test_fail_clear_once"
	require.NoError(t, o.db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if failClear && tx.Statement.Table == "career_source_revisions" {
			failClear = false
			tx.AddError(errors.New("injected source clear failure"))
		}
	}))
	t.Cleanup(func() { _ = o.db.Callback().Update().Remove(callback) })
	h := &Handler{office: o, upload: NewUploadAdapter(files, catalog, nil)}
	require.NoError(t, h.reconcileStaleSources(ctx))
	require.False(t, failClear, "source clear failure must be exercised")
	_, err = os.Stat(physical.path)
	require.ErrorIs(t, err, os.ErrNotExist)
	var resource types.StoredResource
	require.NoError(t, o.db.Unscoped().Where("handle=?", strings.TrimPrefix(ref, types.ResourceScheme)).First(&resource).Error)
	require.Equal(t, types.ResourceStateDeleted, resource.State)
	stillPending, err := o.GetSource(ctx, source.ID)
	require.NoError(t, err)
	require.Equal(t, ref, stillPending.ResourceRef)
	require.Equal(t, "cleanup_pending_parse_failed", stillPending.ErrorCategory)

	require.NoError(t, h.reconcileStaleSources(ctx))
	cleaned, err := o.GetSource(ctx, source.ID)
	require.NoError(t, err)
	require.Equal(t, "failed", cleaned.Status)
	require.Equal(t, "parse_failed", cleaned.ErrorCategory)
	require.Empty(t, cleaned.ResourceRef)
}
