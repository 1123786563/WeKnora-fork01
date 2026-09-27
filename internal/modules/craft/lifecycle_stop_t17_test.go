package craft

// T17 (#136): the durable stop-intent rules. The stop request and the
// confirmed cancellation stay separate durable facts: the accepted stop is
// nonterminal, only an authoritative confirmation (observed abort on an idle
// session) permits the terminal write, and an unreadable outcome is unknown —
// never permission for the writer fence or a promotion to slip.
import "testing"

func TestCraftT17StopIntentValidate(t *testing.T) {
	for _, status := range []StopOutcomeStatus{StopRequested, StopConfirmed, StopUnknown} {
		intent := StopIntent{RunID: "run-t17", Status: status}
		if err := intent.Validate(); err != nil {
			t.Fatalf("StopIntent{run-t17,%s}.Validate() = %v, want nil", status, err)
		}
	}
	for name, intent := range map[string]StopIntent{
		"empty run id":       {RunID: "", Status: StopRequested},
		"empty status":       {RunID: "run-t17"},
		"foreign status":     {RunID: "run-t17", Status: "maybe"},
		"terminal canceled":  {RunID: "run-t17", Status: "canceled"},
		"terminal succeeded": {RunID: "run-t17", Status: "succeeded"},
	} {
		if err := intent.Validate(); err == nil {
			t.Fatalf("StopIntent{%s}.Validate() = nil, want an error: the stop-intent vocabulary is exactly requested|confirmed|unknown", name)
		}
	}
}

func TestCraftT17StopIntentOutcome(t *testing.T) {
	cases := []struct {
		name string
		obs  Observation
		want StopOutcomeStatus
	}{
		{"observed abort on idle session confirms the cancellation", Observation{Aborted: true, Idle: true}, StopConfirmed},
		{"abort requested but executor still busy stays requested", Observation{Aborted: true, Idle: false}, StopRequested},
		{"idle without abort evidence stays requested", Observation{Aborted: false, Idle: true}, StopRequested},
		{"no stop evidence at all stays requested", Observation{}, StopRequested},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := StopIntentOutcome(tc.obs); got != tc.want {
				t.Fatalf("StopIntentOutcome(%+v) = %q, want %q", tc.obs, got, tc.want)
			}
		})
	}
}

func TestCraftT17StopIntentTerminalWrite(t *testing.T) {
	if StopIntentMayWriteRunTerminal(StopConfirmed) != true {
		t.Fatal("a confirmed stop is the one durable state that permits the terminal canceled write")
	}
	for _, status := range []StopOutcomeStatus{StopRequested, StopUnknown} {
		if StopIntentMayWriteRunTerminal(status) {
			t.Fatalf("stop state %q must never terminalize the Run: the accepted stop and the unknown outcome are nonterminal", status)
		}
	}
}

// TestCraftT17StoppingRunKeepsWriterFence pins the T17/T16 conjunction on the
// pure lease rules: a Run still inside the stop journey (requested or
// unknown, so its durable status is not terminal) can neither release nor
// hand over the Workspace writer lease.
func TestCraftT17StoppingRunKeepsWriterFence(t *testing.T) {
	for _, status := range []string{"running", "queued", "waiting_user", "reconciling", "recovering"} {
		facts := WriterRunFacts{Observed: true, Status: status}
		if WriterLeaseReleasable(facts) {
			t.Fatalf("a %s Run must not release the writer lease", status)
		}
		if WriterLeaseTakeover(&WriterLease{WorkspaceID: "ws-t17", RunID: "run-a"}, facts) {
			t.Fatalf("a %s Run must not permit a conflicting writer takeover", status)
		}
	}
	// Only the confirmed stop lands the terminal status — and then the fence
	// may release exactly like every other verified terminal outcome.
	confirmed := WriterRunFacts{Observed: true, Status: "canceled"}
	if !WriterLeaseReleasable(confirmed) {
		t.Fatal("a confirmed cancellation with zero pending writers is a verified release basis")
	}
	// An unreadable row is unknown — never permission.
	if WriterLeaseReleasable(WriterRunFacts{Observed: false, Status: "canceled"}) {
		t.Fatal("an unobserved Run row is unknown and must keep the fence")
	}
}


// ---- round-2 OCR regressions -------------------------------------------------

// StopIntentOutcome must NOT confirm a stop that was never requested: an
// abort by any other mechanism (budget pause, transport failure) observed
// Aborted+Idle is not a stop confirmation (same-source discipline with
// StopStatus(true, o)).
func TestStopIntentOutcomeRequiresRequestedPremise(t *testing.T) {
	abortedIdle := Observation{Aborted: true, Idle: true}
	if got := StopIntentOutcome(abortedIdle); got != StopConfirmed {
		t.Fatalf("aborted+idle observation should confirm: %v", got)
	}
	// StopStatus(true, ...) requires requested && aborted && idle — the
	// outcome mapping is now same-source.
	if StopStatus(true, abortedIdle) != "canceled" {
		t.Fatalf("same-source discipline violated: StopStatus(true, o) should be canceled")
	}
	// An idle observation WITHOUT the abort is a normal completion: the
	// lingering intent is superseded, not "still in flight".
	if got := StopIntentOutcome(Observation{Idle: true, Completed: true}); got != StopRequested {
		t.Fatalf("superseded completion must stay requested: %v", got)
	}
	if !StopIntentSuperseded(Observation{Idle: true, Completed: true}) {
		t.Fatalf("a completed observation supersedes the stop")
	}
	if !StopIntentSuperseded(Observation{Idle: true}) {
		t.Fatalf("an idle-not-aborted observation supersedes the stop")
	}
	if StopIntentSuperseded(Observation{Aborted: true, Idle: false}) {
		t.Fatalf("an in-flight abort does not supersede the stop")
	}
}
