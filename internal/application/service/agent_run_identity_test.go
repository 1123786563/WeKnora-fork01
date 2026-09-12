package service

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/stretchr/testify/require"
)

// TestExecuteDurableRunInjectsRunIdentity pins the worker contract: the
// execution context arrives without request identity, and the executor
// must derive tenant and owner from the durable run row. Before the fix
// the first MustTenantIDFromContext in the model path panicked and took
// the whole server down.
func TestExecuteDurableRunInjectsRunIdentity(t *testing.T) {
	db := openDurableRunTestDB(t)
	store := repository.NewAgentRunStore(db)
	prev := RegisteredAgentRunService()
	RegisterAgentRunService(NewAgentRunService(store))
	t.Cleanup(func() { RegisterAgentRunService(prev) })

	key := admitDurableRun(t, store, durableRunSnapshot(t))
	fence, err := store.Claim(context.Background(), key, "worker-1", time.Minute)
	require.NoError(t, err)

	svc := newDurableRunSessionService(t, db)
	// A bare context: no tenant, no principal — exactly what the
	// background worker hands the executor.
	execErr := svc.ExecuteDurableRun(context.Background(), fence)
	// The run completes (or fails with a model/tool error) but must never
	// panic on missing identity.
	_ = execErr
	run, getErr := store.Get(context.Background(), key)
	require.NoError(t, getErr)
	require.NotEqual(t, "", run.Status)
}
