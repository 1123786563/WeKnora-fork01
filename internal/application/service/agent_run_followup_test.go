package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/stretchr/testify/require"
)

// TestExecuteDurableRunAdmitsAfterFollowUp pins the after-mode row: a
// steering message parked with delivery=after on the running run is
// admitted as the next durable run only after the current one finishes,
// carrying the parked content as its query under the same snapshot.
func TestExecuteDurableRunAdmitsAfterFollowUp(t *testing.T) {
	db := openDurableRunTestDB(t)
	store := repository.NewAgentRunStore(db)
	prev := RegisteredAgentRunService()
	RegisterAgentRunService(NewAgentRunService(store))
	t.Cleanup(func() { RegisterAgentRunService(prev) })

	key := admitDurableRun(t, store, durableRunSnapshot(t))
	message, err := json.Marshal(map[string]any{"role": "user", "content": "and then check again"})
	require.NoError(t, err)
	require.NoError(t, store.AppendInput(context.Background(), key, agentruntime.RunInput{
		SteerID: "after-1", Mode: "after", Message: message,
	}))

	fence, err := store.Claim(context.Background(), key, "worker-1", time.Minute)
	require.NoError(t, err)
	svc := newDurableRunSessionService(t, db)
	require.NoError(t, svc.ExecuteDurableRun(context.Background(), fence))

	run, err := store.Get(context.Background(), key)
	require.NoError(t, err)
	require.Equal(t, "succeeded", run.Status)

	// The follow-up run was admitted on the same session and holds the
	// session slot, waiting for the worker to claim it.
	var followID string
	require.NoError(t, db.Raw(
		"SELECT active_agent_run_id FROM sessions WHERE id = 's1'").Scan(&followID).Error)
	require.NotEmpty(t, followID, "follow-up admission must hold the session slot")
	require.NotEqual(t, key.RunID, followID)
	follow, err := store.Get(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: followID})
	require.NoError(t, err)
	require.Equal(t, "s1", follow.SessionID)
	require.Equal(t, "queued", follow.Status)
	var snap DurableRunSnapshot
	require.NoError(t, json.Unmarshal(follow.Snapshot, &snap))
	require.Equal(t, "and then check again", snap.Query)

	// The parked after input is consumed exactly once.
	pending, err := store.ListPendingInputs(context.Background(), key, "after")
	require.NoError(t, err)
	require.Empty(t, pending)
}
