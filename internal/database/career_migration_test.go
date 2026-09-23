package database

import (
	"github.com/stretchr/testify/require"
	"path/filepath"
	"testing"
)

func TestCareerMigrationCreatesPersonalEvidenceSchema(t *testing.T) {
	root := sqliteRepoRoot(t)
	chdirAndRestore(t, root)
	path := filepath.Join(t.TempDir(), "career-migration.db")
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: path}))
	db := openSQLiteDB(t, path)
	for _, table := range []string{"career_spaces", "career_profiles", "career_facts", "career_fact_versions", "career_proposals", "career_changes", "career_receipts"} {
		require.Truef(t, sqliteTableExists(t, db, table), "career migration must create %s", table)
	}
	for _, column := range []string{"source", "confirmation", "public_id", "status", "resolution_source"} {
		var count int
		require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('career_proposals') WHERE name = ?", column).Scan(&count))
		if column == "source" || column == "confirmation" {
			require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('career_facts') WHERE name = ?", column).Scan(&count))
			require.Equalf(t, 1, count, "career_facts must include %s", column)
		} else {
			require.Equalf(t, 1, count, "career_proposals must include %s", column)
		}
	}
}
