package service

// T17 (#136) — Persist stop intent and retain repairable drafts. The journey
// runs at the highest available control API seam with the REAL durable
// components: the real SQLite migrations, the real AgentRunService (Cancel
// walks the real run-row CAS), the real draft-head and version stores, and
// a real persisted workspace. Faked are only the executor (an OpenCode
// runtime we cannot drive in-process) and the stop-intent store itself (the
// T20-owned production row lands with the central wiring).
//
// The journey pins, in order:
//  1. the stop intent persists BEFORE the executor abort is requested, and
//     the accepted answer leaves the Run nonterminal ("stopping") with the
//     writer fence retained;
//  2. the page-refresh poll (a second service instance over the same
//     durable state) reconstructs the phase from the persisted intent;
//  3. an abort whose outcome cannot be observed stays unknown — distinct
//     from a confirmation — and the Run still does not terminalize;
//  4. only the authoritative confirmation writes the terminal canceled
//     status (the real DB CAS), releases nothing prematurely, and repeated
//     stops are idempotent;
//  5. the Workspace draft retains its source Run identity and the prior
//     successful version keeps the default seat — a stopped run promotes
//     nothing.
import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
)

// t17StopIntentStore is the in-memory stand-in for the T20-owned durable
// stop-intent rows: idempotent by Run identity, scope-guarded, and a
// confirmed stop never replays downwards.
type t17StopIntentStore struct {
	mu      sync.Mutex
	intents map[string]craft.StopIntent
	puts    int
	seq     *controlSequence
}

func newT17StopIntentStore() *t17StopIntentStore {
	return &t17StopIntentStore{intents: map[string]craft.StopIntent{}}
}

func (s *t17StopIntentStore) PutStopIntent(_ context.Context, scope craft.Scope, in craft.StopIntent) (craft.StopIntent, error) {
	if err := in.Validate(); err != nil {
		return craft.StopIntent{}, err
	}
	if scope.TenantID == 0 || scope.SessionID == "" || in.RunID == "" {
		return craft.StopIntent{}, fmt.Errorf("%w: stop intent requires scope and run", craft.ErrInvalidInput)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.puts++
	if stored, ok := s.intents[in.RunID]; ok && stored.Status == craft.StopConfirmed {
		return stored, nil // a confirmed stop is one-directional: a replay never downgrades
	}
	s.intents[in.RunID] = in
	if s.seq != nil {
		s.seq.record("stop_intent:" + in.RunID)
	}
	return in, nil
}

func (s *t17StopIntentStore) GetStopIntent(_ context.Context, scope craft.Scope, runID string) (craft.StopIntent, error) {
	if scope.TenantID == 0 || runID == "" {
		return craft.StopIntent{}, fmt.Errorf("%w: stop intent %s", craft.ErrInvalidInput, runID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.intents[runID]
	if !ok {
		return craft.StopIntent{}, fmt.Errorf("%w: stop intent %s", craft.ErrNotFound, runID)
	}
	return stored, nil
}

// TestCraftT17Journey is the T17 acceptance journey (see the file header).
func TestCraftT17Journey(t *testing.T) {
	db := openCraftSessionDB(t)
	ctx := f17Context("u1")
	const taskID = "s-t17" // Task ID == Session ID
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: taskID}
	require.NoError(t, db.Exec(
		`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES (?, 1, 't17', 'u1', 'trpc')`,
		taskID).Error)

	// The real durable components: delegation task/result bookkeeping, the
	// draft-head store, the version store and the run controller.
	delegations := newFakeDelegationStore()
	drafts := repository.NewCraftDraftHeadStore(db)
	versions := repository.NewCraftVersionStore(db)
	runs := NewAgentRunService(repository.NewAgentRunStore(db))

	craftStore, ok := repository.NewCraftStore(db).(*repository.CraftStore)
	require.True(t, ok, "the concrete craft store assembles from the migrated database")
	workspace, err := craftStore.PutWorkspace(ctx, craft.Workspace{
		Scope: scope, SandboxID: "sbx-t17", Generation: "0",
		OpenCodeSessionID: "oc-t17", RuntimeDigest: pinnedCraftRuntimeDigest,
	}, 0)
	require.NoError(t, err)

	admit := func(runID string) {
		t.Helper()
		_, err := runs.Submit(ctx, agentruntime.Admission{
			Key:                agentruntime.RunKey{TenantID: 1, RunID: runID},
			SessionID:          taskID,
			UserID:             "u1",
			RequestID:          "craft-" + runID,
			AssistantMessageID: "craft-a-" + runID,
			RequestHash:        "craft-h-" + runID,
			Snapshot:           json.RawMessage(`{"version":1,"craft":true}`),
			UserMessage:        json.RawMessage(`{"role":"user","content":"t17"}`),
			AssistantMessage:   json.RawMessage(`{"role":"assistant","content":""}`),
			Deadline:           time.Now().Add(time.Hour),
		})
		require.NoError(t, err)
	}
	terminalize := func(runID string) {
		t.Helper()
		require.NoError(t, db.Exec(
			`UPDATE agent_runs SET status = 'succeeded' WHERE tenant_id = 1 AND run_id = ?`, runID).Error)
		require.NoError(t, db.Exec(
			`UPDATE sessions SET active_agent_run_id = NULL WHERE tenant_id = 1 AND id = ?`, taskID).Error)
	}
	runStatus := func(runID string) string {
		t.Helper()
		var status string
		require.NoError(t, db.Raw(
			`SELECT status FROM agent_runs WHERE tenant_id = 1 AND run_id = ?`, runID).Scan(&status).Error)
		return status
	}

	// ------------------------------------------------------------------
	// The prior success: run-1 finished, sealed its draft head (source Run
	// identity run-1) and published the four-check V1 that holds the default
	// preview seat.
	// ------------------------------------------------------------------
	admit("run-1")
	terminalize("run-1") // the prior success is a confirmed terminal run before it seals a draft
	v1Files := []craft.File{{
		Path: "index.html", Ref: "obj-t17-v1",
		SHA256: strings.Repeat("ab", 32),
		MIME:   "text/html", Bytes: 128,
	}}
	head1, err := drafts.Advance(ctx, scope, workspace.ID, 0, "run-1", v1Files)
	require.NoError(t, err)
	v1, err := versions.Publish(ctx, scope, craft.Version{
		WorkspaceID: workspace.ID, RunID: "run-1", Kind: craft.KindWeb, Files: v1Files,
		Checks: []craft.Check{
			{Name: craft.CheckBuild, Status: craft.CheckPassed},
			{Name: craft.CheckEntry, Status: craft.CheckPassed},
			{Name: craft.CheckPreviewReachable, Status: craft.CheckPassed},
			{Name: craft.CheckPageLoad, Status: craft.CheckPassed},
		},
	})
	require.NoError(t, err)

	// The live Run the member stops: run-2 with one in-flight delegation.
	admit("run-2")
	t17Task := craft.Task{
		ID: "dlg-t17", ToolCallID: "call-t17", Prompt: "build it", PromptMessageID: "msg-t17",
		RequestHash: "hash-t17", WorkspaceID: workspace.ID,
		Scope: scope,
		Fence: agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 1, RunID: "run-2"}, Owner: "worker-2", Epoch: 1},
	}
	_, err = delegations.PrepareTask(ctx, t17Task)
	require.NoError(t, err)

	executor := &controlExecutor{}
	intents := newT17StopIntentStore()
	seq := &controlSequence{}
	executor.seq = seq
	intents.seq = seq
	svc := NewCraftControlService(runs, delegations, executor, newFakeInteractionStore(), nil)
	svc.SetStopIntents(intents)
	svc.SetTaskAccess(f17TaskAccess{roles: map[string]craft.TaskRole{"u1": craft.TaskRoleOwner}})
	stopReq := CraftStopRequest{Scope: scope, RunKey: agentruntime.RunKey{TenantID: 1, RunID: "run-2"}, TaskID: "dlg-t17"}

	// stopFacts assembles the authoritative lease facts of the stop journey:
	// pending counts one in-flight delegation until the run terminalizes.
	stopFacts := func(status string, pending int64) craft.WriterRunFacts {
		return craft.WriterRunFacts{Observed: true, Status: status,
			PendingToolWriters: 0, PendingDelegations: pending}
	}

	// ------------------------------------------------------------------
	// Phase 1 — the stop intent persists BEFORE the abort; the accepted
	// answer is nonterminal and the writer fence stays.
	// ------------------------------------------------------------------
	executor.onObserve = func(craft.Task) (craft.Observation, error) {
		// The abort was delivered but the executor is still busy.
		return craft.Observation{SessionID: "oc-t17", Aborted: false, Idle: false}, nil
	}
	resp, err := svc.Stop(ctx, stopReq)
	require.NoError(t, err)
	require.Equal(t, "stopping", resp.Phase, "an accepted stop with the executor still active answers stopping — a real phase, never folded")
	require.Equal(t, craft.StopRequested, resp.Outcome.Status, "the accepted stop projects the requested outcome (T00 DTO)")
	require.Equal(t, "run-2", resp.Outcome.RunID)

	stored, err := intents.GetStopIntent(ctx, scope, "run-2")
	require.NoError(t, err)
	require.Equal(t, craft.StopRequested, stored.Status, "the stop intent persisted durably before anything else")

	require.Equal(t, []string{"stop_intent:run-2", "abort"}, seq.steps(),
		"the durable stop intent must land BEFORE the executor abort is requested")

	require.False(t, craft.WriterRunTerminal(runStatus("run-2")),
		"the accepted HTTP stop must NOT terminalize the run row (status %q) — stopping is nonterminal", runStatus("run-2"))
	require.False(t, craft.WriterLeaseReleasable(stopFacts(runStatus("run-2"), 1)),
		"a stopping run must not release the workspace writer ownership")
	require.False(t, craft.WriterLeaseTakeover(&craft.WriterLease{WorkspaceID: workspace.ID, RunID: "run-2"}, stopFacts(runStatus("run-2"), 1)),
		"a stopping run must not admit a conflicting writer takeover")

	// ------------------------------------------------------------------
	// Phase 2 — the refresh poll: a second service instance (a new page
	// session over the same durable state) reconstructs the phase from the
	// persisted intent, not from an in-memory cache.
	// ------------------------------------------------------------------
	refreshedExecutor := &controlExecutor{}
	refreshedExecutor.onObserve = executor.onObserve
	refreshed := NewCraftControlService(runs, delegations, refreshedExecutor, newFakeInteractionStore(), nil)
	refreshed.SetStopIntents(intents)
	refreshed.SetTaskAccess(f17TaskAccess{roles: map[string]craft.TaskRole{"u1": craft.TaskRoleOwner}})
	polled, err := refreshed.DelegationStatus(ctx, scope, stopReq.RunKey, "dlg-t17")
	require.NoError(t, err)
	require.Equal(t, "stopping", polled.Phase, "after a page refresh the stop state survives: still stopping")
	require.Equal(t, craft.StopRequested, polled.Outcome.Status)

	// ------------------------------------------------------------------
	// Phase 3 — an unobservable abort outcome is unknown, distinct from a
	// confirmation, and still never terminalizes the run.
	// ------------------------------------------------------------------
	executor.onObserve = func(craft.Task) (craft.Observation, error) {
		return craft.Observation{}, fmt.Errorf("remote sandbox unreachable")
	}
	unknown, err := svc.Stop(ctx, stopReq)
	require.NoError(t, err)
	require.Equal(t, "stopping", unknown.Phase, "an unverified abort keeps the honest nonterminal phase")
	require.Equal(t, craft.StopUnknown, unknown.Outcome.Status,
		"an abort whose outcome cannot be observed is unknown — NOT requested and NOT confirmed")
	require.NotEqual(t, craft.StopConfirmed, unknown.Outcome.Status)

	stored, err = intents.GetStopIntent(ctx, scope, "run-2")
	require.NoError(t, err)
	require.Equal(t, craft.StopUnknown, stored.Status, "the unknown outcome persisted distinctly")
	require.False(t, craft.WriterRunTerminal(runStatus("run-2")),
		"an unknown abort outcome must not terminalize the run row either (status %q)", runStatus("run-2"))
	require.False(t, craft.StopIntentMayWriteRunTerminal(stored.Status),
		"unknown is never permission for the terminal write")
	require.False(t, craft.WriterLeaseReleasable(stopFacts(runStatus("run-2"), 1)),
		"an unknown stop outcome retains the writer fence")

	// ------------------------------------------------------------------
	// Phase 4 — the authoritative confirmation is what finally writes the
	// terminal canceled status (the real DB CAS), and the confirmed stop is
	// the verified release basis for the fence.
	// ------------------------------------------------------------------
	executor.onObserve = func(craft.Task) (craft.Observation, error) {
		return craft.Observation{SessionID: "oc-t17", Aborted: true, Idle: true}, nil
	}
	confirmed, err := svc.Stop(ctx, stopReq)
	require.NoError(t, err)
	require.Equal(t, "canceled", confirmed.Phase)
	require.Equal(t, craft.StopConfirmed, confirmed.Outcome.Status)
	require.Equal(t, "canceled", runStatus("run-2"),
		"the terminal canceled status is written only AFTER the authoritative confirmation")

	stored, err = intents.GetStopIntent(ctx, scope, "run-2")
	require.NoError(t, err)
	require.Equal(t, craft.StopConfirmed, stored.Status)
	require.True(t, craft.StopIntentMayWriteRunTerminal(stored.Status),
		"the confirmed stop is the one durable stop state that permits the terminal write")
	require.True(t, craft.WriterLeaseReleasable(stopFacts("canceled", 0)),
		"a confirmed cancellation with zero pending writers is a verified lease-release basis")

	// ------------------------------------------------------------------
	// Phase 5 — repeated stops are idempotent and the state survives the
	// refresh: no second abort, no status flip, the same durable answer.
	// ------------------------------------------------------------------
	abortsBefore := executor.AbortCount()
	repeated, err := svc.Stop(ctx, stopReq)
	require.NoError(t, err)
	require.Equal(t, "canceled", repeated.Phase, "a repeated stop replays the confirmed answer")
	require.Equal(t, craft.StopConfirmed, repeated.Outcome.Status)
	require.Equal(t, abortsBefore, executor.AbortCount(), "a repeated stop after confirmation must not abort again")
	require.Equal(t, "canceled", runStatus("run-2"), "the idempotent replay leaves the run row untouched")

	polledAgain, err := refreshed.DelegationStatus(ctx, scope, stopReq.RunKey, "dlg-t17")
	require.NoError(t, err)
	require.Equal(t, "canceled", polledAgain.Phase, "after the refresh the poll reports the persisted confirmation")
	require.Equal(t, craft.StopConfirmed, polledAgain.Outcome.Status)

	// ------------------------------------------------------------------
	// Phase 6 — the draft retains its source Run identity and the prior
	// successful version keeps the default seat: the stopped run promoted
	// nothing.
	// ------------------------------------------------------------------
	head, err := drafts.Read(ctx, scope, workspace.ID)
	require.NoError(t, err)
	require.Equal(t, head1.Revision, head.Revision, "the stop never moved the workspace draft head")
	require.Equal(t, "run-1", head.SourceRunID,
		"the repairable draft retains the source Run identity of the last successful write")
	require.Equal(t, head1.ManifestDigest, head.ManifestDigest)
	require.Equal(t, craft.DraftHeadSelected, head.State)

	_, err = delegations.GetResult(ctx, scope, "dlg-t17")
	require.ErrorIs(t, err, craft.ErrNotFound,
		"the stopped delegation left no success result — nothing to promote")

	published, err := versions.List(ctx, scope)
	require.NoError(t, err)
	require.Len(t, published, 1, "no second version appeared: a stopped run promotes nothing")
	defaultVersion, okSel := craft.SelectDefaultVersion(published)
	require.True(t, okSel)
	require.Equal(t, v1.ID, defaultVersion.ID,
		"the prior successful V1 keeps the default preview seat after the stop")
}
