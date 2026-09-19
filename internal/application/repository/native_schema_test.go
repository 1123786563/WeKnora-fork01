package repository

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestNativeSchemaMigrationsCreateScopedNamespace catches a deployment that
// reaches the current migration head without the isolated native-agent
// namespace.  It exercises the real migration set for each active dialect;
// PostgreSQL stays an explicit blocked-env skip when no isolated DSN exists.
func TestNativeSchemaMigrationsCreateScopedNamespace(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openRunTestDB(t)
			require.NoError(t, ValidateNativeSchemaManifest())
			for _, table := range NativeSchemaManifest() {
				require.Truef(t, db.Migrator().HasTable(table.Name), "native schema table %q must exist", table.Name)
				for _, column := range table.Columns {
					require.Truef(t, db.Migrator().HasColumn(table.Name, column), "%s.%s must exist", table.Name, column)
				}
			}

			seedNativeSchemaFixture(t, db)
			if dialect == "sqlite" {
				assertNativeSQLiteConstraints(t, db)
			}
			// A session ID is scoped by its owner; owner mismatch on a run is a
			// database FK violation, not a repository convention.
			require.NoError(t, db.Exec(`INSERT INTO native_agent_sessions (tenant_id, owner_id, session_id) VALUES (?, ?, ?)`, 1, "u2", "s1").Error)
			require.Error(t, db.Exec(`INSERT INTO native_agent_runs (tenant_id, run_id, owner_id, session_id, lease_epoch) VALUES (?, ?, ?, ?, ?)`, 1, "wrong-owner", "missing", "s1", 1).Error)
			// The request identity accepts an idempotent same-hash replay but
			// leaves a changed hash untouched for the caller to report conflict.
			require.NoError(t, db.Exec(`INSERT INTO native_agent_runs (tenant_id, run_id, owner_id, session_id, request_id, input_hash, lease_epoch) VALUES (?, ?, ?, ?, ?, ?, ?)`, 1, "request-first", "u1", "s1", "request-key", "input-hash", 1).Error)
			sameRequest := db.Exec(`INSERT INTO native_agent_runs (tenant_id, run_id, owner_id, session_id, request_id, input_hash, lease_epoch) VALUES (?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (tenant_id, owner_id, session_id, request_id) DO UPDATE SET input_hash = excluded.input_hash
				WHERE native_agent_runs.input_hash = excluded.input_hash`, 1, "request-replay", "u1", "s1", "request-key", "input-hash", 1)
			require.NoError(t, sameRequest.Error)
			require.EqualValues(t, 1, sameRequest.RowsAffected)
			differentRequest := db.Exec(`INSERT INTO native_agent_runs (tenant_id, run_id, owner_id, session_id, request_id, input_hash, lease_epoch) VALUES (?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (tenant_id, owner_id, session_id, request_id) DO UPDATE SET input_hash = excluded.input_hash
				WHERE native_agent_runs.input_hash = excluded.input_hash`, 1, "request-conflict", "u1", "s1", "request-key", "other-hash", 1)
			require.NoError(t, differentRequest.Error)
			require.EqualValues(t, 0, differentRequest.RowsAffected)
			require.NoError(t, db.Exec(`INSERT INTO native_agent_runs (tenant_id, run_id, owner_id, session_id, request_id, input_hash, lease_epoch) VALUES (?, ?, ?, ?, ?, ?, ?)`, 2, "request-first", "u1", "s1", "request-key", "input-hash", 1).Error)

			// Scope rows must remain tenant-local even when users and session IDs
			// collide across tenants.  The stable event identity itself is tenant
			// scoped; the payload hash is retained for P1.3 conflict handling.
			require.NoError(t, db.Exec(`INSERT INTO native_agent_session_events
				(tenant_id, app_name, user_id, session_id, stable_event_id, payload_hash)
				VALUES (?, ?, ?, ?, ?, ?)`, 1, "native", "u1", "s1", "evt-1", "hash-1").Error)
			same := db.Exec(`INSERT INTO native_agent_session_events
				(tenant_id, app_name, user_id, session_id, stable_event_id, payload_hash)
				VALUES (?, ?, ?, ?, ?, ?)
				ON CONFLICT (tenant_id, app_name, user_id, session_id, stable_event_id)
				DO UPDATE SET payload_hash = excluded.payload_hash
				WHERE native_agent_session_events.payload_hash = excluded.payload_hash`, 1, "native", "u1", "s1", "evt-1", "hash-1")
			require.NoError(t, same.Error, "the same stable event ID and hash must be reusable")
			require.EqualValues(t, 1, same.RowsAffected)
			different := db.Exec(`INSERT INTO native_agent_session_events
				(tenant_id, app_name, user_id, session_id, stable_event_id, payload_hash)
				VALUES (?, ?, ?, ?, ?, ?)
				ON CONFLICT (tenant_id, app_name, user_id, session_id, stable_event_id)
				DO UPDATE SET payload_hash = excluded.payload_hash
				WHERE native_agent_session_events.payload_hash = excluded.payload_hash`, 1, "native", "u1", "s1", "evt-1", "hash-2")
			require.NoError(t, different.Error)
			require.EqualValues(t, 0, different.RowsAffected, "a changed hash must be visible to P1.3 as a conflict, never overwrite the receipt")
			require.NoError(t, db.Exec(`INSERT INTO native_agent_session_events
				(tenant_id, app_name, user_id, session_id, stable_event_id, payload_hash)
				VALUES (?, ?, ?, ?, ?, ?)`, 2, "native", "u1", "s1", "evt-1", "hash-1").Error,
				"a matching ID in another tenant is independent")

			require.Error(t, db.Exec(`INSERT INTO native_agent_runs
				(tenant_id, run_id, session_id, owner_id, lease_epoch)
				VALUES (?, ?, ?, ?, ?)`, 999, "foreign-run", "s1", "u1", 1).Error,
				"a run cannot be admitted for an unknown tenant")
			require.Error(t, db.Exec(`INSERT INTO native_agent_attempts
				(tenant_id, run_id, attempt_id, lease_epoch)
				VALUES (?, ?, ?, ?)`, 1, "missing-run", "attempt-1", 1).Error,
				"an attempt requires its scoped run")
			require.NoError(t, db.Exec(`INSERT INTO native_agent_runs
				(tenant_id, run_id, session_id, owner_id, lease_epoch)
				VALUES (?, ?, ?, ?, ?)`, 2, "run-1", "s1", "u1", 1).Error,
				"the same run ID is independent in a second tenant")
			require.NoError(t, db.Exec(`INSERT INTO native_agent_inputs
				(tenant_id, run_id, input_id, input_hash) VALUES (?, ?, ?, ?)`, 1, "run-1", "request-1", "request-hash").Error)
			require.NoError(t, db.Exec(`INSERT INTO native_agent_inputs
				(tenant_id, run_id, input_id, input_hash) VALUES (?, ?, ?, ?)`, 2, "run-1", "request-1", "request-hash").Error,
				"a request identity does not cross tenant scope")
			require.Error(t, db.Exec(`INSERT INTO native_agent_memory_scopes
				(tenant_id, user_id, generation, tombstone_generation) VALUES (?, ?, ?, ?)`, 1, "u3", 1, 2).Error,
				"a tombstone cannot exceed its current memory generation")

			var generation, tombstone int64
			require.NoError(t, db.Raw(`SELECT generation, tombstone_generation
				FROM native_agent_memory_scopes
				WHERE tenant_id = ? AND user_id = ?`, 1, "u1").Row().Scan(&generation, &tombstone))
			require.EqualValues(t, 3, generation)
			require.EqualValues(t, 2, tombstone)
		})
	}
}

func assertNativeSQLiteConstraints(t *testing.T, db *gorm.DB) {
	t.Helper()
	var primaryKeyColumns, foreignKeys, indexes int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM pragma_table_info('native_agent_runs') WHERE pk > 0`).Scan(&primaryKeyColumns).Error)
	require.GreaterOrEqual(t, primaryKeyColumns, int64(2))
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM pragma_foreign_key_list('native_agent_runs')`).Scan(&foreignKeys).Error)
	require.GreaterOrEqual(t, foreignKeys, int64(3))
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM pragma_index_list('native_agent_runs')`).Scan(&indexes).Error)
	require.GreaterOrEqual(t, indexes, int64(2))
}

// nativeSchemaPriorVersion is the migration version immediately below the
// native-agent schema migration in each dialect's chain (sqlite 000083,
// versioned 000162). The guard tests roll back to it instead of one step
// down from head: later migrations keep landing above the native schema, and
// what must stay guarded is that namespace's own down path.
func nativeSchemaPriorVersion(db *gorm.DB) uint {
	if db.Name() == "sqlite" {
		return 82
	}
	return 161
}

// TestNativeSchemaMigrationRollbackGuard catches a destructive down migration:
// a populated native namespace must remain intact, while an empty one can
// safely step down and re-upgrade to the same head.
func TestNativeSchemaMigrationRollbackGuard(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openRunTestDB(t)
			seedNativeSchemaFixture(t, db)
			m := nativeSchemaMigrator(t, db)
			defer func() { _, _ = m.Close() }()
			require.Error(t, m.Migrate(nativeSchemaPriorVersion(db)), "a populated namespace must reject destructive rollback")
			require.True(t, db.Migrator().HasTable("native_agent_runs"))
		})
	}
}

func TestNativeSchemaMigrationEmptyRollbackAndRepeatUpgrade(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openRunTestDB(t)
			m := nativeSchemaMigrator(t, db)
			defer func() { _, _ = m.Close() }()
			require.ErrorIs(t, m.Up(), migrate.ErrNoChange, "repeated upgrade must be a no-op")
			require.NoError(t, m.Migrate(nativeSchemaPriorVersion(db)), "an empty namespace may roll back")
			require.False(t, db.Migrator().HasTable("native_agent_runs"))
			require.NoError(t, m.Up(), "the same database must re-upgrade cleanly")
			require.True(t, db.Migrator().HasTable("native_agent_runs"))
		})
	}
}

// seedNativeSchemaFixture uses only parameter-bound SQL so the migration
// contract is exercised independently of the P1.3/P1.4 repositories.
func seedNativeSchemaFixture(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (?, ?, ?)`, 2, "tenant-2", "test").Error)
	require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES (?, ?, ?, ?, ?)`, "u2", "u2", "u2@example.test", "x", 2).Error)
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES (?, ?, ?, ?, ?)`, "s3", 2, "session-2", "u2", "trpc").Error)
	for _, tenantID := range []int{1, 2} {
		require.NoError(t, db.Exec(`INSERT INTO native_agent_tenants (tenant_id) VALUES (?)`, tenantID).Error)
	}
	require.NoError(t, db.Exec(`INSERT INTO native_agent_sessions (tenant_id, owner_id, session_id) VALUES (?, ?, ?)`, 1, "u1", "s1").Error)
	require.NoError(t, db.Exec(`INSERT INTO native_agent_sessions (tenant_id, owner_id, session_id) VALUES (?, ?, ?)`, 2, "u1", "s1").Error)
	require.NoError(t, db.Exec(`INSERT INTO native_agent_runs (tenant_id, run_id, session_id, owner_id, lease_epoch) VALUES (?, ?, ?, ?, ?)`, 1, "run-1", "s1", "u1", 1).Error)
	require.NoError(t, db.Exec(`INSERT INTO native_agent_memory_scopes (tenant_id, user_id, generation, tombstone_generation) VALUES (?, ?, ?, ?)`, 1, "u1", 3, 2).Error)
}

func nativeSchemaMigrator(t *testing.T, db *gorm.DB) *migrate.Migrate {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	conn, err := db.DB()
	require.NoError(t, err)
	if db.Name() == "sqlite" {
		driver, err := sqlite3migrate.WithInstance(conn, &sqlite3migrate.Config{NoTxWrap: true})
		require.NoError(t, err)
		m, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/sqlite"), "sqlite3", driver)
		require.NoError(t, err)
		return m
	}
	var schema string
	require.NoError(t, db.Raw("SELECT current_schema()").Row().Scan(&schema))
	driver, err := pgmigrate.WithInstance(conn, &pgmigrate.Config{SchemaName: schema})
	require.NoError(t, err)
	m, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/versioned"), "postgres", driver)
	require.NoError(t, err)
	return m
}
