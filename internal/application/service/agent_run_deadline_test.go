package service

import (
	"context"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/stretchr/testify/require"
)

func TestWorkerFailsRunPastPersistedDeadline(t *testing.T) {
	db := openDurableRunTestDB(t)
	store := repository.NewAgentRunStore(db)
	prev := RegisteredAgentRunService()
	RegisterAgentRunService(NewAgentRunService(store))
	t.Cleanup(func() { RegisterAgentRunService(prev) })

	key := admitDurableRun(t, store, durableRunSnapshot(t))
	// A deadline already in the past: the worker must fail the run with the
	// explicit reason instead of executing or looping on renewals.
	past := time.Now().Add(-time.Minute)
	require.NoError(t, db.Exec(
		"UPDATE agent_runs SET deadline = ? WHERE tenant_id = 1 AND run_id = ?",
		past, key.RunID).Error)

	executed := false
	worker, err := NewAgentRunWorker(store, func(context.Context, agentruntime.Fence) error {
		executed = true
		return nil
	}, WorkerConfig{
		Enabled: true, Lease: time.Minute, Heartbeat: 15 * time.Second,
		ScanInterval: 10 * time.Millisecond, MaxWorkers: 2,
	})
	require.NoError(t, err)
	require.NoError(t, worker.Tick(context.Background()))

	deadline := time.Now().Add(5 * time.Second)
	for {
		run, getErr := store.Get(context.Background(), key)
		require.NoError(t, getErr)
		if run.Status != "queued" && run.Status != "running" {
			require.Equal(t, "failed", run.Status)
			require.Equal(t, "deadline_exceeded", run.WaitReason)
			require.False(t, executed, "a run past its deadline must not execute")
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("run past deadline not failed, status=%s", run.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestWorkerCapsExecutionContextAtDeadline(t *testing.T) {
	db := openDurableRunTestDB(t)
	store := repository.NewAgentRunStore(db)
	prev := RegisteredAgentRunService()
	RegisterAgentRunService(NewAgentRunService(store))
	t.Cleanup(func() { RegisterAgentRunService(prev) })

	key := admitDurableRun(t, store, durableRunSnapshot(t))
	soon := time.Now().Add(300 * time.Millisecond)
	require.NoError(t, db.Exec(
		"UPDATE agent_runs SET deadline = ? WHERE tenant_id = 1 AND run_id = ?",
		soon, key.RunID).Error)

	hung := make(chan struct{})
	worker, err := NewAgentRunWorker(store, func(ctx context.Context, _ agentruntime.Fence) error {
		close(hung)
		<-ctx.Done()
		return ctx.Err()
	}, WorkerConfig{
		Enabled: true, Lease: 400 * time.Millisecond, Heartbeat: 150 * time.Millisecond,
		ScanInterval: 10 * time.Millisecond, MaxWorkers: 2,
	})
	require.NoError(t, err)
	require.NoError(t, worker.Tick(context.Background()))

	select {
	case <-hung:
	case <-time.After(10 * time.Second):
		t.Fatal("executor never started")
	}
	// The capped context ends the executor at the persisted deadline; the
	// short lease then expires and repeated scans reclaim the run, where the
	// past-deadline check fails it with the explicit reason.
	deadline := time.Now().Add(15 * time.Second)
	for {
		require.NoError(t, worker.Tick(context.Background()))
		run, getErr := store.Get(context.Background(), key)
		require.NoError(t, getErr)
		if run.Status == "failed" {
			require.Equal(t, "deadline_exceeded", run.WaitReason)
			return
		}
		if run.Status == "succeeded" {
			t.Fatal("deadline-capped run must not report success")
		}
		if time.Now().After(deadline) {
			t.Fatalf("deadline-capped run still status=%s", run.Status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
