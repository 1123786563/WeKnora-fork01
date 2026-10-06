package subagents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/agent/trpc"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"trpc.group/trpc-go/trpc-agent-go/agent/graphagent"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/session/noop"
	sdktool "trpc.group/trpc-go/trpc-agent-go/tool"
)

// Sub-run budgets. Rounds bounds how many times the sub-agent may visit the
// model (a tool-loop round counts one model call); OutputChars bounds the
// cumulative assistant text across rounds. Crossing either budget stops the
// sub-run and reports a partial result with Overrun set — it is never an
// error. SummaryLimit caps the returned summary (8 KiB, tail-preserving),
// mirroring the craft_delegate summary cap.
const (
	subagentMaxRounds      = 8
	subagentMaxOutputChars = 32 << 10
	subagentSummaryLimit   = 8 << 10
)

const (
	// subagentRunLabelPrefix marks a run label as a sub-agent run; the rest of
	// the label is the role slug.
	subagentRunLabelPrefix = "subagent:"
	// subagentOverrunNote is appended to the summary when a budget stopped the
	// sub-run, so the delegating model sees why the result is partial.
	subagentOverrunNote = "[budget exceeded]"
	// subagentTruncationMarker heads an over-limit summary whose beginning was
	// dropped to keep the tail (the sub-run's conclusion).
	subagentTruncationMarker = "…(truncated)\n"

	subagentRunnerName  = "weknora-subagent"
	subagentDefaultUser = "subagent"

	subagentNodeAgent   = "agent"
	subagentNodeTools   = "tools"
	subagentBranchTools = "tools"
	subagentBranchEnd   = "end"
)

// ExecuteRequest describes one sub-agent delegation. The caller (the delegate
// tool) resolves the role, intersects the tool set, and passes the main run's
// model client and event emit func; Execute owns the trpc sub-run.
type ExecuteRequest struct {
	// SessionID is the MAIN session id. Sub-run tool events are attributed to
	// it; the sub-run itself uses a fresh trpc session derived from it.
	SessionID string
	// RunLabel is "subagent:<slug>". The fresh trpc session id is
	// SessionID + ":" + slug, which isolates the sub-run's history.
	RunLabel string
	// SystemPrompt is the role markdown body used as the sub-run system prompt.
	SystemPrompt string
	// Goal is the delegation goal from the model's delegate call.
	Goal string
	// InputRefs references inputs the server already authorized; appended to
	// the goal as an "Additional context:" block.
	InputRefs []string
	// Model is the reused main-run model client.
	Model chat.Chat
	// Tools is the caller-intersected tool set. Empty is first-class: the
	// sub-run is a single LLM call (most roles declare no tools).
	Tools []types.Tool
	// UserID is the authenticated user of the main run (optional; the
	// delegate tool wires it). It rides the per-call ToolExecContext so
	// HITL gates (e.g. MCP approval, issue #1173) can authorize the caller.
	UserID string
	// Emit receives the sub-run's tool call/result events on the MAIN
	// session. Nil skips event emission (test convenience).
	Emit func(ctx context.Context, e event.Event)
}

// ExecuteResult is the bounded decision surface of one sub-agent run.
type ExecuteResult struct {
	// Summary is the final assistant text, capped at subagentSummaryLimit
	// (tail-preserving). When a budget stopped the run it carries the
	// "[budget exceeded]" note.
	Summary string
	// Rounds is the number of model calls the sub-run actually made.
	Rounds int
	// Overrun reports that a budget stopped the sub-run early.
	Overrun bool
}

// Execute runs one budgeted sub-agent trpc sub-run: a lightweight LLM/tools
// graph (per the compatibility probe pattern, without its checkpoint probe
// machinery) driven by a runner with a no-op session service and a fresh
// session id, so the sub-run's history never leaks into the main run.
//
// Model or graph errors return a wrapped error and no result. Budget overrun
// is not an error: the partial text is returned with Overrun set.
func Execute(ctx context.Context, req ExecuteRequest) (ExecuteResult, error) {
	if req.Model == nil {
		return ExecuteResult{}, errors.New("subagent execute: model is required")
	}
	if strings.TrimSpace(req.SessionID) == "" {
		return ExecuteResult{}, errors.New("subagent execute: session id is required")
	}
	if strings.TrimSpace(req.Goal) == "" {
		return ExecuteResult{}, errors.New("subagent execute: goal is required")
	}
	slug, ok := subagentRunSlug(req.RunLabel)
	if !ok {
		return ExecuteResult{}, fmt.Errorf("subagent execute: run label must be %q<slug>, got %q",
			subagentRunLabelPrefix, req.RunLabel)
	}

	budget := &budgetModel{inner: trpc.NewModel(req.Model)}
	sdkTools := make(map[string]sdktool.Tool, len(req.Tools))
	for _, t := range req.Tools {
		if t == nil {
			return ExecuteResult{}, errors.New("subagent execute: nil tool")
		}
		decl, err := subagentToolDeclaration(t)
		if err != nil {
			return ExecuteResult{}, fmt.Errorf("subagent execute: %w", err)
		}
		if _, dup := sdkTools[decl.Name]; dup {
			return ExecuteResult{}, fmt.Errorf("subagent execute: duplicate tool %q", decl.Name)
		}
		sdkTools[decl.Name] = &subagentTool{
			inner:      t,
			decl:       decl,
			attributed: subagentRunLabelPrefix + slug + ":" + decl.Name,
			sessionID:  req.SessionID,
			userID:     req.UserID,
			emit:       req.Emit,
			round:      budget.round,
		}
	}

	g, err := buildSubagentGraph(budget, req.SystemPrompt, sdkTools)
	if err != nil {
		return ExecuteResult{}, fmt.Errorf("subagent execute: build graph: %w", err)
	}
	ag, err := graphagent.New(subagentRunnerName, g)
	if err != nil {
		return ExecuteResult{}, fmt.Errorf("subagent execute: build agent: %w", err)
	}
	r := runner.NewRunner(subagentRunnerName, ag, runner.WithSessionService(noop.NewService()))
	defer func() { _ = r.Close() }()

	userID, ok := types.UserIDFromContext(ctx)
	if !ok || userID == "" {
		userID = subagentDefaultUser
	}
	events, err := r.Run(ctx, userID, req.SessionID+":"+slug,
		model.NewUserMessage(subagentUserMessage(req.Goal, req.InputRefs)))
	if err != nil {
		return ExecuteResult{}, fmt.Errorf("subagent run %q: %w", req.RunLabel, err)
	}
	// Drain the whole channel even after a failure (probe behavior): the
	// runner closes it only once the run has fully unwound.
	var runErr error
	for evt := range events {
		if runErr != nil {
			continue
		}
		if evt == nil || evt.Response == nil {
			continue
		}
		if evt.Error != nil {
			runErr = fmt.Errorf("subagent run %q: %s: %s",
				req.RunLabel, evt.Error.Type, evt.Error.Message)
		}
	}
	if runErr != nil {
		return ExecuteResult{}, runErr
	}
	if err := ctx.Err(); err != nil {
		return ExecuteResult{}, fmt.Errorf("subagent run %q: %w", req.RunLabel, err)
	}

	lastText, rounds, overrun := budget.snapshot()
	return ExecuteResult{
		Summary: capSubagentSummary(lastText, overrun),
		Rounds:  rounds,
		Overrun: overrun,
	}, nil
}

// subagentRunSlug extracts the role slug from a "subagent:<slug>" run label.
func subagentRunSlug(label string) (string, bool) {
	if !strings.HasPrefix(label, subagentRunLabelPrefix) {
		return "", false
	}
	slug := strings.TrimPrefix(label, subagentRunLabelPrefix)
	if strings.TrimSpace(slug) == "" {
		return "", false
	}
	return slug, true
}

// subagentUserMessage renders the delegation user message: the goal followed
// by an "Additional context:" block when input refs were authorized.
func subagentUserMessage(goal string, refs []string) string {
	var b strings.Builder
	b.WriteString(goal)
	if len(refs) > 0 {
		b.WriteString("\n\nAdditional context:\n")
		for _, ref := range refs {
			fmt.Fprintf(&b, "- %s\n", ref)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// capSubagentSummary bounds the summary to subagentSummaryLimit bytes on a
// rune boundary, tail-preserving (the sub-run's conclusion lives at the end).
// On budget overrun the "[budget exceeded]" note is appended first so it
// survives truncation.
func capSubagentSummary(text string, overrun bool) string {
	if overrun {
		if strings.TrimSpace(text) == "" {
			text = subagentOverrunNote
		} else {
			text += "\n\n" + subagentOverrunNote
		}
	}
	if len(text) <= subagentSummaryLimit {
		return text
	}
	cut := subagentSummaryLimit - len(subagentTruncationMarker)
	for cut > 0 && !utf8RuneStart(text[cut]) {
		cut--
	}
	return subagentTruncationMarker + text[len(text)-cut:]
}

func utf8RuneStart(b byte) bool { return b&0xC0 != 0x80 }

// budgetModel wraps the sub-run's model adapter and enforces the round and
// output budgets. It also records the run observables Execute reports:
// rounds, cumulative chars, overrun, and the last assistant text.
type budgetModel struct {
	inner model.Model

	mu       sync.Mutex
	rounds   int
	chars    int
	overrun  bool
	lastText string
}

func (m *budgetModel) Info() model.Info { return m.inner.Info() }

// GenerateContent enforces the round budget before the call and the output
// budget on each completed response. On budget exhaustion it returns a
// terminal text-only response so the graph loop ends without another model
// call. On output overrun it strips tool calls from the over-budget response
// so the loop terminates with the partial text.
func (m *budgetModel) GenerateContent(ctx context.Context, req *model.Request) (<-chan *model.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	if m.rounds >= subagentMaxRounds {
		m.overrun = true
		m.mu.Unlock()
		return syntheticTerminalResponse(), nil
	}
	m.rounds++
	m.mu.Unlock()

	in, err := m.inner.GenerateContent(ctx, req)
	if err != nil {
		return nil, err
	}
	out := make(chan *model.Response, 1)
	go func() {
		defer close(out)
		for resp := range in {
			if resp == nil {
				continue
			}
			select {
			case out <- m.observe(resp):
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// observe records one response and applies the output budget. Error and
// partial responses pass through untouched.
func (m *budgetModel) observe(resp *model.Response) *model.Response {
	if resp.Error != nil || !resp.Done || len(resp.Choices) == 0 {
		return resp
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	message := resp.Choices[0].Message
	if message.Content != "" {
		m.lastText = message.Content
	}
	m.chars += len(message.Content)
	if m.chars > subagentMaxOutputChars {
		m.overrun = true
	}
	if !m.overrun || len(message.ToolCalls) == 0 {
		return resp
	}
	// Budget stop: return the text without the planned tool calls so the
	// graph routes to its end instead of looping.
	stripped := *resp
	choices := make([]model.Choice, len(resp.Choices))
	copy(choices, resp.Choices)
	strippedMessage := message
	strippedMessage.ToolCalls = nil
	choices[0].Message = strippedMessage
	stripped.Choices = choices
	return &stripped
}

// syntheticTerminalResponse is the model-less terminal answer used when the
// round budget blocks another model call.
func syntheticTerminalResponse() <-chan *model.Response {
	stop := "stop"
	out := make(chan *model.Response, 1)
	out <- &model.Response{
		Object: model.ObjectTypeChatCompletion, Done: true,
		Choices: []model.Choice{{
			Message:      model.Message{Role: model.RoleAssistant},
			FinishReason: &stop,
		}},
	}
	close(out)
	return out
}

func (m *budgetModel) round() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rounds
}

// roundsExhausted reports whether another model round would exceed the round
// budget (the current round already used it up).
func (m *budgetModel) roundsExhausted() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rounds >= subagentMaxRounds
}

func (m *budgetModel) markOverrun() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.overrun = true
}

func (m *budgetModel) snapshot() (lastText string, rounds int, overrun bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastText, m.rounds, m.overrun
}

// buildSubagentGraph builds the sub-run graph. With no tools it is a single
// LLM node (one model call, still a valid delegation); with tools it is the
// LLM → tools loop, where the conditional edge routes to the tools node only
// while the round budget allows another model round.
func buildSubagentGraph(budget *budgetModel, systemPrompt string,
	sdkTools map[string]sdktool.Tool,
) (*graph.Graph, error) {
	builder := graph.NewStateGraph(graph.MessagesStateSchema())
	// Non-streaming: the sub-run's output is consumed as one final text, and
	// the budget wrapper reasons about complete responses.
	opts := []graph.Option{graph.WithGenerationConfig(model.GenerationConfig{Stream: false})}

	if len(sdkTools) == 0 {
		builder.AddLLMNode(subagentNodeAgent, budget, systemPrompt, nil, opts...)
		builder.SetEntryPoint(subagentNodeAgent)
		builder.SetFinishPoint(subagentNodeAgent)
		return builder.Compile()
	}

	condition := func(_ context.Context, state graph.State) (string, error) {
		msgs, _ := state[graph.StateKeyMessages].([]model.Message)
		if len(msgs) == 0 {
			return subagentBranchEnd, nil
		}
		last := msgs[len(msgs)-1]
		if len(last.ToolCalls) == 0 {
			return subagentBranchEnd, nil
		}
		if budget.roundsExhausted() {
			// One more tool round would need a model round the budget cannot
			// grant: end with the partial text instead of executing tools the
			// model will never see.
			budget.markOverrun()
			return subagentBranchEnd, nil
		}
		return subagentBranchTools, nil
	}

	builder.AddLLMNode(subagentNodeAgent, budget, systemPrompt, sdkTools, opts...)
	builder.AddToolsNode(subagentNodeTools, sdkTools)
	builder.AddConditionalEdges(subagentNodeAgent, condition, map[string]string{
		subagentBranchTools: subagentNodeTools,
		subagentBranchEnd:   graph.End,
	})
	builder.AddEdge(subagentNodeTools, subagentNodeAgent)
	builder.SetEntryPoint(subagentNodeAgent)
	return builder.Compile()
}

// subagentToolDeclaration validates and converts one repo tool's declaration
// to the SDK surface. An empty parameter schema becomes a bare object schema:
// the model adapter rejects declarations without an input schema.
func subagentToolDeclaration(t types.Tool) (*sdktool.Declaration, error) {
	name := t.Name()
	if name == "" {
		return nil, errors.New("tool name is required")
	}
	decl := &sdktool.Declaration{
		Name: name, Description: t.Description(),
		InputSchema: &sdktool.Schema{Type: "object"},
	}
	if params := t.Parameters(); len(params) > 0 {
		schema := &sdktool.Schema{}
		if err := json.Unmarshal(params, schema); err != nil {
			return nil, fmt.Errorf("tool %s parameters: %w", name, err)
		}
		decl.InputSchema = schema
	}
	return decl, nil
}

// subagentTool adapts one repo tool to the SDK CallableTool surface and
// attributes its execution on the main transcript: every call and result is
// also emitted on the MAIN session id with the "subagent:<slug>:" tool-name
// prefix, with the same sanitization the main engine applies (file bodies
// stripped from sandbox write/edit call args; bulky Data keys stripped from
// results). The tool executes under a ToolExecContext carrying the MAIN
// session id, the attributed call id, and the caller's user id, so HITL
// gates and session-attributed tools behave as in the main run. A tool
// failure is data for the model, never a graph error.
type subagentTool struct {
	inner      types.Tool
	decl       *sdktool.Declaration
	attributed string // "subagent:<slug>:<tool>"
	sessionID  string
	userID     string
	emit       func(context.Context, event.Event)
	round      func() int
}

func (t *subagentTool) Declaration() *sdktool.Declaration { return t.decl }

func (t *subagentTool) Call(ctx context.Context, args []byte) (any, error) {
	callID, _ := ctx.Value(sdktool.ContextKeyToolCallID{}).(string)
	if callID == "" {
		callID = uuid.NewString()
	}
	ctx = tools.WithToolExecContext(ctx, &tools.ToolExecContext{
		SessionID:  t.sessionID,
		ToolCallID: callID,
		UserID:     t.userID,
		// EventBus stays nil here; the delegate tool (M3 T5) wires the main
		// run's bus so approval gates can resolve synchronously.
	})
	var arguments map[string]any
	if len(args) > 0 {
		_ = json.Unmarshal(args, &arguments) // best effort: the event payload is informational
	}
	t.emitEvent(ctx, event.Event{
		ID:        callID + "-tool-hint",
		Type:      event.EventAgentToolCall,
		SessionID: t.sessionID,
		Data: event.AgentToolCallData{
			ToolCallID: callID,
			ToolName:   t.attributed,
			Arguments:  tools.SanitizeSandboxFileCallArgs(t.inner.Name(), arguments),
			Iteration:  t.round(),
		},
	})

	start := time.Now()
	result, err := t.inner.Execute(ctx, args)
	output, errText, success := renderSubagentToolResult(result, err)

	t.emitEvent(ctx, event.Event{
		ID:        callID + "-tool-result",
		Type:      event.EventAgentToolResult,
		SessionID: t.sessionID,
		Data: event.AgentToolResultData{
			ToolCallID: callID,
			ToolName:   t.attributed,
			Output:     output,
			Error:      errText,
			Success:    success,
			Duration:   time.Since(start).Milliseconds(),
			Iteration:  t.round(),
			Data:       tools.SanitizeToolDataForPersist(t.inner.Name(), subagentToolEventData(result)),
		},
	})
	return output, nil
}

func (t *subagentTool) emitEvent(ctx context.Context, e event.Event) {
	if t.emit == nil {
		return
	}
	t.emit(ctx, e)
}

// renderSubagentToolResult renders the model-facing text plus the event
// error/success fields. A failed tool reports its error text to the model
// instead of failing the sub-run.
func renderSubagentToolResult(result *types.ToolResult, err error) (output, errText string, success bool) {
	switch {
	case err != nil:
		return fmt.Sprintf("tool error: %v", err), err.Error(), false
	case result == nil:
		return "tool error: no result", "no result", false
	case result.Error != "":
		out := result.Output
		if out != "" {
			out += "\n"
		}
		return out + "tool error: " + result.Error, result.Error, false
	default:
		return result.Output, "", result.Success
	}
}

func subagentToolEventData(result *types.ToolResult) map[string]interface{} {
	if result == nil {
		return nil
	}
	return result.Data
}
