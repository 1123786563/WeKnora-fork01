package workbench

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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

// RemoteUsageService is the single application seam for remote usage. It
// deliberately accepts no client supplied UsageFact: facts are built from
// the server side request and trusted provider observation at this boundary.
type RemoteUsageService struct{ gate commercial.ExecutionGate }

func NewRemoteUsageService(gate commercial.ExecutionGate) (*RemoteUsageService, error) {
	if gate == nil {
		return nil, errors.New("remote_usage_gate_missing")
	}
	return &RemoteUsageService{gate: gate}, nil
}

func (s *RemoteUsageService) Begin(ctx context.Context, req RemoteUsageRequest) (commercial.Reservation, error) {
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
	if req.Key == "" {
		req.Key = stableUsageKey(req)
	}
	return s.gate.Begin(ctx, commercial.BudgetRequest{TenantID: req.TenantID, RunID: req.RunID, Key: req.Key, Upper: req.Upper, Deadline: req.Deadline})
}

func (s *RemoteUsageService) Finish(ctx context.Context, reservationID string, req RemoteUsageRequest) error {
	if reservationID == "" {
		return ErrRemoteUsageInvalidRequest
	}
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
			return nil
		}
		return ErrRemoteUsageUntrustedSource
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
