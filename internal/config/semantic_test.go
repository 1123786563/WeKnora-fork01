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
