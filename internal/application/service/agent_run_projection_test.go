package service

import (
	"encoding/json"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/stretchr/testify/require"
)

func TestEventsAfter(t *testing.T) {
	events := []agentruntime.RunEvent{{Seq: 1}, {Seq: 2}, {Seq: 3}}
	got := EventsAfter(events, 2)
	require.Equal(t, []agentruntime.RunEvent{{Seq: 3}}, got)
}

func TestRunEventJSON(t *testing.T) {
	in := agentruntime.RunEvent{Seq: 4, AttemptID: "a", Type: "attempt_replaced", Payload: json.RawMessage(`{"old":"x"}`)}
	raw, err := json.Marshal(in)
	require.NoError(t, err)
	var out agentruntime.RunEvent
	require.NoError(t, json.Unmarshal(raw, &out))
	require.Equal(t, in, out)
}
