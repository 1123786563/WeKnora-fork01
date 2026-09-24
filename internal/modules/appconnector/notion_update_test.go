package appconnector

import (
	"encoding/json"
	"errors"
	"testing"
)

func updateArgs(pageID, version, title string, blocks []any) json.RawMessage {
	if blocks == nil {
		blocks = []any{paragraphBlock("hello")}
	}
	raw, _ := json.Marshal(map[string]any{
		"page_id":          pageID,
		"expected_version": version,
		"title":            title,
		"blocks":           blocks,
	})
	return raw
}

func paragraphBlock(text string) map[string]any {
	return map[string]any{
		"object": "block",
		"type":   "paragraph",
		"paragraph": map[string]any{
			"rich_text": []any{map[string]any{
				"type": "text",
				"text": map[string]string{"content": text},
			}},
		},
	}
}

func TestParseNotionUpdateSnapshotAcceptsExactFourFields(t *testing.T) {
	snap, err := ParseNotionUpdateSnapshot(updateArgs("page-1", "2026-09-24T10:00:00.000Z", "T", nil))
	if err != nil {
		t.Fatal(err)
	}
	if snap.PageID != "page-1" || snap.ExpectedVersion != "2026-09-24T10:00:00.000Z" || snap.Title != "T" || len(snap.Blocks) != 1 {
		t.Fatalf("snapshot fields drift: %+v", snap)
	}
}

func TestParseNotionUpdateSnapshotRejectsExtraField(t *testing.T) {
	raw := append([]byte{}, updateArgs("page-1", "v", "T", nil)...)
	raw = raw[:len(raw)-1]
	raw = append(raw, []byte(`,"evil":"x"}`)...)
	if _, err := ParseNotionUpdateSnapshot(raw); !errors.Is(err, ErrNotionSnapshotInvalid) {
		t.Fatalf("approve-then-rewrite must be refused, got %v", err)
	}
}

func TestParseNotionUpdateSnapshotRejectsMissingOrEmpty(t *testing.T) {
	cases := map[string]json.RawMessage{
		"missing page":    []byte(`{"expected_version":"v","title":"T","blocks":[]}`),
		"missing version": []byte(`{"page_id":"p","title":"T","blocks":[]}`),
		"empty page":      updateArgs("", "v", "T", nil),
		"empty version":   updateArgs("p", "", "T", nil),
		"empty title":     updateArgs("p", "v", "", nil),
		"not an object":   []byte(`["page_id"]`),
		"three fields":    []byte(`{"page_id":"p","title":"T","blocks":[]}`),
	}
	for name, raw := range cases {
		if _, err := ParseNotionUpdateSnapshot(raw); !errors.Is(err, ErrNotionSnapshotInvalid) {
			t.Fatalf("%s: want ErrNotionSnapshotInvalid, got %v", name, err)
		}
	}
}

func TestIsNotionUpdateArgs(t *testing.T) {
	if !IsNotionUpdateArgs(updateArgs("p", "v", "T", nil)) {
		t.Fatal("update-shaped args must be detected")
	}
	create, _ := json.Marshal(map[string]any{"parent": "pp", "title": "T", "blocks": []any{}})
	if IsNotionUpdateArgs(create) {
		t.Fatal("create-shaped args must not be treated as update")
	}
	if IsNotionUpdateArgs([]byte(`not json`)) {
		t.Fatal("garbage must not be update")
	}
}

func TestDetectNotionVersionConflict(t *testing.T) {
	if err := DetectNotionVersionConflict("v1", "v1"); err != nil {
		t.Fatalf("matching version must pass: %v", err)
	}
	if err := DetectNotionVersionConflict("v1", "v2"); !errors.Is(err, ErrNotionVersionConflict) {
		t.Fatalf("drift must conflict, got %v", err)
	}
	if err := DetectNotionVersionConflict("", "v2"); !errors.Is(err, ErrNotionVersionConflict) {
		t.Fatalf("empty expected must fail closed, got %v", err)
	}
	if err := DetectNotionVersionConflict("v1", ""); !errors.Is(err, ErrNotionVersionConflict) {
		t.Fatalf("unreadable remote must fail closed, got %v", err)
	}
}

func TestParseNotionPageVersion(t *testing.T) {
	v, err := ParseNotionPageVersion([]byte(`{"object":"page","id":"p1","last_edited_time":"2026-09-24T10:00:00.000Z","parent":{"type":"page_id","page_id":"pp"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if v.PageID != "p1" || v.LastEditedTime != "2026-09-24T10:00:00.000Z" {
		t.Fatalf("version fields drift: %+v", v)
	}
	if _, err := ParseNotionPageVersion([]byte(`{"object":"page","id":"p1"}`)); err == nil {
		t.Fatal("missing last_edited_time must be an error, never a fabricated version")
	}
	if _, err := ParseNotionPageVersion([]byte(`{"object":"page","last_edited_time":"v"}`)); err == nil {
		t.Fatal("missing id must be an error")
	}
}

func TestParseNotionPageReceipt(t *testing.T) {
	r, err := ParseNotionPageReceipt([]byte(`{"object":"page","id":"p1","last_edited_time":"v9"}`))
	if err != nil {
		t.Fatal(err)
	}
	if r.ExternalID != "p1" || r.ExternalVersion != "v9" {
		t.Fatalf("receipt fields drift: %+v", r)
	}
	if _, err := ParseNotionPageReceipt([]byte(`{"object":"page","id":""}`)); err == nil {
		t.Fatal("no real page id must refuse a receipt")
	}
}
