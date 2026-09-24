package appconnector

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrNotionVersionConflict: the external page's current version (Notion
// last_edited_time) no longer equals the version the approved update plan
// was formed against. The update is refused BEFORE any write request —
// per CONTEXT.md「外部发布」: 再次更新前必须读取外部当前版本并形成新的
// 候选变更.
var ErrNotionVersionConflict = errors.New("notion_version_conflict")

// NotionUpdateSnapshot is the A03-approved argument snapshot for updating
// ONE existing external page: exactly page_id, expected_version (the
// last_edited_time the plan read before approval — an immutable approval
// anchor), title and blocks. Every dispatched request is built FROM these
// fields; nothing is added, rewritten or re-derived after approval.
type NotionUpdateSnapshot struct {
	PageID          string
	ExpectedVersion string
	Title           string
	Blocks          []json.RawMessage
}

// ParseNotionUpdateSnapshot validates that args are EXACTLY the approved
// four-field update snapshot: no extra fields (approve-then-rewrite), no
// missing fields, non-empty page id / expected version / title, and blocks
// that are each valid JSON (normalized so the wire bytes are the approved
// bytes). Blocks may be empty (a title-only update).
func ParseNotionUpdateSnapshot(args json.RawMessage) (NotionUpdateSnapshot, error) {
	var s NotionUpdateSnapshot
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.UseNumber()
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return s, fmt.Errorf("%w: %v", ErrNotionSnapshotInvalid, err)
	}
	if len(raw) != 4 {
		return s, fmt.Errorf("%w: snapshot must be exactly page_id, expected_version, title, blocks", ErrNotionSnapshotInvalid)
	}
	if err := json.Unmarshal(raw["page_id"], &s.PageID); err != nil {
		return s, fmt.Errorf("%w: page_id: %v", ErrNotionSnapshotInvalid, err)
	}
	if err := json.Unmarshal(raw["expected_version"], &s.ExpectedVersion); err != nil {
		return s, fmt.Errorf("%w: expected_version: %v", ErrNotionSnapshotInvalid, err)
	}
	if err := json.Unmarshal(raw["title"], &s.Title); err != nil {
		return s, fmt.Errorf("%w: title: %v", ErrNotionSnapshotInvalid, err)
	}
	if s.PageID == "" {
		return s, fmt.Errorf("%w: empty page_id", ErrNotionSnapshotInvalid)
	}
	if s.ExpectedVersion == "" {
		return s, fmt.Errorf("%w: empty expected_version", ErrNotionSnapshotInvalid)
	}
	if s.Title == "" {
		return s, fmt.Errorf("%w: empty title", ErrNotionSnapshotInvalid)
	}
	var blocks []json.RawMessage
	if err := json.Unmarshal(raw["blocks"], &blocks); err != nil {
		return s, fmt.Errorf("%w: blocks: %v", ErrNotionSnapshotInvalid, err)
	}
	s.Blocks = make([]json.RawMessage, 0, len(blocks))
	for i, b := range blocks {
		n, err := NormalizeArgs(b)
		if err != nil {
			return s, fmt.Errorf("%w: block %d: %v", ErrNotionSnapshotInvalid, i, err)
		}
		s.Blocks = append(s.Blocks, n)
	}
	return s, nil
}

// IsNotionUpdateArgs reports whether args carry the update snapshot's
// distinguishing key pair (page_id AND expected_version). It never parses
// the full snapshot — the adapter family uses it only to route an approved
// action to the update adapter; full validation still happens in
// ParseNotionUpdateSnapshot.
func IsNotionUpdateArgs(args json.RawMessage) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(args, &raw); err != nil {
		return false
	}
	_, hasPage := raw["page_id"]
	_, hasVersion := raw["expected_version"]
	return hasPage && hasVersion
}

// DetectNotionVersionConflict compares the approved expected version with
// the version just read from the provider. Anything but an exact match —
// including an unreadable empty side — is a conflict; an unobservable
// remote state must never authorize an overwrite.
func DetectNotionVersionConflict(expected, actual string) error {
	if expected == "" || actual == "" || expected != actual {
		return fmt.Errorf("%w: approved %q but remote has %q", ErrNotionVersionConflict, expected, actual)
	}
	return nil
}

// NotionPageVersion is the reliable read shape of GET /v1/pages/{id} for
// version purposes: the exact page id and its last_edited_time. Notion's
// page object carries last_edited_time as an ISO-8601 string that changes
// on every content/property edit — the external collaboration authority's
// version token (spec: "After publication, the external document is the
// collaboration authority").
type NotionPageVersion struct {
	PageID         string
	LastEditedTime string
}

type notionPageVersionObject struct {
	Object         string `json:"object"`
	ID             string `json:"id"`
	LastEditedTime string `json:"last_edited_time"`
}

// ParseNotionPageVersion extracts the page identity + current version from
// a GET /v1/pages/{id} reply. A reply without a real id or a real
// last_edited_time is an error — a fabricated version is never a basis for
// conflict detection or a receipt.
func ParseNotionPageVersion(raw []byte) (NotionPageVersion, error) {
	var p notionPageVersionObject
	if err := json.Unmarshal(raw, &p); err != nil {
		return NotionPageVersion{}, fmt.Errorf("notion_page_version_unparseable: %v", err)
	}
	if p.ID == "" || p.LastEditedTime == "" {
		return NotionPageVersion{}, fmt.Errorf("notion_page_version_unparseable: no id or last_edited_time")
	}
	return NotionPageVersion{PageID: p.ID, LastEditedTime: p.LastEditedTime}, nil
}

// NotionPageReceipt is the persisted external receipt of one publish: the
// provider page id and the version the publish itself produced (read back
// from the provider's own reply — never fabricated locally).
type NotionPageReceipt struct {
	ExternalID      string
	ExternalVersion string
}

// ParseNotionPageReceipt extracts the receipt fields from a provider page
// payload (the create/update reply the adapter recorded as the action's
// output evidence).
func ParseNotionPageReceipt(raw []byte) (NotionPageReceipt, error) {
	v, err := ParseNotionPageVersion(raw)
	if err != nil {
		return NotionPageReceipt{}, err
	}
	return NotionPageReceipt{ExternalID: v.PageID, ExternalVersion: v.LastEditedTime}, nil
}
