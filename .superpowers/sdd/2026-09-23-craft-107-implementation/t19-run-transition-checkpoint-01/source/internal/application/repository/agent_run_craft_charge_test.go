package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func insertUnresolvedCraftStart(t *testing.T, db *gorm.DB, key agentruntime.RunKey, activity string) {
	t.Helper()
	now := time.Now().UTC()
	require.NoError(t, db.Exec("INSERT INTO craft_charge_start_journal (tenant_id, run_id, activity_key, grant_id, call_id, reservation_key, state, run_revision, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 'intent', 0, ?, ?)",
		key.TenantID, key.RunID, activity, "grant-"+activity, "call-"+activity, "reservation-"+activity, now, now).Error)
}

func TestCraftChargeIntentBlocksClaimAndDecisionResume(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	in := testAdmission()
	in.Key.RunID = "charge-claim"
	_, err := store.Admit(context.Background(), in)
	require.NoError(t, err)
	_, err = store.Claim(context.Background(), in.Key, "worker-old", time.Minute)
	require.NoError(t, err)
	claimed, err := store.Get(context.Background(), in.Key)
	require.NoError(t, err)
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).
		Updates(map[string]any{"status": "waiting_user", "wait_reason": "decision-1", "lease_owner": "", "lease_until": nil}).Error)
	insertUnresolvedCraftStart(t, db, in.Key, "claim-decision")

	_, err = store.ApplyDecision(context.Background(), in.Key, "u1", agentruntime.Decision{
		PendingID: "decision-1", DecisionID: "decision-resume", Action: "retry", ExpectedRevision: claimed.Revision,
	})
	require.ErrorIs(t, err, agentruntime.ErrConflict, "unresolved start cannot be turned into a resumed chargeable Run")
	var run agentRunRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).Take(&run).Error)
	require.Equal(t, "waiting_user", run.Status)
	require.Equal(t, "decision-1", run.WaitReason)

	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).
		Updates(map[string]any{"status": "recovering", "lease_owner": "worker-old", "lease_until": time.Now().Add(-time.Minute)}).Error)
	_, err = store.ClaimDriver(context.Background(), in.Key, "platform", "worker-new", time.Minute)
	require.ErrorIs(t, err, agentruntime.ErrLeaseLost)
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).Take(&run).Error)
	require.Equal(t, int64(1), run.Epoch)
	require.Equal(t, "worker-old", run.LeaseOwner)
}

func TestCraftChargeIntentIsNotScheduledFromQueuedRun(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	in := testAdmission()
	in.Key.RunID = "charge-queued-scan"
	_, err := store.Admit(context.Background(), in)
	require.NoError(t, err)
	insertUnresolvedCraftStart(t, db, in.Key, "queued-scan")

	keys, err := store.Scan(context.Background(), 10)
	require.NoError(t, err)
	require.NotContains(t, keys, in.Key, "a Run with a durable unresolved start must not be scheduled")
	_, err = store.Claim(context.Background(), in.Key, "worker", time.Minute)
	require.ErrorIs(t, err, agentruntime.ErrLeaseLost)
}

func TestCraftChargePendingPauseFinalizesAfterIntentResolves(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	in := testAdmission()
	in.Key.RunID = "charge-pause"
	_, err := store.Admit(context.Background(), in)
	require.NoError(t, err)
	fence, err := store.Claim(context.Background(), in.Key, "worker", time.Minute)
	require.NoError(t, err)
	insertUnresolvedCraftStart(t, db, in.Key, "pause")

	require.NoError(t, store.SetStatus(context.Background(), fence, "waiting_user", "budget_exhausted"))
	var run agentRunRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).Take(&run).Error)
	require.Equal(t, "reconciling", run.Status)
	require.Equal(t, craftChargeStartPendingWaitReason, run.WaitReason)
	var active *string
	require.NoError(t, db.Table("sessions").Select("active_agent_run_id").Where("tenant_id = ? AND id = ?", 1, in.SessionID).Scan(&active).Error)
	require.NotNil(t, active, "pending pause retains the active Run slot")

	require.NoError(t, db.Table("craft_charge_start_journal").Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).
		Update("state", "started").Error)
	keys, err := store.Scan(context.Background(), 10)
	require.NoError(t, err)
	require.Contains(t, keys, in.Key, "resolved pause requests are scheduled for durable finalization")
	_, err = store.Claim(context.Background(), in.Key, "worker-recovery", time.Minute)
	require.ErrorIs(t, err, agentruntime.ErrLeaseLost, "recovery finalizes the pause without claiming the parked Run")
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).Take(&run).Error)
	require.Equal(t, "waiting_user", run.Status)
	require.Equal(t, "budget_exhausted", run.WaitReason)
	require.NoError(t, db.Table("sessions").Select("active_agent_run_id").Where("tenant_id = ? AND id = ?", 1, in.SessionID).Scan(&active).Error)
	require.NotNil(t, active)
}

func TestCraftChargeFinalizeRetainsUnresolvedStartAndDefersCancel(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	in := testAdmission()
	in.Key.RunID = "charge-cancel"
	_, err := store.Admit(context.Background(), in)
	require.NoError(t, err)
	fence, err := store.Claim(context.Background(), in.Key, "worker", time.Minute)
	require.NoError(t, err)
	insertUnresolvedCraftStart(t, db, in.Key, "finalize")
	require.NoError(t, store.Finalize(context.Background(), fence, json.RawMessage("{\"content\":\"completed with retained start evidence\"}")))
	var journal struct{ State string }
	require.NoError(t, db.Table("craft_charge_start_journal").Select("state").Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).Take(&journal).Error)
	require.Equal(t, "intent", journal.State, "terminal finalization retains unresolved journal evidence")

	in.Key.RunID = "charge-cancel-pending"
	in.RequestID = "charge-cancel-pending-request"
	_, err = store.Admit(context.Background(), in)
	require.NoError(t, err)
	fence, err = store.Claim(context.Background(), in.Key, "worker-cancel", time.Minute)
	require.NoError(t, err)
	insertUnresolvedCraftStart(t, db, in.Key, "cancel")
	require.NoError(t, store.CancelRun(context.Background(), in.Key, "member_stop"))
	var run agentRunRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).Take(&run).Error)
	require.Equal(t, "reconciling", run.Status)
	require.Equal(t, craftChargeStartPendingWaitReason, run.WaitReason)
	var active *string
	require.NoError(t, db.Table("sessions").Select("active_agent_run_id").Where("tenant_id = ? AND id = ?", 1, in.SessionID).Scan(&active).Error)
	require.NotNil(t, active, "cancel request is not confirmed cancellation while start is unresolved")

	require.NoError(t, db.Table("craft_charge_start_journal").Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).
		Update("state", "definitely_unstarted").Error)
	keys, err := store.Scan(context.Background(), 10)
	require.NoError(t, err)
	require.Contains(t, keys, in.Key)
	_, err = store.Claim(context.Background(), in.Key, "worker-recovery", time.Minute)
	require.ErrorIs(t, err, agentruntime.ErrLeaseLost)
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).Take(&run).Error)
	require.Equal(t, "canceled", run.Status)
	require.Equal(t, "member_stop", run.WaitReason)
	require.NoError(t, db.Table("sessions").Select("active_agent_run_id").Where("tenant_id = ? AND id = ?", 1, in.SessionID).Scan(&active).Error)
	require.Nil(t, active)
}

func TestDeleteSessionRunsDoesNotCancelUnresolvedCraftRun(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	in := testAdmission()
	in.Key.RunID = "charge-delete"
	_, err := store.Admit(context.Background(), in)
	require.NoError(t, err)
	_, err = store.Claim(context.Background(), in.Key, "worker", time.Minute)
	require.NoError(t, err)
	insertUnresolvedCraftStart(t, db, in.Key, "delete")

	require.NoError(t, store.DeleteSessionRuns(context.Background(), 1, in.SessionID))
	var run agentRunRow
	var journal struct{ State string }
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).Take(&run).Error)
	require.Equal(t, "reconciling", run.Status)
	require.Equal(t, craftChargeStartPendingWaitReason, run.WaitReason)
	require.NoError(t, db.Table("craft_charge_start_journal").Select("state").Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).Take(&journal).Error)
	require.Equal(t, "intent", journal.State, "session deletion preserves unresolved journal evidence")
}
