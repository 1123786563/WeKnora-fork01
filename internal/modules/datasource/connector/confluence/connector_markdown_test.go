package confluence

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// markdownTestClient fails every HTTP request, so same-origin images that the
// asset resolver wants to inline keep their original relative src and exercise
// the absolute-ref rewrite that runs after the resolver.
func markdownTestClient() *client {
	return &client{
		cfg: config{baseURL: testBaseURL, username: "reader", secret: "secret"},
		http: &http.Client{Transport: roundTripper(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("no network in markdown tests")
		})},
	}
}

// convertPageHTML runs a page body through markdownItem and returns the
// emitted Markdown.
func convertPageHTML(t *testing.T, bodyHTML string) string {
	t.Helper()
	full := pageBody{}
	full.Body.View.Value = bodyHTML
	item, err := markdownItem(
		context.Background(), markdownTestClient(), "1",
		page{ID: "p1", Title: "Rich page"}, full,
	)
	if err != nil {
		t.Fatal(err)
	}
	if item.ContentType != "text/markdown" {
		t.Fatalf("content type = %q", item.ContentType)
	}
	return string(item.Content)
}

const richPageHTML = `<h1>Spec</h1>
<p>Some <del>old</del> and <s>stale</s> words.</p>
<p>Links: <a href="/wiki/spaces/ENG/pages/456">child page</a>, <a href="spaces/ENG/pages/9">bare relative</a>, <a href="/wiki/pages/1#Overview">anchored</a>, <a href="https://example.com/x?a=1&amp;b=2">external</a>, <a href="#section">anchor</a>.</p>
<table><thead><tr><th>Feature</th><th>Status</th></tr></thead><tbody><tr><td>Tables</td><td>ok</td></tr><tr><td>Nested <a href="/wiki/x">link</a></td><td><code>code</code></td></tr></tbody></table>
<p><img src="/wiki/download/attachments/1/diagram.png" alt="diagram"/></p>
<div class="wrapper" data-name="unused-parent"><svg aria-label="release flow diagram"><path d="M0 0"/></svg></div>
<div class="wrapper" data-name="mermaid-seq"><svg xmlns="http://www.w3.org/2000/svg"><circle r="1"/></svg></div>
<div class="wrapper"><svg xmlns="http://www.w3.org/2000/svg"><circle r="1"/></svg></div>
<pre><code>let x = 1;</code></pre>
`

func TestMarkdownItemPreservesTables(t *testing.T) {
	md := convertPageHTML(t, richPageHTML)
	separator := regexp.MustCompile(`(?m)^\|(?:-+\|)+$`)
	if !separator.MatchString(md) {
		t.Fatalf("markdown lost the table separator row:\n%s", md)
	}
	if row := tableRow(md, "Feature"); row == "" || !strings.Contains(row, "Status") {
		t.Fatalf("markdown lost the table header row:\n%s", md)
	}
	if row := tableRow(md, "Tables"); row == "" || !strings.Contains(row, "ok") {
		t.Fatalf("markdown lost the table body row:\n%s", md)
	}
	// A link nested inside a cell must keep both its text and absolute target.
	if row := tableRow(md, "Nested"); !strings.Contains(row, "[link](") || !strings.Contains(row, "https://confluence.test/wiki/x)") {
		t.Fatalf("markdown lost the link nested in a table cell: %q", row)
	}
	if strings.Contains(md, "FeatureStatus") {
		t.Fatalf("table was flattened into run-on text:\n%s", md)
	}
}

// tableRow returns the pipe-delimited row containing text, or "".
func tableRow(md, text string) string {
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "|") && strings.Contains(line, text) {
			return line
		}
	}
	return ""
}

func TestMarkdownItemPreservesStrikethrough(t *testing.T) {
	md := convertPageHTML(t, richPageHTML)
	for _, want := range []string{"~~old~~", "~~stale~~"} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown lost strikethrough %q:\n%s", want, md)
		}
	}
}

func TestMarkdownItemReplacesInlineSVGWithPlaceholder(t *testing.T) {
	md := convertPageHTML(t, richPageHTML)
	if strings.Contains(strings.ToLower(md), "<svg") {
		t.Fatalf("raw svg markup leaked into markdown:\n%s", md)
	}
	for _, want := range []string{"diagram: release flow diagram", "diagram: mermaid-seq", "diagram: 3"} {
		if !strings.Contains(md, want) {
			t.Fatalf("missing svg placeholder %q:\n%s", want, md)
		}
	}
}

func TestMarkdownItemAbsolutizesRelativeRefs(t *testing.T) {
	md := convertPageHTML(t, richPageHTML)
	relatives := []string{
		"(https://confluence.test/wiki/spaces/ENG/pages/456)",
		"(https://confluence.test/wiki/spaces/ENG/pages/9)",
		"(https://confluence.test/wiki/pages/1#Overview)",
		"(https://confluence.test/wiki/download/attachments/1/diagram.png)",
	}
	for _, want := range relatives {
		if !strings.Contains(md, want) {
			t.Fatalf("relative reference was not absolutized to %q:\n%s", want, md)
		}
	}
	absolutes := []string{
		"(https://example.com/x?a=1&b=2)",
		"(#section)",
	}
	for _, want := range absolutes {
		if !strings.Contains(md, want) {
			t.Fatalf("already-absolute reference %q did not survive untouched:\n%s", want, md)
		}
	}
}

func TestMarkdownItemKeepsBodyStructure(t *testing.T) {
	md := convertPageHTML(t, richPageHTML)
	if !strings.Contains(md, "# Spec") {
		t.Fatalf("heading was lost:\n%s", md)
	}
	if !strings.Contains(md, "```") || !strings.Contains(md, "let x = 1;") {
		t.Fatalf("code block was lost:\n%s", md)
	}
}

func TestMarkdownItemFallsBackToTitleHeading(t *testing.T) {
	if md := convertPageHTML(t, "   "); md != "# Rich page\n" {
		t.Fatalf("empty body fallback = %q", md)
	}
}
