package utils

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
)

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
//     collide with our "1.0". Reproduce digests with this package's
//     ToolSchemaDigest (or the plugins package's
//     ToolSchemaDigest/ManifestContentDigest wrappers), or an implementation
//     that sorts keys, drops whitespace and keeps number literals unchanged.
//  5. strings are emitted WITHOUT Go's default HTML escaping: '<', '>' and
//     '&' appear literally, never as \u003c/\u003e/\u0026 (JSON.stringify and
//     most non-Go serializers behave the same way). EXCEPTION: U+2028 and
//     U+2029 (LINE/PARAGRAPH SEPARATOR) are ALWAYS emitted as \u2028/\u2029 —
//     Go's encoder escapes them unconditionally (JSONSP compatibility) and
//     SetEscapeHTML(false) does not change that, while JSON.stringify
//     (ES2019+) keeps them literal. Non-Go reimplementations MUST escape
//     these two code points or digests over documents containing them will
//     disagree (locked by the plugins package's
//     TestCanonicalJSONAlwaysEscapesLineSeparators).
//
// The canonical-JSON digest contract lives in this platform package (not the
// plugins module) because TWO modules must compute it with the exact same
// algorithm: plugins at install/rebase time (input_schema_digest persisted
// with the accepted snapshot) and agentruntime at run time (the drift guard
// re-digests the live tool schema and fail-closes on disagreement). A
// module-to-module import is forbidden by architectureguard, so the shared
// seam sinks to the platform layer both may import
// (issue #106 gate regression fix; same pattern as 7d30d6fe6's
// composition-root move).
func CanonicalJSON(v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return CanonicalizeJSON(raw)
}

// CanonicalizeJSON canonicalizes an already-encoded JSON document. Trailing
// non-whitespace content makes the input invalid JSON and yields nil — the
// same strict semantics as json.Unmarshal (json.Decoder.Decode alone would
// happily digest only the first value).
func CanonicalizeJSON(raw []byte) []byte {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var parsed any
	if err := dec.Decode(&parsed); err != nil {
		return nil
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil // trailing garbage: reject the whole document
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(parsed); err != nil {
		return nil
	}
	// json.Encoder.Encode appends a trailing newline; it is not part of the
	// canonical form.
	return bytes.TrimRight(buf.Bytes(), "\n")
}

// SHA256Hex returns the SHA-256 of data as 64 lowercase hex chars — the
// digest convention shared by every weknora.plugin/1 digest (and
// OCSchemaDigest/MCPConfigFingerprint).
func SHA256Hex(data []byte) string {
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
	canonical := CanonicalizeJSON(schema)
	if canonical == nil {
		return ""
	}
	return SHA256Hex(canonical)
}
