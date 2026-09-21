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

	"github.com/Tencent/WeKnora/internal/application/repository"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	domaincommercial "github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	"github.com/Tencent/WeKnora/internal/modules/execution"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrBudgetDenied    = errors.New("budget denied")
	ErrRequestPending  = errors.New("request admission pending")
	ErrRequestRejected = errors.New("request rejected")
)

// TrustedAdmissionBinding is resolved by a server-side policy/target service.
// It is deliberately excluded from JSON input so a client cannot self-assert
// funding, credentials, parent ownership, or pricing.
type TrustedAdmissionBinding struct {
	ParentRunID       string
	Source            string
	Funding           string
	Service           string
	PriceVersion      string
	CredentialVersion int64
	Upper             int64
	Revision          int64
	Status            string
	Dimensions        map[string]int64
}

// AdmissionBindingResolver is the trusted server seam for target, credential,
// parent budget, and pricing selection. Implementations must validate tenant
// and actor ownership before returning a binding.
type AdmissionBindingResolver interface {
	Resolve(context.Context, uint64, string, StartInput) (TrustedAdmissionBinding, error)
}

type AdmissionBindingResolverFunc func(context.Context, uint64, string, StartInput) (TrustedAdmissionBinding, error)

func (f AdmissionBindingResolverFunc) Resolve(ctx context.Context, tenant uint64, actor string, in StartInput) (TrustedAdmissionBinding, error) {
	return f(ctx, tenant, actor, in)
}

type StartInput struct {
	SessionID    string `json:"session_id"`
	AgentID      string `json:"agent_id"`
	TargetID     string `json:"target_id"`
	WorkspaceRef string `json:"workspace_ref"`
	// SpaceID is navigation metadata projected into the immutable run snapshot
	// so the owned execution list can deep-link back into the product session.
	// It is deliberately not part of the request hash: a retry that omits it
	// must still reconcile onto the original request.
	SpaceID     string `json:"space_id"`
	RequestID   string `json:"request_id"`
	Text        string `json:"text"`
	BudgetUpper int64  `json:"budget_upper"`
	// Binding carries the trusted usage binding resolved from the execution
	// target at admission time; it never crosses the HTTP boundary.
	Binding *TrustedAdmissionBinding `json:"-"`
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

// DurableTaskBudget binds admission to the same commercial task-budget
// tables used by ExecutionGateService. It is deliberately small: call-level
// holds remain owned by ExecutionGate; admission only registers the run tree.
type DurableTaskBudget struct{ store *repocommercial.BudgetStore }

func NewDurableTaskBudget(db *gorm.DB) *DurableTaskBudget {
	if db == nil {
		return nil
	}
	return &DurableTaskBudget{store: repocommercial.NewBudgetStore(db)}
}
func (b *DurableTaskBudget) Ensure(_ context.Context, tenant uint64, owner, requestID string, _ int64, _ time.Time) (string, error) {
	if b == nil || b.store == nil || tenant == 0 || owner == "" || requestID == "" {
		return "", ErrBudgetDenied
	}
	return fmt.Sprintf("task/%d/%s/%s", tenant, owner, requestID), nil
}
func (*DurableTaskBudget) ReleaseUnstarted(context.Context, string) error { return nil }

type taskRunRegistrar interface {
	BindRun(context.Context, uint64, string, string, int64, time.Time) error
}

func (b *DurableTaskBudget) BindRun(ctx context.Context, tenant uint64, runID, parent string, upper int64, deadline time.Time) error {
	if b == nil || b.store == nil {
		return ErrBudgetDenied
	}
	if parent != "" {
		return b.store.AttachChildRun(ctx, tenant, runID, parent)
	}
	return b.store.EnsureTaskBudget(ctx, tenant, runID, domaincommercial.Credits(upper), deadline)
}

type AdmissionCoordinator struct {
	db       *gorm.DB
	runs     *repository.AgentRunStore
	requests *repository.WorkbenchRequestRepository
	budget   TaskBudgetPort
	binding  AdmissionBindingResolver
	publish  func(context.Context, agentruntime.RunKey) error
	// admissionGate is the W34 capability switch seam (drain /
	// platform_admission), installed by the container assembly through
	// SetAdmissionGate. nil keeps admission open (legacy behaviour).
	admissionGate func(targetID string) error
}

// SetAdmissionGate installs the W34 capability gate consulted by Start
// before identity, budget reservation or any durable write. Passing nil
// removes the gate.
func (a *AdmissionCoordinator) SetAdmissionGate(gate func(targetID string) error) {
	if a == nil {
		return
	}
	a.admissionGate = gate
}

func NewAdmissionCoordinator(db *gorm.DB, runs *repository.AgentRunStore, budget TaskBudgetPort, publish func(context.Context, agentruntime.RunKey) error) *AdmissionCoordinator {
	if budget == nil {
		budget = NoopTaskBudget{}
	}
	if publish == nil {
		publish = func(context.Context, agentruntime.RunKey) error { return nil }
	}
	return &AdmissionCoordinator{db: db, runs: runs, requests: repository.NewWorkbenchRequestRepository(db), budget: budget, binding: NewDatabaseAdmissionBindingResolver(nil), publish: publish}
}

// NewAdmissionCoordinatorWithBinding is the production constructor. The
// resolver is mandatory so admission cannot fall back to caller-controlled
// target or billing fields.
func NewAdmissionCoordinatorWithBinding(db *gorm.DB, runs *repository.AgentRunStore, budget TaskBudgetPort, publish func(context.Context, agentruntime.RunKey) error, binding AdmissionBindingResolver) (*AdmissionCoordinator, error) {
	if binding == nil {
		return nil, errors.New("trusted admission binding resolver is required")
	}
	if budget == nil {
		budget = NoopTaskBudget{}
	}
	if publish == nil {
		publish = func(context.Context, agentruntime.RunKey) error { return nil }
	}
	return &AdmissionCoordinator{db: db, runs: runs, requests: repository.NewWorkbenchRequestRepository(db), budget: budget, binding: binding, publish: publish}, nil
}

func (a *AdmissionCoordinator) WithBindingResolver(binding AdmissionBindingResolver) *AdmissionCoordinator {
	if a != nil && binding != nil {
		a.binding = binding
	}
	return a
}

// PlatformAdmissionPolicy is the server-owned default for platform runs.
// Deployments that support BYOK or delegated parents replace this policy via
// the database resolver's trusted binding input; no client JSON is read.
func PlatformAdmissionPolicy() TrustedAdmissionBinding {
	return TrustedAdmissionBinding{Source: "platform_gateway", Funding: "platform", Service: "connector", PriceVersion: "remote-v1", Revision: 1, Status: "final", Dimensions: map[string]int64{"connector": 1}}
}

// NewDatabaseAdmissionBindingResolver resolves target ownership from the
// durable execution-target store. A trusted server binding may additionally
// carry BYOK credentials or a parent run; client JSON cannot populate it.
func NewDatabaseAdmissionBindingResolver(targets repository.ExecutionTargetStore) AdmissionBindingResolver {
	return databaseAdmissionBindingResolver{targets: targets, platform: PlatformAdmissionPolicy()}
}

type databaseAdmissionBindingResolver struct {
	targets  repository.ExecutionTargetStore
	platform TrustedAdmissionBinding
}

func (r databaseAdmissionBindingResolver) Resolve(ctx context.Context, tenant uint64, actor string, in StartInput) (TrustedAdmissionBinding, error) {
	if in.Binding != nil {
		// This resolver is the production boundary; a request-scoped Binding
		// is never trusted because it is not loaded from ExecutionTargetStore.
		return TrustedAdmissionBinding{}, execution.ErrTargetUntrusted
	}
	if in.TargetID == "platform" {
		b := r.platform
		b.Upper = in.BudgetUpper
		return b, nil
	}
	if r.targets == nil {
		return TrustedAdmissionBinding{}, execution.ErrTargetUntrusted
	}
	target, err := r.targets.GetOwnedTarget(ctx, tenant, actor, in.TargetID)
	if err != nil {
		return TrustedAdmissionBinding{}, err
	}
	if err := execution.AuthorizeTarget(target, tenant, actor); err != nil {
		return TrustedAdmissionBinding{}, err
	}
	b := r.platform
	policy := target.UsageBinding
	if policy.Source == "" || policy.Funding == "" || policy.Service == "" || policy.PriceVersion == "" || policy.Revision <= 0 || len(policy.Dimensions) == 0 {
		return TrustedAdmissionBinding{}, execution.ErrTargetUntrusted
	}
	b.ParentRunID, b.Source, b.Funding, b.Service, b.PriceVersion = policy.ParentRunID, policy.Source, policy.Funding, policy.Service, policy.PriceVersion
	b.Revision, b.Status, b.Dimensions = policy.Revision, policy.Status, policy.Dimensions
	b.CredentialVersion = target.CredentialVersion
	b.Upper = in.BudgetUpper
	return b, nil
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
	if a == nil {
		return agentruntime.Run{}, errors.New("admission coordinator is not configured")
	}
	// W34 capability gate: consulted BEFORE identity, budget reservation or
	// any durable write, so a closed lane rejects NEW work without side
	// effects. Already-admitted runs and cleanup paths are untouched (drain
	// semantics). The target is normalized the same way as below so the
	// gate always sees "platform" for unset targets.
	gateTarget := in.TargetID
	if strings.TrimSpace(gateTarget) == "" {
		gateTarget = "platform"
	}
	if a.admissionGate != nil {
		if err := a.admissionGate(gateTarget); err != nil {
			return agentruntime.Run{}, err
		}
	}
	if a.runs == nil || a.requests == nil {
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
	bindingResolver := a.binding
	if bindingResolver == nil {
		return agentruntime.Run{}, errors.New("trusted admission binding resolver is required")
	}
	binding, err := bindingResolver.Resolve(ctx, req.TenantID, req.ActorID, in)
	if err != nil {
		return agentruntime.Run{}, err
	}
	if binding.Upper <= 0 {
		binding.Upper = in.BudgetUpper
	}
	if binding.Upper <= 0 {
		binding.Upper = 1
	}
	if binding.Revision <= 0 || binding.Source == "" || binding.Funding == "" || binding.Service == "" || binding.PriceVersion == "" || len(binding.Dimensions) == 0 {
		return agentruntime.Run{}, errors.New("trusted admission binding is incomplete")
	}
	snapshot, _ := json.Marshal(map[string]any{
		"session_id": in.SessionID, "agent_id": in.AgentID, "target_id": in.TargetID, "workspace_ref": in.WorkspaceRef, "space_id": in.SpaceID, "request_id": in.RequestID, "text": in.Text, "budget_upper": in.BudgetUpper,
		"parent_run_id": binding.ParentRunID, "credential_version": binding.CredentialVersion, "usage_source": binding.Source, "usage_funding": binding.Funding, "usage_service": binding.Service, "price_version": binding.PriceVersion, "usage_upper": binding.Upper, "usage_revision": binding.Revision, "usage_status": binding.Status, "usage_dimensions": binding.Dimensions,
	})
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
		reservation, err = a.budget.Ensure(ctx, req.TenantID, req.ActorID, req.RequestID, binding.Upper, deadline)
		ownedReservation = true
		if err != nil {
			_ = a.requests.UpdatePending(ctx, req, "rejected", "", "", err.Error())
			return agentruntime.Run{}, err
		}
	}
	if runID == "" {
		runID = uuid.NewString()
	}
	if registrar, ok := a.budget.(taskRunRegistrar); ok {
		if err = registrar.BindRun(ctx, req.TenantID, runID, binding.ParentRunID, binding.Upper, deadline); err != nil {
			return agentruntime.Run{}, err
		}
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
	userMessage, _ := json.Marshal(map[string]any{"role": "user", "content": in.Text})
	assistantMessage, _ := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	run, err = a.runs.Admit(ctx, agentruntime.Admission{Key: agentruntime.RunKey{TenantID: req.TenantID, RunID: runID}, SessionID: in.SessionID, UserID: req.ActorID, RequestID: in.RequestID, AssistantMessageID: assistantID, Driver: "platform", TargetID: "platform", BudgetRef: reservation, RequestHash: req.RequestHash, Snapshot: snapshot, UserMessage: userMessage, AssistantMessage: assistantMessage, Deadline: deadline, ParentRunID: binding.ParentRunID, UsageCredentialVersion: binding.CredentialVersion, UsageSource: binding.Source, UsageFunding: binding.Funding, UsageService: binding.Service, UsagePriceVersion: binding.PriceVersion, UsageUpper: binding.Upper, UsageRevision: binding.Revision, UsageStatus: binding.Status, UsageDimensions: binding.Dimensions})
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
