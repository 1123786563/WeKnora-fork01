package repository_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openTaskGrantDB opens a REAL migrated sqlite database (full migrations/sqlite
// track, same pattern as openWorkbenchHTTPDB) and seeds the collaboration
// fixtures: tenant 1 with owner u1, viewer-candidate u2, collaborator-candidate
// u3 and bystander u4, plus u1's task s1 and a suspended member u5.
func openTaskGrantDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "task-grants.db") + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	seedTaskGrantFixtures(t, db)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func seedTaskGrantFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (1, 'tenant-1', 'test')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (2, 'tenant-2', 'test')`).Error)
	for _, u := range []string{"u1", "u2", "u3", "u4", "u5"} {
		require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES (?, ?, ?, 'x', 1)`, u, u, u+"@example.test").Error)
	}
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s1', 1, 'task-1', 'u1', 'trpc')`).Error)
	// u5 is seeded suspended: an existing grant for a deactivated member must
	// stop resolving (the granted-read SQL joins active memberships).
	for _, m := range []struct {
		user   string
		role   types.TenantRole
		status types.TenantMemberStatus
	}{
		{"u1", types.TenantRoleContributor, types.TenantMemberStatusActive},
		{"u2", types.TenantRoleViewer, types.TenantMemberStatusActive},
		{"u3", types.TenantRoleContributor, types.TenantMemberStatusActive},
		{"u4", types.TenantRoleContributor, types.TenantMemberStatusActive},
		{"u5", types.TenantRoleViewer, types.TenantMemberStatusSuspended},
	} {
		require.NoError(t, db.Create(&types.TenantMember{
			UserID: m.user, TenantID: 1, Role: m.role, Status: m.status, JoinedAt: time.Now().UTC(),
		}).Error)
	}
}

func TestTaskGrantsTableExistsAfterMigrations(t *testing.T) {
	db := openTaskGrantDB(t)
	require.True(t, db.Migrator().HasTable("task_grants"),
		"task_grants must be created by the production migrations")
	for _, column := range []string{"tenant_id", "task_id", "grantee_id", "role", "granted_by", "created_at", "updated_at"} {
		require.True(t, db.Migrator().HasColumn("task_grants", column),
			"task_grants.%s must exist (aligned with types.TaskGrant)", column)
	}
}

func TestTaskGrantStoreUpsertListRevokeAndRoleLookup(t *testing.T) {
	db := openTaskGrantDB(t)
	store := repository.NewTaskGrantStore(db)
	ctx := context.Background()

	grant, err := store.UpsertGrant(ctx, 1, "s1", "u2", types.TaskGrantRoleViewer, "u1")
	require.NoError(t, err)
	require.Equal(t, types.TaskGrantRoleViewer, grant.Role)
	require.Equal(t, "u1", grant.GrantedBy)

	// Upsert flips the role in place (one row per (tenant, task, grantee)).
	grant, err = store.UpsertGrant(ctx, 1, "s1", "u2", types.TaskGrantRoleCollaborator, "u1")
	require.NoError(t, err)
	require.Equal(t, types.TaskGrantRoleCollaborator, grant.Role)

	grants, err := store.ListGrants(ctx, 1, "s1")
	require.NoError(t, err)
	require.Len(t, grants, 1)
	require.Equal(t, "u2", grants[0].GranteeID)

	role, found, err := store.RoleForGrantee(ctx, 1, "s1", "u2")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, types.TaskGrantRoleCollaborator, role)

	// Cross-tenant and unknown lookups are plain misses.
	_, found, err = store.RoleForGrantee(ctx, 2, "s1", "u2")
	require.NoError(t, err)
	require.False(t, found)

	// Delete is idempotent.
	require.NoError(t, store.DeleteGrant(ctx, 1, "s1", "u2"))
	require.NoError(t, store.DeleteGrant(ctx, 1, "s1", "u2"))
	grants, err = store.ListGrants(ctx, 1, "s1")
	require.NoError(t, err)
	require.Empty(t, grants)
}

func TestTaskGrantStoreTaskOwnerIDResolvesSessionOwner(t *testing.T) {
	db := openTaskGrantDB(t)
	store := repository.NewTaskGrantStore(db)
	ctx := context.Background()

	owner, err := store.TaskOwnerID(ctx, 1, "s1")
	require.NoError(t, err)
	require.Equal(t, "u1", owner, "the task owner authority is sessions.user_id (ADR-0004)")

	_, err = store.TaskOwnerID(ctx, 2, "s1")
	require.ErrorIs(t, err, repository.ErrTaskGrantTaskNotFound, "cross-tenant task id is a uniform miss")
	_, err = store.TaskOwnerID(ctx, 1, "missing")
	require.ErrorIs(t, err, repository.ErrTaskGrantTaskNotFound)
}

// openTaskGrantStoreDB builds the minimal task_grants + sessions schema
// directly. The B3-F74/F75 behaviors under test are store-level SQL semantics
// (NULL scan, upsert conflict), not migration-track properties, so these cases
// stay runnable independently of the migration track (the B4 renumber resolved
// the former 000112 duplicate-number conflict).
func openTaskGrantStoreDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE task_grants (
		tenant_id INTEGER NOT NULL, task_id TEXT NOT NULL, grantee_id TEXT NOT NULL,
		role TEXT NOT NULL, granted_by TEXT NOT NULL,
		created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL,
		PRIMARY KEY (tenant_id, task_id, grantee_id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE sessions (
		id TEXT PRIMARY KEY, tenant_id INTEGER NOT NULL, title TEXT, user_id TEXT, engine_type TEXT)`).Error)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func TestTaskGrantStoreTaskOwnerIDHandlesNullOwnerRows(t *testing.T) {
	db := openTaskGrantStoreDB(t)
	store := repository.NewTaskGrantStore(db)
	// sessions.user_id is nullable in the schema; an ownerless task must be a
	// uniform miss, not a gorm "converting NULL to string" scan failure (B3-F74).
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, user_id) VALUES ('s-null', 1, NULL)`).Error)
	_, err := store.TaskOwnerID(context.Background(), 1, "s-null")
	require.ErrorIs(t, err, repository.ErrTaskGrantTaskNotFound, "NULL user_id 是 ownerless 语义（一致 miss），不是扫描错误")
}

func TestTaskGrantStoreUpsertConflictPathReturnsPersistedCreatedAt(t *testing.T) {
	db := openTaskGrantStoreDB(t)
	store := repository.NewTaskGrantStore(db)
	ctx := context.Background()

	_, err := store.UpsertGrant(ctx, 1, "s1", "u2", types.TaskGrantRoleViewer, "u1")
	require.NoError(t, err)
	past := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Second)
	require.NoError(t, db.Exec(`UPDATE task_grants SET created_at = ? WHERE tenant_id = 1 AND task_id = 's1' AND grantee_id = 'u2'`, past).Error)

	// Conflict path (role flip on an existing grantee row): the returned
	// created_at must match the persisted row, not a freshly minted now (B3-F75).
	second, err := store.UpsertGrant(ctx, 1, "s1", "u2", types.TaskGrantRoleCollaborator, "u1")
	require.NoError(t, err)
	require.WithinDuration(t, past, second.CreatedAt, time.Second, "冲突路径返回 DB 持久化的 created_at")
	var persisted time.Time
	require.NoError(t, db.Raw(`SELECT created_at FROM task_grants WHERE tenant_id = 1 AND task_id = 's1' AND grantee_id = 'u2'`).Scan(&persisted).Error)
	require.WithinDuration(t, persisted, second.CreatedAt, time.Second)
}
