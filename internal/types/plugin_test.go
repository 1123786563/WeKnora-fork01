package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPluginPreviewToolsScanAcceptsDriverTextForms (OCR T01-ocr-r2-011):
// tools_snapshot is JSONB on PostgreSQL (driver returns []byte) but TEXT on
// the SQLite twin (driver returns string) — Scan must populate the snapshot
// from BOTH forms; an unexpected type is an error, never silently swallowed
// (the same contract as types.JSON.Scan, which the previous implementation
// deviated from by returning nil on any non-[]byte value).
func TestPluginPreviewToolsScanAcceptsDriverTextForms(t *testing.T) {
	const raw = `[{"name":"search_my_week_issues","input_schema_digest":"abc"}]`

	var fromBytes PluginPreviewTools
	require.NoError(t, fromBytes.Scan([]byte(raw)))
	require.Len(t, fromBytes, 1)
	require.Equal(t, "search_my_week_issues", fromBytes[0].Name)

	var fromString PluginPreviewTools
	require.NoError(t, fromString.Scan(raw))
	require.Len(t, fromString, 1)
	require.Equal(t, "search_my_week_issues", fromString[0].Name)

	var fromInt PluginPreviewTools
	require.Error(t, fromInt.Scan(42))

	var fromEmpty PluginPreviewTools
	require.NoError(t, fromEmpty.Scan(""))
	require.Nil(t, fromEmpty)
}
