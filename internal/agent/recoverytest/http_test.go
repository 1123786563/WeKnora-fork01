package recoverytest

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/container"
)

func boolPtr(v bool) *bool { return &v }

func TestRecoveryAdmissionGate(t *testing.T) {
	cases := []struct {
		name string
		cfg  *config.Config
		want bool
	}{
		{name: "nil", want: false},
		{name: "disabled", cfg: &config.Config{Agent: &config.AgentConfig{Recovery: config.AgentRecoveryConfig{Enabled: boolPtr(false)}}}, want: false},
		{name: "worker only drains", cfg: &config.Config{Agent: &config.AgentConfig{Recovery: config.AgentRecoveryConfig{Enabled: boolPtr(true), AdmissionEnabled: boolPtr(false)}}}, want: false},
		{name: "admission enabled", cfg: &config.Config{Agent: &config.AgentConfig{Recovery: config.AgentRecoveryConfig{Enabled: boolPtr(true), AdmissionEnabled: boolPtr(true)}}}, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := container.AgentRecoveryAdmissionEnabled(tc.cfg); got != tc.want {
				t.Fatalf("gate = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRecoveryAdmissionGateRejectsInconsistentConfig(t *testing.T) {
	cfg := &config.Config{Agent: &config.AgentConfig{Recovery: config.AgentRecoveryConfig{Enabled: boolPtr(false), AdmissionEnabled: boolPtr(true)}}}
	if err := container.ValidateAgentRuntimeConfig(cfg); err == nil {
		t.Fatal("expected admission to require an enabled recovery worker")
	}
}
