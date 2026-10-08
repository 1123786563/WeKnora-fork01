package service

import (
	"context"
	"io"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

func newCraftTaskACLEnv(t *testing.T) (*craftSessionEnv, *CraftAccessService) {
	t.Helper()
	env := newCraftSessionEnv(t, openGate)
	require.NoError(t, env.db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES
		('viewer','viewer','viewer@example.test','x',1),
		('collaborator','collaborator','collaborator@example.test','x',1),
		('admin','admin','admin@example.test','x',1),
		('stale','stale','stale@example.test','x',1),
		('stranger','stranger','stranger@example.test','x',1),
		('foreign','foreign','foreign@example.test','x',2)`).Error)
	require.NoError(t, env.db.Exec(`INSERT INTO tenant_members (tenant_id,user_id,role,status,joined_at,created_at,updated_at) VALUES
		(1,'viewer','viewer','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
		(1,'collaborator','contributor','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
		(1,'admin','admin','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
		(1,'stale','viewer','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
		(1,'stranger','viewer','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
		(2,'foreign','owner','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	access := NewCraftAccessService(env.db)
	env.svc.access = access
	env.svc.taskList = access
	return env, access
}

func TestCraftSessionTaskAccessGatesContentWritesAndDownloadBytes(t *testing.T) {
	env, access := newCraftTaskACLEnv(t)
	ws := createCraftSession(t, env, "u1", "acl-task", "private task", "web")
	owner := ownerScope(1, "u1", ws.SessionID)
	viewer := ownerScope(1, "viewer", ws.SessionID)
	collaborator := ownerScope(1, "collaborator", ws.SessionID)
	admin := ownerScope(1, "admin", ws.SessionID)
	require.NoError(t, access.Grant(context.Background(), owner, "viewer", craft.TaskRoleViewer))
	require.NoError(t, access.Grant(context.Background(), owner, "collaborator", craft.TaskRoleCollaborator))

	version := publishCraftVersion(t, env, owner, "<h1>shared</h1>")
	for _, scope := range []craft.Scope{viewer, collaborator} {
		ctx := craftCtx(scope.TenantID, scope.UserID, scope.SessionID)
		_, err := env.svc.Get(ctx, scope)
		require.NoError(t, err)
		_, err = env.svc.View(ctx, scope)
		require.NoError(t, err)
		versions, err := env.svc.ListVersions(ctx, scope)
		require.NoError(t, err)
		require.Len(t, versions, 1)
		got, err := env.svc.GetVersion(ctx, scope, version.ID)
		require.NoError(t, err)
		require.Equal(t, version.ID, got.ID)
	}

	files := &craftACLTrackingFiles{body: "<h1>shared</h1>"}
	env.svc.files = files
	file, reader, err := env.svc.OpenVersionFile(craftCtx(1, "viewer", ws.SessionID), viewer, version.ID, "index.html")
	require.NoError(t, err)
	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	require.Equal(t, "<h1>shared</h1>", string(content))
	require.Equal(t, "text/html", file.MIME)
	require.Equal(t, []string{"blob://v1-index"}, files.opened)

	// A Viewer cannot associate input or request a Run, while a Collaborator
	// can. The persisted owner scope remains unchanged for storage and runs.
	addCraftUpload(t, env, ws.SessionID, "collab-input", types.TemporaryDocumentStatusReady, "shared material")
	_, err = env.svc.AssociateInput(craftCtx(1, "viewer", ws.SessionID), viewer,
		"collab-input", sha256Sum("shared material"))
	require.ErrorIs(t, err, craft.ErrForbidden)
	_, _, err = env.svc.StartRun(craftCtx(1, "viewer", ws.SessionID), viewer, CraftRunRequest{RequestID: "viewer-run", Prompt: "view only"})
	require.ErrorIs(t, err, craft.ErrForbidden)

	input, err := env.svc.AssociateInput(craftCtx(1, "collaborator", ws.SessionID), collaborator,
		"collab-input", sha256Sum("shared material"))
	require.NoError(t, err)
	require.Equal(t, "tempdocs://collab-input", input.Ref)
	run, _, err := env.svc.StartRun(craftCtx(1, "collaborator", ws.SessionID), collaborator,
		CraftRunRequest{RequestID: "collab-run", Prompt: "make a refinement"})
	require.NoError(t, err)
	require.Equal(t, ws.SessionID, run.SessionID)

	// A tenant Admin is not a Task member. Cross-tenant, same-tenant
	// nonmember, and missing-checker shared access all fail closed.
	_, err = env.svc.View(adminCraftCtx(1, "admin", ws.SessionID), admin)
	require.ErrorIs(t, err, craft.ErrForbidden)
	stranger := ownerScope(1, "stranger", ws.SessionID)
	foreign := ownerScope(2, "foreign", ws.SessionID)
	assertCraftTaskReadDenied(t, env, stranger, version.ID, files)
	assertCraftTaskWriteDenied(t, env, stranger)
	assertCraftTaskReadDenied(t, env, foreign, version.ID, files)
	assertCraftTaskWriteDenied(t, env, foreign)
	assertCraftTaskReadDenied(t, env, admin, version.ID, files)
	assertCraftTaskWriteDenied(t, env, admin)

	legacy := newCraftSessionEnv(t, openGate)
	legacyWS := createCraftSession(t, legacy, "u1", "no-access-checker", "private", "web")
	_, err = legacy.svc.Get(adminCraftCtx(1, "admin", legacyWS.SessionID), ownerScope(1, "admin", legacyWS.SessionID))
	require.Error(t, err, "a missing checker cannot activate SessionService admin fallback for Craft content")

	// Revocation is checked on every read before opening the object store.
	require.NoError(t, access.Revoke(context.Background(), owner, "viewer"))
	assertCraftTaskReadDenied(t, env, viewer, version.ID, files)
	assertCraftTaskWriteDenied(t, env, viewer)
}

func TestCraftSessionTaskAccessRejectsStaleMembershipAndAdminWithoutGrant(t *testing.T) {
	env, access := newCraftTaskACLEnv(t)
	ws := createCraftSession(t, env, "u1", "stale-acl-task", "private task", "web")
	owner := ownerScope(1, "u1", ws.SessionID)
	stale := ownerScope(1, "stale", ws.SessionID)
	admin := ownerScope(1, "admin", ws.SessionID)
	require.NoError(t, access.Grant(context.Background(), owner, "stale", craft.TaskRoleViewer))
	var oldMembershipID uint64
	require.NoError(t, env.db.Table("tenant_members").Where("tenant_id = ? AND user_id = ? AND deleted_at IS NULL", 1, "stale").Pluck("id", &oldMembershipID).Error)
	require.NotZero(t, oldMembershipID)
	require.NoError(t, env.db.Exec(`DROP INDEX IF EXISTS idx_tenant_members_user_tenant_unique`).Error)
	require.NoError(t, env.db.Exec("UPDATE tenant_members SET status = 'inactive', deleted_at = CURRENT_TIMESTAMP WHERE id = ?", oldMembershipID).Error)
	require.NoError(t, env.db.Exec(`INSERT INTO tenant_members (tenant_id,user_id,role,status,joined_at,created_at,updated_at) VALUES (1,'stale','viewer','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	version := publishCraftVersion(t, env, owner, "private bytes")
	files := &craftACLTrackingFiles{body: "private bytes"}
	env.svc.files = files
	addCraftUpload(t, env, ws.SessionID, "stale-input", types.TemporaryDocumentStatusReady, "material")
	assertCraftTaskReadDenied(t, env, stale, version.ID, files)
	assertCraftTaskWriteDenied(t, env, stale)
	_, err := env.svc.View(adminCraftCtx(1, "admin", ws.SessionID), admin)
	require.ErrorIs(t, err, craft.ErrForbidden)
}

func TestCraftSessionStartRunPersistsActorAndFencesContinueClaimByActor(t *testing.T) {
	env, access := newCraftTaskACLEnv(t)
	ws := createCraftSession(t, env, "u1", "actor-run-task", "actor task", "web")
	owner := ownerScope(1, "u1", ws.SessionID)
	collaborator := ownerScope(1, "collaborator", ws.SessionID)
	require.NoError(t, access.Grant(context.Background(), owner, collaborator.UserID, craft.TaskRoleCollaborator))
	addCraftUpload(t, env, ws.SessionID, "actor-input", types.TemporaryDocumentStatusReady, "actor material")
	input, err := env.svc.AssociateInput(craftCtx(1, collaborator.UserID, ws.SessionID), collaborator,
		"actor-input", sha256Sum("actor material"))
	require.NoError(t, err)
	require.NoError(t, env.svc.DecideInput(craftCtx(1, collaborator.UserID, ws.SessionID), collaborator, input.Ref, "continue"))

	request := CraftRunRequest{RequestID: "actor-run-key", Prompt: "build with my input", InputRefs: []string{input.Ref}}
	run, _, err := env.svc.StartRun(craftCtx(1, collaborator.UserID, ws.SessionID), collaborator, request)
	require.NoError(t, err)
	require.Equal(t, owner.UserID, run.UserID, "Run.UserID remains the Task storage owner")
	require.Equal(t, collaborator.UserID, run.ActorUserID, "Run actor comes from the authenticated Collaborator")

	var claim craftSessionRequestRow
	require.NoError(t, env.db.Where("tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ?",
		1, owner.UserID, ws.SessionID, "input_admission", craftInputDecisionKey(input.Ref)).Take(&claim).Error)
	require.Equal(t, run.Key.RunID, claim.AdmissionRunID)
	require.Equal(t, "admitted", claim.AdmissionState)
	claimToken := claim.AdmissionToken

	replay, _, err := env.svc.StartRun(craftCtx(1, collaborator.UserID, ws.SessionID), collaborator, request)
	require.NoError(t, err, "same actor replays the admitted Run")
	require.Equal(t, run.Key.RunID, replay.Key.RunID)

	_, _, err = env.svc.StartRun(craftCtx(1, owner.UserID, ws.SessionID), owner, request)
	require.ErrorIs(t, err, craft.ErrConflict, "Task owner cannot replay the Collaborator's same-key Run")
	var after craftSessionRequestRow
	require.NoError(t, env.db.Where("tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ?",
		1, owner.UserID, ws.SessionID, "input_admission", craftInputDecisionKey(input.Ref)).Take(&after).Error)
	require.Equal(t, claimToken, after.AdmissionToken, "cross-actor replay must not steal or rotate the T01 claim")
	require.Equal(t, collaborator.UserID, run.ActorUserID)

	// An abandoned claim has no durable Run row from which to recover an
	// actor. Its actor-bound decision digest must therefore fail closed rather
	// than allowing another Task writer to rotate the claim after expiry.
	addCraftUpload(t, env, ws.SessionID, "stale-actor-input", types.TemporaryDocumentStatusReady, "stale actor material")
	staleInput, err := env.svc.AssociateInput(craftCtx(1, collaborator.UserID, ws.SessionID), collaborator,
		"stale-actor-input", sha256Sum("stale actor material"))
	require.NoError(t, err)
	require.NoError(t, env.svc.DecideInput(craftCtx(1, collaborator.UserID, ws.SessionID), collaborator, staleInput.Ref, "continue"))
	var storedSession types.Session
	require.NoError(t, env.db.Where("tenant_id = ? AND id = ?", 1, ws.SessionID).Take(&storedSession).Error)
	claims, staleRunID, err := env.svc.claimInputDecisions(craftCtx(1, collaborator.UserID, ws.SessionID), &storedSession,
		[]craft.Input{staleInput}, "abandoned-collaborator-key", "abandoned-collaborator-run", collaborator.UserID)
	require.NoError(t, err)
	require.Len(t, claims, 1)
	require.Equal(t, "abandoned-collaborator-run", staleRunID)
	key := craftInputDecisionKey(staleInput.Ref)
	require.NoError(t, env.db.Model(&craftSessionRequestRow{}).
		Where("tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ?", 1, owner.UserID, ws.SessionID, "input_admission", key).
		Update("lease_expires_at", time.Now().Add(-time.Minute)).Error)
	_, _, err = env.svc.claimInputDecisions(craftCtx(1, owner.UserID, ws.SessionID), &storedSession,
		[]craft.Input{staleInput}, "abandoned-collaborator-key", "owner-takeover-run", owner.UserID)
	require.ErrorIs(t, err, craft.ErrConflict, "expired claims without durable actor proof cannot be transferred")
	var staleClaim craftSessionRequestRow
	require.NoError(t, env.db.Where("tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ?", 1, owner.UserID, ws.SessionID, "input_admission", key).Take(&staleClaim).Error)
	require.Equal(t, claims[0].Token, staleClaim.AdmissionToken, "failed actor recovery preserves the original claim token")
}

func TestCraftSessionStartRunRejectsScopeThatDiffersFromAuthenticatedCaller(t *testing.T) {
	env, _ := newCraftTaskACLEnv(t)
	ws := createCraftSession(t, env, "u1", "actor-scope-task", "actor task", "web")
	owner := ownerScope(1, "u1", ws.SessionID)
	_, _, err := env.svc.StartRun(craftCtx(1, "different-user", ws.SessionID), owner,
		CraftRunRequest{RequestID: "scope-mismatch", Prompt: "no spoof"})
	require.ErrorIs(t, err, craft.ErrForbidden)
	var runs int64
	require.NoError(t, env.db.Table("agent_runs").Where("tenant_id = ? AND owner_id = ? AND session_id = ?", 1, owner.UserID, ws.SessionID).Count(&runs).Error)
	require.Zero(t, runs, "caller mismatch must be rejected before durable admission")
}

func assertCraftTaskReadDenied(t *testing.T, env *craftSessionEnv, scope craft.Scope, versionID string, files *craftACLTrackingFiles) {
	t.Helper()
	ctx := craftCtx(scope.TenantID, scope.UserID, scope.SessionID)
	_, err := env.svc.Get(ctx, scope)
	require.Error(t, err, "Get must deny %s", scope.UserID)
	_, err = env.svc.View(ctx, scope)
	require.Error(t, err, "View must deny %s", scope.UserID)
	_, err = env.svc.WorkspaceInputs(ctx, scope)
	require.Error(t, err, "WorkspaceInputs must deny %s", scope.UserID)
	_, err = env.svc.ListVersions(ctx, scope)
	require.Error(t, err, "ListVersions must deny %s", scope.UserID)
	_, err = env.svc.GetVersion(ctx, scope, versionID)
	require.Error(t, err, "GetVersion must deny %s", scope.UserID)
	opened := len(files.opened)
	_, reader, err := env.svc.OpenVersionFile(ctx, scope, versionID, "index.html")
	require.Error(t, err, "OpenVersionFile must deny %s", scope.UserID)
	require.Nil(t, reader, "denied access must not return a reader")
	require.Len(t, files.opened, opened, "denied access must not open object bytes")
}

func assertCraftTaskWriteDenied(t *testing.T, env *craftSessionEnv, scope craft.Scope) {
	t.Helper()
	ctx := craftCtx(scope.TenantID, scope.UserID, scope.SessionID)
	_, err := env.svc.AssociateInput(ctx, scope, "collab-input", sha256Sum("shared material"))
	require.Error(t, err, "AssociateInput must deny %s", scope.UserID)
	_, _, err = env.svc.StartRun(ctx, scope, CraftRunRequest{RequestID: "denied-" + scope.UserID, Prompt: "no access"})
	require.Error(t, err, "StartRun must deny %s", scope.UserID)
}

func TestCraftSessionListPaginatesOwnedAndGrantedTasksWithCurrentMembership(t *testing.T) {
	env, access := newCraftTaskACLEnv(t)
	owned := createCraftSession(t, env, "viewer", "list-owned", "owned", "web")
	sharedA := createCraftSession(t, env, "u1", "list-a", "shared A", "web")
	sharedB := createCraftSession(t, env, "u1", "list-b", "shared B", "web")
	sharedC := createCraftSession(t, env, "u1", "list-c", "shared C", "web")
	staleTask := createCraftSession(t, env, "u1", "list-stale", "stale", "web")
	for _, task := range []craft.Workspace{sharedA, sharedB, sharedC} {
		require.NoError(t, access.Grant(context.Background(), ownerScope(1, "u1", task.SessionID), "viewer", craft.TaskRoleViewer))
	}
	require.NoError(t, access.Grant(context.Background(), ownerScope(1, "u1", staleTask.SessionID), "stale", craft.TaskRoleViewer))
	// Old membership incarnation must not keep the stale Task visible after
	// the user leaves and rejoins the tenant.
	var oldID uint64
	require.NoError(t, env.db.Table("tenant_members").Where("tenant_id = ? AND user_id = ? AND deleted_at IS NULL", 1, "stale").Pluck("id", &oldID).Error)
	require.NoError(t, env.db.Exec(`DROP INDEX IF EXISTS idx_tenant_members_user_tenant_unique`).Error)
	require.NoError(t, env.db.Exec("UPDATE tenant_members SET status = 'inactive', deleted_at = CURRENT_TIMESTAMP WHERE id = ?", oldID).Error)
	require.NoError(t, env.db.Exec(`INSERT INTO tenant_members (tenant_id,user_id,role,status,joined_at,created_at,updated_at) VALUES (1,'stale','viewer','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)

	times := map[string]time.Time{
		owned.SessionID:     time.Date(2030, 1, 1, 0, 0, 5, 0, time.UTC),
		sharedA.SessionID:   time.Date(2030, 1, 1, 0, 0, 4, 0, time.UTC),
		sharedB.SessionID:   time.Date(2030, 1, 1, 0, 0, 4, 0, time.UTC),
		sharedC.SessionID:   time.Date(2030, 1, 1, 0, 0, 3, 0, time.UTC),
		staleTask.SessionID: time.Date(2030, 1, 1, 0, 0, 2, 0, time.UTC),
	}
	for id, updated := range times {
		require.NoError(t, env.db.Exec("UPDATE sessions SET updated_at = ? WHERE id = ?", updated, id).Error)
	}
	viewer := ownerScope(1, "viewer", "")
	ctx := craftCtx(1, "viewer", "")
	first, cursor, err := env.svc.List(ctx, viewer, "", 2)
	require.NoError(t, err)
	require.Len(t, first, 2)
	require.NotEmpty(t, cursor)
	second, end, err := env.svc.List(ctx, viewer, cursor, 2)
	require.NoError(t, err)
	require.Len(t, second, 2)
	require.Empty(t, end)
	replay, replayEnd, err := env.svc.List(ctx, viewer, cursor, 2)
	require.NoError(t, err)
	require.Equal(t, second, replay, "the same keyset cursor must replay the same page")
	require.Equal(t, end, replayEnd)

	all := append(append([]CraftSessionSummary(nil), first...), second...)
	wantIDs := []string{owned.SessionID, sharedA.SessionID, sharedB.SessionID, sharedC.SessionID}
	sort.Slice(wantIDs, func(i, j int) bool {
		ti, tj := times[wantIDs[i]], times[wantIDs[j]]
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return wantIDs[i] > wantIDs[j]
	})
	gotIDs := make([]string, 0, len(all))
	for _, row := range all {
		gotIDs = append(gotIDs, row.SessionID)
	}
	require.Equal(t, wantIDs, gotIDs, "owners and current grants are ordered by updated_at DESC, id DESC")

	// Revocation and tenant Admin status do not leak a Task into the next page.
	require.NoError(t, access.Revoke(context.Background(), ownerScope(1, "u1", sharedC.SessionID), "viewer"))
	afterRevoke, _, err := env.svc.List(ctx, viewer, "", 10)
	require.NoError(t, err)
	require.Len(t, afterRevoke, 3)
	for _, row := range afterRevoke {
		require.NotEqual(t, staleTask.SessionID, row.SessionID)
	}
	adminRows, _, err := env.svc.List(adminCraftCtx(1, "admin", ""), ownerScope(1, "admin", ""), "", 10)
	require.NoError(t, err)
	require.Empty(t, adminRows)
	_, _, err = env.svc.List(craftCtx(1, "outside", ""), ownerScope(1, "outside", ""), "", 10)
	require.ErrorIs(t, err, craft.ErrForbidden)
	foreignRows, _, err := env.svc.List(craftCtx(2, "foreign", ""), ownerScope(2, "foreign", ""), "", 10)
	require.NoError(t, err)
	require.Empty(t, foreignRows, "cross-tenant list cannot reveal another tenant's Craft IDs")
}

type craftACLTrackingFiles struct {
	interfaces.FileService
	opened []string
	body   string
}

func (f *craftACLTrackingFiles) GetFile(_ context.Context, ref string) (io.ReadCloser, error) {
	f.opened = append(f.opened, ref)
	return io.NopCloser(strings.NewReader(f.body)), nil
}
