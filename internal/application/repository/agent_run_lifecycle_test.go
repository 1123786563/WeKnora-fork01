package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
)

func TestCancelRunReleasesSlotAndDeleteFences(t *testing.T) {
	db := openRunTestDB(t)
	s := NewAgentRunStore(db)
	key := agentruntime.RunKey{TenantID: 1, RunID: "lifecycle-run"}
	_, err := s.Admit(context.Background(), agentruntime.Admission{Key: key, SessionID: "s1", UserID: "u1", RequestID: "lifecycle-q", AssistantMessageID: "lifecycle-a", RequestHash: "h", Snapshot: json.RawMessage(`{"version":1}`), UserMessage: json.RawMessage(`{"role":"user"}`), AssistantMessage: json.RawMessage(`{"role":"assistant"}`), Deadline: time.Now().Add(time.Hour)})
	require.NoError(t, err)
	f, err := s.Claim(context.Background(), key, "worker", time.Minute)
	require.NoError(t, err)
	require.NoError(t, s.CancelRun(context.Background(), key, "quote \"x\" \\ path"))
	var n int64
	require.NoError(t, db.Table("agent_run_events").Where("tenant_id=? AND run_id=? AND event_type=?", 1, key.RunID, "cancellation_requested").Count(&n).Error)
	require.Equal(t, int64(1), n)
	var payload string
	require.NoError(t, db.Table("agent_run_events").Select("payload").Where("tenant_id=? AND run_id=? AND event_type=?", 1, key.RunID, "cancellation_requested").Scan(&payload).Error)
	var decoded map[string]string
	require.NoError(t, json.Unmarshal([]byte(payload), &decoded))
	require.Equal(t, "quote \"x\" \\ path", decoded["reason"])
	run, err := s.Get(context.Background(), key)
	require.NoError(t, err)
	require.Equal(t, "canceled", run.Status)
	terminalRevision := run.Revision
	require.Error(t, s.SetStatus(context.Background(), f, "succeeded", "stale"))
	require.NoError(t, s.DeleteSessionRuns(context.Background(), 1, "s1"))
	var canceledEvents int64
	require.NoError(t, db.Table("agent_run_events").Where("tenant_id=? AND run_id=? AND event_type=?", 1, key.RunID, "cancellation_requested").Count(&canceledEvents).Error)
	require.Equal(t, int64(1), canceledEvents, "deleting a session must not duplicate a terminal cancellation event")
	run, err = s.Get(context.Background(), key)
	require.NoError(t, err)
	require.Equal(t, "canceled", run.Status)
	require.Equal(t, terminalRevision, run.Revision, "deleting the session must not rewrite an already terminal Run")
	require.Error(t, s.SetStatus(context.Background(), f, "succeeded", "stale after delete"), "session deletion keeps the stale-worker fence")
	var active *string
	require.NoError(t, db.Table("sessions").Select("active_agent_run_id").Where("tenant_id=? AND id=?", 1, "s1").Scan(&active).Error)
	require.Nil(t, active, "a terminal canceled Run has no active session slot")
	var state string
	var deletionRevision int64
	require.NoError(t, db.Table("execution_cleanup").Where("tenant_id=? AND session_id=?", 1, "s1").Select("state, deletion_revision").Row().Scan(&state, &deletionRevision))
	require.Equal(t, "tombstoned", state)
	require.Equal(t, terminalRevision+1, deletionRevision, "the tombstone revision follows the terminal Run revision")
}

func TestTerminalCancellationEnqueuesCaptureAndFencesNextRun(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	registerCraftSessionForCaptureTest(t, db)
	workspace := putCraftWorkspace(t, NewCraftStore(db))
	store := NewAgentRunStore(db)
	key := agentruntime.RunKey{TenantID: 1, RunID: "capture-terminal-run"}
	in := agentRunAdmissionForCaptureTest(key.RunID, workspace.ID)
	in.RequestID, in.AssistantMessageID, in.RequestHash = "capture-terminal-request", "capture-terminal-assistant", "capture-terminal-hash"
	_, err := store.Admit(ctx, in)
	require.NoError(t, err)
	views := NewCraftRunViewStore(db)
	view, err := views.Allocate(ctx, craft.RunViewKey{TenantID: 1, OwnerID: "u1", SessionID: "s1", RunID: key.RunID})
	require.NoError(t, err)
	view, _, err = views.BeginSessionCreate(ctx, view.Key, view.Generation)
	require.NoError(t, err)
	_, err = views.BindRuntime(ctx, view.Key, view.Generation, craft.RunViewRuntime{RuntimeID: "runtime-capture", ContainerID: "container-capture", OpenCodeSessionID: "oc-capture"})
	require.NoError(t, err)
	require.NoError(t, store.CancelRun(ctx, key, "stop"))
	var receipt CraftRunCaptureRowForTest
	require.NoError(t, db.Table("craft_run_captures").Where("tenant_id=? AND workspace_id=? AND run_id=?", 1, workspace.ID, key.RunID).Take(&receipt).Error)
	require.Equal(t, "pending", receipt.State, "terminal commit must atomically enqueue the durable capture receipt")
	require.Equal(t, view.Generation, receipt.Generation)
	next := agentRunAdmissionForCaptureTest("capture-next-run")
	_, err = store.Admit(ctx, next)
	require.Error(t, err, "a later Run cannot skip an unresolved draft predecessor")
}

type CraftRunCaptureRowForTest struct{ State, Generation string }

// TestTerminalRunWithoutBoundRunViewDoesNotFenceNextRun pins the R4 fence
// obligation semantics: only a terminal Run with a bound RunView owns a
// repairable draft capture. A terminal Run whose RunView never bound (no
// container output exists, the enqueue trigger cannot create a receipt) must
// not permanently block later Runs of the same Workspace — that is the #120
// "continue with other material" journey after a runtime-less or failed
// materialization.
func TestTerminalRunWithoutBoundRunViewDoesNotFenceNextRun(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	registerCraftSessionForCaptureTest(t, db)
	workspace := putCraftWorkspace(t, NewCraftStore(db))
	store := NewAgentRunStore(db)
	key := agentruntime.RunKey{TenantID: 1, RunID: "capture-unbound-run"}
	in := agentRunAdmissionForCaptureTest(key.RunID, workspace.ID)
	in.RequestID, in.AssistantMessageID, in.RequestHash = "capture-unbound-request", "capture-unbound-assistant", "capture-unbound-hash"
	_, err := store.Admit(ctx, in)
	require.NoError(t, err)
	// The Run terminates without any RunView allocation or binding.
	require.NoError(t, store.CancelRun(ctx, key, "stop"))
	var receipts int64
	require.NoError(t, db.Table("craft_run_captures").Where("tenant_id=? AND run_id=?", 1, key.RunID).Count(&receipts).Error)
	require.Zero(t, receipts, "a Run without a bound RunView enqueues no capture receipt")
	next := agentRunAdmissionForCaptureTest("capture-next-after-unbound")
	_, err = store.Admit(ctx, next)
	require.NoError(t, err, "a terminal Run without a bound RunView carries no capture obligation")
}

func agentRunAdmissionForCaptureTest(runID string, workspaceIDs ...string) agentruntime.Admission {
	_ = workspaceIDs // the repository, not the fixture caller, derives the Workspace seed
	snapshot, err := json.Marshal(map[string]any{"version": 1, "craft_input_manifest": []craft.Input{}})
	if err != nil {
		panic(err)
	}
	return agentruntime.Admission{Key: agentruntime.RunKey{TenantID: 1, RunID: runID}, SessionID: "s1", UserID: "u1", ActorUserID: "u1", RequestID: runID + "-request", AssistantMessageID: runID + "-assistant", RequestHash: runID + "-hash", Snapshot: snapshot, UserMessage: json.RawMessage(`{"role":"user"}`), AssistantMessage: json.RawMessage(`{"role":"assistant"}`), Deadline: time.Now().Add(time.Hour)}
}

func TestDeleteSessionRunAfterPendingPausePersistsCancelUntilChargeStartResolves(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	in := testAdmission()
	in.Key.RunID = "lifecycle-delete-pending-charge"
	_, err := store.Admit(context.Background(), in)
	require.NoError(t, err)
	fence, err := store.Claim(context.Background(), in.Key, "worker-delete-pending", time.Minute)
	require.NoError(t, err)
	insertUnresolvedCraftStart(t, db, in.Key, "delete-pending-pause")
	require.NoError(t, store.SetStatus(context.Background(), fence, "waiting_user", "budget_exhausted"))

	var run agentRunRow
	require.NoError(t, db.Where("tenant_id=? AND run_id=?", in.Key.TenantID, in.Key.RunID).Take(&run).Error)
	require.Equal(t, "reconciling", run.Status)
	require.Equal(t, craftChargeStartPendingWaitReason, run.WaitReason)
	pauseRevision := run.Revision
	var active *string
	require.NoError(t, db.Table("sessions").Select("active_agent_run_id").Where("tenant_id=? AND id=?", in.Key.TenantID, in.SessionID).Scan(&active).Error)
	require.NotNil(t, active, "an unresolved start keeps the Run slot held during pause")

	require.NoError(t, store.DeleteSessionRuns(context.Background(), in.Key.TenantID, in.SessionID))
	require.NoError(t, db.Where("tenant_id=? AND run_id=?", in.Key.TenantID, in.Key.RunID).Take(&run).Error)
	require.Equal(t, pauseRevision+1, run.Revision, "deletion records one durable cancellation request")
	require.Equal(t, "reconciling", run.Status)
	var pauseRequests, cancelRequests, cancellations int64
	require.NoError(t, db.Table("agent_run_events").Where("tenant_id=? AND run_id=? AND event_type=?", in.Key.TenantID, in.Key.RunID, "craft_charge_pause_requested").Count(&pauseRequests).Error)
	require.NoError(t, db.Table("agent_run_events").Where("tenant_id=? AND run_id=? AND event_type=?", in.Key.TenantID, in.Key.RunID, "craft_charge_cancel_requested").Count(&cancelRequests).Error)
	require.NoError(t, db.Table("agent_run_events").Where("tenant_id=? AND run_id=? AND event_type=?", in.Key.TenantID, in.Key.RunID, "cancellation_requested").Count(&cancellations).Error)
	require.EqualValues(t, 1, pauseRequests)
	require.EqualValues(t, 1, cancelRequests)
	require.EqualValues(t, 1, cancellations)
	require.NoError(t, db.Table("sessions").Select("active_agent_run_id").Where("tenant_id=? AND id=?", in.Key.TenantID, in.SessionID).Scan(&active).Error)
	require.NotNil(t, active)
	var journal struct{ State string }
	require.NoError(t, db.Table("craft_charge_start_journal").Select("state").Where("tenant_id=? AND run_id=?", in.Key.TenantID, in.Key.RunID).Take(&journal).Error)
	require.Equal(t, "intent", journal.State, "deletion retains unresolved charge-start evidence")

	require.NoError(t, db.Table("craft_charge_start_journal").Where("tenant_id=? AND run_id=?", in.Key.TenantID, in.Key.RunID).Update("state", "started").Error)
	_, err = store.Claim(context.Background(), in.Key, "worker-delete-recovery", time.Minute)
	require.ErrorIs(t, err, agentruntime.ErrLeaseLost, "recovery finalizes the pending cancellation without claiming the Run")
	require.NoError(t, db.Where("tenant_id=? AND run_id=?", in.Key.TenantID, in.Key.RunID).Take(&run).Error)
	require.Equal(t, "canceled", run.Status, "durable deletion cancellation takes precedence over the earlier pause")
	require.Equal(t, "session_deleted", run.WaitReason)
	require.Empty(t, run.LeaseOwner)
	require.NoError(t, db.Table("sessions").Select("active_agent_run_id").Where("tenant_id=? AND id=?", in.Key.TenantID, in.SessionID).Scan(&active).Error)
	require.Nil(t, active, "the slot is released after the unresolved start is resolved")
	require.NoError(t, db.Table("craft_charge_start_journal").Select("state").Where("tenant_id=? AND run_id=?", in.Key.TenantID, in.Key.RunID).Take(&journal).Error)
	require.Equal(t, "started", journal.State, "final cancellation retains resolved charge-start evidence")
	var tombstone struct {
		State            string
		DeletionRevision int64
	}
	require.NoError(t, db.Table("execution_cleanup").Select("state, deletion_revision").Where("tenant_id=? AND session_id=?", in.Key.TenantID, in.SessionID).Take(&tombstone).Error)
	require.Equal(t, "tombstoned", tombstone.State)
}
