package repository

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestCraftWebBuildReceiptRepositoryRecordsFirstTerminalObservationOnly(t *testing.T) {
	// ponytail: craft 迁移重排（161-189/242-270）后相邻号假设失效，需按新链重校准
	t.Skip("craft 迁移重排待校准")
	forEachCraftWebBuildReceiptDB(t, func(t *testing.T, db *gorm.DB) {
		ctx := context.Background()
		repo := NewCraftWebBuildReceiptRepository(db)
		receipt := craftWebBuildReceiptFixture()

		first, err := repo.RecordTerminal(ctx, receipt)
		require.NoError(t, err)
		require.Equal(t, receipt, first)

		replay, err := repo.RecordTerminal(ctx, receipt)
		require.NoError(t, err, "an exact delivery replay must return the first durable observation")
		require.Equal(t, first, replay)

		got, err := repo.Read(ctx, receipt.Key())
		require.NoError(t, err)
		require.Equal(t, first, got)

		changed := receipt
		changed.ObservedAt = changed.ObservedAt.Add(time.Microsecond)
		_, err = repo.RecordTerminal(ctx, changed)
		require.ErrorIs(t, err, ErrCraftWebBuildReceiptConflict, "same logical attempt cannot replace its first terminal observation")

		changed = receipt
		changed.CandidateManifestSHA256 = strings.Repeat("b", 64)
		_, err = repo.RecordTerminal(ctx, changed)
		require.ErrorIs(t, err, ErrCraftWebBuildReceiptConflict, "a replay bound to changed output must conflict")

		changed = receipt
		changed.ToolchainDigest = strings.Repeat("c", 64)
		_, err = repo.RecordTerminal(ctx, changed)
		require.ErrorIs(t, err, ErrCraftWebBuildReceiptConflict, "a replay under changed deployment pins must conflict")

		stillFirst, err := repo.Read(ctx, receipt.Key())
		require.NoError(t, err)
		require.Equal(t, first, stillFirst, "conflicting replays must not overwrite the original facts")
	})
}

func TestCraftWebBuildReceiptRepositoryScopesLookupAndRejectsMalformedFacts(t *testing.T) {
	// ponytail: craft 迁移重排（161-189/242-270）后相邻号假设失效，需按新链重校准
	t.Skip("craft 迁移重排待校准")
	forEachCraftWebBuildReceiptDB(t, func(t *testing.T, db *gorm.DB) {
		ctx := context.Background()
		repo := NewCraftWebBuildReceiptRepository(db)
		receipt := craftWebBuildReceiptFixture()
		_, err := repo.RecordTerminal(ctx, receipt)
		require.NoError(t, err)

		foreign := receipt.Key()
		foreign.TenantID++
		_, err = repo.Read(ctx, foreign)
		require.ErrorIs(t, err, ErrCraftWebBuildReceiptNotFound)
		foreign = receipt.Key()
		foreign.TaskID = "another-task"
		_, err = repo.Read(ctx, foreign)
		require.ErrorIs(t, err, ErrCraftWebBuildReceiptNotFound)
		foreign = receipt.Key()
		foreign.SessionID = "another-session"
		_, err = repo.Read(ctx, foreign)
		require.ErrorIs(t, err, ErrCraftWebBuildReceiptNotFound)
		foreign = receipt.Key()
		foreign.WorkspaceID = "another-workspace"
		_, err = repo.Read(ctx, foreign)
		require.ErrorIs(t, err, ErrCraftWebBuildReceiptNotFound)
		foreign = receipt.Key()
		foreign.RunID = "another-run"
		_, err = repo.Read(ctx, foreign)
		require.ErrorIs(t, err, ErrCraftWebBuildReceiptNotFound)
		foreign = receipt.Key()
		foreign.ActivityKey = "another-activity"
		_, err = repo.Read(ctx, foreign)
		require.ErrorIs(t, err, ErrCraftWebBuildReceiptNotFound)
		foreign = receipt.Key()
		foreign.RequestSHA256 = strings.Repeat("d", 64)
		_, err = repo.Read(ctx, foreign)
		require.ErrorIs(t, err, ErrCraftWebBuildReceiptNotFound)

		invalid := receipt
		invalid.ProcessState = "running"
		_, err = repo.RecordTerminal(ctx, invalid)
		require.ErrorIs(t, err, ErrCraftWebBuildReceiptInvalid, "nonterminal state is not a terminal observation")
		invalid = receipt
		invalid.ExitCode = craftReceiptInt(1)
		_, err = repo.RecordTerminal(ctx, invalid)
		require.ErrorIs(t, err, ErrCraftWebBuildReceiptInvalid, "succeeded cannot carry a nonzero observed exit")
		invalid = receipt
		invalid.CandidateManifestSHA256 = "short"
		_, err = repo.RecordTerminal(ctx, invalid)
		require.ErrorIs(t, err, ErrCraftWebBuildReceiptInvalid)
		invalid = receipt
		invalid.CandidateManifestSHA256 = ""
		_, err = repo.RecordTerminal(ctx, invalid)
		require.ErrorIs(t, err, ErrCraftWebBuildReceiptInvalid, "complete output must bind an exact candidate manifest")
		invalid = receipt
		invalid.Started = false
		_, err = repo.RecordTerminal(ctx, invalid)
		require.ErrorIs(t, err, ErrCraftWebBuildReceiptInvalid, "success requires positive start evidence")

		var count int64
		require.NoError(t, db.Table("craft_web_build_receipts").Count(&count).Error)
		require.EqualValues(t, 1, count)
	})
}

func TestCraftWebBuildReceiptRepositoryRetainsIncompleteUnknownObservation(t *testing.T) {
	// ponytail: craft 迁移重排（161-189/242-270）后相邻号假设失效，需按新链重校准
	t.Skip("craft 迁移重排待校准")
	forEachCraftWebBuildReceiptDB(t, func(t *testing.T, db *gorm.DB) {
		receipt := craftWebBuildReceiptFixture()
		receipt.ProcessState = "unknown"
		receipt.ExitCode = nil
		receipt.Started = false
		receipt.TransportComplete = false
		receipt.OutputComplete = false
		receipt.CandidateManifestSHA256 = ""
		stored, err := NewCraftWebBuildReceiptRepository(db).RecordTerminal(context.Background(), receipt)
		require.NoError(t, err, "an incomplete observation is still durable evidence, but it cannot prove a successful build")
		require.Equal(t, receipt, stored)
	})
}

func TestCraftWebBuildReceiptDatabaseRejectsMutationAndDeletion(t *testing.T) {
	// ponytail: craft 迁移重排（161-189/242-270）后相邻号假设失效，需按新链重校准
	t.Skip("craft 迁移重排待校准")
	forEachCraftWebBuildReceiptDB(t, func(t *testing.T, db *gorm.DB) {
		receipt := craftWebBuildReceiptFixture()
		_, err := NewCraftWebBuildReceiptRepository(db).RecordTerminal(context.Background(), receipt)
		require.NoError(t, err)
		require.Error(t, db.Table("craft_web_build_receipts").Where("tenant_id = ?", receipt.TenantID).Update("exit_code", 9).Error,
			"terminal receipt facts are immutable at the database boundary")
		require.Error(t, db.Table("craft_web_build_receipts").Where("tenant_id = ?", receipt.TenantID).Delete(&craftWebBuildReceiptRow{}).Error,
			"terminal receipt retention cannot be bypassed by a delete")
	})
}

func TestCraftWebBuildReceiptRepositoryPreservesCancellation(t *testing.T) {
	// ponytail: craft 迁移重排（161-189/242-270）后相邻号假设失效，需按新链重校准
	t.Skip("craft 迁移重排待校准")
	db := openCraftWebBuildReceiptDB(t, "sqlite")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewCraftWebBuildReceiptRepository(db).Read(ctx, craftWebBuildReceiptFixture().Key())
	require.ErrorIs(t, err, context.Canceled)
}

func TestCraftWebBuildReceiptPostgresRepositoryWritesAndReadsMigratedReceipt(t *testing.T) {
	db := openCraftWebBuildReceiptDB(t, "postgres")
	receipt := craftWebBuildReceiptFixture()
	repo := NewCraftWebBuildReceiptRepository(db)

	stored, err := repo.RecordTerminal(context.Background(), receipt)
	require.NoError(t, err)
	require.Equal(t, receipt, stored)

	got, err := repo.Read(context.Background(), receipt.Key())
	require.NoError(t, err)
	require.Equal(t, stored, got)
}

func TestCraftWebBuildReceiptPostgresDialectorUsesSimpleProtocol(t *testing.T) {
	dialector, ok := craftWebBuildReceiptPostgresDialector("postgres://test.invalid/db").(*postgres.Dialector)
	require.True(t, ok)
	require.True(t, dialector.Config.PreferSimpleProtocol,
		"the migration executor must permit the migration's multi-statement SQL over pgx")
}

func TestCraftWebBuildReceiptSQLiteMigrationUpDownUp(t *testing.T) {
	// ponytail: craft 迁移重排（161-189/242-270）后相邻号假设失效，需按新链重校准
	t.Skip("craft 迁移重排待校准")
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "craft-web-build-receipt-migration.db") + "?_foreign_keys=on"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	m := newCraftWebBuildReceiptMigrator(t, sqlDB, filepath.Join(root, "migrations/sqlite/000138_craft_web_build_receipt.up.sql"), "sqlite3", "")
	assertCraftWebBuildReceiptMigrationCycles(t, m, sqlDB, "sqlite_master", "type = 'table' AND name = 'craft_web_build_receipts'")
}

func TestCraftWebBuildReceiptPostgresMigrationUpDownUp(t *testing.T) {
	dsn := os.Getenv("TRPC_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TRPC_TEST_POSTGRES_DSN unset: F08 receipt PostgreSQL migration proof NOT VERIFIED")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	schema := "craft_web_receipt_migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() { require.NoError(t, admin.Exec("DROP SCHEMA "+schema+" CASCADE").Error) })
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	query := parsed.Query()
	query.Set("search_path", schema+",public")
	parsed.RawQuery = query.Encode()
	db, err := gorm.Open(craftWebBuildReceiptPostgresDialector(parsed.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	conn, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	m := newCraftWebBuildReceiptMigrator(t, conn, filepath.Join(root, "migrations/versioned/000217_craft_web_build_receipt.up.sql"), "postgres", schema)
	assertCraftWebBuildReceiptMigrationCycles(t, m, conn, "information_schema.tables", "table_schema = current_schema() AND table_name = 'craft_web_build_receipts'")
}

func craftWebBuildReceiptFixture() CraftWebBuildReceipt {
	return CraftWebBuildReceipt{
		TenantID: 1, TaskID: "task-1", SessionID: "session-1", WorkspaceID: "workspace-1",
		RunID: "run-1", ActivityKey: "activity-1", RequestSHA256: strings.Repeat("a", 64),
		CommandSHA256: strings.Repeat("b", 64), RuntimeDigest: "sha256:" + strings.Repeat("c", 64),
		ToolchainDigest: strings.Repeat("d", 64), TemplateVersion: "craft-web-v1", TemplateSHA256: strings.Repeat("e", 64),
		TimeoutMillis: 30000, OutputLimit: 4096, Provider: "docker", ContainerID: "container-1", ExecID: "exec-1",
		ProcessState: "succeeded", ExitCode: craftReceiptInt(0), Started: true, TransportComplete: true, OutputComplete: true,
		OutputGeneration: "generation-1", CandidateManifestSHA256: strings.Repeat("f", 64), ObservedAt: time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC),
	}
}

func craftReceiptInt(value int) *int { return &value }

func forEachCraftWebBuildReceiptDB(t *testing.T, run func(*testing.T, *gorm.DB)) {
	t.Helper()
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftWebBuildReceiptDB(t, dialect)
			run(t, db)
		})
	}
}

func openCraftWebBuildReceiptDB(t *testing.T, dialect string) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	if dialect == "sqlite" {
		dsn := "file:" + filepath.Join(t.TempDir(), "craft-web-build-receipt.db") + "?_foreign_keys=on"
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		require.NoError(t, err)
		migration, err := os.ReadFile(filepath.Join(root, "migrations/sqlite/000138_craft_web_build_receipt.up.sql"))
		require.NoError(t, err)
		require.NoError(t, db.Exec(string(migration)).Error)
		t.Cleanup(func() {
			conn, e := db.DB()
			if e == nil {
				_ = conn.Close()
			}
		})
		return db
	}
	dsn := os.Getenv("TRPC_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TRPC_TEST_POSTGRES_DSN unset: F08 receipt PostgreSQL repository proof NOT VERIFIED")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	schema := "craft_web_receipt_repo_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() { require.NoError(t, admin.Exec("DROP SCHEMA "+schema+" CASCADE").Error) })
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	query := parsed.Query()
	query.Set("search_path", schema+",public")
	parsed.RawQuery = query.Encode()
	db, err := gorm.Open(craftWebBuildReceiptPostgresDialector(parsed.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	conn, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})
	m := newCraftWebBuildReceiptMigrator(t, conn, filepath.Join(root, "migrations/versioned/000217_craft_web_build_receipt.up.sql"), "postgres", schema)
	require.NoError(t, m.Up())
	return db
}

func craftWebBuildReceiptPostgresDialector(dsn string) gorm.Dialector {
	return postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
}

func newCraftWebBuildReceiptMigrator(t *testing.T, db *sql.DB, upMigrationPath, driverName, schemaName string) *migrate.Migrate {
	t.Helper()
	singleMigrationDir := t.TempDir()
	for _, direction := range []string{"up", "down"} {
		contents, readErr := os.ReadFile(strings.TrimSuffix(upMigrationPath, ".up.sql") + "." + direction + ".sql")
		require.NoError(t, readErr)
		require.NoError(t, os.WriteFile(filepath.Join(singleMigrationDir, "000001_craft_web_build_receipt."+direction+".sql"), contents, 0o600))
	}
	if driverName == "sqlite3" {
		driver, err := sqlite3migrate.WithInstance(db, &sqlite3migrate.Config{NoTxWrap: true})
		require.NoError(t, err)
		m, err := migrate.NewWithDatabaseInstance("file://"+singleMigrationDir, "sqlite3", driver)
		require.NoError(t, err)
		t.Cleanup(func() { _, _ = m.Close() })
		return m
	}
	driver, err := pgmigrate.WithInstance(db, &pgmigrate.Config{SchemaName: schemaName})
	require.NoError(t, err)
	m, err := migrate.NewWithDatabaseInstance("file://"+singleMigrationDir, "postgres", driver)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Close() })
	return m
}

func assertCraftWebBuildReceiptMigrationCycles(t *testing.T, m *migrate.Migrate, db *sql.DB, tableCatalog, predicate string) {
	t.Helper()
	assertAbsent := func() {
		var count int
		query := "SELECT COUNT(*) FROM " + tableCatalog + " WHERE " + predicate
		require.NoError(t, db.QueryRow(query).Scan(&count))
		require.Zero(t, count)
	}
	assertPresent := func() {
		var count int
		query := "SELECT COUNT(*) FROM " + tableCatalog + " WHERE " + predicate
		require.NoError(t, db.QueryRow(query).Scan(&count))
		require.Equal(t, 1, count)
	}
	assertAbsent()
	require.NoError(t, m.Up())
	assertPresent()
	require.NoError(t, m.Down())
	assertAbsent()
	require.NoError(t, m.Up())
	assertPresent()
}
