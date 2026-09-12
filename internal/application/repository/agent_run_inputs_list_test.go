package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/stretchr/testify/require"
)

func TestAgentRunInputsListPendingByMode(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	key := agentruntime.RunKey{TenantID: 1, RunID: "inputs-r1"}
	ctx := context.Background()

	// Seed an admitted run so the input rows have a real scope.
	user, _ := json.Marshal(map[string]any{"role": "user", "content": "q"})
	assistant, _ := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	_, err := store.Admit(ctx, agentruntime.Admission{
		Key:                key,
		SessionID:          "s1",
		UserID:             "u1",
		RequestID:          "inputs-q1",
		AssistantMessageID: "inputs-a1",
		RequestHash:        "inputs-h1",
		Snapshot:           json.RawMessage(`{"version":1}`),
		UserMessage:        user,
		AssistantMessage:   assistant,
		Deadline:           time.Now().Add(time.Hour),
	})
	require.NoError(t, err)

	first := json.RawMessage(`{"role":"user","content":"one"}`)
	second := json.RawMessage(`{"role":"user","content":"two"}`)
	require.NoError(t, store.AppendInput(ctx, key,
		agentruntime.RunInput{SteerID: "st-1", Mode: "inject", Message: first}))
	require.NoError(t, store.AppendInput(ctx, key,
		agentruntime.RunInput{SteerID: "st-2", Mode: "after", Message: second}))
	require.NoError(t, store.AppendInput(ctx, key,
		agentruntime.RunInput{SteerID: "st-1", Mode: "inject", Message: first}),
		"identical retry must be idempotent")

	injects, err := store.ListPendingInputs(ctx, key, "inject")
	require.NoError(t, err)
	require.Len(t, injects, 1)
	require.Equal(t, "st-1", injects[0].SteerID)

	afters, err := store.ListPendingInputs(ctx, key, "after")
	require.NoError(t, err)
	require.Len(t, afters, 1)
	require.Equal(t, "st-2", afters[0].SteerID)

	_, err = store.ListPendingInputs(ctx, key, "bogus")
	require.ErrorIs(t, err, agentruntime.ErrConflict)
}
