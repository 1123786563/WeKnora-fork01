package repository

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/nativecontract"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func nativeLeaseFixture(t *testing.T, db *gorm.DB, tenant uint64, runID, status string, expiresAt *time.Time) nativecontract.RunIdentity {
	t.Helper()
	if tenant == 2 {
		require.NoError(t, db.Exec("INSERT INTO tenants (id, name, business) VALUES (?, ?, ?)", 2, "tenant-2", "test").Error)
	}
	require.NoError(t, db.Exec("INSERT INTO native_agent_tenants (tenant_id) VALUES (?) ON CONFLICT DO NOTHING", tenant).Error)
	require.NoError(t, db.Exec("INSERT INTO native_agent_sessions (tenant_id, owner_id, session_id) VALUES (?, ?, ?) ON CONFLICT DO NOTHING", tenant, "owner", "session").Error)
	require.NoError(t, db.Exec(`INSERT INTO native_agent_runs
		(tenant_id, run_id, owner_id, session_id, request_id, input_hash, status, lease_epoch, lease_expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, tenant, runID, "owner", "session", runID, "hash-"+runID, status, 0, expiresAt).Error)
	return nativecontract.RunIdentity{TenantID: tenant, RunID: runID, SessionID: "session"}
}

func nativeLeaseCode(t *testing.T, err error) nativecontract.ErrorCode {
	t.Helper()
	var failure *nativecontract.Failure
	require.True(t, errors.As(err, &failure), "expected native failure, got %v", err)
	return failure.Code
}

func TestNativeLeaseClaimIsTenantScopedAndFencesConcurrentWorkers(t *testing.T) {
	db := openRunTestDB(t)
	run := nativeLeaseFixture(t, db, 1, "run-1", string(nativecontract.RunQueued), nil)
	store := NewNativeLeaseStore(db)

	fence, err := store.Claim(context.Background(), run, "worker-a", time.Now(), time.Minute)
	require.NoError(t, err)
	require.Equal(t, int64(1), fence.Epoch)
	require.Equal(t, "worker-a", fence.Owner)
	require.False(t, fence.LeaseUntil.IsZero())

	_, err = store.Claim(context.Background(), run, "worker-b", time.Now(), time.Minute)
	require.Equal(t, nativecontract.ErrLeaseLost, nativeLeaseCode(t, err))
	_, err = store.Claim(context.Background(), nativecontract.RunIdentity{TenantID: 2, RunID: run.RunID}, "worker-c", time.Now(), time.Minute)
	require.Equal(t, nativecontract.ErrLeaseLost, nativeLeaseCode(t, err))
}

func TestNativeLeaseExpiryAdvancesEpochAndRejectsStaleWrites(t *testing.T) {
	db := openRunTestDB(t)
	expired := time.Now().Add(-time.Hour)
	run := nativeLeaseFixture(t, db, 1, "run-expired", string(nativecontract.RunRunning), &expired)
	require.NoError(t, db.Exec("UPDATE native_agent_runs SET lease_owner=?, lease_epoch=? WHERE tenant_id=? AND run_id=?", "old-worker", 1, 1, run.RunID).Error)
	store := NewNativeLeaseStore(db)

	old := nativecontract.Fence{Run: run, Owner: "old-worker", Epoch: 1, LeaseUntil: expired}
	fresh, err := store.Claim(context.Background(), run, "new-worker", time.Now(), time.Minute)
	require.NoError(t, err)
	require.Equal(t, int64(2), fresh.Epoch)
	_, err = store.Renew(context.Background(), old, time.Now(), time.Minute)
	require.Equal(t, nativecontract.ErrLeaseLost, nativeLeaseCode(t, err))
	require.Equal(t, nativecontract.ErrLeaseLost, nativeLeaseCode(t, store.Transition(context.Background(), old, nativecontract.RunSucceeded)))

	var status string
	require.NoError(t, db.Table("native_agent_runs").Select("status").Where("tenant_id=? AND run_id=?", 1, run.RunID).Scan(&status).Error)
	require.Equal(t, string(nativecontract.RunRunning), status)
}

func TestNativeLeaseRenewAndTransitionsRequireLiveFence(t *testing.T) {
	db := openRunTestDB(t)
	run := nativeLeaseFixture(t, db, 1, "run-transition", string(nativecontract.RunQueued), nil)
	store := NewNativeLeaseStore(db)
	fence, err := store.Claim(context.Background(), run, "worker", time.Now(), time.Minute)
	require.NoError(t, err)

	renewed, err := store.Renew(context.Background(), fence, time.Now(), 2*time.Minute)
	require.NoError(t, err)
	require.Equal(t, fence.Run, renewed.Run)
	require.Equal(t, fence.Owner, renewed.Owner)
	require.Equal(t, fence.Epoch, renewed.Epoch)
	require.True(t, renewed.LeaseUntil.After(fence.LeaseUntil))
	require.Equal(t, nativecontract.ErrConflict, nativeLeaseCode(t, store.Transition(context.Background(), fence, nativecontract.RunQueued)))
	require.NoError(t, store.Transition(context.Background(), fence, nativecontract.RunWaiting))
	_, err = store.Renew(context.Background(), fence, time.Now(), time.Minute)
	require.Equal(t, nativecontract.ErrLeaseLost, nativeLeaseCode(t, err))
}

func TestNativeLeaseRecoveryScanOnlyReturnsExpiredInFlightRuns(t *testing.T) {
	db := openRunTestDB(t)
	expired := time.Now().Add(-time.Hour)
	active := time.Now().Add(time.Hour)
	expected := nativeLeaseFixture(t, db, 1, "recover-me", string(nativecontract.RunRunning), &expired)
	nativeLeaseFixture(t, db, 1, "queued", string(nativecontract.RunQueued), nil)
	nativeLeaseFixture(t, db, 1, "waiting", string(nativecontract.RunWaiting), &expired)
	nativeLeaseFixture(t, db, 1, "terminal", string(nativecontract.RunSucceeded), &expired)
	nativeLeaseFixture(t, db, 1, "live", string(nativecontract.RunRunning), &active)

	runs, err := NewNativeLeaseStore(db).ScanRecoverable(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, []nativecontract.RunIdentity{expected}, runs)
}

func TestNativeLeaseConcurrentClaimHasOneWinner(t *testing.T) {
	db := openRunTestDB(t)
	run := nativeLeaseFixture(t, db, 1, "run-race", string(nativecontract.RunQueued), nil)
	stores := []*NativeLeaseStore{NewNativeLeaseStore(db), NewNativeLeaseStore(reopenRunDB(t, db))}
	start := make(chan struct{})
	errs := make(chan error, len(stores))
	var wg sync.WaitGroup
	for i, store := range stores {
		wg.Add(1)
		go func(owner string, store *NativeLeaseStore) {
			defer wg.Done()
			<-start
			_, err := store.Claim(context.Background(), run, owner, time.Now(), time.Minute)
			errs <- err
		}(string(rune('a'+i)), store)
	}
	close(start)
	wg.Wait()
	close(errs)
	winners := 0
	for err := range errs {
		if err == nil {
			winners++
			continue
		}
		require.Equal(t, nativecontract.ErrLeaseLost, nativeLeaseCode(t, err))
	}
	require.Equal(t, 1, winners)
}
