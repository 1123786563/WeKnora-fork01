package native

import (
	"context"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// RecoveryAction chooses the only safe next step from a durable tool policy.
// A confirmed outcome is reused. Unknown non-idempotent effects are held for
// human resolution; this package does not wire any provider SDK or retry path.
func RecoveryAction(policy nativecontract.RecoveryPolicy, confirmed bool) string {
	if confirmed {
		return "reuse"
	}
	switch policy {
	case nativecontract.RecoveryQuery:
		return "query"
	case nativecontract.RecoveryIdempotent, nativecontract.RecoveryReadOnly:
		return "retry"
	default:
		return "hold"
	}
}

// ToolPreflight is the last application-owned validation before a tool can
// reach an observable external action. P3 supplies the durable plan, current
// authorization, budget, and fence checks; the wrapper only enforces their
// ordering for both SDK callable forms.
type ToolPreflight func(context.Context, []byte) error

type guardedCallableTool struct {
	tool.CallableTool
	preflight ToolPreflight
}

func WrapCallableTool(delegate tool.CallableTool, preflight ToolPreflight) tool.CallableTool {
	return &guardedCallableTool{CallableTool: delegate, preflight: preflight}
}

func (t *guardedCallableTool) Call(ctx context.Context, args []byte) (any, error) {
	if err := t.preflight(ctx, args); err != nil {
		return nil, err
	}
	return t.CallableTool.Call(ctx, args)
}

type guardedStreamableTool struct {
	tool.StreamableTool
	preflight ToolPreflight
}

func WrapStreamableTool(delegate tool.StreamableTool, preflight ToolPreflight) tool.StreamableTool {
	return &guardedStreamableTool{StreamableTool: delegate, preflight: preflight}
}

func (t *guardedStreamableTool) StreamableCall(ctx context.Context, args []byte) (*tool.StreamReader, error) {
	if err := t.preflight(ctx, args); err != nil {
		return nil, err
	}
	return t.StreamableTool.StreamableCall(ctx, args)
}
