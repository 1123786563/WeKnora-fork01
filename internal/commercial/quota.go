package commercial

import "math"

// CanIncrease reports whether a resource counter may move from current by
// delta under limit. A nil limit is unlimited; a configured zero limit is
// a hard zero, not unlimited. Negative counters never authorise anything.
// Cleanup (delta <= 0) is allowed while over limit but may not drive the
// counter below zero, so a tenant over quota can always free resources.
// Sums that would wrap int64 are rejected.
func CanIncrease(current, delta int64, limit *int64) bool {
	if current < 0 {
		return false
	}
	if delta <= 0 {
		return delta >= -current
	}
	if current > math.MaxInt64-delta {
		return false
	}
	return limit == nil || current+delta <= *limit
}
