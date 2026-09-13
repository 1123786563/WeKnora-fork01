package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// Capability negotiation (C01 rule): if Capabilities() fails or the
// response protocol version is unsupported, callers must treat every mode
// as unavailable and fail closed - a failed negotiation is never a silent
// success, and unavailable modes carry their unavailable_reason.
//
// SemanticClient is the Go business system's contract for the independent
// semantic service. The transport adapter (internal/infrastructure/semantic)
// maps protobuf wire messages to the domain DTOs in internal/types; protobuf
// objects never cross this interface.
//
// Lifecycle note (C02): implementations own a Close method so callers can
// release transport resources; a non-nil client does NOT imply the service
// is reachable or ready.
type SemanticClient interface {
	Capabilities(ctx context.Context) (types.SemanticCapabilities, error)
	Apply(ctx context.Context, req types.SemanticApplyRequest) (types.SemanticOperation, error)
	Delete(ctx context.Context, rev types.SemanticDocumentRevision) (types.SemanticOperation, error)
	Get(ctx context.Context, scope types.SemanticScopeKey, operationID string) (types.SemanticOperation, error)
	Cancel(ctx context.Context, scope types.SemanticScopeKey, operationID string) (types.SemanticOperation, error)
	Search(ctx context.Context, req types.SemanticSearchRequest) (types.SemanticSearchResponse, error)
	Reason(ctx context.Context, req types.SemanticReasonRequest) (types.SemanticReasonResponse, error)
	Close() error
}
