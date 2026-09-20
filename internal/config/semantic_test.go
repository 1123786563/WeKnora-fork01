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
	cfg.Semantic.ModelSigningKey = strings.Repeat("m", 32)
	if err := ValidateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SEMANTIC_SCOPE_SIGNING_KEY", strings.Repeat("e", 32))
	applySemanticEnvOverrides(cfg)
	if cfg.Semantic.ScopeSigningKey != strings.Repeat("e", 32) {
		t.Fatal("environment override missing")
	}
	t.Setenv("SEMANTIC_MODEL_SIGNING_KEY", strings.Repeat("f", 32))
	applySemanticEnvOverrides(cfg)
	if cfg.Semantic.ModelSigningKey != strings.Repeat("f", 32) {
		t.Fatal("model signing environment override missing")
	}
}

func TestSemanticEnabledRequiresSeparateModelSigningKey(t *testing.T) {
	cfg := &Config{Semantic: &SemanticServiceConfig{Enabled: true, Address: "service:50052", ServerName: "service", RootCAPath: "ca.pem", ServiceToken: strings.Repeat("t", 32), Audience: "semantic", ScopeSigningKey: strings.Repeat("s", 32), ModelSigningKey: strings.Repeat("m", 32)}}
	requireModelKey := func(key string) {
		cfg.Semantic.ModelSigningKey = key
		if ValidateConfig(cfg) == nil {
			t.Fatalf("accepted invalid model signing key")
		}
	}
	requireModelKey("")
	requireModelKey(cfg.Semantic.ScopeSigningKey)
	requireModelKey(cfg.Semantic.ServiceToken)
	cfg.Semantic.ModelSigningKey = strings.Repeat("m", 32)
	if err := ValidateConfig(cfg); err != nil {
		t.Fatal(err)
	}
}
