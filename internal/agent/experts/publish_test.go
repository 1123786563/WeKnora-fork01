package experts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/Tencent/WeKnora/internal/types"
)

// publishTestAgent is one agent carrying EVERY export-relevant field plus the
// excluded classes (KB bindings, model keys, sandbox binding, memory) so the
// whitelist assertions can prove the excluded ones never reach the snapshot.
func publishTestAgent() *types.CustomAgent {
	memory := true
	return &types.CustomAgent{
		ID:          "0b9f6a1e-6c96-4a8e-b7a5-3f2d1c0a9b8d",
		TenantID:    7,
		Name:        "合同审查助手",
		Description: "审查合同条款并提示风险",
		Config: types.CustomAgentConfig{
			SystemPrompt:        "你是一名资深法务，逐条审查合同并指出风险。",
			PersonaMBTI:         "INTJ",
			PersonaStyle:        "保持简洁直接的沟通风格",
			SkillsSelectionMode: "selected",
			SelectedSkills:      []string{"pdf-extract", "doc-render"},
			Subagents:           []string{"code-reviewer", "web-researcher"},
			QuestionSuggestions: &types.QuestionSuggestionConfig{
				Starters: types.StarterSuggestionConfig{
					Enabled: true,
					Mode:    types.SuggestionModeCurated,
					Items:   []string{"帮我审查这份 NDA", "起草一条保密条款"},
					Count:   2,
				},
			},
			// Everything below is EXCLUDED from the export whitelist.
			KBSelectionMode:     "selected",
			KnowledgeBases:      []string{"kb-1", "kb-2"},
			ModelID:             "model-secret-1",
			RerankModelID:       "rerank-secret-1",
			SandboxConfigID:     "sbx-1",
			MemoryEnabled:       &memory,
			MCPServices:         []string{"mcp-1"},
			WebSearchProviderID: "wsp-1",
		},
	}
}

func TestMaterializePublishedAgentWhitelist(t *testing.T) {
	agent := publishTestAgent()
	me, err := MaterializePublishedAgent(agent, PublishedAgentExport{Locale: "zh-CN"})
	require.NoError(t, err)

	m := me.Manifest
	require.Equal(t, TenantExpertID(agent.ID), m.ID)
	require.Equal(t, "tenant-expert-"+agent.ID, m.ID, "manifest ID must be prefix-scoped off the agent ID")
	// zh+en passthrough: the agent carries one name/description pair, so both
	// locale keys store what was there.
	require.Equal(t, "合同审查助手", m.Label["zh"])
	require.Equal(t, "合同审查助手", m.Label["en"])
	require.Equal(t, "审查合同条款并提示风险", m.Description["zh"])
	require.Equal(t, []string{soulFileName}, m.PromptFiles)

	// Whitelisted exports.
	require.Equal(t, []string{"pdf-extract", "doc-render"}, m.Skills, "skill references ride the skills list")
	require.True(t, m.SkillRefs, "published skills are references, not bundled dirs")
	require.Equal(t, []string{"code-reviewer", "web-researcher"}, m.Subagents)
	require.Len(t, m.QuickPrompts, 2)
	require.Equal(t, "帮我审查这份 NDA", m.QuickPrompts[0].Prompt["zh"])
	require.Equal(t, "帮我审查这份 NDA", m.QuickPrompts[0].Prompt["en"])
	require.Equal(t, types.AgentModeSmartReasoning, m.AgentConfig.AgentMode, "minimal smart-reasoning, the MaterializeSkillset contract")
	require.Empty(t, m.AgentConfig.SystemPrompt, "the whole prompt lives in SOUL.md")
	require.Empty(t, m.PersonaMBTI, "persona is baked into SOUL.md, not double-rendered at runtime")

	// The persona document carries the whitelisted prompt material…
	soul := string(me.PersonaFiles[soulFileName])
	require.NotEmpty(t, soul)
	require.Contains(t, soul, "你是一名资深法务")
	require.Contains(t, soul, "INTJ", "the MBTI persona segment is rendered into the snapshot")
	require.Contains(t, soul, "保持简洁直接的沟通风格", "persona_style rides the persona segment")
	// …and none of the excluded configuration.
	for _, secret := range []string{"kb-1", "kb-2", "model-secret-1", "rerank-secret-1", "sbx-1", "mcp-1", "wsp-1", "memory"} {
		require.NotContains(t, soul, secret, "excluded config must not leak into the persona document")
	}

	// The manifest yaml round-trips the two M4 manifest extensions.
	raw, err := yaml.Marshal(m)
	require.NoError(t, err)
	var back ExpertManifest
	require.NoError(t, yaml.Unmarshal(raw, &back))
	require.Equal(t, m.SkillRefs, back.SkillRefs)
	require.Equal(t, m.Subagents, back.Subagents)
	require.Equal(t, m.Skills, back.Skills)
}

func TestMaterializePublishedAgentOverridesNameAndDescription(t *testing.T) {
	me, err := MaterializePublishedAgent(publishTestAgent(), PublishedAgentExport{
		Name: "市场名", Description: "市场描述", Locale: "en",
	})
	require.NoError(t, err)
	require.Equal(t, "市场名", me.Manifest.Label["zh"])
	require.Equal(t, "市场名", me.Manifest.Label["en"])
	require.Equal(t, "市场描述", me.Manifest.Description["zh"])
}

func TestMaterializePublishedAgentRejectsUnsafeReferences(t *testing.T) {
	agent := publishTestAgent()
	agent.Config.SelectedSkills = []string{"ok", "../evil"}
	_, err := MaterializePublishedAgent(agent, PublishedAgentExport{})
	require.Error(t, err)

	agent = publishTestAgent()
	agent.Config.Subagents = []string{"ok/../evil"}
	_, err = MaterializePublishedAgent(agent, PublishedAgentExport{})
	require.Error(t, err)

	// A blank agent id cannot form a safe expert directory.
	agent = publishTestAgent()
	agent.ID = "../escape"
	_, err = MaterializePublishedAgent(agent, PublishedAgentExport{})
	require.Error(t, err)
}

func TestMaterializePublishedAgentMinimalAgentStillShipsSoul(t *testing.T) {
	agent := &types.CustomAgent{ID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890", Name: "裸智能体"}
	me, err := MaterializePublishedAgent(agent, PublishedAgentExport{})
	require.NoError(t, err)
	require.NotEmpty(t, me.PersonaFiles[soulFileName], "the scanner rejects empty prompt files; a header-only SOUL must still exist")
	require.Empty(t, me.Manifest.Skills)
	require.Empty(t, me.Manifest.Subagents)
	require.Empty(t, me.Manifest.QuickPrompts)
}

func TestPublishedSnapshotRoundTripsThroughScanExperts(t *testing.T) {
	root := t.TempDir()
	agent := publishTestAgent()
	me, err := MaterializePublishedAgent(agent, PublishedAgentExport{Locale: "zh-CN"})
	require.NoError(t, err)

	snapshotDir := PublishedSnapshotDir(root, 7, me.Manifest.ID)
	require.NoError(t, WriteMaterializedExpert(snapshotDir, me))

	// The snapshot must parse with the same scanner the catalog uses —
	// skill references (no bundled dirs) included.
	scanned, err := ScanExperts(PublishedTenantDir(root, 7))
	require.NoError(t, err, "a published snapshot must parse through ScanExperts")
	require.Len(t, scanned, 1)
	require.Equal(t, me.Manifest.ID, scanned[0].Manifest.ID)
	require.Empty(t, scanned[0].SkillDirs, "references carry no bundled directories")
	require.Equal(t, []string{"pdf-extract", "doc-render"}, scanned[0].Manifest.Skills)
	require.Equal(t, []string{"code-reviewer", "web-researcher"}, scanned[0].Manifest.Subagents)
	require.NotEmpty(t, scanned[0].PersonaFiles[soulFileName])

	// The staged atomic write left no scratch directories inside the tenant
	// scan root (a stray manifest-bearing dir would break the next scan).
	entries, err := os.ReadDir(PublishedTenantDir(root, 7))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, me.Manifest.ID, entries[0].Name())

	// LoadExpertDir reads exactly one package directory (the install path).
	one, err := LoadExpertDir(snapshotDir)
	require.NoError(t, err)
	require.Equal(t, me.Manifest.ID, one.Manifest.ID)
	// The scanned snapshot rebuilds to the same digest — the integrity rule
	// the install path verifies against the published row.
	rebuilt := &MaterializedExpert{Manifest: one.Manifest, PersonaFiles: one.PersonaFiles}
	require.Equal(t, me.SnapshotSHA256(), rebuilt.SnapshotSHA256())
}

func TestScanExpertsSkillRefsFlagGovernsDirRequirement(t *testing.T) {
	root := t.TempDir()
	write := func(manifest string) string {
		dir := filepath.Join(root, "e")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(manifest), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("# x"), 0o644))
		return dir
	}

	// skill_refs: true — the skills entries are references; no dirs needed.
	write("id: ref-expert\nprompt_files: [SOUL.md]\nskills: [alpha, beta]\nskill_refs: true\n")
	scanned, err := ScanExperts(root)
	require.NoError(t, err)
	require.Len(t, scanned, 1)
	require.Empty(t, scanned[0].SkillDirs)

	// Without the flag the strict bundled-dir contract is unchanged
	// (builtins keep their validation).
	root2 := t.TempDir()
	dir := filepath.Join(root2, "e")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.yaml"),
		[]byte("id: bundled-expert\nprompt_files: [SOUL.md]\nskills: [alpha]\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("# x"), 0o644))
	_, err = ScanExperts(root2)
	require.Error(t, err, "a manifest listing bundled skills without the directories must keep failing")
	require.True(t, strings.Contains(err.Error(), "alpha"))
}
