package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/opencode"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/mcp"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/modules/execution/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---- delegation fakes ----------------------------------------------------

// fakeDelegationStore reproduces the R02 store semantics Delegate relies on:
// idempotent PrepareTask by task id with request-hash conflict, prompt
// message id assignment, and result persistence keyed by task id.
type fakeDelegationStore struct {
	mu          sync.Mutex
	workspaces  map[string]craft.Workspace
	tasks       map[string]craft.Task
	results     map[string]craft.Result
	savedFences map[string]agentruntime.Fence
	prepares    int
	promptSeq   int
}

func newFakeDelegationStore() *fakeDelegationStore {
	return &fakeDelegationStore{
		workspaces:  map[string]craft.Workspace{},
		tasks:       map[string]craft.Task{},
		results:     map[string]craft.Result{},
		savedFences: map[string]agentruntime.Fence{},
	}
}

func (s *fakeDelegationStore) GetWorkspace(_ context.Context, scope craft.Scope) (craft.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws, ok := s.workspaces[scope.SessionID]
	if !ok {
		return craft.Workspace{}, craft.ErrNotFound
	}
	return ws, nil
}

func (s *fakeDelegationStore) PutWorkspace(_ context.Context, in craft.Workspace, expected int64) (craft.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	in.Revision = expected + 1
	s.workspaces[in.Scope.SessionID] = in
	return in, nil
}

func (s *fakeDelegationStore) PrepareTask(_ context.Context, in craft.Task) (craft.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prepares++
	if in.ID == "" {
		in.ID = fmt.Sprintf("dlg_%d", s.prepares)
	}
	if in.PromptMessageID == "" {
		s.promptSeq++
		in.PromptMessageID = fmt.Sprintf("msg_%d", s.promptSeq)
	}
	if stored, ok := s.tasks[in.ID]; ok {
		if stored.RequestHash != in.RequestHash {
			return craft.Task{}, fmt.Errorf("%w: different request for %s", craft.ErrConflict, in.ID)
		}
		return stored, nil
	}
	s.tasks[in.ID] = in
	return in, nil
}

func (s *fakeDelegationStore) GetTask(_ context.Context, scope craft.Scope, id string) (craft.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[id]
	if !ok {
		return craft.Task{}, craft.ErrNotFound
	}
	return task, nil
}

func (s *fakeDelegationStore) SaveResult(_ context.Context, fence agentruntime.Fence, result craft.Result) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tasks[result.TaskID]; !ok {
		return fmt.Errorf("%w: delegation %s", craft.ErrNotFound, result.TaskID)
	}
	s.results[result.TaskID] = result
	s.savedFences[result.TaskID] = fence
	return nil
}

func (s *fakeDelegationStore) GetResult(_ context.Context, scope craft.Scope, id string) (craft.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, ok := s.results[id]
	if !ok {
		return craft.Result{}, craft.ErrNotFound
	}
	return result, nil
}

func (s *fakeDelegationStore) savedFence(id string) agentruntime.Fence {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.savedFences[id]
}

func (s *fakeDelegationStore) prepareCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.prepares
}

// fakeDelegationExecutor records every call.
type fakeDelegationExecutor struct {
	mu        sync.Mutex
	executes  int
	observes  int
	aborts    int
	onExecute func(craft.Task) (craft.Result, error)
	onObserve func(craft.Task) (craft.Observation, error)
}

func (e *fakeDelegationExecutor) Execute(ctx context.Context, task craft.Task) (craft.Result, error) {
	e.mu.Lock()
	e.executes++
	fn := e.onExecute
	e.mu.Unlock()
	if fn != nil {
		return fn(task)
	}
	return craft.Result{TaskID: task.ID, Status: "succeeded", Summary: "sub round done"}, nil
}

func (e *fakeDelegationExecutor) Observe(ctx context.Context, task craft.Task) (craft.Observation, error) {
	e.mu.Lock()
	e.observes++
	fn := e.onObserve
	e.mu.Unlock()
	if fn != nil {
		return fn(task)
	}
	return craft.Observation{SessionID: "oc-1", PromptMessageID: task.PromptMessageID, Idle: true}, nil
}

func (e *fakeDelegationExecutor) Abort(ctx context.Context, task craft.Task) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.aborts++
	return nil
}

func (e *fakeDelegationExecutor) executeCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.executes
}

func (e *fakeDelegationExecutor) observeCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.observes
}

func delegateTestScope() craft.Scope {
	return craft.Scope{TenantID: 7, UserID: "user-7", SessionID: "sess-7"}
}

func delegateTestTask(goal string) craft.Task {
	scope := delegateTestScope()
	return craft.Task{
		ToolCallID:  "call-1",
		Prompt:      "build: " + goal,
		RequestHash: "hash-" + goal,
		Scope:       scope,
		Fence:       agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: scope.TenantID, RunID: "run-1"}, Owner: "worker-1", Epoch: 2},
		WorkspaceID: "ws-7",
		Deadline:    time.Now().Add(time.Hour),
	}
}

// ---- Delegate semantics --------------------------------------------------

// withDerivedDelegationID mirrors production: Delegate derives the durable
// delegation id before its first store call.
func withDerivedDelegationID(task craft.Task) craft.Task {
	task.ID = craftDelegationID(task)
	return task
}

func TestDelegateReusesStoredResultWithoutExecution(t *testing.T) {
	store := newFakeDelegationStore()
	exec := &fakeDelegationExecutor{}
	svc := NewCraftDelegateService(store, exec)
	task := withDerivedDelegationID(delegateTestTask("ship"))
	prepared, err := store.PrepareTask(context.Background(), task)
	require.NoError(t, err)
	require.NoError(t, store.SaveResult(context.Background(), task.Fence, craft.Result{TaskID: prepared.ID, Status: "succeeded", Summary: "already done"}))

	result, err := svc.Delegate(context.Background(), task)
	require.NoError(t, err)
	require.Equal(t, "already done", result.Summary)
	require.Equal(t, 0, exec.executeCount(), "a stored result must never re-execute")
	require.Equal(t, 0, exec.observeCount())
	require.Equal(t, 1, store.prepareCount(), "reuse must not re-prepare")
}

func TestDelegateFreshGoesThroughPrepareTaskAndExecute(t *testing.T) {
	store := newFakeDelegationStore()
	exec := &fakeDelegationExecutor{}
	svc := NewCraftDelegateService(store, exec)

	result, err := svc.Delegate(context.Background(), delegateTestTask("ship"))
	require.NoError(t, err)
	require.Equal(t, "succeeded", result.Status)
	require.Equal(t, 1, exec.executeCount())
	require.Equal(t, 1, store.prepareCount(), "planned delegations must pass through PrepareTask")

	var stored craft.Task
	for _, task := range store.tasks {
		stored = task
	}
	require.Equal(t, result.TaskID, stored.ID)
	require.NotEmpty(t, stored.PromptMessageID)
}

func TestDelegateSettlesDispatchingUnknownByObservationOnly(t *testing.T) {
	store := newFakeDelegationStore()
	exec := &fakeDelegationExecutor{}
	exec.onExecute = func(craft.Task) (craft.Result, error) {
		return craft.Result{}, errors.New("execute must not re-dispatch")
	}
	exec.onObserve = func(task craft.Task) (craft.Observation, error) {
		return craft.Observation{
			SessionID: "oc-1", PromptMessageID: task.PromptMessageID,
			AssistantParentID: task.PromptMessageID, Completed: true, Idle: true, Finish: "stop",
		}, nil
	}
	svc := NewCraftDelegateService(store, exec)

	original := withDerivedDelegationID(delegateTestTask("ship"))
	stored, err := store.PrepareTask(context.Background(), original)
	require.NoError(t, err)

	// A recovered worker replays the same tool call under a NEW fence.
	recovered := original
	recovered.Fence = agentruntime.Fence{RunKey: original.Fence.RunKey, Owner: "worker-2", Epoch: 9}
	result, err := svc.Delegate(context.Background(), recovered)
	require.NoError(t, err)
	require.Equal(t, "succeeded", result.Status, "a completed runtime round settles as succeeded")
	require.Equal(t, 0, exec.executeCount(), "dispatching-unknown must be observed, never re-executed")
	require.Equal(t, 1, exec.observeCount())
	require.Equal(t, recovered.Fence, store.savedFence(stored.ID), "settlement uses the caller's live fence")
}

func TestDelegateNeverAnsweredIdlePromptSettlesFailed(t *testing.T) {
	store := newFakeDelegationStore()
	exec := &fakeDelegationExecutor{}
	exec.onObserve = func(craft.Task) (craft.Observation, error) {
		// Idle runtime, no pending tool, and no assistant round ever parented
		// to this delegation's prompt (interrupted before the POST landed).
		return craft.Observation{SessionID: "oc-1", PromptMessageID: "msg_1", Idle: true}, nil
	}
	svc := NewCraftDelegateService(store, exec)

	task := withDerivedDelegationID(delegateTestTask("ship"))
	stored, err := store.PrepareTask(context.Background(), task)
	require.NoError(t, err)
	result, err := svc.Delegate(context.Background(), task)
	require.NoError(t, err)
	require.Equal(t, "failed", result.Status)
	require.Equal(t, 0, exec.executeCount())
	_, err = store.GetResult(context.Background(), task.Scope, stored.ID)
	require.NoError(t, err, "the definitive failure is persisted, not parked until deadline")
}

func TestDelegateKeepsPendingWhenObservationInconclusive(t *testing.T) {
	store := newFakeDelegationStore()
	exec := &fakeDelegationExecutor{}
	exec.onObserve = func(craft.Task) (craft.Observation, error) {
		// Sub-execution still working: runtime busy, nothing terminal.
		return craft.Observation{SessionID: "oc-1", PromptMessageID: "msg_1"}, nil
	}
	svc := NewCraftDelegateService(store, exec)

	task := withDerivedDelegationID(delegateTestTask("ship"))
	_, err := store.PrepareTask(context.Background(), task)
	require.NoError(t, err)
	_, err = svc.Delegate(context.Background(), task)
	require.Error(t, err)
	require.ErrorIs(t, err, craft.ErrUnknown, "an unprovable outcome stays pending")
	require.Equal(t, 0, exec.executeCount())
	for id := range store.tasks {
		_, err = store.GetResult(context.Background(), task.Scope, id)
		require.ErrorIs(t, err, craft.ErrNotFound, "unknown must never be persisted")
	}
}

func TestDelegateReportsFailureAtDeadline(t *testing.T) {
	// Inconclusive observation past the deadline settles as a stated failure.
	store := newFakeDelegationStore()
	exec := &fakeDelegationExecutor{}
	exec.onObserve = func(craft.Task) (craft.Observation, error) {
		return craft.Observation{SessionID: "oc-1", PromptMessageID: "msg_1"}, nil
	}
	svc := NewCraftDelegateService(store, exec)
	task := withDerivedDelegationID(delegateTestTask("ship"))
	task.Deadline = time.Now().Add(-time.Minute)
	_, err := store.PrepareTask(context.Background(), task)
	require.NoError(t, err)
	result, err := svc.Delegate(context.Background(), task)
	require.NoError(t, err)
	require.Equal(t, "failed", result.Status)
	require.Equal(t, 0, exec.executeCount())

	// A fresh delegation already past its deadline fails without any POST.
	store2 := newFakeDelegationStore()
	exec2 := &fakeDelegationExecutor{}
	exec2.onExecute = func(craft.Task) (craft.Result, error) {
		return craft.Result{}, errors.New("must not dispatch past the deadline")
	}
	svc2 := NewCraftDelegateService(store2, exec2)
	late := delegateTestTask("late")
	late.Deadline = time.Now().Add(-time.Minute)
	result2, err := svc2.Delegate(context.Background(), late)
	require.NoError(t, err)
	require.Equal(t, "failed", result2.Status)
	require.Equal(t, 0, exec2.executeCount())
	_, err = store2.GetResult(context.Background(), delegateTestScope(), result2.TaskID)
	require.NoError(t, err)
}

func TestDelegatePropagatesExecutorUnknownWithoutPersisting(t *testing.T) {
	store := newFakeDelegationStore()
	exec := &fakeDelegationExecutor{}
	exec.onExecute = func(craft.Task) (craft.Result, error) {
		return craft.Result{}, fmt.Errorf("stream lost: %w", craft.ErrUnknown)
	}
	svc := NewCraftDelegateService(store, exec)

	task := delegateTestTask("ship")
	_, err := svc.Delegate(context.Background(), task)
	require.ErrorIs(t, err, craft.ErrUnknown)
	require.Equal(t, 1, exec.executeCount())
	for id := range store.tasks {
		_, rerr := store.GetResult(context.Background(), task.Scope, id)
		require.ErrorIs(t, rerr, craft.ErrNotFound, "unknown must never be persisted")
	}
}

func TestDelegateConflictsOnSameCallWithDifferentRequest(t *testing.T) {
	store := newFakeDelegationStore()
	exec := &fakeDelegationExecutor{}
	// The first attempt dispatches but its outcome stays unknown, so no
	// result row exists for the retry to reuse.
	exec.onExecute = func(craft.Task) (craft.Result, error) {
		return craft.Result{}, fmt.Errorf("lost: %w", craft.ErrUnknown)
	}
	svc := NewCraftDelegateService(store, exec)

	_, err := svc.Delegate(context.Background(), delegateTestTask("ship"))
	require.ErrorIs(t, err, craft.ErrUnknown)
	_, err = svc.Delegate(context.Background(), delegateTestTask("ship differently"))
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Equal(t, 1, exec.executeCount())
}

func TestDelegateRejectsIncompleteTask(t *testing.T) {
	svc := NewCraftDelegateService(newFakeDelegationStore(), &fakeDelegationExecutor{})
	task := delegateTestTask("ship")
	task.Fence = agentruntime.Fence{}
	_, err := svc.Delegate(context.Background(), task)
	require.ErrorIs(t, err, craft.ErrInvalidInput)
}

func TestUnavailableCraftExecutorFailsClosed(t *testing.T) {
	exec := NewUnavailableCraftExecutor("dial pending")
	_, err := exec.Execute(context.Background(), craft.Task{})
	require.ErrorIs(t, err, craft.ErrUnsupported)
	require.NotErrorIs(t, err, craft.ErrUnknown)
}

// ---- registration gate ---------------------------------------------------

func newCraftGateService(t *testing.T, db *gorm.DB, exec craft.Executor) *agentService {
	t.Helper()
	manager := mcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	store := repository.NewCraftStore(db)
	delegation, err := NewCraftDelegation(store, exec)
	require.NoError(t, err)
	return &agentService{db: db, mcpManager: manager, craft: delegation}
}

func seedCraftWorkspace(t *testing.T, db *gorm.DB, sessionID string) craft.Workspace {
	t.Helper()
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: sessionID}
	ws, err := repository.NewCraftStore(db).PutWorkspace(durableRunCtx(), craft.Workspace{
		Scope:             scope,
		SandboxID:         "sbx-" + sessionID,
		Generation:        "g1",
		OpenCodeSessionID: "oc-" + sessionID,
		RuntimeDigest:     "digest-craft",
	}, 0)
	require.NoError(t, err)
	return ws
}

func TestRegisterCraftDelegateToolGatesOnCraftTrpcSession(t *testing.T) {
	db := openDurableRunTestDB(t)
	svc := newCraftGateService(t, db, &fakeDelegationExecutor{})
	ctx := durableRunCtx()

	// s1 is a trpc session (created by the shared helper) with a workspace.
	seedCraftWorkspace(t, db, "s1")
	trpcRegistry := tools.NewToolRegistry()
	require.NoError(t, svc.registerCraftDelegateTool(ctx, trpcRegistry, &types.AgentConfig{}, "s1"))
	require.Contains(t, trpcRegistry.ListTools(), tools.ToolCraftDelegate)
	modelNames := make([]string, 0)
	for _, def := range trpcRegistry.GetModelFunctionDefinitions() {
		modelNames = append(modelNames, def.Name)
	}
	require.Contains(t, modelNames, tools.ToolCraftDelegate)

	// A builtin-engine session keeps the unchanged builtin tool set even
	// when a craft workspace is bound.
	require.NoError(t, db.Exec("INSERT INTO sessions (id, tenant_id, title, user_id, engine_type)"+
		" VALUES ('s-builtin',1,'builtin','u1','builtin')").Error)
	seedCraftWorkspace(t, db, "s-builtin")
	builtinRegistry := tools.NewToolRegistry()
	require.NoError(t, svc.registerCraftDelegateTool(ctx, builtinRegistry, &types.AgentConfig{}, "s-builtin"))
	require.NotContains(t, builtinRegistry.ListTools(), tools.ToolCraftDelegate)

	// A trpc session without a craft workspace is not a craft session.
	require.NoError(t, db.Exec("INSERT INTO sessions (id, tenant_id, title, user_id, engine_type)"+
		" VALUES ('s-plain',1,'plain','u1','trpc')").Error)
	plainRegistry := tools.NewToolRegistry()
	require.NoError(t, svc.registerCraftDelegateTool(ctx, plainRegistry, &types.AgentConfig{}, "s-plain"))
	require.NotContains(t, plainRegistry.ListTools(), tools.ToolCraftDelegate)

	// Without the craft assembly the hook is a no-op.
	bare := &agentService{db: db}
	bareRegistry := tools.NewToolRegistry()
	require.NoError(t, bare.registerCraftDelegateTool(ctx, bareRegistry, &types.AgentConfig{}, "s1"))
	require.NotContains(t, bareRegistry.ListTools(), tools.ToolCraftDelegate)
}

// ---- R03 nit-1: the real active-run query backs ErrBusy --------------------

func TestCraftActiveRunsQueryBacksErrBusyOnRebind(t *testing.T) {
	db := openDurableRunTestDB(t)
	ctx := durableRunCtx()
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}
	active := CraftActiveRunsQuery(db)

	// No runs: not active.
	busy, err := active(ctx, scope)
	require.NoError(t, err)
	require.False(t, busy)

	runStore := repository.NewAgentRunStore(db)
	key := admitDurableRun(t, runStore, durableRunSnapshot(t))
	busy, err = active(ctx, scope)
	require.NoError(t, err)
	require.True(t, busy, "a queued run holds the session")

	fence, err := runStore.Claim(ctx, key, "worker-1", time.Minute)
	require.NoError(t, err)
	busy, err = active(ctx, scope)
	require.NoError(t, err)
	require.True(t, busy, "a running run holds the session")

	// The production query is wired into the workspace resolver: a sandbox
	// rebind while the run is live must answer craft.ErrBusy for real.
	craftStore := repository.NewCraftStore(db)
	_, err = craftStore.PutWorkspace(ctx, craft.Workspace{
		Scope: scope, SandboxID: "old-sbx", Generation: "g1",
		OpenCodeSessionID: "oc-old", RuntimeDigest: "digest-craft",
	}, 0)
	require.NoError(t, err)

	bindings := sandbox.NewMemorySessionSandboxBindingStore()
	bkey := sandbox.SessionSandboxKey{TenantID: scope.TenantID, SessionID: scope.SessionID}
	_, err = bindings.Create(ctx, bkey, sandbox.SessionSandboxBinding{
		Version: sandbox.SessionSandboxBindingVersion, Provider: sandbox.SandboxTypeDocker,
		TenantID: scope.TenantID, SessionID: scope.SessionID,
		SandboxID: "new-sbx", TemplateID: "craft-opencode-1.18.4",
		Generation: "g2", CreatedAt: time.Now().UTC(),
	})
	require.NoError(t, err)

	dials := 0
	resolver, err := NewCraftWorkspaceService(CraftWorkspaceConfig{
		Store:      craftStore,
		Bindings:   bindings,
		ActiveRuns: active, // the production query, not a stub
		Dial: func(context.Context, sandbox.SessionSandboxBinding) (*opencode.Client, error) {
			dials++
			return nil, errors.New("dial must not run while the session is busy")
		},
		RuntimeDigest: "digest-craft",
	})
	require.NoError(t, err)

	_, err = resolver.Resolve(ctx, scope)
	require.ErrorIs(t, err, craft.ErrBusy, "the real active-run query must make rebind refuse")
	require.Equal(t, 0, dials)

	// Once the run reaches a terminal state the same query answers false and
	// the rebind proceeds (reaching the dialer), proving non-stub behavior.
	require.NoError(t, runStore.SetStatus(ctx, fence, "failed", "test"))
	busy, err = active(ctx, scope)
	require.NoError(t, err)
	require.False(t, busy)
	_, err = resolver.Resolve(ctx, scope)
	require.Error(t, err)
	require.NotErrorIs(t, err, craft.ErrBusy)
	require.Equal(t, 1, dials, "with no active run the rebind proceeds to provisioning")
}

func TestCraftActiveRunsQueryCountsWaitingUser(t *testing.T) {
	db := openDurableRunTestDB(t)
	ctx := durableRunCtx()
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}
	runStore := repository.NewAgentRunStore(db)
	key := admitDurableRun(t, runStore, durableRunSnapshot(t))
	fence, err := runStore.Claim(ctx, key, "worker-1", time.Minute)
	require.NoError(t, err)
	require.NoError(t, runStore.SetStatus(ctx, fence, "waiting_user", "tool_outcome_unknown"))
	busy, err := CraftActiveRunsQuery(db)(ctx, scope)
	require.NoError(t, err)
	require.True(t, busy, "waiting_user runs still own the session slot")
}

// ---- integration: the main agent delegates, checks and fixes ---------------

// craftFixtureTurn is one scripted main-model turn.
type craftFixtureTurn struct {
	content   string
	toolCalls []types.LLMToolCall
}

// scriptedCraftModel plays the main model: turn 1 delegates the draft, turn 2
// (after seeing the failed check) delegates the fix, turn 3 explains the
// result. Every call's incoming messages are recorded.
type scriptedCraftModel struct {
	mu      sync.Mutex
	turns   []craftFixtureTurn
	calls   int
	prompts [][]chat.Message
}

func (m *scriptedCraftModel) Chat(ctx context.Context, messages []chat.Message, opts *chat.ChatOptions) (*types.ChatResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	turn := craftFixtureTurn{}
	if m.calls <= len(m.turns) {
		turn = m.turns[m.calls-1]
	}
	finish := "stop"
	if len(turn.toolCalls) > 0 {
		finish = "tool_calls"
	}
	return &types.ChatResponse{Content: turn.content, ToolCalls: turn.toolCalls, FinishReason: finish}, nil
}

func (m *scriptedCraftModel) ChatStream(ctx context.Context, messages []chat.Message, opts *chat.ChatOptions) (<-chan types.StreamResponse, error) {
	m.mu.Lock()
	m.calls++
	call := m.calls
	m.prompts = append(m.prompts, append([]chat.Message(nil), messages...))
	turn := craftFixtureTurn{}
	if call <= len(m.turns) {
		turn = m.turns[call-1]
	}
	finish := "stop"
	if len(turn.toolCalls) > 0 {
		finish = "tool_calls"
	}
	out := make(chan types.StreamResponse, 1)
	out <- types.StreamResponse{
		Content: turn.content, ToolCalls: turn.toolCalls,
		FinishReason: finish, Done: true,
	}
	close(out)
	m.mu.Unlock()
	return out, nil
}

func (m *scriptedCraftModel) GetModelName() string { return "craft-fixture" }
func (m *scriptedCraftModel) GetModelID() string   { return "craft-fixture-1" }

func (m *scriptedCraftModel) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func (m *scriptedCraftModel) prompt(call int) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if call > len(m.prompts) {
		return ""
	}
	var b strings.Builder
	for _, msg := range m.prompts[call-1] {
		fmt.Fprintf(&b, "%s:%s\n", msg.Role, msg.Content)
		for _, call := range msg.ToolCalls {
			fmt.Fprintf(&b, "toolcall:%s:%s\n", call.Function.Name, call.Function.Arguments)
		}
	}
	return b.String()
}

// scriptedSubExecutor plays the sub-executor for the integration run: the
// first delegation fails its build check, the second passes.
type scriptedSubExecutor struct {
	mu       sync.Mutex
	store    craft.Store
	executes int
	goals    []string
}

func (e *scriptedSubExecutor) Execute(ctx context.Context, task craft.Task) (craft.Result, error) {
	e.mu.Lock()
	e.executes++
	n := e.executes
	e.mu.Unlock()
	var result craft.Result
	if n == 1 {
		result = craft.Result{
			TaskID: task.ID, Status: "failed", Summary: "draft produced but the build check failed",
			Checks: []craft.Check{{Name: "build", Status: "failed", Detail: "syntax error in index.html"}},
			Files:  []craft.File{{Path: "index.html", Ref: "resource://craft/draft", SHA256: strings.Repeat("1", 64), MIME: "text/html", Bytes: 128}},
		}
	} else {
		result = craft.Result{
			TaskID: task.ID, Status: "succeeded", Summary: "fixed and verified",
			Checks: []craft.Check{{Name: "build", Status: "passed"}},
			Files:  []craft.File{{Path: "index.html", Ref: "resource://craft/final", SHA256: strings.Repeat("2", 64), MIME: "text/html", Bytes: 256}},
		}
	}
	if err := e.store.SaveResult(ctx, task.Fence, result); err != nil {
		return craft.Result{}, err
	}
	return result, nil
}

func (e *scriptedSubExecutor) Observe(context.Context, craft.Task) (craft.Observation, error) {
	return craft.Observation{}, errors.New("observe not expected in this flow")
}

func (e *scriptedSubExecutor) Abort(context.Context, craft.Task) error { return nil }

func (e *scriptedSubExecutor) executeCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.executes
}

func craftFixtureTurns() []craftFixtureTurn {
	return []craftFixtureTurn{
		{toolCalls: []types.LLMToolCall{{
			ID: "call-draft", Type: "function",
			Function: types.FunctionCall{Name: "craft_delegate", Arguments: `{"goal":"初稿 landing page"}`},
		}}},
		{toolCalls: []types.LLMToolCall{{
			ID: "call-fix", Type: "function",
			Function: types.FunctionCall{Name: "craft_delegate", Arguments: `{"goal":"修复 build 检查失败"}`},
		}}},
		{content: "两轮委派完成：修复后 build 检查通过，站点已就绪。"},
	}
}

func TestExecuteDurableRunCraftDelegationLoop(t *testing.T) {
	db := openDurableRunTestDB(t)
	ctx := durableRunCtx()

	// The session is a Craft session: trpc engine plus a bound workspace.
	craftStore := repository.NewCraftStore(db)
	seedCraftWorkspace(t, db, "s1")

	sub := &scriptedSubExecutor{store: craftStore}
	delegation, err := NewCraftDelegation(craftStore, sub)
	require.NoError(t, err)

	model := &scriptedCraftModel{turns: craftFixtureTurns()}
	manager := mcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	svc := &sessionService{
		cfg:           nil,
		messageRepo:   &durableRunMessageRepo{},
		modelService:  &durableRunModelService{chat: model},
		agentService:  &agentService{db: db, mcpManager: manager, craft: delegation},
		memoryService: nil,
	}

	runStore := repository.NewAgentRunStore(db)
	prev := RegisteredAgentRunService()
	RegisterAgentRunService(NewAgentRunService(runStore))
	t.Cleanup(func() { RegisterAgentRunService(prev) })

	key := admitDurableRun(t, runStore, durableRunSnapshot(t))
	fence, err := runStore.Claim(ctx, key, "worker-1", time.Minute)
	require.NoError(t, err)

	require.NoError(t, svc.ExecuteDurableRun(ctx, fence))

	run, err := runStore.Get(ctx, key)
	require.NoError(t, err)
	require.Equal(t, "succeeded", run.Status)

	// Main model calls: draft delegation -> fix delegation -> explanation.
	require.GreaterOrEqual(t, model.callCount(), 3)
	require.Equal(t, 3, model.callCount(), "exactly the scripted three turns")
	// Sub delegations: exactly two.
	require.Equal(t, 2, sub.executeCount(), "two sub-executions, one per delegation")

	// Turn 2 saw the failed check; turn 3 saw the successful fix.
	require.Contains(t, model.prompt(2), "status: failed", "model must inspect the failed check")
	require.Contains(t, model.prompt(2), "build: failed")
	require.Contains(t, model.prompt(3), "status: succeeded")

	// The finalized assistant message carries only the last turn's answer.
	var content string
	require.NoError(t, db.Raw("SELECT content FROM messages WHERE id = ?", "assistant-"+t.Name()).Scan(&content).Error)
	require.Equal(t, craftFixtureTurns()[2].content, content)

	// Durable delegation journal: two rows, one failed and one succeeded.
	var total, failed, succeeded int
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM craft_delegations").Scan(&total).Error)
	require.Equal(t, 2, total)
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM craft_delegations WHERE status = 'failed'").Scan(&failed).Error)
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM craft_delegations WHERE status = 'succeeded'").Scan(&succeeded).Error)
	require.Equal(t, 1, failed)
	require.Equal(t, 1, succeeded)

	// Repeated recovery of the same tool call reuses the stored result and
	// never re-executes the delegation: re-delegate the persisted task of
	// the fix call through the same service the tool uses.
	var taskJSON string
	require.NoError(t, db.Raw("SELECT task_json FROM craft_delegations WHERE status = 'succeeded'").Scan(&taskJSON).Error)
	var fixTask craft.Task
	require.NoError(t, json.Unmarshal([]byte(taskJSON), &fixTask))
	result, err := NewCraftDelegateService(craftStore, sub).Delegate(context.Background(), fixTask)
	require.NoError(t, err)
	require.Equal(t, "succeeded", result.Status)
	require.Equal(t, 2, sub.executeCount(), "recovery must not re-execute a saved tool call's delegation")
}

// ---- durable journal replay does not re-execute a saved craft call --------

func TestCraftToolCallJournalReplayReusesResult(t *testing.T) {
	db := openDurableRunTestDB(t)
	ctx := durableRunCtx()

	craftStore := repository.NewCraftStore(db)
	seedCraftWorkspace(t, db, "s1")
	sub := &scriptedSubExecutor{store: craftStore}
	delegation, err := NewCraftDelegation(craftStore, sub)
	require.NoError(t, err)

	manager := mcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	agentSvc := &agentService{db: db, mcpManager: manager, craft: delegation}
	model := &scriptedCraftModel{turns: []craftFixtureTurn{{content: "done"}}}
	caps, err := agentSvc.prepareAgentCapabilities(ctx, &types.AgentConfig{
		AllowedTools: []string{tools.ToolThinking}, MultiTurnEnabled: true,
	}, model, nil, event.NewEventBus(), "s1", "assistant-journal")
	require.NoError(t, err)
	require.Contains(t, caps.Tools.ListTools(), tools.ToolCraftDelegate)

	runStore := repository.NewAgentRunStore(db)
	key := agentruntime.RunKey{TenantID: 1, RunID: "run-journal-" + t.Name()}
	user, err := json.Marshal(map[string]any{"role": "user", "content": "hello"})
	require.NoError(t, err)
	assistant, err := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	require.NoError(t, err)
	_, err = runStore.Admit(ctx, agentruntime.Admission{
		Key: key, SessionID: "s1", UserID: "u1", RequestID: "req-journal", AssistantMessageID: "assistant-journal",
		RequestHash: "hash-journal", Snapshot: durableRunSnapshot(t), UserMessage: user, AssistantMessage: assistant,
		Deadline: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	fence, err := runStore.Claim(ctx, key, "worker-1", time.Minute)
	require.NoError(t, err)

	executorTools := agentruntime.NewToolExecutor(runStore, runStore, func(tctx context.Context, name string, args json.RawMessage) (*types.ToolResult, error) {
		return caps.Tools.ExecuteTool(types.WithSessionID(tctx, "s1"), name, args)
	})
	args := json.RawMessage(`{"goal":"journal replay"}`)
	sum := sha256.Sum256(args)
	plan := agentruntime.ToolPlan{
		Version: 1, CallID: "call-journal", Name: tools.ToolCraftDelegate, Identity: tools.ToolCraftDelegate,
		ArgsHash: hex.EncodeToString(sum[:]), Args: args,
	}
	first, err := executorTools.Execute(ctx, fence, plan)
	require.NoError(t, err)
	require.Equal(t, 1, sub.executeCount())

	// A recovered worker replays the exact same plan: the journal reuses the
	// committed result without dispatching the tool again.
	second, err := executorTools.Execute(ctx, fence, plan)
	require.NoError(t, err)
	require.Equal(t, 1, sub.executeCount(), "journal replay must not re-execute the saved tool call")
	require.Equal(t, first.Result.Output, second.Result.Output)
	require.Equal(t, first.Result.Success, second.Result.Success)
}
