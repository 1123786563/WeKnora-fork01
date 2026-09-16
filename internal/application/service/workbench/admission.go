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
	Ensure(context.Context, uint64, string, string, int64, time.Time) (string, error)
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
	budget   TaskBudgetPort
	publish  func(context.Context, agentruntime.RunKey) error
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
	if in.TargetID != "platform" {
		return agentruntime.Run{}, agentruntime.ErrConflict
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
	if len(ids) > 0 {
		runID = ids[0]
	}
	if len(ids) > 1 {
		reservation = ids[1]
	}
	if reservation == "" {
		reservation, err = a.budget.Ensure(ctx, req.TenantID, req.ActorID, req.RequestID, in.BudgetUpper, deadline)
		if err != nil {
			_ = a.requests.UpdatePending(ctx, req, "rejected", "", "", err.Error())
			return agentruntime.Run{}, err
		}
	}
	if runID == "" {
		runID = uuid.NewString()
	}
	if err = a.requests.UpdatePending(ctx, req, "pending", reservation, runID, ""); err != nil {
		_ = a.budget.ReleaseUnstarted(ctx, reservation)
		return agentruntime.Run{}, err
	}
	assistantID := uuid.NewString()
	snapshot, _ := json.Marshal(map[string]any{"session_id": in.SessionID, "agent_id": in.AgentID, "target_id": in.TargetID, "workspace_ref": in.WorkspaceRef, "request_id": in.RequestID, "text": in.Text, "budget_upper": in.BudgetUpper})
	userMessage, _ := json.Marshal(map[string]any{"role": "user", "content": in.Text})
	assistantMessage, _ := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	run, err = a.runs.Admit(ctx, agentruntime.Admission{Key: agentruntime.RunKey{TenantID: req.TenantID, RunID: runID}, SessionID: in.SessionID, UserID: req.ActorID, RequestID: in.RequestID, AssistantMessageID: assistantID, Driver: "platform", TargetID: "platform", BudgetRef: reservation, RequestHash: req.RequestHash, Snapshot: snapshot, UserMessage: userMessage, AssistantMessage: assistantMessage, Deadline: deadline})
	if err != nil {
		_ = a.requests.UpdatePending(ctx, req, "rejected", reservation, "", err.Error())
		_ = a.budget.ReleaseUnstarted(ctx, reservation)
		return agentruntime.Run{}, err
	}
	if err = a.publish(ctx, run.Key); err != nil {
		_ = a.requests.UpdatePending(ctx, req, "rejected", reservation, run.Key.RunID, err.Error())
		return agentruntime.Run{}, err
	}
	if err = a.requests.UpdatePending(ctx, req, "admitted", reservation, run.Key.RunID, ""); err != nil {
		return agentruntime.Run{}, err
	}
	run.BudgetRef = reservation
	return run, nil
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
