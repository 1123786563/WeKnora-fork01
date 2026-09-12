package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
)

// commercialGateCallTTL bounds how long one round's reservation may stay
// held before settlement or expiry resolves it.
const commercialGateCallTTL = 15 * time.Minute

// CommercialGateBinding wires ONE billable AgentEngine turn to the
// commercial ExecutionGate. TenantID/RunID/Funding are resolved server-side
// (never from model arguments); Funding that fails commercial.ValidateFunding
// is a HARD error — a BYOK binding that stops validating must never fall
// back to platform funding silently.
//
// runtime_binding INCOMPLETE (honest ledger note): this adapter binds only
// the CURRENT AgentEngine entry point. The Craft/OpenCode child-runtime
// adapters do not exist yet; opening a child runtime for billed dispatch
// requires its own Begin/Finish evidence at its outbound model/tool/sandbox
// control points, per spec. No fake same-named module is created here.
type CommercialGateBinding struct {
	Gate         commercial.ExecutionGate
	TenantID     uint64
	RunID        string
	DelegationID string
	Funding      string
	PriceVersion string
	Upper        commercial.Credits
	// halted is set when Finish reports abnormal cost: every later Begin of
	// this binding fails fast so abnormal cost stops further dispatch.
	halted       bool
	reservation  commercial.Reservation
	attemptCount int
}

// SetCommercialGate arms the engine's billable boundary. A nil binding (or a
// binding with a nil Gate) preserves the engine's existing non-commercial
// behavior exactly — the hooks below are no-ops then.
func (e *AgentEngine) SetCommercialGate(b *CommercialGateBinding) {
	e.commercialGate = b
}

// beginBillableCall runs BEFORE the real outbound model call. Gate denial is
// a hard stop: the round errors out instead of dispatching ungated.
func (e *AgentEngine) beginBillableCall(ctx context.Context, round int) error {
	b := e.commercialGate
	if b == nil || b.Gate == nil {
		return nil
	}
	if b.halted {
		return commercial.ErrAbnormalCost
	}
	if err := commercial.ValidateFunding(b.Funding); err != nil {
		logger.Warnf(ctx, "[Agent][Commercial] refusing dispatch: untrusted funding")
		return err
	}
	if b.Upper <= 0 {
		return commercial.ErrInsufficientBudgetGate
	}
	b.attemptCount++
	req := commercial.BudgetRequest{
		TenantID: b.TenantID,
		RunID:    b.RunID,
		Key:      fmt.Sprintf("%s:round:%d:%d", b.RunID, round, b.attemptCount),
		Upper:    b.Upper,
		Deadline: time.Now().Add(commercialGateCallTTL),
	}
	res, err := b.Gate.Begin(ctx, req)
	if err != nil {
		logger.Warnf(ctx, "[Agent][Commercial] gate denied round %d: %v", round, err)
		return err
	}
	b.reservation = res
	return nil
}

// finishBillableCall runs after trusted provider usage returns for the round.
// Display TokenUsage semantics are untouched; the adaptation is additive.
func (e *AgentEngine) finishBillableCall(ctx context.Context, round int, usage types.TokenUsage) {
	b := e.commercialGate
	if b == nil || b.Gate == nil || b.reservation.ID == "" || usage.TotalTokens == 0 {
		return
	}
	fact := chat.UsageFactFromTokenUsage(usage, chat.UsageFactInput{
		TenantID:     b.TenantID,
		RunID:        b.RunID,
		DelegationID: b.DelegationID,
		CallID:       fmt.Sprintf("%s:round:%d", b.RunID, round),
		AttemptID:    b.reservation.ID,
		Funding:      b.Funding,
		Service:      commercial.ServiceModel,
		PriceVersion: b.PriceVersion,
	}, time.Now().UTC())
	if err := b.Gate.Finish(ctx, b.reservation.ID, fact); err != nil {
		logger.Warnf(ctx, "[Agent][Commercial] gate finish failed for round %d: %v", round, err)
		if err == commercial.ErrAbnormalCost {
			b.halted = true
		}
	}
	b.reservation = commercial.Reservation{}
}
