package repository

import (
	"context"
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
	require.NoError(t, db.Exec(`INSERT INTO mobile_devices (tenant_id, owner_id, device_id, environment) VALUES (1,'u1','d','dev')`).Error)
	s := NewNotificationStore(db)
	in := NotificationIntent{TenantID: 1, EventID: "e", OwnerID: "u1", DeviceID: "d", Environment: "dev", Kind: "completed", RunID: "r1", ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, s.Enqueue(ctx, in))
	require.NoError(t, s.Enqueue(ctx, in))
	rows, err := s.Claim(ctx, "worker", 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(1), rows[0].Attempt)
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
	require.NoError(t, db.Exec(`INSERT INTO mobile_devices (tenant_id, owner_id, device_id, environment) VALUES (1,'u1','d','dev')`).Error)
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
	require.NoError(t, db.Exec(`INSERT INTO mobile_devices (tenant_id, owner_id, device_id, environment) VALUES
		(1,'u1','active','dev'), (1,'u1','revoked','dev'), (1,'u2','other','dev')`).Error)
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
	require.NoError(t, db.Exec(`INSERT INTO mobile_devices (tenant_id, owner_id, device_id, environment) VALUES (1,'u1','d','dev')`).Error)
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
