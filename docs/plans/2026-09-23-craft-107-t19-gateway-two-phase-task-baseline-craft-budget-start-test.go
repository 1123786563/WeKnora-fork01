package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

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
	require.Equal(t, "intent", journal.State, "canceled result persistence stays recoverable as intent")
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
