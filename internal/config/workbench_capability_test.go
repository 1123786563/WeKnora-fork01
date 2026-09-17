package config

import (
	"testing"
)

// W34 capability switches: each lane closes independently so an operator can
// stop NEW work of one kind without cutting reads or cleanup of the others.

func boolPtr(v bool) *bool { return &v }

func TestWorkbenchCapabilityDefaultsAreSafeOn(t *testing.T) {
	var cfg Config // no workbench section at all
	if !cfg.AreWorkbenchReadsEnabled() {
		t.Fatal("reads must stay enabled when the section is unset")
	}
	if !cfg.IsWorkbenchPlatformAdmissionEnabled() {
		t.Fatal("platform admission must stay enabled when the section is unset")
	}
	if !cfg.IsWorkbenchPaseoAdmissionEnabled() {
		t.Fatal("paseo admission gate must stay open (enabling Paseo itself is opt-in elsewhere)")
	}
	if !cfg.IsWorkbenchVoiceAdmissionEnabled() {
		t.Fatal("voice admission must stay enabled when the section is unset")
	}
	if !cfg.AreWorkbenchNotificationsEnabled() {
		t.Fatal("notifications must stay enabled when the section is unset")
	}
	if cfg.IsWorkbenchWorkerDraining() {
		t.Fatal("worker drain must be off unless explicitly requested")
	}
}

func TestWorkbenchCapabilitySwitchesCloseIndependently(t *testing.T) {
	cfg := &Config{Workbench: &WorkbenchConfig{
		PlatformAdmission: boolPtr(false),
		WorkerDrain:       boolPtr(true),
	}}
	if !cfg.AreWorkbenchReadsEnabled() {
		t.Fatal("closing platform admission must not close reads")
	}
	if cfg.IsWorkbenchPlatformAdmissionEnabled() {
		t.Fatal("platform admission should be closed")
	}
	if !cfg.IsWorkbenchPaseoAdmissionEnabled() {
		t.Fatal("paseo admission should be unaffected")
	}
	if !cfg.AreWorkbenchNotificationsEnabled() {
		t.Fatal("notifications should be unaffected")
	}
	if !cfg.IsWorkbenchWorkerDraining() {
		t.Fatal("worker drain should be on")
	}
}

func TestWorkbenchCapabilityEnvOverrides(t *testing.T) {
	t.Setenv("WEKNORA_WORKBENCH_PLATFORM_ADMISSION", "false")
	t.Setenv("WEKNORA_WORKBENCH_WORKER_DRAIN", "true")
	t.Setenv("WEKNORA_WORKBENCH_NOTIFICATIONS_ENABLED", "false")

	var cfg Config
	applyWorkbenchCapabilityDefaults(&cfg)

	if cfg.IsWorkbenchPlatformAdmissionEnabled() {
		t.Fatal("env override should close platform admission")
	}
	if !cfg.IsWorkbenchWorkerDraining() {
		t.Fatal("env override should enable worker drain")
	}
	if cfg.AreWorkbenchNotificationsEnabled() {
		t.Fatal("env override should disable notifications")
	}
	if !cfg.AreWorkbenchReadsEnabled() {
		t.Fatal("reads must not be affected by unrelated overrides")
	}
}

func TestWorkbenchCapabilityUnparseableEnvIgnored(t *testing.T) {
	t.Setenv("WEKNORA_WORKBENCH_WORKER_DRAIN", "not-a-bool")

	var cfg Config
	applyWorkbenchCapabilityDefaults(&cfg)

	if cfg.IsWorkbenchWorkerDraining() {
		t.Fatal("unparseable env must never enable drain")
	}
}
