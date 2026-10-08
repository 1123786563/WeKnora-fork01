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
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
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
	// Outcome is the T17 (#136) frozen stop-outcome projection (T00 DTO):
	// requested (the member asked; the executor may still run), confirmed
	// (an authoritative observation confirmed the cancellation) and unknown
	// (the abort outcome could not be determined). The phase keeps its R06
	// vocabulary for existing clients; the outcome is the distinct,
	// non-collapsible answer for "did it actually stop".
	Outcome craft.StopOutcome
}

// CraftStopIntentStore is the T17 (#136) durable stop-intent seam: the
// member's stop request persists as its own durable fact (requested) BEFORE
// any executor abort is requested, and only an authoritative confirmation
// moves it to confirmed. Rows are idempotent by Run identity; a replay of
// the same Run never downgrades a confirmed stop. A nil seam keeps the
// pre-T17 stop ordering (the run cancel intent first) — the fail-closed
// degrade, exactly like the T16 nil lease seam.
type CraftStopIntentStore interface {
	// PutStopIntent persists (or replays) one Run's stop intent; it is
	// called before Abort on every stop journey.
	PutStopIntent(context.Context, craft.Scope, craft.StopIntent) (craft.StopIntent, error)
	// GetStopIntent reads one Run's durable stop intent. Implementations
	// MUST return an error wrapping craft.ErrNotFound when no intent row
	// exists for the Run — callers branch on exactly that sentinel to
	// separate "no stop journey on record" from a store failure.
	GetStopIntent(context.Context, craft.Scope, string) (craft.StopIntent, error)
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
	stopIntents  atomic.Pointer[CraftStopIntentStore] // post-construction injection (T17)
	taskAccess   atomic.Pointer[craft.TaskAccessChecker]
}

// SetTaskAccess installs the live Craft membership/role checker used by
// Stop and DelegationStatus. A missing checker fails closed at authorization.
func (s *CraftControlService) SetTaskAccess(checker craft.TaskAccessChecker) {
	if s == nil || checker == nil {
		return
	}
	s.taskAccess.Store(&checker)
}

func (s *CraftControlService) currentTaskAccess() craft.TaskAccessChecker {
	if s == nil {
		return nil
	}
	if held := s.taskAccess.Load(); held != nil {
		return *held
	}
	return nil
}

// authorizeRunAccess combines the authenticated caller's current Task role
// with durable Run ownership. Scope.UserID remains the Session storage owner;
// it is not used as a substitute for the authenticated caller.
func (s *CraftControlService) authorizeRunAccess(
	ctx context.Context, scope craft.Scope, key agentruntime.RunKey, run agentruntime.Run, action craft.TaskAction,
) (string, error) {
	if scope.TenantID == 0 || scope.SessionID == "" || scope.UserID == "" || key.RunID == "" ||
		key.TenantID != scope.TenantID || run.Key.TenantID != scope.TenantID ||
		run.Key.RunID != key.RunID || run.SessionID != scope.SessionID || run.UserID != scope.UserID {
		return "", fmt.Errorf("%w: Run is outside the Task scope", craft.ErrForbidden)
	}
	caller := types.CallerFromContext(ctx)
	callerID := caller.UserID
	if callerID == "" || caller.TenantID != scope.TenantID {
		return "", craft.ErrForbidden
	}
	callerScope := scope
	callerScope.UserID = callerID
	if err := craft.RequireTaskAccess(ctx, s.currentTaskAccess(), callerScope, action); err != nil {
		return "", err
	}
	if action == craft.TaskWrite && callerID != run.UserID && callerID != run.ActorUserID {
		return "", fmt.Errorf("%w: collaborator may stop only a Run they initiated", craft.ErrForbidden)
	}
	return callerID, nil
}

// persistedRunScope validates the route scope against the tenant-scoped Run
// key, then uses the Run's persisted owner for all Craft storage operations.
// The caller remains independently available through CallerFromContext.
func persistedRunScope(scope craft.Scope, key agentruntime.RunKey, run agentruntime.Run) (craft.Scope, error) {
	if scope.TenantID == 0 || scope.SessionID == "" || scope.UserID == "" || key.RunID == "" ||
		key.TenantID != scope.TenantID || run.Key != key || run.SessionID != scope.SessionID || run.UserID == "" {
		return craft.Scope{}, fmt.Errorf("%w: Run is outside the Task scope", craft.ErrForbidden)
	}
	scope.UserID = run.UserID
	return scope, nil
}

func validateControlTask(task craft.Task, scope craft.Scope, key agentruntime.RunKey, taskID string) error {
	if task.ID != taskID || !craft.SameScope(task.Scope, scope) || task.Fence.RunKey != key {
		return fmt.Errorf("%w: delegation is outside the addressed Run", craft.ErrForbidden)
	}
	return nil
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

// SetStopIntents installs the T17 (#136) durable stop-intent store
// post-construction (the production store is wired by the container). Nil
// keeps the pre-T17 stop ordering (the run cancel intent first).
func (s *CraftControlService) SetStopIntents(store CraftStopIntentStore) {
	if s == nil || store == nil {
		return
	}
	s.stopIntents.Store(&store)
}

// currentStopIntents returns the live stop-intent store or nil (the
// pre-T17 stop ordering).
func (s *CraftControlService) currentStopIntents() CraftStopIntentStore {
	if held := s.stopIntents.Load(); held != nil {
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

// durableStopOutcome projects the frozen T00 stop outcome of one Run: the
// persisted stop intent is the authority; without a persisted intent (the
// pre-T17 assemblies) the just-observed fallback stands.
func (s *CraftControlService) durableStopOutcome(ctx context.Context, scope craft.Scope, runID string, fallback craft.StopOutcomeStatus) craft.StopOutcome {
	if s.currentStopIntents() != nil {
		intent, err := s.currentStopIntents().GetStopIntent(ctx, scope, runID)
		switch {
		case err == nil:
			return craft.StopOutcome{RunID: runID, Status: intent.Status}
		case intentErrNotFound(err):
			// No intent row exists — the fallback observation stands.
		default:
			// A transient store failure must NOT be silently degraded to the
			// fallback: a confirmed stop could project as requested. Warn so
			// the failure is visible, then keep the fallback (the read-only
			// projection surface cannot repair the store).
			logger.Warnf(ctx, "[CraftControl] durable stop intent read failed for run %s; projecting fallback %q: %v", runID, fallback, err)
		}
	}
	return craft.StopOutcome{RunID: runID, Status: fallback}
}

// intentErrNotFound reports whether a GetStopIntent error means "no intent
// row exists" (as opposed to a store failure).
func intentErrNotFound(err error) bool {
	return err != nil && errors.Is(err, craft.ErrNotFound)
}

// Stop stops one delegated execution with the mandated ordering. T17 (#136)
// durable path (a stop-intent store is wired): the member's stop intent
// persists as its own durable fact BEFORE the executor abort is requested;
// the accepted answer keeps the Run nonterminal ("stopping" — the writer
// fence and the promotion gate stay in force); an abort whose outcome
// cannot be observed persists and answers unknown; and only the
// authoritative confirmation (observed abort on an idle session) writes the
// terminal canceled status and the confirmed intent. Repeated stops replay
// the durable answer. A normal completion that arrived first is preserved
// with its original result and never re-classified. The pre-T17 path (no
// stop-intent store) keeps the R06 ordering: the run cancel intent first,
// then abort, then verify.
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

	// requestedOutcome is the durable T00 projection of this stop journey at
	// the points where the stop has not been confirmed: the persisted intent
	// (requested or unknown) is the authority, the requested fallback stands
	// for the pre-T17 assemblies.
	requestedOutcome := func() craft.StopOutcome {
		return s.durableStopOutcome(detachCtx, req.Scope, req.RunKey.RunID, craft.StopRequested)
	}
	// confirmedReplay is the idempotent answer once the cancellation is
	// durably confirmed (the run row CAS or the confirmed intent marker).
	confirmedReplay := func() CraftStopStatus {
		return CraftStopStatus{Phase: "canceled",
			Note:    "stop already confirmed; replaying the durable answer",
			Outcome: craft.StopOutcome{RunID: req.RunKey.RunID, Status: craft.StopConfirmed}}
	}
	// supersededOutcome is the bypass projection for a NON-canceled terminal
	// row that overtook the stop: the durable intent row (possibly a stale
	// unknown from an unobservable abort that never landed) must not be
	// replayed next to the terminal phase — requested records the ask, the
	// phase carries the settlement.
	supersededOutcome := func() craft.StopOutcome {
		return craft.StopOutcome{RunID: req.RunKey.RunID, Status: craft.StopRequested}
	}

	run, err := s.runs.Get(detachCtx, req.RunKey)
	if err != nil {
		return CraftStopStatus{}, err
	}
	req.Scope, err = persistedRunScope(req.Scope, req.RunKey, run)
	if err != nil {
		return CraftStopStatus{}, err
	}
	if _, err := s.authorizeRunAccess(detachCtx, req.Scope, req.RunKey, run, craft.TaskWrite); err != nil {
		return CraftStopStatus{}, err
	}
	var task craft.Task
	hasTask := false
	if s.store != nil {
		task, err = s.store.GetTask(detachCtx, req.Scope, req.TaskID)
		if err == nil {
			if err := validateControlTask(task, req.Scope, req.RunKey, req.TaskID); err != nil {
				return CraftStopStatus{}, err
			}
			hasTask = true
		} else if !errors.Is(err, craft.ErrNotFound) {
			return CraftStopStatus{}, err
		}
	}
	switch run.Status {
	case "succeeded":
		return CraftStopStatus{Phase: "completed", Note: "run already completed normally; nothing to stop",
			Outcome: supersededOutcome()}, nil
	case "failed":
		return CraftStopStatus{Phase: "failed", Note: "run already failed; nothing to stop",
			Outcome: supersededOutcome()}, nil
	}
	if s.currentStopIntents() != nil {
		// A canceled run row replays confirmed ONLY when the durable stop
		// intent actually says confirmed: session deletion
		// (CancelSessionRuns) and the generic cancel endpoint also write
		// "canceled", and conflating them fabricates a stop confirmation.
		intent, ierr := s.currentStopIntents().GetStopIntent(detachCtx, req.Scope, req.RunKey.RunID)
		if ierr == nil && intent.Status == craft.StopConfirmed {
			return confirmedReplay(), nil
		}
		if run.Status == "canceled" && intentErrNotFound(ierr) {
			// The run was canceled by a NON-stop path with no stop intent on
			// record — replay the row's terminal fact WITHOUT claiming a stop
			// journey (and without aborting again): the independent wording
			// matters, confirmedReplay() would announce "stop already
			// confirmed" and fabricate a stop journey that never happened.
			return CraftStopStatus{Phase: "canceled",
				Note:    "run already canceled by a non-stop path; nothing to stop",
				Outcome: craft.StopOutcome{RunID: req.RunKey.RunID, Status: craft.StopConfirmed}}, nil
		}
		// 1. The stop intent persists BEFORE anything is aborted: an accepted
		//    stop survives every later failure. NOTE: unlike the pre-T17
		//    branch below (whose immediate runs.Cancel fails every fenced
		//    write and so blocks new dispatch), nothing here blocks new
		//    dispatch until T20 lands the intent-aware dispatch check in the
		//    same batch as the container's SetStopIntents assembly — the run
		//    row stays nonterminal by design and the window is bounded by
		//    the Abort in step 3.
		if _, err := s.currentStopIntents().PutStopIntent(detachCtx, req.Scope, craft.StopIntent{
			RunID: req.RunKey.RunID, Status: craft.StopRequested,
		}); err != nil {
			return CraftStopStatus{}, err
		}
	} else {
		// Pre-T17 ordering: the durable run cancel intent first — a canceled
		// run fails every fenced write, which is what blocks new dispatch.
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
	}

	// 2. A delegation that already settled keeps its result: the stop raced
	//    a normal completion and the completion wins. A CANCELED result is
	//    the executor's abort confirmation, NOT a pre-stop settlement (see
	//    the abort-window branch below) — it must continue to the terminal
	//    write so the stop converges.
	if s.store != nil && hasTask {
		if result, err := s.store.GetResult(detachCtx, req.Scope, req.TaskID); err == nil && result.Status != "canceled" {
			return CraftStopStatus{
				Phase: stopPhaseForResult(result), Result: &result,
				Note:    "delegation already settled before the stop; the original result is preserved",
				Outcome: requestedOutcome(),
			}, nil
		} else if !errors.Is(err, craft.ErrNotFound) && err != nil {
			return CraftStopStatus{}, err
		}
	}

	// 3. Abort the addressed sub-execution, then verify.
	if s.store != nil && !hasTask {
		return CraftStopStatus{
			Phase:   "stopping",
			Note:    "cancel intent recorded; the delegation record is unavailable for abort",
			Outcome: requestedOutcome(),
		}, nil
	}
	abortNote := ""
	exec := s.currentExecutor()
	if exec != nil && task.ID != "" {
		if err := exec.Abort(detachCtx, task); err != nil {
			abortNote = fmt.Sprintf("; abort delivery unclear: %v", err)
		}
	}
	if exec == nil || task.ID == "" {
		return CraftStopStatus{
			Phase:   "stopping",
			Note:    "cancel intent recorded; no executor is available to abort the sub-execution",
			Outcome: requestedOutcome(),
		}, nil
	}
	observation, err := exec.Observe(detachCtx, task)
	if err != nil {
		// The abort outcome could not be observed: that is the distinct
		// durable unknown — never a confirmation, and the run stays
		// nonterminal with the fence retained. EXCEPT when the run row is
		// already terminally canceled (an earlier stop crashed between the
		// CAS and its confirmed marker): recording unknown here would
		// PERMANENTLY downgrade a confirmed journey — back-fill the marker
		// instead, the same repair as the Cancel-conflict replay above.
		if s.currentStopIntents() != nil && run.Status == "canceled" {
			_, _ = s.currentStopIntents().PutStopIntent(detachCtx, req.Scope, craft.StopIntent{
				RunID: req.RunKey.RunID, Status: craft.StopConfirmed,
			})
			return confirmedReplay(), nil
		}
		if s.currentStopIntents() != nil {
			if _, uerr := s.currentStopIntents().PutStopIntent(detachCtx, req.Scope, craft.StopIntent{
				RunID: req.RunKey.RunID, Status: craft.StopUnknown,
			}); uerr == nil {
				return CraftStopStatus{
					Phase:   "stopping",
					Note:    fmt.Sprintf("stop intent recorded and abort requested; the abort outcome is unknown: %v%s", err, abortNote), //nolint:lll // 一行诊断信息
					Outcome: craft.StopOutcome{RunID: req.RunKey.RunID, Status: craft.StopUnknown},
				}, nil
			}
		}
		return CraftStopStatus{
			Phase:   "stopping",
			Note:    fmt.Sprintf("cancel intent recorded and abort requested; remote state unverified: %v%s", err, abortNote), //nolint:lll // 预存长行,import 修复入 range
			Outcome: requestedOutcome(),
		}, nil
	}
	// The completion may have landed inside the abort window. A CANCELED
	// result here is the executor's own abort confirmation (the real
	// opencode executor writes it through the fenced SaveResult while the
	// run row is still running under T17): returning early on it would
	// strand the run non-terminal with Outcome=requested forever. Fall
	// through to the terminal write + confirmed marker instead.
	if s.store != nil {
		if result, rerr := s.store.GetResult(detachCtx, req.Scope, req.TaskID); rerr == nil && result.Status != "canceled" {
			return CraftStopStatus{
				Phase: stopPhaseForResult(result), Result: &result,
				Note:    "the delegation completed normally while the stop was in flight; the original result is preserved",
				Outcome: requestedOutcome(),
			}, nil
		}
	}
	if opencode.Completed(observation) {
		return CraftStopStatus{
			Phase:   "completed",
			Note:    "the sub-execution completed normally; the cancellation raced and lost",
			Outcome: requestedOutcome(),
		}, nil
	}
	phase := craft.StopStatus(true, observation)
	if phase == "stopping" {
		return CraftStopStatus{
			Phase: phase,
			Note: "cancel intent recorded and abort requested; the runtime has not confirmed the abort yet" +
				abortNote,
			Outcome: requestedOutcome(),
		}, nil
	}
	// The authoritative confirmation: observed abort on an idle session.
	if s.currentStopIntents() != nil {
		// The terminal canceled status is written only NOW — after the
		// confirmation — so the accepted stop never terminalized the run.
		if err := s.runs.Cancel(detachCtx, req.RunKey); err != nil {
			if errors.Is(err, agentruntime.ErrConflict) {
				again, getErr := s.runs.Get(detachCtx, req.RunKey)
				if getErr == nil {
					switch again.Status {
					case "succeeded":
						return CraftStopStatus{Phase: "completed",
							Note:    "run completed normally before the confirmed stop landed",
							Outcome: requestedOutcome()}, nil
					case "failed":
						return CraftStopStatus{Phase: "failed",
							Note:    "run failed before the confirmed stop landed",
							Outcome: requestedOutcome()}, nil
					case "canceled":
						// The CAS already ran (an earlier stop crashed between
						// the CAS and the confirmed marker, or that marker
						// write failed): back-fill the marker so the durable
						// answer matches the replay — otherwise every later
						// poll projects Outcome=requested next to
						// Phase=canceled forever. This branch is also the
						// repair path for a marker write that failed below.
						_, _ = s.currentStopIntents().PutStopIntent(detachCtx, req.Scope, craft.StopIntent{
							RunID: req.RunKey.RunID, Status: craft.StopConfirmed,
						})
						return confirmedReplay(), nil
					}
				}
			}
			// The terminal write failed: the intent honestly stays requested
			// — the fence is retained and a later stop retries.
			return CraftStopStatus{}, err
		}
		if _, merr := s.currentStopIntents().PutStopIntent(detachCtx, req.Scope, craft.StopIntent{
			RunID: req.RunKey.RunID, Status: craft.StopConfirmed,
		}); merr != nil {
			return CraftStopStatus{
				Phase: "canceled",
				Note: fmt.Sprintf("cancellation confirmed; the confirmed-stop marker did not persist: %v "+
					"(a stop retry back-fills it via the run's terminal canceled row)", merr), //nolint:lll // 一行诊断信息
				Outcome: craft.StopOutcome{RunID: req.RunKey.RunID, Status: craft.StopConfirmed},
			}, nil
		}
		return CraftStopStatus{
			Phase:   "canceled",
			Outcome: craft.StopOutcome{RunID: req.RunKey.RunID, Status: craft.StopConfirmed},
		}, nil
	}
	// T17 tail: phase here is "canceled" (the observed confirmation) — back
	// fill the confirmed outcome so the DTO's vocabulary stays closed (a
	// zero-value Status would be a fourth value the frontend banner cannot
	// look up once T20 serializes Outcome).
	return CraftStopStatus{Phase: phase, Outcome: craft.StopOutcome{RunID: req.RunKey.RunID, Status: craft.StopConfirmed}}, nil
}

// DelegationStatus is the read-only verification path a client polls after
// a stop answered "stopping". It never writes a cancel intent and never
// aborts — in particular a canceled subscription context (browser
// disconnect) cannot mutate anything. With the T17 (#136) stop-intent store
// wired, the poll reconstructs the stop state from the PERSISTED intent (a
// page refresh reads the same durable facts) and projects the frozen T00
// outcome alongside the honest phase.
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
	scope, err = persistedRunScope(scope, key, run)
	if err != nil {
		return CraftStopStatus{}, err
	}
	if _, err := s.authorizeRunAccess(ctx, scope, key, run, craft.TaskRead); err != nil {
		return CraftStopStatus{}, err
	}
	var task craft.Task
	hasTask := false
	if s.store != nil && taskID != "" {
		task, err = s.store.GetTask(ctx, scope, taskID)
		if err == nil {
			if err := validateControlTask(task, scope, key, taskID); err != nil {
				return CraftStopStatus{}, err
			}
			hasTask = true
		} else if !errors.Is(err, craft.ErrNotFound) {
			return CraftStopStatus{}, err
		}
	}
	requested := run.Status == "canceled"
	intentOnRecord := false
	if s.currentStopIntents() != nil {
		intent, ierr := s.currentStopIntents().GetStopIntent(ctx, scope, key.RunID)
		if ierr == nil {
			intentOnRecord = true
		}
		switch {
		case ierr == nil && intent.Status == craft.StopConfirmed:
			// The persisted confirmation answers before anything is even
			// observed: a refresh replays the durable fact.
			return CraftStopStatus{
				Phase:   "canceled",
				Outcome: craft.StopOutcome{RunID: key.RunID, Status: craft.StopConfirmed},
			}, nil
		case ierr == nil && run.Status != "canceled" && intent.Status != craft.StopUnknown:
			// The persisted intent (requested) is what tells the poll the
			// member asked to stop: the T17 run row stays nonterminal until
			// the confirmation. An UNKNOWN intent is not a stop request —
			// it must not flip the poll to stopping on its own.
			requested = true
		case !intentErrNotFound(ierr) && ierr != nil:
			// A transient store read failure: conservatively keep the run-row
			// projection (fail-closed stays "requested" only when the row
			// says canceled) and make the failure visible.
			logger.Warnf(ctx, "[CraftControl] stop intent read failed for run %s in status poll; projecting from the run row: %v", key.RunID, ierr)
		}
	}
	// The generic status poll projects a stop outcome ONLY when a stop
	// journey is on record: a durable intent row exists, or the run row is
	// itself terminally canceled. A delegation that never touched the stop
	// surface must not fabricate one — the zero Outcome is the honest
	// "no stop journey to report" (T20's serializer decides not to banner).
	stopJourney := intentOnRecord || requested
	// rowOutcome is the fallback when the intent row cannot answer: a
	// terminally canceled row replays confirmed (symmetric with Stop()'s
	// non-stop cancel replay); anything else stays requested.
	rowOutcome := craft.StopRequested
	if run.Status == "canceled" {
		rowOutcome = craft.StopConfirmed
	}
	projectOutcome := func(fallback craft.StopOutcomeStatus) craft.StopOutcome {
		if !stopJourney {
			return craft.StopOutcome{}
		}
		return s.durableStopOutcome(ctx, scope, key.RunID, fallback)
	}
	if s.store != nil && hasTask {
		if result, err := s.store.GetResult(ctx, scope, taskID); err == nil {
			return CraftStopStatus{Phase: stopPhaseForResult(result), Result: &result,
				Outcome: projectOutcome(rowOutcome)}, nil
		}
		if exec := s.currentExecutor(); exec != nil {
			observation, oerr := exec.Observe(ctx, task)
			if oerr != nil {
				// The run row's own terminal fact overtakes a stale unknown:
				// a run that settled normally (succeeded/failed) AFTER the
				// unknown was persisted — the result row may not have landed
				// — must not answer "stopping/unknown" forever (the same
				// terminal-first principle the superseded branch below pins).
				if craft.WriterRunTerminal(run.Status) && run.Status != "canceled" {
					settled := "completed"
					if run.Status == "failed" {
						settled = "failed"
					}
					return CraftStopStatus{Phase: settled,
						Outcome: func() craft.StopOutcome {
							if !stopJourney {
								return craft.StopOutcome{}
							}
							return craft.StopOutcome{RunID: key.RunID, Status: craft.StopRequested}
						}()}, nil
				}
				// A persisted unknown is honest on the poll surface too: the
				// refresh reads the durable fact instead of an error.
				if s.currentStopIntents() != nil {
					if intent, ierr := s.currentStopIntents().GetStopIntent(ctx, scope, key.RunID); ierr == nil &&
						intent.Status == craft.StopUnknown {
						return CraftStopStatus{
							Phase:   "stopping",
							Note:    fmt.Sprintf("the abort outcome is unknown and awaits reconciliation: %v", oerr),
							Outcome: craft.StopOutcome{RunID: key.RunID, Status: craft.StopUnknown},
						}, nil
					}
				}
				return CraftStopStatus{}, oerr
			}
			// A NON-canceled terminal fact overtakes a never-confirmed stop:
			// the run row settled (succeeded/failed) or the observation
			// completed without the stop's abort ever landing (the
			// authoritative opencode.Completed verdict implies exactly this
			// predicate, so that branch is subsumed here and must stay
			// BELOW it — above, a stale unknown intent row would keep
			// projecting "outcome unknown" next to Phase=completed forever).
			// The durable intent row is stale — durableStopOutcome would
			// replay it, so bypass it and project the overtaken fact:
			// requested records the honest ask, the terminal phase carries
			// what actually happened. Retiring the stale intent row itself
			// needs a store capability T20 owns (recorded there).
			if (craft.WriterRunTerminal(run.Status) && run.Status != "canceled") ||
				craft.StopIntentSuperseded(observation) {
				settled := "completed"
				if run.Status == "failed" {
					settled = "failed"
				}
				// The zero Outcome for a run that never touched the stop
				// surface (no intent row, not canceled) — the same evidence
				// gate projectOutcome applies: a superseded projection is
				// still a stop projection and must not fabricate an ask.
				if !stopJourney {
					return CraftStopStatus{Phase: settled, Outcome: craft.StopOutcome{}}, nil
				}
				return CraftStopStatus{Phase: settled,
					Outcome: craft.StopOutcome{RunID: key.RunID, Status: craft.StopRequested}}, nil
			}
			phase := craft.StopStatus(requested, observation)
			// The requested premise passes through to the outcome mapping
			// (same-source with StopStatus): an abort by any OTHER mechanism
			// can never fall back to confirmed here, and StopStatus never
			// reports "canceled" without requested — no phase clamp needed.
			fallback := craft.StopIntentOutcome(requested, observation)
			return CraftStopStatus{Phase: phase,
				Outcome: projectOutcome(fallback)}, nil
		}
	}
	if run.Status == "canceled" {
		// The row's terminal fact answers even without an observable
		// delegation (no result, no task row, no executor): replay it —
		// symmetric with Stop()'s non-stop cancel replay — instead of an
		// empty-observation "stopping".
		return CraftStopStatus{Phase: "canceled",
			Outcome: projectOutcome(craft.StopConfirmed)}, nil
	}
	if craft.WriterRunTerminal(run.Status) {
		// The superseded tail: the run row settled normally while the stop
		// never confirmed — same bypass projection as the observe block,
		// gated the same way: a run that never touched the stop surface
		// projects no stop outcome at all.
		settled := "completed"
		if run.Status == "failed" {
			settled = "failed"
		}
		if !stopJourney {
			return CraftStopStatus{Phase: settled, Outcome: craft.StopOutcome{}}, nil
		}
		return CraftStopStatus{Phase: settled,
			Outcome: craft.StopOutcome{RunID: key.RunID, Status: craft.StopRequested}}, nil
	}
	return CraftStopStatus{Phase: craft.StopStatus(requested, craft.Observation{}),
		Outcome: projectOutcome(craft.StopRequested)}, nil
}
