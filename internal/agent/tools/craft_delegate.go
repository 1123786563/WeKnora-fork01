package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
)

// ToolCraftDelegate is the model-facing name of the Craft delegation tool.
const ToolCraftDelegate = "craft_delegate"

// craftDelegateSummaryLimit bounds the delegation summary the tool returns to
// the model. The complete sub-execution log stays in Craft events and object
// storage; the main model only needs the decision surface.
const craftDelegateSummaryLimit = 8 << 10

// DelegateArgs is the complete model-facing argument surface of the craft
// delegation tool. The model may only state a goal and reference inputs the
// server already authorized for the session: every authority field (tenant,
// user, session, workspace, sandbox URL, deadlines, tool call identity) is
// assembled from the persisted run context and injection attempts are
// rejected by ParseDelegateArgs.
type DelegateArgs struct {
	Goal      string   `json:"goal"`
	InputRefs []string `json:"input_refs"`
}

// ParseDelegateArgs strictly decodes model arguments. Unknown fields — in
// particular injected authority fields such as tenant_id or sandbox_url — are
// rejected, trailing input after the first JSON value is rejected, and a
// blank goal is rejected.
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
	if strings.TrimSpace(a.Goal) == "" {
		return a, errors.New("goal required")
	}
	return a, nil
}

// CraftDelegateCall is the delegation boundary the application service
// provides (CraftDelegateService.Delegate). The tool never talks to the
// OpenCode runtime itself.
type CraftDelegateCall func(ctx context.Context, task craft.Task) (craft.Result, error)

// CraftDelegateToolConfig carries the server-assembled execution identity for
// one run. Nothing in it is model-controllable.
type CraftDelegateToolConfig struct {
	// Scope is the authenticated tenant/user/session of the run.
	Scope craft.Scope
	// WorkspaceID is the session's bound Craft workspace.
	WorkspaceID string
	// Inputs are the inputs authorized for this session. References outside
	// this manifest are rejected before any delegation is prepared.
	Inputs []craft.Input
	// SkillDigests pin the skill versions visible to the run; they are
	// covered by the delegation RequestHash.
	SkillDigests []string
	// Delegate performs the durable delegation.
	Delegate CraftDelegateCall
}

// CraftDelegateTool lets the main agent delegate implementation work to the
// sandboxed OpenCode sub-executor and inspect its verified result.
type CraftDelegateTool struct {
	BaseTool
	cfg CraftDelegateToolConfig
}

// NewCraftDelegateTool validates the assembled identity and returns the tool.
func NewCraftDelegateTool(cfg CraftDelegateToolConfig) (*CraftDelegateTool, error) {
	if cfg.Scope.TenantID == 0 || cfg.Scope.UserID == "" || cfg.Scope.SessionID == "" {
		return nil, fmt.Errorf("craft delegate requires an authenticated scope")
	}
	if strings.TrimSpace(cfg.WorkspaceID) == "" {
		return nil, fmt.Errorf("craft delegate requires a bound workspace")
	}
	if cfg.Delegate == nil {
		return nil, fmt.Errorf("craft delegate requires a delegation service")
	}
	schema := json.RawMessage(`{
  "type": "object",
  "properties": {
    "goal": {
      "type": "string",
      "description": "What the sub-executor must build or fix in this Craft workspace. Be concrete about the expected outcome and the checks it must pass."
    },
    "input_refs": {
      "type": "array",
      "items": {"type": "string"},
      "description": "References of inputs already authorized for this session. References the server did not authorize are rejected."
    }
  },
  "required": ["goal"],
  "additionalProperties": false
}`)
	return &CraftDelegateTool{
		BaseTool: NewBaseTool(ToolCraftDelegate,
			"Delegate an implementation step to the Craft sub-executor working in this session's sandboxed workspace, "+
				"then inspect its verified result. Returns the outcome with checks and produced files. "+
				"If checks failed you may delegate a fix with a new goal; the sub-executor keeps working in the same workspace.",
			schema),
		cfg: cfg,
	}, nil
}

// craftDelegateInput resolves the model's input references against the
// authorized manifest. Duplicates collapse; unknown references are refused
// before anything is prepared or dispatched.
func craftDelegateInput(refs []string, allowed []craft.Input) ([]craft.Input, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	byRef := make(map[string]craft.Input, len(allowed))
	for _, in := range allowed {
		byRef[in.Ref] = in
	}
	resolved := make([]craft.Input, 0, len(refs))
	seen := make(map[string]bool, len(refs))
	for _, ref := range refs {
		if seen[ref] {
			continue
		}
		seen[ref] = true
		in, ok := byRef[ref]
		if !ok {
			return nil, fmt.Errorf("input ref %q is not authorized for this session", ref)
		}
		resolved = append(resolved, in)
	}
	return resolved, nil
}

// CraftDelegateRequestHash binds the delegation request identity: goal,
// authorized input digests and pinned skill versions (plus the workspace for
// domain separation). A retried call with a different request is a conflict,
// not a silent replacement.
func CraftDelegateRequestHash(goal string, inputs []craft.Input, skillDigests []string, workspaceID string) string {
	h := sha256.New()
	_, _ = h.Write([]byte("craft-delegate-v1\x00"))
	_, _ = h.Write([]byte(workspaceID))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(goal))
	_, _ = h.Write([]byte{0})
	digests := make([]string, 0, len(inputs))
	for _, in := range inputs {
		digests = append(digests, in.SHA256)
	}
	sort.Strings(digests)
	for _, d := range digests {
		_, _ = h.Write([]byte(d))
		_, _ = h.Write([]byte{0})
	}
	skills := append([]string(nil), skillDigests...)
	sort.Strings(skills)
	for _, d := range skills {
		_, _ = h.Write([]byte(d))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// craftDelegatePrompt renders the prompt submitted to the sub-executor: the
// model's goal plus the staged read-only material manifest.
func craftDelegatePrompt(goal string, inputs []craft.Input) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(goal))
	if len(inputs) > 0 {
		b.WriteString("\n\nAuthorized read-only materials staged in the workspace:")
		for _, in := range inputs {
			path, err := craft.InputPath(in.SHA256, in.Name)
			if err != nil {
				path = in.Name
			}
			fmt.Fprintf(&b, "\n- %s (source ref %s, %d bytes)", path, in.Ref, in.Bytes)
		}
	}
	return b.String()
}

// craftDelegationDeadline derives the delegation deadline from the persisted
// run context: the durable worker installs the run's absolute deadline on
// the execution context, so the sub-execution is bounded by exactly the
// budget the run row persists. Without a context deadline the task carries
// no deadline and the executor applies its own bounded execution window.
func craftDelegationDeadline(ctx context.Context) time.Time {
	if dl, ok := ctx.Deadline(); ok && !dl.IsZero() {
		return dl
	}
	return time.Time{}
}

// Execute assembles the delegation task from the persisted run context and
// the strictly parsed model arguments, delegates it, and renders a bounded
// summary. craft.ErrUnknown is the only error propagated to the caller: it
// keeps the main ToolCall pending. Every definitive failure is returned as an
// unsuccessful ToolResult so the model can see it and decide to fix or
// re-delegate.
func (t *CraftDelegateTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	parsed, err := ParseDelegateArgs(args)
	if err != nil {
		return &types.ToolResult{Success: false, Error: fmt.Sprintf("craft_delegate arguments rejected: %v", err)}, nil
	}
	inputs, err := craftDelegateInput(parsed.InputRefs, t.cfg.Inputs)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error()}, nil
	}
	fence, ok := agentruntime.RunFenceFromContext(ctx)
	if !ok {
		return &types.ToolResult{Success: false, Error: "craft_delegate requires a durable run fence"}, nil
	}
	dispatch, hasDispatch := agentruntime.ToolDispatchFromContext(ctx)
	toolCallID := ""
	if hasDispatch {
		toolCallID = dispatch.CallID
	}
	if toolCallID == "" {
		if meta, hasMeta := ToolExecFromContext(ctx); hasMeta {
			toolCallID = meta.ToolCallID
		}
	}
	if toolCallID == "" {
		return &types.ToolResult{Success: false, Error: "craft_delegate requires the durable tool call identity"}, nil
	}

	task := craft.Task{
		ToolCallID:   toolCallID,
		Prompt:       craftDelegatePrompt(parsed.Goal, inputs),
		RequestHash:  CraftDelegateRequestHash(parsed.Goal, inputs, t.cfg.SkillDigests, t.cfg.WorkspaceID),
		Scope:        t.cfg.Scope,
		Fence:        fence,
		WorkspaceID:  t.cfg.WorkspaceID,
		Inputs:       inputs,
		SkillDigests: append([]string(nil), t.cfg.SkillDigests...),
		Deadline:     craftDelegationDeadline(ctx),
	}
	result, err := t.cfg.Delegate(ctx, task)
	if err != nil {
		if errors.Is(err, craft.ErrUnknown) {
			// The only pending signal: the main ToolCall waits for a
			// durable decision instead of receiving a made-up outcome.
			return nil, err
		}
		return &types.ToolResult{Success: false, Error: fmt.Sprintf("craft delegation failed: %v", err)}, nil
	}
	return renderCraftDelegateResult(result), nil
}

// renderCraftDelegateResult renders the bounded decision surface for the
// model: status, summary, checks and produced file references, capped at
// craftDelegateSummaryLimit bytes on a rune boundary.
func renderCraftDelegateResult(result craft.Result) *types.ToolResult {
	var b strings.Builder
	fmt.Fprintf(&b, "status: %s\n", result.Status)
	if result.TaskID != "" {
		fmt.Fprintf(&b, "task_id: %s\n", result.TaskID)
	}
	if result.Summary != "" {
		fmt.Fprintf(&b, "summary: %s\n", result.Summary)
	}
	if len(result.Checks) > 0 {
		b.WriteString("checks:\n")
		for _, check := range result.Checks {
			line := fmt.Sprintf("- %s: %s", check.Name, check.Status)
			if check.Detail != "" {
				line += " (" + check.Detail + ")"
			}
			fmt.Fprintf(&b, "%s\n", line)
		}
	}
	if len(result.Files) > 0 {
		b.WriteString("files:\n")
		for _, file := range result.Files {
			ref := file.Ref
			if ref == "" {
				ref = file.Path
			}
			fmt.Fprintf(&b, "- %s (%d bytes, ref %s)\n", file.Path, file.Bytes, ref)
		}
	}
	out := strings.TrimRight(b.String(), "\n")
	if len(out) > craftDelegateSummaryLimit {
		const suffix = "\n…(truncated; full log in Craft events)"
		cut := craftDelegateSummaryLimit - len(suffix)
		for cut > 0 && !utf8RuneStart(out[cut]) {
			cut--
		}
		out = out[:cut] + suffix
	}
	return &types.ToolResult{
		Success: result.Status == "succeeded",
		Output:  out,
		Data: map[string]interface{}{
			"status":  result.Status,
			"task_id": result.TaskID,
		},
	}
}

// utf8RuneStart reports whether b begins a UTF-8 rune.
func utf8RuneStart(b byte) bool { return b&0xC0 != 0x80 }
