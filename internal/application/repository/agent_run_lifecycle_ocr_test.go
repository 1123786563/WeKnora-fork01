package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestDeleteSessionRunsSucceedsWithTerminalRuns is the OCR high-finding
// regression: a session that contains a succeeded (or failed) Run must stay
// deletable. The pre-fix ids query selected terminal runs too and
// cancelRunTx refuses them with ErrConflict, so any session that had ever
// completed work failed to delete forever.
func TestDeleteSessionRunsSucceedsWithTerminalRuns(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)

	done := testAdmission()
	done.Key.RunID = "lifecycle-delete-succeeded"
	_, err := store.Admit(context.Background(), done)
	require.NoError(t, err)
	fence, err := store.Claim(context.Background(), done.Key, "worker", time.Minute)
	require.NoError(t, err)
	require.NoError(t, store.Finalize(context.Background(), fence, json.RawMessage(`{"content":"done"}`)))

	lost := testAdmission()
	lost.Key.RunID = "lifecycle-delete-failed"
	lost.SessionID = done.SessionID
	lost.RequestID = "q-lifecycle-2"
	lost.AssistantMessageID = "a-lifecycle-2"
	_, err = store.Admit(context.Background(), lost)
	require.NoError(t, err)
	lostFence, err := store.Claim(context.Background(), lost.Key, "worker", time.Minute)
	require.NoError(t, err)
	require.NoError(t, store.SetStatus(context.Background(), lostFence, "failed", "boom"))

	require.NoError(t, store.DeleteSessionRuns(context.Background(), 1, done.SessionID),
		"a session with terminal runs must be deletable")

	var statuses []string
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND session_id = ?", 1, done.SessionID).
		Order("run_id").Pluck("status", &statuses).Error)
	require.Equal(t, []string{"failed", "succeeded"}, statuses,
		"terminal runs and their settlement facts survive session deletion")
}
