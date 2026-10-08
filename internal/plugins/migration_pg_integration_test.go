//go:build integration

// T06 (plan 03 Task 6 Step 6): the PostgreSQL migration contract of the
// plugin install slice, executed against REAL PostgreSQL.
//
// Environment contract (same convention as oc_integration_test.go):
//
//	PLUGIN_TEST_DATABASE_URL  PostgreSQL DSN of a DISPOSABLE database.
//	                          Missing → the test FAILS with "blocked-env" —
//	                          it never Skip-passes.
//
// The suite provisions an isolated per-run schema, materializes the MINIMAL
// parent tables, then executes the REAL migration files 000189.up and
// 000190.up and asserts:
//
//   - plugin_previews / plugin_installations exist;
//   - plugin_installations.manifest_url exists and is NOT NULL — it is the
//     long-lived upgrade source T14/T16 re-fetch from, frozen once by 000190
//     and never re-altered by later tasks;
//   - mcp_services.plugin_installation_id exists.
//
// It then seeds one MANUAL mcp_services row (plugin_installation_id NULL)
// plus one plugin-materialized row with derived approvals / oauth_tokens /
// oauth_clients / metadata rows on BOTH services, executes 000190.down and
// 000189.down, and asserts the rollback safety contract (GAP-3): every
// plugin-derived row is gone, every manual row survives untouched.
//
// Parent tables are hand-written minimal DDL, NOT AutoMigrate: the frozen
// types.MCPService model already carries PluginInstallationID, so
// AutoMigrate would pre-create the column and 000190's ALTER TABLE ADD
// COLUMN would fail with "column already exists" — the exact drift this
// suite exists to catch would be masked.
package plugins_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// pluginMigrationFiles are the migration files under test, in apply order.
var pluginMigrationFiles = []string{
	"000189_plugin_previews.up.sql",
	"000190_plugin_installations.up.sql",
}

func TestPluginMigrationEnvironment(t *testing.T) {
	if os.Getenv("PLUGIN_TEST_DATABASE_URL") == "" {
		t.Fatal("blocked-env: PLUGIN_TEST_DATABASE_URL required")
	}
}

// pluginMigrationsOpenDB opens the acceptance PostgreSQL, provisions an
// isolated schema, applies the REAL versioned migration files named in
// pluginMigrationFiles and registers the fixture cleanup (drop THIS schema
// only). The migration files are static repository assets read from
// migrations/versioned — the same execution mode oc_integration_test.go
// uses; they carry no caller-controlled input.
func pluginMigrationsOpenDB(t *testing.T) (*gorm.DB, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("PLUGIN_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("blocked-env: PLUGIN_TEST_DATABASE_URL required")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("open admin: %v", err)
	}
	schema := fmt.Sprintf("plugins_t06_%d", time.Now().UnixNano())
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		if err := admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Errorf("drop isolated schema %s: %v", schema, err)
		}
		sqlAdmin, _ := admin.DB()
		if sqlAdmin != nil {
			_ = sqlAdmin.Close()
		}
	})
	dsnSchema := dsn
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsnSchema = dsn + sep + "search_path=" + schema
	} else {
		// pgx keyword/value DSN: space-separated settings.
		dsnSchema = dsn + " search_path=" + schema
	}
	db, err := gorm.Open(postgres.Open(dsnSchema), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("open isolated schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range pluginMigrationFiles {
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "versioned", file))
		if err != nil {
			t.Fatalf("read migration %s: %v", file, err)
		}
		if _, err := sqlDB.ExecContext(context.Background(), string(raw)); err != nil {
			t.Fatalf("apply migration %s: %v", file, err)
		}
	}
	return db, sqlDB
}

// pluginMigrationDown executes one repository migration file (static asset,
// no caller input — same execution mode as the up pass above).
func pluginMigrationDown(t *testing.T, sqlDB *sql.DB, file string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "versioned", file))
	if err != nil {
		t.Fatalf("read migration %s: %v", file, err)
	}
	if _, err := sqlDB.ExecContext(context.Background(), string(raw)); err != nil {
		t.Fatalf("apply migration %s: %v", file, err)
	}
}

func pluginTableExists(t *testing.T, db *gorm.DB, table string) bool {
	t.Helper()
	var n int
	require.NoError(t, db.Raw(
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = ?",
		table,
	).Scan(&n).Error)
	return n == 1
}

func pluginColumnNullable(t *testing.T, db *gorm.DB, table, column string) (exists bool, nullable bool) {
	t.Helper()
	var isNullable string
	err := db.Raw(
		"SELECT is_nullable FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?",
		table, column,
	).Scan(&isNullable).Error
	require.NoError(t, err)
	if isNullable == "" {
		return false, false
	}
	return true, isNullable == "YES"
}

// TestPluginInstallationsMigrationUpAndDown pins the 000189/000190 contract:
// schema shapes on the way up (manifest_url NOT NULL frozen once), full
// derived-row cleanup on the way down, manual services untouched.
func TestPluginInstallationsMigrationUpAndDown(t *testing.T) {
	db, sqlDB := pluginMigrationsOpenDB(t)

	// Minimal parent tables (see file comment for why not AutoMigrate).
	require.NoError(t, db.Exec(`CREATE TABLE mcp_services (
		id VARCHAR(36) PRIMARY KEY,
		tenant_id BIGINT NOT NULL,
		name VARCHAR(255) NOT NULL,
		enabled BOOLEAN NOT NULL DEFAULT TRUE
	)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE mcp_tool_approvals (
		id VARCHAR(36) PRIMARY KEY,
		tenant_id BIGINT NOT NULL,
		service_id VARCHAR(36) NOT NULL,
		tool_name VARCHAR(512) NOT NULL
	)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE mcp_oauth_tokens (
		id VARCHAR(36) PRIMARY KEY,
		tenant_id BIGINT NOT NULL,
		user_id VARCHAR(64) NOT NULL,
		service_id VARCHAR(36) NOT NULL
	)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE mcp_oauth_clients (
		id VARCHAR(36) PRIMARY KEY,
		tenant_id BIGINT NOT NULL,
		service_id VARCHAR(36) NOT NULL,
		client_id VARCHAR(512) NOT NULL
	)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE mcp_metadata (
		id VARCHAR(36) PRIMARY KEY,
		tenant_id BIGINT NOT NULL,
		service_id VARCHAR(36) NOT NULL
	)`).Error)

	// ---- UP assertions: 000189 + 000190 already applied by the harness ----
	require.True(t, pluginTableExists(t, db, "plugin_previews"))
	require.True(t, pluginTableExists(t, db, "plugin_installations"))

	// manifest_url: exists and NOT NULL — the frozen long-lived upgrade
	// source (T14/T16 re-fetch from it; later tasks must never re-alter).
	exists, nullable := pluginColumnNullable(t, db, "plugin_installations", "manifest_url")
	require.True(t, exists, "plugin_installations.manifest_url must exist")
	require.False(t, nullable, "plugin_installations.manifest_url must be NOT NULL")

	exists, _ = pluginColumnNullable(t, db, "mcp_services", "plugin_installation_id")
	require.True(t, exists, "mcp_services.plugin_installation_id must exist")

	// ---- Seed: manual + plugin-materialized service, derived rows on both ----
	require.NoError(t, db.Exec("INSERT INTO mcp_services (id, tenant_id, name) VALUES (?, 1, 'manual-svc')", "svc-manual").Error)
	require.NoError(t, db.Exec("INSERT INTO mcp_services (id, tenant_id, name, plugin_installation_id) VALUES (?, 1, 'plugin:com.example.p', ?)",
		"svc-plugin", "inst-plugin-1").Error)
	require.NoError(t, db.Exec("INSERT INTO plugin_installations (id, tenant_id, plugin_id, name, manifest_url, accepted_version, transport_type, endpoint_url, tools_snapshot, tools_digest, service_id, drift_state, state, created_by) VALUES (?, 1, 'com.example.p', 'P', 'https://x.example.com/m.json', '1.0.0', 'http-streamable', 'https://x.example.com/mcp', '[]'::jsonb, 'digest', 'svc-plugin', 'none', 'active', 'admin')",
		"inst-plugin-1").Error)
	for _, svc := range []string{"svc-manual", "svc-plugin"} {
		require.NoError(t, db.Exec("INSERT INTO mcp_tool_approvals (id, tenant_id, service_id, tool_name) VALUES (?, 1, ?, 'tool-a')", "appr-"+svc, svc).Error)
		require.NoError(t, db.Exec("INSERT INTO mcp_oauth_tokens (id, tenant_id, user_id, service_id) VALUES (?, 1, 'user-1', ?)", "tok-"+svc, svc).Error)
		require.NoError(t, db.Exec("INSERT INTO mcp_oauth_clients (id, tenant_id, service_id, client_id) VALUES (?, 1, ?, 'cid')", "cli-"+svc, svc).Error)
		require.NoError(t, db.Exec("INSERT INTO mcp_metadata (id, tenant_id, service_id) VALUES (?, 1, ?)", "meta-"+svc, svc).Error)
	}

	// ---- DOWN: 000190 then 000189 ----
	pluginMigrationDown(t, sqlDB, "000190_plugin_installations.down.sql")

	require.False(t, pluginTableExists(t, db, "plugin_installations"))
	exists, _ = pluginColumnNullable(t, db, "mcp_services", "plugin_installation_id")
	require.False(t, exists, "mcp_services.plugin_installation_id must be dropped")

	// Plugin-derived rows fully cleaned across every service-keyed table;
	// manual rows survive untouched (GAP-3 rollback safety). One literal
	// statement per table, values bound via ?.
	var n int
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM mcp_services WHERE id = ?", "svc-plugin").Scan(&n).Error)
	require.Zero(t, n, "plugin-materialized service must be removed")
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM mcp_tool_approvals WHERE service_id = ?", "svc-plugin").Scan(&n).Error)
	require.Zero(t, n, "plugin-derived approval rows must be removed")
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM mcp_oauth_tokens WHERE service_id = ?", "svc-plugin").Scan(&n).Error)
	require.Zero(t, n, "member OAuth token rows keyed to the removed service must be cascade-cleaned")
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM mcp_oauth_clients WHERE service_id = ?", "svc-plugin").Scan(&n).Error)
	require.Zero(t, n, "dynamic-client rows keyed to the removed service must be cascade-cleaned")
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM mcp_metadata WHERE service_id = ?", "svc-plugin").Scan(&n).Error)
	require.Zero(t, n, "plugin-derived metadata rows must be removed")

	require.NoError(t, db.Raw("SELECT COUNT(*) FROM mcp_services WHERE id = ?", "svc-manual").Scan(&n).Error)
	require.Equal(t, 1, n, "manual service must survive the down migration")
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM mcp_tool_approvals WHERE service_id = ?", "svc-manual").Scan(&n).Error)
	require.Equal(t, 1, n, "manual approval rows must survive")
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM mcp_oauth_tokens WHERE service_id = ?", "svc-manual").Scan(&n).Error)
	require.Equal(t, 1, n, "manual OAuth token rows must survive")
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM mcp_oauth_clients WHERE service_id = ?", "svc-manual").Scan(&n).Error)
	require.Equal(t, 1, n, "manual OAuth client rows must survive")
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM mcp_metadata WHERE service_id = ?", "svc-manual").Scan(&n).Error)
	require.Equal(t, 1, n, "manual metadata rows must survive")

	pluginMigrationDown(t, sqlDB, "000189_plugin_previews.down.sql")
	require.False(t, pluginTableExists(t, db, "plugin_previews"))
}
