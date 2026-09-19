package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-agent-go/graph"
)

func nativeCommitFixture(t *testing.T) (*NativeCommitCoordinator, nativecontract.Fence) {
	t.Helper()
	db := openRunTestDB(t)
	require.NoError(t, db.Exec("INSERT INTO native_agent_tenants (tenant_id) VALUES (?)", 1).Error)
	require.NoError(t, db.Exec("INSERT INTO native_agent_sessions (tenant_id, owner_id, session_id) VALUES (?, ?, ?)", 1, "owner/dTE", "session/czE").Error)
	require.NoError(t, db.Exec(`INSERT INTO native_agent_runs
		(tenant_id, run_id, owner_id, session_id, status, lease_owner, lease_epoch, lease_expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, 1, "run-1", "owner/dTE", "session/czE", "running", "worker-1", 7, time.Now().Add(time.Hour)).Error)
	return NewNativeCommitCoordinator(db), nativecontract.Fence{
		Run:   nativecontract.RunIdentity{TenantID: 1, SessionID: "session/czE", RunID: "run-1"},
		Owner: "worker-1", Epoch: 7, LeaseUntil: time.Now().Add(time.Hour),
	}
}

func nativeCommitIntent(fence nativecontract.Fence, id, hash string) nativecontract.CommitIntent {
	return nativecontract.CommitIntent{Version: 1, ID: id, PayloadHash: hash, Fence: fence}
}

func failureCode(t *testing.T, err error) nativecontract.ErrorCode {
	t.Helper()
	var failure *nativecontract.Failure
	require.True(t, errors.As(err, &failure), "expected typed native failure, got %v", err)
	return failure.Code
}

func TestNativeCommitSameIntentIsIdempotentAndChangedHashConflicts(t *testing.T) {
	coordinator, fence := nativeCommitFixture(t)
	ctx := context.Background()

	first, err := coordinator.Commit(ctx, nativeCommitIntent(fence, "intent-1", "hash-1"))
	require.NoError(t, err)
	require.True(t, first.Applied)
	second, err := coordinator.Commit(ctx, nativeCommitIntent(fence, "intent-1", "hash-1"))
	require.NoError(t, err)
	require.Equal(t, first, second)

	_, err = coordinator.Commit(ctx, nativeCommitIntent(fence, "intent-1", "changed"))
	require.Equal(t, nativecontract.ErrConflict, failureCode(t, err))
}

func TestNativeBarrierRequiresAppliedIntent(t *testing.T) {
	coordinator, fence := nativeCommitFixture(t)
	ctx := context.Background()

	require.Equal(t, nativecontract.ErrConflict, failureCode(t, coordinator.Barrier(ctx, fence, "missing")))
	_, err := coordinator.Commit(ctx, nativeCommitIntent(fence, "intent-1", "hash-1"))
	require.NoError(t, err)
	require.NoError(t, coordinator.Barrier(ctx, fence, "intent-1"))
}

func TestNativeCommitReconcileRejectsStaleFence(t *testing.T) {
	coordinator, fence := nativeCommitFixture(t)
	ctx := context.Background()
	_, err := coordinator.Commit(ctx, nativeCommitIntent(fence, "intent-1", "hash-1"))
	require.NoError(t, err)

	stale := fence
	stale.Epoch--
	_, err = coordinator.Reconcile(ctx, stale, "intent-1")
	require.Equal(t, nativecontract.ErrLeaseLost, failureCode(t, err))
}

func TestNativeCommitCheckpointRequiresAndRecordsDurableToolReceipt(t *testing.T) {
	coordinator, fence := nativeCommitFixture(t)
	ctx := context.Background()
	checkpoint := &nativecontract.CheckpointWrite{
		SchemaVersion: 1, SDKVersion: "sdk-1", GraphVersion: "graph-1", Namespace: "native/run", LineageID: "lineage-1",
		Request: graph.PutFullRequest{Checkpoint: &graph.Checkpoint{ID: "checkpoint-1"}}, ResultCallIDs: []string{"call-1"},
	}
	missing := nativeCommitIntent(fence, "intent-missing", "hash-missing")
	missing.Checkpoint = checkpoint
	_, err := coordinator.Commit(ctx, missing)
	require.Equal(t, nativecontract.ErrCheckpoint, failureCode(t, err))

	// Tool-call identity is owned by the already durable P1.4 journal. Commit
	// writes its actual result into that identity before it makes the checkpoint runnable.
	require.NoError(t, coordinator.db.Exec(`INSERT INTO native_agent_attempts
		(tenant_id, run_id, attempt_id, lease_epoch) VALUES (?, ?, ?, ?)`, 1, "run-1", "attempt-1", 7).Error)
	require.NoError(t, coordinator.db.Exec(`INSERT INTO native_agent_tool_calls
		(tenant_id, run_id, attempt_id, call_id, plan_version, args_hash, lease_epoch)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, 1, "run-1", "attempt-1", "call-1", 1, "args-1", 7).Error)
	applied := nativeCommitIntent(fence, "intent-applied", "hash-applied")
	applied.Checkpoint = checkpoint
	applied.Results = []nativecontract.ToolOutcome{{AttemptID: "attempt-1", CallID: "call-1", ResultHash: "result-1", Effect: nativecontract.EffectConfirmed, Content: []byte(`{}`)}}
	_, err = coordinator.Commit(ctx, applied)
	require.NoError(t, err)
	var resultCalls string
	require.NoError(t, coordinator.db.Table("native_agent_checkpoints").Select("result_call_ids").Where("tenant_id=? AND run_id=? AND checkpoint_id=?", 1, "run-1", "checkpoint-1").Row().Scan(&resultCalls))
	require.JSONEq(t, `["call-1"]`, resultCalls)
}
