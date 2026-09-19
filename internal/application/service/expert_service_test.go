package service

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/experts"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// fixtureExpert mirrors the shape of the shipped pilot experts (stock-assistant
// in particular): zh/en locale text, two persona files, six quick prompts, one
// bundled skill, and agent_config overrides.
func fixtureExpert() *experts.Expert {
	quickPrompts := make([]experts.QuickPrompt, 0, 6)
	for i := 0; i < 6; i++ {
		quickPrompts = append(quickPrompts, experts.QuickPrompt{
			Title:       experts.LocaleText{"zh": "标题", "en": "Title"},
			Description: experts.LocaleText{"zh": "描述", "en": "Description"},
			Prompt: experts.LocaleText{
				"zh": "中文快捷指令 " + string(rune('A'+i)),
				"en": "english quick prompt " + string(rune('A'+i)),
			},
			Color:    "#e8f4ff",
			IconName: "line-chart",
		})
	}
	return &experts.Expert{
		Manifest: experts.ExpertManifest{
			ID:           "stock-assistant",
			Label:        experts.LocaleText{"zh": "老钱 · 证券观察员", "en": "Lao Qian · Market Observer"},
			Description:  experts.LocaleText{"zh": "市场观察者", "en": "Market observer"},
			IconName:     "candlestick-chart",
			Color:        "#e74c3c",
			PersonaMBTI:  "ISTP",
			PromptFiles:  []string{"SOUL.md", "IDENTITY.md"},
			QuickPrompts: quickPrompts,
			Skills:       []string{"stock-info"},
			AgentConfig: experts.ExpertAgentConfig{
				AgentMode:        types.AgentModeSmartReasoning,
				WebSearchEnabled: true,
			},
		},
		PersonaFiles: map[string][]byte{
			"SOUL.md":     []byte("SOUL persona content"),
			"IDENTITY.md": []byte("IDENTITY persona content"),
		},
		SkillDirs: map[string]string{"stock-info": "/experts/stock-assistant/skills/stock-info"},
	}
}

func zhPrompt(i int) string { return "中文快捷指令 " + string(rune('A'+i)) }
func enPrompt(i int) string { return "english quick prompt " + string(rune('A'+i)) }

func TestBuildAgentFromExpert(t *testing.T) {
	tests := []struct {
		name string
		// mutate applies optional per-case tweaks to the fixture expert.
		mutate       func(*experts.Expert)
		locale       string
		nameOverride string
		verify       func(t *testing.T, agent *types.CustomAgent)
	}{
		{
			name:   "zh-CN locale full mapping",
			locale: "zh-CN",
			verify: func(t *testing.T, agent *types.CustomAgent) {
				if agent.Name != "老钱 · 证券观察员" {
					t.Errorf("Name = %q, want zh label", agent.Name)
				}
				if agent.Description != "市场观察者" {
					t.Errorf("Description = %q, want zh description", agent.Description)
				}
				if agent.Avatar != "candlestick-chart" {
					t.Errorf("Avatar = %q, want icon_name", agent.Avatar)
				}
				if agent.ID != "" {
					t.Errorf("builder must not invent an ID, got %q", agent.ID)
				}
				if agent.IsBuiltin {
					t.Error("instantiated agents are tenant agents, not builtin")
				}
				wantPrompt := "SOUL persona content\n\n---\n\nIDENTITY persona content"
				if agent.Config.SystemPrompt != wantPrompt {
					t.Errorf("SystemPrompt = %q, want %q", agent.Config.SystemPrompt, wantPrompt)
				}
				qs := agent.Config.QuestionSuggestions
				if qs == nil {
					t.Fatal("QuestionSuggestions not set")
				}
				if !qs.Starters.Enabled || qs.Starters.Mode != types.SuggestionModeCurated {
					t.Errorf("Starters = %+v, want enabled curated", qs.Starters)
				}
				wantItems := []string{zhPrompt(0), zhPrompt(1), zhPrompt(2), zhPrompt(3), zhPrompt(4), zhPrompt(5)}
				if !reflect.DeepEqual(qs.Starters.Items, wantItems) {
					t.Errorf("Starters.Items = %v, want %v", qs.Starters.Items, wantItems)
				}
				if qs.Starters.Count != len(wantItems) {
					t.Errorf("Starters.Count = %d, want %d", qs.Starters.Count, len(wantItems))
				}
				// CreateAgent runs QuestionSuggestions.EnsureDefaults()
				// before Validate; the builder forms only Starters, so
				// prove the composition: defaults fill FollowUps, the
				// built starters survive untouched, and the merged config
				// validates.
				composed := *qs
				composed.EnsureDefaults()
				if err := composed.Validate(); err != nil {
					t.Errorf("built suggestions must pass Validate after EnsureDefaults: %v", err)
				}
				if composed.Starters.Mode != types.SuggestionModeCurated ||
					composed.Starters.Count != len(wantItems) ||
					!reflect.DeepEqual(composed.Starters.Items, wantItems) {
					t.Errorf("EnsureDefaults must not override built starters: %+v", composed.Starters)
				}
				src := agent.Config.ExpertSource
				if src == nil || src.ExpertID != "stock-assistant" || src.Source != "builtin" {
					t.Errorf("ExpertSource = %+v, want {stock-assistant builtin}", src)
				}
				if agent.Config.AgentMode != types.AgentModeSmartReasoning {
					t.Errorf("AgentMode = %q, want override %q", agent.Config.AgentMode, types.AgentModeSmartReasoning)
				}
				if !agent.Config.WebSearchEnabled {
					t.Error("WebSearchEnabled override not applied")
				}
				if agent.Config.PersonaMBTI != "ISTP" {
					t.Errorf("PersonaMBTI = %q, want passthrough", agent.Config.PersonaMBTI)
				}
				if agent.Config.SkillsSelectionMode != "selected" {
					t.Errorf("SkillsSelectionMode = %q, want selected", agent.Config.SkillsSelectionMode)
				}
				if !reflect.DeepEqual(agent.Config.SelectedSkills, []string{"stock-info"}) {
					t.Errorf("SelectedSkills = %v, want [stock-info]", agent.Config.SelectedSkills)
				}
			},
		},
		{
			name:   "en-US locale resolves english text",
			locale: "en-US",
			verify: func(t *testing.T, agent *types.CustomAgent) {
				if agent.Name != "Lao Qian · Market Observer" {
					t.Errorf("Name = %q, want en label", agent.Name)
				}
				if agent.Description != "Market observer" {
					t.Errorf("Description = %q, want en description", agent.Description)
				}
				wantItems := []string{enPrompt(0), enPrompt(1), enPrompt(2), enPrompt(3), enPrompt(4), enPrompt(5)}
				if !reflect.DeepEqual(agent.Config.QuestionSuggestions.Starters.Items, wantItems) {
					t.Errorf("Starters.Items = %v, want %v", agent.Config.QuestionSuggestions.Starters.Items, wantItems)
				}
			},
		},
		{
			name:         "name override wins over label",
			locale:       "zh-CN",
			nameOverride: "我的老钱",
			verify: func(t *testing.T, agent *types.CustomAgent) {
				if agent.Name != "我的老钱" {
					t.Errorf("Name = %q, want override", agent.Name)
				}
			},
		},
		{
			name:   "agent system prompt appended after persona files",
			locale: "zh-CN",
			mutate: func(e *experts.Expert) {
				e.Manifest.AgentConfig.SystemPrompt = "EXTRA RULES"
			},
			verify: func(t *testing.T, agent *types.CustomAgent) {
				want := "SOUL persona content\n\n---\n\nIDENTITY persona content\n\n---\n\nEXTRA RULES"
				if agent.Config.SystemPrompt != want {
					t.Errorf("SystemPrompt = %q, want %q", agent.Config.SystemPrompt, want)
				}
			},
		},
		{
			name:   "zero-value agent config fields are skipped",
			locale: "zh-CN",
			mutate: func(e *experts.Expert) {
				e.Manifest.AgentConfig = experts.ExpertAgentConfig{}
			},
			verify: func(t *testing.T, agent *types.CustomAgent) {
				if agent.Config.AgentMode != "" {
					t.Errorf("AgentMode = %q, want empty (CreateAgent defaults it)", agent.Config.AgentMode)
				}
				if agent.Config.Temperature != 0 {
					t.Errorf("Temperature = %v, want 0", agent.Config.Temperature)
				}
				if agent.Config.MaxIterations != 0 {
					t.Errorf("MaxIterations = %d, want 0", agent.Config.MaxIterations)
				}
				if agent.Config.WebSearchEnabled || agent.Config.MultiTurnEnabled {
					t.Error("false bools must not be applied")
				}
				if agent.Config.KBSelectionMode != "" {
					t.Errorf("KBSelectionMode = %q, want empty", agent.Config.KBSelectionMode)
				}
				if agent.Config.SystemPrompt != "SOUL persona content\n\n---\n\nIDENTITY persona content" {
					t.Errorf("SystemPrompt = %q, want persona files only", agent.Config.SystemPrompt)
				}
			},
		},
		{
			name:   "non-zero overrides all applied",
			locale: "zh-CN",
			mutate: func(e *experts.Expert) {
				e.Manifest.AgentConfig = experts.ExpertAgentConfig{
					AgentMode:        types.AgentModeSmartReasoning,
					KBSelectionMode:  "none",
					Temperature:      0.2,
					MaxIterations:    15,
					WebSearchEnabled: true,
					MultiTurnEnabled: true,
				}
			},
			verify: func(t *testing.T, agent *types.CustomAgent) {
				cfg := agent.Config
				if cfg.KBSelectionMode != "none" || cfg.Temperature != 0.2 ||
					cfg.MaxIterations != 15 || !cfg.WebSearchEnabled || !cfg.MultiTurnEnabled {
					t.Errorf("overrides not applied: %+v", cfg)
				}
			},
		},
		{
			name:   "no-skill expert leaves skills selection untouched",
			locale: "zh-CN",
			mutate: func(e *experts.Expert) {
				e.Manifest.Skills = nil
				e.SkillDirs = nil
			},
			verify: func(t *testing.T, agent *types.CustomAgent) {
				if agent.Config.SkillsSelectionMode != "" {
					t.Errorf("SkillsSelectionMode = %q, want empty", agent.Config.SkillsSelectionMode)
				}
				if agent.Config.SelectedSkills != nil {
					t.Errorf("SelectedSkills = %v, want nil", agent.Config.SelectedSkills)
				}
			},
		},
		{
			name:   "expert without quick prompts leaves suggestions to defaults",
			locale: "zh-CN",
			mutate: func(e *experts.Expert) {
				e.Manifest.QuickPrompts = nil
			},
			verify: func(t *testing.T, agent *types.CustomAgent) {
				if agent.Config.QuestionSuggestions != nil {
					// Zero prompts cannot form a valid curated set (Count must
					// be >= 1); the builder must leave the field nil so
					// CreateAgent's EnsureDefaults supplies the standard one.
					t.Errorf("QuestionSuggestions = %+v, want nil", agent.Config.QuestionSuggestions)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := fixtureExpert()
			if tt.mutate != nil {
				tt.mutate(e)
			}
			agent := buildAgentFromExpert(e, tt.locale, tt.nameOverride)
			if agent == nil {
				t.Fatal("buildAgentFromExpert returned nil")
			}
			tt.verify(t, agent)
		})
	}
}

// fixtureExpertWithNPrompts returns the stock-assistant fixture retooled to
// carry n distinct quick prompts, for exercising the starters cap.
func fixtureExpertWithNPrompts(n int) *experts.Expert {
	e := fixtureExpert()
	quickPrompts := make([]experts.QuickPrompt, 0, n)
	for i := 0; i < n; i++ {
		quickPrompts = append(quickPrompts, experts.QuickPrompt{
			Title:       experts.LocaleText{"zh": "标题", "en": "Title"},
			Description: experts.LocaleText{"zh": "描述", "en": "Description"},
			Prompt: experts.LocaleText{
				"zh": "中文快捷指令 " + string(rune('A'+i)),
				"en": "english quick prompt " + string(rune('A'+i)),
			},
			Color:    "#e8f4ff",
			IconName: "line-chart",
		})
	}
	e.Manifest.QuickPrompts = quickPrompts
	return e
}

// TestBuildAgentFromExpertCapsStartersAtValidationLimit is the regression
// test for the general-assistant instantiate failure: its manifest ships 12
// quick prompts while CreateAgent's Validate caps curated starters at 8.
// The builder must keep only the first 8 prompts, in manifest order, so the
// built suggestions pass the same EnsureDefaults+Validate gate CreateAgent
// runs.
func TestBuildAgentFromExpertCapsStartersAtValidationLimit(t *testing.T) {
	e := fixtureExpertWithNPrompts(12)

	agent := buildAgentFromExpert(e, "zh-CN", "")

	qs := agent.Config.QuestionSuggestions
	if qs == nil {
		t.Fatal("QuestionSuggestions not set")
	}
	if len(qs.Starters.Items) != 8 {
		t.Errorf("Starters.Items has %d entries, want capped at 8", len(qs.Starters.Items))
	}
	if qs.Starters.Count != 8 {
		t.Errorf("Starters.Count = %d, want 8", qs.Starters.Count)
	}
	wantItems := []string{zhPrompt(0), zhPrompt(1), zhPrompt(2), zhPrompt(3), zhPrompt(4), zhPrompt(5), zhPrompt(6), zhPrompt(7)}
	if !reflect.DeepEqual(qs.Starters.Items, wantItems) {
		t.Errorf("Starters.Items = %v, want first 8 prompts in manifest order %v", qs.Starters.Items, wantItems)
	}
	// The built agent must clear the real CreateAgent gate, which the 12-item
	// version failed with "starter suggestion count must be between 1 and 8".
	agent.EnsureDefaults()
	if err := agent.Config.QuestionSuggestions.Validate(); err != nil {
		t.Errorf("built suggestions must pass the CreateAgent validation gate: %v", err)
	}
}

// builtinExpertsRoot is the shipped expert library at the repo root, reached
// from this package directory (internal/application/service).
func builtinExpertsRoot(t *testing.T) []*experts.Expert {
	t.Helper()
	scanned, err := experts.ScanExperts(filepath.Join("..", "..", "..", "config", "experts"))
	if err != nil {
		t.Fatalf("ScanExperts(config/experts): %v", err)
	}
	return scanned
}

// TestInstantiateGeneralAssistantSucceeds pins the shipped general-assistant
// manifest (12 quick prompts) end-to-end: Instantiate must produce an agent
// that clears CreateAgent's EnsureDefaults+Validate gate, with the first 8
// prompts in manifest order.
func TestInstantiateGeneralAssistantSucceeds(t *testing.T) {
	var general *experts.Expert
	for _, e := range builtinExpertsRoot(t) {
		if e.Manifest.ID == "general-assistant" {
			general = e
			break
		}
	}
	if general == nil {
		t.Fatal("general-assistant expert not found under config/experts")
	}
	if len(general.Manifest.QuickPrompts) <= 8 {
		t.Fatalf("fixture drifted: general-assistant now ships %d quick prompts, want >8 for this regression", len(general.Manifest.QuickPrompts))
	}

	agents := &expertAgentsFake{validate: true}
	svc := NewExpertService(func() []*experts.Expert { return []*experts.Expert{general} }, agents, nil)

	res, err := svc.Instantiate(expertCtx("zh-CN"), 7, "general-assistant", interfaces.InstantiateRequest{})
	if err != nil {
		t.Fatalf("Instantiate(general-assistant) failed (was the 500): %v", err)
	}

	qs := res.Agent.Config.QuestionSuggestions
	if qs == nil {
		t.Fatal("created agent has no QuestionSuggestions")
	}
	if len(qs.Starters.Items) != 8 || qs.Starters.Count != 8 {
		t.Errorf("Starters items=%d count=%d, want 8/8", len(qs.Starters.Items), qs.Starters.Count)
	}
	wantFirst := resolveExpertLocaleText(general.Manifest.QuickPrompts[0].Prompt, "zh-CN")
	if qs.Starters.Items[0] != wantFirst {
		t.Errorf("Starters.Items[0] = %q, want manifest's first prompt %q", qs.Starters.Items[0], wantFirst)
	}
}

// TestBuildAgentFromExpertDoesNotMutateSharedExpert guards the shared-cache
// contract: LoadBuiltinExperts returns process-wide objects, so the builder
// must never alias or write into the manifest's slices.
func TestBuildAgentFromExpertDoesNotMutateSharedExpert(t *testing.T) {
	e := fixtureExpert()
	first := buildAgentFromExpert(e, "zh-CN", "")
	// Vandalize the first result through every aliased-looking field.
	first.Config.SelectedSkills[0] = "tampered"
	first.Config.QuestionSuggestions.Starters.Items[0] = "tampered"
	first.Config.ExpertSource.ExpertID = "tampered"

	second := buildAgentFromExpert(e, "zh-CN", "")
	if got := second.Config.SelectedSkills[0]; got != "stock-info" {
		t.Errorf("SelectedSkills aliased manifest slice, got %q", got)
	}
	if got := second.Config.QuestionSuggestions.Starters.Items[0]; got != zhPrompt(0) {
		t.Errorf("Starters.Items aliased, got %q", got)
	}
	if got := second.Config.ExpertSource.ExpertID; got != "stock-assistant" {
		t.Errorf("ExpertSource aliased between builds, got %q", got)
	}
	if !reflect.DeepEqual(e.Manifest.Skills, []string{"stock-info"}) {
		t.Errorf("manifest Skills mutated: %v", e.Manifest.Skills)
	}
}

// expertAgentsFake records the CreateAgent call and mimics the parts of
// customAgentService.CreateAgent the instantiate flow depends on (ID
// generation); it deliberately skips EnsureDefaults so tests observe exactly
// what the builder produced. Set validate=true to also replay the real
// CreateAgent's EnsureDefaults+Validate gate (the suggestion-limit check).
type expertAgentsFake struct {
	created  []*types.CustomAgent
	err      error
	validate bool
}

var _ interfaces.CustomAgentService = (*expertAgentsFake)(nil)

func (f *expertAgentsFake) CreateAgent(_ context.Context, agent *types.CustomAgent) (*types.CustomAgent, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.validate {
		agent.EnsureDefaults()
		if err := agent.Config.QuestionSuggestions.Validate(); err != nil {
			return nil, err
		}
	}
	if agent.ID == "" {
		agent.ID = "agent-1"
	}
	f.created = append(f.created, agent)
	return agent, nil
}

func (f *expertAgentsFake) GetAgentByID(context.Context, string) (*types.CustomAgent, error) {
	return nil, ErrAgentNotFound
}
func (f *expertAgentsFake) GetAgentByIDAndTenant(context.Context, string, uint64) (*types.CustomAgent, error) {
	return nil, ErrAgentNotFound
}
func (f *expertAgentsFake) ListAgents(context.Context) ([]*types.CustomAgent, error) { return nil, nil }
func (f *expertAgentsFake) UpdateAgent(_ context.Context, agent *types.CustomAgent) (*types.CustomAgent, error) {
	return agent, nil
}
func (f *expertAgentsFake) DeleteAgent(context.Context, string) error { return nil }
func (f *expertAgentsFake) CopyAgent(context.Context, string) (*types.CustomAgent, error) {
	return nil, nil
}
func (f *expertAgentsFake) GetSuggestedQuestions(
	context.Context, string, []string, []string, []types.TagScope, int,
) ([]types.SuggestedQuestion, error) {
	return nil, nil
}
func (f *expertAgentsFake) GetKnowledgeSuggestedQuestions(
	context.Context, string, []string, []string, []types.TagScope, int,
) ([]types.SuggestedQuestion, error) {
	return nil, nil
}

func expertCtx(locale string) context.Context {
	return context.WithValue(context.Background(), types.LanguageContextKey, locale)
}

func TestInstantiateExpert(t *testing.T) {
	t.Run("happy path creates agent with provenance", func(t *testing.T) {
		e := fixtureExpert()
		agents := &expertAgentsFake{}
		svc := NewExpertService(func() []*experts.Expert { return []*experts.Expert{e} }, agents, nil)

		res, err := svc.Instantiate(expertCtx("zh-CN"), 7, "stock-assistant", interfaces.InstantiateRequest{})
		if err != nil {
			t.Fatal(err)
		}
		if len(agents.created) != 1 {
			t.Fatalf("CreateAgent called %d times, want 1", len(agents.created))
		}
		if res.Agent == nil || res.Agent.ID != "agent-1" {
			t.Fatalf("result agent = %+v, want created agent back", res.Agent)
		}
		sent := agents.created[0]
		if sent.Name != "老钱 · 证券观察员" {
			t.Errorf("created name = %q, want locale label", sent.Name)
		}
		if sent.Config.ExpertSource == nil || sent.Config.ExpertSource.ExpertID != "stock-assistant" {
			t.Errorf("created ExpertSource = %+v, want stock-assistant", sent.Config.ExpertSource)
		}
		if len(res.PendingSkills) != 0 || len(res.SkillInstallIDs) != 0 {
			t.Errorf("noop resolver must report no installs/pending, got %+v", res)
		}
	})

	t.Run("unknown expert", func(t *testing.T) {
		svc := NewExpertService(func() []*experts.Expert { return nil }, &expertAgentsFake{}, nil)
		if _, err := svc.Instantiate(expertCtx("zh-CN"), 7, "missing", interfaces.InstantiateRequest{}); !errors.Is(err, ErrExpertNotFound) {
			t.Fatalf("err = %v, want ErrExpertNotFound", err)
		}
	})

	t.Run("request name and sandbox honored", func(t *testing.T) {
		e := fixtureExpert()
		agents := &expertAgentsFake{}
		svc := NewExpertService(func() []*experts.Expert { return []*experts.Expert{e} }, agents, nil)

		res, err := svc.Instantiate(expertCtx("zh-CN"), 7, "stock-assistant", interfaces.InstantiateRequest{
			AgentName:       "自定义名字",
			SandboxConfigID: "sandbox-9",
		})
		if err != nil {
			t.Fatal(err)
		}
		if res.Agent.Name != "自定义名字" {
			t.Errorf("Name = %q, want request name", res.Agent.Name)
		}
		if res.Agent.Config.SandboxConfigID != "sandbox-9" {
			t.Errorf("SandboxConfigID = %q, want sandbox-9", res.Agent.Config.SandboxConfigID)
		}
	})

	t.Run("create agent error propagates", func(t *testing.T) {
		e := fixtureExpert()
		boom := errors.New("boom")
		svc := NewExpertService(
			func() []*experts.Expert { return []*experts.Expert{e} },
			&expertAgentsFake{err: boom},
			nil,
		)
		_, err := svc.Instantiate(expertCtx("zh-CN"), 7, "stock-assistant", interfaces.InstantiateRequest{})
		if err == nil || !strings.Contains(err.Error(), "stock-assistant") || !errors.Is(err, boom) {
			t.Fatalf("err = %v, want wrapped boom mentioning expert id", err)
		}
	})
}

func TestExpertServiceCatalog(t *testing.T) {
	e := fixtureExpert()
	svc := NewExpertService(func() []*experts.Expert { return []*experts.Expert{e} }, &expertAgentsFake{}, nil)

	listed, err := svc.ListExperts(context.Background())
	if err != nil || len(listed) != 1 || listed[0] != e {
		t.Fatalf("ListExperts = %v, %v", listed, err)
	}
	got, err := svc.GetExpert(context.Background(), "stock-assistant")
	if err != nil || got != e {
		t.Fatalf("GetExpert = %v, %v", got, err)
	}
	if _, err := svc.GetExpert(context.Background(), "nope"); !errors.Is(err, ErrExpertNotFound) {
		t.Fatalf("GetExpert(unknown) err = %v, want ErrExpertNotFound", err)
	}
}
