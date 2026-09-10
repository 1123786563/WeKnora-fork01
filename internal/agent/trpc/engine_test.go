package trpc

import (
	"context"
	"testing"

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
