package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSyncLogLifecycleUpdateHeartbeat verifies UpdateHeartbeat persists the
// heartbeat timestamp and refreshes updated_at.
func TestSyncLogLifecycleUpdateHeartbeat(t *testing.T) {
	db := setupDataSourceRepoTestDB(t)
	repo := NewSyncLogRepository(db)
	ctx := context.Background()

	log := &types.SyncLog{
		ID:           "log-heartbeat",
		DataSourceID: "ds-1",
		TenantID:     1,
		Status:       types.SyncLogStatusRunning,
	}
	require.NoError(t, repo.Create(ctx, log))

	// Push updated_at into the past so the heartbeat refresh is observable.
	staleUpdatedAt := time.Now().UTC().Add(-time.Hour)
	require.NoError(t, db.Exec("UPDATE sync_logs SET updated_at = ? WHERE id = ?", staleUpdatedAt, log.ID).Error)

	heartbeatAt := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, repo.UpdateHeartbeat(ctx, log.ID, heartbeatAt))

	var stored types.SyncLog
	require.NoError(t, db.First(&stored, "id = ?", log.ID).Error)
	require.NotNil(t, stored.HeartbeatAt, "heartbeat_at must be persisted")
	assert.WithinDuration(t, heartbeatAt, *stored.HeartbeatAt, time.Second)
	assert.True(t, stored.UpdatedAt.After(staleUpdatedAt), "updated_at must be refreshed by a heartbeat")
}

// TestSyncLogLifecycleUpdateAsynqTaskID verifies UpdateAsynqTaskID persists
// the task id and a second call overwrites the first.
func TestSyncLogLifecycleUpdateAsynqTaskID(t *testing.T) {
	db := setupDataSourceRepoTestDB(t)
	repo := NewSyncLogRepository(db)
	ctx := context.Background()

	log := &types.SyncLog{
		ID:           "log-task-id",
		DataSourceID: "ds-1",
		TenantID:     1,
		Status:       types.SyncLogStatusRunning,
	}
	require.NoError(t, repo.Create(ctx, log))

	require.NoError(t, repo.UpdateAsynqTaskID(ctx, log.ID, "task-first"))
	require.NoError(t, repo.UpdateAsynqTaskID(ctx, log.ID, "task-second"))

	var stored types.SyncLog
	require.NoError(t, db.First(&stored, "id = ?", log.ID).Error)
	assert.Equal(t, "task-second", stored.AsynqTaskID)
}

// TestSyncLogLifecycleRequestCancel verifies RequestCancel flags only running
// rows; terminal and pending rows are a no-op that still returns nil.
func TestSyncLogLifecycleRequestCancel(t *testing.T) {
	db := setupDataSourceRepoTestDB(t)
	repo := NewSyncLogRepository(db)
	ctx := context.Background()

	running := &types.SyncLog{
		ID:           "log-cancel-running",
		DataSourceID: "ds-1",
		TenantID:     1,
		Status:       types.SyncLogStatusRunning,
	}
	success := &types.SyncLog{
		ID:           "log-cancel-success",
		DataSourceID: "ds-1",
		TenantID:     1,
		Status:       types.SyncLogStatusSuccess,
	}
	pending := &types.SyncLog{
		ID:           "log-cancel-pending",
		DataSourceID: "ds-1",
		TenantID:     1,
		Status:       types.SyncLogStatusPending,
	}
	for _, log := range []*types.SyncLog{running, success, pending} {
		require.NoError(t, repo.Create(ctx, log))
	}

	require.NoError(t, repo.RequestCancel(ctx, running.ID))
	require.NoError(t, repo.RequestCancel(ctx, success.ID))
	require.NoError(t, repo.RequestCancel(ctx, pending.ID))

	var storedRunning, storedSuccess, storedPending types.SyncLog
	require.NoError(t, db.First(&storedRunning, "id = ?", running.ID).Error)
	require.NoError(t, db.First(&storedSuccess, "id = ?", success.ID).Error)
	require.NoError(t, db.First(&storedPending, "id = ?", pending.ID).Error)
	assert.True(t, storedRunning.CancelRequested, "running row must accept cancel_requested")
	assert.False(t, storedSuccess.CancelRequested, "terminal row must not be flagged")
	assert.False(t, storedPending.CancelRequested, "pending row must not be flagged")
}
