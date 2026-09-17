package service

import (
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types"
)

func w34GateBool(v bool) *bool { return &v }

// w34DrainTestConfig builds the config of a deployment with durable tRPC
// recovery fully enabled, optionally draining (W34 worker_drain).
func w34DrainTestConfig(drain bool) *config.Config {
	return &config.Config{
		Agent: &config.AgentConfig{Recovery: config.AgentRecoveryConfig{
			Enabled:          w34GateBool(true),
			AdmissionEnabled: w34GateBool(true),
		}},
		Workbench: &config.WorkbenchConfig{WorkerDrain: w34GateBool(drain)},
	}
}

// TestSubmitDurableAgentRunRefusesAdmissionUnderWorkbenchDrain is the W34
// production wiring test: the live tRPC admission entrypoint must honour
// worker drain — setting WEKNORA_WORKBENCH_WORKER_DRAIN=true during a
// rolling upgrade or incident must refuse NEW admissions instead of being a
// silent no-op.
func TestSubmitDurableAgentRunRefusesAdmissionUnderWorkbenchDrain(t *testing.T) {
	svc := &sessionService{cfg: w34DrainTestConfig(true)}
	err := svc.submitDurableAgentRun(durableRunCtx(), &types.QARequest{Query: "hello"}, &types.AgentConfig{}, "model-1", false)
	if err == nil || !strings.Contains(err.Error(), "drain") {
		t.Fatalf("expected a drain rejection, got %v", err)
	}
}

// TestSubmitDurableAgentRunPassesAdmissionGateWhenNotDraining pins the
// negative space: without drain the same entrypoint must NOT reject with a
// drain error (the next failure belongs to the un-registered run service,
// proving the gate itself opened).
func TestSubmitDurableAgentRunPassesAdmissionGateWhenNotDraining(t *testing.T) {
	svc := &sessionService{cfg: w34DrainTestConfig(false)}
	err := svc.submitDurableAgentRun(durableRunCtx(), &types.QARequest{Query: "hello"}, &types.AgentConfig{}, "model-1", false)
	if err == nil {
		t.Fatal("expected the downstream unavailability failure, not success")
	}
	if strings.Contains(err.Error(), "drain") {
		t.Fatalf("gate should be open when not draining, got %v", err)
	}
}
