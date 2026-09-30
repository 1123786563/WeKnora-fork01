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
