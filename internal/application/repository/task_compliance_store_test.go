package repository_test

// T13 (#43) Task 1: the tenant task-retention policy lane. Migration↔model
// alignment first (production migrations must create both tables), then the
// upsert round-trip and the "never set → (nil, nil)" default that keeps
// every existing tenant ungated until a policy exists.

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

// openTaskComplianceDB opens a REAL fully-migrated sqlite database (same
// pattern as openTaskGrantDB in task_grant_store_test.go).
func openTaskComplianceDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "task-compliance.db") + "?_foreign_keys=on&_busy_timeout=5000"
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
	require.NoError(t, db.Exec(
		"INSERT INTO tenants (id, name, business) VALUES (1, 'tenant-1', 'test'), (2, 'tenant-2', 'test')").Error)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func TestTaskComplianceTablesExistAfterMigrations(t *testing.T) {
	db := openTaskComplianceDB(t)
	require.True(t, db.Migrator().HasTable("tenant_task_policies"),
		"tenant_task_policies must be created by the production migrations")
	require.True(t, db.Migrator().HasTable("task_compliance_access"),
		"task_compliance_access must be created by the production migrations")
	for _, column := range []string{"tenant_id", "retention_days", "legal_hold", "updated_by", "created_at", "updated_at"} {
		require.True(t, db.Migrator().HasColumn("tenant_task_policies", column),
			"tenant_task_policies must carry %s", column)
	}
	for _, column := range []string{"id", "tenant_id", "task_id", "admin_id", "reason", "expires_at", "created_at"} {
		require.True(t, db.Migrator().HasColumn("task_compliance_access", column),
			"task_compliance_access must carry %s", column)
	}
}

func TestTaskPolicyStoreMissingReturnsNil(t *testing.T) {
	store := repository.NewTaskComplianceStore(openTaskComplianceDB(t))
	policy, err := store.GetTaskPolicy(context.Background(), 1)
	require.NoError(t, err)
	require.Nil(t, policy, "a tenant that never set a policy stays ungated (nil, nil)")
}

func TestTaskPolicyStoreUpsertRoundTrip(t *testing.T) {
	store := repository.NewTaskComplianceStore(openTaskComplianceDB(t))

	saved, err := store.UpsertTaskPolicy(context.Background(), types.TenantTaskPolicy{
		TenantID: 1, RetentionDays: 30, LegalHold: true, UpdatedBy: "u9",
	})
	require.NoError(t, err)
	require.Equal(t, 30, saved.RetentionDays)
	require.True(t, saved.LegalHold)

	// Rewrite the same tenant: PK=tenant_id, so the second write updates.
	saved, err = store.UpsertTaskPolicy(context.Background(), types.TenantTaskPolicy{
		TenantID: 1, RetentionDays: 0, LegalHold: false, UpdatedBy: "u9",
	})
	require.NoError(t, err)
	require.Equal(t, 0, saved.RetentionDays)
	require.False(t, saved.LegalHold)

	policy, err := store.GetTaskPolicy(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, policy)
	require.Equal(t, 0, policy.RetentionDays)
	require.False(t, policy.LegalHold)

	// Tenant isolation: tenant 2 never set one.
	other, err := store.GetTaskPolicy(context.Background(), 2)
	require.NoError(t, err)
	require.Nil(t, other)
}

func seedComplianceTask(t *testing.T, db *gorm.DB, tenant int, sessionID string) {
	t.Helper()
	require.NoError(t, db.Exec(
		"INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES (?, ?, ?, 'x', ?)",
		"owner-"+sessionID, "owner-"+sessionID, sessionID+"@example.test", tenant).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES (?, ?, ?, ?, 'trpc')",
		sessionID, tenant, "task-"+sessionID, "owner-"+sessionID).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO messages (id, request_id, session_id, role, content) VALUES (?, ?, ?, 'user', 'private question'), (?, ?, ?, 'assistant', 'private answer')",
		"m-"+sessionID+"-1", "req-"+sessionID, sessionID, "m-"+sessionID+"-2", "req-"+sessionID, sessionID).Error)
	require.NoError(t, db.Exec(
		// Brief's seed omits deadline, but agent_runs.deadline is NOT NULL with
		// no default (migrations/sqlite/000055_workbench_runs.up.sql:31) — the
		// seed must supply it or the insert fails on the real migrated schema.
		"INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, engine_type, status, snapshot, deadline) VALUES (?, ?, ?, ?, ?, ?, 'h1', 'trpc', 'succeeded', '{}', datetime('now','+1 hour'))",
		tenant, "run-"+sessionID, sessionID, "owner-"+sessionID, "req-"+sessionID, "m-"+sessionID+"-2").Error)
}

func TestOpenAndActiveComplianceAccessWindow(t *testing.T) {
	db := openTaskComplianceDB(t)
	seedComplianceTask(t, db, 1, "s1")
	store := repository.NewTaskComplianceStore(db)
	ctx := context.Background()
	now := time.Now().UTC()

	opened, err := store.OpenComplianceAccess(ctx, types.TaskComplianceAccess{
		TenantID: 1, TaskID: "s1", AdminID: "u9",
		Reason: "security incident review", ExpiresAt: now.Add(2 * time.Hour),
	})
	require.NoError(t, err)
	require.NotZero(t, opened.ID)

	// Review Focus 2: the active-window lookup filters by now in real time.
	window, err := store.ActiveComplianceAccess(ctx, 1, "s1", "u9", now.Add(time.Minute))
	require.NoError(t, err)
	require.NotNil(t, window)
	require.Equal(t, opened.ID, window.ID)

	stale, err := store.ActiveComplianceAccess(ctx, 1, "s1", "u9", now.Add(3*time.Hour))
	require.NoError(t, err)
	require.Nil(t, stale, "an expired window must not authorize anything")

	other, err := store.ActiveComplianceAccess(ctx, 1, "s1", "u2", now.Add(time.Minute))
	require.NoError(t, err)
	require.Nil(t, other, "a window is bound to the requesting administrator only")
}

func TestTaskMetadataFactsProjectsMetadataOnly(t *testing.T) {
	db := openTaskComplianceDB(t)
	seedComplianceTask(t, db, 1, "s1")
	seedComplianceTask(t, db, 2, "s2")
	store := repository.NewTaskComplianceStore(db)
	ctx := context.Background()

	facts, err := store.TaskMetadataFacts(ctx, 1, "s1")
	require.NoError(t, err)
	require.Equal(t, "s1", facts.TaskID)
	require.Equal(t, "task-s1", facts.Title)
	require.Equal(t, "owner-s1", facts.OwnerID)
	require.Equal(t, int64(1), facts.RunCount)
	require.Equal(t, "succeeded", facts.LastRunState)
	require.Nil(t, facts.DeletedAt, "a live task has no deleted_at")

	// Cross-tenant probe: one uniform miss (T12 convention).
	_, err = store.TaskMetadataFacts(ctx, 2, "s1")
	require.ErrorIs(t, err, types.ErrTaskComplianceNotFound)

	// A soft-deleted task is still visible to the metadata lane (deleted_at
	// set) — the administrator must be able to see a task WAS deleted.
	require.NoError(t, db.Exec("UPDATE sessions SET deleted_at = ? WHERE id = ?", time.Now().UTC(), "s1").Error)
	facts, err = store.TaskMetadataFacts(ctx, 1, "s1")
	require.NoError(t, err)
	require.NotNil(t, facts.DeletedAt)
}

func TestListTaskMessagesScopedToSession(t *testing.T) {
	db := openTaskComplianceDB(t)
	seedComplianceTask(t, db, 1, "s1")
	store := repository.NewTaskComplianceStore(db)

	messages, err := store.ListTaskMessages(context.Background(), "s1", 10)
	require.NoError(t, err)
	require.Len(t, messages, 2)
	require.Equal(t, "user", messages[0].Role)
	require.Equal(t, "private question", messages[0].Content)
	require.Equal(t, "assistant", messages[1].Role)

	// Another session's rows never leak in.
	none, err := store.ListTaskMessages(context.Background(), "s-unknown", 10)
	require.NoError(t, err)
	require.Empty(t, none)
}
