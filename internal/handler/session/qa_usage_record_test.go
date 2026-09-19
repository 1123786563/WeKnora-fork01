package session

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// stubUsageRecorder captures RecordChatTurn calls from the completion hook.
type stubUsageRecorder struct {
	calls []stubUsageRecordCall
	err   error
}

type stubUsageRecordCall struct {
	tenantID uint64
	userID   string
	model    string
	usage    *types.TokenUsage
}

func (s *stubUsageRecorder) RecordChatTurn(_ context.Context, tenantID uint64, userID, model string, usage *types.TokenUsage) error {
	if s.err != nil {
		return s.err
	}
	s.calls = append(s.calls, stubUsageRecordCall{tenantID: tenantID, userID: userID, model: model, usage: usage})
	return nil
}

// stubMessageServiceForCompletion covers exactly what completeAssistantMessage
// touches: the synchronous UpdateMessage plus the fire-and-forget KB indexing.
type stubMessageServiceForCompletion struct {
	interfaces.MessageService
	updated []*types.Message
}

func (s *stubMessageServiceForCompletion) UpdateMessage(_ context.Context, m *types.Message) error {
	s.updated = append(s.updated, m)
	return nil
}

func (s *stubMessageServiceForCompletion) IndexMessageToKB(context.Context, string, string, string, string) {}

func newCompletionHandlerForUsage(recorder interfaces.UsageRecorderService) (*Handler, *stubMessageServiceForCompletion) {
	msgs := &stubMessageServiceForCompletion{}
	return &Handler{messageService: msgs, usageRecorder: recorder}, msgs
}

func TestCompleteAssistantMessageRecordsChatUsage(t *testing.T) {
	rec := &stubUsageRecorder{}
	h, msgs := newCompletionHandlerForUsage(rec)
	assistant := &types.Message{
		ID:        "am-1",
		SessionID: "sess-1",
		ModelID:   "gpt-x",
		Usage:     &types.TokenUsage{PromptTokens: 11, CompletionTokens: 7},
	}

	h.completeAssistantMessage(context.Background(), assistant, "hello", "um-1", 9, "owner-1")

	if len(rec.calls) != 1 {
		t.Fatalf("RecordChatTurn calls = %d, want 1", len(rec.calls))
	}
	got := rec.calls[0]
	if got.tenantID != 9 || got.userID != "owner-1" || got.model != "gpt-x" || got.usage != assistant.Usage {
		t.Errorf("recorded call = %+v, want tenant 9 user owner-1 model gpt-x same usage ptr", got)
	}
	if !assistant.IsCompleted {
		t.Error("assistant message not marked completed")
	}
	if len(msgs.updated) != 1 {
		t.Errorf("UpdateMessage calls = %d, want 1", len(msgs.updated))
	}

	// A second invocation on the now-completed message must not double-count.
	h.completeAssistantMessage(context.Background(), assistant, "hello", "um-1", 9, "owner-1")
	if len(rec.calls) != 1 {
		t.Errorf("RecordChatTurn calls after repeat completion = %d, want still 1", len(rec.calls))
	}
}

func TestCompleteAssistantMessageUsageSkips(t *testing.T) {
	cases := []struct {
		name      string
		recorder  interfaces.UsageRecorderService
		assistant *types.Message
	}{
		{
			name:      "nil usage block",
			recorder:  &stubUsageRecorder{},
			assistant: &types.Message{ModelID: "gpt-x"},
		},
		{
			name:      "already completed",
			recorder:  &stubUsageRecorder{},
			assistant: &types.Message{ModelID: "gpt-x", IsCompleted: true, Usage: &types.TokenUsage{PromptTokens: 1}},
		},
		{
			name:      "nil recorder (tests construct Handler bare)",
			recorder:  nil,
			assistant: &types.Message{ModelID: "gpt-x", Usage: &types.TokenUsage{PromptTokens: 1}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, msgs := newCompletionHandlerForUsage(tc.recorder)
			h.completeAssistantMessage(context.Background(), tc.assistant, "", "", 1, "u")
			if !tc.assistant.IsCompleted {
				t.Error("message must still complete")
			}
			if len(msgs.updated) != 1 {
				t.Errorf("UpdateMessage calls = %d, want 1", len(msgs.updated))
			}
			if rec, ok := tc.recorder.(*stubUsageRecorder); ok && len(rec.calls) != 0 {
				t.Errorf("RecordChatTurn calls = %d, want 0", len(rec.calls))
			}
		})
	}
}

func TestCompleteAssistantMessageUsageErrorDoesNotBlock(t *testing.T) {
	rec := &stubUsageRecorder{err: context.Canceled}
	h, msgs := newCompletionHandlerForUsage(rec)
	assistant := &types.Message{
		ID: "am-2", SessionID: "s", ModelID: "m",
		Usage: &types.TokenUsage{PromptTokens: 3},
	}
	h.completeAssistantMessage(context.Background(), assistant, "q", "u-1", 1, "owner")

	if !assistant.IsCompleted {
		t.Error("recording failure must not block message completion")
	}
	if len(msgs.updated) != 1 {
		t.Errorf("UpdateMessage calls = %d, want 1", len(msgs.updated))
	}
}
