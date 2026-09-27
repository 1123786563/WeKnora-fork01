package service

// T17 (#136) round-3 OCR regressions: the crash-window repair, the
// unknown-downgrade guard, the poll's stop-journey evidence gate and the
// superseded bypass. Each test drives the REAL run store plus the fake
// delegation store (the journey-test pattern) and the memory stop-intent
// store.
import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/opencode"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
)

// seedControlStopJourney submits a running run plus its delegation task and
// returns the assembled control service with the memory stop-intent store.
func seedControlStopJourney(t *testing.T, sessionID, runID string, exec craft.Executor) (*CraftControlService, *memoryStopIntentStore, agentruntime.RunKey, craft.Scope) {
	t.Helper()
	db := openCraftSessionDB(t)
	ctx := context.Background()
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: sessionID}
	require.NoError(t, db.Exec(
		`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES (?, 1, 't17r3', 'u1', 'trpc')`,
		sessionID).Error)

	delegations := newFakeDelegationStore()
	runs := NewAgentRunService(repository.NewAgentRunStore(db))
	_, err := runs.Submit(ctx, agentruntime.Admission{
		Key:       agentruntime.RunKey{TenantID: 1, RunID: runID},
		SessionID: sessionID, UserID: "u1",
		RequestID: "r-" + runID, AssistantMessageID: "a-" + runID,
		RequestHash:      "h-" + runID,
		Snapshot:         json.RawMessage(`{"version":1,"craft":true}`),
		UserMessage:      json.RawMessage(`{"role":"user","content":"r3"}`),
		AssistantMessage: json.RawMessage(`{"role":"assistant","content":""}`),
		Deadline:         time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	_, err = delegations.PutWorkspace(ctx, craft.Workspace{
		Scope: scope, SandboxID: "sbx-r3", Generation: "0",
		OpenCodeSessionID: "oc-r3", RuntimeDigest: "digest-r3",
	}, 0)
	require.NoError(t, err)
	ws, err := delegations.GetWorkspace(ctx, scope)
	require.NoError(t, err)
	_, err = delegations.PrepareTask(ctx, craft.Task{
		ID: "dlg-r3", ToolCallID: "call-r3", Prompt: "stop me", PromptMessageID: "msg_r3",
		RequestHash: "hash-r3", WorkspaceID: ws.ID, Scope: scope,
		Fence: agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 1, RunID: runID}, Owner: "worker-r3", Epoch: 1},
	})
	require.NoError(t, err)

	intents := &memoryStopIntentStore{}
	svc := NewCraftControlService(runs, delegations, exec, nil, nil)
	svc.SetStopIntents(intents)
	return svc, intents, agentruntime.RunKey{TenantID: 1, RunID: runID}, scope
}

// TestControlStopCrashWindowBackfillsConfirmedMarker pins the round-3 high
// fix: an earlier stop crashed between the terminal CAS and the confirmed
// marker (run row canceled, intent still requested). The retry must BACK-FILL
// the confirmed marker via the Cancel-conflict replay — otherwise every later
// poll projects Outcome=requested next to Phase=canceled forever.
func TestControlStopCrashWindowBackfillsConfirmedMarker(t *testing.T) {
	ctx := context.Background()
	exec := &controlExecutor{}
	exec.onObserve = func(craft.Task) (craft.Observation, error) {
		return craft.Observation{Aborted: true, Idle: true}, nil
	}
	svc, intents, key, scope := seedControlStopJourney(t, "s-t17r3a", "run-t17r3a", exec)

	// The crash window: the terminal CAS landed, the confirmed marker did not.
	require.NoError(t, svc.runs.Cancel(ctx, key))
	_, err := intents.PutStopIntent(ctx, scope, craft.StopIntent{RunID: key.RunID, Status: craft.StopRequested})
	require.NoError(t, err)

	status, err := svc.Stop(context.Background(), CraftStopRequest{Scope: scope, RunKey: key, TaskID: "dlg-r3"})
	require.NoError(t, err)
	require.Equal(t, "canceled", status.Phase)
	require.Equal(t, craft.StopConfirmed, status.Outcome.Status)
	require.Equal(t, craft.StopConfirmed, intents.rows[key.RunID].Status,
		"the retry must repair the missing confirmed marker, not replay requested forever")
}

// TestControlStopUnknownNeverDowngradesTerminalCanceledRun pins the round-3
// guard: on the SAME crash-window row (run canceled, intent requested), a
// retry whose Observe FAILS (session already gone) must NOT persist unknown —
// it back-fills confirmed instead of permanently downgrading the journey.
func TestControlStopUnknownNeverDowngradesTerminalCanceledRun(t *testing.T) {
	ctx := context.Background()
	exec := &controlExecutor{}
	exec.onObserve = func(craft.Task) (craft.Observation, error) {
		return craft.Observation{}, context.DeadlineExceeded
	}
	svc, intents, key, scope := seedControlStopJourney(t, "s-t17r3b", "run-t17r3b", exec)

	require.NoError(t, svc.runs.Cancel(ctx, key))
	_, err := intents.PutStopIntent(ctx, scope, craft.StopIntent{RunID: key.RunID, Status: craft.StopRequested})
	require.NoError(t, err)

	status, err := svc.Stop(context.Background(), CraftStopRequest{Scope: scope, RunKey: key, TaskID: "dlg-r3"})
	require.NoError(t, err)
	require.Equal(t, "canceled", status.Phase)
	require.Equal(t, craft.StopConfirmed, status.Outcome.Status)
	require.Equal(t, craft.StopConfirmed, intents.rows[key.RunID].Status,
		"a terminally canceled row must never be downgraded to unknown")
}

// TestControlStopNonStopCancelReplayDoesNotClaimStopJourney pins the round-3
// wording fix: a run canceled by a NON-stop path (session deletion, generic
// cancel) with no intent on record replays the row's terminal fact with its
// own note — never "stop already confirmed".
func TestControlStopNonStopCancelReplayDoesNotClaimStopJourney(t *testing.T) {
	ctx := context.Background()
	exec := &controlExecutor{}
	svc, _, key, scope := seedControlStopJourney(t, "s-t17r3c", "run-t17r3c", exec)

	require.NoError(t, svc.runs.Cancel(ctx, key))

	status, err := svc.Stop(context.Background(), CraftStopRequest{Scope: scope, RunKey: key, TaskID: "dlg-r3"})
	require.NoError(t, err)
	require.Equal(t, "canceled", status.Phase)
	require.Equal(t, craft.StopConfirmed, status.Outcome.Status)
	require.Contains(t, status.Note, "non-stop path",
		"the replay must not announce a stop journey that never happened")
	require.NotContains(t, status.Note, "stop already confirmed")
}

// TestControlStatusPollProjectsNoOutcomeWithoutStopJourney pins the round-3
// evidence gate: the generic status endpoint polls a delegation that never
// touched the stop surface — no intent row, nonterminal run — so the Outcome
// must be the zero value (no fabricated "已请求停止" banner once T20 serializes).
func TestControlStatusPollProjectsNoOutcomeWithoutStopJourney(t *testing.T) {
	exec := &controlExecutor{}
	exec.onObserve = func(craft.Task) (craft.Observation, error) {
		return craft.Observation{SessionID: "oc-r3", Idle: false}, nil
	}
	svc, _, key, scope := seedControlStopJourney(t, "s-t17r3d", "run-t17r3d", exec)

	status, err := svc.DelegationStatus(context.Background(), scope, key, "dlg-r3")
	require.NoError(t, err)
	require.Equal(t, "running", status.Phase)
	require.Empty(t, status.Outcome.RunID, "no stop journey on record — no outcome to project")
	require.Empty(t, status.Outcome.Status, "the zero outcome is the honest no-journey answer")
}

// TestControlStatusPollSupersededBypassesStaleIntent pins the round-3
// superseded convergence: a stale unknown intent (abort outcome was unknown)
// whose delegation then completed normally must NOT project "outcome unknown"
// next to Phase=completed — the bypass projects the overtaken fact instead.
func TestControlStatusPollSupersededBypassesStaleIntent(t *testing.T) {
	ctx := context.Background()
	exec := &controlExecutor{}
	exec.onObserve = func(craft.Task) (craft.Observation, error) {
		// The delegation finished normally while the abort outcome was
		// never determined: completed WITHOUT the abort.
		return craft.Observation{SessionID: "oc-r3", PromptMessageID: "msg_r3",
			AssistantParentID: "msg_r3", Completed: true, Idle: true, Finish: "stop"}, nil
	}
	svc, intents, key, scope := seedControlStopJourney(t, "s-t17r3e", "run-t17r3e", exec)

	_, err := intents.PutStopIntent(ctx, scope, craft.StopIntent{RunID: key.RunID, Status: craft.StopUnknown})
	require.NoError(t, err)

	status, err := svc.DelegationStatus(context.Background(), scope, key, "dlg-r3")
	require.NoError(t, err)
	require.Equal(t, "completed", status.Phase)
	require.Equal(t, craft.StopRequested, status.Outcome.Status,
		"the stale unknown intent must be bypassed: the terminal phase carries the settlement")
	require.True(t, opencode.Completed(craft.Observation{SessionID: "oc-r3", PromptMessageID: "msg_r3",
		AssistantParentID: "msg_r3", Completed: true, Idle: true, Finish: "stop"}),
		"fixture sanity: this observation is an authoritative completion")
}

// TestControlStatusPollTailReplaysTerminalRow pins the round-3 tail: without
// a result or an observable delegation, a terminally canceled run row answers
// canceled (not an empty-observation "stopping"), and a run row that never
// existed as a stop journey projects no outcome at all.
func TestControlStatusPollTailReplaysTerminalRow(t *testing.T) {
	ctx := context.Background()
	svc, _, key, scope := seedControlStopJourney(t, "s-t17r3f", "run-t17r3f", &controlExecutor{})
	require.NoError(t, svc.runs.Cancel(ctx, key))

	status, err := svc.DelegationStatus(context.Background(), scope, key, "")
	require.NoError(t, err)
	require.Equal(t, "canceled", status.Phase, "the row's terminal fact answers without an observation")
	require.Equal(t, craft.StopConfirmed, status.Outcome.Status)
}
