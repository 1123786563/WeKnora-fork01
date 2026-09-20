package types

import (
	"encoding/json"
	"testing"
)

// SyncResult.Errors is persisted as jsonb and read back when the sync-log drawer
// is opened. Historically each entry was a plain string; it is now a structured
// SyncItemError (title + i18n code + params + fallback message) so the frontend
// can localise it. Old rows must still deserialize, so a bare string decodes into
// the Message field.
func TestSyncItemError_UnmarshalAcceptsLegacyStringAndObject(t *testing.T) {
	// Legacy format: array of plain strings.
	var legacy []SyncItemError
	if err := json.Unmarshal([]byte(`["季度报告: export failed"]`), &legacy); err != nil {
		t.Fatalf("legacy string form must still decode: %v", err)
	}
	if len(legacy) != 1 || legacy[0].Message != "季度报告: export failed" {
		t.Fatalf("legacy string should map to Message, got %+v", legacy)
	}
	if legacy[0].Code != "" {
		t.Errorf("legacy string must not invent a code, got %q", legacy[0].Code)
	}

	// New format: structured object round-trips.
	in := SyncItemError{
		Title:   "季度报告",
		Code:    "feishu_api_error",
		Params:  map[string]string{"code": "1663"},
		Message: "Feishu API error (code=1663); will retry",
	}
	b, err := json.Marshal([]SyncItemError{in})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out []SyncItemError
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal structured: %v", err)
	}
	if len(out) != 1 || out[0].Code != "feishu_api_error" || out[0].Params["code"] != "1663" || out[0].Title != "季度报告" {
		t.Fatalf("structured form did not round-trip: %+v", out)
	}
}

// ExternalID (SP2 targeted reindex) lets a failure sample be retried
// individually. Rows persisted before the field existed must keep decoding
// with a zero ExternalID, and the field must round-trip when set.
func TestSyncItemError_ExternalIDLegacyCompatAndRoundTrip(t *testing.T) {
	// Pre-external_id structured row: missing field decodes to the zero value.
	var old []SyncItemError
	legacy := `[{"title":"季度报告","code":"feishu_api_error","message":"boom"}]`
	if err := json.Unmarshal([]byte(legacy), &old); err != nil {
		t.Fatalf("old object without external_id must decode: %v", err)
	}
	if len(old) != 1 || old[0].ExternalID != "" {
		t.Fatalf("missing external_id must decode to the zero value, got %+v", old)
	}

	// New row round-trips the external id so the UI can offer a targeted retry.
	b, err := json.Marshal(SyncItemError{Title: "季度报告", Code: "ingest_failed", ExternalID: "nt-123"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out SyncItemError
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.ExternalID != "nt-123" {
		t.Fatalf("external_id did not round-trip, got %q", out.ExternalID)
	}

	// omitempty keeps unset samples byte-identical to the legacy shape.
	bEmpty, err := json.Marshal(SyncItemError{Title: "t"})
	if err != nil {
		t.Fatalf("marshal empty: %v", err)
	}
	if string(bEmpty) != `{"title":"t"}` {
		t.Fatalf("unset external_id must be omitted, got %s", bEmpty)
	}
}
