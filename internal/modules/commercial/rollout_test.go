package commercial

import (
	"testing"
	"time"
)

func TestRollbackKeepsPaidRecoveryAndSettlement(t *testing.T) {
	for _, s := range []string{"payment_callback", "refund_query", "fulfillment_replay", "settlement"} {
		if !AllowDuringRollback(s) {
			t.Fatal(s)
		}
	}
	if AllowDuringRollback("new_order") {
		t.Fatal("new money accepted while paused")
	}
}

// TestRollbackRejectsEveryOtherOperation pins the closed side of the
// switch: only the four recovery-safe operations pass; everything a
// customer could use to spend NEW money during a paused rollout fails.
func TestRollbackRejectsEveryOtherOperation(t *testing.T) {
	for _, s := range []string{"new_order", "new_dispatch", "connector_action", "refund_create", ""} {
		if AllowDuringRollback(s) {
			t.Fatalf("operation %q allowed during rollback", s)
		}
	}
}

// TestRecoveryCategoryRoundTrip keeps the queue vocabulary closed: every
// category constant survives a string round-trip, and classification maps
// known operation/state pairs onto the intended category.
func TestRecoveryCategoryRoundTrip(t *testing.T) {
	if len(RecoveryCategories) != 6 {
		t.Fatalf("categories = %d, want 6", len(RecoveryCategories))
	}
	for _, c := range RecoveryCategories {
		got, ok := ParseRecoveryCategory(string(c))
		if !ok || got != c {
			t.Fatalf("round-trip %q -> (%q,%v)", c, got, ok)
		}
	}
	if _, ok := ParseRecoveryCategory("not_a_category"); ok {
		t.Fatal("unknown category parsed")
	}
	cases := []struct {
		kind, state string
		want        RecoveryCategory
	}{
		{"payment_callback", "pending", RecoveryCategoryPaidUnfulfilled},
		{"fulfillment_replay", "dead", RecoveryCategoryPaidUnfulfilled},
		{"refund_query", "unknown", RecoveryCategoryRefundUnknown},
		{"refund_query", "revocation_pending", RecoveryCategoryRevocationPending},
		{"settlement", "pending", RecoveryCategoryUsageUnconfirmed},
		{"settlement", "discrepancy", RecoveryCategoryBalanceDiscrepancy},
		{"connector_action", "pending", RecoveryCategoryUnknownAction},
		{"", "", RecoveryCategoryUnknownAction},
	}
	for _, c := range cases {
		if got := ClassifyRecovery(c.kind, c.state); got != c.want {
			t.Fatalf("ClassifyRecovery(%q,%q) = %q, want %q", c.kind, c.state, got, c.want)
		}
	}
}

// TestCommercialSwitchesDefaultOpenAndGateNewConsumption verifies the
// safe-on default: all three switches default open, and closing one
// rejects only its own NEW consumption lane — recovery-safe operations
// stay allowed.
func TestCommercialSwitchesDefaultOpenAndGateNewConsumption(t *testing.T) {
	def := DefaultCommercialSwitches()
	if !def.NewOrders || !def.NewDispatch || !def.ConnectorNewActions {
		t.Fatal("default switches must be open (safe-on)")
	}
	closed := CommercialSwitches{}
	for _, op := range []string{"new_order", "new_dispatch", "connector_action"} {
		if def.AllowsNewConsumption(op) != true {
			t.Fatalf("open switch rejected %q", op)
		}
		if closed.AllowsNewConsumption(op) != false {
			t.Fatalf("closed switch accepted %q", op)
		}
	}
	for _, op := range []string{"payment_callback", "refund_query", "settlement", ""} {
		if !closed.AllowsNewConsumption(op) {
			t.Fatalf("closed switch blocked non-new operation %q", op)
		}
	}
}

// TestRecoveryAlertThresholdsValidation keeps threshold config honest:
// negative bounds and an all-zero config (which would never alert) are
// rejected; a valid config breaches on count and age as configured.
func TestRecoveryAlertThresholdsValidation(t *testing.T) {
	if err := (RecoveryAlertThresholds{MaxCount: -1}).Validate(); err == nil {
		t.Fatal("negative count accepted")
	}
	if err := (RecoveryAlertThresholds{MaxAge: -time.Minute}).Validate(); err == nil {
		t.Fatal("negative age accepted")
	}
	if err := (RecoveryAlertThresholds{}).Validate(); err == nil {
		t.Fatal("all-zero thresholds accepted")
	}
	valid := RecoveryAlertThresholds{MaxCount: 10, MaxAgeP90: time.Hour, MaxAge: 6 * time.Hour}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid thresholds rejected: %v", err)
	}
	if valid.Breached(RecoveryCategoryMetrics{Category: RecoveryCategoryPaidUnfulfilled, Count: 3, AgeP90: 30 * time.Minute, MaxAge: time.Hour}) {
		t.Fatal("healthy metrics breached")
	}
	if !valid.Breached(RecoveryCategoryMetrics{Count: 11}) {
		t.Fatal("count breach missed")
	}
	if !valid.Breached(RecoveryCategoryMetrics{AgeP90: 2 * time.Hour}) {
		t.Fatal("p90 age breach missed")
	}
	if !valid.Breached(RecoveryCategoryMetrics{MaxAge: 7 * time.Hour}) {
		t.Fatal("max-age breach missed")
	}
}

// TestRedactForLogMasksCredentialsAndTruncatesPayloads guards the recovery
// log path: credential-like keys are masked whatever their nesting, and
// oversized values are truncated instead of dumped whole.
func TestRedactForLogMasksCredentialsAndTruncatesPayloads(t *testing.T) {
	in := map[string]any{
		"order_id":      "ord_1",
		"access_token":  "super-secret",
		"Authorization": "Bearer x",
		"nested": map[string]any{
			"api_key":     "k-123",
			"description": "ok",
		},
		"big": string(make([]byte, 4096)),
	}
	out := RedactForLog(in).(map[string]any)
	if out["order_id"] != "ord_1" {
		t.Fatalf("plain field mangled: %v", out["order_id"])
	}
	if out["access_token"] != "[REDACTED]" {
		t.Fatalf("credential not masked: %v", out["access_token"])
	}
	if out["Authorization"] != "[REDACTED]" {
		t.Fatalf("authorization not masked: %v", out["Authorization"])
	}
	nested := out["nested"].(map[string]any)
	if nested["api_key"] != "[REDACTED]" || nested["description"] != "ok" {
		t.Fatalf("nested redaction wrong: %v", nested)
	}
	if big, _ := out["big"].(string); len(big) >= 4096 {
		t.Fatalf("oversized payload not truncated: %d", len(big))
	}
	if RedactForLog("plain") != "plain" {
		t.Fatal("scalar value mangled")
	}
}
