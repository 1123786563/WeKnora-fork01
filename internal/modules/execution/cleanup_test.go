package execution

import "testing"

func TestCleanupRequiresStopAndSettlement(t *testing.T) {
	if CanPurge(CleanupFacts{Stopped: false, Settled: true, RetentionElapsed: true}) {
		t.Fatal("orphaned live process")
	}
	if CanPurge(CleanupFacts{Stopped: true, Settled: false, RetentionElapsed: true}) {
		t.Fatal("lost unsettled usage")
	}
	if !CanPurge(CleanupFacts{Stopped: true, Settled: true, RetentionElapsed: true}) {
		t.Fatal("safe cleanup blocked")
	}
}
