package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// versionedSQLiteTables is the set of tables that SQLite migrations must
// create to stay in sync with the versioned (PostgreSQL) migrations:
// 000041 task queue, 000053 system settings, 000055 processing spans,
// 000063 knowledge multi-tags, and 000086-000090 skill storage/catalog.
var versionedSQLiteTables = []string{
	"task_pending_ops",
	"task_dead_letters",
	"system_settings",
	"knowledge_processing_spans",
	"knowledge_tag_relations",
	"tenant_skills",
	"tenant_skill_snapshots",
	"tenant_user_env_vars",
	"tenant_skill_catalog",
	"execution_targets",
	"execution_target_identities",
	"execution_workspaces",
	"execution_cleanup",
	"execution_cleanup_artifacts",
	"mobile_devices",
	"mobile_notification_intents",
	"mobile_notification_checkpoints",
	"mobile_notification_provider_state",
}

// versionedSQLiteColumns maps each existing table to the columns that the
// versioned migrations add and the SQLite baseline was missing.
var versionedSQLiteColumns = map[string][]string{
	"tenants":            {"api_principal_config"},           // 000064
	"users":              {"is_system_admin"},                // 000053
	"knowledges":         {"pending_subtasks_count"},         // 000056
	"messages":           {"attachments", "usage"},           // 000034, 000085
	"tenant_invitations": {"token", "accepted_count"},        // 000054
	"embed_channels":     {"allow_memory"},                   // 000060
	"mcp_oauth_tokens":   {"principal_type", "principal_id"}, // 000064
	"mcp_tool_approvals": {"enabled"},                        // 000091
	"tenant_skills": {
		"catalog_id", "install_session_id", "install_message_id", "envs",
	}, // 000086-000090
	"tenant_skill_snapshots":      {"planned_name"},                                                         // 000086, 000088
	"execution_targets":           {"revoked_at", "runtime_id", "external_target_id", "usage_binding_json"}, // 000057, 000071
	"execution_target_identities": {"credential_version", "external_target_id"},                             // 000057
	"execution_workspaces":        {"target_id", "root_ref"},                                                // 000057
}

// 000014-000016 add the durable agent run tables (runs, tool calls and
// attempts, decisions, inputs, events, checkpoints); 000019 the workbench
// interaction queue, 000020 the execution dispatch log and 000021 the
// execution observation cursors. The versioned SQLite migration stream
// continues through 000040 (open-connector 000041-000044) and the Craft
// tables through 000052, native OIDC exchange through 000053, tenant skills
// at 000054, the workbench run rebuild at 000055, the workbench request
// queue at 000056, the execution target schema at 000057, the mobile device
// registry at 000058, the notification outbox at 000059, notification
// delivery columns at 000060, the cleanup ledger and artifact receipts at
// 000061-000066, artifact versions at 000067, the mobile voice plane at
// 000068-000069, the notification provider pause state at 000070 and the
// execution target usage binding at 000071.
const expectedSQLiteMigrationVersion = 71

func TestSQLiteMigrationsCreateVersionedSchema(t *testing.T) {
	repoRoot := sqliteRepoRoot(t)
	chdirAndRestore(t, repoRoot)

	dbPath := filepath.Join(t.TempDir(), "fresh.db")
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: dbPath}))

	db := openSQLiteDB(t, dbPath)
	version, dirty := sqliteMigrationState(t, db)
	require.Equal(t, expectedSQLiteMigrationVersion, version)
	require.False(t, dirty)

	for _, table := range versionedSQLiteTables {
		require.Truef(t, sqliteTableExists(t, db, table), "SQLite migrations must create table %s", table)
	}
	for _, index := range []string{"uq_mobile_devices_active_token", "idx_mobile_devices_owner"} {
		require.Truef(t, sqliteIndexExists(t, db, index), "SQLite migrations must create index %s", index)
	}
	for table, columns := range versionedSQLiteColumns {
		for _, column := range columns {
			require.Truef(
				t,
				sqliteColumnExists(t, db, table, column),
				"SQLite migrations must add column %s.%s",
				table,
				column,
			)
		}
	}

	assertSQLiteShareLinkInvitationsWork(t, db)
	assertSQLiteMCPOAuthPrincipalUpsertWorks(t, db)
	assertSQLiteSkillStorageWorks(t, db)
	require.False(t, sqliteColumnExists(t, db, "knowledges", "tag_id"),
		"SQLite migrations must drop legacy knowledges.tag_id after multi-tag migration")
}

func TestSQLiteMigrationsUpgradeV4PreservesData(t *testing.T) {
	repoRoot := sqliteRepoRoot(t)

	// Build a legacy v4 migration root (000000_init .. 000004_memory) so we
	// can prove the new migrations upgrade an existing Lite database without
	// replaying the baseline.
	legacyRoot := copySQLiteMigrationsV4(t, repoRoot)
	chdirAndRestore(t, legacyRoot)

	dbPath := filepath.Join(t.TempDir(), "upgrade.db")
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: dbPath}))

	db := openSQLiteDB(t, dbPath)
	versionBefore, dirtyBefore := sqliteMigrationState(t, db)
	require.Equal(t, 4, versionBefore)
	require.False(t, dirtyBefore)
	_, err := db.Exec("INSERT INTO tenants (name, business) VALUES (?, ?)", "upgrade-sentinel", "migration-test")
	require.NoError(t, err)
	_, err = db.Exec(
		"INSERT INTO knowledges (id, tenant_id, knowledge_base_id, type, title, source, tag_id) "+
			"VALUES (?, 1, ?, 'document', 'tagged-doc', 'manual', ?)",
		"legacy-knowledge-1", "legacy-kb-1", "legacy-tag-1",
	)
	require.NoError(t, err)

	// Run the full migration set from the repo root.
	chdirAndRestore(t, repoRoot)
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: dbPath}))

	db = openSQLiteDB(t, dbPath)
	versionAfter, dirtyAfter := sqliteMigrationState(t, db)
	require.Equal(t, expectedSQLiteMigrationVersion, versionAfter)
	require.False(t, dirtyAfter)

	for _, table := range versionedSQLiteTables {
		require.Truef(t, sqliteTableExists(t, db, table), "upgraded SQLite DB must have table %s", table)
	}
	for _, index := range []string{"uq_mobile_devices_active_token", "idx_mobile_devices_owner"} {
		require.Truef(t, sqliteIndexExists(t, db, index), "upgraded SQLite DB must create index %s", index)
	}
	for table, columns := range versionedSQLiteColumns {
		for _, column := range columns {
			require.Truef(
				t,
				sqliteColumnExists(t, db, table, column),
				"upgraded SQLite DB must have column %s.%s",
				table,
				column,
			)
		}
	}

	var sentinelName string
	require.NoError(t, db.QueryRow("SELECT name FROM tenants WHERE business = ?", "migration-test").Scan(&sentinelName))
	require.Equal(t, "upgrade-sentinel", sentinelName)

	var relationCount int
	require.NoError(t, db.QueryRow(
		"SELECT COUNT(*) FROM knowledge_tag_relations WHERE knowledge_id = ? AND tag_id = ?",
		"legacy-knowledge-1", "legacy-tag-1",
	).Scan(&relationCount))
	require.Equal(t, 1, relationCount)
	require.False(t, sqliteColumnExists(t, db, "knowledges", "tag_id"))
}

func sqliteRepoRoot(t *testing.T) string {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	return repoRoot
}

func chdirAndRestore(t *testing.T, dir string) {
	t.Helper()
	previousDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(previousDir) })
}

func openSQLiteDB(t *testing.T, dbPath string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func sqliteMigrationState(t *testing.T, db *sql.DB) (version int, dirty bool) {
	t.Helper()
	require.NoError(t, db.QueryRow("SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty))
	return version, dirty
}

// sqliteMigrationHead reads the migration fixture itself so schema assertions
// continue to validate the current SQLite stream when independent migrations
// are added.
func sqliteMigrationHead(t *testing.T, repoRoot string) int {
	t.Helper()
	versions := sqliteMigrationVersions(t, repoRoot)
	return versions[len(versions)-1]
}

// sqliteMigrationStepsAfter counts the applied SQLite migration files after a
// known version. Migration versions are sparse, so a numeric range cannot be
// used as the number of migrate.Steps calls required to reach the current head.
func sqliteMigrationStepsAfter(t *testing.T, repoRoot string, version int) int {
	t.Helper()
	steps := 0
	for _, candidate := range sqliteMigrationVersions(t, repoRoot) {
		if candidate > version {
			steps++
		}
	}
	require.Positivef(t, steps, "SQLite fixture must contain migrations after version %d", version)
	return steps
}

func sqliteMigrationVersions(t *testing.T, repoRoot string) []int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(repoRoot, "migrations", "sqlite"))
	require.NoError(t, err)

	versions := make([]int, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		versions = append(versions, sqliteMigrationFileVersion(t, entry.Name()))
	}
	require.NotEmpty(t, versions, "SQLite fixture must contain up migrations")
	sort.Ints(versions)
	for index := 1; index < len(versions); index++ {
		require.NotEqualf(t, versions[index-1], versions[index], "SQLite fixture has duplicate version %d", versions[index])
	}
	return versions
}

func sqliteMigrationFileVersion(t *testing.T, name string) int {
	t.Helper()
	versionText, _, found := strings.Cut(name, "_")
	require.Truef(t, found, "SQLite migration must use a versioned filename: %s", name)
	version, err := strconv.Atoi(versionText)
	require.NoErrorf(t, err, "SQLite migration must start with a numeric version: %s", name)
	return version
}

func sqliteTableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
		table,
	).Scan(&n))
	return n == 1
}

func sqliteIndexExists(t *testing.T, db *sql.DB, index string) bool {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?", index).Scan(&n))
	return n == 1
}

func sqliteColumnExists(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(
		"SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?",
		table,
		column,
	).Scan(&n))
	return n == 1
}

func assertSQLiteShareLinkInvitationsWork(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec("INSERT INTO tenants (name, business) VALUES (?, ?)", "share-link-tenant", "share-link-test")
	require.NoError(t, err)

	expiresAt := "2099-01-01 00:00:00"
	shareLinkInsert := "INSERT INTO tenant_invitations " +
		"(tenant_id, invitee_user_id, token, role, status, expires_at) " +
		"VALUES (1, '', ?, 'member', 'pending', ?)"
	_, err = db.Exec(shareLinkInsert, "token-a", expiresAt)
	require.NoError(t, err)
	_, err = db.Exec(shareLinkInsert, "token-b", expiresAt)
	require.NoError(t, err)

	var count int
	require.NoError(t, db.QueryRow(
		"SELECT COUNT(*) FROM tenant_invitations WHERE tenant_id = 1 AND invitee_user_id = '' AND status = 'pending'",
	).Scan(&count))
	require.Equal(t, 2, count)
}

func assertSQLiteMCPOAuthPrincipalUpsertWorks(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(
		"INSERT INTO mcp_services (id, tenant_id, name, transport_type) VALUES (?, 1, 'svc', 'http')",
		"svc-migration-1",
	)
	require.NoError(t, err)

	tokenInsertPrefix := "INSERT INTO mcp_oauth_tokens " +
		"(id, tenant_id, user_id, service_id, principal_type, principal_id, access_token) "
	_, err = db.Exec(
		tokenInsertPrefix +
			"VALUES ('tok-1', 1, 'u1', 'svc-migration-1', 'web_user', 'u1', 'token-1')",
	)
	require.NoError(t, err)

	_, err = db.Exec(
		tokenInsertPrefix +
			"VALUES ('tok-2', 1, 'u1', 'svc-migration-1', 'web_user', 'u1', 'token-2') " +
			"ON CONFLICT(tenant_id, principal_type, principal_id, service_id) " +
			"DO UPDATE SET access_token = excluded.access_token",
	)
	require.NoError(t, err)

	var accessToken string
	require.NoError(t, db.QueryRow(
		"SELECT access_token FROM mcp_oauth_tokens "+
			"WHERE tenant_id = 1 AND principal_type = 'web_user' "+
			"AND principal_id = 'u1' AND service_id = 'svc-migration-1'",
	).Scan(&accessToken))
	require.Equal(t, "token-2", accessToken)

	var rowCount int
	require.NoError(t, db.QueryRow(
		"SELECT COUNT(*) FROM mcp_oauth_tokens WHERE tenant_id = 1 AND service_id = 'svc-migration-1'",
	).Scan(&rowCount))
	require.Equal(t, 1, rowCount)
}

func assertSQLiteSkillStorageWorks(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(
		"INSERT INTO tenant_skill_catalog (id, tenant_id, name, version, description) VALUES (?, 1, ?, ?, ?)",
		"catalog-migration-1", "csv-tool", "1.0.0", "CSV helper",
	)
	require.NoError(t, err)
	_, err = db.Exec(
		"INSERT INTO tenant_skills (id, tenant_id, sandbox_config_id, catalog_id, name, status) VALUES (?, 1, ?, ?, ?, ?)",
		"skill-migration-1", "config-migration-1", "catalog-migration-1", "csv-tool", "ready",
	)
	require.NoError(t, err)
	_, err = db.Exec(
		"INSERT INTO tenant_skill_snapshots (id, tenant_id, sandbox_config_id, skill_id, trigger, state) VALUES (?, 1, ?, ?, ?, ?)",
		"snapshot-migration-1", "config-migration-1", "skill-migration-1", "install", "active",
	)
	require.NoError(t, err)
	_, err = db.Exec(
		"INSERT INTO tenant_user_env_vars (id, tenant_id, principal_type, principal_id, sandbox_config_id, name, value) VALUES (?, 1, ?, ?, ?, ?, ?)",
		"env-migration-1", "web_user", "user-migration-1", "config-migration-1", "API_KEY", "encrypted-value",
	)
	require.NoError(t, err)

	var count int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM tenant_skill_catalog WHERE id = ?", "catalog-migration-1").Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM tenant_skills WHERE catalog_id = ?", "catalog-migration-1").Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM tenant_skill_snapshots WHERE skill_id = ?", "skill-migration-1").Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM tenant_user_env_vars WHERE name = ?", "API_KEY").Scan(&count))
	require.Equal(t, 1, count)

	_, err = db.Exec(
		"INSERT INTO tenant_skills (id, tenant_id, sandbox_config_id, catalog_id, name, status) VALUES (?, 1, ?, ?, ?, ?)",
		"skill-migration-duplicate", "config-migration-1", "catalog-migration-1", "csv-tool", "ready",
	)
	require.Error(t, err, "active skill names must remain unique within a sandbox config")
}

func copySQLiteMigrationsV4(t *testing.T, repoRoot string) string {
	t.Helper()
	dest := t.TempDir()
	srcDir := filepath.Join(repoRoot, "migrations", "sqlite")
	destDir := filepath.Join(dest, "migrations", "sqlite")
	require.NoError(t, os.MkdirAll(destDir, 0o755))

	legacy := []string{
		"000000_init.up.sql",
		"000001_remove_wiki_log.up.sql",
		"000002_knowledge_folder_path.up.sql",
		"000003_knowledge_base_auto_tag_config.up.sql",
		"000004_memory.up.sql",
	}
	for _, name := range legacy {
		data, err := os.ReadFile(filepath.Join(srcDir, name))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(destDir, name), data, 0o600))
	}
	return dest
}
