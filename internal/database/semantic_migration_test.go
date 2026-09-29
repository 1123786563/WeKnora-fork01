package database

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSemanticMigrationSQLiteUpDownUp(t *testing.T) {
	semanticControlTables := []string{"semantic_document_revisions", "semantic_outbox", "semantic_access_epochs", "semantic_denials"}
	semanticPolicyTables := []string{"semantic_model_policies", "semantic_model_policy_revisions"}
	semanticInvocationTables := []string{"semantic_model_invocation_runs", "semantic_model_invocations"}
	root := sqliteRepoRoot(t)
	chdirAndRestore(t, root)
	path := filepath.Join(t.TempDir(), "semantic-migration.db")
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: path}))
	db := openSQLiteDB(t, path)
	version, dirty := sqliteMigrationState(t, db)
	latest := latestSQLiteMigrationVersion(t, filepath.Join(root, "migrations/sqlite"))
	require.Equal(t, latest, version)
	require.False(t, dirty)
	for _, table := range append(append(semanticControlTables, semanticPolicyTables...), semanticInvocationTables...) {
		require.True(t, sqliteTableExists(t, db, table))
	}
	require.True(t, sqliteIndexExists(t, db, "idx_semantic_outbox_claim"))
	m, err := newSQLiteMigrator("file://"+filepath.Join(root, "migrations/sqlite"), path, "", true)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Close() })
	// Return to version 106, immediately before semantic_model_invocations(107).
	require.NoError(t, m.Migrate(106))
	version, dirty = sqliteMigrationState(t, db)
	require.Equal(t, 106, version)
	require.False(t, dirty)
	for _, table := range semanticInvocationTables {
		require.False(t, sqliteTableExists(t, db, table), "down migration must remove %s", table)
	}
	for _, table := range append(semanticControlTables, semanticPolicyTables...) {
		require.True(t, sqliteTableExists(t, db, table), "down migration must preserve prior %s", table)
	}
	require.True(t, sqliteIndexExists(t, db, "idx_semantic_outbox_claim"), "down migration must preserve prior index")
	require.NoError(t, m.Up())
	version, dirty = sqliteMigrationState(t, db)
	require.Equal(t, latest, version)
	require.False(t, dirty)
	for _, table := range append(append(semanticControlTables, semanticPolicyTables...), semanticInvocationTables...) {
		require.True(t, sqliteTableExists(t, db, table), "up migration must restore %s", table)
	}
	require.True(t, sqliteIndexExists(t, db, "idx_semantic_outbox_claim"))
}

func latestSQLiteMigrationVersion(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	latest := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		versionText, _, ok := strings.Cut(name, "_")
		if !ok {
			continue
		}
		version, parseErr := strconv.Atoi(versionText)
		require.NoError(t, parseErr)
		if version > latest {
			latest = version
		}
	}
	return latest
}
