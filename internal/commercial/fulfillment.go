package commercial

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
)

// Benefit kinds select the verified settlement path of a fulfillment line.
// Subscription grants and one-off credit top-ups never share a settlement
// identity, so the two kinds can never double-settle the same purchase.
const (
	BenefitKindSubscription = "subscription"
	BenefitKindTopUp        = "top_up"
)

// Fulfillment record states. applied means an explicit external receipt is on
// file; attention marks an indeterminate outcome (e.g. a timeout after the
// remote may have persisted) that a later pass must reconcile; refused marks a
// definitive business rejection, which is a failure, never a success.
const (
	FulfillmentStateApplied   = "applied"
	FulfillmentStateAttention = "attention"
	FulfillmentStateRefused   = "refused"
)

var (
	ErrInvalidBenefitRequest  = errors.New("invalid_benefit_request")
	ErrBenefitNotFound        = errors.New("benefit_not_found")
	ErrGatewayBusinessRefusal = errors.New("gateway_business_refusal")
	ErrGatewayIndeterminate   = errors.New("gateway_indeterminate")
	ErrGatewayUnconfigured    = errors.New("gateway_unconfigured")
)

// FulfillmentKey derives the stable, collision-free idempotency identity of
// one order line's benefit. The NUL separator keeps (order, line) pairs
// unambiguous, the hash hides order internals, and the fulfill: prefix keeps
// the key namespace separate from MonthlyGrantKey's.
func FulfillmentKey(orderID, lineID string) string {
	sum := sha256.Sum256([]byte(orderID + "\x00" + lineID))
	return "fulfill:" + hex.EncodeToString(sum[:])
}

// BenefitRequest is one benefit line to grant through the commercial gateway.
type BenefitRequest struct {
	Key         string
	TenantID    uint64
	CustomerID  string
	Kind        string
	PlanRef     string
	Credits     Credits
	EffectiveAt time.Time
	ExpiresAt   time.Time
}

// Validate enforces the gateway contract: a keyed, addressed, positive,
// effective grant. Tenant comes from the order, never from the calling user.
func (r BenefitRequest) Validate() error {
	if r.Key == "" || r.TenantID == 0 || r.CustomerID == "" || r.Credits <= 0 || r.EffectiveAt.IsZero() {
		return ErrInvalidBenefitRequest
	}
	switch r.Kind {
	case BenefitKindSubscription, BenefitKindTopUp:
	default:
		return ErrInvalidBenefitRequest
	}
	return nil
}

// BenefitReceipt is the external proof of one applied benefit. ExternalID is
// the provider settlement identity; an empty receipt is never a success.
type BenefitReceipt struct {
	ExternalID  string
	EffectiveAt time.Time
}

// CommercialGateway issues and discovers benefits at the commercial provider.
// ApplyBenefit must be idempotent on Request.Key; FindBenefit discovers a
// benefit that may have persisted even when its ApplyBenefit response was
// lost, which is what makes recovery possible without duplicate grants.
type CommercialGateway interface {
	ApplyBenefit(ctx context.Context, req BenefitRequest) (BenefitReceipt, error)
	FindBenefit(ctx context.Context, key string) (BenefitReceipt, error)
	// RevokeBenefit claws back EXACTLY credits from the benefit settled
	// under key (C05 precise-credits revocation). It must be idempotent on
	// (key, credits): a replay after a lost response never revokes twice,
	// and an unconfirmed revocation must surface as an error so the refund
	// stays revocation_pending and recovery retries the revocation only —
	// never a second payout.
	RevokeBenefit(ctx context.Context, key string, credits Credits) error
	// Settle reports one call's final consumption to the provider under
	// Settlement.ID — the revision-derived idempotency key — so a replay of
	// the same revision (lost response, retry, crash recovery) can never
	// double-settle. Ingest acceptance alone is NOT confirmation: a receipt
	// proves only that the provider recorded the transaction.
	Settle(ctx context.Context, s Settlement) (SettlementReceipt, error)
	// ConfirmSettlement resolves the explicit confirmation of a previously
	// settled transaction. A result counts as confirmed ONLY when it
	// carries event/transaction correlation evidence (external id plus the
	// watermark it advanced); a balance drop is never evidence. Unknown
	// outcomes stay with the caller for reconciliation.
	ConfirmSettlement(ctx context.Context, settlementID string) (SettlementReceipt, error)
}

// FulfillmentOutcome classifies a gateway result for state transitions.
type FulfillmentOutcome int

const (
	// FulfillmentApplied means a receipt is on hand: the line is fulfilled.
	FulfillmentApplied FulfillmentOutcome = iota
	// FulfillmentMissing means the provider provably has no such benefit;
	// applying is safe.
	FulfillmentMissing
	// FulfillmentRefused means the provider definitively rejected the grant.
	// This is a business failure and must never be recorded as success.
	FulfillmentRefused
	// FulfillmentUnknown means the outcome cannot be determined (timeout,
	// lost response, unreadable answer). The line goes to attention and is
	// reconciled by FindBenefit on a later pass; it is never success and
	// never a licence to blindly re-apply.
	FulfillmentUnknown
)

// ClassifyFulfillment maps a gateway error onto an outcome. Unknown transport
// errors default to indeterminate: recording an unprovable outcome as either
// success or definitive refusal would fabricate evidence.
func ClassifyFulfillment(err error) FulfillmentOutcome {
	switch {
	case err == nil:
		return FulfillmentApplied
	case errors.Is(err, ErrBenefitNotFound):
		return FulfillmentMissing
	case errors.Is(err, ErrGatewayBusinessRefusal):
		return FulfillmentRefused
	default:
		return FulfillmentUnknown
	}
}

// FirstEffectiveAt pins a top-up's effective time on its first success and
// reuses it verbatim on every retry, so a recovered grant never shifts the
// interval it covers.
func FirstEffectiveAt(fixed, retry time.Time) time.Time {
	if !fixed.IsZero() {
		return fixed
	}
	return retry
}

// benefitCustomerContextKey carries the provider customer the lookup of a
// fulfillment key is scoped to; gateways read it in FindBenefit.
type benefitCustomerContextKey struct{}

// WithBenefitCustomer scopes ctx to a provider customer for FindBenefit. The
// worker derives the customer from the order, not from the calling user.
func WithBenefitCustomer(ctx context.Context, customerID string) context.Context {
	return context.WithValue(ctx, benefitCustomerContextKey{}, customerID)
}

// BenefitCustomerFrom returns the customer scoped onto ctx, if any.
func BenefitCustomerFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(benefitCustomerContextKey{}).(string)
	return v
}
