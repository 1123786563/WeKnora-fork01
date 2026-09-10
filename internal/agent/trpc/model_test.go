package trpc

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

type modelChatFake struct {
	messages []chat.Message
	opts     *chat.ChatOptions
	response *types.ChatResponse
	chunks   []types.StreamResponse
	err      error
}

func (f *modelChatFake) GetModelName() string { return "existing-model" }
func (f *modelChatFake) GetModelID() string   { return "existing-id" }
func (f *modelChatFake) Chat(_ context.Context, messages []chat.Message,
	opts *chat.ChatOptions,
) (*types.ChatResponse, error) {
	f.messages, f.opts = messages, opts
	return f.response, f.err
}

func (f *modelChatFake) ChatStream(_ context.Context, messages []chat.Message,
	opts *chat.ChatOptions,
) (<-chan types.StreamResponse, error) {
	f.messages, f.opts = messages, opts
	ch := make(chan types.StreamResponse, len(f.chunks))
	for _, chunk := range f.chunks {
		ch <- chunk
	}
	close(ch)
	return ch, f.err
}

type declaredTool struct{ declaration *tool.Declaration }

func (d declaredTool) Declaration() *tool.Declaration { return d.declaration }

func TestModelMapsExistingChatAndPreservesMultimodal(t *testing.T) {
	f := &modelChatFake{response: &types.ChatResponse{
		Content: "answer", ReasoningContent: "reason", FinishReason: "stop",
		Usage: types.TokenUsage{
			PromptTokens: 8, CompletionTokens: 3, TotalTokens: 11,
			CacheReadTokens: 4, CacheWriteTokens: 2,
		},
		ToolCalls: []types.LLMToolCall{{
			ID: "c1", Type: "function", Function: types.FunctionCall{Name: "lookup", Arguments: `{"q":"x"}`},
			ProviderMetadata: types.ToolCallMetadata{"thought_signature": json.RawMessage(`"signed"`)},
		}},
	}}
	user := model.NewUserMessage("inspect")
	user.AddImageData([]byte{1, 2, 3}, "high", "png")
	temperature, topP, maxTokens, thinking := 0.3, 0.9, 500, true
	req := &model.Request{
		Messages: []model.Message{user, {Role: model.RoleTool, ToolID: "old", ToolName: "lookup", Content: "result"}},
		GenerationConfig: model.GenerationConfig{
			Temperature: &temperature, TopP: &topP,
			MaxTokens: &maxTokens, ThinkingEnabled: &thinking,
		},
		Tools: map[string]tool.Tool{"lookup": declaredTool{&tool.Declaration{
			Name: "lookup", Description: "Find", InputSchema: &tool.Schema{
				Type: "object", Required: []string{"q"}, Properties: map[string]*tool.Schema{"q": {Type: "string"}},
			},
		}}},
		ExtraFields: map[string]any{"parallel_tool_calls": false, "tool_choice": "required"},
	}
	stream, err := NewModel(f).GenerateContent(context.Background(), req)
	require.NoError(t, err)
	response := <-stream
	require.True(t, response.Done)
	require.Equal(t, "reason", response.Choices[0].Message.ReasoningContent)
	require.Equal(t, "signed", response.Choices[0].Message.ToolCalls[0].ExtraFields["thought_signature"])
	require.Equal(t, 11, response.Usage.TotalTokens)
	require.Equal(t, 2, response.Usage.PromptTokensDetails.CacheCreationTokens)
	require.Equal(t, 0.3, f.opts.Temperature)
	require.Equal(t, 500, f.opts.MaxTokens)
	require.Equal(t, "required", f.opts.ToolChoice)
	require.False(t, *f.opts.ParallelToolCalls)
	require.JSONEq(t, `{"type":"object","required":["q"],"properties":{"q":{"type":"string"}}}`,
		string(f.opts.Tools[0].Function.Parameters))
	require.Equal(t, "data:image/png;base64,AQID", f.messages[0].MultiContent[1].ImageURL.URL)
	require.Equal(t, "old", f.messages[1].ToolCallID)
	require.Equal(t, "lookup", f.messages[1].Name)
}

func TestModelStreamOnlyPublishesCompleteToolPlan(t *testing.T) {
	for _, finalSnapshot := range []bool{false, true} {
		t.Run(map[bool]string{false: "fragments", true: "snapshot"}[finalSnapshot], func(t *testing.T) {
			f := &modelChatFake{chunks: []types.StreamResponse{
				{ResponseType: types.ResponseTypeThinking, Content: "reason"},
				{ResponseType: types.ResponseTypeAnswer, Content: "answer"},
				{ToolCalls: []types.LLMToolCall{{
					ID: "c1", Type: "function",
					Function: types.FunctionCall{Name: "lookup", Arguments: `{"q":`},
				}}},
				{ToolCalls: []types.LLMToolCall{{ID: "c1", Function: types.FunctionCall{Arguments: `"x"}`}}}},
			}}
			last := types.StreamResponse{
				Done: true, FinishReason: "tool_calls",
				Usage: &types.TokenUsage{TotalTokens: 10},
			}
			if finalSnapshot {
				last.ToolCalls = []types.LLMToolCall{{
					ID: "c1", Type: "function",
					Function: types.FunctionCall{Name: "lookup", Arguments: `{"q":"x"}`},
				}}
			}
			f.chunks = append(f.chunks, last)
			var recorded ModelAttempt
			ctx := WithModelAttempt(context.Background(), "attempt-1", func(a ModelAttempt) error {
				recorded = a
				return nil
			})
			req := &model.Request{GenerationConfig: model.GenerationConfig{Stream: true}}
			stream, err := NewModel(f).GenerateContent(ctx, req)
			require.NoError(t, err)
			var complete *model.Response
			var reasoning string
			for response := range stream {
				require.Equal(t, "attempt-1", response.ID)
				if response.IsPartial {
					require.Empty(t, response.Choices[0].Delta.ToolCalls)
					reasoning += response.Choices[0].Delta.ReasoningContent
				} else {
					complete = response
				}
			}
			require.NotNil(t, complete)
			require.Nil(t, complete.Error)
			require.Equal(t, "reason", reasoning)
			require.Equal(t, "answer", complete.Choices[0].Message.Content)
			require.JSONEq(t, `{"q":"x"}`, string(complete.Choices[0].Message.ToolCalls[0].Function.Arguments))
			require.Equal(t, "attempt-1", recorded.Response.ID)
			require.Equal(t, 10, recorded.Usage.TotalTokens)
		})
	}
}

func TestModelErrorsCannotCreateSuccessfulPlans(t *testing.T) {
	for _, chunks := range [][]types.StreamResponse{
		{{Content: "partial"}},
		{{ResponseType: types.ResponseTypeError, Content: "provider failed", Done: true}},
		{{Done: true, FinishReason: types.FinishReasonIncomplete}},
		{{Done: true, FinishReason: "tool_calls", ToolCalls: []types.LLMToolCall{{
			ID:       "c1",
			Function: types.FunctionCall{Name: "bad", Arguments: `{"x":`},
		}}}},
	} {
		f := &modelChatFake{chunks: chunks}
		req := &model.Request{GenerationConfig: model.GenerationConfig{Stream: true}}
		stream, err := NewModel(f).GenerateContent(context.Background(), req)
		require.NoError(t, err)
		var last *model.Response
		for response := range stream {
			last = response
		}
		require.NotNil(t, last)
		require.NotNil(t, last.Error)
		require.Empty(t, last.Choices)
	}
	f := &modelChatFake{err: errors.New("provider failed")}
	_, err := NewModel(f).GenerateContent(context.Background(), &model.Request{})
	require.ErrorContains(t, err, "provider failed")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = NewModel(f).GenerateContent(ctx, &model.Request{})
	require.ErrorIs(t, err, context.Canceled)
}
