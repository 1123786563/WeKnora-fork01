package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
)

const (
	// snapshotVerifyBudget bounds the final snapshot verification reads
	// independently of the execution deadline.
	snapshotVerifyBudget = 15 * time.Second
	// defaultExecutionWindow is the fallback bound when the delegation
	// carries no deadline, so an unbounded stream can never pin a worker.
	defaultExecutionWindow = 10 * time.Minute
	// maxTerminalToolEvents caps how many terminal tool states are mirrored
	// into Craft events for one delegation.
	maxTerminalToolEvents = 64
)

// emitFunc writes one Craft event payload into the existing RunEvent
// stream. The executor never talks to the browser; the service layer owns
// sequencing and delivery.
type emitFunc func(context.Context, craft.Task, string, json.RawMessage) error

// Executor implements craft.Executor on top of the pinned OpenCode client.
// It submits each delegation prompt exactly once, normalizes the live
// sub-event stream, and settles the outcome only from a verified snapshot.
type Executor struct {
	client *Client
	store  craft.Store
	emit   emitFunc
}

var _ craft.Executor = (*Executor)(nil)

// NewExecutor assembles the Craft executor. emit may be nil; event
// writing failures never change an execution outcome.
func NewExecutor(client *Client, store craft.Store,
	emit func(context.Context, craft.Task, string, json.RawMessage) error,
) craft.Executor {
	return &Executor{client: client, store: store, emit: emit}
}

func validateExecutionTask(task craft.Task) error {
	if task.Scope.TenantID == 0 || task.Scope.UserID == "" || task.Scope.SessionID == "" ||
		task.ToolCallID == "" || task.WorkspaceID == "" || task.Prompt == "" ||
		task.Fence.TenantID == 0 || task.Fence.RunID == "" || task.Fence.Owner == "" || task.Fence.Epoch <= 0 {
		return fmt.Errorf("%w: incomplete delegation task", craft.ErrInvalidInput)
	}
	return nil
}

// boundWorkspace resolves and checks the session workspace binding. All
// access goes through the Store port; the executor never joins the agent
// tables itself, so dangling delegation references cannot reach it.
func (e *Executor) boundWorkspace(ctx context.Context, task craft.Task) (craft.Workspace, error) {
	workspace, err := e.store.GetWorkspace(ctx, task.Scope)
	if err != nil {
		return craft.Workspace{}, err
	}
	if workspace.ID != task.WorkspaceID {
		return craft.Workspace{}, fmt.Errorf("%w: task workspace %s is not bound to this session",
			craft.ErrForbidden, task.WorkspaceID)
	}
	return workspace, nil
}

// executionRun carries the per-Execute mutable state.
type executionRun struct {
	task        craft.Task
	sessionID   string
	promptID    string
	state       *subState
	startedSent bool
}

// Execute runs one delegated sub-execution with the mandated commit
// ordering: assign the prompt message id, persist the delegation through
// PrepareTask, subscribe to /event, then submit the prompt to the bound
// session. The prompt is submitted at most once per delegation id, even
// across retries; a failed or lost POST is resolved by observing the
// runtime instead of re-submitting.
func (e *Executor) Execute(ctx context.Context, task craft.Task) (craft.Result, error) {
	if e.client == nil || e.store == nil {
		return craft.Result{}, fmt.Errorf("%w: executor is missing its client or store", craft.ErrInvalidInput)
	}
	if err := validateExecutionTask(task); err != nil {
		return craft.Result{}, err
	}
	workspace, err := e.boundWorkspace(ctx, task)
	if err != nil {
		return craft.Result{}, err
	}
	if task.PromptMessageID == "" {
		if task.PromptMessageID, err = NewMessageID(); err != nil {
			return craft.Result{}, err
		}
	}
	// Persist before dispatch: an interrupted execution must never depend
	// on in-memory state to be reconstructed.
	prepared, err := e.store.PrepareTask(ctx, task)
	if err != nil {
		return craft.Result{}, err
	}
	run := &executionRun{task: prepared, sessionID: workspace.OpenCodeSessionID, promptID: prepared.PromptMessageID}
	if run.promptID == "" {
		return craft.Result{}, fmt.Errorf("%w: stored delegation has no prompt message id", craft.ErrInvalidInput)
	}
	// A stored id from a previous 48-bit wrap window must not be re-submitted.
	if !messageIDReusableAt(run.promptID, time.Now()) {
		return e.unresolved(run, nil, "persisted prompt message id %q is outside the current message id wrap window", run.promptID)
	}
	if run.sessionID == "" {
		e.emitEvent(ctx, prepared, "workspace.unavailable", map[string]any{"workspace_id": task.WorkspaceID})
		return e.finish(ctx, prepared, "failed", "workspace has no bound OpenCode session")
	}

	runCtx := ctx
	cancel := context.CancelFunc(func() {})
	deadline := prepared.Deadline
	if deadline.IsZero() {
		runCtx, cancel = context.WithTimeout(ctx, defaultExecutionWindow)
	} else if time.Now().Before(deadline) {
		runCtx, cancel = context.WithDeadline(ctx, deadline)
	} else {
		return e.unresolved(run, nil, "delegation deadline %s has already passed", deadline.Format(time.RFC3339))
	}
	defer cancel()

	// Retry check: if the runtime already holds this exact prompt message
	// id, the delegation was accepted earlier and must not be submitted
	// again. A FAILED pre-flight read can neither prove acceptance nor the
	// lack of it — re-submitting a side-effecting prompt against an
	// unreadable runtime would be a blind retry (CFT-S02-T015), so the
	// execution stays unknown and reconcilable instead.
	alreadyAccepted := false
	if pre, err := snapshotObservation(runCtx, e.client, run.sessionID, run.promptID); err == nil {
		alreadyAccepted = pre.promptSeen
	} else {
		return e.unresolved(run, nil, "pre-flight snapshot failed, refusing to re-submit the prompt: %v", err)
	}

	run.state = newSubState(run.sessionID, run.promptID, func(kind, partID string) {
		e.emitEvent(ctx, prepared, "interaction.pending", map[string]any{"kind": kind, "part_id": partID})
	})

	// Subscribe before prompting so no sub-event of this round can be
	// missed. A failed subscription never falls back to a blind prompt.
	stream, subscribeErr := e.client.Events(runCtx)
	if stream != nil {
		defer stream.Close()
	}
	var promptErr error
	if subscribeErr != nil {
		promptErr = fmt.Errorf("event subscription failed: %w", subscribeErr)
	} else if !alreadyAccepted {
		promptErr = e.client.Prompt(runCtx, run.sessionID, run.promptID, prepared.Prompt)
	}
	if promptErr == nil {
		run.startedSent = true
		e.emitEvent(ctx, prepared, "delegation.started", map[string]any{"prompt_message_id": run.promptID})
	}
	if stream != nil {
		e.consume(runCtx, stream, run.state)
	}
	return e.settle(ctx, run, promptErr)
}

// consume reads the SSE stream until a terminal signal, the context ends,
// or the stream breaks. Every break path funnels into the same snapshot
// verification, so a lost stream is never mistaken for an outcome.
func (e *Executor) consume(ctx context.Context, stream io.ReadCloser, state *subState) {
	_ = scanEventFrames(stream, func(frame eventFrame) bool {
		state.apply(frame)
		select {
		case <-ctx.Done():
			return false
		default:
			return !state.terminalSignal()
		}
	})
}

// settle performs the authoritative snapshot verification and classifies
// the delegation outcome. Only the Completed predicate together with a
// successfully collected snapshot yields succeeded.
func (e *Executor) settle(ctx context.Context, run *executionRun, promptErr error) (craft.Result, error) {
	verifyCtx, cancel := context.WithTimeout(ctx, snapshotVerifyBudget)
	defer cancel()
	snap, err := snapshotObservation(verifyCtx, e.client, run.sessionID, run.promptID)
	if err != nil {
		return e.unresolved(run, run.state, "snapshot verification failed: %v", err)
	}
	obs := snap.obs
	if run.state != nil {
		obs.Aborted = obs.Aborted || run.state.aborted
	}
	if !run.startedSent && snap.promptSeen {
		run.startedSent = true
		e.emitEvent(ctx, run.task, "delegation.started", map[string]any{"prompt_message_id": run.promptID})
	}
	if promptErr != nil && !snap.promptSeen && obs.Idle {
		// The runtime never accepted the prompt and is not working on
		// anything for this session: a definitive rejection, not an
		// unknown outcome.
		return e.finish(ctx, run.task, "failed", fmt.Sprintf("prompt was not accepted: %v", promptErr))
	}
	if Completed(obs) && snap.assistant != nil {
		e.emitTerminal(ctx, run, snap)
		return e.finish(ctx, run.task, "succeeded", mergedText(snap.assistant.Parts))
	}
	if obs.Aborted {
		e.emitTerminal(ctx, run, snap)
		return e.finish(ctx, run.task, "canceled", "assistant execution aborted (MessageAbortedError)")
	}
	if snap.errorName != "" {
		e.emitTerminal(ctx, run, snap)
		return e.finish(ctx, run.task, "failed", fmt.Sprintf("assistant failed: %s", snap.errorName))
	}
	if snap.assistant != nil && snap.assistant.CompletedAt > 0 && snap.assistant.Finish != "stop" {
		e.emitTerminal(ctx, run, snap)
		return e.finish(ctx, run.task, "failed", fmt.Sprintf("assistant ended with finish %q", snap.assistant.Finish))
	}
	if promptErr != nil && !snap.promptSeen {
		return e.unresolved(run, run.state, "prompt acceptance undetermined: %v", promptErr)
	}
	assistantFinish, completedAt := "", int64(0)
	if snap.assistant != nil {
		assistantFinish, completedAt = snap.assistant.Finish, snap.assistant.CompletedAt
	}
	return e.unresolved(run, run.state,
		"sub-execution not verifiably terminal (finish=%q completed=%d idle=%v pending_tool=%v status=%q)",
		assistantFinish, completedAt, obs.Idle, obs.PendingTool, snap.status)
}

// emitTerminal mirrors the finished round into Craft events: merged text
// (bounded) and terminal tool states. Tool payloads carry only structural
// fields; raw tool inputs and outputs stay inside the sandbox, so
// sensitive command arguments and model credentials never leave the
// runtime boundary.
func (e *Executor) emitTerminal(ctx context.Context, run *executionRun, snap snapshot) {
	text := ""
	if snap.assistant != nil {
		text = mergedText(snap.assistant.Parts)
	}
	if text == "" && run.state != nil {
		text = run.state.mergedText()
	}
	if text != "" {
		e.emitEvent(ctx, run.task, "delegation.text", map[string]any{"text": boundString(text, maxEmitTextBytes)})
	}
	if run.state != nil {
		tools := run.state.terminalTools()
		if len(tools) > maxTerminalToolEvents {
			tools = tools[:maxTerminalToolEvents]
		}
		for _, tool := range tools {
			e.emitEvent(ctx, run.task, "delegation.tool", map[string]any{
				"tool": tool.tool, "call_id": tool.callID, "part_id": tool.partID, "status": tool.status,
			})
		}
	}
}

// finish persists a definitive terminal outcome and reports it.
func (e *Executor) finish(ctx context.Context, task craft.Task, status, summary string) (craft.Result, error) {
	result := craft.Result{TaskID: task.ID, Status: status, Summary: boundString(summary, maxSummaryBytes)}
	if err := e.store.SaveResult(ctx, task.Fence, result); err != nil {
		return result, err
	}
	e.emitEvent(ctx, task, "delegation.finished", map[string]any{"status": status})
	return result, nil
}

// unresolved reports an undeterminable outcome. The delegation stays
// without a stored result so a later Observe or a recovered Execute can
// settle it; the caller keeps the main tool call pending via ErrUnknown.
func (e *Executor) unresolved(run *executionRun, state *subState, format string, args ...any) (craft.Result, error) {
	reason := fmt.Sprintf(format, args...)
	result := craft.Result{TaskID: run.task.ID, Status: "unknown", Summary: boundString(reason, maxSummaryBytes)}
	if state != nil && state.liveErrorName != "" {
		result.Summary = boundString(result.Summary+"; last runtime error: "+state.liveErrorName, maxSummaryBytes)
	}
	return result, fmt.Errorf("%w: %s", craft.ErrUnknown, result.Summary)
}

// Observe reads the runtime snapshot for one delegation: session status,
// the message list and the parts together. It never prompts, never
// mutates state and never emits events.
func (e *Executor) Observe(ctx context.Context, task craft.Task) (craft.Observation, error) {
	if e.client == nil || e.store == nil {
		return craft.Observation{}, fmt.Errorf("%w: executor is missing its client or store", craft.ErrInvalidInput)
	}
	if task.PromptMessageID == "" {
		return craft.Observation{}, fmt.Errorf("%w: observe requires the persisted prompt message id", craft.ErrInvalidInput)
	}
	workspace, err := e.boundWorkspace(ctx, task)
	if err != nil {
		return craft.Observation{}, err
	}
	if workspace.OpenCodeSessionID == "" {
		return craft.Observation{}, fmt.Errorf("%w: workspace has no bound OpenCode session", craft.ErrUnsupported)
	}
	snap, err := snapshotObservation(ctx, e.client, workspace.OpenCodeSessionID, task.PromptMessageID)
	if err != nil {
		return craft.Observation{}, err
	}
	return snap.obs, nil
}

// Abort aborts the bound session and, when the snapshot confirms it,
// records the canceled outcome. An abort that raced a completed round is
// left to the completed classification.
func (e *Executor) Abort(ctx context.Context, task craft.Task) error {
	if e.client == nil || e.store == nil {
		return fmt.Errorf("%w: executor is missing its client or store", craft.ErrInvalidInput)
	}
	workspace, err := e.boundWorkspace(ctx, task)
	if err != nil {
		return err
	}
	if workspace.OpenCodeSessionID == "" {
		return fmt.Errorf("%w: workspace has no bound OpenCode session", craft.ErrUnsupported)
	}
	if err := e.client.Abort(ctx, workspace.OpenCodeSessionID); err != nil {
		return err
	}
	if task.PromptMessageID == "" {
		return nil
	}
	verifyCtx, cancel := context.WithTimeout(ctx, snapshotVerifyBudget)
	defer cancel()
	snap, err := snapshotObservation(verifyCtx, e.client, workspace.OpenCodeSessionID, task.PromptMessageID)
	if err != nil || Completed(snap.obs) {
		// The abort request was accepted; the outcome is settled by a
		// later observe when the snapshot cannot confirm it yet.
		return nil
	}
	if snap.obs.Aborted {
		result := craft.Result{
			TaskID: task.ID, Status: "canceled",
			Summary: "assistant execution aborted (MessageAbortedError)",
		}
		if err := e.store.SaveResult(ctx, task.Fence, result); err != nil {
			return err
		}
		e.emitEvent(ctx, task, "delegation.finished", map[string]any{"status": "canceled"})
	}
	return nil
}

func (e *Executor) emitEvent(ctx context.Context, task craft.Task, kind string, data any) {
	if e.emit == nil {
		return
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return
	}
	// Event writing is telemetry: failures must not change outcomes.
	_ = e.emit(ctx, task, kind, raw)
}
