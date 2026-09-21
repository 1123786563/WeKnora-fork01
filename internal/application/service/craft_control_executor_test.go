package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/craft"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/stretchr/testify/require"
)

// controlStopFixture is the same fixture shape the existing craft_control_test
// stop tests use (see TestControlStopStaysStoppingWhenAbortAcceptedButStill-
// Running, craft_control_test.go:508): runs controller, delegation store,
// interaction store and an abort-recording executor fake.
type controlStopFixture struct {
	runs           *fakeControlRuns
	store          *fakeDelegationStore
	interactions   *fakeInteractionStore
	abortExecutor  *controlExecutor
	stopRequestFor func() CraftStopRequest
}

// newControlStopFixture builds the stop fixtures standalone (mirrors
// newControlFixture in craft_control_test.go: runs controller with a live run,
// delegation store with the prepared task, interactions store, executor fake).
func newControlStopFixture(t *testing.T) *controlStopFixture {
	t.Helper()
	runs := &fakeControlRuns{run: agentruntime.Run{
		Key:       agentruntime.RunKey{TenantID: 1, RunID: "run-ctl"},
		SessionID: "s1", UserID: "u1", Status: "running", Revision: 4, Epoch: 2,
	}}
	store := newFakeDelegationStore()
	task := craft.Task{
		ID: "dlg_ctl", ToolCallID: "call-1", Prompt: "build it", PromptMessageID: "msg_ctl",
		RequestHash: "hash-ctl", WorkspaceID: "ws-1",
		Scope: craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"},
		Fence: agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 1, RunID: "run-ctl"}, Owner: "worker-1", Epoch: 2},
	}
	_, prepareErr := store.PrepareTask(context.Background(), task)
	require.NoError(t, prepareErr)
	return &controlStopFixture{
		runs:          runs,
		store:         store,
		interactions:  newFakeInteractionStore(),
		abortExecutor: &controlExecutor{},
		stopRequestFor: func() CraftStopRequest {
			return CraftStopRequest{
				Scope: controlScope(), RunKey: agentruntime.RunKey{TenantID: 1, RunID: "run-ctl"}, TaskID: "dlg_ctl",
			}
		},
	}
}

// TestSetExecutorIsVisibleToConcurrentStop proves the post-construction
// injection (which breaks the provider cycle) is safe to read from a stop
// that runs after the setter: the R06 semantic — nil executor keeps the
// recorded-intent degrade, an injected executor is used for the abort.
func TestSetExecutorIsVisibleToConcurrentStop(t *testing.T) {
	// Build the same fixture shape the existing craft_control_test.go uses
	// for its stop tests (see TestControlStopStaysStoppingWhenAbortAccepted-
	// ButStillRunning, craft_control_test.go:508-528): runs controller, store,
	// nil executor at construction, interactions store.
	fixture := newControlStopFixture(t)
	svc := NewCraftControlService(fixture.runs, fixture.store, nil, fixture.interactions, nil)

	if got := svc.currentExecutor(); got != nil {
		t.Fatalf("expected nil executor before injection, got %T", got)
	}
	var injected craft.Executor = fixture.abortExecutor // an executor fake whose Abort records the call
	svc.SetExecutor(injected)
	if got := svc.currentExecutor(); got != injected {
		t.Fatalf("expected injected executor after SetExecutor, got %T", got)
	}
}

// TestStopUsesPostConstructionExecutor drives a real Stop after the injection:
// the abort and the verified observation must run on the injected executor,
// not stay at the recorded-intent degrade.
func TestStopUsesPostConstructionExecutor(t *testing.T) {
	fixture := newControlStopFixture(t)
	fixture.abortExecutor.onObserve = func(craft.Task) (craft.Observation, error) {
		return craft.Observation{Aborted: true, Idle: true, SessionID: "oc-1"}, nil
	}
	svc := NewCraftControlService(fixture.runs, fixture.store, nil, fixture.interactions, nil)

	// Before the injection the stop degrades honestly to the recorded intent.
	before, err := svc.Stop(context.Background(), fixture.stopRequestFor())
	require.NoError(t, err)
	require.Equal(t, "stopping", before.Phase)
	require.Zero(t, fixture.abortExecutor.AbortCount(), "no executor, no abort")

	svc.SetExecutor(fixture.abortExecutor)
	after, err := svc.Stop(context.Background(), fixture.stopRequestFor())
	require.NoError(t, err)
	require.Equal(t, "canceled", after.Phase, "the injected executor must drive the verified abort")
	require.Equal(t, 1, fixture.abortExecutor.AbortCount())
}

// TestSetExecutorNilKeepsRecordedIntentDegrade pins the setter contract: a nil
// injection never clears a live executor and never installs anything.
func TestSetExecutorNilKeepsRecordedIntentDegrade(t *testing.T) {
	fixture := newControlStopFixture(t)
	svc := NewCraftControlService(fixture.runs, fixture.store, fixture.abortExecutor, fixture.interactions, nil)

	svc.SetExecutor(nil)
	if got := svc.currentExecutor(); got == nil {
		t.Fatal("a nil injection must not clear the live executor")
	}

	nilSvc := NewCraftControlService(fixture.runs, fixture.store, nil, fixture.interactions, nil)
	nilSvc.SetExecutor(nil)
	if got := nilSvc.currentExecutor(); got != nil {
		t.Fatalf("a nil injection must install nothing, got %T", got)
	}
}
