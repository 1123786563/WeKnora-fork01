package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/stretchr/testify/require"
)

func TestCraftRunAdmissionStoresOwnerAndAuthenticatedActorSeparately(t *testing.T) {
	db := openRunTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES (?, ?, ?, ?, ?)`,
		"collaborator", "collaborator", "collaborator@example.test", "x", 1).Error)
	require.NoError(t, db.Exec(`INSERT INTO craft_sessions (session_id, tenant_id, kind) VALUES ('s1', 1, 'web')`).Error)

	in := testAdmission()
	in.Key.RunID = "craft-actor-run"
	in.ActorUserID = "collaborator"
	store := NewAgentRunStore(db)
	run, err := store.Admit(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, "u1", run.UserID, "Run.UserID remains the storage owner")
	require.Equal(t, "collaborator", run.ActorUserID, "actor identity is durable and distinct")

	var row struct{ OwnerID, ActorUserID string }
	require.NoError(t, db.Table("agent_runs").Select("owner_id, actor_user_id").Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).Take(&row).Error)
	require.Equal(t, "u1", row.OwnerID)
	require.Equal(t, "collaborator", row.ActorUserID)
	loaded, err := store.Get(context.Background(), in.Key)
	require.NoError(t, err)
	require.Equal(t, "u1", loaded.UserID)
	require.Equal(t, "collaborator", loaded.ActorUserID, "worker recovery can restore the persisted actor")

	replay, err := store.Admit(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, run.Key, replay.Key)
	require.Equal(t, run.ActorUserID, replay.ActorUserID)
}

func TestCraftRunAdmissionRejectsCrossActorReplayAndCrossTenantActor(t *testing.T) {
	db := openRunTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (2, 'tenant-2', 'test')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES
		('collaborator', 'collaborator', 'collaborator@example.test', 'x', 1),
		('other-tenant-user', 'other', 'other@example.test', 'x', 2)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO craft_sessions (session_id, tenant_id, kind) VALUES ('s1', 1, 'web')`).Error)

	in := testAdmission()
	in.Key.RunID = "craft-actor-replay"
	in.ActorUserID = "collaborator"
	store := NewAgentRunStore(db)
	_, err := store.Admit(context.Background(), in)
	require.NoError(t, err)

	otherActor := in
	otherActor.ActorUserID = "u1"
	otherActor.Key.RunID = "craft-actor-replay-owner"
	_, err = store.Admit(context.Background(), otherActor)
	require.ErrorIs(t, err, agentruntime.ErrConflict, "same owner/request/body cannot replay another actor's Run")

	spoof := in
	spoof.ActorUserID = "other-tenant-user"
	spoof.RequestID = "cross-tenant-actor"
	spoof.RequestHash = "cross-tenant-actor-hash"
	spoof.Key.RunID = "cross-tenant-actor-run"
	_, err = store.Admit(context.Background(), spoof)
	require.ErrorIs(t, err, agentruntime.ErrConflict, "actor must belong to the admission tenant")
}

func TestCraftRunAdmissionFailsClosedWithoutActorAndForLegacyReplay(t *testing.T) {
	db := openRunTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO craft_sessions (session_id, tenant_id, kind) VALUES ('s1', 1, 'web')`).Error)
	store := NewAgentRunStore(db)
	in := testAdmission()
	in.Key.RunID = "craft-actor-missing"
	_, err := store.Admit(context.Background(), in)
	require.ErrorIs(t, err, agentruntime.ErrConflict, "new Craft rows cannot infer actor from storage owner")

	require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES (?, ?, ?, ?, ?)`,
		"collaborator", "collaborator", "collaborator@example.test", "x", 1).Error)
	in.ActorUserID = "collaborator"
	in.Key.RunID = "craft-actor-legacy"
	run, err := store.Admit(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, "collaborator", run.ActorUserID)
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).
		Update("actor_user_id", nil).Error)
	legacy, err := store.Get(context.Background(), in.Key)
	require.NoError(t, err)
	require.Empty(t, legacy.ActorUserID, "legacy actor absence remains explicit for worker fail-closed handling")

	_, err = store.Admit(context.Background(), in)
	require.ErrorIs(t, err, agentruntime.ErrConflict, "legacy Craft rows with unknown actor cannot be replayed as the owner or guessed actor")
}

func TestGenericRunAdmissionDefaultsActorToOwnerForCompatibility(t *testing.T) {
	db := openRunTestDB(t)
	in := testAdmission()
	in.ActorUserID = ""
	in.Deadline = time.Now().Add(time.Hour)
	run, err := NewAgentRunStore(db).Admit(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, in.UserID, run.UserID)
	require.Equal(t, in.UserID, run.ActorUserID)
	require.True(t, json.Valid(run.Snapshot))
}
