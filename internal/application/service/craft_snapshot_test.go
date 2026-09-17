package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// -----------------------------------------------------------------------------
// Harness: the real stores over the real migrations, with the isolated-state
// source faked at exactly its seam.
// -----------------------------------------------------------------------------

// fakeSnapshotFiles is the controlled storage: content-addressed saves with
// per-object checksums, and a tamper hook for the corruption cases.
type fakeSnapshotFiles struct {
	interfaces.FileService
	mu     sync.Mutex
	blobs  map[string][]byte
	Tamper map[string][]byte // ref -> replacement bytes served instead
}

func newFakeSnapshotFiles() *fakeSnapshotFiles {
	return &fakeSnapshotFiles{blobs: map[string][]byte{}, Tamper: map[string][]byte{}}
}

func (f *fakeSnapshotFiles) SaveBytes(_ context.Context, data []byte, _ uint64, name string, _ bool) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ref := "resource://snapshot/" + name
	f.blobs[ref] = data
	return ref, nil
}

func (f *fakeSnapshotFiles) GetFile(_ context.Context, ref string) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if tampered, ok := f.Tamper[ref]; ok {
		return io.NopCloser(strings.NewReader(string(tampered))), nil
	}
	data, ok := f.blobs[ref]
	if !ok {
		return nil, errors.New("object missing")
	}
	return io.NopCloser(strings.NewReader(string(data))), nil
}

// fakeSnapshotSource is the isolated execution state of one shared serve:
// the live message chains per OpenCode session, the staged files per
// generation, and the knobs every refusal case needs.
type fakeSnapshotSource struct {
	mu sync.Mutex
	// chains is the live OC message chain per session id.
	chains map[string][]craft.SessionRecord
	// staged is the materialized file tree per generation.
	staged map[string]map[string]string
	// released records generations cleaned after a lost CAS.
	released []string
	// quiescent/quiescentReason drive the capture gate.
	quiescent       bool
	quiescentReason string
	// isolated toggles the provider capability.
	isolated bool
	// failMaterialize makes RestoreGeneration fail after partial work.
	failMaterialize error
	// stealRevision bumps the workspace revision during materialization,
	// simulating a concurrent writer winning the CAS race.
	stealRevision func()
	// restoreCount counts materializations.
	restoreCount int
	// generationSeq mints a fresh generation per restore.
	generationSeq int
}

func newFakeSnapshotSource() *fakeSnapshotSource {
	return &fakeSnapshotSource{
		chains: map[string][]craft.SessionRecord{}, staged: map[string]map[string]string{},
		quiescent: true, isolated: true,
	}
}

func (f *fakeSnapshotSource) Quiescent(_ context.Context, _ craft.Workspace) (bool, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.quiescent, f.quiescentReason, nil
}

func (f *fakeSnapshotSource) ExportSessionData(_ context.Context, ws craft.Workspace) (craft.SessionExport, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	records, ok := f.chains[ws.OpenCodeSessionID]
	if !ok {
		return craft.SessionExport{}, fmt.Errorf("%w: opencode session %s no longer exists in the serve", craft.ErrUnsupported, ws.OpenCodeSessionID)
	}
	copied := make([]craft.SessionRecord, len(records))
	copy(copied, records)
	return craft.SessionExport{
		OpenCodeSessionID: ws.OpenCodeSessionID,
		SchemaVersion:     "fake-serve/messages-v1",
		SkillDigests:      []string{},
		Records:           copied,
	}, nil
}

func (f *fakeSnapshotSource) RestoreGeneration(_ context.Context, ws craft.Workspace, files []craft.File, export craft.SessionExport) (craft.Workspace, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.isolated {
		return craft.Workspace{}, fmt.Errorf("%w: provider cannot restore isolated opencode data", craft.ErrUnsupported)
	}
	live, ok := f.chains[ws.OpenCodeSessionID]
	if !ok {
		return craft.Workspace{}, fmt.Errorf("%w: opencode session %s no longer exists; its data is not restorable through this provider", craft.ErrUnsupported, ws.OpenCodeSessionID)
	}
	if err := craft.SessionChainPrefix(export.Records, live); err != nil {
		return craft.Workspace{}, fmt.Errorf("%w: opencode session no longer carries the snapshot chain: %v", craft.ErrConflict, err)
	}
	if f.failMaterialize != nil {
		return craft.Workspace{}, f.failMaterialize
	}
	if f.stealRevision != nil {
		f.stealRevision()
	}
	f.restoreCount++
	f.generationSeq++
	generation := fmt.Sprintf("g-restore-%d", f.generationSeq)
	tree := map[string]string{}
	for _, file := range files {
		tree[file.Path] = file.SHA256
	}
	f.staged[generation] = tree
	return craft.Workspace{
		Scope: ws.Scope, ID: ws.ID, SandboxID: ws.SandboxID,
		Generation: generation, OpenCodeSessionID: ws.OpenCodeSessionID,
		RuntimeDigest: ws.RuntimeDigest,
	}, nil
}

func (f *fakeSnapshotSource) ReleaseGeneration(_ context.Context, candidate craft.Workspace) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.staged, candidate.Generation)
	f.released = append(f.released, candidate.Generation)
	return nil
}

func (f *fakeSnapshotSource) IsolatedDataRestore() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.isolated
}

// appendFakeRound continues the live chain like a real executed round would:
// one user message and one completed assistant answer.
func (f *fakeSnapshotSource) appendFakeRound(prompt, answer string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for sessionID := range f.chains {
		records := f.chains[sessionID]
		parent := ""
		if len(records) > 0 {
			parent = records[len(records)-1].ID
		}
		userID := fmt.Sprintf("msg_user_%d", len(records)+1)
		assistantID := fmt.Sprintf("msg_assistant_%d", len(records)+2)
		f.chains[sessionID] = append(records,
			craft.SessionRecord{ID: userID, ParentID: parent, Role: "user", Parts: []string{prompt}},
			craft.SessionRecord{ID: assistantID, ParentID: userID, Role: "assistant", Finish: "stop", Parts: []string{answer}},
		)
	}
}

// openSnapshotServiceDB mirrors the craft session service harness (the
// migrations now include 000046_craft_snapshots).
func openSnapshotServiceDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dbPath := filepath.Join(t.TempDir(), "craft-snapshots.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance(
		"file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()
	db, err := gorm.Open(gormsqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec(
		`INSERT INTO tenants (id, name, business) VALUES (1, 'tenant-1', 'test')`).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO users (id, username, email, password_hash, tenant_id)
		 VALUES ('u1', 'u1', 'u1@example.test', 'x', 1), ('u2', 'u2', 'u2@example.test', 'x', 1)`).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES
		 ('s1', 1, 'craft-1', 'u1', 'trpc'), ('s2', 1, 'craft-2', 'u2', 'trpc')`).Error)
	t.Cleanup(func() {
		conn, e := db.DB()
		if e == nil {
			_ = conn.Close()
		}
	})
	return db
}

// snapshotEnv is one assembled snapshot service over the real stores.
type snapshotEnv struct {
	db        *gorm.DB
	sessions  *fakeCraftSessions
	files     *fakeSnapshotFiles
	source    *fakeSnapshotSource
	store     craft.Store
	versions  craft.VersionStore
	snapshots craft.SnapshotStore
	svc       *CraftSnapshotService
	scope     craft.Scope
	workspace craft.Workspace
}

const snapshotRuntimeDigest = "sha256:snapshot-runtime"

func newSnapshotEnv(t *testing.T) *snapshotEnv {
	t.Helper()
	db := openSnapshotServiceDB(t)
	store := repository.NewCraftStore(db)
	env := &snapshotEnv{
		db: db, sessions: &fakeCraftSessions{db: db}, files: newFakeSnapshotFiles(),
		source: newFakeSnapshotSource(),
		store:  store, versions: repository.NewCraftVersionStore(db),
		snapshots: repository.NewCraftSnapshotStore(db),
		scope:     craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"},
	}
	svc, err := NewCraftSnapshotService(CraftSnapshotConfig{
		DB: db, Sessions: env.sessions, Store: store, Versions: env.versions,
		Snapshots: env.snapshots, Files: env.files, Source: env.source,
		ActiveRuns: CraftActiveRunsQuery(db), RuntimeDigest: snapshotRuntimeDigest,
	})
	require.NoError(t, err)
	env.svc = svc
	ws, err := store.PutWorkspace(context.Background(), craft.Workspace{
		Scope: env.scope, SandboxID: "sbx-1", Generation: "0",
		OpenCodeSessionID: "oc-s1", RuntimeDigest: snapshotRuntimeDigest,
	}, 0)
	require.NoError(t, err)
	env.workspace = ws
	env.source.chains["oc-s1"] = []craft.SessionRecord{
		{ID: "msg_user_1", Role: "user", Parts: []string{"make a site"}},
		{ID: "msg_assistant_2", ParentID: "msg_user_1", Role: "assistant", Finish: "stop", Parts: []string{"done v1"}},
	}
	return env
}

// publishSnapshotVersion publishes a version of the workspace with real
// content-addressed files stored through the fake controlled storage.
func (env *snapshotEnv) publishSnapshotVersion(t *testing.T, runID, content string) craft.Version {
	t.Helper()
	ref := "resource://version/" + runID
	env.files.mu.Lock()
	env.files.blobs[ref] = []byte(content)
	env.files.mu.Unlock()
	sum := sha256.Sum256([]byte(content))
	files := []craft.File{{
		Path: "index.html", Ref: ref, SHA256: hex.EncodeToString(sum[:]),
		MIME: "text/html", Bytes: int64(len(content)),
	}}
	digest, err := craft.ManifestDigest(files)
	require.NoError(t, err)
	published, err := env.versions.Publish(context.Background(), env.scope, craft.Version{
		ID:          craft.VersionID(env.workspace.ID, runID, digest),
		WorkspaceID: env.workspace.ID, RunID: runID, Kind: craft.KindWeb,
		Files: files, Checks: []craft.Check{{Name: craft.CheckEntry, Status: craft.CheckPassed}},
	})
	require.NoError(t, err)
	return published
}

func (env *snapshotEnv) capture(t *testing.T, versionID string) craft.Snapshot {
	t.Helper()
	snap, err := env.svc.Capture(context.Background(), env.scope, versionID)
	require.NoError(t, err)
	return snap
}

// insertPendingDelegation seeds an unfinished delegation row (no terminal
// result) for the workspace.
func (env *snapshotEnv) insertPendingDelegation(t *testing.T, id string) {
	t.Helper()
	require.NoError(t, env.db.Exec(
		`INSERT INTO craft_delegations (id, tenant_id, run_id, tool_call_id, workspace_id, request_hash, task_json, status)
		 VALUES (?, 1, 'run-pending', ?, ?, 'hash-'+?, '{}', 'prepared')`,
		id, "call-"+id, env.workspace.ID, id).Error)
}

// admitActiveRun seeds a live agent run for the session the way Submit does.
func (env *snapshotEnv) admitActiveRun(t *testing.T, runID string) {
	t.Helper()
	runs := repository.NewAgentRunStore(env.db)
	_, err := runs.Admit(context.Background(), agentruntime.Admission{
		Key:       agentruntime.RunKey{TenantID: 1, RunID: runID},
		SessionID: "s1", UserID: "u1", RequestID: "snapshot-" + runID,
		AssistantMessageID: "a-" + runID, RequestHash: "h-" + runID,
		Snapshot:         []byte(`{"version":1}`),
		UserMessage:      []byte(`{"role":"user"}`),
		AssistantMessage: []byte(`{"role":"assistant"}`),
		Deadline:         time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
}

// -----------------------------------------------------------------------------
// Capture (Step 4): quiescence gate, idempotent identity, completeness.
// -----------------------------------------------------------------------------

// TestCraftSnapshotCaptureGates pins the C05 capture gate: a non-idle
// OpenCode runtime refuses, a pending or outcome-unknown delegation refuses,
// and a quiescent workspace captures completely — both digests recorded, the
// session object uploaded, and the identical retry answering the same id.
func TestCraftSnapshotCaptureGates(t *testing.T) {
	env := newSnapshotEnv(t)
	v1 := env.publishSnapshotVersion(t, "run-1", "<h1>v1</h1>")

	env.source.mu.Lock()
	env.source.quiescent = false
	env.source.quiescentReason = "session busy on a tool round"
	env.source.mu.Unlock()
	_, err := env.svc.Capture(context.Background(), env.scope, v1.ID)
	require.ErrorIs(t, err, craft.ErrBusy)
	require.Contains(t, err.Error(), "not quiescent")

	env.source.mu.Lock()
	env.source.quiescent = true
	env.source.mu.Unlock()
	env.insertPendingDelegation(t, "dlg-pending")
	_, err = env.svc.Capture(context.Background(), env.scope, v1.ID)
	require.ErrorIs(t, err, craft.ErrBusy)
	require.Contains(t, err.Error(), "pending or outcome-unknown")
	require.NoError(t, env.db.Exec("DELETE FROM craft_delegations WHERE id = 'dlg-pending'").Error)

	snap := env.capture(t, v1.ID)
	require.True(t, snap.Quiescent)
	require.Equal(t, env.workspace.ID, snap.WorkspaceID)
	require.Equal(t, v1.ID, snap.VersionID)
	require.NotEmpty(t, snap.FilesDigest)
	require.NotEmpty(t, snap.SessionDigest)
	require.Equal(t, snapshotRuntimeDigest, snap.RuntimeDigest)

	// The identical retry adopts the stored row: one snapshot, same id.
	again := env.capture(t, v1.ID)
	require.Equal(t, craft.SnapshotID(snap), craft.SnapshotID(again))
	stored, err := env.snapshots.Get(context.Background(), env.scope, craft.SnapshotID(snap))
	require.NoError(t, err)
	require.Len(t, stored.Objects, 2, "one file object + one session object")
	require.Equal(t, craft.SnapshotObjectSession, stored.Objects[1].Kind)
	require.Equal(t, snapshotRuntimeDigest, stored.Manifest.RuntimeDigest)
	require.Equal(t, "fake-serve/messages-v1", stored.Manifest.SchemaVersion)
	require.NotZero(t, stored.Manifest.Records)
}

// -----------------------------------------------------------------------------
// Restore (Steps 5–6) — the Step 7 integration cases.
// -----------------------------------------------------------------------------

// TestCraftSnapshotRestoreV1EditV3KeepsV2 is the acceptance walk: v1 captured,
// a second round publishes v2, the user restores v1 (files + session history
// verified in a new generation), a third round publishes v3, and v2 keeps
// downloading unchanged — artifacts are immutable, restore never rewrites
// history.
func TestCraftSnapshotRestoreV1EditV3KeepsV2(t *testing.T) {
	env := newSnapshotEnv(t)
	v1 := env.publishSnapshotVersion(t, "run-1", "<h1>v1</h1>")
	snapV1 := env.capture(t, v1.ID)

	// Round two runs on the same session and publishes v2.
	env.source.appendFakeRound("make it v2", "done v2")
	v2 := env.publishSnapshotVersion(t, "run-2", "<h1>v2</h1>")
	require.NotEqual(t, v1.ID, v2.ID)

	// Restore v1 at the revision the client last saw.
	current, err := env.store.GetWorkspace(context.Background(), env.scope)
	require.NoError(t, err)
	restored, err := env.svc.Restore(craftCtx(1, "u1", "s1"), env.scope, craft.SnapshotID(snapV1), current.Revision)
	require.NoError(t, err)
	require.NotEqual(t, current.Generation, restored.Generation, "restore binds a new generation")
	require.Equal(t, current.OpenCodeSessionID, restored.OpenCodeSessionID, "the session history continues atomically")
	require.Greater(t, restored.Revision, current.Revision)

	// The restored generation carries exactly v1's files.
	env.source.mu.Lock()
	tree, ok := env.source.staged[restored.Generation]
	env.source.mu.Unlock()
	require.True(t, ok)
	require.Equal(t, v1.Files[0].SHA256, tree["index.html"])

	// Round three publishes v3 as a brand new version.
	env.source.appendFakeRound("back to v1 then evolve to v3", "done v3")
	v3 := env.publishSnapshotVersion(t, "run-3", "<h1>v3</h1>")
	require.NotEqual(t, v1.ID, v3.ID)
	require.NotEqual(t, v2.ID, v3.ID)

	// v2 (and v1) still download their pinned bytes — nothing was rewritten.
	kept, err := env.versions.Get(context.Background(), env.scope, v2.ID)
	require.NoError(t, err)
	require.Equal(t, v2.Files, kept.Files)
	reader, err := env.files.GetFile(context.Background(), kept.Files[0].Ref)
	require.NoError(t, err)
	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	_ = reader.Close()
	require.Equal(t, "<h1>v2</h1>", string(content))

	list, err := env.versions.List(context.Background(), env.scope)
	require.NoError(t, err)
	require.Len(t, list, 3)
}

// TestCraftSnapshotRestoreRefusals pins every structural refusal: an active
// run, a revision race, a corrupt session object (files intact), a session
// chain that diverged, a provider without isolated data restore, a snapshot
// from a replaced runtime, and a non-owner scope.
func TestCraftSnapshotRestoreRefusals(t *testing.T) {
	env := newSnapshotEnv(t)
	v1 := env.publishSnapshotVersion(t, "run-1", "<h1>v1</h1>")
	snap := env.capture(t, v1.ID)
	snapID := craft.SnapshotID(snap)

	// Active run refuses: queued/running/recovering/waiting_user all hold the
	// session's run slot.
	env.admitActiveRun(t, "run-live")
	ws, err := env.store.GetWorkspace(context.Background(), env.scope)
	require.NoError(t, err)
	_, err = env.svc.Restore(craftCtx(1, "u1", "s1"), env.scope, snapID, ws.Revision)
	require.ErrorIs(t, err, craft.ErrBusy)
	require.Contains(t, err.Error(), "active run")

	// Revision race refuses with a reload hint.
	_, err = env.svc.Restore(craftCtx(1, "u1", "s1"), env.scope, snapID, ws.Revision+5)
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Contains(t, err.Error(), "revision moved")

	// A waiting_user run also refuses (the run still owns the slot).
	require.NoError(t, env.db.Exec("UPDATE agent_runs SET status = 'waiting_user' WHERE run_id = 'run-live'").Error)
	_, err = env.svc.Restore(craftCtx(1, "u1", "s1"), env.scope, snapID, ws.Revision)
	require.ErrorIs(t, err, craft.ErrBusy)
	require.NoError(t, env.db.Exec("UPDATE agent_runs SET status = 'succeeded' WHERE run_id = 'run-live'").Error)

	// Corrupt session object with intact files refuses on the checksum.
	stored, err := env.snapshots.Get(context.Background(), env.scope, snapID)
	require.NoError(t, err)
	var sessionRef string
	for _, o := range stored.Objects {
		if o.Kind == craft.SnapshotObjectSession {
			sessionRef = o.Ref
		}
	}
	require.NotEmpty(t, sessionRef)
	// Same byte length as the pinned object, different content: the size
	// check passes and the per-object checksum is what refuses.
	env.files.mu.Lock()
	tampered := []byte(" {\"records\":[],\"tampered\":true}")
	for len(tampered) < len(env.files.blobs[sessionRef]) {
		tampered = append(tampered, ' ')
	}
	env.files.mu.Unlock()
	env.files.Tamper[sessionRef] = tampered
	_, err = env.svc.Restore(craftCtx(1, "u1", "s1"), env.scope, snapID, ws.Revision)
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Contains(t, err.Error(), "checksum")
	delete(env.files.Tamper, sessionRef)

	// A session chain that diverged past the snapshot refuses: the live
	// history no longer contains the snapshot state verbatim.
	env.source.mu.Lock()
	env.source.chains["oc-s1"] = []craft.SessionRecord{
		{ID: "msg_user_1", Role: "user", Parts: []string{"make a DIFFERENT site"}},
		{ID: "msg_assistant_2", ParentID: "msg_user_1", Role: "assistant", Finish: "stop", Parts: []string{"done v1"}},
	}
	env.source.mu.Unlock()
	_, err = env.svc.Restore(craftCtx(1, "u1", "s1"), env.scope, snapID, ws.Revision)
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Contains(t, err.Error(), "snapshot chain")
	env.source.appendFakeRound("restore the chain", "ok")

	// Provider without isolated data restore answers unsupported.
	env.source.mu.Lock()
	env.source.isolated = false
	env.source.mu.Unlock()
	_, err = env.svc.Restore(craftCtx(1, "u1", "s1"), env.scope, snapID, ws.Revision)
	require.ErrorIs(t, err, craft.ErrUnsupported)
	env.source.mu.Lock()
	env.source.isolated = true
	env.source.mu.Unlock()

	// A snapshot from a replaced runtime answers unsupported with the two
	// digests named.
	other, err := NewCraftSnapshotService(CraftSnapshotConfig{
		DB: env.db, Sessions: env.sessions, Store: env.store, Versions: env.versions,
		Snapshots: env.snapshots, Files: env.files, Source: env.source,
		ActiveRuns: CraftActiveRunsQuery(env.db), RuntimeDigest: "sha256:new-runtime",
	})
	require.NoError(t, err)
	_, err = other.Restore(craftCtx(1, "u1", "s1"), env.scope, snapID, ws.Revision)
	require.ErrorIs(t, err, craft.ErrUnsupported)
	require.Contains(t, err.Error(), snapshotRuntimeDigest)
	require.Contains(t, err.Error(), "sha256:new-runtime")

	// The session's owner gate: another user's scope is forbidden, another
	// tenant invisible.
	_, err = env.svc.Restore(adminCraftCtx(1, "u2", "s1"), craft.Scope{TenantID: 1, UserID: "u2", SessionID: "s1"}, snapID, ws.Revision)
	require.ErrorIs(t, err, craft.ErrForbidden)
	_, err = env.svc.Restore(craftCtx(2, "u1", "s1"), craft.Scope{TenantID: 2, UserID: "u1", SessionID: "s1"}, snapID, ws.Revision)
	require.ErrorIs(t, err, craft.ErrNotFound)

	// A snapshot of another workspace is never restorable here.
	require.NoError(t, env.db.Exec(
		`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s-other', 1, 'x', 'u1', 'trpc')`).Error)
	otherScope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s-other"}
	otherWS, err := env.store.PutWorkspace(context.Background(), craft.Workspace{
		Scope: otherScope, SandboxID: "sbx-2", Generation: "0",
		OpenCodeSessionID: "oc-s2", RuntimeDigest: snapshotRuntimeDigest,
	}, 0)
	require.NoError(t, err)
	_, err = env.svc.Restore(craftCtx(1, "u1", "s-other"), otherScope, snapID, otherWS.Revision)
	require.ErrorIs(t, err, craft.ErrNotFound, "another session's snapshot is invisible, never restorable")

	// After all refusals the binding never moved.
	after, err := env.store.GetWorkspace(context.Background(), env.scope)
	require.NoError(t, err)
	require.Equal(t, ws.Revision, after.Revision)
	require.Equal(t, ws.Generation, after.Generation)
}

// TestCraftSnapshotRestoreKeepsOldBindingWhenInterrupted pins the no-half-
// restore contract: a materialization that dies mid-way, and a concurrent
// writer that wins the binding CAS, both leave the previous binding live and
// clean the candidate generation.
func TestCraftSnapshotRestoreKeepsOldBindingWhenInterrupted(t *testing.T) {
	env := newSnapshotEnv(t)
	v1 := env.publishSnapshotVersion(t, "run-1", "<h1>v1</h1>")
	snap := env.capture(t, v1.ID)
	before, err := env.store.GetWorkspace(context.Background(), env.scope)
	require.NoError(t, err)

	// The source dies mid-materialization (a process death between staging
	// and the CAS): the service returns the error, the binding is untouched.
	env.source.mu.Lock()
	env.source.failMaterialize = errors.New("process killed while staging the generation")
	env.source.mu.Unlock()
	_, err = env.svc.Restore(craftCtx(1, "u1", "s1"), env.scope, craft.SnapshotID(snap), before.Revision)
	require.Error(t, err)
	after, err := env.store.GetWorkspace(context.Background(), env.scope)
	require.NoError(t, err)
	require.Equal(t, before.Revision, after.Revision)
	require.Equal(t, before.Generation, after.Generation)

	// A concurrent writer bumps the revision during materialization: the
	// service's CAS loses, the candidate is released, the old binding stays.
	env.source.mu.Lock()
	env.source.failMaterialize = nil
	env.source.stealRevision = func() {
		_, _ = env.store.PutWorkspace(context.Background(), craft.Workspace{
			Scope: env.scope, ID: before.ID, SandboxID: "sbx-1", Generation: "g-concurrent",
			OpenCodeSessionID: before.OpenCodeSessionID, RuntimeDigest: snapshotRuntimeDigest,
			Revision: before.Revision,
		}, before.Revision)
	}
	env.source.mu.Unlock()
	_, err = env.svc.Restore(craftCtx(1, "u1", "s1"), env.scope, craft.SnapshotID(snap), before.Revision)
	require.ErrorIs(t, err, craft.ErrConflict)
	concurrent, err := env.store.GetWorkspace(context.Background(), env.scope)
	require.NoError(t, err)
	require.Equal(t, "g-concurrent", concurrent.Generation)
	require.Equal(t, before.Revision+1, concurrent.Revision)

	// The candidate generation the loser materialized was released.
	env.source.mu.Lock()
	released := len(env.source.released)
	stagedCount := len(env.source.staged)
	env.source.mu.Unlock()
	require.Equal(t, 1, released, "the losing candidate was cleaned")
	require.Equal(t, 0, stagedCount, "no half-restored generation stays behind")

	// The same snapshot still restores cleanly at the new revision.
	restored, err := env.svc.Restore(craftCtx(1, "u1", "s1"), env.scope, craft.SnapshotID(snap), concurrent.Revision)
	require.NoError(t, err)
	require.NotEqual(t, concurrent.Generation, restored.Generation)
}

// TestCraftRestoreIdempotentKey pins the /restore idempotency contract: a
// retried request replays without re-executing, the same key with different
// parameters conflicts, and a failed restore frees the key for a real retry.
func TestCraftRestoreIdempotentKey(t *testing.T) {
	env := newSnapshotEnv(t)
	v1 := env.publishSnapshotVersion(t, "run-1", "<h1>v1</h1>")
	snap := env.capture(t, v1.ID)
	ws, err := env.store.GetWorkspace(context.Background(), env.scope)
	require.NoError(t, err)

	first, err := env.svc.RestoreIdempotent(craftCtx(1, "u1", "s1"), env.scope, CraftRestoreRequest{
		RequestID: "restore-1", SnapshotID: craft.SnapshotID(snap), Revision: ws.Revision,
	})
	require.NoError(t, err)
	require.False(t, first.Replayed)
	env.source.mu.Lock()
	executed := env.source.restoreCount
	env.source.mu.Unlock()
	require.Equal(t, 1, executed)

	// The identical retry replays without another materialization.
	retry, err := env.svc.RestoreIdempotent(craftCtx(1, "u1", "s1"), env.scope, CraftRestoreRequest{
		RequestID: "restore-1", SnapshotID: craft.SnapshotID(snap), Revision: ws.Revision,
	})
	require.NoError(t, err)
	require.True(t, retry.Replayed)
	require.Equal(t, first.Workspace.Generation, retry.Workspace.Generation)
	env.source.mu.Lock()
	executed = env.source.restoreCount
	env.source.mu.Unlock()
	require.Equal(t, 1, executed, "replay never re-executes")

	// The same key with different parameters is a conflict.
	_, err = env.svc.RestoreIdempotent(craftCtx(1, "u1", "s1"), env.scope, CraftRestoreRequest{
		RequestID: "restore-1", SnapshotID: craft.SnapshotID(snap), Revision: ws.Revision + 99,
	})
	require.ErrorIs(t, err, craft.ErrConflict)

	// A failed restore frees its key: after the blocker clears, the retry
	// really restores.
	env.admitActiveRun(t, "run-live")
	_, err = env.svc.RestoreIdempotent(craftCtx(1, "u1", "s1"), env.scope, CraftRestoreRequest{
		RequestID: "restore-2", SnapshotID: craft.SnapshotID(snap), Revision: retry.Workspace.Revision,
	})
	require.ErrorIs(t, err, craft.ErrBusy)
	require.NoError(t, env.db.Exec("UPDATE agent_runs SET status = 'failed' WHERE run_id = 'run-live'").Error)
	second, err := env.svc.RestoreIdempotent(craftCtx(1, "u1", "s1"), env.scope, CraftRestoreRequest{
		RequestID: "restore-2", SnapshotID: craft.SnapshotID(snap), Revision: retry.Workspace.Revision,
	})
	require.NoError(t, err)
	require.False(t, second.Replayed)
	env.source.mu.Lock()
	executed = env.source.restoreCount
	env.source.mu.Unlock()
	require.Equal(t, 2, executed)
}

// TestCraftSnapshotSessionGoneUnsupported pins the deleted-serve-data case:
// files and the stored snapshot stay complete, but once the OpenCode session
// no longer exists the provider honestly answers unsupported instead of a
// files-only restore — and the binding is kept.
func TestCraftSnapshotSessionGoneUnsupported(t *testing.T) {
	env := newSnapshotEnv(t)
	v1 := env.publishSnapshotVersion(t, "run-1", "<h1>v1</h1>")
	snap := env.capture(t, v1.ID)

	// The sandbox state (files) is gone; the snapshot storage is complete.
	env.source.mu.Lock()
	delete(env.source.chains, "oc-s1")
	env.source.mu.Unlock()
	ws, err := env.store.GetWorkspace(context.Background(), env.scope)
	require.NoError(t, err)
	_, err = env.svc.Restore(craftCtx(1, "u1", "s1"), env.scope, craft.SnapshotID(snap), ws.Revision)
	require.ErrorIs(t, err, craft.ErrUnsupported)
	require.Contains(t, err.Error(), "not restorable")
	after, err := env.store.GetWorkspace(context.Background(), env.scope)
	require.NoError(t, err)
	require.Equal(t, ws.Revision, after.Revision)

	// Restoring the serve data (the session exists again with the identical
	// chain — the prefix check passes) makes the same snapshot restorable.
	env.source.mu.Lock()
	env.source.chains["oc-s1"] = []craft.SessionRecord{
		{ID: "msg_user_1", Role: "user", Parts: []string{"make a site"}},
		{ID: "msg_assistant_2", ParentID: "msg_user_1", Role: "assistant", Finish: "stop", Parts: []string{"done v1"}},
	}
	env.source.mu.Unlock()
	restored, err := env.svc.Restore(craftCtx(1, "u1", "s1"), env.scope, craft.SnapshotID(snap), ws.Revision)
	require.NoError(t, err)
	require.NotEqual(t, ws.Generation, restored.Generation)
}

// TestCraftSnapshotListForWorkbench pins the listing the workbench restores
// its "continue from this version" affordance from: newest first, one row
// per captured version.
func TestCraftSnapshotListForWorkbench(t *testing.T) {
	env := newSnapshotEnv(t)
	v1 := env.publishSnapshotVersion(t, "run-1", "<h1>v1</h1>")
	v2 := env.publishSnapshotVersion(t, "run-2", "<h1>v2</h1>")
	env.capture(t, v1.ID)
	env.source.appendFakeRound("v2", "done")
	time.Sleep(2 * time.Millisecond) // created_at has sub-second precision; keep ordering deterministic
	env.capture(t, v2.ID)

	list, err := env.svc.ListSnapshots(craftCtx(1, "u1", "s1"), env.scope)
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, v2.ID, list[0].VersionID)
	require.Equal(t, v1.ID, list[1].VersionID)

	// Another session's scope sees nothing (its binding does not exist in
	// this scope).
	_, err = env.svc.ListSnapshots(craftCtx(1, "u2", "s2"), craft.Scope{TenantID: 1, UserID: "u2", SessionID: "s2"})
	require.ErrorIs(t, err, craft.ErrNotFound)
}
