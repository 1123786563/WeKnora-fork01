package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/modules/airesource/mcp"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func personaLocaleCtx(locale, userID string) context.Context {
	ctx := context.WithValue(context.Background(), types.LanguageContextKey, locale)
	if userID != "" {
		ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	}
	return ctx
}

// TestRenderPersonaSegment covers the persona rendering in isolation: the whole
// segment (including PersonaStyle) is skipped when PersonaMBTI is unset or
// unknown, and otherwise the rendered block is returned alone — joining it in
// front of prompt content is the consumer's job (agent.PrependPersonaSegment /
// the engine's BuildSystemPromptOptions.PersonaSegment).
func TestRenderPersonaSegment(t *testing.T) {
	tests := []struct {
		name        string
		ctx         context.Context
		config      *types.AgentConfig
		wantEmpty   bool
		wantPrefix  string
		wantSubstrs []string
	}{
		{
			name:      "nil config yields empty segment",
			ctx:       personaLocaleCtx("en-US", "user-1"),
			config:    nil,
			wantEmpty: true,
		},
		{
			name:      "empty persona mbti skips whole segment including style",
			ctx:       personaLocaleCtx("en-US", "user-1"),
			config:    &types.AgentConfig{PersonaStyle: "Be terse."},
			wantEmpty: true,
		},
		{
			name:      "unknown code yields empty segment",
			ctx:       personaLocaleCtx("en-US", "user-1"),
			config:    &types.AgentConfig{PersonaMBTI: "ZZZZ", PersonaStyle: "Be terse."},
			wantEmpty: true,
		},
		{
			name:       "english locale renders persona segment",
			ctx:        personaLocaleCtx("en-US", "user-1"),
			config:     &types.AgentConfig{PersonaMBTI: "INTJ", PersonaStyle: "Answer in bullet points."},
			wantPrefix: "# Persona: INTJ — ",
			wantSubstrs: []string{
				"You are " + personaAgentNameFallback + ", an AI assistant working with user-1.",
				"Answer in bullet points.",
			},
		},
		{
			name:       "chinese locale renders zh persona fields",
			ctx:        personaLocaleCtx("zh-CN", "user-1"),
			config:     &types.AgentConfig{PersonaMBTI: "ENFP", PersonaStyle: "用短句回答。"},
			wantPrefix: "# Persona: ENFP — ",
			wantSubstrs: []string{
				"特质：",
				"## 行为风格",
				"用短句回答。",
			},
		},
		{
			name:       "missing user falls back to the default mention",
			ctx:        personaLocaleCtx("zh-CN", ""),
			config:     &types.AgentConfig{PersonaMBTI: "INTJ"},
			wantPrefix: "# Persona: INTJ — ",
			wantSubstrs: []string{
				"与 the user 协作",
			},
		},
		{
			name:       "lowercase code is normalized like the renderer does",
			ctx:        personaLocaleCtx("en-US", "user-1"),
			config:     &types.AgentConfig{PersonaMBTI: " intj "},
			wantPrefix: "# Persona: INTJ — ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderPersonaSegment(tt.ctx, tt.config)
			if tt.wantEmpty {
				require.Empty(t, got)
				return
			}
			require.True(t, strings.HasPrefix(got, tt.wantPrefix),
				"segment %q must start with %q", firstN(got, 60), tt.wantPrefix)
			for _, want := range tt.wantSubstrs {
				require.Contains(t, got, want)
			}
		})
	}
}

// TestPrepareCapabilitiesCarriesPersonaSegment asserts the wiring on the real
// assembly path: the persona segment is carried SEPARATELY from the custom
// prompt template so the builtin engine prepends it in front of whichever
// template applies, while the durable snapshot joins it in front of the system
// content verbatim.
func TestPrepareCapabilitiesCarriesPersonaSegment(t *testing.T) {
	manager := mcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	mcpService := &agentCapabilitiesMCPService{
		service:  &types.MCPService{ID: "calendar", TenantID: 7, Enabled: true},
		metadata: &types.MCPMetadata{ServiceID: "calendar"},
	}
	svc := &agentService{mcpServiceService: mcpService, mcpManager: manager}
	ctx := personaLocaleCtx("en-US", "user-1")
	config := &types.AgentConfig{
		AllowedTools:          []string{tools.ToolThinking},
		UseCustomSystemPrompt: true,
		SystemPrompt:          "You are a research helper.",
		PersonaMBTI:           "INTJ",
		PersonaStyle:          "Answer in bullet points.",
	}
	caps, err := svc.prepareAgentCapabilities(ctx, config, &fakeAgentChatModel{}, nil, nil, "session-1", "message-1")
	require.NoError(t, err)
	// The custom prompt stays a clean template for the builtin engine.
	require.Equal(t, "You are a research helper.", caps.SystemPrompt)
	// The persona segment rides separately, rendered in the request locale.
	require.True(t, strings.HasPrefix(caps.PersonaSegment, "# Persona: INTJ — "),
		"PersonaSegment %q must start with the persona header", firstN(caps.PersonaSegment, 60))
	require.Contains(t, caps.PersonaSegment, "Answer in bullet points.")
	// (d) The durable snapshot joins persona in front of the system content
	// verbatim, in the same layout the builtin engine's prepend produces.
	snapshot := caps.CapabilitySnapshot()
	require.True(t, strings.HasPrefix(snapshot.SystemPrompt, caps.PersonaSegment+agent.PersonaSegmentSeparator))
	require.Contains(t, snapshot.SystemPrompt, "You are a research helper.")

	// A persona-only agent (no custom prompt) keeps an empty template — the
	// builtin engine then applies the default scaffolding behind the segment —
	// while the durable snapshot still carries the persona verbatim.
	personaOnly, err := svc.prepareAgentCapabilities(
		ctx,
		&types.AgentConfig{AllowedTools: []string{tools.ToolThinking}, PersonaMBTI: "INTJ"},
		&fakeAgentChatModel{}, nil, nil, "session-1", "message-1",
	)
	require.NoError(t, err)
	require.Empty(t, personaOnly.SystemPrompt, "persona-only agent must keep the default template slot empty")
	require.True(t, strings.HasPrefix(personaOnly.CapabilitySnapshot().SystemPrompt, "# Persona: INTJ — "))

	// The same assembly without a persona keeps the prompt verbatim.
	bare, err := svc.prepareAgentCapabilities(
		personaLocaleCtx("en-US", "user-1"),
		&types.AgentConfig{AllowedTools: []string{tools.ToolThinking}},
		&fakeAgentChatModel{}, nil, nil, "session-1", "message-1",
	)
	require.NoError(t, err)
	require.Empty(t, bare.PersonaSegment)
	require.Equal(t, "", bare.SystemPrompt)
	require.Empty(t, bare.CapabilitySnapshot().SystemPrompt)
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
