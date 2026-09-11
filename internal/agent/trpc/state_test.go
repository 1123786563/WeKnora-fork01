package trpc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

func TestStateRoundTripPendingCalls(t *testing.T) {
	before := State{
		Version: 1, PendingCallIDs: []string{"c1", "c2"}, NextCallIndex: 1,
		AppliedCallIDs: map[string]bool{"c1": true}, ModelAttemptID: "m1", InputCursor: 9007199254740993,
		Messages:        []model.Message{model.NewAssistantMessage("completed response")},
		CompactionState: json.RawMessage(`{"version":1,"summary":"compacted","input_cursor":12}`),
		UsageAttempts: map[string]json.RawMessage{"m1": json.RawMessage(
			`{"version":1,"response":{"id":"m1","done":true},"usage":{"total_tokens":20}}`)},
	}
	raw, err := json.Marshal(before)
	require.NoError(t, err)
	var after State
	require.NoError(t, json.Unmarshal(raw, &after))
	require.Equal(t, before.PendingCallIDs, after.PendingCallIDs)
	require.Equal(t, 1, after.NextCallIndex)
	require.True(t, after.AppliedCallIDs["c1"])
	require.Equal(t, before.InputCursor, after.InputCursor)
}

func TestStateRejectsUnknownAndMalformedVersions(t *testing.T) {
	for _, raw := range []string{
		`{"version":2}`, `{"messages":[]}`,
		`{"version":1,"compaction_state":{"version":2}}`,
		`{"version":1,"compaction_state":{"version":1,"summary":4}}`,
		`{"version":1,"usage_attempts":{"m1":{"version":3}}}`,
		`{"version":1,"usage_attempts":{"m1":{}}}`,
		`{"version":1,"next_call_index":2,"pending_call_ids":["c1"]}`,
	} {
		var state State
		require.Error(t, json.Unmarshal([]byte(raw), &state), raw)
	}
}
