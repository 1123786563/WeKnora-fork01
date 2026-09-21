package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/subagents"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/tools"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// subagentRoleMD renders one minimal role markdown document, the exact shape a
// tenant_subagents Content column stores verbatim.
func subagentRoleMD(name, toolList string) string {
	return "---\nname: " + name + "\ndescription: role for tests\ntools: " + toolList +
		"\n---\n\n# " + name + "\n\n正文内容\n"
}

func seedTenantSubagent(t *testing.T, repo repository.TenantSubagentRepository,
	tenantID uint64, slug, locale, name, toolList string,
) {
	t.Helper()
	require.NoError(t, repo.Upsert(durableRunCtx(), &types.TenantSubagentEntity{
		ID:       "row-" + slug + "-" + locale,
		TenantID: tenantID,
		Slug:     slug,
		Locale:   locale,
		Content:  subagentRoleMD(name, toolList),
		Division: "engineering",
		Source:   types.SubagentSourceBuiltin,
	}))
}

func TestRegisterSubagentDelegateToolConditions(t *testing.T) {
	db := openDurableRunTestDB(t)
	ctx := durableRunCtx()
	repo := repository.NewTenantSubagentRepository(db)

	seedTenantSubagent(t, repo, 1, "code-reviewer", "zh", "代码评审员", "Read, Write, Edit")
	seedTenantSubagent(t, repo, 1, "web-researcher", "zh", "网络研究员", "WebSearch")

	svc := &agentService{db: db}
	model := &fakeAgentChatModel{}
	bus := event.NewEventBus()

	// Delegation off: empty Subagents config never registers the tool.
	offRegistry := tools.NewToolRegistry()
	require.NoError(t, svc.registerSubagentDelegateTool(ctx, offRegistry,
		&types.AgentConfig{}, model, bus, "s1"))
	require.NotContains(t, offRegistry.ListTools(), subagents.ToolSubagentDelegate)

	// Non-empty config with installed overlap registers the tool, and the
	// description lists ONLY the installed specialists.
	onRegistry := tools.NewToolRegistry()
	require.NoError(t, svc.registerSubagentDelegateTool(ctx, onRegistry,
		&types.AgentConfig{Subagents: []string{"code-reviewer", "not-installed"}}, model, bus, "s1"))
	delegate, err := onRegistry.GetTool(subagents.ToolSubagentDelegate)
	require.NoError(t, err)
	require.Contains(t, delegate.Description(), "code-reviewer")
	require.NotContains(t, delegate.Description(), "not-installed",
		"the model must not see slugs that are not installed")

	// Delegating an unconfigured-but-installed slug must fail at lookup time:
	// the tool exposes exactly the configured intersection.
	require.NotContains(t, delegate.Description(), "web-researcher",
		"an installed role outside the agent's Subagents config is not offered")

	// Non-empty config with zero installed overlap stays unregistered.
	noneRegistry := tools.NewToolRegistry()
	require.NoError(t, svc.registerSubagentDelegateTool(ctx, noneRegistry,
		&types.AgentConfig{Subagents: []string{"ghost-role", "other-ghost"}}, model, bus, "s1"))
	require.NotContains(t, noneRegistry.ListTools(), subagents.ToolSubagentDelegate)
}

func TestRegisterSubagentDelegateToolDegradedAssemblies(t *testing.T) {
	db := openDurableRunTestDB(t)
	ctx := durableRunCtx()
	repo := repository.NewTenantSubagentRepository(db)
	seedTenantSubagent(t, repo, 1, "code-reviewer", "zh", "代码评审员", "Read")

	configured := &types.AgentConfig{Subagents: []string{"code-reviewer"}}

	// No tenant on the context: nothing to resolve against.
	noTenantRegistry := tools.NewToolRegistry()
	require.NoError(t, (&agentService{db: db}).registerSubagentDelegateTool(t.Context(),
		noTenantRegistry, configured, &fakeAgentChatModel{}, nil, "s1"))
	require.NotContains(t, noTenantRegistry.ListTools(), subagents.ToolSubagentDelegate)

	// No database handle: the capability is optional, never a run failure.
	noDBRegistry := tools.NewToolRegistry()
	require.NoError(t, (&agentService{}).registerSubagentDelegateTool(ctx,
		noDBRegistry, configured, &fakeAgentChatModel{}, nil, "s1"))
	require.NotContains(t, noDBRegistry.ListTools(), subagents.ToolSubagentDelegate)

	// No chat model: delegation reuses the main run's model, so it cannot
	// register without one.
	noModelRegistry := tools.NewToolRegistry()
	require.NoError(t, (&agentService{db: db}).registerSubagentDelegateTool(ctx,
		noModelRegistry, configured, nil, nil, "s1"))
	require.NotContains(t, noModelRegistry.ListTools(), subagents.ToolSubagentDelegate)

	// A nil registry is a wiring bug, not a panic.
	require.NoError(t, (&agentService{db: db}).registerSubagentDelegateTool(ctx, nil,
		configured, &fakeAgentChatModel{}, nil, "s1"))
}

func TestResolveTenantSubagentRolesLocaleFallback(t *testing.T) {
	rows := []types.TenantSubagentEntity{
		{Slug: "zh-only", Locale: "zh", Content: subagentRoleMD("中文角色", "Read")},
		{Slug: "en-only", Locale: "en", Content: subagentRoleMD("English role", "WebSearch")},
		{Slug: "both", Locale: "en", Content: subagentRoleMD("English both", "Read")},
		{Slug: "both", Locale: "zh", Content: subagentRoleMD("中文双版", "Read")},
		{Slug: "broken", Locale: "zh", Content: "# no frontmatter fence"},
	}

	// zh request: zh wins, en-only falls back to en, broken is skipped.
	zh := resolveTenantSubagentRoles(t.Context(), rows, []string{"zh-only", "en-only", "both", "broken", "ghost"}, "zh")
	require.Len(t, zh, 3)
	require.Equal(t, "中文角色", zh["zh-only"].Label)
	require.Equal(t, "English role", zh["en-only"].Label)
	require.Equal(t, "中文双版", zh["both"].Label)
	require.Equal(t, "\n# 中文角色\n\n正文内容\n", zh["zh-only"].SystemPrompt)
	require.Equal(t, "Read", zh["zh-only"].ToolsRaw)

	// en request: en wins for both; zh-only falls back to zh.
	en := resolveTenantSubagentRoles(t.Context(), rows, []string{"both", "zh-only"}, "en")
	require.Equal(t, "English both", en["both"].Label)
	require.Equal(t, "中文角色", en["zh-only"].Label)
}

// TestRoleMappingTargetsSurviveSharedAgentReadOnlyNarrowing guards the
// read-only shared-agent contract: if a future role mapping ever targeted a
// wiki write tool, a shared agent's specialist could mutate the source
// workspace's Wiki state through delegation, silently escaping
// filterSharedAgentWriteTools. None of the mappable targets may be dropped by
// the read-only filter today.
func TestRoleMappingTargetsSurviveSharedAgentReadOnlyNarrowing(t *testing.T) {
	raw := "WebSearch, WebFetch, Read, Write, Edit, Bash, 搜索, 网页, 阅读, 写作, 编辑, 终端"
	mapped := subagents.MapRoleTools(raw)
	require.NotEmpty(t, mapped)
	narrowed := filterSharedAgentWriteTools(mapped)
	assert.Equal(t, mapped, narrowed,
		"a role-mapped tool is subject to the shared-agent read-only filter and must be re-reviewed")
}

func TestEffectiveAllowedToolsMirrorsRegisterToolsSemantics(t *testing.T) {
	explicit := &types.AgentConfig{AllowedTools: []string{tools.ToolThinking}}
	require.Equal(t, []string{tools.ToolThinking}, effectiveAllowedTools(explicit))

	// A legacy agent without an allowlist falls back to the defaults — the
	// same semantics registerTools applies (agent_service.go registerTools).
	require.Equal(t, tools.DefaultAllowedTools(), effectiveAllowedTools(&types.AgentConfig{}))

	// The copy must not alias the caller's slice.
	in := []string{tools.ToolThinking}
	got := effectiveAllowedTools(&types.AgentConfig{AllowedTools: in})
	in[0] = "mutated"
	require.Equal(t, tools.ToolThinking, got[0])
}

func TestSubagentEmitAdapter(t *testing.T) {
	require.Nil(t, subagentEmitAdapter(nil), "a run without a bus skips event emission")

	bus := event.NewEventBus()
	seen := 0
	bus.On(event.EventAgentToolCall, func(ctx context.Context, e event.Event) error {
		seen++
		return nil
	})
	emit := subagentEmitAdapter(bus)
	require.NotNil(t, emit)
	emit(t.Context(), event.Event{Type: event.EventAgentToolCall})
	require.Equal(t, 1, seen)
}
