package repository

import (
	"context"
	"encoding/json"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
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
	require.NoError(t, s.CancelRun(context.Background(), key, "quote \"x\" \\ path"))
	var n int64
	require.NoError(t, db.Table("agent_run_events").Where("tenant_id=? AND run_id=? AND event_type=?", 1, key.RunID, "cancellation_requested").Count(&n).Error)
	require.Equal(t, int64(1), n)
	var payload string
	require.NoError(t, db.Table("agent_run_events").Select("payload").Where("tenant_id=? AND run_id=? AND event_type=?", 1, key.RunID, "cancellation_requested").Scan(&payload).Error)
	var decoded map[string]string
	require.NoError(t, json.Unmarshal([]byte(payload), &decoded))
	require.Equal(t, "quote \"x\" \\ path", decoded["reason"])
	run, err := s.Get(context.Background(), key)
	require.NoError(t, err)
	require.Equal(t, "canceled", run.Status)
	require.Error(t, s.SetStatus(context.Background(), f, "succeeded", "stale"))
	require.NoError(t, s.DeleteSessionRuns(context.Background(), 1, "s1"))
	var canceledEvents int64
	require.NoError(t, db.Table("agent_run_events").Where("tenant_id=? AND run_id=? AND event_type=?", 1, key.RunID, "cancellation_requested").Count(&canceledEvents).Error)
	require.Equal(t, int64(2), canceledEvents, "session deletion appends a durable cancellation event")
	run, err = s.Get(context.Background(), key)
	require.NoError(t, err)
	require.Equal(t, "canceled", run.Status)
	var state string
	require.NoError(t, db.Table("execution_cleanup").Where("tenant_id=? AND session_id=?", 1, "s1").Pluck("state", &state).Error)
	require.Equal(t, "tombstoned", state)
}
