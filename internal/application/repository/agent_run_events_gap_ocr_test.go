package repository

// T18 (#137) OCR regressions: the ReadEvents gap rule covers the WHOLE
// returned page (a mid-page hole must 409 exactly like a trimmed window
// head), the removed head pre-check stays semantically covered by the first
// iteration, and FirstEventSeq resolves the honest cursor-recovery point.
import (
	"context"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/stretchr/testify/require"
)

func seedGapEvents(t *testing.T, seqs ...int64) *AgentRunStore {
	t.Helper()
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	_, err := store.Admit(context.Background(), testAdmission())
	require.NoError(t, err)
	for _, seq := range seqs {
		require.NoError(t, db.Exec(`INSERT INTO agent_run_events
			(tenant_id, run_id, seq, attempt_id, event_type, payload)
			VALUES (1, 'r1', ?, 'attempt-1', 'text.delta', '{}')`, seq).Error)
	}
	return store
}

func TestAgentRunStoreReadEventsRejectsMidPageGap(t *testing.T) {
	store := seedGapEvents(t, 1, 2, 7)
	key := agentruntime.RunKey{TenantID: 1, RunID: "r1"}

	// after=0: the first row (1 == after+1) passes, but 3..6 are missing
	// MID-PAGE — both real consumers advance the cursor to the page's max
	// seq, so the hole must surface now, never on a later page.
	_, err := store.ReadEvents(context.Background(), key, 0, 256)
	require.ErrorIs(t, err, agentruntime.ErrCursorExpired)

	// after=2 over the same retained window: the page [7] starts past the
	// missing 3 — also a hole in front of the requested page.
	_, err = store.ReadEvents(context.Background(), key, 2, 256)
	require.ErrorIs(t, err, agentruntime.ErrCursorExpired)

	// after=6: the client already consumed 1..6 and the page [7] is
	// contiguous from after+1 — it must deliver normally.
	events, err := store.ReadEvents(context.Background(), key, 6, 256)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, int64(7), events[0].Seq)
}

func TestAgentRunStoreReadEventsRejectsTrimmedWindowHead(t *testing.T) {
	// Retained window {5,6} with a cursor at 0: the removed head pre-check
	// is subsumed by the continuity loop's FIRST iteration (5 != 1).
	store := seedGapEvents(t, 5, 6)
	_, err := store.ReadEvents(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"}, 0, 256)
	require.ErrorIs(t, err, agentruntime.ErrCursorExpired)
	// from after=4 the same window replays normally.
	events, err := store.ReadEvents(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"}, 4, 256)
	require.NoError(t, err)
	require.Len(t, events, 2)
}

func TestAgentRunStoreFirstEventSeq(t *testing.T) {
	store := seedGapEvents(t, 3, 9)
	first, err := store.FirstEventSeq(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"})
	require.NoError(t, err)
	require.Equal(t, int64(3), first)

	// A run with no events resolves 0 — the recovery caller keeps its
	// cursor and projects nothing.
	db := openRunTestDB(t)
	empty := NewAgentRunStore(db)
	_, err = empty.Admit(context.Background(), testAdmission())
	require.NoError(t, err)
	first, err = empty.FirstEventSeq(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "r1"})
	require.NoError(t, err)
	require.Equal(t, int64(0), first)
}
