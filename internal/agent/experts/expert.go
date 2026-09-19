// Package experts implements the builtin expert-template library: yaml
// manifests under config/experts/<id>/ together with their persona
// documents and bundled skill directories.
//
// The package is pure logic and data: no import-time disk IO — scanning is
// explicit (ScanExperts) or lazy on first use (LoadBuiltinExperts), mirroring
// the builtin_agents.yaml loading model.
package experts

// LocaleText holds per-locale strings keyed by language code. Keys are
// exactly "zh" and "en" for shipped experts.
type LocaleText map[string]string

// QuickPrompt is a one-tap prompt shown on the expert's card; on
// instantiation they map onto QuestionSuggestions.Starters.Items.
type QuickPrompt struct {
	Title       LocaleText `yaml:"title"`
	Description LocaleText `yaml:"description"`
	Prompt      LocaleText `yaml:"prompt"`
	Color       string     `yaml:"color"`
	IconName    string     `yaml:"icon_name"`
}

// ExpertAgentConfig carries the agent-runtime overrides an expert template
// applies when instantiating a CustomAgent. Zero values mean "leave the
// CreateAgent defaults untouched" — the instantiate mapping skips unset
// fields rather than overwriting defaults with empty values.
type ExpertAgentConfig struct {
	AgentMode        string  `yaml:"agent_mode"`
	KBSelectionMode  string  `yaml:"kb_selection_mode"`
	Temperature      float64 `yaml:"temperature"`
	MaxIterations    int     `yaml:"max_iterations"`
	SystemPrompt     string  `yaml:"system_prompt"`
	WebSearchEnabled bool    `yaml:"web_search_enabled"`
	MultiTurnEnabled bool    `yaml:"multi_turn_enabled"`
}

// ExpertManifest is the parsed content of an expert's manifest.yaml.
type ExpertManifest struct {
	ID           string            `yaml:"id"`
	Label        LocaleText        `yaml:"label"`
	Description  LocaleText        `yaml:"description"`
	IconName     string            `yaml:"icon_name"`
	Color        string            `yaml:"color"`
	PersonaMBTI  string            `yaml:"persona_mbti"`
	PromptFiles  []string          `yaml:"prompt_files"`
	QuickPrompts []QuickPrompt     `yaml:"quick_prompts"`
	Skills       []string          `yaml:"skills"`
	AgentConfig  ExpertAgentConfig `yaml:"agent_config"`
}

// Expert is one scanned expert package. The scanner fills PersonaFiles with
// the referenced persona documents and SkillDirs with the absolute paths of
// the bundled skill directories. Values are shared process-wide once cached
// by LoadBuiltinExperts — treat them as read-only.
type Expert struct {
	Manifest     ExpertManifest
	PersonaFiles map[string][]byte // prompt file base name → content
	SkillDirs    map[string]string // skill slug → absolute directory path
}
