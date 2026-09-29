// Package publish owns the external publication seam (T18 #48): forming
// an approved Notion publish plan from a confirmed artifact version,
// bridging the A03 action pipeline to the Notion adapter family, and
// settling the durable receipt. Downstream providers (#49 feishu, #50
// confluence) join this seam; #51 extends the single-action plan record.
package publish

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Publish content bounds. MaxPublishBlocks bounds the derived block count
// (Notion append batches carry ≤100 blocks per request; the adapter batches
// beyond that — the cap bounds plan size, not wire size).
const (
	MaxPublishBlocks = 500
	// notionRichTextChunk is below Notion's documented 2000-character cap
	// per rich text text object.
	notionRichTextChunk = 1900
)

var (
	// ErrPublishContentTooLarge: the derived plan exceeds the publish
	// bounds — refused at plan formation, never truncated silently.
	ErrPublishContentTooLarge = errors.New("publish_content_too_large")
	// ErrPublishEmptyContent: the artifact carries no publishable text.
	ErrPublishEmptyContent = errors.New("publish_empty_content")
)

type notionRichTextItem struct {
	Type string `json:"type"`
	Text struct {
		Content string `json:"content"`
	} `json:"text"`
}

type notionParagraphBlock struct {
	Object    string `json:"object"`
	Type      string `json:"type"`
	Paragraph struct {
		RichText []notionRichTextItem `json:"rich_text"`
	} `json:"paragraph"`
}

// NotionParagraphBlocks derives Notion paragraph blocks from plain text:
// paragraphs split on blank lines, each trimmed, long paragraphs chunked
// into ≤1900-rune rich_text text objects (≤100 per block). The output is
// deterministic pure derivation — the same artifact bytes always produce
// the same blocks, so the approval digest pins exactly what will be sent.
func NotionParagraphBlocks(text string) ([]json.RawMessage, error) {
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
		return nil, ErrPublishEmptyContent
	}
	if len(paragraphs) > MaxPublishBlocks {
		return nil, fmt.Errorf("%w: %d paragraphs exceed %d blocks", ErrPublishContentTooLarge, len(paragraphs), MaxPublishBlocks)
	}
	out := make([]json.RawMessage, 0, len(paragraphs))
	for _, p := range paragraphs {
		runes := []rune(p)
		chunks := make([]string, 0, len(runes)/notionRichTextChunk+1)
		for start := 0; start < len(runes); start += notionRichTextChunk {
			end := start + notionRichTextChunk
			if end > len(runes) {
				end = len(runes)
			}
			chunks = append(chunks, string(runes[start:end]))
		}
		if len(chunks) > 100 {
			return nil, fmt.Errorf("%w: one paragraph needs %d rich_text objects", ErrPublishContentTooLarge, len(chunks))
		}
		block := notionParagraphBlock{Object: "block", Type: "paragraph"}
		for _, c := range chunks {
			item := notionRichTextItem{Type: "text"}
			item.Text.Content = c
			block.Paragraph.RichText = append(block.Paragraph.RichText, item)
		}
		raw, err := json.Marshal(block)
		if err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, nil
}
