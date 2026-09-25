package appconnector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
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

// notionUpdatePageRequest is the wire body of the title PATCH.
type notionUpdatePageRequest struct {
	Properties notionCreatePageProperties `json:"properties"`
}

// NotionUpdateAdapter executes ONE approved page update against the same
// NO-01 reviewed contract family as NotionCreateAdapter: the update is a
// multi-step write — read the current version, PATCH the title, append the
// approved blocks in batches — persisting the block range between steps.
// The version pre-read is the publish conflict gate: the remote
// last_edited_time must still equal the snapshot's approved
// expected_version, otherwise the update is refused with
// ErrNotionVersionConflict and ZERO write requests leave the process.
//
// Outcome semantics (identical to create):
//   - pre-read failures are definitive FAILED (a GET cannot have produced
//     the write; nothing left the process);
//   - every write-step transport failure / 5xx / unparseable reply is
//     ErrNotionOutcomeUnknown — the effect may exist remotely;
//   - Query reconciles ONLY via the reliable page read + children read.
type NotionUpdateAdapter struct {
	// Policy is the admin-reviewed outbound contract (A04).
	Policy HTTPPolicy
	// Token returns the Notion integration token (Authorization: Bearer).
	Token func(ctx context.Context) (string, error)
	// ConnectionCapabilities reports the connection's granted
	// capabilities; the reviewed insert-content capability is required.
	ConnectionCapabilities func(ctx context.Context, a Action) ([]string, error)
	// Recheck re-validates the A03 approval right before the outbound
	// call; a revocation between approval and execute blocks the update.
	Recheck func(ctx context.Context, a Action) error
	// LoadProgress / SaveProgress persist the multi-step recovery record
	// (the same NO-03 record shape as create: real page id + completed
	// block range).
	LoadProgress func(a Action) NotionPageProgress
	SaveProgress func(a Action, p NotionPageProgress) error
	// MaxBatch caps the blocks per append request (test hook; 0 = the
	// documented NotionAppendBatchLimit, never above it).
	MaxBatch int
}

var _ Adapter = (*NotionUpdateAdapter)(nil)

func (m *NotionUpdateAdapter) configError() error {
	if m.Policy.Host == "" || m.Policy.Scheme == "" {
		return fmt.Errorf("%w: no reviewed outbound policy", ErrNotionNotConfigured)
	}
	if m.Token == nil {
		return fmt.Errorf("%w: no token source", ErrNotionNotConfigured)
	}
	return nil
}

func (m *NotionUpdateAdapter) requireInsertCapability(ctx context.Context, a Action) error {
	if m.ConnectionCapabilities == nil {
		return fmt.Errorf("%w: no capability source", ErrNotionMissingCapability)
	}
	caps, err := m.ConnectionCapabilities(ctx, a)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNotionMissingCapability, err)
	}
	for _, c := range caps {
		if c == NotionCapabilityInsert {
			return nil
		}
	}
	return fmt.Errorf("%w: connection lacks %s", ErrNotionMissingCapability, NotionCapabilityInsert)
}

func (m *NotionUpdateAdapter) loadProgress(a Action) NotionPageProgress {
	if m.LoadProgress == nil {
		return NotionPageProgress{}
	}
	return m.LoadProgress(a)
}

func (m *NotionUpdateAdapter) storeProgress(a Action, p NotionPageProgress) error {
	if m.SaveProgress == nil {
		return nil
	}
	if err := m.SaveProgress(a, p); err != nil {
		return fmt.Errorf("%w: persisting progress: %v", ErrNotionOutcomeUnknown, err)
	}
	return nil
}

func (m *NotionUpdateAdapter) batchSize() int {
	if m.MaxBatch > 0 && m.MaxBatch < NotionAppendBatchLimit {
		return m.MaxBatch
	}
	return NotionAppendBatchLimit
}

func (m *NotionUpdateAdapter) targetURL(path string) *url.URL {
	host := m.Policy.Host
	if m.Policy.Port != "" {
		host = net.JoinHostPort(host, m.Policy.Port)
	}
	return &url.URL{Scheme: m.Policy.Scheme, Host: host, Path: path}
}

// do performs ONE policy-validated request through the A04 client — the
// same request/redirect re-validation as the create adapter. Transport
// failures wrap ErrNotionOutcomeUnknown.
func (m *NotionUpdateAdapter) do(ctx context.Context, method string, u *url.URL, body []byte) (int, []byte, error) {
	if err := m.Policy.ValidateRequest(method, u); err != nil {
		return 0, nil, err
	}
	tok, err := m.Token(ctx)
	if err != nil {
		return 0, nil, err
	}
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), rd)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Notion-Version", NotionAPIVersion)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := m.Policy.NewClient().Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: %v", ErrNotionOutcomeUnknown, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("%w: %v", ErrNotionOutcomeUnknown, err)
	}
	return resp.StatusCode, raw, nil
}

// readPageVersion performs the version pre-read (GET /v1/pages/{id}).
func (m *NotionUpdateAdapter) readPageVersion(ctx context.Context, pageID string) (NotionPageVersion, error) {
	u := m.targetURL(fmt.Sprintf(NotionPageFormat, url.PathEscape(pageID)))
	status, raw, err := m.do(ctx, http.MethodGet, u, nil)
	if err != nil {
		return NotionPageVersion{}, err
	}
	if status != http.StatusOK {
		e := parseNotionError(raw)
		return NotionPageVersion{}, fmt.Errorf("notion_provider_error: status=%d code=%s message=%s", status, e.Code, e.Message)
	}
	return ParseNotionPageVersion(raw)
}

// patchTitle performs the idempotent title step (PATCH /v1/pages/{id}).
func (m *NotionUpdateAdapter) patchTitle(ctx context.Context, pageID, title string) (json.RawMessage, error) {
	body, err := json.Marshal(notionUpdatePageRequest{
		Properties: notionCreatePageProperties{Title: notionTitleProperty{Title: []notionRichText{{Text: notionText{Content: title}}}}},
	})
	if err != nil {
		return nil, err
	}
	u := m.targetURL(fmt.Sprintf(NotionPageFormat, url.PathEscape(pageID)))
	status, raw, err := m.do(ctx, http.MethodPatch, u, body)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		e := parseNotionError(raw)
		if status >= 500 {
			return nil, fmt.Errorf("%w: status=%d code=%s", ErrNotionOutcomeUnknown, status, e.Code)
		}
		return nil, fmt.Errorf("notion_provider_error: status=%d code=%s message=%s", status, e.Code, e.Message)
	}
	return raw, nil
}

// appendChildren performs one recoverable append step of the REMAINING
// blocks.
func (m *NotionUpdateAdapter) appendChildren(ctx context.Context, pageID string, blocks []json.RawMessage) (json.RawMessage, error) {
	body, err := json.Marshal(notionAppendChildrenRequest{Children: blocks})
	if err != nil {
		return nil, err
	}
	u := m.targetURL(fmt.Sprintf(NotionAppendChildrenFormat, url.PathEscape(pageID)))
	status, raw, err := m.do(ctx, http.MethodPatch, u, body)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		e := parseNotionError(raw)
		if status >= 500 {
			return nil, fmt.Errorf("%w: status=%d code=%s", ErrNotionOutcomeUnknown, status, e.Code)
		}
		return nil, fmt.Errorf("notion_provider_error: status=%d code=%s message=%s", status, e.Code, e.Message)
	}
	return raw, nil
}

func (m *NotionUpdateAdapter) readChildren(ctx context.Context, pageID string) ([]json.RawMessage, error) {
	u := m.targetURL(fmt.Sprintf(NotionAppendChildrenFormat, url.PathEscape(pageID)))
	status, raw, err := m.do(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("notion_query_unverifiable: status=%d", status)
	}
	var list notionBlockList
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("notion_query_unverifiable: %v", err)
	}
	return list.Results, nil
}

// notionBlocksContained reports whether the approved blocks appear in the
// page's children as one contiguous run (external collaborators may have
// appended their own blocks before or after ours).
func notionBlocksContained(children, blocks []json.RawMessage) bool {
	if len(blocks) == 0 {
		return true
	}
	if len(children) < len(blocks) {
		return false
	}
	for start := 0; start+len(blocks) <= len(children); start++ {
		match := true
		for i := range blocks {
			want, werr := NormalizeArgs(blocks[i])
			got, rerr := NormalizeArgs(children[start+i])
			if werr != nil || rerr != nil || !bytes.Equal(want, got) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// Execute performs the approved update. Order: A03 recheck, snapshot
// validation, capability check, version PRE-READ + conflict detection —
// all BEFORE any network write — then the multi-step write with progress
// persisted between steps, ending with a reliable read-back whose payload
// is the action's output evidence (the receipt basis).
func (m *NotionUpdateAdapter) Execute(ctx context.Context, a Action) (ActionResult, error) {
	if err := m.configError(); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	if m.Recheck != nil {
		if err := m.Recheck(ctx, a); err != nil {
			return ActionResult{State: ActionAwaitingApproval}, fmt.Errorf("%w: %v", ErrNotionApprovalRevoked, err)
		}
	}
	snap, err := ParseNotionUpdateSnapshot(a.Args)
	if err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	if err := m.requireInsertCapability(ctx, a); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	// AC1: read the external current version FIRST. A read can never have
	// produced the write, so an unreadable pre-read is a definitive
	// failure — zero write requests leave the process.
	ver, gerr := m.readPageVersion(ctx, snap.PageID)
	if gerr != nil {
		return ActionResult{State: ActionFailed}, gerr
	}
	if cerr := DetectNotionVersionConflict(snap.ExpectedVersion, ver.LastEditedTime); cerr != nil {
		return ActionResult{State: ActionFailed}, cerr
	}
	pageID := snap.PageID
	progress := m.loadProgress(a)
	done := progress.PageID == pageID && progress.BlocksDone >= 0 && progress.BlocksDone <= len(snap.Blocks)
	blocksDone := 0
	if done {
		blocksDone = progress.BlocksDone
	}
	if blocksDone == 0 {
		if _, terr := m.patchTitle(ctx, pageID, snap.Title); terr != nil {
			state := ActionFailed
			if errors.Is(terr, ErrNotionOutcomeUnknown) {
				state = ActionUnknown
			}
			return ActionResult{State: state, ExternalID: pageID}, terr
		}
		if serr := m.storeProgress(a, NotionPageProgress{PageID: pageID, BlocksDone: 0}); serr != nil {
			return ActionResult{State: ActionUnknown, ExternalID: pageID}, serr
		}
	}
	for blocksDone < len(snap.Blocks) {
		end := blocksDone + m.batchSize()
		if end > len(snap.Blocks) {
			end = len(snap.Blocks)
		}
		if _, aerr := m.appendChildren(ctx, pageID, snap.Blocks[blocksDone:end]); aerr != nil {
			state := ActionFailed
			if errors.Is(aerr, ErrNotionOutcomeUnknown) {
				state = ActionUnknown
			}
			return ActionResult{State: state, ExternalID: pageID}, aerr
		}
		blocksDone = end
		if serr := m.storeProgress(a, NotionPageProgress{PageID: pageID, BlocksDone: blocksDone}); serr != nil {
			return ActionResult{State: ActionUnknown, ExternalID: pageID}, serr
		}
	}
	// Reliable read-back: the payload carrying the page id + the version
	// THIS publish produced stays the action's output evidence (the
	// receipt basis). A lost read-back cannot flip the writes to failed —
	// the effect exists; the honest state is unknown for Query to resolve.
	final, ferr := m.readPageVersion(ctx, pageID)
	if ferr != nil {
		// Any read-back failure parks unknown: the writes already landed, so
		// neither failed (AC2) nor a fabricated success is honest.
		return ActionResult{State: ActionUnknown, ExternalID: pageID}, ferr
	}
	raw, _ := json.Marshal(map[string]string{"object": "page", "id": final.PageID, "last_edited_time": final.LastEditedTime})
	return ActionResult{State: ActionSucceeded, ExternalID: pageID, Output: raw}, nil
}

// Query is the reconciliation entry point for an update parked in
// unknown: it reconciles ONLY via the reliable page read + children read.
// The approved blocks must be present as a contiguous run before success
// is claimed; anything less stays the honest unknown.
func (m *NotionUpdateAdapter) Query(ctx context.Context, a Action) (ActionResult, error) {
	if err := m.configError(); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	snap, err := ParseNotionUpdateSnapshot(a.Args)
	if err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	ver, gerr := m.readPageVersion(ctx, snap.PageID)
	if gerr != nil {
		return ActionResult{State: ActionUnknown}, gerr
	}
	kids, kerr := m.readChildren(ctx, snap.PageID)
	if kerr != nil {
		return ActionResult{State: ActionUnknown}, kerr
	}
	if !notionBlocksContained(kids, snap.Blocks) {
		return ActionResult{State: ActionUnknown}, fmt.Errorf("notion_query_unverifiable: approved blocks not present as a contiguous run")
	}
	raw, _ := json.Marshal(map[string]string{"object": "page", "id": ver.PageID, "last_edited_time": ver.LastEditedTime})
	return ActionResult{State: ActionSucceeded, ExternalID: snap.PageID, Output: raw}, nil
}

// ReadNotionPageVersion performs a one-off version read (GET
// /v1/pages/{id}) through the given reviewed policy and token source —
// the plan-formation pre-read shared by the publish seam. It performs no
// write of any kind.
func ReadNotionPageVersion(ctx context.Context, pol HTTPPolicy, token func(ctx context.Context) (string, error), pageID string) (string, error) {
	m := &NotionUpdateAdapter{Policy: pol, Token: token}
	ver, err := m.readPageVersion(ctx, pageID)
	if err != nil {
		return "", err
	}
	return ver.LastEditedTime, nil
}
