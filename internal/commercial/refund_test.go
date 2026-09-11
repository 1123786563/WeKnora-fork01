package commercial

import "testing"

func TestRefundUnknownKeepsLock(t *testing.T) {
	for _, s := range []string{"pending", "unknown", "succeeded", "revocation_pending"} {
		if CanUnlockRefund(s) {
			t.Fatalf("unlocked %s", s)
		}
	}
	if !CanUnlockRefund("failed_confirmed") {
		t.Fatal("confirmed failure not released")
	}
}

// TestRefundUnlockRules pins the full unlock table: a lock may be released
// ONLY on a channel outcome that is confirmed failed or confirmed as never
// created. Success, revocation progress, in-flight, and indeterminate
// outcomes all keep the lock; unrecognised values fail closed.
func TestRefundUnlockRules(t *testing.T) {
	cases := []struct {
		state string
		want  bool
	}{
		{RefundChannelPending, false},
		{RefundChannelUnknown, false},
		{RefundChannelSucceeded, false},
		{RefundStateRevocationPending, false},
		{RefundStateCompleted, false},
		{RefundStateRequested, false},
		{RefundStateReviewing, false},
		{RefundStateFailedConfirmed, true},
		{RefundStateNotCreatedConfirmed, true},
		{"", false},
		{"SUCCESS", false},
		{"failed", false},
	}
	for _, c := range cases {
		if got := CanUnlockRefund(c.state); got != c.want {
			t.Fatalf("CanUnlockRefund(%q) = %v, want %v", c.state, got, c.want)
		}
	}
}

// TestRefundChannelTransitionStateMachine drives the stored refund state
// through every confirmed channel outcome. Success can only ever reach
// revocation_pending (payout alone never completes a refund); a confirmed
// failure or confirmed not-created reaches its unlock state; anything
// unproven falls back to reviewing for a re-query by the ORIGINAL refund
// key — never a forced progression.
func TestRefundChannelTransitionStateMachine(t *testing.T) {
	cases := []struct {
		current string
		channel string
		want    string
	}{
		{RefundStatePending, RefundChannelSucceeded, RefundStateRevocationPending},
		{RefundStateRevocationPending, RefundChannelSucceeded, RefundStateRevocationPending},
		{RefundStatePending, RefundChannelFailed, RefundStateFailedConfirmed},
		{RefundStatePending, RefundChannelNotCreated, RefundStateNotCreatedConfirmed},
		{RefundStatePending, RefundChannelUnknown, RefundStateReviewing},
		{RefundStateReviewing, RefundChannelUnknown, RefundStateReviewing},
		{RefundStateRequested, RefundChannelUnknown, RefundStateReviewing},
		{RefundStatePending, "garbage", RefundStateReviewing},
	}
	for _, c := range cases {
		if got := TransitionRefundOnChannel(c.current, c.channel); got != c.want {
			t.Fatalf("TransitionRefundOnChannel(%q, %q) = %q, want %q", c.current, c.channel, got, c.want)
		}
	}
}

// TestRefundRevocationCompletion: revocation_pending completes ONLY on a
// confirmed precise-credits revocation; a failed revocation stays
// revocation_pending so recovery retries the revocation and never pays out
// a second time.
func TestRefundRevocationCompletion(t *testing.T) {
	if got := TransitionRefundOnRevocation(RefundStateRevocationPending, true); got != RefundStateCompleted {
		t.Fatalf("confirmed revocation = %q, want completed", got)
	}
	if got := TransitionRefundOnRevocation(RefundStateRevocationPending, false); got != RefundStateRevocationPending {
		t.Fatalf("failed revocation = %q, want revocation_pending", got)
	}
	// A completed refund never moves again on a late channel result.
	if got := TransitionRefundOnChannel(RefundStateCompleted, RefundChannelFailed); got != RefundStateCompleted {
		t.Fatalf("completed refund regressed to %q", got)
	}
}

// TestRefundRequestStateFields keeps the produced projection honest:
// amounts are exact fen, credits are exact millionths.
func TestRefundRequestStateFields(t *testing.T) {
	r := RefundRequestState{
		ID: "rf_1", OrderID: "ord_1", State: RefundStateRequested,
		TenantID: 7, Amount: CNYFen(9900), CreditAmount: Credits(99_000_000),
	}
	if r.Amount != CNYFen(9900) || r.CreditAmount.String() != "99.000000" {
		t.Fatalf("field types drifted: %+v", r)
	}
}

// TestRefundEligibilityDefaultNotReady documents the P03 seam: until the
// budget coordinator's occupancy-refundable check exists, the default
// checker must refuse — refunds stay requested/reviewing, never approved
// into paid-out states.
func TestRefundEligibilityDefaultNotReady(t *testing.T) {
	if err := DefaultRefundEligibility.RefundEligible(nil, 1, "ord", Credits(1)); err == nil {
		t.Fatal("default eligibility must not be ready before P03 lands")
	}
}
