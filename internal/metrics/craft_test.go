package metrics

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestCraftMetricsLowCardinalityVocabulary(t *testing.T) {
	ResetCraftMetrics()
	// In-vocabulary values record under their own series.
	CraftDelegationSettled("succeeded")
	CraftDelegationSettled("failed")
	CraftReconciled("corrected")
	CraftReconciled("matched")
	// Out-of-vocabulary values — including unbounded identities a buggy
	// caller might try to stuff into a label — fold onto "other" instead of
	// creating a new series.
	CraftDelegationSettled("run_12345")
	CraftDelegationSettled("tenant-9")
	CraftReconciled("session abc")

	snap := CraftMetricsSnapshot()
	if got := snap[fmt.Sprintf("craft_delegations_total{status=%q}", "succeeded")]; got != 1 {
		t.Fatalf("succeeded = %v, want 1", got)
	}
	if got := snap[fmt.Sprintf("craft_delegations_total{status=%q}", "other")]; got != 2 {
		t.Fatalf("folded other = %v, want 2", got)
	}
	if got := snap[fmt.Sprintf("craft_reconcile_total{outcome=%q}", "other")]; got != 1 {
		t.Fatalf("reconcile other = %v, want 1", got)
	}
	// No series may ever carry an unbounded identity.
	for k := range snap {
		if strings.Contains(k, "run_") || strings.Contains(k, "tenant-") {
			t.Fatalf("unbounded identity leaked into metric label: %s", k)
		}
	}
}

func TestCraftMetricsGaugesCountersAndHistogram(t *testing.T) {
	ResetCraftMetrics()
	SetCraftPendingDecisions(3)
	CraftPreviewFailure()
	CraftPreviewFailure()
	CraftUsageUnknown()
	ObserveCraftWorkspaceRestore(300 * time.Millisecond) // bucket 0.5
	ObserveCraftWorkspaceRestore(2 * time.Second)        // bucket 2.5
	ObserveCraftWorkspaceRestore(10 * time.Minute)       // +Inf

	snap := CraftMetricsSnapshot()
	if got := snap["craft_pending_decisions"]; got != 3 {
		t.Fatalf("pending = %v, want 3", got)
	}
	if got := snap["craft_preview_failures_total"]; got != 2 {
		t.Fatalf("preview failures = %v, want 2", got)
	}
	if got := snap["craft_usage_unknown_total"]; got != 1 {
		t.Fatalf("usage unknown = %v, want 1", got)
	}
	if got := snap["craft_workspace_restore_seconds_count"]; got != 3 {
		t.Fatalf("restore count = %v, want 3", got)
	}
	// Cumulative buckets: 300ms and 2s both fit le=2.5; the 10m restore only
	// fits +Inf.
	if got := snap[fmt.Sprintf("craft_workspace_restore_seconds_bucket{le=%q}", "2.5")]; got != 2 {
		t.Fatalf("le=2.5 = %v, want 2", got)
	}
	if got := snap[fmt.Sprintf("craft_workspace_restore_seconds_bucket{le=%q}", "+Inf")]; got != 3 {
		t.Fatalf("le=+Inf = %v, want 3", got)
	}
	if got := snap["craft_workspace_restore_seconds_sum"]; got < 602 || got > 603 {
		t.Fatalf("sum = %v, want ~602.3", got)
	}
}
