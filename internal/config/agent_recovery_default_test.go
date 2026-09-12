// Package config tests cover deployment defaults for the durable tRPC
// recovery flags (opt-out: unset means enabled).
package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAgentRecoveryDefaultEnabled pins the opt-out default: a deployment
// that says nothing about agent.recovery gets the durable worker and
// admission enabled; an explicit false disables each independently.
func TestAgentRecoveryDefaultEnabled(t *testing.T) {
	unset := AgentRecoveryConfig{}
	require.True(t, unset.RecoveryEnabled(), "unset recovery must default to enabled")
	require.True(t, unset.RecoveryAdmissionEnabled(), "unset admission must default to enabled")

	off := false
	disabled := AgentRecoveryConfig{Enabled: &off}
	require.False(t, disabled.RecoveryEnabled())
	require.True(t, disabled.RecoveryAdmissionEnabled())

	admissionOff := AgentRecoveryConfig{AdmissionEnabled: &off}
	require.True(t, admissionOff.RecoveryEnabled())
	require.False(t, admissionOff.RecoveryAdmissionEnabled())
}
