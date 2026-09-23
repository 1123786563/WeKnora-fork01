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
//   - a live directory larger than maxLiveTools, or a manifest declaring the
//     same tool name twice — both are re-checked here (ValidateManifest
//     already rejects duplicate declarations, but this function is exported
//     and must not silently rely on the caller having validated first).
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
	// The live directory has no transport-level size bound (only the manifest
	// download is capped at 1MiB) — refuse to process an oversized directory
	// before any O(n) map/digest/snapshot work.
	if len(live) > maxLiveTools {
		return nil, "", fmt.Errorf("live endpoint returned %d tools, exceeding the maximum of %d", len(live), maxLiveTools)
	}
	var problems []string
	liveByName := make(map[string]*types.MCPTool, len(live))
	unvettedName := make(map[int]bool, len(live))
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
			// A directory naming one tool twice is self-contradictory; letting
			// the last entry win would silently mask the other's schema.
			problems = append(problems, fmt.Sprintf("live endpoint returned duplicate tool name %q", tool.Name))
			continue
		}
		liveByName[tool.Name] = tool
	}
	declared := make(map[string]bool, len(manifest.Tools))
	for _, decl := range manifest.Tools {
		// Re-check here instead of trusting the caller to have run
		// ValidateManifest: a duplicate declaration would otherwise append
		// the same tool to the snapshot twice, silently. The name echoed
		// here is UNVETTED at this point (this function must not rely on
		// the caller having validated first) — truncate it via echoQuoted
		// so a hostile manifest cannot balloon the error (整分支 OCR 一轮 F3).
		if declared[decl.Name] {
			return nil, "", fmt.Errorf("manifest declares duplicate tool name %s", echoQuoted(decl.Name))
		}
		declared[decl.Name] = true
	}

	snapshot := make([]types.PluginToolSnapshot, 0, len(manifest.Tools))
	for _, decl := range manifest.Tools {
		actual, ok := liveByName[decl.Name]
		if !ok {
			problems = append(problems, fmt.Sprintf("manifest tool %q missing from live endpoint", decl.Name))
			continue
		}
		liveDigest := ToolSchemaDigest(actual.InputSchema)
		if liveDigest != decl.InputSchemaDigest {
			problems = append(problems, fmt.Sprintf(
				"manifest tool %q schema digest mismatch (declared %s, live %s)",
				decl.Name, decl.InputSchemaDigest, liveDigest,
			))
			continue
		}
		if err := validateDescription(fmt.Sprintf("live description of tool %q", decl.Name), actual.Description); err != nil {
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
	for i, tool := range live {
		if tool == nil || unvettedName[i] || declared[tool.Name] {
			continue
		}
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
