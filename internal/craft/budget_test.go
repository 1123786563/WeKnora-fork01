package craft

import (
	"testing"
	"time"
)

// TestBudgetCannotRunAfterDeadline is the O02 Step 1 RED acceptance, verbatim
// from the brief: an expired grant and a cap-exhausted grant both refuse.
func TestBudgetCannotRunAfterDeadline(t *testing.T) {
	now := time.Unix(100, 0)
	g := BudgetGrant{ID: "g", Deadline: now, MaxCalls: 3, UsedCalls: 0, Allowed: true}
	if BudgetAllows(g, now) {
		t.Fatal("expired")
	}
	g.Deadline = now.Add(time.Minute)
	g.UsedCalls = 3
	if BudgetAllows(g, now) {
		t.Fatal("call cap")
	}
}

// TestBudgetAllowsRequiresLiveGrantIdentity pins the remaining quick-check
// invariants: a missing grant identity, a denied admission and a negative
// deadline headroom each refuse, while a live grant with headroom passes.
func TestBudgetAllowsRequiresLiveGrantIdentity(t *testing.T) {
	now := time.Unix(100, 0)
	live := BudgetGrant{ID: "g", Deadline: now.Add(time.Minute), MaxCalls: 2, UsedCalls: 1, Allowed: true}
	if !BudgetAllows(live, now) {
		t.Fatal("live grant with headroom must allow")
	}
	noID := live
	noID.ID = ""
	if BudgetAllows(noID, now) {
		t.Fatal("grant without identity must refuse")
	}
	denied := live
	denied.Allowed = false
	if BudgetAllows(denied, now) {
		t.Fatal("denied grant must refuse")
	}
	exactZeroHeadroom := BudgetGrant{ID: "g", Deadline: now.Add(time.Minute), MaxCalls: 0, UsedCalls: 0, Allowed: true}
	if BudgetAllows(exactZeroHeadroom, now) {
		t.Fatal("zero-call grant must refuse")
	}
}
