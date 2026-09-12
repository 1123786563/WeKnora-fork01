package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/stretchr/testify/require"
)

// TestAgentRunAdmitToleratesPrecreatedAssistantMessage pins the production
// HTTP contract: the SSE handler persists the assistant placeholder with the
// request-scoped id before admission runs, so the admission transaction must
// not fail on the existing row - the run owns finalization by id regardless.
func TestAgentRunAdmitToleratesPrecreatedAssistantMessage(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	ctx := context.Background()

	user, _ := json.Marshal(map[string]any{"role": "user", "content": "q"})
	assistant, _ := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	adm := agentruntime.Admission{
		Key:       agentruntime.RunKey{TenantID: 1, RunID: "adm-pre-r1"},
		SessionID: "s1", UserID: "u1", RequestID: "adm-pre-q1",
		AssistantMessageID: "amsg-pre-1", RequestHash: "adm-pre-h1",
		Snapshot:    json.RawMessage(`{"version":1}`),
		UserMessage: user, AssistantMessage: assistant,
		Deadline: time.Now().Add(time.Hour),
	}
	_, err := store.Admit(ctx, adm)
	require.NoError(t, err)

	// Terminal the first run and free the slot, then admit a second run
	// that REUSES the same assistant message id, exactly like a handler
	// retry that already persisted the assistant placeholder.
	require.NoError(t, db.Exec(
		"UPDATE agent_runs SET status='succeeded' WHERE tenant_id=1 AND run_id='adm-pre-r1'").Error)
	require.NoError(t, db.Exec(
		"UPDATE sessions SET active_agent_run_id=NULL WHERE id='s1'").Error)
	adm2 := adm
	adm2.Key.RunID = "adm-pre-r2"
	adm2.RequestID = "adm-pre-q2"
	adm2.RequestHash = "adm-pre-h2"
	run2, err := store.Admit(ctx, adm2)
	require.NoError(t, err, "admission must tolerate the handler-precreated assistant row")
	require.NotEmpty(t, run2.Key.RunID)
}
