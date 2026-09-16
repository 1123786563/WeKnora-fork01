package workbench

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/execution"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrBudgetDenied    = errors.New("budget denied")
	ErrRequestPending  = errors.New("request admission pending")
	ErrRequestRejected = errors.New("request rejected")
)

type StartInput struct {
	SessionID    string `json:"session_id"`
	AgentID      string `json:"agent_id"`
	TargetID     string `json:"target_id"`
	WorkspaceRef string `json:"workspace_ref"`
	RequestID    string `json:"request_id"`
	Text         string `json:"text"`
	BudgetUpper  int64  `json:"budget_upper"`
}

type RequestState struct {
	State  string `json:"state"`
	RunID  string `json:"run_id,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type TaskBudgetPort interface {
	// Ensure must use the tuple (tenant, owner, requestID) as a durable
	// idempotency key. A retry after an unknown response or a process crash
	// before reservation_ref is persisted must return the original reservation
	// reference and must not create or charge a second task reservation. A
	// changed upper bound for the same key must be rejected by the ledger.
	Ensure(context.Context, uint64, string, string, int64, time.Time) (string, error)
	// ReleaseUnstarted is only called when the run was proven not to have been
	// admitted or dispatched. Unknown dispatch state must remain reserved.
	ReleaseUnstarted(context.Context, string) error
}

// NoopTaskBudget is a compatibility adapter for deployments that have not yet
// registered the credit ledger. It creates a stable task reservation key; the
// actual call-level usage ledger remains owned by the billing service.
type NoopTaskBudget struct{}

func (NoopTaskBudget) Ensure(_ context.Context, tenant uint64, owner, requestID string, upper int64, _ time.Time) (string, error) {
	if tenant == 0 || owner == "" || requestID == "" || upper < 0 {
		return "", ErrBudgetDenied
	}
	return fmt.Sprintf("task/%d/%s/%s", tenant, owner, requestID), nil
}
func (NoopTaskBudget) ReleaseUnstarted(context.Context, string) error { return nil }

type AdmissionCoordinator struct {
	db       *gorm.DB
	runs     *repository.AgentRunStore
	requests *repository.WorkbenchRequestRepository
	targets  repository.ExecutionTargetStore
	budget   TaskBudgetPort
	publish  func(context.Context, agentruntime.RunKey) error
}

// NewAdmissionCoordinatorWithTargets wires W20 admission to the trusted
// execution-target projection used by personal-node registration.
func NewAdmissionCoordinatorWithTargets(db *gorm.DB, runs *repository.AgentRunStore, targets repository.ExecutionTargetStore, budget TaskBudgetPort, publish func(context.Context, agentruntime.RunKey) error) *AdmissionCoordinator {
	a := NewAdmissionCoordinator(db, runs, budget, publish)
	a.targets = targets
	return a
}

func NewAdmissionCoordinator(db *gorm.DB, runs *repository.AgentRunStore, budget TaskBudgetPort, publish func(context.Context, agentruntime.RunKey) error) *AdmissionCoordinator {
	if budget == nil {
		budget = NoopTaskBudget{}
	}
	if publish == nil {
		publish = func(context.Context, agentruntime.RunKey) error { return nil }
	}
	return &AdmissionCoordinator{db: db, runs: runs, requests: repository.NewWorkbenchRequestRepository(db), budget: budget, publish: publish}
}

func admitThenPublish(admit func() error, publish func() error) error {
	if err := admit(); err != nil {
		return err
	}
	return publish()
}

func requestHash(in StartInput) string {
	canonical := struct {
		SessionID, AgentID, TargetID, WorkspaceRef, Text string
		BudgetUpper                                      int64
	}{in.SessionID, in.AgentID, in.TargetID, in.WorkspaceRef, in.Text, in.BudgetUpper}
	b, _ := json.Marshal(canonical)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func contextIdentity(ctx context.Context) (uint64, string, error) {
	tenant, ok := types.TenantIDFromContext(ctx)
	if !ok || tenant == 0 {
		return 0, "", errors.New("tenant context is required")
	}
	actor, ok := types.UserIDFromContext(ctx)
	if !ok || strings.TrimSpace(actor) == "" {
		return 0, "", errors.New("actor context is required")
	}
	return tenant, actor, nil
}

func (a *AdmissionCoordinator) Start(ctx context.Context, in StartInput) (agentruntime.Run, error) {
	if a == nil || a.runs == nil || a.requests == nil {
		return agentruntime.Run{}, errors.New("admission coordinator is not configured")
	}
	tenant, actor, err := contextIdentity(ctx)
	if err != nil {
		return agentruntime.Run{}, err
	}
	if strings.TrimSpace(in.SessionID) == "" || strings.TrimSpace(in.RequestID) == "" || strings.TrimSpace(in.Text) == "" || in.BudgetUpper < 0 {
		return agentruntime.Run{}, agentruntime.ErrConflict
	}
	if in.TargetID == "" {
		in.TargetID = "platform"
	}
	var target execution.Target
	if in.TargetID != "platform" {
		if a.targets == nil {
			return agentruntime.Run{}, execution.ErrTargetForbidden
		}
		target, err = a.targets.GetOwnedTarget(ctx, tenant, actor, in.TargetID)
		if err != nil {
			return agentruntime.Run{}, execution.ErrTargetForbidden
		}
		if err = execution.AuthorizeTarget(target, tenant, actor); err != nil {
			return agentruntime.Run{}, err
		}
		if strings.TrimSpace(in.WorkspaceRef) == "" {
			return agentruntime.Run{}, agentruntime.ErrConflict
		}
		workspace, workspaceErr := a.targets.GetOwnedWorkspace(ctx, tenant, actor, in.WorkspaceRef)
		if workspaceErr != nil || workspace.TargetID != target.ID {
			return agentruntime.Run{}, execution.ErrTargetForbidden
		}
	}
	hash := requestHash(in)
	req := repository.WorkbenchRequest{TenantID: tenant, ActorID: actor, RequestID: in.RequestID, RequestHash: hash,
		SessionID: in.SessionID, AgentID: in.AgentID, TargetID: in.TargetID, WorkspaceRef: in.WorkspaceRef, Text: in.Text, BudgetUpper: in.BudgetUpper}
	if err := a.requests.CreatePending(ctx, req); err != nil {
		existing, getErr := a.requests.Get(ctx, tenant, actor, in.RequestID)
		if getErr != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(strings.ToLower(err.Error()), "unique") {
				return agentruntime.Run{}, getErr
			}
			return agentruntime.Run{}, err
		}
		if existing.RequestHash != hash {
			return agentruntime.Run{}, agentruntime.ErrConflict
		}
		return a.resumeExisting(ctx, existing, in)
	}
	return a.admitPending(ctx, req, in)
}

func (a *AdmissionCoordinator) resumeExisting(ctx context.Context, req repository.WorkbenchRequest, in StartInput) (agentruntime.Run, error) {
	for i := 0; i < 20; i++ {
		current, err := a.requests.Get(ctx, req.TenantID, req.ActorID, req.RequestID)
		if err != nil {
			return agentruntime.Run{}, err
		}
		if current.RequestHash != req.RequestHash {
			return agentruntime.Run{}, agentruntime.ErrConflict
		}
		switch current.State {
		case "admitted":
			return a.runs.Get(ctx, agentruntime.RunKey{TenantID: current.TenantID, RunID: current.RunID})
		case "dispatching":
			// The durable run is the outbox. A worker scans queued platform
			// runs, so an uncertain wake-up must never be published twice.
			return a.runs.Get(ctx, agentruntime.RunKey{TenantID: current.TenantID, RunID: current.RunID})
		case "rejected":
			return agentruntime.Run{}, fmt.Errorf("%w: %s", ErrRequestRejected, current.Reason)
		case "pending":
			if current.RunID != "" {
				return a.admitPending(ctx, current, in, current.RunID, current.ReservationRef)
			}
		}
		req = current
		time.Sleep(5 * time.Millisecond)
	}
	return agentruntime.Run{}, ErrRequestPending
}

func (a *AdmissionCoordinator) admitPending(ctx context.Context, req repository.WorkbenchRequest, in StartInput, ids ...string) (run agentruntime.Run, err error) {
	deadline := time.Now().Add(10 * time.Minute)
	var runID, reservation string
	ownedReservation := false
	if len(ids) > 0 {
		runID = ids[0]
	}
	if len(ids) > 1 {
		reservation = ids[1]
	}
	if reservation == "" {
		reservation, err = a.budget.Ensure(ctx, req.TenantID, req.ActorID, req.RequestID, in.BudgetUpper, deadline)
		ownedReservation = true
		if err != nil {
			_ = a.requests.UpdatePending(ctx, req, "rejected", "", "", err.Error())
			return agentruntime.Run{}, err
		}
	}
	if runID == "" {
		runID = uuid.NewString()
	}
	if err = a.requests.UpdatePending(ctx, req, "pending", reservation, runID, ""); err != nil {
		current, getErr := a.requests.Get(ctx, req.TenantID, req.ActorID, req.RequestID)
		if getErr == nil && (current.State == "dispatching" || current.State == "admitted") && current.RunID != "" {
			return a.runs.Get(ctx, agentruntime.RunKey{TenantID: current.TenantID, RunID: current.RunID})
		}
		// A concurrent retry may have already written the same pending run
		// identity. It is safe to continue into AgentRunStore.Admit, whose
		// request hash check makes that operation idempotent.
		if getErr == nil && current.State == "pending" && current.RunID == runID {
			// continue
		} else {
			if ownedReservation {
				_ = a.budget.ReleaseUnstarted(ctx, reservation)
			}
			return agentruntime.Run{}, err
		}
	}
	assistantID := uuid.NewString()
	driver := "platform"
	credentialVersion := int64(0)
	if in.TargetID != "platform" {
		driver = "paseo"
		// Re-resolve at the final admission seam so a revoke between Start and
		// retry cannot reuse a stale target projection.
		resolved, resolveErr := a.targets.GetOwnedTarget(ctx, req.TenantID, req.ActorID, in.TargetID)
		if resolveErr != nil {
			return agentruntime.Run{}, a.rejectUnadmitted(ctx, req, reservation, execution.ErrTargetForbidden)
		}
		if resolveErr = execution.AuthorizeTarget(resolved, req.TenantID, req.ActorID); resolveErr != nil {
			return agentruntime.Run{}, a.rejectUnadmitted(ctx, req, reservation, resolveErr)
		}
		credentialVersion = resolved.CredentialVersion
	}
	snapshot, _ := json.Marshal(map[string]any{"session_id": in.SessionID, "agent_id": in.AgentID, "target_id": in.TargetID, "workspace_ref": in.WorkspaceRef, "credential_version": credentialVersion, "request_id": in.RequestID, "text": in.Text, "budget_upper": in.BudgetUpper})
	userMessage, _ := json.Marshal(map[string]any{"role": "user", "content": in.Text})
	assistantMessage, _ := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	run, err = a.runs.Admit(ctx, agentruntime.Admission{Key: agentruntime.RunKey{TenantID: req.TenantID, RunID: runID}, SessionID: in.SessionID, UserID: req.ActorID, RequestID: in.RequestID, AssistantMessageID: assistantID, Driver: driver, TargetID: in.TargetID, BudgetRef: reservation, RequestHash: req.RequestHash, Snapshot: snapshot, UserMessage: userMessage, AssistantMessage: assistantMessage, Deadline: deadline})
	if err != nil {
		_ = a.requests.UpdatePending(ctx, req, "rejected", reservation, "", err.Error())
		_ = a.budget.ReleaseUnstarted(ctx, reservation)
		return agentruntime.Run{}, err
	}
	// Mark the durable dispatch record before invoking the wake-up callback.
	// If the callback succeeds but this request returns before its final CAS,
	// retries observe dispatching and return the same run without republishing.
	if err = a.requests.UpdateFromState(ctx, req, "pending", "dispatching", reservation, run.Key.RunID, ""); err != nil {
		// Another concurrent admission may have completed this transition.
		// The durable run is already idempotently present, so return it when
		// the row says dispatching/admitted instead of creating a second wakeup.
		current, getErr := a.requests.Get(ctx, req.TenantID, req.ActorID, req.RequestID)
		if getErr == nil && (current.State == "dispatching" || current.State == "admitted") {
			return a.runs.Get(ctx, agentruntime.RunKey{TenantID: current.TenantID, RunID: current.RunID})
		}
		return agentruntime.Run{}, err
	}
	if err = a.publish(ctx, run.Key); err != nil {
		// A known callback failure is retryable. If this CAS itself fails, the
		// dispatching state remains safe and the worker's durable scan recovers it.
		_ = a.requests.UpdateFromState(ctx, req, "dispatching", "pending", reservation, run.Key.RunID, err.Error())
		return agentruntime.Run{}, err
	}
	if err = a.requests.UpdateFromState(ctx, req, "dispatching", "admitted", reservation, run.Key.RunID, ""); err != nil {
		return agentruntime.Run{}, err
	}
	run.BudgetRef = reservation
	return run, nil
}

// rejectUnadmitted closes the durable request before releasing the reservation
// owned by this admission attempt. Once the request is rejected, retries take
// the terminal path and cannot release the same reservation a second time.
// Keeping the reservation reference on the rejected row also gives a durable
// audit/recovery key if a budget provider reports an unknown release result.
func (a *AdmissionCoordinator) rejectUnadmitted(ctx context.Context, req repository.WorkbenchRequest, reservation string, cause error) error {
	reason := cause.Error()
	if err := a.requests.UpdatePending(ctx, req, "rejected", reservation, "", reason); err != nil {
		return fmt.Errorf("%w: reject request: %v", cause, err)
	}
	if reservation != "" {
		if err := a.budget.ReleaseUnstarted(ctx, reservation); err != nil {
			return fmt.Errorf("%w: release reservation: %v", cause, err)
		}
	}
	return cause
}

func (a *AdmissionCoordinator) LookupRequest(ctx context.Context, requestID string) (RequestState, error) {
	tenant, actor, err := contextIdentity(ctx)
	if err != nil {
		return RequestState{}, err
	}
	req, err := a.requests.Get(ctx, tenant, actor, strings.TrimSpace(requestID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return RequestState{State: "unknown"}, nil
	}
	if err != nil {
		return RequestState{}, err
	}
	return RequestState{State: req.State, RunID: req.RunID, Reason: req.Reason}, nil
}
