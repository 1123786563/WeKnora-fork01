package session

import (
	"context"
	"sync"
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
	// Record the attempt before applying the failure so tests can assert how
	// many times the hook tried to account the turn.
	s.calls = append(s.calls, stubUsageRecordCall{tenantID: tenantID, userID: userID, model: model, usage: usage})
	if s.err != nil {
		return s.err
	}
	return nil
}

// stubMessageServiceForCompletion covers exactly what completeAssistantMessage
// touches: the synchronous UpdateMessage plus the fire-and-forget KB indexing.
// Mutex-guarded: the concurrent completion test drives several goroutines
// through the full function body.
type stubMessageServiceForCompletion struct {
	interfaces.MessageService
	mu      sync.Mutex
	updated []*types.Message
}

func (s *stubMessageServiceForCompletion) UpdateMessage(_ context.Context, m *types.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updated = append(s.updated, m)
	return nil
}

func (s *stubMessageServiceForCompletion) IndexMessageToKB(context.Context, string, string, string, string) {}

func (s *stubMessageServiceForCompletion) updatedCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.updated)
}

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
	if got := msgs.updatedCount(); got != 1 {
		t.Errorf("UpdateMessage calls = %d, want 1", got)
	}

	// A second invocation on the same message instance must not double-count:
	// the one-shot usage latch stays claimed for the message's lifetime.
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
			// A prior completion path already won the one-shot gate for this
			// message ID: the loser must skip accounting entirely.
			name:      "usage already claimed for this message",
			recorder:  &stubUsageRecorder{},
			assistant: &types.Message{ID: "pre-claimed", ModelID: "gpt-x", Usage: &types.TokenUsage{PromptTokens: 1}},
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
			if tc.name == "usage already claimed for this message" {
				h.usageRecordOnce.Store(tc.assistant.ID, struct{}{}) // a prior path won the race
			}
			h.completeAssistantMessage(context.Background(), tc.assistant, "", "", 1, "u")
			if !tc.assistant.IsCompleted {
				t.Error("message must still complete")
			}
			if got := msgs.updatedCount(); got != 1 {
				t.Errorf("UpdateMessage calls = %d, want 1", got)
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
	if got := msgs.updatedCount(); got != 1 {
		t.Errorf("UpdateMessage calls = %d, want 1", got)
	}
}

// TestCompleteAssistantMessageConcurrentCompletionRecordsOnce is the
// regression test for the double-count window: the stop watcher, the QA defer,
// and the final-answer event handler can all call completeAssistantMessage on
// the SAME *Message with no lock between them. The one-shot atomic claim must
// let exactly one of them record the turn's usage, no matter how the entries
// interleave.
func TestCompleteAssistantMessageConcurrentCompletionRecordsOnce(t *testing.T) {
	const goroutines = 16
	cases := []struct {
		name string
		err  error // recorder failure mode; nil records normally
	}{
		{name: "recording succeeds"},
		{name: "recording fails", err: context.Canceled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &stubUsageRecorder{err: tc.err}
			h, msgs := newCompletionHandlerForUsage(rec)
			assistant := &types.Message{
				ID: "am-race", SessionID: "sess", ModelID: "m",
				Usage: &types.TokenUsage{PromptTokens: 5, CompletionTokens: 2},
			}

			start := make(chan struct{})
			var wg sync.WaitGroup
			for i := 0; i < goroutines; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start // align every goroutine at the gate, then race in
					h.completeAssistantMessage(context.Background(), assistant, "q", "um-1", 3, "owner")
				}()
			}
			close(start)
			wg.Wait()

			if len(rec.calls) != 1 {
				t.Errorf("RecordChatTurn calls after %d concurrent completions = %d, want exactly 1", goroutines, len(rec.calls))
			}
			if !assistant.IsCompleted {
				t.Error("assistant message not marked completed")
			}
			if got := msgs.updatedCount(); got != goroutines {
				// Every path still completes the message — the latch only
				// guards usage accounting, never message persistence.
				t.Errorf("UpdateMessage calls = %d, want %d", got, goroutines)
			}
		})
	}
}
