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
			for _, table := range nativeSchemaTableNames {
				require.Truef(t, db.Migrator().HasTable(table), "native schema table %q must exist", table)
			}
			for table, columns := range nativeSchemaRequiredColumns {
				for _, column := range columns {
					require.Truef(t, db.Migrator().HasColumn(table, column), "%s.%s must exist", table, column)
				}
			}

			seedNativeSchemaFixture(t, db)

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
			require.Error(t, m.Steps(-1), "a populated namespace must reject destructive rollback")
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
			require.NoError(t, m.Steps(-1), "an empty namespace may roll back")
			require.False(t, db.Migrator().HasTable("native_agent_runs"))
			require.NoError(t, m.Up(), "the same database must re-upgrade cleanly")
			require.True(t, db.Migrator().HasTable("native_agent_runs"))
		})
	}
}

var nativeSchemaTableNames = []string{
	"native_agent_tenants",
	"native_agent_sessions",
	"native_session_state",
	"native_agent_runs",
	"native_agent_inputs",
	"native_agent_config_bindings",
	"native_agent_memory_scopes",
	"native_agent_memory_entries",
	"native_memory_jobs",
	"native_agent_attempts",
	"native_agent_tool_calls",
	"native_agent_tool_plans",
	"native_agent_tool_results",
	"native_agent_pending_decisions",
	"native_agent_commit_intents",
	"native_agent_checkpoints",
	"native_agent_session_events",
	"native_agent_events",
	"native_agent_usage_observations",
}

var nativeSchemaRequiredColumns = map[string][]string{
	"native_agent_runs":               {"tenant_id", "run_id", "owner_id", "session_id", "request_id", "input_hash", "revision", "lease_owner", "lease_epoch"},
	"native_session_state":            {"tenant_id", "owner_id", "session_id", "state_key", "revision", "state_value"},
	"native_memory_jobs":              {"tenant_id", "subject_id", "job_id", "generation", "through_event_id", "status"},
	"native_agent_session_events":     {"tenant_id", "app_name", "user_id", "session_id", "stable_event_id", "payload", "payload_hash", "ordinal"},
	"native_agent_attempts":           {"kind", "logical_call_id", "invocation_id", "attempt_number", "provider_request_id", "lease_epoch"},
	"native_agent_tool_plans":         {"call_id", "plan_version", "kind", "service_id", "installation_id", "name", "schema_hash", "config_version", "args", "args_hash", "policy", "idempotency_key"},
	"native_agent_tool_results":       {"provider_receipt", "query_anchor", "result_hash", "effect_state", "is_error", "content"},
	"native_agent_pending_decisions":  {"call_id", "plan_version", "args_hash", "wait_kind", "expected_revision", "decision_id", "decision_hash", "detail"},
	"native_agent_commit_intents":     {"version", "payload", "payload_hash", "lease_epoch", "terminal_status"},
	"native_agent_checkpoints":        {"schema_version", "sdk_version", "graph_version", "namespace", "lineage_id", "request_payload"},
	"native_agent_events":             {"sequence", "event_id", "intent_id", "payload", "payload_hash"},
	"native_agent_usage_observations": {"revision", "provider", "model", "input_tokens", "output_tokens", "payload_hash"},
	"native_agent_inputs":             {"input_id", "input_hash", "payload", "role", "created_at"},
	"native_agent_config_bindings":    {"schema_version", "sdk_version", "graph_version", "config_hash", "credential_ref", "tool_set_hash", "source_kind", "source_id", "source_version", "target_id"},
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
