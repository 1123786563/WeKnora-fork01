package service

import (
	"fmt"
	stdhtml "html"
	"regexp"
	"strings"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	xhtml "golang.org/x/net/html"
)

// Server-side re-screening of staged web HTML members (round-3 OCR build-log
// trust finding). The sandboxed build toolchain screens fragments in
// render_html before emitting them, but build-log.json and index.html live
// in the Agent-writable output tree with world-readable toolchain digests:
// a forged exit_code=0 build log plus a hand-written index.html used to
// reach promotion without ANY screening. This port of the toolchain's
// denylist closes that path server-side — the collection itself refuses
// HTML members carrying script/navigation/egress shapes, whatever the build
// log claims. (The build-log EXIT CODE remains a self-attested fact; that
// residual is recorded in the ledger.)

var (
	craftScreenExternalURLRe = regexp.MustCompile(`(?i)(https?:)?//[^\s"'<>` + "`" + `]|://`)
	craftScreenAbsoluteRefRe = regexp.MustCompile(`(?i)(?:src|href|action|poster)\s*=\s*["']?\s*(?:/[a-zA-Z]|[a-zA-Z][a-zA-Z0-9+.-]*:)`)
	craftScreenActiveDataRe  = regexp.MustCompile(`(?i)data:text/html|javascript:`)
	craftScreenCSSFetchRe    = regexp.MustCompile(`(?i)url\(|@import`)
	craftScreenEmbedTagRe    = regexp.MustCompile(`(?i)<\s*(base|iframe|object|embed|form|script|meta)\b`)
	craftScreenCSSEscapeRe   = regexp.MustCompile(`\\([0-9a-fA-F]{1,6})[ \t\r\n\f]?`)
)

// craftScreenCSSEscape resolves CSS backslash hex escapes once (\75 rl -> url).
func craftScreenCSSEscape(text string) string {
	return craftScreenCSSEscapeRe.ReplaceAllStringFunc(text, func(match string) string {
		hexRunes := strings.TrimRight(match[1:], " \t\r\n\f")
		var code rune
		if _, err := fmt.Sscanf(hexRunes, "%x", &code); err != nil {
			return match
		}
		if code == 0 || code > 0x10FFFF || (code >= 0xD800 && code <= 0xDFFF) {
			return match
		}
		return string(code)
	})
}

// craftScreenHTMLVariants produces the decode closure (HTML entities × CSS
// escapes) over one fragment, bounded and fail-closed exactly like the
// toolchain's build.py: unexplored variants refuse the member.
func craftScreenHTMLVariants(fragment string) ([]string, error) {
	variants := make([]string, 0, 8)
	queue := []string{fragment}
	for len(queue) > 0 && len(variants) < 64 {
		candidate := queue[0]
		queue = queue[1:]
		if slicesContains(variants, candidate) {
			continue
		}
		variants = append(variants, candidate)
		queue = append(queue, stdhtml.UnescapeString(candidate))
		queue = append(queue, craftScreenCSSEscape(candidate))
	}
	if len(queue) > 0 {
		return nil, fmt.Errorf("fragment nests too many encodings for the server-side screen")
	}
	return variants, nil
}

func slicesContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// craftScreenHasEventAttrs structurally detects on* attributes with the real
// HTML tokenizer: quoted attribute values containing ">", "/" as the
// tag/attribute separator and entity decoding are all handled.
func craftScreenHasEventAttrs(variant string) bool {
	tokenizer := xhtml.NewTokenizer(strings.NewReader(variant))
	for {
		switch tokenizer.Next() {
		case xhtml.ErrorToken:
			return false
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			for _, attr := range tokenizer.Token().Attr {
				if strings.HasPrefix(strings.ToLower(attr.Key), "on") {
					return true
				}
			}
		}
	}
}

// craftScreenWebHTMLMember applies the full denylist to one staged HTML
// member. A violation is a round-level ErrInvalidInput: nothing is uploaded.
func craftScreenWebHTMLMember(rel string, data []byte) error {
	variants, err := craftScreenHTMLVariants(string(data))
	if err != nil {
		return fmt.Errorf("%w: web member %q: %v", craft.ErrInvalidInput, rel, err)
	}
	for _, variant := range variants {
		for pattern, why := range map[*regexp.Regexp]string{
			craftScreenExternalURLRe: "an external URL",
			craftScreenAbsoluteRefRe: "an absolute or scheme reference",
			craftScreenActiveDataRe:  "an active data/javascript URI",
			craftScreenCSSFetchRe:    "a css url()/@import fetch",
			craftScreenEmbedTagRe:    "an embedding/script/navigation tag",
		} {
			if pattern.MatchString(variant) {
				return fmt.Errorf("%w: web member %q contains %s: offline local assets only", craft.ErrInvalidInput, rel, why)
			}
		}
		if craftScreenHasEventAttrs(variant) {
			return fmt.Errorf("%w: web member %q contains an inline event handler attribute", craft.ErrInvalidInput, rel)
		}
	}
	_ = craft.KindWeb
	return nil
}

// craftScreenWebMemberReportsHTML reports whether a staged member is HTML.
func craftScreenWebMemberIsHTML(rel string) bool {
	lower := strings.ToLower(rel)
	return strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".htm")
}
