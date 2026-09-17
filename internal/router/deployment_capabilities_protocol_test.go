package router

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
)

// W37 carry-forward: the router binds the config-resolved protocol window
// into the /system/capabilities snapshot it hands the system handler.

func intPtrRouter(v int) *int { return &v }

func TestRouterAppliesConfiguredProtocolWindowToCapabilities(t *testing.T) {
	// nil config (unit wiring without dig) keeps the default window.
	defaults := deploymentCapabilitiesFromRouter(RouterParams{})
	if defaults.ProtocolMinimum != 2 || defaults.ProtocolMaximum != 3 {
		t.Fatalf("nil-config window = [%d,%d], want defaults [2,3]", defaults.ProtocolMinimum, defaults.ProtocolMaximum)
	}

	// An explicit config window reaches the advertised snapshot.
	params := RouterParams{Config: &config.Config{Workbench: &config.WorkbenchConfig{
		ProtocolMinimum: intPtrRouter(3),
		ProtocolMaximum: intPtrRouter(5),
	}}}
	data := deploymentCapabilitiesFromRouter(params)
	if data.ProtocolMinimum != 3 || data.ProtocolMaximum != 5 {
		t.Fatalf("configured window = [%d,%d], want [3,5]", data.ProtocolMinimum, data.ProtocolMaximum)
	}
}
