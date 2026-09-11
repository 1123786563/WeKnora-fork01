package commercial

import (
	"math"
	"testing"
)

// TestQuotaAllowsCleanupWhileOverLimit is the brief Step 1 scenario: a
// tenant already over quota may still free resources but never grow, and
// a configured zero limit is a hard zero rather than unlimited.
func TestQuotaAllowsCleanupWhileOverLimit(t *testing.T) {
	limit := int64(10)
	if CanIncrease(12, 1, &limit) {
		t.Fatal("over quota growth")
	}
	if !CanIncrease(12, -1, &limit) {
		t.Fatal("cleanup blocked")
	}
	zero := int64(0)
	if CanIncrease(0, 1, &zero) {
		t.Fatal("zero treated as unlimited")
	}
}

// TestQuotaNegativeCurrentRejected: a corrupt negative counter must never
// authorise anything, not even a cleanup that would mask the corruption.
func TestQuotaNegativeCurrentRejected(t *testing.T) {
	limit := int64(10)
	if CanIncrease(-1, 1, &limit) {
		t.Fatal("negative current accepted for growth")
	}
	if CanIncrease(-1, -1, &limit) {
		t.Fatal("negative current accepted for cleanup")
	}
}

// TestQuotaCleanupMayNotGoBelowZero: cleanup is bounded by zero; a delta
// that would drive the counter negative is rejected, landing exactly on
// zero is fine.
func TestQuotaCleanupMayNotGoBelowZero(t *testing.T) {
	limit := int64(10)
	if CanIncrease(3, -4, &limit) {
		t.Fatal("cleanup below zero accepted")
	}
	if !CanIncrease(3, -3, &limit) {
		t.Fatal("cleanup to exactly zero rejected")
	}
}

// TestQuotaNilLimitIsUnlimited: an absent limit means unlimited, including
// right below the int64 ceiling.
func TestQuotaNilLimitIsUnlimited(t *testing.T) {
	if !CanIncrease(math.MaxInt64-1, 1, nil) {
		t.Fatal("nil limit must be unlimited")
	}
}

// TestQuotaOverflowGuard: a positive delta whose sum would wrap int64 is
// rejected regardless of how generous the limit is.
func TestQuotaOverflowGuard(t *testing.T) {
	limit := int64(math.MaxInt64)
	if CanIncrease(math.MaxInt64-1, 2, &limit) {
		t.Fatal("int64 overflow accepted under limit")
	}
	if CanIncrease(math.MaxInt64-1, 2, nil) {
		t.Fatal("int64 overflow accepted without limit")
	}
}
