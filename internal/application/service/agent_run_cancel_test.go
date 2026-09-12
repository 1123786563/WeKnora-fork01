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

func TestCancelReleasesSessionSlotOnRealStore(t *testing.T) {
	db := openDurableRunTestDB(t)
	store := repository.NewAgentRunStore(db)
	prev := RegisteredAgentRunService()
	runs := NewAgentRunService(store)
	RegisterAgentRunService(runs)
	t.Cleanup(func() { RegisterAgentRunService(prev) })

	key := admitDurableRun(t, store, durableRunSnapshot(t))
	require.NoError(t, runs.Cancel(context.Background(), key))

	run, err := store.Get(context.Background(), key)
	require.NoError(t, err)
	require.Equal(t, "canceled", run.Status)

	var slot *string
	require.NoError(t, db.Raw(
		"SELECT active_agent_run_id FROM sessions WHERE id = 's1'").Scan(&slot).Error)
	require.Nil(t, slot, "cancel must release the session slot")

	// A new run admits immediately after the cancellation.
	key2 := agentruntime.RunKey{TenantID: 1, RunID: "run-after-cancel"}
	user, _ := mustJSON(t, map[string]any{"role": "user", "content": "again"})
	assistant, _ := mustJSON(t, map[string]any{"role": "assistant", "content": ""})
	_, err = store.Admit(context.Background(), agentruntime.Admission{
		Key: key2, SessionID: "s1", UserID: "u1", RequestID: "q2",
		AssistantMessageID: "a2", RequestHash: "h2",
		Snapshot: durableRunSnapshot(t), UserMessage: user,
		AssistantMessage: assistant, Deadline: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)

	// A canceled run is never claimable again.
	_, err = store.Claim(context.Background(), key, "worker", time.Minute)
	require.ErrorIs(t, err, agentruntime.ErrLeaseLost)
}

func mustJSON(t *testing.T, v any) ([]byte, error) {
	t.Helper()
	return json.Marshal(v)
}
