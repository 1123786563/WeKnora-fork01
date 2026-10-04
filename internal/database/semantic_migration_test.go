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
	// 全量 up 的终态等于 migrations/sqlite 目录中的最新迁移号：多 lane
	// 合并持续追加迁移（semantic 三连 105-107、agent_versions 108、
	// tenant_agent_marketplace 109，其后 craft/workbench/docker journal
	// 等多族已入链），硬编码终态号会让本契约在每次新增迁移后误红
	// （曾长期钉在 109 而对 T02 收编轮造成假失败）。
	latest := latestSQLiteMigrationNumber(t, root)
	require.Equal(t, latest, version)
	require.False(t, dirty)
	for _, table := range append(append(semanticControlTables, semanticPolicyTables...), semanticInvocationTables...) {
		require.True(t, sqliteTableExists(t, db, table))
	}
	require.True(t, sqliteIndexExists(t, db, "idx_semantic_outbox_claim"))
	m, err := newSQLiteMigrator("file://"+filepath.Join(root, "migrations/sqlite"), path, "", true)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Close() })
	// 回滚到 semantic_model_invocations(107) 之下：迁移版本号稀疏（117/118
	// 等缺号），回退步数必须按 fixture 实际文件数计，不可用数值差。
	require.NoError(t, m.Steps(-sqliteMigrationStepsAfter(t, root, 106)))
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

// latestSQLiteMigrationNumber derives the expected full-up terminus from the
// migrations/sqlite directory itself, so the contract stays true as lanes
// keep appending numbered migrations.
func latestSQLiteMigrationNumber(t *testing.T, repoRoot string) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(repoRoot, "migrations", "sqlite"))
	require.NoError(t, err)
	latest := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		number, err := strconv.Atoi(strings.SplitN(name, "_", 2)[0])
		require.NoError(t, err, "migration file %s must start with its number", name)
		if number > latest {
			latest = number
		}
	}
	require.Positive(t, latest, "migrations/sqlite must contain at least one up migration")
	return latest
}
