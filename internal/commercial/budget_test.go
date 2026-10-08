package commercial

import "testing"

func TestAvailableIncludesUnacknowledgedSpend(t *testing.T) {
	got, err := Available(100, 20, 30, 10)
	if err != nil || got != 40 {
		t.Fatalf("%d %v", got, err)
	}
	if _, err := Available(100, -1, 0, 0); err == nil {
		t.Fatal("negative protection")
	}
}

// TestAvailableRejectsEveryNegativeProjection guards each input separately:
// a single negative component means the projection itself is broken, which
// is an error — never a silent zero.
func TestAvailableRejectsEveryNegativeProjection(t *testing.T) {
	cases := []struct {
		name                                      string
		verified, unreflected, held, refundLocked Credits
	}{
		{"negative_verified", -1, 0, 0, 0},
		{"negative_unreflected", 0, -1, 0, 0},
		{"negative_held", 0, 0, -1, 0},
		{"negative_refund_locked", 0, 0, 0, -1},
	}
	for _, tc := range cases {
		if got, err := Available(tc.verified, tc.unreflected, tc.held, tc.refundLocked); err == nil {
			t.Fatalf("%s: Available=%d, want invalid_projection error", tc.name, got)
		}
	}
}

// TestAvailableInsufficientReturnsZeroNotError: when the deductions exceed
// the verified balance the caller learns "nothing to spend" (0, nil) — an
// ordinary outcome, distinct from a broken projection error.
func TestAvailableInsufficientReturnsZeroNotError(t *testing.T) {
	got, err := Available(10, 20, 0, 0)
	if err != nil {
		t.Fatalf("insufficiency must not be an error: %v", err)
	}
	if got != 0 {
		t.Fatalf("Available=%d want 0", got)
	}
}

// TestAvailableExactExhaustionYieldsZero: deductions exactly equal to the
// verified balance are fully covered — the result is zero, still no error.
func TestAvailableExactExhaustionYieldsZero(t *testing.T) {
	got, err := Available(100, 30, 50, 20)
	if err != nil {
		t.Fatalf("exact exhaustion must not be an error: %v", err)
	}
	if got != 0 {
		t.Fatalf("Available=%d want 0", got)
	}
}
