package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/opencode"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/logger"
)

// craftControlBudget bounds every durable control operation on a
// server-side context derived from the caller, so a disconnecting client can
// neither trigger an abort nor abort a durable stop half way through.
const craftControlBudget = 15 * time.Second

// CraftRunController is the run surface the control service needs.
// *AgentRunService satisfies it without adaptation.
type CraftRunController interface {
	Get(context.Context, agentruntime.RunKey) (agentruntime.Run, error)
	Cancel(context.Context, agentruntime.RunKey) error
	WaitForDecision(context.Context, agentruntime.Fence, string) error
}

// CraftInteractionStore durably records pending Craft interactions and
// applies user decisions with revision and args-hash CAS. The durable
// production store and the delivery outbox are C02's contract; until then
// the control service runs fail-closed on this port.
type CraftInteractionStore interface {
	PutInteraction(context.Context, CraftInteractionRecord) (CraftInteractionRecord, error)
	GetInteraction(context.Context, craft.Scope, string) (CraftInteractionRecord, error)
	ApplyInteractionDecision(context.Context, CraftDecisionRequest) (CraftInteractionRecord, error)
	MarkInteractionDelivery(context.Context, craft.Scope, string, string) error
}

// CraftOpenCodeReplier forwards a durable decision to the OpenCode runtime
// over the locked reply routes. *opencode.Client satisfies it.
type CraftOpenCodeReplier interface {
	ReplyQuestion(context.Context, string, [][]string) error
	RejectQuestion(context.Context, string) error
	ReplyPermission(context.Context, string, string, string) error
}

var (
	_ CraftRunController   = (*AgentRunService)(nil)
	_ CraftOpenCodeReplier = (*opencode.Client)(nil)
)

// CraftInteractionRecord is the server-side durable view of one pending
// question or permission raised by a delegated sub-execution: PendingID is
// the value the main run is parked on, OpenCodeRequestID addresses the
// runtime, and ArgsHash/Revision guard every decision against stale or
// tampered approvals.
type CraftInteractionRecord struct {
	craft.Interaction
	Scope             craft.Scope
	RunID             string
	TaskID            string
	ToolCallID        string
	PendingID         string
	OpenCodeSessionID string
	OpenCodeRequestID string
	Status            string // pending | decided | canceled
	Delivery          string // pending | delivered | unknown
	DecisionID        string
	DecidedAction     string
	// Pending is the structured question/permission payload the user decides
	// on (C02): multi-question options, multi-choice flags, itemized
	// permission facts. It is zero when the registration carried no payload.
	Pending craft.PendingDecision
	// RecordedAnswers re-displays the original answers after a restart.
	RecordedAnswers []craft.Answer
	// DecidedBy is the operator identity that took the decision (audit;
	// never a secret).
	DecidedBy string
}

// CraftDecisionRequest is one user decision on a pending interaction.
type CraftDecisionRequest struct {
	Scope            craft.Scope
	InteractionID    string
	DecisionID       string
	Action           string // craft.DecisionAnswer|DecisionApprove|DecisionReject
	ArgsHash         string
	ExpectedRevision int64
	Answer           string
	// Answers carries the C02 multi-question answer set (choices per question
	// plus optional custom text); validated against the pending payload.
	Answers []craft.Answer
	// Operator is the authenticated user taking the decision; recorded with
	// the durable decision for audit (identity only, never secrets).
	Operator string
	Reason   string
}

// CraftDecisionOutcome reports what actually happened: the decision is
// durable, the runtime delivery may still be unconfirmed.
type CraftDecisionOutcome struct {
	Record       CraftInteractionRecord
	Decided      bool
	Delivered    bool
	DeliveryNote string
}

// CraftStopRequest addresses one delegated execution for a durable stop.
type CraftStopRequest struct {
	Scope  craft.Scope
	RunKey agentruntime.RunKey
	TaskID string
}

// CraftStopStatus is the verifiable stop phase: stopping | canceled |
// completed | failed. "canceled" is only reported after the runtime
// confirmed the abort; a normal completion is never re-classified.
type CraftStopStatus struct {
	Phase  string
	Result *craft.Result
	Note   string
}

// CraftControlService drives the minimal R06 interaction and stop surface:
// it parks OpenCode questions and permissions on the existing durable
// pending state, applies decisions durably before forwarding them, and
// stops a delegation by recording the cancel intent first, then aborting,
// then verifying. It never auto-approves, never fabricates delivery and
// never aborts from a subscription context.
type CraftControlService struct {
	runs         CraftRunController
	store        craft.Store
	executor     atomic.Pointer[craft.Executor] // post-construction injection (wireCraftInteractionRegistrar)
	interactions CraftInteractionStore
	reply        CraftOpenCodeReplier
}

// NewCraftControlService assembles the control service. A nil interaction
// store or replier keeps registration and decisions fail-closed; a nil
// executor leaves stop at the recorded-intent stage.
func NewCraftControlService(
	runs CraftRunController,
	store craft.Store,
	executor craft.Executor,
	interactions CraftInteractionStore,
	reply CraftOpenCodeReplier,
) *CraftControlService {
	svc := &CraftControlService{
		runs: runs, store: store,
		interactions: interactions, reply: reply,
	}
	svc.SetExecutor(executor)
	return svc
}

// SetExecutor installs the craft executor post-construction (the provider
// cycle is broken exactly like the interaction emitter: the runtime is
// assembled after this service). Nil keeps the recorded-intent degrade.
func (s *CraftControlService) SetExecutor(executor craft.Executor) {
	if s == nil || executor == nil {
		return
	}
	s.executor.Store(&executor)
}

// currentExecutor returns the live executor or nil (recorded-intent stop).
func (s *CraftControlService) currentExecutor() craft.Executor {
	if held := s.executor.Load(); held != nil {
		return *held
	}
	return nil
}

// craftControlContext derives the server-side budget context: values are
// kept, caller cancellation is not. A client disconnecting never cancels
// durable work already in flight.
func craftControlContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), craftControlBudget)
}

// craftInteractionID derives the durable interaction identity from the
// delegation and the OpenCode request it carries: stable across retries.
func craftInteractionID(scope craft.Scope, taskID, ocRequestID string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("craft-interaction/%d/%s/%s/%s",
		scope.TenantID, scope.SessionID, taskID, ocRequestID)))
	return "itx_" + hex.EncodeToString(sum[:16])
}

// craftInteractionArgsHash digests the exact interaction payload the user
// decides on, so an approval can never authorize different arguments.
func craftInteractionArgsHash(kind, prompt, ocRequestID string) string {
	sum := sha256.Sum256([]byte(kind + "\x00" + prompt + "\x00" + ocRequestID))
	return "iargs_" + hex.EncodeToString(sum[:16])
}

// RegisterInteraction persists a pending interaction and hangs the OpenCode
// wait on the existing durable pending state of the main run. Without an
// interaction store (no UI support) it fails closed with ErrUnsupported:
// the request keeps waiting explicitly for a human, it is never silently
// approved in the background.
func (s *CraftControlService) RegisterInteraction(
	ctx context.Context, fence agentruntime.Fence, task craft.Task,
	in craft.Interaction, ocSessionID, ocRequestID string,
) (CraftInteractionRecord, error) {
	if s == nil || s.interactions == nil {
		return CraftInteractionRecord{}, fmt.Errorf(
			"%w: no durable interaction support; the request keeps waiting for a human decision",
			craft.ErrUnsupported)
	}
	if fence.TenantID == 0 || fence.RunID == "" || fence.Owner == "" || fence.Epoch <= 0 ||
		task.ID == "" || task.ToolCallID == "" || ocSessionID == "" || ocRequestID == "" {
		return CraftInteractionRecord{}, fmt.Errorf("%w: incomplete interaction registration", craft.ErrInvalidInput)
	}
	if in.Kind != craft.InteractionQuestion && in.Kind != craft.InteractionPermission {
		return CraftInteractionRecord{}, fmt.Errorf("%w: interaction kind %q", craft.ErrInvalidInput, in.Kind)
	}
	if in.Prompt == "" {
		return CraftInteractionRecord{}, fmt.Errorf("%w: interaction prompt is required", craft.ErrInvalidInput)
	}
	if in.ID == "" {
		in.ID = craftInteractionID(task.Scope, task.ID, ocRequestID)
	}
	if in.ArgsHash == "" {
		in.ArgsHash = craftInteractionArgsHash(in.Kind, in.Prompt, ocRequestID)
	}
	record := CraftInteractionRecord{
		Interaction:       in,
		Scope:             task.Scope,
		RunID:             task.Fence.RunID,
		TaskID:            task.ID,
		ToolCallID:        task.ToolCallID,
		PendingID:         in.ID,
		OpenCodeSessionID: ocSessionID,
		OpenCodeRequestID: ocRequestID,
	}
	stored, err := s.interactions.PutInteraction(ctx, record)
	if err != nil {
		return CraftInteractionRecord{}, err
	}
	// Park the main run on its existing pending state; the park is retried
	// on the next registration if it fails, the record already survives.
	if s.runs != nil {
		if err := s.runs.WaitForDecision(ctx, fence, stored.PendingID); err != nil {
			return stored, err
		}
	}
	return stored, nil
}

// craftInteractionLister is the optional listing surface a C02 interaction
// store offers; the production store implements it.
type craftInteractionLister interface {
	ListInteractions(context.Context, craft.Scope, string) ([]CraftInteractionRecord, error)
}

// ListInteractions exposes the session's pending decisions (and their
// recorded answers once decided) for the interaction surface. Without a C02
// listing store it fails closed with ErrUnsupported.
func (s *CraftControlService) ListInteractions(ctx context.Context, scope craft.Scope, sessionID string) ([]CraftInteractionRecord, error) {
	if s == nil || s.interactions == nil {
		return nil, fmt.Errorf("%w: no durable interaction support", craft.ErrUnsupported)
	}
	lister, ok := s.interactions.(craftInteractionLister)
	if !ok {
		return nil, fmt.Errorf("%w: the interaction store cannot list", craft.ErrUnsupported)
	}
	return lister.ListInteractions(ctx, scope, sessionID)
}

// Decide applies one user decision. The decision is written durably first;
// only then is it forwarded to the OpenCode runtime. A changed args hash or
// a stale revision answers ErrConflict (409), another user's interaction
// answers ErrForbidden (403), and a forward the runtime cannot confirm is
// kept as an honest delivery_unknown — success is never fabricated and the
// reliable redelivery outbox belongs to C02.
func (s *CraftControlService) Decide(ctx context.Context, req CraftDecisionRequest) (CraftDecisionOutcome, error) {
	if s == nil || s.interactions == nil {
		return CraftDecisionOutcome{}, fmt.Errorf(
			"%w: no durable interaction support; the decision cannot be recorded", craft.ErrUnsupported)
	}
	if req.InteractionID == "" || req.DecisionID == "" || req.Action == "" {
		return CraftDecisionOutcome{}, fmt.Errorf("%w: interaction, decision and action are required", craft.ErrInvalidInput)
	}
	record, err := s.interactions.GetInteraction(ctx, req.Scope, req.InteractionID)
	if err != nil {
		return CraftDecisionOutcome{}, err
	}
	if !craft.DecisionAllowed(record.Kind, req.Action) {
		return CraftDecisionOutcome{}, fmt.Errorf(
			"%w: action %q is not a user decision for a %s", craft.ErrInvalidInput, req.Action, record.Kind)
	}
	if record.Kind == craft.InteractionQuestion && req.Action == craft.DecisionAnswer {
		if len(req.Answers) == 0 && req.Answer == "" {
			return CraftDecisionOutcome{}, fmt.Errorf("%w: answering a question requires the answers", craft.ErrInvalidInput)
		}
		// The C02 multi-question rules run server-side against the exact
		// pending payload: known question ids, legal choices, single-choice
		// at most one, text bounded by 8 KiB.
		if len(req.Answers) > 0 {
			if err := craft.ValidateAnswers(record.Pending, req.Answers); err != nil {
				return CraftDecisionOutcome{}, err
			}
		}
	}
	if record.Kind == craft.InteractionPermission && (req.Answer != "" || len(req.Answers) > 0) {
		return CraftDecisionOutcome{}, fmt.Errorf("%w: a permission decision carries no answers", craft.ErrInvalidInput)
	}
	if record.Kind == craft.InteractionQuestion && req.Action == craft.DecisionReject && (req.Answer != "" || len(req.Answers) > 0) {
		return CraftDecisionOutcome{}, fmt.Errorf("%w: rejecting a question carries no answers", craft.ErrInvalidInput)
	}
	if req.ArgsHash == "" || req.ArgsHash != record.ArgsHash {
		return CraftDecisionOutcome{}, fmt.Errorf(
			"%w: interaction %s arguments changed since the user decided", craft.ErrConflict, req.InteractionID)
	}
	// A terminal interaction no longer accepts NEW decisions (410); an
	// idempotent replay of the same decision id falls through to the store,
	// which returns the original result.
	if record.Status != "pending" && record.DecisionID != req.DecisionID {
		return CraftDecisionOutcome{}, fmt.Errorf(
			"%w: interaction %s is %s", craft.ErrGone, req.InteractionID, record.Status)
	}
	if record.Status == "pending" && req.ExpectedRevision != record.Revision {
		return CraftDecisionOutcome{}, fmt.Errorf(
			"%w: interaction %s revision %d is not current %d",
			craft.ErrConflict, req.InteractionID, req.ExpectedRevision, record.Revision)
	}

	// The durable decision always precedes the runtime forward.
	applied, err := s.interactions.ApplyInteractionDecision(ctx, req)
	if err != nil {
		return CraftDecisionOutcome{}, err
	}
	outcome := CraftDecisionOutcome{Record: applied, Decided: true}

	forwardCtx, cancel := craftControlContext(ctx)
	defer cancel()
	var forwardErr error
	if s.reply == nil {
		forwardErr = errors.New("no OpenCode forwarder is wired")
	} else {
		switch {
		case record.Kind == craft.InteractionQuestion && req.Action == craft.DecisionAnswer:
			forwardErr = s.reply.ReplyQuestion(forwardCtx, record.OpenCodeRequestID, decideAnswerRows(req))
		case record.Kind == craft.InteractionQuestion:
			forwardErr = s.reply.RejectQuestion(forwardCtx, record.OpenCodeRequestID)
		case req.Action == craft.DecisionApprove:
			forwardErr = s.reply.ReplyPermission(forwardCtx, record.OpenCodeSessionID, record.OpenCodeRequestID, "once")
		default:
			forwardErr = s.reply.ReplyPermission(forwardCtx, record.OpenCodeSessionID, record.OpenCodeRequestID, "reject")
		}
	}
	if forwardErr == nil {
		outcome.Delivered = true
		outcome.Record.Delivery = "delivered"
		if err := s.interactions.MarkInteractionDelivery(ctx, req.Scope, req.InteractionID, "delivered"); err != nil {
			outcome.DeliveryNote = fmt.Sprintf("delivered; delivery marker not persisted: %v", err)
		}
		s.syncOutboxDelivery(ctx, record, req, "delivered", "")
		return outcome, nil
	}
	outcome.Record.Delivery = "unknown"
	note := fmt.Sprintf("decision recorded; OpenCode delivery unconfirmed: %v", forwardErr)
	if err := s.interactions.MarkInteractionDelivery(ctx, req.Scope, req.InteractionID, "unknown"); err != nil {
		note = fmt.Sprintf("%s; delivery marker not persisted: %v", note, err)
	}
	s.syncOutboxDelivery(ctx, record, req, "unknown", note)
	outcome.DeliveryNote = note
	return outcome, nil
}

// decideAnswerRows maps the C02 answer set onto the locked question reply
// shape; the legacy single Answer field stays a one-row fallback.
func decideAnswerRows(req CraftDecisionRequest) [][]string {
	if len(req.Answers) > 0 {
		return answerRows(req.Answers)
	}
	return [][]string{{req.Answer}}
}

// syncOutboxDelivery keeps the durable outbox in step with the inline forward
// result: a confirmed forward acks the outbox item; an unconfirmable one is
// marked delivery_unknown so the user-facing state is honest. Stores without
// the C02 outbox (the R06 fake) keep their previous behavior.
func (s *CraftControlService) syncOutboxDelivery(ctx context.Context, record CraftInteractionRecord, req CraftDecisionRequest, state, note string) {
	outbox, ok := s.interactions.(CraftDecisionOutboxRo)
	if !ok || record.RunID == "" {
		return
	}
	key := agentruntime.RunKey{TenantID: record.Scope.TenantID, RunID: record.RunID}
	var err error
	if state == "delivered" {
		err = outbox.AckDecision(ctx, key, req.InteractionID, req.DecisionID, note)
	} else {
		err = outbox.MarkDecisionUnknown(ctx, key, req.InteractionID, req.DecisionID, note)
	}
	if err != nil {
		logger.Warnf(ctx, "[CraftControl] outbox %s persistence failed for %s: %v", state, req.DecisionID, err)
	}
}

// stopPhaseForResult maps a stored terminal result onto the stop phase.
func stopPhaseForResult(result craft.Result) string {
	switch result.Status {
	case "canceled":
		return "canceled"
	case "failed":
		return "failed"
	default:
		return "completed"
	}
}

// Stop stops one delegated execution with the mandated ordering: the main
// run cancel intent is written first (which also blocks every new dispatch
// under the run fence), the runtime abort is requested second, and only an
// observation confirming the abort on an idle session reports "canceled" —
// everything else stays "stopping". A normal completion that arrived first
// is preserved with its original result and never re-classified.
func (s *CraftControlService) Stop(ctx context.Context, req CraftStopRequest) (CraftStopStatus, error) {
	if s == nil || s.runs == nil {
		return CraftStopStatus{}, fmt.Errorf("%w: control service is not assembled", craft.ErrInvalidInput)
	}
	if req.RunKey.TenantID == 0 || req.RunKey.RunID == "" || req.TaskID == "" ||
		req.Scope.TenantID == 0 || req.Scope.UserID == "" || req.Scope.SessionID == "" {
		return CraftStopStatus{}, fmt.Errorf("%w: incomplete stop request", craft.ErrInvalidInput)
	}
	detachCtx, cancel := craftControlContext(ctx)
	defer cancel()

	run, err := s.runs.Get(detachCtx, req.RunKey)
	if err != nil {
		return CraftStopStatus{}, err
	}
	if run.SessionID != req.Scope.SessionID || run.UserID != req.Scope.UserID {
		return CraftStopStatus{}, fmt.Errorf("%w: run %s is not owned by this session user",
			craft.ErrForbidden, req.RunKey.RunID)
	}
	switch run.Status {
	case "succeeded":
		return CraftStopStatus{Phase: "completed", Note: "run already completed normally; nothing to stop"}, nil
	case "failed":
		return CraftStopStatus{Phase: "failed", Note: "run already failed; nothing to stop"}, nil
	}

	// 1. Durable cancel intent on the main run. A canceled run fails every
	//    fenced write, which is what blocks new dispatch.
	if err := s.runs.Cancel(detachCtx, req.RunKey); err != nil {
		if errors.Is(err, agentruntime.ErrConflict) {
			again, getErr := s.runs.Get(detachCtx, req.RunKey)
			if getErr == nil {
				switch again.Status {
				case "succeeded":
					return CraftStopStatus{Phase: "completed", Note: "run completed normally before the stop landed"}, nil
				case "failed":
					return CraftStopStatus{Phase: "failed", Note: "run failed before the stop landed"}, nil
				}
			}
		}
		return CraftStopStatus{}, err
	}

	// 2. A delegation that already settled keeps its result: the stop raced
	//    a normal completion and the completion wins.
	if s.store != nil {
		if result, err := s.store.GetResult(detachCtx, req.Scope, req.TaskID); err == nil {
			return CraftStopStatus{
				Phase: stopPhaseForResult(result), Result: &result,
				Note: "delegation already settled before the stop; the original result is preserved",
			}, nil
		} else if !errors.Is(err, craft.ErrNotFound) {
			return CraftStopStatus{}, err
		}
	}

	// 3. Abort the addressed sub-execution, then verify.
	var task craft.Task
	if s.store != nil {
		stored, err := s.store.GetTask(detachCtx, req.Scope, req.TaskID)
		if err != nil {
			if errors.Is(err, craft.ErrNotFound) {
				return CraftStopStatus{Phase: "stopping",
					Note: "cancel intent recorded; the delegation record is unavailable for abort"}, nil
			}
			return CraftStopStatus{}, err
		}
		task = stored
	}
	abortNote := ""
	exec := s.currentExecutor()
	if exec != nil && task.ID != "" {
		if err := exec.Abort(detachCtx, task); err != nil {
			abortNote = fmt.Sprintf("; abort delivery unclear: %v", err)
		}
	}
	if exec == nil || task.ID == "" {
		return CraftStopStatus{Phase: "stopping",
			Note: "cancel intent recorded; no executor is available to abort the sub-execution"}, nil
	}
	observation, err := exec.Observe(detachCtx, task)
	if err != nil {
		return CraftStopStatus{Phase: "stopping",
			Note: fmt.Sprintf("cancel intent recorded and abort requested; remote state unverified: %v%s", err, abortNote)}, nil
	}
	// The completion may have landed inside the abort window.
	if s.store != nil {
		if result, rerr := s.store.GetResult(detachCtx, req.Scope, req.TaskID); rerr == nil {
			return CraftStopStatus{
				Phase: stopPhaseForResult(result), Result: &result,
				Note: "the delegation completed normally while the stop was in flight; the original result is preserved",
			}, nil
		}
	}
	if opencode.Completed(observation) {
		return CraftStopStatus{Phase: "completed",
			Note: "the sub-execution completed normally; the cancellation raced and lost"}, nil
	}
	phase := craft.StopStatus(true, observation)
	if phase == "stopping" {
		return CraftStopStatus{Phase: phase,
			Note: "cancel intent recorded and abort requested; the runtime has not confirmed the abort yet" + abortNote}, nil
	}
	return CraftStopStatus{Phase: phase}, nil
}

// DelegationStatus is the read-only verification path a client polls after
// a stop answered "stopping". It never writes a cancel intent and never
// aborts — in particular a canceled subscription context (browser
// disconnect) cannot mutate anything.
func (s *CraftControlService) DelegationStatus(
	ctx context.Context, scope craft.Scope, key agentruntime.RunKey, taskID string,
) (CraftStopStatus, error) {
	if s == nil || s.runs == nil {
		return CraftStopStatus{}, fmt.Errorf("%w: control service is not assembled", craft.ErrInvalidInput)
	}
	run, err := s.runs.Get(ctx, key)
	if err != nil {
		return CraftStopStatus{}, err
	}
	if run.SessionID != scope.SessionID || run.UserID != scope.UserID {
		return CraftStopStatus{}, fmt.Errorf("%w: run %s is not owned by this session user",
			craft.ErrForbidden, key.RunID)
	}
	requested := run.Status == "canceled"
	if s.store != nil && taskID != "" {
		if result, err := s.store.GetResult(ctx, scope, taskID); err == nil {
			return CraftStopStatus{Phase: stopPhaseForResult(result), Result: &result}, nil
		}
		task, err := s.store.GetTask(ctx, scope, taskID)
		if exec := s.currentExecutor(); err == nil && exec != nil {
			observation, oerr := exec.Observe(ctx, task)
			if oerr != nil {
				return CraftStopStatus{}, oerr
			}
			if opencode.Completed(observation) {
				return CraftStopStatus{Phase: "completed"}, nil
			}
			return CraftStopStatus{Phase: craft.StopStatus(requested, observation)}, nil
		}
	}
	return CraftStopStatus{Phase: craft.StopStatus(requested, craft.Observation{})}, nil
}
