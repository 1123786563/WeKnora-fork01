package publish

import (
	"fmt"
	"html"
	"strings"
)

// ConfluenceStorageBody derives the Confluence storage-format (XHTML)
// body from plain text: paragraphs split on blank lines, each trimmed and
// HTML-escaped, wrapped in <p> elements separated by newlines (a single
// newline inside a paragraph stays in place — Confluence renders it as
// whitespace, which matches the plain-text semantics). The output is
// deterministic pure derivation — the same artifact bytes always produce
// the same storage string, so the approval digest pins exactly what will
// be sent.
//
// The content-too-large gate counts the raw blank-line-separated parts
// (a trailing separator yields a final empty part, which still consumed a
// split boundary), so "p\n\n" repeated MaxPublishBlocks times is refused
// as MaxPublishBlocks+1 paragraphs; rendered <p> elements only ever come
// from non-blank paragraphs.
func ConfluenceStorageBody(text string) (string, error) {
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
		return "", ErrPublishEmptyContent
	}
	if len(parts) > MaxPublishBlocks {
		return "", fmt.Errorf("%w: %d paragraphs exceed %d blocks", ErrPublishContentTooLarge, len(parts), MaxPublishBlocks)
	}
	var b strings.Builder
	for i, p := range paragraphs {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("<p>")
		b.WriteString(html.EscapeString(p))
		b.WriteString("</p>")
	}
	return b.String(), nil
}
