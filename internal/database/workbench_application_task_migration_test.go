package database

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestApplicationTaskMigrationCreatesAndRemovesProjectionSchema(t *testing.T) {
	repoRoot := sqliteRepoRoot(t)
	chdirAndRestore(t, repoRoot)
	dbPath := filepath.Join(t.TempDir(), "workbench-application-task.db")
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: dbPath}))
	db := openSQLiteDB(t, dbPath)
	require.True(t, sqliteTableExists(t, db, "workbench_application_tasks"))

	var columns []string
	rows, err := db.Query("SELECT name FROM pragma_table_info('workbench_application_tasks') ORDER BY cid")
	require.NoError(t, err)
	for rows.Next() {
		var column string
		require.NoError(t, rows.Scan(&column))
		columns = append(columns, column)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{
		"tenant_id", "owner_id", "origin", "origin_request_id", "application_id",
		"task_id", "run_id", "title", "created_at", "updated_at",
	}, columns)

	type indexDefinition struct {
		Name string
		SQL  string
	}
	var indexes []indexDefinition
	gormDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gormDB.Table("sqlite_master").
		Select("name, sql").
		Where("type = ? AND name IN ?", "index", []string{
			"uq_workbench_application_tasks_request",
			"uq_workbench_application_tasks_application",
		}).
		Find(&indexes).Error)
	require.Len(t, indexes, 2)
	for _, index := range indexes {
		require.Contains(t, index.SQL, "CREATE UNIQUE INDEX")
	}

	versioned, err := os.ReadFile(filepath.Join(repoRoot, "migrations", "versioned", "000208_workbench_application_tasks.up.sql"))
	require.NoError(t, err)
	versionedSQL := string(versioned)
	require.Contains(t, versionedSQL, "uq_workbench_application_tasks_request")
	require.Contains(t, versionedSQL, "uq_workbench_application_tasks_application")

	m, err := newSQLiteMigrator("file://"+filepath.Join(repoRoot, "migrations/sqlite"), dbPath, "", true)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Close() })
	// Down to the explicit pre-projection version instead of a step count:
	// parallel career migrations leave gaps (118 reserved), so one step down
	// from the terminal version is baseline-dependent.
	require.NoError(t, m.Migrate(116))
	version, dirty := sqliteMigrationState(t, db)
	require.Equal(t, 116, version)
	require.False(t, dirty)
	require.False(t, sqliteTableExists(t, db, "workbench_application_tasks"))
}
