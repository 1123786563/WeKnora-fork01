package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/stretchr/testify/require"
)

// seedTerminalRun 写入交错的事件序列：终端块稀疏地夹在普通事件之间，
// 分页读必须只返回 tool.terminal 子序列且按 seq 升序。
func seedTerminalRun(t *testing.T, count int) (agentruntime.RunKey, *AgentRunSnapshotRepository) {
	t.Helper()
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	snapshots := NewAgentRunSnapshotRepository(db)
	ctx := context.Background()
	key := agentruntime.RunKey{TenantID: 1, RunID: "term-r1"}
	user, _ := json.Marshal(map[string]any{"role": "user", "content": "q"})
	assistant, _ := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	_, err := store.Admit(ctx, agentruntime.Admission{
		Key: key, SessionID: "s1", UserID: "u1", RequestID: "term-q1",
		AssistantMessageID: "term-a1", RequestHash: "term-h1",
		Snapshot:    json.RawMessage(`{"version":1}`),
		UserMessage: user, AssistantMessage: assistant,
		Deadline: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	fence, err := store.Claim(ctx, key, "w", time.Minute)
	require.NoError(t, err)
	for i := 0; i < count; i++ {
		payload, _ := json.Marshal(map[string]any{"stream": "stdout", "text": "$ echo " + string(rune('a'+i))})
		_, err = store.AppendEvent(ctx, fence, agentruntime.RunEvent{Type: "tool.terminal", Payload: payload})
		require.NoError(t, err)
		noise, _ := json.Marshal(map[string]any{"n": i})
		_, err = store.AppendEvent(ctx, fence, agentruntime.RunEvent{Type: "text.delta", Payload: noise})
		require.NoError(t, err)
	}
	return key, snapshots
}

func TestReadRunTerminalEventsPagesSparseSubsequence(t *testing.T) {
	key, snapshots := seedTerminalRun(t, 3)

	first, err := snapshots.ReadRunTerminalEvents(context.Background(), key, 0, 2)
	require.NoError(t, err)
	require.Len(t, first, 2)
	require.EqualValues(t, 1, first[0].Seq)
	require.EqualValues(t, 3, first[1].Seq) // seq 2 是 text.delta，不得出现
	for _, event := range first {
		require.Equal(t, TerminalLogEventType, event.Type)
		require.NotEmpty(t, event.OccurredAt)
	}

	second, err := snapshots.ReadRunTerminalEvents(context.Background(), key, first[len(first)-1].Seq, 2)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.EqualValues(t, 5, second[0].Seq)

	third, err := snapshots.ReadRunTerminalEvents(context.Background(), key, 5, 2)
	require.NoError(t, err)
	require.Empty(t, third)
}

func TestReadRunTerminalEventsScopesByTenantAndRun(t *testing.T) {
	key, snapshots := seedTerminalRun(t, 1)

	foreignTenant := agentruntime.RunKey{TenantID: 2, RunID: key.RunID}
	empty, err := snapshots.ReadRunTerminalEvents(context.Background(), foreignTenant, 0, 10)
	require.NoError(t, err)
	require.Empty(t, empty)

	foreignRun := agentruntime.RunKey{TenantID: key.TenantID, RunID: "other-run"}
	emptyTwo, err := snapshots.ReadRunTerminalEvents(context.Background(), foreignRun, 0, 10)
	require.NoError(t, err)
	require.Empty(t, emptyTwo)
}

func TestReadRunTerminalEventsRejectsInvalidInput(t *testing.T) {
	key, snapshots := seedTerminalRun(t, 1)
	_, err := snapshots.ReadRunTerminalEvents(context.Background(), agentruntime.RunKey{}, 0, 10)
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
	_, err = snapshots.ReadRunTerminalEvents(context.Background(), key, -1, 10)
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
}
