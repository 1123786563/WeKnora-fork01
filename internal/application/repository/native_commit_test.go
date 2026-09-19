package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	"trpc.group/trpc-go/trpc-agent-go/session"
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

func nativeCommitSessionHash(t *testing.T, e *event.Event) string {
	t.Helper()
	payload, err := json.Marshal(struct {
		Version int          `json:"version"`
		Event   *event.Event `json:"event"`
	}{Version: 1, Event: e})
	require.NoError(t, err)
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:])
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

func TestNativeCommitStaleFenceWritesNoDurableRecords(t *testing.T) {
	coordinator, fence := nativeCommitFixture(t)
	stale := fence
	stale.Epoch--
	intent := nativeCommitIntent(stale, "intent-stale", "hash-stale")
	intent.Events = []nativecontract.BusinessEvent{nativeBusinessEvent("event-stale", "stale")}

	_, err := coordinator.Commit(context.Background(), intent)
	require.Equal(t, nativecontract.ErrLeaseLost, failureCode(t, err))
	for _, table := range []string{
		"native_agent_commit_intents",
		"native_agent_session_events",
		"native_agent_tool_results",
		"native_agent_checkpoints",
		"native_agent_events",
	} {
		var count int64
		require.NoError(t, coordinator.db.Table(table).Count(&count).Error)
		require.Zero(t, count, "stale fence wrote %s", table)
	}
}

func TestNativeCommitPersistsPendingIntentAcrossDownstreamBarrierFailures(t *testing.T) {
	for _, gap := range []struct {
		name, table string
		configure   func(*NativeCommitCoordinator, nativecontract.Fence) nativecontract.CommitIntent
	}{
		{
			name: "session append", table: "native_agent_session_events",
			configure: func(_ *NativeCommitCoordinator, fence nativecontract.Fence) nativecontract.CommitIntent {
				intent := nativeCommitIntent(fence, "intent-session", "hash-session")
				e := &event.Event{ID: "session-event", Author: "agent"}
				intent.SessionAppends = []nativecontract.SessionAppend{{
					Key:           session.Key{AppName: "weknora/native-v1/tenant/1", UserID: "owner/dTE", SessionID: "session/czE"},
					StableEventID: e.ID, PayloadHash: nativeCommitSessionHash(t, e), Event: e,
				}}
				return intent
			},
		},
		{
			name: "tool result", table: "native_agent_tool_results",
			configure: func(coordinator *NativeCommitCoordinator, fence nativecontract.Fence) nativecontract.CommitIntent {
				intent := nativeCommitIntent(fence, "intent-result", "hash-result")
				require.NoError(t, coordinator.db.Exec(`INSERT INTO native_agent_attempts (tenant_id, run_id, attempt_id, lease_epoch) VALUES (?, ?, ?, ?)`, 1, "run-1", "attempt-result", 7).Error)
				require.NoError(t, coordinator.db.Exec(`INSERT INTO native_agent_tool_calls (tenant_id, run_id, attempt_id, call_id, plan_version, args_hash, lease_epoch) VALUES (?, ?, ?, ?, ?, ?, ?)`, 1, "run-1", "attempt-result", "call-result", 1, "args", 7).Error)
				intent.Results = []nativecontract.ToolOutcome{{AttemptID: "attempt-result", CallID: "call-result", ResultHash: "result", Effect: nativecontract.EffectConfirmed, Content: []byte(`{}`)}}
				return intent
			},
		},
		{
			name: "checkpoint", table: "native_agent_checkpoints",
			configure: func(_ *NativeCommitCoordinator, fence nativecontract.Fence) nativecontract.CommitIntent {
				intent := nativeCommitIntent(fence, "intent-checkpoint", "hash-checkpoint")
				intent.Checkpoint = &nativecontract.CheckpointWrite{SchemaVersion: 1, SDKVersion: "sdk", GraphVersion: "graph", Namespace: "native/run", LineageID: "lineage", Request: graph.PutFullRequest{Checkpoint: &graph.Checkpoint{ID: "checkpoint-gap"}}}
				return intent
			},
		},
		{
			name: "event", table: "native_agent_events",
			configure: func(_ *NativeCommitCoordinator, fence nativecontract.Fence) nativecontract.CommitIntent {
				intent := nativeCommitIntent(fence, "intent-event", "hash-event")
				intent.Events = []nativecontract.BusinessEvent{nativeBusinessEvent("event-gap", "gap")}
				return intent
			},
		},
	} {
		t.Run(gap.name, func(t *testing.T) {
			coordinator, fence := nativeCommitFixture(t)
			intent := gap.configure(coordinator, fence)
			trigger := "abort_" + strings.ReplaceAll(intent.ID, "-", "_")
			require.NoError(t, coordinator.db.Exec("CREATE TRIGGER "+trigger+" BEFORE INSERT ON "+gap.table+" BEGIN SELECT RAISE(ABORT, 'injected "+gap.name+" failure'); END").Error)

			_, err := coordinator.Commit(context.Background(), intent)
			require.Error(t, err)
			var state string
			require.NoError(t, coordinator.db.Table("native_agent_commit_intents").Select("state").Where("tenant_id=? AND run_id=? AND intent_id=?", 1, "run-1", intent.ID).Row().Scan(&state))
			require.Equal(t, "pending", state)

			require.NoError(t, coordinator.db.Exec("DROP TRIGGER "+trigger).Error)
			require.NoError(t, coordinator.db.Exec("UPDATE native_agent_runs SET lease_owner=?, lease_epoch=?, lease_expires_at=? WHERE tenant_id=? AND run_id=?", "worker-2", 8, time.Now().Add(time.Hour), 1, "run-1").Error)
			freshFence := fence
			freshFence.Owner, freshFence.Epoch, freshFence.LeaseUntil = "worker-2", 8, time.Now().Add(time.Hour)
			receipt, err := NewNativeCommitCoordinator(reopenRunDB(t, coordinator.db)).Reconcile(context.Background(), freshFence, intent.ID)
			require.NoError(t, err)
			require.True(t, receipt.Applied)
		})
	}
}

func TestNativeCommitSessionAppendCanonicalizesHashAndRequiresEventID(t *testing.T) {
	coordinator, fence := nativeCommitFixture(t)
	e := &event.Event{ID: "event-1", Author: "agent"}
	valid := nativeCommitIntent(fence, "intent-valid", "hash-valid")
	valid.SessionAppends = []nativecontract.SessionAppend{{
		Key:           session.Key{AppName: "weknora/native-v1/tenant/1", UserID: "owner/dTE", SessionID: "session/czE"},
		StableEventID: e.ID, PayloadHash: "caller-supplied-hash", Event: e,
	}}
	_, err := coordinator.Commit(context.Background(), valid)
	require.NoError(t, err)
	var stored string
	require.NoError(t, coordinator.db.Table("native_agent_session_events").Select("payload_hash").Where("tenant_id=? AND stable_event_id=?", 1, e.ID).Row().Scan(&stored))
	require.Equal(t, nativeCommitSessionHash(t, e), stored)

	wrongID := valid
	wrongID.ID, wrongID.PayloadHash = "intent-wrong-id", "hash-wrong-id"
	wrongID.SessionAppends = append([]nativecontract.SessionAppend(nil), valid.SessionAppends...)
	wrongID.SessionAppends[0].StableEventID = "another-id"
	_, err = coordinator.Commit(context.Background(), wrongID)
	require.Equal(t, nativecontract.ErrInvalid, failureCode(t, err))
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
