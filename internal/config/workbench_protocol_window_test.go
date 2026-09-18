package config

import "testing"

// W37 carry-forward: the protocol compatibility window advertised by
// /system/capabilities comes from config (constants + override), mirroring
// packages/domain/src/mobile/compatibility.ts SERVER_PROTOCOL_WINDOW.

func intPtr(v int) *int { return &v }

func TestProtocolWindowDefaultsWhenUnset(t *testing.T) {
	var cfg Config // no workbench section at all
	minimum, maximum := cfg.ProtocolWindow()
	if minimum != 2 || maximum != 3 {
		t.Fatalf("default window = [%d,%d], want [2,3] (aligned with the TS SERVER_PROTOCOL_WINDOW)", minimum, maximum)
	}
}

func TestProtocolWindowConfigOverride(t *testing.T) {
	cfg := &Config{Workbench: &WorkbenchConfig{
		ProtocolMinimum: intPtr(3),
		ProtocolMaximum: intPtr(5),
	}}
	minimum, maximum := cfg.ProtocolWindow()
	if minimum != 3 || maximum != 5 {
		t.Fatalf("configured window = [%d,%d], want [3,5]", minimum, maximum)
	}
}

func TestProtocolWindowEnvOverride(t *testing.T) {
	t.Setenv("WEKNORA_WORKBENCH_PROTOCOL_MINIMUM", "2")
	t.Setenv("WEKNORA_WORKBENCH_PROTOCOL_MAXIMUM", "4")
	var cfg Config
	applyWorkbenchCapabilityDefaults(&cfg)
	minimum, maximum := cfg.ProtocolWindow()
	if minimum != 2 || maximum != 4 {
		t.Fatalf("env-overridden window = [%d,%d], want [2,4]", minimum, maximum)
	}
}

func TestProtocolWindowInvalidOverrideFallsBackToDefaults(t *testing.T) {
	t.Setenv("WEKNORA_WORKBENCH_PROTOCOL_MINIMUM", "not-a-number")
	t.Setenv("WEKNORA_WORKBENCH_PROTOCOL_MAXIMUM", "0")
	var cfg Config
	applyWorkbenchCapabilityDefaults(&cfg)
	minimum, maximum := cfg.ProtocolWindow()
	if minimum != 2 || maximum != 3 {
		t.Fatalf("invalid overrides must not corrupt the window; got [%d,%d], want [2,3]", minimum, maximum)
	}
}
