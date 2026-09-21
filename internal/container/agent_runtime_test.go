package container

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/config"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/execution"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func w34Bool(v bool) *bool { return &v }

// w34RunStore builds a real AgentRunStore over a throwaway in-memory sqlite
// database. Assembly-level tests only construct runtimes; they never execute
// runs, so no migrations are needed.
func w34RunStore(t *testing.T) *repository.AgentRunStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	return repository.NewAgentRunStore(db)
}

// TestWorkbenchAdmissionGatesCloseIndependently pins the W34 capability
// switch consumption at the runtime assembly layer: closing one admission
// lane must not close the others, and worker drain refuses NEW admissions
// everywhere while already-admitted work keeps running.
func TestWorkbenchAdmissionGatesCloseIndependently(t *testing.T) {
	recoveryOn := &config.AgentRecoveryConfig{
		Enabled:          w34Bool(true),
		AdmissionEnabled: w34Bool(true),
	}
	baseline := &config.Config{Agent: &config.AgentConfig{Recovery: *recoveryOn}}
	if !AgentRecoveryAdmissionEnabled(baseline) {
		t.Fatal("baseline durable admission should be open")
	}
	if !WorkbenchPlatformAdmissionEnabled(baseline) {
		t.Fatal("baseline platform admission should be open")
	}
	if !WorkbenchPaseoAdmissionEnabled(baseline) {
		t.Fatal("baseline paseo admission should be open")
	}

	draining := &config.Config{
		Agent:     &config.AgentConfig{Recovery: *recoveryOn},
		Workbench: &config.WorkbenchConfig{WorkerDrain: w34Bool(true)},
	}
	if AgentRecoveryAdmissionEnabled(draining) {
		t.Fatal("worker drain must refuse NEW durable admissions")
	}
	if WorkbenchPlatformAdmissionEnabled(draining) {
		t.Fatal("worker drain must refuse NEW platform admissions")
	}
	if WorkbenchPaseoAdmissionEnabled(draining) {
		t.Fatal("worker drain must refuse NEW paseo admissions")
	}

	platformClosed := &config.Config{
		Agent:     &config.AgentConfig{Recovery: *recoveryOn},
		Workbench: &config.WorkbenchConfig{PlatformAdmission: w34Bool(false)},
	}
	if WorkbenchPlatformAdmissionEnabled(platformClosed) {
		t.Fatal("platform admission switch must close the platform lane")
	}
	if !WorkbenchPaseoAdmissionEnabled(platformClosed) {
		t.Fatal("closing platform admission must not close the paseo lane")
	}

	paseoClosed := &config.Config{
		Agent:     &config.AgentConfig{Recovery: *recoveryOn},
		Workbench: &config.WorkbenchConfig{PaseoAdmission: w34Bool(false)},
	}
	if WorkbenchPaseoAdmissionEnabled(paseoClosed) {
		t.Fatal("paseo admission switch must close the paseo lane")
	}
	if !WorkbenchPlatformAdmissionEnabled(paseoClosed) {
		t.Fatal("closing paseo admission must not close the platform lane")
	}
}

// TestAgentRuntimeAssemblesUnderDrain verifies the drain semantics that
// matter for rolling upgrades: the worker is still constructed and enabled,
// so already-admitted runs finish and cleanup paths stay reachable — only
// NEW admissions are refused.
func TestAgentRuntimeAssemblesUnderDrain(t *testing.T) {
	cfg := &config.Config{
		Agent: &config.AgentConfig{Recovery: config.AgentRecoveryConfig{
			Enabled:          w34Bool(true),
			AdmissionEnabled: w34Bool(true),
		}},
		Workbench: &config.WorkbenchConfig{WorkerDrain: w34Bool(true)},
	}
	r, err := NewAgentRuntime(cfg, w34RunStore(t), func(context.Context, agentruntime.Fence) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if r == nil || r.Worker == nil {
		t.Fatal("drain must still assemble a worker")
	}
	if !r.Worker.Config().Enabled {
		t.Fatal("drain must keep the worker enabled so admitted runs finish")
	}
}

// TestAgentRuntimeDrainCountsStopUnconfirmed ties the runtime stop budget to
// the W34 stop_unconfirmed signal: a worker that cannot confirm exit within
// its drain budget is counted, never silently treated as a clean stop.
func TestAgentRuntimeDrainCountsStopUnconfirmed(t *testing.T) {
	execution.ResetExecutionDeploymentMetrics()
	defer execution.ResetExecutionDeploymentMetrics()

	worker, err := service.NewAgentRunWorker(
		w34RunStore(t),
		func(context.Context, agentruntime.Fence) error { return nil },
		service.WorkerConfig{Enabled: false},
	)
	if err != nil {
		t.Fatal(err)
	}
	// The worker loop is intentionally NOT started, so Wait cannot observe
	// a closed done channel within the budget — the unconfirmed path.
	r := &AgentRuntime{Worker: worker}
	r.Drain()

	snap := execution.ExecutionDeploymentMetricsSnapshot()
	if got := snap["execution_stop_unconfirmed_total"]; got != 1 {
		t.Fatalf("execution_stop_unconfirmed_total = %v, want 1", got)
	}
}
