package repository

import (
	"context"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/stretchr/testify/require"
)

func TestReadRunSnapshotUsesAuthoritativeProjection(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	_, err := store.Admit(context.Background(), testAdmission())
	require.NoError(t, err)
	require.NoError(t, db.Exec(`INSERT INTO agent_run_events
		(tenant_id, run_id, seq, attempt_id, event_type, payload, created_at)
		VALUES (1, 'r1', 1, 'attempt-1', 'text.delta', '{"text":"hello"}', ?)`, time.Now().UTC()).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_run_events
		(tenant_id, run_id, seq, attempt_id, event_type, payload, created_at)
		VALUES (1, 'r1', 2, 'attempt-1', 'run.completed', '{"ok":true}', ?)`, time.Now().UTC()).Error)

	snapshot, err := NewAgentRunSnapshotRepository(db).ReadRunSnapshot(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"})
	require.NoError(t, err)
	require.Equal(t, int64(2), snapshot.Watermark)
	require.Equal(t, []int64{1, 2}, []int64{snapshot.Events[0].Seq, snapshot.Events[1].Seq})
	require.Equal(t, "text.delta", snapshot.Events[0].Type)
	require.Equal(t, "platform", snapshot.Execution.Driver)
}

func TestReadRunEventsRejectsCursorBeforeRetainedHistory(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	_, err := store.Admit(context.Background(), testAdmission())
	require.NoError(t, err)
	require.NoError(t, db.Exec(`INSERT INTO agent_run_events
		(tenant_id, run_id, seq, attempt_id, event_type, payload)
		VALUES (1, 'r1', 7, 'attempt-1', 'text.delta', '{"text":"retained"}')`).Error)

	_, _, err = NewAgentRunSnapshotRepository(db).ReadRunEvents(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"}, 0, 10)
	require.ErrorIs(t, err, agentruntime.ErrCursorExpired)
}
