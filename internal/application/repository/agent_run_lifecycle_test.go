package repository

import (
	"context"
	"encoding/json"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestCancelRunReleasesSlotAndDeleteFences(t *testing.T) {
	db := openRunTestDB(t)
	s := NewAgentRunStore(db)
	key := agentruntime.RunKey{TenantID: 1, RunID: "lifecycle-run"}
	_, err := s.Admit(context.Background(), agentruntime.Admission{Key: key, SessionID: "s1", UserID: "u1", RequestID: "lifecycle-q", AssistantMessageID: "lifecycle-a", RequestHash: "h", Snapshot: json.RawMessage(`{"version":1}`), UserMessage: json.RawMessage(`{"role":"user"}`), AssistantMessage: json.RawMessage(`{"role":"assistant"}`), Deadline: time.Now().Add(time.Hour)})
	require.NoError(t, err)
	f, err := s.Claim(context.Background(), key, "worker", time.Minute)
	require.NoError(t, err)
	require.NoError(t, s.CancelRun(context.Background(), key, "user_canceled"))
	var n int64
	require.NoError(t, db.Table("agent_run_events").Where("tenant_id=? AND run_id=? AND event_type=?", 1, key.RunID, "cancellation_requested").Count(&n).Error)
	require.Equal(t, int64(1), n)
	run, err := s.Get(context.Background(), key)
	require.NoError(t, err)
	require.Equal(t, "canceled", run.Status)
	require.Error(t, s.SetStatus(context.Background(), f, "succeeded", "stale"))
	require.NoError(t, s.DeleteSessionRuns(context.Background(), 1, "s1"))
	_, err = s.Get(context.Background(), key)
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
}
