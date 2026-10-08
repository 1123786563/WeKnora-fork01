package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNormalizeMode pins the mode normalization contract ported from the
// legacy types.NormalizeQueryHistoryMode: empty/unknown input can never
// disable or de-identify workspace history — it falls back to normal.
func TestNormalizeMode(t *testing.T) {
	assert.Equal(t, Normal, NormalizeMode("unknown"))
	assert.Equal(t, Anonymized, NormalizeMode(" anonymized "))

	assert.Equal(t, Normal, NormalizeMode(""))
	assert.Equal(t, Normal, NormalizeMode("   "))
	assert.Equal(t, Normal, NormalizeMode("bogus"))
	assert.Equal(t, Normal, NormalizeMode("NORMAL"))
	assert.Equal(t, Normal, NormalizeMode(" normal "))
	assert.Equal(t, Normal, NormalizeMode("normal"))
	assert.Equal(t, Anonymized, NormalizeMode("anonymized"))
	assert.Equal(t, Disabled, NormalizeMode("disabled"))
}

// TestModeConstants pins the on-the-wire and in-column mode values.
func TestModeConstants(t *testing.T) {
	assert.Equal(t, "normal", Normal)
	assert.Equal(t, "anonymized", Anonymized)
	assert.Equal(t, "disabled", Disabled)
}

// TestConfigNormalize keeps the nil receiver a no-op (a nil config stays
// "unconfigured" rather than gaining a default) and canonicalizes the mode.
func TestConfigNormalize(t *testing.T) {
	var nilConfig *Config
	assert.NotPanics(t, func() { nilConfig.Normalize() })
	assert.Nil(t, nilConfig)

	trimmed := &Config{Mode: "  anonymized "}
	trimmed.Normalize()
	assert.Equal(t, Anonymized, trimmed.Mode)

	unknown := &Config{Mode: "bogus"}
	unknown.Normalize()
	assert.Equal(t, Normal, unknown.Mode)
}

// TestConfigValueScanRoundTrip covers the driver.Valuer / sql.Scanner pair
// GORM uses for the tenants.query_history_config JSONB column.
func TestConfigValueScanRoundTrip(t *testing.T) {
	cfg := &Config{Mode: Disabled}
	value, err := cfg.Value()
	require.NoError(t, err)
	raw, ok := value.([]byte)
	require.True(t, ok, "Value must serialize to JSON bytes")
	assert.JSONEq(t, `{"mode":"disabled"}`, string(raw))

	var scanned Config
	require.NoError(t, scanned.Scan(raw))
	assert.Equal(t, Disabled, scanned.Mode)

	var nilConfig *Config
	value, err = nilConfig.Value()
	require.NoError(t, err)
	assert.Nil(t, value, "a nil config must persist as SQL NULL")

	// Scanning NULL leaves the receiver untouched.
	before := Config{Mode: Anonymized}
	require.NoError(t, before.Scan(nil))
	assert.Equal(t, Anonymized, before.Mode)

	// Non-[]byte driver values are tolerated as no-ops (legacy contract).
	require.NoError(t, before.Scan("not-bytes"))
	assert.Equal(t, Anonymized, before.Mode)
}

// TestConfigJSONTagStable pins the exact serialized shape: one lowercase
// "mode" key, byte-identical to the legacy types.QueryHistoryConfig.
func TestConfigJSONTagStable(t *testing.T) {
	data, err := json.Marshal(&Config{Mode: Anonymized})
	require.NoError(t, err)
	assert.Equal(t, `{"mode":"anonymized"}`, string(data))
}

// TestExportStatusConstants pins the export-job lifecycle statuses.
func TestExportStatusConstants(t *testing.T) {
	assert.Equal(t, "pending", ExportPending)
	assert.Equal(t, "running", ExportRunning)
	assert.Equal(t, "done", ExportDone)
	assert.Equal(t, "failed", ExportFailed)
}

// TestExportJobTableNameStable pins the GORM table name; the admin tooling
// and the migration chain both reference it positionally.
func TestExportJobTableNameStable(t *testing.T) {
	assert.Equal(t, "query_history_export_jobs", ExportJob{}.TableName())
}

// TestExportJobPendingDefault documents that the domain zero value carries no
// status: defaulting to pending happens at the store boundary (and via the
// column default 'pending'), which the repository smoke tests keep pinning.
func TestExportJobPendingDefault(t *testing.T) {
	var job ExportJob
	assert.Equal(t, ExportStatus(""), job.Status)
	assert.Empty(t, job.FilePath)
	assert.Empty(t, job.ErrorMessage)
}

// TestExportJobJSONRoundTrip pins the admin-facing JSON contract: exact keys,
// exact order, RFC3339 timestamps.
func TestExportJobJSONRoundTrip(t *testing.T) {
	stamp := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	job := ExportJob{
		ID:           7,
		TenantID:     3,
		RequestedBy:  "u1",
		Status:       ExportRunning,
		FilePath:     "",
		ErrorMessage: "boom",
		CreatedAt:    stamp,
		UpdatedAt:    stamp,
	}
	data, err := json.Marshal(job)
	require.NoError(t, err)
	assert.Equal(t,
		`{"id":7,"tenant_id":3,"requested_by":"u1","status":"running","file_path":"","error_message":"boom","created_at":"2026-09-21T10:00:00Z","updated_at":"2026-09-21T10:00:00Z"}`,
		string(data))

	var got ExportJob
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, job, got)
}

// TestExportRowJSONRoundTrip pins the CSV-source row serialization: the
// aggregated per-session fields keep their snake_case keys.
func TestExportRowJSONRoundTrip(t *testing.T) {
	stamp := time.Date(2026, 9, 21, 9, 30, 0, 0, time.UTC)
	row := ExportRow{
		SessionID:    "s1",
		Title:        "Quarterly review",
		UserID:       "u1",
		Source:       "web",
		EngineType:   "builtin",
		CreatedAt:    stamp,
		UpdatedAt:    stamp,
		MessageCount: 12,
		LikeCount:    3,
		DislikeCount: 1,
	}
	data, err := json.Marshal(row)
	require.NoError(t, err)
	assert.Equal(t,
		`{"session_id":"s1","title":"Quarterly review","user_id":"u1","source":"web","engine_type":"builtin","created_at":"2026-09-21T09:30:00Z","updated_at":"2026-09-21T09:30:00Z","message_count":12,"like_count":3,"dislike_count":1}`,
		string(data))

	var got ExportRow
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, row, got)
}

// TestExportPayloadJSONMirrorsLegacyWire pins the async task wire format:
// keys, order, and omitempty behavior are byte-identical to the legacy
// types.QueryHistoryExportPayload (flat tracing fields included) so payloads
// enqueued before the module migration keep parsing in the new worker.
func TestExportPayloadJSONMirrorsLegacyWire(t *testing.T) {
	payload := ExportPayload{
		LangfuseTraceID:             "trace-1",
		LangfuseParentObservationID: "obs-1",
		LangfuseTraceparent:         "00-trace-1-span-1-01",
		JobID:                       5,
		TenantID:                    2,
		RequestedBy:                 "u1",
		UserID:                      "u2",
		StartTimeMs:                 1000,
		EndTimeMs:                   2000,
		FeedbackRating:              "like",
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	assert.Equal(t,
		`{"lf_trace_id":"trace-1","lf_parent_obs_id":"obs-1","lf_traceparent":"00-trace-1-span-1-01","job_id":5,"tenant_id":2,"requested_by":"u1","user_id":"u2","start_time_ms":1000,"end_time_ms":2000,"feedback_rating":"like"}`,
		string(data))

	// omitempty: an empty payload collapses to the identifying ids only.
	minimal, err := json.Marshal(ExportPayload{JobID: 5, TenantID: 2})
	require.NoError(t, err)
	assert.Equal(t, `{"job_id":5,"tenant_id":2}`, string(minimal))

	var got ExportPayload
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, payload, got)
}
