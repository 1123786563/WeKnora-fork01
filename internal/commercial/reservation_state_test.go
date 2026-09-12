package commercial

import "testing"

func TestExpiredLeaseDoesNotReleaseDispatchedCost(t *testing.T) {
	if MayReleaseWithoutQuery("dispatched") {
		t.Fatal("unknown spend released")
	}
	if MayReleaseWithoutQuery("settling") {
		t.Fatal("pending settlement released")
	}
	if !MayReleaseWithoutQuery("held") {
		t.Fatal("unstarted reservation stranded")
	}
}

// TestMayReleaseWithoutQueryStateTable pins the whole release state table:
// only an unstarted (held) reservation releases without querying the
// provider; every state that implies possible external spend — dispatched,
// settling, settled, released — and every unrecognized state keeps the
// protection (fail-closed, never a licence to zero spend).
func TestMayReleaseWithoutQueryStateTable(t *testing.T) {
	cases := map[string]bool{
		"held":       true,
		"dispatched": false,
		"settling":   false,
		"settled":    false,
		"released":   false,
		"confirmed":  false,
		"unknown":    false,
		"":           false,
	}
	for state, want := range cases {
		if got := MayReleaseWithoutQuery(state); got != want {
			t.Fatalf("state %q: MayReleaseWithoutQuery=%v, want %v", state, got, want)
		}
	}
}
