package main

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
)

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

func TestRecoveryContractChatAllowsExactPostToolAnswer(t *testing.T) {
	model := &recoveryContractChat{inner: scriptedChatResponse{response: &types.ChatResponse{
		Content: "recovered answer", FinishReason: "stop",
	}}, toolName: toolName}

	_, err := model.Chat(context.Background(), []chat.Message{{Role: "tool", Name: toolName, Content: `{"count":1}`}}, nil)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
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
