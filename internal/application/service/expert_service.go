package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Tencent/WeKnora/internal/agent/experts"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// ErrExpertNotFound is returned by ExpertService.GetExpert/Instantiate when no
// expert template matches the requested ID.
var ErrExpertNotFound = errors.New("expert not found")

const (
	// expertSourceBuiltin stamps agents instantiated from the shipped
	// config/experts templates.
	expertSourceBuiltin = "builtin"
	// expertPersonaFileSeparator joins an expert's persona documents (and
	// the agent-config system prompt) into one markdown system prompt.
	expertPersonaFileSeparator = "\n\n---\n\n"
	// skillsSelectionModeSelected mirrors the "selected" value of
	// CustomAgentConfig.SkillsSelectionMode (no shared constant exists).
	skillsSelectionModeSelected = "selected"
)

// ExpertSkillResolution is what an ExpertSkillResolver produced for one
// expert instantiation: the skill names the agent should pin, install jobs
// started for bundled skills, and slugs still waiting for those installs.
type ExpertSkillResolution struct {
	Selected   []string
	InstallIDs []string
	Pending    []string
}

// ExpertSkillResolver installs/resolves an expert's bundled skills for a
// tenant. Task 4 provides the real implementation (bundled-skill install
// into the tenant sandbox image); until then the noop resolver below simply
// selects the manifest skill names as-is.
type ExpertSkillResolver interface {
	ResolveExpertSkills(ctx context.Context, tenantID uint64, e *experts.Expert) (ExpertSkillResolution, error)
}

// noopSkillResolver is the Task-3 stand-in ExpertSkillResolver: manifest
// skill names are selected as-is, nothing is installed, nothing is pending.
type noopSkillResolver struct{}

func (noopSkillResolver) ResolveExpertSkills(_ context.Context, _ uint64, e *experts.Expert) (ExpertSkillResolution, error) {
	return ExpertSkillResolution{Selected: append([]string(nil), e.Manifest.Skills...)}, nil
}

// expertService implements interfaces.ExpertService.
type expertService struct {
	catalog func() []*experts.Expert
	agents  interfaces.CustomAgentService
	skills  ExpertSkillResolver
}

var _ interfaces.ExpertService = (*expertService)(nil)

// NewExpertService creates an ExpertService over a catalog function (usually
// experts.LoadBuiltinExperts), delegating agent creation to the
// CustomAgentService and skill handling to the resolver. A nil skills
// resolver falls back to the no-op one (select manifest names, no installs).
func NewExpertService(
	catalog func() []*experts.Expert,
	agents interfaces.CustomAgentService,
	skills ExpertSkillResolver,
) interfaces.ExpertService {
	if skills == nil {
		skills = noopSkillResolver{}
	}
	return &expertService{catalog: catalog, agents: agents, skills: skills}
}

// ListExperts returns every expert the catalog exposes.
func (s *expertService) ListExperts(_ context.Context) ([]*experts.Expert, error) {
	return s.catalog(), nil
}

// GetExpert returns the expert with the given manifest ID.
func (s *expertService) GetExpert(_ context.Context, id string) (*experts.Expert, error) {
	if e := s.findExpert(id); e != nil {
		return e, nil
	}
	return nil, ErrExpertNotFound
}

func (s *expertService) findExpert(id string) *experts.Expert {
	for _, e := range s.catalog() {
		if e != nil && e.Manifest.ID == id {
			return e
		}
	}
	return nil
}

// Instantiate creates a tenant custom agent from an expert template:
// resolve the expert → build the agent (pure mapping) → CreateAgent (which
// fills defaults, validates and persists) → resolve bundled skills for the
// tenant. The locale for label/description/starter resolution comes from ctx.
func (s *expertService) Instantiate(
	ctx context.Context,
	tenantID uint64,
	expertID string,
	req interfaces.InstantiateRequest,
) (*interfaces.InstantiateResult, error) {
	e := s.findExpert(expertID)
	if e == nil {
		return nil, ErrExpertNotFound
	}

	locale := types.LanguageFromContextOrDefault(ctx)
	agent := buildAgentFromExpert(e, locale, req.AgentName)
	if req.SandboxConfigID != "" {
		agent.Config.SandboxConfigID = req.SandboxConfigID
	}

	created, err := s.agents.CreateAgent(ctx, agent)
	if err != nil {
		return nil, fmt.Errorf("experts: instantiate %q: create agent: %w", expertID, err)
	}

	resolution, err := s.skills.ResolveExpertSkills(ctx, tenantID, e)
	if err != nil {
		return nil, fmt.Errorf("experts: instantiate %q: resolve skills: %w", expertID, err)
	}

	logger.Infof(ctx, "Instantiated expert %q as agent %s (tenant %d)",
		expertID, created.ID, tenantID)
	return &interfaces.InstantiateResult{
		Agent:           created,
		PendingSkills:   resolution.Pending,
		SkillInstallIDs: resolution.InstallIDs,
	}, nil
}

// buildAgentFromExpert maps an expert template onto a CustomAgent for one
// locale. It is pure: no context, no DB, and it never mutates (or aliases
// slices from) the expert — experts returned by experts.LoadBuiltinExperts
// are shared process-wide and must stay read-only. Everything CreateAgent
// would default (ID, tenant, timestamps, retrieval knobs, follow-up
// suggestions) is deliberately left zero.
//
// Mapping:
//   - locale-resolved label → Name (nameOverride, when non-blank, wins);
//     description → Description; icon_name → Avatar
//   - prompt files joined with "\n\n---\n\n" (in manifest order), with the
//     agent_config system prompt appended after the same separator →
//     Config.SystemPrompt
//   - quick prompts (locale-resolved prompt text) → fully-formed curated
//     QuestionSuggestions.Starters
//   - agent_config overrides: non-zero strings/numbers and true booleans
//     only — zero values leave the CreateAgent defaults untouched
//   - PersonaMBTI passthrough
//   - SkillsSelectionMode="selected" + SelectedSkills=manifest skills, only
//     when the expert bundles skills
//   - ExpertSource{ExpertID, Source:"builtin"} provenance
func buildAgentFromExpert(e *experts.Expert, locale, nameOverride string) *types.CustomAgent {
	m := e.Manifest

	name := strings.TrimSpace(nameOverride)
	if name == "" {
		name = resolveExpertLocaleText(m.Label, locale)
	}

	agent := &types.CustomAgent{
		Name:        name,
		Description: resolveExpertLocaleText(m.Description, locale),
		Avatar:      m.IconName,
	}
	agent.Config.PersonaMBTI = m.PersonaMBTI

	// System prompt: persona documents in manifest order, then the expert's
	// agent-config prompt. Long items are never truncated here — if a
	// template ships oversized starter prompts, CreateAgent's Validate is
	// the loud failure point.
	promptParts := make([]string, 0, len(m.PromptFiles)+1)
	for _, file := range m.PromptFiles {
		if content, ok := e.PersonaFiles[file]; ok {
			promptParts = append(promptParts, string(content))
		}
	}
	if m.AgentConfig.SystemPrompt != "" {
		promptParts = append(promptParts, m.AgentConfig.SystemPrompt)
	}
	if len(promptParts) > 0 {
		agent.Config.SystemPrompt = strings.Join(promptParts, expertPersonaFileSeparator)
	}

	// Agent-config overrides: zero value means "leave CreateAgent defaults".
	// Bools apply only when true — false is indistinguishable from unset.
	if m.AgentConfig.AgentMode != "" {
		agent.Config.AgentMode = m.AgentConfig.AgentMode
	}
	if m.AgentConfig.KBSelectionMode != "" {
		agent.Config.KBSelectionMode = m.AgentConfig.KBSelectionMode
	}
	if m.AgentConfig.Temperature != 0 {
		agent.Config.Temperature = m.AgentConfig.Temperature
	}
	if m.AgentConfig.MaxIterations != 0 {
		agent.Config.MaxIterations = m.AgentConfig.MaxIterations
	}
	if m.AgentConfig.WebSearchEnabled {
		agent.Config.WebSearchEnabled = true
	}
	if m.AgentConfig.MultiTurnEnabled {
		agent.Config.MultiTurnEnabled = true
	}

	// Starter suggestions: fully formed up front because CreateAgent's
	// EnsureDefaults only fills a nil QuestionSuggestions — a half-built one
	// would keep zero Starters. Curated mode + exact count keeps the
	// manifest prompts verbatim. Zero resolvable prompts must leave the
	// field nil instead: curated mode requires Count >= 1, so a hand-made
	// empty set would fail Validate.
	starterItems := make([]string, 0, len(m.QuickPrompts))
	for _, qp := range m.QuickPrompts {
		if prompt := resolveExpertLocaleText(qp.Prompt, locale); prompt != "" {
			starterItems = append(starterItems, prompt)
		}
	}
	if len(starterItems) > 0 {
		agent.Config.QuestionSuggestions = &types.QuestionSuggestionConfig{
			Starters: types.StarterSuggestionConfig{
				Enabled: true,
				Mode:    types.SuggestionModeCurated,
				Items:   starterItems,
				Count:   len(starterItems),
			},
		}
	}

	// Skills: pin exactly the bundled skills; experts without skills leave
	// the selection mode untouched (CreateAgent's defaults apply).
	if len(m.Skills) > 0 {
		agent.Config.SkillsSelectionMode = skillsSelectionModeSelected
		agent.Config.SelectedSkills = append([]string(nil), m.Skills...)
	}

	agent.Config.ExpertSource = &types.ExpertSourceStruct{
		ExpertID: m.ID,
		Source:   expertSourceBuiltin,
	}
	return agent
}

// resolveExpertLocaleText picks the best locale match from an experts
// LocaleText. types.resolveI18n implements the same rule for the builtin
// agents' i18n map but is unexported (and typed for BuiltinAgentI18n), so the
// rule is restated here: exact match → language-prefix match → "en" → first
// key (sorted, so a single-locale manifest resolves deterministically).
func resolveExpertLocaleText(m experts.LocaleText, locale string) string {
	if len(m) == 0 {
		return ""
	}
	// 1. Exact match (e.g. "zh-CN").
	if v, ok := m[locale]; ok {
		return v
	}
	// 2. Language-only match ("zh-CN" → "zh", then any same-prefix key).
	if idx := strings.IndexAny(locale, "-_"); idx > 0 {
		lang := locale[:idx]
		if v, ok := m[lang]; ok {
			return v
		}
		for _, k := range sortedLocaleKeys(m) {
			if strings.HasPrefix(k, lang) {
				return m[k]
			}
		}
	}
	// 3. English fallback — shipped experts always carry zh/en.
	if v, ok := m["en"]; ok {
		return v
	}
	// 4. First key, deterministically.
	keys := sortedLocaleKeys(m)
	return m[keys[0]]
}

func sortedLocaleKeys(m experts.LocaleText) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
