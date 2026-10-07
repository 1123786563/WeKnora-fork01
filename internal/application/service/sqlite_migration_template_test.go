package service

import (
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// The SQLite migration chain (~190 files, one fsync-bearing transaction each)
// costs 5-9s per fresh database file under parallel `go test` load, and this
// package opens ~70 such databases across its test helpers. Migrating every
// one individually pushed the whole package past `go test`'s default 10m
// per-binary timeout (observed 605s+ under gate load, timing out mid
// migration in sqlite3_step). sqliteMigrationTemplate runs the real chain
// exactly once per test binary into a template file; every helper then
// clones that file, so each test still owns a fully isolated database with
// the byte-identical migrated schema (schema_migrations bookkeeping
// included) at file-copy cost.
var sqliteMigrationTemplate struct {
	once sync.Once
	path string
	err  error
}

// sqliteMigrationTemplatePath lazily builds (once per test binary) the fully
// migrated template database and returns its path. Failures are recorded and
// surfaced through require on every caller so sync.Once state stays sound.
func sqliteMigrationTemplatePath(t *testing.T) string {
	t.Helper()
	sqliteMigrationTemplate.once.Do(func() {
		_, filename, _, ok := runtime.Caller(0)
		if !ok {
			sqliteMigrationTemplate.err = errors.New("cannot locate test source file for repo root")
			return
		}
		repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
		dir, err := os.MkdirTemp("", "weknora-sqlite-template-")
		if err != nil {
			sqliteMigrationTemplate.err = err
			return
		}
		templatePath := filepath.Join(dir, "template.db")
		dsn := "file:" + templatePath + "?_foreign_keys=on&_busy_timeout=5000"
		sqlDB, err := sql.Open("sqlite3", dsn)
		if err != nil {
			sqliteMigrationTemplate.err = err
			return
		}
		driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
		if err != nil {
			_ = sqlDB.Close()
			sqliteMigrationTemplate.err = err
			return
		}
		migrator, err := migrate.NewWithDatabaseInstance(
			"file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver,
		)
		if err != nil {
			_ = sqlDB.Close()
			sqliteMigrationTemplate.err = err
			return
		}
		err = migrator.Up()
		// migrator.Close also closes the underlying *sql.DB, flushing and
		// releasing the template file so later clones copy a clean image.
		_, _ = migrator.Close()
		if err != nil && !errors.Is(err, migrate.ErrNoChange) {
			sqliteMigrationTemplate.err = err
			return
		}
		sqliteMigrationTemplate.path = templatePath
	})
	require.NoError(t, sqliteMigrationTemplate.err)
	require.NotEmpty(t, sqliteMigrationTemplate.path, "sqlite migration template path not set")
	return sqliteMigrationTemplate.path
}

// cloneMigratedSQLiteDB copies the once-migrated template into the test's
// private temp dir and opens it with gorm using the same DSN options the
// per-helper originals used (foreign keys on, 5s busy timeout). The returned
// database is closed automatically when the test finishes.
func cloneMigratedSQLiteDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	src, err := os.Open(sqliteMigrationTemplatePath(t))
	require.NoError(t, err)
	dbPath := filepath.Join(t.TempDir(), name)
	dst, err := os.Create(dbPath)
	require.NoError(t, err)
	_, copyErr := io.Copy(dst, src)
	dstErr := dst.Close()
	srcErr := src.Close()
	require.NoError(t, copyErr)
	require.NoError(t, dstErr)
	require.NoError(t, srcErr)
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"
	db, err := gorm.Open(gormsqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		conn, e := db.DB()
		if e == nil {
			_ = conn.Close()
		}
	})
	return db
}
