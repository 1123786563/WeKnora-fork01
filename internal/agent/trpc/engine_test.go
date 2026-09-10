package trpc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

type engineModel struct{}

func (*engineModel) Info() model.Info { return model.Info{Name: "engine-test"} }
func (*engineModel) GenerateContent(context.Context, *model.Request) (<-chan *model.Response, error) {
	return nil, context.Canceled
}

func TestNewGraphRunnerValidatesBindings(t *testing.T) {
	_, err := NewGraphRunner(GraphBindings{Model: &engineModel{}, Store: nil})
	require.Error(t, err)
}

func TestGraphRunnerRejectsInvalidFence(t *testing.T) {
	r := &GraphRunner{}
	err := r.Run(context.Background(), agentruntime.Fence{})
	require.Error(t, err)
}

func TestGraphRunnerDeepCopiesInitialState(t *testing.T) {
	in := State{Version: StateVersion, PendingCallIDs: []string{"c1"}, AppliedCallIDs: map[string]bool{"c1": true}, CompactionState: json.RawMessage(`{"v":1}`), UsageAttempts: map[string]json.RawMessage{"m": json.RawMessage(`{"v":1}`)}}
	r, err := NewGraphRunner(GraphBindings{Model: &engineModel{}, Store: &minimalStore{}, InitialState: in})
	require.NoError(t, err)
	in.PendingCallIDs[0] = "mutated"
	in.AppliedCallIDs["c1"] = false
	in.CompactionState[0] = 'x'
	require.Equal(t, "c1", r.bindings.InitialState.PendingCallIDs[0])
	require.True(t, r.bindings.InitialState.AppliedCallIDs["c1"])
	require.Equal(t, byte('{'), r.bindings.InitialState.CompactionState[0])
}

type minimalStore struct{}

func (*minimalStore) Admit(context.Context, agentruntime.Admission) (agentruntime.Run, error) {
	return agentruntime.Run{}, agentruntime.ErrNotFound
}
func (*minimalStore) Get(context.Context, agentruntime.RunKey) (agentruntime.Run, error) {
	return agentruntime.Run{}, agentruntime.ErrNotFound
}
func (*minimalStore) Claim(context.Context, agentruntime.RunKey, string, time.Duration) (agentruntime.Fence, error) {
	return agentruntime.Fence{}, agentruntime.ErrNotFound
}
func (*minimalStore) Renew(context.Context, agentruntime.Fence, time.Duration) error { return nil }
func (*minimalStore) Scan(context.Context, int) ([]agentruntime.RunKey, error)       { return nil, nil }
func (*minimalStore) SaveCheckpoint(context.Context, agentruntime.Fence, agentruntime.CheckpointRecord) error {
	return nil
}
func (*minimalStore) LoadCheckpoint(context.Context, agentruntime.RunKey) (agentruntime.CheckpointRecord, error) {
	return agentruntime.CheckpointRecord{}, agentruntime.ErrNotFound
}

func TestGraphRunnerDeepCopiesNestedMessageAliases(t *testing.T) {
	idx := 3
	arg := []byte(`{"x":1}`)
	extra := map[string]any{"nested": []any{"a"}}
	text := "hello"
	in := State{Version: StateVersion, Messages: []model.Message{{ToolCalls: []model.ToolCall{{Index: &idx, ExtraFields: extra, Function: model.FunctionDefinitionParam{Arguments: arg}}}, ContentParts: []model.ContentPart{{Text: &text}}}}}
	r, err := NewGraphRunner(GraphBindings{Model: &engineModel{}, Store: &minimalStore{}, InitialState: in})
	require.NoError(t, err)
	idx = 9
	arg[0] = 'x'
	extra["nested"].([]any)[0] = "changed"
	text = "changed"
	got := r.bindings.InitialState.Messages[0]
	require.Equal(t, 3, *got.ToolCalls[0].Index)
	require.Equal(t, byte('{'), got.ToolCalls[0].Function.Arguments[0])
	require.Equal(t, "a", got.ToolCalls[0].ExtraFields["nested"].([]any)[0])
	require.Equal(t, "hello", *got.ContentParts[0].Text)
}
