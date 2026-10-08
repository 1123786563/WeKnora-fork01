package adapters

// GORM export-job adapter tests (Wave 1, Task 7, brief Step 2): the
// ports.ExportJobStore contract over the migrated schema. CRUD defaults, the
// tenant-scoped Get whose miss and cross-tenant probe answer the SAME legacy
// 404 AppError (so a job id cannot be probed across workspaces), lifecycle
// transitions that always rewrite file_path and error_message, and the
// tenant-scoped update that keeps a foreign workspace's transition from
// landing.

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/stretchr/testify/require"
)

func TestGormExportJobStoreCRUD(t *testing.T) {
	db := openAuditTestDB(t)
	store := NewGormExportJobStore(db)
	ctx := context.Background()

	job := &domain.ExportJob{TenantID: 1, RequestedBy: "u1"}
	require.NoError(t, store.Create(ctx, job), "Create must default the status to pending")
	require.NotZero(t, job.ID)
	require.Equal(t, domain.ExportPending, job.Status)

	got, err := store.Get(ctx, 1, job.ID)
	require.NoError(t, err)
	require.Equal(t, domain.ExportPending, got.Status)
	require.Equal(t, "u1", got.RequestedBy)

	// Cross-tenant probes and misses answer the SAME sentinel — the existing
	// 404 AppError the legacy repository answers — so the caller maps both to
	// 404 without learning whether the id exists.
	_, foreignErr := store.Get(ctx, 2, job.ID)
	_, missingErr := store.Get(ctx, 1, 999999)
	require.ErrorIs(t, foreignErr, repository.ErrQueryHistoryExportJobNotFound)
	require.ErrorIs(t, missingErr, repository.ErrQueryHistoryExportJobNotFound)
	require.Equal(t, missingErr.Error(), foreignErr.Error(),
		"the cross-tenant miss must be indistinguishable from the plain miss")
	appErr, ok := apperrors.IsAppError(foreignErr)
	require.True(t, ok, "the sentinel is the existing AppError")
	require.Equal(t, 404, appErr.HTTPCode)

	// Transition to failed with a message.
	require.NoError(t, store.UpdateStatus(ctx, 1, job.ID, domain.ExportFailed, "", "boom"))
	got, err = store.Get(ctx, 1, job.ID)
	require.NoError(t, err)
	require.Equal(t, domain.ExportFailed, got.Status)
	require.Equal(t, "boom", got.ErrorMessage)

	// A later done transition rewrites BOTH columns: no stale error message
	// survives a successful retry.
	require.NoError(t, store.UpdateStatus(ctx, 1, job.ID, domain.ExportDone, "local://exports/1.csv", ""))
	got, err = store.Get(ctx, 1, job.ID)
	require.NoError(t, err)
	require.Equal(t, domain.ExportDone, got.Status)
	require.Equal(t, "local://exports/1.csv", got.FilePath)
	require.Empty(t, got.ErrorMessage)
}

func TestGormExportJobStoreUpdateStatusIsTenantScoped(t *testing.T) {
	db := openAuditTestDB(t)
	store := NewGormExportJobStore(db)
	ctx := context.Background()

	job := &domain.ExportJob{TenantID: 1, RequestedBy: "u1"}
	require.NoError(t, store.Create(ctx, job))

	// The port hardens the legacy update with the tenant scope: a foreign
	// workspace's transition must not touch the job row.
	require.NoError(t, store.UpdateStatus(ctx, 2, job.ID, domain.ExportFailed, "", "hijack"))
	got, err := store.Get(ctx, 1, job.ID)
	require.NoError(t, err)
	require.Equal(t, domain.ExportPending, got.Status)
	require.Empty(t, got.ErrorMessage, "no cross-tenant write may land")
	require.Empty(t, got.FilePath)
}
