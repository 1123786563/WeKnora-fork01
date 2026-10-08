package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	stdhtml "html"
	"regexp"
	"slices"
	"strings"

	"github.com/Tencent/WeKnora/internal/craft"
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
		if slices.Contains(variants, candidate) {
			continue
		}
		variants = append(variants, candidate)
		queue = append(queue, stdhtml.UnescapeString(candidate))
		queue = append(queue, craftScreenCSSEscape(candidate))
		queue = append(queue, craftScreenControlCharsRe.ReplaceAllString(candidate, ""))
	}
	if len(queue) > 0 {
		return nil, fmt.Errorf("fragment nests too many encodings for the server-side screen")
	}
	return variants, nil
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

// craftScreenTemplateShellLiterals are the EXACT template constructs the
// pinned template.html (toolchain.lock.json sha256-pinned, a trusted face)
// contributes to every legitimate index.html: the two meta tags and the one
// offline asset script reference. They are stripped BEFORE the full denylist
// runs, so the screen denies INJECTED meta/script tags while legitimate
// builds pass — the denylist itself never learns about them.
var craftScreenTemplateShellLiterals = []string{
	`<meta charset="utf-8">`,
	`<meta name="viewport" content="width=device-width, initial-scale=1">`,
	`<script src="assets/craft-web.js"></script>`,
}

// craftScreenPinnedAssetDigest is the sha256 of the offline runtime asset
// the pinned template references (assets/craft-web.js). An index.html may
// claim the template shell literal, but the STAGED asset member itself must
// byte-match the pinned digest — an Agent-planted replacement script with
// a clean HTML page would otherwise execute on the preview origin.
const craftScreenPinnedAssetDigest = "5b930dd96b33bf707653ed6c1b5dc294cb8a790ee7ba7b6adfd13d65a0130064"

// craftScreenPinnedAssetPath is the output-relative member the template
// shell references; its digest is verified per staged round.
const craftScreenPinnedAssetPath = "assets/craft-web.js"

// craftScreenControlChars matches the ASCII C0 controls plus DEL: browsers
// strip these from URLs BEFORE scheme parsing (WHATWG URL Standard), so
// "jav\tascript:" must be screened in its stripped form too.
var craftScreenControlCharsRe = regexp.MustCompile("[\x00-\x1f\x7f]")

// craftScreenVerifyPinnedAsset enforces the template-shell exemption's
// other half: when an HTML member referenced the pinned script literal, the
// staged assets/craft-web.js member must hash to the pinned digest. A
// mismatched (planted) asset refuses the round.
func craftScreenVerifyPinnedAsset(rel string, data []byte) error {
	if rel != craftScreenPinnedAssetPath {
		return nil
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != craftScreenPinnedAssetDigest {
		return fmt.Errorf("%w: staged %s does not match the pinned toolchain asset digest", craft.ErrInvalidInput, rel)
	}
	return nil
}

// craftScreenWebHTMLMember applies the full denylist to one staged HTML
// member. A violation is a round-level ErrInvalidInput: nothing is uploaded.
func craftScreenWebHTMLMember(rel string, data []byte) error {
	page := string(data)
	for _, literal := range craftScreenTemplateShellLiterals {
		page = strings.ReplaceAll(page, literal, "")
	}
	variants, err := craftScreenHTMLVariants(page)
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
	return nil
}

// craftScreenWebMemberReportsHTML reports whether a staged member is HTML.
func craftScreenWebMemberIsHTML(rel string) bool {
	lower := strings.ToLower(rel)
	// Browsers parse and execute inline script/handlers in all of these when
	// navigated directly — the screen must cover them, not just .html.
	switch {
	case strings.HasSuffix(lower, ".html"), strings.HasSuffix(lower, ".htm"),
		strings.HasSuffix(lower, ".svg"), strings.HasSuffix(lower, ".xhtml"), strings.HasSuffix(lower, ".xht"):
		return true
	}
	return false
}
