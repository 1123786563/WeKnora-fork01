package craft

// CFT-S03-T023: the cancel-acceptance matrix. StopStatus is the authority
// that separates "the abort was accepted" from "the execution is canceled":
// an accepted abort HTTP call alone never claims cancellation. The store's
// late-approval rejection (terminal interaction → ErrGone), the stale-
// revision conflict and the crash-window no-duplicate delivery are pinned
// in their own suites (interaction_store / decision_delivery tests).
import "testing"

func TestStopStatusAcceptanceNeverClaimsCancellation(t *testing.T) {
	cases := []struct {
		name      string
		requested bool
		obs       Observation
		want      string
	}{
		{"no stop requested keeps running", false, Observation{}, "running"},
		{"accepted but unconfirmed stays stopping", true, Observation{}, "stopping"},
		{"aborted but still busy stays stopping", true, Observation{Aborted: true}, "stopping"},
		{"idle without abort evidence stays stopping", true, Observation{Idle: true}, "stopping"},
		{"aborted AND idle is canceled", true, Observation{Aborted: true, Idle: true}, "canceled"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := StopStatus(tc.requested, tc.obs); got != tc.want {
				t.Fatalf("StopStatus(requested=%v, obs=%+v) = %q, want %q", tc.requested, tc.obs, got, tc.want)
			}
		})
	}
}
