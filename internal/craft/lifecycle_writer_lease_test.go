package craft

// T16 (#134): the Workspace writer lease rules as pure decisions — an
// unknown outcome always retains the fence, only a confirmed terminal run
// with zero unfinished writers may release or be superseded.
import "testing"

func TestCraftWriterLeaseLifecycleRules(t *testing.T) {
	terminal := WriterRunFacts{Observed: true, Status: "succeeded"}
	if !WriterLeaseReleasable(terminal) {
		t.Fatal("a verified terminal run with no pending writers must be releasable")
	}
	for _, status := range []string{"succeeded", "failed", "canceled"} {
		if !WriterRunTerminal(status) {
			t.Fatalf("%s must be terminal", status)
		}
		if !WriterLeaseReleasable(WriterRunFacts{Observed: true, Status: status}) {
			t.Fatalf("%s with no pending writers must be releasable", status)
		}
	}
	for _, status := range []string{"queued", "running", "waiting_user", "reconciling", "recovering", ""} {
		if WriterRunTerminal(status) {
			t.Fatalf("%q must not be terminal", status)
		}
		if WriterLeaseReleasable(WriterRunFacts{Observed: true, Status: status}) {
			t.Fatalf("a live run (%q) must retain the fence", status)
		}
	}
	// An unreadable run row is an UNKNOWN outcome: never permission.
	if WriterLeaseReleasable(WriterRunFacts{Observed: false}) {
		t.Fatal("an unobserved (unknown) run must retain the fence")
	}
	// Unfinished writers under a terminal status still fence the workspace.
	if WriterLeaseReleasable(WriterRunFacts{Observed: true, Status: "succeeded", PendingToolWriters: 1}) {
		t.Fatal("a pending tool writer must retain the fence")
	}
	if WriterLeaseReleasable(WriterRunFacts{Observed: true, Status: "canceled", PendingDelegations: 2}) {
		t.Fatal("a pending delegation must retain the fence")
	}
}

func TestCraftWriterLeaseTakeoverRules(t *testing.T) {
	held := &WriterLease{WorkspaceID: "ws-1", TaskID: "s-1", RunID: "run-1", Revision: 3}
	if !WriterLeaseTakeover(nil, WriterRunFacts{}) {
		t.Fatal("an absent lease is always takeable")
	}
	// Same-Run idempotency is resolved by the store; the rule itself only
	// answers whether the CURRENT holder may be superseded.
	for _, facts := range []WriterRunFacts{
		{Observed: false},
		{Observed: true, Status: "running"},
		{Observed: true, Status: "waiting_user"},
		{Observed: true, Status: "succeeded", PendingToolWriters: 1},
	} {
		if WriterLeaseTakeover(held, facts) {
			t.Fatalf("holder with facts %+v must not be superseded", facts)
		}
	}
	for _, status := range []string{"succeeded", "failed", "canceled"} {
		if !WriterLeaseTakeover(held, WriterRunFacts{Observed: true, Status: status}) {
			t.Fatalf("an authoritatively terminal holder (%s) may be superseded", status)
		}
	}
}
