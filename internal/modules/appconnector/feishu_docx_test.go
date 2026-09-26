package appconnector

// FE-PUB-01 contract-layer tests for the Feishu docx publish family
// (#49). The contract facts are pinned against the official oapi-sdk-go
// v3.9.7 docx/v1 service source; the fakes in Task 2 reproduce the wire
// shapes from that source.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseFeishuDocCreateSnapshotExactFields(t *testing.T) {
	args := json.RawMessage(`{"parent_folder":"fld-1","title":"Report","blocks":[{"block_type":2}]}`)
	snap, err := ParseFeishuDocCreateSnapshot(args)
	require.NoError(t, err)
	require.Equal(t, "fld-1", snap.ParentFolder)
	require.Equal(t, "Report", snap.Title)
	require.Len(t, snap.Blocks, 1)
}

func TestParseFeishuDocCreateSnapshotRejectsShapeDrift(t *testing.T) {
	cases := map[string]json.RawMessage{
		"extra field":    json.RawMessage(`{"parent_folder":"f","title":"T","blocks":[],"x":1}`),
		"missing title":  json.RawMessage(`{"parent_folder":"f","blocks":[]}`),
		"missing folder": json.RawMessage(`{"title":"T","blocks":[]}`),
		"missing blocks": json.RawMessage(`{"parent_folder":"f","title":"T"}`),
		"empty folder":   json.RawMessage(`{"parent_folder":"","title":"T","blocks":[]}`),
		"empty title":    json.RawMessage(`{"parent_folder":"f","title":"","blocks":[]}`),
		"invalid block":  json.RawMessage(`{"parent_folder":"f","title":"T","blocks":["not-json-object"]}`),
		"not an object":  json.RawMessage(`["parent_folder"]`),
	}
	for name, args := range cases {
		_, err := ParseFeishuDocCreateSnapshot(args)
		require.ErrorIs(t, err, ErrFeishuPublishSnapshotInvalid, name)
	}
}

func TestParseFeishuDocUpdateSnapshotExactFields(t *testing.T) {
	args := json.RawMessage(`{"document_id":"doc-1","expected_revision":"3","title":"Report v2","blocks":[]}`)
	snap, err := ParseFeishuDocUpdateSnapshot(args)
	require.NoError(t, err)
	require.Equal(t, "doc-1", snap.DocumentID)
	require.Equal(t, "3", snap.ExpectedRevision)
	require.Equal(t, "Report v2", snap.Title)
	require.Empty(t, snap.Blocks)
}

func TestParseFeishuDocUpdateSnapshotRejectsShapeDrift(t *testing.T) {
	cases := map[string]json.RawMessage{
		"extra field":      json.RawMessage(`{"document_id":"d","expected_revision":"1","title":"T","blocks":[],"x":1}`),
		"missing revision": json.RawMessage(`{"document_id":"d","title":"T","blocks":[]}`),
		"empty revision":   json.RawMessage(`{"document_id":"d","expected_revision":"","title":"T","blocks":[]}`),
		"missing document": json.RawMessage(`{"expected_revision":"1","title":"T","blocks":[]}`),
		"missing title":    json.RawMessage(`{"document_id":"d","expected_revision":"1","blocks":[]}`),
		"missing blocks":   json.RawMessage(`{"document_id":"d","expected_revision":"1","title":"T"}`),
		"blocks not json":  json.RawMessage(`{"document_id":"d","expected_revision":"1","title":"T","blocks":[12]}`),
	}
	for name, args := range cases {
		_, err := ParseFeishuDocUpdateSnapshot(args)
		require.ErrorIs(t, err, ErrFeishuPublishSnapshotInvalid, name)
	}
}

func TestIsFeishuDocUpdateArgs(t *testing.T) {
	require.True(t, IsFeishuDocUpdateArgs(json.RawMessage(`{"document_id":"d","expected_revision":"1","title":"T","blocks":[]}`)))
	require.False(t, IsFeishuDocUpdateArgs(json.RawMessage(`{"parent_folder":"f","title":"T","blocks":[]}`)))
	require.False(t, IsFeishuDocUpdateArgs(json.RawMessage(`{"document_id":"d"}`)))
	require.False(t, IsFeishuDocUpdateArgs(json.RawMessage(`not json`)))
}

func TestDetectFeishuRevisionConflict(t *testing.T) {
	require.NoError(t, DetectFeishuRevisionConflict("3", "3"))
	require.ErrorIs(t, DetectFeishuRevisionConflict("3", "4"), ErrFeishuPublishRevisionConflict)
	require.ErrorIs(t, DetectFeishuRevisionConflict("", "3"), ErrFeishuPublishRevisionConflict)
	require.ErrorIs(t, DetectFeishuRevisionConflict("3", ""), ErrFeishuPublishRevisionConflict)
	// The numeric wire shape must not leak into the comparison: "03" vs
	// "3" is a mismatch — the adapter always stores strconv.Itoa output.
	require.ErrorIs(t, DetectFeishuRevisionConflict("03", "3"), ErrFeishuPublishRevisionConflict)
}

func TestFeishuTextBlocksDerivesTextParagraphs(t *testing.T) {
	blocks, err := FeishuTextBlocks("第一段。\n\n第二段。")
	require.NoError(t, err)
	require.Len(t, blocks, 2)
	var first struct {
		BlockType int `json:"block_type"`
		Text      struct {
			Elements []struct {
				TextRun struct {
					Content string `json:"content"`
				} `json:"text_run"`
			} `json:"elements"`
			Style struct{} `json:"style"`
		} `json:"text"`
	}
	require.NoError(t, json.Unmarshal(blocks[0], &first))
	require.Equal(t, 2, first.BlockType, "block_type 2 = text block")
	require.Len(t, first.Text.Elements, 1)
	require.Equal(t, "第一段。", first.Text.Elements[0].TextRun.Content)

	// Deterministic: the same bytes always derive the same blocks.
	again, err := FeishuTextBlocks("第一段。\n\n第二段。")
	require.NoError(t, err)
	require.Equal(t, blocks, again)
}

func TestFeishuTextBlocksEmptyAndOversize(t *testing.T) {
	_, err := FeishuTextBlocks("  \n\n  ")
	require.ErrorIs(t, err, ErrFeishuPublishEmptyContent)

	long := strings.Repeat("段\n\n", feishuDocMaxParagraphs) + "段" // 501 paragraphs
	_, err = FeishuTextBlocks(long)
	require.ErrorIs(t, err, ErrFeishuPublishContentTooLarge)
}

func TestFeishuTextBlocksChunksLongParagraph(t *testing.T) {
	long := strings.Repeat("x", feishuTextRunChunk+10)
	blocks, err := FeishuTextBlocks(long)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	joined := ""
	var parsed struct {
		Text struct {
			Elements []struct {
				TextRun struct {
					Content string `json:"content"`
				} `json:"text_run"`
			} `json:"elements"`
		} `json:"text"`
	}
	require.NoError(t, json.Unmarshal(blocks[0], &parsed))
	for _, e := range parsed.Text.Elements {
		joined += e.TextRun.Content
	}
	require.Equal(t, long, joined, "chunking must preserve the full content")
}

func TestParseFeishuDocumentVersionReadsNumericRevision(t *testing.T) {
	// data.document.revision_id is a JSON NUMBER on the wire (SDK
	// Document.RevisionId *int); the parser stringifies it.
	raw := []byte(`{"code":0,"data":{"document":{"document_id":"doc-1","revision_id":7,"title":""}}}`)
	v, err := ParseFeishuDocumentVersion(raw)
	require.NoError(t, err)
	require.Equal(t, "doc-1", v.DocumentID)
	require.Equal(t, "7", v.RevisionID)

	_, err = ParseFeishuDocumentVersion([]byte(`{"code":0,"data":{"document":{"document_id":"doc-1"}}}`))
	require.Error(t, err, "a reply without a revision is never a version")

	_, err = ParseFeishuDocumentVersion([]byte(`{"code":99991663,"msg":"denied"}`))
	require.Error(t, err)

	// The official not-exist code is TYPED: the plan pre-read tells
	// "folder has no revision" apart from an unreadable destination.
	_, err = ParseFeishuDocumentVersion([]byte(`{"code":99991661,"msg":"not exist"}`))
	require.ErrorIs(t, err, ErrFeishuPublishNotFound)
}

func TestParseFeishuDocReceipt(t *testing.T) {
	rcpt, err := ParseFeishuDocReceipt([]byte(`{"document":{"document_id":"doc-9","revision_id":12}}`))
	require.NoError(t, err)
	require.Equal(t, "doc-9", rcpt.ExternalID)
	require.Equal(t, "12", rcpt.ExternalVersion)
}

func TestFeishuDocBlockContentsExtractsTextRuns(t *testing.T) {
	raw := []byte(`[{"block_id":"b1","parent_id":"doc-1","block_type":2,"text":{"elements":[{"text_run":{"content":"a"}}],"style":{}}},
		{"block_id":"b2","block_type":2,"text":{"elements":[{"text_run":{"content":"b"}}]}}]`)
	got, err := feishuDocBlockContents(raw)
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b"}, got)
}
