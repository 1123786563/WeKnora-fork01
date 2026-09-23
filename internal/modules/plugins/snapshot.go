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
//   - a live tool the manifest never declared.
//
// The returned error names EVERY differing tool so the admin review surface
// can show the exact divergence. Snapshot fields come from the manifest
// declaration (read_only / requires_personal_auth / scopes) except
// description and input_schema_digest, which are recomputed from the live
// endpoint — the live data is the authority.
func BuildVerifiedSnapshot(manifest *types.PluginManifest, live []*types.MCPTool) ([]types.PluginToolSnapshot, string, error) {
	if manifest == nil {
		return nil, "", fmt.Errorf("manifest is required")
	}
	liveByName := make(map[string]*types.MCPTool, len(live))
	for _, tool := range live {
		if tool != nil {
			liveByName[tool.Name] = tool
		}
	}
	declared := make(map[string]bool, len(manifest.Tools))
	for _, decl := range manifest.Tools {
		declared[decl.Name] = true
	}

	var problems []string
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
		snapshot = append(snapshot, types.PluginToolSnapshot{
			Name:                 decl.Name,
			Description:          actual.Description,
			InputSchemaDigest:    liveDigest,
			ReadOnly:             decl.ReadOnly,
			RequiresPersonalAuth: decl.RequiresPersonalAuth,
			Scopes:               decl.Scopes,
		})
	}
	for _, tool := range live {
		if tool == nil || declared[tool.Name] {
			continue
		}
		problems = append(problems, fmt.Sprintf("undeclared tool %q present on endpoint", tool.Name))
	}
	if len(problems) > 0 {
		return nil, "", fmt.Errorf("manifest and live endpoint disagree: %s", strings.Join(problems, "; "))
	}
	return snapshot, SnapshotDigest(snapshot), nil
}
