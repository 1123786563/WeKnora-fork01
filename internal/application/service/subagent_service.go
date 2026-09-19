package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/Tencent/WeKnora/internal/agent/subagents"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// ErrSubagentNotFound is returned by SubagentService catalog lookups and
// installs when no builtin role matches the requested slug.
var ErrSubagentNotFound = errors.New("subagent not found")

// subagentService implements interfaces.SubagentService: the builtin role
// catalog (lazy, read-only, shared process-wide), the tenant's installed rows
// (TenantSubagentRepository), and the agent config mutations delegated to
// CustomAgentService (the persona Apply pattern: GetAgentByID, mutate
// Config, UpdateAgent).
type subagentService struct {
	catalog func() *subagents.Catalog
	agents  interfaces.CustomAgentService
	store   repository.TenantSubagentRepository
}

var _ interfaces.SubagentService = (*subagentService)(nil)

// NewSubagentService creates a SubagentService over a catalog function
// (usually subagents.LoadBuiltinSubagents), the shared CustomAgentService,
// and the tenant subagent repository.
func NewSubagentService(
	catalog func() *subagents.Catalog,
	agents interfaces.CustomAgentService,
	store repository.TenantSubagentRepository,
) interfaces.SubagentService {
	return &subagentService{catalog: catalog, agents: agents, store: store}
}

// ListCatalog projects the whole library for one tenant: entries slug-sorted
// (BySlug is a map, so the order must be made deterministic here), divisions
// in divisions.json order with per-division counts, total = distinct slugs,
// and Installed = the tenant has at least one row for the slug (any locale —
// the installed copy, not the library, is the source of truth).
func (s *subagentService) ListCatalog(ctx context.Context, tenantID uint64) (*interfaces.SubagentCatalog, error) {
	catalog := s.catalog()
	rows, err := s.installedSlugs(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	entries := make([]interfaces.SubagentCatalogEntry, 0, len(catalog.BySlug))
	counts := make(map[string]int, len(catalog.Divisions))
	for slug, entry := range catalog.BySlug {
		entries = append(entries, subagentListEntry(slug, entry, rows[slug]))
		counts[entry.Division]++
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Slug < entries[j].Slug })

	divisions := make([]interfaces.SubagentCatalogDivision, 0, len(catalog.Divisions))
	for _, d := range catalog.Divisions {
		divisions = append(divisions, interfaces.SubagentCatalogDivision{
			Slug:  d.Slug,
			Label: d.Label,
			Icon:  d.Icon,
			Color: d.Color,
			Count: counts[d.Slug],
		})
	}
	return &interfaces.SubagentCatalog{
		Divisions: divisions,
		Total:     len(catalog.BySlug),
		Entries:   entries,
	}, nil
}

// GetCatalogEntry returns one role's detail: frontmatter scalars from the zh
// definition (en-only roles fall back to en — the zh tree ships the
// authoritative metadata, including divisions.json) and both locale bodies
// present-or-empty, so the client filters presentation locale-side.
func (s *subagentService) GetCatalogEntry(ctx context.Context, tenantID uint64, slug string) (*interfaces.SubagentCatalogDetail, error) {
	entry := s.catalog().BySlug[slug]
	if entry == nil {
		return nil, ErrSubagentNotFound
	}
	rows, err := s.installedSlugs(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	detail := &interfaces.SubagentCatalogDetail{
		Slug:      entry.Slug,
		Division:  entry.Division,
		Installed: rows[entry.Slug],
	}
	if entry.Zh != nil {
		detail.NameZh = entry.Zh.Frontmatter.Name
		detail.BodyZh = string(entry.Zh.Body)
	}
	if entry.En != nil {
		detail.NameEn = entry.En.Frontmatter.Name
		detail.BodyEn = string(entry.En.Body)
	}
	// Scalar frontmatter: prefer zh, fall back to en.
	for _, def := range []*subagents.SubagentDefinition{entry.Zh, entry.En} {
		if def == nil {
			continue
		}
		detail.Emoji = def.Frontmatter.Emoji
		detail.Color = def.Frontmatter.Color
		detail.Vibe = def.Frontmatter.Vibe
		detail.ToolsRaw = def.Frontmatter.ToolsRaw
		break
	}
	return detail, nil
}

// ListAgentSubagents returns the agent's configured slugs in config order —
// the delegate tool registration preserves this order, so the API must too.
func (s *subagentService) ListAgentSubagents(ctx context.Context, agentID string) ([]string, error) {
	agent, err := s.agent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	return copySlugs(agent.Config.Subagents), nil
}

// InstallForAgent resolves the role through the catalog fallback chain
// (requested locale → en → zh), upserts the resolved definition into the
// tenant's rows, and appends the slug to the agent config when absent.
//
// Ordering: the row is upserted BEFORE the agent update. A failed
// UpdateAgent can leave an orphan row, which is harmless — rows are shared
// across agents and idempotent to rewrite — while the reverse order could
// persist a config naming a role the tenant has no row for (delegation would
// silently stay off for it).
func (s *subagentService) InstallForAgent(
	ctx context.Context, tenantID uint64, agentID, slug, locale string,
) ([]string, error) {
	slug = strings.TrimSpace(slug)
	locale = resolveSubagentLocale(ctx, locale)
	def := s.catalog().Resolve(slug, locale)
	if def == nil {
		return nil, ErrSubagentNotFound
	}

	agent, err := s.agent(ctx, agentID)
	if err != nil {
		return nil, err
	}

	content, err := roleMarkdown(def)
	if err != nil {
		return nil, err
	}
	// Fresh UUID per upsert (T3 hand-off): the repo's conflict target keeps a
	// live slot's existing ID, so this ID lands only on first install.
	if err := s.store.Upsert(ctx, &types.TenantSubagentEntity{
		ID:       uuid.NewString(),
		TenantID: tenantID,
		Slug:     def.Slug,
		Locale:   def.Locale,
		Content:  content,
		Division: def.Division,
		Source:   types.SubagentSourceBuiltin,
	}); err != nil {
		return nil, fmt.Errorf("subagents: install %q for tenant %d: upsert row: %w", def.Slug, tenantID, err)
	}

	if !subagentConfigured(agent.Config.Subagents, def.Slug) {
		agent.Config.Subagents = append(copySlugs(agent.Config.Subagents), def.Slug)
		if updated, err := s.agents.UpdateAgent(ctx, agent); err != nil {
			return nil, fmt.Errorf("subagents: install %q on agent %s: update agent: %w", def.Slug, agentID, err)
		} else if updated != nil {
			agent = updated
		}
	} else {
		logger.Infof(ctx, "subagents: %q already configured on agent %s (install refreshed the tenant row only)",
			def.Slug, agentID)
	}
	return copySlugs(agent.Config.Subagents), nil
}

// RemoveFromAgent drops slug from the agent's config. The tenant row is
// deliberately KEPT: rows are shared across agents (and across reinstalls),
// so one agent's removal must not break another's delegation; row deletion
// is tenant-admin scope, not this endpoint. An absent slug is a no-op.
func (s *subagentService) RemoveFromAgent(ctx context.Context, agentID, slug string) ([]string, error) {
	agent, err := s.agent(ctx, agentID)
	if err != nil {
		return nil, err
	}

	kept := make([]string, 0, len(agent.Config.Subagents))
	removed := false
	for _, configured := range agent.Config.Subagents {
		if strings.TrimSpace(configured) == strings.TrimSpace(slug) {
			removed = true
			continue
		}
		kept = append(kept, configured)
	}
	if removed {
		agent.Config.Subagents = kept
		if _, err := s.agents.UpdateAgent(ctx, agent); err != nil {
			return nil, fmt.Errorf("subagents: remove %q from agent %s: update agent: %w", slug, agentID, err)
		}
	}
	return copySlugs(agent.Config.Subagents), nil
}

// agent loads the agent through CustomAgentService, mapping the not-found
// sentinel (and a nil agent, which some service paths return without an
// error) onto ErrAgentNotFound for the handler's 404.
func (s *subagentService) agent(ctx context.Context, agentID string) (*types.CustomAgent, error) {
	agent, err := s.agents.GetAgentByID(ctx, agentID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, ErrAgentNotFound
	}
	return agent, nil
}

// installedSlugs reduces the tenant's rows to a slug → installed map.
func (s *subagentService) installedSlugs(ctx context.Context, tenantID uint64) (map[string]bool, error) {
	rows, err := s.store.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("subagents: list tenant %d rows: %w", tenantID, err)
	}
	installed := make(map[string]bool, len(rows))
	for _, row := range rows {
		installed[row.Slug] = true
	}
	return installed, nil
}

// subagentListEntry projects one catalog entry for the list view.
func subagentListEntry(slug string, entry *subagents.CatalogEntry, installed bool) interfaces.SubagentCatalogEntry {
	out := interfaces.SubagentCatalogEntry{
		Slug:      slug,
		Division:  entry.Division,
		Installed: installed,
	}
	// Scalar display fields: prefer zh, fall back to en.
	for _, def := range []*subagents.SubagentDefinition{entry.Zh, entry.En} {
		if def != nil {
			out.Emoji = def.Frontmatter.Emoji
			out.Color = def.Frontmatter.Color
			break
		}
	}
	if entry.Zh != nil {
		out.NameZh = entry.Zh.Frontmatter.Name
	}
	if entry.En != nil {
		out.NameEn = entry.En.Frontmatter.Name
	}
	return out
}

// resolveSubagentLocale picks the locale the catalog resolves with: the
// explicit request value when given, else the request context's language,
// then normalizes any Accept-Language-style tag ("zh-CN", "en-US") onto the
// library's locale keys ("zh"/"en" — the same rule subagentLocale applies).
func resolveSubagentLocale(ctx context.Context, locale string) string {
	return normalizeSubagentLocale(types.ResolveLanguage(ctx, locale))
}

func normalizeSubagentLocale(locale string) string {
	if strings.HasPrefix(locale, "zh") {
		return "zh"
	}
	return "en"
}

// roleMarkdown re-renders one definition as the full role document the
// tenant_subagents Content column stores: "---" fences around the frontmatter
// (re-marshaled from the parsed struct — the scanner drops unknown
// frontmatter keys by contract) with the body bytes verbatim below the
// closing fence. The result round-trips through ParseRoleMarkdown, which is
// the runtime contract (the delegate tool parses installed rows back into
// frontmatter + sub-run system prompt).
func roleMarkdown(def *subagents.SubagentDefinition) (string, error) {
	if def == nil {
		return "", errors.New("subagents: nil role definition")
	}
	fm, err := yaml.Marshal(def.Frontmatter)
	if err != nil {
		return "", fmt.Errorf("subagents: render role frontmatter %q: %w", def.Slug, err)
	}
	var b strings.Builder
	b.WriteString("---\n")
	b.Write(fm) // yaml.Marshal output ends with a newline
	b.WriteString("---\n")
	b.Write(def.Body)
	return b.String(), nil
}

// subagentConfigured reports whether the exact slug is already configured.
// Trims are tolerated on both sides so a hand-edited config with stray
// whitespace still counts as the same role.
func subagentConfigured(configured []string, slug string) bool {
	for _, existing := range configured {
		if strings.TrimSpace(existing) == slug {
			return true
		}
	}
	return false
}

// copySlugs always returns a non-nil copy, so JSON responses serialize []
// rather than null for an empty configuration.
func copySlugs(slugs []string) []string {
	out := make([]string, 0, len(slugs))
	out = append(out, slugs...)
	return out
}
