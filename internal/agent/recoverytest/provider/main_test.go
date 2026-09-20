package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
)

func TestCounterCountRejectsMalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	if _, err := counterCount(server.URL); err == nil {
		t.Fatal("counterCount() error = nil, want invalid response error")
	}
}

func TestCompletedAssistantRowsSQLUsesDialectBooleanLiteral(t *testing.T) {
	if got := completedAssistantRowsSQL("postgres"); !strings.Contains(got, "is_completed = true") {
		t.Fatalf("PostgreSQL completed-message query = %q, want boolean predicate", got)
	}
	if got := completedAssistantRowsSQL("sqlite"); !strings.Contains(got, "is_completed = 1") {
		t.Fatalf("SQLite completed-message query = %q, want integer predicate", got)
	}
}

func TestNewRecoveryChatUsesScriptedModelWithoutOllamaConfiguration(t *testing.T) {
	t.Setenv(recoveryOllamaModelEnv, "")

	model, prompt, err := newRecoveryChat(toolName)
	if err != nil {
		t.Fatalf("newRecoveryChat() error = %v", err)
	}
	if _, ok := model.(*scriptedModel); !ok {
		t.Fatalf("newRecoveryChat() = %T, want *scriptedModel", model)
	}
	if prompt != "" {
		t.Fatalf("newRecoveryChat() prompt = %q, want empty", prompt)
	}
}

func TestNewRecoveryChatUsesConfiguredOllamaModel(t *testing.T) {
	t.Setenv(recoveryOllamaModelEnv, "qwen2.5:0.5b")
	t.Setenv("OLLAMA_BASE_URL", "http://127.0.0.1:11434")

	model, prompt, err := newRecoveryChat(toolName)
	if err != nil {
		t.Fatalf("newRecoveryChat() error = %v", err)
	}
	contract, ok := model.(*recoveryContractChat)
	if !ok {
		t.Fatalf("newRecoveryChat() = %T, want *recoveryContractChat", model)
	}
	ollamaChat, ok := contract.inner.(*chat.OllamaChat)
	if !ok {
		t.Fatalf("recovery contract chat = %T, want *chat.OllamaChat", contract.inner)
	}
	if got := ollamaChat.GetModelName(); got != "qwen2.5:0.5b" {
		t.Fatalf("Ollama model name = %q", got)
	}
	if got := ollamaChat.GetModelID(); got != "recovery-ollama:qwen2.5:0.5b" {
		t.Fatalf("Ollama model ID = %q", got)
	}
	if prompt == "" {
		t.Fatal("configured Ollama path must add its durable tool contract prompt")
	}
}

func TestRecoveryContractChatRejectsUnplannedInitialAnswer(t *testing.T) {
	model := &recoveryContractChat{inner: scriptedChatResponse{response: &types.ChatResponse{
		Content: "I will not call the tool", FinishReason: "stop",
	}}, toolName: toolName}

	_, err := model.Chat(context.Background(), []chat.Message{{Role: "user", Content: "count one"}}, nil)
	if err == nil {
		t.Fatal("Chat() error = nil, want contract violation")
	}
}

func TestRecoveryContractChatAllowsNaturalLanguagePostToolAnswer(t *testing.T) {
	model := &recoveryContractChat{inner: scriptedChatResponse{response: &types.ChatResponse{
		Content: "The counter is now one.", FinishReason: "stop",
	}}, toolName: toolName}

	_, err := model.Chat(context.Background(), []chat.Message{{Role: "tool", Name: toolName, Content: `{"count":1}`}}, nil)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
}

func TestRecoveryContractChatUsesDeterministicOptionsForEachPhase(t *testing.T) {
	inner := &recordingChat{}
	model := &recoveryContractChat{inner: inner, toolName: toolName}
	if _, err := model.Chat(context.Background(), []chat.Message{{Role: "user", Content: "count one"}}, &chat.ChatOptions{Temperature: 0.9}); err != nil {
		t.Fatalf("initial Chat() error = %v", err)
	}
	stream, err := model.ChatStream(context.Background(), []chat.Message{{Role: "tool", Name: toolName, Content: `{"count":1}`}}, nil)
	if err != nil {
		t.Fatalf("post-tool ChatStream() error = %v", err)
	}
	for range stream {
	}
	if len(inner.options) != 2 {
		t.Fatalf("calls = %d, want 2", len(inner.options))
	}
	assertDeterministicOptions(t, inner.options[0], "required")
	assertDeterministicOptions(t, inner.options[1], "none")
}

func assertDeterministicOptions(t *testing.T, opts *chat.ChatOptions, choice string) {
	t.Helper()
	if opts == nil || opts.Temperature != 0 || opts.TopP != 1 || opts.Seed != recoveryOllamaSeed ||
		opts.MaxTokens != recoveryOllamaMaxTokens || opts.MaxCompletionTokens != 0 || opts.Thinking == nil || *opts.Thinking || opts.ToolChoice != choice {
		t.Fatalf("options = %#v, want deterministic %q options", opts, choice)
	}
}

type scriptedChatResponse struct{ response *types.ChatResponse }

func (s scriptedChatResponse) GetModelName() string { return "test" }
func (s scriptedChatResponse) GetModelID() string   { return "test" }
func (s scriptedChatResponse) Chat(context.Context, []chat.Message, *chat.ChatOptions) (*types.ChatResponse, error) {
	return s.response, nil
}
func (s scriptedChatResponse) ChatStream(context.Context, []chat.Message, *chat.ChatOptions) (<-chan types.StreamResponse, error) {
	ch := make(chan types.StreamResponse)
	close(ch)
	return ch, nil
}

type recordingChat struct{ options []*chat.ChatOptions }

func (*recordingChat) GetModelName() string { return "test" }
func (*recordingChat) GetModelID() string   { return "test" }
func (c *recordingChat) Chat(_ context.Context, messages []chat.Message, opts *chat.ChatOptions) (*types.ChatResponse, error) {
	c.options = append(c.options, opts)
	return &types.ChatResponse{FinishReason: "tool_calls", ToolCalls: []types.LLMToolCall{{
		ID: "call-1", Function: types.FunctionCall{Name: toolName, Arguments: `{"tick":1}`},
	}}}, nil
}
func (c *recordingChat) ChatStream(_ context.Context, _ []chat.Message, opts *chat.ChatOptions) (<-chan types.StreamResponse, error) {
	c.options = append(c.options, opts)
	out := make(chan types.StreamResponse, 1)
	out <- types.StreamResponse{ResponseType: types.ResponseTypeAnswer, Content: "done", Done: true}
	close(out)
	return out, nil
}
