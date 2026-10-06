package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	runrepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCraftChargeStartTwoPhaseLeavesIntentAndHoldUntilResolution(t *testing.T) {
	db := openCraftBudgetTestDB(t)
	svc, err := NewCraftBudgetService(db, nil, craftBudgetPolicy())
	require.NoError(t, err)
	seedCraftFundedTenant(t, db, 111, 10000)
	seedCraftBudgetRun(t, db, 111, "run-two-phase", "task-two-phase")
	grant, err := svc.Admit(context.Background(), craft.Scope{TenantID: 111, UserID: "owner", SessionID: "task-two-phase"}, "run-two-phase")
	require.NoError(t, err)

	attempt, err := svc.BeginBinding(context.Background(), grant.ID, "activity-two-phase", CraftCallBinding{ModelID: "lead", Funding: commercial.FundingPlatform})
	require.NoError(t, err)
	defer attempt.CancelInitiation()
	deadline, ok := attempt.InitiationContext().Deadline()
	require.True(t, ok, "attempt handle exposes the bounded initiation context")
	require.WithinDuration(t, time.Now().Add(craftChargeStartTimeout), deadline, 2*time.Second)

	var journal CraftChargeStartJournalRow
	require.NoError(t, db.Where("tenant_id=? AND run_id=? AND activity_key=?", 111, "run-two-phase", "activity-two-phase").Take(&journal).Error)
	require.Equal(t, "intent", journal.State, "headers alone cannot resolve the durable attempt")
	var reservation repocommercial.ReservationRow
	require.NoError(t, db.Where("tenant_id=? AND key=?", 111, CraftCallKey("activity/activity-two-phase")).Take(&reservation).Error)
	require.Equal(t, commercial.ReservationStateDispatched, reservation.State, "the G4 hold is already durable")

	store := runrepo.NewAgentRunStore(db)
	require.NoError(t, store.CancelRun(context.Background(), agentruntime.RunKey{TenantID: 111, RunID: "run-two-phase"}, "user_stop"))
	var run struct {
		Status     string
		WaitReason string `gorm:"column:wait_reason"`
	}
	require.NoError(t, db.Table("agent_runs").Select("status, wait_reason").Where("tenant_id=? AND run_id=?", 111, "run-two-phase").Take(&run).Error)
	require.Equal(t, "reconciling", run.Status, "Run cancellation cannot release the slot before body resolution")
	require.Equal(t, "craft_charge_start_pending", run.WaitReason)

	require.NoError(t, attempt.Resolve(context.Background(), CraftChargeStartStarted), "complete body observation resolves intent exactly once")
	require.Error(t, attempt.Resolve(context.Background(), CraftChargeStartUnknown), "a resolved started attempt cannot be downgraded")
	require.NoError(t, db.Where("tenant_id=? AND run_id=? AND activity_key=?", 111, "run-two-phase", "activity-two-phase").Take(&journal).Error)
	require.Equal(t, "started", journal.State)
	require.Len(t, craftReservations(t, db, 111), 1, "the dispatch hold remains for normal usage settlement")

	_, err = store.Claim(context.Background(), agentruntime.RunKey{TenantID: 111, RunID: "run-two-phase"}, "worker-recovery", time.Minute)
	require.ErrorIs(t, err, agentruntime.ErrLeaseLost, "recovery finalizes the pending cancellation instead of claiming the Run")
	require.NoError(t, db.Table("agent_runs").Select("status, wait_reason").Where("tenant_id=? AND run_id=?", 111, "run-two-phase").Take(&run).Error)
	require.Equal(t, "canceled", run.Status)
	require.Equal(t, "user_stop", run.WaitReason)
}

func TestCraftChargeStartTwoPhaseUnknownWriteFailureKeepsIntentHeldAndNonReplayable(t *testing.T) {
	db := openCraftBudgetTestDB(t)
	svc, err := NewCraftBudgetService(db, nil, craftBudgetPolicy())
	require.NoError(t, err)
	seedCraftFundedTenant(t, db, 112, 10000)
	seedCraftBudgetRun(t, db, 112, "run-two-phase-unknown", "task-two-phase-unknown")
	grant, err := svc.Admit(context.Background(), craft.Scope{TenantID: 112, UserID: "owner", SessionID: "task-two-phase-unknown"}, "run-two-phase-unknown")
	require.NoError(t, err)
	attempt, err := svc.BeginBinding(context.Background(), grant.ID, "activity-two-phase-unknown", CraftCallBinding{ModelID: "lead", Funding: commercial.FundingPlatform})
	require.NoError(t, err)
	defer attempt.CancelInitiation()

	require.NoError(t, db.Exec("CREATE TRIGGER fail_two_phase_resolution BEFORE UPDATE OF state ON craft_charge_start_journal BEGIN SELECT RAISE(ABORT, 'injected resolution failure'); END").Error)
	require.Error(t, attempt.Resolve(context.Background(), CraftChargeStartUnknown))
	var journal CraftChargeStartJournalRow
	require.NoError(t, db.Where("tenant_id=? AND run_id=? AND activity_key=?", 112, "run-two-phase-unknown", "activity-two-phase-unknown").Take(&journal).Error)
	require.Equal(t, "intent", journal.State, "failed resolution leaves the original unresolved intent")
	require.Len(t, craftReservations(t, db, 112), 1, "failed resolution keeps the dispatched G4 hold")

	require.NoError(t, db.Exec("DROP TRIGGER fail_two_phase_resolution").Error)
	require.NoError(t, attempt.Resolve(context.Background(), CraftChargeStartUnknown))
	require.Error(t, attempt.Resolve(context.Background(), CraftChargeStartStarted), "unknown cannot later be promoted to started")
	require.NoError(t, db.Where("tenant_id=? AND run_id=? AND activity_key=?", 112, "run-two-phase-unknown", "activity-two-phase-unknown").Take(&journal).Error)
	require.Equal(t, "unknown", journal.State)

	_, err = svc.BeginBinding(context.Background(), grant.ID, "activity-two-phase-unknown", CraftCallBinding{ModelID: "lead", Funding: commercial.FundingPlatform})
	require.ErrorIs(t, err, craft.ErrConflict, "same activity key never creates a second physical attempt")
	require.Len(t, craftReservations(t, db, 112), 1)
}

func TestCraftChargeStartCommitsIntentAndHoldBeforeCallbackAndReplayDoesNotSend(t *testing.T) {
	db := openCraftBudgetTestDB(t)
	svc, err := NewCraftBudgetService(db, nil, craftBudgetPolicy())
	require.NoError(t, err)
	seedCraftFundedTenant(t, db, 100, 10000)
	seedCraftBudgetRun(t, db, 100, "run-journal", "task-journal")
	grant, err := svc.Admit(context.Background(), craft.Scope{TenantID: 100, UserID: "owner", SessionID: "task-journal"}, "run-journal")
	require.NoError(t, err)

	starts := 0
	outcome, err := svc.StartBinding(context.Background(), grant.ID, "activity-stable", CraftCallBinding{ModelID: "lead", Funding: commercial.FundingPlatform},
		func(context.Context) (CraftChargeStartOutcome, error) {
			starts++
			var journal CraftChargeStartJournalRow
			require.NoError(t, db.Where("tenant_id = ? AND run_id = ? AND activity_key = ?", 100, "run-journal", "activity-stable").Take(&journal).Error)
			require.Equal(t, "intent", journal.State, "intent must be committed before external initiation")
			var reservation repocommercial.ReservationRow
			require.NoError(t, db.Where("tenant_id = ? AND key = ?", 100, CraftCallKey("activity/activity-stable")).Take(&reservation).Error)
			require.Equal(t, commercial.ReservationStateDispatched, reservation.State, "G4 hold must be dispatched before external initiation")
			return CraftChargeStartUnknown, errors.New("response lost after possible send")
		})
	require.Error(t, err)
	require.Equal(t, CraftChargeStartUnknown, outcome)
	require.Equal(t, 1, starts)

	var journal CraftChargeStartJournalRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ? AND activity_key = ?", 100, "run-journal", "activity-stable").Take(&journal).Error)
	require.Equal(t, "unknown", journal.State, "possible send with lost response remains unresolved")
	require.Len(t, craftReservations(t, db, 100), 1, "unknown outcome retains its G4 hold")

	_, err = svc.StartBinding(context.Background(), grant.ID, "activity-stable", CraftCallBinding{ModelID: "lead", Funding: commercial.FundingPlatform},
		func(context.Context) (CraftChargeStartOutcome, error) {
			starts++
			return CraftChargeStartStarted, nil
		})
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Equal(t, 1, starts, "same-key replay never invokes external start again")
}

func TestCraftChargeStartResultWriteFailureLeavesIntentAndHold(t *testing.T) {
	db := openCraftBudgetTestDB(t)
	svc, err := NewCraftBudgetService(db, nil, craftBudgetPolicy())
	require.NoError(t, err)
	seedCraftFundedTenant(t, db, 101, 10000)
	seedCraftBudgetRun(t, db, 101, "run-result-write", "task-result-write")
	grant, err := svc.Admit(context.Background(), craft.Scope{TenantID: 101, UserID: "owner", SessionID: "task-result-write"}, "run-result-write")
	require.NoError(t, err)

	_, err = svc.StartBinding(context.Background(), grant.ID, "activity-write-failure", CraftCallBinding{ModelID: "lead", Funding: commercial.FundingPlatform},
		func(context.Context) (CraftChargeStartOutcome, error) {
			// Model an accepted external operation followed by a DB failure while
			// recording its result. The committed intent must remain recoverable.
			require.NoError(t, db.Exec("CREATE TRIGGER fail_craft_journal_result BEFORE UPDATE OF state ON craft_charge_start_journal BEGIN SELECT RAISE(ABORT, 'injected outcome write failure'); END").Error)
			return CraftChargeStartStarted, nil
		})
	require.Error(t, err)
	var journal CraftChargeStartJournalRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ? AND activity_key = ?", 101, "run-result-write", "activity-write-failure").Take(&journal).Error)
	require.Equal(t, "intent", journal.State)
	require.Len(t, craftReservations(t, db, 101), 1)
}

func TestCraftChargeStartCallbackOutsideTransactionAllowsPauseAndCancellationRemainsUnknown(t *testing.T) {
	db := openCraftBudgetTestDB(t)
	if pool, err := db.DB(); err == nil {
		pool.SetMaxOpenConns(4)
	}
	svc, err := NewCraftBudgetService(db, nil, craftBudgetPolicy())
	require.NoError(t, err)
	seedCraftFundedTenant(t, db, 102, 10000)
	seedCraftBudgetRun(t, db, 102, "run-cancel", "task-cancel")
	grant, err := svc.Admit(context.Background(), craft.Scope{TenantID: 102, UserID: "owner", SessionID: "task-cancel"}, "run-cancel")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	type startResult struct {
		outcome CraftChargeStartOutcome
		err     error
	}
	done := make(chan startResult, 1)
	go func() {
		outcome, err := svc.StartBinding(ctx, grant.ID, "activity-cancel", CraftCallBinding{ModelID: "lead", Funding: commercial.FundingPlatform},
			func(context.Context) (CraftChargeStartOutcome, error) {
				once.Do(func() { close(entered) })
				<-release // ignores transport cancellation
				return CraftChargeStartStarted, nil
			})
		done <- startResult{outcome: outcome, err: err}
	}()
	<-entered
	cancel()
	require.NoError(t, svc.PauseRunForBudget(context.Background(), grant.ID), "callback cannot retain a SQL Run lock")
	close(release)
	select {
	case got := <-done:
		require.Error(t, got.err, "cancellation after possible send must remain unresolved")
		require.Equal(t, CraftChargeStartUnknown, got.outcome)
	case <-time.After(5 * time.Second):
		t.Fatal("start coordinator remained blocked after callback release")
	}
	var journal CraftChargeStartJournalRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ? AND activity_key = ?", 102, "run-cancel", "activity-cancel").Take(&journal).Error)
	// The OCR fix contract: the resolve write detaches from the canceled
	// caller context, so a mid-send cancellation PERSISTS the unknown
	// outcome instead of stranding the journal in 'intent' (where the
	// activity replay is refused and the lease recovery scan excludes the
	// Run until manual reconciliation). 'unknown' is the reconcilable state.
	require.Equal(t, "unknown", journal.State, "canceled result persistence records the unknown outcome for reconciliation")
	require.Len(t, craftReservations(t, db, 102), 1)
}

func TestCraftChargeStartDeniedPrepareNeverInvokesCallback(t *testing.T) {
	tests := []struct {
		name      string
		tenant    uint64
		maxCalls  int
		funds     int64
		wantError error
	}{
		{name: "max calls", tenant: 103, maxCalls: 1, funds: 10000, wantError: craft.ErrGrantExhausted},
		{name: "G4 reserve denied", tenant: 104, maxCalls: 5, funds: 100, wantError: craft.ErrBudgetDenied},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := openCraftBudgetTestDB(t)
			policy := craftBudgetPolicy()
			policy.MaxCalls = tc.maxCalls
			svc, err := NewCraftBudgetService(db, nil, policy)
			require.NoError(t, err)
			seedCraftFundedTenant(t, db, tc.tenant, tc.funds)
			runID, taskID := "run-denied-"+tc.name, "task-denied-"+tc.name
			seedCraftBudgetRun(t, db, tc.tenant, runID, taskID)
			grant, err := svc.Admit(context.Background(), craft.Scope{TenantID: tc.tenant, UserID: "owner", SessionID: taskID}, runID)
			require.NoError(t, err)
			if tc.name == "max calls" {
				_, err = svc.AuthorizeBinding(context.Background(), grant.ID, CraftCallBinding{ModelID: "prior", Funding: commercial.FundingPlatform})
				require.NoError(t, err)
			}

			starts := 0
			_, err = svc.StartBinding(context.Background(), grant.ID, "activity-denied", CraftCallBinding{ModelID: "lead", Funding: commercial.FundingPlatform},
				func(context.Context) (CraftChargeStartOutcome, error) {
					starts++
					return CraftChargeStartStarted, nil
				})
			require.ErrorIs(t, err, tc.wantError)
			require.Zero(t, starts, "a denied prepare must never authorize transport initiation")
			var journal CraftChargeStartJournalRow
			require.ErrorIs(t, db.Where("tenant_id = ? AND run_id = ? AND activity_key = ?", tc.tenant, runID, "activity-denied").Take(&journal).Error, gorm.ErrRecordNotFound)
			var run struct{ Status, WaitReason string }
			require.NoError(t, db.Table("agent_runs").Select("status, wait_reason").Where("tenant_id = ? AND run_id = ?", tc.tenant, runID).Take(&run).Error)
			require.Equal(t, "waiting_user", run.Status)
			require.Equal(t, craftBudgetWaitReason, run.WaitReason)
			if tc.name == "G4 reserve denied" {
				require.Empty(t, craftReservations(t, db, tc.tenant), "denied reservation and partial hold must roll back")
			}
		})
	}
}

func TestCraftChargeStartPrepareWriteFailureNeverInvokesCallback(t *testing.T) {
	db := openCraftBudgetTestDB(t)
	svc, err := NewCraftBudgetService(db, nil, craftBudgetPolicy())
	require.NoError(t, err)
	seedCraftFundedTenant(t, db, 105, 10000)
	seedCraftBudgetRun(t, db, 105, "run-prepare-failure", "task-prepare-failure")
	grant, err := svc.Admit(context.Background(), craft.Scope{TenantID: 105, UserID: "owner", SessionID: "task-prepare-failure"}, "run-prepare-failure")
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TRIGGER fail_craft_journal_prepare BEFORE INSERT ON craft_charge_start_journal BEGIN SELECT RAISE(ABORT, 'injected prepare write failure'); END").Error)
	starts := 0
	_, err = svc.StartBinding(context.Background(), grant.ID, "activity-prepare-failure", CraftCallBinding{ModelID: "lead", Funding: commercial.FundingPlatform},
		func(context.Context) (CraftChargeStartOutcome, error) {
			starts++
			return CraftChargeStartStarted, nil
		})
	require.Error(t, err)
	require.Zero(t, starts, "a failed prepare/commit must not authorize transport initiation")
	require.Empty(t, craftReservations(t, db, 105), "prepare transaction failure rolls back its G4 hold")
	require.Equal(t, int64(0), craftGrantCalls(t, db, 105, grant.ID), "prepare transaction failure rolls back call intent")
}

func TestCraftChargeStartCommitFailureNeverInvokesCallback(t *testing.T) {
	db := openCraftBudgetTestDB(t)
	svc, err := NewCraftBudgetService(db, nil, craftBudgetPolicy())
	require.NoError(t, err)
	seedCraftFundedTenant(t, db, 106, 10000)
	seedCraftBudgetRun(t, db, 106, "run-commit-failure", "task-commit-failure")
	grant, err := svc.Admit(context.Background(), craft.Scope{TenantID: 106, UserID: "owner", SessionID: "task-commit-failure"}, "run-commit-failure")
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE craft_commit_guard_parent (id INTEGER PRIMARY KEY)").Error)
	require.NoError(t, db.Exec("CREATE TABLE craft_commit_guard_child (parent_id INTEGER, FOREIGN KEY(parent_id) REFERENCES craft_commit_guard_parent(id) DEFERRABLE INITIALLY DEFERRED)").Error)
	require.NoError(t, db.Exec("CREATE TRIGGER fail_craft_start_commit AFTER INSERT ON craft_charge_start_journal BEGIN INSERT INTO craft_commit_guard_child(parent_id) VALUES (999); END").Error)

	starts := 0
	_, err = svc.StartBinding(context.Background(), grant.ID, "activity-commit-failure", CraftCallBinding{ModelID: "lead", Funding: commercial.FundingPlatform},
		func(context.Context) (CraftChargeStartOutcome, error) {
			starts++
			return CraftChargeStartStarted, nil
		})
	require.Error(t, err, "deferred foreign-key violation must fail transaction commit")
	require.Zero(t, starts, "a failed transaction commit cannot authorize transport initiation")
	require.Empty(t, craftReservations(t, db, 106), "failed commit rolls back the G4 hold")
	require.Equal(t, int64(0), craftGrantCalls(t, db, 106, grant.ID), "failed commit rolls back the call intent")
}

// TestCraftChargeStartDefinitelyUnstartedRestartClearsReservation pins the
// wrap-up OCR column fix: the clean-restart branch deletes the dispatched
// reservation by ReservationRow's OWN key column (`key`) — the previous
// reservation_key predicate matched no column and failed the whole restart
// transaction, so the "clean restart" never actually worked.
func TestCraftChargeStartDefinitelyUnstartedRestartClearsReservation(t *testing.T) {
	db := openCraftBudgetTestDB(t)
	svc, err := NewCraftBudgetService(db, nil, craftBudgetPolicy())
	require.NoError(t, err)
	seedCraftFundedTenant(t, db, 101, 10000)
	seedCraftBudgetRun(t, db, 101, "run-restart", "task-restart")
	scope := craft.Scope{TenantID: 101, UserID: "owner", SessionID: "task-restart"}
	grant, err := svc.Admit(context.Background(), scope, "run-restart")
	require.NoError(t, err)
	binding := CraftCallBinding{ModelID: "lead", Funding: commercial.FundingPlatform}

	// Land one started call (journal + call + dispatched reservation), then
	// mark the journal definitely_unstarted — the durable precondition the
	// restart branch exists to clean up after.
	_, err = svc.StartBinding(context.Background(), grant.ID, "activity-restart", binding,
		func(context.Context) (CraftChargeStartOutcome, error) {
			return CraftChargeStartStarted, nil
		})
	require.NoError(t, err)
	require.NoError(t, db.Model(&CraftChargeStartJournalRow{}).
		Where("tenant_id = ? AND run_id = ?", 101, "run-restart").
		Update("state", "definitely_unstarted").Error)
	require.Len(t, craftReservations(t, db, 101), 1)

	// The restart clears the stale rows in ONE transaction and re-reserves.
	outcome, err := svc.StartBinding(context.Background(), grant.ID, "activity-restart", binding,
		func(context.Context) (CraftChargeStartOutcome, error) {
			var reservation repocommercial.ReservationRow
			require.NoError(t, db.Where("tenant_id = ? AND key = ?", 101, CraftCallKey("activity/activity-restart")).Take(&reservation).Error)
			require.Equal(t, commercial.ReservationStateDispatched, reservation.State, "the restart re-reserves and dispatches")
			return CraftChargeStartStarted, nil
		})
	require.NoError(t, err, "the definitely_unstarted restart must succeed — a wrong column here used to fail the whole transaction")
	require.Equal(t, CraftChargeStartStarted, outcome)
	require.Len(t, craftReservations(t, db, 101), 1, "exactly ONE live reservation after the restart")
	var journal CraftChargeStartJournalRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 101, "run-restart").Take(&journal).Error)
	require.NotEqual(t, "definitely_unstarted", journal.State)
}
