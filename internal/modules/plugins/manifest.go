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
	"net/url"
	"regexp"
	"unicode/utf8"

	"github.com/Tencent/WeKnora/internal/types"
)

const (
	// PluginProtocolV1 is the only manifest protocol version accepted today.
	PluginProtocolV1 = "weknora.plugin/1"

	// maxPluginNameRunes bounds the human-readable plugin name.
	maxPluginNameRunes = 255
)

var (
	// pluginIDPattern: lowercase segments separated by dots/hyphens, 3..128
	// chars total, must start with a letter or digit.
	pluginIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{2,127}$`)

	// pluginVersionPattern: MAJOR.MINOR.PATCH, digits only.
	pluginVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

	// schemaDigestPattern: 64 lowercase hex chars (canonical-JSON SHA-256),
	// the same convention as OCSchemaDigest and MCPConfigFingerprint.
	schemaDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

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
	if !pluginIDPattern.MatchString(m.PluginID) {
		return fmt.Errorf("invalid plugin_id %q (must match ^[a-z0-9][a-z0-9.-]{2,127}$)", m.PluginID)
	}
	if !pluginVersionPattern.MatchString(m.Version) {
		return fmt.Errorf("invalid version %q (must be MAJOR.MINOR.PATCH)", m.Version)
	}
	if m.Name == "" || utf8.RuneCountInString(m.Name) > maxPluginNameRunes {
		return fmt.Errorf("name must be 1..%d characters", maxPluginNameRunes)
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
	}
	return nil
}

// CanonicalJSON re-encodes v so that equal JSON documents (modulo key order
// and number formatting) always produce the same bytes: objects become
// map[string]any (encoding/json sorts keys) and numbers are preserved
// verbatim via json.Number.
func CanonicalJSON(v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return canonicalizeJSON(raw)
}

func canonicalizeJSON(raw []byte) []byte {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var parsed any
	if err := dec.Decode(&parsed); err != nil {
		return nil
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
// OCSchemaDigest/MCPConfigFingerprint. Key order in the input does not affect
// the result. Empty or invalid JSON yields "" so it can never accidentally
// equal a declared digest.
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
// embedded in the document it certifies.
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
