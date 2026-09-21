package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/opencode"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---- recovery fakes ------------------------------------------------------

// fakeCraftRuns records durable parks so tests can assert exactly which
// existing waiting reason the recovery program transitioned to.
type fakeCraftRuns struct {
	mu    sync.Mutex
	parks []string
}

func (f *fakeCraftRuns) Get(_ context.Context, key agentruntime.RunKey) (agentruntime.Run, error) {
	return agentruntime.Run{Key: key, Status: "running"}, nil
}

func (f *fakeCraftRuns) Cancel(context.Context, agentruntime.RunKey) error { return nil }

func (f *fakeCraftRuns) WaitForDecision(_ context.Context, _ agentruntime.Fence, pending string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.parks = append(f.parks, pending)
	return nil
}

func (f *fakeCraftRuns) parkReasons() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.parks...)
}

// fakeCraftScopeResolver rebuilds the execution scope from durable state.
type fakeCraftScopeResolver struct {
	scope craft.Scope
	err   error
}

func (f *fakeCraftScopeResolver) ResolveScope(context.Context, agentruntime.RunKey) (craft.Scope, error) {
	return f.scope, f.err
}

// craftRecoveryTakeoverFence is the claiming takeover worker's fence: same
// run, new owner, new epoch — the crashed worker's fence stays inside the
// stored task.
func craftRecoveryTakeoverFence(task craft.Task) agentruntime.Fence {
	return agentruntime.Fence{
		RunKey: task.Fence.RunKey, Owner: "takeover-worker", Epoch: task.Fence.Epoch + 1,
	}
}

// newCraftRecoveryHarness prepares one prepared delegation on a healthy
// workspace and assembles the recovery program around injectable fakes.
func newCraftRecoveryHarness(t *testing.T, cfg CraftRecoveryConfig) (
	*fakeDelegationStore, *fakeDelegationExecutor, *fakeCraftRuns, *CraftRecovery, craft.Task,
) {
	t.Helper()
	store := newFakeDelegationStore()
	exec := &fakeDelegationExecutor{}
	runs := &fakeCraftRuns{}
	scope := delegateTestScope()
	store.workspaces[scope.SessionID] = craft.Workspace{
		ID: "ws-7", Scope: scope, SandboxID: "sbx-1", Generation: "1",
		OpenCodeSessionID: "oc-1", RuntimeDigest: "digest-v1", Revision: 1,
	}
	task := withDerivedDelegationID(delegateTestTask("recover"))
	task.PromptMessageID = "msg_recovery000000000000"
	task.Deadline = time.Now().Add(time.Hour)
	prepared, err := store.PrepareTask(context.Background(), task)
	require.NoError(t, err)
	if cfg.ObserveInterval <= 0 {
		cfg.ObserveInterval = time.Millisecond
	}
	if cfg.RuntimeDigest == "" {
		cfg.RuntimeDigest = "digest-v1"
	}
	recovery, err := NewCraftRecovery(store, exec, runs,
		&fakeCraftScopeResolver{scope: scope}, cfg)
	require.NoError(t, err)
	return store, exec, runs, recovery, prepared
}

func completedCraftObservation(task craft.Task) craft.Observation {
	return craft.Observation{
		SessionID: "oc-1", PromptMessageID: task.PromptMessageID,
		AssistantParentID: task.PromptMessageID, Completed: true, Idle: true, Finish: "stop",
	}
}

// ---- reuse route ---------------------------------------------------------

func TestCraftRecoveryReusesStoredResultWithoutAnyExecution(t *testing.T) {
	store, exec, runs, recovery, task := newCraftRecoveryHarness(t, CraftRecoveryConfig{})
	stored := craft.Result{TaskID: task.ID, Status: "succeeded", Summary: "already settled"}
	require.NoError(t, store.SaveResult(context.Background(), task.Fence, stored))

	result, err := recovery.Reconcile(context.Background(), craftRecoveryTakeoverFence(task), task.ID)
	require.NoError(t, err)
	require.Equal(t, stored, result)
	require.Equal(t, 0, exec.executeCount(), "reuse must never re-execute")
	require.Equal(t, 0, exec.observeCount(), "a stored result needs no observation")
	require.Empty(t, runs.parkReasons())
}

func TestCraftRecoveryReuseStillReauthorizesResources(t *testing.T) {
	store, exec, runs, recovery, task := newCraftRecoveryHarness(t, CraftRecoveryConfig{})
	require.NoError(t, store.SaveResult(context.Background(), task.Fence,
		craft.Result{TaskID: task.ID, Status: "succeeded"}))

	// The binding the delegation recorded no longer matches the session's
	// live workspace: the stored result must not be applied.
	store.mu.Lock()
	ws := store.workspaces[task.Scope.SessionID]
	ws.ID = "ws-rebound"
	store.workspaces[task.Scope.SessionID] = ws
	store.mu.Unlock()

	result, err := recovery.Reconcile(context.Background(), craftRecoveryTakeoverFence(task), task.ID)
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Equal(t, craft.Result{}, result, "a result without re-authorization must not be applied")
	require.Equal(t, 0, exec.executeCount())
	require.Equal(t, 0, exec.observeCount())
	require.Empty(t, runs.parkReasons())
}

// TestCraftRecoveryReuseFailsClosedOnRevokedOwnership proves re-authorization
// against the real store ACL: a workspace whose owner changed answers
// forbidden and blocks the reuse of the stored result.
func TestCraftRecoveryReuseFailsClosedOnRevokedOwnership(t *testing.T) {
	h := newCraftRecoveryRealHarness(t, "digest-v1")
	require.NoError(t, h.craftStore.SaveResult(context.Background(), h.fenceA,
		craft.Result{TaskID: h.task.ID, Status: "succeeded", Summary: "settled before revoke"}))
	h.expireLeaseA(t)
	fence := h.claimTakeover(t)
	require.NoError(t, h.db.Exec("UPDATE craft_workspaces SET owner_id = 'other-user'").Error)

	exec := &fakeDelegationExecutor{}
	recovery, err := NewCraftRecovery(h.craftStore, exec, h.runService,
		CraftRunScopeQuery(h.db), CraftRecoveryConfig{RuntimeDigest: "digest-v1"})
	require.NoError(t, err)
	result, err := recovery.Reconcile(context.Background(), fence, h.task.ID)
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Equal(t, craft.Result{}, result)
	require.Equal(t, 0, exec.observeCount())
	require.Equal(t, 0, exec.executeCount())
}

// ---- collect route -------------------------------------------------------

func TestCraftRecoveryCollectsExactCompletedUnderTakeoverEpoch(t *testing.T) {
	store, exec, runs, recovery, task := newCraftRecoveryHarness(t, CraftRecoveryConfig{})
	exec.onObserve = func(tk craft.Task) (craft.Observation, error) {
		return completedCraftObservation(tk), nil
	}
	takeover := craftRecoveryTakeoverFence(task)
	require.NotEqual(t, task.Fence, takeover, "takeover must carry a new owner/epoch")

	result, err := recovery.Reconcile(context.Background(), takeover, task.ID)
	require.NoError(t, err)
	require.Equal(t, "succeeded", result.Status)
	require.Equal(t, 1, exec.observeCount())
	require.Equal(t, 0, exec.executeCount(), "recovery never resubmits the prompt")
	require.Empty(t, runs.parkReasons())
	savedUnder := store.savedFence(task.ID)
	require.Equal(t, takeover, savedUnder, "the takeover epoch must be able to write the result back")
}

func TestCraftRecoverySettlesDefinitiveFailuresFromObservation(t *testing.T) {
	_, exec, runs, recovery, task := newCraftRecoveryHarness(t, CraftRecoveryConfig{})
	exec.onObserve = func(tk craft.Task) (craft.Observation, error) {
		return craft.Observation{
			SessionID: "oc-1", PromptMessageID: tk.PromptMessageID,
			AssistantParentID: tk.PromptMessageID, Completed: true, Idle: true, Finish: "error",
		}, nil
	}
	result, err := recovery.Reconcile(context.Background(), craftRecoveryTakeoverFence(task), task.ID)
	require.NoError(t, err)
	require.Equal(t, "failed", result.Status)
	require.Empty(t, runs.parkReasons())
}

// ---- observe route -------------------------------------------------------

func TestCraftRecoveryObservesUntilTerminalUnderDeadline(t *testing.T) {
	store, exec, runs, recovery, task := newCraftRecoveryHarness(t,
		CraftRecoveryConfig{ObserveInterval: time.Millisecond})
	rounds := 0
	exec.onObserve = func(tk craft.Task) (craft.Observation, error) {
		rounds++
		if rounds < 3 {
			return craft.Observation{
				SessionID: "oc-1", PromptMessageID: tk.PromptMessageID, Idle: false, PendingTool: true,
			}, nil
		}
		return completedCraftObservation(tk), nil
	}
	result, err := recovery.Reconcile(context.Background(), craftRecoveryTakeoverFence(task), task.ID)
	require.NoError(t, err)
	require.Equal(t, "succeeded", result.Status)
	require.Equal(t, 3, exec.observeCount(), "a still-running remote is observed, not parked or executed")
	require.Equal(t, 0, exec.executeCount())
	require.Empty(t, runs.parkReasons())
	require.Equal(t, craftRecoveryTakeoverFence(task), store.savedFence(task.ID))
}

func TestCraftRecoveryWaitsDurablyWhenDeadlineExhaustedUndeterminable(t *testing.T) {
	store, exec, runs, recovery, task := newCraftRecoveryHarness(t,
		CraftRecoveryConfig{ObserveInterval: 5 * time.Millisecond})
	store.mu.Lock()
	stored := store.tasks[task.ID]
	stored.Deadline = time.Now().Add(40 * time.Millisecond)
	store.tasks[task.ID] = stored
	store.mu.Unlock()
	exec.onObserve = func(tk craft.Task) (craft.Observation, error) {
		return craft.Observation{
			SessionID: "oc-1", PromptMessageID: tk.PromptMessageID, Idle: false, PendingTool: true,
		}, nil
	}

	result, err := recovery.Reconcile(context.Background(), craftRecoveryTakeoverFence(task), task.ID)
	require.ErrorIs(t, err, craft.ErrUnknown, "undeterminable outcome keeps the run pending")
	require.Equal(t, craft.Result{}, result)
	require.Equal(t, []string{task.ToolCallID}, runs.parkReasons(),
		"the park reuses the existing unknown-outcome pending identity (the tool call id)")
	store.mu.Lock()
	_, hasResult := store.results[task.ID]
	store.mu.Unlock()
	require.False(t, hasResult, "unknown must never be persisted as a result")
	require.Equal(t, 0, exec.executeCount())

	// A restart of the recovery program keeps waiting instead of resubmitting.
	_, err = recovery.Reconcile(context.Background(), craftRecoveryTakeoverFence(task), task.ID)
	require.ErrorIs(t, err, craft.ErrUnknown)
	require.Equal(t, 0, exec.executeCount())
	require.Len(t, runs.parkReasons(), 2)
}

// ---- wait route ----------------------------------------------------------

func TestCraftRecoveryWaitsOnRuntimeDigestChangeWithoutObserving(t *testing.T) {
	_, exec, runs, recovery, task := newCraftRecoveryHarness(t,
		CraftRecoveryConfig{RuntimeDigest: "digest-v2"})
	exec.onObserve = func(tk craft.Task) (craft.Observation, error) {
		return completedCraftObservation(task), nil
	}
	result, err := recovery.Reconcile(context.Background(), craftRecoveryTakeoverFence(task), task.ID)
	require.ErrorIs(t, err, ErrSandboxUnavailable)
	require.Equal(t, craft.Result{}, result)
	require.Equal(t, 0, exec.observeCount(), "an incompatible image must not be observed through")
	require.Equal(t, []string{craftRecoveryWaitSandbox}, runs.parkReasons())
}

func TestCraftRecoveryNeverContinuesOnEmptyOpenCodeSession(t *testing.T) {
	store, exec, runs, recovery, task := newCraftRecoveryHarness(t, CraftRecoveryConfig{})
	store.mu.Lock()
	ws := store.workspaces[task.Scope.SessionID]
	ws.OpenCodeSessionID = ""
	store.workspaces[task.Scope.SessionID] = ws
	store.mu.Unlock()

	result, err := recovery.Reconcile(context.Background(), craftRecoveryTakeoverFence(task), task.ID)
	require.ErrorIs(t, err, ErrSandboxUnavailable)
	require.Equal(t, craft.Result{}, result)
	require.Equal(t, 0, exec.observeCount(), "an empty session must never be continued")
	require.Equal(t, []string{craftRecoveryWaitSandbox}, runs.parkReasons())
}

func TestCraftRecoveryMapsObserveFailuresToExistingWaitingReasons(t *testing.T) {
	t.Run("missing runtime maps to sandbox wait", func(t *testing.T) {
		_, exec, runs, recovery, task := newCraftRecoveryHarness(t, CraftRecoveryConfig{})
		exec.onObserve = func(craft.Task) (craft.Observation, error) {
			return craft.Observation{}, fmt.Errorf("%w: workspace has no bound OpenCode session", craft.ErrUnsupported)
		}
		_, err := recovery.Reconcile(context.Background(), craftRecoveryTakeoverFence(task), task.ID)
		require.ErrorIs(t, err, ErrSandboxUnavailable)
		require.Equal(t, []string{craftRecoveryWaitSandbox}, runs.parkReasons())
	})
	t.Run("undetermined snapshot maps to unknown wait", func(t *testing.T) {
		_, exec, runs, recovery, task := newCraftRecoveryHarness(t, CraftRecoveryConfig{})
		exec.onObserve = func(craft.Task) (craft.Observation, error) {
			return craft.Observation{}, errors.New("snapshot read failed mid-request")
		}
		_, err := recovery.Reconcile(context.Background(), craftRecoveryTakeoverFence(task), task.ID)
		require.ErrorIs(t, err, craft.ErrUnknown)
		require.Equal(t, []string{task.ToolCallID}, runs.parkReasons())
	})
}

func TestCraftRecoveryRejectsForeignTaskAndBrokenInput(t *testing.T) {
	_, _, _, recovery, task := newCraftRecoveryHarness(t, CraftRecoveryConfig{})

	_, err := recovery.Reconcile(context.Background(), craftRecoveryTakeoverFence(task), "dlg_missing")
	require.ErrorIs(t, err, craft.ErrNotFound)

	foreign := craftRecoveryTakeoverFence(task)
	foreign.RunID = "another-run"
	_, err = recovery.Reconcile(context.Background(), foreign, task.ID)
	require.ErrorIs(t, err, craft.ErrForbidden)
}

// ---- real durable store --------------------------------------------------

type craftRecoveryRealHarness struct {
	db         *gorm.DB
	runs       *repository.AgentRunStore
	runService *AgentRunService
	craftStore craft.Store
	key        agentruntime.RunKey
	fenceA     agentruntime.Fence
	task       craft.Task
}

func newCraftRecoveryRealHarness(t *testing.T, digest string) *craftRecoveryRealHarness {
	t.Helper()
	db := openDurableRunTestDB(t)
	runs := repository.NewAgentRunStore(db)
	runService := NewAgentRunService(runs)
	key := agentruntime.RunKey{TenantID: 1, RunID: "craft-rec-run"}
	user, _ := json.Marshal(map[string]any{"role": "user", "content": "delegate one round"})
	assistant, _ := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	_, err := runs.Admit(context.Background(), agentruntime.Admission{
		Key: key, SessionID: "s1", UserID: "u1", RequestID: "req-1",
		AssistantMessageID: "asst-1", RequestHash: "rh",
		Snapshot:    json.RawMessage(`{"version":1,"query":"delegate","model_id":"m"}`),
		UserMessage: user, AssistantMessage: assistant,
		Deadline: time.Now().Add(10 * time.Minute),
	})
	require.NoError(t, err)
	fenceA, err := runs.Claim(context.Background(), key, "worker-a", 30*time.Second)
	require.NoError(t, err)
	craftStore := repository.NewCraftStore(db)
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}
	workspace, err := craftStore.PutWorkspace(context.Background(), craft.Workspace{
		Scope: scope, SandboxID: "sbx-1", Generation: "1",
		OpenCodeSessionID: "oc-real-1", RuntimeDigest: digest,
	}, 0)
	require.NoError(t, err)
	_, err = runs.EnsureToolPlan(context.Background(), fenceA, agentruntime.ToolPlan{
		Version: 1, CallID: "call-c-1", Name: "craft_delegate", Identity: "craft_delegate",
		ArgsHash: "ah-1", Args: json.RawMessage(`{"goal":"recover"}`),
	})
	require.NoError(t, err)
	promptID, err := opencode.NewMessageID()
	require.NoError(t, err)
	task := craft.Task{
		ID: "dlg_craft_rec_1", ToolCallID: "call-c-1", Prompt: "goal: recover the round",
		RequestHash: "rh-1", Scope: scope, Fence: fenceA, WorkspaceID: workspace.ID,
		PromptMessageID: promptID, Deadline: time.Now().Add(5 * time.Minute),
	}
	_, err = craftStore.PrepareTask(context.Background(), task)
	require.NoError(t, err)
	return &craftRecoveryRealHarness{
		db: db, runs: runs, runService: runService, craftStore: craftStore,
		key: key, fenceA: fenceA, task: task,
	}
}

// expireLeaseA force-expires worker A's lease so a takeover claim is possible
// without waiting in wall-clock time.
func (h *craftRecoveryRealHarness) expireLeaseA(t *testing.T) {
	t.Helper()
	require.NoError(t, h.db.Exec(
		"UPDATE agent_runs SET lease_until = '2000-01-01 00:00:00' WHERE tenant_id = 1 AND run_id = ?",
		h.key.RunID).Error)
}

func (h *craftRecoveryRealHarness) claimTakeover(t *testing.T) agentruntime.Fence {
	t.Helper()
	fence, err := h.runs.Claim(context.Background(), h.key, "takeover-worker", 30*time.Second)
	require.NoError(t, err)
	require.Greater(t, fence.Epoch, h.fenceA.Epoch)
	return fence
}

func TestCraftRecoveryTakeoverEpochWritesBackAndOldEpochIsRejected(t *testing.T) {
	h := newCraftRecoveryRealHarness(t, "digest-v1")
	exec := &fakeDelegationExecutor{}
	exec.onObserve = func(tk craft.Task) (craft.Observation, error) {
		return completedCraftObservation(tk), nil
	}
	recovery, err := NewCraftRecovery(h.craftStore, exec, h.runService,
		CraftRunScopeQuery(h.db), CraftRecoveryConfig{RuntimeDigest: "digest-v1"})
	require.NoError(t, err)

	h.expireLeaseA(t)
	takeover := h.claimTakeover(t)

	result, err := recovery.Reconcile(context.Background(), takeover, h.task.ID)
	require.NoError(t, err)
	require.Equal(t, "succeeded", result.Status)
	require.Equal(t, 0, exec.executeCount(), "lease loss and takeover never resubmit the prompt")

	// The stored prompt identity is untouched: no second id was minted.
	stored, err := h.craftStore.GetTask(context.Background(), h.task.Scope, h.task.ID)
	require.NoError(t, err)
	require.Equal(t, h.task.PromptMessageID, stored.PromptMessageID)

	// A late result write from the superseded epoch must be rejected.
	lateErr := h.craftStore.SaveResult(context.Background(), h.fenceA,
		craft.Result{TaskID: h.task.ID, Status: "failed", Summary: "stale worker late write"})
	require.Error(t, lateErr, "an old epoch must not write results")
	require.ErrorIs(t, lateErr, craft.ErrConflict)
	require.Contains(t, lateErr.Error(), "lease", "the rejection must name the lost lease")

	// A late event append from the superseded epoch must be rejected too.
	_, eventErr := h.runs.AppendEvent(context.Background(), h.fenceA,
		agentruntime.RunEvent{Type: "craft", Payload: json.RawMessage(`{}`)})
	require.ErrorIs(t, eventErr, agentruntime.ErrLeaseLost)
}

func TestCraftRecoveryScopeResolverRebuildsAuthorityFromDurableState(t *testing.T) {
	h := newCraftRecoveryRealHarness(t, "digest-v1")
	resolver := CraftRunScopeQuery(h.db)
	scope, err := resolver.ResolveScope(context.Background(), h.key)
	require.NoError(t, err)
	require.Equal(t, craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}, scope)

	require.NoError(t, h.db.Exec("DELETE FROM sessions WHERE id = 's1'").Error)
	_, err = resolver.ResolveScope(context.Background(), h.key)
	require.ErrorIs(t, err, craft.ErrNotFound)

	h2 := newCraftRecoveryRealHarness(t, "digest-v1")
	require.NoError(t, h2.db.Exec("UPDATE sessions SET engine_type = 'builtin' WHERE id = 's1'").Error)
	_, err = CraftRunScopeQuery(h2.db).ResolveScope(context.Background(), h2.key)
	require.ErrorIs(t, err, craft.ErrForbidden)

	h3 := newCraftRecoveryRealHarness(t, "digest-v1")
	require.NoError(t, h3.db.Exec(
		"UPDATE agent_runs SET status = 'failed' WHERE run_id = ?", h3.key.RunID).Error)
	_, err = CraftRunScopeQuery(h3.db).ResolveScope(context.Background(), h3.key)
	require.ErrorIs(t, err, craft.ErrConflict)
}

func TestCraftRecoveryChatDoesNotUnlockUnknownWait(t *testing.T) {
	h := newCraftRecoveryRealHarness(t, "digest-v1")
	exec := &fakeDelegationExecutor{}
	exec.onObserve = func(craft.Task) (craft.Observation, error) {
		return craft.Observation{}, errors.New("session snapshot unavailable")
	}
	recovery, err := NewCraftRecovery(h.craftStore, exec, h.runService,
		CraftRunScopeQuery(h.db), CraftRecoveryConfig{RuntimeDigest: "digest-v1"})
	require.NoError(t, err)

	h.expireLeaseA(t)
	takeover := h.claimTakeover(t)
	_, err = recovery.Reconcile(context.Background(), takeover, h.task.ID)
	require.ErrorIs(t, err, craft.ErrUnknown)

	run, err := h.runs.Get(context.Background(), h.key)
	require.NoError(t, err)
	require.Equal(t, "waiting_user", run.Status)
	require.Equal(t, h.task.ToolCallID, run.WaitReason)

	// An ordinary chat message cannot unlock the unknown wait: the session's
	// durable run slot is still held by the waiting run.
	chatUser, _ := json.Marshal(map[string]any{"role": "user", "content": "just chatting"})
	chatAssistant, _ := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	_, err = h.runService.Submit(context.Background(), agentruntime.Admission{
		Key:       agentruntime.RunKey{TenantID: 1, RunID: "craft-rec-run-2"},
		SessionID: "s1", UserID: "u1", RequestID: "req-2", AssistantMessageID: "asst-2",
		RequestHash: "rh2", Snapshot: json.RawMessage(`{"version":1}`),
		UserMessage: chatUser, AssistantMessage: chatAssistant,
		Deadline: time.Now().Add(time.Minute),
	})
	require.ErrorIs(t, err, agentruntime.ErrRunActive)

	run, err = h.runs.Get(context.Background(), h.key)
	require.NoError(t, err)
	require.Equal(t, "waiting_user", run.Status, "chat must not release the unknown wait")
	require.Equal(t, h.task.ToolCallID, run.WaitReason)
	require.Equal(t, 0, exec.executeCount())
}

func TestCraftRecoveryManualRetryUsesDecisionWithoutResubmission(t *testing.T) {
	h := newCraftRecoveryRealHarness(t, "digest-v1")
	exec := &fakeDelegationExecutor{}
	exec.onObserve = func(craft.Task) (craft.Observation, error) {
		return craft.Observation{}, errors.New("session snapshot unavailable")
	}
	recovery, err := NewCraftRecovery(h.craftStore, exec, h.runService,
		CraftRunScopeQuery(h.db), CraftRecoveryConfig{RuntimeDigest: "digest-v1"})
	require.NoError(t, err)

	h.expireLeaseA(t)
	takeover := h.claimTakeover(t)
	_, err = recovery.Reconcile(context.Background(), takeover, h.task.ID)
	require.ErrorIs(t, err, craft.ErrUnknown)

	current, err := h.runs.Get(context.Background(), h.key)
	require.NoError(t, err)
	require.Equal(t, "waiting_user", current.Status)

	// The manual retry goes through the existing durable decision surface and
	// keeps the side-effect risk statement in its reason.
	h.runService.SetDecisionPolicy(func(context.Context, agentruntime.Decision) error { return nil })
	actorCtx := context.WithValue(context.Background(), types.UserIDContextKey, "u1")
	const riskNote = "operator verified the external runtime and accepts duplicate side-effect risk"
	retried, err := h.runService.Resolve(actorCtx, h.key, agentruntime.Decision{
		PendingID: current.WaitReason, DecisionID: "dec-craft-retry-1",
		ToolCallID: h.task.ToolCallID, ArgsHash: "ah-1", Action: "retry",
		Reason: riskNote, ExpectedRevision: current.Revision,
	})
	require.NoError(t, err)
	require.Equal(t, "queued", retried.Status)

	var storedReason string
	require.NoError(t, h.db.Raw(
		"SELECT reason FROM agent_run_decisions WHERE run_id = ? AND decision_id = 'dec-craft-retry-1'",
		h.key.RunID).Scan(&storedReason).Error)
	require.Equal(t, riskNote, storedReason)

	// The retried attempt still resolves through observation only: the
	// decision re-queued the run, a fresh claim executes it, and the
	// delegation row exists — so the delegate service never re-posts.
	h.expireLeaseA(t) // the park already released the lease; force-claimable
	nextFence := h.claimTakeover(t)
	require.Greater(t, nextFence.Epoch, takeover.Epoch)
	after := &fakeDelegationExecutor{}
	after.onExecute = func(craft.Task) (craft.Result, error) {
		return craft.Result{}, errors.New("retry must never re-dispatch the prompt")
	}
	after.onObserve = func(tk craft.Task) (craft.Observation, error) {
		return completedCraftObservation(tk), nil
	}
	delegate := NewCraftDelegateService(h.craftStore, after)
	retriedTask := h.task
	retriedTask.Fence = nextFence
	result, err := delegate.Delegate(context.Background(), retriedTask)
	require.NoError(t, err)
	require.Equal(t, "succeeded", result.Status)
	require.Equal(t, 0, after.executeCount(), "a retried attempt observes; it never resubmits")
	require.Equal(t, h.task.ID, result.TaskID)
}
