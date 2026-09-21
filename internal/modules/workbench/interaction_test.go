package workbench

import (
	"errors"
	"testing"
)

func TestInteractionDomainsDoNotGrantEachOther(t *testing.T) {
	for _, pair := range [][2]string{{"recovery", "approve"}, {"budget", "approve"}, {"tool_approval", "extend"}} {
		if ValidateInteractionAction(pair[0], pair[1]) == nil {
			t.Fatalf("accepted %v", pair)
		}
	}
	if err := ValidateInteractionAction("tool_approval", "approve"); err != nil {
		t.Fatal(err)
	}
}

func TestExecutionCommandIsAClosedUnion(t *testing.T) {
	for _, command := range []ExecutionCommand{
		{Action: "cancel", Text: "unexpected"},
		{Action: "steer"},
		{Action: "shell", Text: "rm -rf /"},
	} {
		if !errors.Is(command.Validate(), ErrCommandActionMismatch) {
			t.Fatalf("command %+v was accepted", command)
		}
	}
	if err := (ExecutionCommand{Action: "cancel"}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (ExecutionCommand{Action: "steer", Text: "continue with sources"}).Validate(); err != nil {
		t.Fatal(err)
	}
}
