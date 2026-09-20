package native

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

type nativeTestTool struct{ calls atomic.Int64 }

func (t *nativeTestTool) Declaration() *tool.Declaration            { return &tool.Declaration{Name: "write"} }
func (t *nativeTestTool) Call(context.Context, []byte) (any, error) { t.calls.Add(1); return "ok", nil }

type nativeTestStreamTool struct{ calls atomic.Int64 }

func (t *nativeTestStreamTool) Declaration() *tool.Declaration {
	return &tool.Declaration{Name: "stream-write"}
}
func (t *nativeTestStreamTool) StreamableCall(context.Context, []byte) (*tool.StreamReader, error) {
	t.calls.Add(1)
	return tool.NewStream(1).Reader, nil
}

func TestNativeToolRecoveryActionUsesOnlyConfirmedOrExplicitCapabilities(t *testing.T) {
	require.Equal(t, "reuse", RecoveryAction(nativecontract.RecoveryHold, true))
	require.Equal(t, "query", RecoveryAction(nativecontract.RecoveryQuery, false))
	require.Equal(t, "retry", RecoveryAction(nativecontract.RecoveryIdempotent, false))
	require.Equal(t, "retry", RecoveryAction(nativecontract.RecoveryReadOnly, false))
	require.Equal(t, "hold", RecoveryAction(nativecontract.RecoveryHold, false))
}

func TestNativeToolWrappersRejectRevokedCallableAndStreamableToolsBeforeDispatch(t *testing.T) {
	denied := func(context.Context, []byte) error { return errors.New("permission revoked") }
	callable := &nativeTestTool{}
	_, err := WrapCallableTool(callable, denied).Call(context.Background(), []byte(`{}`))
	require.Error(t, err)
	require.Zero(t, callable.calls.Load())

	streamable := &nativeTestStreamTool{}
	_, err = WrapStreamableTool(streamable, denied).StreamableCall(context.Background(), []byte(`{}`))
	require.Error(t, err)
	require.Zero(t, streamable.calls.Load())
}
