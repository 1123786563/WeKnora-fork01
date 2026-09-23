package database

import (
	"context"
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
