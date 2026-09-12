package repository

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/stretchr/testify/require"
)

func TestAgentRunEventRetentionTrim(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	ctx := context.Background()
	key := agentruntime.RunKey{TenantID: 1, RunID: "trim-r1"}
	user, _ := json.Marshal(map[string]any{"role": "user", "content": "q"})
	assistant, _ := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	_, err := store.Admit(ctx, agentruntime.Admission{
		Key: key, SessionID: "s1", UserID: "u1", RequestID: "trim-q1",
		AssistantMessageID: "trim-a1", RequestHash: "trim-h1",
		Snapshot:    json.RawMessage(`{"version":1}`),
		UserMessage: user, AssistantMessage: assistant,
		Deadline: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	fence, err := store.Claim(ctx, key, "w", time.Minute)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		payload, _ := json.Marshal(map[string]int{"n": i})
		_, err = store.AppendEvent(ctx, fence, agentruntime.RunEvent{
			Type: "tool_result", Payload: payload,
		})
		require.NoError(t, err)
	}

	last, err := store.LastEventSeq(ctx, key)
	require.NoError(t, err)
	require.EqualValues(t, 5, last)

	trimmed, err := store.TrimEventsBefore(ctx, key, 3)
	require.NoError(t, err)
	require.EqualValues(t, 2, trimmed)

	_, err = store.ReadEvents(ctx, key, 0, 10)
	require.ErrorIs(t, err, agentruntime.ErrCursorExpired)

	events, err := store.ReadEvents(ctx, key, 2, 10)
	require.NoError(t, err)
	require.Len(t, events, 3)
	require.EqualValues(t, 3, events[0].Seq)

	trimmed, err = store.TrimEventsBefore(ctx, key, 3)
	require.NoError(t, err)
	require.EqualValues(t, 0, trimmed)
	_, err = store.TrimEventsBefore(ctx, key, -1)
	require.True(t, errors.Is(err, agentruntime.ErrConflict))
}
