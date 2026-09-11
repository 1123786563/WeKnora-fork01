package repository

import (
	"context"
	"encoding/json"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestAgentRunInputApplyIsIdempotent(t *testing.T) {
	s, f := admitEventRun(t)
	in := agentruntime.RunInput{SteerID: "steer-1", Mode: "inject", Message: json.RawMessage(`{"content":"more"}`)}
	require.NoError(t, s.AppendInput(context.Background(), f.RunKey, in))
	cp := agentruntime.CheckpointRecord{Namespace: "n", ID: "c1", Seq: 1, State: json.RawMessage(`{"v":1}`), PendingWrites: json.RawMessage(`{}`)}
	require.NoError(t, s.ApplyInput(context.Background(), f, "steer-1", cp))
	require.NoError(t, s.ApplyInput(context.Background(), f, "steer-1", cp))
	var status string
	require.NoError(t, s.db.Table("agent_run_inputs").Select("status").Where("tenant_id=? AND run_id=? AND steer_id=?", 1, "event-run", "steer-1").Scan(&status).Error)
	require.Equal(t, "processed", status)
	_ = time.Now()
}
