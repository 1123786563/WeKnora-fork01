package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/stretchr/testify/require"
)

func TestAgentRunSetStatusFailedReleasesSessionSlot(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	ctx := context.Background()
	key := agentruntime.RunKey{TenantID: 1, RunID: "r1"}
	_, err := store.Admit(ctx, agentruntime.Admission{
		Key:                key,
		SessionID:          "s1",
		UserID:             "u1",
		RequestID:          "q1",
		AssistantMessageID: "a1",
		RequestHash:        "h1",
		Snapshot:           json.RawMessage(`{"version":1}`),
		UserMessage:        json.RawMessage(`{"role":"user"}`),
		AssistantMessage:   json.RawMessage(`{"role":"assistant"}`),
		Deadline:           time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	fence, err := store.Claim(ctx, key, "worker-1", time.Minute)
	require.NoError(t, err)

	require.NoError(t, store.SetStatus(ctx, fence, "failed", "model unavailable"))

	var slot *string
	require.NoError(t, db.Raw("SELECT active_agent_run_id FROM sessions WHERE id = 's1'").Scan(&slot).Error)
	require.Nil(t, slot, "a terminal failure must release the session slot")

	// The session admits a new run immediately after the failure.
	key2 := agentruntime.RunKey{TenantID: 1, RunID: "r2"}
	_, err = store.Admit(ctx, agentruntime.Admission{
		Key:                key2,
		SessionID:          "s1",
		UserID:             "u1",
		RequestID:          "q2",
		AssistantMessageID: "a2",
		RequestHash:        "h2",
		Snapshot:           json.RawMessage(`{"version":1}`),
		UserMessage:        json.RawMessage(`{"role":"user"}`),
		AssistantMessage:   json.RawMessage(`{"role":"assistant"}`),
		Deadline:           time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
}

func TestAgentRunSetStatusWaitingKeepsSessionSlot(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	ctx := context.Background()
	key := agentruntime.RunKey{TenantID: 1, RunID: "r1"}
	_, err := store.Admit(ctx, agentruntime.Admission{
		Key:                key,
		SessionID:          "s1",
		UserID:             "u1",
		RequestID:          "q1",
		AssistantMessageID: "a1",
		RequestHash:        "h1",
		Snapshot:           json.RawMessage(`{"version":1}`),
		UserMessage:        json.RawMessage(`{"role":"user"}`),
		AssistantMessage:   json.RawMessage(`{"role":"assistant"}`),
		Deadline:           time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	fence, err := store.Claim(ctx, key, "worker-1", time.Minute)
	require.NoError(t, err)

	require.NoError(t, store.SetStatus(ctx, fence, "waiting_user", "call-1"))

	var slot *string
	require.NoError(t, db.Raw("SELECT active_agent_run_id FROM sessions WHERE id = 's1'").Scan(&slot).Error)
	require.NotNil(t, slot, "waiting_user occupies the active slot per spec")
	require.Equal(t, "r1", *slot)
}
