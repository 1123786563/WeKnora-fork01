package handler

import "testing"

// W37 carry-forward (W36 review Important): GET /system/capabilities must
// advertise the protocol compatibility window [minimum, maximum] the server
// serves, so a mobile build can classify itself via
// packages/domain/src/mobile/compatibility.ts clientGate before issuing
// control commands.

func allDeploymentFeaturesAvailableHandler() DeploymentFeatureAvailability {
	return DeploymentFeatureAvailability{
		Organizations: true,
		Agents:        true,
		IM:            true,
		Embed:         true,
		API:           true,
		MCP:           true,
		WebSearch:     true,
		VectorStore:   true,
		Storage:       true,
		Sandbox:       true,
	}
}

func TestDeploymentCapabilitiesAdvertiseDefaultProtocolWindow(t *testing.T) {
	result := BuildDeploymentCapabilities("standard", allDeploymentFeaturesAvailableHandler())
	if result.ProtocolMinimum != DefaultProtocolMinimum || result.ProtocolMaximum != DefaultProtocolMaximum {
		t.Fatalf("default protocol window = [%d,%d], want constants [%d,%d]",
			result.ProtocolMinimum, result.ProtocolMaximum, DefaultProtocolMinimum, DefaultProtocolMaximum)
	}
	if DefaultProtocolMinimum != 2 || DefaultProtocolMaximum != 3 {
		t.Fatalf("the constants must stay aligned with SERVER_PROTOCOL_WINDOW {minimum:2, maximum:3}; got [%d,%d]",
			DefaultProtocolMinimum, DefaultProtocolMaximum)
	}
}

func TestDeploymentCapabilitiesApplyConfiguredProtocolWindow(t *testing.T) {
	result := BuildDeploymentCapabilities("standard", allDeploymentFeaturesAvailableHandler()).
		WithProtocolWindow(3, 4)
	if result.ProtocolMinimum != 3 || result.ProtocolMaximum != 4 {
		t.Fatalf("configured window = [%d,%d], want [3,4]", result.ProtocolMinimum, result.ProtocolMaximum)
	}
}

func TestDeploymentCapabilitiesRejectInvalidProtocolWindows(t *testing.T) {
	base := BuildDeploymentCapabilities("standard", allDeploymentFeaturesAvailableHandler()).WithProtocolWindow(2, 4)
	for _, tc := range [][2]int{
		{0, 3},  // minimum below 1
		{-1, 3}, // negative generation
		{4, 3},  // maximum below minimum
		{2, 0},  // non-positive maximum
	} {
		result := base.WithProtocolWindow(tc[0], tc[1])
		if result.ProtocolMinimum != 2 || result.ProtocolMaximum != 4 {
			t.Fatalf("invalid window [%d,%d] must keep the last valid window [2,4]; got [%d,%d]",
				tc[0], tc[1], result.ProtocolMinimum, result.ProtocolMaximum)
		}
	}
}
