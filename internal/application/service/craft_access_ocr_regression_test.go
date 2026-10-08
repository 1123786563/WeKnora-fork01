package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newCraftAccessAuditEnv builds the minimal T08 schema with one registered
// task and one active member. The shared-cache database name is per-test so
// parallel package runs do not collide.
func newCraftAccessAuditEnv(t *testing.T) (*gorm.DB, *CraftAccessService) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "-"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	for _, ddl := range []string{
		`CREATE TABLE sessions (id text primary key, tenant_id integer not null, user_id text not null, deleted_at datetime)`,
		`CREATE TABLE craft_sessions (session_id text primary key, tenant_id integer not null, kind text not null)`,
		`CREATE TABLE tenant_members (id integer primary key, tenant_id integer not null, user_id text not null, status text not null, deleted_at datetime)`,
		`CREATE TABLE craft_task_grants (tenant_id integer not null, session_id text not null, user_id text not null, membership_id integer not null, role text not null, granted_by text not null, created_at datetime, updated_at datetime, primary key (tenant_id, session_id, user_id))`,
		`CREATE TABLE audit_logs (id integer primary key autoincrement, tenant_id integer, actor_user_id text, action text, scope_type text, scope_id text, target_type text, target_id text, target_user_id text, outcome text, details text, created_at datetime)`,
		`INSERT INTO sessions (id,tenant_id,user_id) VALUES ('task-audit',1,'owner')`,
		`INSERT INTO craft_sessions (session_id,tenant_id,kind) VALUES ('task-audit',1,'web')`,
		`INSERT INTO tenant_members (tenant_id,user_id,status) VALUES (1,'owner','active'),(1,'stranger','active')`,
	} {
		require.NoError(t, db.Exec(ddl).Error)
	}
	return db, NewCraftAccessService(db)
}

// TestCraftAccessDenialAuditDedupsWithinWindow is the OCR medium-finding
// regression: repeated denials of the same task by the same actor write one
// durable row per trailing window, mirroring LogDenied's sliding-window
// dedup, so a probing client cannot flood audit_logs at request rate.
func TestCraftAccessDenialAuditDedupsWithinWindow(t *testing.T) {
	db, svc := newCraftAccessAuditEnv(t)
	ctx := context.Background()
	scope := craft.Scope{TenantID: 1, UserID: "stranger", SessionID: "task-audit"}

	for i := 0; i < 5; i++ {
		require.ErrorIs(t, svc.CheckTaskAccess(ctx, scope, craft.TaskRead), craft.ErrForbidden, "attempt %d", i)
	}
	var rows int64
	require.NoError(t, db.Table("audit_logs").Where("action = ?", "craft.access_denied").Count(&rows).Error)
	require.EqualValues(t, 1, rows, "repeated identical denials within the dedup window write one row")

	// A different action on the same task is a distinct tuple and still
	// audited.
	require.ErrorIs(t, svc.CheckTaskAccess(ctx, scope, craft.TaskPreview), craft.ErrForbidden)
	require.NoError(t, db.Table("audit_logs").Where("action = ?", "craft.access_denied").Count(&rows).Error)
	require.EqualValues(t, 2, rows)
}

// TestCraftAccessAuditFitsActorColumnBound is the OCR medium-finding
// regression: synthetic API-key principals exceed the audit_logs
// actor_user_id VARCHAR(36) bound; the column carries a bounded hash and the
// full identity is preserved in Details, on both denial and grant rows.
func TestCraftAccessAuditFitsActorColumnBound(t *testing.T) {
	db, svc := newCraftAccessAuditEnv(t)
	ctx := context.Background()
	longActor := "api_external_user:1:" + strings.Repeat("ext", 50) // 19 + 150 chars
	// Grant also requires the target to hold an active tenant membership.
	require.NoError(t, db.Exec("INSERT INTO tenant_members (tenant_id,user_id,status) VALUES (1,?,'active')", longActor).Error)

	scope := craft.Scope{TenantID: 1, UserID: longActor, SessionID: "task-audit"}
	require.ErrorIs(t, svc.CheckTaskAccess(ctx, scope, craft.TaskRead), craft.ErrForbidden)

	var denial struct{ ActorUserID, Details string }
	require.NoError(t, db.Table("audit_logs").Where("action = ?", "craft.access_denied").Take(&denial).Error)
	require.LessOrEqual(t, len(denial.ActorUserID), 36, "column form must fit VARCHAR(36)")
	require.Contains(t, denial.ActorUserID, "sha256:", "overflowing identities are hashed into the column")
	require.Contains(t, denial.Details, longActor, "the full identity is preserved in Details")

	// Grant rows bound both actor and target columns the same way.
	require.NoError(t, svc.Grant(ctx, craft.Scope{TenantID: 1, UserID: "owner", SessionID: "task-audit"}, longActor, craft.TaskRoleViewer))
	var grant struct{ ActorUserID, TargetUserID, Details string }
	require.NoError(t, db.Table("audit_logs").Where("action = ?", "craft.member_added").Take(&grant).Error)
	require.LessOrEqual(t, len(grant.ActorUserID), 36)
	require.LessOrEqual(t, len(grant.TargetUserID), 36, "target identity column is bounded too")
	require.Contains(t, grant.Details, longActor)
	require.Contains(t, grant.Details, fmt.Sprintf("%q", "target_user_id_full"))
}

// TestCraftAuditActorUserIDHelper pins the pure helper: short identities pass
// through unchanged, long ones hash with the full form returned.
func TestCraftAuditActorUserIDHelper(t *testing.T) {
	column, full := craftAuditActorUserID("owner")
	require.Equal(t, "owner", column)
	require.Empty(t, full)

	column, full = craftAuditActorUserID("api_external_user:1:" + strings.Repeat("x", 128))
	require.NotEqual(t, "", full)
	require.Equal(t, full, "api_external_user:1:"+strings.Repeat("x", 128))
	require.LessOrEqual(t, len(column), 36)
	require.Contains(t, column, "sha256:")
}
