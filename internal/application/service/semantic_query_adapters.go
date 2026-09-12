package service

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)
// NewSemanticClientSearcher adapts the SemanticClient for the facade.
func NewSemanticClientSearcher(client interfaces.SemanticClient) SemanticSearcher {
	return semanticClientSearcher{client: client}
}

// NewSemanticClientVectorSearcher adapts ordinary retrieval for fusion.
func NewSemanticClientVectorSearcher(client interfaces.SemanticClient) VectorSearcher {
	return semanticClientVectorSearcher{client: client}
}

// semanticClientSearcher adapts the real SemanticClient gRPC surface to
// the facade's SemanticSearcher seam, carrying the issued scope.
type semanticClientSearcher struct {
	client interfaces.SemanticClient
}

func (s semanticClientSearcher) Search(ctx context.Context, issued types.SemanticAccessScope) ([]types.SemanticEvidence, string, error) {
	scope := issued
	response, err := s.client.Search(ctx, types.SemanticSearchRequest{AccessScope: &scope})
	if err != nil {
		return nil, "", err
	}
	return response.Evidence, response.Mode, nil
}

// semanticClientVectorSearcher adapts ordinary retrieval for fusion.
// HONEST LIMITATION: the semantic gRPC service supports mode "graphrag"
// only, so ordinary-retrieval fusion cannot run against it in production
// (the real seam targets WeKnora's own retriever stack - W-wiring; see
// the ledger). Requests fail and the facade degrades explicitly.
type semanticClientVectorSearcher struct {
	client interfaces.SemanticClient
}

func (s semanticClientVectorSearcher) Search(ctx context.Context, query string) ([]types.SemanticEvidence, error) {
	response, err := s.client.Search(ctx, types.SemanticSearchRequest{Query: query, Mode: "retrieval"})
	if err != nil {
		return nil, err
	}
	return response.Evidence, nil
}