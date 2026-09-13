package craft

import "testing"

// TestControlDoesNotInventApprovalOrCancellation is the R06 RED-first
// contract: a question can never be silently approved, and a cancellation is
// only confirmed after the abort is verified on the runtime — an abort
// request alone (or an idle runtime without an abort) never reports
// canceled.
func TestControlDoesNotInventApprovalOrCancellation(t *testing.T) {
	if DecisionAllowed("question", "approve") {
		t.Fatal("question approved")
	}
	if StopStatus(true, Observation{Idle: true}) == "canceled" {
		t.Fatal("abort unverified")
	}
	if StopStatus(true, Observation{Aborted: true, Idle: true}) != "canceled" {
		t.Fatal("abort lost")
	}
}
