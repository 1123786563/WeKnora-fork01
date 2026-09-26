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
	"strconv"
	"strings"
)

// FE-PUB-01 fixed contract for the Feishu docx publish family (#49),
// validated against the official oapi-sdk-go v3.9.7 docx/v1 service
// source (service/docx/v1/resource.go + model.go). The SDK client itself
// is NOT used at runtime (its transport would bypass the A04 outbound
// policy — the same discipline as the FS-01 IM send family); these
// constants pin the method contract instead. Nothing here is derived
// from model output.
const (
	// FeishuDocumentCreatePath is POST /open-apis/docx/v1/documents
	// (SDK resource.go:285). This adapter's create body carries ONLY
	// folder_token: the approved title stays approval/ledger metadata
	// (recorded on the publication row) — it is never sent in the create
	// body and never compared against the remote title. (The SDK's
	// CreateDocumentReqBody model also declares an optional title field
	// at model.go:7897; the reviewed contract here deliberately does not
	// use it.)
	FeishuDocumentCreatePath = "/open-apis/docx/v1/documents"
	// FeishuDocumentGetFormat is GET /open-apis/docx/v1/documents/{id}
	// (SDK resource.go:315) — the reliable version read; the version
	// token is document.revision_id (a JSON number, stringified here).
	FeishuDocumentGetFormat = "/open-apis/docx/v1/documents/%s"
	// FeishuDocumentChildrenFormat is the nested-block children resource
	// (SDK resource.go:533 POST / :563 GET) under the document's root
	// block, whose block_id equals the document_id.
	FeishuDocumentChildrenFormat = "/open-apis/docx/v1/documents/%s/blocks/%s/children"
	// FeishuCapabilityWriteDocx is the reviewed schema_json scope that
	// grants document WRITES. A connection whose reviewed scopes carry
	// only read/sync capabilities never satisfies the publish capability
	// check (AC2: 读取/同步权限不会自动升级为写权限).
	FeishuCapabilityWriteDocx = "write_docx"
	// FeishuDocAppendBatchLimit is the official per-request children cap
	// of the create-children API. The adapter never exceeds it; MaxBatch
	// (Task 2) only lowers it.
	FeishuDocAppendBatchLimit = 50
	// feishuTextRunChunk is the conservative chunk (runes) per
	// text_run.content object — below any documented provider cap, and
	// identical to the Notion projection's conservative value.
	feishuTextRunChunk = 1900
	// feishuDocMaxParagraphs bounds the derived block count — same value
	// and meaning as the Notion projection cap (publish/blocks.go:19
	// MaxPublishBlocks).
	feishuDocMaxParagraphs = 500
)

// Feishu docx publish rejections. Distinct from the FS-01 IM send
// sentinels: a different provider family fails with its own names.
var (
	// ErrFeishuPublishSnapshotInvalid: the action arguments are not
	// exactly the approved snapshot (an extra field, a missing field, a
	// non-JSON block) — refused rather than silently forwarded.
	ErrFeishuPublishSnapshotInvalid = errors.New("feishu_docx_snapshot_invalid")
	// ErrFeishuPublishMissingCapability: the connection does not carry
	// the reviewed write_docx capability (AC2).
	ErrFeishuPublishMissingCapability = errors.New("feishu_docx_missing_capability")
	// ErrFeishuPublishOutcomeUnknown: the request may or may not have
	// produced its remote effect. Resolves ONLY via Query's reliable
	// read — never via a second create.
	ErrFeishuPublishOutcomeUnknown = errors.New("feishu_docx_outcome_unknown")
	// ErrFeishuPublishRevisionConflict: the remote document's current
	// revision no longer equals the approved expected_revision; the
	// update is refused BEFORE any write (CONTEXT.md「外部发布」: 再次
	// 更新前必须读取外部当前版本并形成新的候选变更).
	ErrFeishuPublishRevisionConflict = errors.New("feishu_docx_revision_conflict")
	// ErrFeishuPublishNotConfigured: the adapter is missing a reviewed
	// outbound policy or a token source — fail closed.
	ErrFeishuPublishNotConfigured = errors.New("feishu_docx_adapter_not_configured")
	// ErrFeishuPublishEmptyContent: the artifact carries no publishable text.
	ErrFeishuPublishEmptyContent = errors.New("feishu_docx_empty_content")
	// ErrFeishuPublishContentTooLarge: the derived plan exceeds the publish bounds.
	ErrFeishuPublishContentTooLarge = errors.New("feishu_docx_content_too_large")
	// ErrFeishuPublishNotFound: the remote target does not exist (provider
	// code 99991661 or HTTP 404). The plan-time pre-read distinguishes
	// this typed shape from transport failure: a create destination (the
	// reviewed FOLDER) legitimately has no document revision, while an
	// unreadable update target must fail the plan.
	ErrFeishuPublishNotFound = errors.New("feishu_docx_target_not_found")
)

// FeishuDocProgress is the persisted recovery record of a multi-step
// docx publication — the FE-03 counterpart of NotionPageProgress
// (notion_create.go:106-109): the REAL document id, persisted the moment
// the provider confirms the create BEFORE any content step, plus the
// number of leading snapshot blocks already appended, persisted after
// every successful batch. A crash between steps resumes from exactly
// this record, so a second document is never created and committed
// blocks are never re-sent.
type FeishuDocProgress struct {
	DocumentID string
	BlocksDone int
}

// FeishuDocCreateSnapshot is the A03-approved argument snapshot for one
// document creation: exactly parent_folder (the reviewed destination
// folder), title (approval/ledger metadata — see the create-path note
// above) and blocks.
type FeishuDocCreateSnapshot struct {
	ParentFolder string
	Title        string
	Blocks       []json.RawMessage
}

// ParseFeishuDocCreateSnapshot validates that args are EXACTLY the
// approved three-field create snapshot.
func ParseFeishuDocCreateSnapshot(args json.RawMessage) (FeishuDocCreateSnapshot, error) {
	var s FeishuDocCreateSnapshot
	raw, err := feishuDocArgsObject(args)
	if err != nil {
		return s, err
	}
	if len(raw) != 3 {
		return s, fmt.Errorf("%w: snapshot must be exactly parent_folder, title, blocks", ErrFeishuPublishSnapshotInvalid)
	}
	if err := json.Unmarshal(raw["parent_folder"], &s.ParentFolder); err != nil {
		return s, fmt.Errorf("%w: parent_folder: %v", ErrFeishuPublishSnapshotInvalid, err)
	}
	if err := json.Unmarshal(raw["title"], &s.Title); err != nil {
		return s, fmt.Errorf("%w: title: %v", ErrFeishuPublishSnapshotInvalid, err)
	}
	if s.ParentFolder == "" {
		return s, fmt.Errorf("%w: empty parent_folder", ErrFeishuPublishSnapshotInvalid)
	}
	if s.Title == "" {
		return s, fmt.Errorf("%w: empty title", ErrFeishuPublishSnapshotInvalid)
	}
	s.Blocks, err = feishuDocBlocksField(raw["blocks"])
	if err != nil {
		return s, err
	}
	return s, nil
}

// FeishuDocUpdateSnapshot is the A03-approved argument snapshot for
// appending to ONE existing external document: exactly document_id,
// expected_revision (the revision the plan read before approval — the
// immutable approval anchor), title and blocks. Blocks may be empty.
type FeishuDocUpdateSnapshot struct {
	DocumentID       string
	ExpectedRevision string
	Title            string
	Blocks           []json.RawMessage
}

// ParseFeishuDocUpdateSnapshot validates that args are EXACTLY the
// approved four-field update snapshot.
func ParseFeishuDocUpdateSnapshot(args json.RawMessage) (FeishuDocUpdateSnapshot, error) {
	var s FeishuDocUpdateSnapshot
	raw, err := feishuDocArgsObject(args)
	if err != nil {
		return s, err
	}
	if len(raw) != 4 {
		return s, fmt.Errorf("%w: snapshot must be exactly document_id, expected_revision, title, blocks", ErrFeishuPublishSnapshotInvalid)
	}
	if err := json.Unmarshal(raw["document_id"], &s.DocumentID); err != nil {
		return s, fmt.Errorf("%w: document_id: %v", ErrFeishuPublishSnapshotInvalid, err)
	}
	if err := json.Unmarshal(raw["expected_revision"], &s.ExpectedRevision); err != nil {
		return s, fmt.Errorf("%w: expected_revision: %v", ErrFeishuPublishSnapshotInvalid, err)
	}
	if err := json.Unmarshal(raw["title"], &s.Title); err != nil {
		return s, fmt.Errorf("%w: title: %v", ErrFeishuPublishSnapshotInvalid, err)
	}
	if s.DocumentID == "" {
		return s, fmt.Errorf("%w: empty document_id", ErrFeishuPublishSnapshotInvalid)
	}
	if s.ExpectedRevision == "" {
		return s, fmt.Errorf("%w: empty expected_revision", ErrFeishuPublishSnapshotInvalid)
	}
	if s.Title == "" {
		return s, fmt.Errorf("%w: empty title", ErrFeishuPublishSnapshotInvalid)
	}
	s.Blocks, err = feishuDocBlocksField(raw["blocks"])
	if err != nil {
		return s, err
	}
	return s, nil
}

// IsFeishuDocUpdateArgs reports whether args carry the update snapshot's
// distinguishing key pair (document_id AND expected_revision). It never
// parses the full snapshot — the bridge routes on it; full validation
// happens in ParseFeishuDocUpdateSnapshot.
func IsFeishuDocUpdateArgs(args json.RawMessage) bool {
	raw, err := feishuDocArgsObject(args)
	if err != nil {
		return false
	}
	_, hasDoc := raw["document_id"]
	_, hasRev := raw["expected_revision"]
	return hasDoc && hasRev
}

// DetectFeishuRevisionConflict compares the approved expected revision
// with the revision just read from the provider. Anything but an exact
// string match — including an unreadable empty side — is a conflict; an
// unobservable remote state must never authorize an overwrite.
func DetectFeishuRevisionConflict(expected, actual string) error {
	if expected == "" || actual == "" || expected != actual {
		return fmt.Errorf("%w: approved %q but remote has %q", ErrFeishuPublishRevisionConflict, expected, actual)
	}
	return nil
}

// feishuTextElement / feishuTextBlock mirror the official block shapes
// (SDK Block:502, Text:5298, TextElement:5346, TextRun:5687).
type feishuTextElement struct {
	TextRun struct {
		Content string `json:"content"`
	} `json:"text_run"`
}

type feishuTextBlock struct {
	BlockType int `json:"block_type"`
	Text      struct {
		Elements []feishuTextElement `json:"elements"`
		Style    struct{}            `json:"style"`
	} `json:"text"`
}

// FeishuTextBlocks derives Feishu text blocks from plain text — the
// deterministic pure counterpart of NotionParagraphBlocks
// (publish/blocks.go:53): paragraphs split on blank lines, each trimmed,
// long paragraphs chunked into ≤feishuTextRunChunk-rune text_run content
// objects. The same artifact bytes always produce the same blocks, so
// the approval digest pins exactly what will be sent. The parent package
// cannot import the publish subpackage (publish/plan.go:11 imports THIS
// package), so the paragraph derivation is self-contained here.
func FeishuTextBlocks(text string) ([]json.RawMessage, error) {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	parts := strings.Split(normalized, "\n\n")
	paragraphs := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			paragraphs = append(paragraphs, trimmed)
		}
	}
	if len(paragraphs) == 0 {
		return nil, ErrFeishuPublishEmptyContent
	}
	if len(paragraphs) > feishuDocMaxParagraphs {
		return nil, fmt.Errorf("%w: %d paragraphs exceed %d blocks", ErrFeishuPublishContentTooLarge, len(paragraphs), feishuDocMaxParagraphs)
	}
	out := make([]json.RawMessage, 0, len(paragraphs))
	for _, p := range paragraphs {
		block := feishuTextBlock{BlockType: 2}
		runes := []rune(p)
		chunks := make([]string, 0, len(runes)/feishuTextRunChunk+1)
		for start := 0; start < len(runes); start += feishuTextRunChunk {
			end := start + feishuTextRunChunk
			if end > len(runes) {
				end = len(runes)
			}
			chunks = append(chunks, string(runes[start:end]))
		}
		if len(chunks) > 100 {
			return nil, fmt.Errorf("%w: one paragraph needs %d text_run objects", ErrFeishuPublishContentTooLarge, len(chunks))
		}
		for _, c := range chunks {
			var el feishuTextElement
			el.TextRun.Content = c
			block.Text.Elements = append(block.Text.Elements, el)
		}
		raw, err := json.Marshal(block)
		if err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, nil
}

// FeishuDocVersion is the reliable read shape of the get-document call:
// the exact document id and its revision as a string (the wire value is
// a JSON number — stringified via Itoa so the snapshot's
// expected_revision and the live value compare byte-for-byte).
type FeishuDocVersion struct {
	DocumentID string
	RevisionID string
}

// FeishuDocReceipt is the persisted external receipt of one publish:
// the provider document id and the revision the publish itself produced
// (read back from the provider's own reply — never fabricated locally).
type FeishuDocReceipt struct {
	ExternalID      string
	ExternalVersion string
}

// ParseFeishuDocReceipt extracts the receipt fields from a provider
// reply payload (the create/update evidence the adapter recorded as the
// action's output).
func ParseFeishuDocReceipt(raw []byte) (FeishuDocReceipt, error) {
	v, err := ParseFeishuDocumentVersion(raw)
	if err != nil {
		return FeishuDocReceipt{}, err
	}
	return FeishuDocReceipt{ExternalID: v.DocumentID, ExternalVersion: v.RevisionID}, nil
}

// feishuDocIdentity is the provider's document object (SDK Document,
// model.go:2140-2145): a real id plus the revision as a JSON number.
type feishuDocIdentity struct {
	DocumentID string `json:"document_id"`
	RevisionID *int   `json:"revision_id"`
	Title      string `json:"title"`
}

// feishuDocEnvelope accepts BOTH provider reply shapes: the full wire
// envelope `{"code":int,"msg":string,"data":{"document":…}}` (code==0 is
// success) and the bare, already-unwrapped data payload `{"document":…}`
// (SDK CreateDocumentRespData) that create/update replies carry when
// recorded as an action's output payload.
type feishuDocEnvelope struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Document *feishuDocIdentity `json:"document"`
	} `json:"data"`
	Document *feishuDocIdentity `json:"document"`
}

// ParseFeishuDocumentVersion extracts the document identity + current
// revision from a get-document reply. A reply without a real id or a
// real (present, non-nil) revision is an error — a fabricated version is
// never a basis for conflict detection or a receipt.
func ParseFeishuDocumentVersion(raw []byte) (FeishuDocVersion, error) {
	var env feishuDocEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return FeishuDocVersion{}, fmt.Errorf("%w: %v", ErrFeishuPublishOutcomeUnknown, err)
	}
	if env.Code != 0 {
		if env.Code == 99991661 { // the official "not exist / no permission" code
			return FeishuDocVersion{}, fmt.Errorf("%w: code=%d msg=%s", ErrFeishuPublishNotFound, env.Code, env.Msg)
		}
		return FeishuDocVersion{}, fmt.Errorf("feishu_provider_error: code=%d msg=%s", env.Code, env.Msg)
	}
	doc := env.Data.Document
	if doc == nil {
		doc = env.Document
	}
	if doc == nil || doc.DocumentID == "" || doc.RevisionID == nil {
		return FeishuDocVersion{}, fmt.Errorf("%w: reply carries no real document id/revision", ErrFeishuPublishOutcomeUnknown)
	}
	return FeishuDocVersion{
		DocumentID: doc.DocumentID,
		RevisionID: strconv.Itoa(*doc.RevisionID),
	}, nil
}

// feishuDocBlockContents extracts the text_run content sequence from a
// children payload (a JSON array of provider blocks). Query reconciliation
// compares THIS semantic projection — provider blocks carry server-side
// fields (block_id/parent_id/…) the approved snapshot bytes never have,
// so whole-block byte comparison would report every real success as
// unverifiable (see Review Focus 5).
func feishuDocBlockContents(raw []byte) ([]string, error) {
	var blocks []struct {
		Text struct {
			Elements []feishuTextElement `json:"elements"`
		} `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(blocks))
	for _, b := range blocks {
		joined := ""
		for _, el := range b.Text.Elements {
			joined += el.TextRun.Content
		}
		out = append(out, joined)
	}
	return out, nil
}

// ---- shared parsing helpers ----

func feishuDocArgsObject(args json.RawMessage) (map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.UseNumber()
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFeishuPublishSnapshotInvalid, err)
	}
	return raw, nil
}

func feishuDocBlocksField(field json.RawMessage) ([]json.RawMessage, error) {
	var blocks []json.RawMessage
	if err := json.Unmarshal(field, &blocks); err != nil {
		return nil, fmt.Errorf("%w: blocks: %v", ErrFeishuPublishSnapshotInvalid, err)
	}
	out := make([]json.RawMessage, 0, len(blocks))
	for i, b := range blocks {
		n, err := NormalizeArgs(b)
		if err != nil {
			return nil, fmt.Errorf("%w: block %d: %v", ErrFeishuPublishSnapshotInvalid, i, err)
		}
		// A provider block is a JSON OBJECT (SDK Block, model.go:502).
		// NormalizeArgs accepts any valid JSON value — a bare string or
		// number is not a block shape and must be refused here.
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(n, &obj); err != nil {
			return nil, fmt.Errorf("%w: block %d: not a JSON object", ErrFeishuPublishSnapshotInvalid, i)
		}
		out = append(out, n)
	}
	return out, nil
}

// FeishuDocxAdapter executes ONE approved docx publication against the
// FE-PUB-01 reviewed contract, routed through the A04 outbound policy.
// Outcome semantics mirror the Notion family exactly (the service layer
// above cannot tell the providers apart — AC1):
//
//   - pre-read failures (update branch) are definitive FAILED — a GET
//     can never have produced the write;
//   - every write-step transport failure / 5xx / unparseable reply is
//     ErrFeishuPublishOutcomeUnknown — the effect may exist remotely;
//   - Query reconciles ONLY via the reliable document read + children
//     read; a fresh create is never the recovery for an unknown create.
type FeishuDocxAdapter struct {
	// Policy is the admin-reviewed outbound contract (A04).
	Policy HTTPPolicy
	// Token returns the connection's Feishu credential (Bearer).
	Token func(ctx context.Context) (string, error)
	// ConnectionCapabilities reports the connection's reviewed scopes;
	// the write_docx capability is required (AC2).
	ConnectionCapabilities func(ctx context.Context, a Action) ([]string, error)
	// Recheck re-validates the A03 approval right before outbound calls.
	Recheck func(ctx context.Context, a Action) error
	// LoadProgress / SaveProgress persist the FE-03 recovery record.
	LoadProgress func(a Action) FeishuDocProgress
	SaveProgress func(a Action, p FeishuDocProgress) error
	// MaxBatch caps children per append request (test hook; 0 = the
	// documented FeishuDocAppendBatchLimit, never above it).
	MaxBatch int
}

var _ Adapter = (*FeishuDocxAdapter)(nil)

func (m *FeishuDocxAdapter) configError() error {
	if m.Policy.Host == "" || m.Policy.Scheme == "" {
		return fmt.Errorf("%w: no reviewed outbound policy", ErrFeishuPublishNotConfigured)
	}
	if m.Token == nil {
		return fmt.Errorf("%w: no token source", ErrFeishuPublishNotConfigured)
	}
	return nil
}

func (m *FeishuDocxAdapter) requireWriteCapability(ctx context.Context, a Action) error {
	if m.ConnectionCapabilities == nil {
		return fmt.Errorf("%w: no capability source", ErrFeishuPublishMissingCapability)
	}
	caps, err := m.ConnectionCapabilities(ctx, a)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrFeishuPublishMissingCapability, err)
	}
	for _, c := range caps {
		if c == FeishuCapabilityWriteDocx {
			return nil
		}
	}
	return fmt.Errorf("%w: connection lacks %s", ErrFeishuPublishMissingCapability, FeishuCapabilityWriteDocx)
}

func (m *FeishuDocxAdapter) loadProgress(a Action) FeishuDocProgress {
	if m.LoadProgress == nil {
		return FeishuDocProgress{}
	}
	return m.LoadProgress(a)
}

func (m *FeishuDocxAdapter) storeProgress(a Action, p FeishuDocProgress) error {
	if m.SaveProgress == nil {
		return nil
	}
	if err := m.SaveProgress(a, p); err != nil {
		return fmt.Errorf("%w: persisting progress: %v", ErrFeishuPublishOutcomeUnknown, err)
	}
	return nil
}

func (m *FeishuDocxAdapter) batchSize() int {
	if m.MaxBatch > 0 && m.MaxBatch < FeishuDocAppendBatchLimit {
		return m.MaxBatch
	}
	return FeishuDocAppendBatchLimit
}

// Execute routes on the approved snapshot's shape (AC1 lives BELOW this
// point only).
func (m *FeishuDocxAdapter) Execute(ctx context.Context, a Action) (ActionResult, error) {
	if err := m.configError(); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	if m.Recheck != nil {
		if err := m.Recheck(ctx, a); err != nil {
			return ActionResult{State: ActionAwaitingApproval}, fmt.Errorf("approval revoked: %v", err)
		}
	}
	if IsFeishuDocUpdateArgs(a.Args) {
		return m.executeUpdate(ctx, a)
	}
	return m.executeCreate(ctx, a)
}

func (m *FeishuDocxAdapter) executeCreate(ctx context.Context, a Action) (ActionResult, error) {
	snap, err := ParseFeishuDocCreateSnapshot(a.Args)
	if err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	if err := m.requireWriteCapability(ctx, a); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	progress := m.loadProgress(a)
	docID := progress.DocumentID
	done := progress.BlocksDone
	var output json.RawMessage
	if docID == "" {
		body, _ := json.Marshal(map[string]string{"folder_token": snap.ParentFolder})
		raw, cerr := m.do(ctx, http.MethodPost, m.targetURL(FeishuDocumentCreatePath, nil), body)
		if cerr != nil {
			state := ActionFailed
			if errors.Is(cerr, ErrFeishuPublishOutcomeUnknown) {
				// Unknown create: NO document id is persisted, NO second
				// create may follow; reconciliation happens via Query.
				state = ActionUnknown
			}
			return ActionResult{State: state}, cerr
		}
		v, verr := ParseFeishuDocumentVersion(raw)
		if verr != nil {
			return ActionResult{State: ActionUnknown}, verr
		}
		docID = v.DocumentID
		done = 0
		output = json.RawMessage(raw)
		// The REAL document id is persisted the moment the provider
		// confirms the create — BEFORE any content step.
		if serr := m.storeProgress(a, FeishuDocProgress{DocumentID: docID, BlocksDone: 0}); serr != nil {
			return ActionResult{State: ActionUnknown}, serr
		}
	}
	for done < len(snap.Blocks) {
		end := done + m.batchSize()
		if end > len(snap.Blocks) {
			end = len(snap.Blocks)
		}
		if aerr := m.appendChildren(ctx, docID, snap.Blocks[done:end]); aerr != nil {
			state := ActionFailed
			if errors.Is(aerr, ErrFeishuPublishOutcomeUnknown) {
				state = ActionUnknown
			}
			return ActionResult{State: state, ExternalID: docID}, aerr
		}
		done = end
		if serr := m.storeProgress(a, FeishuDocProgress{DocumentID: docID, BlocksDone: done}); serr != nil {
			return ActionResult{State: ActionUnknown, ExternalID: docID}, serr
		}
	}
	return ActionResult{State: ActionSucceeded, ExternalID: docID, Output: output}, nil
}

func (m *FeishuDocxAdapter) executeUpdate(ctx context.Context, a Action) (ActionResult, error) {
	snap, err := ParseFeishuDocUpdateSnapshot(a.Args)
	if err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	if err := m.requireWriteCapability(ctx, a); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	// AC1: read the external current revision FIRST — an unreadable
	// pre-read is a definitive failure with zero writes.
	ver, gerr := ReadFeishuDocumentVersion(ctx, m.Policy, m.Token, snap.DocumentID)
	if gerr != nil {
		return ActionResult{State: ActionFailed}, gerr
	}
	if cerr := DetectFeishuRevisionConflict(snap.ExpectedRevision, ver.RevisionID); cerr != nil {
		return ActionResult{State: ActionFailed}, cerr
	}
	docID := snap.DocumentID
	progress := m.loadProgress(a)
	blocksDone := 0
	if progress.DocumentID == docID && progress.BlocksDone >= 0 && progress.BlocksDone <= len(snap.Blocks) {
		blocksDone = progress.BlocksDone
	}
	for blocksDone < len(snap.Blocks) {
		end := blocksDone + m.batchSize()
		if end > len(snap.Blocks) {
			end = len(snap.Blocks)
		}
		if aerr := m.appendChildren(ctx, docID, snap.Blocks[blocksDone:end]); aerr != nil {
			state := ActionFailed
			if errors.Is(aerr, ErrFeishuPublishOutcomeUnknown) {
				state = ActionUnknown
			}
			return ActionResult{State: state, ExternalID: docID}, aerr
		}
		blocksDone = end
		if serr := m.storeProgress(a, FeishuDocProgress{DocumentID: docID, BlocksDone: blocksDone}); serr != nil {
			return ActionResult{State: ActionUnknown, ExternalID: docID}, serr
		}
	}
	// Reliable read-back: the payload carrying the document id + the
	// revision THIS publish produced is the receipt basis. A lost
	// read-back parks unknown — the writes already landed.
	raw, ferr := m.do(ctx, http.MethodGet, m.targetURL(fmt.Sprintf(FeishuDocumentGetFormat, urlPathEscape(docID)), nil), nil)
	if ferr != nil {
		return ActionResult{State: ActionUnknown, ExternalID: docID}, ferr
	}
	// The read-back must parse as a REAL document id + revision — the
	// parsed value itself is not needed here (the raw envelope is the
	// output evidence), but an unparseable read-back parks unknown.
	if _, perr := ParseFeishuDocumentVersion(raw); perr != nil {
		return ActionResult{State: ActionUnknown, ExternalID: docID}, perr
	}
	return ActionResult{State: ActionSucceeded, ExternalID: docID, Output: json.RawMessage(raw)}, nil
}

// Query is the reconciliation entry point for an unknown outcome.
// Without any reconciliation anchor it stays honestly unknown (Feishu
// has no reliable create-confirmation search; a title match is never
// proof). The anchor is the persisted checkpoint's document id when one
// exists; for updates the APPROVED snapshot's document_id is an equal
// authority (the plan's update gate binds it to a document this
// connection previously published) and covers the case where the FIRST
// batch's reply was lost so no checkpoint was ever persisted. With an
// anchor, Query reconciles via the reliable document read + the FULL
// paginated children read, comparing the text_run CONTENT sequences —
// provider blocks carry server-side fields the snapshot bytes never do.
func (m *FeishuDocxAdapter) Query(ctx context.Context, a Action) (ActionResult, error) {
	if err := m.configError(); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	progress := m.loadProgress(a)
	documentID := progress.DocumentID
	var want []string
	if IsFeishuDocUpdateArgs(a.Args) {
		snap, err := ParseFeishuDocUpdateSnapshot(a.Args)
		if err != nil {
			return ActionResult{State: ActionUnknown}, err
		}
		if documentID == "" {
			documentID = snap.DocumentID
		}
		for _, b := range snap.Blocks {
			contents, err := feishuDocBlockContents([]byte("[" + string(b) + "]"))
			if err != nil {
				return ActionResult{State: ActionUnknown}, err
			}
			want = append(want, contents...)
		}
	} else {
		snap, err := ParseFeishuDocCreateSnapshot(a.Args)
		if err != nil {
			return ActionResult{State: ActionUnknown}, err
		}
		for _, b := range snap.Blocks {
			contents, err := feishuDocBlockContents([]byte("[" + string(b) + "]"))
			if err != nil {
				return ActionResult{State: ActionUnknown}, err
			}
			want = append(want, contents...)
		}
	}
	if documentID == "" {
		return ActionResult{State: ActionUnknown}, fmt.Errorf("feishu_query_unverifiable: no persisted document id")
	}
	// The document must still exist.
	if _, gerr := ReadFeishuDocumentVersion(ctx, m.Policy, m.Token, documentID); gerr != nil {
		return ActionResult{State: ActionUnknown}, gerr
	}
	kids, kerr := m.readAllChildren(ctx, documentID)
	if kerr != nil {
		return ActionResult{State: ActionUnknown}, kerr
	}
	got, gerr := feishuDocBlockContents(kids)
	if gerr != nil {
		return ActionResult{State: ActionUnknown}, gerr
	}
	if !containsPrefix(got, want) {
		return ActionResult{State: ActionUnknown}, fmt.Errorf("feishu_query_unverifiable: %d of %d approved paragraphs present in order", len(got), len(want))
	}
	raw, rerr := m.do(ctx, http.MethodGet, m.targetURL(fmt.Sprintf(FeishuDocumentGetFormat, urlPathEscape(documentID)), nil), nil)
	if rerr != nil {
		return ActionResult{State: ActionUnknown, ExternalID: documentID}, rerr
	}
	return ActionResult{State: ActionSucceeded, ExternalID: documentID, Output: json.RawMessage(raw)}, nil
}

// containsPrefix reports whether want appears in got as one contiguous
// run starting at any offset (external collaborators may have appended
// their own paragraphs before or after ours) — the semantic counterpart
// of notionBlocksContained (notion_update.go:375-397).
func containsPrefix(got, want []string) bool {
	if len(want) == 0 {
		return true
	}
	if len(got) < len(want) {
		return false
	}
	for start := 0; start+len(want) <= len(got); start++ {
		match := true
		for i := range want {
			if got[start+i] != want[i] {
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

func urlPathEscape(s string) string { return url.PathEscape(s) }

// appendChildren performs one recoverable append step of the REMAINING
// blocks (index -1 = append at the document end).
func (m *FeishuDocxAdapter) appendChildren(ctx context.Context, docID string, blocks []json.RawMessage) error {
	body, err := json.Marshal(map[string]any{"children": blocks, "index": -1})
	if err != nil {
		return err
	}
	u := m.targetURL(fmt.Sprintf(FeishuDocumentChildrenFormat, urlPathEscape(docID), urlPathEscape(docID)), nil)
	raw, err := m.do(ctx, http.MethodPost, u, body)
	if err != nil {
		return err
	}
	var env feishuDocEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("%w: %v", ErrFeishuPublishOutcomeUnknown, err)
	}
	if env.Code != 0 {
		return fmt.Errorf("feishu_provider_error: code=%d msg=%s", env.Code, env.Msg)
	}
	return nil
}

// readAllChildren walks the paginated children list to the end — a
// reconciliation that reads only the first page would miscount committed
// blocks (Review Focus 3).
func (m *FeishuDocxAdapter) readAllChildren(ctx context.Context, docID string) ([]byte, error) {
	pageToken := ""
	var all []json.RawMessage
	for {
		path := fmt.Sprintf(FeishuDocumentChildrenFormat, urlPathEscape(docID), urlPathEscape(docID))
		var query url.Values
		if pageToken != "" {
			query = url.Values{"page_token": {pageToken}}
		}
		raw, err := m.do(ctx, http.MethodGet, m.targetURL(path, query), nil)
		if err != nil {
			return nil, err
		}
		var page struct {
			Code int `json:"code"`
			Data struct {
				Items     []json.RawMessage `json:"items"`
				PageToken string            `json:"page_token"`
				HasMore   bool              `json:"has_more"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrFeishuPublishOutcomeUnknown, err)
		}
		if page.Code != 0 {
			return nil, fmt.Errorf("feishu_provider_error: code=%d", page.Code)
		}
		all = append(all, page.Data.Items...)
		if !page.Data.HasMore || page.Data.PageToken == "" {
			break
		}
		pageToken = page.Data.PageToken
	}
	return json.Marshal(all)
}

// do performs ONE policy-validated request through the A04 client — the
// same request/redirect re-validation as every adapter in this package
// (same shape as FeishuSendAdapter.do, feishu_send.go:242). Transport
// failures wrap ErrFeishuPublishOutcomeUnknown; provider 4xx is a
// definitive provider error; 5xx is unknown.
func (m *FeishuDocxAdapter) do(ctx context.Context, method string, u *url.URL, body []byte) ([]byte, error) {
	if err := m.Policy.ValidateRequest(method, u); err != nil {
		return nil, err // policy denial: the request never leaves
	}
	tok, err := m.Token(ctx)
	if err != nil {
		return nil, err
	}
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	if body != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	resp, err := m.Policy.NewClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFeishuPublishOutcomeUnknown, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFeishuPublishOutcomeUnknown, err)
	}
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("%w: status=%d", ErrFeishuPublishOutcomeUnknown, resp.StatusCode)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: status=404", ErrFeishuPublishNotFound)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("feishu_provider_error: status=%d body=%s", resp.StatusCode, truncateForLog(raw))
	}
	return raw, nil
}

// targetURL mirrors FeishuSendAdapter.targetURL (feishu_send.go:220):
// the reviewed scheme/host/port plus the exact path and — when present —
// the query as RawQuery (the children pagination page_token must travel
// as a REAL query parameter, never as escaped path text).
func (m *FeishuDocxAdapter) targetURL(path string, query url.Values) *url.URL {
	host := m.Policy.Host
	if m.Policy.Port != "" {
		host = net.JoinHostPort(host, m.Policy.Port)
	}
	u := &url.URL{Scheme: m.Policy.Scheme, Host: host, Path: path}
	if query != nil {
		u.RawQuery = query.Encode()
	}
	return u
}

func truncateForLog(raw []byte) string {
	s := string(raw)
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

// ReadFeishuDocumentVersion performs a one-off version read through the
// given reviewed policy and token source — the plan-formation pre-read
// shared by the publish seam (the counterpart of ReadNotionPageVersion,
// notion_update.go:507+). It performs no write.
func ReadFeishuDocumentVersion(ctx context.Context, policy HTTPPolicy, token func(context.Context) (string, error), documentID string) (FeishuDocVersion, error) {
	probe := &FeishuDocxAdapter{Policy: policy, Token: token}
	raw, err := probe.do(ctx, http.MethodGet, probe.targetURL(fmt.Sprintf(FeishuDocumentGetFormat, urlPathEscape(documentID)), nil), nil)
	if err != nil {
		return FeishuDocVersion{}, err
	}
	return ParseFeishuDocumentVersion(raw)
}
