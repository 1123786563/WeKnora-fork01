package publish

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestNotionParagraphBlocksSplitsParagraphs(t *testing.T) {
	blocks, err := NotionParagraphBlocks("First paragraph.\n\nSecond paragraph.\n\n\nThird.")
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 3 {
		t.Fatalf("want 3 paragraph blocks, got %d", len(blocks))
	}
	var first struct {
		Object    string `json:"object"`
		Type      string `json:"type"`
		Paragraph struct {
			RichText []struct {
				Type string `json:"type"`
				Text struct {
					Content string `json:"content"`
				} `json:"text"`
			} `json:"rich_text"`
		} `json:"paragraph"`
	}
	if err := json.Unmarshal(blocks[0], &first); err != nil {
		t.Fatal(err)
	}
	if first.Object != "block" || first.Type != "paragraph" || len(first.Paragraph.RichText) != 1 ||
		first.Paragraph.RichText[0].Type != "text" || first.Paragraph.RichText[0].Text.Content != "First paragraph." {
		t.Fatalf("block shape drift: %s", blocks[0])
	}
}

func TestNotionParagraphBlocksNormalizesLineEndingsAndWhitespace(t *testing.T) {
	blocks, err := NotionParagraphBlocks("  a  \r\n\r\n \r\n\r\nb  \n")
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 2 {
		t.Fatalf("whitespace-only paragraphs must be dropped, got %d", len(blocks))
	}
	var probe struct {
		Paragraph struct {
			RichText []struct {
				Text struct {
					Content string `json:"content"`
				} `json:"text"`
			} `json:"rich_text"`
		} `json:"paragraph"`
	}
	_ = json.Unmarshal(blocks[0], &probe)
	if probe.Paragraph.RichText[0].Text.Content != "a" {
		t.Fatalf("paragraph must be trimmed: %q", probe.Paragraph.RichText[0].Text.Content)
	}
}

func TestNotionParagraphBlocksChunksLongParagraphs(t *testing.T) {
	// One paragraph of 3x the 1900-rune chunk limit: ONE block carrying a
	// rich_text array of 3 text objects (Notion caps each text object; a
	// block may carry several).
	long := strings.Repeat("字", 1900*3)
	blocks, err := NotionParagraphBlocks(long)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 {
		t.Fatalf("long paragraph stays one block, got %d", len(blocks))
	}
	var probe struct {
		Paragraph struct {
			RichText []json.RawMessage `json:"rich_text"`
		} `json:"paragraph"`
	}
	_ = json.Unmarshal(blocks[0], &probe)
	if len(probe.Paragraph.RichText) != 3 {
		t.Fatalf("want 3 rich_text chunks, got %d", len(probe.Paragraph.RichText))
	}
}

func TestNotionParagraphBlocksRejectsEmpty(t *testing.T) {
	if _, err := NotionParagraphBlocks("   \n\n  \n"); !errors.Is(err, ErrPublishEmptyContent) {
		t.Fatalf("empty content must be refused, got %v", err)
	}
}

func TestNotionParagraphBlocksRejectsTooManyBlocks(t *testing.T) {
	paragraphs := make([]string, MaxPublishBlocks+1)
	for i := range paragraphs {
		paragraphs[i] = "p"
	}
	if _, err := NotionParagraphBlocks(strings.Join(paragraphs, "\n\n")); !errors.Is(err, ErrPublishContentTooLarge) {
		t.Fatalf("block cap must be enforced, got %v", err)
	}
}
