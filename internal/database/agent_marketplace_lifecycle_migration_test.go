package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

var agentMarketplaceLifecycleColumns = map[string][]string{
	"agent_adoptions":             {"ended_by", "ended_at", "end_reason"},
	"agent_adoption_variants":     {"retired_by", "retired_at", "retirement_reason"},
	"agent_marketplace_listings":  {"unlisted_by", "unlisted_at", "unlist_reason"},
	"agent_releases":              {"deprecated_by", "deprecated_at", "deprecation_reason", "replacement_release_id"},
	"public_marketplace_listings": {"unlisted_by", "unlisted_at", "unlist_reason"},
	"public_agent_releases":       {"deprecated_by", "deprecated_at", "deprecation_reason", "replacement_release_id"},
}

func TestAgentMarketplaceLifecycleMigrationSQLiteUpDownUp(t *testing.T) {
	root := sqliteRepoRoot(t)
	chdirAndRestore(t, root)
	dbPath := filepath.Join(t.TempDir(), "agent-lifecycle-migration.db")
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: dbPath}))
	db := openSQLiteDB(t, dbPath)
	assertAgentMarketplaceLifecycleColumns(t, db, true)

	sqliteUp, err := os.ReadFile(filepath.Join(root, "migrations/sqlite/000124_agent_marketplace_lifecycle.up.sql"))
	require.NoError(t, err)
	versionedUp, err := os.ReadFile(filepath.Join(root, "migrations/versioned/000203_agent_marketplace_lifecycle.up.sql"))
	require.NoError(t, err)
	for table, columns := range agentMarketplaceLifecycleColumns {
		for _, column := range columns {
			require.Containsf(t, string(sqliteUp), "ALTER TABLE "+table+" ADD COLUMN "+column, "SQLite migration missing %s.%s", table, column)
			require.Containsf(t, string(versionedUp), "ALTER TABLE "+table+" ADD COLUMN "+column, "versioned migration missing %s.%s", table, column)
		}
	}

	m, err := newSQLiteMigrator("file://"+filepath.Join(root, "migrations/sqlite"), dbPath, "", true)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Close() })
	require.NoError(t, m.Steps(-sqliteMigrationStepsAfter(t, root, 123)))
	assertAgentMarketplaceLifecycleColumns(t, db, false)
	require.NoError(t, m.Up())
	assertAgentMarketplaceLifecycleColumns(t, db, true)
}

func assertAgentMarketplaceLifecycleColumns(t *testing.T, db *sql.DB, expected bool) {
	t.Helper()
	for table, columns := range agentMarketplaceLifecycleColumns {
		for _, column := range columns {
			var found int
			row := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?", table, column)
			require.NoError(t, row.Scan(&found))
			require.Equalf(t, expected, found == 1, "%s.%s presence", table, column)
		}
	}
}
