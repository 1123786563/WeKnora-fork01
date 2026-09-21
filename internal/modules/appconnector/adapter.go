package appconnector

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/commercial"
)

// ActionResult is the provider outcome of one connector call. State uses
// the Action lifecycle states; ExternalID is the provider's own id for the
// effect (empty when the provider has none); Output is the raw provider
// payload, never fabricated locally.
type ActionResult struct {
	State      string
	ExternalID string
	Output     json.RawMessage
}

// Adapter executes connector actions against one real provider. Execute
// performs the approved effect; Query asks the provider for the status of a
// previously dispatched action.
type Adapter interface {
	Execute(ctx context.Context, a Action) (ActionResult, error)
	Query(ctx context.Context, a Action) (ActionResult, error)
}

// QueryUnsupported is embedded by adapters whose provider offers no
// status-query API. Query then returns the HONEST unknown state with a nil
// error — an unsupported query must never fabricate a failure, because a
// fabricated failure would trigger a re-send of an effect that may already
// have happened. unknown resolves only through a provider query or human
// reconciliation.
type QueryUnsupported struct{}

// Query reports unknown.
func (QueryUnsupported) Query(ctx context.Context, a Action) (ActionResult, error) {
	return ActionResult{State: ActionUnknown}, nil
}

// DispatchIntent is the A03 execution boundary. ClaimDispatch consumes the
// approval and persists the queued→dispatched intent in one transaction
// BEFORE any real outbound call runs; the returned release reports the
// provider outcome so the action settles into succeeded/failed/unknown. A
// claim error parks the action — nothing is dispatched and nothing is
// billed.
type DispatchIntent interface {
	ClaimDispatch(ctx context.Context, a Action) (release func(outcome ActionResult, err error), err error)
}

// GatedAdapter wraps the real provider call BETWEEN the A03 intent and the
// U05 budget gate: intent claim first, then the budget reservation, only
// then the real call, and finally settlement plus release. A denial at
// either wrapper means the real call never runs.
type GatedAdapter struct {
	Intent DispatchIntent
	Gate   commercial.ExecutionGate
	Inner  Adapter
	// Budget derives the reservation request for one action. Nil uses a
	// default request bound to the action identity (one connector call).
	Budget func(ctx context.Context, a Action) commercial.BudgetRequest
	// Now is used for the reservation deadline (test hook).
	Now func() time.Time
}

func (g *GatedAdapter) budgetRequest(ctx context.Context, a Action) commercial.BudgetRequest {
	if g.Budget != nil {
		return g.Budget(ctx, a)
	}
	now := time.Now
	if g.Now != nil {
		now = g.Now
	}
	return commercial.BudgetRequest{
		TenantID: a.TenantID,
		RunID:    a.ID,
		Key:      a.Target,
		Upper:    1,
		Deadline: now().Add(10 * time.Minute),
	}
}

// Execute runs the guarded sequence. On intent denial the action stays
// parked (queued) with zero budget drawn. On budget denial the intent is
// released as not-dispatched and the caller sees the U05 gate error — the
// real call must not run, because unbilled spend is worse than a parked
// action. Provider errors settle the action per the outcome the inner
// adapter reports; release always runs exactly once after a successful
// claim.
func (g *GatedAdapter) Execute(ctx context.Context, a Action) (ActionResult, error) {
	release, err := g.Intent.ClaimDispatch(ctx, a)
	if err != nil {
		return ActionResult{State: ActionQueued}, err
	}
	req := g.budgetRequest(ctx, a)
	reservation, err := g.Gate.Begin(ctx, req)
	if err != nil {
		gateErr := fmt.Errorf("%w: %v", commercial.ErrInsufficientBudgetGate, err)
		release(ActionResult{State: ActionQueued}, gateErr)
		return ActionResult{State: ActionQueued}, gateErr
	}
	out, callErr := g.Inner.Execute(ctx, a)
	if finishErr := g.Gate.Finish(ctx, reservation.ID, commercial.UsageFact{
		TenantID:   a.TenantID,
		RunID:      a.ID,
		Funding:    commercial.FundingPlatform,
		Service:    commercial.ServiceConnector,
		Dimensions: map[string]int64{commercial.DimensionConnector: 1},
		Status:     commercial.UsageStatusFinal,
	}); finishErr != nil && callErr == nil {
		callErr = finishErr
	}
	release(out, callErr)
	return out, callErr
}

// Query delegates to the inner adapter's status query. When the provider
// does not support queries the inner adapter embeds QueryUnsupported and
// the honest unknown comes back — never a fabricated failure.
func (g *GatedAdapter) Query(ctx context.Context, a Action) (ActionResult, error) {
	if g.Inner == nil {
		return ActionResult{State: ActionUnknown}, nil
	}
	return g.Inner.Query(ctx, a)
}
