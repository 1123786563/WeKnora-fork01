package database

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/career"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCareerMigrationCreatesPersonalEvidenceSchema(t *testing.T) {
	root := sqliteRepoRoot(t)
	chdirAndRestore(t, root)
	path := filepath.Join(t.TempDir(), "career-migration.db")
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: path}))
	db := openSQLiteDB(t, path)
	for _, table := range []string{"career_spaces", "career_profiles", "career_facts", "career_fact_versions", "career_proposals", "career_changes", "career_receipts", "career_source_revisions", "career_opportunities", "career_opportunity_observations", "career_opportunity_snapshots", "career_opportunity_receipts", "career_evaluations", "career_applications"} {
		require.Truef(t, sqliteTableExists(t, db, table), "career migration must create %s", table)
	}
	for _, column := range []string{"id", "tenant_id", "user_id", "revision", "file_name", "mime_type", "size", "digest", "request_id", "intent_hash", "expected_revision", "claim_token", "lease_until", "resource_ref", "status", "error_category", "error_message", "extracted_text", "missing_categories", "review_flags", "created_at", "completed_at"} {
		var count int
		require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('career_source_revisions') WHERE name = ?", column).Scan(&count))
		require.Equalf(t, 1, count, "career_source_revisions must include %s", column)
	}
	for table, columns := range map[string][]string{
		"career_opportunities":            {"id", "tenant_id", "user_id", "created_at"},
		"career_opportunity_observations": {"id", "tenant_id", "user_id", "opportunity_id", "snapshot_id", "source_kind", "source_label", "source_ref", "acquired_at", "created_at", "source_status", "completeness", "failure_code", "submitted_url", "final_url", "adapter_id", "adapter_version", "observed_http_status"},
		"career_opportunity_snapshots":    {"id", "tenant_id", "user_id", "opportunity_id", "observation_id", "raw_text", "raw_sha256", "extracted", "status", "acquired_at", "created_at"},
		"career_opportunity_receipts":     {"tenant_id", "user_id", "request_id", "fingerprint", "body", "created_at"},
		"career_evaluations":              {"id", "tenant_id", "user_id", "request_id", "fingerprint", "intent", "opportunity_id", "snapshot_id", "profile_revision", "receipt_body", "evaluation_body", "created_at"},
		"career_applications":             {"id", "tenant_id", "user_id", "request_id", "fingerprint", "opportunity_id", "snapshot_id", "evaluation_id", "profile_revision", "evidence_body", "batch_identity", "continue_despite_hard_failure", "evaluation_status", "qualified", "warning_body", "link_state", "task_id", "run_id", "receipt_body", "created_at", "updated_at"},
	} {
		for _, column := range columns {
			var count int
			require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?", table, column).Scan(&count))
			require.Equalf(t, 1, count, "%s must include %s", table, column)
		}
	}
	for _, column := range []string{"source", "confirmation", "public_id", "status", "resolution_source", "evidence"} {
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

func TestCareerSQLiteMigrationPersistsPrivateSourceRevisionAndPublicMetadata(t *testing.T) {
	root := sqliteRepoRoot(t)
	chdirAndRestore(t, root)
	path := filepath.Join(t.TempDir(), "career-source.db")
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: path}))
	careerDB, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	office, err := career.NewOffice(careerDB)
	require.NoError(t, err)
	ctx := career.WithScope(context.Background(), career.Scope{UserID: "source-owner", TenantID: 913})
	require.NoError(t, office.ClaimSpace(ctx))
	source, err := office.CreateSource(ctx, career.SourceUpload{ID: "source-rev-1", FileName: "resume.pdf", MIMEType: "application/pdf", Size: 32, Digest: "sha256:source", ResourceRef: "private://source", Text: "private extracted resume"}, "")
	require.NoError(t, err)
	require.Equal(t, uint64(1), source.Revision)
	encoded, err := json.Marshal(source)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private://source")
	require.NotContains(t, string(encoded), "private extracted resume")
	sqlCareerDB, err := careerDB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlCareerDB.Close())
	reopenedCareerDB, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	reopenedOffice, err := career.NewOffice(reopenedCareerDB)
	require.NoError(t, err)
	listed, err := reopenedOffice.ListSources(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, "source-rev-1", listed[0].ID)
	require.Equal(t, "ready", listed[0].Status)
	var stored struct {
		ResourceRef   string
		ExtractedText string
	}
	require.NoError(t, reopenedCareerDB.Table("career_source_revisions").Select("resource_ref, extracted_text").Where("id=?", source.ID).Scan(&stored).Error)
	require.Equal(t, "private://source", stored.ResourceRef)
	require.Equal(t, "private extracted resume", stored.ExtractedText)
}

func TestCareerOfficeOpensAfterVersionedSQLiteMigration(t *testing.T) {
	root := sqliteRepoRoot(t)
	chdirAndRestore(t, root)
	path := filepath.Join(t.TempDir(), "career-startup.db")
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: path}))

	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	office, err := career.NewOffice(db)
	require.NoError(t, err)
	// Reopen through a separate connection to represent process restart against
	// an already migrated database.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	db, err = gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	office, err = career.NewOffice(db)
	require.NoError(t, err)

	ctx := career.WithScope(context.Background(), career.Scope{UserID: "startup-user", TenantID: 42})
	require.NoError(t, office.ClaimSpace(ctx))
	// Schema uniqueness must remain enforced after GORM initialization.
	require.NoError(t, db.Exec("INSERT INTO career_facts (tenant_id,user_id,key,value,revision,source,confirmation,request_id,created_at) VALUES (42,'startup-user','skill','go',1,'{}','{}','r1',CURRENT_TIMESTAMP)").Error)
	require.Error(t, db.Exec("INSERT INTO career_facts (tenant_id,user_id,key,value,revision,source,confirmation,request_id,created_at) VALUES (42,'startup-user','skill','rust',2,'{}','{}','r2',CURRENT_TIMESTAMP)").Error)
	require.NoError(t, db.Exec("INSERT INTO career_receipts (tenant_id,user_id,request_id,fingerprint,body,created_at) VALUES (42,'startup-user','r1','f','{}',CURRENT_TIMESTAMP)").Error)
	require.Error(t, db.Exec("INSERT INTO career_receipts (tenant_id,user_id,request_id,fingerprint,body,created_at) VALUES (42,'startup-user','r1','g','{}',CURRENT_TIMESTAMP)").Error)
	require.NoError(t, db.Exec("INSERT INTO career_changes (tenant_id,user_id,revision,kind,body,created_at) VALUES (42,'startup-user',1,'fact_confirmed','{}',CURRENT_TIMESTAMP)").Error)
	require.Error(t, db.Exec("INSERT INTO career_changes (tenant_id,user_id,revision,kind,body,created_at) VALUES (42,'startup-user',1,'fact_confirmed','{}',CURRENT_TIMESTAMP)").Error)
}

func TestCareerOpportunitySQLiteMigrationUpDownUp(t *testing.T) {
	root := sqliteRepoRoot(t)
	chdirAndRestore(t, root)
	path := filepath.Join(t.TempDir(), "career-up-down.db")
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: path}))
	db := openSQLiteDB(t, path)
	version, _ := sqliteMigrationState(t, db)
	require.Equal(t, 119, version)
	m, err := newSQLiteMigrator("file://"+filepath.Join(root, "migrations/sqlite"), path, "", true)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Close() })
	require.NoError(t, m.Migrate(114))
	for _, table := range []string{"career_opportunities", "career_opportunity_observations", "career_opportunity_snapshots", "career_opportunity_receipts"} {
		require.False(t, sqliteTableExists(t, db, table), "down migration must remove %s", table)
	}
	require.True(t, sqliteTableExists(t, db, "career_source_revisions"), "down migration must preserve the prior Career schema")
	require.NoError(t, m.Up())
	for _, table := range []string{"career_opportunities", "career_opportunity_observations", "career_opportunity_snapshots", "career_opportunity_receipts"} {
		require.True(t, sqliteTableExists(t, db, table), "up migration must restore %s", table)
	}
	require.True(t, sqliteTableExists(t, db, "career_source_revisions"))
}

func TestSourceObservationMigrationUpAndDown(t *testing.T) {
	root := sqliteRepoRoot(t)
	chdirAndRestore(t, root)
	for _, tree := range []string{"migrations/versioned/000198_source_import_observations.up.sql", "migrations/versioned/000198_source_import_observations.down.sql", "migrations/sqlite/000119_source_import_observations.up.sql", "migrations/sqlite/000119_source_import_observations.down.sql"} {
		require.FileExistsf(t, filepath.Join(root, tree), "paired migration files must exist: %s", tree)
	}
	path := filepath.Join(t.TempDir(), "career-source-observation.db")
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: path}))
	db := openSQLiteDB(t, path)
	version, _ := sqliteMigrationState(t, db)
	require.Equal(t, 119, version)
	addedColumns := []string{"source_status", "completeness", "failure_code", "submitted_url", "final_url", "adapter_id", "adapter_version", "observed_http_status"}
	preservedColumns := []string{"id", "tenant_id", "user_id", "opportunity_id", "snapshot_id", "source_kind", "source_label", "source_ref", "acquired_at", "created_at"}
	columnPresent := func(column string) bool {
		var count int
		require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('career_opportunity_observations') WHERE name = ?", column).Scan(&count))
		return count == 1
	}
	for _, column := range addedColumns {
		require.Truef(t, columnPresent(column), "up migration must add %s", column)
	}
	_, err := db.Exec("INSERT INTO career_opportunity_observations (id,tenant_id,user_id,opportunity_id,snapshot_id,source_kind,source_label,source_ref,acquired_at,created_at,source_status,completeness,failure_code,submitted_url,final_url,adapter_id,adapter_version,observed_http_status) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
		"obs-1", uint64(42), "u", "opp-1", "snap-1", "url", "", "https://jobs.example.test/j", "2026-09-25 00:00:00", "2026-09-25 00:00:00", "partial", "incomplete", "timeout", "https://jobs.example.test/j", "https://jobs.example.test/final", "stub", "1", 200)
	require.NoError(t, err)

	m, err := newSQLiteMigrator("file://"+filepath.Join(root, "migrations/sqlite"), path, "", true)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Close() })
	// 117/118 are reserved for the parallel T14 tasks and absent from this
	// baseline, so the nearest real prior version is 116.
	require.NoError(t, m.Migrate(116))
	downVersion, _ := sqliteMigrationState(t, db)
	require.Equal(t, 116, downVersion)
	require.True(t, sqliteTableExists(t, db, "career_opportunity_observations"), "down migration must keep the table")
	for _, column := range preservedColumns {
		require.Truef(t, columnPresent(column), "down migration must preserve %s", column)
	}
	for _, column := range addedColumns {
		require.Falsef(t, columnPresent(column), "down migration must drop only the added column %s", column)
	}
	require.NoError(t, m.Up())
	version, _ = sqliteMigrationState(t, db)
	require.Equal(t, 119, version)
	for _, column := range addedColumns {
		require.Truef(t, columnPresent(column), "re-applying up must restore %s", column)
	}
}

func TestCareerEvaluationSQLiteMigrationUpDownUp(t *testing.T) {
	root := sqliteRepoRoot(t)
	chdirAndRestore(t, root)
	path := filepath.Join(t.TempDir(), "career-evaluation-up-down.db")
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: path}))
	db := openSQLiteDB(t, path)
	version, _ := sqliteMigrationState(t, db)
	require.Equal(t, 119, version)
	m, err := newSQLiteMigrator("file://"+filepath.Join(root, "migrations/sqlite"), path, "", true)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Close() })
	require.NoError(t, m.Migrate(115))
	require.False(t, sqliteTableExists(t, db, "career_evaluations"))
	for _, table := range []string{"career_opportunities", "career_opportunity_observations", "career_opportunity_snapshots", "career_opportunity_receipts"} {
		require.True(t, sqliteTableExists(t, db, table), "evaluation down migration must preserve %s", table)
	}
	require.NoError(t, m.Up())
	require.True(t, sqliteTableExists(t, db, "career_evaluations"))
	for _, column := range []string{"id", "tenant_id", "user_id", "request_id", "fingerprint", "intent", "opportunity_id", "snapshot_id", "profile_revision", "receipt_body", "evaluation_body", "created_at"} {
		var count int
		require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('career_evaluations') WHERE name = ?", column).Scan(&count))
		require.Equalf(t, 1, count, "career_evaluations must include %s", column)
	}
}

func TestCareerApplicationSQLiteMigrationUpDownUp(t *testing.T) {
	root := sqliteRepoRoot(t)
	chdirAndRestore(t, root)
	for _, tree := range []string{
		"migrations/versioned/000197_career_applications.up.sql",
		"migrations/versioned/000197_career_applications.down.sql",
		"migrations/sqlite/000118_career_applications.up.sql",
		"migrations/sqlite/000118_career_applications.down.sql",
	} {
		require.FileExistsf(t, filepath.Join(root, tree), "paired migration files must exist: %s", tree)
	}
	path := filepath.Join(t.TempDir(), "career-application-up-down.db")
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: path}))
	db := openSQLiteDB(t, path)
	version, _ := sqliteMigrationState(t, db)
	require.Equal(t, 119, version)
	require.True(t, sqliteTableExists(t, db, "career_applications"))

	insertApplication := func(id, requestID, opportunityID, batch string) error {
		_, err := db.Exec(`INSERT INTO career_applications
			(id,tenant_id,user_id,request_id,fingerprint,opportunity_id,snapshot_id,evaluation_id,
			 profile_revision,evidence_body,batch_identity,continue_despite_hard_failure,evaluation_status,
			 qualified,warning_body,link_state,receipt_body,created_at,updated_at)
			VALUES (?,42,'app-owner',?,?,?, 'snap-1','eval-1', 1,'{}',?,0,'eligible', 1,'','linking','{}',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
			id, requestID, strings.Repeat("f", 64), opportunityID, batch)
		return err
	}
	require.NoError(t, insertApplication("app-1", "req-1", "opp-1", "batch-a"))
	require.Error(t, insertApplication("app-2", "req-1", "opp-1", "batch-b"), "request uniqueness must reject a duplicate (tenant,user,request)")
	require.Error(t, insertApplication("app-3", "req-2", "opp-1", "batch-a"), "job/batch uniqueness must reject a second application for the same job and batch")
	require.NoError(t, insertApplication("app-4", "req-3", "opp-1", "batch-b"))

	m, err := newSQLiteMigrator("file://"+filepath.Join(root, "migrations/sqlite"), path, "", true)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Close() })
	require.NoError(t, m.Migrate(117))
	require.False(t, sqliteTableExists(t, db, "career_applications"))
	for _, table := range []string{"career_evaluations", "career_opportunities", "career_opportunity_snapshots"} {
		require.Truef(t, sqliteTableExists(t, db, table), "application down migration must preserve %s", table)
	}
	require.NoError(t, m.Up())
	require.True(t, sqliteTableExists(t, db, "career_applications"))
	var count int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM career_applications").Scan(&count))
	require.Zero(t, count, "re-applying up must restore an empty table")
}

func TestCareerOfficeRejectsMalformedVersionedSQLiteSchema(t *testing.T) {
	root := sqliteRepoRoot(t)
	chdirAndRestore(t, root)

	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, *gorm.DB)
		want   string
	}{
		{
			name: "missing required column",
			mutate: func(t *testing.T, db *gorm.DB) {
				require.NoError(t, db.Exec("ALTER TABLE career_facts DROP COLUMN confirmation").Error)
			},
			want: "career_facts.confirmation",
		},
		{
			name: "missing fact uniqueness",
			mutate: func(t *testing.T, db *gorm.DB) {
				require.NoError(t, db.Exec("ALTER TABLE career_facts RENAME TO career_facts_old").Error)
				require.NoError(t, db.Exec("CREATE TABLE career_facts (id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, user_id TEXT NOT NULL, key TEXT NOT NULL, value TEXT NOT NULL, revision INTEGER NOT NULL, source TEXT NOT NULL, confirmation TEXT NOT NULL, request_id TEXT NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)").Error)
				require.NoError(t, db.Exec("DROP TABLE career_facts_old").Error)
			},
			want: "career_facts uniqueness",
		},
		{
			name: "partial fact uniqueness",
			mutate: func(t *testing.T, db *gorm.DB) {
				require.NoError(t, db.Exec("ALTER TABLE career_facts RENAME TO career_facts_old").Error)
				require.NoError(t, db.Exec("CREATE TABLE career_facts (id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, user_id TEXT NOT NULL, key TEXT NOT NULL, value TEXT NOT NULL, revision INTEGER NOT NULL, source TEXT NOT NULL, confirmation TEXT NOT NULL, request_id TEXT NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)").Error)
				require.NoError(t, db.Exec("DROP TABLE career_facts_old").Error)
				require.NoError(t, db.Exec("CREATE UNIQUE INDEX uq_career_facts_partial ON career_facts (tenant_id, user_id, key) WHERE key <> 'skip'").Error)
				for i := 0; i < 2; i++ {
					require.NoError(t, db.Exec("INSERT INTO career_facts (tenant_id,user_id,key,value,revision,source,confirmation,request_id,created_at) VALUES (42,'u','skip',? ,?,'{}','{}',?,CURRENT_TIMESTAMP)", i, i, i).Error)
				}
			},
			want: "career_facts uniqueness",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "malformed.db")
			require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: path}))
			db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
			require.NoError(t, err)
			tc.mutate(t, db)
			_, err = career.NewOffice(db)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestCareerOfficeRejectsPartialVersionedSQLiteSchema(t *testing.T) {
	root := sqliteRepoRoot(t)
	chdirAndRestore(t, root)
	path := filepath.Join(t.TempDir(), "partial.db")
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: path}))
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("DROP TABLE career_receipts").Error)

	_, err = career.NewOffice(db)
	require.ErrorContains(t, err, "incomplete Career SQLite schema")
	require.False(t, db.Migrator().HasTable("career_receipts"), "startup must not run AutoMigrate over a partial versioned schema")
}
