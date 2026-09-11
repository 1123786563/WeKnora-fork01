package recoverytest

import "testing"

func TestCrashAfterToolResult(t *testing.T) {
	r := runCrashCase(t, "after_tool_result_before_checkpoint")
	if r.ExternalCalls != 1 || r.FinalStatus != "succeeded" || r.AssistantRows != 1 || r.LostEvents != 0 {
		t.Fatalf("unexpected crash report: %#v", r)
	}
}
