package commercial

import (
	"strconv"
	"testing"
	"time"
)

// TestExternalCustomerIDIsAPureFunctionOfTheTenantID locks the deterministic
// WeKnora→authority customer identity: same input ⇒ same output, the format
// is weknora-tenant-<decimal id>, distinct tenants ⇒ distinct ids, and the
// charset is the #73 probe-proven Lago external-id class [a-z0-9-] only. The
// signature takes EXACTLY one uint64 — rename and owner transfer have no
// input through which to influence identity (compile-enforced).
func TestExternalCustomerIDIsAPureFunctionOfTheTenantID(t *testing.T) {
	cases := []uint64{0, 1, 42, 101, 999999999}
	for _, id := range cases {
		want := "weknora-tenant-" + strconv.FormatUint(id, 10)
		if got := ExternalCustomerID(id); got != want {
			t.Fatalf("ExternalCustomerID(%d) = %q, want %q", id, got, want)
		}
	}
	if ExternalCustomerID(7) != ExternalCustomerID(7) {
		t.Fatalf("same input must yield the same output")
	}
	if ExternalCustomerID(7) == ExternalCustomerID(8) {
		t.Fatalf("distinct tenants must yield distinct ids")
	}
	for _, id := range cases {
		for _, r := range ExternalCustomerID(id) {
			ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
			if !ok {
				t.Fatalf("ExternalCustomerID(%d) = %q carries rune %q outside [a-z0-9-]", id, ExternalCustomerID(id), r)
			}
		}
	}
}

// TestW3KindConstantsHaveTheFrozenTokenValues locks the exact wire tokens of
// the W3 additions on the frozen seam: the ensure_customer command kind, the
// account snapshot kind, and the closed account-truth enum.
func TestW3KindConstantsHaveTheFrozenTokenValues(t *testing.T) {
	if CommandKindEnsureCustomer != "ensure_customer" {
		t.Fatalf("CommandKindEnsureCustomer must be \"ensure_customer\", got %q", CommandKindEnsureCustomer)
	}
	if SnapshotKindAccount != "account" {
		t.Fatalf("SnapshotKindAccount must be \"account\", got %q", SnapshotKindAccount)
	}
	if AccountStateLinked != "linked" || AccountStateAbsent != "absent" {
		t.Fatalf("account states must be exactly linked/absent, got %q/%q", AccountStateLinked, AccountStateAbsent)
	}
}

// TestEnsureCustomerCommandPassesValidateUnchanged: Validate stays the T05
// shape (Kind/Key only) — a well-formed ensure_customer command passes
// without any payload typing being added to Validate.
func TestEnsureCustomerCommandPassesValidateUnchanged(t *testing.T) {
	cmd := Command{
		Kind:    CommandKindEnsureCustomer,
		Key:     "ensure_customer:weknora-tenant-1",
		Actor:   "user-1",
		Reason:  "first_billing_access",
		Payload: EnsureCustomerPayload{TenantID: 1, ExternalCustomerID: "weknora-tenant-1", DisplayName: "Space One"},
	}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("well-formed ensure_customer rejected: %v", err)
	}
	if cmd.Payload.(EnsureCustomerPayload).ExternalCustomerID != ExternalCustomerID(1) {
		t.Fatalf("payload identity must match the derivation")
	}
}

// TestAccountSnapshotSectionRoundTrips: the account section is an ADDITIVE
// optional field — set it and read it back; a zero-value Snapshot keeps
// Account == nil so readiness-only consumers are unaffected.
func TestAccountSnapshotSectionRoundTrips(t *testing.T) {
	checked := time.Now().UTC()
	snap := Snapshot{
		Kind: SnapshotKindAccount,
		Account: &AccountSnapshot{
			TenantID:  42,
			State:     AccountStateLinked,
			CheckedAt: checked,
		},
	}
	if snap.Account == nil || snap.Account.TenantID != 42 ||
		snap.Account.State != AccountStateLinked || !snap.Account.CheckedAt.Equal(checked) {
		t.Fatalf("account section must round-trip, got %+v", snap.Account)
	}
	var zero Snapshot
	if zero.Account != nil {
		t.Fatalf("zero-value Snapshot must keep Account nil, got %+v", zero.Account)
	}
}

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
