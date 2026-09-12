package commercial

import (
	"context"
	"errors"
)

// RefundState is the refund lifecycle vocabulary. It is a DISTINCT type
// from the channel payout states (RefundChannelState) and from the
// payment attempt states (payment.AttemptState): the compiler rejects
// feeding a payment or channel state into a lifecycle transition.
type RefundState string

// String keeps the raw wire value available where persistence, SQL or
// operator-facing projections need the plain string.
func (s RefundState) String() string { return string(s) }

// Refund lifecycle states. A request starts requested; manual review moves
// it to reviewing; approval (only with a configured eligibility policy)
// holds locks and starts the channel op in pending. Channel success is
// NEVER terminal by itself: the refund goes to revocation_pending and only
// a confirmed precise-credits revocation completes it. failed_confirmed
// and not_created_confirmed are the only states whose locks may release.
const (
	RefundStateRequested           RefundState = "requested"
	RefundStateReviewing           RefundState = "reviewing"
	RefundStatePending             RefundState = "pending"
	RefundStateRevocationPending   RefundState = "revocation_pending"
	RefundStateCompleted           RefundState = "completed"
	RefundStateFailedConfirmed     RefundState = "failed_confirmed"
	RefundStateNotCreatedConfirmed RefundState = "not_created_confirmed"
)

// RefundChannelState is the channel (provider-side) refund outcome
// vocabulary, aligned with the C02 Provider.Refund/QueryRefund result
// states. Distinct from RefundState and payment.AttemptState.
type RefundChannelState string

const (
	RefundChannelPending    RefundChannelState = "pending"
	RefundChannelUnknown    RefundChannelState = "unknown"
	RefundChannelSucceeded  RefundChannelState = "succeeded"
	RefundChannelFailed     RefundChannelState = "failed"
	RefundChannelNotCreated RefundChannelState = "not_created"
)

// Manual review bases recorded for every review decision: whether the
// period being refunded had not started yet or was already effective.
const (
	RefundBasisPeriodNotStarted = "period_not_started"
	RefundBasisPeriodEffective  = "period_effective"
)

var (
	// ErrRefundNotReady reports that the P03 occupancy-refundable check is
	// unavailable (or refused): the refund stays requested/reviewing and is
	// never approved into a paid-out state.
	ErrRefundNotReady = errors.New("refund_eligibility_not_ready")
)

// CanUnlockRefund reports whether a refund LIFECYCLE state releases its
// locked credits: only a CONFIRMED failure or a CONFIRMED never-created
// refund unlocks. Success, in-flight, and indeterminate outcomes keep the
// lock — unrecognised values fail closed. The parameter is deliberately
// the lifecycle type: deciding an unlock from a channel or payment state
// is now a compile error, not a silent fail-open.
func CanUnlockRefund(state RefundState) bool {
	return state == RefundStateFailedConfirmed || state == RefundStateNotCreatedConfirmed
}

// TransitionRefundOnChannel maps the stored refund state onto the next
// state after one channel outcome. Terminal states never move again; a
// still-processing channel keeps the current state; an unproven outcome
// falls back to reviewing so recovery re-queries the ORIGINAL refund key
// instead of forcing a progression.
func TransitionRefundOnChannel(current RefundState, channelState RefundChannelState) RefundState {
	switch current {
	case RefundStateCompleted, RefundStateFailedConfirmed, RefundStateNotCreatedConfirmed:
		return current
	}
	switch channelState {
	case RefundChannelSucceeded:
		// Channel payout alone never completes a refund: precise-credits
		// revocation must still be confirmed.
		return RefundStateRevocationPending
	case RefundChannelFailed:
		return RefundStateFailedConfirmed
	case RefundChannelNotCreated:
		return RefundStateNotCreatedConfirmed
	case RefundChannelPending:
		return current
	default:
		return RefundStateReviewing
	}
}

// TransitionRefundOnRevocation completes a refund ONLY on a confirmed
// precise-credits revocation; a failed revocation stays
// revocation_pending so recovery retries the revocation and never pays
// out a second time.
func TransitionRefundOnRevocation(current RefundState, confirmed bool) RefundState {
	if current == RefundStateRevocationPending && confirmed {
		return RefundStateCompleted
	}
	return current
}

// RefundRequestState is the produced refund projection: amounts are exact
// fen, credits are exact millionths of a Credit.
type RefundRequestState struct {
	ID           string
	OrderID      string
	State        RefundState
	TenantID     uint64
	Amount       CNYFen
	CreditAmount Credits
}

// RefundEligibilityChecker is the P03 seam: the budget coordinator's
// occupancy-refundable check that must gate every approval. Until P03
// lands, DefaultRefundEligibility refuses, so Approve keeps refunds in
// requested/reviewing with a recorded reason — never a paid-out state.
type RefundEligibilityChecker interface {
	RefundEligible(ctx context.Context, tenantID uint64, orderID string, credits Credits) error
}

type notReadyRefundEligibility struct{}

func (notReadyRefundEligibility) RefundEligible(context.Context, uint64, string, Credits) error {
	return ErrRefundNotReady
}

// DefaultRefundEligibility is the not-ready default (P03 not implemented).
var DefaultRefundEligibility RefundEligibilityChecker = notReadyRefundEligibility{}
