package database

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSemanticMigrationSQLiteUpDownUp(t *testing.T) {
	semanticTables := []string{"semantic_document_revisions", "semantic_outbox", "semantic_access_epochs", "semantic_denials"}
	root := sqliteRepoRoot(t)
	chdirAndRestore(t, root)
	path := filepath.Join(t.TempDir(), "semantic-migration.db")
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: path}))
	db := openSQLiteDB(t, path)
	version, dirty := sqliteMigrationState(t, db)
	require.Equal(t, 99, version)
	require.False(t, dirty)
	for _, table := range semanticTables {
		require.True(t, sqliteTableExists(t, db, table))
	}
	require.True(t, sqliteIndexExists(t, db, "idx_semantic_outbox_claim"))
	m, err := newSQLiteMigrator("file://"+filepath.Join(root, "migrations/sqlite"), path, "", true)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Close() })
	require.NoError(t, m.Steps(-1))
	for _, table := range semanticTables {
		require.False(t, sqliteTableExists(t, db, table), "down migration must remove %s", table)
	}
	require.False(t, sqliteIndexExists(t, db, "idx_semantic_outbox_claim"))
	require.NoError(t, m.Up())
	version, dirty = sqliteMigrationState(t, db)
	require.Equal(t, 99, version)
	require.False(t, dirty)
	for _, table := range semanticTables {
		require.True(t, sqliteTableExists(t, db, table), "up migration must restore %s", table)
	}
	require.True(t, sqliteIndexExists(t, db, "idx_semantic_outbox_claim"))
}
