package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIngestSourceEventDeduplicatesAndPreservesUnknown(t *testing.T) {
	db := openRunTestDB(t)
	_, err := NewAgentRunStore(db).Admit(context.Background(), testAdmission())
	require.NoError(t, err)
	require.NoError(t, db.Exec(`INSERT INTO execution_dispatches (tenant_id, command_id, run_id, attempt_id, payload_hash, state, worker, epoch) VALUES (1, 'binding-1', 'r1', 'a1', '', 'completed', 'w', 1)`).Error)
	store := NewExecutionObservationStore(db)
	source := SourceObservation{BindingID: "binding-1", Generation: "g1", EventID: "e1", AttemptID: "a1", Type: "future.event", Payload: json.RawMessage(`{"x":1}`)}
	first, err := store.IngestSourceEvent(context.Background(), source.BindingID, source)
	require.NoError(t, err)
	require.Equal(t, int64(1), first.Seq)
	second, err := store.IngestSourceEvent(context.Background(), source.BindingID, source)
	require.NoError(t, err)
	require.Equal(t, first.Seq, second.Seq)
	require.Equal(t, "unknown", first.Type)
	var count int64
	require.NoError(t, db.Table("agent_run_events").Where("tenant_id = 1 AND run_id = 'r1'").Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestIngestSourceEventRejectsPayloadConflictAndIsolatesTenant(t *testing.T) {
	db := openRunTestDB(t)
	_, err := NewAgentRunStore(db).Admit(context.Background(), testAdmission())
	require.NoError(t, err)
	require.NoError(t, db.Exec(`INSERT INTO execution_dispatches (tenant_id, command_id, run_id, attempt_id, payload_hash, state, worker, epoch) VALUES (1, 'binding-1', 'r1', 'a1', '', 'completed', 'w', 1)`).Error)
	store := NewExecutionObservationStore(db)
	source := SourceObservation{BindingID: "binding-1", Generation: "g1", EventID: "e1", Type: "text.delta", Payload: json.RawMessage(`{"text":"one"}`)}
	require.NoError(t, func() error {
		_, e := store.IngestSourceEvent(context.Background(), source.BindingID, source)
		return e
	}())
	source.Payload = json.RawMessage(`{"text":"two"}`)
	require.ErrorIs(t, func() error {
		_, e := store.IngestSourceEvent(context.Background(), source.BindingID, source)
		return e
	}(), ErrSourceConflict)
	require.Error(t, func() error { _, e := store.IngestSourceEvent(context.Background(), "missing", source); return e }())
}
