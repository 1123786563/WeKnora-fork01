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

func admitRetentionRun(t *testing.T, runID, sessionID string) (*AgentRunStore, agentruntime.Fence) {
	t.Helper()
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	key := agentruntime.RunKey{TenantID: 1, RunID: runID}
	user, _ := json.Marshal(map[string]any{"role": "user", "content": "q"})
	assistant, _ := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	_, err := store.Admit(context.Background(), agentruntime.Admission{
		Key: key, SessionID: sessionID, UserID: "u1", RequestID: "retention-q-" + runID,
		AssistantMessageID: "retention-a-" + runID, RequestHash: "retention-h-" + runID,
		Snapshot:    json.RawMessage(`{"version":1}`),
		UserMessage: user, AssistantMessage: assistant,
		Deadline: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	fence, err := store.Claim(context.Background(), key, "w", time.Minute)
	require.NoError(t, err)
	return store, fence
}

func ageRunForRetention(t *testing.T, store *AgentRunStore, runID string, age time.Duration) {
	t.Helper()
	past := time.Now().Add(-age).UTC()
	require.NoError(t, store.db.Exec(
		`UPDATE agent_runs SET updated_at = ? WHERE tenant_id = 1 AND run_id = ?`, past, runID,
	).Error)
	require.NoError(t, store.db.Exec(
		`UPDATE agent_run_events SET created_at = ? WHERE tenant_id = 1 AND run_id = ?`, past, runID,
	).Error)
}

// TestAgentRunEventTimeRetentionSnapshotsThenTrims pins the W35 retention
// contract: time-based trimming only deletes unprotected event types of runs
// that finished before the cutoff, and it appends the durable summary event
// in the SAME transaction so a crash can never leave events deleted without
// their snapshot.
func TestAgentRunEventTimeRetentionSnapshotsThenTrims(t *testing.T) {
	store, fence := admitRetentionRun(t, "ret-r1", "s1")
	ctx := context.Background()
	appendEvt := func(evtType string) {
		t.Helper()
		_, err := store.AppendEvent(ctx, fence, agentruntime.RunEvent{
			Type: evtType, Payload: json.RawMessage(`{"n":1}`),
		})
		require.NoError(t, err)
	}
	appendEvt("tool_result")
	appendEvt("tool_result")
	appendEvt("tool_result")
	appendEvt("usage.settled")      // commercial record: own retention policy
	appendEvt("approval.requested") // active approval lane: own retention policy
	require.NoError(t, store.Finalize(ctx, fence, json.RawMessage(`{"content":"done"}`)))
	ageRunForRetention(t, store, "ret-r1", 40*24*time.Hour)

	now := time.Now().Add(time.Hour) // well past every created_at above
	report, err := store.ApplyEventRetention(ctx, EventRetentionOptions{Now: now})
	require.NoError(t, err)
	require.Equal(t, 1, report.RunsConsidered)
	require.Equal(t, 1, report.RunsTrimmed)
	require.EqualValues(t, 3, report.EventsTrimmed)
	require.Equal(t, 1, report.SnapshotsWritten)

	var remaining []agentRunEventRow
	require.NoError(t, store.db.Where("tenant_id=? AND run_id=?", 1, "ret-r1").Order("seq").Find(&remaining).Error)
	types := []string{}
	for _, r := range remaining {
		types = append(types, r.EventType)
	}
	// Only the three tool_result rows are gone; usage/approval/run_completed
	// survive and the snapshot receipt replaces the trimmed prefix.
	require.Equal(t, []string{"usage.settled", "approval.requested", "run_completed", "retention.trimmed"}, types)

	var snapshot agentRunEventRow
	require.NoError(t, store.db.Where("tenant_id=? AND run_id=? AND event_type='retention.trimmed'", 1, "ret-r1").Take(&snapshot).Error)
	var payload struct {
		TrimmedCount int64          `json:"trimmed_count"`
		FirstSeq     int64          `json:"first_seq"`
		LastSeq      int64          `json:"last_seq"`
		TypeCounts   map[string]int `json:"type_counts"`
	}
	require.NoError(t, json.Unmarshal([]byte(snapshot.Payload), &payload))
	require.EqualValues(t, 3, payload.TrimmedCount)
	require.EqualValues(t, 1, payload.FirstSeq)
	require.EqualValues(t, 3, payload.LastSeq)
	require.Equal(t, map[string]int{"tool_result": 3}, payload.TypeCounts)

	// Old cursors over the trimmed prefix still hit the explicit reload error.
	_, err = store.ReadEvents(ctx, fence.RunKey, 0, 10)
	require.ErrorIs(t, err, agentruntime.ErrCursorExpired)

	// A second pass over the same state is a no-op: every remaining old event
	// is protected (usage/approval/run_completed) and the snapshot receipt is
	// itself protected, so the run is no longer even a candidate.
	report, err = store.ApplyEventRetention(ctx, EventRetentionOptions{Now: now})
	require.NoError(t, err)
	require.Equal(t, 0, report.RunsConsidered)
	require.Equal(t, 0, report.RunsTrimmed)
	require.EqualValues(t, 0, report.EventsTrimmed)
	require.Equal(t, 0, report.SnapshotsWritten)
}

// TestAgentRunEventTimeRetentionSkipsActiveAndTombstoned pins the
// coordination with W33 cleanup: live runs are never trimmed, and runs under
// a live (non-purged) deletion tombstone belong to the W33 purge path — the
// time-based retention pass leaves their evidence completely alone.
func TestAgentRunEventTimeRetentionSkipsActiveAndTombstoned(t *testing.T) {
	// runA stays live (never finalized): its events must survive.
	storeA, fenceA := admitRetentionRun(t, "ret-active", "s1")
	_, err := storeA.AppendEvent(context.Background(), fenceA, agentruntime.RunEvent{Type: "tool_result", Payload: json.RawMessage(`{}`)})
	require.NoError(t, err)
	ageRunForRetention(t, storeA, "ret-active", 40*24*time.Hour)

	// runB is terminal and old, but its session carries a live tombstone.
	storeB, fenceB := admitRetentionRun(t, "ret-tomb", "s2")
	_, err = storeB.AppendEvent(context.Background(), fenceB, agentruntime.RunEvent{Type: "tool_result", Payload: json.RawMessage(`{}`)})
	require.NoError(t, err)
	require.NoError(t, storeB.Finalize(context.Background(), fenceB, json.RawMessage(`{"content":"done"}`)))
	ageRunForRetention(t, storeB, "ret-tomb", 40*24*time.Hour)
	require.NoError(t, storeB.db.Exec(
		`INSERT INTO execution_cleanup (tenant_id, owner_id, session_id, deletion_revision, state)
		 VALUES (1, 'u1', 's2', 1, 'tombstoned')`,
	).Error)

	// runC finished but is recent: below the retention horizon.
	storeC, fenceC := admitRetentionRun(t, "ret-recent", "s1")
	_, err = storeC.AppendEvent(context.Background(), fenceC, agentruntime.RunEvent{Type: "tool_result", Payload: json.RawMessage(`{}`)})
	require.NoError(t, err)
	require.NoError(t, storeC.Finalize(context.Background(), fenceC, json.RawMessage(`{"content":"done"}`)))
	// Fresh rows (updated_at = now): not eligible.

	for _, store := range []*AgentRunStore{storeA, storeB, storeC} {
		report, err := store.ApplyEventRetention(context.Background(), EventRetentionOptions{})
		require.NoError(t, err)
		switch report.RunsConsidered {
		case 1: // storeB: considered but tombstone-protected
			require.Equal(t, 1, report.RunsSkippedTombstoned)
			require.Equal(t, 0, report.RunsTrimmed)
		case 0: // storeA (still live) and storeC (recent) are not candidates
			require.Equal(t, 0, report.RunsTrimmed)
		default:
			t.Fatalf("unexpected candidate count %d", report.RunsConsidered)
		}
	}

	var events int64
	require.NoError(t, storeA.db.Table("agent_run_events").Where("tenant_id=? AND run_id=?", 1, "ret-active").Count(&events).Error)
	require.EqualValues(t, 1, events)
	require.NoError(t, storeB.db.Table("agent_run_events").Where("tenant_id=? AND run_id=?", 1, "ret-tomb").Count(&events).Error)
	require.EqualValues(t, 2, events) // tool_result + run_completed untouched
	require.NoError(t, storeC.db.Table("agent_run_events").Where("tenant_id=? AND run_id=?", 1, "ret-recent").Count(&events).Error)
	require.EqualValues(t, 2, events)

	// A negative retention window is a configuration error, not a trim-all.
	_, err = storeA.ApplyEventRetention(context.Background(), EventRetentionOptions{Retention: -time.Hour})
	require.ErrorIs(t, err, agentruntime.ErrConflict)
}
