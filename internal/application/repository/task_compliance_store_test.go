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
