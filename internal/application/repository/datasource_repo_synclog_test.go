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

// TestSyncLogLifecycleHasRunningSyncExcludesStalledRuns is the bug-2
// regression: a "running" row whose latest liveness signal —
// COALESCE(heartbeat_at, started_at) — is older than types.SyncStallWindow
// belongs to a dead run and must not count as running. Before the fix such a
// row blocked the scheduler for that data source forever.
func TestSyncLogLifecycleHasRunningSyncExcludesStalledRuns(t *testing.T) {
	db := setupDataSourceRepoTestDB(t)
	repo := NewSyncLogRepository(db)
	ctx := context.Background()
	// Local time on purpose: the sqlite driver persists timestrings with
	// their zone offset, so seeded values must share the zone of the
	// time.Now() cutoff inside HasRunningSync for SQL comparisons to order
	// correctly (same convention as the container reset tests).
	now := time.Now()

	stalled := &types.SyncLog{
		ID: "log-stalled", DataSourceID: "ds-stalled", TenantID: 1,
		Status: types.SyncLogStatusRunning,
	}
	live := &types.SyncLog{
		ID: "log-live", DataSourceID: "ds-live", TenantID: 1,
		Status: types.SyncLogStatusRunning,
	}
	justStarted := &types.SyncLog{
		ID: "log-just-started", DataSourceID: "ds-just-started", TenantID: 1,
		Status: types.SyncLogStatusRunning,
	}
	for _, row := range []*types.SyncLog{stalled, live, justStarted} {
		require.NoError(t, repo.Create(ctx, row))
	}
	// Heartbeat three hours ago: beyond the 2h15m stall window.
	require.NoError(t, db.Exec(
		`UPDATE sync_logs SET started_at = ?, heartbeat_at = ? WHERE id = ?`,
		now.Add(-4*time.Hour), now.Add(-3*time.Hour), stalled.ID).Error)
	// Heartbeating long task: started past the task timeout, still alive.
	require.NoError(t, db.Exec(
		`UPDATE sync_logs SET started_at = ?, heartbeat_at = ? WHERE id = ?`,
		now.Add(-40*time.Minute), now.Add(-1*time.Minute), live.ID).Error)
	// No heartbeat yet: liveness falls back to started_at.
	require.NoError(t, db.Exec(
		`UPDATE sync_logs SET started_at = ? WHERE id = ?`,
		now.Add(-1*time.Minute), justStarted.ID).Error)

	runningStalled, err := repo.HasRunningSync(ctx, stalled.DataSourceID)
	require.NoError(t, err)
	assert.False(t, runningStalled,
		"a running row with no liveness inside the stall window must not block scheduling")

	runningLive, err := repo.HasRunningSync(ctx, live.DataSourceID)
	require.NoError(t, err)
	assert.True(t, runningLive, "a heartbeating run still counts as running")

	runningJustStarted, err := repo.HasRunningSync(ctx, justStarted.DataSourceID)
	require.NoError(t, err)
	assert.True(t, runningJustStarted,
		"a run without any heartbeat yet falls back to started_at")
}
