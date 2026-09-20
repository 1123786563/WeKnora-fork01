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
	// expertSourceSkillhub stamps agents instantiated from experts a tenant
	// installed out of the SkillHub market (M4).
	expertSourceSkillhub = "skillhub"
	// expertSourceTenant stamps agents instantiated from experts a member
	// published to the tenant-internal market (M4 Task 5).
	expertSourceTenant = "tenant"
	// expertPersonaFileSeparator joins an expert's persona documents (and
	// the agent-config system prompt) into one markdown system prompt.
	expertPersonaFileSeparator = "\n\n---\n\n"
	// skillsSelectionModeSelected mirrors the "selected" value of
	// CustomAgentConfig.SkillsSelectionMode (no shared constant exists).
	skillsSelectionModeSelected = "selected"
	// expertStartersValidationLimit mirrors the upper bound
	// QuestionSuggestionConfig.Validate enforces on curated starters (no
	// shared constant exists there). Manifests may ship more quick prompts
	// than this (general-assistant ships 12); Instantiate keeps the first
	// ones in manifest order rather than failing CreateAgent validation.
	expertStartersValidationLimit = 8
)

// ExpertSource supplies the experts one tenant installed from the skill
// market (M4). It is the per-tenant half of the catalog: ExpertService keeps
// its tenant-blind builtin catalog function and unions this lookup on top,
// because the injected catalog func (M2 seam) has no tenant parameter.
// Implementations must be degraded-mode — scan trouble answers an empty
// list, never an error — so a broken install directory cannot take the
// whole catalog down.
type ExpertSource interface {
	// InstalledExperts returns the tenant's installed experts. The returned
	// slice and experts are shared read-only; callers must not mutate.
	InstalledExperts(ctx context.Context, tenantID uint64) []*experts.Expert
}

// ExpertSkillResolution is what an ExpertSkillResolver produced for one
// expert instantiation: the skill names the agent should pin, install jobs
// started for bundled skills, and slugs still waiting for those installs
// (including the ones just kicked off — an async install is not usable
// until it lands).
type ExpertSkillResolution struct {
	Selected   []string
	InstallIDs []string
	Pending    []string
}

// ExpertSkillResolver installs/resolves an expert's bundled skills for a
// tenant on the given sandbox config ("" means no sandbox: everything
// resolves to selected-and-pending). The production implementation is
// NewBundledSkillResolver; the noop below simply selects the manifest skill
// names as-is.
type ExpertSkillResolver interface {
	ResolveExpertSkills(ctx context.Context, tenantID uint64, sandboxConfigID string, e *experts.Expert) (ExpertSkillResolution, error)
}

// noopSkillResolver is the Task-3 stand-in ExpertSkillResolver: manifest
// skill names are selected as-is, nothing is installed, nothing is pending.
type noopSkillResolver struct{}

func (noopSkillResolver) ResolveExpertSkills(_ context.Context, _ uint64, _ string, e *experts.Expert) (ExpertSkillResolution, error) {
	return ExpertSkillResolution{Selected: append([]string(nil), e.Manifest.Skills...)}, nil
}

// expertService implements interfaces.ExpertService.
type expertService struct {
	catalog func() []*experts.Expert
	agents  interfaces.CustomAgentService
	skills  ExpertSkillResolver
	// installed is the optional per-tenant expert source (M4 market); nil
	// keeps the service builtin-only, exactly as before the seam existed.
	installed ExpertSource
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
	return newExpertService(catalog, agents, skills, nil)
}

// NewExpertServiceWithSource additionally unions a per-tenant ExpertSource
// (M4 skill market) onto the builtin catalog: listing and lookups serve
// builtin ∪ installed, with builtin precedence when IDs collide. The source
// is only consulted for requests that carry a tenant (ListExperts/GetExpert
// read it from ctx; Instantiate uses its explicit tenantID).
func NewExpertServiceWithSource(
	catalog func() []*experts.Expert,
	agents interfaces.CustomAgentService,
	skills ExpertSkillResolver,
	installed ExpertSource,
) interfaces.ExpertService {
	return newExpertService(catalog, agents, skills, installed)
}

func newExpertService(
	catalog func() []*experts.Expert,
	agents interfaces.CustomAgentService,
	skills ExpertSkillResolver,
	installed ExpertSource,
) interfaces.ExpertService {
	if skills == nil {
		skills = noopSkillResolver{}
	}
	return &expertService{catalog: catalog, agents: agents, skills: skills, installed: installed}
}

// ListExperts returns builtin experts plus, when the request carries a
// tenant and a source is wired, that tenant's installed market experts.
// Builtin experts keep precedence: an installed expert whose manifest ID
// collides with a builtin is shadowed, never merged.
func (s *expertService) ListExperts(ctx context.Context) ([]*experts.Expert, error) {
	builtin := s.catalog()
	installed := s.ctxInstalledExperts(ctx)
	if len(installed) == 0 {
		return builtin, nil
	}
	return mergeExperts(builtin, installed), nil
}

// GetExpert returns the expert with the given manifest ID.
func (s *expertService) GetExpert(ctx context.Context, id string) (*experts.Expert, error) {
	tenantID, _ := types.TenantIDFromContext(ctx)
	if e := s.findExpert(ctx, tenantID, id); e != nil {
		return e, nil
	}
	return nil, ErrExpertNotFound
}

// findExpert looks id up in the builtin catalog first, then in the given
// tenant's installed experts. Instantiate passes its explicit tenantID; the
// ctx-only callers resolve it before getting here.
func (s *expertService) findExpert(ctx context.Context, tenantID uint64, id string) *experts.Expert {
	if e := findExpertIn(s.catalog(), id); e != nil {
		return e
	}
	if s.installed == nil {
		return nil
	}
	return findExpertIn(s.installed.InstalledExperts(ctx, tenantID), id)
}

// ctxInstalledExperts answers the requesting tenant's installed experts, or
// nil when no source is wired, the request carries no tenant, or the source
// answers nothing.
func (s *expertService) ctxInstalledExperts(ctx context.Context) []*experts.Expert {
	if s.installed == nil {
		return nil
	}
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok {
		return nil
	}
	return s.installed.InstalledExperts(ctx, tenantID)
}

// findExpertIn scans one slice for a manifest ID match.
func findExpertIn(list []*experts.Expert, id string) *experts.Expert {
	for _, e := range list {
		if e != nil && e.Manifest.ID == id {
			return e
		}
	}
	return nil
}

// mergeExperts unions builtin and installed experts by manifest ID with
// builtin precedence, preserving order (builtins first, then installed in
// source order).
func mergeExperts(builtin, installed []*experts.Expert) []*experts.Expert {
	seen := make(map[string]bool, len(builtin))
	for _, e := range builtin {
		if e != nil {
			seen[e.Manifest.ID] = true
		}
	}
	merged := make([]*experts.Expert, 0, len(builtin)+len(installed))
	merged = append(merged, builtin...)
	for _, e := range installed {
		if e == nil || seen[e.Manifest.ID] {
			continue
		}
		seen[e.Manifest.ID] = true
		merged = append(merged, e)
	}
	return merged
}

// ResolveExpertLocaleText exposes the expert locale-text resolution rule to
// the API layer so the list/detail projections and Instantiate resolve
// identical strings for one request locale. The rule stays defined once, in
// resolveExpertLocaleText below.
func ResolveExpertLocaleText(m experts.LocaleText, locale string) string {
	return resolveExpertLocaleText(m, locale)
}

// ExpertPersonaFileSeparator is the join the API detail preview uses so the
// preview shows exactly the persona-files part of what Instantiate writes
// into Config.SystemPrompt.
const ExpertPersonaFileSeparator = expertPersonaFileSeparator

// Instantiate creates a tenant custom agent from an expert template:
// resolve the expert → build the agent (pure mapping) → resolve bundled
// skills for the tenant → CreateAgent (which fills defaults, validates and
// persists). The locale for label/description/starter resolution comes from
// ctx.
//
// Skills resolve BEFORE the agent is persisted so the record carries the
// resolved install names the runtime matches AllowedSkills against — the
// SKILL.md frontmatter name, which may differ from the manifest slug. The
// cost of this order is that a later CreateAgent failure leaves the already
// started installs running; they are tenant-scoped and idempotent (a ready
// archive is skipped without billing a sandbox), while the reverse order
// would need a second write to patch SelectedSkills and could leave the
// persisted agent pinned to slugs no installed skill answers to.
func (s *expertService) Instantiate(
	ctx context.Context,
	tenantID uint64,
	expertID string,
	req interfaces.InstantiateRequest,
) (*interfaces.InstantiateResult, error) {
	e := s.findExpert(ctx, tenantID, expertID)
	if e == nil {
		return nil, ErrExpertNotFound
	}

	locale := types.LanguageFromContextOrDefault(ctx)
	agent := buildAgentFromExpert(e, locale, req.AgentName)
	if req.SandboxConfigID != "" {
		agent.Config.SandboxConfigID = req.SandboxConfigID
	}

	resolution, err := s.skills.ResolveExpertSkills(ctx, tenantID, req.SandboxConfigID, e)
	if err != nil {
		return nil, fmt.Errorf("experts: instantiate %q: resolve skills: %w", expertID, err)
	}
	// The resolver names one selection per bundled skill; a resolver that
	// produced none leaves the builder's manifest skills in place.
	if len(resolution.Selected) > 0 {
		agent.Config.SelectedSkills = append([]string(nil), resolution.Selected...)
	}

	created, err := s.agents.CreateAgent(ctx, agent)
	if err != nil {
		return nil, fmt.Errorf("experts: instantiate %q: create agent: %w", expertID, err)
	}

	logger.Infof(ctx, "Instantiated expert %q as agent %s (tenant %d, skills: %d selected, %d install(s) started, %d pending)",
		expertID, created.ID, tenantID,
		len(resolution.Selected), len(resolution.InstallIDs), len(resolution.Pending))
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
//     QuestionSuggestions.Starters, capped at the first 8 prompts in
//     manifest order (CreateAgent's Validate limit; overflow is dropped
//     with a warn log, the manifest itself is never truncated)
//   - agent_config overrides: non-zero strings/numbers and true booleans
//     only — zero values leave the CreateAgent defaults untouched
//   - PersonaMBTI passthrough
//   - SkillsSelectionMode="selected" + SelectedSkills=manifest skills, only
//     when the expert bundles skills
//   - ExpertSource provenance: builtin for shipped templates, skillhub +
//     the skillset slug for market-installed ones (expertProvenance)
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
	// Cap at the validation limit: CreateAgent's Validate rejects curated
	// starters beyond 8, and some shipped manifests carry more quick
	// prompts than that (general-assistant ships 12). Keep the first 8 in
	// manifest order and warn about the drop — the manifest itself stays
	// verbatim (asset fidelity). builder is context-free by contract, so the
	// warning rides a background context like the experts loader's does.
	if len(starterItems) > expertStartersValidationLimit {
		logger.Warnf(context.Background(),
			"experts: instantiate %q: manifest carries %d quick prompts, keeping the first %d and dropping the remaining %d (starter validation limit)",
			m.ID, len(starterItems), expertStartersValidationLimit, len(starterItems)-expertStartersValidationLimit)
		starterItems = starterItems[:expertStartersValidationLimit]
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

	// Subagents (the M4 tenant-publish manifest extension): delegation
	// slugs ride in the manifest yaml and map onto the agent config
	// verbatim. A copied slice — experts are shared read-only.
	if len(m.Subagents) > 0 {
		agent.Config.Subagents = append([]string(nil), m.Subagents...)
	}

	agent.Config.ExpertSource = expertProvenance(m.ID)
	return agent
}

// expertProvenance stamps where a template came from: builtin experts carry
// Source "builtin"; market-installed experts (IDs under the
// skillhub-skillset- prefix) carry Source "skillhub" plus the skillset slug;
// tenant-published experts (IDs under the tenant-expert- prefix, M4 Task 5)
// carry Source "tenant" plus the source agent ID — which disambiguates
// same-ID templates across all three sources.
func expertProvenance(expertID string) *types.ExpertSourceStruct {
	if slug, ok := experts.MarketExpertSlugFromID(expertID); ok {
		return &types.ExpertSourceStruct{
			ExpertID: expertID,
			Source:   expertSourceSkillhub,
			Slug:     slug,
		}
	}
	if agentID, ok := experts.TenantExpertAgentFromID(expertID); ok {
		return &types.ExpertSourceStruct{
			ExpertID: expertID,
			Source:   expertSourceTenant,
			Slug:     agentID,
		}
	}
	return &types.ExpertSourceStruct{
		ExpertID: expertID,
		Source:   expertSourceBuiltin,
	}
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
