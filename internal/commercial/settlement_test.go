package commercial

import (
	"errors"
	"testing"
	"time"
)

// TestSettlementRetainsProtectionUntilAcknowledged is the U03 Step 1
// assertion, verbatim: every pre-confirmation state keeps the spend
// protected; only an explicitly confirmed settlement releases it.
func TestSettlementRetainsProtectionUntilAcknowledged(t *testing.T) {
	for _, s := range []string{"dispatched", "unknown", "sent", "accepted"} {
		if !KeepProtection(s) {
			t.Fatalf("released %s", s)
		}
	}
	if KeepProtection("confirmed") {
		t.Fatal("confirmed spend still held")
	}
}

// TestSettlementStateTable pins the full state vocabulary: the four
// pre-confirmation states protect, confirmed releases, and any state the
// vocabulary does not know defaults to KEEPING protection — an unrecognized
// answer is never a licence to release spend.
func TestSettlementStateTable(t *testing.T) {
	protects := map[string]bool{
		SettlementStateDispatched: true,
		SettlementStateUnknown:    true,
		SettlementStateSent:       true,
		SettlementStateAccepted:   true,
	}
	for state, want := range protects {
		if got := KeepProtection(state); got != want {
			t.Fatalf("KeepProtection(%q) = %v, want %v", state, got, want)
		}
	}
	if !KeepProtection("") {
		t.Fatal("empty state must keep protection")
	}
	if !KeepProtection("weird-transport-glitch") {
		t.Fatal("unknown state must keep protection")
	}
	if !KeepProtection(SettlementStateConfirmed + "x") {
		t.Fatal("near-miss of confirmed must keep protection")
	}
}

// TestSettlementRevisionKeyReuse pins the settlement idempotency identity:
// the key is a pure function of the usage revision identity, so a replay
// (lost response, retry, crash) reuses exactly the same key and can never
// double-settle, while a different tenant/call/attempt/revision never
// collides with it — external corrections arrive as their own revision key.
func TestSettlementRevisionKeyReuse(t *testing.T) {
	a := SettlementKey(7, "call_1", "att_1", 1)
	if a == "" {
		t.Fatal("empty settlement key")
	}
	if got := SettlementKey(7, "call_1", "att_1", 1); got != a {
		t.Fatal("same revision must reuse the same settlement key")
	}
	nul := string(rune(0))
	for name, other := range map[string]string{
		"revision":   SettlementKey(7, "call_1", "att_1", 2),
		"attempt":    SettlementKey(7, "call_1", "att_2", 1),
		"call":       SettlementKey(7, "call_2", "att_1", 1),
		"tenant":     SettlementKey(8, "call_1", "att_1", 1),
		"collisions": SettlementKey(7, "call_1"+nul+"att_1", "", 1),
	} {
		if other == a {
			t.Fatalf("settlement key collides on %s", name)
		}
	}
}

// TestSettlementValidate enforces the gateway contract: identity, tenant,
// non-negative amount, positive revision, and a real occurrence time.
func TestSettlementValidate(t *testing.T) {
	base := Settlement{
		ID:            "settle:abc",
		CallID:        "call_1",
		ReservationID: "res_1",
		TenantID:      7,
		Amount:        Credits(60),
		Revision:      1,
		OccurredAt:    time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("valid settlement rejected: %v", err)
	}
	mut := func(f func(*Settlement)) Settlement {
		s := base
		f(&s)
		return s
	}
	for name, s := range map[string]Settlement{
		"no id":          mut(func(s *Settlement) { s.ID = "" }),
		"no call":        mut(func(s *Settlement) { s.CallID = "" }),
		"no reservation": mut(func(s *Settlement) { s.ReservationID = "" }),
		"no tenant":      mut(func(s *Settlement) { s.TenantID = 0 }),
		"neg amount":     mut(func(s *Settlement) { s.Amount = -1 }),
		"no revision":    mut(func(s *Settlement) { s.Revision = 0 }),
		"no time":        mut(func(s *Settlement) { s.OccurredAt = time.Time{} }),
	} {
		if err := s.Validate(); !errors.Is(err, ErrInvalidSettlement) {
			t.Fatalf("%s: want ErrInvalidSettlement, got %v", name, err)
		}
	}
}

// TestSettlementReceiptRequiresCorrelationEvidence pins the anti-inference
// rule: a balance drop is never evidence. Confirmation requires BOTH the
// event/transaction correlation id and the watermark it advanced.
func TestSettlementReceiptRequiresCorrelationEvidence(t *testing.T) {
	if !(SettlementReceipt{ExternalID: "x", Watermark: "w"}).ConfirmedEvidence() {
		t.Fatal("full receipt must be confirmation evidence")
	}
	if (SettlementReceipt{ExternalID: "x"}).ConfirmedEvidence() {
		t.Fatal("external id without watermark is not confirmation")
	}
	if (SettlementReceipt{Watermark: "w"}).ConfirmedEvidence() {
		t.Fatal("watermark without transaction correlation is not confirmation")
	}
	if (SettlementReceipt{}).ConfirmedEvidence() {
		t.Fatal("empty receipt is never confirmation")
	}
}
