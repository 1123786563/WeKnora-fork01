package session

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/stream"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// AC3（Go 面）：evidence 事件经真实 EventBus → 真实 AgentStreamHandler 订阅 →
// 真实 MemoryStreamManager → 真实 buildStreamResponse 序列化为 SSE 帧，不经任何 mock
// 断言中间结构——消费者看到的就是 wire 字节形状。
func TestEvidenceEventStreamsAsEvidenceResponseFrame(t *testing.T) {
	bus := event.NewEventBus()
	streams := stream.NewMemoryStreamManager()
	assistant := &types.Message{ID: "am-1", SessionID: "sess-1"}
	handler := NewAgentStreamHandler(context.Background(), "sess-1", "am-1", "req-1", 1, time.Now(), assistant, streams, bus, nil, nil, nil)
	handler.Subscribe()

	evidence := types.AnswerEvidence{
		State:             types.EvidenceStateCited,
		SemanticGraphUsed: false,
		RetrievedAt:       "2026-09-24T08:00:00Z",
		Citations: []types.EvidenceCitation{{
			CitationID: "chunk-1", KnowledgeID: "doc-1", KnowledgeBaseID: "kb-own",
			Revision: 3, Kind: types.EvidenceKindFact, RetrievedAt: "2026-09-24T08:00:00Z",
		}},
		Conclusions: []types.EvidenceConclusion{{
			Kind: types.EvidenceKindModelInferred, ModelID: "chat-model-1", CitationIDs: []string{"chunk-1"},
		}},
		Reasoning: types.EvidenceReasoning{State: types.EvidenceReasoningNotRequested},
	}
	require.NoError(t, types.ValidateAnswerEvidence(evidence))

	require.NoError(t, bus.Emit(context.Background(), event.Event{
		ID:        "ev-1",
		Type:      event.EventAgentEvidence,
		SessionID: "sess-1",
		Data:      event.AgentEvidenceData{Evidence: evidence},
	}))

	events, _, err := streams.GetEvents(context.Background(), "sess-1", "am-1", 0)
	require.NoError(t, err)
	require.Len(t, events, 1, "evidence 事件必须恰好流化为一帧")
	require.Equal(t, types.ResponseTypeEvidence, events[0].Type)

	response := buildStreamResponse(events[0], "req-1")
	require.Equal(t, types.ResponseTypeEvidence, response.ResponseType)
	payload, err := json.Marshal(response)
	require.NoError(t, err)
	require.Contains(t, string(payload), `"response_type":"evidence"`)
	require.Contains(t, string(payload), `"state":"cited"`)
	require.Contains(t, string(payload), `"kind":"fact"`)
	require.Contains(t, string(payload), `"revision":3`)
	require.Contains(t, string(payload), `"kind":"model_inferred"`)
	require.Contains(t, string(payload), `"semantic_graph_used":false`)
	require.NotContains(t, string(payload), `"knowledge_references"`, "evidence 帧不得混入旧引用形状")
}
