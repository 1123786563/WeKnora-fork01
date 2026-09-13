package config

import (
	"strings"
	"testing"
)

// TestOpenConnectorDefaultsToDisabled pins the SAFE-OFF default: nil section,
// nil pointer, and explicit false all leave the dispatch path closed.
func TestOpenConnectorDefaultsToDisabled(t *testing.T) {
	var nilSection *OpenConnectorConfig
	if nilSection.IsEnabled() {
		t.Fatal("nil section must be disabled")
	}
	if (&OpenConnectorConfig{}).IsEnabled() {
		t.Fatal("nil enabled pointer must be disabled")
	}
	off := false
	if (&OpenConnectorConfig{Enabled: &off}).IsEnabled() {
		t.Fatal("explicit false must be disabled")
	}
}

// TestOpenConnectorRuntimeAddrInternalMapOnly: known ids resolve; unknown
// ids — including raw URLs — do not. The map is the only way config reaches
// an address.
func TestOpenConnectorRuntimeAddrInternalMapOnly(t *testing.T) {
	addr, ok := OpenConnectorRuntimeAddr("shared")
	if !ok || addr != "http://open-connector:3000" {
		t.Fatalf("shared -> %q, %v", addr, ok)
	}
	for _, bad := range []string{"", "http://evil.example", "independent"} {
		if _, ok := OpenConnectorRuntimeAddr(bad); ok {
			t.Fatalf("id %q resolved", bad)
		}
	}
	// surrounding whitespace is trimmed, not part of the id.
	if addr, ok := OpenConnectorRuntimeAddr(" shared "); !ok || addr != "http://open-connector:3000" {
		t.Fatalf("trimmed id -> %q, %v", addr, ok)
	}
}

// TestValidateConfigOpenConnectorEnabledRequiresKnownRuntime pins the
// startup validation: enabled without a runtime id, or with an id that is
// not in the internal map, is a config error.
func TestValidateConfigOpenConnectorEnabledRequiresKnownRuntime(t *testing.T) {
	on := true
	cases := []struct {
		name    string
		runtime string
		wantErr string
	}{
		{"missing runtime", "", "open_connector.runtime is required"},
		{"unknown runtime", "http://elsewhere:3000", "not a known internal runtime id"},
		{"valid runtime", "shared", ""},
	}
	for _, tc := range cases {
		cfg := &Config{OpenConnector: &OpenConnectorConfig{Enabled: &on, Runtime: tc.runtime}}
		err := ValidateConfig(cfg)
		if tc.wantErr == "" {
			if err != nil {
				t.Fatalf("%s: unexpected error %v", tc.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
			t.Fatalf("%s: error=%v want~%q", tc.name, err, tc.wantErr)
		}
	}
	// Disabled: an unknown runtime value is inert.
	cfg := &Config{OpenConnector: &OpenConnectorConfig{Runtime: "http://inert:1"}}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("disabled section must not error: %v", err)
	}
}

// TestApplyOpenConnectorDefaultsEnvOverrides pins the env layering: the
// override can only ever ENABLE (safe-off polarity) and can retarget the
// runtime id; unset vars leave yaml untouched.
func TestApplyOpenConnectorDefaultsEnvOverrides(t *testing.T) {
	t.Setenv("WEKNORA_OPEN_CONNECTOR_ENABLED", "")
	t.Setenv("WEKNORA_OPEN_CONNECTOR_RUNTIME", "")
	cfg := &Config{OpenConnector: &OpenConnectorConfig{Runtime: "shared"}}
	applyOpenConnectorDefaults(cfg)
	if cfg.OpenConnector.IsEnabled() {
		t.Fatal("no env, no yaml flag: must stay disabled")
	}

	t.Setenv("WEKNORA_OPEN_CONNECTOR_ENABLED", "true")
	applyOpenConnectorDefaults(cfg)
	if !cfg.OpenConnector.IsEnabled() {
		t.Fatal("env true must enable")
	}

	t.Setenv("WEKNORA_OPEN_CONNECTOR_ENABLED", "false")
	cfg2 := &Config{OpenConnector: &OpenConnectorConfig{Runtime: "shared"}}
	applyOpenConnectorDefaults(cfg2)
	if cfg2.OpenConnector.IsEnabled() {
		t.Fatal("env false is not an enable; yaml had no flag, must stay disabled")
	}

	t.Setenv("WEKNORA_OPEN_CONNECTOR_RUNTIME", "shared")
	cfg3 := &Config{}
	applyOpenConnectorDefaults(cfg3)
	if cfg3.OpenConnector.Runtime != "shared" {
		t.Fatalf("runtime=%q", cfg3.OpenConnector.Runtime)
	}
}
