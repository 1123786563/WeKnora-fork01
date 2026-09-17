package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/stretchr/testify/require"
)

func TestExecutionDispatchClaimReceiptAndUnknownRecovery(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	in := testAdmission()
	_, err := store.Admit(context.Background(), in)
	require.NoError(t, err)
	dispatch := NewExecutionDispatchStore(db)
	ctx := context.Background()
	record, err := dispatch.ClaimDispatch(ctx, in.Key, "cmd-1", "worker-1", time.Minute)
	require.NoError(t, err)
	require.Equal(t, "claimed", record.State)
	require.True(t, record.New)
	require.Equal(t, "cmd-1", record.AttemptID)
	require.NoError(t, dispatch.ReconcileUnknown(ctx, record, "transport_lost", ""))
	record.State = "unknown"
	require.NoError(t, dispatch.ReconcileUnknown(ctx, record, "running", "external-1"))
	got, err := dispatch.ClaimDispatch(ctx, in.Key, "cmd-1", "worker-2", time.Minute)
	require.NoError(t, err)
	require.Equal(t, "reconciled", got.State)
	require.Equal(t, "external-1", got.ExternalID)
}

func TestExecutionDispatchPayloadHashAndUnknownRequireExplicitRecovery(t *testing.T) {
	db := openRunTestDB(t)
	in := testAdmission()
	_, err := NewAgentRunStore(db).Admit(context.Background(), in)
	require.NoError(t, err)
	dispatch := NewExecutionDispatchStore(db)
	ctx := context.Background()
	record, err := dispatch.ClaimDispatchWithPayloadHash(ctx, in.Key, "cmd-hash", "hash-a", "worker-a", time.Minute)
	require.NoError(t, err)
	_, err = dispatch.ClaimDispatchWithPayloadHash(ctx, in.Key, "cmd-hash", "hash-b", "worker-a", time.Minute)
	require.ErrorIs(t, err, ErrDispatchConflict)
	require.NoError(t, dispatch.ReconcileUnknown(ctx, record, "not_started", ""))
	_, err = dispatch.ClaimDispatchWithPayloadHash(ctx, in.Key, "cmd-hash", "hash-a", "worker-b", time.Minute)
	require.ErrorIs(t, err, ErrDispatchUnknown)
	recovered, err := dispatch.RecoverUnknown(ctx, record, "worker-b", time.Minute)
	require.NoError(t, err)
	require.True(t, recovered.New)
	// The old claimant cannot overwrite the newer lease after recovery. This
	// is deliberately checked before SaveReceipt so reconciliation cannot
	// clear the replacement worker's lease either.
	require.ErrorIs(t, dispatch.ReconcileUnknown(ctx, record, "late_observation", "late-external"), ErrDispatchLeaseLost)
	require.ErrorIs(t, dispatch.SaveReceipt(ctx, record, "stale-external"), ErrDispatchLeaseLost)
	require.NoError(t, dispatch.SaveReceipt(ctx, recovered, "external"))
}

func TestExecutionDispatchRejectsStaleEpochAndTenantCrossing(t *testing.T) {
	db := openRunTestDB(t)
	_, err := NewAgentRunStore(db).Admit(context.Background(), testAdmission())
	require.NoError(t, err)
	dispatch := NewExecutionDispatchStore(db)
	record, err := dispatch.ClaimDispatch(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"}, "cmd-1", "worker", time.Minute)
	require.NoError(t, err)
	bad := record
	bad.TenantID = 2
	require.ErrorIs(t, dispatch.SaveReceipt(context.Background(), bad, "external"), agentruntime.ErrNotFound)
	bad = record
	bad.Epoch++
	require.ErrorIs(t, dispatch.SaveReceipt(context.Background(), bad, "external"), ErrDispatchLeaseLost)
	_, err = dispatch.ClaimDispatchWithPayloadHash(context.Background(), agentruntime.RunKey{TenantID: 2, RunID: "r1"}, "cmd-1", "h", "worker", time.Minute)
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
}

func TestExecutionDispatchSQLiteMigrationHead(t *testing.T) {
	db := openRunTestDB(t)
	if db.Dialector.Name() != "sqlite" {
		t.Skip("SQLite head assertion only")
	}
	var version int64
	require.NoError(t, db.Raw("SELECT version FROM schema_migrations").Scan(&version).Error)
	require.EqualValues(t, 57, version)
	var table string
	require.NoError(t, db.Raw("SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'execution_dispatches'").Scan(&table).Error)
	require.Equal(t, "execution_dispatches", table)
}

func TestExecutionDispatchConcurrentClaimHasOneDurableRecord(t *testing.T) {
	db := openRunTestDB(t)
	_, err := NewAgentRunStore(db).Admit(context.Background(), testAdmission())
	require.NoError(t, err)
	dispatch := NewExecutionDispatchStore(db)
	const workers = 20
	results := make(chan DispatchRecord, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			record, err := dispatch.ClaimDispatchWithPayloadHash(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"}, "cmd-race", "hash-race", "worker", time.Minute)
			if err != nil {
				errs <- err
				return
			}
			if record.New {
				results <- record
			}
		}(i)
	}
	wg.Wait()
	close(results)
	close(errs)
	var claimed int
	for range results {
		claimed++
	}
	for err := range errs {
		require.ErrorIs(t, err, ErrDispatchBusy)
	}
	require.Equal(t, 1, claimed)
	var count int64
	require.NoError(t, db.Table("execution_dispatches").Where("tenant_id = ? AND command_id = ?", 1, "cmd-race").Count(&count).Error)
	require.EqualValues(t, 1, count)
}
