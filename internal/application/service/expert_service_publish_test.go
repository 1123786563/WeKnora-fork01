package service

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/experts"
)

// TestBuildAgentFromExpertMapsManifestSubagents pins the M2 manifest
// extension the tenant expert market (M4 Task 5) rides on: subagent slugs
// travel in the manifest yaml under `subagents: []` and Instantiate maps them
// onto Config.Subagents verbatim. Without the field the agent config stays
// untouched (legacy manifests never gain delegation).
func TestBuildAgentFromExpertMapsManifestSubagents(t *testing.T) {
	e := &experts.Expert{Manifest: experts.ExpertManifest{
		ID:          "some-expert",
		Label:       experts.LocaleText{"zh": "专家", "en": "Expert"},
		PromptFiles: []string{"SOUL.md"},
		Skills:      []string{"pdf-extract"},
		Subagents:   []string{"code-reviewer", "web-researcher"},
	}}

	agent := buildAgentFromExpert(e, "zh", "")
	require.Equal(t, []string{"code-reviewer", "web-researcher"}, agent.Config.Subagents)
	// The manifest slice is copied, never aliased (experts are shared
	// read-only; a later mutation of the agent must not write back).
	agent.Config.Subagents[0] = "mutated"
	require.Equal(t, "code-reviewer", e.Manifest.Subagents[0])

	// A manifest without subagents leaves the field nil.
	plain := &experts.Expert{Manifest: experts.ExpertManifest{ID: "plain", Label: experts.LocaleText{"zh": "普通"}}}
	agent = buildAgentFromExpert(plain, "zh", "")
	require.Nil(t, agent.Config.Subagents)
}

// TestExpertProvenanceRecognizesTenantPublishedExperts pins the provenance
// stamp of a tenant-published expert: Source "tenant" plus the originating
// agent ID as the slug. Builtin and skillhub experts keep their stamps.
func TestExpertProvenanceRecognizesTenantPublishedExperts(t *testing.T) {
	p := expertProvenance("tenant-expert-0b9f6a1e-6c96-4a8e-b7a5-3f2d1c0a9b8d")
	require.Equal(t, "tenant", p.Source)
	require.Equal(t, "0b9f6a1e-6c96-4a8e-b7a5-3f2d1c0a9b8d", p.Slug)
	require.Equal(t, "tenant-expert-0b9f6a1e-6c96-4a8e-b7a5-3f2d1c0a9b8d", p.ExpertID)

	require.Equal(t, "builtin", expertProvenance("stock-assistant").Source)
	require.Equal(t, "skillhub", expertProvenance("skillhub-skillset-foo").Source)
	// A bare prefix with no agent id is not a tenant expert.
	require.Equal(t, "builtin", expertProvenance("tenant-expert-").Source)
}
