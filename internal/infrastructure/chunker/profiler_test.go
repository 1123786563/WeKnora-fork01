package chunker

import (
	"strings"
	"testing"
)

func TestProfileDocument_Empty(t *testing.T) {
	p := ProfileDocument("")
	if p.TotalChars != 0 || p.TotalLines != 0 {
		t.Errorf("empty doc should have zero stats, got %+v", p)
	}
}

func TestProfileDocument_MarkdownHeadings(t *testing.T) {
	doc := `# Title
Some intro text here.

## Section 1
Body of section 1.

## Section 2
Body of section 2.

### Subsection 2.1
Detail.

## Section 3
More body.`
	p := ProfileDocument(doc)
	if p.MdHeadingCounts[1] != 1 {
		t.Errorf("expected 1 H1, got %d", p.MdHeadingCounts[1])
	}
	if p.MdHeadingCounts[2] != 3 {
		t.Errorf("expected 3 H2, got %d", p.MdHeadingCounts[2])
	}
	if p.MdHeadingCounts[3] != 1 {
		t.Errorf("expected 1 H3, got %d", p.MdHeadingCounts[3])
	}
	if p.MdHeadingTotal != 5 {
		t.Errorf("expected 5 headings total, got %d", p.MdHeadingTotal)
	}
	if p.DominantHeadingLevel() != 2 {
		t.Errorf("dominant level should be 2 (≥3 occurrences), got %d", p.DominantHeadingLevel())
	}
}

func TestProfileDocument_DominantLevelFallback(t *testing.T) {
	// No level reaches 3 occurrences — should fall back to most frequent.
	doc := "# Single H1\n## H2 a\n## H2 b\n"
	p := ProfileDocument(doc)
	if p.DominantHeadingLevel() != 2 {
		t.Errorf("expected fallback to level 2 (most frequent), got %d", p.DominantHeadingLevel())
	}
}

func TestProfileDocument_NumberedSections(t *testing.T) {
	doc := `1. Introduction
text

2. Methodology
text

3. Results
text`
	p := ProfileDocument(doc)
	if p.NumberedSectionCount < 3 {
		t.Errorf("expected ≥3 numbered sections, got %d", p.NumberedSectionCount)
	}
}

func TestProfileDocument_GermanChapters(t *testing.T) {
	doc := "Kapitel 1: Einführung\n\nText\n\nKapitel 2: Hauptteil\n\nText"
	p := ProfileDocument(doc)
	if p.GermanChapterCount != 2 {
		t.Errorf("expected 2 German chapters, got %d", p.GermanChapterCount)
	}
}

func TestProfileDocument_ChineseChapters(t *testing.T) {
	doc := "第一章 引言\n\n内容\n\n第二章 方法\n\n内容"
	p := ProfileDocument(doc)
	if p.ChineseChapterCount != 2 {
		t.Errorf("expected 2 Chinese chapters, got %d", p.ChineseChapterCount)
	}
}

func TestProfileDocument_FormFeed(t *testing.T) {
	doc := "page 1 content\f\npage 2 content\f\npage 3 content"
	p := ProfileDocument(doc)
	if p.FormFeedCount != 2 {
		t.Errorf("expected 2 form feeds, got %d", p.FormFeedCount)
	}
}

func TestProfileDocument_DetectsCodeBlock(t *testing.T) {
	doc := "Some prose.\n\n```go\nfunc main() {}\n```\n\nMore prose."
	p := ProfileDocument(doc)
	if !p.HasCode {
		t.Error("expected HasCode=true for fenced block")
	}
}

func TestProfileDocument_DetectsTable(t *testing.T) {
	doc := "Intro.\n\n| col a | col b |\n| --- | --- |\n| 1 | 2 |\n"
	p := ProfileDocument(doc)
	if !p.HasTables {
		t.Error("expected HasTables=true")
	}
}

func TestProfileDocument_LineStatistics(t *testing.T) {
	doc := "short\nthis is a longer line of text\nanother line here"
	p := ProfileDocument(doc)
	if p.TotalLines != 3 {
		t.Errorf("expected 3 lines, got %d", p.TotalLines)
	}
	if p.AvgLineLen <= 0 {
		t.Error("expected positive avg line len")
	}
}

func TestSelectStrategy_HeadingDoc(t *testing.T) {
	doc := "# A\nbody\n## B\nbody\n## C\nbody\n## D\nbody"
	p := ProfileDocument(doc)
	chain := SelectStrategy(p)
	if chain[0] != TierHeading {
		t.Errorf("expected heading tier first, got %v", chain)
	}
}

func TestSelectStrategy_HeuristicDoc(t *testing.T) {
	doc := strings.Repeat("Kapitel 1: Foo\nbody body body\n\n", 1) +
		strings.Repeat("Kapitel 2: Bar\nbody body body\n\n", 1)
	p := ProfileDocument(doc)
	chain := SelectStrategy(p)
	// no markdown headings → heuristic must come first (heading tier skipped)
	if chain[0] != TierHeuristic {
		t.Errorf("expected heuristic tier first, got %v", chain)
	}
}

func TestSelectStrategy_PlainDoc(t *testing.T) {
	doc := "just a paragraph of plain text without any structure indicators at all here"
	p := ProfileDocument(doc)
	chain := SelectStrategy(p)
	if chain[0] != TierLegacy {
		t.Errorf("expected legacy tier first for unstructured doc, got %v", chain)
	}
}

func TestSelectStrategy_AlwaysFallsBackToLegacy(t *testing.T) {
	for _, doc := range []string{"", "simple", "# H1\nbody"} {
		p := ProfileDocument(doc)
		chain := SelectStrategy(p)
		if chain[len(chain)-1] != TierLegacy {
			t.Errorf("chain must end with legacy, got %v for doc=%q", chain, doc)
		}
	}
}

// --- Fenced-code state (CommonMark 0.31.2 §4.5), issue #3951 ---

// TestFenceState_Update exercises the shared fence scanner directly: both
// backtick and tilde fences, matching-length closing, fake closes, and the
// info-string restriction for backtick fences.
func TestFenceState_Update(t *testing.T) {
	cases := []struct {
		name      string
		lines     []string
		delims    []bool // expected update() return per line
		activeEnd bool   // expected fence.active after the last line
	}{
		{
			name:      "backtick open close",
			lines:     []string{"```go", "code", "```"},
			delims:    []bool{true, false, true},
			activeEnd: false,
		},
		{
			name:      "tilde open close",
			lines:     []string{"~~~markdown", "# not heading", "~~~"},
			delims:    []bool{true, false, true},
			activeEnd: false,
		},
		{
			name:      "four backtick fence keeps inner triple backtick literal",
			lines:     []string{"````go", "```go", "still code", "````"},
			delims:    []bool{true, false, false, true},
			activeEnd: false,
		},
		{
			name:      "closing fence rejects trailing text",
			lines:     []string{"```", "code", "``` fake close", "still code", "```"},
			delims:    []bool{true, false, false, false, true},
			activeEnd: false,
		},
		{
			name:      "closing fence must match opening character",
			lines:     []string{"```", "code", "~~~", "still code"},
			delims:    []bool{true, false, false, false},
			activeEnd: true,
		},
		{
			name:      "unclosed fence runs to end of document",
			lines:     []string{"```python", "# literal", "x = 1"},
			delims:    []bool{true, false, false},
			activeEnd: true,
		},
		{
			name:      "backtick fence rejects info string containing backtick",
			lines:     []string{"```js `x`", "prose"},
			delims:    []bool{false, false},
			activeEnd: false,
		},
		{
			name:      "two backticks never open a fence",
			lines:     []string{"`` `inline` ``", "prose"},
			delims:    []bool{false, false},
			activeEnd: false,
		},
		{
			name:      "tilde fence allows backticks in info string",
			lines:     []string{"~~~ `code`", "code", "~~~"},
			delims:    []bool{true, false, true},
			activeEnd: false,
		},
		{
			name:      "up to three leading spaces allowed, four are not",
			lines:     []string{"   ```", "    ```", "```"},
			delims:    []bool{true, false, true},
			activeEnd: false,
		},
		{
			name:      "shorter run never closes longer fence",
			lines:     []string{"~~~~", "```", "text"},
			delims:    []bool{true, false, false},
			activeEnd: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var f fenceState
			for i, line := range tc.lines {
				got := f.update(line)
				if i < len(tc.delims) && got != tc.delims[i] {
					t.Errorf("line %d (%q): update() = %v, want %v", i, line, got, tc.delims[i])
				}
			}
			if f.active != tc.activeEnd {
				t.Errorf("final active = %v, want %v", f.active, tc.activeEnd)
			}
		})
	}
}

// TestProfileDocument_TildeFenceIsCode is the issue #3951 repro: tilde
// fences were previously invisible to the scanner, so literal "#" lines
// inside them were profiled as headings and skewed strategy selection.
func TestProfileDocument_TildeFenceIsCode(t *testing.T) {
	doc := "# Real\n~~~markdown\n# Fake\n## Fake sub\n~~~\n# After\n## Real sub"
	p := ProfileDocument(doc)
	if p.MdHeadingTotal != 3 {
		t.Errorf("expected exactly 3 real headings (Real, After, Real sub), got %d", p.MdHeadingTotal)
	}
	if p.MdHeadingCounts[1] != 2 {
		t.Errorf("expected 2 real H1s (Real, After), got %d", p.MdHeadingCounts[1])
	}
	if p.MdHeadingCounts[2] != 1 {
		t.Errorf("expected 1 real H2 (Real sub), got %d", p.MdHeadingCounts[2])
	}
	if !p.HasCode {
		t.Error("tilde fence must set HasCode")
	}
	if p.CodeRatio <= 0 {
		t.Errorf("fenced content must count toward CodeRatio, got %f", p.CodeRatio)
	}
}

// TestProfileDocument_LongFenceNotClosedByInnerShortFence covers the second
// failure mode: a ``` line inside a ```` fence previously toggled the state
// off, leaking subsequent literal headings into the profile (and letting a
// fake-closing line with trailing text close the block early).
func TestProfileDocument_LongFenceNotClosedByInnerShortFence(t *testing.T) {
	doc := "# Real\n````\n# Fake\n```\nstill literal\n````\n# After"
	p := ProfileDocument(doc)
	if p.MdHeadingTotal != 2 || p.MdHeadingCounts[1] != 2 {
		t.Errorf("expected only Real and After as headings, got total=%d H1=%d", p.MdHeadingTotal, p.MdHeadingCounts[1])
	}
	if !p.HasCode {
		t.Error("expected HasCode=true")
	}
}

// TestProfileDocument_FakeCloseWithTrailingTextKeepsFenceOpen: a closing
// fence may be followed only by spaces/tabs; "``` trailing" is content.
func TestProfileDocument_FakeCloseWithTrailingTextKeepsFenceOpen(t *testing.T) {
	doc := "# Real\n```\ncode line\n``` fake close\n### Fake\n```\n# After"
	p := ProfileDocument(doc)
	if p.MdHeadingTotal != 2 || p.MdHeadingCounts[1] != 2 {
		t.Errorf("expected only Real and After as H1s, got total=%d H1=%d H3=%d",
			p.MdHeadingTotal, p.MdHeadingCounts[1], p.MdHeadingCounts[3])
	}
}

// TestProfileDocument_UnclosedFenceToEof: every line after an unterminated
// opening fence is literal, including trailing "#" lines.
func TestProfileDocument_UnclosedFenceToEof(t *testing.T) {
	doc := "# Real\n```python\n# Not a heading\n## Nor this\ntail = '```' stays literal"
	p := ProfileDocument(doc)
	if p.MdHeadingTotal != 1 || p.MdHeadingCounts[1] != 1 {
		t.Errorf("expected only the leading H1, got total=%d", p.MdHeadingTotal)
	}
	if !p.HasCode {
		t.Error("expected HasCode=true for unclosed fence")
	}
}
