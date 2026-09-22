package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/subagents"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/tools"
	"github.com/Tencent/WeKnora/internal/modules/airesource/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
)

// registerSubagentDelegateTool opens the subagent_delegate tool for sessions
// whose agent configures at least one specialist role that is also installed
// for the tenant. Registration is conditional (the craft_delegate precedent):
//
//   - empty config.Subagents keeps delegation off entirely;
//   - the installed rows come from TenantSubagentRepository.ListByTenant —
//     the installed copy IS the source of truth, so shipping an updated
//     library never changes a tenant's roles until a sync writes new rows;
//   - zero overlap between config and installed rows leaves the tool off.
//
// Identity (session, user, model, tool whitelist, executor, event bus) is
// assembled here from the run context; the model only ever supplies slug,
// goal and already-authorized input references. The tool intersects each
// role's declared tools with the main run's effective allowlist and pulls the
// constructed instances from the registry at delegation time, so a sub-agent
// never gets a tool the main agent could not call itself.
func (s *agentService) registerSubagentDelegateTool(
	ctx context.Context,
	registry *tools.ToolRegistry,
	config *types.AgentConfig,
	chatModel chat.Chat,
	eventBus *event.EventBus,
	sessionID string,
) error {
	if s == nil || registry == nil || config == nil || len(config.Subagents) == 0 {
		return nil
	}
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || tenantID == 0 {
		logger.Warnf(ctx, "subagent_delegate not registered: no tenant on context for session %s", sessionID)
		return nil
	}
	if s.db == nil {
		logger.Warnf(ctx, "subagent_delegate not registered: no database handle on the agent service")
		return nil
	}
	if chatModel == nil {
		logger.Warnf(ctx, "subagent_delegate not registered: no chat model for session %s", sessionID)
		return nil
	}
	rows, err := repository.NewTenantSubagentRepository(s.db).ListByTenant(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("subagent_delegate gate: list installed subagents: %w", err)
	}
	resolved := resolveTenantSubagentRoles(ctx, rows, config.Subagents, subagentLocale(ctx))
	if len(resolved) == 0 {
		logger.Infof(ctx,
			"subagent_delegate not registered: none of the configured subagents is installed for tenant %d", tenantID)
		return nil
	}

	// Preserve the agent config's order for the description; the same order
	// decides which slugs the model is offered.
	slugs := make([]string, 0, len(resolved))
	for _, slug := range config.Subagents {
		if _, ok := resolved[strings.TrimSpace(slug)]; ok {
			slugs = append(slugs, strings.TrimSpace(slug))
		}
	}
	lookup := func(slug string) (subagents.ResolvedRole, error) {
		role, ok := resolved[slug]
		if !ok {
			return subagents.ResolvedRole{}, fmt.Errorf("%w: %s", subagents.ErrSubagentNotInstalled, slug)
		}
		return role, nil
	}
	userID, _ := types.UserIDFromContext(ctx)
	tool, err := subagents.NewSubagentDelegateTool(subagents.SubagentDelegateToolConfig{
		SessionID:    sessionID,
		UserID:       userID,
		Slugs:        slugs,
		AllowedTools: effectiveAllowedTools(config),
		Registry:     registry,
		Model:        chatModel,
		Lookup:       lookup,
		Exec:         subagents.Execute,
		Emit:         subagentEmitAdapter(eventBus),
	})
	if err != nil {
		return fmt.Errorf("subagent_delegate gate: %w", err)
	}
	registry.RegisterTool(tool)
	logger.Infof(ctx, "subagent_delegate registered for session %s with specialists %v", sessionID, slugs)
	return nil
}

// subagentLocale maps the request locale onto the library's locale keys
// ("zh"/"en"); anything not zh-* resolves as en, matching the frontends'
// two-language surface.
func subagentLocale(ctx context.Context) string {
	if strings.HasPrefix(types.LanguageFromContextOrDefault(ctx), "zh") {
		return "zh"
	}
	return "en"
}

// resolveTenantSubagentRoles reduces the tenant's installed role rows to the
// configured slugs, resolving each slug to the row matching the requested
// locale with the en → zh fallback (Octop ships zh-only roles, so en users
// still see them). Rows that fail to parse as role markdown are skipped with
// a warning rather than failing the whole capability.
func resolveTenantSubagentRoles(
	ctx context.Context,
	rows []types.TenantSubagentEntity,
	wanted []string,
	locale string,
) map[string]subagents.ResolvedRole {
	bySlug := make(map[string]map[string]types.TenantSubagentEntity, len(rows))
	for _, row := range rows {
		if row.Slug == "" || strings.TrimSpace(row.Content) == "" {
			continue
		}
		locales, ok := bySlug[row.Slug]
		if !ok {
			locales = make(map[string]types.TenantSubagentEntity)
			bySlug[row.Slug] = locales
		}
		locales[row.Locale] = row
	}

	resolved := make(map[string]subagents.ResolvedRole)
	for _, raw := range wanted {
		slug := strings.TrimSpace(raw)
		if slug == "" {
			continue
		}
		locales := bySlug[slug]
		if len(locales) == 0 {
			continue
		}
		var chosen *types.TenantSubagentEntity
		for _, candidate := range []string{locale, "en", "zh"} {
			if row, ok := locales[candidate]; ok {
				chosen = &row
				break
			}
		}
		if chosen == nil {
			continue
		}
		fm, body, err := subagents.ParseRoleMarkdown(chosen.Content)
		if err != nil {
			logger.Warnf(ctx, "[Subagents] installed role %s (locale %s) is unreadable: %v",
				slug, chosen.Locale, err)
			continue
		}
		resolved[slug] = subagents.ResolvedRole{
			Slug:         slug,
			Label:        fm.Name,
			SystemPrompt: body,
			ToolsRaw:     fm.ToolsRaw,
		}
	}
	return resolved
}

// effectiveAllowedTools mirrors the registerTools allowlist semantics
// (agent_service.go): the explicit, user-editable whitelist when set, the
// defaults for legacy agents without one.
func effectiveAllowedTools(config *types.AgentConfig) []string {
	if config == nil {
		return nil
	}
	if len(config.AllowedTools) > 0 {
		return append([]string(nil), config.AllowedTools...)
	}
	return tools.DefaultAllowedTools()
}

// subagentEmitAdapter adapts the main run's event bus to the executor's emit
// func. A run without a bus (nil) skips event emission.
func subagentEmitAdapter(bus *event.EventBus) func(context.Context, event.Event) {
	if bus == nil {
		return nil
	}
	return func(ctx context.Context, e event.Event) {
		_ = bus.Emit(ctx, e)
	}
}
