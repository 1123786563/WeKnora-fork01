package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/opencode"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"gorm.io/gorm"
)

// Existing waiting reasons the craft recovery program transitions to. They
// are deliberately not new vocabulary: sandbox-class problems reuse the
// recovery hook's "sandbox_unavailable" reason, and an undeterminable
// sub-execution outcome parks on the tool call id — the same pending
// identity the durable tool journal uses for unknown outcomes, which the
// existing decision endpoint already understands.
const (
	craftRecoveryWaitSandbox = "sandbox_unavailable"
	// craftRecoveryDefaultInterval spaces observation polls of a remote
	// sub-execution that is still running.
	craftRecoveryDefaultInterval = 500 * time.Millisecond
	// craftRecoveryDefaultTimeout bounds one snapshot observation call.
	craftRecoveryDefaultTimeout = 15 * time.Second
	// craftRecoveryObserveFallbackBudget bounds the observe loop only when
	// the delegation carries no deadline AND the caller's context has none
	// (C04 review nit-2; production chains always carry an absolute
	// deadline, so this is a defensive backstop, never the common path).
	craftRecoveryObserveFallbackBudget = 15 * time.Minute
)

// CraftRecoveryScopeResolver rebuilds the execution authority of a run from
// durable state after a process failure: tenant, session and the owning
// user are re-derived from the persisted run and session rows — never from
// the delegation's stored JSON, which a stale worker wrote.
type CraftRecoveryScopeResolver interface {
	ResolveScope(ctx context.Context, key agentruntime.RunKey) (craft.Scope, error)
}

// craftRunScopeQuery is the production resolver: it re-reads the run row and
// the session's engine configuration, so a session that was deleted, moved
// to another engine, or whose run already terminated cannot authorize a
// recovery reconciliation.
type craftRunScopeQuery struct{ db *gorm.DB }

// CraftRunScopeQuery assembles the production scope resolver over the
// migrated business database.
func CraftRunScopeQuery(db *gorm.DB) CraftRecoveryScopeResolver {
	return &craftRunScopeQuery{db: db}
}

func (q *craftRunScopeQuery) ResolveScope(ctx context.Context, key agentruntime.RunKey) (craft.Scope, error) {
	if q == nil || q.db == nil {
		return craft.Scope{}, fmt.Errorf("%w: scope resolver requires the business database", craft.ErrInvalidInput)
	}
	var run struct{ SessionID, OwnerID, Status string }
	err := q.db.WithContext(ctx).Table("agent_runs").
		Select("session_id, owner_id, status").
		Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Take(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.Scope{}, fmt.Errorf("%w: run %s", craft.ErrNotFound, key.RunID)
	}
	if err != nil {
		return craft.Scope{}, err
	}
	switch run.Status {
	case "succeeded", "failed", "canceled":
		return craft.Scope{}, fmt.Errorf("%w: run %s is already %s; nothing to reconcile",
			craft.ErrConflict, key.RunID, run.Status)
	}
	var session struct{ UserID, EngineType string }
	err = q.db.WithContext(ctx).Table("sessions").
		Select("user_id, engine_type").
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", key.TenantID, run.SessionID).
		Take(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.Scope{}, fmt.Errorf("%w: session %s of run %s", craft.ErrNotFound, run.SessionID, key.RunID)
	}
	if err != nil {
		return craft.Scope{}, err
	}
	if session.EngineType != "trpc" {
		return craft.Scope{}, fmt.Errorf("%w: session %s no longer runs the trpc engine",
			craft.ErrForbidden, run.SessionID)
	}
	if session.UserID != run.OwnerID {
		return craft.Scope{}, fmt.Errorf("%w: session %s ownership changed", craft.ErrForbidden, run.SessionID)
	}
	return craft.Scope{TenantID: key.TenantID, UserID: run.OwnerID, SessionID: run.SessionID}, nil
}

// CraftRecoveryConfig carries the deployment facts recovery compares
// against. RuntimeDigest is the digest of the runtime the current process is
// configured to trust (W06 stamps it into workspaces at provisioning); an
// empty value means the deployment does not pin digests.
type CraftRecoveryConfig struct {
	RuntimeDigest    string
	ObserveInterval  time.Duration
	ObserveCallLimit time.Duration
}

func (c CraftRecoveryConfig) interval() time.Duration {
	if c.ObserveInterval > 0 {
		return c.ObserveInterval
	}
	return craftRecoveryDefaultInterval
}

func (c CraftRecoveryConfig) callLimit() time.Duration {
	if c.ObserveCallLimit > 0 {
		return c.ObserveCallLimit
	}
	return craftRecoveryDefaultTimeout
}

// CraftRecovery is the post-failure reconciliation program for delegated
// craft sub-executions. After a worker process dies mid-delegation, the
// takeover worker invokes Reconcile under its own new fence: authority is
// rebuilt from durable state, an already-stored result is only re-applied,
// and an outcome that cannot be proven waits durably. The program exposes no
// resubmission branch at all — a prompt that may already have been accepted
// is never re-POSTed, not by route, not by retry, not by deadline.
type CraftRecovery struct {
	store    craft.Store
	executor craft.Executor
	runs     CraftRunController
	scopes   CraftRecoveryScopeResolver
	cfg      CraftRecoveryConfig
}

// NewCraftRecovery validates and assembles the recovery program. runs and
// scopes are required: without a durable park surface and an authority
// resolver a reconciliation could neither wait nor trust its scope.
func NewCraftRecovery(
	store craft.Store,
	executor craft.Executor,
	runs CraftRunController,
	scopes CraftRecoveryScopeResolver,
	cfg CraftRecoveryConfig,
) (*CraftRecovery, error) {
	if store == nil || executor == nil {
		return nil, fmt.Errorf("%w: recovery requires a store and an executor", craft.ErrInvalidInput)
	}
	if runs == nil || scopes == nil {
		return nil, fmt.Errorf("%w: recovery requires a run controller and a scope resolver", craft.ErrInvalidInput)
	}
	return &CraftRecovery{store: store, executor: executor, runs: runs, scopes: scopes, cfg: cfg}, nil
}

// Reconcile settles one delegated sub-execution after a process failure.
//
// Order of operations (product constraint "会话失联先核对，无法确定时持久等待"):
//  1. rebuild the execution authority from the durable run/session rows;
//  2. re-authorize the delegation's resources through the live workspace
//     binding — even a reuse of a stored result must pass this check;
//  3. a stored terminal result is returned as-is (apply_result only);
//  4. otherwise verify sandbox generation, OpenCode session, runtime digest
//     and prompt message id, then let craft.RecoveryRoute decide: collect
//     only on the R04 full-snapshot Completed predicate, observe a verified
//     still-running remote under the delegation's absolute deadline, and
//     wait (durably, on an existing waiting reason) in every other case.
//
// The returned error is craft.ErrUnknown for a durable unknown-outcome wait
// and ErrSandboxUnavailable for a sandbox-class wait; both parks have
// already been persisted under the takeover fence.
func (r *CraftRecovery) Reconcile(
	ctx context.Context, fence agentruntime.Fence, taskID string,
) (craft.Result, error) {
	if r == nil {
		return craft.Result{}, fmt.Errorf("%w: recovery program is not assembled", craft.ErrInvalidInput)
	}
	if fence.TenantID == 0 || fence.RunID == "" || fence.Owner == "" || fence.Epoch <= 0 || taskID == "" {
		return craft.Result{}, fmt.Errorf("%w: reconciliation requires a live fence and task id", craft.ErrInvalidInput)
	}

	// 1. Rebuild authority: the scope comes from durable state under the
	// current authentication/config resolvers, never from the stored task.
	scope, err := r.scopes.ResolveScope(ctx, fence.RunKey)
	if err != nil {
		return craft.Result{}, err
	}

	task, err := r.store.GetTask(ctx, scope, taskID)
	if err != nil {
		return craft.Result{}, err
	}
	if task.Fence.TenantID != fence.TenantID || task.Fence.RunID != fence.RunID {
		return craft.Result{}, fmt.Errorf("%w: delegation %s belongs to another run", craft.ErrForbidden, task.ID)
	}

	// 2. Re-authorize resources: the binding must still exist for the
	// rebuilt scope and still be the delegation's workspace.
	workspace, err := r.store.GetWorkspace(ctx, scope)
	if err != nil {
		return craft.Result{}, err
	}
	if workspace.ID != task.WorkspaceID {
		return craft.Result{}, fmt.Errorf("%w: task workspace %s is not bound to this session",
			craft.ErrForbidden, task.WorkspaceID)
	}

	// 3. A stored terminal result is only re-applied — after the
	// re-authorization above has passed.
	if result, rerr := r.store.GetResult(ctx, scope, taskID); rerr == nil {
		return result, nil
	} else if !errors.Is(rerr, craft.ErrNotFound) {
		return craft.Result{}, rerr
	}

	// 4. Verify the runtime facts before anything is observed through them.
	facts := craft.RecoveryFacts{
		VersionCompatible:  r.runtimeCompatible(workspace),
		WorkspaceAvailable: craftWorkspaceAvailable(workspace),
	}
	if !facts.VersionCompatible {
		return r.parkSandbox(ctx, fence, task, fmt.Sprintf(
			"workspace runtime digest %q no longer matches the configured runtime digest %q; observation through a changed image is not trustworthy",
			workspace.RuntimeDigest, r.cfg.RuntimeDigest))
	}
	if !facts.WorkspaceAvailable {
		return r.parkSandbox(ctx, fence, task, fmt.Sprintf(
			"workspace binding is unavailable for delegation %s (sandbox=%q generation=%q opencode session=%q); an empty session is never continued",
			task.ID, workspace.SandboxID, workspace.Generation, workspace.OpenCodeSessionID))
	}
	if task.PromptMessageID == "" {
		return r.parkUnknown(ctx, fence, task,
			"delegation has no persisted prompt message id; its dispatch state cannot be reconstructed")
	}

	deadline := task.Deadline
	// C04 review nit-2 (landed with the C05 assembly): a delegation without
	// a persisted deadline observed under a context that also carries no
	// deadline could poll a verified-running remote forever. That extreme
	// combination gets a synthetic observation budget so it parks durably
	// instead; production delegation chains always carry an absolute
	// deadline, and the worker hook passes a budgeted context on top.
	syntheticDeadline := false
	if deadline.IsZero() {
		if ctxDeadline, ok := ctx.Deadline(); ok {
			deadline = ctxDeadline
		} else {
			deadline = time.Now().Add(craftRecoveryObserveFallbackBudget)
			syntheticDeadline = true
		}
	}
	for {
		obs, obsErr := r.observeOnce(ctx, task)
		if obsErr != nil {
			// A runtime that cannot answer (missing session, unsupported
			// dial, revoked access) is a sandbox-class wait; a snapshot that
			// simply failed to prove anything is an unknown-outcome wait.
			if errors.Is(obsErr, craft.ErrUnsupported) || errors.Is(obsErr, craft.ErrForbidden) ||
				errors.Is(obsErr, craft.ErrNotFound) {
				return r.parkSandbox(ctx, fence, task, obsErr.Error())
			}
			return r.parkUnknown(ctx, fence, task, obsErr.Error())
		}
		facts.ExactCompleted = opencode.Completed(obs)
		facts.RemoteRunning = !obs.Idle || obs.PendingTool

		switch craft.RecoveryRoute(facts) {
		case craft.RecoveryRouteCollect:
			// ExactCompleted is only true through opencode.Completed — the
			// R04 full-snapshot predicate. classifyCraftObservation maps the
			// same predicate onto the persisted outcome.
			status, summary, _ := classifyCraftObservation(obs)
			return r.settle(ctx, fence, task, status, summary)
		case craft.RecoveryRouteObserve:
			// The remote sub-execution is verified still running: keep
			// observing under the delegation's absolute deadline. The
			// deadline is never reset by a restart.
			if !deadline.IsZero() && !time.Now().Before(deadline) {
				if syntheticDeadline {
					return r.parkUnknown(ctx, fence, task, fmt.Sprintf(
						"delegation carried no deadline and the context none either; the %s observation budget exhausted while the sub-execution is still running",
						craftRecoveryObserveFallbackBudget))
				}
				return r.parkUnknown(ctx, fence, task, fmt.Sprintf(
					"delegation deadline %s reached while the sub-execution is still running; the outcome cannot be determined",
					deadline.Format(time.RFC3339)))
			}
			if werr := craftRecoverySleep(ctx, r.cfg.interval()); werr != nil {
				// C04 review nit-3 (comment, landed with the C05 assembly): the
				// context ended (worker shutdown or lease drop) between two
				// observations. Nothing was learned that could justify a
				// durable park — no fact was observed this round — so the error
				// returns bare, the run stays non-terminal, and the expiring
				// lease hands it to the next takeover. Never park here and
				// never treat cancellation as an outcome.
				return craft.Result{}, werr
			}
		default: // craft.RecoveryRouteWait
			// The runtime is healthy but proves neither completion nor an
			// active round. Definitive terminal facts still settle; anything
			// else parks on the existing unknown-outcome reason.
			if status, summary, settled := classifyCraftObservation(obs); settled {
				return r.settle(ctx, fence, task, status, summary)
			}
			return r.parkUnknown(ctx, fence, task, fmt.Sprintf(
				"sub-execution not verifiably terminal (finish=%q completed=%v idle=%v pending_tool=%v); the prompt may already have been accepted",
				obs.Finish, obs.Completed, obs.Idle, obs.PendingTool))
		}
	}
}

// observeOnce performs one bounded snapshot observation. Observation is the
// only remote interaction of the recovery program.
func (r *CraftRecovery) observeOnce(ctx context.Context, task craft.Task) (craft.Observation, error) {
	callCtx, cancel := context.WithTimeout(ctx, r.cfg.callLimit())
	defer cancel()
	return r.executor.Observe(callCtx, task)
}

// runtimeCompatible compares the digest stamped into the workspace binding
// with the digest the current process is configured to trust. A deployment
// that pins no digest accepts any recorded one; a workspace without a
// recorded digest proves nothing and is incompatible.
func (r *CraftRecovery) runtimeCompatible(workspace craft.Workspace) bool {
	if workspace.RuntimeDigest == "" {
		return false
	}
	if r.cfg.RuntimeDigest == "" {
		return true
	}
	return workspace.RuntimeDigest == r.cfg.RuntimeDigest
}

// craftWorkspaceAvailable verifies the durable workspace binding still
// carries a live sandbox generation and a bound OpenCode session.
func craftWorkspaceAvailable(workspace craft.Workspace) bool {
	return workspace.SandboxID != "" && workspace.Generation != "" && workspace.OpenCodeSessionID != ""
}

// settle persists one definitive outcome under the takeover worker's live
// fence — the new epoch's write-back is exactly what the fence rules allow.
func (r *CraftRecovery) settle(
	ctx context.Context, fence agentruntime.Fence, task craft.Task, status, summary string,
) (craft.Result, error) {
	result := craft.Result{TaskID: task.ID, Status: status, Summary: boundCraftSummary(summary)}
	if err := r.store.SaveResult(ctx, fence, result); err != nil {
		return craft.Result{}, err
	}
	return result, nil
}

// parkSandbox durably parks the run on the existing sandbox waiting reason
// and reports ErrSandboxUnavailable — mirroring the recovery hook contract.
func (r *CraftRecovery) parkSandbox(
	ctx context.Context, fence agentruntime.Fence, task craft.Task, reason string,
) (craft.Result, error) {
	err := fmt.Errorf("%w: craft delegation %s: %s", ErrSandboxUnavailable, task.ID, reason)
	if parkErr := r.park(ctx, fence, task, craftRecoveryWaitSandbox); parkErr != nil {
		return craft.Result{}, errors.Join(err, parkErr)
	}
	return craft.Result{}, err
}

// parkUnknown durably parks the run on the existing unknown-outcome pending
// identity — the tool call id, the same pending the durable tool journal
// uses — so the existing decision endpoint (manual retry with the
// side-effect risk note) can resolve it. craft.ErrUnknown is never
// persisted as a result.
func (r *CraftRecovery) parkUnknown(
	ctx context.Context, fence agentruntime.Fence, task craft.Task, reason string,
) (craft.Result, error) {
	err := fmt.Errorf("%w: craft delegation %s: %s", craft.ErrUnknown, task.ID, reason)
	if parkErr := r.park(ctx, fence, task, task.ToolCallID); parkErr != nil {
		return craft.Result{}, errors.Join(err, parkErr)
	}
	return craft.Result{}, err
}

// park durably records the pending marker and then the waiting state. The
// order matters: parking the run at waiting_user releases the lease in the
// same transaction, so the tool-call marker (which needs the live fence)
// must be written first. The marker is best effort — a row already unknown
// or dispatching keeps its own stronger linkage — while the run park is
// authoritative.
func (r *CraftRecovery) park(
	ctx context.Context, fence agentruntime.Fence, task craft.Task, pending string,
) error {
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if task.ToolCallID != "" && pending != craftRecoveryWaitSandbox {
		if store, ok := r.runs.(interface{ Store() agentruntime.RunStore }); ok {
			if parker, ok := store.Store().(agentruntime.ToolPreflightParker); ok {
				if merr := parker.ParkToolPreflightWait(persistCtx, fence, task.ToolCallID, pending, task.WorkspaceID); merr != nil {
					logger.Warnf(persistCtx,
						"[CraftRecovery] pending marker not written for call %s: %v", task.ToolCallID, merr)
				}
			}
		}
	}
	return r.runs.WaitForDecision(persistCtx, fence, pending)
}

// craftRecoverySleep waits one poll interval or until the context ends.
func craftRecoverySleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
