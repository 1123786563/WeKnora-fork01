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
	require.Equal(t, "cmd-1", record.AttemptID)
	require.NoError(t, dispatch.ReconcileUnknown(ctx, record, "transport_lost", ""))
	record.State = "unknown"
	require.NoError(t, dispatch.ReconcileUnknown(ctx, record, "running", "external-1"))
	require.NoError(t, dispatch.SaveReceipt(ctx, record, "external-1"))
	require.NoError(t, dispatch.SaveReceipt(ctx, record, "external-1"))
	got, err := dispatch.ClaimDispatch(ctx, in.Key, "cmd-1", "worker-2", time.Minute)
	require.NoError(t, err)
	require.Equal(t, "completed", got.State)
	require.Equal(t, "external-1", got.ExternalID)
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
}

func TestExecutionDispatchConcurrentClaimHasOneDurableRecord(t *testing.T) {
	db := openRunTestDB(t)
	_, err := NewAgentRunStore(db).Admit(context.Background(), testAdmission())
	require.NoError(t, err)
	dispatch := NewExecutionDispatchStore(db)
	const workers = 8
	results := make(chan DispatchRecord, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			record, err := dispatch.ClaimDispatch(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"}, "cmd-race", "worker-"+string(rune('a'+i)), time.Minute)
			if err != nil {
				errs <- err
				return
			}
			results <- record
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
