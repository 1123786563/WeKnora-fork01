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

// Notion create rejections.
var (
	// ErrNotionApprovalRevoked: the A03 approval was revoked (or its A02
	// permission version invalidated) between approval and execute; the
	// action stays parked awaiting approval and nothing is dispatched.
	ErrNotionApprovalRevoked = errors.New("notion_approval_revoked")
	// ErrNotionSnapshotInvalid: the action arguments are not exactly the
	// approved three-field create snapshot (parent, title, blocks) — an
	// extra field, a missing field, an empty parent/title, or a block that
	// is not valid JSON is refused rather than silently forwarded.
	ErrNotionSnapshotInvalid = errors.New("notion_snapshot_invalid")
	// ErrNotionParentOutOfScope: the snapshot's parent page is not inside
	// the approval / pre-authorization scope for this connection. The
	// check fails closed: an empty scope list rejects every parent.
	ErrNotionParentOutOfScope = errors.New("notion_parent_out_of_scope")
	// ErrNotionMissingCapability: the connection does not carry the
	// reviewed insert-content capability.
	ErrNotionMissingCapability = errors.New("notion_missing_capability")
	// ErrNotionOutcomeUnknown: the request may or may not have produced
	// its remote effect (response lost, unparseable reply, or a provider
	// reply without a real page id). The outcome stays unknown and
	// resolves ONLY via Query's reliable page read by the persisted id —
	// a title search is never sole proof, and a fresh create is never the
	// recovery for an unknown creation.
	ErrNotionOutcomeUnknown = errors.New("notion_outcome_unknown")
	// ErrNotionNotConfigured: the adapter is missing a reviewed outbound
	// policy or a token source — fail closed, never dial by invention.
	ErrNotionNotConfigured = errors.New("notion_adapter_not_configured")
)

// NO-01 fixed contract, validated against the official Notion "create a
// page" and "append block children" documents (developers.notion.com): the
// API host, the pinned Notion-Version header value, the request paths and
// the insert-content capability. These are the reviewed constants the
// adapter is allowed to use; nothing here is derived from model output.
const (
	NotionAPIHost              = "api.notion.com"
	NotionAPIVersion           = "2022-06-28"
	NotionCreatePagePath       = "/v1/pages"
	NotionPageFormat           = "/v1/pages/%s"
	NotionAppendChildrenFormat = "/v1/blocks/%s/children"
	NotionSearchPath           = "/v1/search"
	NotionCapabilityInsert     = "insert_content"
	// NotionAppendBatchLimit is the documented cap on the number of blocks
	// one append-children request may carry.
	NotionAppendBatchLimit = 100
)

// NO-03 recovery step names.
const (
	NotionStepCreate          = "create"
	NotionStepAppendRemaining = "append_remaining"
	NotionStepComplete        = "complete"
)

// NextNotionStep names the recovery step of a multi-step page creation:
// no persisted page id means create; a persisted page id whose content is
// not yet complete means append the REMAINING blocks to that SAME page —
// never a second page; otherwise the creation is complete.
func NextNotionStep(pageID string, contentDone bool) string {
	if pageID == "" {
		return NotionStepCreate
	}
	if !contentDone {
		return NotionStepAppendRemaining
	}
	return NotionStepComplete
}

// CanRouteNotionStep reports whether an action in the given state may be
// routed through NextNotionStep. Only CONFIRMED states may route —
// dispatched, succeeded, or failed (a failed append step has a persisted
// page id and block range, which is exactly the NO-03 resume case).
// ActionUnknown must NEVER route: an unknown creation has no confirmed
// page id to resume from and resolves only via Query / human
// reconciliation first.
func CanRouteNotionStep(state string) bool {
	switch state {
	case ActionDispatched, ActionSucceeded, ActionFailed:
		return true
	default:
		return false
	}
}

// NotionPageProgress is the persisted recovery record of a multi-step page
// creation: the REAL provider page id — persisted the moment the provider
// confirms the create, BEFORE any content step — plus the number of leading
// snapshot blocks already appended, persisted after every successful batch.
// A local crash between steps resumes from exactly this record, so a second
// page is never created and completed blocks are never re-sent.
type NotionPageProgress struct {
	PageID     string
	BlocksDone int
}

// NotionCreateSnapshot is the A03-approved argument snapshot for one page
// creation: exactly parent (parent page id), title and blocks. Every
// dispatched request is built FROM these fields; nothing is added, rewritten
// or re-derived after approval.
type NotionCreateSnapshot struct {
	Parent string
	Title  string
	Blocks []json.RawMessage
}

// ParseNotionCreateSnapshot validates that args are EXACTLY the approved
// three-field create snapshot: no extra fields (approve-then-rewrite), no
// missing fields, a non-empty parent and title, and blocks that are each
// valid JSON. Block bytes are stored normalized so the wire bytes are the
// approved bytes.
func ParseNotionCreateSnapshot(args json.RawMessage) (NotionCreateSnapshot, error) {
	var s NotionCreateSnapshot
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.UseNumber()
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return s, fmt.Errorf("%w: %v", ErrNotionSnapshotInvalid, err)
	}
	if len(raw) != 3 {
		return s, fmt.Errorf("%w: snapshot must be exactly parent, title, blocks", ErrNotionSnapshotInvalid)
	}
	if err := json.Unmarshal(raw["parent"], &s.Parent); err != nil {
		return s, fmt.Errorf("%w: parent: %v", ErrNotionSnapshotInvalid, err)
	}
	if err := json.Unmarshal(raw["title"], &s.Title); err != nil {
		return s, fmt.Errorf("%w: title: %v", ErrNotionSnapshotInvalid, err)
	}
	if s.Parent == "" {
		return s, fmt.Errorf("%w: empty parent", ErrNotionSnapshotInvalid)
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

// Wire bodies of the NO-01 endpoints. The create body carries ONLY the
// approved parent reference and the title property; content blocks travel
// exclusively through the append-children endpoint so each batch is a
// separately recoverable step.
type notionCreatePageRequest struct {
	Parent     notionParentRef            `json:"parent"`
	Properties notionCreatePageProperties `json:"properties"`
}

type notionParentRef struct {
	PageID string `json:"page_id"`
}

type notionCreatePageProperties struct {
	Title notionTitleProperty `json:"title"`
}

type notionTitleProperty struct {
	Title []notionRichText `json:"title"`
}

type notionRichText struct {
	Text notionText `json:"text"`
}

type notionText struct {
	Content string `json:"content"`
}

type notionAppendChildrenRequest struct {
	Children []json.RawMessage `json:"children"`
}

type notionSearchRequest struct {
	Query  string              `json:"query"`
	Filter notionSearchFilter  `json:"filter"`
	Sort   notionSearchSortDir `json:"sort"`
}

type notionSearchFilter struct {
	Property string `json:"property"`
	Value    string `json:"value"`
}

type notionSearchSortDir struct {
	Direction string `json:"direction"`
	Timestamp string `json:"timestamp"`
}

// notionPageObject is the reliable read shape of GET /v1/pages/{id}.
type notionPageObject struct {
	Object string `json:"object"`
	ID     string `json:"id"`
	Parent struct {
		Type   string `json:"type"`
		PageID string `json:"page_id"`
	} `json:"parent"`
}

type notionBlockList struct {
	Object  string            `json:"object"`
	Results []json.RawMessage `json:"results"`
}

type notionErrorResponse struct {
	Object  string `json:"object"`
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func parseNotionError(raw []byte) notionErrorResponse {
	var e notionErrorResponse
	_ = json.Unmarshal(raw, &e)
	return e
}

// NotionCreateAdapter is the Notion member of the A04 Adapter family. It
// executes ONE approved page creation (A03 snapshot) against the NO-01
// reviewed contract as a multi-step write — create the page, then append
// the approved blocks in batches — persisting the real page id and the
// completed block range between steps (NO-03). Real transport is
// Policy.NewClient(): every request and redirect hop is re-validated by the
// A04 outbound policy; no raw http.Client is ever constructed here.
type NotionCreateAdapter struct {
	// Policy is the admin-reviewed outbound contract (A04).
	Policy HTTPPolicy
	// Token returns the Notion integration token (Authorization: Bearer).
	Token func(ctx context.Context) (string, error)
	// ApprovedParents is the approval / pre-authorization scope: the page
	// ids a creation may be parented under. Empty fails closed for every
	// parent.
	ApprovedParents []string
	// ConnectionCapabilities reports the connection's granted
	// capabilities; the reviewed insert-content capability is required
	// before any write. Nil fails closed.
	ConnectionCapabilities func(ctx context.Context, a Action) ([]string, error)
	// Recheck re-validates the A03 approval right before the outbound
	// call; a revocation between approval and execute blocks the creation.
	Recheck func(ctx context.Context, a Action) error
	// LoadProgress / SaveProgress persist the multi-step recovery record.
	LoadProgress func(a Action) NotionPageProgress
	SaveProgress func(a Action, p NotionPageProgress) error
	// MaxBatch caps the blocks per append request (test hook; 0 = the
	// documented NotionAppendBatchLimit, never above it).
	MaxBatch int
}

var _ Adapter = (*NotionCreateAdapter)(nil)

func (m *NotionCreateAdapter) configError() error {
	if m.Policy.Host == "" || m.Policy.Scheme == "" {
		return fmt.Errorf("%w: no reviewed outbound policy", ErrNotionNotConfigured)
	}
	if m.Token == nil {
		return fmt.Errorf("%w: no token source", ErrNotionNotConfigured)
	}
	return nil
}

// parentApproved fails closed: only a parent explicitly listed in the
// approved scope may receive the new page.
func (m *NotionCreateAdapter) parentApproved(parent string) bool {
	for _, p := range m.ApprovedParents {
		if p == parent {
			return true
		}
	}
	return false
}

func (m *NotionCreateAdapter) requireInsertCapability(ctx context.Context, a Action) error {
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

func (m *NotionCreateAdapter) loadProgress(a Action) NotionPageProgress {
	if m.LoadProgress == nil {
		return NotionPageProgress{}
	}
	return m.LoadProgress(a)
}

func (m *NotionCreateAdapter) storeProgress(a Action, p NotionPageProgress) error {
	if m.SaveProgress == nil {
		return nil
	}
	if err := m.SaveProgress(a, p); err != nil {
		// Progress loss makes the outcome ambiguous: the effect may exist
		// remotely while recovery data is gone. Unknown, never failed —
		// a fabricated failure is what triggers duplicate creations.
		return fmt.Errorf("%w: persisting progress: %v", ErrNotionOutcomeUnknown, err)
	}
	return nil
}

func (m *NotionCreateAdapter) batchSize() int {
	if m.MaxBatch > 0 && m.MaxBatch < NotionAppendBatchLimit {
		return m.MaxBatch
	}
	return NotionAppendBatchLimit
}

func (m *NotionCreateAdapter) targetURL(path string) *url.URL {
	host := m.Policy.Host
	if m.Policy.Port != "" {
		host = net.JoinHostPort(host, m.Policy.Port)
	}
	return &url.URL{Scheme: m.Policy.Scheme, Host: host, Path: path}
}

// do performs ONE policy-validated request through the A04 client. A
// transport-level failure wraps ErrNotionOutcomeUnknown: the effect may
// already exist remotely, so it is unknown, never failed.
func (m *NotionCreateAdapter) do(ctx context.Context, method string, u *url.URL, body []byte) (status int, raw []byte, err error) {
	if err := m.Policy.ValidateRequest(method, u); err != nil {
		return 0, nil, err // policy denial: the request never leaves
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
	raw, err = io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("%w: %v", ErrNotionOutcomeUnknown, err)
	}
	return resp.StatusCode, raw, nil
}

// createPage performs the single create step. The body is built FROM the
// approved snapshot field-by-field: parent page_id and the title property —
// nothing else. A reply without a REAL page id proves nothing.
func (m *NotionCreateAdapter) createPage(ctx context.Context, snap NotionCreateSnapshot) (notionPageObject, json.RawMessage, error) {
	body, err := json.Marshal(notionCreatePageRequest{
		Parent:     notionParentRef{PageID: snap.Parent},
		Properties: notionCreatePageProperties{Title: notionTitleProperty{Title: []notionRichText{{Text: notionText{Content: snap.Title}}}}},
	})
	if err != nil {
		return notionPageObject{}, nil, err
	}
	status, raw, err := m.do(ctx, http.MethodPost, m.targetURL(NotionCreatePagePath), body)
	if err != nil {
		return notionPageObject{}, nil, err
	}
	if status >= 400 {
		e := parseNotionError(raw)
		if status >= 500 {
			// A 5xx leaves the create ambiguous: the page may exist.
			return notionPageObject{}, raw, fmt.Errorf("%w: status=%d code=%s", ErrNotionOutcomeUnknown, status, e.Code)
		}
		return notionPageObject{}, raw, fmt.Errorf("notion_provider_error: status=%d code=%s message=%s", status, e.Code, e.Message)
	}
	var page notionPageObject
	if err := json.Unmarshal(raw, &page); err != nil {
		return notionPageObject{}, raw, fmt.Errorf("%w: %v", ErrNotionOutcomeUnknown, err)
	}
	if page.ID == "" {
		return notionPageObject{}, raw, fmt.Errorf("%w: no real page id in provider reply", ErrNotionOutcomeUnknown)
	}
	return page, raw, nil
}

// appendChildren performs one recoverable append step of the REMAINING
// blocks to the already-created page. The provider's definitive error
// answer fails this step; the persisted block range keeps the resume point
// so only the remaining blocks are re-sent.
func (m *NotionCreateAdapter) appendChildren(ctx context.Context, pageID string, blocks []json.RawMessage) (json.RawMessage, error) {
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
		return nil, fmt.Errorf("notion_provider_error: status=%d code=%s message=%s", status, e.Code, e.Message)
	}
	return raw, nil
}

func (m *NotionCreateAdapter) getPage(ctx context.Context, pageID string) (notionPageObject, json.RawMessage, error) {
	u := m.targetURL(fmt.Sprintf(NotionPageFormat, url.PathEscape(pageID)))
	status, raw, err := m.do(ctx, http.MethodGet, u, nil)
	if err != nil {
		return notionPageObject{}, nil, err
	}
	if status != http.StatusOK {
		return notionPageObject{}, raw, fmt.Errorf("notion_query_unverifiable: status=%d", status)
	}
	var page notionPageObject
	if err := json.Unmarshal(raw, &page); err != nil {
		return notionPageObject{}, raw, fmt.Errorf("notion_query_unverifiable: %v", err)
	}
	if page.ID == "" {
		return notionPageObject{}, raw, fmt.Errorf("notion_query_unverifiable: no page id")
	}
	return page, raw, nil
}

func (m *NotionCreateAdapter) readChildren(ctx context.Context, pageID string) ([]json.RawMessage, error) {
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

// searchByTitle runs the advisory title search for an unknown creation. It
// exists for operator context ONLY — its results are deliberately discarded
// because a same-title page under the same parent is never proof that THIS
// action created it.
func (m *NotionCreateAdapter) searchByTitle(ctx context.Context, snap NotionCreateSnapshot) error {
	body, err := json.Marshal(notionSearchRequest{
		Query:  snap.Title,
		Filter: notionSearchFilter{Property: "object", Value: "page"},
		Sort:   notionSearchSortDir{Direction: "ascending", Timestamp: "last_edited_time"},
	})
	if err != nil {
		return err
	}
	_, _, err = m.do(ctx, http.MethodPost, m.targetURL(NotionSearchPath), body)
	return err
}

// Execute performs the approved creation. Order: A03 recheck, snapshot
// validation, parent-scope check, capability check — all BEFORE any network
// write — then the multi-step write with progress persisted between steps.
// A persisted page id always resumes on the SAME page: a second create is
// structurally unreachable once the provider has confirmed a page id.
func (m *NotionCreateAdapter) Execute(ctx context.Context, a Action) (ActionResult, error) {
	if err := m.configError(); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	if m.Recheck != nil {
		if err := m.Recheck(ctx, a); err != nil {
			return ActionResult{State: ActionAwaitingApproval}, fmt.Errorf("%w: %v", ErrNotionApprovalRevoked, err)
		}
	}
	snap, err := ParseNotionCreateSnapshot(a.Args)
	if err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	if !m.parentApproved(snap.Parent) {
		return ActionResult{State: ActionFailed}, fmt.Errorf("%w: parent %q outside the approved page scope", ErrNotionParentOutOfScope, snap.Parent)
	}
	if err := m.requireInsertCapability(ctx, a); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	progress := m.loadProgress(a)
	pageID := progress.PageID
	done := progress.BlocksDone
	var output json.RawMessage
	if pageID == "" {
		page, raw, cerr := m.createPage(ctx, snap)
		if cerr != nil {
			state := ActionFailed
			if errors.Is(cerr, ErrNotionOutcomeUnknown) {
				// Unknown create: NO page id is persisted, NO second
				// create may follow; reconciliation happens via Query.
				state = ActionUnknown
			}
			return ActionResult{State: state}, cerr
		}
		pageID = page.ID
		done = 0
		output = raw
		// The REAL provider page id is persisted the moment the provider
		// confirms the create — BEFORE any content step — so a crash
		// between steps resumes on the same page.
		if serr := m.storeProgress(a, NotionPageProgress{PageID: pageID, BlocksDone: 0}); serr != nil {
			return ActionResult{State: ActionUnknown}, serr
		}
	}
	for done < len(snap.Blocks) {
		end := done + m.batchSize()
		if end > len(snap.Blocks) {
			end = len(snap.Blocks)
		}
		raw, aerr := m.appendChildren(ctx, pageID, snap.Blocks[done:end])
		if aerr != nil {
			state := ActionFailed
			if errors.Is(aerr, ErrNotionOutcomeUnknown) {
				state = ActionUnknown
			}
			return ActionResult{State: state, ExternalID: pageID}, aerr
		}
		done = end
		// The create response — the payload carrying the REAL page id —
		// stays the action's output evidence; an append reply never
		// replaces it.
		_ = raw
		if serr := m.storeProgress(a, NotionPageProgress{PageID: pageID, BlocksDone: done}); serr != nil {
			return ActionResult{State: ActionUnknown, ExternalID: pageID}, serr
		}
	}
	return ActionResult{State: ActionSucceeded, ExternalID: pageID, Output: output}, nil
}

// Query is the reconciliation entry point. With a persisted page id it
// reconciles ONLY via the reliable page read (GET by that exact id) and the
// children read: the page must still live under the approved parent and
// carry every snapshot block field-by-field before success is claimed;
// anything less stays the honest unknown. Without a persisted page id a
// title search may run for operator context, but its results are NEVER sole
// proof — no page id is ever claimed from a title match, so the outcome
// stays unknown for human reconciliation.
func (m *NotionCreateAdapter) Query(ctx context.Context, a Action) (ActionResult, error) {
	if err := m.configError(); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	snap, err := ParseNotionCreateSnapshot(a.Args)
	if err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	progress := m.loadProgress(a)
	if progress.PageID == "" {
		if serr := m.searchByTitle(ctx, snap); serr != nil {
			return ActionResult{State: ActionUnknown}, serr
		}
		return ActionResult{State: ActionUnknown}, nil
	}
	page, praw, gerr := m.getPage(ctx, progress.PageID)
	if gerr != nil {
		return ActionResult{State: ActionUnknown}, gerr
	}
	if page.Parent.PageID != snap.Parent {
		return ActionResult{State: ActionUnknown}, fmt.Errorf("notion_query_unverifiable: persisted page parent %q != approved %q", page.Parent.PageID, snap.Parent)
	}
	kids, kerr := m.readChildren(ctx, progress.PageID)
	if kerr != nil {
		return ActionResult{State: ActionUnknown}, kerr
	}
	if len(kids) < len(snap.Blocks) {
		// Not provably complete: the honest report is unknown, never a
		// fabricated success or failure.
		return ActionResult{State: ActionUnknown}, fmt.Errorf("notion_query_unverifiable: %d of %d blocks present", len(kids), len(snap.Blocks))
	}
	for i, b := range snap.Blocks {
		want, werr := NormalizeArgs(b)
		got, rerr := NormalizeArgs(kids[i])
		if werr != nil || rerr != nil || !bytes.Equal(want, got) {
			return ActionResult{State: ActionUnknown}, fmt.Errorf("notion_query_unverifiable: block %d differs from the approved snapshot", i)
		}
	}
	return ActionResult{State: ActionSucceeded, ExternalID: progress.PageID, Output: praw}, nil
}
