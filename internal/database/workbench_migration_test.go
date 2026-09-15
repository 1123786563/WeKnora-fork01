package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	for _, table := range []string{
		"agent_run_checkpoints", "agent_tool_calls", "agent_tool_attempts",
		"agent_run_events", "agent_run_decisions", "agent_run_inputs",
	} {
		var count int
		require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE tenant_id = 1 AND run_id = 'legacy-run'").Scan(&count))
		require.Equalf(t, 1, count, "%s row must survive the agent_runs rebuild", table)
	}
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
