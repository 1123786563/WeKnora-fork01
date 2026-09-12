package trpc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session/noop"
)

type fakeInputSource struct{ inputs []agentruntime.RunInput }

func (f *fakeInputSource) ListPendingInputs(
	context.Context, agentruntime.RunKey, string,
) ([]agentruntime.RunInput, error) {
	return f.inputs, nil
}

type recordingEventSink struct{ types []string }

func (r *recordingEventSink) AppendEvent(
	_ context.Context, _ agentruntime.Fence, evt agentruntime.RunEvent,
) (agentruntime.RunEvent, error) {
	r.types = append(r.types, evt.Type)
	return evt, nil
}

type steerCaptureModel struct {
	messages [][]model.Message
}

func (m *steerCaptureModel) Info() model.Info { return model.Info{Name: "steer-test"} }

func (m *steerCaptureModel) GenerateContent(_ context.Context, req *model.Request) (<-chan *model.Response, error) {
	m.messages = append(m.messages, append([]model.Message(nil), req.Messages...))
	finish := "stop"
	out := make(chan *model.Response, 1)
	out <- &model.Response{ID: "m1", Done: true, Choices: []model.Choice{{
		Message:      model.Message{Role: model.RoleAssistant, Content: "done"},
		FinishReason: &finish,
	}}}
	close(out)
	return out, nil
}

func TestGraphInjectsPendingSteerInputBeforeModel(t *testing.T) {
	store := &graphTestStore{
		fence: agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 1, RunID: "r1"}, Owner: "w", Epoch: 1},
		run: agentruntime.Run{
			Key: agentruntime.RunKey{TenantID: 1, RunID: "r1"}, Owner: "w", Epoch: 1,
			Status: "running", LeaseUntil: time.Now().Add(time.Minute),
		},
	}
	inputs := &fakeInputSource{inputs: []agentruntime.RunInput{
		{
			SteerID: "steer-1", Mode: "inject",
			Message: json.RawMessage(`{"role":"user","content":"use this detail"}`),
		},
	}}
	mdl := &steerCaptureModel{}
	sink := &recordingEventSink{}
	finalize := func(context.Context, agentruntime.Fence, json.RawMessage) error { return nil }
	g, err := buildGraph(GraphBindings{
		Model: mdl, Store: store, Inputs: inputs, Events: sink,
		Finalize: finalize,
		InitialState: State{
			Version:  StateVersion,
			Messages: []model.Message{model.NewUserMessage("start")},
		},
	})
	require.NoError(t, err)
	ex, err := graph.NewExecutor(g)
	require.NoError(t, err)
	inv := &agent.Invocation{
		AgentName: "steer-test", InvocationID: "i1",
		SessionService: noop.NewService(), Message: model.NewUserMessage("start"),
	}
	runctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	execState := graph.State{StateKey: State{
		Version:  StateVersion,
		Messages: []model.Message{model.NewUserMessage("start")},
	}}
	events, err := ex.Execute(withFence(runctx, store.fence), execState, inv)
	require.NoError(t, err)
	drainEvents(events)
	require.Equal(t, []string{"steer_injected"}, sink.types)
	require.Len(t, mdl.messages, 1)
	injected := 0
	for _, msg := range mdl.messages[0] {
		if msg.Role == model.RoleUser && msg.Content == "use this detail" {
			injected++
		}
	}
	require.Equal(t, 1, injected, "model round: %v", mdl.messages[0])
}

func TestGraphSkipsAlreadyAppliedSteerInput(t *testing.T) {
	store := &graphTestStore{
		fence: agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 1, RunID: "r1"}, Owner: "w", Epoch: 1},
		run: agentruntime.Run{
			Key: agentruntime.RunKey{TenantID: 1, RunID: "r1"}, Owner: "w", Epoch: 1,
			Status: "running", LeaseUntil: time.Now().Add(time.Minute),
		},
	}
	inputs := &fakeInputSource{inputs: []agentruntime.RunInput{
		{
			SteerID: "steer-1", Mode: "inject",
			Message: json.RawMessage(`{"role":"user","content":"use this detail"}`),
		},
	}}
	mdl := &steerCaptureModel{}
	sink := &recordingEventSink{}
	finalizeNoop := func(context.Context, agentruntime.Fence, json.RawMessage) error { return nil }
	g, err := buildGraph(GraphBindings{
		Model: mdl, Store: store, Inputs: inputs, Events: sink,
		Finalize: finalizeNoop,
		InitialState: State{
			Version:         StateVersion,
			Messages:        []model.Message{model.NewUserMessage("start")},
			AppliedSteerIDs: []string{"steer-1"},
		},
	})
	require.NoError(t, err)
	ex, err := graph.NewExecutor(g)
	require.NoError(t, err)
	inv := &agent.Invocation{
		AgentName: "steer-test", InvocationID: "i1",
		SessionService: noop.NewService(), Message: model.NewUserMessage("start"),
	}
	runctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	state := graph.State{StateKey: State{
		Version:         StateVersion,
		Messages:        []model.Message{model.NewUserMessage("start")},
		AppliedSteerIDs: []string{"steer-1"},
	}}
	events, err := ex.Execute(withFence(runctx, store.fence), state, inv)
	require.NoError(t, err)
	drainEvents(events)
	require.Empty(t, sink.types, "already applied input must not be re-injected")
	require.Len(t, mdl.messages, 1)
	for _, msg := range mdl.messages[0] {
		if msg.Role == model.RoleUser && msg.Content == "use this detail" {
			t.Fatal("already applied input re-entered the model context")
		}
	}
}

// drainEvents consumes the executor event channel to completion.
func drainEvents(events <-chan *event.Event) {
	for evt := range events {
		_ = evt
	}
}

var _ = types.ToolResult{} // keep the types import for future assertions
