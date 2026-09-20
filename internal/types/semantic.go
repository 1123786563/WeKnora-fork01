package types

import (
	"fmt"

	semanticpb "github.com/Tencent/WeKnora/semantic/proto"
)

type SemanticScopeKey struct {
	TenantID uint64
	KBID     string
}

type SemanticDocumentRevision struct {
	Scope       SemanticScopeKey
	DocumentID  string
	Revision    uint64
	ContentHash string
	Deleted     bool
}

func SemanticDocumentRevisionFromWire(wire *semanticpb.DocumentRevision) (SemanticDocumentRevision, error) {
	if wire == nil || wire.Scope == nil || wire.Scope.KbId == "" || wire.DocumentId == "" || wire.Revision == 0 {
		return SemanticDocumentRevision{}, fmt.Errorf("semantic document revision is incomplete")
	}
	if !wire.Deleted {
		return SemanticDocumentRevision{}, fmt.Errorf("semantic delete requires deleted=true")
	}
	return SemanticDocumentRevision{Scope: SemanticScopeKey{TenantID: wire.Scope.TenantId, KBID: wire.Scope.KbId}, DocumentID: wire.DocumentId, Revision: wire.Revision, ContentHash: wire.ContentHash, Deleted: wire.Deleted}, nil
}

type SemanticRetrievalMode string

const (
	SemanticRetrievalModeGraphRAG SemanticRetrievalMode = "graph_rag"
	SemanticRetrievalModeReason   SemanticRetrievalMode = "reason"
)

type SemanticSearchResponse struct {
	RequestedMode SemanticRetrievalMode
	ActualMode    SemanticRetrievalMode
}

func semanticRetrievalModeFromWire(mode semanticpb.RetrievalMode) (SemanticRetrievalMode, error) {
	switch mode {
	case semanticpb.RetrievalMode_RETRIEVAL_MODE_GRAPH_RAG:
		return SemanticRetrievalModeGraphRAG, nil
	case semanticpb.RetrievalMode_RETRIEVAL_MODE_REASON:
		return SemanticRetrievalModeReason, nil
	default:
		return "", fmt.Errorf("unsupported semantic retrieval mode: %s", mode.String())
	}
}

func SemanticSearchResponseFromWire(wire *semanticpb.SearchResponse) (SemanticSearchResponse, error) {
	if wire == nil {
		return SemanticSearchResponse{}, fmt.Errorf("semantic search response is nil")
	}
	requested, err := semanticRetrievalModeFromWire(wire.RequestedMode)
	if err != nil {
		return SemanticSearchResponse{}, err
	}
	actual, err := semanticRetrievalModeFromWire(wire.ActualMode)
	if err != nil {
		return SemanticSearchResponse{}, err
	}
	return SemanticSearchResponse{RequestedMode: requested, ActualMode: actual}, nil
}
