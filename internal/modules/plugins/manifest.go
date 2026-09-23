// Package plugins implements the weknora.plugin/1 manifest protocol: schema
// validation, canonical digests, SSRF-safe manifest fetching, and verification
// of the manifest's declared tool directory against the live MCP endpoint.
package plugins

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"unicode"
	"unicode/utf8"

	"github.com/Tencent/WeKnora/internal/types"
)

const (
	// PluginProtocolV1 is the only manifest protocol version accepted today.
	PluginProtocolV1 = "weknora.plugin/1"

	// maxPluginNameRunes bounds the human-readable plugin name.
	maxPluginNameRunes = 255

	// maxPluginIDLen bounds the plugin_id total length.
	maxPluginIDLen = 128

	// maxScopesPerList bounds one scopes array (auth-level or per-tool).
	maxScopesPerList = 64

	// maxScopeLen bounds a single scope token, in bytes.
	maxScopeLen = 128
)

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
	// excluding '"' (%x22) and '\' (%x5C) — with a length cap.
	scopeTokenPattern = regexp.MustCompile(`^[\x21\x23-\x5B\x5D-\x7E]{1,128}$`)

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
		return fmt.Errorf("unsupported manifest protocol %q (expected %q)", m.Protocol, PluginProtocolV1)
	}
	if !pluginIDPattern.MatchString(m.PluginID) || len(m.PluginID) < 3 || len(m.PluginID) > maxPluginIDLen {
		return fmt.Errorf("invalid plugin_id %q (lowercase segments separated by single dots/hyphens, 3..%d chars, no leading/trailing/consecutive separators)", m.PluginID, maxPluginIDLen)
	}
	if !pluginVersionPattern.MatchString(m.Version) {
		return fmt.Errorf("invalid version %q (must be MAJOR.MINOR.PATCH without leading zeros)", m.Version)
	}
	if err := validateDisplayName(m.Name); err != nil {
		return err
	}
	if !pluginTransportTypes[m.Transport.Type] {
		return fmt.Errorf("unsupported transport type %q (allowed: http-streamable, sse)", m.Transport.Type)
	}
	// Structural endpoint check only (scheme + host). The full SSRF verdict
	// (localhost/loopback/private/reserved rejection) is FetchAndVerify's job.
	endpoint, err := url.Parse(m.Transport.Endpoint)
	if err != nil {
		return fmt.Errorf("invalid transport endpoint: %w", err)
	}
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return fmt.Errorf("transport endpoint scheme must be http or https, got %q", endpoint.Scheme)
	}
	if endpoint.Host == "" {
		return fmt.Errorf("transport endpoint must include a host")
	}
	if len(m.Tools) == 0 {
		return fmt.Errorf("manifest must declare at least one tool")
	}
	seen := make(map[string]bool, len(m.Tools))
	for i, tool := range m.Tools {
		if tool.Name == "" {
			return fmt.Errorf("tools[%d].name must not be empty", i)
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

// validateDisplayName bounds the human-readable name: 1..maxPluginNameRunes
// runes and no control characters (they would corrupt admin-facing display
// surfaces and logs).
func validateDisplayName(name string) error {
	if name == "" || utf8.RuneCountInString(name) > maxPluginNameRunes {
		return fmt.Errorf("name must be 1..%d characters", maxPluginNameRunes)
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return fmt.Errorf("name must not contain control characters (U+%04X)", r)
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
			return fmt.Errorf("%s scope %q must be 1..%d chars from the RFC 6749 scope-token charset (printable ASCII, no quotes/backslashes)", where, scope, maxScopeLen)
		}
		if seen[scope] {
			return fmt.Errorf("%s contains duplicate scope %q", where, scope)
		}
		seen[scope] = true
	}
	return nil
}

// CanonicalJSON re-encodes v into the canonical form used by every
// weknora.plugin/1 digest. The exact algorithm — this is the protocol
// contract, pinned here because plugin developers must reproduce it to
// precompute input_schema_digest/content_digest:
//
//  1. decode the JSON document;
//  2. object keys are sorted (byte-wise, ascending); arrays keep their order;
//  3. all insignificant whitespace between tokens is dropped;
//  4. number literals are preserved VERBATIM — "1", "1.0" and "1e2" are
//     different canonical documents. This is NOT RFC 8785 (JCS), which
//     normalizes numbers to ECMAScript form: JSON.stringify(1.0)==="1" would
//     collide with our "1.0". Reproduce digests with this Go package's
//     ToolSchemaDigest/ManifestContentDigest, or an implementation that
//     sorts keys, drops whitespace and keeps number literals unchanged.
func CanonicalJSON(v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return canonicalizeJSON(raw)
}

// canonicalizeJSON canonicalizes an already-encoded JSON document. Trailing
// non-whitespace content makes the input invalid JSON and yields nil — the
// same strict semantics as json.Unmarshal (json.Decoder.Decode alone would
// happily digest only the first value).
func canonicalizeJSON(raw []byte) []byte {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var parsed any
	if err := dec.Decode(&parsed); err != nil {
		return nil
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil // trailing garbage: reject the whole document
	}
	out, err := json.Marshal(parsed)
	if err != nil {
		return nil
	}
	return out
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ToolSchemaDigest returns the canonical-JSON SHA-256 of a tool input schema,
// as 64 lowercase hex chars — the same digest convention as
// OCSchemaDigest/MCPConfigFingerprint. Key order and whitespace in the input
// do not affect the result; number literals are preserved verbatim (NOT JCS
// — see CanonicalJSON). Empty or invalid JSON (including trailing garbage)
// yields "" so it can never accidentally equal a declared digest.
func ToolSchemaDigest(schema []byte) string {
	if len(schema) == 0 {
		return ""
	}
	canonical := canonicalizeJSON(schema)
	if canonical == nil {
		return ""
	}
	return sha256Hex(canonical)
}

// ManifestContentDigest digests the manifest's semantic content with the
// self-referential content_digest field excluded, so the digest can be
// embedded in the document it certifies. Uses the CanonicalJSON algorithm
// (sorted keys, no whitespace, verbatim number literals — see CanonicalJSON).
func ManifestContentDigest(m *types.PluginManifest) string {
	if m == nil {
		return ""
	}
	copyM := *m
	copyM.ContentDigest = ""
	return sha256Hex(CanonicalJSON(&copyM))
}

// SnapshotDigest digests a verified tool snapshot list. The slice order is
// significant (it is the reviewed directory order); object key order is not.
func SnapshotDigest(tools []types.PluginToolSnapshot) string {
	return sha256Hex(CanonicalJSON(tools))
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
