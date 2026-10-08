package service

// T09 (#135) RED: the Collaborator serialized-edit journey at the highest
// service seam. One admitted Craft Task has one persistent Workspace whose
// storage identity stays the Task owner (Task ID = Session ID); a Run
// requested by a Collaborator must (1) be admitted under the caller's own
// CURRENT Task grant, (2) execute in the SAME Workspace behind the T16
// writer lease, (3) bind the ACTUAL initiating member as the durable actor
// (the identity the RunView knowledge/connector authority follows), and
// (4) leave an audit trail that records the initiating member. Viewer,
// revoked Collaborator and stranger members are refused with an audited
// denial, and a concurrent edit request is surfaced as a typed conflict
// without corrupting the Workspace's published versions.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
)

// t09Env is the journey environment: the real session service assembled with
// the T08 persistent Task ACL and the T16 writer-lease seam (probe-backed).
type t09Env struct {
	*craftSessionEnv
	svc    *CraftSessionService
	access *CraftAccessService
	probe  *t16LeaseProbe
}

// newT09Env assembles the production-shaped service: Access + WriterLeases.
func newT09Env(t *testing.T, leaseStatus craft.WriterAcquireStatus) *t09Env {
	t.Helper()
	env := newCraftSessionEnv(t, openGate)
	probe := &t16LeaseProbe{status: leaseStatus}
	svc, err := NewCraftSessionService(CraftSessionConfig{
		DB: env.db, Sessions: env.sessions, Store: env.store, Versions: env.versions,
		Runs: env.runs, ActiveRuns: CraftActiveRunsQuery(env.db),
		TemporaryDocs: env.docs, Files: &fakeCraftFiles{blobs: map[string][]byte{}},
		Models: env.models, Gate: openGate,
		WriterLeases: probe,
	})
	require.NoError(t, err)
	access := NewCraftAccessService(env.db)
	svc.access = access
	// The journey needs a third same-tenant member (the Viewer); u1/u2 come
	// from the shared fixture.
	require.NoError(t, env.db.Exec(
		`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES ('u3', 'u3', 'u3@example.test', 'x', 1)`,
	).Error)
	require.NoError(t, env.db.Exec(`INSERT INTO tenant_members (tenant_id,user_id,role,status,joined_at,created_at,updated_at) VALUES
		(1,'u3','contributor','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	return &t09Env{craftSessionEnv: env, svc: svc, access: access, probe: probe}
}

// t09Grant adds one same-tenant Task grant through the T08 owner surface.
func t09Grant(t *testing.T, env *t09Env, owner craft.Scope, userID string, role craft.TaskRole) {
	t.Helper()
	require.NoError(t, env.access.Grant(context.Background(), owner, userID, role))
}

// t09AuditRows returns the audit rows for one action, newest first.
func t09AuditRows(t *testing.T, env *t09Env, action string) []craftAccessAudit {
	t.Helper()
	var rows []craftAccessAudit
	require.NoError(t, env.db.Where("action = ?", action).Order("created_at DESC, id DESC").Find(&rows).Error)
	return rows
}

// t09RunsForSession reads the durable run rows admitted for one Task.
func t09RunsForSession(t *testing.T, env *t09Env, sessionID string) []struct {
	RunID       string `gorm:"column:run_id"`
	UserID      string `gorm:"column:owner_id"`
	ActorUserID string `gorm:"column:actor_user_id"`
	RequestID   string `gorm:"column:request_id"`
} {
	t.Helper()
	var runs []struct {
		RunID       string `gorm:"column:run_id"`
		UserID      string `gorm:"column:owner_id"`
		ActorUserID string `gorm:"column:actor_user_id"`
		RequestID   string `gorm:"column:request_id"`
	}
	require.NoError(t, env.db.Table("agent_runs").
		Where("session_id = ?", sessionID).Find(&runs).Error)
	return runs
}

// TestCraftT09Journey is the plan-named Collaborator serialized-edit journey.
func TestCraftT09Journey(t *testing.T) {
	env := newT09Env(t, craft.WriterAcquired)
	ws := createCraftSession(t, env.craftSessionEnv, "u1", "t09-journey", "协作站点", "web")
	owner := ownerScope(1, "u1", ws.SessionID)
	// Owner publishes one immutable version before any collaborator edit:
	// the journey's corruption assertions compare against it.
	version := publishCraftVersion(t, env.craftSessionEnv, owner, "<h1>v1</h1>")

	t09Grant(t, env, owner, "u2", craft.TaskRoleCollaborator)
	t09Grant(t, env, owner, "u3", craft.TaskRoleViewer)

	t.Run("collaborator starts a serialized edit under their own grant", func(t *testing.T) {
		ctx := craftCtx(1, "u2", ws.SessionID)
		run, acquisition, err := env.svc.StartCollaboratorRun(ctx, ownerScope(1, "u2", ws.SessionID), CraftRunRequest{
			RequestID: "t09-run-1", Prompt: "把标题改成蓝色", KnowledgeScope: "kb-collab",
		})
		require.NoError(t, err, "a current Collaborator may request a Run")

		// The Run stays inside the SAME Workspace and Task: storage identity
		// is the owner's session/workspace; the run slot is Task-scoped.
		require.Equal(t, ws.SessionID, run.SessionID)
		require.Equal(t, "u1", run.UserID, "the Task owner remains the durable storage identity")
		require.Equal(t, "u2", run.ActorUserID, "the ACTUAL initiating member is the durable actor")

		// The serialized edit must acquire the workspace writer lease and the
		// T00 frozen acquisition outcome must project to the caller.
		require.Equal(t, craft.WriterAcquired, acquisition.Outcome.Status)
		require.Equal(t, ws.ID, acquisition.Outcome.WorkspaceID, "the lease names the SAME workspace")
		require.Len(t, env.probe.calls, 1)
		require.Equal(t, ws.ID, env.probe.calls[0].workspaceID)
		require.Equal(t, run.Key.RunID, env.probe.calls[0].runID, "the lease binds the collaborator-initiated run")

		// The durable run row records the initiating member (the authority
		// the RunView knowledge/connector path follows: owner personal
		// connections, keys and knowledge are never inherited).
		runs := t09RunsForSession(t, env, ws.SessionID)
		require.Len(t, runs, 1)
		require.Equal(t, "u2", runs[0].ActorUserID)
		require.Equal(t, "u1", runs[0].UserID)

		// The requested knowledge scope is frozen into the durable snapshot
		// under the collaborator's own selection — nothing else widens it.
		snapshot, err := ParseDurableRunSnapshot(run.Snapshot)
		require.NoError(t, err)
		require.NotNil(t, snapshot.CraftKnowledgeSelection)
		require.Equal(t, []string{"kb-collab"}, snapshot.CraftKnowledgeSelection.KnowledgeBaseIDs)
	})

	t.Run("the timeline audit records the actual initiating member", func(t *testing.T) {
		rows := t09AuditRows(t, env, "craft.run_started")
		require.NotEmpty(t, rows, "the run start is audited on the Task timeline")
		row := rows[0]
		require.Equal(t, "u2", row.ActorUserID, "the audit names the initiating member, never a storage-owner fallback")
		require.Equal(t, ws.SessionID, row.ScopeID)
		require.Equal(t, "run", row.TargetType)
		require.NotEmpty(t, row.TargetID, "the audit binds the stable Run ID")
		runs := t09RunsForSession(t, env, ws.SessionID)
		require.Equal(t, runs[0].RunID, row.TargetID)
		var details map[string]string
		require.NoError(t, json.Unmarshal(row.Details, &details))
		require.Equal(t, string(craft.TaskRoleCollaborator), details["role"], "the derived current role is recorded")
		require.Equal(t, "t09-run-1", details["request_id"])
	})

	t.Run("viewer cannot request a run and the refusal is audited", func(t *testing.T) {
		before := len(t09AuditRows(t, env, "craft.access_denied"))
		ctx := craftCtx(1, "u3", ws.SessionID)
		_, acquisition, err := env.svc.StartCollaboratorRun(ctx, ownerScope(1, "u3", ws.SessionID), CraftRunRequest{
			RequestID: "t09-viewer", Prompt: "viewer 尝试发起",
		})
		require.ErrorIs(t, err, craft.ErrForbidden)
		require.Equal(t, craft.WriterAcquisition{}, acquisition, "no lease outcome without admission")
		after := t09AuditRows(t, env, "craft.access_denied")
		require.Len(t, after, before+1, "the viewer refusal is audited")
		require.Equal(t, "u3", after[0].ActorUserID)
		require.Equal(t, string(craft.TaskWrite), after[0].TargetID)
		require.Empty(t, t09RunsForSession(t, env, ws.SessionID)[1:], "no second run was admitted")
	})

	t.Run("concurrent edit conflict is surfaced without corrupting the workspace", func(t *testing.T) {
		// The collaborator's run from the first leg still holds the Task's
		// single run slot: the owner's concurrent edit request is refused
		// with the typed conflict carrying the existing run id.
		_, _, err := env.svc.StartCollaboratorRun(craftCtx(1, "u1", ws.SessionID), owner, CraftRunRequest{
			RequestID: "t09-concurrent", Prompt: "owner 并发修改",
		})
		require.ErrorIs(t, err, craft.ErrBusy)
		var conflict *ActiveRunConflict
		require.ErrorAs(t, err, &conflict, "the 409 shape carries the live run id")
		runs := t09RunsForSession(t, env, ws.SessionID)
		require.Len(t, runs, 1, "no second run was admitted")

		// The Workspace's published history is untouched by the refused edit.
		versions, err := env.versions.List(context.Background(), owner)
		require.NoError(t, err)
		require.Len(t, versions, 1)
		require.Equal(t, version.ID, versions[0].ID, "the published version is unchanged")
	})

	t.Run("revoked collaborator cannot start a new run; history remains", func(t *testing.T) {
		// Free the run slot so the refusal can only come from the grant.
		runs := t09RunsForSession(t, env, ws.SessionID)
		runKey := agentruntime.RunKey{TenantID: 1, RunID: runs[0].RunID}
		stored := env.runs.Store()
		fence, err := stored.Claim(context.Background(), runKey, "t09-worker", time.Minute)
		require.NoError(t, err)
		finalizer, ok := stored.(interface {
			Finalize(context.Context, agentruntime.Fence, json.RawMessage) error
		})
		require.True(t, ok)
		require.NoError(t, finalizer.Finalize(context.Background(), fence, json.RawMessage(`{"done":true}`)))

		require.NoError(t, env.access.Revoke(context.Background(), owner, "u2"))
		_, _, err = env.svc.StartCollaboratorRun(craftCtx(1, "u2", ws.SessionID), ownerScope(1, "u2", ws.SessionID), CraftRunRequest{
			RequestID: "t09-after-revoke", Prompt: "撤销后再发起",
		})
		require.ErrorIs(t, err, craft.ErrForbidden, "a revoked collaborator has no current TaskWrite")

		// The historical audit stays: the original grant, the run the
		// collaborator legitimately started, the revocation and this refusal.
		started := t09AuditRows(t, env, "craft.run_started")
		require.Len(t, started, 1, "the historical collaborator run remains on the timeline")
		require.Equal(t, "u2", started[0].ActorUserID)
		added := t09AuditRows(t, env, "craft.member_added")
		require.NotEmpty(t, added)
		revoked := t09AuditRows(t, env, "craft.member_revoked")
		require.NotEmpty(t, revoked)
		denied := t09AuditRows(t, env, "craft.access_denied")
		require.Equal(t, "u2", denied[0].ActorUserID, "the post-revoke refusal is audited against the caller")

		// The task owner can still start the next serialized edit after the
		// slot freed: revoking one collaborator never freezes the Task.
		ownerRun, ownerAcquisition, err := env.svc.StartCollaboratorRun(craftCtx(1, "u1", ws.SessionID), owner, CraftRunRequest{
			RequestID: "t09-owner-next", Prompt: "owner 继续编辑",
		})
		require.NoError(t, err)
		require.Equal(t, "u1", ownerRun.ActorUserID)
		require.Equal(t, craft.WriterAcquired, ownerAcquisition.Outcome.Status)
	})
}

// TestCraftRunT09LeaseConflictIsProjectedWorkspaceIntact pins the workspace
// serialization surface: when the T16 lease answers conflict (another writing
// Run holds the Workspace), the acquisition outcome projects the conflict to
// the initiating member while the Workspace's published versions stay intact.
func TestCraftRunT09LeaseConflictIsProjectedWorkspaceIntact(t *testing.T) {
	env := newT09Env(t, craft.WriterConflict)
	ws := createCraftSession(t, env.craftSessionEnv, "u1", "t09-lease-conflict", "站点", "web")
	owner := ownerScope(1, "u1", ws.SessionID)
	version := publishCraftVersion(t, env.craftSessionEnv, owner, "<h1>stable</h1>")
	t09Grant(t, env, owner, "u2", craft.TaskRoleCollaborator)

	run, acquisition, err := env.svc.StartCollaboratorRun(craftCtx(1, "u2", ws.SessionID), ownerScope(1, "u2", ws.SessionID), CraftRunRequest{
		RequestID: "t09-conflict", Prompt: "并发修改",
	})
	require.NoError(t, err, "the lease outcome never fails or un-admits the committed run (T16 contract)")
	require.Equal(t, craft.WriterConflict, acquisition.Outcome.Status, "the conflict is surfaced, never swallowed")
	require.Equal(t, ws.ID, acquisition.Outcome.WorkspaceID)
	require.Equal(t, "u2", run.ActorUserID)

	versions, err := env.versions.List(context.Background(), owner)
	require.NoError(t, err)
	require.Len(t, versions, 1)
	require.Equal(t, version.ID, versions[0].ID, "a surfaced conflict never corrupts the published history")
}

// TestCraftRunT09StrangerAndCrossScopeRefused pins the grant narrowing: a
// same-tenant member with NO Task grant and a caller that is not the
// authenticated member are both refused without admission or lease traffic.
func TestCraftRunT09StrangerAndCrossScopeRefused(t *testing.T) {
	env := newT09Env(t, craft.WriterAcquired)
	ws := createCraftSession(t, env.craftSessionEnv, "u1", "t09-stranger", "站点", "web")
	owner := ownerScope(1, "u1", ws.SessionID)
	publishCraftVersion(t, env.craftSessionEnv, owner, "<h1>v1</h1>")

	// u3 is an active same-tenant member but holds no Task grant.
	_, _, err := env.svc.StartCollaboratorRun(craftCtx(1, "u3", ws.SessionID), ownerScope(1, "u3", ws.SessionID), CraftRunRequest{
		RequestID: "t09-stranger", Prompt: "陌生人发起",
	})
	require.ErrorIs(t, err, craft.ErrForbidden)

	// The scope must be the caller's own identity (no spoofed scope user).
	_, _, err = env.svc.StartCollaboratorRun(craftCtx(1, "u3", ws.SessionID), owner, CraftRunRequest{
		RequestID: "t09-spoof", Prompt: "冒充 owner",
	})
	require.ErrorIs(t, err, craft.ErrForbidden)

	require.Empty(t, env.probe.calls, "no lease is ever acquired for a refused request")
	require.Empty(t, t09RunsForSession(t, env, ws.SessionID), "no run was admitted")
}
