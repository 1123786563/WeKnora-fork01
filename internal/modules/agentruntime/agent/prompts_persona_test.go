package agent

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/persona"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// personaPromptTestConfig loads the shipped default templates so tests pin the
// real scaffolding (retrieval norms, citation-ID rules, language instructions)
// rather than a stub.
func personaPromptTestConfig(t *testing.T) *config.Config {
	t.Helper()
	data, err := os.ReadFile("../../../../config/prompt_templates/agent_system_prompt.yaml")
	require.NoError(t, err)
	var file struct {
		Templates []config.PromptTemplate `yaml:"templates"`
	}
	require.NoError(t, yaml.Unmarshal(data, &file))
	return &config.Config{PromptTemplates: &config.PromptTemplatesConfig{AgentSystemPrompt: file.Templates}}
}

const (
	// personaPureMarker opens the default pure template.
	personaPureMarker = "Complete the user's request using the capabilities available in this turn"
	// personaRAGMarker opens the default RAG template.
	personaRAGMarker = "Answer questions about the user's knowledge bases using verified evidence"
	// personaCitationMarker pins the citation-ID rules only the default RAG
	// scaffolding carries.
	personaCitationMarker = "FAQ references use their cN chunk ID with faq_id/faq_ids"
	// personaLanguageMarker pins the response-language instruction.
	personaLanguageMarker = "User Language:"
)

func testPersonaSegment() string {
	return persona.RenderPersona("INTJ", "en-US", persona.RenderInput{
		AgentName:   "the assistant",
		UserDisplay: "user-1",
		Custom:      "Answer in bullet points.",
	})
}

// TestPersonaSegmentPrependsToResolvedTemplate pins the persona contract on
// the builtin engine's prompt assembly: the segment rides in front of
// WHICHEVER template applies — default scaffolding or custom template — and
// never replaces it. It also pins that the segment is joined after placeholder
// resolution, so its literal text is never expanded as template content.
func TestPersonaSegmentPrependsToResolvedTemplate(t *testing.T) {
	cfg := personaPromptTestConfig(t)
	segment := testPersonaSegment()
	kbs := []*KnowledgeBaseInfo{{ID: "kb-1", Name: "KB", Capabilities: []string{"chunks"}}}

	tests := []struct {
		name            string
		segment         string
		custom          string
		kbs             []*KnowledgeBaseInfo
		wantPrefix      string
		wantContains    []string
		wantNotContains []string
	}{
		{
			// (b) persona + NO custom prompt keeps the default pure
			// scaffolding behind the persona block.
			name:       "persona without custom prompt keeps pure scaffolding",
			segment:    segment,
			custom:     "",
			kbs:        nil,
			wantPrefix: "# Persona: INTJ — ",
			wantContains: []string{
				personaPureMarker,
				personaLanguageMarker,
				"Answer in bullet points.",
			},
		},
		{
			// (b, RAG flavor) persona + NO custom prompt keeps retrieval and
			// citation-ID rules behind the persona block.
			name:       "persona without custom prompt keeps rag scaffolding",
			segment:    segment,
			custom:     "",
			kbs:        kbs,
			wantPrefix: "# Persona: INTJ — ",
			wantContains: []string{
				personaRAGMarker,
				personaCitationMarker,
			},
		},
		{
			// (a) persona + custom prompt keeps today's layout: the segment in
			// front of the custom template content.
			name:       "persona before custom template content",
			segment:    segment,
			custom:     "You are a research helper.",
			kbs:        nil,
			wantPrefix: "# Persona: INTJ — ",
			wantContains: []string{
				"You are a research helper.",
				"<steering_guidance>",
			},
			wantNotContains: []string{personaPureMarker, personaRAGMarker},
		},
		{
			// (c) no persona + custom prompt is unchanged.
			name:       "no persona keeps custom template first",
			segment:    "",
			custom:     "You are a research helper.",
			kbs:        nil,
			wantPrefix: "You are a research helper.",
			wantNotContains: []string{
				"# Persona:",
				personaPureMarker,
				personaRAGMarker,
			},
		},
		{
			// (c) no persona + no custom prompt keeps the default scaffolding.
			name:       "no persona keeps default scaffolding",
			segment:    "",
			custom:     "",
			kbs:        nil,
			wantPrefix: "You are WeKnora",
			wantContains: []string{
				personaPureMarker,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := BuildSystemPromptWithOptions(
				tt.kbs, false,
				&BuildSystemPromptOptions{Config: cfg, PersonaSegment: tt.segment, Language: "English"},
				tt.custom,
			)
			require.True(t, strings.HasPrefix(prompt, tt.wantPrefix),
				"prompt must start with %q, got %q", tt.wantPrefix, firstRunes(prompt, 80))
			for _, want := range tt.wantContains {
				require.Contains(t, prompt, want)
			}
			for _, banned := range tt.wantNotContains {
				require.NotContains(t, prompt, banned)
			}
			if tt.segment != "" && tt.custom != "" {
				require.True(t, strings.HasPrefix(prompt, tt.segment+PersonaSegmentSeparator),
					"persona must be joined in front of the custom template with the shared separator")
			}
		})
	}

	// The segment joins AFTER placeholder resolution: literal {{...}} text in
	// the segment survives verbatim instead of being expanded.
	t.Run("persona segment is not placeholder resolved", func(t *testing.T) {
		prompt := BuildSystemPromptWithOptions(
			nil, false,
			&BuildSystemPromptOptions{
				Config:         cfg,
				PersonaSegment: "Tone marker {{language}} {{knowledge_bases}}",
				Language:       "English",
			},
			"",
		)
		require.Contains(t, prompt, "Tone marker {{language}} {{knowledge_bases}}")
		require.Contains(t, prompt, "User Language: English")
	})
}

// TestEngineBuildSystemPromptPrependsPersona pins the engine seam: the persona
// segment set on the engine lands in front of the resolved default
// scaffolding, so a persona-only agent (no custom prompt template) keeps its
// retrieval, citation and language rules.
func TestEngineBuildSystemPromptPrependsPersona(t *testing.T) {
	cfg := personaPromptTestConfig(t)
	kbs := []*KnowledgeBaseInfo{{ID: "kb-1", Name: "KB", Capabilities: []string{"chunks"}}}

	engine := NewAgentEngine(
		&types.AgentConfig{MaxIterations: 10},
		nil,
		nil,
		event.NewEventBus(),
		kbs,
		nil,
		"test-session",
		"", // no custom template: default scaffolding applies
	)
	require.NotNil(t, engine)
	engine.SetAppConfig(cfg)
	engine.SetPersonaSegment(testPersonaSegment())

	prompt := engine.buildSystemPrompt(context.Background())
	require.True(t, strings.HasPrefix(prompt, "# Persona: INTJ — "),
		"engine prompt must start with the persona segment, got %q", firstRunes(prompt, 80))
	require.Contains(t, prompt, personaRAGMarker)
	require.Contains(t, prompt, personaCitationMarker)
	require.Contains(t, prompt, personaLanguageMarker)
	require.Less(t, strings.Index(prompt, "# Persona:"), strings.Index(prompt, personaRAGMarker),
		"persona must precede the default scaffolding")
}

// TestPrependPersonaSegmentJoin pins the shared join helper both consumption
// paths rely on: identical layout to the builtin engine's prepend, and no
// separator when the system content is empty.
func TestPrependPersonaSegmentJoin(t *testing.T) {
	require.Equal(t, "prompt", PrependPersonaSegment("", "prompt"))
	require.Equal(t, "segment", PrependPersonaSegment("segment", ""))
	require.Equal(t, "", PrependPersonaSegment("", ""))
	require.Equal(t, "segment"+PersonaSegmentSeparator+"prompt", PrependPersonaSegment("segment", "prompt"))
	require.Equal(t, "\n---\n\n", PersonaSegmentSeparator)
}

func firstRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
