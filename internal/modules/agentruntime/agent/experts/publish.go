// Tenant-expert publishing (M4 Task 5): exporting an existing tenant agent
// into the immutable expert snapshot the tenant-internal market installs
// from. The export follows the M2 §5 whitelist — persona/system prompt,
// skill references, the subagents list and the starter prompts — and
// deliberately nothing else: KB bindings, model-key-related config, the
// sandbox binding and memory belong to the publisher's workspace, never to
// the installed copy.
//
// The snapshot is a MaterializedExpert in the SAME ScanExperts layout the
// builtin library and the skillhub materializer use, so the catalog, the
// detail view and Instantiate treat published experts identically from the
// moment they are installed.
package experts

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/persona"
	"github.com/Tencent/WeKnora/internal/types"
)

// tenantExpertIDPrefix scopes published-expert manifest IDs off the source
// agent's ID so they can collide with neither builtin manifest IDs nor the
// skillhub-skillset- prefix.
const tenantExpertIDPrefix = "tenant-expert-"

// TenantExpertID derives the materialized expert's manifest ID (and its
// directory name) from the source agent's ID.
func TenantExpertID(agentID string) string {
	return tenantExpertIDPrefix + agentID
}

// TenantExpertAgentFromID splits a published-expert ID back into its source
// agent ID; ok is false for every other ID (builtins and skillhub
// skillsets included).
func TenantExpertAgentFromID(id string) (string, bool) {
	rest, ok := strings.CutPrefix(id, tenantExpertIDPrefix)
	if !ok || rest == "" {
		return "", false
	}
	return rest, true
}

// PublishedAgentExport is the caller-supplied part of a publish: display
// overrides recorded on the publish row and the locale the persona segment
// renders at (the request locale; the rendering is frozen into the
// immutable snapshot).
type PublishedAgentExport struct {
	// Name overrides the agent's name for the expert label and the publish
	// row; empty means passthrough of the agent's own name.
	Name string
	// Description overrides the agent's description the same way.
	Description string
	// Locale selects the persona rendering (zh* → Chinese segments).
	Locale string
}

// personaUserDisplay is the persona template's user slot for a snapshot: at
// runtime the chatting user's id fills it per request, but a published
// snapshot has no such user, so it keeps RenderPersona's generic wording.
const personaUserDisplay = "the user"

// MaterializePublishedAgent maps one agent onto the MaterializedExpert the
// tenant market publishes. It is PURE: no disk, no network — the caller
// writes the tree via WriteMaterializedExpert.
//
// Binding (the M4 §5 whitelist contract):
//   - Manifest.ID = tenant-expert-<agentID> (TenantExpertID);
//   - Label/Description carry the override-or-agent value into BOTH locale
//     keys (the agent has one name/description pair — store what's there);
//   - PromptFiles = ["SOUL.md"]: the persona segment the runtime renders
//     from PersonaMBTI/PersonaStyle (persona.RenderPersona, skipped for
//     unknown codes exactly like renderPersonaSegment) joined with the
//     agent's SystemPrompt. Manifest.PersonaMBTI stays EMPTY so
//     instantiation never renders a second persona block;
//   - Skills = the agent's SelectedSkills names with SkillRefs=true —
//     references the installer resolves (installed → selected, otherwise
//     pending), never bundled payloads;
//   - Subagents = the agent's Subagents slugs;
//   - QuickPrompts = QuestionSuggestions.Starters.Items (both locale keys
//     carry the item verbatim);
//   - AgentConfig = minimal smart-reasoning (the MaterializeSkillset
//     contract — the expert is a persona+skills package, and per-agent
//     runtime knobs are outside the whitelist).
//
// EXCLUDED by the whitelist (asserted by tests): KnowledgeBases /
// KBSelectionMode, ModelID / RerankModelID / VLM/ASR models, all
// model-key-related config, SandboxConfigID, MemoryEnabled, MCP services.
func MaterializePublishedAgent(agent *types.CustomAgent, override PublishedAgentExport) (*MaterializedExpert, error) {
	if agent == nil {
		return nil, fmt.Errorf("experts: publish: agent is nil")
	}
	if !isSafeBaseName(agent.ID) {
		return nil, fmt.Errorf("experts: publish: agent id %q must be a plain directory name", agent.ID)
	}

	name := strings.TrimSpace(override.Name)
	if name == "" {
		name = strings.TrimSpace(agent.Name)
	}
	description := strings.TrimSpace(override.Description)
	if description == "" {
		description = strings.TrimSpace(agent.Description)
	}

	skills, err := referenceList(agent.Config.SelectedSkills, "skill")
	if err != nil {
		return nil, fmt.Errorf("experts: publish agent %q: %w", agent.ID, err)
	}
	subagents, err := referenceList(agent.Config.Subagents, "subagent")
	if err != nil {
		return nil, fmt.Errorf("experts: publish agent %q: %w", agent.ID, err)
	}

	var quickPrompts []QuickPrompt
	if suggestions := agent.Config.QuestionSuggestions; suggestions != nil {
		for _, item := range suggestions.Starters.Items {
			trimmed := strings.TrimSpace(item)
			if trimmed == "" {
				continue
			}
			quickPrompts = append(quickPrompts, QuickPrompt{
				Prompt: LocaleText{"zh": trimmed, "en": trimmed},
			})
		}
	}

	manifest := ExpertManifest{
		ID:           TenantExpertID(agent.ID),
		Label:        LocaleText{"zh": name, "en": name},
		Description:  LocaleText{"zh": description, "en": description},
		PromptFiles:  []string{soulFileName},
		QuickPrompts: quickPrompts,
		Skills:       skills,
		SkillRefs:    true,
		Subagents:    subagents,
		AgentConfig:  ExpertAgentConfig{AgentMode: types.AgentModeSmartReasoning},
	}

	return &MaterializedExpert{
		Manifest:     manifest,
		PersonaFiles: map[string][]byte{soulFileName: publishedSoul(agent, manifest, override.Locale)},
	}, nil
}

// referenceList trims, drops blanks and de-duplicates a reference list,
// rejecting entries that are not plain base names (they become directory
// names and yaml list entries — no separators, no escapes).
func referenceList(raw []string, kind string) ([]string, error) {
	out := make([]string, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, entry := range raw {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		if !isSafeBaseName(trimmed) {
			return nil, fmt.Errorf("%s reference %q must be a plain name", kind, entry)
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	return out, nil
}

// publishedSoul renders the persona document: the exported prompt material
// joined the way Instantiate joins prompt files. The persona segment is
// what the runtime would render (an unknown MBTI code renders nothing, the
// renderPersonaSegment rule), followed by the agent's system prompt. A
// header keeps the document non-empty even for a bare agent — the scanner
// rejects empty prompt files.
func publishedSoul(agent *types.CustomAgent, m ExpertManifest, locale string) []byte {
	parts := make([]string, 0, 3)
	parts = append(parts, "# "+m.Label["zh"])
	if desc := m.Description["zh"]; strings.TrimSpace(desc) != "" {
		parts = append(parts, desc)
	}
	if code := strings.TrimSpace(agent.Config.PersonaMBTI); code != "" {
		code = strings.ToUpper(code)
		if _, ok := persona.Profile(code); ok {
			parts = append(parts, persona.RenderPersona(code, locale, persona.RenderInput{
				AgentName:   m.Label["zh"],
				UserDisplay: personaUserDisplay,
				Custom:      agent.Config.PersonaStyle,
			}))
		}
	}
	if prompt := strings.TrimSpace(agent.Config.SystemPrompt); prompt != "" {
		parts = append(parts, prompt)
	}
	return []byte(strings.Join(parts, "\n\n") + "\n")
}

// publishedRootName is the published-experts root segment under the local
// data base dir. It deliberately sits OUTSIDE the expert-market root: a
// published snapshot is market inventory, not a tenant's installed expert,
// and must never surface in the experts catalog before an install copies
// it over (InstalledExperts scans <expert-market>/<tenantID>/ only).
const publishedRootName = "published-experts"

// PublishedDataRoot resolves the published-expert snapshot root:
// <$LOCAL_STORAGE_BASE_DIR>/published-experts (default
// /data/files/published-experts), the same one-data-root layout
// MarketDataRoot upholds.
func PublishedDataRoot() string {
	return filepath.Join(marketBaseDirOrDefault(), publishedRootName)
}

// PublishedTenantDir is the directory holding one tenant's published
// snapshots: <publishedRoot>/<tenantID>.
func PublishedTenantDir(publishedRoot string, tenantID uint64) string {
	return filepath.Join(publishedRoot, strconv.FormatUint(tenantID, 10))
}

// PublishedSnapshotDir is one published expert's immutable snapshot
// directory: <publishedRoot>/<tenantID>/<expertID>. The expert ID is
// validated [A-Za-z0-9_.-]-shaped by TenantExpertID's contract, so it is a
// safe single path segment.
func PublishedSnapshotDir(publishedRoot string, tenantID uint64, expertID string) string {
	return filepath.Join(PublishedTenantDir(publishedRoot, tenantID), expertID)
}
