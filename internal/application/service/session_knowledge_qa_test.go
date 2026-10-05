package service

import (
	"context"
	"sync"
	"testing"
	"time"

	chatpipeline "github.com/Tencent/WeKnora/internal/modules/conversation/chat_pipeline"

	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/modelcontext"
	"github.com/Tencent/WeKnora/internal/modules/airesource/models/asr"
	"github.com/Tencent/WeKnora/internal/modules/airesource/models/chat"
	"github.com/Tencent/WeKnora/internal/modules/airesource/models/embedding"
	"github.com/Tencent/WeKnora/internal/modules/airesource/models/rerank"
	"github.com/Tencent/WeKnora/internal/modules/airesource/models/vlm"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type captureChatModel struct {
	lastMessages []chat.Message
}

func (m *captureChatModel) Chat(
	context.Context,
	[]chat.Message,
	*chat.ChatOptions,
) (*types.ChatResponse, error) {
	return nil, nil
}

func (m *captureChatModel) ChatStream(
	_ context.Context,
	messages []chat.Message,
	_ *chat.ChatOptions,
) (<-chan types.StreamResponse, error) {
	m.lastMessages = append([]chat.Message(nil), messages...)

	ch := make(chan types.StreamResponse, 1)
	ch <- types.StreamResponse{
		ResponseType: types.ResponseTypeAnswer,
		Content:      "ok",
		Done:         true,
	}
	close(ch)
	return ch, nil
}

func (m *captureChatModel) GetModelName() string { return "capture" }
func (m *captureChatModel) GetModelID() string   { return "capture" }

type stubModelService struct {
	chatModel       chat.Chat
	modelsByID      map[string]*types.Model
	availableModels []*types.Model
}

func TestEmitKnowledgeReferencesEventIgnoresCitationOutputSetting(t *testing.T) {
	bus := event.NewEventBus()
	var emitted []event.Event
	bus.On(event.EventAgentReferences, func(_ context.Context, evt event.Event) error {
		emitted = append(emitted, evt)
		return nil
	})
	disabled := false
	result := &types.SearchResult{ID: "chunk-1", KnowledgeTitle: "Doc"}
	cm := &types.ChatManage{
		PipelineRequest: types.PipelineRequest{CitationEnabled: &disabled},
		PipelineState: types.PipelineState{
			MergeResult: []*types.SearchResult{result},
		},
		PipelineContext: types.PipelineContext{EventBus: bus.AsEventBusInterface()},
	}

	emitKnowledgeReferencesEvent(context.Background(), cm)
	require.Len(t, emitted, 1)
	require.Equal(t, event.EventAgentReferences, emitted[0].Type)
	require.Equal(t, []*types.SearchResult{result}, emitted[0].Data.(event.AgentReferencesData).References)

	enabled := true
	cm.CitationEnabled = &enabled
	emitKnowledgeReferencesEvent(context.Background(), cm)
	require.Len(t, emitted, 2)
}

func (s *stubModelService) CreateModel(context.Context, *types.Model) error {
	return nil
}

func (s *stubModelService) CopyModel(context.Context, string, string) (*types.Model, error) {
	return nil, nil
}

func (s *stubModelService) GetModelByID(_ context.Context, id string) (*types.Model, error) {
	return s.modelsByID[id], nil
}

func (s *stubModelService) ListModels(context.Context) ([]*types.Model, error) {
	return s.availableModels, nil
}

func (s *stubModelService) UpdateModel(context.Context, *types.Model) error {
	return nil
}

func (s *stubModelService) DeleteModel(context.Context, string) error {
	return nil
}

func (s *stubModelService) UpdateModelCredentials(
	context.Context, string, *string, *string,
) (*types.Model, error) {
	return nil, nil
}

func (s *stubModelService) ClearModelCredential(context.Context, string, string) error {
	return nil
}

func (s *stubModelService) GetEmbeddingModel(context.Context, string) (embedding.Embedder, error) {
	return nil, nil
}

func (s *stubModelService) GetEmbeddingModelForTenant(context.Context, string, uint64) (embedding.Embedder, error) {
	return nil, nil
}

func (s *stubModelService) GetRerankModel(context.Context, string) (rerank.Reranker, error) {
	return nil, nil
}

func (s *stubModelService) GetChatModel(context.Context, string) (chat.Chat, error) {
	return s.chatModel, nil
}

func (s *stubModelService) GetVLMModel(context.Context, string) (vlm.VLM, error) {
	return nil, nil
}

func (s *stubModelService) GetASRModel(context.Context, string) (asr.ASR, error) {
	return nil, nil
}

func TestHandleModelFallback_IncludesHistoryMessages(t *testing.T) {
	chatModel := &captureChatModel{}
	svc := &sessionService{
		modelService: &stubModelService{chatModel: chatModel},
	}

	bus := event.NewEventBus()
	cm := &types.ChatManage{
		PipelineRequest: types.PipelineRequest{
			SessionID:      "session-1",
			Query:          "现在还能继续讲吗？",
			ChatModelID:    "chat-model",
			FallbackPrompt: "Answer the latest user question: {{query}}",
			SummaryConfig: types.SummaryConfig{
				Temperature: 0.2,
			},
			Language: "zh-CN",
		},
		PipelineState: types.PipelineState{
			History: []*types.History{
				{
					Query:  "先介绍一下 WeKnora",
					Answer: "WeKnora 是一个知识库问答系统。",
				},
			},
		},
		PipelineContext: types.PipelineContext{
			EventBus: bus.AsEventBusInterface(),
		},
	}

	svc.handleModelFallback(context.Background(), cm)

	// Corrected fallback shape: a system message carries the fallback
	// instruction, history is replayed in the middle, and the turn ends on the
	// user's question. Previously the system message was dropped entirely.
	require.Len(t, chatModel.lastMessages, 4)
	assert.Equal(t, "system", chatModel.lastMessages[0].Role)
	assert.Contains(t, chatModel.lastMessages[0].Content, "Answer the latest user question")
	assert.Equal(t, "user", chatModel.lastMessages[1].Role)
	assert.Equal(t, "先介绍一下 WeKnora", chatModel.lastMessages[1].Content)
	assert.Equal(t, "assistant", chatModel.lastMessages[2].Role)
	assert.Equal(t, "WeKnora 是一个知识库问答系统。", chatModel.lastMessages[2].Content)
	assert.Equal(t, "user", chatModel.lastMessages[3].Role)
	assert.Contains(t, chatModel.lastMessages[3].Content, "现在还能继续讲吗？")
}

// scriptedStreamChat replays a fixed chunk sequence as the model stream, so
// tests can pin how the QA pipeline forwards the stream's usage (issue #3865).
type scriptedStreamChat struct {
	chunks []types.StreamResponse
}

func (m *scriptedStreamChat) Chat(
	context.Context, []chat.Message, *chat.ChatOptions,
) (*types.ChatResponse, error) {
	return nil, nil
}

func (m *scriptedStreamChat) ChatStream(
	context.Context, []chat.Message, *chat.ChatOptions,
) (<-chan types.StreamResponse, error) {
	ch := make(chan types.StreamResponse, len(m.chunks))
	for _, c := range m.chunks {
		ch <- c
	}
	close(ch)
	return ch, nil
}

func (m *scriptedStreamChat) GetModelName() string { return "scripted" }
func (m *scriptedStreamChat) GetModelID() string   { return "scripted" }

// runQACompletionStream drives the QA completion stage over the given model
// chunks and returns the emitted final-answer events in order.
func runQACompletionStream(t *testing.T, chunks []types.StreamResponse) []event.AgentFinalAnswerData {
	t.Helper()
	bus := event.NewEventBus()
	var mu sync.Mutex
	var answers []event.AgentFinalAnswerData
	bus.On(event.EventAgentFinalAnswer, func(_ context.Context, evt event.Event) error {
		data, ok := evt.Data.(event.AgentFinalAnswerData)
		require.True(t, ok)
		mu.Lock()
		answers = append(answers, data)
		mu.Unlock()
		return nil
	})
	events := chatpipeline.NewEventManager()
	chatpipeline.NewPluginChatCompletionStream(events, &stubModelService{
		chatModel: &scriptedStreamChat{chunks: chunks},
	})
	svc := &sessionService{eventManager: events}
	cm := &types.ChatManage{
		PipelineRequest: types.PipelineRequest{
			Query:       "问一下",
			SessionID:   "sess-usage",
			ChatModelID: "m-1",
		},
		PipelineContext: types.PipelineContext{EventBus: bus.AsEventBusInterface()},
	}
	require.NoError(t, svc.KnowledgeQAByEvent(
		context.Background(), cm, []types.EventType{types.CHAT_COMPLETION_STREAM},
	))
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(answers) > 0 && answers[len(answers)-1].Done
	}, 2*time.Second, 10*time.Millisecond, "stream goroutine must deliver the Done marker")
	mu.Lock()
	defer mu.Unlock()
	return append([]event.AgentFinalAnswerData(nil), answers...)
}

// 模型流带 usage → 最终答案 Done 事件必须携带该 usage（qa.go 转发进 complete 事件）。
func TestKnowledgeQAStreamForwardsModelUsageOnFinalAnswerDone(t *testing.T) {
	answers := runQACompletionStream(t, []types.StreamResponse{
		{ResponseType: types.ResponseTypeAnswer, Content: "Hel"},
		{ResponseType: types.ResponseTypeAnswer, Content: "lo", Done: true,
			Usage: &types.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}},
	})
	require.Len(t, answers, 2)
	require.Nil(t, answers[0].Usage, "usage rides the Done marker only")
	require.Equal(t, &types.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}, answers[1].Usage)
}

// 模型流不返回 usage → Done 事件不带 usage、流程不报错（保持现状行为）。
func TestKnowledgeQAStreamWithoutUsageKeepsFieldAbsent(t *testing.T) {
	answers := runQACompletionStream(t, []types.StreamResponse{
		{ResponseType: types.ResponseTypeAnswer, Content: "ok", Done: true},
	})
	require.Len(t, answers, 1)
	require.Nil(t, answers[0].Usage)
}

// 模型回退流同样透传 usage。
func TestConsumeFallbackStreamForwardsUsage(t *testing.T) {
	bus := event.NewEventBus()
	var answers []event.AgentFinalAnswerData
	bus.On(event.EventAgentFinalAnswer, func(_ context.Context, evt event.Event) error {
		answers = append(answers, evt.Data.(event.AgentFinalAnswerData))
		return nil
	})
	cm := &types.ChatManage{}
	cm.EventBus = bus.AsEventBusInterface()
	ch := make(chan types.StreamResponse, 2)
	ch <- types.StreamResponse{ResponseType: types.ResponseTypeAnswer, Content: "partial "}
	ch <- types.StreamResponse{ResponseType: types.ResponseTypeAnswer, Done: true,
		Usage: &types.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}}
	close(ch)
	(&sessionService{}).consumeFallbackStream(context.Background(), cm, ch, modelcontext.NewRegistry(false))
	require.Len(t, answers, 2)
	require.Nil(t, answers[0].Usage)
	require.Equal(t, &types.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}, answers[1].Usage)
}

// 回退流无 usage → 不报错、字段缺省。
func TestConsumeFallbackStreamWithoutUsageKeepsFieldAbsent(t *testing.T) {
	bus := event.NewEventBus()
	var answers []event.AgentFinalAnswerData
	bus.On(event.EventAgentFinalAnswer, func(_ context.Context, evt event.Event) error {
		answers = append(answers, evt.Data.(event.AgentFinalAnswerData))
		return nil
	})
	cm := &types.ChatManage{}
	cm.EventBus = bus.AsEventBusInterface()
	ch := make(chan types.StreamResponse, 1)
	ch <- types.StreamResponse{ResponseType: types.ResponseTypeAnswer, Content: "ok", Done: true}
	close(ch)
	(&sessionService{}).consumeFallbackStream(context.Background(), cm, ch, modelcontext.NewRegistry(false))
	require.Len(t, answers, 1)
	require.Nil(t, answers[0].Usage)
}
