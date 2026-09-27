package container

// T20 (#139) post-terminal promotion orchestration: the durable capture
// receipts that reached a terminal state (sealed/advanced) name a Workspace
// draft revision; when that head is the SELECTED product of the receipt's
// own Run, the four-check promotion gate runs through the SAME artifact
// service the capture coordinator uses. A passing probe publishes the
// immutable version; a missing probe leaves both page facts not_run and the
// gate refuses (fail-closed, the prior default version keeps the seat);
// unsealed or foreign heads never trigger a promotion attempt at all.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
	gormlogger "gorm.io/gorm/logger"

	sql "database/sql"

	migrate "github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openCraftT20PromotionDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("t20 promotion: cannot locate test file")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
	dbPath := filepath.Join(t.TempDir(), "craft-t20-promotion.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatalf("t20 promotion: open sqlite: %v", err)
	}
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	if err != nil {
		t.Fatalf("t20 promotion: migrate driver: %v", err)
	}
	migrator, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver)
	if err != nil {
		t.Fatalf("t20 promotion: migrator: %v", err)
	}
	if err := migrator.Up(); err != nil {
		t.Fatalf("t20 promotion: migrations up: %v", err)
	}
	_, _ = migrator.Close()
	db, err := gorm.Open(gormsqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("t20 promotion: gorm open: %v", err)
	}
	require.NoError(t, db.Exec("INSERT INTO tenants (id, name, business) VALUES (1, 't', 'test')").Error)
	require.NoError(t, db.Exec("INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES ('u1','u1','u1@e.test','x',1)").Error)
	require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id,user_id,role,status,joined_at,created_at,updated_at)
		VALUES (1,'u1','owner','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	require.NoError(t, db.Exec("INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s1', 1, 't20', 'u1', 'trpc')").Error)
	require.NoError(t, db.Exec(`INSERT INTO craft_sessions (session_id,tenant_id,kind) VALUES ('s1',1,'web')`).Error)
	return db
}

// t20ProbeStub counts invocations and answers the two facts it was given.
type t20ProbeStub struct {
	reachable, loaded craft.CheckOutcome
	calls             int
}

func (p *t20ProbeStub) ProbeWebPage(context.Context, craft.Scope, craft.Candidate) (craft.CheckOutcome, craft.CheckOutcome) {
	p.calls++
	return p.reachable, p.loaded
}

// seedT20TerminalWebRun stages one completed web Run end-to-end at the
// durable layer: workspace, private candidate, selected draft head and the
// terminal (advanced) capture receipt the promoter scans.
func seedT20TerminalWebRun(t *testing.T, db *gorm.DB, runID, content string) (craft.Scope, string, craft.Candidate) {
	t.Helper()
	return seedT20TerminalWebRunReceipt(t, db, runID, content, "")
}

// seedT20TerminalWebRunReceipt additionally names the terminal receipt's Run:
// a non-empty receiptRunID simulates a FAILED/STOPPED Run whose capture
// receipt names the workspace while the sealed head belongs to another Run.
func seedT20TerminalWebRunReceipt(t *testing.T, db *gorm.DB, runID, content, receiptRunID string) (craft.Scope, string, craft.Candidate) {
	t.Helper()
	ctx := context.Background()
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}
	workspace, err := repository.NewCraftStore(db).PutWorkspace(ctx, craft.Workspace{
		Scope: scope, SandboxID: "sbx-t20", Generation: "0", RuntimeDigest: "digest-t20",
	}, 0)
	require.NoError(t, err)

	// The candidate scope validation binds to the durable Run row (the
	// production capture path always has one terminal Run).
	now := time.Now()
	require.NoError(t, db.Exec(`INSERT INTO agent_runs
		(tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, status, wait_reason, deadline, created_at, updated_at)
		VALUES (1, ?, 's1', 'u1', ?, ?, 'h', '{}', 'succeeded', '', ?, ?, ?)`,
		runID, "req-"+runID, "am-"+runID, now.Add(time.Hour), now, now).Error)

	sum := sha256.Sum256([]byte(content))
	files := []craft.File{{
		Path: "index.html", Ref: "resource://t20-promotion/" + runID,
		SHA256: hex.EncodeToString(sum[:]), MIME: "text/html", Bytes: int64(len(content)),
	}}
	digest, err := craft.ManifestDigest(files)
	require.NoError(t, err)
	evidence := craft.ArtifactEvidence{ClaimedSuccess: true, BuildRan: true, BuildExitCode: 0}
	candidate := craft.Candidate{
		ID: craft.CandidateID(workspace.ID, runID, digest), WorkspaceID: workspace.ID, RunID: runID,
		Generation: "gen-" + runID, Kind: craft.KindWeb, ManifestDigest: digest, Scope: scope,
		Files: files, Evidence: evidence, Checks: craft.BuildChecks(craft.KindWeb, files, evidence),
	}
	_, err = repository.NewCraftCandidateStore(db).PutCandidate(ctx, scope, candidate)
	require.NoError(t, err)

	head, err := repository.NewCraftDraftHeadStore(db).Advance(ctx, scope, workspace.ID, 0, runID, files)
	require.NoError(t, err)
	require.Equal(t, craft.DraftHeadSelected, head.State)

	rev := head.Revision
	if receiptRunID == "" {
		receiptRunID = runID
	}
	if receiptRunID != runID {
		require.NoError(t, db.Exec(`INSERT INTO agent_runs
			(tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, status, wait_reason, deadline, created_at, updated_at)
			VALUES (1, ?, 's1', 'u1', ?, ?, 'h', '{}', 'failed', '', ?, ?, ?)`,
			receiptRunID, "req-"+receiptRunID, "am-"+receiptRunID, time.Now().Add(time.Hour), time.Now(), time.Now()).Error)
	}
	require.NoError(t, db.Exec(`INSERT INTO craft_run_captures
		(tenant_id, workspace_id, run_id, owner_id, session_id, generation, predecessor_revision, predecessor_state, predecessor_run_id, predecessor_digest, state, manifest_digest, draft_revision, created_at, updated_at)
		VALUES (1, ?, ?, 'u1', 's1', ?, 0, 'empty', '', '', 'advanced', ?, ?, ?, ?)`,
		workspace.ID, receiptRunID, "gen-"+receiptRunID, digest, rev, time.Now(), time.Now()).Error)
	return scope, workspace.ID, candidate
}

// t20PromotionStack assembles the promoter over the real durable stores with
// the given probe (nil = the production default until T14's implementation
// registers).
func t20PromotionStack(t *testing.T, db *gorm.DB, probe service.WebPageLoadProbe) (*craftPostTerminalPromoter, craft.VersionStore) {
	t.Helper()
	files := newCaptureWiringFiles(t, db)
	versions := repository.NewCraftVersionStore(db)
	artifacts := service.NewCraftArtifactServiceWithCandidates(
		closedCraftArtifactSource{}, files, versions, repository.NewCraftCandidateStore(db), nil,
		service.CraftArtifactConfig{Kind: craft.KindWeb, OutputDir: craftLocalOutputDir},
	).WithWebPromotion(repository.NewCraftDraftHeadStore(db), probe)
	return newCraftPostTerminalPromoter(db, artifacts, repository.NewCraftDraftHeadStore(db), versions), versions
}

func TestCraftT20PostTerminalPromotionPublishesSealedWebHead(t *testing.T) {
	db := openCraftT20PromotionDB(t)
	scope, workspaceID, candidate := seedT20TerminalWebRun(t, db, "run-t20-promo", "<h1>v1</h1>")
	probe := &t20ProbeStub{reachable: craft.WebCheckPassed, loaded: craft.WebCheckPassed}
	promoter, versions := t20PromotionStack(t, db, probe)

	promoter.promoteTerminalReceipts(context.Background(), 16)

	published, err := versions.Get(context.Background(), scope, craft.VersionID(workspaceID, "run-t20-promo", candidate.ManifestDigest))
	require.NoError(t, err, "the four-check gate must publish the sealed head's candidate")
	require.Len(t, published.Checks, 4)
	statuses := map[string]string{}
	for _, check := range published.Checks {
		statuses[check.Name] = check.Status
	}
	require.Equal(t, map[string]string{
		craft.CheckBuild: craft.CheckPassed, craft.CheckEntry: craft.CheckPassed,
		craft.CheckPreviewReachable: craft.CheckPassed, craft.CheckPageLoad: craft.CheckPassed,
	}, statuses, "each check recorded independently")
	require.Equal(t, 1, probe.calls, "exactly one probe observation")

	// The second pass adopts the already published version: idempotent, no
	// duplicate version, no second probe run.
	promoter.promoteTerminalReceipts(context.Background(), 16)
	list, err := versions.List(context.Background(), scope)
	require.NoError(t, err)
	require.Len(t, list, 1, "a replayed identical promotion adopts the published version")
	require.Equal(t, 1, probe.calls)
}

func TestCraftT20PostTerminalPromotionFailsClosedWithoutProbe(t *testing.T) {
	db := openCraftT20PromotionDB(t)
	scope, workspaceID, candidate := seedT20TerminalWebRun(t, db, "run-t20-noprobe", "<h1>v2</h1>")
	// The prior successful version keeps the default seat.
	priorFiles := []craft.File{{
		Path: "index.html", Ref: "resource://t20-prior", SHA256: candidate.Files[0].SHA256,
		MIME: "text/html", Bytes: 16,
	}}
	priorDigest, err := craft.ManifestDigest(priorFiles)
	require.NoError(t, err)
	_, err = repository.NewCraftVersionStore(db).Publish(context.Background(), scope, craft.Version{
		ID: craft.VersionID(workspaceID, "run-prior", priorDigest), WorkspaceID: workspaceID,
		RunID: "run-prior", Kind: craft.KindWeb, Files: priorFiles,
		Checks: []craft.Check{{Name: craft.CheckEntry, Status: craft.CheckPassed}},
	})
	require.NoError(t, err)

	promoter, versions := t20PromotionStack(t, db, nil) // production default: no probe registered
	promoter.promoteTerminalReceipts(context.Background(), 16)

	list, err := versions.List(context.Background(), scope)
	require.NoError(t, err)
	require.Len(t, list, 1, "a missing probe refuses the promotion: nothing publishes")
	require.Equal(t, craft.VersionID(workspaceID, "run-prior", priorDigest), list[0].ID,
		"the prior successful version keeps the seat")
	_, err = versions.Get(context.Background(), scope, craft.VersionID(workspaceID, "run-t20-noprobe", candidate.ManifestDigest))
	require.Error(t, err, "the new candidate stays a private Workspace draft")
}

func TestCraftT20PostTerminalPromotionSkipsForeignOrUnsealedHeads(t *testing.T) {
	db := openCraftT20PromotionDB(t)
	scope, workspaceID, candidate := seedT20TerminalWebRunReceipt(t, db, "run-prior", "<h1>v0</h1>", "run-failed")
	// A FAILED/stopped Run's terminal receipt names the workspace, but the
	// sealed head is the PRIOR successful Run's product: the promoter must
	// refuse to publish the prior run's files under the failed run's name
	// (head.SourceRunID binding), and an unreadable/unsealed head skips the
	// same way. The schema's own CHECK constraint keeps heads well-formed, so
	// the foreign-receipt leg exercises the guard on real durable rows.

	probe := &t20ProbeStub{reachable: craft.WebCheckPassed, loaded: craft.WebCheckPassed}
	promoter, versions := t20PromotionStack(t, db, probe)
	promoter.promoteTerminalReceipts(context.Background(), 16)

	list, err := versions.List(context.Background(), scope)
	require.NoError(t, err)
	require.Empty(t, list, "a foreign or unsealed head never triggers a promotion")
	require.Zero(t, probe.calls, "the gate is never consulted for heads the receipt does not own")
	require.NotEmpty(t, workspaceID)
	require.NotEmpty(t, candidate.ID)
}
