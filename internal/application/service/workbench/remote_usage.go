package workbench

import (
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	"github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/execution"
)

var (
	ErrRemoteUsageUntrustedSource = errors.New("remote_usage_untrusted_source")
	ErrRemoteUsageInvalidRequest  = errors.New("remote_usage_invalid_request")
	ErrRemoteUsageNotBillable     = errors.New("remote_usage_not_billable")
)

// RemoteUsageRequest is the server-constructed identity and billing context
// for one physical remote attempt. Source is derived from the authenticated
// gateway binding and must not be copied from a remote event payload.
type RemoteUsageRequest struct {
	TenantID     uint64
	RunID        string
	DelegationID string
	CallID       string
	AttemptID    string
	Reservation  string
	ParentRunID  string
	Key          string
	Upper        commercial.Credits
	Deadline     time.Time
	Source       string
	Funding      string
	Service      string
	PriceVersion string
	Revision     int64
	OccurredAt   time.Time
	Dimensions   map[string]int64
	Status       string
}

// RemoteUsageHandle keeps the server-derived binding private between the
// reservation and settlement calls. Callers receive no mutable billable
// source/funding fields to echo back from a provider payload.
type RemoteUsageHandle struct {
	reservation commercial.Reservation
	request     RemoteUsageRequest
}

// BeginRemote constructs the billable request from the server-owned worker
// fence. A provider payload cannot supply Source/Funding/PriceVersion; those
// values are selected here after the durable dispatch claim has succeeded.
func (s *RemoteUsageService) BeginRemote(ctx context.Context, fence agentruntime.Fence, commandID string) (RemoteUsageHandle, error) {
	req, err := remoteRequestFromFence(fence, commandID)
	if err != nil {
		return RemoteUsageHandle{}, err
	}
	res, err := s.beginBound(ctx, req)
	if err != nil {
		return RemoteUsageHandle{}, err
	}
	return RemoteUsageHandle{reservation: res, request: req}, nil
}

// ReconcileRemoteObservation reconstructs the durable reservation identity
// from the server-owned fence after an unknown/partial/display_only provider
// result. It never reserves a second hold; Finish is the only transition.
func (s *RemoteUsageService) ReconcileRemoteObservation(ctx context.Context, fence agentruntime.Fence, commandID string, observation *agentruntime.RemoteUsageObservation) error {
	req, err := remoteRequestFromFence(fence, commandID)
	if err != nil {
		return err
	}
	return s.FinishRemoteObservation(ctx, RemoteUsageHandle{reservation: commercial.Reservation{ID: stableUsageKey(req)}, request: req}, observation)
}

func remoteRequestFromFence(fence agentruntime.Fence, commandID string) (RemoteUsageRequest, error) {
	if fence.TenantID == 0 || fence.RunID == "" || commandID == "" {
		return RemoteUsageRequest{}, ErrRemoteUsageInvalidRequest
	}
	service := fence.UsageService
	if service == "" {
		service = commercial.ServiceConnector
	}
	funding := fence.UsageFunding
	if funding == "" {
		funding = commercial.FundingPlatform
	}
	source := fence.UsageSource
	if source == "" {
		source = "platform_gateway"
	}
	price := fence.UsagePriceVersion
	if price == "" {
		price = "remote-v1"
	}
	revision := fence.UsageRevision
	if revision <= 0 {
		revision = 1
	}
	status := fence.UsageStatus
	if status == "" {
		status = commercial.UsageStatusFinal
	}
	dimensions := cloneDimensions(fence.UsageDimensions)
	if dimensions == nil {
		dimension := commercial.DimensionConnector
		if service == commercial.ServiceModel {
			dimension = commercial.DimensionModel
		}
		dimensions = map[string]int64{dimension: 1}
	}
	upper := commercial.Credits(fence.UsageUpper)
	if upper <= 0 {
		upper = 1
	}
	now := time.Now().UTC()
	return RemoteUsageRequest{TenantID: fence.TenantID, RunID: fence.RunID, CallID: commandID, AttemptID: commandID, ParentRunID: fence.ParentRunID, Upper: upper, Deadline: now.Add(10 * time.Minute), Source: source, Funding: funding, Service: service, PriceVersion: price, Revision: revision, OccurredAt: now, Dimensions: dimensions, Status: status}, nil
}

func (s *RemoteUsageService) FinishRemote(ctx context.Context, handle RemoteUsageHandle) error {
	return s.FinishRemoteObservation(ctx, handle, nil)
}

// FinishRemoteObservation applies provider quantities while retaining the
// server-owned funding/source/price/identity binding. Unknown or partial
// observations are deliberately rejected as billable facts; the dispatcher
// must persist the command as unknown so a later observation can reconcile it.
func (s *RemoteUsageService) FinishRemoteObservation(ctx context.Context, handle RemoteUsageHandle, observation *agentruntime.RemoteUsageObservation) error {
	req := handle.request
	if observation != nil {
		if observation.Service != "" && observation.Service != req.Service {
			return ErrRemoteUsageInvalidRequest
		}
		if observation.PriceVersion != "" && observation.PriceVersion != req.PriceVersion {
			return ErrRemoteUsageInvalidRequest
		}
		// Revision and billable dimensions are part of the server-owned fence.
		// A provider may repeat them for correlation, but cannot replace them
		// with a cheaper or differently priced fact. Quantity corrections must
		// arrive through a new server-admitted revision.
		if observation.Revision > 0 && observation.Revision != req.Revision {
			return ErrRemoteUsageInvalidRequest
		}
		if observation.Dimensions != nil && !sameDimensions(observation.Dimensions, req.Dimensions) {
			return ErrRemoteUsageInvalidRequest
		}
		if !observation.OccurredAt.IsZero() {
			req.OccurredAt = observation.OccurredAt
		}
		if observation.Status != "" {
			req.Status = observation.Status
		}
	}
	if req.Funding == commercial.FundingBYOK && req.Service == commercial.ServiceModel {
		return s.finishBound(ctx, "", req)
	}
	if handle.reservation.ID == "" {
		return ErrRemoteUsageInvalidRequest
	}
	return s.finishBound(ctx, handle.reservation.ID, req)
}

// RemoteUsageService is the single application seam for remote usage. It
// deliberately accepts no client supplied UsageFact: facts are built from
// the server side request and trusted provider observation at this boundary.
type remoteBudgetTree interface {
	AttachChildRun(context.Context, uint64, string, string) error
}

type RemoteUsageService struct {
	gate   commercial.ExecutionGate
	budget remoteBudgetTree
}

func NewRemoteUsageService(gate commercial.ExecutionGate) (*RemoteUsageService, error) {
	if gate == nil {
		return nil, errors.New("remote_usage_gate_missing")
	}
	s := &RemoteUsageService{gate: gate}
	if tree, ok := gate.(remoteBudgetTree); ok {
		s.budget = tree
	}
	return s, nil
}

// NewRemoteUsageServiceWithDB is the container constructor. It connects the
// same database scoped budget tree used by ExecutionGate, so child admission
// and reservation cannot drift onto separate stores.
func NewRemoteUsageServiceWithDB(gate commercial.ExecutionGate, db *gorm.DB) (*RemoteUsageService, error) {
	s, err := NewRemoteUsageService(gate)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, errors.New("remote_usage_budget_database_missing")
	}
	s.budget = repocommercial.NewBudgetStore(db)
	return s, nil
}

// WithBudgetTree connects child-run admission to the durable task budget.
// The gate remains the single reservation boundary; this seam only records
// the parent/root mapping before that boundary is entered.
func (s *RemoteUsageService) WithBudgetTree(tree remoteBudgetTree) *RemoteUsageService {
	s.budget = tree
	return s
}

func (s *RemoteUsageService) Begin(ctx context.Context, req RemoteUsageRequest) (commercial.Reservation, error) {
	// The public request shape is untrusted. Only BeginRemote, which builds
	// the binding from the server-owned worker fence, may enter the gate.
	return commercial.Reservation{}, ErrRemoteUsageUntrustedSource
}

func (s *RemoteUsageService) beginBound(ctx context.Context, req RemoteUsageRequest) (commercial.Reservation, error) {
	if err := validateRemoteIdentity(req); err != nil {
		return commercial.Reservation{}, err
	}
	if req.Upper <= 0 || req.Deadline.IsZero() {
		return commercial.Reservation{}, ErrRemoteUsageInvalidRequest
	}
	if !trustedSource(req.Source) {
		return commercial.Reservation{}, ErrRemoteUsageUntrustedSource
	}
	if err := commercial.ValidateFunding(req.Funding); err != nil {
		return commercial.Reservation{}, err
	}
	// A BYOK model call has no platform model charge and must not create a
	// second model reservation. Sandbox/connector/parsing work remains gated.
	if req.Service == commercial.ServiceModel && !execution.AllowModelSettlement(req.Source, req.Funding) {
		if req.Funding == commercial.FundingBYOK {
			return commercial.Reservation{}, nil
		}
		return commercial.Reservation{}, ErrRemoteUsageUntrustedSource
	}
	stable := stableUsageKey(req)
	if req.Key != "" && req.Key != stable {
		return commercial.Reservation{}, fmt.Errorf("%w: usage key does not match physical identity", ErrRemoteUsageInvalidRequest)
	}
	if req.ParentRunID != "" {
		if s.budget == nil {
			return commercial.Reservation{}, fmt.Errorf("%w: child budget binding unavailable", ErrRemoteUsageInvalidRequest)
		}
		if err := s.budget.AttachChildRun(ctx, req.TenantID, req.RunID, req.ParentRunID); err != nil {
			return commercial.Reservation{}, err
		}
	}
	return s.gate.Begin(ctx, commercial.BudgetRequest{TenantID: req.TenantID, RunID: req.RunID, Key: stable, Upper: req.Upper, Deadline: req.Deadline})
}

func (s *RemoteUsageService) Finish(ctx context.Context, reservationID string, req RemoteUsageRequest) error {
	return ErrRemoteUsageUntrustedSource
}

func (s *RemoteUsageService) finishBound(ctx context.Context, reservationID string, req RemoteUsageRequest) error {
	if err := validateRemoteIdentity(req); err != nil {
		return err
	}
	if !trustedSource(req.Source) {
		return ErrRemoteUsageUntrustedSource
	}
	if err := commercial.ValidateFunding(req.Funding); err != nil {
		return err
	}
	if req.Status != commercial.UsageStatusFinal {
		return ErrRemoteUsageNotBillable
	}
	if req.Service == commercial.ServiceModel && !execution.AllowModelSettlement(req.Source, req.Funding) {
		if req.Funding == commercial.FundingBYOK {
			// BYOK model calls intentionally have no reservation. Their paired
			// Finish is a safe no-op, so the lifecycle cannot turn into a
			// second platform charge.
			return nil
		}
		return ErrRemoteUsageUntrustedSource
	}
	if reservationID == "" {
		return ErrRemoteUsageInvalidRequest
	}
	stable := stableUsageKey(req)
	if reservationID != stable {
		return fmt.Errorf("%w: reservation does not match physical identity", ErrRemoteUsageInvalidRequest)
	}
	fact := commercial.UsageFact{TenantID: req.TenantID, RunID: req.RunID, DelegationID: req.DelegationID, CallID: req.CallID, AttemptID: req.AttemptID, Funding: req.Funding, Service: req.Service, PriceVersion: req.PriceVersion, Revision: req.Revision, OccurredAt: req.OccurredAt, Dimensions: req.Dimensions, Status: req.Status}
	return s.gate.Finish(ctx, reservationID, fact)
}

func trustedSource(source string) bool { return strings.TrimSpace(source) == "platform_gateway" }

func validateRemoteIdentity(req RemoteUsageRequest) error {
	if req.TenantID == 0 || req.RunID == "" || req.CallID == "" || req.AttemptID == "" || req.Service == "" || req.PriceVersion == "" || req.Revision <= 0 || req.Dimensions == nil || req.OccurredAt.IsZero() {
		return ErrRemoteUsageInvalidRequest
	}
	for _, n := range req.Dimensions {
		if n < 0 {
			return fmt.Errorf("%w: negative dimension", ErrRemoteUsageInvalidRequest)
		}
	}
	return nil
}

func stableUsageKey(req RemoteUsageRequest) string {
	return fmt.Sprintf("%s:%s:%d", req.CallID, req.AttemptID, req.Revision)
}

func sameDimensions(a, b map[string]int64) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

func cloneDimensions(in map[string]int64) map[string]int64 {
	if in == nil {
		return nil
	}
	out := make(map[string]int64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
