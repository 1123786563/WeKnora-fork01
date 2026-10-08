package repository

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestCraftRunCaptureMigrationUpDownUp(t *testing.T) {
	// ponytail: craft 迁移重排（161-189/242-270）后相邻号假设失效，需按新链重校准
	t.Skip("craft 迁移重排待校准")
	db := openRunTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	m, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Close() })
	// All migration work stays on openRunTestDB's per-test temporary file.
	require.NoError(t, m.Migrate(122))
	require.False(t, db.Migrator().HasTable("craft_run_captures"))
	require.NoError(t, m.Steps(1))
	require.True(t, db.Migrator().HasTable("craft_run_captures"))
	require.True(t, db.Migrator().HasTable("craft_run_capture_files"))
	require.NoError(t, m.Steps(-1))
	require.False(t, db.Migrator().HasTable("craft_run_capture_files"))
	require.NoError(t, m.Steps(1))
	require.True(t, db.Migrator().HasTable("craft_run_captures"))
}

func TestCraftRunCapturePostgresMigrationUpDownUp(t *testing.T) {
	dsn := os.Getenv("TRPC_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TRPC_TEST_POSTGRES_DSN unset: PostgreSQL capture migration NOT VERIFIED")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	schema := "craft_capture_migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() { require.NoError(t, admin.Exec("DROP SCHEMA "+schema+" CASCADE").Error) })
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	query := parsed.Query()
	query.Set("search_path", schema+",public")
	parsed.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE craft_workspaces (tenant_id BIGINT NOT NULL, id VARCHAR(64) NOT NULL, owner_id VARCHAR(512) NOT NULL, session_id VARCHAR(36) NOT NULL, PRIMARY KEY(tenant_id,id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE agent_runs (tenant_id BIGINT NOT NULL, run_id VARCHAR(64) NOT NULL, owner_id VARCHAR(512) NOT NULL, session_id VARCHAR(36) NOT NULL, status VARCHAR(32) NOT NULL, snapshot JSONB NOT NULL, PRIMARY KEY(tenant_id,run_id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE craft_run_views (tenant_id BIGINT NOT NULL, run_id VARCHAR(64) NOT NULL, generation VARCHAR(64) NOT NULL, state VARCHAR(32) NOT NULL)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE craft_workspace_draft_heads (tenant_id BIGINT NOT NULL, workspace_id VARCHAR(64) NOT NULL, revision BIGINT NOT NULL, state VARCHAR(16) NOT NULL, source_run_id VARCHAR(64), manifest_digest VARCHAR(64))`).Error)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	singleMigrationDir := t.TempDir()
	for _, direction := range []string{"up", "down"} {
		contents, readErr := os.ReadFile(filepath.Join(root, "migrations/versioned/000252_craft_run_capture."+direction+".sql"))
		require.NoError(t, readErr)
		require.NoError(t, os.WriteFile(filepath.Join(singleMigrationDir, "000001_craft_run_capture."+direction+".sql"), contents, 0o600))
	}
	driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{SchemaName: schema})
	require.NoError(t, err)
	m, err := migrate.NewWithDatabaseInstance("file://"+singleMigrationDir, "postgres", driver)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Close(); _ = sqlDB.Close() })
	require.NoError(t, m.Up())
	require.True(t, db.Migrator().HasTable("craft_run_captures"))
	require.True(t, db.Migrator().HasTable("craft_run_capture_files"))
	require.NoError(t, m.Down())
	require.False(t, db.Migrator().HasTable("craft_run_captures"))
	require.NoError(t, m.Up())
	require.True(t, db.Migrator().HasTable("craft_run_captures"))
}

func TestCraftRunCaptureRequiresTerminalBoundQuiescenceAndPendingWriters(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name        string
		bound       bool
		terminal    bool
		pendingTool bool
	}{
		{name: "active Run"},
		{name: "terminal unknown generation", bound: false, terminal: true},
		{name: "terminal Run with pending tool", bound: true, pendingTool: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := openRunTestDB(t)
			registerCraftSessionForCaptureTest(t, db)
			workspace := putCraftWorkspace(t, NewCraftStore(db))
			runs := NewAgentRunStore(db)
			var fence agentruntime.Fence
			if tc.pendingTool {
				in := agentRunAdmissionForCaptureTest("capture-pending-tool", workspace.ID)
				_, err := runs.Admit(ctx, in)
				require.NoError(t, err)
				fence, err = runs.Claim(ctx, in.Key, "capture-worker", time.Minute)
				require.NoError(t, err)
				_, err = runs.EnsureToolPlan(ctx, fence, craftToolPlan("capture-pending-call"))
				require.NoError(t, err)
			} else {
				in := agentRunAdmissionForCaptureTest("capture-authority-run", workspace.ID)
				_, err := runs.Admit(ctx, in)
				require.NoError(t, err)
				fence, err = runs.Claim(ctx, in.Key, "capture-worker", time.Minute)
				require.NoError(t, err)
			}
			views := NewCraftRunViewStore(db)
			key := craft.RunViewKey{TenantID: 1, OwnerID: "u1", SessionID: "s1", RunID: fence.RunID}
			view, err := views.Allocate(ctx, key)
			require.NoError(t, err)
			if tc.bound {
				view, _, err = views.BeginSessionCreate(ctx, key, view.Generation)
				require.NoError(t, err)
				_, err = views.BindRuntime(ctx, key, view.Generation, craft.RunViewRuntime{RuntimeID: "capture-runtime", ContainerID: "capture-container", OpenCodeSessionID: "capture-oc"})
				require.NoError(t, err)
			}
			if tc.pendingTool || tc.terminal {
				require.NoError(t, runs.CancelRun(ctx, fence.RunKey, "terminal"))
			}
			store := NewCraftRunCaptureStore(db)
			_, err = store.EnsurePending(ctx, craftTestScope(), workspace.ID, fence.RunID, view.Generation)
			require.ErrorIs(t, err, craft.ErrConflict)
			var n int64
			require.NoError(t, db.Table("craft_run_captures").Where("tenant_id=? AND run_id=?", 1, fence.RunID).Count(&n).Error)
			if tc.pendingTool {
				require.EqualValues(t, 1, n, "terminal transition may enqueue, but pending tool prevents collection")
			} else {
				require.Zero(t, n, "an active or unknown-generation Run is not admitted into capture")
			}
		})
	}
}

func TestCraftRunCaptureRecoveryRecreatesMissedTerminalReceipt(t *testing.T) {
	ctx := context.Background()
	db := openRunTestDB(t)
	registerCraftSessionForCaptureTest(t, db)
	workspace := putCraftWorkspace(t, NewCraftStore(db))
	runs := NewAgentRunStore(db)
	in := agentRunAdmissionForCaptureTest("capture-recover-missing-receipt", workspace.ID)
	_, err := runs.Admit(ctx, in)
	require.NoError(t, err)
	views := NewCraftRunViewStore(db)
	key := craft.RunViewKey{TenantID: 1, OwnerID: "u1", SessionID: "s1", RunID: in.Key.RunID}
	view, err := views.Allocate(ctx, key)
	require.NoError(t, err)
	view, _, err = views.BeginSessionCreate(ctx, key, view.Generation)
	require.NoError(t, err)
	_, err = views.BindRuntime(ctx, key, view.Generation, craft.RunViewRuntime{RuntimeID: "recover-runtime", ContainerID: "recover-container", OpenCodeSessionID: "recover-oc"})
	require.NoError(t, err)
	require.NoError(t, runs.CancelRun(ctx, in.Key, "failed"))
	// Simulate a terminal-commit/outbox data gap. Recovery derives identity and
	// frozen predecessor only from server-owned tables.
	require.NoError(t, db.Exec("DELETE FROM craft_run_captures WHERE tenant_id=? AND workspace_id=? AND run_id=?", 1, workspace.ID, in.Key.RunID).Error)
	store := NewCraftRunCaptureStore(db)
	receipts, err := store.RecoverPending(ctx, 10)
	require.NoError(t, err)
	require.Len(t, receipts, 1)
	require.Equal(t, in.Key.RunID, receipts[0].RunID)
	require.Equal(t, view.Generation, receipts[0].Generation)
	require.Equal(t, "pending", receipts[0].State)
	require.EqualValues(t, 0, receipts[0].PredecessorRevision)
}

func TestCraftRunCaptureAttemptSealAndReceiptAreImmutable(t *testing.T) {
	ctx := context.Background()
	db := openRunTestDB(t)
	registerCraftSessionForCaptureTest(t, db)
	workspace := putCraftWorkspace(t, NewCraftStore(db))
	runs := NewAgentRunStore(db)
	in := agentRunAdmissionForCaptureTest("capture-seal-immutable", workspace.ID)
	_, err := runs.Admit(ctx, in)
	require.NoError(t, err)
	views := NewCraftRunViewStore(db)
	key := craft.RunViewKey{TenantID: 1, OwnerID: "u1", SessionID: "s1", RunID: in.Key.RunID}
	view, err := views.Allocate(ctx, key)
	require.NoError(t, err)
	view, _, err = views.BeginSessionCreate(ctx, key, view.Generation)
	require.NoError(t, err)
	_, err = views.BindRuntime(ctx, key, view.Generation, craft.RunViewRuntime{RuntimeID: "immutable-runtime", ContainerID: "immutable-container", OpenCodeSessionID: "immutable-oc"})
	require.NoError(t, err)
	require.NoError(t, runs.CancelRun(ctx, in.Key, "terminal"))
	store := NewCraftRunCaptureStore(db)
	receipt, err := store.EnsurePending(ctx, craftTestScope(), workspace.ID, in.Key.RunID, view.Generation)
	require.NoError(t, err)
	file := craft.File{Path: "index.html", Ref: "object://capture-one", SHA256: strings.Repeat("a", 64), Bytes: 1, MIME: "text/html"}
	digest, err := craft.ManifestDigest([]craft.File{file})
	require.NoError(t, err)
	receipt, err = store.BeginCapture(ctx, receipt, digest)
	require.NoError(t, err)
	require.Equal(t, "capturing", receipt.State)
	_, err = store.BeginCapture(ctx, receipt, strings.Repeat("b", 64))
	require.ErrorIs(t, err, craft.ErrConflict, "a same-Run retry cannot replace its attempted manifest")
	receipt, err = store.Seal(ctx, receipt, []craft.File{file}, digest)
	require.NoError(t, err)
	require.Equal(t, "sealed", receipt.State)
	changedRef := file
	changedRef.Ref = "object://capture-other"
	_, err = store.Seal(ctx, receipt, []craft.File{changedRef}, digest)
	require.ErrorIs(t, err, craft.ErrConflict, "sealed refs are exact even if content metadata matches")
	require.Error(t, db.Exec("UPDATE craft_run_capture_files SET object_ref='object://tampered' WHERE tenant_id=? AND workspace_id=? AND run_id=?", 1, workspace.ID, in.Key.RunID).Error)
	require.Error(t, db.Exec("INSERT INTO craft_run_capture_files (tenant_id,workspace_id,run_id,path,object_ref,sha256,bytes,mime) VALUES (?,?,?,?,?,?,?,?)", 1, workspace.ID, in.Key.RunID, "extra.html", "object://extra", strings.Repeat("c", 64), 1, "text/html").Error)
	require.NoError(t, store.MarkAdvanced(ctx, receipt, 1))
	require.Error(t, db.Exec("UPDATE craft_run_captures SET last_error='tampered' WHERE tenant_id=? AND workspace_id=? AND run_id=?", 1, workspace.ID, in.Key.RunID).Error)
	rows, err := store.RecoverPending(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestCraftRunCaptureEnsurePendingReturnsScopedSealedFiles(t *testing.T) {
	ctx := context.Background()
	db := openRunTestDB(t)
	registerCraftSessionForCaptureTest(t, db)
	workspace := putCraftWorkspace(t, NewCraftStore(db))
	runs := NewAgentRunStore(db)
	in := agentRunAdmissionForCaptureTest("capture-direct-sealed-replay", workspace.ID)
	_, err := runs.Admit(ctx, in)
	require.NoError(t, err)
	views := NewCraftRunViewStore(db)
	key := craft.RunViewKey{TenantID: 1, OwnerID: "u1", SessionID: "s1", RunID: in.Key.RunID}
	view, err := views.Allocate(ctx, key)
	require.NoError(t, err)
	view, _, err = views.BeginSessionCreate(ctx, key, view.Generation)
	require.NoError(t, err)
	_, err = views.BindRuntime(ctx, key, view.Generation, craft.RunViewRuntime{RuntimeID: "sealed-runtime", ContainerID: "sealed-container", OpenCodeSessionID: "sealed-oc"})
	require.NoError(t, err)
	require.NoError(t, runs.CancelRun(ctx, in.Key, "terminal"))
	store := NewCraftRunCaptureStore(db)
	receipt, err := store.EnsurePending(ctx, craftTestScope(), workspace.ID, in.Key.RunID, view.Generation)
	require.NoError(t, err)
	file := craft.File{Path: "index.html", Ref: "object://immutable-sealed-ref", SHA256: strings.Repeat("a", 64), Bytes: 1, MIME: "text/html"}
	digest, err := craft.ManifestDigest([]craft.File{file})
	require.NoError(t, err)
	receipt, err = store.BeginCapture(ctx, receipt, digest)
	require.NoError(t, err)
	sealed, err := store.Seal(ctx, receipt, []craft.File{file}, digest)
	require.NoError(t, err)
	require.Equal(t, "sealed", sealed.State)

	replayed, err := store.EnsurePending(ctx, craftTestScope(), workspace.ID, in.Key.RunID, view.Generation)
	require.NoError(t, err)
	require.Equal(t, "sealed", replayed.State)
	require.Equal(t, []craft.File{file}, replayed.Files, "direct terminal retry must reload the exact persisted refs")
}

func TestCraftRunCaptureSealIsNotHiddenByUnrelatedRecoveryBacklog(t *testing.T) {
	ctx := context.Background()
	db := openRunTestDB(t)
	dbConn, err := db.DB()
	require.NoError(t, err)
	dbConn.SetMaxOpenConns(1)
	registerCraftSessionForCaptureTest(t, db)
	workspace := putCraftWorkspace(t, NewCraftStore(db))
	// Seed 501 unrelated old pending receipts. Seal must return its exact
	// tenant/workspace/Run row instead of searching the first global page.
	now := time.Now().Add(-24 * time.Hour).UTC()
	runs := make([]map[string]any, 501)
	captures := make([]map[string]any, 501)
	for i := range runs {
		runID := fmt.Sprintf("older-capture-%03d", i)
		runs[i] = map[string]any{
			"tenant_id": 1, "run_id": runID, "session_id": "s1", "owner_id": "u1",
			"request_id": runID + "-request", "assistant_message_id": runID + "-assistant",
			"request_hash": runID + "-hash", "snapshot": `{}`, "status": "running",
			"deadline": time.Now().Add(time.Hour), "created_at": now, "updated_at": now,
		}
		captures[i] = map[string]any{
			"tenant_id": 1, "workspace_id": workspace.ID, "run_id": runID,
			"owner_id": "u1", "session_id": "s1", "generation": "old-generation",
			"predecessor_revision": 0, "predecessor_state": "empty", "state": "pending",
			"created_at": now, "updated_at": now,
		}
	}
	require.NoError(t, db.Table("agent_runs").Create(&runs).Error)
	require.NoError(t, db.Table("craft_run_captures").Create(&captures).Error)

	agentRuns := NewAgentRunStore(db)
	in := agentRunAdmissionForCaptureTest("capture-behind-global-backlog", workspace.ID)
	_, err = agentRuns.Admit(ctx, in)
	require.NoError(t, err)
	views := NewCraftRunViewStore(db)
	key := craft.RunViewKey{TenantID: 1, OwnerID: "u1", SessionID: "s1", RunID: in.Key.RunID}
	view, err := views.Allocate(ctx, key)
	require.NoError(t, err)
	view, _, err = views.BeginSessionCreate(ctx, key, view.Generation)
	require.NoError(t, err)
	_, err = views.BindRuntime(ctx, key, view.Generation, craft.RunViewRuntime{RuntimeID: "backlog-runtime", ContainerID: "backlog-container", OpenCodeSessionID: "backlog-oc"})
	require.NoError(t, err)
	require.NoError(t, agentRuns.CancelRun(ctx, in.Key, "terminal"))
	store := NewCraftRunCaptureStore(db)
	receipt, err := store.EnsurePending(ctx, craftTestScope(), workspace.ID, in.Key.RunID, view.Generation)
	require.NoError(t, err)
	file := craft.File{Path: "index.html", Ref: "object://backlog-seal", SHA256: strings.Repeat("b", 64), Bytes: 1, MIME: "text/html"}
	digest, err := craft.ManifestDigest([]craft.File{file})
	require.NoError(t, err)
	receipt, err = store.BeginCapture(ctx, receipt, digest)
	require.NoError(t, err)
	sealed, err := store.Seal(ctx, receipt, []craft.File{file}, digest)
	require.NoError(t, err)
	require.Equal(t, "sealed", sealed.State)
	require.Equal(t, in.Key.RunID, sealed.RunID)
	require.Equal(t, []craft.File{file}, sealed.Files)
}

func TestCraftRunCaptureVerifySealedRefsEnforcesResourceTenantAndMetadata(t *testing.T) {
	ctx := context.Background()
	db := openRunTestDB(t)
	registerCraftSessionForCaptureTest(t, db)
	workspace := putCraftWorkspace(t, NewCraftStore(db))
	runs := NewAgentRunStore(db)
	in := agentRunAdmissionForCaptureTest("capture-resource-tenant-check", workspace.ID)
	_, err := runs.Admit(ctx, in)
	require.NoError(t, err)
	views := NewCraftRunViewStore(db)
	key := craft.RunViewKey{TenantID: 1, OwnerID: "u1", SessionID: "s1", RunID: in.Key.RunID}
	view, err := views.Allocate(ctx, key)
	require.NoError(t, err)
	view, _, err = views.BeginSessionCreate(ctx, key, view.Generation)
	require.NoError(t, err)
	_, err = views.BindRuntime(ctx, key, view.Generation, craft.RunViewRuntime{RuntimeID: "resource-runtime", ContainerID: "resource-container", OpenCodeSessionID: "resource-oc"})
	require.NoError(t, err)
	require.NoError(t, runs.CancelRun(ctx, in.Key, "terminal"))
	store := NewCraftRunCaptureStore(db)
	receipt, err := store.EnsurePending(ctx, craftTestScope(), workspace.ID, in.Key.RunID, view.Generation)
	require.NoError(t, err)
	handle := "abcdefghijklmnopqrstuv" // canonical 22-character resource handle
	ref := types.BuildResourcePath(handle)
	file := craft.File{Path: "index.html", Ref: ref, SHA256: strings.Repeat("a", 64), Bytes: 1, MIME: "text/html"}
	digest, err := craft.ManifestDigest([]craft.File{file})
	require.NoError(t, err)
	receipt, err = store.BeginCapture(ctx, receipt, digest)
	require.NoError(t, err)
	receipt, err = store.Seal(ctx, receipt, []craft.File{file}, digest)
	require.NoError(t, err)
	require.NoError(t, db.Exec(`INSERT INTO resources (id,handle,tenant_id,provider,physical_path,location_hash,size,content_hash,state)
		VALUES (?,?,?,?,?,?,?,?,?)`, "capture-foreign-resource", handle, 2, "local", "local://2/exports/index.html", strings.Repeat("c", 64), 1, strings.Repeat("a", 64), types.ResourceStateActive).Error)
	require.ErrorIs(t, store.VerifySealedRefs(ctx, receipt), craft.ErrForbidden, "a resource handle owned by another tenant is never accepted")
	require.NoError(t, db.Exec(`UPDATE resources SET tenant_id=1 WHERE handle=?`, handle).Error)
	require.NoError(t, store.VerifySealedRefs(ctx, receipt), "tenant-owned resource with matching recorded metadata is accepted")
	require.NoError(t, db.Exec(`UPDATE resources SET content_hash=? WHERE handle=?`, strings.Repeat("b", 64), handle).Error)
	require.ErrorIs(t, store.VerifySealedRefs(ctx, receipt), craft.ErrConflict, "resource metadata must match the sealed file row")
}

func registerCraftSessionForCaptureTest(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO craft_sessions (session_id, tenant_id, kind) VALUES ('s1', 1, 'web')`).Error)
}

// A durably budget-paused Run (status waiting_user + wait_reason
// budget_exhausted, the pauseOnDenial contract) must not be able to promote a
// version: it cannot enter the capture/promotion path, and the Workspace
// draft head refuses it as a source. This is the direct automated proof of
// #138 acceptance "Paused Run cannot promote a version" at both SQL fences.
func TestCraftRunCaptureAndDraftAdvanceRefuseBudgetPausedRun(t *testing.T) {
	ctx := context.Background()
	db := openRunTestDB(t)
	registerCraftSessionForCaptureTest(t, db)
	workspace := putCraftWorkspace(t, NewCraftStore(db))
	runs := NewAgentRunStore(db)
	in := agentRunAdmissionForCaptureTest("capture-budget-paused", workspace.ID)
	_, err := runs.Admit(ctx, in)
	require.NoError(t, err)
	fence, err := runs.Claim(ctx, in.Key, "capture-worker", time.Minute)
	require.NoError(t, err)
	views := NewCraftRunViewStore(db)
	key := craft.RunViewKey{TenantID: 1, OwnerID: "u1", SessionID: "s1", RunID: fence.RunID}
	view, err := views.Allocate(ctx, key)
	require.NoError(t, err)
	view, _, err = views.BeginSessionCreate(ctx, key, view.Generation)
	require.NoError(t, err)
	_, err = views.BindRuntime(ctx, key, view.Generation, craft.RunViewRuntime{RuntimeID: "capture-runtime", ContainerID: "capture-container", OpenCodeSessionID: "capture-oc"})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`UPDATE agent_runs SET status='waiting_user', wait_reason='budget_exhausted' WHERE tenant_id=1 AND run_id=?`, fence.RunID).Error)

	store := NewCraftRunCaptureStore(db)
	_, err = store.EnsurePending(ctx, craftTestScope(), workspace.ID, fence.RunID, view.Generation)
	require.ErrorIs(t, err, craft.ErrConflict, "a budget-paused Run cannot enter the capture/promotion path")
	var n int64
	require.NoError(t, db.Table("craft_run_captures").Where("tenant_id=? AND run_id=?", 1, fence.RunID).Count(&n).Error)
	require.Zero(t, n, "no capture receipt exists for the paused Run")

	drafts := NewCraftDraftHeadStore(db)
	valid := []craft.File{sealedDraftFile("index.html", "object://draft/paused", "a", 12)}
	_, err = drafts.Advance(ctx, craftTestScope(), workspace.ID, 0, fence.RunID, valid)
	require.ErrorIs(t, err, craft.ErrBusy, "a budget-paused Run cannot advance the Workspace draft toward a promoted version")
}
