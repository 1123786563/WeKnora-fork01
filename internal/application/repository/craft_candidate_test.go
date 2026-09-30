package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func candidateRecord(t *testing.T, scope craft.Scope, workspaceID, runID, generation, content string) craft.Candidate {
	t.Helper()
	sum := sha256.Sum256([]byte(content))
	files := []craft.File{{
		Path: "index.html", Ref: "resource://candidate/" + runID,
		SHA256: hex.EncodeToString(sum[:]), MIME: "text/html", Bytes: int64(len(content)),
	}}
	digest, err := craft.ManifestDigest(files)
	require.NoError(t, err)
	evidence := craft.ArtifactEvidence{BuildRan: true, BuildExitCode: 0}
	candidate := craft.Candidate{
		ID: craft.CandidateID(workspaceID, runID, digest), Scope: scope,
		WorkspaceID: workspaceID, RunID: runID, Generation: generation,
		Kind: craft.KindWeb, ManifestDigest: digest, Files: files, Evidence: evidence,
		Checks: craft.BuildChecks(craft.KindWeb, files, evidence),
	}
	return candidate
}

func TestCraftCandidateStoreIsPrivateIdempotentAndScoped(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			store := NewCraftCandidateStore(db)
			workspace := putCraftWorkspace(t, NewCraftStore(db))
			fence := seedCraftRun(t, db, "candidate-run-1", "candidate-call-1")
			candidate := candidateRecord(t, craftTestScope(), workspace.ID, fence.RunID, "rv_generation_1", "<h1>one</h1>")

			first, err := store.PutCandidate(context.Background(), candidate.Scope, candidate)
			require.NoError(t, err)
			require.Equal(t, candidate.ID, first.ID)
			replayInput := candidate
			replayInput.Files = append([]craft.File(nil), candidate.Files...)
			replayInput.Files[0].Ref = "resource://candidate/redundant-retry-upload"
			replay, err := store.PutCandidate(context.Background(), candidate.Scope, replayInput)
			require.NoError(t, err)
			require.Equal(t, first, replay, "same bytes adopt the sealed reference despite a redundant upload ref")

			got, err := store.GetCandidate(context.Background(), candidate.Scope, candidate.ID)
			require.NoError(t, err)
			require.Equal(t, first, got)
			versions := NewCraftVersionStore(db)
			list, err := versions.List(context.Background(), candidate.Scope)
			require.NoError(t, err)
			require.Empty(t, list, "private candidates are not versions")
			_, err = versions.Get(context.Background(), candidate.Scope, candidate.ID)
			require.ErrorIs(t, err, craft.ErrNotFound)

			changed := candidateRecord(t, candidate.Scope, workspace.ID, fence.RunID, "rv_generation_1", "<h1>changed</h1>")
			_, err = store.PutCandidate(context.Background(), candidate.Scope, changed)
			require.ErrorIs(t, err, craft.ErrConflict, "one Run cannot replace its candidate with changed bytes")

			otherOwner := candidate.Scope
			otherOwner.UserID = "other-user"
			_, err = store.GetCandidate(context.Background(), otherOwner, candidate.ID)
			require.ErrorIs(t, err, craft.ErrForbidden)
			otherTask := candidate.Scope
			otherTask.SessionID = "s2"
			_, err = store.GetCandidate(context.Background(), otherTask, candidate.ID)
			require.ErrorIs(t, err, craft.ErrNotFound)

			updateErr := db.Table("craft_candidates").Where("id = ?", candidate.ID).Update("generation", "rv_mutated").Error
			require.Error(t, updateErr, "candidate identity must be immutable at the database boundary")
		})
	}
}

func TestCraftCandidateStoreRollsBackHeaderWhenFileSealFails(t *testing.T) {
	db := openCraftDB(t)
	store := NewCraftCandidateStore(db)
	workspace := putCraftWorkspace(t, NewCraftStore(db))
	fence := seedCraftRun(t, db, "candidate-rollback-run", "candidate-rollback-call")
	candidate := candidateRecord(t, craftTestScope(), workspace.ID, fence.RunID, "rv_generation_rollback", "<h1>rollback</h1>")
	require.NoError(t, db.Exec(`CREATE TRIGGER reject_candidate_file_insert
		BEFORE INSERT ON craft_candidate_files
		BEGIN SELECT RAISE(ABORT, 'injected file seal failure'); END;`).Error)
	_, err := store.PutCandidate(context.Background(), candidate.Scope, candidate)
	require.Error(t, err)
	var rows int64
	require.NoError(t, db.Table("craft_candidates").Where("id = ?", candidate.ID).Count(&rows).Error)
	require.Zero(t, rows, "candidate header and files must commit atomically")
}

func TestCraftCandidateMigrationSQLiteUpDownUp(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "craft-candidate-migration.db") + "?_foreign_keys=on"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	defer sqlDB.Close()
	_, err = sqlDB.Exec(`CREATE TABLE craft_workspaces (tenant_id BIGINT NOT NULL, id TEXT NOT NULL, UNIQUE(tenant_id, id))`)
	require.NoError(t, err)
	_, err = sqlDB.Exec(`CREATE TABLE agent_runs (tenant_id BIGINT NOT NULL, run_id TEXT NOT NULL, PRIMARY KEY(tenant_id, run_id))`)
	require.NoError(t, err)
	singleMigrationDir := t.TempDir()
	for _, direction := range []string{"up", "down"} {
		contents, readErr := os.ReadFile(filepath.Join(root, "migrations/sqlite/000173_craft_candidate."+direction+".sql"))
		require.NoError(t, readErr)
		require.NoError(t, os.WriteFile(filepath.Join(singleMigrationDir, "000001_craft_candidate."+direction+".sql"), contents, 0o600))
	}
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	m, err := migrate.NewWithDatabaseInstance("file://"+singleMigrationDir, "sqlite3", driver)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Close() })
	var tables int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'craft_candidates'`).Scan(&tables))
	require.Zero(t, tables)
	require.NoError(t, m.Up())
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'craft_candidates'`).Scan(&tables))
	require.Equal(t, 1, tables)
	require.NoError(t, m.Down())
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'craft_candidates'`).Scan(&tables))
	require.Zero(t, tables)
	require.NoError(t, m.Up())
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'craft_candidates'`).Scan(&tables))
	require.Equal(t, 1, tables)
}

func TestCraftCandidateMigrationPostgresUpDownUp(t *testing.T) {
	dsn := os.Getenv("TRPC_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TRPC_TEST_POSTGRES_DSN unset: candidate PostgreSQL migration proof NOT VERIFIED")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	schema := "craft_candidate_migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() { require.NoError(t, admin.Exec("DROP SCHEMA "+schema+" CASCADE").Error) })
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, parseErr := url.Parse(dsn)
		require.NoError(t, parseErr)
		q := u.Query()
		q.Set("search_path", schema+",public")
		u.RawQuery = q.Encode()
		dsn = u.String()
	} else {
		dsn += " search_path=" + schema + ",public"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE craft_workspaces (tenant_id BIGINT NOT NULL, id VARCHAR(64) NOT NULL, UNIQUE(tenant_id, id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE agent_runs (tenant_id BIGINT NOT NULL, run_id VARCHAR(64) NOT NULL, PRIMARY KEY(tenant_id, run_id))`).Error)
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	singleMigrationDir := t.TempDir()
	for _, direction := range []string{"up", "down"} {
		contents, readErr := os.ReadFile(filepath.Join(root, "migrations/versioned/000201_craft_candidate."+direction+".sql"))
		require.NoError(t, readErr)
		require.NoError(t, os.WriteFile(filepath.Join(singleMigrationDir, "000001_craft_candidate."+direction+".sql"), contents, 0o600))
	}
	conn, err := db.DB()
	require.NoError(t, err)
	driver, err := pgmigrate.WithInstance(conn, &pgmigrate.Config{SchemaName: schema})
	require.NoError(t, err)
	m, err := migrate.NewWithDatabaseInstance("file://"+singleMigrationDir, "postgres", driver)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Close(); _ = conn.Close() })
	require.NoError(t, m.Up())
	require.True(t, db.Migrator().HasTable("craft_candidates"))
	require.NoError(t, m.Down())
	require.False(t, db.Migrator().HasTable("craft_candidates"))
	require.NoError(t, m.Up())
	require.True(t, db.Migrator().HasTable("craft_candidates"))
}
