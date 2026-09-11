package commercial

import (
	"testing"
	"time"
)

func TestMonthlyGrantKeySurvivesWorkerReplay(t *testing.T) {
	a := time.Date(2028, 2, 29, 2, 0, 0, 0, time.UTC)
	if MonthlyGrantKey("s1", a) != MonthlyGrantKey("s1", a.In(time.FixedZone("display", 3600))) {
		t.Fatal("display zone changed key")
	}
	if MonthlyGrantKey("s1", a) == MonthlyGrantKey("s2", a) {
		t.Fatal("subscriptions collided")
	}
}

func TestLifecycleDueMonthsAnnualGrantsMonthByMonth(t *testing.T) {
	anchor := time.Date(2028, 1, 15, 9, 0, 0, 0, time.UTC)
	sub := Subscription{
		ID:        "sub_annual",
		Plan:      PlanVersion{Key: "pro", Version: 1, Monthly: Credits(1000)},
		Anchor:    anchor,
		PaidUntil: MonthBoundary(anchor, 12),
	}
	if got := len(sub.DueMonths(anchor.Add(24 * time.Hour))); got != 1 {
		t.Fatalf("first month only, got %d due", got)
	}
	if got := len(sub.DueMonths(MonthBoundary(anchor, 1))); got != 2 {
		t.Fatalf("two elapsed months, got %d due", got)
	}
	if got := len(sub.DueMonths(MonthBoundary(anchor, 4))); got != 5 {
		t.Fatalf("annual must never grant all months at once: got %d due", got)
	}
}

func TestLifecycleEarlyRenewalDoesNotGrantFutureMonths(t *testing.T) {
	anchor := time.Date(2028, 3, 10, 9, 0, 0, 0, time.UTC)
	sub := Subscription{
		ID:        "sub_early",
		Plan:      PlanVersion{Key: "pro", Version: 1, Monthly: Credits(1000)},
		Anchor:    anchor,
		PaidUntil: MonthBoundary(anchor, 1),
	}
	// Early renewal extends paid_until by a full year before the first term ends.
	sub.PaidUntil = MonthBoundary(anchor, 13)
	now := MonthBoundary(anchor, 2)
	due := sub.DueMonths(now)
	if len(due) != 3 {
		t.Fatalf("only elapsed months are due after early renewal, got %d", len(due))
	}
	for _, m := range due {
		if m.After(now) {
			t.Fatalf("future month %v granted early", m)
		}
	}
}

func TestLifecycleExpiryProjectsBaseTierWithOverLimitReason(t *testing.T) {
	anchor := time.Date(2028, 5, 1, 9, 0, 0, 0, time.UTC)
	sub := Subscription{
		ID:        "sub_exp",
		Plan:      PlanVersion{Key: "pro", Version: 2, Monthly: Credits(1000)},
		Anchor:    anchor,
		PaidUntil: MonthBoundary(anchor, 1),
	}
	active := sub.ProjectionAt(MonthBoundary(anchor, 1).Add(-24 * time.Hour))
	if active.Downgraded || active.Plan.Key != "pro" || active.Reason != "" {
		t.Fatalf("active projection must keep the purchased plan: %+v", active)
	}
	expired := sub.ProjectionAt(MonthBoundary(anchor, 1))
	if !expired.Downgraded || expired.Plan.Key != BaseTier.Key || expired.Reason != DowngradeReasonOverLimit {
		t.Fatalf("expiry must project base tier with over-limit reason: %+v", expired)
	}
	if due := sub.DueMonths(MonthBoundary(anchor, 1)); len(due) != 0 {
		t.Fatalf("expired subscription must not schedule new grants, got %v", due)
	}
}

func TestLifecycleDueMonthsBoundProtectsAgainstRunawayWalk(t *testing.T) {
	anchor := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	far := time.Date(2900, 1, 1, 0, 0, 0, 0, time.UTC)
	sub := Subscription{ID: "sub_old", Anchor: anchor, PaidUntil: far}
	if got := len(sub.DueMonths(far)); got > MaxDueMonths {
		t.Fatalf("walk exceeded safety cap: %d", got)
	}
}
