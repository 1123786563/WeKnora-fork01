package appconnector

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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
