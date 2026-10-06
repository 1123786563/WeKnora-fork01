package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
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
	require.Equal(t, "worker", run.LeaseOwner, "an unresolved start retains its execution lease")
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
	require.Empty(t, run.LeaseOwner, "the lease is released only after the start outcome resolves")
	require.NoError(t, db.Table("sessions").Select("active_agent_run_id").Where("tenant_id = ? AND id = ?", 1, in.SessionID).Scan(&active).Error)
	require.NotNil(t, active)
}

func TestCraftChargePendingPauseThenCancelWinsAndRepeatsIdempotently(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	in := testAdmission()
	in.Key.RunID = "charge-pause-then-cancel"
	_, err := store.Admit(context.Background(), in)
	require.NoError(t, err)
	fence, err := store.Claim(context.Background(), in.Key, "worker-pause-cancel", time.Minute)
	require.NoError(t, err)
	insertUnresolvedCraftStart(t, db, in.Key, "pause-then-cancel")
	require.NoError(t, store.SetStatus(context.Background(), fence, "waiting_user", "budget_exhausted"))

	var run agentRunRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).Take(&run).Error)
	pausedRevision := run.Revision
	require.Equal(t, "reconciling", run.Status)
	require.ErrorIs(t, store.CancelRunOwnedAtRevision(context.Background(), 1, "other-user", in.Key.RunID, pausedRevision, "member_stop"), agentruntime.ErrConflict)
	require.ErrorIs(t, store.CancelRunOwnedAtRevision(context.Background(), 1, "u1", in.Key.RunID, pausedRevision-1, "member_stop"), agentruntime.ErrConflict)
	require.NoError(t, store.CancelRunOwnedAtRevision(context.Background(), 1, "u1", in.Key.RunID, pausedRevision, "member_stop"))
	require.NoError(t, store.CancelRun(context.Background(), in.Key, "member_stop"), "repeated cancel is idempotent")
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).Take(&run).Error)
	require.Equal(t, pausedRevision+1, run.Revision, "duplicate cancel must not advance revision again")
	var cancelRequests int64
	require.NoError(t, db.Table("agent_run_events").Where("tenant_id = ? AND run_id = ? AND event_type = ?", 1, in.Key.RunID, "craft_charge_cancel_requested").Count(&cancelRequests).Error)
	require.EqualValues(t, 1, cancelRequests)

	require.NoError(t, db.Table("craft_charge_start_journal").Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).
		Update("state", "started").Error)
	keys, err := store.Scan(context.Background(), 10)
	require.NoError(t, err)
	require.Contains(t, keys, in.Key)
	_, err = store.Claim(context.Background(), in.Key, "worker-recovery", time.Minute)
	require.ErrorIs(t, err, agentruntime.ErrLeaseLost)
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).Take(&run).Error)
	require.Equal(t, "canceled", run.Status, "later cancel must win over an earlier pending pause")
	require.Equal(t, "member_stop", run.WaitReason)
	var active *string
	require.NoError(t, db.Table("sessions").Select("active_agent_run_id").Where("tenant_id = ? AND id = ?", 1, in.SessionID).Scan(&active).Error)
	require.Nil(t, active)
	var journal struct{ State string }
	require.NoError(t, db.Table("craft_charge_start_journal").Select("state").Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).Take(&journal).Error)
	require.Equal(t, "started", journal.State, "final cancellation retains resolved charge-start evidence")
}

func TestCraftChargeCancelCannotBeDowngradedByStalePause(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	in := testAdmission()
	in.Key.RunID = "charge-cancel-then-pause"
	_, err := store.Admit(context.Background(), in)
	require.NoError(t, err)
	fence, err := store.Claim(context.Background(), in.Key, "worker-cancel-pause", time.Minute)
	require.NoError(t, err)
	insertUnresolvedCraftStart(t, db, in.Key, "cancel-then-pause")
	require.NoError(t, store.CancelRun(context.Background(), in.Key, "member_stop"))
	require.ErrorIs(t, store.SetStatus(context.Background(), fence, "waiting_user", "late_budget_pause"), agentruntime.ErrLeaseLost)
	var run agentRunRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).Take(&run).Error)
	require.Equal(t, "reconciling", run.Status)
	require.Equal(t, craftChargeStartPendingWaitReason, run.WaitReason)
	var lastSeq int64
	require.NoError(t, db.Table("agent_run_events").Select("COALESCE(MAX(seq), 0)").Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).Scan(&lastSeq).Error)
	require.NoError(t, db.Create(&agentRunEventRow{
		TenantID: 1, RunID: in.Key.RunID, Seq: lastSeq + 1,
		EventType: "craft_charge_pause_requested", Payload: `{"reason":"late_budget_pause"}`,
	}).Error, "even a later stale pause event cannot downgrade a recorded cancellation")
	require.NoError(t, db.Table("craft_charge_start_journal").Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).
		Update("state", "definitely_unstarted").Error)
	_, err = store.Claim(context.Background(), in.Key, "worker-recovery", time.Minute)
	require.ErrorIs(t, err, agentruntime.ErrLeaseLost)
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).Take(&run).Error)
	require.Equal(t, "canceled", run.Status)
	require.Equal(t, "member_stop", run.WaitReason)
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

func TestPauseForCraftBudgetInTxDefersIntentAndUnknownStarts(t *testing.T) {
	for _, state := range []string{"intent", "unknown"} {
		t.Run(state, func(t *testing.T) {
			db := openRunTestDB(t)
			store := NewAgentRunStore(db)
			in := testAdmission()
			in.Key.RunID = "budget-pause-" + state
			_, err := store.Admit(context.Background(), in)
			require.NoError(t, err)
			_, err = store.Claim(context.Background(), in.Key, "budget-worker", time.Minute)
			require.NoError(t, err)
			insertUnresolvedCraftStart(t, db, in.Key, "budget-"+state)
			if state == "unknown" {
				require.NoError(t, db.Table("craft_charge_start_journal").Where("tenant_id = ? AND run_id = ?", in.Key.TenantID, in.Key.RunID).Update("state", state).Error)
			}

			err = db.WithContext(context.Background()).Transaction(func(tx *gorm.DB) error {
				return store.PauseForCraftBudgetInTx(context.Background(), tx, in.Key, "budget_exhausted")
			})
			require.NoError(t, err)
			var run agentRunRow
			require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", in.Key.TenantID, in.Key.RunID).Take(&run).Error)
			require.Equal(t, "reconciling", run.Status)
			require.Equal(t, craftChargeStartPendingWaitReason, run.WaitReason)
			require.Equal(t, "budget-worker", run.LeaseOwner, "the worker lease remains held until the start outcome resolves")
			var eventCount int64
			require.NoError(t, db.Table("agent_run_events").Where("tenant_id = ? AND run_id = ? AND event_type = ?", in.Key.TenantID, in.Key.RunID, "craft_charge_pause_requested").Count(&eventCount).Error)
			require.EqualValues(t, 1, eventCount)
			keys, err := store.Scan(context.Background(), 20)
			require.NoError(t, err)
			require.NotContains(t, keys, in.Key, "unresolved charge holds are not schedulable")
			_, err = store.Claim(context.Background(), in.Key, "budget-recovery", time.Minute)
			require.ErrorIs(t, err, agentruntime.ErrLeaseLost)
		})
	}
}

func TestPauseForCraftBudgetInTxIsIdempotentAndCancelWins(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	in := testAdmission()
	in.Key.RunID = "budget-pause-idempotent"
	_, err := store.Admit(context.Background(), in)
	require.NoError(t, err)
	_, err = store.Claim(context.Background(), in.Key, "budget-worker", time.Minute)
	require.NoError(t, err)
	insertUnresolvedCraftStart(t, db, in.Key, "budget-idempotent")

	pause := func(reason string) error {
		return db.WithContext(context.Background()).Transaction(func(tx *gorm.DB) error {
			return store.PauseForCraftBudgetInTx(context.Background(), tx, in.Key, reason)
		})
	}
	require.NoError(t, pause("budget_exhausted"))
	var before agentRunRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", in.Key.TenantID, in.Key.RunID).Take(&before).Error)
	require.NoError(t, pause("budget_exhausted"))
	var after agentRunRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", in.Key.TenantID, in.Key.RunID).Take(&after).Error)
	require.Equal(t, before.Revision, after.Revision, "replaying a budget pause must not advance Run revision")
	var eventCount int64
	require.NoError(t, db.Table("agent_run_events").Where("tenant_id = ? AND run_id = ? AND event_type = ?", in.Key.TenantID, in.Key.RunID, "craft_charge_pause_requested").Count(&eventCount).Error)
	require.EqualValues(t, 1, eventCount)

	require.NoError(t, store.CancelRun(context.Background(), in.Key, "member_stop"))
	err = pause("budget_exhausted")
	require.ErrorIs(t, err, agentruntime.ErrConflict, "budget pause cannot downgrade a pending cancellation")
	var cancelCount int64
	require.NoError(t, db.Table("agent_run_events").Where("tenant_id = ? AND run_id = ? AND event_type = ?", in.Key.TenantID, in.Key.RunID, "craft_charge_cancel_requested").Count(&cancelCount).Error)
	require.EqualValues(t, 1, cancelCount)
}

func TestPauseForCraftBudgetInTxTransitionsResolvedOrMissingJournal(t *testing.T) {
	for _, journalState := range []string{"absent", "started", "definitely_unstarted"} {
		t.Run(journalState, func(t *testing.T) {
			db := openRunTestDB(t)
			store := NewAgentRunStore(db)
			in := testAdmission()
			in.Key.RunID = "budget-resolved-" + journalState
			_, err := store.Admit(context.Background(), in)
			require.NoError(t, err)
			_, err = store.Claim(context.Background(), in.Key, "budget-worker", time.Minute)
			require.NoError(t, err)
			if journalState != "absent" {
				insertUnresolvedCraftStart(t, db, in.Key, "resolved-"+journalState)
				require.NoError(t, db.Table("craft_charge_start_journal").Where("tenant_id = ? AND run_id = ?", in.Key.TenantID, in.Key.RunID).Update("state", journalState).Error)
			}
			err = db.WithContext(context.Background()).Transaction(func(tx *gorm.DB) error {
				return store.PauseForCraftBudgetInTx(context.Background(), tx, in.Key, "budget_exhausted")
			})
			require.NoError(t, err)
			var run agentRunRow
			require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", in.Key.TenantID, in.Key.RunID).Take(&run).Error)
			require.Equal(t, "waiting_user", run.Status)
			require.Equal(t, "budget_exhausted", run.WaitReason)
			require.Empty(t, run.LeaseOwner)
		})
	}
}

func TestPauseForCraftBudgetInTxRollsBackWhenPauseEventCannotBeWritten(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	in := testAdmission()
	in.Key.RunID = "budget-pause-event-failure"
	_, err := store.Admit(context.Background(), in)
	require.NoError(t, err)
	_, err = store.Claim(context.Background(), in.Key, "budget-worker", time.Minute)
	require.NoError(t, err)
	insertUnresolvedCraftStart(t, db, in.Key, "budget-event-failure")
	require.NoError(t, db.Exec(`CREATE TRIGGER reject_budget_pause BEFORE INSERT ON agent_run_events
		WHEN NEW.event_type = 'craft_charge_pause_requested'
		BEGIN SELECT RAISE(ABORT, 'reject budget pause event'); END`).Error)

	err = db.WithContext(context.Background()).Transaction(func(tx *gorm.DB) error {
		return store.PauseForCraftBudgetInTx(context.Background(), tx, in.Key, "budget_exhausted")
	})
	require.Error(t, err)
	var run agentRunRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", in.Key.TenantID, in.Key.RunID).Take(&run).Error)
	require.Equal(t, "running", run.Status)
	require.Equal(t, "budget-worker", run.LeaseOwner)
	require.EqualValues(t, 1, run.Revision)
}
