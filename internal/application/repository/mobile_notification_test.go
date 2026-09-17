package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/stretchr/testify/require"
)

func TestNotificationOutboxDeduplicates(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	_, err := NewAgentRunStore(db).Admit(ctx, testAdmission())
	require.NoError(t, err)
	// The W13 device migration is an accepted dependency. Keep this focused
	// test runnable on a pre-integration branch by providing its minimal table.
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS mobile_devices (
		tenant_id INTEGER NOT NULL, owner_id VARCHAR(512) NOT NULL, device_id VARCHAR(128) NOT NULL,
		environment VARCHAR(32) NOT NULL, token_ciphertext TEXT NOT NULL DEFAULT '', token_hash VARCHAR(64) NOT NULL DEFAULT '',
		revoked_at DATETIME, PRIMARY KEY (tenant_id, owner_id, device_id, environment)
	)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO mobile_devices (tenant_id, owner_id, device_id, environment, platform, token_ciphertext, token_hash) VALUES (1,'u1','d','dev','ios','cipher','hash-d')`).Error)
	s := NewNotificationStore(db)
	in := NotificationIntent{TenantID: 1, EventID: "e", OwnerID: "u1", DeviceID: "d", Environment: "dev", Kind: "completed", RunID: "r1", ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, s.Enqueue(ctx, in))
	require.NoError(t, s.Enqueue(ctx, in))
	rows, err := s.Claim(ctx, "worker", 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(1), rows[0].Attempt)
}

func TestNotificationProviderStatePauseAlertAndRecoverySurvivesStoreRestart(t *testing.T) {
	db := openRunTestDB(t)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS mobile_notification_provider_state (
		provider_key TEXT PRIMARY KEY, paused INTEGER NOT NULL DEFAULT 0,
		reason TEXT NOT NULL DEFAULT '', alert_count INTEGER NOT NULL DEFAULT 0,
		paused_at DATETIME, recovered_at DATETIME, updated_at DATETIME NOT NULL)`).Error)
	ctx := context.Background()
	stateStore := NewNotificationProviderStateStore(db)
	require.NoError(t, stateStore.Pause(ctx, "mobile", "invalid endpoint"))
	require.NoError(t, stateStore.Pause(ctx, "mobile", "invalid endpoint"))
	state, err := stateStore.State(ctx, "mobile")
	require.NoError(t, err)
	require.True(t, state.Paused)
	require.EqualValues(t, 1, state.AlertCount)
	// Reconstructing the repository against the same database models a process
	// restart: the pause and alert evidence remain durable.
	restarted := NewNotificationProviderStateStore(db)
	paused, err := restarted.IsPaused(ctx, "mobile")
	require.NoError(t, err)
	require.True(t, paused)
	require.NoError(t, restarted.Recover(ctx, "mobile"))
	state, err = restarted.State(ctx, "mobile")
	require.NoError(t, err)
	require.False(t, state.Paused)
	require.NotNil(t, state.RecoveredAt)
}

func TestNotificationOutboxFenceRejectsStaleAck(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	_, err := NewAgentRunStore(db).Admit(ctx, testAdmission())
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS mobile_devices (
		tenant_id INTEGER NOT NULL, owner_id VARCHAR(512) NOT NULL, device_id VARCHAR(128) NOT NULL,
		environment VARCHAR(32) NOT NULL, token_ciphertext TEXT NOT NULL DEFAULT '', token_hash VARCHAR(64) NOT NULL DEFAULT '',
		revoked_at DATETIME, PRIMARY KEY (tenant_id, owner_id, device_id, environment)
	)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO mobile_devices (tenant_id, owner_id, device_id, environment, platform, token_ciphertext, token_hash) VALUES (1,'u1','d','dev','ios','cipher','hash-d')`).Error)
	s := NewNotificationStore(db)
	require.NoError(t, s.Enqueue(ctx, NotificationIntent{TenantID: 1, EventID: "e", OwnerID: "u1", DeviceID: "d", Environment: "dev", Kind: "completed", RunID: "r1", ExpiresAt: time.Now().Add(time.Hour)}))
	rows, err := s.Claim(ctx, "worker-a", 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	first := rows[0]
	require.False(t, s.Ack(ctx, first.ID, "worker-b", first.Fence))
	require.True(t, s.Retry(ctx, first.ID, "worker-a", first.Fence))
	require.False(t, s.Ack(ctx, first.ID, "worker-a", first.Fence))
	second, err := s.Claim(ctx, "worker-b", 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Greater(t, second[0].Fence, first.Fence)
	require.False(t, s.Ack(ctx, first.ID, "worker-a", first.Fence))
	require.True(t, s.Ack(ctx, second[0].ID, "worker-b", second[0].Fence))
}

func TestNotificationProjectEventScopesAndFiltersDevices(t *testing.T) {
	db := openRunTestDB(t)
	_, err := NewAgentRunStore(db).Admit(context.Background(), testAdmission())
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS mobile_devices (
		tenant_id INTEGER NOT NULL, owner_id VARCHAR(512) NOT NULL, device_id VARCHAR(128) NOT NULL,
		environment VARCHAR(32) NOT NULL, token_ciphertext TEXT NOT NULL DEFAULT '', token_hash VARCHAR(64) NOT NULL DEFAULT '',
		revoked_at DATETIME, PRIMARY KEY (tenant_id, owner_id, device_id, environment)
	)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_run_events (tenant_id, run_id, seq, attempt_id, event_type, payload) VALUES
		(1,'r1',3,'a','token','{}'), (1,'r1',4,'a','run_completed','{}')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO mobile_devices (tenant_id, owner_id, device_id, environment, platform, token_ciphertext, token_hash) VALUES
		(1,'u1','active','dev','ios','cipher','hash-active'), (1,'u1','revoked','dev','ios','cipher','hash-revoked'), (1,'u2','other','dev','ios','cipher','hash-other')`).Error)
	require.NoError(t, db.Exec(`UPDATE mobile_devices SET revoked_at = CURRENT_TIMESTAMP WHERE owner_id = 'u1' AND device_id = 'revoked'`).Error)
	s := NewNotificationStore(db)
	require.NoError(t, s.ProjectEvent(context.Background(), RunNotificationEvent{TenantID: 1, OwnerID: "u1", RunID: "r1", Seq: 3, Type: "token"}))
	var count int64
	require.NoError(t, db.Table("mobile_notification_intents").Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, s.ProjectEvent(context.Background(), RunNotificationEvent{TenantID: 1, OwnerID: "u1", RunID: "r1", Seq: 4, Type: "run_completed"}))
	require.NoError(t, db.Table("mobile_notification_intents").Where("owner_id = ?", "u1").Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestNotificationProjectEventDerivesDurableIdentity(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	_, err := NewAgentRunStore(db).Admit(ctx, testAdmission())
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS mobile_devices (tenant_id INTEGER NOT NULL, owner_id TEXT NOT NULL, device_id TEXT NOT NULL, environment TEXT NOT NULL, token_ciphertext TEXT NOT NULL DEFAULT '', token_hash TEXT NOT NULL DEFAULT '', revoked_at DATETIME, PRIMARY KEY (tenant_id, owner_id, device_id, environment))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS mobile_notification_intents (id TEXT PRIMARY KEY, tenant_id INTEGER NOT NULL, event_id TEXT NOT NULL, owner_id TEXT NOT NULL, device_id TEXT NOT NULL, environment TEXT NOT NULL, kind TEXT NOT NULL, run_id TEXT NOT NULL, expires_at DATETIME NOT NULL, state TEXT NOT NULL, attempt INTEGER NOT NULL, lease_owner TEXT NOT NULL, lease_until DATETIME, fence INTEGER NOT NULL, created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, UNIQUE (tenant_id,event_id,owner_id,device_id,environment))`).Error)
	require.NoError(t, db.Exec(`INSERT INTO mobile_devices (tenant_id, owner_id, device_id, environment, platform, token_ciphertext, token_hash) VALUES (1,'u1','d','dev','ios','cipher','hash-d')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_run_events (tenant_id, run_id, seq, attempt_id, event_type, payload) VALUES (1,'r1',9,'a','run_completed','{}')`).Error)
	s := NewNotificationStore(db)
	require.Error(t, s.ProjectEvent(ctx, RunNotificationEvent{TenantID: 1, OwnerID: "attacker", RunID: "r1", Seq: 9, Type: "run_completed"}))
	require.Error(t, s.ProjectEvent(ctx, RunNotificationEvent{TenantID: 1, OwnerID: "u1", RunID: "r1", Seq: 10, Type: "run_completed"}))
	require.Error(t, s.ProjectEvent(ctx, RunNotificationEvent{TenantID: 1, OwnerID: "u1", RunID: "r1", Seq: 9, Type: "run_failed"}))
	require.NoError(t, s.ProjectEvent(ctx, RunNotificationEvent{TenantID: 1, OwnerID: "u1", RunID: "r1", Seq: 9, Type: "run_completed"}))
	var count int64
	require.NoError(t, db.Table("mobile_notification_intents").Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestNotificationCheckpointIsMonotonic(t *testing.T) {
	db := openRunTestDB(t)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS mobile_notification_checkpoints (consumer TEXT NOT NULL, tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, cursor INTEGER NOT NULL DEFAULT 0, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (consumer,tenant_id,run_id))`).Error)
	s := NewNotificationStore(db)
	key := agentruntime.RunKey{TenantID: 1, RunID: "r1"}
	require.NoError(t, s.SaveCheckpoint(context.Background(), "c", key, 4))
	require.NoError(t, s.SaveCheckpoint(context.Background(), "c", key, 2))
	got, err := s.LoadCheckpoint(context.Background(), "c", key)
	require.NoError(t, err)
	require.EqualValues(t, 4, got)
}

func TestEventRunKeysPageContinuesPastFirstPage(t *testing.T) {
	db := openRunTestDB(t)
	for i := 1; i <= 257; i++ {
		runID := fmt.Sprintf("run-%03d", i)
		// agent_run_events enforces a foreign key onto agent_runs; admit the
		// minimal parent row so the paging fixture satisfies the durable schema.
		require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, deadline)
			VALUES (?, ?, 's1', 'u1', ?, 'a', 'h', '{}', datetime('now', '+10 minutes'))`, 1, runID, runID).Error)
		require.NoError(t, db.Exec(`INSERT INTO agent_run_events (tenant_id, run_id, seq, attempt_id, event_type, payload) VALUES (?, ?, 1, 'a', 'run_completed', '{}')`, 1, runID).Error)
	}
	s := NewNotificationStore(db)
	first, err := s.EventRunKeysPage(context.Background(), 256, agentruntime.RunKey{})
	require.NoError(t, err)
	require.Len(t, first, 256)
	second, err := s.EventRunKeysPage(context.Background(), 256, first[len(first)-1])
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Equal(t, "run-257", second[0].RunID)
}

// TestNotificationProjectionUsesMigratedSchemaAndDoesNotStarveRuns proves the
// complete durable page protocol. It uses the same migration helper as the
// agent-run repository, seeds real agent_runs/mobile_devices rows, projects
// every page, and checks both the 257th intent and its durable cursor.
func TestNotificationProjectionUsesMigratedSchemaAndDoesNotStarveRuns(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	store := NewNotificationStore(db)
	for i := 1; i <= 257; i++ {
		runID := fmt.Sprintf("page-run-%03d", i)
		require.NoError(t, db.Exec(`INSERT INTO agent_runs
			(tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, deadline)
			VALUES (?, ?, 's1', 'u1', ?, ?, 'hash', '{}', ?)`,
			1, runID, "request-"+runID, "assistant-"+runID, time.Now().Add(time.Hour)).Error)
		require.NoError(t, db.Exec(`INSERT INTO agent_run_events
			(tenant_id, run_id, seq, event_type, payload) VALUES (?, ?, 1, 'run_completed', '{}')`, 1, runID).Error)
	}
	require.NoError(t, db.Exec(`INSERT INTO mobile_devices
		(tenant_id, owner_id, device_id, environment, platform, token_ciphertext, token_hash)
		VALUES (1, 'u1', 'page-device', 'dev', 'ios', 'cipher', 'hash-page-device')`).Error)

	var after agentruntime.RunKey
	for {
		keys, err := store.EventRunKeysPage(ctx, 256, after)
		require.NoError(t, err)
		if len(keys) == 0 {
			break
		}
		for _, key := range keys {
			events, err := NewAgentRunStore(db).ReadEvents(ctx, key, 0, 256)
			require.NoError(t, err)
			refs := make([]RunNotificationEvent, 0, len(events))
			for _, evt := range events {
				refs = append(refs, RunNotificationEvent{TenantID: key.TenantID, OwnerID: "u1", RunID: key.RunID, Seq: evt.Seq, Type: evt.Type})
			}
			require.NoError(t, store.ProjectEventsAndCheckpoint(ctx, "mobile-notification-projector", key, refs, int64(len(events))))
			after = key
		}
		if len(keys) < 256 {
			break
		}
	}
	var intents int64
	require.NoError(t, db.Table("mobile_notification_intents").Count(&intents).Error)
	require.EqualValues(t, 257, intents)
	last := agentruntime.RunKey{TenantID: 1, RunID: "page-run-257"}
	cursor, err := store.LoadCheckpoint(ctx, "mobile-notification-projector", last)
	require.NoError(t, err)
	require.EqualValues(t, 1, cursor)
}

func TestNotificationProjectionRollbackAndRestartReplayUsesMigratedSchema(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	runs := NewAgentRunStore(db)
	store := NewNotificationStore(db)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs
		(tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, deadline)
		VALUES (1, 'rollback-run', 's1', 'u1', 'rollback-request', 'rollback-assistant', 'hash', '{}', ?)`, time.Now().Add(time.Hour)).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_run_events
		(tenant_id, run_id, seq, event_type, payload) VALUES (1, 'rollback-run', 1, 'run_completed', '{}')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO mobile_devices
		(tenant_id, owner_id, device_id, environment, platform, token_ciphertext, token_hash)
		VALUES (1, 'u1', 'rollback-device', 'dev', 'ios', 'cipher', 'hash-rollback-device')`).Error)
	key := agentruntime.RunKey{TenantID: 1, RunID: "rollback-run"}
	failed := []RunNotificationEvent{
		{TenantID: 1, OwnerID: "u1", RunID: "rollback-run", Seq: 1, Type: "run_completed"},
		{TenantID: 1, OwnerID: "wrong-owner", RunID: "rollback-run", Seq: 1, Type: "run_completed"},
	}
	require.Error(t, store.ProjectEventsAndCheckpoint(ctx, "mobile-notification-projector", key, failed, 1))
	var intents int64
	require.NoError(t, db.Table("mobile_notification_intents").Count(&intents).Error)
	require.Zero(t, intents, "projection and checkpoint must roll back together")
	cursor, err := store.LoadCheckpoint(ctx, "mobile-notification-projector", key)
	require.NoError(t, err)
	require.Zero(t, cursor)

	events, err := runs.ReadEvents(ctx, key, 0, 256)
	require.NoError(t, err)
	require.NoError(t, store.ProjectEventsAndCheckpoint(ctx, "mobile-notification-projector", key,
		[]RunNotificationEvent{{TenantID: 1, OwnerID: "u1", RunID: key.RunID, Seq: events[0].Seq, Type: events[0].Type}}, 1))
	require.NoError(t, store.ProjectEventsAndCheckpoint(ctx, "mobile-notification-projector", key,
		[]RunNotificationEvent{{TenantID: 1, OwnerID: "u1", RunID: key.RunID, Seq: events[0].Seq, Type: events[0].Type}}, 1))
	require.NoError(t, db.Table("mobile_notification_intents").Count(&intents).Error)
	require.EqualValues(t, 1, intents)
	cursor, err = store.LoadCheckpoint(ctx, "mobile-notification-projector", key)
	require.NoError(t, err)
	require.EqualValues(t, 1, cursor)
}

func TestBudgetExhaustionEventProjectsDurableIntent(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	store := NewNotificationStore(db)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs
		(tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, deadline)
		VALUES (1, 'budget-run', 's1', 'u1', 'budget-request', 'budget-assistant', 'hash-budget', '{}', ?)`, time.Now().Add(time.Hour)).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_run_events
		(tenant_id, run_id, seq, event_type, payload) VALUES (1, 'budget-run', 1, 'budget_exhausted', '{"reason":"budget_exhausted"}')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO mobile_devices
		(tenant_id, owner_id, device_id, environment, platform, token_ciphertext, token_hash)
		VALUES (1, 'u1', 'budget-device', 'dev', 'android', 'cipher', 'hash-budget-device')`).Error)
	require.NoError(t, store.ProjectEventsAndCheckpoint(ctx, "mobile-notification-projector",
		agentruntime.RunKey{TenantID: 1, RunID: "budget-run"},
		[]RunNotificationEvent{{TenantID: 1, OwnerID: "u1", RunID: "budget-run", Seq: 1, Type: "budget_exhausted"}}, 1))
	var kind string
	require.NoError(t, db.Table("mobile_notification_intents").Select("kind").Where("run_id = ?", "budget-run").Scan(&kind).Error)
	require.Equal(t, "budget_exhausted", kind)
}
