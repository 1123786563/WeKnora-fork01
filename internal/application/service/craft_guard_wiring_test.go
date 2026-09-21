package service

// O03 integration wiring tests: the dispatch guard refuses a NEW delegation
// (but never the reuse of a stored result), the restore guard refuses a
// restore onto a session under teardown, and a stranded 'deleting' object row
// is re-driven by the sweep instead of living forever.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/metrics"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

type stubDispatchGuard struct {
	calls    int
	refusing error
}

func (g *stubDispatchGuard) GuardDispatch(_ context.Context, _ uint64, _ string) error {
	g.calls++
	return g.refusing
}

type recordingDelegateExecutor struct {
	craft.Executor
	executed int
}

func (e *recordingDelegateExecutor) Execute(_ context.Context, _ craft.Task) (craft.Result, error) {
	e.executed++
	return craft.Result{TaskID: "recorded", Status: "succeeded", Summary: "ran"}, nil
}

func guardWiringTask(scope craft.Scope, workspaceID string) craft.Task {
	return craft.Task{
		Scope: scope, ID: "dlg-guard", ToolCallID: "call-guard", WorkspaceID: workspaceID,
		Prompt: "p", RequestHash: "rh",
		Fence: agentruntime.Fence{
			RunKey: agentruntime.RunKey{TenantID: scope.TenantID, RunID: "run-guard"},
			Owner:  scope.UserID, Epoch: 1,
		},
	}
}

// The guard refuses a FRESH dispatch before anything is prepared, and the
// refusal is the guard's own error (craft.ErrBusy from the lifecycle mark).
func TestDelegateDispatchGuardRefusesFreshDispatch(t *testing.T) {
	store := newFakeDelegationStore()
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}
	executor := &recordingDelegateExecutor{}
	delegate := NewCraftDelegateService(store, executor)
	task := guardWiringTask(scope, "ws-1")

	guard := &stubDispatchGuard{refusing: fmt.Errorf("%w: session is being cleaned up", craft.ErrBusy)}
	delegate.SetDispatchGuard(guard)
	_, err := delegate.Delegate(context.Background(), task)
	require.ErrorIs(t, err, craft.ErrBusy)
	require.Equal(t, 1, guard.calls)
	require.Equal(t, 0, executor.executed, "a refused dispatch must not reach the executor")

	// Without a guard the same task dispatches through — and the terminal
	// executor outcome is counted by craft_delegations_total (the success
	// path never passes through settle()).
	metrics.ResetCraftMetrics()
	delegate.SetDispatchGuard(nil)
	result, err := delegate.Delegate(context.Background(), task)
	require.NoError(t, err)
	require.Equal(t, "succeeded", result.Status)
	require.Equal(t, 1, executor.executed)
	require.Equal(t, float64(1), metrics.CraftMetricsSnapshot()["craft_delegations_total{status=\"succeeded\"}"])
}

// Reuse of an already stored terminal outcome is NOT a dispatch: the guard
// is not even consulted on the retry-after-crash path.
func TestDelegateDispatchGuardDoesNotBlockStoredResultReuse(t *testing.T) {
	store := newFakeDelegationStore()
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}
	executor := &recordingDelegateExecutor{}
	delegate := NewCraftDelegateService(store, executor)
	task := guardWiringTask(scope, "ws-1")

	_, err := store.PrepareTask(context.Background(), task)
	require.NoError(t, err)
	stored := craft.Result{TaskID: task.ID, Status: "succeeded", Summary: "already settled"}
	require.NoError(t, store.SaveResult(context.Background(), task.Fence, stored))

	guard := &stubDispatchGuard{refusing: fmt.Errorf("%w: session is being cleaned up", craft.ErrBusy)}
	delegate.SetDispatchGuard(guard)
	result, err := delegate.Delegate(context.Background(), task)
	require.NoError(t, err)
	require.Equal(t, stored.Status, result.Status)
	require.Equal(t, 0, guard.calls, "stored-result reuse is observation, not dispatch")
	require.Equal(t, 0, executor.executed)
}

type stubRestoreGuard struct {
	calls    int
	refusing error
}

func (g *stubRestoreGuard) GuardRestore(_ context.Context, _ uint64, _ string) error {
	g.calls++
	return g.refusing
}

// The restore guard fires before any restore work: a session under teardown
// answers the guard's refusal even for a well-formed restore request.
func TestSnapshotRestoreGuardRefusesTeardown(t *testing.T) {
	env := newLifecycleEnv(t)
	snapshots := newSnapshotHarness(t, env)
	guard := &stubRestoreGuard{refusing: fmt.Errorf("%w: session is being cleaned up", craft.ErrBusy)}
	snapshots.SetRestoreGuard(guard)

	_, err := snapshots.Restore(context.Background(), env.scope, craft.SnapshotIDPrefix+"snap-1", 1)
	require.ErrorIs(t, err, craft.ErrBusy)
	require.Equal(t, 1, guard.calls)

	// Without a guard the request proceeds into the normal restore path
	// (it will fail on its own validation, which is fine here).
	snapshots.SetRestoreGuard(nil)
	_, err = snapshots.Restore(context.Background(), env.scope, craft.SnapshotIDPrefix+"snap-1", 1)
	require.Error(t, err)
	require.NotErrorIs(t, err, craft.ErrBusy)
	require.Equal(t, 1, guard.calls)
}

// newSnapshotHarness assembles a minimal snapshot service over the lifecycle
// env's durable stores. Its only job here is to prove guard ordering; the
// restore path itself is covered by the C05 suite.
func newSnapshotHarness(t *testing.T, env *lifecycleEnv) *CraftSnapshotService {
	t.Helper()
	svc, err := NewCraftSnapshotService(CraftSnapshotConfig{
		DB: env.db, Sessions: &usageViewSessions{rows: map[string]types.Session{}},
		Store: env.store, Versions: env.versions,
		Snapshots: env.snapshots, Files: newFakeSnapshotFiles(),
		Source: newFakeSnapshotSource(), ActiveRuns: CraftActiveRunsQuery(env.db),
		RuntimeDigest: "sha256:runtime",
	})
	require.NoError(t, err)
	return svc
}

// O03 review liveness fix: a crash between the object CAS and the provider
// delete strands the row in 'deleting'. The sweep re-drives it — the object
// is reclaimed (idempotent delete) instead of the ledger row living forever.
func TestLifecycleSweepRedrivesStrandedDeletingObject(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()

	require.NoError(t, env.svc.RecordOrphanCandidate(ctx, 1, "s1", "resource://orphan/stranded", "association never completed"))
	env.now.Store(10_000 + int64(craft.DefaultOrphanCandidateWindow/time.Second) + 1)

	// Simulate the crash window: the row already CAS'd to deleting.
	require.NoError(t, env.db.Exec(
		"UPDATE craft_lifecycle_states SET state = ? WHERE resource_ref = ?",
		craft.LifecycleStateDeleting, "resource://orphan/stranded").Error)

	require.NoError(t, env.svc.Sweep(ctx, 10))
	require.Equal(t, []string{"resource://orphan/stranded"}, env.objects.callList())

	var row craftLifecycleStateRow
	require.NoError(t, env.db.Where("resource_ref = ?", "resource://orphan/stranded").First(&row).Error)
	require.Equal(t, craft.LifecycleStateDeleted, row.State, "a stranded deleting row must be re-driven, not skipped")
}

// The sweep refreshes the craft_pending_decisions gauge from the durable
// interaction table.
func TestLifecycleSweepRefreshesPendingDecisionGauge(t *testing.T) {
	env := newLifecycleEnv(t)
	ctx := context.Background()
	env.insertPendingInteraction(t, "ix-g1")
	env.insertPendingInteraction(t, "ix-g2")

	require.NoError(t, env.svc.Sweep(ctx, 10))
	// Two pending interactions of the harness session... the harness may add
	// its own; assert at least the two we created are counted.
	require.GreaterOrEqual(t, metrics.CraftMetricsSnapshot()["craft_pending_decisions"], float64(2))
}
