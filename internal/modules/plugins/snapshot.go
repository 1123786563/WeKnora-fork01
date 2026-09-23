package plugins

import (
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
)

// BuildVerifiedSnapshot cross-checks the manifest's declared tool directory
// against the LIVE ListTools result and derives the authoritative snapshot.
//
// Verification is bidirectional and exhaustive — every disagreement is a
// rejection, never a silent acceptance:
//   - a declared tool missing from the live endpoint;
//   - a declared tool whose live schema digest differs from the declaration;
//   - a live tool the manifest never declared;
//   - a live tool name failing identifier hygiene (oversized or carrying
//     control/format characters) — live names are untrusted remote data and
//     are vetted with the same validateName rules as manifest names BEFORE
//     any of them is quoted into the rejection message;
//   - a live directory larger than maxLiveTools; a manifest declaring the
//     same tool name twice, declaring no tools at all, or declaring a
//     malformed (non-64-hex) input_schema_digest — all re-checked here
//     (ValidateManifest already rejects each of them, but this function is
//     exported and must not silently rely on the caller having validated
//     first; an unchecked empty digest would ""==""-match ToolSchemaDigest's
//     "" for an unparseable live schema and mint a snapshot whose digest is
//     not 64-hex); a declaration whose name fails the identifier hygiene
//     rules or whose scopes fail the RFC 6749 checks — re-checked for the
//     same reason: unchecked scopes would flow verbatim into the persisted
//     snapshot and the admin/member authorization surfaces.
//
// A name the endpoint serves several times (declared or not) is ONE
// discrepancy and is reported once — duplicate and undeclared echoes are
// both deduplicated.
//
// The returned error names EVERY differing tool — up to
// maxVerificationProblems entries, with the remainder collapsed into a
// counter, so a hostile endpoint cannot balloon the single joined error.
// Snapshot fields come from the manifest declaration (read_only /
// requires_personal_auth / scopes) except description and
// input_schema_digest, which are recomputed from the live endpoint — the
// live data is the authority.
func BuildVerifiedSnapshot(manifest *types.PluginManifest, live []*types.MCPTool) ([]types.PluginToolSnapshot, string, error) {
	if manifest == nil {
		return nil, "", fmt.Errorf("manifest is required")
	}
	// Re-check here instead of trusting the caller to have run
	// ValidateManifest: an empty declaration list would otherwise produce an
	// empty snapshot plus the well-formed digest of "[]" — silently breaking
	// the "a verified snapshot always carries at least the declared tools"
	// contract downstream installations rely on (OCR T01-R1-F4).
	if len(manifest.Tools) == 0 {
		return nil, "", fmt.Errorf("manifest must declare at least one tool")
	}
	// The live directory has no transport-level size bound (only the manifest
	// download is capped at 1MiB) — refuse to process an oversized directory
	// before any O(n) map/digest/snapshot work.
	if len(live) > maxLiveTools {
		return nil, "", fmt.Errorf("live endpoint returned %d tools, exceeding the maximum of %d", len(live), maxLiveTools)
	}
	var problems []string
	liveByName := make(map[string]*types.MCPTool, len(live))
	unvettedName := make(map[int]bool, len(live))
	// One duplicated live name is ONE contradiction — report it once, not
	// (N-1) times (OCR T01-R3-F3).
	reportedDuplicate := make(map[string]bool)
	for i, tool := range live {
		if tool == nil {
			continue
		}
		// Vet the name up front, keyed by position (never by the untrusted
		// name itself): everything that reaches an error message below has
		// passed the same hygiene the manifest validator enforces.
		if err := validateName(fmt.Sprintf("live tools[%d].name", i), tool.Name, maxToolNameLen); err != nil {
			problems = append(problems, err.Error())
			unvettedName[i] = true
			continue
		}
		if _, dup := liveByName[tool.Name]; dup {
			if reportedDuplicate[tool.Name] {
				continue
			}
			reportedDuplicate[tool.Name] = true
			// A directory naming one tool twice is self-contradictory; letting
			// the last entry win would silently mask the other's schema.
			problems = append(problems, fmt.Sprintf("live endpoint returned duplicate tool name %q", tool.Name))
			continue
		}
		liveByName[tool.Name] = tool
	}
	declared := make(map[string]bool, len(manifest.Tools))
	for i, decl := range manifest.Tools {
		// Re-check the declaration's name hygiene and scopes as well: this
		// function must not silently rely on the caller having validated
		// first — unchecked scopes would flow verbatim into the persisted
		// snapshot and the admin/member authorization surfaces, and an
		// unhygienic name would otherwise get a misleading "missing from
		// live endpoint" (it can never match a vetted live name). The
		// `where` uses the position index, never the untrusted name itself
		// (OCR T01-R3-F4).
		if err := validateName(fmt.Sprintf("manifest tools[%d].name", i), decl.Name, maxToolNameLen); err != nil {
			return nil, "", err
		}
		if err := validateScopes(decl.Scopes, fmt.Sprintf("manifest tools[%d]", i)); err != nil {
			return nil, "", err
		}
		// Re-check here instead of trusting the caller to have run
		// ValidateManifest: a duplicate declaration would otherwise append
		// the same tool to the snapshot twice, silently. The name echoed
		// here is UNVETTED at this point (this function must not rely on
		// the caller having validated first) — truncate it via echoQuoted
		// so a hostile manifest cannot balloon the error (整分支 OCR 一轮 F3).
		if declared[decl.Name] {
			return nil, "", fmt.Errorf("manifest declares duplicate tool name %s", echoQuoted(decl.Name))
		}
		// Re-check the digest format too: ToolSchemaDigest returns "" for an
		// unparseable live schema, so an unchecked empty declared digest would
		// ""=="" match it and mint an authoritative snapshot whose digest is
		// not 64-hex — the invariant later install verification and drift
		// detection compute against (OCR T01-R1-F4). The malformed digest
		// value is not echoed: it is unbounded untrusted input.
		if !schemaDigestPattern.MatchString(decl.InputSchemaDigest) {
			return nil, "", fmt.Errorf("manifest tool %s input_schema_digest must be 64 lowercase hex chars (canonical-JSON SHA-256)", echoQuoted(decl.Name))
		}
		declared[decl.Name] = true
	}

	snapshot := make([]types.PluginToolSnapshot, 0, len(manifest.Tools))
	for _, decl := range manifest.Tools {
		// All three rejection messages below echo MANIFEST-side fields, which
		// are unvetted at this point (this function must not rely on the
		// caller having validated first) — truncate them via echoQuoted so a
		// hostile manifest cannot balloon the joined error (整分支 OCR 二轮
		// F1; a legitimate 64-hex digest is exactly maxEchoRunes and passes
		// untruncated; liveDigest is computed locally and always bounded).
		actual, ok := liveByName[decl.Name]
		if !ok {
			problems = append(problems, fmt.Sprintf("manifest tool %s missing from live endpoint", echoQuoted(decl.Name)))
			continue
		}
		liveDigest := ToolSchemaDigest(actual.InputSchema)
		if liveDigest != decl.InputSchemaDigest {
			problems = append(problems, fmt.Sprintf(
				"manifest tool %s schema digest mismatch (declared %s, live %s)",
				echoQuoted(decl.Name), echoQuoted(decl.InputSchemaDigest), liveDigest,
			))
			continue
		}
		if err := validateDescription(fmt.Sprintf("live description of tool %s", echoQuoted(decl.Name)), actual.Description); err != nil {
			problems = append(problems, err.Error())
			continue
		}
		// Defensive copy: the snapshot is the tenant-facing authority and must
		// not share backing arrays with the untrusted manifest document.
		scopes := append([]string(nil), decl.Scopes...)
		snapshot = append(snapshot, types.PluginToolSnapshot{
			Name:                 decl.Name,
			Description:          actual.Description,
			InputSchemaDigest:    liveDigest,
			ReadOnly:             decl.ReadOnly,
			RequiresPersonalAuth: decl.RequiresPersonalAuth,
			Scopes:               scopes,
		})
	}
	// One undeclared name is ONE discrepancy — report it once even when the
	// endpoint served it N times (OCR T01-R3-F3).
	reportedUndeclared := make(map[string]bool)
	for i, tool := range live {
		if tool == nil || unvettedName[i] || declared[tool.Name] {
			continue
		}
		if reportedUndeclared[tool.Name] {
			continue
		}
		reportedUndeclared[tool.Name] = true
		problems = append(problems, fmt.Sprintf("undeclared tool %q present on endpoint", tool.Name))
	}
	if len(problems) > maxVerificationProblems {
		problems = append(problems[:maxVerificationProblems],
			fmt.Sprintf("...and %d more discrepancies", len(problems)-maxVerificationProblems))
	}
	if len(problems) > 0 {
		return nil, "", fmt.Errorf("manifest and live endpoint disagree: %s", strings.Join(problems, "; "))
	}
	return snapshot, SnapshotDigest(snapshot), nil
}
