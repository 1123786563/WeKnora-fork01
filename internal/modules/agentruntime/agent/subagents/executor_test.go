package subagents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	agenttools "github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/modules/airesource/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// scriptChat is a scriptable chat.Chat fake: every Chat call consumes the
// next scripted response via next (1-based call number). It records the last
// request messages/options for assertions.
type scriptChat struct {
	mu       sync.Mutex
	next     func(call int) (*types.ChatResponse, error)
	calls    int
	messages []chat.Message
	opts     *chat.ChatOptions
}

func (f *scriptChat) GetModelName() string { return "scripted-subagent-model" }
func (f *scriptChat) GetModelID() string   { return "scripted-subagent-model-id" }

func (f *scriptChat) Chat(_ context.Context, messages []chat.Message,
	opts *chat.ChatOptions,
) (*types.ChatResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.messages = messages
	f.opts = opts
	if f.next == nil {
		return nil, errors.New("scriptChat has no script")
	}
	return f.next(f.calls)
}

func (f *scriptChat) ChatStream(context.Context, []chat.Message,
	*chat.ChatOptions,
) (<-chan types.StreamResponse, error) {
	return nil, errors.New("scriptChat does not stream")
}

func (f *scriptChat) snapshot() (calls int, messages []chat.Message, opts *chat.ChatOptions) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls, f.messages, f.opts
}

func textResponse(text string) *types.ChatResponse {
	return &types.ChatResponse{Content: text, FinishReason: "stop"}
}

func toolCallResponse(callID, name, args string) *types.ChatResponse {
	return &types.ChatResponse{
		FinishReason: "tool_calls",
		ToolCalls: []types.LLMToolCall{{
			ID: callID, Type: "function",
			Function: types.FunctionCall{Name: name, Arguments: args},
		}},
	}
}

// fakeTool is a types.Tool that records its invocations.
type fakeTool struct {
	mu        sync.Mutex
	name      string
	calls     int
	args      []json.RawMessage
	result    *types.ToolResult
	err       error
	onExecute func(ctx context.Context)
}

func newFakeTool(name string, result *types.ToolResult) *fakeTool {
	return &fakeTool{name: name, result: result}
}

func (t *fakeTool) Name() string        { return t.name }
func (t *fakeTool) Description() string { return "fake tool " + t.name }
func (t *fakeTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}}}`)
}

func (t *fakeTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	t.mu.Lock()
	t.calls++
	t.args = append(t.args, args)
	t.mu.Unlock()
	if t.onExecute != nil {
		t.onExecute(ctx)
	}
	return t.result, t.err
}

func (t *fakeTool) snapshot() (calls int, lastArgs json.RawMessage) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.args) == 0 {
		return t.calls, nil
	}
	return t.calls, t.args[len(t.args)-1]
}

// eventLog records emitted events.
type eventLog struct {
	mu     sync.Mutex
	events []event.Event
}

func (l *eventLog) emit(_ context.Context, e event.Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, e)
}

func (l *eventLog) byType(t event.EventType) []event.Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []event.Event
	for _, e := range l.events {
		if e.Type == t {
			out = append(out, e)
		}
	}
	return out
}

func testRequest(model chat.Chat, tools []types.Tool,
	emit func(context.Context, event.Event),
) ExecuteRequest {
	return ExecuteRequest{
		SessionID:    "sess-1",
		RunLabel:     "subagent:product-manager",
		SystemPrompt: "# Product Manager\nShip the right thing.",
		Goal:         "Draft the pricing launch checklist.",
		InputRefs:    []string{"ref-doc-1", "ref-doc-2"},
		Model:        model,
		Tools:        tools,
		UserID:       "user-42",
		Emit:         emit,
	}
}

func TestExecuteNoToolsSingleRound(t *testing.T) {
	model := &scriptChat{next: func(int) (*types.ChatResponse, error) {
		return textResponse("The launch checklist is ready."), nil
	}}
	res, err := Execute(context.Background(), testRequest(model, nil, nil))
	require.NoError(t, err)
	require.Equal(t, "The launch checklist is ready.", res.Summary)
	require.Equal(t, 1, res.Rounds)
	require.False(t, res.Overrun)

	calls, messages, opts := model.snapshot()
	require.Equal(t, 1, calls)
	require.Empty(t, opts.Tools, "no tools must be advertised for a no-tools role")
	require.NotEmpty(t, messages)
	require.Equal(t, "system", messages[0].Role)
	require.Equal(t, "# Product Manager\nShip the right thing.", messages[0].Content)
	last := messages[len(messages)-1]
	require.Equal(t, "user", last.Role)
	require.Contains(t, last.Content, "Draft the pricing launch checklist.")
	require.Contains(t, last.Content, "Additional context:")
	require.Contains(t, last.Content, "- ref-doc-1")
	require.Contains(t, last.Content, "- ref-doc-2")
}

func TestExecuteToolLoopTwoRounds(t *testing.T) {
	search := newFakeTool("web_search",
		&types.ToolResult{Success: true, Output: "search-results"})
	model := &scriptChat{next: func(call int) (*types.ChatResponse, error) {
		if call == 1 {
			return toolCallResponse("call-1", "web_search", `{"query":"pricing"}`), nil
		}
		return textResponse("Final answer after research."), nil
	}}
	log := &eventLog{}
	res, err := Execute(context.Background(),
		testRequest(model, []types.Tool{search}, log.emit))
	require.NoError(t, err)
	require.Equal(t, "Final answer after research.", res.Summary)
	require.Equal(t, 2, res.Rounds)
	require.False(t, res.Overrun)

	calls, lastArgs := search.snapshot()
	require.Equal(t, 1, calls)
	require.JSONEq(t, `{"query":"pricing"}`, string(lastArgs))

	_, messages, opts := model.snapshot()
	require.Len(t, opts.Tools, 1)
	require.Equal(t, "web_search", opts.Tools[0].Function.Name)
	var toolMsg *chat.Message
	for i := range messages {
		if messages[i].Role == "tool" {
			toolMsg = &messages[i]
		}
	}
	require.NotNil(t, toolMsg, "second model call must see the tool result")
	require.Contains(t, toolMsg.Content, "search-results")

	callEvents := log.byType(event.EventAgentToolCall)
	require.Len(t, callEvents, 1)
	require.Equal(t, "sess-1", callEvents[0].SessionID)
	require.Equal(t, "call-1-tool-hint", callEvents[0].ID)
	callData, ok := callEvents[0].Data.(event.AgentToolCallData)
	require.True(t, ok)
	require.Equal(t, "call-1", callData.ToolCallID)
	require.Equal(t, "subagent:product-manager:web_search", callData.ToolName)
	require.Equal(t, map[string]any{"query": "pricing"}, callData.Arguments)

	resultEvents := log.byType(event.EventAgentToolResult)
	require.Len(t, resultEvents, 1)
	require.Equal(t, "sess-1", resultEvents[0].SessionID)
	require.Equal(t, "call-1-tool-result", resultEvents[0].ID)
	resultData, ok := resultEvents[0].Data.(event.AgentToolResultData)
	require.True(t, ok)
	require.Equal(t, "subagent:product-manager:web_search", resultData.ToolName)
	require.Equal(t, "search-results", resultData.Output)
	require.True(t, resultData.Success)
	require.Equal(t, 1, resultData.Iteration)
}

func TestExecuteRoundOverrun(t *testing.T) {
	search := newFakeTool("web_search",
		&types.ToolResult{Success: true, Output: "more-results"})
	model := &scriptChat{next: func(call int) (*types.ChatResponse, error) {
		return toolCallResponse("call-"+string(rune('0'+call)), "web_search", `{}`), nil
	}}
	res, err := Execute(context.Background(),
		testRequest(model, []types.Tool{search}, nil))
	require.NoError(t, err, "budget overrun is not an error")
	require.Equal(t, subagentMaxRounds, res.Rounds)
	require.True(t, res.Overrun)
	require.Equal(t, subagentOverrunNote, res.Summary)

	calls, _ := search.snapshot()
	require.Equal(t, subagentMaxRounds-1, calls,
		"the final round's tool calls must not execute once the round budget is spent")
	modelCalls, _, _ := model.snapshot()
	require.Equal(t, subagentMaxRounds, modelCalls)
}

func TestExecuteCharOverrun(t *testing.T) {
	huge := strings.Repeat("x", subagentMaxOutputChars+8*1024)
	model := &scriptChat{next: func(int) (*types.ChatResponse, error) {
		return textResponse(huge), nil
	}}
	res, err := Execute(context.Background(), testRequest(model, nil, nil))
	require.NoError(t, err)
	require.True(t, res.Overrun)
	require.Equal(t, 1, res.Rounds)
	require.LessOrEqual(t, len(res.Summary), subagentSummaryLimit)
	require.True(t, strings.HasSuffix(res.Summary, "\n\n"+subagentOverrunNote),
		"summary %q must end with the overrun note", res.Summary)
	body := strings.TrimSuffix(res.Summary, "\n\n"+subagentOverrunNote)
	require.True(t, strings.HasSuffix(body, huge[len(huge)-4000:]),
		"summary must preserve the tail of the model output")
}

func TestExecuteModelErrorsPropagate(t *testing.T) {
	model := &scriptChat{next: func(int) (*types.ChatResponse, error) {
		return nil, errors.New("provider exploded")
	}}
	res, err := Execute(context.Background(), testRequest(model, nil, nil))
	require.Error(t, err)
	require.Contains(t, err.Error(), "provider exploded")
	require.Equal(t, ExecuteResult{}, res)
}

func TestExecuteNilEmitSkipsEvents(t *testing.T) {
	search := newFakeTool("web_search",
		&types.ToolResult{Success: true, Output: "search-results"})
	model := &scriptChat{next: func(call int) (*types.ChatResponse, error) {
		if call == 1 {
			return toolCallResponse("call-1", "web_search", `{}`), nil
		}
		return textResponse("done"), nil
	}}
	res, err := Execute(context.Background(),
		testRequest(model, []types.Tool{search}, nil))
	require.NoError(t, err)
	require.Equal(t, "done", res.Summary)
	require.Equal(t, 2, res.Rounds)
	calls, _ := search.snapshot()
	require.Equal(t, 1, calls)
}

func TestExecuteRequestValidation(t *testing.T) {
	ctx := context.Background()
	model := &scriptChat{next: func(int) (*types.ChatResponse, error) {
		return textResponse("ok"), nil
	}}

	res, err := Execute(ctx, testRequest(nil, nil, nil))
	require.Error(t, err)
	require.Equal(t, ExecuteResult{}, res)

	badLabel := testRequest(model, nil, nil)
	badLabel.RunLabel = "product-manager"
	_, err = Execute(ctx, badLabel)
	require.Error(t, err)

	noSession := testRequest(model, nil, nil)
	noSession.SessionID = ""
	_, err = Execute(ctx, noSession)
	require.Error(t, err)

	noGoal := testRequest(model, nil, nil)
	noGoal.Goal = "   "
	_, err = Execute(ctx, noGoal)
	require.Error(t, err)
}

func TestExecuteAttachesToolExecContext(t *testing.T) {
	var got *agenttools.ToolExecContext
	var attached bool
	probe := newFakeTool("web_search", &types.ToolResult{Success: true, Output: "ok"})
	probe.onExecute = func(ctx context.Context) {
		got, attached = agenttools.ToolExecFromContext(ctx)
	}
	model := &scriptChat{next: func(call int) (*types.ChatResponse, error) {
		if call == 1 {
			return toolCallResponse("call-9", "web_search", `{}`), nil
		}
		return textResponse("done"), nil
	}}
	res, err := Execute(context.Background(),
		testRequest(model, []types.Tool{probe}, nil))
	require.NoError(t, err)
	require.Equal(t, "done", res.Summary)
	require.True(t, attached, "tool Execute must see a ToolExecContext")
	require.NotNil(t, got)
	require.Equal(t, "sess-1", got.SessionID, "main session id for attribution")
	require.Equal(t, "call-9", got.ToolCallID)
	require.Equal(t, "user-42", got.UserID)
	require.Nil(t, got.EventBus, "T5 wires the main run event bus")
}

func TestExecuteSanitizesEmittedEvents(t *testing.T) {
	// A body far beyond the engine's 10-line preview cap, with per-line
	// markers so the test can see exactly what survives sanitization.
	var lines []string
	for i := 1; i <= 50; i++ {
		lines = append(lines, fmt.Sprintf("SECRET-BODY-LINE-%02d", i))
	}
	body := strings.Join(lines, "\n")
	writer := newFakeTool("write_sandbox_file", &types.ToolResult{
		Success: true,
		Output:  "wrote file",
		Data: map[string]interface{}{
			"content":        body,
			"content_base64": "Qk9EWQ==",
			"path":           "/srv/app.go",
		},
	})
	model := &scriptChat{next: func(call int) (*types.ChatResponse, error) {
		if call == 1 {
			return toolCallResponse("call-1", "write_sandbox_file",
				fmt.Sprintf(`{"path":"/srv/app.go","content":%q}`, body)), nil
		}
		return textResponse("done"), nil
	}}
	log := &eventLog{}
	res, err := Execute(context.Background(),
		testRequest(model, []types.Tool{writer}, log.emit))
	require.NoError(t, err)
	require.Equal(t, "done", res.Summary)

	callEvents := log.byType(event.EventAgentToolCall)
	require.Len(t, callEvents, 1)
	callData, ok := callEvents[0].Data.(event.AgentToolCallData)
	require.True(t, ok)
	require.Equal(t, "subagent:product-manager:write_sandbox_file", callData.ToolName)
	require.NotContains(t, callData.Arguments, "content",
		"the raw file-body argument must be replaced by the stats preview")
	require.NotContains(t, fmt.Sprint(callData.Arguments), body,
		"file bodies must not reach the main session stream")
	require.NotContains(t, fmt.Sprint(callData.Arguments), "SECRET-BODY-LINE-42",
		"lines beyond the preview cap must be dropped")
	require.Equal(t, "/srv/app.go", callData.Arguments["path"],
		"sandbox write args keep the decision surface (path/stats)")
	require.NotNil(t, callData.Arguments["bytes"])

	resultEvents := log.byType(event.EventAgentToolResult)
	require.Len(t, resultEvents, 1)
	resultData, ok := resultEvents[0].Data.(event.AgentToolResultData)
	require.True(t, ok)
	require.Equal(t, "subagent:product-manager:write_sandbox_file", resultData.ToolName)
	require.NotContains(t, resultData.Data, "content")
	require.NotContains(t, resultData.Data, "content_base64")
	require.NotContains(t, fmt.Sprint(resultData.Data), "SECRET-BODY-LINE-42")
	require.Equal(t, "/srv/app.go", resultData.Data["path"])
}
