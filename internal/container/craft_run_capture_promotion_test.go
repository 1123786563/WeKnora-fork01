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
	"fmt"
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

// f08CaptureReceiptFence is the F08 T-2 promotion receipt fence double for
// capture-coordinator journeys (the real fence is proven in
// craft_web_build_dispatch_test.go).
type f08CaptureReceiptFence struct{}

func (f08CaptureReceiptFence) VerifyPromotionBuild(context.Context, craft.Scope, string, string, string) error {
	return nil
}

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

type t20BlockingProbe struct {
	started chan struct{}
	release chan struct{}
}

func (p *t20BlockingProbe) ProbeWebPage(context.Context, craft.Scope, craft.Candidate) (craft.CheckOutcome, craft.CheckOutcome) {
	close(p.started)
	<-p.release
	return craft.WebCheckPassed, craft.WebCheckPassed
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
	).WithWebPromotion(repository.NewCraftDraftHeadStore(db), probe).WithWebBuildReceipt(f08CaptureReceiptFence{})
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

func TestCraftT20PromotionRecoveryPagesPastOlderReceipts(t *testing.T) {
	db := openCraftT20PromotionDB(t)
	probe := &t20ProbeStub{reachable: craft.WebCheckPassed, loaded: craft.WebCheckPassed}
	promoter, versions := t20PromotionStack(t, db, probe)
	scope, ws, candidate := seedT20TerminalWebRun(t, db, "run-page-newest", "<h1>newest</h1>")
	for i := 0; i < craftPromotionScanLimit; i++ {
		runID := fmt.Sprintf("run-page-old-%02d", i)
		created := time.Now().Add(-time.Hour + time.Duration(i)*time.Second)
		require.NoError(t, db.Exec(`INSERT INTO agent_runs
			(tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, status, wait_reason, deadline, created_at, updated_at)
			VALUES (1, ?, 's1', 'u1', ?, ?, 'h', '{}', 'failed', '', ?, ?, ?)`,
			runID, "req-"+runID, "am-"+runID, created.Add(time.Hour), created, created).Error)
		require.NoError(t, db.Exec(`INSERT INTO craft_run_captures
			(tenant_id, workspace_id, run_id, owner_id, session_id, generation, predecessor_revision, predecessor_state, predecessor_run_id, predecessor_digest, state, manifest_digest, draft_revision, created_at, updated_at)
			VALUES (1, ?, ?, 'u1', 's1', ?, 0, 'empty', '', '', 'advanced', ?, 1, ?, ?)`,
			ws, runID, "gen-"+runID, candidate.ManifestDigest, created, created).Error)
	}
	promoter.promoteTerminalReceipts(context.Background(), craftPromotionScanLimit)
	// A new process has no shared Go cursor; durable cursor state must continue
	// from the prior page and reach the newer receipt.
	promoter, versions = t20PromotionStack(t, db, probe)
	promoter.promoteTerminalReceipts(context.Background(), craftPromotionScanLimit)
	_, err := versions.Get(context.Background(), scope, craft.VersionID(ws, candidate.RunID, candidate.ManifestDigest))
	require.NoError(t, err, "bounded repeated scans must advance past old terminal receipts")
}

func TestCraftT20PromotionFailureCooldownIsBoundedAndRetryable(t *testing.T) {
	db := openCraftT20PromotionDB(t)
	scope, _, candidate := seedT20TerminalWebRun(t, db, "run-cooldown", "<h1>retry</h1>")
	captures := repository.NewCraftRunCaptureStore(db)
	first, err := captures.ClaimPromotionForRun(context.Background(), scope.TenantID, candidate.RunID, time.Minute)
	require.NoError(t, err)
	require.Len(t, first, 1)
	second, err := captures.ClaimPromotionForRun(context.Background(), scope.TenantID, candidate.RunID, time.Minute)
	require.NoError(t, err)
	require.Empty(t, second, "a failed candidate stays out of immediate retry loops")
	require.NoError(t, db.Exec(`UPDATE craft_run_capture_promotion_attempts SET retry_after = datetime('now', '-1 minute') WHERE tenant_id = ? AND workspace_id = ? AND run_id = ?`,
		scope.TenantID, first[0].WorkspaceID, candidate.RunID).Error)
	third, err := captures.ClaimPromotionForRun(context.Background(), scope.TenantID, candidate.RunID, time.Minute)
	require.NoError(t, err)
	require.Len(t, third, 1, "an incomplete candidate becomes eligible after its cooldown")
}

func TestCraftT20FreshPromotionQuotaSurvivesOverdueBacklog(t *testing.T) {
	db := openCraftT20PromotionDB(t)
	scope, workspaceID, candidate := seedT20TerminalWebRun(t, db, "run-fresh-after-due", "<h1>fresh</h1>")
	captures := repository.NewCraftRunCaptureStore(db)
	for i := 0; i < craftPromotionScanLimit+8; i++ {
		runID := fmt.Sprintf("run-due-backlog-%02d", i)
		created := time.Now().Add(-2*time.Hour + time.Duration(i)*time.Second)
		require.NoError(t, db.Exec(`INSERT INTO agent_runs
			(tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, status, wait_reason, deadline, created_at, updated_at)
			VALUES (1, ?, 's1', 'u1', ?, ?, 'h', '{}', 'failed', '', ?, ?, ?)`,
			runID, "req-"+runID, "am-"+runID, created.Add(time.Hour), created, created).Error)
		require.NoError(t, db.Exec(`INSERT INTO craft_run_captures
			(tenant_id, workspace_id, run_id, owner_id, session_id, generation, predecessor_revision, predecessor_state, predecessor_run_id, predecessor_digest, state, manifest_digest, draft_revision, created_at, updated_at)
			VALUES (1, ?, ?, 'u1', 's1', ?, 0, 'empty', '', '', 'advanced', ?, 1, ?, ?)`,
			workspaceID, runID, "gen-"+runID, candidate.ManifestDigest, created, created).Error)
		claimed, err := captures.ClaimPromotionForRun(context.Background(), scope.TenantID, runID, time.Minute)
		require.NoError(t, err)
		require.Len(t, claimed, 1)
	}
	// Simulate more than one page of failed receipts whose retry cooldowns
	// have elapsed together.
	require.NoError(t, db.Exec("UPDATE craft_run_capture_promotion_attempts SET retry_after = datetime('now', '-1 minute')").Error)
	foundFresh := false
	for tick := 0; tick < 8 && !foundFresh; tick++ {
		batch, err := captures.ClaimPromotionBatch(context.Background(), craftPromotionScanLimit, 5*time.Minute)
		require.NoError(t, err)
		require.LessOrEqual(t, len(batch), craftPromotionScanLimit)
		for _, receipt := range batch {
			foundFresh = foundFresh || receipt.RunID == candidate.RunID
		}
	}
	require.True(t, foundFresh, "bounded fresh keyset pages keep advancing through an overdue backlog and eventually reach new work")
}

func TestCraftT20TargetedAndGlobalPromotionClaimsSerializeSQLite(t *testing.T) {
	db := openCraftT20PromotionDB(t)
	scope, _, candidate := seedT20TerminalWebRun(t, db, "run-claim-race", "<h1>one claim</h1>")
	globalStore := repository.NewCraftRunCaptureStore(db)
	targetStore := repository.NewCraftRunCaptureStore(db)
	start := make(chan struct{})
	type claimResult struct {
		rows []repository.CraftRunCapturePromotionCandidate
		err  error
	}
	globalResult := make(chan claimResult, 1)
	targetResult := make(chan claimResult, 1)
	go func() {
		<-start
		rows, err := globalStore.ClaimPromotionBatch(context.Background(), craftPromotionScanLimit, time.Minute)
		globalResult <- claimResult{rows: rows, err: err}
	}()
	go func() {
		<-start
		rows, err := targetStore.ClaimPromotionForRun(context.Background(), scope.TenantID, candidate.RunID, time.Minute)
		targetResult <- claimResult{rows: rows, err: err}
	}()
	close(start)
	global := <-globalResult
	target := <-targetResult
	require.NoError(t, global.err)
	require.NoError(t, target.err)
	require.Equal(t, 1, len(global.rows)+len(target.rows), "exactly one claim path reserves this receipt")
}

func TestCraftT20PerRunPromotionUsesIndependentBoundedContext(t *testing.T) {
	db := openCraftT20PromotionDB(t)
	scope, ws, candidate := seedT20TerminalWebRun(t, db, "run-targeted", "<h1>targeted</h1>")
	probe := &t20ProbeStub{reachable: craft.WebCheckPassed, loaded: craft.WebCheckPassed}
	promoter, versions := t20PromotionStack(t, db, probe)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	promoter.promoteReceiptsForRun(ctx, scope.TenantID, candidate.RunID)
	_, err := versions.Get(context.Background(), scope, craft.VersionID(ws, candidate.RunID, candidate.ManifestDigest))
	require.NoError(t, err, "per-Run promotion receives its own bounded context after drain cancellation")
	require.Equal(t, 1, probe.calls)
}

func TestCraftT20PromotionRejectsHeadAdvancedAfterValidationSQLite(t *testing.T) {
	db := openCraftT20PromotionDB(t)
	scope, workspaceID, candidate := seedT20TerminalWebRun(t, db, "run-fence-one", "<h1>revision 1</h1>")
	probe := &t20BlockingProbe{started: make(chan struct{}), release: make(chan struct{})}
	drafts := repository.NewCraftDraftHeadStore(db)
	versions := repository.NewCraftVersionStore(db)
	artifacts := service.NewCraftArtifactServiceWithCandidates(
		closedCraftArtifactSource{}, newCaptureWiringFiles(t, db), versions,
		repository.NewCraftCandidateStore(db), nil,
		service.CraftArtifactConfig{Kind: craft.KindWeb, OutputDir: craftLocalOutputDir},
	).WithWebPromotion(drafts, probe).WithWebBuildReceipt(f08CaptureReceiptFence{})
	result := make(chan error, 1)
	go func() {
		_, err := artifacts.PromoteWebVersion(context.Background(), scope, craft.WebPromotionRequest{
			WorkspaceID: workspaceID, RunID: candidate.RunID, CandidateID: candidate.ID, Revision: 1,
		})
		result <- err
	}()
	select {
	case <-probe.started: // Revision 1 has already passed service validation.
	case <-time.After(10 * time.Second):
		t.Fatal("promotion did not reach the page probe after validating revision 1")
	}

	// Advance to revision 2 while revision 1 is between its service check and
	// publish. The transaction's revision CAS must reject the older publish.
	now := time.Now()
	runID := "run-fence-two"
	require.NoError(t, db.Exec(`INSERT INTO agent_runs
		(tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, status, wait_reason, deadline, created_at, updated_at)
		VALUES (1, ?, 's1', 'u1', ?, ?, 'h', '{}', 'succeeded', '', ?, ?, ?)`,
		runID, "req-"+runID, "am-"+runID, now.Add(time.Hour), now, now).Error)
	secondContent := "<h1>revision 2</h1>"
	secondDigest := sha256.Sum256([]byte(secondContent))
	secondFiles := []craft.File{{
		Path: "index.html", Ref: "resource://t20-promotion/" + runID,
		SHA256: hex.EncodeToString(secondDigest[:]),
		MIME:   "text/html", Bytes: int64(len(secondContent)),
	}}
	_, err := drafts.Advance(context.Background(), scope, workspaceID, 1, runID, secondFiles)
	require.NoError(t, err)
	close(probe.release)
	require.ErrorIs(t, <-result, craft.ErrConflict)
	listed, err := versions.List(context.Background(), scope)
	require.NoError(t, err)
	require.Empty(t, listed, "revision 1 must not publish after revision 2 becomes current")
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
