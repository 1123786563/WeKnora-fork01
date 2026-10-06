package session

// issue #3865：快速问答（KnowledgeQA）complete 事件的 usage 断言。模型流的
// usage 经 AgentFinalAnswerData.Usage 到达 qa.go 的收尾闭包（foldQuickAnswerUsage
// 折进本回答累计口径，与智能推理的 TurnUsage 一致），再随 EventAgentComplete
// 交给 handleComplete —— 后者把它同时放进 StreamEvent.Usage（顶层 usage，经
// buildStreamResponse 提升）与 completeData["usage"]（data.usage）。

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func newQuickAnswerUsageEnv(t *testing.T) (*Handler, *completionEventRecorder, *sseStreamContext, *types.Message) {
	t.Helper()
	messages := &imageCompletionMessages{}
	stream := &completionEventRecorder{}
	h := &Handler{messageService: messages, streamManager: stream}
	bus := event.NewEventBus()
	message := &types.Message{
		ID: "m", SessionID: "s", Role: "assistant", Content: "answer", AgentTenantID: 2,
	}
	ctx := types.WithExecutionTenant(context.Background(), 1)
	streamHandler := h.setupStreamHandler(ctx, "s", "m", "req", 1, time.Now(), message, bus)
	streamCtx := &sseStreamContext{
		eventBus: bus, streamHandler: streamHandler, assistantMessage: message,
	}
	return h, stream, streamCtx, message
}

// a) 模型流带 usage（prompt=10/completion=5/total=15）→ complete 事件顶层
// usage 与 data.usage 均为该值，助手消息同样携带（持久化与 SP12 记账口径）。
func TestQuickAnswerCompleteEventCarriesUsage(t *testing.T) {
	h, stream, streamCtx, message := newQuickAnswerUsageEnv(t)

	// The production final-answer handler folds each streamed usage before
	// completion; invoke the same fold on the turn's stream context. The fold
	// runs the shared Accumulate, which classifies an unreported cache status
	// exactly like smart reasoning's TurnUsage does.
	foldQuickAnswerUsage(streamCtx, &types.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15})
	want := &types.TokenUsage{
		PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15,
		CacheStatus: types.PromptCacheStatusUnreported,
	}

	h.completeQuickAnswerTurn(types.WithExecutionTenant(context.Background(), 1), streamCtx, "", "")

	require.True(t, message.IsCompleted)
	require.Equal(t, want, message.Usage, "assistant message carries the folded usage")

	events := stream.events
	require.NotEmpty(t, events)
	last := events[len(events)-1]
	require.Equal(t, types.ResponseTypeComplete, last.Type)
	require.True(t, last.Done)
	require.Equal(t, want, last.Usage, "complete event top-level usage")
	require.Equal(t, want, last.Data["usage"], "complete event data.usage")

	payload, err := json.Marshal(buildStreamResponse(last, "req"))
	require.NoError(t, err)
	require.Contains(t, string(payload), `"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15`)
}

// b) 模型流无 usage → 不报错、两处字段缺省（顶层与 data 均不出现 usage）。
func TestQuickAnswerCompleteEventWithoutUsageOmitsField(t *testing.T) {
	h, stream, streamCtx, message := newQuickAnswerUsageEnv(t)

	h.completeQuickAnswerTurn(types.WithExecutionTenant(context.Background(), 1), streamCtx, "", "")

	require.True(t, message.IsCompleted)
	require.Nil(t, message.Usage)

	last := stream.events[len(stream.events)-1]
	require.Equal(t, types.ResponseTypeComplete, last.Type)
	require.Nil(t, last.Usage)
	require.NotContains(t, last.Data, "usage")

	payload, err := json.Marshal(buildStreamResponse(last, "req"))
	require.NoError(t, err)
	require.NotContains(t, string(payload), `"usage"`)
}

// 聚合口径照抄智能推理：多次模型调用（如主答案后又有回退流）按 TokenUsage.
// Accumulate 累计；nil usage 保持缺省。
func TestFoldQuickAnswerUsageAccumulatesAcrossStreams(t *testing.T) {
	_, _, streamCtx, message := newQuickAnswerUsageEnv(t)

	foldQuickAnswerUsage(streamCtx, &types.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15})
	foldQuickAnswerUsage(streamCtx, &types.TokenUsage{PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10})
	// CacheStatus follows the shared Accumulate semantics (unreported folds to
	// "unreported"), the same aggregation smart reasoning uses.
	require.Equal(t, &types.TokenUsage{
		PromptTokens: 17, CompletionTokens: 8, TotalTokens: 25,
		CacheStatus: types.PromptCacheStatusUnreported,
	}, message.Usage)

	foldQuickAnswerUsage(streamCtx, nil)
	require.Equal(t, &types.TokenUsage{
		PromptTokens: 17, CompletionTokens: 8, TotalTokens: 25,
		CacheStatus: types.PromptCacheStatusUnreported,
	}, message.Usage, "nil usage must not reset or mutate the total")

	foldQuickAnswerUsage(nil, &types.TokenUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2})
	require.NotNil(t, message.Usage)
}
