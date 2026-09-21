package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/modules/execution/sandbox"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// -----------------------------------------------------------------------------
// Harness: the real lifecycle service over the real migrations and the real
// memory binding store (real lifecycle-lock semantics); only the provider
// halves (sandbox delete, run cancel, objects, quota, TTL) are faked.
// -----------------------------------------------------------------------------

// lifecycleSandboxDeleter fakes the provider delete, with a gate that can
// hold one delete mid-flight for the worker-competition tests.
type lifecycleSandboxDeleter struct {
	mu       sync.Mutex
	calls    []string
	failWith error
	// entered signals a delete started; gate, when non-nil, holds the
	// in-flight delete until the test closes it.
	entered chan struct{}
	gate    chan struct{}
}

func (d *lifecycleSandboxDeleter) Delete(_ context.Context, _ uint64, sandboxID string) error {
	d.mu.Lock()
	d.calls = append(d.calls, sandboxID)
	fail := d.failWith
	d.mu.Unlock()
	if d.entered != nil {
		d.entered <- struct{}{}
	}
	if d.gate != nil {
		<-d.gate
	}
	return fail
}

func (d *lifecycleSandboxDeleter) callCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.calls)
}

func (d *lifecycleSandboxDeleter) callList() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.calls...)
}

type lifecycleRunCanceler struct {
	mu       sync.Mutex
	canceled []string
	err      error
}

func (c *lifecycleRunCanceler) CancelSessionRuns(_ context.Context, _ uint64, sessionID string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.canceled = append(c.canceled, sessionID)
	return []string{"run-1"}, c.err
}

type lifecycleObjectDeleter struct {
	mu    sync.Mutex
	calls []string
}

func (d *lifecycleObjectDeleter) DeleteObject(_ context.Context, _ uint64, ref string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, ref)
	return nil
}

func (d *lifecycleObjectDeleter) callCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.calls)
}

func (d *lifecycleObjectDeleter) callList() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.calls...)
}

type lifecycleQuota struct {
	sandboxOver, storageOver bool
}

func (q lifecycleQuota) SandboxOverLimit(context.Context, uint64) (bool, error) {
	return q.sandboxOver, nil
}

func (q lifecycleQuota) StorageOverLimit(context.Context, uint64) (bool, error) {
	return q.storageOver, nil
}

type lifecycleTTL struct {
	expiresAt  time.Time
	extendable bool
	err        error
}

func (t lifecycleTTL) TTL(context.Context, uint64, string) (time.Time, bool, error) {
	return t.expiresAt, t.extendable, t.err
}

// lifecycleEnv assembles the lifecycle service and its collaborators.
type lifecycleEnv struct {
	db        *gorm.DB
	bindings  *sandbox.MemorySessionSandboxBindingStore
	store     craft.Store
	versions  craft.VersionStore
	snapshots craft.SnapshotStore
	deleter   *lifecycleSandboxDeleter
	canceler  *lifecycleRunCanceler
	objects   *lifecycleObjectDeleter
	svc       *CraftLifecycle
	scope     craft.Scope
	workspace craft.Workspace
	now       *atomic.Int64
}

func (e *lifecycleEnv) tick(d time.Duration) time.Time {
	return time.Unix(e.now.Add(int64(d/time.Second)), 0).UTC()
}

// openLifecycleDB mirrors the snapshot harness (the migrations now include
// 000048_craft_lifecycle).
func openLifecycleDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := openSnapshotServiceDB(t)
	require.NoError(t, db.Exec(
		`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES
		 ('gone', 1, 'craft-gone', 'u1', 'trpc')`).Error)
	return db
}

func newLifecycleEnv(t *testing.T) *lifecycleEnv {
	t.Helper()
	db := openLifecycleDB(t)
	bindings := sandbox.NewMemorySessionSandboxBindingStore()
	store := repository.NewCraftStore(db)
	clock := &atomic.Int64{}
	clock.Store(10_000)
	env := &lifecycleEnv{
		db: db, bindings: bindings, store: store,
		versions:  repository.NewCraftVersionStore(db),
		snapshots: repository.NewCraftSnapshotStore(db),
		deleter:   &lifecycleSandboxDeleter{},
		canceler:  &lifecycleRunCanceler{},
		objects:   &lifecycleObjectDeleter{},
		scope:     craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"},
		now:       clock,
	}
	svc, err := NewCraftLifecycle(CraftLifecycleConfig{
		DB: db, Store: store, Bindings: bindings,
		ActiveRuns:     CraftActiveRunsQuery(db),
		SessionExists:  NewCraftSessionExistence(db),
		SandboxDeleter: env.deleter, RunCanceler: env.canceler,
		ObjectDeleter: env.objects,
		Now:           func() time.Time { return time.Unix(clock.Load(), 0).UTC() },
	})
	require.NoError(t, err)
	env.svc = svc
	created, err := bindings.Create(context.Background(),
		sandbox.SessionSandboxKey{TenantID: 1, SessionID: "s1"},
		sandbox.SessionSandboxBinding{
			Version: sandbox.SessionSandboxBindingVersion, Provider: sandbox.SandboxTypeE2B,
			TenantID: 1, SessionID: "s1", SandboxID: "sbx-1", TemplateID: "tpl",
			CreatedAt: time.Unix(9_000, 0).UTC(), Generation: "g0",
		})
	require.True(t, created)
	require.NoError(t, err)
	ws, err := store.PutWorkspace(context.Background(), craft.Workspace{
		Scope: env.scope, SandboxID: "sbx-1", Generation: "g0",
		OpenCodeSessionID: "oc-s1", RuntimeDigest: "sha256:runtime",
	}, 0)
	require.NoError(t, err)
	env.workspace = ws
	return env
}

// softDeleteSession tombstones the session row the way the session service's
// delete path does.
func (e *lifecycleEnv) softDeleteSession(t *testing.T, sessionID string) {
	t.Helper()
	require.NoError(t, e.db.Exec(
		`UPDATE sessions SET deleted_at = ? WHERE id = ?`,
		time.Unix(9_500, 0).UTC(), sessionID).Error)
}

func (e *lifecycleEnv) insertActiveRun(t *testing.T, sessionID string) {
	t.Helper()
	runID := "run-" + sessionID
	require.NoError(t, e.db.Exec(`INSERT INTO agent_runs
		(tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash,
		 snapshot, status, deadline, created_at, updated_at)
		VALUES (1, ?, ?, 'u1', ?, ?, 'rh', '{}', 'running', ?, ?, ?)`,
		runID, sessionID, "req-"+runID, "am-"+runID,
		time.Unix(9_900, 0).UTC(), time.Unix(9_600, 0).UTC(), time.Unix(9_600, 0).UTC()).Error)
}

func (e *lifecycleEnv) insertUnfinishedDelegation(t *testing.T, id string) {
	t.Helper()
	encoded := `{"ID":"` + id + `","Scope":{"TenantID":1,"UserID":"u1","SessionID":"s1"},"Fence":{"TenantID":1,"RunID":"run-x","Owner":"u1","Epoch":1},"WorkspaceID":"` + e.workspace.ID + `","ToolCallID":"call-` + id + `","Prompt":"p","RequestHash":"h"}`
	require.NoError(t, e.db.Exec(`INSERT INTO craft_delegations
		(id, tenant_id, run_id, tool_call_id, workspace_id, prompt_message_id, request_hash, task_json, status, created_at, updated_at)
		VALUES (?, 1, 'run-x', ?, ?, 'pm-1', 'rh', ?, 'prepared', ?, ?)`,
		id, "call-"+id, e.workspace.ID, encoded,
		time.Unix(9_700, 0).UTC(), time.Unix(9_700, 0).UTC()).Error)
}

func (e *lifecycleEnv) insertPendingInteraction(t *testing.T, id string) {
	t.Helper()
	require.NoError(t, e.db.Exec(`INSERT INTO craft_interactions
		(id, tenant_id, session_id, owner_id, run_id, pending_id, kind, args_hash, prompt, status, revision, created_at, updated_at)
		VALUES (?, 1, 's1', 'u1', 'run-1', 'pd-1', 'question', 'ah', 'asked', 'pending', 1, ?, ?)`,
		id, time.Unix(9_800, 0).UTC(), time.Unix(9_800, 0).UTC()).Error)
}

func (e *lifecycleEnv) lifecycleRow(t *testing.T, sessionID string) *craftLifecycleStateRow {
	t.Helper()
	var row craftLifecycleStateRow
	err := e.db.Where("tenant_id = ? AND session_id = ? AND resource_kind = ?",
		uint64(1), sessionID, craft.LifecycleResourceSandbox).
		Order("updated_at DESC").First(&row).Error
	if err != nil {
		return nil
	}
	return &row
}

// publishVersion stores one real immutable version of the workspace.
func (e *lifecycleEnv) publishVersion(t *testing.T, files *fakeSnapshotFiles, runID, content string) craft.Version {
	t.Helper()
	ref := "resource://version/" + runID
	files.blobs[ref] = []byte(content)
	file := craft.File{
		Path: "index.html", Ref: ref, SHA256: sha256SumString(content),
		MIME: "text/html", Bytes: int64(len(content)),
	}
	digest, err := craft.ManifestDigest([]craft.File{file})
	require.NoError(t, err)
	published, err := e.versions.Publish(context.Background(), e.scope, craft.Version{
		ID:          craft.VersionID(e.workspace.ID, runID, digest),
		WorkspaceID: e.workspace.ID, RunID: runID, Kind: craft.KindWeb,
		Files: []craft.File{file},
	})
	require.NoError(t, err)
	return published
}

// -----------------------------------------------------------------------------
// Sweep protection rules (the product constraint)
// -----------------------------------------------------------------------------

// A live session's sandbox is referenced — the artwork stays re-openable —
// so the sweep keeps it even when a tombstone row exists.
func TestLifecycleSweepKeepsLiveSessionSandbox(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()
	_, err := env.svc.TombstoneSession(ctx, 1, "s1", "operator teardown")
	require.NoError(t, err)

	require.NoError(t, env.svc.Sweep(ctx, 10))

	require.Equal(t, 0, env.deleter.callCount(), "live session sandbox deleted")
	row := env.lifecycleRow(t, "s1")
	require.NotNil(t, row)
	require.Equal(t, craft.LifecycleStateKept, row.State)
	require.Contains(t, row.Reason, "re-openable")
	binding, err := env.bindings.Get(ctx, sandbox.SessionSandboxKey{TenantID: 1, SessionID: "s1"})
	require.NoError(t, err)
	require.NotNil(t, binding, "binding cleared on a live session")
}

// An active run, an outcome-unknown delegation or a pending decision each
// keep the sandbox of a deleted session; the sweep records WHY.
func TestLifecycleSweepKeepsProtectedDeletedSessions(t *testing.T) {
	ctx := context.Background()

	t.Run("active run", func(t *testing.T) {
		env := newLifecycleEnv(t)
		env.softDeleteSession(t, "s1")
		env.insertActiveRun(t, "s1")
		require.NoError(t, env.svc.Sweep(ctx, 10))
		require.Equal(t, 0, env.deleter.callCount())
		require.Equal(t, craft.LifecycleStateKept, env.lifecycleRow(t, "s1").State)
	})

	t.Run("unknown delegation", func(t *testing.T) {
		env := newLifecycleEnv(t)
		env.softDeleteSession(t, "s1")
		env.insertUnfinishedDelegation(t, "dlg-unknown")
		require.NoError(t, env.svc.Sweep(ctx, 10))
		require.Equal(t, 0, env.deleter.callCount(), "unknown resource reclaimed")
		row := env.lifecycleRow(t, "s1")
		require.Equal(t, craft.LifecycleStateRisk, row.State)
		require.Contains(t, row.Reason, "dlg-unknown", "risk record must name the unknown task")
	})

	t.Run("pending decision", func(t *testing.T) {
		env := newLifecycleEnv(t)
		env.softDeleteSession(t, "s1")
		env.insertPendingInteraction(t, "itr-1")
		require.NoError(t, env.svc.Sweep(ctx, 10))
		require.Equal(t, 0, env.deleter.callCount())
		require.Equal(t, craft.LifecycleStateKept, env.lifecycleRow(t, "s1").State)
	})
}

// The tombstone precedes everything: it is durable before run cancellation
// is attempted, it blocks dispatch and restore while teardown runs, and an
// unknown remote task keeps a durable risk record with its sandbox retained
// (the sweep, not the tombstone writer, owns the risk transition).
func TestLifecycleTombstoneOrderAndUnknownRiskRecord(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()

	// The canceler refuses: the tombstone must already be durable when the
	// cancellation is attempted, and the failure must be reported, not lost.
	env.canceler.mu.Lock()
	env.canceler.err = errors.New("run service unavailable")
	env.canceler.mu.Unlock()

	env.insertUnfinishedDelegation(t, "dlg-open")

	result, err := env.svc.TombstoneSession(ctx, 1, "s1", "session deleted")
	require.NoError(t, err)
	require.True(t, result.Tombstoned)
	require.Equal(t, "sbx-1", result.SandboxID)
	require.Equal(t, "g0", result.Generation)
	require.Equal(t, []string{"run-1"}, result.CanceledRuns)
	require.Contains(t, result.Note, "run cancellation incomplete")

	// The tombstone is durable and BLOCKING: guards refuse immediately, even
	// though the unknown task will later keep the sandbox.
	require.ErrorIs(t, env.svc.GuardDispatch(ctx, 1, "s1"), craft.ErrBusy)
	require.ErrorIs(t, env.svc.GuardRestore(ctx, 1, "s1"), craft.ErrBusy)

	// The unknown remote task is recorded on the tombstone row.
	require.Equal(t, []string{"dlg-open"}, result.UnknownTasks)
	row := env.lifecycleRow(t, "s1")
	require.Equal(t, craft.LifecycleStateDeleting, row.State, "tombstone left the blocking state")
	require.Contains(t, row.Reason, "dlg-open")

	// Idempotent: a second tombstone stays benign and keeps the guard.
	_, err = env.svc.TombstoneSession(ctx, 1, "s1", "session deleted")
	require.NoError(t, err)
	require.ErrorIs(t, env.svc.GuardDispatch(ctx, 1, "s1"), craft.ErrBusy)

	// The sweep re-derives the protection: unknown ⇒ risk record, sandbox kept.
	require.NoError(t, env.svc.Sweep(ctx, 10))
	require.Equal(t, 0, env.deleter.callCount(), "unknown remote task reclaimed")
	require.Equal(t, craft.LifecycleStateRisk, env.lifecycleRow(t, "s1").State)
}

// A cleanly deleted session (no active run, no unknown delegation, no
// pending decision) has its sandbox reclaimed: provider delete, workspace
// clear, binding clear, terminal record, and a sandbox_stop usage event.
func TestLifecycleSweepReclaimsDeletedSessionSandbox(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()
	env.softDeleteSession(t, "s1")

	require.NoError(t, env.svc.Sweep(ctx, 0)) // limit 0 → default batch (100)

	require.Equal(t, []string{"sbx-1"}, env.deleter.callList())
	row := env.lifecycleRow(t, "s1")
	require.NotNil(t, row)
	require.Equal(t, craft.LifecycleStateDeleted, row.State)

	binding, err := env.bindings.Get(ctx, sandbox.SessionSandboxKey{TenantID: 1, SessionID: "s1"})
	require.NoError(t, err)
	require.Nil(t, binding, "binding not cleared after cleanup")

	workspace, err := env.store.GetWorkspace(ctx, env.scope)
	require.NoError(t, err)
	require.Empty(t, workspace.SandboxID)
	require.Empty(t, workspace.OpenCodeSessionID)
	require.Empty(t, workspace.Generation)

	usage, err := env.svc.LifecycleUsage(ctx, 1, time.Unix(0, 0))
	require.NoError(t, err)
	require.Equal(t, 1, usage.Stops, "sandbox_stop event not recorded")
}

// Discovery: a session soft-deleted without the tombstone entry still gets
// its tombstone row written by the sweep itself before any destructive work.
func TestLifecycleSweepDiscoversDeletedSessions(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()
	env.softDeleteSession(t, "s1")

	// No TombstoneSession call: the sweep must discover the deleted session.
	require.NoError(t, env.svc.Sweep(ctx, 10))
	require.Equal(t, []string{"sbx-1"}, env.deleter.callList())
	require.Equal(t, craft.LifecycleStateDeleted, env.lifecycleRow(t, "s1").State)
}

// A failed provider delete keeps its retry record (attempts, last error,
// backoff eligibility) and the next sweep retries and completes.
func TestLifecycleSweepRetriesFailedProviderDelete(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()
	env.softDeleteSession(t, "s1")

	env.deleter.mu.Lock()
	env.deleter.failWith = errors.New("provider 503")
	env.deleter.mu.Unlock()
	require.NoError(t, env.svc.Sweep(ctx, 10))

	row := env.lifecycleRow(t, "s1")
	require.Equal(t, craft.LifecycleStateFailed, row.State)
	require.Equal(t, int64(1), row.Attempts)
	require.NotNil(t, row.LastError)
	require.Contains(t, *row.LastError, "provider 503")
	require.True(t, row.EligibleAt.After(time.Unix(10_000, 0)), "retry not backed off")
	// The binding survives the failed attempt.
	binding, err := env.bindings.Get(ctx, sandbox.SessionSandboxKey{TenantID: 1, SessionID: "s1"})
	require.NoError(t, err)
	require.NotNil(t, binding)

	// Before the backoff elapses the row is not swept again.
	env.deleter.mu.Lock()
	env.deleter.failWith = nil
	env.deleter.mu.Unlock()
	require.NoError(t, env.svc.Sweep(ctx, 10))
	require.Equal(t, 1, env.deleter.callCount())

	// After the backoff the retry completes.
	env.now.Store(10_000 + int64(craft.DefaultSweepRetryBackoff/time.Second) + 1)
	require.NoError(t, env.svc.Sweep(ctx, 10))
	require.Equal(t, 2, env.deleter.callCount())
	require.Equal(t, craft.LifecycleStateDeleted, env.lifecycleRow(t, "s1").State)
}

// Generation protection: a workspace rebound to a NEW sandbox generation
// keeps that binding — the sweep of the old generation deletes only the old
// provider sandbox and never clears the new instance's workspace row.
func TestLifecycleSweepGenerationProtectsNewInstance(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()
	env.softDeleteSession(t, "s1")

	// Tombstone pins generation g0; before the sweep runs, a restore/rebuild
	// rebinds the workspace to generation g1 (a new sandbox instance).
	_, err := env.svc.TombstoneSession(ctx, 1, "s1", "session deleted")
	require.NoError(t, err)
	dropped, err := env.bindings.DeleteIfMatch(ctx,
		sandbox.SessionSandboxKey{TenantID: 1, SessionID: "s1"}, sandbox.SandboxTypeE2B, "sbx-1")
	require.NoError(t, err)
	require.True(t, dropped)
	created, err := env.bindings.Create(ctx, sandbox.SessionSandboxKey{TenantID: 1, SessionID: "s1"},
		sandbox.SessionSandboxBinding{
			Version: sandbox.SessionSandboxBindingVersion, Provider: sandbox.SandboxTypeE2B,
			TenantID: 1, SessionID: "s1", SandboxID: "sbx-2", TemplateID: "tpl",
			CreatedAt: time.Unix(9_900, 0).UTC(), Generation: "g1",
		})
	require.NoError(t, err)
	require.True(t, created)
	rebound, err := env.store.PutWorkspace(ctx, craft.Workspace{
		ID: env.workspace.ID, Scope: env.scope, SandboxID: "sbx-2", Generation: "g1",
		OpenCodeSessionID: "oc-s2", RuntimeDigest: "sha256:runtime",
	}, env.workspace.Revision)
	require.NoError(t, err)

	require.NoError(t, env.svc.Sweep(ctx, 10))

	// The old provider sandbox is gone; the new instance is untouched.
	require.Equal(t, []string{"sbx-1"}, env.deleter.callList())
	binding, err := env.bindings.Get(ctx, sandbox.SessionSandboxKey{TenantID: 1, SessionID: "s1"})
	require.NoError(t, err)
	require.NotNil(t, binding, "generation-mismatched clear removed the NEW binding")
	require.Equal(t, "sbx-2", binding.SandboxID)
	workspace, err := env.store.GetWorkspace(ctx, env.scope)
	require.NoError(t, err)
	require.Equal(t, "sbx-2", workspace.SandboxID)
	require.Equal(t, "g1", workspace.Generation)
	require.Equal(t, rebound.Revision, workspace.Revision, "workspace row rewritten across generations")
}

// -----------------------------------------------------------------------------
// Two-worker competition (Step 6): snapshot save and restore against reclaim
// -----------------------------------------------------------------------------

// gatingLockStore wraps the memory binding store so the FIRST lock holder
// pauses inside its critical section until the test releases it. Both the
// snapshot service and the sweep share the same underlying per-key
// semaphore, so this is the real capture-vs-reclaim race with a
// deterministic schedule.
type gatingLockStore struct {
	sandbox.SessionSandboxBindingStore
	entered chan struct{}
	release chan struct{}
}

func (g *gatingLockStore) WithLifecycleLock(
	ctx context.Context, key sandbox.SessionSandboxKey, fn func(context.Context) error,
) error {
	hold := func(holdCtx context.Context) error {
		select {
		case g.entered <- struct{}{}:
		default:
		}
		select {
		case <-g.release:
		case <-holdCtx.Done():
			return holdCtx.Err()
		}
		return fn(holdCtx)
	}
	return g.SessionSandboxBindingStore.WithLifecycleLock(ctx, key, hold)
}

// Worker A saves a snapshot (holding the shared lifecycle lock) while worker
// B sweeps the same session: B waits for the lock, then re-derives the state
// and keeps the live session's sandbox — the capture is never raced by a
// delete, and its snapshot is intact and restorable.
func TestLifecycleSnapshotSaveRacesReclaim(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()

	// Real snapshot service sharing the binding store's lifecycle lock.
	gated := &gatingLockStore{
		SessionSandboxBindingStore: env.bindings,
		entered:                    make(chan struct{}, 1),
		release:                    make(chan struct{}),
	}
	files := newFakeSnapshotFiles()
	source := newFakeSnapshotSource()
	source.chains["oc-s1"] = []craft.SessionRecord{
		{ID: "msg_user_1", Role: "user", Parts: []string{"make a site"}},
		{ID: "msg_assistant_2", ParentID: "msg_user_1", Role: "assistant", Finish: "stop", Parts: []string{"v1"}},
	}
	snapSvc, err := NewCraftSnapshotService(CraftSnapshotConfig{
		DB: env.db, Sessions: &fakeCraftSessions{db: env.db}, Store: env.store,
		Versions: env.versions, Snapshots: env.snapshots, Files: files, Source: source,
		ActiveRuns: CraftActiveRunsQuery(env.db), RuntimeDigest: "sha256:runtime",
		Lock: CraftWorkspaceLockFromBindingStore(gated),
	})
	require.NoError(t, err)
	version := env.publishVersion(t, files, "run-cap", "<html>v1</html>")

	// An operator teardown was requested (tombstone), so the sweep WOULD
	// reclaim — but the session is live and the capture is about to hold the
	// shared lock.
	_, err = env.svc.TombstoneSession(ctx, 1, "s1", "operator teardown")
	require.NoError(t, err)

	// Worker A: the capture holds the shared lifecycle lock mid-capture.
	captured := make(chan error, 1)
	go func() { _, err := snapSvc.Capture(ctx, env.scope, version.ID); captured <- err }()

	// Wait until the capture actually holds the lock.
	select {
	case <-gated.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("capture never entered the lifecycle lock")
	}

	// Worker B: sweep while the capture is mid-flight — it blocks on the lock.
	swept := make(chan error, 1)
	go func() { swept <- env.svc.Sweep(ctx, 10) }()
	require.Equal(t, 0, env.deleter.callCount(), "delete raced the lock")

	// Let the capture finish; only then can the sweep take the lock.
	close(gated.release)
	require.NoError(t, <-captured)
	require.NoError(t, <-swept)

	// The sweep re-derived the state under the lock and KEPT the live
	// session's sandbox; the capture's snapshot is intact and restorable.
	require.Equal(t, 0, env.deleter.callCount(), "sandbox deleted under a live capture")
	list, err := env.snapshots.ListByWorkspace(ctx, env.scope)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.True(t, craft.CanRestore(list[0].Snapshot, false, "sha256:runtime"))
	require.Equal(t, craft.LifecycleStateKept, env.lifecycleRow(t, "s1").State)
}

// Worker B holds the delete mid-flight (provider gate) while worker A tries
// to restore: the durable deleting mark refuses the restore, and after the
// delete completes the revision CAS keeps any late restore from writing over
// the cleared workspace.
func TestLifecycleRestoreRacesMidDeleteSweep(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()
	env.softDeleteSession(t, "s1")
	_, err := env.svc.TombstoneSession(ctx, 1, "s1", "session deleted")
	require.NoError(t, err)

	// Gate the provider delete so the sweep sits mid-delete.
	env.deleter.entered = make(chan struct{}, 1)
	env.deleter.gate = make(chan struct{})
	swept := make(chan error, 1)
	go func() { swept <- env.svc.Sweep(ctx, 10) }()

	select {
	case <-env.deleter.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("sweep never reached the provider delete")
	}

	// Mid-delete: guards refuse new dispatch and restore on the durable mark.
	require.ErrorIs(t, env.svc.GuardDispatch(ctx, 1, "s1"), craft.ErrBusy)
	require.ErrorIs(t, env.svc.GuardRestore(ctx, 1, "s1"), craft.ErrBusy)

	// A restore prepared before the teardown must fail the workspace revision
	// CAS once the sweep's clear has landed.
	workspace, err := env.store.GetWorkspace(ctx, env.scope)
	require.NoError(t, err)

	close(env.deleter.gate)
	require.NoError(t, <-swept)
	require.Equal(t, craft.LifecycleStateDeleted, env.lifecycleRow(t, "s1").State)

	// Worker A (restore) arrives late with the pre-teardown revision: the CAS
	// refuses — the cleared binding is never silently overwritten.
	_, err = env.store.PutWorkspace(ctx, craft.Workspace{
		ID: workspace.ID, Scope: env.scope, SandboxID: "sbx-1", Generation: "g0",
		OpenCodeSessionID: "oc-s1", RuntimeDigest: "sha256:runtime",
	}, workspace.Revision)
	require.ErrorIs(t, err, craft.ErrConflict, "stale restore revision overwrote the cleanup")
}

// -----------------------------------------------------------------------------
// Orphan objects
// -----------------------------------------------------------------------------

// An orphan upload is recorded as a candidate and is NOT deleted before its
// 24h window elapses; once mature, a referenced object stays and an
// unreferenced one is deleted after the reference re-check.
func TestLifecycleOrphanCandidateWindowAndReferences(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()

	require.NoError(t, env.svc.RecordOrphanCandidate(ctx, 1, "s1", "resource://orphan/lost", "association never completed"))
	require.NoError(t, env.svc.RecordOrphanCandidate(ctx, 1, "s1", "resource://orphan/pinned", "superseded upload"))

	// Pin the second object through a real version file.
	files := newFakeSnapshotFiles()
	files.blobs["resource://orphan/pinned"] = []byte("pinned")
	pinned := craft.File{
		Path: "index.html", Ref: "resource://orphan/pinned",
		SHA256: sha256SumString("pinned"), MIME: "text/html", Bytes: 6,
	}
	digest, err := craft.ManifestDigest([]craft.File{pinned})
	require.NoError(t, err)
	_, err = env.versions.Publish(ctx, env.scope, craft.Version{
		ID:          craft.VersionID(env.workspace.ID, "run-pin", digest),
		WorkspaceID: env.workspace.ID, RunID: "run-pin", Kind: craft.KindWeb,
		Files: []craft.File{pinned},
	})
	require.NoError(t, err)

	// Before the window: nothing is deleted.
	require.NoError(t, env.svc.Sweep(ctx, 10))
	require.Equal(t, 0, env.objects.callCount())

	// After the window: the referenced object stays, the orphan goes.
	env.now.Store(10_000 + int64(craft.DefaultOrphanCandidateWindow/time.Second) + 1)
	require.NoError(t, env.svc.Sweep(ctx, 10))
	require.Equal(t, []string{"resource://orphan/lost"}, env.objects.callList())

	var kept craftLifecycleStateRow
	require.NoError(t, env.db.Where("resource_ref = ?", "resource://orphan/pinned").First(&kept).Error)
	require.Equal(t, craft.LifecycleStateKept, kept.State)
	require.Contains(t, kept.Reason, "referenced")
	var gone craftLifecycleStateRow
	require.NoError(t, env.db.Where("resource_ref = ?", "resource://orphan/lost").First(&gone).Error)
	require.Equal(t, craft.LifecycleStateDeleted, gone.State)
}

// -----------------------------------------------------------------------------
// Usage accounting, quota, TTL risk
// -----------------------------------------------------------------------------

// Redelivered events are counted once; dwell pairs real start/stop moments
// and storage integrates into byte-days.
func TestLifecycleUsageDedupAndAccounting(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()

	start := time.Unix(20_000, 0).UTC()
	stop := start.Add(2 * time.Hour)
	added, err := env.svc.RecordSandboxEvent(ctx, 1, "s1", "sbx-1", craft.LifecycleEventSandboxStart, start)
	require.NoError(t, err)
	require.True(t, added)
	added, err = env.svc.RecordSandboxEvent(ctx, 1, "s1", "sbx-1", craft.LifecycleEventSandboxStart, start)
	require.NoError(t, err)
	require.False(t, added, "redelivery counted twice")
	_, err = env.svc.RecordSandboxEvent(ctx, 1, "s1", "sbx-1", craft.LifecycleEventSandboxStop, stop)
	require.NoError(t, err)

	at := start
	for i := 0; i < 4; i++ {
		_, err = env.svc.RecordStorageBytes(ctx, 1, "s1", "sbx-1", 1024, at)
		require.NoError(t, err)
		at = at.Add(30 * time.Minute)
	}

	// Account from the end of the observed window (now = the stop moment):
	// four 30-minute observations integrate to the full 2 hours.
	env.now.Store(int64(27_200))
	usage, err := env.svc.LifecycleUsage(ctx, 1, time.Unix(0, 0))
	require.NoError(t, err)
	require.Equal(t, 1, usage.Starts)
	require.Equal(t, 1, usage.Stops)
	require.Equal(t, 0, usage.OpenStarts)
	require.Equal(t, 2*time.Hour, usage.Dwell)
	// 1024 bytes held the whole observed window (2h): 1024 * 2/24 byte-days.
	require.InDelta(t, 1024*2.0/24.0, usage.StorageBytesDay, 1e-9)
}

// Quota over-limit gates only new sandboxes: AdmitNewSandbox refuses while
// the cleanup path stays callable (it never consults the quota).
func TestLifecycleQuotaGatesOnlyNewSandboxes(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()

	svc, err := NewCraftLifecycle(CraftLifecycleConfig{
		DB: env.db, Store: env.store, Bindings: env.bindings,
		ActiveRuns: CraftActiveRunsQuery(env.db), SessionExists: NewCraftSessionExistence(env.db),
		SandboxDeleter: env.deleter, ObjectDeleter: env.objects, Quota: lifecycleQuota{sandboxOver: true},
		Now: func() time.Time { return time.Unix(10_000, 0).UTC() },
	})
	require.NoError(t, err)

	require.ErrorIs(t, svc.AdmitNewSandbox(ctx, 1), ErrCraftQuotaExceeded)
	// Cleanup is not gated: the sweep still runs (and keeps everything, as
	// the protection rules demand on a live session).
	require.NoError(t, svc.Sweep(ctx, 10))

	healthy, err := NewCraftLifecycle(CraftLifecycleConfig{
		DB: env.db, Store: env.store, Bindings: env.bindings,
		ActiveRuns: CraftActiveRunsQuery(env.db), SessionExists: NewCraftSessionExistence(env.db),
		SandboxDeleter: env.deleter, Quota: lifecycleQuota{},
		Now: func() time.Time { return time.Unix(10_000, 0).UTC() },
	})
	require.NoError(t, err)
	require.NoError(t, healthy.AdmitNewSandbox(ctx, 1))
}

// An unextendable provider TTL that ends within the idle window surfaces the
// workspace risk early; an extendable one does not.
func TestLifecycleTTLRiskSurfacesEarly(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()

	dying, err := NewCraftLifecycle(CraftLifecycleConfig{
		DB: env.db, Store: env.store, Bindings: env.bindings,
		ActiveRuns: CraftActiveRunsQuery(env.db), SessionExists: NewCraftSessionExistence(env.db),
		SandboxDeleter: env.deleter,
		TTL:            lifecycleTTL{expiresAt: time.Unix(10_000, 0).UTC().Add(5 * time.Minute)},
		Now:            func() time.Time { return time.Unix(10_000, 0).UTC() },
	})
	require.NoError(t, err)
	risk, err := dying.WorkspaceTTLRisk(ctx, 1, "s1")
	require.NoError(t, err)
	require.True(t, risk.AtRisk)
	require.Contains(t, risk.Reason, "cannot be extended")

	extending, err := NewCraftLifecycle(CraftLifecycleConfig{
		DB: env.db, Store: env.store, Bindings: env.bindings,
		ActiveRuns: CraftActiveRunsQuery(env.db), SessionExists: NewCraftSessionExistence(env.db),
		SandboxDeleter: env.deleter,
		TTL:            lifecycleTTL{expiresAt: time.Unix(10_000, 0).UTC().Add(5 * time.Minute), extendable: true},
		Now:            func() time.Time { return time.Unix(10_000, 0).UTC() },
	})
	require.NoError(t, err)
	risk, err = extending.WorkspaceTTLRisk(ctx, 1, "s1")
	require.NoError(t, err)
	require.False(t, risk.AtRisk)
}

// -----------------------------------------------------------------------------
// Dormancy policy at the service boundary
// -----------------------------------------------------------------------------

// The dormancy decision is verified end-to-end at the assembly boundary: a
// real complete snapshot is captured first, then the pure dormancy rule is
// applied with the verified fact. The pause never touches versions or
// snapshots, and never promises process state back.
func TestLifecycleDormancyDecisionForAssembly(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()

	files := newFakeSnapshotFiles()
	source := newFakeSnapshotSource()
	source.chains["oc-s1"] = []craft.SessionRecord{
		{ID: "msg_user_1", Role: "user", Parts: []string{"make a site"}},
		{ID: "msg_assistant_2", ParentID: "msg_user_1", Role: "assistant", Finish: "stop", Parts: []string{"v1"}},
	}
	snapSvc, err := NewCraftSnapshotService(CraftSnapshotConfig{
		DB: env.db, Sessions: &fakeCraftSessions{db: env.db}, Store: env.store,
		Versions: env.versions, Snapshots: env.snapshots, Files: files, Source: source,
		ActiveRuns: CraftActiveRunsQuery(env.db), RuntimeDigest: "sha256:runtime",
	})
	require.NoError(t, err)
	version := env.publishVersion(t, files, "run-dorm", "<html>v1</html>")
	snap, err := snapSvc.Capture(ctx, env.scope, version.ID)
	require.NoError(t, err)
	complete := craft.CanRestore(snap, false, "sha256:runtime")
	require.True(t, complete, "capture is not a verified complete snapshot")

	decision := craft.SandboxDormancy(45*time.Minute, complete, craft.LifecyclePolicy{})
	require.True(t, decision.MayDormant)
	require.True(t, decision.NeedsRestart, "dormancy must not promise process state back")

	// Without the verified snapshot the same idle sandbox stays.
	stayed := craft.SandboxDormancy(45*time.Minute, false, craft.LifecyclePolicy{})
	require.False(t, stayed.MayDormant)
}

// -----------------------------------------------------------------------------
// Shared helpers
// -----------------------------------------------------------------------------

func sha256SumString(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
