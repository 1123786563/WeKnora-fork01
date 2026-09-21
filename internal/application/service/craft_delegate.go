package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/metrics"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/opencode"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/tools"
	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
)

// craftSummaryLimit bounds a delegation summary persisted by this service.
const craftSummaryLimit = 4 << 10

// CraftDelegation bundles the durable Craft store with the delegation
// service for agent capability assembly. A nil assembly (the default when
// the container does not wire Craft) leaves every session — builtin or tRPC
// — on the unchanged builtin tool set.
type CraftDelegation struct {
	Store    craft.Store
	Delegate *CraftDelegateService
}

// NewCraftDelegation validates and assembles the Craft delegation surface.
func NewCraftDelegation(store craft.Store, executor craft.Executor) (*CraftDelegation, error) {
	if store == nil || executor == nil {
		return nil, errors.New("craft: delegation assembly requires a store and an executor")
	}
	return &CraftDelegation{Store: store, Delegate: NewCraftDelegateService(store, executor)}, nil
}

// CraftDelegateService drives one durable delegation: it reuses an already
// stored result, settles a previously dispatched delegation strictly through
// observation, and takes a fresh delegation through PrepareTask before the
// executor submits it exactly once.
type CraftDelegateService struct {
	store    craft.Store
	executor craft.Executor
	guard    CraftDispatchGuard
}

// CraftDispatchGuard refuses a NEW delegation dispatch while the session's
// resources are being cleaned up (the O03 lifecycle tombstone mark). nil
// keeps the unwired behavior (no refusal) for assemblies without the
// lifecycle service — reuse of a stored result and pure observation are
// never guarded because they dispatch nothing.
type CraftDispatchGuard interface {
	GuardDispatch(ctx context.Context, tenantID uint64, sessionID string) error
}

// SetDispatchGuard installs the lifecycle dispatch guard (O03 integration
// wiring; see internal/container).
func (s *CraftDelegateService) SetDispatchGuard(g CraftDispatchGuard) { s.guard = g }

// NewCraftDelegateService assembles the delegation service.
func NewCraftDelegateService(store craft.Store, executor craft.Executor) *CraftDelegateService {
	return &CraftDelegateService{store: store, executor: executor}
}

// craftDelegationID derives the durable delegation identity from the run
// and tool call it serves: stable across retries, unique per (run, call).
func craftDelegationID(task craft.Task) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("craft-delegation/%d/%s/%s",
		task.Fence.TenantID, task.Fence.RunID, task.ToolCallID)))
	return "dlg_" + hex.EncodeToString(sum[:16])
}

func validateDelegateTask(task craft.Task) error {
	if task.Scope.TenantID == 0 || task.Scope.UserID == "" || task.Scope.SessionID == "" ||
		task.ToolCallID == "" || task.WorkspaceID == "" || task.Prompt == "" ||
		task.Fence.TenantID == 0 || task.Fence.RunID == "" || task.Fence.Owner == "" || task.Fence.Epoch <= 0 {
		return fmt.Errorf("%w: incomplete delegation request", craft.ErrInvalidInput)
	}
	if task.RequestHash == "" {
		return fmt.Errorf("%w: delegation requires a request hash", craft.ErrInvalidInput)
	}
	return nil
}

// Delegate resolves one delegation durably.
//
// Order of operations:
//   - a stored result is returned as-is (retry after crash, no re-execution);
//   - a previously prepared delegation with no stored result is in the
//     dispatching-unknown state: it is settled only by observing the runtime,
//     never by re-submitting the prompt (the executor guarantees the POST
//     happens at most once);
//   - a fresh delegation goes through PrepareTask (fenced, idempotent) before
//     the executor runs it.
//
// craft.ErrUnknown is the only pending signal returned to the caller; it is
// never persisted as a result.
func (s *CraftDelegateService) Delegate(ctx context.Context, task craft.Task) (craft.Result, error) {
	if s == nil || s.store == nil || s.executor == nil {
		return craft.Result{}, fmt.Errorf("%w: delegation service is not assembled", craft.ErrInvalidInput)
	}
	if err := validateDelegateTask(task); err != nil {
		return craft.Result{}, err
	}
	if task.ID == "" {
		task.ID = craftDelegationID(task)
	}

	// 1. Reuse an already stored terminal outcome.
	result, err := s.store.GetResult(ctx, task.Scope, task.ID)
	if err == nil {
		return result, nil
	}
	if !errors.Is(err, craft.ErrNotFound) {
		return craft.Result{}, err
	}

	// 2. Previously prepared, outcome not stored: dispatching-unknown.
	stored, err := s.store.GetTask(ctx, task.Scope, task.ID)
	if err == nil {
		if stored.RequestHash != task.RequestHash {
			return craft.Result{}, fmt.Errorf("%w: delegation %s was prepared with a different request", craft.ErrConflict, task.ID)
		}
		return s.observe(ctx, task, stored)
	}
	if !errors.Is(err, craft.ErrNotFound) {
		return craft.Result{}, err
	}

	// O03 wiring: a session under teardown refuses NEW dispatches. The
	// durable deleting mark is the authority — the lifecycle lock alone is
	// not (a dispatch could start right after a sweep pass releases it).
	if s.guard != nil {
		if gerr := s.guard.GuardDispatch(ctx, task.Scope.TenantID, task.Scope.SessionID); gerr != nil {
			return craft.Result{}, gerr
		}
	}
	// O04 logging contract: one line chains tenant/session/run/tool_call/
	// delegation/prompt identities so an incident is followable end to end.
	// The prompt itself, token counts and model payloads never enter logs.
	logger.Infof(ctx,
		"[CraftDelegation] dispatch tenant=%d session=%s run=%s tool_call=%s delegation=%s prompt=%s",
		task.Fence.TenantID, task.Scope.SessionID, task.Fence.RunID, task.ToolCallID, task.ID, task.RequestHash)

	// 3. Fresh delegation: planned goes through PrepareTask before dispatch.
	prepared, err := s.store.PrepareTask(ctx, task)
	if err != nil {
		return craft.Result{}, err
	}
	if !prepared.Deadline.IsZero() && !time.Now().Before(prepared.Deadline) {
		return s.settle(ctx, prepared, "failed", fmt.Sprintf(
			"delegation deadline %s exceeded before dispatch; sub-execution never started",
			prepared.Deadline.Format(time.RFC3339)))
	}
	result, xerr := s.executor.Execute(ctx, prepared)
	// A terminal result straight from the executor counts too: the success
	// path never passes through settle(), and craft_delegations_total must
	// see every settled outcome, not only the observation/deadline ones.
	if xerr == nil {
		switch result.Status {
		case "succeeded", "failed", "canceled":
			metrics.CraftDelegationSettled(result.Status)
		}
	}
	return result, xerr
}

// observe settles a dispatching-unknown delegation from runtime observation
// only. The persisted outcome is written under the caller's live fence (a
// recovered worker holds a new owner/epoch), while the prompt message id
// comes from the stored task.
func (s *CraftDelegateService) observe(ctx context.Context, live, stored craft.Task) (craft.Result, error) {
	deadlineExceeded := !stored.Deadline.IsZero() && !time.Now().Before(stored.Deadline)
	obs, err := s.executor.Observe(ctx, stored)
	if err != nil {
		if errors.Is(err, craft.ErrUnsupported) {
			return s.settle(ctx, live, "failed", fmt.Sprintf("delegation cannot be observed: %v", err))
		}
		if deadlineExceeded {
			return s.settle(ctx, live, "failed", fmt.Sprintf(
				"delegation deadline exceeded; last observation failed: %v", err))
		}
		return craft.Result{}, fmt.Errorf("%w: observe delegation %s: %v", craft.ErrUnknown, stored.ID, err)
	}
	if status, summary, settled := classifyCraftObservation(obs); settled {
		return s.settle(ctx, live, status, summary)
	}
	if deadlineExceeded {
		return s.settle(ctx, live, "failed",
			"delegation deadline exceeded without a verifiable sub-execution outcome")
	}
	return craft.Result{}, fmt.Errorf(
		"%w: delegation %s is not verifiably terminal (finish=%q completed=%v idle=%v pending_tool=%v)",
		craft.ErrUnknown, stored.ID, obs.Finish, obs.Completed, obs.Idle, obs.PendingTool)
}

// classifyCraftObservation maps a runtime observation onto a definitive
// delegation outcome. The never-answered case is definitive: the runtime is
// idle with nothing pending and no assistant round ever parented to this
// delegation's prompt, so no answer can appear later — this closes the
// parked-until-deadline gap for a delegation interrupted between prepare and
// dispatch.
func classifyCraftObservation(obs craft.Observation) (status, summary string, settled bool) {
	switch {
	case opencode.Completed(obs):
		return "succeeded", "sub-execution verified complete by runtime observation", true
	case obs.Aborted:
		return "canceled", "assistant execution aborted", true
	case obs.Completed && obs.Finish != "" && obs.Finish != "stop":
		return "failed", fmt.Sprintf("assistant ended with finish %q", obs.Finish), true
	case obs.AssistantParentID == "" && obs.Idle && !obs.PendingTool:
		return "failed", "no assistant round answered the delegation prompt and the runtime is idle", true
	default:
		return "", "", false
	}
}

// settle persists one definitive outcome under the caller's live fence.
// Ordering (O04 review nit-2): the metric counts only AFTER the outcome is
// durable. If the process dies (or SaveResult fails) between classification
// and persistence, nothing was counted; the retry goes observe→settle again
// and counts once, when its own persist lands — a crash-retry can no longer
// double-count the same logical delegation. The executor path above has the
// same ordering for free: the executor persists its terminal result before
// returning it.
func (s *CraftDelegateService) settle(ctx context.Context, task craft.Task, status, summary string) (craft.Result, error) {
	result := craft.Result{TaskID: task.ID, Status: status, Summary: boundCraftSummary(summary)}
	if err := s.store.SaveResult(ctx, task.Fence, result); err != nil {
		return craft.Result{}, err
	}
	metrics.CraftDelegationSettled(status)
	logger.Infof(ctx,
		"[CraftDelegation] settled status=%s tenant=%d session=%s run=%s tool_call=%s delegation=%s prompt=%s",
		status, task.Fence.TenantID, task.Scope.SessionID, task.Fence.RunID, task.ToolCallID, task.ID, task.RequestHash)
	return result, nil
}

func boundCraftSummary(summary string) string {
	if len(summary) <= craftSummaryLimit {
		return summary
	}
	return summary[:craftSummaryLimit]
}

// unavailableCraftExecutor is the fail-closed production executor used until
// the Craft runtime deployment provides the sandbox dial. Every delegation
// settles as a definitive unsupported failure — never a fabricated success,
// never a persisted unknown.
type unavailableCraftExecutor struct{ reason string }

func (e unavailableCraftExecutor) Execute(context.Context, craft.Task) (craft.Result, error) {
	return craft.Result{}, fmt.Errorf("%w: %s", craft.ErrUnsupported, e.reason)
}

func (e unavailableCraftExecutor) Observe(context.Context, craft.Task) (craft.Observation, error) {
	return craft.Observation{}, fmt.Errorf("%w: %s", craft.ErrUnsupported, e.reason)
}

func (e unavailableCraftExecutor) Abort(context.Context, craft.Task) error {
	return fmt.Errorf("%w: %s", craft.ErrUnsupported, e.reason)
}

// NewUnavailableCraftExecutor returns the fail-closed executor for
// deployments without the Craft runtime dial.
func NewUnavailableCraftExecutor(reason string) craft.Executor {
	return unavailableCraftExecutor{reason: reason}
}

// CraftActiveRunsQuery is the production active-run check for the Craft
// workspace resolver (R03 nit-1): a session has an active run when a
// non-terminal agent_runs row exists for it. The R03 review's
// status IN ('running','recovering') is deliberately extended with 'queued'
// and 'waiting_user' — those runs still hold the session's active-run slot
// (admission's ErrRunActive predicate), so a sandbox rebind under them would
// corrupt a run that resumes later. This is the real database query, never
// an always-false stub.
func CraftActiveRunsQuery(db *gorm.DB) CraftRunActivity {
	return func(ctx context.Context, scope craft.Scope) (bool, error) {
		if db == nil {
			return false, errors.New("craft: active-run query requires a database")
		}
		var count int64
		err := db.WithContext(ctx).Table("agent_runs").
			Where("tenant_id = ? AND session_id = ?", scope.TenantID, scope.SessionID).
			Where("status IN ('queued','running','recovering','waiting_user')").
			Count(&count).Error
		if err != nil {
			return false, err
		}
		return count > 0, nil
	}
}

// registerCraftDelegateTool opens the craft_delegate tool for exactly the
// Craft+tRPC sessions: the session row must select the tRPC engine and a
// Craft workspace must be bound to the authenticated scope. Builtin sessions
// and tRPC sessions without a workspace keep the unchanged builtin tool set.
// All identity (scope, workspace, skill digests, delegation service) is
// assembled here from persisted state; the model only ever supplies goal and
// authorized input references.
func (s *agentService) registerCraftDelegateTool(
	ctx context.Context,
	registry *tools.ToolRegistry,
	config *types.AgentConfig,
	sessionID string,
) error {
	if s == nil || s.craft == nil || s.db == nil || registry == nil || strings.TrimSpace(sessionID) == "" {
		return nil
	}
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || tenantID == 0 {
		logger.Warnf(ctx, "craft_delegate not registered: no tenant on context for session %s", sessionID)
		return nil
	}
	var session struct {
		EngineType string
		UserID     string
	}
	err := s.db.WithContext(ctx).Table("sessions").
		Select("engine_type, user_id").
		Where("tenant_id = ? AND id = ?", tenantID, sessionID).
		Take(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("craft_delegate gate: read session %s: %w", sessionID, err)
	}
	if session.EngineType != string(types.AgentEngineTRPC) {
		// Builtin engine sessions never see the Craft tool.
		return nil
	}
	if strings.TrimSpace(session.UserID) == "" {
		return nil
	}
	scope := craft.Scope{TenantID: tenantID, UserID: session.UserID, SessionID: sessionID}
	workspace, err := s.craft.Store.GetWorkspace(ctx, scope)
	if errors.Is(err, craft.ErrNotFound) {
		// A tRPC session without a Craft workspace is not a Craft session.
		return nil
	}
	if err != nil {
		return fmt.Errorf("craft_delegate gate: read workspace for session %s: %w", sessionID, err)
	}
	tool, err := tools.NewCraftDelegateTool(tools.CraftDelegateToolConfig{
		Scope:        scope,
		WorkspaceID:  workspace.ID,
		SkillDigests: craftSkillDigests(config),
		Delegate:     s.craft.Delegate.Delegate,
	})
	if err != nil {
		return fmt.Errorf("craft_delegate gate: %w", err)
	}
	registry.RegisterTool(tool)
	logger.Infof(ctx, "craft_delegate registered for craft trpc session %s workspace %s", sessionID, workspace.ID)
	return nil
}

// craftSkillDigests pins the tenant skill versions visible to this run.
func craftSkillDigests(config *types.AgentConfig) []string {
	if config == nil {
		return nil
	}
	digests := make([]string, 0, len(config.TenantSkills))
	for _, skill := range config.TenantSkills {
		if skill == nil || strings.TrimSpace(skill.Name) == "" {
			continue
		}
		sum := sha256.Sum256([]byte(skill.Name + "\x00" + skill.Version))
		digests = append(digests, "skill:"+hex.EncodeToString(sum[:12]))
	}
	sort.Strings(digests)
	return digests
}
