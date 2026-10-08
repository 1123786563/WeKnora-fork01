package execution

import "testing"

func TestRemoteUsageCannotSelfAuthorizeBilling(t *testing.T) {
	if AllowModelSettlement("personal_node", "platform") {
		t.Fatal("untrusted report charged")
	}
	if AllowModelSettlement("platform_gateway", "byok") {
		t.Fatal("BYOK double charged")
	}
	if !AllowModelSettlement("platform_gateway", "platform") {
		t.Fatal("trusted usage rejected")
	}
}
