package commercial

import (
	"context"
	"errors"
)

// Billable outbound services. Each service dimension is gated by its OWN
// settlement dimension (ServiceDimension); a call may never bill a service
// through another service's dimension.
const (
	ServiceModel     = "model"
	ServiceParsing   = "parsing"
	ServiceEmbedding = "embedding"
	ServiceRerank    = "rerank"
	ServiceSandbox   = "sandbox"
	ServiceConnector = "connector"
)

// Per-service settlement dimensions. The model/parsing/sandbox names reuse
// the U01 dimension vocabulary; embedding/rerank/connector and the
// output/duration bound dimensions extend it for their own services.
const (
	DimensionEmbedding = "embedding_tokens"
	DimensionRerank    = "rerank_units"
	DimensionConnector = "connector_calls"
	DimensionOutput    = "output_tokens"
	DimensionDuration  = "duration_seconds"
)

var (
	// ErrUnknownService rejects a billable call whose service has no gating
	// dimension: it can never ride an existing dimension for free.
	ErrUnknownService = errors.New("unknown_service")
	// ErrUntrustedUsage rejects usage whose funding was not derived by the
	// server-side credential binding (client-supplied funding).
	ErrUntrustedUsage = errors.New("untrusted_usage")
	// ErrNotBillable rejects facts that are not final billable deltas
	// (display_only rollups, streaming partials, unknown observations).
	ErrNotBillable = errors.New("not_billable")
	// ErrOverLimit enforces the hard model-output / sandbox-duration upper
	// bounds at the gate.
	ErrOverLimit = errors.New("over_limit")
	// ErrAbnormalCost marks a settle above the reserved upper bound: the
	// run must stop further dispatch instead of continuing to spend.
	ErrAbnormalCost = errors.New("abnormal_cost")
	// ErrInsufficientBudgetGate is the gate-level denial a dispatcher sees
	// when the budget store rejects a hold. The store's own denial errors
	// are wrapped into it so callers can classify "denied, stop" in one
	// check regardless of the underlying projection reason.
	ErrInsufficientBudgetGate = errors.New("insufficient_budget_gate")
)

// ServiceDimension returns the settlement dimension that gates one billable
// service. BYOK waives ONLY the model dimension (BillableModel); every other
// service stays billable under its own dimension regardless of funding.
func ServiceDimension(service string) (string, error) {
	switch service {
	case ServiceModel:
		return DimensionModel, nil
	case ServiceParsing:
		return DimensionParsing, nil
	case ServiceEmbedding:
		return DimensionEmbedding, nil
	case ServiceRerank:
		return DimensionRerank, nil
	case ServiceSandbox:
		return DimensionSandbox, nil
	case ServiceConnector:
		return DimensionConnector, nil
	}
	return "", ErrUnknownService
}

// ExecutionLimits are the hard upper bounds enforced at the billable
// boundary. A zero value disables a bound; configured bounds are rejections,
// never advisories.
type ExecutionLimits struct {
	MaxModelOutputTokens int64
	MaxSandboxSeconds    int64
}

// Enforce rejects a usage fact whose model output tokens or sandbox duration
// exceed the configured bounds. Over-cap facts are rejected at Finish, so
// the provider answer never turns into an uncapped settlement.
func (l ExecutionLimits) Enforce(fact UsageFact) error {
	if l.MaxModelOutputTokens > 0 && fact.Service == ServiceModel &&
		fact.Dimensions[DimensionOutput] > l.MaxModelOutputTokens {
		return ErrOverLimit
	}
	if l.MaxSandboxSeconds > 0 && fact.Service == ServiceSandbox &&
		fact.Dimensions[DimensionDuration] > l.MaxSandboxSeconds {
		return ErrOverLimit
	}
	return nil
}

// ExecutionGate is the billable outbound boundary every commercial dispatch
// must pass through. Begin runs BEFORE the real outbound call (it reserves
// budget and persists the dispatched intent); Finish runs after trusted
// provider usage returns and settles exactly the final delta. A nil gate
// preserves non-commercial behavior; a configured gate makes denial a hard
// stop for dispatch.
type ExecutionGate interface {
	Begin(ctx context.Context, req BudgetRequest) (Reservation, error)
	Finish(ctx context.Context, reservationID string, fact UsageFact) error
}

// TrustedUsageFact reports whether a fact may settle a reservation: the
// funding must come from the server-recognized vocabulary (ValidateFunding
// rejects client-supplied funding), and the status must be a final billable
// delta. display_only parent aggregates of child calls are rejected here —
// the children already settled their own facts, so settling the rollup too
// would double-count the run.
func TrustedUsageFact(fact UsageFact) error {
	if err := ValidateFunding(fact.Funding); err != nil {
		return ErrUntrustedUsage
	}
	if fact.Status != UsageStatusFinal {
		return ErrNotBillable
	}
	return nil
}

// CheckAbnormal flags a settled charge above the reservation's reserved
// upper bound. The settle itself stays recorded (spend stays protected),
// but the dispatcher must treat the run as abnormal and stop further
// dispatch rather than keep drawing on a mis-projected budget.
func CheckAbnormal(charged, reservedUpper Credits) error {
	if charged > reservedUpper {
		return ErrAbnormalCost
	}
	return nil
}
