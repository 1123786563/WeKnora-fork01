package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/craft"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/stretchr/testify/require"
)

// ---- R06 control fakes ------------------------------------------------------

// controlSequence records cross-fake ordering so tests can prove
// "decision first, forward second" and "cancel intent first, abort second".
type controlSequence struct {
	mu   sync.Mutex
	step []string
}

func (s *controlSequence) record(step string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.step = append(s.step, step)
}

func (s *controlSequence) steps() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.step...)
}

// fakeControlRuns stands in for *AgentRunService on the control port.
type fakeControlRuns struct {
	mu       sync.Mutex
	run      agentruntime.Run
	getErr   error
	cancelEr error
	gets     int
	cancels  int
	waits    int
	lastWait agentruntime.Fence
	pendings []string
	seq      *controlSequence
}

func (r *fakeControlRuns) Get(context.Context, agentruntime.RunKey) (agentruntime.Run, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gets++
	if r.getErr != nil {
		return agentruntime.Run{}, r.getErr
	}
	return r.run, nil
}

func (r *fakeControlRuns) Cancel(_ context.Context, key agentruntime.RunKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cancels++
	if r.cancelEr != nil {
		return r.cancelEr
	}
	r.run.Key = key
	r.run.Status = "canceled"
	if r.seq != nil {
		r.seq.record("cancel")
	}
	return nil
}

func (r *fakeControlRuns) WaitForDecision(_ context.Context, fence agentruntime.Fence, pendingID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.waits++
	r.lastWait = fence
	r.pendings = append(r.pendings, pendingID)
	return nil
}

func (r *fakeControlRuns) counts() (gets, cancels, waits int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.gets, r.cancels, r.waits
}

// fakeInteractionStore reproduces the R06 interaction persistence rules:
// owner-scoped reads, idempotent registration and decision CAS on revision
// plus args hash. C02 owns the durable production store and the outbox.
type fakeInteractionStore struct {
	mu       sync.Mutex
	records  map[string]CraftInteractionRecord
	applyErr error
	applies  int
	marks    []string
	seq      *controlSequence
}

func newFakeInteractionStore() *fakeInteractionStore {
	return &fakeInteractionStore{records: map[string]CraftInteractionRecord{}}
}

func (s *fakeInteractionStore) PutInteraction(_ context.Context, in CraftInteractionRecord) (CraftInteractionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if in.Interaction.ID == "" {
		return CraftInteractionRecord{}, fmt.Errorf("%w: interaction id is required", craft.ErrInvalidInput)
	}
	if stored, ok := s.records[in.Interaction.ID]; ok {
		if stored.ArgsHash != in.ArgsHash || stored.Kind != in.Kind || stored.Prompt != in.Prompt {
			return CraftInteractionRecord{}, fmt.Errorf("%w: interaction %s re-registered with different arguments", craft.ErrConflict, in.ID)
		}
		return stored, nil
	}
	in.Revision = 1
	in.Status = "pending"
	if in.PendingID == "" {
		in.PendingID = in.ID
	}
	s.records[in.ID] = in
	return in, nil
}

func (s *fakeInteractionStore) GetInteraction(_ context.Context, scope craft.Scope, id string) (CraftInteractionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[id]
	if !ok || rec.Scope.TenantID != scope.TenantID {
		return CraftInteractionRecord{}, fmt.Errorf("%w: interaction %s", craft.ErrNotFound, id)
	}
	if rec.Scope.UserID != scope.UserID || rec.Scope.SessionID != scope.SessionID {
		return CraftInteractionRecord{}, fmt.Errorf("%w: interaction %s", craft.ErrForbidden, id)
	}
	return rec, nil
}

func (s *fakeInteractionStore) ApplyInteractionDecision(_ context.Context, req CraftDecisionRequest) (CraftInteractionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applies++
	if s.applyErr != nil {
		return CraftInteractionRecord{}, s.applyErr
	}
	rec, ok := s.records[req.InteractionID]
	if !ok {
		return CraftInteractionRecord{}, fmt.Errorf("%w: interaction %s", craft.ErrNotFound, req.InteractionID)
	}
	if rec.Scope.TenantID != req.Scope.TenantID {
		return CraftInteractionRecord{}, fmt.Errorf("%w: interaction %s", craft.ErrNotFound, req.InteractionID)
	}
	if rec.Scope.UserID != req.Scope.UserID || rec.Scope.SessionID != req.Scope.SessionID {
		return CraftInteractionRecord{}, fmt.Errorf("%w: interaction %s", craft.ErrForbidden, req.InteractionID)
	}
	if !craft.DecisionAllowed(rec.Kind, req.Action) {
		return CraftInteractionRecord{}, fmt.Errorf("%w: action %q is not a user decision for %s", craft.ErrInvalidInput, req.Action, rec.Kind)
	}
	if req.DecisionID == "" {
		return CraftInteractionRecord{}, fmt.Errorf("%w: decision id is required", craft.ErrInvalidInput)
	}
	if req.ArgsHash == "" || req.ArgsHash != rec.ArgsHash {
		return CraftInteractionRecord{}, fmt.Errorf("%w: interaction %s arguments changed since the user decided", craft.ErrConflict, req.InteractionID)
	}
	if req.ExpectedRevision != rec.Revision {
		return CraftInteractionRecord{}, fmt.Errorf("%w: interaction %s revision %d is not current %d", craft.ErrConflict, req.InteractionID, req.ExpectedRevision, rec.Revision)
	}
	if rec.Status != "pending" {
		return CraftInteractionRecord{}, fmt.Errorf("%w: interaction %s is already %s", craft.ErrConflict, req.InteractionID, rec.Status)
	}
	rec.Status = "decided"
	rec.Delivery = "pending"
	rec.DecisionID = req.DecisionID
	rec.DecidedAction = req.Action
	rec.Revision++
	s.records[req.InteractionID] = rec
	if s.seq != nil {
		s.seq.record("decision")
	}
	return rec, nil
}

func (s *fakeInteractionStore) MarkInteractionDelivery(_ context.Context, scope craft.Scope, id, delivery string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[id]
	if !ok || rec.Scope.TenantID != scope.TenantID || rec.Scope.UserID != scope.UserID {
		return fmt.Errorf("%w: interaction %s", craft.ErrNotFound, id)
	}
	rec.Delivery = delivery
	s.records[id] = rec
	s.marks = append(s.marks, id+":"+delivery)
	return nil
}

func (s *fakeInteractionStore) record(id string) CraftInteractionRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.records[id]
}

// fakeControlReplier stands in for the OpenCode client reply routes.
type fakeControlReplier struct {
	mu      sync.Mutex
	replies []string
	err     error
	seq     *controlSequence
}

func (r *fakeControlReplier) ReplyQuestion(_ context.Context, requestID string, answers [][]string) error {
	r.mu.Lock()
	r.replies = append(r.replies, fmt.Sprintf("question_reply:%s:%v", requestID, answers))
	err := r.err
	r.mu.Unlock()
	if r.seq != nil {
		r.seq.record("forward")
	}
	return err
}

func (r *fakeControlReplier) RejectQuestion(_ context.Context, requestID string) error {
	r.mu.Lock()
	r.replies = append(r.replies, "question_reject:"+requestID)
	err := r.err
	r.mu.Unlock()
	if r.seq != nil {
		r.seq.record("forward")
	}
	return err
}

func (r *fakeControlReplier) ReplyPermission(_ context.Context, sessionID, requestID, reply string) error {
	r.mu.Lock()
	r.replies = append(r.replies, fmt.Sprintf("permission:%s:%s:%s", sessionID, requestID, reply))
	err := r.err
	r.mu.Unlock()
	if r.seq != nil {
		r.seq.record("forward")
	}
	return err
}

func (r *fakeControlReplier) calls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.replies...)
}

// newControlFixture assembles the control service with recording fakes.
func newControlFixture(t *testing.T) (*CraftControlService, *fakeControlRuns, *fakeDelegationStore,
	*controlExecutor, *fakeInteractionStore, *fakeControlReplier, *controlSequence,
) {
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
	exec := &controlExecutor{}
	interactions := newFakeInteractionStore()
	reply := &fakeControlReplier{}
	seq := &controlSequence{}
	runs.seq = seq
	exec.seq = seq
	interactions.seq = seq
	reply.seq = seq
	return NewCraftControlService(runs, store, exec, interactions, reply), runs, store, exec, interactions, reply, seq
}

func controlScope() craft.Scope {
	return craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}
}

func controlFence() agentruntime.Fence {
	return agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 1, RunID: "run-ctl"}, Owner: "worker-1", Epoch: 2}
}

func controlStopRequest() CraftStopRequest {
	return CraftStopRequest{Scope: controlScope(), RunKey: agentruntime.RunKey{TenantID: 1, RunID: "run-ctl"}, TaskID: "dlg_ctl"}
}

func registerControlInteraction(t *testing.T, svc *CraftControlService, kind, argsHash, prompt, requestID string) CraftInteractionRecord {
	t.Helper()
	ctx := context.Background()
	task, err := svc.store.GetTask(ctx, controlScope(), "dlg_ctl")
	require.NoError(t, err)
	rec, err := svc.RegisterInteraction(ctx, controlFence(), task, craft.Interaction{
		Kind: kind, ArgsHash: argsHash, Prompt: prompt,
	}, "oc-1", requestID)
	require.NoError(t, err)
	return rec
}

// ---- registration and park --------------------------------------------------

func TestControlRegistersInteractionAndParksRun(t *testing.T) {
	svc, runs, _, _, interactions, _, _ := newControlFixture(t)
	ctx := context.Background()
	task, err := svc.store.GetTask(ctx, controlScope(), "dlg_ctl")
	require.NoError(t, err)

	rec, err := svc.RegisterInteraction(ctx, controlFence(), task, craft.Interaction{
		Kind:     craft.InteractionQuestion,
		ArgsHash: "args-q1",
		Prompt:   "Which database engine should I use?",
	}, "oc-1", "qst_1")
	require.NoError(t, err)

	require.NotEmpty(t, rec.ID, "the interaction id must be derived deterministically")
	require.Equal(t, rec.ID, rec.PendingID, "PendingID parks the main run and is persisted")
	require.Equal(t, "qst_1", rec.OpenCodeRequestID)
	require.Equal(t, "oc-1", rec.OpenCodeSessionID)
	require.Equal(t, "call-1", rec.ToolCallID)
	require.Equal(t, "dlg_ctl", rec.TaskID)
	require.Equal(t, int64(1), rec.Revision)
	require.Equal(t, "args-q1", rec.ArgsHash)

	_, _, waits := runs.counts()
	require.Equal(t, 1, waits, "the OpenCode wait must hang on the existing pending state")
	require.Equal(t, controlFence(), runs.lastWait)
	require.Equal(t, []string{rec.PendingID}, runs.pendings)

	again, err := svc.RegisterInteraction(ctx, controlFence(), task, craft.Interaction{
		ID: rec.ID, Kind: craft.InteractionQuestion, ArgsHash: "args-q1",
		Prompt: "Which database engine should I use?",
	}, "oc-1", "qst_1")
	require.NoError(t, err)
	require.Equal(t, rec, again)
	require.Equal(t, rec, interactions.record(rec.ID))
}

func TestControlWithoutInteractionSupportWaitsExplicitly(t *testing.T) {
	runs := &fakeControlRuns{run: agentruntime.Run{
		Key:       agentruntime.RunKey{TenantID: 1, RunID: "run-ctl"},
		SessionID: "s1", UserID: "u1", Status: "running",
	}}
	store := newFakeDelegationStore()
	exec := &fakeDelegationExecutor{}
	reply := &fakeControlReplier{}
	svc := NewCraftControlService(runs, store, exec, nil, reply)
	fence := agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 1, RunID: "run-ctl"}, Owner: "w", Epoch: 1}
	task := craft.Task{ID: "dlg_x", ToolCallID: "c", Prompt: "p", Scope: controlScope(), Fence: fence}

	_, err := svc.RegisterInteraction(context.Background(), fence, task, craft.Interaction{
		Kind: craft.InteractionPermission, ArgsHash: "a", Prompt: "run rm -rf",
	}, "oc-1", "perm_1")
	require.ErrorIs(t, err, craft.ErrUnsupported, "without UI support the request must wait explicitly, never auto-approve")

	_, err = svc.Decide(context.Background(), CraftDecisionRequest{
		Scope: controlScope(), InteractionID: "itx_x", DecisionID: "d1",
		Action: craft.DecisionApprove, ArgsHash: "a", ExpectedRevision: 1,
	})
	require.ErrorIs(t, err, craft.ErrUnsupported)

	_, _, waits := runs.counts()
	require.Zero(t, waits)
	require.Empty(t, reply.calls(), "no approval may be fabricated on the runtime")
}

// ---- decide -----------------------------------------------------------------

func controlQuestionFixture(t *testing.T) (*CraftControlService, *fakeInteractionStore, *fakeControlReplier, CraftInteractionRecord) {
	svc, _, _, _, interactions, reply, _ := newControlFixture(t)
	rec := registerControlInteraction(t, svc, craft.InteractionQuestion, "args-q1", "Which engine?", "qst_1")
	return svc, interactions, reply, rec
}

func TestControlDecideWritesDecisionBeforeForwarding(t *testing.T) {
	svc, _, _, _, _, reply, seq := newControlFixture(t)
	rec := registerControlInteraction(t, svc, craft.InteractionQuestion, "args-q1", "Which engine?", "qst_1")
	outcome, err := svc.Decide(context.Background(), CraftDecisionRequest{
		Scope: controlScope(), InteractionID: rec.ID, DecisionID: "dec-1",
		Action: craft.DecisionAnswer, ArgsHash: rec.ArgsHash, ExpectedRevision: rec.Revision,
		Answer: "postgres",
	})
	require.NoError(t, err)
	require.True(t, outcome.Decided)
	require.True(t, outcome.Delivered)
	require.Equal(t, []string{"decision", "forward"}, seq.steps(),
		"the durable decision must be written before any OpenCode forward")
	require.Equal(t, []string{"question_reply:qst_1:[[postgres]]"}, reply.calls())

	stored := outcome.Record
	require.Equal(t, "decided", stored.Status)
	require.Equal(t, "dec-1", stored.DecisionID)
	require.Equal(t, craft.DecisionAnswer, stored.DecidedAction)
	require.Equal(t, "delivered", stored.Delivery)
	require.Equal(t, rec.Revision+1, stored.Revision, "the decision must consume the revision")
}

func TestControlDecideForwardsNothingWhenDecisionWriteFails(t *testing.T) {
	svc, _, _, _, interactions, reply, _ := newControlFixture(t)
	rec := registerControlInteraction(t, svc, craft.InteractionPermission, "args-p1", "write /etc/hosts", "perm_1")
	interactions.applyErr = errors.New("decision store down")
	_, err := svc.Decide(context.Background(), CraftDecisionRequest{
		Scope: controlScope(), InteractionID: rec.ID, DecisionID: "dec-2",
		Action: craft.DecisionApprove, ArgsHash: rec.ArgsHash, ExpectedRevision: rec.Revision,
	})
	require.Error(t, err)
	require.Empty(t, reply.calls(), "a decision that is not durable must never reach the runtime")
}

func TestControlDecideRejectsChangedArgsHashAsConflict(t *testing.T) {
	svc, _, _, _, _, reply, _ := newControlFixture(t)
	rec := registerControlInteraction(t, svc, craft.InteractionPermission, "args-p1", "write /etc/hosts", "perm_1")

	_, err := svc.Decide(context.Background(), CraftDecisionRequest{
		Scope: controlScope(), InteractionID: rec.ID, DecisionID: "dec-3",
		Action: craft.DecisionApprove, ArgsHash: "args-p1-tampered", ExpectedRevision: rec.Revision,
	})
	require.ErrorIs(t, err, craft.ErrConflict, "an approval for changed arguments must answer 409")
	require.Empty(t, reply.calls())

	_, err = svc.Decide(context.Background(), CraftDecisionRequest{
		Scope: controlScope(), InteractionID: rec.ID, DecisionID: "dec-4",
		Action: craft.DecisionReject, ArgsHash: rec.ArgsHash, ExpectedRevision: rec.Revision + 5,
	})
	require.ErrorIs(t, err, craft.ErrConflict, "a stale revision must answer 409")
	require.Empty(t, reply.calls())
}

func TestControlDecideRejectsCrossUserAsForbidden(t *testing.T) {
	svc, _, _, _, _, reply, _ := newControlFixture(t)
	rec := registerControlInteraction(t, svc, craft.InteractionPermission, "args-p1", "write /etc/hosts", "perm_1")

	intruder := controlScope()
	intruder.UserID = "someone-else"
	_, err := svc.Decide(context.Background(), CraftDecisionRequest{
		Scope: intruder, InteractionID: rec.ID, DecisionID: "dec-5",
		Action: craft.DecisionApprove, ArgsHash: rec.ArgsHash, ExpectedRevision: rec.Revision,
	})
	require.ErrorIs(t, err, craft.ErrForbidden, "another user's interaction must answer 403")
	require.Empty(t, reply.calls(), "no cross-user forward may reach the runtime")
}

func TestControlDecideMapsQuestionRejectAndPermissionReplies(t *testing.T) {
	svc, _, _, _, interactions, reply, _ := newControlFixture(t)
	qrec := registerControlInteraction(t, svc, craft.InteractionQuestion, "args-q1", "Which engine?", "qst_1")
	_, err := svc.Decide(context.Background(), CraftDecisionRequest{
		Scope: controlScope(), InteractionID: qrec.ID, DecisionID: "dec-q",
		Action: craft.DecisionReject, ArgsHash: qrec.ArgsHash, ExpectedRevision: qrec.Revision,
	})
	require.NoError(t, err)

	prec := registerControlInteraction(t, svc, craft.InteractionPermission, "args-p1", "rm file", "perm_1")
	_, err = svc.Decide(context.Background(), CraftDecisionRequest{
		Scope: controlScope(), InteractionID: prec.ID, DecisionID: "dec-p",
		Action: craft.DecisionApprove, ArgsHash: prec.ArgsHash, ExpectedRevision: prec.Revision,
	})
	require.NoError(t, err)

	require.Equal(t, []string{
		"question_reject:qst_1",
		"permission:oc-1:perm_1:once",
	}, reply.calls())
	require.Equal(t, "decided", interactions.record(prec.ID).Status)

	_, err = svc.Decide(context.Background(), CraftDecisionRequest{
		Scope: controlScope(), InteractionID: prec.ID, DecisionID: "dec-x",
		Action: craft.DecisionReject, ArgsHash: prec.ArgsHash, ExpectedRevision: prec.Revision,
	})
	// C02 refines the terminal vocabulary: an already-decided interaction is
	// gone (410) for NEW decision ids — it can never be re-decided; only the
	// same decision id replays the original result.
	require.ErrorIs(t, err, craft.ErrGone, "an already-decided interaction must not be re-decided")
}

func TestControlDecideKeepsUndeliveredDecisionHonest(t *testing.T) {
	svc, interactions, reply, rec := controlQuestionFixture(t)
	reply.err = errors.New("remote unreachable")
	outcome, err := svc.Decide(context.Background(), CraftDecisionRequest{
		Scope: controlScope(), InteractionID: rec.ID, DecisionID: "dec-u",
		Action: craft.DecisionAnswer, ArgsHash: rec.ArgsHash, ExpectedRevision: rec.Revision, Answer: "sqlite",
	})
	require.NoError(t, err, "the decision itself is durable; delivery failure is not a decision failure")
	require.True(t, outcome.Decided)
	require.False(t, outcome.Delivered, "delivery must never be fabricated as successful")
	require.NotEmpty(t, outcome.DeliveryNote)
	require.Equal(t, "unknown", interactions.record(rec.ID).Delivery,
		"the unknown delivery stays recorded for the C02 outbox")
}

// ---- stop -------------------------------------------------------------------

func TestControlStopRecordsIntentThenAbortsAndVerifies(t *testing.T) {
	svc, runs, _, exec, _, _, seq := newControlFixture(t)
	exec.onObserve = func(craft.Task) (craft.Observation, error) {
		return craft.Observation{Aborted: true, Idle: true, SessionID: "oc-1"}, nil
	}
	status, err := svc.Stop(context.Background(), controlStopRequest())
	require.NoError(t, err)
	require.Equal(t, "canceled", status.Phase)
	require.Equal(t, 1, exec.AbortCount())
	_, cancels, _ := runs.counts()
	require.Equal(t, 1, cancels, "the main run cancel intent must be written")
	require.Equal(t, []string{"cancel", "abort"}, seq.steps(),
		"durable cancel intent must precede the runtime abort")
}

func TestControlStopStaysStoppingWhenAbortAcceptedButStillRunning(t *testing.T) {
	svc, _, _, exec, _, _, _ := newControlFixture(t)
	exec.onObserve = func(craft.Task) (craft.Observation, error) {
		// The abort HTTP call returned success, but the session is still busy.
		return craft.Observation{SessionID: "oc-1", Idle: false, PendingTool: true}, nil
	}
	status, err := svc.Stop(context.Background(), controlStopRequest())
	require.NoError(t, err)
	require.Equal(t, "stopping", status.Phase, "HTTP success on abort alone never confirms cancellation")
	require.Equal(t, 1, exec.AbortCount())
}

func TestControlStopPreservesEarlierCompletion(t *testing.T) {
	svc, runs, store, exec, _, _, _ := newControlFixture(t)
	ctx := context.Background()
	require.NoError(t, store.SaveResult(ctx, controlFence(),
		craft.Result{TaskID: "dlg_ctl", Status: "succeeded", Summary: "done"}))

	status, err := svc.Stop(ctx, controlStopRequest())
	require.NoError(t, err)
	require.Equal(t, "completed", status.Phase)
	require.NotNil(t, status.Result)
	require.Equal(t, "succeeded", status.Result.Status, "a normal completion is never overwritten as canceled")
	require.Zero(t, exec.AbortCount(), "no abort is needed for an already finished delegation")

	// The main run itself already finalized: the cancel intent conflicts
	// and the original outcome still wins.
	runs.run.Status = "succeeded"
	runs.cancelEr = agentruntime.ErrConflict
	status, err = svc.Stop(ctx, controlStopRequest())
	require.NoError(t, err)
	require.Equal(t, "completed", status.Phase)
	require.Zero(t, exec.AbortCount())
}

func TestControlStopKeepsResultWhenCompletionRacesAbort(t *testing.T) {
	svc, runs, store, exec, _, _, _ := newControlFixture(t)
	ctx := context.Background()
	saved := craft.Result{TaskID: "dlg_ctl", Status: "succeeded", Summary: "finished at the wire"}
	exec.onObserve = func(craft.Task) (craft.Observation, error) {
		// The worker commits the successful result inside the abort window:
		// completion and cancellation race, completion wins.
		if err := store.SaveResult(ctx, controlFence(), saved); err != nil {
			return craft.Observation{}, err
		}
		return craft.Observation{
			SessionID: "oc-1", PromptMessageID: "msg_ctl",
			Completed: true, Idle: true, Finish: "stop", AssistantParentID: "msg_ctl",
		}, nil
	}

	status, err := svc.Stop(ctx, controlStopRequest())
	require.NoError(t, err)
	require.Equal(t, "completed", status.Phase)
	require.NotNil(t, status.Result)
	require.Equal(t, saved, *status.Result, "the stored result must be preserved, not re-classified")
	require.Equal(t, 1, exec.AbortCount())
	_, cancels, _ := runs.counts()
	require.Equal(t, 1, cancels)
}

func TestControlStopRemoteUnknownStaysStopping(t *testing.T) {
	svc, _, _, exec, _, _, _ := newControlFixture(t)
	exec.onObserve = func(craft.Task) (craft.Observation, error) {
		return craft.Observation{}, errors.New("runtime unreachable")
	}
	status, err := svc.Stop(context.Background(), controlStopRequest())
	require.NoError(t, err)
	require.Equal(t, "stopping", status.Phase, "an unclear remote stays waiting, never canceled")
	require.NotEmpty(t, status.Note)

	exec.onAbort = func(craft.Task) error { return errors.New("abort delivery failed") }
	exec.onObserve = nil
	status, err = svc.Stop(context.Background(), controlStopRequest())
	require.NoError(t, err)
	require.Equal(t, "stopping", status.Phase)
	require.NotEmpty(t, status.Note, "the undelivered abort keeps the stop pending")
}

func TestControlStopRejectsCrossUser(t *testing.T) {
	svc, runs, _, exec, _, _, _ := newControlFixture(t)
	req := controlStopRequest()
	req.Scope.UserID = "someone-else"
	_, err := svc.Stop(context.Background(), req)
	require.ErrorIs(t, err, craft.ErrForbidden, "another user's run must answer 403")
	require.Zero(t, exec.AbortCount())
	_, cancels, _ := runs.counts()
	require.Zero(t, cancels, "no cancel intent may be written for another user")
}

// controlExecutor is the R06 executor fake: it counts aborts and lets each
// test script the runtime observation.
type controlExecutor struct {
	mu        sync.Mutex
	aborts    int
	observes  int
	onAbort   func(craft.Task) error
	onObserve func(craft.Task) (craft.Observation, error)
	seq       *controlSequence
}

func (e *controlExecutor) Execute(context.Context, craft.Task) (craft.Result, error) {
	return craft.Result{}, fmt.Errorf("%w: the control service never dispatches", craft.ErrInvalidInput)
}

func (e *controlExecutor) Observe(_ context.Context, task craft.Task) (craft.Observation, error) {
	e.mu.Lock()
	e.observes++
	fn := e.onObserve
	e.mu.Unlock()
	if fn != nil {
		return fn(task)
	}
	return craft.Observation{SessionID: "oc-1", PromptMessageID: task.PromptMessageID, Idle: true}, nil
}

func (e *controlExecutor) Abort(_ context.Context, task craft.Task) error {
	e.mu.Lock()
	e.aborts++
	fn := e.onAbort
	seq := e.seq
	e.mu.Unlock()
	if fn != nil {
		if err := fn(task); err != nil {
			return err
		}
	}
	if seq != nil {
		seq.record("abort")
	}
	return nil
}

func (e *controlExecutor) AbortCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.aborts
}

func ctxErr(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func TestControlStreamDisconnectNeverAborts(t *testing.T) {
	svc, runs, _, exec, _, _, _ := newControlFixture(t)
	// The browser SSE context is canceled by the disconnect.
	disconnected, cancel := context.WithCancel(context.Background())
	cancel()

	exec.onObserve = func(craft.Task) (craft.Observation, error) {
		if err := ctxErr(disconnected); err != nil {
			return craft.Observation{}, err
		}
		return craft.Observation{Aborted: true, Idle: true}, nil
	}

	_, err := svc.DelegationStatus(disconnected, controlScope(),
		agentruntime.RunKey{TenantID: 1, RunID: "run-ctl"}, "dlg_ctl")
	require.Error(t, err, "a disconnected read fails; it must not mutate anything")

	require.Zero(t, exec.AbortCount(), "an SSE disconnect must never trigger Abort")
	_, cancels, _ := runs.counts()
	require.Zero(t, cancels, "an SSE disconnect must never write a cancel intent")
}

func TestControlStopSurvivesRequesterDisconnect(t *testing.T) {
	svc, runs, _, exec, _, _, _ := newControlFixture(t)
	exec.onObserve = func(craft.Task) (craft.Observation, error) {
		return craft.Observation{Aborted: true, Idle: true}, nil
	}
	// The stop request's own connection dropped after the user clicked stop.
	disconnected, cancel := context.WithCancel(context.Background())
	cancel()

	status, err := svc.Stop(disconnected, controlStopRequest())
	require.NoError(t, err, "a durable stop runs on the server-side budget context")
	require.Equal(t, "canceled", status.Phase)
	_, cancels, _ := runs.counts()
	require.Equal(t, 1, cancels)
	require.Equal(t, 1, exec.AbortCount())
}

func TestControlDelegationStatusIsReadOnly(t *testing.T) {
	svc, runs, _, exec, _, _, _ := newControlFixture(t)
	exec.onObserve = func(craft.Task) (craft.Observation, error) {
		return craft.Observation{Aborted: true, Idle: true}, nil
	}
	status, err := svc.DelegationStatus(context.Background(), controlScope(),
		agentruntime.RunKey{TenantID: 1, RunID: "run-ctl"}, "dlg_ctl")
	require.NoError(t, err)
	require.Equal(t, "running", status.Phase, "no stop requested yet")
	require.Zero(t, exec.AbortCount())
	_, cancels, _ := runs.counts()
	require.Zero(t, cancels)

	// After the durable cancel the same read reports the verified phase.
	require.NoError(t, runs.Cancel(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "run-ctl"}))
	status, err = svc.DelegationStatus(context.Background(), controlScope(),
		agentruntime.RunKey{TenantID: 1, RunID: "run-ctl"}, "dlg_ctl")
	require.NoError(t, err)
	require.Equal(t, "canceled", status.Phase)
	require.Zero(t, exec.AbortCount(), "the status read never aborts")
	_, cancels, _ = runs.counts()
	require.Equal(t, 1, cancels)
}

// ---- the real store backs the stop ordering ---------------------------------

func TestControlStopBlocksNewDispatchOnRealStore(t *testing.T) {
	db := openDurableRunTestDB(t)
	ctx := durableRunCtx()
	runStore := repository.NewAgentRunStore(db)
	key := admitDurableRun(t, runStore, durableRunSnapshot(t))
	fence, err := runStore.Claim(ctx, key, "worker-1", time.Minute)
	require.NoError(t, err)

	craftStore := repository.NewCraftStore(db)
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}
	_, err = craftStore.PutWorkspace(ctx, craft.Workspace{
		Scope: scope, SandboxID: "sbx-1", Generation: "g1",
		OpenCodeSessionID: "oc-1", RuntimeDigest: "digest-craft",
	}, 0)
	require.NoError(t, err)
	ws, err := craftStore.GetWorkspace(ctx, scope)
	require.NoError(t, err)

	plan := agentruntime.ToolPlan{
		CallID: "call-ctl", Name: "craft_delegate", Identity: "craft:delegate:v1",
		ArgsHash: "args-hash-1", Args: []byte(`{"goal":"fix it"}`),
	}
	_, err = runStore.EnsureToolPlan(ctx, fence, plan)
	require.NoError(t, err)

	task := craft.Task{
		ID: "dlg-real", ToolCallID: plan.CallID, Prompt: "fix it", PromptMessageID: "msg_real",
		RequestHash: "hash-real", WorkspaceID: ws.ID, Scope: scope, Fence: fence,
	}
	_, err = craftStore.PrepareTask(ctx, task)
	require.NoError(t, err)

	exec := &controlExecutor{}
	exec.onObserve = func(craft.Task) (craft.Observation, error) {
		return craft.Observation{Aborted: true, Idle: true}, nil
	}
	svc := NewCraftControlService(NewAgentRunService(runStore), craftStore, exec, nil, nil)

	status, err := svc.Stop(ctx, CraftStopRequest{Scope: scope, RunKey: key, TaskID: "dlg-real"})
	require.NoError(t, err)
	require.Equal(t, "canceled", status.Phase)
	require.Equal(t, 1, exec.AbortCount())

	// The durable cancel intent is on the run row and in the event stream.
	run, err := runStore.Get(ctx, key)
	require.NoError(t, err)
	require.Equal(t, "canceled", run.Status)
	var events []struct{ EventType string }
	require.NoError(t, db.Table("agent_run_events").Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).
		Find(&events).Error)
	found := false
	for _, e := range events {
		if e.EventType == "cancellation_requested" {
			found = true
		}
	}
	require.True(t, found, "the cancel intent must be recorded as a durable event")

	// New dispatch under the old fence is blocked: the canceled run fails
	// every fenced write.
	_, err = runStore.EnsureToolPlan(ctx, fence, agentruntime.ToolPlan{
		CallID: "call-after-stop", Name: "craft_delegate", Identity: "craft:delegate:v1",
		ArgsHash: "args-hash-2", Args: []byte(`{"goal":"again"}`),
	})
	require.ErrorIs(t, err, agentruntime.ErrLeaseLost, "a canceled run must refuse new tool dispatch")
	_, err = craftStore.PrepareTask(ctx, craft.Task{
		ID: "dlg-after", ToolCallID: "call-after-stop", Prompt: "again", PromptMessageID: "msg_after",
		RequestHash: "h", WorkspaceID: ws.ID, Scope: scope, Fence: fence,
	})
	require.ErrorIs(t, err, craft.ErrConflict, "a canceled run must refuse new craft delegation")
	err = craftStore.SaveResult(ctx, fence, craft.Result{TaskID: "dlg-real", Status: "succeeded"})
	require.ErrorIs(t, err, craft.ErrConflict, "a canceled run must refuse result writes")

	// A late stop replays idempotently and never flips to a wrong phase.
	status, err = svc.Stop(ctx, CraftStopRequest{Scope: scope, RunKey: key, TaskID: "dlg-real"})
	require.NoError(t, err)
	require.Equal(t, "canceled", status.Phase)
	require.Equal(t, 2, exec.AbortCount())
}
