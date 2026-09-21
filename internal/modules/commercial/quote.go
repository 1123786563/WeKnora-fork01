package commercial

import (
	"errors"
	"math/big"
	"time"
)

var (
	ErrInvalidQuoteInterval = errors.New("invalid_quote_interval")
	ErrQuoteOverflow        = errors.New("quote_overflow")
	ErrQuoteExpired         = errors.New("quote_expired")
	ErrQuoteVersionConflict = errors.New("quote_version_conflict")
)

// Prorate returns delta*remaining/duration rounded once. It is used for the
// final rounding of a single quoted segment and for prorated monthly credit
// refills; multi-segment amounts must stay rational across segments and round
// once at the end via QuoteAmount instead of summing per-segment roundings.
func Prorate(delta, remaining, duration int64, roundUp bool) (int64, error) {
	if delta < 0 || remaining < 0 || duration <= 0 || remaining > duration {
		return 0, errors.New("invalid_quote_interval")
	}
	n := new(big.Int).Mul(big.NewInt(delta), big.NewInt(remaining))
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(n, big.NewInt(duration), r)
	if roundUp && r.Sign() != 0 {
		q.Add(q, big.NewInt(1))
	}
	if !q.IsInt64() {
		return 0, errors.New("quote_overflow")
	}
	return q.Int64(), nil
}

// Segment is one rational portion of a quoted amount: a per-period delta
// applied over Remaining out of Duration units of a paid period.
type Segment struct {
	Delta     int64
	Remaining int64
	Duration  int64
}

// QuoteAmount sums the segments as exact rationals and rounds the total once
// at the end. Rounding each segment up before summing would overcharge
// whenever the per-segment remainders cancel; the billed total must reflect
// the whole quoted interval, not the segments.
func QuoteAmount(segments []Segment, roundUp bool) (CNYFen, error) {
	if len(segments) == 0 {
		return 0, ErrInvalidQuoteInterval
	}
	total := new(big.Rat)
	for _, s := range segments {
		if s.Delta < 0 || s.Remaining < 0 || s.Duration <= 0 || s.Remaining > s.Duration {
			return 0, ErrInvalidQuoteInterval
		}
		num := new(big.Int).Mul(big.NewInt(s.Delta), big.NewInt(s.Remaining))
		total.Add(total, new(big.Rat).SetFrac(num, big.NewInt(s.Duration)))
	}
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(total.Num(), total.Denom(), r)
	if roundUp && r.Sign() != 0 {
		q.Add(q, big.NewInt(1))
	}
	if !q.IsInt64() {
		return 0, ErrQuoteOverflow
	}
	return CNYFen(q.Int64()), nil
}

// Quote is an offer to settle the remaining paid interval of a subscription:
// the upgrade charge, the credit refill it grants, and the subscription
// version it was cut against.
type Quote struct {
	ID                  string
	TenantID            uint64
	SubscriptionVersion int64
	Amount              CNYFen
	CreditDelta         Credits
	ExpiresAt           time.Time
}

// ValidateForUse reports whether the quote may still settle an order for the
// given subscription version at now. Expired quotes never settle, and a quote
// cut for an older subscription version is a concurrent-change conflict.
func (q Quote) ValidateForUse(subscriptionVersion int64, now time.Time) error {
	if q.ID == "" || q.TenantID == 0 || q.ExpiresAt.IsZero() {
		return ErrInvalidQuoteInterval
	}
	if now.After(q.ExpiresAt) {
		return ErrQuoteExpired
	}
	if q.SubscriptionVersion != subscriptionVersion {
		return ErrQuoteVersionConflict
	}
	return nil
}
