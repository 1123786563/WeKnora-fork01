package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/mcp"
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

// TestPrependPersonaSegment covers the persona prepend in isolation: the whole
// segment (including PersonaStyle) is skipped when PersonaMBTI is unset or
// unknown, and otherwise the rendered block lands in front of the system
// prompt in the request locale.
func TestPrependPersonaSegment(t *testing.T) {
	const prompt = "You are a research helper."
	tests := []struct {
		name        string
		ctx         context.Context
		config      *types.AgentConfig
		system      string
		unchanged   bool
		wantPrefix  string
		wantSubstrs []string
	}{
		{
			name:      "nil config leaves prompt untouched",
			ctx:       personaLocaleCtx("en-US", "user-1"),
			config:    nil,
			system:    prompt,
			unchanged: true,
		},
		{
			name:      "empty persona mbti skips whole segment including style",
			ctx:       personaLocaleCtx("en-US", "user-1"),
			config:    &types.AgentConfig{PersonaStyle: "Be terse."},
			system:    prompt,
			unchanged: true,
		},
		{
			name:      "unknown code leaves prompt untouched",
			ctx:       personaLocaleCtx("en-US", "user-1"),
			config:    &types.AgentConfig{PersonaMBTI: "ZZZZ", PersonaStyle: "Be terse."},
			system:    prompt,
			unchanged: true,
		},
		{
			name:       "english locale prepends persona before prompt",
			ctx:        personaLocaleCtx("en-US", "user-1"),
			config:     &types.AgentConfig{PersonaMBTI: "INTJ", PersonaStyle: "Answer in bullet points."},
			system:     prompt,
			wantPrefix: "# Persona: INTJ — ",
			wantSubstrs: []string{
				"You are " + personaAgentNameFallback + ", an AI assistant working with user-1.",
				"Answer in bullet points.",
				"\n---\n\n" + prompt,
			},
		},
		{
			name:       "chinese locale renders zh persona fields",
			ctx:        personaLocaleCtx("zh-CN", "user-1"),
			config:     &types.AgentConfig{PersonaMBTI: "ENFP", PersonaStyle: "用短句回答。"},
			system:     prompt,
			wantPrefix: "# Persona: ENFP — ",
			wantSubstrs: []string{
				"特质：",
				"## 行为风格",
				"用短句回答。",
				"\n---\n\n" + prompt,
			},
		},
		{
			name:       "missing user falls back to the default mention",
			ctx:        personaLocaleCtx("zh-CN", ""),
			config:     &types.AgentConfig{PersonaMBTI: "INTJ"},
			system:     prompt,
			wantPrefix: "# Persona: INTJ — ",
			wantSubstrs: []string{
				"与 the user 协作",
				"\n---\n\n" + prompt,
			},
		},
		{
			name:       "lowercase code is normalized like the renderer does",
			ctx:        personaLocaleCtx("en-US", "user-1"),
			config:     &types.AgentConfig{PersonaMBTI: " intj "},
			system:     prompt,
			wantPrefix: "# Persona: INTJ — ",
			wantSubstrs: []string{
				"\n---\n\n" + prompt,
			},
		},
		{
			name:       "empty system prompt yields the segment alone",
			ctx:        personaLocaleCtx("en-US", "user-1"),
			config:     &types.AgentConfig{PersonaMBTI: "INTJ"},
			system:     "",
			wantPrefix: "# Persona: INTJ — ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := prependPersonaSegment(tt.ctx, tt.config, tt.system)
			if tt.unchanged {
				require.Equal(t, tt.system, got)
				return
			}
			require.True(t, strings.HasPrefix(got, tt.wantPrefix),
				"prompt %q must start with %q", firstN(got, 60), tt.wantPrefix)
			for _, want := range tt.wantSubstrs {
				require.Contains(t, got, want)
			}
			if tt.system == "" {
				require.NotContains(t, got, "\n---\n\n",
					"segment alone must not carry the join separator")
			}
		})
	}
}

// TestPrepareCapabilitiesPrependsPersona asserts the wiring end to end on the
// real assembly path: the returned capabilities carry the persona segment in
// front of the resolved system prompt, which is what both engines (builtin
// ReAct template consumption and trpc verbatim system message) read.
func TestPrepareCapabilitiesPrependsPersona(t *testing.T) {
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
	require.True(t, strings.HasPrefix(caps.SystemPrompt, "# Persona: INTJ — "),
		"SystemPrompt %q must start with the persona segment", firstN(caps.SystemPrompt, 60))
	require.Contains(t, caps.SystemPrompt, "\n---\n\nYou are a research helper.")
	require.Contains(t, caps.SystemPrompt, "Answer in bullet points.")

	// The same assembly without a persona keeps the prompt verbatim.
	bare, err := svc.prepareAgentCapabilities(
		personaLocaleCtx("en-US", "user-1"),
		&types.AgentConfig{AllowedTools: []string{tools.ToolThinking}},
		&fakeAgentChatModel{}, nil, nil, "session-1", "message-1",
	)
	require.NoError(t, err)
	require.False(t, strings.HasPrefix(bare.SystemPrompt, "# Persona:"),
		"prompt without persona must not gain a segment: %q", firstN(bare.SystemPrompt, 60))
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
