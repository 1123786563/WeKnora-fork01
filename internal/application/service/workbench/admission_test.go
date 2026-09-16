package workbench

import (
	"errors"
	"testing"
)

func TestAdmissionNeverDispatchesWithoutBudget(t *testing.T) {
	called := false
	deny := errors.New("budget_denied")
	err := admitThenPublish(func() error { return deny }, func() error { called = true; return nil })
	if !errors.Is(err, deny) || called {
		t.Fatal("unfunded execution was published")
	}
}

func TestRequestHashChangesWithImmutableInput(t *testing.T) {
	a := StartInput{SessionID: "s", AgentID: "a", TargetID: "platform", Text: "hello", BudgetUpper: 10}
	b := a
	b.Text = "changed"
	if requestHash(a) == requestHash(b) {
		t.Fatal("request hash ignored immutable input")
	}
}
