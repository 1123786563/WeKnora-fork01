package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

var semanticTables = []string{
	"semantic_document_revisions",
	"semantic_outbox",
	"semantic_access_epochs",
	"semantic_denials",
	"semantic_backend_states",
	"semantic_completion_receipts",
}

func tableExists(t *testing.T, db *sql.DB, dialect, table string) bool {
	t.Helper()
	var count int
	var err error
	if dialect == "postgres" {
		err = db.QueryRow(
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1", table,
		).Scan(&count)
	} else {
		err = db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table,
		).Scan(&count)
	}
	require.NoError(t, err)
	return count == 1
}

func requireSemanticTables(t *testing.T, db *sql.DB, dialect string, present bool) {
	t.Helper()
	for _, table := range semanticTables {
		require.Equal(t, present, tableExists(t, db, dialect, table), "table %s presence mismatch", table)
	}
}

func TestSemanticMigrationSQLite(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)

	dbPath := filepath.Join(t.TempDir(), "semantic-migration.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance(
		"file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver,
	)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = migrator.Close() })

	require.NoError(t, migrator.Up())
	requireSemanticTables(t, sqlDB, "sqlite", true)

	// Down removes exactly the semantic tables; Up again reinstates them.
	require.NoError(t, migrator.Steps(-1))
	requireSemanticTables(t, sqlDB, "sqlite", false)
	require.NoError(t, migrator.Up())
	requireSemanticTables(t, sqlDB, "sqlite", true)
}

func dropAllPublicTables(t *testing.T, db *sql.DB, dsn string) {
	t.Helper()
	// Guard: only databases whose NAME marks them as test-only. Parse the
	// DSN so "test" in a host/user/query cannot sneak a production DB past.
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	databaseName := strings.Trim(parsed.Path, "/")
	require.NotEmpty(t, databaseName, "DSN must name a database")
	require.Contains(t, databaseName, "test",
		"refusing to drop tables of non-test database %q", databaseName)
	rows, err := db.Query(
		"SELECT tablename FROM pg_tables WHERE schemaname = 'public'")
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var tables []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		tables = append(tables, name)
	}
	require.NoError(t, rows.Err())
	for _, name := range tables {
		_, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %q CASCADE", name))
		require.NoError(t, err, "drop %s", name)
	}
}

func TestSemanticMigrationPostgres(t *testing.T) {
	dsn := os.Getenv("SEMANTIC_TEST_BUSINESS_PG_DSN")
	if dsn == "" {
		dsn = "postgres://semantic:semantic@127.0.0.1:15432/semantic_business_test?sslmode=disable" // isolated container; the DB sets app.skip_embedding=true (extension-free path)
	}
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)

	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open isolated business PG: %v (start the semantic test container; see progress.md I02 record)", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("isolated business PG unreachable: %v (start the semantic test container; see progress.md I02 record)", err)
	}

	// Clean slate regardless of prior state (dirty flags, partial runs):
	// this is a dedicated test database, so dropping every table - including
	// schema_migrations - is safe. NOTE: the migrate postgres driver creates
	// its version table at construction, so the drop MUST run before the
	// migrator is built; Up then runs cleanly from version zero.
	dropAllPublicTables(t, sqlDB, dsn)

	driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance(
		"file://"+filepath.Join(repoRoot, "migrations/versioned"), "postgres", driver,
	)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = migrator.Close() })

	require.NoError(t, migrator.Up())
	requireSemanticTables(t, sqlDB, "postgres", true)

	// Unique keys: the outbox event identity and document revision identity.
	_, err = sqlDB.Exec(
		"INSERT INTO semantic_document_revisions (tenant_id, kb_id, document_id, revision) VALUES (1, 'kb', 'doc', 1)")
	require.NoError(t, err)
	_, err = sqlDB.Exec(
		"INSERT INTO semantic_document_revisions (tenant_id, kb_id, document_id, revision) VALUES (1, 'kb', 'doc', 2)")
	require.Error(t, err, "document revision primary key must reject duplicates")
	_, err = sqlDB.Exec(
		"INSERT INTO semantic_outbox (tenant_id, kb_id, document_id, revision, event_id, payload_hash) VALUES (1, 'kb', 'doc', 1, 'ev-1', 'h')")
	require.NoError(t, err)
	_, err = sqlDB.Exec(
		"INSERT INTO semantic_outbox (tenant_id, kb_id, document_id, revision, event_id, payload_hash) VALUES (1, 'kb', 'doc', 1, 'ev-2', 'h')")
	require.Error(t, err, "outbox identity unique key must reject duplicates")

	// Transaction failure leaves no orphan events.
	tx, err := sqlDB.Begin()
	require.NoError(t, err)
	_, err = tx.Exec(
		"INSERT INTO semantic_outbox (tenant_id, kb_id, document_id, revision, event_id, payload_hash) VALUES (1, 'kb', 'doc', 2, 'ev-3', 'h')")
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	var count int
	require.NoError(t, sqlDB.QueryRow("SELECT COUNT(*) FROM semantic_outbox WHERE event_id = 'ev-3'").Scan(&count))
	require.Zero(t, count, "rolled-back transaction must not leave outbox events")

	// Down drops the semantic tables (only), Up restores them.
	require.NoError(t, migrator.Steps(-1))
	requireSemanticTables(t, sqlDB, "postgres", false)
	require.NoError(t, migrator.Up())
	requireSemanticTables(t, sqlDB, "postgres", true)
}
