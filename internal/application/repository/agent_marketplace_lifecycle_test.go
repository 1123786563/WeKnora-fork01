package repository

// T33 #63 lifecycle primitives + migration alignment tests.

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openLifecycleMigrationDB applies the REAL sqlite migration stream so the
// lifecycle columns are the production schema, not an AutoMigrate sketch.
func openLifecycleMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "lifecycle.db") + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	m, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, m.Up())
	_, _ = m.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestAgentMarketplaceLifecycleMigrationColumns(t *testing.T) {
	db := openLifecycleMigrationDB(t)
	expect := map[string][]string{
		"agent_marketplace_listings": {"unlisted_at", "unlisted_by"},
		"agent_releases":             {"deprecated_at", "deprecated_by", "successor_release_id"},
		"agent_adoptions":            {"ended_at", "ended_by"},
		"agent_adoption_variants":    {"retired_at", "retired_by"},
	}
	for table, columns := range expect {
		rows, err := db.Raw("SELECT name FROM pragma_table_info(?)", table).Rows()
		require.NoError(t, err)
		present := map[string]bool{}
		for rows.Next() {
			var name string
			require.NoError(t, rows.Scan(&name))
			present[name] = true
		}
		require.NoError(t, rows.Err())
		require.NoError(t, rows.Close())
		for _, column := range columns {
			require.Truef(t, present[column], "%s.%s 必须由迁移创建", table, column)
		}
	}
}
