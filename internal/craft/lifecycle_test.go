package craft

import (
	"testing"
	"time"
)

// O03 Step 1 (brief, verbatim): active, unknown, decision-pending and
// referenced resources are never removed, whatever the clock says.
func TestLifecycleKeepsUnknownAndReferencedResources(t *testing.T) {
	now := time.Unix(100, 0)
	for _, s := range []ResourceState{{Unknown: true}, {Active: true}, {DecisionPending: true}, {Referenced: true}} {
		if CanDeleteResource(s, now) {
			t.Fatal("live resource removed")
		}
	}
}

// An otherwise-unprotected resource still waits for its eligibility window:
// before it elapses the resource stays; at or after it the resource may go.
func TestCanDeleteResourceWaitsForEligibility(t *testing.T) {
	eligible := time.Unix(1_000, 0)
	s := ResourceState{EligibleAt: eligible}
	if CanDeleteResource(s, eligible.Add(-1)) {
		t.Fatal("resource removed before eligibility")
	}
	if !CanDeleteResource(s, eligible) {
		t.Fatal("resource kept at eligibility")
	}
}

// Default policy values are the documented initial suggestions and stay
// configurable: 30min sandbox idle before dormancy, 24h orphan candidate
// window, sweep batches of 100.
func TestLifecyclePolicyDefaults(t *testing.T) {
	p := LifecyclePolicy{}.Normalize()
	if p.SandboxIdleThreshold != DefaultSandboxIdleThreshold {
		t.Fatalf("idle threshold %v", p.SandboxIdleThreshold)
	}
	if p.OrphanCandidateWindow != DefaultOrphanCandidateWindow {
		t.Fatalf("orphan window %v", p.OrphanCandidateWindow)
	}
	if p.SweepBatchSize != DefaultSweepBatchSize {
		t.Fatalf("batch %d", p.SweepBatchSize)
	}
	if DefaultSandboxIdleThreshold != 30*time.Minute {
		t.Fatalf("idle threshold default changed: %v", DefaultSandboxIdleThreshold)
	}
	if DefaultOrphanCandidateWindow != 24*time.Hour {
		t.Fatalf("orphan window default changed: %v", DefaultOrphanCandidateWindow)
	}
}

// Dormancy is a pause, never a deletion, and demands BOTH a long-enough idle
// window AND a verified complete snapshot (files + OpenCode session state).
// Without the snapshot an idle sandbox stays: pausing it could lose the only
// copy of the session state. Resuming is always an explicit restart — process
// state is never promised back.
func TestSandboxDormancyRequiresIdleAndCompleteSnapshot(t *testing.T) {
	p := LifecyclePolicy{}.Normalize()

	young := SandboxDormancy(p.SandboxIdleThreshold-time.Minute, true, p)
	if young.MayDormant {
		t.Fatal("busy sandbox dormanted")
	}
	incomplete := SandboxDormancy(p.SandboxIdleThreshold, false, p)
	if incomplete.MayDormant {
		t.Fatal("sandbox without a verified snapshot dormanted")
	}
	ok := SandboxDormancy(p.SandboxIdleThreshold, true, p)
	if !ok.MayDormant {
		t.Fatal("idle sandbox with verified snapshot refused dormancy")
	}
	if !ok.NeedsRestart {
		t.Fatal("dormancy promised process state back")
	}
}

// A provider TTL that cannot be extended surfaces the workspace risk BEFORE
// the sandbox vanishes; an unknown expiry never reads as "lives forever".
func TestProviderTTLRiskShownEarly(t *testing.T) {
	now := time.Unix(10_000, 0)
	p := LifecyclePolicy{}.Normalize()

	soon := now.Add(p.SandboxIdleThreshold / 2)
	if r := ProviderTTLRisk(soon, false, now, p); !r.AtRisk {
		t.Fatal("unextendable dying TTL not surfaced")
	}
	if r := ProviderTTLRisk(now.Add(24*time.Hour), true, now, p); r.AtRisk {
		t.Fatal("extendable far TTL flagged")
	}
	// Unknown expiry: assume nothing about container permanence.
	if r := ProviderTTLRisk(time.Time{}, true, now, p); !r.AtRisk {
		t.Fatal("unknown TTL assumed permanent")
	}
}

// Lifecycle usage events dedup on their content identity: the same
// start/stop redelivered (worker replay, at-least-once delivery) keys
// identically and is counted once; a different moment is a new fact.
func TestLifecycleEventKeyDedupsRedelivery(t *testing.T) {
	at := time.Unix(5_000, 0)
	if LifecycleEventKey(1, "sbx", LifecycleEventSandboxStart, at) !=
		LifecycleEventKey(1, "sbx", LifecycleEventSandboxStart, at) {
		t.Fatal("redelivery counted twice")
	}
	if LifecycleEventKey(1, "sbx", LifecycleEventSandboxStart, at) ==
		LifecycleEventKey(1, "sbx", LifecycleEventSandboxStop, at) {
		t.Fatal("start and stop collided")
	}
	if LifecycleEventKey(1, "sbx", LifecycleEventSandboxStart, at) ==
		LifecycleEventKey(2, "sbx", LifecycleEventSandboxStart, at) {
		t.Fatal("tenant collision")
	}
}

// Dwell is measured from real start/stop pairs and storage is accounted in
// bytes-day: one byte held for one full day is exactly one byte-day.
func TestSandboxDwellAndBytesDay(t *testing.T) {
	start := time.Unix(1_000, 0)
	stop := start.Add(time.Hour)
	dwell, err := SandboxDwell(start, stop)
	if err != nil {
		t.Fatal(err)
	}
	if dwell != time.Hour {
		t.Fatalf("dwell %v", dwell)
	}
	if _, err := SandboxDwell(stop, start); err == nil {
		t.Fatal("negative dwell accepted")
	}
	if got := BytesDay(24, 24*time.Hour); got != 24 {
		t.Fatalf("bytes-day %v", got)
	}
	if got := BytesDay(1024, 0); got != 0 {
		t.Fatalf("bytes-day %v", got)
	}
}

// Quota over-limit gates ONLY new sandbox starts: downloads and cleanup stay
// available so an over-limit workspace can still shed resources.
func TestQuotaOverLimitBlocksOnlyNewSandboxes(t *testing.T) {
	if !QuotaAllows(LifecycleActionStartSandbox, false, false) {
		t.Fatal("healthy quota refused a start")
	}
	if QuotaAllows(LifecycleActionStartSandbox, true, false) {
		t.Fatal("sandbox quota over limit admitted a start")
	}
	if QuotaAllows(LifecycleActionStartSandbox, false, true) {
		t.Fatal("storage quota over limit admitted a start")
	}
	for _, a := range []string{LifecycleActionDownload, LifecycleActionCleanup} {
		if !QuotaAllows(a, true, true) {
			t.Fatalf("%s gated by quota", a)
		}
	}
}
