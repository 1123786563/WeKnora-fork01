package database

import (
	"database/sql"
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
	version, dirty := sqliteMigrationState(t, db)
	require.Equal(t, 15, version)
	require.False(t, dirty)

	chdirAndRestore(t, repoRoot)
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: dbPath}))
	version, dirty = sqliteMigrationState(t, db)
	require.Equal(t, 16, version)
	require.False(t, dirty)

	assertWorkbenchChildSummary(t, db)
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
// entry forms and makes a synthetic v17 fail after a write. v16 must use its
// explicit transaction, while v17 must regain the standard file transaction.
func TestWorkbenchSQLiteURLAndLaterMigrationTransaction(t *testing.T) {
	repoRoot := sqliteRepoRoot(t)
	legacyRoot := copySQLiteMigrationsBeforeWorkbench(t, repoRoot)
	migrationRoot := copySQLiteMigrationsWithV17(t, repoRoot, `
CREATE TABLE workbench_v17_marker (id INTEGER PRIMARY KEY, value TEXT NOT NULL);
INSERT INTO workbench_v17_marker (id, value) VALUES (1, 'must-roll-back');
THIS IS NOT VALID SQL;
`)
	dbPath := filepath.Join(t.TempDir(), "transactional-v17.db")

	chdirAndRestore(t, legacyRoot)
	require.NoError(t, RunMigrations("sqlite3://"+dbPath))
	db := openSQLiteDB(t, dbPath)
	seedWorkbenchMigrationRows(t, db)

	chdirAndRestore(t, migrationRoot)
	err := RunMigrationsWithOptions("sqlite3://"+dbPath, MigrationOptions{SQLiteDBPath: dbPath})
	require.Error(t, err)
	assertWorkbenchChildSummary(t, db)
	var markerCount int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'workbench_v17_marker'").Scan(&markerCount))
	require.Zero(t, markerCount, "v17 must regain the default transaction wrapper after v16")
	version, dirty := sqliteMigrationState(t, db)
	require.Equal(t, 17, version)
	require.True(t, dirty)

	require.NoError(t, os.WriteFile(filepath.Join(migrationRoot, "migrations", "sqlite", "000017_workbench_v17.up.sql"), []byte(`
CREATE TABLE workbench_v17_marker (id INTEGER PRIMARY KEY, value TEXT NOT NULL);
INSERT INTO workbench_v17_marker (id, value) VALUES (1, 'recovered');
`), 0o600))
	require.NoError(t, RunMigrationsWithOptions("sqlite3://"+dbPath, MigrationOptions{
		AutoRecoverDirty: true,
		SQLiteDBPath:     dbPath,
	}))
	version, dirty = sqliteMigrationState(t, db)
	require.Equal(t, 17, version)
	require.False(t, dirty)
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM workbench_v17_marker WHERE value = 'recovered'").Scan(&markerCount))
	require.Equal(t, 1, markerCount)
}

func TestWorkbenchSQLiteURLUpgradeAndDownUpPreserveChildren(t *testing.T) {
	repoRoot := sqliteRepoRoot(t)
	t.Run("fresh", func(t *testing.T) {
		chdirAndRestore(t, repoRoot)
		dbPath := filepath.Join(t.TempDir(), "url-fresh.db")
		require.NoError(t, RunMigrations("sqlite3://"+dbPath))
		db := openSQLiteDB(t, dbPath)
		version, dirty := sqliteMigrationState(t, db)
		require.Equal(t, 16, version)
		require.False(t, dirty)
	})

	t.Run("legacy-upgrade-down-up", func(t *testing.T) {
		legacyRoot := copySQLiteMigrationsBeforeWorkbench(t, repoRoot)
		dbPath := filepath.Join(t.TempDir(), "url-upgrade.db")

		chdirAndRestore(t, legacyRoot)
		require.NoError(t, RunMigrations("sqlite3://"+dbPath))
		db := openSQLiteDB(t, dbPath)
		seedWorkbenchMigrationRows(t, db)

		chdirAndRestore(t, repoRoot)
		require.NoError(t, RunMigrations("sqlite3://"+dbPath))
		assertWorkbenchChildSummary(t, db)
		version, dirty := sqliteMigrationState(t, db)
		require.Equal(t, 16, version)
		require.False(t, dirty)

		require.NoError(t, runWorkbenchSQLiteMigrationSteps(repoRoot, dbPath, -1))
		version, dirty = sqliteMigrationState(t, db)
		require.Equal(t, 15, version)
		require.False(t, dirty)
		assertWorkbenchChildSummary(t, db)
		require.False(t, sqliteColumnExists(t, db, "agent_runs", "driver"))

		require.NoError(t, runWorkbenchSQLiteMigrationSteps(repoRoot, dbPath, 1))
		version, dirty = sqliteMigrationState(t, db)
		require.Equal(t, 16, version)
		require.False(t, dirty)
		assertWorkbenchChildSummary(t, db)
	})
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

	script, err := os.ReadFile(filepath.Join(repoRoot, "migrations", "sqlite", "000016_workbench_runs.down.sql"))
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
		if entry.IsDir() || strings.HasPrefix(entry.Name(), "000016_") {
			continue
		}
		contents, readErr := os.ReadFile(filepath.Join(srcDir, entry.Name()))
		require.NoError(t, readErr)
		require.NoError(t, os.WriteFile(filepath.Join(destDir, entry.Name()), contents, 0o600))
	}
	return dest
}

func copySQLiteMigrationsWithV17(t *testing.T, repoRoot, v17up string) string {
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
	require.NoError(t, os.WriteFile(filepath.Join(destDir, "000017_workbench_v17.up.sql"), []byte(v17up), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(destDir, "000017_workbench_v17.down.sql"), []byte("DROP TABLE workbench_v17_marker;\n"), 0o600))
	return dest
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
