package commercial

import (
	"errors"
	"math/big"
	"time"
)

// Funding sources for a physical model call. Funding is produced by the
// server-side credential binding, never by the caller's claim; anything the
// binding does not recognize is rejected before dispatch.
const (
	FundingPlatform = "platform"
	FundingBYOK     = "byok"
)

// Usage fact statuses. Streaming partials and status=unknown are stored as
// observations only; display_only marks parent aggregates that merely roll
// up child calls; a final fact is the one billable delta of the call.
const (
	UsageStatusFinal       = "final"
	UsageStatusPartial     = "partial"
	UsageStatusUnknown     = "unknown"
	UsageStatusDisplayOnly = "display_only"
)

// Dimension names. BYOK waives ONLY the model dimension; sandbox and
// parsing remain independent billable service facts.
const (
	DimensionModel   = "model"
	DimensionSandbox = "sandbox"
	DimensionParsing = "parsing"
)

var (
	ErrUnknownFunding       = errors.New("unknown_funding")
	ErrInvalidUsageFact     = errors.New("invalid_usage_fact")
	ErrUnknownDimensionRate = errors.New("unknown_dimension_rate")
)

// BillableModel reports whether the MODEL dimension of a call is billable
// under the given funding source. Only server-side credential binding may
// produce funding; ValidateFunding rejects unknown sources before dispatch
// so this predicate never becomes a silent free ride.
func BillableModel(funding string) bool { return funding == FundingPlatform }

// ValidateFunding accepts only the funding sources the server-side
// credential binding can actually produce. Unknown funding must be
// rejected here, before dispatch, instead of being recorded as unpaid.
func ValidateFunding(funding string) error {
	switch funding {
	case FundingPlatform, FundingBYOK:
		return nil
	default:
		return ErrUnknownFunding
	}
}

// UsageFact is one physical usage fact tied to a single attempt of a single
// call. Revisions are append-only: a late correction arrives as a new
// revision and moves the current-version pointer, never as an in-place edit
// of an existing revision.
type UsageFact struct {
	TenantID     uint64
	RunID        string
	DelegationID string
	CallID       string
	AttemptID    string
	Funding      string
	Service      string
	PriceVersion string
	Revision     int64
	OccurredAt   time.Time
	Dimensions   map[string]int64
	Status       string
}

// Validate enforces the fact invariants: identity, a server-recognized
// funding source, a known status, and non-negative dimension quantities.
func (f UsageFact) Validate() error {
	if f.TenantID == 0 || f.CallID == "" || f.AttemptID == "" || f.Service == "" ||
		f.PriceVersion == "" || f.Revision <= 0 || f.Dimensions == nil {
		return ErrInvalidUsageFact
	}
	if err := ValidateFunding(f.Funding); err != nil {
		return err
	}
	switch f.Status {
	case UsageStatusFinal, UsageStatusPartial, UsageStatusUnknown, UsageStatusDisplayOnly:
	default:
		return ErrInvalidUsageFact
	}
	for _, v := range f.Dimensions {
		if v < 0 {
			return ErrInvalidUsageFact
		}
	}
	return nil
}

// BillableDimensions returns the dimensions of the fact that generate
// settlement. BYOK waives only the model dimension: sandbox and parsing
// are independent billable service facts and survive the waiver.
func (f UsageFact) BillableDimensions() map[string]int64 {
	out := make(map[string]int64, len(f.Dimensions))
	for k, v := range f.Dimensions {
		if k == DimensionModel && !BillableModel(f.Funding) {
			continue
		}
		out[k] = v
	}
	return out
}

// DimensionRate is the fixed-point price of one dimension under an
// immutable price version: RateMicro micro-credits per Units units of the
// dimension (e.g. 1500 micro-credits per 1000 model tokens). Units must be
// positive; rates are stored, never derived at read time.
type DimensionRate struct {
	RateMicro int64
	Units     int64
}

// PriceVersionRates binds an immutable price version to its per-dimension
// fixed-point rates. Publishing a new version never edits an old one.
type PriceVersionRates struct {
	Version string
	Rates   map[string]DimensionRate
}

// Validate enforces the fixed-point invariants of the version's rates.
func (p PriceVersionRates) Validate() error {
	if p.Version == "" {
		return ErrInvalidUsageFact
	}
	for _, r := range p.Rates {
		if r.Units <= 0 || r.RateMicro < 0 {
			return ErrInvalidUsageFact
		}
	}
	return nil
}

// ChargeRat returns the exact, unrounded micro-credit amount for one
// dimension usage as a rational number. All multiplication and division
// stays exact in big.Rat; no intermediate rounding is performed.
func (p PriceVersionRates) ChargeRat(dimension string, quantity int64) (*big.Rat, error) {
	rate, ok := p.Rates[dimension]
	if !ok {
		return nil, ErrUnknownDimensionRate
	}
	return new(big.Rat).SetFrac(
		new(big.Int).Mul(big.NewInt(rate.RateMicro), big.NewInt(quantity)),
		big.NewInt(rate.Units),
	), nil
}

// ChargeForCall prices one physical call: it sums the fact's billable
// dimensions at the version's fixed-point rates and rounds exactly ONCE,
// half away from zero, at the end of the physical call. This single
// rounding point is the contract U03 (settlement) consumes — per-dimension
// or per-partial rounding is never performed here.
func (p PriceVersionRates) ChargeForCall(fact UsageFact) (Credits, error) {
	if err := p.Validate(); err != nil {
		return 0, err
	}
	total := new(big.Rat)
	for dimension, quantity := range fact.BillableDimensions() {
		charge, err := p.ChargeRat(dimension, quantity)
		if err != nil {
			return 0, err
		}
		total.Add(total, charge)
	}
	return Credits(roundHalfAwayFromZero(total)), nil
}

// roundHalfAwayFromZero rounds an exact rational to an integer once.
func roundHalfAwayFromZero(r *big.Rat) int64 {
	q, rem := new(big.Int).QuoRem(r.Num(), r.Denom(), new(big.Int))
	twice := new(big.Int).Abs(rem)
	twice.Lsh(twice, 1)
	if twice.CmpAbs(r.Denom()) >= 0 {
		if r.Num().Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	return q.Int64()
}
