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
	if fence.TenantID == 0 || fence.RunID == "" || commandID == "" {
		return RemoteUsageHandle{}, ErrRemoteUsageInvalidRequest
	}
	req := RemoteUsageRequest{
		TenantID: fence.TenantID, RunID: fence.RunID, CallID: commandID,
		AttemptID: commandID, Upper: 1, Deadline: time.Now().UTC().Add(10 * time.Minute),
		Source: "platform_gateway", Funding: commercial.FundingPlatform,
		Service: commercial.ServiceConnector, PriceVersion: "remote-v1", Revision: 1,
		OccurredAt: time.Now().UTC(), Dimensions: map[string]int64{commercial.DimensionConnector: 1},
		Status: commercial.UsageStatusFinal,
	}
	res, err := s.beginBound(ctx, req)
	if err != nil {
		return RemoteUsageHandle{}, err
	}
	return RemoteUsageHandle{reservation: res, request: req}, nil
}

func (s *RemoteUsageService) FinishRemote(ctx context.Context, handle RemoteUsageHandle) error {
	if handle.reservation.ID == "" {
		return ErrRemoteUsageInvalidRequest
	}
	return s.finishBound(ctx, handle.reservation.ID, handle.request)
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
