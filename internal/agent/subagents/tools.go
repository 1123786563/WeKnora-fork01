package subagents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/airesource/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"gopkg.in/yaml.v3"
)

// ToolSubagentDelegate is the model-facing name of the sub-agent delegation
// tool.
const ToolSubagentDelegate = "subagent_delegate"

// subagentNoToolsNote heads the tool result when the specialist ran without
// any tool: the delegating model must see why the specialist could not look
// anything up. Running toolless is first-class (most roles declare no tools),
// so this is a note, never an error.
const subagentNoToolsNote = "[no tools available]"

// ErrSubagentNotInstalled is returned by a SubagentDelegateToolConfig Lookup
// when the requested slug is not one of this run's installed specialists. The
// tool renders it as the model-facing "subagent not installed: <slug>".
var ErrSubagentNotInstalled = errors.New("subagent not installed")

// roleToolNameMap maps the tool names role files declare — Octop's English
// coding-tool names and their Chinese equivalents — onto the WeKnora tool
// names the engine registers. Every target is a tools.Tool* constant, so a
// renamed or removed engine tool breaks compilation here instead of silently
// granting nothing; TestRoleToolNameMapTargetsAreRealTools locks the mapping
// against the registrable set. Library survey (2026-09): the only raw values
// that ship are "WebFetch, WebSearch, Read, Write, Edit",
// "... , Bash", "Read, Write, Edit" and "阅读、写作、编辑".
var roleToolNameMap = map[string]string{
	// English names as they appear in role frontmatter.
	"WebSearch": tools.ToolWebSearch,
	"WebFetch":  tools.ToolWebFetch,
	"Read":      tools.ToolReadFile,
	"Write":     tools.ToolWriteSandboxFile,
	"Edit":      tools.ToolEditSandboxFile,
	"Bash":      tools.ToolShellExec,
	// Chinese equivalents (、" lists).
	"搜索": tools.ToolWebSearch,
	"网页": tools.ToolWebFetch,
	"阅读": tools.ToolReadFile,
	"写作": tools.ToolWriteSandboxFile,
	"编辑": tools.ToolEditSandboxFile,
	"终端": tools.ToolShellExec,
}

// MapRoleTools maps a role's declared tool names (the raw frontmatter value,
// ASCII-comma and CJK 、 lists alike) onto WeKnora tool names. ParseRoleTools
// splits and dedupes the source names; names with no mapping are dropped with
// a warning — a role cannot grant a tool WeKnora does not have — and two
// source names mapping to one tool (e.g. "Read, 阅读") collapse to a single
// target, because the sub-run SDK rejects duplicate declarations.
func MapRoleTools(raw string) []string {
	names := ParseRoleTools(raw)
	mapped := make([]string, 0, len(names))
	seen := make(map[string]bool, len(names))
	var dropped []string
	for _, name := range names {
		target, ok := roleToolNameMap[name]
		if !ok {
			dropped = append(dropped, name)
			continue
		}
		if seen[target] {
			continue
		}
		seen[target] = true
		mapped = append(mapped, target)
	}
	if len(dropped) > 0 {
		logger.Warnf(context.Background(),
			"[Subagents] role tools without a WeKnora mapping, dropped: %v", dropped)
	}
	return mapped
}

// ParseRoleMarkdown splits one role markdown document — frontmatter fence
// included, exactly as a tenant_subagents Content column stores it verbatim —
// into its parsed frontmatter and the body below the closing fence. It reuses
// the scanner's split, so installed rows and scanned library files parse the
// same way. A missing fence pair, unreadable YAML or an empty name is an
// error; the caller drops such rows rather than delegating to a broken role.
func ParseRoleMarkdown(content string) (SubagentFrontmatter, string, error) {
	fmSource, body, ok := splitFrontmatter([]byte(content))
	if !ok {
		return SubagentFrontmatter{}, "", errors.New("subagents: role markdown is missing the --- frontmatter fence")
	}
	var fm SubagentFrontmatter
	if err := yaml.Unmarshal([]byte(fmSource), &fm); err != nil {
		return SubagentFrontmatter{}, "", fmt.Errorf("subagents: parse role frontmatter: %w", err)
	}
	if strings.TrimSpace(fm.Name) == "" {
		return SubagentFrontmatter{}, "", errors.New("subagents: role frontmatter name is empty")
	}
	return fm, string(body), nil
}

// ResolvedRole is one installed specialist as the delegate tool needs it: the
// role markdown's body (the sub-run system prompt), the verbatim tools
// frontmatter (mapped lazily by MapRoleTools at delegation time) and the
// frontmatter display name.
type ResolvedRole struct {
	Slug  string
	Label string
	// SystemPrompt is the role markdown body below the frontmatter fence.
	SystemPrompt string
	// ToolsRaw is the verbatim frontmatter tools value.
	ToolsRaw string
}

// DelegateArgs is the complete model-facing argument surface of the
// sub-agent delegation tool. The model may only name one of the installed
// slugs and state a goal plus already-authorized input references: session,
// user, model, tool whitelist and executor are all assembled server-side and
// injection attempts are rejected by ParseDelegateArgs.
type DelegateArgs struct {
	Slug      string   `json:"slug"`
	Goal      string   `json:"goal"`
	InputRefs []string `json:"input_refs"`
}

// ParseDelegateArgs strictly decodes model arguments (the craft_delegate
// pattern): unknown fields are rejected, trailing input after the first JSON
// value is rejected, and blank slug or goal is rejected — an empty slug can
// never resolve to an installed specialist.
func ParseDelegateArgs(raw []byte) (DelegateArgs, error) {
	var a DelegateArgs
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&a); err != nil {
		return a, err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return a, errors.New("trailing input")
	}
	if strings.TrimSpace(a.Slug) == "" {
		return a, errors.New("slug required")
	}
	if strings.TrimSpace(a.Goal) == "" {
		return a, errors.New("goal required")
	}
	return a, nil
}

// SubagentDelegateExec performs the budgeted sub-run (subagents.Execute). It
// is a config field so tests substitute a fake.
type SubagentDelegateExec func(ctx context.Context, req ExecuteRequest) (ExecuteResult, error)

// SubagentDelegateLookup resolves one installed specialist by slug. Returning
// ErrSubagentNotInstalled (wrapped is fine) renders the model-facing
// "subagent not installed: <slug>"; any other error is surfaced as a resolve
// failure.
type SubagentDelegateLookup func(slug string) (ResolvedRole, error)

// SubagentDelegateToolConfig carries the server-assembled execution identity
// for one run. Nothing in it is model-controllable.
type SubagentDelegateToolConfig struct {
	// SessionID is the MAIN session id; the sub-run attributes its tool
	// events to it and derives its own trpc session from it.
	SessionID string
	// UserID is the authenticated user of the main run; it rides the
	// sub-run's ToolExecContext so HITL gates can authorize the caller. It
	// is resolved from the registration context; on durable runs (whose
	// worker ctx carries no UserIDContextKey) Execute falls back to the
	// ToolExecContext the graph wraps every tool call in.
	UserID string
	// Slugs are the specialists installed for this tenant AND configured on
	// this agent. They are rendered into the description at construction —
	// the model never sees uninstalled or unconfigured roles.
	Slugs []string
	// AllowedTools is the main agent's effective allowlist
	// (config.AllowedTools or the defaults — the registerTools semantics).
	// The role's mapped tools are intersected with it.
	AllowedTools []string
	// Registry is the main run's tool registry; the intersected tool
	// INSTANCES are pulled from it at delegation time, so capability-gated
	// late registrations (sandbox files, shell) are honored. nil never
	// yields tools (test convenience).
	Registry *tools.ToolRegistry
	// Model is the main run's model client, reused by the sub-run.
	Model chat.Chat
	// Lookup resolves an installed specialist's role markdown.
	Lookup SubagentDelegateLookup
	// Exec performs the budgeted sub-run.
	Exec SubagentDelegateExec
	// Emit forwards the sub-run's tool events to the main run's event bus.
	// nil skips event emission.
	Emit func(ctx context.Context, e event.Event)
}

// SubagentDelegateTool lets the main agent delegate a self-contained subtask
// to an installed specialist sub-agent and read back its bounded summary.
type SubagentDelegateTool struct {
	tools.BaseTool
	cfg SubagentDelegateToolConfig
}

// NewSubagentDelegateTool validates the assembled identity and returns the
// tool. The description lists exactly the slugs in cfg — assembled here, at
// registration time, so the model cannot see uninstalled specialists.
func NewSubagentDelegateTool(cfg SubagentDelegateToolConfig) (*SubagentDelegateTool, error) {
	if strings.TrimSpace(cfg.SessionID) == "" {
		return nil, errors.New("subagent delegate requires the main session id")
	}
	slugs := normalizeDelegateSlugs(cfg.Slugs)
	if len(slugs) == 0 {
		return nil, errors.New("subagent delegate requires at least one installed specialist")
	}
	if cfg.Lookup == nil {
		return nil, errors.New("subagent delegate requires a role lookup")
	}
	if cfg.Exec == nil {
		return nil, errors.New("subagent delegate requires an executor")
	}
	if cfg.Model == nil {
		return nil, errors.New("subagent delegate requires the main run model")
	}
	cfg.Slugs = slugs
	schema := json.RawMessage(`{
  "type": "object",
  "properties": {
    "slug": {
      "type": "string",
      "description": "The specialist to delegate to. Must be one of the installed slugs listed in this tool's description."
    },
    "goal": {
      "type": "string",
      "description": "The complete, self-contained goal for the specialist. It does not see this conversation, so include everything it needs."
    },
    "input_refs": {
      "type": "array",
      "items": {"type": "string"},
      "description": "References of inputs already authorized for this session, appended to the specialist's goal as additional context."
    }
  },
  "required": ["slug", "goal"],
  "additionalProperties": false
}`)
	return &SubagentDelegateTool{
		BaseTool: tools.NewBaseTool(ToolSubagentDelegate,
			subagentDelegateDescription(slugs), schema),
		cfg: cfg,
	}, nil
}

// subagentDelegateDescription renders the model-facing description with the
// run's actual specialist list.
func subagentDelegateDescription(slugs []string) string {
	return "Delegate a self-contained subtask to one of this agent's installed specialist sub-agents. " +
		"Each specialist runs its own short reasoning loop under its own role prompt and returns a summary of its work; it does not see this conversation. " +
		"Available specialists (use the slug exactly): " + strings.Join(slugs, ", ") + ". " +
		"Delegate work that benefits from a focused single-purpose pass (review, research, drafting) and answer direct questions yourself."
}

// normalizeDelegateSlugs trims, drops blanks and dedupes preserving order.
func normalizeDelegateSlugs(slugs []string) []string {
	seen := make(map[string]bool, len(slugs))
	out := make([]string, 0, len(slugs))
	for _, slug := range slugs {
		slug = strings.TrimSpace(slug)
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		out = append(out, slug)
	}
	return out
}

// intersectRoleTools narrows a role's declared tools to the tool instances
// this run can actually hand over: mapped names ∩ the main agent's effective
// allowlist ∩ the constructed registry. A tool named on the allowlist but not
// constructed this run (e.g. web tools while WebSearchEnabled is off) is
// skipped — the sub-agent never gets a tool the main agent could not call.
// An empty result is first-class: most roles declare no tools.
func (t *SubagentDelegateTool) intersectRoleTools(ctx context.Context, toolsRaw string) []types.Tool {
	if t.cfg.Registry == nil {
		return nil
	}
	allowed := make(map[string]bool, len(t.cfg.AllowedTools))
	for _, name := range t.cfg.AllowedTools {
		allowed[name] = true
	}
	var execTools []types.Tool
	for _, name := range MapRoleTools(toolsRaw) {
		if !allowed[name] {
			continue
		}
		tool, err := t.cfg.Registry.GetTool(name)
		if err != nil {
			logger.Infof(ctx, "[Subagents] role tool %s is allowed but not constructed this run; skipped", name)
			continue
		}
		execTools = append(execTools, tool)
	}
	return execTools
}

// delegationUserID resolves the user the sub-run acts for. Live sessions
// carry the authenticated user id on the registration context (cfg.UserID).
// Durable-resumed runs do not: the worker's context has no request identity,
// so registration captured an empty id — but the durable graph wraps every
// tool call in a ToolExecContext carrying the run row's user id, the same
// identity the main run's own tools get (agent_run_graph.go executeTool).
// Falling back to it there keeps delegation's authorization identical to the
// main run's tools; the registration-time id keeps precedence everywhere a
// live assembly resolved one.
func (t *SubagentDelegateTool) delegationUserID(ctx context.Context) string {
	if strings.TrimSpace(t.cfg.UserID) != "" {
		return t.cfg.UserID
	}
	if meta, ok := tools.ToolExecFromContext(ctx); ok {
		return meta.UserID
	}
	return ""
}

// Execute resolves the strictly parsed slug against the installed roles,
// intersects the role's tools with the main run's, delegates to the budgeted
// sub-run and returns its summary as the tool result. Every failure is an
// unsuccessful ToolResult so the model can see it and decide; only the
// arguments are parsed before any resolution begins.
func (t *SubagentDelegateTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	parsed, err := ParseDelegateArgs(args)
	if err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("subagent_delegate arguments rejected: %v", err),
		}, nil
	}
	role, err := t.cfg.Lookup(parsed.Slug)
	if err != nil {
		if errors.Is(err, ErrSubagentNotInstalled) {
			return &types.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("subagent not installed: %s", parsed.Slug),
			}, nil
		}
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("resolve subagent %s: %v", parsed.Slug, err),
		}, nil
	}
	if role.Slug == "" {
		role.Slug = parsed.Slug
	}

	execTools := t.intersectRoleTools(ctx, role.ToolsRaw)
	result, err := t.cfg.Exec(ctx, ExecuteRequest{
		SessionID:    t.cfg.SessionID,
		RunLabel:     subagentRunLabelPrefix + role.Slug,
		SystemPrompt: role.SystemPrompt,
		Goal:         parsed.Goal,
		InputRefs:    parsed.InputRefs,
		Model:        t.cfg.Model,
		Tools:        execTools,
		UserID:       t.delegationUserID(ctx),
		Emit:         t.cfg.Emit,
	})
	if err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("subagent run failed: %v", err),
		}, nil
	}

	summary := result.Summary
	if len(execTools) == 0 {
		// First-class toolless run — but the model must know why the
		// specialist could not look anything up.
		if strings.TrimSpace(summary) == "" {
			summary = subagentNoToolsNote
		} else {
			summary = subagentNoToolsNote + "\n\n" + summary
		}
	}
	return &types.ToolResult{
		Success: true,
		Output:  summary,
		Data: map[string]interface{}{
			"slug":    role.Slug,
			"rounds":  result.Rounds,
			"overrun": result.Overrun,
		},
	}, nil
}
