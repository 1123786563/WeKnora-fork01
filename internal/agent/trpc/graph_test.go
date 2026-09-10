package trpc

import (
	"context"
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
