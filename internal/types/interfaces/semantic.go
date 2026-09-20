package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// SemanticClient is the Go transport-neutral seam for the independent
// Semantica service. Implementations map wire DTOs at the boundary.
type SemanticClient interface {
	GetCapabilities(context.Context) (*types.SemanticCapabilities, error)
	ApplyDocumentRevision(context.Context, types.SemanticApplyRequest) (*types.SemanticOperation, error)
	DeleteDocument(context.Context, types.SemanticDocumentRevision) (*types.SemanticOperation, error)
	GetOperation(context.Context, types.SemanticOperationRef) (*types.SemanticOperation, error)
	CancelOperation(context.Context, types.SemanticOperationRef) (*types.SemanticOperation, error)
	Search(context.Context, types.SemanticSearchRequest) (*types.SemanticSearchResponse, error)
	Reason(context.Context, types.SemanticReasonRequest) (*types.SemanticReasonResponse, error)
	Close() error
}
