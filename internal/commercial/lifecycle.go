package commercial

import "time"

// MonthlyGrantKey derives the stable idempotency key for one subscription's
// monthly benefit grant. The key is anchored to the UTC month start so that
// worker replay across display time zones — and concurrent workers in the
// same period — always converge on the same key.
func MonthlyGrantKey(subscription string, start time.Time) string {
	return subscription + "/monthly/" + start.UTC().Format(time.RFC3339)
}

// BaseTier is the fallback projection a tenant lands on after its purchased
// subscription expires. It grants nothing; usage above base limits is
// reported as over limit rather than deleting data or members.
var BaseTier = PlanVersion{Key: "base", Version: 1}

// DowngradeReasonOverLimit marks a projection downgraded to BaseTier because
// the paid term expired while usage exceeds base-tier limits.
const DowngradeReasonOverLimit = "expired_over_base_limit"

// MaxDueMonths bounds the DueMonths walk so a corrupt anchor/paid_until pair
// can never loop a tick forever.
const MaxDueMonths = 1200

// Subscription is the commercial view of one tenant's purchased term. Anchor
// is the billing anchor month start; PaidUntil is the exclusive end of the
// paid interval. Annual terms simply carry a PaidUntil many months out; they
// are still granted month by month, never all at once.
type Subscription struct {
	ID        string
	TenantID  uint64
	Plan      PlanVersion
	Anchor    time.Time
	PaidUntil time.Time
	Version   int64
}

// DueMonths returns the month starts whose monthly benefit is due at now:
// every month start m with m <= now, provided the subscription is still paid
// (now < PaidUntil). Future months are never returned — early renewal that
// extends PaidUntil must not pre-grant them — and an expired subscription
// returns nothing.
func (s Subscription) DueMonths(now time.Time) []time.Time {
	if s.Anchor.IsZero() || !now.Before(s.PaidUntil) {
		return nil
	}
	var due []time.Time
	for m := s.Anchor; !m.After(now); m = MonthBoundary(m, 1) {
		due = append(due, m)
		if len(due) >= MaxDueMonths {
			break
		}
	}
	return due
}

// MonthEnd returns the exclusive end of the benefit month starting at start.
func MonthEnd(start time.Time) time.Time { return MonthBoundary(start, 1) }

// MonthWindowAt returns the [start, end) benefit-month window that contains
// at, derived from the billing anchor with the same month-end clamping the
// monthly grants use. It bounds the prorated monthly-credit delta of an
// upgrade (design 6.2: the delta covers only the remaining part of the
// current month and expires with it).
func MonthWindowAt(anchor, at time.Time) (time.Time, time.Time) {
	if anchor.IsZero() {
		return at, MonthEnd(at)
	}
	start := anchor
	for {
		next := MonthBoundary(start, 1)
		if next.After(at) {
			return start, next
		}
		start = next
	}
}

// Projection is the plan a tenant is currently served on.
type Projection struct {
	Plan       PlanVersion
	Downgraded bool
	Reason     string
}

// ProjectionAt returns the effective plan at now. While the term is paid the
// purchased plan stands; from PaidUntil on the tenant is projected onto
// BaseTier with the over-limit downgrade reason. Purchased top-ups and data
// are never deleted by the projection change.
func (s Subscription) ProjectionAt(now time.Time) Projection {
	if now.Before(s.PaidUntil) {
		return Projection{Plan: s.Plan}
	}
	return Projection{Plan: BaseTier, Downgraded: true, Reason: DowngradeReasonOverLimit}
}
