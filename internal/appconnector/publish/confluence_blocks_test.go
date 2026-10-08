package publish

import (
	"errors"
	"strings"
	"testing"
)

func TestConfluenceStorageBodyParagraphs(t *testing.T) {
	got, err := ConfluenceStorageBody("第一段。\n\n第二段。")
	if err != nil {
		t.Fatal(err)
	}
	if want := "<p>第一段。</p>\n<p>第二段。</p>"; got != want {
		t.Fatalf("storage body drift:\n got %q\nwant %q", got, want)
	}
}

func TestConfluenceStorageBodyNormalizesCRLF(t *testing.T) {
	got, err := ConfluenceStorageBody("a\r\n\r\nb")
	if err != nil || got != "<p>a</p>\n<p>b</p>" {
		t.Fatalf("CRLF normalization drift: %q %v", got, err)
	}
}

func TestConfluenceStorageBodyEscapesHTML(t *testing.T) {
	got, err := ConfluenceStorageBody(`<b>bold</b> & "quoted" 'single'`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "<b>") || !strings.Contains(got, "&lt;b&gt;") || !strings.Contains(got, "&amp;") {
		t.Fatalf("storage body must be HTML-escaped: %q", got)
	}
}

func TestConfluenceStorageBodyKeepsSingleNewlineInsideParagraph(t *testing.T) {
	got, err := ConfluenceStorageBody("line one\nline two")
	if err != nil || got != "<p>line one\nline two</p>" {
		t.Fatalf("single newline stays inside the paragraph (renders as whitespace): %q %v", got, err)
	}
}

func TestConfluenceStorageBodyEmpty(t *testing.T) {
	for _, in := range []string{"", "  \n \n  ", "\n\n"} {
		if _, err := ConfluenceStorageBody(in); !errors.Is(err, ErrPublishEmptyContent) {
			t.Fatalf("%q: want ErrPublishEmptyContent, got %v", in, err)
		}
	}
}

func TestConfluenceStorageBodyTooLarge(t *testing.T) {
	text := strings.Repeat("p\n\n", MaxPublishBlocks) // MaxPublishBlocks+1 paragraphs
	if _, err := ConfluenceStorageBody(text); !errors.Is(err, ErrPublishContentTooLarge) {
		t.Fatalf("want ErrPublishContentTooLarge, got %v", err)
	}
}

// TestConfluenceStorageBodyCrossProviderEdge pins the intentional cross-provider
// divergence at the "exactly MaxPublishBlocks rendered paragraphs plus a
// trailing blank-line separator" edge (T50 final-review finding): the same
// input — Repeat("p\n\n", MaxPublishBlocks) — splits into
// MaxPublishBlocks+1 raw parts, so Confluence refuses it (its gate counts
// raw parts — fail-closed, see the ConfluenceStorageBody doc comment) while
// Notion accepts exactly MaxPublishBlocks non-empty paragraphs. This
// asymmetry is the deliberate fix for the Task 4 plan defect
// (.superpowers/sdd/plan-t50/progress.md 【Task4 自洽——阻塞缺陷】); pinning
// both sides keeps a future "harmonization" from silently loosening the
// Confluence gate or silently tightening the Notion gate.
func TestConfluenceStorageBodyCrossProviderEdge(t *testing.T) {
	text := strings.Repeat("p\n\n", MaxPublishBlocks)

	if _, err := ConfluenceStorageBody(text); !errors.Is(err, ErrPublishContentTooLarge) {
		t.Fatalf("confluence must refuse MaxPublishBlocks+1 raw parts (fail-closed), got %v", err)
	}
	blocks, err := NotionParagraphBlocks(text)
	if err != nil {
		t.Fatalf("notion must accept exactly MaxPublishBlocks non-empty paragraphs, got %v", err)
	}
	if len(blocks) != MaxPublishBlocks {
		t.Fatalf("notion block count drift: want %d, got %d", MaxPublishBlocks, len(blocks))
	}

	// The Confluence gate accepts exactly MaxPublishBlocks raw parts: one
	// fewer repetition splits into exactly MaxPublishBlocks parts
	// (MaxPublishBlocks-1 non-empty + the trailing empty one) and renders
	// MaxPublishBlocks-1 <p> elements.
	body, err := ConfluenceStorageBody(strings.Repeat("p\n\n", MaxPublishBlocks-1))
	if err != nil {
		t.Fatalf("exactly MaxPublishBlocks raw parts must pass the gate, got %v", err)
	}
	if got := strings.Count(body, "<p>"); got != MaxPublishBlocks-1 {
		t.Fatalf("rendered paragraph drift: want %d <p> elements, got %d", MaxPublishBlocks-1, got)
	}
}

func TestConfluenceStorageBodyDeterministic(t *testing.T) {
	a, _ := ConfluenceStorageBody("x\n\ny\n\nz")
	b, _ := ConfluenceStorageBody("x\n\ny\n\nz")
	if a != b {
		t.Fatalf("the same artifact bytes must always produce the same storage string: %q vs %q", a, b)
	}
}
