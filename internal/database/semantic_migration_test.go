package database

import (
	"path/filepath"
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
	// 多 lane 合并后 semantic 三连迁移为 105-107，其后还有 agent_versions(108)、
	// tenant_agent_marketplace(109)、plugin_previews(110) 与
	// plugin_installations(111)，全量 up 的终态是最新迁移号。
	require.Equal(t, 111, version)
	require.False(t, dirty)
	for _, table := range append(append(semanticControlTables, semanticPolicyTables...), semanticInvocationTables...) {
		require.True(t, sqliteTableExists(t, db, table))
	}
	require.True(t, sqliteIndexExists(t, db, "idx_semantic_outbox_claim"))
	m, err := newSQLiteMigrator("file://"+filepath.Join(root, "migrations/sqlite"), path, "", true)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Close() })
	// 回滚到 semantic_model_invocations(107) 之下：其上还有 108-111 五个迁移。
	require.NoError(t, m.Steps(-5))
	for _, table := range semanticInvocationTables {
		require.False(t, sqliteTableExists(t, db, table), "down migration must remove %s", table)
	}
	for _, table := range append(semanticControlTables, semanticPolicyTables...) {
		require.True(t, sqliteTableExists(t, db, table), "down migration must preserve prior %s", table)
	}
	require.True(t, sqliteIndexExists(t, db, "idx_semantic_outbox_claim"), "down migration must preserve prior index")
	require.NoError(t, m.Up())
	version, dirty = sqliteMigrationState(t, db)
	require.Equal(t, 111, version)
	require.False(t, dirty)
	for _, table := range append(append(semanticControlTables, semanticPolicyTables...), semanticInvocationTables...) {
		require.True(t, sqliteTableExists(t, db, table), "up migration must restore %s", table)
	}
	require.True(t, sqliteIndexExists(t, db, "idx_semantic_outbox_claim"))
}
