// Package config tests for the M4 skillhub_market section: declaration,
// defaulting and nil-safe accessors, mirroring the WebSearchConfig precedent.
package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestApplySkillHubMarketDefaultsFullSection(t *testing.T) {
	cfg := &Config{}
	applySkillHubMarketDefaults(cfg)
	require.NotNil(t, cfg.SkillHubMarket)
	require.Equal(t, DefaultSkillHubMarketHost, cfg.SkillHubMarket.Host)
	require.Equal(t, DefaultSkillHubMarketTimeoutSeconds, cfg.SkillHubMarket.TimeoutSeconds)
}

func TestApplySkillHubMarketDefaultsPartialSection(t *testing.T) {
	cfg := &Config{SkillHubMarket: &SkillHubMarketConfig{Host: " http://localhost:9999/ "}}
	applySkillHubMarketDefaults(cfg)
	require.Equal(t, "http://localhost:9999", cfg.SkillHubMarket.Host)
	require.Equal(t, DefaultSkillHubMarketTimeoutSeconds, cfg.SkillHubMarket.TimeoutSeconds)

	cfg = &Config{SkillHubMarket: &SkillHubMarketConfig{TimeoutSeconds: 5}}
	applySkillHubMarketDefaults(cfg)
	require.Equal(t, DefaultSkillHubMarketHost, cfg.SkillHubMarket.Host)
	require.Equal(t, 5, cfg.SkillHubMarket.TimeoutSeconds)

	cfg = &Config{SkillHubMarket: &SkillHubMarketConfig{TimeoutSeconds: -3}}
	applySkillHubMarketDefaults(cfg)
	require.Equal(t, DefaultSkillHubMarketTimeoutSeconds, cfg.SkillHubMarket.TimeoutSeconds)
}

func TestSkillHubMarketAccessorsNilSafe(t *testing.T) {
	var cfg *Config
	require.Equal(t, DefaultSkillHubMarketHost, cfg.SkillHubMarketHost())
	require.Equal(t, time.Duration(DefaultSkillHubMarketTimeoutSeconds)*time.Second, cfg.SkillHubMarketTimeout())

	empty := &Config{}
	require.Equal(t, DefaultSkillHubMarketHost, empty.SkillHubMarketHost())
	require.Equal(t, time.Duration(DefaultSkillHubMarketTimeoutSeconds)*time.Second, empty.SkillHubMarketTimeout())

	set := &Config{SkillHubMarket: &SkillHubMarketConfig{Host: "http://hub.internal:8443/", TimeoutSeconds: 12}}
	require.Equal(t, "http://hub.internal:8443", set.SkillHubMarketHost())
	require.Equal(t, 12*time.Second, set.SkillHubMarketTimeout())
}

func TestValidateConfigRejectsInvalidSkillHubMarketHost(t *testing.T) {
	cfg := &Config{SkillHubMarket: &SkillHubMarketConfig{Host: "ftp://hub.example.com"}}
	err := ValidateConfig(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "skillhub_market.host")

	cfg = &Config{SkillHubMarket: &SkillHubMarketConfig{Host: "https://hub.example.com"}}
	err = ValidateConfig(cfg)
	require.NoError(t, err)
}
