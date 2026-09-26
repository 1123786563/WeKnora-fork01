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

func TestConfluenceStorageBodyDeterministic(t *testing.T) {
	a, _ := ConfluenceStorageBody("x\n\ny\n\nz")
	b, _ := ConfluenceStorageBody("x\n\ny\n\nz")
	if a != b {
		t.Fatalf("the same artifact bytes must always produce the same storage string: %q vs %q", a, b)
	}
}
