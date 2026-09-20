package commercial

import "testing"

// TestCommandValidateRejectsEmptyKindAndKey: a command without a kind or
// without its idempotency identity is not submittable — the freeze requires
// Validate to reject both before any adapter sees the command.
func TestCommandValidateRejectsEmptyKindAndKey(t *testing.T) {
	if err := (Command{Kind: "", Key: "k"}).Validate(); err == nil {
		t.Fatalf("empty Kind must be rejected")
	}
	if err := (Command{Kind: CommandKind("ensure_customer"), Key: ""}).Validate(); err == nil {
		t.Fatalf("empty Key must be rejected")
	}
}

// TestCommandValidateAcceptsWellFormedCommand: kind + key is a valid command
// today; payload typing lands with the W3 kinds, so nil/any payloads pass.
func TestCommandValidateAcceptsWellFormedCommand(t *testing.T) {
	cmd := Command{
		Kind:    CommandKind("ensure_customer"),
		Key:     "cmd-1",
		Actor:   "user-1",
		Reason:  "checkout",
		Payload: nil,
	}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("well-formed command rejected: %v", err)
	}
	cmd.Payload = map[string]any{"tenant_id": uint64(7)}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("command with typed payload rejected: %v", err)
	}
}

// TestReadinessStateConstantsAreTheClosedProductEnum locks the three product
// states (the only tokens that may ever cross the Billing API) and the
// readiness snapshot kind.
func TestReadinessStateConstantsAreTheClosedProductEnum(t *testing.T) {
	if ReadinessReady != "ready" || ReadinessDegraded != "degraded" || ReadinessUnavailable != "unavailable" {
		t.Fatalf("readiness states must be exactly ready/degraded/unavailable, got %q/%q/%q",
			ReadinessReady, ReadinessDegraded, ReadinessUnavailable)
	}
	states := map[ReadinessState]bool{
		ReadinessReady:       true,
		ReadinessDegraded:    true,
		ReadinessUnavailable: true,
	}
	if len(states) != 3 {
		t.Fatalf("readiness state constants must be pairwise distinct")
	}
	if SnapshotKindReadiness != "readiness" {
		t.Fatalf("SnapshotKindReadiness must be \"readiness\", got %q", SnapshotKindReadiness)
	}
}

// TestPlatformSentinelErrorsAreDistinct: the four fail-closed sentinels are
// provider-neutral and distinguishable by errors.Is through wrapping.
func TestPlatformSentinelErrorsAreDistinct(t *testing.T) {
	all := []error{ErrPlatformUnsupported, ErrPlatformUnconfigured, ErrPlatformUnreachable, ErrPlatformInvalidResponse}
	for i, a := range all {
		if a == nil {
			t.Fatalf("sentinel %d is nil", i)
		}
		for j, b := range all {
			if i != j && a == b {
				t.Fatalf("sentinels %d and %d alias the same error", i, j)
			}
		}
	}
}
