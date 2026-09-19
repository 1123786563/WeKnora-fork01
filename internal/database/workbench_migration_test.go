package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
)

// TestWorkbenchSQLiteMigrationPreservesRunChildren catches a parent-table
// rebuild that silently cascades durable checkpoints, tools, events, inputs,
// or decisions while adding remote-driver fields.
func TestWorkbenchSQLiteMigrationPreservesRunChildren(t *testing.T) {
	repoRoot := sqliteRepoRoot(t)
	legacyRoot := copySQLiteMigrationsBeforeWorkbench(t, repoRoot)
	dbPath := filepath.Join(t.TempDir(), "workbench-upgrade.db")

	chdirAndRestore(t, legacyRoot)
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: dbPath}))
	db := openSQLiteDB(t, dbPath)
	seedWorkbenchMigrationRows(t, db)
	snapshotBefore := snapshotWorkbenchRows(t, db)
	version, dirty := sqliteMigrationState(t, db)
	require.Equal(t, 54, version)
	require.False(t, dirty)

	chdirAndRestore(t, repoRoot)
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: dbPath}))
	version, dirty = sqliteMigrationState(t, db)
	require.Equal(t, 57, version)
	require.False(t, dirty)

	assertWorkbenchChildSummary(t, db)
	assertWorkbenchSnapshotsEqual(t, db, snapshotBefore)
	var driver, targetID, budgetRef string
	require.NoError(t, db.QueryRow(
		"SELECT driver, target_id, budget_ref FROM agent_runs WHERE tenant_id = 1 AND run_id = 'legacy-run'",
	).Scan(&driver, &targetID, &budgetRef))
	require.Equal(t, "platform", driver)
	require.Empty(t, targetID)
	require.Empty(t, budgetRef)

	rows, err := db.Query("PRAGMA foreign_key_check")
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	require.False(t, rows.Next(), "rebuilt parent must retain valid child foreign keys")
	require.NoError(t, rows.Err())
}

// TestWorkbenchSQLiteURLAndLaterMigrationTransaction covers both public SQLite
// entry forms and makes a synthetic v59 fail after a write. v55 must use its
// explicit transaction, while v56 and later migrations regain the standard file transaction.
func TestWorkbenchSQLiteURLAndLaterMigrationTransaction(t *testing.T) {
	repoRoot := sqliteRepoRoot(t)
	legacyRoot := copySQLiteMigrationsBeforeWorkbench(t, repoRoot)
	migrationRoot, syntheticVersion := copySQLiteMigrationsWithV58(t, repoRoot, `
CREATE TABLE workbench_v59_marker (id INTEGER PRIMARY KEY, value TEXT NOT NULL);
INSERT INTO workbench_v59_marker (id, value) VALUES (1, 'must-roll-back');
THIS IS NOT VALID SQL;
`)
	dbPath := filepath.Join(t.TempDir(), "transactional-v59.db")

	chdirAndRestore(t, legacyRoot)
	require.NoError(t, RunMigrations("sqlite3://"+dbPath))
	db := openSQLiteDB(t, dbPath)
	seedWorkbenchMigrationRows(t, db)

	chdirAndRestore(t, migrationRoot)
	err := RunMigrationsWithOptions("sqlite3://"+dbPath, MigrationOptions{SQLiteDBPath: dbPath})
	require.Error(t, err)
	assertWorkbenchChildSummary(t, db)
	var markerCount int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'workbench_v59_marker'").Scan(&markerCount))
	require.Zero(t, markerCount, "the synthetic migration must regain the default transaction wrapper after v55")
	version, dirty := sqliteMigrationState(t, db)
	require.Equal(t, syntheticVersion, version)
	require.True(t, dirty)

	require.NoError(t, os.WriteFile(filepath.Join(migrationRoot, "migrations", "sqlite", fmt.Sprintf("%06d_workbench_fixture.up.sql", syntheticVersion)), []byte(`
CREATE TABLE workbench_v59_marker (id INTEGER PRIMARY KEY, value TEXT NOT NULL);
INSERT INTO workbench_v59_marker (id, value) VALUES (1, 'recovered');
`), 0o600))
	require.NoError(t, RunMigrationsWithOptions("sqlite3://"+dbPath, MigrationOptions{
		AutoRecoverDirty: true,
		SQLiteDBPath:     dbPath,
	}))
	version, dirty = sqliteMigrationState(t, db)
	require.Equal(t, syntheticVersion, version)
	require.False(t, dirty)
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM workbench_v59_marker WHERE value = 'recovered'").Scan(&markerCount))
	require.Equal(t, 1, markerCount)
}

func TestWorkbenchSQLiteV16FailureRollsBackAndRecovers(t *testing.T) {
	repoRoot := sqliteRepoRoot(t)
	legacyRoot := copySQLiteMigrationsBeforeWorkbench(t, repoRoot)
	brokenRoot := copySQLiteMigrationsBeforeWorkbench(t, repoRoot)
	brokenUpPath := filepath.Join(brokenRoot, "migrations", "sqlite", "000055_workbench_runs.up.sql")
	validUp, err := os.ReadFile(filepath.Join(repoRoot, "migrations", "sqlite", "000055_workbench_runs.up.sql"))
	require.NoError(t, err)
	brokenUp := strings.Replace(string(validUp), "COMMIT;", "THIS IS NOT VALID SQL;\nCOMMIT;", 1)
	require.NoError(t, os.WriteFile(brokenUpPath, []byte(brokenUp), 0o600))
	dbPath := filepath.Join(t.TempDir(), "v16-failure.db")

	chdirAndRestore(t, legacyRoot)
	require.NoError(t, RunMigrations("sqlite3://"+dbPath))
	db := openSQLiteDB(t, dbPath)
	seedWorkbenchMigrationRows(t, db)
	snapshotBefore := snapshotWorkbenchRows(t, db)

	chdirAndRestore(t, brokenRoot)
	require.Error(t, RunMigrations("sqlite3://"+dbPath))
	assertWorkbenchSnapshotsEqual(t, db, snapshotBefore)
	require.False(t, sqliteTableExists(t, db, "agent_runs_rebuilt"), "failed v55 must not leave the rebuild table behind")
	version, dirty := sqliteMigrationState(t, db)
	require.Equal(t, 55, version)
	require.True(t, dirty)

	require.NoError(t, os.WriteFile(brokenUpPath, validUp, 0o600))
	require.NoError(t, RunMigrationsWithOptions("sqlite3://"+dbPath, MigrationOptions{AutoRecoverDirty: true}))
	version, dirty = sqliteMigrationState(t, db)
	require.Equal(t, 55, version)
	require.False(t, dirty)
	assertWorkbenchChildSummary(t, db)
}

func TestWorkbenchSQLiteURLUpgradeAndDownUpPreserveChildren(t *testing.T) {
	repoRoot := sqliteRepoRoot(t)
	if _, err := os.Stat(filepath.Join(repoRoot, "migrations", "sqlite", "000017_workbench_requests.up.sql")); err != nil {
		t.Skip("blocked-dependency: W04 SQLite migration 000017 is not present in this isolated W18 base")
	}
	t.Run("fresh", func(t *testing.T) {
		chdirAndRestore(t, repoRoot)
		dbPath := filepath.Join(t.TempDir(), "url-fresh.db")
		require.NoError(t, RunMigrations("sqlite3://"+dbPath))
		db := openSQLiteDB(t, dbPath)
		version, dirty := sqliteMigrationState(t, db)
		require.Equal(t, 57, version)
		require.False(t, dirty)
	})

	t.Run("legacy-upgrade-down-up", func(t *testing.T) {
		legacyRoot := copySQLiteMigrationsBeforeWorkbench(t, repoRoot)
		dbPath := filepath.Join(t.TempDir(), "url-upgrade.db")

		chdirAndRestore(t, legacyRoot)
		require.NoError(t, RunMigrations("sqlite3://"+dbPath))
		db := openSQLiteDB(t, dbPath)
		seedWorkbenchMigrationRows(t, db)
		snapshotBefore := snapshotWorkbenchRows(t, db)

		chdirAndRestore(t, repoRoot)
		require.NoError(t, RunMigrations("sqlite3://"+dbPath))
		assertWorkbenchChildSummary(t, db)
		assertWorkbenchSnapshotsEqual(t, db, snapshotBefore)
		version, dirty := sqliteMigrationState(t, db)
		require.Equal(t, 57, version)
		require.False(t, dirty)

		require.NoError(t, runWorkbenchSQLiteMigrationSteps(repoRoot, dbPath, -2))
		version, dirty = sqliteMigrationState(t, db)
		require.Equal(t, 55, version)
		require.False(t, dirty)
		assertWorkbenchChildSummary(t, db)
		require.True(t, sqliteColumnExists(t, db, "agent_runs", "driver"), "W04 down must leave the accepted W02/W03 run schema intact")

		require.NoError(t, runWorkbenchSQLiteMigrationSteps(repoRoot, dbPath, 2))
		version, dirty = sqliteMigrationState(t, db)
		require.Equal(t, 57, version)
		require.False(t, dirty)
		assertWorkbenchChildSummary(t, db)
		assertWorkbenchSnapshotsEqual(t, db, snapshotBefore)
	})
}

func TestWorkbenchSQLiteURLPreservesMigrationTableQuery(t *testing.T) {
	repoRoot := sqliteRepoRoot(t)
	chdirAndRestore(t, repoRoot)
	dbPath := filepath.Join(t.TempDir(), "custom-migrations-table.db")
	dsn := "sqlite3://" + dbPath + "?x-migrations-table=custom_schema_migrations"
	require.NoError(t, RunMigrations(dsn))
	db := openSQLiteDB(t, dbPath)
	var version, dirty int
	require.NoError(t, db.QueryRow("SELECT version, dirty FROM custom_schema_migrations").Scan(&version, &dirty))
	require.Equal(t, 57, version)
	require.Zero(t, dirty)
	var defaultTableCount int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations'").Scan(&defaultTableCount))
	require.Zero(t, defaultTableCount, "x-migrations-table must select the configured table")
}

func TestWorkbenchSQLiteInteractionActionCheck(t *testing.T) {
	repoRoot := sqliteRepoRoot(t)
	chdirAndRestore(t, repoRoot)
	dbPath := filepath.Join(t.TempDir(), "interaction-action-check.db")
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: dbPath}))
	db := openSQLiteDB(t, dbPath)
	defer db.Close()

	for i, invalid := range []struct{ kind, action string }{
		{kind: "tool_approval", action: "extend"},
		{kind: "budget", action: "approve"},
		{kind: "recovery", action: "reject"},
	} {
		_, err := db.Exec(`INSERT INTO workbench_interactions
			(tenant_id, id, run_id, owner_id, kind, args_hash, action)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, 1, fmt.Sprintf("invalid-%d", i), "run-1", "u1", invalid.kind, "hash", invalid.action)
		require.Error(t, err, "SQLite must reject invalid %s/%s action pair", invalid.kind, invalid.action)
	}
}

// TestExecutionTargetSQLiteFullMigrationDownUp verifies the current full
// migration chain retains both the W04 request queue and W18 target identity
// schema. It intentionally runs against the repository migration root; no
// migration versions are filtered from this fixture.
func TestExecutionTargetSQLiteFullMigrationDownUp(t *testing.T) {
	repoRoot := sqliteRepoRoot(t)
	if _, err := os.Stat(filepath.Join(repoRoot, "migrations", "sqlite", "000056_workbench_requests.up.sql")); err != nil {
		t.Skip("blocked-dependency: W04 SQLite migration 000056 is not present in this isolated W18 base")
	}
	chdirAndRestore(t, repoRoot)
	dbPath := filepath.Join(t.TempDir(), "execution-target-down-up.db")
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: dbPath}))
	db := openSQLiteDB(t, dbPath)
	version, dirty := sqliteMigrationState(t, db)
	require.Equal(t, 57, version)
	require.False(t, dirty)
	require.True(t, sqliteTableExists(t, db, "execution_target_identities"))
	require.True(t, sqliteTableExists(t, db, "execution_workspaces"))

	// Step down past every execution/workbench-family migration (targets at
	// 57, requests at 56, the run rebuild at 55, then observations/dispatch/
	// interactions at 21/20/19) so the paseo-owned tables are all absent.
	// The applied set has gaps (17-18 and 22-29 were never used), so reaching
	// version 16 from 57 crosses 31 applied migrations, not a contiguous 41.
	require.NoError(t, runWorkbenchSQLiteMigrationSteps(repoRoot, dbPath, -31))
	version, dirty = sqliteMigrationState(t, db)
	require.Equal(t, 16, version)
	require.False(t, dirty)
	require.False(t, sqliteTableExists(t, db, "execution_target_identities"))
	require.False(t, sqliteTableExists(t, db, "execution_observations"))
	require.False(t, sqliteTableExists(t, db, "execution_source_cursors"))
	require.NoError(t, runWorkbenchSQLiteMigrationSteps(repoRoot, dbPath, 31))
	version, dirty = sqliteMigrationState(t, db)
	require.Equal(t, 57, version)
	require.False(t, dirty)
	require.True(t, sqliteTableExists(t, db, "execution_target_identities"))
	require.True(t, sqliteTableExists(t, db, "execution_observations"))
	require.True(t, sqliteTableExists(t, db, "execution_source_cursors"))
}

// TestWorkbenchSQLiteDownRefusesPaseo catches a rollback that would silently
// drop remote-driver state for a binary that can only execute platform runs.
func TestWorkbenchSQLiteDownRefusesPaseo(t *testing.T) {
	repoRoot := sqliteRepoRoot(t)
	chdirAndRestore(t, repoRoot)
	dbPath := filepath.Join(t.TempDir(), "workbench-down.db")
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: dbPath}))
	db := openSQLiteDB(t, dbPath)
	db.SetMaxOpenConns(1)
	seedWorkbenchPaseoRow(t, db)

	script, err := os.ReadFile(filepath.Join(repoRoot, "migrations", "sqlite", "000055_workbench_runs.down.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(script))
	require.Error(t, err)
	require.NoError(t, func() error { _, rollbackErr := db.Exec("ROLLBACK"); return rollbackErr }())

	var driver string
	require.NoError(t, db.QueryRow("SELECT driver FROM agent_runs WHERE tenant_id = 1 AND run_id = 'paseo-run'").Scan(&driver))
	require.Equal(t, "paseo", driver)
}

func copySQLiteMigrationsBeforeWorkbench(t *testing.T, repoRoot string) string {
	t.Helper()
	dest := t.TempDir()
	srcDir := filepath.Join(repoRoot, "migrations", "sqlite")
	destDir := filepath.Join(dest, "migrations", "sqlite")
	require.NoError(t, os.MkdirAll(destDir, 0o755))
	entries, err := os.ReadDir(srcDir)
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		version := sqliteMigrationFileVersion(t, entry.Name())
		// This fixture models the schema immediately before 000055. Later
		// migrations may depend on the omitted workbench tables, so they must
		// be applied only when the test switches back to the repository root.
		if version > 54 {
			continue
		}
		contents, readErr := os.ReadFile(filepath.Join(srcDir, entry.Name()))
		require.NoError(t, readErr)
		require.NoError(t, os.WriteFile(filepath.Join(destDir, entry.Name()), contents, 0o600))
	}
	return dest
}

func copySQLiteMigrationsWithV58(t *testing.T, repoRoot, v58up string) (string, int) {
	t.Helper()
	dest := t.TempDir()
	srcDir := filepath.Join(repoRoot, "migrations", "sqlite")
	destDir := filepath.Join(dest, "migrations", "sqlite")
	require.NoError(t, os.MkdirAll(destDir, 0o755))
	entries, err := os.ReadDir(srcDir)
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		contents, readErr := os.ReadFile(filepath.Join(srcDir, entry.Name()))
		require.NoError(t, readErr)
		require.NoError(t, os.WriteFile(filepath.Join(destDir, entry.Name()), contents, 0o600))
	}
	// Put the synthetic transaction test after the real fixture head so it
	// cannot replace a migration that later files depend on.
	syntheticVersion := sqliteMigrationHead(t, repoRoot) + 1
	migrationName := fmt.Sprintf("%06d_workbench_fixture", syntheticVersion)
	require.NoError(t, os.WriteFile(filepath.Join(destDir, migrationName+".up.sql"), []byte(v58up), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(destDir, migrationName+".down.sql"), []byte("DROP TABLE workbench_v59_marker;\n"), 0o600))
	return dest, syntheticVersion
}

func runWorkbenchSQLiteMigrationSteps(repoRoot, dbPath string, steps int) error {
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	if err != nil {
		_ = sqlDB.Close()
		return err
	}
	m, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(repoRoot, "migrations", "sqlite"), "sqlite3", driver)
	if err != nil {
		_ = sqlDB.Close()
		return err
	}
	defer func() { _, _ = m.Close() }()
	return m.Steps(steps)
}

func assertWorkbenchChildSummary(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, assertion := range []struct {
		query string
		want  string
	}{
		{"SELECT checkpoint_id FROM agent_run_checkpoints WHERE tenant_id = 1 AND run_id = 'legacy-run'", "checkpoint-1"},
		{"SELECT tool_identity FROM agent_tool_calls WHERE tenant_id = 1 AND run_id = 'legacy-run'", "tool@v1"},
		{"SELECT status FROM agent_tool_attempts WHERE tenant_id = 1 AND run_id = 'legacy-run'", "finished"},
		{"SELECT event_type FROM agent_run_events WHERE tenant_id = 1 AND run_id = 'legacy-run'", "text.delta"},
		{"SELECT pending_id FROM agent_run_decisions WHERE tenant_id = 1 AND run_id = 'legacy-run'", "pending-1"},
		{"SELECT mode FROM agent_run_inputs WHERE tenant_id = 1 AND run_id = 'legacy-run'", "append"},
	} {
		var got string
		require.NoError(t, db.QueryRow(assertion.query).Scan(&got))
		require.Equal(t, assertion.want, got)
	}
	var driver, targetID, budgetRef string
	if sqliteColumnExists(t, db, "agent_runs", "driver") {
		require.NoError(t, db.QueryRow(
			"SELECT driver, target_id, budget_ref FROM agent_runs WHERE tenant_id = 1 AND run_id = 'legacy-run'",
		).Scan(&driver, &targetID, &budgetRef))
		require.Equal(t, "platform", driver)
		require.Empty(t, targetID)
		require.Empty(t, budgetRef)
	}
}

// snapshotWorkbenchRows records every legacy parent column and every child
// column. The migration adds only three parent columns, so the old parent
// projection is used to compare the pre/post schema while children are
// compared with their complete PRAGMA-derived projection.
func snapshotWorkbenchRows(t *testing.T, db *sql.DB) map[string][]string {
	t.Helper()
	columns := map[string][]string{
		"agent_runs": {
			"tenant_id", "run_id", "session_id", "owner_id", "request_id", "assistant_message_id",
			"request_hash", "engine_type", "status", "wait_reason", "snapshot", "graph_version",
			"sdk_version", "schema_version", "lease_owner", "lease_until", "epoch", "revision",
			"max_rounds", "max_tool_calls", "token_budget", "deadline", "created_at", "updated_at",
		},
		"agent_run_checkpoints": nil, "agent_tool_calls": nil, "agent_tool_attempts": nil,
		"agent_run_events": nil, "agent_run_decisions": nil, "agent_run_inputs": nil,
	}
	result := make(map[string][]string, len(columns))
	for table, selected := range columns {
		if selected == nil {
			rows, err := db.Query("PRAGMA table_info(" + table + ")")
			require.NoError(t, err)
			for rows.Next() {
				var cid int
				var name, typ string
				var notNull, pk int
				var defaultValue any
				require.NoError(t, rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk))
				selected = append(selected, name)
			}
			require.NoError(t, rows.Close())
			require.NoError(t, rows.Err())
		}
		query := "SELECT " + strings.Join(selected, ", ") + " FROM " + table + " WHERE tenant_id = 1 AND run_id = 'legacy-run'"
		row := db.QueryRow(query)
		values := make([]any, len(selected))
		pointers := make([]any, len(selected))
		for i := range values {
			pointers[i] = &values[i]
		}
		require.NoError(t, row.Scan(pointers...))
		for i, value := range values {
			result[table+"."+selected[i]] = []string{fmt.Sprintf("%v", value)}
		}
	}
	return result
}

func assertWorkbenchSnapshotsEqual(t *testing.T, db *sql.DB, expected map[string][]string) {
	t.Helper()
	actual := snapshotWorkbenchRows(t, db)
	require.Equal(t, expected, actual, "all preserved parent and child row values must survive the rebuild")
}

func seedWorkbenchMigrationRows(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO tenants (id, name, business) VALUES (1, 'workbench-migration', 'test')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id)
		VALUES ('u1', 'u1', 'u1@example.test', 'x', 1)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type)
		VALUES ('s1', 1, 'migration', 'u1', 'trpc')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO agent_runs (
		tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id,
		request_hash, engine_type, snapshot, deadline
	) VALUES (1, 'legacy-run', 's1', 'u1', 'request-1', 'assistant-1', 'hash-1', 'trpc', '{}', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO agent_run_checkpoints (
		tenant_id, run_id, namespace, checkpoint_id, seq, state, pending_writes
	) VALUES (1, 'legacy-run', 'graph', 'checkpoint-1', 1, '{}', '[]')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO agent_tool_calls (
		tenant_id, run_id, call_id, call_seq, tool_name, tool_identity, args_hash, args
	) VALUES (1, 'legacy-run', 'call-1', 1, 'tool', 'tool@v1', 'args-hash', '{}')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO agent_tool_attempts (
		tenant_id, run_id, call_id, attempt, epoch, status
	) VALUES (1, 'legacy-run', 'call-1', 1, 1, 'finished')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO agent_run_events (
		tenant_id, run_id, seq, event_type, payload
	) VALUES (1, 'legacy-run', 1, 'text.delta', '{}')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO agent_run_decisions (
		tenant_id, run_id, decision_id, pending_id, expected_revision, actor_id, action
	) VALUES (1, 'legacy-run', 'decision-1', 'pending-1', 0, 'u1', 'retry')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO agent_run_inputs (
		tenant_id, run_id, steer_id, mode, message
	) VALUES (1, 'legacy-run', 'input-1', 'append', '{}')`)
	require.NoError(t, err)
}

func seedWorkbenchPaseoRow(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO tenants (id, name, business) VALUES (1, 'workbench-down', 'test')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id)
		VALUES ('u1', 'u1', 'u1@example.test', 'x', 1)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type)
		VALUES ('s1', 1, 'migration', 'u1', 'builtin')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO agent_runs (
		tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id,
		request_hash, engine_type, driver, snapshot, deadline
	) VALUES (1, 'paseo-run', 's1', 'u1', 'request-1', 'assistant-1', 'hash-1', '', 'paseo', '{}', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
}
