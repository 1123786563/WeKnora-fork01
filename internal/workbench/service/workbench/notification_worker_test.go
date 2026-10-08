package workbench

import (
	"context"
	"fmt"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestNotificationWorkerRunOnceReaches257thRun exercises the production
// discovery/cursor loop rather than reproducing it in the repository test.
// A run after the first 256 must be projected in the same pass.
func TestNotificationWorkerRunOnceReaches257thRun(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	ctx := context.Background()
	for i := 1; i <= 257; i++ {
		runID := fmt.Sprintf("worker-page-%03d", i)
		require.NoError(t, db.Exec(`INSERT INTO agent_runs
			(tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, deadline)
			VALUES (?, ?, 's1', 'u1', ?, ?, 'hash', '{}', ?)`,
			1, runID, "request-"+runID, "assistant-"+runID, time.Now().Add(time.Hour)).Error)
		require.NoError(t, db.Exec(`INSERT INTO agent_run_events
			(tenant_id, run_id, seq, event_type, payload) VALUES (?, ?, 1, 'run_completed', '{}')`, 1, runID).Error)
	}
	require.NoError(t, db.Exec(`INSERT INTO mobile_devices
		(tenant_id, owner_id, device_id, environment, platform, token_ciphertext, token_hash)
		VALUES (1, 'u1', 'worker-page-device', 'dev', 'ios', 'cipher', 'hash-worker-page')`).Error)

	runs := repository.NewAgentRunStore(db)
	store := repository.NewNotificationStore(db)
	worker := NewNotificationWorker(NewNotificationProjector(runs, store), store)
	require.NoError(t, worker.RunOnce(ctx))
	var intents int64
	require.NoError(t, db.Table("mobile_notification_intents").Count(&intents).Error)
	require.EqualValues(t, 257, intents)
	cursor, err := store.LoadCheckpoint(ctx, notificationConsumerName,
		agentruntime.RunKey{TenantID: 1, RunID: "worker-page-257"})
	require.NoError(t, err)
	require.EqualValues(t, 1, cursor)
}

// TestNotificationWorkerReplaysAfterCommitBoundaryFailure forces the first
// projection transaction to abort, closes the database, then reconstructs a
// fresh worker. This proves replay is driven by durable events/cursors rather
// than process memory.
func TestNotificationWorkerReplaysAfterCommitBoundaryFailure(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	ctx := context.Background()
	require.NoError(t, db.Exec(`INSERT INTO agent_runs
		(tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, deadline)
		VALUES (1, 'worker-replay', 's1', 'u1', 'replay-request', 'replay-assistant', 'hash', '{}', ?)`, time.Now().Add(time.Hour)).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_run_events
		(tenant_id, run_id, seq, event_type, payload) VALUES (1, 'worker-replay', 1, 'run_completed', '{}')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO mobile_devices
		(tenant_id, owner_id, device_id, environment, platform, token_ciphertext, token_hash)
		VALUES (1, 'u1', 'worker-replay-device', 'dev', 'ios', 'cipher', 'hash-worker-replay')`).Error)
	// Fail at the intent insert, inside ProjectEventsAndCheckpoint's
	// transaction. The event and cursor must remain untouched.
	require.NoError(t, db.Exec(`CREATE TRIGGER fail_notification_projection
		BEFORE INSERT ON mobile_notification_intents
		BEGIN SELECT RAISE(ABORT, 'injected notification commit failure'); END`).Error)
	runs := repository.NewAgentRunStore(db)
	store := repository.NewNotificationStore(db)
	worker := NewNotificationWorker(NewNotificationProjector(runs, store), store)
	require.Error(t, worker.RunOnce(ctx))
	var intents int64
	require.NoError(t, db.Table("mobile_notification_intents").Count(&intents).Error)
	require.Zero(t, intents)
	cursor, err := store.LoadCheckpoint(ctx, notificationConsumerName,
		agentruntime.RunKey{TenantID: 1, RunID: "worker-replay"})
	require.NoError(t, err)
	require.Zero(t, cursor)

	dsn := db.Dialector.(*sqlite.Dialector).DSN
	conn, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	// Reopen after the simulated process crash and remove only the injected
	// fault; durable run/event rows are read by the new worker instance.
	db2, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		c, _ := db2.DB()
		if c != nil {
			_ = c.Close()
		}
	})
	require.NoError(t, db2.Exec(`DROP TRIGGER fail_notification_projection`).Error)
	runs2 := repository.NewAgentRunStore(db2)
	store2 := repository.NewNotificationStore(db2)
	worker2 := NewNotificationWorker(NewNotificationProjector(runs2, store2), store2)
	require.NoError(t, worker2.RunOnce(ctx))
	require.NoError(t, db2.Table("mobile_notification_intents").Count(&intents).Error)
	require.EqualValues(t, 1, intents)
	cursor, err = store2.LoadCheckpoint(ctx, notificationConsumerName,
		agentruntime.RunKey{TenantID: 1, RunID: "worker-replay"})
	require.NoError(t, err)
	require.EqualValues(t, 1, cursor)
	// A second fresh pass is idempotent.
	require.NoError(t, worker2.RunOnce(ctx))
	require.NoError(t, db2.Table("mobile_notification_intents").Count(&intents).Error)
	require.EqualValues(t, 1, intents)
}
