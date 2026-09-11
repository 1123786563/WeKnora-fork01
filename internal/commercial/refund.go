package commercial

import (
	"context"
	"errors"
)

// Refund lifecycle states. A request starts requested; manual review moves
// it to reviewing; approval (only with a configured eligibility policy)
// holds locks and starts the channel op in pending. Channel success is
// NEVER terminal by itself: the refund goes to revocation_pending and only
// a confirmed precise-credits revocation completes it. failed_confirmed
// and not_created_confirmed are the only states whose locks may release.
const (
	RefundStateRequested           = "requested"
	RefundStateReviewing           = "reviewing"
	RefundStatePending             = "pending"
	RefundStateRevocationPending   = "revocation_pending"
	RefundStateCompleted           = "completed"
	RefundStateFailedConfirmed     = "failed_confirmed"
	RefundStateNotCreatedConfirmed = "not_created_confirmed"
)

// Channel (provider-side) refund outcome vocabulary, aligned with the C02
// Provider.Refund/QueryRefund result states.
const (
	RefundChannelPending    = "pending"
	RefundChannelUnknown    = "unknown"
	RefundChannelSucceeded  = "succeeded"
	RefundChannelFailed     = "failed"
	RefundChannelNotCreated = "not_created"
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

// CanUnlockRefund reports whether a refund outcome releases its locked
// credits: only a CONFIRMED failure or a CONFIRMED never-created refund
// unlocks. Success, in-flight, and indeterminate outcomes keep the lock —
// unrecognised values fail closed.
func CanUnlockRefund(channelState string) bool {
	return channelState == "failed_confirmed" || channelState == "not_created_confirmed"
}

// TransitionRefundOnChannel maps the stored refund state onto the next
// state after one channel outcome. Terminal states never move again; a
// still-processing channel keeps the current state; an unproven outcome
// falls back to reviewing so recovery re-queries the ORIGINAL refund key
// instead of forcing a progression.
func TransitionRefundOnChannel(current, channelState string) string {
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
func TransitionRefundOnRevocation(current string, confirmed bool) string {
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
	State        string
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
