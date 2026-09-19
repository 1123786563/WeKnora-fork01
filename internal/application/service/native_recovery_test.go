package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/stretchr/testify/require"
)

type nativeRecoveryLeaseFake struct {
	runs    []nativecontract.RunIdentity
	claim   map[string]error
	claimed []string
}

func (f *nativeRecoveryLeaseFake) ScanRecoverable(context.Context, int) ([]nativecontract.RunIdentity, error) {
	return append([]nativecontract.RunIdentity(nil), f.runs...), nil
}

func (f *nativeRecoveryLeaseFake) Claim(_ context.Context, run nativecontract.RunIdentity, owner string, _ time.Duration) (nativecontract.Fence, error) {
	f.claimed = append(f.claimed, run.RunID)
	if err := f.claim[run.RunID]; err != nil {
		return nativecontract.Fence{}, err
	}
	return nativecontract.Fence{Run: run, Owner: owner, Epoch: 2, LeaseUntil: time.Now().Add(time.Minute)}, nil
}

type nativeRecoveryControlsFake struct {
	value nativecontract.AdmissionControls
}

func (f nativeRecoveryControlsFake) Current(context.Context) (nativecontract.AdmissionControls, error) {
	return f.value, nil
}

func TestNativeRecoveryClaimsOnlyExpiredQualifiedRunsAndIgnoresRacingOwner(t *testing.T) {
	store := &nativeRecoveryLeaseFake{
		runs:  []nativecontract.RunIdentity{{TenantID: 1, RunID: "expired-a"}, {TenantID: 1, RunID: "expired-b"}},
		claim: map[string]error{"expired-b": &nativecontract.Failure{Code: nativecontract.ErrLeaseLost}},
	}
	service := NewNativeRecoveryService(nativeRecoveryControlsFake{value: nativeRecoveryOpenControls()}, store)
	fences, err := service.Recover(context.Background(), "recovery-worker", time.Minute, 10)
	require.NoError(t, err)
	require.Len(t, fences, 1)
	require.Equal(t, "expired-a", fences[0].Run.RunID)
	require.Equal(t, []string{"expired-a", "expired-b"}, store.claimed)
}

func TestNativeRecoveryHoldsWorkWhenExecutionGateIsClosedButDrainDoesNotBlockExistingRuns(t *testing.T) {
	store := &nativeRecoveryLeaseFake{runs: []nativecontract.RunIdentity{{TenantID: 1, RunID: "expired"}}, claim: map[string]error{}}
	closed := nativeRecoveryOpenControls()
	closed.NativeExecutionApproved = false
	service := NewNativeRecoveryService(nativeRecoveryControlsFake{value: closed}, store)
	_, err := service.Recover(context.Background(), "worker", time.Minute, 1)
	var failure *nativecontract.Failure
	require.True(t, errors.As(err, &failure))
	require.Equal(t, nativecontract.ErrExecutionGate, failure.Code)
	require.Empty(t, store.claimed)

	draining := nativeRecoveryOpenControls()
	draining.WorkerDrain = true
	service = NewNativeRecoveryService(nativeRecoveryControlsFake{value: draining}, store)
	fences, err := service.Recover(context.Background(), "worker", time.Minute, 1)
	require.NoError(t, err)
	require.Len(t, fences, 1)
}
