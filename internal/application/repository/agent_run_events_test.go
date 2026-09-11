package repository

import (
	"context"
	"encoding/json"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func admitEventRun(t *testing.T) (*AgentRunStore, agentruntime.Fence) {
	db := openRunTestDB(t)
	s := NewAgentRunStore(db)
	in := agentruntime.Admission{Key: agentruntime.RunKey{TenantID: 1, RunID: "event-run"}, SessionID: "s1", UserID: "u1", RequestID: "event-q", AssistantMessageID: "event-a", RequestHash: "event-h", Snapshot: json.RawMessage(`{"version":1}`), UserMessage: json.RawMessage(`{"id":"event-u","role":"user","content":"hi"}`), AssistantMessage: json.RawMessage(`{"role":"assistant","content":""}`), Deadline: time.Now().Add(time.Hour)}
	_, err := s.Admit(context.Background(), in)
	require.NoError(t, err)
	f, err := s.Claim(context.Background(), in.Key, "worker", time.Minute)
	require.NoError(t, err)
	return s, f
}
func TestAgentRunEventsSequenceAndReplay(t *testing.T) {
	s, f := admitEventRun(t)
	a, err := s.AppendEvent(context.Background(), f, agentruntime.RunEvent{AttemptID: "a1", Type: "attempt_replaced", Payload: json.RawMessage(`{}`)})
	require.NoError(t, err)
	b, err := s.AppendEvent(context.Background(), f, agentruntime.RunEvent{AttemptID: "a1", Type: "token", Payload: json.RawMessage(`{"x":1}`)})
	require.NoError(t, err)
	require.Equal(t, int64(1), a.Seq)
	require.Equal(t, int64(2), b.Seq)
	got, err := s.ReadEvents(context.Background(), f.RunKey, 1, 10)
	require.NoError(t, err)
	require.Equal(t, []int64{2}, []int64{got[0].Seq})
}
func TestAgentRunFinalizeIdempotent(t *testing.T) {
	s, f := admitEventRun(t)
	require.NoError(t, s.Finalize(context.Background(), f, json.RawMessage(`{"content":"done"}`)))
	require.NoError(t, s.Finalize(context.Background(), f, json.RawMessage(`{"content":"done"}`)))
	var n int64
	require.NoError(t, s.db.Table("agent_run_events").Where("tenant_id=? AND run_id=? AND event_type=?", 1, "event-run", "run_completed").Count(&n).Error)
	require.Equal(t, int64(1), n)
	var status string
	require.NoError(t, s.db.Table("agent_runs").Select("status").Where("tenant_id=? AND run_id=?", 1, "event-run").Scan(&status).Error)
	require.Equal(t, "succeeded", status)
}
