package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
)

// craftTestAdmission mirrors the production Craft admission contract: a
// registered Craft Task admission snapshot must carry a valid Craft input
// manifest. Callers registering the session as a Craft Task must also seed
// its Workspace so the repository can freeze the server-owned seed.
func craftTestAdmission() agentruntime.Admission {
	in := testAdmission()
	snapshot, err := json.Marshal(map[string]any{"version": 1, "craft_input_manifest": []craft.Input{}})
	if err != nil {
		panic(err)
	}
	in.Snapshot = snapshot
	return in
}

func TestAdmissionUsesActiveMembershipInsteadOfHomeTenant(t *testing.T) {
	db := openRunTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (2, 'tenant-2', 'test')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES (?, ?, ?, ?, ?)`,
		"cross-home-actor", "cross-home", "cross-home@example.test", "x", 2).Error)
	require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id, user_id, role, status) VALUES
		(1, 'cross-home-actor', 'contributor', 'active'), (2, 'cross-home-actor', 'admin', 'active')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO craft_sessions (session_id, tenant_id, kind) VALUES ('s1', 1, 'web')`).Error)
	putCraftWorkspace(t, NewCraftStore(db))

	craftAdmission := craftTestAdmission()
	craftAdmission.Key.RunID = "craft-cross-home-actor"
	craftAdmission.ActorUserID = "cross-home-actor"
	store := NewAgentRunStore(db)
	run, err := store.Admit(context.Background(), craftAdmission)
	require.NoError(t, err, "an active invited member can start a Craft Run outside their home tenant")
	require.Equal(t, "u1", run.UserID)
	require.Equal(t, "cross-home-actor", run.ActorUserID)

	genericAdmission := testAdmission()
	genericAdmission.Key.RunID = "generic-cross-home-actor"
	genericAdmission.SessionID = "s2"
	genericAdmission.RequestID = "cross-home-generic-request"
	genericAdmission.RequestHash = "cross-home-generic-hash"
	genericAdmission.AssistantMessageID = "cross-home-generic-assistant"
	genericAdmission.ActorUserID = "cross-home-actor"
	generic, err := store.Admit(context.Background(), genericAdmission)
	require.NoError(t, err, "generic Run compatibility also honors active tenant membership")
	require.Equal(t, "u1", generic.UserID)
	require.Equal(t, "cross-home-actor", generic.ActorUserID)
}

func TestCraftRunAdmissionRejectsInactiveActorMembershipAndUser(t *testing.T) {
	tests := []struct {
		name          string
		memberStatus  string
		memberDeleted bool
		memberPresent bool
		userActive    bool
		userDeleted   bool
	}{
		{name: "suspended membership", memberStatus: "suspended", memberPresent: true, userActive: true},
		{name: "removed membership", memberStatus: "active", memberDeleted: true, memberPresent: true, userActive: true},
		{name: "missing membership", memberPresent: false, userActive: true},
		{name: "inactive user", memberStatus: "active", memberPresent: true, userActive: false},
		{name: "deleted user", memberStatus: "active", memberPresent: true, userActive: true, userDeleted: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := openRunTestDB(t)
			require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (2, 'tenant-2', 'test')`).Error)
			require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES (?, ?, ?, ?, ?)`,
				"tenant-2-owner", "tenant-2-owner", "tenant-2-owner@example.test", "x", 2).Error)
			var deletedAt any
			if tc.userDeleted {
				deletedAt = time.Now().UTC()
			}
			require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id, is_active, deleted_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
				"tenant-2-actor", "tenant-2-actor", "tenant-2-actor@example.test", "x", 2, tc.userActive, deletedAt).Error)
			if tc.memberPresent {
				var memberDeleted any
				if tc.memberDeleted {
					memberDeleted = time.Now().UTC()
				}
				require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id, user_id, role, status, deleted_at) VALUES (2, 'tenant-2-actor', 'contributor', ?, ?)`,
					tc.memberStatus, memberDeleted).Error)
			}
			require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s-tenant-2', 2, 'tenant-2-session', 'tenant-2-owner', 'trpc')`).Error)
			require.NoError(t, db.Exec(`INSERT INTO craft_sessions (session_id, tenant_id, kind) VALUES ('s-tenant-2', 2, 'web')`).Error)

			in := testAdmission()
			in.Key = agentruntime.RunKey{TenantID: 2, RunID: "inactive-actor-" + tc.name}
			in.SessionID = "s-tenant-2"
			in.UserID = "tenant-2-owner"
			in.ActorUserID = "tenant-2-actor"
			in.RequestID = "request-" + tc.name
			in.AssistantMessageID = "assistant-" + tc.name
			_, err := NewAgentRunStore(db).Admit(context.Background(), in)
			require.ErrorIs(t, err, agentruntime.ErrConflict, "home tenant alone is not proof of current membership")
		})
	}
}

func TestCraftRunAdmissionStoresOwnerAndAuthenticatedActorSeparately(t *testing.T) {
	db := openRunTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES (?, ?, ?, ?, ?)`,
		"collaborator", "collaborator", "collaborator@example.test", "x", 1).Error)
	require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id, user_id, role, status) VALUES (1, 'collaborator', 'contributor', 'active')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO craft_sessions (session_id, tenant_id, kind) VALUES ('s1', 1, 'web')`).Error)
	putCraftWorkspace(t, NewCraftStore(db))

	in := craftTestAdmission()
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
	require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id, user_id, role, status) VALUES (1, 'collaborator', 'contributor', 'active'), (2, 'other-tenant-user', 'contributor', 'active')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO craft_sessions (session_id, tenant_id, kind) VALUES ('s1', 1, 'web')`).Error)
	putCraftWorkspace(t, NewCraftStore(db))

	in := craftTestAdmission()
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
	putCraftWorkspace(t, NewCraftStore(db))
	store := NewAgentRunStore(db)
	in := craftTestAdmission()
	in.Key.RunID = "craft-actor-missing"
	_, err := store.Admit(context.Background(), in)
	require.ErrorIs(t, err, agentruntime.ErrConflict, "new Craft rows cannot infer actor from storage owner")

	require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES (?, ?, ?, ?, ?)`,
		"collaborator", "collaborator", "collaborator@example.test", "x", 1).Error)
	require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id, user_id, role, status) VALUES (1, 'collaborator', 'contributor', 'active')`).Error)
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
