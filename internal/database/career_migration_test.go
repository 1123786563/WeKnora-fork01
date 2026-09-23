package database

import (
	"context"
	"encoding/json"
	"path/filepath"
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
	for _, table := range []string{"career_spaces", "career_profiles", "career_facts", "career_fact_versions", "career_proposals", "career_changes", "career_receipts", "career_source_revisions"} {
		require.Truef(t, sqliteTableExists(t, db, table), "career migration must create %s", table)
	}
	for _, column := range []string{"id", "tenant_id", "user_id", "revision", "file_name", "mime_type", "size", "digest", "request_id", "intent_hash", "expected_revision", "claim_token", "lease_until", "resource_ref", "status", "error_category", "error_message", "extracted_text", "missing_categories", "review_flags", "created_at", "completed_at"} {
		var count int
		require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('career_source_revisions') WHERE name = ?", column).Scan(&count))
		require.Equalf(t, 1, count, "career_source_revisions must include %s", column)
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

func TestCareerSQLiteMigrationUpDownUpRestoresPriorProposalSchema(t *testing.T) {
	root := sqliteRepoRoot(t)
	chdirAndRestore(t, root)
	path := filepath.Join(t.TempDir(), "career-up-down.db")
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: path}))
	db := openSQLiteDB(t, path)
	version, _ := sqliteMigrationState(t, db)
	require.Equal(t, 114, version)
	m, err := newSQLiteMigrator("file://"+filepath.Join(root, "migrations/sqlite"), path, "", true)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Close() })
	require.NoError(t, m.Migrate(113))
	require.False(t, sqliteTableExists(t, db, "career_source_revisions"))
	var count int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('career_proposals') WHERE name='evidence'").Scan(&count))
	require.Zero(t, count)
	require.NoError(t, m.Up())
	require.True(t, sqliteTableExists(t, db, "career_source_revisions"))
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('career_proposals') WHERE name='evidence'").Scan(&count))
	require.Equal(t, 1, count)
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
