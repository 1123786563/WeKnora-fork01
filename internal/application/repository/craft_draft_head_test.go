package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func sealedDraftFile(path, ref, digit string, bytes int64) craft.File {
	return craft.File{Path: path, Ref: ref, SHA256: strings.Repeat(digit, 64), Bytes: bytes, MIME: "text/html"}
}

func shortDraftIdentity(label string) string {
	sum := sha256.Sum256([]byte(label))
	return "draft-" + hex.EncodeToString(sum[:10])
}

func seedShortDraftRun(t *testing.T, db *gorm.DB, runLabel, callLabel string) agentruntime.Fence {
	t.Helper()
	return seedCraftRun(t, db, shortDraftIdentity(runLabel), shortDraftIdentity(callLabel))
}

func terminalDraftRun(t *testing.T, db *gorm.DB, runID, callID string) agentruntime.Fence {
	t.Helper()
	// This fixture stands for a trusted caller that already observed its
	// RunView idle/stopped under the Workspace writer fence; the repository
	// still validates durable Run and journal state independently.
	fence := seedShortDraftRun(t, db, runID, callID)
	require.NoError(t, db.Exec("UPDATE agent_tool_calls SET status = 'failed' WHERE tenant_id = ? AND run_id = ? AND call_id = ?", fence.TenantID, fence.RunID, shortDraftIdentity(callID)).Error)
	require.NoError(t, NewAgentRunStore(db).SetStatus(context.Background(), fence, "failed", "build failed after quiescent capture"))
	return fence
}

func addCraftDelegation(t *testing.T, db *gorm.DB, workspace craft.Workspace, fence agentruntime.Fence, status string) {
	t.Helper()
	payload, err := json.Marshal(craft.Task{ToolCallID: "delegate-" + fence.RunID, WorkspaceID: workspace.ID, Scope: workspace.Scope})
	require.NoError(t, err)
	err = db.Exec(`INSERT INTO craft_delegations(id, tenant_id, run_id, tool_call_id, workspace_id, prompt_message_id, request_hash, task_json, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, "delegation-"+fence.RunID, fence.TenantID, fence.RunID, "delegate-"+fence.RunID,
		workspace.ID, "prompt-"+fence.RunID, "hash-"+fence.RunID, string(payload), status).Error
	require.NoError(t, err)
}

func TestCraftDraftHeadMigrationUpDownUp(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	t.Run("postgres", func(t *testing.T) {
		dsn := os.Getenv("TRPC_TEST_POSTGRES_DSN")
		if dsn == "" {
			t.Skip("TRPC_TEST_POSTGRES_DSN unset: PostgreSQL migration proof NOT VERIFIED")
		}
		admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		schema := "craft_draft_migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
		pgdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, pgdb.Exec(`CREATE TABLE craft_workspaces (id VARCHAR(64) PRIMARY KEY, tenant_id BIGINT NOT NULL)`).Error)
		require.NoError(t, pgdb.Exec(`CREATE TABLE agent_runs (tenant_id BIGINT NOT NULL, run_id VARCHAR(64) NOT NULL, PRIMARY KEY (tenant_id, run_id))`).Error)
		singleMigrationDir := t.TempDir()
		for _, direction := range []string{"up", "down"} {
			contents, readErr := os.ReadFile(filepath.Join(root, "migrations/versioned/000248_craft_workspace_draft_head."+direction+".sql"))
			require.NoError(t, readErr)
			require.NoError(t, os.WriteFile(filepath.Join(singleMigrationDir, "000001_craft_workspace_draft_head."+direction+".sql"), contents, 0600))
		}
		conn, err := pgdb.DB()
		require.NoError(t, err)
		driver, err := pgmigrate.WithInstance(conn, &pgmigrate.Config{SchemaName: schema})
		require.NoError(t, err)
		m, err := migrate.NewWithDatabaseInstance("file://"+singleMigrationDir, "postgres", driver)
		require.NoError(t, err)
		require.NoError(t, m.Up())
		require.NoError(t, m.Down())
		require.NoError(t, m.Up())
		_, _ = m.Close()
	})
	dbPath := filepath.Join(t.TempDir(), "craft-draft-migrations.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	m, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, m.Migrate(115))
	require.NoError(t, m.Up())
	require.NoError(t, m.Down())
	require.NoError(t, m.Up())
	_, _ = m.Close()
	_ = sqlDB.Close()
}

func TestCraftDraftHeadEmptyReadIsScopedAndDurable(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			workspace := putCraftWorkspace(t, NewCraftStore(db))
			store := NewCraftDraftHeadStore(db)
			got, err := store.Read(context.Background(), workspace.Scope, workspace.ID)
			require.NoError(t, err)
			require.Equal(t, craft.DraftHeadEmpty, got.State)
			require.Zero(t, got.Revision)

			reopened := NewCraftDraftHeadStore(reopenRunDB(t, db))
			got, err = reopened.Read(context.Background(), workspace.Scope, workspace.ID)
			require.NoError(t, err)
			require.Equal(t, craft.DraftHeadEmpty, got.State)

			_, err = store.Read(context.Background(), craft.Scope{TenantID: 1, UserID: "u2", SessionID: workspace.Scope.SessionID}, workspace.ID)
			require.ErrorIs(t, err, craft.ErrForbidden)
		})
	}
}

func TestCraftDraftHeadLegacyReadRemainsDistinctFromNotFound(t *testing.T) {
	db := openCraftDB(t)
	workspace := putCraftWorkspace(t, NewCraftStore(db))
	require.NoError(t, db.Exec("DELETE FROM craft_workspace_draft_heads WHERE workspace_id = ?", workspace.ID).Error)
	_, err := NewCraftDraftHeadStore(db).Read(context.Background(), workspace.Scope, workspace.ID)
	require.ErrorIs(t, err, craft.ErrDraftHeadUnresolved)
	require.False(t, errors.Is(err, craft.ErrNotFound))
}

func TestCraftDraftHeadSQLBindsTenantAndSourceRun(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			store := NewCraftDraftHeadStore(db)
			workspace := putCraftWorkspace(t, NewCraftStore(db))
			legacy, err := NewCraftStore(db).PutWorkspace(context.Background(), craft.Workspace{
				Scope: craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s2"},
			}, 0)
			require.NoError(t, err)
			require.NoError(t, db.Exec("DELETE FROM craft_workspace_draft_heads WHERE workspace_id = ?", legacy.ID).Error)
			// A head cannot claim a tenant that differs from its parent Workspace.
			err = db.Exec(`INSERT INTO craft_workspace_draft_heads(workspace_id, tenant_id, revision, state) VALUES (?, 2, 0, 'empty')`, legacy.ID).Error
			require.Error(t, err)
			// Valid head exists; a selected revision must reference a real Run in
			// the same tenant.
			_, err = store.Read(context.Background(), workspace.Scope, workspace.ID)
			require.NoError(t, err)
			err = db.Exec(`INSERT INTO craft_workspace_draft_revisions(workspace_id, revision, tenant_id, source_run_id, manifest_digest) VALUES (?, 1, ?, 'missing-run', 'digest')`, workspace.ID, workspace.Scope.TenantID).Error
			require.Error(t, err)
		})
	}
}

func TestCraftDraftHeadSelectedReadUsesOneConsistentQuery(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			workspace := putCraftWorkspace(t, NewCraftStore(db))
			fence := seedCraftRun(t, db, "draft-source-run", "draft-call")
			file := craft.File{Path: "index.html", Ref: "object://draft/index", SHA256: strings.Repeat("a", 64), Bytes: 12, MIME: "text/html"}
			digest, err := craft.ManifestDigest([]craft.File{file})
			require.NoError(t, err)
			require.NoError(t, db.Exec(`INSERT INTO craft_workspace_draft_revisions(workspace_id, revision, tenant_id, source_run_id, manifest_digest) VALUES (?, 1, ?, ?, ?)`, workspace.ID, workspace.Scope.TenantID, fence.RunID, digest).Error)
			require.NoError(t, db.Exec(`INSERT INTO craft_workspace_draft_files(workspace_id, revision, path, object_ref, sha256, bytes, mime) VALUES (?, 1, ?, ?, ?, ?, ?)`, workspace.ID, file.Path, file.Ref, file.SHA256, file.Bytes, file.MIME).Error)
			require.NoError(t, db.Exec(`UPDATE craft_workspace_draft_heads SET revision = 1, state = 'selected', source_run_id = ?, manifest_digest = ? WHERE workspace_id = ?`, fence.RunID, digest, workspace.ID).Error)

			var selectCount atomic.Int64
			const callbackName = "craft_draft_head_test_count"
			require.NoError(t, db.Callback().Row().After("gorm:row").Register(callbackName, func(tx *gorm.DB) {
				selectCount.Add(1)
			}))
			t.Cleanup(func() { _ = db.Callback().Row().Remove(callbackName) })

			got, err := NewCraftDraftHeadStore(db).Read(context.Background(), workspace.Scope, workspace.ID)
			require.NoError(t, err)
			require.EqualValues(t, 1, got.Revision)
			require.Equal(t, fence.RunID, got.SourceRunID)
			require.Equal(t, digest, got.ManifestDigest)
			require.Equal(t, []craft.File{file}, got.Files)
			require.EqualValues(t, 1, selectCount.Load(), "authorization, head, revision and files must come from one SELECT snapshot")
		})
	}
}

func TestCraftDraftHeadAdvanceFirstRevisionSequentialAndRestart(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			workspace := putCraftWorkspace(t, NewCraftStore(db))
			store := NewCraftDraftHeadStore(db)
			run1 := terminalDraftRun(t, db, "draft-advance-a", "draft-call-a")
			files1 := []craft.File{sealedDraftFile("index.html", "object://draft/a", "a", 12)}
			first, err := store.Advance(context.Background(), workspace.Scope, workspace.ID, 0, run1.RunID, files1)
			require.NoError(t, err)
			require.EqualValues(t, 1, first.Revision)
			require.Equal(t, craft.DraftHeadSelected, first.State)
			digest1, err := craft.ManifestDigest(files1)
			require.NoError(t, err)
			require.Equal(t, digest1, first.ManifestDigest)
			require.Equal(t, files1, first.Files)
			files1[0].Ref = "object://caller/mutated"
			first.Files[0].Ref = "object://return/mutated"
			persistedFirst, err := store.Read(context.Background(), workspace.Scope, workspace.ID)
			require.NoError(t, err)
			require.Equal(t, "object://draft/a", persistedFirst.Files[0].Ref)
			files1[0].Ref = "object://draft/a"
			_, err = store.Advance(context.Background(), workspace.Scope, workspace.ID, 1, run1.RunID, files1)
			require.ErrorIs(t, err, craft.ErrConflict, "a terminal Run can produce at most one immutable draft revision")

			run2 := terminalDraftRun(t, db, "draft-advance-b", "draft-call-b")
			files2 := []craft.File{sealedDraftFile("about.html", "object://draft/b", "b", 24)}
			second, err := store.Advance(context.Background(), workspace.Scope, workspace.ID, 1, run2.RunID, files2)
			require.NoError(t, err)
			require.EqualValues(t, 2, second.Revision)
			digest2, err := craft.ManifestDigest(files2)
			require.NoError(t, err)
			require.Equal(t, digest2, second.ManifestDigest)
			require.Equal(t, files2, second.Files)

			reopened := NewCraftDraftHeadStore(reopenRunDB(t, db))
			got, err := reopened.Read(context.Background(), workspace.Scope, workspace.ID)
			require.NoError(t, err)
			require.Equal(t, second, got)
			var revisions int64
			require.NoError(t, db.Table("craft_workspace_draft_revisions").Where("workspace_id = ?", workspace.ID).Count(&revisions).Error)
			require.EqualValues(t, 2, revisions)
			var versions int64
			require.NoError(t, db.Table("craft_versions").Where("workspace_id = ?", workspace.ID).Count(&versions).Error)
			require.Zero(t, versions, "draft advancement must not publish or alter preview versions")
		})
	}
}

func TestCraftDraftHeadAdvanceSameRevisionCASLeavesOneManifest(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			workspace := putCraftWorkspace(t, NewCraftStore(db))
			fence := terminalDraftRun(t, db, "draft-cas-run", "draft-cas-call")
			store := NewCraftDraftHeadStore(db)
			files := []craft.File{sealedDraftFile("index.html", "object://draft/cas", "c", 12)}
			start := make(chan struct{})
			errs := make([]error, 2)
			results := make([]craft.DraftHead, 2)
			var wg sync.WaitGroup
			for i := range errs {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-start
					results[i], errs[i] = store.Advance(context.Background(), workspace.Scope, workspace.ID, 0, fence.RunID, files)
				}(i)
			}
			close(start)
			wg.Wait()
			winners := 0
			for _, err := range errs {
				if err == nil {
					winners++
				} else {
					require.ErrorIs(t, err, craft.ErrConflict)
				}
			}
			require.Equal(t, 1, winners)
			var revisions, storedFiles int64
			require.NoError(t, db.Table("craft_workspace_draft_revisions").Where("workspace_id = ?", workspace.ID).Count(&revisions).Error)
			require.NoError(t, db.Table("craft_workspace_draft_files").Where("workspace_id = ?", workspace.ID).Count(&storedFiles).Error)
			require.EqualValues(t, 1, revisions)
			require.EqualValues(t, 1, storedFiles)
		})
	}
}

func TestCraftDraftHeadAdvanceRollsBackHeadAndRevisionWhenFileInsertFails(t *testing.T) {
	db := openCraftDB(t)
	workspace := putCraftWorkspace(t, NewCraftStore(db))
	fence := terminalDraftRun(t, db, "draft-rollback-run", "draft-rollback-call")
	require.NoError(t, db.Exec(`CREATE TRIGGER reject_draft_file BEFORE INSERT ON craft_workspace_draft_files
		BEGIN SELECT RAISE(ABORT, 'injected draft file failure'); END`).Error)
	files := []craft.File{sealedDraftFile("index.html", "object://draft/rollback", "b", 1)}
	_, err := NewCraftDraftHeadStore(db).Advance(context.Background(), workspace.Scope, workspace.ID, 0, fence.RunID, files)
	require.Error(t, err)
	var revision int64
	require.NoError(t, db.Table("craft_workspace_draft_heads").Where("workspace_id=?", workspace.ID).Select("revision").Scan(&revision).Error)
	require.Zero(t, revision)
	var revisions, storedFiles int64
	require.NoError(t, db.Table("craft_workspace_draft_revisions").Where("workspace_id=?", workspace.ID).Count(&revisions).Error)
	require.NoError(t, db.Table("craft_workspace_draft_files").Where("workspace_id=?", workspace.ID).Count(&storedFiles).Error)
	require.Zero(t, revisions)
	require.Zero(t, storedFiles)
}

func TestCraftDraftHeadAdvanceRejectsUnsafeAndIncompleteSources(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			ctx := context.Background()
			db := openCraftDB(t)
			workspace := putCraftWorkspace(t, NewCraftStore(db))
			store := NewCraftDraftHeadStore(db)
			valid := []craft.File{sealedDraftFile("index.html", "object://draft/valid", "d", 10)}

			invalidManifests := [][]craft.File{
				{valid[0], valid[0]},
				{sealedDraftFile("../escape", "object://x", "e", 1)},
				{sealedDraftFile(".env", "object://x", "e", 1)},
				{{Path: "index.html", Ref: "object://x", SHA256: "bad", Bytes: 1}},
				{sealedDraftFile("index.html", "object://x", "e", -1)},
				{sealedDraftFile("index.html", "object://x", "e", craft.MaxDraftHeadBytes+1)},
				{{Path: "index.html", SHA256: strings.Repeat("e", 64), Bytes: 1}},
				nil,
			}
			for _, files := range invalidManifests {
				fence := terminalDraftRun(t, db, "draft-invalid-"+uuid.NewString(), "draft-call-"+uuid.NewString())
				_, err := store.Advance(ctx, workspace.Scope, workspace.ID, 0, fence.RunID, files)
				require.ErrorIs(t, err, craft.ErrInvalidInput)
			}

			for _, status := range []string{"running", "waiting_user", "unknown", "reconciling"} {
				fence := seedShortDraftRun(t, db, "draft-status-"+status, "draft-status-call-"+status)
				waitReason := ""
				if status == "reconciling" {
					waitReason = "stop_requested"
				}
				require.NoError(t, db.Exec("UPDATE agent_runs SET status = ?, wait_reason = ? WHERE tenant_id = ? AND run_id = ?", status, waitReason, fence.TenantID, fence.RunID).Error)
				_, err := store.Advance(ctx, workspace.Scope, workspace.ID, 0, fence.RunID, valid)
				require.ErrorIs(t, err, craft.ErrBusy, "status %s cannot prove a quiescent writer", status)
				require.NoError(t, db.Exec("UPDATE sessions SET active_agent_run_id = NULL WHERE tenant_id = 1 AND id='s1'").Error)
			}

			for _, delegationStatus := range []string{"prepared", "unknown"} {
				pending := seedShortDraftRun(t, db, "draft-pending-delegation-"+delegationStatus, "draft-pending-call-"+delegationStatus)
				addCraftDelegation(t, db, workspace, pending, delegationStatus)
				require.NoError(t, db.Exec("UPDATE agent_tool_calls SET status='failed' WHERE tenant_id=? AND run_id=? AND call_id=?", pending.TenantID, pending.RunID, shortDraftIdentity("draft-pending-call-"+delegationStatus)).Error)
				require.NoError(t, NewAgentRunStore(db).SetStatus(ctx, pending, "failed", "terminal with child"))
				_, err := store.Advance(ctx, workspace.Scope, workspace.ID, 0, pending.RunID, valid)
				require.ErrorIs(t, err, craft.ErrBusy)
			}

			pendingTool := seedShortDraftRun(t, db, "draft-pending-tool", "draft-pending-tool-call")
			require.NoError(t, NewAgentRunStore(db).SetStatus(ctx, pendingTool, "failed", "terminal with unknown tool"))
			_, err := store.Advance(ctx, workspace.Scope, workspace.ID, 0, pendingTool.RunID, valid)
			require.ErrorIs(t, err, craft.ErrBusy)

			terminal := terminalDraftRun(t, db, "draft-active-slot", "draft-active-slot-call")
			require.NoError(t, db.Exec("UPDATE sessions SET active_agent_run_id = ? WHERE tenant_id=1 AND id='s1'", terminal.RunID).Error)
			_, err = store.Advance(ctx, workspace.Scope, workspace.ID, 0, terminal.RunID, valid)
			require.ErrorIs(t, err, craft.ErrBusy)
			var revisions, files int64
			require.NoError(t, db.Table("craft_workspace_draft_revisions").Where("workspace_id=?", workspace.ID).Count(&revisions).Error)
			require.NoError(t, db.Table("craft_workspace_draft_files").Where("workspace_id=?", workspace.ID).Count(&files).Error)
			require.Zero(t, revisions)
			require.Zero(t, files)
		})
	}
}

func TestCraftDraftHeadAdvanceRejectsForeignRunAndReplay(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			ctx := context.Background()
			db := openCraftDB(t)
			workspace := putCraftWorkspace(t, NewCraftStore(db))
			store := NewCraftDraftHeadStore(db)
			foreign := seedShortDraftRun(t, db, "draft-foreign-run", "draft-foreign-call")
			require.NoError(t, db.Exec("UPDATE agent_tool_calls SET status='failed' WHERE tenant_id=? AND run_id=? AND call_id=?", foreign.TenantID, foreign.RunID, shortDraftIdentity("draft-foreign-call")).Error)
			require.NoError(t, NewAgentRunStore(db).SetStatus(ctx, foreign, "failed", "done"))
			require.NoError(t, db.Exec("UPDATE agent_runs SET session_id='s2' WHERE tenant_id=? AND run_id=?", foreign.TenantID, foreign.RunID).Error)
			_, err := store.Advance(ctx, workspace.Scope, workspace.ID, 0, foreign.RunID, []craft.File{sealedDraftFile("index.html", "object://draft/foreign", "f", 1)})
			require.ErrorIs(t, err, craft.ErrForbidden)
			authorizedRun := terminalDraftRun(t, db, "draft-authz-run", "draft-authz-call")
			foreignScope := workspace.Scope
			foreignScope.UserID = "u2"
			_, err = store.Advance(ctx, foreignScope, workspace.ID, 0, authorizedRun.RunID, []craft.File{sealedDraftFile("index.html", "object://draft/unauthorized", "f", 1)})
			require.ErrorIs(t, err, craft.ErrForbidden)

			valid := terminalDraftRun(t, db, "draft-replay-run", "draft-replay-call")
			files := []craft.File{sealedDraftFile("index.html", "object://draft/replay", "a", 1)}
			first, err := store.Advance(ctx, workspace.Scope, workspace.ID, 0, valid.RunID, files)
			require.NoError(t, err)
			_, err = store.Advance(ctx, workspace.Scope, workspace.ID, 0, valid.RunID, files)
			require.ErrorIs(t, err, craft.ErrConflict)
			require.NoError(t, db.Exec("UPDATE craft_workspace_draft_heads SET manifest_digest = ? WHERE workspace_id=?", strings.Repeat("0", 64), workspace.ID).Error)
			_, err = store.Read(ctx, workspace.Scope, workspace.ID)
			require.ErrorIs(t, err, craft.ErrInvalidInput, "tampered selected digest fails closed")
			require.EqualValues(t, 1, first.Revision)
		})
	}
}
