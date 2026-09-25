// Package plugins implements the weknora.plugin/1 manifest protocol: schema
// validation, canonical digests, SSRF-safe manifest fetching, and verification
// of the manifest's declared tool directory against the live MCP endpoint.
package plugins

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/utils"
)

const (
	// PluginProtocolV1 is the only manifest protocol version accepted today.
	PluginProtocolV1 = "weknora.plugin/1"

	// maxPluginNameRunes bounds the human-readable plugin name.
	maxPluginNameRunes = 255

	// maxPluginIDLen bounds the plugin_id total length.
	maxPluginIDLen = 128

	// maxToolNameLen bounds one tools[].name.
	maxToolNameLen = 128

	// maxDescriptionRunes bounds free-text descriptions (manifest-level and
	// live tool descriptions entering the snapshot).
	maxDescriptionRunes = 1024

	// maxScopesPerList bounds one scopes array (auth-level or per-tool).
	maxScopesPerList = 64

	// maxScopeLen bounds a single scope token, in bytes.
	maxScopeLen = 128

	// maxVerificationProblems bounds how many discrepancies the
	// BuildVerifiedSnapshot rejection message lists before collapsing the
	// rest into a counter — the live tool directory is untrusted remote
	// data and can be arbitrarily large, so the single joined error must
	// stay bounded.
	maxVerificationProblems = 32

	// maxLiveTools caps how many live ListTools entries BuildVerifiedSnapshot
	// is willing to process. The live directory is untrusted remote data with
	// no transport-level size bound; without a cap the verification maps,
	// snapshot construction and SnapshotDigest are all O(n) on hostile input
	// (the manifest side is bounded by the 1MiB download cap).
	maxLiveTools = 1024

	// maxEndpointRunes mirrors the endpoint_url varchar(512) schema bound
	// (OCR R1 F45): ValidateManifest enforces it so every manifest consumer
	// rejects oversized endpoints deterministically instead of failing at
	// the first DB write (PG value-too-long → misreported 500) or silently
	// storing oversized data on SQLite.
	maxEndpointRunes = 512

	// maxEchoRunes bounds how much of an untrusted string is echoed back in
	// a validation error before the remainder collapses into a count — a
	// hostile manifest can carry near-1MiB fields past the length checks
	// that would reject them, and %q would balloon the single error message
	// (which reaches the HTTP response) accordingly.
	maxEchoRunes = 64
)

// echoQuoted quotes s like %q but truncates first: at most maxEchoRunes
// runes are shown, then "…(+N more chars)". Use it ONLY for untrusted
// strings that have not yet passed a length check at the point of the
// error (protocol, plugin_id, version, transport type, scope token);
// already-validated names (≤128 runes, no control/format characters) are
// echoed in full for review clarity.
func echoQuoted(s string) string {
	if utf8.RuneCountInString(s) <= maxEchoRunes {
		return fmt.Sprintf("%q", s)
	}
	runes := []rune(s)
	return fmt.Sprintf("%q…(+%d more chars)", string(runes[:maxEchoRunes]), len(runes)-maxEchoRunes)
}

var (
	// pluginIDPattern: lowercase segments separated by single dots/hyphens,
	// must start and end with a letter or digit — no leading, trailing or
	// consecutive separators. Total length 3..maxPluginIDLen (checked
	// separately; the regex itself is length-open).
	pluginIDPattern = regexp.MustCompile(`^[a-z0-9]+([.-][a-z0-9]+)*$`)

	// pluginVersionPattern: MAJOR.MINOR.PATCH, no leading zeros, each
	// component at most 9 digits (so "01.2.0" and unbounded numeric fields
	// are rejected).
	pluginVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$`)

	// schemaDigestPattern: 64 lowercase hex chars (canonical-JSON SHA-256),
	// the same convention as OCSchemaDigest and MCPConfigFingerprint.
	schemaDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

	// scopeTokenPattern: RFC 6749 scope-token charset — printable ASCII
	// excluding '"' (%x22) and '\' (%x5C) — with a length cap derived from
	// maxScopeLen so the bound and the error message cannot drift apart.
	scopeTokenPattern = regexp.MustCompile(
		fmt.Sprintf(`^[\x21\x23-\x5B\x5D-\x7E]{1,%d}$`, maxScopeLen),
	)

	// pluginTransportTypes lists the transports a plugin endpoint may use.
	// stdio is excluded by spec: a plugin is always a remote MCP service.
	pluginTransportTypes = map[string]bool{
		"http-streamable": true,
		"sse":             true,
	}
)

// ValidateManifest checks a weknora.plugin/1 document for protocol
// conformance. It is a pure function: the endpoint's network posture (SSRF,
// reachability) is deliberately NOT checked here — that happens in
// FetchAndVerify before any request is sent.
func ValidateManifest(m *types.PluginManifest) error {
	if m == nil {
		return fmt.Errorf("manifest is required")
	}
	if m.Protocol != PluginProtocolV1 {
		return fmt.Errorf("unsupported manifest protocol %s (expected %q)", echoQuoted(m.Protocol), PluginProtocolV1)
	}
	if !pluginIDPattern.MatchString(m.PluginID) || len(m.PluginID) < 3 || len(m.PluginID) > maxPluginIDLen {
		return fmt.Errorf("invalid plugin_id %s (lowercase segments separated by single dots/hyphens, 3..%d chars, no leading/trailing/consecutive separators)", echoQuoted(m.PluginID), maxPluginIDLen)
	}
	if !pluginVersionPattern.MatchString(m.Version) {
		return fmt.Errorf("invalid version %s (must be MAJOR.MINOR.PATCH without leading zeros)", echoQuoted(m.Version))
	}
	if err := validateName("name", m.Name, maxPluginNameRunes); err != nil {
		return err
	}
	if err := validateDescription("description", m.Description); err != nil {
		return err
	}
	if !pluginTransportTypes[m.Transport.Type] {
		return fmt.Errorf("unsupported transport type %s (allowed: http-streamable, sse)", echoQuoted(m.Transport.Type))
	}
	// Structural endpoint check only (scheme + host). The full SSRF verdict
	// (localhost/loopback/private/reserved rejection) is FetchAndVerify's job.
	endpoint, err := url.Parse(m.Transport.Endpoint)
	if err != nil {
		// Never pass *url.Error through: its message embeds the FULL
		// original URL (remote-controlled, up to ~maxManifestBytes), which
		// would bypass this file's bounded-echo discipline on its way into
		// the admin-facing 400 response (OCR T01-R4-F3). Strip the wrapper
		// and echo the endpoint truncated instead. The inner reason is NOT a
		// fixed-size message family (OCR round-1 F3, review-measured 900KB):
		// parseHost's `invalid port %q after host` embeds everything after
		// the authority's last colon — bound it with echoQuoted too.
		//
		// The echoed endpoint is masked FIRST (OCR round-1 R12 F03): a
		// malformed URL like https://ci-bot:s3cr3t@host:badport/ fails url.Parse
		// BEFORE the userinfo rejection below, so the raw echo would leak the
		// very credentials the well-formed path deliberately hides.
		reason := err
		var uerr *url.Error
		if errors.As(err, &uerr) {
			reason = uerr.Err
		}
		masked := maskEndpointCredentials(m.Transport.Endpoint)
		maskedReason := reason.Error()
		// OCR R1 F12: userinfoOf returns "" for credential-free endpoints and
		// strings.ReplaceAll(s, "", x) inserts x after EVERY rune — guard the
		// replace so the most common malformed form (a bad port on a clean
		// URL) keeps a readable message instead of "REDACTEDiREDACTEDn...".
		if userinfo := userinfoOf(m.Transport.Endpoint); userinfo != "" {
			maskedReason = strings.ReplaceAll(maskedReason, userinfo, "REDACTED")
		}
		return fmt.Errorf("invalid transport endpoint %s: %s", echoQuoted(masked), echoQuoted(maskedReason))
	}
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return fmt.Errorf("transport endpoint scheme must be http or https, got %s", echoQuoted(endpoint.Scheme))
	}
	if endpoint.Host == "" {
		return fmt.Errorf("transport endpoint must include a host")
	}
	// OCR R1 F45: the endpoint lands in endpoint_url varchar(512) and the
	// materialized service URL. Validating here covers EVERY consumer of a
	// fetched/declared manifest (install preview, upgrade preview, upgrade
	// accept) before any SSRF probe or write, so an oversized declaration is
	// a deterministic 4xx (ErrPluginVerifyFailed via FetchAndVerify) rather
	// than a PG value-too-long misreported as 500.
	if utf8.RuneCountInString(m.Transport.Endpoint) > maxEndpointRunes {
		return fmt.Errorf("transport endpoint exceeds %d characters", maxEndpointRunes)
	}
	// Reject userinfo-embedded credentials (https://user:pass@host/mcp):
	// ValidateURLForSSRF never looks at u.User (Hostname() strips it), so
	// without this check the credential-bearing URL would be persisted to
	// plugin_previews, echoed on the admin review surface, and sent as Basic
	// Auth by the Go http client. Human review is this feature's core safety
	// gate — credentials must not appear there (OCR T01-R1-F3). The message
	// deliberately does not echo the URL, which contains the credentials.
	if endpoint.User != nil {
		return fmt.Errorf("transport endpoint must not embed userinfo credentials")
	}
	if len(m.Tools) == 0 {
		return fmt.Errorf("manifest must declare at least one tool")
	}
	seen := make(map[string]bool, len(m.Tools))
	for i, tool := range m.Tools {
		if err := validateName(fmt.Sprintf("tools[%d].name", i), tool.Name, maxToolNameLen); err != nil {
			return err
		}
		if seen[tool.Name] {
			return fmt.Errorf("duplicate tool name %q", tool.Name)
		}
		seen[tool.Name] = true
		if !schemaDigestPattern.MatchString(tool.InputSchemaDigest) {
			return fmt.Errorf("tool %q input_schema_digest must be 64 lowercase hex chars (canonical-JSON SHA-256)", tool.Name)
		}
		if tool.RequiresPersonalAuth && (m.Auth == nil || !m.Auth.PersonalOAuth) {
			return fmt.Errorf("tool %q requires personal auth but manifest does not declare auth.personal_oauth", tool.Name)
		}
		if err := validateScopes(tool.Scopes, fmt.Sprintf("tool %q", tool.Name)); err != nil {
			return err
		}
	}
	if m.Auth != nil {
		if err := validateScopes(m.Auth.Scopes, "auth"); err != nil {
			return err
		}
	}
	return nil
}

// validateName bounds an identifier-like name: non-empty, at most maxRunes
// runes, and free of control AND invisible format characters. Format (Cf)
// characters — bidi overrides (U+202E), zero-width marks (U+200B), BOM
// (U+FEFF), soft hyphen (U+00AD) — are not covered by unicode.IsControl (Cc
// only) yet can visually reorder or hide text in the admin review surface;
// human review is this feature's core safety gate, so names must be exactly
// what they appear to be. Private-use (Co) characters have no standardized
// glyph and are rejected from identifiers as well. Line/paragraph separators
// (Zl/Zp, U+2028/U+2029) are rejected too: they pass none of the classes
// above yet inject line breaks into the admin review surface (整分支 OCR
// 二轮 F2). The rejection message names every rejected class — an admin
// debugging a rejected private-use rune must not be told "format".
func validateName(where, name string, maxRunes int) error {
	if name == "" {
		return fmt.Errorf("%s must not be empty", where)
	}
	if utf8.RuneCountInString(name) > maxRunes {
		return fmt.Errorf("%s must be at most %d characters", where, maxRunes)
	}
	for _, r := range name {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Co, r) ||
			unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r) {
			return fmt.Errorf("%s must not contain control, format, private-use or line/paragraph separator characters (U+%04X)", where, r)
		}
		// OCR R1 F16: '/' and '%' are legal for the general Unicode hygiene
		// above but break addressability — the per-tool policy endpoint
		// /tools/:tool_name/policy matches ONE decoded path segment, so a
		// snapshot tool named "a/b" (or smuggled "a%2Fb") can never be
		// governed. Rejecting them here makes "every snapshot tool is
		// addressable through the policy endpoint" an install-time invariant
		// for both manifest-declared and live directory names.
		if r == '/' || r == '%' {
			return fmt.Errorf("%s must not contain path separator or percent characters (/ or %%; U+%04X) — per-tool policy endpoints address tools by a single URL path segment", where, r)
		}
	}
	return nil
}

// validateDescription bounds free text (manifest description, live tool
// descriptions). Empty is legal (the field is optional). Newlines, tabs and
// CR are allowed — multi-line descriptions are legitimate — but every other
// control character AND every format (Cf) character is rejected: bidi
// overrides and zero-width marks would visually reorder or alter the text
// shown to admins and members. Two exceptions (final-review R5 F9, round-3
// finding): U+200C (ZWNJ) and U+200D (ZWJ) are mandatory parts of legitimate
// compound emoji sequences (family, mixed-skin-tone handshake) and of some
// orthographies (Persian); blanket Cf rejection denied benign
// remote-controlled descriptions at the install gate with no way for the
// admin to fix them. Residual trade-off, recorded here: a ZWJ/ZWNJ between
// ASCII letters can still visually hide text in the review surface — names
// (validateName) keep rejecting ALL Cf/Co/Zl/Zp, and this exemption is
// limited to these two runes in free text. Private-use (Co) characters keep
// their ordinary visible glyphs and stay legal in free text. Shared by the
// manifest validator and BuildVerifiedSnapshot, so live endpoint data passes
// the same hygiene the manifest does.
func validateDescription(where, description string) error {
	if utf8.RuneCountInString(description) > maxDescriptionRunes {
		return fmt.Errorf("%s exceeds %d characters", where, maxDescriptionRunes)
	}
	for _, r := range description {
		if r != '\n' && r != '\r' && r != '\t' && r != '\u200c' && r != '\u200d' &&
			(unicode.IsControl(r) || unicode.Is(unicode.Cf, r)) {
			return fmt.Errorf("%s must not contain control or format characters (U+%04X)", where, r)
		}
	}
	return nil
}

// validateScopes checks one scopes array: each entry must be a single RFC
// 6749 scope token (printable ASCII minus quote and backslash, ≤128 bytes),
// the list is length-bounded, and duplicates are rejected — scopes flow into
// per-tool snapshots and member authorization surfaces, so hostile filler
// must not pass review gates.
func validateScopes(scopes []string, where string) error {
	if len(scopes) > maxScopesPerList {
		return fmt.Errorf("%s scopes exceed %d entries", where, maxScopesPerList)
	}
	seen := make(map[string]bool, len(scopes))
	for _, scope := range scopes {
		if !scopeTokenPattern.MatchString(scope) {
			return fmt.Errorf("%s scope %s must be 1..%d chars from the RFC 6749 scope-token charset (printable ASCII, no quotes/backslashes)", where, echoQuoted(scope), maxScopeLen)
		}
		if seen[scope] {
			return fmt.Errorf("%s contains duplicate scope %q", where, scope)
		}
		seen[scope] = true
	}
	return nil
}

// The canonical-JSON digest contract lives in the platform package
// internal/utils (issue #106 gate regression fix): agentruntime's runtime
// drift guard re-digests live tool schemas with the EXACT install-side
// algorithm, and a module-to-module import is forbidden by
// architectureguard — so the shared seam sits in the platform layer both
// modules may import (composition-root/shared-platform pattern, cf. 7d30d6fe6).
// The wrappers below keep this package's public API and protocol entry
// points (manifest developers are told to reproduce digests via
// plugins.ToolSchemaDigest/ManifestContentDigest) unchanged.

// CanonicalJSON re-encodes v into the canonical form used by every
// weknora.plugin/1 digest. Algorithm contract: see utils.CanonicalJSON
// (sorted keys, dropped whitespace, verbatim number literals — NOT JCS;
// U+2028/U+2029 always escaped).
func CanonicalJSON(v any) []byte {
	return utils.CanonicalJSON(v)
}

// canonicalizeJSON canonicalizes an already-encoded JSON document; trailing
// non-whitespace content yields nil (see utils.CanonicalizeJSON).
func canonicalizeJSON(raw []byte) []byte {
	return utils.CanonicalizeJSON(raw)
}

func sha256Hex(data []byte) string {
	return utils.SHA256Hex(data)
}

// ToolSchemaDigest returns the canonical-JSON SHA-256 of a tool input schema,
// as 64 lowercase hex chars — the same digest convention as
// OCSchemaDigest/MCPConfigFingerprint. Key order and whitespace in the input
// do not affect the result; number literals are preserved verbatim (NOT JCS
// — see CanonicalJSON). Empty or invalid JSON (including trailing garbage)
// yields "" so it can never accidentally equal a declared digest.
func ToolSchemaDigest(schema []byte) string {
	return utils.ToolSchemaDigest(schema)
}

// ManifestContentDigest digests the manifest's semantic content with the
// self-referential content_digest field excluded, so the digest can be
// embedded in the document it certifies. Uses the CanonicalJSON algorithm
// (sorted keys, no whitespace, verbatim number literals — see CanonicalJSON).
//
// DIGEST DOMAIN — structural, not document-level (protocol contract): the
// input is the parsed PluginManifest struct, so the digest covers exactly
// the semantic fields WeKnora consumes. Explicitly-empty optional fields
// ("description": "", "scopes": [], "auth": null) and UNKNOWN extension
// keys present in the raw document do NOT participate — they are invisible
// to the struct round-trip. This differs from ToolSchemaDigest, which is a
// document-level digest of the raw schema JSON. A manifest whose raw JSON
// carries extra keys or explicit empties digests the same as its semantic
// twin, and a content_digest computed per these rules verifies cleanly
// (locked by TestManifestContentDigestIgnoresUnknownAndEmptyOptionalFields).
// Precomputing: parse the document into the manifest shape above, drop
// content_digest, then apply the CanonicalJSON algorithm.
// An unserializable input yields "" (the ToolSchemaDigest failure
// convention), never the valid SHA-256 of empty input that would make two
// different failures compare equal.
func ManifestContentDigest(m *types.PluginManifest) string {
	if m == nil {
		return ""
	}
	copyM := *m
	copyM.ContentDigest = ""
	canonical := CanonicalJSON(&copyM)
	if canonical == nil {
		return ""
	}
	return sha256Hex(canonical)
}

// SnapshotDigest digests a verified tool snapshot list. The slice order is
// significant (it is the reviewed directory order); object key order is not.
// An unserializable input yields "" (the ToolSchemaDigest failure
// convention).
func SnapshotDigest(tools []types.PluginToolSnapshot) string {
	canonical := CanonicalJSON(tools)
	if canonical == nil {
		return ""
	}
	return sha256Hex(canonical)
}

// IdentityFingerprint binds a plugin identity to the exact verified tool set:
// sha256(pluginID \x00 version \x00 endpoint \x00 toolsDigest), hex-encoded.
// It is what the tenant actually accepted — used later to reject stale or
// replayed previews whose remote truth has moved on.
func IdentityFingerprint(pluginID, version, endpoint string, toolsDigest string) string {
	h := sha256.New()
	h.Write([]byte(pluginID))
	h.Write([]byte{0})
	h.Write([]byte(version))
	h.Write([]byte{0})
	h.Write([]byte(endpoint))
	h.Write([]byte{0})
	h.Write([]byte(toolsDigest))
	return hex.EncodeToString(h.Sum(nil))
}

// userinfoOf extracts the raw "user[:pass]@" prefix of a URL's authority, or
// "" when the URL has none (no scheme delimiter, no '@', or a malformed tail
// where the split is ambiguous — failing open to "" makes the caller's
// masking a no-op, never a panic or a wrong redaction).
func userinfoOf(rawURL string) string {
	schemeEnd := strings.Index(rawURL, "://")
	rest := rawURL
	if schemeEnd >= 0 {
		rest = rawURL[schemeEnd+3:]
	}
	authority := rest
	if slash := strings.Index(rest, "/"); slash >= 0 {
		authority = rest[:slash]
	}
	if at := strings.LastIndex(authority, "@"); at > 0 {
		return authority[:at+1]
	}
	return ""
}

// maskEndpointCredentials replaces any userinfo in a URL's authority with
// "REDACTED@" before the URL is echoed into an admin-visible error (OCR
// round-1 R12 F03): a malformed URL fails url.Parse before the userinfo
// rejection fires, so the raw echo would leak the very credentials the
// well-formed path deliberately hides.
func maskEndpointCredentials(rawURL string) string {
	schemeEnd := strings.Index(rawURL, "://")
	prefix, rest := "", rawURL
	if schemeEnd >= 0 {
		prefix, rest = rawURL[:schemeEnd+3], rawURL[schemeEnd+3:]
	}
	authority, tail := rest, ""
	if slash := strings.Index(rest, "/"); slash >= 0 {
		authority, tail = rest[:slash], rest[slash:]
	}
	at := strings.LastIndex(authority, "@")
	if at <= 0 {
		return rawURL
	}
	return prefix + "REDACTED@" + authority[at+1:] + tail
}
