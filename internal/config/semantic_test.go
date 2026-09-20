package config

import (
	"strings"
	"testing"
)

func TestSemanticClientConfigIsOptIn(t *testing.T) {
	cfg := &Config{}
	applySemanticEnvOverrides(cfg)
	if cfg.Semantic == nil || cfg.Semantic.Enabled {
		t.Fatalf("semantic default = %#v, want disabled config", cfg.Semantic)
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("disabled semantic config should be valid: %v", err)
	}
}

func TestSemanticEnabledRequiresTLSAndIdentityConfig(t *testing.T) {
	err := ValidateConfig(&Config{Semantic: &SemanticServiceConfig{Enabled: true}})
	if err == nil || !strings.Contains(err.Error(), "semantic address") {
		t.Fatalf("validation error = %v, want required semantic config", err)
	}
}

func TestSemanticEnabledRequiresSeparateScopeSigningKey(t *testing.T) {
	cfg := &Config{Semantic: &SemanticServiceConfig{Enabled: true, Address: "service:50052", ServerName: "service", RootCAPath: "ca.pem", ServiceToken: strings.Repeat("t", 32), Audience: "semantic"}}
	for _, key := range []string{"", "short", strings.Repeat(" ", 40), "a" + strings.Repeat(" ", 30) + "b", strings.Repeat("t", 32)} {
		cfg.Semantic.ScopeSigningKey = key
		if ValidateConfig(cfg) == nil {
			t.Fatal("accepted invalid signing key")
		}
	}
	cfg.Semantic.ScopeSigningKey = strings.Repeat("k", 32)
	if err := ValidateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SEMANTIC_SCOPE_SIGNING_KEY", strings.Repeat("e", 32))
	applySemanticEnvOverrides(cfg)
	if cfg.Semantic.ScopeSigningKey != strings.Repeat("e", 32) {
		t.Fatal("environment override missing")
	}
}
