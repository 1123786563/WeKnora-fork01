package interfaces

import (
	"context"
	"github.com/Tencent/WeKnora/internal/types"
)

type SemanticModelCapabilityIssuer interface {
	Issue(context.Context, string, string) (types.SemanticModelIssuedCapability, error)
	Verify(context.Context, string) (types.SemanticModelCapability, error)
}
type SemanticModelInvocationStore interface {
	EnsureRun(context.Context, types.SemanticModelCapability) error
	Claim(context.Context, types.SemanticModelCapability, string) (types.SemanticModelInvocationClaim, error)
	MarkDispatched(context.Context, types.SemanticModelCapability) error
	Complete(context.Context, types.SemanticModelCapability, types.SemanticModelInvocationResult) error
	FailBeforeDispatch(context.Context, types.SemanticModelCapability) error
	MarkUnknown(context.Context, types.SemanticModelCapability) error
}
