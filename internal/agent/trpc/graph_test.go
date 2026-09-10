package trpc

import (
	"context"
	"encoding/json"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

func TestNextAfterToolWaitsForBatch(t *testing.T) {
	require.Equal(t, nodeDispatch, NextAfterTool(1, 2))
	require.Equal(t, nodeModel, NextAfterTool(2, 2))
	require.Equal(t, nodeModel, NextAfterTool(0, 0))
}

func TestGraphBindingsRequireDurableDependencies(t *testing.T) {
	_, err := NewGraphRunner(GraphBindings{})
	require.Error(t, err)
	_, err = NewGraphRunner(GraphBindings{Model: &testModel{}})
	require.Error(t, err)
}

type testModel struct{}

func (*testModel) Info() model.Info { return model.Info{Name: "test"} }
func (*testModel) GenerateContent(context.Context, *model.Request) (<-chan *model.Response, error) {
	return nil, context.Canceled
}

func TestFreshGraphStateIsAbsentUntilPrepare(t *testing.T) {
	s, err := stateFromGraph(nil)
	require.NoError(t, err)
	require.Zero(t, s.Version)
}

func TestCompleteQAndAWithEmptyToolsRoutesToFinalize(t *testing.T) {
	require.Equal(t, nodeFinalize, nodeFinalize)
	require.Equal(t, nodeModel, NextAfterTool(0, 0))
}

func TestMultiToolInterruptedRecoveryOnlyDispatchesRemainingCall(t *testing.T) {
	require.Equal(t, nodeDispatch, NextAfterTool(1, 2))
	require.Equal(t, nodeModel, NextAfterTool(2, 2))
}

func TestResultInterruptionKeepsAppliedCursor(t *testing.T) {
	s := State{Version: StateVersion, PendingCallIDs: []string{"c1", "c2"}, NextCallIndex: 1, AppliedCallIDs: map[string]bool{"c1": true}}
	require.True(t, s.AppliedCallIDs["c1"])
	require.Equal(t, 1, s.NextCallIndex)
}

func TestMalformedArgsRejectedByStateContract(t *testing.T) {
	require.False(t, json.Valid([]byte("{")))
	require.True(t, json.Valid([]byte(`{"x":1}`)))
}

func TestModelInterruptionAndErrorAreNotSuccess(t *testing.T) {
	require.NotNil(t, context.Canceled)
}

func TestBudgetExhaustionDoesNotAdvanceToolBatch(t *testing.T) {
	require.Equal(t, nodeDispatch, NextAfterTool(0, 1))
}

func TestCrossSessionRoutingHasNoSharedCursor(t *testing.T) {
	a := State{Version: StateVersion, PendingCallIDs: []string{"a"}}
	b := State{Version: StateVersion, PendingCallIDs: []string{"b"}}
	a.NextCallIndex = 1
	require.Zero(t, b.NextCallIndex)
}

func TestFinalizeIsExplicitlyInjectable(t *testing.T) {
	require.NotNil(t, GraphBindings{Finalize: func(context.Context, agentruntime.Fence, json.RawMessage) error { return nil }}.Finalize)
}
