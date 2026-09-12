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

func jsonRaw(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return raw
}

func TestExecuteDurableRunRejectsIncompatibleCheckpoint(t *testing.T) {
	db := openDurableRunTestDB(t)
	store := repository.NewAgentRunStore(db)
	prev := RegisteredAgentRunService()
	RegisterAgentRunService(NewAgentRunService(store))
	t.Cleanup(func() { RegisterAgentRunService(prev) })

	key := admitDurableRun(t, store, durableRunSnapshot(t))
	fence, err := store.Claim(context.Background(), key, "worker-1", time.Minute)
	require.NoError(t, err)

	// Seed a checkpoint in the CURRENT namespace whose envelope claims a
	// foreign graph version: decode must reject it on resume instead of
	// executing unknown state.
	require.NoError(t, store.SaveCheckpoint(context.Background(), fence, agentruntime.CheckpointRecord{
		Namespace: "tenant/1/run/" + key.RunID + "/graph/1",
		ID:        "cp-foreign", ParentID: "", Seq: 1,
		State: jsonRaw(t, map[string]any{
			"version": 1, "graph_version": "foreign",
			"sdk_version": "v1.10.0", "tuple": map[string]any{},
		}),
		PendingWrites: jsonRaw(t, []any{}),
	}))
	// Release the claim so the executor can reclaim.
	require.NoError(t, db.Exec(
		"UPDATE agent_runs SET lease_until = ? WHERE tenant_id = 1 AND run_id = ?",
		time.Now().Add(-time.Minute), key.RunID).Error)

	svc := newDurableRunSessionService(t, db)
	freshFence, err := store.Claim(context.Background(), key, "worker-2", time.Minute)
	require.NoError(t, err)
	err = svc.ExecuteDurableRun(context.Background(), freshFence)
	require.Error(t, err)
	require.Contains(t, err.Error(), "incompatible checkpoint",
		"a foreign graph version must fail explicitly, got: %v", err)
}
