package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/tools"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestDurableCraftKnowledgeSelectionSnapshotRoundTrip(t *testing.T) {
	query := "Use sales Q1 and explain the regional trend."
	modelQuery := query + "\n\n[已授权输入材料]\n- uploads/sales.csv"
	config := &types.AgentConfig{AllowedTools: []string{tools.ToolThinking}}
	inputs := []craft.Input{{Ref: "resource://upload-1", Name: "sales.csv", SHA256: strings.Repeat("a", 64), Bytes: 12}}
	selection := CraftKnowledgeSelectionSnapshot{Query: query, KnowledgeBaseIDs: []string{"kb-selected"}}

	raw, err := BuildDurableCraftRunSnapshotWithKnowledgeSelection(modelQuery, nil, "model-1", "", config, inputs, selection)
	require.NoError(t, err)
	require.JSONEq(t, `{"query":"Use sales Q1 and explain the regional trend.","knowledge_base_ids":["kb-selected"]}`, selectionJSONFromSnapshot(t, raw))
	parsed, err := ParseDurableRunSnapshot(raw)
	require.NoError(t, err)
	require.Equal(t, modelQuery, parsed.Query, "the model prompt retains server-composed input guidance")
	require.NotNil(t, parsed.CraftKnowledgeSelection)
	require.Equal(t, query, parsed.CraftKnowledgeSelection.Query)
	require.Equal(t, []string{"kb-selected"}, parsed.CraftKnowledgeSelection.KnowledgeBaseIDs)
	require.NotNil(t, parsed.CraftInputManifest)
	require.Equal(t, inputs, *parsed.CraftInputManifest)

	message, err := durableUserMessage(parsed)
	require.NoError(t, err)
	require.Equal(t, modelQuery, message.Content, "the model receives the exact admitted prompt, without snapshot selection expansion")
	require.NotContains(t, message.Content, "kb-selected")

	reordered, err := BuildDurableCraftRunSnapshotWithKnowledgeSelection(modelQuery, nil, "model-1", "", config, inputs,
		CraftKnowledgeSelectionSnapshot{Query: query, KnowledgeBaseIDs: []string{"kb-selected"}})
	require.NoError(t, err)
	reorderedAgain, err := BuildDurableCraftRunSnapshotWithKnowledgeSelection(modelQuery, nil, "model-1", "", config, inputs,
		CraftKnowledgeSelectionSnapshot{Query: query, KnowledgeBaseIDs: []string{"kb-selected"}})
	require.NoError(t, err)
	require.Equal(t, reordered, reorderedAgain, "the one-KB selection produces stable snapshot bytes for the durable request digest")
}

func TestDurableCraftKnowledgeSelectionCompatibilityAndGenericSnapshot(t *testing.T) {
	config := &types.AgentConfig{AllowedTools: []string{tools.ToolThinking}}
	craftRaw, err := BuildDurableCraftRunSnapshot("legacy craft prompt", nil, "model-1", "", config, []craft.Input{})
	require.NoError(t, err)
	legacyCraftRaw := rewriteCraftKnowledgeSelection(t, craftRaw, nil, false)
	legacyCraft, err := ParseDurableRunSnapshot(legacyCraftRaw)
	require.NoError(t, err)
	require.NotNil(t, legacyCraft.CraftKnowledgeSelection, "legacy Craft snapshot without the field is explicit empty selection")
	require.Equal(t, "legacy craft prompt", legacyCraft.CraftKnowledgeSelection.Query)
	require.NotNil(t, legacyCraft.CraftKnowledgeSelection.KnowledgeBaseIDs)
	require.Empty(t, legacyCraft.CraftKnowledgeSelection.KnowledgeBaseIDs)

	newCraft, err := ParseDurableRunSnapshot(craftRaw)
	require.NoError(t, err)
	require.NotNil(t, newCraft.CraftKnowledgeSelection, "new Craft snapshots encode empty selection explicitly")
	require.Empty(t, newCraft.CraftKnowledgeSelection.KnowledgeBaseIDs, "empty means no selected KB, never all KBs")

	genericRaw, err := BuildDurableRunSnapshot("generic prompt", nil, "model-1", "", config)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(genericRaw, &fields))
	require.NotContains(t, fields, "craft_knowledge_selection", "generic snapshot JSON contract stays unchanged")
	generic, err := ParseDurableRunSnapshot(genericRaw)
	require.NoError(t, err)
	require.Nil(t, generic.CraftKnowledgeSelection)
}

func TestDurableCraftKnowledgeSelectionRejectsMalformedSelection(t *testing.T) {
	query := "sales"
	config := &types.AgentConfig{AllowedTools: []string{tools.ToolThinking}}
	valid, err := BuildDurableCraftRunSnapshotWithKnowledgeSelection(query, nil, "model-1", "", config, nil,
		CraftKnowledgeSelectionSnapshot{Query: query, KnowledgeBaseIDs: []string{"kb-a"}})
	require.NoError(t, err)
	tooMany := make([]string, craft.MaxKnowledgeSources+1)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("kb-%02d", i)
	}
	for _, tc := range []struct {
		name  string
		value any
	}{
		{name: "null selection", value: nil},
		{name: "malformed object", value: []string{"kb-a"}},
		{name: "null id list", value: map[string]any{"query": query, "knowledge_base_ids": nil}},
		{name: "duplicate ids", value: map[string]any{"query": query, "knowledge_base_ids": []string{"kb-a", "kb-a"}}},
		{name: "two KBs exceed current selection contract", value: map[string]any{"query": query, "knowledge_base_ids": []string{"kb-a", "kb-b"}}},
		{name: "oversized selection", value: map[string]any{"query": query, "knowledge_base_ids": tooMany}},
		{name: "empty retrieval query", value: map[string]any{"query": "", "knowledge_base_ids": []string{"kb-a"}}},
		{name: "oversized retrieval query", value: map[string]any{"query": strings.Repeat("q", MaxCraftPromptBytes+1), "knowledge_base_ids": []string{"kb-a"}}},
		{name: "empty id", value: map[string]any{"query": query, "knowledge_base_ids": []string{" "}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseDurableRunSnapshot(rewriteCraftKnowledgeSelection(t, valid, tc.value, true))
			require.Error(t, err)
		})
	}

	_, err = BuildDurableCraftRunSnapshotWithKnowledgeSelection(query, nil, "model-1", "", config, nil,
		CraftKnowledgeSelectionSnapshot{Query: query, KnowledgeBaseIDs: []string{"kb-a", "kb-a"}})
	require.Error(t, err, "the builder rejects duplicate IDs before persistence")
	_, err = BuildDurableCraftRunSnapshotWithKnowledgeSelection(query, nil, "model-1", "", config, nil,
		CraftKnowledgeSelectionSnapshot{Query: query, KnowledgeBaseIDs: []string{"kb-a", "kb-b"}})
	require.ErrorIs(t, err, craft.ErrInvalidInput, "the builder rejects two selected KBs under the current zero-or-one contract")
	_, err = BuildDurableCraftRunSnapshotWithKnowledgeSelection(query, nil, "model-1", "", config, nil,
		CraftKnowledgeSelectionSnapshot{Query: query, KnowledgeBaseIDs: tooMany})
	require.Error(t, err, "the builder rejects oversized selections before persistence")
}

func TestDurableCraftWorkspaceSeedValidationAndExecutionBoundary(t *testing.T) {
	config := &types.AgentConfig{AllowedTools: []string{tools.ToolThinking}}
	craftRaw, err := BuildDurableCraftRunSnapshot("draft seed prompt", nil, "model-1", "", config, nil)
	require.NoError(t, err)
	validSeed := CraftWorkspaceSeedSnapshot{
		WorkspaceID: "workspace-1", State: craft.DraftHeadEmpty,
		DraftRevision: 0,
	}
	withSeed := rewriteCraftWorkspaceSeed(t, craftRaw, validSeed, true)
	validSeed.WorkspaceID = "mutated-after-serialization"
	parsed, err := ParseDurableRunSnapshot(withSeed)
	require.NoError(t, err)
	require.Equal(t, "workspace-1", parsed.CraftWorkspaceSeed.WorkspaceID,
		"mutating the caller's seed after serialization cannot change the durable bytes")
	require.NoError(t, requireCraftWorkspaceSeed(parsed, true))

	selected := CraftWorkspaceSeedSnapshot{
		WorkspaceID: "workspace-1", State: craft.DraftHeadSelected,
		DraftRevision: 4, SourceRunID: "run-4", ManifestDigest: strings.Repeat("a", 64),
	}
	selectedRaw := rewriteCraftWorkspaceSeed(t, craftRaw, selected, true)
	selectedParsed, err := ParseDurableRunSnapshot(selectedRaw)
	require.NoError(t, err)
	require.Equal(t, selected, *selectedParsed.CraftWorkspaceSeed)
	require.NoError(t, requireCraftWorkspaceSeed(selectedParsed, true))

	// Generic snapshots remain parseable without a seed, while a Craft snapshot
	// is stopped at execution admission if its server-owned seed is absent.
	genericRaw, err := BuildDurableRunSnapshot("generic prompt", nil, "model-1", "", config)
	require.NoError(t, err)
	generic, err := ParseDurableRunSnapshot(genericRaw)
	require.NoError(t, err)
	require.NoError(t, requireCraftWorkspaceSeed(generic, false))
	legacyCraft, err := ParseDurableRunSnapshot(craftRaw)
	require.NoError(t, err)
	require.Error(t, requireCraftWorkspaceSeed(legacyCraft, true))

	for _, tc := range []struct {
		name string
		seed CraftWorkspaceSeedSnapshot
	}{
		{name: "empty with revision", seed: CraftWorkspaceSeedSnapshot{WorkspaceID: "workspace-1", State: craft.DraftHeadEmpty, DraftRevision: 1}},
		{name: "empty with source", seed: CraftWorkspaceSeedSnapshot{WorkspaceID: "workspace-1", State: craft.DraftHeadEmpty, SourceRunID: "run-1"}},
		{name: "empty with digest", seed: CraftWorkspaceSeedSnapshot{WorkspaceID: "workspace-1", State: craft.DraftHeadEmpty, ManifestDigest: strings.Repeat("a", 64)}},
		{name: "selected without revision", seed: CraftWorkspaceSeedSnapshot{WorkspaceID: "workspace-1", State: craft.DraftHeadSelected, SourceRunID: "run-1", ManifestDigest: strings.Repeat("a", 64)}},
		{name: "selected without source", seed: CraftWorkspaceSeedSnapshot{WorkspaceID: "workspace-1", State: craft.DraftHeadSelected, DraftRevision: 1, ManifestDigest: strings.Repeat("a", 64)}},
		{name: "selected malformed digest", seed: CraftWorkspaceSeedSnapshot{WorkspaceID: "workspace-1", State: craft.DraftHeadSelected, DraftRevision: 1, SourceRunID: "run-1", ManifestDigest: "bad"}},
		{name: "unknown state", seed: CraftWorkspaceSeedSnapshot{WorkspaceID: "workspace-1", State: "unresolved"}},
		{name: "empty workspace", seed: CraftWorkspaceSeedSnapshot{State: craft.DraftHeadEmpty}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseDurableRunSnapshot(rewriteCraftWorkspaceSeed(t, craftRaw, tc.seed, true))
			require.Error(t, err)
		})
	}
	require.Error(t, parseCraftWorkspaceSeedValue(t, craftRaw, nil), "null seed is invalid when the field is present")
	_, err = ParseDurableRunSnapshot(rewriteCraftWorkspaceSeed(t, craftRaw, nil, false))
	require.NoError(t, err, "missing seed remains syntactically parseable for legacy and generic storage")
}

func TestCraftRunSnapshotKeepsSeedOutOfModelQuery(t *testing.T) {
	seed := CraftWorkspaceSeedSnapshot{WorkspaceID: "workspace-secret", State: craft.DraftHeadSelected,
		DraftRevision: 1, SourceRunID: "run-secret", ManifestDigest: strings.Repeat("b", 64)}
	req := CraftRunRequest{RequestID: "seed-query", Prompt: "Build the page", BaseVersionID: "historical-secret"}
	raw, err := craftRunSnapshot(req, nil, "model-1")
	require.NoError(t, err)
	snapshot, err := ParseDurableRunSnapshot(rewriteCraftWorkspaceSeed(t, raw, seed, true))
	require.NoError(t, err)
	require.Equal(t, req.Prompt, snapshot.Query)
	require.NotContains(t, snapshot.Query, req.BaseVersionID)
	require.NotContains(t, snapshot.Query, seed.WorkspaceID)
	require.NotContains(t, snapshot.Query, seed.SourceRunID)
	require.NotContains(t, snapshot.Query, seed.ManifestDigest)
}

func selectionJSONFromSnapshot(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))
	selection, ok := fields["craft_knowledge_selection"]
	require.True(t, ok, "Craft selection is persisted in the immutable snapshot")
	return string(selection)
}

func rewriteCraftKnowledgeSelection(t *testing.T, raw json.RawMessage, value any, present bool) json.RawMessage {
	t.Helper()
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))
	if !present {
		delete(fields, "craft_knowledge_selection")
	} else {
		encoded, err := json.Marshal(value)
		require.NoError(t, err)
		fields["craft_knowledge_selection"] = encoded
	}
	rewritten, err := json.Marshal(fields)
	require.NoError(t, err)
	return rewritten
}

func rewriteCraftWorkspaceSeed(t *testing.T, raw json.RawMessage, value any, present bool) json.RawMessage {
	t.Helper()
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))
	if !present {
		delete(fields, "craft_workspace_seed")
	} else {
		encoded, err := json.Marshal(value)
		require.NoError(t, err)
		fields["craft_workspace_seed"] = encoded
	}
	rewritten, err := json.Marshal(fields)
	require.NoError(t, err)
	return rewritten
}

// T39 #69 D8: the workbench admission lane persists an admission-map snapshot
// (session/agent/text identity plus the repository's server-owned usage
// binding merge). The strict durable reader rejected it with
// `unknown field "text"`, so every leased workbench run produced zero events
// and died at its deadline. The admission identity and usage binding are
// server-owned durable snapshot data and must sit inside the strict schema.
func TestParseDurableRunSnapshotAcceptsWorkbenchAdmissionSnapshot(t *testing.T) {
	raw := json.RawMessage(`{
		"session_id": "s1", "agent_id": "builtin-quick-answer", "target_id": "platform",
		"workspace_ref": "", "space_id": "space-7", "request_id": "req-d8", "text": "整理本周周报",
		"budget_upper": 1,
		"usage_source": "platform_gateway", "usage_funding": "platform", "usage_service": "connector",
		"price_version": "remote-v1", "usage_upper": 1, "usage_revision": 1,
		"usage_status": "final", "usage_dimensions": {"connector": 1}
	}`)
	parsed, err := ParseDurableRunSnapshot(raw)
	require.NoError(t, err, "the persisted workbench admission snapshot must satisfy the strict durable reader")
	require.NotNil(t, parsed.WorkbenchAdmissionSnapshot, "admission identity decodes as an explicit typed group")
	require.Equal(t, "整理本周周报", parsed.WorkbenchAdmissionSnapshot.Text)
	require.Equal(t, "req-d8", parsed.WorkbenchAdmissionSnapshot.RequestID)
	require.Equal(t, "builtin-quick-answer", parsed.WorkbenchAdmissionSnapshot.AgentID)
	require.Equal(t, "space-7", parsed.WorkbenchAdmissionSnapshot.SpaceID)
	require.NotNil(t, parsed.RunUsageBindingSnapshot, "server-owned usage binding decodes as an explicit typed group")
	require.Equal(t, int64(1), parsed.RunUsageBindingSnapshot.UsageUpper)
	require.Equal(t, "platform_gateway", parsed.RunUsageBindingSnapshot.UsageSource)
	require.Equal(t, map[string]int64{"connector": 1}, parsed.RunUsageBindingSnapshot.UsageDimensions)
	require.Empty(t, parsed.ModelID, "workbench admissions carry no graph execution core; the executor fails them explicitly")
}

// WB-GRAPH: a workbench admission that froze the server-resolved graph core
// merges version/query/model_id/agent_config/runtime into the same snapshot
// object as the admission identity and usage binding. The strict reader must
// decode the combined shape — this is the exact byte shape the workbench
// admission coordinator persists once a graph freezer is installed.
func TestParseDurableRunSnapshotAcceptsWorkbenchGraphCoreSnapshot(t *testing.T) {
	raw := json.RawMessage(`{
		"version": 1, "query": "整理本周周报", "model_id": "model-9",
		"agent_config": {"max_iterations": 5, "allowed_tools": ["knowledge_search"]},
		"runtime": {"sandbox_config_id": "ws-1"},
		"session_id": "s1", "agent_id": "builtin-quick-answer", "target_id": "platform",
		"space_id": "space-7", "request_id": "req-wb-graph", "text": "整理本周周报", "budget_upper": 1,
		"usage_source": "platform_gateway", "usage_funding": "platform", "usage_service": "connector",
		"price_version": "remote-v1", "usage_upper": 1, "usage_revision": 1,
		"usage_status": "final", "usage_dimensions": {"connector": 1}
	}`)
	parsed, err := ParseDurableRunSnapshot(raw)
	require.NoError(t, err, "the combined workbench admission + graph core snapshot must satisfy the strict durable reader")
	require.Equal(t, "model-9", parsed.ModelID)
	require.Equal(t, 1, parsed.Version)
	require.Equal(t, "整理本周周报", parsed.Query)
	require.Equal(t, "ws-1", parsed.Runtime.SandboxConfigID)
	config, err := parsed.RestoreAgentConfig()
	require.NoError(t, err, "the frozen agent config round-trips the strict config decoder")
	require.Equal(t, 5, config.MaxIterations)
	require.NotNil(t, parsed.WorkbenchAdmissionSnapshot, "admission identity still classifies the snapshot")
	require.Equal(t, "req-wb-graph", parsed.WorkbenchAdmissionSnapshot.RequestID)
	require.NotNil(t, parsed.RunUsageBindingSnapshot)
	require.Equal(t, "platform_gateway", parsed.RunUsageBindingSnapshot.UsageSource)

	var combined map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &combined))
	combined["rogue_field"] = json.RawMessage(`1`)
	rogueRaw, err := json.Marshal(combined)
	require.NoError(t, err)
	_, err = ParseDurableRunSnapshot(rogueRaw)
	require.Error(t, err, "strictness still applies to the combined shape")
	require.Contains(t, err.Error(), "rogue_field")
}

// Strictness is the security feature: admitting the workbench field group
// into the typed schema must not open the door to unknown fields.
func TestParseDurableRunSnapshotRejectsUnknownFieldOnWorkbenchAdmission(t *testing.T) {
	raw := json.RawMessage(`{"session_id": "s1", "request_id": "req-d8", "text": "t", "rogue_field": 1}`)
	_, err := ParseDurableRunSnapshot(raw)
	require.Error(t, err)
	require.Contains(t, err.Error(), "rogue_field")
}

func parseCraftWorkspaceSeedValue(t *testing.T, raw json.RawMessage, value any) error {
	t.Helper()
	returnErr := rewriteCraftWorkspaceSeed(t, raw, value, true)
	_, err := ParseDurableRunSnapshot(returnErr)
	return err
}
