package workbench

import (
	"context"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestGormWorkspaceLeaseStoreScopesTenantAndFencesEpoch(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:w22_workspace_leases?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&workspaceLeaseRow{}))
	store := NewGormWorkspaceLeaseStore(db)
	first, err := store.AcquireWorkspaceLease(context.Background(), 7, "workspace-1", "run-1", "worker-a", time.Minute)
	require.NoError(t, err)
	_, err = store.AcquireWorkspaceLease(context.Background(), 7, "workspace-1", "run-2", "worker-b", time.Minute)
	require.ErrorIs(t, err, ErrWorkspaceLocked)
	// The same opaque workspace name is independent in another tenant.
	_, err = store.AcquireWorkspaceLease(context.Background(), 8, "workspace-1", "run-2", "worker-b", time.Minute)
	require.NoError(t, err)
	require.ErrorIs(t, store.RenewWorkspaceLease(context.Background(), first, "worker-b", time.Minute), ErrWorkspaceLeaseLost)
	require.NoError(t, store.ReleaseWorkspaceLease(context.Background(), first, "worker-a"))
	second, err := store.AcquireWorkspaceLease(context.Background(), 7, "workspace-1", "run-2", "worker-b", time.Minute)
	require.NoError(t, err)
	require.Greater(t, second.Epoch, first.Epoch)
	require.ErrorIs(t, store.ReleaseWorkspaceLease(context.Background(), first, "worker-a"), ErrWorkspaceLeaseLost)
}
