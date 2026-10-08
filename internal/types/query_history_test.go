package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time proof that the Wave 1 legacy names alias the moved domain
// types (same underlying types; Wave 5 removes this seam).
var (
	_ domain.Config       = QueryHistoryConfig{}
	_ domain.ExportJob    = QueryHistoryExportJob{}
	_ domain.ExportRow    = QueryHistoryExportRow{}
	_ func(string) string = NormalizeQueryHistoryMode
)

// TestQueryHistoryModeAliases pins the mode constants' values and their
// identity with the domain constants.
func TestQueryHistoryModeAliases(t *testing.T) {
	assert.Equal(t, "normal", QueryHistoryModeNormal)
	assert.Equal(t, "anonymized", QueryHistoryModeAnonymized)
	assert.Equal(t, "disabled", QueryHistoryModeDisabled)
	assert.Equal(t, domain.Normal, QueryHistoryModeNormal)
	assert.Equal(t, domain.Anonymized, QueryHistoryModeAnonymized)
	assert.Equal(t, domain.Disabled, QueryHistoryModeDisabled)
}

// TestNormalizeQueryHistoryModeCompat ports the legacy normalization
// expectations: empty/unknown falls back to normal, recognized modes pass
// through trimmed.
func TestNormalizeQueryHistoryModeCompat(t *testing.T) {
	assert.Equal(t, QueryHistoryModeNormal, NormalizeQueryHistoryMode(""))
	assert.Equal(t, QueryHistoryModeNormal, NormalizeQueryHistoryMode("bogus"))
	assert.Equal(t, QueryHistoryModeNormal, NormalizeQueryHistoryMode(" normal "))
	assert.Equal(t, QueryHistoryModeNormal, NormalizeQueryHistoryMode("normal"))
	assert.Equal(t, QueryHistoryModeAnonymized, NormalizeQueryHistoryMode(" anonymized "))
	assert.Equal(t, QueryHistoryModeDisabled, NormalizeQueryHistoryMode("disabled"))
}

// TestQueryHistoryConfigCompat keeps the legacy config behavior byte-stable
// through the alias: nil-safe Normalize, JSON shape, GORM value/scan.
func TestQueryHistoryConfigCompat(t *testing.T) {
	var nilConfig *QueryHistoryConfig
	assert.NotPanics(t, func() { nilConfig.Normalize() })

	cfg := &QueryHistoryConfig{Mode: " bogus "}
	cfg.Normalize()
	assert.Equal(t, QueryHistoryModeNormal, cfg.Mode)

	data, err := json.Marshal(&QueryHistoryConfig{Mode: QueryHistoryModeDisabled})
	require.NoError(t, err)
	assert.Equal(t, `{"mode":"disabled"}`, string(data))

	value, err := cfg.Value()
	require.NoError(t, err)
	raw, ok := value.([]byte)
	require.True(t, ok)
	var scanned QueryHistoryConfig
	require.NoError(t, scanned.Scan(raw))
	assert.Equal(t, QueryHistoryModeNormal, scanned.Mode)

	var nilPtr *QueryHistoryConfig
	value, err = nilPtr.Value()
	require.NoError(t, err)
	assert.Nil(t, value)
	require.NoError(t, scanned.Scan(nil))
	require.NoError(t, scanned.Scan("not-bytes"))
}

// TestQueryHistoryExportStatusAliases pins the export lifecycle statuses.
func TestQueryHistoryExportStatusAliases(t *testing.T) {
	assert.Equal(t, "pending", QueryHistoryExportPending)
	assert.Equal(t, "running", QueryHistoryExportRunning)
	assert.Equal(t, "done", QueryHistoryExportDone)
	assert.Equal(t, "failed", QueryHistoryExportFailed)
	assert.Equal(t, domain.ExportPending, QueryHistoryExportPending)
	assert.Equal(t, domain.ExportDone, QueryHistoryExportDone)
}

// TestQueryHistoryExportJobCompat pins the job alias: stable table name and
// unchanged admin-facing JSON keys.
func TestQueryHistoryExportJobCompat(t *testing.T) {
	job := QueryHistoryExportJob{
		ID:          7,
		TenantID:    3,
		RequestedBy: "u1",
		Status:      QueryHistoryExportPending,
	}
	assert.Equal(t, "query_history_export_jobs", job.TableName())

	data, err := json.Marshal(job)
	require.NoError(t, err)
	assert.Equal(t,
		`{"id":7,"tenant_id":3,"requested_by":"u1","status":"pending","file_path":"","error_message":"","created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z"}`,
		string(data))
}

// TestQueryHistoryExportRowCompat pins the export row alias: the aggregated
// per-session fields keep their snake_case JSON keys.
func TestQueryHistoryExportRowCompat(t *testing.T) {
	row := QueryHistoryExportRow{SessionID: "s1", MessageCount: 2}
	data, err := json.Marshal(row)
	require.NoError(t, err)
	assert.Equal(t,
		`{"session_id":"s1","title":"","user_id":"","source":"","engine_type":"","created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z","message_count":2,"like_count":0,"dislike_count":0}`,
		string(data))
}

// TestQueryHistorySnapshotsStayLocal pins that the snapshot DTOs remain in
// this package with unchanged JSON tags (they are the Wave 5 seam).
func TestQueryHistorySnapshotsStayLocal(t *testing.T) {
	snapshot := QueryHistorySnapshot{Truncated: true}
	data, err := json.Marshal(&snapshot)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"messages":null`)
	assert.Contains(t, string(data), `"feedback":null`)
	assert.Contains(t, string(data), `"truncated":true`)

	shared := SharedSessionSnapshot{Truncated: false}
	data, err = json.Marshal(&shared)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"messages":null`)
	assert.Contains(t, string(data), `"truncated":false`)
	assert.NotContains(t, string(data), "feedback", "the shared snapshot deliberately carries no feedback rows")

	// The snapshot types remain assignable to the port seam's requirement.
	var _ = time.Time(snapshot.Session.CreatedAt)
}
