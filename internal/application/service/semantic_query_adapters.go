package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)
// NewSemanticClientSearcher adapts the SemanticClient for the facade.
func NewSemanticClientSearcher(client interfaces.SemanticClient) SemanticSearcher {
	return semanticClientSearcher{client: client}
}

// NewSemanticClientReasoner adapts the Reason RPC for the facade,
// mapping the request mode onto the wire reasoning modes.
func NewSemanticClientReasoner(client interfaces.SemanticClient) SemanticReasoner {
	return semanticClientReasoner{client: client}
}

type semanticClientReasoner struct {
	client interfaces.SemanticClient
}

func (s semanticClientReasoner) Reason(ctx context.Context, issued types.SemanticAccessScope, req ReasonRequest) (*types.SemanticReasonResult, error) {
	scope := issued
	wireMode := types.SemanticReasoningModeRules
	if req.Mode == "model" {
		wireMode = types.SemanticReasoningModeModel
	}
	response, err := s.client.Reason(ctx, types.SemanticReasonRequest{
		Search: types.SemanticSearchRequest{
			QueryID:     fmt.Sprintf("reason-%d", time.Now().UTC().UnixNano()),
			Query:       req.Query,
			AccessScope: &scope,
		},
		ReasoningMode:  wireMode,
		RuleSetVersion: req.RuleSetVersion,
	})
	if err != nil {
		return nil, err
	}
	return &types.SemanticReasonResult{
		Mode:       response.Retrieval.Mode,
		Status:     response.Status,
		Conclusion: response.Conclusion,
		Evidence:   nil,
	}, nil
}

// NewNoopVectorSearcher returns an ALWAYS-EMPTY ordinary engine. The
// semantic gRPC service has no ordinary-retrieval mode (mode!="graphrag"
// is FAILED_PRECONDITION), so honest wiring means semantic-only fusion
// until the real retriever-stack seam lands (W-wiring; ledgered).
func NewNoopVectorSearcher() VectorSearcher {
	return noopVectorSearcher{}
}

type noopVectorSearcher struct{}

func (noopVectorSearcher) Search(ctx context.Context, query string) ([]types.SemanticEvidence, error) {
	return nil, nil
}

// semanticClientSearcher adapts the real SemanticClient gRPC surface to
// the facade's SemanticSearcher seam, carrying the issued scope.
type semanticClientSearcher struct {
	client interfaces.SemanticClient
}

func (s semanticClientSearcher) Search(ctx context.Context, issued types.SemanticAccessScope, query string, topK int) ([]types.SemanticEvidence, string, error) {
	scope := issued
	request := types.SemanticSearchRequest{
		QueryID:     fmt.Sprintf("q-%d", time.Now().UTC().UnixNano()),
		Query:       query,
		AccessScope: &scope,
		Mode:        "graphrag",
	}
	if topK > 0 {
		request.Limits = &types.SemanticQueryLimits{TopK: uint32(topK)}
	}
	response, err := s.client.Search(ctx, request)
	if err != nil {
		return nil, "", err
	}
	return response.Evidence, response.Mode, nil
}

