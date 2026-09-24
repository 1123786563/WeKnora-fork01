package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCraftT08Journey(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:craft107-t08?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	for _, ddl := range []string{
		`CREATE TABLE sessions (id text primary key, tenant_id integer not null, user_id text not null, deleted_at datetime)`,
		`CREATE TABLE craft_sessions (session_id text primary key, tenant_id integer not null, kind text not null)`,
		`CREATE TABLE tenant_members (id integer primary key, tenant_id integer not null, user_id text not null, status text not null, deleted_at datetime)`,
		`CREATE TABLE craft_task_grants (tenant_id integer not null, session_id text not null, user_id text not null, membership_id integer not null, role text not null, granted_by text not null, created_at datetime, updated_at datetime, primary key (tenant_id, session_id, user_id))`,
		`CREATE TABLE audit_logs (id integer primary key autoincrement, tenant_id integer, actor_user_id text, action text, scope_type text, scope_id text, target_type text, target_id text, target_user_id text, outcome text, details text, created_at datetime)`,
		`INSERT INTO sessions (id,tenant_id,user_id) VALUES ('task-1',1,'owner'),('plain-1',1,'owner')`,
		`INSERT INTO craft_sessions (session_id,tenant_id,kind) VALUES ('task-1',1,'web')`,
		`INSERT INTO tenant_members (tenant_id,user_id,status) VALUES (1,'owner','active'),(1,'viewer','active'),(1,'editor','active'),(1,'admin','active'),(2,'foreign','active')`,
	} {
		require.NoError(t, db.Exec(ddl).Error)
	}
	svc := NewCraftAccessService(db)
	ctx := context.Background()
	scope := func(user string) craft.Scope { return craft.Scope{TenantID: 1, UserID: user, SessionID: "task-1"} }
	require.ErrorIs(t, svc.CheckTaskAccess(ctx, craft.Scope{TenantID: 1, UserID: "owner", SessionID: "plain-1"}, craft.TaskRead), craft.ErrNotFound)
	for _, user := range []string{"viewer", "editor", "admin"} {
		require.ErrorIs(t, svc.CheckTaskAccess(ctx, scope(user), craft.TaskRead), craft.ErrForbidden)
	}
	require.ErrorIs(t, svc.Grant(ctx, scope("admin"), "viewer", craft.TaskRoleViewer), craft.ErrForbidden)
	require.ErrorIs(t, svc.Grant(ctx, scope("owner"), "foreign", craft.TaskRoleViewer), craft.ErrForbidden)
	require.NoError(t, svc.Grant(ctx, scope("owner"), "viewer", craft.TaskRoleViewer))
	require.NoError(t, svc.Grant(ctx, scope("owner"), "editor", craft.TaskRoleCollaborator))
	for _, action := range []craft.TaskAction{craft.TaskRead, craft.TaskPreview} {
		require.NoError(t, svc.CheckTaskAccess(ctx, scope("viewer"), action))
	}
	require.ErrorIs(t, svc.CheckTaskAccess(ctx, scope("viewer"), craft.TaskWrite), craft.ErrForbidden)
	require.ErrorIs(t, svc.CheckTaskAccess(ctx, scope("viewer"), craft.TaskShare), craft.ErrForbidden)
	require.NoError(t, svc.CheckTaskAccess(ctx, scope("editor"), craft.TaskWrite))
	require.ErrorIs(t, svc.CheckTaskAccess(ctx, scope("editor"), craft.TaskShare), craft.ErrForbidden)
	// Opening an original also requires T10's resource-specific ACL. This
	// check only establishes current Task membership, never source authority.
	require.NoError(t, svc.CheckTaskAccess(ctx, scope("editor"), craft.TaskOpenSource))
	require.NoError(t, svc.Revoke(ctx, scope("owner"), "viewer"))
	for _, action := range []craft.TaskAction{craft.TaskRead, craft.TaskPreview, craft.TaskWrite} {
		require.True(t, errors.Is(svc.CheckTaskAccess(ctx, scope("viewer"), action), craft.ErrForbidden), "revoked %s", action)
	}
	listed, err := svc.ListAccessibleTaskIDs(ctx, 1, "viewer")
	require.NoError(t, err)
	require.Empty(t, listed)
	listed, err = svc.ListAccessibleTaskIDs(ctx, 1, "editor")
	require.NoError(t, err)
	require.Equal(t, []string{"task-1"}, listed)
	var audits []struct{ Action, TargetUserID, Details string }
	require.NoError(t, db.Table("audit_logs").Order("id").Find(&audits).Error)
	require.Equal(t, []string{"craft.member_added", "craft.member_added", "craft.member_revoked"}, []string{audits[0].Action, audits[1].Action, audits[2].Action})
	require.Contains(t, audits[0].Details, `"role":"viewer"`)
	var count int64
	require.NoError(t, db.Table("craft_task_grants").Where("user_id = ?", "viewer").Count(&count).Error)
	require.Zero(t, count)

	// A tenant rejoin is a new membership incarnation and requires a new Task
	// Owner decision, even when the prior Craft grant row is retained.
	require.NoError(t, svc.Grant(ctx, scope("owner"), "viewer", craft.TaskRoleViewer))
	var oldMembershipID uint64
	require.NoError(t, db.Table("tenant_members").Where("tenant_id = ? AND user_id = ? AND deleted_at IS NULL", 1, "viewer").Pluck("id", &oldMembershipID).Error)
	require.NotZero(t, oldMembershipID)
	require.NoError(t, db.Exec(`UPDATE tenant_members SET deleted_at = CURRENT_TIMESTAMP, status = 'inactive' WHERE id = ?`, oldMembershipID).Error)
	require.ErrorIs(t, svc.CheckTaskAccess(ctx, scope("viewer"), craft.TaskRead), craft.ErrForbidden)
	require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id, user_id, status) VALUES (1, 'viewer', 'active')`).Error)
	var newMembershipID uint64
	require.NoError(t, db.Table("tenant_members").Where("tenant_id = ? AND user_id = ? AND deleted_at IS NULL", 1, "viewer").Pluck("id", &newMembershipID).Error)
	require.NotZero(t, newMembershipID)
	require.NotEqual(t, oldMembershipID, newMembershipID)
	require.ErrorIs(t, svc.CheckTaskAccess(ctx, scope("viewer"), craft.TaskPreview), craft.ErrForbidden)
	listed, err = svc.ListAccessibleTaskIDs(ctx, 1, "viewer")
	require.NoError(t, err)
	require.Empty(t, listed)
	members, err := svc.ListMembers(ctx, scope("owner"))
	require.NoError(t, err)
	require.Len(t, members, 2) // owner and editor; stale viewer grant is omitted.
	for _, member := range members {
		require.NotEqual(t, "viewer", member.UserID)
	}
	require.NoError(t, svc.Grant(ctx, scope("owner"), "viewer", craft.TaskRoleViewer))
	require.NoError(t, svc.CheckTaskAccess(ctx, scope("viewer"), craft.TaskPreview))
	listed, err = svc.ListAccessibleTaskIDs(ctx, 1, "viewer")
	require.NoError(t, err)
	require.Equal(t, []string{"task-1"}, listed)
	members, err = svc.ListMembers(ctx, scope("owner"))
	require.NoError(t, err)
	require.Len(t, members, 3)
}

func TestCraftTaskLookupIsTenantAndSessionScopedIndependentOfGrant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	for _, ddl := range []string{
		`CREATE TABLE sessions (id text primary key, tenant_id integer not null, user_id text not null, deleted_at datetime)`,
		`CREATE TABLE craft_sessions (session_id text primary key, tenant_id integer not null, kind text not null)`,
		`INSERT INTO sessions (id,tenant_id,user_id) VALUES ('task-1',1,'owner'),('plain-1',1,'owner'),('task-2',2,'owner')`,
		`INSERT INTO craft_sessions (session_id,tenant_id,kind) VALUES ('task-1',1,'web'),('task-2',2,'web')`,
	} {
		require.NoError(t, db.Exec(ddl).Error)
	}
	lookup := NewCraftAccessService(db)
	registered, err := lookup.IsCraftTask(context.Background(), 1, "task-1")
	require.NoError(t, err)
	require.True(t, registered, "classification is based on registration, without requiring the actor to have a grant")
	registered, err = lookup.IsCraftTask(context.Background(), 1, "task-2")
	require.ErrorIs(t, err, craft.ErrNotFound, "same session id in another tenant is an inconsistent session lookup")
	require.False(t, registered)
	registered, err = lookup.IsCraftTask(context.Background(), 1, "plain-1")
	require.NoError(t, err)
	require.False(t, registered, "ordinary sessions remain generic")
	_, err = (*CraftAccessService)(nil).IsCraftTask(context.Background(), 1, "task-1")
	require.ErrorIs(t, err, craft.ErrForbidden, "missing lookup assembly fails closed")
}

func TestCraftTaskLookupFailsClosedForMissingOrDeletedSession(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	for _, ddl := range []string{
		`CREATE TABLE sessions (id text primary key, tenant_id integer not null, user_id text not null, deleted_at datetime)`,
		`CREATE TABLE craft_sessions (session_id text primary key, tenant_id integer not null, kind text not null)`,
		`INSERT INTO sessions (id,tenant_id,user_id) VALUES ('active-craft',1,'owner'),('deleted-craft',1,'owner'),('active-generic',1,'owner'),('wrong-tenant',2,'owner')`,
		`INSERT INTO craft_sessions (session_id,tenant_id,kind) VALUES ('active-craft',1,'web'),('deleted-craft',1,'web'),('wrong-tenant',2,'web')`,
		`UPDATE sessions SET deleted_at = CURRENT_TIMESTAMP WHERE id = 'deleted-craft'`,
	} {
		require.NoError(t, db.Exec(ddl).Error)
	}

	lookup := NewCraftAccessService(db)
	registered, err := lookup.IsCraftTask(context.Background(), 1, "active-craft")
	require.NoError(t, err)
	require.True(t, registered)
	registered, err = lookup.IsCraftTask(context.Background(), 1, "active-generic")
	require.NoError(t, err)
	require.False(t, registered, "only an existing active generic session may classify as non-Craft")

	for _, sessionID := range []string{"deleted-craft", "missing", "wrong-tenant"} {
		t.Run(sessionID, func(t *testing.T) {
			registered, err := lookup.IsCraftTask(context.Background(), 1, sessionID)
			require.ErrorIs(t, err, craft.ErrNotFound, "missing, deleted, or mismatched sessions must fail closed")
			require.False(t, registered)
		})
	}
}

// The lane's local DDL above isolates policy failures; this path also checks
// that T08's real migration and the existing sessions/audit schema agree.
func TestCraftAccessMigrationJourney(t *testing.T) {
	db := openCraftSessionDB(t)
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('task-migrated',1,'craft','u1','trpc')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO craft_sessions (session_id, tenant_id, kind) VALUES ('task-migrated',1,'web')`).Error)
	svc := NewCraftAccessService(db)
	owner := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "task-migrated"}
	viewer := craft.Scope{TenantID: 1, UserID: "u2", SessionID: "task-migrated"}
	require.ErrorIs(t, svc.CheckTaskAccess(context.Background(), viewer, craft.TaskRead), craft.ErrForbidden)
	require.NoError(t, svc.Grant(context.Background(), owner, "u2", craft.TaskRoleViewer))
	require.NoError(t, svc.CheckTaskAccess(context.Background(), viewer, craft.TaskPreview))
	var initialMemberID uint64
	require.NoError(t, db.Table("tenant_members").Where("tenant_id = ? AND user_id = ? AND deleted_at IS NULL", 1, "u2").Pluck("id", &initialMemberID).Error)
	require.NotZero(t, initialMemberID)

	// SQLite's legacy base schema uses an unconditional unique index, while
	// PostgreSQL permits a new row after soft deletion. Drop that index here to
	// exercise the intended membership-incarnation transition against the real
	// Craft migration/schema; tenant-member index parity is outside this test.
	require.NoError(t, db.Exec(`DROP INDEX IF EXISTS idx_tenant_members_user_tenant_unique`).Error)
	require.NoError(t, db.Exec(`UPDATE tenant_members SET deleted_at = CURRENT_TIMESTAMP, status = 'inactive' WHERE tenant_id = ? AND user_id = ? AND id = ?`, 1, "u2", initialMemberID).Error)
	require.ErrorIs(t, svc.CheckTaskAccess(context.Background(), viewer, craft.TaskRead), craft.ErrForbidden)
	listed, err := svc.ListAccessibleTaskIDs(context.Background(), 1, "u2")
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Empty(t, listed)

	require.NoError(t, db.Exec(`INSERT INTO tenant_members (user_id, tenant_id, role, status, joined_at, created_at, updated_at) VALUES ('u2', 1, 'admin', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error)
	var replacementMemberID uint64
	require.NoError(t, db.Table("tenant_members").Where("tenant_id = ? AND user_id = ? AND deleted_at IS NULL", 1, "u2").Pluck("id", &replacementMemberID).Error)
	require.NotZero(t, replacementMemberID)
	require.NotEqual(t, initialMemberID, replacementMemberID)
	require.ErrorIs(t, svc.CheckTaskAccess(context.Background(), viewer, craft.TaskRead), craft.ErrForbidden)
	listed, err = svc.ListAccessibleTaskIDs(context.Background(), 1, "u2")
	require.NoError(t, err)
	require.Empty(t, listed)
	members, err := svc.ListMembers(context.Background(), owner)
	require.NoError(t, err)
	require.Len(t, members, 1, "the stale grant is not an effective Task member")

	require.NoError(t, svc.Grant(context.Background(), owner, "u2", craft.TaskRoleViewer))
	require.NoError(t, svc.CheckTaskAccess(context.Background(), viewer, craft.TaskPreview))
	listed, err = svc.ListAccessibleTaskIDs(context.Background(), 1, "u2")
	require.NoError(t, err)
	require.Equal(t, []string{"task-migrated"}, listed)
	members, err = svc.ListMembers(context.Background(), owner)
	require.NoError(t, err)
	require.Len(t, members, 2)
	require.Equal(t, craft.TaskRoleViewer, members[1].Role)
	var grantMembershipID uint64
	require.NoError(t, db.Table("craft_task_grants").Where("tenant_id = ? AND session_id = ? AND user_id = ?", 1, "task-migrated", "u2").Pluck("membership_id", &grantMembershipID).Error)
	require.Equal(t, replacementMemberID, grantMembershipID)
	require.NoError(t, svc.Revoke(context.Background(), owner, "u2"))
	require.ErrorIs(t, svc.CheckTaskAccess(context.Background(), viewer, craft.TaskRead), craft.ErrForbidden)
}
