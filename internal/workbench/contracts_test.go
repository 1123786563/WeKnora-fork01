package workbench

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExecutionWire(t *testing.T) {
	fixture := []byte(`{"schema_version":1,"run_id":"r","attempt_id":"a","seq":1,"type":"future.event","occurred_at":"2026-09-12T00:00:00Z","payload":{"delta":"ok"}}`)
	event, err := ParseExecutionEvent(fixture)
	require.NoError(t, err)
	require.Equal(t, int64(1), event.Seq)
	require.Equal(t, "future.event", event.Type)

	encoded, err := json.Marshal(event)
	require.NoError(t, err)
	var roundTrip map[string]any
	require.NoError(t, json.Unmarshal(encoded, &roundTrip))
	require.Equal(t, float64(1), roundTrip["seq"])
	require.Equal(t, "future.event", roundTrip["type"])
}

func TestExecutionWireRejectsInvalidValues(t *testing.T) {
	base := ExecutionEvent{
		SchemaVersion: 1,
		RunID:         "r",
		AttemptID:     "",
		Seq:           1,
		Type:          "future.event",
		OccurredAt:    "2026-09-12T00:00:00Z",
		Payload:       json.RawMessage(`{}`),
	}
	require.NoError(t, base.Validate(), "legacy lifecycle events may have an empty attempt id")
	for _, event := range []ExecutionEvent{
		{SchemaVersion: 2, RunID: "r", Seq: 1, Type: "event", OccurredAt: base.OccurredAt, Payload: base.Payload},
		{SchemaVersion: 1, RunID: "r", Seq: 0, Type: "event", OccurredAt: base.OccurredAt, Payload: base.Payload},
		{SchemaVersion: 1, RunID: "r", Seq: 1, Type: "event", OccurredAt: "not-a-date", Payload: base.Payload},
		{SchemaVersion: 1, RunID: "r", Seq: 1, Type: "event", OccurredAt: base.OccurredAt, Payload: json.RawMessage(`[]`)},
	} {
		require.Error(t, event.Validate())
	}
	for _, fixture := range []string{
		`{"schema_version":1,"run_id":"r","seq":1,"type":"event","occurred_at":"2026-09-12T00:00:00Z","payload":{}}`,
		`{"schema_version":1,"run_id":"r","attempt_id":null,"seq":1,"type":"event","occurred_at":"2026-09-12T00:00:00Z","payload":{}}`,
		`{"schema_version":1,"run_id":"r","attempt_id":1,"seq":1,"type":"event","occurred_at":"2026-09-12T00:00:00Z","payload":{}}`,
		`{"schema_version":1,"run_id":"r","attempt_id":"","seq":1,"type":"event","occurred_at":"2026-02-30T00:00:00Z","payload":{}}`,
		`{"schema_version":1,"run_id":"r","attempt_id":"","seq":1,"type":"event","occurred_at":"2026-09-12T00:00:00+24:00","payload":{}}`,
	} {
		_, err := ParseExecutionEvent([]byte(fixture))
		require.Error(t, err)
	}
	validFractional, err := ParseExecutionEvent([]byte(`{"schema_version":1,"run_id":"r","attempt_id":"","seq":1,"type":"event","occurred_at":"2026-09-12T00:00:00.123+08:00","payload":{}}`))
	require.NoError(t, err)
	require.Equal(t, "2026-09-12T00:00:00.123+08:00", validFractional.OccurredAt)
	for _, year := range []string{"0000", "0099", "0100"} {
		fixture := `{"schema_version":1,"run_id":"r","attempt_id":"","seq":1,"type":"event","occurred_at":"` + year + `-01-02T03:04:05Z","payload":{}}`
		event, err := ParseExecutionEvent([]byte(fixture))
		require.NoError(t, err)
		require.Equal(t, year+"-01-02T03:04:05Z", event.OccurredAt)
	}
	overflow := base
	overflow.Seq = MaxSafeInteger + 1
	require.ErrorIs(t, overflow.Validate(), ErrSequenceOverflow)
}

func TestExecutionDTOAndSnapshot(t *testing.T) {
	dto := ExecutionDTO{
		SchemaVersion: 1, RunID: "r", SessionID: "s", Driver: "platform",
		Revision: 0, RunStatus: "queued", ExecutionStatus: "idle", SettlementStatus: "unsettled",
		Capabilities: map[string]Capability{
			"text":  {State: CapabilitySupported},
			"voice": {State: CapabilityUnavailable, Reason: "not configured"},
		},
	}
	require.NoError(t, dto.Validate())
	snapshot := ExecutionSnapshot{Execution: dto, Watermark: 1, Events: []ExecutionEvent{{SchemaVersion: 1, RunID: "r", Seq: 1, Type: "future.event", OccurredAt: "2026-09-12T00:00:00Z", Payload: json.RawMessage(`{}`)}}}
	require.NoError(t, snapshot.Validate())
	snapshot.Events[0].RunID = "other"
	require.Error(t, snapshot.Validate())
	badCapability := dto
	badCapability.Capabilities = map[string]Capability{"voice": {State: CapabilityForbidden}}
	require.Error(t, badCapability.Validate())
}
