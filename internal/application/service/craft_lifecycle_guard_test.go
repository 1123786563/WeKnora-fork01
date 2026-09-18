package service

// CFT-S05-T031: the conservative-lifecycle guard summary. The point suites
// live in craft_lifecycle_test.go; this file re-executes exactly the four
// that map 1:1 onto the acceptance bullets, under the acceptance's own
// name, so the mapping is a green test rather than a citation.
import "testing"

func TestCraftLifecycleGuardSummary(t *testing.T) {
	t.Run("referenced objects survive, orphans go (staging reclaim)", func(t *testing.T) {
		TestLifecycleOrphanCandidateWindowAndReferences(t)
		if t.Failed() {
			t.FailNow()
		}
	})
	t.Run("live work slots are not reclaimed", func(t *testing.T) {
		TestLifecycleSweepKeepsLiveSessionSandbox(t)
		if t.Failed() {
			t.FailNow()
		}
		TestLifecycleSweepGenerationProtectsNewInstance(t)
		if t.Failed() {
			t.FailNow()
		}
	})
	t.Run("gc retries stay in scope (no widening)", func(t *testing.T) {
		TestLifecycleSweepRetriesFailedProviderDelete(t)
		if t.Failed() {
			t.FailNow()
		}
	})
	t.Run("stranded mid-delete objects are re-driven, not widened", func(t *testing.T) {
		TestLifecycleSweepRedrivesStrandedDeletingObject(t)
		if t.Failed() {
			t.FailNow()
		}
	})
}
