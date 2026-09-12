// Package config tests cover deployment defaults for the durable tRPC
// recovery flags (opt-in: unset means disabled).
package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAgentRecoveryDefaultDisabled pins the opt-in default: a deployment
// that says nothing about agent.recovery cannot admit durable work.
func TestAgentRecoveryDefaultDisabled(t *testing.T) {
	unset := AgentRecoveryConfig{}
	require.False(t, unset.RecoveryEnabled(), "unset recovery must default to disabled")
	require.False(t, unset.RecoveryAdmissionEnabled(), "unset admission must default to disabled")

	on := true
	enabled := AgentRecoveryConfig{Enabled: &on, AdmissionEnabled: &on}
	require.True(t, enabled.RecoveryEnabled())
	require.True(t, enabled.RecoveryAdmissionEnabled())

	off := false
	workerOff := AgentRecoveryConfig{Enabled: &off, AdmissionEnabled: &on}
	require.False(t, workerOff.RecoveryEnabled())
	require.True(t, workerOff.RecoveryAdmissionEnabled())

	admissionOff := AgentRecoveryConfig{Enabled: &on, AdmissionEnabled: &off}
	require.True(t, admissionOff.RecoveryEnabled())
	require.False(t, admissionOff.RecoveryAdmissionEnabled())
}

func TestAgentRecoveryEnvOverrides(t *testing.T) {
	t.Setenv("WEKNORA_AGENT_RECOVERY_ENABLED", "true")
	t.Setenv("WEKNORA_AGENT_RECOVERY_ADMISSION_ENABLED", "true")
	cfg := &Config{Agent: &AgentConfig{}}

	applyAgentEnvOverrides(cfg)

	require.True(t, cfg.Agent.Recovery.RecoveryEnabled())
	require.True(t, cfg.Agent.Recovery.RecoveryAdmissionEnabled())
}

func TestAgentRecoveryInvalidEnvKeepsDefault(t *testing.T) {
	t.Setenv("WEKNORA_AGENT_RECOVERY_ENABLED", "not-a-bool")
	cfg := &Config{Agent: &AgentConfig{}}

	applyAgentEnvOverrides(cfg)

	require.False(t, cfg.Agent.Recovery.RecoveryEnabled())
}
