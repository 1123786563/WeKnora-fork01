package container

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/stretchr/testify/require"
)

type fakeAgentSecurityReconciliationStore struct{ passes atomic.Int64 }

func (f *fakeAgentSecurityReconciliationStore) ListPendingRunCancellations(context.Context, int) ([]repository.PendingAgentSecurityCancellation, error) {
	f.passes.Add(1)
	return nil, nil
}
func (f *fakeAgentSecurityReconciliationStore) ReconcileRunCancellation(context.Context, uint64, string) (int64, error) {
	return 0, nil
}

func TestAgentSecurityReconcilerRunsAtStartupAndStopsWithCleaner(t *testing.T) {
	t.Setenv("AGENT_SECURITY_RECONCILE_INTERVAL", "5ms")
	store := &fakeAgentSecurityReconciliationStore{}
	cleaner := NewResourceCleaner()
	StartAgentSecurityReconciler(store, cleaner)
	deadline := time.Now().Add(time.Second)
	for store.passes.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	require.GreaterOrEqual(t, store.passes.Load(), int64(2), "startup and periodic passes should run")
	require.Empty(t, cleaner.Cleanup(context.Background()))
	stoppedAt := store.passes.Load()
	time.Sleep(20 * time.Millisecond)
	require.Equal(t, stoppedAt, store.passes.Load(), "cleanup waits for the worker and prevents later passes")
}
